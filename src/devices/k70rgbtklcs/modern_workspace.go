package k70rgbtklcs

import (
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/performancepresentation"
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
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || len(d.Layouts) == 0 || d.UIKeyboard == "" || d.UIKeyboardRow == "" || len(d.KeyAssignmentTypes) == 0 || !k70RGBTKLCSContains(d.Layouts, d.DeviceProfile.Layout) || !k70RGBTKLCSContains(d.DeviceProfile.Profiles, d.DeviceProfile.Profile) {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	keyboard := d.getCurrentKeyboard()
	if keyboard == nil || len(keyboard.Row) == 0 {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	return keyboardassignmentspresentation.BuildSnapshot(keyboardassignmentspresentation.Source{
		Profiles:             d.DeviceProfile.Profiles,
		ActiveProfile:        d.DeviceProfile.Profile,
		KeyboardLayouts:      d.Layouts,
		ActiveKeyboardLayout: d.DeviceProfile.Layout,
		ClusterControlled:    d.DeviceProfile.RGBCluster,
		LayoutClass:          d.UIKeyboard,
		RowLayoutClass:       d.UIKeyboardRow,
		Rows:                 keyboard.Row,
		AssignmentTypes:      d.KeyAssignmentTypes,
	})
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
	if _, ok := d.PollingRates[d.DeviceProfile.PollingRate]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	ids := make([]int, 0, len(d.PollingRates))
	for id := range d.PollingRates {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	options := make([]performancepresentation.Option, 0, len(ids))
	for _, id := range ids {
		if strings.TrimSpace(d.PollingRates[id]) == "" {
			return performancepresentation.Snapshot{}, false
		}
		options = append(options, performancepresentation.Option{Value: id, Label: d.PollingRates[id]})
	}
	return performancepresentation.Snapshot{
		PollingRate:         &performancepresentation.SelectSetting{Value: d.DeviceProfile.PollingRate, Options: options},
		SaveBooleanSettings: true,
		BooleanSettings: []performancepresentation.BooleanSetting{
			{ID: "perf_winKey", Label: "Disable Win Key", Enabled: d.DeviceProfile.DisableWinKey},
			{ID: "perf_shiftTab", Label: "Disable Shift + Tab", Enabled: d.DeviceProfile.DisableShiftTab},
			{ID: "perf_altTab", Label: "Disable Alt + Tab", Enabled: d.DeviceProfile.DisableAltTab},
			{ID: "perf_altF4", Label: "Disable Alt + F4", Enabled: d.DeviceProfile.DisableAltF4},
		},
	}, true
}

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
	snapshot := deviceprofilepresentation.Snapshot{Supported: true}
	active := 0
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

func k70RGBTKLCSContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
