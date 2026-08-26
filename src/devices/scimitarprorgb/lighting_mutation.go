package scimitarprorgb

import (
	"fmt"

	"LumenForge/src/lightingsettings"
)

// LightingDeviceID identifies this device in the independent-device lighting
// stores.
func (d *Device) LightingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) SupportsLightingEffect(effect string) bool {
	_, ok := scimitarCanonicalEffectDescriptor(effect)
	return ok
}

func (d *Device) SetLightingEffect(effect string) error {
	if d == nil {
		return fmt.Errorf("Scimitar Pro lighting ownership is unavailable")
	}
	if !d.SupportsLightingEffect(effect) {
		return fmt.Errorf("unsupported Scimitar Pro effect %q", effect)
	}
	if d.DeviceProfile == nil {
		return fmt.Errorf("Scimitar Pro lighting ownership is unavailable")
	}
	if d.DeviceProfile.RGBCluster {
		return fmt.Errorf("Scimitar Pro lighting is owned by RGB Cluster")
	}
	if d.DeviceProfile.OpenRGBIntegration {
		return fmt.Errorf("Scimitar Pro lighting is owned by OpenRGB")
	}

	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	if err := d.setCanonicalSelectedEffect(effect); err != nil {
		return fmt.Errorf("persist Scimitar Pro selected effect: %w", err)
	}
	d.restartCanonicalLighting()
	return nil
}

func (d *Device) SetLightingBrightness(brightness uint8) error {
	if d == nil || d.DeviceProfile == nil {
		return fmt.Errorf("Scimitar Pro lighting ownership is unavailable")
	}

	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	if err := d.setCanonicalBrightness(brightness); err != nil {
		return fmt.Errorf("persist Scimitar Pro brightness: %w", err)
	}
	if d.DeviceProfile.RGBCluster || d.DeviceProfile.OpenRGBIntegration {
		return nil
	}
	d.restartCanonicalLighting()
	return nil
}

func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if !d.SupportsLightingEffect(effect) {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported Scimitar Pro effect %q", effect)
	}
	if d == nil || d.lightingSource == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("Scimitar Pro canonical lighting source is unavailable")
	}
	return d.lightingSource.resolveEffectSettings(effect)
}

func (d *Device) SetLightingEffectSettings(effect string, settings lightingsettings.EffectSettings) error {
	if d == nil {
		return fmt.Errorf("Scimitar Pro lighting ownership is unavailable")
	}
	if !d.SupportsLightingEffect(effect) {
		return fmt.Errorf("unsupported Scimitar Pro effect %q", effect)
	}
	if settings.EffectID != effect {
		return fmt.Errorf("Scimitar Pro effect settings do not match %q", effect)
	}
	if err := lightingsettings.Validate(settings); err != nil {
		return fmt.Errorf("validate Scimitar Pro effect settings: %w", err)
	}
	if d == nil || d.DeviceProfile == nil || d.lightingSource == nil {
		return fmt.Errorf("Scimitar Pro lighting ownership is unavailable")
	}

	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	selected, err := d.lightingSource.selectedEffect()
	if err != nil {
		return fmt.Errorf("resolve Scimitar Pro selected effect: %w", err)
	}
	if err := d.lightingSource.setEffectSettings(effect, settings.Clone()); err != nil {
		return fmt.Errorf("persist Scimitar Pro effect settings: %w", err)
	}
	if selected == effect && !d.DeviceProfile.RGBCluster && !d.DeviceProfile.OpenRGBIntegration {
		d.restartCanonicalLighting()
	}
	return nil
}

func (d *Device) ResetLightingEffectSettings(effect string) error {
	if d == nil {
		return fmt.Errorf("Scimitar Pro lighting ownership is unavailable")
	}
	if !d.SupportsLightingEffect(effect) {
		return fmt.Errorf("unsupported Scimitar Pro effect %q", effect)
	}
	if d == nil || d.DeviceProfile == nil || d.lightingSource == nil {
		return fmt.Errorf("Scimitar Pro lighting ownership is unavailable")
	}

	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	selected, err := d.lightingSource.selectedEffect()
	if err != nil {
		return fmt.Errorf("resolve Scimitar Pro selected effect: %w", err)
	}
	deleted, err := d.lightingSource.deleteEffectSettings(effect)
	if err != nil {
		return fmt.Errorf("reset Scimitar Pro effect settings: %w", err)
	}
	if deleted && selected == effect && !d.DeviceProfile.RGBCluster && !d.DeviceProfile.OpenRGBIntegration {
		d.restartCanonicalLighting()
	}
	return nil
}
