package voideliteW

import (
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/headsetpresentation"
	"sort"
)

func (d *Device) DeviceProfileDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) HeadsetDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.UserProfiles) == 0 {
		return deviceprofilepresentation.Snapshot{}, false
	}
	s := deviceprofilepresentation.Snapshot{Supported: true}
	active := 0
	for name, profile := range d.UserProfiles {
		if name == "" || profile == nil {
			return deviceprofilepresentation.Snapshot{}, false
		}
		s.Profiles = append(s.Profiles, name)
		if profile.Active {
			active++
			s.ActiveProfile = name
		}
	}
	if active != 1 {
		return deviceprofilepresentation.Snapshot{}, false
	}
	sort.Strings(s.Profiles)
	return deviceprofilepresentation.WithMutationCapabilities(s, d), true
}

func (d *Device) HeadsetSnapshot() (headsetpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.DeviceProfile.Equalizers) != 10 || d.DeviceProfile.SideTone < 0 || d.DeviceProfile.SideTone > 1 || d.DeviceProfile.SideToneValue < 1 || d.DeviceProfile.SideToneValue > 100 {
		return headsetpresentation.Snapshot{}, false
	}
	labels := []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"}
	s := headsetpresentation.Snapshot{Equalizer: make([]headsetpresentation.EqualizerBand, 0, len(labels))}
	for index, label := range labels {
		band, ok := d.DeviceProfile.Equalizers[index+1]
		if !ok || band.Name != label || band.Value < -12 || band.Value > 12 {
			return headsetpresentation.Snapshot{}, false
		}
		s.Equalizer = append(s.Equalizer, headsetpresentation.EqualizerBand{ID: index + 1, Label: label, Value: band.Value})
	}
	s.Sidetone = &headsetpresentation.SidetoneSetting{Value: d.DeviceProfile.SideTone, Options: []headsetpresentation.SelectOption{{Value: 0, Label: "Disabled"}, {Value: 1, Label: "Enabled"}}, ValueRange: &headsetpresentation.RangedSetting{Value: d.DeviceProfile.SideToneValue, Minimum: 1, Maximum: 100, Step: 1}}
	return s, headsetpresentation.Valid(s)
}
