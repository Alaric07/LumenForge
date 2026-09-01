package memory

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

// LightingDeviceID identifies the controller in the shared Devices workspace.
func (d *Device) LightingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// canonicalChannel reports only physical internal RGB ports. Generated custom
// LED and external-hub channels intentionally remain outside this milestone:
// their current ChannelId values are topology-derived rather than persistence
// identities.
func (d *Device) canonicalChannel(channel *Devices) bool {
	return d != nil && channel != nil && channel.ChannelId >= 0 && channel.ChannelId < maximumRegisters && channel.LedChannels > 0
}

func (d *Device) canonicalChannelTargetID(channelID int) string {
	if d == nil || d.Serial == "" || channelID < 0 || channelID >= maximumRegisters {
		return ""
	}
	return d.Serial + "-rgb-" + strconv.Itoa(channelID)
}

func (d *Device) attachCanonicalChannelLightingRuntime(paths config.Paths) error {
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile, filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		return err
	}
	if runtime == nil || runtime.State == nil {
		return fmt.Errorf("canonical channel lighting state is unavailable")
	}
	d.channelLightingState = runtime.State
	d.channelLightingEffects = runtime.Effects
	d.channelLightingResolver = runtime.Resolver
	if err := d.hydrateCanonicalChannels(); err != nil {
		d.channelLightingState, d.channelLightingEffects, d.channelLightingResolver = nil, nil, nil
		return err
	}
	return nil
}

func (d *Device) SupportsLightingEffect(effect string) bool {
	if d == nil || d.GetRgbProfile(effect) == nil {
		return false
	}
	// The selected effect is stored through the shared independent-target state
	// store, so do not advertise a legacy Memory RGB profile it cannot persist.
	return lightingsettings.ValidateIndependentDeviceLightingState(lightingsettings.IndependentDeviceLightingState{SelectedEffect: effect, Brightness: 100}) == nil
}

func (d *Device) canonicalRendererProfile(channelID int, effect string) (rgb.Profile, bool) {
	if d == nil || d.channelLightingResolver == nil {
		return rgb.Profile{}, false
	}
	if _, supported := rgb.SoftwareEffectDescriptorByID(effect); !supported {
		return rgb.Profile{}, false
	}
	targetID := d.canonicalChannelTargetID(channelID)
	if targetID == "" {
		return rgb.Profile{}, false
	}
	resolution, err := d.channelLightingResolver.Resolve(lightingsettings.IndependentDevice(targetID), effect)
	if err != nil || resolution.Settings.EffectID != effect {
		return rgb.Profile{}, false
	}
	return lightingsettings.RendererProfileFromEffectSettings(resolution.Settings), true
}

func (d *Device) channelRendererProfile(channel *Devices, effect string) *rgb.Profile {
	if d.canonicalChannel(channel) {
		if profile, ok := d.canonicalRendererProfile(channel.ChannelId, effect); ok {
			return &profile
		}
	}
	return d.GetRgbProfile(effect)
}

// channelRendererUsesResolvedColors preserves the legacy zero-color heuristic
// for non-canonical channels. Canonical settings use descriptor palette
// semantics: a single-color profile intentionally has no EndColor.
func (d *Device) channelRendererUsesResolvedColors(channel *Devices, effect string, profile *rgb.Profile) bool {
	if d.canonicalChannel(channel) {
		descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
		if ok {
			switch descriptor.PaletteKind {
			case rgb.LightingPaletteStaticSingle, rgb.LightingPaletteTwoColor, rgb.LightingPaletteTemperatureThree:
				return true
			case rgb.LightingPaletteGradient, rgb.LightingPaletteGenerated, rgb.LightingPaletteNone:
				return false
			}
		}
	}
	return profile != nil && (rgb.Color{}) != profile.StartColor && (rgb.Color{}) != profile.EndColor
}

func (d *Device) channelSelectedEffect(channelID int) (string, error) {
	if d == nil || d.channelLightingState == nil {
		return "", fmt.Errorf("canonical channel lighting state is unavailable")
	}
	targetID := d.canonicalChannelTargetID(channelID)
	if targetID == "" {
		return "", fmt.Errorf("canonical lighting channel is unavailable")
	}
	state, _, err := d.channelLightingState.Resolve(targetID)
	if err != nil {
		return "", err
	}
	if !d.SupportsLightingEffect(state.SelectedEffect) {
		return "", fmt.Errorf("canonical channel effect %q is unsupported", state.SelectedEffect)
	}
	return state.SelectedEffect, nil
}

func (d *Device) setChannelSelectedEffect(channelID int, effect string) error {
	if !d.SupportsLightingEffect(effect) {
		return fmt.Errorf("unsupported lighting effect %q", effect)
	}
	if d == nil || d.channelLightingState == nil {
		return fmt.Errorf("canonical channel lighting state is unavailable")
	}
	targetID := d.canonicalChannelTargetID(channelID)
	if targetID == "" {
		return fmt.Errorf("canonical lighting channel is unavailable")
	}
	state, _, err := d.channelLightingState.Resolve(targetID)
	if err != nil {
		return err
	}
	state.SelectedEffect = effect
	return d.channelLightingState.Set(targetID, state)
}

func (d *Device) hydrateCanonicalChannels() error {
	if d == nil || d.channelLightingState == nil {
		return fmt.Errorf("canonical channel lighting state is unavailable")
	}
	for _, channel := range d.Devices {
		if !d.canonicalChannel(channel) {
			continue
		}
		effect, err := d.channelSelectedEffect(channel.ChannelId)
		if err != nil {
			return err
		}
		channel.RGB = effect
	}
	return nil
}

func (d *Device) restoreCanonicalChannelProfile(profile *DeviceProfile) error {
	if d == nil || profile == nil || d.channelLightingState == nil {
		return fmt.Errorf("canonical channel lighting state is unavailable")
	}
	// Validate every saved selection before persisting any channel so malformed
	// profile data cannot leave a partly restored controller state.
	type selection struct {
		channelID int
		effect    string
	}
	selections := make([]selection, 0, len(d.Devices))
	for _, channel := range d.Devices {
		if !d.canonicalChannel(channel) {
			continue
		}
		effect, ok := profile.RGBProfiles[channel.ChannelId]
		if !ok || !d.SupportsLightingEffect(effect) {
			return fmt.Errorf("saved lighting effect for channel %d is invalid", channel.ChannelId)
		}
		selections = append(selections, selection{channel.ChannelId, effect})
	}
	for _, selection := range selections {
		if err := d.setChannelSelectedEffect(selection.channelID, selection.effect); err != nil {
			return err
		}
	}
	return d.hydrateCanonicalChannels()
}

// LightingSnapshot exposes each stable physical RGB port independently. It is
// presentation-only and never reports a transient renderer frame as state.
func (d *Device) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	if d == nil || d.channelLightingState == nil || d.DeviceProfile == nil {
		return lightingpresentation.Snapshot{}, false
	}
	snapshot := lightingpresentation.Snapshot{
		TargetKind:         "native",
		ClusterControlled:  d.DeviceProfile.RGBCluster,
		ExternalControlled: d.DeviceProfile.OpenRGBIntegration,
		Channels:           make([]lightingpresentation.Channel, 0, len(d.Devices)),
	}
	if d.DeviceProfile.BrightnessSlider != nil {
		snapshot.HasBrightness = true
		snapshot.Brightness = *d.DeviceProfile.BrightnessSlider
	}
	for _, channel := range d.Devices {
		if !d.canonicalChannel(channel) {
			continue
		}
		effect, err := d.channelSelectedEffect(channel.ChannelId)
		if err != nil {
			return lightingpresentation.Snapshot{}, false
		}
		child := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: effect, EffectSupported: d.SupportsLightingEffect(effect), ClusterControlled: d.DeviceProfile.RGBCluster, ExternalControlled: d.DeviceProfile.OpenRGBIntegration, SupportedEffects: make([]lightingpresentation.EffectOption, 0, len(rgbModes))}
		for _, candidate := range rgbModes {
			if d.SupportsLightingEffect(candidate) {
				child.SupportedEffects = append(child.SupportedEffects, lightingpresentation.EffectOption{ID: candidate, Label: candidate})
			}
		}
		if err := d.populateCanonicalChannelSnapshot(&child, channel.ChannelId, effect); err != nil {
			return lightingpresentation.Snapshot{}, false
		}
		if effect == "led" {
			child.IndexedColors = make([]lightingpresentation.IndexedColor, int(channel.LedChannels))
			colors := d.getLedProfileColor(channel.ChannelId, 0)
			for index := range child.IndexedColors {
				color := rgb.Color{}
				if colors != nil {
					color = colors[index]
				}
				child.IndexedColors[index] = lightingpresentation.IndexedColor{Index: index, Label: fmt.Sprintf("LED %d", index+1), ColorHex: fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))}
			}
		}
		channelSnapshot := lightingpresentation.Channel{TargetID: d.canonicalChannelTargetID(channel.ChannelId), ChannelID: strconv.Itoa(channel.ChannelId), Name: channel.Name, Label: channel.Label, LEDCount: int(channel.LedChannels), Lighting: child}
		snapshot.Channels = append(snapshot.Channels, channelSnapshot)
	}
	sort.Slice(snapshot.Channels, func(i, j int) bool { return snapshot.Channels[i].ChannelID < snapshot.Channels[j].ChannelID })
	return snapshot, len(snapshot.Channels) > 0
}

func memoryLightingColorHex(color lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
}

func (d *Device) populateCanonicalChannelSnapshot(snapshot *lightingpresentation.Snapshot, channelID int, effect string) error {
	if snapshot == nil {
		return fmt.Errorf("channel lighting snapshot is unavailable")
	}
	descriptor, generic := rgb.SoftwareEffectDescriptorByID(effect)
	if !generic {
		return nil // probe-temperature retains its existing channel-owned settings.
	}
	if d.channelLightingResolver == nil {
		return nil // lightweight snapshot callers retain the existing effect-only view.
	}
	resolution, err := d.channelLightingResolver.Resolve(lightingsettings.IndependentDevice(d.canonicalChannelTargetID(channelID)), effect)
	if err != nil || resolution.Settings.EffectID != effect {
		return fmt.Errorf("resolve canonical channel effect settings: %w", err)
	}
	settings := resolution.Settings
	snapshot.Customized = resolution.Customized
	snapshot.PaletteKind = string(descriptor.PaletteKind)
	if descriptor.SupportsSpeed && settings.Speed != nil {
		snapshot.HasSpeed, snapshot.Speed = true, *settings.Speed
	}
	switch descriptor.PaletteKind {
	case rgb.LightingPaletteStaticSingle:
		if settings.SingleColor != nil {
			snapshot.SingleColorHex = memoryLightingColorHex(settings.SingleColor.Color)
		}
	case rgb.LightingPaletteTwoColor:
		if settings.TwoColor != nil {
			snapshot.TwoColorStartHex, snapshot.TwoColorEndHex = memoryLightingColorHex(settings.TwoColor.Start), memoryLightingColorHex(settings.TwoColor.End)
		}
	case rgb.LightingPaletteTemperatureThree:
		if settings.Temperature != nil {
			snapshot.HasTemperature = true
			snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: memoryLightingColorHex(settings.Temperature.Low.Color), Celsius: settings.Temperature.Low.Celsius}
			snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: memoryLightingColorHex(settings.Temperature.Middle.Color), Celsius: settings.Temperature.Middle.Celsius}
			snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: memoryLightingColorHex(settings.Temperature.High.Color), Celsius: settings.Temperature.High.Celsius}
		}
	case rgb.LightingPaletteGradient:
		if settings.Gradient != nil {
			snapshot.HasGradient = true
			snapshot.GradientStops = make([]lightingpresentation.GradientStop, len(settings.Gradient.Stops))
			for index, stop := range settings.Gradient.Stops {
				snapshot.GradientStops[index] = lightingpresentation.GradientStop{Position: stop.Position, ColorHex: memoryLightingColorHex(stop.Color), Intensity: stop.Intensity}
			}
		}
	}
	return nil
}

// SetLightingEffect remains the single-target compatibility method. Multi-port
// callers must use SetLightingChannelEffect with a canonical target identity.
func (d *Device) SetLightingEffect(string) error {
	return fmt.Errorf("a lighting channel target is required")
}

func (d *Device) SetLightingChannelEffect(targetID, effect string) error {
	if d == nil || d.DeviceProfile == nil {
		return fmt.Errorf("lighting ownership is unavailable")
	}
	if d.DeviceProfile.RGBCluster || d.DeviceProfile.OpenRGBIntegration {
		return fmt.Errorf("lighting is externally owned")
	}
	for _, channel := range d.Devices {
		if !d.canonicalChannel(channel) || d.canonicalChannelTargetID(channel.ChannelId) != targetID {
			continue
		}
		if err := d.setChannelSelectedEffect(channel.ChannelId, effect); err != nil {
			return err
		}
		channel.RGB = effect
		if d.DeviceProfile.RGBProfiles == nil {
			d.DeviceProfile.RGBProfiles = make(map[int]string)
		}
		// This is an active full-profile snapshot write, not a runtime read.
		// Runtime rendering continues to use the canonical state hydrated above.
		d.DeviceProfile.RGBProfiles[channel.ChannelId] = effect
		d.saveDeviceProfile()
		d.restartCanonicalChannelLighting()
		return nil
	}
	return fmt.Errorf("lighting channel is unavailable")
}

func (d *Device) SetLightingIndexedColor(targetID string, index int, color lightingsettings.Color) error {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.RGBCluster || d.DeviceProfile.OpenRGBIntegration {
		return fmt.Errorf("lighting is externally owned")
	}
	for _, channel := range d.Devices {
		if !d.canonicalChannel(channel) || d.canonicalChannelTargetID(channel.ChannelId) != targetID {
			continue
		}
		effect, err := d.channelSelectedEffect(channel.ChannelId)
		if err != nil || effect != "led" || index < 0 || index >= int(channel.LedChannels) {
			return fmt.Errorf("indexed lighting color is unavailable")
		}
		d.deviceLock.Lock()
		if d.DeviceProfile.RGBPerLed == nil {
			d.DeviceProfile.RGBPerLed = make(map[int]map[int]map[int]rgb.Color)
		}
		if d.DeviceProfile.RGBPerLed[channel.ChannelId] == nil {
			d.DeviceProfile.RGBPerLed[channel.ChannelId] = make(map[int]map[int]rgb.Color)
		}
		if colors := d.DeviceProfile.RGBPerLed[channel.ChannelId][0]; colors == nil || len(colors) != int(channel.LedChannels) {
			d.DeviceProfile.RGBPerLed[channel.ChannelId][0] = d.generateLedObject(channel.LedChannels)
		}
		d.DeviceProfile.RGBPerLed[channel.ChannelId][0][index] = rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue, Hex: fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))}
		d.deviceLock.Unlock()
		d.saveDeviceProfile()
		d.restartCanonicalChannelLighting()
		return nil
	}
	return fmt.Errorf("indexed lighting target is unavailable")
}

func (d *Device) SetLightingIndexedColors(targetID string, colors []lightingsettings.IndexedColor) error {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.RGBCluster || d.DeviceProfile.OpenRGBIntegration {
		return fmt.Errorf("lighting is externally owned")
	}
	for _, channel := range d.Devices {
		if !d.canonicalChannel(channel) || d.canonicalChannelTargetID(channel.ChannelId) != targetID {
			continue
		}
		effect, err := d.channelSelectedEffect(channel.ChannelId)
		if err != nil || effect != "led" || len(colors) != int(channel.LedChannels) {
			return fmt.Errorf("indexed lighting colors are unavailable")
		}
		next := make(map[int]rgb.Color, len(colors))
		for _, item := range colors {
			if item.Index < 0 || item.Index >= int(channel.LedChannels) || len(item.ColorHex) != 7 || item.ColorHex[0] != '#' {
				return fmt.Errorf("indexed lighting colors are invalid")
			}
			value, parseErr := strconv.ParseUint(item.ColorHex[1:], 16, 32)
			if parseErr != nil || !strings.EqualFold(fmt.Sprintf("#%06x", value), item.ColorHex) {
				return fmt.Errorf("indexed lighting colors are invalid")
			}
			if _, duplicate := next[item.Index]; duplicate {
				return fmt.Errorf("indexed lighting colors are invalid")
			}
			next[item.Index] = rgb.Color{Red: float64(uint8(value >> 16)), Green: float64(uint8(value >> 8)), Blue: float64(uint8(value)), Hex: item.ColorHex}
		}
		if len(next) != int(channel.LedChannels) {
			return fmt.Errorf("indexed lighting colors are invalid")
		}
		d.deviceLock.Lock()
		if d.DeviceProfile.RGBPerLed == nil {
			d.DeviceProfile.RGBPerLed = make(map[int]map[int]map[int]rgb.Color)
		}
		if d.DeviceProfile.RGBPerLed[channel.ChannelId] == nil {
			d.DeviceProfile.RGBPerLed[channel.ChannelId] = make(map[int]map[int]rgb.Color)
		}
		d.DeviceProfile.RGBPerLed[channel.ChannelId][0] = next
		d.deviceLock.Unlock()
		d.saveDeviceProfile()
		d.restartCanonicalChannelLighting()
		return nil
	}
	return fmt.Errorf("indexed lighting target is unavailable")
}

func (d *Device) restartCanonicalChannelLighting() {
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

// SetLightingBrightness preserves the controller-wide existing brightness
// authority. Channel targets deliberately do not duplicate Brightness state.
func (d *Device) SetLightingBrightness(brightness uint8) error {
	if d == nil || d.ChangeDeviceBrightnessValue(brightness) != 1 {
		return fmt.Errorf("unable to set controller brightness")
	}
	return nil
}

func (d *Device) ResolveLightingEffectSettings(effect string) (lightingsettings.EffectSettings, error) {
	return lightingsettings.EffectSettings{}, fmt.Errorf("a lighting channel target is required")
}

func (d *Device) ResolveLightingChannelEffectSettings(targetID, effect string) (lightingsettings.EffectSettings, error) {
	if d == nil || !d.SupportsLightingEffect(effect) || d.channelLightingResolver == nil || !d.canonicalChannelTargetExists(targetID) {
		return lightingsettings.EffectSettings{}, fmt.Errorf("channel effect settings are unavailable")
	}
	resolution, err := d.channelLightingResolver.Resolve(lightingsettings.IndependentDevice(targetID), effect)
	if err != nil {
		return lightingsettings.EffectSettings{}, err
	}
	return resolution.Settings.Clone(), nil
}
func (d *Device) SetLightingEffectSettings(effect string, settings lightingsettings.EffectSettings) error {
	return fmt.Errorf("a lighting channel target is required")
}

func (d *Device) SetLightingChannelEffectSettings(targetID, effect string, settings lightingsettings.EffectSettings) error {
	if d == nil || d.DeviceProfile == nil || d.channelLightingEffects == nil || !d.SupportsLightingEffect(effect) || settings.EffectID != effect || !d.canonicalChannelTargetExists(targetID) {
		return fmt.Errorf("channel effect settings are unavailable")
	}
	if err := lightingsettings.Validate(settings); err != nil {
		return err
	}
	if err := d.channelLightingEffects.Set(targetID, effect, settings.Clone()); err != nil {
		return err
	}
	if !d.DeviceProfile.RGBCluster && !d.DeviceProfile.OpenRGBIntegration && d.channelTargetSelectsEffect(targetID, effect) {
		d.restartCanonicalChannelLighting()
	}
	return nil
}
func (d *Device) ResetLightingEffectSettings(effect string) error {
	return fmt.Errorf("a lighting channel target is required")
}

func (d *Device) ResetLightingChannelEffectSettings(targetID, effect string) error {
	if d == nil || d.DeviceProfile == nil || d.channelLightingEffects == nil || !d.SupportsLightingEffect(effect) || !d.canonicalChannelTargetExists(targetID) {
		return fmt.Errorf("channel effect settings are unavailable")
	}
	deleted, err := d.channelLightingEffects.Delete(targetID, effect)
	if err != nil {
		return err
	}
	if deleted && !d.DeviceProfile.RGBCluster && !d.DeviceProfile.OpenRGBIntegration && d.channelTargetSelectsEffect(targetID, effect) {
		d.restartCanonicalChannelLighting()
	}
	return nil
}

func (d *Device) canonicalChannelTargetExists(targetID string) bool {
	for _, channel := range d.Devices {
		if d.canonicalChannel(channel) && d.canonicalChannelTargetID(channel.ChannelId) == targetID {
			return true
		}
	}
	return false
}

func (d *Device) channelTargetSelectsEffect(targetID, effect string) bool {
	for _, channel := range d.Devices {
		if d.canonicalChannel(channel) && d.canonicalChannelTargetID(channel.ChannelId) == targetID && channel.RGB == effect {
			return true
		}
	}
	return false
}
