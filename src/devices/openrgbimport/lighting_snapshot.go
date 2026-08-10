package openrgbimport

import (
	"fmt"
	"slices"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

// LightingEffectOption is a presentation-safe copy of one effect supported by
// an imported controller.
type LightingEffectOption struct {
	ID    string
	Label string
}

// LightingTemperaturePointSnapshot is a presentation-safe copy of one
// canonical semantic temperature point.
type LightingTemperaturePointSnapshot struct {
	ColorHex string
	Celsius  float64
}

// LightingSnapshot is an immutable presentation/configuration view of an
// imported controller's lighting state. It does not confirm live hardware
// output.
type LightingSnapshot struct {
	ConfiguredEffect  string
	EffectSupported   bool
	SupportedEffects  []LightingEffectOption
	HasBrightness     bool
	Brightness        uint8
	HasSpeed          bool
	Speed             float64
	ClusterControlled bool
	PaletteKind       string
	SingleColorHex    string
	TwoColorStartHex  string
	TwoColorEndHex    string
	HasTemperature    bool
	TemperatureLow    LightingTemperaturePointSnapshot
	TemperatureMiddle LightingTemperaturePointSnapshot
	TemperatureHigh   LightingTemperaturePointSnapshot
	Customized        bool
}

func lightingColorHex(color lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
}

// LightingSnapshot returns a complete race-safe value snapshot. Selected
// effect, Brightness, and effect settings come from the cut-over target state
// and canonical resolver.
func (d *Device) LightingSnapshot() (LightingSnapshot, bool) {
	if d == nil {
		return LightingSnapshot{}, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.IsOpenRGB || d.lifecycleInactiveLocked() {
		return LightingSnapshot{}, false
	}

	snapshot := LightingSnapshot{
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
		snapshot.TemperatureLow = LightingTemperaturePointSnapshot{
			ColorHex: lightingColorHex(temperature.Low.Color), Celsius: temperature.Low.Celsius,
		}
		snapshot.TemperatureMiddle = LightingTemperaturePointSnapshot{
			ColorHex: lightingColorHex(temperature.Middle.Color), Celsius: temperature.Middle.Celsius,
		}
		snapshot.TemperatureHigh = LightingTemperaturePointSnapshot{
			ColorHex: lightingColorHex(temperature.High.Color), Celsius: temperature.High.Celsius,
		}
	}

	return snapshot, true
}
