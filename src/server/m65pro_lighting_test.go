package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices"
	"LumenForge/src/devices/m65prorgb"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"reflect"
	"testing"
)

type m65ProLightingProvider struct{ usable bool }

func (p m65ProLightingProvider) LightingDeviceID() string { return "m65-pro-provider" }
func (p m65ProLightingProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, HasBrightness: true, Brightness: 70}, p.usable
}
func (m65ProLightingProvider) ProcessSetRgbCluster(bool) uint8         { return 1 }
func (m65ProLightingProvider) ProcessSetOpenRgbIntegration(bool) uint8 { return 1 }
func TestM65ProSuppressionIsSnapshotDriven(t *testing.T) {
	for _, usable := range []bool{false, true} {
		p := m65ProLightingProvider{usable}
		id := p.LightingDeviceID()
		s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{id: {Serial: id, ProductType: common.ProductTypeM65RgbElite, Instance: p}}, map[string]stats.BatteryStats{}, id)
		if !ok || s.LegacyLighting == usable || (s.Lighting != nil) != usable {
			t.Fatal("snapshot suppression", usable, s)
		}
		if usable && (!s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable) {
			t.Fatal("missing external capabilities")
		}
	}
	d := &m65prorgb.Device{Serial: "failed-m65-pro"}
	s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{d.Serial: {Serial: d.Serial, ProductType: common.ProductTypeM65RgbElite, Instance: d}}, map[string]stats.BatteryStats{}, d.Serial)
	if !ok || !s.LegacyLighting || s.Lighting != nil {
		t.Fatal("failed attachment lost legacy")
	}
}
func TestM65ProCanonicalPreviewInert(t *testing.T) {
	s := buildM65ProRGBModernPreview()
	if s.Product != "M65 PRO RGB" || s.Serial != "preview-m65-pro-rgb-modern" || s.LegacyLighting || s.Lighting == nil || s.Lighting.ConfiguredEffect != "mouse" || s.Lighting.Brightness != 100 || !s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable || s.Lighting.ClusterControlled || s.Lighting.ExternalControlled {
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
	if len(zones) != 2 || zones[0].ID != "0" || zones[0].Label != "Front" || zones[1].Label != "Logo" {
		t.Fatal(zones)
	}
	if s.DPI.MaximumDPI != 12000 || s.DPI.SniperStage.DPI != 400 || s.DPI.SniperStage.ColorHex != "#008080" || zones[0].ColorHex != "#00ffff" || zones[1].ColorHex != "#ffff00" || s.Performance.ButtonOptimization != nil {
		t.Fatal("PRO fixture contract", s)
	}
	original := buildM65ProRGBModernPreview()
	s.Lighting.AuthoredZoneEditor.Zones[0].ColorHex = "bad"
	s.Lighting.SupportedEffects[0].ID = "bad"
	next := buildM65ProRGBModernPreview()
	if !reflect.DeepEqual(original, next) || devices.GetDevice(s.Serial) != nil {
		t.Fatal("fixture aliases or registered live device")
	}
}
