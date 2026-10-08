package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices/ironclawSEW"
	"LumenForge/src/devices/ironclawSEWU"
	"LumenForge/src/devices/ironclawW"
	"LumenForge/src/devices/ironclawWU"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"reflect"
	"sort"
	"testing"
)

var _ nativeDeviceLightingTarget = (*ironclawW.Device)(nil)
var _ nativeDeviceLightingTarget = (*ironclawSEW.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*ironclawW.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*ironclawSEW.Device)(nil)
var _ nativeDeviceLightingTarget = (*ironclawWU.Device)(nil)
var _ nativeDeviceLightingTarget = (*ironclawSEWU.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*ironclawWU.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*ironclawSEWU.Device)(nil)

type ironclawCanonicalProvider struct {
	serial   string
	usable   bool
	snapshot lightingpresentation.Snapshot
}

func (p ironclawCanonicalProvider) LightingDeviceID() string { return p.serial }
func (p ironclawCanonicalProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return p.snapshot, p.usable
}
func (p ironclawCanonicalProvider) ProcessSetRgbCluster(bool) uint8         { return 1 }
func (p ironclawCanonicalProvider) ProcessSetOpenRgbIntegration(bool) uint8 { return 1 }

func TestIronclawDirectHIDCutoverRequiresUsableSnapshot(t *testing.T) {
	for _, productType := range []uint16{common.ProductTypeIronClawRgbWU, common.ProductTypeIronClawSEWU, common.ProductTypeIronClawRgbW, common.ProductTypeIronClawSEW} {
		for _, usable := range []bool{false, true} {
			serial := "ironclaw-canonical"
			provider := ironclawCanonicalProvider{serial: serial, usable: usable, snapshot: lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, HasBrightness: true, Brightness: 70, SupportedEffects: []lightingpresentation.EffectOption{{ID: "mouse", Label: "Mouse"}}}}
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
	for _, instance := range []interface{}{&ironclawW.Device{Serial: "ironclaw-offline"}, &ironclawSEW.Device{Serial: "ironclaw-offline"}, &ironclawWU.Device{Serial: "ironclaw-offline"}, &ironclawSEWU.Device{Serial: "ironclaw-offline"}} {
		summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{"ironclaw-offline": {Serial: "ironclaw-offline", ProductType: common.ProductTypeIronClawRgbWU, Instance: instance}}, map[string]stats.BatteryStats{}, "ironclaw-offline")
		if !ok || !summary.LegacyLighting || summary.Lighting != nil {
			t.Fatal("unattached real package lost fallback")
		}
	}
}

func TestIronclawDirectHIDResolverKeepsPackageIdentityAndGuards(t *testing.T) {
	original := lookupNativeDeviceLightingWrapper
	t.Cleanup(func() { lookupNativeDeviceLightingWrapper = original })
	for _, target := range []nativeDeviceLightingTarget{&ironclawW.Device{Serial: "ProReceiver"}, &ironclawSEW.Device{Serial: "SEReceiver"}, &ironclawWU.Device{Serial: "ProUSB"}, &ironclawSEWU.Device{Serial: "ProSEUSB"}} {
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
}

func TestIronclawDirectHIDPreviewsAreInertExactDescriptors(t *testing.T) {
	want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	sort.Strings(want)
	names := []string{"Logo", "Scroll", "Front"}
	indices := [][]int{{0, 6, 12}, {1, 7, 13}, {2, 8, 14}}
	for _, se := range []bool{false, true} {
		s := buildIronclawDirectHIDModernPreview(se)
		lighting := s.Lighting
		minDPI, maxDPI := 200, 18000
		if se {
			minDPI, maxDPI = 100, 26000
		}
		if s.DPI.MinimumDPI != minDPI || s.DPI.MaximumDPI != maxDPI || (s.Performance.LiftHeight != nil) != se || s.Performance.PollingRate == nil {
			t.Fatal("preview lost package-specific capabilities")
		}
		if s.LegacyLighting || lighting == nil || lighting.ConfiguredEffect != "mouse" || lighting.Brightness != 70 || len(lighting.SupportedEffects) != 20 || lighting.AuthoredZoneEditor == nil || len(lighting.AuthoredZoneEditor.Zones) != 3 {
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
		for id, zone := range ironclawDirectHIDPreviewZones() {
			if zone.name != names[id] || !reflect.DeepEqual(zone.indices, indices[id]) || lighting.AuthoredZoneEditor.Zones[id].Label != zone.name || lighting.AuthoredZoneEditor.Zones[id].ColorHex != zone.color {
				t.Fatalf("zone=%#v", zone)
			}
		}
		if lighting.ClusterControlled || lighting.ExternalControlled {
			t.Fatal("preview initialized external ownership")
		}
		// Every build owns its data; mutations cannot poison subsequent fixtures.
		lighting.AuthoredZoneEditor.Zones[0].Label = "changed"
		if buildIronclawDirectHIDModernPreview(se).Lighting.AuthoredZoneEditor.Zones[0].Label != "Logo" {
			t.Fatal("fixture aliases")
		}
	}
	if buildIronclawDirectHIDModernPreview(false).Serial == buildIronclawDirectHIDModernPreview(true).Serial || buildIronclawDirectHIDModernPreview(false).Product == buildIronclawDirectHIDModernPreview(true).Product {
		t.Fatal("package identities collapsed")
	}
}

func TestIronclawReceiverPreviewsAreInertExactDescriptors(t *testing.T) {
	want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	sort.Strings(want)
	names := []string{"Logo", "Scroll", "Front"}
	indices := [][]int{{0, 6, 12}, {1, 7, 13}, {2, 8, 14}}
	for _, se := range []bool{false, true} {
		s := buildIronclawReceiverModernPreview(se)
		lighting := s.Lighting
		minDPI, maxDPI := 200, 18000
		if se {
			minDPI, maxDPI = 100, 26000
		}
		if s.DPI.MinimumDPI != minDPI || s.DPI.MaximumDPI != maxDPI || (s.Performance.LiftHeight != nil) != se || s.Performance.PollingRate != nil {
			t.Fatal("preview lost package-specific capabilities")
		}
		if s.LegacyLighting || lighting == nil || lighting.ConfiguredEffect != "mouse" || lighting.Brightness != 70 || len(lighting.SupportedEffects) != 20 || lighting.AuthoredZoneEditor == nil || len(lighting.AuthoredZoneEditor.Zones) != 3 {
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
		for id, zone := range ironclawDirectHIDPreviewZones() {
			if zone.name != names[id] || !reflect.DeepEqual(zone.indices, indices[id]) || lighting.AuthoredZoneEditor.Zones[id].Label != zone.name || lighting.AuthoredZoneEditor.Zones[id].ColorHex != zone.color {
				t.Fatalf("zone=%#v", zone)
			}
		}
		if lighting.ClusterControlled || lighting.ExternalControlled {
			t.Fatal("preview initialized external ownership")
		}
		// Every build owns its data; mutations cannot poison subsequent fixtures.
		lighting.AuthoredZoneEditor.Zones[0].Label = "changed"
		if buildIronclawReceiverModernPreview(se).Lighting.AuthoredZoneEditor.Zones[0].Label != "Logo" {
			t.Fatal("fixture aliases")
		}
	}
	if buildIronclawReceiverModernPreview(false).Serial == buildIronclawReceiverModernPreview(true).Serial || buildIronclawReceiverModernPreview(false).Product == buildIronclawReceiverModernPreview(true).Product {
		t.Fatal("package identities collapsed")
	}
}
