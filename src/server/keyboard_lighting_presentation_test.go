package server

import (
	"LumenForge/src/devices"
	"LumenForge/src/keyboards"
	"LumenForge/src/lightingpresentation"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
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

// Independent expected row tables (key ID / column / span), rather than a
// second implementation of the placeholder algorithm. DE differs in rows 4-6.
var k65ExpectedRows = []string{
	"1/12/1 2/13/1 3/14/1 4/15/1 5/17/1",
	"27/0/1 28/2/1 29/3/1 30/4/1 31/5/1 32/7/1 33/8/1 34/9/1 35/10/1 36/12/1 37/13/1 38/14/1 39/15/1 40/17/1 41/18/1 42/19/1",
	"48/0/1 49/1/1 50/2/1 51/3/1 52/4/1 53/5/1 54/6/1 55/7/1 56/8/1 57/9/1 58/10/1 59/11/1 60/12/1 61/13/3 62/17/1 63/18/1 64/19/1",
	"70/0/2 71/2/1 72/3/1 73/4/1 74/5/1 75/6/1 76/7/1 77/8/1 78/9/1 79/10/1 80/11/1 81/12/1 82/13/1 83/14/2 84/17/1 85/18/1 86/19/1",
	"92/0/2 93/2/1 94/3/1 95/4/1 96/5/1 97/6/1 98/7/1 99/8/1 100/9/1 101/10/1 102/11/1 103/12/1 104/13/3",
	"109/0/3 110/3/1 111/4/1 112/5/1 113/6/1 114/7/1 115/8/1 116/9/1 117/10/1 118/11/1 119/12/1 120/13/3 121/18/1",
	"127/0/2 128/2/1 129/3/2 130/5/6 131/11/1 132/12/1 133/13/1 134/14/2 135/17/1 136/18/1 137/19/1",
}

func TestK65BothLayoutsPreserveEveryKeyAndGap(t *testing.T) {
	keyboards.Init()
	firstByLayout := map[string]*devicesWorkspaceSummary{}
	for _, build := range []func(string, string) *devicesWorkspaceSummary{buildK65RGBLayoutModernPreview, buildK65RGBRapidfireLayoutModernPreview} {
		for _, layout := range []string{"US", "DE"} {
			summary := build("geometry-test", layout)
			t.Run(summary.Product+"/"+layout, func(t *testing.T) {
				editor := summary.Lighting.AuthoredZoneEditor
				rows := append([]string(nil), k65ExpectedRows...)
				wantCount, enterID := 92, "104"
				if layout == "DE" {
					wantCount, enterID = 93, "105"
					rows[3] = "70/0/2 71/2/1 72/3/1 73/4/1 74/5/1 75/6/1 76/7/1 77/8/1 78/9/1 79/10/1 80/11/1 81/12/1 82/13/1 84/17/1 85/18/1 86/19/1"
					rows[4] = "92/0/2 93/2/1 94/3/1 95/4/1 96/5/1 97/6/1 98/7/1 99/8/1 100/9/1 101/10/1 102/11/1 103/12/1 104/13/1 105/14/2"
					rows[5] = "109/0/2 110/2/1 111/3/1 112/4/1 113/5/1 114/6/1 115/7/1 116/8/1 117/9/1 118/10/1 119/11/1 120/12/1 121/13/3 122/18/1"
				}
				if len(editor.Zones) != wantCount || editor.LayoutWidth != 2000 || editor.LayoutHeight != 700 || editor.KeyboardUnit != 100 || !editor.KeyboardGeometry {
					t.Fatalf("canvas/count: %#v", editor)
				}
				index := 0
				for r, row := range rows {
					for _, token := range strings.Fields(row) {
						var id, column, span int
						if _, err := fmt.Sscanf(token, "%d/%d/%d", &id, &column, &span); err != nil {
							t.Fatal(err)
						}
						z := editor.Zones[index]
						top, height := r*100, 100
						if layout == "DE" && strconv.Itoa(id) == enterID {
							top, height = 300, 200
						}
						if z.ID != strconv.Itoa(id) || z.GroupID != strconv.Itoa(r+1) || z.GroupLabel != fmt.Sprintf("Row %d", r+1) || !z.HasGeometry || [4]int{z.Left, z.Top, z.Width, z.Height} != [4]int{column * 100, top, span * 100, height} {
							t.Fatalf("key %s out of order or changed: %#v", token, z)
						}
						if z.ID == enterID && z.Label != "Enter" {
							t.Fatal("lost Enter identity")
						}
						index++
					}
				}
				if index != wantCount {
					t.Fatal("row count/table mismatch")
				}
				// Multi-row Enter must occupy only the reserved empty columns 14-15
				// in row 4, without colliding with +, #, or navigation keys.
				for i, a := range editor.Zones {
					for _, b := range editor.Zones[i+1:] {
						if a.Left < b.Left+b.Width && b.Left < a.Left+a.Width && a.Top < b.Top+b.Height && b.Top < a.Top+a.Height {
							t.Fatalf("overlap: %s and %s", a.ID, b.ID)
						}
					}
				}
				body := renderDevicesLightingViewForSerial(t, summary.Serial, summary.Lighting)
				start := strings.Index(body, `data-lf-zone-id="`+enterID+`"`)
				if start < 0 {
					t.Fatal("Enter missing from HTML")
				}
				button := body[start : start+strings.Index(body[start:], "</button>")]
				wantTop, wantHeight, wantWidth := "400", "100", "300"
				if layout == "DE" {
					wantTop, wantHeight, wantWidth = "300", "200", "200"
				}
				for _, marker := range []string{"--lf-authored-zone-top: " + wantTop, "--lf-authored-zone-height: " + wantHeight, "--lf-authored-zone-width: " + wantWidth} {
					if !strings.Contains(button, marker) {
						t.Fatalf("lost Enter bounds: %s", button)
					}
				}
				if strings.Count(body, "data-lf-authored-zone data-lf-zone-id=") != wantCount || !strings.Contains(body, "lf-authored-zone-list-keyboard") {
					t.Fatal("lost shared keyboard rendering or created placeholder buttons")
				}
				sourceKey := "k65rgb-default-" + layout
				if summary.Product == "K65 RGB RAPIDFIRE" {
					sourceKey = "k65rgbRF-default-" + layout
				}
				for _, z := range editor.Zones {
					row, _ := strconv.Atoi(z.GroupID)
					id, _ := strconv.Atoi(z.ID)
					if z.Label != keyboards.GetKeyboard(sourceKey).Row[row].Keys[id].KeyName {
						t.Fatalf("lost package source label: %#v", z)
					}
				}
				if first := firstByLayout[layout]; first != nil {
					if first.Product == summary.Product {
						t.Fatal("package identities collapsed")
					}
					for i, z := range editor.Zones {
						other := first.Lighting.AuthoredZoneEditor.Zones[i]
						// DE Print Screen legitimately differs: Druck vs Drucken.
						z.Label, other.Label = "", ""
						if z != other {
							t.Fatalf("package identity affected geometry: %#v / %#v", z, other)
						}
					}
				} else {
					firstByLayout[layout] = summary
				}
			})
		}
	}
}

func TestK65DEPreviewRoutesAreDistinctAndInert(t *testing.T) {
	router := legacyDevicePreviewRouter(t, true)
	keyboards.Init()
	serials := map[string]bool{}
	for _, key := range []string{"k65-rgb-de-modern", "k65-rgb-rapidfire-de-modern"} {
		fixture, ok := modernDevicePreviewFixtureByKey(key)
		if !ok {
			t.Fatal("missing DE fixture", key)
		}
		s := fixture.Build()
		if serials[s.Serial] || s.LegacyLighting || s.Lighting.ConfiguredEffect != "keyboard" || s.Lighting.Brightness != 70 || len(s.Lighting.AuthoredZoneEditor.Zones) != 93 || s.KeyboardAssignments.ActiveKeyboardLayout != "DE" {
			t.Fatal("wrong DE fixture", key)
		}
		serials[s.Serial] = true
		r := httptest.NewRecorder()
		router.ServeHTTP(r, legacyDevicePreviewRequest(http.MethodGet, "/dev/device-preview/"+key+"?view=lighting"))
		if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), "--lf-authored-zone-height: 200") || !strings.Contains(r.Header().Get("Content-Security-Policy"), "script-src 'none'") || devices.GetDevice(s.Serial) != nil {
			t.Fatal("DE preview lost geometry/inertness", key)
		}
	}
}
