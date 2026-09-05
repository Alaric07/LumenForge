package m65prorgb

import (
	"LumenForge/src/rgb"
	"fmt"
)

func (d *Device) SelectMouseDPIStage(stage int) uint8 {
	if d == nil || d.DeviceProfile == nil {
		return 0
	}
	p, ok := d.DeviceProfile.Profiles[stage]
	if !ok || p.Sniper {
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
	for _, p := range d.DeviceProfile.Profiles {
		if p.Sniper {
			if d.SniperMode != active {
				d.sniperMode(active)
			}
			return 1
		}
	}
	return 0
}
func m65ProRGBColorOK(c rgb.Color) bool {
	return c.Red >= 0 && c.Red <= 255 && c.Green >= 0 && c.Green <= 255 && c.Blue >= 0 && c.Blue <= 255
}
func (d *Device) SaveMouseDPISettings(stages map[int]uint16, colors map[int]rgb.Color) uint8 {
	if d == nil || d.DeviceProfile == nil || len(d.DeviceProfile.Profiles) == 0 || len(stages) != len(d.DeviceProfile.Profiles) || len(colors) != len(d.DeviceProfile.Profiles) {
		return 0
	}
	for k, p := range d.DeviceProfile.Profiles {
		v, a := stages[k]
		c, b := colors[k]
		if !a || !b || p.Color == nil || v < uint16(d.MinDPI) || v > uint16(d.MaxDPI) || !m65ProRGBColorOK(c) {
			return 0
		}
	}
	for k := range stages {
		if _, ok := d.DeviceProfile.Profiles[k]; !ok {
			return 0
		}
	}
	for k := range colors {
		if _, ok := d.DeviceProfile.Profiles[k]; !ok {
			return 0
		}
	}
	for k, v := range stages {
		p := d.DeviceProfile.Profiles[k]
		c := colors[k]
		p.Value = v
		p.Color.Red = c.Red
		p.Color.Green = c.Green
		p.Color.Blue = c.Blue
		p.Color.Hex = fmt.Sprintf("#%02x%02x%02x", int(c.Red), int(c.Green), int(c.Blue))
		d.DeviceProfile.Profiles[k] = p
	}
	d.saveDeviceProfile()
	d.toggleDPI()
	return 1
}
