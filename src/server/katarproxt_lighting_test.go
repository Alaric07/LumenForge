package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices/katarproxt"
	"LumenForge/src/stats"
	"reflect"
	"sort"
	"testing"
)

func TestKatarProXTLightingSuppressionNeedsUsableSnapshot(t *testing.T) {
	for _, usable := range []bool{false, true} {
		serial := "xt-canonical"
		provider := katarCanonicalProvider{katarWorkspaceProvider{serial}, usable}
		summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, ProductType: common.ProductTypeKatarProXT, Instance: provider}}, map[string]stats.BatteryStats{}, serial)
		if !ok || summary.LegacyLighting == usable || (summary.Lighting != nil) != usable || summary.DPI == nil || summary.Buttons == nil {
			t.Fatalf("usable=%t summary=%#v", usable, summary)
		}
		if usable && (summary.Lighting.ClusterOwnershipAvailable || summary.Lighting.ExternalOwnershipAvailable) {
			t.Fatal("invented XT ownership")
		}
	}
	d := &katarproxt.Device{Serial: "xt-unavailable"}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{d.Serial: {Serial: d.Serial, ProductType: common.ProductTypeKatarProXT, Instance: d}}, map[string]stats.BatteryStats{}, d.Serial)
	if !ok || !summary.LegacyLighting || summary.Lighting != nil {
		t.Fatal("unattached XT lost fallback")
	}
}

func TestKatarProXTInertLightingPreview(t *testing.T) {
	s := buildKatarProXTModernPreview()
	if s.Product != "KATAR PRO XT" || s.Serial != "preview-katar-pro-xt-modern" || s.LegacyLighting || s.Lighting == nil || s.Lighting.ConfiguredEffect != "mouse" || s.Lighting.Brightness != 70 || s.Lighting.AuthoredZoneEditor == nil || len(s.Lighting.AuthoredZoneEditor.Zones) != 1 || s.Lighting.AuthoredZoneEditor.Zones[0].ID != "0" || s.Lighting.AuthoredZoneEditor.Zones[0].Label != "Scroll" || s.Lighting.AuthoredZoneEditor.Zones[0].ColorHex != "#00ffff" || s.Lighting.ClusterOwnershipAvailable || s.Lighting.ExternalOwnershipAvailable {
		t.Fatalf("XT fixture=%#v", s)
	}
	got := []string{}
	for _, effect := range s.Lighting.SupportedEffects {
		got = append(got, effect.ID)
	}
	want := []string{"colorpulse", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != 19 || !reflect.DeepEqual(got, want) {
		t.Fatal("XT preview catalogue", got)
	}
	if s.DPI.MaximumDPI != 18000 || s.DPI.RegularStages[1].DPI != 1500 || s.DPI.SniperStage.DPI != 200 {
		t.Fatal("borrowed DPI fixture defaults")
	}
	// Each call returns fresh descriptor data; no device/runtime is constructed.
	s.Lighting.AuthoredZoneEditor.Zones[0].ColorHex = "#000000"
	if buildKatarProXTModernPreview().Lighting.AuthoredZoneEditor.Zones[0].ColorHex != "#00ffff" {
		t.Fatal("shared mutable preview state")
	}
}
