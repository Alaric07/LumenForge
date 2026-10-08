package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices/virtuosoSEW"
	"LumenForge/src/devices/virtuosoSEWU"
	"LumenForge/src/devices/virtuosoW"
	"LumenForge/src/devices/virtuosoWU"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var _ nativeDeviceLightingTarget = (*virtuosoWU.Device)(nil)
var _ nativeDeviceLightingTarget = (*virtuosoSEWU.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*virtuosoWU.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*virtuosoSEWU.Device)(nil)

type virtuosoCanonicalProvider struct {
	serial   string
	usable   bool
	snapshot lightingpresentation.Snapshot
}

func (p virtuosoCanonicalProvider) LightingDeviceID() string { return p.serial }
func (p virtuosoCanonicalProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return p.snapshot, p.usable
}

func TestVirtuosoDirectHIDCutoverRequiresUsableSnapshot(t *testing.T) {
	for _, productType := range []uint16{common.ProductTypeVirtuosoWU, common.ProductTypeVirtuosoSEWU} {
		for _, usable := range []bool{false, true} {
			serial := "virtuoso-canonical"
			provider := virtuosoCanonicalProvider{serial: serial, usable: usable, snapshot: lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "headset", EffectSupported: true, HasBrightness: true, Brightness: 70, SupportedEffects: []lightingpresentation.EffectOption{{ID: "headset", Label: "Headset"}}}}
			summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, ProductType: productType, Instance: provider}}, map[string]stats.BatteryStats{}, serial)
			if !ok || summary.LegacyLighting == usable || (summary.Lighting != nil) != usable {
				t.Fatalf("type=%d usable=%t summary=%#v", productType, usable, summary)
			}
			if usable && (summary.Lighting.ClusterOwnershipAvailable || summary.Lighting.ExternalOwnershipAvailable) {
				t.Fatal("unsupported ownership capabilities advertised")
			}
			provider.snapshot.TargetKind = ""
			summary, ok = devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, ProductType: productType, Instance: provider}}, map[string]stats.BatteryStats{}, serial)
			if !ok || !summary.LegacyLighting || summary.Lighting != nil {
				t.Fatal("unusable snapshot suppressed fallback")
			}
		}
	}
	// Real unattached packages fail closed and remain eligible for legacy Lighting.
	for _, instance := range []interface{}{&virtuosoWU.Device{Serial: "virtuoso-offline"}, &virtuosoSEWU.Device{Serial: "virtuoso-offline"}} {
		summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{"virtuoso-offline": {Serial: "virtuoso-offline", ProductType: common.ProductTypeVirtuosoWU, Instance: instance}}, map[string]stats.BatteryStats{}, "virtuoso-offline")
		if !ok || !summary.LegacyLighting || summary.Lighting != nil {
			t.Fatal("unattached real package lost fallback")
		}
	}
}

func TestVirtuosoDirectHIDResolverKeepsPackageIdentityAndGuards(t *testing.T) {
	original := lookupNativeDeviceLightingWrapper
	t.Cleanup(func() { lookupNativeDeviceLightingWrapper = original })
	for _, target := range []nativeDeviceLightingTarget{&virtuosoWU.Device{Serial: "ProUSB"}, &virtuosoSEWU.Device{Serial: "ProSEUSB"}} {
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
	for _, instance := range []interface{}{&virtuosoW.Device{}, &virtuosoSEW.Device{}} {
		if _, ok := instance.(devicesLightingSnapshotProvider); ok {
			t.Fatal("receiver package migrated without authorization")
		}
	}
}

func TestVirtuosoDirectHIDPreviewsAreInertExactDescriptors(t *testing.T) {
	want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "headset", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	sort.Strings(want)
	previews := []*devicesWorkspaceSummary{buildVirtuosoUSBModernPreview(), buildVirtuosoSEUSBModernPreview()}
	for _, s := range previews {
		l := s.Lighting
		if s.LegacyLighting || l == nil || l.ConfiguredEffect != "headset" || l.Brightness != 70 || len(l.SupportedEffects) != 20 || l.AuthoredZoneEditor == nil || len(l.AuthoredZoneEditor.Zones) != 3 || l.ClusterOwnershipAvailable || l.ExternalOwnershipAvailable || l.ClusterControlled || l.ExternalControlled {
			t.Fatalf("preview=%#v", s)
		}
		got := []string{}
		for _, e := range l.SupportedEffects {
			got = append(got, e.ID)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatal(got)
		}
		for id, z := range virtuosoUSBPreviewZones() {
			names := []string{"Logo", "Microphone", "Indicator LED"}
			indices := [][]int{{0, 3, 6}, {2, 5, 8}, {1, 4, 7}}
			if z.name != names[id] || !reflect.DeepEqual(z.indices, indices[id]) || l.AuthoredZoneEditor.Zones[id].Label != z.name || l.AuthoredZoneEditor.Zones[id].ColorHex != z.color {
				t.Fatal("zones")
			}
		}
		l.AuthoredZoneEditor.Zones[0].Label = "changed"
	}
	if previews[0].Serial == previews[1].Serial || previews[0].Product == previews[1].Product || buildVirtuosoUSBModernPreview().Lighting.AuthoredZoneEditor.Zones[0].Label != "Logo" || buildVirtuosoSEUSBModernPreview().Lighting.AuthoredZoneEditor.Zones[0].Label != "Logo" {
		t.Fatal("identity/alias")
	}
}

func TestVirtuosoUSBPreviewLightingRendersWithoutRegistration(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	for _, key := range []string{"virtuoso-usb-modern", "virtuoso-se-usb-modern"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+key+"?view=lighting"))
		if recorder.Code != http.StatusOK {
			t.Fatal(recorder.Code)
		}
		body := recorder.Body.String()
		for _, want := range []string{`data-lf-authored-zone-control`, `data-lf-effect="headset"`, "Microphone", "Indicator LED", "Logo"} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing %s", key, want)
			}
		}
		for _, bad := range []string{`data-lf-lighting-cluster`, `data-lf-lighting-openrgb`} {
			if strings.Contains(body, bad) {
				t.Fatal("fake ownership UI")
			}
		}
	}
}
