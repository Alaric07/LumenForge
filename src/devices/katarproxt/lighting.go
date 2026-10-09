package katarproxt

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

// KATAR PRO XT deliberately adapts its existing active device profile and RGB file.
// There is no second desired-state or effect-settings database. The shared
// defaults repository supplies immutable reset values only.
func (d *Device) attachLightingRuntime(paths config.Paths) error {
	defaults, err := lightingsettings.LoadDefaultRepository(filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		return err
	}
	if d == nil {
		return fmt.Errorf("KATAR PRO XT device is unavailable")
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.Serial == "" || d.dev == nil || !d.Connected || d.Exit || d.LEDChannels != 1 || d.ChangeableLedChannels != 1 || d.ZoneAmount != 1 || d.DeviceProfile == nil || d.DeviceProfile.BrightnessSlider == nil || *d.DeviceProfile.BrightnessSlider > 100 || d.Rgb == nil {
		return fmt.Errorf("KATAR PRO XT lighting backing state is unavailable")
	}
	zone, ok := d.DeviceProfile.ZoneColors[0]
	if !ok || zone.Color == nil || len(d.DeviceProfile.ZoneColors) != 1 || !slices.Equal(zone.ColorIndex, []int{0, 1, 2}) {
		return fmt.Errorf("KATAR PRO XT Scroll zone is unavailable")
	}
	for _, effect := range rgbModes {
		if effect == "mouse" {
			continue
		}
		profile := d.GetRgbProfile(effect)
		if profile == nil {
			return fmt.Errorf("KATAR PRO XT lighting profile %q is unavailable", effect)
		}
		if _, err := xtEffectSettingsFromProfile(effect, *profile); err != nil {
			return err
		}
	}
	if err := validateXTLightingProfile(*d.DeviceProfile); err != nil {
		return err
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
	if d == nil || d.Serial == "" || d.LEDChannels != 1 || d.ChangeableLedChannels != 1 || d.ZoneAmount != 1 || d.dev == nil || !d.Connected || d.Exit || d.lightingDefaults == nil || d.DeviceProfile == nil || d.DeviceProfile.BrightnessSlider == nil || d.Rgb == nil || *d.DeviceProfile.BrightnessSlider > 100 {
		return fmt.Errorf("KATAR PRO XT canonical lighting is unavailable")
	}
	d.rgbMutex.RLock()
	rgbAvailable := d.Rgb.Profiles != nil
	d.rgbMutex.RUnlock()
	if !rgbAvailable {
		return fmt.Errorf("KATAR PRO XT RGB backing is unavailable")
	}
	zone, ok := d.DeviceProfile.ZoneColors[0]
	if !ok || zone.Color == nil || len(d.DeviceProfile.ZoneColors) != 1 || !slices.Equal(zone.ColorIndex, []int{0, 1, 2}) {
		return fmt.Errorf("KATAR PRO XT Scroll zone is unavailable")
	}
	return validateXTLightingProfile(*d.DeviceProfile)
}

func (d *Device) resolveLightingSettings(effect string) (lightingsettings.EffectSettings, error) {
	if err := d.lightingReady(); err != nil {
		return lightingsettings.EffectSettings{}, err
	}
	if effect == "mouse" || !d.SupportsLightingEffect(effect) {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported KATAR PRO XT effect %q", effect)
	}
	profile := d.GetRgbProfile(effect)
	if profile == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("KATAR PRO XT effect settings are unavailable")
	}
	return xtEffectSettingsFromProfile(effect, *profile)
}

func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	if d == nil {
		return lightingsettings.EffectSettings{}, fmt.Errorf("KATAR PRO XT lighting is unavailable")
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.resolveLightingSettings(effect)
}

func katarXTColorHex(c lightingsettings.Color) string {
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
		snapshot.SingleColorHex = katarXTColorHex(settings.SingleColor.Color)
	}
	if settings.TwoColor != nil {
		snapshot.TwoColorStartHex, snapshot.TwoColorEndHex = katarXTColorHex(settings.TwoColor.Start), katarXTColorHex(settings.TwoColor.End)
	}
	if settings.Temperature != nil {
		snapshot.HasTemperature = true
		snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: katarXTColorHex(settings.Temperature.Low.Color), Celsius: settings.Temperature.Low.Celsius}
		snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: katarXTColorHex(settings.Temperature.Middle.Color), Celsius: settings.Temperature.Middle.Celsius}
		snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: katarXTColorHex(settings.Temperature.High.Color), Celsius: settings.Temperature.High.Celsius}
	}
	if settings.Gradient != nil {
		snapshot.HasGradient = true
		for _, stop := range settings.Gradient.Stops {
			snapshot.GradientStops = append(snapshot.GradientStops, lightingpresentation.GradientStop{Position: stop.Position, ColorHex: katarXTColorHex(stop.Color), Intensity: stop.Intensity})
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
	if err := validateXTLightingProfile(profile); err != nil {
		return err
	}
	if err := d.writeLightingJSON(profile.Path, &profile); err != nil {
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
	d.stopLighting()
	d.setDeviceColor()
}

// No sends or waits on a renderer that may already have exited. A transport
// operation already in progress is not cancelled; the HID boundary is unchanged.
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
	owner := rgb.Exit()
	stop := make(chan struct{})
	done := make(chan struct{})
	d.activeRgb, d.rendererStop, d.rendererDone = owner, stop, done
	return owner, stop, done
}

func (d *Device) retireLightingRenderer(owner *rgb.ActiveRGB, stop chan struct{}) {
	d.rendererMu.Lock()
	defer d.rendererMu.Unlock()
	if d.rendererStop == stop {
		d.rendererStop = nil
		d.activeRgb = nil
	}
}

func (d *Device) rendererCurrent(stop chan struct{}) bool {
	d.rendererMu.Lock()
	defer d.rendererMu.Unlock()
	return d.rendererStop == stop
}

func lightingFrameDelay(stop <-chan struct{}) bool {
	timer := time.NewTimer(40 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-stop:
		return false
	case <-timer.C:
		return true
	}
}

func (d *Device) writeLightingJSON(path string, value interface{}) error {
	if d.lightingWriteJSON != nil {
		return d.lightingWriteJSON(path, value)
	}
	return common.SaveJsonData(path, value)
}

func (d *Device) SetLightingEffect(effect string) error {
	if d == nil {
		return fmt.Errorf("KATAR PRO XT lighting is unavailable")
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
		return fmt.Errorf("unsupported KATAR PRO XT effect %q", effect)
	}
	if effect != "mouse" {
		if _, err := d.resolveLightingSettings(effect); err != nil {
			d.lightingMu.Unlock()
			return err
		}
	}
	profile := cloneXTProfile(*d.DeviceProfile)
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
		return fmt.Errorf("KATAR PRO XT lighting is unavailable")
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
		return fmt.Errorf("KATAR PRO XT brightness mutation is unavailable")
	}
	profile := cloneXTProfile(*d.DeviceProfile)
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
		return fmt.Errorf("KATAR PRO XT lighting is unavailable")
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
		return fmt.Errorf("KATAR PRO XT effect settings mismatch")
	}
	if err := lightingsettings.Validate(value); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	profile := *d.GetRgbProfile(effect)
	rendered := lightingsettings.RendererProfileFromEffectSettings(value)
	// Only editable fields change. Preserve MinTemp/MaxTemp, Smoothness,
	// brightness modes, version and all other KATAR PRO XT renderer metadata.
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
	if _, err := xtEffectSettingsFromProfile(effect, profile); err != nil {
		return err
	}
	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()
	proposed := *d.Rgb
	proposed.Profiles = make(map[string]rgb.Profile, len(d.Rgb.Profiles))
	for key, existing := range d.Rgb.Profiles {
		proposed.Profiles[key] = copyLightingRGBProfile(existing)
	}
	proposed.Profiles[effect] = copyLightingRGBProfile(profile)
	if err := d.writeLightingJSON(filepath.Join(pwd, "database", "rgb", d.Serial+".json"), &proposed); err != nil {
		return err
	}
	d.Rgb.Profiles = proposed.Profiles
	return nil
}

func (d *Device) ResetLightingEffectSettings(effect string) error {
	if d == nil {
		return fmt.Errorf("KATAR PRO XT lighting is unavailable")
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
		return fmt.Errorf("invalid KATAR PRO XT zone selection")
	}
	return d.SetLightingZoneColors(effect, []string{"0"}, color)
}
func (d *Device) SetLightingZoneColors(effect string, zoneIDs []string, color rgb.Color) error {
	if d == nil {
		return fmt.Errorf("KATAR PRO XT lighting is unavailable")
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
		return fmt.Errorf("invalid KATAR PRO XT zone selection")
	}
	value := lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: color.Red, Green: color.Green, Blue: color.Blue}}}
	if err := lightingsettings.Validate(value); err != nil {
		d.lightingMu.Unlock()
		return err
	}
	profile := cloneXTProfile(*d.DeviceProfile)
	zone := profile.ZoneColors[0]
	authored := *zone.Color
	authored.Red, authored.Green, authored.Blue = color.Red, color.Green, color.Blue
	authored.Hex = katarXTColorHex(value.SingleColor.Color)
	zone.Color = &authored
	profile.ZoneColors = map[int]ZoneColors{0: zone}
	err := d.persistLightingProfile(profile)
	d.lightingMu.Unlock()
	if err == nil && profile.RGBProfile == "mouse" {
		d.restartLighting()
	}
	return err
}

// Copy before applying either runtime Sniper substitution or brightness.
func (d *Device) mouseOutputColor(authored *rgb.Color, brightness uint8) rgb.Color {
	if authored == nil {
		return rgb.Color{}
	}
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

// Called with lightingMu held. Mouse colors are device-owned authored state;
// shared conversion/validation owns the complete generic effect contract.
func validateXTLightingProfile(profile DeviceProfile) error {
	if profile.BrightnessSlider == nil || *profile.BrightnessSlider > 100 {
		return fmt.Errorf("invalid XT desired brightness")
	}
	zone, ok := profile.ZoneColors[0]
	if !ok || len(profile.ZoneColors) != 1 || zone.Name != "Scroll" || zone.Color == nil || !slices.Equal(zone.ColorIndex, []int{0, 1, 2}) {
		return fmt.Errorf("invalid XT Scroll topology")
	}
	value := lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: zone.Color.Red, Green: zone.Color.Green, Blue: zone.Color.Blue}}}
	return lightingsettings.Validate(value)
}

func cloneXTProfile(profile DeviceProfile) DeviceProfile {
	if profile.BrightnessSlider != nil {
		value := *profile.BrightnessSlider
		profile.BrightnessSlider = &value
	}
	zones := make(map[int]ZoneColors, len(profile.ZoneColors))
	for id, zone := range profile.ZoneColors {
		zone.ColorIndex = append([]int(nil), zone.ColorIndex...)
		if zone.Color != nil {
			color := *zone.Color
			zone.Color = &color
		}
		zones[id] = zone
	}
	if profile.ZoneColors != nil {
		profile.ZoneColors = zones
	}
	profiles := make(map[int]DPIProfile, len(profile.Profiles))
	for id, stage := range profile.Profiles {
		if stage.Color != nil {
			color := *stage.Color
			stage.Color = &color
		}
		indices := make(map[int][]int, len(stage.ColorIndex))
		for index, values := range stage.ColorIndex {
			indices[index] = append([]int(nil), values...)
		}
		if stage.ColorIndex != nil {
			stage.ColorIndex = indices
		}
		profiles[id] = stage
	}
	if profile.Profiles != nil {
		profile.Profiles = profiles
	}
	return profile
}

func normalizeXTProfile(profile DeviceProfile) DeviceProfile {
	profile = cloneXTProfile(profile)
	if profile.BrightnessSlider == nil {
		value := uint8(100)
		profile.BrightnessSlider = &value
	}
	if profile.PollingRate == 0 {
		profile.PollingRate = 4
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
		profile.Profiles[len(profile.Profiles)] = DPIProfile{Name: "Sniper", Value: 200, Sniper: true, ColorIndex: map[int][]int{0: {0, 1, 2}}, Color: &rgb.Color{Red: 255, Green: 255, Brightness: 1, Hex: "#ffff00"}}
	}
	profile.RgbOff = false
	return profile
}

// Preserve XT's unscaled input-owned 500-ms feedback on the same Scroll LED.
func (d *Device) dpiFlashFrame(profile DPIProfile) []byte {
	if profile.Color == nil {
		return nil
	}
	color := *profile.Color
	static := map[int][]byte{}
	for i := 0; i < d.LEDChannels; i++ {
		static[i] = []byte{byte(color.Red), byte(color.Green), byte(color.Blue)}
	}
	return rgb.SetColor(static)
}

func (d *Device) lightingAttached() bool {
	if d == nil {
		return false
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.lightingDefaults != nil
}

func (d *Device) lightingTemperatures() (float32, float32) {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.CpuTemp, d.GpuTemp
}

func (d *Device) changeLightingBrightnessMetadata(mode uint8) uint8 {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if d.lightingReady() != nil || mode > 3 {
		d.lightingMu.Unlock()
		return 0
	}
	candidate := cloneXTProfile(*d.DeviceProfile)
	candidate.Brightness = mode
	err := d.persistLightingProfile(candidate)
	d.lightingMu.Unlock()
	if err != nil {
		return 0
	}
	d.restartLighting()
	return 1
}

// Recording an override requires attached state, not a hardware write. This
// wired package has no wake/replay lifecycle; unavailable output stays gated.
func (d *Device) setLightingScheduler(value uint8) uint8 {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	d.schedulerDark = value == 0
	ready := d.lightingReady() == nil
	static := d.DeviceProfile != nil && (d.DeviceProfile.RGBProfile == "mouse" || d.DeviceProfile.RGBProfile == "static")
	d.lightingMu.Unlock()
	if ready && static {
		d.restartLighting()
	}
	return 1
}

func (d *Device) setLightingRGBOff(value bool) {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	d.userRGBOff = value
	ready := d.lightingReady() == nil
	d.lightingMu.Unlock()
	if ready {
		d.restartLighting()
	}
}

func (d *Device) changeLightingGradient(effect string, remove bool) (uint8, uint) {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if _, err := d.resolveLightingSettings(effect); err != nil {
		d.lightingMu.Unlock()
		return 0, 0
	}
	profile := d.GetRgbProfile(effect)
	if effect != "gradient" || profile.Gradients == nil {
		d.lightingMu.Unlock()
		return 0, 0
	}
	index := len(profile.Gradients)
	if remove {
		if index < 3 {
			d.lightingMu.Unlock()
			return 2, 0
		}
		index--
		delete(profile.Gradients, index)
	} else {
		// The canonical gradient editor requires complete position/intensity data.
		// Preserve existing stops; add cyan at the midpoint of the largest gap.
		positions := []float64{0, 1}
		for _, color := range profile.Gradients {
			positions = append(positions, color.Position)
		}
		sort.Float64s(positions)
		position, gap := 0.0, -1.0
		for i := 1; i < len(positions); i++ {
			if diff := positions[i] - positions[i-1]; diff > gap {
				gap = diff
				position = (positions[i] + positions[i-1]) / 2
			}
		}
		profile.Gradients[index] = rgb.Color{Green: 255, Blue: 255, Position: position, Brightness: 1}
	}
	err := d.persistLightingRGBProfile(effect, *profile)
	selected := d.DeviceProfile.RGBProfile == effect
	d.lightingMu.Unlock()
	if err != nil {
		return 0, 0
	}
	if selected {
		d.restartLighting()
	}
	return 1, uint(index)
}

// Only this focused switch uses proposed profiles and observable errors. The
// two established files are retained; failed second writes attempt restoration
// of the first, without ever publishing the candidate or changing output.
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
	candidate := normalizeXTProfile(*target)
	if validateXTLightingProfile(candidate) != nil {
		d.lightingMu.Unlock()
		return 0
	}
	stage, ok := candidate.Profiles[candidate.Profile]
	if !ok || stage.Sniper || stage.Color == nil {
		d.lightingMu.Unlock()
		return 0
	}
	// Recognized selections must have complete settings; unknown legacy selections
	// remain loadable for display but cannot be selected by the canonical API.
	if candidate.RGBProfile != "mouse" && d.SupportsLightingEffect(candidate.RGBProfile) {
		if _, err := d.resolveLightingSettings(candidate.RGBProfile); err != nil {
			d.lightingMu.Unlock()
			return 0
		}
	}
	current := d.DeviceProfile
	old := cloneXTProfile(*current)
	inactive := cloneXTProfile(old)
	inactive.Active = false
	inactive.RgbOff = false
	candidate.Active = true
	inactive.Path = xtLightingProfilePath(inactive, d.Serial)
	candidate.Path = xtLightingProfilePath(candidate, d.Serial)
	if candidate.Path == inactive.Path && target != current {
		d.lightingMu.Unlock()
		return 0
	}
	if target != current {
		if err := d.writeLightingJSON(inactive.Path, &inactive); err != nil {
			d.lightingMu.Unlock()
			return 0
		}
	}
	if err := d.writeLightingJSON(candidate.Path, &candidate); err != nil {
		if target != current {
			_ = d.writeLightingJSON(inactive.Path, &old)
		}
		d.lightingMu.Unlock()
		return 0
	}
	if target != current {
		*current = inactive
	}
	*target = candidate
	d.DeviceProfile = target
	d.userRGBOff = false
	d.lightingMu.Unlock()
	d.restartLighting()
	if d.lightingProfileChanged != nil {
		d.lightingProfileChanged()
	} else {
		d.toggleDPI(false)
		d.loadKeyAssignments()
		d.setupKeyAssignment()
	}
	return 1
}

func xtLightingProfilePath(profile DeviceProfile, serial string) string {
	path := profile.Path
	if path == "" {
		path = serial + ".json"
	}
	return filepath.Join(pwd, "database", "profiles", filepath.Base(path))
}

// A one-shot local refresh must not make a mutation wait behind a retired HID
// write. Legacy fallback keeps its synchronous output path; attached refreshes
// capture their own cancellation/retirement identity just like dynamic frames.
func (d *Device) writeSingleLightingFrame(data []byte) {
	if !d.lightingAttached() {
		d.writeColor(data)
		return
	}
	owner, stop, done := d.startLightingRenderer()
	frame := append([]byte(nil), data...)
	go func() {
		defer close(done)
		defer d.retireLightingRenderer(owner, stop)
		if d.rendererCurrent(stop) {
			d.writeColor(frame, stop)
		}
	}()
}

func xtEffectSettingsFromProfile(effect string, profile rgb.Profile) (lightingsettings.EffectSettings, error) {
	value, err := lightingsettings.EffectSettingsFromRGBProfile(effect, profile)
	if err != nil {
		return value, err
	}
	if effect == "gradient" {
		for index := 0; index < len(profile.Gradients); index++ {
			if _, ok := profile.Gradients[index]; !ok {
				return lightingsettings.EffectSettings{}, fmt.Errorf("XT gradient renderer requires contiguous stops")
			}
		}
	}
	return value, nil
}
