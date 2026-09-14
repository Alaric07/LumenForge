package mm700

import (
	"reflect"
	"testing"

	"LumenForge/src/deviceprofilepresentation"
)

func TestMM700DeviceProfileSnapshotUsesExistingUserProfiles(t *testing.T) {
	device := &Device{Serial: "mm700-profile", UserProfiles: map[string]*DeviceProfile{
		"studio":  {Active: false},
		"default": {Active: true},
		"missing": nil,
	}}

	snapshot, ok := device.DeviceProfileSnapshot()
	if !ok || !snapshot.Supported || snapshot.Scope != deviceprofilepresentation.ScopeLighting || snapshot.DefaultProfileDisplayLabel != deviceprofilepresentation.WorkingConfigurationLabel {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	if snapshot.ActiveProfile != "default" {
		t.Fatalf("active profile = %q, want %q", snapshot.ActiveProfile, "default")
	}
	if want := []string{"default", "studio"}; !reflect.DeepEqual(snapshot.Profiles, want) {
		t.Fatalf("profiles = %#v, want %#v", snapshot.Profiles, want)
	}
	if !snapshot.CanSwitch || !snapshot.CanSave || !snapshot.CanDelete {
		t.Fatalf("mutation capabilities = switch:%t save:%t delete:%t", snapshot.CanSwitch, snapshot.CanSave, snapshot.CanDelete)
	}
}

func TestMM700DeviceProfileSnapshotDoesNotInferActiveProfile(t *testing.T) {
	device := &Device{UserProfiles: map[string]*DeviceProfile{
		"default": {Active: false},
		"studio":  {Active: false},
	}}

	snapshot, ok := device.DeviceProfileSnapshot()
	if ok || !snapshot.Supported || snapshot.ActiveProfile != "" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}

func TestMM700DeviceProfileSnapshotFailsClosedForNilDevice(t *testing.T) {
	var device *Device

	if got := device.DeviceProfileDeviceID(); got != "" {
		t.Fatalf("device profile ID = %q, want empty", got)
	}
	if snapshot, ok := device.DeviceProfileSnapshot(); ok || snapshot.Supported || snapshot.Scope != "" || len(snapshot.Profiles) != 0 || snapshot.ActiveProfile != "" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}

func TestMM700DeviceProfileSnapshotDoesNotMutateProfileState(t *testing.T) {
	defaultProfile := &DeviceProfile{Active: true}
	studioProfile := &DeviceProfile{Active: false}
	profiles := map[string]*DeviceProfile{
		"default": defaultProfile,
		"studio":  studioProfile,
		"missing": nil,
	}
	device := &Device{UserProfiles: profiles}

	_, _ = device.DeviceProfileSnapshot()

	if len(device.UserProfiles) != 3 || device.UserProfiles["default"] != defaultProfile || device.UserProfiles["studio"] != studioProfile || device.UserProfiles["missing"] != nil {
		t.Fatalf("user profiles changed: %#v", device.UserProfiles)
	}
	if !defaultProfile.Active || studioProfile.Active {
		t.Fatalf("profile active state changed: default=%t studio=%t", defaultProfile.Active, studioProfile.Active)
	}
}
