package glaivergbpro

import (
	"fmt"

	"LumenForge/src/rgb"
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

// SaveMouseDPISettings validates a complete shared-workspace draft, retains
// Glaive's device-wide regular/sniper color model, and uses its established
// profile persistence and DPI output path. SaveMouseZoneColorsSniper is not
// used because it also changes legacy RGB-zone colors, which this workspace
// does not own.
func (d *Device) SaveMouseDPISettings(stages map[int]uint16, colors map[int]rgb.Color) uint8 {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.DPIColor == nil || d.DeviceProfile.SniperColor == nil || len(stages) == 0 || len(stages) != len(d.DeviceProfile.Profiles) || len(colors) != len(d.DeviceProfile.Profiles) {
		return 0
	}
	var regularColor *rgb.Color
	var sniperColor *rgb.Color
	for key, profile := range d.DeviceProfile.Profiles {
		value, hasValue := stages[key]
		color, hasColor := colors[key]
		if !hasValue || !hasColor || value < uint16(d.MinDPI) || value > uint16(d.MaxDPI) || !glaiveRGBProValidColor(color) {
			return 0
		}
		if profile.Sniper {
			copy := color
			sniperColor = &copy
		} else if regularColor == nil {
			copy := color
			regularColor = &copy
		} else if !glaiveRGBProSameRGB(*regularColor, color) {
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
	if regularColor == nil || sniperColor == nil {
		return 0
	}
	for key, value := range stages {
		profile := d.DeviceProfile.Profiles[key]
		profile.Value = value
		d.DeviceProfile.Profiles[key] = profile
	}
	d.DeviceProfile.DPIColor = glaiveRGBProColorPointer(*regularColor)
	d.DeviceProfile.SniperColor = glaiveRGBProColorPointer(*sniperColor)
	d.saveDeviceProfile()
	d.updateMouseDPI()
	d.toggleDPI()
	return 1
}

func glaiveRGBProValidColor(color rgb.Color) bool {
	return color.Red >= 0 && color.Red <= 255 && color.Green >= 0 && color.Green <= 255 && color.Blue >= 0 && color.Blue <= 255
}
func glaiveRGBProSameRGB(left, right rgb.Color) bool {
	return left.Red == right.Red && left.Green == right.Green && left.Blue == right.Blue
}
func glaiveRGBProColorPointer(color rgb.Color) *rgb.Color {
	color.Hex = fmt.Sprintf("#%02x%02x%02x", int(color.Red), int(color.Green), int(color.Blue))
	return &color
}
