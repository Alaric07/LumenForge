package k95platinumXT

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestKeyboardAssignmentsSnapshotPreservesK95PlatinumXTGeometryAndKeyMetadata(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{3: {Top: 65, Css: "authored-row", Keys: map[int]keyboards.Key{9: {KeyName: "G1", Width: 2, Height: 1, ActionType: 10, ActionCommand: 7}, 1: {KeyName: "A", Width: 1, Height: 1, OnlyColor: true}}}}}
	d := &Device{Serial: "xt", UIKeyboard: "keyboard-8", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{10: "Macro", 0: "None"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}}}
	s, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || s.LayoutClass != "keyboard-8" || s.RowLayoutClass != "keyboard-row-26" || len(s.Rows) != 1 || s.Rows[0].Top != 65 || s.Rows[0].Keys[1].KeyName != "G1" || s.Rows[0].Keys[1].ActionCommand != 7 || len(s.AssignmentTypes) != 2 || s.AssignmentTypes[1].ID != 10 {
		t.Fatalf("snapshot = %#v, ok=%t", s, ok)
	}
}

func TestKeyboardAssignmentsSnapshotFailsClosedForIncompleteK95PlatinumXTData(t *testing.T) {
	validKeyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", Width: 1, Height: 1}}}}}
	for _, d := range []*Device{{}, {DeviceProfile: &DeviceProfile{Profile: "missing", Layout: "US", Keyboards: map[string]*keyboards.Keyboard{}}, Layouts: []string{"US"}, UIKeyboard: "keyboard-8", UIKeyboardRow: "keyboard-row-26", KeyAssignmentTypes: map[int]string{0: "None"}}, {DeviceProfile: &DeviceProfile{Profile: "Default", Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": validKeyboard}}, UIKeyboard: "keyboard-8", UIKeyboardRow: "keyboard-row-26", KeyAssignmentTypes: map[int]string{0: "None"}}, {DeviceProfile: &DeviceProfile{Profile: "Default", Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": {Row: map[int]keyboards.Row{0: {}}}}}, Layouts: []string{"US"}, UIKeyboard: "keyboard-8", UIKeyboardRow: "keyboard-row-26", KeyAssignmentTypes: map[int]string{0: "None"}}} {
		if s, ok := d.KeyboardAssignmentsSnapshot(); ok || s.Available {
			t.Fatalf("incomplete device snapshot = %#v, ok=%t", s, ok)
		}
	}
}

func TestKeyboardAssignmentsSnapshotFailsClosedWhenActiveK95PlatinumXTProfileIsNotListed(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", Width: 1, Height: 1}}}}}
	d := &Device{UIKeyboard: "keyboard-8", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}}}
	if snapshot, ok := d.KeyboardAssignmentsSnapshot(); ok || snapshot.Available {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}
