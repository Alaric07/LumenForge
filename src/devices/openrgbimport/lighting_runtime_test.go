package openrgbimport

import (
	"os"
	"path/filepath"
	"testing"
)

func removeTestDeviceLightingRuntime(t *testing.T, runtime *deviceLightingRuntime) {
	t.Helper()
	t.Cleanup(func() {
		deviceLightingRuntimeCache.Lock()
		defer deviceLightingRuntimeCache.Unlock()
		for key, cached := range deviceLightingRuntimeCache.values {
			if cached == runtime {
				delete(deviceLightingRuntimeCache.values, key)
			}
		}
	})
}

func writeMalformedClusterStore(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"effects":`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDeviceLightingRuntimeDoesNotLoadClusterStore(t *testing.T) {
	paths := deviceLightingPathsForMutableRoot(t.TempDir())
	paths.ShippedDatabaseRoot = testShippedDatabaseRoot
	paths.ClusterEffectSettingsFile = filepath.Join(t.TempDir(), "malformed-cluster.json")
	writeMalformedClusterStore(t, paths.ClusterEffectSettingsFile)

	runtime, err := loadDeviceLightingRuntime(paths)
	if err != nil {
		t.Fatalf("load device lighting runtime with malformed cluster store: %v", err)
	}
	if runtime == nil || runtime.effects == nil || runtime.resolver == nil {
		t.Fatalf("device lighting runtime = %#v", runtime)
	}
	removeTestDeviceLightingRuntime(t, runtime)
}

func TestDeviceLightingRuntimeCacheIgnoresClusterStorePath(t *testing.T) {
	paths := deviceLightingPathsForMutableRoot(t.TempDir())
	paths.ShippedDatabaseRoot = testShippedDatabaseRoot
	paths.ClusterEffectSettingsFile = filepath.Join(t.TempDir(), "first-malformed-cluster.json")
	writeMalformedClusterStore(t, paths.ClusterEffectSettingsFile)

	first, err := loadDeviceLightingRuntime(paths)
	if err != nil {
		t.Fatalf("load first device lighting runtime: %v", err)
	}
	removeTestDeviceLightingRuntime(t, first)

	otherPaths := paths
	otherPaths.ClusterEffectSettingsFile = filepath.Join(t.TempDir(), "second-malformed-cluster.json")
	writeMalformedClusterStore(t, otherPaths.ClusterEffectSettingsFile)
	second, err := loadDeviceLightingRuntime(otherPaths)
	if err != nil {
		t.Fatalf("load second device lighting runtime: %v", err)
	}
	removeTestDeviceLightingRuntime(t, second)
	if second != first {
		t.Fatalf("cluster store path changed device runtime cache ownership: first=%p second=%p", first, second)
	}
}
