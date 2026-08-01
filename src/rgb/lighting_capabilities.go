package rgb

// LightingPaletteKind describes the color inputs consumed by a software
// lighting renderer. It does not describe live hardware state.
type LightingPaletteKind string

const (
	LightingPaletteNone             LightingPaletteKind = "none"
	LightingPaletteStaticSingle     LightingPaletteKind = "static-single-color"
	LightingPaletteTwoColor         LightingPaletteKind = "two-color"
	LightingPaletteTemperatureThree LightingPaletteKind = "temperature-three-color"
	LightingPaletteGradient         LightingPaletteKind = "gradient"
	LightingPaletteGenerated        LightingPaletteKind = "generated-palette"
)

// LightingEffectCapability describes the persisted inputs consumed by a
// confirmed software renderer.
type LightingEffectCapability struct {
	Palette         LightingPaletteKind
	UsesStartColor  bool
	UsesMiddleColor bool
	UsesEndColor    bool
	SupportsSpeed   bool
}

// LightingEffectCapabilities returns renderer-backed capability metadata for
// effects whose input behavior is known.
func LightingEffectCapabilities(effect string) (LightingEffectCapability, bool) {
	capability := LightingEffectCapability{SupportsSpeed: HasSpeedControl(effect)}

	switch effect {
	case "off":
		capability.Palette = LightingPaletteNone
	case "static":
		capability.Palette = LightingPaletteStaticSingle
		capability.UsesStartColor = true
	case "rotator":
		capability.Palette = LightingPaletteStaticSingle
		capability.UsesStartColor = true
	case "circle", "circleshift", "colorpulse", "colorshift", "flickering", "spinner", "storm", "wave":
		capability.Palette = LightingPaletteTwoColor
		capability.UsesStartColor = true
		capability.UsesEndColor = true
	case "cpu-temperature", "gpu-temperature":
		capability.Palette = LightingPaletteTemperatureThree
		capability.UsesStartColor = true
		capability.UsesMiddleColor = true
		capability.UsesEndColor = true
	case "gradient":
		capability.Palette = LightingPaletteGradient
	case "aurora", "colorwarp", "cyberpunkglitch", "flame", "pastelrainbow", "pastelspiralrainbow", "rainbow", "spiralrainbow", "watercolor":
		capability.Palette = LightingPaletteGenerated
	default:
		return LightingEffectCapability{}, false
	}

	return capability, true
}
