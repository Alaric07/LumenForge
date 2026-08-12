package systray

import (
	"reflect"
	"testing"

	"LumenForge/src/common"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/rgb"
	"github.com/godbus/dbus/v5"
)

const importedTrayTestSerial = "openrgb-tray-test"

func installImportedTrayTestSeams(t *testing.T) (*openrgbimport.Device, *common.Device, *openrgbimport.LightingSnapshot, *[]string) {
	t.Helper()
	previousDevices := getTrayDevices
	previousClusterStatus := getTrayDeviceClusterStatus
	previousCall := callTrayDeviceMethod
	previousLookup := lookupOpenRGBTrayDevice
	previousSnapshot := snapshotOpenRGBTrayLighting
	previousSet := setOpenRGBTrayEffect
	previousMap := deviceMap
	previousMenuItems := menuItems
	previousMenuOrder := menuOrder
	previousMenuRevision := menuRevision
	previousScrapbook := deviceAnimationScrapbook
	previousOff := nonClusteredRgbOff

	device := &openrgbimport.Device{Serial: importedTrayTestSerial, IsOpenRGB: true}
	wrapper := &common.Device{Serial: importedTrayTestSerial, Product: "Imported Tray Device", Instance: device}
	snapshot := &openrgbimport.LightingSnapshot{
		ConfiguredEffect: "rainbow",
		SupportedEffects: []openrgbimport.LightingEffectOption{
			{ID: "wave", Label: "Wave"},
			{ID: "aurora", Label: "Aurora"},
		},
	}
	setEffects := []string{}
	getTrayDevices = func() map[string]*common.Device {
		return map[string]*common.Device{importedTrayTestSerial: wrapper}
	}
	getTrayDeviceClusterStatus = func(string) bool {
		t.Fatal("imported tray device fell through to native cluster membership")
		return false
	}
	callTrayDeviceMethod = func(string, string, ...interface{}) []reflect.Value {
		t.Fatal("imported tray device fell through to native effect lookup")
		return nil
	}
	lookupOpenRGBTrayDevice = func(serial string) (*common.Device, *openrgbimport.Device, bool) {
		return wrapper, device, serial == importedTrayTestSerial
	}
	snapshotOpenRGBTrayLighting = func(candidate *openrgbimport.Device) (openrgbimport.LightingSnapshot, bool) {
		return *snapshot, candidate == device
	}
	setOpenRGBTrayEffect = func(candidate *openrgbimport.Device, effect string) error {
		if candidate != device {
			t.Fatalf("tray mutation used device %p, want %p", candidate, device)
		}
		setEffects = append(setEffects, effect)
		snapshot.ConfiguredEffect = effect
		return nil
	}
	deviceMap = map[int]string{0: importedTrayTestSerial}
	menuItems = make(map[int32]MenuLayout)
	menuOrder = nil
	menuRevision = 1
	deviceAnimationScrapbook = make(map[string]string)
	nonClusteredRgbOff = false
	t.Cleanup(func() {
		getTrayDevices = previousDevices
		getTrayDeviceClusterStatus = previousClusterStatus
		callTrayDeviceMethod = previousCall
		lookupOpenRGBTrayDevice = previousLookup
		snapshotOpenRGBTrayLighting = previousSnapshot
		setOpenRGBTrayEffect = previousSet
		deviceMap = previousMap
		menuItems = previousMenuItems
		menuOrder = previousMenuOrder
		menuRevision = previousMenuRevision
		deviceAnimationScrapbook = previousScrapbook
		nonClusteredRgbOff = previousOff
	})
	return device, wrapper, snapshot, &setEffects
}

func trayMenuLabelsForTest(t *testing.T, id int32) []string {
	t.Helper()
	menuMutex.Lock()
	layout, ok := menuItems[id]
	menuMutex.Unlock()
	if !ok {
		t.Fatalf("tray menu %d was not built", id)
	}

	var labels []string
	var collect func(MenuLayout)
	collect = func(item MenuLayout) {
		if value, found := item.Props["label"]; found {
			if label, valid := value.Value().(string); valid {
				labels = append(labels, label)
			}
		}
		for _, child := range item.Children {
			childLayout, valid := child.Value().(MenuLayout)
			if !valid {
				t.Fatalf("tray child = %#v, want MenuLayout", child.Value())
			}
			collect(childLayout)
		}
	}
	collect(layout)
	return labels
}

func requireTrayLabelsForTest(t *testing.T, labels []string, present, absent []string) {
	t.Helper()
	for _, expected := range present {
		if !containsString(labels, expected) {
			t.Fatalf("tray labels = %#v, missing %q", labels, expected)
		}
	}
	for _, unexpected := range absent {
		if containsString(labels, unexpected) {
			t.Fatalf("tray labels = %#v, unexpectedly contain %q", labels, unexpected)
		}
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestImportedDeviceTrayLightingUsesSortedDefensiveCanonicalSnapshot(t *testing.T) {
	_, _, snapshot, _ := installImportedTrayTestSeams(t)

	state, imported := importedDeviceTrayLighting(importedTrayTestSerial)
	if !imported || !state.ready {
		t.Fatalf("imported tray state = %#v, %t", state, imported)
	}
	want := []openrgbimport.LightingEffectOption{
		{ID: "aurora", Label: "Aurora"},
		{ID: "wave", Label: "Wave"},
	}
	if !reflect.DeepEqual(state.effects, want) {
		t.Fatalf("sorted imported effects = %#v, want %#v", state.effects, want)
	}
	if snapshot.SupportedEffects[0].ID != "wave" {
		t.Fatal("sorting the tray catalogue mutated the canonical snapshot")
	}
	state.effects[0].ID = "changed"
	again, _ := importedDeviceTrayLighting(importedTrayTestSerial)
	if !reflect.DeepEqual(again.effects, want) {
		t.Fatalf("tray catalogue was not defensive: %#v", again.effects)
	}
}

func TestIndividualDevicesAboutToShowRefreshesImportedMembership(t *testing.T) {
	_, _, snapshot, _ := installImportedTrayTestSeams(t)
	server := &MenuServer{}

	if needsUpdate, dbusErr := server.AboutToShow(106); !needsUpdate || dbusErr != nil {
		t.Fatalf("standalone AboutToShow = %t, %v", needsUpdate, dbusErr)
	}
	requireTrayLabelsForTest(t, trayMenuLabelsForTest(t, 106),
		[]string{"Imported Tray Device", "Aurora", "Wave"},
		[]string{"Imported Tray Device [Clustered]", "● Managed by Global Cluster"},
	)

	snapshot.ClusterControlled = true
	server.AboutToShow(106)
	requireTrayLabelsForTest(t, trayMenuLabelsForTest(t, 106),
		[]string{"Imported Tray Device [Clustered]", "● Managed by Global Cluster"},
		[]string{"Imported Tray Device", "Aurora", "Wave"},
	)

	snapshot.ClusterControlled = false
	server.AboutToShow(106)
	requireTrayLabelsForTest(t, trayMenuLabelsForTest(t, 106),
		[]string{"Imported Tray Device", "Aurora", "Wave"},
		[]string{"Imported Tray Device [Clustered]", "● Managed by Global Cluster"},
	)
}

func TestIndividualDevicesImportedUnavailableStateAvoidsNativeFallback(t *testing.T) {
	t.Run("unavailable wrapper", func(t *testing.T) {
		_, wrapper, _, _ := installImportedTrayTestSeams(t)
		wrapper.Unavailable = true

		server := &MenuServer{}
		server.AboutToShow(106)
		requireTrayLabelsForTest(t, trayMenuLabelsForTest(t, 106),
			[]string{"Imported Tray Device"},
			[]string{"Imported Tray Device [Clustered]", "● Managed by Global Cluster", "Aurora", "Wave"},
		)
	})

	t.Run("invalid snapshot", func(t *testing.T) {
		device, _, _, _ := installImportedTrayTestSeams(t)
		snapshotOpenRGBTrayLighting = func(candidate *openrgbimport.Device) (openrgbimport.LightingSnapshot, bool) {
			if candidate != device {
				t.Fatalf("snapshot candidate = %p, want %p", candidate, device)
			}
			return openrgbimport.LightingSnapshot{}, false
		}

		server := &MenuServer{}
		server.AboutToShow(106)
		requireTrayLabelsForTest(t, trayMenuLabelsForTest(t, 106),
			[]string{"Imported Tray Device"},
			[]string{"Imported Tray Device [Clustered]", "● Managed by Global Cluster", "Aurora", "Wave"},
		)
	})
}

func TestIndividualDevicesNativeDeviceKeepsLegacyMenuPath(t *testing.T) {
	_, wrapper, _, _ := installImportedTrayTestSeams(t)
	wrapper.Serial = "native-tray-test"
	wrapper.Product = "Native Tray Device"
	wrapper.Instance = struct{}{}
	getTrayDevices = func() map[string]*common.Device {
		return map[string]*common.Device{wrapper.Serial: wrapper}
	}
	lookupOpenRGBTrayDevice = func(serial string) (*common.Device, *openrgbimport.Device, bool) {
		if serial != wrapper.Serial {
			t.Fatalf("native import lookup serial = %q, want %q", serial, wrapper.Serial)
		}
		return nil, nil, false
	}
	clusterChecks := 0
	getTrayDeviceClusterStatus = func(serial string) bool {
		clusterChecks++
		if serial != wrapper.Serial {
			t.Fatalf("native cluster lookup serial = %q, want %q", serial, wrapper.Serial)
		}
		return false
	}
	profileChecks := 0
	callTrayDeviceMethod = func(serial, method string, args ...interface{}) []reflect.Value {
		profileChecks++
		if serial != wrapper.Serial || method != "GetRgbProfiles" || len(args) != 0 {
			t.Fatalf("native profile lookup = %q, %q, %#v", serial, method, args)
		}
		profiles := rgb.RGB{Profiles: map[string]rgb.Profile{"wave": {}, "static": {}}}
		return []reflect.Value{reflect.ValueOf(profiles)}
	}

	server := &MenuServer{}
	server.AboutToShow(106)
	if clusterChecks != 1 || profileChecks != 1 {
		t.Fatalf("native legacy calls = cluster %d, profiles %d", clusterChecks, profileChecks)
	}
	requireTrayLabelsForTest(t, trayMenuLabelsForTest(t, 106),
		[]string{"Native Tray Device", "Static", "Wave"},
		[]string{"Native Tray Device [Clustered]", "● Managed by Global Cluster"},
	)
}

func TestImportedDeviceTrayEventUsesCanonicalEffectAndHonorsClusterOwnership(t *testing.T) {
	_, _, snapshot, setEffects := installImportedTrayTestSeams(t)
	server := &MenuServer{}

	server.Event(1000, "clicked", dbus.Variant{}, 0)
	if !reflect.DeepEqual(*setEffects, []string{"aurora"}) {
		t.Fatalf("imported tray selection = %#v, want canonical aurora", *setEffects)
	}

	snapshot.ClusterControlled = true
	server.Event(1001, "clicked", dbus.Variant{}, 0)
	if !reflect.DeepEqual(*setEffects, []string{"aurora"}) {
		t.Fatalf("cluster-owned imported tray selection mutated lighting: %#v", *setEffects)
	}
}

func TestImportedDeviceTrayStandaloneToggleUsesCanonicalOffAndRestore(t *testing.T) {
	_, _, snapshot, setEffects := installImportedTrayTestSeams(t)
	server := &MenuServer{}

	server.Event(999, "clicked", dbus.Variant{}, 0)
	if snapshot.ConfiguredEffect != "off" || !reflect.DeepEqual(*setEffects, []string{"off"}) {
		t.Fatalf("imported standalone disable = effect %q calls %#v", snapshot.ConfiguredEffect, *setEffects)
	}
	if saved := deviceAnimationScrapbook[importedTrayTestSerial]; saved != "rainbow" {
		t.Fatalf("imported tray scrapbook after disable = %q, want rainbow", saved)
	}

	server.Event(999, "clicked", dbus.Variant{}, 0)
	if snapshot.ConfiguredEffect != "rainbow" || !reflect.DeepEqual(*setEffects, []string{"off", "rainbow"}) {
		t.Fatalf("imported standalone restore = effect %q calls %#v", snapshot.ConfiguredEffect, *setEffects)
	}
	if _, found := deviceAnimationScrapbook[importedTrayTestSerial]; found {
		t.Fatal("successful imported restore retained scrapbook entry")
	}

	snapshot.ConfiguredEffect = "off"
	server.Event(999, "clicked", dbus.Variant{}, 0)
	server.Event(999, "clicked", dbus.Variant{}, 0)
	if snapshot.ConfiguredEffect != "off" || !reflect.DeepEqual(*setEffects, []string{"off", "rainbow"}) {
		t.Fatalf("stale imported restore changed effect %q with calls %#v", snapshot.ConfiguredEffect, *setEffects)
	}
	if _, found := deviceAnimationScrapbook[importedTrayTestSerial]; found {
		t.Fatal("already-Off imported device retained a stale scrapbook entry")
	}
}
