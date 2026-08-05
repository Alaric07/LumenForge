package openrgbimport

import (
	"LumenForge/src/rgb"
	"slices"
)

// LightingEffectOption is a presentation-safe copy of one effect supported by
// an imported controller.
type LightingEffectOption struct {
	ID              string
	Label           string
	CapabilityKnown bool
	Capability      rgb.LightingEffectCapability
}

// LightingDefinitionSnapshot contains only the persisted inputs consumed by a
// confirmed renderer. Presence flags keep black colors and zero speed distinct
// from values that are not meaningful for an effect.
type LightingDefinitionSnapshot struct {
	Palette        rgb.LightingPaletteKind
	HasStartColor  bool
	StartColor     rgb.Color
	HasMiddleColor bool
	MiddleColor    rgb.Color
	HasEndColor    bool
	EndColor       rgb.Color
	HasSpeed       bool
	Speed          float64
}

// LightingOverrideSnapshot is an independent value copy of the configured
// controller-wide override. A nil pointer in LightingSnapshot means no
// override has been configured; Enabled distinguishes a disabled override.
type LightingOverrideSnapshot struct {
	Enabled     bool
	StartColor  rgb.Color
	MiddleColor rgb.Color
	EndColor    rgb.Color
	Speed       float64
}

// LightingSnapshot is an immutable presentation/configuration view of an
// imported controller's lighting state. It does not confirm live hardware
// output.
type LightingSnapshot struct {
	HasActiveProfile  bool
	ConfiguredEffect  string
	EffectSupported   bool
	SupportedEffects  []LightingEffectOption
	HasBrightness     bool
	Brightness        uint8
	ClusterControlled bool
	BaseDefinition    *LightingDefinitionSnapshot
	Override          *LightingOverrideSnapshot
	Effective         *LightingDefinitionSnapshot
}

// LightingSnapshot returns a complete race-safe value snapshot. Selected
// effect, Brightness, and effect settings come from the cut-over target state
// and canonical resolver; legacy profile fields are presentation-only here.
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
		option.Capability, option.CapabilityKnown = rgb.LightingEffectCapabilities(effect)
		snapshot.SupportedEffects = append(snapshot.SupportedEffects, option)
	}

	profile := d.DeviceProfile
	snapshot.HasActiveProfile = profile != nil && profile.Active
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
	if profile != nil && profile.RGBOverride != nil {
		snapshot.Override = &LightingOverrideSnapshot{
			Enabled:     profile.RGBOverride.Enabled,
			StartColor:  profile.RGBOverride.RGBStartColor,
			MiddleColor: profile.RGBOverride.RGBMiddleColor,
			EndColor:    profile.RGBOverride.RGBEndColor,
			Speed:       profile.RGBOverride.RgbModeSpeed,
		}
	}

	capability, known := rgb.LightingEffectCapabilities(snapshot.ConfiguredEffect)
	if !snapshot.EffectSupported || !known || d.lightingResolver == nil {
		return snapshot, true
	}
	resolution, err := d.resolveLightingSettings(snapshot.ConfiguredEffect)
	if err != nil {
		return snapshot, true
	}

	definition := rgbProfileFromLightingSettings(resolution.Settings)
	base := lightingDefinitionSnapshot(definition, capability)
	snapshot.BaseDefinition = &base
	// Base/Override/Effective remain temporarily for response compatibility.
	// Runtime precedence has been removed: Effective reflects the authoritative
	// resolved settings while Override is retained only as legacy presentation.
	effective := base
	snapshot.Effective = &effective
	return snapshot, true
}

func lightingDefinitionSnapshot(definition rgb.Profile, capability rgb.LightingEffectCapability) LightingDefinitionSnapshot {
	snapshot := LightingDefinitionSnapshot{Palette: capability.Palette}
	if capability.UsesStartColor {
		snapshot.HasStartColor = true
		snapshot.StartColor = definition.StartColor
	}
	if capability.UsesMiddleColor {
		snapshot.HasMiddleColor = true
		snapshot.MiddleColor = definition.MiddleColor
	}
	if capability.UsesEndColor {
		snapshot.HasEndColor = true
		snapshot.EndColor = definition.EndColor
	}
	if capability.SupportsSpeed {
		snapshot.HasSpeed = true
		snapshot.Speed = definition.Speed
	}
	return snapshot
}

func applyLightingOverride(definition *LightingDefinitionSnapshot, override LightingOverrideSnapshot) {
	if definition.HasStartColor {
		definition.StartColor = override.StartColor
	}
	if definition.HasMiddleColor {
		definition.MiddleColor = override.MiddleColor
	}
	if definition.HasEndColor {
		definition.EndColor = override.EndColor
	}
	// Gradient passes its stored speed directly to the renderer, so the
	// controller-wide override does not replace it.
	if definition.HasSpeed && definition.Palette != rgb.LightingPaletteGradient {
		definition.Speed = override.Speed
	}
}
