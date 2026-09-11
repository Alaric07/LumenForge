package scufenvisionproV2WU

import (
	"LumenForge/src/common"
	"LumenForge/src/inputmanager"
	"reflect"
	"testing"
)

func TestWorkspaceSnapshotsPreserveV2USBSourceCapabilities(t *testing.T) {
	d := controllerWorkspaceTestDevice()
	if !d.Usb {
		t.Fatal("USB source semantics changed")
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.Supported || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete || !reflect.DeepEqual(profiles.Profiles, []string{"Default", "Gaming"}) || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles = %#v, ok=%t", profiles, ok)
	}
	controller, ok := d.ControllerSnapshot()
	if !ok {
		t.Fatal("valid V2 USB controller snapshot rejected")
	}
	if len(controller.AssignmentTypes) != 8 || len(controller.Assignments) != 8 || len(controller.Vibrations) != 2 || len(controller.Thumbsticks) != 2 || len(controller.Analogs) != 4 {
		t.Fatalf("controller = %#v", controller)
	}
	if controller.Assignments[6].Index != 2048 || controller.Assignments[7].Index != 4096 || controller.Thumbsticks[0].SensitivityX != 20 || controller.Thumbsticks[1].SensitivityY != 30 {
		t.Fatalf("controller source values = %#v", controller)
	}
	for id, analog := range controller.Analogs {
		if analog.ID != uint8(id) || len(analog.Points) != 6 || analog.Points[id].Index != id {
			t.Fatalf("analog %d = %#v", id, analog)
		}
	}
	sleep, ok := d.SleepTimerSnapshot()
	if !ok || sleep.Value != 15 || len(sleep.Options) != 7 || sleep.Options[0].Value != 0 || sleep.Options[0].Label != "Never" {
		t.Fatalf("sleep = %#v, ok=%t", sleep, ok)
	}
}

func controllerWorkspaceTestDevice() *Device {
	points := map[int]common.CurveData{0: {X: 0, Y: 0}, 1: {X: 20, Y: 20}, 2: {X: 40, Y: 40}, 3: {X: 60, Y: 60}, 4: {X: 80, Y: 80}, 5: {X: 100, Y: 100}}
	analog := map[int]AnalogData{}
	for id := 0; id < 4; id++ {
		analog[id] = AnalogData{DeadZoneMin: 5, DeadZoneMax: 5, Points: points}
	}
	profile := &DeviceProfile{Active: true, SleepMode: 15, LeftVibrationValue: 60, RightVibrationValue: 50, LeftThumbStickMode: 1, LeftThumbStickSensitivityX: 20, LeftThumbStickSensitivityY: 20, RightThumbStickMode: 2, RightThumbStickSensitivityX: 30, RightThumbStickSensitivityY: 30, AnalogData: analog}
	return &Device{Serial: "scuf-v2-usb", Usb: true, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 2: "DPI", 3: "Keyboard", 4: "Controller", 8: "Sniper", 9: "Mouse", 10: "Macro"}, ThumbStickModes: map[int]string{0: "None", 1: "Mouse", 2: "Thumbstick"}, KeyAssignment: map[int]inputmanager.KeyAssignment{1: {Name: "Left Button", ActionType: 0}, 2: {Name: "DPAD Up", ActionType: 4, ActionCommand: 141}, 4: {Name: "DPAD Down", ActionType: 4, ActionCommand: 142}, 8: {Name: "DPAD Left", ActionType: 4, ActionCommand: 143}, 16: {Name: "DPAD Right", ActionType: 4, ActionCommand: 144}, 32: {Name: "A", ActionType: 4, ActionCommand: 128}, 2048: {Name: "LT", ActionType: 4, ActionCommand: 134}, 4096: {Name: "RT", ActionType: 4, ActionCommand: 135}}, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"Default": profile, "Gaming": {}}, SleepModes: map[int]string{0: "Never", 1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}}
}
