package m65rgbelite

import (
	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
	"testing"
)

func m65WorkspaceDevice() *Device {
	keys := map[int]inputmanager.KeyAssignment{}
	for _, e := range []struct {
		k int
		n string
	}{{1, "Left"}, {2, "Right"}, {4, "Middle"}, {8, "Back"}, {16, "Forward"}, {32, "DPI Up"}, {64, "DPI Down"}, {128, "Sniper"}} {
		keys[e.k] = inputmanager.KeyAssignment{Name: e.n, Default: true}
	}
	profiles := map[int]DPIProfile{}
	for i := 0; i < 6; i++ {
		profiles[i] = DPIProfile{Name: "Stage", Value: uint16(800 + i*100), Color: &rgb.Color{Red: float64(i)}}
		if i == 5 {
			p := profiles[i]
			p.Name = "Sniper"
			p.Sniper = true
			profiles[i] = p
		}
	}
	return &Device{Serial: "m65", MinDPI: 100, MaxDPI: 18000, DPIAmount: 6, KeyAssignment: keys, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media", 2: "DPI +", 3: "Keyboard", 4: "DPI -", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Profile"}, PollingRates: map[int]string{1: "1000 Hz"}, SwitchModes: map[int]string{1: "Enabled"}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "FPS": {}, "nil": nil}, DeviceProfile: &DeviceProfile{Profile: 1, PollingRate: 1, ButtonOptimization: 1, AngleSnapping: 1, Profiles: profiles}}
}
func TestM65WorkspaceSnapshots(t *testing.T) {
	d := m65WorkspaceDevice()
	dpi, ok := d.DPISnapshot()
	if !ok || len(dpi.Stages) != 6 || !dpi.Stages[5].Sniper {
		t.Fatalf("dpi=%#v ok=%t", dpi, ok)
	}
	b, ok := d.ButtonsSnapshot()
	if !ok || len(b.Buttons) != 8 || b.AssignmentTypes[2].Label != "DPI +" || b.AssignmentTypes[4].Label != "DPI -" {
		t.Fatalf("buttons=%#v ok=%t", b, ok)
	}
	p, ok := d.PerformanceSnapshot()
	if !ok || p.LiftHeight != nil || p.ButtonOptimization == nil {
		t.Fatalf("performance=%#v ok=%t", p, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || len(profiles.Profiles) != 2 || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
	d.UserProfiles["Default"].Active = false
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("profiles did not fail closed without active profile")
	}
	if d.SelectMouseDPIStage(5) != 0 || d.SaveMouseDPISettings(nil, nil) != 0 {
		t.Fatal("invalid DPI draft accepted")
	}
	d.DeviceProfile.Profiles = map[int]DPIProfile{}
	if d.SaveMouseDPISettings(map[int]uint16{}, map[int]rgb.Color{}) != 0 {
		t.Fatal("empty DPI profile set unexpectedly succeeded")
	}
}
