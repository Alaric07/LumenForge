package m75W

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
)

func TestLightingSnapshotFailsClosedWithoutCanonicalSource(t *testing.T) {
	device := &Device{Serial: "m75w-lighting-test"}
	if snapshot, usable := device.LightingSnapshot(); usable || snapshot.TargetKind != "" {
		t.Fatalf("snapshot = %#v, usable=%t", snapshot, usable)
	}
}

func TestCanonicalLightingRestoresAndRefreshesOnlyRendererAdapter(t *testing.T) {
	device, runtime, _, _ := newM75CanonicalLightingTestDevice(t)
	custom := m75StaticSettings(200, 100, 50)
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 50}); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Effects.Set(device.Serial, "static", custom); err != nil {
		t.Fatal(err)
	}
	if !device.syncCanonicalLightingAdapter() {
		t.Fatal("canonical adapter was unavailable")
	}
	if device.DeviceProfile.RGBProfile != "static" || device.DeviceProfile.BrightnessSlider == nil || *device.DeviceProfile.BrightnessSlider != 50 {
		t.Fatalf("renderer adapter = %#v", device.DeviceProfile)
	}
	if profile := device.Rgb.Profiles["static"]; profile.StartColor != (rgb.Color{Red: 200, Green: 100, Blue: 50, Brightness: 1}) {
		t.Fatalf("renderer profile = %#v", profile)
	}
	// Stale legacy values are overwritten, never read as desired state.
	stale := uint8(1)
	device.DeviceProfile.RGBProfile, device.DeviceProfile.BrightnessSlider = "wave", &stale
	device.Rgb.Profiles["static"] = rgb.Profile{ProfileName: "static", StartColor: rgb.Color{Red: 1}}
	if !device.syncCanonicalLightingAdapter() || device.DeviceProfile.RGBProfile != "static" || *device.DeviceProfile.BrightnessSlider != 50 || device.Rgb.Profiles["static"].StartColor.Red != 200 {
		t.Fatalf("legacy state became authority: %#v %#v", device.DeviceProfile, device.Rgb.Profiles["static"])
	}
	snapshot, usable := device.LightingSnapshot()
	if !usable || snapshot.ConfiguredEffect != "static" || snapshot.Brightness != 50 || snapshot.SingleColorHex != "#c86432" || !snapshot.Customized {
		t.Fatalf("restored snapshot = %#v, usable=%t", snapshot, usable)
	}
}

func TestCanonicalLightingResetPersistsDefaultsAndPreservesState(t *testing.T) {
	device, runtime, statePath, effectPath := newM75CanonicalLightingTestDevice(t)
	state := lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 42}
	if err := runtime.State.Set(device.Serial, state); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Effects.Set(device.Serial, "static", m75StaticSettings(9, 8, 7)); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Effects.Set(device.Serial, "wave", m75WaveSettings()); err != nil {
		t.Fatal(err)
	}
	restarts := 0
	device.lightingRestart = func() { restarts++ }
	if err := device.ResetLightingEffectSettings("static"); err != nil {
		t.Fatal(err)
	}
	if restarts != 1 {
		t.Fatalf("restarts = %d, want 1", restarts)
	}
	gotState, found, err := runtime.State.Resolve(device.Serial)
	if err != nil || !found || gotState != state {
		t.Fatalf("state after reset = %#v, found=%t, err=%v", gotState, found, err)
	}
	if _, found, err = runtime.Effects.Get(device.Serial, "static"); err != nil || found {
		t.Fatalf("static customization after reset: found=%t err=%v", found, err)
	}
	if wave, found, err := runtime.Effects.Get(device.Serial, "wave"); err != nil || !found || wave.EffectID != "wave" {
		t.Fatalf("unrelated wave customization = %#v, found=%t, err=%v", wave, found, err)
	}
	resolution, err := runtime.Resolver.Resolve(lightingsettings.IndependentDevice(device.Serial), "static")
	if err != nil || resolution.Customized || resolution.Settings.EffectID != "static" {
		t.Fatalf("reset resolution = %#v, err=%v", resolution, err)
	}
	persistedState, err := lightingsettings.LoadIndependentDeviceStateStore(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if got, found, err := persistedState.Resolve(device.Serial); err != nil || !found || got != state {
		t.Fatalf("persisted state = %#v, found=%t, err=%v", got, found, err)
	}
	persistedEffects, err := lightingsettings.LoadDeviceStore(effectPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := persistedEffects.Get(device.Serial, "static"); err != nil || found {
		t.Fatalf("persisted reset customization: found=%t err=%v", found, err)
	}
}

func TestUpdateRgbProfileDisconnectedCanonicalNoop(t *testing.T) {
	device, runtime, _, _ := newM75CanonicalLightingTestDevice(t)
	before := lightingsettings.IndependentDeviceLightingState{SelectedEffect: "wave", Brightness: 42}
	if err := runtime.State.Set(device.Serial, before); err != nil {
		t.Fatal(err)
	}
	restarts := 0
	device.lightingRestart = func() { restarts++ }

	device.Connected = false
	if got := device.UpdateRgbProfile(0, "static"); got != 0 {
		t.Fatalf("disconnected UpdateRgbProfile = %d, want 0", got)
	}
	if got, found, err := runtime.State.Resolve(device.Serial); err != nil || !found || got != before {
		t.Fatalf("disconnected canonical state = %#v, found=%t, err=%v", got, found, err)
	}
	if restarts != 0 {
		t.Fatalf("disconnected canonical restarts = %d, want 0", restarts)
	}

	device.Connected = true
	if got := device.UpdateRgbProfile(0, "static"); got != 1 {
		t.Fatalf("connected UpdateRgbProfile = %d, want 1", got)
	}
	if got, found, err := runtime.State.Resolve(device.Serial); err != nil || !found || got.SelectedEffect != "static" || got.Brightness != before.Brightness {
		t.Fatalf("connected canonical state = %#v, found=%t, err=%v", got, found, err)
	}
	if restarts != 1 {
		t.Fatalf("connected canonical restarts = %d, want 1", restarts)
	}
}

func TestCanonicalOffEffectiveBrightnessAndUnsupportedCapabilities(t *testing.T) {
	device, runtime, _, _ := newM75CanonicalLightingTestDevice(t)
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{SelectedEffect: "off", Brightness: 70}); err != nil {
		t.Fatal(err)
	}
	override := uint8(25)
	device.schedulerBrightnessOverride.set(&override)
	if !device.syncCanonicalLightingAdapter() {
		t.Fatal("canonical adapter was unavailable")
	}
	if device.DeviceProfile.RGBProfile != "off" || *device.DeviceProfile.BrightnessSlider != 25 || device.Rgb.Profiles["off"].ProfileName != "off" {
		t.Fatalf("off renderer adapter = %#v %#v", device.DeviceProfile, device.Rgb.Profiles["off"])
	}
	// This is the unchanged DPI scaling primitive used by setDeviceColor(dpi).
	dpi := rgb.Color{Red: 200, Green: 100, Blue: 40, Brightness: rgb.GetBrightnessValueFloat(*device.DeviceProfile.BrightnessSlider)}
	if got := rgb.ModifyBrightness(dpi); got.Red != 50 || got.Green != 25 || got.Blue != 10 {
		t.Fatalf("DPI effective brightness = %#v", got)
	}
	if !device.SupportsLightingEffect("mouse") || hasM75UnsupportedOwnership(device) {
		t.Fatal("M75 authored mode or ownership capability mismatch")
	}
	device.lightingRestart = func() {}
	if err := device.SetLightingEffect("mouse"); err != nil {
		t.Fatalf("authored mouse effect = %v", err)
	}
}

func TestCanonicalMouseSnapshotAndZoneMutationPreserveM75Mapping(t *testing.T) {
	device, runtime, _, _ := newM75CanonicalLightingTestDevice(t)
	root := t.TempDir()
	previousPwd := pwd
	pwd = root
	t.Cleanup(func() { pwd = previousPwd })
	logger.Init()
	if err := os.MkdirAll(filepath.Join(root, "database", "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	device.Connected, device.Exit, device.LEDChannels = true, true, 2
	device.DeviceProfile.Profiles = map[int]DPIProfile{}
	device.DeviceProfile.ZoneColors = map[int]ZoneColors{
		0: {Name: "Bottom", Color: &rgb.Color{Red: 1, Green: 2, Blue: 3}, ColorIndex: []int{1, 3, 5}},
		1: {Name: "Logo", Color: &rgb.Color{Red: 4, Green: 5, Blue: 6}, ColorIndex: []int{0, 2, 4}},
	}
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{SelectedEffect: "mouse", Brightness: 70}); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := device.LightingSnapshot()
	if !ok || len(snapshot.SupportedEffects) != 20 || snapshot.AuthoredZoneEditor == nil || len(snapshot.AuthoredZoneEditor.Zones) != 2 {
		t.Fatalf("mouse snapshot = %#v, usable=%t", snapshot, ok)
	}
	for index, effect := range []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"} {
		if snapshot.SupportedEffects[index].ID != effect {
			t.Fatalf("effect %d = %q, want %q", index, snapshot.SupportedEffects[index].ID, effect)
		}
	}
	for index, want := range []struct{ id, label string }{{"0", "Bottom"}, {"1", "Logo"}} {
		got := snapshot.AuthoredZoneEditor.Zones[index]
		if got.ID != want.id || got.Label != want.label {
			t.Fatalf("zone %d = %#v", index, got)
		}
	}
	if err := device.SetLightingZoneColor("mouse", "zone", "0", "", rgb.Color{Red: 9, Green: 8, Blue: 7}); err != nil {
		t.Fatal(err)
	}
	bottom, logo := device.DeviceProfile.ZoneColors[0], device.DeviceProfile.ZoneColors[1]
	if bottom.Color.Red != 9 || bottom.Color.Green != 8 || bottom.Color.Blue != 7 || bottom.ColorIndex[0] != 1 || bottom.ColorIndex[1] != 3 || bottom.ColorIndex[2] != 5 || logo.ColorIndex[0] != 0 || logo.ColorIndex[1] != 2 || logo.ColorIndex[2] != 4 {
		t.Fatalf("zone persistence or mapping changed: bottom=%#v logo=%#v", bottom, logo)
	}
	if _, found, _ := runtime.Effects.Get(device.Serial, "mouse"); found {
		t.Fatal("mouse generic settings were fabricated")
	}
}

func TestSchedulerAndUserRgbOffRemainIndependent(t *testing.T) {
	device, runtime, _, _ := newM75CanonicalLightingTestDevice(t)
	if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 70}); err != nil {
		t.Fatal(err)
	}
	device.lightingRestart = func() {}
	// Scheduler off remains effective after the user explicitly turns RGB on.
	device.SchedulerBrightness(0)
	device.ControlDeviceRgb(false)
	if got, _ := device.resolveEffectiveCanonicalLighting(); got.brightness != 0 {
		t.Fatalf("scheduler off brightness = %d, want 0", got.brightness)
	}
	// User off remains effective after scheduler wake-up.
	device.ControlDeviceRgb(true)
	device.SchedulerBrightness(100)
	if got, _ := device.resolveEffectiveCanonicalLighting(); got.brightness != 0 {
		t.Fatalf("user off brightness after wake = %d, want 0", got.brightness)
	}
	// Clearing either intent must not clear the other.
	device.SchedulerBrightness(0)
	device.ControlDeviceRgb(false)
	if got, _ := device.resolveEffectiveCanonicalLighting(); got.brightness != 0 {
		t.Fatalf("clearing user off cleared scheduler off: %d", got.brightness)
	}
	device.ControlDeviceRgb(true)
	device.SchedulerBrightness(100)
	if got, _ := device.resolveEffectiveCanonicalLighting(); got.brightness != 0 {
		t.Fatalf("clearing scheduler off cleared user off: %d", got.brightness)
	}
}

func TestCanonicalRgbOffCleanupAndRuntimeFailureFallback(t *testing.T) {
	device, _, _, _ := newM75CanonicalLightingTestDevice(t)
	root := t.TempDir()
	previousPwd := pwd
	pwd = root
	t.Cleanup(func() { pwd = previousPwd })
	logger.Init()
	if err := os.MkdirAll(filepath.Join(root, "database", "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	device.Serial = "m75wtest"
	device.DeviceProfile.Path = filepath.Join(root, "database", "profiles", device.Serial+".json")
	device.DeviceProfile.Serial = device.Serial
	device.DeviceProfile.Profiles = map[int]DPIProfile{}
	device.DeviceProfile.Active = true
	device.DeviceProfile.RgbOff = true
	if !device.clearLegacyRgbOff() || device.DeviceProfile.RgbOff {
		t.Fatalf("stale RgbOff was not cleared: %#v", device.DeviceProfile)
	}
	persisted := &DeviceProfile{}
	file, err := os.Open(device.DeviceProfile.Path)
	if err != nil {
		t.Fatal(err)
	}
	err = json.NewDecoder(file).Decode(persisted)
	_ = file.Close()
	if err != nil || persisted.RgbOff {
		t.Fatalf("persisted RgbOff = %#v, err=%v", persisted, err)
	}
	fallback := &Device{Serial: "runtime-failure", BatteryLevel: 73}
	err = fallback.attachIndependentDeviceLightingRuntime(config.Paths{OpenRGBDeviceLightingFile: filepath.Join(root, "state.json"), DeviceEffectSettingsFile: filepath.Join(root, "effects.json"), ShippedDatabaseRoot: filepath.Join(root, "missing-defaults")})
	if err == nil || fallback.lightingSource != nil || fallback.BatteryLevel != 73 {
		t.Fatalf("runtime failure fallback = source=%#v battery=%d err=%v", fallback.lightingSource, fallback.BatteryLevel, err)
	}
}

func hasM75UnsupportedOwnership(device *Device) bool {
	_, cluster := reflect.TypeOf(device).MethodByName("ProcessSetRgbCluster")
	_, openRGB := reflect.TypeOf(device).MethodByName("ProcessSetOpenRgbIntegration")
	return cluster || openRGB
}

func newM75CanonicalLightingTestDevice(t *testing.T) (*Device, *lightingsettings.IndependentDeviceRuntime, string, string) {
	t.Helper()
	root := t.TempDir()
	statePath, effectPath := filepath.Join(root, "state.json"), filepath.Join(root, "effects.json")
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(statePath, effectPath, filepath.Join("..", "..", "..", "database", "rgb.json"))
	if err != nil {
		t.Fatal(err)
	}
	legacy := uint8(1)
	device := &Device{Serial: "m75w-canonical-test", DeviceProfile: &DeviceProfile{RGBProfile: "mouse", BrightnessSlider: &legacy}, Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{}}}
	device.lightingSource = m75IndependentSource{id: device.Serial, state: runtime.State, effects: runtime.Effects, resolver: runtime.Resolver}
	return device, runtime, statePath, effectPath
}

func m75StaticSettings(red, green, blue int) lightingsettings.EffectSettings {
	return lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: float64(red), Green: float64(green), Blue: float64(blue)}}}
}
func m75WaveSettings() lightingsettings.EffectSettings {
	speed := 4.0
	return lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "wave", Speed: &speed, TwoColor: &lightingsettings.TwoColorSettings{Start: lightingsettings.Color{Red: 1}, End: lightingsettings.Color{Blue: 2}}}
}
