package k95platinum

import "testing"

func TestK95DeviceProfileSnapshotUsesExistingUserProfiles(t *testing.T) {
	device := &Device{Serial: "k95", UserProfiles: map[string]*DeviceProfile{"studio": {Active: false}, "default": {Active: true}}}
	snapshot, ok := device.DeviceProfileSnapshot()
	if !ok || !snapshot.Supported || snapshot.ActiveProfile != "default" || len(snapshot.Profiles) != 2 || snapshot.Profiles[0] != "default" || snapshot.Profiles[1] != "studio" { t.Errorf("snapshot = %#v, ok=%t", snapshot, ok) }
}
