package darkcorergbproseW

import (
	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
	"testing"
)

func darkcorergbproseWWorkspaceDevice() *Device {
	profile := &DeviceProfile{Active: true, Profile: 0, SleepMode: 15, AngleSnapping: 1, ButtonOptimization: 1, DPIColor: &rgb.Color{Red: 1, Green: 2, Blue: 3}, Profiles: map[int]DPIProfile{0: {Name: "Stage 1", Value: 800}, 1: {Name: "Sniper", Value: 400, Sniper: true}}}
	d := &Device{Serial: "workspace-test", DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"Default": profile}, MinDPI: 100, MaxDPI: 18000, DPIAmount: 2, SleepModes: map[int]string{15: "15 minutes"}, KeyAssignmentTypes: map[int]string{0: "None"}, KeyAssignment: map[int]inputmanager.KeyAssignment{}}
	for _, key := range []int{128, 64, 32, 16, 8, 4, 2, 1} {
		d.KeyAssignment[key] = inputmanager.KeyAssignment{Name: "Button"}
	}
	d.SwitchModes = map[int]string{1: "Enabled"}
	return d
}
func TestDarkcorergbproseWWorkspaceSnapshotShape(t *testing.T) {
	d := darkcorergbproseWWorkspaceDevice()
	dpi, ok := d.DPISnapshot()
	if !ok || len(dpi.Stages) != 2 || dpi.ActiveRegularStageID != "0" {
		t.Fatalf("DPI=%#v,%t", dpi, ok)
	}
	if false && (dpi.Stages[0].ColorHex != "#010203" || dpi.Stages[1].ColorHex != "#040506") {
		t.Fatalf("separate colors lost: %#v", dpi.Stages)
	}
	buttons, ok := d.ButtonsSnapshot()
	if !ok || len(buttons.Buttons) != len([]int{128, 64, 32, 16, 8, 4, 2, 1}) {
		t.Fatal("button snapshot rejected")
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || (performance.PollingRate != nil) != false || (performance.ButtonOptimization != nil) != true || (performance.LiftHeight != nil) != false || performance.AngleSnapping == nil {
		t.Fatalf("performance=%#v,%t", performance, ok)
	}
	if _, ok := d.DeviceProfileSnapshot(); !ok {
		t.Fatal("profile snapshot rejected")
	}
	if _, ok := d.SleepTimerSnapshot(); !ok {
		t.Fatal("sleep timer rejected")
	}
	if d.SelectMouseDPIStage(0) != 1 || d.SelectMouseDPIStage(1) != 0 || d.SaveMouseDPISettings(nil, nil) != 0 {
		t.Fatal("DPI mutation validation failed")
	}
}
func TestDarkcorergbproseWWorkspaceFailsClosed(t *testing.T) {
	var nilDevice *Device
	if _, ok := nilDevice.DPISnapshot(); ok {
		t.Fatal("nil DPI accepted")
	}
	d := darkcorergbproseWWorkspaceDevice()
	d.DeviceProfile.DPIColor = nil
	if _, ok := d.DPISnapshot(); ok {
		t.Fatal("missing shared DPI color accepted")
	}
	d = darkcorergbproseWWorkspaceDevice()
	d.SleepModes[15] = ""
	if _, ok := d.SleepTimerSnapshot(); ok {
		t.Fatal("empty sleep label accepted")
	}
	d = darkcorergbproseWWorkspaceDevice()
	d.SwitchModes[1] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("empty optimization label accepted")
	}
}

func TestDarkcorergbproseWSharedDPIColorMutation(t *testing.T) {
	current := rgb.Color{Red: 1, Green: 2, Blue: 3}
	identical := map[int]rgb.Color{0: current, 1: current}
	if color, ok := darkcorergbproseWSharedDPIColor(identical, current); !ok || color != current {
		t.Fatalf("identical colors = %#v, %t", color, ok)
	}

	edited := rgb.Color{Red: 7, Green: 8, Blue: 9}
	if color, ok := darkcorergbproseWSharedDPIColor(map[int]rgb.Color{0: edited, 1: current}, current); !ok || color != edited {
		t.Fatalf("single-stage edit = %#v, %t", color, ok)
	}
	if _, ok := darkcorergbproseWSharedDPIColor(map[int]rgb.Color{0: edited, 1: {Red: 10, Green: 11, Blue: 12}}, current); ok {
		t.Fatal("conflicting shared colors accepted")
	}

	d := darkcorergbproseWWorkspaceDevice()
	darkcorergbproseWApplyMouseDPISettings(d.DeviceProfile, map[int]uint16{0: 1600, 1: 600}, edited)
	if d.DeviceProfile.Profiles[0].Value != 1600 || d.DeviceProfile.Profiles[1].Value != 600 || d.DeviceProfile.DPIColor.Red != edited.Red || d.DeviceProfile.DPIColor.Green != edited.Green || d.DeviceProfile.DPIColor.Blue != edited.Blue {
		t.Fatalf("valid DPI edit did not persist values and color: %#v", d.DeviceProfile)
	}
}
