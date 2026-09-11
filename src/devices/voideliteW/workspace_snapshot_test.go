package voideliteW

import "testing"

func voidEliteWorkspaceProfile() *DeviceProfile {
	p := &DeviceProfile{Equalizers: map[int]Equalizer{}, SideTone: 1, SideToneValue: 50}
	for i, label := range []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"} {
		p.Equalizers[i+1] = Equalizer{Name: label}
	}
	return p
}

func TestWorkspaceSnapshotsExposeOnlyCompleteVOIDEliteCapabilities(t *testing.T) {
	d := &Device{Serial: "void-elite", DeviceProfile: voidEliteWorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	profiles, ok := d.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete || profiles.ActiveProfile != "Default" {
		t.Fatalf("profiles=%#v", profiles)
	}
	snapshot, ok := d.HeadsetSnapshot()
	if !ok || len(snapshot.Equalizer) != 10 || snapshot.Equalizer[0].Label != "32" || snapshot.Equalizer[9].Label != "16K" || snapshot.Sidetone == nil || snapshot.Sidetone.ValueRange.Minimum != 1 || snapshot.Sidetone.ValueRange.Maximum != 100 || snapshot.Sidetone.ValueRange.Step != 1 || snapshot.MuteIndicator != nil || snapshot.NoiseCancellation != nil || len(snapshot.Assignments) != 0 || len(snapshot.Wheels) != 0 {
		t.Fatalf("snapshot=%#v", snapshot)
	}
	if got := snapshot.Sidetone.Options; len(got) != 2 || got[0].Value != 0 || got[0].Label != "Disabled" || got[1].Value != 1 || got[1].Label != "Enabled" {
		t.Fatalf("sidetone options=%#v", got)
	}
}

func TestWorkspaceSnapshotsFailClosedForIncompleteVOIDEliteState(t *testing.T) {
	d := &Device{Serial: "void-elite", DeviceProfile: voidEliteWorkspaceProfile(), UserProfiles: map[string]*DeviceProfile{"Default": nil}}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("incomplete profiles were exposed")
	}
	d.UserProfiles = map[string]*DeviceProfile{"Default": {Active: true}}
	d.DeviceProfile.Equalizers = nil
	if _, ok := d.HeadsetSnapshot(); ok {
		t.Fatal("incomplete equalizers were exposed")
	}
	d.DeviceProfile = voidEliteWorkspaceProfile()
	d.DeviceProfile.SideToneValue = 0
	if _, ok := d.HeadsetSnapshot(); ok {
		t.Fatal("invalid sidetone range was exposed")
	}
}
