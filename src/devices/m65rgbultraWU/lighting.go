package m65rgbultraWU

import (
	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
)

// Canonical desired data has no HID, receiver or profile authority. The shared
// stores persist complete values before publishing them to readers.
type lightingEffectStore interface {
	Get(string, string) (lightingsettings.EffectSettings, bool, error)
	Set(string, string, lightingsettings.EffectSettings) error
	Delete(string, string) (bool, error)
	GetAuthoredZones(string, string) (map[string]lightingsettings.Color, bool, error)
	SetAuthoredZones(string, string, map[string]lightingsettings.Color) error
}
type lightingResolver interface {
	Resolve(lightingsettings.Target, string) (lightingsettings.Resolution, error)
}
type canonicalLighting struct {
	state    lightingsettings.IndependentDeviceStateAccess
	effects  lightingEffectStore
	defaults *lightingsettings.DefaultRepository
	resolver lightingResolver
}

func lightingColor(c rgb.Color) lightingsettings.Color {
	return lightingsettings.Color{Red: c.Red, Green: c.Green, Blue: c.Blue}
}

func lightingHex(c lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(c.Red), uint8(c.Green), uint8(c.Blue))
}

func validateLightingColor(c lightingsettings.Color) error {
	return lightingsettings.Validate(lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: c}})
}

func validateLightingPalette(p map[string]lightingsettings.Color) error {
	if len(p) != 1 {
		return fmt.Errorf("complete M65 RGB ULTRA WU palette required")
	}
	for i := 0; i < 1; i++ {
		c, ok := p[strconv.Itoa(i)]
		if !ok {
			return fmt.Errorf("missing zone %d", i)
		}
		if err := validateLightingColor(c); err != nil {
			return err
		}
	}
	return nil
}

func (s *canonicalLighting) palette(serial string) (map[string]lightingsettings.Color, error) {
	p, found, err := s.effects.GetAuthoredZones(serial, "mouse")
	if err != nil {
		return nil, err
	}
	if !found {
		p = defaultLightingPalette()
	}
	if err = validateLightingPalette(p); err != nil {
		return nil, err
	}
	// Do not rely on an implementation of the store interface to copy its maps.
	copy := make(map[string]lightingsettings.Color, 1)
	for id, c := range p {
		copy[id] = c
	}
	return copy, nil
}

func (d *Device) LightingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) attachLightingRuntime(paths config.Paths) error {
	rt, err := lightingsettings.LoadIndependentDeviceRuntime(paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile, filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		return err
	}
	return d.attachLightingSource(rt)
}

// Attachment is data-only, before either renderer starts. Missing records use
// clean defaults without writing or importing legacy RGB/profile customization.
func (d *Device) attachLightingSource(rt *lightingsettings.IndependentDeviceRuntime) error {
	if d == nil || rt == nil || rt.State == nil || rt.Effects == nil || rt.Defaults == nil || rt.Resolver == nil {
		return fmt.Errorf("canonical Lighting unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.lightingStopped {
		return fmt.Errorf("device stopped")
	}
	if d.lightingCanonical != nil {
		return nil
	}
	candidate := &canonicalLighting{rt.State, rt.Effects, rt.Defaults, rt.Resolver}
	if err := d.validateLightingSource(candidate); err != nil {
		return err
	}
	if d.DeviceProfile == nil || d.activeRgb != nil {
		return fmt.Errorf("profile backing or idle legacy renderer required")
	}
	if d.DeviceProfile.RGBCluster && d.DeviceProfile.OpenRGBIntegration {
		return fmt.Errorf("conflicting external owners")
	}
	d.userRGBOff = d.DeviceProfile.RgbOff
	d.lightingBlankPending = d.userRGBOff || d.DeviceProfile.OpenRGBIntegration
	d.owner = newLightingSession(d.DeviceProfile.RGBCluster, d.DeviceProfile.OpenRGBIntegration)
	d.lightingQueue = make(chan *lightingOutput, 1)
	d.lightingStop = make(chan struct{})
	d.lightingDone = make(chan struct{})
	d.captureIndicatorLocked()
	d.lightingCanonical = candidate
	return nil
}

func (d *Device) validateLightingSource(s *canonicalLighting) error {
	if d.Serial == "" || d.LEDChannels != 2 || d.ChangeableLedChannels != 2 || d.ZoneAmount != 1 || s == nil {
		return fmt.Errorf("M65 RGB ULTRA WU Lighting unavailable")
	}
	if !slices.Equal(rgbModes, lightingEffects()) {
		return fmt.Errorf("invalid Lighting catalogue")
	}
	for _, effect := range rgbModes {
		if effect == "mouse" {
			continue
		}
		if _, err := s.defaults.Get(effect); err != nil {
			return err
		}
		if _, ok := rgb.SoftwareEffectDescriptorByID(effect); !ok {
			return fmt.Errorf("missing presentation %s", effect)
		}
		resolved, err := s.resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), effect)
		if err != nil {
			return err
		}
		if resolved.Settings.EffectID != effect {
			return fmt.Errorf("invalid effect identity")
		}
		if err = lightingsettings.Validate(resolved.Settings); err != nil {
			return err
		}
	}
	state, _, err := s.state.Resolve(d.Serial)
	if err != nil {
		return err
	}
	if err = lightingsettings.ValidateIndependentDeviceLightingState(state); err != nil {
		return err
	}
	if !d.SupportsLightingEffect(state.SelectedEffect) {
		return fmt.Errorf("unsupported effect %q", state.SelectedEffect)
	}
	_, err = s.palette(d.Serial)
	return err
}

func (d *Device) lightingAttached() bool {
	if d == nil {
		return false
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.lightingCanonical != nil
}

func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if d == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("device unavailable")
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if effect == "mouse" || !d.SupportsLightingEffect(effect) {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported settings %q", effect)
	}
	if err := d.validateLightingSource(d.lightingCanonical); err != nil {
		return lightingsettings.EffectSettings{}, err
	}
	value, err := d.lightingCanonical.resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), effect)
	return value.Settings.Clone(), err
}

func (d *Device) SetLightingZoneColor(effect, scope, zoneID, groupID string, color rgb.Color) error {
	if groupID != "" {
		return fmt.Errorf("groups unsupported")
	}
	if scope == "all" && zoneID == "" {
		return d.SetLightingZoneColors(effect, []string{"0"}, color)
	}
	if scope != "zone" {
		return fmt.Errorf("invalid zone scope")
	}
	return d.SetLightingZoneColors(effect, []string{zoneID}, color)
}

func (d *Device) SetLightingZoneColors(effect string, ids []string, color rgb.Color) error {
	updates := make(map[string]lightingsettings.Color, len(ids))
	for _, id := range ids {
		if _, duplicate := updates[id]; duplicate {
			return fmt.Errorf("duplicate zone")
		}
		updates[id] = lightingColor(color)
	}
	return d.updateLightingPalette(effect, updates)
}

func (d *Device) updateLightingPalette(effect string, updates map[string]lightingsettings.Color) error {
	return d.mutateLighting(func(s *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if effect != "mouse" || len(updates) == 0 {
			return false, fmt.Errorf("invalid Mouse palette mutation")
		}
		palette, err := s.palette(d.Serial)
		if err != nil {
			return false, err
		}
		for id, c := range updates {
			if _, ok := palette[id]; !ok {
				return false, fmt.Errorf("unknown zone %q", id)
			}
			palette[id] = c
		}
		if err = validateLightingPalette(palette); err != nil {
			return false, err
		}
		return state.SelectedEffect == "mouse", s.effects.SetAuthoredZones(d.Serial, "mouse", palette)
	})
}

func copyLightingRGBProfile(p rgb.Profile) rgb.Profile {
	if p.Gradients != nil {
		colors := make(map[int]rgb.Color, len(p.Gradients))
		for id, c := range p.Gradients {
			colors[id] = c
		}
		p.Gradients = colors
	}
	return p
}

func lightingResult(err error) uint8 {
	if err != nil {
		return 0
	}
	return 1
}

func (d *Device) resolveLightingSettings(effect string) (lightingsettings.Resolution, error) {
	if d.lightingCanonical == nil || effect == "mouse" || !d.SupportsLightingEffect(effect) {
		return lightingsettings.Resolution{}, fmt.Errorf("unsupported M65 RGB ULTRA WU settings %q", effect)
	}
	return d.lightingCanonical.resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), effect)
}

func (d *Device) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	if d == nil {
		return lightingpresentation.Snapshot{}, false
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.lightingStopped || d.validateLightingSource(d.lightingCanonical) != nil {
		return lightingpresentation.Snapshot{}, false
	}
	state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
	if err != nil {
		return lightingpresentation.Snapshot{}, false
	}
	snapshot := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: state.SelectedEffect, EffectSupported: d.SupportsLightingEffect(state.SelectedEffect), EffectSelectionAvailable: true, ClusterControlled: d.owner.kind == lightingCluster, ExternalControlled: d.owner.kind == lightingOpenRGB, HasBrightness: true, Brightness: state.Brightness}
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
		palette, err := d.lightingCanonical.palette(d.Serial)
		if err != nil {
			return lightingpresentation.Snapshot{}, false
		}
		snapshot.AuthoredZoneEditor = &lightingpresentation.AuthoredZoneEditor{EffectID: "mouse", Heading: "Zones", Description: "Choose colors for the selected zones."}
		for id, zone := range lightingTopology() {
			snapshot.AuthoredZoneEditor.Zones = append(snapshot.AuthoredZoneEditor.Zones, lightingpresentation.AuthoredZone{ID: strconv.Itoa(id), Label: zone.Name, ColorHex: lightingHex(palette[strconv.Itoa(id)])})
		}
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
		snapshot.SingleColorHex = lightingHex(settings.SingleColor.Color)
	}
	if settings.TwoColor != nil {
		snapshot.TwoColorStartHex, snapshot.TwoColorEndHex = lightingHex(settings.TwoColor.Start), lightingHex(settings.TwoColor.End)
	}
	if settings.Temperature != nil {
		snapshot.HasTemperature = true
		snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: lightingHex(settings.Temperature.Low.Color), Celsius: settings.Temperature.Low.Celsius}
		snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: lightingHex(settings.Temperature.Middle.Color), Celsius: settings.Temperature.Middle.Celsius}
		snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: lightingHex(settings.Temperature.High.Color), Celsius: settings.Temperature.High.Celsius}
	}
	if settings.Gradient != nil {
		snapshot.HasGradient = true
		for _, stop := range settings.Gradient.Stops {
			snapshot.GradientStops = append(snapshot.GradientStops, lightingpresentation.GradientStop{Position: stop.Position, Intensity: stop.Intensity, ColorHex: lightingHex(stop.Color)})
		}
	}
	return snapshot, true
}

func (d *Device) SetLightingEffect(effect string) error {
	var profilePath string
	err := d.mutateLighting(func(source *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if !d.SupportsLightingEffect(effect) {
			return false, fmt.Errorf("unsupported effect %q", effect)
		}
		if effect != "mouse" {
			if _, err := d.resolveLightingSettings(effect); err != nil {
				return false, err
			}
		}
		if d.DeviceProfile != nil {
			profilePath = d.DeviceProfile.Path
		}
		state.SelectedEffect = effect
		return true, source.state.Set(d.Serial, state)
	})
	if err != nil {
		return err
	}
	d.lightingMu.Lock()
	recoverEndpoint := d.lightingEndpointAttempted && !d.lightingHardwareReady && !d.lightingRecovering
	d.lightingMu.Unlock()
	if recoverEndpoint {
		if !d.recoverCanonicalLightingEndpoint(nil) {
			return fmt.Errorf("Lighting settings saved, but color endpoint recovery failed")
		}
		d.lightingMu.Lock()
		if !d.lightingStarted {
			d.refreshCanonicalLightingLocked(false)
		}
		d.lightingMu.Unlock()
	}
	// Resume only the profile selected at the mutation boundary. A switch may
	// have completed while endpoint recovery ran without the preference lock.
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if !d.lightingStopped && d.DeviceProfile != nil && d.DeviceProfile.Path == profilePath && d.owner.kind == lightingNative && d.userRGBOff {
		return d.setUserRGBOffLocked(false)
	}
	return nil
}

func (d *Device) SetLightingBrightness(value uint8) error {
	return d.mutateLighting(func(source *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if value > 100 {
			return false, fmt.Errorf("brightness must be between 0 and 100")
		}
		state.Brightness = value
		return true, source.state.Set(d.Serial, state)
	})
}

func (d *Device) SetLightingEffectSettings(effect string, value lightingsettings.EffectSettings) error {
	value = value.Clone()
	return d.mutateLighting(func(source *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if effect == "mouse" || !d.SupportsLightingEffect(effect) || value.EffectID != effect {
			return false, fmt.Errorf("invalid effect settings identity")
		}
		if err := lightingsettings.Validate(value); err != nil {
			return false, err
		}
		return state.SelectedEffect == effect, source.effects.Set(d.Serial, effect, value)
	})
}

// Reset is an explicit recovery write. It must not resolve customized settings
// (including the target's corrupt value) through catalogue-wide validation first.
func (d *Device) ResetLightingEffectSettings(effect string) error {
	if d == nil {
		return fmt.Errorf("device unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	source := d.lightingCanonical
	if d.lightingStopped || source == nil || source.state == nil || source.effects == nil || source.defaults == nil || source.resolver == nil {
		return fmt.Errorf("Lighting unavailable")
	}
	if !d.SupportsLightingEffect(effect) {
		return fmt.Errorf("unsupported reset effect %q", effect)
	}
	value, err := source.defaults.Get(effect)
	if err != nil {
		return err
	}
	if value.EffectID != effect {
		return fmt.Errorf("invalid default effect identity")
	}
	if err = lightingsettings.Validate(value); err != nil {
		return err
	}
	state, _, err := source.state.Resolve(d.Serial)
	if err != nil {
		return err
	}
	if _, err = source.effects.Delete(d.Serial, effect); err != nil {
		return err
	}
	if state.SelectedEffect == effect && d.owner.kind == lightingNative {
		d.refreshCanonicalLightingLocked(false)
	}
	return nil
}

func (d *Device) changeLightingGradient(effect string, remove bool) (uint8, uint) {
	var key uint
	minimum := false
	err := d.mutateLighting(func(source *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if effect != "gradient" {
			return false, fmt.Errorf("not a Gradient")
		}
		resolved, err := d.resolveLightingSettings(effect)
		if err != nil {
			return false, err
		}
		if remove {
			value := resolved.Settings.Clone()
			gradient := value.Gradient
			if len(gradient.Stops) < 3 {
				minimum = true
				return false, nil
			}
			// The compatibility API deletes the highest authored key. Remove
			// that pair directly; a map round-trip would reorder tied stops.
			if len(gradient.Keys) == 0 {
				for i := range gradient.Stops {
					gradient.Keys = append(gradient.Keys, i)
				}
			}
			index := 0
			for i, k := range gradient.Keys {
				if k > gradient.Keys[index] {
					index = i
				}
			}
			key = uint(gradient.Keys[index])
			gradient.Keys = append(gradient.Keys[:index], gradient.Keys[index+1:]...)
			gradient.Stops = append(gradient.Stops[:index], gradient.Stops[index+1:]...)
			return state.SelectedEffect == effect, source.effects.Set(d.Serial, effect, value)
		}
		value := resolved.Settings.Clone()
		gradient := value.Gradient
		if len(gradient.Keys) == 0 {
			for i := range gradient.Stops {
				gradient.Keys = append(gradient.Keys, i)
			}
		}
		next := 0
		for _, k := range gradient.Keys {
			if k >= next {
				next = k + 1
			}
		}
		// Split the largest existing interval without sorting authored ties.
		index, gap := 1, -1.0
		for i := 1; i < len(gradient.Stops); i++ {
			if delta := gradient.Stops[i].Position - gradient.Stops[i-1].Position; delta > gap {
				index, gap = i, delta
			}
		}
		stop := lightingsettings.GradientStop{Position: (gradient.Stops[index-1].Position + gradient.Stops[index].Position) / 2, Color: lightingsettings.Color{Green: 255, Blue: 255}}
		gradient.Keys = append(gradient.Keys, 0)
		copy(gradient.Keys[index+1:], gradient.Keys[index:])
		gradient.Keys[index] = next
		gradient.Stops = append(gradient.Stops, lightingsettings.GradientStop{})
		copy(gradient.Stops[index+1:], gradient.Stops[index:])
		gradient.Stops[index] = stop
		key = uint(next)
		return state.SelectedEffect == effect, source.effects.Set(d.Serial, effect, value)
	})
	if err != nil {
		return 0, 0
	}
	if minimum {
		return 2, 0
	}
	return 1, key
}

func lightingTopology() [1]ZoneColors {
	return [1]ZoneColors{{Name: "Logo", ColorIndex: []int{0, 2, 4}}}
}
func defaultLightingPalette() map[string]lightingsettings.Color {
	return map[string]lightingsettings.Color{"0": {Red: 255, Green: 255}}
}

func cloneLightingProfile(profile DeviceProfile) DeviceProfile {
	if profile.Tilts != nil {
		tilts := make(map[int]TiltOption, len(profile.Tilts))
		for k, v := range profile.Tilts {
			tilts[k] = v
		}
		profile.Tilts = tilts
	}
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

func (d *Device) SupportsLightingEffect(effect string) bool { return slices.Contains(rgbModes, effect) }

// No transport operation occurs while a mutation/store lock is held. A failed
// write leaves worker identity, effective output and desired state untouched.
func (d *Device) mutateLighting(change func(*canonicalLighting, lightingsettings.IndependentDeviceLightingState) (bool, error)) error {
	if d == nil {
		return fmt.Errorf("device unavailable")
	}
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	if d.lightingStopped || d.validateLightingSource(d.lightingCanonical) != nil {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return fmt.Errorf("Lighting unavailable")
	}
	state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
	apply := false
	if err == nil {
		apply, err = change(d.lightingCanonical, state)
	}
	if err == nil && apply {
		d.refreshCanonicalLightingLocked(false)
	}
	d.lightingMu.Unlock()
	d.lightingMutation.Unlock()
	return err
}

func (d *Device) canonicalRGBProfile(effect string) *rgb.Profile {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if effect == "mouse" {
		return &rgb.Profile{ProfileName: "mouse", Brightness: 1}
	}
	resolved, err := d.resolveLightingSettings(effect)
	if err != nil {
		return nil
	}
	p := wuRendererProfile(resolved.Settings.Clone())
	return &p
}
func (d *Device) canonicalRGBProfiles() interface{} {
	result := rgb.RGB{Device: d.Product, Profiles: make(map[string]rgb.Profile)}
	for _, effect := range rgbModes {
		if p := d.canonicalRGBProfile(effect); p != nil {
			result.Profiles[effect] = *p
		}
	}
	return result
}
func (d *Device) updateCanonicalRGBProfile(effect string, profile rgb.Profile) uint8 {
	if effect == "mouse" || !d.SupportsLightingEffect(effect) {
		return 0
	}
	if effect == "gradient" {
		profile = copyLightingRGBProfile(profile)
		return lightingResult(d.mutateLighting(func(source *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
			resolved, err := d.resolveLightingSettings(effect)
			if err != nil {
				return false, err
			}
			order := append([]int(nil), resolved.Settings.Gradient.Keys...)
			if len(order) == 0 {
				for i := range resolved.Settings.Gradient.Stops {
					order = append(order, i)
				}
			}
			settings, err := wuGradientSettings(profile, order)
			if err != nil {
				return false, err
			}
			return state.SelectedEffect == effect, source.effects.Set(d.Serial, effect, settings)
		}))
	}
	settings, err := lightingsettings.EffectSettingsFromRGBProfile(effect, copyLightingRGBProfile(profile))
	if err != nil {
		return 0
	}
	return lightingResult(d.SetLightingEffectSettings(effect, settings))
}

func wuRendererProfile(value lightingsettings.EffectSettings) rgb.Profile {
	p := lightingsettings.RendererProfileFromEffectSettings(value)
	if value.Gradient != nil && len(value.Gradient.Keys) > 0 {
		colors := make(map[int]rgb.Color, len(p.Gradients))
		for i, k := range value.Gradient.Keys {
			colors[k] = p.Gradients[i]
		}
		p.Gradients = colors
	}
	return p
}
func wuGradientSettings(p rgb.Profile, authoredOrder []int) (lightingsettings.EffectSettings, error) {
	keys := make([]int, 0, len(p.Gradients))
	for k := range p.Gradients {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	// Position edits move stops; equal positions retain existing authored order.
	// New keys follow existing ties in numeric order from the initial key sort.
	rank := make(map[int]int, len(authoredOrder))
	for i, k := range authoredOrder {
		rank[k] = i
	}
	sort.SliceStable(keys, func(i, j int) bool {
		left, right := p.Gradients[keys[i]].Position, p.Gradients[keys[j]].Position
		if left != right {
			return left < right
		}
		a, ok := rank[keys[i]]
		if !ok {
			a = len(authoredOrder)
		}
		b, ok := rank[keys[j]]
		if !ok {
			b = len(authoredOrder)
		}
		return a < b
	})
	value := lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "gradient", Speed: &p.Speed, Gradient: &lightingsettings.GradientSettings{Keys: keys}}
	for _, k := range keys {
		c := p.Gradients[k]
		value.Gradient.Stops = append(value.Gradient.Stops, lightingsettings.GradientStop{Position: c.Position, Color: lightingColor(c), Intensity: c.Brightness})
	}
	return value, lightingsettings.Validate(value)
}

// WU catalogue, independently traced through its legacy dispatch switch.
func lightingEffects() []string {
	return []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
}

// Only retained profile-owned preferences use this writer. Legacy desired
// Lighting fields remain serialized unchanged, with no behavioral authority.
func (d *Device) persistLightingPreference(p DeviceProfile) error {
	p = cloneLightingProfile(p)
	if d.lightingProfileWrite != nil {
		return d.lightingProfileWrite(p)
	}
	data, err := json.MarshalIndent(p, "", "    ")
	if err != nil {
		return err
	}
	return writeWUPreferenceFile(p.Path, data, d.lightingProfileFileWrite)
}

func (d *Device) setUserRGBOff(off bool) error {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.setUserRGBOffLocked(off)
}

// Caller holds lightingMutation and lightingMu through the decision and save.
func (d *Device) setUserRGBOffLocked(off bool) error {
	if d.lightingStopped || d.DeviceProfile == nil {
		return fmt.Errorf("profile unavailable")
	}
	p := cloneLightingProfile(*d.DeviceProfile)
	p.RgbOff = off
	err := d.persistLightingPreference(p)
	if err == nil {
		d.DeviceProfile.RgbOff = off
		d.userRGBOff = off
		d.refreshCanonicalLightingLocked(off)
	}
	return err
}

func (d *Device) setSchedulerDark(dark bool) error {
	d.lightingMu.Lock()
	if d.lightingStopped {
		d.lightingMu.Unlock()
		return fmt.Errorf("device stopped")
	}
	changed := d.schedulerDark != dark
	d.schedulerDark = dark
	if changed {
		d.refreshCanonicalLightingLocked(false)
	}
	d.lightingMu.Unlock()
	return nil
}

// Only the canonical RGB-off/ownership preference seam uses replacement. Other
// legacy profile saves retain their original behavior and filenames.
func writeWUPreferenceFile(path string, data []byte, write func(*os.File, []byte) (int, error)) error {
	if path == "" {
		return fmt.Errorf("profile path unavailable")
	}
	mode := os.FileMode(0600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".m65-ultra-wu-preference-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode); err == nil {
		var n int
		if write == nil {
			n, err = f.Write(data)
		} else {
			n, err = write(f, data)
		}
		if err == nil && n != len(data) {
			err = io.ErrShortWrite
		}
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
