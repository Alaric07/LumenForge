// Package buttonspresentation contains read-only button assignment data for
// device workspaces.
package buttonspresentation

// Snapshot is a presentation copy of a device's supported button assignments.
type Snapshot struct {
	Buttons         []Button
	AssignmentTypes []AssignmentType
}

// Button is one physical control and its current saved assignment.
type Button struct {
	KeyIndex                 int
	Name                     string
	Default                  bool
	PressAndHold             bool
	OnRelease                bool
	ActionType               uint8
	ActionCommand            uint16
	IsMacro                  bool
	ProfileSwitch            bool
}

// AssignmentType is one assignment kind supported by the device.
type AssignmentType struct {
	ID    uint8
	Label string
}
