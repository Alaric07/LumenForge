package ccxt

import (
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/temperatures"
	"reflect"
	"testing"
)

func TestCCXTDeviceProfileSnapshotUsesActiveUserProfile(t *testing.T) {
	device := &Device{Serial: "ccxt-profile", UserProfiles: map[string]*DeviceProfile{
		"studio":  {Active: false},
		"default": {Active: true},
		"missing": nil,
	}}

	snapshot, ok := device.DeviceProfileSnapshot()
	if !ok || !snapshot.Supported || snapshot.ActiveProfile != "default" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	if want := []string{"default", "studio"}; !reflect.DeepEqual(snapshot.Profiles, want) {
		t.Fatalf("profiles = %#v, want %#v", snapshot.Profiles, want)
	}
	if snapshot.DefaultProfileDisplayLabel != "" {
		t.Fatalf("default display label = %q", snapshot.DefaultProfileDisplayLabel)
	}
}

func TestCCXTDeviceProfileSnapshotFailsClosedWithoutActiveProfile(t *testing.T) {
	device := &Device{UserProfiles: map[string]*DeviceProfile{"default": {Active: false}, "missing": nil}}

	snapshot, ok := device.DeviceProfileSnapshot()
	if ok || !snapshot.Supported || snapshot.ActiveProfile != "" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}

func TestCCXTCoolingSnapshotUsesExistingChannelState(t *testing.T) {
	originalProfiles := coolingTemperatureProfiles
	coolingTemperatureProfiles = func() map[string]temperatures.TemperatureProfileData {
		return map[string]temperatures.TemperatureProfileData{"quiet": {}, "hidden": {Hidden: true}}
	}
	t.Cleanup(func() { coolingTemperatureProfiles = originalProfiles })

	device := &Device{Serial: "ccxt-cooling", Devices: map[int]*Devices{
		2: {ChannelId: 2, Name: "Fan 2", Label: "Rear fan", Rpm: 912, Profile: "quiet", HasSpeed: true},
		1: {ChannelId: 1, Name: "Probe 1", Label: "Coolant", TemperatureString: "31.2°C", IsTemperatureProbe: true, HasTemps: true},
		3: {ChannelId: 3, Name: "RGB port", Label: "ignored"},
	}}

	snapshot, ok := device.CoolingSnapshot()
	if !ok || len(snapshot.Channels) != 1 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	channel := snapshot.Channels[0]
	if channel.ID != 2 || channel.Name != "Fan 2" || channel.Label != "Rear fan" || channel.RPM != 912 || channel.SelectedProfile != "quiet" {
		t.Fatalf("channel = %#v", channel)
	}
	if len(snapshot.TemperatureProbes) != 1 || snapshot.TemperatureProbes[0].ID != 1 || snapshot.TemperatureProbes[0].Temperature != "31.2°C" {
		t.Fatalf("probes = %#v", snapshot.TemperatureProbes)
	}
	if want := "quiet"; len(snapshot.ProfileOptions) != 1 || snapshot.ProfileOptions[0].ID != want {
		t.Fatalf("profile options = %#v", snapshot.ProfileOptions)
	}
}

func TestCCXTDoesNotExposeCanonicalLightingSnapshot(t *testing.T) {
	device := &Device{}
	if _, ok := interface{}(device).(interface {
		LightingSnapshot() (lightingpresentation.Snapshot, bool)
	}); ok {
		t.Fatal("CCXT unexpectedly exposes canonical lighting")
	}
}
