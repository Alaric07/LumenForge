package cone

import (
	"LumenForge/src/coolingpresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/temperatures"
	"sort"
)

var coolingTemperatureProfiles = temperatures.GetTemperatureProfiles

func (d *Device) CoolingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// CoolingSnapshot presents Corsair ONE's fixed hardware topology without
// changing its established logical controller IDs. DeviceList.Channel is the
// source/HID channel (28 for the pump and 14 for Fan 1); DeviceList.Index is
// the package's existing key used by UpdateSpeedProfile and telemetry.
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

	for _, source := range deviceList {
		device, ok := d.Devices[source.Index]
		if !ok || device == nil || !device.HasSpeed || device.Profile == "" {
			return coolingpresentation.Snapshot{}, false
		}
		channel := coolingpresentation.Channel{ID: device.ChannelId, SourceID: int(source.Channel), Name: device.Name, Label: device.Label, RPM: int16(device.Rpm), ContainsPump: device.ContainsPump, SelectedProfile: device.Profile}
		if channel.Name == "" || channel.ID != source.Index || channel.ContainsPump != source.Pump {
			return coolingpresentation.Snapshot{}, false
		}
		if device.HasTemps && device.TemperatureString != "" {
			channel.Temperature = device.TemperatureString
			if device.Temperature > 0 {
				value := float32(device.Temperature)
				channel.Celsius = &value
			}
		}
		if source.Pump {
			if len(device.PumpModes) == 0 {
				return coolingpresentation.Snapshot{}, false
			}
			modes := make([]byte, 0, len(device.PumpModes))
			for mode := range device.PumpModes {
				modes = append(modes, mode)
			}
			sort.Slice(modes, func(i, j int) bool { return modes[i] < modes[j] })
			for _, mode := range modes {
				label := device.PumpModes[mode]
				if label == "" {
					return coolingpresentation.Snapshot{}, false
				}
				channel.PumpModeOptions = append(channel.PumpModeOptions, coolingpresentation.ProfileOption{ID: label, Label: label})
			}
		}
		snapshot.Channels = append(snapshot.Channels, channel)
	}
	return snapshot, len(snapshot.Channels) == len(deviceList)
}

func (d *Device) DeviceProfileDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil {
		return deviceprofilepresentation.Snapshot{}, false
	}
	snapshot := deviceprofilepresentation.Snapshot{Supported: true, Scope: deviceprofilepresentation.ScopeDevice, DefaultProfileDisplayLabel: deviceprofilepresentation.WorkingConfigurationLabel}
	for name, profile := range d.UserProfiles {
		if profile == nil {
			continue
		}
		snapshot.Profiles = append(snapshot.Profiles, name)
		if profile.Active {
			snapshot.ActiveProfile = name
		}
	}
	sort.Strings(snapshot.Profiles)
	snapshot = deviceprofilepresentation.WithMutationCapabilities(snapshot, d)
	return snapshot, snapshot.ActiveProfile != "" && len(snapshot.Profiles) > 0
}
