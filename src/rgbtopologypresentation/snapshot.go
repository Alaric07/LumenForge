// Package rgbtopologypresentation defines source-backed external RGB topology
// data for the modern Devices workspace.
package rgbtopologypresentation

// Option is a source catalogue value.
type Option struct {
	ID    int
	Label string
}

// Port is one independently configurable external RGB connection.
type Port struct {
	ID             int
	Label          string
	DeviceTypes    []Option
	SelectedType   int
	DeviceAmounts  []Option
	SelectedAmount int
}

// Snapshot contains only topology configuration, not effect state.
type Snapshot struct {
	DeviceID string
	Ports    []Port
}
