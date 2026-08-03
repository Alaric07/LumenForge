package rgb

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

var expectedSoftwareEffectLabels = map[string]string{
	"arc":                 "Arc",
	"aurora":              "Aurora",
	"circle":              "Circle",
	"circleshift":         "Circle Shift",
	"colorpulse":          "Color Pulse",
	"colorshift":          "Color Shift",
	"colorwarp":           "Color Warp",
	"comet":               "Comet",
	"cpu-temperature":     "CPU Temperature",
	"cyberpunkglitch":     "Cyberpunk Glitch",
	"datastream":          "Data Stream",
	"flame":               "Flame",
	"flickering":          "Flickering",
	"gpu-temperature":     "GPU Temperature",
	"gradient":            "Gradient",
	"marquee":             "Marquee",
	"nebula":              "Nebula",
	"off":                 "Off",
	"pastelrainbow":       "Pastel Rainbow",
	"pastelspiralrainbow": "Pastel Spiral Rainbow",
	"plasmacore":          "Plasma Core",
	"rain":                "Rain",
	"rainbow":             "Rainbow",
	"rotarystack":         "Rotary Stack",
	"rotator":             "Rotator",
	"sequential":          "Sequential",
	"spinner":             "Spinner",
	"spiralrainbow":       "Spiral Rainbow",
	"stardust":            "Stardust",
	"static":              "Static",
	"storm":               "Storm",
	"tokyonight":          "Tokyo Night",
	"visor":               "Visor",
	"watercolor":          "Water Color",
	"wave":                "Wave",
}

func TestSoftwareEffectDescriptorInventory(t *testing.T) {
	descriptors := SoftwareEffectDescriptors()
	if len(descriptors) != 35 {
		t.Fatalf("descriptor count = %d, want 35", len(descriptors))
	}
	if len(expectedSoftwareEffectLabels) != 35 {
		t.Fatalf("test inventory count = %d, want 35", len(expectedSoftwareEffectLabels))
	}

	seen := make(map[string]struct{}, len(descriptors))
	for _, descriptor := range descriptors {
		if descriptor.ID == "" {
			t.Fatal("descriptor has an empty ID")
		}
		if descriptor.Label == "" {
			t.Fatalf("descriptor %q has an empty label", descriptor.ID)
		}
		if descriptor.Scope == 0 || descriptor.Scope != EffectScopeBoth {
			t.Fatalf("descriptor %q scope = %d, want EffectScopeBoth", descriptor.ID, descriptor.Scope)
		}
		if _, duplicate := seen[descriptor.ID]; duplicate {
			t.Fatalf("duplicate descriptor ID %q", descriptor.ID)
		}
		seen[descriptor.ID] = struct{}{}

		label, expected := expectedSoftwareEffectLabels[descriptor.ID]
		if !expected {
			t.Errorf("unexpected descriptor ID %q", descriptor.ID)
		} else if descriptor.Label != label {
			t.Errorf("descriptor %q label = %q, want %q", descriptor.ID, descriptor.Label, label)
		}
	}

	for id := range expectedSoftwareEffectLabels {
		if _, ok := seen[id]; !ok {
			t.Errorf("missing descriptor ID %q", id)
		}
	}

	for _, id := range []string{"colorwave", "led", "liquid-temperature", "probe-temperature", "rainbowwave", "tlk", "tlr"} {
		if descriptor, ok := SoftwareEffectDescriptorByID(id); ok {
			t.Errorf("device-specific ID %q was registered: %+v", id, descriptor)
		}
	}
}

func TestSoftwareEffectDescriptorLookup(t *testing.T) {
	for _, want := range SoftwareEffectDescriptors() {
		got, ok := SoftwareEffectDescriptorByID(want.ID)
		if !ok {
			t.Fatalf("SoftwareEffectDescriptorByID(%q) returned false", want.ID)
		}
		if got != want {
			t.Errorf("SoftwareEffectDescriptorByID(%q) = %+v, want %+v", want.ID, got, want)
		}
	}

	for _, id := range []string{"", "future-effect", "ARC"} {
		if descriptor, ok := SoftwareEffectDescriptorByID(id); ok || descriptor != (SoftwareEffectDescriptor{}) {
			t.Errorf("SoftwareEffectDescriptorByID(%q) = %+v, %t; want zero, false", id, descriptor, ok)
		}
	}

	descriptor, ok := SoftwareEffectDescriptorByID("arc")
	if !ok {
		t.Fatal("arc descriptor was not found")
	}
	descriptor.Label = "caller mutation"
	descriptor.Icon = "caller.svg"
	again, ok := SoftwareEffectDescriptorByID("arc")
	if !ok || again.Label != "Arc" || again.Icon != "arc.svg" {
		t.Fatalf("lookup mutation altered canonical descriptor: %+v, %t", again, ok)
	}
}

func TestSoftwareEffectDescriptorsOrderAndDefensiveCopy(t *testing.T) {
	descriptors := SoftwareEffectDescriptors()
	if !sort.SliceIsSorted(descriptors, func(i, j int) bool {
		left := strings.ToLower(descriptors[i].Label)
		right := strings.ToLower(descriptors[j].Label)
		if left == right {
			return descriptors[i].ID < descriptors[j].ID
		}
		return left < right
	}) {
		t.Fatal("descriptors are not ordered case-insensitively by display label with stable-ID tie-break")
	}

	again := SoftwareEffectDescriptors()
	if !reflect.DeepEqual(descriptors, again) {
		t.Fatal("successive descriptor reads are not deterministic")
	}
	descriptors[0].ID = "caller mutation"
	descriptors[0].Label = "caller mutation"
	third := SoftwareEffectDescriptors()
	if !reflect.DeepEqual(again, third) {
		t.Fatal("mutating a returned descriptor slice altered canonical state")
	}
}

func TestSoftwareEffectDescriptorMetadata(t *testing.T) {
	type paletteContract struct {
		ids                []string
		palette            LightingPaletteKind
		start, middle, end bool
		speed              bool
	}
	contracts := []paletteContract{
		{ids: []string{"off"}, palette: LightingPaletteNone},
		{ids: []string{"static"}, palette: LightingPaletteStaticSingle, start: true},
		{ids: []string{"rotator", "visor"}, palette: LightingPaletteStaticSingle, start: true, speed: true},
		{
			ids: []string{
				"arc", "circle", "circleshift", "colorpulse", "colorshift", "comet", "datastream",
				"flickering", "marquee", "plasmacore", "rain", "rotarystack", "sequential",
				"spinner", "stardust", "storm", "wave",
			},
			palette: LightingPaletteTwoColor,
			start:   true,
			end:     true,
			speed:   true,
		},
		{
			ids:     []string{"cpu-temperature", "gpu-temperature"},
			palette: LightingPaletteTemperatureThree,
			start:   true,
			middle:  true,
			end:     true,
		},
		{ids: []string{"gradient"}, palette: LightingPaletteGradient, speed: true},
		{
			ids: []string{
				"aurora", "colorwarp", "cyberpunkglitch", "flame", "nebula", "pastelrainbow",
				"pastelspiralrainbow", "rainbow", "spiralrainbow", "tokyonight", "watercolor",
			},
			palette: LightingPaletteGenerated,
			speed:   true,
		},
	}

	seen := make(map[string]struct{}, 35)
	for _, contract := range contracts {
		for _, id := range contract.ids {
			if _, duplicate := seen[id]; duplicate {
				t.Fatalf("metadata contract contains duplicate ID %q", id)
			}
			seen[id] = struct{}{}
			descriptor, ok := SoftwareEffectDescriptorByID(id)
			if !ok {
				t.Fatalf("metadata contract ID %q is not registered", id)
			}
			if descriptor.PaletteKind != contract.palette ||
				descriptor.UsesStart != contract.start ||
				descriptor.UsesMiddle != contract.middle ||
				descriptor.UsesEnd != contract.end ||
				descriptor.SupportsSpeed != contract.speed {
				t.Errorf("descriptor %q palette contract = %+v", id, descriptor)
			}
		}
	}
	if len(seen) != 35 {
		t.Fatalf("metadata contracts cover %d descriptors, want 35", len(seen))
	}

	anyTopology := map[string]struct{}{
		"colorpulse": {}, "colorshift": {}, "colorwarp": {}, "cpu-temperature": {},
		"gpu-temperature": {}, "gradient": {}, "off": {}, "static": {}, "storm": {},
	}
	for _, descriptor := range SoftwareEffectDescriptors() {
		wantTopology := SoftwareEffectTopologyLinear
		if _, ok := anyTopology[descriptor.ID]; ok {
			wantTopology = SoftwareEffectTopologyAny
		}
		if descriptor.Topology != wantTopology {
			t.Errorf("descriptor %q topology = %d, want %d", descriptor.ID, descriptor.Topology, wantTopology)
		}

		wantSensor := SoftwareEffectSensorNone
		switch descriptor.ID {
		case "cpu-temperature":
			wantSensor = SoftwareEffectSensorCPU
		case "gpu-temperature":
			wantSensor = SoftwareEffectSensorGPU
		}
		if descriptor.Sensor != wantSensor {
			t.Errorf("descriptor %q sensor = %d, want %d", descriptor.ID, descriptor.Sensor, wantSensor)
		}
		if descriptor.MinimumLEDs != 0 {
			t.Errorf("descriptor %q minimum LEDs = %d, want 0", descriptor.ID, descriptor.MinimumLEDs)
		}
		if descriptor.Icon != descriptor.ID+".svg" {
			t.Errorf("descriptor %q icon = %q, want %q", descriptor.ID, descriptor.Icon, descriptor.ID+".svg")
		}
		if descriptor.SupportsSpeed != HasSpeedControl(descriptor.ID) {
			t.Errorf("descriptor %q speed support disagrees with HasSpeedControl", descriptor.ID)
		}
	}
}

func TestSoftwareEffectDescriptorExistingCapabilityContracts(t *testing.T) {
	for _, descriptor := range SoftwareEffectDescriptors() {
		capability, known := LightingEffectCapabilities(descriptor.ID)
		if !known {
			continue
		}
		if capability.Palette != descriptor.PaletteKind ||
			capability.UsesStartColor != descriptor.UsesStart ||
			capability.UsesMiddleColor != descriptor.UsesMiddle ||
			capability.UsesEndColor != descriptor.UsesEnd ||
			capability.SupportsSpeed != descriptor.SupportsSpeed {
			t.Errorf("descriptor %q disagrees with existing capability: descriptor=%+v capability=%+v", descriptor.ID, descriptor, capability)
		}
	}
}

func TestSoftwareEffectTemperaturePointContracts(t *testing.T) {
	wantSensors := map[string]SoftwareEffectSensorRequirement{
		"cpu-temperature": SoftwareEffectSensorCPU,
		"gpu-temperature": SoftwareEffectSensorGPU,
	}

	for _, descriptor := range SoftwareEffectDescriptors() {
		if !validSoftwareEffectTemperaturePointContract(descriptor) {
			t.Errorf("descriptor %q violates production temperature-point invariants: %+v", descriptor.ID, descriptor)
		}

		wantSensor, temperatureEffect := wantSensors[descriptor.ID]
		if !temperatureEffect {
			if descriptor.TemperaturePoints != SoftwareEffectTemperaturePointsNone {
				t.Errorf("non-temperature descriptor %q declares temperature points %d", descriptor.ID, descriptor.TemperaturePoints)
			}
			continue
		}

		if descriptor.TemperaturePoints != SoftwareEffectTemperaturePointsLowMiddleHigh ||
			descriptor.PaletteKind != LightingPaletteTemperatureThree ||
			!descriptor.UsesStart || !descriptor.UsesMiddle || !descriptor.UsesEnd ||
			descriptor.SupportsSpeed || descriptor.Sensor != wantSensor {
			t.Errorf("temperature descriptor %q contract = %+v", descriptor.ID, descriptor)
		}

		capability, ok := LightingEffectCapabilities(descriptor.ID)
		if !ok || capability.Palette != LightingPaletteTemperatureThree ||
			!capability.UsesStartColor || !capability.UsesMiddleColor || !capability.UsesEndColor ||
			capability.SupportsSpeed || HasSpeedControl(descriptor.ID) {
			t.Errorf("temperature descriptor %q disagrees with existing capability metadata: %+v, %t", descriptor.ID, capability, ok)
		}
	}
}

func TestSoftwareEffectTemperaturePointContractRejectsIncoherentDescriptors(t *testing.T) {
	validTemperature := newTemperatureSoftwareEffectDescriptor("temperature-test", "Temperature Test", SoftwareEffectSensorCPU)
	nonTemperature, ok := SoftwareEffectDescriptorByID("static")
	if !ok {
		t.Fatal("static descriptor was not found")
	}

	tests := []struct {
		name       string
		descriptor SoftwareEffectDescriptor
		wantValid  bool
	}{
		{name: "valid temperature", descriptor: validTemperature, wantValid: true},
		{name: "valid non-temperature", descriptor: nonTemperature, wantValid: true},
		{name: "temperature missing contract", descriptor: func() SoftwareEffectDescriptor {
			value := validTemperature
			value.TemperaturePoints = SoftwareEffectTemperaturePointsNone
			return value
		}()},
		{name: "temperature missing Start role", descriptor: func() SoftwareEffectDescriptor {
			value := validTemperature
			value.UsesStart = false
			return value
		}()},
		{name: "temperature missing Middle role", descriptor: func() SoftwareEffectDescriptor {
			value := validTemperature
			value.UsesMiddle = false
			return value
		}()},
		{name: "temperature missing End role", descriptor: func() SoftwareEffectDescriptor {
			value := validTemperature
			value.UsesEnd = false
			return value
		}()},
		{name: "temperature supports Speed", descriptor: func() SoftwareEffectDescriptor {
			value := validTemperature
			value.SupportsSpeed = true
			return value
		}()},
		{name: "temperature missing sensor", descriptor: func() SoftwareEffectDescriptor {
			value := validTemperature
			value.Sensor = SoftwareEffectSensorNone
			return value
		}()},
		{name: "non-temperature declares contract", descriptor: func() SoftwareEffectDescriptor {
			value := nonTemperature
			value.TemperaturePoints = SoftwareEffectTemperaturePointsLowMiddleHigh
			return value
		}()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validSoftwareEffectTemperaturePointContract(test.descriptor); got != test.wantValid {
				t.Fatalf("validSoftwareEffectTemperaturePointContract(%+v) = %t, want %t", test.descriptor, got, test.wantValid)
			}
		})
	}
}

func TestEffectScopeIncludes(t *testing.T) {
	if !EffectScopeBoth.Includes(EffectScopeDevice) {
		t.Error("EffectScopeBoth does not include EffectScopeDevice")
	}
	if !EffectScopeBoth.Includes(EffectScopeCluster) {
		t.Error("EffectScopeBoth does not include EffectScopeCluster")
	}
	if EffectScopeDevice.Includes(EffectScopeCluster) {
		t.Error("EffectScopeDevice includes EffectScopeCluster")
	}
	if EffectScopeCluster.Includes(EffectScopeDevice) {
		t.Error("EffectScopeCluster includes EffectScopeDevice")
	}
	if EffectScope(0).Includes(EffectScopeDevice) || EffectScope(0).Includes(EffectScopeCluster) {
		t.Error("zero scope includes a configured target")
	}
	if EffectScopeBoth.Includes(0) {
		t.Error("EffectScopeBoth includes an invalid zero target")
	}
}
