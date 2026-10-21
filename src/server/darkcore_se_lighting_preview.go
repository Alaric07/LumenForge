package server

import "LumenForge/src/lightingpresentation"

// Presentation fixture only: Connected is assumed for the workspace; no device,
// receiver, canonical runtime, persistence or output worker is constructed.
func buildDarkCoreSEWirelessModernPreview() *devicesWorkspaceSummary {
	s := buildWirelessMouseModernPreviewWithCapabilities("DARK CORE SE", "preview-dark-core-rgb-se-wireless-modern", false, false, true, true)
	s.LegacyLighting = false
	s.Lighting = devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{
		TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, EffectSelectionAvailable: true, HasBrightness: true, Brightness: 100,
		SupportedEffects: []lightingpresentation.EffectOption{{ID: "mouse", Label: "Mouse"}, {ID: "static", Label: "Static"}},
		AuthoredZoneEditor: &lightingpresentation.AuthoredZoneEditor{EffectID: "mouse", Heading: "Zones", Description: "Choose colors for the selected zones.", Zones: []lightingpresentation.AuthoredZone{
			{ID: "0", Label: "Logo", ColorHex: "#ffff00"},
			{ID: "1", Label: "Scroll", ColorHex: "#00ffff"},
			{ID: "2", Label: "Side", ColorHex: "#00ffff"},
		}},
	})
	s.Lighting.ClusterOwnershipAvailable = false
	s.Lighting.ExternalOwnershipAvailable = false
	return s
}
