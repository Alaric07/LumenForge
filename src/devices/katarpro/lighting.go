package katarpro

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"

	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

// KATAR PRO deliberately adapts its existing active device profile and RGB file.
// There is no second desired-state or effect-settings database. The shared
// defaults repository supplies immutable reset values only.
func (d *Device) attachLightingRuntime(paths config.Paths) error {
	defaults, err := lightingsettings.LoadDefaultRepository(filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		return err
	}
	if d.DeviceProfile == nil || d.DeviceProfile.BrightnessSlider == nil || *d.DeviceProfile.BrightnessSlider > 100 || d.Rgb == nil {
		return fmt.Errorf("KATAR PRO lighting backing state is unavailable")
	}
	zone, ok := d.DeviceProfile.ZoneColors[0]
	if !ok || zone.Color == nil || len(d.DeviceProfile.ZoneColors) != 1 || !slices.Equal(zone.ColorIndex, []int{0, 1, 2}) {
		return fmt.Errorf("KATAR PRO Scroll zone is unavailable")
	}
	for _, effect := range rgbModes {
		if effect == "mouse" {
			continue
		}
		profile := d.GetRgbProfile(effect)
		if profile == nil {
			return fmt.Errorf("KATAR PRO lighting profile %q is unavailable", effect)
		}
		if _, err := lightingsettings.EffectSettingsFromRGBProfile(effect, *profile); err != nil {
			return err
		}
	}
	d.lightingDefaults = defaults
	// Retire stale persisted darkness; explicit user intent lives in the runtime.
	d.DeviceProfile.RgbOff = false
	return nil
}

func (d *Device) LightingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) SupportsLightingEffect(effect string) bool { return slices.Contains(rgbModes, effect) }

// Called under lightingMu. No mutation may precede this live/runtime guard.
func (d *Device) lightingReady() error {
	if d == nil || d.Serial == "" || d.LEDChannels != 1 || d.dev == nil || !d.Connected || d.Exit || d.lightingDefaults == nil || d.DeviceProfile == nil || d.DeviceProfile.BrightnessSlider == nil || d.Rgb == nil || *d.DeviceProfile.BrightnessSlider > 100 {
		return fmt.Errorf("KATAR PRO canonical lighting is unavailable")
	}
	zone, ok := d.DeviceProfile.ZoneColors[0]
	if !ok || zone.Color == nil || len(d.DeviceProfile.ZoneColors) != 1 || !slices.Equal(zone.ColorIndex, []int{0, 1, 2}) {
		return fmt.Errorf("KATAR PRO Scroll zone is unavailable")
	}
	return nil
}

func (d *Device) resolveLightingSettings(effect string) (lightingsettings.EffectSettings, error) {
	if err := d.lightingReady(); err != nil {
		return lightingsettings.EffectSettings{}, err
	}
	if effect == "mouse" || !d.SupportsLightingEffect(effect) {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported KATAR PRO effect %q", effect)
	}
	profile := d.GetRgbProfile(effect)
	if profile == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("KATAR PRO effect settings are unavailable")
	}
	return lightingsettings.EffectSettingsFromRGBProfile(effect, *profile)
}

func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if d == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("KATAR PRO lighting is unavailable")
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.resolveLightingSettings(effect)
}

func katarColorHex(c lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(c.Red), uint8(c.Green), uint8(c.Blue))
}

func (d *Device) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	if d == nil {
		return lightingpresentation.Snapshot{}, false
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.lightingReady() != nil {
		return lightingpresentation.Snapshot{}, false
	}
	effect := d.DeviceProfile.RGBProfile
	descriptor, supported := rgb.SoftwareEffectDescriptorByID(effect)
	snapshot := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: effect, EffectSupported: supported && d.SupportsLightingEffect(effect), EffectSelectionAvailable: true, HasBrightness: true, Brightness: *d.DeviceProfile.BrightnessSlider}
	for _, candidate := range rgbModes {
		if candidate == "mouse" {
			snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{ID: "mouse", Label: "Mouse"})
			continue
		}
		value, ok := rgb.SoftwareEffectDescriptorByID(candidate)
		if !ok {
			return lightingpresentation.Snapshot{}, false
		}
		snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{ID: candidate, Label: value.Label})
	}
	if effect == "mouse" {
		snapshot.EffectSupported = true
		zone := d.DeviceProfile.ZoneColors[0]
		snapshot.AuthoredZoneEditor = &lightingpresentation.AuthoredZoneEditor{EffectID: "mouse", Heading: "Zones", Description: "Choose a color for the Scroll zone.", Zones: []lightingpresentation.AuthoredZone{{ID: "0", Label: "Scroll", ColorHex: fmt.Sprintf("#%02x%02x%02x", uint8(zone.Color.Red), uint8(zone.Color.Green), uint8(zone.Color.Blue))}}}
		return snapshot, true
	}
	// An old unsupported selection may be shown, but never made selectable.
	if !snapshot.EffectSupported {
		return snapshot, true
	}
	settings, err := d.resolveLightingSettings(effect)
	if err != nil {
		return lightingpresentation.Snapshot{}, false
	}
	defaults, err := d.lightingDefaults.Get(effect)
	if err != nil {
		return lightingpresentation.Snapshot{}, false
	}
	snapshot.Customized = !reflect.DeepEqual(settings, defaults)
	snapshot.PaletteKind = string(descriptor.PaletteKind)
	if settings.Speed != nil {
		snapshot.HasSpeed, snapshot.Speed = true, *settings.Speed
	}
	if settings.SingleColor != nil {
		snapshot.SingleColorHex = katarColorHex(settings.SingleColor.Color)
	}
	if settings.TwoColor != nil {
		snapshot.TwoColorStartHex, snapshot.TwoColorEndHex = katarColorHex(settings.TwoColor.Start), katarColorHex(settings.TwoColor.End)
	}
	if settings.Temperature != nil {
		snapshot.HasTemperature = true
		snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: katarColorHex(settings.Temperature.Low.Color), Celsius: settings.Temperature.Low.Celsius}
		snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: katarColorHex(settings.Temperature.Middle.Color), Celsius: settings.Temperature.Middle.Celsius}
		snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: katarColorHex(settings.Temperature.High.Color), Celsius: settings.Temperature.High.Celsius}
	}
	if settings.Gradient != nil {
		snapshot.HasGradient = true
		for _, stop := range settings.Gradient.Stops {
			snapshot.GradientStops = append(snapshot.GradientStops, lightingpresentation.GradientStop{Position: stop.Position, ColorHex: katarColorHex(stop.Color), Intensity: stop.Intensity})
		}
	}
	return snapshot, true
}

// Persist a proposed profile before publishing it to the renderer. Keep the
// existing filenames/schema and active-profile identity.
func (d *Device) persistLightingProfile(profile DeviceProfile) error {
	path := profile.Path
	if path == "" {
		path = d.Serial + ".json"
	}
	profile.Path = filepath.Join(pwd, "database", "profiles", filepath.Base(path))
	profile.RgbOff = false
	if err := common.SaveJsonData(profile.Path, &profile); err != nil {
		return err
	}
	*d.DeviceProfile = profile
	return nil
}

func (d *Device) restartLighting() {
	if d.lightingRestart != nil {
		d.lightingRestart()
		return
	}
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.setDeviceColor()
}

func (d *Device) SetLightingEffect(effect string) error {
	if d == nil {
		return fmt.Errorf("KATAR PRO lighting is unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	if !d.SupportsLightingEffect(effect) {
		d.lightingMu.Unlock()
		return fmt.Errorf("unsupported KATAR PRO effect %q", effect)
	}
	if effect != "mouse" {
		if _, err := d.resolveLightingSettings(effect); err != nil {
			d.lightingMu.Unlock()
			return err
		}
	}
	profile := *d.DeviceProfile
	profile.RGBProfile = effect
	err := d.persistLightingProfile(profile)
	d.lightingMu.Unlock()
	if err == nil {
		d.restartLighting()
	}
	return err
}

func (d *Device) SetLightingBrightness(value uint8) error {
	if d == nil {
		return fmt.Errorf("KATAR PRO lighting is unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	if value > 100 {
		d.lightingMu.Unlock()
		return fmt.Errorf("KATAR PRO brightness mutation is unavailable")
	}
	profile := *d.DeviceProfile
	profile.BrightnessSlider = &value
	err := d.persistLightingProfile(profile)
	static := profile.RGBProfile == "static" || profile.RGBProfile == "mouse"
	d.lightingMu.Unlock()
	// Dynamic rendering already samples brightness on every frame.
	if err == nil && static {
		d.restartLighting()
	}
	return err
}

func (d *Device) SetLightingEffectSettings(effect string, value lightingsettings.EffectSettings) error {
	if d == nil {
		return fmt.Errorf("KATAR PRO lighting is unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if _, err := d.resolveLightingSettings(effect); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	if value.EffectID != effect {
		d.lightingMu.Unlock()
		return fmt.Errorf("KATAR PRO effect settings mismatch")
	}
	if err := lightingsettings.Validate(value); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	profile := *d.GetRgbProfile(effect)
	rendered := lightingsettings.RendererProfileFromEffectSettings(value)
	// Only editable fields change. Preserve MinTemp/MaxTemp, Smoothness,
	// brightness modes, version and all other KATAR PRO renderer metadata.
	if value.Speed != nil {
		profile.Speed = rendered.Speed
	}
	if value.SingleColor != nil {
		profile.StartColor.Red, profile.StartColor.Green, profile.StartColor.Blue = rendered.StartColor.Red, rendered.StartColor.Green, rendered.StartColor.Blue
	}
	if value.TwoColor != nil {
		profile.StartColor.Red, profile.StartColor.Green, profile.StartColor.Blue = rendered.StartColor.Red, rendered.StartColor.Green, rendered.StartColor.Blue
		profile.EndColor.Red, profile.EndColor.Green, profile.EndColor.Blue = rendered.EndColor.Red, rendered.EndColor.Green, rendered.EndColor.Blue
	}
	if value.Temperature != nil {
		startBrightness, middleBrightness, endBrightness := profile.StartColor.Brightness, profile.MiddleColor.Brightness, profile.EndColor.Brightness
		profile.StartColor, profile.MiddleColor, profile.EndColor = rendered.StartColor, rendered.MiddleColor, rendered.EndColor
		profile.StartColor.Brightness, profile.MiddleColor.Brightness, profile.EndColor.Brightness = startBrightness, middleBrightness, endBrightness
	}
	if value.Gradient != nil {
		profile.Gradients = rendered.Gradients
	}
	err := d.persistLightingRGBProfile(effect, profile)
	selected := d.DeviceProfile.RGBProfile == effect
	d.lightingMu.Unlock()
	if err == nil && selected {
		d.restartLighting()
	}
	return err
}

// Called under lightingMu; uses the existing RGB backing file and publishes
// a new map only after persistence succeeds.
func (d *Device) persistLightingRGBProfile(effect string, profile rgb.Profile) error {
	proposed := *d.Rgb
	proposed.Profiles = make(map[string]rgb.Profile, len(d.Rgb.Profiles))
	for key, existing := range d.Rgb.Profiles {
		proposed.Profiles[key] = existing
	}
	proposed.Profiles[effect] = profile
	err := common.SaveJsonData(filepath.Join(pwd, "database", "rgb", d.Serial+".json"), &proposed)
	if err == nil {
		d.rgbMutex.Lock()
		d.Rgb.Profiles = proposed.Profiles
		d.rgbMutex.Unlock()
	}
	return err
}

func (d *Device) ResetLightingEffectSettings(effect string) error {
	if d == nil {
		return fmt.Errorf("KATAR PRO lighting is unavailable")
	}
	d.lightingMu.Lock()
	if _, err := d.resolveLightingSettings(effect); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	value, err := d.lightingDefaults.Get(effect)
	d.lightingMu.Unlock()
	if err != nil {
		return err
	}
	return d.SetLightingEffectSettings(effect, value)
}

// The existing renderer reads desired selection and a transient effective
// brightness. These values never write darkness into backing profile state.
func (d *Device) lightingOutputState() (string, uint8, bool) {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.DeviceProfile == nil || d.DeviceProfile.BrightnessSlider == nil {
		return "off", 0, true
	}
	brightness := *d.DeviceProfile.BrightnessSlider
	off := d.DeviceProfile.RgbOff
	if d.lightingDefaults != nil {
		off = d.userRGBOff
		if d.schedulerDark || off {
			brightness = 0
		}
	}
	return d.DeviceProfile.RGBProfile, brightness, off
}

// Authored Mouse colors remain device-owned, outside generic effect settings.
func (d *Device) SetLightingZoneColor(effect, scope, zoneID, groupID string, color rgb.Color) error {
	if groupID != "" || (scope != "zone" && scope != "all") || (scope == "zone" && zoneID != "0") || (scope == "all" && zoneID != "") {
		return fmt.Errorf("invalid KATAR PRO zone selection")
	}
	return d.SetLightingZoneColors(effect, []string{"0"}, color)
}
func (d *Device) SetLightingZoneColors(effect string, zoneIDs []string, color rgb.Color) error {
	if d == nil {
		return fmt.Errorf("KATAR PRO lighting is unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	if effect != "mouse" || len(zoneIDs) != 1 || zoneIDs[0] != "0" {
		d.lightingMu.Unlock()
		return fmt.Errorf("invalid KATAR PRO zone selection")
	}
	value := lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: color.Red, Green: color.Green, Blue: color.Blue}}}
	if err := lightingsettings.Validate(value); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	profile := *d.DeviceProfile
	zone := profile.ZoneColors[0]
	authored := *zone.Color
	authored.Red, authored.Green, authored.Blue = color.Red, color.Green, color.Blue
	authored.Hex = katarColorHex(value.SingleColor.Color)
	zone.Color = &authored
	profile.ZoneColors = map[int]ZoneColors{0: zone}
	err := d.persistLightingProfile(profile)
	d.lightingMu.Unlock()
	if err == nil {
		d.restartLighting()
	}
	return err
}

// Copy before applying either runtime Sniper substitution or brightness.
func (d *Device) mouseOutputColor(authored *rgb.Color, brightness uint8) rgb.Color {
	color := *authored
	if d.SniperMode {
		if sniper := d.getSniperColor(); sniper != nil {
			color = *sniper
		}
	}
	color.Brightness = rgb.GetBrightnessValueFloat(brightness)
	return *rgb.ModifyBrightness(color)
}

func copyLightingRGBProfile(value rgb.Profile) rgb.Profile {
	if value.Gradients != nil {
		colors := make(map[int]rgb.Color, len(value.Gradients))
		for key, color := range value.Gradients {
			colors[key] = color
		}
		value.Gradients = colors
	}
	return value
}

// Rendering owns its copies; persistence can publish a new authored zone map
// without sharing pointer-backed colors with the output loop.
func (d *Device) lightingOutputZones(brightness uint8, mouse bool) map[int]ZoneColors {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.DeviceProfile == nil {
		return nil
	}
	zones := make(map[int]ZoneColors, len(d.DeviceProfile.ZoneColors))
	for id, zone := range d.DeviceProfile.ZoneColors {
		zone.ColorIndex = append([]int(nil), zone.ColorIndex...)
		if zone.Color != nil {
			color := *zone.Color
			if mouse {
				color = d.mouseOutputColor(zone.Color, brightness)
			}
			zone.Color = &color
		}
		zones[id] = zone
	}
	return zones
}
