package scimitarrgbelite

import (
	"fmt"
	"slices"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

func scimitarEliteCanonicalEffectDescriptor(effect string) (rgb.SoftwareEffectDescriptor, bool) {
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeDevice) || !slices.Contains(rgbModes, effect) {
		return rgb.SoftwareEffectDescriptor{}, false
	}
	return descriptor, true
}

func scimitarEliteSupportsEffect(effect string) bool {
	if effect == "mouse" {
		return slices.Contains(rgbModes, effect)
	}
	_, ok := scimitarEliteCanonicalEffectDescriptor(effect)
	return ok
}

// scimitarEliteRGBProfileFromCanonicalSettings adapts a resolved canonical effect
// into the legacy-shaped presentation contract used by the existing RGB UI.
// It does not expose or restore legacy profile authority.
func scimitarEliteRGBProfileFromCanonicalSettings(
	effect string,
	settings lightingsettings.EffectSettings,
) (*rgb.Profile, error) {
	descriptor, ok := scimitarEliteCanonicalEffectDescriptor(effect)
	if !ok {
		return nil, fmt.Errorf("unsupported Scimitar RGB Elite effect %q", effect)
	}
	if settings.EffectID != effect {
		return nil, fmt.Errorf("resolved Scimitar RGB Elite effect settings do not match %q", effect)
	}

	profile := &rgb.Profile{}
	if descriptor.SupportsSpeed {
		if settings.Speed == nil {
			return nil, fmt.Errorf("Scimitar RGB Elite effect %q is missing Speed", effect)
		}
		profile.Speed = *settings.Speed
	}

	color := func(value lightingsettings.Color) rgb.Color {
		return rgb.Color{Red: value.Red, Green: value.Green, Blue: value.Blue}
	}
	switch descriptor.PaletteKind {
	case rgb.LightingPaletteNone, rgb.LightingPaletteGenerated:
	case rgb.LightingPaletteStaticSingle:
		if settings.SingleColor == nil {
			return nil, fmt.Errorf("Scimitar RGB Elite effect %q is missing its single color", effect)
		}
		profile.StartColor = color(settings.SingleColor.Color)
	case rgb.LightingPaletteTwoColor:
		if settings.TwoColor == nil {
			return nil, fmt.Errorf("Scimitar RGB Elite effect %q is missing its two colors", effect)
		}
		profile.StartColor = color(settings.TwoColor.Start)
		profile.EndColor = color(settings.TwoColor.End)
	case rgb.LightingPaletteTemperatureThree:
		if settings.Temperature == nil {
			return nil, fmt.Errorf("Scimitar RGB Elite effect %q is missing its temperature colors", effect)
		}
		profile.StartColor = color(settings.Temperature.Low.Color)
		profile.StartColor.Temperature = settings.Temperature.Low.Celsius
		profile.MiddleColor = color(settings.Temperature.Middle.Color)
		profile.MiddleColor.Temperature = settings.Temperature.Middle.Celsius
		profile.EndColor = color(settings.Temperature.High.Color)
		profile.EndColor.Temperature = settings.Temperature.High.Celsius
	case rgb.LightingPaletteGradient:
		if settings.Gradient == nil {
			return nil, fmt.Errorf("Scimitar RGB Elite effect %q is missing its Gradient stops", effect)
		}
		profile.Gradients = make(map[int]rgb.Color, len(settings.Gradient.Stops))
		for index, stop := range settings.Gradient.Stops {
			gradient := color(stop.Color)
			gradient.Position = stop.Position
			gradient.Brightness = stop.Intensity
			profile.Gradients[index] = gradient
		}
	default:
		return nil, fmt.Errorf("unsupported Scimitar RGB Elite effect palette %q", descriptor.PaletteKind)
	}

	return profile, nil
}

// replaceCanonicalEffectSettings converts the complete legacy editor payload
// into canonical effect settings and persists it before reporting whether the
// changed effect is currently selected.
func (d *Device) replaceCanonicalEffectSettings(effect string, profile rgb.Profile) (bool, error) {
	if d == nil || d.lightingSource == nil {
		return false, fmt.Errorf("Scimitar RGB Elite canonical lighting source is unavailable")
	}

	current, err := d.lightingSource.resolveEffectSettings(effect)
	if err != nil {
		return false, fmt.Errorf("resolve Scimitar RGB Elite effect settings: %w", err)
	}
	settings, err := scimitarEliteEffectSettingsFromRGBProfile(effect, profile, current)
	if err != nil {
		return false, err
	}
	selected, err := d.lightingSource.selectedEffect()
	if err != nil {
		return false, fmt.Errorf("resolve Scimitar RGB Elite selected effect: %w", err)
	}
	if err = d.lightingSource.setEffectSettings(effect, settings); err != nil {
		return false, fmt.Errorf("persist Scimitar RGB Elite effect settings: %w", err)
	}
	return selected == effect, nil
}

func scimitarEliteEffectSettingsFromRGBProfile(
	effect string,
	profile rgb.Profile,
	current lightingsettings.EffectSettings,
) (lightingsettings.EffectSettings, error) {
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeDevice) {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported Scimitar RGB Elite effect %q", effect)
	}
	if current.EffectID != effect {
		return lightingsettings.EffectSettings{}, fmt.Errorf("resolved Scimitar RGB Elite effect settings do not match %q", effect)
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
				return lightingsettings.EffectSettings{}, fmt.Errorf("Scimitar RGB Elite Gradient stops must use contiguous indexes")
			}
			stops = append(stops, lightingsettings.GradientStop{
				Position:  value.Position,
				Color:     color(value),
				Intensity: value.Brightness,
			})
		}
		settings.Gradient = &lightingsettings.GradientSettings{Stops: stops}
	default:
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported Scimitar RGB Elite effect palette %q", descriptor.PaletteKind)
	}

	if err := lightingsettings.Validate(settings); err != nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("validate Scimitar RGB Elite effect settings: %w", err)
	}
	return settings, nil
}

type scimitarEliteGradientMutation func([]lightingsettings.GradientStop) ([]lightingsettings.GradientStop, uint, uint8, error)

// mutateCanonicalGradient applies one compatibility gradient mutation to a
// defensive canonical copy. Its returned index is the canonical ordered-stop
// index used by the legacy add/delete response contract.
func (d *Device) mutateCanonicalGradient(
	effect string,
	mutate scimitarEliteGradientMutation,
) (bool, uint8, uint, error) {
	if d == nil || d.lightingSource == nil {
		return false, 0, 0, fmt.Errorf("Scimitar RGB Elite canonical lighting source is unavailable")
	}
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeDevice) || descriptor.PaletteKind != rgb.LightingPaletteGradient {
		return false, 0, 0, fmt.Errorf("Scimitar RGB Elite effect %q does not support Gradient customization", effect)
	}

	current, err := d.lightingSource.resolveEffectSettings(effect)
	if err != nil {
		return false, 0, 0, fmt.Errorf("resolve Scimitar RGB Elite Gradient settings: %w", err)
	}
	if current.Gradient == nil {
		return false, 0, 0, fmt.Errorf("Scimitar RGB Elite Gradient settings are incomplete")
	}
	stops := append([]lightingsettings.GradientStop(nil), current.Gradient.Stops...)
	stops, index, status, err := mutate(stops)
	if err != nil || status != 1 {
		return false, status, index, err
	}

	settings := current.Clone()
	settings.Gradient = &lightingsettings.GradientSettings{
		Stops: append([]lightingsettings.GradientStop(nil), stops...),
	}
	if err = lightingsettings.Validate(settings); err != nil {
		return false, 0, 0, fmt.Errorf("validate Scimitar RGB Elite Gradient settings: %w", err)
	}
	selected, err := d.lightingSource.selectedEffect()
	if err != nil {
		return false, 0, 0, fmt.Errorf("resolve Scimitar RGB Elite selected effect: %w", err)
	}
	if err = d.lightingSource.setEffectSettings(effect, settings); err != nil {
		return false, 0, 0, fmt.Errorf("persist Scimitar RGB Elite Gradient settings: %w", err)
	}
	return selected == effect, 1, index, nil
}

func (d *Device) addCanonicalGradientStop(effect string) (bool, uint8, uint, error) {
	return d.mutateCanonicalGradient(effect, func(stops []lightingsettings.GradientStop) ([]lightingsettings.GradientStop, uint, uint8, error) {
		if len(stops) == 0 {
			return nil, 0, 0, fmt.Errorf("Scimitar RGB Elite Gradient settings are empty")
		}
		index := uint(len(stops))
		position := stops[len(stops)-1].Position
		stops = append(stops, lightingsettings.GradientStop{
			Position:  position,
			Color:     lightingsettings.Color{Red: 0, Green: 255, Blue: 255},
			Intensity: 0,
		})
		return stops, index, 1, nil
	})
}

func (d *Device) deleteCanonicalGradientStop(effect string) (bool, uint8, uint, error) {
	return d.mutateCanonicalGradient(effect, func(stops []lightingsettings.GradientStop) ([]lightingsettings.GradientStop, uint, uint8, error) {
		if len(stops) < 3 {
			return stops, 0, 2, nil
		}
		index := uint(len(stops) - 1)
		return append([]lightingsettings.GradientStop(nil), stops[:len(stops)-1]...), index, 1, nil
	})
}
