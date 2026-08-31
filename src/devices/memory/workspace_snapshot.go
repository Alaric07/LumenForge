package memory

import (
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/memorypresentation"
	"sort"
)

// MemoryDeviceID and MemorySnapshot expose existing DIMM state to the modern
// Devices workspace without changing Memory's legacy lighting authority.
func (d *Device) MemoryDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) MemorySnapshot() (memorypresentation.Snapshot, bool) {
	if d == nil {
		return memorypresentation.Snapshot{}, false
	}

	snapshot := memorypresentation.Snapshot{Available: true}
	for _, module := range d.Devices {
		if module == nil {
			continue
		}
		temperature := ""
		if module.HasTemps && module.Temperature > 0 {
			temperature = module.TemperatureString
		}
		snapshot.Modules = append(snapshot.Modules, memorypresentation.Module{
			ChannelID: module.ChannelId, Name: module.Name, Label: module.Label,
			MemoryType: module.MemoryType, SKU: module.Sku, LEDCount: module.LedChannels,
			Temperature: temperature,
		})
	}
	sort.Slice(snapshot.Modules, func(i, j int) bool { return snapshot.Modules[i].ChannelID < snapshot.Modules[j].ChannelID })
	return snapshot, len(snapshot.Modules) > 0
}

// DeviceProfileDeviceID and DeviceProfileSnapshot adapt the existing complete
// Memory profile state for the shared Overview profile panel.
func (d *Device) DeviceProfileDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil {
		return deviceprofilepresentation.Snapshot{}, false
	}

	snapshot := deviceprofilepresentation.Snapshot{Supported: true}
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
	return snapshot, snapshot.ActiveProfile != "" && len(snapshot.Profiles) > 0
}
