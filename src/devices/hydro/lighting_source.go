package hydro

import (
	"fmt"
	"path/filepath"
	"sync"

	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

type hydroSchedulerBrightnessOverride struct {
	mu    sync.RWMutex
	value *uint8
}

func (override *hydroSchedulerBrightnessOverride) set(value *uint8) bool {
	override.mu.Lock()
	defer override.mu.Unlock()
	if override.value == nil && value == nil {
		return false
	}
	if override.value != nil && value != nil && *override.value == *value {
		return false
	}
	if value == nil {
		override.value = nil
		return true
	}
	copy := *value
	override.value = &copy
	return true
}

func (override *hydroSchedulerBrightnessOverride) effective(desired uint8) uint8 {
	override.mu.RLock()
	defer override.mu.RUnlock()
	if override.value == nil {
		return desired
	}
	return *override.value
}

func (d *Device) attachIndependentDeviceLightingRuntime(paths config.Paths) error {
	if d == nil || d.Serial == "" {
		return fmt.Errorf("Hydro device is unavailable")
	}
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile, filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		return err
	}
	if runtime == nil || runtime.State == nil || runtime.Effects == nil || runtime.Resolver == nil {
		return fmt.Errorf("Hydro canonical lighting runtime is unavailable")
	}
	state, found, err := runtime.State.Resolve(d.Serial)
	if err != nil {
		return err
	}
	if !found {
		state = lightingsettings.DefaultIndependentDeviceLightingState()
		if err = runtime.State.Set(d.Serial, state); err != nil {
			return err
		}
	}
	if state.SelectedEffect != "static" {
		return fmt.Errorf("Hydro selected effect %q is not static", state.SelectedEffect)
	}
	d.lightingRuntime = runtime
	return nil
}

func (d *Device) canonicalLightingState() (lightingsettings.IndependentDeviceLightingState, error) {
	if d == nil || d.lightingRuntime == nil || d.lightingRuntime.State == nil {
		return lightingsettings.IndependentDeviceLightingState{}, fmt.Errorf("Hydro canonical lighting source is unavailable")
	}
	state, _, err := d.lightingRuntime.State.Resolve(d.Serial)
	if err != nil || state.SelectedEffect != "static" {
		if err == nil {
			err = fmt.Errorf("Hydro selected effect %q is not static", state.SelectedEffect)
		}
		return lightingsettings.IndependentDeviceLightingState{}, err
	}
	return state, nil
}

func (d *Device) effectiveCanonicalBrightness() (uint8, error) {
	state, err := d.canonicalLightingState()
	if err != nil {
		return 0, err
	}
	return d.schedulerBrightnessOverride.effective(state.Brightness), nil
}

func (d *Device) resolveCanonicalStatic() (lightingsettings.Resolution, error) {
	if d == nil || d.lightingRuntime == nil || d.lightingRuntime.Resolver == nil {
		return lightingsettings.Resolution{}, fmt.Errorf("Hydro canonical lighting source is unavailable")
	}
	return d.lightingRuntime.Resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), "static")
}

func (d *Device) canonicalConfigurationColor() ([3]byte, bool) {
	settings, err := d.resolveCanonicalStatic()
	if err != nil || settings.Settings.EffectID != "static" || settings.Settings.SingleColor == nil {
		return [3]byte{}, false
	}
	brightness, err := d.effectiveCanonicalBrightness()
	if err != nil {
		return [3]byte{}, false
	}
	color := rgb.Color{Red: settings.Settings.SingleColor.Color.Red, Green: settings.Settings.SingleColor.Color.Green, Blue: settings.Settings.SingleColor.Color.Blue, Brightness: rgb.GetBrightnessValueFloat(brightness)}
	modified := rgb.ModifyBrightness(color)
	return [3]byte{byte(modified.Red), byte(modified.Green), byte(modified.Blue)}, true
}
