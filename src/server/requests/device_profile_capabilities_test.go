package requests

import (
	"LumenForge/src/deviceprofilepresentation"
	"testing"
)

type deviceProfileCapabilityFixture struct {
	serial   string
	snapshot deviceprofilepresentation.Snapshot
}

func (f deviceProfileCapabilityFixture) DeviceProfileDeviceID() string { return f.serial }
func (f deviceProfileCapabilityFixture) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	return f.snapshot, true
}

func TestDeviceProfileActionSupportedFailsClosed(t *testing.T) {
	fixture := deviceProfileCapabilityFixture{serial: "profile-capabilities", snapshot: deviceprofilepresentation.Snapshot{Supported: true, CanSwitch: true}}
	registry := map[string]interface{}{fixture.serial: fixture}
	previousLookup := getDeviceProfileDevice
	getDeviceProfileDevice = func(serial string) interface{} { return registry[serial] }
	t.Cleanup(func() { getDeviceProfileDevice = previousLookup })

	if !deviceProfileActionSupported(fixture.serial, deviceProfileActionSwitch) {
		t.Fatal("switch action was rejected")
	}
	if deviceProfileActionSupported(fixture.serial, deviceProfileActionSave) {
		t.Fatal("save action was accepted")
	}
	if deviceProfileActionSupported(fixture.serial, deviceProfileActionDelete) {
		t.Fatal("delete action was accepted")
	}
}
