package scimitarrgbelite

import (
	"reflect"
	"testing"
)

func TestScimitarEliteDeviceProfileSnapshotUsesExistingUserProfiles(t *testing.T) {
	device := &Device{UserProfiles: map[string]*DeviceProfile{
		"studio":  {Active: false},
		"default": {Active: true},
		"missing": nil,
	}}

	snapshot, ok := device.DeviceProfileSnapshot()
	if !ok {
		t.Fatal("snapshot was unusable")
	}
	if !snapshot.Supported {
		t.Fatal("snapshot was not supported")
	}
	if snapshot.ActiveProfile != "default" {
		t.Fatalf("active profile = %q, want %q", snapshot.ActiveProfile, "default")
	}
	if want := []string{"default", "studio"}; !reflect.DeepEqual(snapshot.Profiles, want) {
		t.Fatalf("profiles = %#v, want %#v", snapshot.Profiles, want)
	}
}

func TestScimitarEliteDeviceProfileSnapshotDoesNotInferActiveProfile(t *testing.T) {
	device := &Device{UserProfiles: map[string]*DeviceProfile{
		"default": {Active: false},
		"studio":  {Active: false},
	}}

	snapshot, ok := device.DeviceProfileSnapshot()
	if ok {
		t.Fatalf("snapshot was usable without an active profile: %#v", snapshot)
	}
	if !snapshot.Supported || snapshot.ActiveProfile != "" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestScimitarEliteDeviceProfileSnapshotFailsClosedForNilDevice(t *testing.T) {
	var device *Device

	if got := device.DeviceProfileDeviceID(); got != "" {
		t.Fatalf("device profile ID = %q, want empty", got)
	}
	if snapshot, ok := device.DeviceProfileSnapshot(); ok || snapshot.Supported || len(snapshot.Profiles) != 0 || snapshot.ActiveProfile != "" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}

func TestScimitarEliteDeviceProfileSnapshotDoesNotMutateProfileState(t *testing.T) {
	defaultProfile := &DeviceProfile{Active: true}
	studioProfile := &DeviceProfile{Active: false}
	profiles := map[string]*DeviceProfile{
		"default": defaultProfile,
		"studio":  studioProfile,
		"missing": nil,
	}
	device := &Device{UserProfiles: profiles}

	_, _ = device.DeviceProfileSnapshot()

	if device.UserProfiles["default"] != defaultProfile || device.UserProfiles["studio"] != studioProfile || device.UserProfiles["missing"] != nil {
		t.Fatalf("user profiles changed: %#v", device.UserProfiles)
	}
	if !defaultProfile.Active || studioProfile.Active {
		t.Fatalf("profile active state changed: default=%t studio=%t", defaultProfile.Active, studioProfile.Active)
	}
}
