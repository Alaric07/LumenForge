package scimitar

import (
	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
	"testing"
)

func scimitarWorkspaceDevice() *Device {
	keys := map[int]inputmanager.KeyAssignment{}
	for _, k := range scimitarVisibleButtonOrder {
		keys[k] = inputmanager.KeyAssignment{Name: "Button", Default: true}
	}
	stages := map[int]DPIProfile{}
	for i := 0; i < 6; i++ {
		stages[i] = DPIProfile{Name: "Stage", Value: uint16(800 + i*100), Color: &rgb.Color{Red: float64(i)}}
	}
	p := stages[5]
	p.Name, p.Sniper = "Sniper", true
	stages[5] = p
	return &Device{Serial: "scimitar", MinDPI: 100, MaxDPI: 18000, DPIAmount: 6, KeyAssignment: keys, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media", 2: "DPI", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Profile"}, PollingRates: map[int]string{1: "1000 Hz"}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "FPS": {}, "nil": nil}, DeviceProfile: &DeviceProfile{Profile: 1, PollingRate: 1, AngleSnapping: 1, Profiles: stages}}
}
func TestScimitarWorkspaceSnapshotsFailClosed(t *testing.T) {
	d := scimitarWorkspaceDevice()
	dpi, ok := d.DPISnapshot()
	if !ok || len(dpi.Stages) != 6 || dpi.ActiveRegularStageID != "1" || !dpi.Stages[5].Sniper {
		t.Fatalf("dpi=%#v ok=%t", dpi, ok)
	}
	b, ok := d.ButtonsSnapshot()
	if !ok || len(b.Buttons) != 17 || b.Buttons[16].KeyIndex != 65536 {
		t.Fatalf("buttons=%#v ok=%t", b, ok)
	}
	p, ok := d.PerformanceSnapshot()
	if !ok || p.PollingRate == nil || p.AngleSnapping == nil || p.LiftHeight != nil || p.ButtonOptimization != nil {
		t.Fatalf("performance=%#v ok=%t", p, ok)
	}
	if d.SelectMouseDPIStage(1) != 1 || d.SelectMouseDPIStage(5) != 0 || d.SaveMouseDPISettings(nil, nil) != 0 {
		t.Fatal("invalid DPI mutation accepted")
	}
	d.DeviceProfile.Profiles = map[int]DPIProfile{}
	if _, ok := d.DPISnapshot(); ok || d.SaveMouseDPISettings(map[int]uint16{}, map[int]rgb.Color{}) != 0 {
		t.Fatal("empty DPI state accepted")
	}
	d = scimitarWorkspaceDevice()
	d.PollingRates[1] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("invalid polling label accepted")
	}
	d = scimitarWorkspaceDevice()
	delete(d.KeyAssignment, 1)
	if _, ok := d.ButtonsSnapshot(); ok {
		t.Fatal("missing button accepted")
	}
	d = scimitarWorkspaceDevice()
	d.UserProfiles["Default"].Active = false
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("inactive profiles accepted")
	}
}
