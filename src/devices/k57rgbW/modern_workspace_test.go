package k57rgbW

import (
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/keyboards"
	"testing"
)

func TestK57RGBWirelessModernWorkspaceContract(t *testing.T) {
	types := map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Brightness +", 12: "Brightness -", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}
	d := &Device{Serial: "k57w", UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: types, SleepModes: map[int]string{1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k57RGBWKeyboardFixture()}, SleepMode: 15, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	a, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || len(a.Rows) != 7 || k57RGBWCount(a.Rows) != 120 || len(a.AssignmentTypes) != len(types) || len(a.ModifierOptions) != 0 || a.LiveRGBAvailable {
		t.Fatalf("assignments=%#v ok=%t", a, ok)
	}
	for _, option := range a.AssignmentTypes {
		if types[int(option.ID)] != option.Label {
			t.Fatalf("assignment=%#v", option)
		}
	}
	key := d.DeviceProfile.Keyboards["Default"].Row[0].Keys[1]
	key.RetainOriginal = true
	d.DeviceProfile.Keyboards["Default"].Row[0].Keys[1] = key
	withRetainedState, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || !withRetainedState.Rows[0].Keys[0].RetainOriginal || len(withRetainedState.ModifierOptions) != 0 {
		t.Fatalf("modifier state=%#v ok=%t", withRetainedState, ok)
	}
	p, ok := d.PerformanceSnapshot()
	if !ok || p.PollingRate != nil || len(p.BooleanSettings) != 4 || !p.BooleanSettings[0].Enabled {
		t.Fatalf("performance=%#v ok=%t", p, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
	sleep, ok := d.SleepTimerSnapshot()
	if !ok || sleep.Value != 15 || len(sleep.Options) != 6 {
		t.Fatalf("sleep=%#v ok=%t", sleep, ok)
	}
}
func TestK57RGBWirelessModernWorkspaceFailsClosed(t *testing.T) {
	k := k57RGBWKeyboardFixture()
	d := &Device{UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, SleepModes: map[int]string{15: "15 minutes"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, SleepMode: 15}}
	k.Row[1] = keyboards.Row{Keys: map[int]keyboards.Key{1: {KeyName: "Duplicate", Width: 1, Height: 1}}}
	if _, ok := d.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("accepted duplicate key")
	}
	delete(k.Row, 1)
	d.DeviceProfile.Layout = "DE"
	if _, ok := d.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("accepted invalid layout")
	}
	d.DeviceProfile.Layout = "US"
	d.SleepModes[15] = ""
	if _, ok := d.SleepTimerSnapshot(); ok {
		t.Fatal("accepted blank sleep option")
	}
	if _, ok := (&Device{DeviceProfile: d.DeviceProfile, UserProfiles: map[string]*DeviceProfile{"Default": {}, "Gaming": {Active: true}, "Other": {Active: true}}}).DeviceProfileSnapshot(); ok {
		t.Fatal("accepted invalid active profiles")
	}
}
func k57RGBWKeyboardFixture() *keyboards.Keyboard {
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
func k57RGBWCount(rows []keyboardassignmentspresentation.Row) int {
	total := 0
	for _, row := range rows {
		total += len(row.Keys)
	}
	return total
}
