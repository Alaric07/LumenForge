package lightingsettings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"LumenForge/src/config"
)

func TestFoundationPathsUseDedicatedStoresWithoutLegacyMigration(t *testing.T) {
	root := t.TempDir()
	applicationRoot := filepath.Join(root, "app")
	dataRoot := filepath.Join(root, "data")
	paths, err := config.ResolvePaths(config.PathOptions{
		Mode:            config.ServiceModeUser,
		ApplicationRoot: applicationRoot,
		ConfigRoot:      filepath.Join(root, "config"),
		DataRoot:        dataRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = config.EnsureRuntimeDirectories(paths); err != nil {
		t.Fatal(err)
	}
	shippedData, err := os.ReadFile(shippedDefaultsPath(t))
	if err != nil {
		t.Fatal(err)
	}
	shippedPath := filepath.Join(paths.ShippedDatabaseRoot, "rgb.json")
	if err = os.MkdirAll(paths.ShippedDatabaseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(shippedPath, shippedData, 0o444); err != nil {
		t.Fatal(err)
	}

	legacyDevicePath := filepath.Join(paths.MutableRGBRoot, "device-one.json")
	legacyClusterPath := filepath.Join(paths.MutableRGBRoot, "cluster.json")
	legacyExperimentalPath := filepath.Join(paths.MutableLightingRoot, "legacy-experiment.json")
	legacyData := []byte(`{"legacy":true}`)
	for _, path := range []string{legacyDevicePath, legacyClusterPath, legacyExperimentalPath} {
		if err = os.WriteFile(path, legacyData, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	defaults, err := LoadDefaultRepository(shippedPath)
	if err != nil {
		t.Fatal(err)
	}
	devices, err := LoadDeviceStore(paths.DeviceEffectSettingsFile)
	if err != nil {
		t.Fatal(err)
	}
	cluster, err := LoadClusterStore(paths.ClusterEffectSettingsFile)
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := NewResolver(defaults, devices, cluster)
	if err != nil {
		t.Fatal(err)
	}
	if result, resolveErr := resolver.Resolve(IndependentDevice("device-one"), "static"); resolveErr != nil || result.Customized {
		t.Fatalf("legacy device data affected resolution: %#v, %v", result, resolveErr)
	}
	if result, resolveErr := resolver.Resolve(RGBCluster(), "static"); resolveErr != nil || result.Customized {
		t.Fatalf("legacy cluster data affected resolution: %#v, %v", result, resolveErr)
	}
	for _, path := range []string{paths.DeviceEffectSettingsFile, paths.ClusterEffectSettingsFile} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("read-only resolution created compatibility store %q: %v", path, statErr)
		}
	}

	if err = devices.Set("device-one", "static", testStaticSettings(1)); err != nil {
		t.Fatal(err)
	}
	if err = cluster.Set("static", testStaticSettings(2)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.DeviceEffectSettingsFile, paths.ClusterEffectSettingsFile} {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("dedicated store %q was not written: %v", path, statErr)
		}
		if filepath.Dir(path) != paths.MutableLightingRoot {
			t.Fatalf("dedicated store escaped lighting root: %q", path)
		}
	}
	for _, path := range []string{legacyDevicePath, legacyClusterPath, legacyExperimentalPath} {
		contents, readErr := os.ReadFile(path)
		if readErr != nil || !reflect.DeepEqual(contents, legacyData) {
			t.Fatalf("legacy file %q changed: %q, %v", path, contents, readErr)
		}
	}
	afterShipped, err := os.ReadFile(shippedPath)
	if err != nil || !reflect.DeepEqual(afterShipped, shippedData) {
		t.Fatalf("shipped defaults changed: %v", err)
	}
}
