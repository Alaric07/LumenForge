package rgb

import "testing"

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

	if capability, ok := LightingEffectCapabilities("future-effect"); ok || capability != (LightingEffectCapability{}) {
		t.Fatalf("unknown effect was classified: %+v, %t", capability, ok)
	}
}
