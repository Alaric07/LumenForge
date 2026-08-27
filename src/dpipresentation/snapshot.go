// Package dpipresentation contains read-only DPI data used by device workspaces.
package dpipresentation

// Snapshot is a presentation copy of a device's current DPI configuration.
type Snapshot struct {
	MinimumDPI           int
	MaximumDPI           int
	ActiveRegularStageID string
	Stages               []Stage
}

// Stage is one configured DPI stage. ID is stable for the lifetime of the
// underlying device profile key.
type Stage struct {
	ID       string
	Name     string
	DPI      uint16
	ColorHex string
	Sniper   bool
	Active   bool
}
