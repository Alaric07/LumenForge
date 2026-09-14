package lnpro

import (
	"reflect"
	"testing"
)

func TestRGBTopologySnapshotPreservesIndependentPortState(t *testing.T) {
	device := &Device{Serial: "pro", DeviceProfile: &DeviceProfile{ExternalHubs: map[int]*ExternalHubData{1: {PortId: 1, ExternalHubDeviceType: 2, ExternalHubDeviceAmount: 4}, 0: {PortId: 0, ExternalHubDeviceType: 1, ExternalHubDeviceAmount: 2}}, Active: true}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}, ExternalLedDevice: []ExternalLedDevice{{Index: 2, Name: "LL Fan"}, {Index: 1, Name: "HD Fan"}}, ExternalLedDeviceAmount: map[int]string{4: "4 Devices", 0: "No Device", 2: "2 Devices"}}
	snapshot, ok := device.RGBTopologySnapshot()
	if !ok || len(snapshot.Ports) != 2 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	if snapshot.Ports[0].ID != 0 || snapshot.Ports[0].SelectedType != 1 || snapshot.Ports[0].SelectedAmount != 2 || snapshot.Ports[1].ID != 1 || snapshot.Ports[1].SelectedType != 2 || snapshot.Ports[1].SelectedAmount != 4 {
		t.Fatalf("ports = %#v", snapshot.Ports)
	}
	if !reflect.DeepEqual([]int{snapshot.Ports[0].DeviceTypes[0].ID, snapshot.Ports[0].DeviceTypes[1].ID, snapshot.Ports[0].DeviceTypes[2].ID}, []int{0, 1, 2}) || !reflect.DeepEqual([]int{snapshot.Ports[0].DeviceAmounts[0].ID, snapshot.Ports[0].DeviceAmounts[1].ID, snapshot.Ports[0].DeviceAmounts[2].ID}, []int{0, 2, 4}) {
		t.Fatalf("options = %#v", snapshot.Ports[0])
	}
}

func TestRGBTopologySnapshotFailsClosedForMissingPortState(t *testing.T) {
	device := &Device{Serial: "pro", DeviceProfile: &DeviceProfile{ExternalHubs: map[int]*ExternalHubData{0: {PortId: 1, ExternalHubDeviceType: 1, ExternalHubDeviceAmount: 1}}}, ExternalLedDevice: []ExternalLedDevice{{Index: 1, Name: "HD Fan"}}, ExternalLedDeviceAmount: map[int]string{1: "1 Device"}}
	if _, ok := device.RGBTopologySnapshot(); ok {
		t.Fatal("invalid topology was accepted")
	}
}
