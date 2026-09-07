package k70protkl

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestK70ProTKLNormalSnapshots(t *testing.T) {
	k := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A"}, 2: {KeyName: "Play", OnlyColor: true}}}}}
	d := &Device{UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 11: "Brightness +"}, PollingRates: map[int]string{4: "1000 Hz"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, PollingRate: 4}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	if s, ok := d.KeyboardAssignmentsSnapshot(); !ok || len(s.Rows) != 1 || s.LayoutClass != "keyboard-6" || s.RowLayoutClass != "keyboard-row-20" || len(s.AssignmentTypes) != 2 {
		t.Fatalf("keyboard=%#v", s)
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
