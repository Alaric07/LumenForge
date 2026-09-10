package k100

import (
	"LumenForge/src/controldialpresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/optioncolorpresentation"
	"LumenForge/src/performancepresentation"
	"LumenForge/src/rgb"
	"sort"
)

func (d *Device) KeyboardAssignmentsDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) KeyboardAssignmentsSnapshot() (keyboardassignmentspresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || len(d.Layouts) == 0 || d.UIKeyboard == "" || d.UIKeyboardRow == "" || len(d.KeyAssignmentTypes) == 0 || !k100Contains(d.Layouts, d.DeviceProfile.Layout) || !k100Contains(d.DeviceProfile.Profiles, d.DeviceProfile.Profile) {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	keyboard := d.getCurrentKeyboard()
	if keyboard == nil {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	snapshot, ok := keyboardassignmentspresentation.BuildSnapshot(keyboardassignmentspresentation.Source{Profiles: d.DeviceProfile.Profiles, ActiveProfile: d.DeviceProfile.Profile, KeyboardLayouts: d.Layouts, ActiveKeyboardLayout: d.DeviceProfile.Layout, ClusterControlled: d.DeviceProfile.RGBCluster, LayoutClass: d.UIKeyboard, RowLayoutClass: d.UIKeyboardRow, Rows: keyboard.Row, AssignmentTypes: d.KeyAssignmentTypes, OmitModifierOptions: true})
	if !ok {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	// UpdateDeviceKeyAssignment intentionally does not persist modifier fields.
	return snapshot, true
}
func (d *Device) PerformanceDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.PollingRates) == 0 {
		return performancepresentation.Snapshot{}, false
	}
	ids := make([]int, 0, len(d.PollingRates))
	for id := range d.PollingRates {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	polling := &performancepresentation.SelectSetting{Value: d.DeviceProfile.PollingRate}
	selected := false
	for _, id := range ids {
		if d.PollingRates[id] == "" {
			return performancepresentation.Snapshot{}, false
		}
		polling.Options = append(polling.Options, performancepresentation.Option{Value: id, Label: d.PollingRates[id]})
		selected = selected || id == polling.Value
	}
	if !selected {
		return performancepresentation.Snapshot{}, false
	}
	debounce := &performancepresentation.SelectSetting{Value: d.DeviceProfile.DebounceTime}
	debounceSelected := false
	for id := 1; id <= 9; id++ {
		label, ok := d.DebounceTimes[id]
		if !ok || label == "" {
			return performancepresentation.Snapshot{}, false
		}
		debounce.Options = append(debounce.Options, performancepresentation.Option{Value: id, Label: label})
		debounceSelected = debounceSelected || id == debounce.Value
	}
	if !debounceSelected {
		return performancepresentation.Snapshot{}, false
	}
	return performancepresentation.Snapshot{PollingRate: polling, DebounceTime: debounce, SaveBooleanSettings: true, BooleanSettings: []performancepresentation.BooleanSetting{{ID: "perf_winKey", Label: "Disable Win Key", Enabled: d.DeviceProfile.DisableWinKey}, {ID: "perf_shiftTab", Label: "Disable Shift + Tab", Enabled: d.DeviceProfile.DisableShiftTab}, {ID: "perf_altTab", Label: "Disable Alt + Tab", Enabled: d.DeviceProfile.DisableAltTab}, {ID: "perf_altF4", Label: "Disable Alt + F4", Enabled: d.DeviceProfile.DisableAltF4}}}, true
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
		if profile == nil {
			continue
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
	return s, controldialpresentation.Valid(s)
}
func (d *Device) OptionColorsDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) OptionColorsSnapshot() (optioncolorpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.ControlDialOptions) == 0 || len(d.DeviceProfile.ControlDialColors) != len(d.ControlDialOptions) {
		return optioncolorpresentation.Snapshot{}, false
	}
	ids := make([]int, 0, len(d.ControlDialOptions))
	for id := range d.ControlDialOptions {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	s := optioncolorpresentation.Snapshot{Selected: d.DeviceProfile.ControlDial}
	for _, id := range ids {
		color, ok := d.DeviceProfile.ControlDialColors[id]
		if !ok || color == nil || d.ControlDialOptions[id] == "" {
			return optioncolorpresentation.Snapshot{}, false
		}
		s.Options = append(s.Options, optioncolorpresentation.Option{Value: id, Label: d.ControlDialOptions[id], Color: rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue}, Action: "control-dial-colors"})
	}
	return s, k100ContainsInt(ids, s.Selected)
}
func k100Contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func k100ContainsInt(values []int, value int) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
