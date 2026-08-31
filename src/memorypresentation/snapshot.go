// Package memorypresentation defines the read-only Memory capability snapshot
// used by the shared Devices workspace.
package memorypresentation

// Snapshot describes the detected physical DIMMs. It intentionally does not
// expose legacy RGB state: canonical Memory lighting has not migrated yet.
type Snapshot struct {
	Available bool
	Modules   []Module
}

// Module is one detected physical DIMM, identified by its existing channel ID.
type Module struct {
	ChannelID   int
	Name        string
	Label       string
	MemoryType  int
	SKU         string
	LEDCount    uint8
	Temperature string
}
