package scimitarrgbelite

import (
	"fmt"
	"strconv"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

// LightingDeviceID identifies this device in the independent-device lighting
// stores.
func (d *Device) LightingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// SetLightingZoneColor updates device-owned authored colors for Mouse without
// treating them as shared EffectSettings.
func (d *Device) SetLightingZoneColor(effect, scope, zoneID, groupID string, color rgb.Color) error {
	if d == nil || d.DeviceProfile == nil || effect != "mouse" || !d.SupportsLightingEffect(effect) {
		return fmt.Errorf("Scimitar RGB Elite authored lighting is unavailable")
	}
	if color.Red < 0 || color.Red > 255 || color.Green < 0 || color.Green > 255 || color.Blue < 0 || color.Blue > 255 {
		return fmt.Errorf("invalid authored zone color")
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	apply := func(zone ZoneColors) ZoneColors {
		if zone.Color == nil {
			zone.Color = &rgb.Color{}
		}
		zone.Color.Red, zone.Color.Green, zone.Color.Blue = color.Red, color.Green, color.Blue
		zone.Color.Hex = fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
		return zone
	}
	switch scope {
	case "zone":
		key, err := strconv.Atoi(zoneID)
		if err != nil || groupID != "" {
			return fmt.Errorf("invalid authored zone selection")
		}
		zone, ok := d.DeviceProfile.ZoneColors[key]
		if !ok {
			return fmt.Errorf("unknown authored zone")
		}
		d.DeviceProfile.ZoneColors[key] = apply(zone)
	case "all":
		if zoneID != "" || groupID != "" || len(d.DeviceProfile.ZoneColors) == 0 {
			return fmt.Errorf("invalid authored zone selection")
		}
		for key, zone := range d.DeviceProfile.ZoneColors {
			d.DeviceProfile.ZoneColors[key] = apply(zone)
		}
	default:
		return fmt.Errorf("unsupported authored zone scope")
	}
	d.saveDeviceProfile()
	selected, err := d.currentCanonicalSelectedEffect()
	if err == nil && selected == effect && !d.DeviceProfile.RGBCluster && !d.DeviceProfile.OpenRGBIntegration {
		if d.lightingRestart != nil {
			d.lightingRestart()
		} else {
			d.restartCanonicalLighting()
		}
	}
	return nil
}

func (d *Device) SetLightingZoneColors(effect string, zoneIDs []string, color rgb.Color) error {
	if d == nil || d.DeviceProfile == nil || effect != "mouse" || !d.SupportsLightingEffect(effect) || len(zoneIDs) == 0 {
		return fmt.Errorf("Scimitar RGB Elite authored lighting is unavailable")
	}
	if color.Red < 0 || color.Red > 255 || color.Green < 0 || color.Green > 255 || color.Blue < 0 || color.Blue > 255 {
		return fmt.Errorf("invalid authored zone color")
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	keys := make([]int, 0, len(zoneIDs))
	seen := map[int]struct{}{}
	for _, id := range zoneIDs {
		key, err := strconv.Atoi(id)
		if err != nil {
			return fmt.Errorf("invalid authored zone selection")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate authored zone")
		}
		if _, ok := d.DeviceProfile.ZoneColors[key]; !ok {
			return fmt.Errorf("unknown authored zone")
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for _, key := range keys {
		zone := d.DeviceProfile.ZoneColors[key]
		if zone.Color == nil {
			zone.Color = &rgb.Color{}
		}
		zone.Color.Red, zone.Color.Green, zone.Color.Blue, zone.Color.Hex = color.Red, color.Green, color.Blue, fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
		d.DeviceProfile.ZoneColors[key] = zone
	}
	d.saveDeviceProfile()
	selected, err := d.currentCanonicalSelectedEffect()
	if err == nil && selected == effect && !d.DeviceProfile.RGBCluster && !d.DeviceProfile.OpenRGBIntegration {
		if d.lightingRestart != nil {
			d.lightingRestart()
		} else {
			d.restartCanonicalLighting()
		}
	}
	return nil
}

func (d *Device) SupportsLightingEffect(effect string) bool {
	return scimitarEliteSupportsEffect(effect)
}

func (d *Device) SetLightingEffect(effect string) error {
	if d == nil {
		return fmt.Errorf("Scimitar RGB Elite lighting ownership is unavailable")
	}
	if !d.SupportsLightingEffect(effect) {
		return fmt.Errorf("unsupported Scimitar RGB Elite effect %q", effect)
	}
	if d.DeviceProfile == nil {
		return fmt.Errorf("Scimitar RGB Elite lighting ownership is unavailable")
	}
	if d.DeviceProfile.RGBCluster {
		return fmt.Errorf("Scimitar RGB Elite lighting is owned by RGB Cluster")
	}
	if d.DeviceProfile.OpenRGBIntegration {
		return fmt.Errorf("Scimitar RGB Elite lighting is owned by OpenRGB")
	}

	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	if err := d.setCanonicalSelectedEffect(effect); err != nil {
		return fmt.Errorf("persist Scimitar RGB Elite selected effect: %w", err)
	}
	d.restartCanonicalLighting()
	return nil
}

func (d *Device) SetLightingBrightness(brightness uint8) error {
	if d == nil || d.DeviceProfile == nil {
		return fmt.Errorf("Scimitar RGB Elite lighting ownership is unavailable")
	}

	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	if err := d.setCanonicalBrightness(brightness); err != nil {
		return fmt.Errorf("persist Scimitar RGB Elite brightness: %w", err)
	}
	if d.DeviceProfile.RGBCluster || d.DeviceProfile.OpenRGBIntegration {
		return nil
	}
	d.restartCanonicalLighting()
	return nil
}

func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if !d.SupportsLightingEffect(effect) || effect == "mouse" {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported Scimitar RGB Elite effect %q", effect)
	}
	if d == nil || d.lightingSource == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("Scimitar RGB Elite canonical lighting source is unavailable")
	}
	return d.lightingSource.resolveEffectSettings(effect)
}

func (d *Device) SetLightingEffectSettings(effect string, settings lightingsettings.EffectSettings) error {
	if d == nil {
		return fmt.Errorf("Scimitar RGB Elite lighting ownership is unavailable")
	}
	if !d.SupportsLightingEffect(effect) || effect == "mouse" {
		return fmt.Errorf("unsupported Scimitar RGB Elite effect %q", effect)
	}
	if settings.EffectID != effect {
		return fmt.Errorf("Scimitar RGB Elite effect settings do not match %q", effect)
	}
	if err := lightingsettings.Validate(settings); err != nil {
		return fmt.Errorf("validate Scimitar RGB Elite effect settings: %w", err)
	}
	if d == nil || d.DeviceProfile == nil || d.lightingSource == nil {
		return fmt.Errorf("Scimitar RGB Elite lighting ownership is unavailable")
	}

	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	selected, err := d.lightingSource.selectedEffect()
	if err != nil {
		return fmt.Errorf("resolve Scimitar RGB Elite selected effect: %w", err)
	}
	if err := d.lightingSource.setEffectSettings(effect, settings.Clone()); err != nil {
		return fmt.Errorf("persist Scimitar RGB Elite effect settings: %w", err)
	}
	if selected == effect && !d.DeviceProfile.RGBCluster && !d.DeviceProfile.OpenRGBIntegration {
		d.restartCanonicalLighting()
	}
	return nil
}

func (d *Device) ResetLightingEffectSettings(effect string) error {
	if d == nil {
		return fmt.Errorf("Scimitar RGB Elite lighting ownership is unavailable")
	}
	if !d.SupportsLightingEffect(effect) || effect == "mouse" {
		return fmt.Errorf("unsupported Scimitar RGB Elite effect %q", effect)
	}
	if d == nil || d.DeviceProfile == nil || d.lightingSource == nil {
		return fmt.Errorf("Scimitar RGB Elite lighting ownership is unavailable")
	}

	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	selected, err := d.lightingSource.selectedEffect()
	if err != nil {
		return fmt.Errorf("resolve Scimitar RGB Elite selected effect: %w", err)
	}
	deleted, err := d.lightingSource.deleteEffectSettings(effect)
	if err != nil {
		return fmt.Errorf("reset Scimitar RGB Elite effect settings: %w", err)
	}
	if deleted && selected == effect && !d.DeviceProfile.RGBCluster && !d.DeviceProfile.OpenRGBIntegration {
		d.restartCanonicalLighting()
	}
	return nil
}
