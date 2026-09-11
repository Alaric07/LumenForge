package virtuosomaxW

import (
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/headsetpresentation"
	"LumenForge/src/sleeptimerpresentation"
	"sort"
)

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

func (d *Device) HeadsetDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.UserProfiles) == 0 {
		return deviceprofilepresentation.Snapshot{}, false
	}
	s := deviceprofilepresentation.Snapshot{Supported: true}
	active := 0
	for name, profile := range d.UserProfiles {
		if name == "" || profile == nil {
			return deviceprofilepresentation.Snapshot{}, false
		}
		s.Profiles = append(s.Profiles, name)
		if profile.Active {
			active++
			s.ActiveProfile = name
		}
	}
	if active != 1 {
		return deviceprofilepresentation.Snapshot{}, false
	}
	sort.Strings(s.Profiles)
	return deviceprofilepresentation.WithMutationCapabilities(s, d), true
}

func (d *Device) SleepTimerSnapshot() (sleeptimerpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.SleepModes) != 7 {
		return sleeptimerpresentation.Snapshot{}, false
	}
	keys := []int{0, 1, 5, 10, 15, 30, 60}
	s := sleeptimerpresentation.Snapshot{Value: d.GetSleepMode()}
	for _, key := range keys {
		label, ok := d.SleepModes[key]
		if !ok || label == "" {
			return sleeptimerpresentation.Snapshot{}, false
		}
		s.Options = append(s.Options, sleeptimerpresentation.Option{Value: key, Label: label})
	}
	for _, option := range s.Options {
		if option.Value == s.Value {
			return s, true
		}
	}
	return sleeptimerpresentation.Snapshot{}, false
}

func (d *Device) HeadsetSnapshot() (headsetpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.DeviceProfile.Equalizers) != 10 || len(d.MuteIndicators) != 2 || len(d.NoiseCancellations) != 3 || len(d.SideToneModes) != 2 || len(d.WheelOptions) != 2 {
		return headsetpresentation.Snapshot{}, false
	}
	labels := []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"}
	s := headsetpresentation.Snapshot{Equalizer: make([]headsetpresentation.EqualizerBand, 0, len(labels))}
	for index, label := range labels {
		band, ok := d.DeviceProfile.Equalizers[index+1]
		if !ok || band.Name != label {
			return headsetpresentation.Snapshot{}, false
		}
		s.Equalizer = append(s.Equalizer, headsetpresentation.EqualizerBand{ID: index + 1, Label: label, Value: band.Value})
	}
	selectSetting := func(value int, options map[int]string, keys []int) *headsetpresentation.SelectSetting {
		if len(options) != len(keys) {
			return nil
		}
		out := &headsetpresentation.SelectSetting{Value: value}
		for _, key := range keys {
			label, ok := options[key]
			if !ok || label == "" {
				return nil
			}
			out.Options = append(out.Options, headsetpresentation.SelectOption{Value: key, Label: label})
		}
		return out
	}
	if s.MuteIndicator = selectSetting(d.DeviceProfile.DisableMicIndicator, d.MuteIndicators, []int{0, 1}); s.MuteIndicator == nil {
		return headsetpresentation.Snapshot{}, false
	}
	if s.NoiseCancellation = selectSetting(d.DeviceProfile.NoiseCancellation, d.NoiseCancellations, []int{0, 1, 2}); s.NoiseCancellation == nil {
		return headsetpresentation.Snapshot{}, false
	}
	sidetone := selectSetting(d.DeviceProfile.SideTone, d.SideToneModes, []int{0, 1})
	if sidetone == nil {
		return headsetpresentation.Snapshot{}, false
	}
	s.Sidetone = &headsetpresentation.SidetoneSetting{Value: sidetone.Value, Options: sidetone.Options, ValueRange: &headsetpresentation.RangedSetting{Value: d.DeviceProfile.SideToneValue, Minimum: 1, Maximum: 100, Step: 1}}
	for _, wheel := range []struct {
		id    uint8
		label string
		value uint8
	}{{1, "Left Wheel", d.DeviceProfile.LeftWheel}, {2, "Right Wheel", d.DeviceProfile.RightWheel}} {
		setting := selectSetting(int(wheel.value), d.WheelOptions, []int{1, 2})
		if setting == nil {
			return headsetpresentation.Snapshot{}, false
		}
		s.Wheels = append(s.Wheels, headsetpresentation.WheelSetting{ID: wheel.id, Label: wheel.label, Setting: *setting})
	}
	muted := d.MuteStatus == 1
	s.Muted = &muted
	return s, headsetpresentation.Valid(s)
}
