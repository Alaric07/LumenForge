package k57rgbWU

import (
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/keyboards"
	"testing"
)

func TestK57RGBUSBModernWorkspaceContract(t *testing.T) {
	types := map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}
	d := &Device{Serial: "k57u", UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: types, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k57RGBWUKeyboardFixture()}, PollingRate: 4, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	a, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || len(a.Rows) != 7 || k57RGBWUCount(a.Rows) != 120 || len(a.AssignmentTypes) != len(types) || len(a.ModifierOptions) != 0 || a.LiveRGBAvailable {
		t.Fatalf("assignments=%#v ok=%t", a, ok)
	}
	if _, exists := types[11]; exists {
		t.Fatal("USB exposed brightness assignment")
	}
	p, ok := d.PerformanceSnapshot()
	if !ok || p.PollingRate == nil || p.PollingRate.Value != 4 || len(p.PollingRate.Options) != 5 || len(p.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v ok=%t", p, ok)
	}
	for i, id := range []int{0, 1, 2, 3, 4} {
		if p.PollingRate.Options[i].Value != id {
			t.Fatalf("polling=%#v", p.PollingRate.Options)
		}
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
}
func TestK57RGBUSBModernWorkspaceFailsClosed(t *testing.T) {
	k := k57RGBWUKeyboardFixture()
	d := &Device{UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, PollingRates: map[int]string{4: "1000 Hz"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, PollingRate: 4}}
	k.Row[1] = keyboards.Row{Keys: map[int]keyboards.Key{1: {KeyName: "Duplicate", Width: 1, Height: 1}}}
	if _, ok := d.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("accepted duplicate key")
	}
	delete(k.Row, 1)
	d.PollingRates[4] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("accepted blank polling option")
	}
	if _, ok := (&Device{DeviceProfile: d.DeviceProfile, UserProfiles: map[string]*DeviceProfile{"Default": {}, "Gaming": {Active: true}, "Other": {Active: true}}}).DeviceProfileSnapshot(); ok {
		t.Fatal("accepted invalid active profiles")
	}
}
func k57RGBWUKeyboardFixture() *keyboards.Keyboard {
	k := &keyboards.Keyboard{Row: make(map[int]keyboards.Row, 7)}
	id := 1
	for row, count := range []int{18, 17, 17, 16, 18, 18, 16} {
		keys := map[int]keyboards.Key{}
		for col := 0; col < count; col++ {
			keys[id] = keyboards.Key{KeyName: "Key", Width: 1, Height: 1}
			id++
		}
		k.Row[row] = keyboards.Row{Keys: keys}
	}
	return k
}
func k57RGBWUCount(rows []keyboardassignmentspresentation.Row) int {
	total := 0
	for _, row := range rows {
		total += len(row.Keys)
	}
	return total
}
