// Package sleeptimerpresentation defines the optional device sleep timer
// capability used by the shared Devices workspace.
package sleeptimerpresentation

// Snapshot is a device-owned sleep timer value and option inventory.
type Snapshot struct {
	Value   int
	Options []Option
}

// Option is one device-owned sleep timer option.
type Option struct {
	Value int
	Label string
}
