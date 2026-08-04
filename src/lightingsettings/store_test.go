package lightingsettings

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

type writerFunc func(path string, data []byte) error

func (function writerFunc) Write(path string, data []byte) error {
	return function(path, data)
}

func TestDeviceStoreIsolationDeletionReloadAndDefensiveCopies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lighting", "independent-device-effects.json")
	store, err := LoadDeviceStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("loading absent store created file: %v", err)
	}

	firstStatic := testStaticSettings(1)
	secondStatic := testStaticSettings(2)
	firstWave := testWaveSettings(5)
	if err = store.Set("device-one", "static", firstStatic); err != nil {
		t.Fatal(err)
	}
	if err = store.Set("device-one", "wave", firstWave); err != nil {
		t.Fatal(err)
	}
	if err = store.Set("device-two", "static", secondStatic); err != nil {
		t.Fatal(err)
	}

	got, found, err := store.Get("device-one", "static")
	if err != nil || !found || !reflect.DeepEqual(got, firstStatic) {
		t.Fatalf("device-one Static = %#v, %t, %v", got, found, err)
	}
	got.SingleColor.Color.Red = 99
	later, found, err := store.Get("device-one", "static")
	if err != nil || !found || later.SingleColor.Color.Red != 1 {
		t.Fatalf("mutating returned record changed store: %#v, %t, %v", later, found, err)
	}

	reloaded, err := LoadDeviceStore(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		device string
		effect string
		want   EffectSettings
	}{
		{device: "device-one", effect: "static", want: firstStatic},
		{device: "device-one", effect: "wave", want: firstWave},
		{device: "device-two", effect: "static", want: secondStatic},
	} {
		value, ok, getErr := reloaded.Get(check.device, check.effect)
		if getErr != nil || !ok || !reflect.DeepEqual(value, check.want) {
			t.Errorf("reloaded %s/%s = %#v, %t, %v", check.device, check.effect, value, ok, getErr)
		}
	}
	if _, found, err = reloaded.Get("device-two", "wave"); err != nil || found {
		t.Fatalf("unwritten default materialized = %t, %v", found, err)
	}

	deleted, err := reloaded.Delete("device-one", "static")
	if err != nil || !deleted {
		t.Fatalf("Delete(existing) = %t, %v", deleted, err)
	}
	deleted, err = reloaded.Delete("device-one", "static")
	if err != nil || deleted {
		t.Fatalf("Delete(missing) = %t, %v", deleted, err)
	}
	if _, found, _ = reloaded.Get("device-one", "static"); found {
		t.Fatal("deleted customization remains")
	}
	if _, found, _ = reloaded.Get("device-one", "wave"); !found {
		t.Fatal("deleting one effect removed another effect")
	}
	if _, found, _ = reloaded.Get("device-two", "static"); !found {
		t.Fatal("deleting one device customization affected another device")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("store permissions = %o", info.Mode().Perm())
	}
}

func TestDeviceStoreRejectsInvalidAndMalformedRecords(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "devices.json")
	writes := 0
	store, err := loadDeviceStore(path, writerFunc(func(_ string, _ []byte) error {
		writes++
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	incomplete := EffectSettings{SchemaVersion: SchemaVersion, EffectID: "static"}
	if err = store.Set("device", "static", incomplete); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("Set(incomplete) error = %v", err)
	}
	mismatch := testStaticSettings(1)
	if err = store.Set("device", "wave", mismatch); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("Set(mismatched key) error = %v", err)
	}
	if writes != 0 {
		t.Fatalf("invalid records performed %d writes", writes)
	}

	malformedCases := [][]byte{
		[]byte(`{"schemaVersion":1,"devices":`),
		[]byte(`{"devices":{}}`),
		[]byte(`{"schemaVersion":1}`),
		[]byte(`{"schemaVersion":2,"devices":{}}`),
		[]byte(`{"schemaVersion":1,"devices":{"device":{"static":{"schemaVersion":1,"effectId":"static"}}}}`),
		[]byte(`{"schemaVersion":1,"devices":{},"unknown":true}`),
	}
	for index, data := range malformedCases {
		if err = os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err = LoadDeviceStore(path); err == nil {
			t.Errorf("malformed fixture %d loaded successfully", index)
		}
	}
}

func TestDeviceStoreFailedAtomicWritePreservesStateAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	store, err := LoadDeviceStore(path)
	if err != nil {
		t.Fatal(err)
	}
	original := testStaticSettings(1)
	if err = store.Set("device", "static", original); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	writeErr := errors.New("injected write failure")
	failing, err := loadDeviceStore(path, writerFunc(func(_ string, _ []byte) error { return writeErr }))
	if err != nil {
		t.Fatal(err)
	}
	if err = failing.Set("device", "static", testStaticSettings(2)); !errors.Is(err, writeErr) {
		t.Fatalf("Set() error = %v", err)
	}
	current, found, err := failing.Get("device", "static")
	if err != nil || !found || !reflect.DeepEqual(current, original) {
		t.Fatalf("failed write changed memory: %#v, %t, %v", current, found, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed write changed the prior valid file")
	}
}

func TestClusterStoreIsolationDeletionReloadAndDefensiveCopies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lighting", "rgb-cluster-effects.json")
	store, err := LoadClusterStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("loading absent store created file: %v", err)
	}
	static := testStaticSettings(3)
	wave := testWaveSettings(6)
	if err = store.Set("static", static); err != nil {
		t.Fatal(err)
	}
	if err = store.Set("wave", wave); err != nil {
		t.Fatal(err)
	}
	returned, found, err := store.Get("wave")
	if err != nil || !found {
		t.Fatalf("Get(wave) = %#v, %t, %v", returned, found, err)
	}
	*returned.Speed = 9
	returned.TwoColor.Start.Red = 200
	later, found, err := store.Get("wave")
	if err != nil || !found || *later.Speed != 6 || later.TwoColor.Start.Red != 10 {
		t.Fatalf("returned cluster record aliased store: %#v, %t, %v", later, found, err)
	}

	reloaded, err := LoadClusterStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if value, ok, getErr := reloaded.Get("static"); getErr != nil || !ok || !reflect.DeepEqual(value, static) {
		t.Fatalf("reloaded Static = %#v, %t, %v", value, ok, getErr)
	}
	deleted, err := reloaded.Delete("static")
	if err != nil || !deleted {
		t.Fatalf("Delete(static) = %t, %v", deleted, err)
	}
	if _, found, _ = reloaded.Get("wave"); !found {
		t.Fatal("deleting cluster Static removed cluster Wave")
	}
	if deleted, err = reloaded.Delete("static"); err != nil || deleted {
		t.Fatalf("Delete(missing) = %t, %v", deleted, err)
	}
}

func TestClusterStoreRejectsMalformedContentAndPreservesStateOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cluster.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"effects":{"static":{"schemaVersion":1,"effectId":"static"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadClusterStore(path); err == nil {
		t.Fatal("incomplete persisted cluster record loaded successfully")
	}

	real, err := LoadClusterStore(filepath.Join(t.TempDir(), "cluster.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = real.Set("gradient", testGradientSettings(5)); err != nil {
		t.Fatal(err)
	}
	failing, err := loadClusterStore(real.path, writerFunc(func(_ string, _ []byte) error { return errors.New("failure") }))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = failing.Delete("gradient"); err == nil {
		t.Fatal("Delete() succeeded through failing writer")
	}
	if _, found, err := failing.Get("gradient"); err != nil || !found {
		t.Fatalf("failed delete changed cluster state: %t, %v", found, err)
	}
}

func TestFinalCustomizationDeletionPersistsValidEmptyStores(t *testing.T) {
	root := t.TempDir()
	devicePath := filepath.Join(root, "devices.json")
	deviceStore, err := LoadDeviceStore(devicePath)
	if err != nil {
		t.Fatal(err)
	}
	if err = deviceStore.Set("device", "static", testStaticSettings(1)); err != nil {
		t.Fatal(err)
	}
	if deleted, deleteErr := deviceStore.Delete("device", "static"); deleteErr != nil || !deleted {
		t.Fatalf("device final Delete() = %t, %v", deleted, deleteErr)
	}
	reloadedDevice, err := LoadDeviceStore(devicePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, getErr := reloadedDevice.Get("device", "static"); getErr != nil || found {
		t.Fatalf("reloaded empty device store = %t, %v", found, getErr)
	}

	clusterPath := filepath.Join(root, "cluster.json")
	clusterStore, err := LoadClusterStore(clusterPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = clusterStore.Set("static", testStaticSettings(2)); err != nil {
		t.Fatal(err)
	}
	if deleted, deleteErr := clusterStore.Delete("static"); deleteErr != nil || !deleted {
		t.Fatalf("cluster final Delete() = %t, %v", deleted, deleteErr)
	}
	reloadedCluster, err := LoadClusterStore(clusterPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, getErr := reloadedCluster.Get("static"); getErr != nil || found {
		t.Fatalf("reloaded empty cluster store = %t, %v", found, getErr)
	}
}

func TestStoresSupportConcurrentReadsAndWrites(t *testing.T) {
	root := t.TempDir()
	devices, err := LoadDeviceStore(filepath.Join(root, "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	cluster, err := LoadClusterStore(filepath.Join(root, "cluster.json"))
	if err != nil {
		t.Fatal(err)
	}

	var wait sync.WaitGroup
	errorsChannel := make(chan error, 32)
	for worker := 0; worker < 8; worker++ {
		deviceID := "device-" + string(rune('a'+worker))
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < 20; iteration++ {
				if setErr := devices.Set(deviceID, "static", testStaticSettings(float64(iteration))); setErr != nil {
					errorsChannel <- setErr
					return
				}
				if _, _, getErr := devices.Get(deviceID, "static"); getErr != nil {
					errorsChannel <- getErr
					return
				}
			}
		}()
	}
	for _, effect := range []string{"static", "wave"} {
		effect := effect
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 1; iteration <= 20; iteration++ {
				settings := testStaticSettings(float64(iteration))
				if effect == "wave" {
					settings = testWaveSettings(5)
				}
				if setErr := cluster.Set(effect, settings); setErr != nil {
					errorsChannel <- setErr
					return
				}
				if _, _, getErr := cluster.Get(effect); getErr != nil {
					errorsChannel <- getErr
					return
				}
			}
		}()
	}
	wait.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		t.Error(err)
	}
	for worker := 0; worker < 8; worker++ {
		deviceID := "device-" + string(rune('a'+worker))
		if _, found, getErr := devices.Get(deviceID, "static"); getErr != nil || !found {
			t.Errorf("concurrent device record %q = %t, %v", deviceID, found, getErr)
		}
	}
	for _, effect := range []string{"static", "wave"} {
		if _, found, getErr := cluster.Get(effect); getErr != nil || !found {
			t.Errorf("concurrent cluster record %q = %t, %v", effect, found, getErr)
		}
	}
}
