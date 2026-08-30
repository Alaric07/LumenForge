package k95platinum

import (
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"fmt"
	"slices"
)

func (d *Device) LightingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func k95CanonicalEffectDescriptor(effect string) (rgb.SoftwareEffectDescriptor, bool) {
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	return descriptor, ok && descriptor.Scope.Includes(rgb.EffectScopeDevice) && slices.Contains(rgbModes, effect)
}
func (d *Device) SupportsLightingEffect(effect string) bool {
	return effect == "keyboard" || func() bool { _, ok := k95CanonicalEffectDescriptor(effect); return ok }()
}
func (d *Device) SetLightingEffect(effect string) error {
	if d == nil || d.DeviceProfile == nil {
		return fmt.Errorf("K95 lighting ownership is unavailable")
	}
	if !d.SupportsLightingEffect(effect) {
		return fmt.Errorf("unsupported K95 effect %q", effect)
	}
	if d.DeviceProfile.RGBCluster {
		return fmt.Errorf("K95 lighting is owned by RGB Cluster")
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	if err := d.setCanonicalSelectedEffect(effect); err != nil {
		return fmt.Errorf("persist K95 selected effect: %w", err)
	}
	d.restartCanonicalLighting()
	return nil
}
func (d *Device) SetLightingBrightness(value uint8) error {
	if d == nil || d.DeviceProfile == nil {
		return fmt.Errorf("K95 lighting ownership is unavailable")
	}
	if d.DeviceProfile.RGBCluster {
		return fmt.Errorf("K95 lighting is owned by RGB Cluster")
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	if err := d.setCanonicalBrightness(value); err != nil {
		return fmt.Errorf("persist K95 brightness: %w", err)
	}
	d.restartCanonicalLighting()
	return nil
}
func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if d == nil || !d.SupportsLightingEffect(effect) || effect == "keyboard" {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported K95 effect %q", effect)
	}
	if d.lightingSource == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("K95 canonical lighting source is unavailable")
	}
	return d.lightingSource.resolveEffectSettings(effect)
}
func (d *Device) SetLightingEffectSettings(effect string, settings lightingsettings.EffectSettings) error {
	if d == nil || d.DeviceProfile == nil || !d.SupportsLightingEffect(effect) || effect == "keyboard" || settings.EffectID != effect {
		return fmt.Errorf("invalid K95 effect settings")
	}
	if err := lightingsettings.Validate(settings); err != nil {
		return err
	}
	if d.DeviceProfile.RGBCluster {
		return fmt.Errorf("K95 lighting is owned by RGB Cluster")
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	selected, err := d.currentCanonicalSelectedEffect()
	if err != nil {
		return err
	}
	if err = d.lightingSource.setEffectSettings(effect, settings.Clone()); err != nil {
		return err
	}
	if selected == effect {
		d.restartCanonicalLighting()
	}
	return nil
}
func (d *Device) ResetLightingEffectSettings(effect string) error {
	if d == nil || d.DeviceProfile == nil || !d.SupportsLightingEffect(effect) || effect == "keyboard" {
		return fmt.Errorf("unsupported K95 effect %q", effect)
	}
	if d.DeviceProfile.RGBCluster {
		return fmt.Errorf("K95 lighting is owned by RGB Cluster")
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	selected, err := d.currentCanonicalSelectedEffect()
	if err != nil {
		return err
	}
	deleted, err := d.lightingSource.deleteEffectSettings(effect)
	if err != nil {
		return err
	}
	if deleted && selected == effect {
		d.restartCanonicalLighting()
	}
	return nil
}
