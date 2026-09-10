package optioncolorpresentation

import "LumenForge/src/rgb"

// Snapshot is a device-owned color inventory keyed by a published option.
type Snapshot struct {
	Selected int
	Options  []Option
}
type Option struct {
	Value  int
	Label  string
	Color  rgb.Color
	Action string
}
