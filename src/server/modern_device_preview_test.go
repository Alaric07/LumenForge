package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices"
	"LumenForge/src/devices/cduo"
	"LumenForge/src/devices/cpro"
	"LumenForge/src/keyboards"
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

func TestK95ModernDevicePreviewsRenderKeyboardPerformanceAndProfilesWithoutRegistration(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	keyboards.Init()
	for _, fixture := range []struct {
		key, serial, geometry, rowGeometry, polling, special string
		rows, keys                                           int
	}{
		{"k70-lux-modern", "preview-k70-lux-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", "BTS", 7, 113},
		{"k70-lux-rgb-modern", "preview-k70-lux-rgb-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", "BTS", 7, 113},
		{"k70-rgb-rf-modern", "preview-k70-rgb-rf-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", "BTS", 7, 113},
		{"k70-mk2-modern", "preview-k70-mk2-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", "BTS", 7, 116},
		{"k95-modern", "preview-k95-modern", "keyboard-7", "keyboard-row-27", "1000 Hz / 1 msec", "G1", 7, 135},
		{"k95-platinum-xt-modern", "preview-k95-platinum-xt-modern", "keyboard-8", "keyboard-row-26", "1000 Hz / 1 msec", "G1", 8, 139},
	} {
		if devices.GetDevice(fixture.serial) != nil {
			t.Fatalf("fixture serial %q unexpectedly registered", fixture.serial)
		}
		preview, ok := modernDevicePreviewFixtureByKey(fixture.key)
		if !ok {
			t.Fatalf("missing preview fixture %q", fixture.key)
		}
		summary := preview.Build()
		if summary.KeyboardAssignments == nil || summary.KeyboardAssignments.LayoutClass != fixture.geometry || summary.KeyboardAssignments.RowLayoutClass != fixture.rowGeometry || len(summary.KeyboardAssignments.Rows) != fixture.rows {
			t.Fatalf("%s keyboard summary = %#v", fixture.key, summary.KeyboardAssignments)
		}
		keyCount := 0
		keyNames := make(map[string]bool)
		for _, row := range summary.KeyboardAssignments.Rows {
			keyCount += len(row.Keys)
			for _, key := range row.Keys {
				keyNames[key.KeyName] = true
			}
		}
		if keyCount != fixture.keys || !keyNames["A"] || !keyNames[fixture.special] || !keyNames["Play"] {
			t.Fatalf("%s keyboard rows=%d keys=%d names=%#v", fixture.key, len(summary.KeyboardAssignments.Rows), keyCount, keyNames)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+fixture.key+"?view=keyboard"))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d: %s", fixture.key, recorder.Code, recorder.Body.String())
		}
		body := recorder.Body.String()
		for _, expected := range []string{"A", fixture.special, "Play", fixture.geometry, fixture.polling, "Disable Win Key", "perf_shiftTab", "perf_altTab", "perf_altF4"} {
			if !strings.Contains(body, expected) {
				t.Errorf("%s preview omitted %q", fixture.key, expected)
			}
		}
		recorder = httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+fixture.key))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Device Profile") || !strings.Contains(recorder.Body.String(), "Default") {
			t.Errorf("%s overview omitted device-profile state", fixture.key)
		}
		if devices.GetDevice(fixture.serial) != nil {
			t.Fatalf("fixture serial %q registered during preview", fixture.serial)
		}
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

func TestM75WirelessModernDevicePreviewRendersSharedMouseWorkspace(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	const serial = "preview-m75-wireless-modern"
	for _, test := range []struct{ query, want string }{{"", "78%"}, {"?view=lighting", "Native Lighting migration is not complete."}, {"?view=dpi", "Sniper"}, {"?view=buttons", "Right Forward"}} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/m75-wireless-modern"+test.query))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), test.want) {
			t.Errorf("preview %q status=%d missing %q", test.query, recorder.Code, test.want)
		}
		for _, directive := range []string{"script-src 'none'", "connect-src 'none'", "form-action 'none'", "base-uri 'none'"} {
			if !strings.Contains(recorder.Header().Get("Content-Security-Policy"), directive) {
				t.Errorf("preview CSP omitted %q", directive)
			}
		}
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/m75-wireless-modern"))
	for _, expected := range []string{"M75 WIRELESS", "Sleep Timer", "15 minutes", "Firmware 1.4.32", "FPS", `href="/dev/device-preview/m75-wireless-modern?view=buttons"`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Errorf("preview omitted %q", expected)
		}
	}
	if devices.GetDevice(serial) != nil {
		t.Fatalf("fixture serial %q was registered", serial)
	}
}

func TestWirelessMouseFamilyModernPreviewsRenderWithoutRegistration(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	for _, test := range []struct {
		key, serial, product, button                     string
		polling, buttonOptimization, angleSnapping, lift bool
	}{
		{"m75-air-wireless-modern", "preview-m75-air-wireless-modern", "M75 AIR WIRELESS", "Right Forward", true, true, true, true},
		{"m65-rgb-ultra-wireless-modern", "preview-m65-rgb-ultra-wireless-modern", "M65 RGB ULTRA WIRELESS", "Right Forward", true, true, true, true},
		{"harpoon-wireless-modern", "preview-harpoon-wireless-modern", "HARPOON WIRELESS", "Right Forward", false, true, true, false},
		{"m55-wireless-modern", "preview-m55-wireless-modern", "M55 WIRELESS", "Right Forward", false, true, true, false},
		{"nightsabre-wireless-modern", "preview-nightsabre-wireless-modern", "NIGHTSABRE WIRELESS", "Right Forward", true, true, true, true},
		{"sabre-rgb-pro-wireless-modern", "preview-sabre-rgb-pro-wireless-modern", "SABRE RGB PRO WIRELESS", "Right Forward", true, true, true, true},
		{"ironclaw-wireless-modern", "preview-ironclaw-wireless-modern", "IRONCLAW WIRELESS", "Right Forward", false, true, true, false},
		{"ironclaw-wireless-se-modern", "preview-ironclaw-wireless-se-modern", "IRONCLAW WIRELESS SE", "Right Forward", false, true, true, true},
		{"scimitar-rgb-elite-wireless-modern", "preview-scimitar-rgb-elite-wireless-modern", "SCIMITAR RGB ELITE WIRELESS", "Right Forward", false, true, true, true},
		{"scimitar-elite-wireless-se-modern", "preview-scimitar-elite-wireless-se-modern", "SCIMITAR ELITE WIRELESS SE", "Right Forward", false, true, true, true},
		{"dark-core-rgb-pro-se-wireless-modern", "preview-dark-core-rgb-pro-se-wireless-modern", "DARK CORE RGB PRO SE WIRELESS", "Right Forward", false, true, true, false},
		{"dark-core-rgb-pro-wireless-modern", "preview-dark-core-rgb-pro-wireless-modern", "DARK CORE RGB PRO WIRELESS", "Right Forward", false, true, true, false},
		{"dark-core-rgb-se-wireless-modern", "preview-dark-core-rgb-se-wireless-modern", "DARK CORE RGB SE WIRELESS", "Right Forward", false, false, true, true},
		{"sabre-v2-pro-wireless-modern", "preview-sabre-v2-pro-wireless-modern", "SABRE V2 PRO WIRELESS", "Right Forward", false, false, true, true},
	} {
		if devices.GetDevice(test.serial) != nil {
			t.Fatalf("fixture serial %q unexpectedly exists", test.serial)
		}
		for _, query := range []string{"", "?view=lighting", "?view=dpi", "?view=buttons"} {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+test.key+query))
			if recorder.Code != http.StatusOK {
				t.Errorf("preview %q%s status=%d", test.key, query, recorder.Code)
			}
			body := recorder.Body.String()
			wants := []string{test.product}
			switch query {
			case "":
				wants = append(wants, "78%", "Sleep Timer", "15 minutes")
			case "?view=lighting":
				wants = append(wants, "Native Lighting migration is not complete.")
			case "?view=dpi":
				wants = append(wants, "Sniper")
			case "?view=buttons":
				wants = append(wants, test.button)
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("preview %q%s omitted %q", test.key, query, want)
				}
			}
			if query == "?view=dpi" && (strings.Contains(body, `data-lf-performance-kind="pollingRate"`) != test.polling || strings.Contains(body, `data-lf-performance-kind="buttonOptimization"`) != test.buttonOptimization || strings.Contains(body, `data-lf-performance-kind="angleSnapping"`) != test.angleSnapping || strings.Contains(body, `data-lf-performance-kind="liftHeight"`) != test.lift) {
				t.Errorf("preview %q capability controls did not match polling=%t buttonOptimization=%t angleSnapping=%t lift=%t", test.key, test.polling, test.buttonOptimization, test.angleSnapping, test.lift)
			}
		}
		if devices.GetDevice(test.serial) != nil {
			t.Fatalf("fixture serial %q was registered", test.serial)
		}
	}
}

func TestGlaiveModernDevicePreviewRendersSharedMouseWorkspace(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	const serial = "preview-glaive-rgb-pro-modern"
	if devices.GetDevice(serial) != nil {
		t.Fatalf("fixture serial %q unexpectedly exists", serial)
	}
	for _, test := range []struct{ query, want string }{{"", "GLAIVE RGB PRO"}, {"?view=lighting", "Native Lighting migration is not complete."}, {"?view=dpi", "Angle Snapping"}, {"?view=buttons", "DPI Down"}} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/glaive-rgb-pro-modern"+test.query))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), test.want) {
			t.Errorf("preview %q status=%d missing %q", test.query, recorder.Code, test.want)
		}
		for _, directive := range []string{"script-src 'none'", "connect-src 'none'", "form-action 'none'", "base-uri 'none'"} {
			if !strings.Contains(recorder.Header().Get("Content-Security-Policy"), directive) {
				t.Errorf("preview CSP omitted %q", directive)
			}
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
