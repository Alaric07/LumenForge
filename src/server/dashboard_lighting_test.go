package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"LumenForge/src/cluster"
	"LumenForge/src/common"
	"LumenForge/src/coolingpresentation"
	"LumenForge/src/dashboard"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/memorypresentation"
	"LumenForge/src/systeminfo"
	"LumenForge/src/templates"
	"LumenForge/src/version"
)

type dashboardLightingCountProvider struct {
	id        string
	snapshot  lightingpresentation.Snapshot
	available bool
}

type dashboardDeviceProfileFixture struct {
	Label string
}

type dashboardPresentationProvider struct {
	dashboardLightingCountProvider
	DeviceProfile *dashboardDeviceProfileFixture
}

type dashboardMemoryProvider struct {
	id       string
	snapshot memorypresentation.Snapshot
}

type dashboardCoolingProvider struct {
	id       string
	snapshot coolingpresentation.Snapshot
}

func (provider dashboardCoolingProvider) CoolingDeviceID() string { return provider.id }
func (provider dashboardCoolingProvider) CoolingSnapshot() (coolingpresentation.Snapshot, bool) {
	return provider.snapshot, true
}

type dashboardAggregateProvider struct {
	dashboardLightingCountProvider
	dashboardCoolingProvider
}

type dashboardMemoryPresentationProvider struct {
	dashboardLightingCountProvider
	dashboardMemoryProvider
}

func (provider dashboardMemoryProvider) MemoryDeviceID() string { return provider.id }
func (provider dashboardMemoryProvider) MemorySnapshot() (memorypresentation.Snapshot, bool) {
	return provider.snapshot, true
}

func (provider dashboardLightingCountProvider) LightingDeviceID() string { return provider.id }
func (provider dashboardLightingCountProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return provider.snapshot, provider.available
}

func TestDashboardLightingUsesCanonicalClusterSnapshot(t *testing.T) {
	previous := getRGBClusterLightingStatus
	getRGBClusterLightingStatus = func() (cluster.LightingSnapshot, int) {
		return cluster.LightingSnapshot{
			SelectedEffect:      "gradient",
			Brightness:          37,
			EffectiveBrightness: 0,
			Available:           true,
		}, 3
	}
	t.Cleanup(func() { getRGBClusterLightingStatus = previous })

	recorder := httptest.NewRecorder()
	getDashboardLighting(recorder, nil)
	if recorder.Code != 200 {
		t.Fatalf("dashboard lighting status = %d", recorder.Code)
	}
	var response struct {
		Effect     string `json:"effect"`
		Brightness int    `json:"brightness"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Effect != "gradient" || response.Brightness != 37 {
		t.Fatalf("dashboard lighting response = %#v", response)
	}
}

func TestDashboardLightingDeviceCountsClassifyPhysicalDevices(t *testing.T) {
	makeDevices := func(total, clustered int) map[string]*common.Device {
		connected := make(map[string]*common.Device, total)
		for index := 0; index < total; index++ {
			serial := fmt.Sprintf("device-%d", index)
			connected[serial] = &common.Device{Serial: serial, Instance: dashboardLightingCountProvider{
				id: serial, available: true, snapshot: lightingpresentation.Snapshot{TargetKind: "native", ClusterControlled: index < clustered},
			}}
		}
		return connected
	}

	for _, test := range []struct {
		name        string
		connected   map[string]*common.Device
		clustered   int
		independent int
	}{
		{name: "ten independent", connected: makeDevices(10, 0), independent: 10},
		{name: "ten clustered", connected: makeDevices(10, 10), clustered: 10},
		{name: "mixed ownership", connected: makeDevices(10, 6), clustered: 6, independent: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := dashboardLightingDeviceCounts(test.connected); got.Clustered != test.clustered || got.Independent != test.independent {
				t.Fatalf("dashboard lighting counts = %#v, want clustered=%d independent=%d", got, test.clustered, test.independent)
			}
		})
	}
}

func TestDashboardLightingDeviceCountsExcludeNonPhysicalLightingEntries(t *testing.T) {
	connected := map[string]*common.Device{
		"aggregate": {
			Serial: "aggregate", Instance: dashboardLightingCountProvider{id: "aggregate", available: true,
				snapshot: lightingpresentation.Snapshot{TargetKind: "native", Channels: []lightingpresentation.Channel{{}, {}}}},
		},
		"memory": {
			Serial: "memory", Instance: dashboardLightingCountProvider{id: "memory", available: true,
				snapshot: lightingpresentation.Snapshot{TargetKind: "native", Channels: []lightingpresentation.Channel{{}, {}, {}, {}}}},
		},
		"openrgb-external": {
			Serial: "openrgb-external", Instance: dashboardLightingCountProvider{id: "openrgb", available: true,
				snapshot: lightingpresentation.Snapshot{TargetKind: "openrgb", ExternalControlled: true}},
		},
		"non-lighting": {Serial: "non-lighting"},
		"stale": {
			Serial: "stale", Unavailable: true, Instance: dashboardLightingCountProvider{id: "stale", available: true,
				snapshot: lightingpresentation.Snapshot{TargetKind: "native"}},
		},
		"cluster": {
			Serial: "cluster", ProductType: common.ProductTypeCluster, Instance: dashboardLightingCountProvider{id: "cluster", available: true,
				snapshot: lightingpresentation.Snapshot{TargetKind: "native"}},
		},
	}

	if got := dashboardLightingDeviceCounts(connected); got.Clustered != 0 || got.Independent != 3 {
		t.Fatalf("dashboard lighting counts = %#v, want clustered=0 independent=3", got)
	}
}

func TestDashboardCurrentDevicesUseCurrentSharedPresentation(t *testing.T) {
	connected := map[string]*common.Device{
		"native-label": {Serial: "native-label", Product: "Commander Core", Instance: dashboardPresentationProvider{
			dashboardLightingCountProvider: dashboardLightingCountProvider{id: "native-label", available: true, snapshot: lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "rainbow", EffectSupported: true, SupportedEffects: []lightingpresentation.EffectOption{{ID: "rainbow", Label: "Rainbow"}}}},
			DeviceProfile:                  &dashboardDeviceProfileFixture{Label: "Desk cooling"},
		}},
		"native-cluster":  {Serial: "native-cluster", Product: "K95", Instance: dashboardLightingCountProvider{id: "native-cluster", available: true, snapshot: lightingpresentation.Snapshot{TargetKind: "native", ClusterControlled: true}}},
		"native-external": {Serial: "native-external", Product: "External controller", Instance: dashboardLightingCountProvider{id: "native-external", available: true, snapshot: lightingpresentation.Snapshot{TargetKind: "native", ExternalControlled: true}}},
		"openrgb":         {Serial: "openrgb", Product: "Nanoleaf", Instance: dashboardLightingCountProvider{id: "openrgb", available: true, snapshot: lightingpresentation.Snapshot{TargetKind: "openrgb", ClusterControlled: true, HasBrightness: true, Brightness: 67}}},
		"stale":           {Serial: "stale", Product: "Disconnected", Unavailable: true, Instance: dashboardLightingCountProvider{id: "stale", available: true, snapshot: lightingpresentation.Snapshot{TargetKind: "native"}}},
	}

	response := dashboardCurrentDevices(connected, nil)
	if len(response.Native) != 3 || len(response.OpenRGB) != 1 {
		t.Fatalf("dashboard current devices = %#v, want three native and one OpenRGB", response)
	}
	if got := response.Native[0]; got.Serial != "native-cluster" || got.Name != "K95" || got.Lighting != "Cluster" {
		t.Fatalf("unlabeled/clustered device = %#v", got)
	}
	if got := response.Native[1]; got.Serial != "native-external" || got.Lighting != "OpenRGB controls this device" {
		t.Fatalf("externally controlled device = %#v", got)
	}
	if got := response.Native[2]; got.Serial != "native-label" || got.Name != "Desk cooling" || got.Product != "Commander Core" || got.Lighting != "Rainbow" {
		t.Fatalf("labeled/local device = %#v", got)
	}
	if got := response.OpenRGB[0]; got.Serial != "openrgb" || got.Name != "Nanoleaf" || got.Lighting != "Cluster" || got.Brightness == nil || *got.Brightness != 67 {
		t.Fatalf("OpenRGB device = %#v", got)
	}
}

func TestDashboardCurrentDevicesKeepTopLevelAggregatesAndMemorySingle(t *testing.T) {
	connected := map[string]*common.Device{
		"aggregate": {Serial: "aggregate", Product: "Commander XT", Instance: dashboardLightingCountProvider{id: "aggregate", available: true, snapshot: lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "static", EffectSupported: true, SupportedEffects: []lightingpresentation.EffectOption{{ID: "static", Label: "Static"}}}}},
		"memory": {Serial: "memory", Product: "Memory kit", Instance: dashboardMemoryPresentationProvider{
			dashboardLightingCountProvider: dashboardLightingCountProvider{id: "memory", available: true, snapshot: lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "static", EffectSupported: true, HasBrightness: true, Brightness: 80, SupportedEffects: []lightingpresentation.EffectOption{{ID: "static", Label: "Static"}}}},
			dashboardMemoryProvider:        dashboardMemoryProvider{id: "memory", snapshot: memorypresentation.Snapshot{Available: true, Modules: []memorypresentation.Module{{ChannelID: 0, Name: "DIMM 1", Label: "Left DIMM", Temperature: "32.0 °C", TemperatureCelsius: 32}, {ChannelID: 1, Name: "Trident Z5 RGB", Temperature: "34.0 °C", TemperatureCelsius: 34}}}},
		}},
	}

	response := dashboardCurrentDevices(connected, nil)
	if len(response.Native) != 2 {
		t.Fatalf("native device count = %d, want two top-level cards", len(response.Native))
	}
	if response.Native[0].Serial != "aggregate" || response.Native[1].Serial != "memory" {
		t.Fatalf("top-level device order = %#v", response.Native)
	}
	if len(response.Native[1].StatusRows) != 0 || response.Native[1].Brightness == nil || *response.Native[1].Brightness != 80 {
		t.Fatalf("memory presentation = %#v", response.Native[1])
	}
	if len(response.Memory) != 2 || response.Memory[0].Identifier != "DIMM 1" || response.Memory[0].Name != "Left DIMM" || response.Memory[1].Name != "Trident Z5 RGB" {
		t.Fatalf("memory gauge presentation = %#v", response.Memory)
	}
}

func TestDashboardMemoryTemperaturesUseDashboardUnits(t *testing.T) {
	summary := &devicesMemoryWorkspaceSummary{Modules: []devicesMemoryModuleSummary{{ChannelID: 0, Name: "DIMM 1", Temperature: "32.0 °C", TemperatureCelsius: 32}}}
	items := dashboardMemoryTemperatures("memory", summary, dashboard.Dashboard{Celsius: false})
	if len(items) != 1 || items[0].Temperature != "89.6 °F" || items[0].Celsius != 32 {
		t.Fatalf("memory temperatures = %#v", items)
	}
}

func TestDashboardDeviceStatusUsesDashboardUnitsForTypedCoolingTelemetry(t *testing.T) {
	coolant, probe := float32(31.5), float32(29)
	summary := &devicesWorkspaceSummary{OverviewCooling: &devicesOverviewCoolingStatusSummary{Pumps: []devicesOverviewCoolingPumpSummary{{Label: "Coolant", RPM: "2400 RPM", Temperature: "31.5 °C"}}}, TemperatureProbes: []devicesOverviewStatusRow{{Label: "Radiator", Value: "29.0 °C"}}}
	snapshot := coolingpresentation.Snapshot{Channels: []coolingpresentation.Channel{{Temperature: "31.5 °C", Celsius: &coolant}}, TemperatureProbes: []coolingpresentation.TemperatureProbe{{Temperature: "29.0 °C", Celsius: &probe}}}
	rows := dashboardDeviceStatusForDashboard(summary, &snapshot, dashboard.Dashboard{Celsius: false})
	if len(rows) != 2 || rows[0].Value != "2400 RPM · 88.7 °F" || rows[1].Value != "84.2 °F" {
		t.Fatalf("Dashboard cooling rows = %#v", rows)
	}
}

func TestDashboardCurrentDevicesKeepAggregateCoolingRowsInsideOneCard(t *testing.T) {
	core := coolingpresentation.Snapshot{Available: true, Channels: []coolingpresentation.Channel{
		{ID: 0, Name: "Pump", Label: "Coolant", RPM: 2400, Temperature: "31.5 °C", ContainsPump: true},
		{ID: 1, Name: "Fan 1", Label: "Front Intake", RPM: 581},
		{ID: 2, Name: "Fan 2", RPM: 0},
	}}
	coreXT := coolingpresentation.Snapshot{Available: true, Channels: []coolingpresentation.Channel{
		{ID: 0, Name: "Fan 1", Label: "Set Label", RPM: 581},
		{ID: 1, Name: "Fan 2", Label: "Rear Exhaust", RPM: 604},
		{ID: 2, Name: "Fan 3", Label: "Disconnected", RPM: 0},
	}, TemperatureProbes: []coolingpresentation.TemperatureProbe{{ID: 3, Name: "Probe 1", Label: "Radiator", Temperature: "29.0 °C"}, {ID: 4, Name: "Probe 2", Temperature: ""}}}
	connected := map[string]*common.Device{
		"commander-core":    {Serial: "commander-core", Product: "iCUE Commander CORE", Instance: dashboardAggregateProvider{dashboardCoolingProvider: dashboardCoolingProvider{id: "commander-core", snapshot: core}}},
		"commander-core-xt": {Serial: "commander-core-xt", Product: "COMMANDER CORE XT", Instance: dashboardAggregateProvider{dashboardCoolingProvider: dashboardCoolingProvider{id: "commander-core-xt", snapshot: coreXT}}},
		"simple":            {Serial: "simple", Product: "Simple Device", Instance: dashboardLightingCountProvider{id: "simple", available: true, snapshot: lightingpresentation.Snapshot{TargetKind: "native"}}},
	}

	response := dashboardCurrentDevices(connected, nil)
	if len(response.Native) != 3 {
		t.Fatalf("aggregate presentation = %#v", response)
	}
	if got := response.Native[0]; got.Serial != "commander-core" || len(got.StatusRows) != 2 || got.StatusRows[0].Label != "Coolant" || got.StatusRows[0].Value != "2400 RPM · 31.5 °C" || got.StatusRows[1].Label != "Front Intake" || got.StatusRows[1].Value != "581 RPM" {
		t.Fatalf("Commander CORE rows = %#v", got)
	}
	if got := response.Native[1]; got.Serial != "commander-core-xt" || len(got.StatusRows) != 3 || got.StatusRows[0].Label != "Fan 1" || got.StatusRows[0].Value != "581 RPM" || got.StatusRows[1].Label != "Rear Exhaust" || got.StatusRows[2].Label != "Radiator" {
		t.Fatalf("Commander CORE XT rows = %#v", got)
	}
	if got := response.Native[2]; got.Serial != "simple" || len(got.StatusRows) != 0 {
		t.Fatalf("simple card = %#v", got)
	}
}

func TestDashboardCoolingTelemetryUsesTypedSnapshotValues(t *testing.T) {
	coolant, intake, radiator := float32(31.5), float32(29), float32(27.25)
	snapshot := coolingpresentation.Snapshot{Available: true, Channels: []coolingpresentation.Channel{
		{ID: 0, Name: "Pump", RPM: 2400, ContainsPump: true, Celsius: &coolant},
		{ID: 1, Name: "Fan 1", RPM: 581}, {ID: 2, Name: "Fan 2", RPM: 604}, {ID: 3, Name: "Fan 3", RPM: 0},
	}, TemperatureProbes: []coolingpresentation.TemperatureProbe{
		{ID: 7, Name: "Probe 1", Label: "Radiator", Celsius: &radiator}, {ID: 8, Name: "Probe 2", Celsius: &intake}, {ID: 9, Name: "Probe 3"},
	}}
	telemetry := dashboardCoolingTelemetry(snapshot)
	if telemetry == nil || telemetry.AverageFanRPM == nil || *telemetry.AverageFanRPM != 593 || telemetry.CoolantCelsius == nil || *telemetry.CoolantCelsius != coolant {
		t.Fatalf("telemetry = %#v", telemetry)
	}
	if len(telemetry.TemperatureProbes) != 2 || telemetry.TemperatureProbes[0].ID != 7 || telemetry.TemperatureProbes[0].Label != "Radiator" || telemetry.TemperatureProbes[1].ID != 8 || telemetry.TemperatureProbes[1].Label != "Probe 2" {
		t.Fatalf("probe telemetry = %#v", telemetry.TemperatureProbes)
	}
	if dashboardCoolingTelemetry(coolingpresentation.Snapshot{Available: true, Channels: []coolingpresentation.Channel{{ContainsPump: true, RPM: 2400}}}) != nil {
		t.Fatal("pump-only snapshot produced Dashboard telemetry")
	}
}

func TestDashboardPerformanceStatusKeepsUsefulMouseAndKeyboardFacts(t *testing.T) {
	mouse := &devicesWorkspaceSummary{
		OverviewPerformance: &devicesOverviewPerformanceStatusSummary{Rows: []devicesOverviewStatusRow{{Label: "DPI", Value: "1600"}, {Label: "Polling Rate", Value: "1000 Hz"}}},
		Performance:         &devicesPerformanceWorkspaceSummary{LiftHeight: &devicesPerformanceSelectSummary{Value: 2, Options: []devicesPerformanceOptionSummary{{Value: 2, Label: "Low"}}}},
	}
	mouseRows := dashboardDeviceStatus(mouse)
	if len(mouseRows) != 2 || mouseRows[0].Label != "DPI" || mouseRows[1].Label != "Polling Rate" {
		t.Fatalf("mouse Dashboard rows = %#v", mouseRows)
	}
	for _, row := range mouseRows {
		if row.Label == "Lift Height" {
			t.Fatalf("mouse Dashboard retained lift height: %#v", mouseRows)
		}
	}
	if mouse.Performance.LiftHeight == nil || mouse.Performance.LiftHeight.Options[0].Label != "Low" {
		t.Fatalf("Devices workspace lift height changed: %#v", mouse.Performance)
	}

	keyboard := &devicesWorkspaceSummary{
		OverviewPerformance: &devicesOverviewPerformanceStatusSummary{Rows: []devicesOverviewStatusRow{{Label: "Polling Rate", Value: "8000 Hz"}}},
		KeyboardAssignments: &devicesKeyboardAssignmentsWorkspaceSummary{KeyboardLayouts: []string{"US", "UK"}, ActiveKeyboardLayout: "US"},
	}
	keyboardRows := dashboardDeviceStatus(keyboard)
	if len(keyboardRows) != 1 || keyboardRows[0].Label != "Polling Rate" || keyboardRows[0].Value != "8000 Hz" {
		t.Fatalf("keyboard Dashboard rows = %#v", keyboardRows)
	}
	for _, row := range keyboardRows {
		if row.Label == "Keyboard Layout" {
			t.Fatalf("keyboard Dashboard retained keyboard layout: %#v", keyboardRows)
		}
	}
	if len(keyboard.KeyboardAssignments.KeyboardLayouts) != 2 || keyboard.KeyboardAssignments.ActiveKeyboardLayout != "US" {
		t.Fatalf("Devices workspace keyboard layouts changed: %#v", keyboard.KeyboardAssignments)
	}
}

func TestDashboardControllerCardsUseNaturalHeight(t *testing.T) {
	stylesheet, err := os.ReadFile(filepath.Join("..", "..", "static", "css", "app-shell.css"))
	if err != nil {
		t.Fatal(err)
	}

	rule := func(selector string) string {
		t.Helper()
		start := strings.Index(string(stylesheet), selector+" {")
		if start < 0 {
			t.Fatalf("missing dashboard CSS rule %q", selector)
		}
		end := strings.Index(string(stylesheet[start:]), "}\n")
		if end < 0 {
			t.Fatalf("unterminated dashboard CSS rule %q", selector)
		}
		return string(stylesheet[start : start+end])
	}

	if grid := rule(".lf-dashboard-device-grid"); !strings.Contains(grid, "display: flex;") || !strings.Contains(grid, "flex-wrap: wrap;") || !strings.Contains(grid, "gap: 10px;") {
		t.Errorf("Dashboard device grid still stretches cards: %q", grid)
	}
	lane := rule(".lf-dashboard-device-lane")
	if !strings.Contains(lane, "flex-direction: column;") || !strings.Contains(lane, "align-items: stretch;") || !strings.Contains(lane, "width: min(300px, 100%);") || !strings.Contains(lane, "gap: 10px;") {
		t.Errorf("Dashboard device lane does not preserve independent natural-height stacks: %q", lane)
	}
	if strings.Contains(string(stylesheet), "width: min(310px, 100%);") || !strings.Contains(string(stylesheet), "minmax(max-content, 1fr)") {
		t.Error("Dense Dashboard lanes do not preserve wider one-line telemetry")
	}
	card := rule(".lf-dashboard-device-card")
	if !strings.Contains(card, "align-self: stretch;") || !strings.Contains(card, "width: 100%;") || !strings.Contains(card, "box-sizing: border-box;") || strings.Contains(card, "height:") || strings.Contains(card, "overflow:") {
		t.Errorf("Dashboard controller card can truncate cooling rows: %q", card)
	}
	wrapper := rule(".lf-dashboard-layout-card")
	if !strings.Contains(wrapper, "min-height: max-content;") || !strings.Contains(wrapper, "height: auto;") || !strings.Contains(wrapper, "overflow: visible;") || !strings.Contains(wrapper, "align-self: stretch;") || !strings.Contains(wrapper, "width: 100%;") || !strings.Contains(wrapper, "box-sizing: border-box;") {
		t.Errorf("Dashboard movable wrapper can clip card content: %q", wrapper)
	}
	if strings.Contains(string(stylesheet), "lf-dashboard-card-resize") || strings.Contains(string(stylesheet), "nwse-resize") {
		t.Error("Dashboard retained a resize affordance")
	}
	statusStart := strings.Index(string(stylesheet), ".lf-dashboard-device-state strong,")
	if statusStart < 0 {
		t.Fatal("missing Dashboard telemetry value rule")
	}
	statusEnd := strings.Index(string(stylesheet[statusStart:]), "}\n")
	if statusEnd < 0 {
		t.Fatal("missing Dashboard telemetry value rule")
	}
	status := string(stylesheet[statusStart : statusStart+statusEnd])
	if !strings.Contains(status, "white-space: normal;") || !strings.Contains(status, "overflow-wrap: anywhere;") || strings.Contains(status, "text-overflow: ellipsis;") {
		t.Errorf("Dashboard telemetry values can still be truncated: %q", status)
	}
	title := rule(".lf-dashboard-layout-card .lf-dashboard-device-title")
	if !strings.Contains(title, "padding-right: 132px;") || !strings.Contains(title, "white-space: normal;") {
		t.Errorf("Dashboard device titles can overlap movement controls: %q", title)
	}
	dragging := rule(".lf-dashboard-card-dragging .lf-dashboard-device-card")
	if !strings.Contains(dragging, "translateY(-4px)") || !strings.Contains(dragging, "box-shadow:") {
		t.Errorf("Dashboard dragged card lacks lifted feedback: %q", dragging)
	}
}

func TestDashboardMembershipRoutesAreRemovedWhileCurrentRoutesRemain(t *testing.T) {
	source, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []string{
		`"/api/dashboard/devices/get"`,
		`"/api/dashboard/devices/add"`,
		`"/api/dashboard/devices/order"`,
		`"/api/dashboard/devices/delete"`,
	} {
		if strings.Contains(string(source), legacy) {
			t.Fatalf("legacy Dashboard membership route remains registered: %s", legacy)
		}
	}
	for _, current := range []string{`"/api/dashboard/devices/current"`, `"/api/dashboard/layout"`} {
		if !strings.Contains(string(source), current) {
			t.Fatalf("current Dashboard route was removed: %s", current)
		}
	}
}

func TestDashboardSystemOverviewTemplateUsesFixedTelemetryPresentation(t *testing.T) {
	initializeDevicesPageTestProcess(t)

	storage := []systeminfo.StorageData{{Key: "nvme0n1", Model: "WD_BLACK SN850X", Temperature: 45}}
	page := templates.Web{
		Page: "index",
		Dashboard: dashboard.Dashboard{
			Celsius:        false,
			TemperatureBar: true,
		},
		CpuTemp:         "96.8 °F",
		CpuTempCelsius:  36,
		DashboardMemory: []dashboardMemoryTemperature{{Serial: "memory", ChannelID: 0, Identifier: "DIMM 1", Name: "Memory family", Temperature: "89.6 °F", Celsius: 32}},
		BuildInfo:       &version.BuildInfo{Revision: "test", BuildVersion: "test"},
		SystemInfo: &systeminfo.SystemInfo{
			CPU: &systeminfo.CpuData{Model: "AMD Ryzen 9"},
			GPU: map[int]systeminfo.GpuData{
				0: {Index: 0, Model: "NVIDIA RTX 4090", Temperature: 55},
				1: {Index: 1, Model: "NVIDIA RTX 3080", Temperature: 64},
			},
			Storage: &storage,
		},
	}

	var rendered bytes.Buffer
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "index.html", page); err != nil {
		t.Fatal(err)
	}
	html := rendered.String()
	for _, want := range []string{
		`class="lf-app-shell lf-cluster-shell lf-dashboard-shell"`,
		`class="lf-global-nav" aria-label="Application navigation"`,
		`class="lf-global-link lf-global-link-active" href="/" aria-label="Dashboard" title="Dashboard" aria-current="page"`,
		`class="lf-dashboard-overview"`,
		`data-lf-dashboard-cpu-temperature`,
		`data-lf-dashboard-gpu-temperature="0"`,
		`data-lf-dashboard-gpu-temperature="1"`,
		`data-lf-dashboard-gauge="memory"`,
		`data-lf-dashboard-memory-temperature`,
		`>DIMM 1<`,
		`class="lf-dashboard-gauge-model" title="AMD Ryzen 9">AMD Ryzen 9<`,
		`class="lf-dashboard-gauge-model" title="NVIDIA RTX 4090">NVIDIA RTX 4090<`,
		`class="lf-dashboard-gauge-model" title="Memory family">Memory family<`,
		`data-lf-dashboard-storage-temperature="nvme0n1"`,
		`class="lf-telemetry-value"`,
		`href="/rgbCluster"`,
		`data-lf-dashboard-devices`,
		`data-lf-dashboard-device-grid`,
		`class="lf-dashboard-gauge-visual"`,
		`lighting: "`,
		`brightness: "`,
		`celsius: `,
		`>96.8 °F<`,
		`>131.0 °F<`,
		`>89.6 °F<`,
		`>113.0 °F<`,
		`Clustered Lighting Devices`,
		`Independent Lighting Devices`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("dashboard overview template missing %q", want)
		}
	}
	if !regexp.MustCompile(`celsius:\s*false\s*,`).MatchString(html) {
		t.Error("dashboard unit bridge did not pass Fahrenheit preference to the client")
	}
	for _, absent := range []string{
		`id="dashboard-overview-card"`,
		`class="sidebar sidebar-sections-initializing`,
		`sidebar-section-toggle`,
		`id="sidebarToggle"`,
		`id="dashboardDeviceSelect"`,
		`id="addDeviceToDashboard"`,
		`id="deleteDeviceFromDashboard"`,
		`id="system-cards"`,
		`Non-Cluster RGB Devices`,
		`Cluster Members`,
		`lighting: "\"Lighting\""`,
		`brightness: "\"Brightness\""`,
	} {
		if strings.Contains(html, absent) {
			t.Errorf("dashboard overview template retained removed control %q", absent)
		}
	}
	lighting := strings.Index(html, `class="lf-dashboard-lighting-summary"`)
	cpu := strings.Index(html, `data-lf-dashboard-gauge="cpu"`)
	gpu := strings.Index(html, `data-lf-dashboard-gauge="gpu"`)
	storageIndex := strings.Index(html, `class="lf-dashboard-storage"`)
	if lighting == -1 || cpu == -1 || gpu == -1 || storageIndex == -1 || !(lighting < cpu && cpu < gpu && gpu < storageIndex) {
		t.Errorf("dashboard overview order = lighting:%d cpu:%d gpu:%d storage:%d, want Lighting before CPU before GPU before Storage", lighting, cpu, gpu, storageIndex)
	}
}

func TestDashboardLowerSectionsUseLocalizedHeadingsAndLabels(t *testing.T) {
	templateSource, err := os.ReadFile(filepath.Join("..", "..", "web", "index.html"))
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`<h2 id="lf-dashboard-devices-title">{{ .Lang "txtDashboardDevices" }}</h2>`,
		`window.dashboardI18n = {`,
		`lighting: {{ .Lang "txtLighting" }},`,
		`brightness: {{ .Lang "txtBrightness" }},`,
		`moveEarlier: "Move earlier",`,
	} {
		if !strings.Contains(string(templateSource), want) {
			t.Errorf("dashboard localization template missing %q", want)
		}
	}
}
