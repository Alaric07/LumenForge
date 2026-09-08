package k55coretkl

import (
	"LumenForge/src/keyboards"
	"encoding/json"
	"strings"
	"testing"
)

func TestK55CoreTKLModernWorkspaceSnapshotsFailClosed(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", KeyData: []uint16{99}, HalfKey: true, HalfKeyEnd: true}}}}}
	assignments := map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}
	polling := map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec"}
	profile := &DeviceProfile{Profile: "default", Profiles: []string{"default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"default": keyboard}, PollingRate: 4}
	device := &Device{Serial: "test", UIKeyboard: "keyboard-test", UIKeyboardRow: "keyboard-row-test", Layouts: []string{"US"}, KeyAssignmentTypes: assignments, PollingRates: polling, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"default": {Active: true}}}

	snapshot, ok := device.KeyboardAssignmentsSnapshot()
	if !ok || len(snapshot.Rows) != 1 || len(snapshot.Rows[0].Keys) != 1 || len(snapshot.AssignmentTypes) != len(assignments) || len(snapshot.ModifierOptions) != 0 {
		t.Fatalf("assignments=%#v ok=%t", snapshot, ok)
	}
	if key := snapshot.Rows[0].Keys[0]; !key.HalfKey || key.HalfKeyStart || !key.HalfKeyEnd {
		t.Fatalf("half-key geometry = %#v", key)
	}
	for _, assignment := range snapshot.AssignmentTypes {
		if assignments[int(assignment.ID)] != assignment.Label {
			t.Fatalf("assignment type = %#v, want source map %#v", assignment, assignments)
		}
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil || strings.Contains(string(encoded), "keyData") {
		t.Fatalf("snapshot leaked KeyData or failed to encode: %s, %v", encoded, err)
	}
	if performance, ok := device.PerformanceSnapshot(); !ok || performance.PollingRate == nil || performance.PollingRate.Value != 4 || len(performance.PollingRate.Options) != len(polling) || len(performance.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v ok=%t", performance, ok)
	}
	if profiles, ok := device.DeviceProfileSnapshot(); !ok || profiles.ActiveProfile != "default" {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}

	keyboard.Row[1] = keyboards.Row{Keys: map[int]keyboards.Key{1: {KeyName: "Duplicate"}}}
	if _, ok := device.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("accepted duplicate KeyIndex")
	}
	delete(keyboard.Row, 1)
	keyboard.Row[0] = keyboards.Row{}
	if _, ok := device.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("accepted empty row")
	}
	keyboard.Row[0] = keyboards.Row{Keys: map[int]keyboards.Key{1: {KeyName: "A"}}}
	device.PollingRates[4] = ""
	if _, ok := device.PerformanceSnapshot(); ok {
		t.Fatal("accepted blank polling label")
	}
	device.PollingRates[4] = "1000 Hz / 1 msec"
	device.DeviceProfile.Keyboards["default"] = nil
	if _, ok := device.DeviceProfileSnapshot(); ok {
		t.Fatal("accepted missing active keyboard profile")
	}
	device.DeviceProfile.Keyboards["default"] = keyboard
	device.UserProfiles = map[string]*DeviceProfile{}
	if _, ok := device.DeviceProfileSnapshot(); ok {
		t.Fatal("accepted zero active device profiles")
	}
	device.UserProfiles = map[string]*DeviceProfile{"default": {Active: true}, "other": {Active: true}}
	if _, ok := device.DeviceProfileSnapshot(); ok {
		t.Fatal("accepted multiple active device profiles")
	}
}
