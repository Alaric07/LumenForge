package k95platinum

import (
	"LumenForge/src/deviceprofilepresentation"
	"sort"
)

// DeviceProfileDeviceID and DeviceProfileSnapshot make this device a thin
// provider for the shared full device-profile overview panel.
func (d *Device) DeviceProfileDeviceID() string {
	if d == nil || len(d.UserProfiles) == 0 {
		return ""
	}
	return d.Serial
}

func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil || len(d.UserProfiles) == 0 {
		return deviceprofilepresentation.Snapshot{}, false
	}
	snapshot := deviceprofilepresentation.Snapshot{Supported: true}
	activeProfiles := 0
	for name, profile := range d.UserProfiles {
		if profile == nil {
			continue
		}
		snapshot.Profiles = append(snapshot.Profiles, name)
		if profile.Active {
			activeProfiles++
			snapshot.ActiveProfile = name
		}
	}
	sort.Strings(snapshot.Profiles)
	if activeProfiles != 1 || snapshot.ActiveProfile == "" || len(snapshot.Profiles) == 0 {
		return deviceprofilepresentation.Snapshot{}, false
	}
	snapshot = deviceprofilepresentation.WithMutationCapabilities(snapshot, d)
	return snapshot, true
}
