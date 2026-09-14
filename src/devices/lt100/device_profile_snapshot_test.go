package lt100

import (
	"reflect"
	"testing"

	"LumenForge/src/deviceprofilepresentation"
)

func TestLT100DeviceProfileSnapshotUsesExistingUserProfiles(t *testing.T) {
	device := &Device{Serial: "lt100-profile", UserProfiles: map[string]*DeviceProfile{
		"studio":  {Active: false},
		"default": {Active: true},
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

func TestLT100DeviceProfileSnapshotFailsClosedForInvalidProfileState(t *testing.T) {
	for name, device := range map[string]*Device{
		"no active profile":        {UserProfiles: map[string]*DeviceProfile{"default": {Active: false}}},
		"multiple active profiles": {UserProfiles: map[string]*DeviceProfile{"default": {Active: true}, "studio": {Active: true}}},
		"nil profile":              {UserProfiles: map[string]*DeviceProfile{"default": nil}},
		"empty profile name":       {UserProfiles: map[string]*DeviceProfile{"": {Active: true}}},
	} {
		t.Run(name, func(t *testing.T) {
			snapshot, ok := device.DeviceProfileSnapshot()
			if ok || snapshot.Supported || snapshot.ActiveProfile != "" || len(snapshot.Profiles) != 0 {
				t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
			}
		})
	}
}

func TestLT100DeviceProfileSnapshotFailsClosedForNilDevice(t *testing.T) {
	var device *Device

	if got := device.DeviceProfileDeviceID(); got != "" {
		t.Fatalf("device profile ID = %q, want empty", got)
	}
	if snapshot, ok := device.DeviceProfileSnapshot(); ok || snapshot.Supported || snapshot.Scope != "" || len(snapshot.Profiles) != 0 || snapshot.ActiveProfile != "" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}

func TestLT100DeviceProfileSnapshotDoesNotMutateProfileState(t *testing.T) {
	defaultProfile := &DeviceProfile{Active: true}
	studioProfile := &DeviceProfile{Active: false}
	profiles := map[string]*DeviceProfile{"default": defaultProfile, "studio": studioProfile}
	device := &Device{UserProfiles: profiles}

	_, _ = device.DeviceProfileSnapshot()

	if len(device.UserProfiles) != 2 || device.UserProfiles["default"] != defaultProfile || device.UserProfiles["studio"] != studioProfile {
		t.Fatalf("user profiles changed: %#v", device.UserProfiles)
	}
	if !defaultProfile.Active || studioProfile.Active {
		t.Fatalf("profile active state changed: default=%t studio=%t", defaultProfile.Active, studioProfile.Active)
	}
}
