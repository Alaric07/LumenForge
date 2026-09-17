package hydro

import (
	"fmt"

	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
)

func (d *Device) LightingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) SupportsLightingEffect(effect string) bool { return effect == "static" }

func hydroLightingColorHex(color lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", uint8(color.Red), uint8(color.Green), uint8(color.Blue))
}

func (d *Device) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	state, err := d.canonicalLightingState()
	if err != nil {
		return lightingpresentation.Snapshot{}, false
	}
	resolution, err := d.resolveCanonicalStatic()
	if err != nil || resolution.Settings.EffectID != "static" || resolution.Settings.SingleColor == nil {
		return lightingpresentation.Snapshot{}, false
	}
	return lightingpresentation.Snapshot{
		TargetKind: "native", ConfiguredEffect: "static", EffectSupported: true, FixedEffect: true,
		HasBrightness: true, Brightness: state.Brightness,
		SupportedEffects: []lightingpresentation.EffectOption{{ID: "static", Label: "Static"}},
		PaletteKind:      "static-single-color", SingleColorHex: hydroLightingColorHex(resolution.Settings.SingleColor.Color), Customized: resolution.Customized,
	}, true
}
