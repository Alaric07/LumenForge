package k70pro

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestK70ProSnapshotsFailClosedAndExposePerformance(t *testing.T) {
	k := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", ActionType: 11}}}}}
	d := &Device{UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{11: "Brightness +"}, PollingRates: map[int]string{4: "1000 Hz"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, PollingRate: 4}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	if s, ok := d.KeyboardAssignmentsSnapshot(); !ok || s.Rows[0].Keys[0].ActionType != 11 {
		t.Fatalf("keyboard=%#v", s)
	}
	if s, ok := d.PerformanceSnapshot(); !ok || len(s.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v", s)
	}
	if _, ok := d.DeviceProfileSnapshot(); !ok {
		t.Fatal("missing profile")
	}
	d.PollingRates[4] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("accepted blank option")
	}
}
