package glaivergb

import (
	"LumenForge/src/buttonspresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/dpipresentation"
	"LumenForge/src/performancepresentation"
	"fmt"
	"sort"
	"strconv"
)

var glaiveRGBVisibleButtonOrder = []int{1, 2, 4, 8, 16, 32}

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

func (d *Device) DPISnapshot() (dpipresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.MinDPI < 1 || d.MaxDPI < d.MinDPI || d.DPIAmount < 1 || len(d.DeviceProfile.Profiles) != d.DPIAmount || d.DeviceProfile.DPIColor == nil || d.DeviceProfile.SniperColor == nil {
		return dpipresentation.Snapshot{}, false
	}
	keys := make([]int, 0, len(d.DeviceProfile.Profiles))
	for key := range d.DeviceProfile.Profiles {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	s := dpipresentation.Snapshot{MinimumDPI: d.MinDPI, MaximumDPI: d.MaxDPI, Stages: make([]dpipresentation.Stage, 0, len(keys))}
	for _, key := range keys {
		p := d.DeviceProfile.Profiles[key]
		name := p.Name
		if name == "" {
			name = fmt.Sprintf("Stage %d", key+1)
		}
		c := d.DeviceProfile.DPIColor
		if p.Sniper {
			c = d.DeviceProfile.SniperColor
		}
		active := !p.Sniper && key == d.DeviceProfile.Profile
		if active {
			s.ActiveRegularStageID = strconv.Itoa(key)
		}
		s.Stages = append(s.Stages, dpipresentation.Stage{ID: strconv.Itoa(key), Name: name, DPI: p.Value, ColorHex: fmt.Sprintf("#%02x%02x%02x", uint8(c.Red), uint8(c.Green), uint8(c.Blue)), Sniper: p.Sniper, Active: active || (p.Sniper && d.SniperMode)})
	}
	return s, true
}
func (d *Device) ButtonsSnapshot() (buttonspresentation.Snapshot, bool) {
	if d == nil || len(d.KeyAssignment) == 0 || len(d.KeyAssignmentTypes) == 0 {
		return buttonspresentation.Snapshot{}, false
	}
	s := buttonspresentation.Snapshot{Buttons: make([]buttonspresentation.Button, 0, len(glaiveRGBVisibleButtonOrder)), AssignmentTypes: make([]buttonspresentation.AssignmentType, 0, 8)}
	for _, k := range glaiveRGBVisibleButtonOrder {
		a, ok := d.KeyAssignment[k]
		if !ok || a.Name == "" {
			return buttonspresentation.Snapshot{}, false
		}
		s.Buttons = append(s.Buttons, buttonspresentation.Button{KeyIndex: k, Name: a.Name, Default: a.Default, PressAndHold: a.ActionHold, OnRelease: a.OnRelease, ActionType: a.ActionType, ActionCommand: a.ActionCommand, IsMacro: a.IsMacro, ProfileSwitch: a.ProfileSwitch})
	}
	for _, id := range []int{0, 1, 2, 3, 8, 9, 10, 11} {
		label, ok := d.KeyAssignmentTypes[id]
		if !ok || label == "" {
			return buttonspresentation.Snapshot{}, false
		}
		s.AssignmentTypes = append(s.AssignmentTypes, buttonspresentation.AssignmentType{ID: uint8(id), Label: label})
	}
	return s, true
}
func glaiveRGBOptions(values map[int]string) []performancepresentation.Option {
	keys := make([]int, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([]performancepresentation.Option, 0, len(keys))
	for _, k := range keys {
		if values[k] == "" {
			return nil
		}
		out = append(out, performancepresentation.Option{Value: k, Label: values[k]})
	}
	return out
}
func (d *Device) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.PollingRates) == 0 || len(d.ButtonOptimizations) == 0 || len(d.LiftHeights) == 0 || d.DeviceProfile.AngleSnapping < 0 || d.DeviceProfile.AngleSnapping > 1 {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.PollingRates[d.DeviceProfile.PollingRate]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.ButtonOptimizations[d.DeviceProfile.ButtonOptimization]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.LiftHeights[d.DeviceProfile.LiftHeight]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	return performancepresentation.Snapshot{PollingRate: &performancepresentation.SelectSetting{Value: d.DeviceProfile.PollingRate, Options: glaiveRGBOptions(d.PollingRates)}, ButtonOptimization: &performancepresentation.SelectSetting{Value: d.DeviceProfile.ButtonOptimization, Options: glaiveRGBOptions(d.ButtonOptimizations)}, AngleSnapping: &performancepresentation.ToggleSetting{Enabled: d.DeviceProfile.AngleSnapping == 1}, LiftHeight: &performancepresentation.SelectSetting{Value: d.DeviceProfile.LiftHeight, Options: glaiveRGBOptions(d.LiftHeights)}}, true
}
func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil {
		return deviceprofilepresentation.Snapshot{}, false
	}
	s := deviceprofilepresentation.Snapshot{Supported: true}
	for n, p := range d.UserProfiles {
		if p == nil {
			continue
		}
		s.Profiles = append(s.Profiles, n)
		if p.Active {
			s.ActiveProfile = n
		}
	}
	sort.Strings(s.Profiles)
	return s, s.ActiveProfile != "" && len(s.Profiles) > 0
}
