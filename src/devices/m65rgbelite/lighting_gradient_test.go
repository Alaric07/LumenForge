package m65rgbelite

import (
	"LumenForge/src/lightingsettings"
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync/atomic"
	"testing"
)

func TestNewGradientColorReturnsFinalIndexForEditing(t *testing.T) {
	for _, test := range []struct {
		name      string
		positions []float64
		position  float64
		index     uint
	}{
		{"endpoints", []float64{0, 1}, .5, 1},
		{"uneven largest gap", []float64{0, .125, .375, 1}, .6875, 3},
		{"middle insertion", []float64{0, .25, 1}, .625, 2},
		{"first equal-size gap", []float64{0, .5, 1}, .25, 1},
		{"existing duplicates", []float64{0, .5, .5, 1}, .25, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			d, rt := lightingFixture(t, true)
			value := gradientSettings(t, d, test.positions...)
			for i := range value.Gradient.Stops {
				value.Gradient.Stops[i].Color.Red = float64(20 + i)
			}
			requireLighting(t, d.SetLightingEffectSettings("gradient", value))
			before := value.Clone()
			// This public device method is the seam called by the add API, which
			// exposes its second result as response.data.
			status, index := d.ProcessNewGradientColor("gradient")
			if status != 1 || index != test.index || index == uint(len(test.positions)) {
				t.Fatalf("status=%d index=%d; want final index %d", status, index, test.index)
			}
			stored, found, err := rt.Effects.Get(d.Serial, "gradient")
			requireLighting(t, err)
			added := lightingsettings.GradientStop{Position: test.position, Color: lightingsettings.Color{Green: 255, Blue: 255}, Intensity: 1}
			if !found || stored.Gradient.Stops[index] != added || !reflect.DeepEqual(value, before) {
				t.Fatal("returned index does not identify insertion or input changed", stored)
			}
			remaining := append([]lightingsettings.GradientStop(nil), stored.Gradient.Stops[:index]...)
			remaining = append(remaining, stored.Gradient.Stops[index+1:]...)
			if !reflect.DeepEqual(remaining, before.Gradient.Stops) {
				t.Fatal("insertion changed neighboring/existing stops")
			}
			// Exercise the existing profile read/edit API adapter using the returned
			// index, as a client of response.data could do.
			profile := d.GetRgbProfile("gradient")
			color := profile.Gradients[int(index)]
			color.Red, color.Green, color.Blue = 91, 92, 93
			profile.Gradients[int(index)] = color
			if d.UpdateRgbProfileData("gradient", *profile) != 1 {
				t.Fatal("editing returned index failed")
			}
			edited, err := d.ResolveLightingEffectSettings("gradient")
			requireLighting(t, err)
			want := stored.Clone()
			want.Gradient.Stops[index].Color = lightingsettings.Color{Red: 91, Green: 92, Blue: 93}
			if !reflect.DeepEqual(edited, want) {
				t.Fatal("returned index edited a different stop", edited)
			}
		})
	}
}

func TestGradientInsertionPreservesStableDuplicateOrderingAndCopies(t *testing.T) {
	d, _ := lightingFixture(t, true)
	value := gradientSettings(t, d, 0, .5, .5, 1)
	stops := make([]lightingsettings.GradientStop, 4, 8)
	copy(stops, value.Gradient.Stops)
	for i := range stops {
		stops[i].Color.Red = float64(i + 1)
	}
	before := append([]lightingsettings.GradientStop(nil), stops...)
	// Test an equal-position insertion directly: the largest-gap algorithm
	// ordinarily chooses an unoccupied midpoint, rather than an existing stop.
	next, index := insertLightingGradientStop(stops, .5)
	if index != 3 || next[1] != before[1] || next[2] != before[2] || next[index].Color != (lightingsettings.Color{Green: 255, Blue: 255}) {
		t.Fatal("new stop did not follow all existing equal positions", index, next)
	}
	want := append(append([]lightingsettings.GradientStop(nil), before...), next[index])
	sort.SliceStable(want, func(i, j int) bool { return want[i].Position < want[j].Position })
	if !reflect.DeepEqual(next, want) {
		t.Fatal("insertion differs from old append/stable-sort semantics")
	}
	next[0].Color = lightingsettings.Color{Blue: 13}
	if !reflect.DeepEqual(stops, before) {
		t.Fatal("insertion reused source backing slice")
	}
}

func TestNewGradientColorFailureReturnsNoUsableIndexOrPublication(t *testing.T) {
	d, rt := lightingFixture(t, true)
	value := gradientSettings(t, d, 0, 1)
	requireLighting(t, d.SetLightingEffectSettings("gradient", value))
	requireLighting(t, d.SetLightingEffect("gradient"))
	before, _ := d.LightingSnapshot()
	path := filepath.Join(filepath.Dir(d.DeviceProfile.Path), "effects.json")
	disk, err := os.ReadFile(path)
	requireLighting(t, err)
	worker := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{})}
	d.lightingWorker = worker
	native, external := d.nativeGeneration, d.externalGeneration
	var output atomic.Int32
	d.lightingWrite = func([]byte) { output.Add(1) }
	d.lightingCanonical.effects = failingEffects{rt.Effects}
	status, index := d.ProcessNewGradientColor("gradient")
	if status != 0 || index != 0 {
		t.Fatalf("failed operation returned status=%d index=%d", status, index)
	}
	actual, err := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, err)
	after, _ := d.LightingSnapshot()
	afterDisk, err := os.ReadFile(path)
	requireLighting(t, err)
	if !reflect.DeepEqual(value, actual) || !reflect.DeepEqual(before, after) || !bytes.Equal(disk, afterDisk) || d.lightingWorker != worker || d.nativeGeneration != native || d.externalGeneration != external || output.Load() != 0 {
		t.Fatal("failed insertion published state or restarted output")
	}
	select {
	case <-worker.stop:
		t.Fatal("failed insertion cancelled renderer")
	default:
	}
}

func gradientSettings(t *testing.T, d *Device, positions ...float64) lightingsettings.EffectSettings {
	t.Helper()
	value, err := d.lightingCanonical.defaults.Get("gradient")
	requireLighting(t, err)
	value.Gradient.Stops = nil
	for _, position := range positions {
		value.Gradient.Stops = append(value.Gradient.Stops, lightingsettings.GradientStop{Position: position, Color: lightingsettings.Color{Red: 128, Green: 64, Blue: 32}, Intensity: 1})
	}
	return value
}

// Observe the actual DeviceStore.Set seam rather than replacing its validator.
type gradientWriteProbe struct {
	lightingEffectStore
	calls     int
	lastError error
}

func (p *gradientWriteProbe) Set(device, effect string, value lightingsettings.EffectSettings) error {
	p.calls++
	p.lastError = p.lightingEffectStore.Set(device, effect, value)
	return p.lastError
}

func TestGradientSharedEndpointAndValidationRules(t *testing.T) {
	d, _ := lightingFixture(t, true)
	for _, test := range []struct {
		name      string
		positions []float64
		valid     bool
	}{
		{"no endpoints", []float64{.25, .75}, true},
		{"no zero", []float64{.25, 1}, true},
		{"no one", []float64{0, .75}, true},
		{"duplicate positions", []float64{.5, .5}, true},
		{"one stop", []float64{.5}, false},
		{"unordered", []float64{.75, .25}, false},
		{"negative", []float64{-.1, .5}, false},
		{"above one", []float64{0, 1.1}, false},
		{"nonfinite position", []float64{0, math.NaN()}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := gradientSettings(t, d, test.positions...)
			err := lightingsettings.Validate(value)
			if (err == nil) != test.valid {
				t.Fatalf("Validate = %v", err)
			}
		})
	}
	for _, test := range []struct {
		name   string
		change func(*lightingsettings.EffectSettings)
		valid  bool
	}{
		{"intensity zero", func(v *lightingsettings.EffectSettings) { v.Gradient.Stops[0].Intensity = 0 }, true},
		{"negative intensity", func(v *lightingsettings.EffectSettings) { v.Gradient.Stops[0].Intensity = -.1 }, false},
		{"intensity above one", func(v *lightingsettings.EffectSettings) { v.Gradient.Stops[0].Intensity = 1.1 }, false},
		{"nonfinite intensity", func(v *lightingsettings.EffectSettings) { v.Gradient.Stops[0].Intensity = math.Inf(1) }, false},
		{"fractional color", func(v *lightingsettings.EffectSettings) { v.Gradient.Stops[0].Color.Red = 127.5 }, true},
		{"negative color", func(v *lightingsettings.EffectSettings) { v.Gradient.Stops[0].Color.Red = -1 }, false},
		{"color above 255", func(v *lightingsettings.EffectSettings) { v.Gradient.Stops[0].Color.Green = 256 }, false},
		{"nonfinite color", func(v *lightingsettings.EffectSettings) { v.Gradient.Stops[0].Color.Blue = math.NaN() }, false},
		{"missing speed", func(v *lightingsettings.EffectSettings) { v.Speed = nil }, false},
		{"speed below one", func(v *lightingsettings.EffectSettings) { speed := .9; v.Speed = &speed }, false},
		{"speed above ten", func(v *lightingsettings.EffectSettings) { speed := 10.1; v.Speed = &speed }, false},
		{"nonfinite speed", func(v *lightingsettings.EffectSettings) { speed := math.NaN(); v.Speed = &speed }, false},
		{"mixed palette variants", func(v *lightingsettings.EffectSettings) { v.SingleColor = &lightingsettings.SingleColorSettings{} }, false},
		{"wrong schema", func(v *lightingsettings.EffectSettings) { v.SchemaVersion++ }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := gradientSettings(t, d, 0, 1)
			test.change(&value)
			err := lightingsettings.Validate(value)
			if (err == nil) != test.valid {
				t.Fatalf("Validate = %v", err)
			}
		})
	}
}

func TestGradientEditsPersistValidSettingsIncludingFinalStopRemoval(t *testing.T) {
	for _, remove := range []bool{false, true} {
		name := "add"
		if remove {
			name = "delete final position-one stop"
		}
		t.Run(name, func(t *testing.T) {
			d, rt := lightingFixture(t, true)
			value := gradientSettings(t, d, 0, .5, 1)
			requireLighting(t, d.SetLightingEffectSettings("gradient", value))
			requireLighting(t, d.SetLightingEffect("gradient"))
			probe := &gradientWriteProbe{lightingEffectStore: rt.Effects}
			d.lightingCanonical.effects = probe
			status, _ := d.changeLightingGradient("gradient", remove)
			if status != 1 || probe.calls != 1 || probe.lastError != nil {
				t.Fatalf("edit status=%d calls=%d err=%v", status, probe.calls, probe.lastError)
			}
			stored, found, err := rt.Effects.Get(d.Serial, "gradient")
			requireLighting(t, err)
			requireLighting(t, lightingsettings.Validate(stored))
			want := 4
			if remove {
				want = 2
			}
			if !found || len(stored.Gradient.Stops) != want {
				t.Fatal("edit was not persisted", stored)
			}
			if remove && stored.Gradient.Stops[1].Position != .5 {
				t.Fatal("unexpected final stop", stored)
			}
			reloaded, err := lightingsettings.LoadDeviceStore(filepath.Join(filepath.Dir(d.DeviceProfile.Path), "effects.json"))
			requireLighting(t, err)
			disk, found, err := reloaded.Get(d.Serial, "gradient")
			requireLighting(t, err)
			if !found || !reflect.DeepEqual(stored, disk) {
				t.Fatal("durable edit differs")
			}
			// An attempted delete from two stops remains a non-mutating minimum-count result.
			if remove {
				status, _ = d.changeLightingGradient("gradient", true)
				if status != 2 || probe.calls != 1 {
					t.Fatal("minimum-stop delete wrote settings")
				}
			}
		})
	}
}

// Simulate already-loaded corruption below DeviceStore's public validator.
// Recovery still uses Delete's error-returning persistence seam, then defaults.
type corruptEffects struct {
	lightingEffectStore
	bad     lightingsettings.EffectSettings
	present bool
	fail    bool
}

func (s *corruptEffects) Delete(device, effect string) (bool, error) {
	if s.fail {
		return false, fmt.Errorf("reset failure")
	}
	found, err := s.lightingEffectStore.Delete(device, effect)
	if err == nil && effect == "gradient" {
		s.present = false
	}
	return found, err
}

type corruptResolver struct {
	store    *corruptEffects
	fallback lightingResolver
}

func (r corruptResolver) Resolve(target lightingsettings.Target, effect string) (lightingsettings.Resolution, error) {
	if effect == "gradient" && r.store.present {
		return lightingsettings.Resolution{Settings: r.store.bad.Clone(), Customized: true}, nil
	}
	return r.fallback.Resolve(target, effect)
}
func TestTargetedResetRecoversCorruptGradientAndFailureIsInert(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			d, rt := lightingFixture(t, true)
			value := gradientSettings(t, d, 0, 1)
			requireLighting(t, d.SetLightingEffectSettings("gradient", value))
			requireLighting(t, d.SetLightingEffect("gradient"))
			unrelated, err := rt.Defaults.Get("static")
			requireLighting(t, err)
			unrelated.SingleColor.Color.Red = 37
			requireLighting(t, d.SetLightingEffectSettings("static", unrelated))
			value.Gradient.Stops = value.Gradient.Stops[:1]
			if d.SetLightingEffectSettings("gradient", value) == nil {
				t.Fatal("invalid settings accepted")
			}
			store := &corruptEffects{lightingEffectStore: rt.Effects, bad: value, present: true, fail: failure}
			d.lightingCanonical.effects = store
			d.lightingCanonical.resolver = corruptResolver{store, rt.Resolver}
			if _, ok := d.LightingSnapshot(); ok {
				t.Fatal("ordinary validation accepted corruption")
			}
			worker := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{})}
			d.lightingWorker = worker
			before := inputJSON(t, d)
			err = d.ResetLightingEffectSettings("gradient")
			if failure {
				if err == nil || !store.present || d.lightingWorker != worker {
					t.Fatal("failed reset published")
				}
			} else {
				requireLighting(t, err)
				if store.present {
					t.Fatal("corruption remained")
				}
				if _, ok := d.LightingSnapshot(); !ok {
					t.Fatal("not recovered")
				}
				if _, found, e := rt.Effects.Get(d.Serial, "gradient"); e != nil || found {
					t.Fatal("reset wrote defaults as customization")
				}
				resolved, e := d.ResolveLightingEffectSettings("gradient")
				requireLighting(t, e)
				defaults, e := rt.Defaults.Get("gradient")
				requireLighting(t, e)
				if !reflect.DeepEqual(resolved, defaults) {
					t.Fatal(resolved)
				}
			}
			other, _, e := rt.Effects.Get(d.Serial, "static")
			requireLighting(t, e)
			if !reflect.DeepEqual(other, unrelated) || !bytes.Equal(before, inputJSON(t, d)) {
				t.Fatal("reset changed unrelated state")
			}
		})
	}
}
