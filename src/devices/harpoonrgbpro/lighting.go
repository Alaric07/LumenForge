package harpoonrgbpro

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

type harpoonEffectStore interface {
	Get(string, string) (lightingsettings.EffectSettings, bool, error)
	Set(string, string, lightingsettings.EffectSettings) error
	Delete(string, string) (bool, error)
	GetAuthoredZones(string, string) (map[string]lightingsettings.Color, bool, error)
	SetAuthoredZones(string, string, map[string]lightingsettings.Color) error
	ImportDevice(string, map[string]lightingsettings.EffectSettings, map[string]map[string]lightingsettings.Color, func() error) error
}
type harpoonSettingsResolver interface {
	Resolve(lightingsettings.Target, string) (lightingsettings.Resolution, error)
}
type harpoonLighting struct {
	state    lightingsettings.IndependentDeviceStateAccess
	effects  harpoonEffectStore
	resolver harpoonSettingsResolver
	defaults *lightingsettings.DefaultRepository
}

// Attachment imports only when the shared target-state record is absent. That
// record is the completion marker, so a reset never reimports obsolete RGB JSON.
func (d *Device) attachLightingRuntime(paths config.Paths) error {
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile, filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		return err
	}
	return d.attachLightingSource(runtime)
}
func (d *Device) attachLightingSource(runtime *lightingsettings.IndependentDeviceRuntime) error {
	if d == nil || runtime == nil || runtime.State == nil || runtime.Effects == nil || runtime.Resolver == nil || runtime.Defaults == nil {
		return fmt.Errorf("HARPOON RGB PRO canonical runtime is unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.lightingCanonical != nil {
		return nil
	}
	if err := d.lightingHardwareReady(); err != nil {
		return err
	}
	source := &harpoonLighting{runtime.State, runtime.Effects, runtime.Resolver, runtime.Defaults}
	state, found, err := source.state.Resolve(d.Serial)
	if err != nil {
		return err
	}
	// Validate the full proposal and every imported effect before touching stores.
	imports := make(map[string]lightingsettings.EffectSettings)
	palettes := make(map[string]map[string]lightingsettings.Color)
	if !found {
		profile := normalizeHarpoonProfile(*d.DeviceProfile)
		zone, ok := profile.ZoneColors[0]
		if !ok || len(profile.ZoneColors) != 1 || zone.Name != "Logo" || zone.Color == nil || !slices.Equal(zone.ColorIndex, []int{0, 1, 2}) {
			return fmt.Errorf("HARPOON RGB PRO Logo backing is unavailable")
		}
		palette := map[string]lightingsettings.Color{"0": canonicalColor(*zone.Color)}
		if err = validateLogo(palette); err != nil {
			return err
		}
		palettes["mouse"] = palette
		state = lightingsettings.IndependentDeviceLightingState{SelectedEffect: profile.RGBProfile, Brightness: *profile.BrightnessSlider}
		// Unknown old selections remain serialized, but cannot become an invalid
		// canonical target record. Known unadvertised effects remain displayable.
		if lightingsettings.ValidateIndependentDeviceLightingState(state) != nil {
			if state.Brightness > 100 {
				return fmt.Errorf("invalid HARPOON RGB PRO brightness")
			}
			state.SelectedEffect = lightingsettings.DefaultIndependentDeviceEffect
		}
	}
	for _, effect := range rgbModes {
		if effect == "mouse" {
			continue
		}
		defaults, defaultErr := source.defaults.Get(effect)
		if defaultErr != nil {
			return defaultErr
		}
		existing, exists, getErr := source.effects.Get(d.Serial, effect)
		if getErr != nil {
			return getErr
		}
		if exists {
			if err = lightingsettings.Validate(existing); err != nil {
				return err
			}
			continue
		}
		if !found {
			if profile := d.legacyRgbProfile(effect); profile != nil {
				settings, convertErr := harpoonEffectSettings(effect, *profile)
				if convertErr != nil {
					return convertErr
				}
				if !reflect.DeepEqual(settings, defaults) {
					imports[effect] = settings
				}
			}
		}
	}
	if palette, exists, paletteErr := source.effects.GetAuthoredZones(d.Serial, "mouse"); paletteErr != nil {
		return paletteErr
	} else if exists {
		if err = validateLogo(palette); err != nil {
			return err
		}
	}
	if err = lightingsettings.ValidateIndependentDeviceLightingState(state); err != nil {
		return err
	}
	if !found {
		if err = source.effects.ImportDevice(d.Serial, imports, palettes, func() error { return source.state.Set(d.Serial, state) }); err != nil {
			return err
		}
	}
	// No legacy state/defaults are published on any failure above. After this
	// point even legacy-shaped getters project canonical values, never dual-read.
	d.lightingCanonical = source
	return nil
}

func (d *Device) LightingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) SupportsLightingEffect(effect string) bool { return slices.Contains(rgbModes, effect) }
func (d *Device) lightingAttached() bool {
	if d == nil {
		return false
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.lightingCanonical != nil
}

// Called under lightingMu. Legacy Lighting fields are not readiness authorities
// after cutover; only the input profile and physical runtime remain device-owned.
func (d *Device) lightingHardwareReady() error {
	if d.Serial == "" || d.dev == nil || !d.Connected || d.Exit || d.DeviceProfile == nil || d.LEDChannels != 1 || d.ChangeableLedChannels != 1 || d.ZoneAmount != 1 {
		return fmt.Errorf("HARPOON RGB PRO lighting is unavailable")
	}
	return nil
}
func (d *Device) lightingReady() error {
	if err := d.lightingHardwareReady(); err != nil {
		return err
	}
	if d.lightingCanonical == nil {
		return fmt.Errorf("HARPOON RGB PRO canonical attachment is unavailable")
	}
	state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
	if err != nil {
		return err
	}
	if err = lightingsettings.ValidateIndependentDeviceLightingState(state); err != nil {
		return err
	}
	_, err = d.canonicalLogo()
	return err
}
func canonicalColor(color rgb.Color) lightingsettings.Color {
	return lightingsettings.Color{Red: color.Red, Green: color.Green, Blue: color.Blue}
}
func lightingColorHex(color lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
}
func validateLogo(palette map[string]lightingsettings.Color) error {
	color, ok := palette["0"]
	if !ok || len(palette) != 1 {
		return fmt.Errorf("invalid HARPOON RGB PRO Logo palette")
	}
	return lightingsettings.Validate(lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: color}})
}
func (d *Device) canonicalLogo() (rgb.Color, error) {
	colors, found, err := d.lightingCanonical.effects.GetAuthoredZones(d.Serial, "mouse")
	if err != nil {
		return rgb.Color{}, err
	}
	if !found {
		colors = map[string]lightingsettings.Color{"0": {Red: 255, Green: 255}}
	} // source-backed immutable Logo default
	if err = validateLogo(colors); err != nil {
		return rgb.Color{}, err
	}
	color := colors["0"]
	return rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue, Brightness: 1, Hex: lightingColorHex(color)}, nil
}
func (d *Device) resolveLightingSettings(effect string) (lightingsettings.Resolution, error) {
	if d.lightingCanonical == nil || effect == "mouse" || !d.SupportsLightingEffect(effect) {
		return lightingsettings.Resolution{}, fmt.Errorf("unsupported HARPOON RGB PRO settings %q", effect)
	}
	return d.lightingCanonical.resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), effect)
}
func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if d == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("device unavailable")
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if err := d.lightingReady(); err != nil {
		return lightingsettings.EffectSettings{}, err
	}
	value, err := d.resolveLightingSettings(effect)
	return value.Settings.Clone(), err
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
	state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
	if err != nil {
		return lightingpresentation.Snapshot{}, false
	}
	snapshot := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: state.SelectedEffect, EffectSupported: d.SupportsLightingEffect(state.SelectedEffect), EffectSelectionAvailable: true, HasBrightness: true, Brightness: state.Brightness}
	for _, effect := range rgbModes {
		label := "Mouse"
		if effect != "mouse" {
			descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
			if !ok {
				return lightingpresentation.Snapshot{}, false
			}
			label = descriptor.Label
		}
		snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{ID: effect, Label: label})
	}
	if state.SelectedEffect == "mouse" {
		color, colorErr := d.canonicalLogo()
		if colorErr != nil {
			return lightingpresentation.Snapshot{}, false
		}
		snapshot.AuthoredZoneEditor = &lightingpresentation.AuthoredZoneEditor{EffectID: "mouse", Heading: "Zones", Description: "Choose a color for the Logo zone.", Zones: []lightingpresentation.AuthoredZone{{ID: "0", Label: "Logo", ColorHex: lightingColorHex(canonicalColor(color))}}}
		return snapshot, true
	}
	if !snapshot.EffectSupported {
		return snapshot, true
	}
	resolution, err := d.resolveLightingSettings(state.SelectedEffect)
	if err != nil {
		return lightingpresentation.Snapshot{}, false
	}
	settings := resolution.Settings
	descriptor, _ := rgb.SoftwareEffectDescriptorByID(state.SelectedEffect)
	snapshot.Customized, snapshot.PaletteKind = resolution.Customized, string(descriptor.PaletteKind)
	if settings.Speed != nil {
		snapshot.HasSpeed, snapshot.Speed = true, *settings.Speed
	}
	if settings.SingleColor != nil {
		snapshot.SingleColorHex = lightingColorHex(settings.SingleColor.Color)
	}
	if settings.TwoColor != nil {
		snapshot.TwoColorStartHex, snapshot.TwoColorEndHex = lightingColorHex(settings.TwoColor.Start), lightingColorHex(settings.TwoColor.End)
	}
	if settings.Temperature != nil {
		snapshot.HasTemperature = true
		snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: lightingColorHex(settings.Temperature.Low.Color), Celsius: settings.Temperature.Low.Celsius}
		snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: lightingColorHex(settings.Temperature.Middle.Color), Celsius: settings.Temperature.Middle.Celsius}
		snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: lightingColorHex(settings.Temperature.High.Color), Celsius: settings.Temperature.High.Celsius}
	}
	if settings.Gradient != nil {
		snapshot.HasGradient = true
		for _, stop := range settings.Gradient.Stops {
			snapshot.GradientStops = append(snapshot.GradientStops, lightingpresentation.GradientStop{Position: stop.Position, Intensity: stop.Intensity, ColorHex: lightingColorHex(stop.Color)})
		}
	}
	return snapshot, true
}

// Store setters publish only after their atomic writer succeeds. lightingMu
// excludes the renderer while persistence is pending; restart happens unlocked.
func (d *Device) mutateLighting(change func(*harpoonLighting, lightingsettings.IndependentDeviceLightingState) (bool, error)) error {
	if d == nil {
		return fmt.Errorf("device unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
	restart := false
	if err == nil {
		restart, err = change(d.lightingCanonical, state)
	}
	d.lightingMu.Unlock()
	if err == nil && restart {
		d.restartLighting(false)
	}
	return err
}
func (d *Device) SetLightingEffect(effect string) error {
	return d.mutateLighting(func(source *harpoonLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if !d.SupportsLightingEffect(effect) {
			return false, fmt.Errorf("unsupported effect %q", effect)
		}
		if effect != "mouse" {
			if _, err := d.resolveLightingSettings(effect); err != nil {
				return false, err
			}
		}
		state.SelectedEffect = effect
		return true, source.state.Set(d.Serial, state)
	})
}
func (d *Device) SetLightingBrightness(value uint8) error {
	return d.mutateLighting(func(source *harpoonLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if value > 100 {
			return false, fmt.Errorf("brightness must be between 0 and 100")
		}
		state.Brightness = value
		return state.SelectedEffect == "mouse" || state.SelectedEffect == "static", source.state.Set(d.Serial, state)
	})
}
func (d *Device) SetLightingEffectSettings(effect string, value lightingsettings.EffectSettings) error {
	value = value.Clone()
	return d.mutateLighting(func(source *harpoonLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if effect == "mouse" || !d.SupportsLightingEffect(effect) || value.EffectID != effect {
			return false, fmt.Errorf("invalid effect settings identity")
		}
		if err := lightingsettings.Validate(value); err != nil {
			return false, err
		}
		return state.SelectedEffect == effect, source.effects.Set(d.Serial, effect, value)
	})
}
func (d *Device) ResetLightingEffectSettings(effect string) error {
	return d.mutateLighting(func(source *harpoonLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if _, err := source.defaults.Get(effect); err != nil || !d.SupportsLightingEffect(effect) {
			return false, fmt.Errorf("unsupported reset effect %q", effect)
		}
		_, err := source.effects.Delete(d.Serial, effect)
		return state.SelectedEffect == effect, err
	})
}
func (d *Device) SetLightingZoneColor(effect, scope, zoneID, groupID string, color rgb.Color) error {
	if groupID != "" || (scope != "zone" && scope != "all") || (scope == "zone" && zoneID != "0") || (scope == "all" && zoneID != "") {
		return fmt.Errorf("invalid Logo selection")
	}
	return d.SetLightingZoneColors(effect, []string{"0"}, color)
}
func (d *Device) SetLightingZoneColors(effect string, zones []string, color rgb.Color) error {
	return d.mutateLighting(func(source *harpoonLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if effect != "mouse" || len(zones) != 1 || zones[0] != "0" {
			return false, fmt.Errorf("invalid Logo selection")
		}
		palette := map[string]lightingsettings.Color{"0": canonicalColor(color)}
		if err := validateLogo(palette); err != nil {
			return false, err
		}
		return state.SelectedEffect == "mouse", source.effects.SetAuthoredZones(d.Serial, "mouse", palette)
	})
}
func (d *Device) changeLightingGradient(effect string, remove bool) (uint8, uint) {
	var index uint
	minimum := false
	err := d.mutateLighting(func(source *harpoonLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if effect != "gradient" {
			return false, fmt.Errorf("not a gradient")
		}
		resolved, err := d.resolveLightingSettings(effect)
		if err != nil {
			return false, err
		}
		value := resolved.Settings.Clone()
		stops := value.Gradient.Stops
		if remove {
			if len(stops) < 3 {
				minimum = true
				return false, nil
			}
			index = uint(len(stops) - 1)
			value.Gradient.Stops = stops[:len(stops)-1]
		} else {
			index = uint(len(stops))
			positions := []float64{0, 1}
			for _, stop := range stops {
				positions = append(positions, stop.Position)
			}
			sort.Float64s(positions)
			position, gap := 0.0, -1.0
			for i := 1; i < len(positions); i++ {
				if diff := positions[i] - positions[i-1]; diff > gap {
					gap = diff
					position = (positions[i] + positions[i-1]) / 2
				}
			}
			value.Gradient.Stops = append(stops, lightingsettings.GradientStop{Position: position, Color: lightingsettings.Color{Green: 255, Blue: 255}, Intensity: 1})
			sort.SliceStable(value.Gradient.Stops, func(i, j int) bool { return value.Gradient.Stops[i].Position < value.Gradient.Stops[j].Position })
		}
		return state.SelectedEffect == effect, source.effects.Set(d.Serial, effect, value)
	})
	if err != nil {
		return 0, 0
	}
	if minimum {
		return 2, 0
	}
	return 1, index
}
func (d *Device) setLightingOverride(scheduler bool, value bool) {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if scheduler {
		d.schedulerDark = value
	} else {
		d.userRGBOff = value
	}
	ready := d.lightingReady() == nil
	d.lightingMu.Unlock()
	if ready {
		d.restartLighting(!scheduler)
	} // RGB-off retains the source's DPI flash, scheduler does not.
}
func (d *Device) restartLighting(dpi bool) {
	if d.lightingRestart != nil {
		d.lightingRestart()
		return
	}
	d.stopLighting()
	d.setDeviceColor(dpi)
}
func (d *Device) stopLighting() {
	d.rendererMu.Lock()
	defer d.rendererMu.Unlock()
	if d.rendererStop != nil {
		close(d.rendererStop)
		d.rendererStop = nil
	}
	d.activeRgb = nil
}
func (d *Device) startLightingRenderer() (*rgb.ActiveRGB, chan struct{}, chan struct{}) {
	d.rendererMu.Lock()
	defer d.rendererMu.Unlock()
	if d.rendererStop != nil {
		close(d.rendererStop)
	}
	owner, stop, done := rgb.Exit(), make(chan struct{}), make(chan struct{})
	d.activeRgb, d.rendererStop, d.rendererDone = owner, stop, done
	return owner, stop, done
}
func (d *Device) retireLightingRenderer(owner *rgb.ActiveRGB, stop chan struct{}) {
	d.rendererMu.Lock()
	defer d.rendererMu.Unlock()
	if d.activeRgb == owner && d.rendererStop == stop {
		d.activeRgb = nil
		d.rendererStop = nil
	}
}
func (d *Device) rendererCurrent(stop chan struct{}) bool {
	d.rendererMu.Lock()
	defer d.rendererMu.Unlock()
	return d.rendererStop == stop
}
func lightingDelay(stop <-chan struct{}, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-stop:
		return false
	case <-timer.C:
		return true
	}
}

func copyLightingRGBProfile(value rgb.Profile) rgb.Profile {
	if value.Gradients != nil {
		gradients := make(map[int]rgb.Color, len(value.Gradients))
		for id, color := range value.Gradients {
			gradients[id] = color
		}
		value.Gradients = gradients
	}
	return value
}
func harpoonEffectSettings(effect string, profile rgb.Profile) (lightingsettings.EffectSettings, error) {
	if effect == "gradient" {
		for index := 0; index < len(profile.Gradients); index++ {
			if _, ok := profile.Gradients[index]; !ok {
				return lightingsettings.EffectSettings{}, fmt.Errorf("gradient requires contiguous stops")
			}
		}
	}
	return lightingsettings.EffectSettingsFromRGBProfile(effect, profile)
}
func cloneHarpoonProfile(profile DeviceProfile) DeviceProfile {
	if profile.BrightnessSlider != nil {
		value := *profile.BrightnessSlider
		profile.BrightnessSlider = &value
	}
	if profile.DPIColor != nil {
		color := *profile.DPIColor
		profile.DPIColor = &color
	}
	if profile.ZoneColors != nil {
		zones := make(map[int]ZoneColors, len(profile.ZoneColors))
		for id, zone := range profile.ZoneColors {
			zone.ColorIndex = append([]int(nil), zone.ColorIndex...)
			if zone.Color != nil {
				color := *zone.Color
				zone.Color = &color
			}
			zones[id] = zone
		}
		profile.ZoneColors = zones
	}
	if profile.Profiles != nil {
		stages := make(map[int]DPIProfile, len(profile.Profiles))
		for id, stage := range profile.Profiles {
			if stage.Color != nil {
				color := *stage.Color
				stage.Color = &color
			}
			if stage.ColorIndex != nil {
				indices := make(map[int][]int, len(stage.ColorIndex))
				for id, values := range stage.ColorIndex {
					indices[id] = append([]int(nil), values...)
				}
				stage.ColorIndex = indices
			}
			stages[id] = stage
		}
		profile.Profiles = stages
	}
	return profile
}
func normalizeHarpoonProfile(profile DeviceProfile) DeviceProfile {
	profile = cloneHarpoonProfile(profile)
	if profile.BrightnessSlider == nil {
		value := uint8(100)
		profile.BrightnessSlider = &value
	}
	if profile.SleepMode == 0 {
		profile.SleepMode = 15
	}
	found := false
	for _, stage := range profile.Profiles {
		if strings.EqualFold(stage.Name, "Sniper") {
			found = true
			break
		}
	}
	if !found {
		if profile.Profiles == nil {
			profile.Profiles = make(map[int]DPIProfile)
		}
		profile.Profiles[len(profile.Profiles)] = DPIProfile{Name: "Sniper", Value: 200, PackerIndex: 6, ColorIndex: map[int][]int{0: {0, 1, 2}}, Color: &rgb.Color{Red: 255, Green: 255, Brightness: 1, Hex: "#ffff00"}, Sniper: true}
	}
	return profile
}
func (d *Device) writeLightingProfile(profile DeviceProfile) error {
	path := profile.Path
	if path == "" {
		path = d.Serial + ".json"
	}
	profile.Path = filepath.Join(pwd, "database", "profiles", filepath.Base(path))
	if d.lightingWriteJSON != nil {
		return d.lightingWriteJSON(profile.Path, &profile)
	}
	return common.SaveJsonData(profile.Path, &profile)
}

// Input profiles remain input-owned. An attached profile switch cannot import
// a second desired Lighting selection or change the canonical palette.
func (d *Device) changeLightingDeviceProfile(name string) uint8 {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if d.lightingReady() != nil {
		d.lightingMu.Unlock()
		return 0
	}
	target, ok := d.UserProfiles[name]
	if !ok || target == nil {
		d.lightingMu.Unlock()
		return 0
	}
	candidate := normalizeHarpoonProfile(*target)
	stage, ok := candidate.Profiles[candidate.Profile]
	if !ok || stage.Sniper || stage.Color == nil || stage.Value < uint16(minDpiValue) || stage.Value > uint16(maxDpiValue) {
		d.lightingMu.Unlock()
		return 0
	}
	// Validate canonical state/settings before changing the input profile. There
	// is no canonical write: device-wide Lighting already persists independently.
	state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
	if err != nil {
		d.lightingMu.Unlock()
		return 0
	}
	if d.SupportsLightingEffect(state.SelectedEffect) && state.SelectedEffect != "mouse" {
		if _, err = d.resolveLightingSettings(state.SelectedEffect); err != nil {
			d.lightingMu.Unlock()
			return 0
		}
	}
	current := d.DeviceProfile
	old := cloneHarpoonProfile(*current)
	inactive := cloneHarpoonProfile(old)
	inactive.Active = false
	candidate.Active = true
	if target != current {
		if filepath.Base(candidate.Path) == filepath.Base(inactive.Path) {
			d.lightingMu.Unlock()
			return 0
		}
		if err = d.writeLightingProfile(inactive); err != nil {
			d.lightingMu.Unlock()
			return 0
		}
	}
	if err = d.writeLightingProfile(candidate); err != nil {
		if target != current {
			_ = d.writeLightingProfile(old)
		}
		d.lightingMu.Unlock()
		return 0
	}
	if target != current {
		*current = inactive
	}
	*target = candidate
	d.DeviceProfile = target
	// Source profile switching replaces RgbOff with the target profile value;
	// canonical transient ownership must not revive a stale serialized flag.
	d.userRGBOff = false
	d.lightingMu.Unlock()
	if d.lightingProfileChanged != nil {
		d.restartLighting(false)
		d.lightingProfileChanged()
	} else {
		d.toggleDPI(false)
		d.loadKeyAssignments()
		d.setupKeyAssignment()
	}
	return 1
}

func lightingStatus(err error) uint8 {
	if err != nil {
		return 0
	}
	return 1
}

// Complete frame input is copied under lightingMu. Attached rendering never
// consults DeviceProfile RGBProfile/BrightnessSlider/ZoneColors or legacy RGB.
type harpoonFrameState struct {
	effect              string
	transientOff        bool
	brightness          uint8
	indicatorBrightness uint8
	zones               map[int]ZoneColors
	profile             *rgb.Profile
	indicator           *rgb.Color
	indicatorIndexes    map[int][]int
	cpu, gpu            float32
}

func (d *Device) lightingFrameState() (harpoonFrameState, error) {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.Exit || d.DeviceProfile == nil {
		return harpoonFrameState{}, fmt.Errorf("lighting output unavailable")
	}
	input := cloneHarpoonProfile(*d.DeviceProfile)
	value := harpoonFrameState{cpu: d.CpuTemp, gpu: d.GpuTemp}
	if d.lightingCanonical != nil {
		state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
		if err != nil {
			return value, err
		}
		value.effect, value.brightness = state.SelectedEffect, state.Brightness
		logo, err := d.canonicalLogo()
		if err != nil {
			return value, err
		}
		value.zones = map[int]ZoneColors{0: {Name: "Logo", ColorIndex: []int{0, 1, 2}, Color: &logo}}
		if value.effect != "mouse" && d.SupportsLightingEffect(value.effect) {
			resolved, err := d.resolveLightingSettings(value.effect)
			if err != nil {
				return value, err
			}
			profile := lightingsettings.RendererProfileFromEffectSettings(resolved.Settings)
			value.profile = &profile
		}
		value.indicatorBrightness = value.brightness
		if d.schedulerDark {
			value.indicatorBrightness = 0
		}
		if d.schedulerDark || d.userRGBOff {
			value.brightness = 0
		}
		if d.userRGBOff {
			value.transientOff = true
			value.effect = "off"
			value.profile = &rgb.Profile{}
		}
	} else {
		if input.BrightnessSlider == nil {
			return value, fmt.Errorf("legacy slider unavailable")
		}
		value.effect, value.brightness, value.zones = input.RGBProfile, *input.BrightnessSlider, input.ZoneColors
		value.indicatorBrightness = value.brightness
		value.profile = d.legacyRgbProfile(value.effect)
		if input.RgbOff {
			value.transientOff = true
			value.effect = "off"
			value.profile = &rgb.Profile{}
		}
	}
	stage := input.Profiles[input.Profile]
	value.indicator, value.indicatorIndexes = stage.Color, stage.ColorIndex
	if d.SniperMode {
		for _, sniper := range input.Profiles {
			if sniper.Sniper {
				value.indicator = sniper.Color
				break
			}
		}
		if value.effect == "mouse" && value.indicator != nil {
			zone := value.zones[0]
			color := *value.indicator
			zone.Color = &color
			value.zones[0] = zone
		}
	}
	return value, nil
}
func composeColor(frame []byte, indexes []int, color rgb.Color, brightness uint8) {
	color.Brightness = rgb.GetBrightnessValueFloat(brightness)
	scaled := rgb.ModifyBrightness(color)
	channels := []byte{byte(scaled.Red), byte(scaled.Green), byte(scaled.Blue)}
	for i, index := range indexes {
		if i < len(channels) && index >= 0 && index < len(frame) {
			frame[index] = channels[i]
		}
	}
}
func (d *Device) legacyRgbProfile(effect string) *rgb.Profile {
	d.rgbMutex.RLock()
	defer d.rgbMutex.RUnlock()
	if d.Rgb != nil {
		if profile, ok := d.Rgb.Profiles[effect]; ok {
			copy := copyLightingRGBProfile(profile)
			return &copy
		}
	}
	return nil
}
func (d *Device) stopped() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.Exit }

func (d *Device) renderLightingFrame(value harpoonFrameState, owner *rgb.ActiveRGB, startTime *time.Time) []byte {
	frame := make([]byte, 3)
	if value.effect == "mouse" {
		for _, zone := range value.zones {
			if zone.Color != nil {
				composeColor(frame, zone.ColorIndex, *zone.Color, value.brightness)
			}
		}
		return frame
	}
	profile := value.profile
	if profile == nil {
		return nil
	}
	if value.effect == "static" {
		for _, zone := range value.zones {
			composeColor(frame, zone.ColorIndex, profile.StartColor, value.brightness)
		}
		return frame
	}
	speed := common.FClamp(profile.Speed, 0.1, 10)
	custom := (rgb.Color{}) != profile.StartColor && (rgb.Color{}) != profile.EndColor
	r := rgb.New(1, speed, nil, nil, profile.Brightness, common.Clamp(profile.Smoothness, 1, 100), time.Duration(speed)*time.Second, custom)
	if custom {
		r.RGBStartColor = &profile.StartColor
		r.RGBEndColor = &profile.EndColor
		r.RGBMiddleColor = &profile.MiddleColor
	} else {
		r.RGBStartColor = owner.RGBStartColor
		r.RGBEndColor = owner.RGBEndColor
		r.RGBMiddleColor = owner.RGBMiddleColor
	}
	if r.RGBMiddleColor == nil {
		r.RGBMiddleColor = &rgb.Color{}
	}
	r.RGBBrightness = rgb.GetBrightnessValueFloat(value.brightness)
	r.RGBStartColor.Brightness, r.RGBEndColor.Brightness, r.RGBMiddleColor.Brightness = r.RGBBrightness, r.RGBBrightness, r.RGBBrightness
	buff := make([]byte, 0)
	switch value.effect {
	case "off":
		{
			for n := 0; n < 1; n++ {
				buff = append(buff, []byte{0, 0, 0}...)
			}
		}
	case "rainbow":
		{
			r.Rainbow(*startTime)
			buff = append(buff, r.Output...)
		}
	case "pastelrainbow":
		{
			r.PastelRainbow(*startTime)
			buff = append(buff, r.Output...)
		}
	case "watercolor":
		{
			r.Watercolor(*startTime)
			buff = append(buff, r.Output...)
		}
	case "gradient":
		{
			r.ColorshiftGradient(*startTime, profile.Gradients, profile.Speed)
			buff = append(buff, r.Output...)
		}
	case "cpu-temperature":
		{
			r.MinTemp = profile.MinTemp
			r.MaxTemp = profile.MaxTemp
			r.Temperature(float64(value.cpu))
			buff = append(buff, r.Output...)
		}
	case "gpu-temperature":
		{
			r.MinTemp = profile.MinTemp
			r.MaxTemp = profile.MaxTemp
			r.Temperature(float64(value.gpu))
			buff = append(buff, r.Output...)
		}
	case "colorpulse":
		{
			r.Colorpulse(startTime)
			buff = append(buff, r.Output...)
		}
	case "static":
		{
			r.Static()
			buff = append(buff, r.Output...)
		}
	case "rotator":
		{
			r.Rotator(startTime)
			buff = append(buff, r.Output...)
		}
	case "wave":
		{
			r.Wave(startTime)
			buff = append(buff, r.Output...)
		}
	case "storm":
		{
			r.Storm()
			buff = append(buff, r.Output...)
		}
	case "flickering":
		{

			r.Flickering(startTime)
			buff = append(buff, r.Output...)
		}
	case "flame":
		{

			r.Flame(startTime)
			buff = append(buff, r.Output...)
		}
	case "aurora":
		{

			r.Aurora(startTime)
			buff = append(buff, r.Output...)
		}
	case "cyberpunkglitch":
		{

			r.CyberpunkGlitch(startTime)
			buff = append(buff, r.Output...)
		}
	case "tokyonight":
		{

			r.TokyoNight(startTime)
			buff = append(buff, r.Output...)
		}
	case "colorshift":
		{
			r.Colorshift(startTime, owner)
			buff = append(buff, r.Output...)
		}
	case "circleshift":
		{
			r.CircleShift(startTime)
			buff = append(buff, r.Output...)
		}
	case "circle":
		{
			r.Circle(startTime)
			buff = append(buff, r.Output...)
		}
	case "spinner":
		{
			r.Spinner(startTime)
			buff = append(buff, r.Output...)
		}
	case "colorwarp":
		{
			r.Colorwarp(startTime, owner)
			buff = append(buff, r.Output...)
		}
	}
	if len(buff) < 3 {
		return nil
	}
	for _, zone := range value.zones {
		for m, index := range zone.ColorIndex {
			if m < len(buff) && index >= 0 && index < len(frame) {
				frame[index] = buff[m]
			}
		}
	}
	return frame
}
