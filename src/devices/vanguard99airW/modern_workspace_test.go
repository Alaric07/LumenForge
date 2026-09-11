package vanguard99airW

import (
	"LumenForge/src/config"
	"LumenForge/src/keyboards"
	"LumenForge/src/rgb"
	"path/filepath"
	"testing"
)

func TestVanguard99AirWModernWorkspaceContract(t *testing.T) {
	initializeVanguard99AirKeyboardFixtures(t)
	keyboard := keyboards.GetKeyboard(defaultLayout)
	if keyboard == nil {
		t.Fatal("missing shipped VANGUARD 99 AIR layout")
	}
	d := &Device{Serial: "wireless", UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-22", Layouts: []string{"US"}, KeyAssignmentTypes: vanguard99AirTestAssignments(), SleepModes: vanguard99AirTestSleepModes(), ControlDialOptions: vanguard99AirWTestDialModes(), FlashTapModes: vanguard99AirTestFlashTapModes(), DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, SleepMode: 15, ControlDial: 1, DisableWinKey: true, DisableShiftTab: true, DisableAltTab: true, DisableAltF4: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	a, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || a.LayoutClass != "keyboard-6" || a.RowLayoutClass != "keyboard-row-22" || len(a.Rows) == 0 || len(a.ModifierOptions) < 2 || a.LiveRGBAvailable {
		t.Fatalf("assignments=%#v ok=%t", a, ok)
	}
	p, ok := d.PerformanceSnapshot()
	if !ok || p.PollingRate != nil || len(p.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v ok=%t", p, ok)
	}
	s, ok := d.SleepTimerSnapshot()
	if !ok || len(s.Options) != 6 {
		t.Fatalf("sleep=%#v ok=%t", s, ok)
	}
	for i, id := range []int{1, 5, 10, 15, 30, 60} {
		if s.Options[i].Value != id || s.Options[i].Label != d.SleepModes[id] {
			t.Fatalf("sleep=%#v", s)
		}
	}
	dial, ok := d.ControlDialSnapshot()
	if !ok || len(dial.Options) != 7 || dial.Options[2].Label != "Vertical Scroll" {
		t.Fatalf("dial=%#v ok=%t", dial, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
	if _, ok := interface{}(d).(interface{ OptionColorsSnapshot() }); ok {
		t.Fatal("wireless variant must not advertise option colors")
	}
}

func TestVanguard99AirWFlashTapSnapshotFlattensMultipleRows(t *testing.T) {
	testVanguard99AirWFlashTap(t)
}
func testVanguard99AirWFlashTap(t *testing.T) {
	t.Helper()
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", KeyData: []uint16{4}}, 2: {KeyName: "No data"}}}, 1: {Keys: map[int]keyboards.Key{1: {KeyName: "D", KeyData: []uint16{7}}, 2: {KeyName: "Fn", KeyData: []uint16{130}}, 3: {KeyName: "Only color", OnlyColor: true, KeyData: []uint16{8}}}}}}
	d := &Device{Serial: "wireless", DeviceProfile: &DeviceProfile{Profile: "default", Keyboards: map[string]*keyboards.Keyboard{"default": keyboard}, FlashTap: &keyboards.FlashTap{Active: 1, Mode: 1, Keys: map[int]keyboards.FlashTapKey{0: {Name: "A", KeyData: 4}, 1: {Name: "D", KeyData: 7}}, Color: rgb.Color{}}}, FlashTapModes: vanguard99AirTestFlashTapModes()}
	s, ok := d.FlashTapSnapshot()
	if !ok || len(s.Keys) != 5 || len(s.SelectedSlots) != 2 {
		t.Fatalf("snapshot=%#v ok=%t", s, ok)
	}
	wantIndexes := make(map[string]int, len(s.Keys))
	for _, key := range s.Keys {
		wantIndexes[key.KeyName] = key.KeyIndex
	}
	for run := 0; run < 8; run++ {
		snapshot, ok := d.FlashTapSnapshot()
		if !ok || len(snapshot.Keys) != len(wantIndexes) {
			t.Fatalf("run %d snapshot=%#v ok=%t", run, snapshot, ok)
		}
		for _, key := range snapshot.Keys {
			if wantIndexes[key.KeyName] != key.KeyIndex {
				t.Fatalf("run %d key %q index=%d want=%d", run, key.KeyName, key.KeyIndex, wantIndexes[key.KeyName])
			}
		}
	}
	seen := map[int]bool{}
	for _, key := range s.Keys {
		if seen[key.KeyIndex] {
			t.Fatalf("duplicate flat index: %#v", s.Keys)
		}
		seen[key.KeyIndex] = true
		if key.KeyName == "No data" || key.KeyName == "Fn" || key.KeyName == "Only color" {
			if key.Eligible {
				t.Fatalf("excluded key became eligible: %#v", key)
			}
		}
	}
	if s.Keys[0].KeyIndex == s.Keys[2].KeyIndex {
		t.Fatal("row-local key index collision")
	}
}
func initializeVanguard99AirKeyboardFixtures(t *testing.T) {
	t.Helper()
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	app := filepath.Clean(filepath.Join(root, "..", "..", ".."))
	t.Setenv("LUMENFORGE_SERVICE_MODE", string(config.ServiceModeUser))
	t.Setenv("LUMENFORGE_APPLICATION_ROOT", app)
	t.Setenv("LUMENFORGE_CONFIG_ROOT", filepath.Join(t.TempDir(), "config"))
	t.Setenv("LUMENFORGE_DATA_ROOT", filepath.Join(t.TempDir(), "data"))
	config.Init()
	keyboards.Init()
}
func vanguard99AirTestAssignments() map[int]string {
	return map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}
}
func vanguard99AirTestSleepModes() map[int]string {
	return map[int]string{1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}
}
func vanguard99AirWTestDialModes() map[int]string {
	return map[int]string{1: "Volume Control", 2: "Brightness", 3: "Vertical Scroll", 4: "Zoom", 5: "Screen Brightness", 6: "Media Control", 7: "Horizontal Scroll"}
}
func vanguard99AirTestFlashTapModes() map[int]string {
	return map[int]string{0: "Neutral", 1: "Last Priority", 2: "First Priority"}
}
