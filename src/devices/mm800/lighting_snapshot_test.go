package mm800

import (
	"path/filepath"
	"strconv"
	"testing"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

func TestMM800LightingSnapshotResolvesDefaultSharedEffectWithoutPersistence(t *testing.T) {
	device, runtime := newMM800CanonicalLightingTestDevice(t)

	snapshot, usable := device.LightingSnapshot()
	if !usable || snapshot.ConfiguredEffect != "static" || !snapshot.EffectSupported || !snapshot.HasBrightness || snapshot.Brightness != 100 || snapshot.Customized || snapshot.SingleColorHex != "#00ffff" {
		t.Fatalf("default snapshot = %#v, usable=%t", snapshot, usable)
	}
	if _, found, err := runtime.State.Resolve(device.Serial); err != nil || found {
		t.Fatalf("snapshot materialized canonical state: found=%t err=%v", found, err)
	}
}

func TestMM800LightingSnapshotUsesCanonicalDesiredStateAndSupportedEffects(t *testing.T) {
	device, runtime := newMM800CanonicalLightingTestDevice(t)
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 36}); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Effects.Set(device.Serial, "static", lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      "static",
		SingleColor:   &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 12, Green: 34, Blue: 56}},
	}); err != nil {
		t.Fatal(err)
	}

	snapshot, usable := device.LightingSnapshot()
	if !usable || snapshot.TargetKind != "native" || snapshot.ConfiguredEffect != "static" || !snapshot.EffectSupported || !snapshot.HasBrightness || snapshot.Brightness != 36 || snapshot.SingleColorHex != "#0c2238" || !snapshot.Customized {
		t.Fatalf("canonical snapshot = %#v, usable=%t", snapshot, usable)
	}
	if !mm800SnapshotHasEffect(snapshot.SupportedEffects, "mousepad", "Mousepad") {
		t.Fatalf("MM800 snapshot does not expose Mousepad: %#v", snapshot.SupportedEffects)
	}
	for _, effect := range snapshot.SupportedEffects {
		if effect.ID != "mousepad" && !device.SupportsLightingEffect(effect.ID) {
			t.Fatalf("snapshot exposed unsupported MM800 effect %#v", effect)
		}
	}
}

func TestMM800LightingSnapshotMousepadHasNoSharedEffectSettings(t *testing.T) {
	device, runtime := newMM800CanonicalLightingTestDevice(t)
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{SelectedEffect: "mousepad", Brightness: 64}); err != nil {
		t.Fatal(err)
	}

	snapshot, usable := device.LightingSnapshot()
	if !usable || !snapshot.EffectSupported || snapshot.ConfiguredEffect != "mousepad" || !snapshot.HasBrightness || snapshot.Brightness != 64 || snapshot.Customized || snapshot.PaletteKind != "" || snapshot.HasSpeed || snapshot.SingleColorHex != "" || snapshot.TwoColorStartHex != "" || snapshot.TwoColorEndHex != "" || snapshot.HasTemperature || snapshot.HasGradient || len(snapshot.GradientStops) != 0 {
		t.Fatalf("Mousepad snapshot = %#v, usable=%t", snapshot, usable)
	}
	if !mm800SnapshotHasEffect(snapshot.SupportedEffects, "mousepad", "Mousepad") {
		t.Fatalf("Mousepad not present in snapshot options: %#v", snapshot.SupportedEffects)
	}
}

func TestMM800LightingSnapshotPresentsAuthoredMousepadZonesDeterministically(t *testing.T) {
	device, runtime := newMM800CanonicalLightingTestDevice(t)
	device.DeviceProfile.Mousepad = &Mousepad{Row: map[int]Row{
		2: {Zones: map[int]Zones{15: {Name: "Fifteen", Left: 15, Top: 2, Width: 3, Height: 4, PacketIndex: []int{9}, Color: rgb.Color{Red: 1}}, 2: {Name: "Two", Left: 2, Top: 2, Width: 3, Height: 4, PacketIndex: []int{8}, Color: rgb.Color{Green: 2}}}},
		1: {Zones: map[int]Zones{1: {Name: "One", Left: 1, Top: 1, Width: 3, Height: 4, PacketIndex: []int{7}, Color: rgb.Color{Blue: 3}}}},
	}}
	for index := 3; index <= 14; index++ {
		device.DeviceProfile.Mousepad.Row[2].Zones[index] = Zones{Name: "Zone", Left: index, Top: 2, Width: 3, Height: 4, PacketIndex: []int{index}, Color: rgb.Color{Red: float64(index)}}
	}
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{SelectedEffect: "mousepad", Brightness: 100}); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := device.LightingSnapshot()
	if !ok || snapshot.AuthoredZoneEditor == nil || snapshot.AuthoredZoneEditor.Heading != "Zones" || snapshot.AuthoredZoneEditor.Description != "Select one or more zones, choose a color, then apply it to the selected zones." || snapshot.AuthoredZoneEditor.HasGroups || len(snapshot.AuthoredZoneEditor.Zones) != 15 {
		t.Fatalf("authored snapshot = %#v", snapshot)
	}
	for index, zone := range snapshot.AuthoredZoneEditor.Zones {
		zoneID := index + 1
		if zone.ID != strconv.Itoa(zoneID) || zone.GroupID != "" || zone.GroupLabel != "" || zone.HasGeometry || zone.Left != 0 || zone.Top != 0 || zone.Width != 0 || zone.Height != 0 {
			t.Fatalf("presented zone %d = %#v", zoneID, zone)
		}
	}
	if snapshot.AuthoredZoneEditor.Zones[0].Label != "One" || snapshot.AuthoredZoneEditor.Zones[14].Label != "Fifteen" {
		t.Fatalf("presented labels = %#v", snapshot.AuthoredZoneEditor.Zones)
	}
	snapshot.AuthoredZoneEditor.Zones[0].ColorHex = "#ffffff"
	if len(device.DeviceProfile.Mousepad.Row) != 2 || len(device.DeviceProfile.Mousepad.Row[1].Zones) != 1 || len(device.DeviceProfile.Mousepad.Row[2].Zones) != 14 {
		t.Fatal("snapshot changed Mousepad row structure")
	}
	if device.DeviceProfile.Mousepad.Row[1].Zones[1].Color.Blue != 3 || device.DeviceProfile.Mousepad.Row[1].Zones[1].Left != 1 || device.DeviceProfile.Mousepad.Row[1].Zones[1].Top != 1 || device.DeviceProfile.Mousepad.Row[1].Zones[1].Width != 3 || device.DeviceProfile.Mousepad.Row[1].Zones[1].Height != 4 || device.DeviceProfile.Mousepad.Row[2].Zones[15].Left != 15 || device.DeviceProfile.Mousepad.Row[2].Zones[15].Top != 2 || device.DeviceProfile.Mousepad.Row[2].Zones[15].Width != 3 || device.DeviceProfile.Mousepad.Row[2].Zones[15].Height != 4 {
		t.Fatal("snapshot changed authored profile state")
	}
}

func TestMM800LightingSnapshotOwnershipFlags(t *testing.T) {
	for _, test := range []struct {
		name     string
		cluster  bool
		external bool
	}{
		{name: "cluster", cluster: true},
		{name: "external", external: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			device, _ := newMM800CanonicalLightingTestDevice(t)
			device.DeviceProfile.RGBCluster = test.cluster
			device.DeviceProfile.OpenRGBIntegration = test.external

			snapshot, usable := device.LightingSnapshot()
			if !usable || snapshot.ClusterControlled != test.cluster || snapshot.ExternalControlled != test.external {
				t.Fatalf("ownership snapshot = %#v, usable=%t", snapshot, usable)
			}
		})
	}
}

func TestMM800LightingSnapshotPresentsSharedPaletteShapesAndCopies(t *testing.T) {
	device, runtime := newMM800CanonicalLightingTestDevice(t)
	settings := map[string]lightingsettings.EffectSettings{
		"wave":            {SchemaVersion: lightingsettings.SchemaVersion, EffectID: "wave", Speed: mm800Float64Pointer(4), TwoColor: &lightingsettings.TwoColorSettings{Start: lightingsettings.Color{Red: 1}, End: lightingsettings.Color{Blue: 2}}},
		"cpu-temperature": {SchemaVersion: lightingsettings.SchemaVersion, EffectID: "cpu-temperature", Temperature: &lightingsettings.TemperatureSettings{Low: lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Green: 1}, Celsius: 20}, Middle: lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Red: 2, Green: 2}, Celsius: 50}, High: lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Blue: 3}, Celsius: 90}}},
		"gradient":        {SchemaVersion: lightingsettings.SchemaVersion, EffectID: "gradient", Speed: mm800Float64Pointer(3), Gradient: &lightingsettings.GradientSettings{Stops: []lightingsettings.GradientStop{{Position: 0.2, Color: lightingsettings.Color{Red: 4}, Intensity: 0.4}, {Position: 0.8, Color: lightingsettings.Color{Blue: 5}, Intensity: 0.9}}}},
	}
	for effect, settings := range settings {
		if err := runtime.Effects.Set(device.Serial, effect, settings); err != nil {
			t.Fatal(err)
		}
	}

	for _, test := range []struct {
		effect string
		check  func(LightingSnapshot) bool
	}{
		{"wave", func(value LightingSnapshot) bool {
			return value.HasSpeed && value.Speed == 4 && value.TwoColorStartHex == "#010000" && value.TwoColorEndHex == "#000002" && value.Customized
		}},
		{"cpu-temperature", func(value LightingSnapshot) bool {
			return value.HasTemperature && value.TemperatureLow.Celsius == 20 && value.TemperatureMiddle.ColorHex == "#020200" && value.TemperatureHigh.Celsius == 90 && value.Customized
		}},
		{"gradient", func(value LightingSnapshot) bool {
			return value.HasSpeed && value.HasGradient && len(value.GradientStops) == 2 && value.GradientStops[0].Position == 0.2 && value.GradientStops[0].Intensity == 0.4 && value.GradientStops[1].ColorHex == "#000005" && value.Customized
		}},
		{"rainbow", func(value LightingSnapshot) bool {
			return value.EffectSupported && value.PaletteKind == string(rgb.LightingPaletteGenerated) && !value.HasGradient && value.SingleColorHex == "" && !value.Customized
		}},
	} {
		if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{SelectedEffect: test.effect, Brightness: 70}); err != nil {
			t.Fatal(err)
		}
		snapshot, usable := device.LightingSnapshot()
		if !usable || !test.check(snapshot) {
			t.Fatalf("%s snapshot = %#v, usable=%t", test.effect, snapshot, usable)
		}
		if test.effect == "gradient" {
			snapshot.GradientStops[0].ColorHex = "#ffffff"
			again, _ := device.LightingSnapshot()
			if again.GradientStops[0].ColorHex != "#040000" {
				t.Fatalf("gradient snapshot aliases canonical settings: %#v", again.GradientStops)
			}
		}
	}
}

func newMM800CanonicalLightingTestDevice(t *testing.T) (*Device, *lightingsettings.IndependentDeviceRuntime) {
	t.Helper()
	root := t.TempDir()
	defaultsPath, err := filepath.Abs(filepath.Join("..", "..", "..", "database"))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(
		filepath.Join(root, "independent-device-state.json"),
		filepath.Join(root, "independent-device-effects.json"),
		filepath.Join(defaultsPath, "rgb.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	return &Device{
		Serial:          "mm800-canonical-read",
		DeviceProfile:   &DeviceProfile{},
		lightingRuntime: runtime,
	}, runtime
}

func mm800SnapshotHasEffect(options []LightingEffectOption, id, label string) bool {
	for _, option := range options {
		if option.ID == id && option.Label == label {
			return true
		}
	}
	return false
}

func mm800Float64Pointer(value float64) *float64 {
	return &value
}
