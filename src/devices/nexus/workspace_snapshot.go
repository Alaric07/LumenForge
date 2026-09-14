package nexus

import (
	"LumenForge/src/screenpresentation"
	"LumenForge/src/stats"
	"fmt"
	"sort"
)

// ScreenDeviceID and ScreenSnapshot expose NEXUS's existing profile selector
// without changing LCD rendering, profile persistence, or device lifecycle.
func (d *Device) ScreenDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) ScreenSnapshot() (screenpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || !d.HasLCD || d.LCDProfiles == nil {
		return screenpresentation.Snapshot{}, false
	}

	snapshot := screenpresentation.Snapshot{Available: true, Selected: d.DeviceProfile.LCDMode}
	for id, profile := range d.LCDProfiles.Profiles {
		if id == "default" {
			continue
		}
		if id == "" || profile.Name == "" {
			return screenpresentation.Snapshot{}, false
		}
		snapshot.Options = append(snapshot.Options, screenpresentation.Option{ID: id, Label: profile.Name, Selected: id == snapshot.Selected})
	}
	for serial, deviceList := range stats.GetAIOStats() {
		for channel, device := range deviceList.Devices {
			if serial == "" || device.Device == "" {
				return screenpresentation.Snapshot{}, false
			}
			id := fmt.Sprintf("%s;%d", serial, channel)
			snapshot.Options = append(snapshot.Options, screenpresentation.Option{ID: id, Label: device.Device, Selected: id == snapshot.Selected})
		}
	}
	if len(snapshot.Options) == 0 {
		return screenpresentation.Snapshot{}, false
	}
	sort.Slice(snapshot.Options, func(i, j int) bool {
		if snapshot.Options[i].Label == snapshot.Options[j].Label {
			return snapshot.Options[i].ID < snapshot.Options[j].ID
		}
		return snapshot.Options[i].Label < snapshot.Options[j].Label
	})
	selected := 0
	for _, option := range snapshot.Options {
		if option.Selected {
			selected++
		}
	}
	if selected != 1 {
		return screenpresentation.Snapshot{}, false
	}
	return snapshot, true
}
