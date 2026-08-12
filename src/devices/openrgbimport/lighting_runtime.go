package openrgbimport

import (
	"fmt"
	"path/filepath"
	"sync"

	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

type deviceLightingRuntime struct {
	state    deviceLightingStateAccess
	effects  *lightingsettings.DeviceStore
	resolver *lightingsettings.Resolver
}

type deviceLightingStateAccess interface {
	Resolve(string) (DeviceLightingState, bool, error)
	Set(string, DeviceLightingState) error
	Delete(string) (bool, error)
}

type deviceLightingEffectAccess interface {
	Set(string, string, lightingsettings.EffectSettings) error
	Delete(string, string) (bool, error)
}

type deviceLightingResolverAccess interface {
	Resolve(lightingsettings.Target, string) (lightingsettings.Resolution, error)
}

var deviceLightingRuntimeCache = struct {
	sync.Mutex
	values map[string]*deviceLightingRuntime
}{values: make(map[string]*deviceLightingRuntime)}

func loadDeviceLightingRuntime(paths config.Paths) (*deviceLightingRuntime, error) {
	key := paths.OpenRGBDeviceLightingFile + "\x00" + paths.DeviceEffectSettingsFile + "\x00" + paths.ShippedDatabaseRoot
	deviceLightingRuntimeCache.Lock()
	defer deviceLightingRuntimeCache.Unlock()
	if runtime := deviceLightingRuntimeCache.values[key]; runtime != nil {
		return runtime, nil
	}

	defaults, err := lightingsettings.LoadDefaultRepository(filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		return nil, fmt.Errorf("load OpenRGB shipped lighting defaults: %w", err)
	}
	state, err := LoadDeviceLightingStateStore(paths.OpenRGBDeviceLightingFile)
	if err != nil {
		return nil, err
	}
	effects, err := lightingsettings.LoadDeviceStore(paths.DeviceEffectSettingsFile)
	if err != nil {
		return nil, err
	}
	resolver, err := lightingsettings.NewDeviceResolver(defaults, effects)
	if err != nil {
		return nil, err
	}
	runtime := &deviceLightingRuntime{state: state, effects: effects, resolver: resolver}
	deviceLightingRuntimeCache.values[key] = runtime
	return runtime, nil
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
	if d == nil || runtime == nil || runtime.state == nil || runtime.effects == nil || runtime.resolver == nil {
		return fmt.Errorf("OpenRGB device lighting runtime is unavailable")
	}
	state, _, err := runtime.state.Resolve(d.Serial)
	if err != nil {
		return err
	}
	d.lightingState = runtime.state
	d.lightingEffects = runtime.effects
	d.lightingResolver = runtime.resolver
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

func rgbProfileFromLightingSettings(settings lightingsettings.EffectSettings) rgb.Profile {
	profile := rgb.Profile{ProfileName: settings.EffectID, Brightness: 1}
	if settings.Speed != nil {
		profile.Speed = *settings.Speed
	}
	if settings.SingleColor != nil {
		profile.StartColor = rgbColorFromLightingColor(settings.SingleColor.Color)
	}
	if settings.TwoColor != nil {
		profile.StartColor = rgbColorFromLightingColor(settings.TwoColor.Start)
		profile.EndColor = rgbColorFromLightingColor(settings.TwoColor.End)
	}
	if settings.Temperature != nil {
		profile.StartColor = rgbTemperatureColor(settings.Temperature.Low)
		profile.MiddleColor = rgbTemperatureColor(settings.Temperature.Middle)
		profile.EndColor = rgbTemperatureColor(settings.Temperature.High)
		profile.MinTemp = settings.Temperature.Low.Celsius
		profile.MaxTemp = settings.Temperature.High.Celsius
	}
	if settings.Gradient != nil {
		profile.Gradients = make(map[int]rgb.Color, len(settings.Gradient.Stops))
		for index, stop := range settings.Gradient.Stops {
			color := rgbColorFromLightingColor(stop.Color)
			color.Position = stop.Position
			color.Brightness = stop.Intensity
			profile.Gradients[index] = color
		}
	}
	return profile
}

func rgbColorFromLightingColor(color lightingsettings.Color) rgb.Color {
	return rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue, Brightness: 1}
}

func rgbTemperatureColor(point lightingsettings.TemperaturePoint) rgb.Color {
	color := rgbColorFromLightingColor(point.Color)
	color.Temperature = point.Celsius
	return color
}
