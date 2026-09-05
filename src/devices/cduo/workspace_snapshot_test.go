package cduo

import (
	"LumenForge/src/temperatures"
	"reflect"
	"testing"
)

func TestCommanderDuoCoolingSnapshotUsesExistingChannelState(t *testing.T) {
	originalProfiles := coolingTemperatureProfiles
	coolingTemperatureProfiles = func() map[string]temperatures.TemperatureProfileData {
		return map[string]temperatures.TemperatureProfileData{"quiet": {}, "hidden": {Hidden: true}, "balanced": {}}
	}
	t.Cleanup(func() { coolingTemperatureProfiles = originalProfiles })

	device := &Device{Serial: "duo-cooling", Devices: map[int]*Devices{
		7: {ChannelId: 7, Name: "Temperature Probe 2", Label: "Case", Temperature: 29.4, TemperatureString: "29.4°C", IsTemperatureProbe: true, HasTemps: true},
		3: {ChannelId: 3, Name: "Fan Channel 4", Label: "Pump", Rpm: 1820, Profile: "balanced", HasSpeed: true, ContainsPump: true},
		1: {ChannelId: 1, Name: "Fan Channel 2", Label: "Front", Rpm: 1040, Profile: "quiet", HasSpeed: true},
		6: {ChannelId: 6, Name: "Temperature Probe 1", Label: "Coolant", Temperature: 31.2, TemperatureString: "31.2°C", IsTemperatureProbe: true, HasTemps: true},
	}}

	snapshot, ok := device.CoolingSnapshot()
	if !ok || len(snapshot.Channels) != 2 || len(snapshot.TemperatureProbes) != 2 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	if got := []int{snapshot.Channels[0].ID, snapshot.Channels[1].ID}; !reflect.DeepEqual(got, []int{1, 3}) {
		t.Fatalf("channel order = %#v", got)
	}
	if channel := snapshot.Channels[1]; channel.Name != "Fan Channel 4" || channel.Label != "Pump" || channel.RPM != 1820 || channel.SelectedProfile != "balanced" || !channel.ContainsPump {
		t.Fatalf("pump channel = %#v", channel)
	}
	if got := []int{snapshot.TemperatureProbes[0].ID, snapshot.TemperatureProbes[1].ID}; !reflect.DeepEqual(got, []int{6, 7}) {
		t.Fatalf("probe order = %#v", got)
	}
	if probe := snapshot.TemperatureProbes[0]; probe.Label != "Coolant" || probe.Temperature != "31.2°C" || probe.Celsius == nil || *probe.Celsius != 31.2 {
		t.Fatalf("probe = %#v", probe)
	}
	if got := []string{snapshot.ProfileOptions[0].ID, snapshot.ProfileOptions[1].ID}; !reflect.DeepEqual(got, []string{"balanced", "quiet"}) {
		t.Fatalf("profile options = %#v", got)
	}
}

func TestCommanderDuoDeviceProfileSnapshotUsesActiveUserProfile(t *testing.T) {
	device := &Device{Serial: "duo-profile", UserProfiles: map[string]*DeviceProfile{
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
}

func TestCommanderDuoDeviceProfileSnapshotFailsClosedWithoutActiveProfile(t *testing.T) {
	device := &Device{UserProfiles: map[string]*DeviceProfile{"default": {Active: false}, "missing": nil}}

	snapshot, ok := device.DeviceProfileSnapshot()
	if ok || !snapshot.Supported || snapshot.ActiveProfile != "" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}
