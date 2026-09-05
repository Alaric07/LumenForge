package harpoonrgbpro

import (
	"fmt"
	"sort"
	"strconv"

	"LumenForge/src/dpipresentation"
)

// DPIDeviceID identifies the device whose current profile state is presented.
func (d *Device) DPIDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// DPISnapshot projects the existing DeviceProfile DPI authority without HID
// access or persistence changes.
func (d *Device) DPISnapshot() (dpipresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.MinDPI < 1 || d.MaxDPI < d.MinDPI || len(d.DeviceProfile.Profiles) == 0 {
		return dpipresentation.Snapshot{}, false
	}

	keys := make([]int, 0, len(d.DeviceProfile.Profiles))
	for key := range d.DeviceProfile.Profiles {
		keys = append(keys, key)
	}
	sort.Ints(keys)

	snapshot := dpipresentation.Snapshot{
		MinimumDPI: d.MinDPI,
		MaximumDPI: d.MaxDPI,
		Stages:     make([]dpipresentation.Stage, 0, len(keys)),
	}
	for _, key := range keys {
		profile := d.DeviceProfile.Profiles[key]
		name := profile.Name
		if name == "" {
			name = fmt.Sprintf("Stage %d", key+1)
		}
		activeRegular := !profile.Sniper && key == d.DeviceProfile.Profile
		if activeRegular {
			snapshot.ActiveRegularStageID = strconv.Itoa(key)
		}
		colorHex := "#000000"
		if profile.Color != nil {
			colorHex = fmt.Sprintf("#%02x%02x%02x", uint8(profile.Color.Red), uint8(profile.Color.Green), uint8(profile.Color.Blue))
		}
		snapshot.Stages = append(snapshot.Stages, dpipresentation.Stage{
			ID: strconv.Itoa(key), Name: name, DPI: profile.Value, ColorHex: colorHex,
			Sniper: profile.Sniper, Active: activeRegular || (profile.Sniper && d.SniperMode),
		})
	}
	return snapshot, true
}
