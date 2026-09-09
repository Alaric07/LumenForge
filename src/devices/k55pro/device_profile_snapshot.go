package k55pro

import (
	"LumenForge/src/deviceprofilepresentation"
	"sort"
	"strings"
)

func (d *Device) DeviceProfileDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || len(d.UserProfiles) == 0 || !containsk55Pro(d.DeviceProfile.Profiles, d.DeviceProfile.Profile) || d.DeviceProfile.Keyboards == nil || d.DeviceProfile.Keyboards[d.DeviceProfile.Profile] == nil {
		return deviceprofilepresentation.Snapshot{}, false
	}
	snapshot := deviceprofilepresentation.Snapshot{Supported: true}
	activeProfiles := 0
	for name, profile := range d.UserProfiles {
		if strings.TrimSpace(name) == "" || profile == nil {
			return deviceprofilepresentation.Snapshot{}, false
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
