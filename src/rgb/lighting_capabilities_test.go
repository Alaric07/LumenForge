package rgb

import (
	"reflect"
	"testing"
)

func TestLightingEffectCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		effects []string
		palette LightingPaletteKind
		start   bool
		middle  bool
		end     bool
		speed   bool
	}{
		{name: "off", effects: []string{"off"}, palette: LightingPaletteNone},
		{name: "static", effects: []string{"static"}, palette: LightingPaletteStaticSingle, start: true},
		{name: "single color animation", effects: []string{"rotator"}, palette: LightingPaletteStaticSingle, start: true, speed: true},
		{
			name:    "two color",
			effects: []string{"circle", "circleshift", "colorpulse", "colorshift", "flickering", "spinner", "storm", "wave"},
			palette: LightingPaletteTwoColor,
			start:   true,
			end:     true,
			speed:   true,
		},
		{
			name:    "temperature",
			effects: []string{"cpu-temperature", "gpu-temperature"},
			palette: LightingPaletteTemperatureThree,
			start:   true,
			middle:  true,
			end:     true,
		},
		{name: "gradient", effects: []string{"gradient"}, palette: LightingPaletteGradient, speed: true},
		{
			name:    "generated",
			effects: []string{"aurora", "colorwarp", "cyberpunkglitch", "flame", "pastelrainbow", "pastelspiralrainbow", "rainbow", "spiralrainbow", "watercolor"},
			palette: LightingPaletteGenerated,
			speed:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, effect := range tt.effects {
				capability, ok := LightingEffectCapabilities(effect)
				if !ok {
					t.Fatalf("LightingEffectCapabilities(%q) was not classified", effect)
				}
				if capability.Palette != tt.palette ||
					capability.UsesStartColor != tt.start ||
					capability.UsesMiddleColor != tt.middle ||
					capability.UsesEndColor != tt.end ||
					capability.SupportsSpeed != tt.speed {
					t.Errorf("LightingEffectCapabilities(%q) = %+v", effect, capability)
				}
				if capability.SupportsSpeed != HasSpeedControl(effect) {
					t.Errorf("LightingEffectCapabilities(%q).SupportsSpeed disagrees with HasSpeedControl", effect)
				}
			}
		})
	}

	unknownEffects := []string{
		"",
		"future-effect",
		"ARC",
		" arc",
		"arc ",
		"colorwave",
		"led",
		"liquid-temperature",
		"probe-temperature",
		"rainbowwave",
		"tlk",
		"tlr",
	}
	for _, effect := range unknownEffects {
		if capability, ok := LightingEffectCapabilities(effect); ok || capability != (LightingEffectCapability{}) {
			t.Errorf("LightingEffectCapabilities(%q) classified unknown effect: %+v, %t", effect, capability, ok)
		}
	}
}

func TestLightingEffectCapabilitiesNewGenericCoverage(t *testing.T) {
	tests := []struct {
		name    string
		effects []string
		want    LightingEffectCapability
	}{
		{
			name:    "two color",
			effects: []string{"arc", "comet", "datastream", "marquee", "plasmacore", "rain", "rotarystack", "sequential", "stardust"},
			want: LightingEffectCapability{
				Palette:        LightingPaletteTwoColor,
				UsesStartColor: true,
				UsesEndColor:   true,
				SupportsSpeed:  true,
			},
		},
		{
			name:    "generated",
			effects: []string{"nebula", "tokyonight"},
			want: LightingEffectCapability{
				Palette:       LightingPaletteGenerated,
				SupportsSpeed: true,
			},
		},
		{
			name:    "single start color",
			effects: []string{"visor"},
			want: LightingEffectCapability{
				Palette:        LightingPaletteStaticSingle,
				UsesStartColor: true,
				SupportsSpeed:  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, effect := range tt.effects {
				got, ok := LightingEffectCapabilities(effect)
				if !ok {
					t.Fatalf("LightingEffectCapabilities(%q) was not classified", effect)
				}
				if got != tt.want {
					t.Errorf("LightingEffectCapabilities(%q) = %+v, want %+v", effect, got, tt.want)
				}
			}
		})
	}
}

func TestLightingEffectCapabilitiesDescriptorParity(t *testing.T) {
	descriptorsBefore := SoftwareEffectDescriptors()

	for _, descriptor := range descriptorsBefore {
		capability, ok := LightingEffectCapabilities(descriptor.ID)
		if !ok {
			t.Fatalf("LightingEffectCapabilities(%q) was not classified", descriptor.ID)
		}

		want := LightingEffectCapability{
			Palette:         descriptor.PaletteKind,
			UsesStartColor:  descriptor.UsesStart,
			UsesMiddleColor: descriptor.UsesMiddle,
			UsesEndColor:    descriptor.UsesEnd,
			SupportsSpeed:   HasSpeedControl(descriptor.ID),
		}
		if capability != want {
			t.Errorf("LightingEffectCapabilities(%q) = %+v, want descriptor-derived %+v", descriptor.ID, capability, want)
		}
	}

	if descriptorsAfter := SoftwareEffectDescriptors(); !reflect.DeepEqual(descriptorsAfter, descriptorsBefore) {
		t.Fatalf("LightingEffectCapabilities mutated the canonical descriptor registry:\nbefore: %#v\nafter:  %#v", descriptorsBefore, descriptorsAfter)
	}
}
