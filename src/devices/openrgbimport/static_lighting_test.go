package openrgbimport

import (
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func canonicalTestDeviceStore(t *testing.T, device *Device) *lightingsettings.DeviceStore {
	t.Helper()
	store, ok := device.lightingEffects.(*lightingsettings.DeviceStore)
	if !ok {
		t.Fatalf("canonical device store type = %T", device.lightingEffects)
	}
	return store
}

func TestOpenRGBCanonicalStaticUniformTopologyAndBrightness(t *testing.T) {
	tests := []struct {
		name       string
		zones      []ZoneConfig
		brightness uint8
		wantPixel  []byte
	}{
		{name: "one zone at zero", zones: []ZoneConfig{{Name: "Only", LedCount: 2}}, brightness: 0, wantPixel: []byte{0, 0, 0}},
		{name: "different zones at intermediate", zones: []ZoneConfig{{Name: "Front", LedCount: 2}, {Name: "Center", LedCount: 1}, {Name: "Rear", LedCount: 3}}, brightness: 50, wantPixel: []byte{100, 50, 25}},
		{name: "reordered topology at maximum", zones: []ZoneConfig{{Name: "Rear", LedCount: 3}, {Name: "Front", LedCount: 2}, {Name: "Center", LedCount: 1}}, brightness: 100, wantPixel: []byte{201, 101, 51}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, calls := installLightingDeviceTestSeams(t)
			device := newStaticOverrideTestDevice(test.brightness)
			device.Config = &DeviceConfig{Serial: device.Serial, Product: device.Product, Zones: append([]ZoneConfig(nil), test.zones...)}
			device.ZoneAmount = len(test.zones)
			device.colorCount = configLedCount(device.Config)
			device.LEDCount = device.colorCount
			device.lastColor = []byte{9, 8, 7}
			device.DeviceProfile.ZoneColors = map[int]ZoneColors{
				0: {Color: &rgb.Color{Red: 1, Green: 2, Blue: 3}, ColorIndex: []int{0, 1, 2}, Name: "legacy-a"},
				1: {Color: &rgb.Color{Red: 250, Green: 240, Blue: 230}, ColorIndex: []int{3, 4, 5}, Name: "legacy-b"},
			}
			device.DeviceProfile.RGBOverride = &RGBOverride{Enabled: true, RGBStartColor: rgb.Color{Red: 77, Green: 88, Blue: 99}}
			setCanonicalStaticColor(t, device, rgb.Color{Red: 201, Green: 101, Blue: 51})

			profileBefore := cloneDeviceProfile(device.DeviceProfile)
			configBefore := cloneDeviceConfig(device.Config)
			lastColorBefore := append([]byte(nil), device.lastColor...)
			store := canonicalTestDeviceStore(t, device)
			settingsBefore, found, err := store.Get(device.Serial, defaultDeviceLightingEffect)
			if err != nil || !found {
				t.Fatalf("canonical Static customization = %#v, %t, %v", settingsBefore, found, err)
			}

			if err = device.SetEffect(defaultDeviceLightingEffect); err != nil {
				t.Fatalf("SetEffect(Static): %v", err)
			}
			if calls.colors != 0 || calls.frames != 1 || len(calls.frameValues) != 1 {
				t.Fatalf("Static output calls = colors %d, frames %d, values %v", calls.colors, calls.frames, calls.frameValues)
			}
			want := make([]byte, device.colorCount*3)
			for offset := 0; offset+2 < len(want); offset += 3 {
				copy(want[offset:offset+3], test.wantPixel)
			}
			if !bytes.Equal(calls.frameValues[0], want) {
				t.Fatalf("Static topology frame = %v, want %v", calls.frameValues[0], want)
			}
			if len(calls.frameValues[0]) != configLedCount(device.Config)*3 {
				t.Fatalf("Static frame length = %d, want %d", len(calls.frameValues[0]), configLedCount(device.Config)*3)
			}
			if !reflect.DeepEqual(device.Config, configBefore) || !reflect.DeepEqual(device.DeviceProfile, profileBefore) || !bytes.Equal(device.lastColor, lastColorBefore) {
				t.Fatal("Static rendering mutated topology or legacy profile state")
			}
			settingsAfter, found, err := store.Get(device.Serial, defaultDeviceLightingEffect)
			if err != nil || !found || !reflect.DeepEqual(settingsAfter, settingsBefore) {
				t.Fatalf("Static rendering changed canonical customization: %#v, %t, %v", settingsAfter, found, err)
			}
		})
	}
}

func TestOpenRGBCanonicalStaticResolutionIsolationAndDeletion(t *testing.T) {
	_, calls := installLightingDeviceTestSeams(t)
	deviceA := newLightingMutationDevice()
	deviceA.Serial = "openrgb-static-device-a"
	deviceB := newLightingMutationDevice()
	deviceB.Serial = "openrgb-static-device-b"
	deviceB.lightingState = deviceA.lightingState
	deviceB.lightingEffects = deviceA.lightingEffects
	deviceB.lightingResolver = deviceA.lightingResolver

	store := canonicalTestDeviceStore(t, deviceA)
	if _, found, err := store.Get(deviceA.Serial, defaultDeviceLightingEffect); err != nil || found {
		t.Fatalf("fresh Static customization found=%t err=%v", found, err)
	}
	defaultFrame := canonicalStaticFrame(t, deviceA, 100)
	if want := []byte{0, 255, 255}; !bytes.Equal(defaultFrame, want) {
		t.Fatalf("shipped Static default frame = %v, want %v", defaultFrame, want)
	}
	if err := deviceA.SetEffect(defaultDeviceLightingEffect); err != nil {
		t.Fatalf("select shipped Static default: %v", err)
	}
	if _, found, err := store.Get(deviceA.Serial, defaultDeviceLightingEffect); err != nil || found {
		t.Fatalf("selecting Static materialized customization: found=%t err=%v", found, err)
	}
	wantSelectedDefault := []byte{0, 102, 102}
	if calls.frames != 1 || !bytes.Equal(calls.frameValues[0], wantSelectedDefault) {
		t.Fatalf("selected default output = %v, want %v", calls.frameValues, wantSelectedDefault)
	}

	setCanonicalStaticColor(t, deviceA, rgb.Color{Red: 30, Green: 60, Blue: 90})
	rainbow, err := deviceA.resolveLightingSettings("rainbow")
	if err != nil {
		t.Fatalf("resolve shipped Rainbow settings: %v", err)
	}
	if err = deviceA.lightingEffects.Set(deviceA.Serial, "rainbow", rainbow.Settings); err != nil {
		t.Fatalf("seed unrelated Rainbow customization: %v", err)
	}
	if got := canonicalStaticFrame(t, deviceA, 100); !bytes.Equal(got, []byte{30, 60, 90}) {
		t.Fatalf("device A customized Static = %v", got)
	}
	if got := canonicalStaticFrame(t, deviceB, 100); !bytes.Equal(got, defaultFrame) {
		t.Fatalf("device B Static = %v, want independent default %v", got, defaultFrame)
	}

	resolution, err := deviceA.resolveLightingSettings(defaultDeviceLightingEffect)
	if err != nil || resolution.Settings.SingleColor == nil {
		t.Fatalf("resolve customized Static = %#v, %v", resolution, err)
	}
	resolution.Settings.SingleColor.Color.Red = 255
	if got := canonicalStaticFrame(t, deviceA, 100); !bytes.Equal(got, []byte{30, 60, 90}) {
		t.Fatalf("mutating resolved copy changed stored Static = %v", got)
	}

	deviceA.brightness = 40
	if err = deviceA.SetBrightness(65); err != nil {
		t.Fatalf("change independent Brightness: %v", err)
	}
	customization, found, err := store.Get(deviceA.Serial, defaultDeviceLightingEffect)
	if err != nil || !found || customization.SingleColor == nil || customization.SingleColor.Color != (lightingsettings.Color{Red: 30, Green: 60, Blue: 90}) {
		t.Fatalf("Brightness changed Static customization: %#v, %t, %v", customization, found, err)
	}
	setCanonicalStaticColor(t, deviceA, rgb.Color{Red: 31, Green: 61, Blue: 91})
	state, found, err := deviceA.lightingState.Resolve(deviceA.Serial)
	if err != nil || !found || state.Brightness != 65 {
		t.Fatalf("Static customization changed target Brightness: %#v, %t, %v", state, found, err)
	}

	deleted, err := store.Delete(deviceA.Serial, defaultDeviceLightingEffect)
	if err != nil || !deleted {
		t.Fatalf("delete test-seeded Static customization = %t, %v", deleted, err)
	}
	if got := canonicalStaticFrame(t, deviceA, 100); !bytes.Equal(got, defaultFrame) {
		t.Fatalf("Static after deletion = %v, want shipped default %v", got, defaultFrame)
	}
}

func TestOpenRGBCanonicalStaticProfileSwitchCannotChangeColor(t *testing.T) {
	profileDir, calls := installLightingDeviceTestSeams(t)
	device := newStaticOverrideTestDevice(40)
	setCanonicalStaticColor(t, device, rgb.Color{Red: 120, Green: 80, Blue: 40})
	current := cloneDeviceProfile(device.DeviceProfile)
	current.Active = true
	current.Path = filepath.Join(profileDir, device.Serial+".json")
	other := cloneDeviceProfile(device.DeviceProfile)
	other.Active = false
	other.Path = filepath.Join(profileDir, device.Serial+"-other.json")
	other.ZoneColors = map[int]ZoneColors{
		0: {Color: &rgb.Color{Red: 1, Green: 2, Blue: 3}, ColorIndex: []int{0, 1, 2, 3, 4, 5}},
		1: {Color: &rgb.Color{Red: 250, Green: 249, Blue: 248}, ColorIndex: []int{6, 7, 8}},
	}
	other.RGBOverride = &RGBOverride{Enabled: true, RGBStartColor: rgb.Color{Red: 9, Green: 19, Blue: 29}}
	device.DeviceProfile = current
	device.UserProfiles = map[string]*DeviceProfile{"default": current, "other": other}

	if result := device.ChangeDeviceProfile("other"); result != 1 {
		t.Fatalf("ChangeDeviceProfile(other) = %d, want 1", result)
	}
	want := []byte{48, 32, 16, 48, 32, 16, 48, 32, 16}
	if calls.colors != 0 || calls.frames != 1 || !bytes.Equal(calls.frameValues[0], want) {
		t.Fatalf("profile-switch Static output = colors %d frames %v, want %v", calls.colors, calls.frameValues, want)
	}
	if device.brightness != 40 || device.effect != defaultDeviceLightingEffect {
		t.Fatalf("profile switch changed target state: effect %q brightness %d", device.effect, device.brightness)
	}
	settings, found, err := canonicalTestDeviceStore(t, device).Get(device.Serial, defaultDeviceLightingEffect)
	if err != nil || !found || settings.SingleColor == nil || settings.SingleColor.Color != (lightingsettings.Color{Red: 120, Green: 80, Blue: 40}) {
		t.Fatalf("profile switch changed canonical Static settings: %#v, %t, %v", settings, found, err)
	}
}

func TestOpenRGBCanonicalStaticRestartReconnectAndCleanBreak(t *testing.T) {
	profileDir, calls := installLightingDeviceTestSeams(t)
	root := t.TempDir()
	paths := deviceLightingPathsForMutableRoot(root)
	paths.ShippedDatabaseRoot = testShippedDatabaseRoot
	runtime, err := loadDeviceLightingRuntime(paths)
	if err != nil {
		t.Fatalf("load canonical lighting runtime: %v", err)
	}
	serial := "openrgb-static-clean-break"
	if err = runtime.state.Set(serial, DeviceLightingState{SelectedEffect: defaultDeviceLightingEffect, Brightness: 50}); err != nil {
		t.Fatalf("seed target state: %v", err)
	}
	settings := lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      defaultDeviceLightingEffect,
		SingleColor:   &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 80, Green: 40, Blue: 20}},
	}
	if err = runtime.effects.Set(serial, defaultDeviceLightingEffect, settings); err != nil {
		t.Fatalf("seed canonical Static customization: %v", err)
	}

	legacyRGBPath := filepath.Join(root, "database", "rgb", serial+".json")
	if err = os.MkdirAll(filepath.Dir(legacyRGBPath), 0o700); err != nil {
		t.Fatal(err)
	}
	legacyRGB := []byte(`{"device":"legacy","profiles":{"static":{"start":{"red":255,"green":1,"blue":2}}}}`)
	if err = os.WriteFile(legacyRGBPath, legacyRGB, 0o600); err != nil {
		t.Fatal(err)
	}
	profile := DeviceProfile{
		Active:           true,
		RGBProfile:       defaultDeviceLightingEffect,
		BrightnessSlider: func() *uint8 { value := uint8(3); return &value }(),
		ZoneColors: map[int]ZoneColors{
			0: {Color: &rgb.Color{Red: 1, Green: 250, Blue: 2}, ColorIndex: []int{0, 1, 2}, Name: "legacy"},
		},
		RGBOverride: &RGBOverride{Enabled: true, RGBStartColor: rgb.Color{Red: 200, Green: 210, Blue: 220}},
	}
	profileData, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(profileDir, serial+".json")
	if err = os.WriteFile(profilePath, profileData, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := DeviceConfig{Serial: serial, Product: "Canonical Static", Zones: []ZoneConfig{{Name: "First", LedCount: 1}, {Name: "Second", LedCount: 2}}}
	device, err := newOfflineDevice(serial, cfg, runtime)
	if err != nil {
		t.Fatalf("offline reconstruction: %v", err)
	}
	device.lastColor = []byte{250, 3, 4}
	device.controllerId = 7
	stateBefore, err := os.ReadFile(paths.OpenRGBDeviceLightingFile)
	if err != nil {
		t.Fatal(err)
	}
	effectsBefore, err := os.ReadFile(paths.DeviceEffectSettingsFile)
	if err != nil {
		t.Fatal(err)
	}
	profileBefore, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}

	if err = device.resumeDesiredState(context.Background()); err != nil {
		t.Fatalf("reconnect Static replay: %v", err)
	}
	want := []byte{40, 20, 10, 40, 20, 10, 40, 20, 10}
	if calls.colors != 0 || calls.frames != 1 || !bytes.Equal(calls.frameValues[0], want) {
		t.Fatalf("reconnected Static output = colors %d frames %v, want %v", calls.colors, calls.frameValues, want)
	}
	if device.brightness != 50 || device.effect != defaultDeviceLightingEffect || device.running || device.stopChan != nil || device.doneChan != nil {
		t.Fatalf("reconstructed desired state = effect %q brightness %d worker %t/%v/%v", device.effect, device.brightness, device.running, device.stopChan, device.doneChan)
	}
	for path, before := range map[string][]byte{
		paths.OpenRGBDeviceLightingFile: stateBefore,
		paths.DeviceEffectSettingsFile:  effectsBefore,
		legacyRGBPath:                   legacyRGB,
		profilePath:                     profileBefore,
	} {
		after, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("restart/reconnect rewrote %q: err=%v before=%q after=%q", path, readErr, before, after)
		}
	}
}

func TestOpenRGBCanonicalStaticMalformedStateDoesNotUseLegacyFallback(t *testing.T) {
	root := t.TempDir()
	paths := deviceLightingPathsForMutableRoot(root)
	paths.ShippedDatabaseRoot = testShippedDatabaseRoot
	if err := os.MkdirAll(filepath.Dir(paths.DeviceEffectSettingsFile), 0o700); err != nil {
		t.Fatal(err)
	}
	malformed := []byte(`{"schemaVersion":1,"devices":{"openrgb-malformed-static":{"static":{"schemaVersion":1,"effectId":"static"}}}}`)
	if err := os.WriteFile(paths.DeviceEffectSettingsFile, malformed, 0o600); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(root, "database", "rgb", "openrgb-malformed-static.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte(`{"static":"legacy fallback"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := loadDeviceLightingRuntime(paths)
	if err == nil || !strings.Contains(err.Error(), "complete single-color settings are required") {
		t.Fatalf("malformed canonical Static runtime error = %v", err)
	}
	after, readErr := os.ReadFile(legacyPath)
	if readErr != nil || !bytes.Equal(after, []byte(`{"static":"legacy fallback"}`)) {
		t.Fatalf("legacy fallback was changed: %q, %v", after, readErr)
	}
}
