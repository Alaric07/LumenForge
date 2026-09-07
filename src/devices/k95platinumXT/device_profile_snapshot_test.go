package k95platinumXT

import "testing"

func TestDeviceProfileSnapshotUsesActiveK95PlatinumXTUserProfileAndFailsClosed(t *testing.T) {
	d := &Device{UserProfiles: map[string]*DeviceProfile{"Gaming": {}, "Default": {Active: true}}}
	s, ok := d.DeviceProfileSnapshot()
	if !ok || s.ActiveProfile != "Default" || len(s.Profiles) != 2 || s.Profiles[0] != "Default" {
		t.Fatalf("snapshot = %#v, ok=%t", s, ok)
	}
	if s, ok := (&Device{UserProfiles: map[string]*DeviceProfile{"Default": {}}}).DeviceProfileSnapshot(); ok || s.Supported {
		t.Fatalf("inactive snapshot = %#v, ok=%t", s, ok)
	}
	if s, ok := (&Device{UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {Active: true}}}).DeviceProfileSnapshot(); ok || s.Supported {
		t.Fatalf("multiply active snapshot = %#v, ok=%t", s, ok)
	}
}
