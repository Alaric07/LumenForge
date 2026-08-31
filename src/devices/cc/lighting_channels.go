package cc

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"

	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

func (d *Device) LightingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// canonicalChannel uses the hardware ChannelId rather than the RgbDevices map
// key, which is an insertion position and is not a persistent identity.
func (d *Device) canonicalChannel(channel *Devices) bool {
	return d != nil && d.channelLightingState != nil && channel != nil && channel.ChannelId >= 0 && channel.ChannelId < 7 && channel.LedChannels > 0
}

func (d *Device) canonicalChannelTargetID(channelID int) string {
	if d == nil || d.Serial == "" || channelID < 0 || channelID >= 7 {
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
	d.channelLightingState, d.channelLightingEffects, d.channelLightingResolver = runtime.State, runtime.Effects, runtime.Resolver
	return d.hydrateCanonicalChannels()
}

func (d *Device) hasPump() bool {
	for _, device := range d.Devices {
		if device != nil && device.ContainsPump {
			return true
		}
	}
	return false
}

func (d *Device) SupportsLightingEffect(effect string) bool {
	if d == nil || d.GetRgbProfile(effect) == nil || effect == "led" || effect == "keyboard" {
		return false
	}
	if effect == "liquid-temperature" && !d.hasPump() {
		return false
	}
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
	if err != nil || !d.SupportsLightingEffect(state.SelectedEffect) {
		return "", fmt.Errorf("canonical channel effect is unsupported")
	}
	return state.SelectedEffect, nil
}

func (d *Device) setChannelSelectedEffect(channelID int, effect string) error {
	if d == nil || d.channelLightingState == nil || !d.SupportsLightingEffect(effect) {
		return fmt.Errorf("unsupported lighting effect %q", effect)
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

// hydrateCanonicalChannels seeds missing canonical state from the existing full
// profile/render state, then makes canonical state authoritative at runtime.
func (d *Device) hydrateCanonicalChannels() error {
	if d == nil || d.channelLightingState == nil {
		return fmt.Errorf("canonical channel lighting state is unavailable")
	}
	for _, channel := range d.RgbDevices {
		if !d.canonicalChannel(channel) {
			continue
		}
		targetID := d.canonicalChannelTargetID(channel.ChannelId)
		state, found, err := d.channelLightingState.Resolve(targetID)
		if err != nil {
			return err
		}
		if !found {
			effect := channel.RGB
			if !d.SupportsLightingEffect(effect) {
				effect = lightingsettings.DefaultIndependentDeviceEffect
			}
			state.SelectedEffect = effect
			if err = d.channelLightingState.Set(targetID, state); err != nil {
				return err
			}
		}
		if !d.SupportsLightingEffect(state.SelectedEffect) {
			if err = lightingsettings.ValidateIndependentDeviceLightingState(state); err != nil {
				return fmt.Errorf("canonical channel effect %q is invalid: %w", state.SelectedEffect, err)
			}
			state.SelectedEffect = lightingsettings.DefaultIndependentDeviceEffect
			if !d.SupportsLightingEffect(state.SelectedEffect) {
				return fmt.Errorf("canonical fallback effect is unsupported")
			}
			if err = d.channelLightingState.Set(targetID, state); err != nil {
				return err
			}
		}
		channel.RGB = state.SelectedEffect
	}
	return nil
}

func (d *Device) snapshotCanonicalChannelEffects() {
	if d == nil || d.DeviceProfile == nil || d.channelLightingState == nil {
		return
	}
	if d.DeviceProfile.RGBProfiles == nil {
		d.DeviceProfile.RGBProfiles = make(map[int]string)
	}
	for _, channel := range d.RgbDevices {
		if !d.canonicalChannel(channel) {
			continue
		}
		if effect, err := d.channelSelectedEffect(channel.ChannelId); err == nil {
			channel.RGB, d.DeviceProfile.RGBProfiles[channel.ChannelId] = effect, effect
		}
	}
}

func (d *Device) restoreCanonicalChannelProfile(profile *DeviceProfile) error {
	selections, err := d.canonicalChannelProfileSelections(profile)
	if err != nil {
		return err
	}
	if err = d.applyCanonicalChannelSelections(selections); err != nil {
		return err
	}
	return d.hydrateCanonicalChannels()
}

type canonicalChannelSelection struct {
	id     int
	effect string
}

func (d *Device) canonicalChannelProfileSelections(profile *DeviceProfile) ([]canonicalChannelSelection, error) {
	if d == nil || profile == nil || d.channelLightingState == nil {
		return nil, fmt.Errorf("canonical channel lighting state is unavailable")
	}
	selections := make([]canonicalChannelSelection, 0, len(d.RgbDevices))
	for _, channel := range d.RgbDevices {
		if !d.canonicalChannel(channel) {
			continue
		}
		effect, ok := profile.RGBProfiles[channel.ChannelId]
		if !ok || !d.SupportsLightingEffect(effect) {
			return nil, fmt.Errorf("saved lighting effect for channel %d is invalid", channel.ChannelId)
		}
		selections = append(selections, canonicalChannelSelection{channel.ChannelId, effect})
	}
	return selections, nil
}

func (d *Device) applyCanonicalChannelSelections(selections []canonicalChannelSelection) error {
	if d == nil || d.channelLightingState == nil {
		return fmt.Errorf("canonical channel lighting state is unavailable")
	}
	previous := make([]lightingsettings.IndependentDeviceLightingState, len(selections))
	for index, selection := range selections {
		state, _, err := d.channelLightingState.Resolve(d.canonicalChannelTargetID(selection.id))
		if err != nil {
			return err
		}
		previous[index] = state
	}
	for _, selection := range selections {
		if err := d.setChannelSelectedEffect(selection.id, selection.effect); err != nil {
			for index := range previous {
				_ = d.channelLightingState.Set(d.canonicalChannelTargetID(selections[index].id), previous[index])
			}
			return err
		}
	}
	return nil
}

// hydrateCanonicalChannelSelections is intentionally infallible after a
// successful preflight/apply: each selection was derived from an existing
// physical channel and already validated before the profile transition.
func (d *Device) hydrateCanonicalChannelSelections(selections []canonicalChannelSelection) {
	for _, selection := range selections {
		for _, channel := range d.RgbDevices {
			if d.canonicalChannel(channel) && channel.ChannelId == selection.id {
				channel.RGB = selection.effect
				break
			}
		}
	}
}

func (d *Device) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	if d == nil || d.channelLightingState == nil || d.DeviceProfile == nil {
		return lightingpresentation.Snapshot{}, false
	}
	snapshot := lightingpresentation.Snapshot{TargetKind: "native", ClusterControlled: d.DeviceProfile.RGBCluster, ExternalControlled: d.DeviceProfile.OpenRGBIntegration, Channels: make([]lightingpresentation.Channel, 0, len(d.RgbDevices))}
	if d.DeviceProfile.BrightnessSlider != nil {
		snapshot.HasBrightness, snapshot.Brightness = true, *d.DeviceProfile.BrightnessSlider
	}
	for _, channel := range d.RgbDevices {
		if !d.canonicalChannel(channel) {
			continue
		}
		effect, err := d.channelSelectedEffect(channel.ChannelId)
		if err != nil {
			return lightingpresentation.Snapshot{}, false
		}
		child := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: effect, EffectSupported: true, ClusterControlled: d.DeviceProfile.RGBCluster, ExternalControlled: d.DeviceProfile.OpenRGBIntegration}
		for _, candidate := range rgbModes {
			if d.SupportsLightingEffect(candidate) {
				child.SupportedEffects = append(child.SupportedEffects, lightingpresentation.EffectOption{ID: candidate, Label: candidate})
			}
		}
		if err := d.populateCanonicalChannelSnapshot(&child, channel.ChannelId, effect); err != nil {
			return lightingpresentation.Snapshot{}, false
		}
		snapshot.Channels = append(snapshot.Channels, lightingpresentation.Channel{TargetID: d.canonicalChannelTargetID(channel.ChannelId), ChannelID: strconv.Itoa(channel.ChannelId), Name: channel.Name, Label: channel.Label, LEDCount: int(channel.LedChannels), ContainsPump: channel.ContainsPump, Lighting: child})
	}
	snapshot.ManualRGBPorts = d.manualRGBPortSnapshot()
	sort.Slice(snapshot.Channels, func(i, j int) bool { return snapshot.Channels[i].ChannelID < snapshot.Channels[j].ChannelID })
	return snapshot, len(snapshot.Channels) > 0
}

func ccLightingColorHex(color lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
}

func (d *Device) populateCanonicalChannelSnapshot(snapshot *lightingpresentation.Snapshot, channelID int, effect string) error {
	if snapshot == nil || d.channelLightingResolver == nil {
		return nil
	}
	descriptor, generic := rgb.SoftwareEffectDescriptorByID(effect)
	if !generic {
		return nil
	}
	resolution, err := d.channelLightingResolver.Resolve(lightingsettings.IndependentDevice(d.canonicalChannelTargetID(channelID)), effect)
	if err != nil || resolution.Settings.EffectID != effect {
		return fmt.Errorf("resolve canonical channel effect settings: %w", err)
	}
	settings := resolution.Settings
	snapshot.Customized, snapshot.PaletteKind = resolution.Customized, string(descriptor.PaletteKind)
	if descriptor.SupportsSpeed && settings.Speed != nil {
		snapshot.HasSpeed, snapshot.Speed = true, *settings.Speed
	}
	switch descriptor.PaletteKind {
	case rgb.LightingPaletteStaticSingle:
		if settings.SingleColor != nil {
			snapshot.SingleColorHex = ccLightingColorHex(settings.SingleColor.Color)
		}
	case rgb.LightingPaletteTwoColor:
		if settings.TwoColor != nil {
			snapshot.TwoColorStartHex, snapshot.TwoColorEndHex = ccLightingColorHex(settings.TwoColor.Start), ccLightingColorHex(settings.TwoColor.End)
		}
	case rgb.LightingPaletteTemperatureThree:
		if settings.Temperature != nil {
			snapshot.HasTemperature = true
			snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: ccLightingColorHex(settings.Temperature.Low.Color), Celsius: settings.Temperature.Low.Celsius}
			snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: ccLightingColorHex(settings.Temperature.Middle.Color), Celsius: settings.Temperature.Middle.Celsius}
			snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: ccLightingColorHex(settings.Temperature.High.Color), Celsius: settings.Temperature.High.Celsius}
		}
	case rgb.LightingPaletteGradient:
		if settings.Gradient != nil {
			snapshot.HasGradient, snapshot.GradientStops = true, make([]lightingpresentation.GradientStop, len(settings.Gradient.Stops))
			for index, stop := range settings.Gradient.Stops {
				snapshot.GradientStops[index] = lightingpresentation.GradientStop{Position: stop.Position, ColorHex: ccLightingColorHex(stop.Color), Intensity: stop.Intensity}
			}
		}
	}
	return nil
}

func (d *Device) manualRGBPortSnapshot() []lightingpresentation.ManualRGBPort {
	if d == nil || d.DeviceProfile == nil || len(d.FreeLedPorts) == 0 {
		return nil
	}
	ports := make([]int, 0, len(d.FreeLedPorts))
	for port := range d.FreeLedPorts {
		if port >= 1 && port <= 6 {
			ports = append(ports, port)
		}
	}
	sort.Ints(ports)
	result := make([]lightingpresentation.ManualRGBPort, 0, len(ports))
	for _, port := range ports {
		selected := d.DeviceProfile.CustomLEDs[port]
		item := lightingpresentation.ManualRGBPort{PortID: port, Name: d.FreeLedPorts[port], Selected: selected, Options: make([]lightingpresentation.ManualRGBDeviceOption, 0, len(d.ExternalLedDevice))}
		for _, option := range d.ExternalLedDevice {
			item.Options = append(item.Options, lightingpresentation.ManualRGBDeviceOption{ID: strconv.Itoa(option.Index), Label: option.Name, Selected: option.Index == selected})
		}
		if len(item.Options) > 0 {
			result = append(result, item)
		}
	}
	return result
}

func (d *Device) SetLightingEffect(string) error {
	return fmt.Errorf("a lighting channel target is required")
}

func (d *Device) SetLightingChannelEffect(targetID, effect string) error {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.RGBCluster || d.DeviceProfile.OpenRGBIntegration {
		return fmt.Errorf("lighting is externally owned")
	}
	for _, channel := range d.RgbDevices {
		if !d.canonicalChannel(channel) || d.canonicalChannelTargetID(channel.ChannelId) != targetID {
			continue
		}
		if err := d.setChannelSelectedEffect(channel.ChannelId, effect); err != nil {
			return err
		}
		channel.RGB = effect
		d.snapshotCanonicalChannelEffects()
		d.saveDeviceProfile()
		d.restartCanonicalChannelLighting()
		return nil
	}
	return fmt.Errorf("lighting channel is unavailable")
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
	for _, channel := range d.RgbDevices {
		if d.canonicalChannel(channel) && d.canonicalChannelTargetID(channel.ChannelId) == targetID {
			return true
		}
	}
	return false
}

func (d *Device) channelTargetSelectsEffect(targetID, effect string) bool {
	for _, channel := range d.RgbDevices {
		if d.canonicalChannel(channel) && d.canonicalChannelTargetID(channel.ChannelId) == targetID && channel.RGB == effect {
			return true
		}
	}
	return false
}
