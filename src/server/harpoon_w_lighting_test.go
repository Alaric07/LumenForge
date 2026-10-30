package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices"
	"LumenForge/src/devices/harpoonW"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"reflect"
	"testing"
)

type harpoonWLightingProvider struct{ usable bool }

func (p harpoonWLightingProvider) LightingDeviceID() string { return "harpoon-w-provider" }
func (p harpoonWLightingProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, HasBrightness: true, Brightness: 70}, p.usable
}
func (harpoonWLightingProvider) ProcessSetRgbCluster(bool) uint8         { return 1 }
func (harpoonWLightingProvider) ProcessSetOpenRgbIntegration(bool) uint8 { return 1 }
func TestHarpoonWCanonicalSuppressionRequiresUsableSnapshot(t *testing.T) {
	for _, usable := range []bool{false, true} {
		provider := harpoonWLightingProvider{usable}
		id := provider.LightingDeviceID()
		s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{id: {Serial: id, ProductType: common.ProductTypeHarpoonRgbW, Instance: provider}}, map[string]stats.BatteryStats{}, id)
		if !ok || s.LegacyLighting == usable || (s.Lighting != nil) != usable {
			t.Fatal("ProductType-only suppression", s)
		}
		if usable && (!s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable) {
			t.Fatal("missing owner capabilities")
		}
	}
	d := &harpoonW.Device{Serial: "harpoon-w-failed"}
	s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{d.Serial: {Serial: d.Serial, ProductType: common.ProductTypeHarpoonRgbW, Instance: d}}, map[string]stats.BatteryStats{}, d.Serial)
	if !ok || !s.LegacyLighting || s.Lighting != nil {
		t.Fatal("failed attachment lost legacy presentation")
	}
}
func TestHarpoonWInertCanonicalPreview(t *testing.T) {
	for _, disconnected := range []bool{false, true} {
		s := buildHarpoonWirelessModernPreview(disconnected)
		if s.Product != "HARPOON RGB" || s.Serial != "preview-harpoon-wireless-modern" || s.LegacyLighting || s.Unavailable != disconnected || s.Lighting == nil || s.Lighting.ConfiguredEffect != "mouse" || s.Lighting.Brightness != 70 || !s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable || s.Lighting.ClusterControlled || s.Lighting.ExternalControlled {
			t.Fatal(s)
		}
		got := map[string]bool{}
		for _, effect := range s.Lighting.SupportedEffects {
			got[effect.ID] = true
		}
		want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
		if len(got) != 20 {
			t.Fatal(got)
		}
		for _, effect := range want {
			if !got[effect] {
				t.Fatal(effect)
			}
		}
		zones := s.Lighting.AuthoredZoneEditor.Zones
		if len(zones) != 1 || zones[0].ID != "0" || zones[0].Label != "Logo" || zones[0].ColorHex != "#ffff00" {
			t.Fatal(zones)
		}
		if s.Performance.PollingRate != nil || s.Performance.LiftHeight != nil || s.DPI.MaximumDPI != 10000 || len(s.DPI.RegularStages) != 5 || s.DPI.SniperStage.ID != "5" || s.DPI.SniperStage.DPI != 200 {
			t.Fatal("borrowed input capabilities")
		}
		original := buildHarpoonWirelessModernPreview(disconnected)
		s.Lighting.SupportedEffects[0].ID = "bad"
		s.Lighting.AuthoredZoneEditor.Zones[0].ColorHex = "bad"
		if !reflect.DeepEqual(original, buildHarpoonWirelessModernPreview(disconnected)) || devices.GetDevice(s.Serial) != nil {
			t.Fatal("fixture aliases/registered live device")
		}
	}
	fixture, ok := modernDevicePreviewFixtureByKey("harpoon-wireless-modern")
	if !ok || fixture.ProductType != common.ProductTypeHarpoonRgbW {
		t.Fatal("fixture identity")
	}
}
