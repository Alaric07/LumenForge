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

// SoftwareEffectTemperaturePointContract describes the thresholded color
// points consumed by a generic temperature renderer.
type SoftwareEffectTemperaturePointContract uint8

const (
	// SoftwareEffectTemperaturePointsNone means the effect consumes no
	// thresholded temperature-point set.
	SoftwareEffectTemperaturePointsNone SoftwareEffectTemperaturePointContract = iota
	// SoftwareEffectTemperaturePointsLowMiddleHigh requires Low, Middle, and
	// High RGB colors, each with a finite Celsius threshold in strictly
	// increasing order.
	SoftwareEffectTemperaturePointsLowMiddleHigh
)

// SoftwareEffectDescriptor describes persisted inputs and compatible render
// targets for one generic LumenForge software effect.
type SoftwareEffectDescriptor struct {
	ID                string
	Label             string
	Scope             EffectScope
	PaletteKind       LightingPaletteKind
	UsesStart         bool
	UsesMiddle        bool
	UsesEnd           bool
	SupportsSpeed     bool
	Sensor            SoftwareEffectSensorRequirement
	Topology          SoftwareEffectTopology
	TemperaturePoints SoftwareEffectTemperaturePointContract
	MinimumLEDs       int
	Icon              string
	// RendererSmoothness preserves the shipped rgb.Profile implementation
	// parameter without making it persisted, user-editable effect state.
	RendererSmoothness int
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
		ID:                 id,
		Label:              label,
		Scope:              EffectScopeBoth,
		PaletteKind:        palette,
		UsesStart:          colors&softwareEffectUsesStart != 0,
		UsesMiddle:         colors&softwareEffectUsesMiddle != 0,
		UsesEnd:            colors&softwareEffectUsesEnd != 0,
		SupportsSpeed:      supportsSpeed,
		Sensor:             sensor,
		Topology:           topology,
		Icon:               id + ".svg",
		RendererSmoothness: softwareEffectRendererSmoothness[id],
	}
}

var softwareEffectRendererSmoothness = map[string]int{
	"arc":                 0,
	"aurora":              20,
	"circle":              20,
	"circleshift":         100,
	"colorpulse":          40,
	"colorshift":          50,
	"colorwarp":           20,
	"comet":               20,
	"cpu-temperature":     40,
	"cyberpunkglitch":     20,
	"datastream":          20,
	"flame":               20,
	"flickering":          40,
	"gpu-temperature":     40,
	"liquid-temperature":  40,
	"gradient":            0,
	"marquee":             0,
	"nebula":              0,
	"off":                 0,
	"pastelrainbow":       0,
	"pastelspiralrainbow": 0,
	"plasmacore":          20,
	"rain":                0,
	"rainbow":             0,
	"rotarystack":         0,
	"rotator":             0,
	"sequential":          0,
	"spinner":             20,
	"spiralrainbow":       0,
	"stardust":            20,
	"static":              20,
	"storm":               20,
	"tokyonight":          20,
	"visor":               0,
	"watercolor":          0,
	"wave":                10,
}

func newTemperatureSoftwareEffectDescriptor(
	id string,
	label string,
	sensor SoftwareEffectSensorRequirement,
) SoftwareEffectDescriptor {
	descriptor := newSoftwareEffectDescriptor(
		id,
		label,
		LightingPaletteTemperatureThree,
		softwareEffectUsesStart|softwareEffectUsesMiddle|softwareEffectUsesEnd,
		false,
		sensor,
		SoftwareEffectTopologyAny,
	)
	descriptor.TemperaturePoints = SoftwareEffectTemperaturePointsLowMiddleHigh
	return descriptor
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
	newTemperatureSoftwareEffectDescriptor("cpu-temperature", "CPU Temperature", SoftwareEffectSensorCPU),
	newSoftwareEffectDescriptor("cyberpunkglitch", "Cyberpunk Glitch", LightingPaletteGenerated, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("datastream", "Data Stream", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("flame", "Flame", LightingPaletteGenerated, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newSoftwareEffectDescriptor("flickering", "Flickering", LightingPaletteTwoColor, softwareEffectUsesStart|softwareEffectUsesEnd, true, SoftwareEffectSensorNone, SoftwareEffectTopologyLinear),
	newTemperatureSoftwareEffectDescriptor("gpu-temperature", "GPU Temperature", SoftwareEffectSensorGPU),
	newSoftwareEffectDescriptor("gradient", "Gradient", LightingPaletteGradient, 0, true, SoftwareEffectSensorNone, SoftwareEffectTopologyAny),
	newTemperatureSoftwareEffectDescriptor("liquid-temperature", "Liquid Temperature", SoftwareEffectSensorNone),
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

func validSoftwareEffectTemperaturePointContract(descriptor SoftwareEffectDescriptor) bool {
	if descriptor.PaletteKind != LightingPaletteTemperatureThree {
		return descriptor.TemperaturePoints == SoftwareEffectTemperaturePointsNone
	}

	return descriptor.TemperaturePoints == SoftwareEffectTemperaturePointsLowMiddleHigh &&
		descriptor.UsesStart && descriptor.UsesMiddle && descriptor.UsesEnd &&
		!descriptor.SupportsSpeed &&
		(descriptor.Sensor == SoftwareEffectSensorNone || descriptor.Sensor == SoftwareEffectSensorCPU || descriptor.Sensor == SoftwareEffectSensorGPU)
}

func init() {
	for _, descriptor := range softwareEffectDescriptors {
		if !validSoftwareEffectTemperaturePointContract(descriptor) {
			panic("invalid software effect temperature-point contract for " + descriptor.ID)
		}
	}
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
