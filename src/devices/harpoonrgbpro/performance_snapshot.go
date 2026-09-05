package harpoonrgbpro

import (
	"sort"

	"LumenForge/src/performancepresentation"
)

func (d *Device) PerformanceDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// PerformanceSnapshot exposes Harpoon's polling-rate capability only.
func (d *Device) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.PollingRates) == 0 {
		return performancepresentation.Snapshot{}, false
	}
	return performancepresentation.Snapshot{PollingRate: &performancepresentation.SelectSetting{
		Value: d.DeviceProfile.PollingRate, Options: harpoonPerformanceOptions(d.PollingRates),
	}}, true
}

func harpoonPerformanceOptions(options map[int]string) []performancepresentation.Option {
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
