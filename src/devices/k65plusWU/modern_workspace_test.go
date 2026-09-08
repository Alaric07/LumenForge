package k65plusWU

import (
	"LumenForge/src/keyboards"
	"reflect"
	"testing"
)

func TestK65PlusUSBWorkspaceProviders(t *testing.T) {
	device := k65PlusUSBWorkspaceDevice()
	assignments, ok := device.KeyboardAssignmentsSnapshot()
	if !ok || assignments.LayoutClass != "keyboard-6" || assignments.RowLayoutClass != "keyboard-row-17" || len(assignments.Rows) != 6 || len(assignments.AssignmentTypes) != 6 || len(assignments.ModifierOptions) != 2 {
		t.Fatalf("assignments=%#v", assignments)
	}
	if assignments.ModifierOptions[0].ID != 0 || assignments.ModifierOptions[0].Label != "None" || assignments.ModifierOptions[1].ID != 1 || assignments.ModifierOptions[1].Label != "Function" || assignments.Rows[1].Keys[0].ModifierKey != 1 || !assignments.Rows[1].Keys[0].RetainOriginal {
		t.Fatalf("modifier state=%#v", assignments)
	}
	keys := 0
	lastID := 0
	for _, row := range assignments.Rows {
		for _, key := range row.Keys {
			keys++
			if key.KeyIndex <= lastID {
				t.Fatalf("key indexes not stable: %#v", assignments.Rows)
			}
			lastID = key.KeyIndex
		}
	}
	if keys != 81 {
		t.Fatalf("keys=%d, want 81", keys)
	}
	if _, leaked := reflect.TypeOf(assignments.Rows[0].Keys[0]).FieldByName("KeyData"); leaked {
		t.Fatal("assignment snapshot leaked hardware KeyData")
	}
	performance, ok := device.PerformanceSnapshot()
	if !ok || performance.PollingRate != nil || len(performance.BooleanSettings) != 4 || !performance.BooleanSettings[0].Enabled || performance.BooleanSettings[1].ID != "perf_shiftTab" || performance.BooleanSettings[2].ID != "perf_altTab" || performance.BooleanSettings[3].ID != "perf_altF4" {
		t.Fatalf("performance=%#v", performance)
	}
	profiles, ok := device.DeviceProfileSnapshot()
	if !ok || profiles.ActiveProfile != "Default" || len(profiles.Profiles) != 1 {
		t.Fatalf("profiles=%#v", profiles)
	}
	dial, ok := device.ControlDialSnapshot()
	if !ok || dial.Value != 2 || len(dial.Options) != 2 || dial.Options[0].Value != 1 || dial.Options[0].Label != "Volume Control" || dial.Options[1].Value != 2 || dial.Options[1].Label != "Brightness" {
		t.Fatalf("dial=%#v", dial)
	}
}

func TestK65PlusUSBWorkspaceProvidersFailClosed(t *testing.T) {
	device := k65PlusUSBWorkspaceDevice()
	device.DeviceProfile.Keyboards["Default"].Row[1].Keys[15] = keyboards.Key{KeyName: "A", ModifierKey: 9}
	if _, ok := device.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("assignment snapshot accepted absent modifier option")
	}

	device = k65PlusUSBWorkspaceDevice()
	device.DeviceProfile.Keyboards["Default"].Row[6] = keyboards.Row{Keys: map[int]keyboards.Key{1: {KeyName: "Duplicate"}}}
	if _, ok := device.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("assignment snapshot accepted duplicate key index")
	}

	device = k65PlusUSBWorkspaceDevice()
	device.DeviceProfile.ControlDial = 3
	if _, ok := device.ControlDialSnapshot(); ok {
		t.Fatal("control dial snapshot accepted selected value absent from options")
	}

	device = k65PlusUSBWorkspaceDevice()
	device.UserProfiles["Default"].Active = false
	if _, ok := device.DeviceProfileSnapshot(); ok {
		t.Fatal("profile snapshot accepted zero active profiles")
	}
	device.UserProfiles["Gaming"] = &DeviceProfile{Active: true}
	device.UserProfiles["Default"].Active = true
	if _, ok := device.DeviceProfileSnapshot(); ok {
		t.Fatal("profile snapshot accepted multiple active profiles")
	}

	if _, ok := (&Device{}).PerformanceSnapshot(); ok {
		t.Fatal("performance snapshot accepted malformed state")
	}
}

func k65PlusUSBWorkspaceDevice() *Device {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{}}
	id := 1
	for row := 0; row < 6; row++ {
		keys := map[int]keyboards.Key{}
		for column := 0; column < 14 && id <= 81; column++ {
			keys[id] = keyboards.Key{KeyName: "Key"}
			id++
		}
		keyboard.Row[row] = keyboards.Row{Keys: keys}
	}
	keyboard.Row[0].Keys[1] = keyboards.Key{KeyName: "Fn", KeyNameInternal: "Function", Modifier: true}
	keyboard.Row[1].Keys[15] = keyboards.Key{KeyName: "A", ModifierKey: 1, RetainOriginal: true}
	return &Device{Serial: "k65-plus-usb", UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-17", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}, ControlDialOptions: map[int]string{1: "Volume Control", 2: "Brightness"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, ControlDial: 2, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
}
