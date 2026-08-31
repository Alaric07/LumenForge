package ccxt

import (
	"LumenForge/src/deviceprofilepresentation"
	"sort"
)

// DeviceProfileDeviceID and DeviceProfileSnapshot expose CCXT's existing full
// controller profiles to the shared Devices overview workspace.
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
