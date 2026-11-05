package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices"
	"LumenForge/src/devices/m65rgbultra"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"reflect"
	"testing"
)

type m65UltraLightingProvider struct{ usable bool }

func (p m65UltraLightingProvider) LightingDeviceID() string { return "m65-ultra-provider" }
func (p m65UltraLightingProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, HasBrightness: true, Brightness: 70}, p.usable
}
func (m65UltraLightingProvider) ProcessSetRgbCluster(bool) uint8         { return 1 }
func (m65UltraLightingProvider) ProcessSetOpenRgbIntegration(bool) uint8 { return 1 }
func TestM65UltraSuppressionIsSnapshotDriven(t *testing.T) {
	for _, usable := range []bool{false, true} {
		p := m65UltraLightingProvider{usable}
		id := p.LightingDeviceID()
		s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{id: {Serial: id, ProductType: common.ProductTypeM65RgbUltra, Instance: p}}, map[string]stats.BatteryStats{}, id)
		if !ok || s.LegacyLighting == usable || (s.Lighting != nil) != usable {
			t.Fatal("snapshot suppression", usable, s)
		}
		if usable && (!s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable) {
			t.Fatal("missing external capabilities")
		}
	}
	d := &m65rgbultra.Device{Serial: "failed-m65-ultra"}
	s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{d.Serial: {Serial: d.Serial, ProductType: common.ProductTypeM65RgbUltra, Instance: d}}, map[string]stats.BatteryStats{}, d.Serial)
	if !ok || !s.LegacyLighting || s.Lighting != nil {
		t.Fatal("failed attachment lost legacy")
	}
}
func TestM65UltraCanonicalPreviewInert(t *testing.T) {
	s := buildM65RGBUltraModernPreview()
	if s.Product != "M65 RGB ULTRA" || s.Serial != "preview-m65-rgb-ultra-modern" || s.LegacyLighting || s.Lighting == nil || s.Lighting.ConfiguredEffect != "mouse" || s.Lighting.Brightness != 100 || !s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable || s.Lighting.ClusterControlled || s.Lighting.ExternalControlled {
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
	if len(zones) != 2 || zones[0].ID != "0" || zones[0].Label != "Scroll" || zones[1].Label != "Logo" {
		t.Fatal(zones)
	}
	if s.DPI.MaximumDPI != 26000 || s.DPI.SniperStage.DPI != 200 || s.DPI.SniperStage.ColorHex != "#ffff00" || zones[0].ColorHex != "#00ffff" || zones[1].ColorHex != "#ffff00" || s.Performance.ButtonOptimization == nil {
		t.Fatal("Elite fixture contract", s)
	}
	original := buildM65RGBUltraModernPreview()
	s.Lighting.AuthoredZoneEditor.Zones[0].ColorHex = "bad"
	s.Lighting.SupportedEffects[0].ID = "bad"
	next := buildM65RGBUltraModernPreview()
	if !reflect.DeepEqual(original, next) || devices.GetDevice(s.Serial) != nil {
		t.Fatal("fixture aliases or registered live device")
	}
}

func TestM65UltraPreviewOverviewMatchesActiveDPI(t *testing.T) {
	elite := buildM65RGBEliteModernPreview()
	s := buildM65RGBUltraModernPreview()
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
	if devices.GetDevice(s.Serial) != nil || !reflect.DeepEqual(s, buildM65RGBUltraModernPreview()) {
		t.Fatal("preview registered live device or changed fixture")
	}
}
