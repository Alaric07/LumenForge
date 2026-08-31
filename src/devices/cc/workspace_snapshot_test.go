package cc

import (
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/temperatures"
	"reflect"
	"testing"
)

func TestCommanderCoreDeviceProfileSnapshotUsesActiveUserProfile(t *testing.T) {
	device := &Device{Serial: "cc-profile", UserProfiles: map[string]*DeviceProfile{
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
	if device.DeviceProfileDeviceID() != "cc-profile" {
		t.Fatalf("profile device ID = %q", device.DeviceProfileDeviceID())
	}
}

func TestCommanderCoreDeviceProfileSnapshotFailsClosedWithoutActiveProfile(t *testing.T) {
	device := &Device{UserProfiles: map[string]*DeviceProfile{"default": {Active: false}, "missing": nil}}

	snapshot, ok := device.DeviceProfileSnapshot()
	if ok || !snapshot.Supported || snapshot.ActiveProfile != "" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}

func TestCommanderCoreCoolingSnapshotPreservesPumpAndTelemetry(t *testing.T) {
	originalProfiles := coolingTemperatureProfiles
	coolingTemperatureProfiles = func() map[string]temperatures.TemperatureProfileData {
		return map[string]temperatures.TemperatureProfileData{"quiet": {}, "hidden": {Hidden: true}}
	}
	t.Cleanup(func() { coolingTemperatureProfiles = originalProfiles })

	device := &Device{Serial: "cc-cooling", Devices: map[int]*Devices{
		0: {ChannelId: 0, Name: "H150i", Label: "Pump", Rpm: 2440, TemperatureString: "31.2°C", Profile: "quiet", HasSpeed: true, HasTemps: true, ContainsPump: true},
		1: {ChannelId: 1, Name: "Fan 1", Label: "Front", Rpm: 912, Profile: "quiet", HasSpeed: true},
		7: {ChannelId: 7, Name: "Temperature Probe 1", Label: "Coolant", TemperatureString: "29.4°C", IsTemperatureProbe: true, HasTemps: true},
		8: {ChannelId: 8, Name: "RGB port", Label: "ignored"},
	}}

	snapshot, ok := device.CoolingSnapshot()
	if !ok || len(snapshot.Channels) != 2 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	pump, fan := snapshot.Channels[0], snapshot.Channels[1]
	if pump.ID != 0 || !pump.ContainsPump || pump.Name != "H150i" || pump.RPM != 2440 || pump.Temperature != "31.2°C" || pump.SelectedProfile != "quiet" {
		t.Fatalf("pump = %#v", pump)
	}
	if fan.ID != 1 || fan.ContainsPump || fan.Name != "Fan 1" || fan.RPM != 912 || fan.Temperature != "" || fan.SelectedProfile != "quiet" {
		t.Fatalf("fan = %#v", fan)
	}
	if len(snapshot.TemperatureProbes) != 1 || snapshot.TemperatureProbes[0].ID != 7 || snapshot.TemperatureProbes[0].Temperature != "29.4°C" {
		t.Fatalf("probes = %#v", snapshot.TemperatureProbes)
	}
	if len(snapshot.ProfileOptions) != 1 || snapshot.ProfileOptions[0].ID != "quiet" {
		t.Fatalf("profile options = %#v", snapshot.ProfileOptions)
	}
}

func TestCommanderCoreDoesNotExposeCanonicalLightingSnapshotProvider(t *testing.T) {
	device := &Device{}
	if _, ok := interface{}(device).(interface {
		LightingSnapshot() (lightingpresentation.Snapshot, bool)
	}); ok {
		t.Fatal("Commander CORE unexpectedly exposes canonical lighting")
	}
}
