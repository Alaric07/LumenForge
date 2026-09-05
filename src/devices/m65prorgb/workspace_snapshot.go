package m65prorgb

import (
	"LumenForge/src/buttonspresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/dpipresentation"
	"LumenForge/src/performancepresentation"
	"fmt"
	"sort"
	"strconv"
)

var m65ProRGBVisibleButtonOrder = []int{1, 2, 4, 8, 16, 32, 64, 128}

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
	if d == nil || d.DeviceProfile == nil || d.MinDPI < 1 || d.MaxDPI < d.MinDPI || d.DPIAmount < 1 || len(d.DeviceProfile.Profiles) != d.DPIAmount {
		return dpipresentation.Snapshot{}, false
	}
	keys := make([]int, 0, len(d.DeviceProfile.Profiles))
	for k := range d.DeviceProfile.Profiles {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	s := dpipresentation.Snapshot{MinimumDPI: d.MinDPI, MaximumDPI: d.MaxDPI, Stages: make([]dpipresentation.Stage, 0, len(keys))}
	for _, k := range keys {
		p := d.DeviceProfile.Profiles[k]
		if p.Color == nil {
			return dpipresentation.Snapshot{}, false
		}
		n := p.Name
		if n == "" {
			n = fmt.Sprintf("Stage %d", k+1)
		}
		a := !p.Sniper && k == d.DeviceProfile.Profile
		if a {
			s.ActiveRegularStageID = strconv.Itoa(k)
		}
		s.Stages = append(s.Stages, dpipresentation.Stage{ID: strconv.Itoa(k), Name: n, DPI: p.Value, ColorHex: fmt.Sprintf("#%02x%02x%02x", uint8(p.Color.Red), uint8(p.Color.Green), uint8(p.Color.Blue)), Sniper: p.Sniper, Active: a || (p.Sniper && d.SniperMode)})
	}
	return s, true
}
func (d *Device) ButtonsSnapshot() (buttonspresentation.Snapshot, bool) {
	if d == nil || len(d.KeyAssignmentTypes) == 0 {
		return buttonspresentation.Snapshot{}, false
	}
	s := buttonspresentation.Snapshot{Buttons: make([]buttonspresentation.Button, 0, len(m65ProRGBVisibleButtonOrder))}
	for _, k := range m65ProRGBVisibleButtonOrder {
		a, ok := d.KeyAssignment[k]
		if !ok || a.Name == "" {
			return buttonspresentation.Snapshot{}, false
		}
		s.Buttons = append(s.Buttons, buttonspresentation.Button{KeyIndex: k, Name: a.Name, Default: a.Default, PressAndHold: a.ActionHold, OnRelease: a.OnRelease, ActionType: a.ActionType, ActionCommand: a.ActionCommand, IsMacro: a.IsMacro, ProfileSwitch: a.ProfileSwitch})
	}
	for _, id := range []int{0, 1, 2, 3, 4, 8, 9, 10, 11} {
		v, ok := d.KeyAssignmentTypes[id]
		if !ok || v == "" {
			return buttonspresentation.Snapshot{}, false
		}
		s.AssignmentTypes = append(s.AssignmentTypes, buttonspresentation.AssignmentType{ID: uint8(id), Label: v})
	}
	return s, true
}
func m65ProRGBOptions(v map[int]string) []performancepresentation.Option {
	keys := make([]int, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([]performancepresentation.Option, 0, len(keys))
	for _, k := range keys {
		if v[k] == "" {
			return nil
		}
		out = append(out, performancepresentation.Option{Value: k, Label: v[k]})
	}
	return out
}
func (d *Device) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.PollingRates) == 0 || len(d.LiftHeights) == 0 || d.DeviceProfile.AngleSnapping < 0 || d.DeviceProfile.AngleSnapping > 1 {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.PollingRates[d.DeviceProfile.PollingRate]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.LiftHeights[d.DeviceProfile.LiftHeight]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	p, l := m65ProRGBOptions(d.PollingRates), m65ProRGBOptions(d.LiftHeights)
	if p == nil || l == nil {
		return performancepresentation.Snapshot{}, false
	}
	return performancepresentation.Snapshot{PollingRate: &performancepresentation.SelectSetting{Value: d.DeviceProfile.PollingRate, Options: p}, AngleSnapping: &performancepresentation.ToggleSetting{Enabled: d.DeviceProfile.AngleSnapping == 1}, LiftHeight: &performancepresentation.SelectSetting{Value: d.DeviceProfile.LiftHeight, Options: l}}, true
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
