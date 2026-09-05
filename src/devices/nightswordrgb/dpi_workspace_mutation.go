package nightswordrgb

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
func nightswordRGBColorOK(c rgb.Color) bool {
	return c.Red >= 0 && c.Red <= 255 && c.Green >= 0 && c.Green <= 255 && c.Blue >= 0 && c.Blue <= 255
}
func nightswordRGBSame(a, b rgb.Color) bool {
	return a.Red == b.Red && a.Green == b.Green && a.Blue == b.Blue
}
func nightswordRGBColor(c rgb.Color) *rgb.Color {
	c.Hex = fmt.Sprintf("#%02x%02x%02x", int(c.Red), int(c.Green), int(c.Blue))
	return &c
}
func (d *Device) SaveMouseDPISettings(stages map[int]uint16, colors map[int]rgb.Color) uint8 {
	if d == nil || d.DeviceProfile == nil || len(d.DeviceProfile.Profiles) == 0 || d.DeviceProfile.DPIColor == nil || d.DeviceProfile.SniperColor == nil || len(stages) != len(d.DeviceProfile.Profiles) || len(colors) != len(d.DeviceProfile.Profiles) {
		return 0
	}
	var regular, sniper *rgb.Color
	for k, p := range d.DeviceProfile.Profiles {
		v, a := stages[k]
		c, b := colors[k]
		if !a || !b || v < uint16(d.MinDPI) || v > uint16(d.MaxDPI) || !nightswordRGBColorOK(c) {
			return 0
		}
		if p.Sniper {
			x := c
			sniper = &x
		} else if regular == nil {
			x := c
			regular = &x
		} else if !nightswordRGBSame(*regular, c) {
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
	if regular == nil || sniper == nil {
		return 0
	}
	for k, v := range stages {
		p := d.DeviceProfile.Profiles[k]
		p.Value = v
		d.DeviceProfile.Profiles[k] = p
	}
	d.DeviceProfile.DPIColor = nightswordRGBColor(*regular)
	d.DeviceProfile.SniperColor = nightswordRGBColor(*sniper)
	d.saveDeviceProfile()
	d.updateMouseDPI()
	d.toggleDPI()
	return 1
}
