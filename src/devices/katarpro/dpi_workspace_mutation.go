package katarpro

import (
	"LumenForge/src/rgb"
	"fmt"
)

func (d *Device) SelectMouseDPIStage(stage int) uint8 {
	if d == nil {
		return 0
	}
	d.lightingMu.Lock()
	if d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		return 0
	}
	profile, ok := d.DeviceProfile.Profiles[stage]
	if !ok || profile.Sniper {
		d.lightingMu.Unlock()
		return 0
	}
	if d.DeviceProfile.Profile == stage {
		d.lightingMu.Unlock()
		return 1
	}
	d.DeviceProfile.Profile = stage
	d.saveDeviceProfileLocked()
	sniper := d.SniperMode
	d.lightingMu.Unlock()
	if !sniper {
		d.toggleDPI(false)
	}
	return 1
}
func (d *Device) SetMouseSniperMode(active bool) uint8 {
	if d == nil {
		return 0
	}
	d.lightingMu.Lock()
	if d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		return 0
	}
	found := false
	for _, profile := range d.DeviceProfile.Profiles {
		if profile.Sniper {
			found = true
			break
		}
	}
	changed := d.SniperMode != active
	d.lightingMu.Unlock()
	if !found {
		return 0
	}
	if changed {
		d.sniperMode(active)
	}
	return 1
}
func (d *Device) SaveMouseDPISettings(stages map[int]uint16, colors map[int]rgb.Color) uint8 {
	if d == nil {
		return 0
	}
	d.lightingMu.Lock()
	result := d.saveMouseDPISettingsLocked(stages, colors)
	d.lightingMu.Unlock()
	if result == 1 {
		d.toggleDPI(false)
	}
	return result
}
func (d *Device) saveMouseDPISettingsLocked(stages map[int]uint16, colors map[int]rgb.Color) uint8 {
	if d == nil || d.DeviceProfile == nil || len(stages) == 0 || len(stages) != len(colors) {
		return 0
	}
	for key, value := range stages {
		profile, exists := d.DeviceProfile.Profiles[key]
		color, colored := colors[key]
		if !exists || !colored || profile.Color == nil || value < uint16(d.MinDPI) || value > uint16(d.MaxDPI) || color.Red < 0 || color.Red > 255 || color.Green < 0 || color.Green > 255 || color.Blue < 0 || color.Blue > 255 {
			return 0
		}
	}
	for key := range colors {
		if _, exists := stages[key]; !exists {
			return 0
		}
	}
	for key, value := range stages {
		profile := d.DeviceProfile.Profiles[key]
		color := colors[key]
		profile.Value = value
		profile.Color.Red = color.Red
		profile.Color.Green = color.Green
		profile.Color.Blue = color.Blue
		profile.Color.Hex = fmt.Sprintf("#%02x%02x%02x", int(color.Red), int(color.Green), int(color.Blue))
		d.DeviceProfile.Profiles[key] = profile
	}
	d.saveDeviceProfileLocked()
	return 1
}
