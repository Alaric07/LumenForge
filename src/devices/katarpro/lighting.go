package katarpro

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

type katarEffectStore interface {
	Get(string, string) (lightingsettings.EffectSettings, bool, error)
	Set(string, string, lightingsettings.EffectSettings) error
	Delete(string, string) (bool, error)
	GetAuthoredZones(string, string) (map[string]lightingsettings.Color, bool, error)
	SetAuthoredZones(string, string, map[string]lightingsettings.Color) error
}
type katarSettingsResolver interface {
	Resolve(lightingsettings.Target, string) (lightingsettings.Resolution, error)
}
type katarLighting struct {
	state    lightingsettings.IndependentDeviceStateAccess
	effects  katarEffectStore
	resolver katarSettingsResolver
	defaults *lightingsettings.DefaultRepository
}

// Clean attachment validates every dependency before publication. Loading missing
// canonical records resolves clean defaults without reading or importing legacy RGB.
func (d *Device) attachLightingRuntime(paths config.Paths) error {
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile, filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		return err
	}
	return d.attachLightingSource(runtime)
}
func (d *Device) attachLightingSource(runtime *lightingsettings.IndependentDeviceRuntime) error {
	if d == nil || runtime == nil || runtime.State == nil || runtime.Effects == nil || runtime.Resolver == nil || runtime.Defaults == nil {
		return fmt.Errorf("KATAR PRO canonical runtime unavailable")
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
	candidate := &katarLighting{runtime.State, runtime.Effects, runtime.Resolver, runtime.Defaults}
	state, _, err := candidate.state.Resolve(d.Serial)
	if err != nil {
		return err
	}
	if err = lightingsettings.ValidateIndependentDeviceLightingState(state); err != nil {
		return err
	}
	if !d.SupportsLightingEffect(state.SelectedEffect) {
		return fmt.Errorf("unsupported KATAR PRO effect %q", state.SelectedEffect)
	}
	for _, effect := range rgbModes {
		if effect == "mouse" {
			continue
		}
		if _, ok := rgb.SoftwareEffectDescriptorByID(effect); !ok {
			return fmt.Errorf("missing effect descriptor %q", effect)
		}
		if _, err = candidate.defaults.Get(effect); err != nil {
			return err
		}
		value, resolveErr := candidate.resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), effect)
		if resolveErr != nil {
			return resolveErr
		}
		if err = lightingsettings.Validate(value.Settings); err != nil {
			return err
		}
	}
	if _, err = canonicalScroll(candidate, d.Serial); err != nil {
		return err
	}
	d.lightingCanonical = candidate
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
		return fmt.Errorf("KATAR PRO lighting is unavailable")
	}
	return nil
}
func (d *Device) lightingReady() error {
	if err := d.lightingHardwareReady(); err != nil {
		return err
	}
	if d.lightingCanonical == nil {
		return fmt.Errorf("KATAR PRO canonical attachment is unavailable")
	}
	state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
	if err != nil {
		return err
	}
	if err = lightingsettings.ValidateIndependentDeviceLightingState(state); err != nil {
		return err
	}
	if !d.SupportsLightingEffect(state.SelectedEffect) {
		return fmt.Errorf("unsupported selected effect")
	}
	_, err = d.canonicalScroll()
	return err
}
func canonicalColor(color rgb.Color) lightingsettings.Color {
	return lightingsettings.Color{Red: color.Red, Green: color.Green, Blue: color.Blue}
}
func lightingColorHex(color lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
}
func validateScroll(palette map[string]lightingsettings.Color) error {
	color, ok := palette["0"]
	if !ok || len(palette) != 1 {
		return fmt.Errorf("invalid KATAR PRO Scroll palette")
	}
	return lightingsettings.Validate(lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: color}})
}
func (d *Device) canonicalScroll() (rgb.Color, error) {
	return canonicalScroll(d.lightingCanonical, d.Serial)
}
func canonicalScroll(source *katarLighting, serial string) (rgb.Color, error) {
	colors, found, err := source.effects.GetAuthoredZones(serial, "mouse")
	if err != nil {
		return rgb.Color{}, err
	}
	if !found {
		colors = map[string]lightingsettings.Color{"0": {Green: 255, Blue: 255}}
	} // source-backed immutable Scroll default
	if err = validateScroll(colors); err != nil {
		return rgb.Color{}, err
	}
	color := colors["0"]
	return rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue, Brightness: 1, Hex: lightingColorHex(color)}, nil
}
func (d *Device) resolveLightingSettings(effect string) (lightingsettings.Resolution, error) {
	if d.lightingCanonical == nil || effect == "mouse" || !d.SupportsLightingEffect(effect) {
		return lightingsettings.Resolution{}, fmt.Errorf("unsupported KATAR PRO settings %q", effect)
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
		color, colorErr := d.canonicalScroll()
		if colorErr != nil {
			return lightingpresentation.Snapshot{}, false
		}
		snapshot.AuthoredZoneEditor = &lightingpresentation.AuthoredZoneEditor{EffectID: "mouse", Heading: "Zones", Description: "Choose a color for the Scroll zone.", Zones: []lightingpresentation.AuthoredZone{{ID: "0", Label: "Scroll", ColorHex: lightingColorHex(canonicalColor(color))}}}
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
func (d *Device) mutateLighting(change func(*katarLighting, lightingsettings.IndependentDeviceLightingState) (bool, error)) error {
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
		d.restartLighting()
	}
	return err
}
func (d *Device) SetLightingEffect(effect string) error {
	return d.mutateLighting(func(source *katarLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
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
	return d.mutateLighting(func(source *katarLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if value > 100 {
			return false, fmt.Errorf("brightness must be between 0 and 100")
		}
		state.Brightness = value
		return state.SelectedEffect == "mouse" || state.SelectedEffect == "static", source.state.Set(d.Serial, state)
	})
}
func (d *Device) SetLightingEffectSettings(effect string, value lightingsettings.EffectSettings) error {
	value = value.Clone()
	return d.mutateLighting(func(source *katarLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
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
	return d.mutateLighting(func(source *katarLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if _, err := source.defaults.Get(effect); err != nil || !d.SupportsLightingEffect(effect) {
			return false, fmt.Errorf("unsupported reset effect %q", effect)
		}
		_, err := source.effects.Delete(d.Serial, effect)
		return state.SelectedEffect == effect, err
	})
}
func (d *Device) SetLightingZoneColor(effect, scope, zoneID, groupID string, color rgb.Color) error {
	if groupID != "" || (scope != "zone" && scope != "all") || (scope == "zone" && zoneID != "0") || (scope == "all" && zoneID != "") {
		return fmt.Errorf("invalid Scroll selection")
	}
	return d.SetLightingZoneColors(effect, []string{"0"}, color)
}
func (d *Device) SetLightingZoneColors(effect string, zones []string, color rgb.Color) error {
	return d.mutateLighting(func(source *katarLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if effect != "mouse" || len(zones) != 1 || zones[0] != "0" {
			return false, fmt.Errorf("invalid Scroll selection")
		}
		palette := map[string]lightingsettings.Color{"0": canonicalColor(color)}
		if err := validateScroll(palette); err != nil {
			return false, err
		}
		return state.SelectedEffect == "mouse", source.effects.SetAuthoredZones(d.Serial, "mouse", palette)
	})
}
func (d *Device) changeLightingGradient(effect string, remove bool) (uint8, uint) {
	var index uint
	minimum := false
	err := d.mutateLighting(func(source *katarLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
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

// Overrides retain KATAR PRO's live-runtime guard and never persist darkness.
func (d *Device) setLightingOverride(scheduler, value bool) uint8 {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if d.lightingReady() != nil {
		d.lightingMu.Unlock()
		return 0
	}
	state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
	if err != nil {
		d.lightingMu.Unlock()
		return 0
	}
	if scheduler {
		d.schedulerDark = value
	} else {
		d.userRGBOff = value
	}
	d.lightingMu.Unlock()
	if !scheduler || state.SelectedEffect == "mouse" || state.SelectedEffect == "static" {
		d.restartLighting()
	}
	return 1
}
func (d *Device) restartLighting() {
	if d.lightingRestart != nil {
		d.lightingRestart()
		return
	}
	d.setDeviceColor()
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
func cloneKatarProfile(profile DeviceProfile) DeviceProfile {
	if profile.BrightnessSlider != nil {
		value := *profile.BrightnessSlider
		profile.BrightnessSlider = &value
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

// Complete frame input is copied under lightingMu. Attached rendering never
// consults DeviceProfile RGBProfile/BrightnessSlider/ZoneColors or legacy RGB.
type katarFrameState struct {
	effect           string
	transientOff     bool
	brightness       uint8
	zones            map[int]ZoneColors
	profile          *rgb.Profile
	indicator        *rgb.Color
	indicatorIndexes map[int][]int
	cpu, gpu         float32
}

func (d *Device) lightingFrameState() (katarFrameState, error) {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.Exit || d.DeviceProfile == nil {
		return katarFrameState{}, fmt.Errorf("lighting output unavailable")
	}
	input := cloneKatarInput(*d.DeviceProfile)
	value := katarFrameState{cpu: d.CpuTemp, gpu: d.GpuTemp}
	if d.lightingCanonical != nil {
		if err := d.lightingHardwareReady(); err != nil {
			return value, err
		}
		state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
		if err != nil {
			return value, err
		}
		value.effect, value.brightness = state.SelectedEffect, state.Brightness
		scroll, err := d.canonicalScroll()
		if err != nil {
			return value, err
		}
		value.zones = map[int]ZoneColors{0: {Name: "Scroll", ColorIndex: []int{0, 1, 2}, Color: &scroll}}
		if value.effect != "mouse" && d.SupportsLightingEffect(value.effect) {
			resolved, err := d.resolveLightingSettings(value.effect)
			if err != nil {
				return value, err
			}
			profile := lightingsettings.RendererProfileFromEffectSettings(resolved.Settings)
			value.profile = &profile
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
		if d.DeviceProfile.BrightnessSlider == nil {
			return value, fmt.Errorf("legacy slider unavailable")
		}
		legacy := cloneKatarProfile(*d.DeviceProfile)
		value.effect, value.brightness, value.zones = legacy.RGBProfile, *legacy.BrightnessSlider, legacy.ZoneColors
		value.profile = d.legacyRgbProfile(value.effect)
		if d.DeviceProfile.RgbOff {
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

func (d *Device) renderLightingFrame(value katarFrameState, owner *rgb.ActiveRGB, startTime *time.Time) []byte {
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
	local := copyLightingRGBProfile(*profile)
	profile = &local
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

// Only input-owned fields are copied for canonical composition.
func cloneKatarInput(profile DeviceProfile) DeviceProfile {
	return cloneKatarProfile(DeviceProfile{Profile: profile.Profile, Profiles: profile.Profiles})
}

// An owner is installed before launch. Missing state is retried at frame cadence.
func (d *Device) setDeviceColor() { d.startDeviceColor(nil) }
func (d *Device) startDeviceColor(flash *rgb.Color) {
	if d.stopped() {
		return
	}
	// Capture a value before handing the optional input overlay to a goroutine.
	if flash != nil {
		color := *flash
		flash = &color
	}
	owner, stop, done := d.startLightingRenderer()
	go func() {
		defer close(done)
		defer d.retireLightingRenderer(owner, stop)
		startTime := time.Now()
		owner.RGBStartColor, owner.RGBEndColor = rgb.GenerateRandomColor(1), rgb.GenerateRandomColor(1)
		if flash != nil {
			frame := []byte{byte(flash.Red), byte(flash.Green), byte(flash.Blue)}
			d.writeColor(frame, stop)
			if !lightingDelay(stop, dpiFlashDuration) {
				return
			}
		}
		for {
			select {
			case <-stop:
				return
			default:
			}
			if d.stopped() {
				return
			}
			value, err := d.lightingFrameState()
			if err == nil {
				frame := d.renderLightingFrame(value, owner, &startTime)
				if len(frame) != 0 {
					d.writeColor(frame, stop)
					if value.transientOff || value.effect == "mouse" || value.effect == "static" {
						return
					}
				}
			}
			if !lightingDelay(stop, 40*time.Millisecond) {
				return
			}
		}
	}()
}

const dpiFlashDuration = 500 * time.Millisecond
