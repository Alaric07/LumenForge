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

func prepareScimitarCanonicalGradientMutationDevice(device *Device) {
	prepareScimitarCanonicalMutationDevice(device)
}

func setScimitarCanonicalGradientState(
	t *testing.T,
	device *Device,
	runtime *lightingsettings.IndependentDeviceRuntime,
	selected string,
) {
	t.Helper()
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{
		SelectedEffect: selected,
		Brightness:     42,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestScimitarProcessNewGradientColorPersistsCanonicalOrderedStop(t *testing.T) {
	device, runtime, effectsPath := newScimitarCanonicalLightingTestDeviceWithEffectPath(t)
	prepareScimitarCanonicalGradientMutationDevice(device)
	setScimitarCanonicalGradientState(t, device, runtime, "gradient")
	defaultGradient, err := runtime.Defaults.Get("gradient")
	if err != nil {
		t.Fatal(err)
	}
	static := lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      "static",
		SingleColor:   &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 1, Green: 2, Blue: 3}},
	}
	if err = runtime.Effects.Set(device.Serial, "static", static); err != nil {
		t.Fatal(err)
	}
	restarts := 0
	device.lightingRestart = func() { restarts++ }

	status, index := device.ProcessNewGradientColor("gradient")
	if status != 1 || index != uint(len(defaultGradient.Gradient.Stops)) {
		t.Fatalf("ProcessNewGradientColor = (%d, %d), want (1, %d)", status, index, len(defaultGradient.Gradient.Stops))
	}
	if restarts != 1 {
		t.Fatalf("active local Gradient restarts = %d, want 1", restarts)
	}
	stored, found, err := runtime.Effects.Get(device.Serial, "gradient")
	if err != nil || !found || stored.Gradient == nil {
		t.Fatalf("stored canonical Gradient = %#v, %t, %v", stored, found, err)
	}
	want := defaultGradient.Clone()
	want.Gradient.Stops = append(want.Gradient.Stops, lightingsettings.GradientStop{
		Position:  defaultGradient.Gradient.Stops[len(defaultGradient.Gradient.Stops)-1].Position,
		Color:     lightingsettings.Color{Red: 0, Green: 255, Blue: 255},
		Intensity: 0,
	})
	if !reflect.DeepEqual(stored, want) {
		t.Fatalf("stored canonical Gradient = %#v, want %#v", stored, want)
	}
	for position := 1; position < len(stored.Gradient.Stops); position++ {
		if stored.Gradient.Stops[position].Position < stored.Gradient.Stops[position-1].Position {
			t.Fatalf("canonical Gradient ordering changed: %#v", stored.Gradient.Stops)
		}
	}
	stored.Gradient.Stops[0].Color.Red = 123
	again, found, err := runtime.Effects.Get(device.Serial, "gradient")
	if err != nil || !found || again.Gradient.Stops[0].Color.Red != want.Gradient.Stops[0].Color.Red {
		t.Fatalf("stored Gradient aliases returned data: %#v, %t, %v", again, found, err)
	}
	defaultsAfter, err := runtime.Defaults.Get("gradient")
	if err != nil || !reflect.DeepEqual(defaultsAfter, defaultGradient) {
		t.Fatalf("Gradient default changed to %#v, %v", defaultsAfter, err)
	}
	state, stateFound, err := runtime.State.Resolve(device.Serial)
	if err != nil || !stateFound || state != (lightingsettings.IndependentDeviceLightingState{
		SelectedEffect: "gradient",
		Brightness:     42,
	}) {
		t.Fatalf("selected effect or brightness changed: %#v, %t, %v", state, stateFound, err)
	}
	storedStatic, staticFound, err := runtime.Effects.Get(device.Serial, "static")
	if err != nil || !staticFound || !reflect.DeepEqual(storedStatic, static) {
		t.Fatalf("unrelated Static customization = %#v, %t, %v", storedStatic, staticFound, err)
	}
	reloaded, err := lightingsettings.LoadDeviceStore(effectsPath)
	if err != nil {
		t.Fatal(err)
	}
	reloadedResolver, err := lightingsettings.NewDeviceResolver(runtime.Defaults, reloaded)
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := reloadedResolver.Resolve(lightingsettings.IndependentDevice(device.Serial), "gradient")
	if err != nil || !resolution.Customized || !reflect.DeepEqual(resolution.Settings, want) {
		t.Fatalf("reloaded canonical Gradient = %#v, %v; want %#v", resolution, err, want)
	}
}

func TestScimitarProcessDeleteGradientColorPersistsCanonicalStopRemoval(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalGradientMutationDevice(device)
	setScimitarCanonicalGradientState(t, device, runtime, "gradient")
	defaults, err := runtime.Defaults.Get("gradient")
	if err != nil {
		t.Fatal(err)
	}
	restarts := 0
	device.lightingRestart = func() { restarts++ }

	status, index := device.ProcessDeleteGradientColor("gradient")
	if status != 1 || index != uint(len(defaults.Gradient.Stops)-1) {
		t.Fatalf("ProcessDeleteGradientColor = (%d, %d), want (1, %d)", status, index, len(defaults.Gradient.Stops)-1)
	}
	if restarts != 1 {
		t.Fatalf("active local Gradient restarts = %d, want 1", restarts)
	}
	stored, found, err := runtime.Effects.Get(device.Serial, "gradient")
	want := defaults.Clone()
	want.Gradient.Stops = append([]lightingsettings.GradientStop(nil), defaults.Gradient.Stops[:len(defaults.Gradient.Stops)-1]...)
	if err != nil || !found || !reflect.DeepEqual(stored, want) {
		t.Fatalf("stored canonical Gradient after delete = %#v, %t, %v; want %#v", stored, found, err, want)
	}
}

func TestScimitarGradientMutationInactiveAndExternalOwnershipDoNotRestart(t *testing.T) {
	for _, test := range []struct {
		name     string
		selected string
		cluster  bool
		openRGB  bool
	}{
		{name: "inactive", selected: "static"},
		{name: "RGB Cluster", selected: "gradient", cluster: true},
		{name: "legacy OpenRGB", selected: "gradient", openRGB: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			device, runtime := newScimitarCanonicalLightingTestDevice(t)
			prepareScimitarCanonicalGradientMutationDevice(device)
			device.DeviceProfile.RGBCluster = test.cluster
			device.DeviceProfile.OpenRGBIntegration = test.openRGB
			setScimitarCanonicalGradientState(t, device, runtime, test.selected)
			restarts := 0
			writes := 0
			device.lightingRestart = func() { restarts++ }
			device.lightingFrameWrite = func([]byte) { writes++ }

			status, _ := device.ProcessNewGradientColor("gradient")
			if status != 1 {
				t.Fatalf("ProcessNewGradientColor = %d, want 1", status)
			}
			stored, found, err := runtime.Effects.Get(device.Serial, "gradient")
			if err != nil || !found || stored.Gradient == nil || len(stored.Gradient.Stops) != 5 {
				t.Fatalf("canonical Gradient after mutation = %#v, %t, %v", stored, found, err)
			}
			if restarts != 0 || writes != 0 {
				t.Fatalf("%s restarted %d times or wrote %d frames", test.name, restarts, writes)
			}
		})
	}
}

func TestScimitarGradientMutationRejectsInvalidAndMinimumDelete(t *testing.T) {
	logger.Init()

	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalGradientMutationDevice(device)
	setScimitarCanonicalGradientState(t, device, runtime, "gradient")
	restarts := 0
	device.lightingRestart = func() { restarts++ }

	if status, index := device.ProcessNewGradientColor("static"); status != 0 || index != 0 {
		t.Fatalf("ProcessNewGradientColor(static) = (%d, %d), want (0, 0)", status, index)
	}
	if _, found, err := runtime.Effects.Get(device.Serial, "gradient"); err != nil || found {
		t.Fatalf("invalid mutation persisted Gradient: %t, %v", found, err)
	}
	defaults, err := runtime.Defaults.Get("gradient")
	if err != nil {
		t.Fatal(err)
	}
	minimum := defaults.Clone()
	minimum.Gradient.Stops = append([]lightingsettings.GradientStop(nil), defaults.Gradient.Stops[:2]...)
	if err = runtime.Effects.Set(device.Serial, "gradient", minimum); err != nil {
		t.Fatal(err)
	}
	if status, index := device.ProcessDeleteGradientColor("gradient"); status != 2 || index != 0 {
		t.Fatalf("ProcessDeleteGradientColor at minimum = (%d, %d), want (2, 0)", status, index)
	}
	stored, found, err := runtime.Effects.Get(device.Serial, "gradient")
	if err != nil || !found || !reflect.DeepEqual(stored, minimum) {
		t.Fatalf("minimum Gradient changed to %#v, %t, %v", stored, found, err)
	}
	if restarts != 0 {
		t.Fatalf("invalid mutation restarted lighting %d times", restarts)
	}
}

func TestScimitarGradientMutationPersistenceFailureDoesNotMutateOrRestart(t *testing.T) {
	logger.Init()
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalGradientMutationDevice(device)
	setScimitarCanonicalGradientState(t, device, runtime, "gradient")
	defaults, err := runtime.Defaults.Get("gradient")
	if err != nil {
		t.Fatal(err)
	}
	failingStore := &failingScimitarEffectSettingsStore{err: errors.New("injected canonical Gradient write failure")}
	source := device.lightingSource.(independentDeviceLightingSource)
	source.effects = failingStore
	device.lightingSource = source
	restarts := 0
	device.lightingRestart = func() { restarts++ }
	marker := &rgb.ActiveRGB{Exit: make(chan bool, 1)}
	device.activeRgb = marker

	if status, index := device.ProcessNewGradientColor("gradient"); status != 0 || index != 0 {
		t.Fatalf("ProcessNewGradientColor after persistence failure = (%d, %d), want (0, 0)", status, index)
	}
	if failingStore.setCalls != 1 || failingStore.deviceID != device.Serial || failingStore.effect != "gradient" {
		t.Fatalf("failed canonical Gradient Set = %#v", failingStore)
	}
	if _, found, err := runtime.Effects.Get(device.Serial, "gradient"); err != nil || found {
		t.Fatalf("failed persistence changed canonical Gradient: %t, %v", found, err)
	}
	defaultsAfter, err := runtime.Defaults.Get("gradient")
	if err != nil || !reflect.DeepEqual(defaultsAfter, defaults) {
		t.Fatalf("failed persistence changed Gradient default: %#v, %v", defaultsAfter, err)
	}
	if restarts != 0 || device.activeRgb != marker || len(marker.Exit) != 0 {
		t.Fatal("persistence failure restarted or stopped active lighting")
	}
}
