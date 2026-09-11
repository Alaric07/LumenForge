package virtuosorgbXTW

import "testing"

func virtuosoRGBXTWorkspaceProfile() *DeviceProfile {
	profile := &DeviceProfile{Equalizers: map[int]Equalizer{}, SleepMode: 15, DisableMicIndicator: 1, SideTone: 1, SideToneValue: 50}
	for index, label := range []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"} {
		profile.Equalizers[index+1] = Equalizer{Name: label}
	}
	return profile
}

func TestWorkspaceSnapshotsExposeOnlyCompleteVirtuosoRGBXTWirelessCapabilities(t *testing.T) {
	d := &Device{Serial: "virtuoso-rgb-xt-w", DeviceProfile: virtuosoRGBXTWorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {}}, SleepModes: map[int]string{0: "Off", 1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}, SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v", profiles)
	}
	sleep, ok := d.SleepTimerSnapshot()
	if !ok || len(sleep.Options) != 7 {
		t.Fatalf("sleep=%#v", sleep)
	}
	for index, value := range []int{0, 1, 5, 10, 15, 30, 60} {
		if sleep.Options[index].Value != value {
			t.Fatalf("sleep options=%#v", sleep.Options)
		}
	}
	headset, ok := d.HeadsetSnapshot()
	if !ok || headset.MuteIndicator == nil || headset.Sidetone == nil || len(headset.Assignments) != 0 || len(headset.Equalizer) != 10 {
		t.Fatalf("headset=%#v", headset)
	}
	for index, label := range []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"} {
		if headset.Equalizer[index].Label != label {
			t.Fatalf("equalizer=%#v", headset.Equalizer)
		}
	}
	if got := headset.MuteIndicator.Options; len(got) != 2 || got[0].Value != 0 || got[0].Label != "Disabled" || got[1].Value != 1 || got[1].Label != "Enabled" {
		t.Fatalf("mute options=%#v", got)
	}
	if got := headset.Sidetone; len(got.Options) != 2 || got.Options[0].Value != 0 || got.Options[0].Label != "Disabled" || got.Options[1].Value != 1 || got.Options[1].Label != "Enabled" || got.ValueRange.Minimum != 1 || got.ValueRange.Maximum != 100 || got.ValueRange.Step != 1 {
		t.Fatalf("sidetone=%#v", got)
	}
}

func TestWorkspaceSnapshotsFailClosedForIncompleteVirtuosoRGBXTWirelessProfiles(t *testing.T) {
	d := &Device{Serial: "virtuoso-rgb-xt-w", DeviceProfile: virtuosoRGBXTWorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": nil}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}, SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("incomplete profile state was exposed")
	}
	d.UserProfiles = map[string]*DeviceProfile{"Default": {Active: true}}
	d.DeviceProfile.Equalizers = nil
	if _, ok := d.HeadsetSnapshot(); ok {
		t.Fatal("incomplete equalizers were exposed")
	}
	d.DeviceProfile = virtuosoRGBXTWorkspaceProfile()
	d.SideToneModes = map[int]string{0: "Disabled"}
	if _, ok := d.HeadsetSnapshot(); ok {
		t.Fatal("incomplete sidetone state was exposed")
	}
}
