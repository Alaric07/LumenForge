package cluster

import (
	"fmt"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

// SetLightingEffect persists the canonical RGB Cluster selected effect before
// reapplying output.
func (d *Device) SetLightingEffect(effect string) error {
	if !d.runtimeAvailable() {
		return fmt.Errorf("RGB Cluster lighting runtime is unavailable")
	}
	d.lightingMutationMu.Lock()
	defer d.lightingMutationMu.Unlock()

	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeCluster) {
		return fmt.Errorf("effect %q is not supported by RGB Cluster", effect)
	}
	if _, err := d.resolver.Resolve(lightingsettings.RGBCluster(), effect); err != nil {
		return fmt.Errorf("resolve RGB Cluster effect %q: %w", effect, err)
	}
	state, err := d.lightingState.Snapshot()
	if err != nil {
		return err
	}
	state.SelectedEffect = effect
	if err = d.lightingState.Set(state); err != nil {
		return err
	}
	if err = d.refreshCompatibilityProjection(); err != nil {
		return err
	}
	d.restartWorker()
	return nil
}

// SetLightingBrightness persists desired RGB Cluster Brightness independently
// from the selected effect's canonical settings.
func (d *Device) SetLightingBrightness(brightness uint8) error {
	if !d.runtimeAvailable() {
		return fmt.Errorf("RGB Cluster lighting runtime is unavailable")
	}
	if brightness > 100 {
		return fmt.Errorf("RGB Cluster brightness must be between 0 and 100")
	}
	d.lightingMutationMu.Lock()
	defer d.lightingMutationMu.Unlock()

	state, err := d.lightingState.Snapshot()
	if err != nil {
		return err
	}
	state.Brightness = brightness
	if err = d.lightingState.Set(state); err != nil {
		return err
	}
	if err = d.refreshCompatibilityProjection(); err != nil {
		return err
	}
	d.restartWorker()
	return nil
}

// SetLightingSpeed replaces only the selected effect's renderer-consumed Speed.
func (d *Device) SetLightingSpeed(expectedEffect string, speed float64) error {
	return d.mutateSelectedLightingSettings(expectedEffect, func(descriptor rgb.SoftwareEffectDescriptor) error {
		if !descriptor.SupportsSpeed {
			return fmt.Errorf("effect %q does not support Speed", expectedEffect)
		}
		return nil
	}, func(settings *lightingsettings.EffectSettings) {
		settings.Speed = &speed
	})
}

// SetLightingSingleColor replaces the complete selected single-color palette.
func (d *Device) SetLightingSingleColor(expectedEffect string, color lightingsettings.Color) error {
	return d.mutateSelectedLightingSettings(expectedEffect, requireClusterPalette(expectedEffect, rgb.LightingPaletteStaticSingle), func(settings *lightingsettings.EffectSettings) {
		settings.SingleColor = &lightingsettings.SingleColorSettings{Color: color}
	})
}

// SetLightingTwoColor replaces the complete selected Start/End palette.
func (d *Device) SetLightingTwoColor(expectedEffect string, start, end lightingsettings.Color) error {
	return d.mutateSelectedLightingSettings(expectedEffect, requireClusterPalette(expectedEffect, rgb.LightingPaletteTwoColor), func(settings *lightingsettings.EffectSettings) {
		settings.TwoColor = &lightingsettings.TwoColorSettings{Start: start, End: end}
	})
}

// SetLightingTemperature replaces the complete selected Low/Middle/High contract.
func (d *Device) SetLightingTemperature(expectedEffect string, low, middle, high lightingsettings.TemperaturePoint) error {
	return d.mutateSelectedLightingSettings(expectedEffect, requireClusterPalette(expectedEffect, rgb.LightingPaletteTemperatureThree), func(settings *lightingsettings.EffectSettings) {
		settings.Temperature = &lightingsettings.TemperatureSettings{Low: low, Middle: middle, High: high}
	})
}

// SetLightingGradient replaces the complete selected ordered Gradient contract.
func (d *Device) SetLightingGradient(expectedEffect string, stops []lightingsettings.GradientStop) error {
	return d.mutateSelectedLightingSettings(expectedEffect, requireClusterPalette(expectedEffect, rgb.LightingPaletteGradient), func(settings *lightingsettings.EffectSettings) {
		settings.Gradient = &lightingsettings.GradientSettings{Stops: append([]lightingsettings.GradientStop(nil), stops...)}
	})
}

func requireClusterPalette(effect string, palette rgb.LightingPaletteKind) func(rgb.SoftwareEffectDescriptor) error {
	return func(descriptor rgb.SoftwareEffectDescriptor) error {
		if descriptor.PaletteKind != palette {
			return fmt.Errorf("effect %q does not use the requested palette", effect)
		}
		return nil
	}
}

func (d *Device) mutateSelectedLightingSettings(
	expectedEffect string,
	requireCapability func(rgb.SoftwareEffectDescriptor) error,
	mutate func(*lightingsettings.EffectSettings),
) error {
	if !d.runtimeAvailable() {
		return fmt.Errorf("RGB Cluster lighting runtime is unavailable")
	}
	d.lightingMutationMu.Lock()
	defer d.lightingMutationMu.Unlock()

	state, err := d.lightingState.Snapshot()
	if err != nil {
		return err
	}
	if state.SelectedEffect != expectedEffect {
		return fmt.Errorf("RGB Cluster selected effect changed")
	}
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(expectedEffect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeCluster) {
		return fmt.Errorf("effect %q is not supported by RGB Cluster", expectedEffect)
	}
	if requireCapability == nil || mutate == nil {
		return fmt.Errorf("RGB Cluster lighting mutation is unavailable")
	}
	if err = requireCapability(descriptor); err != nil {
		return err
	}
	resolution, err := d.resolver.Resolve(lightingsettings.RGBCluster(), expectedEffect)
	if err != nil {
		return err
	}
	settings := resolution.Settings.Clone()
	mutate(&settings)
	if err = lightingsettings.Validate(settings); err != nil {
		return err
	}
	if err = d.effects.Set(expectedEffect, settings); err != nil {
		return err
	}
	if err = d.refreshCompatibilityProjection(); err != nil {
		return err
	}
	d.restartWorker()
	return nil
}
