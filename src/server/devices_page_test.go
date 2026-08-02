package server

import (
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/devices"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/rgb"
	"LumenForge/src/stats"
	"LumenForge/src/templates"
	"bytes"
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
	source := openrgbimport.LightingSnapshot{
		HasActiveProfile:  true,
		ConfiguredEffect:  "wave",
		EffectSupported:   true,
		HasBrightness:     true,
		Brightness:        0,
		ClusterControlled: true,
		SupportedEffects: []openrgbimport.LightingEffectOption{
			{
				ID:              "wave",
				Label:           "Wave <Label> & More",
				CapabilityKnown: true,
				Capability: rgb.LightingEffectCapability{
					Palette:        rgb.LightingPaletteTwoColor,
					UsesStartColor: true,
					UsesEndColor:   true,
					SupportsSpeed:  true,
				},
			},
			{ID: "future-effect", Label: "Future Effect", CapabilityKnown: false},
		},
		BaseDefinition: &openrgbimport.LightingDefinitionSnapshot{
			Palette:       rgb.LightingPaletteTwoColor,
			HasStartColor: true,
			StartColor:    rgb.Color{Red: -1, Green: 15.9, Blue: 300, Hex: "not-used"},
			HasEndColor:   true,
			EndColor:      rgb.Color{},
			HasSpeed:      true,
			Speed:         0,
		},
		Override: &openrgbimport.LightingOverrideSnapshot{
			Enabled:    false,
			StartColor: rgb.Color{},
			EndColor:   rgb.Color{Red: 255, Green: 128, Blue: 1},
			Speed:      0,
		},
	}
	source.Effective = source.BaseDefinition

	summary := openRGBLightingWorkspaceSummaryFromSnapshot(source)
	if !summary.HasActiveProfile || summary.ConfiguredEffect != "wave" || summary.ConfiguredEffectLabel != "Wave <Label> & More" ||
		!summary.EffectSupported || !summary.ConfiguredCapabilityKnown || summary.ConfiguredPalette != "Two color" ||
		!summary.ConfiguredSupportsSpeed || !summary.HasBrightness || summary.Brightness != 0 || !summary.ClusterControlled {
		t.Fatalf("configured Lighting presentation = %#v", summary)
	}
	if len(summary.SupportedEffects) != 2 ||
		summary.SupportedEffects[0].ID != "future-effect" || summary.SupportedEffects[0].Label != "Future Effect" ||
		summary.SupportedEffects[0].Palette != "Unknown" || summary.SupportedEffects[0].CapabilityKnown || summary.SupportedEffects[0].SupportsSpeed || summary.SupportedEffects[0].Selected ||
		summary.SupportedEffects[1].ID != "wave" || summary.SupportedEffects[1].Label != "Wave <Label> & More" ||
		summary.SupportedEffects[1].Palette != "Two color" || !summary.SupportedEffects[1].CapabilityKnown || !summary.SupportedEffects[1].SupportsSpeed || !summary.SupportedEffects[1].Selected {
		t.Fatalf("supported Lighting effects = %#v", summary.SupportedEffects)
	}
	if summary.BaseDefinition == nil || !summary.BaseDefinition.HasStart || summary.BaseDefinition.Start.Hex != "#000FFF" ||
		summary.BaseDefinition.Start.RGB != "RGB 0, 15, 255" || !summary.BaseDefinition.HasEnd ||
		summary.BaseDefinition.End.Hex != "#000000" || !summary.BaseDefinition.HasSpeed || summary.BaseDefinition.Speed != "0" {
		t.Fatalf("base Lighting definition = %#v", summary.BaseDefinition)
	}
	if summary.Override == nil || summary.Override.Enabled || summary.Override.Start.Hex != "#000000" ||
		summary.Override.End.Hex != "#FF8001" || summary.Override.Speed != "0" {
		t.Fatalf("Lighting override = %#v", summary.Override)
	}

	source.SupportedEffects[0].ID = "mutated"
	source.BaseDefinition.StartColor.Red = 255
	source.Override.StartColor.Red = 255
	if summary.SupportedEffects[1].ID != "wave" || summary.BaseDefinition.Start.Hex != "#000FFF" || summary.Override.Start.Hex != "#000000" {
		t.Fatal("Lighting presentation retained mutable snapshot data")
	}

	for _, test := range []struct {
		known   bool
		palette rgb.LightingPaletteKind
		label   string
	}{
		{known: true, palette: rgb.LightingPaletteNone, label: "None"},
		{known: true, palette: rgb.LightingPaletteStaticSingle, label: "Single color"},
		{known: true, palette: rgb.LightingPaletteTwoColor, label: "Two color"},
		{known: true, palette: rgb.LightingPaletteTemperatureThree, label: "Temperature"},
		{known: true, palette: rgb.LightingPaletteGradient, label: "Gradient"},
		{known: true, palette: rgb.LightingPaletteGenerated, label: "Generated palette"},
		{known: false, palette: rgb.LightingPaletteTwoColor, label: "Unknown"},
	} {
		if label := openRGBLightingPaletteLabel(test.known, test.palette); label != test.label {
			t.Errorf("palette label = %q, want %q", label, test.label)
		}
	}
}

func TestDevicesLightingEffectSelectorPresentation(t *testing.T) {
	source := openrgbimport.LightingSnapshot{
		ConfiguredEffect: "wave",
		EffectSupported:  true,
		SupportedEffects: []openrgbimport.LightingEffectOption{
			{ID: "wave", Label: "Wave"},
			{ID: "aurora-z", Label: "aurora"},
			{ID: "off", Label: "Off"},
			{ID: "circle", Label: "Circle"},
			{ID: "aurora-a", Label: "Aurora"},
		},
	}
	summary := openRGBLightingWorkspaceSummaryFromSnapshot(source)
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

	emptyLabelOff := openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		SupportedEffects: []openrgbimport.LightingEffectOption{{ID: "off"}},
	})
	if len(emptyLabelOff.SupportedEffects) != 1 || emptyLabelOff.SupportedEffects[0].ID != "off" || emptyLabelOff.SupportedEffects[0].Label != "Off" {
		t.Fatalf("empty-label supported Off presentation = %#v", emptyLabelOff.SupportedEffects)
	}

	withoutOff := openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		SupportedEffects: []openrgbimport.LightingEffectOption{{ID: "static", Label: "Static"}},
	})
	if len(withoutOff.SupportedEffects) != 1 || withoutOff.SupportedEffects[0].ID != "static" {
		t.Fatalf("presentation fabricated an effect absent from the snapshot: %#v", withoutOff.SupportedEffects)
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

func runDevicesLightingEffectSelectorTemplateAssertions(t *testing.T) {

	normalBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		ConfiguredEffect: "wave",
		EffectSupported:  true,
		SupportedEffects: []openrgbimport.LightingEffectOption{
			{ID: "wave", Label: "Wave <Bright> & Wide"},
			{ID: "off", Label: "Off"},
		},
	}))
	for _, expected := range []string{
		`<label class="lf-lighting-label" for="lf-lighting-effect-selector">Configured effect</label>`,
		`data-lf-effect-selector`,
		`data-lf-device-serial="lighting-template-device"`,
		`data-lf-current-effect="wave"`,
		`<option value="off">Off</option>`,
		`value="wave" selected>Wave &lt;Bright&gt; &amp; Wide</option>`,
		`Stable ID <code class="lf-lighting-effect-id">wave</code>`,
		`id="lf-lighting-effect-status" aria-live="polite"`,
	} {
		if !strings.Contains(normalBody, expected) {
			t.Errorf("normal effect selector does not contain %q", expected)
		}
	}
	if strings.Count(normalBody, "<option ") != 2 {
		t.Errorf("normal effect selector rendered %d options, want exactly the two snapshot options", strings.Count(normalBody, "<option "))
	}
	if strings.Index(normalBody, `<option value="off">Off</option>`) > strings.Index(normalBody, `value="wave" selected>`) {
		t.Error("Off is not alphabetized before Wave in the rendered effect selector")
	}
	if strings.Contains(normalBody, "Controlled by RGB Cluster") || strings.Contains(normalBody, `id="lf-lighting-effect-cluster-explanation"`) {
		t.Error("non-clustered effect selector renders the cluster ownership explanation")
	}

	withoutOffBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		SupportedEffects: []openrgbimport.LightingEffectOption{{ID: "static", Label: "Static"}},
	}))
	if strings.Contains(withoutOffBody, `value="off"`) || strings.Contains(withoutOffBody, `>Off</option>`) {
		t.Error("effect selector fabricated Off when the snapshot did not report it")
	}

	emptyBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		SupportedEffects: []openrgbimport.LightingEffectOption{{ID: "off", Label: "Off"}},
	}))
	for _, expected := range []string{
		`<option value="" selected disabled>Not configured</option>`,
		`<option value="off">Off</option>`,
	} {
		if !strings.Contains(emptyBody, expected) {
			t.Errorf("empty configured effect selector does not contain %q", expected)
		}
	}
	if strings.Contains(emptyBody, `Stable ID <code`) {
		t.Error("empty configured effect fabricated a stable ID")
	}

	unsupportedBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		ConfiguredEffect: "legacy<effect>",
		EffectSupported:  false,
		SupportedEffects: []openrgbimport.LightingEffectOption{{ID: "static", Label: "Static & Safe"}},
	}))
	for _, expected := range []string{
		`value="legacy&lt;effect&gt;" selected disabled>Unsupported: legacy&lt;effect&gt;</option>`,
		`<option value="static">Static &amp; Safe</option>`,
		`Stable ID <code class="lf-lighting-effect-id">legacy&lt;effect&gt;</code>`,
	} {
		if !strings.Contains(unsupportedBody, expected) {
			t.Errorf("unsupported configured effect selector does not contain %q", expected)
		}
	}
	if strings.Contains(unsupportedBody, "legacy<effect>") || strings.Contains(unsupportedBody, "Static & Safe") {
		t.Error("effect selector rendered an unescaped stable ID or display label")
	}

	clusterBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		ConfiguredEffect:  "static",
		EffectSupported:   true,
		ClusterControlled: true,
		SupportedEffects:  []openrgbimport.LightingEffectOption{{ID: "static", Label: "Static"}},
	}))
	for _, expected := range []string{
		`data-lf-cluster-controlled="true"`,
		`aria-describedby="lf-lighting-effect-status lf-lighting-effect-cluster-explanation"`,
		`id="lf-lighting-effect-cluster-explanation"`,
		`Controlled by RGB Cluster. Change active lighting from the <a href="/rgbCluster">RGB Cluster workspace</a>.`,
		`id="lf-lighting-cluster-note"`,
	} {
		if !strings.Contains(clusterBody, expected) {
			t.Errorf("cluster-controlled effect selector does not contain %q", expected)
		}
	}
	if strings.Count(clusterBody, "RGB Cluster owns output") != 1 {
		t.Error("cluster-controlled Lighting view does not retain exactly one ownership explanation")
	}
	if strings.Count(clusterBody, "Controlled by RGB Cluster. Change active lighting") != 1 {
		t.Error("cluster-controlled Lighting view does not render exactly one concise inline explanation")
	}
	selectorStart := strings.Index(clusterBody, `id="lf-lighting-effect-selector"`)
	if selectorStart < 0 {
		t.Fatal("cluster-controlled Lighting view does not render the effect selector")
	}
	selectorEnd := strings.Index(clusterBody[selectorStart:], ">")
	if selectorEnd < 0 || !strings.Contains(clusterBody[selectorStart:selectorStart+selectorEnd], "disabled") {
		t.Error("cluster-controlled effect selector is not disabled")
	}

	unavailableBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{}))
	if !strings.Contains(unavailableBody, "Effect selection unavailable") ||
		!strings.Contains(unavailableBody, "No supported effects were reported for this controller.") ||
		strings.Contains(unavailableBody, "data-lf-effect-selector") || strings.Contains(unavailableBody, "<select") {
		t.Error("missing supported effects did not produce the safe unavailable selector state")
	}
}

func renderDevicesLightingView(t *testing.T, lighting *openRGBLightingWorkspaceSummary) string {
	t.Helper()
	const serial = "lighting-template-device"
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
	visibleBrightness := uint8(0)
	black := rgb.Color{}
	staticEnd := rgb.Color{Red: 20, Green: 30, Blue: 40, Brightness: 1}
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
			Active:           true,
			RGBProfile:       "static",
			BrightnessSlider: &visibleBrightness,
			RGBCluster:       true,
			RGBOverride: &openrgbimport.RGBOverride{
				Enabled:        true,
				RGBStartColor:  black,
				RGBMiddleColor: rgb.Color{Red: 50, Green: 60, Blue: 70, Brightness: 1},
				RGBEndColor:    rgb.Color{Red: 80, Green: 90, Blue: 100, Brightness: 1},
				RgbModeSpeed:   0,
			},
		},
		RGBModes: []string{"static", "wave", "cpu-temperature", "gradient", "rainbow", "off"},
		Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{
			"static":          {ProfileName: "Static <Effect> & More", StartColor: black, EndColor: staticEnd},
			"wave":            {ProfileName: "Wave", StartColor: black, EndColor: staticEnd, Speed: 2},
			"cpu-temperature": {ProfileName: "CPU Temperature"},
			"gradient":        {ProfileName: "Gradient", Speed: 4},
			"rainbow":         {ProfileName: "Rainbow", Speed: 5},
			"off":             {ProfileName: "Off"},
		}},
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
		"Effect control",
		"<nav class=\"lf-device-workspace-nav\" aria-label=\"Device workspace\">",
		"class=\"lf-device-workspace-link\" href=\"/devices?device=" + visibleSerial + "\">Overview</a>",
		"class=\"lf-device-workspace-link lf-device-workspace-link-active\" href=\"/devices?device=" + visibleSerial + "&amp;view=lighting\" aria-current=\"page\">Lighting</a>",
		">Static</strong>",
		"<code class=\"lf-lighting-effect-id\">static</code>",
		"data-lf-device-serial=\"" + visibleSerial + "\"",
		"data-lf-current-effect=\"static\"",
		"data-lf-cluster-controlled=\"true\"",
		"id=\"lf-lighting-effect-selector\"",
		"<option value=\"off\">Off</option>",
		"value=\"static\" selected>Static</option>",
		"aria-describedby=\"lf-lighting-effect-status lf-lighting-effect-cluster-explanation\"",
		"Controlled by RGB Cluster. Change active lighting from the <a href=\"/rgbCluster\">RGB Cluster workspace</a>.",
		"id=\"lf-lighting-effect-status\" aria-live=\"polite\"",
		"lf-lighting-status-supported\">Supported",
		"<small>Brightness</small><strong>0%</strong>",
		"RGB Cluster owns output",
		"Local configuration remains stored",
		"RGB Cluster owned",
		"Device RGB definition",
		"Local OpenRGB override",
		"lf-lighting-source-state\">Enabled",
		"<code class=\"lf-lighting-color-hex\">#000000</code>",
		"<span class=\"lf-lighting-color-rgb\">RGB 0, 0, 0</span>",
		"<span>Stored speed</span><strong>0</strong>",
		"Effective configuration",
		"This reflects stored configuration precedence, not confirmed device output.",
		"href=\"/device/" + visibleSerial + "\"",
		"Open full controls",
	} {
		if !strings.Contains(lightingBody, expected) {
			t.Errorf("Lighting GET /devices response does not contain %q", expected)
		}
	}
	for _, excluded := range []string{
		"class=\"lf-overview-workspace\"",
		"<form",
		"<button",
		"<input",
		"type=\"color\"",
		"type=\"range\"",
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
		"Static &lt;Effect&gt; &amp; More",
	} {
		if strings.Contains(strings.ToLower(lightingBody), strings.ToLower(excluded)) {
			t.Errorf("Lighting GET /devices response unexpectedly contains %q", excluded)
		}
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

	start := rgb.Color{Red: 1, Green: 2, Blue: 3}
	middle := rgb.Color{Red: 4, Green: 5, Blue: 6}
	end := rgb.Color{Red: 7, Green: 8, Blue: 9}
	paletteTests := []struct {
		name         string
		paletteLabel string
		definition   openrgbimport.LightingDefinitionSnapshot
		hasStart     bool
		hasMiddle    bool
		hasEnd       bool
		hasSpeed     bool
	}{
		{
			name:         "Static",
			paletteLabel: "Single color",
			definition:   openrgbimport.LightingDefinitionSnapshot{Palette: rgb.LightingPaletteStaticSingle, HasStartColor: true, StartColor: start},
			hasStart:     true,
		},
		{
			name:         "Two color",
			paletteLabel: "Two color",
			definition:   openrgbimport.LightingDefinitionSnapshot{Palette: rgb.LightingPaletteTwoColor, HasStartColor: true, StartColor: start, HasEndColor: true, EndColor: end, HasSpeed: true, Speed: 0},
			hasStart:     true,
			hasEnd:       true,
			hasSpeed:     true,
		},
		{
			name:         "Temperature",
			paletteLabel: "Temperature",
			definition:   openrgbimport.LightingDefinitionSnapshot{Palette: rgb.LightingPaletteTemperatureThree, HasStartColor: true, StartColor: start, HasMiddleColor: true, MiddleColor: middle, HasEndColor: true, EndColor: end},
			hasStart:     true,
			hasMiddle:    true,
			hasEnd:       true,
		},
		{
			name:         "Gradient",
			paletteLabel: "Gradient",
			definition:   openrgbimport.LightingDefinitionSnapshot{Palette: rgb.LightingPaletteGradient, HasSpeed: true, Speed: 3},
			hasSpeed:     true,
		},
		{
			name:         "Generated palette",
			paletteLabel: "Generated palette",
			definition:   openrgbimport.LightingDefinitionSnapshot{Palette: rgb.LightingPaletteGenerated, HasSpeed: true, Speed: 4},
			hasSpeed:     true,
		},
		{
			name:         "Off",
			paletteLabel: "None",
			definition:   openrgbimport.LightingDefinitionSnapshot{Palette: rgb.LightingPaletteNone},
		},
	}
	for _, test := range paletteTests {
		t.Run("Lighting palette "+test.name, func(t *testing.T) {
			definition := test.definition
			lighting := openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
				HasActiveProfile: true,
				ConfiguredEffect: strings.ToLower(strings.ReplaceAll(test.name, " ", "-")),
				EffectSupported:  true,
				BaseDefinition:   &definition,
				Effective:        &definition,
			})
			body := renderDevicesLightingView(t, lighting)
			for markup, want := range map[string]bool{
				"lf-lighting-palette-stop-name\">Start</span>":  test.hasStart,
				"lf-lighting-palette-stop-name\">Middle</span>": test.hasMiddle,
				"lf-lighting-palette-stop-name\">End</span>":    test.hasEnd,
				"<span>Speed</span>":                            test.hasSpeed,
			} {
				if found := strings.Contains(body, markup); found != want {
					t.Errorf("%s response presence of %q = %t, want %t", test.name, markup, found, want)
				}
			}
			if !strings.Contains(body, "class=\"lf-lighting-definition-kind\">"+test.paletteLabel+"</span>") {
				t.Errorf("%s response does not render its palette label", test.name)
			}
		})
	}

	unsupportedSnapshot := openrgbimport.LightingSnapshot{
		HasActiveProfile: true,
		ConfiguredEffect: "legacy<script>",
		EffectSupported:  false,
		SupportedEffects: []openrgbimport.LightingEffectOption{
			{ID: "static", Label: "Static", CapabilityKnown: true, Capability: rgb.LightingEffectCapability{Palette: rgb.LightingPaletteStaticSingle}},
		},
	}
	unsupportedBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(unsupportedSnapshot))
	for _, expected := range []string{"legacy&lt;script&gt;", "lf-lighting-status-unsupported\">Unsupported", "Stored definition unavailable"} {
		if !strings.Contains(unsupportedBody, expected) {
			t.Errorf("unsupported Lighting response does not contain %q", expected)
		}
	}
	if strings.Contains(unsupportedBody, "legacy<script>") || strings.Contains(unsupportedBody, "lf-lighting-palette-stop-name\">Start</span>") {
		t.Error("unsupported Lighting response rendered raw input or fabricated colors")
	}

	missingDefinitionBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		HasActiveProfile: true,
		ConfiguredEffect: "static",
		EffectSupported:  true,
		SupportedEffects: []openrgbimport.LightingEffectOption{{
			ID:              "static",
			Label:           "Static",
			CapabilityKnown: true,
			Capability:      rgb.LightingEffectCapability{Palette: rgb.LightingPaletteStaticSingle, UsesStartColor: true},
		}},
	}))
	for _, expected := range []string{"lf-lighting-status-supported\">Supported", "Single color", "Stored definition unavailable"} {
		if !strings.Contains(missingDefinitionBody, expected) {
			t.Errorf("supported effect with missing definition does not contain %q", expected)
		}
	}
	if strings.Contains(missingDefinitionBody, "lf-lighting-palette-stop-name\">Start</span>") || strings.Contains(missingDefinitionBody, "#000000") {
		t.Error("supported effect with missing definition rendered fabricated colors")
	}

	emptyEffectBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{HasActiveProfile: true}))
	if !strings.Contains(emptyEffectBody, "Not configured") || !strings.Contains(emptyEffectBody, "<small>Brightness</small>") ||
		!strings.Contains(emptyEffectBody, "Unavailable") || strings.Contains(emptyEffectBody, "lf-lighting-status-unsupported\">Unsupported") {
		t.Error("empty configured effect was not rendered as a neutral state")
	}

	for _, test := range []struct {
		name     string
		override *openrgbimport.LightingOverrideSnapshot
		expected string
	}{
		{name: "nil", expected: "No stored override"},
		{name: "disabled", override: &openrgbimport.LightingOverrideSnapshot{StartColor: rgb.Color{}, Speed: 0}, expected: ">Disabled</span>"},
		{name: "enabled", override: &openrgbimport.LightingOverrideSnapshot{Enabled: true, StartColor: rgb.Color{}, Speed: 0}, expected: ">Enabled</span>"},
	} {
		t.Run("Lighting override "+test.name, func(t *testing.T) {
			body := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{Override: test.override}))
			if !strings.Contains(body, test.expected) {
				t.Errorf("%s override response does not contain %q", test.name, test.expected)
			}
			if test.override != nil {
				for _, expected := range []string{"#000000", "RGB 0, 0, 0", "<span>Stored speed</span><strong>0</strong>"} {
					if !strings.Contains(body, expected) {
						t.Errorf("%s override response does not preserve %q", test.name, expected)
					}
					if test.name == "disabled" && !strings.Contains(body, "lf-lighting-source-card lf-lighting-source-card-inactive") {
						t.Error("disabled override values are not visually marked inactive")
					}
				}
			}
		})
	}
}
