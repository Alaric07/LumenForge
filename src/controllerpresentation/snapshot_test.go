package controllerpresentation

import (
	"LumenForge/src/common"
	"testing"
)

func controllerTestSnapshot(t *testing.T) Snapshot {
	t.Helper()
	types := map[int]string{0: "None", 1: "Media Keys", 2: "DPI", 3: "Keyboard", 4: "Controller", 8: "Sniper", 9: "Mouse", 10: "Macro"}
	modes := []AssignmentType{{ID: 0, Label: "None"}, {ID: 1, Label: "Mouse"}, {ID: 2, Label: "Thumbstick"}}
	points := map[int]common.CurveData{0: {X: 0, Y: 0}, 1: {X: 100, Y: 100}}
	analogs := map[int]AnalogData{}
	for id := 0; id < 4; id++ {
		analogs[id] = AnalogData{DeadZoneMin: 2, DeadZoneMax: 15, Points: points}
	}
	s, ok := Build(map[int]Assignment{1: {Index: 1, Label: "Left Button", ActionType: 0}, 2048: {Index: 2048, Label: "Left Trigger", ActionType: 4, ActionCommand: 134}}, types, [2]uint8{60, 50}, [2]Thumbstick{{Module: 0, Label: "Left Thumbstick", Mode: 1, SensitivityX: 20, SensitivityY: 20, Modes: modes}, {Module: 1, Label: "Right Thumbstick", Mode: 2, SensitivityX: 20, SensitivityY: 20, Modes: modes}}, analogs)
	if !ok {
		t.Fatal("valid snapshot rejected")
	}
	return s
}

func TestBuildRetainsControllerContracts(t *testing.T) {
	s := controllerTestSnapshot(t)
	if len(s.AssignmentTypes) != 8 || s.AssignmentTypes[5].ID != 8 || s.Assignments[1].Index != 2048 || len(s.Analogs) != 4 || s.Analogs[3].Label != "Right Trigger" || s.Vibrations[0].Value != 60 {
		t.Fatalf("snapshot = %#v", s)
	}
}

func TestBuildFailsClosed(t *testing.T) {
	types := map[int]string{0: "None"}
	_, ok := Build(map[int]Assignment{1: {Index: 1, Label: "Left Button"}}, types, [2]uint8{}, [2]Thumbstick{}, nil)
	if ok {
		t.Fatal("malformed contract accepted")
	}
}
