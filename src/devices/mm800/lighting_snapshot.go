package mm800

import (
	"fmt"

	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

// Compatibility aliases keep package-local consumers source-compatible while
// shared presentation types remain the LightingSnapshot contract.
type LightingEffectOption = lightingpresentation.EffectOption
type LightingTemperaturePointSnapshot = lightingpresentation.TemperaturePoint
type LightingGradientStopSnapshot = lightingpresentation.GradientStop
type LightingSnapshot = lightingpresentation.Snapshot

func mm800LightingColorHex(color lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
}

// LightingSnapshot reads canonical desired state and resolved canonical
// settings. Mousepad remains a device-specific authored mode, so it has no
// shared effect-settings presentation. Returned slices contain presentation
// copies.
func (d *Device) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	if d == nil || d.lightingRuntime == nil || d.lightingRuntime.Resolver == nil {
		return lightingpresentation.Snapshot{}, false
	}

	state, err := d.canonicalLightingState()
	if err != nil {
		return lightingpresentation.Snapshot{}, false
	}

	snapshot := lightingpresentation.Snapshot{
		TargetKind:       "native",
		ConfiguredEffect: state.SelectedEffect,
		HasBrightness:    true,
		Brightness:       state.Brightness,
		SupportedEffects: make([]lightingpresentation.EffectOption, 0, len(rgbModes)),
	}
	if d.DeviceProfile != nil {
		snapshot.ClusterControlled = d.DeviceProfile.RGBCluster
		snapshot.ExternalControlled = d.DeviceProfile.OpenRGBIntegration
	}

	for _, effect := range rgbModes {
		if effect == "mousepad" {
			snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{
				ID: "mousepad", Label: "Mousepad",
			})
			continue
		}
		descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
		if !ok || !descriptor.Scope.Includes(rgb.EffectScopeDevice) {
			continue
		}
		snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{
			ID: effect, Label: descriptor.Label,
		})
	}

	if state.SelectedEffect == "mousepad" {
		snapshot.EffectSupported = true
		return snapshot, true
	}
	descriptor, supported := rgb.SoftwareEffectDescriptorByID(state.SelectedEffect)
	snapshot.EffectSupported = supported && descriptor.Scope.Includes(rgb.EffectScopeDevice)
	if !snapshot.EffectSupported {
		return snapshot, true
	}

	resolution, err := d.lightingRuntime.Resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), state.SelectedEffect)
	if err != nil || resolution.Settings.EffectID != state.SelectedEffect {
		return lightingpresentation.Snapshot{}, false
	}
	settings := resolution.Settings.Clone()
	snapshot.Customized = resolution.Customized
	snapshot.PaletteKind = string(descriptor.PaletteKind)
	if descriptor.SupportsSpeed && settings.Speed != nil {
		snapshot.HasSpeed = true
		snapshot.Speed = *settings.Speed
	}

	switch descriptor.PaletteKind {
	case rgb.LightingPaletteStaticSingle:
		if settings.SingleColor != nil {
			snapshot.SingleColorHex = mm800LightingColorHex(settings.SingleColor.Color)
		}
	case rgb.LightingPaletteTwoColor:
		if settings.TwoColor != nil {
			snapshot.TwoColorStartHex = mm800LightingColorHex(settings.TwoColor.Start)
			snapshot.TwoColorEndHex = mm800LightingColorHex(settings.TwoColor.End)
		}
	case rgb.LightingPaletteTemperatureThree:
		if descriptor.TemperaturePoints == rgb.SoftwareEffectTemperaturePointsLowMiddleHigh && settings.Temperature != nil {
			temperature := settings.Temperature
			snapshot.HasTemperature = true
			snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: mm800LightingColorHex(temperature.Low.Color), Celsius: temperature.Low.Celsius}
			snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: mm800LightingColorHex(temperature.Middle.Color), Celsius: temperature.Middle.Celsius}
			snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: mm800LightingColorHex(temperature.High.Color), Celsius: temperature.High.Celsius}
		}
	case rgb.LightingPaletteGradient:
		if settings.Gradient != nil {
			snapshot.HasGradient = true
			snapshot.GradientStops = make([]lightingpresentation.GradientStop, len(settings.Gradient.Stops))
			for index, stop := range settings.Gradient.Stops {
				snapshot.GradientStops[index] = lightingpresentation.GradientStop{Position: stop.Position, ColorHex: mm800LightingColorHex(stop.Color), Intensity: stop.Intensity}
			}
		}
	}

	return snapshot, true
}
