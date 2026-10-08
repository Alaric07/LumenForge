package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices/scimitarSEW"
	"LumenForge/src/devices/scimitarSEWU"
	"LumenForge/src/devices/scimitarW"
	"LumenForge/src/devices/scimitarWU"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"reflect"
	"sort"
	"testing"
)

var _ nativeDeviceLightingTarget = (*scimitarWU.Device)(nil)
var _ nativeDeviceLightingTarget = (*scimitarSEWU.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*scimitarWU.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*scimitarSEWU.Device)(nil)

type scimitarCanonicalProvider struct {
	serial   string
	usable   bool
	snapshot lightingpresentation.Snapshot
}

func (p scimitarCanonicalProvider) LightingDeviceID() string { return p.serial }
func (p scimitarCanonicalProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return p.snapshot, p.usable
}
func (p scimitarCanonicalProvider) ProcessSetRgbCluster(bool) uint8         { return 1 }
func (p scimitarCanonicalProvider) ProcessSetOpenRgbIntegration(bool) uint8 { return 1 }

func TestScimitarDirectHIDCutoverRequiresUsableSnapshot(t *testing.T) {
	for _, productType := range []uint16{common.ProductTypeScimitarRgbEliteWU, common.ProductTypeScimitarRgbEliteSEWU} {
		for _, usable := range []bool{false, true} {
			serial := "scimitar-canonical"
			provider := scimitarCanonicalProvider{serial: serial, usable: usable, snapshot: lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, HasBrightness: true, Brightness: 70, SupportedEffects: []lightingpresentation.EffectOption{{ID: "mouse", Label: "Mouse"}}}}
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
	for _, instance := range []interface{}{&scimitarWU.Device{Serial: "scimitar-offline"}, &scimitarSEWU.Device{Serial: "scimitar-offline"}} {
		summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{"scimitar-offline": {Serial: "scimitar-offline", ProductType: common.ProductTypeScimitarRgbEliteWU, Instance: instance}}, map[string]stats.BatteryStats{}, "scimitar-offline")
		if !ok || !summary.LegacyLighting || summary.Lighting != nil {
			t.Fatal("unattached real package lost fallback")
		}
	}
}

func TestScimitarDirectHIDResolverKeepsPackageIdentityAndGuards(t *testing.T) {
	original := lookupNativeDeviceLightingWrapper
	t.Cleanup(func() { lookupNativeDeviceLightingWrapper = original })
	for _, target := range []nativeDeviceLightingTarget{&scimitarWU.Device{Serial: "ProUSB"}, &scimitarSEWU.Device{Serial: "ProSEUSB"}} {
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
	for _, instance := range []interface{}{&scimitarW.Device{}, &scimitarSEW.Device{}} {
		if _, ok := instance.(devicesLightingSnapshotProvider); ok {
			t.Fatal("receiver package migrated without authorization")
		}
	}
}

func TestScimitarDirectHIDPreviewsAreInertExactDescriptors(t *testing.T) {
	want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	sort.Strings(want)
	names := []string{"Side", "Logo"}
	indices := [][]int{{1, 5, 9}, {0, 4, 8}}
	for _, se := range []bool{false, true} {
		s := buildScimitarDirectHIDModernPreview(se)
		lighting := s.Lighting
		minDPI, maxDPI := 100, 26000
		if se {
			minDPI, maxDPI = 100, 33000
		}
		if s.DPI.MinimumDPI != minDPI || s.DPI.MaximumDPI != maxDPI || s.Performance.LiftHeight == nil || s.Performance.PollingRate == nil {
			t.Fatal("preview lost package-specific capabilities")
		}
		if s.LegacyLighting || lighting == nil || lighting.ConfiguredEffect != "mouse" || lighting.Brightness != 70 || len(lighting.SupportedEffects) != 20 || lighting.AuthoredZoneEditor == nil || len(lighting.AuthoredZoneEditor.Zones) != 2 {
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
		for id, zone := range scimitarDirectHIDPreviewZones() {
			if zone.name != names[id] || !reflect.DeepEqual(zone.indices, indices[id]) || lighting.AuthoredZoneEditor.Zones[id].Label != zone.name || lighting.AuthoredZoneEditor.Zones[id].ColorHex != zone.color {
				t.Fatalf("zone=%#v", zone)
			}
		}
		if lighting.ClusterControlled || lighting.ExternalControlled {
			t.Fatal("preview initialized external ownership")
		}
		// Every build owns its data; mutations cannot poison subsequent fixtures.
		lighting.AuthoredZoneEditor.Zones[0].Label = "changed"
		if buildScimitarDirectHIDModernPreview(se).Lighting.AuthoredZoneEditor.Zones[0].Label != "Side" {
			t.Fatal("fixture aliases")
		}
	}
	if buildScimitarDirectHIDModernPreview(false).Serial == buildScimitarDirectHIDModernPreview(true).Serial || buildScimitarDirectHIDModernPreview(false).Product == buildScimitarDirectHIDModernPreview(true).Product {
		t.Fatal("package identities collapsed")
	}
}
