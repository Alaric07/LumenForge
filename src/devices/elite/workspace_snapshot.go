package elite

import (
	"LumenForge/src/coolingpresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/temperatures"
	"sort"
)

var coolingTemperatureProfiles = temperatures.GetTemperatureProfiles

// CoolingDeviceID and CoolingSnapshot adapt existing ELITE AIO state without
// changing its HID, profile, or pump-control behavior.
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
		if device == nil || !device.HasSpeed {
			continue
		}
		channel := coolingpresentation.Channel{ID: device.ChannelId, Name: device.Name, Label: device.Label, RPM: int16(device.Rpm), ContainsPump: device.ContainsPump, SelectedProfile: device.Profile}
		if device.HasTemps && device.TemperatureString != "" {
			channel.Temperature = device.TemperatureString
			if device.Temperature > 0 {
				value := float32(device.Temperature)
				channel.Celsius = &value
			}
		}
		if device.ContainsPump && len(device.PumpModes) > 0 {
			modes := make([]byte, 0, len(device.PumpModes))
			for mode := range device.PumpModes {
				modes = append(modes, mode)
			}
			sort.Slice(modes, func(i, j int) bool { return modes[i] < modes[j] })
			for _, mode := range modes {
				channel.PumpModeOptions = append(channel.PumpModeOptions, coolingpresentation.ProfileOption{ID: device.PumpModes[mode], Label: device.PumpModes[mode]})
			}
		}
		snapshot.Channels = append(snapshot.Channels, channel)
	}
	sort.Slice(snapshot.Channels, func(i, j int) bool { return snapshot.Channels[i].ID < snapshot.Channels[j].ID })
	return snapshot, len(snapshot.Channels) > 0
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
	return snapshot, snapshot.ActiveProfile != "" && len(snapshot.Profiles) > 0 && snapshot.CanSwitch && snapshot.CanSave && snapshot.CanDelete
}
