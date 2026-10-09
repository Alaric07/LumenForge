package server

import (
	"LumenForge/src/keyboards"
	"LumenForge/src/lightingpresentation"
	"reflect"
	"strings"
	"testing"
)

func TestKeyboardLightingCanvasUsesGeometryNotZoneIdentity(t *testing.T) {
	// Sparse output identities must not create columns or reorder keys. The gap
	// at column 3 is intentionally absent from the zone list.
	zones := []lightingpresentation.AuthoredZone{
		{ID: "137", Label: "One", HasGeometry: true, Left: 0, Top: 0, Width: 100, Height: 100},
		{ID: "0", Label: "Two", HasGeometry: true, Left: 100, Top: 0, Width: 200, Height: 100},
		{ID: "139", Label: "Next row", HasGeometry: true, Left: 400, Top: 100, Width: 100, Height: 100},
	}
	build := func(effect string) *devicesLightingAuthoredZoneEditorSummary {
		return devicesLightingWorkspaceSummaryFromSnapshot(lightingpresentation.Snapshot{
			TargetKind: "native", ConfiguredEffect: effect,
			AuthoredZoneEditor: &lightingpresentation.AuthoredZoneEditor{EffectID: effect, Zones: zones},
		}).AuthoredZoneEditor
	}
	got := build("keyboard")
	if !got.KeyboardGeometry || got.KeyboardUnit != 100 || got.LayoutWidth != 500 || got.LayoutHeight != 200 || len(got.Zones) != 3 {
		t.Fatalf("canvas: %#v", got)
	}
	for i, z := range zones {
		if got.Zones[i].ID != z.ID || got.Zones[i].Left != z.Left || got.Zones[i].Top != z.Top || got.Zones[i].Width != z.Width || got.Zones[i].Height != z.Height {
			t.Fatalf("reordered or changed geometry: %#v", got.Zones)
		}
	}
	zones[0].ID, zones[1].ID, zones[2].ID = "0", "999", "1"
	renamed := build("keyboard")
	if renamed.KeyboardUnit != got.KeyboardUnit || renamed.LayoutWidth != got.LayoutWidth || renamed.LayoutHeight != got.LayoutHeight {
		t.Fatal("identity affected canvas sizing")
	}
	if build("mousepad").KeyboardGeometry {
		t.Fatal("keyboard sizing applied to another geometric editor")
	}
	for i := range zones {
		zones[i].HasGeometry = false
	}
	if build("keyboard").KeyboardGeometry {
		t.Fatal("manufactured a spatial keyboard from a zone list")
	}
}

func TestK65LightingSharedKeyboardPresentation(t *testing.T) {
	keyboards.Init()
	for _, build := range []func() *devicesWorkspaceSummary{buildK65RGBModernPreview, buildK65RGBRapidfireModernPreview} {
		summary := build()
		editor := summary.Lighting.AuthoredZoneEditor
		if !editor.KeyboardGeometry || editor.KeyboardUnit != 100 || editor.LayoutWidth != 2000 || editor.LayoutHeight != 700 || len(editor.Zones) != 92 {
			t.Fatalf("keyboard canvas: %#v", editor)
		}
		// Independent source-backed anchors cover empty cells, wide spans and rows.
		want := map[string][4]int{
			"1": {1200, 0, 100, 100}, "27": {0, 100, 100, 100},
			"28": {200, 100, 100, 100}, "61": {1300, 200, 300, 100},
			"70": {0, 300, 200, 100}, "121": {1800, 500, 100, 100},
			"130": {500, 600, 600, 100}, "135": {1700, 600, 100, 100},
		}
		groups := map[string]bool{}
		for _, z := range editor.Zones {
			groups[z.GroupID] = true
			if expected, ok := want[z.ID]; ok {
				if [4]int{z.Left, z.Top, z.Width, z.Height} != expected {
					t.Fatalf("key %s: %#v, want %v", z.ID, z, expected)
				}
				delete(want, z.ID)
			}
			if z.Top == 0 && z.Left < 1200 || z.Top == 100 && z.Left == 100 {
				t.Fatalf("placeholder rendered as a key: %#v", z)
			}
		}
		if len(want) != 0 || len(groups) != 7 {
			t.Fatalf("missing anchors/groups: %v, %v", want, groups)
		}
		before := append([]devicesLightingAuthoredZoneSummary(nil), editor.Zones...)
		body := renderDevicesLightingViewForSerial(t, summary.Serial, summary.Lighting)
		for _, marker := range []string{"lf-authored-keyboard-scroll", "lf-authored-zone-list-keyboard", "--lf-authored-key-unit: 100", "--lf-authored-zone-layout-width: 2000"} {
			if !strings.Contains(body, marker) {
				t.Fatalf("missing %s", marker)
			}
		}
		if strings.Count(body, "data-lf-authored-zone data-lf-zone-id=") != 92 || !reflect.DeepEqual(editor.Zones, before) {
			t.Fatal("rendering changed authored zones or manufactured placeholder buttons")
		}
	}
}
