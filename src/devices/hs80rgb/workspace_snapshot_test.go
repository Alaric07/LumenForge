package hs80rgb

import "testing"

func TestWorkspaceSnapshotFailsClosedWithoutCompleteProfileAndEqualizer(t *testing.T) {
	d := &Device{Serial: "hs80", UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("profile snapshot accepted missing active profile state")
	}
	if _, ok := d.HeadsetSnapshot(); ok {
		t.Fatal("headset snapshot accepted missing equalizer")
	}
}

func TestWorkspaceSnapshotUsesSourceEqualizerOrder(t *testing.T) {
	labels := []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"}
	p := &DeviceProfile{Equalizers: map[int]Equalizer{}}
	for i, label := range labels {
		p.Equalizers[i+1] = Equalizer{Name: label}
	}
	d := &Device{Serial: "hs80", DeviceProfile: p}
	s, ok := d.HeadsetSnapshot()
	if !ok {
		t.Fatal("headset snapshot unavailable")
	}
	for i, label := range labels {
		if s.Equalizer[i].Label != label {
			t.Fatalf("band %d = %q", i, s.Equalizer[i].Label)
		}
	}
}
