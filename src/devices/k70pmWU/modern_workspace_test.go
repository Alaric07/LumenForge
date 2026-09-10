package k70pmWU

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestK70ProMiniUSBModernWorkspaceContract(t *testing.T) {
	d := &Device{Serial: "k70pmwu", UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-18", Layouts: []string{"US"}, KeyAssignmentTypes: k70PMWUTestAssignments(), PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k70PMWUTestKeyboard()}, PollingRate: 4, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	a, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || len(a.Rows) != 1 || len(a.ModifierOptions) != 2 || a.LiveRGBAvailable {
		t.Fatalf("assignments=%#v ok=%t", a, ok)
	}
	p, ok := d.PerformanceSnapshot()
	if !ok || p.PollingRate == nil || len(p.PollingRate.Options) != 8 || len(p.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v ok=%t", p, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
	d.DeviceProfile.PollingRate = 99
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("accepted unsupported polling rate")
	}
}
func k70PMWUTestKeyboard() *keyboards.Keyboard {
	return &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", Width: 1, Height: 1}, 2: {KeyName: "Fn", KeyNameInternal: "Fn", Width: 1, Height: 1, Modifier: true}}}}}
}
func k70PMWUTestAssignments() map[int]string {
	return map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}
}
