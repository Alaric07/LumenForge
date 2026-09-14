// Package screenpresentation defines string-selected device screen profiles.
package screenpresentation

// Option is one backend-advertised screen profile or dynamic target.
type Option struct {
	ID       string
	Label    string
	Selected bool
}

// Snapshot is a read-only view of a device screen selector. Mutations remain
// on the device's existing screen-profile method.
type Snapshot struct {
	Available bool
	Selected  string
	Options   []Option
}
