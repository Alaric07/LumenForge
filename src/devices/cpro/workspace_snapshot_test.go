package cpro

import (
	"LumenForge/src/temperatures"
	"reflect"
	"testing"
)

func TestCommanderProWorkspaceSnapshots(t *testing.T) {
	original := coolingTemperatureProfiles
	coolingTemperatureProfiles = func() map[string]temperatures.TemperatureProfileData {
		return map[string]temperatures.TemperatureProfileData{"quiet": {}, "hidden": {Hidden: true}, "balanced": {}}
	}
	t.Cleanup(func() { coolingTemperatureProfiles = original })
	device := &Device{Serial: "cpro", Devices: map[int]*Devices{4: {ChannelId: 4, Name: "Probe 2", Label: "Case", Temperature: 28.5, TemperatureString: "28.5°C", IsTemperatureProbe: true}, 1: {ChannelId: 1, Name: "Fan 2", Label: "Pump", Rpm: 2100, Profile: "quiet", HasSpeed: true, ContainsPump: true}, 0: {ChannelId: 0, Name: "Fan 1", Label: "Front", Rpm: 980, Profile: "balanced", HasSpeed: true}, 3: {ChannelId: 3, Name: "Probe 1", Label: "Coolant", Temperature: 31.2, TemperatureString: "31.2°C", IsTemperatureProbe: true}}, UserProfiles: map[string]*DeviceProfile{"studio": {}, "default": {Active: true}, "nil": nil}, RailVoltages: map[int]*RailVoltage{2: {Name: "+3.3V", Value: 3.31}, 0: {Name: "+12V", Value: 12.08}, 1: {Name: "+5V", Value: 5.02}}}
	cooling, ok := device.CoolingSnapshot()
	if !ok || len(cooling.Channels) != 2 || len(cooling.TemperatureProbes) != 2 || cooling.Channels[1].RPM != 2100 || !cooling.Channels[1].ContainsPump {
		t.Fatalf("cooling = %#v, ok=%t", cooling, ok)
	}
	if got := []int{cooling.Channels[0].ID, cooling.Channels[1].ID}; !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("channels = %#v", got)
	}
	if got := []int{cooling.TemperatureProbes[0].ID, cooling.TemperatureProbes[1].ID}; !reflect.DeepEqual(got, []int{3, 4}) {
		t.Fatalf("probes = %#v", got)
	}
	if got := []string{cooling.ProfileOptions[0].ID, cooling.ProfileOptions[1].ID}; !reflect.DeepEqual(got, []string{"balanced", "quiet"}) {
		t.Fatalf("profiles = %#v", got)
	}
	profiles, ok := device.DeviceProfileSnapshot()
	if !ok || profiles.ActiveProfile != "default" || !reflect.DeepEqual(profiles.Profiles, []string{"default", "studio"}) {
		t.Fatalf("profiles = %#v, ok=%t", profiles, ok)
	}
	telemetry, ok := device.TelemetrySnapshot()
	if !ok || !reflect.DeepEqual(telemetry.Rows[0].Label, "+12V") || telemetry.Rows[0].Value != "12.08 V" || telemetry.Rows[2].Value != "3.31 V" {
		t.Fatalf("telemetry = %#v, ok=%t", telemetry, ok)
	}
}

func TestCommanderProProfileSnapshotFailsClosedWithoutActiveProfile(t *testing.T) {
	snapshot, ok := (&Device{UserProfiles: map[string]*DeviceProfile{"default": {}}}).DeviceProfileSnapshot()
	if ok || !snapshot.Supported {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}
