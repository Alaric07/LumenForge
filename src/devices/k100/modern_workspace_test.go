package k100

import (
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/keyboards"
	"LumenForge/src/rgb"
	"reflect"
	"testing"
)

func TestK100ModernWorkspaceContract(t *testing.T) {
	keyboard := k100TestKeyboard(165)
	if keyboard.Rows != 6 {
		t.Fatalf("test keyboard scalar rows = %d", keyboard.Rows)
	}
	// The actual row map, not the stale scalar Rows value, controls geometry.
	keyboard.Row[0].Keys[1] = keyboards.Key{KeyName: "Ctrl", Modifier: true}
	polling := map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}
	debounce := map[int]string{1: "1ms", 2: "2ms", 3: "3ms", 4: "4ms", 5: "5ms", 6: "6ms", 7: "7ms", 8: "8ms", 9: "9ms"}
	modes := map[int]string{1: "Volume Control", 2: "Brightness", 3: "Vertical Scroll", 4: "Zoom", 5: "Screen Brightness", 6: "Media Control", 7: "Horizontal Scroll"}
	colors := map[int]*rgb.Color{}
	for id := range modes {
		colors[id] = &rgb.Color{Red: 255}
	}
	assignmentTypes := map[int]string{0: "None", 1: "Media Keys", 2: "DPI +", 3: "Keyboard", 4: "DPI -", 8: "Sniper", 9: "Mouse", 10: "Macro", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}
	d := &Device{Serial: "k100", UIKeyboard: "keyboard-8", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US", "DE", "FR", "SE", "UK"}, KeyAssignmentTypes: assignmentTypes, PollingRates: polling, DebounceTimes: debounce, ControlDialOptions: modes, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DebounceTime: 1, ControlDial: 1, ControlDialColors: colors, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	a, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || len(a.Rows) != 8 || len(a.ModifierOptions) != 0 {
		t.Fatalf("assignments=%#v ok=%t", a, ok)
	}
	gotAssignmentTypes := map[int]string{}
	for _, assignmentType := range a.AssignmentTypes {
		gotAssignmentTypes[int(assignmentType.ID)] = assignmentType.Label
	}
	if !reflect.DeepEqual(gotAssignmentTypes, assignmentTypes) {
		t.Fatalf("assignment types = %#v", gotAssignmentTypes)
	}
	d.DeviceProfile.Layout = "DE"
	d.DeviceProfile.Keyboards["Default"] = k100TestKeyboard(166)
	if alternate, ok := d.KeyboardAssignmentsSnapshot(); !ok || len(alternate.Rows) != 8 || k100KeyCount(alternate) != 166 {
		t.Fatalf("alternate assignments = %#v, ok = %t", alternate, ok)
	}
	d.DeviceProfile.Layout = "US"
	d.DeviceProfile.Keyboards["Default"] = keyboard
	p, ok := d.PerformanceSnapshot()
	if !ok || p.PollingRate == nil || len(p.PollingRate.Options) != 8 || p.DebounceTime == nil || len(p.DebounceTime.Options) != 9 || len(p.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v", p)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || profiles.CanDelete {
		t.Fatalf("profiles=%#v", profiles)
	}
	dial, ok := d.ControlDialSnapshot()
	if !ok || len(dial.Options) != 7 {
		t.Fatalf("dial=%#v", dial)
	}
	optionColors, ok := d.OptionColorsSnapshot()
	if !ok || optionColors.Selected != 1 || len(optionColors.Options) != 7 || optionColors.Options[0].Action != "control-dial-colors" {
		t.Fatalf("colors=%#v", optionColors)
	}
	d.DeviceProfile.PollingRate = 99
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("accepted stale polling rate")
	}
	d.DeviceProfile.PollingRate = 4
	d.DeviceProfile.ControlDial = 99
	if _, ok := d.OptionColorsSnapshot(); ok {
		t.Fatal("accepted stale control dial color selection")
	}
}

func k100TestKeyboard(keyCount int) *keyboards.Keyboard {
	keyboard := &keyboards.Keyboard{Rows: 6, Row: map[int]keyboards.Row{}}
	id := 1
	for row := 0; row < 8; row++ {
		keys := map[int]keyboards.Key{}
		for column := 0; column < 21 && id <= keyCount; column++ {
			keys[id] = keyboards.Key{KeyName: "Key"}
			id++
		}
		keyboard.Row[row] = keyboards.Row{Keys: keys}
	}
	return keyboard
}

func k100KeyCount(snapshot keyboardassignmentspresentation.Snapshot) int {
	count := 0
	for _, row := range snapshot.Rows {
		count += len(row.Keys)
	}
	return count
}
