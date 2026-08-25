package scimitarprorgb

import (
	"fmt"
	"path/filepath"

	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
)

type scimitarResolvedLighting struct {
	selectedEffect string
	brightness     uint8
	settings       lightingsettings.EffectSettings
}

type scimitarLightingSource interface {
	resolve() (scimitarResolvedLighting, error)
	selectedEffect() (string, error)
	setSelectedEffect(string) error
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

type independentDeviceLightingSource struct {
	deviceID string
	state    independentDeviceLightingStateAccess
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
	if d == nil || runtime == nil || runtime.State == nil || runtime.Resolver == nil {
		return fmt.Errorf("Scimitar Pro canonical lighting runtime is unavailable")
	}
	source := independentDeviceLightingSource{
		deviceID: d.Serial,
		state:    runtime.State,
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
