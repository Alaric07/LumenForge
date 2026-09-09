package k70protkl

import (
	"LumenForge/src/deviceprofilepresentation"
	"sort"
)

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
	s := deviceprofilepresentation.Snapshot{Supported: true}
	n := 0
	for name, p := range d.UserProfiles {
		if p == nil || name == "" {
			return deviceprofilepresentation.Snapshot{}, false
		}
		s.Profiles = append(s.Profiles, name)
		if p.Active {
			n++
			s.ActiveProfile = name
		}
	}
	sort.Strings(s.Profiles)
	if n != 1 || s.ActiveProfile == "" {
		return deviceprofilepresentation.Snapshot{}, false
	}
	s = deviceprofilepresentation.WithMutationCapabilities(s, d)
	return s, true
}
