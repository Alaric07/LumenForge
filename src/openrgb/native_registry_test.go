package openrgb

import (
	"LumenForge/src/common"
	"testing"
)

func TestNativeControllerReconciliationWithoutTarget(t *testing.T) {
	const serial = "harpoon-w-registry-fixture"
	defer ReconcileDeviceController(serial, nil)
	first := &common.OpenRGBController{Serial: serial, Colors: make([]byte, 3)}
	second := &common.OpenRGBController{Serial: serial, Colors: make([]byte, 3)}
	AddDeviceController(first)
	AddDeviceController(first) // exercise cleanup of an existing duplicate
	ReconcileDeviceController(serial, second)
	mutex.Lock()
	count := 0
	for _, c := range controllers {
		if c.Serial == serial {
			count++
			if c != second {
				t.Error("stale controller")
			}
		}
	}
	mutex.Unlock()
	if count != 1 {
		t.Fatal("duplicate registrations", count)
	}
	ReconcileDeviceController(serial, nil)
	mutex.Lock()
	defer mutex.Unlock()
	for _, c := range controllers {
		if c.Serial == serial {
			t.Fatal("offline registration retained")
		}
	}
}
