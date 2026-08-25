package scimitarprorgb

import (
	"os"
	"path/filepath"
	"testing"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

func setScimitarCanonicalPresentationSettings(
	t *testing.T,
	device *Device,
	runtime *lightingsettings.IndependentDeviceRuntime,
	settings lightingsettings.EffectSettings,
) {
	t.Helper()
	if err := runtime.Effects.Set(device.Serial, settings.EffectID, settings); err != nil {
		t.Fatal(err)
	}
}

func TestScimitarGetRgbProfilePresentsCanonicalEffectSettings(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)
	device.Rgb.Profiles["static"] = rgb.Profile{StartColor: rgb.Color{Red: 1, Green: 2, Blue: 3}}

	gradientSpeed := 4.0
	setScimitarCanonicalPresentationSettings(t, device, runtime, lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      "gradient",
		Speed:         &gradientSpeed,
		Gradient: &lightingsettings.GradientSettings{Stops: []lightingsettings.GradientStop{
			{Position: 0.2, Color: lightingsettings.Color{Red: 10, Green: 20, Blue: 30}, Intensity: 0.4},
			{Position: 0.8, Color: lightingsettings.Color{Red: 40, Green: 50, Blue: 60}, Intensity: 0.9},
		}},
	})
	setScimitarCanonicalPresentationSettings(t, device, runtime, lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      "cpu-temperature",
		Temperature: &lightingsettings.TemperatureSettings{
			Low:    lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Red: 11}, Celsius: 21},
			Middle: lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Green: 22}, Celsius: 51},
			High:   lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Blue: 33}, Celsius: 81},
		},
	})
	setScimitarCanonicalPresentationSettings(t, device, runtime, lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      "wave",
		Speed:         &gradientSpeed,
		TwoColor: &lightingsettings.TwoColorSettings{
			Start: lightingsettings.Color{Red: 70, Green: 71, Blue: 72},
			End:   lightingsettings.Color{Red: 80, Green: 81, Blue: 82},
		},
	})

	static := device.GetRgbProfile("static")
	if static == nil || static.StartColor != (rgb.Color{Green: 255, Blue: 255}) {
		t.Fatalf("canonical Static presentation = %#v", static)
	}
	if static.ProfileName != "" || static.Brightness != 0 || static.Smoothness != 0 || static.Gradients != nil {
		t.Fatalf("Static presentation retained legacy authority: %#v", static)
	}

	wave := device.GetRgbProfile("wave")
	if wave == nil || wave.Speed != 4 || wave.StartColor.Red != 70 || wave.EndColor.Blue != 82 {
		t.Fatalf("canonical Wave presentation = %#v", wave)
	}

	temperature := device.GetRgbProfile("cpu-temperature")
	if temperature == nil || temperature.StartColor.Red != 11 || temperature.StartColor.Temperature != 21 ||
		temperature.MiddleColor.Green != 22 || temperature.MiddleColor.Temperature != 51 ||
		temperature.EndColor.Blue != 33 || temperature.EndColor.Temperature != 81 {
		t.Fatalf("canonical CPU temperature presentation = %#v", temperature)
	}

	gradient := device.GetRgbProfile("gradient")
	if gradient == nil || gradient.Speed != 4 || len(gradient.Gradients) != 2 ||
		gradient.Gradients[0] != (rgb.Color{Red: 10, Green: 20, Blue: 30, Brightness: 0.4, Position: 0.2}) ||
		gradient.Gradients[1] != (rgb.Color{Red: 40, Green: 50, Blue: 60, Brightness: 0.9, Position: 0.8}) {
		t.Fatalf("canonical Gradient presentation = %#v", gradient)
	}

	generated := device.GetRgbProfile("rainbow")
	if generated == nil || generated.Gradients != nil || generated.StartColor != (rgb.Color{}) || generated.EndColor != (rgb.Color{}) {
		t.Fatalf("generated-palette presentation invented colors: %#v", generated)
	}
	if device.GetRgbProfile("mouse") != nil || device.GetRgbProfile("unknown") != nil {
		t.Fatal("retired or unsupported effect received canonical presentation")
	}

	gradient.Gradients[0] = rgb.Color{}
	stored, found, err := runtime.Effects.Get(device.Serial, "gradient")
	if err != nil || !found || stored.Gradient == nil || stored.Gradient.Stops[0].Color.Red != 10 {
		t.Fatalf("presentation mutated canonical Gradient data: %#v, %t, %v", stored, found, err)
	}
}

func TestScimitarGetRgbProfilesUsesCanonicalEffectsAfterReload(t *testing.T) {
	device, runtime, effectsPath := newScimitarCanonicalLightingTestDeviceWithEffectPath(t)
	prepareScimitarCanonicalMutationDevice(device)
	custom := lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      "static",
		SingleColor:   &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 9, Green: 8, Blue: 7}},
	}
	setScimitarCanonicalPresentationSettings(t, device, runtime, custom)

	defaultsPath, err := filepath.Abs(filepath.Join("..", "..", "..", "database", "rgb.json"))
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := lightingsettings.LoadIndependentDeviceRuntime(
		filepath.Join(filepath.Dir(effectsPath), "independent-device-state.json"),
		effectsPath,
		defaultsPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	reloadedDevice := &Device{Serial: device.Serial, Product: "SCIMITAR PRO RGB", Rgb: &rgb.RGB{
		Device:   "legacy product metadata",
		Profiles: map[string]rgb.Profile{"mouse": {}, "static": {StartColor: rgb.Color{Red: 1}}},
	}}
	if err = reloadedDevice.attachIndependentDeviceLightingSource(reloaded); err != nil {
		t.Fatal(err)
	}

	presented, ok := reloadedDevice.GetRgbProfiles().(rgb.RGB)
	if !ok {
		t.Fatalf("GetRgbProfiles type = %T, want rgb.RGB", reloadedDevice.GetRgbProfiles())
	}
	if presented.Device != "legacy product metadata" || len(presented.Profiles) != len(rgbModes) {
		t.Fatalf("canonical RGB collection = %#v", presented)
	}
	if profile := presented.Profiles["static"]; profile.StartColor != (rgb.Color{Red: 9, Green: 8, Blue: 7}) {
		t.Fatalf("reloaded canonical Static presentation = %#v", profile)
	}
	if _, found := presented.Profiles["mouse"]; found {
		t.Fatal("retired mouse profile was presented")
	}

	defaults, err := reloaded.Defaults.Get("static")
	if err != nil || defaults.SingleColor == nil || defaults.SingleColor.Color.Green != 255 || defaults.SingleColor.Color.Blue != 255 {
		t.Fatalf("presentation changed shipped defaults: %#v, %v", defaults, err)
	}
}

func TestScimitarCanonicalMutationsAndLegacyUpgradeDoNotUsePublicPresentationLookup(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)
	device.Rgb.Profiles = map[string]rgb.Profile{"mouse": {}}
	device.lightingRestart = func() {}

	if result := device.UpdateRgbProfileData("static", rgb.Profile{StartColor: rgb.Color{Red: 12}}); result != 1 {
		t.Fatalf("UpdateRgbProfileData without legacy Static = %d, want 1", result)
	}
	if status, _ := device.ProcessNewGradientColor("gradient"); status != 1 {
		t.Fatalf("ProcessNewGradientColor without legacy Gradient = %d, want 1", status)
	}
	if status, _ := device.ProcessDeleteGradientColor("gradient"); status != 1 {
		t.Fatalf("ProcessDeleteGradientColor without legacy Gradient = %d, want 1", status)
	}
	if result := device.UpdateRgbProfile(-1, "rainbow"); result != 1 {
		t.Fatalf("UpdateRgbProfile without legacy Rainbow = %d, want 1", result)
	}
	if _, found, err := runtime.Effects.Get(device.Serial, "static"); err != nil || !found {
		t.Fatalf("canonical Static mutation was not persisted: %t, %v", found, err)
	}

	legacy := &Device{Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{"static": {StartColor: rgb.Color{Red: 1}}}}}
	path := filepath.Join(t.TempDir(), "legacy-rgb.json")
	legacy.upgradeRgbProfile(path, []string{"static"})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("legacy upgrade did not use the retained legacy lookup: %v", err)
	}
}
