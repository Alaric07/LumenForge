package k70coretkl

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestK70CoreTKLSnapshotsFailClosedAndExposePerformance(t *testing.T) {
	k := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A"}}}}}
	d := &Device{UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, PollingRates: map[int]string{4: "1000 Hz"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, PollingRate: 4}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	if _, ok := d.KeyboardAssignmentsSnapshot(); !ok {
		t.Fatal("missing keyboard")
	}
	if s, ok := d.PerformanceSnapshot(); !ok || len(s.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v", s)
	}
	if _, ok := d.DeviceProfileSnapshot(); !ok {
		t.Fatal("missing profile")
	}
	d.UserProfiles["Gaming"] = &DeviceProfile{Active: true}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("accepted multiple active profiles")
	}
}
