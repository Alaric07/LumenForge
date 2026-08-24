package lightingsettings

import (
	"reflect"
	"testing"

	"LumenForge/src/rgb"
)

func TestRendererProfileFromEffectSettings(t *testing.T) {
	speed := 4.5
	tests := []struct {
		name     string
		settings EffectSettings
		want     rgb.Profile
	}{
		{
			name:     "generated palette and Speed",
			settings: EffectSettings{SchemaVersion: SchemaVersion, EffectID: "rainbow", Speed: &speed},
			want:     rgb.Profile{ProfileName: "rainbow", Brightness: 1, Speed: speed},
		},
		{
			name: "single color",
			settings: EffectSettings{
				SchemaVersion: SchemaVersion,
				EffectID:      "static",
				SingleColor:   &SingleColorSettings{Color: Color{Red: 1, Green: 2, Blue: 3}},
			},
			want: rgb.Profile{
				ProfileName: "static",
				Brightness:  1,
				Smoothness:  20,
				StartColor:  rgb.Color{Red: 1, Green: 2, Blue: 3, Brightness: 1},
			},
		},
		{
			name: "two color",
			settings: EffectSettings{
				SchemaVersion: SchemaVersion,
				EffectID:      "wave",
				Speed:         &speed,
				TwoColor: &TwoColorSettings{
					Start: Color{Red: 4, Green: 5, Blue: 6},
					End:   Color{Red: 7, Green: 8, Blue: 9},
				},
			},
			want: rgb.Profile{
				ProfileName: "wave",
				Brightness:  1,
				Smoothness:  10,
				Speed:       speed,
				StartColor:  rgb.Color{Red: 4, Green: 5, Blue: 6, Brightness: 1},
				EndColor:    rgb.Color{Red: 7, Green: 8, Blue: 9, Brightness: 1},
			},
		},
		{
			name: "temperature",
			settings: EffectSettings{
				SchemaVersion: SchemaVersion,
				EffectID:      "cpu-temperature",
				Temperature: &TemperatureSettings{
					Low:    TemperaturePoint{Color: Color{Red: 10, Green: 11, Blue: 12}, Celsius: 20},
					Middle: TemperaturePoint{Color: Color{Red: 13, Green: 14, Blue: 15}, Celsius: 50},
					High:   TemperaturePoint{Color: Color{Red: 16, Green: 17, Blue: 18}, Celsius: 95},
				},
			},
			want: rgb.Profile{
				ProfileName: "cpu-temperature",
				Brightness:  1,
				Smoothness:  40,
				StartColor:  rgb.Color{Red: 10, Green: 11, Blue: 12, Brightness: 1, Temperature: 20},
				MiddleColor: rgb.Color{Red: 13, Green: 14, Blue: 15, Brightness: 1, Temperature: 50},
				EndColor:    rgb.Color{Red: 16, Green: 17, Blue: 18, Brightness: 1, Temperature: 95},
				MinTemp:     20,
				MaxTemp:     95,
			},
		},
		{
			name: "Gradient",
			settings: EffectSettings{
				SchemaVersion: SchemaVersion,
				EffectID:      "gradient",
				Speed:         &speed,
				Gradient: &GradientSettings{Stops: []GradientStop{
					{Position: 0.2, Color: Color{Red: 19, Green: 20, Blue: 21}, Intensity: 0.4},
					{Position: 0.8, Color: Color{Red: 22, Green: 23, Blue: 24}, Intensity: 0.9},
				}},
			},
			want: rgb.Profile{
				ProfileName: "gradient",
				Brightness:  1,
				Speed:       speed,
				Gradients: map[int]rgb.Color{
					0: {Red: 19, Green: 20, Blue: 21, Brightness: 0.4, Position: 0.2},
					1: {Red: 22, Green: 23, Blue: 24, Brightness: 0.9, Position: 0.8},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := RendererProfileFromEffectSettings(test.settings)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("renderer profile = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestRendererProfileFromEffectSettingsUsesLegacyRendererSmoothness(t *testing.T) {
	tests := map[string]int{
		"wave":        10,
		"colorpulse":  40,
		"colorshift":  50,
		"circleshift": 100,
	}
	for effectID, want := range tests {
		profile := RendererProfileFromEffectSettings(EffectSettings{EffectID: effectID})
		if profile.Smoothness != want {
			t.Errorf("%s renderer Smoothness = %d, want %d", effectID, profile.Smoothness, want)
		}
	}
}

func TestRendererProfileFromEffectSettingsDoesNotAliasGradientInput(t *testing.T) {
	settings := EffectSettings{
		SchemaVersion: SchemaVersion,
		EffectID:      "gradient",
		Gradient: &GradientSettings{Stops: []GradientStop{
			{Position: 0, Color: Color{Red: 1}, Intensity: 0.5},
			{Position: 1, Color: Color{Blue: 2}, Intensity: 1},
		}},
	}
	profile := RendererProfileFromEffectSettings(settings)
	profile.Gradients[0] = rgb.Color{Red: 99}
	if settings.Gradient.Stops[0].Color.Red != 1 || settings.Gradient.Stops[0].Intensity != 0.5 {
		t.Fatalf("renderer profile mutation changed settings: %#v", settings)
	}
}
