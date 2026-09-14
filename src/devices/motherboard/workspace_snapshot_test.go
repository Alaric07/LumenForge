package motherboard

import (
	"LumenForge/src/temperatures"
	"testing"
)

func TestCoolingSnapshotPreservesNativeMotherboardHeaders(t *testing.T) {
	original := coolingTemperatureProfiles
	coolingTemperatureProfiles = func() map[string]temperatures.TemperatureProfileData {
		return map[string]temperatures.TemperatureProfileData{"balanced": {}, "quiet": {}, "hidden": {Hidden: true}}
	}
	t.Cleanup(func() { coolingTemperatureProfiles = original })
	d := &Device{Serial: "native-board", Devices: map[int]*Devices{
		9: {ChannelId: 9, Name: "Pump", Label: "Pump", Rpm: 2450, Profile: "quiet", HasSpeed: true, ContainsPump: true, HeaderMode: 0, OperatingModes: map[int]string{2: "DC", 0: "BIOS", 1: "PWM"}},
		2: {ChannelId: 2, Name: "CPU Fan", Label: "CPU Cooler", Rpm: 1180, Profile: "balanced", HasSpeed: true, HeaderMode: 1, OperatingModes: map[int]string{2: "DC", 0: "BIOS", 1: "PWM"}},
		4: {ChannelId: 4, Name: "Chassis Fan", Label: "Front", Rpm: 820, Profile: "missing", HasSpeed: true},
	}}
	snapshot, ok := d.CoolingSnapshot()
	if !ok || len(snapshot.Channels) != 2 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	if snapshot.Channels[0].ID != 2 || snapshot.Channels[1].ID != 9 {
		got := []int{snapshot.Channels[0].ID, snapshot.Channels[1].ID}
		t.Fatalf("channel ids = %#v", got)
	}
	if snapshot.Channels[1].Label != "Pump" || !snapshot.Channels[1].ContainsPump || !snapshot.Channels[1].SpeedProfileDisabled {
		t.Fatalf("pump channel = %#v", snapshot.Channels[1])
	}
	if got := snapshot.Channels[0].OperatingModeOptions; len(got) != 3 || got[0].ID != 0 || got[1].ID != 1 || got[2].ID != 2 || got[0].Label != "BIOS" {
		t.Fatalf("operating modes = %#v", got)
	}
}

func TestDeviceProfileSnapshotRequiresCompleteNativeContract(t *testing.T) {
	d := &Device{Serial: "native-board", DeviceProfile: &DeviceProfile{}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Quiet": {}}}
	snapshot, ok := d.DeviceProfileSnapshot()
	if !ok || snapshot.ActiveProfile != "Default" || !snapshot.CanSwitch || !snapshot.CanSave || !snapshot.CanDelete {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	if _, ok := (&Device{Serial: "native-board", UserProfiles: d.UserProfiles}).DeviceProfileSnapshot(); ok {
		t.Fatal("profile snapshot unexpectedly usable without active device profile")
	}
}
