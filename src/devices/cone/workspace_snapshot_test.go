package cone

import (
	"LumenForge/src/temperatures"
	"testing"
)

func TestCoolingSnapshotPreservesCorsairOneSourceTopology(t *testing.T) {
	original := coolingTemperatureProfiles
	coolingTemperatureProfiles = func() map[string]temperatures.TemperatureProfileData {
		return map[string]temperatures.TemperatureProfileData{}
	}
	t.Cleanup(func() { coolingTemperatureProfiles = original })
	d := &Device{Serial: "corsair-one", Devices: map[int]*Devices{
		0: {ChannelId: 0, Name: "Pump", Label: "Liquid pump", Rpm: 2400, Temperature: 32, TemperatureString: "32.0°C", Profile: "Performance", HasSpeed: true, HasTemps: true, ContainsPump: true, PumpModes: map[byte]string{0: "Quiet", 1: "Normal", 2: "Performance"}},
		1: {ChannelId: 1, Name: "Fan 1", Label: "Rear fan", Rpm: 1100, Profile: "Balanced", HasSpeed: true},
	}}
	snapshot, ok := d.CoolingSnapshot()
	if !ok || len(snapshot.Channels) != 2 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	pump, fan := snapshot.Channels[0], snapshot.Channels[1]
	if pump.ID != 0 || pump.SourceID != 28 || !pump.ContainsPump || pump.RPM != 2400 || pump.Temperature != "32.0°C" || pump.Label != "Liquid pump" {
		t.Fatalf("pump = %#v", pump)
	}
	if fan.ID != 1 || fan.SourceID != 14 || fan.ContainsPump || fan.RPM != 1100 || fan.SelectedProfile != "Balanced" || fan.Label != "Rear fan" {
		t.Fatalf("fan = %#v", fan)
	}
	for index, option := range []string{"Quiet", "Normal", "Performance"} {
		if pump.PumpModeOptions[index].ID != option || pump.PumpModeOptions[index].Label != option {
			t.Fatalf("pump modes = %#v", pump.PumpModeOptions)
		}
	}
}

func TestCoolingSnapshotFailsClosedWhenRequiredCorsairOneStateIsIncomplete(t *testing.T) {
	original := coolingTemperatureProfiles
	coolingTemperatureProfiles = func() map[string]temperatures.TemperatureProfileData {
		return map[string]temperatures.TemperatureProfileData{}
	}
	t.Cleanup(func() { coolingTemperatureProfiles = original })
	d := &Device{Devices: map[int]*Devices{0: {ChannelId: 0, Name: "Pump", Profile: "Performance", HasSpeed: true, ContainsPump: true}}}
	if _, ok := d.CoolingSnapshot(); ok {
		t.Fatal("incomplete source topology unexpectedly exposed")
	}
}

func TestDeviceProfileSnapshotUsesCompleteCorsairOneContract(t *testing.T) {
	d := &Device{Serial: "corsair-one", DeviceProfile: &DeviceProfile{}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	snapshot, ok := d.DeviceProfileSnapshot()
	if !ok || !snapshot.CanSwitch || !snapshot.CanSave || !snapshot.CanDelete || snapshot.ActiveProfile != "Default" {
		t.Fatalf("profiles = %#v, ok=%t", snapshot, ok)
	}
	if _, ok := (&Device{}).DeviceProfileSnapshot(); ok {
		t.Fatal("nil device profile unexpectedly exposed")
	}
}
