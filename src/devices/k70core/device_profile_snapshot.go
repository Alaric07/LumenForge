package k70core

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
	active := 0
	for name, p := range d.UserProfiles {
		if p == nil {
			continue
		}
		s.Profiles = append(s.Profiles, name)
		if p.Active {
			active++
			s.ActiveProfile = name
		}
	}
	sort.Strings(s.Profiles)
	if active != 1 || s.ActiveProfile == "" || len(s.Profiles) == 0 {
		return deviceprofilepresentation.Snapshot{}, false
	}
	s = deviceprofilepresentation.WithMutationCapabilities(s, d)
	return s, true
}
