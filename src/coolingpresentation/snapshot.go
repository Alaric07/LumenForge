// Package coolingpresentation defines controller-neutral cooling workspace data.
package coolingpresentation

// ProfileOption is an existing temperature/speed profile advertised for a
// controller channel.
type ProfileOption struct {
	ID    string
	Label string
}

// Channel is a speed-capable controller channel.
type Channel struct {
	ID              int
	Name            string
	Label           string
	RPM             int16
	Temperature     string
	Celsius         *float32
	ContainsPump    bool
	SelectedProfile string
}

// TemperatureProbe is read-only telemetry from a controller probe channel.
type TemperatureProbe struct {
	ID          int
	Name        string
	Label       string
	Temperature string
	Celsius     *float32
}

// Snapshot is a read-only cooling capability snapshot. Mutations continue to
// use the controller's existing generic speed and label routes.
type Snapshot struct {
	Available         bool
	Channels          []Channel
	ProfileOptions    []ProfileOption
	TemperatureProbes []TemperatureProbe
}
