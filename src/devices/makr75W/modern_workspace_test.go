package makr75W

import (
	"LumenForge/src/config"
	"LumenForge/src/keyboards"
	"path/filepath"
	"testing"
)

func TestMAKR75WirelessModernWorkspaceContract(t *testing.T) {
	initializeMAKR75KeyboardFixtures(t)
	keyboard := keyboards.GetKeyboard("makr75-default-US")
	if keyboard == nil {
		t.Fatal("missing MAKR 75 shipped keyboard layout")
	}
	sleepModes := map[int]string{1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}
	dialModes := makr75TestDialModes()
	d := &Device{Serial: "makr75-wireless", UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-17", Layouts: []string{"US"}, KeyAssignmentTypes: makr75TestAssignments(), SleepModes: sleepModes, ControlDialOptions: dialModes, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, SleepMode: 15, ControlDial: 1, DisableWinKey: true, DisableShiftTab: true, DisableAltTab: true, DisableAltF4: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	assignments, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || assignments.LayoutClass != "keyboard-6" || assignments.RowLayoutClass != "keyboard-row-17" || len(assignments.Rows) == 0 || len(assignments.AssignmentTypes) != len(d.KeyAssignmentTypes) || len(assignments.ModifierOptions) < 2 || assignments.LiveRGBAvailable {
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
	for index, value := range []int{1, 5, 10, 15, 30, 60} {
		if sleep.Options[index].Value != value || sleep.Options[index].Label != sleepModes[value] {
			t.Fatalf("sleep=%#v", sleep)
		}
	}
	dial, ok := d.ControlDialSnapshot()
	if !ok || len(dial.Options) != 7 {
		t.Fatalf("dial=%#v ok=%t", dial, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
	if _, ok := interface{}(d).(interface{ OptionColorsSnapshot() }); ok {
		t.Fatal("wireless MAKR 75 must not advertise option colors")
	}
	d.SleepModes[15] = ""
	if _, ok := d.SleepTimerSnapshot(); ok {
		t.Fatal("accepted blank sleep option")
	}
}

func initializeMAKR75KeyboardFixtures(t *testing.T) {
	t.Helper()
	packageRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	applicationRoot := filepath.Clean(filepath.Join(packageRoot, "..", "..", ".."))
	t.Setenv("LUMENFORGE_SERVICE_MODE", string(config.ServiceModeUser))
	t.Setenv("LUMENFORGE_APPLICATION_ROOT", applicationRoot)
	t.Setenv("LUMENFORGE_CONFIG_ROOT", filepath.Join(t.TempDir(), "config"))
	t.Setenv("LUMENFORGE_DATA_ROOT", filepath.Join(t.TempDir(), "data"))
	config.Init()
	keyboards.Init()
}

func makr75TestAssignments() map[int]string {
	return map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}
}
func makr75TestDialModes() map[int]string {
	return map[int]string{1: "Volume Control", 2: "Brightness", 3: "Vertical Scroll", 4: "Zoom", 5: "Screen Brightness", 6: "Media Control", 7: "Horizontal Scroll"}
}
