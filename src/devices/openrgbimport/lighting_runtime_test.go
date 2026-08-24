package openrgbimport

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"LumenForge/src/lightingsettings"
)

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
	if runtime == nil || runtime.State == nil || runtime.Effects == nil || runtime.Defaults == nil || runtime.Resolver == nil {
		t.Fatalf("device lighting runtime = %#v", runtime)
	}
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
	otherPaths := paths
	otherPaths.ClusterEffectSettingsFile = filepath.Join(t.TempDir(), "second-malformed-cluster.json")
	writeMalformedClusterStore(t, otherPaths.ClusterEffectSettingsFile)
	second, err := loadDeviceLightingRuntime(otherPaths)
	if err != nil {
		t.Fatalf("load second device lighting runtime: %v", err)
	}
	if second != first {
		t.Fatalf("cluster store path changed device runtime cache ownership: first=%p second=%p", first, second)
	}
}

func TestOpenRGBDeviceUsesExtractedIndependentDeviceRuntime(t *testing.T) {
	paths := deviceLightingPathsForMutableRoot(t.TempDir())
	paths.ShippedDatabaseRoot = testShippedDatabaseRoot
	runtime, err := loadDeviceLightingRuntime(paths)
	if err != nil {
		t.Fatal(err)
	}

	serial := "openrgb-common-runtime"
	wantState := lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 43}
	if err = runtime.State.Set(serial, wantState); err != nil {
		t.Fatal(err)
	}
	custom := lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      "static",
		SingleColor: &lightingsettings.SingleColorSettings{
			Color: lightingsettings.Color{Red: 21, Green: 34, Blue: 55},
		},
	}
	if err = runtime.Effects.Set(serial, "static", custom); err != nil {
		t.Fatal(err)
	}

	device := &Device{Serial: serial}
	if err = device.attachLightingRuntime(runtime); err != nil {
		t.Fatal(err)
	}
	if device.effect != wantState.SelectedEffect || device.brightness != wantState.Brightness {
		t.Fatalf("attached OpenRGB state = effect %q brightness %d, want %#v", device.effect, device.brightness, wantState)
	}
	resolution, err := device.resolveLightingSettings("static")
	if err != nil || !resolution.Customized || resolution.Settings.SingleColor == nil ||
		resolution.Settings.SingleColor.Color != custom.SingleColor.Color {
		t.Fatalf("attached OpenRGB resolution = %#v, %v", resolution, err)
	}
	profile := device.resolvedRendererProfile("static")
	wantProfile := lightingsettings.RendererProfileFromEffectSettings(custom)
	if profile == nil || !reflect.DeepEqual(*profile, wantProfile) {
		t.Fatalf("attached OpenRGB renderer profile = %#v, want %#v", profile, wantProfile)
	}
}
