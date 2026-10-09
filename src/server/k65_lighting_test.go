package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices"
	"LumenForge/src/devices/k65rgb"
	"LumenForge/src/devices/k65rgbRF"
	"LumenForge/src/keyboards"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var _ nativeDeviceLightingTarget = (*k65rgb.Device)(nil)
var _ nativeDeviceLightingTarget = (*k65rgbRF.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*k65rgb.Device)(nil)
var _ nativeDeviceAuthoredZoneLightingMultiTarget = (*k65rgbRF.Device)(nil)

type k65CanonicalProvider struct {
	serial   string
	usable   bool
	snapshot lightingpresentation.Snapshot
}

func (p k65CanonicalProvider) LightingDeviceID() string { return p.serial }
func (p k65CanonicalProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return p.snapshot, p.usable
}
func TestK65CanonicalSuppressionRequiresUsableSnapshot(t *testing.T) {
	for _, instance := range []interface{}{&k65rgb.Device{Serial: "k65"}, &k65rgbRF.Device{Serial: "k65"}, k65CanonicalProvider{serial: "k65"}} {
		wrappers := map[string]*common.Device{"k65": {Serial: "k65", ProductType: common.ProductTypeK65Rgb, Instance: instance}}
		summary, ok := devicesWorkspaceSummaryForSerial(wrappers, map[string]stats.BatteryStats{}, "k65")
		if !ok || !summary.LegacyLighting || summary.Lighting != nil {
			t.Fatal("lost fallback", summary)
		}
	}
	instance := k65CanonicalProvider{serial: "k65", usable: true, snapshot: lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "keyboard", EffectSupported: true, EffectSelectionAvailable: true, HasBrightness: true, Brightness: 70, SupportedEffects: []lightingpresentation.EffectOption{{ID: "keyboard", Label: "Keyboard"}}}}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{"k65": {Serial: "k65", ProductType: common.ProductTypeK65Rgb, Instance: instance}}, map[string]stats.BatteryStats{}, "k65")
	if !ok || summary.LegacyLighting || summary.Lighting == nil {
		t.Fatal("suppression")
	}
}
func TestK65CanonicalPreviewsAreDistinctAndInert(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	keyboards.Init()
	catalogue := []string{"circle", "circleshift", "colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "keyboard", "off", "rainbow", "pastelrainbow", "rotator", "spinner", "static", "storm", "watercolor", "wave"}
	sort.Strings(catalogue)
	first := buildK65RGBModernPreview()
	second := buildK65RGBRapidfireModernPreview()
	if first.Serial == second.Serial || first.Product == second.Product {
		t.Fatal("identity")
	}
	for _, test := range []struct {
		key, layout string
		build       func() *devicesWorkspaceSummary
	}{{"k65-rgb-modern", "k65rgb-default-US", buildK65RGBModernPreview}, {"k65-rgb-rapidfire-modern", "k65rgbRF-default-US", buildK65RGBRapidfireModernPreview}} {
		source := keyboards.GetKeyboard(test.layout)
		before := source.Row[1].Keys[1].Color
		summary := test.build()
		l := summary.Lighting
		if summary.LegacyLighting || l == nil || l.ConfiguredEffect != "keyboard" || l.Brightness != 70 || len(l.SupportedEffects) != 23 || l.AuthoredZoneEditor == nil || len(l.AuthoredZoneEditor.Zones) != 92 || l.ClusterOwnershipAvailable || l.ExternalOwnershipAvailable || summary.KeyboardAssignments.LiveRGBAvailable {
			t.Fatalf("preview %#v", summary)
		}
		got := []string{}
		for _, e := range l.SupportedEffects {
			got = append(got, e.ID)
		}
		if !reflect.DeepEqual(got, catalogue) {
			t.Fatal(got)
		}
		z := l.AuthoredZoneEditor.Zones[0]
		if z.ID != "1" || z.Label != "BTS" || !z.HasGeometry || z.Left != 1200 || z.Top != 0 || z.Width != 100 || z.Height != 100 {
			t.Fatal(z)
		}
		l.AuthoredZoneEditor.Zones[0].Label = "changed"
		if test.build().Lighting.AuthoredZoneEditor.Zones[0].Label != "BTS" || source.Row[1].Keys[1].Color != before {
			t.Fatal("fixture alias")
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+test.key+"?view=lighting"))
		if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), "data-lf-authored-zone") || strings.Contains(recorder.Body.String(), "data-lf-keyboard-live-rgb") {
			t.Fatal("render", recorder.Code)
		}
		if devices.GetDevice(summary.Serial) != nil {
			t.Fatal("registered preview")
		}
	}
}
