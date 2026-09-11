package virtuosoSEWU

import "testing"

func virtuosoSEWorkspaceProfile() *DeviceProfile {
	labels := []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"}
	profile := &DeviceProfile{Equalizers: make(map[int]Equalizer), DisableMicIndicator: 0}
	for index, label := range labels {
		profile.Equalizers[index+1] = Equalizer{Name: label}
	}
	return profile
}

func TestWorkspaceSnapshotsExposeOnlyCompleteVirtuosoSEUSBCapabilities(t *testing.T) {
	d := &Device{Serial: "virtuoso-se-wu", Usb: true, DeviceProfile: virtuosoSEWorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {}}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v", profiles)
	}
	headset, ok := d.HeadsetSnapshot()
	if !ok || headset.MuteIndicator == nil || headset.Sidetone != nil || len(headset.Assignments) != 0 || len(headset.Equalizer) != 10 {
		t.Fatalf("headset=%#v", headset)
	}
	if headset.Equalizer[0].Label != "32" || headset.Equalizer[9].Label != "16K" {
		t.Fatalf("equalizer=%#v", headset.Equalizer)
	}
	if got := headset.MuteIndicator.Options; len(got) != 2 || got[0].Value != 0 || got[0].Label != "Disabled" || got[1].Value != 1 || got[1].Label != "Enabled" {
		t.Fatalf("mute options=%#v", got)
	}
	if _, ok := interface{}(d).(interface{ SleepTimerDeviceID() string }); ok {
		t.Fatal("USB Virtuoso SE exposed dormant sleep timer support")
	}
}

func TestWorkspaceSnapshotsFailClosedForIncompleteVirtuosoSEUSBProfiles(t *testing.T) {
	d := &Device{Serial: "virtuoso-se-wu", DeviceProfile: virtuosoSEWorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": nil}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("incomplete profile state was exposed")
	}
	d.UserProfiles = map[string]*DeviceProfile{"Default": {Active: true}}
	d.MuteIndicators = map[int]string{0: "Disabled"}
	if _, ok := d.HeadsetSnapshot(); ok {
		t.Fatal("incomplete mute indicator was exposed")
	}
}
