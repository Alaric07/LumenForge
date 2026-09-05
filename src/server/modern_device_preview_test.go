package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices"
	"LumenForge/src/devices/cduo"
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
			t.Errorf("preview %q status = %d, missing %q", view.query, recorder.Code, view.want)
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
