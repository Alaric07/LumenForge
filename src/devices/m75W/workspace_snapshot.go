package m75W

import (
	"LumenForge/src/buttonspresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/dpipresentation"
	"LumenForge/src/performancepresentation"
	"LumenForge/src/sleeptimerpresentation"
	"fmt"
	"sort"
	"strconv"
)

var m75WVisibleButtonOrder = []int{1, 2, 4, 8, 16, 32, 64, 128}

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
func (d *Device) SleepTimerDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) DPISnapshot() (dpipresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.MinDPI < 1 || d.MaxDPI < d.MinDPI || d.DPIAmount < 1 || len(d.DeviceProfile.Profiles) != d.DPIAmount {
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
		if profile.Color == nil {
			return dpipresentation.Snapshot{}, false
		}
		name := profile.Name
		if name == "" {
			name = fmt.Sprintf("Stage %d", key+1)
		}
		active := !profile.Sniper && key == d.DeviceProfile.Profile
		if active {
			snapshot.ActiveRegularStageID = strconv.Itoa(key)
		}
		snapshot.Stages = append(snapshot.Stages, dpipresentation.Stage{ID: strconv.Itoa(key), Name: name, DPI: profile.Value, ColorHex: fmt.Sprintf("#%02x%02x%02x", uint8(profile.Color.Red), uint8(profile.Color.Green), uint8(profile.Color.Blue)), Sniper: profile.Sniper, Active: active || (profile.Sniper && d.SniperMode)})
	}
	return snapshot, snapshot.ActiveRegularStageID != ""
}

func (d *Device) ButtonsSnapshot() (buttonspresentation.Snapshot, bool) {
	if d == nil || len(d.KeyAssignmentTypes) == 0 {
		return buttonspresentation.Snapshot{}, false
	}
	snapshot := buttonspresentation.Snapshot{Buttons: make([]buttonspresentation.Button, 0, len(m75WVisibleButtonOrder))}
	for _, key := range m75WVisibleButtonOrder {
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

func m75WOptions(options map[int]string) []performancepresentation.Option {
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
func (d *Device) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.PollingRates) == 0 || len(d.SwitchModes) == 0 || len(d.LiftHeights) == 0 || d.DeviceProfile.AngleSnapping < 0 || d.DeviceProfile.AngleSnapping > 1 {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.PollingRates[d.DeviceProfile.PollingRate]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.SwitchModes[d.DeviceProfile.ButtonOptimization]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.LiftHeights[d.DeviceProfile.LiftHeight]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	polling, optimization, lift := m75WOptions(d.PollingRates), m75WOptions(d.SwitchModes), m75WOptions(d.LiftHeights)
	if polling == nil || optimization == nil || lift == nil {
		return performancepresentation.Snapshot{}, false
	}
	return performancepresentation.Snapshot{PollingRate: &performancepresentation.SelectSetting{Value: d.DeviceProfile.PollingRate, Options: polling}, ButtonOptimization: &performancepresentation.SelectSetting{Value: d.DeviceProfile.ButtonOptimization, Options: optimization}, AngleSnapping: &performancepresentation.ToggleSetting{Enabled: d.DeviceProfile.AngleSnapping == 1}, LiftHeight: &performancepresentation.SelectSetting{Value: d.DeviceProfile.LiftHeight, Options: lift}}, true
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
func (d *Device) SleepTimerSnapshot() (sleeptimerpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.SleepModes) == 0 {
		return sleeptimerpresentation.Snapshot{}, false
	}
	keys := make([]int, 0, len(d.SleepModes))
	for key := range d.SleepModes {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	snapshot := sleeptimerpresentation.Snapshot{Value: d.GetSleepMode(), Options: make([]sleeptimerpresentation.Option, 0, len(keys))}
	found := false
	for _, key := range keys {
		label := d.SleepModes[key]
		if label == "" {
			return sleeptimerpresentation.Snapshot{}, false
		}
		snapshot.Options = append(snapshot.Options, sleeptimerpresentation.Option{Value: key, Label: label})
		found = found || key == snapshot.Value
	}
	return snapshot, found
}
