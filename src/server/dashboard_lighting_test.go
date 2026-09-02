package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"LumenForge/src/cluster"
	"LumenForge/src/common"
	"LumenForge/src/dashboard"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/systeminfo"
	"LumenForge/src/templates"
	"LumenForge/src/version"
)

type dashboardLightingCountProvider struct {
	id        string
	snapshot  lightingpresentation.Snapshot
	available bool
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
		name              string
		connected         map[string]*common.Device
		clustered         int
		independent       int
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

func TestDashboardSystemOverviewTemplateUsesFixedTelemetryPresentation(t *testing.T) {
	initializeDevicesPageTestProcess(t)

	storage := []systeminfo.StorageData{{Key: "nvme0n1", Model: "WD_BLACK SN850X", Temperature: 45}}
	page := templates.Web{
		Page: "index",
		Dashboard: dashboard.Dashboard{
			Celsius:        true,
			TemperatureBar: true,
		},
		CpuTemp:        "36.0 °C",
		CpuTempCelsius: 36,
		BuildInfo: &version.BuildInfo{Revision: "test", BuildVersion: "test"},
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
		`data-lf-dashboard-storage-temperature="nvme0n1"`,
		`class="lf-telemetry-value"`,
		`href="/rgbCluster"`,
		`id="system-cards" class="device-placeholder`,
		`class="lf-dashboard-gauge-visual"`,
		`Clustered Lighting Devices`,
		`Independent Lighting Devices`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("dashboard overview template missing %q", want)
		}
	}
	for _, absent := range []string{
		`id="dashboard-overview-card"`,
		`class="sidebar sidebar-sections-initializing`,
		`sidebar-section-toggle`,
		`id="sidebarToggle"`,
		`id="dashboardDeviceSelect"`,
		`id="addDeviceToDashboard"`,
		`id="deleteDeviceFromDashboard"`,
		`Non-Cluster RGB Devices`,
		`Cluster Members`,
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
