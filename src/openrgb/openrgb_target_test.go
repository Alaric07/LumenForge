package openrgb

import (
	"LumenForge/src/common"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net"
	"sync"
	"testing"
	"time"
)

const targetTestUnknownOpcode uint32 = 999999

type blockingTargetWriteConn struct {
	net.Conn
	writeStarted chan struct{}
	release      chan struct{}
	startedOnce  sync.Once
	releaseOnce  sync.Once
}

func (c *blockingTargetWriteConn) Write(payload []byte) (int, error) {
	c.startedOnce.Do(func() { close(c.writeStarted) })
	<-c.release
	return len(payload), nil
}

func (c *blockingTargetWriteConn) releaseWrite() {
	c.releaseOnce.Do(func() { close(c.release) })
}

func targetTestHeader(deviceID, packetType, packetSize uint32) []byte {
	return makeHeader(deviceID, packetType, packetSize)
}

func writeTargetTestPacket(t *testing.T, conn net.Conn, deviceID, packetType uint32, payload []byte) {
	t.Helper()
	if err := conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set target test write deadline: %v", err)
	}
	packet := append(targetTestHeader(deviceID, packetType, uint32(len(payload))), payload...)
	if err := writeAll(conn, packet); err != nil {
		t.Fatalf("write target test packet: %v", err)
	}
}

func readTargetTestResponse(t *testing.T, conn net.Conn) (uint32, uint32, []byte) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set target test read deadline: %v", err)
	}
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(conn, header); err != nil {
		t.Fatalf("read target response header: %v", err)
	}
	if string(header[:4]) != "ORGB" {
		t.Fatalf("target response magic = %q", header[:4])
	}
	deviceID := binary.LittleEndian.Uint32(header[4:8])
	packetType := binary.LittleEndian.Uint32(header[8:12])
	size := binary.LittleEndian.Uint32(header[12:16])
	payload := make([]byte, int(size))
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatalf("read target response payload: %v", err)
	}
	return deviceID, packetType, payload
}

func startTargetPipe(t *testing.T) (net.Conn, <-chan struct{}) {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		handleConn(serverConn)
		close(done)
	}()
	t.Cleanup(func() {
		_ = clientConn.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("target connection handler did not stop")
		}
	})
	return clientConn, done
}

func expectTargetPeerClosed(t *testing.T, conn net.Conn, within time.Duration) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(within)); err != nil {
		if errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
			return
		}
		t.Fatalf("set close-observation deadline: %v", err)
	}
	buffer := make([]byte, 1)
	if _, err := conn.Read(buffer); err == nil {
		t.Fatal("target connection remained open")
	} else if netError, ok := err.(net.Error); ok && netError.Timeout() {
		t.Fatalf("target connection was not closed within %s", within)
	}
}

func useShortTargetTimeouts(t *testing.T, timeout time.Duration) {
	t.Helper()
	previousHeader := targetHeaderTimeout
	previousPayload := targetPayloadTimeout
	previousWrite := targetWriteTimeout
	previousIdle := targetIdleTimeout
	targetHeaderTimeout = timeout
	targetPayloadTimeout = timeout
	targetWriteTimeout = timeout
	targetIdleTimeout = timeout
	t.Cleanup(func() {
		targetHeaderTimeout = previousHeader
		targetPayloadTimeout = previousPayload
		targetWriteTimeout = previousWrite
		targetIdleTimeout = previousIdle
	})
}

func waitForTargetClientCount(t *testing.T, server *targetServer, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := targetClientCountForTest(server); got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("active target clients = %d, want %d", targetClientCountForTest(server), want)
}

func targetClientCountForTest(server *targetServer) int {
	server.mu.Lock()
	defer server.mu.Unlock()
	return len(server.clients)
}

func installBlockingTargetClientForTest(t *testing.T) *blockingTargetWriteConn {
	t.Helper()
	serverConn, peerConn := net.Pipe()
	conn := &blockingTargetWriteConn{
		Conn:         serverConn,
		writeStarted: make(chan struct{}),
		release:      make(chan struct{}),
	}
	client := &targetClient{conn: conn}
	server := newTargetServer(nil)
	server.clients[client] = struct{}{}
	server.latest = client

	targetLifecycleMutex.Lock()
	if currentTargetServer != nil {
		targetLifecycleMutex.Unlock()
		_ = serverConn.Close()
		_ = peerConn.Close()
		t.Fatal("an OpenRGB target server was already active")
	}
	currentTargetServer = server
	targetLifecycleMutex.Unlock()

	t.Cleanup(func() {
		conn.releaseWrite()
		targetLifecycleMutex.Lock()
		if currentTargetServer == server {
			currentTargetServer = nil
		}
		targetLifecycleMutex.Unlock()
		client.close()
		_ = peerConn.Close()
	})
	return conn
}

func TestTargetNotificationsDoNotHoldControllerMutexDuringWrite(t *testing.T) {
	tests := []struct {
		name   string
		notify func()
	}{
		{name: "send", notify: SendToOpenRGB},
		{name: "update", notify: func() {
			UpdateDeviceController("original", &common.OpenRGBController{Serial: "replacement"})
		}},
		{name: "remove", notify: func() { RemoveDeviceControllerBySerial("original") }},
		{name: "controller change", notify: func() { NotifyControllerChange("original") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutex.Lock()
			previousControllers := controllers
			controllers = []*common.OpenRGBController{{Serial: "original"}}
			mutex.Unlock()
			t.Cleanup(func() {
				mutex.Lock()
				controllers = previousControllers
				mutex.Unlock()
			})

			conn := installBlockingTargetClientForTest(t)
			notified := make(chan struct{})
			go func() {
				test.notify()
				close(notified)
			}()

			select {
			case <-conn.writeStarted:
			case <-time.After(time.Second):
				t.Fatal("notification write did not start")
			}

			mutexAvailable := make(chan struct{})
			go func() {
				mutex.Lock()
				mutex.Unlock()
				close(mutexAvailable)
			}()
			select {
			case <-mutexAvailable:
			case <-time.After(time.Second):
				conn.releaseWrite()
				<-notified
				t.Fatal("controller mutex remained held during notification write")
			}

			conn.releaseWrite()
			select {
			case <-notified:
			case <-time.After(time.Second):
				t.Fatal("notification did not finish after write release")
			}
		})
	}
}

func TestTargetPacketSizeLimits(t *testing.T) {
	valid := []struct {
		name       string
		packetType uint32
		sizes      []uint32
	}{
		{name: "controller count", packetType: OPCODE_REQUEST_CONTROLLER_COUNT, sizes: []uint32{0}},
		{name: "controller data", packetType: OPCODE_REQUEST_CONTROLLER_DATA, sizes: []uint32{0, 4}},
		{name: "protocol version", packetType: OPCODE_REQUEST_PROTOCOL_VERSION, sizes: []uint32{0, 4}},
		{name: "client name", packetType: OPCODE_SET_CLIENT_NAME, sizes: []uint32{0, maxClientNameSize}},
		{name: "LED update", packetType: OPCODE_RGBCONTROLLER_UPDATELEDS, sizes: []uint32{6, maxLEDUpdateSize}},
		{name: "mode update", packetType: OPCODE_UPDATE_MODE, sizes: []uint32{4, maxPayloadSize}},
		{name: "profile list", packetType: OPCODE_REQUEST_PROFILE_LIST, sizes: []uint32{0}},
		{name: "plugin list", packetType: OPCODE_REQUEST_PLUGIN_LIST, sizes: []uint32{0}},
		{name: "unimplemented opcode", packetType: targetTestUnknownOpcode, sizes: []uint32{0, maxPayloadSize}},
	}
	for _, test := range valid {
		for _, size := range test.sizes {
			if !validTargetPacketSize(test.packetType, size) {
				t.Errorf("%s payload size %d rejected", test.name, size)
			}
		}
	}

	invalid := []struct {
		name       string
		packetType uint32
		size       uint32
	}{
		{name: "controller count payload", packetType: OPCODE_REQUEST_CONTROLLER_COUNT, size: 1},
		{name: "controller data payload", packetType: OPCODE_REQUEST_CONTROLLER_DATA, size: 1},
		{name: "protocol payload", packetType: OPCODE_REQUEST_PROTOCOL_VERSION, size: 3},
		{name: "client name above cap", packetType: OPCODE_SET_CLIENT_NAME, size: maxClientNameSize + 1},
		{name: "LED update too short", packetType: OPCODE_RGBCONTROLLER_UPDATELEDS, size: 5},
		{name: "LED update above cap", packetType: OPCODE_RGBCONTROLLER_UPDATELEDS, size: maxLEDUpdateSize + 1},
		{name: "mode update too short", packetType: OPCODE_UPDATE_MODE, size: 3},
		{name: "profile list payload", packetType: OPCODE_REQUEST_PROFILE_LIST, size: 1},
		{name: "plugin list payload", packetType: OPCODE_REQUEST_PLUGIN_LIST, size: 1},
		{name: "protocol limit exceeded", packetType: OPCODE_UPDATE_MODE, size: maxPayloadSize + 1},
		{name: "unimplemented opcode above protocol limit", packetType: targetTestUnknownOpcode, size: maxPayloadSize + 1},
		{name: "maximum uint32", packetType: OPCODE_UPDATE_MODE, size: math.MaxUint32},
	}
	for _, test := range invalid {
		if validTargetPacketSize(test.packetType, test.size) {
			t.Errorf("%s payload size %d accepted", test.name, test.size)
		}
	}
}

func TestTargetRejectsMaximumUint32PayloadWithoutWaitingForPayload(t *testing.T) {
	for _, packetType := range []uint32{OPCODE_UPDATE_MODE, targetTestUnknownOpcode} {
		conn, _ := startTargetPipe(t)
		if err := conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatalf("set write deadline: %v", err)
		}
		if err := writeAll(conn, targetTestHeader(0, packetType, math.MaxUint32)); err != nil {
			t.Fatalf("write oversized target header for opcode %d: %v", packetType, err)
		}
		expectTargetPeerClosed(t, conn, time.Second)
	}
}

func TestTargetRejectsInvalidOpcodePayloadBeforeBody(t *testing.T) {
	tests := []struct {
		name       string
		packetType uint32
		packetSize uint32
	}{
		{name: "zero-payload request", packetType: OPCODE_REQUEST_CONTROLLER_COUNT, packetSize: 1},
		{name: "oversized client name", packetType: OPCODE_SET_CLIENT_NAME, packetSize: maxClientNameSize + 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn, _ := startTargetPipe(t)
			if err := writeAll(conn, targetTestHeader(0, test.packetType, test.packetSize)); err != nil {
				t.Fatalf("write invalid target header: %v", err)
			}
			expectTargetPeerClosed(t, conn, time.Second)
		})
	}
}

func TestTargetReadAndWriteDeadlines(t *testing.T) {
	const timeout = 25 * time.Millisecond

	t.Run("idle connection", func(t *testing.T) {
		useShortTargetTimeouts(t, timeout)
		conn, _ := startTargetPipe(t)
		expectTargetPeerClosed(t, conn, time.Second)
	})

	t.Run("partial header", func(t *testing.T) {
		useShortTargetTimeouts(t, timeout)
		conn, _ := startTargetPipe(t)
		if _, err := conn.Write([]byte("O")); err != nil {
			t.Fatalf("write partial header: %v", err)
		}
		expectTargetPeerClosed(t, conn, time.Second)
	})

	t.Run("partial payload", func(t *testing.T) {
		useShortTargetTimeouts(t, timeout)
		conn, _ := startTargetPipe(t)
		if err := writeAll(conn, append(targetTestHeader(0, OPCODE_SET_CLIENT_NAME, 2), 'x')); err != nil {
			t.Fatalf("write partial payload: %v", err)
		}
		expectTargetPeerClosed(t, conn, time.Second)
	})

	t.Run("blocked response", func(t *testing.T) {
		useShortTargetTimeouts(t, timeout)
		conn, done := startTargetPipe(t)
		writeTargetTestPacket(t, conn, 0, OPCODE_REQUEST_CONTROLLER_COUNT, nil)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("blocked target write did not time out")
		}
	})
}

func TestTargetPreservesSupportedProtocolOperations(t *testing.T) {
	colors := make(chan []byte, 1)
	controller := &common.OpenRGBController{
		Name:        "Target test controller",
		Vendor:      "LumenForge",
		Description: "OpenRGB target test",
		FwVersion:   "1.0",
		Serial:      "target-test",
		Location:    "test",
		Zones: []common.OpenRGBZone{{
			Name:     "Zone",
			NumLEDs:  1,
			MinLeds:  1,
			ZoneType: common.ZoneTypeLinear,
		}},
		Colors:     []byte{1, 2, 3},
		ActiveMode: 2,
		WriteColorEx: func(frame []byte, _ int) {
			colors <- append([]byte(nil), frame...)
		},
	}
	mutex.Lock()
	previousControllers := controllers
	controllers = []*common.OpenRGBController{controller}
	mutex.Unlock()
	t.Cleanup(func() {
		mutex.Lock()
		controllers = previousControllers
		mutex.Unlock()
	})

	conn, done := startTargetPipe(t)
	writeTargetTestPacket(t, conn, 0, OPCODE_SET_CLIENT_NAME, []byte("target-test-client\x00"))

	protocolRequest := make([]byte, 4)
	binary.LittleEndian.PutUint32(protocolRequest, protocolVersion)
	writeTargetTestPacket(t, conn, 0, OPCODE_REQUEST_PROTOCOL_VERSION, protocolRequest)
	deviceID, packetType, payload := readTargetTestResponse(t, conn)
	if deviceID != 0 || packetType != OPCODE_REQUEST_PROTOCOL_VERSION || len(payload) != 4 || binary.LittleEndian.Uint32(payload) != protocolVersion {
		t.Fatalf("protocol response = device %d opcode %d payload %v", deviceID, packetType, payload)
	}

	writeTargetTestPacket(t, conn, 0, OPCODE_REQUEST_CONTROLLER_COUNT, nil)
	deviceID, packetType, payload = readTargetTestResponse(t, conn)
	if deviceID != 0 || packetType != OPCODE_REQUEST_CONTROLLER_COUNT || len(payload) != 4 || binary.LittleEndian.Uint32(payload) != 1 {
		t.Fatalf("controller count response = device %d opcode %d payload %v", deviceID, packetType, payload)
	}

	writeTargetTestPacket(t, conn, 0, OPCODE_REQUEST_CONTROLLER_DATA, protocolRequest)
	deviceID, packetType, payload = readTargetTestResponse(t, conn)
	if deviceID != 0 || packetType != OPCODE_REQUEST_CONTROLLER_DATA || len(payload) == 0 {
		t.Fatalf("controller data response = device %d opcode %d payload length %d", deviceID, packetType, len(payload))
	}

	writeTargetTestPacket(t, conn, 0, targetTestUnknownOpcode, []byte{1, 2, 3})
	writeTargetTestPacket(t, conn, 0, OPCODE_REQUEST_CONTROLLER_COUNT, nil)
	deviceID, packetType, payload = readTargetTestResponse(t, conn)
	if deviceID != 0 || packetType != OPCODE_REQUEST_CONTROLLER_COUNT || len(payload) != 4 {
		t.Fatalf("response after unimplemented opcode = device %d opcode %d payload %v", deviceID, packetType, payload)
	}

	malformedLEDs := make([]byte, 10)
	binary.LittleEndian.PutUint32(malformedLEDs[:4], uint32(len(malformedLEDs)))
	binary.LittleEndian.PutUint16(malformedLEDs[4:6], 2)
	writeTargetTestPacket(t, conn, 0, OPCODE_RGBCONTROLLER_UPDATELEDS, malformedLEDs)
	writeTargetTestPacket(t, conn, 0, OPCODE_REQUEST_CONTROLLER_COUNT, nil)
	_, _, _ = readTargetTestResponse(t, conn)
	select {
	case frame := <-colors:
		t.Fatalf("malformed LED update dispatched frame %v", frame)
	default:
	}

	ledPayload := make([]byte, 10)
	binary.LittleEndian.PutUint32(ledPayload[:4], uint32(len(ledPayload)))
	binary.LittleEndian.PutUint16(ledPayload[4:6], 1)
	copy(ledPayload[6:], []byte{10, 20, 30, 255})
	writeTargetTestPacket(t, conn, 0, opcodeSetCustomMode, nil)
	writeTargetTestPacket(t, conn, 0, OPCODE_RGBCONTROLLER_UPDATELEDS, ledPayload)
	select {
	case frame := <-colors:
		if string(frame) != string([]byte{10, 20, 30}) {
			t.Fatalf("LED update frame = %v", frame)
		}
	case <-time.After(time.Second):
		t.Fatal("LED update was not dispatched")
	}

	modePayload := make([]byte, 4)
	binary.LittleEndian.PutUint32(modePayload, 1)
	writeTargetTestPacket(t, conn, 0, OPCODE_UPDATE_MODE, modePayload)

	for _, opcode := range []uint32{OPCODE_REQUEST_PROFILE_LIST, OPCODE_REQUEST_PLUGIN_LIST} {
		writeTargetTestPacket(t, conn, 0, opcode, nil)
		deviceID, packetType, payload = readTargetTestResponse(t, conn)
		if deviceID != 0 || packetType != opcode || len(payload) != 8 {
			t.Fatalf("list response = device %d opcode %d payload %v", deviceID, packetType, payload)
		}
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("close target test client: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("target handler did not stop after protocol test")
	}
	mutex.RLock()
	activeMode := controller.ActiveMode
	mutex.RUnlock()
	if activeMode != 0 {
		t.Fatalf("updated mode = %d, want 0", activeMode)
	}
}

func TestTargetClientCapSlotReleaseAndClose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for target test: %v", err)
	}
	server := newTargetServer(listener)
	targetLifecycleMutex.Lock()
	if currentTargetServer != nil {
		targetLifecycleMutex.Unlock()
		_ = listener.Close()
		t.Fatal("an OpenRGB target server was already active")
	}
	currentTargetServer = server
	targetLifecycleMutex.Unlock()
	go server.serve()
	t.Cleanup(Close)

	clients := make([]net.Conn, 0, maxTargetClients+1)
	for i := 0; i < maxTargetClients; i++ {
		client, dialErr := net.Dial("tcp", listener.Addr().String())
		if dialErr != nil {
			t.Fatalf("dial admitted target client %d: %v", i, dialErr)
		}
		clients = append(clients, client)
	}
	waitForTargetClientCount(t, server, maxTargetClients)

	rejected, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial excess target client: %v", err)
	}
	expectTargetPeerClosed(t, rejected, time.Second)
	_ = rejected.Close()
	if got := targetClientCountForTest(server); got != maxTargetClients {
		t.Fatalf("active target clients after rejection = %d, want %d", got, maxTargetClients)
	}

	if err := clients[0].Close(); err != nil {
		t.Fatalf("close admitted target client: %v", err)
	}
	waitForTargetClientCount(t, server, maxTargetClients-1)
	replacement, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial replacement target client: %v", err)
	}
	clients = append(clients, replacement)
	waitForTargetClientCount(t, server, maxTargetClients)

	closed := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		go func() {
			Close()
			closed <- struct{}{}
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-closed:
		case <-time.After(time.Second):
			t.Fatal("concurrent Close did not return")
		}
	}
	Close()
	if got := targetClientCountForTest(server); got != 0 {
		t.Fatalf("active target clients after Close = %d", got)
	}
	for _, client := range clients[1:] {
		expectTargetPeerClosed(t, client, time.Second)
		_ = client.Close()
	}
}

func TestTargetNotificationUsesTrackedClient(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for notification test: %v", err)
	}
	server := newTargetServer(listener)
	targetLifecycleMutex.Lock()
	if currentTargetServer != nil {
		targetLifecycleMutex.Unlock()
		_ = listener.Close()
		t.Fatal("an OpenRGB target server was already active")
	}
	currentTargetServer = server
	targetLifecycleMutex.Unlock()
	go server.serve()
	t.Cleanup(Close)

	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial notification client: %v", err)
	}
	defer client.Close()
	waitForTargetClientCount(t, server, 1)
	SendToOpenRGB()
	deviceID, packetType, payload := readTargetTestResponse(t, client)
	if deviceID != 0 || packetType != OPCODE_DEVICE_LIST_UPDATED || len(payload) != 0 {
		t.Fatalf("notification = device %d opcode %d payload %v", deviceID, packetType, payload)
	}
}
