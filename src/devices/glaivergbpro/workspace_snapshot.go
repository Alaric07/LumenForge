package glaivergbpro

import (
	"fmt"
	"sort"
	"strconv"

	"LumenForge/src/buttonspresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/dpipresentation"
	"LumenForge/src/performancepresentation"
)

// Glaive physical key masks are the IDs persisted by the existing assignment
// file and interpreted by the listener.
var glaiveRGBProVisibleButtonOrder = []int{1, 2, 4, 8, 16, 32, 64}

func (d *Device) DPIDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) ButtonsDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) PerformanceDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) DeviceProfileDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// DPISnapshot projects the existing Glaive DPI authority without HID access.
// The device stores one color for regular stages and another for sniper mode.
func (d *Device) DPISnapshot() (dpipresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.MinDPI < 1 || d.MaxDPI < d.MinDPI || d.DPIAmount < 1 || len(d.DeviceProfile.Profiles) != d.DPIAmount || d.DeviceProfile.DPIColor == nil || d.DeviceProfile.SniperColor == nil {
		return dpipresentation.Snapshot{}, false
	}
	keys := make([]int, 0, len(d.DeviceProfile.Profiles))
	for key := range d.DeviceProfile.Profiles {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	snapshot := dpipresentation.Snapshot{MinimumDPI: d.MinDPI, MaximumDPI: d.MaxDPI, Stages: make([]dpipresentation.Stage, 0, len(keys))}
	for _, key := range keys {
		profile := d.DeviceProfile.Profiles[key]
		name := profile.Name
		if name == "" {
			name = fmt.Sprintf("Stage %d", key+1)
		}
		color := d.DeviceProfile.DPIColor
		if profile.Sniper {
			color = d.DeviceProfile.SniperColor
		}
		activeRegular := !profile.Sniper && key == d.DeviceProfile.Profile
		if activeRegular {
			snapshot.ActiveRegularStageID = strconv.Itoa(key)
		}
		snapshot.Stages = append(snapshot.Stages, dpipresentation.Stage{ID: strconv.Itoa(key), Name: name, DPI: profile.Value, ColorHex: fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue)), Sniper: profile.Sniper, Active: activeRegular || (profile.Sniper && d.SniperMode)})
	}
	return snapshot, true
}

func (d *Device) ButtonsSnapshot() (buttonspresentation.Snapshot, bool) {
	if d == nil || len(d.KeyAssignment) == 0 || len(d.KeyAssignmentTypes) == 0 {
		return buttonspresentation.Snapshot{}, false
	}
	snapshot := buttonspresentation.Snapshot{Buttons: make([]buttonspresentation.Button, 0, len(glaiveRGBProVisibleButtonOrder)), AssignmentTypes: make([]buttonspresentation.AssignmentType, 0, 8)}
	for _, key := range glaiveRGBProVisibleButtonOrder {
		assignment, ok := d.KeyAssignment[key]
		if !ok || assignment.Name == "" {
			return buttonspresentation.Snapshot{}, false
		}
		snapshot.Buttons = append(snapshot.Buttons, buttonspresentation.Button{KeyIndex: key, Name: assignment.Name, Default: assignment.Default, PressAndHold: assignment.ActionHold, OnRelease: assignment.OnRelease, ActionType: assignment.ActionType, ActionCommand: assignment.ActionCommand, IsMacro: assignment.IsMacro, ProfileSwitch: assignment.ProfileSwitch})
	}
	for _, id := range []int{0, 1, 2, 3, 8, 9, 10, 11} {
		label, ok := d.KeyAssignmentTypes[id]
		if !ok || label == "" {
			return buttonspresentation.Snapshot{}, false
		}
		snapshot.AssignmentTypes = append(snapshot.AssignmentTypes, buttonspresentation.AssignmentType{ID: uint8(id), Label: label})
	}
	return snapshot, true
}

func (d *Device) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.PollingRates) == 0 || d.DeviceProfile.AngleSnapping < 0 || d.DeviceProfile.AngleSnapping > 1 {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.PollingRates[d.DeviceProfile.PollingRate]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	keys := make([]int, 0, len(d.PollingRates))
	for value := range d.PollingRates {
		keys = append(keys, value)
	}
	sort.Ints(keys)
	options := make([]performancepresentation.Option, 0, len(keys))
	for _, value := range keys {
		options = append(options, performancepresentation.Option{Value: value, Label: d.PollingRates[value]})
	}
	return performancepresentation.Snapshot{PollingRate: &performancepresentation.SelectSetting{Value: d.DeviceProfile.PollingRate, Options: options}, AngleSnapping: &performancepresentation.ToggleSetting{Enabled: d.DeviceProfile.AngleSnapping == 1}}, true
}

func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil {
		return deviceprofilepresentation.Snapshot{}, false
	}
	snapshot := deviceprofilepresentation.Snapshot{Supported: true}
	for name, profile := range d.UserProfiles {
		if profile == nil {
			continue
		}
		snapshot.Profiles = append(snapshot.Profiles, name)
		if profile.Active {
			snapshot.ActiveProfile = name
		}
	}
	sort.Strings(snapshot.Profiles)
	return snapshot, snapshot.ActiveProfile != "" && len(snapshot.Profiles) > 0
}
