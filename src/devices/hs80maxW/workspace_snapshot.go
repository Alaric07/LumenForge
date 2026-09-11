package hs80maxW

import (
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/headsetpresentation"
	"LumenForge/src/sleeptimerpresentation"
	"sort"
)

func (d *Device) DeviceProfileDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) SleepTimerDeviceID() string {
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

func (d *Device) SleepTimerSnapshot() (sleeptimerpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.SleepModes) != 7 {
		return sleeptimerpresentation.Snapshot{}, false
	}
	keys := []int{0, 1, 5, 10, 15, 30, 60}
	s := sleeptimerpresentation.Snapshot{Value: d.GetSleepMode()}
	for _, key := range keys {
		label, ok := d.SleepModes[key]
		if !ok || label == "" {
			return sleeptimerpresentation.Snapshot{}, false
		}
		s.Options = append(s.Options, sleeptimerpresentation.Option{Value: key, Label: label})
	}
	for _, option := range s.Options {
		if option.Value == s.Value {
			return s, true
		}
	}
	return sleeptimerpresentation.Snapshot{}, false
}

func (d *Device) HeadsetSnapshot() (headsetpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.DeviceProfile.Equalizers) != 10 || len(d.MuteIndicators) != 2 || len(d.SideToneModes) != 2 || len(d.KeyAssignment) != 1 || len(d.KeyAssignmentTypes) != 6 {
		return headsetpresentation.Snapshot{}, false
	}
	labels := []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"}
	s := headsetpresentation.Snapshot{Equalizer: make([]headsetpresentation.EqualizerBand, 0, 10)}
	for i, label := range labels {
		band, ok := d.DeviceProfile.Equalizers[i+1]
		if !ok || band.Name != label {
			return headsetpresentation.Snapshot{}, false
		}
		s.Equalizer = append(s.Equalizer, headsetpresentation.EqualizerBand{ID: i + 1, Label: label, Value: band.Value})
	}
	selectSetting := func(value int, options map[int]string) *headsetpresentation.SelectSetting {
		keys := make([]int, 0, len(options))
		for key := range options {
			keys = append(keys, key)
		}
		sort.Ints(keys)
		out := &headsetpresentation.SelectSetting{Value: value}
		for _, key := range keys {
			if options[key] == "" {
				return nil
			}
			out.Options = append(out.Options, headsetpresentation.SelectOption{Value: key, Label: options[key]})
		}
		return out
	}
	if s.MuteIndicator = selectSetting(d.DeviceProfile.DisableMicIndicator, d.MuteIndicators); s.MuteIndicator == nil {
		return headsetpresentation.Snapshot{}, false
	}
	side := selectSetting(d.DeviceProfile.SideTone, d.SideToneModes)
	if side == nil {
		return headsetpresentation.Snapshot{}, false
	}
	s.Sidetone = &headsetpresentation.SidetoneSetting{Value: side.Value, Options: side.Options, ValueRange: &headsetpresentation.RangedSetting{Value: d.DeviceProfile.SideToneValue, Minimum: 0, Maximum: 100, Step: 1}}
	assignment, ok := d.KeyAssignment[1]
	if !ok || assignment.Name != "Scroll Press" {
		return headsetpresentation.Snapshot{}, false
	}
	types := make([]headsetpresentation.AssignmentType, 0, len(d.KeyAssignmentTypes))
	keys := make([]int, 0, len(d.KeyAssignmentTypes))
	for key := range d.KeyAssignmentTypes {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	for _, key := range keys {
		if d.KeyAssignmentTypes[key] == "" {
			return headsetpresentation.Snapshot{}, false
		}
		types = append(types, headsetpresentation.AssignmentType{ID: uint8(key), Label: d.KeyAssignmentTypes[key]})
	}
	s.Assignments = []headsetpresentation.Assignment{{ID: 1, Label: assignment.Name, Default: assignment.Default, ActionHold: assignment.ActionHold, IsMacro: assignment.IsMacro, OnRelease: assignment.OnRelease, ActionType: assignment.ActionType, ActionCommand: assignment.ActionCommand, Types: types}}
	muted := d.MuteStatus == 1
	s.Muted = &muted
	return s, headsetpresentation.Valid(s)
}
