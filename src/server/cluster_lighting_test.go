package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"LumenForge/src/cluster"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

type clusterLightingMutationCalls struct {
	effects      []string
	brightness   []uint8
	speeds       []clusterLightingSpeedCall
	singleColors []clusterLightingSingleColorCall
	twoColors    []clusterLightingTwoColorCall
	temperatures []clusterLightingTemperatureCall
	gradients    []clusterLightingGradientCall
	resets       []string
	setError     error
}

type clusterLightingSpeedCall struct {
	effect string
	speed  float64
}

type clusterLightingSingleColorCall struct {
	effect string
	color  lightingsettings.Color
}

type clusterLightingTwoColorCall struct {
	effect     string
	start, end lightingsettings.Color
}

type clusterLightingTemperatureCall struct {
	effect            string
	low, middle, high lightingsettings.TemperaturePoint
}

type clusterLightingGradientCall struct {
	effect string
	stops  []lightingsettings.GradientStop
}

func installClusterLightingMutationTestSeams(t *testing.T) *clusterLightingMutationCalls {
	t.Helper()
	previousDevice := getClusterLightingDevice
	previousSnapshot := getClusterLightingSnapshot
	previousEffect := setClusterLightingEffect
	previousBrightness := setClusterLightingBrightness
	previousSpeed := setClusterLightingSpeed
	previousSingleColor := setClusterLightingSingleColor
	previousTwoColor := setClusterLightingTwoColor
	previousTemperature := setClusterLightingTemperature
	previousGradient := setClusterLightingGradient
	previousReset := setClusterLightingReset
	t.Cleanup(func() {
		getClusterLightingDevice = previousDevice
		getClusterLightingSnapshot = previousSnapshot
		setClusterLightingEffect = previousEffect
		setClusterLightingBrightness = previousBrightness
		setClusterLightingSpeed = previousSpeed
		setClusterLightingSingleColor = previousSingleColor
		setClusterLightingTwoColor = previousTwoColor
		setClusterLightingTemperature = previousTemperature
		setClusterLightingGradient = previousGradient
		setClusterLightingReset = previousReset
	})

	device := &cluster.Device{}
	calls := &clusterLightingMutationCalls{}
	getClusterLightingDevice = func() *cluster.Device { return device }
	setClusterLightingEffect = func(got *cluster.Device, effect string) error {
		if got != device {
			t.Fatal("effect mutation received a different Cluster device")
		}
		calls.effects = append(calls.effects, effect)
		return calls.setError
	}
	setClusterLightingBrightness = func(got *cluster.Device, brightness uint8) error {
		if got != device {
			t.Fatal("brightness mutation received a different Cluster device")
		}
		calls.brightness = append(calls.brightness, brightness)
		return calls.setError
	}
	setClusterLightingSpeed = func(got *cluster.Device, effect string, speed float64) error {
		if got != device {
			t.Fatal("Speed mutation received a different Cluster device")
		}
		calls.speeds = append(calls.speeds, clusterLightingSpeedCall{effect: effect, speed: speed})
		return calls.setError
	}
	setClusterLightingSingleColor = func(got *cluster.Device, effect string, color lightingsettings.Color) error {
		if got != device {
			t.Fatal("single-color mutation received a different Cluster device")
		}
		calls.singleColors = append(calls.singleColors, clusterLightingSingleColorCall{effect: effect, color: color})
		return calls.setError
	}
	setClusterLightingTwoColor = func(got *cluster.Device, effect string, start, end lightingsettings.Color) error {
		if got != device {
			t.Fatal("two-color mutation received a different Cluster device")
		}
		calls.twoColors = append(calls.twoColors, clusterLightingTwoColorCall{effect: effect, start: start, end: end})
		return calls.setError
	}
	setClusterLightingTemperature = func(got *cluster.Device, effect string, low, middle, high lightingsettings.TemperaturePoint) error {
		if got != device {
			t.Fatal("temperature mutation received a different Cluster device")
		}
		calls.temperatures = append(calls.temperatures, clusterLightingTemperatureCall{effect: effect, low: low, middle: middle, high: high})
		return calls.setError
	}
	setClusterLightingGradient = func(got *cluster.Device, effect string, stops []lightingsettings.GradientStop) error {
		if got != device {
			t.Fatal("Gradient mutation received a different Cluster device")
		}
		calls.gradients = append(calls.gradients, clusterLightingGradientCall{effect: effect, stops: append([]lightingsettings.GradientStop(nil), stops...)})
		return calls.setError
	}
	setClusterLightingReset = func(got *cluster.Device, effect string) error {
		if got != device {
			t.Fatal("reset mutation received a different Cluster device")
		}
		calls.resets = append(calls.resets, effect)
		return calls.setError
	}
	return calls
}

func TestRGBClusterLightingStatusRouteUsesCanonicalSnapshotReadOnly(t *testing.T) {
	previousDevice := getClusterLightingDevice
	previousSnapshot := getClusterLightingSnapshot
	t.Cleanup(func() {
		getClusterLightingDevice = previousDevice
		getClusterLightingSnapshot = previousSnapshot
	})
	device := &cluster.Device{}
	getClusterLightingDevice = func() *cluster.Device { return device }
	snapshotCalls := 0
	getClusterLightingSnapshot = func(got *cluster.Device) cluster.LightingSnapshot {
		if got != device {
			t.Fatal("status requested a different Cluster device")
		}
		snapshotCalls++
		return cluster.LightingSnapshot{Available: true, SelectedEffect: "wave"}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/cluster/lighting/status", nil)
	request.Host = "127.0.0.1"
	setRoutes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status response code = %d", recorder.Code)
	}
	var response clusterLightingStatusResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Status != 1 || response.Effect != "wave" || snapshotCalls != 1 {
		t.Fatalf("status response/calls = %#v/%d", response, snapshotCalls)
	}
}

func TestRGBClusterLightingStatusRouteFailsClosedWithoutDetails(t *testing.T) {
	previousDevice := getClusterLightingDevice
	t.Cleanup(func() { getClusterLightingDevice = previousDevice })
	getClusterLightingDevice = func() *cluster.Device { return nil }

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/cluster/lighting/status", nil)
	request.Host = "127.0.0.1"
	setRoutes().ServeHTTP(recorder, request)
	if strings.Contains(recorder.Body.String(), "unavailable") || strings.Contains(recorder.Body.String(), "error") {
		t.Fatalf("unavailable response leaked details: %q", recorder.Body.String())
	}
	var response clusterLightingStatusResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Status != 0 || response.Effect != "" {
		t.Fatalf("unavailable response = %#v", response)
	}
}

func TestClusterLightingMutationRoutesCallCanonicalMethodsOnce(t *testing.T) {
	calls := installClusterLightingMutationTestSeams(t)
	router := setRoutes()

	tests := []struct {
		name   string
		path   string
		body   string
		assert func(*testing.T)
	}{
		{name: "effect", path: "/api/cluster/lighting/effect", body: `{"effect":"static"}`, assert: func(t *testing.T) {
			if !reflect.DeepEqual(calls.effects, []string{"static"}) {
				t.Fatalf("effect calls = %#v", calls.effects)
			}
		}},
		{name: "effect reset", path: "/api/cluster/lighting/effect-reset", body: `{"effect":"static"}`, assert: func(t *testing.T) {
			if !reflect.DeepEqual(calls.resets, []string{"static"}) {
				t.Fatalf("reset calls = %#v", calls.resets)
			}
		}},
		{name: "brightness", path: "/api/cluster/lighting/brightness", body: `{"brightness":60}`, assert: func(t *testing.T) {
			if !reflect.DeepEqual(calls.brightness, []uint8{60}) {
				t.Fatalf("brightness calls = %#v", calls.brightness)
			}
		}},
		{name: "speed", path: "/api/cluster/lighting/speed", body: `{"effect":"wave","speed":4}`, assert: func(t *testing.T) {
			if !reflect.DeepEqual(calls.speeds, []clusterLightingSpeedCall{{effect: "wave", speed: 4}}) {
				t.Fatalf("Speed calls = %#v", calls.speeds)
			}
		}},
		{name: "single color", path: "/api/cluster/lighting/single-color", body: `{"effect":"static","color":"#010203"}`, assert: func(t *testing.T) {
			want := []clusterLightingSingleColorCall{{effect: "static", color: lightingsettings.Color{Red: 1, Green: 2, Blue: 3}}}
			if !reflect.DeepEqual(calls.singleColors, want) {
				t.Fatalf("single-color calls = %#v", calls.singleColors)
			}
		}},
		{name: "two color", path: "/api/cluster/lighting/two-color", body: `{"effect":"wave","start":"#112233","end":"#aabbcc"}`, assert: func(t *testing.T) {
			want := []clusterLightingTwoColorCall{{effect: "wave", start: lightingsettings.Color{Red: 17, Green: 34, Blue: 51}, end: lightingsettings.Color{Red: 170, Green: 187, Blue: 204}}}
			if !reflect.DeepEqual(calls.twoColors, want) {
				t.Fatalf("two-color calls = %#v", calls.twoColors)
			}
		}},
		{name: "temperature", path: "/api/cluster/lighting/temperature", body: `{"effect":"cpu-temperature","low":{"color":"#010203","celsius":20},"middle":{"color":"#040506","celsius":50},"high":{"color":"#070809","celsius":95}}`, assert: func(t *testing.T) {
			if len(calls.temperatures) != 1 || calls.temperatures[0].effect != "cpu-temperature" || calls.temperatures[0].middle.Celsius != 50 || calls.temperatures[0].middle.Color != (lightingsettings.Color{Red: 4, Green: 5, Blue: 6}) {
				t.Fatalf("temperature calls = %#v", calls.temperatures)
			}
		}},
		{name: "Gradient", path: "/api/cluster/lighting/gradient", body: `{"effect":"gradient","stops":[{"position":0.5,"color":"#010203","intensity":0.25},{"position":0.5,"color":"#040506","intensity":0.75}]}`, assert: func(t *testing.T) {
			want := []lightingsettings.GradientStop{
				{Position: 0.5, Color: lightingsettings.Color{Red: 1, Green: 2, Blue: 3}, Intensity: 0.25},
				{Position: 0.5, Color: lightingsettings.Color{Red: 4, Green: 5, Blue: 6}, Intensity: 0.75},
			}
			if len(calls.gradients) != 1 || calls.gradients[0].effect != "gradient" || !reflect.DeepEqual(calls.gradients[0].stops, want) {
				t.Fatalf("Gradient calls = %#v", calls.gradients)
			}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, test.path, test.body)
			requireLightingMutationResponse(t, recorder, 1)
			test.assert(t)
		})
	}
}

func TestClusterLightingMutationRoutesStrictlyRejectInvalidRequests(t *testing.T) {
	calls := installClusterLightingMutationTestSeams(t)
	router := setRoutes()
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "unknown field", path: "/api/cluster/lighting/effect", body: `{"effect":"static","extra":true}`},
		{name: "missing effect", path: "/api/cluster/lighting/effect", body: `{}`},
		{name: "unknown effect", path: "/api/cluster/lighting/effect", body: `{"effect":"missing"}`},
		{name: "malformed", path: "/api/cluster/lighting/effect", body: `{"effect":`},
		{name: "trailing JSON", path: "/api/cluster/lighting/effect", body: `{"effect":"static"}{}`},
		{name: "oversized", path: "/api/cluster/lighting/effect", body: strings.Repeat(" ", clusterLightingRequestLimit+1) + `{"effect":"static"}`},
		{name: "reset missing effect", path: "/api/cluster/lighting/effect-reset", body: `{}`},
		{name: "reset unknown effect", path: "/api/cluster/lighting/effect-reset", body: `{"effect":"missing"}`},
		{name: "reset rejects serial", path: "/api/cluster/lighting/effect-reset", body: `{"effect":"static","serial":"not-used"}`},
		{name: "reset malformed", path: "/api/cluster/lighting/effect-reset", body: `{"effect":`},
		{name: "reset trailing JSON", path: "/api/cluster/lighting/effect-reset", body: `{"effect":"static"}{}`},
		{name: "brightness missing", path: "/api/cluster/lighting/brightness", body: `{}`},
		{name: "brightness fraction", path: "/api/cluster/lighting/brightness", body: `{"brightness":1.5}`},
		{name: "brightness range", path: "/api/cluster/lighting/brightness", body: `{"brightness":101}`},
		{name: "Speed missing", path: "/api/cluster/lighting/speed", body: `{"effect":"wave"}`},
		{name: "Speed non-finite", path: "/api/cluster/lighting/speed", body: `{"effect":"wave","speed":1e999}`},
		{name: "Speed unsupported", path: "/api/cluster/lighting/speed", body: `{"effect":"off","speed":1}`},
		{name: "single color missing", path: "/api/cluster/lighting/single-color", body: `{"effect":"static"}`},
		{name: "single color format", path: "/api/cluster/lighting/single-color", body: `{"effect":"static","color":"red"}`},
		{name: "single color wrong palette", path: "/api/cluster/lighting/single-color", body: `{"effect":"wave","color":"#010203"}`},
		{name: "two color incomplete", path: "/api/cluster/lighting/two-color", body: `{"effect":"wave","start":"#010203"}`},
		{name: "two color wrong palette", path: "/api/cluster/lighting/two-color", body: `{"effect":"static","start":"#010203","end":"#040506"}`},
		{name: "temperature incomplete", path: "/api/cluster/lighting/temperature", body: `{"effect":"cpu-temperature","low":{"color":"#010203","celsius":20},"high":{"color":"#070809","celsius":95}}`},
		{name: "temperature unordered", path: "/api/cluster/lighting/temperature", body: `{"effect":"cpu-temperature","low":{"color":"#010203","celsius":20},"middle":{"color":"#040506","celsius":95},"high":{"color":"#070809","celsius":50}}`},
		{name: "Gradient too short", path: "/api/cluster/lighting/gradient", body: `{"effect":"gradient","stops":[{"position":0,"color":"#010203","intensity":1}]}`},
		{name: "Gradient incomplete", path: "/api/cluster/lighting/gradient", body: `{"effect":"gradient","stops":[{"position":0,"color":"#010203","intensity":1},{"position":1,"color":"#040506"}]}`},
		{name: "Gradient unordered", path: "/api/cluster/lighting/gradient", body: `{"effect":"gradient","stops":[{"position":1,"color":"#010203","intensity":1},{"position":0,"color":"#040506","intensity":1}]}`},
		{name: "Gradient range", path: "/api/cluster/lighting/gradient", body: `{"effect":"gradient","stops":[{"position":0,"color":"#010203","intensity":1.1},{"position":1,"color":"#040506","intensity":1}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, test.path, test.body)
			requireLightingMutationResponse(t, recorder, 0)
		})
	}
	if len(calls.effects)+len(calls.brightness)+len(calls.speeds)+len(calls.singleColors)+len(calls.twoColors)+len(calls.temperatures)+len(calls.gradients)+len(calls.resets) != 0 {
		t.Fatalf("invalid requests reached mutation seams: %#v", calls)
	}
}

func TestClusterLightingResetFailureIsGenericAndProtected(t *testing.T) {
	calls := installClusterLightingMutationTestSeams(t)
	calls.setError = errors.New("private persistence path /secret/cluster.json")
	router := setRoutes()
	recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/cluster/lighting/effect-reset", `{"effect":"static"}`)
	response := requireLightingMutationResponse(t, recorder, 0)
	if !reflect.DeepEqual(calls.resets, []string{"static"}) || strings.Contains(recorder.Body.String(), "/secret/") || strings.Contains(response.Message, "persistence") {
		t.Fatalf("failed reset response = %#v, body %q, calls %#v", response, recorder.Body.String(), calls.resets)
	}

	unprotected := httptest.NewRequest(http.MethodPost, "/api/cluster/lighting/effect-reset", strings.NewReader(`{"effect":"static"}`))
	unprotected.Host = "127.0.0.1"
	unprotected.Header.Set("Content-Type", "application/json")
	unprotectedRecorder := httptest.NewRecorder()
	router.ServeHTTP(unprotectedRecorder, unprotected)
	if unprotectedRecorder.Code != http.StatusForbidden || len(calls.resets) != 1 {
		t.Fatalf("unprotected reset response = %d %q, calls %#v", unprotectedRecorder.Code, unprotectedRecorder.Body.String(), calls.resets)
	}
}

func TestClusterLightingMutationFailureIsGenericAndProtected(t *testing.T) {
	calls := installClusterLightingMutationTestSeams(t)
	calls.setError = errors.New("private persistence path /secret/cluster.json")
	router := setRoutes()
	recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/cluster/lighting/effect", `{"effect":"static"}`)
	response := requireLightingMutationResponse(t, recorder, 0)
	if len(calls.effects) != 1 || strings.Contains(recorder.Body.String(), "/secret/") || strings.Contains(response.Message, "persistence") {
		t.Fatalf("failed mutation response = %#v, body %q, calls %#v", response, recorder.Body.String(), calls.effects)
	}

	unprotected := httptest.NewRequest(http.MethodPost, "/api/cluster/lighting/effect", strings.NewReader(`{"effect":"static"}`))
	unprotected.Host = "127.0.0.1"
	unprotected.Header.Set("Content-Type", "application/json")
	unprotectedRecorder := httptest.NewRecorder()
	router.ServeHTTP(unprotectedRecorder, unprotected)
	if unprotectedRecorder.Code != http.StatusForbidden || len(calls.effects) != 1 {
		t.Fatalf("unprotected response = %d %q, calls %#v", unprotectedRecorder.Code, unprotectedRecorder.Body.String(), calls.effects)
	}
}

func TestClusterLightingWorkspaceSummaryUsesCanonicalSnapshot(t *testing.T) {
	speed := 4.0
	stops := []lightingsettings.GradientStop{
		{Position: 0, Color: lightingsettings.Color{Red: 1, Green: 2, Blue: 3}, Intensity: 0.25},
		{Position: 0.5, Color: lightingsettings.Color{Red: 4, Green: 5, Blue: 6}, Intensity: 0.5},
		{Position: 0.5, Color: lightingsettings.Color{Red: 7, Green: 8, Blue: 9}, Intensity: 0.75},
	}
	snapshot := cluster.LightingSnapshot{
		SelectedEffect: "gradient", Brightness: 60, EffectiveBrightness: 0, Customized: true,
		ControllerCount: 3, Available: true,
		Settings: lightingsettings.EffectSettings{
			SchemaVersion: lightingsettings.SchemaVersion, EffectID: "gradient", Speed: &speed,
			Gradient: &lightingsettings.GradientSettings{Stops: stops},
		},
	}
	summary := clusterLightingWorkspaceSummaryFromSnapshot(snapshot)
	if !summary.Available || summary.ControllerCount != 3 || summary.ConfiguredEffect != "gradient" ||
		summary.ConfiguredEffectLabel != "Gradient" || summary.ConfiguredEffectIconURL != "/static/img/icons/rgb/gradient.svg" ||
		!summary.EffectSupported || summary.Brightness != 60 || summary.EffectiveBrightness != 0 ||
		!summary.HasSpeedControl || summary.Speed != "4" || summary.PaletteKind != string(rgb.LightingPaletteGradient) ||
		!summary.HasGradient || len(summary.GradientStops) != 3 || summary.GradientStops[1].Position != "0.5" ||
		summary.GradientStops[2].ColorHex != "#070809" || summary.GradientStops[2].Intensity != "0.75" || !summary.Customized {
		t.Fatalf("Gradient Cluster summary = %#v", summary)
	}
	selected := 0
	for index, effect := range summary.SupportedEffects {
		if index > 0 && strings.ToLower(summary.SupportedEffects[index-1].Label) > strings.ToLower(effect.Label) {
			t.Fatalf("supported effects are not label-sorted: %#v", summary.SupportedEffects)
		}
		if effect.Selected {
			selected++
			if effect.ID != "gradient" {
				t.Fatalf("selected effect = %#v", effect)
			}
		}
		descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect.ID)
		if !ok || !descriptor.Scope.Includes(rgb.EffectScopeCluster) {
			t.Fatalf("non-Cluster effect in summary = %#v", effect)
		}
	}
	if selected != 1 {
		t.Fatalf("selected supported-effect count = %d", selected)
	}
	snapshot.Settings.Gradient.Stops[0].Color.Red = 255
	if summary.GradientStops[0].ColorHex != "#010203" {
		t.Fatalf("summary aliased snapshot data: %#v", summary.GradientStops)
	}
}

func TestClusterLightingWorkspaceSummaryPresentsCompletePaletteContracts(t *testing.T) {
	tests := []struct {
		name     string
		effect   string
		settings lightingsettings.EffectSettings
		assert   func(*testing.T, *clusterLightingWorkspaceSummary)
	}{
		{name: "single color", effect: "static", settings: lightingsettings.EffectSettings{
			SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static",
			SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 17, Green: 34, Blue: 51}},
		}, assert: func(t *testing.T, summary *clusterLightingWorkspaceSummary) {
			if summary.SingleColorHex != "#112233" {
				t.Fatalf("single color = %#v", summary)
			}
		}},
		{name: "two color", effect: "wave", settings: func() lightingsettings.EffectSettings {
			speed := 2.0
			return lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "wave", Speed: &speed,
				TwoColor: &lightingsettings.TwoColorSettings{Start: lightingsettings.Color{Red: 1}, End: lightingsettings.Color{Blue: 2}}}
		}(), assert: func(t *testing.T, summary *clusterLightingWorkspaceSummary) {
			if summary.TwoColorStartHex != "#010000" || summary.TwoColorEndHex != "#000002" {
				t.Fatalf("two colors = %#v", summary)
			}
		}},
		{name: "temperature", effect: "cpu-temperature", settings: lightingsettings.EffectSettings{
			SchemaVersion: lightingsettings.SchemaVersion, EffectID: "cpu-temperature",
			Temperature: &lightingsettings.TemperatureSettings{
				Low:    lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Blue: 1}, Celsius: 20},
				Middle: lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Green: 2}, Celsius: 50},
				High:   lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Red: 3}, Celsius: 95},
			},
		}, assert: func(t *testing.T, summary *clusterLightingWorkspaceSummary) {
			if !summary.HasTemperature || len(summary.TemperaturePoints) != 3 || summary.TemperatureMiddle.Label != "Middle" ||
				summary.TemperatureMiddle.ColorHex != "#000200" || summary.TemperatureMiddle.Celsius != "50" {
				t.Fatalf("temperature points = %#v", summary)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			summary := clusterLightingWorkspaceSummaryFromSnapshot(cluster.LightingSnapshot{
				SelectedEffect: test.effect, Brightness: 100, EffectiveBrightness: 100, Available: true, Settings: test.settings,
			})
			if !summary.Available || summary.PaletteKind == "" {
				t.Fatalf("palette summary = %#v", summary)
			}
			test.assert(t, summary)
		})
	}
}

func TestClusterLightingWorkspaceSummaryFailsClosed(t *testing.T) {
	if summary := clusterLightingWorkspaceSummaryFromSnapshot(cluster.LightingSnapshot{}); summary.Available || summary.ConfiguredEffect != "" || len(summary.SupportedEffects) != 0 {
		t.Fatalf("unavailable summary = %#v", summary)
	}
	invalid := clusterLightingWorkspaceSummaryFromSnapshot(cluster.LightingSnapshot{
		SelectedEffect: "static", Available: true,
		Settings: lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "wave"},
	})
	if invalid.Available || invalid.ConfiguredEffect != "" || len(invalid.SupportedEffects) != 0 {
		t.Fatalf("invalid summary = %#v", invalid)
	}
}
