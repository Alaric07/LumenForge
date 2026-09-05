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
	Key   string
	Title string
	Views []modernDevicePreviewView
	Build func() *devicesWorkspaceSummary
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
	{Key: "commander-duo-modern", Title: "Commander Duo", Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "cooling", Label: "Cooling"}}, Build: buildCommanderDuoModernPreview},
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
		commanderDuoModernPreviewSerial: {
			Serial:      commanderDuoModernPreviewSerial,
			Product:     summary.Product,
			Firmware:    summary.Firmware,
			ProductType: common.ProductTypeCCXT,
			Image:       summary.Image,
		},
	}
	web.Device = summary
	web.BatteryStats = map[string]stats.BatteryStats{}
	web.ModernDevicePreview = modernDevicePreviewNavigationForFixture(fixture, summary.View)
	renderModernDevicePreview(w, "devices.html", web)
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
