package lt100

import (
	"sort"

	"LumenForge/src/deviceprofilepresentation"
)

// DeviceProfileDeviceID and DeviceProfileSnapshot make this device a thin
// provider for the shared lighting-profile workspace panel.
func (d *Device) DeviceProfileDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil || len(d.UserProfiles) == 0 {
		return deviceprofilepresentation.Snapshot{}, false
	}

	snapshot := deviceprofilepresentation.Snapshot{
		Supported:                  true,
		Scope:                      deviceprofilepresentation.ScopeLighting,
		DefaultProfileDisplayLabel: deviceprofilepresentation.WorkingConfigurationLabel,
	}
	activeProfiles := 0
	for name, profile := range d.UserProfiles {
		if name == "" || profile == nil {
			return deviceprofilepresentation.Snapshot{}, false
		}
		snapshot.Profiles = append(snapshot.Profiles, name)
		if profile.Active {
			activeProfiles++
			snapshot.ActiveProfile = name
		}
	}
	if activeProfiles != 1 {
		return deviceprofilepresentation.Snapshot{}, false
	}

	sort.Strings(snapshot.Profiles)
	snapshot = deviceprofilepresentation.WithMutationCapabilities(snapshot, d)
	return snapshot, true
}
