package ironclaw

import (
	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
	"testing"
)

func ironclawWorkspaceDevice() *Device {
	keys := map[int]inputmanager.KeyAssignment{}
	for _, e := range []struct {
		k int
		n string
	}{{1, "Left"}, {2, "Right"}, {4, "Middle"}, {8, "Back"}, {16, "Forward"}, {32, "DPI Button"}, {256, "Profile Button"}} {
		keys[e.k] = inputmanager.KeyAssignment{Name: e.n, Default: true}
	}
	return &Device{Serial: "ironclaw", MinDPI: 100, MaxDPI: 18000, DPIAmount: 4, KeyAssignment: keys, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media", 2: "DPI", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Profile"}, PollingRates: map[int]string{1: "1000 Hz", 8: "125 Hz"}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "FPS": {}, "nil": nil}, DeviceProfile: &DeviceProfile{Profile: 1, PollingRate: 1, AngleSnapping: 1, DPIColor: &rgb.Color{Green: 255}, Profiles: map[int]DPIProfile{0: {Name: "Stage 1", Value: 800}, 1: {Name: "Stage 2", Value: 1500}, 2: {Name: "Stage 3", Value: 3000}, 3: {Name: "Sniper", Value: 200, Sniper: true}}}}
}
func TestIronclawWorkspaceSnapshots(t *testing.T) {
	d := ironclawWorkspaceDevice()
	dpi, ok := d.DPISnapshot()
	if !ok || len(dpi.Stages) != 4 || dpi.Stages[3].ColorHex != "#ffff00" {
		t.Fatalf("dpi=%#v ok=%t", dpi, ok)
	}
	b, ok := d.ButtonsSnapshot()
	if !ok || len(b.Buttons) != 7 || b.Buttons[6].KeyIndex != 256 {
		t.Fatalf("buttons=%#v ok=%t", b, ok)
	}
	p, ok := d.PerformanceSnapshot()
	if !ok || p.PollingRate == nil || p.AngleSnapping == nil || p.ButtonOptimization != nil || p.LiftHeight != nil {
		t.Fatalf("performance=%#v ok=%t", p, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || len(profiles.Profiles) != 2 || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
	if d.SelectMouseDPIStage(3) != 0 || d.SaveMouseDPISettings(nil, nil) != 0 {
		t.Fatal("invalid DPI draft accepted")
	}
	d.DeviceProfile.Profiles = map[int]DPIProfile{}
	if d.SaveMouseDPISettings(map[int]uint16{}, map[int]rgb.Color{}) != 0 {
		t.Fatal("empty profiles accepted")
	}
}

func TestIronclawPerformanceSnapshotFailsClosedForInvalidPollingLabel(t *testing.T) {
	d := ironclawWorkspaceDevice()
	d.PollingRates[8] = ""
	if snapshot, ok := d.PerformanceSnapshot(); ok || snapshot.PollingRate != nil || snapshot.AngleSnapping != nil {
		t.Fatalf("snapshot=%#v ok=%t", snapshot, ok)
	}
}
