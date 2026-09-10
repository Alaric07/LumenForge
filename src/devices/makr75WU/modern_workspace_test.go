package makr75WU

import (
	"LumenForge/src/config"
	"LumenForge/src/keyboards"
	"LumenForge/src/rgb"
	"path/filepath"
	"testing"
)

func TestMAKR75USBModernWorkspaceContract(t *testing.T) {
	initializeMAKR75KeyboardFixtures(t)
	keyboard := keyboards.GetKeyboard("makr75-default-US")
	if keyboard == nil {
		t.Fatal("missing MAKR 75 shipped keyboard layout")
	}
	polling := map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}
	dialModes := makr75WUTestDialModes()
	colors := map[int]*rgb.Color{}
	for id := range dialModes {
		colors[id] = &rgb.Color{Red: float64(id)}
	}
	d := &Device{Serial: "makr75-usb", UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-17", Layouts: []string{"US"}, KeyAssignmentTypes: makr75WUTestAssignments(), PollingRates: polling, ControlDialOptions: dialModes, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, ControlDial: 1, ControlDialColors: colors, DisableWinKey: true, DisableShiftTab: true, DisableAltTab: true, DisableAltF4: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	assignments, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || assignments.LayoutClass != "keyboard-6" || assignments.RowLayoutClass != "keyboard-row-17" || len(assignments.Rows) == 0 || len(assignments.AssignmentTypes) != len(d.KeyAssignmentTypes) || len(assignments.ModifierOptions) < 2 || assignments.LiveRGBAvailable {
		t.Fatalf("assignments=%#v ok=%t", assignments, ok)
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || performance.PollingRate == nil || len(performance.PollingRate.Options) != 8 || len(performance.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v ok=%t", performance, ok)
	}
	for id := 0; id <= 7; id++ {
		if performance.PollingRate.Options[id].Value != id || performance.PollingRate.Options[id].Label != polling[id] {
			t.Fatalf("polling=%#v", performance.PollingRate)
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
	optionColors, ok := d.OptionColorsSnapshot()
	if !ok || optionColors.Selected != 1 || len(optionColors.Options) != 7 || optionColors.Options[0].Action != "control-dial-colors" {
		t.Fatalf("optionColors=%#v ok=%t", optionColors, ok)
	}
	d.DeviceProfile.PollingRate = 99
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("accepted unsupported polling rate")
	}
	d.DeviceProfile.PollingRate = 4
	d.DeviceProfile.ControlDial = 99
	if _, ok := d.OptionColorsSnapshot(); ok {
		t.Fatal("accepted unsupported color target")
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

func makr75WUTestAssignments() map[int]string {
	return map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}
}
func makr75WUTestDialModes() map[int]string {
	return map[int]string{1: "Volume Control", 2: "Brightness", 3: "Vertical Scroll", 4: "Zoom", 5: "Screen Brightness", 6: "Media Control", 7: "Horizontal Scroll"}
}
