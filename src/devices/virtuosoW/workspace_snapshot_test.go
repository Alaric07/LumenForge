package virtuosoW

import "testing"

func virtuosoWorkspaceProfile() *DeviceProfile {
	labels := []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"}
	profile := &DeviceProfile{Equalizers: make(map[int]Equalizer), SleepMode: 15, DisableMicIndicator: 1}
	for index, label := range labels {
		profile.Equalizers[index+1] = Equalizer{Name: label}
	}
	return profile
}

func TestWorkspaceSnapshotsExposeOnlyCompleteVirtuosoWirelessCapabilities(t *testing.T) {
	profile := virtuosoWorkspaceProfile()
	d := &Device{Serial: "virtuoso-w", DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {}}, SleepModes: map[int]string{0: "Off", 1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}, SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v", profiles)
	}
	sleep, ok := d.SleepTimerSnapshot()
	if !ok || len(sleep.Options) != 7 {
		t.Fatalf("sleep=%#v", sleep)
	}
	wantSleep := []int{0, 1, 5, 10, 15, 30, 60}
	for index, value := range wantSleep {
		if sleep.Options[index].Value != value {
			t.Fatalf("sleep options=%#v", sleep.Options)
		}
	}
	headset, ok := d.HeadsetSnapshot()
	if !ok || headset.MuteIndicator == nil || headset.MuteIndicator.Value != 1 || headset.Sidetone != nil || len(headset.Assignments) != 0 {
		t.Fatalf("headset=%#v", headset)
	}
	wantBands := []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"}
	for index, label := range wantBands {
		if headset.Equalizer[index].Label != label {
			t.Fatalf("equalizer=%#v", headset.Equalizer)
		}
	}
	if got := headset.MuteIndicator.Options; len(got) != 2 || got[0].Value != 0 || got[0].Label != "Disabled" || got[1].Value != 1 || got[1].Label != "Enabled" {
		t.Fatalf("mute options=%#v", got)
	}
}

func TestWorkspaceSnapshotsFailClosedForIncompleteVirtuosoWirelessProfiles(t *testing.T) {
	d := &Device{Serial: "virtuoso-w", DeviceProfile: virtuosoWorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": nil}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("incomplete profile state was exposed")
	}
	d.UserProfiles = map[string]*DeviceProfile{"Default": {Active: true}}
	d.DeviceProfile.Equalizers = nil
	if _, ok := d.HeadsetSnapshot(); ok {
		t.Fatal("incomplete equalizers were exposed")
	}
}
