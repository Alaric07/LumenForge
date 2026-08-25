package scimitarprorgb

import (
	"fmt"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

// replaceCanonicalEffectSettings converts the complete legacy editor payload
// into canonical effect settings and persists it before reporting whether the
// changed effect is currently selected.
func (d *Device) replaceCanonicalEffectSettings(effect string, profile rgb.Profile) (bool, error) {
	if d == nil || d.lightingSource == nil {
		return false, fmt.Errorf("Scimitar Pro canonical lighting source is unavailable")
	}

	current, err := d.lightingSource.resolveEffectSettings(effect)
	if err != nil {
		return false, fmt.Errorf("resolve Scimitar Pro effect settings: %w", err)
	}
	settings, err := scimitarEffectSettingsFromRGBProfile(effect, profile, current)
	if err != nil {
		return false, err
	}
	selected, err := d.lightingSource.selectedEffect()
	if err != nil {
		return false, fmt.Errorf("resolve Scimitar Pro selected effect: %w", err)
	}
	if err = d.lightingSource.setEffectSettings(effect, settings); err != nil {
		return false, fmt.Errorf("persist Scimitar Pro effect settings: %w", err)
	}
	return selected == effect, nil
}

func scimitarEffectSettingsFromRGBProfile(
	effect string,
	profile rgb.Profile,
	current lightingsettings.EffectSettings,
) (lightingsettings.EffectSettings, error) {
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeDevice) {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported Scimitar Pro effect %q", effect)
	}
	if current.EffectID != effect {
		return lightingsettings.EffectSettings{}, fmt.Errorf("resolved Scimitar Pro effect settings do not match %q", effect)
	}

	settings := current.Clone()
	settings.SchemaVersion = lightingsettings.SchemaVersion
	settings.EffectID = effect
	settings.Speed = nil
	settings.SingleColor = nil
	settings.TwoColor = nil
	settings.Temperature = nil
	settings.Gradient = nil

	if descriptor.SupportsSpeed {
		speed := profile.Speed
		settings.Speed = &speed
	}

	color := func(value rgb.Color) lightingsettings.Color {
		return lightingsettings.Color{Red: value.Red, Green: value.Green, Blue: value.Blue}
	}
	switch descriptor.PaletteKind {
	case rgb.LightingPaletteNone, rgb.LightingPaletteGenerated:
	case rgb.LightingPaletteStaticSingle:
		settings.SingleColor = &lightingsettings.SingleColorSettings{Color: color(profile.StartColor)}
	case rgb.LightingPaletteTwoColor:
		settings.TwoColor = &lightingsettings.TwoColorSettings{
			Start: color(profile.StartColor),
			End:   color(profile.EndColor),
		}
	case rgb.LightingPaletteTemperatureThree:
		settings.Temperature = &lightingsettings.TemperatureSettings{
			Low: lightingsettings.TemperaturePoint{
				Color: color(profile.StartColor), Celsius: profile.StartColor.Temperature,
			},
			Middle: lightingsettings.TemperaturePoint{
				Color: color(profile.MiddleColor), Celsius: profile.MiddleColor.Temperature,
			},
			High: lightingsettings.TemperaturePoint{
				Color: color(profile.EndColor), Celsius: profile.EndColor.Temperature,
			},
		}
	case rgb.LightingPaletteGradient:
		stops := make([]lightingsettings.GradientStop, 0, len(profile.Gradients))
		for index := 0; index < len(profile.Gradients); index++ {
			value, found := profile.Gradients[index]
			if !found {
				return lightingsettings.EffectSettings{}, fmt.Errorf("Scimitar Pro Gradient stops must use contiguous indexes")
			}
			stops = append(stops, lightingsettings.GradientStop{
				Position:  value.Position,
				Color:     color(value),
				Intensity: value.Brightness,
			})
		}
		settings.Gradient = &lightingsettings.GradientSettings{Stops: stops}
	default:
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported Scimitar Pro effect palette %q", descriptor.PaletteKind)
	}

	if err := lightingsettings.Validate(settings); err != nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("validate Scimitar Pro effect settings: %w", err)
	}
	return settings, nil
}
