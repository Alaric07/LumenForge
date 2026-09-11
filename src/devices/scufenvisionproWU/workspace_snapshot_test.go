package scufenvisionproWU

import (
	"LumenForge/src/common"
	"LumenForge/src/inputmanager"
	"testing"
)

func TestControllerSnapshotAndCompleteSleepTimer(t *testing.T) {
	points := map[int]common.CurveData{0: {X: 0, Y: 0}, 1: {X: 100, Y: 100}}
	analog := map[int]AnalogData{}
	for id := 0; id < 4; id++ {
		analog[id] = AnalogData{DeadZoneMin: 5, DeadZoneMax: 5, Points: points}
	}
	d := &Device{Serial: "scuf-usb", Usb: true, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 2: "DPI", 3: "Keyboard", 4: "Controller", 8: "Sniper", 9: "Mouse", 10: "Macro"}, ThumbStickModes: map[int]string{0: "None", 1: "Mouse", 2: "Thumbstick"}, KeyAssignment: map[int]inputmanager.KeyAssignment{1: {Name: "Left Button", ActionType: 0}, 4096: {Name: "Right Trigger", ActionType: 4, ActionCommand: 135}}, DeviceProfile: &DeviceProfile{SleepMode: 15, LeftVibrationValue: 60, RightVibrationValue: 50, LeftThumbStickMode: 1, LeftThumbStickSensitivityX: 20, LeftThumbStickSensitivityY: 20, RightThumbStickMode: 2, RightThumbStickSensitivityX: 20, RightThumbStickSensitivityY: 20, AnalogData: analog}, SleepModes: map[int]string{0: "Never", 1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}}
	if !d.Usb {
		t.Fatal("USB source state changed")
	}
	if _, ok := d.ControllerSnapshot(); !ok {
		t.Fatal("controller snapshot rejected")
	}
	sleep, ok := d.SleepTimerSnapshot()
	if !ok || len(sleep.Options) != 7 || sleep.Options[0].Label != "Never" {
		t.Fatalf("sleep = %#v, ok=%t", sleep, ok)
	}
}
