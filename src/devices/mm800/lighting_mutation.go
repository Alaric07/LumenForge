package mm800

import (
	"fmt"
	"slices"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

func (d *Device) LightingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) SupportsLightingEffect(effect string) bool {
	if !slices.Contains(rgbModes, effect) {
		return false
	}
	if effect == "mousepad" {
		return true
	}
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	return ok && descriptor.Scope.Includes(rgb.EffectScopeDevice)
}

func (d *Device) SetLightingEffect(effect string) error {
	if d == nil || d.DeviceProfile == nil {
		return fmt.Errorf("MM800 lighting ownership is unavailable")
	}
	if !d.SupportsLightingEffect(effect) {
		return fmt.Errorf("unsupported MM800 effect %q", effect)
	}
	if d.DeviceProfile.RGBCluster {
		return fmt.Errorf("MM800 lighting is owned by RGB Cluster")
	}
	if d.DeviceProfile.OpenRGBIntegration {
		return fmt.Errorf("MM800 lighting is owned by OpenRGB")
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	if err := d.setCanonicalSelectedEffect(effect); err != nil {
		return fmt.Errorf("persist MM800 selected effect: %w", err)
	}
	d.restartCanonicalLighting()
	return nil
}

func (d *Device) SetLightingBrightness(brightness uint8) error {
	if d == nil || d.DeviceProfile == nil {
		return fmt.Errorf("MM800 lighting ownership is unavailable")
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	if err := d.setCanonicalBrightness(brightness); err != nil {
		return fmt.Errorf("persist MM800 brightness: %w", err)
	}
	if d.locallyOwnsLighting() {
		d.restartCanonicalLighting()
	}
	return nil
}

func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if !d.SupportsLightingEffect(effect) {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported MM800 effect %q", effect)
	}
	return d.resolveCanonicalEffectSettings(effect)
}

func (d *Device) SetLightingEffectSettings(effect string, settings lightingsettings.EffectSettings) error {
	if d == nil || d.DeviceProfile == nil || d.lightingRuntime == nil {
		return fmt.Errorf("MM800 lighting ownership is unavailable")
	}
	if !d.SupportsLightingEffect(effect) || effect == "mousepad" {
		return fmt.Errorf("unsupported MM800 effect customization %q", effect)
	}
	if settings.EffectID != effect {
		return fmt.Errorf("MM800 effect settings do not match %q", effect)
	}
	if err := lightingsettings.Validate(settings); err != nil {
		return fmt.Errorf("validate MM800 effect settings: %w", err)
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	state, err := d.canonicalLightingState()
	if err != nil {
		return fmt.Errorf("resolve MM800 selected effect: %w", err)
	}
	if err = d.lightingRuntime.Effects.Set(d.Serial, effect, settings.Clone()); err != nil {
		return fmt.Errorf("persist MM800 effect settings: %w", err)
	}
	if state.SelectedEffect == effect && d.locallyOwnsLighting() {
		d.restartCanonicalLighting()
	}
	return nil
}

func (d *Device) ResetLightingEffectSettings(effect string) error {
	if d == nil || d.DeviceProfile == nil || d.lightingRuntime == nil {
		return fmt.Errorf("MM800 lighting ownership is unavailable")
	}
	if !d.SupportsLightingEffect(effect) || effect == "mousepad" {
		return fmt.Errorf("unsupported MM800 effect customization %q", effect)
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	state, err := d.canonicalLightingState()
	if err != nil {
		return fmt.Errorf("resolve MM800 selected effect: %w", err)
	}
	deleted, err := d.lightingRuntime.Effects.Delete(d.Serial, effect)
	if err != nil {
		return fmt.Errorf("reset MM800 effect settings: %w", err)
	}
	if deleted && state.SelectedEffect == effect && d.locallyOwnsLighting() {
		d.restartCanonicalLighting()
	}
	return nil
}
