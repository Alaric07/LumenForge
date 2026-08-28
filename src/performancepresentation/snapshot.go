// Package performancepresentation defines read-only performance capability
// data for the shared Devices workspace.
package performancepresentation

// Snapshot describes the performance controls a device supports. A nil
// setting means the device does not support that control.
type Snapshot struct {
	PollingRate     *SelectSetting
	AngleSnapping   *ToggleSetting
	LiftHeight      *SelectSetting
	BooleanSettings []BooleanSetting
}

// SelectSetting is a currently selected numeric option and its device-owned
// option inventory.
type SelectSetting struct {
	Value   int
	Options []Option
}

// Option is one device-owned select option.
type Option struct {
	Value int
	Label string
}

// ToggleSetting is a currently selected boolean capability value.
type ToggleSetting struct {
	Enabled bool
}

// BooleanSetting is one device-owned boolean performance control.
type BooleanSetting struct {
	ID      string
	Label   string
	Enabled bool
}
