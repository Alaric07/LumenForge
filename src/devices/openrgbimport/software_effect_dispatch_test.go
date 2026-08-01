package openrgbimport

import (
	"LumenForge/src/rgb"
	"bytes"
	"context"
	"net"
	"slices"
	"testing"
	"time"
)

func newSoftwareEffectDispatchRunner(ledCount int) *rgb.ActiveRGB {
	start := &rgb.Color{
		Red:         12,
		Green:       34,
		Blue:        56,
		Brightness:  1,
		Temperature: 30,
	}
	end := &rgb.Color{
		Red:         210,
		Green:       120,
		Blue:        40,
		Brightness:  1,
		Temperature: 80,
	}
	runner := rgb.New(ledCount, 2, start, end, 1, 0, 0, true)
	runner.RGBMiddleColor = &rgb.Color{
		Red:         80,
		Green:       160,
		Blue:        220,
		Brightness:  1,
		Temperature: 55,
	}
	runner.MinTemp = 30
	runner.MaxTemp = 80
	return runner
}

func installSoftwareEffectResumeSeams(t *testing.T) (string, <-chan []byte) {
	t.Helper()

	previousProfileDir := deviceProfileDir
	previousPersistentFrame := sendLightingPersistentFrame
	profileDir := t.TempDir()
	frames := make(chan []byte, 1)
	deviceProfileDir = func() string { return profileDir }
	sendLightingPersistentFrame = func(conn net.Conn, _ uint32, frame []byte) (net.Conn, error) {
		select {
		case frames <- append([]byte(nil), frame...):
		default:
		}
		return conn, nil
	}
	t.Cleanup(func() {
		deviceProfileDir = previousProfileDir
		sendLightingPersistentFrame = previousPersistentFrame
	})
	return profileDir, frames
}

func TestOpenRGBSoftwareEffectDispatchMappingsAndFrameSafety(t *testing.T) {
	tests := []struct {
		effect   string
		ledCount int
	}{
		{effect: "arc", ledCount: 2},
		{effect: "comet", ledCount: 4},
		{effect: "datastream", ledCount: 8},
		{effect: "marquee", ledCount: 2},
		{effect: "nebula", ledCount: 4},
		{effect: "plasmacore", ledCount: 8},
		{effect: "rain", ledCount: 2},
		{effect: "rotarystack", ledCount: 4},
		{effect: "sequential", ledCount: 2},
		{effect: "stardust", ledCount: 4},
		{effect: "tokyonight", ledCount: 8},
		{effect: "visor", ledCount: 1},
	}

	for _, test := range tests {
		t.Run(test.effect, func(t *testing.T) {
			runner := newSoftwareEffectDispatchRunner(test.ledCount)
			startTime := time.Now().Add(-time.Second)

			if !dispatchSoftwareEffect(test.effect, runner, &startTime, nil) {
				t.Fatalf("dispatchSoftwareEffect(%q) reported no explicit mapping", test.effect)
			}
			if got, want := len(runner.Output), test.ledCount*3; got != want {
				t.Fatalf("dispatchSoftwareEffect(%q) output length = %d, want %d", test.effect, got, want)
			}
		})
	}
}

func TestOpenRGBSoftwareEffectDispatchVisorSingleLED(t *testing.T) {
	runner := newSoftwareEffectDispatchRunner(1)
	startTime := time.Now()

	if !dispatchSoftwareEffect("visor", runner, &startTime, nil) {
		t.Fatal("dispatchSoftwareEffect(\"visor\") reported no explicit mapping")
	}
	if want := []byte{12, 34, 56}; !bytes.Equal(runner.Output, want) {
		t.Fatalf("single-LED visor output = %v, want %v", runner.Output, want)
	}
}

func TestOpenRGBSoftwareEffectDispatchPreservesExistingMappings(t *testing.T) {
	installLightingTemperatureTestSeams(t, 45, 60, 65)
	tests := []string{
		"rainbow",
		"colorpulse",
		"aurora",
		"cpu-temperature",
		"gpu-temperature",
		"spiralrainbow",
		"pastelspiralrainbow",
	}

	for _, effect := range tests {
		t.Run(effect, func(t *testing.T) {
			runner := newSoftwareEffectDispatchRunner(4)
			startTime := time.Now().Add(-time.Second)

			if !dispatchSoftwareEffect(effect, runner, &startTime, nil) {
				t.Fatalf("dispatchSoftwareEffect(%q) reported no explicit mapping", effect)
			}
			if got, want := len(runner.Output), 12; got != want {
				t.Fatalf("dispatchSoftwareEffect(%q) output length = %d, want %d", effect, got, want)
			}
		})
	}
}

func TestOpenRGBSoftwareEffectDispatchUnknownUsesStatic(t *testing.T) {
	for _, effect := range []string{"", "unknown", "RAINBOW", " rainbow", "rainbow "} {
		t.Run(effect, func(t *testing.T) {
			runner := newSoftwareEffectDispatchRunner(4)
			expected := newSoftwareEffectDispatchRunner(4)
			expected.Static()
			startTime := time.Now()

			if dispatchSoftwareEffect(effect, runner, &startTime, nil) {
				t.Fatalf("dispatchSoftwareEffect(%q) reported an explicit mapping", effect)
			}
			if !bytes.Equal(runner.Output, expected.Output) {
				t.Fatalf("dispatchSoftwareEffect(%q) output = %v, want Static output %v", effect, runner.Output, expected.Output)
			}
		})
	}
}

func TestOpenRGBSoftwareEffectEligibilityRejectsUnadvertisedProfiles(t *testing.T) {
	catalogueBefore := append([]string(nil), rgbModes...)
	unsupportedEffects := []string{
		"arc",
		"visor",
		"spiralrainbow",
		"pastelspiralrainbow",
		"unknown",
		"",
		"RAINBOW",
		" rainbow",
		"rainbow ",
	}

	for _, effect := range unsupportedEffects {
		t.Run(effect, func(t *testing.T) {
			device := &Device{
				effect:        effect,
				RGBModes:      append([]string(nil), rgbModes...),
				DeviceProfile: &DeviceProfile{RGBProfile: effect},
			}
			runner := newSoftwareEffectDispatchRunner(4)
			expected := newSoftwareEffectDispatchRunner(4)
			expected.Static()
			startTime := time.Now()

			if dispatchEligibleSoftwareEffect(device.effect, device.RGBModes, runner, &startTime, nil) {
				t.Fatalf("dispatchEligibleSoftwareEffect(%q) reported an eligible mapping", effect)
			}
			if !bytes.Equal(runner.Output, expected.Output) {
				t.Fatalf("dispatchEligibleSoftwareEffect(%q) output = %v, want Static output %v", effect, runner.Output, expected.Output)
			}
			if device.effect != effect || device.DeviceProfile.RGBProfile != effect {
				t.Fatalf("unsupported profile changed: effect = %q, profile = %q, want %q", device.effect, device.DeviceProfile.RGBProfile, effect)
			}
			if !slices.Equal(device.RGBModes, catalogueBefore) {
				t.Fatalf("device RGBModes changed: got %v, want %v", device.RGBModes, catalogueBefore)
			}
		})
	}
	if !slices.Equal(rgbModes, catalogueBefore) {
		t.Fatalf("package RGB catalogue changed: got %v, want %v", rgbModes, catalogueBefore)
	}
}

func TestOpenRGBSoftwareEffectResumeUsesEligibilityBoundary(t *testing.T) {
	profileDir, frames := installSoftwareEffectResumeSeams(t)
	device := newLightingMutationDevice()
	brightness := uint8(100)
	device.RGBModes = append([]string(nil), rgbModes...)
	device.colorCount = 4
	device.brightness = brightness
	device.lastColor = []byte{12, 34, 56}
	device.effect = "arc"
	device.DeviceProfile.RGBProfile = "arc"
	device.DeviceProfile.BrightnessSlider = &brightness

	if err := device.resumeDesiredState(context.Background()); err != nil {
		t.Fatalf("resume unsupported persisted effect: %v", err)
	}
	t.Cleanup(device.Stop)

	var frame []byte
	select {
	case frame = <-frames:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for resumed Static fallback frame")
	}
	device.Stop()

	wantFrame := []byte{
		12, 34, 56,
		12, 34, 56,
		12, 34, 56,
		12, 34, 56,
	}
	if !bytes.Equal(frame, wantFrame) {
		t.Fatalf("resumed unsupported effect frame = %v, want Static fallback %v", frame, wantFrame)
	}
	if device.effect != "arc" || device.DeviceProfile.RGBProfile != "arc" {
		t.Fatalf("resumed unsupported profile changed: effect = %q, profile = %q", device.effect, device.DeviceProfile.RGBProfile)
	}
	if persisted := readLightingDeviceProfile(t, profileDir); persisted.RGBProfile != "arc" {
		t.Fatalf("persisted unsupported profile = %q, want arc", persisted.RGBProfile)
	}
}

func TestOpenRGBSoftwareEffectEligibilityPreservesSupportedProfiles(t *testing.T) {
	installLightingTemperatureTestSeams(t, 45, 60, 65)
	for _, effect := range []string{"colorpulse", "rainbow", "cpu-temperature", "gpu-temperature"} {
		t.Run(effect, func(t *testing.T) {
			runner := newSoftwareEffectDispatchRunner(4)
			startTime := time.Now().Add(-time.Second)

			if !dispatchEligibleSoftwareEffect(effect, rgbModes, runner, &startTime, nil) {
				t.Fatalf("dispatchEligibleSoftwareEffect(%q) reported an ineligible mapping", effect)
			}
			if got, want := len(runner.Output), 12; got != want {
				t.Fatalf("dispatchEligibleSoftwareEffect(%q) output length = %d, want %d", effect, got, want)
			}
		})
	}

	for _, effect := range []string{"static", "off"} {
		if !slices.Contains(rgbModes, effect) {
			t.Errorf("existing special-path effect %q is missing from RGBModes", effect)
		}
	}
}

func TestOpenRGBSoftwareEffectRGBModesExcludeUnadvertisedDispatchCases(t *testing.T) {
	unadvertised := []string{
		"arc",
		"comet",
		"datastream",
		"marquee",
		"nebula",
		"plasmacore",
		"rain",
		"rotarystack",
		"sequential",
		"stardust",
		"tokyonight",
		"visor",
		"spiralrainbow",
		"pastelspiralrainbow",
	}
	for _, effect := range unadvertised {
		if slices.Contains(rgbModes, effect) {
			t.Errorf("RGBModes unexpectedly advertises dispatch-only effect %q", effect)
		}
	}
}
