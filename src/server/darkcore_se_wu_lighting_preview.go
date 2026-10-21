package server

import "LumenForge/src/lightingpresentation"

// Pure connected direct-USB presentation fixture. No device, canonical store,
// HID, receiver, persistence, listener or renderer is constructed.
func buildDarkCoreSEUSBModernPreview() *devicesWorkspaceSummary {
	s := &devicesWorkspaceSummary{Product: "DARK CORE SE", Serial: "preview-dark-core-rgb-se-usb-modern", Image: "icon-mouse.svg", Unavailable: false}
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
