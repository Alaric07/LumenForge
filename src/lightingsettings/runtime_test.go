package lightingsettings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIndependentDeviceRuntimeOwnsSharedStateSettingsDefaultsAndResolver(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(root, "openrgb-device-state.json")
	effectPath := filepath.Join(root, "independent-device-effects.json")
	defaultsPath := shippedDefaultsPath(t)

	first, err := LoadIndependentDeviceRuntime(statePath, effectPath, defaultsPath)
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || first.State == nil || first.Effects == nil || first.Defaults == nil || first.Resolver == nil {
		t.Fatalf("independent-device runtime = %#v", first)
	}

	state, found, err := first.State.Resolve("openrgb-runtime-device")
	if err != nil || found || state != DefaultIndependentDeviceLightingState() {
		t.Fatalf("missing runtime state = %#v, %t, %v", state, found, err)
	}

	wantState := IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 42}
	if err = first.State.Set("openrgb-runtime-device", wantState); err != nil {
		t.Fatal(err)
	}
	wantSettings := testStaticSettings(73)
	if err = first.Effects.Set("openrgb-runtime-device", "static", wantSettings); err != nil {
		t.Fatal(err)
	}

	second, err := LoadIndependentDeviceRuntime(statePath, effectPath, defaultsPath)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("same resolved paths returned separate runtimes: first=%p second=%p", first, second)
	}
	if second.State != first.State || second.Effects != first.Effects || second.Defaults != first.Defaults || second.Resolver != first.Resolver {
		t.Fatal("shared runtime returned different owned dependencies")
	}

	gotState, found, err := second.State.Resolve("openrgb-runtime-device")
	if err != nil || !found || gotState != wantState {
		t.Fatalf("shared runtime state = %#v, %t, %v; want %#v", gotState, found, err, wantState)
	}
	resolution, err := second.Resolver.Resolve(IndependentDevice("openrgb-runtime-device"), "static")
	if err != nil || !resolution.Customized || resolution.Settings.SingleColor == nil || resolution.Settings.SingleColor.Color.Red != 73 {
		t.Fatalf("shared runtime resolution = %#v, %v", resolution, err)
	}

	reloadedState, err := LoadIndependentDeviceStateStore(statePath)
	if err != nil {
		t.Fatal(err)
	}
	persisted, found, err := reloadedState.Resolve("openrgb-runtime-device")
	if err != nil || !found || persisted != wantState {
		t.Fatalf("reloaded runtime state = %#v, %t, %v; want %#v", persisted, found, err, wantState)
	}
	reloadedEffects, err := LoadDeviceStore(effectPath)
	if err != nil {
		t.Fatal(err)
	}
	persistedSettings, found, err := reloadedEffects.Get("openrgb-runtime-device", "static")
	if err != nil || !found || persistedSettings.SingleColor == nil || persistedSettings.SingleColor.Color.Red != 73 {
		t.Fatalf("reloaded runtime settings = %#v, %t, %v", persistedSettings, found, err)
	}
}

func TestIndependentDeviceRuntimeNormalizesCachePaths(t *testing.T) {
	root := t.TempDir()
	statePath := filepath.Join(root, "independent-device-state.json")
	effectPath := filepath.Join(root, "independent-device-effects.json")
	defaultsPath := shippedDefaultsPath(t)
	absoluteDefaultsPath, err := filepath.Abs(defaultsPath)
	if err != nil {
		t.Fatal(err)
	}

	first, err := LoadIndependentDeviceRuntime(statePath, effectPath, absoluteDefaultsPath)
	if err != nil {
		t.Fatal(err)
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relativeStatePath, err := filepath.Rel(workingDirectory, statePath)
	if err != nil {
		t.Fatal(err)
	}
	relativeEffectPath, err := filepath.Rel(workingDirectory, effectPath)
	if err != nil {
		t.Fatal(err)
	}
	relativeDefaultsPath, err := filepath.Rel(workingDirectory, absoluteDefaultsPath)
	if err != nil {
		t.Fatal(err)
	}

	second, err := LoadIndependentDeviceRuntime(
		"."+string(filepath.Separator)+relativeStatePath,
		"."+string(filepath.Separator)+relativeEffectPath,
		"."+string(filepath.Separator)+relativeDefaultsPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("equivalent paths returned separate runtimes: first=%p second=%p", first, second)
	}
	if second.State != first.State || second.Effects != first.Effects || second.Defaults != first.Defaults || second.Resolver != first.Resolver {
		t.Fatal("equivalent paths returned different owned dependencies")
	}

	distinct, err := LoadIndependentDeviceRuntime(
		filepath.Join(root, "other-independent-device-state.json"),
		effectPath,
		absoluteDefaultsPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	if distinct == first {
		t.Fatalf("distinct path sets returned the same runtime: first=%p distinct=%p", first, distinct)
	}
}
