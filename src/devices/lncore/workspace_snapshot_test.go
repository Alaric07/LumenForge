package lncore

import (
	"reflect"
	"testing"
)

func TestRGBTopologySnapshotPreservesCoreSourceOptions(t *testing.T) {
	device := &Device{Serial: "core", DeviceProfile: &DeviceProfile{ExternalHubDeviceType: 2, ExternalHubDeviceAmount: 3, Active: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}, ExternalLedDevice: []ExternalLedDevice{{Index: 2, Name: "LL Fan"}, {Index: 1, Name: "HD Fan"}}, ExternalLedDeviceAmount: map[int]string{3: "3 Devices", 0: "No Device", 1: "1 Device"}}
	snapshot, ok := device.RGBTopologySnapshot()
	if !ok || snapshot.DeviceID != "core" || len(snapshot.Ports) != 1 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	port := snapshot.Ports[0]
	if port.ID != 0 || port.SelectedType != 2 || port.SelectedAmount != 3 || !reflect.DeepEqual([]int{port.DeviceTypes[0].ID, port.DeviceTypes[1].ID, port.DeviceTypes[2].ID}, []int{0, 1, 2}) || !reflect.DeepEqual([]int{port.DeviceAmounts[0].ID, port.DeviceAmounts[1].ID, port.DeviceAmounts[2].ID}, []int{0, 1, 3}) {
		t.Fatalf("port = %#v", port)
	}
}

func TestRGBTopologySnapshotFailsClosedForInvalidCoreState(t *testing.T) {
	device := &Device{Serial: "core", DeviceProfile: &DeviceProfile{ExternalHubDeviceType: 9}, ExternalLedDevice: []ExternalLedDevice{{Index: 1, Name: "HD Fan"}}, ExternalLedDeviceAmount: map[int]string{0: "No Device"}}
	if _, ok := device.RGBTopologySnapshot(); ok {
		t.Fatal("invalid topology was accepted")
	}
}
