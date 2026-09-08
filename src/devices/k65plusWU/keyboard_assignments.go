package k65plusWU

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
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || len(d.Layouts) == 0 || d.UIKeyboard == "" || d.UIKeyboardRow == "" || len(d.KeyAssignmentTypes) == 0 || !k65PlusWUContains(d.Layouts, d.DeviceProfile.Layout) || !k65PlusWUContains(d.DeviceProfile.Profiles, d.DeviceProfile.Profile) {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	keyboard := d.getCurrentKeyboard()
	if keyboard == nil || len(keyboard.Row) == 0 {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	snapshot := keyboardassignmentspresentation.Snapshot{Available: true, Profiles: append([]string(nil), d.DeviceProfile.Profiles...), ActiveProfile: d.DeviceProfile.Profile, KeyboardLayouts: append([]string(nil), d.Layouts...), ActiveKeyboardLayout: d.DeviceProfile.Layout, ClusterControlled: d.DeviceProfile.RGBCluster, LayoutClass: d.UIKeyboard, RowLayoutClass: d.UIKeyboardRow}
	seen := map[int]bool{}
	rowIDs := make([]int, 0, len(keyboard.Row))
	for id := range keyboard.Row {
		rowIDs = append(rowIDs, id)
	}
	sort.Ints(rowIDs)
	for _, rowID := range rowIDs {
		row := keyboard.Row[rowID]
		if len(row.Keys) == 0 {
			return keyboardassignmentspresentation.Snapshot{}, false
		}
		presentationRow := keyboardassignmentspresentation.Row{Index: rowID, Top: row.Top, CSS: row.Css, OverrideCSS: row.OverrideCss}
		keyIDs := make([]int, 0, len(row.Keys))
		for id := range row.Keys {
			if seen[id] {
				return keyboardassignmentspresentation.Snapshot{}, false
			}
			seen[id] = true
			keyIDs = append(keyIDs, id)
		}
		sort.Ints(keyIDs)
		for _, id := range keyIDs {
			key := row.Keys[id]
			if strings.TrimSpace(key.KeyName) == "" {
				return keyboardassignmentspresentation.Snapshot{}, false
			}
			red, green, blue := key.Color.Red, key.Color.Green, key.Color.Blue
			if key.NoColor {
				red, green, blue = 255, 255, 255
			}
			presentationRow.Keys = append(presentationRow.Keys, keyboardassignmentspresentation.Key{KeyIndex: id, KeyName: key.KeyName, SubKeyName: key.SubKeyName, Width: key.Width, Height: key.Height, Left: key.Left, Top: key.Top, CSS: key.Css, KeySpace: key.KeySpace, ExtraCSS: key.ExtraCss, Spacing: append([]int(nil), key.Spacing...), KeyEmpty: append([]string(nil), key.KeyEmpty...), Assignable: !key.OnlyColor, Default: key.Default, NoColor: key.NoColor, ActionType: key.ActionType, ActionCommand: key.ActionCommand, DeviceID: key.DeviceId, ActionHold: key.ActionHold, ModifierKey: key.ModifierKey, RetainOriginal: key.RetainOriginal, ToggleDelay: key.ToggleDelay, ProfileSwitch: key.ProfileSwitch, Red: red, Green: green, Blue: blue})
		}
		snapshot.Rows = append(snapshot.Rows, presentationRow)
	}
	typeIDs := make([]int, 0, len(d.KeyAssignmentTypes))
	for id := range d.KeyAssignmentTypes {
		typeIDs = append(typeIDs, id)
	}
	sort.Ints(typeIDs)
	for _, id := range typeIDs {
		if strings.TrimSpace(d.KeyAssignmentTypes[id]) == "" {
			return keyboardassignmentspresentation.Snapshot{}, false
		}
		snapshot.AssignmentTypes = append(snapshot.AssignmentTypes, keyboardassignmentspresentation.AssignmentType{ID: uint8(id), Label: d.KeyAssignmentTypes[id]})
	}
	options := map[uint8]string{0: "None"}
	for _, row := range keyboard.Row {
		for index, key := range row.Keys {
			if !key.Modifier {
				continue
			}
			if index < 1 || index > 255 || (strings.TrimSpace(key.KeyNameInternal) == "" && strings.TrimSpace(key.KeyName) == "") {
				return keyboardassignmentspresentation.Snapshot{}, false
			}
			label := key.KeyNameInternal
			if strings.TrimSpace(label) == "" {
				label = key.KeyName
			}
			if _, exists := options[uint8(index)]; exists {
				return keyboardassignmentspresentation.Snapshot{}, false
			}
			options[uint8(index)] = label
		}
	}
	optionIDs := make([]int, 0, len(options))
	for id := range options {
		optionIDs = append(optionIDs, int(id))
	}
	sort.Ints(optionIDs)
	for _, id := range optionIDs {
		snapshot.ModifierOptions = append(snapshot.ModifierOptions, keyboardassignmentspresentation.ModifierOption{ID: uint8(id), Label: options[uint8(id)]})
	}
	for _, row := range snapshot.Rows {
		for _, key := range row.Keys {
			if _, exists := options[key.ModifierKey]; !exists {
				return keyboardassignmentspresentation.Snapshot{}, false
			}
		}
	}
	return snapshot, len(snapshot.Rows) > 0
}

func k65PlusWUContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
