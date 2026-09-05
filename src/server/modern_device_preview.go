package server

import (
	"LumenForge/src/common"
	"LumenForge/src/stats"
	"LumenForge/src/templates"
	"net/http"
)

const commanderDuoModernPreviewSerial = "preview-commander-duo-modern"

type modernDevicePreviewView struct {
	ID    string
	Label string
}

type modernDevicePreviewFixture struct {
	Key         string
	Title       string
	ProductType uint16
	DeviceType  uint32
	Views       []modernDevicePreviewView
	Build       func() *devicesWorkspaceSummary
}

type modernDevicePreviewNavigationView struct {
	Label  string
	Href   string
	Active bool
}

type modernDevicePreviewNavigation struct {
	Views []modernDevicePreviewNavigationView
}

var modernDevicePreviewFixtures = []modernDevicePreviewFixture{
	{Key: "commander-duo-modern", Title: "Commander Duo", ProductType: common.ProductTypeCCXT, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "cooling", Label: "Cooling"}}, Build: buildCommanderDuoModernPreview},
	{Key: "commander-pro-modern", Title: "Commander Pro", ProductType: common.ProductTypeCPro, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "cooling", Label: "Cooling"}}, Build: buildCommanderProModernPreview},
	{Key: "harpoon-rgb-pro-modern", Title: "Harpoon RGB Pro", ProductType: common.ProductTypeHarpoonRgbPro, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildHarpoonRGBProModernPreview},
	{Key: "glaive-rgb-pro-modern", Title: "Glaive RGB Pro", ProductType: common.ProductTypeGlaiveRgbPro, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildGlaiveRGBProModernPreview},
	{Key: "glaive-rgb-modern", Title: "Glaive RGB", ProductType: common.ProductTypeGlaiveRgb, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildGlaiveRGBModernPreview},
	{Key: "m65-rgb-elite-modern", Title: "M65 RGB Elite", ProductType: common.ProductTypeM65RgbElite, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildM65RGBEliteModernPreview},
	{Key: "sabre-rgb-pro-modern", Title: "Sabre RGB Pro", ProductType: common.ProductTypeSabreRgbPro, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildSabreRGBProModernPreview},
	{Key: "nightsword-rgb-modern", Title: "Nightsword RGB", ProductType: common.ProductTypeNightswordRgb, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildNightswordRGBModernPreview},
	{Key: "ironclaw-rgb-modern", Title: "Ironclaw RGB", ProductType: common.ProductTypeIronClawRgb, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildIronclawRGBModernPreview},
	{Key: "katar-pro-modern", Title: "Katar Pro", ProductType: common.ProductTypeKatarPro, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildKatarProModernPreview},
	{Key: "katar-pro-xt-modern", Title: "Katar Pro XT", ProductType: common.ProductTypeKatarProXT, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildKatarProXTModernPreview},
}

func modernDevicePreviewFixtureByKey(key string) (modernDevicePreviewFixture, bool) {
	for _, fixture := range modernDevicePreviewFixtures {
		if fixture.Key == key {
			return fixture, true
		}
	}
	return modernDevicePreviewFixture{}, false
}

// uiModernDevicePreview renders fixture-backed modern workspace data. It does
// not create or register a hardware device, and its CSP disables interaction.
func uiModernDevicePreview(w http.ResponseWriter, r *http.Request) {
	key, valid := getVar("/dev/device-preview/", r)
	if !valid {
		http.NotFound(w, r)
		return
	}
	fixture, ok := modernDevicePreviewFixtureByKey(key)
	if !ok {
		http.NotFound(w, r)
		return
	}

	summary := fixture.Build()
	summary.View = devicesWorkspaceView(r.URL.Query()["view"], summary)
	web := legacyDevicePreviewWeb()
	web.Page = "devices"
	web.Devices = map[string]*common.Device{
		summary.Serial: {
			Serial:      summary.Serial,
			Product:     summary.Product,
			Firmware:    summary.Firmware,
			ProductType: fixture.ProductType,
			DeviceType:  fixture.DeviceType,
			Image:       summary.Image,
		},
	}
	web.Device = summary
	web.BatteryStats = map[string]stats.BatteryStats{}
	web.ModernDevicePreview = modernDevicePreviewNavigationForFixture(fixture, summary.View)
	renderModernDevicePreview(w, "devices.html", web)
}

func buildCommanderProModernPreview() *devicesWorkspaceSummary {
	summary := &devicesWorkspaceSummary{
		Product: "Commander Pro", Serial: "preview-commander-pro-modern", Firmware: "1.4.17", Image: "icon-device.svg", View: "overview", LegacyLighting: true,
		Cooling:           &devicesCoolingWorkspaceSummary{ProfileOptions: []devicesCoolingProfileOptionSummary{{ID: "Balanced", Label: "Balanced"}, {ID: "Quiet", Label: "Quiet"}, {ID: "Performance", Label: "Performance"}}, Channels: []devicesCoolingChannelSummary{{ID: 0, Name: "Fan Channel 1", Label: "Front intake", RPM: 940, SelectedProfile: "Quiet"}, {ID: 1, Name: "Fan Channel 2", Label: "Radiator fan", RPM: 1380, SelectedProfile: "Balanced"}, {ID: 2, Name: "Fan Channel 3", Label: "Rear exhaust", RPM: 1160, SelectedProfile: "Performance"}}, TemperatureProbes: []devicesCoolingTemperatureProbeSummary{{ID: 6, Name: "Temperature Probe 1", Label: "Coolant", Temperature: "31.4°C"}, {ID: 7, Name: "Temperature Probe 2", Label: "Case", Temperature: "28.8°C"}}},
		DeviceProfiles:    &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"Default", "Quiet", "Studio"}, ActiveProfile: "Default", Scope: "device", Label: "Device Profile", Description: devicesGenericDeviceProfileDescription},
		OverviewCooling:   &devicesOverviewCoolingStatusSummary{Fans: []devicesOverviewStatusRow{{ChannelID: 0, Label: "Front intake", Value: "940 RPM", Telemetry: true}, {ChannelID: 1, Label: "Radiator fan", Value: "1380 RPM", Telemetry: true}, {ChannelID: 2, Label: "Rear exhaust", Value: "1160 RPM", Telemetry: true}}},
		TemperatureProbes: []devicesOverviewStatusRow{{ChannelID: 6, Label: "Coolant", Value: "31.4°C", Telemetry: true}, {ChannelID: 7, Label: "Case", Value: "28.8°C", Telemetry: true}},
		OverviewTelemetry: []devicesOverviewStatusRow{{Label: "+12V", Value: "12.08 V", Telemetry: true}, {Label: "+5V", Value: "5.02 V", Telemetry: true}, {Label: "+3.3V", Value: "3.31 V", Telemetry: true}},
	}
	return summary
}

func modernDevicePreviewNavigationForFixture(fixture modernDevicePreviewFixture, currentView string) modernDevicePreviewNavigation {
	navigation := modernDevicePreviewNavigation{Views: make([]modernDevicePreviewNavigationView, 0, len(fixture.Views))}
	for _, view := range fixture.Views {
		href := "/dev/device-preview/" + fixture.Key
		if view.ID != "overview" {
			href += "?view=" + view.ID
		}
		navigation.Views = append(navigation.Views, modernDevicePreviewNavigationView{Label: view.Label, Href: href, Active: view.ID == currentView})
	}
	return navigation
}

func renderModernDevicePreview(w http.ResponseWriter, name string, data any) {
	applyLegacyDevicePreviewHeaders(w)
	w.Header().Set("Content-Security-Policy", legacyDevicePreviewCSP)
	executeTemplateOrRespond(w, templates.GetTemplate(), name, data, true)
}

func buildCommanderDuoModernPreview() *devicesWorkspaceSummary {
	return &devicesWorkspaceSummary{
		Product:        "iCUE COMMANDER DUO",
		Serial:         commanderDuoModernPreviewSerial,
		Firmware:       "0.9.42",
		Image:          "icon-device.svg",
		View:           "overview",
		LegacyLighting: true,
		Cooling: &devicesCoolingWorkspaceSummary{
			ProfileOptions: []devicesCoolingProfileOptionSummary{
				{ID: "Balanced", Label: "Balanced"},
				{ID: "Quiet", Label: "Quiet"},
				{ID: "Performance", Label: "Performance"},
			},
			Channels: []devicesCoolingChannelSummary{
				{ID: 0, Name: "Fan Channel 1", Label: "Front intake", RPM: 980, SelectedProfile: "Quiet"},
				{ID: 1, Name: "Fan Channel 2", Label: "Radiator pump", RPM: 2240, ContainsPump: true, SelectedProfile: "Performance"},
				{ID: 2, Name: "Fan Channel 3", Label: "Radiator fan", RPM: 1320, SelectedProfile: "Balanced"},
			},
			TemperatureProbes: []devicesCoolingTemperatureProbeSummary{
				{ID: 3, Name: "Temperature Probe 1", Label: "Coolant", Temperature: "31.2°C"},
				{ID: 4, Name: "Temperature Probe 2", Label: "Case exhaust", Temperature: "28.6°C"},
			},
		},
		DeviceProfiles: &devicesDeviceProfileWorkspaceSummary{
			Profiles:      []string{"Default", "Quiet", "Studio"},
			ActiveProfile: "Default",
			Scope:         "device",
			Label:         "Device Profile",
			Description:   devicesCCXTDeviceProfileDescription,
		},
		OverviewCooling: &devicesOverviewCoolingStatusSummary{
			Pumps: []devicesOverviewCoolingPumpSummary{{ChannelID: 1, Label: "Radiator pump", RPM: "2240 RPM"}},
			Fans:  []devicesOverviewStatusRow{{ChannelID: 0, Label: "Front intake", Value: "980 RPM", Telemetry: true}, {ChannelID: 2, Label: "Radiator fan", Value: "1320 RPM", Telemetry: true}},
		},
		TemperatureProbes: []devicesOverviewStatusRow{{ChannelID: 3, Label: "Coolant", Value: "31.2°C", Telemetry: true}, {ChannelID: 4, Label: "Case exhaust", Value: "28.6°C", Telemetry: true}},
	}
}

func buildHarpoonRGBProModernPreview() *devicesWorkspaceSummary {
	regularStages := []devicesDPIStageSummary{
		{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#ff0000"},
		{ID: "1", Name: "Stage 2", DPI: 1500, ColorHex: "#ffa500", Active: true},
		{ID: "2", Name: "Stage 3", DPI: 3000, ColorHex: "#ffff00"},
		{ID: "3", Name: "Stage 4", DPI: 6000, ColorHex: "#00ff00"},
		{ID: "4", Name: "Stage 5", DPI: 9000, ColorHex: "#0000ff"},
	}
	return &devicesWorkspaceSummary{
		Product: "HARPOON RGB PRO", Serial: "preview-harpoon-rgb-pro-modern", Firmware: "1.12.41", Image: "icon-mouse.svg", View: "overview", LegacyLighting: true,
		DPI:                 &devicesDPIWorkspaceSummary{MinimumDPI: 200, MaximumDPI: 12000, ActiveRegularStageID: "1", RegularStages: regularStages, SniperStage: &devicesDPIStageSummary{ID: "5", Name: "Sniper", DPI: 200, ColorHex: "#ffff00", Sniper: true}},
		Performance:         &devicesPerformanceWorkspaceSummary{PollingRate: &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Not Set"}, {Value: 1, Label: "1000 Hz / 1 msec"}, {Value: 2, Label: "500 Hz / 2 msec"}, {Value: 4, Label: "250 Hz / 4 msec"}, {Value: 8, Label: "125 Hz / 8 msec"}}}},
		Buttons:             &devicesButtonsWorkspaceSummary{Buttons: []devicesButtonsButtonSummary{{KeyIndex: 1, Name: "Left Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 8, Name: "Back Button", Default: true}, {KeyIndex: 16, Name: "Forward Button", Default: true}, {KeyIndex: 32, Name: "DPI Button", Default: true}}, AssignmentTypes: []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI"}, {ID: 3, Label: "Keyboard"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}},
		DeviceProfiles:      &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"Default", "FPS"}, ActiveProfile: "Default", Scope: "device", Label: "Device Profile", Description: devicesGenericDeviceProfileDescription},
		OverviewPerformance: &devicesOverviewPerformanceStatusSummary{Rows: []devicesOverviewStatusRow{{Label: "DPI", Value: "1500", Telemetry: true}, {Label: "Active Stage", Value: "Stage 2"}, {Label: "Polling Rate", Value: "1000 Hz / 1 msec", Telemetry: true}}},
	}
}

func buildGlaiveRGBProModernPreview() *devicesWorkspaceSummary {
	return &devicesWorkspaceSummary{
		Product: "GLAIVE RGB PRO", Serial: "preview-glaive-rgb-pro-modern", Firmware: "1.6.19", Image: "icon-mouse.svg", View: "overview", LegacyLighting: true,
		DPI:                 &devicesDPIWorkspaceSummary{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "1", RegularStages: []devicesDPIStageSummary{{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#00ff00"}, {ID: "1", Name: "Stage 2", DPI: 1500, ColorHex: "#00ff00", Active: true}, {ID: "2", Name: "Stage 3", DPI: 3000, ColorHex: "#00ff00"}}, SniperStage: &devicesDPIStageSummary{ID: "3", Name: "Sniper", DPI: 400, ColorHex: "#ffff00", Sniper: true}},
		Performance:         &devicesPerformanceWorkspaceSummary{PollingRate: &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Not Set"}, {Value: 1, Label: "1000 Hz / 1 msec"}, {Value: 2, Label: "500 Hz / 2 msec"}, {Value: 4, Label: "250 Hz / 4 msec"}, {Value: 8, Label: "125 Hz / 8 msec"}}}, AngleSnapping: &devicesPerformanceToggleSummary{Enabled: true}},
		Buttons:             &devicesButtonsWorkspaceSummary{Buttons: []devicesButtonsButtonSummary{{KeyIndex: 1, Name: "Left Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 8, Name: "Back Button", Default: true}, {KeyIndex: 16, Name: "Forward Button", Default: true}, {KeyIndex: 32, Name: "DPI Up", Default: true}, {KeyIndex: 64, Name: "DPI Down", Default: true}}, AssignmentTypes: []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI"}, {ID: 3, Label: "Keyboard"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}},
		DeviceProfiles:      &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"Default", "FPS"}, ActiveProfile: "Default", Scope: "device", Label: "Device Profile", Description: devicesGenericDeviceProfileDescription},
		OverviewPerformance: &devicesOverviewPerformanceStatusSummary{Rows: []devicesOverviewStatusRow{{Label: "DPI", Value: "1500", Telemetry: true}, {Label: "Active Stage", Value: "Stage 2"}, {Label: "Polling Rate", Value: "1000 Hz / 1 msec", Telemetry: true}}},
	}
}

func buildGlaiveRGBModernPreview() *devicesWorkspaceSummary {
	s := buildGlaiveRGBProModernPreview()
	s.Product, s.Serial, s.Firmware = "GLAIVE RGB", "preview-glaive-rgb-modern", "1.2.7"
	s.DPI.MaximumDPI = 16000
	s.DPI.RegularStages = append(s.DPI.RegularStages, devicesDPIStageSummary{ID: "3", Name: "Stage 4", DPI: 6000, ColorHex: "#00ff00"}, devicesDPIStageSummary{ID: "4", Name: "Stage 5", DPI: 9000, ColorHex: "#00ff00"})
	s.DPI.SniperStage.ID, s.DPI.SniperStage.DPI = "5", 200
	s.Performance.ButtonOptimization = &devicesPerformanceSelectSummary{Value: 4, Options: []devicesPerformanceOptionSummary{{Value: 1, Label: "Extreme"}, {Value: 2, Label: "Very Fast"}, {Value: 3, Label: "Fast"}, {Value: 4, Label: "Normal"}}}
	s.Performance.LiftHeight = &devicesPerformanceSelectSummary{Value: 3, Options: []devicesPerformanceOptionSummary{{Value: 2, Label: "Low"}, {Value: 3, Label: "Medium"}, {Value: 4, Label: "High"}}}
	s.Buttons.Buttons = s.Buttons.Buttons[:6]
	s.Buttons.Buttons[5].Name = "DPI Toggle"
	return s
}

func buildM65RGBEliteModernPreview() *devicesWorkspaceSummary {
	return &devicesWorkspaceSummary{Product: "M65 RGB ELITE", Serial: "preview-m65-rgb-elite-modern", Firmware: "1.10.22", Image: "icon-mouse.svg", View: "overview", LegacyLighting: true,
		DPI:            &devicesDPIWorkspaceSummary{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "1", RegularStages: []devicesDPIStageSummary{{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#ff0000"}, {ID: "1", Name: "Stage 2", DPI: 1500, ColorHex: "#ffffff", Active: true}, {ID: "2", Name: "Stage 3", DPI: 3000, ColorHex: "#00ff00"}, {ID: "3", Name: "Stage 4", DPI: 6000, ColorHex: "#800080"}, {ID: "4", Name: "Stage 5", DPI: 9000, ColorHex: "#00bfff"}}, SniperStage: &devicesDPIStageSummary{ID: "5", Name: "Sniper", DPI: 400, ColorHex: "#ffff00", Sniper: true}},
		Performance:    &devicesPerformanceWorkspaceSummary{PollingRate: &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Not Set"}, {Value: 1, Label: "1000 Hz / 1 msec"}, {Value: 2, Label: "500 Hz / 2 msec"}, {Value: 4, Label: "250 Hz / 4 msec"}, {Value: 8, Label: "125 Hz / 8 msec"}}}, ButtonOptimization: &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Disabled"}, {Value: 1, Label: "Enabled"}}}, AngleSnapping: &devicesPerformanceToggleSummary{Enabled: true}},
		Buttons:        &devicesButtonsWorkspaceSummary{Buttons: []devicesButtonsButtonSummary{{KeyIndex: 1, Name: "Left Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 8, Name: "Back Button", Default: true}, {KeyIndex: 16, Name: "Forward Button", Default: true}, {KeyIndex: 32, Name: "DPI Up", Default: true}, {KeyIndex: 64, Name: "DPI Down", Default: true}, {KeyIndex: 128, Name: "Sniper", PressAndHold: true}}, AssignmentTypes: []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI +"}, {ID: 3, Label: "Keyboard"}, {ID: 4, Label: "DPI -"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}},
		DeviceProfiles: &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"Default", "FPS"}, ActiveProfile: "Default", Scope: "device", Label: "Device Profile", Description: devicesGenericDeviceProfileDescription}, OverviewPerformance: &devicesOverviewPerformanceStatusSummary{Rows: []devicesOverviewStatusRow{{Label: "DPI", Value: "1500", Telemetry: true}, {Label: "Active Stage", Value: "Stage 2"}, {Label: "Polling Rate", Value: "1000 Hz / 1 msec", Telemetry: true}}}}
}

func buildSabreRGBProModernPreview() *devicesWorkspaceSummary {
	s := buildM65RGBEliteModernPreview()
	s.Product, s.Serial, s.Firmware = "SABRE RGB PRO", "preview-sabre-rgb-pro-modern", "0.8.15"
	s.Performance.PollingRate = &devicesPerformanceSelectSummary{Value: 7, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Not Set"}, {Value: 1, Label: "125 Hz / 8 msec"}, {Value: 2, Label: "250 Hz / 4 msec"}, {Value: 3, Label: "500 Hz / 2 msec"}, {Value: 4, Label: "1000 Hz / 1 msec"}, {Value: 5, Label: "2000 Hz / 0.5 msec"}, {Value: 6, Label: "4000 Hz / 0.25 msec"}, {Value: 7, Label: "8000 Hz / 0.125 msec"}}}
	s.Buttons.Buttons = s.Buttons.Buttons[:6]
	s.Buttons.Buttons[5].Name = "DPI"
	s.Buttons.AssignmentTypes = []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI"}, {ID: 3, Label: "Keyboard"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}
	return s
}

func buildNightswordRGBModernPreview() *devicesWorkspaceSummary {
	return &devicesWorkspaceSummary{Product: "NIGHTSWORD RGB", Serial: "preview-nightsword-rgb-modern", Firmware: "1.4.12", Image: "icon-mouse.svg", View: "overview", LegacyLighting: true,
		DPI:            &devicesDPIWorkspaceSummary{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "1", RegularStages: []devicesDPIStageSummary{{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#00ff00"}, {ID: "1", Name: "Stage 2", DPI: 1500, ColorHex: "#00ff00", Active: true}, {ID: "2", Name: "Stage 3", DPI: 3000, ColorHex: "#00ff00"}}, SniperStage: &devicesDPIStageSummary{ID: "3", Name: "Sniper", DPI: 200, ColorHex: "#ffff00", Sniper: true}},
		Performance:    &devicesPerformanceWorkspaceSummary{PollingRate: &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Not Set"}, {Value: 1, Label: "1000 Hz / 1 msec"}, {Value: 2, Label: "500 Hz / 2 msec"}, {Value: 4, Label: "250 Hz / 4 msec"}, {Value: 8, Label: "125 Hz / 8 msec"}}}},
		Buttons:        &devicesButtonsWorkspaceSummary{Buttons: []devicesButtonsButtonSummary{{KeyIndex: 1, Name: "Left Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 8, Name: "Back Button", Default: true}, {KeyIndex: 16, Name: "Forward Button", Default: true}, {KeyIndex: 32, Name: "DPI Up", Default: true}, {KeyIndex: 64, Name: "DPI Down", Default: true}, {KeyIndex: 128, Name: "Sniper", PressAndHold: true}, {KeyIndex: 256, Name: "Profile Up", Default: true, ProfileSwitch: true}, {KeyIndex: 512, Name: "Profile Down", Default: true, ProfileSwitch: true}}, AssignmentTypes: []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI"}, {ID: 3, Label: "Keyboard"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}},
		DeviceProfiles: &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"Default", "FPS"}, ActiveProfile: "Default", Scope: "device", Label: "Device Profile", Description: devicesGenericDeviceProfileDescription}, OverviewPerformance: &devicesOverviewPerformanceStatusSummary{Rows: []devicesOverviewStatusRow{{Label: "DPI", Value: "1500", Telemetry: true}, {Label: "Active Stage", Value: "Stage 2"}, {Label: "Polling Rate", Value: "1000 Hz / 1 msec", Telemetry: true}}}}
}

func buildIronclawRGBModernPreview() *devicesWorkspaceSummary {
	s := buildNightswordRGBModernPreview()
	s.Product, s.Serial, s.Firmware = "IRONCLAW RGB", "preview-ironclaw-rgb-modern", "2.1.8"
	s.Buttons.Buttons = []devicesButtonsButtonSummary{{KeyIndex: 1, Name: "Left Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 8, Name: "Back Button", Default: true}, {KeyIndex: 16, Name: "Forward Button", Default: true}, {KeyIndex: 32, Name: "DPI Button", Default: true}, {KeyIndex: 256, Name: "Profile Button", Default: true, ProfileSwitch: true}}
	s.Performance.AngleSnapping = &devicesPerformanceToggleSummary{Enabled: true}
	return s
}

func buildKatarProModernPreview() *devicesWorkspaceSummary {
	return buildKatarModernPreview("KATAR PRO", "preview-katar-pro-modern", "2.4.17", "Stage 2")
}

func buildKatarProXTModernPreview() *devicesWorkspaceSummary {
	return buildKatarModernPreview("KATAR PRO XT", "preview-katar-pro-xt-modern", "3.1.28", "Stage 2")
}

func buildKatarModernPreview(product, serial, firmware, activeStage string) *devicesWorkspaceSummary {
	return &devicesWorkspaceSummary{
		Product: product, Serial: serial, Firmware: firmware, Image: "icon-mouse.svg", View: "overview", LegacyLighting: true,
		DPI:                 &devicesDPIWorkspaceSummary{MinimumDPI: 100, MaximumDPI: 12400, ActiveRegularStageID: "1", RegularStages: []devicesDPIStageSummary{{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#ff0000"}, {ID: "1", Name: activeStage, DPI: 1600, ColorHex: "#00ff00", Active: true}, {ID: "2", Name: "Stage 3", DPI: 3200, ColorHex: "#0000ff"}}, SniperStage: &devicesDPIStageSummary{ID: "3", Name: "Sniper", DPI: 400, ColorHex: "#ffff00", Sniper: true}},
		Performance:         &devicesPerformanceWorkspaceSummary{PollingRate: &devicesPerformanceSelectSummary{Value: 4, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Not Set"}, {Value: 1, Label: "125 Hz / 8 msec"}, {Value: 2, Label: "250 Hz / 4 msec"}, {Value: 3, Label: "500 Hz / 2 msec"}, {Value: 4, Label: "1000 Hz / 1 msec"}}}, ButtonOptimization: &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Disabled"}, {Value: 1, Label: "Enabled"}}}},
		Buttons:             &devicesButtonsWorkspaceSummary{Buttons: []devicesButtonsButtonSummary{{KeyIndex: 1, Name: "Left Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 8, Name: "Forward Button", Default: true}, {KeyIndex: 16, Name: "Back Button", Default: true}, {KeyIndex: 32, Name: "DPI Button", Default: true}}, AssignmentTypes: []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI"}, {ID: 3, Label: "Keyboard"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}},
		DeviceProfiles:      &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"Default", "FPS"}, ActiveProfile: "Default", Scope: "device", Label: "Device Profile", Description: devicesGenericDeviceProfileDescription},
		OverviewPerformance: &devicesOverviewPerformanceStatusSummary{Rows: []devicesOverviewStatusRow{{Label: "DPI", Value: "1600", Telemetry: true}, {Label: "Active Stage", Value: activeStage}, {Label: "Polling Rate", Value: "1000 Hz / 1 msec", Telemetry: true}}},
	}
}
