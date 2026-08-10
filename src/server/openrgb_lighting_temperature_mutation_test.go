package server

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"LumenForge/src/common"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/lightingsettings"
)

func TestOpenRGBLightingTemperatureMutationRequestValidation(t *testing.T) {
	_, _, calls := installLightingMutationTestSeams(t)
	router := setRoutes()

	valid := func(effect string) string {
		return `{"serial":"openrgb-lighting-test","effect":"` + effect + `","low":{"color":"#A1B2C3","celsius":20.5},"middle":{"color":"#D4E5F6","celsius":50.25},"high":{"color":"#010203","celsius":90.75}}`
	}
	for _, effect := range []string{"cpu-temperature", "gpu-temperature"} {
		t.Run("valid "+effect, func(t *testing.T) {
			calls.temperatures = nil
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/temperature", valid(effect))
			response := requireLightingMutationResponse(t, recorder, 1)
			want := lightingTemperatureMutationCall{
				serial: lightingMutationTestSerial,
				effect: effect,
				low:    lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Red: 161, Green: 178, Blue: 195}, Celsius: 20.5},
				middle: lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Red: 212, Green: 229, Blue: 246}, Celsius: 50.25},
				high:   lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Red: 1, Green: 2, Blue: 3}, Celsius: 90.75},
			}
			if response.Message != "Applied successfully" || len(calls.temperatures) != 1 || calls.temperatures[0] != want {
				t.Fatalf("response %#v calls %#v, want %#v", response, calls.temperatures, want)
			}
		})
	}

	invalid := []struct {
		name string
		body string
	}{
		{name: "missing low", body: `{"serial":"openrgb-lighting-test","effect":"cpu-temperature","middle":{"color":"#ffff00","celsius":50},"high":{"color":"#ff0000","celsius":95}}`},
		{name: "missing middle", body: `{"serial":"openrgb-lighting-test","effect":"cpu-temperature","low":{"color":"#00ff00","celsius":20},"high":{"color":"#ff0000","celsius":95}}`},
		{name: "missing high", body: `{"serial":"openrgb-lighting-test","effect":"cpu-temperature","low":{"color":"#00ff00","celsius":20},"middle":{"color":"#ffff00","celsius":50}}`},
		{name: "missing nested color", body: `{"serial":"openrgb-lighting-test","effect":"cpu-temperature","low":{"celsius":20},"middle":{"color":"#ffff00","celsius":50},"high":{"color":"#ff0000","celsius":95}}`},
		{name: "missing nested Celsius", body: `{"serial":"openrgb-lighting-test","effect":"cpu-temperature","low":{"color":"#00ff00"},"middle":{"color":"#ffff00","celsius":50},"high":{"color":"#ff0000","celsius":95}}`},
		{name: "invalid low color", body: `{"serial":"openrgb-lighting-test","effect":"cpu-temperature","low":{"color":"green","celsius":20},"middle":{"color":"#ffff00","celsius":50},"high":{"color":"#ff0000","celsius":95}}`},
		{name: "invalid middle color", body: `{"serial":"openrgb-lighting-test","effect":"cpu-temperature","low":{"color":"#00ff00","celsius":20},"middle":{"color":"yellow","celsius":50},"high":{"color":"#ff0000","celsius":95}}`},
		{name: "invalid high color", body: `{"serial":"openrgb-lighting-test","effect":"cpu-temperature","low":{"color":"#00ff00","celsius":20},"middle":{"color":"#ffff00","celsius":50},"high":{"color":"red","celsius":95}}`},
		{name: "equal", body: `{"serial":"openrgb-lighting-test","effect":"cpu-temperature","low":{"color":"#00ff00","celsius":50},"middle":{"color":"#ffff00","celsius":50},"high":{"color":"#ff0000","celsius":95}}`},
		{name: "reversed", body: `{"serial":"openrgb-lighting-test","effect":"cpu-temperature","low":{"color":"#00ff00","celsius":90},"middle":{"color":"#ffff00","celsius":50},"high":{"color":"#ff0000","celsius":20}}`},
		{name: "unknown top-level", body: strings.TrimSuffix(valid("cpu-temperature"), "}") + `,"extra":true}`},
		{name: "unknown nested", body: `{"serial":"openrgb-lighting-test","effect":"cpu-temperature","low":{"color":"#00ff00","celsius":20,"extra":true},"middle":{"color":"#ffff00","celsius":50},"high":{"color":"#ff0000","celsius":95}}`},
		{name: "trailing JSON", body: valid("cpu-temperature") + `{}`},
		{name: "oversized", body: strings.Repeat(" ", openRGBImportRequestLimit+1) + valid("cpu-temperature")},
		{name: "unsupported", body: strings.Replace(valid("cpu-temperature"), "cpu-temperature", "wave", 1)},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			calls.temperatures = nil
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/temperature", test.body)
			requireLightingMutationResponse(t, recorder, 0)
			if len(calls.temperatures) != 0 {
				t.Fatalf("temperature calls = %#v, want none", calls.temperatures)
			}
		})
	}

	t.Run("unavailable device", func(t *testing.T) {
		previousLookup := lookupOpenRGBImportForLighting
		lookupOpenRGBImportForLighting = func(string) (*common.Device, *openrgbimport.Device, bool) { return nil, nil, false }
		t.Cleanup(func() { lookupOpenRGBImportForLighting = previousLookup })
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/temperature", valid("cpu-temperature"))
		response := requireLightingMutationResponse(t, recorder, 0)
		if response.Message != "OpenRGB import is unavailable" {
			t.Fatalf("response = %#v", response)
		}
	})

	t.Run("device errors remain generic", func(t *testing.T) {
		calls.temperatures = nil
		calls.setError = errors.New("device is controlled by RGB cluster")
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/temperature", valid("cpu-temperature"))
		response := requireLightingMutationResponse(t, recorder, 0)
		if response.Message != "Failed to set temperature colors" || strings.Contains(recorder.Body.String(), calls.setError.Error()) || len(calls.temperatures) != 1 {
			t.Fatalf("response = %s calls %#v", recorder.Body.String(), calls.temperatures)
		}
	})
}
