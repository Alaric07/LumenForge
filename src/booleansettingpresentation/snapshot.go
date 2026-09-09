// Package booleansettingpresentation defines independent mutable boolean
// settings exposed by the shared Devices workspace.
package booleansettingpresentation

// Snapshot is a device-owned inventory of independently persisted settings.
type Snapshot struct {
	Settings []Setting
}

// Setting identifies one setting and the device-owned action that persists it.
// Action is intentionally opaque to the presentation layer.
type Setting struct {
	ID          string
	Label       string
	Value       bool
	Action      string
	Description string
}
