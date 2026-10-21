package server

import (
	"reflect"
	"testing"

	"LumenForge/src/common"
	"LumenForge/src/devices/darkcorergbseW"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/stats"
)

type darkCoreSELightingProvider struct {
	serial string
	usable bool
	kind   string
}

func (p darkCoreSELightingProvider) LightingDeviceID() string { return p.serial }
func (p darkCoreSELightingProvider) LightingSnapshot() (lightingpresentation.Snapshot, bool) {
	return lightingpresentation.Snapshot{TargetKind: p.kind, ConfiguredEffect: "static", EffectSupported: true, EffectSelectionAvailable: true, HasBrightness: true, Brightness: 100, SingleColorHex: "#00ffff", PaletteKind: "static-single-color", SupportedEffects: []lightingpresentation.EffectOption{{ID: "mouse", Label: "Mouse"}, {ID: "static", Label: "Static"}}}, p.usable
}

func TestDarkCoreSELightingFallbackRequiresUsableMatchingSnapshot(t *testing.T) {
	for _, test := range []struct {
		name      string
		instance  any
		canonical bool
	}{
		{"failed attachment", &darkcorergbseW.Device{Serial: "6987"}, false},
		{"unusable", darkCoreSELightingProvider{"6987", false, "native"}, false},
		{"wrong identity", darkCoreSELightingProvider{"other", true, "native"}, false},
		{"empty target", darkCoreSELightingProvider{"6987", true, ""}, false},
		{"usable", darkCoreSELightingProvider{"6987", true, "native"}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{"6987": {Serial: "6987", ProductType: common.ProductTypeDarkCoreRgbSEW, Instance: test.instance}}, map[string]stats.BatteryStats{}, "6987")
			if !ok || s.LegacyLighting == test.canonical || (s.Lighting != nil) != test.canonical {
				t.Fatalf("summary %#v", s)
			}
			if s.Lighting != nil && (s.Lighting.ClusterOwnershipAvailable || s.Lighting.ExternalOwnershipAvailable) {
				t.Fatal("invented external ownership")
			}
		})
	}
}
func TestDarkCoreSECanonicalPreviewIsExactAndIndependent(t *testing.T) {
	first, second := buildDarkCoreSEWirelessModernPreview(), buildDarkCoreSEWirelessModernPreview()
	if first.Product != "DARK CORE SE" || first.Serial != "preview-dark-core-rgb-se-wireless-modern" || first.LegacyLighting || first.Lighting == nil {
		t.Fatalf("preview %#v", first)
	}
	l := first.Lighting
	if l.ConfiguredEffect != "mouse" || l.Brightness != 100 || l.ClusterOwnershipAvailable || l.ExternalOwnershipAvailable {
		t.Fatalf("lighting %#v", l)
	}
	var ids []string
	for _, e := range l.SupportedEffects {
		ids = append(ids, e.ID)
	}
	if !reflect.DeepEqual(ids, []string{"mouse", "static"}) {
		t.Fatal("catalogue", ids)
	}
	if l.AuthoredZoneEditor == nil || len(l.AuthoredZoneEditor.Zones) != 3 {
		t.Fatal("zones")
	}
	for i, want := range []struct{ id, label, color string }{{"0", "Logo", "#ffff00"}, {"1", "Scroll", "#00ffff"}, {"2", "Side", "#00ffff"}} {
		got := l.AuthoredZoneEditor.Zones[i]
		if got.ID != want.id || got.Label != want.label || got.ColorHex != want.color {
			t.Fatalf("zone %d %#v", i, got)
		}
	}
	l.AuthoredZoneEditor.Zones[0].ColorHex = "#ffffff"
	l.SupportedEffects[0].ID = "gradient"
	if second.Lighting.AuthoredZoneEditor.Zones[0].ColorHex != "#ffff00" || second.Lighting.SupportedEffects[0].ID != "mouse" {
		t.Fatal("preview shares authored state")
	}
	// Builder constructs summary literals only, without any native Device/runtime.
}
