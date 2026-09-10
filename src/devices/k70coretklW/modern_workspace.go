package k70coretklW

import (
	"LumenForge/src/controldialpresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/performancepresentation"
	"LumenForge/src/sleeptimerpresentation"
	"sort"
	"strings"
)

func (d *Device) KeyboardAssignmentsDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) KeyboardAssignmentsSnapshot() (keyboardassignmentspresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || len(d.Layouts) == 0 || d.UIKeyboard == "" || d.UIKeyboardRow == "" || len(d.KeyAssignmentTypes) == 0 || !k70CoreTKLWContains(d.Layouts, d.DeviceProfile.Layout) || !k70CoreTKLWContains(d.DeviceProfile.Profiles, d.DeviceProfile.Profile) {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	k := d.getCurrentKeyboard()
	if k == nil {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	return keyboardassignmentspresentation.BuildSnapshot(keyboardassignmentspresentation.Source{Profiles: d.DeviceProfile.Profiles, ActiveProfile: d.DeviceProfile.Profile, KeyboardLayouts: d.Layouts, ActiveKeyboardLayout: d.DeviceProfile.Layout, LayoutClass: d.UIKeyboard, RowLayoutClass: d.UIKeyboardRow, Rows: k.Row, AssignmentTypes: d.KeyAssignmentTypes})
}

func (d *Device) PerformanceDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil {
		return performancepresentation.Snapshot{}, false
	}
	return performancepresentation.Snapshot{SaveBooleanSettings: true, BooleanSettings: []performancepresentation.BooleanSetting{{ID: "perf_winKey", Label: "Disable Win Key", Enabled: d.DeviceProfile.DisableWinKey}, {ID: "perf_shiftTab", Label: "Disable Shift + Tab", Enabled: d.DeviceProfile.DisableShiftTab}, {ID: "perf_altTab", Label: "Disable Alt + Tab", Enabled: d.DeviceProfile.DisableAltTab}, {ID: "perf_altF4", Label: "Disable Alt + F4", Enabled: d.DeviceProfile.DisableAltF4}}}, true
}

func (d *Device) SleepTimerDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) SleepTimerSnapshot() (sleeptimerpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.SleepModes) == 0 {
		return sleeptimerpresentation.Snapshot{}, false
	}
	ids := make([]int, 0, len(d.SleepModes))
	for id := range d.SleepModes {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	s := sleeptimerpresentation.Snapshot{Value: d.DeviceProfile.SleepMode}
	found := false
	for _, id := range ids {
		if strings.TrimSpace(d.SleepModes[id]) == "" {
			return sleeptimerpresentation.Snapshot{}, false
		}
		s.Options = append(s.Options, sleeptimerpresentation.Option{Value: id, Label: d.SleepModes[id]})
		found = found || id == s.Value
	}
	return s, found
}

func (d *Device) DeviceProfileDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || d.DeviceProfile.Keyboards[d.DeviceProfile.Profile] == nil || len(d.UserProfiles) == 0 {
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
	sort.Strings(s.Profiles)
	if active != 1 || s.ActiveProfile == "" {
		return deviceprofilepresentation.Snapshot{}, false
	}
	return deviceprofilepresentation.WithMutationCapabilities(s, d), true
}

func (d *Device) ControlDialDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) ControlDialSnapshot() (controldialpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.ControlDialOptions) == 0 {
		return controldialpresentation.Snapshot{}, false
	}
	s := controldialpresentation.Snapshot{Available: true, Value: d.DeviceProfile.ControlDial, Options: controldialpresentation.Sorted(d.ControlDialOptions)}
	for _, option := range s.Options {
		if strings.TrimSpace(option.Label) == "" {
			return controldialpresentation.Snapshot{}, false
		}
	}
	return s, controldialpresentation.Valid(s)
}

func k70CoreTKLWContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
