package rgb

import (
	"bytes"
	"testing"
	"time"
)

func TestFlickeringRendererSmallLEDContract(t *testing.T) {
	startColor := Color{Red: 120, Green: 80, Blue: 40}
	endColor := Color{Red: 20, Green: 60, Blue: 100}
	speeds := []float64{0.1, 0.5, 0.99, 1, 4}
	ledCounts := []int{1, 2, 4, 8}

	for _, speed := range speeds {
		for _, ledCount := range ledCounts {
			renderer := New(ledCount, speed, &startColor, &endColor, 1, 0, 0, true)
			startTime := time.Now()

			renderer.Flickering(&startTime)

			if len(renderer.Output) != ledCount*3 {
				t.Fatalf("Flickering output length for %d LEDs at speed %v = %d, want %d", ledCount, speed, len(renderer.Output), ledCount*3)
			}
		}
	}
}

func TestFlickeringRandomUpperBoundContract(t *testing.T) {
	speeds := []float64{0.1, 0.5, 0.99, 1, 4}
	ledCounts := []int{1, 2, 4, 8}

	for _, speed := range speeds {
		for _, ledCount := range ledCounts {
			calculated := ledCount * int(speed)
			want := calculated
			if want < 2 {
				want = 2
			}

			got := flickeringRandomUpperBound(ledCount, speed)
			if got != want {
				t.Fatalf("Flickering random upper bound for %d LEDs at speed %v = %d, want %d (calculated %d)", ledCount, speed, got, want, calculated)
			}
			if got <= 1 {
				t.Fatalf("Flickering target 1 is unreachable with random upper bound %d for %d LEDs at speed %v", got, ledCount, speed)
			}
		}
	}
}

func TestFlickeringRendererZeroLEDContract(t *testing.T) {
	startColor := Color{Red: 120, Green: 80, Blue: 40}
	endColor := Color{Red: 20, Green: 60, Blue: 100}
	renderer := New(0, 0.1, &startColor, &endColor, 1, 0, 0, true)
	startTime := time.Now()

	renderer.Flickering(&startTime)

	if len(renderer.Output) != 0 {
		t.Fatalf("Flickering zero-LED output length = %d, want 0", len(renderer.Output))
	}
}

func TestVisorRendererSmallLEDContract(t *testing.T) {
	startColor := Color{Red: 101, Green: 53, Blue: 205}
	endColor := Color{Red: 11, Green: 17, Blue: 23}

	for _, ledCount := range []int{1, 2, 4, 8} {
		renderer := New(ledCount, 2, &startColor, &endColor, 0.5, 0, 0, true)
		startTime := time.Now()

		for update := 0; update < 3; update++ {
			renderer.Visor(&startTime)
			if len(renderer.Output) != ledCount*3 {
				t.Fatalf("Visor output length for %d LEDs after update %d = %d, want %d", ledCount, update, len(renderer.Output), ledCount*3)
			}
		}
	}
}

func TestVisorRendererOneLEDFallback(t *testing.T) {
	startColor := Color{Red: 101, Green: 53, Blue: 205}
	endColor := Color{Red: 11, Green: 17, Blue: 23}
	renderer := New(1, 2, &startColor, &endColor, 0.5, 0, 0, true)
	startTime := time.Now()

	renderer.Visor(&startTime)

	want := []byte{50, 26, 102}
	if !bytes.Equal(renderer.Output, want) {
		t.Fatalf("Visor one-LED output = %v, want brightness-scaled Start color %v", renderer.Output, want)
	}
}

func TestVisorRendererZeroLEDContract(t *testing.T) {
	startColor := Color{Red: 101, Green: 53, Blue: 205}
	endColor := Color{Red: 11, Green: 17, Blue: 23}
	renderer := New(0, 2, &startColor, &endColor, 0.5, 0, 0, true)
	startTime := time.Now()

	renderer.Visor(&startTime)

	if len(renderer.Output) != 0 {
		t.Fatalf("Visor zero-LED output length = %d, want 0", len(renderer.Output))
	}
}
