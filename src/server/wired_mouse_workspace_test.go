package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"LumenForge/src/common"
	"LumenForge/src/devices"
)

func TestWiredMouseModernPreviewsRenderWithoutRegistration(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	glaive := buildGlaiveRGBModernPreview()
	if glaive.Buttons == nil || len(glaive.Buttons.Buttons) != 6 || glaive.Buttons.Buttons[5].Name != "DPI Toggle" {
		t.Fatalf("Glaive preview buttons = %#v", glaive.Buttons)
	}
	for _, fixture := range []struct{ key, serial, dpiWant, buttonWant string }{
		{"glaive-rgb-modern", "preview-glaive-rgb-modern", "Lift Height", "DPI Toggle"},
		{"m65-rgb-elite-modern", "preview-m65-rgb-elite-modern", "Button Optimization", "DPI +"},
		{"sabre-rgb-pro-modern", "preview-sabre-rgb-pro-modern", "8000 Hz / 0.125 msec", "DPI"},
		{"nightsword-rgb-modern", "preview-nightsword-rgb-modern", "1000 Hz / 1 msec", "Profile Down"},
		{"ironclaw-rgb-modern", "preview-ironclaw-rgb-modern", "Angle Snapping", "Profile Button"},
	} {
		if devices.GetDevice(fixture.serial) != nil {
			t.Fatalf("fixture %q unexpectedly registered", fixture.serial)
		}
		for _, view := range []struct{ query, want string }{{"", "Preview Mode"}, {"?view=lighting", "Native Lighting migration is not complete."}, {"?view=dpi", fixture.dpiWant}, {"?view=buttons", fixture.buttonWant}} {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+fixture.key+view.query))
			if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), view.want) {
				t.Errorf("%s %s status=%d missing %q", fixture.key, view.query, recorder.Code, view.want)
			}
			if fixture.key == "glaive-rgb-modern" && view.query == "?view=buttons" && strings.Contains(recorder.Body.String(), "DPI Down") {
				t.Errorf("%s exposed inherited DPI Down button", fixture.key)
			}
			for _, directive := range []string{"script-src 'none'", "connect-src 'none'", "form-action 'none'", "base-uri 'none'"} {
				if !strings.Contains(recorder.Header().Get("Content-Security-Policy"), directive) {
					t.Errorf("%s CSP missing %q", fixture.key, directive)
				}
			}
		}
		if devices.GetDevice(fixture.serial) != nil {
			t.Fatalf("fixture %q registered by rendering", fixture.serial)
		}
	}
}

func TestNightswordAndIronclawPreviewShapes(t *testing.T) {
	nightsword := buildNightswordRGBModernPreview()
	if nightsword.Buttons == nil || len(nightsword.Buttons.Buttons) != 10 || nightsword.Performance == nil || nightsword.Performance.AngleSnapping != nil {
		t.Fatalf("nightsword = %#v", nightsword)
	}
	ironclaw := buildIronclawRGBModernPreview()
	if ironclaw.Buttons == nil || len(ironclaw.Buttons.Buttons) != 7 || ironclaw.Performance == nil || ironclaw.Performance.AngleSnapping == nil || ironclaw.Performance.ButtonOptimization != nil || ironclaw.Performance.LiftHeight != nil {
		t.Fatalf("ironclaw = %#v", ironclaw)
	}
}

func TestRemainingWiredMousePreviewShapes(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		build                           func() *devicesWorkspaceSummary
		buttons                         int
		angle, buttonOptimization, lift bool
	}{
		{"m55", buildM55ModernPreview, 6, true, true, false},
		{"m55-rgb-pro", buildM55RGBProModernPreview, 8, false, false, false},
		{"m65-pro-rgb", buildM65ProRGBModernPreview, 8, true, false, true},
		{"m65-rgb-ultra", buildM65RGBUltraModernPreview, 12, true, true, true},
		{"m75", buildM75ModernPreview, 8, true, true, true},
		{"sabre-pro-cs", buildSabreProCSModernPreview, 6, true, true, false},
		{"scimitar-rgb", buildScimitarRGBModernPreview, 17, true, false, true},
		{"scimitar-elite", buildScimitarEliteModernPreview, 17, true, false, false},
	} {
		s := tc.build()
		if s.Buttons == nil || len(s.Buttons.Buttons) != tc.buttons || s.Performance == nil || (s.Performance.AngleSnapping != nil) != tc.angle || (s.Performance.ButtonOptimization != nil) != tc.buttonOptimization || (s.Performance.LiftHeight != nil) != tc.lift || !s.LegacyLighting {
			t.Fatalf("%s = %#v", tc.name, s)
		}
	}
}

func TestWiredMouseProductTypesUseLegacyLighting(t *testing.T) {
	for _, productType := range []uint16{common.ProductTypeGlaiveRgb, common.ProductTypeM65RgbElite, common.ProductTypeSabreRgbPro, common.ProductTypeNightswordRgb, common.ProductTypeIronClawRgb, common.ProductTypeM55, common.ProductTypeM55RgbPro, common.ProductTypeM65RgbUltra, common.ProductTypeM75, common.ProductTypeSabreProCs, common.ProductTypeScimitarRgb} {
		serial := "wired-mouse"
		summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, ProductType: productType}}, nil, serial)
		if !ok || !summary.LegacyLighting {
			t.Fatalf("productType=%d summary=%#v ok=%t", productType, summary, ok)
		}
	}
}
