package cluster

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type clusterWriterFunc func(string, []byte) error

func (function clusterWriterFunc) Write(path string, data []byte) error {
	return function(path, data)
}

func TestClusterStateAndLayoutStoresLoadDefaultsWithoutCreatingFiles(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(root, "lighting", "rgb-cluster-state.json")
	layoutPath := filepath.Join(root, "rgb-cluster-layout.json")
	stateStore, err := loadClusterLightingStateStore(statePath)
	if err != nil {
		t.Fatal(err)
	}
	state, err := stateStore.Snapshot()
	if err != nil || state != defaultClusterLightingState() {
		t.Fatalf("default state = %#v, %v", state, err)
	}
	layoutStore, err := loadClusterLayoutStore(layoutPath)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := layoutStore.Snapshot()
	if err != nil || len(layout.DeviceOrder) != 0 {
		t.Fatalf("default layout = %#v, %v", layout, err)
	}
	for _, path := range []string{statePath, layoutPath} {
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("loading created %q: %v", path, statErr)
		}
	}
}

func TestClusterStateAndLayoutPersistReloadAndCopy(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(root, "lighting", "rgb-cluster-state.json")
	layoutPath := filepath.Join(root, "rgb-cluster-layout.json")
	stateStore, err := loadClusterLightingStateStore(statePath)
	if err != nil {
		t.Fatal(err)
	}
	wantState := clusterLightingState{SchemaVersion: clusterPersistenceSchemaVersion, SelectedEffect: "static", Brightness: 42}
	if err = stateStore.Set(wantState); err != nil {
		t.Fatal(err)
	}
	reloadedState, err := loadClusterLightingStateStore(statePath)
	if err != nil {
		t.Fatal(err)
	}
	gotState, err := reloadedState.Snapshot()
	if err != nil || gotState != wantState {
		t.Fatalf("reloaded state = %#v, %v", gotState, err)
	}

	layoutStore, err := loadClusterLayoutStore(layoutPath)
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{"serial-b", "serial-a"}
	if err = layoutStore.Set(clusterLayout{SchemaVersion: clusterPersistenceSchemaVersion, DeviceOrder: wantOrder}); err != nil {
		t.Fatal(err)
	}
	wantOrder[0] = "caller-mutated"
	stored, err := layoutStore.Snapshot()
	if err != nil || !reflect.DeepEqual(stored.DeviceOrder, []string{"serial-b", "serial-a"}) {
		t.Fatalf("stored layout = %#v, %v", stored, err)
	}
	stored.DeviceOrder[0] = "snapshot-mutated"
	again, err := layoutStore.Snapshot()
	if err != nil || !reflect.DeepEqual(again.DeviceOrder, []string{"serial-b", "serial-a"}) {
		t.Fatalf("layout snapshot aliased store = %#v, %v", again, err)
	}
	reloadedLayout, err := loadClusterLayoutStore(layoutPath)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := reloadedLayout.Snapshot()
	if err != nil || !reflect.DeepEqual(reloaded.DeviceOrder, []string{"serial-b", "serial-a"}) {
		t.Fatalf("reloaded layout = %#v, %v", reloaded, err)
	}
}

func TestClusterStateAndLayoutRejectMalformedAndPreserveMemoryOnWriteFailure(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string][]byte{
		"state-schema.json":     []byte(`{"schemaVersion":2,"selectedEffect":"rainbow","brightness":100}`),
		"state-effect.json":     []byte(`{"schemaVersion":1,"selectedEffect":"unknown","brightness":100}`),
		"state-brightness.json": []byte(`{"schemaVersion":1,"selectedEffect":"rainbow","brightness":101}`),
		"layout-schema.json":    []byte(`{"schemaVersion":2,"deviceOrder":[]}`),
		"layout-duplicate.json": []byte(`{"schemaVersion":1,"deviceOrder":["same","same"]}`),
	} {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if len(name) >= 5 && name[:5] == "state" {
			if _, err := loadClusterLightingStateStore(path); err == nil {
				t.Fatalf("malformed state %q loaded", name)
			}
		} else if _, err := loadClusterLayoutStore(path); err == nil {
			t.Fatalf("malformed layout %q loaded", name)
		}
	}

	writeErr := errors.New("injected write failure")
	stateStore, err := loadClusterLightingStateStoreWithWriter(filepath.Join(root, "state.json"), clusterWriterFunc(func(string, []byte) error { return writeErr }))
	if err != nil {
		t.Fatal(err)
	}
	if err = stateStore.Set(clusterLightingState{SchemaVersion: 1, SelectedEffect: "static", Brightness: 50}); !errors.Is(err, writeErr) {
		t.Fatalf("state Set error = %v", err)
	}
	state, _ := stateStore.Snapshot()
	if state != defaultClusterLightingState() {
		t.Fatalf("failed state write changed memory: %#v", state)
	}

	layoutStore, err := loadClusterLayoutStoreWithWriter(filepath.Join(root, "layout.json"), clusterWriterFunc(func(string, []byte) error { return writeErr }))
	if err != nil {
		t.Fatal(err)
	}
	if err = layoutStore.Set(clusterLayout{SchemaVersion: 1, DeviceOrder: []string{"changed"}}); !errors.Is(err, writeErr) {
		t.Fatalf("layout Set error = %v", err)
	}
	layout, _ := layoutStore.Snapshot()
	if len(layout.DeviceOrder) != 0 {
		t.Fatalf("failed layout write changed memory: %#v", layout)
	}
}
