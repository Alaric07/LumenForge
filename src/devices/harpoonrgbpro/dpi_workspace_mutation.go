package harpoonrgbpro

import (
	"fmt"

	"LumenForge/src/rgb"
)

// SelectMouseDPIStage selects an existing regular stage through the same
// DeviceProfile state used by the physical DPI button.
func (d *Device) SelectMouseDPIStage(stage int) uint8 {
	if d == nil || d.DeviceProfile == nil {
		return 0
	}
	profile, exists := d.DeviceProfile.Profiles[stage]
	if !exists || profile.Sniper {
		return 0
	}
	if d.DeviceProfile.Profile == stage {
		return 1
	}
	d.DeviceProfile.Profile = stage
	d.saveDeviceProfile()
	if !d.SniperMode {
		d.toggleDPI(false)
	}
	return 1
}

// SetMouseSniperMode delegates to Harpoon's existing runtime Sniper path.
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

// SaveMouseDPISettings applies the shared workspace's complete DPI draft to
// the existing Harpoon DeviceProfile, then uses its established output path.
func (d *Device) SaveMouseDPISettings(stages map[int]uint16, colors map[int]rgb.Color) uint8 {
	if d == nil || d.DeviceProfile == nil || len(stages) == 0 || len(stages) != len(colors) {
		return 0
	}
	for key, value := range stages {
		profile, exists := d.DeviceProfile.Profiles[key]
		color, hasColor := colors[key]
		if !exists || !hasColor || profile.Color == nil || value < uint16(d.MinDPI) || value > uint16(d.MaxDPI) ||
			color.Red < 0 || color.Red > 255 || color.Green < 0 || color.Green > 255 || color.Blue < 0 || color.Blue > 255 {
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
	d.saveDeviceProfile()
	d.updateMouseDPI()
	d.toggleDPI(false)
	return 1
}
