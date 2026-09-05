package harpoonrgbpro

import "LumenForge/src/buttonspresentation"

// Harpoon physical key masks are the IDs persisted by its existing key
// assignment file and interpreted by its listener.
var harpoonVisibleButtonOrder = []int{1, 2, 4, 8, 16, 32}

func (d *Device) ButtonsDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// ButtonsSnapshot projects the existing physical assignments and supported
// assignment vocabulary without interacting with HID or persistence.
func (d *Device) ButtonsSnapshot() (buttonspresentation.Snapshot, bool) {
	if d == nil || len(d.KeyAssignment) == 0 || len(d.KeyAssignmentTypes) == 0 {
		return buttonspresentation.Snapshot{}, false
	}

	snapshot := buttonspresentation.Snapshot{
		Buttons:         make([]buttonspresentation.Button, 0, len(harpoonVisibleButtonOrder)),
		AssignmentTypes: make([]buttonspresentation.AssignmentType, 0, len(d.KeyAssignmentTypes)),
	}
	for _, keyIndex := range harpoonVisibleButtonOrder {
		assignment, ok := d.KeyAssignment[keyIndex]
		if !ok || assignment.Name == "" {
			return buttonspresentation.Snapshot{}, false
		}
		snapshot.Buttons = append(snapshot.Buttons, buttonspresentation.Button{
			KeyIndex: keyIndex, Name: assignment.Name, Default: assignment.Default,
			PressAndHold: assignment.ActionHold, OnRelease: assignment.OnRelease,
			ActionType: assignment.ActionType, ActionCommand: assignment.ActionCommand,
			IsMacro: assignment.IsMacro, ProfileSwitch: assignment.ProfileSwitch,
		})
	}
	for _, id := range []int{0, 1, 2, 3, 8, 9, 10, 11} {
		label, ok := d.KeyAssignmentTypes[id]
		if !ok || label == "" {
			return buttonspresentation.Snapshot{}, false
		}
		snapshot.AssignmentTypes = append(snapshot.AssignmentTypes, buttonspresentation.AssignmentType{ID: uint8(id), Label: label})
	}
	return snapshot, true
}
