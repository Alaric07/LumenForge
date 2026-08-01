package rgb

// EffectScope identifies compatible render targets for a LumenForge software
// effect. OpenRGB may transport frames to imported hardware, but it is not the
// source of these animations. Native firmware effects remain device-owned.
type EffectScope uint8

const (
	// EffectScopeDevice includes a compatible individual device LED buffer.
	EffectScopeDevice EffectScope = 1 << iota
	// EffectScopeCluster includes a compatible RGB Cluster LED buffer.
	EffectScopeCluster
	// EffectScopeBoth includes individual device and RGB Cluster LED buffers.
	EffectScopeBoth = EffectScopeDevice | EffectScopeCluster
)

// Includes reports whether the scope contains every bit in target. A zero
// target is never a valid configured scope.
func (scope EffectScope) Includes(target EffectScope) bool {
	return target != 0 && scope&target == target
}

// SoftwareEffectSensorRequirement identifies sensor input consumed by a
// generic LumenForge software effect.
type SoftwareEffectSensorRequirement uint8

const (
	// SoftwareEffectSensorNone means the effect consumes no sensor input.
	SoftwareEffectSensorNone SoftwareEffectSensorRequirement = iota
	// SoftwareEffectSensorCPU means the effect consumes CPU temperature input.
	SoftwareEffectSensorCPU
	// SoftwareEffectSensorGPU means the effect consumes GPU temperature input.
	SoftwareEffectSensorGPU
)

// SoftwareEffectTopology describes the ordered LED topology expected by a
// generic LumenForge software renderer. It is descriptive metadata and does
// not reject or filter devices.
type SoftwareEffectTopology uint8

const (
	// SoftwareEffectTopologyAny means output does not meaningfully depend on LED position.
	SoftwareEffectTopologyAny SoftwareEffectTopology = iota
	// SoftwareEffectTopologyLinear means the renderer uses one ordered LED sequence.
	SoftwareEffectTopologyLinear
)

// SoftwareEffectDescriptor describes persisted inputs and compatible render
// targets for one generic LumenForge software effect.
type SoftwareEffectDescriptor struct {
	ID            string
	Label         string
	Scope         EffectScope
	PaletteKind   LightingPaletteKind
	UsesStart     bool
	UsesMiddle    bool
	UsesEnd       bool
	SupportsSpeed bool
	Sensor        SoftwareEffectSensorRequirement
	Topology      SoftwareEffectTopology
	MinimumLEDs   int
	Icon          string
}

type softwareEffectColorUsage uint8

const (
	softwareEffectUsesStart softwareEffectColorUsage = 1 << iota
	softwareEffectUsesMiddle
	softwareEffectUsesEnd
)

func newSoftwareEffectDescriptor(
	id string,
	label string,
	palette LightingPaletteKind,
	colors softwareEffectColorUsage,
	supportsSpeed bool,
	sensor SoftwareEffectSensorRequirement,
	topology SoftwareEffectTopology,
) SoftwareEffectDescriptor {
	return SoftwareEffectDescriptor{
		ID:            id,
		Label:         label,
		Scope:         EffectScopeBoth,
		PaletteKind:   palette,
		UsesStart:     colors&softwareEffectUsesStart != 0,
		UsesMiddle:    colors&softwareEffectUsesMiddle != 0,
		UsesEnd:       colors&softwareEffectUsesEnd != 0,
		SupportsSpeed: supportsSpeed,
		Sensor:        sensor,
		Topology:      topology,
		Icon:          id + ".svg",
	}
}

// softwareEffectDescriptors is ordered case-insensitively by display label,
// with stable ID as the tie-break. Keep this order deterministic for callers.
var softwareEffectDescriptors = []SoftwareEffectDescriptor{
	newSoftwareEffectDescriptor("arc", "Arc", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("aurora", "Aurora", LightingPaletteGenerated, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("circle", "Circle", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("circleshift", "Circle Shift", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("colorpulse", "Color Pulse", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyAny),
	newSoftwareEffectDescriptor("colorshift", "Color Shift", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyAny),
	newSoftwareEffectDescriptor("colorwarp", "Color Warp", LightingPaletteGenerated, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyAny),
	newSoftwareEffectDescriptor("comet", "Comet", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("cpu-temperature", "CPU Temperature", LightingPaletteTemperatureThree, softwareEffectUsesStart|softwareEffectUsesMiddle|softwareEffectUsesEnd, false, SoftwareEffectSensorCPU, SoftwareEffectTopologyAny),
	newSoftwareEffectDescriptor("cyberpunkglitch", "Cyberpunk Glitch", LightingPaletteGenerated, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("datastream", "Data Stream", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("flame", "Flame", LightingPaletteGenerated, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("flickering", "Flickering", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("gpu-temperature", "GPU Temperature", LightingPaletteTemperatureThree, softwareEffectUsesStart|softwareEffectUsesMiddle|softwareEffectUsesEnd, false, SoftwareEffectSensorGPU, SoftwareEffectTopologyAny),
	newSoftwareEffectDescriptor("gradient", "Gradient", LightingPaletteGradient, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyAny),
	newSoftwareEffectDescriptor("marquee", "Marquee", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("nebula", "Nebula", LightingPaletteGenerated, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("off", "Off", LightingPaletteNone, 0, false, SoftwareEffectSensorNone, SoftwareEffectTopologyAny),
	newSoftwareEffectDescriptor("pastelrainbow", "Pastel Rainbow", LightingPaletteGenerated, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("pastelspiralrainbow", "Pastel Spiral Rainbow", LightingPaletteGenerated, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("plasmacore", "Plasma Core", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("rain", "Rain", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("rainbow", "Rainbow", LightingPaletteGenerated, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("rotarystack", "Rotary Stack", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("rotator", "Rotator", LightingPaletteStaticSingle, softwareEffectUsesStart, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("sequential", "Sequential", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("spinner", "Spinner", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("spiralrainbow", "Spiral Rainbow", LightingPaletteGenerated, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("stardust", "Stardust", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("static", "Static", LightingPaletteStaticSingle, softwareEffectUsesStart, false, SoftwareEffectSensorNone, SoftwareEffectTopologyAny),
	newSoftwareEffectDescriptor("storm", "Storm", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyAny),
	newSoftwareEffectDescriptor("tokyonight", "Tokyo Night", LightingPaletteGenerated, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("visor", "Visor", LightingPaletteStaticSingle, softwareEffectUsesStart, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("watercolor", "Water Color", LightingPaletteGenerated, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("wave", "Wave", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
}

// SoftwareEffectDescriptorByID looks up a generic LumenForge software-effect
// descriptor by its case-sensitive stable ID.
func SoftwareEffectDescriptorByID(id string) (SoftwareEffectDescriptor, bool) {
	for _, descriptor := range softwareEffectDescriptors {
		if descriptor.ID == id {
			return descriptor, true
		}
	}
	return SoftwareEffectDescriptor{}, false
}

// SoftwareEffectDescriptors returns a defensive copy of the canonical generic
// LumenForge software-effect descriptors in display-label order.
func SoftwareEffectDescriptors() []SoftwareEffectDescriptor {
	descriptors := make([]SoftwareEffectDescriptor, len(softwareEffectDescriptors))
	copy(descriptors, softwareEffectDescriptors)
	return descriptors
}
