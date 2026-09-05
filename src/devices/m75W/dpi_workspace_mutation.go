package m75W

import (
	"LumenForge/src/rgb"
	"fmt"
)

func (d *Device) SelectMouseDPIStage(stage int) uint8 {
	if d == nil || d.DeviceProfile == nil {
		return 0
	}
	profile, ok := d.DeviceProfile.Profiles[stage]
	if !ok || profile.Sniper {
		return 0
	}
	if d.DeviceProfile.Profile == stage {
		return 1
	}
	d.DeviceProfile.Profile = stage
	d.saveDeviceProfile()
	if !d.SniperMode {
		d.toggleDPI(true)
	}
	return 1
}
func (d *Device) SetMouseSniperMode(active bool) uint8 {
	if d == nil || d.DeviceProfile == nil {
		return 0
	}
	for _, profile := range d.DeviceProfile.Profiles {
		if profile.Sniper {
			if d.SniperMode != active {
				d.sniperMode(active)
			}
			return 1
		}
	}
	return 0
}
func m75WColorOK(color rgb.Color) bool {
	return color.Red >= 0 && color.Red <= 255 && color.Green >= 0 && color.Green <= 255 && color.Blue >= 0 && color.Blue <= 255
}
func (d *Device) SaveMouseDPISettings(stages map[int]uint16, colors map[int]rgb.Color) uint8 {
	if d == nil || d.DeviceProfile == nil || len(d.DeviceProfile.Profiles) == 0 || len(stages) != len(d.DeviceProfile.Profiles) || len(colors) != len(d.DeviceProfile.Profiles) {
		return 0
	}
	for key, profile := range d.DeviceProfile.Profiles {
		value, hasValue := stages[key]
		color, hasColor := colors[key]
		if !hasValue || !hasColor || profile.Color == nil || value < uint16(d.MinDPI) || value > uint16(d.MaxDPI) || !m75WColorOK(color) {
			return 0
		}
	}
	for key := range stages {
		if _, ok := d.DeviceProfile.Profiles[key]; !ok {
			return 0
		}
	}
	for key := range colors {
		if _, ok := d.DeviceProfile.Profiles[key]; !ok {
			return 0
		}
	}
	for key, value := range stages {
		profile := d.DeviceProfile.Profiles[key]
		color := colors[key]
		profile.Value = value
		profile.Color.Red, profile.Color.Green, profile.Color.Blue = color.Red, color.Green, color.Blue
		profile.Color.Hex = fmt.Sprintf("#%02x%02x%02x", int(color.Red), int(color.Green), int(color.Blue))
		d.DeviceProfile.Profiles[key] = profile
	}
	d.saveDeviceProfile()
	d.toggleDPI(true)
	return 1
}
