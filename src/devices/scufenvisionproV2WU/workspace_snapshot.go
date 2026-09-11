package scufenvisionproV2WU

import (
	"LumenForge/src/controllerpresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/sleeptimerpresentation"
	"sort"
)

func (d *Device) DeviceProfileDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) ControllerDeviceID() string {
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

func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.UserProfiles) == 0 {
		return deviceprofilepresentation.Snapshot{}, false
	}
	s := deviceprofilepresentation.Snapshot{Supported: true}
	for name, profile := range d.UserProfiles {
		if name == "" || profile == nil {
			return deviceprofilepresentation.Snapshot{}, false
		}
		s.Profiles = append(s.Profiles, name)
		if profile.Active {
			if s.ActiveProfile != "" {
				return deviceprofilepresentation.Snapshot{}, false
			}
			s.ActiveProfile = name
		}
	}
	if s.ActiveProfile == "" {
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
	s := sleeptimerpresentation.Snapshot{Value: d.DeviceProfile.SleepMode}
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

func (d *Device) ControllerSnapshot() (controllerpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil {
		return controllerpresentation.Snapshot{}, false
	}
	assignments := make(map[int]controllerpresentation.Assignment, len(d.KeyAssignment))
	for index, value := range d.KeyAssignment {
		assignments[index] = controllerpresentation.Assignment{Index: index, Label: value.Name, Default: value.Default, ActionHold: value.ActionHold, IsMacro: value.IsMacro, OnRelease: value.OnRelease, ActionType: value.ActionType, ActionCommand: value.ActionCommand}
	}
	modes := make([]controllerpresentation.AssignmentType, 3)
	for id := 0; id < 3; id++ {
		label, ok := d.ThumbStickModes[id]
		if !ok {
			return controllerpresentation.Snapshot{}, false
		}
		modes[id] = controllerpresentation.AssignmentType{ID: uint8(id), Label: label}
	}
	sticks := [2]controllerpresentation.Thumbstick{{Module: 0, Label: "Left Thumbstick", Mode: d.DeviceProfile.LeftThumbStickMode, SensitivityX: d.DeviceProfile.LeftThumbStickSensitivityX, SensitivityY: d.DeviceProfile.LeftThumbStickSensitivityY, InvertY: d.DeviceProfile.LeftThumbStickInvertY, Modes: modes}, {Module: 1, Label: "Right Thumbstick", Mode: d.DeviceProfile.RightThumbStickMode, SensitivityX: d.DeviceProfile.RightThumbStickSensitivityX, SensitivityY: d.DeviceProfile.RightThumbStickSensitivityY, InvertY: d.DeviceProfile.RightThumbStickInvertY, Modes: modes}}
	analogs := make(map[int]controllerpresentation.AnalogData, len(d.DeviceProfile.AnalogData))
	for id, value := range d.DeviceProfile.AnalogData {
		analogs[id] = controllerpresentation.AnalogData{DeadZoneMin: value.DeadZoneMin, DeadZoneMax: value.DeadZoneMax, Points: value.Points}
	}
	return controllerpresentation.Build(assignments, d.KeyAssignmentTypes, [2]uint8{d.DeviceProfile.LeftVibrationValue, d.DeviceProfile.RightVibrationValue}, sticks, analogs)
}
