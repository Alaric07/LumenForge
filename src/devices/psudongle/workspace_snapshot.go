package psudongle

import (
	"LumenForge/src/psupresentation"
	"strconv"
)

var modernPSUFanModeValues = []int{0, 4, 5, 6, 7, 8, 9, 10}

func (d *Device) PSUID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) PSUSnapshot() (psupresentation.Snapshot, bool) {
	if d == nil || d.Serial == "" || d.DeviceProfile == nil || d.Devices == nil || !validModernPSUFanModes(d.FanModes) || !validModernPSUFanMode(d.DeviceProfile.FanMode) {
		return psupresentation.Snapshot{}, false
	}
	fan, vrm, psu, powerOut := d.Devices[1], d.Devices[2], d.Devices[3], d.Devices[4]
	if fan == nil || !fan.HasSpeed || vrm == nil || !vrm.HasTemps || vrm.TemperatureString == "" || psu == nil || !psu.HasTemps || psu.TemperatureString == "" || powerOut == nil || !powerOut.Output || !powerOut.HasWatts {
		return psupresentation.Snapshot{}, false
	}
	options := make([]psupresentation.FanMode, 0, len(modernPSUFanModeValues))
	for _, mode := range modernPSUFanModeValues {
		options = append(options, psupresentation.FanMode{Value: mode, Label: d.FanModes[mode]})
	}
	s := psupresentation.Snapshot{DeviceID: d.Serial, Fan: psupresentation.Fan{RPM: strconv.FormatFloat(fan.Rpm, 'f', -1, 64) + " RPM", Mode: d.DeviceProfile.FanMode, Options: options}, Temperatures: []psupresentation.Temperature{{Label: "VRM Temperature", Value: vrm.TemperatureString}, {Label: "PSU Temperature", Value: psu.TemperatureString}}, PowerOut: psuValue(powerOut.Watts, "W")}
	for _, channel := range []int{5, 6, 7} {
		rail := d.Devices[channel]
		if rail == nil || !rail.Rail || !rail.HasWatts || !rail.HasAmps || !rail.HasVolts {
			return psupresentation.Snapshot{}, false
		}
		s.Rails = append(s.Rails, psupresentation.Rail{Label: rail.Name, Watts: psuValue(rail.Watts, "W"), Amps: psuValue(rail.Amps, "A"), Volts: psuValue(rail.Volts, "V")})
	}
	return s, len(s.Rails) == 3
}

func validModernPSUFanMode(mode int) bool {
	for _, value := range modernPSUFanModeValues {
		if value == mode {
			return true
		}
	}
	return false
}

func validModernPSUFanModes(modes map[int]string) bool {
	if len(modes) != len(modernPSUFanModeValues) {
		return false
	}
	for _, value := range modernPSUFanModeValues {
		if modes[value] == "" {
			return false
		}
	}
	return true
}

func psuValue(value float64, unit string) string {
	return strconv.FormatFloat(value, 'f', -1, 64) + " " + unit
}
