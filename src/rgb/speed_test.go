package rgb

import (
	"math"
	"reflect"
	"testing"
)

func TestProfileSpeedRange(t *testing.T) {
	tests := []struct {
		profile string
		minimum float64
		maximum float64
	}{
		{profile: "circle", minimum: 1, maximum: 10},
		{profile: "aurora", minimum: 1, maximum: 10},
		{profile: "flame", minimum: 0.1, maximum: 10},
		{profile: "cyberpunkglitch", minimum: 0.1, maximum: 10},
	}

	for _, test := range tests {
		t.Run(test.profile, func(t *testing.T) {
			minimum, maximum := ProfileSpeedRange(test.profile)
			if minimum != test.minimum || maximum != test.maximum {
				t.Fatalf(
					"ProfileSpeedRange(%q) = (%v, %v), want (%v, %v)",
					test.profile,
					minimum,
					maximum,
					test.minimum,
					test.maximum,
				)
			}
		})
	}
}

func TestHasSpeedControl(t *testing.T) {
	noSpeedEffects := []string{
		"off",
		"static",
		"cpu-temperature",
		"gpu-temperature",
	}
	for _, effect := range noSpeedEffects {
		if HasSpeedControl(effect) {
			t.Errorf("HasSpeedControl(%q) = true, want false", effect)
		}
	}

	speedEffects := []string{
		"arc",
		"aurora",
		"circle",
		"circleshift",
		"colorpulse",
		"colorshift",
		"colorwarp",
		"comet",
		"cyberpunkglitch",
		"datastream",
		"flame",
		"flickering",
		"gradient",
		"marquee",
		"nebula",
		"pastelrainbow",
		"pastelspiralrainbow",
		"plasmacore",
		"rain",
		"rainbow",
		"rotarystack",
		"rotator",
		"sequential",
		"spinner",
		"spiralrainbow",
		"stardust",
		"storm",
		"tokyonight",
		"visor",
		"watercolor",
		"wave",
	}
	for _, effect := range speedEffects {
		if !HasSpeedControl(effect) {
			t.Errorf("HasSpeedControl(%q) = false, want true", effect)
		}
	}

	if got := len(noSpeedEffects) + len(speedEffects); got != 35 {
		t.Fatalf("generic speed contract covers %d effects, want 35", got)
	}
}

func TestHasSpeedControlCompatibilityInputs(t *testing.T) {
	for _, profile := range []string{"liquid-temperature", "probe-temperature"} {
		if HasSpeedControl(profile) {
			t.Errorf("HasSpeedControl(%q) = true, want false", profile)
		}
	}

	for _, profile := range []string{
		"",
		"future-effect",
		"ARC",
		" arc",
		"arc ",
		"colorwave",
		"led",
		"rainbowwave",
		"tlk",
		"tlr",
	} {
		if !HasSpeedControl(profile) {
			t.Errorf("HasSpeedControl(%q) = false, want true", profile)
		}
	}
}

func TestHasSpeedControlDescriptorParityAndImmutability(t *testing.T) {
	descriptorsBefore := SoftwareEffectDescriptors()
	if len(descriptorsBefore) != 35 {
		t.Fatalf("descriptor count = %d, want 35", len(descriptorsBefore))
	}

	for _, descriptor := range descriptorsBefore {
		if got := HasSpeedControl(descriptor.ID); got != descriptor.SupportsSpeed {
			t.Errorf("HasSpeedControl(%q) = %t, want descriptor value %t", descriptor.ID, got, descriptor.SupportsSpeed)
		}
	}

	if descriptorsAfter := SoftwareEffectDescriptors(); !reflect.DeepEqual(descriptorsAfter, descriptorsBefore) {
		t.Fatalf("HasSpeedControl mutated the canonical descriptor registry:\nbefore: %#v\nafter:  %#v", descriptorsBefore, descriptorsAfter)
	}
}

func TestProfileSpeedForUpdatePreservesNoSpeedProfiles(t *testing.T) {
	if actual := ProfileSpeedForUpdate("static", 0, 7.5); actual != 7.5 {
		t.Errorf("ProfileSpeedForUpdate(static) = %v, want 7.5", actual)
	}
	if actual := ProfileSpeedForUpdate("circle", 3.5, 7.5); actual != 3.5 {
		t.Errorf("ProfileSpeedForUpdate(circle) = %v, want 3.5", actual)
	}
}

func TestRainSpeedFactorPreservesLevelsAndInterpolates(t *testing.T) {
	tests := []struct {
		stored float64
		factor float64
	}{
		{stored: 1, factor: 0.5},
		{stored: 1.5, factor: 0.4},
		{stored: 2, factor: 0.3},
		{stored: 2.5, factor: 0.2},
		{stored: 3, factor: 0.1},
		{stored: 0.5, factor: 1},
		{stored: 4, factor: 1},
	}

	for _, test := range tests {
		if actual := rainSpeedFactor(test.stored); math.Abs(actual-test.factor) > 0.0000001 {
			t.Errorf("rainSpeedFactor(%v) = %v, want %v", test.stored, actual, test.factor)
		}
	}
}

func TestStormFlashChanceUsesDurationSpeed(t *testing.T) {
	tests := []struct {
		speed  float64
		chance float32
	}{
		{speed: 1, chance: 0.004},
		{speed: 4, chance: 0.001},
		{speed: 10, chance: 0.0004},
		{speed: 0, chance: 0.001},
	}

	for _, test := range tests {
		if actual := stormFlashChance(test.speed); math.Abs(float64(actual-test.chance)) > 0.0000001 {
			t.Errorf("stormFlashChance(%v) = %v, want %v", test.speed, actual, test.chance)
		}
	}
}
