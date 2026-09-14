package motherboard

import (
	"LumenForge/src/coolingpresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/temperatures"
	"sort"
)

var coolingTemperatureProfiles = temperatures.GetTemperatureProfiles

// CoolingDeviceID and CoolingSnapshot expose only the native motherboard's
// existing header state. ProductTypeMotherboard is deliberately not used as a
// workspace discriminator because OpenRGB imports share that product type.
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
	profiles := map[string]bool{}
	for name, profile := range coolingTemperatureProfiles() {
		if !profile.Hidden {
			profiles[name] = true
			snapshot.ProfileOptions = append(snapshot.ProfileOptions, coolingpresentation.ProfileOption{ID: name, Label: name})
		}
	}
	sort.Slice(snapshot.ProfileOptions, func(i, j int) bool { return snapshot.ProfileOptions[i].ID < snapshot.ProfileOptions[j].ID })
	if len(snapshot.ProfileOptions) == 0 {
		return coolingpresentation.Snapshot{}, false
	}
	for _, device := range d.Devices {
		if device == nil || !device.HasSpeed || device.ChannelId < 0 || device.Name == "" || !profiles[device.Profile] {
			continue
		}
		channel := coolingpresentation.Channel{ID: device.ChannelId, SourceID: device.ChannelId, Name: device.Name, Label: device.Label, RPM: device.Rpm, ContainsPump: device.ContainsPump, SelectedProfile: device.Profile, HeaderMode: device.HeaderMode, SpeedProfileDisabled: motherboardHeaderIsBIOS(device)}
		for id, label := range device.OperatingModes {
			if label != "" {
				channel.OperatingModeOptions = append(channel.OperatingModeOptions, coolingpresentation.OperatingModeOption{ID: id, Label: label})
			}
		}
		sort.Slice(channel.OperatingModeOptions, func(i, j int) bool { return channel.OperatingModeOptions[i].ID < channel.OperatingModeOptions[j].ID })
		snapshot.Channels = append(snapshot.Channels, channel)
	}
	sort.Slice(snapshot.Channels, func(i, j int) bool { return snapshot.Channels[i].SourceID < snapshot.Channels[j].SourceID })
	return snapshot, len(snapshot.Channels) > 0
}

func motherboardHeaderIsBIOS(device *Devices) bool {
	if device == nil {
		return false
	}
	for id, label := range device.OperatingModes {
		if id == device.HeaderMode && label == "BIOS" {
			return true
		}
	}
	return false
}

// OperatingModeOptions lets the existing route validate against the exact
// header options before delegating to UpdateOperatingMode.
func (d *Device) OperatingModeOptions(channelID int) map[int]string {
	if d == nil || d.Devices[channelID] == nil || len(d.Devices[channelID].OperatingModes) == 0 {
		return nil
	}
	options := make(map[int]string, len(d.Devices[channelID].OperatingModes))
	for id, label := range d.Devices[channelID].OperatingModes {
		if label != "" {
			options[id] = label
		}
	}
	return options
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
	snapshot := deviceprofilepresentation.Snapshot{Supported: true}
	active := 0
	for name, profile := range d.UserProfiles {
		if profile == nil {
			continue
		}
		snapshot.Profiles = append(snapshot.Profiles, name)
		if profile.Active {
			snapshot.ActiveProfile = name
			active++
		}
	}
	sort.Strings(snapshot.Profiles)
	snapshot = deviceprofilepresentation.WithMutationCapabilities(snapshot, d)
	return snapshot, active == 1 && snapshot.ActiveProfile != "" && len(snapshot.Profiles) > 0
}
