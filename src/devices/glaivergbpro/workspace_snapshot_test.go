package glaivergbpro

import (
	"testing"

	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
)

func glaiveWorkspaceDevice() *Device {
	green := &rgb.Color{Red: 0, Green: 255, Blue: 0}
	yellow := &rgb.Color{Red: 255, Green: 255, Blue: 0}
	assignments := map[int]inputmanager.KeyAssignment{}
	for _, entry := range []struct {
		key  int
		name string
	}{{1, "Left Button"}, {2, "Right Button"}, {4, "Middle Button"}, {8, "Back Button"}, {16, "Forward Button"}, {32, "DPI Up"}, {64, "DPI Down"}} {
		assignments[entry.key] = inputmanager.KeyAssignment{Name: entry.name, Default: true}
	}
	return &Device{Serial: "glaive-test", MinDPI: 100, MaxDPI: 18000, DPIAmount: 4, PollingRates: map[int]string{0: "Not Set", 1: "1000 Hz / 1 msec"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 2: "DPI", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Profile Switch"}, KeyAssignment: assignments, UserProfiles: map[string]*DeviceProfile{"FPS": {}, "Default": {Active: true}, "broken": nil}, DeviceProfile: &DeviceProfile{Profile: 1, PollingRate: 1, AngleSnapping: 1, DPIColor: green, SniperColor: yellow, Profiles: map[int]DPIProfile{0: {Name: "Stage 1", Value: 800}, 1: {Name: "Stage 2", Value: 1500}, 2: {Name: "Stage 3", Value: 3000}, 3: {Name: "Sniper", Value: 400, Sniper: true}}}}
}

func TestGlaiveDPISnapshotUsesSortedStagesAndSeparateSniperColor(t *testing.T) {
	snapshot, ok := glaiveWorkspaceDevice().DPISnapshot()
	if !ok || snapshot.MinimumDPI != 100 || snapshot.MaximumDPI != 18000 || snapshot.ActiveRegularStageID != "1" || len(snapshot.Stages) != 4 {
		t.Fatalf("snapshot=%#v ok=%t", snapshot, ok)
	}
	if snapshot.Stages[0].ID != "0" || snapshot.Stages[1].ID != "1" || snapshot.Stages[3].ID != "3" || snapshot.Stages[0].ColorHex != "#00ff00" || snapshot.Stages[3].ColorHex != "#ffff00" || !snapshot.Stages[3].Sniper {
		t.Fatalf("stages=%#v", snapshot.Stages)
	}
}

func TestGlaiveDPISnapshotFailsClosedWithoutCompleteColorState(t *testing.T) {
	d := glaiveWorkspaceDevice()
	d.DeviceProfile.SniperColor = nil
	if _, ok := d.DPISnapshot(); ok {
		t.Fatal("DPI snapshot unexpectedly succeeded without sniper color")
	}
}

func TestGlaiveButtonsSnapshotUsesAllPhysicalButtonIDs(t *testing.T) {
	snapshot, ok := glaiveWorkspaceDevice().ButtonsSnapshot()
	if !ok || len(snapshot.Buttons) != 7 || snapshot.Buttons[0].KeyIndex != 1 || snapshot.Buttons[6].KeyIndex != 64 || len(snapshot.AssignmentTypes) != 8 {
		t.Fatalf("snapshot=%#v ok=%t", snapshot, ok)
	}
}

func TestGlaivePerformanceSnapshotExposesPollingAndAngleSnappingOnly(t *testing.T) {
	snapshot, ok := glaiveWorkspaceDevice().PerformanceSnapshot()
	if !ok || snapshot.PollingRate == nil || snapshot.AngleSnapping == nil || !snapshot.AngleSnapping.Enabled || snapshot.ButtonOptimization != nil || snapshot.LiftHeight != nil {
		t.Fatalf("snapshot=%#v ok=%t", snapshot, ok)
	}
}

func TestGlaiveDeviceProfileSnapshotSortsAndFailsClosedWithoutActiveProfile(t *testing.T) {
	d := glaiveWorkspaceDevice()
	snapshot, ok := d.DeviceProfileSnapshot()
	if !ok || len(snapshot.Profiles) != 2 || snapshot.Profiles[0] != "Default" || snapshot.Profiles[1] != "FPS" {
		t.Fatalf("snapshot=%#v ok=%t", snapshot, ok)
	}
	d.UserProfiles["Default"].Active = false
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("profile snapshot unexpectedly succeeded without active profile")
	}
}

func TestGlaiveDPIWorkspaceMutationRejectsIncompleteAndInvalidDrafts(t *testing.T) {
	d := glaiveWorkspaceDevice()
	if d.SelectMouseDPIStage(3) != 0 {
		t.Fatal("sniper stage was accepted as a regular selection")
	}
	d.SniperMode = true
	if d.SetMouseSniperMode(true) != 1 {
		t.Fatal("existing sniper mode was not recognized")
	}
	colors := map[int]rgb.Color{0: {Red: 0, Green: 255, Blue: 0}, 1: {Red: 0, Green: 255, Blue: 0}, 2: {Red: 0, Green: 255, Blue: 0}, 3: {Red: 255, Green: 255, Blue: 0}}
	if d.SaveMouseDPISettings(map[int]uint16{0: 800, 1: 1500, 2: 3000}, colors) != 0 {
		t.Fatal("incomplete draft unexpectedly succeeded")
	}
	if d.SaveMouseDPISettings(map[int]uint16{0: 800, 1: 1500, 2: 3000, 3: 400}, map[int]rgb.Color{0: colors[0], 1: {Red: 1, Green: 255, Blue: 0}, 2: colors[2], 3: colors[3]}) != 0 {
		t.Fatal("conflicting regular colors unexpectedly succeeded")
	}
}
