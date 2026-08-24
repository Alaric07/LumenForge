package lightingsettings

import (
	"fmt"
	"path/filepath"
	"sync"
)

// IndependentDeviceRuntime is the single owner of target state, effect
// customizations, shipped defaults, and canonical resolution for independent
// devices sharing one resolved persistence path set.
type IndependentDeviceRuntime struct {
	State    IndependentDeviceStateAccess
	Effects  *DeviceStore
	Defaults *DefaultRepository
	Resolver *Resolver
}

var independentDeviceRuntimeCache = struct {
	sync.Mutex
	values map[string]*IndependentDeviceRuntime
}{values: make(map[string]*IndependentDeviceRuntime)}

// LoadIndependentDeviceRuntime returns the shared runtime for one resolved
// target-state, effect-settings, and shipped-defaults path set.
func LoadIndependentDeviceRuntime(statePath, effectSettingsPath, shippedDefaultsPath string) (*IndependentDeviceRuntime, error) {
	statePath, err := normalizeIndependentDeviceRuntimePath(statePath)
	if err != nil {
		return nil, fmt.Errorf("resolve independent-device state path: %w", err)
	}
	effectSettingsPath, err = normalizeIndependentDeviceRuntimePath(effectSettingsPath)
	if err != nil {
		return nil, fmt.Errorf("resolve independent-device effect-settings path: %w", err)
	}
	shippedDefaultsPath, err = normalizeIndependentDeviceRuntimePath(shippedDefaultsPath)
	if err != nil {
		return nil, fmt.Errorf("resolve independent-device shipped-defaults path: %w", err)
	}

	key := statePath + "\x00" + effectSettingsPath + "\x00" + shippedDefaultsPath
	independentDeviceRuntimeCache.Lock()
	defer independentDeviceRuntimeCache.Unlock()
	if runtime := independentDeviceRuntimeCache.values[key]; runtime != nil {
		return runtime, nil
	}

	defaults, err := LoadDefaultRepository(shippedDefaultsPath)
	if err != nil {
		return nil, fmt.Errorf("load independent-device shipped lighting defaults: %w", err)
	}
	state, err := LoadIndependentDeviceStateStore(statePath)
	if err != nil {
		return nil, err
	}
	effects, err := LoadDeviceStore(effectSettingsPath)
	if err != nil {
		return nil, err
	}
	resolver, err := NewDeviceResolver(defaults, effects)
	if err != nil {
		return nil, err
	}
	runtime := &IndependentDeviceRuntime{
		State:    state,
		Effects:  effects,
		Defaults: defaults,
		Resolver: resolver,
	}
	independentDeviceRuntimeCache.values[key] = runtime
	return runtime, nil
}

func normalizeIndependentDeviceRuntimePath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}
