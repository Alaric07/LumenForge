package virtuosoWU

import "testing"

func virtuosoWorkspaceProfile() *DeviceProfile {
	labels := []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"}
	profile := &DeviceProfile{Equalizers: make(map[int]Equalizer), DisableMicIndicator: 0}
	for index, label := range labels {
		profile.Equalizers[index+1] = Equalizer{Name: label}
	}
	return profile
}

func TestWorkspaceSnapshotsExposeOnlyCompleteVirtuosoUSBCapabilities(t *testing.T) {
	profile := virtuosoWorkspaceProfile()
	d := &Device{Serial: "virtuoso-wu", Usb: true, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {}}, SleepModes: map[int]string{0: "Off", 1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}, SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v", profiles)
	}
	headset, ok := d.HeadsetSnapshot()
	if !ok || headset.MuteIndicator == nil || headset.Sidetone != nil || len(headset.Assignments) != 0 {
		t.Fatalf("headset=%#v", headset)
	}
	if len(headset.Equalizer) != 10 || headset.Equalizer[0].Label != "32" || headset.Equalizer[9].Label != "16K" {
		t.Fatalf("equalizer=%#v", headset.Equalizer)
	}
	if got := headset.MuteIndicator.Options; len(got) != 2 || got[0].Value != 0 || got[0].Label != "Disabled" || got[1].Value != 1 || got[1].Label != "Enabled" {
		t.Fatalf("mute options=%#v", got)
	}
	if _, ok := interface{}(d).(interface{ SleepTimerDeviceID() string }); ok {
		t.Fatal("USB Virtuoso exposed dormant sleep timer support")
	}
}

func TestWorkspaceSnapshotsFailClosedForIncompleteVirtuosoUSBProfiles(t *testing.T) {
	d := &Device{Serial: "virtuoso-wu", DeviceProfile: virtuosoWorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": nil}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("incomplete profile state was exposed")
	}
	d.UserProfiles = map[string]*DeviceProfile{"Default": {Active: true}}
	d.MuteIndicators = map[int]string{0: "Disabled"}
	if _, ok := d.HeadsetSnapshot(); ok {
		t.Fatal("incomplete mute indicator was exposed")
	}
}
