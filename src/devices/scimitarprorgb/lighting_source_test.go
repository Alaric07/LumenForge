package scimitarprorgb

import (
	"bytes"
	"errors"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
)

type scimitarTemplateTestDashboard struct {
	TemperatureBar bool
}

type scimitarTemplateTestPage struct {
	Device       *Device
	Devices      interface{}
	Rgb          map[string]rgb.Profile
	Macros       interface{}
	Temperatures interface{}
	Dashboard    scimitarTemplateTestDashboard
}

func (scimitarTemplateTestPage) Lang(string) string {
	return ""
}

type failingScimitarLightingState struct {
	state    lightingsettings.IndependentDeviceLightingState
	setError error
	setCalls int
}

func (state *failingScimitarLightingState) Resolve(string) (lightingsettings.IndependentDeviceLightingState, bool, error) {
	return state.state, true, nil
}

func (state *failingScimitarLightingState) Set(string, lightingsettings.IndependentDeviceLightingState) error {
	state.setCalls++
	return state.setError
}

func TestScimitarCanonicalLightingSourceRendersDefaultStatic(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	device.DeviceProfile = &DeviceProfile{RGBProfile: "mouse"}
	resolved, err := device.resolveCanonicalLighting()
	if err != nil {
		t.Fatal(err)
	}
	if resolved.selectedEffect != "static" || resolved.brightness != 100 || resolved.settings.SingleColor == nil {
		t.Fatalf("default canonical lighting = %#v", resolved)
	}

	profile := lightingsettings.RendererProfileFromEffectSettings(resolved.settings)
	logicalFrame := composeScimitarStaticLogicalFrame(profile, resolved.brightness)
	zoneColors, dpi := scimitarTestFrameLayout()
	hardwareFrame := newScimitarLightingAdapter(scimitarTestLEDChannels, zoneColors, dpi).
		composeScimitarHardwareFrame(logicalFrame, rgb.Color{Red: 1, Green: 2, Blue: 3})
	want := []byte{
		1, 0, 255, 255,
		2, 0, 255, 255,
		3, 1, 2, 3,
		4, 0, 255, 255,
		5, 0, 255, 255,
	}
	if !bytes.Equal(hardwareFrame, want) {
		t.Fatalf("default canonical Static frame = %v, want %v", hardwareFrame, want)
	}

	if _, found, resolveErr := runtime.State.Resolve(device.Serial); resolveErr != nil || found {
		t.Fatalf("default canonical state was persisted: found=%t err=%v", found, resolveErr)
	}
	if _, err = os.Stat(scimitarCanonicalStatePath(t, runtime)); !os.IsNotExist(err) {
		t.Fatalf("resolving default canonical state created persistence: %v", err)
	}
}

func TestScimitarCanonicalLightingSourceRendersCustomizedStaticAtCanonicalBrightness(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	state := lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 50}
	if err := runtime.State.Set(device.Serial, state); err != nil {
		t.Fatal(err)
	}
	settings := lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      "static",
		SingleColor: &lightingsettings.SingleColorSettings{
			Color: lightingsettings.Color{Red: 200, Green: 100, Blue: 50},
		},
	}
	if err := runtime.Effects.Set(device.Serial, "static", settings); err != nil {
		t.Fatal(err)
	}

	resolved, err := device.resolveCanonicalLighting()
	if err != nil {
		t.Fatal(err)
	}
	profile := lightingsettings.RendererProfileFromEffectSettings(resolved.settings)
	logicalFrame := composeScimitarStaticLogicalFrame(profile, resolved.brightness)
	wantLogical := [scimitarEffectZoneCount][scimitarRGBChannelsPerZone]byte{
		{100, 50, 25},
		{100, 50, 25},
		{100, 50, 25},
		{100, 50, 25},
	}
	if logicalFrame.zones != wantLogical {
		t.Fatalf("custom canonical Static frame = %v, want %v", logicalFrame.zones, wantLogical)
	}

	resolved.settings.SingleColor.Color.Red = 1
	again, err := device.resolveCanonicalLighting()
	if err != nil || again.settings.SingleColor == nil || again.settings.SingleColor.Color.Red != 200 {
		t.Fatalf("canonical source returned aliased settings: %#v, %v", again, err)
	}
}

func TestScimitarTemplateUsesCanonicalSelectedEffect(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)
	device.RGBModes = []string{"rainbow", "static"}
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{
		SelectedEffect: "rainbow",
		Brightness:     100,
	}); err != nil {
		t.Fatal(err)
	}

	rendered := renderScimitarTemplateForTest(t, device)
	if !strings.Contains(rendered, `<option value="0;rainbow" selected>rainbow</option>`) {
		t.Fatalf("canonical Rainbow option was not selected:\n%s", rendered)
	}

	device.DeviceProfile.RGBProfile = "static"
	rendered = renderScimitarTemplateForTest(t, device)
	if !strings.Contains(rendered, `<option value="0;rainbow" selected>rainbow</option>`) {
		t.Fatalf("legacy RGBProfile changed the canonical Rainbow selection:\n%s", rendered)
	}
	if strings.Contains(rendered, `<option value="0;static" selected>static</option>`) {
		t.Fatalf("legacy Static profile was rendered as selected:\n%s", rendered)
	}
}

func TestScimitarTemplateResolvesUnstoredCanonicalDefault(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)
	device.DeviceProfile.RGBProfile = "rainbow"
	device.RGBModes = []string{"rainbow", "static"}

	rendered := renderScimitarTemplateForTest(t, device)
	if !strings.Contains(rendered, `<option value="0;static" selected>static</option>`) {
		t.Fatalf("unstored canonical Static default was not selected:\n%s", rendered)
	}
	if strings.Contains(rendered, `<option value="0;rainbow" selected>rainbow</option>`) {
		t.Fatalf("legacy Rainbow profile was rendered as selected:\n%s", rendered)
	}
	if _, found, err := runtime.State.Resolve(device.Serial); err != nil || found {
		t.Fatalf("template default resolution persisted state: found=%t err=%v", found, err)
	}
	if _, err := os.Stat(scimitarCanonicalStatePath(t, runtime)); !os.IsNotExist(err) {
		t.Fatalf("template default resolution created persistence: %v", err)
	}
}

func TestScimitarUpdateRgbProfilePersistsCanonicalSelectedEffect(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)
	initial := lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 42}
	if err := runtime.State.Set(device.Serial, initial); err != nil {
		t.Fatal(err)
	}
	restarts := 0
	device.lightingRestart = func() { restarts++ }

	if result := device.UpdateRgbProfile(-1, "rainbow"); result != 1 {
		t.Fatalf("UpdateRgbProfile(rainbow) = %d, want 1", result)
	}
	state, found, err := runtime.State.Resolve(device.Serial)
	if err != nil || !found || state != (lightingsettings.IndependentDeviceLightingState{
		SelectedEffect: "rainbow",
		Brightness:     42,
	}) {
		t.Fatalf("persisted canonical state = %#v, %t, %v", state, found, err)
	}
	resolved, err := device.resolveCanonicalLighting()
	if err != nil || resolved.selectedEffect != "rainbow" || resolved.settings.EffectID != "rainbow" {
		t.Fatalf("resolved canonical lighting = %#v, %v", resolved, err)
	}
	if current := device.GetCurrentRgbProfile(); current != "rainbow" {
		t.Fatalf("current canonical RGB profile = %q, want rainbow", current)
	}
	if device.DeviceProfile.RGBProfile != "mouse" {
		t.Fatalf("legacy RGBProfile changed to %q, want mouse", device.DeviceProfile.RGBProfile)
	}
	if restarts != 1 {
		t.Fatalf("canonical lighting restarts = %d, want 1", restarts)
	}

	reloaded, err := lightingsettings.LoadIndependentDeviceStateStore(scimitarCanonicalStatePath(t, runtime))
	if err != nil {
		t.Fatal(err)
	}
	reloadedState, reloadedFound, err := reloaded.Resolve(device.Serial)
	if err != nil || !reloadedFound || reloadedState != state {
		t.Fatalf("reloaded canonical state = %#v, %t, %v; want %#v", reloadedState, reloadedFound, err, state)
	}
}

func TestScimitarUpdateRgbProfilePersistenceFailureDoesNotRestartLighting(t *testing.T) {
	logger.Init()
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)
	restarts := 0
	device.lightingRestart = func() { restarts++ }
	writeError := errors.New("injected canonical state write failure")
	failingState := &failingScimitarLightingState{
		state:    lightingsettings.DefaultIndependentDeviceLightingState(),
		setError: writeError,
	}
	device.lightingSource = independentDeviceLightingSource{
		deviceID: device.Serial,
		state:    failingState,
		resolver: runtime.Resolver,
	}
	marker := &rgb.ActiveRGB{Exit: make(chan bool, 1)}
	device.activeRgb = marker

	if result := device.UpdateRgbProfile(-1, "rainbow"); result != 0 {
		t.Fatalf("UpdateRgbProfile with persistence failure = %d, want 0", result)
	}
	if failingState.setCalls != 1 {
		t.Fatalf("canonical state Set calls = %d, want 1", failingState.setCalls)
	}
	if restarts != 0 || device.activeRgb != marker || len(marker.Exit) != 0 {
		t.Fatal("persistence failure restarted or stopped active lighting")
	}
	if device.DeviceProfile.RGBProfile != "mouse" {
		t.Fatalf("persistence failure changed legacy RGBProfile to %q", device.DeviceProfile.RGBProfile)
	}
}

func TestScimitarUpdateRgbProfilePreservesOwnershipRejections(t *testing.T) {
	for _, test := range []struct {
		name       string
		cluster    bool
		openRGB    bool
		wantResult uint8
	}{
		{name: "RGB Cluster", cluster: true, wantResult: 5},
		{name: "legacy OpenRGB", openRGB: true, wantResult: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			device, runtime := newScimitarCanonicalLightingTestDevice(t)
			prepareScimitarCanonicalMutationDevice(device)
			device.DeviceProfile.RGBCluster = test.cluster
			device.DeviceProfile.OpenRGBIntegration = test.openRGB
			state := &failingScimitarLightingState{
				state:    lightingsettings.DefaultIndependentDeviceLightingState(),
				setError: errors.New("ownership rejection reached persistence"),
			}
			device.lightingSource = independentDeviceLightingSource{
				deviceID: device.Serial,
				state:    state,
				resolver: runtime.Resolver,
			}
			restarts := 0
			device.lightingRestart = func() { restarts++ }

			if result := device.UpdateRgbProfile(-1, "rainbow"); result != test.wantResult {
				t.Fatalf("UpdateRgbProfile while %s-owned = %d, want %d", test.name, result, test.wantResult)
			}
			if state.setCalls != 0 {
				t.Fatalf("%s ownership reached canonical persistence %d times", test.name, state.setCalls)
			}
			if restarts != 0 {
				t.Fatalf("%s ownership restarted canonical lighting %d times", test.name, restarts)
			}
			if device.DeviceProfile.RGBProfile != "mouse" {
				t.Fatalf("%s ownership changed legacy RGBProfile to %q", test.name, device.DeviceProfile.RGBProfile)
			}
		})
	}
}

func TestScimitarUpdateRgbProfileRejectsLegacyMouseMode(t *testing.T) {
	logger.Init()
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)
	restarts := 0
	device.lightingRestart = func() { restarts++ }

	if result := device.UpdateRgbProfile(-1, "mouse"); result != 0 {
		t.Fatalf("UpdateRgbProfile(mouse) = %d, want 0", result)
	}
	state, found, err := runtime.State.Resolve(device.Serial)
	if err != nil || found || state != lightingsettings.DefaultIndependentDeviceLightingState() {
		t.Fatalf("state after legacy mouse selection = %#v, %t, %v", state, found, err)
	}
	if restarts != 0 {
		t.Fatalf("legacy mouse selection restarted canonical lighting %d times", restarts)
	}
	profiles, ok := device.GetRgbProfiles().(rgb.RGB)
	if !ok {
		t.Fatalf("Scimitar RGB profiles = %T", device.GetRgbProfiles())
	}
	if _, exposed := profiles.Profiles["mouse"]; exposed {
		t.Fatal("legacy mouse mode remains exposed as a selectable effect")
	}
}

func prepareScimitarCanonicalMutationDevice(device *Device) {
	brightness := uint8(100)
	device.Connected = true
	device.DeviceProfile = &DeviceProfile{
		RGBProfile:       "mouse",
		BrightnessSlider: &brightness,
		RgbOff:           true,
		Profile:          0,
		Profiles: map[int]DPIProfile{
			0: {Color: &rgb.Color{}},
		},
	}
	device.Rgb = &rgb.RGB{Profiles: map[string]rgb.Profile{
		"mouse":   {},
		"off":     {},
		"rainbow": {},
		"static":  {},
	}}
	device.Exit = true
}

func newScimitarCanonicalLightingTestDevice(
	t *testing.T,
) (*Device, *lightingsettings.IndependentDeviceRuntime) {
	t.Helper()
	root := t.TempDir()
	defaultsPath, err := filepath.Abs(filepath.Join("..", "..", "..", "database"))
	if err != nil {
		t.Fatal(err)
	}
	paths := config.Paths{
		OpenRGBDeviceLightingFile: filepath.Join(root, "independent-device-state.json"),
		DeviceEffectSettingsFile:  filepath.Join(root, "independent-device-effects.json"),
		ShippedDatabaseRoot:       defaultsPath,
	}
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(
		paths.OpenRGBDeviceLightingFile,
		paths.DeviceEffectSettingsFile,
		filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	device := &Device{Serial: "scimitar-canonical-read"}
	if err = device.attachIndependentDeviceLightingSource(runtime); err != nil {
		t.Fatal(err)
	}
	if _, ok := device.lightingSource.(independentDeviceLightingSource); !ok {
		t.Fatalf("Scimitar canonical source = %#v", device.lightingSource)
	}
	return device, runtime
}

func renderScimitarTemplateForTest(t *testing.T, device *Device) string {
	t.Helper()
	pageTemplate, err := template.New("scimitar-template-test").Parse(`
		{{ define "head" }}{{ end }}
		{{ define "sidebar" }}{{ end }}
		{{ define "temperature-bar" }}{{ end }}
		{{ define "404-no-device" }}{{ end }}
	`)
	if err != nil {
		t.Fatal(err)
	}
	templatePath := filepath.Join("..", "..", "..", "web", "scimitarprorgb.html")
	pageTemplate, err = pageTemplate.ParseFiles(templatePath)
	if err != nil {
		t.Fatal(err)
	}

	var rendered bytes.Buffer
	if err = pageTemplate.ExecuteTemplate(&rendered, "scimitarprorgb.html", scimitarTemplateTestPage{Device: device}); err != nil {
		t.Fatal(err)
	}
	return rendered.String()
}

func scimitarCanonicalStatePath(t *testing.T, runtime *lightingsettings.IndependentDeviceRuntime) string {
	t.Helper()
	store, ok := runtime.State.(*lightingsettings.IndependentDeviceStateStore)
	if !ok {
		t.Fatalf("Scimitar canonical state store = %T", runtime.State)
	}
	return store.Path()
}
