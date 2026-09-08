package k65plusW

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestK65PlusWirelessWorkspaceProviders(t *testing.T) {
	k := plusKeyboard()
	d := &Device{Serial: "k65", UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-17", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}, ControlDialOptions: map[int]string{1: "Volume Control", 2: "Brightness"}, SleepModes: map[int]string{5: "5 minutes", 15: "15 minutes"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, ControlDial: 2, SleepMode: 15, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	a, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || a.LayoutClass != "keyboard-6" || a.RowLayoutClass != "keyboard-row-17" || len(a.Rows) != 6 || len(a.ModifierOptions) != 2 || a.ModifierOptions[0].Label != "None" || a.Rows[1].Keys[0].ModifierKey != 1 || !a.Rows[1].Keys[0].RetainOriginal {
		t.Fatalf("assignments=%#v", a)
	}
	n := 0
	for _, r := range a.Rows {
		n += len(r.Keys)
	}
	if n != 81 {
		t.Fatalf("keys=%d", n)
	}
	if p, ok := d.PerformanceSnapshot(); !ok || p.PollingRate != nil || len(p.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v", p)
	}
	if p, ok := d.DeviceProfileSnapshot(); !ok || p.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v", p)
	}
	if c, ok := d.ControlDialSnapshot(); !ok || c.Value != 2 || len(c.Options) != 2 {
		t.Fatalf("dial=%#v", c)
	}
	if s, ok := d.SleepTimerSnapshot(); !ok || s.Value != 15 || len(s.Options) != 2 {
		t.Fatalf("sleep=%#v", s)
	}
}
func TestK65PlusWirelessProvidersFailClosed(t *testing.T) {
	k := plusKeyboard()
	d := &Device{UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-17", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, ControlDial: 3, SleepMode: 3}, ControlDialOptions: map[int]string{1: "Volume"}, SleepModes: map[int]string{15: "15 minutes"}}
	if _, ok := d.KeyboardAssignmentsSnapshot(); !ok {
		t.Fatal("assignment base")
	}
	k.Row[1].Keys[15] = keyboards.Key{KeyName: "A", ModifierKey: 9}
	if _, ok := d.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("modifier")
	}
	if _, ok := d.ControlDialSnapshot(); ok {
		t.Fatal("dial")
	}
	if _, ok := d.SleepTimerSnapshot(); ok {
		t.Fatal("sleep")
	}
}
func plusKeyboard() *keyboards.Keyboard {
	k := &keyboards.Keyboard{Row: map[int]keyboards.Row{}}
	id := 1
	for r := 0; r < 6; r++ {
		m := map[int]keyboards.Key{}
		for c := 0; c < 14 && id <= 81; c++ {
			m[id] = keyboards.Key{KeyName: "Key"}
			id++
		}
		k.Row[r] = keyboards.Row{Keys: m}
	}
	k.Row[0].Keys[1] = keyboards.Key{KeyName: "Fn", KeyNameInternal: "Function", Modifier: true}
	k.Row[1].Keys[15] = keyboards.Key{KeyName: "A", ModifierKey: 1, RetainOriginal: true}
	return k
}
