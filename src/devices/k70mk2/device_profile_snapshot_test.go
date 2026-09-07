package k70mk2

import "testing"

func TestDeviceProfileSnapshotK70MK2RequiresExactlyOneActiveProfile(t *testing.T) {
	valid := (&Device{UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {}}})
	if s, ok := valid.DeviceProfileSnapshot(); !ok || s.ActiveProfile != "Default" {
		t.Fatalf("snapshot=%#v ok=%t", s, ok)
	}
	for _, p := range []map[string]*DeviceProfile{{"Default": {}}, {"Default": {Active: true}, "Gaming": {Active: true}}} {
		if _, ok := (&Device{UserProfiles: p}).DeviceProfileSnapshot(); ok {
			t.Fatal("accepted invalid active profiles")
		}
	}
}
