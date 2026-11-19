package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices"
	"LumenForge/src/devices/m65rgbultraWU"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
	"reflect"
	"testing"
)

type wuLightingProvider struct{ usable bool }

func (p wuLightingProvider) LightingDeviceID() string { return "wu-provider" }
func (p wuLightingProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return lightingpresentation.Snapshot{TargetKind: "native", ConfiguredEffect: "static", EffectSupported: true, EffectSelectionAvailable: true, HasBrightness: true, Brightness: 100}, p.usable
}
func (wuLightingProvider) ProcessSetRgbCluster(bool) uint8         { return 1 }
func (wuLightingProvider) ProcessSetOpenRgbIntegration(bool) uint8 { return 1 }
func TestWUCanonicalSuppressionRequiresUsableSnapshot(t *testing.T) {
	for _, usable := range []bool{false, true} {
		p := wuLightingProvider{usable}
		id := p.LightingDeviceID()
		s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{id: {Serial: id, ProductType: common.ProductTypeM65RgbUltraWU, Instance: p}}, map[string]stats.BatteryStats{}, id)
		if !ok || s.LegacyLighting == usable || (s.Lighting != nil) != usable {
			t.Fatal(usable, s)
		}
		if usable && (!s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable) {
			t.Fatal("missing completed external adapters")
		}
	}
	d := &m65rgbultraWU.Device{Serial: "failed-WU"}
	s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{d.Serial: {Serial: d.Serial, ProductType: common.ProductTypeM65RgbUltraWU, Instance: d}}, nil, d.Serial)
	if !ok || !s.LegacyLighting || s.Lighting != nil {
		t.Fatal("failed attachment suppressed legacy fallback", s)
	}
}
func TestWUPreviewIndependentInertAndVerifiedCapabilities(t *testing.T) {
	fixture, ok := modernDevicePreviewFixtureByKey("m65-rgb-ultra-wireless-usb-modern")
	if !ok || fixture.ProductType != common.ProductTypeM65RgbUltraWU {
		t.Fatal(fixture)
	}
	s := fixture.Build()
	if s.Product != "M65 RGB ULTRA" || s.Serial != "preview-m65-rgb-ultra-wireless-usb-modern" || s.LegacyLighting || s.Lighting == nil || s.Lighting.ConfiguredEffect != "mouse" || s.Lighting.Brightness != 100 {
		t.Fatal(s)
	}
	if !s.Lighting.ClusterOwnershipAvailable || !s.Lighting.ExternalOwnershipAvailable || s.Lighting.ClusterControlled || s.Lighting.ExternalControlled {
		t.Fatal("external capabilities", s.Lighting)
	}
	want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	// The live WU snapshot pins source order; the generic server DTO sorts labels.
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
	if len(zones) != 1 || zones[0].ID != "0" || zones[0].Label != "Logo" || zones[0].ColorHex != "#ffff00" {
		t.Fatal(zones)
	}
	if s.SleepTimer != nil || s.Performance.PollingRate == nil || s.Performance.PollingRate.Value != 4 || len(s.Performance.PollingRate.Options) != 5 || s.Performance.PollingRate.Options[1].Label != "125 Hz / 8 msec" {
		t.Fatal("USB controls", s.Performance, s.SleepTimer)
	}
	if !s.HasBattery || s.MouseGestures == nil || len(s.MouseGestures.Tilts) != 4 || s.DPI.MinimumDPI != 100 || s.DPI.MaximumDPI != 26000 || s.DPI.ActiveRegularStageID != "1" || s.DPI.SniperStage.DPI != 200 || s.DPI.SniperStage.ColorHex != "#ffff00" || len(s.Buttons.Buttons) != 12 {
		t.Fatal("WU input fixture", s)
	}
	for i, v := range []uint16{400, 800, 1200, 1600, 3000} {
		if s.DPI.RegularStages[i].DPI != v {
			t.Fatal(i, s.DPI.RegularStages[i])
		}
	}
	if s.OverviewPerformance.Rows[0].Value != "800" || s.OverviewPerformance.Rows[2].Value != "1000 Hz / 1 msec" {
		t.Fatal("overview disagrees with USB controls", s.OverviewPerformance)
	}
	original := fixture.Build()
	s.Lighting.AuthoredZoneEditor.Zones[0].ColorHex = "bad"
	s.Buttons.Buttons[0].Name = "bad"
	s.Performance.PollingRate.Options[0].Label = "bad"
	if !reflect.DeepEqual(original, fixture.Build()) || devices.GetDevice(s.Serial) != nil {
		t.Fatal("preview shared state or registered live device")
	}
	wireless, _ := modernDevicePreviewFixtureByKey("m65-rgb-ultra-wireless-modern")
	if wireless.ProductType != common.ProductTypeM65RgbUltraW || wireless.Build().Serial == s.Serial || wireless.Build().Performance.PollingRate != nil {
		t.Fatal("WU modified W fixture")
	}
}
