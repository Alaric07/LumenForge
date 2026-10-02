package server

import (
	"LumenForge/src/buttonspresentation"
	"LumenForge/src/common"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/devices/katarproW"
	"LumenForge/src/dpipresentation"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/performancepresentation"
	"LumenForge/src/stats"
	"reflect"
	"sort"
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
	return deviceprofilepresentation.Snapshot{Supported: true, CanSwitch: true, CanSave: true, CanDelete: true, Profiles: []string{"Default", "FPS"}, ActiveProfile: "Default"}, true
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

type katarCanonicalProvider struct {
	katarWorkspaceProvider
	usable bool
}

func (p katarCanonicalProvider) LightingDeviceID() string { return p.serial }
func (p katarCanonicalProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, HasBrightness: true, Brightness: 70, SupportedEffects: []lightingpresentation.EffectOption{{ID: "mouse", Label: "Mouse"}}}, p.usable
}
func TestKatarLightingCutoverRequiresUsableSnapshot(t *testing.T) {
	for _, usable := range []bool{false, true} {
		serial := "katar-canonical"
		p := katarCanonicalProvider{katarWorkspaceProvider{serial}, usable}
		summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, ProductType: common.ProductTypeKatarPro, Instance: p}}, map[string]stats.BatteryStats{}, serial)
		if !ok || summary.LegacyLighting == usable || (summary.Lighting != nil) != usable || summary.DPI == nil || summary.Buttons == nil {
			t.Fatalf("usable=%t summary=%#v", usable, summary)
		}
		if usable && (summary.Lighting.ClusterOwnershipAvailable || summary.Lighting.ExternalOwnershipAvailable) {
			t.Fatal("unsupported ownership exposed")
		}
	}
}
func TestKatarProCanonicalPreviewAndWirelessSeparation(t *testing.T) {
	s := buildKatarProModernPreview()
	if s.LegacyLighting || s.Lighting == nil || s.Lighting.ConfiguredEffect != "mouse" || s.Lighting.Brightness != 70 || len(s.Lighting.SupportedEffects) != 19 || s.Lighting.AuthoredZoneEditor == nil || len(s.Lighting.AuthoredZoneEditor.Zones) != 1 || s.Lighting.AuthoredZoneEditor.Zones[0].Label != "Scroll" {
		t.Fatalf("preview=%#v", s)
	}
	var got []string
	for _, effect := range s.Lighting.SupportedEffects {
		got = append(got, effect.ID)
	}
	sort.Strings(got)
	want := []string{"colorpulse", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("catalogue=%v", got)
	}
	w := buildKatarProWirelessModernPreview()
	if w.Lighting != nil {
		t.Fatal("wireless lighting exposed")
	}
	if _, ok := interface{}(&katarproW.Device{}).(devicesLightingSnapshotProvider); ok {
		t.Fatal("wireless canonical Lighting provider added")
	}
}
