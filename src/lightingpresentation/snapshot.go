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
}
