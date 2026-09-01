package ccxt

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
	return d != nil && channel != nil && !channel.ExternalLed && channel.ChannelId >= 0 && channel.ChannelId < 6 && channel.LedChannels > 0
}

func (d *Device) canonicalChannelTargetID(channelID int) string {
	if d == nil || d.Serial == "" || channelID < 0 || channelID >= 6 {
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
	return d.hydrateCanonicalChannels()
}

func (d *Device) SupportsLightingEffect(effect string) bool {
	if d == nil || d.GetRgbProfile(effect) == nil || effect == "keyboard" {
		return false
	}
	// The selected effect is stored through the shared independent-target state
	// store, so do not advertise a legacy CCXT RGB profile it cannot persist.
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
	for _, channel := range d.RgbDevices {
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
	selections := make([]selection, 0, len(d.RgbDevices))
	for _, channel := range d.RgbDevices {
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
		Channels:           make([]lightingpresentation.Channel, 0, len(d.RgbDevices)),
	}
	snapshot.ThreePinPort = d.threePinPortSnapshot()
	if d.DeviceProfile.BrightnessSlider != nil {
		snapshot.HasBrightness = true
		snapshot.Brightness = *d.DeviceProfile.BrightnessSlider
	}
	for _, channel := range d.RgbDevices {
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
		channelSnapshot := lightingpresentation.Channel{TargetID: d.canonicalChannelTargetID(channel.ChannelId), ChannelID: strconv.Itoa(channel.ChannelId), Name: channel.Name, Label: channel.Label, LEDCount: int(channel.LedChannels), Lighting: child}
		if effect == "probe-temperature" {
			probe := &lightingpresentation.ProbeTemperature{ChannelID: strconv.Itoa(channel.ChannelId), ProbeID: channel.ProbeId, Minimum: channel.MinTemp, Maximum: channel.MaxTemp}
			for _, source := range d.Devices {
				if source != nil && source.IsTemperatureProbe {
					probe.Sources = append(probe.Sources, lightingpresentation.ProbeTemperatureSource{ID: source.ChannelId, Label: fmt.Sprintf("%s - %s", source.Name, source.Label), Selected: source.ChannelId == channel.ProbeId})
				}
			}
			sort.Slice(probe.Sources, func(i, j int) bool { return probe.Sources[i].ID < probe.Sources[j].ID })
			channelSnapshot.ProbeTemperature = probe
		}
		snapshot.Channels = append(snapshot.Channels, channelSnapshot)
	}
	sort.Slice(snapshot.Channels, func(i, j int) bool { return snapshot.Channels[i].ChannelID < snapshot.Channels[j].ChannelID })
	if len(snapshot.Channels) > 0 {
		bulk := &lightingpresentation.BulkEffectControl{SupportedEffects: make([]lightingpresentation.EffectOption, 0, len(rgbModes))}
		for _, candidate := range rgbModes {
			if d.SupportsLightingEffect(candidate) {
				bulk.SupportedEffects = append(bulk.SupportedEffects, lightingpresentation.EffectOption{ID: candidate, Label: candidate})
			}
		}
		bulk.ConfiguredEffect = snapshot.Channels[0].Lighting.ConfiguredEffect
		for _, channel := range snapshot.Channels[1:] {
			if channel.Lighting.ConfiguredEffect != bulk.ConfiguredEffect {
				bulk.ConfiguredEffect, bulk.Mixed = "", true
				break
			}
		}
		snapshot.BulkEffectControl = bulk
	}
	return snapshot, len(snapshot.Channels) > 0
}

func (d *Device) threePinPortSnapshot() *lightingpresentation.ThreePinPort {
	if d == nil || d.DeviceProfile == nil || len(d.ExternalLedDevice) == 0 {
		return nil
	}
	port := &lightingpresentation.ThreePinPort{DeviceType: d.DeviceProfile.ExternalHubDeviceType, Quantity: d.DeviceProfile.ExternalHubDeviceAmount}
	options := append([]ExternalLedDevice(nil), d.ExternalLedDevice...)
	sort.Slice(options, func(i, j int) bool { return options[i].Index < options[j].Index })
	for _, option := range options {
		port.DeviceOptions = append(port.DeviceOptions, lightingpresentation.ThreePinDeviceOption{ID: strconv.Itoa(option.Index), Label: option.Name, Selected: option.Index == port.DeviceType})
	}
	for _, amount := range d.threePinPortAmounts(port.DeviceType) {
		label := fmt.Sprintf("%d Devices", amount)
		if amount == 0 {
			label = "No Devices"
		} else if amount == 1 {
			label = "1 Device"
		}
		port.QuantityOptions = append(port.QuantityOptions, lightingpresentation.ThreePinQuantityOption{Value: strconv.Itoa(amount), Label: label, Selected: amount == port.Quantity})
	}
	port.QuantityDisabled = port.DeviceType == 0 || len(port.QuantityOptions) <= 1
	return port
}

func (d *Device) threePinPortAmounts(deviceType int) []int {
	if deviceType == 0 {
		return []int{0}
	}
	metadata := d.getExternalLedDevice(deviceType)
	if metadata == nil {
		return nil
	}
	if metadata.Kit {
		return []int{1}
	}
	maximum := 6
	switch deviceType {
	case 7, 8, 9, 15:
		maximum = 1
	case 10, 11, 12:
		maximum = 2
	}
	amounts := make([]int, maximum+1)
	for amount := range amounts {
		amounts[amount] = amount
	}
	return amounts
}

func ccxtLightingColorHex(color lightingsettings.Color) string {
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
			snapshot.SingleColorHex = ccxtLightingColorHex(settings.SingleColor.Color)
		}
	case rgb.LightingPaletteTwoColor:
		if settings.TwoColor != nil {
			snapshot.TwoColorStartHex, snapshot.TwoColorEndHex = ccxtLightingColorHex(settings.TwoColor.Start), ccxtLightingColorHex(settings.TwoColor.End)
		}
	case rgb.LightingPaletteTemperatureThree:
		if settings.Temperature != nil {
			snapshot.HasTemperature = true
			snapshot.TemperatureLow = lightingpresentation.TemperaturePoint{ColorHex: ccxtLightingColorHex(settings.Temperature.Low.Color), Celsius: settings.Temperature.Low.Celsius}
			snapshot.TemperatureMiddle = lightingpresentation.TemperaturePoint{ColorHex: ccxtLightingColorHex(settings.Temperature.Middle.Color), Celsius: settings.Temperature.Middle.Celsius}
			snapshot.TemperatureHigh = lightingpresentation.TemperaturePoint{ColorHex: ccxtLightingColorHex(settings.Temperature.High.Color), Celsius: settings.Temperature.High.Celsius}
		}
	case rgb.LightingPaletteGradient:
		if settings.Gradient != nil {
			snapshot.HasGradient = true
			snapshot.GradientStops = make([]lightingpresentation.GradientStop, len(settings.Gradient.Stops))
			for index, stop := range settings.Gradient.Stops {
				snapshot.GradientStops[index] = lightingpresentation.GradientStop{Position: stop.Position, ColorHex: ccxtLightingColorHex(stop.Color), Intensity: stop.Intensity}
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
	for _, channel := range d.RgbDevices {
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

// SetLightingAllChannelEffects applies one real effect to every canonical RGB
// port. It remains a convenience mutation over child state.
func (d *Device) SetLightingAllChannelEffects(effect string) error {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.RGBCluster || d.DeviceProfile.OpenRGBIntegration {
		return fmt.Errorf("lighting is externally owned")
	}
	if !d.SupportsLightingEffect(effect) {
		return fmt.Errorf("unsupported bulk lighting effect %q", effect)
	}
	channels := make([]*Devices, 0, len(d.RgbDevices))
	for _, channel := range d.RgbDevices {
		if d.canonicalChannel(channel) {
			if _, err := d.channelSelectedEffect(channel.ChannelId); err != nil {
				return err
			}
			channels = append(channels, channel)
		}
	}
	if len(channels) == 0 {
		return fmt.Errorf("lighting channels are unavailable")
	}
	for _, channel := range channels {
		if err := d.setChannelSelectedEffect(channel.ChannelId, effect); err != nil {
			return err
		}
	}
	if d.DeviceProfile.RGBProfiles == nil {
		d.DeviceProfile.RGBProfiles = make(map[int]string)
	}
	for _, channel := range channels {
		channel.RGB = effect
		d.DeviceProfile.RGBProfiles[channel.ChannelId] = effect
	}
	d.saveDeviceProfile()
	d.restartCanonicalChannelLighting()
	return nil
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
