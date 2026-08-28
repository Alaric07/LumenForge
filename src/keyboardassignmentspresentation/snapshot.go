// Package keyboardassignmentspresentation defines keyboard assignment data for
// the shared Devices workspace.
package keyboardassignmentspresentation

// Snapshot is a read-only keyboard-assignment capability snapshot.
type Snapshot struct {
	Available       bool
	Profiles        []string
	ActiveProfile   string
	ClusterControlled bool
	LayoutClass, RowLayoutClass string
	Rows            []Row
	AssignmentTypes []AssignmentType
}

type AssignmentType struct { ID uint8; Label string }

// Row retains device-owned layout placement while presenting keys in a stable
// order suitable for the template.
type Row struct { Index, Top int; CSS, OverrideCSS string; Keys []Key }

// Key is the small assignment and geometry contract the workspace needs.
type Key struct {
	KeyIndex int
	KeyName, SubKeyName string
	Width, Height, Left, Top int
	CSS string
	KeySpace, ExtraCSS string
	Spacing []int
	KeyEmpty []string
	Assignable, Default bool
	ActionType uint8
	ActionCommand uint16
	DeviceID string
	ActionHold bool
	ToggleDelay uint16
	ProfileSwitch bool
	ColorHex string
}
