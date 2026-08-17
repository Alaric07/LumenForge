package openrgb

// Package: OpenRGB TCP Target Server
// Author: Nikola Jurkovic
// License: GPL-3.0 or later

import (
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/localnetwork"
	"LumenForge/src/logger"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// Opcodes
const (
	OPCODE_REQUEST_CONTROLLER_COUNT = 0
	OPCODE_REQUEST_CONTROLLER_DATA  = 1
	OPCODE_REQUEST_PROTOCOL_VERSION = 40
	OPCODE_SET_CLIENT_NAME          = 50
	OPCODE_DEVICE_LIST_UPDATED      = 100
	OPCODE_REQUEST_PROFILE_LIST     = 150
	OPCODE_REQUEST_PLUGIN_LIST      = 200
	OPCODE_RGBCONTROLLER_UPDATELEDS = 1050
	OPCODE_UPDATE_MODE              = 1101
)

const (
	headerSize             = 16 // headerSize Header is 4 bytes magic ('ORGB') + 3 × uint32
	protocolVersion uint32 = 4  // protocolVersion OpenRGB clients will ask for this protocol version.
	// Normal local use needs one client; this leaves room for tools without permitting unbounded handlers.
	maxTargetClients  = 16
	maxClientNameSize = 4 * 1024
	maxLEDUpdateSize  = 6 + maxLEDCount*4
)

var (
	debug                = false // Debug mode
	controllers          []*common.OpenRGBController
	mutex                sync.RWMutex
	targetLifecycleMutex sync.Mutex
	currentTargetServer  *targetServer
	targetHeaderTimeout  = ioTimeout
	targetPayloadTimeout = ioTimeout
	targetWriteTimeout   = ioTimeout
	// Keep persistent clients useful while ensuring abandoned connections eventually expire.
	targetIdleTimeout = 5 * time.Minute
)

type targetClient struct {
	conn      net.Conn
	writeMu   sync.Mutex
	closeOnce sync.Once
}

func (c *targetClient) close() {
	c.closeOnce.Do(func() {
		_ = c.conn.Close()
	})
}

func (c *targetClient) writeResponse(deviceID, packetType uint32, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := c.conn.SetWriteDeadline(time.Now().Add(targetWriteTimeout)); err != nil {
		return err
	}

	header := makeHeader(deviceID, packetType, uint32(len(payload)))
	if err := writeAll(c.conn, header); err != nil {
		return err
	}
	return writeAll(c.conn, payload)
}

type targetServer struct {
	mu       sync.Mutex
	listener net.Listener
	clients  map[*targetClient]struct{}
	latest   *targetClient
	closed   bool
	wg       sync.WaitGroup
}

func newTargetServer(listener net.Listener) *targetServer {
	return &targetServer{
		listener: listener,
		clients:  make(map[*targetClient]struct{}),
	}
}

func (s *targetServer) register(conn net.Conn) (*targetClient, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || len(s.clients) >= maxTargetClients {
		return nil, false
	}

	client := &targetClient{conn: conn}
	s.clients[client] = struct{}{}
	s.latest = client
	s.wg.Add(1)
	return client, true
}

func (s *targetServer) unregister(client *targetClient) {
	s.mu.Lock()
	delete(s.clients, client)
	if s.latest == client {
		s.latest = nil
	}
	s.mu.Unlock()
	s.wg.Done()
}

func (s *targetServer) latestClient() *targetClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.latest
}

func (s *targetServer) serve() {
	for {
		accepted, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logger.Log(logger.Fields{"error": err}).Error("Failed to accept connection")
			continue
		}

		client, ok := s.register(accepted)
		if !ok {
			_ = accepted.Close()
			continue
		}
		if debug {
			logger.Log(logger.Fields{"address": accepted.RemoteAddr()}).Info("Accepting connection")
		}
		go func() {
			defer s.unregister(client)
			handleTargetClient(client)
		}()
	}
}

func (s *targetServer) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		s.wg.Wait()
		return
	}
	s.closed = true
	listener := s.listener
	clients := make([]*targetClient, 0, len(s.clients))
	for client := range s.clients {
		clients = append(clients, client)
	}
	s.mu.Unlock()

	if listener != nil {
		_ = listener.Close()
	}
	for _, client := range clients {
		client.close()
	}
	s.wg.Wait()
}

func targetEnabled() bool {
	targetLifecycleMutex.Lock()
	defer targetLifecycleMutex.Unlock()
	return currentTargetServer != nil
}

func notifyTargetClient() {
	targetLifecycleMutex.Lock()
	server := currentTargetServer
	targetLifecycleMutex.Unlock()
	if server == nil {
		return
	}
	if client := server.latestClient(); client != nil {
		_ = client.writeResponse(0, OPCODE_DEVICE_LIST_UPDATED, nil)
	}
}

// ClearDeviceControllers will clear device controller list
func ClearDeviceControllers() {
	if targetEnabled() {
		mutex.Lock()
		defer mutex.Unlock()

		if controllers != nil && len(controllers) > 0 {
			controllers = controllers[:0]
		}
	}
}

// AddDeviceController will add new OpenRGB Controller
func AddDeviceController(controller *common.OpenRGBController) {
	mutex.Lock()
	defer mutex.Unlock()
	controllers = append(controllers, controller)
}

// SendToOpenRGB will notify OpenRGB about device list change
func SendToOpenRGB() {
	if targetEnabled() {
		// Notify connected client about device change.
		notifyTargetClient()
	}
}

// UpdateDeviceController will update existing OpenRGB Controller
func UpdateDeviceController(serial string, ctrl *common.OpenRGBController) {
	if targetEnabled() {
		mutex.Lock()
		for key, controller := range controllers {
			if controller.Serial == serial {
				controllers[key] = ctrl
			}
		}
		mutex.Unlock()

		notifyTargetClient()
	}
}

// RemoveDeviceControllerBySerial removes a controller by its serial
func RemoveDeviceControllerBySerial(serial string) {
	if targetEnabled() {
		mutex.Lock()

		for i, c := range controllers {
			if c.Serial == serial {
				controllers = append(controllers[:i], controllers[i+1:]...)
			}
		}
		mutex.Unlock()

		notifyTargetClient()
	}
}

// GetDeviceController will return existing OpenRGB Controller
func GetDeviceController(serial string) *common.OpenRGBController {
	if targetEnabled() {
		mutex.RLock()
		defer mutex.RUnlock()
		for _, controller := range controllers {
			if controller.Serial == serial {
				return controller
			}
		}
		return nil
	}
	return nil
}

// NotifyControllerChange will notify OpenRGB about controller change
func NotifyControllerChange(serial string) {
	if currentTargetClient() == nil {
		return
	}

	mutex.Lock()
	newControllers := controllers[:0]
	for _, controller := range controllers {
		if controller.Serial != serial {
			newControllers = append(newControllers, controller)
		}
	}
	controllers = newControllers
	mutex.Unlock()

	// Notify connected client about device change.
	notifyTargetClient()
}

func currentTargetClient() *targetClient {
	targetLifecycleMutex.Lock()
	server := currentTargetServer
	targetLifecycleMutex.Unlock()
	if server == nil {
		return nil
	}
	return server.latestClient()
}

// Init will initialize OpenRGB Client Target
func Init() {
	Close()
	cfg := config.GetConfig()
	if cfg.EnableOpenRGBTargetServer {
		debug = cfg.Debug
		address := targetListenAddress(cfg)
		listener, err := net.Listen("tcp", address)
		if err != nil {
			logger.Log(logger.Fields{"error": err, "address": address}).Error("Failed to create listener, OpenRGB target server disabled")
			return
		}

		server := newTargetServer(listener)
		targetLifecycleMutex.Lock()
		currentTargetServer = server
		targetLifecycleMutex.Unlock()

		if debug {
			logger.Log(logger.Fields{"address": address}).Info("OpenRGB-backend listening")
		}
		go server.serve()
	}
}

func targetListenAddress(cfg config.Configuration) string {
	return localnetwork.Address(cfg.OpenRGBPort)
}

// Close will close any active connections and listener
func Close() {
	targetLifecycleMutex.Lock()
	server := currentTargetServer
	currentTargetServer = nil
	targetLifecycleMutex.Unlock()
	if server != nil {
		server.close()
	}
}

// handleConn will handle connections from OpenRGB Client
func handleConn(conn net.Conn) {
	handleTargetClient(&targetClient{conn: conn})
}

func handleTargetClient(client *targetClient) {
	conn := client.conn
	defer func() {
		if debug {
			logger.Log(logger.Fields{"address": conn.RemoteAddr()}).Info("Closing connection")
		}
		client.close()
	}()

	var clientName = "lumenforge"
	for {
		header, err := readTargetHeader(conn)
		if err != nil {
			if err == io.EOF {
				return
			}
			if debug {
				logger.Log(logger.Fields{"error": err}).Error("Failed to read header")
			}
			return
		}

		if bytes.Equal(header, make([]byte, headerSize)) {
			if debug {
				logger.Log(logger.Fields{}).Info("Header is zero. (client disconnected)")
			}
			return
		}

		// Validate magic
		if string(header[0:4]) != "ORGB" {
			if debug {
				logger.Log(logger.Fields{"magic": header[0:4], "source": conn.RemoteAddr()}).Error("Bad magic payload received")
			}
			return
		}

		// Parse header fields (little endian)
		deviceID := binary.LittleEndian.Uint32(header[4:8])
		packetType := binary.LittleEndian.Uint32(header[8:12])
		packetSize := binary.LittleEndian.Uint32(header[12:16])

		if debug {
			logger.Log(logger.Fields{"deviceID": deviceID, "pktType": packetType, "pktSize": packetSize}).Info("header packet received")
		}

		if !validTargetPacketSize(packetType, packetSize) {
			if debug {
				logger.Log(logger.Fields{"pktType": packetType, "pktSize": packetSize}).Warn("Invalid OpenRGB target payload size")
			}
			return
		}

		// The protocol-wide limit is checked before conversion and allocation.
		payload := make([]byte, int(packetSize))
		if packetSize > 0 {
			if err = conn.SetReadDeadline(time.Now().Add(targetPayloadTimeout)); err != nil {
				return
			}
			if _, err = io.ReadFull(conn, payload); err != nil {
				if debug {
					logger.Log(logger.Fields{"error": err}).Error("Failed to read payload")
				}
				return
			}
		}

		// Packet processing
		switch packetType {
		case OPCODE_SET_CLIENT_NAME:
			// set client name
			clientName = strings.TrimRight(string(payload), "\x00")
			if debug {
				logger.Log(logger.Fields{"clientName": clientName}).Info("Setting client name")
			}
			//sendHeader(conn, 0, OPCODE_DEVICE_LIST_UPDATED, 0)
		case OPCODE_REQUEST_PROTOCOL_VERSION:
			// send protocol version
			buf := make([]byte, 4)
			binary.LittleEndian.PutUint32(buf, protocolVersion)
			if err = client.writeResponse(0, OPCODE_REQUEST_PROTOCOL_VERSION, buf); err != nil {
				if debug {
					logger.Log(logger.Fields{"error": err}).Error("Write protocol version failed")
				}
				return
			}
			if debug {
				logger.Log(logger.Fields{"protocolVersion": protocolVersion, "clientName": clientName}).Info("sent protocol version")
			}
		case OPCODE_REQUEST_CONTROLLER_COUNT:
			// Send controller count
			mutex.RLock()
			count := uint32(len(controllers))
			mutex.RUnlock()
			b := make([]byte, 4)
			binary.LittleEndian.PutUint32(b, count)
			if err = client.writeResponse(0, OPCODE_REQUEST_CONTROLLER_COUNT, b); err != nil {
				if debug {
					logger.Log(logger.Fields{"error": err}).Error("Write controller count failed")
				}
				return
			}
			if debug {
				logger.Log(logger.Fields{"count": count}).Info("sent controller count")
			}
		case OPCODE_REQUEST_CONTROLLER_DATA:
			// OpenRGB asks for controller data for the deviceID in the header.
			mutex.RLock()
			valid := int(deviceID) >= 0 && int(deviceID) < len(controllers)
			mutex.RUnlock()
			if !valid {
				if debug {
					logger.Log(logger.Fields{"deviceID": deviceID}).Error("Invalid deviceID requested")
				}
				if err = client.writeResponse(deviceID, OPCODE_REQUEST_CONTROLLER_DATA, nil); err != nil {
					return
				}
				continue
			}
			payload = buildDeviceDataPayload(deviceID)
			if err = client.writeResponse(deviceID, OPCODE_REQUEST_CONTROLLER_DATA, payload); err != nil {
				if debug {
					logger.Log(logger.Fields{"error": err}).Error("Write controller data failed")
				}
				return
			}
			if debug {
				logger.Log(logger.Fields{"deviceId": deviceID, "len": len(payload)}).Info("Sent controller data for device")
			}
		case OPCODE_RGBCONTROLLER_UPDATELEDS:
			if debug {
				logger.Log(logger.Fields{"deviceId": deviceID, "payloadLen": len(payload)}).Info("Received OPCODE_RGBCONTROLLER_UPDATELEDS for device")
			}
			if len(payload) < 6 {
				if debug {
					logger.Log(logger.Fields{}).Warn("payload too small for updateleds")
				}
				continue
			}

			// First 4 bytes: total payload size (optional to check)
			_ = binary.LittleEndian.Uint32(payload[0:4])
			ledCount := binary.LittleEndian.Uint16(payload[4:6])

			expectedSize := 4 + 2 + int(ledCount)*4
			if expectedSize != len(payload) {
				if debug {
					logger.Log(logger.Fields{"expected": expectedSize, "got": len(payload)}).Warn("payload size mismatch")
				}
				continue
			}

			// Now parse LED colors
			ledsData := payload[6:]
			var buffer []byte
			for i := 0; i < int(ledCount); i++ {
				base := i * 4
				r := ledsData[base]
				g := ledsData[base+1]
				b := ledsData[base+2]
				buffer = append(buffer, r, g, b)
			}
			// Send it
			mutex.Lock()
			if int(deviceID) >= len(controllers) {
				// Slipstream devices going to sleep mode, or just powered off
				mutex.Unlock()
				return
			}

			ctrl := controllers[int(deviceID)]
			mutex.Unlock()
			ctrl.WriteColorEx(buffer, ctrl.ChannelId)
		case OPCODE_UPDATE_MODE:
			// Mode change request. Payload commonly contains mode index (uint32) as first bytes.
			mutex.Lock()
			if int(deviceID) >= 0 && int(deviceID) < len(controllers) && len(payload) >= 4 {
				modeIndex := int32(binary.LittleEndian.Uint32(payload[0:4])) // This is not used...
				controllers[deviceID].ActiveMode = 0
				if debug {
					logger.Log(logger.Fields{"deviceId": deviceID, "modeIndex": modeIndex}).Info("device mode changed")
				}
			} else {
				if debug {
					logger.Log(logger.Fields{"deviceId": deviceID, "payloadLen": len(payload)}).Error("UPDATE_MODE for invalid device or small payload")
				}
			}
			mutex.Unlock()
		case OPCODE_REQUEST_PROFILE_LIST:
			buf := make([]byte, 8)
			binary.LittleEndian.PutUint32(buf[0:4], uint32(len(buf)))
			binary.LittleEndian.PutUint32(buf[4:8], 0)
			if err = client.writeResponse(0, OPCODE_REQUEST_PROFILE_LIST, buf); err != nil {
				if debug {
					logger.Log(logger.Fields{"error": err}).Error("Failed to send PROFILE LIST")
				}
				return
			}
		case OPCODE_REQUEST_PLUGIN_LIST:
			buf := make([]byte, 8)
			binary.LittleEndian.PutUint32(buf[0:4], uint32(len(buf)))
			binary.LittleEndian.PutUint32(buf[4:8], 0)
			if err = client.writeResponse(0, OPCODE_REQUEST_PLUGIN_LIST, buf); err != nil {
				if debug {
					logger.Log(logger.Fields{"error": err}).Error("Failed to send PLUGIN LIST")
				}
				return
			}
		}
	}
}

func readTargetHeader(conn net.Conn) ([]byte, error) {
	header := make([]byte, headerSize)
	if err := conn.SetReadDeadline(time.Now().Add(targetIdleTimeout)); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(conn, header[:1]); err != nil {
		return nil, err
	}
	if err := conn.SetReadDeadline(time.Now().Add(targetHeaderTimeout)); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(conn, header[1:]); err != nil {
		return nil, err
	}
	return header, nil
}

func validTargetPayloadSize(packetType, packetSize uint32) bool {
	switch packetType {
	case OPCODE_REQUEST_CONTROLLER_COUNT, OPCODE_REQUEST_PROFILE_LIST, OPCODE_REQUEST_PLUGIN_LIST:
		return packetSize == 0
	case OPCODE_REQUEST_CONTROLLER_DATA, OPCODE_REQUEST_PROTOCOL_VERSION:
		return packetSize == 0 || packetSize == 4
	case OPCODE_SET_CLIENT_NAME:
		return packetSize <= maxClientNameSize
	case OPCODE_RGBCONTROLLER_UPDATELEDS:
		return packetSize >= 6 && packetSize <= maxLEDUpdateSize
	case OPCODE_UPDATE_MODE:
		return packetSize >= 4
	default:
		return true
	}
}

func validTargetPacketSize(packetType, packetSize uint32) bool {
	return packetSize <= maxPayloadSize && validTargetPayloadSize(packetType, packetSize)
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := w.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrUnexpectedEOF
		}
		data = data[written:]
	}
	return nil
}

func makeHeader(deviceID, packetType, packetSize uint32) []byte {
	header := make([]byte, headerSize)
	copy(header[0:4], "ORGB")
	binary.LittleEndian.PutUint32(header[4:8], deviceID)
	binary.LittleEndian.PutUint32(header[8:12], packetType)
	binary.LittleEndian.PutUint32(header[12:16], packetSize)
	return header
}

func buildDeviceDataPayload(deviceID uint32) []byte {
	// Get controller
	mutex.RLock()
	if int(deviceID) < 0 || int(deviceID) >= len(controllers) {
		mutex.RUnlock()
		return []byte{}
	}
	ctrl := controllers[deviceID]
	mutex.RUnlock()

	if debug {
		logger.Log(logger.Fields{"controllerId": deviceID, "controllerData": ctrl}).Error("Requesting controller data")
	}

	deviceType := ctrl.DeviceType
	activeMode := ctrl.ActiveMode

	zoneCount := len(ctrl.Zones)
	totalLEDs := totalLED(ctrl)

	// Build it
	controllerBuf := new(bytes.Buffer)

	// helper to write "pack_string" (uint16 len+1, bytes, trailing NUL)
	writeString := func(b *bytes.Buffer, s string) {
		sb := []byte(s)
		_ = binary.Write(b, binary.LittleEndian, uint16(len(sb)+1))
		b.Write(sb)
		b.WriteByte(0)
	}

	// device_type (int32)
	err := binary.Write(controllerBuf, binary.LittleEndian, int32(deviceType))
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("controllerBuf::deviceType write error")
		}
		return []byte{}
	}

	// name
	writeString(controllerBuf, ctrl.Name)

	// metadata.pack(version): vendor (if version>=1), description, fwVersion, serial, location
	writeString(controllerBuf, ctrl.Vendor)

	writeString(controllerBuf, ctrl.Description)
	writeString(controllerBuf, ctrl.FwVersion)
	writeString(controllerBuf, ctrl.Serial)
	writeString(controllerBuf, ctrl.Location)

	// 4) number of modes (uint16) and active_mode (int32)
	err = binary.Write(controllerBuf, binary.LittleEndian, uint16(1)) // one mode
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("controllerBuf::modes write error")
		}
		return []byte{}
	}

	err = binary.Write(controllerBuf, binary.LittleEndian, activeMode)
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("controllerBuf::activeMode write error")
		}
		return []byte{}
	}

	const ModeFlagHasPerLedColor = 1 << 5
	modeBuf := new(bytes.Buffer)
	err = binary.Write(modeBuf, binary.LittleEndian, int32(0)) // id
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("controllerBuf::id write error")
		}
		return []byte{}
	}

	writeString(modeBuf, "Direct")                             // Direct
	err = binary.Write(modeBuf, binary.LittleEndian, int32(0)) // value
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeBuf::value write error")
		}
		return []byte{}
	}
	err = binary.Write(modeBuf, binary.LittleEndian, uint32(ModeFlagHasPerLedColor)) // flags
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeBuf::flags write error")
		}
		return []byte{}
	}
	err = binary.Write(modeBuf, binary.LittleEndian, uint32(0)) // speed_min
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeBuf::speed_min write error")
		}
		return []byte{}
	}
	err = binary.Write(modeBuf, binary.LittleEndian, uint32(0)) // speed_max
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeBuf::speed_max write error")
		}
		return []byte{}
	}
	err = binary.Write(modeBuf, binary.LittleEndian, uint32(0)) // brightness_min
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeBuf::brightness_min write error")
		}
		return []byte{}
	}
	err = binary.Write(modeBuf, binary.LittleEndian, uint32(0)) // brightness_max
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeBuf::brightness_max write error")
		}
		return []byte{}
	}
	err = binary.Write(modeBuf, binary.LittleEndian, uint32(0)) // colors_min
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeBuf::colors_min write error")
		}
		return []byte{}
	}
	err = binary.Write(modeBuf, binary.LittleEndian, uint32(0)) // colors_max
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeBuf::colors_max write error")
		}
		return []byte{}
	}
	err = binary.Write(modeBuf, binary.LittleEndian, uint32(0)) // speed
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeBuf::speed write error")
		}
		return []byte{}
	}
	err = binary.Write(modeBuf, binary.LittleEndian, uint32(0)) // brightness
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeBuf::brightness write error")
		}
		return []byte{}
	}
	err = binary.Write(modeBuf, binary.LittleEndian, uint32(0)) // direction
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeBuf::direction write error")
		}
		return []byte{}
	}
	err = binary.Write(modeBuf, binary.LittleEndian, ctrl.ColorMode) // color_mode
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeBuf::color_mode write error")
		}
		return []byte{}
	}
	err = binary.Write(modeBuf, binary.LittleEndian, uint16(0)) // no mode-specific colors
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeBuf::mode-specific write error")
		}
		return []byte{}
	}

	// prefix mode data with its size (mode.pack behavior)
	modeData := new(bytes.Buffer)
	sizeForMode := uint32(modeBuf.Len() + 4)
	err = binary.Write(modeData, binary.LittleEndian, sizeForMode)
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("modeData::sizeForMode write error")
		}
		return []byte{}
	}

	modeData.Write(modeBuf.Bytes())

	// ControllerData.pack appends mode.pack()[struct.calcsize("Ii"):] (skip 8 bytes)
	modeBytes := modeData.Bytes()
	if len(modeBytes) > 8 {
		controllerBuf.Write(modeBytes[8:])
	}

	// zones: pack_list (uint16 count + each zone.pack)
	zonesPacked := new(bytes.Buffer)
	err = binary.Write(zonesPacked, binary.LittleEndian, uint16(zoneCount))
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("zonesPacked::zoneCount write error")
		}
		return []byte{}
	}

	for z := 0; z < zoneCount; z++ {
		zoneName := ctrl.Zones[z].Name
		writeString(zonesPacked, zoneName)
		err = binary.Write(zonesPacked, binary.LittleEndian, int32(ctrl.Zones[z].ZoneType))
		if err != nil {
			if debug {
				logger.Log(logger.Fields{"error": err}).Error("zonesPacked::mode write error")
			}
			return []byte{}
		}

		err = binary.Write(zonesPacked, binary.LittleEndian, ctrl.Zones[z].NumLEDs) // leds_min
		if err != nil {
			if debug {
				logger.Log(logger.Fields{"error": err}).Error("zonesPacked::leds_min write error")
			}
			return []byte{}
		}

		err = binary.Write(zonesPacked, binary.LittleEndian, ctrl.Zones[z].NumLEDs) // leds_max
		if err != nil {
			if debug {
				logger.Log(logger.Fields{"error": err}).Error("zonesPacked::leds_max write error")
			}
			return []byte{}
		}

		err = binary.Write(zonesPacked, binary.LittleEndian, ctrl.Zones[z].NumLEDs) // num_leds
		if err != nil {
			if debug {
				logger.Log(logger.Fields{"error": err}).Error("zonesPacked::num_leds write error")
			}
			return []byte{}
		}

		if ctrl.Zones[z].ZoneType == common.ZoneTypeMatrix {
			matrix, ok := common.MatrixMaps[ctrl.Zones[z].NumLEDs]
			if !ok || len(matrix) == 0 || len(matrix[0]) == 0 {
				err = binary.Write(zonesPacked, binary.LittleEndian, uint16(0)) // matrix size = 0
				if err != nil {
					if debug {
						logger.Log(logger.Fields{"error": err}).Error("zonesPacked::matrix size write error")
					}
					return []byte{}
				}
			} else {
				height := uint32(len(matrix))
				width := uint32(len(matrix[0]))

				// byte length of matrix payload (entries + height + width)
				matrixLen := width*height*4 + 8
				if matrixLen > 0xFFFF {
					if debug {
						logger.Log(logger.Fields{"matrixLen": matrixLen, "zone": ctrl.Zones[z].Name}).Error("matrix too large for uint16 length")
					}
					return []byte{}
				}

				// write length (uint16, in BYTES), then height & width (uint32 each)
				if err = binary.Write(zonesPacked, binary.LittleEndian, uint16(matrixLen)); err != nil {
					if debug {
						logger.Log(logger.Fields{"error": err}).Error("zonesPacked::matrixLen write error")
					}
					return []byte{}
				}
				if err = binary.Write(zonesPacked, binary.LittleEndian, height); err != nil {
					if debug {
						logger.Log(logger.Fields{"error": err}).Error("zonesPacked::matrixHeight write error")
					}
					return []byte{}
				}
				if err = binary.Write(zonesPacked, binary.LittleEndian, width); err != nil {
					if debug {
						logger.Log(logger.Fields{"error": err}).Error("zonesPacked::matrixWidth write error")
					}
					return []byte{}
				}

				// write map data row-major (y, then x)
				for y := 0; y < int(height); y++ {
					if len(matrix[y]) != int(width) {
						if debug {
							logger.Log(logger.Fields{"zone": ctrl.Zones[z].Name, "row": y}).Error("matrix row width mismatch")
						}
						return []byte{}
					}
					for x := 0; x < int(width); x++ {
						if err = binary.Write(zonesPacked, binary.LittleEndian, matrix[y][x]); err != nil {
							if debug {
								logger.Log(logger.Fields{"error": err}).Error("zonesPacked::matrix write error")
							}
							return []byte{}
						}
					}
				}
			}
		} else {
			err = binary.Write(zonesPacked, binary.LittleEndian, uint16(0)) // matrix size = 0
			if err != nil {
				if debug {
					logger.Log(logger.Fields{"error": err}).Error("zonesPacked::matrix size write error")
				}
				return []byte{}
			}
		}

		segCount := len(ctrl.Zones[z].Segments)
		err = binary.Write(zonesPacked, binary.LittleEndian, uint16(segCount)) // count segments
		if err != nil {
			if debug {
				logger.Log(logger.Fields{"error": err}).Error("zonesPacked::segments write error")
			}
			return []byte{}
		}

		for s := 0; s < segCount; s++ {
			seg := ctrl.Zones[z].Segments[s]

			// segment_name
			writeString(zonesPacked, seg.Name)

			// segment_type
			err = binary.Write(zonesPacked, binary.LittleEndian, seg.Type)
			if err != nil {
				if debug {
					logger.Log(logger.Fields{"error": err}).Error("zonesPacked::segment_type write error")
				}
				return []byte{}
			}

			// segment_start_idx
			err = binary.Write(zonesPacked, binary.LittleEndian, seg.StartIdx)
			if err != nil {
				if debug {
					logger.Log(logger.Fields{"error": err}).Error("zonesPacked::segment_start_idx write error")
				}
				return []byte{}
			}

			// segment_leds_count
			err = binary.Write(zonesPacked, binary.LittleEndian, seg.LedCount)
			if err != nil {
				if debug {
					logger.Log(logger.Fields{"error": err}).Error("zonesPacked::segment_leds_count write error")
				}
				return []byte{}
			}
		}
	}
	controllerBuf.Write(zonesPacked.Bytes())

	// LEDs: pack_list uint16 count then each LED pack_string + uint32 value(index)
	ledsPacked := new(bytes.Buffer)
	err = binary.Write(ledsPacked, binary.LittleEndian, uint16(totalLEDs))
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("ledsPacked::pack_list led count write error")
		}
		return []byte{}
	}

	ledIndex := uint32(0)
	for z := 0; z < zoneCount; z++ {
		for l := uint32(0); l < ctrl.Zones[z].NumLEDs; l++ {
			writeString(ledsPacked, fmt.Sprintf("%s LED %d", ctrl.Zones[z].Name, l))
			err = binary.Write(ledsPacked, binary.LittleEndian, ledIndex)
			if err != nil {
				if debug {
					logger.Log(logger.Fields{"error": err}).Error("ledsPacked::ledIndex write error")
				}
				return []byte{}
			}
			ledIndex++
		}
	}
	controllerBuf.Write(ledsPacked.Bytes())

	// Colors: pack_list uint16 count then RGBColor.pack (BBBx) for each LED
	colorsPacked := new(bytes.Buffer)
	err = binary.Write(colorsPacked, binary.LittleEndian, uint16(totalLEDs))
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("colorsPacked::totalLEDs write error")
		}
		return []byte{}
	}

	// read colors from controller state (3 bytes per LED) and write r,g,b, padding
	mutex.RLock()
	for i := uint32(0); i < totalLEDs; i++ {
		idx := i * 3
		var r, g, b byte
		if idx+2 < uint32(len(ctrl.Colors)) {
			r = ctrl.Colors[idx+0]
			g = ctrl.Colors[idx+1]
			b = ctrl.Colors[idx+2]
		}
		colorsPacked.WriteByte(r)
		colorsPacked.WriteByte(g)
		colorsPacked.WriteByte(b)
		colorsPacked.WriteByte(0) // padding
	}
	mutex.RUnlock()
	controllerBuf.Write(colorsPacked.Bytes())

	// Final length prefix: ControllerData.pack prefixes its payload with uint32(len + 4)
	finalController := new(bytes.Buffer)
	totalSize := uint32(controllerBuf.Len() + 4)
	err = binary.Write(finalController, binary.LittleEndian, totalSize)
	if err != nil {
		if debug {
			logger.Log(logger.Fields{"error": err}).Error("finalController::totalSize write error")
		}
		return []byte{}
	}

	finalController.Write(controllerBuf.Bytes())

	// Send it
	return finalController.Bytes()
}

// totalLED returns sum of NumLEDs across zones
func totalLED(ctrl *common.OpenRGBController) uint32 {
	var total uint32
	for _, z := range ctrl.Zones {
		total += z.NumLEDs
	}
	return total
}
