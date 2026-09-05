package server

import (
	"LumenForge/src/config"
	"LumenForge/src/devices"
	"LumenForge/src/devices/cpro"
	"LumenForge/src/server/requests"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"LumenForge/src/templates"
)

func initializeLegacyDevicePreviewTestProcess(t *testing.T) {
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

func legacyDevicePreviewRouter(t *testing.T, enabled bool) http.Handler {
	t.Helper()
	initializeLegacyDevicePreviewTestProcess(t)
	original := legacyDevicePreviewEnabled
	legacyDevicePreviewEnabled = func() bool { return enabled }
	t.Cleanup(func() { legacyDevicePreviewEnabled = original })
	return setRoutes()
}

func legacyDevicePreviewRequest(method, target string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.Host = "localhost"
	return request
}

func TestLegacyDevicePreviewDebugGating(t *testing.T) {
	router := legacyDevicePreviewRouter(t, false)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview"))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("debug-disabled picker status = %d, want 404", recorder.Code)
	}
}

func TestLegacyDevicePreviewPickerUsesRegistry(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("picker status = %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "Modern Device Previews") || !strings.Contains(body, "Legacy Device Previews") {
		t.Errorf("picker did not distinguish modern and legacy previews: %s", body)
	}
	for _, fixture := range modernDevicePreviewFixtures {
		if !strings.Contains(body, `href="/dev/device-preview/`+fixture.Key+`"`) || !strings.Contains(body, `>`+fixture.Title+`</span>`) {
			t.Errorf("picker omitted modern fixture %q: %s", fixture.Key, body)
		}
	}
	for _, fixture := range legacyDevicePreviewFixtures {
		if !strings.Contains(body, `value="`+fixture.Key+`"`) || !strings.Contains(body, `>`+fixture.Title+`</option>`) {
			t.Errorf("picker omitted registry fixture %q: %s", fixture.Key, body)
		}
	}
	csp := recorder.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"script-src 'none'", "connect-src 'none'", "form-action 'self'"} {
		if !strings.Contains(csp, directive) {
			t.Errorf("picker CSP %q omitted %q", csp, directive)
		}
	}
}

func TestLegacyDevicePreviewRendersRealTemplatesWithCSP(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	for _, test := range []struct {
		fixture string
		want    string
		labels  []string
		noNil   bool
	}{
		{fixture: "commander-pro", want: "Commander Pro", noNil: true},
		{fixture: "k70-pro", want: "K70 RGB PRO", labels: []string{"Esc", "F12", "Q", "A", "Z", "Space"}},
		{fixture: "virtuoso-wireless", want: "VIRTUOSO Wireless", noNil: true},
	} {
		t.Run(test.fixture, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+test.fixture))
			if recorder.Code != http.StatusOK {
				t.Fatalf("preview status = %d: %s", recorder.Code, recorder.Body.String())
			}
			body := recorder.Body.String()
			if !strings.Contains(body, test.want) || !strings.Contains(body, "Preview Mode — Hardware actions and scripts are disabled") {
				t.Fatalf("preview did not render expected real template content: %s", body)
			}
			if test.noNil && (strings.Contains(body, "&lt;nil&gt;") || strings.Contains(body, "<nil>")) {
				t.Fatalf("preview rendered a nil presentation value: %s", body)
			}
			for _, label := range test.labels {
				if !strings.Contains(body, ">"+label+"</span>") {
					t.Errorf("preview omitted representative keyboard key %q", label)
				}
			}
			if !strings.Contains(body, `/static/css/themes/default.css`) {
				t.Errorf("preview omitted normal theme stylesheet: %s", body)
			}
			csp := recorder.Header().Get("Content-Security-Policy")
			for _, directive := range []string{"script-src 'none'", "connect-src 'none'", "form-action 'none'", "base-uri 'none'"} {
				if !strings.Contains(csp, directive) {
					t.Errorf("preview CSP %q omitted %q", csp, directive)
				}
			}
		})
	}
}

func TestLegacyDevicePreviewRejectsUnknownAndMalformedPaths(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	for _, path := range []string{
		"/dev/device-preview/not-a-device",
		"/dev/device-preview/commander-pro/extra",
		"/dev/device-preview/",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, legacyDevicePreviewRequest(http.MethodGet, path))
		if recorder.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", path, recorder.Code)
		}
	}
}

func TestLegacyDevicePreviewDoesNotRegisterFixtureDevices(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	fixture := legacyDevicePreviewFixtures[0]
	device := fixture.Build()
	serial := device.(*cpro.Device).Serial
	if devices.GetDevice(serial) != nil {
		t.Fatalf("fixture serial %q unexpectedly exists before preview", serial)
	}

	previewRecorder := httptest.NewRecorder()
	router.ServeHTTP(previewRecorder, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+fixture.Key))
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("preview status = %d", previewRecorder.Code)
	}
	if devices.GetDevice(serial) != nil {
		t.Fatalf("fixture serial %q was registered by preview rendering", serial)
	}

}

func TestLegacyDevicePreviewFixtureSerialCannotDispatchMutation(t *testing.T) {
	fixture := legacyDevicePreviewFixtures[0]
	serial := fixture.Build().(*cpro.Device).Serial
	request := httptest.NewRequest(http.MethodPost, "/api/color", strings.NewReader(`{"deviceId":"`+serial+`","channelId":0,"profile":"rainbow"}`))
	response := requests.ProcessChangeColor(request)
	if response.Status != 0 {
		t.Fatalf("fixture serial mutation response = %#v, want failed dispatch", response)
	}
	if devices.GetDevice(serial) != nil {
		t.Fatalf("fixture serial %q became dispatchable", serial)
	}
}
