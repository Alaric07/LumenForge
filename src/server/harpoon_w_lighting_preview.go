package server

import (
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/rgb"
)

// Pure W fixture. No receiver, device initialization, stores, global registry,
// production dispatch, renderer or goroutines participate in construction.
func buildHarpoonWirelessModernPreview(disconnected bool) *devicesWorkspaceSummary {
	s := &devicesWorkspaceSummary{Product: "HARPOON RGB", Serial: "preview-harpoon-wireless-modern", Firmware: "n/a", Image: "icon-mouse.svg", View: "overview", Unavailable: disconnected, HasBattery: true, BatteryLevel: 78,
		DPI:               &devicesDPIWorkspaceSummary{MinimumDPI: 200, MaximumDPI: 10000, ActiveRegularStageID: "1", RegularStages: []devicesDPIStageSummary{{ID: "0", Name: "Stage 1", DPI: 400, ColorHex: "#ff0000"}, {ID: "1", Name: "Stage 2", DPI: 800, ColorHex: "#ffffff", Active: true}, {ID: "2", Name: "Stage 3", DPI: 1200, ColorHex: "#00ff00"}, {ID: "3", Name: "Stage 4", DPI: 1600, ColorHex: "#ff00ff"}, {ID: "4", Name: "Stage 5", DPI: 3200, ColorHex: "#00ffff"}}, SniperStage: &devicesDPIStageSummary{ID: "5", Name: "Sniper", DPI: 200, ColorHex: "#ffff00", Sniper: true}},
		Performance:       &devicesPerformanceWorkspaceSummary{ButtonOptimization: &devicesPerformanceSelectSummary{Value: 0, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Disabled"}, {Value: 1, Label: "Enabled"}}}, AngleSnapping: &devicesPerformanceToggleSummary{}},
		DeviceProfiles:    &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"default"}, ActiveProfile: "default", CanSwitch: !disconnected, CanSave: true, CanDelete: true, Scope: "device", Label: "Device Profile", Description: devicesGenericDeviceProfileDescription},
		OverviewTelemetry: []devicesOverviewStatusRow{{Label: "Connection", Value: "Receiver connected"}},
	}
	s.Buttons = &devicesButtonsWorkspaceSummary{Buttons: []devicesButtonsButtonSummary{{KeyIndex: 32, Name: "Forward Button", Default: true}, {KeyIndex: 16, Name: "Back Button", Default: true}, {KeyIndex: 8, Name: "DPI Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 1, Name: "Left Button", Default: true}}, AssignmentTypes: []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI"}, {ID: 3, Label: "Keyboard"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}}
	s.SleepTimer = &devicesSleepTimerWorkspaceSummary{Value: 15, Options: []devicesSleepTimerOptionSummary{{Value: 1, Label: "1 minute"}, {Value: 5, Label: "5 minutes"}, {Value: 10, Label: "10 minutes"}, {Value: 15, Label: "15 minutes"}, {Value: 30, Label: "30 minutes"}, {Value: 60, Label: "1 hour"}}}
	if disconnected {
		s.OverviewTelemetry[0].Value = "Receiver present; mouse disconnected"
	}
	snapshot := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, EffectSelectionAvailable: true, HasBrightness: true, Brightness: 70}
	for _, effect := range []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"} {
		label := "Mouse"
		if effect != "mouse" {
			descriptor, _ := rgb.SoftwareEffectDescriptorByID(effect)
			label = descriptor.Label
		}
		snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{ID: effect, Label: label})
	}
	snapshot.AuthoredZoneEditor = &lightingpresentation.AuthoredZoneEditor{EffectID: "mouse", Heading: "Zones", Description: "Choose a color for the Logo zone.", Zones: []lightingpresentation.AuthoredZone{{ID: "0", Label: "Logo", ColorHex: "#ffff00"}}}
	s.Lighting = devicesLightingWorkspaceSummaryFromSnapshot(snapshot)
	s.Lighting.ClusterOwnershipAvailable, s.Lighting.ExternalOwnershipAvailable = true, true
	// The fixture's authored mouse/70 selection is explicit, not a default import.
	return s
}
