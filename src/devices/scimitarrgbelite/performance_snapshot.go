package scimitarrgbelite

import (
	"sort"

	"LumenForge/src/performancepresentation"
)

// PerformanceDeviceID identifies the device whose current state is returned
// by PerformanceSnapshot.
func (d *Device) PerformanceDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// PerformanceSnapshot returns a read-only copy of the current performance
// settings. Their mutation and persistence remain owned by DeviceProfile.
func (d *Device) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil {
		return performancepresentation.Snapshot{}, false
	}

	snapshot := performancepresentation.Snapshot{
		AngleSnapping: &performancepresentation.ToggleSetting{Enabled: d.DeviceProfile.AngleSnapping != 0},
	}
	if len(d.PollingRates) > 0 {
		snapshot.PollingRate = &performancepresentation.SelectSetting{
			Value:   d.DeviceProfile.PollingRate,
			Options: performanceOptions(d.PollingRates),
		}
	}
	if len(d.LiftHeights) > 0 {
		snapshot.LiftHeight = &performancepresentation.SelectSetting{
			Value:   d.DeviceProfile.LiftHeight,
			Options: performanceOptions(d.LiftHeights),
		}
	}
	return snapshot, true
}

func performanceOptions(options map[int]string) []performancepresentation.Option {
	keys := make([]int, 0, len(options))
	for value := range options {
		keys = append(keys, value)
	}
	sort.Ints(keys)
	presented := make([]performancepresentation.Option, 0, len(keys))
	for _, value := range keys {
		presented = append(presented, performancepresentation.Option{Value: value, Label: options[value]})
	}
	return presented
}
