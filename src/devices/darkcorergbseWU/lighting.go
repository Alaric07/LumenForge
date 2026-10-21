package darkcorergbseWU

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
)

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

// These are hardware selectors/types, not byte offsets or input indicator LEDs.
// Return values keep topology and defaults out of mutable profile authority.
func lightingTopology() [3]ZoneColors {
	return [3]ZoneColors{
		{Name: "Logo", ColorIndex: 1, ColorType: 3},
		{Name: "Scroll", ColorIndex: 4, ColorType: 5},
		{Name: "Side", ColorIndex: 2, ColorType: 4},
	}
}
func defaultLightingPalette() map[string]lightingsettings.Color {
	return map[string]lightingsettings.Color{"0": {Red: 255, Green: 255}, "1": {Green: 255, Blue: 255}, "2": {Green: 255, Blue: 255}}
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
		return fmt.Errorf("complete DARK CORE SE palette required")
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
func (d *Device) SupportsLightingEffect(effect string) bool {
	return effect == "mouse" || effect == "static"
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
	// Attachment is data-only. WU opens its own HID in Init; readiness gates
	// applying output, never resolving or persisting desired Lighting.
	if err := d.validateLightingSource(candidate); err != nil {
		return err
	}
	if d.DeviceProfile == nil {
		return fmt.Errorf("RGB-off preference backing is unavailable")
	}
	// Only this user preference retains profile persistence. Authored Lighting
	// still resolves exclusively from the validated canonical candidate.
	d.userRGBOff = d.DeviceProfile.RgbOff
	d.lightingCanonical = candidate
	return nil
}
func (d *Device) validateLightingSource(s *canonicalLighting) error {
	if d.Serial == "" || d.LEDChannels != 3 || d.ZoneAmount != 3 || s == nil {
		return fmt.Errorf("DARK CORE SE Lighting unavailable")
	}
	if len(rgbModes) != 2 || rgbModes[0] != "mouse" || rgbModes[1] != "static" {
		return fmt.Errorf("invalid Lighting catalogue")
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
	if _, err = s.defaults.Get("static"); err != nil {
		return err
	}
	resolved, err := s.resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), "static")
	if err != nil {
		return err
	}
	if resolved.Settings.EffectID != "static" {
		return fmt.Errorf("invalid Static identity")
	}
	if err = lightingsettings.Validate(resolved.Settings); err != nil {
		return err
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
	if effect != "static" {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported settings %q", effect)
	}
	if err := d.validateLightingSource(d.lightingCanonical); err != nil {
		return lightingsettings.EffectSettings{}, err
	}
	value, err := d.lightingCanonical.resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), effect)
	return value.Settings.Clone(), err
}
func (d *Device) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	if d == nil {
		return lightingpresentation.Snapshot{}, false
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	source := d.lightingCanonical
	if d.validateLightingSource(source) != nil {
		return lightingpresentation.Snapshot{}, false
	}
	state, _, err := source.state.Resolve(d.Serial)
	if err != nil {
		return lightingpresentation.Snapshot{}, false
	}
	snapshot := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: state.SelectedEffect, EffectSupported: true, EffectSelectionAvailable: true, HasBrightness: true, Brightness: state.Brightness, SupportedEffects: []lightingpresentation.EffectOption{{ID: "mouse", Label: "Mouse"}, {ID: "static", Label: "Static"}}}
	if state.SelectedEffect == "mouse" {
		palette, err := source.palette(d.Serial)
		if err != nil {
			return lightingpresentation.Snapshot{}, false
		}
		snapshot.AuthoredZoneEditor = &lightingpresentation.AuthoredZoneEditor{EffectID: "mouse", Heading: "Zones", Description: "Choose colors for the selected zones."}
		for id, zone := range lightingTopology() {
			snapshot.AuthoredZoneEditor.Zones = append(snapshot.AuthoredZoneEditor.Zones, lightingpresentation.AuthoredZone{ID: strconv.Itoa(id), Label: zone.Name, ColorHex: lightingHex(palette[strconv.Itoa(id)])})
		}
	} else {
		resolved, err := source.resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), "static")
		if err != nil {
			return lightingpresentation.Snapshot{}, false
		}
		snapshot.PaletteKind = string(rgb.LightingPaletteStaticSingle)
		snapshot.Customized = resolved.Customized
		snapshot.SingleColorHex = lightingHex(resolved.Settings.SingleColor.Color)
	}
	return snapshot, true
}

// Serialize proposals through persistence, then release mutation locks before
// hardware I/O. A stalled HID write must not prevent recording newer intent.
func (d *Device) mutateLighting(change func(*canonicalLighting, lightingsettings.IndependentDeviceLightingState) (bool, error)) error {
	if d == nil {
		return fmt.Errorf("device unavailable")
	}
	apply, err := func() (bool, error) {
		d.lightingMutation.Lock()
		defer d.lightingMutation.Unlock()
		d.lightingMu.Lock()
		defer d.lightingMu.Unlock()
		if d.Exit {
			return false, fmt.Errorf("device stopped")
		}
		if err := d.validateLightingSource(d.lightingCanonical); err != nil {
			return false, err
		}
		state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
		if err != nil {
			return false, err
		}
		apply, err := change(d.lightingCanonical, state)
		// Failed persistence leaves active output and its cancellation token intact.
		if err == nil && apply {
			d.cancelLightingLocked()
		}
		return apply, err
	}()
	if err == nil && apply {
		d.setDeviceColor()
	}
	return err
}
func (d *Device) SetLightingEffect(effect string) error {
	return d.mutateLighting(func(s *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if !d.SupportsLightingEffect(effect) {
			return false, fmt.Errorf("unsupported effect %q", effect)
		}
		state.SelectedEffect = effect
		return true, s.state.Set(d.Serial, state)
	})
}
func (d *Device) SetLightingBrightness(value uint8) error {
	return d.mutateLighting(func(s *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if value > 100 {
			return false, fmt.Errorf("brightness must be 0..100")
		}
		state.Brightness = value
		return true, s.state.Set(d.Serial, state)
	})
}
func (d *Device) SetLightingEffectSettings(effect string, value lightingsettings.EffectSettings) error {
	value = value.Clone()
	return d.mutateLighting(func(s *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if effect != "static" || value.EffectID != effect {
			return false, fmt.Errorf("unsupported settings %q", effect)
		}
		if err := lightingsettings.Validate(value); err != nil {
			return false, err
		}
		return state.SelectedEffect == effect, s.effects.Set(d.Serial, effect, value)
	})
}
func (d *Device) ResetLightingEffectSettings(effect string) error {
	return d.mutateLighting(func(s *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if effect != "static" {
			return false, fmt.Errorf("unsupported reset %q", effect)
		}
		_, err := s.effects.Delete(d.Serial, effect)
		return state.SelectedEffect == effect, err
	})
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
func (d *Device) setLightingOverride(scheduler, dark bool) uint8 {
	if !scheduler {
		if err := d.setUserRGBOff(dark); err != nil {
			logger.Log(logger.Fields{"error": err}).Error("Unable to persist device RGB-off preference")
			return 0
		}
		return 1
	}
	if d == nil {
		return 0
	}
	changed, ok := func() (bool, bool) {
		d.lightingMutation.Lock()
		defer d.lightingMutation.Unlock()
		d.lightingMu.Lock()
		defer d.lightingMu.Unlock()
		if d.Exit || d.validateLightingSource(d.lightingCanonical) != nil {
			return false, false
		}
		current := &d.schedulerDark
		if *current == dark {
			return false, true
		}
		*current = dark
		d.cancelLightingLocked()
		return true, true
	}()
	if !ok {
		return 0
	}
	if changed {
		d.setDeviceColor()
	}
	return 1
}

// The preference uses the existing active-profile file/schema. Write a copied
// proposal atomically; publishing the bool must not reload profiles, upgrade
// input state, or synchronize any canonical authored Lighting into legacy fields.
func (d *Device) persistUserRGBOff(value bool) error {
	if d.DeviceProfile == nil {
		return fmt.Errorf("RGB-off preference backing is unavailable")
	}
	proposal := *copyDeviceProfile(d.DeviceProfile)
	name := filepath.Base(proposal.Path)
	if proposal.Path == "" {
		name = d.Serial + ".json"
	}
	proposal.Path = filepath.Join(pwd, "database", "profiles", name)
	proposal.RgbOff = value
	data, err := json.MarshalIndent(&proposal, "", "    ")
	if err != nil {
		return err
	}
	write := writeRGBOffPreference
	if d.lightingRGBOffWrite != nil {
		write = d.lightingRGBOffWrite
	}
	if err = write(proposal.Path, data); err != nil {
		return err
	}
	d.DeviceProfile.RgbOff = value
	return nil
}

func writeRGBOffPreference(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".rgb-off-*.tmp")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err = file.Chmod(0600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func (d *Device) setUserRGBOff(value bool) error {
	if d == nil {
		return fmt.Errorf("device unavailable")
	}
	changed, err := func() (bool, error) {
		d.lightingMutation.Lock()
		defer d.lightingMutation.Unlock()
		d.lightingMu.Lock()
		defer d.lightingMu.Unlock()
		if d.Exit {
			return false, fmt.Errorf("device stopped")
		}
		if err := d.validateLightingSource(d.lightingCanonical); err != nil {
			return false, err
		}
		if d.DeviceProfile == nil {
			return false, fmt.Errorf("RGB-off preference backing is unavailable")
		}
		if d.userRGBOff == value && d.DeviceProfile.RgbOff == value {
			return false, nil
		}
		if err := d.persistUserRGBOff(value); err != nil {
			return false, err
		}
		d.userRGBOff = value
		d.cancelLightingLocked()
		return true, nil
	}()
	if err == nil && changed {
		d.setDeviceColor()
	}
	return err
}

// A synchronous three-zone apply owns a close-only token. Cancellation never
// waits for a goroutine/HID write. No animation worker or retry loop is needed.
type lightingApply struct{ stop chan struct{} }

func (d *Device) cancelLightingLocked() {
	if d.lightingApply != nil {
		close(d.lightingApply.stop)
		d.lightingApply = nil
	}
	d.activeRgb = nil
}
func (d *Device) stopLighting() {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	d.cancelLightingLocked()
}
func (d *Device) retireLightingApply(owner *lightingApply) {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.lightingApply == owner {
		d.lightingApply = nil
	}
}
func (d *Device) lightingOutputReadyLocked() bool {
	return !d.Exit && d.Connected && (d.dev != nil || d.lightingWrite != nil)
}
func (d *Device) lightingApplyCurrent(owner *lightingApply) bool {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.lightingApply == owner && d.lightingOutputReadyLocked()
}

// WU has no receiver Connect/sleep/wake callbacks. This readiness seam is also
// used by inert tests; production reconnect creates a fresh Device through Init.
func (d *Device) setLightingAvailable(available bool) bool {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.Exit || d.Connected == available {
		return false
	}
	d.cancelLightingLocked()
	d.Connected = available
	return true
}
func (d *Device) exitLighting() {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	d.Exit = true
	d.Connected = false
	d.cancelLightingLocked()
}
func (d *Device) lightingExited() bool {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.Exit
}

type lightingZoneOutput struct {
	selector, kind int
	rgb            [3]byte
}

func (d *Device) composeLightingLocked() ([3]lightingZoneOutput, error) {
	var result [3]lightingZoneOutput
	if err := d.validateLightingSource(d.lightingCanonical); err != nil {
		return result, err
	}
	s := d.lightingCanonical
	state, _, err := s.state.Resolve(d.Serial)
	if err != nil {
		return result, err
	}
	palette, err := s.palette(d.Serial)
	if err != nil {
		return result, err
	}
	if state.SelectedEffect == "static" {
		resolved, err := s.resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), "static")
		if err != nil {
			return result, err
		}
		for id := range palette {
			palette[id] = resolved.Settings.SingleColor.Color
		}
	}
	brightness := state.Brightness
	if d.schedulerDark || d.userRGBOff {
		brightness = 0
	}
	for id, zone := range lightingTopology() {
		c := palette[strconv.Itoa(id)]
		factor := float64(brightness) / 100
		result[id] = lightingZoneOutput{selector: zone.ColorIndex, kind: zone.ColorType, rgb: [3]byte{byte(c.Red * factor), byte(c.Green * factor), byte(c.Blue * factor)}}
	}
	return result, nil
}
func (d *Device) setDeviceColor() {
	d.lightingMu.Lock()
	if d.lightingCanonical == nil {
		exited := d.Exit
		d.lightingMu.Unlock()
		if !exited {
			d.setLegacyDeviceColor()
		}
		return
	}
	if !d.lightingOutputReadyLocked() {
		d.lightingMu.Unlock()
		return
	}
	frame, err := d.composeLightingLocked()
	if err != nil {
		d.lightingMu.Unlock()
		return
	}
	d.cancelLightingLocked()
	owner := &lightingApply{stop: make(chan struct{})}
	d.lightingApply = owner
	d.lightingMu.Unlock()
	defer d.retireLightingApply(owner)
	for _, zone := range frame {
		select {
		case <-owner.stop:
			return
		default:
		}
		d.writeColor(zone.rgb[:], zone.selector, zone.kind, owner)
	}
}

// Legacy-shaped getters project canonical settings and never consult old RGB.
func (d *Device) canonicalRGBProfile(effect string) (*rgb.Profile, bool) {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.lightingCanonical == nil {
		return nil, false
	}
	if d.validateLightingSource(d.lightingCanonical) != nil || !d.SupportsLightingEffect(effect) {
		return nil, true
	}
	if effect == "mouse" {
		return &rgb.Profile{ProfileName: "Mouse"}, true
	}
	value, err := d.lightingCanonical.resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), effect)
	if err != nil {
		return nil, true
	}
	profile := lightingsettings.RendererProfileFromEffectSettings(value.Settings.Clone())
	return &profile, true
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

// The compatibility form spans two authorities. Canonical palette persistence
// must succeed before touching input state. Input colors retain their existing
// package-local save/programming path; they never become Lighting settings.
func (d *Device) saveCanonicalMouseColors(dpi rgb.Color, zones map[int]rgb.Color, sniper *rgb.Color) uint8 {
	updates := make(map[string]lightingsettings.Color, len(zones))
	for id, c := range zones {
		updates[strconv.Itoa(id)] = lightingColor(c)
	}
	err := d.mutateLighting(func(s *canonicalLighting, state lightingsettings.IndependentDeviceLightingState) (bool, error) {
		if !d.lightingOutputReadyLocked() || d.DeviceProfile == nil || d.DeviceProfile.DPIColor == nil || len(updates) == 0 {
			return false, fmt.Errorf("mouse input unavailable")
		}
		if err := validateLightingColor(lightingColor(dpi)); err != nil {
			return false, err
		}
		if sniper != nil {
			if d.DeviceProfile.SniperColor == nil {
				return false, fmt.Errorf("Sniper input unavailable")
			}
			if err := validateLightingColor(lightingColor(*sniper)); err != nil {
				return false, err
			}
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
		// No input pointer, file or output is changed on a canonical save failure.
		if err = s.effects.SetAuthoredZones(d.Serial, "mouse", palette); err != nil {
			return false, err
		}
		d.cancelLightingLocked()
		input := *copyDeviceProfile(d.DeviceProfile)
		dpiColor := *input.DPIColor
		dpiColor.Red, dpiColor.Green, dpiColor.Blue = dpi.Red, dpi.Green, dpi.Blue
		dpiColor.Hex = lightingHex(lightingColor(dpi))
		input.DPIColor = &dpiColor
		if sniper != nil {
			c := *input.SniperColor
			c.Red, c.Green, c.Blue = sniper.Red, sniper.Green, sniper.Blue
			c.Hex = lightingHex(lightingColor(*sniper))
			input.SniperColor = &c
		}
		d.DeviceProfile = &input
		// Preserve the existing input persistence semantics and profile identity.
		// Lighting fields are retained serialized values, never synchronized here.
		d.saveDeviceProfile()
		return false, nil // program input first, then apply Lighting in source order
	})
	if err != nil {
		return 0
	}
	d.updateMouseDPI()
	if sniper == nil {
		d.toggleDPI(false)
	}
	d.setDeviceColor()
	return 1
}

// Profile/input data stays outside canonical Lighting. Clone nested legacy data
// before compatibility saves/upgrades can change maps or pointed colors.
func copyDeviceProfile(p *DeviceProfile) *DeviceProfile {
	if p == nil {
		return nil
	}
	c := *p
	if p.BrightnessSlider != nil {
		b := *p.BrightnessSlider
		c.BrightnessSlider = &b
	}
	if p.DPIColor != nil {
		v := *p.DPIColor
		c.DPIColor = &v
	}
	if p.SniperColor != nil {
		v := *p.SniperColor
		c.SniperColor = &v
	}
	c.Profiles = make(map[int]DPIProfile, len(p.Profiles))
	for id, v := range p.Profiles {
		c.Profiles[id] = v
	}
	c.ZoneColors = make(map[int]ZoneColors, len(p.ZoneColors))
	for id, v := range p.ZoneColors {
		if v.Color != nil {
			color := *v.Color
			v.Color = &color
		}
		c.ZoneColors[id] = v
	}
	return &c
}

// Keep the source profile/input switch and its stored RGB-off preference. None
// of the newly selected profile's authored Lighting fields enter canonical state.
func (d *Device) changeCanonicalDeviceProfile(name string) uint8 {
	ready := false
	ok := func() bool {
		d.lightingMutation.Lock()
		defer d.lightingMutation.Unlock()
		d.lightingMu.Lock()
		defer d.lightingMu.Unlock()
		profile, found := d.UserProfiles[name]
		if d.Exit || !found || profile == nil || d.DeviceProfile == nil {
			return false
		}
		current := copyDeviceProfile(d.DeviceProfile)
		current.Active = false
		d.DeviceProfile = current
		d.saveDeviceProfile()
		next := copyDeviceProfile(profile)
		next.Active = true
		d.DeviceProfile = next
		d.saveDeviceProfile()
		d.userRGBOff = d.DeviceProfile.RgbOff
		ready = d.lightingOutputReadyLocked()
		d.cancelLightingLocked()
		return true
	}()
	if !ok {
		return 0
	}
	d.setDeviceColor()
	if ready {
		d.toggleDPI(false)
		d.loadKeyAssignments()
		d.setupKeyAssignment()
	}
	return 1
}
