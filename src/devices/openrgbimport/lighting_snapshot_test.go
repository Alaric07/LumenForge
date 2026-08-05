package openrgbimport

import (
	"LumenForge/src/rgb"
	"reflect"
	"sync"
	"testing"
)

func lightingTestColor(red, green, blue float64) rgb.Color {
	return rgb.Color{Red: red, Green: green, Blue: blue, Brightness: 1}
}

func lightingTestDevice(effect string, modes []string, definitions map[string]rgb.Profile) *Device {
	brightness := uint8(50)
	device := &Device{
		Serial:    "openrgb-lighting-snapshot-test",
		IsOpenRGB: true,
		RGBModes:  append([]string(nil), modes...),
		DeviceProfile: &DeviceProfile{
			Active:           true,
			RGBProfile:       effect,
			BrightnessSlider: &brightness,
		},
		Rgb: &rgb.RGB{Profiles: definitions},
	}
	attachTestLightingRuntime(device)
	device.effect = effect
	device.brightness = brightness
	for effectID, definition := range definitions {
		settings, err := lightingSettingsFromRGBProfile(effectID, definition)
		if err == nil {
			if err = device.lightingEffects.Set(device.Serial, effectID, settings); err != nil {
				panic(err)
			}
		}
	}
	return device
}

func TestOpenRGBLightingSnapshotNilAndInactive(t *testing.T) {
	var nilDevice *Device
	if snapshot, ok := nilDevice.LightingSnapshot(); ok || !reflect.DeepEqual(snapshot, LightingSnapshot{}) {
		t.Fatalf("nil receiver returned %+v, %t", snapshot, ok)
	}

	for _, device := range []*Device{
		{},
		{IsOpenRGB: true, lifecycleDetached: true},
		{IsOpenRGB: true, lifecycleActivating: true},
	} {
		if snapshot, ok := device.LightingSnapshot(); ok || !reflect.DeepEqual(snapshot, LightingSnapshot{}) {
			t.Fatalf("inactive device returned %+v, %t", snapshot, ok)
		}
	}
}

func TestOpenRGBLightingSnapshotConfiguredState(t *testing.T) {
	start := lightingTestColor(10, 20, 30)
	middle := lightingTestColor(40, 50, 60)
	end := lightingTestColor(70, 80, 90)
	device := lightingTestDevice("colorpulse", []string{"static", "colorpulse", "off"}, map[string]rgb.Profile{
		"static":     {ProfileName: "Static", StartColor: start},
		"colorpulse": {ProfileName: "Color Pulse", StartColor: start, MiddleColor: middle, EndColor: end, Speed: 3},
		"off":        {},
	})
	device.DeviceProfile.RGBCluster = true
	device.DeviceProfile.RGBOverride = &RGBOverride{
		Enabled:        true,
		RGBStartColor:  lightingTestColor(1, 2, 3),
		RGBMiddleColor: lightingTestColor(4, 5, 6),
		RGBEndColor:    lightingTestColor(7, 8, 9),
		RgbModeSpeed:   0,
	}

	snapshot, ok := device.LightingSnapshot()
	if !ok {
		t.Fatal("LightingSnapshot reported an active OpenRGB device as unavailable")
	}
	if snapshot.ConfiguredEffect != "colorpulse" || !snapshot.EffectSupported {
		t.Fatalf("configured state = %+v", snapshot)
	}
	if !snapshot.HasBrightness || snapshot.Brightness != 50 || !snapshot.ClusterControlled {
		t.Fatalf("profile state = %+v", snapshot)
	}
	wantOptions := []LightingEffectOption{
		{ID: "static", Label: "Static"},
		{ID: "colorpulse", Label: "Color Pulse"},
		{ID: "off", Label: "Off"},
	}
	if !reflect.DeepEqual(snapshot.SupportedEffects, wantOptions) {
		t.Fatalf("supported effects = %#v, want %#v", snapshot.SupportedEffects, wantOptions)
	}
	if !snapshot.HasSpeed || snapshot.Speed != 3 || snapshot.PaletteKind != "two-color" || !snapshot.Customized {
		t.Fatalf("base properties = %+v", snapshot)
	}
}

func TestOpenRGBLightingSnapshotSupportedCatalogCapabilities(t *testing.T) {
	catalogue := importerSoftwareEffectCatalogue()
	definitions := make(map[string]rgb.Profile, len(catalogue))
	for _, effect := range catalogue {
		definitions[effect] = rgb.Profile{ProfileName: effect}
	}
	device := lightingTestDevice("static", catalogue, definitions)

	snapshot, ok := device.LightingSnapshot()
	if !ok || len(snapshot.SupportedEffects) != len(catalogue) {
		t.Fatalf("supported effects = %#v, %t", snapshot.SupportedEffects, ok)
	}
	for i, option := range snapshot.SupportedEffects {
		descriptor, found := rgb.SoftwareEffectDescriptorByID(catalogue[i])
		if !found {
			t.Fatalf("catalogue effect %q has no descriptor", catalogue[i])
		}
		if option.ID != descriptor.ID || option.Label != descriptor.Label {
			t.Errorf("option %d = %+v, want ID %q and label %q", i, option, descriptor.ID, descriptor.Label)
		}

	}
}
func TestOpenRGBLightingSnapshotPaletteSemantics(t *testing.T) {
	start := lightingTestColor(11, 12, 13)

	tests := []struct {
		name    string
		effect  string
		palette string
		hex     string
		speed   bool
	}{
		{name: "Static/single-color", effect: "static", palette: "static-single-color", hex: "#0b0c0d"},
		{name: "Off", effect: "off", palette: "none"},
		{name: "two-color", effect: "wave", palette: "two-color", speed: true},
		{name: "CPU temperature", effect: "cpu-temperature", palette: "temperature-three-color"},
		{name: "GPU temperature", effect: "gpu-temperature", palette: "temperature-three-color"},
		{name: "generated palette", effect: "rainbow", palette: "generated-palette", speed: true},
		{name: "Gradient", effect: "gradient", palette: "gradient", speed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := rgb.Profile{
				ProfileName: tt.name,
				StartColor:  start,
				Speed:       6,
				MiddleColor: lightingTestColor(21, 22, 23),
				EndColor:    lightingTestColor(31, 32, 33),
			}
			profile.StartColor.Temperature = 30
			profile.MiddleColor.Temperature = 50
			profile.EndColor.Temperature = 70

			if tt.palette == "gradient" {
				profile.Gradients = map[int]rgb.Color{
					0: {Red: 10, Green: 20, Blue: 30, Position: 0.0, Brightness: 1.0},
					1: {Red: 40, Green: 50, Blue: 60, Position: 1.0, Brightness: 1.0},
				}
			}

			device := lightingTestDevice(tt.effect, []string{tt.effect}, map[string]rgb.Profile{
				tt.effect: profile,
			})

			snapshot, ok := device.LightingSnapshot()
			if !ok {
				t.Fatal("snapshot failed")
			}
			if snapshot.PaletteKind != tt.palette {
				t.Errorf("PaletteKind = %q, want %q", snapshot.PaletteKind, tt.palette)
			}
			if snapshot.HasSpeed != tt.speed || (tt.speed && snapshot.Speed != 6) {
				t.Errorf("Speed = %v (has %v), want %v (has %v)", snapshot.Speed, snapshot.HasSpeed, 6, tt.speed)
			}
			if snapshot.SingleColorHex != tt.hex {
				t.Errorf("SingleColorHex = %q, want %q", snapshot.SingleColorHex, tt.hex)
			}
			if !snapshot.Customized {
				t.Error("snapshot.Customized is false")
			}
		})
	}
}

func TestOpenRGBLightingSnapshotOverrideStates(t *testing.T) {
	device := lightingTestDevice("static", []string{"static"}, map[string]rgb.Profile{
		"static": {ProfileName: "Static", StartColor: lightingTestColor(9, 8, 7)},
	})

	t.Run("nil override", func(t *testing.T) {
		device.DeviceProfile.RGBOverride = nil
		snapshot, _ := device.LightingSnapshot()
		if snapshot.SingleColorHex != "#090807" || !snapshot.Customized {
			t.Errorf("snapshot = %+v", snapshot)
		}
	})

	t.Run("disabled override", func(t *testing.T) {
		device.DeviceProfile.RGBOverride = &RGBOverride{Enabled: false, RGBStartColor: lightingTestColor(255, 255, 255), RgbModeSpeed: 50}
		snapshot, _ := device.LightingSnapshot()
		if snapshot.SingleColorHex != "#090807" || snapshot.Speed != 0 || !snapshot.Customized {
			t.Errorf("disabled override affected state: %+v", snapshot)
		}
	})

	t.Run("enabled override", func(t *testing.T) {
		device.DeviceProfile.RGBOverride = &RGBOverride{Enabled: true, RGBStartColor: lightingTestColor(255, 255, 255), RgbModeSpeed: 50}
		snapshot, _ := device.LightingSnapshot()
		if snapshot.SingleColorHex != "#090807" || snapshot.Speed != 0 || !snapshot.Customized {
			t.Errorf("enabled override affected presentation-only canonical state: %+v", snapshot)
		}
	})
}

func TestOpenRGBLightingSnapshotBrightnessAndCluster(t *testing.T) {
	for _, tt := range []struct {
		name       string
		brightness uint8
	}{
		{name: "zero", brightness: 0},
		{name: "intermediate", brightness: 47},
		{name: "maximum", brightness: 100},
	} {
		t.Run(tt.name, func(t *testing.T) {
			device := lightingTestDevice("static", []string{"static"}, map[string]rgb.Profile{"static": {}})
			device.brightness = tt.brightness
			device.DeviceProfile.RGBCluster = true
			snapshot, _ := device.LightingSnapshot()
			if !snapshot.HasBrightness || snapshot.Brightness != tt.brightness || !snapshot.ClusterControlled {
				t.Fatalf("snapshot = %+v", snapshot)
			}
		})
	}

	t.Run("legacy profile brightness ignored", func(t *testing.T) {
		device := lightingTestDevice("static", []string{"static"}, map[string]rgb.Profile{"static": {}})
		legacyBrightness := uint8(20)
		device.DeviceProfile.BrightnessSlider = &legacyBrightness
		device.brightness = 64
		snapshot, _ := device.LightingSnapshot()
		if !snapshot.HasBrightness || snapshot.Brightness != 64 {
			t.Fatalf("legacy profile brightness affected authoritative state: %+v", snapshot)
		}
	})
}
func TestOpenRGBLightingSnapshotUnknownStates(t *testing.T) {
	t.Run("missing active profile", func(t *testing.T) {
		device := lightingTestDevice("static", []string{"static"}, map[string]rgb.Profile{"static": {ProfileName: "Static"}})
		device.DeviceProfile = nil
		snapshot, ok := device.LightingSnapshot()
		if !ok || snapshot.ConfiguredEffect != "static" || len(snapshot.SupportedEffects) != 1 {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})

	t.Run("inactive profile", func(t *testing.T) {
		device := lightingTestDevice("static", []string{"static"}, map[string]rgb.Profile{"static": {ProfileName: "Static"}})
		device.DeviceProfile.Active = false
		snapshot, ok := device.LightingSnapshot()
		if !ok || snapshot.ConfiguredEffect != "static" {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})

	t.Run("unsupported configured effect", func(t *testing.T) {
		device := lightingTestDevice("saved-unknown", []string{"static"}, map[string]rgb.Profile{
			"static":        {ProfileName: "Static"},
			"saved-unknown": {ProfileName: "Unknown", StartColor: lightingTestColor(1, 2, 3)},
		})
		snapshot, ok := device.LightingSnapshot()
		if !ok || snapshot.ConfiguredEffect != "saved-unknown" || snapshot.EffectSupported {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})

	t.Run("empty configured effect", func(t *testing.T) {
		device := lightingTestDevice("", []string{"static"}, map[string]rgb.Profile{"static": {ProfileName: "Static"}})
		snapshot, ok := device.LightingSnapshot()
		if !ok || snapshot.ConfiguredEffect != "static" || !snapshot.EffectSupported {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})

	t.Run("supported effect missing definition", func(t *testing.T) {
		device := lightingTestDevice("wave", []string{"wave"}, map[string]rgb.Profile{})
		snapshot, ok := device.LightingSnapshot()
		if !ok || !snapshot.EffectSupported || len(snapshot.SupportedEffects) != 1 {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})

	t.Run("missing RGB state", func(t *testing.T) {
		device := lightingTestDevice("wave", []string{"wave"}, nil)
		device.Rgb = nil
		snapshot, ok := device.LightingSnapshot()
		if !ok || !snapshot.EffectSupported || len(snapshot.SupportedEffects) != 1 {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})

	t.Run("unclassified supported effect", func(t *testing.T) {
		device := lightingTestDevice("future-effect", []string{"future-effect"}, map[string]rgb.Profile{
			"future-effect": {ProfileName: "Future"},
		})
		snapshot, ok := device.LightingSnapshot()
		if !ok || !snapshot.EffectSupported {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})
}

func TestOpenRGBLightingSnapshotCopiesOwnedValues(t *testing.T) {
	brightness := uint8(22)
	sourceProfile := rgb.Profile{ProfileName: "Wave", StartColor: lightingTestColor(1, 2, 3), EndColor: lightingTestColor(4, 5, 6), Speed: 7}
	device := lightingTestDevice("wave", []string{"wave", "static"}, map[string]rgb.Profile{"wave": sourceProfile})
	device.DeviceProfile.BrightnessSlider = &brightness
	device.DeviceProfile.RGBOverride = &RGBOverride{Enabled: true, RGBStartColor: lightingTestColor(7, 8, 9), RGBEndColor: lightingTestColor(10, 11, 12), RgbModeSpeed: 3}

	snapshot, ok := device.LightingSnapshot()
	if !ok {
		t.Fatal("snapshot failed")
	}
	want := snapshot
	want.SupportedEffects = append([]LightingEffectOption(nil), snapshot.SupportedEffects...)

	device.mu.Lock()
	device.RGBModes[0] = "changed"
	brightness = 99
	device.DeviceProfile.RGBOverride.RGBStartColor.Red = 200
	device.rgbMutex.Lock()
	changedProfile := device.Rgb.Profiles["wave"]
	changedProfile.ProfileName = "Changed"
	changedProfile.StartColor.Red = 201
	device.Rgb.Profiles["wave"] = changedProfile
	device.rgbMutex.Unlock()
	device.mu.Unlock()

	if !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("snapshot changed after source mutation:\n got %#v\nwant %#v", snapshot, want)
	}

	snapshot.SupportedEffects[0].ID = "caller-change"
	if device.RGBModes[0] == "caller-change" {
		t.Fatal("caller mutation reached device state")
	}
}

func TestOpenRGBLightingSnapshotConcurrentRead(t *testing.T) {
	device := lightingTestDevice("static", []string{"static", "wave"}, map[string]rgb.Profile{
		"static": {ProfileName: "Static", StartColor: lightingTestColor(1, 0, 0)},
		"wave":   {ProfileName: "Wave", StartColor: lightingTestColor(2, 0, 0), EndColor: lightingTestColor(3, 0, 0), Speed: 4},
	})
	device.brightness = 25

	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		for i := 0; i < 500; i++ {
			device.mu.Lock()
			if i%2 == 0 {
				device.effect = "static"
				device.brightness = 25
				device.RGBModes = []string{"static", "wave"}
			} else {
				device.effect = "wave"
				device.brightness = 75
				device.RGBModes = []string{"wave", "static"}
			}
			device.mu.Unlock()
		}
	}()
	go func() {
		defer wait.Done()
		for i := 0; i < 500; i++ {
			snapshot, ok := device.LightingSnapshot()
			if !ok || !snapshot.HasBrightness {
				t.Errorf("incomplete snapshot: %+v, %t", snapshot, ok)
				return
			}
			switch snapshot.ConfiguredEffect {
			case "static":
				if snapshot.Brightness != 25 {
					t.Errorf("inconsistent static snapshot: %+v", snapshot)
					return
				}
			case "wave":
				if snapshot.Brightness != 75 {
					t.Errorf("inconsistent wave snapshot: %+v", snapshot)
					return
				}
			default:
				t.Errorf("unexpected effect %q", snapshot.ConfiguredEffect)
				return
			}
		}
	}()
	wait.Wait()
}
