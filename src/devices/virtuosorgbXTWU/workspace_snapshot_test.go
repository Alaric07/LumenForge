package virtuosorgbXTWU

import "testing"

func virtuosoRGBXTWorkspaceProfile() *DeviceProfile {
	profile := &DeviceProfile{Equalizers: map[int]Equalizer{}, DisableMicIndicator: 1, SideTone: 1, SideToneValue: 50}
	for index, label := range []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"} {
		profile.Equalizers[index+1] = Equalizer{Name: label}
	}
	return profile
}

func TestWorkspaceSnapshotsExposeOnlyCompleteVirtuosoRGBXTUSBCapabilities(t *testing.T) {
	d := &Device{Serial: "virtuoso-rgb-xt-wu", Usb: true, DeviceProfile: virtuosoRGBXTWorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {}}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}, SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v", profiles)
	}
	headset, ok := d.HeadsetSnapshot()
	if !ok || headset.MuteIndicator == nil || headset.Sidetone == nil || len(headset.Assignments) != 0 || len(headset.Equalizer) != 10 {
		t.Fatalf("headset=%#v", headset)
	}
	if headset.Equalizer[0].Label != "32" || headset.Equalizer[9].Label != "16K" {
		t.Fatalf("equalizer=%#v", headset.Equalizer)
	}
	if got := headset.Sidetone; len(got.Options) != 2 || got.Options[0].Value != 0 || got.Options[0].Label != "Disabled" || got.Options[1].Value != 1 || got.Options[1].Label != "Enabled" || got.ValueRange.Minimum != 1 || got.ValueRange.Maximum != 100 || got.ValueRange.Step != 1 {
		t.Fatalf("sidetone=%#v", got)
	}
	if _, ok := interface{}(d).(interface{ SleepTimerDeviceID() string }); ok {
		t.Fatal("USB Virtuoso RGB XT exposed dormant sleep timer support")
	}
}

func TestWorkspaceSnapshotsFailClosedForIncompleteVirtuosoRGBXTUSBProfiles(t *testing.T) {
	d := &Device{Serial: "virtuoso-rgb-xt-wu", DeviceProfile: virtuosoRGBXTWorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": nil}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}, SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("incomplete profile state was exposed")
	}
	d.UserProfiles = map[string]*DeviceProfile{"Default": {Active: true}}
	d.DeviceProfile.SideToneValue = 0
	if _, ok := d.HeadsetSnapshot(); ok {
		t.Fatal("invalid sidetone value was exposed")
	}
}
