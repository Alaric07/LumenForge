package k70rgbtklcs

import (
	"LumenForge/src/keyboards"
	"reflect"
	"testing"
)

func TestK70RGBTKLCSModernWorkspaceContract(t *testing.T) {
	assignmentTypes := map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Brightness +", 12: "Brightness -", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}
	pollingRates := map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", Width: 65, Height: 70}, 2: {KeyName: "Fn", KeyNameInternal: "Fn", Width: 65, Height: 70, Modifier: true}}}}}
	device := &Device{Serial: "k70rgbtklcs", UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: assignmentTypes, PollingRates: pollingRates, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true, DisableShiftTab: true, DisableAltTab: true, DisableAltF4: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}

	assignments, ok := device.KeyboardAssignmentsSnapshot()
	if !ok || assignments.LayoutClass != "keyboard-7" || assignments.RowLayoutClass != "keyboard-row-20" || len(assignments.Rows) != 1 || len(assignments.ModifierOptions) != 2 {
		t.Fatalf("assignments = %#v, ok = %t", assignments, ok)
	}
	gotAssignmentTypes := map[int]string{}
	for _, assignmentType := range assignments.AssignmentTypes {
		gotAssignmentTypes[int(assignmentType.ID)] = assignmentType.Label
	}
	if !reflect.DeepEqual(gotAssignmentTypes, assignmentTypes) {
		t.Fatalf("assignment types = %#v", gotAssignmentTypes)
	}

	performance, ok := device.PerformanceSnapshot()
	if !ok || performance.PollingRate == nil || len(performance.PollingRate.Options) != 8 || len(performance.BooleanSettings) != 4 {
		t.Fatalf("performance = %#v, ok = %t", performance, ok)
	}
	for id, label := range pollingRates {
		if option := performance.PollingRate.Options[id]; option.Value != id || option.Label != label {
			t.Fatalf("polling option %d = %#v, want %q", id, option, label)
		}
	}

	profiles, ok := device.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete {
		t.Fatalf("profiles = %#v, ok = %t", profiles, ok)
	}

	device.DeviceProfile.PollingRate = 8
	if _, ok := device.PerformanceSnapshot(); ok {
		t.Fatal("accepted an unsupported polling rate")
	}
}
