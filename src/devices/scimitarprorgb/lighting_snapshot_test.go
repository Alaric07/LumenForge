package scimitarprorgb

import (
	"testing"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

func TestScimitarLightingSnapshotUsesCanonicalDesiredStateAndSettings(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)
	device.Rgb.Profiles["static"] = rgb.Profile{StartColor: rgb.Color{Red: 1}}
	device.DeviceProfile.RGBProfile = "rainbow"
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 36}); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Effects.Set(device.Serial, "static", lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static",
		SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 12, Green: 34, Blue: 56}},
	}); err != nil {
		t.Fatal(err)
	}

	snapshot, usable := device.LightingSnapshot()
	if !usable || snapshot.ConfiguredEffect != "static" || snapshot.Brightness != 36 || snapshot.SingleColorHex != "#0c2238" || !snapshot.Customized {
		t.Fatalf("canonical snapshot = %#v, usable=%t", snapshot, usable)
	}
	wantEffects := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	if len(snapshot.SupportedEffects) != len(wantEffects) {
		t.Fatalf("Scimitar snapshot effects = %#v", snapshot.SupportedEffects)
	}
	for index, effect := range snapshot.SupportedEffects {
		if effect.ID != wantEffects[index] {
			t.Fatalf("Scimitar snapshot effect %d = %q, want %q", index, effect.ID, wantEffects[index])
		}
		if effect.ID == "mouse" {
			t.Fatal("Scimitar snapshot exposed retired mouse effect")
		}
		if _, ok := scimitarCanonicalEffectDescriptor(effect.ID); !ok {
			t.Fatalf("Scimitar snapshot exposed unsupported effect %q", effect.ID)
		}
	}
}

func TestScimitarLightingSnapshotPresentsCanonicalPaletteShapesAndCopies(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)

	settings := map[string]lightingsettings.EffectSettings{
		"wave": {SchemaVersion: lightingsettings.SchemaVersion, EffectID: "wave", Speed: float64Pointer(4), TwoColor: &lightingsettings.TwoColorSettings{Start: lightingsettings.Color{Red: 1}, End: lightingsettings.Color{Blue: 2}}},
		"cpu-temperature": {SchemaVersion: lightingsettings.SchemaVersion, EffectID: "cpu-temperature", Temperature: &lightingsettings.TemperatureSettings{Low: lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Green: 1}, Celsius: 20}, Middle: lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Red: 2, Green: 2}, Celsius: 50}, High: lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Blue: 3}, Celsius: 90}}},
		"gradient": {SchemaVersion: lightingsettings.SchemaVersion, EffectID: "gradient", Speed: float64Pointer(3), Gradient: &lightingsettings.GradientSettings{Stops: []lightingsettings.GradientStop{{Position: 0.2, Color: lightingsettings.Color{Red: 4}, Intensity: 0.4}, {Position: 0.8, Color: lightingsettings.Color{Blue: 5}, Intensity: 0.9}}}},
	}
	for effect, value := range settings {
		if err := runtime.Effects.Set(device.Serial, effect, value); err != nil {
			t.Fatal(err)
		}
	}

	for _, test := range []struct {
		effect string
		check  func(LightingSnapshot) bool
	}{
		{"wave", func(value LightingSnapshot) bool { return value.HasSpeed && value.Speed == 4 && value.TwoColorStartHex == "#010000" && value.TwoColorEndHex == "#000002" }},
		{"cpu-temperature", func(value LightingSnapshot) bool { return value.HasTemperature && value.TemperatureLow.Celsius == 20 && value.TemperatureMiddle.ColorHex == "#020200" && value.TemperatureHigh.Celsius == 90 }},
		{"gradient", func(value LightingSnapshot) bool { return value.HasSpeed && value.HasGradient && len(value.GradientStops) == 2 && value.GradientStops[0].Position == 0.2 && value.GradientStops[0].Intensity == 0.4 && value.GradientStops[1].ColorHex == "#000005" }},
		{"rainbow", func(value LightingSnapshot) bool { return value.EffectSupported && value.PaletteKind == string(rgb.LightingPaletteGenerated) && !value.HasGradient && value.SingleColorHex == "" }},
		{"off", func(value LightingSnapshot) bool { return value.EffectSupported && value.PaletteKind == string(rgb.LightingPaletteNone) && !value.HasSpeed }},
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

func TestScimitarLightingSnapshotResolvesDefaultsWithoutPersistence(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	snapshot, usable := device.LightingSnapshot()
	if !usable || snapshot.ConfiguredEffect != "static" || snapshot.Brightness != 100 || snapshot.Customized || snapshot.SingleColorHex != "#00ffff" {
		t.Fatalf("default snapshot = %#v, usable=%t", snapshot, usable)
	}
	if _, found, err := runtime.State.Resolve(device.Serial); err != nil || found {
		t.Fatalf("snapshot materialized canonical state: found=%t err=%v", found, err)
	}
}

func float64Pointer(value float64) *float64 { return &value }
