package k95platinum

import (
	"LumenForge/src/keyboardassignmentspresentation"
	"sort"
)

// KeyboardAssignmentsDeviceID and KeyboardAssignmentsSnapshot make this
// device a thin provider for the generic Devices workspace.
func (d *Device) KeyboardAssignmentsDeviceID() string {
	if d == nil { return "" }
	return d.Serial
}

func (d *Device) KeyboardAssignmentsSnapshot() (keyboardassignmentspresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil { return keyboardassignmentspresentation.Snapshot{}, false }
	keyboard := d.getCurrentKeyboard()
	if keyboard == nil { return keyboardassignmentspresentation.Snapshot{}, false }
	snapshot := keyboardassignmentspresentation.Snapshot{Available: true, LiveRGBAvailable: true, LiveRGBEnabled: d.DeviceProfile.KeyboardLiveSync, Profiles: append([]string(nil), d.DeviceProfile.Profiles...), ActiveProfile: d.DeviceProfile.Profile, KeyboardLayouts: append([]string(nil), d.Layouts...), ActiveKeyboardLayout: d.DeviceProfile.Layout, ClusterControlled: d.DeviceProfile.RGBCluster, LayoutClass: d.UIKeyboard, RowLayoutClass: d.UIKeyboardRow}
	rowIDs := make([]int, 0, len(keyboard.Row))
	for id := range keyboard.Row { rowIDs = append(rowIDs, id) }
	sort.Ints(rowIDs)
	for _, rowID := range rowIDs {
		row := keyboard.Row[rowID]
		presented := keyboardassignmentspresentation.Row{Index: rowID, Top: row.Top, CSS: row.Css, OverrideCSS: row.OverrideCss}
		keyIDs := make([]int, 0, len(row.Keys))
		for id := range row.Keys { keyIDs = append(keyIDs, id) }
		sort.Ints(keyIDs)
		for _, keyID := range keyIDs {
			key := row.Keys[keyID]
		red, green, blue := key.Color.Red, key.Color.Green, key.Color.Blue
		if key.NoColor { red, green, blue = 255, 255, 255 }
			presented.Keys = append(presented.Keys, keyboardassignmentspresentation.Key{KeyIndex: keyID, KeyName: key.KeyName, SubKeyName: key.SubKeyName, Width: key.Width, Height: key.Height, Left: key.Left, Top: key.Top, CSS: key.Css, KeySpace: key.KeySpace, ExtraCSS: key.ExtraCss, Spacing: append([]int(nil), key.Spacing...), KeyEmpty: append([]string(nil), key.KeyEmpty...), Assignable: !key.OnlyColor, Default: key.Default, NoColor: key.NoColor, ActionType: key.ActionType, ActionCommand: key.ActionCommand, DeviceID: key.DeviceId, ActionHold: key.ActionHold, ToggleDelay: key.ToggleDelay, ProfileSwitch: key.ProfileSwitch, Red: red, Green: green, Blue: blue})
		}
		snapshot.Rows = append(snapshot.Rows, presented)
	}
	typeIDs := make([]int, 0, len(d.KeyAssignmentTypes))
	for id := range d.KeyAssignmentTypes { typeIDs = append(typeIDs, id) }
	sort.Ints(typeIDs)
	for _, id := range typeIDs { snapshot.AssignmentTypes = append(snapshot.AssignmentTypes, keyboardassignmentspresentation.AssignmentType{ID: uint8(id), Label: d.KeyAssignmentTypes[id]}) }
	return snapshot, len(snapshot.Rows) > 0 && len(snapshot.AssignmentTypes) > 0
}
