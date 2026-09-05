package katarpro

import (
	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
	"reflect"
	"testing"
)

func TestKatarWorkspaceSnapshots(t *testing.T) {
	assignments := map[int]inputmanager.KeyAssignment{1: {Name: "Left Button", Default: true}, 2: {Name: "Right Button", Default: true}, 4: {Name: "Middle Button", Default: true}, 8: {Name: "Forward Button", ActionType: 10, ActionCommand: 42, IsMacro: true}, 16: {Name: "Back Button", Default: true}, 32: {Name: "DPI Button", Default: true}}
	d := &Device{Serial: "katar", MinDPI: 100, MaxDPI: 12400, DeviceProfile: &DeviceProfile{Profile: 1, PollingRate: 4, ButtonOptimization: 1, Profiles: map[int]DPIProfile{2: {Name: "Sniper", Value: 400, Color: &rgb.Color{Red: 255, Green: 170}, Sniper: true}, 0: {Value: 800, Color: &rgb.Color{Red: 1, Green: 2, Blue: 3}}, 1: {Name: "Stage 2", Value: 1600, Color: &rgb.Color{Green: 255}}}}, KeyAssignment: assignments, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media", 2: "DPI", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Profile"}, PollingRates: map[int]string{4: "1000", 1: "125", 0: "Not Set"}, SwitchModes: map[int]string{1: "Enabled", 0: "Disabled"}, UserProfiles: map[string]*DeviceProfile{"z": {}, "a": {Active: true}, "nil": nil}}
	dpi, ok := d.DPISnapshot()
	if !ok || dpi.MinimumDPI != 100 || dpi.MaximumDPI != 12400 || dpi.ActiveRegularStageID != "1" || dpi.Stages[0].ID != "0" || dpi.Stages[0].Name != "Stage 1" || dpi.Stages[0].ColorHex != "#010203" || !dpi.Stages[2].Sniper {
		t.Fatalf("dpi=%#v ok=%t", dpi, ok)
	}
	buttons, ok := d.ButtonsSnapshot()
	if !ok || !reflect.DeepEqual([]int{buttons.Buttons[0].KeyIndex, buttons.Buttons[1].KeyIndex, buttons.Buttons[2].KeyIndex, buttons.Buttons[3].KeyIndex, buttons.Buttons[4].KeyIndex, buttons.Buttons[5].KeyIndex}, katarProVisibleButtonOrder) || buttons.Buttons[3].ActionCommand != 42 || len(buttons.AssignmentTypes) != 8 {
		t.Fatalf("buttons=%#v ok=%t", buttons, ok)
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || performance.PollingRate == nil || performance.ButtonOptimization == nil || performance.AngleSnapping != nil || performance.LiftHeight != nil || performance.ButtonOptimization.Value != 1 || !reflect.DeepEqual([]int{performance.ButtonOptimization.Options[0].Value, performance.ButtonOptimization.Options[1].Value}, []int{0, 1}) {
		t.Fatalf("performance=%#v ok=%t", performance, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !reflect.DeepEqual(profiles.Profiles, []string{"a", "z"}) || profiles.ActiveProfile != "a" {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
	d.DeviceProfile = nil
	if _, ok := d.DPISnapshot(); ok {
		t.Fatal("DPI snapshot did not fail closed")
	}
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("performance snapshot did not fail closed")
	}
}
