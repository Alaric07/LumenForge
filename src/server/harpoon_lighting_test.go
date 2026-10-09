package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices/harpoonrgbpro"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"reflect"
	"sort"
	"testing"
)

type harpoonLightingProvider struct {
	serial string
	usable bool
}

func (p harpoonLightingProvider) LightingDeviceID() string { return p.serial }
func (p harpoonLightingProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "mouse", EffectSupported: true, HasBrightness: true, Brightness: 70, SupportedEffects: []lightingpresentation.EffectOption{{ID: "mouse", Label: "Mouse"}}, AuthoredZoneEditor: &lightingpresentation.AuthoredZoneEditor{EffectID: "mouse", Zones: []lightingpresentation.AuthoredZone{{ID: "0", Label: "Logo", ColorHex: "#ffff00"}}}}, p.usable
}
func TestHarpoonLightingSuppressionRequiresUsableSnapshot(t *testing.T) {
	for _, usable := range []bool{false, true} {
		const id = "harpoon-canonical"
		p := harpoonLightingProvider{id, usable}
		summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{id: {Serial: id, ProductType: common.ProductTypeHarpoonRgbPro, Instance: p}}, map[string]stats.BatteryStats{}, id)
		if !ok || summary.LegacyLighting == usable || (summary.Lighting != nil) != usable {
			t.Fatal("suppression", usable, summary)
		}
		if usable && (summary.Lighting.ClusterOwnershipAvailable || summary.Lighting.ExternalOwnershipAvailable) {
			t.Fatal("unsupported external ownership")
		}
	}
	d := &harpoonrgbpro.Device{Serial: "harpoon-failed-attachment"}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{d.Serial: {Serial: d.Serial, ProductType: common.ProductTypeHarpoonRgbPro, Instance: d}}, map[string]stats.BatteryStats{}, d.Serial)
	if !ok || !summary.LegacyLighting || summary.Lighting != nil {
		t.Fatal("failed attachment lost legacy presentation")
	}
}
func TestHarpoonInertCanonicalLightingPreview(t *testing.T) {
	s := buildHarpoonRGBProModernPreview()
	if s.Product != "HARPOON RGB PRO" || s.Serial != "preview-harpoon-rgb-pro-modern" || s.LegacyLighting || s.Lighting == nil || s.Lighting.ConfiguredEffect != "mouse" || s.Lighting.Brightness != 70 || s.Lighting.AuthoredZoneEditor == nil || len(s.Lighting.AuthoredZoneEditor.Zones) != 1 {
		t.Fatal("preview", s)
	}
	zone := s.Lighting.AuthoredZoneEditor.Zones[0]
	if zone.ID != "0" || zone.Label != "Logo" || zone.ColorHex != "#ffff00" || s.Lighting.ClusterOwnershipAvailable || s.Lighting.ExternalOwnershipAvailable {
		t.Fatal("preview topology/ownership", zone)
	}
	got := []string{}
	for _, effect := range s.Lighting.SupportedEffects {
		got = append(got, effect.ID)
	}
	want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	sort.Strings(want)
	if len(got) != 20 || !reflect.DeepEqual(got, want) {
		t.Fatal("catalogue", got)
	}
	if len(s.DPI.RegularStages) != 5 || s.DPI.SniperStage.ID != "5" || s.DPI.MaximumDPI != 12000 || s.Performance.PollingRate.Value != 1 {
		t.Fatal("borrowed input defaults")
	}
	s.Lighting.AuthoredZoneEditor.Zones[0].ColorHex = "#000000"
	s.Lighting.SupportedEffects[0].ID = "invented"
	next := buildHarpoonRGBProModernPreview()
	if next.Lighting.AuthoredZoneEditor.Zones[0].ColorHex != "#ffff00" || next.Lighting.SupportedEffects[0].ID != want[0] {
		t.Fatal("shared mutable preview")
	}
}
