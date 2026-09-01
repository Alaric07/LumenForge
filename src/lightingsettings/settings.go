// Package lightingsettings owns complete software-effect settings, immutable
// shipped defaults, target customization persistence, and canonical resolution.
package lightingsettings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"

	"LumenForge/src/rgb"
)

const (
	// SchemaVersion is the only complete effect-settings schema understood by
	// this clean-break foundation.
	SchemaVersion        = 1
	maximumGradientStops = 1024
)

var (
	ErrUnknownEffect      = errors.New("unknown lighting effect")
	ErrInvalidSettings    = errors.New("invalid lighting effect settings")
	ErrDefaultUnavailable = errors.New("shipped lighting default unavailable")
	ErrInvalidTarget      = errors.New("invalid lighting target")
)

// Color is a renderer color without effect-independent owning Brightness.
type Color struct {
	Red   float64 `json:"red"`
	Green float64 `json:"green"`
	Blue  float64 `json:"blue"`
}

// IndexedColor is one requested color in a complete indexed LED palette.
type IndexedColor struct {
	Index    int
	ColorHex string
}

// UnmarshalJSON distinguishes a complete black color from an incomplete
// persisted color whose zero-valued channels were merely omitted.
func (color *Color) UnmarshalJSON(data []byte) error {
	var value struct {
		Red   *float64 `json:"red"`
		Green *float64 `json:"green"`
		Blue  *float64 `json:"blue"`
	}
	if err := decodeStrictJSON(data, &value); err != nil {
		return err
	}
	if value.Red == nil || value.Green == nil || value.Blue == nil {
		return fmt.Errorf("complete red, green, and blue channels are required")
	}
	*color = Color{Red: *value.Red, Green: *value.Green, Blue: *value.Blue}
	return nil
}

// SingleColorSettings is the complete palette for a single-color effect.
type SingleColorSettings struct {
	Color Color `json:"color"`
}

func (settings *SingleColorSettings) UnmarshalJSON(data []byte) error {
	var value struct {
		Color *Color `json:"color"`
	}
	if err := decodeStrictJSON(data, &value); err != nil {
		return err
	}
	if value.Color == nil {
		return fmt.Errorf("single color is required")
	}
	settings.Color = *value.Color
	return nil
}

// TwoColorSettings is the complete palette for a two-color effect.
type TwoColorSettings struct {
	Start Color `json:"start"`
	End   Color `json:"end"`
}

func (settings *TwoColorSettings) UnmarshalJSON(data []byte) error {
	var value struct {
		Start *Color `json:"start"`
		End   *Color `json:"end"`
	}
	if err := decodeStrictJSON(data, &value); err != nil {
		return err
	}
	if value.Start == nil || value.End == nil {
		return fmt.Errorf("complete Start and End colors are required")
	}
	*settings = TwoColorSettings{Start: *value.Start, End: *value.End}
	return nil
}

// TemperaturePoint pairs a semantic temperature color with its renderer-used
// Celsius threshold.
type TemperaturePoint struct {
	Color   Color   `json:"color"`
	Celsius float64 `json:"celsius"`
}

func (point *TemperaturePoint) UnmarshalJSON(data []byte) error {
	var value struct {
		Color   *Color   `json:"color"`
		Celsius *float64 `json:"celsius"`
	}
	if err := decodeStrictJSON(data, &value); err != nil {
		return err
	}
	if value.Color == nil || value.Celsius == nil {
		return fmt.Errorf("temperature color and Celsius threshold are required")
	}
	*point = TemperaturePoint{Color: *value.Color, Celsius: *value.Celsius}
	return nil
}

// TemperatureSettings preserves the canonical Low, Middle, and High roles.
type TemperatureSettings struct {
	Low    TemperaturePoint `json:"low"`
	Middle TemperaturePoint `json:"middle"`
	High   TemperaturePoint `json:"high"`
}

func (settings *TemperatureSettings) UnmarshalJSON(data []byte) error {
	var value struct {
		Low    *TemperaturePoint `json:"low"`
		Middle *TemperaturePoint `json:"middle"`
		High   *TemperaturePoint `json:"high"`
	}
	if err := decodeStrictJSON(data, &value); err != nil {
		return err
	}
	if value.Low == nil || value.Middle == nil || value.High == nil {
		return fmt.Errorf("complete Low, Middle, and High points are required")
	}
	*settings = TemperatureSettings{Low: *value.Low, Middle: *value.Middle, High: *value.High}
	return nil
}

// GradientStop is one ordered Gradient color. Intensity is relative to the
// effect-independent owning Brightness.
type GradientStop struct {
	Position  float64 `json:"position"`
	Color     Color   `json:"color"`
	Intensity float64 `json:"intensity"`
}

func (stop *GradientStop) UnmarshalJSON(data []byte) error {
	var value struct {
		Position  *float64 `json:"position"`
		Color     *Color   `json:"color"`
		Intensity *float64 `json:"intensity"`
	}
	if err := decodeStrictJSON(data, &value); err != nil {
		return err
	}
	if value.Position == nil || value.Color == nil || value.Intensity == nil {
		return fmt.Errorf("Gradient stop position, color, and intensity are required")
	}
	*stop = GradientStop{Position: *value.Position, Color: *value.Color, Intensity: *value.Intensity}
	return nil
}

// GradientSettings is a complete ordered Gradient palette.
type GradientSettings struct {
	Stops []GradientStop `json:"stops"`
}

func (settings *GradientSettings) UnmarshalJSON(data []byte) error {
	var value struct {
		Stops *[]GradientStop `json:"stops"`
	}
	if err := decodeStrictJSON(data, &value); err != nil {
		return err
	}
	if value.Stops == nil {
		return fmt.Errorf("Gradient stops are required")
	}
	settings.Stops = *value.Stops
	return nil
}

// EffectSettings is one complete descriptor-matched software-effect record.
// Optional variants select a complete shape; they never mean field inheritance.
type EffectSettings struct {
	SchemaVersion int                  `json:"schemaVersion"`
	EffectID      string               `json:"effectId"`
	Speed         *float64             `json:"speed,omitempty"`
	SingleColor   *SingleColorSettings `json:"singleColor,omitempty"`
	TwoColor      *TwoColorSettings    `json:"twoColor,omitempty"`
	Temperature   *TemperatureSettings `json:"temperature,omitempty"`
	Gradient      *GradientSettings    `json:"gradient,omitempty"`
}

// Clone returns a deep copy that shares no mutable settings data with value.
func (value EffectSettings) Clone() EffectSettings {
	cloned := value
	if value.Speed != nil {
		speed := *value.Speed
		cloned.Speed = &speed
	}
	if value.SingleColor != nil {
		single := *value.SingleColor
		cloned.SingleColor = &single
	}
	if value.TwoColor != nil {
		two := *value.TwoColor
		cloned.TwoColor = &two
	}
	if value.Temperature != nil {
		temperature := *value.Temperature
		cloned.Temperature = &temperature
	}
	if value.Gradient != nil {
		gradient := *value.Gradient
		gradient.Stops = append([]GradientStop(nil), value.Gradient.Stops...)
		cloned.Gradient = &gradient
	}
	return cloned
}

// Validate verifies that value is complete and exactly matches its canonical
// effect descriptor. It never fills omitted data from shipped defaults.
func Validate(value EffectSettings) error {
	if value.SchemaVersion != SchemaVersion {
		return invalidSettings(value.EffectID, "unsupported schema version %d", value.SchemaVersion)
	}
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(value.EffectID)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownEffect, value.EffectID)
	}

	if descriptor.SupportsSpeed {
		if value.Speed == nil || !finite(*value.Speed) {
			return invalidSettings(value.EffectID, "finite Speed is required")
		}
		minimum, maximum := rgb.ProfileSpeedRange(value.EffectID)
		if *value.Speed < minimum || *value.Speed > maximum {
			return invalidSettings(value.EffectID, "Speed must be between %g and %g", minimum, maximum)
		}
	} else if value.Speed != nil {
		return invalidSettings(value.EffectID, "Speed is not supported")
	}

	variants := 0
	for _, present := range []bool{
		value.SingleColor != nil,
		value.TwoColor != nil,
		value.Temperature != nil,
		value.Gradient != nil,
	} {
		if present {
			variants++
		}
	}

	switch descriptor.PaletteKind {
	case rgb.LightingPaletteNone, rgb.LightingPaletteGenerated:
		if variants != 0 {
			return invalidSettings(value.EffectID, "effect does not accept a color-settings variant")
		}
	case rgb.LightingPaletteStaticSingle:
		if variants != 1 || value.SingleColor == nil {
			return invalidSettings(value.EffectID, "complete single-color settings are required")
		}
		if err := validateColor(value.SingleColor.Color); err != nil {
			return invalidSettings(value.EffectID, "single color: %v", err)
		}
	case rgb.LightingPaletteTwoColor:
		if variants != 1 || value.TwoColor == nil {
			return invalidSettings(value.EffectID, "complete Start and End colors are required")
		}
		if err := validateColor(value.TwoColor.Start); err != nil {
			return invalidSettings(value.EffectID, "Start color: %v", err)
		}
		if err := validateColor(value.TwoColor.End); err != nil {
			return invalidSettings(value.EffectID, "End color: %v", err)
		}
	case rgb.LightingPaletteTemperatureThree:
		if variants != 1 || value.Temperature == nil {
			return invalidSettings(value.EffectID, "complete Low, Middle, and High temperature points are required")
		}
		if descriptor.TemperaturePoints != rgb.SoftwareEffectTemperaturePointsLowMiddleHigh {
			return invalidSettings(value.EffectID, "descriptor does not define Low/Middle/High temperature points")
		}
		if err := validateTemperature(*value.Temperature); err != nil {
			return invalidSettings(value.EffectID, "%v", err)
		}
	case rgb.LightingPaletteGradient:
		if variants != 1 || value.Gradient == nil {
			return invalidSettings(value.EffectID, "complete Gradient settings are required")
		}
		if err := validateGradient(*value.Gradient); err != nil {
			return invalidSettings(value.EffectID, "%v", err)
		}
	default:
		return invalidSettings(value.EffectID, "unsupported palette contract %q", descriptor.PaletteKind)
	}

	return nil
}

func validateColor(color Color) error {
	for _, channel := range []struct {
		name  string
		value float64
	}{
		{name: "red", value: color.Red},
		{name: "green", value: color.Green},
		{name: "blue", value: color.Blue},
	} {
		if !finite(channel.value) || channel.value < 0 || channel.value > 255 {
			return fmt.Errorf("%s channel must be finite and between 0 and 255", channel.name)
		}
	}
	return nil
}

func validateTemperature(settings TemperatureSettings) error {
	points := []struct {
		name  string
		point TemperaturePoint
	}{
		{name: "Low", point: settings.Low},
		{name: "Middle", point: settings.Middle},
		{name: "High", point: settings.High},
	}
	for _, item := range points {
		if err := validateColor(item.point.Color); err != nil {
			return fmt.Errorf("%s color: %w", item.name, err)
		}
		if !finite(item.point.Celsius) {
			return fmt.Errorf("%s Celsius threshold must be finite", item.name)
		}
	}
	if !(settings.Low.Celsius < settings.Middle.Celsius && settings.Middle.Celsius < settings.High.Celsius) {
		return fmt.Errorf("temperature thresholds must be strictly ordered Low, Middle, High")
	}
	return nil
}

func validateGradient(settings GradientSettings) error {
	if len(settings.Stops) < 2 || len(settings.Stops) > maximumGradientStops {
		return fmt.Errorf("Gradient requires between 2 and %d stops", maximumGradientStops)
	}
	previous := -1.0
	for index, stop := range settings.Stops {
		if err := validateColor(stop.Color); err != nil {
			return fmt.Errorf("Gradient stop %d color: %w", index, err)
		}
		if !finite(stop.Position) || stop.Position < 0 || stop.Position > 1 {
			return fmt.Errorf("Gradient stop %d position must be finite and between 0 and 1", index)
		}
		if stop.Position < previous {
			return fmt.Errorf("Gradient stops must be ordered by position")
		}
		if !finite(stop.Intensity) || stop.Intensity < 0 || stop.Intensity > 1 {
			return fmt.Errorf("Gradient stop %d intensity must be finite and between 0 and 1", index)
		}
		previous = stop.Position
	}
	return nil
}

func invalidSettings(effectID, format string, args ...any) error {
	return fmt.Errorf("%w for %q: %s", ErrInvalidSettings, effectID, fmt.Sprintf(format, args...))
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func decodeStrictJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}
