package k70coretkl

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
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || len(d.Layouts) == 0 || d.UIKeyboard == "" || d.UIKeyboardRow == "" || len(d.KeyAssignmentTypes) == 0 || !containsK70CoreTKL(d.Layouts, d.DeviceProfile.Layout) || !containsK70CoreTKL(d.DeviceProfile.Profiles, d.DeviceProfile.Profile) {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	k := d.getCurrentKeyboard()
	if k == nil || len(k.Row) == 0 {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	s := keyboardassignmentspresentation.Snapshot{Available: true, Profiles: append([]string(nil), d.DeviceProfile.Profiles...), ActiveProfile: d.DeviceProfile.Profile, KeyboardLayouts: append([]string(nil), d.Layouts...), ActiveKeyboardLayout: d.DeviceProfile.Layout, LayoutClass: d.UIKeyboard, RowLayoutClass: d.UIKeyboardRow}
	rows := make([]int, 0, len(k.Row))
	for id := range k.Row {
		rows = append(rows, id)
	}
	sort.Ints(rows)
	for _, rid := range rows {
		r := k.Row[rid]
		if len(r.Keys) == 0 {
			return keyboardassignmentspresentation.Snapshot{}, false
		}
		pr := keyboardassignmentspresentation.Row{Index: rid, Top: r.Top, CSS: r.Css, OverrideCSS: r.OverrideCss}
		ids := make([]int, 0, len(r.Keys))
		for id := range r.Keys {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			x := r.Keys[id]
			red, green, blue := x.Color.Red, x.Color.Green, x.Color.Blue
			if x.NoColor {
				red, green, blue = 255, 255, 255
			}
			pr.Keys = append(pr.Keys, keyboardassignmentspresentation.Key{KeyIndex: id, KeyName: x.KeyName, SubKeyName: x.SubKeyName, Width: x.Width, Height: x.Height, Left: x.Left, Top: x.Top, CSS: x.Css, KeySpace: x.KeySpace, ExtraCSS: x.ExtraCss, Spacing: append([]int(nil), x.Spacing...), KeyEmpty: append([]string(nil), x.KeyEmpty...), Assignable: !x.OnlyColor, Default: x.Default, NoColor: x.NoColor, ActionType: x.ActionType, ActionCommand: x.ActionCommand, DeviceID: x.DeviceId, ActionHold: x.ActionHold, ToggleDelay: x.ToggleDelay, ProfileSwitch: x.ProfileSwitch, Red: red, Green: green, Blue: blue})
		}
		s.Rows = append(s.Rows, pr)
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
func containsK70CoreTKL(v []string, x string) bool {
	for _, a := range v {
		if a == x {
			return true
		}
	}
	return false
}
