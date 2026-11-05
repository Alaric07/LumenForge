package m65rgbultra

import (
	"LumenForge/src/lightingsettings"
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

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
