package scimitarSEW

import (
	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
	"testing"
)

func scimitarSEWWorkspaceDevice() *Device {
	profile := &DeviceProfile{Active: true, Profile: 0, SleepMode: 15, AngleSnapping: 1, ButtonOptimization: 1, LiftHeight: 2, Profiles: map[int]DPIProfile{0: {Name: "Stage 1", Value: 800, Color: &rgb.Color{Red: 1, Green: 2, Blue: 3}}, 1: {Name: "Sniper", Value: 400, Color: &rgb.Color{Red: 1, Green: 2, Blue: 3}, Sniper: true}}}
	d := &Device{Serial: "workspace-test", DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"Default": profile}, MinDPI: 100, MaxDPI: 18000, DPIAmount: 2, SleepModes: map[int]string{15: "15 minutes"}, SwitchModes: map[int]string{0: "Disabled", 1: "Enabled"}, KeyAssignmentTypes: map[int]string{0: "None"}, KeyAssignment: map[int]inputmanager.KeyAssignment{}}
	for _, key := range []int{65536, 32768, 16384, 8192, 4096, 2048, 1024, 512, 256, 128, 64, 32, 16, 8, 4, 2, 1} {
		d.KeyAssignment[key] = inputmanager.KeyAssignment{Name: "Button"}
	}

	d.LiftHeights = map[int]string{2: "Low"}
	return d
}

func TestScimitarSEWWorkspaceSnapshotShapeAndCapabilities(t *testing.T) {
	d := scimitarSEWWorkspaceDevice()
	dpi, ok := d.DPISnapshot()
	if !ok || len(dpi.Stages) != 2 || dpi.ActiveRegularStageID != "0" {
		t.Fatalf("DPI snapshot = %#v, %t", dpi, ok)
	}
	buttons, ok := d.ButtonsSnapshot()
	if !ok || len(buttons.Buttons) != len([]int{65536, 32768, 16384, 8192, 4096, 2048, 1024, 512, 256, 128, 64, 32, 16, 8, 4, 2, 1}) {
		t.Fatalf("buttons snapshot = %#v, %t", buttons, ok)
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || (performance.PollingRate != nil) != false || (performance.LiftHeight != nil) != true || performance.ButtonOptimization == nil || performance.AngleSnapping == nil {
		t.Fatalf("performance snapshot = %#v, %t", performance, ok)
	}
	if _, ok := d.DeviceProfileSnapshot(); !ok {
		t.Fatal("profile snapshot rejected")
	}
	if _, ok := d.SleepTimerSnapshot(); !ok {
		t.Fatal("sleep timer snapshot rejected")
	}
	if d.SelectMouseDPIStage(0) != 1 || d.SelectMouseDPIStage(1) != 0 || d.SaveMouseDPISettings(nil, nil) != 0 {
		t.Fatal("DPI stage mutation did not enforce regular-stage selection")
	}
}

func TestScimitarSEWWorkspaceSnapshotsFailClosed(t *testing.T) {
	var nilDevice *Device
	if _, ok := nilDevice.DPISnapshot(); ok {
		t.Fatal("nil DPI snapshot accepted")
	}
	d := scimitarSEWWorkspaceDevice()
	d = scimitarSEWWorkspaceDevice()
	d.SleepModes[15] = ""
	if _, ok := d.SleepTimerSnapshot(); ok {
		t.Fatal("empty sleep label accepted")
	}
	d = scimitarSEWWorkspaceDevice()
	d.SwitchModes[1] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("empty performance label accepted")
	}
}
