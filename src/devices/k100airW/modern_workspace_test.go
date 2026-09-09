package k100airW

import (
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/keyboards"
	"testing"
)

func TestK100AirWirelessModernWorkspaceContract(t *testing.T) {
	d := &Device{Serial: "k100airw", UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US", "DE", "FR"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard"}, SleepModes: map[int]string{1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k100AirWKeyboardFixture()}, SleepMode: 15, AutoBrightness: 1, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	assignments, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || len(assignments.Rows) != 7 || k100AirWKeyCount(assignments.Rows) != 118 || assignments.LayoutClass != "keyboard-7" || assignments.RowLayoutClass != "keyboard-row-25" || len(assignments.ModifierOptions) != 2 {
		t.Fatalf("assignments=%#v ok=%t", assignments, ok)
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || performance.PollingRate != nil || len(performance.BooleanSettings) != 4 || !performance.BooleanSettings[0].Enabled {
		t.Fatalf("performance=%#v ok=%t", performance, ok)
	}
	sleep, ok := d.SleepTimerSnapshot()
	if !ok || sleep.Value != 15 || len(sleep.Options) != 6 {
		t.Fatalf("sleep=%#v ok=%t", sleep, ok)
	}
	setting, ok := d.BooleanSettingsSnapshot()
	if !ok || len(setting.Settings) != 1 || setting.Settings[0].Action != "auto-brightness" || !setting.Settings[0].Value {
		t.Fatalf("setting=%#v ok=%t", setting, ok)
	}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete {
		t.Fatalf("profiles=%#v ok=%t", profiles, ok)
	}
}

func TestK100AirWirelessModernWorkspaceFailsClosed(t *testing.T) {
	d := &Device{UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, SleepModes: map[int]string{15: "15 minutes"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k100AirWKeyboardFixture()}, SleepMode: 15}}
	d.DeviceProfile.Keyboards["Default"].Row[1] = keyboards.Row{Keys: map[int]keyboards.Key{1: {KeyName: "Duplicate"}}}
	if _, ok := d.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("accepted duplicate key")
	}
	d.DeviceProfile.Keyboards["Default"] = k100AirWKeyboardFixture()
	d.SleepModes[15] = ""
	if _, ok := d.SleepTimerSnapshot(); ok {
		t.Fatal("accepted blank sleep option")
	}
}

func k100AirWKeyboardFixture() *keyboards.Keyboard {
	k := &keyboards.Keyboard{Row: make(map[int]keyboards.Row, 7)}
	id := 1
	for row, count := range []int{17, 17, 17, 17, 17, 17, 16} {
		keys := map[int]keyboards.Key{}
		for col := 0; col < count; col++ {
			keys[id] = keyboards.Key{KeyName: "Key"}
			if id == 1 {
				keys[id] = keyboards.Key{KeyName: "Ctrl", KeyNameInternal: "Ctrl", Modifier: true}
			}
			id++
		}
		k.Row[row] = keyboards.Row{Keys: keys}
	}
	return k
}
func k100AirWKeyCount(rows []keyboardassignmentspresentation.Row) int {
	total := 0
	for _, row := range rows {
		total += len(row.Keys)
	}
	return total
}
