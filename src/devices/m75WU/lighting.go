package m75WU

// This transport keeps the same source-backed lighting contract as m75W. The
// implementation is intentionally package-local so direct-HID lifecycle and
// frame output remain independent.

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"sync"

	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

type m75LightingSource interface {
	resolve() (m75ResolvedLighting, error)
	resolveSettings(string) (lightingsettings.Resolution, error)
	selected() (string, error)
	brightness() (uint8, error)
	setSelected(string) error
	setBrightness(uint8) error
	setSettings(string, lightingsettings.EffectSettings) error
	deleteSettings(string) (bool, error)
}
type m75ResolvedLighting struct {
	effect     string
	brightness uint8
	settings   lightingsettings.EffectSettings
}
type m75State interface {
	Resolve(string) (lightingsettings.IndependentDeviceLightingState, bool, error)
	Set(string, lightingsettings.IndependentDeviceLightingState) error
}
type m75Resolver interface {
	Resolve(lightingsettings.Target, string) (lightingsettings.Resolution, error)
}
type m75Effects interface {
	Set(string, string, lightingsettings.EffectSettings) error
}
type m75EffectsDelete interface {
	Delete(string, string) (bool, error)
}
type m75IndependentSource struct {
	id       string
	state    m75State
	resolver m75Resolver
	effects  m75Effects
}

func (s m75IndependentSource) selected() (string, error) {
	if s.state == nil {
		return "", fmt.Errorf("M75 canonical lighting runtime is unavailable")
	}
	v, _, err := s.state.Resolve(s.id)
	return v.SelectedEffect, err
}
func (s m75IndependentSource) brightness() (uint8, error) {
	if s.state == nil {
		return 0, fmt.Errorf("M75 canonical lighting runtime is unavailable")
	}
	v, _, err := s.state.Resolve(s.id)
	return v.Brightness, err
}
func (s m75IndependentSource) resolveSettings(effect string) (lightingsettings.Resolution, error) {
	if s.resolver == nil {
		return lightingsettings.Resolution{}, fmt.Errorf("M75 canonical lighting runtime is unavailable")
	}
	v, err := s.resolver.Resolve(lightingsettings.IndependentDevice(s.id), effect)
	v.Settings = v.Settings.Clone()
	return v, err
}
func (s m75IndependentSource) resolve() (m75ResolvedLighting, error) {
	effect, err := s.selected()
	if err != nil {
		return m75ResolvedLighting{}, err
	}
	brightness, err := s.brightness()
	if err != nil {
		return m75ResolvedLighting{}, err
	}
	if effect == "mouse" {
		return m75ResolvedLighting{effect: effect, brightness: brightness}, nil
	}
	resolved, err := s.resolveSettings(effect)
	if err != nil {
		return m75ResolvedLighting{}, err
	}
	return m75ResolvedLighting{effect, brightness, resolved.Settings}, nil
}
func (s m75IndependentSource) setSelected(effect string) error {
	if !m75SupportsEffect(effect) {
		return fmt.Errorf("unsupported M75 effect %q", effect)
	}
	if effect != "mouse" {
		if _, err := s.resolveSettings(effect); err != nil {
			return err
		}
	}
	v, _, err := s.state.Resolve(s.id)
	if err != nil {
		return err
	}
	v.SelectedEffect = effect
	return s.state.Set(s.id, v)
}
func (s m75IndependentSource) setBrightness(value uint8) error {
	v, _, err := s.state.Resolve(s.id)
	if err != nil {
		return err
	}
	v.Brightness = value
	return s.state.Set(s.id, v)
}
func (s m75IndependentSource) setSettings(effect string, value lightingsettings.EffectSettings) error {
	if s.effects == nil {
		return fmt.Errorf("M75 canonical effect customization store is unavailable")
	}
	return s.effects.Set(s.id, effect, value)
}
func (s m75IndependentSource) deleteSettings(effect string) (bool, error) {
	store, ok := s.effects.(m75EffectsDelete)
	if !ok {
		return false, fmt.Errorf("M75 canonical effect customization deletion is unavailable")
	}
	return store.Delete(s.id, effect)
}

type m75SchedulerBrightnessOverride struct {
	mu    sync.RWMutex
	value *uint8
}

type m75UserRGBOff struct {
	mu    sync.RWMutex
	value bool
}

func (state *m75UserRGBOff) set(value bool) bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.value == value {
		return false
	}
	state.value = value
	return true
}

func (state *m75UserRGBOff) enabled() bool {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.value
}

func (o *m75SchedulerBrightnessOverride) set(value *uint8) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.value == nil && value == nil {
		return false
	}
	if o.value != nil && value != nil && *o.value == *value {
		return false
	}
	if value == nil {
		o.value = nil
		return true
	}
	copy := *value
	o.value = &copy
	return true
}
func (o *m75SchedulerBrightnessOverride) effective(value uint8) uint8 {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.value == nil {
		return value
	}
	return *o.value
}
func (d *Device) attachIndependentDeviceLightingRuntime(paths config.Paths) error {
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile, filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		return err
	}
	if runtime == nil || runtime.State == nil || runtime.Effects == nil || runtime.Resolver == nil {
		return fmt.Errorf("M75 canonical lighting runtime is unavailable")
	}
	source := m75IndependentSource{id: d.Serial, state: runtime.State, effects: runtime.Effects, resolver: runtime.Resolver}
	if _, err = source.resolve(); err != nil {
		return err
	}
	d.lightingSource = source
	return nil
}
func (d *Device) currentCanonicalSelectedEffect() (string, error) {
	if d == nil || d.lightingSource == nil {
		return "", fmt.Errorf("M75 canonical lighting source is unavailable")
	}
	return d.lightingSource.selected()
}
func (d *Device) currentCanonicalBrightness() (uint8, error) {
	if d == nil || d.lightingSource == nil {
		return 0, fmt.Errorf("M75 canonical lighting source is unavailable")
	}
	return d.lightingSource.brightness()
}
func (d *Device) resolveEffectiveCanonicalLighting() (m75ResolvedLighting, error) {
	if d == nil || d.lightingSource == nil {
		return m75ResolvedLighting{}, fmt.Errorf("M75 canonical lighting source is unavailable")
	}
	value, err := d.lightingSource.resolve()
	value.brightness = d.schedulerBrightnessOverride.effective(value.brightness)
	if d.userRGBOff.enabled() {
		value.brightness = 0
	}
	return value, err
}

func (d *Device) clearLegacyRgbOff() bool {
	if d == nil || d.lightingSource == nil || d.DeviceProfile == nil || !d.DeviceProfile.RgbOff {
		return false
	}
	d.DeviceProfile.RgbOff = false
	d.saveDeviceProfile()
	return true
}
func (d *Device) restartCanonicalLighting() {
	if d.lightingRestart != nil {
		d.lightingRestart()
		return
	}
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.setDeviceColor(false)
}
func m75Descriptor(effect string) (rgb.SoftwareEffectDescriptor, bool) {
	value, ok := rgb.SoftwareEffectDescriptorByID(effect)
	return value, ok && value.Scope.Includes(rgb.EffectScopeDevice) && slices.Contains(rgbModes, effect)
}
func m75SupportsEffect(effect string) bool {
	if effect == "mouse" {
		return true
	}
	_, ok := m75Descriptor(effect)
	return ok
}
func (d *Device) LightingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) SupportsLightingEffect(effect string) bool { return m75SupportsEffect(effect) }
func (d *Device) SetLightingEffect(effect string) error {
	if d == nil || d.DeviceProfile == nil || d.lightingSource == nil || !m75SupportsEffect(effect) {
		return fmt.Errorf("unsupported M75 effect %q", effect)
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	if err := d.lightingSource.setSelected(effect); err != nil {
		return err
	}
	d.restartCanonicalLighting()
	return nil
}
func (d *Device) SetLightingBrightness(value uint8) error {
	if d == nil || d.DeviceProfile == nil || d.lightingSource == nil {
		return fmt.Errorf("M75 lighting ownership is unavailable")
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	if err := d.lightingSource.setBrightness(value); err != nil {
		return err
	}
	d.restartCanonicalLighting()
	return nil
}
func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if d == nil || effect == "mouse" || !m75SupportsEffect(effect) || d.lightingSource == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported M75 effect %q", effect)
	}
	value, err := d.lightingSource.resolveSettings(effect)
	return value.Settings.Clone(), err
}
func (d *Device) SetLightingEffectSettings(effect string, value lightingsettings.EffectSettings) error {
	if d == nil || d.DeviceProfile == nil || effect == "mouse" || !m75SupportsEffect(effect) || value.EffectID != effect {
		return fmt.Errorf("invalid M75 effect settings")
	}
	if err := lightingsettings.Validate(value); err != nil {
		return err
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	selected, err := d.currentCanonicalSelectedEffect()
	if err != nil {
		return err
	}
	if err = d.lightingSource.setSettings(effect, value.Clone()); err != nil {
		return err
	}
	if selected == effect {
		d.restartCanonicalLighting()
	}
	return nil
}
func (d *Device) ResetLightingEffectSettings(effect string) error {
	if d == nil || d.DeviceProfile == nil || d.lightingSource == nil || effect == "mouse" || !m75SupportsEffect(effect) {
		return fmt.Errorf("unsupported M75 effect %q", effect)
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	selected, err := d.currentCanonicalSelectedEffect()
	if err != nil {
		return err
	}
	deleted, err := d.lightingSource.deleteSettings(effect)
	if err != nil {
		return err
	}
	if deleted && selected == effect {
		d.restartCanonicalLighting()
	}
	return nil
}
func m75ColorHex(c lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(c.Red), uint8(c.Green), uint8(c.Blue))
}
func m75AuthoredColorHex(c rgb.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(c.Red), uint8(c.Green), uint8(c.Blue))
}
func (d *Device) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	effect, err := d.currentCanonicalSelectedEffect()
	if err != nil {
		return lightingpresentation.Snapshot{}, false
	}
	brightness, err := d.currentCanonicalBrightness()
	if err != nil {
		return lightingpresentation.Snapshot{}, false
	}
	descriptor, supported := m75Descriptor(effect)
	snapshot := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: effect, HasBrightness: true, Brightness: brightness, EffectSupported: supported}
	for _, candidate := range rgbModes {
		if candidate == "mouse" {
			snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{ID: candidate, Label: "Mouse"})
			continue
		}
		if value, ok := m75Descriptor(candidate); ok {
			snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{ID: candidate, Label: value.Label})
		}
	}
	if effect == "mouse" {
		snapshot.EffectSupported = true
		snapshot.AuthoredZoneEditor = m75AuthoredZoneEditor(d.DeviceProfile)
		return snapshot, true
	}
	if !supported {
		return snapshot, true
	}
	resolved, err := d.lightingSource.resolveSettings(effect)
	if err != nil || resolved.Settings.EffectID != effect {
		return lightingpresentation.Snapshot{}, false
	}
	settings := resolved.Settings
	snapshot.Customized, snapshot.PaletteKind = resolved.Customized, string(descriptor.PaletteKind)
	if descriptor.SupportsSpeed && settings.Speed != nil {
		snapshot.HasSpeed, snapshot.Speed = true, *settings.Speed
	}
	if settings.SingleColor != nil {
		snapshot.SingleColorHex = m75ColorHex(settings.SingleColor.Color)
	}
	if settings.TwoColor != nil {
		snapshot.TwoColorStartHex, snapshot.TwoColorEndHex = m75ColorHex(settings.TwoColor.Start), m75ColorHex(settings.TwoColor.End)
	}
	if settings.Temperature != nil {
		snapshot.HasTemperature = true
		snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: m75ColorHex(settings.Temperature.Low.Color), Celsius: settings.Temperature.Low.Celsius}
		snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: m75ColorHex(settings.Temperature.Middle.Color), Celsius: settings.Temperature.Middle.Celsius}
		snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: m75ColorHex(settings.Temperature.High.Color), Celsius: settings.Temperature.High.Celsius}
	}
	if settings.Gradient != nil {
		snapshot.HasGradient = true
		for _, stop := range settings.Gradient.Stops {
			snapshot.GradientStops = append(snapshot.GradientStops, lightingpresentation.GradientStop{Position: stop.Position, ColorHex: m75ColorHex(stop.Color), Intensity: stop.Intensity})
		}
	}
	return snapshot, true
}

func m75AuthoredZoneEditor(profile *DeviceProfile) *lightingpresentation.AuthoredZoneEditor {
	if profile == nil || len(profile.ZoneColors) == 0 {
		return nil
	}
	keys := make([]int, 0, len(profile.ZoneColors))
	for key := range profile.ZoneColors {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	editor := &lightingpresentation.AuthoredZoneEditor{
		EffectID: "mouse", Heading: "Zones",
		Description: "Select one or more zones, choose a color, then apply it to the selected zones.",
		Zones:       make([]lightingpresentation.AuthoredZone, 0, len(keys)),
	}
	for _, key := range keys {
		zone := profile.ZoneColors[key]
		if zone.Color == nil {
			continue
		}
		editor.Zones = append(editor.Zones, lightingpresentation.AuthoredZone{ID: strconv.Itoa(key), Label: zone.Name, ColorHex: m75AuthoredColorHex(*zone.Color)})
	}
	return editor
}

// SetLightingZoneColor adapts M75's device-owned Mouse zones without putting
// their colors into generic EffectSettings.
func (d *Device) SetLightingZoneColor(effect, scope, zoneID, groupID string, color rgb.Color) error {
	if d == nil || d.DeviceProfile == nil || effect != "mouse" || !d.SupportsLightingEffect(effect) || groupID != "" {
		return fmt.Errorf("M75 authored lighting is unavailable")
	}
	if color.Red < 0 || color.Red > 255 || color.Green < 0 || color.Green > 255 || color.Blue < 0 || color.Blue > 255 {
		return fmt.Errorf("invalid authored zone color")
	}
	var zoneIDs []string
	switch scope {
	case "zone":
		zoneIDs = []string{zoneID}
	case "all":
		if zoneID != "" || len(d.DeviceProfile.ZoneColors) == 0 {
			return fmt.Errorf("invalid authored zone selection")
		}
		for key := range d.DeviceProfile.ZoneColors {
			zoneIDs = append(zoneIDs, strconv.Itoa(key))
		}
	default:
		return fmt.Errorf("unsupported authored zone scope")
	}
	return d.SetLightingZoneColors(effect, zoneIDs, color)
}

func (d *Device) SetLightingZoneColors(effect string, zoneIDs []string, color rgb.Color) error {
	if d == nil || d.DeviceProfile == nil || effect != "mouse" || !d.SupportsLightingEffect(effect) || len(zoneIDs) == 0 {
		return fmt.Errorf("M75 authored lighting is unavailable")
	}
	if color.Red < 0 || color.Red > 255 || color.Green < 0 || color.Green > 255 || color.Blue < 0 || color.Blue > 255 {
		return fmt.Errorf("invalid authored zone color")
	}
	updates := make(map[int]rgb.Color, len(zoneIDs))
	for _, id := range zoneIDs {
		key, err := strconv.Atoi(id)
		if err != nil {
			return fmt.Errorf("invalid authored zone selection")
		}
		if _, exists := updates[key]; exists {
			return fmt.Errorf("duplicate authored zone")
		}
		if _, exists := d.DeviceProfile.ZoneColors[key]; !exists {
			return fmt.Errorf("unknown authored zone")
		}
		updates[key] = color
	}
	if d.SaveMouseZoneColors(rgb.Color{}, updates) != 1 {
		return fmt.Errorf("persist M75 authored zone colors")
	}
	return nil
}
func (d *Device) syncCanonicalLightingAdapter() bool {
	value, err := d.resolveEffectiveCanonicalLighting()
	if err != nil || d.DeviceProfile == nil || d.Rgb == nil {
		return false
	}
	if value.effect != "mouse" {
		profile := lightingsettings.RendererProfileFromEffectSettings(value.settings)
		d.Rgb.Profiles[value.effect] = profile
	}
	d.DeviceProfile.RGBProfile = value.effect
	brightness := value.brightness
	d.DeviceProfile.BrightnessSlider = &brightness
	return true
}
