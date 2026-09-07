package k70core

import "testing"

func TestDeviceProfileSnapshotK70CoreRequiresExactlyOneActiveProfile(t *testing.T) {
	if snapshot, ok := (&Device{UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {}}}).DeviceProfileSnapshot(); !ok || snapshot.ActiveProfile != "Default" {
		t.Fatalf("snapshot=%#v ok=%t", snapshot, ok)
	}
	for _, profiles := range []map[string]*DeviceProfile{{"Default": {}}, {"Default": {Active: true}, "Gaming": {Active: true}}} {
		if _, ok := (&Device{UserProfiles: profiles}).DeviceProfileSnapshot(); ok {
			t.Fatal("accepted invalid active profile state")
		}
	}
}
