package server

import (
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/coolingpresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/devices"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/dpipresentation"
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/performancepresentation"
	"LumenForge/src/rgb"
	"LumenForge/src/stats"
	"LumenForge/src/templates"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const devicesPageHelperEnvironment = "LUMENFORGE_DEVICES_PAGE_TEST_HELPER"

type devicesPageLightingSnapshotProvider struct {
	serial   string
	snapshot lightingpresentation.Snapshot
}

type devicesPageDPISnapshotProvider struct {
	serial   string
	snapshot dpipresentation.Snapshot
}

type devicesPagePerformanceSnapshotProvider struct {
	serial   string
	snapshot performancepresentation.Snapshot
}

type devicesPageKeyboardAssignmentsSnapshotProvider struct {
	serial   string
	snapshot keyboardassignmentspresentation.Snapshot
}

func (provider devicesPageKeyboardAssignmentsSnapshotProvider) KeyboardAssignmentsDeviceID() string {
	return provider.serial
}
func (provider devicesPageKeyboardAssignmentsSnapshotProvider) KeyboardAssignmentsSnapshot() (keyboardassignmentspresentation.Snapshot, bool) {
	return provider.snapshot, true
}

type devicesPageDeviceProfileSnapshotProvider struct {
	serial   string
	snapshot deviceprofilepresentation.Snapshot
}

type devicesPageCoolingSnapshotProvider struct {
	serial   string
	snapshot coolingpresentation.Snapshot
}

func (provider devicesPageCoolingSnapshotProvider) CoolingDeviceID() string { return provider.serial }
func (provider devicesPageCoolingSnapshotProvider) CoolingSnapshot() (coolingpresentation.Snapshot, bool) {
	return provider.snapshot, true
}

func (provider devicesPageDeviceProfileSnapshotProvider) DeviceProfileDeviceID() string {
	return provider.serial
}
func (provider devicesPageDeviceProfileSnapshotProvider) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	return provider.snapshot, true
}

type devicesPageLightingDeviceProfileSnapshotProvider struct {
	serial           string
	lightingSnapshot lightingpresentation.Snapshot
	profileSnapshot  deviceprofilepresentation.Snapshot
}

func (provider devicesPageLightingDeviceProfileSnapshotProvider) LightingDeviceID() string {
	return provider.serial
}
func (provider devicesPageLightingDeviceProfileSnapshotProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return provider.lightingSnapshot, true
}
func (provider devicesPageLightingDeviceProfileSnapshotProvider) DeviceProfileDeviceID() string {
	return provider.serial
}
func (provider devicesPageLightingDeviceProfileSnapshotProvider) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	return provider.profileSnapshot, true
}

type devicesPageKeyboardDeviceProfileSnapshotProvider struct {
	serial           string
	keyboardSnapshot keyboardassignmentspresentation.Snapshot
	profileSnapshot  deviceprofilepresentation.Snapshot
}

func (provider devicesPageKeyboardDeviceProfileSnapshotProvider) KeyboardAssignmentsDeviceID() string {
	return provider.serial
}
func (provider devicesPageKeyboardDeviceProfileSnapshotProvider) KeyboardAssignmentsSnapshot() (keyboardassignmentspresentation.Snapshot, bool) {
	return provider.keyboardSnapshot, true
}
func (provider devicesPageKeyboardDeviceProfileSnapshotProvider) DeviceProfileDeviceID() string {
	return provider.serial
}
func (provider devicesPageKeyboardDeviceProfileSnapshotProvider) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	return provider.profileSnapshot, true
}

func (provider devicesPageDPISnapshotProvider) DPIDeviceID() string {
	return provider.serial
}

func (provider devicesPageDPISnapshotProvider) DPISnapshot() (dpipresentation.Snapshot, bool) {
	return provider.snapshot, true
}

func (provider devicesPagePerformanceSnapshotProvider) PerformanceDeviceID() string {
	return provider.serial
}

func (provider devicesPagePerformanceSnapshotProvider) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	return provider.snapshot, true
}

func (provider devicesPageLightingSnapshotProvider) LightingDeviceID() string {
	return provider.serial
}

func (provider devicesPageLightingSnapshotProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return provider.snapshot, true
}

func TestDevicesWorkspaceKeyboardPresentationAndView(t *testing.T) {
	const serial = "keyboard-assignment-device"
	snapshot := keyboardassignmentspresentation.Snapshot{
		Available:        true,
		LiveRGBAvailable: true, LiveRGBEnabled: true,
		Profiles: []string{"default"}, ActiveProfile: "default", KeyboardLayouts: []string{"US", "UK"}, ActiveKeyboardLayout: "US",
		LayoutClass: "keyboard-8", RowLayoutClass: "keyboard-row-26",
		Rows:            []keyboardassignmentspresentation.Row{{Index: 0, CSS: "keyboard-row-26", Keys: []keyboardassignmentspresentation.Key{{KeyIndex: 11, KeyName: "G1", Width: 1, Height: 1, KeySpace: "keyboard-key wide3", CSS: "top-32", Spacing: []int{0}, KeyEmpty: []string{"keyboard-key-empty"}, Assignable: true, Red: 12.5, Green: 34, Blue: 56}, {KeyIndex: 12, KeyName: "M1", Width: 1, Height: 1, Assignable: false, Red: 0, Green: 255, Blue: 255}}}},
		AssignmentTypes: []keyboardassignmentspresentation.AssignmentType{{ID: 0, Label: "None"}, {ID: 10, Label: "Macro"}},
	}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "Keyboard", Instance: devicesPageKeyboardAssignmentsSnapshotProvider{serial: serial, snapshot: snapshot}}}, map[string]stats.BatteryStats{}, serial)
	if !ok || summary.KeyboardAssignments == nil {
		t.Fatalf("summary = %#v, ok=%t", summary, ok)
	}
	if got := summary.KeyboardAssignments.Rows[0].Keys; got[0].KeyIndex != 11 || got[0].KeySpace != "keyboard-key wide3" || len(got[0].Spacing) != 1 || len(got[0].KeyEmpty) != 1 || !got[0].Assignable || got[1].Assignable {
		t.Errorf("presented keys = %#v", got)
	}
	if got := devicesWorkspaceView([]string{"keyboard"}, summary); got != "keyboard" {
		t.Errorf("view = %q", got)
	}
	if got := devicesWorkspaceView([]string{"key-assignments"}, summary); got != "overview" {
		t.Errorf("retired view = %q", got)
	}
	summary.Performance = &devicesPerformanceWorkspaceSummary{PollingRate: &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 1, Label: "1000 Hz"}}}, BooleanSettings: []devicesPerformanceBooleanSummary{{ID: "perf_winKey", Label: "Disable Win Key"}, {ID: "perf_shiftTab", Label: "Disable Shift + Tab"}, {ID: "perf_altTab", Label: "Disable Alt + Tab"}, {ID: "perf_altF4", Label: "Disable Alt + F4"}}}
	if got := devicesWorkspaceView([]string{"dpi"}, summary); got != "overview" {
		t.Errorf("retired performance view = %q", got)
	}
	var rendered bytes.Buffer
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "devices.html", templates.Web{Devices: map[string]*common.Device{serial: {Serial: serial, Product: "Keyboard"}}, Device: &devicesWorkspaceSummary{Serial: serial, Product: "Keyboard", KeyboardAssignments: summary.KeyboardAssignments, Performance: summary.Performance, DeviceProfiles: &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"default", "studio"}, ActiveProfile: "default"}, View: "keyboard"}, BatteryStats: map[string]stats.BatteryStats{}, Page: "devices"}); err != nil {
		t.Fatal(err)
	}
	body := rendered.String()
	for _, expected := range []string{"data-lf-keyboard-assignments-workspace", "data-lf-keyboard-key", "data-lf-keyboard-color-key", "data-lf-keyboard-editor", "data-lf-keyboard-assignment-close", "keyboard-8", "keyboard-row-26", "keyboard-key wide3", "keyboard-key-empty", "Macro", `data-lf-key-red="12.5"`, `data-lf-normal-color="rgba(12.5, 34, 56, 1)"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q", expected)
		}
	}
	for _, expected := range []string{"data-lf-keyboard-live-rgb checked", "Key Assignments"} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q", expected)
		}
	}
	for _, expected := range []string{"lf-keyboard-settings-card", "data-lf-keyboard-settings-card", "Changes save automatically.", "lf-keyboard-lockouts-card", "data-lf-keyboard-lockouts-card", "Keyboard Settings", "Keyboard Layout", `select id="lf-keyboard-layout" class="lf-buttons-select" data-lf-keyboard-layout`, `option value="US" selected`, "Polling Rate", "Key Lockouts", "Color &amp; Key Assignments", "Color &amp; Assignment Preset", `select id="lf-keyboard-layout-profile" class="lf-buttons-select" data-lf-keyboard-profile`, `option value="default" selected>Working Configuration`, "Save Preset As", "Save Preset", "Delete Preset", `href="/devices?device=keyboard-assignment-device&amp;view=keyboard"`, ">Keyboard</a>"} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q", expected)
		}
	}
	previous := -1
	for _, expected := range []string{"Keyboard Settings", "Key Lockouts", "Color &amp; Key Assignments", "Color &amp; Assignment Preset", "lf-keyboard-visualization", "data-lf-keyboard-color-apply", "data-lf-keyboard-assignment-open"} {
		position := strings.Index(body, expected)
		if position < 0 || position <= previous {
			t.Errorf("keyboard workspace order omitted or misplaced %q", expected)
		}
		previous = position
	}
	if presetForm, keyboard := strings.Index(body, "data-lf-keyboard-profile-dialog"), strings.Index(body, "lf-keyboard-visualization"); presetForm < 0 || keyboard < 0 || presetForm >= keyboard {
		t.Error("keyboard preset form did not render below preset controls and above the keyboard visualization")
	}
	if count := strings.Count(body, "data-lf-device-profiles-workspace"); count != 0 {
		t.Errorf("device profile workspace count = %d, want 0", count)
	}
	for _, removed := range []string{">Performance</a>", ">Key Assignments</a>", "Save Performance Settings", "Save Key Lockouts", "Device Profile", "Save Device Profile As", "Delete Device Profile"} {
		if strings.Contains(body, removed) {
			t.Errorf("keyboard workspace retained %q", removed)
		}
	}
	for _, expected := range []string{"data-lf-keyboard-profile-save disabled", "data-lf-keyboard-profile-delete disabled"} {
		if !strings.Contains(body, expected) {
			t.Errorf("default profile did not disable %q", expected)
		}
	}
	for _, forbidden := range []string{"data-lf-keyboard-assignment-open disabled", "data-lf-keyboard-color value=\"#ffffff\" disabled", "data-lf-keyboard-color-scope disabled", "data-lf-keyboard-color-apply disabled"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("default profile incorrectly disabled %q", forbidden)
		}
	}
	for _, obsolete := range []string{"--lf-key-width", "--lf-key-height", "--lf-key-left", "--lf-key-top", "--lf-row-top"} {
		if strings.Contains(body, obsolete) {
			t.Errorf("keyboard workspace rendered raw coordinate layout %q", obsolete)
		}
	}
	styles, err := os.ReadFile(filepath.Join("..", "..", "static", "css", "app-shell.css"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"repeat(26, var(--lf-kb-key-width))", "--lf-kb-column-gap", ".lf-keyboard-placeholder", "button.lf-keyboard-key-static { opacity: 1; cursor: pointer; }"} {
		if !strings.Contains(string(styles), expected) {
			t.Errorf("keyboard grid CSS missing %q", expected)
		}
	}
	for _, obsolete := range []string{"flex: var(--lf-key-width)", "margin-left: calc(var(--lf-key-left)", "top: calc(var(--lf-key-top)"} {
		if strings.Contains(string(styles), obsolete) {
			t.Errorf("keyboard grid CSS retained raw layout %q", obsolete)
		}
	}
}

func TestDevicesOverviewKeyboardDeviceProfilePresentation(t *testing.T) {
	const serial = "k95-device-profile"
	snapshot := deviceprofilepresentation.Snapshot{Supported: true, Profiles: []string{"default", "studio"}, ActiveProfile: "default"}
	keyboardSnapshot := keyboardassignmentspresentation.Snapshot{Available: true, Profiles: []string{"default"}, ActiveProfile: "default", Rows: []keyboardassignmentspresentation.Row{{Keys: []keyboardassignmentspresentation.Key{{KeyName: "A", Width: 1, Height: 1}}}}, AssignmentTypes: []keyboardassignmentspresentation.AssignmentType{{Label: "None"}}}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K95 RGB Platinum", Instance: devicesPageKeyboardDeviceProfileSnapshotProvider{serial: serial, keyboardSnapshot: keyboardSnapshot, profileSnapshot: snapshot}}}, map[string]stats.BatteryStats{}, serial)
	if !ok || summary.DeviceProfiles == nil || summary.DeviceProfiles.ActiveProfile != "default" || len(summary.DeviceProfiles.ProfileDisplayLabels) != 0 || summary.DeviceProfiles.Description != devicesKeyboardDeviceProfileDescription {
		t.Fatalf("summary = %#v, ok=%t", summary, ok)
	}
	var rendered bytes.Buffer
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "devices.html", templates.Web{Devices: map[string]*common.Device{serial: {Serial: serial, Product: "K95 RGB Platinum"}}, Device: summary, BatteryStats: map[string]stats.BatteryStats{}, Page: "devices"}); err != nil {
		t.Fatal(err)
	}
	body := rendered.String()
	for _, expected := range []string{"lf-overview-workspace", "data-lf-device-profiles-workspace", "Device Profile", devicesKeyboardDeviceProfileDescription, "Save Device Profile As", "Device Profile to delete", "Delete Device Profile", `select id="lf-device-profile" class="lf-buttons-select" data-lf-device-profile`, `option value="default" selected>default`, `option value="studio"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q", expected)
		}
	}
}

func TestDevicesOverviewScimitarEliteDeviceProfilePresentation(t *testing.T) {
	const serial = "scimitar-elite-device-profile"
	snapshot := deviceprofilepresentation.Snapshot{Supported: true, Profiles: []string{"default", "studio"}, ActiveProfile: "default"}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "SCIMITAR RGB ELITE", ProductType: common.ProductTypeScimitarRgbElite, Instance: devicesPageDeviceProfileSnapshotProvider{serial: serial, snapshot: snapshot}}}, map[string]stats.BatteryStats{}, serial)
	if !ok || summary.DeviceProfiles == nil || summary.DeviceProfiles.ActiveProfile != "default" || summary.DeviceProfiles.Description != devicesScimitarEliteDeviceProfileDescription {
		t.Fatalf("summary = %#v, ok=%t", summary, ok)
	}
	var rendered bytes.Buffer
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "devices.html", templates.Web{Devices: map[string]*common.Device{serial: {Serial: serial, Product: "SCIMITAR RGB ELITE", ProductType: common.ProductTypeScimitarRgbElite}}, Device: summary, BatteryStats: map[string]stats.BatteryStats{}, Page: "devices"}); err != nil {
		t.Fatal(err)
	}
	body := rendered.String()
	for _, expected := range []string{"lf-overview-workspace", "data-lf-device-profiles-workspace", "Device Profile", devicesScimitarEliteDeviceProfileDescription, "Save Device Profile As", "Device Profile to delete", "Delete Device Profile", `select id="lf-device-profile" class="lf-buttons-select" data-lf-device-profile`, `option value="default" selected>default`, `option value="studio"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q", expected)
		}
	}
}

func TestDevicesCCXTModernCoolingWorkspaceAndFullProfile(t *testing.T) {
	const serial = "ccxt-modern-workspace"
	profile := deviceprofilepresentation.Snapshot{Supported: true, Profiles: []string{"default", "studio"}, ActiveProfile: "default"}
	cooling := coolingpresentation.Snapshot{Available: true, Channels: []coolingpresentation.Channel{{ID: 1, Name: "Fan 1", Label: "Front", RPM: 1040, SelectedProfile: "quiet"}}, ProfileOptions: []coolingpresentation.ProfileOption{{ID: "quiet", Label: "quiet"}}, TemperatureProbes: []coolingpresentation.TemperatureProbe{{ID: 2, Name: "Probe 1", Label: "Coolant", Temperature: "30.0°C"}}}
	instance := struct {
		devicesPageDeviceProfileSnapshotProvider
		devicesPageCoolingSnapshotProvider
	}{devicesPageDeviceProfileSnapshotProvider{serial: serial, snapshot: profile}, devicesPageCoolingSnapshotProvider{serial: serial, snapshot: cooling}}
	device := &common.Device{Serial: serial, Product: "iCUE COMMANDER CORE XT", ProductType: common.ProductTypeCCXT, Instance: instance}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: device}, map[string]stats.BatteryStats{}, serial)
	if !ok || summary.Cooling == nil || summary.DeviceProfiles == nil || summary.DeviceProfiles.Description != devicesCCXTDeviceProfileDescription || !summary.LegacyLighting {
		t.Fatalf("summary = %#v, ok=%t", summary, ok)
	}
	if summary.DeviceProfiles.ProfileDisplayLabels != nil {
		t.Fatalf("default profile was relabeled: %#v", summary.DeviceProfiles.ProfileDisplayLabels)
	}
	summary.View = devicesWorkspaceView([]string{"cooling"}, summary)
	var rendered bytes.Buffer
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "devices.html", templates.Web{Devices: map[string]*common.Device{serial: device}, Device: summary, BatteryStats: map[string]stats.BatteryStats{}, Page: "devices"}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Overview", "Cooling", "Lighting", "data-lf-cooling-workspace", "Front", "1040 RPM", "quiet", "Temperature probes", "30.0°C"} {
		if !strings.Contains(rendered.String(), expected) {
			t.Errorf("missing %q", expected)
		}
	}
}

func TestDevicesLightingProfilePresentation(t *testing.T) {
	const serial = "mm800-lighting-profile"
	profileSnapshot := deviceprofilepresentation.Snapshot{Supported: true, Scope: deviceprofilepresentation.ScopeLighting, Profiles: []string{"default", "studio"}, ActiveProfile: "default", DefaultProfileDisplayLabel: deviceprofilepresentation.WorkingConfigurationLabel}
	mousepadSnapshot := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mousepad", EffectSupported: true, HasBrightness: true, Brightness: 72,
		AuthoredZoneEditor: &lightingpresentation.AuthoredZoneEditor{EffectID: "mousepad", Heading: "Zones", Description: "Select one or more zones, choose a color, then apply it to the selected zones.", Zones: []lightingpresentation.AuthoredZone{{ID: "1", Label: "Zone 1", ColorHex: "#102030"}}}}
	device := &common.Device{Serial: serial, Product: "MM800", ProductType: common.ProductTypeMM800, Instance: devicesPageLightingDeviceProfileSnapshotProvider{serial: serial, lightingSnapshot: mousepadSnapshot, profileSnapshot: profileSnapshot}}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: device}, map[string]stats.BatteryStats{}, serial)
	if !ok || summary.DeviceProfiles == nil || summary.DeviceProfiles.Scope != deviceprofilepresentation.ScopeLighting || summary.DeviceProfiles.ProfileDisplayLabels["default"] != deviceprofilepresentation.WorkingConfigurationLabel || summary.DeviceProfiles.Label != "Lighting Profile" || summary.DeviceProfiles.Description != devicesLightingProfileDescription {
		t.Fatalf("summary = %#v, ok=%t", summary, ok)
	}

	var rendered bytes.Buffer
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "devices.html", templates.Web{Devices: map[string]*common.Device{serial: device}, Device: &devicesWorkspaceSummary{Product: device.Product, Serial: serial, Lighting: summary.Lighting, DeviceProfiles: summary.DeviceProfiles, View: "lighting"}, BatteryStats: map[string]stats.BatteryStats{}, Page: "devices"}); err != nil {
		t.Fatal(err)
	}
	body := rendered.String()
	for _, expected := range []string{"Lighting Profile", devicesLightingProfileDescription, "Active Lighting Profile", "Save Lighting Profile As", "Lighting Profile to delete", "Delete Lighting Profile", `data-lf-device-profiles-workspace`, `data-lf-device-profile-label="Lighting Profile"`, `select id="lf-device-profile" class="lf-buttons-select" data-lf-device-profile`, `option value="default" selected>Working Configuration`, `option value="studio"`, "<h3>Zones</h3>", "Select one or more zones, choose a color, then apply it to the selected zones."} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q", expected)
		}
	}
	for _, removed := range []string{"Authored zones", "<h2>Device Profile</h2>"} {
		if strings.Contains(body, removed) {
			t.Errorf("MM800 lighting retained %q", removed)
		}
	}
	zonesAt, profileAt, ownershipAt := strings.Index(body, "<h3>Zones</h3>"), strings.Index(body, "<h2>Lighting Profile</h2>"), strings.Index(body, `class="lf-lighting-ownership-panel"`)
	if zonesAt < 0 || profileAt <= zonesAt || ownershipAt <= profileAt {
		t.Errorf("MM800 lighting profile order = zones:%d profile:%d ownership:%d", zonesAt, profileAt, ownershipAt)
	}

	rendered.Reset()
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "devices.html", templates.Web{Devices: map[string]*common.Device{serial: device}, Device: &devicesWorkspaceSummary{Product: device.Product, Serial: serial, Lighting: summary.Lighting, DeviceProfiles: summary.DeviceProfiles, View: "overview"}, BatteryStats: map[string]stats.BatteryStats{}, Page: "devices"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered.String(), "data-lf-device-profiles-workspace") {
		t.Fatal("MM800 lighting profile rendered on Overview")
	}

	for _, effect := range []string{"static", "gradient"} {
		lighting := devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: effect, EffectSupported: true})
		rendered.Reset()
		if err := templates.GetTemplate().ExecuteTemplate(&rendered, "devices.html", templates.Web{Devices: map[string]*common.Device{serial: device}, Device: &devicesWorkspaceSummary{Product: device.Product, Serial: serial, Lighting: lighting, DeviceProfiles: summary.DeviceProfiles, View: "lighting"}, BatteryStats: map[string]stats.BatteryStats{}, Page: "devices"}); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(rendered.String(), "data-lf-device-profiles-workspace") {
			t.Errorf("Lighting Profile rendered for generic effect %q", effect)
		}
	}

	rendered.Reset()
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "devices.html", templates.Web{Devices: map[string]*common.Device{serial: device}, Device: &devicesWorkspaceSummary{Product: device.Product, Serial: serial, OpenRGB: &openRGBWorkspaceSummary{}, Lighting: summary.Lighting, View: "lighting"}, BatteryStats: map[string]stats.BatteryStats{}, Page: "devices"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered.String(), "data-lf-device-profiles-workspace") {
		t.Fatal("OpenRGB presentation gained a profile panel")
	}
}

func TestOpenRGBLegacyTemplateOmitsDuplicateLightingControls(t *testing.T) {
	templateSource, err := os.ReadFile(filepath.Join("..", "..", "web", "openrgb.html"))
	if err != nil {
		t.Fatalf("read OpenRGB template: %v", err)
	}
	for _, obsolete := range []string{
		"txtRgbOverride",
		"rgbOverride",
		"mbBrightnessSlider",
		"mbRgbProfile",
		"mbSpeedProfile",
		"OpenRGBImportEffect",
		"OpenRGBImportSpeed",
		"OpenRGBImportBrightness",
		"function setBrightness(",
		"function setEffect(",
		"function setSpeed(",
	} {
		if strings.Contains(string(templateSource), obsolete) {
			t.Errorf("OpenRGB template still exposes RGB Override marker %q", obsolete)
		}
	}
}

func TestDevicesPageLightingViewAndOverview(t *testing.T) {
	if os.Getenv(devicesPageHelperEnvironment) == "1" {
		runDevicesPageRouteAssertions(t)
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestDevicesPageLightingViewAndOverview$")
	command.Env = append(os.Environ(), devicesPageHelperEnvironment+"=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Devices page helper process failed: %v\n%s", err, output)
	}
}

func requestDevicesPage(t *testing.T, handler http.Handler, rawQuery string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/devices", nil)
	request.URL.RawQuery = rawQuery
	request.Host = "127.0.0.1:27003"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestDevicesLightingPresentationModel(t *testing.T) {
	source := lightingpresentation.Snapshot{TargetKind: "openrgb",
		ConfiguredEffect:  "wave",
		EffectSupported:   true,
		HasBrightness:     true,
		Brightness:        0,
		ClusterControlled: true,
		SupportedEffects: []lightingpresentation.EffectOption{
			{ID: "future-effect", Label: "Future Effect"},
			{
				ID:    "wave",
				Label: "Wave <Label> & More",
			},
		},
		HasSpeed:          true,
		Speed:             0.5,
		PaletteKind:       "Two color",
		SingleColorHex:    "#ff0000",
		TwoColorStartHex:  "#112233",
		TwoColorEndHex:    "#aabbcc",
		HasTemperature:    true,
		TemperatureLow:    lightingpresentation.TemperaturePoint{ColorHex: "#010203", Celsius: 20.5},
		TemperatureMiddle: lightingpresentation.TemperaturePoint{ColorHex: "#040506", Celsius: 50.25},
		TemperatureHigh:   lightingpresentation.TemperaturePoint{ColorHex: "#070809", Celsius: 80.75},
		HasGradient:       true,
		GradientStops: []lightingpresentation.GradientStop{
			{Position: 0, ColorHex: "#112233", Intensity: 0},
			{Position: 1, ColorHex: "#aabbcc", Intensity: 1},
		},
		Customized: true,
	}

	summary := devicesLightingWorkspaceSummaryFromSnapshot(source)
	if summary.ConfiguredEffect != "wave" || summary.ConfiguredEffectLabel != "Wave <Label> & More" ||
		summary.ConfiguredEffectIconURL != "/static/img/icons/rgb/wave.svg" ||
		!summary.EffectSupported || !summary.HasBrightness || summary.Brightness != 0 || !summary.ClusterControlled ||
		!summary.HasSpeedControl || summary.Speed != "0.5" || summary.PaletteKind != "Two color" || summary.SingleColorHex != "#ff0000" ||
		summary.TwoColorStartHex != "#112233" || summary.TwoColorEndHex != "#aabbcc" || !summary.Customized {
		t.Fatalf("configured Lighting presentation = %#v", summary)
	}
	if !summary.HasTemperature || len(summary.TemperaturePoints) != 3 ||
		summary.TemperaturePoints[0] != (openRGBLightingTemperaturePointSummary{Role: "low", Label: "Low", ColorHex: "#010203", Celsius: "20.5"}) ||
		summary.TemperaturePoints[1] != (openRGBLightingTemperaturePointSummary{Role: "middle", Label: "Middle", ColorHex: "#040506", Celsius: "50.25"}) ||
		summary.TemperaturePoints[2] != (openRGBLightingTemperaturePointSummary{Role: "high", Label: "High", ColorHex: "#070809", Celsius: "80.75"}) {
		t.Fatalf("temperature presentation = %#v", summary.TemperaturePoints)
	}
	if !summary.HasGradient || len(summary.GradientStops) != 2 ||
		summary.GradientStops[0] != (openRGBLightingGradientStopSummary{Number: 1, Position: "0", ColorHex: "#112233", Intensity: "0"}) ||
		summary.GradientStops[1] != (openRGBLightingGradientStopSummary{Number: 2, Position: "1", ColorHex: "#aabbcc", Intensity: "1"}) {
		t.Fatalf("Gradient presentation = %#v", summary.GradientStops)
	}
	if len(summary.SupportedEffects) != 2 ||
		summary.SupportedEffects[0].ID != "future-effect" || summary.SupportedEffects[0].Label != "Future Effect" || summary.SupportedEffects[0].Selected ||
		summary.SupportedEffects[1].ID != "wave" || summary.SupportedEffects[1].Label != "Wave <Label> & More" || !summary.SupportedEffects[1].Selected {
		t.Fatalf("supported Lighting effects = %#v", summary.SupportedEffects)
	}

	source.SupportedEffects[0].ID = "mutated"
	if summary.SupportedEffects[0].ID != "future-effect" {
		t.Fatal("Lighting presentation retained mutable snapshot data")
	}
}

func TestDevicesLightingAuthoredZoneEditorPresentation(t *testing.T) {
	initializeDevicesPageTestProcess(t)

	without := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "static", EffectSupported: true}))
	if strings.Contains(without, "data-lf-authored-zone-control") {
		t.Fatal("authored-zone editor rendered without snapshot data")
	}

	nonGeometric := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true,
		AuthoredZoneEditor: &lightingpresentation.AuthoredZoneEditor{EffectID: "mouse", Heading: "Zones", Description: "Select one or more zones, choose a color, then apply it to the selected zones.", Zones: []lightingpresentation.AuthoredZone{{ID: "front", Label: "Front", ColorHex: "#102030"}}}}
	nonGeometricBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(nonGeometric))
	for _, want := range []string{`data-lf-authored-zone-control`, `data-lf-effect="mouse"`, `data-lf-zone-id="front"`, `aria-pressed="false"`, `#102030`, `Clear selection`, `data-lf-authored-zone-clear`, `Selected zones`, `data-lf-authored-zone-apply="zones"`, `data-lf-authored-zone-apply="all"`, `<h3>Zones</h3>`, `Select one or more zones, choose a color, then apply it to the selected zones.`} {
		if !strings.Contains(nonGeometricBody, want) {
			t.Errorf("non-geometric editor omitted %q", want)
		}
	}
	clearAt := strings.Index(nonGeometricBody, `data-lf-authored-zone-clear`)
	if clearAt < 0 {
		t.Fatal("authored-zone clear selection control is missing")
	}
	clearEnd := strings.Index(nonGeometricBody[clearAt:], ">")
	if clearEnd < 0 || !strings.Contains(nonGeometricBody[clearAt:clearAt+clearEnd], "disabled") {
		t.Fatal("authored-zone clear selection control is not initially disabled")
	}
	if strings.Contains(nonGeometricBody, `data-lf-authored-zone-apply="group"`) || strings.Contains(nonGeometricBody, "--lf-authored-zone-left") {
		t.Fatalf("non-geometric editor manufactured group/geometry metadata:\n%s", nonGeometricBody)
	}
	if strings.Contains(nonGeometricBody, "Authored zones") {
		t.Fatal("Scimitar authored-zone editor retained its obsolete heading")
	}

	geometric := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mousepad", EffectSupported: true,
		AuthoredZoneEditor: &lightingpresentation.AuthoredZoneEditor{EffectID: "mousepad", HasGroups: true, Zones: []lightingpresentation.AuthoredZone{{ID: "1", Label: "One", ColorHex: "#aabbcc", GroupID: "row-1", GroupLabel: "Row 1", HasGeometry: true, Left: 1, Top: 2, Width: 3, Height: 4}}}}
	geometricBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(geometric))
	for _, want := range []string{`lf-authored-zone-list-geometric`, `--lf-authored-zone-layout-width: 4`, `--lf-authored-zone-layout-height: 6`, `data-lf-zone-id="1"`, `data-lf-group-id="row-1"`, `--lf-authored-zone-left: 1`, `--lf-authored-zone-top: 2`, `--lf-authored-zone-width: 3`, `--lf-authored-zone-height: 4`, `Selected group`, `data-lf-authored-zone-apply="group"`, `selected zones, their group, or all zones`, `#aabbcc`} {
		if !strings.Contains(geometricBody, want) {
			t.Errorf("geometric editor omitted %q", want)
		}
	}
	if strings.Contains(strings.ToLower(geometricBody), "mm800") || strings.Contains(strings.ToLower(geometricBody), "scimitar") {
		t.Fatal("authored-zone template contains product-specific rendering")
	}

	readOnly := geometric
	readOnly.ClusterControlled = true
	readOnlyBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(readOnly))
	for _, marker := range []string{`<button type="button" class="lf-button lf-button-secondary lf-authored-zone" data-lf-authored-zone `, `data-lf-authored-zone-clear`, `data-lf-authored-zone-color`, `data-lf-authored-zone-apply="zones"`} {
		at := strings.Index(readOnlyBody, marker)
		if at < 0 {
			t.Fatalf("read-only editor omitted %q", marker)
		}
		end := strings.Index(readOnlyBody[at:], ">")
		if end < 0 || !strings.Contains(readOnlyBody[at:at+end], "disabled") {
			t.Errorf("read-only authored control %q is not disabled", marker)
		}
	}
}

func TestDevicesWorkspaceDPIPresentationAndViews(t *testing.T) {
	initializeDevicesPageTestProcess(t)
	serial := "scimitar-elite-dpi"
	snapshot := dpipresentation.Snapshot{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "1", Stages: []dpipresentation.Stage{
		{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#102030"},
		{ID: "1", Name: "Stage 2", DPI: 1600, ColorHex: "#aabbcc", Active: true},
		{ID: "2", Name: "Stage 3", DPI: 2400, ColorHex: "#102030"},
		{ID: "3", Name: "Stage 4", DPI: 3200, ColorHex: "#102030"},
		{ID: "4", Name: "Stage 5", DPI: 4000, ColorHex: "#102030"},
		{ID: "5", Name: "Sniper", DPI: 400, ColorHex: "#ffaa00", Sniper: true},
	}}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{
		serial: {Serial: serial, Product: "SCIMITAR RGB ELITE", Instance: devicesPageDPISnapshotProvider{serial: serial, snapshot: snapshot}},
	}, map[string]stats.BatteryStats{}, serial)
	if !ok || summary == nil || summary.DPI == nil || summary.Lighting != nil || summary.DPI.MinimumDPI != 100 || summary.DPI.MaximumDPI != 18000 || summary.DPI.ActiveRegularStageID != "1" || len(summary.DPI.RegularStages) != 5 || summary.DPI.SniperStage == nil {
		t.Fatalf("DPI Devices summary = %#v, ok=%t", summary, ok)
	}
	if summary.DPI.RegularStages[1].ColorHex != "#aabbcc" || !summary.DPI.RegularStages[1].Active || summary.DPI.SniperStage.ID != "5" || !summary.DPI.SniperStage.Sniper || summary.DPI.SniperStage.ColorHex != "#ffaa00" {
		t.Fatalf("DPI stages = %#v / %#v", summary.DPI.RegularStages, summary.DPI.SniperStage)
	}
	body := renderDevicesDPIView(t, summary.DPI)
	for _, want := range []string{"Stage 2", "Sniper", "lf-dpi-stage-sniper", "data-lf-dpi-slider", "data-lf-dpi-number", "data-lf-dpi-color", "data-lf-dpi-save", `min="100"`, `max="18000"`, "#ffaa00"} {
		if !strings.Contains(body, want) {
			t.Errorf("DPI template omitted %q", want)
		}
	}
	for _, unwanted := range []string{"DPI stages", "Regular stages", "Current device profile", "Read-only snapshot", "Stage ID", "100–18000 DPI"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("DPI template retained %q", unwanted)
		}
	}
	previous := -1
	for _, name := range []string{"Stage 1", "Stage 2", "Stage 3", "Stage 4", "Stage 5", "Sniper"} {
		position := strings.Index(body, name)
		if position < 0 || position <= previous {
			t.Errorf("DPI stage order does not include %q in sequence", name)
		}
		previous = position
	}
	if strings.Contains(body, "<select") {
		t.Error("DPI template exposed an unexpected selector")
	}
	if got := devicesWorkspaceView([]string{"dpi"}, summary); got != "dpi" {
		t.Errorf("DPI-capable workspace view = %q, want dpi", got)
	}
	if got := devicesWorkspaceView([]string{"lighting"}, summary); got != "overview" {
		t.Errorf("DPI-only Lighting view = %q, want overview", got)
	}
	if got := devicesWorkspaceView([]string{"dpi"}, &devicesWorkspaceSummary{}); got != "overview" {
		t.Errorf("unsupported DPI view = %q, want overview", got)
	}
	if got := devicesWorkspaceView([]string{"dpi", "dpi"}, summary); got != "overview" {
		t.Errorf("duplicate DPI view = %q, want overview", got)
	}
	if got := devicesWorkspaceView([]string{"lighting"}, &devicesWorkspaceSummary{Lighting: &devicesLightingWorkspaceSummary{}}); got != "lighting" {
		t.Errorf("existing Lighting view = %q, want lighting", got)
	}
}

func TestDevicesWorkspacePerformancePresentationAndViews(t *testing.T) {
	initializeDevicesPageTestProcess(t)
	serial := "performance-device"
	performance := performancepresentation.Snapshot{
		PollingRate:   &performancepresentation.SelectSetting{Value: 2, Options: []performancepresentation.Option{{Value: 1, Label: "1000 Hz"}, {Value: 2, Label: "500 Hz"}}},
		AngleSnapping: &performancepresentation.ToggleSetting{Enabled: true},
		LiftHeight:    &performancepresentation.SelectSetting{Value: 3, Options: []performancepresentation.Option{{Value: 2, Label: "Low"}, {Value: 3, Label: "Medium"}}},
		BooleanSettings: []performancepresentation.BooleanSetting{
			{ID: "custom-one", Label: "Custom One", Enabled: true},
			{ID: "custom-two", Label: "Custom Two"},
		},
	}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{
		serial: {Serial: serial, Product: "Performance device", Instance: devicesPagePerformanceSnapshotProvider{serial: serial, snapshot: performance}},
	}, map[string]stats.BatteryStats{}, serial)
	if !ok || summary == nil || summary.DPI != nil || summary.Performance == nil || summary.Performance.PollingRate == nil || summary.Performance.AngleSnapping == nil || summary.Performance.LiftHeight == nil || len(summary.Performance.BooleanSettings) != 2 {
		t.Fatalf("Performance Devices summary = %#v, ok=%t", summary, ok)
	}
	if got := summary.Performance.BooleanSettings; got[0].ID != "custom-one" || got[1].ID != "custom-two" || !got[0].Enabled || got[1].Enabled {
		t.Errorf("generic BooleanSettings = %#v, want copied provider order and values", got)
	}
	if got := devicesWorkspaceView([]string{"dpi"}, summary); got != "dpi" {
		t.Errorf("Performance-only workspace view = %q, want dpi", got)
	}
	body := renderDevicesPerformanceView(t, nil, summary.Performance)
	for _, want := range []string{"Performance", "Performance Settings", "Polling Rate", "Angle Snapping", "Lift Height", "Custom One", "Custom Two", `data-lf-performance-kind="pollingRate"`, `data-lf-performance-kind="angleSnapping"`, `data-lf-performance-kind="liftHeight"`, `data-lf-performance-setting-id="custom-one"`, `data-lf-performance-setting-id="custom-two"`, `data-lf-buttons-toast`} {
		if !strings.Contains(body, want) {
			t.Errorf("Performance template omitted %q", want)
		}
	}
	if strings.Contains(body, "Mouse Settings") {
		t.Errorf("Performance template rendered an obsolete or unsupported control: %s", body)
	}
	if strings.Contains(body, "Saving…") || strings.Contains(body, "Saved.") {
		t.Errorf("Performance template rendered inline success feedback: %s", body)
	}
	unsupported := renderDevicesPerformanceView(t, nil, &devicesPerformanceWorkspaceSummary{AngleSnapping: &devicesPerformanceToggleSummary{Enabled: false}})
	for _, unwanted := range []string{"Polling Rate", "Lift Height"} {
		if strings.Contains(unsupported, unwanted) {
			t.Errorf("unsupported Performance control rendered %q", unwanted)
		}
	}
}

func TestDevicesPerformanceBooleanSettingsFailClosedWhenMalformed(t *testing.T) {
	for _, snapshot := range []performancepresentation.Snapshot{
		{BooleanSettings: []performancepresentation.BooleanSetting{{ID: "", Label: "Missing ID"}}},
		{BooleanSettings: []performancepresentation.BooleanSetting{{ID: "missing-label", Label: ""}}},
		{BooleanSettings: []performancepresentation.BooleanSetting{{ID: "duplicate", Label: "First"}, {ID: "duplicate", Label: "Second"}}},
	} {
		summary := devicesPerformanceWorkspaceSummaryFromSnapshot(snapshot)
		if summary != nil {
			t.Errorf("malformed BooleanSettings produced summary %#v", summary)
		}
	}
}

func TestDevicesWorkspacePerformanceKeepsDPIEditor(t *testing.T) {
	initializeDevicesPageTestProcess(t)
	dpi := &devicesDPIWorkspaceSummary{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "0", RegularStages: []devicesDPIStageSummary{{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#102030", Active: true}}, SniperStage: &devicesDPIStageSummary{ID: "5", Name: "Sniper", DPI: 400, ColorHex: "#aabbcc", Sniper: true}}
	performance := &devicesPerformanceWorkspaceSummary{AngleSnapping: &devicesPerformanceToggleSummary{Enabled: true}}
	body := renderDevicesPerformanceView(t, dpi, performance)
	if !strings.Contains(body, ">DPI</h2>") || !strings.Contains(body, "data-lf-dpi-workspace") || !strings.Contains(body, "Performance Settings") {
		t.Fatalf("combined Performance view omitted existing DPI editor: %s", body)
	}
	if !strings.Contains(body, `href="/devices?device=performance-template-device&amp;view=dpi"`) ||
		!strings.Contains(body, `>Performance</a>`) || strings.Contains(body, `>DPI</a>`) {
		t.Fatalf("DPI-capable workspace did not expose Performance navigation: %s", body)
	}
}

func TestDevicesPerformanceRoutesUseExistingRequestPayloads(t *testing.T) {
	router := setRoutes()
	for _, route := range []struct {
		path string
		body string
	}{
		{path: "/api/devices/performance/polling-rate", body: `{"deviceId":"missing-performance-device","pollingRate":2}`},
		{path: "/api/devices/performance/angle-snapping", body: `{"deviceId":"missing-performance-device","angleSnapping":1}`},
		{path: "/api/devices/performance/lift-height", body: `{"deviceId":"missing-performance-device","liftHeight":3}`},
		{path: "/api/devices/performance/keyboard", body: `{"deviceId":"missing-performance-device","perf_winKey":true,"perf_shiftTab":false,"perf_altTab":true,"perf_altF4":false}`},
		{path: "/api/keyboard/setPerformance", body: `{"deviceId":"missing-performance-device","perf_winKey":true,"perf_shiftTab":false,"perf_altTab":true,"perf_altF4":false}`},
	} {
		t.Run(route.path, func(t *testing.T) {
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, route.path, route.body)
			if recorder.Code != http.StatusOK {
				t.Fatalf("shared Performance route HTTP status = %d", recorder.Code)
			}
			var response Response
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("decode shared Performance route response: %v", err)
			}
			if response.Code != http.StatusOK || response.Status != 0 {
				t.Fatalf("shared Performance route response = %#v", response)
			}
		})
	}
}

func TestDevicesLightingOwnershipControlsFollowTargetKind(t *testing.T) {
	native := devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "native"})
	nativeBody := renderDevicesLightingView(t, native)
	for _, want := range []string{"RGB Cluster", "OpenRGB Integration", `data-lf-ownership-kind="cluster"`, `data-lf-ownership-kind="openrgb-integration"`} {
		if !strings.Contains(nativeBody, want) {
			t.Errorf("native Lighting omitted %q", want)
		}
	}
	nativeOwnership := strings.Index(nativeBody, `class="lf-lighting-ownership-panel"`)
	nativeMain := strings.Index(nativeBody, `class="lf-lighting-main"`)
	if nativeOwnership < nativeMain || nativeMain < 0 {
		t.Error("native Lighting ownership did not render in the main content column")
	}
	if strings.Contains(nativeBody, `class="lf-lighting-rail"`) {
		t.Error("native Lighting rendered an empty ownership rail")
	}
	for _, obsolete := range []string{"Workspace state", `lf-lighting-rail-panel`, `>Configuration<`, `class="lf-lighting-owner-banner"`, "RGB Cluster currently controls this device.", "OpenRGB currently controls this device."} {
		if strings.Contains(nativeBody, obsolete) {
			t.Errorf("native Lighting retained obsolete rail content %q", obsolete)
		}
	}
	localClusterInput := strings.Split(nativeBody, `id="lf-lighting-rgb-cluster"`)[1]
	localExternalInput := strings.Split(nativeBody, `id="lf-lighting-openrgb-integration"`)[1]
	if strings.Contains(localClusterInput[:strings.Index(localClusterInput, ">")], "disabled") || strings.Contains(localExternalInput[:strings.Index(localExternalInput, ">")], "disabled") {
		t.Error("normal native ownership did not leave both toggles usable")
	}

	imported := devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb"})
	importedBody := renderDevicesLightingView(t, imported)
	if !strings.Contains(importedBody, "RGB Cluster") {
		t.Error("imported Lighting omitted RGB Cluster")
	}
	if strings.Contains(importedBody, "OpenRGB Integration") || strings.Contains(importedBody, "OpenRGB Link") {
		t.Error("imported Lighting rendered an actionable OpenRGB ownership control")
	}
	if strings.Count(importedBody, `data-lf-ownership-kind=`) != 1 || !strings.Contains(importedBody, `class="lf-lighting-ownership-panel"`) {
		t.Error("imported Lighting did not render exactly one main ownership control")
	}

	cluster := devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "native", ClusterControlled: true})
	clusterBody := renderDevicesLightingView(t, cluster)
	clusterInput := strings.Split(clusterBody, `id="lf-lighting-rgb-cluster"`)[1]
	externalInput := strings.Split(clusterBody, `id="lf-lighting-openrgb-integration"`)[1]
	if strings.Contains(clusterInput[:strings.Index(clusterInput, ">")], "disabled") || !strings.Contains(externalInput[:strings.Index(externalInput, ">")], "disabled") {
		t.Error("native Cluster ownership did not preserve the active toggle and disable OpenRGB Integration")
	}
	if strings.Contains(clusterBody, `class="lf-lighting-rail"`) || !strings.Contains(clusterBody, `class="lf-lighting-layout lf-lighting-layout-no-rail"`) {
		t.Error("cluster-owned Lighting did not retain its full-width layout")
	}
	if !strings.Contains(clusterBody, `class="lf-lighting-owner-banner"`) || !strings.Contains(clusterBody, "RGB Cluster controls this device") {
		t.Error("cluster-owned Lighting omitted its primary-card ownership banner")
	}
	clusterBanner := strings.Index(clusterBody, `class="lf-lighting-owner-banner"`)
	clusterPrimary := strings.Index(clusterBody, `class="lf-lighting-primary"`)
	clusterEffectStage := strings.Index(clusterBody, `class="lf-lighting-effect-stage"`)
	if clusterPrimary < 0 || clusterBanner <= clusterPrimary || clusterEffectStage <= clusterBanner {
		t.Error("cluster-owned Lighting did not place its ownership banner before local controls")
	}
	for _, obsolete := range []string{"RGB Cluster owned", "RGB Cluster owns output", "Local configuration remains stored", "RGB Cluster currently owns this device's lighting output.", "Controlled by RGB Cluster", "RGB Cluster currently controls this device."} {
		if strings.Contains(clusterBody, obsolete) {
			t.Errorf("cluster-owned Lighting retained obsolete ownership copy %q", obsolete)
		}
	}

	external := devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "native", ExternalControlled: true})
	externalBody := renderDevicesLightingView(t, external)
	clusterInput = strings.Split(externalBody, `id="lf-lighting-rgb-cluster"`)[1]
	externalInput = strings.Split(externalBody, `id="lf-lighting-openrgb-integration"`)[1]
	if !strings.Contains(clusterInput[:strings.Index(clusterInput, ">")], "disabled") || strings.Contains(externalInput[:strings.Index(externalInput, ">")], "disabled") {
		t.Error("native OpenRGB ownership did not preserve the active toggle and disable RGB Cluster")
	}
	if strings.Contains(externalBody, `class="lf-lighting-rail"`) || !strings.Contains(externalBody, `class="lf-lighting-layout lf-lighting-layout-no-rail"`) || !strings.Contains(externalBody, `class="lf-lighting-owner-banner"`) || !strings.Contains(externalBody, "OpenRGB controls this device") {
		t.Error("OpenRGB-owned Lighting did not retain its full-width ownership panel")
	}
	externalBanner := strings.Index(externalBody, `class="lf-lighting-owner-banner"`)
	externalPrimary := strings.Index(externalBody, `class="lf-lighting-primary"`)
	externalEffectStage := strings.Index(externalBody, `class="lf-lighting-effect-stage"`)
	if externalPrimary < 0 || externalBanner <= externalPrimary || externalEffectStage <= externalBanner {
		t.Error("OpenRGB-owned Lighting did not place its ownership banner before local controls")
	}
	for _, obsolete := range []string{"External OpenRGB owned", "External OpenRGB owns output", "Local canonical configuration remains stored", "OpenRGB currently controls this device."} {
		if strings.Contains(externalBody, obsolete) {
			t.Errorf("OpenRGB-owned Lighting retained obsolete ownership copy %q", obsolete)
		}
	}
}

func TestDevicesLightingChannelWorkspaceUsesSharedBottomOwnershipControls(t *testing.T) {
	summary := devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{
		TargetKind: "native", ClusterControlled: true,
		Channels: []lightingpresentation.Channel{{
			TargetID: "ccxt-port-0", ChannelID: "0", Name: "8-LED Series Fan", Label: "RGB Intake", LEDCount: 8,
			Lighting: lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "static", EffectSupported: true, ClusterControlled: true, SupportedEffects: []lightingpresentation.EffectOption{{ID: "static", Label: "Static"}}},
		}},
	})
	body := renderDevicesLightingView(t, summary)
	channelsAt := strings.Index(body, `data-lf-lighting-channel-list`)
	ownershipAt := strings.Index(body, `class="lf-lighting-ownership-panel"`)
	if channelsAt < 0 || ownershipAt <= channelsAt {
		t.Fatalf("channel ownership placement = channels:%d ownership:%d", channelsAt, ownershipAt)
	}
	for _, want := range []string{`data-lf-ownership-kind="cluster"`, `data-lf-ownership-kind="openrgb-integration"`, `id="lf-lighting-rgb-cluster" type="checkbox" data-lf-lighting-ownership-input checked`, `data-lf-cluster-controlled="true"`} {
		if !strings.Contains(body, want) {
			t.Errorf("CCXT channel Lighting omitted %q", want)
		}
	}
	clusterInput := strings.Split(body, `id="lf-lighting-rgb-cluster"`)[1]
	externalInput := strings.Split(body, `id="lf-lighting-openrgb-integration"`)[1]
	if strings.Contains(clusterInput[:strings.Index(clusterInput, ">")], "disabled") || !strings.Contains(externalInput[:strings.Index(externalInput, ">")], "disabled") {
		t.Error("CCXT Cluster ownership did not preserve shared mutual exclusion")
	}
	channelSelect := strings.Split(body, `data-lf-effect-selector`)[1]
	if !strings.Contains(channelSelect[:strings.Index(channelSelect, ">")], "disabled") {
		t.Error("CCXT Cluster ownership did not disable its native channel mutation")
	}
}

func TestDevicesLightingOwnershipRoutesReuseLegacyHandlers(t *testing.T) {
	router := setRoutes()
	for _, route := range []struct {
		modern string
		legacy string
	}{
		{modern: "/api/devices/lighting/rgb-cluster", legacy: "/api/color/setCluster"},
		{modern: "/api/devices/lighting/openrgb-integration", legacy: "/api/color/setOpenRgbIntegration"},
	} {
		t.Run(route.modern, func(t *testing.T) {
			const body = `{"deviceId":"missinglightingdevice","mode":1}`
			modernRecorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, route.modern, body)
			legacyRecorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, route.legacy, body)
			if modernRecorder.Code != http.StatusOK || legacyRecorder.Code != http.StatusOK {
				t.Fatalf("modern HTTP = %d, legacy HTTP = %d", modernRecorder.Code, legacyRecorder.Code)
			}
			modern := decodeLifecycleResponse(t, modernRecorder)
			legacy := decodeLifecycleResponse(t, legacyRecorder)
			if modern.Code != legacy.Code || modern.Status != legacy.Status || modern.Message != legacy.Message {
				t.Errorf("modern response = %#v, legacy response = %#v", modern, legacy)
			}
		})
	}
}

func TestDevicesWorkspaceAddsWritableNativeScimitarLightingOnly(t *testing.T) {
	if os.Getenv(devicesPageHelperEnvironment) == "scimitar-native" {
		initializeDevicesPageTestProcess(t)
		runDevicesWorkspaceAddsWritableNativeScimitarLightingOnlyAssertions(t)
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestDevicesWorkspaceAddsWritableNativeScimitarLightingOnly$")
	command.Env = append(os.Environ(), devicesPageHelperEnvironment+"=scimitar-native")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Scimitar native Lighting helper process failed: %v\n%s", err, output)
	}
}

func runDevicesWorkspaceAddsWritableNativeScimitarLightingOnlyAssertions(t *testing.T) {
	scimitarSerial := "scimitar-presentation"
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{
		scimitarSerial: {
			Serial: scimitarSerial, Product: "SCIMITAR PRO RGB", ProductType: common.ProductTypeScimitarProRgb - 1,
			Instance: devicesPageLightingSnapshotProvider{serial: scimitarSerial, snapshot: lightingpresentation.Snapshot{TargetKind: "native",
				ConfiguredEffect: "gradient", EffectSupported: true, HasBrightness: true, Brightness: 64,
				SupportedEffects: []lightingpresentation.EffectOption{{ID: "gradient", Label: "Gradient"}},
				PaletteKind:      string(rgb.LightingPaletteGradient), HasGradient: true,
				GradientStops:      []lightingpresentation.GradientStop{{Position: 0.2, ColorHex: "#102030", Intensity: 0.4}},
				ClusterControlled:  false,
				ExternalControlled: false,
			}},
		},
	}, map[string]stats.BatteryStats{}, scimitarSerial)
	if !ok || summary == nil || summary.Lighting == nil || summary.Lighting.ReadOnly ||
		summary.Lighting.TargetKind != "native" || summary.Lighting.ConfiguredEffect != "gradient" ||
		!summary.Lighting.HasBrightness || summary.Lighting.Brightness != 64 ||
		summary.Lighting.PaletteKind != string(rgb.LightingPaletteGradient) || !summary.Lighting.HasGradient ||
		summary.Lighting.ClusterControlled || summary.Lighting.ExternalControlled || len(summary.Lighting.GradientStops) != 1 || summary.Lighting.GradientStops[0].ColorHex != "#102030" {
		t.Fatalf("Scimitar Devices summary = %#v, ok=%t", summary, ok)
	}
	body := renderDevicesLightingView(t, summary.Lighting)
	if !strings.Contains(body, `data-lf-lighting-read-only="false"`) || !strings.Contains(body, `data-lf-lighting-target="native"`) {
		t.Fatalf("Scimitar Lighting markup is not marked writable native:\n%s", body)
	}
	for _, control := range []string{"lf-lighting-effect-selector", "lf-lighting-brightness-slider", "lf-lighting-gradient-add", "lf-lighting-gradient-save"} {
		start := strings.Index(body, `id="`+control+`"`)
		if start < 0 {
			t.Fatalf("Scimitar Lighting markup omitted %s", control)
		}
		end := strings.Index(body[start:], ">")
		if end < 0 || strings.Contains(body[start:start+end], "disabled") {
			t.Errorf("Scimitar Lighting control %s is disabled", control)
		}
	}

	ordinary, ordinaryOK := devicesWorkspaceSummaryForSerial(map[string]*common.Device{
		"ordinary": {Serial: "ordinary", Product: "Ordinary", ProductType: common.ProductTypeScimitarProRgb - 1, Instance: &struct{}{}},
	}, map[string]stats.BatteryStats{}, "ordinary")
	if !ordinaryOK || ordinary == nil || ordinary.Lighting != nil {
		t.Fatalf("unrelated native device gained Lighting: %#v, ok=%t", ordinary, ordinaryOK)
	}
}

func TestDevicesLightingEffectIconInventory(t *testing.T) {
	root := devicesThemeRepositoryRoot(t)
	descriptors := rgb.SoftwareEffectDescriptors()
	deviceDescriptors := make([]rgb.SoftwareEffectDescriptor, 0, len(descriptors))
	seenIDs := make(map[string]string, len(descriptors))

	for _, descriptor := range descriptors {
		if !descriptor.Scope.Includes(rgb.EffectScopeDevice) {
			continue
		}
		deviceDescriptors = append(deviceDescriptors, descriptor)
		if previous, exists := seenIDs[descriptor.ID]; exists && previous != descriptor.Icon {
			t.Errorf("software effect ID %q maps to conflicting icons %q and %q", descriptor.ID, previous, descriptor.Icon)
		}
		seenIDs[descriptor.ID] = descriptor.Icon

		if descriptor.Icon == "" || !strings.HasSuffix(descriptor.Icon, ".svg") {
			t.Errorf("software effect %q icon = %q, want a non-empty SVG filename", descriptor.ID, descriptor.Icon)
			continue
		}
		if filepath.Base(descriptor.Icon) != descriptor.Icon || strings.ContainsAny(descriptor.Icon, `/\\`) {
			t.Errorf("software effect %q icon is not a safe filename: %q", descriptor.ID, descriptor.Icon)
			continue
		}

		wantURL := "/static/img/icons/rgb/" + descriptor.Icon
		summary := devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
			ConfiguredEffect: descriptor.ID,
			EffectSupported:  true,
			SupportedEffects: []lightingpresentation.EffectOption{{ID: descriptor.ID, Label: descriptor.Label}},
		})
		if summary.ConfiguredEffectIconURL != wantURL {
			t.Errorf("software effect %q icon URL = %q, want %q", descriptor.ID, summary.ConfiguredEffectIconURL, wantURL)
		}
		if _, err := os.Stat(filepath.Join(root, "static", "img", "icons", "rgb", descriptor.Icon)); err != nil {
			t.Errorf("software effect %q icon asset %q: %v", descriptor.ID, descriptor.Icon, err)
		}
	}

	if len(deviceDescriptors) != 35 {
		t.Fatalf("device-scoped software effect descriptors = %d, want 35", len(deviceDescriptors))
	}
	for id, wantIcon := range map[string]string{
		"off":                 "off.svg",
		"aurora":              "aurora.svg",
		"spiralrainbow":       "spiralrainbow.svg",
		"pastelspiralrainbow": "pastelspiralrainbow.svg",
		"visor":               "visor.svg",
		"watercolor":          "watercolor.svg",
	} {
		descriptor, ok := rgb.SoftwareEffectDescriptorByID(id)
		if !ok || descriptor.Icon != wantIcon {
			t.Errorf("software effect %q icon = %q, found = %t, want %q", id, descriptor.Icon, ok, wantIcon)
		}
	}

	for _, id := range []string{"unknown", "../wave", `wave');background:url(/escaped.svg);/*`} {
		if got := devicesLightingEffectIconURL(id); got != "" {
			t.Errorf("non-canonical software effect ID %q produced icon URL %q", id, got)
		}
	}
}

func TestDevicesLightingEffectSelectorPresentation(t *testing.T) {
	source := lightingpresentation.Snapshot{TargetKind: "openrgb",
		ConfiguredEffect: "wave",
		EffectSupported:  true,
		SupportedEffects: []lightingpresentation.EffectOption{
			{ID: "wave", Label: "Wave"},
			{ID: "aurora-z", Label: "aurora"},
			{ID: "off", Label: "Off"},
			{ID: "circle", Label: "Circle"},
			{ID: "aurora-a", Label: "Aurora"},
		},
	}
	summary := devicesLightingWorkspaceSummaryFromSnapshot(source)
	if len(summary.SupportedEffects) != len(source.SupportedEffects) {
		t.Fatalf("presentation effect options = %d, want %d snapshot-supported effects", len(summary.SupportedEffects), len(source.SupportedEffects))
	}
	wantOrder := []string{"aurora-a", "aurora-z", "circle", "off", "wave"}
	for index, want := range wantOrder {
		if summary.SupportedEffects[index].ID != want {
			t.Fatalf("sorted effect %d = %q, want %q: %#v", index, summary.SupportedEffects[index].ID, want, summary.SupportedEffects)
		}
	}
	if !summary.SupportedEffects[4].Selected {
		t.Error("configured stable effect ID is not selected")
	}
	wantSourceOrder := []string{"wave", "aurora-z", "off", "circle", "aurora-a"}
	for index, want := range wantSourceOrder {
		if source.SupportedEffects[index].ID != want {
			t.Fatalf("source effect %d was reordered to %q, want %q", index, source.SupportedEffects[index].ID, want)
		}
	}
	summary.SupportedEffects[0].ID = "changed"
	if source.SupportedEffects[4].ID != "aurora-a" {
		t.Error("presentation effect options alias the source snapshot")
	}

	emptyLabelOff := devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
		SupportedEffects: []lightingpresentation.EffectOption{{ID: "off"}},
	})
	if len(emptyLabelOff.SupportedEffects) != 1 || emptyLabelOff.SupportedEffects[0].ID != "off" || emptyLabelOff.SupportedEffects[0].Label != "Off" {
		t.Fatalf("empty-label supported Off presentation = %#v", emptyLabelOff.SupportedEffects)
	}

	withoutOff := devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
		SupportedEffects: []lightingpresentation.EffectOption{{ID: "static", Label: "Static"}},
	})
	if len(withoutOff.SupportedEffects) != 1 || withoutOff.SupportedEffects[0].ID != "static" {
		t.Fatalf("presentation fabricated an effect absent from the snapshot: %#v", withoutOff.SupportedEffects)
	}
}

func devicesLightingSpeedSnapshot(effect string, speed float64) lightingpresentation.Snapshot {
	capability, _ := rgb.LightingEffectCapabilities(effect)
	return lightingpresentation.Snapshot{TargetKind: "openrgb",
		ConfiguredEffect: effect,
		EffectSupported:  true,
		SupportedEffects: []lightingpresentation.EffectOption{{
			ID:    effect,
			Label: effect,
		}},
		HasSpeed: capability.SupportsSpeed,
		Speed:    speed,
	}
}

func TestDevicesLightingSpeedControlPresentation(t *testing.T) {
	for _, effect := range []string{"circle", "flame", "cyberpunkglitch", "rain", "aurora", "gradient"} {
		t.Run(effect, func(t *testing.T) {
			summary := devicesLightingWorkspaceSummaryFromSnapshot(devicesLightingSpeedSnapshot(effect, 2))
			if !summary.HasSpeedControl || summary.Speed != "2" {
				t.Fatalf("speed presentation for %q = %#v", effect, summary)
			}
		})
	}

	for _, test := range []struct {
		name     string
		snapshot lightingpresentation.Snapshot
	}{
		{name: "unsupported effect", snapshot: func() lightingpresentation.Snapshot {
			value := devicesLightingSpeedSnapshot("circle", 2)
			value.EffectSupported = false
			value.HasSpeed = false
			return value
		}()},
		{name: "missing speed", snapshot: func() lightingpresentation.Snapshot {
			value := devicesLightingSpeedSnapshot("circle", 2)
			value.HasSpeed = false
			return value
		}()},
	} {
		t.Run(test.name, func(t *testing.T) {
			if summary := devicesLightingWorkspaceSummaryFromSnapshot(test.snapshot); summary.HasSpeedControl {
				t.Fatalf("%s produced a Speed control: %#v", test.name, summary)
			}
		})
	}
}

func TestDevicesLightingEffectSelectorTemplate(t *testing.T) {
	if os.Getenv(devicesPageHelperEnvironment) == "effect-selector" {
		initializeDevicesPageTestProcess(t)
		runDevicesLightingEffectSelectorTemplateAssertions(t)
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestDevicesLightingEffectSelectorTemplate$")
	command.Env = append(os.Environ(), devicesPageHelperEnvironment+"=effect-selector")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Devices Lighting effect selector helper process failed: %v\n%s", err, output)
	}
}

func TestDevicesLightingBrightnessTemplate(t *testing.T) {
	if os.Getenv(devicesPageHelperEnvironment) == "brightness-slider" {
		initializeDevicesPageTestProcess(t)
		runDevicesLightingBrightnessTemplateAssertions(t)
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestDevicesLightingBrightnessTemplate$")
	command.Env = append(os.Environ(), devicesPageHelperEnvironment+"=brightness-slider")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Devices Lighting brightness slider helper process failed: %v\n%s", err, output)
	}
}

func TestDevicesLightingSpeedTemplate(t *testing.T) {
	if os.Getenv(devicesPageHelperEnvironment) == "speed-slider" {
		initializeDevicesPageTestProcess(t)
		runDevicesLightingSpeedTemplateAssertions(t)
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestDevicesLightingSpeedTemplate$")
	command.Env = append(os.Environ(), devicesPageHelperEnvironment+"=speed-slider")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Devices Lighting speed slider helper process failed: %v\n%s", err, output)
	}
}

func TestDevicesLightingColorResetTemplate(t *testing.T) {
	if os.Getenv(devicesPageHelperEnvironment) == "color-reset" {
		initializeDevicesPageTestProcess(t)
		runDevicesLightingColorResetTemplateAssertions(t)
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestDevicesLightingColorResetTemplate$")
	command.Env = append(os.Environ(), devicesPageHelperEnvironment+"=color-reset")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Devices Lighting color/reset helper process failed: %v\n%s", err, output)
	}
}

func runDevicesLightingColorResetTemplateAssertions(t *testing.T) {
	staticSnapshot := lightingpresentation.Snapshot{TargetKind: "openrgb",
		ConfiguredEffect: "static",
		EffectSupported:  true,
		PaletteKind:      string(rgb.LightingPaletteStaticSingle),
		SingleColorHex:   "#00ffff",
	}
	uncustomizedBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(staticSnapshot))
	for _, expected := range []string{
		`id="lf-lighting-color-input"`,
		`type="color"`,
		`value="#00ffff"`,
		`data-lf-current-color="#00ffff"`,
		`id="lf-lighting-color-hex"`,
		`data-lf-reset-control`,
		`data-lf-device-serial="lighting-template-device"`,
		`data-lf-effect="static"`,
	} {
		if !strings.Contains(uncustomizedBody, expected) {
			t.Errorf("uncustomized Static template does not contain %q", expected)
		}
	}
	resetStart := strings.Index(uncustomizedBody, `class="lf-reset-control"`)
	if resetStart < 0 {
		t.Fatal("uncustomized Static template does not render the revealable Reset container")
	}
	resetEnd := strings.Index(uncustomizedBody[resetStart:], ">")
	if resetEnd < 0 || !strings.Contains(uncustomizedBody[resetStart:resetStart+resetEnd], "hidden") {
		t.Error("uncustomized Static Reset container is not initially hidden")
	}

	staticSnapshot.Customized = true
	customizedBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(staticSnapshot))
	resetStart = strings.Index(customizedBody, `class="lf-reset-control"`)
	if resetStart < 0 {
		t.Fatal("customized Static template does not render Reset")
	}
	resetEnd = strings.Index(customizedBody[resetStart:], ">")
	if resetEnd < 0 || strings.Contains(customizedBody[resetStart:resetStart+resetEnd], "hidden") {
		t.Error("customized Static template keeps Reset hidden")
	}
	if !strings.Contains(customizedBody, `data-lf-reset-button`) || !strings.Contains(customizedBody, `Reset to default`) {
		t.Error("customized Static template does not render the Reset button")
	}

	staticSnapshot.ClusterControlled = true
	clusterBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(staticSnapshot))
	for _, id := range []string{"lf-lighting-color-input", "lf-lighting-color-hex"} {
		inputStart := strings.Index(clusterBody, `id="`+id+`"`)
		if inputStart < 0 {
			t.Errorf("cluster-owned Static color control %s is absent", id)
			continue
		}
		inputEnd := strings.Index(clusterBody[inputStart:], ">")
		if inputEnd < 0 || !strings.Contains(clusterBody[inputStart:inputStart+inputEnd], "disabled") {
			t.Errorf("cluster-owned Static color control %s is active", id)
		}
	}
	if strings.Contains(clusterBody, `data-lf-reset-control`) || strings.Contains(clusterBody, `data-lf-reset-button`) {
		t.Error("cluster-owned Static template exposes local Reset controls")
	}

	unsupportedBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
		ConfiguredEffect: "aurora",
		EffectSupported:  true,
		PaletteKind:      string(rgb.LightingPaletteGenerated),
		SingleColorHex:   "#00ffff",
	}))
	if strings.Contains(unsupportedBody, `data-lf-color-input`) || strings.Contains(unsupportedBody, `data-lf-color-hex`) {
		t.Error("non-single-color effect rendered the color editor")
	}

	twoColorSnapshot := lightingpresentation.Snapshot{TargetKind: "openrgb",
		ConfiguredEffect: "wave",
		EffectSupported:  true,
		HasSpeed:         true,
		Speed:            5,
		PaletteKind:      string(rgb.LightingPaletteTwoColor),
		TwoColorStartHex: "#418fe8",
		TwoColorEndHex:   "#828282",
	}
	twoColorBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(twoColorSnapshot))
	for _, expected := range []string{
		`data-lf-two-color-control`,
		`data-lf-current-start="#418fe8"`,
		`data-lf-current-end="#828282"`,
		`for="lf-lighting-start-color-input">Start</label>`,
		`id="lf-lighting-start-color-input"`,
		`id="lf-lighting-start-color-hex"`,
		`for="lf-lighting-end-color-input">End</label>`,
		`id="lf-lighting-end-color-input"`,
		`id="lf-lighting-end-color-hex"`,
		`id="lf-lighting-two-color-status" aria-live="polite"`,
		`data-lf-speed-control`,
	} {
		if !strings.Contains(twoColorBody, expected) {
			t.Errorf("uncustomized two-color template does not contain %q", expected)
		}
	}
	for _, id := range []string{
		"lf-lighting-start-color-input",
		"lf-lighting-start-color-hex",
		"lf-lighting-end-color-input",
		"lf-lighting-end-color-hex",
		"lf-lighting-two-color-status",
	} {
		if count := strings.Count(twoColorBody, ` id="`+id+`"`); count != 1 {
			t.Errorf("two-color template ID %q count = %d, want 1", id, count)
		}
	}
	resetStart = strings.Index(twoColorBody, `class="lf-reset-control"`)
	if resetStart < 0 {
		t.Fatal("uncustomized two-color Reset container is absent")
	}
	resetEnd = strings.Index(twoColorBody[resetStart:], ">")
	if resetEnd < 0 || !strings.Contains(twoColorBody[resetStart:resetStart+resetEnd], "hidden") {
		t.Error("uncustomized two-color Reset container is not hidden")
	}

	twoColorSnapshot.Customized = true
	customizedTwoColorBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(twoColorSnapshot))
	resetStart = strings.Index(customizedTwoColorBody, `class="lf-reset-control"`)
	if resetStart < 0 {
		t.Fatal("customized two-color Reset container is absent")
	}
	resetEnd = strings.Index(customizedTwoColorBody[resetStart:], ">")
	if resetEnd < 0 || strings.Contains(customizedTwoColorBody[resetStart:resetStart+resetEnd], "hidden") {
		t.Error("customized two-color Reset is not visible")
	}

	twoColorSnapshot.ClusterControlled = true
	clusterTwoColorBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(twoColorSnapshot))
	for _, id := range []string{
		"lf-lighting-start-color-input",
		"lf-lighting-start-color-hex",
		"lf-lighting-end-color-input",
		"lf-lighting-end-color-hex",
	} {
		inputStart := strings.Index(clusterTwoColorBody, ` id="`+id+`"`)
		if inputStart < 0 {
			t.Errorf("cluster-owned two-color control %s is absent", id)
			continue
		}
		inputEnd := strings.Index(clusterTwoColorBody[inputStart:], ">")
		if inputEnd < 0 || !strings.Contains(clusterTwoColorBody[inputStart:inputStart+inputEnd], "disabled") {
			t.Errorf("cluster-owned two-color control %s is not disabled", id)
		}
	}
	if strings.Contains(clusterTwoColorBody, `data-lf-reset-control`) {
		t.Error("cluster-owned two-color template exposes local Reset")
	}

	for _, test := range []struct {
		effect string
		high   float64
	}{
		{effect: "cpu-temperature", high: 95},
		{effect: "gpu-temperature", high: 80},
	} {
		temperatureSnapshot := lightingpresentation.Snapshot{TargetKind: "openrgb",
			ConfiguredEffect:  test.effect,
			EffectSupported:   true,
			PaletteKind:       string(rgb.LightingPaletteTemperatureThree),
			HasTemperature:    true,
			TemperatureLow:    lightingpresentation.TemperaturePoint{ColorHex: "#00ff00", Celsius: 20},
			TemperatureMiddle: lightingpresentation.TemperaturePoint{ColorHex: "#ffff00", Celsius: 50},
			TemperatureHigh:   lightingpresentation.TemperaturePoint{ColorHex: "#ff0000", Celsius: test.high},
		}
		body := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(temperatureSnapshot))
		for _, expected := range []string{
			`data-lf-temperature-control`, `value="#00ff00"`, `value="#ffff00"`, `value="#ff0000"`,
			`value="` + fmt.Sprint(test.high) + `"`, `step="any"`, `Low temperature threshold in degrees Celsius`,
			`Middle temperature threshold in degrees Celsius`, `High temperature threshold in degrees Celsius`,
		} {
			if !strings.Contains(body, expected) {
				t.Errorf("%s temperature template does not contain %q", test.effect, expected)
			}
		}
		for _, role := range []string{"low", "middle", "high"} {
			for _, suffix := range []string{"color", "hex", "celsius"} {
				id := "lf-lighting-temperature-" + role + "-" + suffix
				if count := strings.Count(body, ` id="`+id+`"`); count != 1 {
					t.Errorf("temperature template ID %q count = %d, want 1", id, count)
				}
			}
		}
		if strings.Contains(body, `data-lf-speed-control`) || strings.Contains(body, "MinTemp") || strings.Contains(body, "MaxTemp") {
			t.Errorf("%s rendered unsupported temperature controls", test.effect)
		}
		uncustomizedResetStart := strings.Index(body, `class="lf-reset-control"`)
		if uncustomizedResetStart < 0 {
			t.Errorf("%s uncustomized temperature Reset container is absent", test.effect)
		} else if uncustomizedResetEnd := strings.Index(body[uncustomizedResetStart:], ">"); uncustomizedResetEnd < 0 ||
			!strings.Contains(body[uncustomizedResetStart:uncustomizedResetStart+uncustomizedResetEnd], "hidden") {
			t.Errorf("%s uncustomized temperature Reset is not hidden", test.effect)
		}
		temperatureSnapshot.Customized = true
		customizedBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(temperatureSnapshot))
		resetStart := strings.Index(customizedBody, `class="lf-reset-control"`)
		if resetStart < 0 {
			t.Errorf("%s customized temperature Reset is not visible", test.effect)
		} else if resetEnd := strings.Index(customizedBody[resetStart:], ">"); resetEnd < 0 ||
			strings.Contains(customizedBody[resetStart:resetStart+resetEnd], "hidden") {
			t.Errorf("%s customized temperature Reset is not visible", test.effect)
		}
	}

	clusterTemperature := lightingpresentation.Snapshot{TargetKind: "openrgb",
		ConfiguredEffect: "cpu-temperature", EffectSupported: true,
		PaletteKind: string(rgb.LightingPaletteTemperatureThree), HasTemperature: true, ClusterControlled: true,
		TemperatureLow:    lightingpresentation.TemperaturePoint{ColorHex: "#00ff00", Celsius: 20},
		TemperatureMiddle: lightingpresentation.TemperaturePoint{ColorHex: "#ffff00", Celsius: 50},
		TemperatureHigh:   lightingpresentation.TemperaturePoint{ColorHex: "#ff0000", Celsius: 95},
	}
	clusterTemperatureBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(clusterTemperature))
	if strings.Count(clusterTemperatureBody, " disabled") < 9 || strings.Contains(clusterTemperatureBody, `data-lf-reset-control`) {
		t.Error("cluster-owned temperature editor is active or exposes Reset")
	}

	gradientSnapshot := lightingpresentation.Snapshot{TargetKind: "openrgb",
		ConfiguredEffect: "gradient", EffectSupported: true, HasBrightness: true, Brightness: 60,
		HasSpeed: true, Speed: 10, PaletteKind: string(rgb.LightingPaletteGradient), HasGradient: true,
		GradientStops: []lightingpresentation.GradientStop{
			{Position: 0, ColorHex: "#ff0000", Intensity: 1},
			{Position: 0.25, ColorHex: "#00ff00", Intensity: 1},
			{Position: 0.5, ColorHex: "#0000ff", Intensity: 1},
			{Position: 0.75, ColorHex: "#ffff00", Intensity: 1},
		},
	}
	gradientBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(gradientSnapshot))
	for _, expected := range []string{
		`data-lf-gradient-control`, `id="lf-lighting-gradient-stops"`, `Gradient stops`, `Add stop`, `Save Gradient`,
		`Position uses 0 for the start and 1 for the end`, `Intensity is relative to device Brightness`,
		`data-lf-speed-control`, `data-lf-brightness-slider`,
		`value="#ff0000"`, `value="#00ff00"`, `value="#0000ff"`, `value="#ffff00"`,
		`value="0.25"`, `value="0.5"`, `value="0.75"`, `data-lf-gradient-remove`,
	} {
		if !strings.Contains(gradientBody, expected) {
			t.Errorf("Gradient template does not contain %q", expected)
		}
	}
	for _, obsolete := range []string{`lf-lighting-primary-metrics`, `<small>Palette</small>`, `<dt>Palette capability</dt>`} {
		if strings.Contains(gradientBody, obsolete) {
			t.Errorf("Gradient template still exposes the temporary Palette readout %q", obsolete)
		}
	}
	for number := 1; number <= 4; number++ {
		for _, field := range []string{"color", "hex", "position", "intensity"} {
			id := fmt.Sprintf("lf-lighting-gradient-%s-%d", field, number)
			if count := strings.Count(gradientBody, ` id="`+id+`"`); count != 1 {
				t.Errorf("Gradient input ID %q count = %d, want 1", id, count)
			}
		}
		if !strings.Contains(gradientBody, fmt.Sprintf(`aria-label="Remove stop %d"`, number)) {
			t.Errorf("Gradient stop %d Remove label is absent", number)
		}
	}
	gradientResetStart := strings.Index(gradientBody, `class="lf-reset-control"`)
	if gradientResetStart < 0 {
		t.Error("uncustomized Gradient Reset is not hidden")
	} else if gradientResetEnd := strings.Index(gradientBody[gradientResetStart:], ">"); gradientResetEnd < 0 ||
		!strings.Contains(gradientBody[gradientResetStart:gradientResetStart+gradientResetEnd], "hidden") {
		t.Error("uncustomized Gradient Reset is not hidden")
	}
	gradientSnapshot.Customized = true
	customGradientBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(gradientSnapshot))
	gradientResetStart = strings.Index(customGradientBody, `class="lf-reset-control"`)
	if gradientResetStart < 0 {
		t.Error("customized Gradient Reset is not visible")
	} else if gradientResetEnd := strings.Index(customGradientBody[gradientResetStart:], ">"); gradientResetEnd < 0 ||
		strings.Contains(customGradientBody[gradientResetStart:gradientResetStart+gradientResetEnd], "hidden") {
		t.Error("customized Gradient Reset is not visible")
	}

	twoStopGradient := gradientSnapshot
	twoStopGradient.GradientStops = gradientSnapshot.GradientStops[:2]
	twoStopBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(twoStopGradient))
	for _, remove := range strings.Split(twoStopBody, `data-lf-gradient-remove`)[1:] {
		if end := strings.Index(remove, ">"); end < 0 || !strings.Contains(remove[:end], "disabled") {
			t.Error("two-stop Gradient Remove is enabled")
		}
	}

	gradientSnapshot.ClusterControlled = true
	clusterGradientBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(gradientSnapshot))
	if strings.Count(clusterGradientBody, " disabled") < 19 || strings.Contains(clusterGradientBody, `data-lf-reset-control`) {
		t.Error("cluster-owned Gradient controls are active or expose Reset")
	}
	for _, palette := range []rgb.LightingPaletteKind{
		rgb.LightingPaletteStaticSingle, rgb.LightingPaletteTwoColor, rgb.LightingPaletteTemperatureThree,
		rgb.LightingPaletteGenerated, rgb.LightingPaletteNone,
	} {
		body := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
			ConfiguredEffect: "other", EffectSupported: true, PaletteKind: string(palette), HasGradient: true,
			GradientStops: gradientSnapshot.GradientStops,
		}))
		if strings.Contains(body, `data-lf-gradient-control`) {
			t.Errorf("palette %q rendered Gradient controls", palette)
		}
	}

	for _, palette := range []rgb.LightingPaletteKind{
		rgb.LightingPaletteStaticSingle,
		rgb.LightingPaletteGenerated,
		rgb.LightingPaletteTemperatureThree,
		rgb.LightingPaletteGradient,
		rgb.LightingPaletteNone,
		"unsupported",
	} {
		body := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
			ConfiguredEffect: "other",
			EffectSupported:  true,
			PaletteKind:      string(palette),
			TwoColorStartHex: "#112233",
			TwoColorEndHex:   "#445566",
		}))
		if strings.Contains(body, `data-lf-two-color-control`) {
			t.Errorf("palette %q rendered the two-color editor", palette)
		}
	}
}

func runDevicesLightingSpeedTemplateAssertions(t *testing.T) {
	for _, effect := range []string{"circle", "flame", "cyberpunkglitch", "rain", "aurora", "gradient"} {
		snapshot := devicesLightingSpeedSnapshot(effect, 2)
		body := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(snapshot))
		for _, expected := range []string{
			`data-lf-speed-control`,
			`for="lf-lighting-speed-slider">Speed</label>`,
			`id="lf-lighting-speed-number"`,
			`type="number"`,
			`min="1"`,
			`max="10"`,
			`step="0.1"`,
			`aria-label="Speed level"`,
			`id="lf-lighting-speed-slider"`,
			`type="range"`,
			`data-lf-current-stored-speed="2"`,
			`data-lf-effect="` + effect + `"`,
			`data-lf-speed-control-mode="software"`,
			`data-lf-number-id="lf-lighting-speed-number"`,
			`data-lf-status-id="lf-lighting-speed-status"`,
			`<span>Slow</span><span>Fast</span>`,
			`id="lf-lighting-speed-status" aria-live="polite"`,
		} {
			if !strings.Contains(body, expected) {
				t.Errorf("%s Speed template does not contain %q", effect, expected)
			}
		}
		for _, forbidden := range []string{
			"Effective speed",
			"Persistent speed",
			"Stored animation speed",
			"Release to save",
			"Speed saved.",
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s Speed template contains duplicate or permanent copy %q", effect, forbidden)
			}
		}
	}

	for _, effect := range []string{"static", "off", "cpu-temperature", "gpu-temperature"} {
		body := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(devicesLightingSpeedSnapshot(effect, 2)))
		if strings.Contains(body, "data-lf-speed-slider") || strings.Contains(body, "data-lf-speed-number") || strings.Contains(body, "Speed / Unavailable") {
			t.Errorf("%s rendered an unavailable or interactive Speed control", effect)
		}
	}

	clusterSnapshot := devicesLightingSpeedSnapshot("rain", 2)
	clusterSnapshot.ClusterControlled = true
	clusterBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(clusterSnapshot))
	for _, id := range []string{"lf-lighting-speed-slider", "lf-lighting-speed-number"} {
		inputStart := strings.Index(clusterBody, `id="`+id+`"`)
		if inputStart < 0 {
			t.Errorf("cluster-owned Speed control %s is absent", id)
			continue
		}
		inputEnd := strings.Index(clusterBody[inputStart:], ">")
		if inputEnd < 0 || !strings.Contains(clusterBody[inputStart:inputStart+inputEnd], "disabled") {
			t.Errorf("cluster-owned Speed control %s is not disabled", id)
		}
	}
	if !strings.Contains(clusterBody, `aria-describedby="lf-lighting-speed-status"`) || strings.Contains(clusterBody, "lf-lighting-speed-cluster-explanation") {
		t.Error("cluster-owned Speed retains a stale ownership description reference")
	}
}

func runDevicesLightingBrightnessTemplateAssertions(t *testing.T) {
	for _, brightness := range []uint8{0, 100} {
		body := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
			HasBrightness: true,
			Brightness:    brightness,
		}))
		value := fmt.Sprintf("%d", brightness)
		for _, expected := range []string{
			`<label class="lf-range-control-label" for="lf-lighting-brightness-slider">Brightness</label>`,
			`id="lf-lighting-brightness-number"`,
			`type="number"`,
			`data-lf-brightness-number`,
			`aria-label="Brightness percentage"`,
			`<span class="lf-range-control-suffix" aria-hidden="true">%</span>`,
			`id="lf-lighting-brightness-slider"`,
			`type="range"`,
			`min="0"`,
			`max="100"`,
			`step="1"`,
			`value="` + value + `"`,
			`style="--lf-range-progress: ` + value + `%;"`,
			`data-lf-current-brightness="` + value + `"`,
			`aria-valuetext="` + value + ` percent"`,
			`id="lf-lighting-brightness-status" aria-live="polite"`,
			`data-lf-brightness-readout data-lf-device-serial="lighting-template-device">` + value + `%</strong>`,
		} {
			if !strings.Contains(body, expected) {
				t.Errorf("brightness %s template does not contain %q", value, expected)
			}
		}
		if strings.Contains(body, `<small>Brightness</small>`) {
			t.Errorf("brightness %s template retained the duplicate primary metric", value)
		}
		for _, forbidden := range []string{
			"Stored local output level for this device.",
			"Changes are saved when the value is committed.",
			"Brightness saved.",
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("brightness %s template contains permanent status or helper copy %q", value, forbidden)
			}
		}
	}

	escapedBody := renderDevicesLightingViewForSerial(t, `lighting"&<serial`, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
		HasBrightness: true,
		Brightness:    50,
	}))
	if !strings.Contains(escapedBody, `data-lf-device-serial="lighting&#34;&amp;&lt;serial"`) ||
		strings.Contains(escapedBody, `data-lf-device-serial="lighting"&<serial"`) {
		t.Error("brightness control did not contextually escape the device serial data attribute")
	}

	unavailableBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb"}))
	if !strings.Contains(unavailableBody, "lf-range-control-unavailable") ||
		!strings.Contains(unavailableBody, `>Unavailable</strong>`) ||
		strings.Contains(unavailableBody, "data-lf-brightness-slider") ||
		strings.Contains(unavailableBody, "data-lf-brightness-number") ||
		strings.Contains(unavailableBody, `type="range"`) || strings.Contains(unavailableBody, `type="number"`) {
		t.Error("missing brightness snapshot did not render the non-interactive unavailable state")
	}
	for _, forbidden := range []string{"Stored local output level for this device.", "Brightness unavailable for this stored lighting profile."} {
		if strings.Contains(unavailableBody, forbidden) {
			t.Errorf("unavailable brightness state retained permanent helper copy %q", forbidden)
		}
	}

	clusterBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
		HasBrightness:     true,
		Brightness:        40,
		ClusterControlled: true,
	}))
	for _, id := range []string{"lf-lighting-brightness-slider", "lf-lighting-brightness-number"} {
		inputStart := strings.Index(clusterBody, `id="`+id+`"`)
		if inputStart < 0 {
			t.Fatalf("cluster-owned Lighting view does not render %s", id)
		}
		inputEnd := strings.Index(clusterBody[inputStart:], ">")
		if inputEnd < 0 || !strings.Contains(clusterBody[inputStart:inputStart+inputEnd], "disabled") {
			t.Errorf("cluster-owned brightness control %s is not disabled", id)
		}
	}
	if !strings.Contains(clusterBody, `aria-describedby="lf-lighting-brightness-status"`) || strings.Contains(clusterBody, "lf-lighting-brightness-cluster-explanation") {
		t.Error("cluster-owned Brightness retains a stale ownership description reference")
	}
}

func runDevicesLightingEffectSelectorTemplateAssertions(t *testing.T) {

	normalBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
		ConfiguredEffect: "wave",
		EffectSupported:  true,
		SupportedEffects: []lightingpresentation.EffectOption{
			{ID: "wave", Label: "Wave <Bright> & Wide"},
			{ID: "off", Label: "Off"},
		},
	}))
	for _, expected := range []string{
		`<span class="lf-lighting-effect-icon-frame" aria-hidden="true">`,
		`class="lf-lighting-effect-icon-art" style="--lf-lighting-effect-mask: url('/static/img/icons/rgb/wave.svg');"`,
		`<strong class="lf-lighting-effect-name">Wave &lt;Bright&gt; &amp; Wide</strong>`,
		`data-lf-effect-selector`,
		`data-lf-device-serial="lighting-template-device"`,
		`data-lf-current-effect="wave"`,
		`<option value="off">Off</option>`,
		`value="wave" selected>Wave &lt;Bright&gt; &amp; Wide</option>`,
		`id="lf-lighting-effect-status" aria-live="polite"`,
	} {
		if !strings.Contains(normalBody, expected) {
			t.Errorf("normal effect selector does not contain %q", expected)
		}
	}
	for _, unwanted := range []string{"Stored configuration", `>Lighting</h2>`, "Configured effect", "Stable ID <code"} {
		if strings.Contains(normalBody, unwanted) {
			t.Errorf("Lighting markup retained %q", unwanted)
		}
	}
	if strings.Count(normalBody, "<option ") != 2 {
		t.Errorf("normal effect selector rendered %d options, want exactly the two snapshot options", strings.Count(normalBody, "<option "))
	}
	selectorStart := strings.Index(normalBody, `<select`)
	selectorEnd := strings.Index(normalBody, `</select>`)
	if selectorStart < 0 || selectorEnd < selectorStart {
		t.Fatal("normal effect selector markup is incomplete")
	}
	for _, forbidden := range []string{"<img", "<span", "style=", "mask"} {
		if strings.Contains(normalBody[selectorStart:selectorEnd], forbidden) {
			t.Errorf("native effect selector contains non-text option presentation %q", forbidden)
		}
	}
	if strings.Index(normalBody, `<option value="off">Off</option>`) > strings.Index(normalBody, `value="wave" selected>`) {
		t.Error("Off is not alphabetized before Wave in the rendered effect selector")
	}
	if strings.Contains(normalBody, "Controlled by RGB Cluster") || strings.Contains(normalBody, `id="lf-lighting-effect-cluster-explanation"`) {
		t.Error("non-clustered effect selector renders the cluster ownership explanation")
	}
	if strings.Contains(normalBody, "ZgotmplZ") || strings.Contains(normalBody, `<img class="lf-lighting-effect`) {
		t.Error("known effect icon did not render as a safely escaped CSS mask")
	}
	poisonedLabelBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
		ConfiguredEffect: "wave",
		EffectSupported:  true,
		SupportedEffects: []lightingpresentation.EffectOption{{ID: "wave", Label: `Wave "');background:url(/label.svg);\\`}},
	}))
	iconTagStart := strings.Index(poisonedLabelBody, `<span class="lf-lighting-effect-icon-art"`)
	if iconTagStart < 0 {
		t.Fatal("effect with CSS-like label does not render its canonical icon")
	}
	iconTagEnd := strings.Index(poisonedLabelBody[iconTagStart:], ">")
	if iconTagEnd < 0 {
		t.Fatal("effect with CSS-like label renders an incomplete icon tag")
	}
	wantIconTag := `<span class="lf-lighting-effect-icon-art" style="--lf-lighting-effect-mask: url('/static/img/icons/rgb/wave.svg');">`
	if iconTag := poisonedLabelBody[iconTagStart : iconTagStart+iconTagEnd+1]; iconTag != wantIconTag {
		t.Errorf("CSS-like effect label influenced icon tag: got %q, want %q", iconTag, wantIconTag)
	}

	withoutOffBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
		SupportedEffects: []lightingpresentation.EffectOption{{ID: "static", Label: "Static"}},
	}))
	if strings.Contains(withoutOffBody, `value="off"`) || strings.Contains(withoutOffBody, `>Off</option>`) {
		t.Error("effect selector fabricated Off when the snapshot did not report it")
	}

	emptyBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
		SupportedEffects: []lightingpresentation.EffectOption{{ID: "off", Label: "Off"}},
	}))
	for _, expected := range []string{
		`<option value="" selected disabled>Not configured</option>`,
		`<option value="off">Off</option>`,
	} {
		if !strings.Contains(emptyBody, expected) {
			t.Errorf("empty configured effect selector does not contain %q", expected)
		}
	}
	if strings.Contains(emptyBody, `Stable ID <code`) || strings.Contains(emptyBody, "Configured effect") {
		t.Error("empty configured effect retained hidden presentation labels")
	}

	maliciousEffectID := `legacy');background:url(/escaped.svg);\\<effect>`
	unsupportedBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
		ConfiguredEffect: maliciousEffectID,
		EffectSupported:  false,
		SupportedEffects: []lightingpresentation.EffectOption{{ID: "static", Label: "Static & Safe"}},
	}))
	for _, expected := range []string{
		`<option value="static">Static &amp; Safe</option>`,
		`class="lf-lighting-effect-icon-fallback"`,
	} {
		if !strings.Contains(unsupportedBody, expected) {
			t.Errorf("unsupported configured effect selector does not contain %q", expected)
		}
	}
	if strings.Contains(unsupportedBody, maliciousEffectID) || strings.Contains(unsupportedBody, "Static & Safe") {
		t.Error("effect selector rendered an unescaped stable ID or display label")
	}
	if strings.Contains(unsupportedBody, `--lf-lighting-effect-mask:`) {
		t.Error("unsupported configured effect influenced a CSS mask URL")
	}

	knownUnsupportedBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
		ConfiguredEffect: "wave",
		EffectSupported:  false,
		SupportedEffects: []lightingpresentation.EffectOption{{ID: "static", Label: "Static"}},
	}))
	if !strings.Contains(knownUnsupportedBody, `class="lf-lighting-effect-icon-fallback"`) ||
		strings.Contains(knownUnsupportedBody, `/static/img/icons/rgb/wave.svg`) {
		t.Error("known but unsupported effect did not retain the generic fallback")
	}

	clusterBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb",
		ConfiguredEffect:  "static",
		EffectSupported:   true,
		ClusterControlled: true,
		SupportedEffects:  []lightingpresentation.EffectOption{{ID: "static", Label: "Static"}},
	}))
	for _, expected := range []string{
		`data-lf-cluster-controlled="true"`,
		`aria-describedby="lf-lighting-effect-status"`,
		`/static/img/icons/rgb/static.svg`,
		"RGB Cluster controls this device",
	} {
		if !strings.Contains(clusterBody, expected) {
			t.Errorf("cluster-controlled effect selector does not contain %q", expected)
		}
	}
	if strings.Contains(clusterBody, "lf-lighting-effect-cluster-explanation") || strings.Contains(clusterBody, `class="lf-lighting-rail"`) {
		t.Error("cluster-controlled Lighting view retains obsolete ownership helper markup")
	}
	selectorStart = strings.Index(clusterBody, `id="lf-lighting-effect-selector"`)
	if selectorStart < 0 {
		t.Fatal("cluster-controlled Lighting view does not render the effect selector")
	}
	selectorEnd = strings.Index(clusterBody[selectorStart:], ">")
	if selectorEnd < 0 || !strings.Contains(clusterBody[selectorStart:selectorStart+selectorEnd], "disabled") {
		t.Error("cluster-controlled effect selector is not disabled")
	}

	unavailableBody := renderDevicesLightingView(t, devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{TargetKind: "openrgb"}))
	if !strings.Contains(unavailableBody, "Effect selection unavailable") ||
		!strings.Contains(unavailableBody, "No supported effects were reported for this controller.") ||
		strings.Contains(unavailableBody, "data-lf-effect-selector") || strings.Contains(unavailableBody, "<select") {
		t.Error("missing supported effects did not produce the safe unavailable selector state")
	}
}

func renderDevicesLightingView(t *testing.T, lighting *openRGBLightingWorkspaceSummary) string {
	t.Helper()
	return renderDevicesLightingViewForSerial(t, "lighting-template-device", lighting)
}

func renderDevicesDPIView(t *testing.T, dpi *devicesDPIWorkspaceSummary) string {
	t.Helper()
	var rendered bytes.Buffer
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "devices.html", templates.Web{
		Devices: map[string]*common.Device{
			"dpi-template-device": {Product: "DPI Template Device", Serial: "dpi-template-device"},
		},
		Device:       &devicesWorkspaceSummary{Product: "DPI Template Device", Serial: "dpi-template-device", DPI: dpi, View: "dpi"},
		BatteryStats: map[string]stats.BatteryStats{},
		Page:         "devices",
	}); err != nil {
		t.Fatal(err)
	}
	return rendered.String()
}

func renderDevicesPerformanceView(t *testing.T, dpi *devicesDPIWorkspaceSummary, performance *devicesPerformanceWorkspaceSummary) string {
	t.Helper()
	var rendered bytes.Buffer
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "devices.html", templates.Web{
		Devices:      map[string]*common.Device{"performance-template-device": {Product: "Performance Template Device", Serial: "performance-template-device"}},
		Device:       &devicesWorkspaceSummary{Product: "Performance Template Device", Serial: "performance-template-device", DPI: dpi, Performance: performance, View: "dpi"},
		BatteryStats: map[string]stats.BatteryStats{}, Page: "devices",
	}); err != nil {
		t.Fatal(err)
	}
	return rendered.String()
}

func renderDevicesLightingViewForSerial(t *testing.T, serial string, lighting *openRGBLightingWorkspaceSummary) string {
	t.Helper()
	var rendered bytes.Buffer
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "devices.html", templates.Web{
		Devices: map[string]*common.Device{
			serial: {Product: "Lighting Template Device", Serial: serial},
		},
		Device: &devicesWorkspaceSummary{
			Product:  "Lighting Template Device",
			Serial:   serial,
			OpenRGB:  &openRGBWorkspaceSummary{},
			Lighting: lighting,
			View:     "lighting",
		},
		BatteryStats: map[string]stats.BatteryStats{},
		Page:         "devices",
	}); err != nil {
		t.Fatal(err)
	}
	return rendered.String()
}

func initializeDevicesPageTestProcess(t *testing.T) {
	t.Helper()

	packageRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	applicationRoot := filepath.Clean(filepath.Join(packageRoot, "..", ".."))
	temporaryRoot := t.TempDir()

	t.Setenv("LUMENFORGE_SERVICE_MODE", string(config.ServiceModeUser))
	t.Setenv("LUMENFORGE_APPLICATION_ROOT", applicationRoot)
	t.Setenv("LUMENFORGE_CONFIG_ROOT", filepath.Join(temporaryRoot, "config"))
	t.Setenv("LUMENFORGE_DATA_ROOT", filepath.Join(temporaryRoot, "data"))
	config.Init()
	templates.Init()
}

func runDevicesPageRouteAssertions(t *testing.T) {
	initializeDevicesPageTestProcess(t)
	router := setRoutes()

	emptyRecorder := requestDevicesPage(t, router, "")
	if emptyRecorder.Code != http.StatusOK {
		t.Fatalf("empty GET /devices status = %d: %s", emptyRecorder.Code, emptyRecorder.Body.String())
	}
	emptyBody := emptyRecorder.Body.String()
	for _, expected := range []string{
		"No devices available",
		"class=\"lf-workspace-empty\"",
		"href=\"/static/css/app-shell.css\"",
		"src=\"/static/js/devices-lighting.js\"",
	} {
		if !strings.Contains(emptyBody, expected) {
			t.Errorf("empty GET /devices response does not contain %q", expected)
		}
	}
	for _, excluded := range []string{"Select a device", "class=\"lf-device-hero\""} {
		if strings.Contains(emptyBody, excluded) {
			t.Errorf("empty GET /devices response unexpectedly contains %q", excluded)
		}
	}

	visibleSerial := "lf-devices-page-visible"
	visibleProduct := "Visible <Test> & Device"
	visibleDisplayIdentifier := "ORGB <Display> & ID"
	visibleVendor := "Vendor <North> & Co"
	visibleLocation := "Desk <Left> & Rear"
	visibleDescription := "Imported <Controller> & Lighting"
	visibleZoneOne := "Zone <One> & Main"
	visibleZoneTwo := "Zone Two"
	visibleInstance := &openrgbimport.Device{
		Product:            visibleProduct,
		Serial:             visibleSerial,
		IsOpenRGB:          true,
		DisplaySerial:      visibleDisplayIdentifier,
		DisplaySerialLabel: "SERIAL",
		Description:        visibleDescription,
		Config: &openrgbimport.DeviceConfig{
			Serial:   visibleSerial,
			Product:  visibleProduct,
			Vendor:   visibleVendor,
			Location: visibleLocation,
			Zones: []openrgbimport.ZoneConfig{
				{Name: visibleZoneOne, LedCount: 1},
				{Name: visibleZoneTwo, LedCount: 12},
			},
		},
		DeviceProfile: &openrgbimport.DeviceProfile{
			Active:     true,
			RGBCluster: true,
		},
		RGBModes: []string{"static", "wave", "cpu-temperature", "gradient", "rainbow", "off"},
	}
	visible := &common.Device{
		Product:     visibleProduct,
		Serial:      visibleSerial,
		Firmware:    "test-firmware",
		Image:       "icon-mouse.svg",
		Instance:    visibleInstance,
		Unavailable: true,
	}
	if err := devices.RegisterOpenRGBImport(visible, visibleInstance); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		devices.RemoveOpenRGBImport(visibleSerial, visibleInstance)
	})

	otherSerial := "lf-devices-page-other"
	otherInstance := &openrgbimport.Device{Serial: otherSerial, Product: "Other Test Device"}
	other := &common.Device{
		Product:  "Other Test Device",
		Serial:   otherSerial,
		Instance: otherInstance,
	}
	if err := devices.RegisterOpenRGBImport(other, otherInstance); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		devices.RemoveOpenRGBImport(otherSerial, otherInstance)
	})

	hiddenSerial := "lf-devices-page-hidden"
	hiddenInstance := &openrgbimport.Device{Serial: hiddenSerial, Product: "Hidden Test Device"}
	hidden := &common.Device{
		Product:  "Hidden Test Device",
		Serial:   hiddenSerial,
		Hidden:   true,
		Instance: hiddenInstance,
	}
	if err := devices.RegisterOpenRGBImport(hidden, hiddenInstance); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		devices.RemoveOpenRGBImport(hiddenSerial, hiddenInstance)
	})

	zeroZoneSerial := "lf-devices-page-openrgb-zero-zones"
	zeroZoneInstance := &openrgbimport.Device{
		Product:   "OpenRGB Zero Zones",
		Serial:    zeroZoneSerial,
		IsOpenRGB: true,
		Config: &openrgbimport.DeviceConfig{
			Serial: zeroZoneSerial,
			Zones:  []openrgbimport.ZoneConfig{},
		},
	}
	zeroZone := &common.Device{
		Product:  zeroZoneInstance.Product,
		Serial:   zeroZoneSerial,
		Instance: zeroZoneInstance,
	}
	if err := devices.RegisterOpenRGBImport(zeroZone, zeroZoneInstance); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		devices.RemoveOpenRGBImport(zeroZoneSerial, zeroZoneInstance)
	})

	unselectedRecorder := requestDevicesPage(t, router, "")
	if unselectedRecorder.Code != http.StatusOK {
		t.Fatalf("unselected GET /devices status = %d: %s", unselectedRecorder.Code, unselectedRecorder.Body.String())
	}
	unselectedBody := unselectedRecorder.Body.String()
	for _, expected := range []string{
		"Devices workspace",
		"href=\"/devices?device=" + visibleSerial + "\"",
		"Visible &lt;Test&gt; &amp; Device",
		"class=\"lf-workspace-empty\"",
		"href=\"/static/css/app-shell.css\"",
	} {
		if !strings.Contains(unselectedBody, expected) {
			t.Errorf("unselected GET /devices response does not contain %q", expected)
		}
	}
	for _, excluded := range []string{
		hiddenSerial,
		"Hidden Test Device",
		"Battery 0%",
		"class=\"lf-device-hero\"",
		"Visible <Test> & Device",
	} {
		if strings.Contains(unselectedBody, excluded) {
			t.Errorf("unselected GET /devices response unexpectedly contains %q", excluded)
		}
	}

	unrelatedRecorder := requestDevicesPage(t, router, "foo=bar")
	if unrelatedRecorder.Code != http.StatusOK {
		t.Fatalf("unrelated-query GET /devices status = %d: %s", unrelatedRecorder.Code, unrelatedRecorder.Body.String())
	}
	unrelatedBody := unrelatedRecorder.Body.String()
	for _, expected := range []string{
		"href=\"/devices?device=" + visibleSerial + "\"",
		"class=\"lf-workspace-empty\"",
	} {
		if !strings.Contains(unrelatedBody, expected) {
			t.Errorf("unrelated-query GET /devices response does not contain %q", expected)
		}
	}
	if strings.Contains(unrelatedBody, "class=\"lf-device-hero\"") {
		t.Error("unrelated-only query unexpectedly selects a device")
	}

	selectedRecorder := requestDevicesPage(t, router, "device="+url.QueryEscape(visibleSerial))
	if selectedRecorder.Code != http.StatusOK {
		t.Fatalf("selected GET /devices status = %d: %s", selectedRecorder.Code, selectedRecorder.Body.String())
	}
	selectedBody := selectedRecorder.Body.String()
	for _, expected := range []string{
		"class=\"lf-overview-workspace\"",
		"Visible &lt;Test&gt; &amp; Device",
		visibleSerial,
		"src=\"/static/img/icons/icon-mouse.svg\"",
		"test-firmware",
		"href=\"/device/" + visibleSerial + "\"",
		"Open full controls",
		"<dt class=\"lf-device-summary-label\">Internal ID</dt>",
		"<dd class=\"lf-device-summary-value\">" + visibleSerial + "</dd>",
		"<h2 class=\"lf-openrgb-title\">OpenRGB controller</h2>",
		"<h3 class=\"lf-openrgb-subtitle\">Controller metadata</h3>",
		"<dt class=\"lf-device-summary-label\">Serial</dt>",
		"<dd class=\"lf-device-summary-value\">ORGB &lt;Display&gt; &amp; ID</dd>",
		"Vendor &lt;North&gt; &amp; Co",
		"Desk &lt;Left&gt; &amp; Rear",
		"Imported &lt;Controller&gt; &amp; Lighting",
		"<dt class=\"lf-device-summary-label\">Effect</dt>",
		"<dd class=\"lf-device-summary-value\">static</dd>",
		"<dt class=\"lf-device-summary-label\">Brightness</dt>",
		"<dd class=\"lf-device-summary-value\">0%</dd>",
		"Zone &lt;One&gt; &amp; Main",
		"<span class=\"lf-openrgb-zone-led-count\">1 LED</span>",
		visibleZoneTwo,
		"<span class=\"lf-openrgb-zone-led-count\">12 LEDs</span>",
		"class=\"lf-device-item lf-device-item-active\" href=\"/devices?device=" + visibleSerial + "\" aria-current=\"page\"",
		"class=\"lf-device-item\" href=\"/devices?device=" + otherSerial + "\"",
		"<nav class=\"lf-device-workspace-nav\" aria-label=\"Device workspace\">",
		"class=\"lf-device-workspace-link lf-device-workspace-link-active\" href=\"/devices?device=" + visibleSerial + "\" aria-current=\"page\">Overview</a>",
		"href=\"/devices?device=" + visibleSerial + "&amp;view=lighting\">Lighting</a>",
		"Unavailable",
	} {
		if !strings.Contains(selectedBody, expected) {
			t.Errorf("selected GET /devices response does not contain %q", expected)
		}
	}
	for _, excluded := range []string{
		"Visible <Test> & Device",
		"class=\"lf-device-item lf-device-item-active\" href=\"/devices?device=" + otherSerial + "\"",
		"<dt class=\"lf-device-summary-label\">Battery</dt>",
		"Battery 0%",
		visibleDisplayIdentifier,
		visibleVendor,
		visibleLocation,
		visibleDescription,
		visibleZoneOne,
		"class=\"lf-lighting-workspace\"",
		"Read-only preview",
	} {
		if strings.Contains(selectedBody, excluded) {
			t.Errorf("selected GET /devices response unexpectedly contains %q", excluded)
		}
	}
	lowerSelectedBody := strings.ToLower(selectedBody)
	for _, claim := range []string{"connected", "online", "healthy"} {
		if strings.Contains(lowerSelectedBody, claim) {
			t.Errorf("unavailable selected device response contains %q claim", claim)
		}
	}
	if strings.Count(selectedBody, "<h1 ") != 1 {
		t.Errorf("selected GET /devices contains %d h1 elements, want 1", strings.Count(selectedBody, "<h1 "))
	}
	for _, label := range []string{"Internal ID", "Serial"} {
		markup := "<dt class=\"lf-device-summary-label\">" + label + "</dt>"
		if count := strings.Count(selectedBody, markup); count != 1 {
			t.Errorf("selected OpenRGB response contains %q %d times, want 1", markup, count)
		}
	}
	for _, zoneName := range []string{"Zone &lt;One&gt; &amp; Main", visibleZoneTwo} {
		if count := strings.Count(selectedBody, zoneName); count != 1 {
			t.Errorf("selected GET /devices contains zone name %q %d times, want 1", zoneName, count)
		}
	}

	lightingRecorder := requestDevicesPage(t, router, "device="+url.QueryEscape(visibleSerial)+"&view=lighting")
	if lightingRecorder.Code != http.StatusOK {
		t.Fatalf("Lighting GET /devices status = %d: %s", lightingRecorder.Code, lightingRecorder.Body.String())
	}
	lightingBody := lightingRecorder.Body.String()
	for _, expected := range []string{
		"class=\"lf-lighting-workspace\"",
		"Stored lighting configuration and renderer capabilities",
		"OpenRGB imported controller",
		"Stored lighting state",
		`class="lf-lighting-ownership-panel"`,
		"RGB Cluster",
		`data-lf-ownership-kind="cluster"`,
		"<nav class=\"lf-device-workspace-nav\" aria-label=\"Device workspace\">",
		"class=\"lf-device-workspace-link\" href=\"/devices?device=" + visibleSerial + "\">Overview</a>",
		"class=\"lf-device-workspace-link lf-device-workspace-link-active\" href=\"/devices?device=" + visibleSerial + "&amp;view=lighting\" aria-current=\"page\">Lighting</a>",
		">Static</strong>",
		"class=\"lf-lighting-effect-name\">Static</strong>",
		"data-lf-device-serial=\"" + visibleSerial + "\"",
		"data-lf-current-effect=\"static\"",
		"/static/img/icons/rgb/static.svg",
		"data-lf-cluster-controlled=\"true\"",
		"id=\"lf-lighting-effect-selector\"",
		"<option value=\"off\">Off</option>",
		"value=\"static\" selected>Static</option>",
		"aria-describedby=\"lf-lighting-effect-status\"",
		"id=\"lf-lighting-effect-status\" aria-live=\"polite\"",
		`data-lf-brightness-readout data-lf-device-serial="` + visibleSerial + `">0%</strong>`,
		`id="lf-lighting-brightness-slider"`,
		`type="range"`,
		`value="0"`,
		`style="--lf-range-progress: 0%;"`,
		`data-lf-current-brightness="0"`,
		`id="lf-lighting-brightness-status" aria-live="polite"`,
		"RGB Cluster controls this device",
	} {
		if !strings.Contains(lightingBody, expected) {
			t.Errorf("Lighting GET /devices response does not contain %q", expected)
		}
	}
	for _, excluded := range []string{
		"class=\"lf-overview-workspace\"",
		"<form",
		"method=\"post\"",
		"contenteditable",
		"fetch(",
		"onclick=",
		"onchange=",
		"oninput=",
		"onsubmit=",
		"/api/",
		"Supported effects reference",
		"lf-lighting-effect-list",
		"Workspace state",
		">Configuration<",
		"Effect support</dt><dd>Supported</dd>",
		"Static &lt;Effect&gt; &amp; More",
		"</div>div>",
	} {
		if strings.Contains(strings.ToLower(lightingBody), strings.ToLower(excluded)) {
			t.Errorf("Lighting GET /devices response unexpectedly contains %q", excluded)
		}
	}
	if strings.Contains(lightingBody, `--lf-lighting-effect-mask: url('`+visibleSerial) {
		t.Error("device serial influenced the configured effect mask URL")
	}
	queryPayload := `url('/query.svg');\\`
	lightingWithUnrelatedQuery := requestDevicesPage(t, router, "device="+url.QueryEscape(visibleSerial)+"&view=lighting&foo="+url.QueryEscape(queryPayload))
	if lightingWithUnrelatedQuery.Code != http.StatusOK ||
		!strings.Contains(lightingWithUnrelatedQuery.Body.String(), `/static/img/icons/rgb/static.svg`) ||
		strings.Contains(lightingWithUnrelatedQuery.Body.String(), queryPayload) {
		t.Error("unrelated query value influenced the configured effect mask presentation")
	}
	if strings.Count(lightingBody, "<h1 ") != 1 {
		t.Errorf("Lighting GET /devices contains %d h1 elements, want 1", strings.Count(lightingBody, "<h1 "))
	}
	unknownViewRecorder := requestDevicesPage(t, router, "device="+url.QueryEscape(visibleSerial)+"&view=unknown")
	if unknownViewRecorder.Code != http.StatusOK {
		t.Fatalf("unknown-view GET /devices status = %d: %s", unknownViewRecorder.Code, unknownViewRecorder.Body.String())
	}
	unknownViewBody := unknownViewRecorder.Body.String()
	if !strings.Contains(unknownViewBody, "class=\"lf-overview-workspace\"") ||
		strings.Contains(unknownViewBody, "class=\"lf-lighting-workspace\"") ||
		!strings.Contains(unknownViewBody, "Device overview and configured hardware details") {
		t.Error("unknown view did not fall back to the Overview workspace")
	}

	selectedWithUnrelatedRecorder := requestDevicesPage(t, router, "device="+url.QueryEscape(visibleSerial)+"&foo=bar")
	if selectedWithUnrelatedRecorder.Code != http.StatusOK {
		t.Fatalf("selected GET /devices with unrelated query status = %d: %s", selectedWithUnrelatedRecorder.Code, selectedWithUnrelatedRecorder.Body.String())
	}
	selectedWithUnrelatedBody := selectedWithUnrelatedRecorder.Body.String()
	for _, expected := range []string{
		"class=\"lf-overview-workspace\"",
		"class=\"lf-device-item lf-device-item-active\" href=\"/devices?device=" + visibleSerial + "\" aria-current=\"page\"",
		"href=\"/device/" + visibleSerial + "\"",
	} {
		if !strings.Contains(selectedWithUnrelatedBody, expected) {
			t.Errorf("selected GET /devices with unrelated query does not contain %q", expected)
		}
	}

	fallbackRecorder := requestDevicesPage(t, router, "device="+url.QueryEscape(otherSerial))
	if fallbackRecorder.Code != http.StatusOK {
		t.Fatalf("fallback-image GET /devices status = %d: %s", fallbackRecorder.Code, fallbackRecorder.Body.String())
	}
	if !strings.Contains(fallbackRecorder.Body.String(), "class=\"lf-device-hero-image lf-device-hero-image-fallback\" src=\"/static/img/icons/icon-device.svg\"") {
		t.Error("selected device without an image does not render the generic fallback")
	}
	if strings.Contains(fallbackRecorder.Body.String(), "class=\"lf-openrgb-overview\"") {
		t.Error("generic selected device unexpectedly renders the OpenRGB overview")
	}
	if strings.Contains(fallbackRecorder.Body.String(), ">Lighting</a>") {
		t.Error("generic selected device unexpectedly renders Lighting navigation")
	}

	nativeLightingRecorder := requestDevicesPage(t, router, "device="+url.QueryEscape(otherSerial)+"&view=lighting")
	if nativeLightingRecorder.Code != http.StatusOK {
		t.Fatalf("native Lighting GET /devices status = %d: %s", nativeLightingRecorder.Code, nativeLightingRecorder.Body.String())
	}
	nativeLightingBody := nativeLightingRecorder.Body.String()
	if !strings.Contains(nativeLightingBody, "class=\"lf-overview-workspace\"") ||
		strings.Contains(nativeLightingBody, "class=\"lf-lighting-workspace\"") ||
		strings.Contains(nativeLightingBody, ">Lighting</a>") {
		t.Error("native device fabricated a Lighting capability instead of retaining Overview")
	}

	zeroZoneRecorder := requestDevicesPage(t, router, "device="+url.QueryEscape(zeroZoneSerial))
	if zeroZoneRecorder.Code != http.StatusOK {
		t.Fatalf("zero-zone OpenRGB GET /devices status = %d: %s", zeroZoneRecorder.Code, zeroZoneRecorder.Body.String())
	}
	zeroZoneBody := zeroZoneRecorder.Body.String()
	for _, expected := range []string{
		"<h2 class=\"lf-openrgb-title\">OpenRGB controller</h2>",
		"No configured zones",
		"href=\"/device/" + zeroZoneSerial + "\"",
	} {
		if !strings.Contains(zeroZoneBody, expected) {
			t.Errorf("zero-zone OpenRGB response does not contain %q", expected)
		}
	}
	if strings.Contains(zeroZoneBody, "class=\"lf-openrgb-zone-item\"") {
		t.Error("zero-zone OpenRGB response renders a fabricated zone item")
	}
	for _, excluded := range []string{"Controller metadata", "<dt class=\"lf-device-summary-label\">SERIAL</dt>"} {
		if strings.Contains(zeroZoneBody, excluded) {
			t.Errorf("zero-zone OpenRGB response unexpectedly contains %q", excluded)
		}
	}
	if !strings.Contains(zeroZoneBody, "class=\"lf-openrgb-grid lf-openrgb-grid-single\"") {
		t.Error("zero-zone OpenRGB response does not let the lighting summary use the available width")
	}

	stats.UpdateBatteryStats(visibleSerial, visibleProduct, 37, 0)
	batteryRecorder := requestDevicesPage(t, router, "device="+url.QueryEscape(visibleSerial))
	if batteryRecorder.Code != http.StatusOK {
		t.Fatalf("battery GET /devices status = %d: %s", batteryRecorder.Code, batteryRecorder.Body.String())
	}
	batteryBody := batteryRecorder.Body.String()
	if !strings.Contains(batteryBody, "<dt class=\"lf-device-summary-label\">Battery</dt>") ||
		!strings.Contains(batteryBody, "<dd class=\"lf-device-summary-value\">37%</dd>") {
		t.Error("matching battery record is not rendered in the selected summary")
	}
	stats.UpdateBatteryStats(visibleSerial, visibleProduct, 0, 0)
	zeroBatteryRecorder := requestDevicesPage(t, router, "device="+url.QueryEscape(visibleSerial))
	if zeroBatteryRecorder.Code != http.StatusOK {
		t.Fatalf("zero-battery GET /devices status = %d: %s", zeroBatteryRecorder.Code, zeroBatteryRecorder.Body.String())
	}
	if !strings.Contains(zeroBatteryRecorder.Body.String(), "<dd class=\"lf-device-summary-value\">0%</dd>") {
		t.Error("real zero-level battery record is not rendered in the selected summary")
	}

	badRequests := []struct {
		name          string
		rawQuery      string
		rejectedTexts []string
	}{
		{name: "malformed encoding", rawQuery: "device=%zz", rejectedTexts: []string{"device=%zz", "%zz"}},
		{name: "malformed unrelated encoding", rawQuery: "foo=%zz", rejectedTexts: []string{"foo=%zz", "%zz"}},
		{name: "duplicate values", rawQuery: "device=" + visibleSerial + "&device=" + otherSerial, rejectedTexts: []string{visibleSerial, otherSerial}},
		{name: "empty value", rawQuery: "device=", rejectedTexts: []string{"device="}},
		{name: "disallowed characters", rawQuery: "device=invalid_name", rejectedTexts: []string{"invalid_name"}},
		{name: "encoded slash", rawQuery: "device=invalid%2Fserial", rejectedTexts: []string{"invalid%2Fserial", "invalid/serial"}},
	}
	for _, test := range badRequests {
		t.Run(test.name, func(t *testing.T) {
			recorder := requestDevicesPage(t, router, test.rawQuery)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d: %s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			for _, rejectedText := range test.rejectedTexts {
				if strings.Contains(recorder.Body.String(), rejectedText) {
					t.Errorf("bad-request response reflects rejected text %q", rejectedText)
				}
			}
		})
	}

	unknownSerial := "lf-devices-page-unknown"
	unknownRecorder := requestDevicesPage(t, router, "device="+unknownSerial)
	if unknownRecorder.Code != http.StatusNotFound {
		t.Errorf("unknown device status = %d, want %d", unknownRecorder.Code, http.StatusNotFound)
	}
	if strings.Contains(unknownRecorder.Body.String(), unknownSerial) {
		t.Error("unknown-device response reflects the rejected serial")
	}
	unknownLightingRecorder := requestDevicesPage(t, router, "device="+unknownSerial+"&view=lighting")
	if unknownLightingRecorder.Code != http.StatusNotFound || strings.Contains(unknownLightingRecorder.Body.String(), unknownSerial) {
		t.Error("unknown Lighting selection did not preserve safe not-found behavior")
	}

	hiddenRecorder := requestDevicesPage(t, router, "device="+hiddenSerial)
	if hiddenRecorder.Code != http.StatusNotFound {
		t.Errorf("hidden device status = %d, want %d", hiddenRecorder.Code, http.StatusNotFound)
	}
	for _, excluded := range []string{hiddenSerial, "Hidden Test Device"} {
		if strings.Contains(hiddenRecorder.Body.String(), excluded) {
			t.Errorf("hidden-device response unexpectedly contains %q", excluded)
		}
	}
	hiddenLightingRecorder := requestDevicesPage(t, router, "device="+hiddenSerial+"&view=lighting")
	if hiddenLightingRecorder.Code != http.StatusNotFound {
		t.Errorf("hidden Lighting device status = %d, want %d", hiddenLightingRecorder.Code, http.StatusNotFound)
	}

	if summary, ok := devicesWorkspaceSummaryForSerial(
		map[string]*common.Device{"nil-device": nil},
		map[string]stats.BatteryStats{},
		"nil-device",
	); ok || summary != nil {
		t.Error("nil device wrapper produced a selected summary")
	}
	if summary, ok := devicesWorkspaceSummaryForSerial(
		map[string]*common.Device{"requested-device": {Serial: "different-device"}},
		map[string]stats.BatteryStats{},
		"requested-device",
	); ok || summary != nil {
		t.Error("mismatched device wrapper serial produced a selected summary")
	}

	nonOpenRGBSummary, ok := devicesWorkspaceSummaryForSerial(
		map[string]*common.Device{
			"ordinary-device": {Serial: "ordinary-device", Product: "Ordinary Device", Instance: &struct{}{}},
		},
		map[string]stats.BatteryStats{},
		"ordinary-device",
	)
	if !ok || nonOpenRGBSummary == nil {
		t.Fatal("ordinary device did not produce a generic selected summary")
	}
	if nonOpenRGBSummary.OpenRGB != nil {
		t.Error("ordinary device produced an OpenRGB selected summary")
	}
	var nonOpenRGBRendered bytes.Buffer
	if err := templates.GetTemplate().ExecuteTemplate(&nonOpenRGBRendered, "devices.html", templates.Web{
		Devices: map[string]*common.Device{
			"ordinary-device": {Serial: "ordinary-device", Product: "Ordinary Device"},
		},
		Device:       nonOpenRGBSummary,
		BatteryStats: map[string]stats.BatteryStats{},
		Page:         "devices",
	}); err != nil {
		t.Fatal(err)
	}
	nonOpenRGBBody := nonOpenRGBRendered.String()
	if !strings.Contains(nonOpenRGBBody, "class=\"lf-overview-workspace\"") ||
		!strings.Contains(nonOpenRGBBody, "<dt class=\"lf-device-summary-label\">Serial</dt>") ||
		!strings.Contains(nonOpenRGBBody, "<dd class=\"lf-device-summary-value\">ordinary-device</dd>") ||
		strings.Contains(nonOpenRGBBody, "<dt class=\"lf-device-summary-label\">Internal ID</dt>") ||
		strings.Contains(nonOpenRGBBody, "class=\"lf-openrgb-overview\"") {
		t.Error("ordinary device template did not preserve the generic selected summary")
	}

	mismatchedOpenRGBSummary, ok := devicesWorkspaceSummaryForSerial(
		map[string]*common.Device{
			"requested-openrgb": {
				Serial:   "requested-openrgb",
				Product:  "Mismatched OpenRGB",
				Instance: &openrgbimport.Device{Serial: "different-openrgb", IsOpenRGB: true},
			},
		},
		map[string]stats.BatteryStats{},
		"requested-openrgb",
	)
	if !ok || mismatchedOpenRGBSummary == nil {
		t.Fatal("mismatched OpenRGB instance did not preserve the generic summary")
	}
	if mismatchedOpenRGBSummary.OpenRGB != nil {
		t.Error("mismatched OpenRGB instance produced an OpenRGB selected summary")
	}
	if mismatchedOpenRGBSummary.Lighting != nil {
		t.Error("mismatched OpenRGB instance produced a Lighting workspace summary")
	}

	for _, test := range []struct {
		internalLabel string
		displayLabel  string
	}{
		{internalLabel: "SERIAL", displayLabel: "Serial"},
		{internalLabel: "VERSION", displayLabel: "Firmware"},
		{internalLabel: "FALLBACK", displayLabel: "OpenRGB ID"},
	} {
		summary := openRGBWorkspaceSummaryFromSnapshot(openrgbimport.DeviceSnapshot{
			DisplaySerial:      "display-identifier",
			DisplaySerialLabel: test.internalLabel,
		})
		if summary.DisplayIdentifierLabel != test.displayLabel {
			t.Errorf("OpenRGB identifier label %q = %q, want %q", test.internalLabel, summary.DisplayIdentifierLabel, test.displayLabel)
		}
		if !summary.HasMetadata {
			t.Errorf("OpenRGB identifier label %q did not mark metadata as present", test.internalLabel)
		}
	}

	emptyOpenRGB := openRGBWorkspaceSummaryFromSnapshot(openrgbimport.DeviceSnapshot{Brightness: 100})
	if emptyOpenRGB == nil || emptyOpenRGB.Brightness != 100 || emptyOpenRGB.HasMetadata || len(emptyOpenRGB.Zones) != 0 {
		t.Fatalf("empty OpenRGB conversion = %#v", emptyOpenRGB)
	}
	var emptyOpenRGBRendered bytes.Buffer
	if err := templates.GetTemplate().ExecuteTemplate(&emptyOpenRGBRendered, "devices.html", templates.Web{
		Devices: map[string]*common.Device{
			"empty-openrgb": {Product: "Empty OpenRGB", Serial: "empty-openrgb"},
		},
		Device: &devicesWorkspaceSummary{
			Product: "Empty OpenRGB",
			Serial:  "empty-openrgb",
			OpenRGB: emptyOpenRGB,
		},
		BatteryStats: map[string]stats.BatteryStats{},
		Page:         "devices",
	}); err != nil {
		t.Fatal(err)
	}
	emptyOpenRGBBody := emptyOpenRGBRendered.String()
	for _, expected := range []string{
		"OpenRGB controller",
		"Lighting summary",
		"100%",
		"No configured zones",
		"class=\"lf-openrgb-grid lf-openrgb-grid-single\"",
	} {
		if !strings.Contains(emptyOpenRGBBody, expected) {
			t.Errorf("empty OpenRGB template response does not contain %q", expected)
		}
	}
	for _, excluded := range []string{
		"Controller metadata",
		"<dt class=\"lf-device-summary-label\">Vendor</dt>",
		"<dt class=\"lf-device-summary-label\">Location</dt>",
		"<dt class=\"lf-device-summary-label\">Description</dt>",
		"<dt class=\"lf-device-summary-label\">Effect</dt>",
		"class=\"lf-openrgb-zone-item\"",
	} {
		if strings.Contains(emptyOpenRGBBody, excluded) {
			t.Errorf("empty OpenRGB template response unexpectedly contains %q", excluded)
		}
	}

	escapedSerial := "lf;device:escaped"
	escapedProduct := "Escaped <Product> & Name"
	escapedOpenRGBSnapshot := openrgbimport.DeviceSnapshot{
		DisplaySerial:      "Identifier <Value> & More",
		DisplaySerialLabel: "SERIAL",
		Description:        "Description <Value> & More",
		Effect:             "Effect <Value> & More",
		Brightness:         42,
		Config: &openrgbimport.DeviceConfig{
			Vendor:   "Vendor <Value> & More",
			Location: "Location <Value> & More",
			Zones: []openrgbimport.ZoneConfig{
				{Name: "Zone <Value> & More", LedCount: 4},
			},
		},
	}
	escapedOpenRGB := openRGBWorkspaceSummaryFromSnapshot(escapedOpenRGBSnapshot)
	escapedOpenRGBSnapshot.Config.Zones[0].Name = "mutated source zone"
	if len(escapedOpenRGB.Zones) != 1 || escapedOpenRGB.Zones[0].Name != "Zone <Value> & More" {
		t.Fatal("OpenRGB workspace summary retained the snapshot zone slice")
	}
	escapedSummary := &devicesWorkspaceSummary{
		Product:  escapedProduct,
		Serial:   escapedSerial,
		OpenRGB:  escapedOpenRGB,
		Lighting: &openRGBLightingWorkspaceSummary{},
		View:     "overview",
	}
	var rendered bytes.Buffer
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "devices.html", templates.Web{
		Devices: map[string]*common.Device{
			escapedSerial: {Product: escapedProduct, Serial: escapedSerial},
			"nil-entry":   nil,
		},
		Device:       escapedSummary,
		BatteryStats: map[string]stats.BatteryStats{},
		Page:         "devices",
	}); err != nil {
		t.Fatal(err)
	}
	escapedBody := rendered.String()
	if !strings.Contains(escapedBody, "Escaped &lt;Product&gt; &amp; Name") {
		t.Error("escaped template response does not contain escaped product HTML")
	}
	lowerEscapedBody := strings.ToLower(escapedBody)
	for _, expected := range []string{
		"href=\"/devices?device=" + url.QueryEscape(escapedSerial) + "\"",
		"href=\"/devices?device=" + url.QueryEscape(escapedSerial) + "&amp;view=lighting\"",
		"href=\"/device/" + escapedSerial + "\"",
	} {
		if !strings.Contains(lowerEscapedBody, strings.ToLower(expected)) {
			t.Errorf("escaped template response does not contain %q", expected)
		}
	}
	if strings.Contains(escapedBody, escapedProduct) {
		t.Error("escaped template response contains raw product HTML")
	}
	for _, expected := range []string{
		"Identifier &lt;Value&gt; &amp; More",
		"Vendor &lt;Value&gt; &amp; More",
		"Location &lt;Value&gt; &amp; More",
		"Description &lt;Value&gt; &amp; More",
		"Effect &lt;Value&gt; &amp; More",
		"Zone &lt;Value&gt; &amp; More",
	} {
		if !strings.Contains(escapedBody, expected) {
			t.Errorf("escaped OpenRGB response does not contain %q", expected)
		}
	}
	for _, raw := range []string{
		"Identifier <Value> & More",
		"Vendor <Value> & More",
		"Location <Value> & More",
		"Description <Value> & More",
		"Effect <Value> & More",
		"Zone <Value> & More",
	} {
		if strings.Contains(escapedBody, raw) {
			t.Errorf("escaped OpenRGB response contains raw metadata %q", raw)
		}
	}

}
