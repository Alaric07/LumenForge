package openrgbimport

import (
	"fmt"
	"slices"

	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

// Compatibility aliases keep existing package-local consumers source-compatible
// while shared presentation types are the contract returned by LightingSnapshot.
type LightingEffectOption = lightingpresentation.EffectOption
type LightingTemperaturePointSnapshot = lightingpresentation.TemperaturePoint
type LightingGradientStopSnapshot = lightingpresentation.GradientStop
type LightingSnapshot = lightingpresentation.Snapshot

// LightingDeviceID identifies this imported device for Devices lighting.
func (d *Device) LightingDeviceID() string {
	if d == nil {
		return ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.Serial
}

func lightingColorHex(color lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
}

// LightingSnapshot returns a complete race-safe value snapshot. Selected
// effect, Brightness, and effect settings come from the cut-over target state
// and canonical resolver.
func (d *Device) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	if d == nil {
		return lightingpresentation.Snapshot{}, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.IsOpenRGB || d.lifecycleInactiveLocked() {
		return lightingpresentation.Snapshot{}, false
	}

	snapshot := lightingpresentation.Snapshot{
		TargetKind:       "openrgb",
		SupportedEffects: make([]LightingEffectOption, 0, len(d.RGBModes)),
	}
	for _, effect := range d.RGBModes {
		option := LightingEffectOption{ID: effect}
		if descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect); ok {
			option.Label = descriptor.Label
		}
		snapshot.SupportedEffects = append(snapshot.SupportedEffects, option)
	}

	profile := d.DeviceProfile
	snapshot.ConfiguredEffect = d.effect
	if snapshot.ConfiguredEffect == "" {
		snapshot.ConfiguredEffect = defaultDeviceLightingEffect
	}
	snapshot.EffectSupported = slices.Contains(d.RGBModes, snapshot.ConfiguredEffect)
	snapshot.HasBrightness = true
	snapshot.Brightness = d.brightness
	if profile != nil {
		snapshot.ClusterControlled = profile.RGBCluster
	}

	descriptor, known := rgb.SoftwareEffectDescriptorByID(snapshot.ConfiguredEffect)
	if !snapshot.EffectSupported || !known || d.lightingResolver == nil {
		return snapshot, true
	}
	resolution, err := d.resolveLightingSettings(snapshot.ConfiguredEffect)
	if err != nil {
		return snapshot, true
	}

	snapshot.PaletteKind = string(descriptor.PaletteKind)
	snapshot.Customized = resolution.Customized
	if descriptor.SupportsSpeed && resolution.Settings.Speed != nil {
		snapshot.HasSpeed = true
		snapshot.Speed = *resolution.Settings.Speed
	}
	if descriptor.PaletteKind == rgb.LightingPaletteStaticSingle && resolution.Settings.SingleColor != nil {
		snapshot.SingleColorHex = lightingColorHex(resolution.Settings.SingleColor.Color)
	}
	if descriptor.PaletteKind == rgb.LightingPaletteTwoColor && resolution.Settings.TwoColor != nil {
		snapshot.TwoColorStartHex = lightingColorHex(resolution.Settings.TwoColor.Start)
		snapshot.TwoColorEndHex = lightingColorHex(resolution.Settings.TwoColor.End)
	}
	if descriptor.PaletteKind == rgb.LightingPaletteTemperatureThree &&
		descriptor.TemperaturePoints == rgb.SoftwareEffectTemperaturePointsLowMiddleHigh &&
		resolution.Settings.Temperature != nil {
		temperature := resolution.Settings.Temperature
		snapshot.HasTemperature = true
		snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{
			ColorHex: lightingColorHex(temperature.Low.Color), Celsius: temperature.Low.Celsius,
		}
		snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{
			ColorHex: lightingColorHex(temperature.Middle.Color), Celsius: temperature.Middle.Celsius,
		}
		snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{
			ColorHex: lightingColorHex(temperature.High.Color), Celsius: temperature.High.Celsius,
		}
	}
	if descriptor.PaletteKind == rgb.LightingPaletteGradient && resolution.Settings.Gradient != nil {
		snapshot.HasGradient = true
		snapshot.GradientStops = make([]lightingpresentation.GradientStop, len(resolution.Settings.Gradient.Stops))
		for index, stop := range resolution.Settings.Gradient.Stops {
			snapshot.GradientStops[index] = lightingpresentation.GradientStop{
				Position: stop.Position, ColorHex: lightingColorHex(stop.Color), Intensity: stop.Intensity,
			}
		}
	}

	return snapshot, true
}
