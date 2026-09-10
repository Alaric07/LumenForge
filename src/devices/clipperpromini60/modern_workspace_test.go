package clipperpromini60

import (
	"LumenForge/src/keyboards"
	"LumenForge/src/rgb"
	"testing"
)

func clipperProMini60WorkspaceDevice() *Device {
	keyboard := &keyboards.Keyboard{Key: keyboardKey, Layout: "US", Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{
		4:   {KeyName: "A", KeyNameInternal: "A", Width: 1, Height: 1, KeyData: []uint16{4, 57}, ActuationPoint: 20, ActuationResetPoint: 19, EnableActuationPointReset: true, EnableSecondaryActuationPoint: true, SecondaryActuationPoint: 35, SecondaryActuationResetPoint: 34},
		7:   {KeyName: "D", Width: 1, Height: 1, KeyData: []uint16{7, 57}, ActuationPoint: 20, ActuationResetPoint: 19},
		130: {KeyName: "Fn", KeyNameInternal: "Function", Width: 1, Height: 1, KeyData: []uint16{130}, Modifier: true},
	}}}}
	profile := &DeviceProfile{Profile: "default", Profiles: []string{"default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"default": keyboard}, PollingRate: 4, ControlDial: 1, DisableWinKey: true, DisableShiftTab: true, DisableAltTab: true, DisableAltF4: true, FlashTap: &keyboards.FlashTap{Active: 1, Mode: 1, Keys: map[int]keyboards.FlashTapKey{0: {Name: "A", KeyData: 4}, 1: {Name: "D", KeyData: 7}}, Color: rgb.Color{Red: 250, Green: 200, Blue: 0}}}
	return &Device{Serial: "clipper", UIKeyboard: "keyboard-5", UIKeyboardRow: "keyboard-row-16", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Brightness +", 12: "Brightness -", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}, ControlDialOptions: map[int]string{1: "Volume Control", 2: "Brightness", 3: "Scroll", 4: "Zoom", 5: "Screen Brightness", 6: "Media Control", 7: "Horizontal Scroll"}, FlashTapModes: map[int]string{0: "Neutral", 1: "Last Priority", 2: "First Priority"}, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"default": {Active: true}, "Gaming": {}}}
}

func TestClipperProMini60ModernWorkspaceContract(t *testing.T) {
	d := clipperProMini60WorkspaceDevice()
	if d.DeviceProfile.Keyboards["default"].Key != keyboardKey || d.DeviceProfile.Keyboards["default"].Layout != "US" || defaultLayout != "clipperpromini60-default-US" {
		t.Fatalf("source layout = %#v, default = %q", d.DeviceProfile.Keyboards["default"], defaultLayout)
	}
	assignments, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || assignments.LayoutClass != "keyboard-5" || assignments.RowLayoutClass != "keyboard-row-16" || len(assignments.Rows) != 1 || len(assignments.AssignmentTypes) != 14 || len(assignments.ModifierOptions) != 2 {
		t.Fatalf("assignments=%#v ok=%t", assignments, ok)
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || performance.PollingRate == nil || len(performance.PollingRate.Options) != 8 || len(performance.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v ok=%t", performance, ok)
	}
	for value, label := range map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"} {
		if performance.PollingRate.Options[value].Value != value || performance.PollingRate.Options[value].Label != label {
			t.Fatalf("polling options=%#v", performance.PollingRate.Options)
		}
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
	dial, ok := d.ControlDialSnapshot()
	if !ok || len(dial.Options) != 7 || dial.Options[0].Label != "Volume Control" || dial.Options[6].Label != "Horizontal Scroll" {
		t.Fatalf("dial=%#v ok=%t", dial, ok)
	}
	actuation, ok := d.KeyActuationSnapshot()
	if !ok || !actuation.Supported || actuation.MinValue != 1 || actuation.MaxValue != 40 || actuation.SecondaryMinimumGap != 4 || len(actuation.Keys) != 3 || !actuation.Keys[0].Supported || actuation.Keys[2].Supported || actuation.Keys[0].SecondaryActuationPoint != 35 {
		t.Fatalf("actuation=%#v ok=%t", actuation, ok)
	}
	flashTap, ok := d.FlashTapSnapshot()
	if !ok || !flashTap.Supported || !flashTap.Active || flashTap.Mode != 1 || len(flashTap.Modes) != 3 || flashTap.Modes[0].Label != "Neutral" || flashTap.Modes[1].Label != "Last Priority" || flashTap.Modes[2].Label != "First Priority" || len(flashTap.SelectedSlots) != 2 || flashTap.SelectedSlots[0].KeyIndex != 4 || flashTap.SelectedSlots[1].KeyIndex != 7 || flashTap.Keys[2].Eligible {
		t.Fatalf("flashTap=%#v ok=%t", flashTap, ok)
	}
}

func TestClipperProMini60AdvancedSnapshotsFailClosed(t *testing.T) {
	d := clipperProMini60WorkspaceDevice()
	key := d.DeviceProfile.Keyboards["default"].Row[0].Keys[4]
	key.ActuationResetPoint = key.ActuationPoint
	d.DeviceProfile.Keyboards["default"].Row[0].Keys[4] = key
	if _, ok := d.KeyActuationSnapshot(); ok {
		t.Fatal("accepted invalid actuation state")
	}
	d = clipperProMini60WorkspaceDevice()
	d.DeviceProfile.FlashTap.Keys[1] = keyboards.FlashTapKey{Name: "A", KeyData: 4}
	if _, ok := d.FlashTapSnapshot(); ok {
		t.Fatal("accepted duplicate FlashTap selection")
	}
}
