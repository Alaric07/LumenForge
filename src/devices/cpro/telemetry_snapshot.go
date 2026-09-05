package cpro

import (
	"LumenForge/src/telemetrypresentation"
	"fmt"
	"sort"
)

func (d *Device) TelemetryDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) TelemetrySnapshot() (telemetrypresentation.Snapshot, bool) {
	if d == nil {
		return telemetrypresentation.Snapshot{}, false
	}
	keys := make([]int, 0, len(d.RailVoltages))
	for key := range d.RailVoltages {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	snapshot := telemetrypresentation.Snapshot{Available: true}
	for _, key := range keys {
		if rail := d.RailVoltages[key]; rail != nil && rail.Name != "" {
			snapshot.Rows = append(snapshot.Rows, telemetrypresentation.Row{Label: rail.Name, Value: fmt.Sprintf("%.2f V", rail.Value)})
		}
	}
	return snapshot, len(snapshot.Rows) > 0
}
