package makr75WU

import (
	"LumenForge/src/controldialpresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/optioncolorpresentation"
	"LumenForge/src/performancepresentation"
	"LumenForge/src/rgb"
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
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || len(d.Layouts) == 0 || d.UIKeyboard == "" || d.UIKeyboardRow == "" || len(d.KeyAssignmentTypes) == 0 || !makr75WUContains(d.Layouts, d.DeviceProfile.Layout) || !makr75WUContains(d.DeviceProfile.Profiles, d.DeviceProfile.Profile) {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	keyboard := d.getCurrentKeyboard()
	if keyboard == nil {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	return keyboardassignmentspresentation.BuildSnapshot(keyboardassignmentspresentation.Source{Profiles: d.DeviceProfile.Profiles, ActiveProfile: d.DeviceProfile.Profile, KeyboardLayouts: d.Layouts, ActiveKeyboardLayout: d.DeviceProfile.Layout, ClusterControlled: d.DeviceProfile.RGBCluster, LayoutClass: d.UIKeyboard, RowLayoutClass: d.UIKeyboardRow, Rows: keyboard.Row, AssignmentTypes: d.KeyAssignmentTypes})
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
	polling, selected := &performancepresentation.SelectSetting{Value: d.DeviceProfile.PollingRate}, false
	for _, id := range ids {
		if strings.TrimSpace(d.PollingRates[id]) == "" {
			return performancepresentation.Snapshot{}, false
		}
		polling.Options = append(polling.Options, performancepresentation.Option{Value: id, Label: d.PollingRates[id]})
		selected = selected || id == polling.Value
	}
	if !selected {
		return performancepresentation.Snapshot{}, false
	}
	return performancepresentation.Snapshot{PollingRate: polling, SaveBooleanSettings: true, BooleanSettings: []performancepresentation.BooleanSetting{{ID: "perf_winKey", Label: "Disable Win Key", Enabled: d.DeviceProfile.DisableWinKey}, {ID: "perf_shiftTab", Label: "Disable Shift + Tab", Enabled: d.DeviceProfile.DisableShiftTab}, {ID: "perf_altTab", Label: "Disable Alt + Tab", Enabled: d.DeviceProfile.DisableAltTab}, {ID: "perf_altF4", Label: "Disable Alt + F4", Enabled: d.DeviceProfile.DisableAltF4}}}, true
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
	snapshot, active := deviceprofilepresentation.Snapshot{Supported: true}, 0
	for name, profile := range d.UserProfiles {
		if name == "" || profile == nil {
			return deviceprofilepresentation.Snapshot{}, false
		}
		snapshot.Profiles = append(snapshot.Profiles, name)
		if profile.Active {
			active++
			snapshot.ActiveProfile = name
		}
	}
	sort.Strings(snapshot.Profiles)
	if active != 1 || snapshot.ActiveProfile == "" {
		return deviceprofilepresentation.Snapshot{}, false
	}
	return deviceprofilepresentation.WithMutationCapabilities(snapshot, d), true
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
	snapshot := controldialpresentation.Snapshot{Available: true, Value: d.DeviceProfile.ControlDial, Options: controldialpresentation.Sorted(d.ControlDialOptions)}
	for _, option := range snapshot.Options {
		if strings.TrimSpace(option.Label) == "" {
			return controldialpresentation.Snapshot{}, false
		}
	}
	return snapshot, controldialpresentation.Valid(snapshot)
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
	snapshot := optioncolorpresentation.Snapshot{Selected: d.DeviceProfile.ControlDial}
	for _, id := range ids {
		color, ok := d.DeviceProfile.ControlDialColors[id]
		if !ok || color == nil || strings.TrimSpace(d.ControlDialOptions[id]) == "" {
			return optioncolorpresentation.Snapshot{}, false
		}
		snapshot.Options = append(snapshot.Options, optioncolorpresentation.Option{Value: id, Label: d.ControlDialOptions[id], Color: rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue}, Action: "control-dial-colors"})
	}
	return snapshot, makr75WUContainsInt(ids, snapshot.Selected)
}

func makr75WUContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
func makr75WUContainsInt(values []int, value int) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
