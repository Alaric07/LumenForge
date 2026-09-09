package darkcorergbproWU

import (
	"LumenForge/src/buttonspresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/dpipresentation"
	"LumenForge/src/performancepresentation"
	"LumenForge/src/rgb"
	"fmt"
	"sort"
	"strconv"
)

var darkcorergbproWUVisibleButtonOrder = []int{128, 64, 32, 16, 8, 4, 2, 1}

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
func darkcorergbproWUColorOK(c rgb.Color) bool {
	return c.Red >= 0 && c.Red <= 255 && c.Green >= 0 && c.Green <= 255 && c.Blue >= 0 && c.Blue <= 255
}
func (d *Device) DPISnapshot() (dpipresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.MinDPI < 1 || d.MaxDPI < d.MinDPI || d.DPIAmount < 1 || len(d.DeviceProfile.Profiles) != d.DPIAmount {
		return dpipresentation.Snapshot{}, false
	}
	if d.DeviceProfile.DPIColor == nil || !darkcorergbproWUColorOK(*d.DeviceProfile.DPIColor) {
		return dpipresentation.Snapshot{}, false
	}
	regularColor := d.DeviceProfile.DPIColor
	keys := make([]int, 0, len(d.DeviceProfile.Profiles))
	for key := range d.DeviceProfile.Profiles {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	snapshot := dpipresentation.Snapshot{MinimumDPI: d.MinDPI, MaximumDPI: d.MaxDPI, Stages: make([]dpipresentation.Stage, 0, len(keys))}
	hasSniper := false
	for _, key := range keys {
		profile := d.DeviceProfile.Profiles[key]
		if profile.Name == "" || profile.Value < uint16(d.MinDPI) || profile.Value > uint16(d.MaxDPI) {
			return dpipresentation.Snapshot{}, false
		}
		color := regularColor
		active := !profile.Sniper && key == d.DeviceProfile.Profile
		if active {
			snapshot.ActiveRegularStageID = strconv.Itoa(key)
		}
		hasSniper = hasSniper || profile.Sniper
		snapshot.Stages = append(snapshot.Stages, dpipresentation.Stage{ID: strconv.Itoa(key), Name: profile.Name, DPI: profile.Value, ColorHex: fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue)), Sniper: profile.Sniper, Active: active || (profile.Sniper && d.SniperMode)})
	}
	return snapshot, hasSniper && snapshot.ActiveRegularStageID != ""
}
func (d *Device) ButtonsSnapshot() (buttonspresentation.Snapshot, bool) {
	if d == nil || len(d.KeyAssignmentTypes) == 0 {
		return buttonspresentation.Snapshot{}, false
	}
	snapshot := buttonspresentation.Snapshot{Buttons: make([]buttonspresentation.Button, 0, len(darkcorergbproWUVisibleButtonOrder))}
	for _, key := range darkcorergbproWUVisibleButtonOrder {
		a, ok := d.KeyAssignment[key]
		if !ok || a.Name == "" {
			return buttonspresentation.Snapshot{}, false
		}
		snapshot.Buttons = append(snapshot.Buttons, buttonspresentation.Button{KeyIndex: key, Name: a.Name, Default: a.Default, PressAndHold: a.ActionHold, OnRelease: a.OnRelease, ActionType: a.ActionType, ActionCommand: a.ActionCommand, IsMacro: a.IsMacro, ProfileSwitch: a.ProfileSwitch})
	}
	keys := make([]int, 0, len(d.KeyAssignmentTypes))
	for key := range d.KeyAssignmentTypes {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	for _, key := range keys {
		label := d.KeyAssignmentTypes[key]
		if label == "" {
			return buttonspresentation.Snapshot{}, false
		}
		snapshot.AssignmentTypes = append(snapshot.AssignmentTypes, buttonspresentation.AssignmentType{ID: uint8(key), Label: label})
	}
	return snapshot, true
}
func darkcorergbproWUOptions(options map[int]string) []performancepresentation.Option {
	if len(options) == 0 {
		return nil
	}
	keys := make([]int, 0, len(options))
	for key := range options {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	result := make([]performancepresentation.Option, 0, len(keys))
	for _, key := range keys {
		if options[key] == "" {
			return nil
		}
		result = append(result, performancepresentation.Option{Value: key, Label: options[key]})
	}
	return result
}
func darkcorergbproWUOptionsOK(options map[int]string) bool {
	return darkcorergbproWUOptions(options) != nil
}
func (d *Device) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.AngleSnapping < 0 || d.DeviceProfile.AngleSnapping > 1 || !darkcorergbproWUOptionsOK(d.SwitchModes) {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.SwitchModes[d.DeviceProfile.ButtonOptimization]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	snapshot := performancepresentation.Snapshot{AngleSnapping: &performancepresentation.ToggleSetting{Enabled: d.DeviceProfile.AngleSnapping == 1}}
	snapshot.ButtonOptimization = &performancepresentation.SelectSetting{Value: d.DeviceProfile.ButtonOptimization, Options: darkcorergbproWUOptions(d.SwitchModes)}
	if !darkcorergbproWUOptionsOK(d.PollingRates) {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.PollingRates[d.DeviceProfile.PollingRate]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	snapshot.PollingRate = &performancepresentation.SelectSetting{Value: d.DeviceProfile.PollingRate, Options: darkcorergbproWUOptions(d.PollingRates)}
	return snapshot, true
}
func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil || len(d.UserProfiles) == 0 {
		return deviceprofilepresentation.Snapshot{}, false
	}
	snapshot := deviceprofilepresentation.Snapshot{Supported: true}
	for name, profile := range d.UserProfiles {
		if name == "" || profile == nil {
			return deviceprofilepresentation.Snapshot{}, false
		}
		snapshot.Profiles = append(snapshot.Profiles, name)
		if profile.Active {
			if snapshot.ActiveProfile != "" {
				return deviceprofilepresentation.Snapshot{}, false
			}
			snapshot.ActiveProfile = name
		}
	}
	sort.Strings(snapshot.Profiles)
	snapshot = deviceprofilepresentation.WithMutationCapabilities(snapshot, d)
	return snapshot, snapshot.ActiveProfile != ""
}
