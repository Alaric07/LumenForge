package k65rgb

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
	options, ok := k65RGBPerformanceOptions(d.PollingRates)
	if !ok {
		return performancepresentation.Snapshot{}, false
	}
	return performancepresentation.Snapshot{SaveBooleanSettings: true, PollingRate: &performancepresentation.SelectSetting{Value: d.DeviceProfile.PollingRate, Options: options}, BooleanSettings: []performancepresentation.BooleanSetting{{ID: "perf_winKey", Label: "Disable Win Key", Enabled: d.DeviceProfile.DisableWinKey}, {ID: "perf_shiftTab", Label: "Disable Shift + Tab", Enabled: d.DeviceProfile.DisableShiftTab}, {ID: "perf_altTab", Label: "Disable Alt + Tab", Enabled: d.DeviceProfile.DisableAltTab}, {ID: "perf_altF4", Label: "Disable Alt + F4", Enabled: d.DeviceProfile.DisableAltF4}}}, true
}
func k65RGBPerformanceOptions(options map[int]string) ([]performancepresentation.Option, bool) {
	keys := make([]int, 0, len(options))
	for value := range options {
		keys = append(keys, value)
	}
	sort.Ints(keys)
	presented := make([]performancepresentation.Option, 0, len(keys))
	for _, value := range keys {
		if strings.TrimSpace(options[value]) == "" {
			return nil, false
		}
		presented = append(presented, performancepresentation.Option{Value: value, Label: options[value]})
	}
	return presented, len(presented) > 0
}
