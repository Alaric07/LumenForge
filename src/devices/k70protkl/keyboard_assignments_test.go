package k70protkl

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestK70ProTKLNormalSnapshots(t *testing.T) {
	k := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", ModifierKey: 3, RetainOriginal: true}, 2: {KeyName: "Play", OnlyColor: true}, 3: {KeyName: "Fn", KeyNameInternal: "Function", Modifier: true}}}}}
	d := &Device{UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 11: "Brightness +"}, PollingRates: map[int]string{4: "1000 Hz"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, PollingRate: 4}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	if s, ok := d.KeyboardAssignmentsSnapshot(); !ok || len(s.Rows) != 1 || s.LayoutClass != "keyboard-6" || s.RowLayoutClass != "keyboard-row-20" || len(s.AssignmentTypes) != 2 {
		t.Fatalf("keyboard=%#v", s)
	} else if len(s.ModifierOptions) != 2 || s.ModifierOptions[1].ID != 3 || s.ModifierOptions[1].Label != "Function" || s.Rows[0].Keys[0].ModifierKey != 3 || !s.Rows[0].Keys[0].RetainOriginal {
		t.Fatalf("modifier presentation=%#v", s)
	}
	if s, ok := d.PerformanceSnapshot(); !ok || len(s.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v", s)
	}
	if _, ok := d.DeviceProfileSnapshot(); !ok {
		t.Fatal("profile")
	}
	d.PollingRates[4] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("blank option")
	}
	d.UserProfiles["Gaming"] = &DeviceProfile{Active: true}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("multiple active")
	}
}

func TestK70ProTKLKeyboardAssignmentsRejectInvalidModifierState(t *testing.T) {
	k := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", ModifierKey: 4}, 3: {KeyName: "Fn", Modifier: true}}}}}
	d := &Device{UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}}}
	if _, ok := d.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("missing selected modifier must fail closed")
	}
	k.Row[0].Keys[3] = keyboards.Key{Modifier: true}
	if _, ok := d.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("blank modifier label must fail closed")
	}
}
