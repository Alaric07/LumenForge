package darkcorergbseWU

import (
	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
	"testing"
)

func darkcorergbseWUWorkspaceDevice() *Device {
	profile := &DeviceProfile{Active: true, Profile: 0, SleepMode: 15, AngleSnapping: 1, PollingRate: 1, LiftHeight: 2, DPIColor: &rgb.Color{Red: 1, Green: 2, Blue: 3}, SniperColor: &rgb.Color{Red: 4, Green: 5, Blue: 6}, Profiles: map[int]DPIProfile{0: {Name: "Stage 1", Value: 800}, 1: {Name: "Sniper", Value: 400, Sniper: true}}}
	d := &Device{Serial: "workspace-test", DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"Default": profile}, MinDPI: 100, MaxDPI: 18000, DPIAmount: 2, KeyAssignmentTypes: map[int]string{0: "None"}, KeyAssignment: map[int]inputmanager.KeyAssignment{}}
	for _, key := range []int{256, 128, 64, 32, 16, 8, 4, 2, 1} {
		d.KeyAssignment[key] = inputmanager.KeyAssignment{Name: "Button"}
	}
	d.PollingRates = map[int]string{1: "1000 Hz"}
	d.LiftHeights = map[int]string{2: "Low"}
	return d
}
func TestDarkcorergbseWUWorkspaceSnapshotShape(t *testing.T) {
	d := darkcorergbseWUWorkspaceDevice()
	dpi, ok := d.DPISnapshot()
	if !ok || len(dpi.Stages) != 2 || dpi.ActiveRegularStageID != "0" {
		t.Fatalf("DPI=%#v,%t", dpi, ok)
	}
	if true && (dpi.Stages[0].ColorHex != "#010203" || dpi.Stages[1].ColorHex != "#040506") {
		t.Fatalf("separate colors lost: %#v", dpi.Stages)
	}
	buttons, ok := d.ButtonsSnapshot()
	if !ok || len(buttons.Buttons) != len([]int{256, 128, 64, 32, 16, 8, 4, 2, 1}) {
		t.Fatal("button snapshot rejected")
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || (performance.PollingRate != nil) != true || (performance.ButtonOptimization != nil) != false || (performance.LiftHeight != nil) != true || performance.AngleSnapping == nil {
		t.Fatalf("performance=%#v,%t", performance, ok)
	}
	if _, ok := d.DeviceProfileSnapshot(); !ok {
		t.Fatal("profile snapshot rejected")
	}
	if d.SelectMouseDPIStage(0) != 1 || d.SelectMouseDPIStage(1) != 0 || d.SaveMouseDPISettings(nil, nil) != 0 {
		t.Fatal("DPI mutation validation failed")
	}
}
func TestDarkcorergbseWUWorkspaceFailsClosed(t *testing.T) {
	var nilDevice *Device
	if _, ok := nilDevice.DPISnapshot(); ok {
		t.Fatal("nil DPI accepted")
	}
	d := darkcorergbseWUWorkspaceDevice()
	d.DeviceProfile.SniperColor = nil
	if _, ok := d.DPISnapshot(); ok {
		t.Fatal("missing sniper color accepted")
	}
	d = darkcorergbseWUWorkspaceDevice()
	d.PollingRates[1] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("empty polling label accepted")
	}
	d = darkcorergbseWUWorkspaceDevice()
	d.LiftHeights[2] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("empty lift label accepted")
	}
	d = darkcorergbseWUWorkspaceDevice()
}
