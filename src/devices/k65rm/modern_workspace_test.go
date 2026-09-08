package k65rm

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestK65RGBMiniModernWorkspaceSnapshots(t *testing.T) {
	k := k65RMMiniKeyboard(61)
	d := &Device{Serial: "mini", UIKeyboard: "keyboard-5", UIKeyboardRow: "keyboard-row-16", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz", 2: "250 Hz", 3: "500 Hz", 4: "1000 Hz", 5: "2000 Hz", 6: "4000 Hz", 7: "8000 Hz"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, PollingRate: 4, DisableWinKey: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	s, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || s.LayoutClass != "keyboard-5" || s.RowLayoutClass != "keyboard-row-16" || len(s.Rows) != 5 || len(s.ModifierOptions) != 2 || s.Rows[1].Keys[0].ModifierKey != 1 || !s.Rows[1].Keys[0].RetainOriginal {
		t.Fatalf("assignments=%#v", s)
	}
	count := 0
	for _, row := range s.Rows {
		count += len(row.Keys)
	}
	if count != 61 {
		t.Fatalf("keys=%d", count)
	}
	if p, ok := d.PerformanceSnapshot(); !ok || p.PollingRate.Value != 4 || len(p.PollingRate.Options) != 8 || len(p.BooleanSettings) != 4 {
		t.Fatalf("performance=%#v", p)
	}
	if p, ok := d.DeviceProfileSnapshot(); !ok || p.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v", p)
	}
}
func TestK65RGBMiniSnapshotFailsClosedForMalformedModifier(t *testing.T) {
	k := k65RMMiniKeyboard(61)
	k.Row[0].Keys[1] = keyboards.Key{Modifier: true}
	d := &Device{UIKeyboard: "keyboard-5", UIKeyboardRow: "keyboard-row-16", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}}}
	if _, ok := d.KeyboardAssignmentsSnapshot(); ok {
		t.Fatal("accepted blank modifier")
	}
}
func k65RMMiniKeyboard(count int) *keyboards.Keyboard {
	k := &keyboards.Keyboard{Row: map[int]keyboards.Row{}}
	id := 1
	for row := 0; row < 5; row++ {
		keys := map[int]keyboards.Key{}
		for col := 0; col < 14 && id <= count; col++ {
			keys[id] = keyboards.Key{KeyName: "Key", Width: 1, Height: 1}
			id++
		}
		k.Row[row] = keyboards.Row{Keys: keys}
	}
	k.Row[0].Keys[1] = keyboards.Key{KeyName: "Fn", KeyNameInternal: "Function", Modifier: true}
	k.Row[1].Keys[15] = keyboards.Key{KeyName: "A", ModifierKey: 1, RetainOriginal: true}
	return k
}
