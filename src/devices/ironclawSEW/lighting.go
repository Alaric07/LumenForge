package ironclawSEW

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"

	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
)

// IRONCLAW SE deliberately adapts its existing active device profile and RGB file.
// There is no second desired-state or effect-settings database. The shared
// defaults repository supplies immutable reset values only.
func (d *Device) attachLightingRuntime(paths config.Paths) error {
	defaults, err := lightingsettings.LoadDefaultRepository(filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		return err
	}
	if d.DeviceProfile == nil || d.DeviceProfile.BrightnessSlider == nil || *d.DeviceProfile.BrightnessSlider > 100 || d.Rgb == nil {
		return fmt.Errorf("IRONCLAW SE lighting backing state is unavailable")
	}
	if err := d.validateLightingZones(); err != nil {
		return err
	}
	if err := d.validateLightingIndicators(); err != nil {
		return err
	}
	for _, effect := range rgbModes {
		if effect == "mouse" {
			continue
		}
		profile := d.GetRgbProfile(effect)
		if profile == nil {
			return fmt.Errorf("IRONCLAW SE lighting profile %q is unavailable", effect)
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
	if d == nil || d.Serial == "" || d.LEDChannels != 6 || d.ChangeableLedChannels != 3 || d.dev == nil || d.dev.Dev == nil || !d.Connected || d.Exit || d.lightingDefaults == nil || d.DeviceProfile == nil || d.DeviceProfile.BrightnessSlider == nil || d.Rgb == nil || *d.DeviceProfile.BrightnessSlider > 100 {
		return fmt.Errorf("IRONCLAW SE canonical lighting is unavailable")
	}
	if err := d.validateLightingZones(); err != nil {
		return err
	}
	if err := d.validateLightingIndicators(); err != nil {
		return err
	}
	// A usable attachment also requires complete, valid RGB backing settings.
	// Check before every mutation, including brightness and authored Mouse edits.
	for _, effect := range rgbModes {
		if effect == "mouse" {
			continue
		}
		profile := d.GetRgbProfile(effect)
		if profile == nil {
			return fmt.Errorf("lighting profile %q unavailable", effect)
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
	if effect == "mouse" || !d.SupportsLightingEffect(effect) {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported IRONCLAW SE effect %q", effect)
	}
	profile := d.GetRgbProfile(effect)
	if profile == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("IRONCLAW SE effect settings are unavailable")
	}
	return lightingsettings.EffectSettingsFromRGBProfile(effect, *profile)
}

func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if d == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("IRONCLAW SE lighting is unavailable")
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.resolveLightingSettings(effect)
}

func ironclawColorHex(c lightingsettings.Color) string {
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
	snapshot := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: effect, EffectSupported: supported && d.SupportsLightingEffect(effect), ClusterControlled: d.DeviceProfile.RGBCluster, ExternalControlled: d.DeviceProfile.OpenRGBIntegration, EffectSelectionAvailable: true, HasBrightness: true, Brightness: *d.DeviceProfile.BrightnessSlider}
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
		snapshot.AuthoredZoneEditor = &lightingpresentation.AuthoredZoneEditor{EffectID: "mouse", Heading: "Zones", Description: "Choose colors for the selected zones."}
		for id := 0; id < 3; id++ {
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
		snapshot.SingleColorHex = ironclawColorHex(settings.SingleColor.Color)
	}
	if settings.TwoColor != nil {
		snapshot.TwoColorStartHex, snapshot.TwoColorEndHex = ironclawColorHex(settings.TwoColor.Start), ironclawColorHex(settings.TwoColor.End)
	}
	if settings.Temperature != nil {
		snapshot.HasTemperature = true
		snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: ironclawColorHex(settings.Temperature.Low.Color), Celsius: settings.Temperature.Low.Celsius}
		snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: ironclawColorHex(settings.Temperature.Middle.Color), Celsius: settings.Temperature.Middle.Celsius}
		snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: ironclawColorHex(settings.Temperature.High.Color), Celsius: settings.Temperature.High.Celsius}
	}
	if settings.Gradient != nil {
		snapshot.HasGradient = true
		for _, stop := range settings.Gradient.Stops {
			snapshot.GradientStops = append(snapshot.GradientStops, lightingpresentation.GradientStop{Position: stop.Position, ColorHex: ironclawColorHex(stop.Color), Intensity: stop.Intensity})
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
	external := d.DeviceProfile == nil || d.DeviceProfile.RGBCluster || d.DeviceProfile.OpenRGBIntegration || d.Exit || !d.Connected
	d.lightingMu.Unlock()
	if external {
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
		return fmt.Errorf("IRONCLAW SE lighting is unavailable")
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
		return fmt.Errorf("unsupported IRONCLAW SE effect %q", effect)
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
		return fmt.Errorf("IRONCLAW SE lighting is unavailable")
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
		return fmt.Errorf("IRONCLAW SE brightness mutation is unavailable")
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
		return fmt.Errorf("IRONCLAW SE lighting is unavailable")
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
		return fmt.Errorf("IRONCLAW SE effect settings mismatch")
	}
	if err := lightingsettings.Validate(value); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	profile := *d.GetRgbProfile(effect)
	rendered := lightingsettings.RendererProfileFromEffectSettings(value)
	// Only editable fields change. Preserve MinTemp/MaxTemp, Smoothness,
	// brightness modes, version and all other IRONCLAW SE renderer metadata.
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
		return fmt.Errorf("IRONCLAW SE lighting is unavailable")
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
		return d.SetLightingZoneColors(effect, []string{"0", "1", "2"}, color)
	default:
		return fmt.Errorf("invalid authored scope")
	}
}
func (d *Device) SetLightingZoneColors(effect string, zoneIDs []string, color rgb.Color) error {
	if d == nil {
		return fmt.Errorf("IRONCLAW SE lighting is unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	if effect != "mouse" || len(zoneIDs) == 0 {
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
		zone.Color.Hex = ironclawColorHex(value.SingleColor.Color)
		profile.ZoneColors[key] = zone
	}
	err := d.persistLightingProfile(profile)
	selected := profile.RGBProfile == "mouse"
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
func (d *Device) lightingOutputZones(brightness uint8, mouse bool) map[int]ZoneColors {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	zones := d.copyLightingZones()
	if mouse {
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

var lightingZoneNames = [...]string{"Logo", "Scroll", "Front"}
var lightingZoneIndices = [...][3]int{{0, 6, 12}, {1, 7, 13}, {2, 8, 14}}

func (d *Device) validateLightingZones() error {
	if d.DeviceProfile == nil || len(d.DeviceProfile.ZoneColors) != 3 {
		return fmt.Errorf("IRONCLAW SE authored zones unavailable")
	}
	for id, name := range lightingZoneNames {
		zone, ok := d.DeviceProfile.ZoneColors[id]
		if !ok || zone.Color == nil || !ironclawSEWColorOK(*zone.Color) || zone.Name != name || !slices.Equal(zone.ColorIndex, lightingZoneIndices[id][:]) {
			return fmt.Errorf("IRONCLAW SE authored zone %d unavailable", id)
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

func (d *Device) setLightingDarkness(scheduler, dark bool) error {
	if d == nil {
		return fmt.Errorf("IRONCLAW SE lighting unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	changed := false
	static := d.DeviceProfile.RGBProfile == "mouse" || d.DeviceProfile.RGBProfile == "static"
	if scheduler {
		changed = d.schedulerDark != dark
		d.schedulerDark = dark
	} else {
		changed = d.userRGBOff != dark
		d.userRGBOff = dark
	}
	d.lightingMu.Unlock()
	if changed && (static || !scheduler) {
		d.restartLighting()
	}
	return nil
}

// The retained combined Mouse form updates DPI color and authored zones in one
// proposed profile, preserving input configuration and failing before publication.
func (d *Device) saveLightingMouseColors(dpi rgb.Color, updates map[int]rgb.Color) error {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	profile := *d.DeviceProfile
	profile.ZoneColors = d.copyLightingZones()
	validate := func(c rgb.Color) error {
		return lightingsettings.Validate(lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: c.Red, Green: c.Green, Blue: c.Blue}}})
	}
	if err := validate(dpi); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	if profile.DPIColor == nil || len(updates) == 0 {
		d.lightingMu.Unlock()
		return fmt.Errorf("Mouse colors unavailable")
	}
	dpiCopy := *profile.DPIColor
	dpiCopy.Red, dpiCopy.Green, dpiCopy.Blue = dpi.Red, dpi.Green, dpi.Blue
	dpiCopy.Hex = fmt.Sprintf("#%02x%02x%02x", uint8(dpi.Red), uint8(dpi.Green), uint8(dpi.Blue))
	profile.DPIColor = &dpiCopy
	for id, color := range updates {
		zone, ok := profile.ZoneColors[id]
		if !ok {
			d.lightingMu.Unlock()
			return fmt.Errorf("unknown Mouse zone")
		}
		if err := validate(color); err != nil {
			d.lightingMu.Unlock()
			return err
		}
		zone.Color.Red, zone.Color.Green, zone.Color.Blue = color.Red, color.Green, color.Blue
		zone.Color.Hex = fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
		profile.ZoneColors[id] = zone
	}
	err := d.persistLightingProfile(profile)
	d.lightingMu.Unlock()
	if err == nil {
		d.restartLighting()
	}
	return err
}

func (d *Device) switchLightingProfile(name string) error {
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
	if proposed.BrightnessSlider == nil {
		brightness := uint8(100)
		proposed.BrightnessSlider = &brightness
	}
	if proposed.PollingRate == 0 {
		proposed.PollingRate = 4
	}
	if proposed.LiftHeight == 0 {
		proposed.LiftHeight = 2
	}
	if proposed.SleepMode == 0 {
		proposed.SleepMode = 15
	}
	// Legacy saveDeviceProfile upgrades a missing Sniper stage. Normalize on
	// a copied map so a rejected write cannot mutate the inactive profile.
	if proposed.Profiles == nil || proposed.DPIColor == nil {
		d.lightingMu.Unlock()
		return fmt.Errorf("invalid DPI backing profile")
	}
	proposed.Profiles = make(map[int]DPIProfile, len(target.Profiles))
	for id, stage := range target.Profiles {
		proposed.Profiles[id] = stage
	}
	probe := &Device{DeviceProfile: &proposed}
	probe.upgradeDpiProfiles()
	if *proposed.BrightnessSlider > 100 || probe.validateLightingZones() != nil || probe.validateLightingIndicators() != nil {
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
	d.schedulerDark, d.userRGBOff = false, false
	d.lightingMu.Unlock()
	// Retire the previous renderer even when the selected profile is externally
	// owned and canonical restart therefore skips local output.
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.restartLighting()
	return nil
}

// Ownership transitions retain the existing registration and frame paths. Only
// the flag persistence/publication participates in the canonical mutation seam.
func (d *Device) setLightingOwnership(clusterOwner, enabled bool) error {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if err := d.lightingReady(); err != nil {
		return err
	}
	profile := *d.DeviceProfile
	if clusterOwner {
		if profile.OpenRGBIntegration {
			return fmt.Errorf("OpenRGB owns Lighting")
		}
		profile.RGBCluster = enabled
	} else {
		if profile.RGBCluster {
			return fmt.Errorf("Cluster owns Lighting")
		}
		profile.OpenRGBIntegration = enabled
	}
	return d.persistLightingProfile(profile)
}

// Copy device-owned indicator and zone metadata before composing an externally
// owned frame. Supplied zone bytes pass through without local rescaling.
func (d *Device) externalLightingFrame(data []byte) ([]byte, bool) {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	buf := make([]byte, d.LEDChannels*3)
	if d.DeviceProfile == nil {
		return nil, false
	}

	zones := d.copyLightingZones()
	// DPI
	if d.DeviceProfile.DPIColor == nil {
		return nil, false
	}
	dpiCopy := *d.DeviceProfile.DPIColor
	dpiColor := &dpiCopy
	dpiLeds := d.DeviceProfile.Profiles[d.DeviceProfile.Profile]
	if d.SniperMode {
		dpiColor = &rgb.Color{Red: 255, Green: 255, Blue: 0, Brightness: 1, Hex: ""}
		dpiLeds = d.getSniperColorIndex()
	}

	if len(dpiLeds.ColorIndex) == 0 {
		return nil, false
	}

	for i := 0; i < len(dpiLeds.ColorIndex); i++ {
		dpiColorIndexRange := dpiLeds.ColorIndex[i]
		for key, dpiColorIndex := range dpiColorIndexRange {
			switch key {
			case 0: // Red
				buf[dpiColorIndex] = byte(dpiColor.Red)
			case 1: // Green
				buf[dpiColorIndex] = byte(dpiColor.Green)
			case 2: // Blue
				buf[dpiColorIndex] = byte(dpiColor.Blue)
			}
		}
	}

	zoneKeys := make([]int, 0, len(zones))
	for key := range zones {
		zoneKeys = append(zoneKeys, key)
	}
	sort.Ints(zoneKeys)

	m := 0
	for _, key := range zoneKeys {
		zoneColor := zones[key]
		for _, zoneColorIndex := range zoneColor.ColorIndex {
			if m >= len(data) {
				break
			}
			buf[zoneColorIndex] = data[m]
			m++
		}
	}

	return buf, true
}

// Apply the selected profile DPI after canonical publication; output was already restarted.
func (d *Device) applyLightingProfileDPI() {
	if d.DeviceProfile != nil {
		d.deviceLock.Lock()
		profile := d.DeviceProfile.Profiles[d.DeviceProfile.Profile]
		value := profile.Value

		// Send DPI packet
		if value < uint16(minDpiValue) {
			value = uint16(minDpiValue)
		}
		if value > uint16(maxDpiValue) {
			value = uint16(maxDpiValue)
		}

		for _, dpiCode := range modeDpi {
			buf := make([]byte, 4)
			buf[0] = dpiCode
			buf[1] = 0x00
			binary.LittleEndian.PutUint16(buf[2:4], value)
			_, err := d.transfer(cmdSetDpi, buf)
			if err != nil {
				logger.Log(logger.Fields{"error": err, "vendorId": d.VendorId}).Error("Unable to set dpi")
			}
		}
		d.deviceLock.Unlock()
	}
}

// Validate only backing data needed for safe indicator composition. Input values
// and DPI protocol remain owned by the existing receiver implementation.
func (d *Device) validateLightingIndicators() error {
	if d.DeviceProfile == nil || d.DeviceProfile.DPIColor == nil || !ironclawSEWColorOK(*d.DeviceProfile.DPIColor) {
		return fmt.Errorf("Lighting indicator color unavailable")
	}
	selected, ok := d.DeviceProfile.Profiles[d.DeviceProfile.Profile]
	if !ok || len(selected.ColorIndex) == 0 {
		return fmt.Errorf("Lighting indicator stage unavailable")
	}
	for _, stage := range d.DeviceProfile.Profiles {
		for _, indices := range stage.ColorIndex {
			if len(indices) != 3 {
				return fmt.Errorf("invalid Lighting indicator mapping")
			}
			for _, index := range indices {
				if index < 0 || index >= 18 {
					return fmt.Errorf("invalid Lighting indicator byte index")
				}
			}
		}
	}
	return nil
}
