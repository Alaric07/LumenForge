package darkcorergbproW

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
		d.toggleDPI()
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

func darkcorergbproWSharedDPIColor(colors map[int]rgb.Color, current rgb.Color) (rgb.Color, bool) {
	var candidate rgb.Color
	haveCandidate := false
	for _, color := range colors {
		if !darkcorergbproWColorOK(color) {
			return rgb.Color{}, false
		}
		if color.Red == current.Red && color.Green == current.Green && color.Blue == current.Blue {
			continue
		}
		if !haveCandidate {
			candidate, haveCandidate = color, true
			continue
		}
		if color.Red != candidate.Red || color.Green != candidate.Green || color.Blue != candidate.Blue {
			return rgb.Color{}, false
		}
	}
	if haveCandidate {
		return candidate, true
	}
	for _, color := range colors {
		return color, true
	}
	return rgb.Color{}, false
}

func darkcorergbproWApplyMouseDPISettings(profile *DeviceProfile, stages map[int]uint16, color rgb.Color) {
	for key, value := range stages {
		stage := profile.Profiles[key]
		stage.Value = value
		profile.Profiles[key] = stage
	}
	profile.DPIColor.Red, profile.DPIColor.Green, profile.DPIColor.Blue = color.Red, color.Green, color.Blue
	profile.DPIColor.Hex = fmt.Sprintf("#%02x%02x%02x", int(color.Red), int(color.Green), int(color.Blue))
}

func (d *Device) SaveMouseDPISettings(stages map[int]uint16, colors map[int]rgb.Color) uint8 {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.DPIColor == nil || len(d.DeviceProfile.Profiles) == 0 || len(stages) != len(d.DeviceProfile.Profiles) || len(colors) != len(d.DeviceProfile.Profiles) {
		return 0
	}
	for key := range d.DeviceProfile.Profiles {
		value, hasValue := stages[key]
		color, hasColor := colors[key]
		if !hasValue || !hasColor || value < uint16(d.MinDPI) || value > uint16(d.MaxDPI) || !darkcorergbproWColorOK(color) {
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
	color, ok := darkcorergbproWSharedDPIColor(colors, *d.DeviceProfile.DPIColor)
	if !ok {
		return 0
	}
	darkcorergbproWApplyMouseDPISettings(d.DeviceProfile, stages, color)
	d.saveDeviceProfile()
	d.toggleDPI()
	return 1
}
