package k65rgbRF

import (
	"LumenForge/src/keyboards"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

// K65 RGB RAPIDFIRE deliberately adapts its existing active device profile and RGB file.
// There is no second desired-state or effect-settings database. The shared
// defaults repository supplies immutable reset values only.
func (d *Device) attachLightingRuntime(paths config.Paths) error {
	defaults, err := lightingsettings.LoadDefaultRepository(filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		return err
	}
	if d.DeviceProfile == nil || d.DeviceProfile.BrightnessSlider > 100 || d.Rgb == nil {
		return fmt.Errorf("K65 RGB RAPIDFIRE lighting backing state is unavailable")
	}
	layouts := make(map[string]*keyboards.Keyboard)
	for _, suffix := range []string{"", "-de"} {
		data, err := os.ReadFile(filepath.Join(paths.ShippedDatabaseRoot, "keyboard", "k65rgbRF"+suffix+".json"))
		if err != nil {
			return err
		}
		var layout keyboards.Keyboard
		if err = json.Unmarshal(data, &layout); err != nil {
			return err
		}
		layouts[layout.Layout] = &layout
	}
	probe := &Device{DeviceProfile: d.DeviceProfile, lightingLayouts: layouts}
	if err := probe.validateLightingKeyboard(); err != nil {
		return err
	}
	for _, effect := range rgbModes {
		if effect == "keyboard" {
			continue
		}
		profile := d.GetRgbProfile(effect)
		if profile == nil {
			return fmt.Errorf("K65 RGB RAPIDFIRE lighting profile %q is unavailable", effect)
		}
		if _, err := lightingsettings.EffectSettingsFromRGBProfile(effect, *profile); err != nil {
			return err
		}
	}
	copy, err := cloneLightingProfile(d.DeviceProfile)
	if err != nil {
		return err
	}
	*d.DeviceProfile = *copy
	d.lightingLayouts = layouts
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
	if d == nil || d.Serial == "" || d.LEDChannels != 168 || d.dev == nil || d.Exit || d.lightingDefaults == nil || d.DeviceProfile == nil || d.Rgb == nil || d.DeviceProfile.BrightnessSlider > 100 {
		return fmt.Errorf("K65 RGB RAPIDFIRE canonical lighting is unavailable")
	}
	if err := d.validateLightingKeyboard(); err != nil {
		return err
	}
	if d.DeviceProfile.RGBProfile != "keyboard" && d.GetRgbProfile(d.DeviceProfile.RGBProfile) == nil {
		return fmt.Errorf("selected Lighting backing unavailable")
	}
	for _, effect := range rgbModes {
		if effect == "keyboard" {
			continue
		}
		profile := d.GetRgbProfile(effect)
		if profile == nil {
			return fmt.Errorf("K65 RGB RAPIDFIRE effect backing unavailable")
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
	if effect == "keyboard" || !d.SupportsLightingEffect(effect) {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported K65 RGB RAPIDFIRE effect %q", effect)
	}
	profile := d.GetRgbProfile(effect)
	if profile == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("K65 RGB RAPIDFIRE effect settings are unavailable")
	}
	return lightingsettings.EffectSettingsFromRGBProfile(effect, *profile)
}

func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if d == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("K65 RGB RAPIDFIRE lighting is unavailable")
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.resolveLightingSettings(effect)
}

func keyboardColorHex(c lightingsettings.Color) string {
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
		if candidate == "keyboard" {
			snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{ID: "keyboard", Label: "Keyboard"})
			continue
		}
		value, ok := rgb.SoftwareEffectDescriptorByID(candidate)
		if !ok {
			return lightingpresentation.Snapshot{}, false
		}
		snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{ID: candidate, Label: value.Label})
	}
	if effect == "keyboard" {
		snapshot.EffectSupported = true
		snapshot.AuthoredZoneEditor = d.keyboardZoneEditor()
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
		snapshot.SingleColorHex = keyboardColorHex(settings.SingleColor.Color)
	}
	if settings.TwoColor != nil {
		snapshot.TwoColorStartHex, snapshot.TwoColorEndHex = keyboardColorHex(settings.TwoColor.Start), keyboardColorHex(settings.TwoColor.End)
	}
	if settings.Temperature != nil {
		snapshot.HasTemperature = true
		snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: keyboardColorHex(settings.Temperature.Low.Color), Celsius: settings.Temperature.Low.Celsius}
		snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: keyboardColorHex(settings.Temperature.Middle.Color), Celsius: settings.Temperature.Middle.Celsius}
		snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: keyboardColorHex(settings.Temperature.High.Color), Celsius: settings.Temperature.High.Celsius}
	}
	if settings.Gradient != nil {
		snapshot.HasGradient = true
		for _, stop := range settings.Gradient.Stops {
			snapshot.GradientStops = append(snapshot.GradientStops, lightingpresentation.GradientStop{Position: stop.Position, ColorHex: keyboardColorHex(stop.Color), Intensity: stop.Intensity})
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
	if err := d.saveLightingJSON(profile.Path, &profile); err != nil {
		return err
	}
	*d.DeviceProfile = profile
	return nil
}

func (d *Device) restartLighting() {
	d.lightingMu.Lock()
	unavailable := d.DeviceProfile == nil || d.Exit
	d.lightingMu.Unlock()
	if unavailable {
		return
	}

	if d.lightingRestart != nil {
		d.lightingRestart()
		return
	}
	d.stopLightingRenderer()
	d.setDeviceColor()
}

func (d *Device) SetLightingEffect(effect string) error {
	if d == nil {
		return fmt.Errorf("K65 RGB RAPIDFIRE lighting is unavailable")
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
		return fmt.Errorf("unsupported K65 RGB RAPIDFIRE effect %q", effect)
	}
	if effect != "keyboard" {
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
		return fmt.Errorf("K65 RGB RAPIDFIRE lighting is unavailable")
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
		return fmt.Errorf("K65 RGB RAPIDFIRE brightness mutation is unavailable")
	}
	profile := *d.DeviceProfile
	profile.BrightnessSlider = value
	err := d.persistLightingProfile(profile)
	static := profile.RGBProfile == "static" || profile.RGBProfile == "keyboard"
	d.lightingMu.Unlock()
	// Dynamic rendering already samples brightness on every frame.
	if err == nil && static {
		d.restartLighting()
	}
	return err
}

func (d *Device) SetLightingEffectSettings(effect string, value lightingsettings.EffectSettings) error {
	if d == nil {
		return fmt.Errorf("K65 RGB RAPIDFIRE lighting is unavailable")
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
		return fmt.Errorf("K65 RGB RAPIDFIRE effect settings mismatch")
	}
	if err := lightingsettings.Validate(value); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	profile := *d.GetRgbProfile(effect)
	rendered := lightingsettings.RendererProfileFromEffectSettings(value)
	// Only editable fields change. Preserve MinTemp/MaxTemp, Smoothness,
	// brightness modes, version and all other K65 RGB RAPIDFIRE renderer metadata.
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
	err := d.saveLightingJSON(filepath.Join(pwd, "database", "rgb", d.Serial+".json"), &proposed)
	if err == nil {
		d.rgbMutex.Lock()
		d.Rgb.Profiles = proposed.Profiles
		d.rgbMutex.Unlock()
	}
	return err
}

func (d *Device) ResetLightingEffectSettings(effect string) error {
	if d == nil {
		return fmt.Errorf("K65 RGB RAPIDFIRE lighting is unavailable")
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
		if d.schedulerDark || off {
			brightness = 0
		}
	}
	return d.DeviceProfile.RGBProfile, brightness, off
}

// The shipped layout is the authority for sparse packet positions and geometry.
// Authored colors and input assignments may vary; transport positions may not.
func (d *Device) validateLightingKeyboard() error {
	if d.DeviceProfile == nil {
		return fmt.Errorf("keyboard profile unavailable")
	}
	p := d.DeviceProfile
	k := p.Keyboards[p.Profile]
	if d.lightingLayouts != nil && d.lightingLayouts[p.Layout] == nil {
		return fmt.Errorf("invalid keyboard layout")
	}
	if k == nil {
		return fmt.Errorf("keyboard profile unavailable")
	}
	template := d.lightingLayouts[k.Layout]
	if template == nil {
		template = keyboards.GetKeyboard(fmt.Sprintf("%s-%s", keyboardKey, k.Layout))
	}
	if k == nil || template == nil || len(k.Row) != len(template.Row) || k.Key != template.Key || k.Layout != template.Layout || k.Rows != template.Rows {
		return fmt.Errorf("keyboard topology unavailable")
	}
	seen := map[int]bool{}
	for id, row := range template.Row {
		actual, ok := k.Row[id]
		if !ok || len(actual.Keys) != len(row.Keys) {
			return fmt.Errorf("invalid keyboard row")
		}
		for keyID, key := range row.Keys {
			value, ok := actual.Keys[keyID]
			if !ok || seen[keyID] || !slices.Equal(value.PacketIndex, key.PacketIndex) || value.Width != key.Width || value.Height != key.Height || value.Left != key.Left || value.Top != key.Top || value.KeySpace != key.KeySpace || value.NoColor != key.NoColor || !slices.Equal(value.KeyEmpty, key.KeyEmpty) || !slices.Equal(value.Spacing, key.Spacing) || !lightingColorOK(value.Color) {
				return fmt.Errorf("invalid keyboard mapping")
			}
			seen[keyID] = true
			for _, index := range value.PacketIndex {
				if index < 0 || index >= 168 {
					return fmt.Errorf("invalid packet index")
				}
			}
		}
	}
	return nil
}
func lightingColorOK(c rgb.Color) bool {
	return lightingsettings.Validate(lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: c.Red, Green: c.Green, Blue: c.Blue}}}) == nil
}
func cloneLightingProfile(p *DeviceProfile) (*DeviceProfile, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var result DeviceProfile
	err = json.Unmarshal(data, &result)
	return &result, err
}
func copyLightingRGBProfile(p rgb.Profile) rgb.Profile {
	if p.Gradients != nil {
		m := make(map[int]rgb.Color, len(p.Gradients))
		for id, c := range p.Gradients {
			m[id] = c
		}
		p.Gradients = m
	}
	return p
}
func sortedLightingKeys(row keyboards.Row) []int {
	ids := make([]int, 0, len(row.Keys))
	for id := range row.Keys {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// Coordinates encode the established 20-column grid, including placeholders and
// wide spans. JSON Left/Top are legacy margins, not absolute key positions.
func (d *Device) keyboardZoneEditor() *lightingpresentation.AuthoredZoneEditor {
	editor := &lightingpresentation.AuthoredZoneEditor{EffectID: "keyboard", Heading: "Keyboard", Description: "Choose colors for the selected keys.", HasGroups: true}
	k := d.DeviceProfile.Keyboards[d.DeviceProfile.Profile]
	rows := make([]int, 0, len(k.Row))
	for id := range k.Row {
		rows = append(rows, id)
	}
	sort.Ints(rows)
	for rowIndex, id := range rows {
		row := k.Row[id]
		column := 0
		for _, keyID := range sortedLightingKeys(row) {
			key := row.Keys[keyID]
			column += len(key.KeyEmpty) + len(key.Spacing)
			span := 1
			for _, class := range strings.Fields(key.KeySpace) {
				if class == "wide" {
					span = 2
				}
				if strings.HasPrefix(class, "wide") && len(class) > 4 {
					if n, err := strconv.Atoi(class[4:]); err == nil {
						span = n
					}
				}
			}
			// The shipped enter-custom class extends the key into the empty cells
			// above it. Preserve that two-row rectangle in logical grid units;
			// packet positions and legacy pixel dimensions are not coordinates.
			top, height := rowIndex*100, 100
			if rowIndex > 0 && slices.Contains(strings.Fields(key.Css), "enter-custom") {
				top -= 100
				height = 200
			}
			if len(key.PacketIndex) > 0 && !key.NoColor {
				editor.Zones = append(editor.Zones, lightingpresentation.AuthoredZone{ID: strconv.Itoa(keyID), Label: key.KeyName, ColorHex: fmt.Sprintf("#%02x%02x%02x", uint8(key.Color.Red), uint8(key.Color.Green), uint8(key.Color.Blue)), GroupID: strconv.Itoa(id), GroupLabel: fmt.Sprintf("Row %d", id), HasGeometry: true, Left: column * 100, Top: top, Width: span * 100, Height: height})
			}
			column += span
		}
	}
	return editor
}
func (d *Device) SetLightingZoneColor(effect, scope, zoneID, groupID string, color rgb.Color) error {
	if d == nil {
		return fmt.Errorf("lighting unavailable")
	}
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	var ids []string
	switch scope {
	case "zone":
		if groupID != "" {
			d.lightingMu.Unlock()
			return fmt.Errorf("invalid group")
		}
		ids = []string{zoneID}
	case "group", "all":
		if zoneID != "" || (scope == "all" && groupID != "") {
			d.lightingMu.Unlock()
			return fmt.Errorf("invalid selection")
		}
		for id, row := range d.DeviceProfile.Keyboards[d.DeviceProfile.Profile].Row {
			if scope == "group" && strconv.Itoa(id) != groupID {
				continue
			}
			for keyID, key := range row.Keys {
				if !key.NoColor && len(key.PacketIndex) > 0 {
					ids = append(ids, strconv.Itoa(keyID))
				}
			}
		}
	default:
		d.lightingMu.Unlock()
		return fmt.Errorf("invalid scope")
	}
	d.lightingMu.Unlock()
	return d.SetLightingZoneColors(effect, ids, color)
}
func (d *Device) SetLightingZoneColors(effect string, ids []string, color rgb.Color) error {
	if d == nil {
		return fmt.Errorf("lighting unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	if effect != "keyboard" || len(ids) == 0 || !lightingColorOK(color) {
		d.lightingMu.Unlock()
		return fmt.Errorf("invalid authored colors")
	}
	proposed, err := cloneLightingProfile(d.DeviceProfile)
	if err != nil {
		d.lightingMu.Unlock()
		return err
	}
	seen := map[string]bool{}
	for _, id := range ids {
		keyID, err := strconv.Atoi(id)
		found := false
		if err == nil && !seen[id] {
			for rowID, row := range proposed.Keyboards[proposed.Profile].Row {
				key, ok := row.Keys[keyID]
				if !ok || key.NoColor || len(key.PacketIndex) == 0 {
					continue
				}
				key.Color = rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue}
				row.Keys[keyID] = key
				proposed.Keyboards[proposed.Profile].Row[rowID] = row
				found = true
			}
		}
		if !found {
			d.lightingMu.Unlock()
			return fmt.Errorf("invalid authored key selection")
		}
		seen[id] = true
	}
	err = d.persistLightingProfile(*proposed)
	selected := proposed.RGBProfile == "keyboard"
	d.lightingMu.Unlock()
	if err == nil && selected {
		d.restartLighting()
	}
	return err
}

// Each renderer owns one stop channel. Closing it is bounded even when the
// renderer has already returned; no sender waits for a dead goroutine.
type lightingRenderer struct {
	rgb  *rgb.ActiveRGB
	once sync.Once
	done chan struct{}
}

func (d *Device) stopLightingRenderer() {
	d.rendererMu.Lock()
	runner := d.lightingRenderer
	d.lightingRenderer = nil
	d.activeRgb = nil
	if runner != nil {
		runner.once.Do(func() { close(runner.rgb.Exit) })
	}
	d.rendererMu.Unlock()
	if runner != nil {
		select {
		case <-runner.done:
		case <-time.After(250 * time.Millisecond):
		}
	}
}
func (d *Device) beginLightingRenderer() *lightingRenderer {
	runner := &lightingRenderer{rgb: rgb.Exit(), done: make(chan struct{})}
	d.rendererMu.Lock()
	d.lightingRenderer = runner
	d.activeRgb = runner.rgb
	d.rendererMu.Unlock()
	return runner
}
func (d *Device) finishLightingRenderer(runner *lightingRenderer) {
	d.rendererMu.Lock()
	if d.lightingRenderer == runner {
		d.lightingRenderer = nil
		d.activeRgb = nil
	}
	d.rendererMu.Unlock()
	close(runner.done)
}
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
		return fmt.Errorf("lighting unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	changed := false
	static := d.DeviceProfile.RGBProfile == "keyboard" || d.DeviceProfile.RGBProfile == "static"
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

// Capture state, authored maps, and effective overrides together for one frame.
func (d *Device) lightingFrameState() (*DeviceProfile, error) {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.DeviceProfile == nil || d.Exit {
		return nil, fmt.Errorf("lighting unavailable")
	}
	if d.lightingDefaults != nil {
		if err := d.validateLightingKeyboard(); err != nil {
			return nil, err
		}
	} else {
		// Legacy eligibility is independent of canonical layout/color metadata.
		keyboard := d.DeviceProfile.Keyboards[d.DeviceProfile.Profile]
		if keyboard == nil {
			return nil, fmt.Errorf("keyboard profile unavailable")
		}
		for _, row := range keyboard.Row {
			for _, key := range row.Keys {
				for _, index := range key.PacketIndex {
					if index < 0 || index >= colorPacketLength {
						return nil, fmt.Errorf("invalid legacy packet index")
					}
					if !d.DeviceProfile.RgbOff && d.DeviceProfile.RGBProfile != "keyboard" && d.DeviceProfile.RGBProfile != "static" && index >= d.LEDChannels {
						return nil, fmt.Errorf("legacy renderer channel unavailable")
					}
				}
			}
		}
	}
	p, err := cloneLightingProfile(d.DeviceProfile)
	if err != nil {
		return nil, err
	}
	if d.lightingDefaults != nil {
		p.RgbOff = d.userRGBOff
		if d.schedulerDark || p.RgbOff {
			p.BrightnessSlider = 0
		}
	}
	return p, nil
}
func (d *Device) switchLightingProfile(name string) error {
	if d == nil {
		return fmt.Errorf("lighting unavailable")
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	target := d.UserProfiles[name]
	if target == nil {
		d.lightingMu.Unlock()
		return fmt.Errorf("unknown profile")
	}
	proposed, err := cloneLightingProfile(target)
	if err != nil {
		d.lightingMu.Unlock()
		return err
	}
	if proposed.Layout == "" {
		proposed.Layout = "US"
	}
	if proposed.Path == "" {
		proposed.Path = d.Serial + ".json"
	}
	probe := &Device{DeviceProfile: proposed, lightingLayouts: d.lightingLayouts}
	if proposed.BrightnessSlider > 100 || probe.validateLightingKeyboard() != nil || !d.SupportsLightingEffect(proposed.RGBProfile) {
		d.lightingMu.Unlock()
		return fmt.Errorf("invalid lighting profile")
	}
	proposed.Path = filepath.Join(pwd, "database", "profiles", filepath.Base(proposed.Path))
	proposed.Active = true
	proposed.RgbOff = false
	current := *d.DeviceProfile
	retired := current
	retired.Active = false
	if proposed.Path != current.Path {
		if err = d.saveLightingJSON(current.Path, retired); err != nil {
			d.lightingMu.Unlock()
			return err
		}
		if err = d.saveLightingJSON(proposed.Path, proposed); err != nil {
			rollback := d.saveLightingJSON(current.Path, current)
			d.lightingMu.Unlock()
			if rollback != nil {
				return fmt.Errorf("profile switch: %w; rollback: %v", err, rollback)
			}
			return err
		}
		d.DeviceProfile.Active = false
	} else if err = d.saveLightingJSON(proposed.Path, proposed); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	*target = *proposed
	d.DeviceProfile = target
	d.schedulerDark = false
	d.userRGBOff = false
	d.lightingMu.Unlock()
	d.restartLighting()
	return nil
}
func (d *Device) updateLightingKeyColor(keyID, option int, color rgb.Color, selections []int) uint8 {
	var err error
	switch option {
	case 0:
		err = d.SetLightingZoneColor("keyboard", "zone", strconv.Itoa(keyID), "", color)
	case 1:
		d.lightingMu.Lock()
		group := ""
		if d.DeviceProfile != nil {
			for id, row := range d.DeviceProfile.Keyboards[d.DeviceProfile.Profile].Row {
				if _, ok := row.Keys[keyID]; ok {
					group = strconv.Itoa(id)
				}
			}
		}
		d.lightingMu.Unlock()
		if group == "" {
			return 0
		}
		err = d.SetLightingZoneColor("keyboard", "group", "", group, color)
	case 2:
		err = d.SetLightingZoneColor("keyboard", "all", "", "", color)
	case 3:
		ids := make([]string, len(selections))
		for i, id := range selections {
			ids[i] = strconv.Itoa(id)
		}
		err = d.SetLightingZoneColors("keyboard", ids, color)
	default:
		return 0
	}
	if err != nil {
		return 0
	}
	return 1
}

// Keyboard-profile operations keep input/profile fields while copying authored
// state, validating the selected topology, and persisting before publication.
func (d *Device) mutateLightingKeyboard(change func(*DeviceProfile) error, restart bool) error {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if err := d.lightingReady(); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	p, err := cloneLightingProfile(d.DeviceProfile)
	if err != nil {
		d.lightingMu.Unlock()
		return err
	}
	if err = change(p); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	if (&Device{DeviceProfile: p, lightingLayouts: d.lightingLayouts}).validateLightingKeyboard() != nil {
		d.lightingMu.Unlock()
		return fmt.Errorf("invalid keyboard profile")
	}
	detached, copyErr := cloneLightingProfile(p)
	if copyErr != nil {
		d.lightingMu.Unlock()
		return copyErr
	}
	err = d.persistLightingProfile(*detached)
	d.lightingMu.Unlock()
	if err == nil && restart {
		d.restartLighting()
	}
	return err
}

// LightingPreview is an inert snapshot constructor over an already loaded
// shipped keyboard fixture. It never attaches a runtime or touches hardware.
func LightingPreview(keyboard *keyboards.Keyboard) (lightingpresentation.Snapshot, bool) {
	if keyboard == nil {
		return lightingpresentation.Snapshot{}, false
	}
	p := &DeviceProfile{Layout: keyboard.Layout, Profile: "default", Keyboards: map[string]*keyboards.Keyboard{"default": keyboard}, RGBProfile: "keyboard", BrightnessSlider: 70}
	d := &Device{DeviceProfile: p}
	if d.validateLightingKeyboard() != nil {
		return lightingpresentation.Snapshot{}, false
	}
	s := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "keyboard", EffectSupported: true, EffectSelectionAvailable: true, HasBrightness: true, Brightness: 70, AuthoredZoneEditor: d.keyboardZoneEditor()}
	for _, id := range rgbModes {
		label := "Keyboard"
		if id != "keyboard" {
			descriptor, ok := rgb.SoftwareEffectDescriptorByID(id)
			if !ok {
				return lightingpresentation.Snapshot{}, false
			}
			label = descriptor.Label
		}
		s.SupportedEffects = append(s.SupportedEffects, lightingpresentation.EffectOption{ID: id, Label: label})
	}
	return s, true
}

// Tests can fail the persistence boundary without publishing a proposal.
func (d *Device) saveLightingJSON(path string, value interface{}) error {
	if d.lightingPersist != nil {
		return d.lightingPersist(path, value)
	}
	return saveLightingJSON(path, value)
}
