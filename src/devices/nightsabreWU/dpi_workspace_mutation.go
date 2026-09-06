package nightsabreWU

import (
	"LumenForge/src/rgb"
	"fmt"
)

func (d *Device) SelectMouseDPIStage(s int) uint8 {
	if d == nil || d.DeviceProfile == nil {
		return 0
	}
	x, ok := d.DeviceProfile.Profiles[s]
	if !ok || x.Sniper {
		return 0
	}
	d.DeviceProfile.Profile = s
	d.saveDeviceProfile()
	if !d.SniperMode {
		d.toggleDPI()
	}
	return 1
}
func (d *Device) SetMouseSniperMode(a bool) uint8 {
	if d == nil || d.DeviceProfile == nil {
		return 0
	}
	for _, x := range d.DeviceProfile.Profiles {
		if x.Sniper {
			if d.SniperMode != a {
				d.sniperMode(a)
			}
			return 1
		}
	}
	return 0
}
func nightsabreWUColorOK(c rgb.Color) bool {
	return c.Red >= 0 && c.Red <= 255 && c.Green >= 0 && c.Green <= 255 && c.Blue >= 0 && c.Blue <= 255
}
func (d *Device) SaveMouseDPISettings(v map[int]uint16, c map[int]rgb.Color) uint8 {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.DPIColor == nil || len(v) != len(d.DeviceProfile.Profiles) || len(c) != len(d.DeviceProfile.Profiles) {
		return 0
	}
	var x rgb.Color
	first := true
	for k, p := range d.DeviceProfile.Profiles {
		n, ok := v[k]
		z, zok := c[k]
		if !ok || !zok || n < uint16(d.MinDPI) || n > uint16(d.MaxDPI) || !nightsabreWUColorOK(z) {
			return 0
		}
		if first {
			x = z
			first = false
		} else if z.Red != x.Red || z.Green != x.Green || z.Blue != x.Blue {
			return 0
		}
		p.Value = n
		d.DeviceProfile.Profiles[k] = p
	}
	d.DeviceProfile.DPIColor.Red, d.DeviceProfile.DPIColor.Green, d.DeviceProfile.DPIColor.Blue = x.Red, x.Green, x.Blue
	d.DeviceProfile.DPIColor.Hex = fmt.Sprintf("#%02x%02x%02x", int(x.Red), int(x.Green), int(x.Blue))
	d.saveDeviceProfile()
	d.toggleDPI()
	return 1
}
