package keyboardassignmentspresentation

import (
	"LumenForge/src/keyboards"
	"sort"
	"strings"
)

// Source contains the validated device-owned keyboard data used to build a
// presentation snapshot. Rows are intentionally passed through from the
// active device profile rather than recreated from UI geometry.
type Source struct {
	Profiles             []string
	ActiveProfile        string
	KeyboardLayouts      []string
	ActiveKeyboardLayout string
	ClusterControlled    bool
	LayoutClass          string
	RowLayoutClass       string
	Rows                 map[int]keyboards.Row
	AssignmentTypes      map[int]string
	OmitModifierOptions  bool
}

// BuildSnapshot converts an active keyboard map into a fail-closed snapshot.
func BuildSnapshot(source Source) (Snapshot, bool) {
	if source.ActiveProfile == "" || len(source.Profiles) == 0 || len(source.KeyboardLayouts) == 0 || source.ActiveKeyboardLayout == "" || source.LayoutClass == "" || source.RowLayoutClass == "" || len(source.Rows) == 0 || len(source.AssignmentTypes) == 0 {
		return Snapshot{}, false
	}
	s := Snapshot{Available: true, Profiles: append([]string(nil), source.Profiles...), ActiveProfile: source.ActiveProfile, KeyboardLayouts: append([]string(nil), source.KeyboardLayouts...), ActiveKeyboardLayout: source.ActiveKeyboardLayout, ClusterControlled: source.ClusterControlled, LayoutClass: source.LayoutClass, RowLayoutClass: source.RowLayoutClass}
	rowIDs := make([]int, 0, len(source.Rows))
	for id := range source.Rows {
		rowIDs = append(rowIDs, id)
	}
	sort.Ints(rowIDs)
	seen := map[int]bool{}
	for _, rowID := range rowIDs {
		row := source.Rows[rowID]
		if len(row.Keys) == 0 {
			return Snapshot{}, false
		}
		presented := Row{Index: rowID, Top: row.Top, CSS: row.Css, OverrideCSS: row.OverrideCss}
		keyIDs := make([]int, 0, len(row.Keys))
		for id := range row.Keys {
			keyIDs = append(keyIDs, id)
		}
		sort.Ints(keyIDs)
		for _, id := range keyIDs {
			if seen[id] {
				return Snapshot{}, false
			}
			seen[id] = true
			key := row.Keys[id]
			// Lighting-only elements (for example, a dial indicator) have no
			// keyboard legend and cannot be assigned. They are still part of the
			// physical layout, so preserve them without treating them as editable
			// keys.
			if strings.TrimSpace(key.KeyName) == "" && !key.OnlyColor {
				return Snapshot{}, false
			}
			red, green, blue := key.Color.Red, key.Color.Green, key.Color.Blue
			if key.NoColor {
				red, green, blue = 255, 255, 255
			}
			presented.Keys = append(presented.Keys, Key{KeyIndex: id, KeyName: key.KeyName, SubKeyName: key.SubKeyName, Width: key.Width, Height: key.Height, Left: key.Left, Top: key.Top, CSS: key.Css, KeySpace: key.KeySpace, ExtraCSS: key.ExtraCss, Spacing: append([]int(nil), key.Spacing...), KeyEmpty: append([]string(nil), key.KeyEmpty...), Assignable: !key.OnlyColor, LightingOnly: key.OnlyColor && strings.TrimSpace(key.KeyName) == "", Default: key.Default, NoColor: key.NoColor, ActionType: key.ActionType, ActionCommand: key.ActionCommand, DeviceID: key.DeviceId, ActionHold: key.ActionHold, ModifierKey: key.ModifierKey, RetainOriginal: key.RetainOriginal, ToggleDelay: key.ToggleDelay, ProfileSwitch: key.ProfileSwitch, Red: red, Green: green, Blue: blue})
		}
		s.Rows = append(s.Rows, presented)
	}
	typeIDs := make([]int, 0, len(source.AssignmentTypes))
	for id := range source.AssignmentTypes {
		typeIDs = append(typeIDs, id)
	}
	sort.Ints(typeIDs)
	for _, id := range typeIDs {
		if strings.TrimSpace(source.AssignmentTypes[id]) == "" {
			return Snapshot{}, false
		}
		s.AssignmentTypes = append(s.AssignmentTypes, AssignmentType{ID: uint8(id), Label: source.AssignmentTypes[id]})
	}
	if source.OmitModifierOptions {
		return s, len(s.Rows) > 0 && len(s.AssignmentTypes) > 0
	}
	options := map[uint8]string{0: "None"}
	for _, row := range source.Rows {
		for id, key := range row.Keys {
			if key.Modifier {
				if id < 1 || id > 255 || (strings.TrimSpace(key.KeyNameInternal) == "" && strings.TrimSpace(key.KeyName) == "") {
					return Snapshot{}, false
				}
				label := key.KeyNameInternal
				if strings.TrimSpace(label) == "" {
					label = key.KeyName
				}
				if _, duplicate := options[uint8(id)]; duplicate {
					return Snapshot{}, false
				}
				options[uint8(id)] = label
			}
		}
	}
	if len(options) == 1 {
		for _, row := range s.Rows {
			for _, key := range row.Keys {
				if key.ModifierKey != 0 {
					return Snapshot{}, false
				}
			}
		}
		return s, len(s.Rows) > 0 && len(s.AssignmentTypes) > 0
	}
	optionIDs := make([]int, 0, len(options))
	for id := range options {
		optionIDs = append(optionIDs, int(id))
	}
	sort.Ints(optionIDs)
	for _, id := range optionIDs {
		s.ModifierOptions = append(s.ModifierOptions, ModifierOption{ID: uint8(id), Label: options[uint8(id)]})
	}
	for _, row := range s.Rows {
		for _, key := range row.Keys {
			if _, ok := options[key.ModifierKey]; !ok {
				return Snapshot{}, false
			}
		}
	}
	return s, len(s.Rows) > 0 && len(s.AssignmentTypes) > 0
}
