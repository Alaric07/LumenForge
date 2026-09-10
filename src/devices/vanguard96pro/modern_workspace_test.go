package vanguard96pro

import (
	"LumenForge/src/keyboards"
	"LumenForge/src/rgb"
	"testing"
)

func vanguard96ProWorkspaceDevice() *Device {
	keyboard := &keyboards.Keyboard{Key: keyboardKey, Layout: "US", Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{4: {KeyName: "A", KeyData: []uint16{4}, ActuationPoint: 20, ActuationResetPoint: 19, EnableActuationPointReset: true, EnableSecondaryActuationPoint: true, SecondaryActuationPoint: 35, SecondaryActuationResetPoint: 34}, 7: {KeyName: "D", KeyData: []uint16{7}, ActuationPoint: 20, ActuationResetPoint: 19}, 130: {KeyName: "Fn", KeyData: []uint16{130}, Modifier: true}}}}}
	profile := &DeviceProfile{Profile: "default", Profiles: []string{"default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"default": keyboard}, PollingRate: 4, ControlDial: 1, DisableWinKey: true, DisableShiftTab: true, DisableAltTab: true, DisableAltF4: true, FlashTap: &keyboards.FlashTap{Active: 1, Mode: 1, Keys: map[int]keyboards.FlashTapKey{0: {Name: "A", KeyData: 4}, 1: {Name: "D", KeyData: 7}}, Color: rgb.Color{Red: 19, Green: 97, Blue: 203}}}
	return &Device{Serial: "vanguard96pro", UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-21", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Brightness +", 12: "Brightness -", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}, ControlDialOptions: map[int]string{1: "Volume Control", 2: "Brightness", 3: "Scroll", 4: "Zoom", 5: "Screen Brightness", 6: "Media Control", 7: "Horizontal Scroll"}, FlashTapModes: map[int]string{0: "Neutral", 1: "Last Priority", 2: "First Priority"}, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"default": {Active: true}, "Gaming": {}}}
}

func TestVanguard96ProModernWorkspaceContract(t *testing.T) {
	d := vanguard96ProWorkspaceDevice()
	if keyboardKey != "vanguard96-default" || defaultLayout != "vanguard96-default-US" {
		t.Fatalf("source keyboard constants = %q %q", keyboardKey, defaultLayout)
	}
	assignments, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || assignments.LayoutClass != "keyboard-6" || assignments.RowLayoutClass != "keyboard-row-21" || len(assignments.Rows) != 1 || len(assignments.AssignmentTypes) != 14 || len(assignments.ModifierOptions) < 2 {
		t.Fatalf("assignments=%#v ok=%t", assignments, ok)
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || performance.PollingRate == nil || len(performance.PollingRate.Options) != 8 || len(performance.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v ok=%t", performance, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
	dial, ok := d.ControlDialSnapshot()
	if !ok || len(dial.Options) != 7 {
		t.Fatalf("dial=%#v ok=%t", dial, ok)
	}
	actuation, ok := d.KeyActuationSnapshot()
	if !ok || !actuation.Supported || len(actuation.Keys) != 3 || !actuation.Keys[0].Supported || actuation.Keys[2].Supported || actuation.Keys[0].SecondaryActuationPoint != 35 {
		t.Fatalf("actuation=%#v ok=%t", actuation, ok)
	}
	flashTap, ok := d.FlashTapSnapshot()
	if !ok || !flashTap.Supported || flashTap.Mode != 1 || len(flashTap.Modes) != 3 || len(flashTap.SelectedSlots) != 2 || flashTap.Keys[2].Eligible {
		t.Fatalf("flashTap=%#v ok=%t", flashTap, ok)
	}
}
