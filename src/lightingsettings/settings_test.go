package lightingsettings

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func testSpeed(value float64) *float64 {
	return &value
}

func testColor(red, green, blue float64) Color {
	return Color{Red: red, Green: green, Blue: blue}
}

func testStaticSettings(red float64) EffectSettings {
	return EffectSettings{
		SchemaVersion: SchemaVersion,
		EffectID:      "static",
		SingleColor:   &SingleColorSettings{Color: testColor(red, 20, 30)},
	}
}

func testWaveSettings(speed float64) EffectSettings {
	return EffectSettings{
		SchemaVersion: SchemaVersion,
		EffectID:      "wave",
		Speed:         testSpeed(speed),
		TwoColor: &TwoColorSettings{
			Start: testColor(10, 20, 30),
			End:   testColor(40, 50, 60),
		},
	}
}

func testGradientSettings(speed float64) EffectSettings {
	return EffectSettings{
		SchemaVersion: SchemaVersion,
		EffectID:      "gradient",
		Speed:         testSpeed(speed),
		Gradient: &GradientSettings{Stops: []GradientStop{
			{Position: 0, Color: testColor(255, 0, 0), Intensity: 0.5},
			{Position: 1, Color: testColor(0, 0, 255), Intensity: 1},
		}},
	}
}

func testTemperatureSettings(effect string) EffectSettings {
	return EffectSettings{
		SchemaVersion: SchemaVersion,
		EffectID:      effect,
		Temperature: &TemperatureSettings{
			Low:    TemperaturePoint{Color: testColor(0, 255, 0), Celsius: 20},
			Middle: TemperaturePoint{Color: testColor(255, 255, 0), Celsius: 50},
			High:   TemperaturePoint{Color: testColor(255, 0, 0), Celsius: 80},
		},
	}
}

func TestValidateCompleteEffectSettings(t *testing.T) {
	valid := []EffectSettings{
		{SchemaVersion: SchemaVersion, EffectID: "off"},
		{SchemaVersion: SchemaVersion, EffectID: "rainbow", Speed: testSpeed(4)},
		testStaticSettings(10),
		testWaveSettings(5),
		testTemperatureSettings("gpu-temperature"),
		testGradientSettings(5),
	}
	for _, settings := range valid {
		if err := Validate(settings); err != nil {
			t.Errorf("Validate(%q) error = %v", settings.EffectID, err)
		}
	}
}

func TestValidateRejectsIncompleteAndContradictorySettings(t *testing.T) {
	tests := []struct {
		name     string
		settings EffectSettings
		want     error
	}{
		{name: "schema", settings: EffectSettings{EffectID: "off"}, want: ErrInvalidSettings},
		{name: "unknown", settings: EffectSettings{SchemaVersion: SchemaVersion, EffectID: "unknown"}, want: ErrUnknownEffect},
		{name: "missing Speed", settings: EffectSettings{SchemaVersion: SchemaVersion, EffectID: "rainbow"}, want: ErrInvalidSettings},
		{name: "unexpected Speed", settings: func() EffectSettings {
			value := testStaticSettings(1)
			value.Speed = testSpeed(1)
			return value
		}(), want: ErrInvalidSettings},
		{name: "non-finite Speed", settings: EffectSettings{SchemaVersion: SchemaVersion, EffectID: "rainbow", Speed: testSpeed(math.NaN())}, want: ErrInvalidSettings},
		{name: "Speed range", settings: testWaveSettings(0.5), want: ErrInvalidSettings},
		{name: "missing single color", settings: EffectSettings{SchemaVersion: SchemaVersion, EffectID: "static"}, want: ErrInvalidSettings},
		{name: "extra variant", settings: func() EffectSettings {
			value := testWaveSettings(5)
			value.SingleColor = &SingleColorSettings{}
			return value
		}(), want: ErrInvalidSettings},
		{name: "channel range", settings: testStaticSettings(256), want: ErrInvalidSettings},
		{name: "non-finite channel", settings: testStaticSettings(math.Inf(1)), want: ErrInvalidSettings},
		{name: "incomplete temperature", settings: EffectSettings{SchemaVersion: SchemaVersion, EffectID: "gpu-temperature"}, want: ErrInvalidSettings},
		{name: "unordered temperature", settings: func() EffectSettings {
			value := testTemperatureSettings("gpu-temperature")
			value.Temperature.Middle.Celsius = 10
			return value
		}(), want: ErrInvalidSettings},
		{name: "non-finite temperature", settings: func() EffectSettings {
			value := testTemperatureSettings("gpu-temperature")
			value.Temperature.High.Celsius = math.Inf(1)
			return value
		}(), want: ErrInvalidSettings},
		{name: "too few Gradient stops", settings: func() EffectSettings {
			value := testGradientSettings(5)
			value.Gradient.Stops = value.Gradient.Stops[:1]
			return value
		}(), want: ErrInvalidSettings},
		{name: "unordered Gradient", settings: func() EffectSettings {
			value := testGradientSettings(5)
			value.Gradient.Stops[0].Position = 0.8
			value.Gradient.Stops[1].Position = 0.2
			return value
		}(), want: ErrInvalidSettings},
		{name: "Gradient position", settings: func() EffectSettings {
			value := testGradientSettings(5)
			value.Gradient.Stops[1].Position = 1.1
			return value
		}(), want: ErrInvalidSettings},
		{name: "Gradient intensity", settings: func() EffectSettings {
			value := testGradientSettings(5)
			value.Gradient.Stops[1].Intensity = -0.1
			return value
		}(), want: ErrInvalidSettings},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Validate(test.settings)
			if !errors.Is(err, test.want) {
				t.Fatalf("Validate() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestEffectSettingsCloneCopiesMutableNestedValues(t *testing.T) {
	original := testGradientSettings(5)
	cloned := original.Clone()
	*cloned.Speed = 8
	cloned.Gradient.Stops[0].Color.Red = 1
	cloned.Gradient.Stops = append(cloned.Gradient.Stops, GradientStop{})
	if *original.Speed != 5 || original.Gradient.Stops[0].Color.Red != 255 || len(original.Gradient.Stops) != 2 {
		t.Fatalf("Clone() aliased original: original=%#v clone=%#v", original, cloned)
	}
}

func TestEffectSettingsDoesNotOwnBrightness(t *testing.T) {
	for _, value := range []any{
		EffectSettings{},
		Color{},
		SingleColorSettings{},
		TwoColorSettings{},
		TemperaturePoint{},
		TemperatureSettings{},
		GradientStop{},
		GradientSettings{},
	} {
		typeOfValue := reflect.TypeOf(value)
		for index := 0; index < typeOfValue.NumField(); index++ {
			field := typeOfValue.Field(index)
			if strings.Contains(strings.ToLower(field.Name), "brightness") || strings.Contains(strings.ToLower(field.Tag.Get("json")), "brightness") {
				t.Fatalf("%s unexpectedly owns Brightness through field %s", typeOfValue.Name(), field.Name)
			}
		}
	}
}
