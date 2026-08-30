package k95platinum

import (
	"LumenForge/src/keyboards"
	"LumenForge/src/rgb"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestK95KeyboardAssignmentsSnapshotPreservesLayoutAndAssignmentState(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{
		4: {Top: 40, Css: "keyboard-row-25", OverrideCss: "layout-override", Keys: map[int]keyboards.Key{9: {KeyName: "B", Width: 2, Height: 1, OnlyColor: true}, 2: {KeyName: "A", Width: 1, Height: 1, KeySpace: "keyboard-key wide3", Css: "top-32", ExtraCss: "layout-extra", Spacing: []int{0}, KeyEmpty: []string{"keyboard-key-empty"}, Default: false, ActionType: 3, ActionCommand: 42}}},
		1: {Top: 10, Keys: map[int]keyboards.Key{7: {KeyName: "C", Width: 1, Height: 1, Default: true, ActionType: 1}}},
	}}
	device := &Device{Serial: "k95", Layouts: []string{"US", "UK"}, UIKeyboard: "keyboard-8", UIKeyboardRow: "keyboard-row-26", DeviceProfile: &DeviceProfile{Profile: "active", Layout: "US", KeyboardLiveSync: true, Keyboards: map[string]*keyboards.Keyboard{"active": keyboard}}, KeyAssignmentTypes: map[int]string{10: "Macro", 0: "None", 3: "Keyboard"}}
	snapshot, ok := device.KeyboardAssignmentsSnapshot()
	if !ok || !snapshot.Available || len(snapshot.Rows) != 2 || snapshot.Rows[0].Index != 1 || snapshot.Rows[1].Index != 4 { t.Fatalf("snapshot rows = %#v, ok=%t", snapshot.Rows, ok) }
	if snapshot.LayoutClass != "keyboard-8" || snapshot.RowLayoutClass != "keyboard-row-26" { t.Errorf("layout classes = %q, %q", snapshot.LayoutClass, snapshot.RowLayoutClass) }
	if got := snapshot.KeyboardLayouts; len(got) != 2 || got[0] != "US" || got[1] != "UK" || snapshot.ActiveKeyboardLayout != "US" { t.Errorf("keyboard layouts = %#v, active = %q", got, snapshot.ActiveKeyboardLayout) }
	if !snapshot.LiveRGBAvailable || !snapshot.LiveRGBEnabled { t.Errorf("live RGB = available:%t enabled:%t", snapshot.LiveRGBAvailable, snapshot.LiveRGBEnabled) }
	if got := snapshot.Rows[1].Keys; len(got) != 2 || got[0].KeyIndex != 2 || got[0].ActionCommand != 42 || got[1].KeyIndex != 9 || got[1].Assignable { t.Errorf("keys = %#v", got) }
	if got := snapshot.Rows[1]; got.CSS != "keyboard-row-25" || got.OverrideCSS != "layout-override" || got.Keys[0].KeySpace != "keyboard-key wide3" || got.Keys[0].CSS != "top-32" || got.Keys[0].ExtraCSS != "layout-extra" || len(got.Keys[0].Spacing) != 1 || len(got.Keys[0].KeyEmpty) != 1 { t.Errorf("layout metadata = %#v", got) }
	if got := snapshot.AssignmentTypes; len(got) != 3 || got[0].ID != 0 || got[1].ID != 3 || got[2].ID != 10 { t.Errorf("types = %#v", got) }
}

func TestK95KeyboardAssignmentsSnapshotFailsClosedWithoutActiveKeyboard(t *testing.T) {
	device := &Device{Serial: "k95", DeviceProfile: &DeviceProfile{Profile: "missing", Keyboards: map[string]*keyboards.Keyboard{}}, KeyAssignmentTypes: map[int]string{0: "None"}}
	if snapshot, ok := device.KeyboardAssignmentsSnapshot(); ok || len(snapshot.Rows) != 0 { t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok) }
}

func TestK95KeyboardAssignmentsSnapshotPreservesNoColorLegend(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "Lock", Width: 1, Height: 1, NoColor: true, Color: rgb.Color{Red: 1, Green: 2, Blue: 3}}}}}}
	device := &Device{Serial: "k95", DeviceProfile: &DeviceProfile{Profile: "active", Keyboards: map[string]*keyboards.Keyboard{"active": keyboard}}, KeyAssignmentTypes: map[int]string{0: "None"}}
	snapshot, ok := device.KeyboardAssignmentsSnapshot()
	if !ok { t.Fatal("snapshot unavailable") }
	key := snapshot.Rows[0].Keys[0]
	if !key.NoColor || key.Red != 255 || key.Green != 255 || key.Blue != 255 { t.Errorf("NoColor presentation = %#v", key) }
}

func TestK95KeyboardAssignmentsSnapshotUsesAuthoredDefaultLayoutColors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "database", "keyboard", "k95platinum.json"))
	if err != nil { t.Fatal(err) }
	authored := &keyboards.Keyboard{}
	if err := json.Unmarshal(data, authored); err != nil { t.Fatal(err) }
	device := &Device{Serial: "k95", DeviceProfile: &DeviceProfile{Profile: "default", Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"default": authored}}, KeyAssignmentTypes: map[int]string{0: "None"}}
	snapshot, ok := device.KeyboardAssignmentsSnapshot()
	if !ok { t.Fatal("snapshot unavailable") }
	if got := snapshot.Rows[0].Keys[0]; got.Red != 0 || got.Green != 255 || got.Blue != 255 { t.Errorf("top LED color = %#v", got) }
	if got := snapshot.Rows[0].Keys[8]; got.Red != 255 || got.Green != 255 || got.Blue != 0 { t.Errorf("logo color = %#v", got) }
}

func TestK95KeyboardAssignmentsSnapshotUsesCurrentDefaultColors(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {Color: rgb.Color{Red: 1}}}}}}
	key := keyboard.Row[0].Keys[1]
	key.Color = rgb.Color{Red: 12, Green: 34, Blue: 56}
	keyboard.Row[0].Keys[1] = key
	device := &Device{Serial: "k95", DeviceProfile: &DeviceProfile{Profile: "default", Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"default": keyboard}}, KeyAssignmentTypes: map[int]string{0: "None"}}
	snapshot, ok := device.KeyboardAssignmentsSnapshot()
	if !ok || snapshot.Rows[0].Keys[0].Red != 12 || snapshot.Rows[0].Keys[0].Green != 34 || snapshot.Rows[0].Keys[0].Blue != 56 { t.Errorf("default snapshot = %#v, ok=%t", snapshot, ok) }
}

func TestK95KeyboardAssignmentsSnapshotUsesCustomProfileColors(t *testing.T) {
	custom := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {Color: rgb.Color{Red: 12, Green: 34, Blue: 56}}}}}}
	device := &Device{Serial: "k95", DeviceProfile: &DeviceProfile{Profile: "gaming", Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"gaming": custom}}, KeyAssignmentTypes: map[int]string{0: "None"}}
	snapshot, ok := device.KeyboardAssignmentsSnapshot()
	if !ok || snapshot.Rows[0].Keys[0].Red != 12 || snapshot.Rows[0].Keys[0].Green != 34 || snapshot.Rows[0].Keys[0].Blue != 56 { t.Errorf("custom snapshot = %#v, ok=%t", snapshot, ok) }
}

func TestK95DefaultKeyboardEditsDirectlyAndSaveAsClonesCurrentState(t *testing.T) {
	defaultKeyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{4: {KeyName: "A", Color: rgb.Color{Red: 1}}}}}}
	gaming := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{4: {KeyName: "A", Color: rgb.Color{Red: 22}}}}}}
	device := &Device{DeviceProfile: &DeviceProfile{Profile: "default", Keyboards: map[string]*keyboards.Keyboard{"default": defaultKeyboard, "gaming": gaming}}}
	active := device.getCurrentKeyboard()
	if active != defaultKeyboard { t.Fatalf("default keyboard = %#v", active) }
	key := active.Row[0].Keys[4]
	key.Color.Red = 99
	active.Row[0].Keys[4] = key
	if got := device.DeviceProfile.Keyboards["default"]; got != defaultKeyboard || got.Row[0].Keys[4].Color.Red != 99 { t.Errorf("default edit = %#v", got) }
	copyForSaveAs := cloneKeyboard(device.getCurrentKeyboard())
	if copyForSaveAs == nil || copyForSaveAs == defaultKeyboard || copyForSaveAs.Row[0].Keys[4].Color.Red != 99 { t.Errorf("Save As copy = %#v", copyForSaveAs) }
	device.DeviceProfile.Profile = "gaming"
	if got := device.getCurrentKeyboard(); got != gaming || got.Row[0].Keys[4].Color.Red != 22 { t.Errorf("gaming keyboard = %#v", got) }
	device.DeviceProfile.Profile = "default"
	if got := device.getCurrentKeyboard(); got != defaultKeyboard || got.Row[0].Keys[4].Color.Red != 99 { t.Errorf("default edit was discarded: %#v", got) }
}

func TestK95KeyboardColorSkipsNoColorKeys(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{
		0: {Keys: map[int]keyboards.Key{1: {Color: rgb.Color{Red: 1}}, 2: {NoColor: true, Color: rgb.Color{Red: 2}}}},
		1: {Keys: map[int]keyboards.Key{3: {Color: rgb.Color{Red: 3}}}},
	}}
	color := rgb.Color{Red: 9, Green: 8, Blue: 7}
	setKeyboardRowColor(keyboard, 0, color)
	if got := keyboard.Row[0].Keys[1].Color; got.Red != 9 || got.Green != 8 || got.Blue != 7 { t.Errorf("colorable row key = %#v", got) }
	if got := keyboard.Row[0].Keys[2].Color; got.Red != 2 { t.Errorf("NoColor row key changed = %#v", got) }
	setKeyboardAllColor(keyboard, rgb.Color{Red: 6, Green: 5, Blue: 4})
	if got := keyboard.Row[1].Keys[3].Color; got.Red != 6 || got.Green != 5 || got.Blue != 4 { t.Errorf("colorable all-key value = %#v", got) }
	if got := keyboard.Row[0].Keys[2].Color; got.Red != 2 { t.Errorf("NoColor all-key value changed = %#v", got) }
}
