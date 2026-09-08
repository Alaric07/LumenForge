package k65rgb

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestK65RGBModernWorkspaceSnapshotsUseSourceContracts(t *testing.T) {
	keyboard := k65RGBUSKeyboardFixture()
	d := &Device{Serial: "k65", UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}, PollingRates: map[int]string{0: "Not Set", 1: "1000 Hz / 1 msec", 2: "500 Hz / 2 msec", 4: "250 Hz / 4 msec", 8: "125 Hz / 8 msec"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 1, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	assignments, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || assignments.LayoutClass != "keyboard-7" || assignments.RowLayoutClass != "keyboard-row-20" || len(assignments.Rows) != 7 || len(assignments.AssignmentTypes) != len(d.KeyAssignmentTypes) || len(assignments.ModifierOptions) != 0 {
		t.Fatalf("assignments=%#v ok=%t", assignments, ok)
	}
	keys := 0
	seen := map[int]bool{}
	for _, row := range assignments.Rows {
		for _, key := range row.Keys {
			keys++
			if seen[key.KeyIndex] {
				t.Fatalf("duplicate key index %d", key.KeyIndex)
			}
			seen[key.KeyIndex] = true
		}
	}
	if keys != 92 {
		t.Fatalf("key count=%d", keys)
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || performance.PollingRate.Value != 1 || len(performance.PollingRate.Options) != 5 || len(performance.BooleanSettings) != 4 || !performance.BooleanSettings[0].Enabled {
		t.Fatalf("performance=%#v ok=%t", performance, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
}

// k65RGBUSKeyboardFixture mirrors the source layout contract that matters to
// presentation: seven actual rows and 92 stable keys. It deliberately avoids
// global keyboard-registry initialization, which is production startup work.
func k65RGBUSKeyboardFixture() *keyboards.Keyboard {
	keyboard := &keyboards.Keyboard{Row: make(map[int]keyboards.Row, 7)}
	keyIndex := 1
	for rowID, count := range []int{21, 13, 13, 13, 13, 13, 6} {
		keys := make(map[int]keyboards.Key, count)
		for column := 0; column < count; column++ {
			keys[keyIndex] = keyboards.Key{KeyName: "Key", Width: 1, Height: 1}
			keyIndex++
		}
		keyboard.Row[rowID] = keyboards.Row{Keys: keys}
	}
	keyboard.Row[0].Keys[1] = keyboards.Key{KeyName: "BTS", Width: 1, Height: 1}
	keyboard.Row[1].Keys[22] = keyboards.Key{KeyName: "A", Width: 1, Height: 1}
	return keyboard
}
func TestK65RGBModernWorkspaceSnapshotsFailClosed(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A"}}}}}
	base := &Device{UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, PollingRates: map[int]string{1: "1000 Hz"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 1}}
	base.KeyAssignmentTypes[0] = ""
	if _, ok := base.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("accepted blank assignment label")
	}
	base.KeyAssignmentTypes[0] = "None"
	base.PollingRates[1] = ""
	if _, ok := base.PerformanceSnapshot(); ok {
		t.Fatal("accepted blank polling label")
	}
	for _, profiles := range []map[string]*DeviceProfile{{"Default": {}}, {"Default": {Active: true}, "Gaming": {Active: true}}} {
		if _, ok := (&Device{UserProfiles: profiles}).DeviceProfileSnapshot(); ok {
			t.Fatal("accepted invalid active profile state")
		}
	}
}
