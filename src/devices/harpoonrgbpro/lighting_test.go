package harpoonrgbpro

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"github.com/sstallion/go-hid"
)

var harpoonCatalogue = []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}

func newHarpoonLightingFixture(t *testing.T, attach bool) (*Device, *lightingsettings.IndependentDeviceRuntime, config.Paths) {
	t.Helper()
	logger.Init()
	previous := pwd
	pwd = t.TempDir()
	t.Cleanup(func() { pwd = previous })
	for _, name := range []string{"profiles", "rgb"} {
		if err := os.MkdirAll(filepath.Join(pwd, "database", name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile("../../../database/rgb.json")
	if err != nil {
		t.Fatal(err)
	}
	var profiles rgb.RGB
	if err = json.Unmarshal(data, &profiles); err != nil {
		t.Fatal(err)
	}
	profiles.Profiles["off"] = rgb.Profile{}
	brightness := uint8(63)
	profile := &DeviceProfile{Product: "HARPOON RGB PRO", Serial: "HarpoonTEST", Active: true, RGBProfile: "mouse", BrightnessSlider: &brightness, OriginalBrightness: 91, PollingRate: 1, SleepMode: 15, Profile: 1, Path: filepath.Join(pwd, "database", "profiles", "HarpoonTEST.json"), ZoneColors: map[int]ZoneColors{0: {Name: "Logo", ColorIndex: []int{0, 1, 2}, Color: &rgb.Color{Red: 200, Green: 100, Blue: 40, Brightness: 1, Hex: "#c86428"}}}, Profiles: map[int]DPIProfile{1: {Name: "Stage 2", Value: 1500, ColorIndex: map[int][]int{0: {0, 1, 2}}, Color: &rgb.Color{Red: 255, Green: 120, Blue: 40, Brightness: 1}}, 5: {Name: "Sniper", Value: 200, PackerIndex: 6, Sniper: true, ColorIndex: map[int][]int{0: {0, 1, 2}}, Color: &rgb.Color{Red: 120, Green: 80, Blue: 40, Brightness: 1}}}}
	d := &Device{Product: profile.Product, Serial: profile.Serial, dev: &hid.Device{}, Connected: true, Usb: true, LEDChannels: 1, ChangeableLedChannels: 1, ZoneAmount: 1, MinDPI: 200, MaxDPI: 12000, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"default": profile}, Rgb: &profiles, lightingRestart: func() {}}
	// Use the independently audited contiguous five-stage topology. The legacy
	// upgrade deliberately inserts Sniper at len(map), not at a borrowed index.
	for _, id := range []int{0, 2, 3, 4} {
		d.DeviceProfile.Profiles[id] = DPIProfile{Name: "Regular", Value: []uint16{800, 1500, 3000, 6000, 9000}[id], ColorIndex: map[int][]int{0: {0, 1, 2}}, Color: &rgb.Color{Red: 255, Brightness: 1}}
	}
	paths := config.Paths{MutableDataRoot: pwd, ShippedDatabaseRoot: "../../../database", OpenRGBDeviceLightingFile: filepath.Join(pwd, "database", "lighting", "openrgb-device-state.json"), DeviceEffectSettingsFile: filepath.Join(pwd, "database", "lighting", "independent-device-effects.json")}
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile, filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = common.SaveJsonData(profile.Path, profile); err != nil {
		t.Fatal(err)
	}
	if err = common.SaveJsonData(filepath.Join(pwd, "database", "rgb", d.Serial+".json"), &profiles); err != nil {
		t.Fatal(err)
	}
	if attach {
		if err = d.attachLightingSource(runtime); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		d.stopLighting()
		d.rendererMu.Lock()
		done := d.rendererDone
		d.rendererMu.Unlock()
		if done != nil {
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Error("renderer did not retire")
			}
		}
	})
	return d, runtime, paths
}
func mustHarpoonSnapshot(t *testing.T, d *Device) lightingSnapshotAlias {
	t.Helper()
	snapshot, ok := d.LightingSnapshot()
	if !ok {
		t.Fatal("unusable snapshot")
	}
	return lightingSnapshotAlias{snapshot.ConfiguredEffect, snapshot.Brightness}
}

type lightingSnapshotAlias struct {
	effect     string
	brightness uint8
}

func mustHarpoonFrame(t *testing.T, d *Device) harpoonFrameState {
	t.Helper()
	state, err := d.lightingFrameState()
	if err != nil {
		t.Fatal(err)
	}
	return state
}
func harpoonRendererOwner() *rgb.ActiveRGB {
	owner := rgb.Exit()
	owner.RGBStartColor = &rgb.Color{Red: 128, Green: 200, Brightness: 1}
	owner.RGBEndColor = &rgb.Color{Blue: 255, Brightness: 1}
	return owner
}

func TestHarpoonCatalogueTopologyImportAndCompleteSettings(t *testing.T) {
	d, runtime, _ := newHarpoonLightingFixture(t, true)
	snapshot, ok := d.LightingSnapshot()
	var got []string
	for _, effect := range snapshot.SupportedEffects {
		got = append(got, effect.ID)
	}
	if !ok || !reflect.DeepEqual(got, harpoonCatalogue) || len(got) != 20 || snapshot.ConfiguredEffect != "mouse" || snapshot.Brightness != 63 || len(snapshot.AuthoredZoneEditor.Zones) != 1 || snapshot.AuthoredZoneEditor.Zones[0].Label != "Logo" || snapshot.AuthoredZoneEditor.Zones[0].ID != "0" || snapshot.AuthoredZoneEditor.Zones[0].ColorHex != "#c86428" {
		t.Fatalf("snapshot=%#v", snapshot)
	}
	if snapshot.ClusterControlled || snapshot.ExternalControlled || len(snapshot.Channels) != 0 {
		t.Fatal("invented ownership/topology")
	}
	for _, effect := range harpoonCatalogue {
		if err := d.SetLightingEffect(effect); err != nil {
			t.Fatalf("select %s: %v", effect, err)
		}
		if effect == "mouse" {
			continue
		}
		settings, err := d.ResolveLightingEffectSettings(effect)
		if err != nil || lightingsettings.Validate(settings) != nil {
			t.Fatalf("%s incomplete: %#v %v", effect, settings, err)
		}
	}
	for _, effect := range []string{"pastelspiralrainbow", "circle", "circleshift", "spinner", "custom", "liquid-temperature", "probe-temperature"} {
		if d.SupportsLightingEffect(effect) || d.SetLightingEffect(effect) == nil {
			t.Fatalf("unexpected effect %s", effect)
		}
	}
	frame := mustHarpoonFrame(t, d)
	if !slicesEqual(frame.zones[0].ColorIndex, []int{0, 1, 2}) {
		t.Fatal("Logo mapping changed")
	}
	persisted, found, err := runtime.State.Resolve(d.Serial)
	if err != nil || !found || persisted.Brightness != 63 {
		t.Fatal("state import", persisted, found, err)
	}
	if _, ok := interface{}(d).(interface{ ProcessSetRgbCluster(bool) uint8 }); ok {
		t.Fatal("Cluster capability")
	}
	if _, ok := interface{}(d).(interface{ ProcessSetOpenRgbIntegration(bool) uint8 }); ok {
		t.Fatal("OpenRGB capability")
	}
}
func slicesEqual(a, b []int) bool { return reflect.DeepEqual(a, b) }

func TestHarpoonCanonicalSoleAuthorityAndOneTimeImport(t *testing.T) {
	d, runtime, paths := newHarpoonLightingFixture(t, false)
	custom := d.Rgb.Profiles["static"]
	custom.StartColor.Red = 17
	d.Rgb.Profiles["static"] = custom
	if err := d.attachLightingSource(runtime); err != nil {
		t.Fatal(err)
	}
	beforeLegacy, _ := os.ReadFile(d.DeviceProfile.Path)
	beforeRGB, _ := os.ReadFile(filepath.Join(pwd, "database", "rgb", d.Serial+".json"))
	d.DeviceProfile.RGBProfile = "wave"
	*d.DeviceProfile.BrightnessSlider = 2
	d.DeviceProfile.ZoneColors[0].Color.Red = 1
	custom.StartColor.Red = 99
	d.Rgb.Profiles["static"] = custom
	if got := mustHarpoonSnapshot(t, d); got.effect != "mouse" || got.brightness != 63 {
		t.Fatal("legacy silently took authority", got)
	}
	if frame := mustHarpoonFrame(t, d); frame.zones[0].Color.Red != 200 {
		t.Fatal("legacy Logo took authority")
	}
	settings, _ := d.ResolveLightingEffectSettings("static")
	if settings.SingleColor.Color.Red != 17 {
		t.Fatal("legacy RGB took authority")
	}
	d.lightingWriteJSON = func(string, interface{}) error {
		t.Fatal("canonical mutation wrote legacy backing")
		return errors.New("legacy blocked")
	}
	if err := d.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetLightingBrightness(47); err != nil {
		t.Fatal(err)
	}
	if err := d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 9, Green: 8, Blue: 7}); err != nil {
		t.Fatal(err)
	}
	settings.SingleColor.Color.Red = 23
	if err := d.SetLightingEffectSettings("static", settings); err != nil {
		t.Fatal(err)
	}
	if err := d.ResetLightingEffectSettings("static"); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := runtime.Effects.Get(d.Serial, "static"); found {
		t.Fatal("reset persisted defaults rather than deleting")
	}
	afterLegacy, _ := os.ReadFile(d.DeviceProfile.Path)
	afterRGB, _ := os.ReadFile(filepath.Join(pwd, "database", "rgb", d.Serial+".json"))
	if !bytes.Equal(beforeLegacy, afterLegacy) || !bytes.Equal(beforeRGB, afterRGB) {
		t.Fatal("dual writes")
	}
	d.lightingCanonical = nil
	if err := d.attachLightingRuntime(paths); err != nil {
		t.Fatal(err)
	}
	if got := mustHarpoonSnapshot(t, d); got.effect != "static" || got.brightness != 47 {
		t.Fatal("reattachment reimported old state", got)
	}
	settings, _ = d.ResolveLightingEffectSettings("static")
	defaults, _ := runtime.Defaults.Get("static")
	if !reflect.DeepEqual(settings, defaults) {
		t.Fatal("reset reimported old RGB customization")
	}
	if frame := mustHarpoonFrame(t, d); frame.zones[0].Color.Red != 9 {
		t.Fatal("Logo reimported")
	}
	// Reload from disk, independently of the shared runtime cache.
	state, err := lightingsettings.LoadIndependentDeviceStateStore(paths.OpenRGBDeviceLightingFile)
	if err != nil {
		t.Fatal(err)
	}
	got, _, _ := state.Resolve(d.Serial)
	if got.Brightness != 47 || got.SelectedEffect != "static" {
		t.Fatal(got)
	}
	effects, err := lightingsettings.LoadDeviceStore(paths.DeviceEffectSettingsFile)
	if err != nil {
		t.Fatal(err)
	}
	zones, _, _ := effects.GetAuthoredZones(d.Serial, "mouse")
	if zones["0"].Red != 9 {
		t.Fatal(zones)
	}
}

type harpoonFailState struct {
	lightingsettings.IndependentDeviceStateAccess
	before func()
	fail   bool
}

func (s harpoonFailState) Set(id string, value lightingsettings.IndependentDeviceLightingState) error {
	if s.before != nil {
		s.before()
	}
	if s.fail {
		return errors.New("injected state failure")
	}
	return s.IndependentDeviceStateAccess.Set(id, value)
}

type harpoonFailEffects struct {
	harpoonEffectStore
	before func()
	fail   bool
}

func (s harpoonFailEffects) check() error {
	if s.before != nil {
		s.before()
	}
	if s.fail {
		return errors.New("injected settings failure")
	}
	return nil
}
func (s harpoonFailEffects) Set(id, effect string, value lightingsettings.EffectSettings) error {
	if err := s.check(); err != nil {
		return err
	}
	return s.harpoonEffectStore.Set(id, effect, value)
}
func (s harpoonFailEffects) Delete(id, effect string) (bool, error) {
	if err := s.check(); err != nil {
		return false, err
	}
	return s.harpoonEffectStore.Delete(id, effect)
}
func (s harpoonFailEffects) SetAuthoredZones(id, effect string, value map[string]lightingsettings.Color) error {
	if err := s.check(); err != nil {
		return err
	}
	return s.harpoonEffectStore.SetAuthoredZones(id, effect, value)
}

func TestHarpoonPersistenceBeforePublicationAndFailures(t *testing.T) {
	d, runtime, _ := newHarpoonLightingFixture(t, true)
	restarts, outputs := 0, 0
	d.lightingRestart = func() { restarts++ }
	d.lightingReport = func([]byte) error { outputs++; return nil }
	legacy := cloneHarpoonProfile(*d.DeviceProfile)
	oldState, _, _ := runtime.State.Resolve(d.Serial)
	oldLogo, _, _ := runtime.Effects.GetAuthoredZones(d.Serial, "mouse")
	gradient, _ := d.ResolveLightingEffectSettings("gradient")
	check := func() {
		state, _, _ := runtime.State.Resolve(d.Serial)
		logo, _, _ := runtime.Effects.GetAuthoredZones(d.Serial, "mouse")
		if state != oldState || !reflect.DeepEqual(logo, oldLogo) || !reflect.DeepEqual(legacy, *d.DeviceProfile) || restarts != 0 || outputs != 0 {
			t.Fatal("publication before persistence")
		}
	}
	d.lightingCanonical.state = harpoonFailState{runtime.State, check, true}
	d.lightingCanonical.effects = harpoonFailEffects{runtime.Effects, check, true}
	gradient.Gradient.Stops[0].Color.Red = 10
	for _, mutation := range []func() error{
		func() error { return d.SetLightingEffect("wave") }, func() error { return d.SetLightingBrightness(12) }, func() error { return d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{}) }, func() error { return d.SetLightingEffectSettings("gradient", gradient) }, func() error { return d.ResetLightingEffectSettings("gradient") },
	} {
		if mutation() == nil {
			t.Fatal("failure reported as success")
		}
		check()
	}
	if status, _ := d.ProcessNewGradientColor("gradient"); status != 0 {
		t.Fatal("gradient failure hidden")
	}
	if status, _ := d.ProcessDeleteGradientColor("gradient"); status != 0 {
		t.Fatal("delete failure hidden")
	}
	if d.UpdateRgbProfile(0, "wave") != 0 || d.ChangeDeviceBrightnessValue(20) != 0 || d.SaveMouseZoneColors(rgb.Color{}, map[int]rgb.Color{0: {}}) != 0 {
		t.Fatal("legacy wrappers hid canonical failure")
	}
	d.lightingCanonical.state = harpoonFailState{runtime.State, check, false}
	if err := d.SetLightingBrightness(17); err != nil {
		t.Fatal(err)
	}
	state, _, _ := runtime.State.Resolve(d.Serial)
	if state.Brightness != 17 || restarts != 1 || outputs != 0 || !reflect.DeepEqual(legacy, *d.DeviceProfile) {
		t.Fatal("wrong post-write publication")
	}
}

func TestHarpoonDarknessKeepsLatestDesiredState(t *testing.T) {
	d, runtime, paths := newHarpoonLightingFixture(t, true)
	for i := 0; i < 2; i++ {
		if d.SchedulerBrightness(0) != 1 {
			t.Fatal("scheduler")
		}
	}
	if got := mustHarpoonSnapshot(t, d); got.brightness != 63 {
		t.Fatal("darkness replaced desire")
	}
	if err := d.SetLightingBrightness(41); err != nil {
		t.Fatal(err)
	}
	if err := d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 100, Blue: 100}); err != nil {
		t.Fatal(err)
	}
	if frame := mustHarpoonFrame(t, d); frame.brightness != 0 || frame.indicatorBrightness != 0 {
		t.Fatal("scheduler not dark")
	}
	d.ControlDeviceRgb(true)
	d.SchedulerBrightness(255)
	if frame := mustHarpoonFrame(t, d); frame.brightness != 0 || frame.effect != "off" || frame.indicatorBrightness != 41 {
		t.Fatal("independent RGB-off/flash semantics", frame)
	}
	if err := d.SetLightingBrightness(72); err != nil {
		t.Fatal(err)
	}
	if err := d.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	settings, _ := d.ResolveLightingEffectSettings("static")
	settings.SingleColor.Color.Red = 101
	if err := d.SetLightingEffectSettings("static", settings); err != nil {
		t.Fatal(err)
	}
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(false)
	if frame := mustHarpoonFrame(t, d); frame.effect != "static" || frame.brightness != 0 {
		t.Fatal("RGB-off cleared scheduler")
	}
	d.SchedulerBrightness(1)
	if frame := mustHarpoonFrame(t, d); frame.effect != "static" || frame.brightness != 72 || frame.profile.StartColor.Red != 101 {
		t.Fatal("restoration lost desired state", frame)
	}
	persisted, err := lightingsettings.LoadIndependentDeviceStateStore(paths.OpenRGBDeviceLightingFile)
	if err != nil {
		t.Fatal(err)
	}
	state, _, _ := persisted.Resolve(d.Serial)
	if state.Brightness != 72 || state.SelectedEffect != "static" {
		t.Fatal(state)
	}
	if d.DeviceProfile.RgbOff || *d.DeviceProfile.BrightnessSlider != 63 || d.DeviceProfile.OriginalBrightness != 91 {
		t.Fatal("transient override wrote legacy state")
	}
	state, _, _ = runtime.State.Resolve(d.Serial)
	if state.Brightness != 72 {
		t.Fatal("desired state changed")
	}
}

func TestHarpoonExactDispatchAndHardwareReport(t *testing.T) {
	d, _, _ := newHarpoonLightingFixture(t, true)
	var report []byte
	d.lightingReport = func(data []byte) error { report = append([]byte(nil), data...); return nil }
	for _, effect := range harpoonCatalogue {
		if err := d.SetLightingEffect(effect); err != nil {
			t.Fatal(err)
		}
		value := mustHarpoonFrame(t, d)
		owner := harpoonRendererOwner()
		start := time.Now().Add(-350 * time.Millisecond)
		frame := d.renderLightingFrame(value, owner, &start)
		if len(frame) != 3 {
			t.Fatalf("missing output dispatch %s: %v", effect, frame)
		}
		d.writeColor(frame)
		if len(report) != 65 || !bytes.Equal(report[:6], []byte{0, 7, 0x22, 1, 1, 3}) || !bytes.Equal(report[6:9], frame) || !bytes.Equal(report[9:], make([]byte, 56)) {
			t.Fatalf("changed report %s: %v", effect, report)
		}
		if effect == "mouse" && !bytes.Equal(frame, []byte{126, 63, 25}) {
			t.Fatal("Logo scaling", frame)
		}
		if effect == "off" && !bytes.Equal(frame, []byte{0, 0, 0}) {
			t.Fatal("Off", frame)
		}
	}
}

func TestHarpoonDpiSniperCompositionIsCopiedAndMouseOnly(t *testing.T) {
	d, _, _ := newHarpoonLightingFixture(t, true)
	before := cloneHarpoonProfile(*d.DeviceProfile)
	value := mustHarpoonFrame(t, d)
	flash := make([]byte, 3)
	for _, indexes := range value.indicatorIndexes {
		composeColor(flash, indexes, *value.indicator, value.indicatorBrightness)
	}
	if !bytes.Equal(flash, []byte{160, 75, 25}) {
		t.Fatal("Harpoon scaled stage flash", flash)
	}
	d.SniperMode = true
	value = mustHarpoonFrame(t, d)
	start := time.Now()
	frame := d.renderLightingFrame(value, harpoonRendererOwner(), &start)
	if !bytes.Equal(frame, []byte{75, 50, 25}) {
		t.Fatal("Mouse Sniper", frame)
	}
	if err := d.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	value = mustHarpoonFrame(t, d)
	frame = d.renderLightingFrame(value, harpoonRendererOwner(), &start)
	scaled := value.profile.StartColor
	scaled.Brightness = .63
	want := rgb.ModifyBrightness(scaled)
	if !bytes.Equal(frame, []byte{byte(want.Red), byte(want.Green), byte(want.Blue)}) {
		t.Fatal("Sniper overlaid Static")
	}
	if !reflect.DeepEqual(before, *d.DeviceProfile) {
		t.Fatal("composition mutated authored input/legacy colors")
	}
	if err := d.SetLightingEffect("mouse"); err != nil {
		t.Fatal(err)
	}
	stage := d.DeviceProfile.Profiles[5]
	stage.Color = nil
	d.DeviceProfile.Profiles[5] = stage
	value = mustHarpoonFrame(t, d)
	frame = d.renderLightingFrame(value, harpoonRendererOwner(), &start)
	if !bytes.Equal(frame, []byte{126, 63, 25}) {
		t.Fatal("nil Sniper killed Logo")
	}
}

func TestHarpoonSettingsGradientCopiesAndReset(t *testing.T) {
	d, runtime, _ := newHarpoonLightingFixture(t, true)
	for _, effect := range []string{"static", "colorshift", "cpu-temperature", "gpu-temperature", "gradient"} {
		value, err := d.ResolveLightingEffectSettings(effect)
		if err != nil {
			t.Fatal(err)
		}
		if value.Speed != nil {
			*value.Speed = 3
		}
		if value.SingleColor != nil {
			value.SingleColor.Color.Red = 12
		}
		if value.TwoColor != nil {
			value.TwoColor.End.Blue = 34
		}
		if value.Temperature != nil {
			value.Temperature.Middle.Celsius = 45
		}
		if value.Gradient != nil {
			value.Gradient.Stops[0].Intensity = .2
		}
		expected := value.Clone()
		if err = d.SetLightingEffectSettings(effect, value); err != nil {
			t.Fatal(err)
		}
		if value.Gradient != nil {
			value.Gradient.Stops[0].Intensity = .9
		}
		if value.SingleColor != nil {
			value.SingleColor.Color.Red = 99
		}
		stored, _ := d.ResolveLightingEffectSettings(effect)
		if !reflect.DeepEqual(expected, stored) {
			t.Fatal("caller settings alias", effect)
		}
		if err = d.ResetLightingEffectSettings(effect); err != nil {
			t.Fatal(err)
		}
		got, _ := d.ResolveLightingEffectSettings(effect)
		defaults, _ := runtime.Defaults.Get(effect)
		if !reflect.DeepEqual(got, defaults) {
			t.Fatal("reset", effect)
		}
	}
	before := d.GetRgbProfile("gradient")
	stop := before.Gradients[0]
	stop.Red = 1
	before.Gradients[0] = stop
	if d.GetRgbProfile("gradient").Gradients[0].Red == 1 {
		t.Fatal("getter map alias")
	}
	if status, _ := d.ProcessNewGradientColor("gradient"); status != 1 {
		t.Fatal("gradient add")
	}
	value, _ := d.ResolveLightingEffectSettings("gradient")
	if lightingsettings.Validate(value) != nil {
		t.Fatal("incomplete added stop")
	}
	if status, _ := d.ProcessDeleteGradientColor("gradient"); status != 1 {
		t.Fatal("gradient delete")
	}
	bad := value.Clone()
	bad.Gradient.Stops[0].Intensity = 2
	if d.SetLightingEffectSettings("gradient", bad) == nil {
		t.Fatal("invalid gradient accepted")
	}
}

func TestHarpoonAttachmentFailureAtomicityAndLegacyFallback(t *testing.T) {
	d, runtime, paths := newHarpoonLightingFixture(t, false)
	old := cloneHarpoonProfile(*d.DeviceProfile)
	runtime.State = harpoonFailState{IndependentDeviceStateAccess: runtime.State, fail: true}
	if err := d.attachLightingSource(runtime); err == nil || d.lightingAttached() {
		t.Fatal("failed target write published attachment")
	}
	if !reflect.DeepEqual(old, *d.DeviceProfile) {
		t.Fatal("attachment mutated legacy state")
	}
	if _, found, _ := runtime.Effects.Get(d.Serial, "static"); found {
		t.Fatal("partial settings publication")
	}
	if _, found, _ := runtime.Effects.GetAuthoredZones(d.Serial, "mouse"); found {
		t.Fatal("partial Logo publication")
	}
	disk, err := lightingsettings.LoadDeviceStore(paths.DeviceEffectSettingsFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, _ := disk.GetAuthoredZones(d.Serial, "mouse"); found {
		t.Fatal("failed import left persisted Logo")
	}
	if _, ok := d.LightingSnapshot(); ok {
		t.Fatal("unattached snapshot")
	}
	reports := make(chan []byte, 16)
	d.lightingReport = func(report []byte) error {
		select {
		case reports <- report:
		default:
		}
		return nil
	}
	if d.ChangeDeviceBrightnessValue(51) != 1 {
		t.Fatal("canonical eligibility leaked into legacy mutation")
	}
	got := waitHarpoonReport(t, reports)
	if !bytes.Equal(got[6:9], []byte{102, 51, 20}) {
		t.Fatal("legacy fallback output", got)
	}
	if *d.DeviceProfile.BrightnessSlider != 51 {
		t.Fatal("legacy fallback persistence")
	}
	// Invalid imported gradient fails before any writes, still leaving fallback.
	originalState := runtime.State.(harpoonFailState).IndependentDeviceStateAccess
	runtime.State = originalState
	gradient := d.Rgb.Profiles["gradient"]
	gradient.Gradients = map[int]rgb.Color{1: {Red: 255, Brightness: 1, Position: .3}, 3: {Blue: 255, Brightness: 1, Position: .9}}
	d.Rgb.Profiles["gradient"] = gradient
	if d.attachLightingSource(runtime) == nil || d.lightingAttached() {
		t.Fatal("sparse gradient partial attachment")
	}
	if _, found, _ := runtime.State.Resolve(d.Serial); found {
		t.Fatal("failed validation initialized target")
	}
}

func TestHarpoonAttachmentNormalizesOnlyAuditedDefaults(t *testing.T) {
	d, runtime, _ := newHarpoonLightingFixture(t, false)
	d.DeviceProfile.BrightnessSlider = nil
	d.DeviceProfile.SleepMode = 0
	d.DeviceProfile.PollingRate = 0
	delete(d.DeviceProfile.Profiles, 5)
	before := cloneHarpoonProfile(*d.DeviceProfile)
	if err := d.attachLightingSource(runtime); err != nil {
		t.Fatal(err)
	}
	if got := mustHarpoonSnapshot(t, d); got.brightness != 100 {
		t.Fatal("missing slider normalization", got)
	}
	if !reflect.DeepEqual(before, *d.DeviceProfile) {
		t.Fatal("attachment altered input profile")
	}
	normalized := normalizeHarpoonProfile(before)
	if normalized.SleepMode != 15 || normalized.PollingRate != 0 || normalized.BrightnessSlider == nil || *normalized.BrightnessSlider != 100 {
		t.Fatal("borrowed defaults")
	}
	sniper := normalized.Profiles[len(before.Profiles)]
	if !sniper.Sniper || sniper.Value != 200 || sniper.PackerIndex != 6 || sniper.Color.Hex != "#ffff00" || !reflect.DeepEqual(sniper.ColorIndex, map[int][]int{0: {0, 1, 2}}) {
		t.Fatal("Sniper upgrade changed", sniper)
	}
	if before.Profiles == nil || len(before.Profiles) == len(normalized.Profiles) {
		t.Fatal("normalization mutated original map")
	}
}

func TestHarpoonFreshLegacyProfileDefaults(t *testing.T) {
	d, _, _ := newHarpoonLightingFixture(t, false)
	d.DeviceProfile = nil
	d.UserProfiles = nil
	d.saveDeviceProfile()
	p := d.DeviceProfile
	if p == nil || p.RGBProfile != "mouse" || *p.BrightnessSlider != 100 || p.OriginalBrightness != 100 || p.SleepMode != 15 || p.PollingRate != 1 || p.Profile != 1 || len(p.Profiles) != 6 || p.ZoneColors[0].Color.Hex != "#ffff00" {
		t.Fatal("fresh Harpoon defaults", p)
	}
	for id, want := range []uint16{800, 1500, 3000, 6000, 9000, 200} {
		if p.Profiles[id].Value != want || !reflect.DeepEqual(p.Profiles[id].ColorIndex, map[int][]int{0: {0, 1, 2}}) {
			t.Fatal("wrong DPI defaults", id)
		}
	}
	if !p.Profiles[5].Sniper || p.Profiles[5].PackerIndex != 6 {
		t.Fatal("Sniper defaults")
	}
}

func TestHarpoonProfileSwitchKeepsDeviceWideCanonicalAuthority(t *testing.T) {
	d, _, _ := newHarpoonLightingFixture(t, true)
	if err := d.SetLightingBrightness(49); err != nil {
		t.Fatal(err)
	}
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(true)
	d.SniperMode = true
	candidate := cloneHarpoonProfile(*d.DeviceProfile)
	candidate.Active = false
	candidate.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-gaming.json")
	candidate.RGBProfile = "pastelspiralrainbow"
	candidate.BrightnessSlider = nil
	candidate.PollingRate = 0
	candidate.SleepMode = 0
	candidate.RgbOff = true
	candidate.Label = "preserve input label"
	candidate.KeyAssignmentHash = "input-owned"
	delete(candidate.Profiles, 5)
	d.UserProfiles["gaming"] = &candidate
	restarts, inputs := 0, 0
	d.lightingRestart = func() { restarts++ }
	d.lightingProfileChanged = func() { inputs++ }
	if d.ChangeDeviceProfile("gaming") != 1 {
		t.Fatal("profile switch")
	}
	got := mustHarpoonSnapshot(t, d)
	frame := mustHarpoonFrame(t, d)
	if got.effect != "mouse" || got.brightness != 49 || frame.brightness != 0 || d.userRGBOff || !d.schedulerDark || d.DeviceProfile.PollingRate != 0 || d.DeviceProfile.SleepMode != 15 || *d.DeviceProfile.BrightnessSlider != 100 || !d.SniperMode || d.DeviceProfile.Profile != 1 || d.DeviceProfile.Label != "preserve input label" || d.DeviceProfile.KeyAssignmentHash != "input-owned" || restarts != 1 || inputs != 1 {
		t.Fatalf("profile scope/defaults changed: %#v %#v", got, d.DeviceProfile)
	}
	// Unsupported legacy selections remain serialized/loadable, unselectable.
	if d.DeviceProfile.RGBProfile != "pastelspiralrainbow" || d.SetLightingEffect("pastelspiralrainbow") == nil {
		t.Fatal("legacy selection became selectable")
	}
	old := cloneHarpoonProfile(*d.DeviceProfile)
	oldPointer := d.DeviceProfile
	d.lightingWriteJSON = func(string, interface{}) error { return errors.New("input profile write failure") }
	if d.ChangeDeviceProfile("default") != 0 || d.DeviceProfile != oldPointer || !reflect.DeepEqual(old, *d.DeviceProfile) || restarts != 1 || inputs != 1 {
		t.Fatal("failed switch changed state/output")
	}
}

func TestHarpoonUnavailableGuardsBeforeWrites(t *testing.T) {
	checks := []struct {
		name       string
		breakState func(*Device)
	}{
		{"attachment", func(d *Device) { d.lightingCanonical = nil }},
		{"hid", func(d *Device) { d.dev = nil }},
		{"connected", func(d *Device) { d.Connected = false }},
		{"exit", func(d *Device) { d.Exit = true }},
		{"profile", func(d *Device) { d.DeviceProfile = nil }},
		{"leds", func(d *Device) { d.LEDChannels = 2 }},
		{"changeable", func(d *Device) { d.ChangeableLedChannels = 0 }},
		{"zones", func(d *Device) { d.ZoneAmount = 2 }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			d, runtime, _ := newHarpoonLightingFixture(t, true)
			state, _, _ := runtime.State.Resolve(d.Serial)
			writes, restarts := 0, 0
			value, _ := d.ResolveLightingEffectSettings("static")
			d.lightingCanonical.state = harpoonFailState{IndependentDeviceStateAccess: runtime.State, before: func() { writes++ }}
			d.lightingCanonical.effects = harpoonFailEffects{harpoonEffectStore: runtime.Effects, before: func() { writes++ }}
			d.lightingRestart = func() { restarts++ }
			check.breakState(d)
			if _, ok := d.LightingSnapshot(); ok {
				t.Fatal("unavailable snapshot")
			}
			for _, mutation := range []func() error{func() error { return d.SetLightingBrightness(20) }, func() error { return d.SetLightingEffect("wave") }, func() error { return d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{}) }, func() error { return d.SetLightingEffectSettings("static", value) }, func() error { return d.ResetLightingEffectSettings("static") }} {
				if mutation() == nil {
					t.Fatal("unavailable mutation")
				}
			}
			after, _, _ := runtime.State.Resolve(d.Serial)
			if writes != 0 || restarts != 0 || state != after {
				t.Fatal("unavailable guard had side effects")
			}
		})
	}
	d, _, _ := newHarpoonLightingFixture(t, true)
	d.Exit = true
	d.dev = nil
	restarts := 0
	d.lightingRestart = func() { restarts++ }
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(true)
	if !d.schedulerDark || !d.userRGBOff || restarts != 0 {
		t.Fatal("offline override not retained/gated")
	}
}

type harpoonFailResolver struct {
	harpoonSettingsResolver
	missing atomic.Bool
}

func (r *harpoonFailResolver) Resolve(target lightingsettings.Target, effect string) (lightingsettings.Resolution, error) {
	if r.missing.Load() {
		return lightingsettings.Resolution{}, errors.New("temporary missing effect")
	}
	return r.harpoonSettingsResolver.Resolve(target, effect)
}
func waitHarpoonReport(t *testing.T, reports <-chan []byte) []byte {
	t.Helper()
	select {
	case frame := <-reports:
		return frame
	case <-time.After(2 * time.Second):
		t.Fatal("no report")
		return nil
	}
}
func rendererDone(d *Device) chan struct{} {
	d.rendererMu.Lock()
	defer d.rendererMu.Unlock()
	return d.rendererDone
}
func awaitHarpoonRetirement(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("renderer did not retire")
	}
}

func TestHarpoonRendererMissingStateRetryAndRetirement(t *testing.T) {
	d, _, _ := newHarpoonLightingFixture(t, true)
	if err := d.SetLightingEffect("wave"); err != nil {
		t.Fatal(err)
	}
	resolver := &harpoonFailResolver{harpoonSettingsResolver: d.lightingCanonical.resolver}
	resolver.missing.Store(true)
	d.lightingCanonical.resolver = resolver
	reports := make(chan []byte, 64)
	d.lightingReport = func(report []byte) error {
		select {
		case reports <- report:
		default:
		}
		return nil
	}
	d.lightingRestart = nil
	d.setDeviceColor(false)
	oldDone := rendererDone(d)
	select {
	case <-oldDone:
		t.Fatal("temporary missing state killed renderer")
	case <-reports:
		t.Fatal("missing state produced report")
	case <-time.After(90 * time.Millisecond):
	}
	resolver.missing.Store(false)
	waitHarpoonReport(t, reports)
	d.stopLighting()
	d.stopLighting()
	awaitHarpoonRetirement(t, oldDone)
	d.rendererMu.Lock()
	active := d.activeRgb
	d.rendererMu.Unlock()
	if active != nil {
		t.Fatal("dead renderer ownership")
	}
	if err := d.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	done := rendererDone(d)
	waitHarpoonReport(t, reports)
	awaitHarpoonRetirement(t, done)
	d.rendererMu.Lock()
	active = d.activeRgb
	d.rendererMu.Unlock()
	if active != nil {
		t.Fatal("one-shot activeRgb stayed published")
	}
	// Runtime RGB-off writes black once; selected "off" remains a dynamic mode.
	d.lightingRestart = func() {}
	d.ControlDeviceRgb(true)
	d.setDeviceColor(false)
	done = rendererDone(d)
	black := waitHarpoonReport(t, reports)
	if !bytes.Equal(black[6:9], []byte{0, 0, 0}) {
		t.Fatal("runtime RGB-off did not clear Logo", black)
	}
	awaitHarpoonRetirement(t, done)
}

func TestHarpoonFallbackMissingStateRetriesWithoutStrictCanonicalValidation(t *testing.T) {
	d, _, _ := newHarpoonLightingFixture(t, false)
	d.DeviceProfile.RGBProfile = "wave"
	wave := d.Rgb.Profiles["wave"]
	delete(d.Rgb.Profiles, "wave")
	reports := make(chan []byte, 64)
	d.lightingReport = func(report []byte) error {
		select {
		case reports <- report:
		default:
		}
		return nil
	}
	d.setDeviceColor(false)
	done := rendererDone(d)
	select {
	case <-done:
		t.Fatal("fallback missing profile exited")
	case <-time.After(90 * time.Millisecond):
	}
	d.rgbMutex.Lock()
	d.Rgb.Profiles["wave"] = wave
	d.rgbMutex.Unlock()
	waitHarpoonReport(t, reports)
	colors := d.GetRgbProfiles().(rgb.RGB)
	gradient := colors.Profiles["gradient"]
	gradient.Gradients[0] = rgb.Color{}
	if d.GetRgbProfile("gradient").Gradients[0] == (rgb.Color{}) {
		t.Fatal("fallback gradient alias")
	}
	for i := 0; i < 3; i++ {
		if status, _ := d.ProcessNewGradientColor("gradient"); status != 1 {
			t.Fatal("fallback gradient add")
		}
		if status, _ := d.ProcessDeleteGradientColor("gradient"); status != 1 {
			t.Fatal("fallback gradient delete")
		}
	}
	d.stopLighting()
	awaitHarpoonRetirement(t, rendererDone(d))
}

func TestHarpoonRestartDoesNotWaitForStalledOldOutput(t *testing.T) {
	d, _, _ := newHarpoonLightingFixture(t, true)
	if err := d.SetLightingEffect("wave"); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	reports := make(chan []byte, 32)
	d.lightingReport = func(report []byte) error {
		once.Do(func() { close(entered); <-release })
		select {
		case reports <- report:
		default:
		}
		return nil
	}
	d.lightingRestart = nil
	d.setDeviceColor(false)
	oldDone := rendererDone(d)
	<-entered
	changed := make(chan error, 1)
	go func() { changed <- d.SetLightingEffect("mouse") }()
	select {
	case err := <-changed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(200 * time.Millisecond):
		close(release)
		t.Fatal("mutation waited on old output")
	}
	newDone := rendererDone(d)
	close(release)
	awaitHarpoonRetirement(t, oldDone)
	awaitHarpoonRetirement(t, newDone)
	d.rendererMu.Lock()
	active := d.activeRgb
	d.rendererMu.Unlock()
	if active != nil {
		t.Fatal("stale owner survived")
	}
}

func TestHarpoonDpiFlashTimingAndInputCommands(t *testing.T) {
	d, _, _ := newHarpoonLightingFixture(t, true)
	reports := make(chan []byte, 64)
	d.lightingReport = func(report []byte) error {
		select {
		case reports <- report:
		default:
		}
		return nil
	}
	d.setDeviceColor(true)
	flash := waitHarpoonReport(t, reports)
	start := time.Now()
	if !bytes.Equal(flash[6:9], []byte{160, 75, 25}) {
		t.Fatal("wrong flash")
	}
	logo := waitHarpoonReport(t, reports)
	if time.Since(start) < 450*time.Millisecond || !bytes.Equal(logo[6:9], []byte{126, 63, 25}) {
		t.Fatal("500-ms restoration", logo)
	}
	awaitHarpoonRetirement(t, rendererDone(d))
	d.toggleDPI(false)
	input := waitHarpoonReport(t, reports)
	if !bytes.Equal(input[:6], []byte{0, 7, 0x13, 2, 0, 2}) {
		t.Fatal("DPI command changed", input)
	}
	waitHarpoonReport(t, reports)
	awaitHarpoonRetirement(t, rendererDone(d))
	d.sniperMode(true)
	input = waitHarpoonReport(t, reports)
	if !bytes.Equal(input[:6], []byte{0, 7, 0x13, 2, 0, 0}) {
		t.Fatal("Sniper command changed", input)
	}
	waitHarpoonReport(t, reports)
	waitHarpoonReport(t, reports)
	awaitHarpoonRetirement(t, rendererDone(d))
	if !d.SniperMode || d.DeviceProfile.Profile != 1 {
		t.Fatal("input state changed")
	}
	d.sniperMode(false)
	input = waitHarpoonReport(t, reports)
	if !bytes.Equal(input[:6], []byte{0, 7, 0x13, 2, 0, 2}) {
		t.Fatal("Sniper restore input", input)
	}
	waitHarpoonReport(t, reports)
}

func TestHarpoonConcurrentRendererAndCanonicalMutations(t *testing.T) {
	d, _, _ := newHarpoonLightingFixture(t, true)
	if err := d.SetLightingEffect("gradient"); err != nil {
		t.Fatal(err)
	}
	d.lightingRestart = nil
	reports := make(chan []byte, 512)
	d.lightingReport = func(report []byte) error {
		select {
		case reports <- report:
		default:
		}
		return nil
	}
	d.setDeviceColor(false)
	var workers sync.WaitGroup
	for _, action := range []func(int){
		func(i int) {
			if err := d.SetLightingBrightness(uint8(30 + i)); err != nil {
				t.Error(err)
			}
		},
		func(i int) {
			settings, err := d.ResolveLightingEffectSettings("gradient")
			if err != nil {
				t.Error(err)
				return
			}
			settings.Gradient.Stops[0].Intensity = .4
			if err = d.SetLightingEffectSettings("gradient", settings); err != nil {
				t.Error(err)
			}
		},
		func(i int) { d.SchedulerBrightness(0); d.SchedulerBrightness(255) },
		func(i int) {
			if err := d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: float64(i)}); err != nil {
				t.Error(err)
			}
			d.GetRgbProfiles()
			d.LightingSnapshot()
		},
	} {
		workers.Add(1)
		go func(action func(int)) {
			defer workers.Done()
			for i := 0; i < 8; i++ {
				action(i)
			}
		}(action)
	}
	workers.Wait()
	waitHarpoonReport(t, reports)
	d.stopLighting()
	awaitHarpoonRetirement(t, rendererDone(d))
}
