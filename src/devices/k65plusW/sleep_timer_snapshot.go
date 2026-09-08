package k65plusW

import (
	"LumenForge/src/sleeptimerpresentation"
	"sort"
	"strings"
)

func (d *Device) SleepTimerDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) SleepTimerSnapshot() (sleeptimerpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.SleepModes) == 0 {
		return sleeptimerpresentation.Snapshot{}, false
	}
	ids := make([]int, 0, len(d.SleepModes))
	for id := range d.SleepModes {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	s := sleeptimerpresentation.Snapshot{Value: d.DeviceProfile.SleepMode}
	found := false
	for _, id := range ids {
		if strings.TrimSpace(d.SleepModes[id]) == "" {
			return sleeptimerpresentation.Snapshot{}, false
		}
		s.Options = append(s.Options, sleeptimerpresentation.Option{Value: id, Label: d.SleepModes[id]})
		found = found || id == s.Value
	}
	return s, found
}
