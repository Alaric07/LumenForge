package glaivergb

import (
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
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
)

var auditedEffects = []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}

func requireLighting(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func lightingFixture(t *testing.T, attach bool) (*Device, *lightingsettings.IndependentDeviceRuntime) {
	t.Helper()
	logger.Init()
	root := t.TempDir()
	paths := config.Paths{ShippedDatabaseRoot: "../../../database", OpenRGBDeviceLightingFile: filepath.Join(root, "state.json"), DeviceEffectSettingsFile: filepath.Join(root, "effects.json")}
	rt, err := lightingsettings.LoadIndependentDeviceRuntime(paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile, filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	requireLighting(t, err)
	b := uint8(17)
	p := &DeviceProfile{Active: true, Serial: "glaive-test", Product: "GLAIVE RGB", Path: filepath.Join(root, "profile.json"), RGBProfile: "gradient", BrightnessSlider: &b, OriginalBrightness: 82, Profile: 1, PollingRate: 1, LiftHeight: 3, ButtonOptimization: 2, ZoneColors: map[int]ZoneColors{0: {Name: "legacy", Color: &rgb.Color{Red: 9}, ColorIndex: []int{99}}}, Profiles: map[int]DPIProfile{0: {Name: "Sniper", Value: 400, Sniper: true, PackerIndex: 6}, 1: {Name: "Stage 1", Value: 800, PackerIndex: 1}}}
	d := &Device{Serial: p.Serial, Product: p.Product, VendorId: 6940, ProductId: 6964, ChangeableLedChannels: 3, ZoneAmount: 3, DPIAmount: 6, DeviceProfile: p, UserProfiles: map[string]*DeviceProfile{"default": p}, Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{"static": {StartColor: rgb.Color{Red: 5}}}}}
	d.lightingClusterRemove = func(string) {}
	d.lightingClusterRegister = func(*common.ClusterController) {}
	d.lightingOpenRGBRegister = func(*common.OpenRGBController, bool) {}
	requireLighting(t, os.WriteFile(p.Path, []byte("profile sentinel"), 0600))
	if attach {
		requireLighting(t, d.attachLightingSource(rt))
	}
	t.Cleanup(func() { d.exitLighting() })
	return d, rt
}
func frameState(t *testing.T, d *Device) lightingFrameState {
	t.Helper()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	v, err := d.lightingFrameLocked()
	requireLighting(t, err)
	return v
}
func rendererOwner() *rgb.ActiveRGB {
	r := rgb.Exit()
	r.RGBStartColor = &rgb.Color{Red: 255}
	r.RGBEndColor = &rgb.Color{Blue: 255}
	return r
}
func inputJSON(t *testing.T, d *Device) []byte {
	t.Helper()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	v, e := json.Marshal(d.DeviceProfile)
	requireLighting(t, e)
	return v
}

func TestCanonicalCatalogueTopologyAndEveryDispatch(t *testing.T) {
	d, rt := lightingFixture(t, true)
	if !reflect.DeepEqual(rgbModes, auditedEffects) || d.LEDChannels != 0 || d.ChangeableLedChannels != 3 || d.ZoneAmount != 3 || d.ProductId != 6964 || d.VendorId != 6940 {
		t.Fatal("identity/catalogue changed")
	}
	s, ok := d.LightingSnapshot()
	if !ok || s.ConfiguredEffect != "static" || s.Brightness != 100 || s.SingleColorHex != "#00ffff" {
		t.Fatalf("defaults %#v", s)
	}
	var names []string
	for _, e := range s.SupportedEffects {
		names = append(names, e.ID)
	}
	if !reflect.DeepEqual(names, auditedEffects) {
		t.Fatal(names)
	}
	if _, found, err := rt.State.Resolve(d.Serial); err != nil || found {
		t.Fatal("attachment imported legacy state")
	}
	for _, effect := range auditedEffects {
		requireLighting(t, d.SetLightingEffect(effect))
		s, ok = d.LightingSnapshot()
		if !ok || !s.EffectSupported {
			t.Fatal("presentation", effect)
		}
		start := time.Now()
		out := renderLightingFrame(frameState(t, d), rendererOwner(), &start)
		if len(out) != 9 {
			t.Fatal("missing dispatch", effect, len(out))
		}
	}
	requireLighting(t, d.SetLightingEffect("mouse"))
	s, ok = d.LightingSnapshot()
	if !ok || s.AuthoredZoneEditor == nil {
		t.Fatal("mouse editor")
	}
	want := []struct {
		name          string
		selector, pos int
		indices       []int
		color         string
	}{{"Front", 1, 0, []int{1, 2, 3}, "#ff0000"}, {"Logo", 2, 4, []int{5, 6, 7}, "#ffff00"}, {"Side", 6, 8, []int{9, 10, 11}, "#00ffff"}}
	for id, z := range lightingTopology() {
		w := want[id]
		if z.Name != w.name || z.LEDIndex != w.selector || z.LEDIndexPosition != w.pos || !reflect.DeepEqual(z.ColorIndex, w.indices) || s.AuthoredZoneEditor.Zones[id].ColorHex != w.color || s.AuthoredZoneEditor.Zones[id].ID != string(rune('0'+id)) {
			t.Fatalf("zone %#v", z)
		}
	}
	packet := composeLightingPacket([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
	if !bytes.Equal(packet, []byte{1, 1, 2, 3, 2, 4, 5, 6, 6, 7, 8, 9, 0, 0, 0, 0}) {
		t.Fatal(packet)
	}
	top := lightingTopology()
	top[0].ColorIndex[0] = 99
	if lightingTopology()[0].ColorIndex[0] != 1 {
		t.Fatal("topology alias")
	}
}
func TestCanonicalAuthorityCopiesAndRestart(t *testing.T) {
	d, rt := lightingFixture(t, true)
	before := inputJSON(t, d)
	requireLighting(t, d.SetLightingEffect("mouse"))
	requireLighting(t, d.SetLightingBrightness(60))
	requireLighting(t, d.SetLightingZoneColors("mouse", []string{"0", "2"}, rgb.Color{Red: 200, Green: 100, Blue: 50}))
	if !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("canonical mutation wrote legacy/input state")
	}
	disk, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	if string(disk) != "profile sentinel" {
		t.Fatal("legacy file changed")
	}
	// Direct obsolete fields cannot influence canonical rendering or presentation.
	d.DeviceProfile.RGBProfile = "off"
	*d.DeviceProfile.BrightnessSlider = 0
	d.DeviceProfile.ZoneColors[0].Color.Red = 0
	d.Rgb = nil
	s, ok := d.LightingSnapshot()
	if !ok || s.ConfiguredEffect != "mouse" || s.Brightness != 60 || s.AuthoredZoneEditor.Zones[0].ColorHex != "#c86432" {
		t.Fatal(s)
	}
	s.AuthoredZoneEditor.Zones[0].ColorHex = "bad"
	s2, _ := d.LightingSnapshot()
	if s2.AuthoredZoneEditor.Zones[0].ColorHex == "bad" {
		t.Fatal("snapshot aliases")
	}
	value, err := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, err)
	value.Gradient.Stops[0].Color.Red = 13
	again, err := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, err)
	if again.Gradient.Stops[0].Color.Red == 13 {
		t.Fatal("resolver aliases")
	}
	p := d.GetRgbProfile("gradient")
	p.Gradients[0] = rgb.Color{Red: 33}
	if d.GetRgbProfile("gradient").Gradients[0].Red == 33 {
		t.Fatal("compat getter aliases")
	}
	requireLighting(t, d.ResetLightingEffectSettings("static"))
	next := &Device{Serial: d.Serial, Product: d.Product, ChangeableLedChannels: 3, ZoneAmount: 3, DeviceProfile: &DeviceProfile{}}
	root := filepath.Dir(d.DeviceProfile.Path)
	reloadedState, err := lightingsettings.LoadIndependentDeviceStateStore(filepath.Join(root, "state.json"))
	requireLighting(t, err)
	reloadedEffects, err := lightingsettings.LoadDeviceStore(filepath.Join(root, "effects.json"))
	requireLighting(t, err)
	reloadedResolver, err := lightingsettings.NewDeviceResolver(rt.Defaults, reloadedEffects)
	requireLighting(t, err)
	requireLighting(t, next.attachLightingSource(&lightingsettings.IndependentDeviceRuntime{State: reloadedState, Effects: reloadedEffects, Defaults: rt.Defaults, Resolver: reloadedResolver}))
	ss, ok := next.LightingSnapshot()
	if !ok || ss.ConfiguredEffect != "mouse" || ss.Brightness != 60 || ss.AuthoredZoneEditor.Zones[0].ColorHex != "#c86432" {
		t.Fatal("restart", ss)
	}
}

type failingState struct {
	lightingsettings.IndependentDeviceStateAccess
}

func (f failingState) Set(string, lightingsettings.IndependentDeviceLightingState) error {
	return errors.New("state failure")
}

type failingEffects struct{ lightingEffectStore }

func (f failingEffects) Set(string, string, lightingsettings.EffectSettings) error {
	return errors.New("effect failure")
}
func (f failingEffects) Delete(string, string) (bool, error) {
	return false, errors.New("reset failure")
}
func (f failingEffects) SetAuthoredZones(string, string, map[string]lightingsettings.Color) error {
	return errors.New("palette failure")
}
func (f failingEffects) SetAuthoredZonesWithCommit(string, string, map[string]lightingsettings.Color, func() error) error {
	return errors.New("palette failure")
}
func TestPersistenceFailureLeavesStateWorkerInputAndOutput(t *testing.T) {
	d, _ := lightingFixture(t, true)
	requireLighting(t, d.SetLightingEffect("mouse"))
	before := inputJSON(t, d)
	snapshot, _ := d.LightingSnapshot()
	worker := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{}), generation: d.nativeGeneration}
	d.lightingWorker = worker
	var writes atomic.Int32
	d.lightingWrite = func([]byte) { writes.Add(1) }
	d.Connected = true
	originalState, originalEffects := d.lightingCanonical.state, d.lightingCanonical.effects
	d.lightingCanonical.state = failingState{originalState}
	if d.SetLightingEffect("rainbow") == nil || d.SetLightingBrightness(30) == nil {
		t.Fatal("state error swallowed")
	}
	d.lightingCanonical.state = originalState
	d.lightingCanonical.effects = failingEffects{originalEffects}
	settings, err := d.ResolveLightingEffectSettings("static")
	requireLighting(t, err)
	if d.SetLightingEffectSettings("static", settings) == nil || d.ResetLightingEffectSettings("static") == nil || d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 1}) == nil {
		t.Fatal("settings error swallowed")
	}
	if code, _ := d.ProcessNewGradientColor("gradient"); code != 0 {
		t.Fatal("gradient error swallowed")
	}
	if d.SaveMouseZoneColors(rgb.Color{}, map[int]rgb.Color{0: {Red: 3}}) != 0 {
		t.Fatal("legacy palette error swallowed")
	}
	d.lightingCanonical.effects = originalEffects
	d.lightingProfileWrite = func(string, []byte) error { return errors.New("profile failure") }
	if d.setUserRGBOff(true) == nil || d.ProcessSetRgbCluster(true) != 0 || d.ProcessSetOpenRgbIntegration(true) != 0 {
		t.Fatal("preference/ownership error swallowed")
	}
	after, _ := d.LightingSnapshot()
	if !reflect.DeepEqual(snapshot, after) || !bytes.Equal(before, inputJSON(t, d)) || d.lightingWorker != worker || writes.Load() != 0 {
		t.Fatal("failed transaction changed state/output")
	}
	select {
	case <-worker.stop:
		t.Fatal("failed mutation canceled renderer")
	default:
	}
}
func TestCombinedColorMutationIsAtomic(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.DeviceProfile.DPIColor = &rgb.Color{Green: 255}
	d.DeviceProfile.SniperColor = &rgb.Color{Red: 255, Green: 255}
	requireLighting(t, d.SetLightingEffect("mouse"))
	before := inputJSON(t, d)
	s, _ := d.LightingSnapshot()
	d.lightingProfileWrite = func(string, []byte) error { return errors.New("input save failure") }
	if d.SaveMouseZoneColorsSniper(rgb.Color{Red: 5}, map[int]rgb.Color{0: {Green: 50}}, rgb.Color{Blue: 5}) != 0 {
		t.Fatal("input persistence failure")
	}
	after, _ := d.LightingSnapshot()
	if !bytes.Equal(before, inputJSON(t, d)) || !reflect.DeepEqual(s, after) {
		t.Fatal("partial combined save")
	}
	d.lightingProfileWrite = nil
	if d.SaveMouseZoneColorsSniper(rgb.Color{Red: 5}, map[int]rgb.Color{0: {Green: 50}}, rgb.Color{Blue: 5}) != 1 {
		t.Fatal("combined save")
	}
	if d.DeviceProfile.DPIColor.Red != 5 || d.DeviceProfile.SniperColor.Blue != 5 {
		t.Fatal("input metadata lost")
	}
	s, _ = d.LightingSnapshot()
	if s.AuthoredZoneEditor.Zones[0].ColorHex != "#003200" {
		t.Fatal(s)
	}
	if d.DeviceProfile.ZoneColors[0].Color.Red != 9 {
		t.Fatal("dual wrote legacy palette")
	}
	clone := cloneLightingProfile(*d.DeviceProfile)
	clone.Profiles[0] = DPIProfile{Value: 999}
	clone.SniperColor.Blue = 0
	clone.ZoneColors[0].ColorIndex[0] = 1
	if d.DeviceProfile.Profiles[0].Value != 400 || d.DeviceProfile.SniperColor.Blue != 5 || d.DeviceProfile.ZoneColors[0].ColorIndex[0] != 99 {
		t.Fatal("input clone aliases")
	}
}
func TestBrightnessSchedulerRGBOffAndReplay(t *testing.T) {
	d, rt := lightingFixture(t, true)
	requireLighting(t, d.SetLightingEffect("mouse"))
	requireLighting(t, d.SetLightingBrightness(50))
	if d.SetLightingBrightness(101) == nil {
		t.Fatal("brightness range")
	}
	start := time.Now()
	out := renderLightingFrame(frameState(t, d), rendererOwner(), &start)
	if out[0] != 127 || out[3] != 127 {
		t.Fatal("mouse scaling", out)
	}
	requireLighting(t, d.setSchedulerDark(true))
	requireLighting(t, d.setSchedulerDark(true))
	requireLighting(t, d.SetLightingBrightness(73))
	if frameState(t, d).brightness != 0 {
		t.Fatal("scheduler did not hold dark")
	}
	state, _, _ := rt.State.Resolve(d.Serial)
	if state.Brightness != 73 || d.DeviceProfile.OriginalBrightness != 82 || *d.DeviceProfile.BrightnessSlider != 17 {
		t.Fatal("scheduler persisted")
	}
	requireLighting(t, d.setSchedulerDark(false))
	if frameState(t, d).brightness != 73 {
		t.Fatal("stale restore")
	}
	requireLighting(t, d.setUserRGBOff(true))
	requireLighting(t, d.SetLightingBrightness(66))
	requireLighting(t, d.setSchedulerDark(true))
	requireLighting(t, d.setUserRGBOff(false))
	if frameState(t, d).brightness != 0 {
		t.Fatal("off cleared scheduler")
	}
	requireLighting(t, d.setSchedulerDark(false))
	if frameState(t, d).brightness != 66 {
		t.Fatal("latest desired not restored")
	}
	requireLighting(t, d.setUserRGBOff(true))
	data, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	var p DeviceProfile
	requireLighting(t, json.Unmarshal(data, &p))
	next := &Device{Serial: d.Serial, ChangeableLedChannels: 3, ZoneAmount: 3, DeviceProfile: &p}
	requireLighting(t, next.attachLightingSource(rt))
	if !next.userRGBOff || next.schedulerDark || !frameState(t, next).blank {
		t.Fatal("RGB-off startup replay")
	}
	// Unit-brightness random cache plus one final scale avoids Colorwarp's inherited
	// endpoint/global double scaling without changing the shared effect algorithm.
	v := frameState(t, d)
	v.blank = false
	v.effect = "colorwarp"
	v.brightness = 50
	v.profile = d.GetRgbProfile("colorwarp")
	owner := rendererOwner()
	owner.LastCycle[0].RGBStartColor = &rgb.Color{Red: 200}
	owner.LastCycle[0].RGBEndColor = &rgb.Color{Red: 200}
	start = time.Now()
	if got := renderLightingFrame(v, owner, &start); got[0] != 100 {
		t.Fatal("double scaling", got)
	}
}

func TestExternalChainsRawBrightnessSessionsAndProfiles(t *testing.T) {
	d, _ := lightingFixture(t, true)
	var clusterCallback func([]byte, int)
	var openCallback func([]byte, int)
	members := 0
	var openRegistrations int
	d.lightingClusterRegister = func(c *common.ClusterController) {
		members++
		clusterCallback = c.WriteColorEx
		if c.Serial != d.Serial || c.LedChannels != 3 {
			t.Fatal("cluster registration")
		}
	}
	d.lightingClusterRemove = func(string) { members = 0 }
	d.lightingOpenRGBRegister = func(c *common.OpenRGBController, replace bool) {
		openCallback = c.WriteColorEx
		openRegistrations++
		if len(c.Zones) != 3 || len(c.Colors) != 9 || c.Zones[2].Name != "Side" {
			t.Fatal("OpenRGB topology")
		}
	}
	frames := make(chan []byte, 40)
	d.lightingWrite = func(p []byte) { frames <- p }
	d.Connected = true
	requireLighting(t, d.SetLightingBrightness(10))
	// Wait for one-shot native output before testing external exact byte counts.
	waitFrame(t, frames)
	if d.ProcessSetRgbCluster(true) != 1 || members != 1 {
		t.Fatal("enable cluster")
	}
	oldCluster := clusterCallback
	raw := []byte{201, 2, 3, 4, 5, 6, 7, 8, 9}
	clusterCallback(raw, 0)
	if got := waitFrame(t, frames); !bytes.Equal(got, composeLightingPacket(raw)) {
		t.Fatal("Cluster scaled", got)
	}
	requireLighting(t, d.SetLightingEffect("mouse"))
	requireLighting(t, d.SetLightingBrightness(80))
	requireLighting(t, d.setSchedulerDark(true))
	if d.ProcessSetOpenRgbIntegration(true) != 2 {
		t.Fatal("owners not exclusive")
	}
	select {
	case p := <-frames:
		t.Fatal("Native competed with Cluster", p)
	default:
	}
	// Profile switch with Cluster retained replaces rather than duplicates callback.
	p := cloneLightingProfile(*d.DeviceProfile)
	p.Path = filepath.Join(t.TempDir(), "next.json")
	p.RGBCluster = true
	p.RgbOff = false
	d.UserProfiles["next"] = &p
	if d.ChangeDeviceProfile("next") != 1 || members != 1 {
		t.Fatal("Cluster profile refresh")
	}
	oldCluster(raw, 0)
	select {
	case p := <-frames:
		t.Fatal("stale Cluster callback", p)
	default:
	}
	clusterCallback(raw, 0)
	if !bytes.Equal(waitFrame(t, frames), composeLightingPacket(raw)) {
		t.Fatal("new cluster callback missing")
	}
	if !d.schedulerDark {
		t.Fatal("profile switch reset scheduler")
	}
	if d.ProcessSetRgbCluster(false) != 1 || members != 0 {
		t.Fatal("disable cluster")
	}
	waitFrame(t, frames)
	if d.ProcessSetOpenRgbIntegration(true) != 1 {
		t.Fatal("enable OpenRGB")
	}
	waitFrame(t, frames) // source black entry
	staleOpen := openCallback
	openCallback(raw, 0)
	queued := <-d.externalQueue
	raw[0] = 0
	d.applyLightingPacket(composeLightingPacket(queued.data), "openrgb", queued.generation, nil)
	if got := waitFrame(t, frames); got[1] != 201 {
		t.Fatal("OpenRGB copy/scaling", got)
	}
	if d.ProcessSetRgbCluster(true) != 2 {
		t.Fatal("OpenRGB exclusion")
	}
	if d.ProcessSetOpenRgbIntegration(false) != 1 {
		t.Fatal("disable OpenRGB")
	}
	waitFrame(t, frames)
	d.applyLightingPacket(composeLightingPacket(queued.data), "openrgb", queued.generation, nil)
	staleOpen(raw, 0)
	select {
	case p := <-frames:
		t.Fatal("stale OpenRGB frame", p)
	default:
	}
	select {
	case <-d.externalQueue:
		t.Fatal("stale callback queued")
	default:
	}
	if openRegistrations < 3 {
		t.Fatal("OpenRGB lifecycle not refreshed")
	}
}
func waitFrame(t *testing.T, ch <-chan []byte) []byte {
	t.Helper()
	select {
	case p := <-ch:
		return p
	case <-time.After(2 * time.Second):
		t.Fatal("output timeout")
		return nil
	}
}

func TestStaleNativeAndExternalWritesAfterBlockedApply(t *testing.T) {
	for _, source := range []string{"native", "cluster", "openrgb"} {
		t.Run(source, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			var writes atomic.Int32
			d.lightingWrite = func([]byte) { writes.Add(1) }
			d.Connected = true
			worker := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{}), generation: d.nativeGeneration}
			d.lightingWorker = worker
			d.clusterOwned = source == "cluster"
			d.openRGBOwned = source == "openrgb"
			generation := d.externalGeneration
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			d.lightingBeforeApply = func(string) { close(entered); <-release }
			go func() {
				defer close(done)
				d.applyLightingPacket(composeLightingPacket(make([]byte, 9)), source, generation, worker)
			}()
			<-entered
			d.lightingMu.Lock()
			d.externalGeneration++
			d.cancelLightingLocked()
			d.clusterOwned = false
			d.openRGBOwned = false
			d.lightingMu.Unlock()
			close(release)
			<-done
			if writes.Load() != 0 {
				t.Fatal("stale frame written")
			}
		})
	}
}
func TestCancellationDoesNotWaitForRunningHIDOrRetireNewOwner(t *testing.T) {
	d, _ := lightingFixture(t, true)
	entered, release := make(chan struct{}), make(chan struct{})
	d.Connected = true
	d.lightingWrite = func([]byte) { close(entered); <-release }
	d.setDeviceColor()
	<-entered
	d.lightingMu.Lock()
	old := d.lightingWorker
	d.cancelLightingLocked()
	newOwner := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{}), generation: d.nativeGeneration}
	d.lightingWorker = newOwner
	d.lightingMu.Unlock()
	d.retireLightingWorker(old)
	d.lightingMu.Lock()
	current := d.lightingWorker
	d.lightingMu.Unlock()
	if current != newOwner {
		t.Fatal("old worker retired new owner")
	}
	close(release)
	select {
	case <-old.done:
	case <-time.After(time.Second):
		t.Fatal("worker failed to retire")
	}
}
func TestTemporaryMissingStateRetriesWithPacing(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.lightingWrite = func([]byte) {}
	d.Connected = true
	d.lightingCanonical.resolver = &missingResolver{}
	d.setDeviceColor()
	d.lightingMu.Lock()
	worker := d.lightingWorker
	d.lightingMu.Unlock()
	time.Sleep(90 * time.Millisecond)
	d.lightingMu.Lock()
	calls := d.lightingCanonical.resolver.(*missingResolver).calls
	d.cancelLightingLocked()
	d.lightingMu.Unlock()
	if calls < 2 || calls > 4 {
		t.Fatal("unpaced retry", calls)
	}
	select {
	case <-worker.done:
	case <-time.After(time.Second):
		t.Fatal("retry cancellation")
	}
}

type missingResolver struct{ calls int }

func (m *missingResolver) Resolve(lightingsettings.Target, string) (lightingsettings.Resolution, error) {
	m.calls++
	return lightingsettings.Resolution{}, errors.New("temporarily missing")
}
func TestFailedAttachmentRetainsIndependentLegacyFallback(t *testing.T) {
	d, _ := lightingFixture(t, false)
	if d.attachLightingSource(nil) == nil || d.lightingAttached() {
		t.Fatal("partial attachment")
	}
	if _, ok := d.LightingSnapshot(); ok {
		t.Fatal("unusable snapshot")
	}
	if d.GetRgbProfile("static") == nil {
		t.Fatal("fallback depends on canonical")
	}
	d.Connected = false
	d.Exit = true
	d.setDeviceColor() // fallback safely suppressed on Exit by writeColor
}
func TestConcurrentCanonicalLifecycleAndOwnership(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.lightingProfileWrite = func(string, []byte) error { return nil }
	d.lightingWrite = func([]byte) {}
	d.Connected = true
	for _, name := range []string{"a", "b"} {
		p := cloneLightingProfile(*d.DeviceProfile)
		p.Path = filepath.Join(t.TempDir(), name)
		p.RgbOff = name == "b"
		d.UserProfiles[name] = &p
	}
	var wg sync.WaitGroup
	for action := 0; action < 7; action++ {
		wg.Add(1)
		go func(action int) {
			defer wg.Done()
			for i := 0; i < 12; i++ {
				switch action {
				case 0:
					_ = d.SetLightingEffect("mouse")
					_ = d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: float64(i)})
					_ = d.SetLightingBrightness(uint8(i))
				case 1:
					_ = d.setSchedulerDark(i%2 == 0)
					_ = d.setUserRGBOff(i%2 == 0)
				case 2:
					d.ProcessSetRgbCluster(i%2 == 0)
					d.applyCurrentCluster(make([]byte, 9))
				case 3:
					d.ProcessSetOpenRgbIntegration(i%2 == 0)
					d.enqueueCurrentOpenRGB(make([]byte, 9))
				case 4:
					d.ChangeDeviceProfile("a")
					d.ChangeDeviceProfile("b")
				case 6:
					d.SelectMouseDPIStage(1)
					d.SetMouseSniperMode(i%2 == 0)
					d.CallSniperMode(i%2 != 0)
					d.SaveMouseDPI(map[int]uint16{1: 800})
					d.DPISnapshot()
				case 5:
					d.ProcessNewGradientColor("gradient")
					d.ProcessDeleteGradientColor("gradient")
					d.LightingSnapshot()
				}
			}
		}(action)
	}
	d.startLightingQueue()
	wg.Wait()
	d.exitLighting()
}

func TestSourceProfileDefaultsAndNoInventedIndicators(t *testing.T) {
	logger.Init()
	root := t.TempDir()
	previous := pwd
	pwd = root
	defer func() { pwd = previous }()
	requireLighting(t, os.MkdirAll(filepath.Join(root, "database", "profiles"), 0700))
	d := &Device{Serial: "6964", Product: "GLAIVE RGB"}
	d.saveDeviceProfile()
	p := d.DeviceProfile
	if p == nil || p.RGBProfile != "mouse" || *p.BrightnessSlider != 100 || p.OriginalBrightness != 100 || p.PollingRate != 1 || p.LiftHeight != 3 || p.ButtonOptimization != 2 || p.AngleSnapping != 0 || p.Profile != 1 || p.SleepMode != 15 || p.DPIColor != nil || p.SniperColor != nil || len(p.Profiles) != 6 {
		t.Fatalf("source defaults %#v", p)
	}
	values := []uint16{400, 800, 1500, 3000, 6000, 9000}
	for id, want := range values {
		stage := p.Profiles[id]
		if stage.Value != want || stage.Sniper != (id == 0) || stage.Color != nil || stage.ColorIndex != nil {
			t.Fatal("invented/default input state", id, stage)
		}
	}
	if p.ZoneColors[2].LEDIndex != 6 || p.ZoneColors[0].Color.Red != 255 {
		t.Fatal("default topology")
	}
}
func TestDPISniperPacketsAndCanonicalIsolation(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.Connected = true
	var commands, payloads [][]byte
	d.lightingDPIWrite = func(command, data []byte) error {
		commands = append(commands, append([]byte(nil), command...))
		payloads = append(payloads, append([]byte(nil), data...))
		return nil
	}
	desired, _ := d.LightingSnapshot()
	d.updateMouseDPI()
	if len(payloads) != 2 {
		t.Fatal(payloads)
	}
	for i, data := range payloads {
		if !bytes.Equal(commands[i], []byte{0x13}) || len(data) != 10 || data[7] != 0 || data[8] != 0 || data[9] != 0 {
			t.Fatal("DPI RGB/protocol changed", data)
		}
		if data[0] != 208 && data[0] != 209 {
			t.Fatal(data)
		}
	}
	commands, payloads = nil, nil
	if d.SetMouseSniperMode(true) != 1 || len(payloads) != 1 || !bytes.Equal(payloads[0], []byte{0}) || !bytes.Equal(commands[0], []byte{0x13, 0x02, 0x00}) {
		t.Fatal("sniper selection", commands, payloads)
	}
	// Selection while Sniper is active persists the desired regular stage without
	// replacing its active hardware selection.
	if d.SelectMouseDPIStage(1) != 1 || len(payloads) != 1 {
		t.Fatal("regular selection displaced Sniper")
	}
	if d.SetMouseSniperMode(false) != 1 || !bytes.Equal(payloads[1], []byte{1}) {
		t.Fatal("Sniper release")
	}
	after, _ := d.LightingSnapshot()
	if !reflect.DeepEqual(desired, after) {
		t.Fatal("input altered canonical Lighting")
	}
	// Nil input color backing stays unsupported, rather than inventing a palette
	// or overlay from the Pro's defaults.
	if d.SaveMouseZoneColorsSniper(rgb.Color{}, map[int]rgb.Color{0: {}}, rgb.Color{}) != 0 {
		t.Fatal("nil indicator backing accepted")
	}
}
func TestProfileRGBOffReplayAndSwitchFailure(t *testing.T) {
	d, _ := lightingFixture(t, true)
	requireLighting(t, d.SetLightingBrightness(77))
	requireLighting(t, d.setSchedulerDark(true))
	p := cloneLightingProfile(*d.DeviceProfile)
	p.Path = filepath.Join(t.TempDir(), "off.json")
	p.RgbOff = true
	p.OpenRGBIntegration = true
	d.UserProfiles["off"] = &p
	before := inputJSON(t, d)
	writes := 0
	d.lightingProfileWrite = func(string, []byte) error {
		writes++
		if writes == 2 {
			return errors.New("second file failure")
		}
		return nil
	}
	if d.ChangeDeviceProfile("off") != 0 || !bytes.Equal(before, inputJSON(t, d)) || d.openRGBOwned || d.userRGBOff {
		t.Fatal("partial profile switch")
	}
	d.lightingProfileWrite = nil
	if d.ChangeDeviceProfile("off") != 1 || !d.userRGBOff || !d.openRGBOwned || !d.schedulerDark {
		t.Fatal("preference replay")
	}
	s, _ := d.LightingSnapshot()
	if s.Brightness != 77 {
		t.Fatal("profile replaced canonical state")
	}
	if d.ChangeDeviceProfile("default") != 1 || d.userRGBOff || d.openRGBOwned || !d.schedulerDark {
		t.Fatal("profile release replay")
	}
}
func TestActualInFlightExternalWriteDoesNotContinueOldSession(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.Connected = true
	d.clusterOwned = true
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var writes atomic.Int32
	d.lightingWrite = func([]byte) {
		if writes.Add(1) == 1 {
			close(entered)
			<-release
		}
	}
	old := d.externalGeneration
	packet := composeLightingPacket(make([]byte, 9))
	go func() {
		defer close(done)
		d.applyLightingPacket(packet, "cluster", old, nil)
		d.applyLightingPacket(packet, "cluster", old, nil)
	}()
	<-entered
	// Transition does not take deviceLock, so it completes while old HID is held.
	d.lightingMu.Lock()
	d.clusterOwned = false
	d.externalGeneration++
	d.cancelLightingLocked()
	d.lightingMu.Unlock()
	close(release)
	<-done
	if writes.Load() != 1 {
		t.Fatal("old owner wrote again after HID returned")
	}
}

func TestSaveNamedProfileDoesNotReplaceCurrentOwnership(t *testing.T) {
	d, _ := lightingFixture(t, true)
	root := filepath.Dir(d.DeviceProfile.Path)
	previous := pwd
	pwd = root
	defer func() { pwd = previous }()
	requireLighting(t, os.MkdirAll(filepath.Join(root, "database", "profiles"), 0700))
	requireLighting(t, os.MkdirAll(filepath.Join(root, "database", "key-assignments"), 0700))
	before := inputJSON(t, d)
	generation := d.externalGeneration
	if d.SaveUserProfile("copy") != 1 || !bytes.Equal(before, inputJSON(t, d)) || d.externalGeneration != generation || d.UserProfiles["copy"].Active {
		t.Fatal("named save changed active profile")
	}
	d.UserProfiles["copy"].ZoneColors[0].Color.Red = 100
	if d.DeviceProfile.ZoneColors[0].Color.Red != 9 {
		t.Fatal("saved profile aliases current input")
	}
}
