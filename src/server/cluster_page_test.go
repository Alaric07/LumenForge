package server

import (
	"LumenForge/src/cluster"
	"LumenForge/src/common"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/templates"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const clusterPageHelperEnvironment = "LUMENFORGE_CLUSTER_PAGE_TEST_HELPER"

func TestRGBClusterPageUsesCanonicalLightingWorkspace(t *testing.T) {
	if os.Getenv(clusterPageHelperEnvironment) == "1" {
		runRGBClusterPageAssertions(t)
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestRGBClusterPageUsesCanonicalLightingWorkspace$")
	command.Env = append(os.Environ(), clusterPageHelperEnvironment+"=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("RGB Cluster page helper process failed: %v\n%s", err, output)
	}
}

func runRGBClusterPageAssertions(t *testing.T) {
	initializeDevicesPageTestProcess(t)
	previousStatus := getRGBClusterLightingStatus
	t.Cleanup(func() { getRGBClusterLightingStatus = previousStatus })

	staticSnapshot := cluster.LightingSnapshot{
		SelectedEffect:  "static",
		Brightness:      60,
		ControllerCount: 2,
		Available:       true,
		Settings: lightingsettings.EffectSettings{
			SchemaVersion: lightingsettings.SchemaVersion,
			EffectID:      "static",
			SingleColor: &lightingsettings.SingleColorSettings{
				Color: lightingsettings.Color{Red: 17, Green: 34, Blue: 51},
			},
		},
	}
	getRGBClusterLightingStatus = func() (cluster.LightingSnapshot, int) {
		return staticSnapshot, staticSnapshot.ControllerCount
	}

	request := httptest.NewRequest(http.MethodGet, "/rgbCluster", nil)
	request.Host = "127.0.0.1:27003"
	recorder := httptest.NewRecorder()
	setRoutes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /rgbCluster status = %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		`class="lf-app-shell lf-cluster-shell"`,
		`class="lf-global-link lf-global-link-active" href="/rgbCluster"`,
		`href="/static/css/app-shell.css"`,
		`src="/static/js/devices-lighting.js?v=4"`,
		`src="/static/js/cluster.js?v=4"`,
		`<h1 class="lf-workspace-title">RGB Cluster</h1>`,
		`data-lf-brightness-readout data-lf-lighting-target="cluster">60%</strong>`,
		`id="lf-cluster-effect-selector"`,
		`data-lf-lighting-target="cluster"`,
		`data-lf-current-effect="static"`,
		`<option value="off">Off</option>`,
		`value="static" selected>Static</option>`,
		`id="lf-cluster-brightness-slider"`,
		`data-lf-current-brightness="60"`,
		`id="lf-cluster-color-input"`,
		`value="#112233"`,
		`data-lf-reset-control`,
		`data-lf-reset-button`,
		`Reset to default`,
		`id="clusterSortable"`,
		`No cluster members configured.`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("GET /rgbCluster response does not contain %q", expected)
		}
	}
	resetControlTag := clusterPageOpeningTag(t, body, `data-lf-reset-control`)
	resetButtonTag := clusterPageOpeningTag(t, body, `data-lf-reset-button`)
	for _, tag := range []string{resetControlTag, resetButtonTag} {
		if !strings.Contains(tag, `data-lf-lighting-target="cluster"`) || !strings.Contains(tag, `data-lf-effect="static"`) || strings.Contains(tag, `data-lf-device-serial`) {
			t.Errorf("Cluster Reset tag has the wrong target contract: %s", tag)
		}
	}
	if !strings.Contains(resetControlTag, `hidden`) {
		t.Errorf("pristine Cluster Reset control is not hidden: %s", resetControlTag)
	}
	for _, obsolete := range []string{
		`clusterLightingToggle`,
		`clusterRgbProfile`,
		`btnApplySolidColor`,
		`brightnessSliderValue`,
		`LastNonOffProfile`,
		`/api/color`,
		`/api/brightness/gradual`,
		`DeviceProfile`,
	} {
		if strings.Contains(body, obsolete) {
			t.Errorf("GET /rgbCluster response unexpectedly contains %q", obsolete)
		}
	}
	if count := strings.Count(body, "<h1 "); count != 1 {
		t.Errorf("GET /rgbCluster h1 count = %d, want 1", count)
	}
	customizedSnapshot := staticSnapshot
	customizedSnapshot.Customized = true
	customizedBody := renderRGBClusterPage(t, clusterLightingWorkspaceSummaryFromSnapshot(customizedSnapshot), nil)
	if tag := clusterPageOpeningTag(t, customizedBody, `data-lf-reset-control`); strings.Contains(tag, `hidden`) {
		t.Errorf("customized Cluster Reset control is hidden: %s", tag)
	}
	unavailableBody := renderRGBClusterPage(t, clusterLightingWorkspaceSummaryFromSnapshot(cluster.LightingSnapshot{}), nil)
	if strings.Contains(unavailableBody, `data-lf-reset-control`) || strings.Contains(unavailableBody, `data-lf-reset-button`) {
		t.Error("unavailable Cluster lighting rendered Reset")
	}

	device := &cluster.Device{
		Controllers: []*common.ClusterController{
			{Product: "First <Member>", Serial: "member-one"},
			{Product: "Second & Member", Serial: "member-two"},
		},
	}
	memberBody := renderRGBClusterPage(t, clusterLightingWorkspaceSummaryFromSnapshot(staticSnapshot), device)
	first := strings.Index(memberBody, `data-serial="member-one"`)
	second := strings.Index(memberBody, `data-serial="member-two"`)
	if first < 0 || second < 0 || first >= second {
		t.Errorf("Cluster member order was not preserved: first=%d second=%d", first, second)
	}
	for _, expected := range []string{"First &lt;Member&gt;", "Second &amp; Member"} {
		if !strings.Contains(memberBody, expected) {
			t.Errorf("Cluster member rendering does not contain %q", expected)
		}
	}
	assertRGBClusterPaletteGates(t)
}

func renderRGBClusterPage(t *testing.T, lighting *clusterLightingWorkspaceSummary, device *cluster.Device) string {
	t.Helper()
	var rendered bytes.Buffer
	page := struct {
		templates.Web
		Lighting *clusterLightingWorkspaceSummary
	}{
		Web:      templates.Web{Device: device, Page: "rgbCluster"},
		Lighting: lighting,
	}
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "cluster.html", page); err != nil {
		t.Fatal(err)
	}
	return rendered.String()
}

func clusterPageOpeningTag(t *testing.T, body, marker string) string {
	t.Helper()
	markerIndex := strings.Index(body, marker)
	if markerIndex < 0 {
		t.Fatalf("Cluster page does not contain %q", marker)
	}
	start := strings.LastIndex(body[:markerIndex], "<")
	endOffset := strings.Index(body[markerIndex:], ">")
	if start < 0 || endOffset < 0 {
		t.Fatalf("Cluster page has malformed tag around %q", marker)
	}
	return body[start : markerIndex+endOffset+1]
}

func assertRGBClusterPaletteGates(t *testing.T) {
	t.Helper()
	speed := 4.0
	tests := []struct {
		name     string
		snapshot cluster.LightingSnapshot
		present  []string
		absent   []string
	}{
		{
			name: "two color with Speed",
			snapshot: cluster.LightingSnapshot{SelectedEffect: "wave", Brightness: 50, Available: true,
				Settings: lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "wave", Speed: &speed,
					TwoColor: &lightingsettings.TwoColorSettings{Start: lightingsettings.Color{Red: 1}, End: lightingsettings.Color{Blue: 2}}}},
			present: []string{`data-lf-speed-slider`, `data-lf-two-color-control`, `value="#010000"`, `value="#000002"`},
			absent:  []string{`data-lf-color-input`, `data-lf-temperature-control`, `data-lf-gradient-control`},
		},
		{
			name: "temperature",
			snapshot: cluster.LightingSnapshot{SelectedEffect: "cpu-temperature", Brightness: 50, Available: true,
				Settings: lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "cpu-temperature",
					Temperature: &lightingsettings.TemperatureSettings{
						Low:    lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Blue: 1}, Celsius: 20},
						Middle: lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Green: 2}, Celsius: 50},
						High:   lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Red: 3}, Celsius: 95},
					}}},
			present: []string{`data-lf-temperature-control`, `lf-cluster-temperature-low-color`, `lf-cluster-temperature-middle-color`, `lf-cluster-temperature-high-color`},
			absent:  []string{`data-lf-speed-slider`, `data-lf-color-input`, `data-lf-two-color-control`, `data-lf-gradient-control`},
		},
		{
			name: "Gradient",
			snapshot: cluster.LightingSnapshot{SelectedEffect: "gradient", Brightness: 50, Available: true,
				Settings: lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "gradient", Speed: &speed,
					Gradient: &lightingsettings.GradientSettings{Stops: []lightingsettings.GradientStop{
						{Position: 0, Color: lightingsettings.Color{Red: 1}, Intensity: 0.5},
						{Position: 1, Color: lightingsettings.Color{Blue: 2}, Intensity: 1},
					}}}},
			present: []string{`data-lf-speed-slider`, `data-lf-gradient-control`, `data-lf-gradient-add`, `data-lf-gradient-save`},
			absent:  []string{`data-lf-color-input`, `data-lf-two-color-control`, `data-lf-temperature-control`},
		},
		{
			name: "Off",
			snapshot: cluster.LightingSnapshot{SelectedEffect: "off", Brightness: 0, Available: true,
				Settings: lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "off"}},
			present: []string{`value="off" selected>Off</option>`, `data-lf-current-effect="off"`},
			absent:  []string{`data-lf-speed-slider`, `data-lf-color-input`, `data-lf-two-color-control`, `data-lf-temperature-control`, `data-lf-gradient-control`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := renderRGBClusterPage(t, clusterLightingWorkspaceSummaryFromSnapshot(test.snapshot), nil)
			if tag := clusterPageOpeningTag(t, body, `data-lf-reset-control`); !strings.Contains(tag, `hidden`) {
				t.Errorf("pristine Cluster %s Reset control is not hidden: %s", test.name, tag)
			}
			for _, expected := range test.present {
				if !strings.Contains(body, expected) {
					t.Errorf("Cluster %s page does not contain %q", test.name, expected)
				}
			}
			for _, excluded := range test.absent {
				if strings.Contains(body, excluded) {
					t.Errorf("Cluster %s page unexpectedly contains %q", test.name, excluded)
				}
			}
		})
	}
}
