package scimitarrgbelite

import (
	"LumenForge/src/buttonspresentation"
)

var scimitarEliteVisibleButtonOrder = []int{
	2, 4, 32, 64,
	256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072, 262144, 524288,
}

// ButtonsDeviceID identifies the device whose assignments are returned by
// ButtonsSnapshot.
func (d *Device) ButtonsDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// ButtonsSnapshot returns the existing key assignment state in physical button
// order. Assignment ownership and persistence remain in KeyAssignment.
func (d *Device) ButtonsSnapshot() (buttonspresentation.Snapshot, bool) {
	if d == nil || len(d.KeyAssignment) == 0 || len(d.KeyAssignmentTypes) == 0 {
		return buttonspresentation.Snapshot{}, false
	}

	snapshot := buttonspresentation.Snapshot{
		Buttons:         make([]buttonspresentation.Button, 0, len(scimitarEliteVisibleButtonOrder)),
		AssignmentTypes: make([]buttonspresentation.AssignmentType, 0, len(d.KeyAssignmentTypes)),
	}
	for _, keyIndex := range scimitarEliteVisibleButtonOrder {
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
