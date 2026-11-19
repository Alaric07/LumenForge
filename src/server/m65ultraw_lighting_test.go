package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices"
	"LumenForge/src/devices/m65rgbultraW"
	"LumenForge/src/inputmanager"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"reflect"
	"testing"
)

type m65UltraWLightingProvider struct{ usable bool }

func (p m65UltraWLightingProvider) LightingDeviceID() string { return "m65-ultra-provider" }
func (p m65UltraWLightingProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, HasBrightness: true, Brightness: 70}, p.usable
}
func (m65UltraWLightingProvider) ProcessSetRgbCluster(bool) uint8         { return 1 }
func (m65UltraWLightingProvider) ProcessSetOpenRgbIntegration(bool) uint8 { return 1 }
func TestM65UltraWSuppressionIsSnapshotDriven(t *testing.T) {
	for _, usable := range []bool{false, true} {
		p := m65UltraWLightingProvider{usable}
		id := p.LightingDeviceID()
		s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{id: {Serial: id, ProductType: common.ProductTypeM65RgbUltraW, Instance: p}}, map[string]stats.BatteryStats{}, id)
		if !ok || s.LegacyLighting == usable || (s.Lighting != nil) != usable {
			t.Fatal("snapshot suppression", usable, s)
		}
		if usable && (!s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable) {
			t.Fatal("missing external capabilities")
		}
	}
	d := &m65rgbultraW.Device{Serial: "failed-m65-ultra"}
	s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{d.Serial: {Serial: d.Serial, ProductType: common.ProductTypeM65RgbUltraW, Instance: d}}, map[string]stats.BatteryStats{}, d.Serial)
	if !ok || !s.LegacyLighting || s.Lighting != nil {
		t.Fatal("failed attachment lost legacy")
	}
}
func TestM65UltraWCanonicalPreviewInert(t *testing.T) {
	s := buildM65RGBUltraWirelessModernPreview()
	if s.Product != "M65 RGB ULTRA" || s.Serial != "preview-m65-rgb-ultra-wireless-modern" || s.LegacyLighting || s.Lighting == nil || s.Lighting.ConfiguredEffect != "mouse" || s.Lighting.Brightness != 100 || !s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable || s.Lighting.ClusterControlled || s.Lighting.ExternalControlled {
		t.Fatal(s)
	}
	want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	// Server presentation sorts options; pin exact membership independently of the
	// package snapshot's source-order contract.
	got := map[string]bool{}
	for _, e := range s.Lighting.SupportedEffects {
		got[e.ID] = true
	}
	if len(got) != 20 {
		t.Fatal(got)
	}
	for _, id := range want {
		if !got[id] {
			t.Fatal(id)
		}
	}
	zones := s.Lighting.AuthoredZoneEditor.Zones
	if len(zones) != 1 || zones[0].ID != "0" || zones[0].Label != "Logo" {
		t.Fatal(zones)
	}
	if s.DPI.MaximumDPI != 26000 || s.DPI.SniperStage.DPI != 200 || s.DPI.SniperStage.ColorHex != "#ffff00" || zones[0].ColorHex != "#ffff00" || s.Performance.ButtonOptimization == nil || s.Performance.PollingRate != nil || !s.HasBattery || len(s.Buttons.Buttons) != 12 {
		t.Fatal("Elite fixture contract", s)
	}
	original := buildM65RGBUltraWirelessModernPreview()
	s.Lighting.AuthoredZoneEditor.Zones[0].ColorHex = "bad"
	s.Lighting.SupportedEffects[0].ID = "bad"
	next := buildM65RGBUltraWirelessModernPreview()
	if !reflect.DeepEqual(original, next) || devices.GetDevice(s.Serial) != nil {
		t.Fatal("fixture aliases or registered live device")
	}
}

func TestM65UltraWPreviewOverviewMatchesActiveDPI(t *testing.T) {
	elite := buildM65RGBEliteModernPreview()
	s := buildM65RGBUltraWirelessModernPreview()
	if s.DPI.ActiveRegularStageID != "1" {
		t.Fatal("active stage ID", s.DPI.ActiveRegularStageID)
	}
	active := s.DPI.RegularStages[1]
	if active.ID != s.DPI.ActiveRegularStageID || active.Name != "Stage 2" || !active.Active || active.DPI != 800 {
		t.Fatal("Ultra Stage 2", active)
	}
	rows := s.OverviewPerformance.Rows
	if rows[0].Label != "DPI" || rows[0].Value != "800" || rows[1].Label != "Active Stage" || rows[1].Value != active.Name {
		t.Fatal("Ultra overview disagrees with DPI workspace", rows)
	}
	if elite.DPI.ActiveRegularStageID != "1" || elite.DPI.RegularStages[1].DPI != 1500 || elite.OverviewPerformance.Rows[0].Value != "1500" || !reflect.DeepEqual(elite, buildM65RGBEliteModernPreview()) {
		t.Fatal("Ultra construction changed Elite preview")
	}
	if devices.GetDevice(s.Serial) != nil || !reflect.DeepEqual(s, buildM65RGBUltraWirelessModernPreview()) {
		t.Fatal("preview registered live device or changed fixture")
	}
}

func TestWirelessPreviewUsesOnlyWOverviewAndAssignmentCapabilities(t *testing.T) {
	elite := buildM65RGBEliteModernPreview()
	s := buildM65RGBUltraWirelessModernPreview()
	for _, row := range s.OverviewPerformance.Rows {
		if row.Label == "Polling Rate" {
			t.Fatal("wireless child claims polling overview")
		}
	}
	if s.OverviewPerformance.Rows[0].Value != "800" || s.OverviewPerformance.Rows[1].Value != "Stage 2" {
		t.Fatal("DPI overview changed", s.OverviewPerformance.Rows)
	}
	// Concrete live provider: ButtonsSnapshot only returns the W-visible type IDs.
	assignments := map[int]inputmanager.KeyAssignment{}
	for _, button := range s.Buttons.Buttons {
		assignments[button.KeyIndex] = inputmanager.KeyAssignment{Name: button.Name}
	}
	live := &m65rgbultraW.Device{KeyAssignment: assignments, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 2: "DPI +", 3: "Keyboard", 4: "DPI -", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Profile Switch"}}
	snapshot, ok := live.ButtonsSnapshot()
	if !ok {
		t.Fatal("live snapshot unavailable")
	}
	if len(snapshot.AssignmentTypes) != len(s.Buttons.AssignmentTypes) {
		t.Fatal("assignment count")
	}
	for i, kind := range snapshot.AssignmentTypes {
		if s.Buttons.AssignmentTypes[i].ID != kind.ID || s.Buttons.AssignmentTypes[i].Label != kind.Label || kind.ID == 4 {
			t.Fatal("preview differs from live W", kind)
		}
	}
	if !reflect.DeepEqual(elite, buildM65RGBEliteModernPreview()) || devices.GetDevice(s.Serial) != nil {
		t.Fatal("preview changed Elite or registered live")
	}
}
