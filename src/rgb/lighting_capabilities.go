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
// canonical LumenForge software effects. Speed support remains governed by the
// existing persistent speed policy.
func LightingEffectCapabilities(effect string) (LightingEffectCapability, bool) {
	descriptor, ok := SoftwareEffectDescriptorByID(effect)
	if !ok {
		return LightingEffectCapability{}, false
	}

	return LightingEffectCapability{
		Palette:         descriptor.PaletteKind,
		UsesStartColor:  descriptor.UsesStart,
		UsesMiddleColor: descriptor.UsesMiddle,
		UsesEndColor:    descriptor.UsesEnd,
		SupportsSpeed:   HasSpeedControl(descriptor.ID),
	}, true
}
