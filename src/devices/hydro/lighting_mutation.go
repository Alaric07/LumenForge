package hydro

import (
	"fmt"

	"LumenForge/src/lightingsettings"
)

func (d *Device) SetLightingBrightness(brightness uint8) error {
	if brightness > 100 {
		return fmt.Errorf("Hydro brightness is invalid")
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	state, err := d.canonicalLightingState()
	if err != nil {
		return err
	}
	state.Brightness = brightness
	if err = d.lightingRuntime.State.Set(d.Serial, state); err != nil {
		return fmt.Errorf("persist Hydro brightness: %w", err)
	}
	d.setConfiguration()
	return nil
}

func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if effect != "static" {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported Hydro effect %q", effect)
	}
	resolution, err := d.resolveCanonicalStatic()
	if err != nil {
		return lightingsettings.EffectSettings{}, err
	}
	return resolution.Settings.Clone(), nil
}

func (d *Device) SetLightingEffectSettings(effect string, settings lightingsettings.EffectSettings) error {
	if effect != "static" || settings.EffectID != "static" || settings.SingleColor == nil {
		return fmt.Errorf("invalid Hydro static settings")
	}
	if err := lightingsettings.Validate(settings); err != nil {
		return fmt.Errorf("validate Hydro static settings: %w", err)
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	if _, err := d.canonicalLightingState(); err != nil {
		return err
	}
	if err := d.lightingRuntime.Effects.Set(d.Serial, effect, settings.Clone()); err != nil {
		return fmt.Errorf("persist Hydro static settings: %w", err)
	}
	d.setConfiguration()
	return nil
}

func (d *Device) ResetLightingEffectSettings(effect string) error {
	if effect != "static" {
		return fmt.Errorf("unsupported Hydro effect %q", effect)
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	if _, err := d.canonicalLightingState(); err != nil {
		return err
	}
	deleted, err := d.lightingRuntime.Effects.Delete(d.Serial, effect)
	if err != nil {
		return fmt.Errorf("reset Hydro static settings: %w", err)
	}
	if deleted {
		d.setConfiguration()
	}
	return nil
}
