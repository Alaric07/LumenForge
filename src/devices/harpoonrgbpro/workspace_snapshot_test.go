package harpoonrgbpro

import (
	"reflect"
	"testing"

	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
)

func harpoonWorkspaceDevice() *Device {
	assignments := map[int]inputmanager.KeyAssignment{}
	for index, key := range harpoonVisibleButtonOrder {
		assignments[key] = inputmanager.KeyAssignment{Name: []string{"Left Button", "Right Button", "Middle Button", "Back Button", "Forward Button", "DPI Button"}[index], Default: key != 8, ActionType: 3, ActionCommand: uint16(index)}
	}
	assignments[8] = inputmanager.KeyAssignment{Name: "Back Button", ActionHold: true, OnRelease: true, ActionType: 10, ActionCommand: 42, IsMacro: true}
	return &Device{
		Serial: "harpoon-workspace", MinDPI: 200, MaxDPI: 12000,
		DeviceProfile: &DeviceProfile{Profile: 1, PollingRate: 2, Profiles: map[int]DPIProfile{
			2: {Name: "Sniper", Value: 200, Color: &rgb.Color{Red: 255, Green: 170}, Sniper: true},
			0: {Name: "", Value: 800, Color: &rgb.Color{Red: 1, Green: 2, Blue: 3}},
			1: {Name: "Stage 2", Value: 1600, Color: &rgb.Color{Red: 16, Green: 32, Blue: 48}},
		}},
		KeyAssignment:      assignments,
		KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 2: "DPI", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Profile Switch"},
		PollingRates:       map[int]string{8: "125 Hz", 1: "1000 Hz", 2: "500 Hz", 4: "250 Hz", 0: "Not Set"},
		UserProfiles:       map[string]*DeviceProfile{"studio": {}, "default": {Active: true}, "missing": nil},
	}
}

func TestHarpoonDPISnapshot(t *testing.T) {
	d := harpoonWorkspaceDevice()
	snapshot, ok := d.DPISnapshot()
	if !ok || snapshot.MinimumDPI != 200 || snapshot.MaximumDPI != 12000 || snapshot.ActiveRegularStageID != "1" || len(snapshot.Stages) != 3 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	if snapshot.Stages[0].ID != "0" || snapshot.Stages[0].Name != "Stage 1" || snapshot.Stages[0].ColorHex != "#010203" || !snapshot.Stages[1].Active || !snapshot.Stages[2].Sniper || snapshot.Stages[2].ColorHex != "#ffaa00" {
		t.Fatalf("stages = %#v", snapshot.Stages)
	}
	d.DeviceProfile = nil
	if _, ok := d.DPISnapshot(); ok {
		t.Fatal("snapshot succeeded without profile")
	}
}

func TestHarpoonButtonsSnapshot(t *testing.T) {
	snapshot, ok := harpoonWorkspaceDevice().ButtonsSnapshot()
	if !ok || len(snapshot.Buttons) != 6 || len(snapshot.AssignmentTypes) != 8 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	if got := []int{snapshot.Buttons[0].KeyIndex, snapshot.Buttons[1].KeyIndex, snapshot.Buttons[2].KeyIndex, snapshot.Buttons[3].KeyIndex, snapshot.Buttons[4].KeyIndex, snapshot.Buttons[5].KeyIndex}; !reflect.DeepEqual(got, harpoonVisibleButtonOrder) {
		t.Fatalf("button IDs = %v", got)
	}
	if state := snapshot.Buttons[3]; state.Default || !state.PressAndHold || !state.OnRelease || state.ActionType != 10 || state.ActionCommand != 42 || !state.IsMacro {
		t.Fatalf("button state = %#v", state)
	}
	if got := []uint8{snapshot.AssignmentTypes[0].ID, snapshot.AssignmentTypes[1].ID, snapshot.AssignmentTypes[2].ID, snapshot.AssignmentTypes[3].ID, snapshot.AssignmentTypes[4].ID, snapshot.AssignmentTypes[5].ID, snapshot.AssignmentTypes[6].ID, snapshot.AssignmentTypes[7].ID}; !reflect.DeepEqual(got, []uint8{0, 1, 2, 3, 8, 9, 10, 11}) {
		t.Fatalf("assignment types = %v", got)
	}
	d := harpoonWorkspaceDevice()
	delete(d.KeyAssignment, 32)
	if _, ok := d.ButtonsSnapshot(); ok {
		t.Fatal("snapshot succeeded with missing physical button")
	}
}

func TestHarpoonPerformanceAndDeviceProfilesSnapshots(t *testing.T) {
	d := harpoonWorkspaceDevice()
	performance, ok := d.PerformanceSnapshot()
	if !ok || performance.PollingRate == nil || performance.PollingRate.Value != 2 || performance.AngleSnapping != nil || performance.LiftHeight != nil || len(performance.BooleanSettings) != 0 {
		t.Fatalf("performance = %#v, ok=%t", performance, ok)
	}
	if got := []int{performance.PollingRate.Options[0].Value, performance.PollingRate.Options[1].Value, performance.PollingRate.Options[2].Value, performance.PollingRate.Options[3].Value, performance.PollingRate.Options[4].Value}; !reflect.DeepEqual(got, []int{0, 1, 2, 4, 8}) {
		t.Fatalf("polling options = %v", got)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.Supported || profiles.ActiveProfile != "default" || !reflect.DeepEqual(profiles.Profiles, []string{"default", "studio"}) {
		t.Fatalf("profiles = %#v, ok=%t", profiles, ok)
	}
	d.DeviceProfile = nil
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("performance succeeded without profile")
	}
	d.UserProfiles = map[string]*DeviceProfile{"default": {}}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("profiles succeeded without active profile")
	}
}
