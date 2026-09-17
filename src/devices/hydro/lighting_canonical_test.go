package hydro

import (
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
)

func newHydroCanonicalLightingTestDevice(t *testing.T) (*Device, *lightingsettings.IndependentDeviceRuntime) {
	t.Helper()
	root := t.TempDir()
	paths := config.Paths{
		OpenRGBDeviceLightingFile: filepath.Join(root, "state.json"),
		DeviceEffectSettingsFile:  filepath.Join(root, "effects.json"),
		ShippedDatabaseRoot:       filepath.Join("..", "..", "..", "database"),
	}
	d := &Device{Serial: "hydro-canonical-test"}
	if err := d.attachIndependentDeviceLightingRuntime(paths); err != nil {
		t.Fatal(err)
	}
	return d, d.lightingRuntime
}

func TestHydroCanonicalLightingInitializesFixedStaticState(t *testing.T) {
	d, runtime := newHydroCanonicalLightingTestDevice(t)
	state, found, err := runtime.State.Resolve(d.Serial)
	if err != nil || !found || state != (lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 100}) {
		t.Fatalf("state = %#v, %t, %v", state, found, err)
	}
	if _, ok := interface{}(d).(interface{ SetLightingEffect(string) error }); ok {
		t.Fatal("Hydro unexpectedly exposes effect selection")
	}
	if _, ok := interface{}(d).(interface{ ProcessSetRgbCluster(bool) uint8 }); ok {
		t.Fatal("Hydro unexpectedly exposes RGB Cluster")
	}
	if _, ok := interface{}(d).(interface{ ProcessSetOpenRgbIntegration(bool) uint8 }); ok {
		t.Fatal("Hydro unexpectedly exposes OpenRGB integration")
	}
}

func TestHydroCanonicalLightingFailsClosedForNonStaticState(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{OpenRGBDeviceLightingFile: filepath.Join(root, "state.json"), DeviceEffectSettingsFile: filepath.Join(root, "effects.json"), ShippedDatabaseRoot: filepath.Join("..", "..", "..", "database")}
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile, filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = runtime.State.Set("hydro-invalid-state", lightingsettings.IndependentDeviceLightingState{SelectedEffect: "rainbow", Brightness: 100}); err != nil {
		t.Fatal(err)
	}
	if err = (&Device{Serial: "hydro-invalid-state"}).attachIndependentDeviceLightingRuntime(paths); err == nil {
		t.Fatal("non-static state attached")
	}
}

func TestHydroCanonicalStaticColorBrightnessAndScheduler(t *testing.T) {
	d, runtime := newHydroCanonicalLightingTestDevice(t)
	var writes [][]byte
	d.configurationWrite = func(data []byte) { writes = append(writes, data) }
	settings, err := d.ResolveLightingEffectSettings("static")
	if err != nil {
		t.Fatal(err)
	}
	settings.SingleColor = &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 200, Green: 100, Blue: 50}}
	if err := d.SetLightingEffectSettings("static", settings); err != nil {
		t.Fatal(err)
	}
	if err := d.SetLightingBrightness(50); err != nil {
		t.Fatal(err)
	}
	if len(writes) != 2 || !reflect.DeepEqual(writes[1][:3], []byte{100, 50, 25}) {
		t.Fatalf("writes = %#v", writes)
	}
	if d.SchedulerBrightness(0) != 1 || len(writes) != 3 || !reflect.DeepEqual(writes[2][:3], []byte{0, 0, 0}) {
		t.Fatalf("dark scheduler write = %#v", writes)
	}
	if err := d.SetLightingBrightness(25); err != nil {
		t.Fatal(err)
	}
	state, _, err := runtime.State.Resolve(d.Serial)
	if err != nil || state.Brightness != 25 || !reflect.DeepEqual(writes[3][:3], []byte{0, 0, 0}) {
		t.Fatalf("manual dark update = %#v, %#v, %v", state, writes, err)
	}
	if d.SchedulerBrightness(1) != 1 || len(writes) != 5 || reflect.DeepEqual(writes[4][:3], []byte{0, 0, 0}) {
		t.Fatalf("scheduler release = %#v", writes)
	}
	if err = d.ResetLightingEffectSettings("static"); err != nil {
		t.Fatal(err)
	}
	if _, customized, err := runtime.Effects.Get(d.Serial, "static"); err != nil || customized {
		t.Fatalf("static customization after reset = %t, %v", customized, err)
	}
	state, _, err = runtime.State.Resolve(d.Serial)
	if err != nil || state.Brightness != 25 {
		t.Fatalf("brightness after reset = %#v, %v", state, err)
	}
}

func TestHydroSchedulerReleaseSerializesWithManualCanonicalBrightness(t *testing.T) {
	d, runtime := newHydroCanonicalLightingTestDevice(t)
	d.configurationWrite = func([]byte) {}
	if d.SchedulerBrightness(0) != 1 {
		t.Fatal("unable to enable scheduler darkness")
	}
	entered, unblock := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var writesMu sync.Mutex
	var writes [][]byte
	d.configurationWrite = func(data []byte) {
		copy := append([]byte(nil), data...)
		writesMu.Lock()
		writes = append(writes, copy)
		writesMu.Unlock()
		once.Do(func() { close(entered); <-unblock })
	}
	released := make(chan uint8, 1)
	go func() { released <- d.SchedulerBrightness(1) }()
	<-entered
	manual := make(chan error, 1)
	go func() { manual <- d.SetLightingBrightness(25) }()
	select {
	case err := <-manual:
		t.Fatalf("manual brightness completed before scheduler release: %v", err)
	default:
	}
	close(unblock)
	if result := <-released; result != 1 {
		t.Fatalf("scheduler release = %d", result)
	}
	if err := <-manual; err != nil {
		t.Fatal(err)
	}
	state, _, err := runtime.State.Resolve(d.Serial)
	if err != nil || state.Brightness != 25 {
		t.Fatalf("canonical state = %#v, %v", state, err)
	}
	expected, ok := d.canonicalConfigurationColor()
	if !ok {
		t.Fatal("unable to resolve final canonical output")
	}
	writesMu.Lock()
	defer writesMu.Unlock()
	if len(writes) != 2 || !reflect.DeepEqual(writes[len(writes)-1][:3], expected[:]) {
		t.Fatalf("writes = %#v, want final %#v", writes, expected)
	}
}
