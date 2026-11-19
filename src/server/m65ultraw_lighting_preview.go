package server

import (
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/rgb"
	"strconv"
)

// Plain fixture data only: no device Init, receiver, persistence, sessions,
// callbacks, workers, or input programming. Mouse is an explicit sample selection;
// runtime clean canonical state resolves to Static at 100%.
func buildM65RGBUltraWirelessModernPreview() *devicesWorkspaceSummary {
	s := buildM65RGBEliteInputPreview()
	s.Product, s.Serial, s.Firmware = "M65 RGB ULTRA", "preview-m65-rgb-ultra-wireless-modern", "n/a"
	s.HasBattery, s.BatteryLevel = true, 78
	s.SleepTimer = &devicesSleepTimerWorkspaceSummary{Value: 15, Options: []devicesSleepTimerOptionSummary{{Value: 1, Label: "1 minute"}, {Value: 5, Label: "5 minutes"}, {Value: 10, Label: "10 minutes"}, {Value: 15, Label: "15 minutes"}, {Value: 30, Label: "30 minutes"}, {Value: 60, Label: "1 hour"}}}
	s.Performance.PollingRate = nil // Polling belongs to the receiver, not this child.
	s.Performance.LiftHeight = &devicesPerformanceSelectSummary{Value: 2, Options: []devicesPerformanceOptionSummary{{Value: 2, Label: "Low"}, {Value: 3, Label: "Medium"}, {Value: 4, Label: "High"}, {Value: 6, Label: "Calibrated"}, {Value: 7, Label: "Ultra Low"}}}
	s.Performance.ButtonOptimization = &devicesPerformanceSelectSummary{Value: 0, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Disabled"}, {Value: 1, Label: "Enabled"}}}
	s.Performance.AngleSnapping = &devicesPerformanceToggleSummary{Enabled: false}
	s.DPI.MinimumDPI, s.DPI.MaximumDPI = 100, 26000
	values := []uint16{400, 800, 1200, 1600, 3000}
	colors := []string{"#ff0000", "#ffffff", "#00ff00", "#ff00ff", "#00ffff"}
	for i := range s.DPI.RegularStages {
		stage := &s.DPI.RegularStages[i]
		stage.DPI, stage.ColorHex = values[i], colors[i]
		stage.Active = stage.ID == "1"
	}
	s.DPI.ActiveRegularStageID = "1"
	s.DPI.SniperStage.DPI, s.DPI.SniperStage.ColorHex = 200, "#ffff00"
	s.OverviewPerformance.Rows[0].Value = strconv.Itoa(800)
	s.OverviewPerformance.Rows[1].Value = "Stage 2"
	rows := s.OverviewPerformance.Rows[:0]
	for _, row := range s.OverviewPerformance.Rows {
		if row.Label != "Polling Rate" {
			rows = append(rows, row)
		}
	}
	s.OverviewPerformance.Rows = rows
	// Match live W ButtonsSnapshot, including omission of type4.
	s.Buttons.AssignmentTypes = []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI +"}, {ID: 3, Label: "Keyboard"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}
	s.Buttons.Buttons = []devicesButtonsButtonSummary{{KeyIndex: 2048, Name: "Tilt Right", Default: true}, {KeyIndex: 1024, Name: "Tilt Left", Default: true}, {KeyIndex: 512, Name: "Tilt Back", Default: true}, {KeyIndex: 256, Name: "Tilt Front", Default: true}, {KeyIndex: 128, Name: "Sniper", ActionType: 8, PressAndHold: true}, {KeyIndex: 64, Name: "DPI -", Default: true}, {KeyIndex: 32, Name: "DPI +", Default: true}, {KeyIndex: 8, Name: "Forward Button", Default: true}, {KeyIndex: 16, Name: "Back Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 1, Name: "Left Button", Default: true}}
	snapshot := lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, EffectSelectionAvailable: true, HasBrightness: true, Brightness: 100}
	for _, id := range []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"} {
		label := "Mouse"
		if id != "mouse" {
			descriptor, _ := rgb.SoftwareEffectDescriptorByID(id)
			label = descriptor.Label
		}
		snapshot.SupportedEffects = append(snapshot.SupportedEffects, lightingpresentation.EffectOption{ID: id, Label: label})
	}
	snapshot.AuthoredZoneEditor = &lightingpresentation.AuthoredZoneEditor{EffectID: "mouse", Heading: "Zones", Description: "Choose a color for the Logo zone.", Zones: []lightingpresentation.AuthoredZone{{ID: "0", Label: "Logo", ColorHex: "#ffff00"}}}
	s.Lighting = devicesLightingWorkspaceSummaryFromSnapshot(snapshot)
	s.Lighting.ClusterOwnershipAvailable, s.Lighting.ExternalOwnershipAvailable = true, true
	s.LegacyLighting = false
	return s
}
