package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices"
	"LumenForge/src/devices/harpoonWU"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"reflect"
	"sort"
	"testing"
)

type harpoonWULightingProvider struct{ usable bool }

func (p harpoonWULightingProvider) LightingDeviceID() string { return "harpoon-wu-provider" }
func (p harpoonWULightingProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, HasBrightness: true, Brightness: 70}, p.usable
}
func (harpoonWULightingProvider) ProcessSetRgbCluster(bool) uint8         { return 1 }
func (harpoonWULightingProvider) ProcessSetOpenRgbIntegration(bool) uint8 { return 1 }
func TestHarpoonWUCanonicalSuppressionRequiresUsableSnapshot(t *testing.T) {
	for _, usable := range []bool{false, true} {
		provider := harpoonWULightingProvider{usable}
		id := provider.LightingDeviceID()
		s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{id: {Serial: id, ProductType: common.ProductTypeHarpoonRgbWU, Instance: provider}}, map[string]stats.BatteryStats{}, id)
		if !ok || s.LegacyLighting == usable || (s.Lighting != nil) != usable {
			t.Fatal("ProductType-only suppression", s)
		}
		if usable && (!s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable) {
			t.Fatal("missing owner capabilities")
		}
	}
	d := &harpoonWU.Device{Serial: "harpoon-wu-failed"}
	s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{d.Serial: {Serial: d.Serial, ProductType: common.ProductTypeHarpoonRgbWU, Instance: d}}, map[string]stats.BatteryStats{}, d.Serial)
	if !ok || !s.LegacyLighting || s.Lighting != nil {
		t.Fatal("failed attachment lost legacy presentation")
	}
}
func TestHarpoonWUInertCanonicalPreview(t *testing.T) {
	{
		s := buildHarpoonUSBModernPreview()
		if s.Product != "HARPOON RGB" || s.Serial != "preview-harpoon-usb-modern" || s.LegacyLighting || s.Unavailable || s.Lighting == nil || s.Lighting.ConfiguredEffect != "mouse" || s.Lighting.Brightness != 70 || !s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable || s.Lighting.ClusterControlled || s.Lighting.ExternalControlled {
			t.Fatal(s)
		}
		got := []string{}
		for _, effect := range s.Lighting.SupportedEffects {
			got = append(got, effect.ID)
		}
		want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
		if len(got) != 20 {
			t.Fatal(got)
		}
		// Shared workspace presentation sorts labels; the native provider order
		// is separately pinned in harpoonWU catalogue tests.
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatal(got)
		}
		zones := s.Lighting.AuthoredZoneEditor.Zones
		if len(zones) != 1 || zones[0].ID != "0" || zones[0].Label != "Logo" || zones[0].ColorHex != "#ffff00" {
			t.Fatal(zones)
		}
		if s.Performance.PollingRate == nil || s.Performance.LiftHeight != nil || s.DPI.MaximumDPI != 10000 || len(s.DPI.RegularStages) != 5 || s.DPI.SniperStage.ID != "5" || s.DPI.SniperStage.DPI != 200 {
			t.Fatal("borrowed input capabilities")
		}
		for i, value := range []int{400, 1000, 1600, 3000, 5000} {
			if int(s.DPI.RegularStages[i].DPI) != value {
				t.Fatal("WU DPI preview defaults")
			}
		}
		if s.Performance.PollingRate.Value != 4 {
			t.Fatal("WU polling preview default")
		}
		original := buildHarpoonUSBModernPreview()
		s.Lighting.SupportedEffects[0].ID = "bad"
		s.Lighting.AuthoredZoneEditor.Zones[0].ColorHex = "bad"
		if !reflect.DeepEqual(original, buildHarpoonUSBModernPreview()) || devices.GetDevice(s.Serial) != nil {
			t.Fatal("fixture aliases/registered live device")
		}
	}
	fixture, ok := modernDevicePreviewFixtureByKey("harpoon-usb-modern")
	if !ok || fixture.ProductType != common.ProductTypeHarpoonRgbWU {
		t.Fatal("fixture identity")
	}
}
