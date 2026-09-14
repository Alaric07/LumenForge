package hydro

import (
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/temperatures"
	"testing"
)

func TestCoolingSnapshotPreservesHydroPumpModesAndLogicalFan(t *testing.T) {
	original := coolingTemperatureProfiles
	coolingTemperatureProfiles = func() map[string]temperatures.TemperatureProfileData {
		return map[string]temperatures.TemperatureProfileData{}
	}
	t.Cleanup(func() { coolingTemperatureProfiles = original })
	d := &Device{Serial: "hydro-aio", Devices: map[int]*Devices{
		1: {ChannelId: 1, Name: "Fans", Label: "Radiator fans", Rpm: 1200, Profile: "Balanced", HasSpeed: true},
		0: {ChannelId: 0, Name: "Pump", Label: "Pump", Rpm: 2100, Temperature: 30.0, TemperatureString: "30.0°C", Profile: "Quiet", HasSpeed: true, HasTemps: true, ContainsPump: true, PumpModes: map[byte]string{66: "Performance", 40: "Quiet"}},
	}}
	snapshot, ok := d.CoolingSnapshot()
	if !ok || len(snapshot.Channels) != 2 || snapshot.Channels[0].Name != "Pump" || snapshot.Channels[1].Name != "Fans" || len(snapshot.Channels[0].PumpModeOptions) != 2 || snapshot.Channels[0].PumpModeOptions[0].ID != "Quiet" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}

func TestHydroDoesNotExposeIncompleteDeviceProfileContract(t *testing.T) {
	if _, ok := interface{}(&Device{}).(interface {
		DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool)
	}); ok {
		t.Fatal("Hydro unexpectedly exposes Device Profiles")
	}
}
