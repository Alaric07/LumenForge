package scufenvisionproV2W

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"

	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

// SCUF deliberately adapts its existing active device profile and RGB file.
// There is no second desired-state or effect-settings database. The shared
// defaults repository supplies immutable reset values only.
func (d *Device) attachLightingRuntime(paths config.Paths) error {
	defaults, err := lightingsettings.LoadDefaultRepository(filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		return err
	}
	if d.DeviceProfile == nil || d.DeviceProfile.BrightnessSlider > 100 || d.Rgb == nil || !d.SupportsLightingEffect(d.DeviceProfile.RGBProfile) {
		return fmt.Errorf("SCUF lighting backing state is unavailable")
	}
	if err := d.validateLightingZones(); err != nil {
		return err
	}
	for _, effect := range rgbModes {
		if effect == "controller" {
			continue
		}
		profile := d.GetRgbProfile(effect)
		if profile == nil {
			return fmt.Errorf("SCUF lighting profile %q is unavailable", effect)
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
	if d == nil || d.Serial == "" || d.LEDChannels != 9 || d.ChangeableLedChannels != 9 || d.dev == nil || !d.Connected || d.Exit || d.lightingDefaults == nil || d.DeviceProfile == nil || d.Rgb == nil || d.DeviceProfile.BrightnessSlider > 100 {
		return fmt.Errorf("SCUF canonical lighting is unavailable")
	}
	if err := d.validateLightingZones(); err != nil {
		return err
	}
	if !d.SupportsLightingEffect(d.DeviceProfile.RGBProfile) || (d.DeviceProfile.RGBProfile != "controller" && d.GetRgbProfile(d.DeviceProfile.RGBProfile) == nil) {
		return fmt.Errorf("selected Lighting backing unavailable")
	}
	for _, effect := range rgbModes {
		if effect == "controller" {
			continue
		}
		profile := d.GetRgbProfile(effect)
		if profile == nil {
			return fmt.Errorf("SCUF effect backing unavailable")
		}
		if _, err := lightingsettings.EffectSettingsFromRGBProfile(effect, *profile); err != nil {
			return err
		}
	}
	return nil
}

func (d *Device) resolveLightingSettings(effect string) (lightingsettings.EffectSettings, error) {
	if err := d.lightingReady(); err != nil {
		return lightingsettings.EffectSettings{}, err
	}
	if effect == "controller" || !d.SupportsLightingEffect(effect) {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported SCUF effect %q", effect)
	}
	profile := d.GetRgbProfile(effect)
	if profile == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("SCUF effect settings are unavailable")
	}
	return lightingsettings.EffectSettingsFromRGBProfile(effect, *profile)
}

func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if d == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("SCUF lighting is unavailable")
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.resolveLightingSettings(effect)
}

func controllerColorHex(c lightingsettings.Color) string {
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
	snapshot := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: effect, EffectSupported: supported && d.SupportsLightingEffect(effect), EffectSelectionAvailable: true, HasBrightness: true, Brightness: d.DeviceProfile.BrightnessSlider}
	for _, candidate := range rgbModes {
		if candidate == "controller" {
			snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{ID: "controller", Label: "Controller"})
			continue
		}
		value, ok := rgb.SoftwareEffectDescriptorByID(candidate)
		if !ok {
			return lightingpresentation.Snapshot{}, false
		}
		snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{ID: candidate, Label: value.Label})
	}
	if effect == "controller" {
		snapshot.EffectSupported = true
		snapshot.AuthoredZoneEditor = &lightingpresentation.AuthoredZoneEditor{EffectID: "controller", Heading: "Zones", Description: "Choose colors for the selected zones."}
		for id := 0; id < 1; id++ {
			zone := d.DeviceProfile.ZoneColors[id]
			snapshot.AuthoredZoneEditor.Zones = append(snapshot.AuthoredZoneEditor.Zones, lightingpresentation.AuthoredZone{ID: strconv.Itoa(id), Label: zone.Name, ColorHex: fmt.Sprintf("#%02x%02x%02x", uint8(zone.Color.Red), uint8(zone.Color.Green), uint8(zone.Color.Blue))})
		}
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
		snapshot.SingleColorHex = controllerColorHex(settings.SingleColor.Color)
	}
	if settings.TwoColor != nil {
		snapshot.TwoColorStartHex, snapshot.TwoColorEndHex = controllerColorHex(settings.TwoColor.Start), controllerColorHex(settings.TwoColor.End)
	}
	if settings.Temperature != nil {
		snapshot.HasTemperature = true
		snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: controllerColorHex(settings.Temperature.Low.Color), Celsius: settings.Temperature.Low.Celsius}
		snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: controllerColorHex(settings.Temperature.Middle.Color), Celsius: settings.Temperature.Middle.Celsius}
		snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: controllerColorHex(settings.Temperature.High.Color), Celsius: settings.Temperature.High.Celsius}
	}
	if settings.Gradient != nil {
		snapshot.HasGradient = true
		for _, stop := range settings.Gradient.Stops {
			snapshot.GradientStops = append(snapshot.GradientStops, lightingpresentation.GradientStop{Position: stop.Position, ColorHex: controllerColorHex(stop.Color), Intensity: stop.Intensity})
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
	if err := saveLightingJSON(profile.Path, &profile); err != nil {
		return err
	}
	*d.DeviceProfile = profile
	return nil
}

func (d *Device) restartLighting() {
	d.lightingMu.Lock()
	unavailable := d.DeviceProfile == nil || d.Exit || !d.Connected
	d.lightingMu.Unlock()
	if unavailable {
		return
	}

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
		return fmt.Errorf("SCUF lighting is unavailable")
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
		return fmt.Errorf("unsupported SCUF effect %q", effect)
	}
	if effect != "controller" {
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
		return fmt.Errorf("SCUF lighting is unavailable")
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
		return fmt.Errorf("SCUF brightness mutation is unavailable")
	}
	profile := *d.DeviceProfile
	profile.BrightnessSlider = value
	err := d.persistLightingProfile(profile)
	static := profile.RGBProfile == "static" || profile.RGBProfile == "controller"
	d.lightingMu.Unlock()
	// Dynamic rendering already samples brightness on every frame.
	if err == nil && static {
		d.restartLighting()
	}
	return err
}

func (d *Device) SetLightingEffectSettings(effect string, value lightingsettings.EffectSettings) error {
	if d == nil {
		return fmt.Errorf("SCUF lighting is unavailable")
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
		return fmt.Errorf("SCUF effect settings mismatch")
	}
	if err := lightingsettings.Validate(value); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	profile := *d.GetRgbProfile(effect)
	rendered := lightingsettings.RendererProfileFromEffectSettings(value)
	// Only editable fields change. Preserve MinTemp/MaxTemp, Smoothness,
	// brightness modes, version and all other SCUF renderer metadata.
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
	err := saveLightingJSON(filepath.Join(pwd, "database", "rgb", d.Serial+".json"), &proposed)
	if err == nil {
		d.rgbMutex.Lock()
		d.Rgb.Profiles = proposed.Profiles
		d.rgbMutex.Unlock()
	}
	return err
}

func (d *Device) ResetLightingEffectSettings(effect string) error {
	if d == nil {
		return fmt.Errorf("SCUF lighting is unavailable")
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
	if d.DeviceProfile == nil {
		return "off", 0, true
	}
	brightness := d.DeviceProfile.BrightnessSlider
	off := d.DeviceProfile.RgbOff
	if d.lightingDefaults != nil {
		off = d.userRGBOff
		if off {
			brightness = 0
		}
	}
	return d.DeviceProfile.RGBProfile, brightness, off
}

// Authored Controller colors remain device-owned, outside generic effect settings.
func (d *Device) SetLightingZoneColor(effect, scope, zoneID, groupID string, color rgb.Color) error {
	if groupID != "" {
		return fmt.Errorf("invalid authored group")
	}
	switch scope {
	case "zone":
		return d.SetLightingZoneColors(effect, []string{zoneID}, color)
	case "all":
		if zoneID != "" {
			return fmt.Errorf("invalid authored zone selection")
		}
		return d.SetLightingZoneColors(effect, []string{"0"}, color)
	default:
		return fmt.Errorf("invalid authored scope")
	}
}
func (d *Device) SetLightingZoneColors(effect string, zoneIDs []string, color rgb.Color) error {
	if d == nil {
		return fmt.Errorf("SCUF lighting is unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	if effect != "controller" || len(zoneIDs) == 0 {
		d.lightingMu.Unlock()
		return fmt.Errorf("invalid authored zones")
	}
	value := lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: color.Red, Green: color.Green, Blue: color.Blue}}}
	if err := lightingsettings.Validate(value); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	profile := *d.DeviceProfile
	profile.ZoneColors = d.copyLightingZones()
	seen := make(map[int]bool)
	for _, id := range zoneIDs {
		key, err := strconv.Atoi(id)
		zone, exists := profile.ZoneColors[key]
		if err != nil || !exists || seen[key] {
			d.lightingMu.Unlock()
			return fmt.Errorf("invalid authored zone selection")
		}
		seen[key] = true
		zone.Color.Red, zone.Color.Green, zone.Color.Blue = color.Red, color.Green, color.Blue
		zone.Color.Hex = controllerColorHex(value.SingleColor.Color)
		profile.ZoneColors[key] = zone
	}
	err := d.persistLightingProfile(profile)
	selected := profile.RGBProfile == "controller"
	d.lightingMu.Unlock()
	if err == nil && selected {
		d.restartLighting()
	}
	return err
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

// Called under lightingMu. Authored pointers and byte mappings never escape to output.
func (d *Device) copyLightingZones() map[int]ZoneColors {
	if d.DeviceProfile == nil {
		return nil
	}
	zones := make(map[int]ZoneColors, len(d.DeviceProfile.ZoneColors))
	for id, zone := range d.DeviceProfile.ZoneColors {
		zone.ColorIndex = append([]int(nil), zone.ColorIndex...)
		if zone.Color != nil {
			color := *zone.Color
			zone.Color = &color
		}
		zones[id] = zone
	}
	return zones
}
func (d *Device) lightingOutputZones(brightness uint8, controller bool) map[int]ZoneColors {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	zones := d.copyLightingZones()
	if controller {
		for id, zone := range zones {
			if zone.Color != nil {
				zone.Color.Brightness = rgb.GetBrightnessValueFloat(brightness)
				zone.Color = rgb.ModifyBrightness(*zone.Color)
				zones[id] = zone
			}
		}
	}
	return zones
}

var lightingZoneNames = [...]string{"Controller"}
var lightingZoneIndices = [...][3]int{{0, 9, 18}}

func (d *Device) validateLightingZones() error {
	if d.DeviceProfile == nil || len(d.DeviceProfile.ZoneColors) != 1 {
		return fmt.Errorf("SCUF authored zones unavailable")
	}
	for id, name := range lightingZoneNames {
		zone, ok := d.DeviceProfile.ZoneColors[id]
		if !ok || zone.Color == nil || !lightingColorOK(*zone.Color) || zone.Name != name || !slices.Equal(zone.ColorIndex, lightingZoneIndices[id][:]) {
			return fmt.Errorf("SCUF authored zone %d unavailable", id)
		}
	}
	return nil
}

// A failed write/close/rename leaves both the previous backing file and published
// state intact. This is the package-local error-producing canonical write seam.
func saveLightingJSON(path string, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "    ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".lighting-*.tmp")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (d *Device) setLightingDarkness(dark bool) error {
	if d == nil {
		return fmt.Errorf("SCUF lighting unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	changed := d.userRGBOff != dark
	d.userRGBOff = dark
	d.lightingMu.Unlock()
	if changed {
		d.restartLighting()
	}
	return nil
}

// The legacy Controller form uses the same persist-before-publication seam.
func (d *Device) saveLightingControllerColors(updates map[int]rgb.Color) error {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	validate := func(c rgb.Color) error {
		return lightingsettings.Validate(lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: c.Red, Green: c.Green, Blue: c.Blue}}})
	}
	if len(updates) == 0 {
		d.lightingMu.Unlock()
		return fmt.Errorf("Controller colors unavailable")
	}
	profile := *d.DeviceProfile
	profile.ZoneColors = d.copyLightingZones()
	for id, color := range updates {
		zone, ok := profile.ZoneColors[id]
		if !ok {
			d.lightingMu.Unlock()
			return fmt.Errorf("unknown Controller zone")
		}
		if err := validate(color); err != nil {
			d.lightingMu.Unlock()
			return err
		}
		zone.Color.Red, zone.Color.Green, zone.Color.Blue = color.Red, color.Green, color.Blue
		zone.Color.Hex = controllerColorHex(lightingsettings.Color{Red: color.Red, Green: color.Green, Blue: color.Blue})
		profile.ZoneColors[id] = zone
	}
	err := d.persistLightingProfile(profile)
	selected := profile.RGBProfile == "controller"
	d.lightingMu.Unlock()
	if err == nil && selected {
		d.restartLighting()
	}
	return err
}

func (d *Device) switchLightingProfile(name string) error {
	if d == nil {
		return fmt.Errorf("SCUF lighting unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	target, ok := d.UserProfiles[name]
	if !ok || target == nil {
		d.lightingMu.Unlock()
		return fmt.Errorf("unknown device profile")
	}
	// Validate the proposed active backing state without publishing it.
	proposed := *target
	if proposed.Path == "" {
		proposed.Path = d.Serial + ".json"
	}
	proposed.Path = filepath.Join(pwd, "database", "profiles", filepath.Base(proposed.Path))
	copySource := &Device{DeviceProfile: &proposed}
	proposed.ZoneColors = copySource.copyLightingZones()
	probe := &Device{DeviceProfile: &proposed}
	if proposed.BrightnessSlider > 100 || probe.validateLightingZones() != nil || (!d.SupportsLightingEffect(proposed.RGBProfile) || (proposed.RGBProfile != "controller" && d.GetRgbProfile(proposed.RGBProfile) == nil)) {
		d.lightingMu.Unlock()
		return fmt.Errorf("invalid Lighting profile")
	}
	current := *d.DeviceProfile
	proposed.Active, proposed.RgbOff = true, false
	if proposed.Path != current.Path {
		retired := current
		retired.Active = false
		if err := saveLightingJSON(current.Path, retired); err != nil {
			d.lightingMu.Unlock()
			return err
		}
		if err := saveLightingJSON(proposed.Path, proposed); err != nil {
			rollback := saveLightingJSON(current.Path, current)
			d.lightingMu.Unlock()
			if rollback != nil {
				return fmt.Errorf("profile selection failed: %w; rollback: %v", err, rollback)
			}
			return err
		}
		d.DeviceProfile.Active = false
	} else if err := saveLightingJSON(proposed.Path, proposed); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	*target = proposed
	d.DeviceProfile = target
	d.userRGBOff = false
	d.lightingMu.Unlock()
	// Retire the previous renderer before rendering the selected profile.
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.restartLighting()
	return nil
}

func lightingColorOK(c rgb.Color) bool {
	return lightingsettings.Validate(lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: c.Red, Green: c.Green, Blue: c.Blue}}}) == nil
}
