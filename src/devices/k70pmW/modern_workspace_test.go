package k70pmW

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestK70ProMiniWirelessModernWorkspaceContract(t *testing.T) {
	d := &Device{Serial: "k70pmw", UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-18", Layouts: []string{"US"}, KeyAssignmentTypes: k70PMTestAssignments(), SleepModes: map[int]string{1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k70PMTestKeyboard()}, SleepMode: 15, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	a, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || len(a.Rows) != 1 || len(a.ModifierOptions) != 2 || a.LiveRGBAvailable {
		t.Fatalf("assignments=%#v ok=%t", a, ok)
	}
	p, ok := d.PerformanceSnapshot()
	if !ok || p.PollingRate != nil || len(p.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v ok=%t", p, ok)
	}
	s, ok := d.SleepTimerSnapshot()
	if !ok || s.Value != 15 || len(s.Options) != 6 {
		t.Fatalf("sleep=%#v ok=%t", s, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
	d.SleepModes[15] = ""
	if _, ok := d.SleepTimerSnapshot(); ok {
		t.Fatal("accepted blank sleep option")
	}
}
func k70PMTestKeyboard() *keyboards.Keyboard {
	return &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", Width: 1, Height: 1}, 2: {KeyName: "Fn", KeyNameInternal: "Fn", Width: 1, Height: 1, Modifier: true}}}}}
}
func k70PMTestAssignments() map[int]string {
	return map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}
}
