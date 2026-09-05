package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices"
	"LumenForge/src/devices/cduo"
	"LumenForge/src/devices/cpro"
	"LumenForge/src/server/requests"
	"LumenForge/src/stats"
	"LumenForge/src/temperatures"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCommanderDuoWorkspaceSummaryUsesModernCoolingAndProfiles(t *testing.T) {
	initializeLegacyDevicePreviewTestProcess(t)
	temperatures.Init()
	const serial = "duo-modern-workspace"
	instance := &cduo.Device{
		Serial: serial,
		Devices: map[int]*cduo.Devices{
			2: {ChannelId: 2, Name: "Fan Channel 3", Label: "Pump", Rpm: 2200, Profile: "Performance", HasSpeed: true, ContainsPump: true},
			4: {ChannelId: 4, Name: "Temperature Probe 1", Label: "Coolant", TemperatureString: "30.0°C", IsTemperatureProbe: true, HasTemps: true},
		},
		UserProfiles: map[string]*cduo.DeviceProfile{"Default": {Active: true}, "Quiet": {}},
	}
	device := &common.Device{Serial: serial, Product: "iCUE COMMANDER DUO", ProductType: common.ProductTypeCCXT, Instance: instance}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: device}, map[string]stats.BatteryStats{}, serial)
	if !ok || summary.Cooling == nil || summary.DeviceProfiles == nil || !summary.LegacyLighting {
		t.Fatalf("summary = %#v, ok=%t", summary, ok)
	}
	if channel := summary.Cooling.Channels[0]; channel.ID != 2 || channel.RPM != 2200 || !channel.ContainsPump || channel.SelectedProfile != "Performance" {
		t.Fatalf("cooling channel = %#v", channel)
	}
	if summary.DeviceProfiles.ActiveProfile != "Default" || summary.DeviceProfiles.Description != devicesCCXTDeviceProfileDescription {
		t.Fatalf("profiles = %#v", summary.DeviceProfiles)
	}
}

func TestCommanderProWorkspaceSummaryUsesModernCoolingProfilesAndTelemetry(t *testing.T) {
	initializeLegacyDevicePreviewTestProcess(t)
	temperatures.Init()
	const serial = "cpro-modern-workspace"
	instance := &cpro.Device{Serial: serial, Devices: map[int]*cpro.Devices{0: {ChannelId: 0, Name: "Fan 1", Label: "Front", Rpm: 980, Profile: "Quiet", HasSpeed: true}}, UserProfiles: map[string]*cpro.DeviceProfile{"Default": {Active: true}}, RailVoltages: map[int]*cpro.RailVoltage{0: {Name: "+12V", Value: 12.08}, 1: {Name: "+5V", Value: 5.02}, 2: {Name: "+3.3V", Value: 3.31}}}
	device := &common.Device{Serial: serial, Product: "Commander Pro", ProductType: common.ProductTypeCPro, Instance: instance}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: device}, map[string]stats.BatteryStats{}, serial)
	if !ok || summary.Cooling == nil || summary.DeviceProfiles == nil || !summary.LegacyLighting || len(summary.OverviewTelemetry) != 3 {
		t.Fatalf("summary = %#v, ok=%t", summary, ok)
	}
	if summary.OverviewTelemetry[0].Label != "+12V" || summary.OverviewTelemetry[0].Value != "12.08 V" {
		t.Fatalf("telemetry = %#v", summary.OverviewTelemetry)
	}
}

func TestModernDevicePreviewDebugGating(t *testing.T) {
	router := legacyDevicePreviewRouter(t, false)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/commander-duo-modern"))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("debug-disabled modern preview status = %d, want 404", recorder.Code)
	}
}

func TestCommanderDuoModernDevicePreviewRendersFixtureWithoutRegistration(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	if devices.GetDevice(commanderDuoModernPreviewSerial) != nil {
		t.Fatalf("fixture serial %q unexpectedly exists before preview", commanderDuoModernPreviewSerial)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/commander-duo-modern?view=cooling"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("modern preview status = %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, expected := range []string{"iCUE COMMANDER DUO", "Front intake", "Radiator pump", "2240 RPM", "Coolant", "Case exhaust", "Preview Mode — Hardware actions and scripts are disabled", "data-lf-cooling-workspace", `href="/dev/device-preview/commander-duo-modern"`, `href="/dev/device-preview/commander-duo-modern?view=lighting"`, `href="/dev/device-preview/commander-duo-modern?view=cooling"`, `href="/dev/device-preview"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("preview omitted %q", expected)
		}
	}
	for _, directive := range []string{"script-src 'none'", "connect-src 'none'", "form-action 'none'", "base-uri 'none'"} {
		if !strings.Contains(recorder.Header().Get("Content-Security-Policy"), directive) {
			t.Errorf("preview CSP omitted %q", directive)
		}
	}
	for _, view := range []struct {
		query string
		want  string
	}{
		{query: "", want: "Studio"},
		{query: "?view=lighting", want: "Native Lighting migration is not complete."},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/commander-duo-modern"+view.query))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), view.want) {
			t.Errorf("preview %q status = %d, missing %q: %s", view.query, recorder.Code, view.want, recorder.Body.String())
		}
	}
	if devices.GetDevice(commanderDuoModernPreviewSerial) != nil {
		t.Fatalf("fixture serial %q was registered by preview rendering", commanderDuoModernPreviewSerial)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/speed", strings.NewReader(`{"deviceId":"`+commanderDuoModernPreviewSerial+`","channelId":0,"profile":"Quiet"}`))
	if response := requests.ProcessChangeSpeed(request); response.Status != 0 {
		t.Fatalf("fixture serial mutation response = %#v, want failed dispatch", response)
	}
}

func TestCommanderProModernDevicePreviewRendersFixtureWithoutRegistration(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	const serial = "preview-commander-pro-modern"
	for _, view := range []struct{ query, want string }{{"", "12.08 V"}, {"?view=cooling", "Radiator fan"}, {"?view=lighting", "Native Lighting migration is not complete."}} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/commander-pro-modern"+view.query))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), view.want) {
			t.Errorf("preview %q status = %d, missing %q", view.query, recorder.Code, view.want)
		}
	}
	if devices.GetDevice(serial) != nil {
		t.Fatalf("fixture serial %q was registered", serial)
	}
}

func TestHarpoonModernDevicePreviewRendersSharedMouseWorkspace(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	const serial = "preview-harpoon-rgb-pro-modern"
	if devices.GetDevice(serial) != nil {
		t.Fatalf("fixture serial %q unexpectedly exists", serial)
	}
	for _, test := range []struct{ query, want string }{
		{"", "1000 Hz / 1 msec"},
		{"?view=lighting", "Native Lighting migration is not complete."},
		{"?view=dpi", "Stage 2"},
		{"?view=buttons", "Forward Button"},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/harpoon-rgb-pro-modern"+test.query))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), test.want) {
			t.Errorf("preview %q status = %d, missing %q", test.query, recorder.Code, test.want)
		}
		for _, directive := range []string{"script-src 'none'", "connect-src 'none'", "form-action 'none'"} {
			if !strings.Contains(recorder.Header().Get("Content-Security-Policy"), directive) {
				t.Errorf("preview CSP omitted %q", directive)
			}
		}
	}
	for _, expected := range []string{"href=\"/dev/device-preview/harpoon-rgb-pro-modern\"", "href=\"/dev/device-preview/harpoon-rgb-pro-modern?view=lighting\"", "href=\"/dev/device-preview/harpoon-rgb-pro-modern?view=dpi\"", "href=\"/dev/device-preview/harpoon-rgb-pro-modern?view=buttons\""} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/harpoon-rgb-pro-modern"))
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Errorf("preview navigation omitted %q", expected)
		}
	}
	if devices.GetDevice(serial) != nil {
		t.Fatalf("fixture serial %q was registered", serial)
	}
}

func TestKatarModernDevicePreviewsRenderSharedMouseWorkspace(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	for _, fixture := range []struct{ key, serial, product, stage string }{
		{"katar-pro-modern", "preview-katar-pro-modern", "KATAR PRO", "Stage 2"},
		{"katar-pro-xt-modern", "preview-katar-pro-xt-modern", "KATAR PRO XT", "Stage 2"},
	} {
		if devices.GetDevice(fixture.serial) != nil {
			t.Fatalf("fixture serial %q unexpectedly exists", fixture.serial)
		}
		for _, test := range []struct{ query, want string }{{"", fixture.product}, {"?view=lighting", "Native Lighting migration is not complete."}, {"?view=dpi", "Button Optimization"}, {"?view=buttons", "DPI Button"}} {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+fixture.key+test.query))
			if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), test.want) {
				t.Errorf("%s %q status=%d missing %q", fixture.key, test.query, recorder.Code, test.want)
			}
			for _, directive := range []string{"script-src 'none'", "connect-src 'none'", "form-action 'none'", "base-uri 'none'"} {
				if !strings.Contains(recorder.Header().Get("Content-Security-Policy"), directive) {
					t.Errorf("%s CSP omitted %q", fixture.key, directive)
				}
			}
		}
		if devices.GetDevice(fixture.serial) != nil {
			t.Fatalf("fixture serial %q was registered", fixture.serial)
		}
	}
}
