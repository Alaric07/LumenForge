package katarpro

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

	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"github.com/sstallion/go-hid"
)

var katarCatalogue = []string{"colorpulse", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}

func katarFixture(t *testing.T, attach bool) (*Device, *lightingsettings.IndependentDeviceRuntime, config.Paths) {
	t.Helper()
	logger.Init()
	previous := pwd
	pwd = t.TempDir()
	t.Cleanup(func() { pwd = previous })
	for _, dir := range []string{"profiles", "rgb"} {
		must(t, os.MkdirAll(filepath.Join(pwd, "database", dir), 0700))
	}
	paths := config.Paths{ShippedDatabaseRoot: "../../../database", OpenRGBDeviceLightingFile: filepath.Join(pwd, "canonical-state.json"), DeviceEffectSettingsFile: filepath.Join(pwd, "canonical-effects.json")}
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile, filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	must(t, err)
	data, err := os.ReadFile("../../../database/rgb.json")
	must(t, err)
	var profiles rgb.RGB
	must(t, json.Unmarshal(data, &profiles))
	brightness := uint8(23)
	profile := &DeviceProfile{Active: true, Product: "KATAR PRO", Serial: "KatarTEST", Path: filepath.Join(pwd, "database/profiles/KatarTEST.json"), RGBProfile: "wave", BrightnessSlider: &brightness, OriginalBrightness: 91, RgbOff: true, Profile: 1, PollingRate: 4, SleepMode: 15, Profiles: map[int]DPIProfile{
		0: {Name: "Stage 1", Value: 800, Color: &rgb.Color{Red: 255}, ColorIndex: map[int][]int{0: {0, 1, 2}}},
		1: {Name: "Stage 2", Value: 1500, Color: &rgb.Color{Red: 123, Green: 45, Blue: 67}, ColorIndex: map[int][]int{0: {0, 1, 2}}},
		2: {Name: "Stage 3", Value: 3000, Color: &rgb.Color{Green: 255}, ColorIndex: map[int][]int{0: {0, 1, 2}}},
		3: {Name: "Sniper", Value: 200, Sniper: true, Color: &rgb.Color{Red: 120, Green: 80, Blue: 40}, ColorIndex: map[int][]int{0: {0, 1, 2}}},
	}, ZoneColors: map[int]ZoneColors{0: {Name: "Scroll", ColorIndex: []int{0, 1, 2}, Color: &rgb.Color{Red: 1, Green: 2, Blue: 3}}}}
	d := &Device{Serial: profile.Serial, Product: profile.Product, dev: &hid.Device{}, Connected: true, Usb: true, LEDChannels: 1, ChangeableLedChannels: 1, ZoneAmount: 1, MinDPI: 200, MaxDPI: 12400, Rgb: &profiles, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"default": profile}, lightingRestart: func() {}}
	must(t, os.WriteFile(profile.Path, []byte("legacy profile sentinel"), 0600))
	must(t, os.WriteFile(filepath.Join(pwd, "database/rgb", d.Serial+".json"), []byte("legacy rgb sentinel"), 0600))
	if attach {
		must(t, d.attachLightingSource(runtime))
	}
	t.Cleanup(func() { d.stopLighting(); awaitRenderer(t, rendererDone(d)) })
	return d, runtime, paths
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func rendererDone(d *Device) chan struct{} {
	d.rendererMu.Lock()
	defer d.rendererMu.Unlock()
	return d.rendererDone
}
func awaitRenderer(t *testing.T, done <-chan struct{}) {
	t.Helper()
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("renderer did not retire")
	}
}
func frame(t *testing.T, d *Device) []byte {
	t.Helper()
	v, err := d.lightingFrameState()
	must(t, err)
	owner := rgb.Exit()
	owner.RGBStartColor = &rgb.Color{Red: 50}
	owner.RGBEndColor = &rgb.Color{Blue: 80}
	now := time.Now()
	return d.renderLightingFrame(v, owner, &now)
}

func TestKatarCanonicalCleanDefaultsCatalogueDispatchAndTopology(t *testing.T) {
	d, rt, paths := katarFixture(t, true)
	snap, ok := d.LightingSnapshot()
	if !ok || snap.ConfiguredEffect != "static" || snap.Brightness != 100 || len(snap.SupportedEffects) != 19 {
		t.Fatalf("snapshot=%#v", snap)
	}
	if _, found, err := rt.State.Resolve(d.Serial); err != nil || found {
		t.Fatal("attachment wrote/imported legacy state")
	}
	if _, err := os.Stat(paths.DeviceEffectSettingsFile); !os.IsNotExist(err) {
		t.Fatal("attachment persisted settings")
	}
	var got []string
	for _, e := range snap.SupportedEffects {
		got = append(got, e.ID)
	}
	if !reflect.DeepEqual(got, katarCatalogue) || !reflect.DeepEqual(rgbModes, katarCatalogue) {
		t.Fatalf("catalogue=%v", got)
	}
	for _, effect := range katarCatalogue {
		must(t, d.SetLightingEffect(effect))
		snap, ok = d.LightingSnapshot()
		if !ok || !snap.EffectSupported {
			t.Fatalf("unpresented %s", effect)
		}
		if value := frame(t, d); len(value) != 3 {
			t.Fatalf("missing dispatch %s: %v", effect, value)
		}
	}
	for _, effect := range []string{"colorshift", "circle", "circleshift", "spinner", "pastelspiralrainbow", "custom"} {
		if d.SupportsLightingEffect(effect) || d.SetLightingEffect(effect) == nil {
			t.Fatalf("unexpected effect %s", effect)
		}
	}
	must(t, d.SetLightingEffect("mouse"))
	snap, _ = d.LightingSnapshot()
	if snap.AuthoredZoneEditor == nil || len(snap.AuthoredZoneEditor.Zones) != 1 || snap.AuthoredZoneEditor.Zones[0].ID != "0" || snap.AuthoredZoneEditor.Zones[0].Label != "Scroll" || snap.AuthoredZoneEditor.Zones[0].ColorHex != "#00ffff" {
		t.Fatalf("zones=%#v", snap)
	}
	v, err := d.lightingFrameState()
	must(t, err)
	if len(v.zones) != 1 || !reflect.DeepEqual(v.zones[0].ColorIndex, []int{0, 1, 2}) || !bytes.Equal(frame(t, d), []byte{0, 255, 255}) {
		t.Fatalf("topology=%#v", v)
	}
	if snap.ClusterControlled || snap.ExternalControlled || len(snap.Channels) != 0 {
		t.Fatal("invented capabilities")
	}
}

func TestKatarCanonicalAuthorityPersistenceResetAndCopies(t *testing.T) {
	d, _, paths := katarFixture(t, true)
	profileBefore := cloneKatarProfile(*d.DeviceProfile)
	must(t, d.SetLightingEffect("mouse"))
	must(t, d.SetLightingBrightness(60))
	must(t, d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 200, Green: 100, Blue: 50}))
	settings, err := d.ResolveLightingEffectSettings("gradient")
	must(t, err)
	settings.Gradient.Stops[0].Color.Red = 7
	must(t, d.SetLightingEffectSettings("gradient", settings))
	saved := settings.Clone()
	settings.Gradient.Stops[0].Color.Red = 199
	got, err := d.ResolveLightingEffectSettings("gradient")
	must(t, err)
	if !reflect.DeepEqual(got, saved) {
		t.Fatal("setter retained caller slice")
	}
	got.Gradient.Stops[0].Color.Red = 198
	snap, _ := d.LightingSnapshot()
	snap.AuthoredZoneEditor.Zones[0].ColorHex = "#ffffff"
	snap.SupportedEffects[0].ID = "bad"
	// Poison all legacy Lighting authority; canonical snapshots/output remain usable.
	d.DeviceProfile.RGBProfile = "unknown"
	d.DeviceProfile.BrightnessSlider = nil
	d.DeviceProfile.ZoneColors = nil
	d.DeviceProfile.RgbOff = true
	d.Rgb = nil
	snap, ok := d.LightingSnapshot()
	if !ok || snap.ConfiguredEffect != "mouse" || snap.Brightness != 60 || snap.AuthoredZoneEditor.Zones[0].ColorHex != "#c86432" {
		t.Fatalf("legacy authority leaked: %#v", snap)
	}
	if !bytes.Equal(frame(t, d), []byte{120, 60, 30}) {
		t.Fatal("renderer read legacy data")
	}
	if d.GetRgbProfile("gradient").Gradients[0].Red != 7 {
		t.Fatal("legacy getter failed canonical projection")
	}
	must(t, d.ResetLightingEffectSettings("gradient"))
	snapProfile := d.GetRgbProfiles().(rgb.RGB)
	if len(snapProfile.Profiles) != 19 {
		t.Fatal("canonical compatibility catalogue")
	}
	// Reopen the files, not the runtime cache, to prove persistence.
	state, err := lightingsettings.LoadIndependentDeviceStateStore(paths.OpenRGBDeviceLightingFile)
	must(t, err)
	target, found, err := state.Resolve(d.Serial)
	must(t, err)
	if !found || target.SelectedEffect != "mouse" || target.Brightness != 60 {
		t.Fatalf("persisted=%#v", target)
	}
	effects, err := lightingsettings.LoadDeviceStore(paths.DeviceEffectSettingsFile)
	must(t, err)
	palette, found, err := effects.GetAuthoredZones(d.Serial, "mouse")
	must(t, err)
	if !found || palette["0"].Red != 200 {
		t.Fatal("palette not persisted")
	}
	if _, found, err = effects.Get(d.Serial, "gradient"); err != nil || found {
		t.Fatal("reset did not remove customization")
	}
	for path, want := range map[string]string{profileBefore.Path: "legacy profile sentinel", filepath.Join(pwd, "database/rgb", d.Serial+".json"): "legacy rgb sentinel"} {
		data, err := os.ReadFile(path)
		must(t, err)
		if string(data) != want {
			t.Fatal("canonical mutation dual-wrote legacy")
		}
	}
	if d.DeviceProfile.Profiles[1].Value != profileBefore.Profiles[1].Value || d.DeviceProfile.OriginalBrightness != 91 {
		t.Fatal("canonical edit changed input/metadata")
	}
}

func TestKatarPersistenceFailureAtomicity(t *testing.T) {
	for _, kind := range []string{"state", "effect", "gradient", "zone", "reset"} {
		t.Run(kind, func(t *testing.T) {
			d, rt, paths := katarFixture(t, true)
			must(t, d.SetLightingEffect("mouse"))
			must(t, d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 90, Green: 80, Blue: 70}))
			settings, err := d.ResolveLightingEffectSettings("gradient")
			must(t, err)
			must(t, d.SetLightingEffectSettings("gradient", settings))
			before, _ := d.LightingSnapshot()
			input := cloneKatarProfile(*d.DeviceProfile)
			stateBefore, _, _ := rt.State.Resolve(d.Serial)
			gradientBefore, _ := d.ResolveLightingEffectSettings("gradient")
			restarts := 0
			d.lightingRestart = func() { restarts++ }
			path := paths.DeviceEffectSettingsFile
			if kind == "state" {
				path = paths.OpenRGBDeviceLightingFile
			}
			must(t, os.Remove(path))
			must(t, os.Mkdir(path, 0700)) // deterministic rename failure, even as root
			switch kind {
			case "state":
				if d.SetLightingBrightness(9) == nil || d.SetLightingEffect("wave") == nil {
					t.Fatal("state failure hidden")
				}
			case "effect":
				settings.Gradient.Stops[0].Color.Red = 45
				if d.SetLightingEffectSettings("gradient", settings) == nil {
					t.Fatal("effect failure hidden")
				}
			case "gradient":
				if code, _ := d.ProcessNewGradientColor("gradient"); code != 0 {
					t.Fatal("add failure hidden")
				}
				if code, _ := d.ProcessDeleteGradientColor("gradient"); code != 0 {
					t.Fatal("delete failure hidden")
				}
			case "zone":
				if d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 1}) == nil {
					t.Fatal("zone failure hidden")
				}
			case "reset":
				if d.ResetLightingEffectSettings("gradient") == nil {
					t.Fatal("reset failure hidden")
				}
			}
			after, _ := d.LightingSnapshot()
			stateAfter, _, _ := rt.State.Resolve(d.Serial)
			gradientAfter, _ := d.ResolveLightingEffectSettings("gradient")
			if restarts != 0 || !reflect.DeepEqual(before, after) || stateBefore != stateAfter || !reflect.DeepEqual(gradientBefore, gradientAfter) || !reflect.DeepEqual(input, *d.DeviceProfile) {
				t.Fatal("failure published/restarted/changed input")
			}
		})
	}
}

type gatedState struct {
	lightingsettings.IndependentDeviceStateAccess
	entered, release chan struct{}
}

func (s *gatedState) Set(id string, value lightingsettings.IndependentDeviceLightingState) error {
	close(s.entered)
	<-s.release
	return errors.New("injected state writer failure")
}
func TestKatarPersistBeforePublication(t *testing.T) {
	d, rt, _ := katarFixture(t, true)
	before, _, _ := rt.State.Resolve(d.Serial)
	gate := &gatedState{rt.State, make(chan struct{}), make(chan struct{})}
	d.lightingCanonical.state = gate
	var restarts atomic.Int32
	d.lightingRestart = func() { restarts.Add(1) }
	done := make(chan error, 1)
	go func() { done <- d.SetLightingEffect("wave") }()
	<-gate.entered
	current, _, err := rt.State.Resolve(d.Serial)
	must(t, err)
	if current != before || restarts.Load() != 0 {
		t.Fatal("published before persistence")
	}
	close(gate.release)
	if <-done == nil || restarts.Load() != 0 {
		t.Fatal("failed persist restarted")
	}
}

func TestKatarTransientOverridesAndStoppedSemantics(t *testing.T) {
	d, _, _ := katarFixture(t, true)
	must(t, d.SetLightingEffect("mouse"))
	must(t, d.SetLightingBrightness(70))
	if d.SchedulerBrightness(0) != 1 {
		t.Fatal("scheduler")
	}
	must(t, d.SetLightingBrightness(40))
	if !bytes.Equal(frame(t, d), []byte{0, 0, 0}) {
		t.Fatal("scheduler did not stay dark")
	}
	d.ControlDeviceRgb(true)
	if d.SchedulerBrightness(255) != 1 || !bytes.Equal(frame(t, d), []byte{0, 0, 0}) {
		t.Fatal("scheduler cleared user off")
	}
	must(t, d.SetLightingBrightness(80))
	must(t, d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 200}))
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(false)
	if !bytes.Equal(frame(t, d), []byte{0, 0, 0}) {
		t.Fatal("user off cleared scheduler")
	}
	d.SchedulerBrightness(1)
	if !bytes.Equal(frame(t, d), []byte{160, 0, 0}) {
		t.Fatal("did not restore latest desired")
	}
	snap, _ := d.LightingSnapshot()
	if snap.Brightness != 80 || snap.ConfiguredEffect != "mouse" || d.DeviceProfile.OriginalBrightness != 91 {
		t.Fatal("override changed desired/metadata")
	}
	d.lightingMu.Lock()
	d.Exit = true
	d.lightingMu.Unlock()
	if d.SchedulerBrightness(0) != 0 || d.SetLightingBrightness(1) == nil {
		t.Fatal("stopped accepted mutation")
	}
	d.ControlDeviceRgb(true)
	if d.schedulerDark || d.userRGBOff {
		t.Fatal("stopped override recorded contrary to source")
	}
	if _, ok := d.LightingSnapshot(); ok {
		t.Fatal("stopped snapshot")
	}
}

func TestKatarSniperAndFrameDefensiveCopies(t *testing.T) {
	d, _, _ := katarFixture(t, true)
	must(t, d.SetLightingEffect("mouse"))
	must(t, d.SetLightingBrightness(50))
	must(t, d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 200, Green: 100, Blue: 40}))
	d.lightingMu.Lock()
	d.SniperMode = true
	d.lightingMu.Unlock()
	before := cloneKatarProfile(*d.DeviceProfile)
	if !bytes.Equal(frame(t, d), []byte{60, 40, 20}) {
		t.Fatal("Mouse Sniper substitution")
	}
	snap, _ := d.LightingSnapshot()
	if snap.AuthoredZoneEditor.Zones[0].ColorHex != "#c86428" {
		t.Fatal("Sniper overwrote authored palette")
	}
	must(t, d.SetLightingEffect("static"))
	settings, err := d.ResolveLightingEffectSettings("static")
	must(t, err)
	settings.SingleColor.Color = lightingsettings.Color{Green: 200}
	must(t, d.SetLightingEffectSettings("static", settings))
	if !bytes.Equal(frame(t, d), []byte{0, 100, 0}) {
		t.Fatal("Sniper leaked into static")
	}
	v, err := d.lightingFrameState()
	must(t, err)
	v.indicator.Red = 1
	v.indicatorIndexes[0][0] = 2
	v.zones[0].Color.Red = 1
	v.zones[0].ColorIndex[0] = 2
	if !reflect.DeepEqual(before, *d.DeviceProfile) {
		t.Fatal("output mutated pointer-backed input")
	}
	returned := d.GetRgbProfile("gradient")
	returned.Gradients[0] = rgb.Color{Red: 3}
	if d.GetRgbProfile("gradient").Gradients[0].Red == 3 {
		t.Fatal("getter leaked gradient")
	}
}

func TestKatarHardwareReportAndDPIFlash(t *testing.T) {
	d, _, _ := katarFixture(t, true)
	reports := make(chan []byte, 12)
	d.lightingReport = func(report []byte) error { reports <- report; return nil }
	d.writeColor([]byte{11, 22, 33})
	got := <-reports
	want := make([]byte, 65)
	copy(want, []byte{0, 8, 6, 0, 3, 0, 0, 0, 11, 22, 33})
	if !bytes.Equal(got, want) {
		t.Fatalf("report=%v", got)
	}
	must(t, d.SetLightingEffect("mouse"))
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(true)
	d.lightingRestart = nil
	started := time.Now()
	d.toggleDPI(false)
	dpi := <-reports
	if !bytes.Equal(dpi[1:8], []byte{8, 1, 0x20, 0, 0xdc, 5, 0}) {
		t.Fatalf("DPI command=%v", dpi)
	}
	flash := <-reports
	if !bytes.Equal(flash[8:11], []byte{123, 45, 67}) {
		t.Fatal("DPI flash scaled/hidden")
	}
	restored := <-reports
	if elapsed := time.Since(started); elapsed < dpiFlashDuration || elapsed > 1500*time.Millisecond {
		t.Fatalf("flash duration=%s", elapsed)
	}
	if !bytes.Equal(restored[8:11], []byte{0, 0, 0}) {
		t.Fatal("flash did not restore off")
	}
	awaitRenderer(t, rendererDone(d))
	if dpiFlashDuration != 500*time.Millisecond {
		t.Fatal("changed flash contract")
	}
	d.lightingMu.Lock()
	d.Exit = true
	d.lightingMu.Unlock()
	d.writeColor([]byte{1, 2, 3})
	select {
	case <-reports:
		t.Fatal("stopped write")
	default:
	}
}

func TestKatarProfileDefaultsAndCanonicalProfileIsolation(t *testing.T) {
	d, _, _ := katarFixture(t, true)
	d.lightingMu.Lock()
	d.DeviceProfile = nil
	d.lightingMu.Unlock()
	d.saveDeviceProfile()
	p := d.DeviceProfile
	if p.RGBProfile != "mouse" || *p.BrightnessSlider != 100 || p.OriginalBrightness != 100 || p.PollingRate != 4 || p.SleepMode != 15 || p.Profile != 1 || p.ButtonOptimization != 0 || p.Profiles[1].Value != 1500 || p.Profiles[2].Value != 3000 || !p.Profiles[3].Sniper || p.Profiles[3].Value != 200 {
		t.Fatalf("defaults=%#v", p)
	}
	p.BrightnessSlider = nil
	p.PollingRate = 0
	p.SleepMode = 0
	delete(p.Profiles, 3)
	d.saveDeviceProfile()
	p = d.DeviceProfile
	if p.BrightnessSlider == nil || *p.BrightnessSlider != 100 || p.PollingRate != 4 || p.SleepMode != 15 || !p.Profiles[3].Sniper {
		t.Fatal("upgrades")
	}
	must(t, d.SetLightingEffect("mouse"))
	must(t, d.SetLightingBrightness(65))
	must(t, d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Blue: 200}))
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(true)
	other := cloneKatarProfile(*p)
	other.Path = filepath.Join(pwd, "database/profiles", d.Serial+"-gaming.json")
	other.RGBProfile = "rainbow"
	value := uint8(1)
	other.BrightnessSlider = &value
	other.ZoneColors[0].Color.Red = 99
	other.PollingRate = 2
	other.RgbOff = true
	d.UserProfiles["gaming"] = &other
	d.lightingProfileChanged = func() {}
	d.lightingReport = func([]byte) error { return nil }
	if d.ChangeDeviceProfile("gaming") != 1 {
		t.Fatal("profile switch")
	}
	awaitRenderer(t, rendererDone(d))
	snap, ok := d.LightingSnapshot()
	if !ok || snap.ConfiguredEffect != "mouse" || snap.Brightness != 65 || snap.AuthoredZoneEditor.Zones[0].ColorHex != "#0000c8" || !d.schedulerDark || !d.userRGBOff || d.DeviceProfile.PollingRate != 2 {
		t.Fatalf("profile stole canonical authority: %#v", snap)
	}
}

func TestKatarFailedAttachmentLegacyFallbackAndReadiness(t *testing.T) {
	d, rt, _ := katarFixture(t, false)
	if err := rt.State.Set(d.Serial, lightingsettings.IndependentDeviceLightingState{SelectedEffect: "colorshift", Brightness: 20}); err != nil {
		t.Fatal(err)
	}
	if d.attachLightingSource(rt) == nil || d.lightingAttached() {
		t.Fatal("published unusable attachment")
	}
	if _, ok := d.LightingSnapshot(); ok {
		t.Fatal("failed attachment exposed canonical Lighting")
	}
	if _, ok := d.DPISnapshot(); !ok {
		t.Fatal("failed attachment broke DPI")
	}
	// Fallback accepts source-backed legacy settings even when canonical rejects them.
	p := d.Rgb.Profiles["static"]
	p.StartColor = rgb.Color{Red: 80, Green: 40, Blue: 20}
	d.Rgb.Profiles["static"] = p
	d.DeviceProfile.RgbOff = false
	d.DeviceProfile.RGBProfile = "static"
	if !bytes.Equal(frame(t, d), []byte{18, 9, 4}) {
		t.Fatalf("fallback frame=%v", frame(t, d))
	}
	d.lightingReport = func([]byte) error { return nil }
	if d.UpdateRgbProfile(0, "mouse") != 1 {
		t.Fatal("legacy effect mutation")
	}
	awaitRenderer(t, rendererDone(d))
	_, err := rt.State.Delete(d.Serial)
	must(t, err)
	must(t, d.attachLightingSource(rt))
	for _, invalidate := range []func(){func() { d.Connected = false }, func() { d.dev = nil }, func() { d.LEDChannels = 2 }, func() { d.ChangeableLedChannels = 2 }, func() { d.ZoneAmount = 2 }, func() { d.DeviceProfile = nil }} {
		savedProfile := d.DeviceProfile
		invalidate()
		if _, ok := d.LightingSnapshot(); ok || d.SetLightingEffect("mouse") == nil {
			t.Fatal("readiness guard")
		}
		d.Connected = true
		d.dev = &hid.Device{}
		d.LEDChannels = 1
		d.ChangeableLedChannels = 1
		d.ZoneAmount = 1
		d.DeviceProfile = savedProfile
	}
}

// Transient resolver failure must pace retry and recover without a new mutation.
type temporaryResolver struct {
	katarSettingsResolver
	missing atomic.Bool
	calls   atomic.Int32
}

func (r *temporaryResolver) Resolve(target lightingsettings.Target, effect string) (lightingsettings.Resolution, error) {
	r.calls.Add(1)
	if r.missing.Load() {
		return lightingsettings.Resolution{}, errors.New("temporary missing composition")
	}
	return r.katarSettingsResolver.Resolve(target, effect)
}
func TestKatarRendererRecoveryRetirementAndSupersededFrames(t *testing.T) {
	d, _, _ := katarFixture(t, true)
	must(t, d.SetLightingEffect("static"))
	resolver := &temporaryResolver{katarSettingsResolver: d.lightingCanonical.resolver}
	resolver.missing.Store(true)
	d.lightingCanonical.resolver = resolver
	reports := make(chan []byte, 30)
	d.lightingReport = func(report []byte) error { reports <- report; return nil }
	d.lightingRestart = nil
	d.setDeviceColor()
	done := rendererDone(d)
	time.Sleep(100 * time.Millisecond)
	if calls := resolver.calls.Load(); calls < 2 || calls > 5 {
		t.Fatalf("retry not paced: %d", calls)
	}
	resolver.missing.Store(false)
	select {
	case <-reports:
	case <-time.After(time.Second):
		t.Fatal("no recovery")
	}
	awaitRenderer(t, done)
	d.rendererMu.Lock()
	dead := d.activeRgb != nil || d.rendererStop != nil
	d.rendererMu.Unlock()
	if dead {
		t.Fatal("dead renderer retained ownership")
	}
	// Stale retirement cannot clear newer renderer ownership.
	old, oldStop, oldDone := d.startLightingRenderer()
	newOwner, newStop, newDone := d.startLightingRenderer()
	d.retireLightingRenderer(old, oldStop)
	close(oldDone)
	if !d.rendererCurrent(newStop) {
		t.Fatal("stale retirement cleared current renderer")
	}
	d.retireLightingRenderer(newOwner, newStop)
	close(newDone)
	// Queue a frame behind the existing transport lock, supersede its owner, then
	// allow it through. The transport boundary must drop it after lock acquisition.
	_, stale, staleDone := d.startLightingRenderer()
	d.mutex.Lock()
	writeDone := make(chan struct{})
	go func() { d.writeColor([]byte{77, 88, 99}, stale); close(writeDone) }()
	_, current, currentDone := d.startLightingRenderer()
	d.mutex.Unlock()
	<-writeDone
	close(staleDone)
	select {
	case <-reports:
		t.Fatal("superseded frame reached report hook")
	default:
	}
	d.writeColor([]byte{1, 2, 3}, current)
	if got := <-reports; !bytes.Equal(got[8:11], []byte{1, 2, 3}) {
		t.Fatal("current frame lost")
	}
	d.stopLighting()
	close(currentDone)
	// Stop/restart must not wait for an active transport call to finish.
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	d.lightingReport = func([]byte) error { once.Do(func() { close(entered); <-release }); return nil }
	d.setDeviceColor()
	blockedDone := rendererDone(d)
	<-entered
	stopped := make(chan struct{})
	go func() { d.stopLighting(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stop blocked on transport")
	}
	d.setDeviceColor()
	freshDone := rendererDone(d)
	close(release)
	awaitRenderer(t, blockedDone)
	awaitRenderer(t, freshDone)
}

func TestKatarConcurrentRendererMutationsAndInputSnapshots(t *testing.T) {
	d, _, _ := katarFixture(t, true)
	d.lightingRestart = nil
	d.lightingReport = func([]byte) error { return nil }
	must(t, d.SetLightingEffect("wave"))
	d.saveDeviceProfile()
	other := cloneKatarProfile(*d.DeviceProfile)
	other.Path = filepath.Join(pwd, "database/profiles", d.Serial+"-other.json")
	data, err := json.Marshal(other)
	must(t, err)
	must(t, os.WriteFile(other.Path, data, 0600))
	d.UserProfiles["other"] = &other
	d.lightingProfileChanged = func() {}
	var wg sync.WaitGroup
	for worker := 0; worker < 5; worker++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 12; i++ {
				switch id {
				case 0:
					if err := d.SetLightingBrightness(uint8(20 + i)); err != nil {
						t.Error(err)
					}
				case 1:
					d.SchedulerBrightness(uint8(i % 2))
					d.ControlDeviceRgb(i%2 == 0)
				case 2:
					d.LightingSnapshot()
					d.DPISnapshot()
					d.GetRgbProfiles()
				case 4:
					if d.ChangeDeviceProfile("other") != 1 {
						t.Error("profile switch")
					}
				case 3: // Exercise the input-owned map seam concurrently with composition.
					d.lightingMu.Lock()
					if d.saveMouseDPISettingsLocked(map[int]uint16{1: 1500}, map[int]rgb.Color{1: {Red: 123, Green: 45, Blue: 67}}) != 1 {
						t.Error("input mutation")
					}
					d.lightingMu.Unlock()
				}
			}
		}(worker)
	}
	wg.Wait()
	d.stopLighting()
	awaitRenderer(t, rendererDone(d))
}

func TestKatarCompleteEditableSettingsAndGradientMutation(t *testing.T) {
	d, rt, _ := katarFixture(t, true)
	for _, effect := range []string{"static", "colorpulse", "cpu-temperature", "gpu-temperature", "gradient", "wave"} {
		must(t, d.SetLightingEffect(effect))
		value, err := d.ResolveLightingEffectSettings(effect)
		must(t, err)
		if value.Speed != nil {
			*value.Speed = 3.5
		}
		if value.SingleColor != nil {
			value.SingleColor.Color = lightingsettings.Color{Red: 24, Green: 48, Blue: 72}
		}
		if value.TwoColor != nil {
			value.TwoColor.Start.Red = 18
			value.TwoColor.End.Blue = 81
		}
		if value.Temperature != nil {
			value.Temperature.Low.Celsius = 20
			value.Temperature.Middle.Celsius = 50
			value.Temperature.High.Celsius = 90
			value.Temperature.Middle.Color.Green = 111
		}
		if value.Gradient != nil {
			value.Gradient.Stops[0].Color.Red = 42
			value.Gradient.Stops[0].Intensity = .7
		}
		must(t, d.SetLightingEffectSettings(effect, value))
		stored, found, err := rt.Effects.Get(d.Serial, effect)
		must(t, err)
		if !found || !reflect.DeepEqual(value, stored) {
			t.Fatalf("incomplete %s", effect)
		}
		snap, ok := d.LightingSnapshot()
		if !ok || !snap.Customized {
			t.Fatalf("unpresented customization %s", effect)
		}
		if value.Temperature != nil && (snap.TemperatureMiddle.Celsius != 50 || snap.TemperatureMiddle.ColorHex != lightingColorHex(value.Temperature.Middle.Color)) {
			t.Fatal("temperature contract")
		}

		must(t, d.ResetLightingEffectSettings(effect))
		defaults, err := rt.Defaults.Get(effect)
		must(t, err)
		resolved, err := d.ResolveLightingEffectSettings(effect)
		must(t, err)
		if !reflect.DeepEqual(defaults, resolved) {
			t.Fatalf("reset %s", effect)
		}
	}
	initial, err := d.ResolveLightingEffectSettings("gradient")
	must(t, err)
	if code, _ := d.ProcessNewGradientColor("gradient"); code != 1 {
		t.Fatal("canonical add gradient")
	}
	added, err := d.ResolveLightingEffectSettings("gradient")
	must(t, err)
	if len(added.Gradient.Stops) != len(initial.Gradient.Stops)+1 {
		t.Fatal("gradient not added")
	}
	if code, _ := d.ProcessDeleteGradientColor("gradient"); code != 1 {
		t.Fatal("canonical delete gradient")
	}
	remaining, _ := d.ResolveLightingEffectSettings("gradient")
	if len(remaining.Gradient.Stops) != len(initial.Gradient.Stops) {
		t.Fatal("gradient not deleted")
	}
	before := cloneKatarProfile(*d.DeviceProfile)
	restarts := 0
	d.lightingRestart = func() { restarts++ }
	if d.ChangeDeviceBrightness(2) != 1 || restarts != 0 || !reflect.DeepEqual(before, *d.DeviceProfile) {
		t.Fatal("obsolete brightness metadata changed canonical state")
	}
}
