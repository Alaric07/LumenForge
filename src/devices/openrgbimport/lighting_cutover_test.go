package openrgbimport

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/openrgb"
)

type failingLightingStateAccess struct {
	deviceLightingStateAccess
	err error
}

func TestOpenRGBLegacyRGBCompatibilitySurfaceRemoved(t *testing.T) {
	deviceType := reflect.TypeOf(&Device{})
	for _, method := range []string{
		"GetRgbProfiles",
		"GetRgbProfile",
		"UpdateRgbProfileData",
		"UpdateRgbProfile",
		"ProcessGetRgbOverride",
		"SetRGBOverride",
		"ProcessSetRgbOverride",
	} {
		if _, found := deviceType.MethodByName(method); found {
			t.Errorf("OpenRGB Device still exposes legacy method %s", method)
		}
	}
	for _, target := range []struct {
		name   string
		typeOf reflect.Type
		field  string
	}{
		{name: "Device", typeOf: reflect.TypeOf(Device{}), field: "Rgb"},
		{name: "DeviceSnapshot", typeOf: reflect.TypeOf(DeviceSnapshot{}), field: "Rgb"},
		{name: "DeviceProfile", typeOf: reflect.TypeOf(DeviceProfile{}), field: "RGBOverride"},
	} {
		if _, found := target.typeOf.FieldByName(target.field); found {
			t.Errorf("%s still exposes legacy field %s", target.name, target.field)
		}
	}
}

func (access failingLightingStateAccess) Set(string, DeviceLightingState) error { return access.err }

type failOnLightingStateRestore struct {
	deviceLightingStateAccess
	setCalls     int
	restoreError error
}

func (access *failOnLightingStateRestore) Set(deviceID string, state DeviceLightingState) error {
	access.setCalls++
	if access.setCalls == 2 {
		return access.restoreError
	}
	return access.deviceLightingStateAccess.Set(deviceID, state)
}

type failingLightingEffectAccess struct {
	deviceLightingEffectAccess
	err error
}

func (access failingLightingEffectAccess) Set(string, string, lightingsettings.EffectSettings) error {
	return access.err
}

type selectiveLightingStateResolutionFailure struct {
	deviceLightingStateAccess
	failures map[string]error
}

func (access selectiveLightingStateResolutionFailure) Resolve(deviceID string) (DeviceLightingState, bool, error) {
	if err := access.failures[deviceID]; err != nil {
		return DeviceLightingState{}, false, err
	}
	return access.deviceLightingStateAccess.Resolve(deviceID)
}

type optionalFileContents struct {
	data   []byte
	exists bool
}

func readOptionalFile(t *testing.T, path string) optionalFileContents {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return optionalFileContents{}
	}
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return optionalFileContents{data: data, exists: true}
}

func assertOptionalFileUnchanged(t *testing.T, path string, before optionalFileContents) {
	t.Helper()
	after := readOptionalFile(t, path)
	if after.exists != before.exists || !reflect.DeepEqual(after.data, before.data) {
		t.Fatalf("file %q changed: before=%#v after=%#v", path, before, after)
	}
}

func testOfflineReconstruction(
	t *testing.T,
	serials []string,
	states map[string]DeviceLightingState,
	failures map[string]error,
	wantSerials []string,
) {
	t.Helper()
	preserveOpenRGBStatus(t)
	StopManager()
	setConfiguredDevices(nil)
	t.Cleanup(func() { setConfiguredDevices(nil) })

	root := t.TempDir()
	paths := deviceLightingPathsForMutableRoot(root)
	paths.ShippedDatabaseRoot = testShippedDatabaseRoot
	restorePaths := config.UsePathsForTest(paths)
	t.Cleanup(restorePaths)
	configPath := useStorePath(t)

	configs := make(map[string]DeviceConfig, len(serials))
	for _, serial := range serials {
		configs[serial] = testConfig(serial, "Offline "+serial)
	}
	if err := saveConfigStore(&ConfigStore{Devices: configs}); err != nil {
		t.Fatal(err)
	}

	runtime, err := loadDeviceLightingRuntime(paths)
	if err != nil {
		t.Fatal(err)
	}
	stateStore, ok := runtime.state.(*DeviceLightingStateStore)
	if !ok {
		t.Fatalf("device lighting state store type = %T", runtime.state)
	}
	for serial, state := range states {
		if err = stateStore.Set(serial, state); err != nil {
			t.Fatalf("persist state for %q: %v", serial, err)
		}
	}
	runtime.state = selectiveLightingStateResolutionFailure{
		deviceLightingStateAccess: stateStore,
		failures:                  failures,
	}

	legacyFiles := make(map[string]optionalFileContents, len(failures)*2)
	for serial := range failures {
		legacyRGBPath := filepath.Join(root, "database", "rgb", serial+".json")
		legacyProfilePath := filepath.Join(root, "database", "profiles", serial+".json")
		if err = os.MkdirAll(filepath.Dir(legacyRGBPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err = os.MkdirAll(filepath.Dir(legacyProfilePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(legacyRGBPath, []byte(`{"legacy":"rgb-fallback-must-not-run"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(legacyProfilePath, []byte(`{"legacy":"profile-fallback-must-not-run"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		legacyFiles[legacyRGBPath] = readOptionalFile(t, legacyRGBPath)
		legacyFiles[legacyProfilePath] = readOptionalFile(t, legacyProfilePath)
	}

	unchangedFiles := map[string]optionalFileContents{
		configPath:                      readOptionalFile(t, configPath),
		paths.OpenRGBDeviceLightingFile: readOptionalFile(t, paths.OpenRGBDeviceLightingFile),
		paths.DeviceEffectSettingsFile:  readOptionalFile(t, paths.DeviceEffectSettingsFile),
		paths.ClusterEffectSettingsFile: readOptionalFile(t, paths.ClusterEffectSettingsFile),
	}
	for path, contents := range legacyFiles {
		unchangedFiles[path] = contents
	}

	wrappers := InitAll()
	if len(wrappers) != len(wantSerials) {
		t.Fatalf("InitAll returned %d devices, want %d", len(wrappers), len(wantSerials))
	}
	for index, serial := range wantSerials {
		if wrappers[index] == nil || wrappers[index].Serial != serial || !wrappers[index].Unavailable {
			t.Fatalf("wrapper %d = %#v, want unavailable %q", index, wrappers[index], serial)
		}
	}

	configured := configuredDevicesSnapshot()
	if len(configured) != len(wantSerials) {
		t.Fatalf("configured devices = %d, want %d", len(configured), len(wantSerials))
	}
	for _, serial := range wantSerials {
		device := configured[serial]
		if device == nil {
			t.Fatalf("configured device %q is missing", serial)
		}
		want := states[serial]
		if device.effect != want.SelectedEffect || device.brightness != want.Brightness {
			t.Fatalf("configured device %q state = effect %q brightness %d, want %#v", serial, device.effect, device.brightness, want)
		}
		if device.DeviceProfile == nil || device.DeviceProfile.RGBProfile != want.SelectedEffect ||
			device.DeviceProfile.BrightnessSlider == nil || *device.DeviceProfile.BrightnessSlider != want.Brightness {
			t.Fatalf("configured device %q profile = %#v, want effect %q brightness %d", serial, device.DeviceProfile, want.SelectedEffect, want.Brightness)
		}
	}
	for serial := range failures {
		if configured[serial] != nil {
			t.Fatalf("failed device %q remained configured", serial)
		}
	}
	state, statusErr := openrgb.GetStatus()
	if state != openrgb.StateOffline || statusErr != nil {
		t.Fatalf("OpenRGB status after per-device failures = %q, %v; want Offline without a global error", state, statusErr)
	}
	for path, before := range unchangedFiles {
		assertOptionalFileUnchanged(t, path, before)
	}
}

func newCutoverTestDevice(t *testing.T, serial string) (*Device, config.Paths) {
	t.Helper()
	paths := deviceLightingPathsForMutableRoot(t.TempDir())
	paths.ShippedDatabaseRoot = testShippedDatabaseRoot
	runtime, err := loadDeviceLightingRuntime(paths)
	if err != nil {
		t.Fatal(err)
	}
	brightness := uint8(1)
	device := &Device{
		Product:      "Cutover Test Controller",
		Serial:       serial,
		IsOpenRGB:    true,
		controllerId: 7,
		colorCount:   1,
		lastColor:    []byte{100, 150, 200},
		RGBModes:     importerSoftwareEffectCatalogue(),
		DeviceProfile: &DeviceProfile{
			Active:           true,
			RGBProfile:       "obsolete-legacy-effect",
			BrightnessSlider: &brightness,
			ZoneColors:       map[int]ZoneColors{},
		},
	}
	if err = device.attachLightingRuntime(runtime); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(device.Stop)
	return device, paths
}

func reconstructedDeviceLightingState(
	t *testing.T,
	device *Device,
	store *DeviceLightingStateStore,
) (*Device, DeviceLightingState, bool) {
	t.Helper()

	reloadedStore, err := LoadDeviceLightingStateStore(store.path)
	if err != nil {
		t.Fatalf("reload device lighting state: %v", err)
	}
	state, found, err := reloadedStore.Resolve(device.Serial)
	if err != nil {
		t.Fatalf("resolve reloaded device lighting state: %v", err)
	}
	effects, ok := device.lightingEffects.(*lightingsettings.DeviceStore)
	if !ok {
		t.Fatalf("device lighting effect store type = %T", device.lightingEffects)
	}
	resolver, ok := device.lightingResolver.(*lightingsettings.Resolver)
	if !ok {
		t.Fatalf("device lighting resolver type = %T", device.lightingResolver)
	}
	reconstructed := &Device{Serial: device.Serial}
	if err = reconstructed.attachLightingRuntime(&deviceLightingRuntime{
		state:    reloadedStore,
		effects:  effects,
		resolver: resolver,
	}); err != nil {
		t.Fatalf("attach reloaded device lighting runtime: %v", err)
	}
	return reconstructed, state, found
}

func TestSaveDeviceConfigRestoresPersistedLightingStateOnFailure(t *testing.T) {
	useStorePath(t)
	serial := "openrgb-config-lighting-rollback"
	saved := testConfig(serial, "Configuration Rollback Controller")
	if err := saveConfigStore(&ConfigStore{Devices: map[string]DeviceConfig{serial: saved}}); err != nil {
		t.Fatal(err)
	}
	device := testDevice(saved)
	device.controllerId = 7
	prior := DeviceLightingState{SelectedEffect: "rainbow", Brightness: 37}
	store, ok := device.lightingState.(*DeviceLightingStateStore)
	if !ok {
		t.Fatalf("device lighting state store type = %T", device.lightingState)
	}
	if err := store.Set(serial, prior); err != nil {
		t.Fatal(err)
	}
	device.effect = prior.SelectedEffect
	device.brightness = prior.Brightness
	device.DeviceProfile = &DeviceProfile{
		Active:           true,
		RGBProfile:       prior.SelectedEffect,
		BrightnessSlider: func() *uint8 { value := prior.Brightness; return &value }(),
		ZoneColors:       buildZoneColorsFromConfig(&saved, device.lastColor),
	}

	configurationSaveError := errors.New("injected configuration save failure")
	previousSendFrame := sendConfigFrame
	previousRename := renameConfigStore
	sendConfigFrame = func(uint32, []byte) error { return nil }
	renameConfigStore = func(string, string) error { return configurationSaveError }
	t.Cleanup(func() {
		sendConfigFrame = previousSendFrame
		renameConfigStore = previousRename
	})

	updated := saved
	updated.Zones = []ZoneConfig{{Name: "Updated Zone", LedCount: 1}}
	if err := device.SaveDeviceConfig(&updated); !errors.Is(err, configurationSaveError) {
		t.Fatalf("SaveDeviceConfig error = %v, want configuration save failure", err)
	}
	if !reflect.DeepEqual(device.Config, &saved) || device.effect != prior.SelectedEffect || device.brightness != prior.Brightness {
		t.Fatalf("in-memory rollback = config %#v, effect %q, brightness %d", device.Config, device.effect, device.brightness)
	}
	if device.DeviceProfile == nil || device.DeviceProfile.RGBProfile != prior.SelectedEffect ||
		device.DeviceProfile.BrightnessSlider == nil || *device.DeviceProfile.BrightnessSlider != prior.Brightness {
		t.Fatalf("rolled-back profile presentation = %#v, want effect %q brightness %d", device.DeviceProfile, prior.SelectedEffect, prior.Brightness)
	}
	persisted, found, err := store.Resolve(serial)
	if err != nil || !found || persisted != prior {
		t.Fatalf("restored persisted state = %#v, %t, %v; want %#v", persisted, found, err, prior)
	}
	reconstructed, reloaded, found := reconstructedDeviceLightingState(t, device, store)
	if !found || reloaded != prior || reconstructed.effect != prior.SelectedEffect || reconstructed.brightness != prior.Brightness {
		t.Fatalf("reconstructed state = %#v, %t; device effect %q brightness %d", reloaded, found, reconstructed.effect, reconstructed.brightness)
	}
}

func TestSaveDeviceConfigRemovesTemporaryLightingStateOnFailure(t *testing.T) {
	useStorePath(t)
	serial := "openrgb-config-lighting-default-rollback"
	saved := testConfig(serial, "Default Rollback Controller")
	if err := saveConfigStore(&ConfigStore{Devices: map[string]DeviceConfig{serial: saved}}); err != nil {
		t.Fatal(err)
	}
	device := testDevice(saved)
	device.controllerId = 7
	store, ok := device.lightingState.(*DeviceLightingStateStore)
	if !ok {
		t.Fatalf("device lighting state store type = %T", device.lightingState)
	}
	if state, found, err := store.Resolve(serial); err != nil || found || state != DefaultDeviceLightingState() {
		t.Fatalf("initial default state = %#v, %t, %v", state, found, err)
	}

	outputError := errors.New("injected configuration output failure")
	previousSendFrame := sendConfigFrame
	sendConfigFrame = func(uint32, []byte) error { return outputError }
	t.Cleanup(func() { sendConfigFrame = previousSendFrame })

	updated := saved
	updated.Zones = []ZoneConfig{{Name: "Updated Zone", LedCount: 2}}
	if err := device.SaveDeviceConfig(&updated); !errors.Is(err, outputError) {
		t.Fatalf("SaveDeviceConfig error = %v, want output failure", err)
	}
	state, found, err := store.Resolve(serial)
	wantDefault := DefaultDeviceLightingState()
	if err != nil || found || state != wantDefault {
		t.Fatalf("rolled-back default state = %#v, %t, %v; want absent record resolving to %#v", state, found, err, wantDefault)
	}
	if device.effect != wantDefault.SelectedEffect || device.brightness != wantDefault.Brightness || device.brightness == 0 {
		t.Fatalf("rolled-back in-memory state = effect %q brightness %d, want %#v", device.effect, device.brightness, wantDefault)
	}
	if device.DeviceProfile == nil || device.DeviceProfile.RGBProfile != wantDefault.SelectedEffect ||
		device.DeviceProfile.BrightnessSlider == nil || *device.DeviceProfile.BrightnessSlider != wantDefault.Brightness {
		t.Fatalf("rolled-back profile presentation = %#v, want effect %q brightness %d", device.DeviceProfile, wantDefault.SelectedEffect, wantDefault.Brightness)
	}
	if err = validateDeviceLightingState(DeviceLightingState{
		SelectedEffect: device.effect,
		Brightness:     device.brightness,
	}); err != nil {
		t.Fatalf("rolled-back state does not pass effect validation: %v", err)
	}
	reconstructed, reloaded, found := reconstructedDeviceLightingState(t, device, store)
	if found || reloaded != wantDefault || reconstructed.effect != wantDefault.SelectedEffect || reconstructed.brightness != wantDefault.Brightness {
		t.Fatalf("reconstructed default state = %#v, %t; device effect %q brightness %d", reloaded, found, reconstructed.effect, reconstructed.brightness)
	}
}

func TestSaveDeviceConfigReportsLightingStateRollbackFailure(t *testing.T) {
	useStorePath(t)
	serial := "openrgb-config-lighting-rollback-failure"
	saved := testConfig(serial, "Rollback Failure Controller")
	if err := saveConfigStore(&ConfigStore{Devices: map[string]DeviceConfig{serial: saved}}); err != nil {
		t.Fatal(err)
	}
	device := testDevice(saved)
	device.controllerId = 7
	prior := DeviceLightingState{SelectedEffect: "rainbow", Brightness: 41}
	store, ok := device.lightingState.(*DeviceLightingStateStore)
	if !ok {
		t.Fatalf("device lighting state store type = %T", device.lightingState)
	}
	if err := store.Set(serial, prior); err != nil {
		t.Fatal(err)
	}
	device.effect = prior.SelectedEffect
	device.brightness = prior.Brightness
	restoreError := errors.New("injected lighting state restoration failure")
	access := &failOnLightingStateRestore{
		deviceLightingStateAccess: store,
		restoreError:              restoreError,
	}
	device.lightingState = access

	outputError := errors.New("injected configuration output failure")
	previousSendFrame := sendConfigFrame
	sendConfigFrame = func(uint32, []byte) error { return outputError }
	t.Cleanup(func() { sendConfigFrame = previousSendFrame })

	updated := saved
	updated.Zones = []ZoneConfig{{Name: "Updated Zone", LedCount: 2}}
	err := device.SaveDeviceConfig(&updated)
	if !errors.Is(err, outputError) || !errors.Is(err, restoreError) ||
		!strings.Contains(err.Error(), "restore device lighting state after configuration failure") {
		t.Fatalf("SaveDeviceConfig rollback error = %v", err)
	}
	if access.setCalls != 2 {
		t.Fatalf("lighting state Set calls = %d, want temporary write and restoration attempt", access.setCalls)
	}
	temporary := DeviceLightingState{SelectedEffect: "static", Brightness: prior.Brightness}
	persisted, found, resolveErr := store.Resolve(serial)
	if resolveErr != nil || !found || persisted != temporary {
		t.Fatalf("state after failed rollback = %#v, %t, %v; want %#v", persisted, found, resolveErr, temporary)
	}
	if device.effect != temporary.SelectedEffect || device.brightness != temporary.Brightness {
		t.Fatalf("in-memory state after failed rollback = effect %q brightness %d", device.effect, device.brightness)
	}
	if device.DeviceProfile == nil || device.DeviceProfile.RGBProfile != temporary.SelectedEffect ||
		device.DeviceProfile.BrightnessSlider == nil || *device.DeviceProfile.BrightnessSlider != temporary.Brightness {
		t.Fatalf("profile after failed rollback = %#v, want effect %q brightness %d", device.DeviceProfile, temporary.SelectedEffect, temporary.Brightness)
	}
}

func TestOpenRGBLightingCutoverSelectionBrightnessAndSpeed(t *testing.T) {
	_, calls := installLightingDeviceTestSeams(t)
	device, paths := newCutoverTestDevice(t, "openrgb-cutover-device")
	if device.effect != "static" || device.brightness != 100 {
		t.Fatalf("clean-install state = effect %q, brightness %d", device.effect, device.brightness)
	}
	if _, err := os.Stat(paths.OpenRGBDeviceLightingFile); !os.IsNotExist(err) {
		t.Fatalf("construction wrote target state: %v", err)
	}
	if _, err := os.Stat(paths.DeviceEffectSettingsFile); !os.IsNotExist(err) {
		t.Fatalf("construction materialized customization: %v", err)
	}

	if err := device.SetBrightness(40); err != nil {
		t.Fatal(err)
	}
	if err := device.SetEffect("rainbow"); err != nil {
		t.Fatal(err)
	}
	device.Stop()
	if err := device.SetEffectSpeed(device.Serial, "rainbow", 4.5); err != nil {
		t.Fatal(err)
	}
	device.Stop()
	stateStore, err := LoadDeviceLightingStateStore(paths.OpenRGBDeviceLightingFile)
	if err != nil {
		t.Fatal(err)
	}
	state, found, err := stateStore.Resolve(device.Serial)
	if err != nil || !found || state != (DeviceLightingState{SelectedEffect: "rainbow", Brightness: 40}) {
		t.Fatalf("persisted target state = %#v, %t, %v", state, found, err)
	}
	effectStore, err := lightingsettings.LoadDeviceStore(paths.DeviceEffectSettingsFile)
	if err != nil {
		t.Fatal(err)
	}
	custom, found, err := effectStore.Get(device.Serial, "rainbow")
	if err != nil || !found || custom.Speed == nil || *custom.Speed != 4.5 {
		t.Fatalf("persisted complete customization = %#v, %t, %v", custom, found, err)
	}
	if err = lightingsettings.Validate(custom); err != nil {
		t.Fatalf("persisted customization is incomplete: %v", err)
	}
	otherEffect, err := device.resolveLightingSettings("wave")
	if err != nil || otherEffect.Customized {
		t.Fatalf("Rainbow edit affected Wave = %#v, %v", otherEffect, err)
	}
	otherDevice := &Device{Serial: "openrgb-cutover-other-device", IsOpenRGB: true}
	runtime, err := loadDeviceLightingRuntime(paths)
	if err != nil {
		t.Fatal(err)
	}
	if err = otherDevice.attachLightingRuntime(runtime); err != nil {
		t.Fatal(err)
	}
	otherDeviceEffect, err := otherDevice.resolveLightingSettings("rainbow")
	if err != nil || otherDeviceEffect.Customized {
		t.Fatalf("first device edit affected second device = %#v, %v", otherDeviceEffect, err)
	}
	if _, found, err = effectStore.Get(device.Serial, "static"); err != nil || found {
		t.Fatalf("effect selection materialized Static customization: %t, %v", found, err)
	}
	if calls.persistentFrames == 0 {
		t.Fatal("successful animated mutations produced no renderer output")
	}

	if err = device.SetEffect("off"); err != nil {
		t.Fatal(err)
	}
	if err = device.SetEffect("rainbow"); err != nil {
		t.Fatal(err)
	}
	device.Stop()
	if restoredSpeed := device.resolvedRendererProfile("rainbow"); restoredSpeed == nil || restoredSpeed.Speed != 4.5 || device.brightness != 40 {
		t.Fatalf("switching back restored profile %#v, brightness %d", restoredSpeed, device.brightness)
	}

	restarted := &Device{Serial: device.Serial, IsOpenRGB: true}
	if err = restarted.attachLightingRuntime(runtime); err != nil {
		t.Fatal(err)
	}
	if restarted.effect != "rainbow" || restarted.brightness != 40 {
		t.Fatalf("restart state = effect %q, brightness %d", restarted.effect, restarted.brightness)
	}
	resolution, err := restarted.resolveLightingSettings("rainbow")
	if err != nil || !resolution.Customized || resolution.Settings.Speed == nil || *resolution.Settings.Speed != 4.5 {
		t.Fatalf("restart resolution = %#v, %v", resolution, err)
	}
}

func TestOpenRGBLightingCutoverPersistenceFailuresChangeNothing(t *testing.T) {
	device, _ := newCutoverTestDevice(t, "openrgb-cutover-failure")
	beforeState := DeviceLightingState{SelectedEffect: device.effect, Brightness: device.brightness}
	confirmedState := device.lightingState
	device.lightingState = failingLightingStateAccess{deviceLightingStateAccess: confirmedState, err: errors.New("injected state failure")}
	if err := device.SetBrightness(20); err == nil {
		t.Fatal("Brightness succeeded despite persistence failure")
	}
	if err := device.SetEffect("rainbow"); err == nil {
		t.Fatal("effect selection succeeded despite persistence failure")
	}
	if got := (DeviceLightingState{SelectedEffect: device.effect, Brightness: device.brightness}); got != beforeState {
		t.Fatalf("failed target mutations changed active state: %#v", got)
	}
	persisted, found, err := confirmedState.Resolve(device.Serial)
	if err != nil || found || persisted != DefaultDeviceLightingState() {
		t.Fatalf("failed target mutations changed persisted state: %#v, %t, %v", persisted, found, err)
	}

	device.lightingState = confirmedState
	if err = device.lightingState.Set(device.Serial, DeviceLightingState{SelectedEffect: "rainbow", Brightness: 100}); err != nil {
		t.Fatal(err)
	}
	device.effect = "rainbow"
	prior, err := device.resolveLightingSettings("rainbow")
	if err != nil {
		t.Fatal(err)
	}
	confirmedEffects := device.lightingEffects
	device.lightingEffects = failingLightingEffectAccess{deviceLightingEffectAccess: confirmedEffects, err: errors.New("injected customization failure")}
	if err = device.SetEffectSpeed(device.Serial, "rainbow", 5); err == nil {
		t.Fatal("Speed succeeded despite customization persistence failure")
	}
	after, err := device.resolveLightingSettings("rainbow")
	if err != nil || !reflect.DeepEqual(after, prior) {
		t.Fatalf("failed Speed mutation changed resolution: before=%#v after=%#v err=%v", prior, after, err)
	}
}

func TestOpenRGBLightingReconnectReadsWithoutWriting(t *testing.T) {
	_, _ = installLightingDeviceTestSeams(t)
	device, paths := newCutoverTestDevice(t, "openrgb-cutover-reconnect")
	if err := device.lightingState.Set(device.Serial, DeviceLightingState{SelectedEffect: "off", Brightness: 25}); err != nil {
		t.Fatal(err)
	}
	device.effect = "off"
	device.brightness = 25
	before, err := os.ReadFile(paths.OpenRGBDeviceLightingFile)
	if err != nil {
		t.Fatal(err)
	}
	if err = device.resumeDesiredState(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(paths.OpenRGBDeviceLightingFile)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("reconnect rewrote authoritative target state")
	}
	for _, legacy := range []string{
		filepath.Join(paths.MutableDataRoot, "database", "rgb", device.Serial+".json"),
		filepath.Join(paths.MutableDataRoot, "database", "rgb", "cluster.json"),
	} {
		if _, err = os.Stat(legacy); !os.IsNotExist(err) {
			t.Fatalf("cut-over path touched legacy file %q: %v", legacy, err)
		}
	}
}

func TestOpenRGBLightingReconnectRestoresCustomizedSpeedWithoutDuplicateWorker(t *testing.T) {
	_, _ = installLightingDeviceTestSeams(t)
	device, paths := newCutoverTestDevice(t, "openrgb-cutover-reconnect-speed")
	if err := device.lightingState.Set(device.Serial, DeviceLightingState{SelectedEffect: "rainbow", Brightness: 55}); err != nil {
		t.Fatal(err)
	}
	resolution, err := device.resolveLightingSettings("rainbow")
	if err != nil {
		t.Fatal(err)
	}
	custom := resolution.Settings.Clone()
	speed := 4.75
	custom.Speed = &speed
	if err = device.lightingEffects.Set(device.Serial, "rainbow", custom); err != nil {
		t.Fatal(err)
	}
	stateBefore, err := os.ReadFile(paths.OpenRGBDeviceLightingFile)
	if err != nil {
		t.Fatal(err)
	}
	settingsBefore, err := os.ReadFile(paths.DeviceEffectSettingsFile)
	if err != nil {
		t.Fatal(err)
	}

	restarted := &Device{
		Product:      device.Product,
		Serial:       device.Serial,
		IsOpenRGB:    true,
		controllerId: 7,
		colorCount:   1,
		lastColor:    []byte{100, 150, 200},
		RGBModes:     importerSoftwareEffectCatalogue(),
		DeviceProfile: &DeviceProfile{
			Active:     true,
			ZoneColors: map[int]ZoneColors{},
		},
	}
	runtime, err := loadDeviceLightingRuntime(paths)
	if err != nil {
		t.Fatal(err)
	}
	if err = restarted.attachLightingRuntime(runtime); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restarted.Stop)
	if err = restarted.resumeDesiredState(context.Background()); err != nil {
		t.Fatal(err)
	}
	restarted.mu.Lock()
	firstDone := restarted.doneChan
	firstRunner := restarted.rgbRunner
	if !restarted.running || firstDone == nil || firstRunner == nil || firstRunner.RgbModeSpeed != speed || restarted.brightness != 55 {
		restarted.mu.Unlock()
		t.Fatalf("first reconnect state = running %t, done %p, runner %#v, brightness %d", restarted.running, firstDone, firstRunner, restarted.brightness)
	}
	restarted.mu.Unlock()

	if err = restarted.resumeDesiredState(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstDone:
	default:
		t.Fatal("first reconnect worker remained active after replacement")
	}
	restarted.mu.Lock()
	secondDone := restarted.doneChan
	secondRunner := restarted.rgbRunner
	restarted.mu.Unlock()
	if secondDone == nil || secondDone == firstDone || secondRunner == nil || secondRunner.RgbModeSpeed != speed {
		t.Fatalf("second reconnect worker = done %p, first %p, runner %#v", secondDone, firstDone, secondRunner)
	}
	restarted.Stop()

	stateAfter, err := os.ReadFile(paths.OpenRGBDeviceLightingFile)
	if err != nil {
		t.Fatal(err)
	}
	settingsAfter, err := os.ReadFile(paths.DeviceEffectSettingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stateAfter, stateBefore) || !reflect.DeepEqual(settingsAfter, settingsBefore) {
		t.Fatal("reconnect rewrote authoritative lighting persistence")
	}
}

func TestOpenRGBLightingDiscoveryReadsWithoutWritingOrLegacyFallback(t *testing.T) {
	root := t.TempDir()
	paths := deviceLightingPathsForMutableRoot(root)
	paths.ShippedDatabaseRoot = testShippedDatabaseRoot
	restorePaths := config.UsePathsForTest(paths)
	t.Cleanup(restorePaths)

	serial := "openrgb-cutover-discovery"
	legacyRGBPath := filepath.Join(root, "database", "rgb", serial+".json")
	legacyProfilePath := filepath.Join(root, "database", "profiles", serial+".json")
	if err := os.MkdirAll(filepath.Dir(legacyRGBPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyProfilePath), 0o755); err != nil {
		t.Fatal(err)
	}
	legacyRGB := []byte(`{"device":"legacy","profiles":{"rainbow":{"speed":9}}}`)
	legacyProfile := []byte(`{"Active":true,"RGBProfile":"rainbow","BrightnessSlider":1}`)
	if err := os.WriteFile(legacyRGBPath, legacyRGB, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyProfilePath, legacyProfile, 0o644); err != nil {
		t.Fatal(err)
	}

	runtime, err := loadDeviceLightingRuntime(paths)
	if err != nil {
		t.Fatal(err)
	}
	device, err := newOfflineDevice(serial, testConfig(serial, "Discovery"), runtime)
	if err != nil {
		t.Fatal(err)
	}
	if device.effect != "static" || device.brightness != 100 {
		t.Fatalf("discovered state = effect %q, brightness %d", device.effect, device.brightness)
	}
	if _, err = os.Stat(paths.OpenRGBDeviceLightingFile); !os.IsNotExist(err) {
		t.Fatalf("discovery wrote target state: %v", err)
	}
	if _, err = os.Stat(paths.DeviceEffectSettingsFile); !os.IsNotExist(err) {
		t.Fatalf("discovery materialized customization: %v", err)
	}
	for path, want := range map[string][]byte{
		legacyRGBPath:     legacyRGB,
		legacyProfilePath: legacyProfile,
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read legacy file %q: %v", path, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("discovery changed legacy file %q: got %q, want %q", path, got, want)
		}
	}
}

func TestInitAllOfflineReconstructionMixedValidAndInvalidDevices(t *testing.T) {
	validBefore := "openrgb-offline-01-valid"
	invalid := "openrgb-offline-02-invalid"
	validAfter := "openrgb-offline-03-valid"
	testOfflineReconstruction(
		t,
		[]string{validAfter, invalid, validBefore},
		map[string]DeviceLightingState{
			validBefore: {SelectedEffect: "rainbow", Brightness: 31},
			invalid:     {SelectedEffect: "wave", Brightness: 47},
			validAfter:  {SelectedEffect: "colorpulse", Brightness: 82},
		},
		map[string]error{invalid: errors.New("injected selected-state resolution failure")},
		[]string{validBefore, validAfter},
	)
}

func TestInitAllOfflineReconstructionBoundaryFailures(t *testing.T) {
	tests := []struct {
		name        string
		serials     []string
		states      map[string]DeviceLightingState
		failures    map[string]error
		wantSerials []string
	}{
		{
			name:    "first invalid",
			serials: []string{"openrgb-offline-first-01-invalid", "openrgb-offline-first-02-valid"},
			states: map[string]DeviceLightingState{
				"openrgb-offline-first-01-invalid": {SelectedEffect: "wave", Brightness: 20},
				"openrgb-offline-first-02-valid":   {SelectedEffect: "rainbow", Brightness: 64},
			},
			failures: map[string]error{
				"openrgb-offline-first-01-invalid": errors.New("injected first-device resolution failure"),
			},
			wantSerials: []string{"openrgb-offline-first-02-valid"},
		},
		{
			name:    "last invalid",
			serials: []string{"openrgb-offline-last-02-invalid", "openrgb-offline-last-01-valid"},
			states: map[string]DeviceLightingState{
				"openrgb-offline-last-01-valid":   {SelectedEffect: "colorpulse", Brightness: 73},
				"openrgb-offline-last-02-invalid": {SelectedEffect: "wave", Brightness: 24},
			},
			failures: map[string]error{
				"openrgb-offline-last-02-invalid": errors.New("injected last-device resolution failure"),
			},
			wantSerials: []string{"openrgb-offline-last-01-valid"},
		},
		{
			name:    "all invalid",
			serials: []string{"openrgb-offline-all-02-invalid", "openrgb-offline-all-01-invalid"},
			states: map[string]DeviceLightingState{
				"openrgb-offline-all-01-invalid": {SelectedEffect: "rainbow", Brightness: 45},
				"openrgb-offline-all-02-invalid": {SelectedEffect: "wave", Brightness: 55},
			},
			failures: map[string]error{
				"openrgb-offline-all-01-invalid": errors.New("injected first resolution failure"),
				"openrgb-offline-all-02-invalid": errors.New("injected second resolution failure"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testOfflineReconstruction(t, test.serials, test.states, test.failures, test.wantSerials)
		})
	}
}

func TestInitAllOfflineReconstructionSharedRuntimeFailureRemainsGlobal(t *testing.T) {
	preserveOpenRGBStatus(t)
	StopManager()
	setConfiguredDevices(nil)
	t.Cleanup(func() { setConfiguredDevices(nil) })

	root := t.TempDir()
	paths := deviceLightingPathsForMutableRoot(root)
	paths.ShippedDatabaseRoot = filepath.Join(root, "missing-shipped-database")
	restorePaths := config.UsePathsForTest(paths)
	t.Cleanup(restorePaths)
	configPath := useStorePath(t)
	serial := "openrgb-offline-runtime-global-failure"
	if err := saveConfigStore(&ConfigStore{Devices: map[string]DeviceConfig{
		serial: testConfig(serial, "Shared Runtime Failure"),
	}}); err != nil {
		t.Fatal(err)
	}
	configBefore := readOptionalFile(t, configPath)
	stateBefore := readOptionalFile(t, paths.OpenRGBDeviceLightingFile)
	effectsBefore := readOptionalFile(t, paths.DeviceEffectSettingsFile)

	if wrappers := InitAll(); len(wrappers) != 0 {
		t.Fatalf("InitAll returned %d devices after shared runtime failure, want 0", len(wrappers))
	}
	if configured := configuredDevicesSnapshot(); len(configured) != 0 {
		t.Fatalf("configured devices after shared runtime failure = %d, want 0", len(configured))
	}
	state, statusErr := openrgb.GetStatus()
	if state != openrgb.StateOffline || statusErr == nil ||
		!strings.Contains(statusErr.Error(), "load OpenRGB shipped lighting defaults") {
		t.Fatalf("OpenRGB status after shared runtime failure = %q, %v", state, statusErr)
	}
	assertOptionalFileUnchanged(t, configPath, configBefore)
	assertOptionalFileUnchanged(t, paths.OpenRGBDeviceLightingFile, stateBefore)
	assertOptionalFileUnchanged(t, paths.DeviceEffectSettingsFile, effectsBefore)
}

func TestOpenRGBLightingSnapshotUsesAuthoritativeStateAndResolution(t *testing.T) {
	device, _ := newCutoverTestDevice(t, "openrgb-cutover-snapshot")
	if err := device.lightingState.Set(device.Serial, DeviceLightingState{SelectedEffect: "rainbow", Brightness: 35}); err != nil {
		t.Fatal(err)
	}
	resolution, err := device.resolveLightingSettings("rainbow")
	if err != nil {
		t.Fatal(err)
	}
	custom := resolution.Settings.Clone()
	speed := 4.25
	custom.Speed = &speed
	if err = device.lightingEffects.Set(device.Serial, "rainbow", custom); err != nil {
		t.Fatal(err)
	}
	device.effect = "rainbow"
	device.brightness = 35
	device.DeviceProfile.RGBProfile = "obsolete-legacy-effect"
	legacyBrightness := uint8(1)
	device.DeviceProfile.BrightnessSlider = &legacyBrightness

	snapshot, ok := device.LightingSnapshot()
	if !ok || snapshot.ConfiguredEffect != "rainbow" || !snapshot.HasBrightness || snapshot.Brightness != 35 {
		t.Fatalf("authoritative snapshot = %#v, %t", snapshot, ok)
	}
	if !snapshot.HasSpeed || snapshot.Speed != speed || !snapshot.Customized {
		t.Fatalf("resolved authoritative definition = %#v", snapshot)
	}
}
