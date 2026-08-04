package lightingsettings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"LumenForge/src/rgb"
)

// DefaultRepository owns validated hidden shipped definitions. Its contents
// are immutable after construction.
type DefaultRepository struct {
	settings map[string]EffectSettings
}

// LoadDefaultRepository loads the shipped rgb.json source without using or
// mutating the legacy package-global RGB editor state.
func LoadDefaultRepository(path string) (*DefaultRepository, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %q", ErrDefaultUnavailable, path)
		}
		return nil, fmt.Errorf("open shipped lighting defaults %q: %w", path, err)
	}
	defer file.Close()

	var shipped struct {
		Profiles map[string]shippedProfile `json:"profiles"`
	}
	decoder := json.NewDecoder(io.LimitReader(file, 4<<20))
	if err = decoder.Decode(&shipped); err != nil {
		return nil, fmt.Errorf("decode shipped lighting defaults %q: %w", path, err)
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return nil, fmt.Errorf("decode shipped lighting defaults %q: %w", path, err)
	}
	if shipped.Profiles == nil {
		return nil, fmt.Errorf("decode shipped lighting defaults %q: profiles are missing", path)
	}

	definitions := make(map[string]EffectSettings, len(rgb.SoftwareEffectDescriptors()))
	for _, descriptor := range rgb.SoftwareEffectDescriptors() {
		if descriptor.ID == "off" {
			// rgb.Init has always supplied Off as a code-defined shipped profile.
			definitions[descriptor.ID] = EffectSettings{SchemaVersion: SchemaVersion, EffectID: descriptor.ID}
			continue
		}
		profile, ok := shipped.Profiles[descriptor.ID]
		if !ok {
			return nil, fmt.Errorf("%w: effect %q", ErrDefaultUnavailable, descriptor.ID)
		}
		settings, conversionErr := settingsFromShippedProfile(descriptor, profile)
		if conversionErr != nil {
			return nil, fmt.Errorf("shipped lighting default %q: %w", descriptor.ID, conversionErr)
		}
		definitions[descriptor.ID] = settings
	}

	return &DefaultRepository{settings: definitions}, nil
}

// Get returns a defensive copy of one complete hidden shipped default.
func (repository *DefaultRepository) Get(effectID string) (EffectSettings, error) {
	if repository == nil {
		return EffectSettings{}, fmt.Errorf("%w: repository is nil", ErrDefaultUnavailable)
	}
	if _, known := rgb.SoftwareEffectDescriptorByID(effectID); !known {
		return EffectSettings{}, fmt.Errorf("%w: %q", ErrUnknownEffect, effectID)
	}
	settings, ok := repository.settings[effectID]
	if !ok {
		return EffectSettings{}, fmt.Errorf("%w: effect %q", ErrDefaultUnavailable, effectID)
	}
	return settings.Clone(), nil
}

type shippedProfile struct {
	Speed     *float64          `json:"speed"`
	Start     *rgb.Color        `json:"start"`
	Middle    *rgb.Color        `json:"middle"`
	End       *rgb.Color        `json:"end"`
	Gradients map[int]rgb.Color `json:"gradients"`
}

func settingsFromShippedProfile(descriptor rgb.SoftwareEffectDescriptor, profile shippedProfile) (EffectSettings, error) {
	settings := EffectSettings{SchemaVersion: SchemaVersion, EffectID: descriptor.ID}
	if descriptor.SupportsSpeed {
		if profile.Speed == nil {
			return EffectSettings{}, fmt.Errorf("Speed is missing")
		}
		speed := *profile.Speed
		settings.Speed = &speed
	}

	switch descriptor.PaletteKind {
	case rgb.LightingPaletteNone, rgb.LightingPaletteGenerated:
	case rgb.LightingPaletteStaticSingle:
		if profile.Start == nil {
			return EffectSettings{}, fmt.Errorf("single color is missing")
		}
		settings.SingleColor = &SingleColorSettings{Color: colorFromRGB(*profile.Start)}
	case rgb.LightingPaletteTwoColor:
		if profile.Start == nil || profile.End == nil {
			return EffectSettings{}, fmt.Errorf("Start or End color is missing")
		}
		settings.TwoColor = &TwoColorSettings{
			Start: colorFromRGB(*profile.Start),
			End:   colorFromRGB(*profile.End),
		}
	case rgb.LightingPaletteTemperatureThree:
		if profile.Start == nil || profile.Middle == nil || profile.End == nil {
			return EffectSettings{}, fmt.Errorf("Low, Middle, or High temperature point is missing")
		}
		settings.Temperature = &TemperatureSettings{
			Low:    temperaturePointFromRGB(*profile.Start),
			Middle: temperaturePointFromRGB(*profile.Middle),
			High:   temperaturePointFromRGB(*profile.End),
		}
	case rgb.LightingPaletteGradient:
		if profile.Gradients == nil {
			return EffectSettings{}, fmt.Errorf("Gradient stops are missing")
		}
		type indexedStop struct {
			index int
			stop  GradientStop
		}
		indexed := make([]indexedStop, 0, len(profile.Gradients))
		for index, color := range profile.Gradients {
			indexed = append(indexed, indexedStop{index: index, stop: GradientStop{
				Position:  color.Position,
				Color:     colorFromRGB(color),
				Intensity: color.Brightness,
			}})
		}
		sort.SliceStable(indexed, func(first, second int) bool {
			if indexed[first].stop.Position == indexed[second].stop.Position {
				return indexed[first].index < indexed[second].index
			}
			return indexed[first].stop.Position < indexed[second].stop.Position
		})
		stops := make([]GradientStop, len(indexed))
		for index := range indexed {
			stops[index] = indexed[index].stop
		}
		settings.Gradient = &GradientSettings{Stops: stops}
	default:
		return EffectSettings{}, fmt.Errorf("unsupported palette contract %q", descriptor.PaletteKind)
	}

	if err := Validate(settings); err != nil {
		return EffectSettings{}, err
	}
	return settings, nil
}

func colorFromRGB(color rgb.Color) Color {
	return Color{Red: color.Red, Green: color.Green, Blue: color.Blue}
}

func temperaturePointFromRGB(color rgb.Color) TemperaturePoint {
	return TemperaturePoint{Color: colorFromRGB(color), Celsius: color.Temperature}
}
