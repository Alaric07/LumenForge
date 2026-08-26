package mm800

import (
	"fmt"
	"slices"
	"strconv"

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

// SetLightingZoneColor updates the persisted authored Mousepad colors without
// routing through the legacy numeric selection endpoint.
func (d *Device) SetLightingZoneColor(effect, scope, zoneID, groupID string, color rgb.Color) error {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Mousepad == nil || effect != "mousepad" || !d.SupportsLightingEffect(effect) {
		return fmt.Errorf("MM800 authored lighting is unavailable")
	}
	if color.Red < 0 || color.Red > 255 || color.Green < 0 || color.Green > 255 || color.Blue < 0 || color.Blue > 255 {
		return fmt.Errorf("invalid authored zone color")
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	apply := func(zone Zones) Zones {
		zone.Color.Red, zone.Color.Green, zone.Color.Blue = color.Red, color.Green, color.Blue
		zone.Color.Hex = fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
		return zone
	}
	applyRow := func(rowID int) error {
		row, ok := d.DeviceProfile.Mousepad.Row[rowID]
		if !ok || len(row.Zones) == 0 {
			return fmt.Errorf("unknown authored group")
		}
		for key, zone := range row.Zones {
			row.Zones[key] = apply(zone)
		}
		d.DeviceProfile.Mousepad.Row[rowID] = row
		return nil
	}
	switch scope {
	case "zone":
		key, err := strconv.Atoi(zoneID)
		if err != nil || groupID != "" {
			return fmt.Errorf("invalid authored zone selection")
		}
		found := false
		for rowID, row := range d.DeviceProfile.Mousepad.Row {
			zone, ok := row.Zones[key]
			if !ok {
				continue
			}
			row.Zones[key] = apply(zone)
			d.DeviceProfile.Mousepad.Row[rowID] = row
			found = true
			break
		}
		if !found {
			return fmt.Errorf("unknown authored zone")
		}
	case "group":
		rowID, err := strconv.Atoi(groupID)
		if err != nil || zoneID != "" {
			return fmt.Errorf("invalid authored group selection")
		}
		if err = applyRow(rowID); err != nil {
			return err
		}
	case "all":
		if zoneID != "" || groupID != "" || len(d.DeviceProfile.Mousepad.Row) == 0 {
			return fmt.Errorf("invalid authored zone selection")
		}
		for rowID := range d.DeviceProfile.Mousepad.Row {
			if err := applyRow(rowID); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported authored zone scope")
	}
	d.saveDeviceProfile()
	selected, err := d.currentCanonicalSelectedEffect()
	if err == nil && selected == effect && d.locallyOwnsLighting() {
		if d.lightingRestart != nil {
			d.lightingRestart()
		} else {
			d.restartCanonicalLighting()
		}
	}
	return nil
}

func (d *Device) SetLightingZoneColors(effect string, zoneIDs []string, color rgb.Color) error {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Mousepad == nil || effect != "mousepad" || !d.SupportsLightingEffect(effect) || len(zoneIDs) == 0 {
		return fmt.Errorf("MM800 authored lighting is unavailable")
	}
	if color.Red < 0 || color.Red > 255 || color.Green < 0 || color.Green > 255 || color.Blue < 0 || color.Blue > 255 {
		return fmt.Errorf("invalid authored zone color")
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	type location struct{ rowID, zoneID int }
	locations := make([]location, 0, len(zoneIDs))
	seen := map[int]struct{}{}
	for _, id := range zoneIDs {
		key, err := strconv.Atoi(id)
		if err != nil {
			return fmt.Errorf("invalid authored zone selection")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate authored zone")
		}
		found := false
		for rowID, row := range d.DeviceProfile.Mousepad.Row {
			if _, ok := row.Zones[key]; ok {
				locations = append(locations, location{rowID, key})
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown authored zone")
		}
		seen[key] = struct{}{}
	}
	for _, loc := range locations {
		row := d.DeviceProfile.Mousepad.Row[loc.rowID]
		zone := row.Zones[loc.zoneID]
		zone.Color.Red, zone.Color.Green, zone.Color.Blue, zone.Color.Hex = color.Red, color.Green, color.Blue, fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
		row.Zones[loc.zoneID] = zone
		d.DeviceProfile.Mousepad.Row[loc.rowID] = row
	}
	d.saveDeviceProfile()
	selected, err := d.currentCanonicalSelectedEffect()
	if err == nil && selected == effect && d.locallyOwnsLighting() {
		if d.lightingRestart != nil {
			d.lightingRestart()
		} else {
			d.restartCanonicalLighting()
		}
	}
	return nil
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
