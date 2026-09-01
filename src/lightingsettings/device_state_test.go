package lightingsettings

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"LumenForge/src/rgb"
)

type failingIndependentDeviceStateWriter struct {
	err error
}

func (writer *failingIndependentDeviceStateWriter) Write(string, []byte) error {
	return writer.err
}

func TestIndependentDeviceStateStoreDefaultsPersistenceAndIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lighting", "openrgb-device-state.json")
	store, err := LoadIndependentDeviceStateStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("loading absent state created %q: %v", path, err)
	}

	defaultState, found, err := store.Resolve("openrgb-device-a")
	if err != nil || found || defaultState != (IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 100}) {
		t.Fatalf("clean-install state = %#v, %t, %v", defaultState, found, err)
	}
	first := IndependentDeviceLightingState{SelectedEffect: "rainbow", Brightness: 42}
	second := IndependentDeviceLightingState{SelectedEffect: "wave", Brightness: 73}
	if err = store.Set("openrgb-device-a", first); err != nil {
		t.Fatal(err)
	}
	if err = store.Set("openrgb-device-b", second); err != nil {
		t.Fatal(err)
	}

	reloaded, err := LoadIndependentDeviceStateStore(path)
	if err != nil {
		t.Fatal(err)
	}
	for deviceID, want := range map[string]IndependentDeviceLightingState{
		"openrgb-device-a": first,
		"openrgb-device-b": second,
	} {
		got, ok, resolveErr := reloaded.Resolve(deviceID)
		if resolveErr != nil || !ok || got != want {
			t.Fatalf("reloaded %q = %#v, %t, %v; want %#v", deviceID, got, ok, resolveErr, want)
		}
	}
	returned, _, err := reloaded.Resolve("openrgb-device-a")
	if err != nil {
		t.Fatal(err)
	}
	returned.SelectedEffect = "off"
	returned.Brightness = 0
	unchanged, _, err := reloaded.Resolve("openrgb-device-a")
	if err != nil || unchanged != first {
		t.Fatalf("caller mutation changed stored state: %#v, %v", unchanged, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state permissions = %o, want 600", info.Mode().Perm())
	}
	directoryInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("state directory permissions = %o, want 700", directoryInfo.Mode().Perm())
	}

	updated := first
	updated.SelectedEffect = "off"
	if err = reloaded.Set("openrgb-device-a", updated); err != nil {
		t.Fatal(err)
	}
	gotSecond, _, _ := reloaded.Resolve("openrgb-device-b")
	if gotSecond != second {
		t.Fatalf("updating first device changed second: %#v", gotSecond)
	}
	deleted, err := reloaded.Delete("openrgb-device-a")
	if err != nil || !deleted {
		t.Fatalf("Delete = %t, %v", deleted, err)
	}
	gotSecond, _, _ = reloaded.Resolve("openrgb-device-b")
	if gotSecond != second {
		t.Fatalf("deleting first device changed second: %#v", gotSecond)
	}
}

func TestIndependentDeviceStateStoreDocumentValidationAndAtomicFailure(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "malformed", data: `{`},
		{name: "unknown field", data: `{"schemaVersion":1,"devices":{},"extra":true}`},
		{name: "wrong schema", data: `{"schemaVersion":2,"devices":{}}`},
		{name: "missing records", data: `{"schemaVersion":1}`},
		{name: "trailing value", data: `{"schemaVersion":1,"devices":{}} {}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte(test.data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadIndependentDeviceStateStore(path); err == nil {
				t.Fatal("invalid state loaded successfully")
			}
		})
	}

	path := filepath.Join(t.TempDir(), "state.json")
	store, err := LoadIndependentDeviceStateStore(path)
	if err != nil {
		t.Fatal(err)
	}
	confirmed := IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 35}
	if err = store.Set("device", confirmed); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	store.writer = &failingIndependentDeviceStateWriter{err: errors.New("injected write failure")}
	if err = store.Set("device", IndependentDeviceLightingState{SelectedEffect: "rainbow", Brightness: 90}); err == nil {
		t.Fatal("Set succeeded despite writer failure")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("failed write changed the persisted file")
	}
	got, found, err := store.Resolve("device")
	if err != nil || !found || got != confirmed {
		t.Fatalf("failed write changed in-memory state: %#v, %t, %v", got, found, err)
	}
}

func TestIndependentDeviceStateStoreDiscardsInvalidRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	data := []byte(`{
  "schemaVersion": 1,
  "devices": {
    "valid-device": {"selectedEffect": "rainbow", "brightness": 42},
    "stale-effect": {"selectedEffect": "removed-effect", "brightness": 50},
    "invalid/identity": {"selectedEffect": "static", "brightness": 60},
    "invalid-brightness": {"selectedEffect": "static", "brightness": 101}
  }
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := LoadIndependentDeviceStateStore(path)
	if err != nil {
		t.Fatalf("load state with invalid records: %v", err)
	}
	valid, found, err := store.Resolve("valid-device")
	if err != nil || !found || valid != (IndependentDeviceLightingState{SelectedEffect: "rainbow", Brightness: 42}) {
		t.Fatalf("valid state = %#v, %t, %v", valid, found, err)
	}

	for _, deviceID := range []string{"stale-effect", "invalid-brightness"} {
		state, stateFound, resolveErr := store.Resolve(deviceID)
		if resolveErr != nil || stateFound || state != DefaultIndependentDeviceLightingState() {
			t.Fatalf("discarded state %q = %#v, %t, %v", deviceID, state, stateFound, resolveErr)
		}
	}
	if _, found = store.devices["invalid/identity"]; found {
		t.Fatal("invalid identity was retained")
	}
}

func TestIndependentDeviceStateValidation(t *testing.T) {
	store, err := LoadIndependentDeviceStateStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		id    string
		state IndependentDeviceLightingState
	}{
		{name: "empty identity", state: DefaultIndependentDeviceLightingState()},
		{name: "identity with slash", id: "device/other", state: DefaultIndependentDeviceLightingState()},
		{name: "identity with space", id: "device other", state: DefaultIndependentDeviceLightingState()},
		{name: "unknown effect", id: "device", state: IndependentDeviceLightingState{SelectedEffect: "unknown", Brightness: 50}},
		{name: "brightness", id: "device", state: IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 101}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := store.Set(test.id, test.state); err == nil {
				t.Fatal("invalid state was accepted")
			}
		})
	}
	if independentDeviceEffectDescriptor(rgb.SoftwareEffectDescriptor{Scope: rgb.EffectScopeCluster}, true) {
		t.Fatal("RGB Cluster-only effect was accepted for independent-device state")
	}
}

func TestIndependentDeviceStateStoreAcceptsKeyboardSpecialEffectOnly(t *testing.T) {
	store, err := LoadIndependentDeviceStateStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	keyboard := IndependentDeviceLightingState{SelectedEffect: "keyboard", Brightness: 50}
	if err := store.Set("k95", keyboard); err != nil {
		t.Fatalf("Set keyboard special effect: %v", err)
	}
	got, found, err := store.Resolve("k95")
	if err != nil || !found || got != keyboard {
		t.Fatalf("keyboard state = %#v, %t, %v", got, found, err)
	}
	if err := store.Set("k95", IndependentDeviceLightingState{SelectedEffect: "not-a-real-effect", Brightness: 50}); err == nil {
		t.Fatal("arbitrary unknown effect was accepted")
	}
}

func TestIndependentDeviceStateStoreAcceptsMemoryLedSpecialEffect(t *testing.T) {
	store, err := LoadIndependentDeviceStateStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	led := IndependentDeviceLightingState{SelectedEffect: "led", Brightness: 50}
	if err := store.Set("i2c0-rgb-0", led); err != nil {
		t.Fatalf("Set Memory led special effect: %v", err)
	}
}
