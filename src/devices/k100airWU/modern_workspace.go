package k100airWU

import (
	"LumenForge/src/booleansettingpresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/performancepresentation"
	"sort"
)

func (d *Device) KeyboardAssignmentsDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) KeyboardAssignmentsSnapshot() (keyboardassignmentspresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || len(d.Layouts) == 0 || d.UIKeyboard == "" || d.UIKeyboardRow == "" || len(d.KeyAssignmentTypes) == 0 || !k100AirWUContains(d.Layouts, d.DeviceProfile.Layout) || !k100AirWUContains(d.DeviceProfile.Profiles, d.DeviceProfile.Profile) {
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
	if active != 1 || s.ActiveProfile == "" || len(s.Profiles) == 0 {
		return deviceprofilepresentation.Snapshot{}, false
	}
	return deviceprofilepresentation.WithMutationCapabilities(s, d), true
}
func (d *Device) BooleanSettingsDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) BooleanSettingsSnapshot() (booleansettingpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil {
		return booleansettingpresentation.Snapshot{}, false
	}
	return booleansettingpresentation.Snapshot{Settings: []booleansettingpresentation.Setting{{ID: "auto-brightness", Label: "Auto Brightness", Value: d.DeviceProfile.AutoBrightness == 1, Action: "auto-brightness", Description: "Adjust brightness automatically based on ambient light."}}}, true
}
func (d *Device) UpdateBooleanSetting(action string, value bool) uint8 {
	if d == nil || action != "auto-brightness" {
		return 0
	}
	enabled := 0
	if value {
		enabled = 1
	}
	return d.UpdateAutoBrightness(enabled)
}
func k100AirWUContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
