package sabrergbpro

import (
	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
	"testing"
)

func sabreWorkspaceDevice() *Device {
	keys := map[int]inputmanager.KeyAssignment{}
	for _, e := range []struct {
		k int
		n string
	}{{1, "Left"}, {2, "Right"}, {4, "Middle"}, {8, "Back"}, {16, "Forward"}, {32, "DPI"}} {
		keys[e.k] = inputmanager.KeyAssignment{Name: e.n, Default: true}
	}
	profiles := map[int]DPIProfile{}
	for i := 0; i < 6; i++ {
		profiles[i] = DPIProfile{Name: "Stage", Value: uint16(200 + i*200)}
		if i == 5 {
			p := profiles[i]
			p.Name = "Sniper"
			p.Sniper = true
			profiles[i] = p
		}
	}
	return &Device{Serial: "sabre", MinDPI: 100, MaxDPI: 18000, DPIAmount: 6, KeyAssignment: keys, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media", 2: "DPI", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Profile"}, PollingRates: map[int]string{1: "125 Hz", 7: "8000 Hz"}, SwitchModes: map[int]string{1: "Enabled"}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "FPS": {}, "nil": nil}, DeviceProfile: &DeviceProfile{Profile: 1, PollingRate: 7, ButtonOptimization: 1, AngleSnapping: 1, DPIColor: &rgb.Color{Green: 255}, SniperColor: &rgb.Color{Red: 255, Green: 255}, Profiles: profiles}}
}
func TestSabreWorkspaceSnapshots(t *testing.T) {
	d := sabreWorkspaceDevice()
	dpi, ok := d.DPISnapshot()
	if !ok || len(dpi.Stages) != 6 || dpi.Stages[5].ColorHex != "#ffff00" {
		t.Fatalf("dpi=%#v ok=%t", dpi, ok)
	}
	p, ok := d.PerformanceSnapshot()
	if !ok || len(p.PollingRate.Options) != 2 || p.PollingRate.Options[1].Value != 7 || p.LiftHeight != nil {
		t.Fatalf("performance=%#v ok=%t", p, ok)
	}
	b, ok := d.ButtonsSnapshot()
	if !ok || len(b.Buttons) != 6 {
		t.Fatalf("buttons=%#v ok=%t", b, ok)
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
}
