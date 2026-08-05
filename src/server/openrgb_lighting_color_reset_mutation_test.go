package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		name    string
		hex     string
		want    lightingsettings.Color
		wantErr bool
	}{
		{name: "uppercase valid", hex: "#1A2B3C", want: lightingsettings.Color{Red: 26, Green: 43, Blue: 60}, wantErr: false},
		{name: "lowercase valid", hex: "#1a2b3c", want: lightingsettings.Color{Red: 26, Green: 43, Blue: 60}, wantErr: false},
		{name: "missing hash", hex: "1A2B3C", wantErr: true},
		{name: "wrong length short", hex: "#1A2B3", wantErr: true},
		{name: "wrong length long", hex: "#1A2B3C4", wantErr: true},
		{name: "invalid hexadecimal", hex: "#1A2G3C", wantErr: true},
		{name: "shorthand", hex: "#123", wantErr: true},
		{name: "whitespace", hex: "# 1a2b3c", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHexColor(tt.hex)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseHexColor(%q) error = %v, wantErr %v", tt.hex, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("parseHexColor(%q) = %v, want %v", tt.hex, got, tt.want)
			}
		})
	}
}

func TestOpenRGBLightingColorMutationRequestValidation(t *testing.T) {
	_, _, calls := installLightingMutationTestSeams(t)
	router := setRoutes()

	colorCases := []struct {
		name      string
		body      string
		wantValue *lightingsettings.Color
	}{
		{name: "valid uppercase hex", body: `{"serial":"openrgb-lighting-test","effect":"static","color":"#AABBCC"}`, wantValue: &lightingsettings.Color{Red: 170, Green: 187, Blue: 204}},
		{name: "valid lowercase hex", body: `{"serial":"openrgb-lighting-test","effect":"static","color":"#aabbcc"}`, wantValue: &lightingsettings.Color{Red: 170, Green: 187, Blue: 204}},
		{name: "missing # rejected", body: `{"serial":"openrgb-lighting-test","effect":"static","color":"AABBCC"}`},
		{name: "wrong length rejected", body: `{"serial":"openrgb-lighting-test","effect":"static","color":"#AABB"}`},
		{name: "invalid hexadecimal rejected", body: `{"serial":"openrgb-lighting-test","effect":"static","color":"#AABBXX"}`},
		{name: "missing color rejected", body: `{"serial":"openrgb-lighting-test","effect":"static"}`},
		{name: "missing effect rejected", body: `{"serial":"openrgb-lighting-test","color":"#AABBCC"}`},
		{name: "missing serial rejected", body: `{"effect":"static","color":"#AABBCC"}`},
		{name: "unknown JSON fields rejected", body: `{"serial":"openrgb-lighting-test","effect":"static","color":"#AABBCC","extra":true}`},
		{name: "trailing JSON rejected", body: `{"serial":"openrgb-lighting-test","effect":"static","color":"#AABBCC"}{}`},
		{name: "oversized request rejected", body: strings.Repeat(" ", openRGBImportRequestLimit+1) + `{"serial":"openrgb-lighting-test","effect":"static","color":"#AABBCC"}`},
		{name: "unsupported effect rejected", body: `{"serial":"openrgb-lighting-test","effect":"UNSUPPORTED","color":"#AABBCC"}`},
	}
	for _, test := range colorCases {
		t.Run("color "+test.name, func(t *testing.T) {
			calls.colors = nil
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/single-color", test.body)
			if test.wantValue == nil {
				requireLightingMutationResponse(t, recorder, 0)
				if len(calls.colors) != 0 {
					t.Fatalf("color mutation calls = %v, want none", calls.colors)
				}
				return
			}
			response := requireLightingMutationResponse(t, recorder, 1)
			if response.Message != "Applied successfully" {
				t.Fatalf("success message = %q, want %q", response.Message, "Applied successfully")
			}
			want := lightingColorMutationCall{serial: lightingMutationTestSerial, effect: "static", color: *test.wantValue}
			if len(calls.colors) != 1 || calls.colors[0] != want {
				t.Fatalf("color mutation calls = %+v, want [%+v]", calls.colors, want)
			}
		})
	}

	t.Run("internal errors remain generic", func(t *testing.T) {
		calls.colors = nil
		calls.setError = errors.New("private color mutation failure at /tmp/internal")
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/single-color", `{"serial":"openrgb-lighting-test","effect":"static","color":"#AABBCC"}`)
		response := requireLightingMutationResponse(t, recorder, 0)
		if response.Message != "Failed to set device color" {
			t.Fatalf("unexpected message: %s", response.Message)
		}
		if strings.Contains(recorder.Body.String(), "private") || strings.Contains(recorder.Body.String(), "/tmp") {
			t.Fatalf("color error response exposed internal error: %s", recorder.Body.String())
		}
	})

	t.Run("cluster ownership rejected", func(t *testing.T) {
		previousLookup := lookupOpenRGBImportForLighting
		previousColor := setOpenRGBImportColorValue
		defer func() {
			lookupOpenRGBImportForLighting = previousLookup
			setOpenRGBImportColorValue = previousColor
		}()

		profile := &openrgbimport.DeviceProfile{Active: true, RGBProfile: "static", RGBCluster: true}
		device := &openrgbimport.Device{
			Serial:        lightingMutationTestSerial,
			IsOpenRGB:     true,
			RGBModes:      []string{"static"},
			DeviceProfile: profile,
			Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{
				"static": {ProfileName: "static"},
			}},
		}
		wrapper := &common.Device{Serial: lightingMutationTestSerial, Instance: device}
		lookupOpenRGBImportForLighting = func(serial string) (*common.Device, *openrgbimport.Device, bool) {
			return wrapper, device, serial == lightingMutationTestSerial
		}

		callCount := 0
		var observedErr error
		setOpenRGBImportColorValue = func(device *openrgbimport.Device, serial, effect string, color lightingsettings.Color) error {
			callCount++
			observedErr = device.SetEffectColor(serial, effect, color)
			return observedErr
		}

		recorder := requestOpenRGBLightingMutation(t, setRoutes(), http.MethodPost, "/api/openrgbimport/single-color", `{"serial":"openrgb-lighting-test","effect":"static","color":"#123456"}`)
		response := requireLightingMutationResponse(t, recorder, 0)
		if callCount != 1 {
			t.Fatalf("cluster-owned color mutation calls = %d, want 1", callCount)
		}
		const ownershipError = "device is controlled by RGB cluster"
		if observedErr == nil || observedErr.Error() != ownershipError {
			t.Fatalf("real SetEffectColor error = %v, want %q", observedErr, ownershipError)
		}
		if response.Message != "Failed to set device color" || strings.Contains(recorder.Body.String(), ownershipError) {
			t.Fatalf("cluster-owned color response = %s", recorder.Body.String())
		}
	})
}

func TestOpenRGBLightingResetMutationRequestValidation(t *testing.T) {
	_, _, calls := installLightingMutationTestSeams(t)
	router := setRoutes()

	resetCases := []struct {
		name      string
		body      string
		wantValid bool
	}{
		{name: "valid reset", body: `{"serial":"openrgb-lighting-test","effect":"static"}`, wantValid: true},
		{name: "missing fields rejected", body: `{"serial":"openrgb-lighting-test"}`},
		{name: "unknown fields rejected", body: `{"serial":"openrgb-lighting-test","effect":"static","extra":true}`},
		{name: "trailing JSON rejected", body: `{"serial":"openrgb-lighting-test","effect":"static"}{}`},
		{name: "stale effect rejected", body: `{"serial":"openrgb-lighting-test","effect":"UNSUPPORTED"}`},
	}
	for _, test := range resetCases {
		t.Run("reset "+test.name, func(t *testing.T) {
			calls.resets = nil
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/effect-reset", test.body)
			if !test.wantValid {
				requireLightingMutationResponse(t, recorder, 0)
				if len(calls.resets) != 0 {
					t.Fatalf("reset mutation calls = %v, want none", calls.resets)
				}
				return
			}
			response := requireLightingMutationResponse(t, recorder, 1)
			if response.Message != "Reset successfully" {
				t.Fatalf("success message = %q, want %q", response.Message, "Reset successfully")
			}
			want := lightingResetMutationCall{serial: lightingMutationTestSerial, effect: "static"}
			if len(calls.resets) != 1 || calls.resets[0] != want {
				t.Fatalf("reset mutation calls = %+v, want [%+v]", calls.resets, want)
			}
		})
	}

	t.Run("internal errors remain generic", func(t *testing.T) {
		calls.resets = nil
		calls.setError = errors.New("private reset mutation failure at /tmp/internal")
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/effect-reset", `{"serial":"openrgb-lighting-test","effect":"static"}`)
		response := requireLightingMutationResponse(t, recorder, 0)
		if response.Message != "Failed to reset effect customization" {
			t.Fatalf("unexpected message: %s", response.Message)
		}
		if strings.Contains(recorder.Body.String(), "private") || strings.Contains(recorder.Body.String(), "/tmp") {
			t.Fatalf("reset error response exposed internal error: %s", recorder.Body.String())
		}
	})

	t.Run("cluster ownership rejected", func(t *testing.T) {
		previousLookup := lookupOpenRGBImportForLighting
		previousReset := resetOpenRGBImportCustomizationValue
		defer func() {
			lookupOpenRGBImportForLighting = previousLookup
			resetOpenRGBImportCustomizationValue = previousReset
		}()

		profile := &openrgbimport.DeviceProfile{Active: true, RGBProfile: "static", RGBCluster: true}
		device := &openrgbimport.Device{
			Serial:        lightingMutationTestSerial,
			IsOpenRGB:     true,
			RGBModes:      []string{"static"},
			DeviceProfile: profile,
			Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{
				"static": {ProfileName: "static"},
			}},
		}
		wrapper := &common.Device{Serial: lightingMutationTestSerial, Instance: device}
		lookupOpenRGBImportForLighting = func(serial string) (*common.Device, *openrgbimport.Device, bool) {
			return wrapper, device, serial == lightingMutationTestSerial
		}

		callCount := 0
		var observedErr error
		resetOpenRGBImportCustomizationValue = func(device *openrgbimport.Device, serial, effect string) error {
			callCount++
			observedErr = device.ResetEffectCustomization(serial, effect)
			return observedErr
		}

		recorder := requestOpenRGBLightingMutation(t, setRoutes(), http.MethodPost, "/api/openrgbimport/effect-reset", `{"serial":"openrgb-lighting-test","effect":"static"}`)
		response := requireLightingMutationResponse(t, recorder, 0)
		if callCount != 1 {
			t.Fatalf("cluster-owned reset mutation calls = %d, want 1", callCount)
		}
		const ownershipError = "device is controlled by RGB cluster"
		if observedErr == nil || observedErr.Error() != ownershipError {
			t.Fatalf("real ResetEffectCustomization error = %v, want %q", observedErr, ownershipError)
		}
		if response.Message != "Failed to reset effect customization" || strings.Contains(recorder.Body.String(), ownershipError) {
			t.Fatalf("cluster-owned reset response = %s", recorder.Body.String())
		}
	})
}
