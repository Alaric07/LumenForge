package voidV2W

import "testing"

func voidV2WorkspaceProfile() *DeviceProfile {
	p := &DeviceProfile{Equalizers: map[int]Equalizer{}, SleepMode: 15, SideTone: 1, SideToneValue: 50}
	for i, label := range []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"} {
		p.Equalizers[i+1] = Equalizer{Name: label}
	}
	return p
}
func voidV2SleepModes() map[int]string {
	return map[int]string{1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}
}

func TestWorkspaceSnapshotsExposeOnlyCompleteVOIDV2Capabilities(t *testing.T) {
	d := &Device{Serial: "void-v2", DeviceProfile: voidV2WorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {}}, SleepModes: voidV2SleepModes(), SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete {
		t.Fatalf("profiles=%#v", profiles)
	}
	sleep, ok := d.SleepTimerSnapshot()
	if !ok || len(sleep.Options) != 6 {
		t.Fatalf("sleep=%#v", sleep)
	}
	for i, value := range []int{1, 5, 10, 15, 30, 60} {
		if sleep.Options[i].Value != value {
			t.Fatalf("sleep=%#v", sleep.Options)
		}
	}
	snapshot, ok := d.HeadsetSnapshot()
	if !ok || len(snapshot.Equalizer) != 10 || snapshot.Equalizer[0].Label != "32" || snapshot.Equalizer[9].Label != "16K" || snapshot.Sidetone == nil || snapshot.Sidetone.ValueRange.Minimum != 1 || snapshot.Sidetone.ValueRange.Maximum != 100 || snapshot.Sidetone.ValueRange.Step != 1 || snapshot.MuteIndicator != nil || snapshot.NoiseCancellation != nil || len(snapshot.Assignments) != 0 || len(snapshot.Wheels) != 0 {
		t.Fatalf("snapshot=%#v", snapshot)
	}
	if got := snapshot.Sidetone.Options; len(got) != 2 || got[0].Value != 0 || got[0].Label != "Disabled" || got[1].Value != 1 || got[1].Label != "Enabled" {
		t.Fatalf("sidetone options=%#v", got)
	}
}

func TestWorkspaceSnapshotsFailClosedForIncompleteVOIDV2State(t *testing.T) {
	d := &Device{Serial: "void-v2", DeviceProfile: voidV2WorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": nil}, SleepModes: voidV2SleepModes(), SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("incomplete profiles were exposed")
	}
	d.UserProfiles = map[string]*DeviceProfile{"Default": {Active: true}}
	d.SleepModes = map[int]string{1: "1 minute"}
	if _, ok := d.SleepTimerSnapshot(); ok {
		t.Fatal("malformed sleep options were exposed")
	}
	d.SleepModes = voidV2SleepModes()
	d.SideToneModes = map[int]string{0: "Disabled"}
	if _, ok := d.HeadsetSnapshot(); ok {
		t.Fatal("malformed sidetone options were exposed")
	}
	d.SideToneModes = map[int]string{0: "Disabled", 1: "Enabled"}
	d.DeviceProfile.Equalizers = nil
	if _, ok := d.HeadsetSnapshot(); ok {
		t.Fatal("incomplete equalizers were exposed")
	}
}
