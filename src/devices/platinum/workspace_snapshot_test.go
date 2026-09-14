package platinum

import (
	"LumenForge/src/temperatures"
	"testing"
)

func TestCoolingSnapshotKeepsModelDependentFansAndPumpClassification(t *testing.T) {
	original := coolingTemperatureProfiles
	coolingTemperatureProfiles = func() map[string]temperatures.TemperatureProfileData {
		return map[string]temperatures.TemperatureProfileData{}
	}
	t.Cleanup(func() { coolingTemperatureProfiles = original })
	d := &Device{Serial: "platinum-aio", Devices: map[int]*Devices{
		0: {ChannelId: 0, Name: "Pump", Label: "Coolant", Rpm: 2400, Temperature: 32.4, TemperatureString: "32.4°C", Profile: "Balanced", HasSpeed: true, HasTemps: true, ContainsPump: true},
		1: {ChannelId: 1, Name: "Fan 1", Rpm: 1000, Profile: "Balanced", HasSpeed: true},
		2: {ChannelId: 2, Name: "Fan 2", Rpm: 1050, Profile: "Balanced", HasSpeed: true},
		3: {ChannelId: 3, Name: "Fan 3", Rpm: 1100, Profile: "Balanced", HasSpeed: true},
	}}
	snapshot, ok := d.CoolingSnapshot()
	if !ok || len(snapshot.Channels) != 4 || !snapshot.Channels[0].ContainsPump || snapshot.Channels[3].Name != "Fan 3" || len(snapshot.Channels[0].PumpModeOptions) != 0 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}

func TestDeviceProfileSnapshotFailsClosedWithoutProfile(t *testing.T) {
	if _, ok := (&Device{Serial: "platinum"}).DeviceProfileSnapshot(); ok {
		t.Fatal("nil profile unexpectedly exposed")
	}
}
