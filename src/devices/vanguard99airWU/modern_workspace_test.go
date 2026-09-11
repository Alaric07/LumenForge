package vanguard99airWU

import (
	"LumenForge/src/config"
	"LumenForge/src/keyboards"
	"LumenForge/src/rgb"
	"path/filepath"
	"testing"
)

func TestVanguard99AirWUModernWorkspaceContract(t *testing.T) {
	initializeVanguard99AirKeyboardFixtures(t)
	keyboard := keyboards.GetKeyboard(defaultLayout)
	if keyboard == nil {
		t.Fatal("missing shipped VANGUARD 99 AIR layout")
	}
	colors := map[int]*rgb.Color{}
	for id := range vanguard99AirWUTestDialModes() {
		colors[id] = &rgb.Color{Red: float64(id)}
	}
	d := &Device{Serial: "usb", UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-22", Layouts: []string{"US"}, KeyAssignmentTypes: vanguard99AirWUTestAssignments(), PollingRates: vanguard99AirWUTestPolling(), ControlDialOptions: vanguard99AirWUTestDialModes(), FlashTapModes: vanguard99AirWUTestFlashTapModes(), DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, ControlDial: 1, ControlDialColors: colors, DisableWinKey: true, DisableShiftTab: true, DisableAltTab: true, DisableAltF4: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	a, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || a.LayoutClass != "keyboard-6" || a.RowLayoutClass != "keyboard-row-22" || len(a.ModifierOptions) < 2 || a.LiveRGBAvailable {
		t.Fatalf("assignments=%#v ok=%t", a, ok)
	}
	p, ok := d.PerformanceSnapshot()
	if !ok || p.PollingRate == nil || len(p.PollingRate.Options) != 8 || len(p.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v ok=%t", p, ok)
	}
	for id := 0; id <= 7; id++ {
		if p.PollingRate.Options[id].Value != id || p.PollingRate.Options[id].Label != d.PollingRates[id] {
			t.Fatalf("polling=%#v", p.PollingRate)
		}
	}
	dial, ok := d.ControlDialSnapshot()
	if !ok || len(dial.Options) != 7 || dial.Options[2].Label != "Scroll" {
		t.Fatalf("dial=%#v ok=%t", dial, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
	optionColors, ok := d.OptionColorsSnapshot()
	if !ok || optionColors.Selected != 1 || len(optionColors.Options) != 7 || optionColors.Options[0].Action != "control-dial-colors" {
		t.Fatalf("colors=%#v ok=%t", optionColors, ok)
	}
}
func TestVanguard99AirWUFlashTapSnapshotFlattensMultipleRows(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", KeyData: []uint16{4}}, 2: {KeyName: "No data"}}}, 1: {Keys: map[int]keyboards.Key{1: {KeyName: "D", KeyData: []uint16{7}}, 2: {KeyName: "Fn", KeyData: []uint16{130}}, 3: {KeyName: "Only color", OnlyColor: true, KeyData: []uint16{8}}}}}}
	d := &Device{Serial: "usb", DeviceProfile: &DeviceProfile{Profile: "default", Keyboards: map[string]*keyboards.Keyboard{"default": keyboard}, FlashTap: &keyboards.FlashTap{Active: 1, Mode: 1, Keys: map[int]keyboards.FlashTapKey{0: {Name: "A", KeyData: 4}, 1: {Name: "D", KeyData: 7}}, Color: rgb.Color{}}}, FlashTapModes: vanguard99AirWUTestFlashTapModes()}
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
func vanguard99AirWUTestAssignments() map[int]string {
	return map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}
}
func vanguard99AirWUTestPolling() map[int]string {
	return map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}
}
func vanguard99AirWUTestDialModes() map[int]string {
	return map[int]string{1: "Volume Control", 2: "Brightness", 3: "Scroll", 4: "Zoom", 5: "Screen Brightness", 6: "Media Control", 7: "Horizontal Scroll"}
}
func vanguard99AirWUTestFlashTapModes() map[int]string {
	return map[int]string{0: "Neutral", 1: "Last Priority", 2: "First Priority"}
}
