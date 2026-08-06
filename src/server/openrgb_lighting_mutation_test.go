package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"context"
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
	speeds     []lightingSpeedMutationCall
	colors     []lightingColorMutationCall
	twoColors  []lightingTwoColorMutationCall
	resets     []lightingResetMutationCall
	setError   error
}

type lightingSpeedMutationCall struct {
	serial string
	effect string
	speed  float64
}

type lightingColorMutationCall struct {
	serial string
	effect string
	color  lightingsettings.Color
}

type lightingTwoColorMutationCall struct {
	serial string
	effect string
	start  lightingsettings.Color
	end    lightingsettings.Color
}

type lightingResetMutationCall struct {
	serial string
	effect string
}

func installLightingMutationTestSeams(t *testing.T) (*openrgbimport.Device, *common.Device, *lightingMutationCalls) {
	t.Helper()

	previousLookup := lookupOpenRGBImportForLighting
	previousBrightness := setOpenRGBImportBrightnessValue
	previousEffect := setOpenRGBImportEffectValue
	previousSpeed := setOpenRGBImportSpeedValue
	previousColor := setOpenRGBImportColorValue
	previousTwoColor := setOpenRGBImportTwoColorValue
	previousReset := resetOpenRGBImportCustomizationValue
	t.Cleanup(func() {
		lookupOpenRGBImportForLighting = previousLookup
		setOpenRGBImportBrightnessValue = previousBrightness
		setOpenRGBImportEffectValue = previousEffect
		setOpenRGBImportSpeedValue = previousSpeed
		setOpenRGBImportColorValue = previousColor
		setOpenRGBImportTwoColorValue = previousTwoColor
		resetOpenRGBImportCustomizationValue = previousReset
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
	setOpenRGBImportSpeedValue = func(_ *openrgbimport.Device, serial, effect string, speed float64) error {
		calls.speeds = append(calls.speeds, lightingSpeedMutationCall{serial: serial, effect: effect, speed: speed})
		return calls.setError
	}
	setOpenRGBImportColorValue = func(_ *openrgbimport.Device, serial, effect string, color lightingsettings.Color) error {
		calls.colors = append(calls.colors, lightingColorMutationCall{serial: serial, effect: effect, color: color})
		return calls.setError
	}
	setOpenRGBImportTwoColorValue = func(_ *openrgbimport.Device, serial, effect string, start, end lightingsettings.Color) error {
		calls.twoColors = append(calls.twoColors, lightingTwoColorMutationCall{serial: serial, effect: effect, start: start, end: end})
		return calls.setError
	}
	resetOpenRGBImportCustomizationValue = func(_ *openrgbimport.Device, serial, effect string) error {
		calls.resets = append(calls.resets, lightingResetMutationCall{serial: serial, effect: effect})
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

	speedCases := []struct {
		name      string
		body      string
		wantValue *float64
	}{
		{name: "minimum", body: `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":1}`, wantValue: float64Pointer(1)},
		{name: "maximum", body: `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":10}`, wantValue: float64Pointer(10)},
		{name: "fraction", body: `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":4.25}`, wantValue: float64Pointer(4.25)},
		{name: "below minimum", body: `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":0.99}`},
		{name: "above maximum", body: `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":10.01}`},
		{name: "quoted number", body: `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":"4"}`},
		{name: "null", body: `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":null}`},
		{name: "missing speed", body: `{"serial":"openrgb-lighting-test","effect":"rainbow"}`},
		{name: "missing effect", body: `{"serial":"openrgb-lighting-test","speed":4}`},
		{name: "empty effect", body: `{"serial":"openrgb-lighting-test","effect":"","speed":4}`},
		{name: "missing serial", body: `{"effect":"rainbow","speed":4}`},
		{name: "empty serial", body: `{"serial":"","effect":"rainbow","speed":4}`},
		{name: "unsupported effect", body: `{"serial":"openrgb-lighting-test","effect":"wave","speed":4}`},
		{name: "no-speed effect", body: `{"serial":"openrgb-lighting-test","effect":"static","speed":4}`},
		{name: "wrong serial type", body: `{"serial":1,"effect":"rainbow","speed":4}`},
		{name: "wrong effect type", body: `{"serial":"openrgb-lighting-test","effect":1,"speed":4}`},
		{name: "unknown field", body: `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":4,"extra":true}`},
		{name: "trailing JSON", body: `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":4}{}`},
		{name: "malformed", body: `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":`},
		{name: "non-finite", body: `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":NaN}`},
		{name: "oversized", body: strings.Repeat(" ", openRGBImportRequestLimit+1) + `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":4}`},
	}
	for _, test := range speedCases {
		t.Run("speed "+test.name, func(t *testing.T) {
			calls.speeds = nil
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/speed", test.body)
			if test.wantValue == nil {
				requireLightingMutationResponse(t, recorder, 0)
				if len(calls.speeds) != 0 {
					t.Fatalf("speed mutation calls = %#v, want none", calls.speeds)
				}
				return
			}
			response := requireLightingMutationResponse(t, recorder, 1)
			if response.Message != "Speed set" {
				t.Fatalf("success message = %q, want %q", response.Message, "Speed set")
			}
			want := lightingSpeedMutationCall{serial: lightingMutationTestSerial, effect: "rainbow", speed: *test.wantValue}
			if len(calls.speeds) != 1 || calls.speeds[0] != want {
				t.Fatalf("speed mutation calls = %#v, want %#v", calls.speeds, want)
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
		{path: "/api/openrgbimport/speed", body: `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":4}`},
	} {
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, test.path, test.body)
		response := requireLightingMutationResponse(t, recorder, 0)
		if strings.Contains(response.Message, "private") || strings.Contains(response.Message, "/tmp") {
			t.Fatalf("response exposed internal error: %#v", response)
		}
	}
}

func TestOpenRGBLegacyStaticColorEndpointIsRetired(t *testing.T) {
	_, _, calls := installLightingMutationTestSeams(t)
	router := setRoutes()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/openrgbimport/color", strings.NewReader(
		`{"serial":"openrgb-lighting-test","r":10,"g":20,"b":30}`,
	))
	addLocalRequestProtection(t, router, request)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("retired color endpoint HTTP status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if len(calls.brightness) != 0 || len(calls.effects) != 0 || len(calls.speeds) != 0 {
		t.Fatalf("retired color endpoint reached lighting mutations: %#v", calls)
	}

	effect := requestOpenRGBLightingMutation(
		t,
		router,
		http.MethodPost,
		"/api/openrgbimport/effect",
		`{"serial":"openrgb-lighting-test","effect":"static"}`,
	)
	requireLightingMutationResponse(t, effect, 1)
	if len(calls.effects) != 1 || calls.effects[0] != "static" {
		t.Fatalf("unrelated effect endpoint calls = %v, want [static]", calls.effects)
	}
}

func TestOpenRGBLightingLegacySpeedLookupErrorRedaction(t *testing.T) {
	previousLookup := lookupOpenRGBImportLegacy
	t.Cleanup(func() { lookupOpenRGBImportLegacy = previousLookup })
	router := setRoutes()

	for _, internalError := range []string{
		"read /var/lib/lumenforge/database/profiles/private.json: permission denied",
		"OpenRGB controller 7 rejected speed change",
		"OpenRGB import lifecycle is detached",
		"device is controlled by RGB cluster",
		"arbitrary private lookup detail",
	} {
		t.Run(internalError, func(t *testing.T) {
			lookupOpenRGBImportLegacy = func(string) (*openrgbimport.Device, error) {
				return nil, errors.New(internalError)
			}
			recorder := requestOpenRGBLightingMutation(
				t,
				router,
				http.MethodPost,
				"/api/openrgbimport/speed",
				`{"serial":"openrgb-lighting-test","speed":"fast"}`,
			)
			response := requireLightingMutationResponse(t, recorder, 0)
			if response.Message != "OpenRGB device is not available" {
				t.Fatalf("legacy speed lookup message = %q", response.Message)
			}
			if strings.Contains(recorder.Body.String(), internalError) {
				t.Fatalf("legacy speed response exposed internal lookup error: %s", recorder.Body.String())
			}
		})
	}

	t.Run("successful categorical request remains compatible", func(t *testing.T) {
		device := &openrgbimport.Device{}
		lookupOpenRGBImportLegacy = func(string) (*openrgbimport.Device, error) {
			return device, nil
		}
		recorder := requestOpenRGBLightingMutation(
			t,
			router,
			http.MethodPost,
			"/api/openrgbimport/speed",
			`{"serial":"openrgb-lighting-test","speed":"slow"}`,
		)
		response := requireLightingMutationResponse(t, recorder, 1)
		if response.Message != "Speed set" || device.GetSpeed() != "slow" {
			t.Fatalf("legacy speed success response = %#v, device speed %q", response, device.GetSpeed())
		}
	})
}

func TestOpenRGBLightingSpeedTimeoutErrorRedaction(t *testing.T) {
	_, _, calls := installLightingMutationTestSeams(t)
	calls.setError = errors.Join(errors.New("wait for initial OpenRGB effect output"), context.DeadlineExceeded)
	recorder := requestOpenRGBLightingMutation(
		t,
		setRoutes(),
		http.MethodPost,
		"/api/openrgbimport/speed",
		`{"serial":"openrgb-lighting-test","effect":"rainbow","speed":4}`,
	)
	response := requireLightingMutationResponse(t, recorder, 0)
	if response.Message != "Unable to set speed" {
		t.Fatalf("speed timeout response message = %q", response.Message)
	}
	if strings.Contains(recorder.Body.String(), "DeadlineExceeded") || strings.Contains(recorder.Body.String(), "initial OpenRGB effect output") {
		t.Fatalf("speed timeout response exposed internal error: %s", recorder.Body.String())
	}
}

func uint8Pointer(value uint8) *uint8 {
	return &value
}

func float64Pointer(value float64) *float64 {
	return &value
}

func TestOpenRGBLightingSpeedClusterOwnershipGuard(t *testing.T) {
	previousLookup := lookupOpenRGBImportForLighting
	previousSpeed := setOpenRGBImportSpeedValue
	t.Cleanup(func() {
		lookupOpenRGBImportForLighting = previousLookup
		setOpenRGBImportSpeedValue = previousSpeed
	})

	profile := &openrgbimport.DeviceProfile{Active: true, RGBProfile: "rainbow", RGBCluster: true}
	device := &openrgbimport.Device{
		Serial:        lightingMutationTestSerial,
		IsOpenRGB:     true,
		RGBModes:      []string{"rainbow"},
		DeviceProfile: profile,
		Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{
			"rainbow": {ProfileName: "rainbow", Speed: 2},
		}},
	}
	wrapper := &common.Device{Serial: lightingMutationTestSerial, Instance: device}
	lookupOpenRGBImportForLighting = func(serial string) (*common.Device, *openrgbimport.Device, bool) {
		return wrapper, device, serial == lightingMutationTestSerial
	}
	calls := 0
	var observedErr error
	setOpenRGBImportSpeedValue = func(device *openrgbimport.Device, serial, effect string, speed float64) error {
		calls++
		observedErr = device.SetEffectSpeed(serial, effect, speed)
		return observedErr
	}

	recorder := requestOpenRGBLightingMutation(t, setRoutes(), http.MethodPost, "/api/openrgbimport/speed", `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":4}`)
	response := requireLightingMutationResponse(t, recorder, 0)
	if calls != 1 {
		t.Fatalf("cluster-owned speed mutation calls = %d, want 1", calls)
	}
	const ownershipError = "device is controlled by RGB cluster"
	if observedErr == nil || observedErr.Error() != ownershipError {
		t.Fatalf("real SetEffectSpeed error = %v, want %q", observedErr, ownershipError)
	}
	if response.Message != "Unable to set speed" || strings.Contains(recorder.Body.String(), ownershipError) {
		t.Fatalf("cluster-owned speed response = %s", recorder.Body.String())
	}
	if device.DeviceProfile != profile || profile.RGBProfile != "rainbow" || device.Rgb.Profiles["rainbow"].Speed != 2 {
		t.Fatalf("cluster rejection changed fixture state: profile=%#v RGB=%#v", profile, device.Rgb)
	}
}

func TestOpenRGBLightingBrightnessMutationErrorRedaction(t *testing.T) {
	_, _, calls := installLightingMutationTestSeams(t)
	calls.setError = errors.New("private brightness mutation failure at /tmp/internal-profile")
	router := setRoutes()

	recorder := requestOpenRGBLightingMutation(
		t,
		router,
		http.MethodPost,
		"/api/openrgbimport/brightness",
		`{"serial":"openrgb-lighting-test","brightness":50}`,
	)
	response := requireLightingMutationResponse(t, recorder, 0)
	if response.Message != "Unable to set brightness" {
		t.Fatalf("brightness error response message = %q, want generic brightness failure", response.Message)
	}
	if strings.Contains(recorder.Body.String(), "private") || strings.Contains(recorder.Body.String(), "/tmp") {
		t.Fatalf("brightness error response exposed internal error: %s", recorder.Body.String())
	}
	if len(calls.brightness) != 1 || calls.brightness[0] != 50 {
		t.Fatalf("failed brightness mutation calls = %v, want [50]", calls.brightness)
	}
}

func TestOpenRGBLightingBrightnessClusterOwnershipGuard(t *testing.T) {
	previousLookup := lookupOpenRGBImportForLighting
	previousBrightness := setOpenRGBImportBrightnessValue
	t.Cleanup(func() {
		lookupOpenRGBImportForLighting = previousLookup
		setOpenRGBImportBrightnessValue = previousBrightness
	})

	initialBrightness := uint8(0)
	profile := &openrgbimport.DeviceProfile{
		Active:           true,
		Serial:           lightingMutationTestSerial,
		RGBProfile:       "static",
		BrightnessSlider: &initialBrightness,
		RGBCluster:       true,
	}
	device := &openrgbimport.Device{
		Serial:        lightingMutationTestSerial,
		IsOpenRGB:     true,
		RGBModes:      []string{"static", "off", "rainbow"},
		DeviceProfile: profile,
	}
	wrapper := &common.Device{Serial: lightingMutationTestSerial, Instance: device}
	lookupOpenRGBImportForLighting = func(serial string) (*common.Device, *openrgbimport.Device, bool) {
		if serial != lightingMutationTestSerial {
			return nil, nil, false
		}
		return wrapper, device, true
	}

	initialDeviceBrightness := device.GetBrightness()
	initialProfileBrightness := *profile.BrightnessSlider
	const requestedBrightness = uint8(50)
	if requestedBrightness == initialDeviceBrightness {
		t.Fatal("cluster-ownership test requires a changed brightness value")
	}

	calls := 0
	var observedErr error
	setOpenRGBImportBrightnessValue = func(device *openrgbimport.Device, brightness uint8) error {
		calls++
		observedErr = device.SetBrightness(brightness)
		return observedErr
	}

	router := setRoutes()
	recorder := requestOpenRGBLightingMutation(
		t,
		router,
		http.MethodPost,
		"/api/openrgbimport/brightness",
		`{"serial":"openrgb-lighting-test","brightness":50}`,
	)
	response := requireLightingMutationResponse(t, recorder, 0)

	if calls != 1 {
		t.Fatalf("cluster-owned brightness mutation calls = %d, want 1", calls)
	}
	const ownershipError = "device is controlled by RGB cluster"
	if observedErr == nil || observedErr.Error() != ownershipError {
		t.Fatalf("real SetBrightness error = %v, want %q", observedErr, ownershipError)
	}
	if response.Message != "Unable to set brightness" {
		t.Fatalf("cluster-owned response message = %q, want generic brightness failure", response.Message)
	}
	if strings.Contains(recorder.Body.String(), ownershipError) {
		t.Fatalf("cluster-owned response exposed internal error: %s", recorder.Body.String())
	}
	if got := device.GetBrightness(); got != initialDeviceBrightness {
		t.Fatalf("device brightness = %d, want unchanged value %d", got, initialDeviceBrightness)
	}
	if device.DeviceProfile != profile || profile.BrightnessSlider == nil || *profile.BrightnessSlider != initialProfileBrightness {
		t.Fatalf("active profile brightness = %#v, want unchanged value %d", profile.BrightnessSlider, initialProfileBrightness)
	}
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
		{name: "speed", path: "/api/openrgbimport/speed", body: `{"serial":"openrgb-lighting-test","effect":"rainbow","speed":4}`},
		{name: "two color", path: "/api/openrgbimport/two-color", body: `{"serial":"openrgb-lighting-test","effect":"wave","start":"#112233","end":"#aabbcc"}`},
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
