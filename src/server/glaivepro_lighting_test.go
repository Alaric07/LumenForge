package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices"
	"LumenForge/src/devices/glaivergbpro"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"reflect"
	"testing"
)

type glaiveProLightingProvider struct{ usable bool }

func (p glaiveProLightingProvider) LightingDeviceID() string { return "glaive-pro-provider" }
func (p glaiveProLightingProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, HasBrightness: true, Brightness: 100}, p.usable
}
func (glaiveProLightingProvider) ProcessSetRgbCluster(bool) uint8         { return 1 }
func (glaiveProLightingProvider) ProcessSetOpenRgbIntegration(bool) uint8 { return 1 }
func TestGlaiveProSuppressionIsSnapshotDriven(t *testing.T) {
	for _, usable := range []bool{false, true} {
		p := glaiveProLightingProvider{usable}
		id := p.LightingDeviceID()
		s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{id: {Serial: id, ProductType: common.ProductTypeGlaiveRgbPro, Instance: p}}, map[string]stats.BatteryStats{}, id)
		if !ok || s.LegacyLighting == usable || (s.Lighting != nil) != usable {
			t.Fatal("snapshot suppression", usable, s)
		}
		if usable && (!s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable) {
			t.Fatal("missing external capabilities")
		}
	}
	d := &glaivergbpro.Device{Serial: "failed-glaive"}
	s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{d.Serial: {Serial: d.Serial, ProductType: common.ProductTypeGlaiveRgbPro, Instance: d}}, map[string]stats.BatteryStats{}, d.Serial)
	if !ok || !s.LegacyLighting || s.Lighting != nil {
		t.Fatal("failed attachment lost legacy")
	}
}
func TestGlaiveProCanonicalPreviewInert(t *testing.T) {
	s := buildGlaiveRGBProModernPreview()
	if s.Product != "GLAIVE RGB PRO" || s.Serial != "preview-glaive-rgb-pro-modern" || s.LegacyLighting || s.Lighting == nil || s.Lighting.ConfiguredEffect != "mouse" || s.Lighting.Brightness != 100 || !s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable || s.Lighting.ClusterControlled || s.Lighting.ExternalControlled {
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
	if len(zones) != 3 || zones[0].ID != "0" || zones[0].Label != "Front" || zones[1].Label != "Logo" || zones[2].Label != "Side" {
		t.Fatal(zones)
	}
	original := buildGlaiveRGBProModernPreview()
	s.Lighting.AuthoredZoneEditor.Zones[0].ColorHex = "bad"
	s.Lighting.SupportedEffects[0].ID = "bad"
	next := buildGlaiveRGBProModernPreview()
	if !reflect.DeepEqual(original, next) || devices.GetDevice(s.Serial) != nil {
		t.Fatal("fixture aliases or registered live device")
	}
}
