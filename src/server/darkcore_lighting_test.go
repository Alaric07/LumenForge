package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices/darkcorergbproW"
	"LumenForge/src/devices/darkcorergbproWU"
	"LumenForge/src/devices/darkcorergbproseW"
	"LumenForge/src/devices/darkcorergbproseWU"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"reflect"
	"sort"
	"testing"
)

var _ nativeDeviceLightingTarget = (*darkcorergbproWU.Device)(nil)
var _ nativeDeviceLightingTarget = (*darkcorergbproseWU.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*darkcorergbproWU.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*darkcorergbproseWU.Device)(nil)

type darkCoreCanonicalProvider struct {
	serial   string
	usable   bool
	snapshot lightingpresentation.Snapshot
}

func (p darkCoreCanonicalProvider) LightingDeviceID() string { return p.serial }
func (p darkCoreCanonicalProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return p.snapshot, p.usable
}
func (p darkCoreCanonicalProvider) ProcessSetRgbCluster(bool) uint8         { return 1 }
func (p darkCoreCanonicalProvider) ProcessSetOpenRgbIntegration(bool) uint8 { return 1 }

func TestDarkCoreDirectHIDCutoverRequiresUsableSnapshot(t *testing.T) {
	for _, productType := range []uint16{common.ProductTypeDarkCoreRgbProWU, common.ProductTypeDarkCoreRgbProSEWU} {
		for _, usable := range []bool{false, true} {
			serial := "darkcore-canonical"
			provider := darkCoreCanonicalProvider{serial: serial, usable: usable, snapshot: lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, HasBrightness: true, Brightness: 70, SupportedEffects: []lightingpresentation.EffectOption{{ID: "mouse", Label: "Mouse"}}}}
			summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, ProductType: productType, Instance: provider}}, map[string]stats.BatteryStats{}, serial)
			if !ok || summary.LegacyLighting == usable || (summary.Lighting != nil) != usable {
				t.Fatalf("type=%d usable=%t summary=%#v", productType, usable, summary)
			}
			if usable && (!summary.Lighting.ClusterOwnershipAvailable || !summary.Lighting.ExternalOwnershipAvailable) {
				t.Fatal("real ownership capabilities hidden")
			}
			provider.snapshot.TargetKind = ""
			summary, ok = devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, ProductType: productType, Instance: provider}}, map[string]stats.BatteryStats{}, serial)
			if !ok || !summary.LegacyLighting || summary.Lighting != nil {
				t.Fatal("unusable snapshot suppressed fallback")
			}
		}
	}
	// Real unattached packages fail closed and remain eligible for legacy Lighting.
	for _, instance := range []interface{}{&darkcorergbproWU.Device{Serial: "darkcore-offline"}, &darkcorergbproseWU.Device{Serial: "darkcore-offline"}} {
		summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{"darkcore-offline": {Serial: "darkcore-offline", ProductType: common.ProductTypeDarkCoreRgbProWU, Instance: instance}}, map[string]stats.BatteryStats{}, "darkcore-offline")
		if !ok || !summary.LegacyLighting || summary.Lighting != nil {
			t.Fatal("unattached real package lost fallback")
		}
	}
}

func TestDarkCoreDirectHIDResolverKeepsPackageIdentityAndGuards(t *testing.T) {
	original := lookupNativeDeviceLightingWrapper
	t.Cleanup(func() { lookupNativeDeviceLightingWrapper = original })
	for _, target := range []nativeDeviceLightingTarget{&darkcorergbproWU.Device{Serial: "ProUSB"}, &darkcorergbproseWU.Device{Serial: "ProSEUSB"}} {
		lookupNativeDeviceLightingWrapper = func(serial string) (*common.Device, bool) {
			return &common.Device{Serial: target.LightingDeviceID(), Instance: target}, true
		}
		resolved, err := getNativeDeviceLightingTarget(target.LightingDeviceID())
		if err != nil || resolved != target {
			t.Fatal("package adapter unresolved")
		}
		if _, err = getNativeDeviceLightingTarget("wrong-identity"); err == nil {
			t.Fatal("identity mismatch accepted")
		}
		if resolved.SetLightingBrightness(50) == nil {
			t.Fatal("server target accepted unavailable runtime")
		}
	}
	for _, instance := range []interface{}{&darkcorergbproW.Device{}, &darkcorergbproseW.Device{}} {
		if _, ok := instance.(devicesLightingSnapshotProvider); ok {
			t.Fatal("receiver package migrated without authorization")
		}
	}
}

func TestDarkCoreDirectHIDPreviewsAreInertExactDescriptors(t *testing.T) {
	want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	sort.Strings(want)
	names := []string{"Scroll", "Logo", "Side Accent 1", "Side Accent 2", "Side Accent 3", "Side Accent 4", "Side Accent 5", "Side Accent 6"}
	indices := [][]int{{0, 12, 24}, {6, 18, 30}, {1, 13, 25}, {2, 14, 26}, {3, 15, 27}, {4, 16, 28}, {5, 17, 29}, {7, 19, 31}}
	for _, se := range []bool{false, true} {
		s := buildDarkCoreDirectHIDModernPreview(se)
		lighting := s.Lighting
		if s.LegacyLighting || lighting == nil || lighting.ConfiguredEffect != "mouse" || lighting.Brightness != 70 || len(lighting.SupportedEffects) != 20 || lighting.AuthoredZoneEditor == nil || len(lighting.AuthoredZoneEditor.Zones) != 8 {
			t.Fatalf("preview=%#v", s)
		}
		got := []string{}
		for _, effect := range lighting.SupportedEffects {
			got = append(got, effect.ID)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("catalogue=%v", got)
		}
		for id, zone := range darkCoreDirectHIDPreviewZones() {
			if zone.name != names[id] || !reflect.DeepEqual(zone.indices, indices[id]) || lighting.AuthoredZoneEditor.Zones[id].Label != zone.name || lighting.AuthoredZoneEditor.Zones[id].ColorHex != zone.color {
				t.Fatalf("zone=%#v", zone)
			}
		}
		if lighting.ClusterControlled || lighting.ExternalControlled {
			t.Fatal("preview initialized external ownership")
		}
		// Every build owns its data; mutations cannot poison subsequent fixtures.
		lighting.AuthoredZoneEditor.Zones[0].Label = "changed"
		if buildDarkCoreDirectHIDModernPreview(se).Lighting.AuthoredZoneEditor.Zones[0].Label != "Scroll" {
			t.Fatal("fixture aliases")
		}
	}
	if buildDarkCoreDirectHIDModernPreview(false).Serial == buildDarkCoreDirectHIDModernPreview(true).Serial || buildDarkCoreDirectHIDModernPreview(false).Product == buildDarkCoreDirectHIDModernPreview(true).Product {
		t.Fatal("package identities collapsed")
	}
}
