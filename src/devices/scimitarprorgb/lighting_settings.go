package scimitarprorgb

import (
	"fmt"
	"slices"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

func scimitarCanonicalEffectDescriptor(effect string) (rgb.SoftwareEffectDescriptor, bool) {
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeDevice) || !slices.Contains(rgbModes, effect) {
		return rgb.SoftwareEffectDescriptor{}, false
	}
	return descriptor, true
}

type scimitarGradientMutation func([]lightingsettings.GradientStop) ([]lightingsettings.GradientStop, uint, uint8, error)

func (d *Device) mutateCanonicalGradient(effect string, mutate scimitarGradientMutation) (bool, uint8, uint, error) {
	if d == nil || d.lightingSource == nil {
		return false, 0, 0, fmt.Errorf("Scimitar Pro canonical lighting source is unavailable")
	}
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeDevice) || descriptor.PaletteKind != rgb.LightingPaletteGradient {
		return false, 0, 0, fmt.Errorf("Scimitar Pro effect %q does not support Gradient customization", effect)
	}
	current, err := d.lightingSource.resolveEffectSettings(effect)
	if err != nil || current.Gradient == nil {
		if err != nil {
			return false, 0, 0, fmt.Errorf("resolve Scimitar Pro Gradient settings: %w", err)
		}
		return false, 0, 0, fmt.Errorf("Scimitar Pro Gradient settings are incomplete")
	}
	stops := append([]lightingsettings.GradientStop(nil), current.Gradient.Stops...)
	stops, index, status, err := mutate(stops)
	if err != nil || status != 1 {
		return false, status, index, err
	}
	settings := current.Clone()
	settings.Gradient = &lightingsettings.GradientSettings{Stops: append([]lightingsettings.GradientStop(nil), stops...)}
	if err = lightingsettings.Validate(settings); err != nil {
		return false, 0, 0, fmt.Errorf("validate Scimitar Pro Gradient settings: %w", err)
	}
	selected, err := d.lightingSource.selectedEffect()
	if err != nil {
		return false, 0, 0, fmt.Errorf("resolve Scimitar Pro selected effect: %w", err)
	}
	if err = d.lightingSource.setEffectSettings(effect, settings); err != nil {
		return false, 0, 0, fmt.Errorf("persist Scimitar Pro Gradient settings: %w", err)
	}
	return selected == effect, 1, index, nil
}

func (d *Device) addCanonicalGradientStop(effect string) (bool, uint8, uint, error) {
	return d.mutateCanonicalGradient(effect, func(stops []lightingsettings.GradientStop) ([]lightingsettings.GradientStop, uint, uint8, error) {
		if len(stops) == 0 {
			return nil, 0, 0, fmt.Errorf("Scimitar Pro Gradient settings are empty")
		}
		index := uint(len(stops))
		stops = append(stops, lightingsettings.GradientStop{Position: stops[len(stops)-1].Position, Color: lightingsettings.Color{Red: 0, Green: 255, Blue: 255}, Intensity: 0})
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
