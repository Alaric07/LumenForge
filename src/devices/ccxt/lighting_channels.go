package ccxt

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"

	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
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
		snapshot.Channels = append(snapshot.Channels, lightingpresentation.Channel{TargetID: d.canonicalChannelTargetID(channel.ChannelId), ChannelID: strconv.Itoa(channel.ChannelId), Name: channel.Name, Label: channel.Label, LEDCount: int(channel.LedChannels), Lighting: child})
	}
	sort.Slice(snapshot.Channels, func(i, j int) bool { return snapshot.Channels[i].ChannelID < snapshot.Channels[j].ChannelID })
	return snapshot, len(snapshot.Channels) > 0
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

// The single-target settings surface is intentionally unavailable for this
// milestone: RGB profile settings are currently controller-owned, while this
// change establishes only per-channel selected-effect authority.
func (d *Device) ResolveLightingEffectSettings(string) (lightingsettings.EffectSettings, error) {
	return lightingsettings.EffectSettings{}, fmt.Errorf("channel effect settings are unavailable")
}
func (d *Device) SetLightingEffectSettings(string, lightingsettings.EffectSettings) error {
	return fmt.Errorf("channel effect settings are unavailable")
}
func (d *Device) ResetLightingEffectSettings(string) error {
	return fmt.Errorf("channel effect settings are unavailable")
}
