package server

import (
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/devices"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/templates"
	"net/http"
	"net/http/httptest"
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

func runDevicesPageRouteAssertions(t *testing.T) {
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

	visibleSerial := "lf-devices-page-visible"
	visibleInstance := &openrgbimport.Device{Serial: visibleSerial, Product: "Visible Test Device"}
	visible := &common.Device{
		Product:  "Visible Test Device",
		Serial:   visibleSerial,
		Firmware: "test-firmware",
		Image:    "icon-device.svg",
		Instance: visibleInstance,
	}
	if err := devices.RegisterOpenRGBImport(visible, visibleInstance); err != nil {
		t.Fatal(err)
	}
	visibleRegistered := true
	t.Cleanup(func() {
		if visibleRegistered {
			devices.RemoveOpenRGBImport(visibleSerial, visibleInstance)
		}
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

	request := httptest.NewRequest(http.MethodGet, "/devices", nil)
	request.Host = "127.0.0.1:27003"
	recorder := httptest.NewRecorder()
	setRoutes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /devices status = %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		"Devices workspace",
		"href=\"/device/" + visibleSerial + "\"",
		"Visible Test Device",
		"href=\"/static/css/app-shell.css\"",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("GET /devices response does not contain %q", expected)
		}
	}
	for _, excluded := range []string{hiddenSerial, "Hidden Test Device", "Battery 0%"} {
		if strings.Contains(body, excluded) {
			t.Errorf("GET /devices response unexpectedly contains %q", excluded)
		}
	}

	if _, ok := devices.RemoveOpenRGBImport(visibleSerial, visibleInstance); !ok {
		t.Fatal("unable to remove visible test device before empty-state render")
	}
	visibleRegistered = false
	emptyRequest := httptest.NewRequest(http.MethodGet, "/devices", nil)
	emptyRequest.Host = "127.0.0.1:27003"
	emptyRecorder := httptest.NewRecorder()
	setRoutes().ServeHTTP(emptyRecorder, emptyRequest)
	if emptyRecorder.Code != http.StatusOK {
		t.Fatalf("empty GET /devices status = %d: %s", emptyRecorder.Code, emptyRecorder.Body.String())
	}
	emptyBody := emptyRecorder.Body.String()
	if !strings.Contains(emptyBody, "No devices available") {
		t.Error("empty GET /devices response does not contain the empty-device state")
	}
	if strings.Contains(emptyBody, "Select a device") {
		t.Error("empty GET /devices response instructs the user to select a device")
	}
}
