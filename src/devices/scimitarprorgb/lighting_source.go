package scimitarprorgb

import (
	"fmt"
	"path/filepath"
	"sync"

	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
)

type scimitarResolvedLighting struct {
	selectedEffect string
	brightness     uint8
	settings       lightingsettings.EffectSettings
}

type scimitarSchedulerBrightnessOverride struct {
	mu    sync.RWMutex
	value *uint8
}

func (override *scimitarSchedulerBrightnessOverride) set(value *uint8) bool {
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

func (override *scimitarSchedulerBrightnessOverride) effective(desired uint8) uint8 {
	override.mu.RLock()
	defer override.mu.RUnlock()
	if override.value == nil {
		return desired
	}
	return *override.value
}

type scimitarLightingSource interface {
	resolve() (scimitarResolvedLighting, error)
	resolveEffectSettings(string) (lightingsettings.EffectSettings, error)
	resolveEffectSettingsWithStatus(string) (lightingsettings.Resolution, error)
	selectedEffect() (string, error)
	setSelectedEffect(string) error
	setEffectSettings(string, lightingsettings.EffectSettings) error
	brightness() (uint8, error)
	setBrightness(uint8) error
}

type independentDeviceLightingStateAccess interface {
	Resolve(string) (lightingsettings.IndependentDeviceLightingState, bool, error)
	Set(string, lightingsettings.IndependentDeviceLightingState) error
}

type independentDeviceLightingResolver interface {
	Resolve(lightingsettings.Target, string) (lightingsettings.Resolution, error)
}

type independentDeviceEffectSettingsAccess interface {
	Set(string, string, lightingsettings.EffectSettings) error
}

type independentDeviceLightingSource struct {
	deviceID string
	state    independentDeviceLightingStateAccess
	effects  independentDeviceEffectSettingsAccess
	resolver independentDeviceLightingResolver
}

func (source independentDeviceLightingSource) selectedEffect() (string, error) {
	if source.state == nil {
		return "", fmt.Errorf("independent-device lighting runtime is unavailable")
	}
	state, _, err := source.state.Resolve(source.deviceID)
	if err != nil {
		return "", err
	}
	return state.SelectedEffect, nil
}

func (source independentDeviceLightingSource) brightness() (uint8, error) {
	if source.state == nil {
		return 0, fmt.Errorf("independent-device lighting runtime is unavailable")
	}
	state, _, err := source.state.Resolve(source.deviceID)
	if err != nil {
		return 0, err
	}
	return state.Brightness, nil
}

func (source independentDeviceLightingSource) setSelectedEffect(effect string) error {
	if source.state == nil || source.resolver == nil {
		return fmt.Errorf("independent-device lighting runtime is unavailable")
	}
	state, _, err := source.state.Resolve(source.deviceID)
	if err != nil {
		return err
	}
	if _, err = source.resolver.Resolve(lightingsettings.IndependentDevice(source.deviceID), effect); err != nil {
		return err
	}
	state.SelectedEffect = effect
	return source.state.Set(source.deviceID, state)
}

func (source independentDeviceLightingSource) setBrightness(brightness uint8) error {
	if source.state == nil {
		return fmt.Errorf("independent-device lighting runtime is unavailable")
	}
	state, _, err := source.state.Resolve(source.deviceID)
	if err != nil {
		return err
	}
	state.Brightness = brightness
	return source.state.Set(source.deviceID, state)
}

func (source independentDeviceLightingSource) resolveEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	resolution, err := source.resolveEffectSettingsWithStatus(effect)
	if err != nil {
		return lightingsettings.EffectSettings{}, err
	}
	return resolution.Settings.Clone(), nil
}

func (source independentDeviceLightingSource) resolveEffectSettingsWithStatus(effect string) (lightingsettings.Resolution, error) {
	if source.resolver == nil {
		return lightingsettings.Resolution{}, fmt.Errorf("independent-device lighting runtime is unavailable")
	}
	resolution, err := source.resolver.Resolve(lightingsettings.IndependentDevice(source.deviceID), effect)
	if err != nil {
		return lightingsettings.Resolution{}, err
	}
	resolution.Settings = resolution.Settings.Clone()
	return resolution, nil
}

func (source independentDeviceLightingSource) setEffectSettings(effect string, settings lightingsettings.EffectSettings) error {
	if source.effects == nil {
		return fmt.Errorf("independent-device effect customization store is unavailable")
	}
	return source.effects.Set(source.deviceID, effect, settings)
}

func (source independentDeviceLightingSource) resolve() (scimitarResolvedLighting, error) {
	if source.state == nil || source.resolver == nil {
		return scimitarResolvedLighting{}, fmt.Errorf("independent-device lighting runtime is unavailable")
	}
	state, _, err := source.state.Resolve(source.deviceID)
	if err != nil {
		return scimitarResolvedLighting{}, err
	}
	resolution, err := source.resolver.Resolve(
		lightingsettings.IndependentDevice(source.deviceID),
		state.SelectedEffect,
	)
	if err != nil {
		return scimitarResolvedLighting{}, err
	}
	return scimitarResolvedLighting{
		selectedEffect: state.SelectedEffect,
		brightness:     state.Brightness,
		settings:       resolution.Settings.Clone(),
	}, nil
}

func (d *Device) attachIndependentDeviceLightingRuntime(paths config.Paths) error {
	if d == nil {
		return fmt.Errorf("Scimitar Pro device is unavailable")
	}
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(
		paths.OpenRGBDeviceLightingFile,
		paths.DeviceEffectSettingsFile,
		filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"),
	)
	if err != nil {
		return err
	}
	return d.attachIndependentDeviceLightingSource(runtime)
}

func (d *Device) attachIndependentDeviceLightingSource(runtime *lightingsettings.IndependentDeviceRuntime) error {
	if d == nil || runtime == nil || runtime.State == nil || runtime.Effects == nil || runtime.Resolver == nil {
		return fmt.Errorf("Scimitar Pro canonical lighting runtime is unavailable")
	}
	source := independentDeviceLightingSource{
		deviceID: d.Serial,
		state:    runtime.State,
		effects:  runtime.Effects,
		resolver: runtime.Resolver,
	}
	if _, err := source.resolve(); err != nil {
		return err
	}
	d.lightingSource = source
	return nil
}

func (d *Device) resolveCanonicalLighting() (scimitarResolvedLighting, error) {
	if d == nil || d.lightingSource == nil {
		return scimitarResolvedLighting{}, fmt.Errorf("Scimitar Pro canonical lighting source is unavailable")
	}
	return d.lightingSource.resolve()
}

func (d *Device) currentCanonicalSelectedEffect() (string, error) {
	if d == nil || d.lightingSource == nil {
		return "", fmt.Errorf("Scimitar Pro canonical lighting source is unavailable")
	}
	return d.lightingSource.selectedEffect()
}

func (d *Device) currentCanonicalBrightness() (uint8, error) {
	if d == nil || d.lightingSource == nil {
		return 0, fmt.Errorf("Scimitar Pro canonical lighting source is unavailable")
	}
	return d.lightingSource.brightness()
}

func (d *Device) effectiveBrightness() (uint8, error) {
	brightness, err := d.currentCanonicalBrightness()
	if err != nil {
		return 0, err
	}
	return d.schedulerBrightnessOverride.effective(brightness), nil
}

func (d *Device) resolveEffectiveCanonicalLighting() (scimitarResolvedLighting, error) {
	resolved, err := d.resolveCanonicalLighting()
	if err != nil {
		return scimitarResolvedLighting{}, err
	}
	resolved.brightness = d.schedulerBrightnessOverride.effective(resolved.brightness)
	return resolved, nil
}

func (d *Device) setCanonicalSelectedEffect(effect string) error {
	if d == nil || d.lightingSource == nil {
		return fmt.Errorf("Scimitar Pro canonical lighting source is unavailable")
	}
	return d.lightingSource.setSelectedEffect(effect)
}

func (d *Device) setCanonicalBrightness(brightness uint8) error {
	if d == nil || d.lightingSource == nil {
		return fmt.Errorf("Scimitar Pro canonical lighting source is unavailable")
	}
	return d.lightingSource.setBrightness(brightness)
}

func (d *Device) restartCanonicalLighting() {
	if d.lightingRestart != nil {
		d.lightingRestart()
		return
	}
	d.stopLighting()
	d.setDeviceColor()
}
