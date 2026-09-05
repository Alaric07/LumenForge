// Package telemetrypresentation defines read-only device telemetry snapshots.
package telemetrypresentation

type Row struct {
	Label string
	Value string
}

type Snapshot struct {
	Available bool
	Rows      []Row
}
