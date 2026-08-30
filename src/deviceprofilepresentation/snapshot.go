// Package deviceprofilepresentation defines full device-profile data for the
// shared Devices overview workspace.
package deviceprofilepresentation

// Snapshot is a read-only full device-profile capability snapshot.
type Snapshot struct {
	Supported     bool
	Profiles      []string
	ActiveProfile string
}
