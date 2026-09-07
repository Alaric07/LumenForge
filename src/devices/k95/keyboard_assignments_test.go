package k95

import (
	"LumenForge/src/keyboards"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestKeyboardAssignmentsSnapshotPreservesK95GeometryAndKeyMetadata(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{3: {Top: 65, Css: "authored-row", Keys: map[int]keyboards.Key{9: {KeyName: "G1", Width: 2, Height: 1, ActionType: 10, ActionCommand: 7}, 1: {KeyName: "A", Width: 1, Height: 1, OnlyColor: true}}}}}
	d := &Device{Serial: "k95", UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-27", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{10: "Macro", 0: "None"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}}}
	s, ok := d.KeyboardAssignmentsSnapshot()
	if !ok || s.LayoutClass != "keyboard-7" || s.RowLayoutClass != "keyboard-row-27" || len(s.Rows) != 1 || s.Rows[0].Top != 65 || s.Rows[0].Keys[1].KeyName != "G1" || s.Rows[0].Keys[1].ActionCommand != 7 || len(s.AssignmentTypes) != 2 || s.AssignmentTypes[1].ID != 10 {
		t.Fatalf("snapshot = %#v, ok=%t", s, ok)
	}
}

func TestKeyboardAssignmentsSnapshotFailsClosedForIncompleteK95Data(t *testing.T) {
	validKeyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", Width: 1, Height: 1}}}}}
	for _, d := range []*Device{{}, {DeviceProfile: &DeviceProfile{Profile: "missing", Layout: "US", Keyboards: map[string]*keyboards.Keyboard{}}, Layouts: []string{"US"}, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-27", KeyAssignmentTypes: map[int]string{0: "None"}}, {DeviceProfile: &DeviceProfile{Profile: "Default", Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": validKeyboard}}, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-27", KeyAssignmentTypes: map[int]string{0: "None"}}, {DeviceProfile: &DeviceProfile{Profile: "Default", Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {}}}}}, Layouts: []string{"US"}, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-27", KeyAssignmentTypes: map[int]string{0: "None"}}} {
		if s, ok := d.KeyboardAssignmentsSnapshot(); ok || s.Available {
			t.Fatalf("incomplete device snapshot = %#v, ok=%t", s, ok)
		}
	}
}

func TestKeyboardAssignmentsSnapshotFailsClosedWhenActiveK95ProfileIsNotListed(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", Width: 1, Height: 1}}}}}
	d := &Device{UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-27", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None"}, DeviceProfile: &DeviceProfile{Profile: "Default", Profiles: []string{"Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}}}
	if snapshot, ok := d.KeyboardAssignmentsSnapshot(); ok || snapshot.Available {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}

func TestK95DefaultLayoutKeepsMacroBlockAndNumpadSixInTheirPhysicalRows(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "database", "keyboard", "k95.json"))
	if err != nil {
		t.Fatal(err)
	}
	keyboard := &keyboards.Keyboard{}
	if err := json.Unmarshal(data, keyboard); err != nil {
		t.Fatal(err)
	}
	if len(keyboard.Row) != 7 {
		t.Fatalf("K95 rows = %d, want 7", len(keyboard.Row))
	}
	keyCount := 0
	macroKeys := make(map[string]bool)
	for _, row := range keyboard.Row {
		keyCount += len(row.Keys)
		for _, key := range row.Keys {
			macroKeys[key.KeyName] = true
		}
	}
	if keyCount != 135 {
		t.Fatalf("K95 keys = %d, want 135", keyCount)
	}
	for index := 1; index <= 18; index++ {
		name := "G" + strconv.Itoa(index)
		if !macroKeys[name] {
			t.Errorf("K95 macro block omitted %q", name)
		}
	}
	numpadRow := keyboard.Row[5]
	if numpadRow.Css != "keyboard-row-26" {
		t.Fatalf("K95 numpad row CSS = %q, want keyboard-row-26", numpadRow.Css)
	}
	if key4, key5, key6 := numpadRow.Keys[113], numpadRow.Keys[114], numpadRow.Keys[115]; key4.KeyName != "4" || len(key4.KeyEmpty) != 5 || key5.KeyName != "5" || key6.KeyName != "6" {
		t.Fatalf("K95 numpad row keys = %#v, %#v, %#v", key4, key5, key6)
	}
}
