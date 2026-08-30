package scimitarrgbelite

import (
	"fmt"
	"sort"
	"strconv"

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

func scimitarEliteLightingColorHex(color lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
}

// LightingSnapshot reads only canonical desired state and resolved canonical
// settings. The returned slices contain presentation copies.
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

	snapshot := lightingpresentation.Snapshot{
		TargetKind:       "native",
		ConfiguredEffect: effect,
		HasBrightness:    true,
		Brightness:       brightness,
		SupportedEffects: make([]LightingEffectOption, 0, len(rgbModes)),
	}
	if d.DeviceProfile != nil {
		snapshot.ClusterControlled = d.DeviceProfile.RGBCluster
		snapshot.ExternalControlled = d.DeviceProfile.OpenRGBIntegration
	}
	for _, candidate := range rgbModes {
		if candidate == "mouse" {
			snapshot.SupportedEffects = append(snapshot.SupportedEffects, LightingEffectOption{ID: candidate, Label: "Mouse"})
			continue
		}
		descriptor, ok := scimitarEliteCanonicalEffectDescriptor(candidate)
		if !ok {
			continue
		}
		snapshot.SupportedEffects = append(snapshot.SupportedEffects, LightingEffectOption{
			ID: candidate, Label: descriptor.Label,
		})
	}

	if effect == "mouse" {
		snapshot.EffectSupported = true
		snapshot.AuthoredZoneEditor = scimitarEliteAuthoredZoneEditor(d.DeviceProfile)
		return snapshot, true
	}
	descriptor, supported := scimitarEliteCanonicalEffectDescriptor(effect)
	snapshot.EffectSupported = supported
	if !supported {
		return snapshot, true
	}
	resolution, err := d.lightingSource.resolveEffectSettingsWithStatus(effect)
	if err != nil || resolution.Settings.EffectID != effect {
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
			snapshot.SingleColorHex = scimitarEliteLightingColorHex(settings.SingleColor.Color)
		}
	case rgb.LightingPaletteTwoColor:
		if settings.TwoColor != nil {
			snapshot.TwoColorStartHex = scimitarEliteLightingColorHex(settings.TwoColor.Start)
			snapshot.TwoColorEndHex = scimitarEliteLightingColorHex(settings.TwoColor.End)
		}
	case rgb.LightingPaletteTemperatureThree:
		if descriptor.TemperaturePoints == rgb.SoftwareEffectTemperaturePointsLowMiddleHigh && settings.Temperature != nil {
			temperature := settings.Temperature
			snapshot.HasTemperature = true
			snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: scimitarEliteLightingColorHex(temperature.Low.Color), Celsius: temperature.Low.Celsius}
			snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: scimitarEliteLightingColorHex(temperature.Middle.Color), Celsius: temperature.Middle.Celsius}
			snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: scimitarEliteLightingColorHex(temperature.High.Color), Celsius: temperature.High.Celsius}
		}
	case rgb.LightingPaletteGradient:
		if settings.Gradient != nil {
			snapshot.HasGradient = true
			snapshot.GradientStops = make([]lightingpresentation.GradientStop, len(settings.Gradient.Stops))
			for index, stop := range settings.Gradient.Stops {
				snapshot.GradientStops[index] = lightingpresentation.GradientStop{Position: stop.Position, ColorHex: scimitarEliteLightingColorHex(stop.Color), Intensity: stop.Intensity}
			}
		}
	}
	return snapshot, true
}

func scimitarEliteAuthoredZoneEditor(profile *DeviceProfile) *lightingpresentation.AuthoredZoneEditor {
	if profile == nil || len(profile.ZoneColors) == 0 {
		return nil
	}
	keys := make([]int, 0, len(profile.ZoneColors))
	for key := range profile.ZoneColors {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	editor := &lightingpresentation.AuthoredZoneEditor{
		EffectID:    "mouse",
		Heading:     "Zones",
		Description: "Select one or more zones, choose a color, then apply it to the selected zones.",
		Zones:       make([]lightingpresentation.AuthoredZone, 0, len(keys)),
	}
	for _, key := range keys {
		zone := profile.ZoneColors[key]
		if zone.Color == nil {
			continue
		}
		editor.Zones = append(editor.Zones, lightingpresentation.AuthoredZone{
			ID: strconv.Itoa(key), Label: zone.Name,
			ColorHex: fmt.Sprintf("#%02x%02x%02x", uint8(zone.Color.Red), uint8(zone.Color.Green), uint8(zone.Color.Blue)),
		})
	}
	return editor
}
