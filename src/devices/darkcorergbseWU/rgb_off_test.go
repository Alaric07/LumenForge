package darkcorergbseWU

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sync/atomic"
	"testing"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

func persistedRGBOffProfile(t *testing.T, path string) DeviceProfile {
	t.Helper()
	data, err := os.ReadFile(path)
	requireLighting(t, err)
	var profile DeviceProfile
	requireLighting(t, json.Unmarshal(data, &profile))
	return profile
}

func TestRGBOffAttachmentSeedsOnlyPersistedUserPreference(t *testing.T) {
	for _, off := range []bool{false, true} {
		t.Run(map[bool]string{false: "on", true: "off"}[off], func(t *testing.T) {
			d, rt, _ := lightingFixture(t, false)
			d.DeviceProfile.RgbOff = off
			requireLighting(t, writeRGBOffPreference(d.DeviceProfile.Path, inputJSON(t, d)))
			d.loadDeviceProfiles() // same profile load used before production attachment
			requireLighting(t, d.attachLightingSource(rt))
			if d.userRGBOff != off || d.schedulerDark {
				t.Fatal("wrong override seed")
			}
			snap, ok := d.LightingSnapshot()
			if !ok || snap.ConfiguredEffect != "static" || snap.Brightness != 100 {
				t.Fatal("legacy authored state was imported")
			}
			want := [3]byte{0, 255, 255}
			if off {
				want = [3]byte{}
			}
			for _, zone := range lightingFrame(t, d) {
				if zone.rgb != want {
					t.Fatal("persisted preference not effective")
				}
			}
		})
	}
}

func TestControlRGBOffPersistsBeforePublication(t *testing.T) {
	d, _, _ := lightingFixture(t, true)
	var writes atomic.Int32
	d.lightingWrite = func([]byte) error { writes.Add(1); return nil }
	d.setLightingAvailable(true)
	for _, next := range []bool{true, false} {
		prior := !next
		before := writes.Load()
		called := false
		d.lightingRGBOffWrite = func(path string, data []byte) error {
			called = true
			// The seam is called under lightingMu, before either bool or the active
			// apply changes. Inspect without attempting to reacquire that mutex.
			if d.userRGBOff != prior || d.DeviceProfile.RgbOff != prior || writes.Load() != before {
				t.Fatal("published/output before preference persistence")
			}
			var candidate DeviceProfile
			requireLighting(t, json.Unmarshal(data, &candidate))
			if candidate.RgbOff != next {
				t.Fatal("wrong proposal")
			}
			return writeRGBOffPreference(path, data)
		}
		d.ControlDeviceRgb(next)
		if !called || d.userRGBOff != next || d.DeviceProfile.RgbOff != next || persistedRGBOffProfile(t, d.DeviceProfile.Path).RgbOff != next {
			t.Fatal("preference not committed")
		}
		if writes.Load() != before+3 {
			t.Fatal("committed preference did not apply")
		}
	}
}

func TestRGBOffPersistenceFailureLeavesPreferenceAndOutputUntouched(t *testing.T) {
	for _, prior := range []bool{false, true} {
		t.Run(map[bool]string{false: "disable-fails", true: "enable-fails"}[prior], func(t *testing.T) {
			d, _, _ := lightingFixture(t, true)
			requireLighting(t, d.setUserRGBOff(prior))
			// Ensure both cases start with a real persisted document.
			requireLighting(t, writeRGBOffPreference(d.DeviceProfile.Path, inputJSON(t, d)))
			before, err := os.ReadFile(d.DeviceProfile.Path)
			requireLighting(t, err)
			input := inputJSON(t, d)
			snap, _ := d.LightingSnapshot()
			frame := lightingFrame(t, d)
			var writes atomic.Int32
			d.lightingWrite = func([]byte) error { writes.Add(1); return nil }
			d.setLightingAvailable(true)
			owner := &lightingApply{stop: make(chan struct{})}
			d.lightingApply = owner
			failure := errors.New("injected preference persistence failure")
			d.lightingRGBOffWrite = func(string, []byte) error { return failure }
			if err := d.setUserRGBOff(!prior); !errors.Is(err, failure) {
				t.Fatal("write error hidden", err)
			}
			d.ControlDeviceRgb(!prior) // void compatibility method must also retain state
			after, err := os.ReadFile(d.DeviceProfile.Path)
			requireLighting(t, err)
			got, _ := d.LightingSnapshot()
			if !bytes.Equal(before, after) || !bytes.Equal(input, inputJSON(t, d)) || d.userRGBOff != prior || d.lightingApply != owner || writes.Load() != 0 || !reflect.DeepEqual(snap, got) || frame != lightingFrame(t, d) {
				t.Fatal("failed save published/cancelled/wrote output")
			}
			select {
			case <-owner.stop:
				t.Fatal("failed save cancelled active apply")
			default:
			}
		})
	}
}

func TestRGBOffActualProfileWriteFailureIsAtomic(t *testing.T) {
	d, _, _ := lightingFixture(t, true)
	// Renaming a temporary file over a directory fails even under root.
	requireLighting(t, os.Remove(d.DeviceProfile.Path))
	requireLighting(t, os.Mkdir(d.DeviceProfile.Path, 0700))
	if err := d.setUserRGBOff(true); err == nil {
		t.Fatal("real preference write failure hidden")
	}
	if d.userRGBOff || d.DeviceProfile.RgbOff {
		t.Fatal("rename failure published preference")
	}
}

func TestRGBOffRestartReconnectAndSchedulerIndependence(t *testing.T) {
	d, rt, paths := lightingFixture(t, true)
	requireLighting(t, d.SetLightingBrightness(70))
	if d.SchedulerBrightness(0) != 1 {
		t.Fatal("scheduler disconnected rejected")
	}
	// Scheduler changes must never create or modify a preference file.
	original, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	requireLighting(t, d.setUserRGBOff(true))
	saved, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	if bytes.Equal(original, saved) || !persistedRGBOffProfile(t, d.DeviceProfile.Path).RgbOff {
		t.Fatal("disconnected preference not persisted")
	}
	requireLighting(t, d.SetLightingBrightness(40))
	if d.SchedulerBrightness(1) != 1 || lightingFrame(t, d)[0].rgb != [3]byte{} {
		t.Fatal("scheduler cleared user preference")
	}
	afterScheduler, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	if !bytes.Equal(saved, afterScheduler) {
		t.Fatal("scheduler persisted into profile")
	}
	// Reopen canonical stores and profile as a fresh process would; do not use
	// the cached runtime or copy either runtime override into the new Device.
	state, err := lightingsettings.LoadIndependentDeviceStateStore(paths.OpenRGBDeviceLightingFile)
	requireLighting(t, err)
	effects, err := lightingsettings.LoadDeviceStore(paths.DeviceEffectSettingsFile)
	requireLighting(t, err)
	resolver, err := lightingsettings.NewDeviceResolver(rt.Defaults, effects)
	requireLighting(t, err)
	fresh := &Device{Serial: d.Serial, LEDChannels: 3, ZoneAmount: 3}
	fresh.loadDeviceProfiles()
	requireLighting(t, fresh.attachLightingSource(&lightingsettings.IndependentDeviceRuntime{State: state, Effects: effects, Defaults: rt.Defaults, Resolver: resolver}))
	if !fresh.userRGBOff || fresh.schedulerDark {
		t.Fatal("restart lost preference or resurrected scheduler")
	}
	var packets [][]byte
	fresh.lightingWrite = func(p []byte) error { packets = append(packets, p); return nil }
	fresh.setLightingAvailable(true)
	fresh.setDeviceColor()
	if len(packets) != 3 || lightingFrame(t, fresh)[0].rgb != [3]byte{} {
		t.Fatal("reconnect ignored saved preference")
	}
	fresh.SchedulerBrightness(0)
	fresh.ControlDeviceRgb(false)
	if fresh.userRGBOff || persistedRGBOffProfile(t, fresh.DeviceProfile.Path).RgbOff || lightingFrame(t, fresh)[0].rgb != [3]byte{} {
		t.Fatal("clearing user preference released scheduler")
	}
	fresh.SchedulerBrightness(1)
	snap, _ := fresh.LightingSnapshot()
	if snap.Brightness != 40 || lightingFrame(t, fresh)[0].rgb != [3]byte{0, 102, 102} {
		t.Fatal("did not restore latest desired brightness")
	}
	// Scheduler-only darkness survives reconnect in runtime but never restart.
	fresh.SchedulerBrightness(0)
	if persistedRGBOffProfile(t, fresh.DeviceProfile.Path).RgbOff {
		t.Fatal("scheduler wrote user preference")
	}
	fresh.setLightingAvailable(false)
	fresh.setLightingAvailable(true)
	fresh.setDeviceColor()
	if lightingFrame(t, fresh)[0].rgb != [3]byte{} {
		t.Fatal("reconnect lost runtime scheduler")
	}
}

func TestRGBOffPersistenceDoesNotRestoreLegacyAuthoredAuthority(t *testing.T) {
	d, _, _ := lightingFixture(t, true)
	requireLighting(t, d.SetLightingEffect("mouse"))
	requireLighting(t, d.SetLightingBrightness(60))
	requireLighting(t, d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 200, Green: 100, Blue: 50}))
	settings, err := d.ResolveLightingEffectSettings("static")
	requireLighting(t, err)
	settings.SingleColor.Color = lightingsettings.Color{Red: 40, Blue: 80}
	requireLighting(t, d.SetLightingEffectSettings("static", settings))
	d.DeviceProfile.RGBProfile = "gradient"
	d.DeviceProfile.BrightnessSlider = nil
	d.DeviceProfile.ZoneColors = nil
	d.DeviceProfile.RgbOff = true
	d.Rgb = nil
	// Directly changing serialized preference backing is not an ongoing read of
	// any old authored state; only attachment seeds the effective preference.
	if lightingFrame(t, d)[0].rgb != [3]byte{120, 60, 30} {
		t.Fatal("profile mutation became ongoing authority")
	}
	requireLighting(t, d.setUserRGBOff(true))
	requireLighting(t, d.setUserRGBOff(false))
	snap, ok := d.LightingSnapshot()
	if !ok || snap.ConfiguredEffect != "mouse" || snap.Brightness != 60 || snap.AuthoredZoneEditor.Zones[0].ColorHex != "#c86432" {
		t.Fatal("preference persistence restored legacy authority")
	}
	got, err := d.ResolveLightingEffectSettings("static")
	requireLighting(t, err)
	if !reflect.DeepEqual(got, settings) {
		t.Fatal("preference persistence changed canonical settings")
	}
	if lightingFrame(t, d)[0].rgb != [3]byte{120, 60, 30} {
		t.Fatal("preference restore read old colors")
	}
}
