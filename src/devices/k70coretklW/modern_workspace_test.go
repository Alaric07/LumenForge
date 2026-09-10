package k70coretklW

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestK70CoreTKLWirelessModernWorkspaceContract(t *testing.T) {
	d := &Device{Serial: "k70coretklw", UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: k70CoreTKLTestAssignments(), ControlDialOptions: k70CoreTKLTestDial(), SleepModes: map[int]string{1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k70CoreTKLTestKeyboard()}, SleepMode: 15, ControlDial: 1, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	assignments, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || len(assignments.Rows) != 1 || len(assignments.ModifierOptions) != 2 || assignments.LiveRGBAvailable {
		t.Fatalf("assignments=%#v ok=%t", assignments, ok)
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || performance.PollingRate != nil || len(performance.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v ok=%t", performance, ok)
	}
	sleep, ok := d.SleepTimerSnapshot()
	if !ok || sleep.Value != 15 || len(sleep.Options) != 6 {
		t.Fatalf("sleep=%#v ok=%t", sleep, ok)
	}
	dial, ok := d.ControlDialSnapshot()
	if !ok || len(dial.Options) != 7 {
		t.Fatalf("dial=%#v ok=%t", dial, ok)
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

func k70CoreTKLTestKeyboard() *keyboards.Keyboard {
	return &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", Width: 1, Height: 1}, 2: {KeyName: "Fn", KeyNameInternal: "Fn", Width: 1, Height: 1, Modifier: true}}}}}
}
func k70CoreTKLTestAssignments() map[int]string {
	return map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}
}
func k70CoreTKLTestDial() map[int]string {
	return map[int]string{1: "Volume Control", 2: "Brightness", 3: "Scroll", 4: "Zoom", 5: "Screen Brightness", 6: "Media Control", 7: "Horizontal Scroll"}
}
