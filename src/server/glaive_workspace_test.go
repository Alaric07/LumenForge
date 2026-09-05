package server

import (
	"testing"

	"LumenForge/src/buttonspresentation"
	"LumenForge/src/common"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/dpipresentation"
	"LumenForge/src/performancepresentation"
	"LumenForge/src/stats"
)

type glaiveWorkspaceProvider struct{ serial string }

func (p glaiveWorkspaceProvider) DPIDeviceID() string { return p.serial }
func (p glaiveWorkspaceProvider) DPISnapshot() (dpipresentation.Snapshot, bool) {
	return dpipresentation.Snapshot{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "1", Stages: []dpipresentation.Stage{{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#00ff00"}, {ID: "1", Name: "Stage 2", DPI: 1500, ColorHex: "#00ff00"}, {ID: "3", Name: "Sniper", DPI: 400, ColorHex: "#ffff00", Sniper: true}}}, true
}
func (p glaiveWorkspaceProvider) ButtonsDeviceID() string { return p.serial }
func (p glaiveWorkspaceProvider) ButtonsSnapshot() (buttonspresentation.Snapshot, bool) {
	return buttonspresentation.Snapshot{Buttons: []buttonspresentation.Button{{KeyIndex: 1, Name: "Left"}}, AssignmentTypes: []buttonspresentation.AssignmentType{{ID: 0, Label: "None"}}}, true
}
func (p glaiveWorkspaceProvider) PerformanceDeviceID() string { return p.serial }
func (p glaiveWorkspaceProvider) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	return performancepresentation.Snapshot{PollingRate: &performancepresentation.SelectSetting{Value: 1, Options: []performancepresentation.Option{{Value: 1, Label: "1000 Hz"}}}, AngleSnapping: &performancepresentation.ToggleSetting{Enabled: true}}, true
}
func (p glaiveWorkspaceProvider) DeviceProfileDeviceID() string { return p.serial }
func (p glaiveWorkspaceProvider) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	return deviceprofilepresentation.Snapshot{Supported: true, Profiles: []string{"Default", "FPS"}, ActiveProfile: "Default"}, true
}

func TestGlaiveWorkspaceSummaryUsesSharedMousePresentations(t *testing.T) {
	const serial = "glaive-workspace"
	p := glaiveWorkspaceProvider{serial}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, ProductType: common.ProductTypeGlaiveRgbPro, Instance: p}}, map[string]stats.BatteryStats{}, serial)
	if !ok || !summary.LegacyLighting || summary.DPI == nil || summary.Buttons == nil || summary.Performance == nil || summary.Performance.AngleSnapping == nil || summary.DeviceProfiles == nil || summary.Performance.ButtonOptimization != nil || summary.Performance.LiftHeight != nil {
		t.Fatalf("summary=%#v ok=%t", summary, ok)
	}
}
