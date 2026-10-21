package server

import (
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/rgb"
)

// Pure fixture data: no device construction, stores, registration or workers.
func addGlaiveCanonicalLightingPreview(s *devicesWorkspaceSummary) {
	snapshot := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, EffectSelectionAvailable: true, HasBrightness: true, Brightness: 70}
	for _, effect := range []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"} {
		label := "Mouse"
		if effect != "mouse" {
			descriptor, _ := rgb.SoftwareEffectDescriptorByID(effect)
			label = descriptor.Label
		}
		snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{ID: effect, Label: label})
	}
	snapshot.AuthoredZoneEditor = &lightingpresentation.AuthoredZoneEditor{EffectID: "mouse", Heading: "Zones", Description: "Choose colors for the selected zones.", Zones: []lightingpresentation.AuthoredZone{{ID: "0", Label: "Front", ColorHex: "#ff0000"}, {ID: "1", Label: "Logo", ColorHex: "#ffff00"}, {ID: "2", Label: "Side", ColorHex: "#00ffff"}}}
	s.Lighting = devicesLightingWorkspaceSummaryFromSnapshot(snapshot)
	s.Lighting.ClusterOwnershipAvailable = true
	s.Lighting.ExternalOwnershipAvailable = true
	s.LegacyLighting = false
}
