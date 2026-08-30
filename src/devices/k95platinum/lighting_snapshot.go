package k95platinum

import (
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"fmt"
)

type LightingSnapshot = lightingpresentation.Snapshot

func k95LightingColorHex(c lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(c.Red), uint8(c.Green), uint8(c.Blue))
}
func (d *Device) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	if d == nil || d.lightingSource == nil {
		return lightingpresentation.Snapshot{}, false
	}
	effect, err := d.currentCanonicalSelectedEffect()
	if err != nil {
		return lightingpresentation.Snapshot{}, false
	}
	brightness, err := d.currentCanonicalBrightness()
	if err != nil {
		return lightingpresentation.Snapshot{}, false
	}
	value := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: effect, HasBrightness: true, Brightness: brightness, SupportedEffects: make([]lightingpresentation.EffectOption, 0, len(rgbModes))}
	if d.DeviceProfile != nil {
		value.ClusterControlled = d.DeviceProfile.RGBCluster
	}
	for _, candidate := range rgbModes {
		if candidate == "keyboard" {
			value.SupportedEffects = append(value.SupportedEffects, lightingpresentation.EffectOption{ID: candidate, Label: "Keyboard"})
			continue
		}
		if descriptor, ok := k95CanonicalEffectDescriptor(candidate); ok {
			value.SupportedEffects = append(value.SupportedEffects, lightingpresentation.EffectOption{ID: candidate, Label: descriptor.Label})
		}
	}
	if effect == "keyboard" {
		value.EffectSupported = true
		return value, true
	}
	descriptor, ok := k95CanonicalEffectDescriptor(effect)
	value.EffectSupported = ok
	if !ok {
		return value, true
	}
	resolution, err := d.lightingSource.resolveEffectSettingsWithStatus(effect)
	if err != nil || resolution.Settings.EffectID != effect {
		return lightingpresentation.Snapshot{}, false
	}
	settings := resolution.Settings
	value.Customized = resolution.Customized
	value.PaletteKind = string(descriptor.PaletteKind)
	if descriptor.SupportsSpeed && settings.Speed != nil {
		value.HasSpeed = true
		value.Speed = *settings.Speed
	}
	switch descriptor.PaletteKind {
	case rgb.LightingPaletteStaticSingle:
		if settings.SingleColor != nil {
			value.SingleColorHex = k95LightingColorHex(settings.SingleColor.Color)
		}
	case rgb.LightingPaletteTwoColor:
		if settings.TwoColor != nil {
			value.TwoColorStartHex = k95LightingColorHex(settings.TwoColor.Start)
			value.TwoColorEndHex = k95LightingColorHex(settings.TwoColor.End)
		}
	case rgb.LightingPaletteTemperatureThree:
		if settings.Temperature != nil {
			t := settings.Temperature
			value.HasTemperature = true
			value.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: k95LightingColorHex(t.Low.Color), Celsius: t.Low.Celsius}
			value.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: k95LightingColorHex(t.Middle.Color), Celsius: t.Middle.Celsius}
			value.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: k95LightingColorHex(t.High.Color), Celsius: t.High.Celsius}
		}
	case rgb.LightingPaletteGradient:
		if settings.Gradient != nil {
			value.HasGradient = true
			value.GradientStops = make([]lightingpresentation.GradientStop, len(settings.Gradient.Stops))
			for i, s := range settings.Gradient.Stops {
				value.GradientStops[i] = lightingpresentation.GradientStop{Position: s.Position, ColorHex: k95LightingColorHex(s.Color), Intensity: s.Intensity}
			}
		}
		return value, true
	}
	return value, true
}
