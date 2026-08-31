// Package lightingpresentation contains immutable presentation DTOs for
// individual device lighting in the Devices workspace.
package lightingpresentation

// EffectOption is a presentation-safe copy of one supported lighting effect.
type EffectOption struct {
	ID    string
	Label string
}

// TemperaturePoint is a presentation-safe canonical semantic temperature point.
type TemperaturePoint struct {
	ColorHex string
	Celsius  float64
}

// GradientStop is a presentation-safe ordered Gradient stop.
type GradientStop struct {
	Position  float64
	ColorHex  string
	Intensity float64
}

// AuthoredZoneEditor describes a device-owned authored-color mode without
// making it a shared software effect setting.
type AuthoredZoneEditor struct {
	EffectID, Heading, Description string
	HasGroups                      bool
	Zones                          []AuthoredZone
}

// AuthoredZone is an immutable presentation copy of one authored lighting
// zone. Group and geometry fields are optional so simple devices need not
// manufacture layout metadata.
type AuthoredZone struct {
	ID          string
	Label       string
	ColorHex    string
	GroupID     string
	GroupLabel  string
	HasGeometry bool
	Left        int
	Top         int
	Width       int
	Height      int
}

// Snapshot is an immutable presentation/configuration view of one individual
// device's canonical lighting state. It does not claim current hardware output.
// TargetKind identifies the corresponding Devices lighting mutation target.
type Snapshot struct {
	TargetKind         string
	ConfiguredEffect   string
	EffectSupported    bool
	SupportedEffects   []EffectOption
	HasBrightness      bool
	Brightness         uint8
	HasSpeed           bool
	Speed              float64
	ClusterControlled  bool
	ExternalControlled bool
	PaletteKind        string
	SingleColorHex     string
	TwoColorStartHex   string
	TwoColorEndHex     string
	HasTemperature     bool
	TemperatureLow     TemperaturePoint
	TemperatureMiddle  TemperaturePoint
	TemperatureHigh    TemperaturePoint
	HasGradient        bool
	GradientStops      []GradientStop
	Customized         bool
	AuthoredZoneEditor *AuthoredZoneEditor
	ThreePinPort       *ThreePinPort
	// Channels is populated by controllers with independently configurable
	// physical lighting targets. Each entry composes the same immutable lighting
	// view rather than pretending the controller has one selected effect.
	Channels []Channel
}

// ThreePinPort is immutable presentation data for a controller-owned physical
// RGB-only port. It does not describe or assign canonical lighting targets.
type ThreePinPort struct {
	DeviceType       int
	DeviceOptions    []ThreePinDeviceOption
	Quantity         int
	QuantityOptions  []ThreePinQuantityOption
	QuantityDisabled bool
}

type ThreePinDeviceOption struct {
	ID, Label string
	Selected  bool
}

type ThreePinQuantityOption struct {
	Value, Label string
	Selected     bool
}

// Channel is an immutable presentation copy of one physical lighting target.
// TargetID is the canonical mutation identity; ChannelID remains the device's
// human/debuggable physical channel identity.
type Channel struct {
	TargetID  string
	ChannelID string
	Name      string
	Label     string
	LEDCount  int
	Lighting  Snapshot
	// ProbeTemperature is present only for a channel currently using CCXT's
	// channel-owned probe-temperature effect.
	ProbeTemperature *ProbeTemperature
}

// ProbeTemperature describes CCXT's existing channel-owned temperature-probe
// settings. It deliberately does not participate in canonical effect settings.
type ProbeTemperature struct {
	ChannelID string
	ProbeID   int
	Minimum   float64
	Maximum   float64
	Sources   []ProbeTemperatureSource
}

type ProbeTemperatureSource struct {
	ID       int
	Label    string
	Selected bool
}
