package ccxt

import (
	"LumenForge/src/coolingpresentation"
	"LumenForge/src/temperatures"
	"sort"
)

var coolingTemperatureProfiles = temperatures.GetTemperatureProfiles

// CoolingDeviceID and CoolingSnapshot adapt existing CCXT state to the shared
// Devices cooling workspace without changing controller behavior.
func (d *Device) CoolingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) CoolingSnapshot() (coolingpresentation.Snapshot, bool) {
	if d == nil {
		return coolingpresentation.Snapshot{}, false
	}

	snapshot := coolingpresentation.Snapshot{Available: true}
	for name, profile := range coolingTemperatureProfiles() {
		if !profile.Hidden {
			snapshot.ProfileOptions = append(snapshot.ProfileOptions, coolingpresentation.ProfileOption{ID: name, Label: name})
		}
	}
	sort.Slice(snapshot.ProfileOptions, func(i, j int) bool { return snapshot.ProfileOptions[i].ID < snapshot.ProfileOptions[j].ID })

	for _, device := range d.Devices {
		if device == nil {
			continue
		}
		if device.HasSpeed {
			var celsius *float32
			if device.Temperature > 0 {
				value := device.Temperature
				celsius = &value
			}
			snapshot.Channels = append(snapshot.Channels, coolingpresentation.Channel{ID: device.ChannelId, Name: device.Name, Label: device.Label, RPM: device.Rpm, Temperature: device.TemperatureString, Celsius: celsius, ContainsPump: device.ContainsPump, SelectedProfile: device.Profile})
			continue
		}
		if device.IsTemperatureProbe {
			var celsius *float32
			if device.Temperature > 0 {
				value := device.Temperature
				celsius = &value
			}
			snapshot.TemperatureProbes = append(snapshot.TemperatureProbes, coolingpresentation.TemperatureProbe{ID: device.ChannelId, Name: device.Name, Label: device.Label, Temperature: device.TemperatureString, Celsius: celsius})
		}
	}
	sort.Slice(snapshot.Channels, func(i, j int) bool { return snapshot.Channels[i].ID < snapshot.Channels[j].ID })
	sort.Slice(snapshot.TemperatureProbes, func(i, j int) bool { return snapshot.TemperatureProbes[i].ID < snapshot.TemperatureProbes[j].ID })
	return snapshot, len(snapshot.Channels) > 0
}
