package k95platinum

import (
	"LumenForge/src/common"
	"LumenForge/src/keyboards"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"os"
	"path/filepath"
	"testing"
)

func TestK95KeyboardAssignmentsSnapshotPreservesLayoutAndAssignmentState(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{
		4: {Top: 40, Css: "keyboard-row-25", OverrideCss: "layout-override", Keys: map[int]keyboards.Key{9: {KeyName: "B", Width: 2, Height: 1, OnlyColor: true}, 2: {KeyName: "A", Width: 1, Height: 1, KeySpace: "keyboard-key wide3", Css: "top-32", ExtraCss: "layout-extra", Spacing: []int{0}, KeyEmpty: []string{"keyboard-key-empty"}, Default: false, ActionType: 3, ActionCommand: 42}}},
		1: {Top: 10, Keys: map[int]keyboards.Key{7: {KeyName: "C", Width: 1, Height: 1, Default: true, ActionType: 1}}},
	}}
	device := &Device{Serial: "k95", UIKeyboard: "keyboard-8", UIKeyboardRow: "keyboard-row-26", DeviceProfile: &DeviceProfile{Profile: "active", Keyboards: map[string]*keyboards.Keyboard{"active": keyboard}}, KeyAssignmentTypes: map[int]string{10: "Macro", 0: "None", 3: "Keyboard"}}
	snapshot, ok := device.KeyboardAssignmentsSnapshot()
	if !ok || !snapshot.Available || len(snapshot.Rows) != 2 || snapshot.Rows[0].Index != 1 || snapshot.Rows[1].Index != 4 { t.Fatalf("snapshot rows = %#v, ok=%t", snapshot.Rows, ok) }
	if snapshot.LayoutClass != "keyboard-8" || snapshot.RowLayoutClass != "keyboard-row-26" { t.Errorf("layout classes = %q, %q", snapshot.LayoutClass, snapshot.RowLayoutClass) }
	if got := snapshot.Rows[1].Keys; len(got) != 2 || got[0].KeyIndex != 2 || got[0].ActionCommand != 42 || got[1].KeyIndex != 9 || got[1].Assignable { t.Errorf("keys = %#v", got) }
	if got := snapshot.Rows[1]; got.CSS != "keyboard-row-25" || got.OverrideCSS != "layout-override" || got.Keys[0].KeySpace != "keyboard-key wide3" || got.Keys[0].CSS != "top-32" || got.Keys[0].ExtraCSS != "layout-extra" || len(got.Keys[0].Spacing) != 1 || len(got.Keys[0].KeyEmpty) != 1 { t.Errorf("layout metadata = %#v", got) }
	if got := snapshot.AssignmentTypes; len(got) != 3 || got[0].ID != 0 || got[1].ID != 3 || got[2].ID != 10 { t.Errorf("types = %#v", got) }
}

func TestK95KeyboardAssignmentsSnapshotFailsClosedWithoutActiveKeyboard(t *testing.T) {
	device := &Device{Serial: "k95", DeviceProfile: &DeviceProfile{Profile: "missing", Keyboards: map[string]*keyboards.Keyboard{}}, KeyAssignmentTypes: map[int]string{0: "None"}}
	if snapshot, ok := device.KeyboardAssignmentsSnapshot(); ok || len(snapshot.Rows) != 0 { t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok) }
}

func TestK95DefaultKeyboardEditsUseWorkingCopyForSaveAs(t *testing.T) {
	baseline := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{4: {KeyName: "A", Color: rgb.Color{Red: 1}}}}}}
	device := &Device{DeviceProfile: &DeviceProfile{Profile: "default", Keyboards: map[string]*keyboards.Keyboard{"default": baseline}}}
	working := device.editableKeyboard()
	if working == nil || working == baseline || device.defaultKeyboardBaseline != baseline { t.Fatalf("working copy = %#v, baseline = %#v", working, device.defaultKeyboardBaseline) }
	key := working.Row[0].Keys[4]
	key.Color.Red = 99
	working.Row[0].Keys[4] = key
	if baseline.Row[0].Keys[4].Color.Red != 1 { t.Errorf("baseline was modified: %#v", baseline.Row[0].Keys[4]) }
	copyForSaveAs := cloneKeyboard(device.getCurrentKeyboard())
	if copyForSaveAs == nil || copyForSaveAs == working || copyForSaveAs.Row[0].Keys[4].Color.Red != 99 { t.Errorf("Save As copy = %#v", copyForSaveAs) }
}

func TestK95SelectingDefaultAgainRestoresProtectedBaseline(t *testing.T) {
	baseline := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{4: {KeyName: "A", Color: rgb.Color{Red: 1}}}}}}
	device := &Device{DeviceProfile: &DeviceProfile{Profile: "default", Keyboards: map[string]*keyboards.Keyboard{"default": baseline}}}
	working := device.editableKeyboard()
	key := working.Row[0].Keys[4]
	key.Color.Red = 99
	working.Row[0].Keys[4] = key

	device.restoreDefaultKeyboardBaseline()
	if device.defaultKeyboardBaseline != nil { t.Fatalf("baseline remained after revert: %#v", device.defaultKeyboardBaseline) }
	if got := device.DeviceProfile.Keyboards["default"]; got != baseline || got.Row[0].Keys[4].Color.Red != 1 { t.Errorf("restored default = %#v", got) }
}

func TestK95LeavingEditedDefaultDiscardsWorkingCopyBeforeProfileSwitch(t *testing.T) {
	baseline := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{4: {KeyName: "A", Color: rgb.Color{Red: 1}}}}}}
	gaming := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{4: {KeyName: "A", Color: rgb.Color{Red: 22}}}}}}
	device := &Device{DeviceProfile: &DeviceProfile{Profile: "default", Keyboards: map[string]*keyboards.Keyboard{"default": baseline, "gaming": gaming}}}
	working := device.editableKeyboard()
	key := working.Row[0].Keys[4]
	key.Color.Red = 99
	working.Row[0].Keys[4] = key

	device.restoreDefaultKeyboardBaseline()
	device.DeviceProfile.Profile = "gaming"
	if got := device.getCurrentKeyboard(); got != gaming || got.Row[0].Keys[4].Color.Red != 22 { t.Errorf("gaming profile leaked default edit: %#v", got) }
	device.DeviceProfile.Profile = "default"
	if got := device.getCurrentKeyboard(); got != baseline || got.Row[0].Keys[4].Color.Red != 1 { t.Errorf("default profile retained abandoned edit: %#v", got) }
	if device.defaultKeyboardBaseline != nil { t.Fatalf("baseline remained after profile switch: %#v", device.defaultKeyboardBaseline) }
}

func TestK95DefaultKeyboardBaselineDoesNotSurviveProfileReload(t *testing.T) {
	previousPwd := pwd
	pwd = t.TempDir()
	defer func() { pwd = previousPwd }()
	if err := os.MkdirAll(filepath.Join(pwd, "database", "profiles"), 0755); err != nil { t.Fatal(err) }
	oldBaseline := &keyboards.Keyboard{Version: 1, Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{4: {KeyName: "A", Color: rgb.Color{Red: 1}}}}}}
	loadedDefault := cloneKeyboard(oldBaseline)
	loadedKey := loadedDefault.Row[0].Keys[4]
	loadedKey.Color.Red = 2
	loadedDefault.Row[0].Keys[4] = loadedKey
	profile := &DeviceProfile{Active: true, Serial: "k95copy", Layout: "US", Profile: "default", Profiles: []string{"default"}, Keyboards: map[string]*keyboards.Keyboard{"default": loadedDefault}}
	if err := common.SaveJsonData(filepath.Join(pwd, "database", "profiles", "k95copy.json"), profile); err != nil { t.Fatal(err) }
	device := &Device{Serial: "k95copy", DeviceProfile: &DeviceProfile{Profile: "default", Keyboards: map[string]*keyboards.Keyboard{"default": cloneKeyboard(oldBaseline)}}, defaultKeyboardBaseline: oldBaseline}
	logger.Init()
	device.loadDeviceProfiles()
	if device.defaultKeyboardBaseline != nil { t.Fatalf("stale baseline = %#v", device.defaultKeyboardBaseline) }
	working := device.editableKeyboard()
	if working == nil || device.defaultKeyboardBaseline == oldBaseline || device.defaultKeyboardBaseline.Row[0].Keys[4].Color.Red != 2 { t.Errorf("new baseline = %#v", device.defaultKeyboardBaseline) }
}

func TestK95DefaultKeyboardSaveKeepsBaselineAndSaveAsCopy(t *testing.T) {
	baseline := &keyboards.Keyboard{Version: 2, Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{4: {KeyName: "A", Color: rgb.Color{Red: 1}, KeyData: []uint16{1, 2}}}}}}
	working := cloneKeyboard(baseline)
	workingKey := working.Row[0].Keys[4]
	workingKey.Color.Red = 99
	working.Row[0].Keys[4] = workingKey
	saveAsCopy := cloneKeyboard(working)
	device := &Device{DeviceProfile: &DeviceProfile{Profile: "default", Keyboards: map[string]*keyboards.Keyboard{"default": working, "gaming": saveAsCopy}}, defaultKeyboardBaseline: baseline}

	firstSave := device.keyboardProfilesForSave()
	secondSave := device.keyboardProfilesForSave()
	if firstSave["default"] != baseline || secondSave["default"] != baseline { t.Fatalf("default persisted from working copy: %#v %#v", firstSave["default"], secondSave["default"]) }
	if firstSave["gaming"] != saveAsCopy || firstSave["gaming"].Row[0].Keys[4].Color.Red != 99 { t.Errorf("Save As copy = %#v", firstSave["gaming"]) }
	if baseline.Row[0].Keys[4].Color.Red != 1 { t.Errorf("baseline was modified: %#v", baseline.Row[0].Keys[4]) }
}

func TestK95KeyboardUpgradeUsesProtectedBaseline(t *testing.T) {
	layout := &keyboards.Keyboard{Version: 2, Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{4: {KeyData: []uint16{1, 2}}}}}}
	baseline := cloneKeyboard(layout)
	working := cloneKeyboard(layout)
	working.Version = 1
	working.Row[0].Keys[4] = keyboards.Key{}
	if needsUpgrade, version := keyboardNeedsUpgrade(baseline, layout); needsUpgrade || version != 2 { t.Fatalf("protected baseline upgrade result = %t, %d", needsUpgrade, version) }
	if needsUpgrade, _ := keyboardNeedsUpgrade(working, layout); !needsUpgrade { t.Fatal("stale working copy did not require upgrade") }
	device := &Device{DeviceProfile: &DeviceProfile{Keyboards: map[string]*keyboards.Keyboard{"default": working}}, defaultKeyboardBaseline: baseline}
	device.DeviceProfile.Keyboards["default"] = layout
	device.defaultKeyboardBaseline = nil
	if saved := device.keyboardProfilesForSave()["default"]; saved != layout { t.Errorf("upgraded default was not selected for persistence: %#v", saved) }
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
