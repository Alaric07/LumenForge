package m65rgbultra

import (
	"LumenForge/src/common"
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
	rt, err := lightingsettings.LoadIndependentDeviceRuntime(filepath.Join(root, "state.json"), filepath.Join(root, "effects.json"), "../../../database/rgb.json")
	requireLighting(t, err)
	brightness := uint8(17)
	stages := map[int]DPIProfile{}
	colors := []rgb.Color{{Red: 255}, {Red: 255, Green: 255, Blue: 255}, {Green: 255}, {Red: 128, Blue: 128}, {Green: 191, Blue: 255}, {Red: 255, Green: 255}}
	for id, v := range []uint16{400, 800, 1200, 1600, 3000, 200} {
		c := colors[id]
		name := []string{"Stage 1", "Stage 2", "Stage 3", "Stage 4", "Stage 5", "Sniper"}[id]
		stages[id] = DPIProfile{Name: name, Value: v, PackerIndex: []int{1, 2, 3, 4, 5, 6}[id], Color: &c, Sniper: id == 5, ColorIndex: map[int][]int{0: {2, 5, 8}}}
	}
	p := &DeviceProfile{Serial: "m65-ultra-test", Product: "M65 RGB ULTRA", Path: filepath.Join(root, "profile.json"), Active: true, RGBProfile: "gradient", BrightnessSlider: &brightness, OriginalBrightness: 82, Profile: 1, PollingRate: 4, LiftHeight: 2, Tilts: map[int]TiltOption{0: {Name: "Left Tilt", Value: 20}, 1: {Name: "Right Tilt", Value: 20}, 2: {Name: "Forward Tilt", Value: 20}, 3: {Name: "Backward Tilt", Value: 20}}, SleepMode: 15, Profiles: stages, ZoneColors: map[int]ZoneColors{0: {Name: "obsolete", Color: &rgb.Color{Red: 9}, ColorIndex: []int{99}}}}
	d := &Device{Serial: p.Serial, Product: p.Product, VendorId: 6940, ProductId: 7070, Usb: true, Connected: true, LEDChannels: 3, ChangeableLedChannels: 2, ZoneAmount: 2, DPIAmount: 6, MinDPI: 100, MaxDPI: 26000, DeviceProfile: p, UserProfiles: map[string]*DeviceProfile{"default": p}, Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{"static": {StartColor: rgb.Color{Red: 9}}}}, lightingHardwareReady: false, LiftHeights: map[int]string{2: "Low", 3: "Medium", 4: "High", 6: "Calibrated", 7: "Ultra Low"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz", 2: "250 Hz", 3: "500 Hz", 4: "1000 Hz", 5: "2000 Hz", 6: "4000 Hz", 7: "8000 Hz"}, SwitchModes: map[int]string{0: "Disabled", 1: "Enabled"}}
	d.lightingClusterRegister = func(*common.ClusterController) {}
	d.lightingClusterRemove = func(string) {}
	d.lightingOpenRGBReconcile = func(string, *common.OpenRGBController) {}
	d.lightingDPIWrite = func([]byte, []byte) error { return nil }
	d.lightingWrite = func([]byte) {}
	data, _ := json.Marshal(p)
	requireLighting(t, os.WriteFile(p.Path, data, 0600))
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
	v, e := d.lightingFrameLocked()
	requireLighting(t, e)
	return v
}
func rendererOwner() *rgb.ActiveRGB {
	o := rgb.Exit()
	o.RGBStartColor = &rgb.Color{Red: 100, Green: 80, Blue: 60}
	o.RGBEndColor = &rgb.Color{Red: 40, Green: 20, Blue: 10}
	return o
}
func render(t *testing.T, d *Device) []byte {
	t.Helper()
	start := time.Now()
	return renderLightingFrame(frameState(t, d), rendererOwner(), &start)
}
func waitDone(t *testing.T, c <-chan struct{}) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out")
	}
}
func waitFrame(t *testing.T, c <-chan []byte) []byte {
	t.Helper()
	select {
	case p := <-c:
		return p
	case <-time.After(3 * time.Second):
		t.Fatal("no frame")
		return nil
	}
}
func noFrame(t *testing.T, c <-chan []byte) {
	t.Helper()
	select {
	case p := <-c:
		t.Fatalf("unexpected output %v", p)
	case <-time.After(60 * time.Millisecond):
	}
}
func inputJSON(t *testing.T, d *Device) []byte {
	t.Helper()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	b, e := json.Marshal(d.DeviceProfile)
	requireLighting(t, e)
	return b
}

func TestCatalogueTopologyDefaultsEveryDispatch(t *testing.T) {
	d, rt := lightingFixture(t, true)
	if !reflect.DeepEqual(rgbModes, auditedEffects) {
		t.Fatal(rgbModes)
	}
	s, ok := d.LightingSnapshot()
	if !ok || s.ConfiguredEffect != "static" || s.Brightness != 100 || len(s.SupportedEffects) != 20 {
		t.Fatal(s)
	}
	for i, e := range s.SupportedEffects {
		if e.ID != auditedEffects[i] || e.Label == "" {
			t.Fatal(e)
		}
	}
	if _, found, e := rt.State.Resolve(d.Serial); e != nil || found {
		t.Fatal("attachment imported legacy selection")
	}
	if _, e := os.Stat(filepath.Join(filepath.Dir(d.DeviceProfile.Path), "effects.json")); !os.IsNotExist(e) {
		t.Fatal("attachment persisted defaults", e)
	}
	zones := lightingTopology()
	want := [2]ZoneColors{{Name: "Scroll", ColorIndex: []int{1, 4, 7}}, {Name: "Logo", ColorIndex: []int{0, 3, 6}}}
	if !reflect.DeepEqual(zones, want) {
		t.Fatal(zones)
	}
	zones[0].ColorIndex[0] = 99
	if lightingTopology()[0].ColorIndex[0] != 1 {
		t.Fatal("topology aliases")
	}
	if got := composeLightingPacket([]byte{1, 2, 3, 4, 5, 6}); !bytes.Equal(got, []byte{4, 1, 0, 5, 2, 0, 6, 3, 0}) {
		t.Fatal(got)
	}
	for _, effect := range auditedEffects {
		requireLighting(t, d.SetLightingEffect(effect))
		if got := render(t, d); len(got) != 6 {
			t.Fatalf("%s has no dispatch: %v", effect, got)
		}
	}
	requireLighting(t, d.SetLightingEffect("mouse"))
	s, _ = d.LightingSnapshot()
	z := s.AuthoredZoneEditor.Zones
	if len(z) != 2 || z[0].ID != "0" || z[0].Label != "Scroll" || z[0].ColorHex != "#00ffff" || z[1].Label != "Logo" || z[1].ColorHex != "#ffff00" {
		t.Fatal(z)
	}
}
func TestCanonicalSoleAuthorityCopiesAndNoLegacyWrites(t *testing.T) {
	d, rt := lightingFixture(t, true)
	before := inputJSON(t, d)
	legacy, _ := json.Marshal(d.Rgb)
	requireLighting(t, d.SetLightingEffect("mouse"))
	requireLighting(t, d.SetLightingBrightness(50))
	requireLighting(t, d.SetLightingZoneColor("mouse", "zone", "0", "", rgb.Color{Red: 200, Green: 100, Blue: 40}))
	if got := render(t, d); !bytes.Equal(got, []byte{100, 50, 20, 127, 127, 0}) {
		t.Fatal(got)
	}
	if d.SaveMouseZoneColors(rgb.Color{}, map[int]rgb.Color{1: {Blue: 200}}) != 1 {
		t.Fatal("legacy endpoint")
	}
	if d.ChangeDeviceBrightness(3) != 0 {
		t.Fatal("legacy brightness gained authority")
	}
	d.loadRgb()
	d.saveRgbProfile()
	d.saveDeviceProfile()
	if !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("canonical writes changed profile Lighting/input")
	}
	after, _ := json.Marshal(d.Rgb)
	if !bytes.Equal(legacy, after) {
		t.Fatal("legacy RGB written")
	}
	disk, e := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, e)
	if !bytes.Equal(before, disk) {
		t.Fatal("dual profile write")
	}
	settings, e := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, e)
	requireLighting(t, d.SetLightingEffectSettings("gradient", settings))
	settings.Gradient.Stops[0].Color.Red = 17
	actual, e := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, e)
	if actual.Gradient.Stops[0].Color.Red == 17 {
		t.Fatal("settings alias")
	}
	p := d.GetRgbProfile("gradient")
	p.Gradients[0] = rgb.Color{Red: 19}
	p2 := d.GetRgbProfile("gradient")
	if p2.Gradients[0].Red == 19 {
		t.Fatal("profile view aliases")
	}
	clone := cloneLightingProfile(*d.DeviceProfile)
	clone.Profiles[0].Color.Red = 3
	clone.Profiles[0].ColorIndex[0][0] = 99
	clone.ZoneColors[0].ColorIndex[0] = 33
	if d.DeviceProfile.Profiles[0].Color.Red != 255 || d.DeviceProfile.Profiles[0].ColorIndex[0][0] != 2 || d.DeviceProfile.ZoneColors[0].ColorIndex[0] != 99 {
		t.Fatal("input clone aliases")
	}
	reload, e := lightingsettings.LoadIndependentDeviceStateStore(rt.State.(*lightingsettings.IndependentDeviceStateStore).Path())
	requireLighting(t, e)
	state, _, e := reload.Resolve(d.Serial)
	requireLighting(t, e)
	if state.SelectedEffect != "mouse" || state.Brightness != 50 {
		t.Fatal(state)
	}
	// Even direct mutation of inert serialized fields cannot regain authority.
	d.DeviceProfile.RGBProfile = "off"
	*d.DeviceProfile.BrightnessSlider = 0
	d.DeviceProfile.ZoneColors[0].Color.Red = 0
	s, ok := d.LightingSnapshot()
	if !ok || s.ConfiguredEffect != "mouse" || s.Brightness != 50 {
		t.Fatal(s)
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
	return false, errors.New("delete failure")
}
func (f failingEffects) SetAuthoredZones(string, string, map[string]lightingsettings.Color) error {
	return errors.New("palette failure")
}
func TestPersistenceFailureKeepsStateWorkerInputAndOutput(t *testing.T) {
	for _, kind := range []string{"state", "effects", "profile"} {
		t.Run(kind, func(t *testing.T) {
			d, rt := lightingFixture(t, true)
			var writes atomic.Int32
			d.lightingWrite = func([]byte) { writes.Add(1) }
			old := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{})}
			d.lightingWorker = old
			before := inputJSON(t, d)
			snapshot, _ := d.LightingSnapshot()
			generation := d.externalGeneration
			switch kind {
			case "state":
				d.lightingCanonical.state = failingState{rt.State}
				if d.SetLightingEffect("rainbow") == nil || d.SetLightingBrightness(25) == nil {
					t.Fatal("success")
				}
			case "effects":
				d.lightingCanonical.effects = failingEffects{rt.Effects}
				v, e := d.ResolveLightingEffectSettings("static")
				requireLighting(t, e)
				if d.SetLightingEffectSettings("static", v) == nil || d.ResetLightingEffectSettings("static") == nil || d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 4}) == nil {
					t.Fatal("success")
				}
			case "profile":
				d.lightingProfileWrite = func(string, []byte) error { return errors.New("profile failure") }
				if d.setUserRGBOff(true) == nil || d.ProcessSetRgbCluster(true) != 0 || d.ProcessSetOpenRgbIntegration(true) != 0 || d.SaveMouseDPISettings(map[int]uint16{}, map[int]rgb.Color{}) != 0 {
					t.Fatal("success")
				}
				if d.SaveMouseDPI(map[int]uint16{0: 1234}) != 0 {
					t.Fatal("DPI success")
				}
			}
			after, _ := d.LightingSnapshot()
			if !reflect.DeepEqual(snapshot, after) || !bytes.Equal(before, inputJSON(t, d)) || d.lightingWorker != old || d.externalGeneration != generation || writes.Load() != 0 {
				t.Fatal("failed persistence published")
			}
			select {
			case <-old.stop:
				t.Fatal("worker cancelled")
			default:
			}
		})
	}
}
func TestBrightnessSchedulerRGBOffAndExternalIndicator(t *testing.T) {
	d, _ := lightingFixture(t, true)
	before := inputJSON(t, d)
	requireLighting(t, d.SetLightingEffect("mouse"))
	requireLighting(t, d.SetLightingBrightness(50))
	if got := render(t, d); !bytes.Equal(got, []byte{0, 127, 127, 127, 127, 0}) {
		t.Fatal(got)
	}
	if d.ChangeDeviceBrightnessValue(101) != 0 {
		t.Fatal("range")
	}
	requireLighting(t, d.setSchedulerDark(true))
	requireLighting(t, d.setSchedulerDark(true))
	requireLighting(t, d.SetLightingBrightness(70))
	if !bytes.Equal(render(t, d), make([]byte, 6)) {
		t.Fatal("not dark")
	}
	if !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("scheduler persisted")
	}
	requireLighting(t, d.setSchedulerDark(false))
	if frameState(t, d).brightness != 70 {
		t.Fatal("stale restore")
	}
	frames := make(chan []byte, 30)
	d.lightingWrite = func(p []byte) { frames <- p }
	readyLighting(d)
	requireLighting(t, d.setUserRGBOff(true))
	p := waitFrame(t, frames)
	if !bytes.Equal(p, []byte{0, 0, 178, 0, 0, 178, 0, 0, 178}) {
		t.Fatal(p)
	}
	b, e := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, e)
	var profile DeviceProfile
	requireLighting(t, json.Unmarshal(b, &profile))
	if !profile.RgbOff {
		t.Fatal("not persisted")
	}
	if d.ProcessSetRgbCluster(true) != 1 {
		t.Fatal("cluster")
	}
	waitFrame(t, frames)
	requireLighting(t, d.setSchedulerDark(true))
	d.writeColorCluster([]byte{1, 2, 3, 4, 5, 6}, 0)
	p = waitFrame(t, frames)
	if !bytes.Equal(p, []byte{4, 1, 0, 5, 2, 0, 6, 3, 0}) {
		t.Fatal("external scaled", p)
	}
	requireLighting(t, d.setSchedulerDark(false))
	d.writeColorCluster([]byte{9, 8, 7, 6, 5, 4}, 0)
	p = waitFrame(t, frames)
	if p[5] != 178 || p[1] != 9 {
		t.Fatal(p)
	}
	if d.ProcessSetRgbCluster(false) != 1 {
		t.Fatal("release")
	}
	waitFrame(t, frames)
	requireLighting(t, d.setUserRGBOff(false))
	p = waitFrame(t, frames)
	if p[4] != 178 || p[0] != 178 {
		t.Fatal(p)
	}
}
func TestColorwarpBrightnessScalesOnce(t *testing.T) {
	d, _ := lightingFixture(t, true)
	requireLighting(t, d.SetLightingEffect("colorwarp"))
	a := frameState(t, d)
	a.brightness = 100
	b := a
	b.brightness = 50
	o1, o2 := rendererOwner(), rendererOwner()
	o1.LastCycle[0].RGBStartColor = &rgb.Color{Red: 200, Green: 100, Blue: 60}
	o1.LastCycle[0].RGBEndColor = &rgb.Color{Red: 200, Green: 100, Blue: 60}
	o2.LastCycle[0].RGBStartColor = &rgb.Color{Red: 200, Green: 100, Blue: 60}
	o2.LastCycle[0].RGBEndColor = &rgb.Color{Red: 200, Green: 100, Blue: 60}
	start := time.Now()
	full := renderLightingFrame(a, o1, &start)
	half := renderLightingFrame(b, o2, &start)
	for i, v := range full {
		if half[i] != v/2 {
			t.Fatal(full, half)
		}
	}
}

func TestExternalRegistrationsSessionsAndRawBytes(t *testing.T) {
	for _, source := range []string{"cluster", "openrgb"} {
		t.Run(source, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			frames := make(chan []byte, 30)
			d.lightingWrite = func(p []byte) { frames <- p }
			readyLighting(d)
			var cluster *common.ClusterController
			var open *common.OpenRGBController
			adds, removes := 0, 0
			d.lightingClusterRegister = func(c *common.ClusterController) { cluster = c; adds++ }
			d.lightingClusterRemove = func(string) { removes++ }
			d.lightingOpenRGBReconcile = func(_ string, c *common.OpenRGBController) { open = c }
			d.setupOpenRGBController()
			if open == nil || len(open.Colors) != 6 || len(open.Zones) != 2 || open.Zones[0].Name != "Scroll" || open.Zones[1].Name != "Logo" {
				t.Fatal(open)
			}
			d.startQueueWorker()
			enable := d.ProcessSetRgbCluster
			opposite := d.ProcessSetOpenRgbIntegration
			if source == "openrgb" {
				enable, opposite = opposite, enable
			}
			if enable(true) != 1 {
				t.Fatal("enable")
			}
			if source == "openrgb" {
				waitFrame(t, frames)
			}
			if opposite(true) != 2 {
				t.Fatal("exclusive owners")
			}
			oldGeneration := d.externalGeneration
			var callback func([]byte, int)
			if source == "cluster" {
				if cluster == nil || cluster.LedChannels != 2 {
					t.Fatal(cluster)
				}
				callback = cluster.WriteColorEx
			} else {
				callback = open.WriteColorEx
			}
			d.setupClusterController()
			d.setupOpenRGBController()
			if source == "cluster" && adds != 1 {
				t.Fatal("duplicate member", adds, removes)
			}
			var count atomic.Int32
			d.lightingProfileWrite = func(path string, b []byte) error { count.Add(1); return os.WriteFile(path, b, 0600) }
			requireLighting(t, d.SetLightingEffect("rainbow"))
			requireLighting(t, d.SetLightingBrightness(1))
			requireLighting(t, d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 10}))
			noFrame(t, frames)
			if count.Load() != 0 {
				t.Fatal("desired touched profile")
			}
			data := []byte{11, 22, 33, 44, 55, 66}
			callback(data, 0)
			p := waitFrame(t, frames)
			if !bytes.Equal(p, []byte{44, 11, 2, 55, 22, 2, 66, 33, 2}) {
				t.Fatal(p)
			}
			if enable(false) != 1 {
				t.Fatal("disable")
			}
			waitFrame(t, frames)
			d.stopLighting()
			callback(data, 0)
			d.applyLightingPacket(composeLightingPacket(data), source, oldGeneration, nil)
			noFrame(t, frames)
			if enable(true) != 1 {
				t.Fatal("re-enable")
			}
			if source == "openrgb" {
				waitFrame(t, frames)
			}
			callback(data, 0)
			noFrame(t, frames)
			d.stopCanonicalLighting(false)
			if open != nil {
				t.Fatal("stale OpenRGB registration")
			}
		})
	}
}
func TestProfileSwitchPreservesCanonicalAndRefreshesSessions(t *testing.T) {
	d, _ := lightingFixture(t, true)
	frames := make(chan []byte, 20)
	d.lightingWrite = func(p []byte) { frames <- p }
	readyLighting(d)
	requireLighting(t, d.SetLightingEffect("mouse"))
	waitFrame(t, frames)
	next := cloneLightingProfile(*d.DeviceProfile)
	next.Path = filepath.Join(filepath.Dir(next.Path), "next.json")
	next.RGBProfile = "off"
	next.RgbOff = true
	next.RGBCluster = true
	d.UserProfiles["next"] = &next
	var c *common.ClusterController
	d.lightingClusterRegister = func(v *common.ClusterController) { c = v }
	requireLighting(t, d.setSchedulerDark(true))
	waitFrame(t, frames)
	generation := d.externalGeneration
	if d.ChangeDeviceProfile("next") != 1 {
		t.Fatal("switch")
	}
	p := waitFrame(t, frames)
	s, ok := d.LightingSnapshot()
	if !ok || s.ConfiguredEffect != "mouse" || s.Brightness != 100 || !s.ClusterControlled || d.externalGeneration == generation || !d.userRGBOff || !d.schedulerDark || c == nil || p[5] != 0 {
		t.Fatal(s, p)
	}
	if d.DeviceProfile.Profiles[5].Value != 200 || d.DeviceProfile.Profiles[5].Color.Green != 255 {
		t.Fatal("Sniper changed")
	}
	before := inputJSON(t, d)
	d.lightingProfileWrite = func(string, []byte) error { return errors.New("save") }
	if d.ChangeDeviceProfile("default") != 0 || !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("failed switch published")
	}
}

func TestPrivateWorkerBoundedCancellationAndStaleRetirement(t *testing.T) {
	d, _ := lightingFixture(t, true)
	legacy := rgb.Exit()
	d.activeRgb = legacy
	requireLighting(t, d.SetLightingEffect("rainbow"))
	started, release := make(chan struct{}), make(chan struct{})
	frames := make(chan []byte, 10)
	var first atomic.Bool
	d.lightingWrite = func(p []byte) {
		if first.CompareAndSwap(false, true) {
			close(started)
			<-release
		}
		frames <- p
	}
	readyLighting(d)
	d.setDeviceColor()
	waitDone(t, started)
	d.lightingMu.Lock()
	old := d.lightingWorker
	d.lightingMu.Unlock()
	done := make(chan struct{})
	go func() { _ = d.SetLightingEffect("static"); close(done) }()
	waitDone(t, done)
	d.lightingMu.Lock()
	current := d.lightingWorker
	d.lightingMu.Unlock()
	if current == nil || current == old || d.activeRgb != legacy {
		t.Fatal("mixed owners")
	}
	d.retireLightingWorker(old)
	d.lightingMu.Lock()
	same := d.lightingWorker == current
	d.lightingMu.Unlock()
	if !same {
		t.Fatal("stale retirement")
	}
	close(release)
	waitFrame(t, frames)
	waitFrame(t, frames)
	waitDone(t, old.done)
	waitDone(t, current.done)
}
func TestStaleFramesRejectedAfterSerialization(t *testing.T) {
	for _, source := range []string{"native", "cluster", "openrgb"} {
		for _, transition := range []string{"owner", "profile", "stop"} {
			t.Run(source+"/"+transition, func(t *testing.T) {
				d, _ := lightingFixture(t, true)
				frames := make(chan []byte, 30)
				d.lightingWrite = func(p []byte) { frames <- p }
				readyLighting(d)
				if source == "cluster" {
					d.ProcessSetRgbCluster(true)
				}
				if source == "openrgb" {
					d.ProcessSetOpenRgbIntegration(true)
					waitFrame(t, frames)
				}
				// Hold output serialization, let the old frame finish rendering, then change
				// ownership. The actual apply gate must reject it after taking deviceLock.
				w := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{}), generation: d.nativeGeneration}
				d.lightingMu.Lock()
				d.lightingWorker = w
				d.lightingMu.Unlock()
				generation := d.externalGeneration
				blocked := make(chan struct{})
				var once sync.Once
				d.lightingBeforeApply = func(s string) {
					if s == source {
						once.Do(func() { close(blocked) })
					}
				}
				d.deviceLock.Lock()
				done := make(chan struct{})
				go func() {
					d.applyLightingPacket(composeLightingPacket([]byte{77, 88, 99, 66, 55, 44}), source, generation, w)
					close(done)
				}()
				waitDone(t, blocked)
				switched := make(chan struct{})
				go func() {
					switch transition {
					case "owner":
						if source == "native" {
							d.ProcessSetRgbCluster(true)
						} else if source == "cluster" {
							d.ProcessSetRgbCluster(false)
						} else {
							d.ProcessSetOpenRgbIntegration(false)
						}
					case "profile":
						p := cloneLightingProfile(*d.DeviceProfile)
						p.Path = filepath.Join(filepath.Dir(p.Path), "other.json")
						d.lightingMu.Lock()
						d.UserProfiles["other"] = &p
						d.lightingMu.Unlock()
						d.ChangeDeviceProfile("other")
					case "stop":
						d.stopCanonicalLighting(false)
					}
					close(switched)
				}()
				// Profile selection may wait for its own DPI write, but invalidation precedes it.
				deadline := time.Now().Add(3 * time.Second)
				for {
					d.lightingMu.Lock()
					invalid := d.externalGeneration != generation || d.nativeGeneration != w.generation
					d.lightingMu.Unlock()
					if invalid {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("transition blocked on output")
					}
					time.Sleep(time.Millisecond)
				}
				d.deviceLock.Unlock()
				waitDone(t, done)
				waitDone(t, switched)
				for {
					select {
					case p := <-frames:
						if p[1] == 77 {
							t.Fatal("stale frame wrote", p)
						}
					default:
						return
					}
				}
			})
		}
	}
}

func TestMissingIndicatorReleasesLockForEveryOwner(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.ProcessSetOpenRgbIntegration(true)
	d.stopLighting()
	d.lightingMu.Lock()
	d.DeviceProfile.Profiles[1] = DPIProfile{Color: nil}
	generation := d.externalGeneration
	d.lightingMu.Unlock()
	done := make(chan struct{})
	go func() {
		d.applyLightingPacket(composeLightingPacket([]byte{1, 2, 3, 4, 5, 6}), "openrgb", generation, nil)
		close(done)
	}()
	waitDone(t, done)
	lock := make(chan struct{})
	go func() { d.deviceLock.Lock(); d.deviceLock.Unlock(); close(lock) }()
	waitDone(t, lock)
	d.lightingMu.Lock()
	d.DeviceProfile.Profiles[1] = DPIProfile{Color: &rgb.Color{Red: 255}}
	d.lightingMu.Unlock()
	frames := make(chan []byte, 20)
	d.lightingWrite = func(p []byte) { frames <- p }
	readyLighting(d)
	d.startQueueWorker()
	d.writeColorEx([]byte{1, 2, 3, 4, 5, 6}, 0)
	waitFrame(t, frames)
	if d.ProcessSetOpenRgbIntegration(false) != 1 {
		t.Fatal("transition")
	}
	waitFrame(t, frames)
	if d.ProcessSetRgbCluster(true) != 1 {
		t.Fatal("cluster")
	}
	d.writeColorCluster([]byte{1, 2, 3, 4, 5, 6}, 0)
	waitFrame(t, frames)
	if d.ProcessSetRgbCluster(false) != 1 {
		t.Fatal("native")
	}
	waitFrame(t, frames)
	p := cloneLightingProfile(*d.DeviceProfile)
	p.Path = filepath.Join(filepath.Dir(p.Path), "next.json")
	d.UserProfiles["next"] = &p
	switched := make(chan struct{})
	go func() { d.ChangeDeviceProfile("next"); d.StopDirty(); close(switched) }()
	waitDone(t, switched)
}

func TestConcurrentLightingInputsAndOwnership(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	d.startQueueWorker()
	next := cloneLightingProfile(*d.DeviceProfile)
	next.Path = filepath.Join(filepath.Dir(next.Path), "next.json")
	d.UserProfiles["next"] = &next
	var wg sync.WaitGroup
	actions := []func(){func() { _ = d.SetLightingEffect("rainbow"); _ = d.SetLightingBrightness(40) }, func() {
		_ = d.SetLightingZoneColors("mouse", []string{"0", "1"}, rgb.Color{Red: 30})
		d.ProcessNewGradientColor("gradient")
		d.ProcessDeleteGradientColor("gradient")
	}, func() {
		d.ProcessSetRgbCluster(true)
		d.writeColorCluster([]byte{1, 2, 3, 4, 5, 6}, 0)
		d.ProcessSetRgbCluster(false)
	}, func() {
		d.ProcessSetOpenRgbIntegration(true)
		d.writeColorEx([]byte{6, 5, 4, 3, 2, 1}, 0)
		d.ProcessSetOpenRgbIntegration(false)
	}, func() { d.ChangeDeviceProfile("next"); d.ChangeDeviceProfile("default") }, func() {
		d.SchedulerBrightness(0)
		d.ControlDeviceRgb(true)
		d.ControlDeviceRgb(false)
		d.SchedulerBrightness(1)
	}, func() {
		d.SelectMouseDPIStage(2)
		d.SetMouseSniperMode(true)
		d.SetMouseSniperMode(false)
		d.SaveMouseDpiColors(rgb.Color{}, map[int]rgb.Color{5: {Red: 255, Green: 255}})
	}, func() { d.LightingSnapshot(); d.DPISnapshot(); d.PerformanceSnapshot(); d.DeviceProfileSnapshot() }}
	for _, action := range actions {
		wg.Add(1)
		go func(f func()) {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				f()
			}
		}(action)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	waitDone(t, done)
	d.StopDirty()
}

func readyLighting(d *Device) {
	d.lightingMu.Lock()
	d.lightingHardwareReady = true
	d.lightingMu.Unlock()
}

func TestFailedAttachmentKeepsIndependentLegacyFallback(t *testing.T) {
	d, rt := lightingFixture(t, false)
	d.ChangeableLedChannels = 1
	if d.attachLightingSource(rt) == nil || d.lightingCanonical != nil || d.lightingWorker != nil {
		t.Fatal("partial attachment")
	}
	d.ChangeableLedChannels = 2
	d.DeviceProfile.RGBProfile = "static"
	d.DeviceProfile.ZoneColors = map[int]ZoneColors{}
	for i, z := range lightingTopology() {
		z.Color = &rgb.Color{}
		d.DeviceProfile.ZoneColors[i] = z
	}
	reports := make(chan []byte, 10)
	d.lightingReportWrite = func(b []byte) (int, error) { reports <- append([]byte(nil), b...); return len(b), nil }
	// No canonical runtime: fallback still reads its own RGB/profile and transport.
	d.setDeviceColor()
	p := waitFrame(t, reports)
	if len(p) != 65 || p[1] != 8 {
		t.Fatal(p)
	}
	if _, ok := d.LightingSnapshot(); ok {
		t.Fatal("unusable runtime exposed canonical UI")
	}
	d.DeviceProfile.OpenRGBIntegration = true
	d.applyLegacyOpenRGBFrame([]byte{1, 2, 3, 4, 5, 6})
	waitFrame(t, reports)
}

type temporaryResolver struct {
	calls    atomic.Int32
	fallback lightingResolver
	missing  atomic.Bool
}

func (r *temporaryResolver) Resolve(target lightingsettings.Target, effect string) (lightingsettings.Resolution, error) {
	r.calls.Add(1)
	if r.missing.Load() {
		return lightingsettings.Resolution{}, errors.New("temporarily missing")
	}
	return r.fallback.Resolve(target, effect)
}
func TestTemporaryMissingStateRetriesWithPacing(t *testing.T) {
	d, rt := lightingFixture(t, true)
	requireLighting(t, d.SetLightingEffect("rainbow"))
	resolver := &temporaryResolver{fallback: rt.Resolver}
	resolver.missing.Store(true)
	d.lightingCanonical.resolver = resolver
	frames := make(chan []byte, 10)
	d.lightingWrite = func(p []byte) { frames <- p }
	readyLighting(d)
	d.setDeviceColor()
	time.Sleep(110 * time.Millisecond)
	n := resolver.calls.Load()
	if n < 2 || n > 5 {
		t.Fatal("missing state busy-spun or retired", n)
	}
	resolver.missing.Store(false)
	waitFrame(t, frames)
	d.stopLighting()
}

func TestNativeOpenRGBClusterExplicitTransitions(t *testing.T) {
	d, _ := lightingFixture(t, true)
	frames := make(chan []byte, 20)
	d.lightingWrite = func(p []byte) { frames <- p }
	readyLighting(d)
	if d.ProcessSetRgbCluster(true) != 1 || d.ProcessSetOpenRgbIntegration(true) != 2 {
		t.Fatal("exclusivity")
	}
	if d.ProcessSetRgbCluster(false) != 1 {
		t.Fatal("cluster release")
	}
	waitFrame(t, frames)
	if d.ProcessSetOpenRgbIntegration(true) != 1 || d.ProcessSetRgbCluster(true) != 2 {
		t.Fatal("OpenRGB")
	}
	waitFrame(t, frames)
	if d.ProcessSetOpenRgbIntegration(false) != 1 {
		t.Fatal("OpenRGB release")
	}
	waitFrame(t, frames)
	p := cloneLightingProfile(*d.DeviceProfile)
	p.Path = filepath.Join(filepath.Dir(p.Path), "conflict.json")
	p.RGBCluster = true
	p.OpenRGBIntegration = true
	d.UserProfiles["conflict"] = &p
	before := inputJSON(t, d)
	if d.ChangeDeviceProfile("conflict") != 0 || !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("conflicting owners admitted")
	}
}

func TestGradientPersistenceFailureKeepsStops(t *testing.T) {
	d, rt := lightingFixture(t, true)
	v, e := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, e)
	requireLighting(t, d.SetLightingEffectSettings("gradient", v))
	d.lightingCanonical.effects = failingEffects{rt.Effects}
	if result, _ := d.ProcessNewGradientColor("gradient"); result != 0 {
		t.Fatal("add success")
	}
	if result, _ := d.ProcessDeleteGradientColor("gradient"); result != 0 {
		t.Fatal("delete success")
	}
	actual, e := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, e)
	if !reflect.DeepEqual(v, actual) {
		t.Fatal("failed gradient edit changed stops")
	}
}
