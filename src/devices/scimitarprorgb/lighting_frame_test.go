package scimitarprorgb

import (
	"bytes"
	"testing"

	"LumenForge/src/rgb"
)

const scimitarTestLEDChannels = 5

func TestComposeScimitarLogicalFrame(t *testing.T) {
	data := []byte{
		0x11, 0x12, 0x13, // Front
		0x21, 0x22, 0x23, // Scroll
		0x31, 0x32, 0x33, // Side
		0x41, 0x42, 0x43, // Logo
	}
	want := [][]byte{
		{0x11, 0x12, 0x13},
		{0x21, 0x22, 0x23},
		{0x31, 0x32, 0x33},
		{0x41, 0x42, 0x43},
	}

	frame := composeScimitarLogicalFrame(data)
	if len(frame.zones) != len(want) {
		t.Fatalf("logical zone count = %d, want %d", len(frame.zones), len(want))
	}
	for zone := range want {
		if !bytes.Equal(frame.zones[zone][:], want[zone]) {
			t.Fatalf("logical zone %d = %v, want %v", zone, frame.zones[zone], want[zone])
		}
	}

	data[0] = 0xff
	if frame.zones[0][0] != want[0][0] {
		t.Fatalf("logical frame aliases source data: first channel = %d, want %d", frame.zones[0][0], want[0][0])
	}
}

func TestComposeScimitarZoneColorLogicalFrameWithoutHardwareTopology(t *testing.T) {
	colors := [scimitarEffectZoneCount]*rgb.Color{
		{Red: 200, Green: 100, Blue: 50}, // Front
		{Red: 180, Green: 90, Blue: 30},  // Scroll
		{Red: 160, Green: 80, Blue: 20},  // Side
		{Red: 140, Green: 70, Blue: 10},  // Logo
	}
	logicalFrame := composeScimitarZoneColorLogicalFrame(colors, 50)
	wantLogical := [scimitarEffectZoneCount][scimitarRGBChannelsPerZone]byte{
		{100, 50, 25},
		{90, 45, 15},
		{80, 40, 10},
		{70, 35, 5},
	}
	if logicalFrame.zones != wantLogical {
		t.Fatalf("local logical frame = %v, want %v", logicalFrame.zones, wantLogical)
	}
	for zone, color := range colors {
		if color.Brightness != 0.5 {
			t.Fatalf("zone %d stored brightness = %v, want 0.5", zone, color.Brightness)
		}
	}

	zoneColors, dpi := scimitarTestFrameLayout()
	adapter := newScimitarLightingAdapter(scimitarTestLEDChannels, zoneColors, dpi)
	hardwareFrame := adapter.composeScimitarHardwareFrame(
		logicalFrame,
		rgb.Color{Red: 60, Green: 30, Blue: 3},
	)
	wantHardware := []byte{
		1, 100, 50, 25,
		2, 70, 35, 5,
		3, 60, 30, 3,
		4, 90, 45, 15,
		5, 80, 40, 10,
	}
	if !bytes.Equal(hardwareFrame, wantHardware) {
		t.Fatalf("local hardware frame = %v, want %v", hardwareFrame, wantHardware)
	}
}

func TestComposeScimitarHardwareFrameDoesNotAliasLogicalFrame(t *testing.T) {
	zoneColors, dpi := scimitarTestFrameLayout()
	logicalFrame := composeScimitarLogicalFrame([]byte{
		0x11, 0x12, 0x13,
		0x21, 0x22, 0x23,
		0x31, 0x32, 0x33,
		0x41, 0x42, 0x43,
	})
	adapter := scimitarLightingAdapter{
		ledChannels: scimitarTestLEDChannels,
		zoneColors:  zoneColors,
		dpiLeds:     dpi,
	}

	hardwareFrame := adapter.composeScimitarHardwareFrame(
		logicalFrame,
		rgb.Color{Red: 0x51, Green: 0x52, Blue: 0x53},
	)
	logicalFrame.zones[0][0] = 0xff

	if hardwareFrame[1] != 0x11 {
		t.Fatalf("hardware frame aliases logical frame: Front red = %d, want %d", hardwareFrame[1], 0x11)
	}
}

func TestScimitarLightingOwnershipTransitions(t *testing.T) {
	zoneColors, dpi := scimitarTestFrameLayout()
	logicalColors := []byte{
		0x11, 0x12, 0x13,
		0x21, 0x22, 0x23,
		0x31, 0x32, 0x33,
		0x41, 0x42, 0x43,
	}
	dpiColor := rgb.Color{Red: 0x51, Green: 0x52, Blue: 0x53}
	profile := &DeviceProfile{}

	owner := scimitarLightingOwner(profile)
	if !owner.allowsLocalRendering() {
		t.Fatalf("initial owner = %d, want local rendering allowed", owner)
	}
	localFrame := composeScimitarColorFrame(
		scimitarTestLEDChannels,
		zoneColors,
		dpi,
		dpiColor,
		logicalColors,
	)
	if len(localFrame) == 0 {
		t.Fatal("local rendering produced an empty hardware frame")
	}

	profile.RGBCluster = true
	owner = scimitarLightingOwner(profile)
	if owner != scimitarLightingCluster || owner.allowsLocalRendering() {
		t.Fatalf("RGB Cluster owner = %d, want local rendering blocked", owner)
	}

	profile.RGBCluster = false
	owner = scimitarLightingOwner(profile)
	if !owner.allowsLocalRendering() {
		t.Fatalf("owner after leaving RGB Cluster = %d, want local rendering restored", owner)
	}
	if restored := composeScimitarColorFrame(
		scimitarTestLEDChannels,
		zoneColors,
		dpi,
		dpiColor,
		logicalColors,
	); !bytes.Equal(restored, localFrame) {
		t.Fatalf("local frame after leaving RGB Cluster = %v, want %v", restored, localFrame)
	}
}

func TestComposeScimitarColorFrameLayoutAndLogicalZoneOrder(t *testing.T) {
	zoneColors, dpi := scimitarTestFrameLayout()
	logicalColors := []byte{
		0x11, 0x12, 0x13, // Front
		0x21, 0x22, 0x23, // Scroll
		0x31, 0x32, 0x33, // Side
		0x41, 0x42, 0x43, // Logo
	}

	frame := composeScimitarColorFrame(
		scimitarTestLEDChannels,
		zoneColors,
		dpi,
		rgb.Color{Red: 0x51, Green: 0x52, Blue: 0x53},
		logicalColors,
	)
	want := []byte{
		1, 0x11, 0x12, 0x13, // Front
		2, 0x41, 0x42, 0x43, // Logo
		3, 0x51, 0x52, 0x53, // DPI indicator
		4, 0x21, 0x22, 0x23, // Scroll
		5, 0x31, 0x32, 0x33, // Side
	}

	if len(frame) != scimitarTestLEDChannels*4 {
		t.Fatalf("frame length = %d, want %d bytes for five LED records", len(frame), scimitarTestLEDChannels*4)
	}
	if !bytes.Equal(frame, want) {
		t.Fatalf("frame = %v, want %v", frame, want)
	}
}

func TestComposeScimitarOpenRGBFrameKeepsDPISeparate(t *testing.T) {
	zoneColors, dpi := scimitarTestFrameLayout()
	dpiColor := rgb.Color{Red: 0x61, Green: 0x62, Blue: 0x63}
	firstOpenRGBFrame := []byte{
		1, 2, 3,
		4, 5, 6,
		7, 8, 9,
		10, 11, 12,
	}
	secondOpenRGBFrame := []byte{
		101, 102, 103,
		104, 105, 106,
		107, 108, 109,
		110, 111, 112,
	}

	first := composeScimitarColorFrame(
		scimitarTestLEDChannels,
		zoneColors,
		dpi,
		dpiColor,
		firstOpenRGBFrame,
	)
	second := composeScimitarColorFrame(
		scimitarTestLEDChannels,
		zoneColors,
		dpi,
		dpiColor,
		secondOpenRGBFrame,
	)
	wantDPIRecord := []byte{3, 0x61, 0x62, 0x63}

	if !bytes.Equal(first[8:12], wantDPIRecord) {
		t.Fatalf("first OpenRGB DPI record = %v, want %v", first[8:12], wantDPIRecord)
	}
	if !bytes.Equal(second[8:12], wantDPIRecord) {
		t.Fatalf("second OpenRGB DPI record = %v, want %v", second[8:12], wantDPIRecord)
	}
	if !bytes.Equal(firstOpenRGBFrame, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}) {
		t.Fatalf("OpenRGB source frame was modified: %v", firstOpenRGBFrame)
	}
}

func TestComposeScimitarColorFrameBrightness(t *testing.T) {
	zoneColors, dpi := scimitarTestFrameLayout()
	zoneBaseColors := []rgb.Color{
		{Red: 200, Green: 100, Blue: 50},
		{Red: 180, Green: 90, Blue: 30},
		{Red: 160, Green: 80, Blue: 20},
		{Red: 140, Green: 70, Blue: 10},
	}
	dpiBaseColor := rgb.Color{Red: 120, Green: 60, Blue: 6}

	tests := []struct {
		name       string
		brightness uint8
		want       []byte
	}{
		{
			name:       "zero percent",
			brightness: 0,
			want: []byte{
				1, 0, 0, 0,
				2, 0, 0, 0,
				3, 0, 0, 0,
				4, 0, 0, 0,
				5, 0, 0, 0,
			},
		},
		{
			name:       "fifty percent",
			brightness: 50,
			want: []byte{
				1, 100, 50, 25,
				2, 70, 35, 5,
				3, 60, 30, 3,
				4, 90, 45, 15,
				5, 80, 40, 10,
			},
		},
		{
			name:       "one hundred percent",
			brightness: 100,
			want: []byte{
				1, 200, 100, 50,
				2, 140, 70, 10,
				3, 120, 60, 6,
				4, 180, 90, 30,
				5, 160, 80, 20,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logicalColors := make([]byte, 0, len(zoneBaseColors)*3)
			for _, color := range zoneBaseColors {
				logicalColors = append(logicalColors, scimitarTestBrightnessBytes(color, tt.brightness)...)
			}
			dpiBytes := scimitarTestBrightnessBytes(dpiBaseColor, tt.brightness)

			frame := composeScimitarColorFrame(
				scimitarTestLEDChannels,
				zoneColors,
				dpi,
				rgb.Color{Red: float64(dpiBytes[0]), Green: float64(dpiBytes[1]), Blue: float64(dpiBytes[2])},
				logicalColors,
			)
			if !bytes.Equal(frame, tt.want) {
				t.Fatalf("frame at %d%% brightness = %v, want %v", tt.brightness, frame, tt.want)
			}
		})
	}
}

func TestComposeScimitarClusterFrameDoesNotApplyLocalBrightness(t *testing.T) {
	zoneColors, dpi := scimitarTestFrameLayout()
	clusterFrame := []byte{
		200, 100, 50,
		180, 90, 30,
		160, 80, 20,
		140, 70, 10,
	}
	dpiBytes := scimitarTestBrightnessBytes(rgb.Color{Red: 120, Green: 60, Blue: 6}, 0)

	frame := composeScimitarColorFrame(
		scimitarTestLEDChannels,
		zoneColors,
		dpi,
		rgb.Color{Red: float64(dpiBytes[0]), Green: float64(dpiBytes[1]), Blue: float64(dpiBytes[2])},
		clusterFrame,
	)
	want := []byte{
		1, 200, 100, 50,
		2, 140, 70, 10,
		3, 0, 0, 0,
		4, 180, 90, 30,
		5, 160, 80, 20,
	}

	if !bytes.Equal(frame, want) {
		t.Fatalf("cluster frame at zero local brightness = %v, want unscaled cluster zones %v", frame, want)
	}
}

func scimitarTestFrameLayout() (map[int]ZoneColors, DPIProfile) {
	return map[int]ZoneColors{
			0: {
				Name:             "Front",
				ColorIndex:       []int{1, 2, 3},
				LEDIndex:         1,
				LEDIndexPosition: 0,
			},
			1: {
				Name:             "Scroll",
				ColorIndex:       []int{13, 14, 15},
				LEDIndex:         4,
				LEDIndexPosition: 12,
			},
			2: {
				Name:             "Side",
				ColorIndex:       []int{17, 18, 19},
				LEDIndex:         5,
				LEDIndexPosition: 16,
			},
			3: {
				Name:             "Logo",
				ColorIndex:       []int{5, 6, 7},
				LEDIndex:         2,
				LEDIndexPosition: 4,
			},
		}, DPIProfile{
			Name:             "DPI",
			ColorIndex:       map[int][]int{0: {9, 10, 11}},
			LEDIndex:         3,
			LEDIndexPosition: 8,
		}
}

func scimitarTestBrightnessBytes(color rgb.Color, brightness uint8) []byte {
	color.Brightness = rgb.GetBrightnessValueFloat(brightness)
	modified := rgb.ModifyBrightness(color)
	return []byte{byte(modified.Red), byte(modified.Green), byte(modified.Blue)}
}
