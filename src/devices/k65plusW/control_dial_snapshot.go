package k65plusW

import (
	"LumenForge/src/controldialpresentation"
	"strings"
)

func (d *Device) ControlDialDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) ControlDialSnapshot() (controldialpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.ControlDialOptions) == 0 {
		return controldialpresentation.Snapshot{}, false
	}
	s := controldialpresentation.Snapshot{Available: true, Value: d.DeviceProfile.ControlDial, Options: controldialpresentation.Sorted(d.ControlDialOptions)}
	for _, o := range s.Options {
		if strings.TrimSpace(o.Label) == "" {
			return controldialpresentation.Snapshot{}, false
		}
	}
	return s, controldialpresentation.Valid(s)
}
