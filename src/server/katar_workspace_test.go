package server

import (
	"LumenForge/src/buttonspresentation"
	"LumenForge/src/common"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/dpipresentation"
	"LumenForge/src/performancepresentation"
	"LumenForge/src/stats"
	"testing"
)

type katarWorkspaceProvider struct{ serial string }

func (p katarWorkspaceProvider) DPIDeviceID() string { return p.serial }
func (p katarWorkspaceProvider) DPISnapshot() (dpipresentation.Snapshot, bool) {
	return dpipresentation.Snapshot{MinimumDPI: 100, MaximumDPI: 12400, ActiveRegularStageID: "1", Stages: []dpipresentation.Stage{{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#ff0000"}, {ID: "1", Name: "Stage 2", DPI: 1600, ColorHex: "#00ff00"}, {ID: "3", Name: "Sniper", DPI: 400, ColorHex: "#ffff00", Sniper: true}}}, true
}
func (p katarWorkspaceProvider) ButtonsDeviceID() string { return p.serial }
func (p katarWorkspaceProvider) ButtonsSnapshot() (buttonspresentation.Snapshot, bool) {
	return buttonspresentation.Snapshot{Buttons: []buttonspresentation.Button{{KeyIndex: 1, Name: "Left"}}, AssignmentTypes: []buttonspresentation.AssignmentType{{ID: 0, Label: "None"}}}, true
}
func (p katarWorkspaceProvider) PerformanceDeviceID() string { return p.serial }
func (p katarWorkspaceProvider) PerformanceSnapshot() (performancepresentation.Snapshot, bool) {
	return performancepresentation.Snapshot{PollingRate: &performancepresentation.SelectSetting{Value: 4, Options: []performancepresentation.Option{{Value: 4, Label: "1000 Hz"}}}, ButtonOptimization: &performancepresentation.SelectSetting{Value: 1, Options: []performancepresentation.Option{{Value: 0, Label: "Disabled"}, {Value: 1, Label: "Enabled"}}}}, true
}
func (p katarWorkspaceProvider) DeviceProfileDeviceID() string { return p.serial }
func (p katarWorkspaceProvider) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	return deviceprofilepresentation.Snapshot{Supported: true, Profiles: []string{"Default", "FPS"}, ActiveProfile: "Default"}, true
}
func TestKatarWorkspaceSummaryUsesSharedMousePresentations(t *testing.T) {
	for _, productType := range []uint16{common.ProductTypeKatarPro, common.ProductTypeKatarProXT} {
		serial := "katar-workspace"
		p := katarWorkspaceProvider{serial}
		summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, ProductType: productType, Instance: p}}, map[string]stats.BatteryStats{}, serial)
		if !ok || !summary.LegacyLighting || summary.DPI == nil || summary.Buttons == nil || summary.Performance == nil || summary.Performance.ButtonOptimization == nil || summary.DeviceProfiles == nil || summary.Performance.AngleSnapping != nil || summary.Performance.LiftHeight != nil {
			t.Fatalf("summary=%#v ok=%t", summary, ok)
		}
	}
}
