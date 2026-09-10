package strafergbmk2

import (
	"LumenForge/src/keyboards"
	"reflect"
	"testing"
)

func TestModernWorkspaceSnapshotsExposeOnlyStrafeRGBMK2Capabilities(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A"}, 2: {KeyName: "Play", OnlyColor: true}}}}}
	device := &Device{
		Serial:             "strafe-modern-workspace",
		UIKeyboard:         "keyboard-7",
		UIKeyboardRow:      "keyboard-row-25",
		Layouts:            []string{"US"},
		KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"},
		PollingRates:       map[int]string{0: "Not Set", 8: "125 Hz / 8 msec", 4: "250 Hz / 4 msec", 2: "500 Hz / 2 msec", 1: "1000 Hz / 1 msec"},
		DeviceProfile:      &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 1, DisableWinKey: true, DisableShiftTab: true, DisableAltTab: true, DisableAltF4: true},
		UserProfiles:       map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {}},
	}

	assignments, ok := device.KeyboardAssignmentsSnapshot()
	if !ok || assignments.LayoutClass != "keyboard-7" || assignments.RowLayoutClass != "keyboard-row-25" || len(assignments.Rows) != 1 || len(assignments.Rows[0].Keys) != 2 || assignments.Rows[0].Keys[1].Assignable || len(assignments.ModifierOptions) != 0 {
		t.Fatalf("assignments = %#v, ok=%t", assignments, ok)
	}
	if got, want := assignments.AssignmentTypes, []struct {
		ID    uint8
		Label string
	}{{0, "None"}, {1, "Media Keys"}, {3, "Keyboard"}, {8, "Sniper"}, {9, "Mouse"}, {10, "Macro"}, {13, "Scroll Up"}, {14, "Scroll Down"}, {15, "Zoom In"}, {16, "Zoom Out"}, {17, "Screen Brightness +"}, {18, "Screen Brightness -"}}; len(got) != len(want) {
		t.Fatalf("assignment types = %#v", got)
	} else {
		for index, expected := range want {
			if got[index].ID != expected.ID || got[index].Label != expected.Label {
				t.Fatalf("assignment type %d = %#v, want %#v", index, got[index], expected)
			}
		}
	}

	performance, ok := device.PerformanceSnapshot()
	if !ok || len(performance.BooleanSettings) != 4 || performance.PollingRate == nil {
		t.Fatalf("performance = %#v, ok=%t", performance, ok)
	}
	if got, want := performance.PollingRate.Options, []struct {
		Value int
		Label string
	}{{0, "Not Set"}, {1, "1000 Hz / 1 msec"}, {2, "500 Hz / 2 msec"}, {4, "250 Hz / 4 msec"}, {8, "125 Hz / 8 msec"}}; len(got) != len(want) {
		t.Fatalf("polling options = %#v", got)
	} else {
		for index, expected := range want {
			if got[index].Value != expected.Value || got[index].Label != expected.Label {
				t.Fatalf("polling option %d = %#v, want %#v", index, got[index], expected)
			}
		}
	}

	profiles, ok := device.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete || !reflect.DeepEqual(profiles.Profiles, []string{"Default", "Gaming"}) {
		t.Fatalf("profiles = %#v, ok=%t", profiles, ok)
	}
}

func TestModernWorkspaceSnapshotsFailClosed(t *testing.T) {
	device := &Device{Serial: "strafe-modern-workspace", UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": {Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A"}}}}}}}}
	if _, ok := device.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("accepted missing active keyboard profile")
	}
	device.PollingRates = map[int]string{1: ""}
	device.DeviceProfile.PollingRate = 1
	if _, ok := device.PerformanceSnapshot(); ok {
		t.Fatal("accepted blank polling option")
	}
	device.UserProfiles = map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {Active: true}}
	if _, ok := device.DeviceProfileSnapshot(); ok {
		t.Fatal("accepted multiple active device profiles")
	}
}
