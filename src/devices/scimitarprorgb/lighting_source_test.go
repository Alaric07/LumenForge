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
	lastSet  lightingsettings.IndependentDeviceLightingState
}

func (state *failingScimitarLightingState) Resolve(string) (lightingsettings.IndependentDeviceLightingState, bool, error) {
	return state.state, true, nil
}

func (state *failingScimitarLightingState) Set(_ string, value lightingsettings.IndependentDeviceLightingState) error {
	state.setCalls++
	state.lastSet = value
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
	if brightness := device.GetCurrentBrightness(); brightness != 100 {
		t.Fatalf("default canonical brightness = %d, want 100", brightness)
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
	legacyBrightness := uint8(12)
	device.DeviceProfile.BrightnessSlider = &legacyBrightness
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{
		SelectedEffect: "rainbow",
		Brightness:     37,
	}); err != nil {
		t.Fatal(err)
	}

	rendered := renderScimitarTemplateForTest(t, device)
	if !strings.Contains(rendered, `<option value="0;rainbow" selected>rainbow</option>`) {
		t.Fatalf("canonical Rainbow option was not selected:\n%s", rendered)
	}
	if !strings.Contains(rendered, `id="brightnessSlider" name="brightnessSlider" min="0" max="100" value="37"`) ||
		!strings.Contains(rendered, `id="brightnessSliderValue">37 %</div>`) {
		t.Fatalf("canonical brightness was not rendered into the legacy page:\n%s", rendered)
	}
	if strings.Contains(rendered, `value="12"`) || strings.Contains(rendered, `>12 %</div>`) {
		t.Fatalf("legacy BrightnessSlider leaked into the rendered page:\n%s", rendered)
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
	if !strings.Contains(rendered, `id="brightnessSlider" name="brightnessSlider" min="0" max="100" value="100"`) ||
		!strings.Contains(rendered, `id="brightnessSliderValue">100 %</div>`) {
		t.Fatalf("unstored canonical brightness default was not rendered:\n%s", rendered)
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

func TestScimitarChangeDeviceBrightnessValuePersistsCanonicalStateAndRestartsLocalLighting(t *testing.T) {
	for _, effect := range []string{"static", "rainbow"} {
		t.Run(effect, func(t *testing.T) {
			device, runtime := newScimitarCanonicalLightingTestDevice(t)
			prepareScimitarCanonicalMutationDevice(device)
			legacyBrightness := uint8(91)
			device.DeviceProfile.BrightnessSlider = &legacyBrightness
			if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{
				SelectedEffect: effect,
				Brightness:     42,
			}); err != nil {
				t.Fatal(err)
			}
			restarts := 0
			device.lightingRestart = func() { restarts++ }

			if result := device.ChangeDeviceBrightnessValue(65); result != 1 {
				t.Fatalf("ChangeDeviceBrightnessValue(65) = %d, want 1", result)
			}
			state, found, err := runtime.State.Resolve(device.Serial)
			if err != nil || !found || state != (lightingsettings.IndependentDeviceLightingState{
				SelectedEffect: effect,
				Brightness:     65,
			}) {
				t.Fatalf("canonical state after brightness mutation = %#v, %t, %v", state, found, err)
			}
			if device.DeviceProfile.BrightnessSlider == nil || *device.DeviceProfile.BrightnessSlider != 91 {
				t.Fatalf("legacy BrightnessSlider changed to %v, want 91", device.DeviceProfile.BrightnessSlider)
			}
			if restarts != 1 {
				t.Fatalf("local %s restart count = %d, want 1", effect, restarts)
			}

			reloaded, err := lightingsettings.LoadIndependentDeviceStateStore(scimitarCanonicalStatePath(t, runtime))
			if err != nil {
				t.Fatal(err)
			}
			reloadedState, reloadedFound, err := reloaded.Resolve(device.Serial)
			if err != nil || !reloadedFound || reloadedState != state {
				t.Fatalf("reloaded brightness state = %#v, %t, %v; want %#v", reloadedState, reloadedFound, err, state)
			}
		})
	}
}

func TestScimitarChangeDeviceBrightnessValueFailureDoesNotMutateOrRestart(t *testing.T) {
	logger.Init()
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)
	legacyBrightness := uint8(91)
	device.DeviceProfile.BrightnessSlider = &legacyBrightness
	initial := lightingsettings.IndependentDeviceLightingState{SelectedEffect: "rainbow", Brightness: 42}
	writeError := errors.New("injected canonical brightness write failure")
	failingState := &failingScimitarLightingState{state: initial, setError: writeError}
	device.lightingSource = independentDeviceLightingSource{
		deviceID: device.Serial,
		state:    failingState,
		resolver: runtime.Resolver,
	}
	restarts := 0
	device.lightingRestart = func() { restarts++ }
	marker := &rgb.ActiveRGB{Exit: make(chan bool, 1)}
	device.activeRgb = marker

	if result := device.ChangeDeviceBrightnessValue(65); result != 0 {
		t.Fatalf("ChangeDeviceBrightnessValue with persistence failure = %d, want 0", result)
	}
	if failingState.setCalls != 1 || failingState.lastSet != (lightingsettings.IndependentDeviceLightingState{
		SelectedEffect: "rainbow",
		Brightness:     65,
	}) {
		t.Fatalf("failed canonical Set = %#v after %d calls", failingState.lastSet, failingState.setCalls)
	}
	if failingState.state != initial {
		t.Fatalf("failed persistence changed stored state to %#v, want %#v", failingState.state, initial)
	}
	if restarts != 0 || device.activeRgb != marker || len(marker.Exit) != 0 {
		t.Fatal("persistence failure restarted or stopped active lighting")
	}
	if device.DeviceProfile.BrightnessSlider == nil || *device.DeviceProfile.BrightnessSlider != 91 {
		t.Fatalf("persistence failure changed legacy BrightnessSlider to %v", device.DeviceProfile.BrightnessSlider)
	}

	failingState.setCalls = 0
	if result := device.ChangeDeviceBrightnessValue(101); result != 0 {
		t.Fatalf("ChangeDeviceBrightnessValue(101) = %d, want 0", result)
	}
	if failingState.setCalls != 0 || restarts != 0 {
		t.Fatalf("invalid brightness reached persistence %d times or restarted %d times", failingState.setCalls, restarts)
	}
}

func TestScimitarChangeDeviceBrightnessValuePreservesExternalOwnership(t *testing.T) {
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
			legacyBrightness := uint8(91)
			device.DeviceProfile.BrightnessSlider = &legacyBrightness
			if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{
				SelectedEffect: "rainbow",
				Brightness:     42,
			}); err != nil {
				t.Fatal(err)
			}
			restarts := 0
			device.lightingRestart = func() { restarts++ }

			if result := device.ChangeDeviceBrightnessValue(65); result != 1 {
				t.Fatalf("externally owned brightness mutation = %d, want 1", result)
			}
			state, found, err := runtime.State.Resolve(device.Serial)
			if err != nil || !found || state.SelectedEffect != "rainbow" || state.Brightness != 65 {
				t.Fatalf("externally owned canonical state = %#v, %t, %v", state, found, err)
			}
			if restarts != 0 {
				t.Fatalf("%s-owned brightness restarted local output %d times", test.name, restarts)
			}
			if device.DeviceProfile.BrightnessSlider == nil || *device.DeviceProfile.BrightnessSlider != 91 {
				t.Fatalf("%s-owned mutation changed legacy BrightnessSlider to %v", test.name, device.DeviceProfile.BrightnessSlider)
			}
		})
	}
}

func TestScimitarSchedulerBrightnessUsesTransientEffectiveOverride(t *testing.T) {
	for _, effect := range []string{"static", "rainbow"} {
		t.Run(effect, func(t *testing.T) {
			device, runtime := newScimitarCanonicalLightingTestDevice(t)
			prepareScimitarCanonicalMutationDevice(device)
			legacyBrightness := uint8(91)
			device.DeviceProfile.BrightnessSlider = &legacyBrightness
			device.DeviceProfile.OriginalBrightness = 73
			state := lightingsettings.IndependentDeviceLightingState{
				SelectedEffect: effect,
				Brightness:     100,
			}
			if err := runtime.State.Set(device.Serial, state); err != nil {
				t.Fatal(err)
			}
			statePath := scimitarCanonicalStatePath(t, runtime)
			persistedBefore, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			restarts := 0
			device.lightingRestart = func() { restarts++ }

			if brightness, err := device.effectiveBrightness(); err != nil || brightness != 100 {
				t.Fatalf("effective brightness before scheduler = %d, %v; want 100", brightness, err)
			}
			if result := device.SchedulerBrightness(0); result != 1 {
				t.Fatalf("SchedulerBrightness(0) = %d, want 1", result)
			}
			resolved, err := device.resolveEffectiveCanonicalLighting()
			if err != nil || resolved.brightness != 0 {
				t.Fatalf("effective lighting under scheduler override = %#v, %v", resolved, err)
			}
			if effect == "static" {
				profile := lightingsettings.RendererProfileFromEffectSettings(resolved.settings)
				if frame := composeScimitarStaticLogicalFrame(profile, resolved.brightness); frame != (scimitarLogicalFrame{}) {
					t.Fatalf("Static logical frame under scheduler override = %#v, want black", frame)
				}
			}
			if desired := device.GetCurrentBrightness(); desired != 100 {
				t.Fatalf("canonical desired brightness while scheduled dark = %d, want 100", desired)
			}
			if restarts != 1 {
				t.Fatalf("local %s scheduler-off restart count = %d, want 1", effect, restarts)
			}
			if result := device.SchedulerBrightness(0); result != 1 || restarts != 1 {
				t.Fatalf("repeated scheduler-off result/restarts = %d/%d, want 1/1", result, restarts)
			}

			if result := device.SchedulerBrightness(1); result != 1 {
				t.Fatalf("SchedulerBrightness(1) = %d, want 1", result)
			}
			if brightness, err := device.effectiveBrightness(); err != nil || brightness != 100 {
				t.Fatalf("effective brightness after scheduler restore = %d, %v; want 100", brightness, err)
			}
			if restarts != 2 {
				t.Fatalf("local %s scheduler-restore restart count = %d, want 2", effect, restarts)
			}
			persistedAfter, err := os.ReadFile(statePath)
			if err != nil || !bytes.Equal(persistedAfter, persistedBefore) {
				t.Fatalf("scheduler override changed canonical persistence: %v", err)
			}
			if device.DeviceProfile.BrightnessSlider == nil || *device.DeviceProfile.BrightnessSlider != 91 ||
				device.DeviceProfile.OriginalBrightness != 73 {
				t.Fatalf("scheduler changed legacy brightness fields: slider=%v original=%d",
					device.DeviceProfile.BrightnessSlider, device.DeviceProfile.OriginalBrightness)
			}
		})
	}
}

func TestScimitarManualBrightnessWhileScheduledDarkUpdatesDesiredOnly(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)
	device.RGBModes = []string{"static"}
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{
		SelectedEffect: "static",
		Brightness:     100,
	}); err != nil {
		t.Fatal(err)
	}
	restarts := 0
	device.lightingRestart = func() { restarts++ }

	device.SchedulerBrightness(0)
	if result := device.ChangeDeviceBrightnessValue(40); result != 1 {
		t.Fatalf("ChangeDeviceBrightnessValue(40) while dark = %d, want 1", result)
	}
	state, found, err := runtime.State.Resolve(device.Serial)
	if err != nil || !found || state.Brightness != 40 || state.SelectedEffect != "static" {
		t.Fatalf("canonical state after manual change while dark = %#v, %t, %v", state, found, err)
	}
	if desired := device.GetCurrentBrightness(); desired != 40 {
		t.Fatalf("desired brightness while dark = %d, want 40", desired)
	}
	if effective, err := device.effectiveBrightness(); err != nil || effective != 0 {
		t.Fatalf("effective brightness after manual change while dark = %d, %v; want 0", effective, err)
	}
	if restarts != 2 {
		t.Fatalf("scheduler-off plus manual-change restart count = %d, want 2", restarts)
	}
	rendered := renderScimitarTemplateForTest(t, device)
	if !strings.Contains(rendered, `id="brightnessSlider" name="brightnessSlider" min="0" max="100" value="40"`) ||
		strings.Contains(rendered, `id="brightnessSlider" name="brightnessSlider" min="0" max="100" value="0"`) {
		t.Fatalf("template did not preserve desired brightness while effective output was dark:\n%s", rendered)
	}

	device.SchedulerBrightness(1)
	if effective, err := device.effectiveBrightness(); err != nil || effective != 40 {
		t.Fatalf("effective brightness after scheduler restore = %d, %v; want 40", effective, err)
	}
	if restarts != 3 {
		t.Fatalf("scheduler restore restart count = %d, want 3", restarts)
	}
}

func TestScimitarSchedulerBrightnessPreservesExternalZoneOwnership(t *testing.T) {
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
			device.Exit = false
			device.DeviceProfile.RGBCluster = test.cluster
			device.DeviceProfile.OpenRGBIntegration = test.openRGB
			zoneColors, dpi := scimitarTestFrameLayout()
			dpi.Color = &rgb.Color{Red: 120, Green: 60, Blue: 6}
			device.LEDChannels = scimitarTestLEDChannels
			device.DeviceProfile.ZoneColors = zoneColors
			device.DeviceProfile.Profiles = map[int]DPIProfile{0: dpi}
			if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{
				SelectedEffect: "rainbow",
				Brightness:     75,
			}); err != nil {
				t.Fatal(err)
			}
			restarts := 0
			device.lightingRestart = func() { restarts++ }
			var writes [][]byte
			device.lightingFrameWrite = func(frame []byte) {
				writes = append(writes, append([]byte(nil), frame...))
			}
			owner := scimitarExternalFrameOpenRGB
			if test.cluster {
				owner = scimitarExternalFrameCluster
			}
			externalZones := []byte{200, 100, 50, 180, 90, 30, 160, 80, 20, 140, 70, 10}
			initialFrame, ok := device.composeAndCacheExternallyOwnedFrame(owner, externalZones, 75)
			if !ok {
				t.Fatalf("%s external frame was not accepted", test.name)
			}
			dpiBytes := scimitarTestBrightnessBytes(rgb.Color{Red: 120, Green: 60, Blue: 6}, 75)
			wantInitial := []byte{
				1, 200, 100, 50,
				2, 140, 70, 10,
				3, dpiBytes[0], dpiBytes[1], dpiBytes[2],
				4, 180, 90, 30,
				5, 160, 80, 20,
			}
			if !bytes.Equal(initialFrame, wantInitial) {
				t.Fatalf("%s initial external frame = %v, want %v", test.name, initialFrame, wantInitial)
			}
			externalZones[0] = 1

			device.SchedulerBrightness(0)
			effective, err := device.effectiveBrightness()
			if err != nil || effective != 0 {
				t.Fatalf("%s effective brightness while dark = %d, %v", test.name, effective, err)
			}
			if len(writes) != 1 {
				t.Fatalf("%s immediate scheduler-off write count = %d, want 1", test.name, len(writes))
			}
			wantDark := []byte{
				1, 200, 100, 50,
				2, 140, 70, 10,
				3, 0, 0, 0,
				4, 180, 90, 30,
				5, 160, 80, 20,
			}
			if !bytes.Equal(writes[0], wantDark) {
				t.Fatalf("%s immediate dark frame = %v, want %v", test.name, writes[0], wantDark)
			}
			if restarts != 0 {
				t.Fatalf("%s scheduler transition restarted local output %d times", test.name, restarts)
			}

			device.SchedulerBrightness(1)
			if effective, err = device.effectiveBrightness(); err != nil || effective != 75 {
				t.Fatalf("%s effective brightness after restore = %d, %v; want 75", test.name, effective, err)
			}
			if len(writes) != 2 {
				t.Fatalf("%s immediate scheduler-restore write count = %d, want 2", test.name, len(writes))
			}
			wantRestored := append([]byte(nil), wantDark...)
			copy(wantRestored[9:12], dpiBytes)
			if !bytes.Equal(writes[1], wantRestored) {
				t.Fatalf("%s immediate restored frame = %v, want %v", test.name, writes[1], wantRestored)
			}
			if restarts != 0 {
				t.Fatalf("%s scheduler restore restarted local output %d times", test.name, restarts)
			}
			state, found, err := runtime.State.Resolve(device.Serial)
			if err != nil || !found || state.Brightness != 75 {
				t.Fatalf("%s scheduler refresh changed canonical state: %#v, %t, %v", test.name, state, found, err)
			}
		})
	}
}

func TestScimitarSchedulerBrightnessDoesNotInventExternalFrame(t *testing.T) {
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
			device.Exit = false
			device.DeviceProfile.RGBCluster = test.cluster
			device.DeviceProfile.OpenRGBIntegration = test.openRGB
			if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{
				SelectedEffect: "static",
				Brightness:     100,
			}); err != nil {
				t.Fatal(err)
			}
			writes := 0
			device.lightingFrameWrite = func([]byte) { writes++ }

			device.SchedulerBrightness(0)
			device.SchedulerBrightness(1)
			if writes != 0 {
				t.Fatalf("%s scheduler transition without cache wrote %d frames", test.name, writes)
			}
		})
	}
}

func TestScimitarExternalFrameCacheDoesNotCrossOwnership(t *testing.T) {
	device, runtime := newScimitarCanonicalLightingTestDevice(t)
	prepareScimitarCanonicalMutationDevice(device)
	device.Exit = false
	device.DeviceProfile.OpenRGBIntegration = false
	device.DeviceProfile.RGBCluster = true
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{
		SelectedEffect: "static",
		Brightness:     100,
	}); err != nil {
		t.Fatal(err)
	}
	if !device.externalFrameCache.store(scimitarExternalFrameOpenRGB, []byte{
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
	}) {
		t.Fatal("OpenRGB frame was not cached")
	}
	writes := 0
	device.lightingFrameWrite = func([]byte) { writes++ }

	device.SchedulerBrightness(0)
	if writes != 0 {
		t.Fatalf("Cluster ownership replayed stale OpenRGB cache %d times", writes)
	}
	device.externalFrameCache.clear()
	if _, ok := device.externalFrameCache.load(scimitarExternalFrameOpenRGB); ok {
		t.Fatal("cleared external frame cache remained available")
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

func installScimitarProfilePersistenceTestRoot(t *testing.T) {
	t.Helper()
	previousPwd := pwd
	pwd = t.TempDir()
	if err := os.MkdirAll(filepath.Join(pwd, "database", "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pwd = previousPwd
	})
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
