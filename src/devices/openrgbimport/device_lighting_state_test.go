package openrgbimport

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"LumenForge/src/rgb"
)

type failingDeviceLightingStateWriter struct {
	err error
}

func (writer *failingDeviceLightingStateWriter) Write(string, []byte) error {
	return writer.err
}

func TestDeviceLightingStateStoreDefaultsPersistenceAndIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lighting", "openrgb-device-state.json")
	store, err := LoadDeviceLightingStateStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("loading absent state created %q: %v", path, err)
	}

	defaultState, found, err := store.Resolve("openrgb-device-a")
	if err != nil || found || defaultState != (DeviceLightingState{SelectedEffect: "static", Brightness: 100}) {
		t.Fatalf("clean-install state = %#v, %t, %v", defaultState, found, err)
	}
	first := DeviceLightingState{SelectedEffect: "rainbow", Brightness: 42}
	second := DeviceLightingState{SelectedEffect: "wave", Brightness: 73}
	if err = store.Set("openrgb-device-a", first); err != nil {
		t.Fatal(err)
	}
	if err = store.Set("openrgb-device-b", second); err != nil {
		t.Fatal(err)
	}

	reloaded, err := LoadDeviceLightingStateStore(path)
	if err != nil {
		t.Fatal(err)
	}
	for deviceID, want := range map[string]DeviceLightingState{
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

func TestDeviceLightingStateStoreStrictValidationAndAtomicFailure(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "malformed", data: `{`},
		{name: "unknown field", data: `{"schemaVersion":1,"devices":{},"extra":true}`},
		{name: "wrong schema", data: `{"schemaVersion":2,"devices":{}}`},
		{name: "unknown effect", data: `{"schemaVersion":1,"devices":{"device":{"selectedEffect":"missing","brightness":50}}}`},
		{name: "invalid brightness", data: `{"schemaVersion":1,"devices":{"device":{"selectedEffect":"static","brightness":101}}}`},
		{name: "trailing value", data: `{"schemaVersion":1,"devices":{}} {}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte(test.data), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadDeviceLightingStateStore(path); err == nil {
				t.Fatal("invalid state loaded successfully")
			}
		})
	}

	path := filepath.Join(t.TempDir(), "state.json")
	store, err := LoadDeviceLightingStateStore(path)
	if err != nil {
		t.Fatal(err)
	}
	confirmed := DeviceLightingState{SelectedEffect: "static", Brightness: 35}
	if err = store.Set("device", confirmed); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	store.writer = &failingDeviceLightingStateWriter{err: errors.New("injected write failure")}
	if err = store.Set("device", DeviceLightingState{SelectedEffect: "rainbow", Brightness: 90}); err == nil {
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

func TestDeviceLightingStateValidation(t *testing.T) {
	store, err := LoadDeviceLightingStateStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		id    string
		state DeviceLightingState
	}{
		{name: "empty identity", state: DefaultDeviceLightingState()},
		{name: "identity with slash", id: "device/other", state: DefaultDeviceLightingState()},
		{name: "identity with space", id: "device other", state: DefaultDeviceLightingState()},
		{name: "unknown effect", id: "device", state: DeviceLightingState{SelectedEffect: "unknown", Brightness: 50}},
		{name: "brightness", id: "device", state: DeviceLightingState{SelectedEffect: "static", Brightness: 101}},
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
