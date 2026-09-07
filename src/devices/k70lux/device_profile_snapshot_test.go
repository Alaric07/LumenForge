package k70lux

import "testing"

func TestDeviceProfileSnapshotK70LUXRequiresExactlyOneActiveProfile(t *testing.T) {
	for _, profiles := range []map[string]*DeviceProfile{{"Default": {Active: true}, "Gaming": {}}, {"Default": {}}, {"Default": {Active: true}, "Gaming": {Active: true}}} {
		s, ok := (&Device{UserProfiles: profiles}).DeviceProfileSnapshot()
		want := len(profiles) == 2 && profiles["Default"].Active && !profiles["Gaming"].Active
		if ok != want || (ok && s.ActiveProfile != "Default") {
			t.Fatalf("snapshot=%#v ok=%t", s, ok)
		}
	}
}
