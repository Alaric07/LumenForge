package k57rgbWU

import (
	"LumenForge/src/keyboardassignmentspresentation"
	"sort"
	"strings"
)

func (d *Device) KeyboardAssignmentsDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) KeyboardAssignmentsSnapshot() (keyboardassignmentspresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || len(d.Layouts) == 0 || d.UIKeyboard == "" || d.UIKeyboardRow == "" || len(d.KeyAssignmentTypes) == 0 || !k57RGBWUContains(d.Layouts, d.DeviceProfile.Layout) || !k57RGBWUContains(d.DeviceProfile.Profiles, d.DeviceProfile.Profile) {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	keyboard := d.getCurrentKeyboard()
	if keyboard == nil || len(keyboard.Row) == 0 {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	s := keyboardassignmentspresentation.Snapshot{Available: true, Profiles: append([]string(nil), d.DeviceProfile.Profiles...), ActiveProfile: d.DeviceProfile.Profile, KeyboardLayouts: append([]string(nil), d.Layouts...), ActiveKeyboardLayout: d.DeviceProfile.Layout, LayoutClass: d.UIKeyboard, RowLayoutClass: d.UIKeyboardRow}
	rows := make([]int, 0, len(keyboard.Row))
	for id := range keyboard.Row {
		rows = append(rows, id)
	}
	sort.Ints(rows)
	seen := map[int]bool{}
	for _, rowID := range rows {
		row := keyboard.Row[rowID]
		if len(row.Keys) == 0 {
			return keyboardassignmentspresentation.Snapshot{}, false
		}
		presented := keyboardassignmentspresentation.Row{Index: rowID, Top: row.Top, CSS: row.Css, OverrideCSS: row.OverrideCss}
		ids := make([]int, 0, len(row.Keys))
		for id := range row.Keys {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			if seen[id] {
				return keyboardassignmentspresentation.Snapshot{}, false
			}
			seen[id] = true
			key := row.Keys[id]
			if strings.TrimSpace(key.KeyName) == "" || key.ModifierKey != 0 || key.RetainOriginal {
				return keyboardassignmentspresentation.Snapshot{}, false
			}
			red, green, blue := key.Color.Red, key.Color.Green, key.Color.Blue
			if key.NoColor {
				red, green, blue = 255, 255, 255
			}
			presented.Keys = append(presented.Keys, keyboardassignmentspresentation.Key{KeyIndex: id, KeyName: key.KeyName, SubKeyName: key.SubKeyName, Width: key.Width, Height: key.Height, Left: key.Left, Top: key.Top, CSS: key.Css, KeySpace: key.KeySpace, ExtraCSS: key.ExtraCss, Spacing: append([]int(nil), key.Spacing...), KeyEmpty: append([]string(nil), key.KeyEmpty...), Assignable: !key.OnlyColor, Default: key.Default, NoColor: key.NoColor, ActionType: key.ActionType, ActionCommand: key.ActionCommand, DeviceID: key.DeviceId, ActionHold: key.ActionHold, ToggleDelay: key.ToggleDelay, ProfileSwitch: key.ProfileSwitch, Red: red, Green: green, Blue: blue})
		}
		s.Rows = append(s.Rows, presented)
	}
	ids := make([]int, 0, len(d.KeyAssignmentTypes))
	for id := range d.KeyAssignmentTypes {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		if strings.TrimSpace(d.KeyAssignmentTypes[id]) == "" {
			return keyboardassignmentspresentation.Snapshot{}, false
		}
		s.AssignmentTypes = append(s.AssignmentTypes, keyboardassignmentspresentation.AssignmentType{ID: uint8(id), Label: d.KeyAssignmentTypes[id]})
	}
	return s, len(s.Rows) > 0 && len(s.AssignmentTypes) > 0
}
func k57RGBWUContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
