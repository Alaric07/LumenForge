package openrgbimport

import (
	"fmt"
	"slices"

	"LumenForge/src/rgb"
)

// LightingEffectOption is a presentation-safe copy of one effect supported by
// an imported controller.
type LightingEffectOption struct {
	ID    string
	Label string
}

// LightingSnapshot is an immutable presentation/configuration view of an
// imported controller's lighting state. It does not confirm live hardware
// output.
type LightingSnapshot struct {
	ConfiguredEffect  string
	EffectSupported   bool
	SupportedEffects  []LightingEffectOption
	HasBrightness     bool
	Brightness        uint8
	HasSpeed          bool
	Speed             float64
	ClusterControlled bool
	PaletteKind       string
	SingleColorHex    string
	Customized        bool
}

// LightingSnapshot returns a complete race-safe value snapshot. Selected
// effect, Brightness, and effect settings come from the cut-over target state
// and canonical resolver.
func (d *Device) LightingSnapshot() (LightingSnapshot, bool) {
	if d == nil {
		return LightingSnapshot{}, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.IsOpenRGB || d.lifecycleInactiveLocked() {
		return LightingSnapshot{}, false
	}

	snapshot := LightingSnapshot{
		SupportedEffects: make([]LightingEffectOption, 0, len(d.RGBModes)),
	}
	for _, effect := range d.RGBModes {
		option := LightingEffectOption{ID: effect}
		if descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect); ok {
			option.Label = descriptor.Label
		}
		snapshot.SupportedEffects = append(snapshot.SupportedEffects, option)
	}

	profile := d.DeviceProfile
	snapshot.ConfiguredEffect = d.effect
	if snapshot.ConfiguredEffect == "" {
		snapshot.ConfiguredEffect = defaultDeviceLightingEffect
	}
	snapshot.EffectSupported = slices.Contains(d.RGBModes, snapshot.ConfiguredEffect)
	snapshot.HasBrightness = true
	snapshot.Brightness = d.brightness
	if profile != nil {
		snapshot.ClusterControlled = profile.RGBCluster
	}

	descriptor, known := rgb.SoftwareEffectDescriptorByID(snapshot.ConfiguredEffect)
	if !snapshot.EffectSupported || !known || d.lightingResolver == nil {
		return snapshot, true
	}
	resolution, err := d.resolveLightingSettings(snapshot.ConfiguredEffect)
	if err != nil {
		return snapshot, true
	}

	snapshot.PaletteKind = string(descriptor.PaletteKind)
	snapshot.Customized = resolution.Customized
	if descriptor.SupportsSpeed && resolution.Settings.Speed != nil {
		snapshot.HasSpeed = true
		snapshot.Speed = *resolution.Settings.Speed
	}
	if descriptor.PaletteKind == rgb.LightingPaletteStaticSingle && resolution.Settings.SingleColor != nil {
		snapshot.SingleColorHex = fmt.Sprintf("#%02x%02x%02x",
			uint8(resolution.Settings.SingleColor.Color.Red),
			uint8(resolution.Settings.SingleColor.Color.Green),
			uint8(resolution.Settings.SingleColor.Color.Blue),
		)
	}

	return snapshot, true
}
