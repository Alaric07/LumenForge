package elite

import (
	"LumenForge/src/temperatures"
	"testing"
)

func TestCoolingSnapshotOrdersChannelsAndPumpModes(t *testing.T) {
	original := coolingTemperatureProfiles
	coolingTemperatureProfiles = func() map[string]temperatures.TemperatureProfileData {
		return map[string]temperatures.TemperatureProfileData{}
	}
	t.Cleanup(func() { coolingTemperatureProfiles = original })
	d := &Device{Serial: "elite-aio", Devices: map[int]*Devices{
		3: {ChannelId: 3, Name: "Fan 3", Label: "Radiator right", Rpm: 1150, Profile: "Balanced", HasSpeed: true},
		0: {ChannelId: 0, Name: "Pump", Label: "Coolant", Rpm: 2450, Temperature: 31.5, TemperatureString: "31.5°C", Profile: "Normal", HasSpeed: true, HasTemps: true, ContainsPump: true, PumpModes: map[byte]string{2: "Performance", 0: "Quiet", 1: "Normal"}},
		1: {ChannelId: 1, Name: "Fan 1", Label: "Radiator left", Rpm: 1050, Profile: "Balanced", HasSpeed: true},
	}}
	snapshot, ok := d.CoolingSnapshot()
	if !ok || len(snapshot.Channels) != 3 || snapshot.Channels[0].ID != 0 || !snapshot.Channels[0].ContainsPump || snapshot.Channels[0].Temperature != "31.5°C" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	for index, mode := range []string{"Quiet", "Normal", "Performance"} {
		if snapshot.Channels[0].PumpModeOptions[index].ID != mode {
			t.Fatalf("modes = %#v", snapshot.Channels[0].PumpModeOptions)
		}
	}
}

func TestDeviceProfileSnapshotFailsClosedWithoutCompleteContract(t *testing.T) {
	if _, ok := (&Device{Serial: "elite", DeviceProfile: &DeviceProfile{}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}}).DeviceProfileSnapshot(); !ok {
		t.Fatal("ELITE has the complete existing profile contract")
	}
	if _, ok := (&Device{Serial: "elite"}).DeviceProfileSnapshot(); ok {
		t.Fatal("nil profile unexpectedly exposed")
	}
}
