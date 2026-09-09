package k60rgbpro

import (
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/keyboards"
	"testing"
)

func TestK60RGBProModernWorkspaceUsesSourceContracts(t *testing.T) {
	keyboard := k60RGBProKeyboardFixture([]int{18, 17, 17, 16, 18, 18})
	assignmentTypes := map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}
	d := &Device{Serial: "k60", UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: assignmentTypes, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	assignments, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || assignments.LayoutClass != "keyboard-6" || assignments.RowLayoutClass != "keyboard-row-26" || len(assignments.Rows) != 6 || len(assignments.AssignmentTypes) != len(assignmentTypes) || len(assignments.ModifierOptions) != 0 || assignments.LiveRGBAvailable {
		t.Fatalf("assignments=%#v ok=%t", assignments, ok)
	}
	if keys := keyboardAssignmentKeyCount(assignments.Rows); keys != 104 {
		t.Fatalf("key count=%d, want 104", keys)
	}
	for _, option := range assignments.AssignmentTypes {
		if assignmentTypes[int(option.ID)] != option.Label {
			t.Fatalf("assignment option=%#v", option)
		}
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || performance.PollingRate == nil || performance.PollingRate.Value != 4 || len(performance.PollingRate.Options) != 5 || len(performance.BooleanSettings) != 4 || !performance.BooleanSettings[0].Enabled {
		t.Fatalf("performance=%#v ok=%t", performance, ok)
	}
	for value, label := range d.PollingRates {
		found := false
		for _, option := range performance.PollingRate.Options {
			if option.Value == value && option.Label == label {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing polling option %d=%q", value, label)
		}
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || profiles.ActiveProfile != "Default" || len(profiles.Profiles) != 1 {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
}

func TestK60RGBProModernWorkspaceFailsClosed(t *testing.T) {
	keyboard := k60RGBProKeyboardFixture([]int{1})
	base := &Device{UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, PollingRates: map[int]string{4: "1000 Hz"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4}}
	keyboard.Row[1] = keyboards.Row{Keys: map[int]keyboards.Key{1: {KeyName: "Duplicate"}}}
	if snapshot, ok := base.KeyboardAssignmentsSnapshot(); ok || snapshot.Available {
		t.Fatalf("duplicate key snapshot=%#v ok=%t", snapshot, ok)
	}
	delete(keyboard.Row, 1)
	base.DeviceProfile.Layout = "DE"
	if _, ok := base.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("accepted malformed layout")
	}
	base.DeviceProfile.Layout = "US"
	base.DeviceProfile.Profiles = []string{"Gaming"}
	if _, ok := base.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("accepted malformed keyboard profile")
	}
	base.DeviceProfile.Profiles = []string{"Default"}
	base.PollingRates[4] = ""
	if _, ok := base.PerformanceSnapshot(); ok {
		t.Fatal("accepted blank polling label")
	}
	for _, profiles := range []map[string]*DeviceProfile{{"Default": {}}, {"Default": {Active: true}, "Gaming": {Active: true}}} {
		if _, ok := (&Device{DeviceProfile: base.DeviceProfile, UserProfiles: profiles}).DeviceProfileSnapshot(); ok {
			t.Fatal("accepted malformed active profile state")
		}
	}
}

func k60RGBProKeyboardFixture(rowCounts []int) *keyboards.Keyboard {
	keyboard := &keyboards.Keyboard{Row: make(map[int]keyboards.Row, len(rowCounts))}
	keyID := 1
	for rowID, count := range rowCounts {
		keys := make(map[int]keyboards.Key, count)
		for column := 0; column < count; column++ {
			keys[keyID] = keyboards.Key{KeyName: "Key", Width: 1, Height: 1}
			keyID++
		}
		keyboard.Row[rowID] = keyboards.Row{Keys: keys}
	}
	return keyboard
}

func keyboardAssignmentKeyCount(rows []keyboardassignmentspresentation.Row) int {
	count := 0
	for _, row := range rows {
		count += len(row.Keys)
	}
	return count
}
