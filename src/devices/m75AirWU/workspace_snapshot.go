package m75AirWU

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

var m75AirWUVisibleButtonOrder = []int{16, 8, 4, 2, 1}

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
	ks := make([]int, 0, len(d.DeviceProfile.Profiles))
	for k := range d.DeviceProfile.Profiles {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	s := dpipresentation.Snapshot{MinimumDPI: d.MinDPI, MaximumDPI: d.MaxDPI}
	for _, k := range ks {
		p := d.DeviceProfile.Profiles[k]
		if p.Color == nil || p.Color.Red < 0 || p.Color.Red > 255 || p.Color.Green < 0 || p.Color.Green > 255 || p.Color.Blue < 0 || p.Color.Blue > 255 {
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
	return s, s.ActiveRegularStageID != ""
}
func (d *Device) ButtonsSnapshot() (buttonspresentation.Snapshot, bool) {
	if d == nil || len(d.KeyAssignmentTypes) == 0 {
		return buttonspresentation.Snapshot{}, false
	}
	s := buttonspresentation.Snapshot{}
	for _, k := range m75AirWUVisibleButtonOrder {
		a, ok := d.KeyAssignment[k]
		if !ok || a.Name == "" {
			return buttonspresentation.Snapshot{}, false
		}
		s.Buttons = append(s.Buttons, buttonspresentation.Button{KeyIndex: k, Name: a.Name, Default: a.Default, PressAndHold: a.ActionHold, OnRelease: a.OnRelease, ActionType: a.ActionType, ActionCommand: a.ActionCommand, IsMacro: a.IsMacro, ProfileSwitch: a.ProfileSwitch})
	}
	for _, k := range []int{0, 1, 2, 3, 8, 9, 10, 11} {
		l, ok := d.KeyAssignmentTypes[k]
		if !ok || l == "" {
			return buttonspresentation.Snapshot{}, false
		}
		s.AssignmentTypes = append(s.AssignmentTypes, buttonspresentation.AssignmentType{ID: uint8(k), Label: l})
	}
	return s, true
}
func mouseOptions(m map[int]string) []performancepresentation.Option {
	ks := make([]int, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	r := make([]performancepresentation.Option, 0, len(ks))
	for _, k := range ks {
		if m[k] == "" {
			return nil
		}
		r = append(r, performancepresentation.Option{Value: k, Label: m[k]})
	}
	return r
}
func mouseOptionsOK(m map[int]string) bool { return len(m) > 0 && mouseOptions(m) != nil }
func (d *Device) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.SwitchModes[d.DeviceProfile.ButtonOptimization]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	if len(d.PollingRates) == 0 || d.DeviceProfile.AngleSnapping < 0 || d.DeviceProfile.AngleSnapping > 1 || !mouseOptionsOK(d.PollingRates) || !mouseOptionsOK(d.SwitchModes) {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.PollingRates[d.DeviceProfile.PollingRate]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	s := performancepresentation.Snapshot{PollingRate: &performancepresentation.SelectSetting{Value: d.DeviceProfile.PollingRate, Options: mouseOptions(d.PollingRates)}, ButtonOptimization: &performancepresentation.SelectSetting{Value: d.DeviceProfile.ButtonOptimization, Options: mouseOptions(d.SwitchModes)}, AngleSnapping: &performancepresentation.ToggleSetting{Enabled: d.DeviceProfile.AngleSnapping == 1}}
	if !mouseOptionsOK(d.LiftHeights) {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.LiftHeights[d.DeviceProfile.LiftHeight]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	s.LiftHeight = &performancepresentation.SelectSetting{Value: d.DeviceProfile.LiftHeight, Options: mouseOptions(d.LiftHeights)}
	return s, true
}
func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil || len(d.UserProfiles) == 0 {
		return deviceprofilepresentation.Snapshot{}, false
	}
	s := deviceprofilepresentation.Snapshot{Supported: true}
	for n, p := range d.UserProfiles {
		if p == nil {
			return deviceprofilepresentation.Snapshot{}, false
		}
		s.Profiles = append(s.Profiles, n)
		if p.Active {
			s.ActiveProfile = n
		}
	}
	sort.Strings(s.Profiles)
	return s, s.ActiveProfile != ""
}
func (d *Device) SleepTimerSnapshot() (sleeptimerpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.SleepModes) == 0 {
		return sleeptimerpresentation.Snapshot{}, false
	}
	s := sleeptimerpresentation.Snapshot{Value: d.GetSleepMode()}
	for k, l := range d.SleepModes {
		if l == "" {
			return sleeptimerpresentation.Snapshot{}, false
		}
		s.Options = append(s.Options, sleeptimerpresentation.Option{Value: k, Label: l})
	}
	sort.Slice(s.Options, func(i, j int) bool { return s.Options[i].Value < s.Options[j].Value })
	for _, o := range s.Options {
		if o.Value == s.Value {
			return s, true
		}
	}
	return sleeptimerpresentation.Snapshot{}, false
}
