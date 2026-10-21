package glaivergb

import (
	"LumenForge/src/cluster"
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/openrgb"
	"LumenForge/src/rgb"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"time"
)

type lightingEffectStore interface {
	Get(string, string) (lightingsettings.EffectSettings, bool, error)
	Set(string, string, lightingsettings.EffectSettings) error
	Delete(string, string) (bool, error)
	GetAuthoredZones(string, string) (map[string]lightingsettings.Color, bool, error)
	SetAuthoredZones(string, string, map[string]lightingsettings.Color) error
	SetAuthoredZonesWithCommit(string, string, map[string]lightingsettings.Color, func() error) error
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
	if len(p) != 3 {
		return fmt.Errorf("complete GLAIVE RGB palette required")
	}
	for i := 0; i < 3; i++ {
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
	copy := make(map[string]lightingsettings.Color, 3)
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

func (d *Device) attachLightingSource(rt *lightingsettings.IndependentDeviceRuntime) error {
	if d == nil || rt == nil || rt.State == nil || rt.Effects == nil || rt.Defaults == nil || rt.Resolver == nil {
		return fmt.Errorf("canonical Lighting unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.lightingCanonical != nil {
		return nil
	}
	candidate := &canonicalLighting{rt.State, rt.Effects, rt.Defaults, rt.Resolver}
	// Attachment is data-only. This package opens HID in Init; readiness gates
	// applying output, never resolving or persisting desired Lighting.
	if err := d.validateLightingSource(candidate); err != nil {
		return err
	}
	if d.DeviceProfile == nil {
		return fmt.Errorf("RGB-off preference backing is unavailable")
	}
	// Preferences and ownership retain profile persistence. Authored Lighting
	// still resolves exclusively from the validated canonical candidate.
	if d.DeviceProfile.RGBCluster && d.DeviceProfile.OpenRGBIntegration {
		return fmt.Errorf("conflicting external owners")
	}
	d.userRGBOff = d.DeviceProfile.RgbOff
	d.clusterOwned, d.openRGBOwned = d.DeviceProfile.RGBCluster, d.DeviceProfile.OpenRGBIntegration
	d.externalGeneration++
	d.externalQueue = make(chan lightingExternalFrame, 10)
	d.externalStop = make(chan struct{})
	d.lightingCanonical = candidate
	return nil
}

func (d *Device) validateLightingSource(s *canonicalLighting) error {
	if d.Serial == "" || d.LEDChannels != 0 || d.ChangeableLedChannels != 3 || d.ZoneAmount != 3 || s == nil {
		return fmt.Errorf("GLAIVE RGB Lighting unavailable")
	}
	if len(rgbModes) != 20 {
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
		return d.SetLightingZoneColors(effect, []string{"0", "1", "2"}, color)
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
		return lightingsettings.Resolution{}, fmt.Errorf("unsupported GLAIVE RGB settings %q", effect)
	}
	return d.lightingCanonical.resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), effect)
}

func (d *Device) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	if d == nil {
		return lightingpresentation.Snapshot{}, false
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.validateLightingSource(d.lightingCanonical) != nil {
		return lightingpresentation.Snapshot{}, false
	}
	state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
	if err != nil {
		return lightingpresentation.Snapshot{}, false
	}
	snapshot := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: state.SelectedEffect, EffectSupported: d.SupportsLightingEffect(state.SelectedEffect), EffectSelectionAvailable: true, ClusterControlled: d.clusterOwned, ExternalControlled: d.openRGBOwned, HasBrightness: true, Brightness: state.Brightness}
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
	return d.mutateLighting(func(source *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
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

func (d *Device) ResetLightingEffectSettings(effect string) error {
	return d.mutateLighting(func(source *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if _, err := source.defaults.Get(effect); err != nil || !d.SupportsLightingEffect(effect) {
			return false, fmt.Errorf("unsupported reset effect %q", effect)
		}
		_, err := source.effects.Delete(d.Serial, effect)
		return state.SelectedEffect == effect, err
	})
}

func (d *Device) changeLightingGradient(effect string, remove bool) (uint8, uint) {
	var index uint
	minimum := false
	err := d.mutateLighting(func(source *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
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

// Source selectors/byte positions stay below canonical logical RGB frames.
func lightingTopology() [3]ZoneColors {
	return [3]ZoneColors{
		{Name: "Front", LEDIndex: 1, LEDIndexPosition: 0, ColorIndex: []int{1, 2, 3}},
		{Name: "Logo", LEDIndex: 2, LEDIndexPosition: 4, ColorIndex: []int{5, 6, 7}},
		{Name: "Side", LEDIndex: 6, LEDIndexPosition: 8, ColorIndex: []int{9, 10, 11}},
	}
}
func defaultLightingPalette() map[string]lightingsettings.Color {
	return map[string]lightingsettings.Color{"0": {Red: 255}, "1": {Red: 255, Green: 255}, "2": {Green: 255, Blue: 255}}
}

func cloneLightingProfile(profile DeviceProfile) DeviceProfile {
	if profile.BrightnessSlider != nil {
		value := *profile.BrightnessSlider
		profile.BrightnessSlider = &value
	}
	if profile.DPIColor != nil {
		color := *profile.DPIColor
		profile.DPIColor = &color
	}
	if profile.SniperColor != nil {
		c := *profile.SniperColor
		profile.SniperColor = &c
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

// Called under lightingMu. Desired data remains available without HID and under
// external ownership. Failed store writes never retire the active renderer.
func (d *Device) mutateLighting(change func(*canonicalLighting, lightingsettings.IndependentDeviceLightingState) (bool, error)) error {
	if d == nil {
		return fmt.Errorf("device unavailable")
	}
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	if d.Exit || d.validateLightingSource(d.lightingCanonical) != nil {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return fmt.Errorf("Lighting unavailable")
	}
	state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
	apply := false
	if err == nil {
		apply, err = change(d.lightingCanonical, state)
	}
	apply = err == nil && apply && !d.clusterOwned && !d.openRGBOwned
	if apply {
		d.cancelLightingLocked()
	}
	d.lightingMu.Unlock()
	d.lightingMutation.Unlock()
	if apply {
		d.setDeviceColor()
	}
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
	p := lightingsettings.RendererProfileFromEffectSettings(resolved.Settings.Clone())
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
	settings, err := lightingsettings.EffectSettingsFromRGBProfile(effect, copyLightingRGBProfile(profile))
	if err != nil {
		return 0
	}
	return lightingResult(d.SetLightingEffectSettings(effect, settings))
}

// Error-returning atomic seam for retained profile preferences/ownership only.
// Serialized legacy Lighting fields remain unchanged and non-authoritative.
func writeLightingProfile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".glaive-profile-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
func (d *Device) persistLightingProfile(p DeviceProfile) error {
	if p.Path == "" {
		return fmt.Errorf("profile path unavailable")
	}
	data, err := json.MarshalIndent(&p, "", "    ")
	if err != nil {
		return err
	}
	write := d.lightingProfileWrite
	if write == nil {
		write = writeLightingProfile
	}
	return write(p.Path, data)
}
func (d *Device) setSchedulerDark(dark bool) error {
	d.lightingMu.Lock()
	if d.Exit || d.lightingCanonical == nil {
		d.lightingMu.Unlock()
		return fmt.Errorf("Lighting unavailable")
	}
	changed := d.schedulerDark != dark
	d.schedulerDark = dark
	apply := changed && !d.clusterOwned && !d.openRGBOwned
	if apply {
		d.cancelLightingLocked()
	}
	d.lightingMu.Unlock()
	if apply {
		d.setDeviceColor()
	}
	return nil
}
func (d *Device) setUserRGBOff(off bool) error {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	if d.Exit || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return fmt.Errorf("profile unavailable")
	}
	if d.userRGBOff == off {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return nil
	}
	p := cloneLightingProfile(*d.DeviceProfile)
	p.RgbOff = off
	err := d.persistLightingProfile(p)
	if err == nil {
		d.DeviceProfile.RgbOff = off
		d.userRGBOff = off
		d.cancelLightingLocked()
	}
	d.lightingMu.Unlock()
	d.lightingMutation.Unlock()
	// Source RGB-off emits a one-shot black packet even under external ownership;
	// future external frames remain unscaled and may overwrite it.
	if err == nil {
		d.setDeviceColor()
	}
	return err
}

// Native workers and external sessions use independent generations: editing
// desired Native state must never invalidate a valid external controller.
type lightingWorker struct {
	stop       chan struct{}
	done       chan struct{}
	generation uint64
}
type lightingExternalFrame struct {
	data       []byte
	generation uint64
}

func (d *Device) cancelLightingLocked() {
	d.nativeGeneration++
	if d.lightingWorker != nil {
		close(d.lightingWorker.stop)
		d.lightingWorker = nil
	}
}
func (d *Device) stopLighting() { d.lightingMu.Lock(); d.cancelLightingLocked(); d.lightingMu.Unlock() }
func (d *Device) isLightingStopped() bool {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.Exit
}
func (d *Device) exitLighting() {
	d.lightingMu.Lock()
	if !d.Exit {
		d.Exit = true
		d.Connected = false
		d.invalidateExternalLocked()
		d.cancelLightingLocked()
		if d.externalStop != nil {
			close(d.externalStop)
		}
	}
	d.lightingMu.Unlock()
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
func (d *Device) retireLightingWorker(owner *lightingWorker) {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.lightingWorker == owner {
		d.lightingWorker = nil
	}
}

type lightingFrameState struct {
	effect     string
	brightness uint8
	palette    map[string]lightingsettings.Color
	profile    *rgb.Profile
	cpu, gpu   float32
	blank      bool
}

func (d *Device) lightingFrameLocked() (lightingFrameState, error) {
	v := lightingFrameState{cpu: d.CpuTemp, gpu: d.GpuTemp}
	if err := d.validateLightingSource(d.lightingCanonical); err != nil {
		return v, err
	}
	state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
	if err != nil {
		return v, err
	}
	v.effect, v.brightness = state.SelectedEffect, state.Brightness
	v.palette, err = d.lightingCanonical.palette(d.Serial)
	if err != nil {
		return v, err
	}
	if d.schedulerDark || d.userRGBOff {
		v.brightness = 0
	}
	v.blank = d.userRGBOff || d.openRGBOwned
	if v.blank {
		v.effect = "off"
		v.profile = &rgb.Profile{}
		return v, nil
	}
	if v.effect != "mouse" {
		resolved, e := d.resolveLightingSettings(v.effect)
		if e != nil {
			return v, e
		}
		p := lightingsettings.RendererProfileFromEffectSettings(resolved.Settings.Clone())
		v.profile = &p
	}
	return v, nil
}
func composeLightingPacket(rgbData []byte) []byte {
	if len(rgbData) != 9 {
		return nil
	}
	packet := make([]byte, LEDPacketLength)
	for id, zone := range lightingTopology() {
		packet[zone.LEDIndexPosition] = byte(zone.LEDIndex)
		for channel, index := range zone.ColorIndex {
			packet[index] = rgbData[id*3+channel]
		}
	}
	return packet
}

// A queued writer checks identity after acquiring the existing output lock.
// Lifecycle transitions never wait for that lock or an in-flight HID call.
func (d *Device) applyLightingPacket(packet []byte, source string, generation uint64, worker *lightingWorker) {
	if len(packet) != LEDPacketLength {
		return
	}
	if d.lightingBeforeApply != nil {
		d.lightingBeforeApply(source)
	}
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	valid := !d.Exit && d.Connected && (d.dev != nil || d.lightingWrite != nil)
	switch source {
	case "native":
		valid = valid && d.lightingWorker == worker && worker != nil && worker.generation == d.nativeGeneration && !d.clusterOwned && !d.openRGBOwned
	case "blank":
		valid = valid && d.lightingWorker == worker && worker != nil && worker.generation == d.nativeGeneration && (d.userRGBOff || d.openRGBOwned)
	case "cluster":
		valid = valid && d.clusterOwned && generation == d.externalGeneration
	case "openrgb":
		valid = valid && d.openRGBOwned && generation == d.externalGeneration
	default:
		valid = false
	}
	d.lightingMu.Unlock()
	if !valid {
		return
	}
	if d.lightingWrite != nil {
		d.lightingWrite(append([]byte(nil), packet...))
		return
	}
	if _, err := d.transfer(cmdWrite, cmdWriteColor, packet); err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to write Lighting frame")
	}
}
func (d *Device) setDeviceColor() {
	if d.isLightingStopped() {
		return
	}
	if !d.lightingAttached() {
		d.setLegacyDeviceColor()
		return
	}
	d.lightingMu.Lock()
	if d.Exit || !d.Connected || (d.dev == nil && d.lightingWrite == nil) {
		d.lightingMu.Unlock()
		return
	}
	d.cancelLightingLocked()
	// Entry precedence matches the source: OpenRGB/RGB-off blank first, then
	// Cluster suppresses Native. Desired mutations never enter here while external.
	if d.clusterOwned && !d.userRGBOff && !d.openRGBOwned {
		d.lightingMu.Unlock()
		return
	}
	worker := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{}), generation: d.nativeGeneration}
	owner := rgb.Exit()
	// The RGB effect owner is worker-local; activeRgb belongs to legacy fallback.
	d.lightingWorker = worker
	d.lightingMu.Unlock()
	go func() {
		defer close(worker.done)
		defer d.retireLightingWorker(worker)
		owner.RGBStartColor, owner.RGBEndColor = rgb.GenerateRandomColor(1), rgb.GenerateRandomColor(1)
		start := time.Now()
		for {
			select {
			case <-worker.stop:
				return
			default:
			}
			d.lightingMu.Lock()
			value, err := d.lightingFrameLocked()
			d.lightingMu.Unlock()
			if err != nil {
				if !lightingDelay(worker.stop, 40*time.Millisecond) {
					return
				}
				continue
			}
			frame := renderLightingFrame(value, owner, &start)
			source := "native"
			if value.blank {
				source = "blank"
			}
			d.applyLightingPacket(composeLightingPacket(frame), source, 0, worker)
			if value.blank || value.effect == "mouse" || value.effect == "static" {
				return
			}
			if !lightingDelay(worker.stop, 40*time.Millisecond) {
				return
			}
		}
	}()
}

// The owning brightness is applied once. In Colorwarp, retain the established
// algorithm but generate its cached random endpoints at unit brightness to avoid
// scaling both the endpoints and their interpolation.
func renderLightingFrame(value lightingFrameState, owner *rgb.ActiveRGB, start *time.Time) []byte {
	frame := make([]byte, 9)
	if value.blank {
		return frame
	}
	if value.effect == "mouse" || value.effect == "static" {
		factor := float64(value.brightness) / 100
		for id := 0; id < 3; id++ {
			c := value.palette[strconv.Itoa(id)]
			if value.effect == "static" {
				if value.profile == nil {
					return nil
				}
				c = lightingColor(value.profile.StartColor)
			}
			frame[id*3], frame[id*3+1], frame[id*3+2] = byte(c.Red*factor), byte(c.Green*factor), byte(c.Blue*factor)
		}
		return frame
	}
	if value.profile == nil {
		return nil
	}
	profile := copyLightingRGBProfile(*value.profile)
	speed := common.FClamp(profile.Speed, 0.1, 10)
	custom := profile.StartColor != (rgb.Color{}) && profile.EndColor != (rgb.Color{})
	r := rgb.New(3, speed, nil, nil, 1, common.Clamp(profile.Smoothness, 1, 100), time.Duration(speed)*time.Second, custom)
	if custom {
		r.RGBStartColor, r.RGBMiddleColor, r.RGBEndColor = &profile.StartColor, &profile.MiddleColor, &profile.EndColor
	} else {
		r.RGBStartColor, r.RGBMiddleColor, r.RGBEndColor = owner.RGBStartColor, owner.RGBMiddleColor, owner.RGBEndColor
	}
	if r.RGBMiddleColor == nil {
		r.RGBMiddleColor = &rgb.Color{}
	}
	r.RGBBrightness = float64(value.brightness) / 100
	r.RGBStartColor.Brightness, r.RGBEndColor.Brightness, r.RGBMiddleColor.Brightness = r.RGBBrightness, r.RGBBrightness, r.RGBBrightness
	switch value.effect {
	case "off":
		return make([]byte, 9)
	case "rainbow":
		r.Rainbow(*start)
	case "pastelrainbow":
		r.PastelRainbow(*start)
	case "watercolor":
		r.Watercolor(*start)
	case "gradient":
		r.ColorshiftGradient(*start, profile.Gradients, profile.Speed)
	case "cpu-temperature":
		r.MinTemp, r.MaxTemp = profile.MinTemp, profile.MaxTemp
		r.Temperature(float64(value.cpu))
	case "gpu-temperature":
		r.MinTemp, r.MaxTemp = profile.MinTemp, profile.MaxTemp
		r.Temperature(float64(value.gpu))
	case "colorpulse":
		r.Colorpulse(start)
	case "rotator":
		r.Rotator(start)
	case "wave":
		r.Wave(start)
	case "storm":
		r.Storm()
	case "flickering":
		r.Flickering(start)
	case "flame":
		r.Flame(start)
	case "aurora":
		r.Aurora(start)
	case "cyberpunkglitch":
		r.CyberpunkGlitch(start)
	case "tokyonight":
		r.TokyoNight(start)
	case "colorshift":
		r.Colorshift(start, owner)
	case "colorwarp":
		r.RGBBrightness = 1
		r.Colorwarp(start, owner)
		rgb.ModifyBrightnessSlice(r.Output, float64(value.brightness)/100)
	}
	if len(r.Output) != 9 {
		return nil
	}
	return r.Output
}

// Registration closures capture the session, so even callbacks retained by a
// distributor during replacement cannot write into the new profile/owner.
// Caller serializes reconciliation with lightingMutation, never deviceLock.
func (d *Device) refreshLightingRegistrations(clusterOwner, openRGBOwner bool) {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.refreshLightingRegistrationsLocked(clusterOwner, openRGBOwner)
}
func (d *Device) refreshLightingRegistrationsLocked(clusterOwner, openRGBOwner bool) {
	d.lightingMu.Lock()
	generation, enabled, stopped := d.externalGeneration, d.clusterOwned, d.Exit
	registered := d.openRGBRegistered
	if openRGBOwner && !stopped {
		d.openRGBRegistered = true
	}
	d.lightingMu.Unlock()
	if clusterOwner {
		remove := d.lightingClusterRemove
		if remove == nil {
			remove = func(serial string) { cluster.Get().RemoveDeviceControllerBySerial(serial) }
		}
		remove(d.Serial)
		if enabled && !stopped {
			controller := &common.ClusterController{Product: d.Product, Serial: d.Serial, LedChannels: 3, WriteColorEx: func(data []byte, _ int) {
				d.applyLightingPacket(composeLightingPacket(append([]byte(nil), data...)), "cluster", generation, nil)
			}}
			if d.lightingClusterRegister != nil {
				d.lightingClusterRegister(controller)
			} else {
				cluster.Get().AddDeviceController(controller)
			}
		}
	}
	if openRGBOwner && !stopped {
		controller := &common.OpenRGBController{Name: d.Product, Vendor: "Corsair", Description: "LumenForge Backend Device", FwVersion: d.Firmware, Serial: d.Serial, Location: fmt.Sprintf("HID: %s", d.Path), Colors: make([]byte, 9), DeviceType: common.DeviceTypeMouse, ColorMode: common.ColorModePerLed, WriteColorEx: func(data []byte, _ int) { d.enqueueOpenRGB(data, generation) }}
		for _, zone := range lightingTopology() {
			controller.Zones = append(controller.Zones, common.OpenRGBZone{Name: zone.Name, NumLEDs: 1, ZoneType: common.ZoneTypeLinear})
		}
		if d.lightingOpenRGBRegister != nil {
			d.lightingOpenRGBRegister(controller, registered)
		} else if registered {
			openrgb.UpdateDeviceController(d.Serial, controller)
		} else {
			openrgb.AddDeviceController(controller)
		}
	}
}
func (d *Device) setLightingOwnership(clusterOwner, enabled bool) uint8 {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	if d.Exit || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	if (clusterOwner && d.openRGBOwned) || (!clusterOwner && d.clusterOwned) {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 2
	}
	p := cloneLightingProfile(*d.DeviceProfile)
	if clusterOwner {
		p.RGBCluster = enabled
	} else {
		p.OpenRGBIntegration = enabled
	}
	err := d.persistLightingProfile(p)
	if err != nil {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	// Publish only the changed preference, retaining profile/input identity.
	d.DeviceProfile.RGBCluster, d.DeviceProfile.OpenRGBIntegration = p.RGBCluster, p.OpenRGBIntegration
	d.clusterOwned, d.openRGBOwned = p.RGBCluster, p.OpenRGBIntegration
	d.invalidateExternalLocked()
	d.cancelLightingLocked()
	d.lightingMu.Unlock()
	d.refreshLightingRegistrationsLocked(true, true)
	d.lightingMutation.Unlock()
	d.setDeviceColor()
	return 1
}
func (d *Device) enqueueOpenRGB(data []byte, generation uint64) {
	if len(data) != 9 {
		return
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.Exit || !d.openRGBOwned || generation != d.externalGeneration {
		return
	}
	frame := lightingExternalFrame{data: append([]byte(nil), data...), generation: generation}
	select {
	case d.externalQueue <- frame:
	default:
	}
}
func (d *Device) enqueueCurrentOpenRGB(data []byte) {
	d.lightingMu.Lock()
	g := d.externalGeneration
	d.lightingMu.Unlock()
	d.enqueueOpenRGB(data, g)
}
func (d *Device) applyCurrentCluster(data []byte) {
	d.lightingMu.Lock()
	g := d.externalGeneration
	d.lightingMu.Unlock()
	d.applyLightingPacket(composeLightingPacket(append([]byte(nil), data...)), "cluster", g, nil)
}
func (d *Device) startLightingQueue() {
	d.lightingMu.Lock()
	if d.externalStarted || d.Exit {
		d.lightingMu.Unlock()
		return
	}
	d.externalStarted = true
	queue, stop := d.externalQueue, d.externalStop
	d.lightingMu.Unlock()
	go func() {
		for {
			select {
			case <-stop:
				return
			case frame := <-queue:
				d.applyLightingPacket(composeLightingPacket(frame.data), "openrgb", frame.generation, nil)
				if !lightingDelay(stop, 20*time.Millisecond) {
					return
				}
			}
		}
	}()
}

func (d *Device) changeCanonicalDeviceProfile(name string) uint8 {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	next, ok := d.UserProfiles[name]
	if !ok || next == nil || d.DeviceProfile == nil || d.Exit || (next.RGBCluster && next.OpenRGBIntegration) {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	old := cloneLightingProfile(*d.DeviceProfile)
	proposal := cloneLightingProfile(*next)
	old.Active, proposal.Active = false, true
	// Keep source two-file switch, but errors are visible and runtime is unchanged
	// until both writes succeed. Restore first file if the second write fails.
	if err := d.persistLightingProfile(old); err != nil {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	if err := d.persistLightingProfile(proposal); err != nil {
		old.Active = true
		if restoreErr := d.persistLightingProfile(old); restoreErr != nil {
			logger.Log(logger.Fields{"error": restoreErr}).Error("Unable to restore active profile marker")
		}
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	for key, p := range d.UserProfiles {
		if p == d.DeviceProfile {
			q := cloneLightingProfile(*p)
			q.Active = false
			d.UserProfiles[key] = &q
		}
	}
	d.UserProfiles[name] = &proposal
	d.DeviceProfile = &proposal
	d.userRGBOff, d.clusterOwned, d.openRGBOwned = proposal.RgbOff, proposal.RGBCluster, proposal.OpenRGBIntegration
	// Scheduler remains a runtime override across profile switches.
	d.invalidateExternalLocked()
	d.cancelLightingLocked()
	ready := d.Connected && d.dev != nil
	d.lightingMu.Unlock()
	d.refreshLightingRegistrationsLocked(true, true)
	d.lightingMutation.Unlock()
	d.setDeviceColor()
	if ready {
		d.toggleDPI()
		d.loadKeyAssignments()
		d.setupKeyAssignment()
	}
	return 1
}

// Legacy combined color endpoints retain input metadata; this device does not
// program DPI/Sniper RGB bytes. Validate input before saving the single canonical
// palette. A canonical save failure cannot mutate input pointers or emit output.
func (d *Device) saveCanonicalMouseColors(dpi *rgb.Color, zones map[int]rgb.Color, sniper *rgb.Color) uint8 {
	inputChanged := dpi != nil || sniper != nil
	err := d.mutateLighting(func(source *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if len(zones) == 0 || d.DeviceProfile == nil {
			return false, fmt.Errorf("missing zone/input state")
		}
		palette, err := source.palette(d.Serial)
		if err != nil {
			return false, err
		}
		for id, c := range zones {
			key := strconv.Itoa(id)
			if _, ok := palette[key]; !ok {
				return false, fmt.Errorf("invalid zone")
			}
			palette[key] = lightingColor(c)
		}
		if err = validateLightingPalette(palette); err != nil {
			return false, err
		}
		if !inputChanged {
			return state.SelectedEffect == "mouse", source.effects.SetAuthoredZones(d.Serial, "mouse", palette)
		}
		if dpi == nil || sniper == nil || d.DeviceProfile.DPIColor == nil || d.DeviceProfile.SniperColor == nil {
			return false, fmt.Errorf("input colors unavailable")
		}
		if err = validateLightingColor(lightingColor(*dpi)); err != nil {
			return false, err
		}
		if err = validateLightingColor(lightingColor(*sniper)); err != nil {
			return false, err
		}
		proposal := cloneLightingProfile(*d.DeviceProfile)
		proposal.DPIColor = glaiveRGBColor(*dpi)
		proposal.SniperColor = glaiveRGBColor(*sniper)
		if err = source.effects.SetAuthoredZonesWithCommit(d.Serial, "mouse", palette, func() error { return d.persistLightingProfile(proposal) }); err != nil {
			return false, err
		}
		d.DeviceProfile.DPIColor, d.DeviceProfile.SniperColor = proposal.DPIColor, proposal.SniperColor
		return state.SelectedEffect == "mouse", nil
	})
	if err != nil {
		return 0
	}
	if inputChanged {
		d.lightingMu.Lock()
		ready := !d.Exit && d.Connected && d.dev != nil
		d.lightingMu.Unlock()
		if ready {
			d.updateMouseDPI()
			d.toggleDPI()
		}
	}
	return 1
}

// Retire queued frames at the session boundary. A dequeued/in-flight frame is
// additionally rejected by its captured generation at the final write gate.
func (d *Device) invalidateExternalLocked() {
	d.externalGeneration++
	for {
		select {
		case <-d.externalQueue:
		default:
			return
		}
	}
}
