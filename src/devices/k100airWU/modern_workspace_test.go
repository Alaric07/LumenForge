package k100airWU

import (
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/keyboards"
	"testing"
)

func TestK100AirUSBModernWorkspaceContract(t *testing.T) {
	polling := map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}
	d := &Device{Serial: "k100airu", UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US", "DE", "FR"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard"}, PollingRates: polling, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k100AirWUKeyboardFixture()}, PollingRate: 4, AutoBrightness: 1, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	assignments, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || len(assignments.Rows) != 7 || k100AirWUKeyCount(assignments.Rows) != 118 || len(assignments.ModifierOptions) != 2 {
		t.Fatalf("assignments=%#v ok=%t", assignments, ok)
	}
	performance, ok := d.PerformanceSnapshot()
	if !ok || performance.PollingRate == nil || performance.PollingRate.Value != 4 || len(performance.PollingRate.Options) != 8 || len(performance.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v ok=%t", performance, ok)
	}
	for id, option := range performance.PollingRate.Options {
		if option.Value != id || option.Label != polling[id] {
			t.Fatalf("polling=%#v", performance.PollingRate.Options)
		}
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

func TestK100AirUSBModernWorkspaceFailsClosed(t *testing.T) {
	d := &Device{UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, PollingRates: map[int]string{4: "1000 Hz"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k100AirWUKeyboardFixture()}, PollingRate: 4}}
	d.PollingRates[4] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("accepted blank polling option")
	}
	d.PollingRates[4] = "1000 Hz"
	d.DeviceProfile.PollingRate = 7
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("accepted unsupported selected polling rate")
	}
}

func k100AirWUKeyboardFixture() *keyboards.Keyboard {
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
func k100AirWUKeyCount(rows []keyboardassignmentspresentation.Row) int {
	total := 0
	for _, row := range rows {
		total += len(row.Keys)
	}
	return total
}
