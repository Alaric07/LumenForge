package scimitarprorgb

import (
	"errors"
	"reflect"
	"testing"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
)

type failingScimitarEffectSettingsStore struct {
	err      error
	setCalls int
	deviceID string
	effect   string
	settings lightingsettings.EffectSettings
}

func (store *failingScimitarEffectSettingsStore) Set(
	deviceID string,
	effect string,
	settings lightingsettings.EffectSettings,
) error {
	store.setCalls++
	store.deviceID = deviceID
	store.effect = effect
	store.settings = settings.Clone()
	return store.err
}

func TestScimitarEffectSettingsFromRGBProfileUsesDescriptorShape(t *testing.T) {
	device, _ := newScimitarCanonicalLightingTestDevice(t)
	color := func(red, green, blue float64) rgb.Color {
		return rgb.Color{Red: red, Green: green, Blue: blue}
	}
	canonicalColor := func(red, green, blue float64) lightingsettings.Color {
		return lightingsettings.Color{Red: red, Green: green, Blue: blue}
	}
	floatPointer := func(value float64) *float64 { return &value }

	tests := []struct {
		name    string
		effect  string
		profile rgb.Profile
		want    lightingsettings.EffectSettings
	}{
		{
			name:    "single color",
			effect:  "static",
			profile: rgb.Profile{Speed: 9, StartColor: color(1, 2, 3), Brightness: 0.25, Smoothness: 99},
			want: lightingsettings.EffectSettings{
				SchemaVersion: lightingsettings.SchemaVersion,
				EffectID:      "static",
				SingleColor:   &lightingsettings.SingleColorSettings{Color: canonicalColor(1, 2, 3)},
			},
		},
		{
			name:    "single color with speed",
			effect:  "rotator",
			profile: rgb.Profile{Speed: 2, StartColor: color(4, 5, 6)},
			want: lightingsettings.EffectSettings{
				SchemaVersion: lightingsettings.SchemaVersion,
				EffectID:      "rotator",
				Speed:         floatPointer(2),
				SingleColor:   &lightingsettings.SingleColorSettings{Color: canonicalColor(4, 5, 6)},
			},
		},
		{
			name:    "two colors with speed",
			effect:  "wave",
			profile: rgb.Profile{Speed: 3, StartColor: color(7, 8, 9), EndColor: color(10, 11, 12)},
			want: lightingsettings.EffectSettings{
				SchemaVersion: lightingsettings.SchemaVersion,
				EffectID:      "wave",
				Speed:         floatPointer(3),
				TwoColor: &lightingsettings.TwoColorSettings{
					Start: canonicalColor(7, 8, 9),
					End:   canonicalColor(10, 11, 12),
				},
			},
		},
		{
			name:   "temperature colors",
			effect: "cpu-temperature",
			profile: rgb.Profile{
				Speed:       9,
				StartColor:  rgb.Color{Red: 13, Green: 14, Blue: 15, Temperature: 20},
				MiddleColor: rgb.Color{Red: 16, Green: 17, Blue: 18, Temperature: 50},
				EndColor:    rgb.Color{Red: 19, Green: 20, Blue: 21, Temperature: 80},
			},
			want: lightingsettings.EffectSettings{
				SchemaVersion: lightingsettings.SchemaVersion,
				EffectID:      "cpu-temperature",
				Temperature: &lightingsettings.TemperatureSettings{
					Low:    lightingsettings.TemperaturePoint{Color: canonicalColor(13, 14, 15), Celsius: 20},
					Middle: lightingsettings.TemperaturePoint{Color: canonicalColor(16, 17, 18), Celsius: 50},
					High:   lightingsettings.TemperaturePoint{Color: canonicalColor(19, 20, 21), Celsius: 80},
				},
			},
		},
		{
			name:   "gradient with speed",
			effect: "gradient",
			profile: rgb.Profile{Speed: 4, Gradients: map[int]rgb.Color{
				0: {Red: 22, Green: 23, Blue: 24, Position: 0, Brightness: 0.5},
				1: {Red: 25, Green: 26, Blue: 27, Position: 1, Brightness: 1},
			}},
			want: lightingsettings.EffectSettings{
				SchemaVersion: lightingsettings.SchemaVersion,
				EffectID:      "gradient",
				Speed:         floatPointer(4),
				Gradient: &lightingsettings.GradientSettings{Stops: []lightingsettings.GradientStop{
					{Position: 0, Color: canonicalColor(22, 23, 24), Intensity: 0.5},
					{Position: 1, Color: canonicalColor(25, 26, 27), Intensity: 1},
				}},
			},
		},
		{
			name:    "generated palette with speed",
			effect:  "rainbow",
			profile: rgb.Profile{Speed: 5, StartColor: color(28, 29, 30), AlternateColors: true},
			want: lightingsettings.EffectSettings{
				SchemaVersion: lightingsettings.SchemaVersion,
				EffectID:      "rainbow",
				Speed:         floatPointer(5),
			},
		},
		{
			name:    "no settings",
			effect:  "off",
			profile: rgb.Profile{Speed: -1, StartColor: color(999, 999, 999), ProfileName: "ignored"},
			want: lightingsettings.EffectSettings{
				SchemaVersion: lightingsettings.SchemaVersion,
				EffectID:      "off",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current, err := device.lightingSource.resolveEffectSettings(test.effect)
			if err != nil {
				t.Fatal(err)
			}
			got, err := scimitarEffectSettingsFromRGBProfile(test.effect, test.profile, current)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("canonical %s settings = %#v, want %#v", test.effect, got, test.want)
			}
		})
	}
}

func TestScimitarUpdateRgbProfileDataPersistsCanonicalSettingsAndReloads(t *testing.T) {
	device, runtime, effectsPath := newScimitarCanonicalLightingTestDeviceWithEffectPath(t)
	prepareScimitarCanonicalMutationDevice(device)
	device.DeviceProfile.RgbOff = false
	state := lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 42}
	if err := runtime.State.Set(device.Serial, state); err != nil {
		t.Fatal(err)
	}
	waveSpeed := 2.0
	wave := lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      "wave",
		Speed:         &waveSpeed,
		TwoColor: &lightingsettings.TwoColorSettings{
			Start: lightingsettings.Color{Red: 1},
			End:   lightingsettings.Color{Blue: 2},
		},
	}
	if err := runtime.Effects.Set(device.Serial, "wave", wave); err != nil {
		t.Fatal(err)
	}
	legacyBefore := device.Rgb.Profiles["static"]
	restarts := 0
	device.lightingRestart = func() { restarts++ }

	profile := rgb.Profile{
		Speed:       9,
		Brightness:  0.25,
		Smoothness:  99,
		StartColor:  rgb.Color{Red: 11, Green: 22, Blue: 33, Brightness: 0.75},
		EndColor:    rgb.Color{Red: 44, Green: 55, Blue: 66},
		ProfileName: "legacy-only",
		PerLed:      true,
		Version:     123,
	}
	if result := device.UpdateRgbProfileData("static", profile); result != 1 {
		t.Fatalf("UpdateRgbProfileData(static) = %d, want 1", result)
	}
	if restarts != 1 {
		t.Fatalf("selected local effect restarts = %d, want 1", restarts)
	}
	stored, found, err := runtime.Effects.Get(device.Serial, "static")
	want := lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      "static",
		SingleColor: &lightingsettings.SingleColorSettings{
			Color: lightingsettings.Color{Red: 11, Green: 22, Blue: 33},
		},
	}
	if err != nil || !found || !reflect.DeepEqual(stored, want) {
		t.Fatalf("stored canonical Static = %#v, %t, %v; want %#v", stored, found, err, want)
	}
	resolvedState, stateFound, err := runtime.State.Resolve(device.Serial)
	if err != nil || !stateFound || resolvedState != state {
		t.Fatalf("selected effect or brightness changed = %#v, %t, %v", resolvedState, stateFound, err)
	}
	storedWave, waveFound, err := runtime.Effects.Get(device.Serial, "wave")
	if err != nil || !waveFound || !reflect.DeepEqual(storedWave, wave) {
		t.Fatalf("unrelated Wave customization = %#v, %t, %v", storedWave, waveFound, err)
	}
	if legacyAfter := device.Rgb.Profiles["static"]; !reflect.DeepEqual(legacyAfter, legacyBefore) {
		t.Fatalf("legacy Static profile changed from %#v to %#v", legacyBefore, legacyAfter)
	}

	reloaded, err := lightingsettings.LoadDeviceStore(effectsPath)
	if err != nil {
		t.Fatal(err)
	}
	reloadedResolver, err := lightingsettings.NewDeviceResolver(runtime.Defaults, reloaded)
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := reloadedResolver.Resolve(lightingsettings.IndependentDevice(device.Serial), "static")
	if err != nil || !resolution.Customized || !reflect.DeepEqual(resolution.Settings, want) {
		t.Fatalf("reloaded canonical Static resolution = %#v, %v; want %#v", resolution, err, want)
	}
}

func TestScimitarUpdateRgbProfileDataInactiveEffectDoesNotRestart(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{
		SelectedEffect: "static",
		Brightness:     42,
	}); err != nil {
		t.Fatal(err)
	}
	restarts := 0
	device.lightingRestart = func() { restarts++ }

	if result := device.UpdateRgbProfileData("rainbow", rgb.Profile{Speed: 3}); result != 1 {
		t.Fatalf("UpdateRgbProfileData(rainbow) = %d, want 1", result)
	}
	stored, found, err := runtime.Effects.Get(device.Serial, "rainbow")
	if err != nil || !found || stored.Speed == nil || *stored.Speed != 3 {
		t.Fatalf("stored inactive Rainbow = %#v, %t, %v", stored, found, err)
	}
	if restarts != 0 {
		t.Fatalf("inactive effect restarted lighting %d times", restarts)
	}
}

func TestScimitarUpdateRgbProfileDataExternalOwnershipPersistsWithoutOutput(t *testing.T) {
	for _, test := range []struct {
		name    string
		cluster bool
		openRGB bool
	}{
		{name: "RGB Cluster", cluster: true},
		{name: "legacy OpenRGB", openRGB: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			device, runtime := newScimitarCanonicalLightingTestDevice(t)
			prepareScimitarCanonicalMutationDevice(device)
			device.DeviceProfile.RGBCluster = test.cluster
			device.DeviceProfile.OpenRGBIntegration = test.openRGB
			if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{
				SelectedEffect: "static",
				Brightness:     42,
			}); err != nil {
				t.Fatal(err)
			}
			restarts := 0
			writes := 0
			device.lightingRestart = func() { restarts++ }
			device.lightingFrameWrite = func([]byte) { writes++ }

			profile := rgb.Profile{StartColor: rgb.Color{Red: 70, Green: 80, Blue: 90}}
			if result := device.UpdateRgbProfileData("static", profile); result != 1 {
				t.Fatalf("externally owned UpdateRgbProfileData = %d, want 1", result)
			}
			stored, found, err := runtime.Effects.Get(device.Serial, "static")
			if err != nil || !found || stored.SingleColor == nil ||
				stored.SingleColor.Color != (lightingsettings.Color{Red: 70, Green: 80, Blue: 90}) {
				t.Fatalf("externally owned canonical settings = %#v, %t, %v", stored, found, err)
			}
			if restarts != 0 || writes != 0 {
				t.Fatalf("external ownership restarted %d times or wrote %d frames", restarts, writes)
			}
		})
	}
}

func TestScimitarUpdateRgbProfileDataPersistenceFailureDoesNotMutateOrRestart(t *testing.T) {
	logger.Init()
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)
	initial := lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      "static",
		SingleColor: &lightingsettings.SingleColorSettings{
			Color: lightingsettings.Color{Red: 1, Green: 2, Blue: 3},
		},
	}
	if err := runtime.Effects.Set(device.Serial, "static", initial); err != nil {
		t.Fatal(err)
	}
	legacyBefore := device.Rgb.Profiles["static"]
	writeError := errors.New("injected canonical effect-settings write failure")
	failingStore := &failingScimitarEffectSettingsStore{err: writeError}
	source := device.lightingSource.(independentDeviceLightingSource)
	source.effects = failingStore
	device.lightingSource = source
	restarts := 0
	device.lightingRestart = func() { restarts++ }
	marker := &rgb.ActiveRGB{Exit: make(chan bool, 1)}
	device.activeRgb = marker

	profile := rgb.Profile{StartColor: rgb.Color{Red: 11, Green: 22, Blue: 33}}
	if result := device.UpdateRgbProfileData("static", profile); result != 0 {
		t.Fatalf("UpdateRgbProfileData after persistence failure = %d, want 0", result)
	}
	if failingStore.setCalls != 1 || failingStore.deviceID != device.Serial || failingStore.effect != "static" {
		t.Fatalf("failed canonical Set = %#v", failingStore)
	}
	stored, found, err := runtime.Effects.Get(device.Serial, "static")
	if err != nil || !found || !reflect.DeepEqual(stored, initial) {
		t.Fatalf("failed persistence changed canonical settings to %#v, %t, %v", stored, found, err)
	}
	if restarts != 0 || device.activeRgb != marker || len(marker.Exit) != 0 {
		t.Fatal("persistence failure restarted or stopped active lighting")
	}
	if legacyAfter := device.Rgb.Profiles["static"]; !reflect.DeepEqual(legacyAfter, legacyBefore) {
		t.Fatalf("persistence failure changed legacy Static from %#v to %#v", legacyBefore, legacyAfter)
	}
}
