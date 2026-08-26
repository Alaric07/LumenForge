package mm800

import (
	"fmt"
	"path/filepath"
	"sync"

	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
)

type mm800SchedulerBrightnessOverride struct {
	mu    sync.RWMutex
	value *uint8
}

func (override *mm800SchedulerBrightnessOverride) set(value *uint8) bool {
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

func (override *mm800SchedulerBrightnessOverride) effective(desired uint8) uint8 {
	override.mu.RLock()
	defer override.mu.RUnlock()
	if override.value == nil {
		return desired
	}
	return *override.value
}

func (d *Device) attachIndependentDeviceLightingRuntime(paths config.Paths) error {
	if d == nil {
		return fmt.Errorf("MM800 device is unavailable")
	}
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(
		paths.OpenRGBDeviceLightingFile,
		paths.DeviceEffectSettingsFile,
		filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"),
	)
	if err != nil {
		return err
	}
	if runtime == nil || runtime.State == nil || runtime.Effects == nil || runtime.Resolver == nil {
		return fmt.Errorf("MM800 canonical lighting runtime is unavailable")
	}
	d.lightingRuntime = runtime
	return nil
}

func (d *Device) canonicalLightingState() (lightingsettings.IndependentDeviceLightingState, error) {
	if d == nil || d.lightingRuntime == nil || d.lightingRuntime.State == nil {
		return lightingsettings.IndependentDeviceLightingState{}, fmt.Errorf("MM800 canonical lighting source is unavailable")
	}
	state, _, err := d.lightingRuntime.State.Resolve(d.Serial)
	return state, err
}

func (d *Device) currentCanonicalSelectedEffect() (string, error) {
	state, err := d.canonicalLightingState()
	return state.SelectedEffect, err
}

func (d *Device) effectiveCanonicalBrightness() (uint8, error) {
	state, err := d.canonicalLightingState()
	if err != nil {
		return 0, err
	}
	return d.schedulerBrightnessOverride.effective(state.Brightness), nil
}

func (d *Device) setCanonicalSelectedEffect(effect string) error {
	state, err := d.canonicalLightingState()
	if err != nil {
		return err
	}
	state.SelectedEffect = effect
	return d.lightingRuntime.State.Set(d.Serial, state)
}

func (d *Device) setCanonicalBrightness(brightness uint8) error {
	state, err := d.canonicalLightingState()
	if err != nil {
		return err
	}
	state.Brightness = brightness
	return d.lightingRuntime.State.Set(d.Serial, state)
}

func (d *Device) resolveCanonicalEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if effect == "mousepad" {
		return lightingsettings.EffectSettings{}, fmt.Errorf("MM800 mousepad settings are device-specific")
	}
	if d == nil || d.lightingRuntime == nil || d.lightingRuntime.Resolver == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("MM800 canonical lighting source is unavailable")
	}
	resolution, err := d.lightingRuntime.Resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), effect)
	if err != nil {
		return lightingsettings.EffectSettings{}, err
	}
	return resolution.Settings.Clone(), nil
}

func (d *Device) restartCanonicalLighting() {
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.setDeviceColor()
}

func (d *Device) locallyOwnsLighting() bool {
	return d != nil && d.DeviceProfile != nil && !d.DeviceProfile.RGBCluster && !d.DeviceProfile.OpenRGBIntegration
}
