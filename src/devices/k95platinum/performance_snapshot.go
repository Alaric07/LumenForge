package k95platinum

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
		SaveBooleanSettings: true,
		BooleanSettings: []performancepresentation.BooleanSetting{
			{ID: "perf_winKey", Label: "Disable Win Key", Enabled: d.DeviceProfile.DisableWinKey},
			{ID: "perf_shiftTab", Label: "Disable Shift + Tab", Enabled: d.DeviceProfile.DisableShiftTab},
			{ID: "perf_altTab", Label: "Disable Alt + Tab", Enabled: d.DeviceProfile.DisableAltTab},
			{ID: "perf_altF4", Label: "Disable Alt + F4", Enabled: d.DeviceProfile.DisableAltF4},
		},
	}
	if len(d.PollingRates) > 0 {
		snapshot.PollingRate = &performancepresentation.SelectSetting{
			Value:   d.DeviceProfile.PollingRate,
			Options: k95PerformanceOptions(d.PollingRates),
		}
	}
	return snapshot, true
}

func k95PerformanceOptions(options map[int]string) []performancepresentation.Option {
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
