package server

import (
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/devices"
	"LumenForge/src/devices/openrgbimport"
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

func TestDevicesPageRouteRendersVisibleDevices(t *testing.T) {
	if os.Getenv(devicesPageHelperEnvironment) == "1" {
		runDevicesPageRouteAssertions(t)
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestDevicesPageRouteRendersVisibleDevices$")
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

func runDevicesPageRouteAssertions(t *testing.T) {
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
	} {
		if !strings.Contains(emptyBody, expected) {
			t.Errorf("empty GET /devices response does not contain %q", expected)
		}
	}
	for _, excluded := range []string{"Select a device", "class=\"lf-selected-device-summary\""} {
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
		"class=\"lf-selected-device-summary\"",
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
	if strings.Contains(unrelatedBody, "class=\"lf-selected-device-summary\"") {
		t.Error("unrelated-only query unexpectedly selects a device")
	}

	selectedRecorder := requestDevicesPage(t, router, "device="+url.QueryEscape(visibleSerial))
	if selectedRecorder.Code != http.StatusOK {
		t.Fatalf("selected GET /devices status = %d: %s", selectedRecorder.Code, selectedRecorder.Body.String())
	}
	selectedBody := selectedRecorder.Body.String()
	for _, expected := range []string{
		"class=\"lf-selected-device-summary\"",
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

	selectedWithUnrelatedRecorder := requestDevicesPage(t, router, "device="+url.QueryEscape(visibleSerial)+"&foo=bar")
	if selectedWithUnrelatedRecorder.Code != http.StatusOK {
		t.Fatalf("selected GET /devices with unrelated query status = %d: %s", selectedWithUnrelatedRecorder.Code, selectedWithUnrelatedRecorder.Body.String())
	}
	selectedWithUnrelatedBody := selectedWithUnrelatedRecorder.Body.String()
	for _, expected := range []string{
		"class=\"lf-selected-device-summary\"",
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
	if !strings.Contains(fallbackRecorder.Body.String(), "class=\"lf-selected-device-image lf-selected-device-image-fallback\" src=\"/static/img/icons/icon-device.svg\"") {
		t.Error("selected device without an image does not render the generic fallback")
	}
	if strings.Contains(fallbackRecorder.Body.String(), "class=\"lf-openrgb-overview\"") {
		t.Error("generic selected device unexpectedly renders the OpenRGB overview")
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

	hiddenRecorder := requestDevicesPage(t, router, "device="+hiddenSerial)
	if hiddenRecorder.Code != http.StatusNotFound {
		t.Errorf("hidden device status = %d, want %d", hiddenRecorder.Code, http.StatusNotFound)
	}
	for _, excluded := range []string{hiddenSerial, "Hidden Test Device"} {
		if strings.Contains(hiddenRecorder.Body.String(), excluded) {
			t.Errorf("hidden-device response unexpectedly contains %q", excluded)
		}
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
	if !strings.Contains(nonOpenRGBBody, "class=\"lf-selected-device-summary\"") ||
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
		Product: escapedProduct,
		Serial:  escapedSerial,
		OpenRGB: escapedOpenRGB,
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
