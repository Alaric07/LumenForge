package darkcorergbseWU

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
func (d *Device) SaveMouseDPISettings(stages map[int]uint16, colors map[int]rgb.Color) uint8 {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.DPIColor == nil || d.DeviceProfile.SniperColor == nil || len(d.DeviceProfile.Profiles) == 0 || len(stages) != len(d.DeviceProfile.Profiles) || len(colors) != len(d.DeviceProfile.Profiles) {
		return 0
	}
	var regular, sniper rgb.Color
	hasRegular, hasSniper := false, false
	for key, profile := range d.DeviceProfile.Profiles {
		value, hasValue := stages[key]
		color, hasColor := colors[key]
		if !hasValue || !hasColor || value < uint16(d.MinDPI) || value > uint16(d.MaxDPI) || !darkcorergbseWUColorOK(color) {
			return 0
		}
		if profile.Sniper {
			if !hasSniper {
				sniper = color
				hasSniper = true
			} else if color.Red != sniper.Red || color.Green != sniper.Green || color.Blue != sniper.Blue {
				return 0
			}
		} else {
			if !hasRegular {
				regular = color
				hasRegular = true
			} else if color.Red != regular.Red || color.Green != regular.Green || color.Blue != regular.Blue {
				return 0
			}
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
	if !hasRegular || !hasSniper {
		return 0
	}
	for key, value := range stages {
		profile := d.DeviceProfile.Profiles[key]
		profile.Value = value
		d.DeviceProfile.Profiles[key] = profile
	}
	d.DeviceProfile.DPIColor.Red, d.DeviceProfile.DPIColor.Green, d.DeviceProfile.DPIColor.Blue = regular.Red, regular.Green, regular.Blue
	d.DeviceProfile.SniperColor.Red, d.DeviceProfile.SniperColor.Green, d.DeviceProfile.SniperColor.Blue = sniper.Red, sniper.Green, sniper.Blue
	d.DeviceProfile.DPIColor.Hex = fmt.Sprintf("#%02x%02x%02x", int(regular.Red), int(regular.Green), int(regular.Blue))
	d.DeviceProfile.SniperColor.Hex = fmt.Sprintf("#%02x%02x%02x", int(sniper.Red), int(sniper.Green), int(sniper.Blue))
	d.saveDeviceProfile()
	d.toggleDPI(true)
	return 1
}
