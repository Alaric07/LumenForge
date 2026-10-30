package harpoonW

import (
	"LumenForge/src/lightingsettings"
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
)

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

func TestGradientOverflowIsRejectedByActualStoreBeforePublication(t *testing.T) {
	d, rt := lightingFixture(t, true)
	value := gradientSettings(t, d)
	for i := 0; i < 1024; i++ {
		value.Gradient.Stops = append(value.Gradient.Stops, lightingsettings.GradientStop{Position: float64(i) / 1023, Color: lightingsettings.Color{Red: 128}, Intensity: 1})
	}
	requireLighting(t, lightingsettings.Validate(value))
	requireLighting(t, d.SetLightingEffectSettings("gradient", value))
	requireLighting(t, d.SetLightingEffect("gradient"))
	probe := &gradientWriteProbe{lightingEffectStore: rt.Effects}
	d.lightingCanonical.effects = probe
	path := filepath.Join(filepath.Dir(d.DeviceProfile.Path), "effects.json")
	before, err := os.ReadFile(path)
	requireLighting(t, err)
	stateBefore, _, err := rt.State.Resolve(d.Serial)
	requireLighting(t, err)
	inputBefore := inputJSON(t, d)
	worker := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{}), generation: d.nativeGeneration}
	d.lightingWorker = worker
	d.Connected, d.lightingHardwareReady = true, true
	var writes atomic.Int32
	d.lightingWrite = func([]byte) { writes.Add(1) }
	session := d.externalGeneration
	status, _ := d.changeLightingGradient("gradient", false)
	if status != 0 || probe.calls != 1 || !errors.Is(probe.lastError, lightingsettings.ErrInvalidSettings) {
		t.Fatalf("invalid edit status=%d calls=%d err=%v", status, probe.calls, probe.lastError)
	}
	after, err := os.ReadFile(path)
	requireLighting(t, err)
	stored, _, err := rt.Effects.Get(d.Serial, "gradient")
	requireLighting(t, err)
	stateAfter, _, err := rt.State.Resolve(d.Serial)
	requireLighting(t, err)
	if !bytes.Equal(before, after) || !reflect.DeepEqual(value, stored) || stateBefore != stateAfter || !bytes.Equal(inputBefore, inputJSON(t, d)) || d.lightingWorker != worker || d.externalGeneration != session || writes.Load() != 0 {
		t.Fatal("invalid edit persisted/published/restarted/output")
	}
	select {
	case <-worker.stop:
		t.Fatal("invalid edit cancelled worker")
	default:
	}
}

// DeviceStore.Set and LoadDeviceStore reject invalid records. Inject corruption
// below that public boundary using the existing package-local store/resolver test
// seams: Get can now return a previously corrupted loaded customization. Valid
// recovery writes still use the real shared store's validator and persistence.
type corruptLoadedEffects struct {
	lightingEffectStore
	corrupt map[string]lightingsettings.EffectSettings
	failure error
	calls   int
}

func (s *corruptLoadedEffects) Get(device, effect string) (lightingsettings.EffectSettings, bool, error) {
	if value, ok := s.corrupt[effect]; ok {
		return value.Clone(), true, nil
	}
	return s.lightingEffectStore.Get(device, effect)
}
func (s *corruptLoadedEffects) Set(device, effect string, value lightingsettings.EffectSettings) error {
	s.calls++
	if s.failure != nil {
		return s.failure
	}
	if err := s.lightingEffectStore.Set(device, effect, value); err != nil {
		return err
	}
	delete(s.corrupt, effect)
	return nil
}

type corruptLoadedResolver struct {
	source   *corruptLoadedEffects
	fallback lightingResolver
}

func (r corruptLoadedResolver) Resolve(target lightingsettings.Target, effect string) (lightingsettings.Resolution, error) {
	value, found, err := r.source.Get(target.ID, effect)
	if err != nil {
		return lightingsettings.Resolution{}, err
	}
	if found {
		return lightingsettings.Resolution{Settings: value, Customized: true}, nil
	}
	return r.fallback.Resolve(target, effect)
}

func TestExplicitResetRepairsCorruptionWithoutCatalogueValidation(t *testing.T) {
	for _, owner := range []string{"native", "cluster", "openrgb", "unselected", "persistence failure"} {
		t.Run(owner, func(t *testing.T) {
			d, rt := lightingFixture(t, true)
			defaults, err := rt.Defaults.Get("gradient")
			requireLighting(t, err)
			requireLighting(t, d.SetLightingEffect("gradient"))
			other, err := rt.Defaults.Get("static")
			requireLighting(t, err)
			other.SingleColor.Color.Red = 37
			requireLighting(t, d.SetLightingEffectSettings("static", other))
			if owner == "unselected" {
				requireLighting(t, d.SetLightingEffect("static"))
			}
			bad := defaults.Clone()
			bad.Gradient.Stops = bad.Gradient.Stops[:1]
			requireInvalid := lightingsettings.Validate(bad)
			if requireInvalid == nil {
				t.Fatal("corruption fixture is valid")
			}
			store := &corruptLoadedEffects{lightingEffectStore: rt.Effects, corrupt: map[string]lightingsettings.EffectSettings{"gradient": bad}}
			d.lightingCanonical.effects = store
			d.lightingCanonical.resolver = corruptLoadedResolver{source: store, fallback: rt.Resolver}
			if err := d.validateLightingSource(d.lightingCanonical); err == nil {
				t.Fatal("ordinary validation accepted corruption")
			}
			if _, ok := d.LightingSnapshot(); ok {
				t.Fatal("snapshot accepted corruption")
			}
			if d.SetLightingBrightness(19) == nil {
				t.Fatal("ordinary mutation ignored corruption")
			}
			worker := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{}), generation: d.nativeGeneration}
			d.lightingWorker = worker
			session := d.externalGeneration
			inputBefore := inputJSON(t, d)
			path := filepath.Join(filepath.Dir(d.DeviceProfile.Path), "effects.json")
			diskBefore, err := os.ReadFile(path)
			requireLighting(t, err)
			frames := make(chan []byte, 10)
			d.lightingWrite = func(p []byte) { frames <- p }
			d.Connected, d.lightingHardwareReady = true, true
			d.clusterOwned = owner == "cluster"
			d.openRGBOwned = owner == "openrgb"
			if owner == "persistence failure" {
				store.failure = errors.New("injected recovery persistence failure")
			}
			err = d.ResetLightingEffectSettings("gradient")
			if store.calls != 1 {
				t.Fatal("reset did not reach the target write seam")
			}
			if owner == "persistence failure" {
				if !errors.Is(err, store.failure) {
					t.Fatal("reset swallowed failure", err)
				}
				stored, _, getErr := store.Get(d.Serial, "gradient")
				requireLighting(t, getErr)
				if !reflect.DeepEqual(stored, bad) || d.lightingWorker != worker {
					t.Fatal("failed reset changed corrupt value or renderer")
				}
				if _, ok := d.LightingSnapshot(); ok {
					t.Fatal("failed reset concealed corruption")
				}
				diskAfter, readErr := os.ReadFile(path)
				requireLighting(t, readErr)
				if !bytes.Equal(diskBefore, diskAfter) {
					t.Fatal("failed reset changed durable settings")
				}
				noFrame(t, frames)
			} else {
				requireLighting(t, err)
				stored, found, getErr := store.Get(d.Serial, "gradient")
				requireLighting(t, getErr)
				if !found || !reflect.DeepEqual(stored, defaults) {
					t.Fatal("reset did not restore exact canonical default")
				}
				requireLighting(t, lightingsettings.Validate(stored))
				reloaded, loadErr := lightingsettings.LoadDeviceStore(path)
				requireLighting(t, loadErr)
				disk, diskFound, getErr := reloaded.Get(d.Serial, "gradient")
				requireLighting(t, getErr)
				if !diskFound || !reflect.DeepEqual(disk, defaults) {
					t.Fatal("reset default was not durable")
				}
				if _, ok := d.LightingSnapshot(); !ok {
					t.Fatal("reset did not recover ordinary snapshot")
				}
				if owner == "native" {
					waitFrame(t, frames)
					select {
					case <-worker.stop:
					default:
						t.Fatal("selected Native effect did not restart")
					}
				} else {
					if d.lightingWorker != worker {
						t.Fatal("reset disturbed unselected/external owner")
					}
					noFrame(t, frames)
				}
			}
			unrelated, found, getErr := rt.Effects.Get(d.Serial, "static")
			requireLighting(t, getErr)
			if !found || !reflect.DeepEqual(other, unrelated) || d.externalGeneration != session || !bytes.Equal(inputBefore, inputJSON(t, d)) {
				t.Fatal("target reset changed unrelated settings/input/session")
			}
			if owner != "native" {
				select {
				case <-worker.stop:
					t.Fatal("reset cancelled unrelated/failed output")
				default:
				}
			}
		})
	}
}

func TestResetDoesNotReadUnrelatedCorruptCustomizations(t *testing.T) {
	d, rt := lightingFixture(t, true)
	gradient, err := rt.Defaults.Get("gradient")
	requireLighting(t, err)
	badGradient := gradient.Clone()
	badGradient.Gradient.Stops = nil
	badStatic, err := rt.Defaults.Get("static")
	requireLighting(t, err)
	badStatic.SingleColor.Color.Red = 256
	store := &corruptLoadedEffects{lightingEffectStore: rt.Effects, corrupt: map[string]lightingsettings.EffectSettings{"gradient": badGradient, "static": badStatic}}
	d.lightingCanonical.effects = store
	d.lightingCanonical.resolver = corruptLoadedResolver{source: store, fallback: rt.Resolver}
	requireLighting(t, d.ResetLightingEffectSettings("gradient"))
	value, _, err := store.Get(d.Serial, "gradient")
	requireLighting(t, err)
	if !reflect.DeepEqual(value, gradient) {
		t.Fatal("target did not recover")
	}
	value, _, err = store.Get(d.Serial, "static")
	requireLighting(t, err)
	if !reflect.DeepEqual(value, badStatic) {
		t.Fatal("unrelated corruption silently repaired")
	}
	if _, ok := d.LightingSnapshot(); ok {
		t.Fatal("ordinary validation was weakened for unrelated corruption")
	}
}
