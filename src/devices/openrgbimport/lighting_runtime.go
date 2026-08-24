package openrgbimport

import (
	"fmt"
	"path/filepath"

	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
)

const defaultDeviceLightingEffect = lightingsettings.DefaultIndependentDeviceEffect

type deviceLightingRuntime = lightingsettings.IndependentDeviceRuntime

type deviceLightingStateAccess interface {
	Resolve(string) (lightingsettings.IndependentDeviceLightingState, bool, error)
	Set(string, lightingsettings.IndependentDeviceLightingState) error
	Delete(string) (bool, error)
}

type deviceLightingEffectAccess interface {
	Set(string, string, lightingsettings.EffectSettings) error
	Delete(string, string) (bool, error)
}

type deviceLightingResolverAccess interface {
	Resolve(lightingsettings.Target, string) (lightingsettings.Resolution, error)
}

func loadDeviceLightingRuntime(paths config.Paths) (*deviceLightingRuntime, error) {
	return lightingsettings.LoadIndependentDeviceRuntime(
		paths.OpenRGBDeviceLightingFile,
		paths.DeviceEffectSettingsFile,
		filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"),
	)
}

func deviceLightingPathsForMutableRoot(root string) config.Paths {
	paths := config.GetPaths()
	lightingRoot := filepath.Join(root, "database", "lighting")
	paths.MutableDataRoot = root
	paths.MutableLightingRoot = lightingRoot
	paths.OpenRGBDeviceLightingFile = filepath.Join(lightingRoot, "openrgb-device-state.json")
	paths.DeviceEffectSettingsFile = filepath.Join(lightingRoot, "independent-device-effects.json")
	paths.ClusterEffectSettingsFile = filepath.Join(lightingRoot, "rgb-cluster-effects.json")
	return paths
}

func (d *Device) attachLightingRuntime(runtime *deviceLightingRuntime) error {
	if d == nil || runtime == nil || runtime.State == nil || runtime.Effects == nil || runtime.Resolver == nil {
		return fmt.Errorf("OpenRGB device lighting runtime is unavailable")
	}
	state, _, err := runtime.State.Resolve(d.Serial)
	if err != nil {
		return err
	}
	d.lightingState = runtime.State
	d.lightingEffects = runtime.Effects
	d.lightingResolver = runtime.Resolver
	d.effect = state.SelectedEffect
	d.brightness = state.Brightness
	return nil
}

func (d *Device) resolveLightingSettings(effect string) (lightingsettings.Resolution, error) {
	if d == nil || d.lightingResolver == nil {
		return lightingsettings.Resolution{}, fmt.Errorf("OpenRGB device lighting resolver is unavailable")
	}
	return d.lightingResolver.Resolve(lightingsettings.IndependentDevice(d.Serial), effect)
}
