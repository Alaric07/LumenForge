package vanguard99airWU

import (
	"LumenForge/src/controldialpresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/flashtappresentation"
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/optioncolorpresentation"
	"LumenForge/src/performancepresentation"
	"LumenForge/src/rgb"
	"math"
	"sort"
	"strings"
)

func (d *Device) KeyboardAssignmentsDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) KeyboardAssignmentsSnapshot() (keyboardassignmentspresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || len(d.Layouts) == 0 || d.UIKeyboard == "" || d.UIKeyboardRow == "" || len(d.KeyAssignmentTypes) == 0 || !vanguard99airWUContains(d.Layouts, d.DeviceProfile.Layout) || !vanguard99airWUContains(d.DeviceProfile.Profiles, d.DeviceProfile.Profile) {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	k := d.getCurrentKeyboard()
	if k == nil || len(k.Row) == 0 {
		return keyboardassignmentspresentation.Snapshot{}, false
	}
	return keyboardassignmentspresentation.BuildSnapshot(keyboardassignmentspresentation.Source{Profiles: d.DeviceProfile.Profiles, ActiveProfile: d.DeviceProfile.Profile, KeyboardLayouts: d.Layouts, ActiveKeyboardLayout: d.DeviceProfile.Layout, ClusterControlled: d.DeviceProfile.RGBCluster, LayoutClass: d.UIKeyboard, RowLayoutClass: d.UIKeyboardRow, Rows: k.Row, AssignmentTypes: d.KeyAssignmentTypes})
}
func (d *Device) PerformanceDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.PollingRates) == 0 {
		return performancepresentation.Snapshot{}, false
	}
	if _, ok := d.PollingRates[d.DeviceProfile.PollingRate]; !ok {
		return performancepresentation.Snapshot{}, false
	}
	ids := make([]int, 0, len(d.PollingRates))
	for id := range d.PollingRates {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	options := make([]performancepresentation.Option, 0, len(ids))
	for _, id := range ids {
		if strings.TrimSpace(d.PollingRates[id]) == "" {
			return performancepresentation.Snapshot{}, false
		}
		options = append(options, performancepresentation.Option{Value: id, Label: d.PollingRates[id]})
	}
	return performancepresentation.Snapshot{PollingRate: &performancepresentation.SelectSetting{Value: d.DeviceProfile.PollingRate, Options: options}, SaveBooleanSettings: true, BooleanSettings: []performancepresentation.BooleanSetting{{ID: "perf_winKey", Label: "Disable Win Key", Enabled: d.DeviceProfile.DisableWinKey}, {ID: "perf_shiftTab", Label: "Disable Shift + Tab", Enabled: d.DeviceProfile.DisableShiftTab}, {ID: "perf_altTab", Label: "Disable Alt + Tab", Enabled: d.DeviceProfile.DisableAltTab}, {ID: "perf_altF4", Label: "Disable Alt + F4", Enabled: d.DeviceProfile.DisableAltF4}}}, true
}
func (d *Device) DeviceProfileDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || d.DeviceProfile.Keyboards[d.DeviceProfile.Profile] == nil || len(d.UserProfiles) == 0 {
		return deviceprofilepresentation.Snapshot{}, false
	}
	s, active := deviceprofilepresentation.Snapshot{Supported: true}, 0
	for name, p := range d.UserProfiles {
		if name == "" || p == nil {
			return deviceprofilepresentation.Snapshot{}, false
		}
		s.Profiles = append(s.Profiles, name)
		if p.Active {
			active++
			s.ActiveProfile = name
		}
	}
	sort.Strings(s.Profiles)
	if active != 1 || s.ActiveProfile == "" {
		return deviceprofilepresentation.Snapshot{}, false
	}
	return deviceprofilepresentation.WithMutationCapabilities(s, d), true
}
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
	return s, controldialpresentation.Valid(s)
}
func (d *Device) OptionColorsDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) OptionColorsSnapshot() (optioncolorpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.ControlDialOptions) == 0 || len(d.DeviceProfile.ControlDialColors) != len(d.ControlDialOptions) {
		return optioncolorpresentation.Snapshot{}, false
	}
	ids := make([]int, 0, len(d.ControlDialOptions))
	for id := range d.ControlDialOptions {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	s := optioncolorpresentation.Snapshot{Selected: d.DeviceProfile.ControlDial}
	for _, id := range ids {
		color, ok := d.DeviceProfile.ControlDialColors[id]
		if !ok || color == nil || strings.TrimSpace(d.ControlDialOptions[id]) == "" {
			return optioncolorpresentation.Snapshot{}, false
		}
		s.Options = append(s.Options, optioncolorpresentation.Option{Value: id, Label: d.ControlDialOptions[id], Color: rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue}, Action: "control-dial-colors"})
	}
	return s, vanguard99airWUContainsInt(ids, s.Selected)
}
func (d *Device) FlashTapDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) FlashTapSnapshot() (flashtappresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || d.DeviceProfile.FlashTap == nil || len(d.FlashTapModes) == 0 {
		return flashtappresentation.Snapshot{}, false
	}
	k := d.getCurrentKeyboard()
	if k == nil || len(k.Row) == 0 {
		return flashtappresentation.Snapshot{}, false
	}
	f := d.DeviceProfile.FlashTap
	if f.Active < 0 || f.Active > 1 || math.IsNaN(f.Color.Red) || math.IsNaN(f.Color.Green) || math.IsNaN(f.Color.Blue) || math.IsInf(f.Color.Red, 0) || math.IsInf(f.Color.Green, 0) || math.IsInf(f.Color.Blue, 0) {
		return flashtappresentation.Snapshot{}, false
	}
	ids := make([]int, 0, len(d.FlashTapModes))
	for id, label := range d.FlashTapModes {
		if strings.TrimSpace(label) == "" {
			return flashtappresentation.Snapshot{}, false
		}
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if _, ok := d.FlashTapModes[f.Mode]; !ok {
		return flashtappresentation.Snapshot{}, false
	}
	byData, byIndex := map[uint16]int{}, map[int]flashtappresentation.Key{}
	rowIDs := make([]int, 0, len(k.Row))
	for rowID := range k.Row {
		rowIDs = append(rowIDs, rowID)
	}
	sort.Ints(rowIDs)
	for _, rowID := range rowIDs {
		row := k.Row[rowID]
		if len(row.Keys) == 0 {
			return flashtappresentation.Snapshot{}, false
		}
		keyIDs := make([]int, 0, len(row.Keys))
		for keyID := range row.Keys {
			keyIDs = append(keyIDs, keyID)
		}
		sort.Ints(keyIDs)
		for _, keyID := range keyIDs {
			key := row.Keys[keyID]
			index := len(byIndex)
			if key.KeyName == "" {
				return flashtappresentation.Snapshot{}, false
			}
			eligible := !key.OnlyColor && len(key.KeyData) > 0 && !hasNoFlashTap(key.KeyData[0])
			if eligible {
				if _, exists := byData[key.KeyData[0]]; exists {
					return flashtappresentation.Snapshot{}, false
				}
				byData[key.KeyData[0]] = index
			}
			byIndex[index] = flashtappresentation.Key{KeyIndex: index, KeyName: key.KeyName, Eligible: eligible}
		}
	}
	keys := make([]flashtappresentation.Key, 0, len(byIndex))
	for _, key := range byIndex {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].KeyIndex < keys[j].KeyIndex })
	if len(f.Keys) != 2 {
		return flashtappresentation.Snapshot{}, false
	}
	selected, slots := map[int]bool{}, make([]flashtappresentation.SelectedSlot, 0, 2)
	for slot := 0; slot < 2; slot++ {
		key, ok := f.Keys[slot]
		if !ok {
			return flashtappresentation.Snapshot{}, false
		}
		index, ok := byData[uint16(key.KeyData)]
		if !ok || selected[index] {
			return flashtappresentation.Snapshot{}, false
		}
		selected[index] = true
		slots = append(slots, flashtappresentation.SelectedSlot{SlotIndex: slot, KeyIndex: index})
	}
	for i := range keys {
		keys[i].Selected = selected[keys[i].KeyIndex]
	}
	s := flashtappresentation.Snapshot{Supported: true, Active: f.Active == 1, Mode: f.Mode, Keys: keys, SelectedSlots: slots, Color: flashtappresentation.Color{Red: f.Color.Red, Green: f.Color.Green, Blue: f.Color.Blue}}
	for _, id := range ids {
		s.Modes = append(s.Modes, flashtappresentation.Option{Value: id, Label: d.FlashTapModes[id]})
	}
	return s, len(s.Keys) > 0
}
func vanguard99airWUContains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func vanguard99airWUContainsInt(values []int, value int) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
