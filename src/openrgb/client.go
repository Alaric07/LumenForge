package openrgb

import (
	"LumenForge/src/config"
	"LumenForge/src/localnetwork"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type ConnectionState string

const (
	StateConnected     ConnectionState = "Connected"
	StateOffline       ConnectionState = "Offline"
	StateNotConfigured ConnectionState = "Not Configured"
)

var (
	statusMutex   sync.RWMutex
	currentStatus ConnectionState = StateOffline
	lastError     error
)

const (
	connectTimeout     = 2 * time.Second
	ioTimeout          = 3 * time.Second
	protocolTimeout    = 1 * time.Second
	operationTimeout   = 10 * time.Second
	maxPayloadSize     = 16 * 1024 * 1024
	maxControllerCount = 1024
	maxLEDCount        = 65535
	maxZoneCount       = 128

	// Protocol 4 is the newest controller-data layout parsed below. Protocol 5
	// adds zone flags, alternate LED names, and controller flags.
	maxSupportedProtocolVersion uint32 = 4
)

var (
	sdkAddress = func() (string, error) {
		return sdkAddressForConfig(config.GetConfig())
	}
	dialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		dialer := &net.Dialer{Timeout: connectTimeout}
		return dialer.DialContext(ctx, network, address)
	}
)

func sdkAddressForConfig(cfg config.Configuration) (string, error) {
	if cfg.OpenRGBPort <= 0 || cfg.OpenRGBPort > 65535 {
		return "", fmt.Errorf("OpenRGB port is not configured")
	}
	return localnetwork.Address(cfg.OpenRGBPort), nil
}

func GetStatus() (ConnectionState, error) {
	statusMutex.RLock()
	defer statusMutex.RUnlock()
	return currentStatus, lastError
}

func setStatus(state ConnectionState, err error) {
	statusMutex.Lock()
	defer statusMutex.Unlock()
	currentStatus = state
	lastError = err
}

// SetNotConfigured marks the SDK client inactive because no imports are configured.
func SetNotConfigured() {
	setStatus(StateNotConfigured, nil)
}

// SetDisconnected records an SDK communication failure.
func SetDisconnected(err error) {
	setStatus(StateOffline, err)
}

// SetConnected records a successful SDK protocol exchange.
func SetConnected() {
	setStatus(StateConnected, nil)
}

const (
	opcodeRequestControllerCount uint32 = 0
	opcodeRequestControllerData  uint32 = 1
	opcodeRequestProtocolVersion uint32 = 40
	opcodeSetCustomMode          uint32 = 1100
	opcodeUpdateLeds             uint32 = 1050
)

type DiscoveredController struct {
	ID            int
	Name          string
	Version       string
	Location      string
	Serial        string
	Vendor        string
	Description   string
	ParsedStrings []string
	LEDCount      int
	Zones         []DiscoveredZone
}

type DiscoveredZone struct {
	Name           string
	Type           int32
	MinLEDCount    int
	MaxLEDCount    int
	LEDCount       int
	SegmentCount   int
	Classification string
}

func classifyZone(name string, ledCount int, minLEDCount int, maxLEDCount int, segmentCount int) string {
	lowerName := strings.ToLower(strings.TrimSpace(name))

	switch {
	case strings.Contains(lowerName, "addressable"):
		return "addressable"
	case strings.Contains(lowerName, "argb"):
		return "addressable"
	case strings.Contains(lowerName, "strip"):
		return "addressable"
	case strings.Contains(lowerName, "mainboard"):
		return "zone-based"
	case strings.Contains(lowerName, "logo"):
		return "zone-based"
	case strings.Contains(lowerName, "backplate"):
		return "zone-based"
	case segmentCount > 0:
		return "addressable"
	case ledCount > 1 && maxLEDCount > 1:
		return "addressable"
	default:
		return "zone-based"
	}
}

func writeHeader(buf *bytes.Buffer, controllerId uint32, opcode uint32, size uint32) error {
	if _, err := buf.WriteString("ORGB"); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, controllerId); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, opcode); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, size); err != nil {
		return err
	}
	return nil
}

func readHeader(conn net.Conn) (uint32, uint32, uint32, error) {
	buf := make([]byte, 16)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return 0, 0, 0, err
	}

	if string(buf[:4]) != "ORGB" {
		return 0, 0, 0, fmt.Errorf("invalid OpenRGB header magic")
	}

	controllerId := binary.LittleEndian.Uint32(buf[4:8])
	opcode := binary.LittleEndian.Uint32(buf[8:12])
	size := binary.LittleEndian.Uint32(buf[12:16])
	if size > maxPayloadSize {
		return 0, 0, 0, fmt.Errorf("OpenRGB payload size %d exceeds limit %d", size, maxPayloadSize)
	}

	return controllerId, opcode, size, nil
}

func readPayload(conn net.Conn, size uint32) ([]byte, error) {
	if size > maxPayloadSize {
		return nil, fmt.Errorf("OpenRGB payload size %d exceeds limit %d", size, maxPayloadSize)
	}
	buf := make([]byte, size)
	_, err := io.ReadFull(conn, buf)
	return buf, err
}

func readResponse(conn net.Conn, expectedControllerID, expectedOpcode uint32) ([]byte, error) {
	controllerID, opcode, size, err := readHeader(conn)
	if err != nil {
		return nil, err
	}
	if controllerID != expectedControllerID {
		return nil, fmt.Errorf("unexpected OpenRGB controller ID %d, expected %d", controllerID, expectedControllerID)
	}
	if opcode != expectedOpcode {
		return nil, fmt.Errorf("unexpected OpenRGB opcode %d, expected %d", opcode, expectedOpcode)
	}
	return readPayload(conn, size)
}

func dial(ctx context.Context) (net.Conn, error) {
	address, err := sdkAddress()
	if err != nil {
		return nil, err
	}
	return dialContext(ctx, "tcp", address)
}

func HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	return HealthCheckContext(ctx)
}

func HealthCheckContext(ctx context.Context) error {
	conn, err := dial(ctx)
	if err != nil {
		SetDisconnected(err)
		return err
	}
	defer conn.Close()
	stopContextWatch := watchContext(ctx, conn)
	defer stopContextWatch()

	packet := new(bytes.Buffer)
	if err := writeHeader(packet, 0, opcodeRequestControllerCount, 0); err != nil {
		setStatus(StateOffline, err)
		return err
	}
	if err := writePacket(ctx, conn, packet.Bytes()); err != nil {
		setStatus(StateOffline, err)
		return err
	}

	if err := setReadDeadline(ctx, conn); err != nil {
		SetDisconnected(err)
		return err
	}
	payload, err := readResponse(conn, 0, opcodeRequestControllerCount)
	if err != nil {
		setStatus(StateOffline, err)
		return err
	}
	if len(payload) != 4 {
		err := fmt.Errorf("invalid controller count payload size %d", len(payload))
		setStatus(StateOffline, err)
		return err
	}
	if count := binary.LittleEndian.Uint32(payload); count > maxControllerCount {
		err := fmt.Errorf("OpenRGB controller count %d exceeds limit %d", count, maxControllerCount)
		setStatus(StateOffline, err)
		return err
	}

	setStatus(StateConnected, nil)
	return nil
}

func deadlineFor(ctx context.Context, limit time.Duration) time.Time {
	deadline := time.Now().Add(limit)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		return ctxDeadline
	}
	return deadline
}

func setReadDeadline(ctx context.Context, conn net.Conn) error {
	return setReadDeadlineFor(ctx, conn, ioTimeout)
}

func setReadDeadlineFor(ctx context.Context, conn net.Conn, limit time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return conn.SetReadDeadline(deadlineFor(ctx, limit))
}

func setWriteDeadline(ctx context.Context, conn net.Conn) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return conn.SetWriteDeadline(deadlineFor(ctx, ioTimeout))
}

func writePacket(ctx context.Context, conn net.Conn, data []byte) error {
	if err := setWriteDeadline(ctx, conn); err != nil {
		return err
	}
	for len(data) > 0 {
		written, err := conn.Write(data)
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

func watchContext(ctx context.Context, conn net.Conn) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.SetDeadline(time.Now())
		case <-done:
		}
	}()
	return func() {
		close(done)
	}
}

func FindControllerIDByNameOrVendor(nameMatch string, vendorMatch string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()

	controllers, err := DiscoverControllersContext(ctx)
	if err != nil {
		return -1, err
	}

	nameMatch = strings.ToLower(nameMatch)
	vendorMatch = strings.ToLower(vendorMatch)
	for _, controller := range controllers {
		nameOK := nameMatch != "" && strings.Contains(strings.ToLower(controller.Name), nameMatch)
		vendorOK := vendorMatch != "" && strings.Contains(strings.ToLower(controller.Vendor), vendorMatch)

		if nameOK || vendorOK {
			return controller.ID, nil
		}
	}

	return -1, fmt.Errorf("no matching OpenRGB controller found")
}

type controllerDataParser struct {
	data     []byte
	offset   int
	protocol uint32
}

func (p *controllerDataParser) require(n int, field string) error {
	if n < 0 || p.offset < 0 || p.offset > len(p.data)-n {
		return fmt.Errorf("%s exceeds controller payload", field)
	}
	return nil
}

func (p *controllerDataParser) readU16(field string) (uint16, error) {
	if err := p.require(2, field); err != nil {
		return 0, err
	}
	value := binary.LittleEndian.Uint16(p.data[p.offset : p.offset+2])
	p.offset += 2
	return value, nil
}

func (p *controllerDataParser) readU32(field string) (uint32, error) {
	if err := p.require(4, field); err != nil {
		return 0, err
	}
	value := binary.LittleEndian.Uint32(p.data[p.offset : p.offset+4])
	p.offset += 4
	return value, nil
}

func (p *controllerDataParser) readString(field string) (string, error) {
	length, err := p.readU16(field + " length")
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", fmt.Errorf("%s is not null-terminated", field)
	}
	if err := p.require(int(length), field); err != nil {
		return "", err
	}
	raw := p.data[p.offset : p.offset+int(length)]
	p.offset += int(length)
	if raw[len(raw)-1] != 0 {
		return "", fmt.Errorf("%s is not null-terminated", field)
	}
	return string(raw[:len(raw)-1]), nil
}

func (p *controllerDataParser) skip(n int, field string) error {
	if err := p.require(n, field); err != nil {
		return err
	}
	p.offset += n
	return nil
}

func (p *controllerDataParser) parseModes() error {
	modeCount, err := p.readU16("mode count")
	if err != nil {
		return err
	}
	if _, err = p.readU32("active mode"); err != nil {
		return err
	}

	for modeIndex := 0; modeIndex < int(modeCount); modeIndex++ {
		prefix := fmt.Sprintf("mode %d", modeIndex)
		if _, err = p.readString(prefix + " name"); err != nil {
			return err
		}
		if err = p.skip(16, prefix+" value, flags, and speed limits"); err != nil {
			return err
		}
		if p.protocol >= 3 {
			if err = p.skip(8, prefix+" brightness limits"); err != nil {
				return err
			}
		}
		if err = p.skip(12, prefix+" color limits and speed"); err != nil {
			return err
		}
		if p.protocol >= 3 {
			if err = p.skip(4, prefix+" brightness"); err != nil {
				return err
			}
		}
		if err = p.skip(8, prefix+" direction and color mode"); err != nil {
			return err
		}
		colorCount, colorErr := p.readU16(prefix + " color count")
		if colorErr != nil {
			return colorErr
		}
		if err = p.skip(int(colorCount)*4, prefix+" colors"); err != nil {
			return err
		}
	}
	return nil
}

func (p *controllerDataParser) parseZones() ([]DiscoveredZone, error) {
	zoneCount, err := p.readU16("zone count")
	if err != nil {
		return nil, err
	}
	if zoneCount > maxZoneCount {
		return nil, fmt.Errorf("zone count %d exceeds limit %d", zoneCount, maxZoneCount)
	}

	zones := make([]DiscoveredZone, 0, zoneCount)
	for zoneIndex := 0; zoneIndex < int(zoneCount); zoneIndex++ {
		prefix := fmt.Sprintf("zone %d", zoneIndex)
		name, readErr := p.readString(prefix + " name")
		if readErr != nil {
			return nil, readErr
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = fmt.Sprintf("Zone %d", zoneIndex+1)
		}
		zoneType, readErr := p.readU32(prefix + " type")
		if readErr != nil {
			return nil, readErr
		}
		minLEDs, readErr := p.readU32(prefix + " minimum LED count")
		if readErr != nil {
			return nil, readErr
		}
		maxLEDs, readErr := p.readU32(prefix + " maximum LED count")
		if readErr != nil {
			return nil, readErr
		}
		ledCount, readErr := p.readU32(prefix + " LED count")
		if readErr != nil {
			return nil, readErr
		}
		if minLEDs > maxLEDCount || maxLEDs > maxLEDCount || ledCount > maxLEDCount {
			return nil, fmt.Errorf("%s LED metadata exceeds limit %d", prefix, maxLEDCount)
		}

		matrixLength, readErr := p.readU16(prefix + " matrix length")
		if readErr != nil {
			return nil, readErr
		}
		if matrixLength != 0 {
			if matrixLength < 8 || (matrixLength-8)%4 != 0 {
				return nil, fmt.Errorf("%s matrix length %d is invalid", prefix, matrixLength)
			}
			height, heightErr := p.readU32(prefix + " matrix height")
			if heightErr != nil {
				return nil, heightErr
			}
			width, widthErr := p.readU32(prefix + " matrix width")
			if widthErr != nil {
				return nil, widthErr
			}
			expectedLength := uint64(8) + uint64(height)*uint64(width)*4
			if expectedLength != uint64(matrixLength) {
				return nil, fmt.Errorf("%s matrix length %d does not match %dx%d dimensions", prefix, matrixLength, width, height)
			}
			if err = p.skip(int(matrixLength)-8, prefix+" matrix data"); err != nil {
				return nil, err
			}
		}

		segmentCount := uint16(0)
		if p.protocol >= 4 {
			segmentCount, readErr = p.readU16(prefix + " segment count")
			if readErr != nil {
				return nil, readErr
			}
			for segmentIndex := 0; segmentIndex < int(segmentCount); segmentIndex++ {
				segmentPrefix := fmt.Sprintf("%s segment %d", prefix, segmentIndex)
				if _, readErr = p.readString(segmentPrefix + " name"); readErr != nil {
					return nil, readErr
				}
				if err = p.skip(12, segmentPrefix+" type, start, and LED count"); err != nil {
					return nil, err
				}
			}
		}

		zones = append(zones, DiscoveredZone{
			Name:           name,
			Type:           int32(zoneType),
			MinLEDCount:    int(minLEDs),
			MaxLEDCount:    int(maxLEDs),
			LEDCount:       int(ledCount),
			SegmentCount:   int(segmentCount),
			Classification: classifyZone(name, int(ledCount), int(minLEDs), int(maxLEDs), int(segmentCount)),
		})
	}
	return zones, nil
}

func (p *controllerDataParser) parseLEDsAndColors() (int, error) {
	ledCount, err := p.readU16("LED count")
	if err != nil {
		return 0, err
	}
	for ledIndex := 0; ledIndex < int(ledCount); ledIndex++ {
		prefix := fmt.Sprintf("LED %d", ledIndex)
		if _, err = p.readString(prefix + " name"); err != nil {
			return 0, err
		}
		if err = p.skip(4, prefix+" value"); err != nil {
			return 0, err
		}
	}

	colorCount, err := p.readU16("controller color count")
	if err != nil {
		return 0, err
	}
	if err = p.skip(int(colorCount)*4, "controller colors"); err != nil {
		return 0, err
	}
	return int(ledCount), nil
}

func parseControllerData(payload []byte, protocol uint32) (DiscoveredController, error) {
	if protocol > maxSupportedProtocolVersion {
		return DiscoveredController{}, fmt.Errorf("unsupported OpenRGB protocol version %d", protocol)
	}

	parser := controllerDataParser{data: payload, protocol: protocol}
	dataSize, err := parser.readU32("controller data size")
	if err != nil {
		return DiscoveredController{}, err
	}
	if int(dataSize) != len(payload) {
		return DiscoveredController{}, fmt.Errorf("controller data size %d does not match payload size %d", dataSize, len(payload))
	}
	if _, err = parser.readU32("controller type"); err != nil {
		return DiscoveredController{}, err
	}

	name, err := parser.readString("controller name")
	if err != nil {
		return DiscoveredController{}, err
	}
	vendor := ""
	if protocol >= 1 {
		vendor, err = parser.readString("controller vendor")
		if err != nil {
			return DiscoveredController{}, err
		}
	}
	description, err := parser.readString("controller description")
	if err != nil {
		return DiscoveredController{}, err
	}
	version, err := parser.readString("controller version")
	if err != nil {
		return DiscoveredController{}, err
	}
	serial, err := parser.readString("controller serial")
	if err != nil {
		return DiscoveredController{}, err
	}
	location, err := parser.readString("controller location")
	if err != nil {
		return DiscoveredController{}, err
	}
	if err = parser.parseModes(); err != nil {
		return DiscoveredController{}, err
	}
	zones, err := parser.parseZones()
	if err != nil {
		return DiscoveredController{}, err
	}
	ledCount, err := parser.parseLEDsAndColors()
	if err != nil {
		return DiscoveredController{}, err
	}
	if parser.offset != len(payload) {
		return DiscoveredController{}, fmt.Errorf("controller payload has %d trailing bytes", len(payload)-parser.offset)
	}

	return DiscoveredController{
		Name:          name,
		Version:       version,
		Location:      location,
		Serial:        serial,
		Vendor:        vendor,
		Description:   description,
		ParsedStrings: []string{name, vendor, description, version, location, serial},
		LEDCount:      ledCount,
		Zones:         zones,
	}, nil
}

func isLegacyASUSMotherboard(name, vendor string) bool {
	n := strings.ToLower(name)
	v := strings.ToLower(vendor)
	return strings.Contains(n, "asus rog strix z890-e gaming wifi") || strings.Contains(v, "asus aura")
}

func isImportableController(name, vendor string, ledCount int) bool {
	if name == "" && vendor == "" {
		return false
	}
	if isLegacyASUSMotherboard(name, vendor) {
		return true
	}

	return true
}

func DiscoverControllers() ([]DiscoveredController, error) {
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	return DiscoverControllersContext(ctx)
}

func DiscoverControllersContext(ctx context.Context) ([]DiscoveredController, error) {
	return discoverControllersContext(ctx, true)
}

// DiscoverControllersStatusNeutralContext performs one bounded SDK discovery
// without changing the process-wide importer connection status.
func DiscoverControllersStatusNeutralContext(ctx context.Context) ([]DiscoveredController, error) {
	return discoverControllersContext(ctx, false)
}

func negotiateProtocolVersion(ctx context.Context, conn net.Conn) (uint32, error) {
	packet := new(bytes.Buffer)
	if err := writeHeader(packet, 0, opcodeRequestProtocolVersion, 4); err != nil {
		return 0, err
	}
	if err := binary.Write(packet, binary.LittleEndian, maxSupportedProtocolVersion); err != nil {
		return 0, err
	}
	if err := writePacket(ctx, conn, packet.Bytes()); err != nil {
		return 0, err
	}
	if err := setReadDeadlineFor(ctx, conn, protocolTimeout); err != nil {
		return 0, err
	}

	payload, err := readResponse(conn, 0, opcodeRequestProtocolVersion)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, ctxErr
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			// Protocol-0 servers intentionally do not answer this request.
			return 0, nil
		}
		return 0, err
	}
	if len(payload) != 4 {
		return 0, fmt.Errorf("invalid protocol version payload size %d", len(payload))
	}

	serverVersion := binary.LittleEndian.Uint32(payload)
	if serverVersion > maxSupportedProtocolVersion {
		return maxSupportedProtocolVersion, nil
	}
	return serverVersion, nil
}

func writeControllerDataRequest(ctx context.Context, conn net.Conn, packet *bytes.Buffer, controllerID, protocol uint32) error {
	packet.Reset()
	payloadSize := uint32(0)
	if protocol > 0 {
		payloadSize = 4
	}
	if err := writeHeader(packet, controllerID, opcodeRequestControllerData, payloadSize); err != nil {
		return err
	}
	if protocol > 0 {
		if err := binary.Write(packet, binary.LittleEndian, protocol); err != nil {
			return err
		}
	}
	return writePacket(ctx, conn, packet.Bytes())
}

func discoverControllersContext(ctx context.Context, updateStatus bool) ([]DiscoveredController, error) {
	recordFailure := func(err error) {
		if updateStatus {
			SetDisconnected(err)
		}
	}

	conn, err := dial(ctx)
	if err != nil {
		recordFailure(err)
		return nil, err
	}
	defer conn.Close()
	stopContextWatch := watchContext(ctx, conn)
	defer stopContextWatch()

	protocol, err := negotiateProtocolVersion(ctx, conn)
	if err != nil {
		recordFailure(err)
		return nil, err
	}

	packet := new(bytes.Buffer)
	if err := writeHeader(packet, 0, opcodeRequestControllerCount, 0); err != nil {
		recordFailure(err)
		return nil, err
	}
	if err := writePacket(ctx, conn, packet.Bytes()); err != nil {
		recordFailure(err)
		return nil, err
	}

	if err := setReadDeadline(ctx, conn); err != nil {
		recordFailure(err)
		return nil, err
	}
	payload, err := readResponse(conn, 0, opcodeRequestControllerCount)
	if err != nil {
		recordFailure(err)
		return nil, err
	}
	if len(payload) != 4 {
		err = fmt.Errorf("invalid controller count payload size %d", len(payload))
		recordFailure(err)
		return nil, err
	}

	count := binary.LittleEndian.Uint32(payload[:4])
	if count > maxControllerCount {
		err = fmt.Errorf("OpenRGB controller count %d exceeds limit %d", count, maxControllerCount)
		recordFailure(err)
		return nil, err
	}
	result := make([]DiscoveredController, 0, count)

	for i := uint32(0); i < count; i++ {
		if err := writeControllerDataRequest(ctx, conn, packet, i, protocol); err != nil {
			recordFailure(err)
			return nil, err
		}

		if err := setReadDeadline(ctx, conn); err != nil {
			recordFailure(err)
			return nil, err
		}
		payload, err = readResponse(conn, i, opcodeRequestControllerData)
		if err != nil {
			recordFailure(err)
			return nil, err
		}
		controller, err := parseControllerData(payload, protocol)
		if err != nil {
			err = fmt.Errorf("invalid OpenRGB controller %d data: %w", i, err)
			recordFailure(err)
			return nil, err
		}
		controller.ID = int(i)

		if !isImportableController(controller.Name, controller.Vendor, controller.LEDCount) {
			continue
		}
		result = append(result, controller)
	}

	if updateStatus {
		setStatus(StateConnected, nil)
	}
	return result, nil
}

func SendColor(controllerId uint32, colorCount int, rgb []byte) error {
	err := SendColorContext(context.Background(), controllerId, colorCount, rgb)
	if err != nil {
		SetDisconnected(err)
	}
	return err
}

// SendColorContext sends a static color and allows the caller to cancel the SDK operation.
func SendColorContext(parent context.Context, controllerId uint32, colorCount int, rgb []byte) error {
	if colorCount < 0 || colorCount > maxLEDCount {
		return fmt.Errorf("invalid OpenRGB LED count %d", colorCount)
	}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	conn, err := dial(ctx)
	if err != nil {
		return outputOperationError(ctx, err)
	}
	defer conn.Close()
	stopContextWatch := watchContext(ctx, conn)
	defer stopContextWatch()

	// Switch device into direct/custom mode
	{
		packet := new(bytes.Buffer)
		if err := writeHeader(packet, controllerId, opcodeSetCustomMode, 0); err != nil {
			return err
		}
		if err := writePacket(ctx, conn, packet.Bytes()); err != nil {
			return outputOperationError(ctx, err)
		}
	}

	packet := new(bytes.Buffer)
	payloadSize := uint32(4 + 2 + colorCount*4)
	if err := writeHeader(packet, controllerId, opcodeUpdateLeds, payloadSize); err != nil {
		return err
	}

	dataSize := payloadSize
	if err := binary.Write(packet, binary.LittleEndian, dataSize); err != nil {
		return err
	}

	if err := binary.Write(packet, binary.LittleEndian, uint16(colorCount)); err != nil {
		return err
	}

	color := []byte{0, 0, 0, 0}
	if len(rgb) >= 3 {
		color[0] = rgb[0]
		color[1] = rgb[1]
		color[2] = rgb[2]
	}

	for i := 0; i < colorCount; i++ {
		if _, err := packet.Write(color); err != nil {
			return err
		}
	}

	err = writePacket(ctx, conn, packet.Bytes())
	return outputOperationError(ctx, err)
}

func SendFrame(controllerId uint32, frame []byte) error {
	err := SendFrameContext(context.Background(), controllerId, frame)
	if err != nil {
		SetDisconnected(err)
	}
	return err
}

// SendFrameContext sends an LED frame and allows the caller to cancel the SDK operation.
func SendFrameContext(parent context.Context, controllerId uint32, frame []byte) error {
	if len(frame)%3 != 0 || len(frame)/3 > maxLEDCount {
		return fmt.Errorf("invalid OpenRGB frame length %d", len(frame))
	}
	ctx, cancel := boundedOperationContext(parent)
	defer cancel()
	conn, err := dial(ctx)
	if err != nil {
		return outputOperationError(ctx, err)
	}
	defer conn.Close()
	stopContextWatch := watchContext(ctx, conn)
	defer stopContextWatch()

	// Switch device into direct/custom mode
	{
		packet := new(bytes.Buffer)
		if err := writeHeader(packet, controllerId, opcodeSetCustomMode, 0); err != nil {
			return err
		}
		if err := writePacket(ctx, conn, packet.Bytes()); err != nil {
			return outputOperationError(ctx, err)
		}
	}

	total := len(frame) / 3
	packet := new(bytes.Buffer)
	payloadSize := uint32(4 + 2 + total*4)
	if err := writeHeader(packet, controllerId, opcodeUpdateLeds, payloadSize); err != nil {
		return err
	}

	dataSize := payloadSize
	if err := binary.Write(packet, binary.LittleEndian, dataSize); err != nil {
		return err
	}

	if err := binary.Write(packet, binary.LittleEndian, uint16(total)); err != nil {
		return err
	}

	for i := 0; i < total; i++ {
		color := []byte{
			frame[i*3],
			frame[i*3+1],
			frame[i*3+2],
			0,
		}

		if _, err := packet.Write(color); err != nil {
			return err
		}
	}

	err = writePacket(ctx, conn, packet.Bytes())
	return outputOperationError(ctx, err)
}

func boundedOperationContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, operationTimeout)
}

func outputOperationError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

func SendSingleLED(controllerId uint32, ledIndex uint32, rgb []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	conn, err := dial(ctx)
	if err != nil {
		SetDisconnected(err)
		return err
	}
	defer conn.Close()
	stopContextWatch := watchContext(ctx, conn)
	defer stopContextWatch()

	// Switch device into direct/custom mode
	{
		packet := new(bytes.Buffer)
		if err := writeHeader(packet, controllerId, opcodeSetCustomMode, 0); err != nil {
			return err
		}
		if err := writePacket(ctx, conn, packet.Bytes()); err != nil {
			SetDisconnected(err)
			return err
		}
	}

	const opcodeUpdateSingleLED uint32 = 1052
	packet := new(bytes.Buffer)
	if err := writeHeader(packet, controllerId, opcodeUpdateSingleLED, 8); err != nil {
		return err
	}

	if err := binary.Write(packet, binary.LittleEndian, ledIndex); err != nil {
		return err
	}

	color := []byte{0, 0, 0, 0}
	if len(rgb) >= 3 {
		color[0] = rgb[0]
		color[1] = rgb[1]
		color[2] = rgb[2]
	}

	if _, err := packet.Write(color); err != nil {
		return err
	}

	err = writePacket(ctx, conn, packet.Bytes())
	if err != nil {
		SetDisconnected(err)
	}
	return err
}

func SendFramePersistent(conn net.Conn, controllerId uint32, frame []byte) (net.Conn, error) {
	if len(frame)%3 != 0 || len(frame)/3 > maxLEDCount {
		return nil, fmt.Errorf("invalid OpenRGB frame length %d", len(frame))
	}
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	var err error
	if conn == nil {
		conn, err = dial(ctx)
		if err != nil {
			SetDisconnected(err)
			return nil, err
		}
		// Switch device into direct/custom mode
		packet := new(bytes.Buffer)
		if err := writeHeader(packet, controllerId, opcodeSetCustomMode, 0); err != nil {
			conn.Close()
			return nil, err
		}
		if err := writePacket(ctx, conn, packet.Bytes()); err != nil {
			conn.Close()
			SetDisconnected(err)
			return nil, err
		}
	}

	total := len(frame) / 3
	packet := new(bytes.Buffer)
	payloadSize := uint32(4 + 2 + total*4)
	if err := writeHeader(packet, controllerId, opcodeUpdateLeds, payloadSize); err != nil {
		conn.Close()
		return nil, err
	}

	dataSize := payloadSize
	if err := binary.Write(packet, binary.LittleEndian, dataSize); err != nil {
		conn.Close()
		return nil, err
	}

	if err := binary.Write(packet, binary.LittleEndian, uint16(total)); err != nil {
		conn.Close()
		return nil, err
	}

	for i := 0; i < total; i++ {
		color := []byte{
			frame[i*3],
			frame[i*3+1],
			frame[i*3+2],
			0,
		}

		if _, err := packet.Write(color); err != nil {
			conn.Close()
			return nil, err
		}
	}

	if err = writePacket(ctx, conn, packet.Bytes()); err != nil {
		conn.Close()
		SetDisconnected(err)
		return nil, err
	}

	return conn, nil
}
