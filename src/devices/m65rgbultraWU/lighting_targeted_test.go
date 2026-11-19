package m65rgbultraWU

import (
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func targetedWUPair(t *testing.T) (*Device, *Device, config.Paths) {
	t.Helper()
	a, _, _ := wuFixture(t, false)
	defaults, err := filepath.Abs("../../../database")
	mustLighting(t, err)
	paths := config.Paths{OpenRGBDeviceLightingFile: filepath.Join(pwd, "state.json"), DeviceEffectSettingsFile: filepath.Join(pwd, "effects.json"), ShippedDatabaseRoot: defaults}
	p := cloneLightingProfile(*a.DeviceProfile)
	p.Serial = "WU456"
	p.Path = filepath.Join(pwd, "database", "profiles", "WU456.json")
	b := &Device{Serial: p.Serial, Product: a.Product, DeviceProfile: &p, LEDChannels: 2, ChangeableLedChannels: 2, ZoneAmount: 1}
	reg := &wuRegistry{}
	reg.hooks(b)
	mustLighting(t, a.attachLightingRuntime(paths))
	mustLighting(t, b.attachLightingRuntime(paths))
	t.Cleanup(b.stopCanonicalLighting)
	if a.lightingCanonical.state != b.lightingCanonical.state || a.lightingCanonical.effects != b.lightingCanonical.effects {
		t.Fatal("same persistence paths did not share store ownership")
	}
	for _, path := range []string{paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("attachment wrote canonical persistence", path, err)
		}
	}
	return a, b, paths
}

func TestWUTargetedCrossDeviceStatePersistence(t *testing.T) {
	a, b, paths := targetedWUPair(t)
	mustLighting(t, a.SetLightingBrightness(37))
	mustLighting(t, b.SetLightingEffect("rainbow"))
	store, err := lightingsettings.LoadIndependentDeviceStateStore(paths.OpenRGBDeviceLightingFile)
	mustLighting(t, err)
	for _, want := range []struct {
		device     *Device
		effect     string
		brightness uint8
	}{{a, "static", 37}, {b, "rainbow", 100}} {
		got, found, err := store.Resolve(want.device.Serial)
		mustLighting(t, err)
		if !found || got.SelectedEffect != want.effect || got.Brightness != want.brightness {
			t.Fatal("cross-device state record lost", want.device.Serial, got, found)
		}
	}
}

func TestWUTargetedCrossDeviceEffectsPersistence(t *testing.T) {
	a, b, paths := targetedWUPair(t)
	values := []lightingsettings.EffectSettings{
		{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 31, Green: 47}}},
		{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Blue: 93, Green: 67}}},
	}
	for i, d := range []*Device{a, b} {
		mustLighting(t, d.SetLightingEffectSettings("static", values[i]))
	}
	store, err := lightingsettings.LoadDeviceStore(paths.DeviceEffectSettingsFile)
	mustLighting(t, err)
	for i, d := range []*Device{a, b} {
		got, found, err := store.Get(d.Serial, "static")
		mustLighting(t, err)
		if !found || !reflect.DeepEqual(got, values[i]) {
			t.Fatal("cross-device effect record lost", d.Serial, got, found)
		}
		live, err := d.ResolveLightingEffectSettings("static")
		mustLighting(t, err)
		if !reflect.DeepEqual(live, values[i]) {
			t.Fatal("shared resolver lost device customization", d.Serial, live)
		}
	}
}

func TestWUTargetedGradientCompatibilityEditOrder(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	want := wuGateGradient([]int{1, 0, 2}, []float64{0, 0, 1})
	mustLighting(t, d.SetLightingEffectSettings("gradient", want))
	check := func(want lightingsettings.EffectSettings) {
		t.Helper()
		got, err := d.ResolveLightingEffectSettings("gradient")
		mustLighting(t, err)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("compatibility edit changed authored order", got, want)
		}
		store, err := lightingsettings.LoadDeviceStore(filepath.Join(pwd, "effects.json"))
		mustLighting(t, err)
		saved, found, err := store.Get(d.Serial, "gradient")
		mustLighting(t, err)
		if !found || !reflect.DeepEqual(saved, want) {
			t.Fatal("reloaded compatibility order changed", saved, want)
		}
	}
	check(want)
	profile := d.GetRgbProfile("gradient")
	profile.Speed = 3.5
	if d.UpdateRgbProfileData("gradient", *profile) != 1 {
		t.Fatal("speed-only compatibility edit failed")
	}
	speed := 3.5
	want.Speed = &speed
	check(want)
	// Moved stops still sort by position; new tied keys have deterministic order.
	profile = d.GetRgbProfile("gradient")
	moved := profile.Gradients[1]
	moved.Position = .75
	profile.Gradients[1] = moved
	profile.Gradients[7] = rgb.Color{Red: 70, Position: .75, Brightness: 1}
	profile.Gradients[3] = rgb.Color{Blue: 30, Position: .75, Brightness: 1}
	if d.UpdateRgbProfileData("gradient", *profile) != 1 {
		t.Fatal("moved/new stop edit failed")
	}
	first, middle, last := want.Gradient.Stops[0], want.Gradient.Stops[1], want.Gradient.Stops[2]
	first.Position = .75
	want.Gradient.Keys = []int{0, 1, 3, 7, 2}
	want.Gradient.Stops = []lightingsettings.GradientStop{middle, first, {Position: .75, Intensity: 1, Color: lightingsettings.Color{Blue: 30}}, {Position: .75, Intensity: 1, Color: lightingsettings.Color{Red: 70}}, last}
	check(want)
}

func TestWUTargetedRGBOffResumeKeepsProfileIdentity(t *testing.T) {
	d, rt, _ := wuFixture(t, false)
	d.DeviceProfile.RgbOff = true
	d.saveDeviceProfile()
	aPath := d.DeviceProfile.Path
	b := cloneLightingProfile(*d.DeviceProfile)
	b.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-alternate.json")
	b.Active = false
	mustLighting(t, common.SaveJsonData(b.Path, &b))
	d.loadDeviceProfiles()
	mustLighting(t, d.attachLightingSource(rt))
	entered, gate := make(chan struct{}), make(chan struct{})
	release := releaseWU(t, gate)
	fail := true
	var once sync.Once
	d.lightingReportIO = func(r []byte) ([]byte, error) {
		if r[2] == 0x0d {
			if fail {
				return nil, errors.New("initial endpoint unavailable")
			}
			once.Do(func() { close(entered); <-gate })
		}
		return make([]byte, 64), nil
	}
	d.initLeds()
	fail = false
	selected := make(chan error, 1)
	go func() { selected <- d.SetLightingEffect("static") }()
	waitWU(t, entered, "profile A selection paused in endpoint recovery")
	published := make(chan struct{}, 1)
	previous := d.lightingOpenRGBReconcile
	var publishOnce sync.Once
	d.lightingOpenRGBReconcile = func(serial string, c *common.OpenRGBController) {
		previous(serial, c)
		if d.DeviceProfile.Path == b.Path {
			publishOnce.Do(func() { published <- struct{}{} })
		}
	}
	switched := make(chan uint8, 1)
	go func() { switched <- d.ChangeDeviceProfile("alternate") }()
	waitWU(t, published, "profile B published before selection resumes")
	release()
	mustLighting(t, waitWU(t, selected, "profile A effect selection completed"))
	if waitWU(t, switched, "profile B switch completed") != 1 {
		t.Fatal("profile switch failed")
	}
	if d.DeviceProfile.Path != b.Path || !d.DeviceProfile.RgbOff || !d.userRGBOff {
		t.Fatal("profile A resume cleared profile B")
	}
	for _, path := range []string{aPath, b.Path} {
		if p := readWUCompatibilityProfile(t, path); !p.RgbOff {
			t.Fatal("selection cleared an unintended persisted profile", path)
		}
	}
}
