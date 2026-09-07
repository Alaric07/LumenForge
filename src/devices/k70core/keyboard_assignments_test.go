package k70core

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestK70CoreSnapshotsFailClosedAndExposePerformance(t *testing.T) {
	k := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A"}}}}}
	d := &Device{UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, PollingRates: map[int]string{4: "1000 Hz"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, PollingRate: 4}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	if s, ok := d.KeyboardAssignmentsSnapshot(); !ok || len(s.Rows) != 1 {
		t.Fatalf("keyboard=%#v ok=%t", s, ok)
	}
	if s, ok := d.PerformanceSnapshot(); !ok || len(s.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v ok=%t", s, ok)
	}
	if s, ok := d.DeviceProfileSnapshot(); !ok || s.ActiveProfile != "Default" {
		t.Fatalf("profile=%#v ok=%t", s, ok)
	}
	d.DeviceProfile.Profiles = nil
	if _, ok := d.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("accepted missing membership")
	}
}
