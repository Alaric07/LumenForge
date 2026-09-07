package k70lux

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestKeyboardAssignmentsSnapshotK70LUX(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", Width: 1}, 2: {KeyName: "Play", OnlyColor: true}}}}}
	d := &Device{UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 10: "Macro"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}}}
	s, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || len(s.Rows) != 1 || len(s.Rows[0].Keys) != 2 || s.LayoutClass != "keyboard-7" || s.RowLayoutClass != "keyboard-row-25" || s.Rows[0].Keys[1].Assignable {
		t.Fatalf("snapshot=%#v ok=%t", s, ok)
	}
}
func TestKeyboardAssignmentsSnapshotK70LUXFailsClosedForInvalidProfileMembership(t *testing.T) {
	d := &Device{UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": {Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A"}}}}}}}}
	if s, ok := d.KeyboardAssignmentsSnapshot(); ok || s.Available {
		t.Fatalf("snapshot=%#v ok=%t", s, ok)
	}
}
