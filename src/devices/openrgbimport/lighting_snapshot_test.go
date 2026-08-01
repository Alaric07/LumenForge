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
	return &Device{
		IsOpenRGB: true,
		RGBModes:  append([]string(nil), modes...),
		DeviceProfile: &DeviceProfile{
			Active:           true,
			RGBProfile:       effect,
			BrightnessSlider: &brightness,
		},
		Rgb: &rgb.RGB{Profiles: definitions},
	}
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
	if !snapshot.HasActiveProfile || snapshot.ConfiguredEffect != "colorpulse" || !snapshot.EffectSupported {
		t.Fatalf("configured state = %+v", snapshot)
	}
	if !snapshot.HasBrightness || snapshot.Brightness != 50 || !snapshot.ClusterControlled {
		t.Fatalf("profile state = %+v", snapshot)
	}
	wantOptions := []LightingEffectOption{
		{ID: "static", Label: "Static", CapabilityKnown: true, Capability: rgb.LightingEffectCapability{Palette: rgb.LightingPaletteStaticSingle, UsesStartColor: true}},
		{ID: "colorpulse", Label: "Color Pulse", CapabilityKnown: true, Capability: rgb.LightingEffectCapability{Palette: rgb.LightingPaletteTwoColor, UsesStartColor: true, UsesEndColor: true, SupportsSpeed: true}},
		{ID: "off", CapabilityKnown: true, Capability: rgb.LightingEffectCapability{Palette: rgb.LightingPaletteNone}},
	}
	if !reflect.DeepEqual(snapshot.SupportedEffects, wantOptions) {
		t.Fatalf("supported effects = %#v, want %#v", snapshot.SupportedEffects, wantOptions)
	}
	if snapshot.BaseDefinition == nil || snapshot.BaseDefinition.Palette != rgb.LightingPaletteTwoColor ||
		!snapshot.BaseDefinition.HasStartColor || snapshot.BaseDefinition.StartColor != start ||
		snapshot.BaseDefinition.HasMiddleColor || !snapshot.BaseDefinition.HasEndColor ||
		snapshot.BaseDefinition.EndColor != end || !snapshot.BaseDefinition.HasSpeed || snapshot.BaseDefinition.Speed != 3 {
		t.Fatalf("base definition = %+v", snapshot.BaseDefinition)
	}
	if snapshot.Override == nil || !snapshot.Override.Enabled || snapshot.Override.Speed != 0 {
		t.Fatalf("override = %+v", snapshot.Override)
	}
	if snapshot.Effective == nil || snapshot.Effective.StartColor != device.DeviceProfile.RGBOverride.RGBStartColor ||
		snapshot.Effective.EndColor != device.DeviceProfile.RGBOverride.RGBEndColor || snapshot.Effective.HasMiddleColor ||
		snapshot.Effective.Speed != 0 {
		t.Fatalf("effective definition = %+v", snapshot.Effective)
	}
}

func TestOpenRGBLightingSnapshotSupportedCatalogCapabilities(t *testing.T) {
	definitions := make(map[string]rgb.Profile, len(rgbModes))
	for _, effect := range rgbModes {
		definitions[effect] = rgb.Profile{ProfileName: effect}
	}
	device := lightingTestDevice("static", rgbModes, definitions)

	snapshot, ok := device.LightingSnapshot()
	if !ok || len(snapshot.SupportedEffects) != len(rgbModes) {
		t.Fatalf("supported effects = %#v, %t", snapshot.SupportedEffects, ok)
	}
	for i, option := range snapshot.SupportedEffects {
		if option.ID != rgbModes[i] || option.Label != rgbModes[i] {
			t.Errorf("option %d = %+v, want ID and label %q", i, option, rgbModes[i])
		}
		if !option.CapabilityKnown {
			t.Errorf("OpenRGB-supported effect %q has no confirmed capability mapping", option.ID)
		}
	}
}

func TestOpenRGBLightingSnapshotPaletteSemantics(t *testing.T) {
	start := lightingTestColor(11, 12, 13)
	middle := lightingTestColor(21, 22, 23)
	end := lightingTestColor(31, 32, 33)
	override := &RGBOverride{
		Enabled:        true,
		RGBStartColor:  rgb.Color{},
		RGBMiddleColor: lightingTestColor(41, 42, 43),
		RGBEndColor:    lightingTestColor(51, 52, 53),
		RgbModeSpeed:   0,
	}
	tests := []struct {
		name            string
		effect          string
		palette         rgb.LightingPaletteKind
		start           bool
		middle          bool
		end             bool
		speed           bool
		effectiveSpeed  float64
		effectiveStart  rgb.Color
		effectiveMiddle rgb.Color
		effectiveEnd    rgb.Color
	}{
		{name: "static", effect: "static", palette: rgb.LightingPaletteStaticSingle, start: true, effectiveStart: override.RGBStartColor},
		{name: "off", effect: "off", palette: rgb.LightingPaletteNone},
		{name: "two color", effect: "wave", palette: rgb.LightingPaletteTwoColor, start: true, end: true, speed: true, effectiveStart: override.RGBStartColor, effectiveEnd: override.RGBEndColor},
		{name: "CPU temperature", effect: "cpu-temperature", palette: rgb.LightingPaletteTemperatureThree, start: true, middle: true, end: true, effectiveStart: override.RGBStartColor, effectiveMiddle: override.RGBMiddleColor, effectiveEnd: override.RGBEndColor},
		{name: "GPU temperature", effect: "gpu-temperature", palette: rgb.LightingPaletteTemperatureThree, start: true, middle: true, end: true, effectiveStart: override.RGBStartColor, effectiveMiddle: override.RGBMiddleColor, effectiveEnd: override.RGBEndColor},
		{name: "generated", effect: "rainbow", palette: rgb.LightingPaletteGenerated, speed: true},
		{name: "gradient", effect: "gradient", palette: rgb.LightingPaletteGradient, speed: true, effectiveSpeed: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device := lightingTestDevice(tt.effect, []string{tt.effect}, map[string]rgb.Profile{
				tt.effect: {
					ProfileName: tt.name,
					StartColor:  start,
					MiddleColor: middle,
					EndColor:    end,
					Speed:       6,
					Gradients:   map[int]rgb.Color{0: start, 1: end},
				},
			})
			copyOverride := *override
			device.DeviceProfile.RGBOverride = &copyOverride

			snapshot, ok := device.LightingSnapshot()
			if !ok || snapshot.BaseDefinition == nil || snapshot.Effective == nil {
				t.Fatalf("snapshot = %+v, %t", snapshot, ok)
			}
			base := snapshot.BaseDefinition
			effective := snapshot.Effective
			if base.Palette != tt.palette || base.HasStartColor != tt.start ||
				base.HasMiddleColor != tt.middle || base.HasEndColor != tt.end || base.HasSpeed != tt.speed {
				t.Fatalf("base definition = %+v", base)
			}
			if tt.start && effective.StartColor != tt.effectiveStart {
				t.Errorf("effective start = %+v, want %+v", effective.StartColor, tt.effectiveStart)
			}
			if tt.middle && effective.MiddleColor != tt.effectiveMiddle {
				t.Errorf("effective middle = %+v, want %+v", effective.MiddleColor, tt.effectiveMiddle)
			}
			if tt.end && effective.EndColor != tt.effectiveEnd {
				t.Errorf("effective end = %+v, want %+v", effective.EndColor, tt.effectiveEnd)
			}
			if effective.HasSpeed && effective.Speed != tt.effectiveSpeed {
				t.Errorf("effective speed = %v, want %v", effective.Speed, tt.effectiveSpeed)
			}
			if (tt.palette == rgb.LightingPaletteGenerated || tt.palette == rgb.LightingPaletteGradient || tt.palette == rgb.LightingPaletteNone) &&
				(effective.HasStartColor || effective.HasMiddleColor || effective.HasEndColor) {
				t.Errorf("effect fabricated fixed colors: %+v", effective)
			}
		})
	}
}

func TestOpenRGBLightingSnapshotUnknownStates(t *testing.T) {
	t.Run("missing active profile", func(t *testing.T) {
		device := lightingTestDevice("static", []string{"static"}, map[string]rgb.Profile{"static": {ProfileName: "Static"}})
		device.DeviceProfile = nil
		snapshot, ok := device.LightingSnapshot()
		if !ok || snapshot.HasActiveProfile || snapshot.ConfiguredEffect != "" || snapshot.BaseDefinition != nil || len(snapshot.SupportedEffects) != 1 {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})

	t.Run("inactive profile", func(t *testing.T) {
		device := lightingTestDevice("static", []string{"static"}, map[string]rgb.Profile{"static": {ProfileName: "Static"}})
		device.DeviceProfile.Active = false
		snapshot, ok := device.LightingSnapshot()
		if !ok || snapshot.HasActiveProfile || snapshot.BaseDefinition != nil {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})

	t.Run("unsupported configured effect", func(t *testing.T) {
		device := lightingTestDevice("saved-unknown", []string{"static"}, map[string]rgb.Profile{
			"static":        {ProfileName: "Static"},
			"saved-unknown": {ProfileName: "Unknown", StartColor: lightingTestColor(1, 2, 3)},
		})
		snapshot, ok := device.LightingSnapshot()
		if !ok || snapshot.ConfiguredEffect != "saved-unknown" || snapshot.EffectSupported || snapshot.BaseDefinition != nil || snapshot.Effective != nil {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})

	t.Run("empty configured effect", func(t *testing.T) {
		device := lightingTestDevice("", []string{"static"}, map[string]rgb.Profile{"static": {ProfileName: "Static"}})
		snapshot, ok := device.LightingSnapshot()
		if !ok || snapshot.ConfiguredEffect != "" || snapshot.EffectSupported || snapshot.BaseDefinition != nil {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})

	t.Run("supported effect missing definition", func(t *testing.T) {
		device := lightingTestDevice("wave", []string{"wave"}, map[string]rgb.Profile{})
		snapshot, ok := device.LightingSnapshot()
		if !ok || !snapshot.EffectSupported || len(snapshot.SupportedEffects) != 1 || snapshot.BaseDefinition != nil || snapshot.Effective != nil {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})

	t.Run("missing RGB state", func(t *testing.T) {
		device := lightingTestDevice("wave", []string{"wave"}, nil)
		device.Rgb = nil
		snapshot, ok := device.LightingSnapshot()
		if !ok || !snapshot.EffectSupported || len(snapshot.SupportedEffects) != 1 || snapshot.BaseDefinition != nil || snapshot.Effective != nil {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})

	t.Run("unclassified supported effect", func(t *testing.T) {
		device := lightingTestDevice("future-effect", []string{"future-effect"}, map[string]rgb.Profile{
			"future-effect": {ProfileName: "Future"},
		})
		snapshot, ok := device.LightingSnapshot()
		if !ok || !snapshot.EffectSupported || snapshot.SupportedEffects[0].CapabilityKnown || snapshot.BaseDefinition != nil {
			t.Fatalf("snapshot = %+v, %t", snapshot, ok)
		}
	})
}

func TestOpenRGBLightingSnapshotOverrideStates(t *testing.T) {
	baseStart := lightingTestColor(9, 8, 7)
	baseDefinition := map[string]rgb.Profile{"static": {ProfileName: "Static", StartColor: baseStart}}

	t.Run("nil", func(t *testing.T) {
		device := lightingTestDevice("static", []string{"static"}, baseDefinition)
		snapshot, _ := device.LightingSnapshot()
		if snapshot.Override != nil || snapshot.Effective == nil || snapshot.Effective.StartColor != baseStart {
			t.Fatalf("snapshot = %+v", snapshot)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		device := lightingTestDevice("static", []string{"static"}, baseDefinition)
		device.DeviceProfile.RGBOverride = &RGBOverride{RGBStartColor: rgb.Color{}, RgbModeSpeed: 0}
		snapshot, _ := device.LightingSnapshot()
		if snapshot.Override == nil || snapshot.Override.Enabled || snapshot.Override.StartColor != (rgb.Color{}) || snapshot.Override.Speed != 0 {
			t.Fatalf("override = %+v", snapshot.Override)
		}
		if snapshot.Effective == nil || snapshot.Effective.StartColor != baseStart {
			t.Fatalf("disabled override changed effective state: %+v", snapshot.Effective)
		}
	})

	t.Run("enabled black", func(t *testing.T) {
		device := lightingTestDevice("static", []string{"static"}, baseDefinition)
		device.DeviceProfile.RGBOverride = &RGBOverride{Enabled: true, RGBStartColor: rgb.Color{}, RgbModeSpeed: 0}
		snapshot, _ := device.LightingSnapshot()
		if snapshot.Override == nil || snapshot.Override.StartColor != (rgb.Color{}) || snapshot.Override.Speed != 0 {
			t.Fatalf("override = %+v", snapshot.Override)
		}
		if snapshot.Effective == nil || !snapshot.Effective.HasStartColor || snapshot.Effective.StartColor != (rgb.Color{}) {
			t.Fatalf("black override was not effective: %+v", snapshot.Effective)
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
			*device.DeviceProfile.BrightnessSlider = tt.brightness
			device.DeviceProfile.RGBCluster = true
			snapshot, _ := device.LightingSnapshot()
			if !snapshot.HasBrightness || snapshot.Brightness != tt.brightness || !snapshot.ClusterControlled {
				t.Fatalf("snapshot = %+v", snapshot)
			}
		})
	}

	device := lightingTestDevice("static", []string{"static"}, map[string]rgb.Profile{"static": {}})
	device.DeviceProfile.BrightnessSlider = nil
	snapshot, _ := device.LightingSnapshot()
	if snapshot.HasBrightness || snapshot.Brightness != 0 {
		t.Fatalf("missing brightness was presented as configured: %+v", snapshot)
	}
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
	want.BaseDefinition = copyLightingDefinition(snapshot.BaseDefinition)
	want.Effective = copyLightingDefinition(snapshot.Effective)
	wantOverride := *snapshot.Override
	want.Override = &wantOverride

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
	snapshot.BaseDefinition.StartColor.Red = 202
	snapshot.Override.StartColor.Red = 203
	if device.RGBModes[0] == "caller-change" || device.DeviceProfile.RGBOverride.RGBStartColor.Red == 203 {
		t.Fatal("caller mutation reached device state")
	}
	device.rgbMutex.RLock()
	defer device.rgbMutex.RUnlock()
	if device.Rgb.Profiles["wave"].StartColor.Red == 202 {
		t.Fatal("caller mutation reached RGB definition")
	}
}

func copyLightingDefinition(definition *LightingDefinitionSnapshot) *LightingDefinitionSnapshot {
	if definition == nil {
		return nil
	}
	copy := *definition
	return &copy
}

func TestOpenRGBLightingSnapshotConcurrentRead(t *testing.T) {
	device := lightingTestDevice("static", []string{"static", "wave"}, map[string]rgb.Profile{
		"static": {ProfileName: "Static", StartColor: lightingTestColor(1, 0, 0)},
		"wave":   {ProfileName: "Wave", StartColor: lightingTestColor(2, 0, 0), EndColor: lightingTestColor(3, 0, 0), Speed: 4},
	})
	*device.DeviceProfile.BrightnessSlider = 25

	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		for i := 0; i < 500; i++ {
			device.mu.Lock()
			device.rgbMutex.Lock()
			if i%2 == 0 {
				brightness := uint8(25)
				device.DeviceProfile = &DeviceProfile{Active: true, RGBProfile: "static", BrightnessSlider: &brightness}
				device.RGBModes = []string{"static", "wave"}
				device.Rgb.Profiles["static"] = rgb.Profile{ProfileName: "Static", StartColor: lightingTestColor(1, 0, 0)}
			} else {
				brightness := uint8(75)
				device.DeviceProfile = &DeviceProfile{Active: true, RGBProfile: "wave", BrightnessSlider: &brightness}
				device.RGBModes = []string{"wave", "static"}
				device.Rgb.Profiles["wave"] = rgb.Profile{ProfileName: "Wave", StartColor: lightingTestColor(2, 0, 0), EndColor: lightingTestColor(3, 0, 0), Speed: 4}
			}
			device.rgbMutex.Unlock()
			device.mu.Unlock()
		}
	}()
	go func() {
		defer wait.Done()
		for i := 0; i < 500; i++ {
			snapshot, ok := device.LightingSnapshot()
			if !ok || snapshot.BaseDefinition == nil || !snapshot.HasBrightness {
				t.Errorf("incomplete snapshot: %+v, %t", snapshot, ok)
				return
			}
			switch snapshot.ConfiguredEffect {
			case "static":
				if snapshot.Brightness != 25 || snapshot.BaseDefinition.StartColor.Red != 1 {
					t.Errorf("inconsistent static snapshot: %+v", snapshot)
					return
				}
			case "wave":
				if snapshot.Brightness != 75 || snapshot.BaseDefinition.StartColor.Red != 2 {
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
