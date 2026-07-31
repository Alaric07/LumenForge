package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices/openrgbimport"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const lightingMutationTestSerial = "openrgb-lighting-test"

type lightingMutationCalls struct {
	brightness []uint8
	effects    []string
	setError   error
}

func installLightingMutationTestSeams(t *testing.T) (*openrgbimport.Device, *common.Device, *lightingMutationCalls) {
	t.Helper()

	previousLookup := lookupOpenRGBImportForLighting
	previousBrightness := setOpenRGBImportBrightnessValue
	previousEffect := setOpenRGBImportEffectValue
	t.Cleanup(func() {
		lookupOpenRGBImportForLighting = previousLookup
		setOpenRGBImportBrightnessValue = previousBrightness
		setOpenRGBImportEffectValue = previousEffect
	})

	device := &openrgbimport.Device{
		Serial:    lightingMutationTestSerial,
		IsOpenRGB: true,
		RGBModes:  []string{"static", "off", "rainbow"},
	}
	wrapper := &common.Device{Serial: lightingMutationTestSerial, Instance: device}
	calls := &lightingMutationCalls{}
	lookupOpenRGBImportForLighting = func(serial string) (*common.Device, *openrgbimport.Device, bool) {
		if serial != lightingMutationTestSerial {
			return nil, nil, false
		}
		return wrapper, device, true
	}
	setOpenRGBImportBrightnessValue = func(_ *openrgbimport.Device, brightness uint8) error {
		calls.brightness = append(calls.brightness, brightness)
		return calls.setError
	}
	setOpenRGBImportEffectValue = func(_ *openrgbimport.Device, effect string) error {
		calls.effects = append(calls.effects, effect)
		return calls.setError
	}
	return device, wrapper, calls
}

func requestOpenRGBLightingMutation(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	addLocalRequestProtection(t, router, request)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func requireLightingMutationResponse(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int) *Response {
	t.Helper()
	response := decodeLifecycleResponse(t, recorder)
	if recorder.Code != http.StatusOK || response.Code != http.StatusOK || response.Status != wantStatus || response.Message == "" {
		t.Fatalf("response = %#v, HTTP %d, want application status %d", response, recorder.Code, wantStatus)
	}
	return response
}

func TestOpenRGBLightingMutationRequestValidation(t *testing.T) {
	_, _, calls := installLightingMutationTestSeams(t)
	router := setRoutes()

	brightnessCases := []struct {
		name      string
		body      string
		wantValue *uint8
	}{
		{name: "zero", body: `{"serial":"openrgb-lighting-test","brightness":0}`, wantValue: uint8Pointer(0)},
		{name: "maximum", body: `{"serial":"openrgb-lighting-test","brightness":100}`, wantValue: uint8Pointer(100)},
		{name: "intermediate", body: `{"serial":"openrgb-lighting-test","brightness":47}`, wantValue: uint8Pointer(47)},
		{name: "negative", body: `{"serial":"openrgb-lighting-test","brightness":-1}`},
		{name: "above maximum", body: `{"serial":"openrgb-lighting-test","brightness":101}`},
		{name: "overflow", body: `{"serial":"openrgb-lighting-test","brightness":999999999999999999999999}`},
		{name: "fraction", body: `{"serial":"openrgb-lighting-test","brightness":1.5}`},
		{name: "string", body: `{"serial":"openrgb-lighting-test","brightness":"50"}`},
		{name: "null", body: `{"serial":"openrgb-lighting-test","brightness":null}`},
		{name: "wrong serial type", body: `{"serial":1,"brightness":50}`},
		{name: "omitted brightness", body: `{"serial":"openrgb-lighting-test"}`},
		{name: "omitted serial", body: `{"brightness":50}`},
		{name: "empty serial", body: `{"serial":"","brightness":50}`},
		{name: "unknown field", body: `{"serial":"openrgb-lighting-test","brightness":50,"extra":true}`},
		{name: "trailing JSON", body: `{"serial":"openrgb-lighting-test","brightness":50}{}`},
		{name: "malformed", body: `{"serial":"openrgb-lighting-test","brightness":`},
		{name: "oversized", body: strings.Repeat(" ", openRGBImportRequestLimit+1) + `{"serial":"openrgb-lighting-test","brightness":50}`},
	}
	for _, test := range brightnessCases {
		t.Run("brightness "+test.name, func(t *testing.T) {
			calls.brightness = nil
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/brightness", test.body)
			if test.wantValue == nil {
				requireLightingMutationResponse(t, recorder, 0)
				if len(calls.brightness) != 0 {
					t.Fatalf("brightness mutation calls = %v, want none", calls.brightness)
				}
				return
			}
			response := requireLightingMutationResponse(t, recorder, 1)
			if response.Message != "Brightness set" {
				t.Fatalf("success message = %q, want %q", response.Message, "Brightness set")
			}
			if len(calls.brightness) != 1 || calls.brightness[0] != *test.wantValue {
				t.Fatalf("brightness mutation calls = %v, want [%d]", calls.brightness, *test.wantValue)
			}
		})
	}

	effectCases := []struct {
		name      string
		body      string
		wantValue string
	}{
		{name: "supported", body: `{"serial":"openrgb-lighting-test","effect":"static"}`, wantValue: "static"},
		{name: "empty", body: `{"serial":"openrgb-lighting-test","effect":""}`},
		{name: "unsupported", body: `{"serial":"openrgb-lighting-test","effect":"STATIC"}`},
		{name: "omitted effect", body: `{"serial":"openrgb-lighting-test"}`},
		{name: "omitted serial", body: `{"effect":"static"}`},
		{name: "wrong type", body: `{"serial":"openrgb-lighting-test","effect":1}`},
		{name: "null", body: `{"serial":"openrgb-lighting-test","effect":null}`},
		{name: "wrong serial type", body: `{"serial":1,"effect":"static"}`},
		{name: "unknown field", body: `{"serial":"openrgb-lighting-test","effect":"static","extra":true}`},
		{name: "trailing JSON", body: `{"serial":"openrgb-lighting-test","effect":"static"}{}`},
		{name: "malformed", body: `{"serial":"openrgb-lighting-test","effect":`},
		{name: "oversized", body: strings.Repeat(" ", openRGBImportRequestLimit+1) + `{"serial":"openrgb-lighting-test","effect":"static"}`},
	}
	for _, test := range effectCases {
		t.Run("effect "+test.name, func(t *testing.T) {
			calls.effects = nil
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/effect", test.body)
			if test.wantValue == "" {
				requireLightingMutationResponse(t, recorder, 0)
				if len(calls.effects) != 0 {
					t.Fatalf("effect mutation calls = %v, want none", calls.effects)
				}
				return
			}
			response := requireLightingMutationResponse(t, recorder, 1)
			if response.Message != "Effect set" {
				t.Fatalf("success message = %q, want %q", response.Message, "Effect set")
			}
			if len(calls.effects) != 1 || calls.effects[0] != test.wantValue {
				t.Fatalf("effect mutation calls = %v, want [%q]", calls.effects, test.wantValue)
			}
		})
	}

	calls.setError = errors.New("private mutation failure at /tmp/internal")
	for _, test := range []struct {
		path string
		body string
	}{
		{path: "/api/openrgbimport/brightness", body: `{"serial":"openrgb-lighting-test","brightness":50}`},
		{path: "/api/openrgbimport/effect", body: `{"serial":"openrgb-lighting-test","effect":"static"}`},
	} {
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, test.path, test.body)
		response := requireLightingMutationResponse(t, recorder, 0)
		if strings.Contains(response.Message, "private") || strings.Contains(response.Message, "/tmp") {
			t.Fatalf("response exposed internal error: %#v", response)
		}
	}
}

func uint8Pointer(value uint8) *uint8 {
	return &value
}

func TestOpenRGBLightingMutationTargetValidation(t *testing.T) {
	validDevice, validWrapper, calls := installLightingMutationTestSeams(t)
	router := setRoutes()

	tests := []struct {
		name   string
		serial string
		lookup func(string) (*common.Device, *openrgbimport.Device, bool)
	}{
		{name: "missing device", serial: "openrgb-missing", lookup: func(string) (*common.Device, *openrgbimport.Device, bool) { return nil, nil, false }},
		{name: "nil wrapper", serial: lightingMutationTestSerial, lookup: func(string) (*common.Device, *openrgbimport.Device, bool) { return nil, validDevice, true }},
		{name: "hidden wrapper", serial: lightingMutationTestSerial, lookup: func(string) (*common.Device, *openrgbimport.Device, bool) {
			copy := *validWrapper
			copy.Hidden = true
			return &copy, validDevice, true
		}},
		{name: "unavailable wrapper", serial: lightingMutationTestSerial, lookup: func(string) (*common.Device, *openrgbimport.Device, bool) {
			copy := *validWrapper
			copy.Unavailable = true
			return &copy, validDevice, true
		}},
		{name: "mismatched wrapper serial", serial: lightingMutationTestSerial, lookup: func(string) (*common.Device, *openrgbimport.Device, bool) {
			copy := *validWrapper
			copy.Serial = "openrgb-other"
			return &copy, validDevice, true
		}},
		{name: "nil importer", serial: lightingMutationTestSerial, lookup: func(string) (*common.Device, *openrgbimport.Device, bool) { return validWrapper, nil, true }},
		{name: "non importer wrapper instance", serial: lightingMutationTestSerial, lookup: func(string) (*common.Device, *openrgbimport.Device, bool) {
			copy := *validWrapper
			copy.Instance = struct{}{}
			return &copy, validDevice, true
		}},
		{name: "non OpenRGB state", serial: lightingMutationTestSerial, lookup: func(string) (*common.Device, *openrgbimport.Device, bool) {
			device := &openrgbimport.Device{Serial: lightingMutationTestSerial}
			copy := *validWrapper
			copy.Instance = device
			return &copy, device, true
		}},
		{name: "mismatched importer serial", serial: lightingMutationTestSerial, lookup: func(string) (*common.Device, *openrgbimport.Device, bool) {
			device := &openrgbimport.Device{Serial: "openrgb-other", IsOpenRGB: true}
			copy := *validWrapper
			copy.Instance = device
			return &copy, device, true
		}},
		{name: "stale importer identity", serial: lightingMutationTestSerial, lookup: func(string) (*common.Device, *openrgbimport.Device, bool) {
			device := &openrgbimport.Device{Serial: lightingMutationTestSerial, IsOpenRGB: true}
			return validWrapper, device, true
		}},
		{name: "invalid serial", serial: "invalid/serial", lookup: func(string) (*common.Device, *openrgbimport.Device, bool) { return validWrapper, validDevice, true }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lookupOpenRGBImportForLighting = test.lookup
			calls.brightness = nil
			body := `{"serial":"` + test.serial + `","brightness":50}`
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/brightness", body)
			requireLightingMutationResponse(t, recorder, 0)
			if len(calls.brightness) != 0 {
				t.Fatalf("brightness mutation calls = %v, want none", calls.brightness)
			}
			if strings.Contains(recorder.Body.String(), test.serial) {
				t.Fatalf("response reflected rejected serial %q: %s", test.serial, recorder.Body.String())
			}
		})
	}
}

func TestOpenRGBLightingMutationSecurityCompatibility(t *testing.T) {
	installLightingMutationTestSeams(t)
	router := setRoutes()
	routes := []struct {
		name string
		path string
		body string
	}{
		{name: "brightness", path: "/api/openrgbimport/brightness", body: `{"serial":"openrgb-lighting-test","brightness":50}`},
		{name: "effect", path: "/api/openrgbimport/effect", body: `{"serial":"openrgb-lighting-test","effect":"static"}`},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			getRequest := httptest.NewRequest(http.MethodGet, route.path, nil)
			getRequest.Host = "127.0.0.1"
			getRecorder := httptest.NewRecorder()
			router.ServeHTTP(getRecorder, getRequest)
			if getRecorder.Code != http.StatusMethodNotAllowed {
				t.Fatalf("GET status = %d, want %d", getRecorder.Code, http.StatusMethodNotAllowed)
			}

			securityCases := []struct {
				name       string
				prepare    func(*http.Request)
				wantStatus int
			}{
				{name: "wrong content type", prepare: func(request *http.Request) {
					addLocalRequestProtection(t, router, request)
					request.Header.Set("Content-Type", "text/plain")
				}, wantStatus: http.StatusUnsupportedMediaType},
				{name: "missing proof", prepare: func(request *http.Request) { request.Header.Set("Content-Type", "application/json") }, wantStatus: http.StatusForbidden},
				{name: "incorrect proof", prepare: func(request *http.Request) {
					addLocalRequestProtection(t, router, request)
					request.Header.Set(requestProofHeader, "incorrect")
				}, wantStatus: http.StatusForbidden},
				{name: "cross origin", prepare: func(request *http.Request) {
					addLocalRequestProtection(t, router, request)
					request.Header.Set("Origin", "https://attacker.example")
				}, wantStatus: http.StatusForbidden},
			}
			for _, test := range securityCases {
				t.Run(test.name, func(t *testing.T) {
					request := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body))
					request.Host = "127.0.0.1"
					test.prepare(request)
					recorder := httptest.NewRecorder()
					router.ServeHTTP(recorder, request)
					if recorder.Code != test.wantStatus {
						t.Fatalf("status = %d, want %d: %s", recorder.Code, test.wantStatus, recorder.Body.String())
					}
				})
			}
		})
	}
}
