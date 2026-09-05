package m75W

import (
	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
	"testing"
)

func m75WWorkspaceDevice() *Device {
	keys := map[int]inputmanager.KeyAssignment{}
	for _, key := range m75WVisibleButtonOrder {
		keys[key] = inputmanager.KeyAssignment{Name: "Button", Default: true}
	}
	stages := map[int]DPIProfile{}
	for index := 0; index < 6; index++ {
		stages[index] = DPIProfile{Name: "Stage", Value: uint16(800 + index*100), Color: &rgb.Color{Red: float64(index), Green: 2, Blue: 3}}
	}
	sniper := stages[5]
	sniper.Name, sniper.Sniper = "Sniper", true
	stages[5] = sniper
	return &Device{Serial: "m75-wireless", MinDPI: 100, MaxDPI: 26000, DPIAmount: 6, KeyAssignment: keys, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media", 2: "DPI", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Profile"}, PollingRates: map[int]string{7: "8000 Hz", 1: "1000 Hz"}, SwitchModes: map[int]string{1: "Enabled", 0: "Disabled"}, LiftHeights: map[int]string{3: "Medium", 2: "Low"}, SleepModes: map[int]string{15: "15 minutes", 1: "1 minute", 5: "5 minutes", 10: "10 minutes", 30: "30 minutes", 60: "1 hour"}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "FPS": {}, "nil": nil}, DeviceProfile: &DeviceProfile{Profile: 1, PollingRate: 7, AngleSnapping: 1, ButtonOptimization: 1, LiftHeight: 3, SleepMode: 15, Profiles: stages}}
}

func TestM75WWorkspaceSnapshots(t *testing.T) {
	d := m75WWorkspaceDevice()
	dpi, ok := d.DPISnapshot()
	if !ok || len(dpi.Stages) != 6 || dpi.ActiveRegularStageID != "1" || dpi.Stages[5].ID != "5" || !dpi.Stages[5].Sniper || dpi.MinimumDPI != 100 || dpi.MaximumDPI != 26000 || dpi.Stages[1].ColorHex != "#010203" {
		t.Fatalf("dpi = %#v, ok=%t", dpi, ok)
	}
	buttons, ok := d.ButtonsSnapshot()
	if !ok || len(buttons.Buttons) != 8 || buttons.Buttons[0].KeyIndex != 1 || buttons.Buttons[7].KeyIndex != 128 || len(buttons.AssignmentTypes) != 8 || buttons.AssignmentTypes[7].ID != 11 {
		t.Fatalf("buttons = %#v, ok=%t", buttons, ok)
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || performance.PollingRate == nil || performance.ButtonOptimization == nil || performance.AngleSnapping == nil || performance.LiftHeight == nil || len(performance.PollingRate.Options) != 2 || performance.PollingRate.Options[0].Value != 1 {
		t.Fatalf("performance = %#v, ok=%t", performance, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || len(profiles.Profiles) != 2 || profiles.Profiles[0] != "Default" || profiles.Profiles[1] != "FPS" || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles = %#v, ok=%t", profiles, ok)
	}
	sleep, ok := d.SleepTimerSnapshot()
	if !ok || sleep.Value != 15 || len(sleep.Options) != 6 || sleep.Options[0].Value != 1 || sleep.Options[5].Value != 60 {
		t.Fatalf("sleep = %#v, ok=%t", sleep, ok)
	}
	if d.SelectMouseDPIStage(1) != 1 || d.SetMouseSniperMode(false) != 1 || d.SelectMouseDPIStage(5) != 0 {
		t.Fatal("DPI mutation did not accept the valid stage/sniper state and reject Sniper stage selection")
	}
}

func TestM75WWorkspaceSnapshotsFailClosed(t *testing.T) {
	d := m75WWorkspaceDevice()
	d.DeviceProfile.Profiles = map[int]DPIProfile{}
	if _, ok := d.DPISnapshot(); ok {
		t.Fatal("empty DPI stages accepted")
	}
	d = m75WWorkspaceDevice()
	d.DeviceProfile.Profiles[1] = DPIProfile{Color: nil}
	if _, ok := d.DPISnapshot(); ok {
		t.Fatal("invalid DPI color accepted")
	}
	d = m75WWorkspaceDevice()
	d.PollingRates[1] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("invalid polling label accepted")
	}
	d = m75WWorkspaceDevice()
	delete(d.KeyAssignment, 1)
	if _, ok := d.ButtonsSnapshot(); ok {
		t.Fatal("missing required button accepted")
	}
	d = m75WWorkspaceDevice()
	d.UserProfiles["Default"].Active = false
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("missing active profile accepted")
	}
	d = m75WWorkspaceDevice()
	d.UserProfiles = map[string]*DeviceProfile{}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("empty profile set accepted")
	}
	d = m75WWorkspaceDevice()
	d.SleepModes[15] = ""
	if _, ok := d.SleepTimerSnapshot(); ok {
		t.Fatal("invalid sleep option accepted")
	}
}
