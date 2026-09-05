package glaivergb

import (
	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
	"testing"
)

func glaiveRGBWorkspaceDevice() *Device {
	green := &rgb.Color{Green: 255}
	yellow := &rgb.Color{Red: 255, Green: 255}
	keys := map[int]inputmanager.KeyAssignment{}
	for _, e := range []struct {
		k int
		n string
	}{{1, "Left"}, {2, "Right"}, {4, "Middle"}, {8, "Back"}, {16, "Forward"}, {32, "DPI Toggle"}} {
		keys[e.k] = inputmanager.KeyAssignment{Name: e.n, Default: true}
	}
	return &Device{Serial: "glaive", MinDPI: 100, MaxDPI: 16000, DPIAmount: 6, KeyAssignment: keys, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media", 2: "DPI", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Profile"}, PollingRates: map[int]string{1: "1000 Hz"}, ButtonOptimizations: map[int]string{4: "Normal"}, LiftHeights: map[int]string{3: "Medium"}, UserProfiles: map[string]*DeviceProfile{"FPS": {}, "Default": {Active: true}, "nil": nil}, DeviceProfile: &DeviceProfile{Profile: 1, PollingRate: 1, ButtonOptimization: 4, LiftHeight: 3, AngleSnapping: 1, DPIColor: green, SniperColor: yellow, Profiles: map[int]DPIProfile{0: {Name: "Stage 1", Value: 800}, 1: {Name: "Stage 2", Value: 1500}, 2: {Name: "Stage 3", Value: 3000}, 3: {Name: "Stage 4", Value: 6000}, 4: {Name: "Stage 5", Value: 9000}, 5: {Name: "Sniper", Value: 200, Sniper: true}}}}
}
func TestGlaiveWorkspaceSnapshots(t *testing.T) {
	d := glaiveRGBWorkspaceDevice()
	dpi, ok := d.DPISnapshot()
	if !ok || len(dpi.Stages) != 6 || dpi.ActiveRegularStageID != "1" || dpi.Stages[5].ColorHex != "#ffff00" {
		t.Fatalf("dpi=%#v ok=%t", dpi, ok)
	}
	b, ok := d.ButtonsSnapshot()
	if !ok || len(b.Buttons) != 6 || b.Buttons[5].KeyIndex != 32 {
		t.Fatalf("buttons=%#v ok=%t", b, ok)
	}
	p, ok := d.PerformanceSnapshot()
	if !ok || p.LiftHeight == nil || p.ButtonOptimization == nil || p.AngleSnapping == nil {
		t.Fatalf("performance=%#v ok=%t", p, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || len(profiles.Profiles) != 2 || profiles.Profiles[0] != "Default" {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
	if d.SelectMouseDPIStage(5) != 0 || d.SaveMouseDPISettings(map[int]uint16{}, map[int]rgb.Color{}) != 0 {
		t.Fatal("invalid DPI draft accepted")
	}
}
