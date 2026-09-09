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
		{"k60-rgb-pro-modern", "preview-k60-rgb-pro-modern", "keyboard-6", "keyboard-row-26", "1000 Hz / 1 msec", []string{"A"}, 6, 104, true, false, true, false, false},
		{"k65-plus-usb-modern", "preview-k65-plus-usb-modern", "keyboard-6", "keyboard-row-17", "", []string{"A"}, 6, 81, false, true, true, true, false},
		{"k65-pro-mini-modern", "preview-k65-pro-mini-modern", "keyboard-5", "keyboard-row-17", "1000 Hz / 1 msec", []string{"A"}, 5, 67, false, true, true, false, false},
		{"k65-rgb-mini-modern", "preview-k65-rgb-mini-modern", "keyboard-5", "keyboard-row-16", "1000 Hz / 1 msec", []string{"A"}, 5, 61, false, true, true, false, false},
		{"k65-rgb-modern", "preview-k65-rgb-modern", "keyboard-7", "keyboard-row-20", "1000 Hz / 1 msec", []string{"A", "BTS"}, 7, 92, true, false, true, false, false},
		{"k65-rgb-rapidfire-modern", "preview-k65-rgb-rapidfire-modern", "keyboard-7", "keyboard-row-20", "1000 Hz / 1 msec", []string{"A", "BTS"}, 7, 92, true, false, true, false, false},
		{"k68-rgb-modern", "preview-k68-rgb-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", []string{"A", "BTS", "Play"}, 7, 113, true, false, true, false, false},
		{"k70-lux-modern", "preview-k70-lux-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", []string{"A", "BTS", "Play"}, 7, 113, false, false, false, false, false},
		{"k70-lux-rgb-modern", "preview-k70-lux-rgb-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", []string{"A", "BTS", "Play"}, 7, 113, false, false, false, false, false},
		{"k70-rgb-rf-modern", "preview-k70-rgb-rf-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", []string{"A", "BTS", "Play"}, 7, 113, false, false, false, false, false},
		{"k70-mk2-modern", "preview-k70-mk2-modern", "keyboard-7", "keyboard-row-25", "1000 Hz / 1 msec", []string{"A", "BTS", "Play"}, 7, 116, false, false, false, false, false},
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
		recorder = httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+fixture.key))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Device Profile") || !strings.Contains(recorder.Body.String(), "Default") {
			t.Errorf("%s overview omitted device-profile state", fixture.key)
		}
		if fixture.controlDial && !strings.Contains(recorder.Body.String(), "Control Dial") {
			t.Errorf("%s overview omitted control dial", fixture.key)
		}
		if !fixture.sleepTimer && strings.Contains(recorder.Body.String(), "Sleep Timer") {
			t.Errorf("%s overview advertised unsupported sleep timer", fixture.key)
		}
		if devices.GetDevice(fixture.serial) != nil {
			t.Fatalf("fixture serial %q registered during preview", fixture.serial)
		}
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
