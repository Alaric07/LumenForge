package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices"
	"LumenForge/src/devices/glaivergb"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"reflect"
	"testing"
)

type glaiveLightingProvider struct{ usable bool }

func (p glaiveLightingProvider) LightingDeviceID() string { return "glaive-provider" }
func (p glaiveLightingProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, HasBrightness: true, Brightness: 70}, p.usable
}
func (glaiveLightingProvider) ProcessSetRgbCluster(bool) uint8         { return 1 }
func (glaiveLightingProvider) ProcessSetOpenRgbIntegration(bool) uint8 { return 1 }
func TestGlaiveSuppressionIsSnapshotDriven(t *testing.T) {
	for _, usable := range []bool{false, true} {
		p := glaiveLightingProvider{usable}
		id := p.LightingDeviceID()
		s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{id: {Serial: id, ProductType: common.ProductTypeGlaiveRgb, Instance: p}}, map[string]stats.BatteryStats{}, id)
		if !ok || s.LegacyLighting == usable || (s.Lighting != nil) != usable {
			t.Fatal("snapshot suppression", usable, s)
		}
		if usable && (!s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable) {
			t.Fatal("missing external capabilities")
		}
	}
	d := &glaivergb.Device{Serial: "failed-glaive"}
	s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{d.Serial: {Serial: d.Serial, ProductType: common.ProductTypeGlaiveRgb, Instance: d}}, map[string]stats.BatteryStats{}, d.Serial)
	if !ok || !s.LegacyLighting || s.Lighting != nil {
		t.Fatal("failed attachment lost legacy")
	}
}
func TestGlaiveCanonicalPreviewInert(t *testing.T) {
	s := buildGlaiveRGBModernPreview()
	if s.Product != "GLAIVE RGB" || s.Serial != "preview-glaive-rgb-modern" || s.LegacyLighting || s.Lighting == nil || s.Lighting.ConfiguredEffect != "mouse" || s.Lighting.Brightness != 70 || !s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable || s.Lighting.ClusterControlled || s.Lighting.ExternalControlled {
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
	original := buildGlaiveRGBModernPreview()
	s.Lighting.AuthoredZoneEditor.Zones[0].ColorHex = "bad"
	s.Lighting.SupportedEffects[0].ID = "bad"
	next := buildGlaiveRGBModernPreview()
	if !reflect.DeepEqual(original, next) || devices.GetDevice(s.Serial) != nil {
		t.Fatal("fixture aliases or registered live device")
	}
}
