package devices

import (
	"LumenForge/src/common"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/rgb"
	"testing"
)

type registryTestDevice struct{}

type schedulerReplayTestDevice struct {
	brightness []uint8
}

func (device *schedulerReplayTestDevice) SchedulerBrightness(value uint8) {
	device.brightness = append(device.brightness, value)
}

func isolateScheduledBrightnessForTest(t *testing.T) {
	t.Helper()
	schedulerMutex.Lock()
	previousBrightness := scheduledBrightness
	scheduledBrightness = nil
	schedulerMutex.Unlock()
	t.Cleanup(func() {
		schedulerMutex.Lock()
		scheduledBrightness = previousBrightness
		schedulerMutex.Unlock()
	})
}

type legacyRGBBoundaryTestDevice struct {
	profileLists        int
	controls            int
	profileSelections   int
	profileUpdates      int
	schedulerBrightness int
}

func (d *legacyRGBBoundaryTestDevice) GetRgbProfiles() interface{} {
	d.profileLists++
	return "profiles"
}

func (d *legacyRGBBoundaryTestDevice) ControlDeviceRgb(bool) {
	d.controls++
}

func (d *legacyRGBBoundaryTestDevice) UpdateRgbProfile(int, string) {
	d.profileSelections++
}

func (d *legacyRGBBoundaryTestDevice) UpdateRgbProfileData(string, rgb.Profile) {
	d.profileUpdates++
}

func (d *legacyRGBBoundaryTestDevice) SchedulerBrightness(uint8) {
	d.schedulerBrightness++
}

func (*registryTestDevice) SnapshotCount() int {
	return len(GetDevices())
}

func TestGetDevicesReturnsWrapperSnapshot(t *testing.T) {
	instance := &registryTestDevice{}
	mutex.Lock()
	previousDevices := devices
	devices = map[string]*common.Device{
		"test-device": {
			Product:     "Test Device",
			Serial:      "test-device",
			Instance:    instance,
			Unavailable: true,
		},
	}
	mutex.Unlock()
	t.Cleanup(func() {
		mutex.Lock()
		devices = previousDevices
		mutex.Unlock()
	})

	snapshot := GetDevices()
	snapshot["test-device"].Unavailable = false
	delete(snapshot, "test-device")

	secondSnapshot := GetDevices()
	if len(secondSnapshot) != 1 || !secondSnapshot["test-device"].Unavailable {
		t.Fatalf("mutating snapshot changed registry: %#v", secondSnapshot)
	}

	setDeviceAvailability("test-device", false)
	if GetDevices()["test-device"].Unavailable {
		t.Fatal("availability helper did not update registry wrapper")
	}
	setDevicePresentation("test-device", "Updated Product", "2.0", "updated.svg")
	presented := GetDevices()["test-device"]
	if presented.Product != "Updated Product" || presented.Firmware != "2.0" || presented.Image != "updated.svg" {
		t.Fatalf("presentation helper did not update registry wrapper: %#v", presented)
	}

	result := CallDeviceMethod("test-device", "SnapshotCount")
	if len(result) != 1 || result[0].Int() != 1 {
		t.Fatalf("reentrant registry method result = %#v", result)
	}
}

func TestLateDeviceReceivesCurrentScheduledBrightness(t *testing.T) {
	isolateScheduledBrightnessForTest(t)
	mutex.Lock()
	previousDevices := devices
	devices = make(map[string]*common.Device)
	mutex.Unlock()
	t.Cleanup(func() {
		mutex.Lock()
		devices = previousDevices
		mutex.Unlock()
	})

	ScheduleDeviceBrightness(0)
	late := &schedulerReplayTestDevice{}
	addDevice(&common.Device{Serial: "late-lights-out-device", Instance: late})
	if len(late.brightness) != 1 || late.brightness[0] != 0 {
		t.Fatalf("late device scheduler brightness = %v, want [0]", late.brightness)
	}

	ScheduleDeviceBrightness(1)
	later := &schedulerReplayTestDevice{}
	addDevice(&common.Device{Serial: "late-restored-device", Instance: later})
	if len(later.brightness) != 1 || later.brightness[0] != 1 {
		t.Fatalf("later device scheduler brightness = %v, want [1]", later.brightness)
	}
}

func TestOpenRGBImportRegistryHelpersUseExactInstance(t *testing.T) {
	mutex.Lock()
	previousDevices := devices
	devices = make(map[string]*common.Device)
	mutex.Unlock()
	t.Cleanup(func() {
		mutex.Lock()
		devices = previousDevices
		mutex.Unlock()
	})

	serial := "openrgb-hash-registry-test"
	instance := &openrgbimport.Device{Serial: serial, Product: "Imported"}
	wrapper := &common.Device{
		Serial:      serial,
		Product:     "Imported",
		Firmware:    "1.2.3",
		Image:       "imported.svg",
		Instance:    instance,
		Unavailable: true,
	}
	if err := RegisterOpenRGBImport(wrapper, instance); err != nil {
		t.Fatal(err)
	}
	if !wrapper.Unavailable {
		t.Fatal("registered importer wrapper availability changed unexpectedly")
	}

	if err := RegisterOpenRGBImport(wrapper, instance); err != nil {
		t.Fatalf("same-wrapper idempotent register: %v", err)
	}
	differentWrapper := &common.Device{Serial: serial, Product: "Snapshot", Instance: instance}
	if err := RegisterOpenRGBImport(differentWrapper, instance); err == nil {
		t.Fatal("different wrapper for the same importer instance was accepted")
	}
	lookedUp, lookedUpInstance, ok := LookupOpenRGBImport(serial)
	if !ok || lookedUpInstance != instance || lookedUp == wrapper {
		t.Fatalf("lookup = %#v, %p, %v", lookedUp, lookedUpInstance, ok)
	}
	lookedUp.Product = "Mutated Snapshot"
	if GetDevices()[serial].Product != "Imported" {
		t.Fatal("mutating lookup snapshot changed the registry")
	}

	replacement := &openrgbimport.Device{Serial: serial, Product: "Replacement"}
	if err := RegisterOpenRGBImport(&common.Device{Serial: serial, Instance: replacement}, replacement); err == nil {
		t.Fatal("different importer instance collision was accepted")
	}
	if removed, ok := RemoveOpenRGBImport(serial, replacement); ok || removed != nil {
		t.Fatal("pointer-mismatch removal succeeded")
	}
	removed, ok := RemoveOpenRGBImport(serial, instance)
	if !ok || removed != wrapper {
		t.Fatal("exact importer instance was not removed")
	}
	if removed.Product != "Imported" || removed.Firmware != "1.2.3" ||
		removed.Image != "imported.svg" || !removed.Unavailable ||
		removed.Instance != instance {
		t.Fatalf("removed wrapper fields changed: %#v", removed)
	}
	if _, _, ok := LookupOpenRGBImport(serial); ok {
		t.Fatal("removed importer still exists")
	}
}

func TestLegacyGlobalRGBHelpersSkipClusterAndOpenRGBImports(t *testing.T) {
	isolateScheduledBrightnessForTest(t)
	native := &legacyRGBBoundaryTestDevice{}
	cluster := &legacyRGBBoundaryTestDevice{}
	imported := &openrgbimport.Device{Serial: "imported", IsOpenRGB: true}
	mutex.Lock()
	previousDevices := devices
	devices = map[string]*common.Device{
		"native": {
			ProductType: common.ProductTypeLinkHub,
			Serial:      "native",
			Instance:    native,
		},
		"cluster": {
			ProductType: common.ProductTypeCluster,
			Serial:      "cluster",
			Instance:    cluster,
		},
		"imported": {
			ProductType: common.ProductTypeMotherboard,
			Serial:      "imported",
			Instance:    imported,
		},
	}
	mutex.Unlock()
	t.Cleanup(func() {
		mutex.Lock()
		devices = previousDevices
		mutex.Unlock()
	})

	profiles := GetRgbProfiles()
	if profiles["native"] != "profiles" {
		t.Fatalf("native legacy RGB profiles = %#v", profiles["native"])
	}
	if _, ok := profiles["cluster"]; ok {
		t.Fatalf("Cluster leaked into legacy RGB profiles: %#v", profiles)
	}
	if _, ok := profiles["imported"]; ok {
		t.Fatalf("OpenRGB import leaked into legacy RGB profiles: %#v", profiles)
	}
	ControlDeviceRgb(true)
	UpdateGlobalRgbProfile("wave")
	UpdateAllDevicesStaticColor(rgb.Color{Red: 1, Green: 2, Blue: 3})

	if native.profileLists != 1 || native.controls != 1 || native.profileSelections != 2 || native.profileUpdates != 1 {
		t.Fatalf("native legacy RGB calls = %#v", native)
	}
	if cluster.profileLists != 0 || cluster.controls != 0 || cluster.profileSelections != 0 || cluster.profileUpdates != 0 {
		t.Fatalf("Cluster received legacy global RGB calls = %#v", cluster)
	}
	if eligibleForLegacyGlobalRGB(devices["imported"]) {
		t.Fatal("OpenRGB import is eligible for legacy global RGB")
	}

	mutex.Lock()
	delete(devices, "imported")
	mutex.Unlock()
	ScheduleDeviceBrightness(0)
	if native.schedulerBrightness != 1 || cluster.schedulerBrightness != 1 {
		t.Fatalf("scheduler Brightness calls = native %d Cluster %d", native.schedulerBrightness, cluster.schedulerBrightness)
	}
}
