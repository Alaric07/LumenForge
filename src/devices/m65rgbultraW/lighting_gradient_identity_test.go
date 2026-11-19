package m65rgbultraW

import (
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestUltraGradientMapKeysPersistAcrossSortingAndReload(t *testing.T) {
	d, rt := lightingFixture(t, true)
	p := d.GetRgbProfile("gradient")
	p.Gradients = map[int]rgb.Color{0: {Red: 10, Position: 0, Brightness: 1}, 1: {Red: 20, Position: 1, Brightness: 1}}
	if d.UpdateRgbProfileData("gradient", *p) != 1 {
		t.Fatal("seed")
	}
	status, key := d.ProcessNewGradientColor("gradient")
	if status != 1 || key != 2 {
		t.Fatal(status, key)
	}
	p = d.GetRgbProfile("gradient")
	if p.Gradients[2] != (rgb.Color{Green: 255, Blue: 255}) {
		t.Fatal("key is not the new source-default color", p.Gradients)
	}
	// Position sorting places key 2 ahead of key 1; its identity must not change.
	stored, _, err := rt.Effects.Get(d.Serial, "gradient")
	requireLighting(t, err)
	if !reflect.DeepEqual(stored.Gradient.Keys, []int{0, 2, 1}) {
		t.Fatal(stored)
	}
	p.Gradients[2] = rgb.Color{Red: 93, Position: .25, Brightness: .7}
	if d.UpdateRgbProfileData("gradient", *p) != 1 {
		t.Fatal("edit returned key")
	}
	if d.GetRgbProfile("gradient").Gradients[2].Red != 93 || d.GetRgbProfile("gradient").Gradients[1].Red != 20 {
		t.Fatal("wrong key edited")
	}
	effects, err := lightingsettings.LoadDeviceStore(filepath.Join(filepath.Dir(d.DeviceProfile.Path), "effects.json"))
	requireLighting(t, err)
	resolver, err := lightingsettings.NewDeviceResolver(rt.Defaults, effects)
	requireLighting(t, err)
	value, err := resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), "gradient")
	requireLighting(t, err)
	if !reflect.DeepEqual(wirelessRendererProfile(value.Settings).Gradients, d.GetRgbProfile("gradient").Gradients) {
		t.Fatal("key identities lost on reload")
	}
	status, key = d.ProcessDeleteGradientColor("gradient")
	if status != 1 || key != 2 {
		t.Fatal(status, key)
	}
	if _, ok := d.GetRgbProfile("gradient").Gradients[2]; ok {
		t.Fatal("wrong key removed")
	}
	status, key = d.ProcessNewGradientColor("gradient")
	if status != 1 || key != 2 {
		t.Fatal("source maxKey+1 allocation changed", status, key)
	}
	before := d.GetRgbProfile("gradient")
	d.lightingCanonical.effects = failingEffects{rt.Effects}
	if status, key = d.ProcessNewGradientColor("gradient"); status != 0 || key != 0 {
		t.Fatal(status, key)
	}
	if !reflect.DeepEqual(before, d.GetRgbProfile("gradient")) {
		t.Fatal("failed add published")
	}
}

func TestUltraGradientEqualPositionsKeepNumericKeyInputOrder(t *testing.T) {
	d, _ := lightingFixture(t, true)
	p := d.GetRgbProfile("gradient")
	p.Gradients = map[int]rgb.Color{0: {Red: 1, Position: .5, Brightness: 1}, 1: {Red: 2, Position: .5, Brightness: 1}, 2: {Red: 3, Position: 0, Brightness: 1}}
	if d.UpdateRgbProfileData("gradient", *p) != 1 {
		t.Fatal("valid duplicate positions")
	}
	q := d.GetRgbProfile("gradient")
	if !reflect.DeepEqual(p.Gradients, q.Gradients) {
		t.Fatal("map identities changed", q.Gradients)
	}
	// Canonical conversion restores the original numeric-key map. The existing
	// rgb Gradient renderer remains the authority for equal-position rendering.
	before := copyLightingRGBProfile(*q)
	r := rgb.New(2, 1, nil, nil, 1, 1, 0, true)
	r.ColorshiftGradient(time.Now(), q.Gradients, 1)
	if !reflect.DeepEqual(before.Gradients, q.Gradients) {
		t.Fatal("render mutated stops")
	}
}
