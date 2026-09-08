package k70max

import (
	"LumenForge/src/keyboards"
	"math"
	"testing"
)

func maxWorkspaceDevice(keyboard *keyboards.Keyboard) *Device {
	return &Device{Serial: "max", UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Brightness +", 12: "Brightness -", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}, PollingRates: map[int]string{4: "1000 Hz / 1 msec"}, FlashTapModes: map[int]string{0: "Neutral", 1: "Last Priority"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true, DisableAltTab: true, FlashTap: &keyboards.FlashTap{Active: 1, Mode: 1, Keys: map[int]keyboards.FlashTapKey{0: {Name: "D", KeyData: 7}, 1: {Name: "A", KeyData: 4}}}}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
}

func TestK70MaxKeyboardAssignmentsUseActualSevenRows(t *testing.T) {
	k := &keyboards.Keyboard{Row: map[int]keyboards.Row{}}
	id := 1
	for row := 0; row < 7; row++ {
		keys := map[int]keyboards.Key{}
		count := 16
		if row == 0 {
			count = 21
		}
		for col := 0; col < count && id <= 117; col++ {
			keys[id] = keyboards.Key{KeyName: "Key", Width: 1, Height: 1}
			id++
		}
		k.Row[row] = keyboards.Row{Keys: keys}
	}
	// Fill the final row to the source-backed US total and expose a layout modifier.
	for id <= 117 {
		k.Row[6].Keys[id] = keyboards.Key{KeyName: "Key", Width: 1, Height: 1}
		id++
	}
	k.Row[0].Keys[3] = keyboards.Key{KeyName: "Fn", KeyNameInternal: "Function", Width: 1, Height: 1, Modifier: true}
	d := maxWorkspaceDevice(k)
	d.DeviceProfile.Keyboards["Default"].Row[1].Keys[22] = keyboards.Key{KeyName: "A", Width: 1, Height: 1, ModifierKey: 3, RetainOriginal: true}
	s, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || s.LayoutClass != "keyboard-7" || s.RowLayoutClass != "keyboard-row-25" || len(s.Rows) != 7 {
		t.Fatalf("snapshot=%#v", s)
	}
	count := 0
	for _, row := range s.Rows {
		count += len(row.Keys)
	}
	if count != 117 || len(s.AssignmentTypes) != 14 || s.AssignmentTypes[13].ID != 18 || s.AssignmentTypes[13].Label != "Screen Brightness -" || len(s.ModifierOptions) != 2 || s.ModifierOptions[1].ID != 3 || !s.Rows[1].Keys[0].RetainOriginal {
		t.Fatalf("geometry/modifiers=%#v", s)
	}
	d.DeviceProfile.Keyboards["Default"].Row[1].Keys[22] = keyboards.Key{KeyName: "A", Width: 1, Height: 1, ModifierKey: 9}
	if _, ok := d.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("invalid modifier state accepted")
	}
}

func TestK70MaxPerformanceAndProfilesFailClosed(t *testing.T) {
	d := maxWorkspaceDevice(&keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", Width: 1, Height: 1}}}}})
	if s, ok := d.PerformanceSnapshot(); !ok || len(s.BooleanSettings) != 4 || s.PollingRate.Value != 4 {
		t.Fatalf("performance=%#v", s)
	}
	d.PollingRates[4] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("blank polling label accepted")
	}
	d.PollingRates[4] = "1000 Hz"
	if _, ok := d.DeviceProfileSnapshot(); !ok {
		t.Fatal("one active profile rejected")
	}
	d.UserProfiles["Gaming"] = &DeviceProfile{Active: true}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("multiple active profiles accepted")
	}
	d.UserProfiles["Gaming"].Active = false
	d.UserProfiles["Default"].Active = false
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("zero active profiles accepted")
	}
}

func TestK70MaxAdvancedSnapshotsPreserveSlotsAndRejectMalformedState(t *testing.T) {
	k := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{4: {KeyName: "A", Width: 1, Height: 1, KeyData: []uint16{4}, ActuationPoint: 20, ActuationResetPoint: 18, EnableActuationPointReset: true, EnableSecondaryActuationPoint: true, SecondaryActuationPoint: 30, SecondaryActuationResetPoint: 29}, 7: {KeyName: "D", Width: 1, Height: 1, KeyData: []uint16{7}, ActuationPoint: 20, ActuationResetPoint: 18}, 9: {KeyName: "Logo", Width: 1, Height: 1, KeyData: []uint16{128}}}}}}
	d := maxWorkspaceDevice(k)
	d.DeviceProfile.FlashTap.Color.Red, d.DeviceProfile.FlashTap.Color.Green, d.DeviceProfile.FlashTap.Color.Blue = 17, 93, 201
	actuation, ok := d.KeyActuationSnapshot()
	if !ok || !actuation.Keys[0].Supported || actuation.Keys[2].Supported {
		t.Fatalf("actuation=%#v", actuation)
	}
	flashTap, ok := d.FlashTapSnapshot()
	if !ok || flashTap.SelectedSlots[0].KeyIndex != 7 || flashTap.SelectedSlots[1].KeyIndex != 4 || flashTap.Color.Red != 17 || flashTap.Keys[2].Eligible {
		t.Fatalf("flashTap=%#v", flashTap)
	}
	k.Row[0].Keys[4] = keyboards.Key{KeyName: "A", Width: 1, Height: 1, KeyData: []uint16{4}, ActuationPoint: 20, ActuationResetPoint: 20}
	if _, ok := d.KeyActuationSnapshot(); ok {
		t.Fatal("invalid actuation relation accepted")
	}
	k.Row[0].Keys[4] = keyboards.Key{KeyName: "A", Width: 1, Height: 1, KeyData: []uint16{4}, ActuationPoint: 20, ActuationResetPoint: 18}
	d.DeviceProfile.FlashTap.Color.Red = math.NaN()
	if _, ok := d.FlashTapSnapshot(); ok {
		t.Fatal("non-finite color accepted")
	}
	d.DeviceProfile.FlashTap.Color.Red = 17
	d.DeviceProfile.FlashTap.Mode = 9
	if _, ok := d.FlashTapSnapshot(); ok {
		t.Fatal("unknown FlashTap mode accepted")
	}
	d.DeviceProfile.FlashTap.Mode = 1
	d.DeviceProfile.FlashTap.Keys[1] = keyboards.FlashTapKey{Name: "D", KeyData: 7}
	if _, ok := d.FlashTapSnapshot(); ok {
		t.Fatal("duplicate FlashTap selection accepted")
	}
}
