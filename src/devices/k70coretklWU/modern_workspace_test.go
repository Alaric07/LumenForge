package k70coretklWU

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestK70CoreTKLUSBModernWorkspaceContract(t *testing.T) {
	d := &Device{Serial: "k70coretklwu", UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: k70CoreTKLWUTestAssignments(), ControlDialOptions: k70CoreTKLWUTestDial(), PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k70CoreTKLWUTestKeyboard()}, PollingRate: 4, ControlDial: 1, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	assignments, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || len(assignments.Rows) != 1 || len(assignments.ModifierOptions) != 2 || assignments.LiveRGBAvailable {
		t.Fatalf("assignments=%#v ok=%t", assignments, ok)
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || performance.PollingRate == nil || len(performance.PollingRate.Options) != 5 || len(performance.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v ok=%t", performance, ok)
	}
	dial, ok := d.ControlDialSnapshot()
	if !ok || len(dial.Options) != 7 {
		t.Fatalf("dial=%#v ok=%t", dial, ok)
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

func k70CoreTKLWUTestKeyboard() *keyboards.Keyboard {
	return &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", Width: 1, Height: 1}, 2: {KeyName: "Fn", KeyNameInternal: "Fn", Width: 1, Height: 1, Modifier: true}}}}}
}
func k70CoreTKLWUTestAssignments() map[int]string {
	return map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}
}
func k70CoreTKLWUTestDial() map[int]string {
	return map[int]string{1: "Volume Control", 2: "Brightness", 3: "Scroll", 4: "Zoom", 5: "Screen Brightness", 6: "Media Control", 7: "Horizontal Scroll"}
}
