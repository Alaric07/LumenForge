package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices/scufenvisionproV2W"
	"LumenForge/src/devices/scufenvisionproV2WU"
	"LumenForge/src/devices/scufenvisionproW"
	"LumenForge/src/devices/scufenvisionproWU"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var _ nativeDeviceLightingTarget = (*scufenvisionproW.Device)(nil)
var _ nativeDeviceLightingTarget = (*scufenvisionproV2W.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*scufenvisionproW.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*scufenvisionproV2W.Device)(nil)

type scufCanonicalProvider struct {
	serial   string
	usable   bool
	snapshot lightingpresentation.Snapshot
}

func (p scufCanonicalProvider) LightingDeviceID() string { return p.serial }
func (p scufCanonicalProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return p.snapshot, p.usable
}

func TestSCUFDirectHIDCutoverRequiresUsableSnapshot(t *testing.T) {
	for _, productType := range []uint16{common.ProductTypeScufEnvisionProW, common.ProductTypeScufEnvisionProV2W, common.ProductTypeScufEnvisionProWU} {
		for _, usable := range []bool{false, true} {
			serial := "scuf-canonical"
			provider := scufCanonicalProvider{serial: serial, usable: usable, snapshot: lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "controller", EffectSupported: true, HasBrightness: true, Brightness: 70, SupportedEffects: []lightingpresentation.EffectOption{{ID: "controller", Label: "Controller"}}}}
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
	for _, instance := range []interface{}{&scufenvisionproW.Device{Serial: "scuf-offline"}, &scufenvisionproV2W.Device{Serial: "scuf-offline"}} {
		summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{"scuf-offline": {Serial: "scuf-offline", ProductType: common.ProductTypeScufEnvisionProW, Instance: instance}}, map[string]stats.BatteryStats{}, "scuf-offline")
		if !ok || !summary.LegacyLighting || summary.Lighting != nil {
			t.Fatal("unattached real package lost fallback")
		}
	}
}

func TestSCUFDirectHIDResolverKeepsPackageIdentityAndGuards(t *testing.T) {
	original := lookupNativeDeviceLightingWrapper
	t.Cleanup(func() { lookupNativeDeviceLightingWrapper = original })
	for _, target := range []nativeDeviceLightingTarget{&scufenvisionproW.Device{Serial: "ProUSB"}, &scufenvisionproV2W.Device{Serial: "ProSEUSB"}} {
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
	for _, instance := range []interface{}{&scufenvisionproWU.Device{}, &scufenvisionproV2WU.Device{}} {
		if _, ok := instance.(devicesLightingSnapshotProvider); ok {
			t.Fatal("receiver package migrated without authorization")
		}
	}
}

func TestSCUFDirectHIDPreviewsAreInertExactDescriptors(t *testing.T) {
	want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "controller", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	sort.Strings(want)
	previews := []*devicesWorkspaceSummary{buildSCUFEnvisionProModernPreview("SCUF ENVISION PRO", "preview-scuf-envision-pro-wireless-modern", false), buildSCUFEnvisionProModernPreview("SCUF ENVISION PRO V2", "preview-scuf-envision-pro-v2-wireless-modern", false)}
	for _, s := range previews {
		l := s.Lighting
		if s.LegacyLighting || l == nil || l.ConfiguredEffect != "controller" || l.Brightness != 70 || len(l.SupportedEffects) != 20 || l.AuthoredZoneEditor == nil || len(l.AuthoredZoneEditor.Zones) != 1 || l.ClusterOwnershipAvailable || l.ExternalOwnershipAvailable || l.ClusterControlled || l.ExternalControlled {
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
		if l.AuthoredZoneEditor.Zones[0].Label != "Controller" || l.AuthoredZoneEditor.Zones[0].ColorHex != "#00ffff" {
			t.Fatal("zones")
		}

		l.AuthoredZoneEditor.Zones[0].Label = "changed"
	}
	if previews[0].Serial == previews[1].Serial || previews[0].Product == previews[1].Product || buildSCUFEnvisionProModernPreview("SCUF ENVISION PRO", "preview-scuf-envision-pro-wireless-modern", false).Lighting.AuthoredZoneEditor.Zones[0].Label != "Controller" || buildSCUFEnvisionProModernPreview("SCUF ENVISION PRO V2", "preview-scuf-envision-pro-v2-wireless-modern", false).Lighting.AuthoredZoneEditor.Zones[0].Label != "Controller" {
		t.Fatal("identity/alias")
	}
}

func TestSCUFUSBPreviewLightingRendersWithoutRegistration(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	for _, key := range []string{"scuf-envision-pro-wireless-modern", "scuf-envision-pro-v2-wireless-modern"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+key+"?view=lighting"))
		if recorder.Code != http.StatusOK {
			t.Fatal(recorder.Code)
		}
		body := recorder.Body.String()
		for _, want := range []string{`data-lf-authored-zone-control`, `data-lf-effect="controller"`, "Controller"} {
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
