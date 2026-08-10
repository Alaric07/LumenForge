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
	"fmt"
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
		ConfiguredEffect:  "wave",
		EffectSupported:   true,
		HasBrightness:     true,
		Brightness:        0,
		ClusterControlled: true,
		SupportedEffects: []openrgbimport.LightingEffectOption{
			{ID: "future-effect", Label: "Future Effect"},
			{
				ID:    "wave",
				Label: "Wave <Label> & More",
			},
		},
		HasSpeed:          true,
		Speed:             0.5,
		PaletteKind:       "Two color",
		SingleColorHex:    "#ff0000",
		TwoColorStartHex:  "#112233",
		TwoColorEndHex:    "#aabbcc",
		HasTemperature:    true,
		TemperatureLow:    openrgbimport.LightingTemperaturePointSnapshot{ColorHex: "#010203", Celsius: 20.5},
		TemperatureMiddle: openrgbimport.LightingTemperaturePointSnapshot{ColorHex: "#040506", Celsius: 50.25},
		TemperatureHigh:   openrgbimport.LightingTemperaturePointSnapshot{ColorHex: "#070809", Celsius: 80.75},
		HasGradient:       true,
		GradientStops: []openrgbimport.LightingGradientStopSnapshot{
			{Position: 0, ColorHex: "#112233", Intensity: 0},
			{Position: 1, ColorHex: "#aabbcc", Intensity: 1},
		},
		Customized: true,
	}

	summary := openRGBLightingWorkspaceSummaryFromSnapshot(source)
	if summary.ConfiguredEffect != "wave" || summary.ConfiguredEffectLabel != "Wave <Label> & More" ||
		summary.ConfiguredEffectIconURL != "/static/img/icons/rgb/wave.svg" ||
		!summary.EffectSupported || !summary.HasBrightness || summary.Brightness != 0 || !summary.ClusterControlled ||
		!summary.HasSpeedControl || summary.Speed != "0.5" || summary.PaletteKind != "Two color" || summary.SingleColorHex != "#ff0000" ||
		summary.TwoColorStartHex != "#112233" || summary.TwoColorEndHex != "#aabbcc" || !summary.Customized {
		t.Fatalf("configured Lighting presentation = %#v", summary)
	}
	if !summary.HasTemperature || len(summary.TemperaturePoints) != 3 ||
		summary.TemperaturePoints[0] != (openRGBLightingTemperaturePointSummary{Role: "low", Label: "Low", ColorHex: "#010203", Celsius: "20.5"}) ||
		summary.TemperaturePoints[1] != (openRGBLightingTemperaturePointSummary{Role: "middle", Label: "Middle", ColorHex: "#040506", Celsius: "50.25"}) ||
		summary.TemperaturePoints[2] != (openRGBLightingTemperaturePointSummary{Role: "high", Label: "High", ColorHex: "#070809", Celsius: "80.75"}) {
		t.Fatalf("temperature presentation = %#v", summary.TemperaturePoints)
	}
	if !summary.HasGradient || len(summary.GradientStops) != 2 ||
		summary.GradientStops[0] != (openRGBLightingGradientStopSummary{Number: 1, Position: "0", ColorHex: "#112233", Intensity: "0"}) ||
		summary.GradientStops[1] != (openRGBLightingGradientStopSummary{Number: 2, Position: "1", ColorHex: "#aabbcc", Intensity: "1"}) {
		t.Fatalf("Gradient presentation = %#v", summary.GradientStops)
	}
	if len(summary.SupportedEffects) != 2 ||
		summary.SupportedEffects[0].ID != "future-effect" || summary.SupportedEffects[0].Label != "Future Effect" || summary.SupportedEffects[0].Selected ||
		summary.SupportedEffects[1].ID != "wave" || summary.SupportedEffects[1].Label != "Wave <Label> & More" || !summary.SupportedEffects[1].Selected {
		t.Fatalf("supported Lighting effects = %#v", summary.SupportedEffects)
	}

	source.SupportedEffects[0].ID = "mutated"
	if summary.SupportedEffects[0].ID != "future-effect" {
		t.Fatal("Lighting presentation retained mutable snapshot data")
	}
}

func TestDevicesLightingEffectIconInventory(t *testing.T) {
	root := devicesThemeRepositoryRoot(t)
	descriptors := rgb.SoftwareEffectDescriptors()
	deviceDescriptors := make([]rgb.SoftwareEffectDescriptor, 0, len(descriptors))
	seenIDs := make(map[string]string, len(descriptors))

	for _, descriptor := range descriptors {
		if !descriptor.Scope.Includes(rgb.EffectScopeDevice) {
			continue
		}
		deviceDescriptors = append(deviceDescriptors, descriptor)
		if previous, exists := seenIDs[descriptor.ID]; exists && previous != descriptor.Icon {
			t.Errorf("software effect ID %q maps to conflicting icons %q and %q", descriptor.ID, previous, descriptor.Icon)
		}
		seenIDs[descriptor.ID] = descriptor.Icon

		if descriptor.Icon == "" || !strings.HasSuffix(descriptor.Icon, ".svg") {
			t.Errorf("software effect %q icon = %q, want a non-empty SVG filename", descriptor.ID, descriptor.Icon)
			continue
		}
		if filepath.Base(descriptor.Icon) != descriptor.Icon || strings.ContainsAny(descriptor.Icon, `/\\`) {
			t.Errorf("software effect %q icon is not a safe filename: %q", descriptor.ID, descriptor.Icon)
			continue
		}

		wantURL := "/static/img/icons/rgb/" + descriptor.Icon
		summary := openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
			ConfiguredEffect: descriptor.ID,
			EffectSupported:  true,
			SupportedEffects: []openrgbimport.LightingEffectOption{{ID: descriptor.ID, Label: descriptor.Label}},
		})
		if summary.ConfiguredEffectIconURL != wantURL {
			t.Errorf("software effect %q icon URL = %q, want %q", descriptor.ID, summary.ConfiguredEffectIconURL, wantURL)
		}
		if _, err := os.Stat(filepath.Join(root, "static", "img", "icons", "rgb", descriptor.Icon)); err != nil {
			t.Errorf("software effect %q icon asset %q: %v", descriptor.ID, descriptor.Icon, err)
		}
	}

	if len(deviceDescriptors) != 35 {
		t.Fatalf("device-scoped software effect descriptors = %d, want 35", len(deviceDescriptors))
	}
	for id, wantIcon := range map[string]string{
		"off":                 "off.svg",
		"aurora":              "aurora.svg",
		"spiralrainbow":       "spiralrainbow.svg",
		"pastelspiralrainbow": "pastelspiralrainbow.svg",
		"visor":               "visor.svg",
		"watercolor":          "watercolor.svg",
	} {
		descriptor, ok := rgb.SoftwareEffectDescriptorByID(id)
		if !ok || descriptor.Icon != wantIcon {
			t.Errorf("software effect %q icon = %q, found = %t, want %q", id, descriptor.Icon, ok, wantIcon)
		}
	}

	for _, id := range []string{"unknown", "../wave", `wave');background:url(/escaped.svg);/*`} {
		if got := openRGBLightingEffectIconURL(id); got != "" {
			t.Errorf("non-canonical software effect ID %q produced icon URL %q", id, got)
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

func devicesLightingSpeedSnapshot(effect string, speed float64) openrgbimport.LightingSnapshot {
	capability, _ := rgb.LightingEffectCapabilities(effect)
	return openrgbimport.LightingSnapshot{
		ConfiguredEffect: effect,
		EffectSupported:  true,
		SupportedEffects: []openrgbimport.LightingEffectOption{{
			ID:    effect,
			Label: effect,
		}},
		HasSpeed: capability.SupportsSpeed,
		Speed:    speed,
	}
}

func TestDevicesLightingSpeedControlPresentation(t *testing.T) {
	for _, effect := range []string{"circle", "flame", "cyberpunkglitch", "rain", "aurora", "gradient"} {
		t.Run(effect, func(t *testing.T) {
			summary := openRGBLightingWorkspaceSummaryFromSnapshot(devicesLightingSpeedSnapshot(effect, 2))
			if !summary.HasSpeedControl || summary.Speed != "2" {
				t.Fatalf("speed presentation for %q = %#v", effect, summary)
			}
		})
	}

	for _, test := range []struct {
		name     string
		snapshot openrgbimport.LightingSnapshot
	}{
		{name: "unsupported effect", snapshot: func() openrgbimport.LightingSnapshot {
			value := devicesLightingSpeedSnapshot("circle", 2)
			value.EffectSupported = false
			value.HasSpeed = false
			return value
		}()},
		{name: "missing speed", snapshot: func() openrgbimport.LightingSnapshot {
			value := devicesLightingSpeedSnapshot("circle", 2)
			value.HasSpeed = false
			return value
		}()},
	} {
		t.Run(test.name, func(t *testing.T) {
			if summary := openRGBLightingWorkspaceSummaryFromSnapshot(test.snapshot); summary.HasSpeedControl {
				t.Fatalf("%s produced a Speed control: %#v", test.name, summary)
			}
		})
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

func TestDevicesLightingBrightnessTemplate(t *testing.T) {
	if os.Getenv(devicesPageHelperEnvironment) == "brightness-slider" {
		initializeDevicesPageTestProcess(t)
		runDevicesLightingBrightnessTemplateAssertions(t)
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestDevicesLightingBrightnessTemplate$")
	command.Env = append(os.Environ(), devicesPageHelperEnvironment+"=brightness-slider")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Devices Lighting brightness slider helper process failed: %v\n%s", err, output)
	}
}

func TestDevicesLightingSpeedTemplate(t *testing.T) {
	if os.Getenv(devicesPageHelperEnvironment) == "speed-slider" {
		initializeDevicesPageTestProcess(t)
		runDevicesLightingSpeedTemplateAssertions(t)
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestDevicesLightingSpeedTemplate$")
	command.Env = append(os.Environ(), devicesPageHelperEnvironment+"=speed-slider")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Devices Lighting speed slider helper process failed: %v\n%s", err, output)
	}
}

func TestDevicesLightingColorResetTemplate(t *testing.T) {
	if os.Getenv(devicesPageHelperEnvironment) == "color-reset" {
		initializeDevicesPageTestProcess(t)
		runDevicesLightingColorResetTemplateAssertions(t)
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestDevicesLightingColorResetTemplate$")
	command.Env = append(os.Environ(), devicesPageHelperEnvironment+"=color-reset")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Devices Lighting color/reset helper process failed: %v\n%s", err, output)
	}
}

func runDevicesLightingColorResetTemplateAssertions(t *testing.T) {
	staticSnapshot := openrgbimport.LightingSnapshot{
		ConfiguredEffect: "static",
		EffectSupported:  true,
		PaletteKind:      string(rgb.LightingPaletteStaticSingle),
		SingleColorHex:   "#00ffff",
	}
	uncustomizedBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(staticSnapshot))
	for _, expected := range []string{
		`id="lf-lighting-color-input"`,
		`type="color"`,
		`value="#00ffff"`,
		`data-lf-current-color="#00ffff"`,
		`id="lf-lighting-color-hex"`,
		`data-lf-reset-control`,
		`data-lf-device-serial="lighting-template-device"`,
		`data-lf-effect="static"`,
	} {
		if !strings.Contains(uncustomizedBody, expected) {
			t.Errorf("uncustomized Static template does not contain %q", expected)
		}
	}
	resetStart := strings.Index(uncustomizedBody, `class="lf-reset-control"`)
	if resetStart < 0 {
		t.Fatal("uncustomized Static template does not render the revealable Reset container")
	}
	resetEnd := strings.Index(uncustomizedBody[resetStart:], ">")
	if resetEnd < 0 || !strings.Contains(uncustomizedBody[resetStart:resetStart+resetEnd], "hidden") {
		t.Error("uncustomized Static Reset container is not initially hidden")
	}

	staticSnapshot.Customized = true
	customizedBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(staticSnapshot))
	resetStart = strings.Index(customizedBody, `class="lf-reset-control"`)
	if resetStart < 0 {
		t.Fatal("customized Static template does not render Reset")
	}
	resetEnd = strings.Index(customizedBody[resetStart:], ">")
	if resetEnd < 0 || strings.Contains(customizedBody[resetStart:resetStart+resetEnd], "hidden") {
		t.Error("customized Static template keeps Reset hidden")
	}
	if !strings.Contains(customizedBody, `data-lf-reset-button`) || !strings.Contains(customizedBody, `Reset to default`) {
		t.Error("customized Static template does not render the Reset button")
	}

	staticSnapshot.ClusterControlled = true
	clusterBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(staticSnapshot))
	for _, id := range []string{"lf-lighting-color-input", "lf-lighting-color-hex"} {
		inputStart := strings.Index(clusterBody, `id="`+id+`"`)
		if inputStart < 0 {
			t.Errorf("cluster-owned Static color control %s is absent", id)
			continue
		}
		inputEnd := strings.Index(clusterBody[inputStart:], ">")
		if inputEnd < 0 || !strings.Contains(clusterBody[inputStart:inputStart+inputEnd], "disabled") {
			t.Errorf("cluster-owned Static color control %s is active", id)
		}
	}
	if strings.Contains(clusterBody, `data-lf-reset-control`) || strings.Contains(clusterBody, `data-lf-reset-button`) {
		t.Error("cluster-owned Static template exposes local Reset controls")
	}

	unsupportedBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		ConfiguredEffect: "aurora",
		EffectSupported:  true,
		PaletteKind:      string(rgb.LightingPaletteGenerated),
		SingleColorHex:   "#00ffff",
	}))
	if strings.Contains(unsupportedBody, `data-lf-color-input`) || strings.Contains(unsupportedBody, `data-lf-color-hex`) {
		t.Error("non-single-color effect rendered the color editor")
	}

	twoColorSnapshot := openrgbimport.LightingSnapshot{
		ConfiguredEffect: "wave",
		EffectSupported:  true,
		HasSpeed:         true,
		Speed:            5,
		PaletteKind:      string(rgb.LightingPaletteTwoColor),
		TwoColorStartHex: "#418fe8",
		TwoColorEndHex:   "#828282",
	}
	twoColorBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(twoColorSnapshot))
	for _, expected := range []string{
		`data-lf-two-color-control`,
		`data-lf-current-start="#418fe8"`,
		`data-lf-current-end="#828282"`,
		`for="lf-lighting-start-color-input">Start</label>`,
		`id="lf-lighting-start-color-input"`,
		`id="lf-lighting-start-color-hex"`,
		`for="lf-lighting-end-color-input">End</label>`,
		`id="lf-lighting-end-color-input"`,
		`id="lf-lighting-end-color-hex"`,
		`id="lf-lighting-two-color-status" aria-live="polite"`,
		`data-lf-speed-control`,
	} {
		if !strings.Contains(twoColorBody, expected) {
			t.Errorf("uncustomized two-color template does not contain %q", expected)
		}
	}
	for _, id := range []string{
		"lf-lighting-start-color-input",
		"lf-lighting-start-color-hex",
		"lf-lighting-end-color-input",
		"lf-lighting-end-color-hex",
		"lf-lighting-two-color-status",
	} {
		if count := strings.Count(twoColorBody, ` id="`+id+`"`); count != 1 {
			t.Errorf("two-color template ID %q count = %d, want 1", id, count)
		}
	}
	resetStart = strings.Index(twoColorBody, `class="lf-reset-control"`)
	if resetStart < 0 {
		t.Fatal("uncustomized two-color Reset container is absent")
	}
	resetEnd = strings.Index(twoColorBody[resetStart:], ">")
	if resetEnd < 0 || !strings.Contains(twoColorBody[resetStart:resetStart+resetEnd], "hidden") {
		t.Error("uncustomized two-color Reset container is not hidden")
	}

	twoColorSnapshot.Customized = true
	customizedTwoColorBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(twoColorSnapshot))
	resetStart = strings.Index(customizedTwoColorBody, `class="lf-reset-control"`)
	if resetStart < 0 {
		t.Fatal("customized two-color Reset container is absent")
	}
	resetEnd = strings.Index(customizedTwoColorBody[resetStart:], ">")
	if resetEnd < 0 || strings.Contains(customizedTwoColorBody[resetStart:resetStart+resetEnd], "hidden") {
		t.Error("customized two-color Reset is not visible")
	}

	twoColorSnapshot.ClusterControlled = true
	clusterTwoColorBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(twoColorSnapshot))
	for _, id := range []string{
		"lf-lighting-start-color-input",
		"lf-lighting-start-color-hex",
		"lf-lighting-end-color-input",
		"lf-lighting-end-color-hex",
	} {
		inputStart := strings.Index(clusterTwoColorBody, ` id="`+id+`"`)
		if inputStart < 0 {
			t.Errorf("cluster-owned two-color control %s is absent", id)
			continue
		}
		inputEnd := strings.Index(clusterTwoColorBody[inputStart:], ">")
		if inputEnd < 0 || !strings.Contains(clusterTwoColorBody[inputStart:inputStart+inputEnd], "disabled") {
			t.Errorf("cluster-owned two-color control %s is not disabled", id)
		}
	}
	if strings.Contains(clusterTwoColorBody, `data-lf-reset-control`) {
		t.Error("cluster-owned two-color template exposes local Reset")
	}

	for _, test := range []struct {
		effect string
		high   float64
	}{
		{effect: "cpu-temperature", high: 95},
		{effect: "gpu-temperature", high: 80},
	} {
		temperatureSnapshot := openrgbimport.LightingSnapshot{
			ConfiguredEffect:  test.effect,
			EffectSupported:   true,
			PaletteKind:       string(rgb.LightingPaletteTemperatureThree),
			HasTemperature:    true,
			TemperatureLow:    openrgbimport.LightingTemperaturePointSnapshot{ColorHex: "#00ff00", Celsius: 20},
			TemperatureMiddle: openrgbimport.LightingTemperaturePointSnapshot{ColorHex: "#ffff00", Celsius: 50},
			TemperatureHigh:   openrgbimport.LightingTemperaturePointSnapshot{ColorHex: "#ff0000", Celsius: test.high},
		}
		body := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(temperatureSnapshot))
		for _, expected := range []string{
			`data-lf-temperature-control`, `value="#00ff00"`, `value="#ffff00"`, `value="#ff0000"`,
			`value="` + fmt.Sprint(test.high) + `"`, `step="any"`, `Low temperature threshold in degrees Celsius`,
			`Middle temperature threshold in degrees Celsius`, `High temperature threshold in degrees Celsius`,
		} {
			if !strings.Contains(body, expected) {
				t.Errorf("%s temperature template does not contain %q", test.effect, expected)
			}
		}
		for _, role := range []string{"low", "middle", "high"} {
			for _, suffix := range []string{"color", "hex", "celsius"} {
				id := "lf-lighting-temperature-" + role + "-" + suffix
				if count := strings.Count(body, ` id="`+id+`"`); count != 1 {
					t.Errorf("temperature template ID %q count = %d, want 1", id, count)
				}
			}
		}
		if strings.Contains(body, `data-lf-speed-control`) || strings.Contains(body, "MinTemp") || strings.Contains(body, "MaxTemp") {
			t.Errorf("%s rendered unsupported temperature controls", test.effect)
		}
		uncustomizedResetStart := strings.Index(body, `class="lf-reset-control"`)
		if uncustomizedResetStart < 0 {
			t.Errorf("%s uncustomized temperature Reset container is absent", test.effect)
		} else if uncustomizedResetEnd := strings.Index(body[uncustomizedResetStart:], ">"); uncustomizedResetEnd < 0 ||
			!strings.Contains(body[uncustomizedResetStart:uncustomizedResetStart+uncustomizedResetEnd], "hidden") {
			t.Errorf("%s uncustomized temperature Reset is not hidden", test.effect)
		}
		temperatureSnapshot.Customized = true
		customizedBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(temperatureSnapshot))
		resetStart := strings.Index(customizedBody, `class="lf-reset-control"`)
		if resetStart < 0 {
			t.Errorf("%s customized temperature Reset is not visible", test.effect)
		} else if resetEnd := strings.Index(customizedBody[resetStart:], ">"); resetEnd < 0 ||
			strings.Contains(customizedBody[resetStart:resetStart+resetEnd], "hidden") {
			t.Errorf("%s customized temperature Reset is not visible", test.effect)
		}
	}

	clusterTemperature := openrgbimport.LightingSnapshot{
		ConfiguredEffect: "cpu-temperature", EffectSupported: true,
		PaletteKind: string(rgb.LightingPaletteTemperatureThree), HasTemperature: true, ClusterControlled: true,
		TemperatureLow:    openrgbimport.LightingTemperaturePointSnapshot{ColorHex: "#00ff00", Celsius: 20},
		TemperatureMiddle: openrgbimport.LightingTemperaturePointSnapshot{ColorHex: "#ffff00", Celsius: 50},
		TemperatureHigh:   openrgbimport.LightingTemperaturePointSnapshot{ColorHex: "#ff0000", Celsius: 95},
	}
	clusterTemperatureBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(clusterTemperature))
	if strings.Count(clusterTemperatureBody, " disabled") < 9 || strings.Contains(clusterTemperatureBody, `data-lf-reset-control`) {
		t.Error("cluster-owned temperature editor is active or exposes Reset")
	}

	gradientSnapshot := openrgbimport.LightingSnapshot{
		ConfiguredEffect: "gradient", EffectSupported: true, HasBrightness: true, Brightness: 60,
		HasSpeed: true, Speed: 10, PaletteKind: string(rgb.LightingPaletteGradient), HasGradient: true,
		GradientStops: []openrgbimport.LightingGradientStopSnapshot{
			{Position: 0, ColorHex: "#ff0000", Intensity: 1},
			{Position: 0.25, ColorHex: "#00ff00", Intensity: 1},
			{Position: 0.5, ColorHex: "#0000ff", Intensity: 1},
			{Position: 0.75, ColorHex: "#ffff00", Intensity: 1},
		},
	}
	gradientBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(gradientSnapshot))
	for _, expected := range []string{
		`data-lf-gradient-control`, `id="lf-lighting-gradient-stops"`, `Gradient stops`, `Add stop`, `Save Gradient`,
		`Position uses 0 for the start and 1 for the end`, `Intensity is relative to device Brightness`,
		`data-lf-speed-control`, `data-lf-brightness-slider`, `Palette`,
		`value="#ff0000"`, `value="#00ff00"`, `value="#0000ff"`, `value="#ffff00"`,
		`value="0.25"`, `value="0.5"`, `value="0.75"`, `data-lf-gradient-remove`,
	} {
		if !strings.Contains(gradientBody, expected) {
			t.Errorf("Gradient template does not contain %q", expected)
		}
	}
	for number := 1; number <= 4; number++ {
		for _, field := range []string{"color", "hex", "position", "intensity"} {
			id := fmt.Sprintf("lf-lighting-gradient-%s-%d", field, number)
			if count := strings.Count(gradientBody, ` id="`+id+`"`); count != 1 {
				t.Errorf("Gradient input ID %q count = %d, want 1", id, count)
			}
		}
		if !strings.Contains(gradientBody, fmt.Sprintf(`aria-label="Remove stop %d"`, number)) {
			t.Errorf("Gradient stop %d Remove label is absent", number)
		}
	}
	gradientResetStart := strings.Index(gradientBody, `class="lf-reset-control"`)
	if gradientResetStart < 0 {
		t.Error("uncustomized Gradient Reset is not hidden")
	} else if gradientResetEnd := strings.Index(gradientBody[gradientResetStart:], ">"); gradientResetEnd < 0 ||
		!strings.Contains(gradientBody[gradientResetStart:gradientResetStart+gradientResetEnd], "hidden") {
		t.Error("uncustomized Gradient Reset is not hidden")
	}
	gradientSnapshot.Customized = true
	customGradientBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(gradientSnapshot))
	gradientResetStart = strings.Index(customGradientBody, `class="lf-reset-control"`)
	if gradientResetStart < 0 {
		t.Error("customized Gradient Reset is not visible")
	} else if gradientResetEnd := strings.Index(customGradientBody[gradientResetStart:], ">"); gradientResetEnd < 0 ||
		strings.Contains(customGradientBody[gradientResetStart:gradientResetStart+gradientResetEnd], "hidden") {
		t.Error("customized Gradient Reset is not visible")
	}

	twoStopGradient := gradientSnapshot
	twoStopGradient.GradientStops = gradientSnapshot.GradientStops[:2]
	twoStopBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(twoStopGradient))
	for _, remove := range strings.Split(twoStopBody, `data-lf-gradient-remove`)[1:] {
		if end := strings.Index(remove, ">"); end < 0 || !strings.Contains(remove[:end], "disabled") {
			t.Error("two-stop Gradient Remove is enabled")
		}
	}

	gradientSnapshot.ClusterControlled = true
	clusterGradientBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(gradientSnapshot))
	if strings.Count(clusterGradientBody, " disabled") < 19 || strings.Contains(clusterGradientBody, `data-lf-reset-control`) {
		t.Error("cluster-owned Gradient controls are active or expose Reset")
	}
	for _, palette := range []rgb.LightingPaletteKind{
		rgb.LightingPaletteStaticSingle, rgb.LightingPaletteTwoColor, rgb.LightingPaletteTemperatureThree,
		rgb.LightingPaletteGenerated, rgb.LightingPaletteNone,
	} {
		body := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
			ConfiguredEffect: "other", EffectSupported: true, PaletteKind: string(palette), HasGradient: true,
			GradientStops: gradientSnapshot.GradientStops,
		}))
		if strings.Contains(body, `data-lf-gradient-control`) {
			t.Errorf("palette %q rendered Gradient controls", palette)
		}
	}

	for _, palette := range []rgb.LightingPaletteKind{
		rgb.LightingPaletteStaticSingle,
		rgb.LightingPaletteGenerated,
		rgb.LightingPaletteTemperatureThree,
		rgb.LightingPaletteGradient,
		rgb.LightingPaletteNone,
		"unsupported",
	} {
		body := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
			ConfiguredEffect: "other",
			EffectSupported:  true,
			PaletteKind:      string(palette),
			TwoColorStartHex: "#112233",
			TwoColorEndHex:   "#445566",
		}))
		if strings.Contains(body, `data-lf-two-color-control`) {
			t.Errorf("palette %q rendered the two-color editor", palette)
		}
	}
}

func runDevicesLightingSpeedTemplateAssertions(t *testing.T) {
	for _, effect := range []string{"circle", "flame", "cyberpunkglitch", "rain", "aurora", "gradient"} {
		snapshot := devicesLightingSpeedSnapshot(effect, 2)
		body := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(snapshot))
		for _, expected := range []string{
			`data-lf-speed-control`,
			`for="lf-lighting-speed-slider">Speed</label>`,
			`id="lf-lighting-speed-number"`,
			`type="number"`,
			`min="1"`,
			`max="10"`,
			`step="0.1"`,
			`aria-label="Speed level"`,
			`id="lf-lighting-speed-slider"`,
			`type="range"`,
			`data-lf-current-stored-speed="2"`,
			`data-lf-effect="` + effect + `"`,
			`data-lf-speed-control-mode="software"`,
			`data-lf-number-id="lf-lighting-speed-number"`,
			`data-lf-status-id="lf-lighting-speed-status"`,
			`<span>Slow</span><span>Fast</span>`,
			`id="lf-lighting-speed-status" aria-live="polite"`,
		} {
			if !strings.Contains(body, expected) {
				t.Errorf("%s Speed template does not contain %q", effect, expected)
			}
		}
		for _, forbidden := range []string{
			"Effective speed",
			"Persistent speed",
			"Stored animation speed",
			"Release to save",
			"Speed saved.",
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s Speed template contains duplicate or permanent copy %q", effect, forbidden)
			}
		}
	}

	for _, effect := range []string{"static", "off", "cpu-temperature", "gpu-temperature"} {
		body := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(devicesLightingSpeedSnapshot(effect, 2)))
		if strings.Contains(body, "data-lf-speed-slider") || strings.Contains(body, "data-lf-speed-number") || strings.Contains(body, "Speed / Unavailable") {
			t.Errorf("%s rendered an unavailable or interactive Speed control", effect)
		}
	}

	clusterSnapshot := devicesLightingSpeedSnapshot("rain", 2)
	clusterSnapshot.ClusterControlled = true
	clusterBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(clusterSnapshot))
	for _, id := range []string{"lf-lighting-speed-slider", "lf-lighting-speed-number"} {
		inputStart := strings.Index(clusterBody, `id="`+id+`"`)
		if inputStart < 0 {
			t.Errorf("cluster-owned Speed control %s is absent", id)
			continue
		}
		inputEnd := strings.Index(clusterBody[inputStart:], ">")
		if inputEnd < 0 || !strings.Contains(clusterBody[inputStart:inputStart+inputEnd], "disabled") {
			t.Errorf("cluster-owned Speed control %s is not disabled", id)
		}
	}
	for _, expected := range []string{
		`aria-describedby="lf-lighting-speed-status lf-lighting-speed-cluster-explanation"`,
		`id="lf-lighting-speed-cluster-explanation"`,
		`href="/rgbCluster"`,
	} {
		if !strings.Contains(clusterBody, expected) {
			t.Errorf("cluster-owned Speed template does not contain %q", expected)
		}
	}
}

func runDevicesLightingBrightnessTemplateAssertions(t *testing.T) {
	for _, brightness := range []uint8{0, 100} {
		body := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
			HasBrightness: true,
			Brightness:    brightness,
		}))
		value := fmt.Sprintf("%d", brightness)
		for _, expected := range []string{
			`<label class="lf-range-control-label" for="lf-lighting-brightness-slider">Brightness</label>`,
			`id="lf-lighting-brightness-number"`,
			`type="number"`,
			`data-lf-brightness-number`,
			`aria-label="Brightness percentage"`,
			`<span class="lf-range-control-suffix" aria-hidden="true">%</span>`,
			`id="lf-lighting-brightness-slider"`,
			`type="range"`,
			`min="0"`,
			`max="100"`,
			`step="1"`,
			`value="` + value + `"`,
			`style="--lf-range-progress: ` + value + `%;"`,
			`data-lf-current-brightness="` + value + `"`,
			`aria-valuetext="` + value + ` percent"`,
			`id="lf-lighting-brightness-status" aria-live="polite"`,
			`data-lf-brightness-readout data-lf-device-serial="lighting-template-device">` + value + `%</strong>`,
		} {
			if !strings.Contains(body, expected) {
				t.Errorf("brightness %s template does not contain %q", value, expected)
			}
		}
		if strings.Contains(body, `<small>Brightness</small>`) {
			t.Errorf("brightness %s template retained the duplicate primary metric", value)
		}
		for _, forbidden := range []string{
			"Stored local output level for this device.",
			"Changes are saved when the value is committed.",
			"Brightness saved.",
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("brightness %s template contains permanent status or helper copy %q", value, forbidden)
			}
		}
	}

	escapedBody := renderDevicesLightingViewForSerial(t, `lighting"&<serial`, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		HasBrightness: true,
		Brightness:    50,
	}))
	if !strings.Contains(escapedBody, `data-lf-device-serial="lighting&#34;&amp;&lt;serial"`) ||
		strings.Contains(escapedBody, `data-lf-device-serial="lighting"&<serial"`) {
		t.Error("brightness control did not contextually escape the device serial data attribute")
	}

	unavailableBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{}))
	if !strings.Contains(unavailableBody, "lf-range-control-unavailable") ||
		!strings.Contains(unavailableBody, `>Unavailable</strong>`) ||
		strings.Contains(unavailableBody, "data-lf-brightness-slider") ||
		strings.Contains(unavailableBody, "data-lf-brightness-number") ||
		strings.Contains(unavailableBody, `type="range"`) || strings.Contains(unavailableBody, `type="number"`) {
		t.Error("missing brightness snapshot did not render the non-interactive unavailable state")
	}
	for _, forbidden := range []string{"Stored local output level for this device.", "Brightness unavailable for this stored lighting profile."} {
		if strings.Contains(unavailableBody, forbidden) {
			t.Errorf("unavailable brightness state retained permanent helper copy %q", forbidden)
		}
	}

	clusterBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		HasBrightness:     true,
		Brightness:        40,
		ClusterControlled: true,
	}))
	for _, id := range []string{"lf-lighting-brightness-slider", "lf-lighting-brightness-number"} {
		inputStart := strings.Index(clusterBody, `id="`+id+`"`)
		if inputStart < 0 {
			t.Fatalf("cluster-owned Lighting view does not render %s", id)
		}
		inputEnd := strings.Index(clusterBody[inputStart:], ">")
		if inputEnd < 0 || !strings.Contains(clusterBody[inputStart:inputStart+inputEnd], "disabled") {
			t.Errorf("cluster-owned brightness control %s is not disabled", id)
		}
	}
	for _, expected := range []string{
		`aria-describedby="lf-lighting-brightness-status lf-lighting-brightness-cluster-explanation"`,
		`id="lf-lighting-brightness-cluster-explanation"`,
		`href="/rgbCluster"`,
		"RGB Cluster currently owns this device's lighting output.",
	} {
		if !strings.Contains(clusterBody, expected) {
			t.Errorf("cluster-owned brightness template does not contain %q", expected)
		}
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
		`<span class="lf-lighting-effect-icon-frame" aria-hidden="true">`,
		`class="lf-lighting-effect-icon-art" style="--lf-lighting-effect-mask: url('/static/img/icons/rgb/wave.svg');"`,
		`<strong class="lf-lighting-effect-name">Wave &lt;Bright&gt; &amp; Wide</strong>`,
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
	selectorStart := strings.Index(normalBody, `<select`)
	selectorEnd := strings.Index(normalBody, `</select>`)
	if selectorStart < 0 || selectorEnd < selectorStart {
		t.Fatal("normal effect selector markup is incomplete")
	}
	for _, forbidden := range []string{"<img", "<span", "style=", "mask"} {
		if strings.Contains(normalBody[selectorStart:selectorEnd], forbidden) {
			t.Errorf("native effect selector contains non-text option presentation %q", forbidden)
		}
	}
	if strings.Index(normalBody, `<option value="off">Off</option>`) > strings.Index(normalBody, `value="wave" selected>`) {
		t.Error("Off is not alphabetized before Wave in the rendered effect selector")
	}
	if strings.Contains(normalBody, "Controlled by RGB Cluster") || strings.Contains(normalBody, `id="lf-lighting-effect-cluster-explanation"`) {
		t.Error("non-clustered effect selector renders the cluster ownership explanation")
	}
	if strings.Contains(normalBody, "ZgotmplZ") || strings.Contains(normalBody, `<img class="lf-lighting-effect`) {
		t.Error("known effect icon did not render as a safely escaped CSS mask")
	}
	poisonedLabelBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		ConfiguredEffect: "wave",
		EffectSupported:  true,
		SupportedEffects: []openrgbimport.LightingEffectOption{{ID: "wave", Label: `Wave "');background:url(/label.svg);\\`}},
	}))
	iconTagStart := strings.Index(poisonedLabelBody, `<span class="lf-lighting-effect-icon-art"`)
	if iconTagStart < 0 {
		t.Fatal("effect with CSS-like label does not render its canonical icon")
	}
	iconTagEnd := strings.Index(poisonedLabelBody[iconTagStart:], ">")
	if iconTagEnd < 0 {
		t.Fatal("effect with CSS-like label renders an incomplete icon tag")
	}
	wantIconTag := `<span class="lf-lighting-effect-icon-art" style="--lf-lighting-effect-mask: url('/static/img/icons/rgb/wave.svg');">`
	if iconTag := poisonedLabelBody[iconTagStart : iconTagStart+iconTagEnd+1]; iconTag != wantIconTag {
		t.Errorf("CSS-like effect label influenced icon tag: got %q, want %q", iconTag, wantIconTag)
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

	maliciousEffectID := `legacy');background:url(/escaped.svg);\\<effect>`
	unsupportedBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		ConfiguredEffect: maliciousEffectID,
		EffectSupported:  false,
		SupportedEffects: []openrgbimport.LightingEffectOption{{ID: "static", Label: "Static & Safe"}},
	}))
	for _, expected := range []string{
		`<option value="static">Static &amp; Safe</option>`,
		`class="lf-lighting-effect-icon-fallback"`,
	} {
		if !strings.Contains(unsupportedBody, expected) {
			t.Errorf("unsupported configured effect selector does not contain %q", expected)
		}
	}
	if strings.Contains(unsupportedBody, maliciousEffectID) || strings.Contains(unsupportedBody, "Static & Safe") {
		t.Error("effect selector rendered an unescaped stable ID or display label")
	}
	if strings.Contains(unsupportedBody, `--lf-lighting-effect-mask:`) {
		t.Error("unsupported configured effect influenced a CSS mask URL")
	}

	knownUnsupportedBody := renderDevicesLightingView(t, openRGBLightingWorkspaceSummaryFromSnapshot(openrgbimport.LightingSnapshot{
		ConfiguredEffect: "wave",
		EffectSupported:  false,
		SupportedEffects: []openrgbimport.LightingEffectOption{{ID: "static", Label: "Static"}},
	}))
	if !strings.Contains(knownUnsupportedBody, `class="lf-lighting-effect-icon-fallback"`) ||
		strings.Contains(knownUnsupportedBody, `/static/img/icons/rgb/wave.svg`) {
		t.Error("known but unsupported effect did not retain the generic fallback")
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
		`/static/img/icons/rgb/static.svg`,
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
	selectorStart = strings.Index(clusterBody, `id="lf-lighting-effect-selector"`)
	if selectorStart < 0 {
		t.Fatal("cluster-controlled Lighting view does not render the effect selector")
	}
	selectorEnd = strings.Index(clusterBody[selectorStart:], ">")
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
	return renderDevicesLightingViewForSerial(t, "lighting-template-device", lighting)
}

func renderDevicesLightingViewForSerial(t *testing.T, serial string, lighting *openRGBLightingWorkspaceSummary) string {
	t.Helper()
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
		"/static/img/icons/rgb/static.svg",
		"data-lf-cluster-controlled=\"true\"",
		"id=\"lf-lighting-effect-selector\"",
		"<option value=\"off\">Off</option>",
		"value=\"static\" selected>Static</option>",
		"aria-describedby=\"lf-lighting-effect-status lf-lighting-effect-cluster-explanation\"",
		"Controlled by RGB Cluster. Change active lighting from the <a href=\"/rgbCluster\">RGB Cluster workspace</a>.",
		"id=\"lf-lighting-effect-status\" aria-live=\"polite\"",
		"lf-lighting-status-supported\">Supported",
		`data-lf-brightness-readout data-lf-device-serial="` + visibleSerial + `">0%</strong>`,
		`id="lf-lighting-brightness-slider"`,
		`type="range"`,
		`value="0"`,
		`style="--lf-range-progress: 0%;"`,
		`data-lf-current-brightness="0"`,
		`id="lf-lighting-brightness-status" aria-live="polite"`,
		"RGB Cluster currently owns this device's lighting output.",
	} {
		if !strings.Contains(lightingBody, expected) {
			t.Errorf("Lighting GET /devices response does not contain %q", expected)
		}
	}
	for _, excluded := range []string{
		"class=\"lf-overview-workspace\"",
		"<form",
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
		"</div>div>",
	} {
		if strings.Contains(strings.ToLower(lightingBody), strings.ToLower(excluded)) {
			t.Errorf("Lighting GET /devices response unexpectedly contains %q", excluded)
		}
	}
	if strings.Contains(lightingBody, `--lf-lighting-effect-mask: url('`+visibleSerial) {
		t.Error("device serial influenced the configured effect mask URL")
	}
	queryPayload := `url('/query.svg');\\`
	lightingWithUnrelatedQuery := requestDevicesPage(t, router, "device="+url.QueryEscape(visibleSerial)+"&view=lighting&foo="+url.QueryEscape(queryPayload))
	if lightingWithUnrelatedQuery.Code != http.StatusOK ||
		!strings.Contains(lightingWithUnrelatedQuery.Body.String(), `/static/img/icons/rgb/static.svg`) ||
		strings.Contains(lightingWithUnrelatedQuery.Body.String(), queryPayload) {
		t.Error("unrelated query value influenced the configured effect mask presentation")
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

}
