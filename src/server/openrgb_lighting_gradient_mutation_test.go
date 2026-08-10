package server

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"LumenForge/src/common"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/lightingsettings"
)

func TestOpenRGBLightingGradientMutationRequestValidation(t *testing.T) {
	_, _, calls := installLightingMutationTestSeams(t)
	router := setRoutes()

	validTwo := `{"serial":"openrgb-lighting-test","effect":"gradient","stops":[{"position":0,"color":"#A1B2C3","intensity":0},{"position":1,"color":"#D4E5F6","intensity":1}]}`
	validFour := `{"serial":"openrgb-lighting-test","effect":"gradient","stops":[{"position":0,"color":"#ff0000","intensity":1},{"position":0.33,"color":"#00ff00","intensity":0.75},{"position":0.33,"color":"#0000ff","intensity":0.5},{"position":1,"color":"#ffff00","intensity":0.25}]}`
	tests := []struct {
		name string
		body string
		want []lightingsettings.GradientStop
	}{
		{name: "valid two stops with zeros and uppercase", body: validTwo, want: []lightingsettings.GradientStop{
			{Position: 0, Color: lightingsettings.Color{Red: 161, Green: 178, Blue: 195}, Intensity: 0},
			{Position: 1, Color: lightingsettings.Color{Red: 212, Green: 229, Blue: 246}, Intensity: 1},
		}},
		{name: "valid four stops with equal decimal positions", body: validFour, want: []lightingsettings.GradientStop{
			{Position: 0, Color: lightingsettings.Color{Red: 255}, Intensity: 1},
			{Position: 0.33, Color: lightingsettings.Color{Green: 255}, Intensity: 0.75},
			{Position: 0.33, Color: lightingsettings.Color{Blue: 255}, Intensity: 0.5},
			{Position: 1, Color: lightingsettings.Color{Red: 255, Green: 255}, Intensity: 0.25},
		}},
		{name: "missing Stops", body: `{"serial":"openrgb-lighting-test","effect":"gradient"}`},
		{name: "null Stops", body: `{"serial":"openrgb-lighting-test","effect":"gradient","stops":null}`},
		{name: "too few", body: `{"serial":"openrgb-lighting-test","effect":"gradient","stops":[{"position":0,"color":"#000000","intensity":1}]}`},
		{name: "too many", body: `{"serial":"openrgb-lighting-test","effect":"gradient","stops":[` + strings.Repeat(`{},`, 1024) + `{}` + `]}`},
		{name: "missing Position", body: `{"serial":"openrgb-lighting-test","effect":"gradient","stops":[{"color":"#000000","intensity":1},{"position":1,"color":"#ffffff","intensity":1}]}`},
		{name: "missing Color", body: `{"serial":"openrgb-lighting-test","effect":"gradient","stops":[{"position":0,"intensity":1},{"position":1,"color":"#ffffff","intensity":1}]}`},
		{name: "missing Intensity", body: `{"serial":"openrgb-lighting-test","effect":"gradient","stops":[{"position":0,"color":"#000000"},{"position":1,"color":"#ffffff","intensity":1}]}`},
		{name: "malformed Color", body: strings.Replace(validTwo, "#A1B2C3", "red", 1)},
		{name: "unrepresentable Position", body: strings.Replace(validTwo, `"position":0`, `"position":1e999`, 1)},
		{name: "Position below", body: strings.Replace(validTwo, `"position":0`, `"position":-0.1`, 1)},
		{name: "Position above", body: strings.Replace(validTwo, `"position":1`, `"position":1.1`, 1)},
		{name: "Intensity below", body: strings.Replace(validTwo, `"intensity":0`, `"intensity":-0.1`, 1)},
		{name: "Intensity above", body: strings.Replace(validTwo, `"intensity":1`, `"intensity":1.1`, 1)},
		{name: "decreasing", body: strings.Replace(validFour, `"position":0.33,"color":"#0000ff"`, `"position":0.2,"color":"#0000ff"`, 1)},
		{name: "unknown top-level", body: strings.TrimSuffix(validTwo, "}") + `,"extra":true}`},
		{name: "unknown nested", body: strings.Replace(validTwo, `"intensity":0}`, `"intensity":0,"extra":true}`, 1)},
		{name: "trailing JSON", body: validTwo + `{}`},
		{name: "oversized", body: strings.Repeat(" ", openRGBImportRequestLimit+1) + validTwo},
		{name: "unsupported effect", body: strings.Replace(validTwo, `"gradient"`, `"wave"`, 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls.gradients = nil
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/gradient", test.body)
			if test.want == nil {
				requireLightingMutationResponse(t, recorder, 0)
				if len(calls.gradients) != 0 {
					t.Fatalf("Gradient calls = %#v, want none", calls.gradients)
				}
				return
			}
			response := requireLightingMutationResponse(t, recorder, 1)
			if response.Message != "Applied successfully" || len(calls.gradients) != 1 ||
				calls.gradients[0].serial != lightingMutationTestSerial || calls.gradients[0].effect != "gradient" ||
				!reflect.DeepEqual(calls.gradients[0].stops, test.want) {
				t.Fatalf("response %#v calls %#v, want %#v", response, calls.gradients, test.want)
			}
		})
	}

	t.Run("unavailable device", func(t *testing.T) {
		previousLookup := lookupOpenRGBImportForLighting
		lookupOpenRGBImportForLighting = func(string) (*common.Device, *openrgbimport.Device, bool) { return nil, nil, false }
		t.Cleanup(func() { lookupOpenRGBImportForLighting = previousLookup })
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/gradient", validTwo)
		response := requireLightingMutationResponse(t, recorder, 0)
		if response.Message != "OpenRGB import is unavailable" {
			t.Fatalf("response = %#v", response)
		}
	})

	t.Run("device errors remain generic", func(t *testing.T) {
		calls.gradients = nil
		calls.setError = errors.New("device is controlled by RGB cluster")
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/openrgbimport/gradient", validTwo)
		response := requireLightingMutationResponse(t, recorder, 0)
		if response.Message != "Failed to set Gradient" || strings.Contains(recorder.Body.String(), calls.setError.Error()) || len(calls.gradients) != 1 {
			t.Fatalf("response = %s calls %#v", recorder.Body.String(), calls.gradients)
		}
	})
}
