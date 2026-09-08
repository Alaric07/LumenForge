package k70max

import (
	"LumenForge/src/performancepresentation"
	"sort"
	"strings"
)

func (d *Device) PerformanceDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.PollingRates) == 0 {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.PollingRates[d.DeviceProfile.PollingRate]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	ids := make([]int, 0, len(d.PollingRates))
	for id := range d.PollingRates {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	options := make([]performancepresentation.Option, 0, len(ids))
	for _, id := range ids {
		if strings.TrimSpace(d.PollingRates[id]) == "" {
			return performancepresentation.Snapshot{}, false
		}
		options = append(options, performancepresentation.Option{Value: id, Label: d.PollingRates[id]})
	}
	return performancepresentation.Snapshot{SaveBooleanSettings: true, PollingRate: &performancepresentation.SelectSetting{Value: d.DeviceProfile.PollingRate, Options: options}, BooleanSettings: []performancepresentation.BooleanSetting{{ID: "perf_winKey", Label: "Disable Win Key", Enabled: d.DeviceProfile.DisableWinKey}, {ID: "perf_shiftTab", Label: "Disable Shift + Tab", Enabled: d.DeviceProfile.DisableShiftTab}, {ID: "perf_altTab", Label: "Disable Alt + Tab", Enabled: d.DeviceProfile.DisableAltTab}, {ID: "perf_altF4", Label: "Disable Alt + F4", Enabled: d.DeviceProfile.DisableAltF4}}}, true
}
