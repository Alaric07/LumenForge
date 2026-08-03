package rgb

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestInterpolateTemperatureColorThreePointContract(t *testing.T) {
	low := Color{Red: 0, Green: 0, Blue: 0, Temperature: 10}
	middle := Color{Red: 100, Green: 200, Blue: 50, Temperature: 40}
	high := Color{Red: 255, Green: 255, Blue: 255, Temperature: 100}

	if !validThreePointTemperatureThresholds(&low, &middle, &high) {
		t.Fatal("valid non-symmetric Low/Middle/High thresholds were rejected")
	}

	tests := []struct {
		name        string
		temperature float64
		want        Color
	}{
		{name: "below Low", temperature: -20, want: Color{}},
		{name: "exact Low", temperature: 10, want: Color{}},
		{name: "between Low and Middle", temperature: 25, want: Color{Red: 50, Green: 100, Blue: 25}},
		{name: "exact Middle", temperature: 40, want: Color{Red: 100, Green: 200, Blue: 50}},
		{name: "between Middle and High", temperature: 70, want: Color{Red: 177, Green: 227, Blue: 152}},
		{name: "exact High", temperature: 100, want: Color{Red: 255, Green: 255, Blue: 255}},
		{name: "above High", temperature: 140, want: Color{Red: 255, Green: 255, Blue: 255}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := interpolateTemperatureColor(&low, &middle, &high, test.temperature, 1)
			if got == nil || *got != test.want {
				t.Fatalf("interpolateTemperatureColor(%v) = %#v, want %#v", test.temperature, got, test.want)
			}
		})
	}
}

func TestInterpolateTemperatureColorMalformedThreePointFallback(t *testing.T) {
	baseLow := Color{Red: 12, Green: 34, Blue: 56, Temperature: 20}
	baseMiddle := Color{Red: 80, Green: 100, Blue: 120, Temperature: 50}
	baseHigh := Color{Red: 210, Green: 220, Blue: 230, Temperature: 80}
	want := Color{Red: 12, Green: 34, Blue: 56}

	tests := []struct {
		name   string
		low    float64
		middle float64
		high   float64
	}{
		{name: "equal Low and Middle", low: 20, middle: 20, high: 80},
		{name: "equal Middle and High", low: 20, middle: 80, high: 80},
		{name: "reversed Low and Middle", low: 50, middle: 20, high: 80},
		{name: "reversed Middle and High", low: 20, middle: 80, high: 50},
		{name: "NaN Low", low: math.NaN(), middle: 50, high: 80},
		{name: "NaN Middle", low: 20, middle: math.NaN(), high: 80},
		{name: "NaN High", low: 20, middle: 50, high: math.NaN()},
		{name: "positive infinity Low", low: math.Inf(1), middle: 50, high: 80},
		{name: "positive infinity Middle", low: 20, middle: math.Inf(1), high: 80},
		{name: "positive infinity High", low: 20, middle: 50, high: math.Inf(1)},
		{name: "negative infinity Low", low: math.Inf(-1), middle: 50, high: 80},
		{name: "negative infinity Middle", low: 20, middle: math.Inf(-1), high: 80},
		{name: "negative infinity High", low: 20, middle: 50, high: math.Inf(-1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			low := baseLow
			middle := baseMiddle
			high := baseHigh
			low.Temperature = test.low
			middle.Temperature = test.middle
			high.Temperature = test.high

			if validThreePointTemperatureThresholds(&low, &middle, &high) {
				t.Fatal("malformed Low/Middle/High thresholds were accepted")
			}

			got := interpolateThreePointTemperatureColor(&low, &middle, &high, 60, 1)
			if got == nil || *got != want {
				t.Fatalf("malformed threshold fallback = %#v, want Low color %#v", got, want)
			}
			if math.IsNaN(got.Red) || math.IsNaN(got.Green) || math.IsNaN(got.Blue) ||
				math.IsInf(got.Red, 0) || math.IsInf(got.Green, 0) || math.IsInf(got.Blue, 0) {
				t.Fatalf("malformed threshold fallback contains a non-finite component: %#v", got)
			}
		})
	}
}

func TestInterpolateTemperatureColorMissingPointSafety(t *testing.T) {
	low := Color{Red: 10, Green: 20, Blue: 30, Temperature: 20}
	middle := Color{Red: 40, Green: 50, Blue: 60, Temperature: 50}
	high := Color{Red: 70, Green: 80, Blue: 90, Temperature: 80}

	if validThreePointTemperatureThresholds(nil, &middle, &high) ||
		validThreePointTemperatureThresholds(&low, nil, &high) ||
		validThreePointTemperatureThresholds(&low, &middle, nil) {
		t.Fatal("a missing Low/Middle/High point was accepted")
	}

	tests := []struct {
		name   string
		low    *Color
		middle *Color
		high   *Color
		want   Color
	}{
		{name: "missing Low", middle: &middle, high: &high, want: Color{}},
		{name: "missing Middle", low: &low, high: &high, want: Color{Red: 10, Green: 20, Blue: 30}},
		{name: "missing High", low: &low, middle: &middle, want: Color{Red: 10, Green: 20, Blue: 30}},
		{name: "only Low", low: &low, want: Color{Red: 10, Green: 20, Blue: 30}},
		{name: "no points", want: Color{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := interpolateThreePointTemperatureColor(test.low, test.middle, test.high, 50, 1)
			if got == nil || *got != test.want {
				t.Fatalf("missing-point fallback = %#v, want %#v", got, test.want)
			}
			if math.IsNaN(got.Red) || math.IsNaN(got.Green) || math.IsNaN(got.Blue) ||
				math.IsInf(got.Red, 0) || math.IsInf(got.Green, 0) || math.IsInf(got.Blue, 0) {
				t.Fatalf("missing-point fallback contains a non-finite component: %#v", got)
			}
		})
	}
}

func TestInterpolateTemperatureColorLegacyTwoPointPathUnchanged(t *testing.T) {
	low := Color{Red: 10, Green: 20, Blue: 30, Temperature: 20}
	high := Color{Red: 70, Green: 80, Blue: 90, Temperature: 80}
	want := Color{Red: 40, Green: 50, Blue: 60}

	if got := interpolateTemperatureColor(&low, nil, &high, 50, 1); got == nil || *got != want {
		t.Fatalf("legacy two-point interpolation = %#v, want %#v", got, want)
	}
}

func TestInterpolateTemperatureColorDoesNotMutateInputs(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
	}{
		{
			name: "valid",
			profile: Profile{
				StartColor:  Color{Red: 1, Green: 2, Blue: 3, Brightness: 0.4, Temperature: 20, Position: 0.1, Hex: "010203"},
				MiddleColor: Color{Red: 4, Green: 5, Blue: 6, Brightness: 0.5, Temperature: 50, Position: 0.5, Hex: "040506"},
				EndColor:    Color{Red: 7, Green: 8, Blue: 9, Brightness: 0.6, Temperature: 80, Position: 0.9, Hex: "070809"},
			},
		},
		{
			name: "malformed",
			profile: Profile{
				StartColor:  Color{Red: 1, Green: 2, Blue: 3, Brightness: 0.4, Temperature: 50, Position: 0.1, Hex: "010203"},
				MiddleColor: Color{Red: 4, Green: 5, Blue: 6, Brightness: 0.5, Temperature: 20, Position: 0.5, Hex: "040506"},
				EndColor:    Color{Red: 7, Green: 8, Blue: 9, Brightness: 0.6, Temperature: 80, Position: 0.9, Hex: "070809"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := test.profile
			before := profile
			_ = validThreePointTemperatureThresholds(&profile.StartColor, &profile.MiddleColor, &profile.EndColor)
			_ = interpolateThreePointTemperatureColor(&profile.StartColor, &profile.MiddleColor, &profile.EndColor, 60, 0.75)
			if !reflect.DeepEqual(profile, before) {
				t.Fatalf("temperature rendering mutated source profile:\nbefore: %#v\nafter:  %#v", before, profile)
			}
		})
	}
}

func TestShippedTemperatureProfilesSatisfyContractAndRenderUnchanged(t *testing.T) {
	data, err := os.ReadFile("../../database/rgb.json")
	if err != nil {
		t.Fatal(err)
	}
	var definitions RGB
	if err = json.Unmarshal(data, &definitions); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		id            string
		low           float64
		middle        float64
		high          float64
		temperature   float64
		wantSingleLED []byte
	}{
		{
			id:            "cpu-temperature",
			low:           20,
			middle:        50,
			high:          95,
			temperature:   72.5,
			wantSingleLED: []byte{255, 127, 0},
		},
		{
			id:            "gpu-temperature",
			low:           20,
			middle:        50,
			high:          80,
			temperature:   65,
			wantSingleLED: []byte{255, 127, 0},
		},
	}

	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			profile, ok := definitions.Profiles[test.id]
			if !ok {
				t.Fatalf("shipped profile %q is missing", test.id)
			}
			if profile.StartColor.Temperature != test.low ||
				profile.MiddleColor.Temperature != test.middle ||
				profile.EndColor.Temperature != test.high {
				t.Fatalf(
					"shipped thresholds = (%v, %v, %v), want (%v, %v, %v)",
					profile.StartColor.Temperature,
					profile.MiddleColor.Temperature,
					profile.EndColor.Temperature,
					test.low,
					test.middle,
					test.high,
				)
			}
			if profile.StartColor != (Color{Red: 0, Green: 255, Blue: 0, Brightness: 1, Temperature: test.low}) ||
				profile.MiddleColor != (Color{Red: 255, Green: 255, Blue: 0, Brightness: 1, Temperature: test.middle}) ||
				profile.EndColor != (Color{Red: 255, Green: 0, Blue: 0, Brightness: 1, Temperature: test.high}) {
				t.Fatalf("shipped temperature colors changed: %#v", profile)
			}
			if !validThreePointTemperatureThresholds(&profile.StartColor, &profile.MiddleColor, &profile.EndColor) {
				t.Fatal("shipped profile violates the Low/Middle/High threshold contract")
			}
			profileBefore := profile

			runner := New(
				1,
				profile.Speed,
				&profile.StartColor,
				&profile.EndColor,
				profile.Brightness,
				profile.Smoothness,
				0,
				false,
			)
			runner.RGBMiddleColor = &profile.MiddleColor
			runner.MinTemp = -1000
			runner.MaxTemp = -999
			runner.Temperature(test.temperature)
			if len(runner.Output) != len(test.wantSingleLED) {
				t.Fatalf("rendered frame length = %d, want %d", len(runner.Output), len(test.wantSingleLED))
			}
			for index, want := range test.wantSingleLED {
				if runner.Output[index] != want {
					t.Fatalf("rendered frame = %v, want %v", runner.Output, test.wantSingleLED)
				}
			}
			if !reflect.DeepEqual(profile, profileBefore) {
				t.Fatalf("rendering mutated shipped profile copy:\nbefore: %#v\nafter:  %#v", profileBefore, profile)
			}
		})
	}
}
