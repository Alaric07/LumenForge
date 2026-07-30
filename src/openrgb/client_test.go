package openrgb

import (
	"LumenForge/src/config"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func withFakeSDKServer(t *testing.T, handler func(net.Conn)) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	previousAddress := sdkAddress
	previousDial := dialContext
	done := make(chan struct{})
	if err == nil {
		sdkAddress = func() (string, error) { return listener.Addr().String(), nil }
		go func() {
			defer close(done)
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			defer conn.Close()
			handler(conn)
		}()
	} else {
		clientConn, serverConn := net.Pipe()
		sdkAddress = func() (string, error) { return "pipe", nil }
		dialContext = func(context.Context, string, string) (net.Conn, error) {
			return clientConn, nil
		}
		go func() {
			defer close(done)
			defer serverConn.Close()
			handler(serverConn)
		}()
	}

	t.Cleanup(func() {
		sdkAddress = previousAddress
		dialContext = previousDial
		if listener != nil {
			_ = listener.Close()
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("fake OpenRGB server did not stop")
		}
	})
}

type sdkRequest struct {
	controllerID uint32
	opcode       uint32
	payload      []byte
}

func readRequest(t *testing.T, conn net.Conn) sdkRequest {
	t.Helper()
	header := make([]byte, 16)
	if _, err := io.ReadFull(conn, header); err != nil {
		t.Fatalf("read request: %v", err)
	}
	if string(header[:4]) != "ORGB" {
		t.Fatalf("request magic = %q, want ORGB", header[:4])
	}
	size := binary.LittleEndian.Uint32(header[12:16])
	payload := make([]byte, size)
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatalf("read request payload: %v", err)
	}
	return sdkRequest{
		controllerID: binary.LittleEndian.Uint32(header[4:8]),
		opcode:       binary.LittleEndian.Uint32(header[8:12]),
		payload:      payload,
	}
}

func responseHeader(magic string, controllerID, opcode, size uint32) []byte {
	header := make([]byte, 16)
	copy(header[:4], magic)
	binary.LittleEndian.PutUint32(header[4:8], controllerID)
	binary.LittleEndian.PutUint32(header[8:12], opcode)
	binary.LittleEndian.PutUint32(header[12:16], size)
	return header
}

func writeControllerCount(conn net.Conn, count uint32) error {
	if _, err := conn.Write(responseHeader("ORGB", 0, opcodeRequestControllerCount, 4)); err != nil {
		return err
	}
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint32(payload, count)
	_, err := conn.Write(payload)
	return err
}

func writeProtocolVersion(conn net.Conn, version uint32) error {
	if _, err := conn.Write(responseHeader("ORGB", 0, opcodeRequestProtocolVersion, 4)); err != nil {
		return err
	}
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint32(payload, version)
	_, err := conn.Write(payload)
	return err
}

func negotiateTestProtocol(t *testing.T, conn net.Conn, serverVersion uint32) uint32 {
	t.Helper()
	request := readRequest(t, conn)
	if request.controllerID != 0 || request.opcode != opcodeRequestProtocolVersion {
		t.Fatalf("protocol request = %#v", request)
	}
	if len(request.payload) != 4 {
		t.Fatalf("protocol request payload size = %d, want 4", len(request.payload))
	}
	clientVersion := binary.LittleEndian.Uint32(request.payload)
	if clientVersion != maxSupportedProtocolVersion {
		t.Fatalf("client protocol version = %d, want %d", clientVersion, maxSupportedProtocolVersion)
	}
	if err := writeProtocolVersion(conn, serverVersion); err != nil {
		t.Fatalf("write protocol version: %v", err)
	}
	if serverVersion < clientVersion {
		return serverVersion
	}
	return clientVersion
}

type testSegment struct {
	name     string
	ledCount uint32
}

type testZone struct {
	name     string
	ledCount uint32
	segments []testSegment
}

func writeTestString(buf *bytes.Buffer, value string) {
	encoded := append([]byte(value), 0)
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(encoded)))
	_, _ = buf.Write(encoded)
}

func testColorsFromBytes(data []byte) []uint32 {
	for len(data)%4 != 0 {
		data = append(data, 0)
	}
	colors := make([]uint32, len(data)/4)
	for index := range colors {
		colors[index] = binary.LittleEndian.Uint32(data[index*4 : index*4+4])
	}
	return colors
}

func controllerPayload(protocol uint32, name, vendor, description, version, serial, location string, zones []testZone, ledCount uint16, modeColors []uint32) []byte {
	payload := new(bytes.Buffer)
	_ = binary.Write(payload, binary.LittleEndian, uint32(0))
	_ = binary.Write(payload, binary.LittleEndian, int32(0))
	writeTestString(payload, name)
	if protocol >= 1 {
		writeTestString(payload, vendor)
	}
	for _, value := range []string{description, version, serial, location} {
		writeTestString(payload, value)
	}

	_ = binary.Write(payload, binary.LittleEndian, uint16(1))
	_ = binary.Write(payload, binary.LittleEndian, int32(0))
	writeTestString(payload, "Direct")
	for range 4 {
		_ = binary.Write(payload, binary.LittleEndian, uint32(0))
	}
	if protocol >= 3 {
		for range 2 {
			_ = binary.Write(payload, binary.LittleEndian, uint32(100))
		}
	}
	for range 3 {
		_ = binary.Write(payload, binary.LittleEndian, uint32(0))
	}
	if protocol >= 3 {
		_ = binary.Write(payload, binary.LittleEndian, uint32(100))
	}
	for range 2 {
		_ = binary.Write(payload, binary.LittleEndian, uint32(0))
	}
	_ = binary.Write(payload, binary.LittleEndian, uint16(len(modeColors)))
	for _, color := range modeColors {
		_ = binary.Write(payload, binary.LittleEndian, color)
	}

	_ = binary.Write(payload, binary.LittleEndian, uint16(len(zones)))
	for _, zone := range zones {
		writeTestString(payload, zone.name)
		_ = binary.Write(payload, binary.LittleEndian, int32(2))
		_ = binary.Write(payload, binary.LittleEndian, zone.ledCount)
		_ = binary.Write(payload, binary.LittleEndian, zone.ledCount)
		_ = binary.Write(payload, binary.LittleEndian, zone.ledCount)
		_ = binary.Write(payload, binary.LittleEndian, uint16(0))
		if protocol >= 4 {
			_ = binary.Write(payload, binary.LittleEndian, uint16(len(zone.segments)))
			for _, segment := range zone.segments {
				writeTestString(payload, segment.name)
				_ = binary.Write(payload, binary.LittleEndian, int32(0))
				_ = binary.Write(payload, binary.LittleEndian, uint32(0))
				_ = binary.Write(payload, binary.LittleEndian, segment.ledCount)
			}
		}
	}

	_ = binary.Write(payload, binary.LittleEndian, ledCount)
	for ledIndex := 0; ledIndex < int(ledCount); ledIndex++ {
		writeTestString(payload, "LED")
		_ = binary.Write(payload, binary.LittleEndian, uint32(ledIndex))
	}
	_ = binary.Write(payload, binary.LittleEndian, ledCount)
	for ledIndex := 0; ledIndex < int(ledCount); ledIndex++ {
		_ = binary.Write(payload, binary.LittleEndian, uint32(0))
	}

	result := payload.Bytes()
	binary.LittleEndian.PutUint32(result[:4], uint32(len(result)))
	return result
}

func TestDiscoverControllersValidTransactionSetsConnected(t *testing.T) {
	withFakeSDKServer(t, func(conn net.Conn) {
		negotiateTestProtocol(t, conn, maxSupportedProtocolVersion)
		request := readRequest(t, conn)
		if request.opcode != opcodeRequestControllerCount {
			t.Fatalf("controller count request = %#v", request)
		}
		if err := writeControllerCount(conn, 0); err != nil {
			t.Errorf("write response: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	controllers, err := DiscoverControllersContext(ctx)
	if err != nil {
		t.Fatalf("DiscoverControllersContext: %v", err)
	}
	if len(controllers) != 0 {
		t.Fatalf("got %d controllers, want 0", len(controllers))
	}
	state, statusErr := GetStatus()
	if state != StateConnected || statusErr != nil {
		t.Fatalf("status = %q, %v; want Connected, nil", state, statusErr)
	}
}

func TestStatusNeutralDiscoveryPreservesGlobalStatus(t *testing.T) {
	withFakeSDKServer(t, func(conn net.Conn) {
		negotiateTestProtocol(t, conn, maxSupportedProtocolVersion)
		readRequest(t, conn)
		if err := writeControllerCount(conn, 0); err != nil {
			t.Errorf("write response: %v", err)
		}
	})

	sentinel := errors.New("manager retry state")
	SetDisconnected(sentinel)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	controllers, err := DiscoverControllersStatusNeutralContext(ctx)
	if err != nil {
		t.Fatalf("DiscoverControllersStatusNeutralContext: %v", err)
	}
	if len(controllers) != 0 {
		t.Fatalf("got %d controllers, want 0", len(controllers))
	}
	state, statusErr := GetStatus()
	if state != StateOffline || statusErr != sentinel {
		t.Fatalf("status = %q, %v; want unchanged Offline sentinel", state, statusErr)
	}
}

func TestStatusNeutralDiscoveryFailurePreservesGlobalStatus(t *testing.T) {
	previousAddress := sdkAddress
	sdkAddress = func() (string, error) { return "", errors.New("injected address failure") }
	t.Cleanup(func() { sdkAddress = previousAddress })

	SetConnected()
	if _, err := DiscoverControllersStatusNeutralContext(context.Background()); err == nil {
		t.Fatal("expected status-neutral discovery failure")
	}
	state, statusErr := GetStatus()
	if state != StateConnected || statusErr != nil {
		t.Fatalf("status = %q, %v; want unchanged Connected status", state, statusErr)
	}
}

func TestDialAloneDoesNotMarkSDKConnected(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	previousAddress := sdkAddress
	previousDial := dialContext
	sdkAddress = func() (string, error) { return "pipe", nil }
	dialContext = func(context.Context, string, string) (net.Conn, error) { return clientConn, nil }
	t.Cleanup(func() {
		sdkAddress = previousAddress
		dialContext = previousDial
	})
	SetDisconnected(errors.New("not connected"))

	conn, err := dial(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	state, _ := GetStatus()
	if state != StateOffline {
		t.Fatalf("status = %q after dial, want Offline", state)
	}
}

func TestSDKAddressUsesIPv4LoopbackAndConfiguredOpenRGBPort(t *testing.T) {
	tests := []struct {
		name          string
		listenAddress string
		openRGBPort   int
		want          string
	}{
		{name: "default port", listenAddress: "0.0.0.0", openRGBPort: 6742, want: "127.0.0.1:6742"},
		{name: "configured port", listenAddress: "192.168.1.50", openRGBPort: 6743, want: "127.0.0.1:6743"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			address, err := sdkAddressForConfig(config.Configuration{
				ListenAddress: test.listenAddress,
				OpenRGBPort:   test.openRGBPort,
			})
			if err != nil {
				t.Fatalf("sdkAddressForConfig() returned error: %v", err)
			}
			if address != test.want {
				t.Fatalf("sdkAddressForConfig() = %q, want %q", address, test.want)
			}
		})
	}
}

func TestDiscoverControllersNegotiatesProtocolFourAndPreservesNanoleafLayout(t *testing.T) {
	payload := controllerPayload(
		4,
		"Nanoleaf NL22",
		"Nanoleaf",
		"Nanoleaf Shapes",
		"5.3.2",
		"S17422A2735",
		"192.0.2.10:16021",
		[]testZone{{name: "Nanoleaf Layout", ledCount: 15}},
		15,
		[]uint32{0x00112233},
	)
	withFakeSDKServer(t, func(conn net.Conn) {
		negotiated := negotiateTestProtocol(t, conn, 7)
		if negotiated != 4 {
			t.Fatalf("negotiated protocol = %d, want 4", negotiated)
		}
		request := readRequest(t, conn)
		if request.opcode != opcodeRequestControllerCount || len(request.payload) != 0 {
			t.Fatalf("controller count request = %#v", request)
		}
		if err := writeControllerCount(conn, 1); err != nil {
			t.Errorf("write count: %v", err)
			return
		}
		request = readRequest(t, conn)
		if request.controllerID != 0 || request.opcode != opcodeRequestControllerData ||
			len(request.payload) != 4 || binary.LittleEndian.Uint32(request.payload) != 4 {
			t.Fatalf("controller data request = %#v, want protocol 4 payload", request)
		}
		if _, err := conn.Write(responseHeader("ORGB", 0, opcodeRequestControllerData, uint32(len(payload)))); err != nil {
			t.Errorf("write header: %v", err)
			return
		}
		if _, err := conn.Write(payload); err != nil {
			t.Errorf("write payload: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	controllers, err := DiscoverControllersContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(controllers) != 1 {
		t.Fatalf("controllers = %#v", controllers)
	}
	controller := controllers[0]
	if controller.ID != 0 ||
		controller.Name != "Nanoleaf NL22" ||
		controller.Vendor != "Nanoleaf" ||
		controller.Description != "Nanoleaf Shapes" ||
		controller.Version != "5.3.2" ||
		controller.Serial != "S17422A2735" ||
		controller.Location != "192.0.2.10:16021" {
		t.Fatalf("controller metadata = %#v", controller)
	}
	if controller.LEDCount != 15 || len(controller.Zones) != 1 {
		t.Fatalf("controller layout = %#v", controller)
	}
	zone := controller.Zones[0]
	if zone.Name != "Nanoleaf Layout" || zone.LEDCount != 15 {
		t.Fatalf("controller zone = %#v", zone)
	}
}

func TestDiscoverControllersFallsBackToProtocolZero(t *testing.T) {
	payload := controllerPayload(
		0,
		"Legacy Controller",
		"must not be encoded",
		"Legacy Description",
		"0.4",
		"legacy-serial",
		"legacy-location",
		[]testZone{{name: "Legacy Zone", ledCount: 3}},
		3,
		[]uint32{0x00010203},
	)
	withFakeSDKServer(t, func(conn net.Conn) {
		request := readRequest(t, conn)
		if request.opcode != opcodeRequestProtocolVersion {
			t.Fatalf("first request = %#v, want protocol negotiation", request)
		}

		request = readRequest(t, conn)
		if request.opcode != opcodeRequestControllerCount || len(request.payload) != 0 {
			t.Fatalf("controller count request = %#v", request)
		}
		if err := writeControllerCount(conn, 1); err != nil {
			t.Fatalf("write count: %v", err)
		}

		request = readRequest(t, conn)
		if request.opcode != opcodeRequestControllerData || len(request.payload) != 0 {
			t.Fatalf("protocol-0 controller request = %#v, want empty payload", request)
		}
		if _, err := conn.Write(responseHeader("ORGB", 0, opcodeRequestControllerData, uint32(len(payload)))); err != nil {
			t.Fatalf("write controller header: %v", err)
		}
		if _, err := conn.Write(payload); err != nil {
			t.Fatalf("write controller payload: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	controllers, err := DiscoverControllersContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(controllers) != 1 {
		t.Fatalf("controllers = %#v", controllers)
	}
	controller := controllers[0]
	if controller.Name != "Legacy Controller" ||
		controller.Vendor != "" ||
		controller.Description != "Legacy Description" ||
		controller.Version != "0.4" ||
		controller.Serial != "legacy-serial" ||
		controller.Location != "legacy-location" {
		t.Fatalf("protocol-0 metadata = %#v", controller)
	}
	if controller.LEDCount != 3 || len(controller.Zones) != 1 ||
		controller.Zones[0].Name != "Legacy Zone" || controller.Zones[0].LEDCount != 3 {
		t.Fatalf("protocol-0 layout = %#v", controller)
	}
}

func TestParseControllerDataDoesNotPreferFabricatedMultiZoneBytes(t *testing.T) {
	fabricated := new(bytes.Buffer)
	_ = binary.Write(fabricated, binary.LittleEndian, uint16(2))
	for index, ledCount := range []uint32{14, 1} {
		writeTestString(fabricated, "Fabricated Zone")
		_ = binary.Write(fabricated, binary.LittleEndian, int32(index))
		_ = binary.Write(fabricated, binary.LittleEndian, ledCount)
		_ = binary.Write(fabricated, binary.LittleEndian, ledCount)
		_ = binary.Write(fabricated, binary.LittleEndian, ledCount)
		_ = binary.Write(fabricated, binary.LittleEndian, uint16(0))
		_ = binary.Write(fabricated, binary.LittleEndian, uint16(0))
	}
	payload := controllerPayload(
		4,
		"One Zone Controller",
		"Vendor",
		"Description",
		"1.0",
		"serial",
		"location",
		[]testZone{{name: "Real Zone", ledCount: 15}},
		15,
		testColorsFromBytes(fabricated.Bytes()),
	)

	controller, err := parseControllerData(payload, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(controller.Zones) != 1 || controller.Zones[0].Name != "Real Zone" || controller.Zones[0].LEDCount != 15 {
		t.Fatalf("parsed zones = %#v", controller.Zones)
	}
}

func TestDiscoverControllersUsesLowerServerProtocol(t *testing.T) {
	payload := controllerPayload(
		2,
		"Protocol Two Controller",
		"Protocol Two Vendor",
		"Description",
		"2.0",
		"protocol-two-serial",
		"protocol-two-location",
		[]testZone{{name: "Protocol Two Zone", ledCount: 2}},
		2,
		[]uint32{0x00010203},
	)
	withFakeSDKServer(t, func(conn net.Conn) {
		if negotiated := negotiateTestProtocol(t, conn, 2); negotiated != 2 {
			t.Fatalf("negotiated protocol = %d, want 2", negotiated)
		}
		readRequest(t, conn)
		if err := writeControllerCount(conn, 1); err != nil {
			t.Fatalf("write count: %v", err)
		}
		request := readRequest(t, conn)
		if len(request.payload) != 4 || binary.LittleEndian.Uint32(request.payload) != 2 {
			t.Fatalf("controller request = %#v, want protocol 2", request)
		}
		_, _ = conn.Write(responseHeader("ORGB", 0, opcodeRequestControllerData, uint32(len(payload))))
		_, _ = conn.Write(payload)
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	controllers, err := DiscoverControllersContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(controllers) != 1 || controllers[0].Vendor != "Protocol Two Vendor" ||
		len(controllers[0].Zones) != 1 || controllers[0].Zones[0].SegmentCount != 0 {
		t.Fatalf("protocol-2 controller = %#v", controllers)
	}
}

func TestParseControllerDataRejectsMalformedPayloads(t *testing.T) {
	valid := controllerPayload(
		4,
		"Malformed Test",
		"Vendor",
		"Description",
		"4.0",
		"serial",
		"location",
		[]testZone{{
			name:     "Zone",
			ledCount: 1,
			segments: []testSegment{{name: "Segment", ledCount: 1}},
		}},
		1,
		[]uint32{0x00010203},
	)
	withDeclaredSize := func(data []byte, size int) []byte {
		result := append([]byte(nil), data[:size]...)
		if len(result) >= 4 {
			binary.LittleEndian.PutUint32(result[:4], uint32(len(result)))
		}
		return result
	}

	sizeMismatch := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(sizeMismatch[:4], uint32(len(sizeMismatch)+1))
	trailing := append(append([]byte(nil), valid...), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(trailing[:4], uint32(len(trailing)))

	tests := []struct {
		name     string
		payload  []byte
		protocol uint32
	}{
		{name: "short header", payload: []byte{1, 2, 3}, protocol: 4},
		{name: "declared size mismatch", payload: sizeMismatch, protocol: 4},
		{name: "truncated metadata", payload: withDeclaredSize(valid, 12), protocol: 4},
		{name: "truncated mode", payload: withDeclaredSize(valid, len(valid)/2), protocol: 4},
		{name: "truncated segment or LED data", payload: withDeclaredSize(valid, len(valid)-5), protocol: 4},
		{name: "trailing bytes", payload: trailing, protocol: 4},
		{name: "unsupported protocol", payload: valid, protocol: maxSupportedProtocolVersion + 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseControllerData(test.payload, test.protocol); err == nil {
				t.Fatal("malformed controller payload parsed successfully")
			}
		})
	}
}

func TestDiscoverControllersRejectsMalformedResponses(t *testing.T) {
	tests := []struct {
		name    string
		handler func(net.Conn)
	}{
		{
			name: "invalid magic",
			handler: func(conn net.Conn) {
				negotiateTestProtocol(t, conn, 4)
				readRequest(t, conn)
				_, _ = conn.Write(responseHeader("NOPE", 0, opcodeRequestControllerCount, 0))
			},
		},
		{
			name: "truncated header",
			handler: func(conn net.Conn) {
				negotiateTestProtocol(t, conn, 4)
				readRequest(t, conn)
				_, _ = conn.Write([]byte("ORGB"))
			},
		},
		{
			name: "truncated payload",
			handler: func(conn net.Conn) {
				negotiateTestProtocol(t, conn, 4)
				readRequest(t, conn)
				_, _ = conn.Write(responseHeader("ORGB", 0, opcodeRequestControllerCount, 4))
				_, _ = conn.Write([]byte{1, 0})
			},
		},
		{
			name: "oversized payload",
			handler: func(conn net.Conn) {
				negotiateTestProtocol(t, conn, 4)
				readRequest(t, conn)
				_, _ = conn.Write(responseHeader("ORGB", 0, opcodeRequestControllerCount, maxPayloadSize+1))
			},
		},
		{
			name: "excessive controller count",
			handler: func(conn net.Conn) {
				negotiateTestProtocol(t, conn, 4)
				readRequest(t, conn)
				_ = writeControllerCount(conn, maxControllerCount+1)
			},
		},
		{
			name: "wrong controller ID",
			handler: func(conn net.Conn) {
				negotiateTestProtocol(t, conn, 4)
				readRequest(t, conn)
				_, _ = conn.Write(responseHeader("ORGB", 9, opcodeRequestControllerCount, 0))
			},
		},
		{
			name: "wrong opcode",
			handler: func(conn net.Conn) {
				negotiateTestProtocol(t, conn, 4)
				readRequest(t, conn)
				_, _ = conn.Write(responseHeader("ORGB", 0, opcodeRequestControllerData, 0))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withFakeSDKServer(t, test.handler)
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()
			if _, err := DiscoverControllersContext(ctx); err == nil {
				t.Fatal("expected malformed response error")
			}
		})
	}
}

func TestDiscoverControllersRejectsMalformedControllerResponse(t *testing.T) {
	tests := []struct {
		name         string
		controllerID uint32
		opcode       uint32
	}{
		{name: "wrong controller ID", controllerID: 7, opcode: opcodeRequestControllerData},
		{name: "wrong opcode", controllerID: 0, opcode: opcodeRequestControllerCount},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withFakeSDKServer(t, func(conn net.Conn) {
				negotiateTestProtocol(t, conn, 4)
				readRequest(t, conn)
				if err := writeControllerCount(conn, 1); err != nil {
					t.Errorf("write count: %v", err)
					return
				}
				readRequest(t, conn)
				_, _ = conn.Write(responseHeader("ORGB", test.controllerID, test.opcode, 0))
			})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if _, err := DiscoverControllersContext(ctx); err == nil {
				t.Fatal("expected malformed controller response error")
			}
		})
	}
}

func TestDiscoverControllersStalledResponseHonorsContext(t *testing.T) {
	withFakeSDKServer(t, func(conn net.Conn) {
		readRequest(t, conn)
		_, _ = io.Copy(io.Discard, conn)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := DiscoverControllersContext(ctx)
	if err == nil {
		t.Fatal("expected stalled response error")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("stalled discovery took %v", elapsed)
	}
}

func TestSendFrameContextCancellationStopsStalledWrite(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	previousAddress := sdkAddress
	previousDial := dialContext
	sdkAddress = func() (string, error) { return "pipe", nil }
	dialContext = func(context.Context, string, string) (net.Conn, error) { return clientConn, nil }
	t.Cleanup(func() {
		sdkAddress = previousAddress
		dialContext = previousDial
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	customModeRead := make(chan struct{})
	releaseServer := make(chan struct{})
	defer close(releaseServer)
	go func() {
		defer serverConn.Close()
		if _, err := io.ReadFull(serverConn, make([]byte, 16)); err == nil {
			close(customModeRead)
		}
		<-releaseServer
	}()

	statusMarker := errors.New("prior status")
	SetDisconnected(statusMarker)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- SendFrameContext(ctx, 1, []byte{1, 2, 3})
	}()
	select {
	case <-customModeRead:
	case <-time.After(time.Second):
		t.Fatal("custom-mode packet was not written")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("SendFrameContext error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SendFrameContext did not stop after cancellation")
	}
	state, statusErr := GetStatus()
	if state != StateOffline || statusErr != statusMarker {
		t.Fatalf("status = %q, %v; cancellation should not replace prior SDK status", state, statusErr)
	}
}
