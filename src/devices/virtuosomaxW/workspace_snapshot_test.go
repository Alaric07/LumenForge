package virtuosomaxW

import "testing"

func virtuosoMAXWorkspaceProfile() *DeviceProfile {
	profile := &DeviceProfile{Equalizers: make(map[int]Equalizer), SleepMode: 15, DisableMicIndicator: 1, NoiseCancellation: 0, SideTone: 1, SideToneValue: 50, LeftWheel: 1, RightWheel: 2}
	for index, label := range []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"} {
		profile.Equalizers[index+1] = Equalizer{Name: label}
	}
	return profile
}

func TestWorkspaceSnapshotsExposeOnlyCompleteVirtuosoMAXCapabilities(t *testing.T) {
	d := &Device{Serial: "virtuoso-max", DeviceProfile: virtuosoMAXWorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {}}, SleepModes: map[int]string{0: "Off", 1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}, NoiseCancellations: map[int]string{0: "Off", 1: "On", 2: "Transparency"}, SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}, WheelOptions: map[int]string{1: "System Volume", 2: "Bluetooth Volume"}}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v", profiles)
	}
	sleep, ok := d.SleepTimerSnapshot()
	if !ok || len(sleep.Options) != 7 || sleep.Options[0].Value != 0 || sleep.Options[6].Value != 60 {
		t.Fatalf("sleep=%#v", sleep)
	}
	headset, ok := d.HeadsetSnapshot()
	if !ok || headset.MuteIndicator == nil || headset.NoiseCancellation == nil || headset.Sidetone == nil || len(headset.Wheels) != 2 || len(headset.Assignments) != 0 {
		t.Fatalf("headset=%#v", headset)
	}
	if headset.Sidetone.ValueRange.Minimum != 1 || headset.Sidetone.ValueRange.Maximum != 100 || headset.Sidetone.ValueRange.Step != 1 {
		t.Fatalf("sidetone=%#v", headset.Sidetone)
	}
	if headset.Wheels[0].ID != 1 || headset.Wheels[0].Label != "Left Wheel" || headset.Wheels[0].Setting.Value != 1 || headset.Wheels[1].ID != 2 || headset.Wheels[1].Label != "Right Wheel" || headset.Wheels[1].Setting.Value != 2 {
		t.Fatalf("wheels=%#v", headset.Wheels)
	}
}

func TestWorkspaceSnapshotsFailClosedForIncompleteVirtuosoMAXState(t *testing.T) {
	d := &Device{Serial: "virtuoso-max", DeviceProfile: virtuosoMAXWorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}, NoiseCancellations: map[int]string{0: "Off", 1: "On", 2: "Transparency"}, SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}, WheelOptions: map[int]string{1: "System Volume", 2: "Bluetooth Volume"}}
	d.WheelOptions = map[int]string{1: "System Volume"}
	if _, ok := d.HeadsetSnapshot(); ok {
		t.Fatal("incomplete wheel options were exposed")
	}
}
