package rgb

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGradientOwningBrightnessScalesIntensityAwareResultOnce(t *testing.T) {
	gradients := map[int]Color{
		0: {Red: 200, Brightness: 0.5, Position: 0},
		1: {Blue: 100, Brightness: 1, Position: 1},
	}
	original := cloneGradientColors(gradients)

	tests := []struct {
		name       string
		brightness float64
		want       Color
	}{
		{name: "maximum is identity", brightness: 1, want: Color{Red: 46, Blue: 15}},
		{name: "zero is black", brightness: 0, want: Color{}},
		{name: "half", brightness: 0.5, want: Color{Red: 23, Blue: 7}},
		{name: "rounding boundary", brightness: 0.333, want: Color{Red: 15, Blue: 4}},
		{name: "low value", brightness: 0.01, want: Color{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := gradientColorAtProgress(gradients, 0.25, tt.brightness)
			assertGradientColor(t, got, ok, tt.want)
		})
	}

	if !reflect.DeepEqual(gradients, original) {
		t.Fatalf("gradient data mutated:\n got: %#v\nwant: %#v", gradients, original)
	}

	fullWhite := map[int]Color{
		0: {Red: 255, Green: 255, Blue: 255, Brightness: 1, Position: 0},
		1: {Red: 255, Green: 255, Blue: 255, Brightness: 1, Position: 1},
	}
	got, ok := gradientColorAtProgress(fullWhite, 0.5, 0.5)
	assertGradientColor(t, got, ok, Color{Red: 127, Green: 127, Blue: 127})
}

func TestGradientStopPositionsAndIntensitiesRemainRelative(t *testing.T) {
	nonSymmetric := map[int]Color{
		0: {Red: 255, Brightness: 1, Position: 0.1},
		1: {Green: 255, Brightness: 1, Position: 0.6},
		2: {Blue: 255, Brightness: 1, Position: 0.9},
	}

	tests := []struct {
		name     string
		progress float64
		want     Color
	}{
		{name: "exact first stop", progress: 0.1, want: Color{Red: 255}},
		{name: "between non-symmetric stops", progress: 0.35, want: Color{Red: 127, Green: 127}},
		{name: "exact last stop", progress: 0.9, want: Color{Blue: 255}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := gradientColorAtProgress(nonSymmetric, tt.progress, 1)
			assertGradientColor(t, got, ok, tt.want)
		})
	}

	blackToWhite := map[int]Color{
		0: {Brightness: 1, Position: 0},
		1: {Red: 255, Green: 255, Blue: 255, Brightness: 1, Position: 1},
	}
	got, ok := gradientColorAtProgress(blackToWhite, 0.5, 1)
	assertGradientColor(t, got, ok, Color{Red: 127, Green: 127, Blue: 127})

	equalIntensity := map[int]Color{
		0: {Red: 255, Brightness: 0.5, Position: 0},
		1: {Blue: 255, Brightness: 0.5, Position: 1},
	}
	got, ok = gradientColorAtProgress(equalIntensity, 0.5, 1)
	assertGradientColor(t, got, ok, Color{Red: 31, Blue: 31})

	zeroIntensity := map[int]Color{
		0: {Red: 255, Green: 255, Blue: 255, Position: 0},
		1: {Red: 255, Green: 255, Blue: 255, Position: 1},
	}
	got, ok = gradientColorAtProgress(zeroIntensity, 0.5, 1)
	assertGradientColor(t, got, ok, Color{})

	maximumIntensity := map[int]Color{
		0: {Red: 255, Green: 255, Blue: 255, Brightness: 1, Position: 0},
		1: {Red: 255, Green: 255, Blue: 255, Brightness: 1, Position: 1},
	}
	got, ok = gradientColorAtProgress(maximumIntensity, 0.5, 1)
	assertGradientColor(t, got, ok, Color{Red: 255, Green: 255, Blue: 255})
}

func TestGradientWrapAroundBeforeFirstStop(t *testing.T) {
	gradients := map[int]Color{
		0: {Red: 200, Brightness: 0.5, Position: 0.25},
		1: {Blue: 100, Brightness: 1, Position: 0.75},
	}
	original := cloneGradientColors(gradients)

	fractionTests := []struct {
		name     string
		progress float64
		want     float64
	}{
		{name: "cycle boundary", progress: 0, want: 0.5},
		{name: "between boundary and first stop", progress: 0.125, want: 0.75},
		{name: "immediately below first stop", progress: 0.2499, want: 0.9998},
		{name: "immediately above last stop", progress: 0.7501, want: 0.0002},
	}
	for _, tt := range fractionTests {
		t.Run("fraction "+tt.name, func(t *testing.T) {
			got, ok := gradientWrapSegmentProgress(tt.progress, 0.75, 0.25)
			if !ok {
				t.Fatalf("gradientWrapSegmentProgress(%v) rejected", tt.progress)
			}
			if math.Abs(got-tt.want) > 1e-12 {
				t.Fatalf("gradientWrapSegmentProgress(%v) = %v, want %v", tt.progress, got, tt.want)
			}
		})
	}

	colorTests := []struct {
		name       string
		progress   float64
		brightness float64
		want       Color
	}{
		{name: "boundary maximum brightness", progress: 0, brightness: 1, want: Color{Red: 37, Blue: 37}},
		{name: "boundary intermediate brightness", progress: 0, brightness: 0.5, want: Color{Red: 18, Blue: 18}},
		{name: "boundary zero brightness", progress: 0, brightness: 0, want: Color{}},
		{name: "between boundary and first stop", progress: 0.125, brightness: 1, want: Color{Red: 46, Blue: 15}},
		{name: "immediately below first stop", progress: math.Nextafter(0.25, 0), brightness: 1, want: Color{Red: 50}},
		{name: "exact first stop", progress: 0.25, brightness: 1, want: Color{Red: 50}},
		{name: "exact last stop", progress: 0.75, brightness: 1, want: Color{Blue: 100}},
		{name: "immediately above last stop", progress: math.Nextafter(0.75, 1), brightness: 1, want: Color{Blue: 99}},
		{name: "immediately below cycle end", progress: math.Nextafter(1, 0), brightness: 1, want: Color{Red: 37, Blue: 37}},
	}
	for _, tt := range colorTests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := gradientColorAtProgress(gradients, tt.progress, tt.brightness)
			assertGradientColor(t, got, ok, tt.want)
			assertFiniteGradientColor(t, got)
		})
	}

	runner := &ActiveRGB{LightChannels: 2, RGBBrightness: 1}
	runner.colorshiftGradientAtProgress(gradients, 0)
	wantFrame := []byte{37, 0, 37, 37, 0, 37}
	if !reflect.DeepEqual(runner.Output, wantFrame) {
		t.Fatalf("wrap Gradient frame = %v, want %v", runner.Output, wantFrame)
	}

	if !reflect.DeepEqual(gradients, original) {
		t.Fatalf("wrap rendering mutated gradient stops:\n got: %#v\nwant: %#v", gradients, original)
	}
}

func TestGradientWrapAroundWithFirstStopAtZeroRemainsUnchanged(t *testing.T) {
	gradients := map[int]Color{
		0: {Red: 255, Brightness: 1, Position: 0},
		1: {Blue: 255, Brightness: 1, Position: 0.75},
	}

	first, ok := gradientColorAtProgress(gradients, 0, 1)
	assertGradientColor(t, first, ok, Color{Red: 255})
	aboveLast, ok := gradientColorAtProgress(gradients, 0.875, 1)
	assertGradientColor(t, aboveLast, ok, Color{Red: 127, Blue: 127})
}

func TestGradientChangingOneStopIntensityIsNotGlobalBrightness(t *testing.T) {
	base := map[int]Color{
		0: {Red: 200, Brightness: 1, Position: 0},
		1: {Blue: 200, Brightness: 1, Position: 1},
	}
	reducedStop := cloneGradientColors(base)
	color := reducedStop[0]
	color.Brightness = 0.5
	reducedStop[0] = color

	baseColor, ok := gradientColorAtProgress(base, 0.25, 1)
	assertGradientColor(t, baseColor, ok, Color{Red: 150, Blue: 50})
	reducedStopColor, ok := gradientColorAtProgress(reducedStop, 0.25, 1)
	assertGradientColor(t, reducedStopColor, ok, Color{Red: 46, Blue: 31})
	globalHalfColor, ok := gradientColorAtProgress(base, 0.25, 0.5)
	assertGradientColor(t, globalHalfColor, ok, Color{Red: 75, Blue: 25})

	if reducedStopColor == globalHalfColor {
		t.Fatal("one stop's relative intensity behaved like global brightness")
	}
}

func TestColorshiftGradientAtProgressBuildsFrameWithoutMutatingStops(t *testing.T) {
	gradients := map[int]Color{
		0: {Red: 255, Brightness: 1, Position: 0},
		1: {Blue: 255, Brightness: 1, Position: 1},
	}
	original := cloneGradientColors(gradients)
	runner := &ActiveRGB{LightChannels: 3, RGBBrightness: 0.5}

	runner.colorshiftGradientAtProgress(gradients, 0.5)

	want := []byte{63, 0, 63, 63, 0, 63, 63, 0, 63}
	if !reflect.DeepEqual(runner.Output, want) {
		t.Fatalf("Gradient frame = %v, want %v", runner.Output, want)
	}
	if !reflect.DeepEqual(gradients, original) {
		t.Fatalf("gradient stops mutated:\n got: %#v\nwant: %#v", gradients, original)
	}
}

func TestGradientStopOrderingRemainsSortedWithoutSourceMutation(t *testing.T) {
	gradients := map[int]Color{
		0: {Blue: 255, Brightness: 1, Position: 1},
		1: {Red: 255, Brightness: 1, Position: 0},
	}
	original := cloneGradientColors(gradients)

	got, ok := gradientColorAtProgress(gradients, 0, 1)
	assertGradientColor(t, got, ok, Color{Red: 255})

	if !reflect.DeepEqual(gradients, original) {
		t.Fatalf("unordered gradient stops mutated:\n got: %#v\nwant: %#v", gradients, original)
	}
}

func TestGradientMalformedInputBoundary(t *testing.T) {
	valid := map[int]Color{
		0: {Red: 255, Brightness: 1, Position: 0},
		1: {Blue: 255, Brightness: 1, Position: 1},
	}

	rejected := []struct {
		name      string
		gradients map[int]Color
		progress  float64
	}{
		{name: "no stops", gradients: nil, progress: 0.5},
		{name: "one stop", gradients: map[int]Color{0: valid[0]}, progress: 0.5},
		{name: "missing indexed stop", gradients: map[int]Color{0: valid[0], 2: valid[1]}, progress: 0.5},
		{name: "negative position", gradients: gradientWithPosition(-0.1), progress: 0.5},
		{name: "position above one", gradients: gradientWithPosition(1.1), progress: 0.5},
		{name: "NaN position", gradients: gradientWithPosition(math.NaN()), progress: 0.5},
		{name: "positive infinite position", gradients: gradientWithPosition(math.Inf(1)), progress: 0.5},
		{name: "negative infinite position", gradients: gradientWithPosition(math.Inf(-1)), progress: 0.5},
		{name: "NaN progress", gradients: valid, progress: math.NaN()},
		{name: "infinite progress", gradients: valid, progress: math.Inf(1)},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := gradientColorAtProgress(tt.gradients, tt.progress, 1); ok {
				t.Fatalf("gradientColorAtProgress() = %#v, true; want rejected input", got)
			}
		})
	}

	duplicatePositions := map[int]Color{
		0: {Red: 255, Brightness: 1, Position: 0},
		1: {Green: 255, Brightness: 1, Position: 0},
		2: {Blue: 255, Brightness: 1, Position: 1},
	}
	got, ok := gradientColorAtProgress(duplicatePositions, 0.5, 1)
	if !ok {
		t.Fatalf("duplicate-position Gradient was rejected: %#v", got)
	}
	assertFiniteGradientColor(t, got)
}

func TestGradientMalformedBrightnessAndChannelValuesAreBounded(t *testing.T) {
	intensityTests := []struct {
		name      string
		intensity float64
		want      Color
	}{
		{name: "NaN", intensity: math.NaN(), want: Color{}},
		{name: "negative", intensity: -1, want: Color{}},
		{name: "negative infinity", intensity: math.Inf(-1), want: Color{}},
		{name: "above range", intensity: 2, want: Color{Red: 255, Green: 255, Blue: 255}},
		{name: "positive infinity", intensity: math.Inf(1), want: Color{Red: 255, Green: 255, Blue: 255}},
	}
	for _, tt := range intensityTests {
		t.Run("stop intensity "+tt.name, func(t *testing.T) {
			gradients := map[int]Color{
				0: {Red: 255, Green: 255, Blue: 255, Brightness: tt.intensity, Position: 0},
				1: {Red: 255, Green: 255, Blue: 255, Brightness: tt.intensity, Position: 1},
			}
			got, ok := gradientColorAtProgress(gradients, 0.5, 1)
			assertGradientColor(t, got, ok, tt.want)
			assertFiniteGradientColor(t, got)
		})
	}

	brightnessTests := []struct {
		name       string
		brightness float64
		want       Color
	}{
		{name: "NaN", brightness: math.NaN(), want: Color{}},
		{name: "negative", brightness: -1, want: Color{}},
		{name: "negative infinity", brightness: math.Inf(-1), want: Color{}},
		{name: "above range", brightness: 2, want: Color{Red: 255, Green: 255, Blue: 255}},
		{name: "positive infinity", brightness: math.Inf(1), want: Color{Red: 255, Green: 255, Blue: 255}},
	}
	for _, tt := range brightnessTests {
		t.Run("owning brightness "+tt.name, func(t *testing.T) {
			gradients := map[int]Color{
				0: {Red: 255, Green: 255, Blue: 255, Brightness: 1, Position: 0},
				1: {Red: 255, Green: 255, Blue: 255, Brightness: 1, Position: 1},
			}
			got, ok := gradientColorAtProgress(gradients, 0.5, tt.brightness)
			assertGradientColor(t, got, ok, tt.want)
			assertFiniteGradientColor(t, got)
		})
	}

	channels := map[int]Color{
		0: {Red: math.NaN(), Green: math.Inf(1), Blue: math.Inf(-1), Brightness: 1, Position: 0},
		1: {Red: -1, Green: 300, Blue: 100, Brightness: 1, Position: 1},
	}
	got, ok := gradientColorAtProgress(channels, 0.5, 1)
	assertGradientColor(t, got, ok, Color{Green: 255, Blue: 50})
	assertFiniteGradientColor(t, got)
}

func TestGradientRejectedInputLeavesRendererStateUnchanged(t *testing.T) {
	runner := &ActiveRGB{
		LightChannels: 1,
		RGBBrightness: 1,
		Raw:           map[int][]byte{0: {4, 5, 6}},
		Output:        []byte{4, 5, 6},
	}
	wantRaw := map[int][]byte{0: {4, 5, 6}}
	wantOutput := []byte{4, 5, 6}

	runner.colorshiftGradientAtProgress(nil, 0.5)
	runner.colorshiftGradientAtProgress(map[int]Color{0: {Red: 255, Brightness: 1}}, 0.5)

	if !reflect.DeepEqual(runner.Raw, wantRaw) || !reflect.DeepEqual(runner.Output, wantOutput) {
		t.Fatalf("rejected Gradient changed renderer state: Raw=%v Output=%v", runner.Raw, runner.Output)
	}
}

func TestGradientProgressPreservesSpeedContract(t *testing.T) {
	tests := []struct {
		name     string
		elapsed  float64
		duration float64
		want     float64
	}{
		{name: "configured duration", elapsed: 2.5, duration: 5, want: 0.5},
		{name: "slower configured duration", elapsed: 2.5, duration: 10, want: 0.25},
		{name: "cycle wraps", elapsed: 6.25, duration: 5, want: 0.25},
		{name: "zero uses established default", elapsed: 2.5, duration: 0, want: 0.5},
		{name: "negative uses established default", elapsed: 2.5, duration: -1, want: 0.5},
		{name: "NaN uses safe default", elapsed: 2.5, duration: math.NaN(), want: 0.5},
		{name: "infinite duration remains stationary", elapsed: 2.5, duration: math.Inf(1), want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gradientProgress(tt.elapsed, tt.duration); got != tt.want {
				t.Fatalf("gradientProgress(%v, %v) = %v, want %v", tt.elapsed, tt.duration, got, tt.want)
			}
		})
	}
}

func TestShippedGradientProfileAndMaximumBrightnessOutputRemainUnchanged(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "database", "rgb.json"))
	if err != nil {
		t.Fatalf("read shipped RGB profiles: %v", err)
	}
	var shipped RGB
	if err := json.Unmarshal(data, &shipped); err != nil {
		t.Fatalf("decode shipped RGB profiles: %v", err)
	}
	profile, ok := shipped.Profiles["gradient"]
	if !ok {
		t.Fatal("shipped Gradient profile is missing")
	}
	wantStops := map[int]Color{
		0: {Red: 255, Brightness: 1, Position: 0},
		1: {Green: 255, Brightness: 1, Position: 0.33},
		2: {Blue: 255, Brightness: 1, Position: 0.66},
		3: {Red: 255, Green: 255, Brightness: 1, Position: 1},
	}
	if profile.Speed != 10 || profile.Brightness != 1 || !reflect.DeepEqual(profile.Gradients, wantStops) {
		t.Fatalf("shipped Gradient profile = %#v", profile)
	}
	original := cloneGradientColors(profile.Gradients)

	first, ok := gradientColorAtProgress(profile.Gradients, 0, 1)
	assertGradientColor(t, first, ok, Color{Red: 255})
	midpoint, ok := gradientColorAtProgress(profile.Gradients, 0.165, 1)
	assertGradientColor(t, midpoint, ok, Color{Red: 127, Green: 127})
	last, ok := gradientColorAtProgress(profile.Gradients, 1, 1)
	assertGradientColor(t, last, ok, Color{Red: 255, Green: 255})

	if !reflect.DeepEqual(profile.Gradients, original) {
		t.Fatalf("shipped Gradient profile copy mutated:\n got: %#v\nwant: %#v", profile.Gradients, original)
	}
}

func assertGradientColor(t *testing.T, got Color, ok bool, want Color) {
	t.Helper()
	if !ok || got.Red != want.Red || got.Green != want.Green || got.Blue != want.Blue {
		t.Fatalf("Gradient color = %#v, ok=%t; want %#v, true", got, ok, want)
	}
}

func assertFiniteGradientColor(t *testing.T, color Color) {
	t.Helper()
	for name, value := range map[string]float64{"red": color.Red, "green": color.Green, "blue": color.Blue} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 255 {
			t.Fatalf("Gradient %s channel = %v, want finite byte range", name, value)
		}
	}
}

func cloneGradientColors(gradients map[int]Color) map[int]Color {
	cloned := make(map[int]Color, len(gradients))
	for index, color := range gradients {
		cloned[index] = color
	}
	return cloned
}

func gradientWithPosition(position float64) map[int]Color {
	return map[int]Color{
		0: {Red: 255, Brightness: 1, Position: 0},
		1: {Blue: 255, Brightness: 1, Position: position},
	}
}
