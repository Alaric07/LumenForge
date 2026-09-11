package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices"
	"LumenForge/src/devices/cduo"
	"LumenForge/src/devices/cpro"
	"LumenForge/src/devices/scufenvisionproV2W"
	"LumenForge/src/devices/scufenvisionproV2WU"
	"LumenForge/src/devices/virtuosoSEW"
	"LumenForge/src/devices/virtuosoSEWU"
	"LumenForge/src/devices/virtuosoW"
	"LumenForge/src/devices/virtuosoWU"
	"LumenForge/src/devices/virtuosorgbXTW"
	"LumenForge/src/devices/virtuosorgbXTWU"
	"LumenForge/src/inputmanager"
	"LumenForge/src/keyboards"
	"LumenForge/src/server/requests"
	"LumenForge/src/stats"
	"LumenForge/src/temperatures"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestSCUFEnvisionProModernPreviewsPreserveCapabilitySplit(t *testing.T) {
	for _, test := range []struct {
		key     string
		product uint16
		sleep   bool
	}{{"scuf-envision-pro-wireless-modern", common.ProductTypeScufEnvisionProW, false}, {"scuf-envision-pro-usb-modern", common.ProductTypeScufEnvisionProWU, true}, {"scuf-envision-pro-v2-wireless-modern", common.ProductTypeScufEnvisionProV2W, false}, {"scuf-envision-pro-v2-usb-modern", common.ProductTypeScufEnvisionProV2WU, true}} {
		fixture, ok := modernDevicePreviewFixtureByKey(test.key)
		if !ok || fixture.ProductType != test.product {
			t.Fatalf("fixture %q = %#v", test.key, fixture)
		}
		summary := fixture.Build()
		if summary == nil || summary.Controller == nil || !summary.LegacyLighting || !summary.HasBattery || (summary.SleepTimer != nil) != test.sleep {
			t.Fatalf("%s summary=%#v", test.key, summary)
		}
		if len(summary.Controller.AssignmentTypes) != 8 || len(summary.Controller.Assignments) != 8 || len(summary.Controller.Analogs) != 4 || summary.Controller.Assignments[2].Index != 2048 || summary.Controller.Assignments[3].Index != 4096 {
			t.Fatalf("%s controller=%#v", test.key, summary.Controller)
		}
	}
}

func TestSCUFEnvisionProV2DongleIsNotAModernControllerPreview(t *testing.T) {
	for _, fixture := range modernDevicePreviewFixtures {
		if fixture.ProductType == common.ProductTypeScufDongleV2 {
			t.Fatalf("V2 dongle fixture unexpectedly registered: %#v", fixture)
		}
	}
}

func TestSCUFEnvisionProV2WorkspaceRoutingUsesDistinctProductTypes(t *testing.T) {
	for _, test := range []struct {
		serial  string
		product uint16
		device  interface{}
		sleep   bool
	}{
		{"scuf-v2-wireless-routing", common.ProductTypeScufEnvisionProV2W, v2WirelessWorkspaceDevice(), false},
		{"scuf-v2-usb-routing", common.ProductTypeScufEnvisionProV2WU, v2USBWorkspaceDevice(), true},
	} {
		summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{test.serial: {Serial: test.serial, Product: "SCUF ENVISION PRO V2", ProductType: test.product, Instance: test.device}}, map[string]stats.BatteryStats{test.serial: {Level: 78}}, test.serial)
		if !ok || summary.Controller == nil || summary.DeviceProfiles == nil || !summary.LegacyLighting || !summary.HasBattery || (summary.SleepTimer != nil) != test.sleep {
			t.Fatalf("%d summary = %#v, ok=%t", test.product, summary, ok)
		}
		if len(summary.Controller.AssignmentTypes) != 8 || len(summary.Controller.Assignments) != 8 || summary.Controller.Assignments[6].Index != 2048 || summary.Controller.Assignments[7].Index != 4096 || len(summary.Controller.Analogs) != 4 {
			t.Fatalf("%d controller = %#v", test.product, summary.Controller)
		}
	}
}

func v2WorkspaceAssignments() map[int]inputmanager.KeyAssignment {
	return map[int]inputmanager.KeyAssignment{1: {Name: "Left Button", ActionType: 0}, 2: {Name: "DPAD Up", ActionType: 4, ActionCommand: 141}, 4: {Name: "DPAD Down", ActionType: 4, ActionCommand: 142}, 8: {Name: "DPAD Left", ActionType: 4, ActionCommand: 143}, 16: {Name: "DPAD Right", ActionType: 4, ActionCommand: 144}, 32: {Name: "A", ActionType: 4, ActionCommand: 128}, 2048: {Name: "LT", ActionType: 4, ActionCommand: 134}, 4096: {Name: "RT", ActionType: 4, ActionCommand: 135}}
}

func v2WorkspaceAssignmentTypes() map[int]string {
	return map[int]string{0: "None", 1: "Media Keys", 2: "DPI", 3: "Keyboard", 4: "Controller", 8: "Sniper", 9: "Mouse", 10: "Macro"}
}

func v2WirelessWorkspaceDevice() *scufenvisionproV2W.Device {
	points := map[int]common.CurveData{0: {X: 0, Y: 0}, 1: {X: 20, Y: 20}, 2: {X: 40, Y: 40}, 3: {X: 60, Y: 60}, 4: {X: 80, Y: 80}, 5: {X: 100, Y: 100}}
	analogs := map[int]scufenvisionproV2W.AnalogData{}
	for id := 0; id < 4; id++ {
		analogs[id] = scufenvisionproV2W.AnalogData{DeadZoneMin: 5, DeadZoneMax: 5, Points: points}
	}
	profile := &scufenvisionproV2W.DeviceProfile{Active: true, LeftVibrationValue: 60, RightVibrationValue: 50, LeftThumbStickMode: 1, LeftThumbStickSensitivityX: 20, LeftThumbStickSensitivityY: 20, RightThumbStickMode: 2, RightThumbStickSensitivityX: 30, RightThumbStickSensitivityY: 30, AnalogData: analogs}
	return &scufenvisionproV2W.Device{Serial: "scuf-v2-wireless-routing", DeviceProfile: profile, UserProfiles: map[string]*scufenvisionproV2W.DeviceProfile{"Default": profile}, KeyAssignment: v2WorkspaceAssignments(), KeyAssignmentTypes: v2WorkspaceAssignmentTypes(), ThumbStickModes: map[int]string{0: "None", 1: "Mouse", 2: "Thumbstick"}}
}

func v2USBWorkspaceDevice() *scufenvisionproV2WU.Device {
	points := map[int]common.CurveData{0: {X: 0, Y: 0}, 1: {X: 20, Y: 20}, 2: {X: 40, Y: 40}, 3: {X: 60, Y: 60}, 4: {X: 80, Y: 80}, 5: {X: 100, Y: 100}}
	analogs := map[int]scufenvisionproV2WU.AnalogData{}
	for id := 0; id < 4; id++ {
		analogs[id] = scufenvisionproV2WU.AnalogData{DeadZoneMin: 5, DeadZoneMax: 5, Points: points}
	}
	profile := &scufenvisionproV2WU.DeviceProfile{Active: true, SleepMode: 15, LeftVibrationValue: 60, RightVibrationValue: 50, LeftThumbStickMode: 1, LeftThumbStickSensitivityX: 20, LeftThumbStickSensitivityY: 20, RightThumbStickMode: 2, RightThumbStickSensitivityX: 30, RightThumbStickSensitivityY: 30, AnalogData: analogs}
	return &scufenvisionproV2WU.Device{Serial: "scuf-v2-usb-routing", Usb: true, DeviceProfile: profile, UserProfiles: map[string]*scufenvisionproV2WU.DeviceProfile{"Default": profile}, KeyAssignment: v2WorkspaceAssignments(), KeyAssignmentTypes: v2WorkspaceAssignmentTypes(), ThumbStickModes: map[int]string{0: "None", 1: "Mouse", 2: "Thumbstick"}, SleepModes: map[int]string{0: "Never", 1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}}
}

func TestSCUFEnvisionProAnalogPreviewRendersIndexedNativePointControls(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/scuf-envision-pro-usb-modern?view=analog"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, expected := range []string{"lf-controller-curve-figure", "Response Curve", "aria-label=\"Curve scale from 0 to 100\"", "data-lf-controller-point", "data-lf-point-index=\"0\"", "data-lf-point-index=\"5\"", "Point 0 X", "Point 5 Y", "data-lf-controller-deadzone-min", "data-lf-controller-deadzone-max", "Save Left Thumbstick", "Save Right Thumbstick", "Save Left Trigger", "Save Right Trigger"} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q", expected)
		}
	}
	for index := 0; index < 6; index++ {
		if count := strings.Count(body, `data-lf-point-index="`+strconv.Itoa(index)+`"`); count != 4 {
			t.Errorf("point %d rendered %d times, want one per analog device", index, count)
		}
	}
	if strings.Index(body, `data-lf-point-index="0"`) > strings.Index(body, `data-lf-point-index="5"`) {
		t.Fatal("point controls are not rendered in deterministic index order")
	}
	if count := strings.Count(body, `data-lf-controller-analog data-lf-analog-device`); count != 4 {
		t.Fatalf("analog editors=%d, want 4", count)
	}
	if count := strings.Count(body, "Response Curve"); count != 4 {
		t.Errorf("response curve headings=%d, want 4", count)
	}
	if count := strings.Count(body, `data-lf-controller-point-x`); count != 24 {
		t.Errorf("X controls=%d, want 24", count)
	}
	if count := strings.Count(body, `data-lf-controller-point-y`); count != 24 {
		t.Errorf("Y controls=%d, want 24", count)
	}
	if !strings.Contains(body, `data-lf-analog-device="0" open`) || strings.Contains(body, `data-lf-analog-device="1" open`) || strings.Contains(body, `data-lf-analog-device="2" open`) || strings.Contains(body, `data-lf-analog-device="3" open`) {
		t.Error("analog disclosure defaults are incorrect")
	}
}

func TestSCUFEnvisionProControllerAndAssignmentsPreviewsRetainIndependentControls(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	for _, test := range []struct {
		view     string
		expected []string
	}{
		{"controller", []string{"data-lf-module=\"0\"", "data-lf-module=\"1\"", "Save Left Thumbstick", "Save Right Thumbstick"}},
		{"assignments", []string{"data-lf-key-index=\"2048\"", "data-lf-key-index=\"4096\"", "data-lf-controller-assignment-save"}},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/scuf-envision-pro-usb-modern?view="+test.view))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status=%d: %s", test.view, recorder.Code, recorder.Body.String())
		}
		for _, expected := range test.expected {
			if !strings.Contains(recorder.Body.String(), expected) {
				t.Errorf("%s missing %q", test.view, expected)
			}
		}
	}
}

func TestHS80ModernPreviewsMatchSourceBackedMuteIndicatorCapabilities(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	for _, test := range []struct {
		key           string
		muteIndicator bool
	}{
		{"hs80-rgb-modern", false},
		{"hs80-rgb-wireless-modern", true},
		{"hs80-rgb-wireless-usb-modern", true},
	} {
		fixture, ok := modernDevicePreviewFixtureByKey(test.key)
		if !ok {
			t.Fatalf("missing preview fixture %q", test.key)
		}
		summary := fixture.Build()
		if summary.Headset == nil || (summary.Headset.MuteIndicator != nil) != test.muteIndicator || len(summary.Headset.Equalizer) != 10 {
			t.Fatalf("%s headset=%#v", test.key, summary.Headset)
		}
		if summary.Headset.Equalizer[0].Label != "32" || summary.Headset.Equalizer[9].Label != "16K" {
			t.Fatalf("%s equalizer=%#v", test.key, summary.Headset.Equalizer)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+test.key))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status=%d: %s", test.key, recorder.Code, recorder.Body.String())
		}
		body := recorder.Body.String()
		if strings.Contains(body, "Mute Indicator") != test.muteIndicator {
			t.Fatalf("%s mute indicator rendering mismatch", test.key)
		}
		firstBand, lastBand := strings.Index(body, `aria-label="32 equalizer"`), strings.Index(body, `aria-label="16K equalizer"`)
		if !strings.Contains(body, `type="range" min="-12" max="12"`) || firstBand < 0 || lastBand < 0 || firstBand > lastBand {
			t.Fatalf("%s equalizer rendering is not ordered sliders", test.key)
		}
	}
}

func TestHS80MAXWirelessModernPreviewRendersOnlySourceBackedCapabilities(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	fixture, ok := modernDevicePreviewFixtureByKey("hs80-max-wireless-modern")
	if !ok {
		t.Fatal("missing HS80 MAX Wireless preview fixture")
	}
	summary := fixture.Build()
	if summary.DeviceProfiles == nil || summary.Headset == nil || summary.Headset.MuteIndicator == nil || summary.Headset.Sidetone == nil || len(summary.Headset.Assignments) != 1 || summary.Headset.Assignments[0].Label != "Scroll Press" || !summary.HasBattery || summary.SleepTimer == nil || !summary.LegacyLighting {
		t.Fatalf("summary=%#v", summary)
	}
	if got := summary.Headset.Assignments[0].Types; len(got) != 6 || got[0].Label != "None" || got[5].Label != "Profile Switch" {
		t.Fatalf("assignment types=%#v", got)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/hs80-max-wireless-modern"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, expected := range []string{"Sidetone", "Scroll Press", "Mute Indicator", "Battery", "Sleep Timer", "data-lf-headset-sidetone-value", `min="0" max="100"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q", expected)
		}
	}
	if strings.Contains(body, "Active Noise Cancellation") {
		t.Error("preview exposed unsupported ANC")
	}
	lighting := httptest.NewRecorder()
	router.ServeHTTP(lighting, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/hs80-max-wireless-modern?view=lighting"))
	if lighting.Code != http.StatusOK || !strings.Contains(lighting.Body.String(), "Native Lighting migration is not complete.") {
		t.Fatalf("lighting=%d: %s", lighting.Code, lighting.Body.String())
	}
}

func TestVirtuosoMAXWirelessModernPreviewRendersOnlySourceBackedCapabilities(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	fixture, ok := modernDevicePreviewFixtureByKey("virtuoso-max-wireless-modern")
	if !ok || fixture.ProductType != common.ProductTypeVirtuosoMAXW {
		t.Fatalf("missing or misrouted Virtuoso MAX preview: %#v", fixture)
	}
	summary := fixture.Build()
	if summary == nil || summary.DeviceProfiles == nil || !summary.DeviceProfiles.CanSwitch || !summary.DeviceProfiles.CanSave || !summary.DeviceProfiles.CanDelete || summary.Headset == nil || summary.Headset.MuteIndicator == nil || summary.Headset.NoiseCancellation == nil || summary.Headset.Sidetone == nil || len(summary.Headset.Wheels) != 2 || len(summary.Headset.Assignments) != 0 || !summary.HasBattery || summary.SleepTimer == nil || !summary.LegacyLighting {
		t.Fatalf("summary=%#v", summary)
	}
	if summary.Headset.Sidetone.ValueRange.Minimum != 1 || summary.Headset.Sidetone.ValueRange.Maximum != 100 || summary.Headset.Wheels[0].ID != 1 || summary.Headset.Wheels[1].ID != 2 {
		t.Fatalf("headset=%#v", summary.Headset)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/virtuoso-max-wireless-modern"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, expected := range []string{"Active Noise Cancellation", "Sidetone", "Left Wheel", "Right Wheel", "Mute Indicator", "Battery", "Sleep Timer", `min="1" max="100"`, "System Volume", "Bluetooth Volume"} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q", expected)
		}
	}
	if strings.Contains(body, "Button Assignment") {
		t.Error("preview exposed unsupported button assignments")
	}
}

func TestVirtuosoModernPreviewsRenderOnlySourceBackedCapabilities(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	for _, test := range []struct {
		key, serial string
		productType uint16
		sleep       bool
	}{
		{"virtuoso-wireless-modern", "preview-virtuoso-wireless-modern", common.ProductTypeVirtuosoW, true},
		{"virtuoso-usb-modern", "preview-virtuoso-usb-modern", common.ProductTypeVirtuosoWU, false},
		{"virtuoso-se-wireless-modern", "preview-virtuoso-se-wireless-modern", common.ProductTypeVirtuosoSEW, true},
		{"virtuoso-se-usb-modern", "preview-virtuoso-se-usb-modern", common.ProductTypeVirtuosoSEWU, false},
	} {
		fixture, ok := modernDevicePreviewFixtureByKey(test.key)
		if !ok || fixture.ProductType != test.productType {
			t.Fatalf("missing or misrouted fixture %q: %#v", test.key, fixture)
		}
		if devices.GetDevice(test.serial) != nil {
			t.Fatalf("fixture %q registered a device", test.key)
		}
		summary := fixture.Build()
		if summary == nil || summary.DeviceProfiles == nil || !summary.DeviceProfiles.CanSwitch || !summary.DeviceProfiles.CanSave || !summary.DeviceProfiles.CanDelete || summary.Headset == nil || summary.Headset.MuteIndicator == nil || len(summary.Headset.Equalizer) != 10 || !summary.HasBattery || !summary.LegacyLighting || (summary.SleepTimer != nil) != test.sleep || summary.Headset.Sidetone != nil || len(summary.Headset.Assignments) != 0 {
			t.Fatalf("%s summary=%#v", test.key, summary)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+test.key))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status=%d: %s", test.key, recorder.Code, recorder.Body.String())
		}
		body := recorder.Body.String()
		if strings.Contains(body, "Sleep Timer") != test.sleep || strings.Contains(body, "Sidetone") || strings.Contains(body, "Active Noise Cancellation") || strings.Contains(body, "Scroll Press") || !strings.Contains(body, `type="range" min="-12" max="12" step="1"`) {
			t.Fatalf("%s rendered unexpected headset controls", test.key)
		}
	}
}

func TestVirtuosoSEWorkspaceSummariesUseSourceBackedCapabilities(t *testing.T) {
	labels := []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"}
	wProfile := &virtuosoSEW.DeviceProfile{Equalizers: map[int]virtuosoSEW.Equalizer{}, SleepMode: 15}
	wuProfile := &virtuosoSEWU.DeviceProfile{Equalizers: map[int]virtuosoSEWU.Equalizer{}}
	for index, label := range labels {
		wProfile.Equalizers[index+1] = virtuosoSEW.Equalizer{Name: label}
		wuProfile.Equalizers[index+1] = virtuosoSEWU.Equalizer{Name: label}
	}
	w := &virtuosoSEW.Device{Serial: "virtuoso-se-w", DeviceProfile: wProfile, UserProfiles: map[string]*virtuosoSEW.DeviceProfile{"Default": {Active: true}}, SleepModes: map[int]string{0: "Off", 1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}}
	wu := &virtuosoSEWU.Device{Serial: "virtuoso-se-wu", Usb: true, DeviceProfile: wuProfile, UserProfiles: map[string]*virtuosoSEWU.DeviceProfile{"Default": {Active: true}}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}}
	devicesBySerial := map[string]*common.Device{"virtuoso-se-w": {Serial: "virtuoso-se-w", Product: "VIRTUOSO SE", ProductType: common.ProductTypeVirtuosoSEW, Instance: w}, "virtuoso-se-wu": {Serial: "virtuoso-se-wu", Product: "VIRTUOSO SE", ProductType: common.ProductTypeVirtuosoSEWU, Instance: wu}}
	batteries := map[string]stats.BatteryStats{"virtuoso-se-w": {Level: 71}, "virtuoso-se-wu": {Level: 72}}
	for serial, wantSleep := range map[string]bool{"virtuoso-se-w": true, "virtuoso-se-wu": false} {
		summary, ok := devicesWorkspaceSummaryForSerial(devicesBySerial, batteries, serial)
		if !ok || summary == nil || !summary.HasBattery || (summary.SleepTimer != nil) != wantSleep || summary.Headset == nil || summary.Headset.MuteIndicator == nil || summary.Headset.Sidetone != nil || len(summary.Headset.Assignments) != 0 || !summary.LegacyLighting {
			t.Fatalf("%s summary=%#v, ok=%t", serial, summary, ok)
		}
		if len(summary.Headset.Equalizer) != 10 || summary.Headset.Equalizer[0].Label != "32" || summary.Headset.Equalizer[9].Label != "16K" || len(summary.Headset.MuteIndicator.Options) != 2 {
			t.Fatalf("%s headset=%#v", serial, summary.Headset)
		}
	}
}

func TestVirtuosoWorkspaceSummariesUseSourceBackedCapabilities(t *testing.T) {
	labels := []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"}
	wProfile := &virtuosoW.DeviceProfile{Equalizers: map[int]virtuosoW.Equalizer{}, SleepMode: 15}
	wuProfile := &virtuosoWU.DeviceProfile{Equalizers: map[int]virtuosoWU.Equalizer{}}
	for index, label := range labels {
		wProfile.Equalizers[index+1] = virtuosoW.Equalizer{Name: label}
		wuProfile.Equalizers[index+1] = virtuosoWU.Equalizer{Name: label}
	}
	w := &virtuosoW.Device{Serial: "virtuoso-w", DeviceProfile: wProfile, UserProfiles: map[string]*virtuosoW.DeviceProfile{"Default": {Active: true}}, SleepModes: map[int]string{0: "Off", 1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}}
	wu := &virtuosoWU.Device{Serial: "virtuoso-wu", Usb: true, DeviceProfile: wuProfile, UserProfiles: map[string]*virtuosoWU.DeviceProfile{"Default": {Active: true}}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}}
	devicesBySerial := map[string]*common.Device{"virtuoso-w": {Serial: "virtuoso-w", Product: "VIRTUOSO", ProductType: common.ProductTypeVirtuosoW, Instance: w}, "virtuoso-wu": {Serial: "virtuoso-wu", Product: "VIRTUOSO", ProductType: common.ProductTypeVirtuosoWU, Instance: wu}}
	batteries := map[string]stats.BatteryStats{"virtuoso-w": {Level: 71}, "virtuoso-wu": {Level: 72}}
	for serial, wantSleep := range map[string]bool{"virtuoso-w": true, "virtuoso-wu": false} {
		summary, ok := devicesWorkspaceSummaryForSerial(devicesBySerial, batteries, serial)
		if !ok || summary == nil || !summary.HasBattery || (summary.SleepTimer != nil) != wantSleep || summary.Headset == nil || summary.Headset.Sidetone != nil || len(summary.Headset.Assignments) != 0 || !summary.LegacyLighting {
			t.Fatalf("%s summary=%#v, ok=%t", serial, summary, ok)
		}
	}
}

func TestVirtuosoRGBXTWorkspaceSummariesAndPreviewsUseSourceBackedCapabilities(t *testing.T) {
	labels := []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"}
	wProfile := &virtuosorgbXTW.DeviceProfile{Equalizers: map[int]virtuosorgbXTW.Equalizer{}, SleepMode: 15, SideTone: 1, SideToneValue: 50}
	wuProfile := &virtuosorgbXTWU.DeviceProfile{Equalizers: map[int]virtuosorgbXTWU.Equalizer{}, SideTone: 1, SideToneValue: 50}
	for index, label := range labels {
		wProfile.Equalizers[index+1] = virtuosorgbXTW.Equalizer{Name: label}
		wuProfile.Equalizers[index+1] = virtuosorgbXTWU.Equalizer{Name: label}
	}
	w := &virtuosorgbXTW.Device{Serial: "virtuoso-rgb-xt-w", DeviceProfile: wProfile, UserProfiles: map[string]*virtuosorgbXTW.DeviceProfile{"Default": {Active: true}}, SleepModes: map[int]string{0: "Off", 1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}, SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}}
	wu := &virtuosorgbXTWU.Device{Serial: "virtuoso-rgb-xt-wu", Usb: true, DeviceProfile: wuProfile, UserProfiles: map[string]*virtuosorgbXTWU.DeviceProfile{"Default": {Active: true}}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}, SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}}
	devicesBySerial := map[string]*common.Device{"virtuoso-rgb-xt-w": {Serial: "virtuoso-rgb-xt-w", Product: "VIRTUOSO XT", ProductType: common.ProductTypeVirtuosoXTW, Instance: w}, "virtuoso-rgb-xt-wu": {Serial: "virtuoso-rgb-xt-wu", Product: "VIRTUOSO XT", ProductType: common.ProductTypeVirtuosoXTWU, Instance: wu}}
	batteries := map[string]stats.BatteryStats{"virtuoso-rgb-xt-w": {Level: 71}, "virtuoso-rgb-xt-wu": {Level: 72}}
	for serial, wantSleep := range map[string]bool{"virtuoso-rgb-xt-w": true, "virtuoso-rgb-xt-wu": false} {
		summary, ok := devicesWorkspaceSummaryForSerial(devicesBySerial, batteries, serial)
		if !ok || summary == nil || !summary.HasBattery || (summary.SleepTimer != nil) != wantSleep || summary.Headset == nil || summary.Headset.MuteIndicator == nil || summary.Headset.Sidetone == nil || len(summary.Headset.Assignments) != 0 || !summary.LegacyLighting {
			t.Fatalf("%s summary=%#v, ok=%t", serial, summary, ok)
		}
		if len(summary.Headset.Equalizer) != 10 || summary.Headset.Equalizer[0].Label != "32" || summary.Headset.Equalizer[9].Label != "16K" || len(summary.Headset.MuteIndicator.Options) != 2 || summary.Headset.Sidetone.ValueRange.Minimum != 1 || summary.Headset.Sidetone.ValueRange.Maximum != 100 || summary.Headset.Sidetone.ValueRange.Step != 1 {
			t.Fatalf("%s headset=%#v", serial, summary.Headset)
		}
	}

	router := legacyDevicePreviewRouter(t, true)
	for _, test := range []struct {
		key, serial string
		productType uint16
		sleep       bool
	}{
		{"virtuoso-rgb-xt-wireless-modern", "preview-virtuoso-rgb-xt-wireless-modern", common.ProductTypeVirtuosoXTW, true},
		{"virtuoso-rgb-xt-usb-modern", "preview-virtuoso-rgb-xt-usb-modern", common.ProductTypeVirtuosoXTWU, false},
	} {
		fixture, ok := modernDevicePreviewFixtureByKey(test.key)
		if !ok || fixture.ProductType != test.productType || devices.GetDevice(test.serial) != nil {
			t.Fatalf("missing, misrouted, or registered fixture %q: %#v", test.key, fixture)
		}
		summary := fixture.Build()
		if summary == nil || summary.Headset == nil || summary.Headset.Sidetone == nil || (summary.SleepTimer != nil) != test.sleep || summary.Headset.Sidetone.ValueRange.Minimum != 1 || summary.Headset.Sidetone.ValueRange.Maximum != 100 || len(summary.Headset.Assignments) != 0 {
			t.Fatalf("%s summary=%#v", test.key, summary)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+test.key))
		body := recorder.Body.String()
		if recorder.Code != http.StatusOK || strings.Contains(body, "Active Noise Cancellation") || strings.Contains(body, "Scroll Press") || strings.Contains(body, "Wheel") || strings.Contains(body, "Sleep Timer") != test.sleep || !strings.Contains(body, "Sidetone") || !strings.Contains(body, `min="1" max="100" step="1"`) {
			t.Fatalf("%s rendered unexpected controls: %s", test.key, body)
		}
		if devices.GetDevice(test.serial) != nil {
			t.Fatalf("fixture %q registered a device", test.key)
		}
	}
}

func TestVOIDModernPreviewsRenderOnlySourceBackedCapabilities(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	for _, test := range []struct {
		key, serial string
		productType uint16
		sleep       bool
	}{
		{"void-elite-wireless-modern", "preview-void-elite-wireless-modern", common.ProductTypeHS80RGB, false},
		{"void-wireless-v2-modern", "preview-void-wireless-v2-modern", common.ProductTypeVoidV2W, true},
	} {
		fixture, ok := modernDevicePreviewFixtureByKey(test.key)
		if !ok || fixture.ProductType != test.productType || devices.GetDevice(test.serial) != nil {
			t.Fatalf("missing, misrouted, or registered fixture %q: %#v", test.key, fixture)
		}
		summary := fixture.Build()
		if summary == nil || summary.DeviceProfiles == nil || !summary.DeviceProfiles.CanSwitch || !summary.DeviceProfiles.CanSave || !summary.DeviceProfiles.CanDelete || !summary.HasBattery || !summary.LegacyLighting || (summary.SleepTimer != nil) != test.sleep || summary.Headset == nil || summary.Headset.Sidetone == nil || summary.Headset.MuteIndicator != nil || summary.Headset.NoiseCancellation != nil || len(summary.Headset.Equalizer) != 10 || len(summary.Headset.Assignments) != 0 || len(summary.Headset.Wheels) != 0 {
			t.Fatalf("%s summary=%#v", test.key, summary)
		}
		if got := summary.Headset.Sidetone; got.ValueRange.Minimum != 1 || got.ValueRange.Maximum != 100 || got.ValueRange.Step != 1 {
			t.Fatalf("%s sidetone=%#v", test.key, got)
		}
		if test.sleep {
			if len(summary.SleepTimer.Options) != 6 {
				t.Fatalf("%s sleep=%#v", test.key, summary.SleepTimer)
			}
			for index, value := range []int{1, 5, 10, 15, 30, 60} {
				if summary.SleepTimer.Options[index].Value != value {
					t.Fatalf("%s sleep=%#v", test.key, summary.SleepTimer.Options)
				}
			}
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+test.key))
		body := recorder.Body.String()
		if recorder.Code != http.StatusOK || strings.Contains(body, "Mute Indicator") || strings.Contains(body, "Active Noise Cancellation") || strings.Contains(body, "Button Assignment") || strings.Contains(body, "Wheel") || strings.Contains(body, "Sleep Timer") != test.sleep || !strings.Contains(body, "Sidetone") || !strings.Contains(body, `min="1" max="100" step="1"`) {
			t.Fatalf("%s rendered unexpected controls: %s", test.key, body)
		}
		if devices.GetDevice(test.serial) != nil {
			t.Fatalf("fixture %q registered a device", test.key)
		}
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

func TestModernKeyboardDevicePreviewsRenderWorkspaceWithoutRegistration(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	keyboards.Init()
	for _, fixture := range []struct {
		key, serial, geometry, rowGeometry, polling string
		requiredNames                               []string
		rows, keys                                  int
		noModifiers, requireModifiers, noAdvanced   bool
		controlDial, sleepTimer                     bool
	}{
		{"k55-rgb-modern", "preview-k55-rgb-modern", "keyboard-6", "keyboard-row-26", "1000 Hz / 1 msec", []string{"A"}, 7, 120, true, false, true, false, false},
		{"k55-core-modern", "preview-k55-core-modern", "keyboard-6", "keyboard-row-25", "1000 Hz / 1 msec", []string{"A"}, 6, 112, true, false, true, false, false},
		{"k55-core-tkl-modern", "preview-k55-core-tkl-modern", "keyboard-6", "keyboard-row-21", "1000 Hz / 1 msec", []string{"A"}, 6, 92, true, false, true, false, false},
		{"k55-pro-modern", "preview-k55-pro-modern", "keyboard-7", "keyboard-row-26", "1000 Hz / 1 msec", []string{"A"}, 7, 120, true, false, true, false, false},
		{"k55-pro-xt-modern", "preview-k55-pro-xt-modern", "keyboard-7", "keyboard-row-26", "1000 Hz / 1 msec", []string{"A"}, 7, 120, true, false, true, false, false},
		{"k57-rgb-wireless-modern", "preview-k57-rgb-wireless-modern", "keyboard-7", "keyboard-row-26", "", []string{"A"}, 7, 120, true, false, true, false, true},
		{"k57-rgb-usb-modern", "preview-k57-rgb-usb-modern", "keyboard-7", "keyboard-row-26", "1000 Hz / 1 msec", []string{"A"}, 7, 120, true, false, true, false, false},
		{"k100-modern", "preview-k100-modern", "keyboard-8", "keyboard-row-26", "1000 Hz / 1 msec", []string{"A"}, 8, 165, true, false, true, true, false},
		{"k100-air-wireless-modern", "preview-k100-air-wireless-modern", "keyboard-7", "keyboard-row-25", "", []string{"A"}, 7, 118, false, true, true, false, true},
		{"k100-air-usb-modern", "preview-k100-air-usb-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", []string{"A"}, 7, 118, false, true, true, false, false},
		{"k60-rgb-pro-modern", "preview-k60-rgb-pro-modern", "keyboard-6", "keyboard-row-26", "1000 Hz / 1 msec", []string{"A"}, 6, 104, true, false, true, false, false},
		{"k65-plus-usb-modern", "preview-k65-plus-usb-modern", "keyboard-6", "keyboard-row-17", "", []string{"A"}, 6, 81, false, true, true, true, false},
		{"k65-pro-mini-modern", "preview-k65-pro-mini-modern", "keyboard-5", "keyboard-row-17", "1000 Hz / 1 msec", []string{"A"}, 5, 67, false, true, true, false, false},
		{"k65-rgb-mini-modern", "preview-k65-rgb-mini-modern", "keyboard-5", "keyboard-row-16", "1000 Hz / 1 msec", []string{"A"}, 5, 61, false, true, true, false, false},
		{"k65-rgb-modern", "preview-k65-rgb-modern", "keyboard-7", "keyboard-row-20", "1000 Hz / 1 msec", []string{"A", "BTS"}, 7, 92, true, false, true, false, false},
		{"k65-rgb-rapidfire-modern", "preview-k65-rgb-rapidfire-modern", "keyboard-7", "keyboard-row-20", "1000 Hz / 1 msec", []string{"A", "BTS"}, 7, 92, true, false, true, false, false},
		{"k68-rgb-modern", "preview-k68-rgb-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", []string{"A", "BTS", "Play"}, 7, 113, true, false, true, false, false},
		{"k70-lux-modern", "preview-k70-lux-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", []string{"A", "BTS", "Play"}, 7, 113, false, false, false, false, false},
		{"k70-core-tkl-wireless-modern", "preview-k70-core-tkl-wireless-modern", "keyboard-6", "keyboard-row-20", "", []string{"A"}, 6, 86, false, true, true, true, true},
		{"k70-core-tkl-usb-modern", "preview-k70-core-tkl-usb-modern", "keyboard-6", "keyboard-row-20", "1000 Hz / 1 msec", []string{"A"}, 6, 86, false, true, true, true, false},
		{"k70-lux-rgb-modern", "preview-k70-lux-rgb-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", []string{"A", "BTS", "Play"}, 7, 113, false, false, false, false, false},
		{"k70-rgb-rf-modern", "preview-k70-rgb-rf-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", []string{"A", "BTS", "Play"}, 7, 113, false, false, false, false, false},
		{"k70-pro-mini-wireless-modern", "preview-k70-pro-mini-wireless-modern", "keyboard-7", "keyboard-row-18", "", []string{"A"}, 7, 93, false, true, true, false, true},
		{"k70-pro-mini-usb-modern", "preview-k70-pro-mini-usb-modern", "keyboard-7", "keyboard-row-18", "1000 Hz / 1 msec", []string{"A"}, 7, 93, false, true, true, false, false},
		{"k70-rgb-tkl-cs-modern", "preview-k70-rgb-tkl-cs-modern", "keyboard-7", "keyboard-row-20", "1000 Hz / 1 msec", []string{"A", "BTS", "PLAY"}, 7, 98, false, true, true, false, false},
		{"k70-mk2-modern", "preview-k70-mk2-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", []string{"A", "BTS", "Play"}, 7, 116, false, false, false, false, false},
		{"strafe-rgb-mk2-modern", "preview-strafe-rgb-mk2-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", []string{"A", "BTS", "Play"}, 7, 116, true, false, true, false, false},
		{"k95-modern", "preview-k95-modern", "keyboard-7", "keyboard-row-27", "1000 Hz / 1 msec", []string{"A", "G1", "Play"}, 7, 135, false, false, false, false, false},
		{"k95-platinum-xt-modern", "preview-k95-platinum-xt-modern", "keyboard-8", "keyboard-row-26", "1000 Hz / 1 msec", []string{"A", "G1", "Play"}, 8, 139, false, false, false, false, false},
	} {
		if devices.GetDevice(fixture.serial) != nil {
			t.Fatalf("fixture serial %q unexpectedly registered", fixture.serial)
		}
		preview, ok := modernDevicePreviewFixtureByKey(fixture.key)
		if !ok {
			t.Fatalf("missing preview fixture %q", fixture.key)
		}
		summary := preview.Build()
		if summary.KeyboardAssignments == nil || summary.Performance == nil || summary.DeviceProfiles == nil || !summary.LegacyLighting || summary.KeyboardAssignments.LayoutClass != fixture.geometry || summary.KeyboardAssignments.RowLayoutClass != fixture.rowGeometry || len(summary.KeyboardAssignments.Rows) != fixture.rows {
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
		if keyCount != fixture.keys {
			t.Fatalf("%s keyboard rows=%d keys=%d names=%#v", fixture.key, len(summary.KeyboardAssignments.Rows), keyCount, keyNames)
		}
		for _, name := range fixture.requiredNames {
			if !keyNames[name] {
				t.Fatalf("%s keyboard names=%#v, missing %q", fixture.key, keyNames, name)
			}
		}
		if fixture.noModifiers && len(summary.KeyboardAssignments.ModifierOptions) != 0 {
			t.Fatalf("%s advertised modifier options: %#v", fixture.key, summary.KeyboardAssignments.ModifierOptions)
		}
		if fixture.requireModifiers && len(summary.KeyboardAssignments.ModifierOptions) < 2 {
			t.Fatalf("%s omitted source modifier options: %#v", fixture.key, summary.KeyboardAssignments.ModifierOptions)
		}
		if fixture.noAdvanced && (summary.KeyActuation != nil || summary.FlashTap != nil) {
			t.Fatalf("%s advertised unsupported advanced controls", fixture.key)
		}
		if (summary.ControlDial != nil) != fixture.controlDial || (summary.SleepTimer != nil) != fixture.sleepTimer {
			t.Fatalf("%s controlDial=%#v sleepTimer=%#v", fixture.key, summary.ControlDial, summary.SleepTimer)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+fixture.key+"?view=keyboard"))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d: %s", fixture.key, recorder.Code, recorder.Body.String())
		}
		body := recorder.Body.String()
		expected := append([]string{fixture.geometry, fixture.polling, "Disable Win Key", "perf_shiftTab", "perf_altTab", "perf_altF4"}, fixture.requiredNames...)
		for _, expected := range expected {
			if !strings.Contains(body, expected) {
				t.Errorf("%s preview omitted %q", fixture.key, expected)
			}
		}
		if fixture.key == "k100-modern" {
			if strings.Contains(body, "lf-keyboard-lighting-geometry") || strings.Contains(body, `data-lf-key-index="1"`) || strings.Contains(body, `data-lf-key-index="23"`) || strings.Contains(body, `data-lf-key-index="31"`) || !strings.Contains(body, `data-lf-key-index="24"`) || !strings.Contains(body, `data-lf-key-index="25"`) || !strings.Contains(body, `data-lf-key-index="26"`) || !strings.Contains(body, `data-lf-key-index="33"`) || !strings.Contains(body, `data-lf-key-index="34"`) || !strings.Contains(body, `data-lf-key-index="71"`) || !strings.Contains(body, `data-lf-key-index="74"`) || !strings.Contains(body, `data-lf-key-index="50"`) || !strings.Contains(body, `data-lf-key-index="98"`) || strings.Count(body, "data-lf-keyboard-row") != 7 {
				t.Fatalf("k100-modern lighting-only keyboard rendering is invalid")
			}
			lightingOnly := 0
			for _, row := range summary.KeyboardAssignments.Rows {
				for _, key := range row.Keys {
					if key.LightingOnly {
						lightingOnly++
						if key.Assignable || key.Width < 1 || key.Height < 1 {
							t.Fatalf("k100-modern lighting-only summary key = %#v", key)
						}
					}
				}
			}
			if lightingOnly == 0 {
				t.Fatal("k100-modern omitted lighting-only geometry")
			}
			keysByIndex := make(map[int]devicesKeyboardAssignmentKeySummary)
			rowsByIndex := make(map[int]int)
			for _, row := range summary.KeyboardAssignments.Rows {
				for _, key := range row.Keys {
					keysByIndex[key.KeyIndex] = key
					rowsByIndex[key.KeyIndex] = row.Index
				}
			}
			for index, expected := range map[int]struct {
				name, keySpace, css string
			}{
				81:  {name: "Tab", keySpace: "keyboard-key wide", css: "top-32"},
				82:  {name: "Q", css: "top-32"},
				94:  {name: "\\ |", keySpace: "keyboard-key wide", css: "top-32"},
				124: {name: "Shift", keySpace: "keyboard-key wide3", css: "top-32"},
				125: {name: "Z", css: "top-32"},
				135: {name: "Shift", keySpace: "keyboard-key wide3", css: "top-32"},
			} {
				key, ok := keysByIndex[index]
				if !ok || key.KeyName != expected.name || key.KeySpace != expected.keySpace || key.CSS != expected.css || key.Height != 70 || key.Top != 15 || !key.Assignable || key.HalfKey {
					t.Fatalf("k100-modern ordinary row key %d = %#v", index, key)
				}
			}
			for index, expected := range map[int]struct {
				name string
				row  int
			}{101: {name: "+", row: 4}, 140: {name: "Enter", row: 6}} {
				key, ok := keysByIndex[index]
				if !ok || key.KeyName != expected.name || rowsByIndex[index] != expected.row || key.KeySpace != "keyboard-key-125-top32" || key.CSS != "" || key.Height != 155 || key.Top != 15 || !key.Assignable || key.HalfKey {
					t.Fatalf("k100-modern tall key %d = %#v", index, key)
				}
			}
		}
		if strings.HasPrefix(fixture.key, "k100-air-") && (!strings.Contains(body, "Auto Brightness") || !strings.Contains(body, `data-lf-boolean-setting-id="auto-brightness"`)) {
			t.Errorf("%s preview omitted Auto Brightness setting", fixture.key)
		}
		recorder = httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+fixture.key))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Device Profile") || !strings.Contains(recorder.Body.String(), "Default") {
			t.Errorf("%s overview omitted device-profile state", fixture.key)
		}
		if fixture.controlDial && !strings.Contains(recorder.Body.String(), "Control Dial") {
			t.Errorf("%s overview omitted control dial", fixture.key)
		}
		if fixture.key == "k100-modern" {
			if !summary.DeviceProfiles.CanSwitch || !summary.DeviceProfiles.CanSave || summary.DeviceProfiles.CanDelete || summary.Performance.DebounceTime == nil || summary.OptionColors == nil || summary.OptionColors.Selected != 1 || len(summary.OptionColors.Options) != 7 {
				t.Fatalf("k100-modern capabilities = profiles:%#v performance:%#v colors:%#v", summary.DeviceProfiles, summary.Performance, summary.OptionColors)
			}
			body := recorder.Body.String()
			if !strings.Contains(body, "Control Dial Colors") || strings.Contains(body, "Delete Device Profile") {
				t.Fatalf("k100-modern overview controls = %s", body)
			}
		}
		if !fixture.sleepTimer && strings.Contains(recorder.Body.String(), "Sleep Timer") {
			t.Errorf("%s overview advertised unsupported sleep timer", fixture.key)
		}
		if devices.GetDevice(fixture.serial) != nil {
			t.Fatalf("fixture serial %q registered during preview", fixture.serial)
		}
	}
}

func TestStrafeRGBMK2ModernPreviewProvidesOnlySourceBackedKeyboardWorkspaceWithoutRegistration(t *testing.T) {
	const serial = "preview-strafe-rgb-mk2-modern"
	if devices.GetDevice(serial) != nil {
		t.Fatalf("fixture serial %q unexpectedly registered", serial)
	}
	fixture, ok := modernDevicePreviewFixtureByKey("strafe-rgb-mk2-modern")
	if !ok || fixture.ProductType != common.ProductTypeStrafeRgbMk2 || fixture.Title != "STRAFE RGB MK2" {
		t.Fatalf("fixture=%#v ok=%t", fixture, ok)
	}
	summary := fixture.Build()
	if summary == nil {
		t.Fatal("preview summary is nil")
	}
	if !summary.LegacyLighting {
		t.Fatal("preview did not advertise legacy Lighting")
	}
	if summary.KeyboardAssignments == nil || summary.KeyboardAssignments.LayoutClass != "keyboard-7" || summary.KeyboardAssignments.RowLayoutClass != "keyboard-row-25" || len(summary.KeyboardAssignments.Rows) != 7 || len(summary.KeyboardAssignments.ModifierOptions) != 0 || summary.Performance == nil || summary.Performance.PollingRate == nil || len(summary.Performance.BooleanSettings) != 4 || summary.DeviceProfiles == nil || !summary.DeviceProfiles.CanSwitch || !summary.DeviceProfiles.CanSave || !summary.DeviceProfiles.CanDelete {
		t.Fatalf("summary=%#v", summary)
	}
	for value, label := range map[int]string{0: "Not Set", 8: "125 Hz / 8 msec", 4: "250 Hz / 4 msec", 2: "500 Hz / 2 msec", 1: "1000 Hz / 1 msec"} {
		found := false
		for _, option := range summary.Performance.PollingRate.Options {
			if option.Value == value && option.Label == label {
				found = true
			}
		}
		if !found {
			t.Fatalf("polling options=%#v, missing %d=%q", summary.Performance.PollingRate.Options, value, label)
		}
	}
	if summary.SleepTimer != nil || summary.ControlDial != nil || summary.OptionColors != nil || summary.KeyActuation != nil || summary.FlashTap != nil || summary.HasBattery || summary.KeyboardAssignments.LiveRGBAvailable {
		t.Fatalf("unsupported controls leaked into summary=%#v", summary)
	}
	keyCount := 0
	for _, row := range summary.KeyboardAssignments.Rows {
		keyCount += len(row.Keys)
	}
	if keyCount != 116 {
		t.Fatalf("keyboard key count=%d", keyCount)
	}
	if devices.GetDevice(serial) != nil {
		t.Fatal("preview fixture registered hardware or persistence state")
	}
}

func TestModernKeyboardLiveRGBPreviewCapabilitiesMatchDeviceContracts(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	keyboards.Init()
	for _, fixture := range []struct {
		key, serial string
		live        bool
	}{
		{"k55-rgb-modern", "preview-k55-rgb-modern", false},
		{"k55-core-modern", "preview-k55-core-modern", false},
		{"k55-core-tkl-modern", "preview-k55-core-tkl-modern", false},
		{"k55-pro-modern", "preview-k55-pro-modern", false},
		{"k55-pro-xt-modern", "preview-k55-pro-xt-modern", false},
		{"k57-rgb-wireless-modern", "preview-k57-rgb-wireless-modern", false},
		{"k57-rgb-usb-modern", "preview-k57-rgb-usb-modern", false},
		{"k60-rgb-pro-modern", "preview-k60-rgb-pro-modern", false},
		{"k65-plus-usb-modern", "preview-k65-plus-usb-modern", false},
		{"k65-plus-wireless-modern", "preview-k65-plus-wireless-modern", false},
		{"k65-pro-mini-modern", "preview-k65-pro-mini-modern", false},
		{"k65-rgb-mini-modern", "preview-k65-rgb-mini-modern", false},
		{"k65-rgb-modern", "preview-k65-rgb-modern", false},
		{"k65-rgb-rapidfire-modern", "preview-k65-rgb-rapidfire-modern", false},
		{"k68-rgb-modern", "preview-k68-rgb-modern", false},
		{"k70-core-modern", "preview-k70-core-modern", false},
		{"k70-core-tkl-modern", "preview-k70-core-tkl-modern", false},
		{"k70-pro-modern", "preview-k70-pro-modern", false},
		{"k70-pro-tkl-modern", "preview-k70-pro-tkl-modern", false},
		{"k70-max-modern", "preview-k70-max-modern", false},
		{"k70-lux-modern", "preview-k70-lux-modern", false},
		{"k70-lux-rgb-modern", "preview-k70-lux-rgb-modern", false},
		{"k70-rgb-rf-modern", "preview-k70-rgb-rf-modern", false},
		{"k70-mk2-modern", "preview-k70-mk2-modern", false},
		{"k95-modern", "preview-k95-modern", false},
		{"k95-platinum-modern", "preview-k95-platinum-modern", true},
		{"k95-platinum-xt-modern", "preview-k95-platinum-xt-modern", false},
	} {
		preview, ok := modernDevicePreviewFixtureByKey(fixture.key)
		if !ok {
			t.Fatalf("missing preview fixture %q", fixture.key)
		}
		summary := preview.Build()
		if summary.KeyboardAssignments == nil || summary.KeyboardAssignments.LiveRGBAvailable != fixture.live || (summary.KeyboardAssignments.LiveRGBEnabled && !fixture.live) {
			t.Fatalf("%s live RGB summary = %#v", fixture.key, summary.KeyboardAssignments)
		}
		if devices.GetDevice(fixture.serial) != nil {
			t.Fatalf("fixture serial %q registered during build", fixture.serial)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+fixture.key+"?view=keyboard"))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d: %s", fixture.key, recorder.Code, recorder.Body.String())
		}
		if got := strings.Contains(recorder.Body.String(), "data-lf-keyboard-live-rgb"); got != fixture.live {
			t.Errorf("%s rendered live RGB = %t, want %t", fixture.key, got, fixture.live)
		}
	}
}

func TestK57ModernPreviewsKeepTransportSpecificBatteryAndSleepCapabilities(t *testing.T) {
	wireless := buildK57RGBWirelessModernPreview()
	if !wireless.HasBattery || wireless.BatteryLevel != 78 || wireless.SleepTimer == nil || wireless.Performance == nil || wireless.Performance.PollingRate != nil || wireless.ControlDial != nil {
		t.Fatalf("wireless summary=%#v", wireless)
	}
	usb := buildK57RGBUSBModernPreview()
	if !usb.HasBattery || usb.BatteryLevel != 78 || usb.SleepTimer != nil || usb.Performance == nil || usb.Performance.PollingRate == nil || usb.ControlDial != nil {
		t.Fatalf("USB summary=%#v", usb)
	}
}

func TestK70ProMiniModernPreviewsKeepTransportSpecificCapabilities(t *testing.T) {
	wireless := buildK70ProMiniWirelessModernPreview()
	if !wireless.HasBattery || wireless.BatteryLevel != 78 || wireless.SleepTimer == nil || wireless.Performance == nil || wireless.Performance.PollingRate != nil || wireless.ControlDial != nil || wireless.OptionColors != nil || wireless.KeyActuation != nil || wireless.FlashTap != nil || !wireless.LegacyLighting {
		t.Fatalf("wireless summary=%#v", wireless)
	}
	usb := buildK70ProMiniUSBModernPreview()
	if !usb.HasBattery || usb.BatteryLevel != 78 || usb.SleepTimer != nil || usb.Performance == nil || usb.Performance.PollingRate == nil || len(usb.Performance.PollingRate.Options) != 8 || usb.ControlDial != nil || usb.OptionColors != nil || usb.KeyActuation != nil || usb.FlashTap != nil || !usb.LegacyLighting {
		t.Fatalf("USB summary=%#v", usb)
	}
}

func TestK70CoreTKLModernPreviewsKeepTransportSpecificCapabilities(t *testing.T) {
	wireless := buildK70CoreTKLWirelessModernPreview()
	if !wireless.HasBattery || wireless.BatteryLevel != 78 || wireless.SleepTimer == nil || wireless.Performance == nil || wireless.Performance.PollingRate != nil || wireless.ControlDial == nil || len(wireless.ControlDial.Options) != 7 || wireless.OptionColors != nil || wireless.KeyActuation != nil || wireless.FlashTap != nil || !wireless.LegacyLighting {
		t.Fatalf("wireless summary=%#v", wireless)
	}
	usb := buildK70CoreTKLUSBModernPreview()
	if !usb.HasBattery || usb.BatteryLevel != 78 || usb.SleepTimer != nil || usb.Performance == nil || usb.Performance.PollingRate == nil || len(usb.Performance.PollingRate.Options) != 5 || usb.ControlDial == nil || len(usb.ControlDial.Options) != 7 || usb.OptionColors != nil || usb.KeyActuation != nil || usb.FlashTap != nil || !usb.LegacyLighting {
		t.Fatalf("USB summary=%#v", usb)
	}
}

func TestK100AirModernPreviewsKeepTransportSpecificCapabilities(t *testing.T) {
	keyboards.Init()
	for layout, expectedKeys := range map[string]int{"US": 118, "DE": 119, "FR": 119} {
		keyboard := keyboards.GetKeyboard("k100air-default-" + layout)
		keyCount := 0
		for _, row := range keyboard.Row {
			keyCount += len(row.Keys)
		}
		if len(keyboard.Row) != 7 || keyCount != expectedKeys {
			t.Fatalf("%s geometry rows=%d keys=%d", layout, len(keyboard.Row), keyCount)
		}
	}
	wireless := buildK100AirWirelessModernPreview()
	if !wireless.HasBattery || wireless.BatteryLevel != 78 || wireless.SleepTimer == nil || wireless.Performance == nil || wireless.Performance.PollingRate != nil || wireless.ControlDial != nil || wireless.BooleanSettings == nil || len(wireless.BooleanSettings.Settings) != 1 || wireless.KeyboardAssignments == nil || wireless.KeyboardAssignments.LayoutClass != "keyboard-7" || wireless.KeyboardAssignments.RowLayoutClass != "keyboard-row-25" || len(wireless.KeyboardAssignments.Rows) != 7 || !wireless.LegacyLighting {
		t.Fatalf("wireless summary=%#v", wireless)
	}
	usb := buildK100AirUSBModernPreview()
	if !usb.HasBattery || usb.BatteryLevel != 78 || usb.SleepTimer != nil || usb.Performance == nil || usb.Performance.PollingRate == nil || len(usb.Performance.PollingRate.Options) != 8 || usb.ControlDial != nil || usb.BooleanSettings == nil || len(usb.BooleanSettings.Settings) != 1 || usb.KeyboardAssignments == nil || usb.KeyboardAssignments.LayoutClass != "keyboard-7" || usb.KeyboardAssignments.RowLayoutClass != "keyboard-row-25" || len(usb.KeyboardAssignments.Rows) != 7 || !usb.LegacyLighting {
		t.Fatalf("USB summary=%#v", usb)
	}
}

func TestK70ProTKLModernPreviewProvidesCompleteAdvancedKeyboardWorkspaceWithoutRegistration(t *testing.T) {
	const serial = "preview-k70-pro-tkl-modern"
	if devices.GetDevice(serial) != nil {
		t.Fatalf("fixture serial %q unexpectedly registered", serial)
	}
	fixture, ok := modernDevicePreviewFixtureByKey("k70-pro-tkl-modern")
	if !ok || fixture.ProductType != common.ProductTypeK70ProTkl || fixture.Title != "K70 RGB PRO TKL" {
		t.Fatalf("fixture=%#v ok=%t", fixture, ok)
	}
	summary := fixture.Build()
	if summary.KeyboardAssignments == nil || summary.KeyboardAssignments.LayoutClass != "keyboard-6" || summary.KeyboardAssignments.RowLayoutClass != "keyboard-row-20" || len(summary.KeyboardAssignments.Rows) != 6 || summary.Performance == nil || summary.DeviceProfiles == nil || !summary.LegacyLighting {
		t.Fatalf("summary=%#v", summary)
	}
	keyCount := 0
	for _, row := range summary.KeyboardAssignments.Rows {
		keyCount += len(row.Keys)
	}
	if keyCount != 86 || summary.KeyActuation == nil || len(summary.KeyActuation.Keys) != 2 || !summary.KeyActuation.Keys[0].Supported || summary.KeyActuation.Keys[1].Supported || summary.KeyActuation.Keys[0].SecondaryActuationPoint != 30 {
		t.Fatalf("keyboard=%#v actuation=%#v", summary.KeyboardAssignments, summary.KeyActuation)
	}
	if summary.FlashTap == nil || !summary.FlashTap.Active || summary.FlashTap.Mode != 1 || len(summary.FlashTap.Modes) != 3 || len(summary.FlashTap.SelectedSlots) != 2 || summary.FlashTap.SelectedSlots[0].KeyIndex != 7 || summary.FlashTap.SelectedSlots[1].KeyIndex != 4 || summary.FlashTap.Color != (devicesFlashTapColorSummary{Red: 17, Green: 93, Blue: 201}) {
		t.Fatalf("flashTap=%#v", summary.FlashTap)
	}
	if devices.GetDevice(serial) != nil {
		t.Fatal("preview fixture registered hardware")
	}
}

func TestK70RGBTKLCSModernPreviewProvidesItsSupportedKeyboardWorkspaceWithoutRegistration(t *testing.T) {
	const serial = "preview-k70-rgb-tkl-cs-modern"
	if devices.GetDevice(serial) != nil {
		t.Fatalf("fixture serial %q unexpectedly registered", serial)
	}
	fixture, ok := modernDevicePreviewFixtureByKey("k70-rgb-tkl-cs-modern")
	if !ok || fixture.ProductType != common.ProductTypeK70RgbTkl || fixture.Title != "K70 RGB TKL CS" {
		t.Fatalf("fixture = %#v, ok = %t", fixture, ok)
	}
	summary := fixture.Build()
	if summary.KeyboardAssignments == nil || summary.KeyboardAssignments.LayoutClass != "keyboard-7" || summary.KeyboardAssignments.RowLayoutClass != "keyboard-row-20" || len(summary.KeyboardAssignments.Rows) != 7 || len(summary.KeyboardAssignments.ModifierOptions) < 2 || summary.Performance == nil || summary.Performance.PollingRate == nil || len(summary.Performance.PollingRate.Options) != 8 || summary.DeviceProfiles == nil || !summary.DeviceProfiles.CanSwitch || !summary.DeviceProfiles.CanSave || !summary.DeviceProfiles.CanDelete || !summary.LegacyLighting {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.SleepTimer != nil || summary.ControlDial != nil || summary.OptionColors != nil || summary.KeyActuation != nil || summary.FlashTap != nil || summary.HasBattery {
		t.Fatalf("unsupported controls leaked into summary = %#v", summary)
	}
	keyCount := 0
	for _, row := range summary.KeyboardAssignments.Rows {
		keyCount += len(row.Keys)
	}
	if keyCount != 98 {
		t.Fatalf("keyboard key count = %d", keyCount)
	}
	if devices.GetDevice(serial) != nil {
		t.Fatal("preview fixture registered hardware")
	}
}

func TestMAKR75ModernPreviewsProvideOnlyTheirSourceBackedWorkspacesWithoutRegistration(t *testing.T) {
	initializeLegacyDevicePreviewTestProcess(t)
	keyboards.Init()
	for _, test := range []struct {
		key, serial, title              string
		productType                     uint16
		battery, sleep, polling, colors bool
	}{
		{"makr75-wireless-modern", "preview-makr75-wireless-modern", "MAKR 75 Wireless", common.ProductTypeMakr75W, true, true, false, false},
		{"makr75-usb-modern", "preview-makr75-usb-modern", "MAKR 75 USB", common.ProductTypeMakr75WU, false, false, true, true},
	} {
		t.Run(test.key, func(t *testing.T) {
			if devices.GetDevice(test.serial) != nil {
				t.Fatalf("fixture serial %q unexpectedly registered", test.serial)
			}
			fixture, ok := modernDevicePreviewFixtureByKey(test.key)
			if !ok || fixture.Title != test.title || fixture.ProductType != test.productType {
				t.Fatalf("fixture=%#v ok=%t", fixture, ok)
			}
			summary := fixture.Build()
			if summary == nil || summary.Product != "MAKR 75" || !summary.LegacyLighting || summary.KeyboardAssignments == nil || summary.KeyboardAssignments.LayoutClass != "keyboard-6" || summary.KeyboardAssignments.RowLayoutClass != "keyboard-row-17" || len(summary.KeyboardAssignments.Rows) == 0 || len(summary.KeyboardAssignments.AssignmentTypes) != 6 || len(summary.KeyboardAssignments.ModifierOptions) < 2 || summary.Performance == nil || len(summary.Performance.BooleanSettings) != 4 || summary.DeviceProfiles == nil || !summary.DeviceProfiles.CanSwitch || !summary.DeviceProfiles.CanSave || !summary.DeviceProfiles.CanDelete || summary.ControlDial == nil || len(summary.ControlDial.Options) != 7 {
				t.Fatalf("summary=%#v", summary)
			}
			if summary.HasBattery != test.battery || (test.battery && summary.BatteryLevel != 78) || (summary.SleepTimer != nil) != test.sleep || (summary.Performance.PollingRate != nil) != test.polling || (summary.OptionColors != nil) != test.colors || summary.KeyActuation != nil || summary.FlashTap != nil || summary.KeyboardAssignments.LiveRGBAvailable {
				t.Fatalf("unsupported or missing capability in summary=%#v", summary)
			}
			if test.sleep && len(summary.SleepTimer.Options) != 6 {
				t.Fatalf("sleep=%#v", summary.SleepTimer)
			}
			if test.polling && len(summary.Performance.PollingRate.Options) != 8 {
				t.Fatalf("polling=%#v", summary.Performance.PollingRate)
			}
			if test.colors && (summary.OptionColors.Selected != summary.ControlDial.Value || len(summary.OptionColors.Options) != 7) {
				t.Fatalf("colors=%#v dial=%#v", summary.OptionColors, summary.ControlDial)
			}
			if devices.GetDevice(test.serial) != nil {
				t.Fatal("preview fixture registered hardware")
			}
		})
	}
}

func TestVanguard99AirModernPreviewsProvideOnlyTheirSourceBackedWorkspacesWithoutRegistration(t *testing.T) {
	initializeLegacyDevicePreviewTestProcess(t)
	keyboards.Init()
	for _, test := range []struct {
		key, serial, title, dial string
		productType              uint16
		sleep, polling, colors   bool
	}{
		{"vanguard99air-wireless-modern", "preview-vanguard99air-wireless-modern", "VANGUARD 99 AIR Wireless", "Vertical Scroll", common.ProductTypeVanguard99AirW, true, false, false},
		{"vanguard99air-usb-modern", "preview-vanguard99air-usb-modern", "VANGUARD 99 AIR USB", "Scroll", common.ProductTypeVanguard99AirWU, false, true, true},
	} {
		t.Run(test.key, func(t *testing.T) {
			if devices.GetDevice(test.serial) != nil {
				t.Fatalf("fixture serial %q unexpectedly registered", test.serial)
			}
			fixture, ok := modernDevicePreviewFixtureByKey(test.key)
			if !ok || fixture.Title != test.title || fixture.ProductType != test.productType {
				t.Fatalf("fixture=%#v ok=%t", fixture, ok)
			}
			summary := fixture.Build()
			if summary == nil || summary.Product != "VANGUARD 99 AIR" || !summary.LegacyLighting || !summary.HasBattery || summary.BatteryLevel != 78 || summary.KeyboardAssignments == nil || summary.KeyboardAssignments.LayoutClass != "keyboard-6" || summary.KeyboardAssignments.RowLayoutClass != "keyboard-row-22" || len(summary.KeyboardAssignments.Rows) == 0 || len(summary.KeyboardAssignments.AssignmentTypes) != 6 || len(summary.KeyboardAssignments.ModifierOptions) < 2 || summary.KeyboardAssignments.LiveRGBAvailable || summary.Performance == nil || len(summary.Performance.BooleanSettings) != 4 || summary.DeviceProfiles == nil || !summary.DeviceProfiles.CanSwitch || !summary.DeviceProfiles.CanSave || !summary.DeviceProfiles.CanDelete || summary.ControlDial == nil || len(summary.ControlDial.Options) != 7 || summary.ControlDial.Options[2].Label != test.dial || summary.FlashTap == nil || len(summary.FlashTap.Modes) != 3 || summary.KeyActuation != nil {
				t.Fatalf("summary=%#v", summary)
			}
			if (summary.SleepTimer != nil) != test.sleep || (summary.Performance.PollingRate != nil) != test.polling || (summary.OptionColors != nil) != test.colors {
				t.Fatalf("unsupported or missing capability in summary=%#v", summary)
			}
			if test.sleep && len(summary.SleepTimer.Options) != 6 {
				t.Fatalf("sleep=%#v", summary.SleepTimer)
			}
			if test.polling && len(summary.Performance.PollingRate.Options) != 8 {
				t.Fatalf("polling=%#v", summary.Performance.PollingRate)
			}
			if test.colors && (summary.OptionColors.Selected != summary.ControlDial.Value || len(summary.OptionColors.Options) != 7) {
				t.Fatalf("colors=%#v dial=%#v", summary.OptionColors, summary.ControlDial)
			}
			if devices.GetDevice(test.serial) != nil {
				t.Fatal("preview fixture registered hardware")
			}
		})
	}
}

func TestK55CoreModernPreviewPreservesItsSourceRowOverride(t *testing.T) {
	fixture, ok := modernDevicePreviewFixtureByKey("k55-core-modern")
	if !ok {
		t.Fatal("missing K55 CORE modern preview")
	}
	summary := fixture.Build()
	if summary.KeyboardAssignments == nil || len(summary.KeyboardAssignments.Rows) != 6 || summary.KeyboardAssignments.Rows[3].CSS != "keyboard-row-24" {
		t.Fatalf("K55 CORE rows = %#v", summary.KeyboardAssignments)
	}
}

func TestK55CoreModernPreviewsKeepHalfKeyPairsWithinTheirDeclaredTopRows(t *testing.T) {
	for _, fixture := range []struct {
		name    string
		summary *devicesWorkspaceSummary
		tracks  int
	}{
		{name: "K55 CORE RGB", summary: buildK55CoreModernPreview(), tracks: 25},
		{name: "K55 CORE TKL", summary: buildK55CoreTKLModernPreview(), tracks: 21},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			if fixture.summary.KeyboardAssignments == nil || len(fixture.summary.KeyboardAssignments.Rows) == 0 {
				t.Fatal("missing keyboard workspace")
			}
			items := fixture.summary.KeyboardAssignments.Rows[0].Items()
			tracks, groups := 0, 0
			for _, item := range items {
				tracks += len(item.KeyEmpty) + len(item.Spacing) + 1
				if item.HalfKeyPair {
					groups++
				}
			}
			if tracks != fixture.tracks || groups != 2 {
				t.Fatalf("top row tracks=%d groups=%d items=%#v", tracks, groups, items)
			}
			last := items[len(items)-1].Keys[0]
			if last.KeyName != "VOL+" || last.KeyIndex != 22 {
				t.Fatalf("top row last key = %#v", last)
			}
		})
	}
}

func TestK70MaxModernPreviewProvidesFullKeyboardWorkspaceWithoutRegistration(t *testing.T) {
	const serial = "preview-k70-max-modern"
	if devices.GetDevice(serial) != nil {
		t.Fatalf("fixture serial %q unexpectedly registered", serial)
	}
	fixture, ok := modernDevicePreviewFixtureByKey("k70-max-modern")
	if !ok || fixture.ProductType != common.ProductTypeK70Max || fixture.Title != "K70 MAX" {
		t.Fatalf("fixture=%#v ok=%t", fixture, ok)
	}
	summary := fixture.Build()
	if summary.KeyboardAssignments == nil || summary.KeyboardAssignments.LayoutClass != "keyboard-7" || summary.KeyboardAssignments.RowLayoutClass != "keyboard-row-25" || len(summary.KeyboardAssignments.Rows) != 7 || summary.Performance == nil || summary.DeviceProfiles == nil || !summary.LegacyLighting {
		t.Fatalf("summary=%#v", summary)
	}
	keyCount := 0
	for _, row := range summary.KeyboardAssignments.Rows {
		keyCount += len(row.Keys)
	}
	if keyCount != 117 || len(summary.KeyboardAssignments.ModifierOptions) < 2 || summary.KeyActuation == nil || len(summary.KeyActuation.Keys) != 2 || !summary.KeyActuation.Keys[0].Supported || summary.KeyActuation.Keys[1].Supported {
		t.Fatalf("keyboard=%#v actuation=%#v", summary.KeyboardAssignments, summary.KeyActuation)
	}
	modifierState := false
	for _, row := range summary.KeyboardAssignments.Rows {
		for _, key := range row.Keys {
			if key.KeyIndex == 71 && key.ModifierKey == summary.KeyboardAssignments.ModifierOptions[1].ID && key.RetainOriginal {
				modifierState = true
			}
		}
	}
	if !modifierState {
		t.Fatalf("modifier state=%#v", summary.KeyboardAssignments)
	}
	if summary.FlashTap == nil || !summary.FlashTap.Active || summary.FlashTap.Mode != 1 || len(summary.FlashTap.SelectedSlots) != 2 || summary.FlashTap.SelectedSlots[0].KeyIndex != 73 || summary.FlashTap.SelectedSlots[1].KeyIndex != 71 || summary.FlashTap.Color != (devicesFlashTapColorSummary{Red: 19, Green: 97, Blue: 203}) {
		t.Fatalf("flashTap=%#v", summary.FlashTap)
	}
	if devices.GetDevice(serial) != nil {
		t.Fatal("preview fixture registered hardware")
	}
}

func TestClipperProMini60ModernPreviewUsesItsShippedKeyboardWithoutRegistration(t *testing.T) {
	keyboards.Init()
	const serial = "preview-clipper-pro-mini-60-modern"
	if source := keyboards.GetKeyboard("clipperpromini60-default-US"); source == nil || source.Key != "clipperpromini60-default" || source.Layout != "US" {
		t.Fatalf("shipped Clipper keyboard source = %#v", source)
	}
	if devices.GetDevice(serial) != nil {
		t.Fatalf("fixture serial %q unexpectedly registered", serial)
	}
	fixture, ok := modernDevicePreviewFixtureByKey("clipper-pro-mini-60-modern")
	if !ok || fixture.ProductType != common.ProductTypeClipperProMini60 || fixture.Title != "CLIPPER PRO MINI 60" {
		t.Fatalf("fixture=%#v ok=%t", fixture, ok)
	}
	summary := fixture.Build()
	if summary == nil || !summary.LegacyLighting || summary.KeyboardAssignments == nil || summary.KeyboardAssignments.LayoutClass != "keyboard-5" || summary.KeyboardAssignments.RowLayoutClass != "keyboard-row-16" || len(summary.KeyboardAssignments.Rows) != 5 || len(summary.KeyboardAssignments.AssignmentTypes) != 14 || len(summary.KeyboardAssignments.ModifierOptions) < 2 || summary.Performance == nil || summary.Performance.PollingRate == nil || len(summary.Performance.PollingRate.Options) != 8 || summary.DeviceProfiles == nil || !summary.DeviceProfiles.CanSwitch || !summary.DeviceProfiles.CanSave || !summary.DeviceProfiles.CanDelete || summary.ControlDial == nil || len(summary.ControlDial.Options) != 7 || summary.KeyActuation == nil || summary.FlashTap == nil {
		t.Fatalf("summary=%#v", summary)
	}
	if summary.SleepTimer != nil || summary.HasBattery || summary.OptionColors != nil {
		t.Fatalf("unsupported controls leaked into summary=%#v", summary)
	}
	eligible := false
	for _, key := range summary.FlashTap.Keys {
		if key.Eligible {
			eligible = true
		}
	}
	if !eligible || summary.FlashTap.Mode != 1 || len(summary.FlashTap.Modes) != 3 || summary.FlashTap.Modes[0].Label != "Neutral" || summary.FlashTap.Modes[1].Label != "Last Priority" || summary.FlashTap.Modes[2].Label != "First Priority" {
		t.Fatalf("advanced snapshots actuation=%#v flashTap=%#v", summary.KeyActuation, summary.FlashTap)
	}
	if devices.GetDevice(serial) != nil {
		t.Fatal("preview fixture registered hardware")
	}
}

func TestVanguard96ModernPreviewsUseShippedKeyboardWithoutRegistration(t *testing.T) {
	initializeLegacyDevicePreviewTestProcess(t)
	keyboards.Init()
	if source := keyboards.GetKeyboard("vanguard96-default-US"); source == nil || source.Key != "vanguard96-default" || source.Layout != "US" {
		t.Fatalf("shipped Vanguard keyboard source = %#v", source)
	}
	for _, test := range []struct {
		key, serial string
		productType uint16
		actuation   bool
	}{{"vanguard-96-modern", "preview-vanguard-96-modern", common.ProductTypeVanguard96, false}, {"vanguard-96-pro-modern", "preview-vanguard-96-pro-modern", common.ProductTypeVanguard96Pro, true}} {
		fixture, ok := modernDevicePreviewFixtureByKey(test.key)
		if !ok || fixture.ProductType != test.productType {
			t.Fatalf("fixture=%#v ok=%t", fixture, ok)
		}
		summary := fixture.Build()
		if summary == nil || !summary.LegacyLighting || summary.KeyboardAssignments == nil || summary.KeyboardAssignments.LayoutClass != "keyboard-6" || summary.KeyboardAssignments.RowLayoutClass != "keyboard-row-21" || len(summary.KeyboardAssignments.ModifierOptions) < 2 || summary.Performance == nil || summary.Performance.PollingRate == nil || len(summary.Performance.PollingRate.Options) != 8 || len(summary.Performance.BooleanSettings) != 4 || summary.DeviceProfiles == nil || !summary.DeviceProfiles.CanSwitch || !summary.DeviceProfiles.CanSave || !summary.DeviceProfiles.CanDelete || summary.ControlDial == nil || len(summary.ControlDial.Options) != 7 || summary.FlashTap == nil || len(summary.FlashTap.Modes) != 3 || (summary.KeyActuation != nil) != test.actuation {
			t.Fatalf("summary=%#v", summary)
		}
		if summary.SleepTimer != nil || summary.HasBattery || summary.OptionColors != nil || summary.KeyboardAssignments.LiveRGBAvailable || devices.GetDevice(test.serial) != nil {
			t.Fatalf("unsupported control or registration leaked: %#v", summary)
		}
	}
}

func TestVanguard96WirelessModernPreviewsExposeOnlySourceBackedWorkspaces(t *testing.T) {
	initializeLegacyDevicePreviewTestProcess(t)
	keyboards.Init()
	for _, test := range []struct {
		key, serial, row, dial  string
		battery, sleep, polling bool
	}{
		{"vanguard-96-wireless-modern", "preview-vanguard-96-wireless-modern", "keyboard-row-22", "Vertical Scroll", true, true, false},
		{"vanguard-96-usb-modern", "preview-vanguard-96-usb-modern", "keyboard-row-21", "Scroll", true, false, true},
	} {
		fixture, ok := modernDevicePreviewFixtureByKey(test.key)
		if !ok || devices.GetDevice(test.serial) != nil {
			t.Fatalf("fixture=%#v ok=%t", fixture, ok)
		}
		summary := fixture.Build()
		if summary == nil || !summary.LegacyLighting || summary.KeyboardAssignments == nil || summary.KeyboardAssignments.RowLayoutClass != test.row || len(summary.KeyboardAssignments.ModifierOptions) < 2 || summary.Performance == nil || len(summary.Performance.BooleanSettings) != 4 || (summary.Performance.PollingRate != nil) != test.polling || summary.DeviceProfiles == nil || !summary.DeviceProfiles.CanSwitch || !summary.DeviceProfiles.CanSave || !summary.DeviceProfiles.CanDelete || summary.ControlDial == nil || len(summary.ControlDial.Options) != 7 || summary.ControlDial.Options[2].Label != test.dial || summary.FlashTap == nil || len(summary.FlashTap.Modes) != 3 || (summary.SleepTimer != nil) != test.sleep || summary.HasBattery != test.battery || summary.KeyActuation != nil || summary.OptionColors != nil || summary.KeyboardAssignments.LiveRGBAvailable {
			t.Fatalf("summary=%#v", summary)
		}
		if devices.GetDevice(test.serial) != nil {
			t.Fatal("preview fixture registered hardware")
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
