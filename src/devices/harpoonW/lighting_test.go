package harpoonW

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
	colors := []rgb.Color{{Red: 255}, {Red: 255, Green: 255, Blue: 255}, {Green: 255}, {Red: 255, Blue: 255}, {Green: 255, Blue: 255}, {Red: 255, Green: 255}}
	stages := map[int]DPIProfile{}
	for id, value := range []uint16{400, 800, 1200, 1600, 3200, 200} {
		c := colors[id]
		stages[id] = DPIProfile{Name: []string{"Stage 1", "Stage 2", "Stage 3", "Stage 4", "Stage 5", "Sniper"}[id], Value: value, PackerIndex: id + 1, Color: &c, ColorIndex: map[int][]int{0: {0, 2, 4}}, Sniper: id == 5}
	}
	profile := &DeviceProfile{Active: true, Serial: "harpoon-w-test", Product: "HARPOON RGB", Path: filepath.Join(root, "profile.json"), RGBProfile: "gradient", BrightnessSlider: &brightness, OriginalBrightness: 82, Profile: 1, PollingRate: 4, SleepMode: 15, Profiles: stages, ZoneColors: map[int]ZoneColors{0: {Name: "obsolete", Color: &rgb.Color{Red: 9}, ColorIndex: []int{99}}}}
	d := &Device{Serial: profile.Serial, Product: profile.Product, VendorId: 6940, ProductId: 7006, Endpoint: 9, Path: "9", LEDChannels: 2, ChangeableLedChannels: 1, ZoneAmount: 1, DPIAmount: 6, MinDPI: 200, MaxDPI: 10000, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"default": profile}, Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{"static": {StartColor: rgb.Color{Red: 9}}}}}
	d.lightingClusterRemove = func(string) {}
	d.lightingClusterRegister = func(*common.ClusterController) {}
	d.lightingOpenRGBReconcile = func(string, *common.OpenRGBController) {}
	d.lightingSetup = func() bool { return true }
	d.lightingDPIWrite = func([]byte, []byte) error { return nil }
	d.lightingWrite = func([]byte) {}
	data, _ := json.Marshal(profile)
	requireLighting(t, os.WriteFile(profile.Path, data, 0600))
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
func render(t *testing.T, d *Device) []byte {
	t.Helper()
	now := time.Now()
	return renderLightingFrame(frameState(t, d), rendererOwner(), &now)
}
func waitFrame(t *testing.T, frames <-chan []byte) []byte {
	t.Helper()
	select {
	case p := <-frames:
		return p
	case <-time.After(2 * time.Second):
		t.Fatal("no Lighting frame")
		return nil
	}
}
func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked Lighting operation")
	}
}
func inputJSON(t *testing.T, d *Device) []byte {
	t.Helper()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	data, err := json.Marshal(d.DeviceProfile)
	requireLighting(t, err)
	return data
}
func noFrame(t *testing.T, frames <-chan []byte) {
	t.Helper()
	select {
	case p := <-frames:
		t.Fatalf("unexpected frame %v", p)
	default:
	}
}

func TestCatalogueTopologyDefaultsEveryDispatch(t *testing.T) {
	d, rt := lightingFixture(t, true)
	if !reflect.DeepEqual(rgbModes, auditedEffects) || len(rgbModes) != 20 || d.LEDChannels != 2 || d.ChangeableLedChannels != 1 || d.ZoneAmount != 1 || d.Usb || d.ProductId != 7006 || common.ProductTypeHarpoonRgbW != 225 {
		t.Fatal("W contract")
	}
	s, ok := d.LightingSnapshot()
	if !ok || s.ConfiguredEffect != "static" || s.Brightness != 100 || s.SingleColorHex != "#00ffff" {
		t.Fatal(s)
	}
	var effects []string
	for _, e := range s.SupportedEffects {
		effects = append(effects, e.ID)
	}
	if !reflect.DeepEqual(effects, auditedEffects) {
		t.Fatal(effects)
	}
	if _, found, err := rt.State.Resolve(d.Serial); err != nil || found {
		t.Fatal("attachment imported legacy")
	}
	for _, effect := range auditedEffects {
		requireLighting(t, d.SetLightingEffect(effect))
		if _, ok := d.LightingSnapshot(); !ok {
			t.Fatal("no presentation", effect)
		}
		if got := render(t, d); len(got) != 3 {
			t.Fatal("no renderer dispatch", effect, got)
		}
	}
	requireLighting(t, d.SetLightingEffect("mouse"))
	s, _ = d.LightingSnapshot()
	zones := s.AuthoredZoneEditor.Zones
	if len(zones) != 1 || zones[0].ID != "0" || zones[0].Label != "Logo" || zones[0].ColorHex != "#ffff00" {
		t.Fatal(zones)
	}
	topology := lightingTopology()
	if !reflect.DeepEqual(topology[0].ColorIndex, []int{1, 3, 5}) {
		t.Fatal(topology)
	}
	topology[0].ColorIndex[0] = 99
	if lightingTopology()[0].ColorIndex[0] != 1 {
		t.Fatal("topology alias")
	}
	if got := composeLightingPacket([]byte{10, 20, 30}); !bytes.Equal(got, []byte{0, 10, 0, 20, 0, 30}) {
		t.Fatal(got)
	}
	for _, effect := range []string{"pastelspiralrainbow", "circle", "spinner", "circleshift"} {
		if d.SetLightingEffect(effect) == nil {
			t.Fatal("unadvertised selectable", effect)
		}
	}
	if d.SetLightingBrightness(101) == nil {
		t.Fatal("brightness range")
	}
}

func TestCanonicalSoleAuthorityCopiesAndReset(t *testing.T) {
	d, rt := lightingFixture(t, true)
	before := inputJSON(t, d)
	requireLighting(t, d.SetLightingEffect("mouse"))
	requireLighting(t, d.SetLightingBrightness(50))
	requireLighting(t, d.SetLightingZoneColor("mouse", "zone", "0", "", rgb.Color{Red: 200, Green: 100, Blue: 40}))
	if !bytes.Equal(render(t, d), []byte{100, 50, 20}) {
		t.Fatal("double scaling")
	}
	if !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("dual-wrote profile Lighting")
	}
	d.DeviceProfile.RGBProfile = "bad"
	*d.DeviceProfile.BrightnessSlider = 0
	d.DeviceProfile.ZoneColors[0].Color.Red = 250
	d.Rgb.Profiles["static"] = rgb.Profile{StartColor: rgb.Color{Red: 255}}
	s, _ := d.LightingSnapshot()
	if s.ConfiguredEffect != "mouse" || s.Brightness != 50 || s.AuthoredZoneEditor.Zones[0].ColorHex != "#c86428" {
		t.Fatal("legacy authority", s)
	}
	d.loadRgb()
	d.saveRgbProfile()
	d.saveDeviceProfile()
	if !bytes.Equal(render(t, d), []byte{100, 50, 20}) {
		t.Fatal("legacy reload authority")
	}
	requireLighting(t, d.SetLightingEffect("gradient"))
	settings, err := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, err)
	settings.Gradient.Stops[0].Color.Red = 123
	requireLighting(t, d.SetLightingEffectSettings("gradient", settings))
	settings.Gradient.Stops[0].Color.Red = 1
	got, err := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, err)
	if got.Gradient.Stops[0].Color.Red != 123 {
		t.Fatal("settings alias")
	}
	projected := d.GetRgbProfile("gradient")
	projected.Gradients[0] = rgb.Color{Red: 88}
	got, _ = d.ResolveLightingEffectSettings("gradient")
	if got.Gradient.Stops[0].Color.Red != 123 {
		t.Fatal("getter alias")
	}
	requireLighting(t, d.ResetLightingEffectSettings("gradient"))
	reset, found, err := rt.Effects.Get(d.Serial, "gradient")
	requireLighting(t, err)
	defaults, err := rt.Defaults.Get("gradient")
	requireLighting(t, err)
	if !found || !reflect.DeepEqual(reset, defaults) {
		t.Fatal("reset did not persist canonical default")
	}
	if d.UpdateRgbProfile(0, "static") != 1 || d.ChangeDeviceBrightnessValue(25) != 1 {
		t.Fatal("compatibility mutation")
	}
	for i := 0; i < 3; i++ {
		if !bytes.Equal(render(t, d), []byte{0, 63, 63}) {
			t.Fatal("cumulative Static scaling")
		}
	}
}

type failingState struct {
	lightingsettings.IndependentDeviceStateAccess
}

func (f failingState) Set(string, lightingsettings.IndependentDeviceLightingState) error {
	return errors.New("injected state write")
}

type failingEffects struct{ lightingEffectStore }

func (f failingEffects) Set(string, string, lightingsettings.EffectSettings) error {
	return errors.New("injected settings write")
}
func (f failingEffects) Delete(string, string) (bool, error) {
	return false, errors.New("injected reset write")
}
func (f failingEffects) SetAuthoredZones(string, string, map[string]lightingsettings.Color) error {
	return errors.New("injected palette write")
}
func TestPersistenceFailureDoesNotPublishCancelOrOutput(t *testing.T) {
	for _, operation := range []string{"effect", "brightness", "settings", "palette", "reset", "rgb-off", "cluster", "openrgb", "profile"} {
		t.Run(operation, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			// A live ownership token without a goroutine makes cancellation observable.
			worker := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{}), generation: 7}
			d.lightingWorker = worker
			d.nativeGeneration = 7
			d.Connected = true
			d.lightingHardwareReady = true
			before := inputJSON(t, d)
			s, _ := d.LightingSnapshot()
			generation := d.externalGeneration
			var writes atomic.Int32
			d.lightingWrite = func([]byte) { writes.Add(1) }
			d.lightingCanonical.state = failingState{d.lightingCanonical.state}
			d.lightingCanonical.effects = failingEffects{d.lightingCanonical.effects}
			d.lightingProfileWrite = func(string, []byte) error { return errors.New("injected profile write") }
			proposal := cloneLightingProfile(*d.DeviceProfile)
			proposal.Path = filepath.Join(t.TempDir(), "next.json")
			proposal.RgbOff = true
			d.UserProfiles["next"] = &proposal
			failed := false
			switch operation {
			case "effect":
				failed = d.SetLightingEffect("mouse") != nil
			case "brightness":
				failed = d.SetLightingBrightness(22) != nil
			case "settings":
				v, _ := d.ResolveLightingEffectSettings("static")
				v.SingleColor.Color.Red = 32
				failed = d.SetLightingEffectSettings("static", v) != nil
			case "palette":
				failed = d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 22}) != nil
			case "reset":
				failed = d.ResetLightingEffectSettings("static") != nil
			case "rgb-off":
				failed = d.setUserRGBOff(true) != nil
			case "cluster":
				failed = d.ProcessSetRgbCluster(true) == 0
			case "openrgb":
				failed = d.ProcessSetOpenRgbIntegration(true) == 0
			case "profile":
				failed = d.ChangeDeviceProfile("next") == 0
			}
			after, _ := d.LightingSnapshot()
			if !failed || !reflect.DeepEqual(s, after) || !bytes.Equal(before, inputJSON(t, d)) || d.lightingWorker != worker || d.externalGeneration != generation || writes.Load() != 0 {
				t.Fatal("failed persistence published/cancelled/output")
			}
			select {
			case <-worker.stop:
				t.Fatal("failed write cancelled worker")
			default:
			}
		})
	}
}

func TestSchedulerOfflineRGBOffAndLatestRestoration(t *testing.T) {
	d, rt := lightingFixture(t, true)
	before := inputJSON(t, d)
	for _, value := range []uint8{0, 0} {
		if d.SchedulerBrightness(value) != 1 {
			t.Fatal("offline scheduler rejected")
		}
	}
	requireLighting(t, d.SetLightingBrightness(40))
	if !bytes.Equal(render(t, d), []byte{0, 0, 0}) {
		t.Fatal("not dark")
	}
	if !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("scheduler persisted into profile")
	}
	state, _, _ := rt.State.Resolve(d.Serial)
	if state.Brightness != 40 || !d.schedulerDark {
		t.Fatal(state)
	}
	if d.SchedulerBrightness(255) != 1 || !bytes.Equal(render(t, d), []byte{0, 102, 102}) {
		t.Fatal("stale restoration")
	}
	requireLighting(t, d.setUserRGBOff(true))
	data, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	var saved DeviceProfile
	requireLighting(t, json.Unmarshal(data, &saved))
	if !saved.RgbOff || *saved.BrightnessSlider != 17 || saved.OriginalBrightness != 82 {
		t.Fatal("off/desired conflated")
	}
	requireLighting(t, d.SetLightingBrightness(60))
	if !bytes.Equal(render(t, d), []byte{0, 0, 0}) {
		t.Fatal("RGB-off ignored")
	}
	frames := make(chan []byte, 20)
	d.lightingWrite = func(p []byte) { frames <- p }
	d.Connect()
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 0, 255, 0, 255, 0}) {
		t.Fatal("offline off replay/indicator", got)
	}
	d.disconnectLighting(false)
	noFrame(t, frames)
	requireLighting(t, d.setUserRGBOff(false))
	requireLighting(t, d.SetLightingBrightness(20))
	d.SchedulerBrightness(0)
	d.Connect()
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 0, 255, 0, 255, 0}) {
		t.Fatal("wake scheduler", got)
	}
	d.SchedulerBrightness(1)
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 0, 255, 51, 255, 51}) {
		t.Fatal("wake latest", got)
	}
	// Restart attachment reads persisted off preference but clean/shared desired data.
	d.disconnectLighting(false)
	requireLighting(t, d.setUserRGBOff(true))
	next := &Device{Serial: d.Serial, Product: d.Product, LEDChannels: 2, ChangeableLedChannels: 1, ZoneAmount: 1, DeviceProfile: &saved}
	requireLighting(t, next.attachLightingSource(rt))
	if !next.userRGBOff {
		t.Fatal("restart lost persisted preference")
	}
	next.exitLighting()
}

func TestExternalOwnershipBytesPrecedenceAndRegistration(t *testing.T) {
	for _, owner := range []string{"cluster", "openrgb"} {
		t.Run(owner, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			frames := make(chan []byte, 40)
			d.lightingWrite = func(p []byte) { frames <- p }
			var clusterController *common.ClusterController
			var openController *common.OpenRGBController
			registrations := 0
			d.lightingClusterRegister = func(c *common.ClusterController) { clusterController = c; registrations++ }
			d.lightingOpenRGBReconcile = func(_ string, c *common.OpenRGBController) { openController = c; registrations++ }
			d.Connect()
			waitFrame(t, frames)
			d.startLightingQueue()
			if owner == "cluster" {
				if d.ProcessSetRgbCluster(true) != 1 || d.ProcessSetOpenRgbIntegration(true) != 2 {
					t.Fatal("exclusivity")
				}
			} else {
				if d.ProcessSetOpenRgbIntegration(true) != 1 || d.ProcessSetRgbCluster(true) != 2 {
					t.Fatal("exclusivity")
				}
				waitFrame(t, frames)
			}
			registrationsBefore := registrations
			d.setupClusterController()
			d.setupOpenRGBController()
			if registrations != registrationsBefore {
				t.Fatal("registration not idempotent")
			}
			oldCluster, oldOpen := clusterController, openController
			if oldOpen == nil || len(oldOpen.Colors) != 3 || len(oldOpen.Zones) != 1 || oldOpen.Zones[0].Name != "Logo" || oldOpen.Zones[0].NumLEDs != 1 || oldOpen.Location != "HID: 9" {
				t.Fatal("OpenRGB exposure")
			}
			if owner == "cluster" && (oldCluster == nil || oldCluster.LedChannels != 1) {
				t.Fatal("Cluster target")
			}
			requireLighting(t, d.SetLightingBrightness(1))
			requireLighting(t, d.SetLightingEffect("mouse"))
			requireLighting(t, d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 200}))
			d.SchedulerBrightness(0)
			noFrame(t, frames)
			requireLighting(t, d.setUserRGBOff(true))
			waitFrame(t, frames) // source one-shot black Logo
			input := []byte{11, 22, 33}
			if owner == "cluster" {
				oldCluster.WriteColorEx(input, 0)
			} else {
				oldOpen.WriteColorEx(input, 0)
				input[0] = 99
			}
			if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 11, 255, 22, 255, 33}) {
				t.Fatal("external bytes scaled/indicator lost", got)
			}
			d.disconnectLighting(false)
			if openController != nil {
				t.Fatal("offline OpenRGB registration")
			}
			if d.ProcessSetRgbCluster(false) != 0 || d.ProcessSetOpenRgbIntegration(false) != 0 || d.ChangeDeviceProfile("default") != 0 {
				t.Fatal("offline ownership/profile semantics changed")
			}
			requireLighting(t, d.SetLightingBrightness(30))
			d.Connect()
			waitFrame(t, frames)
			// Both old closures must be inert after replacement/reconnect.
			if oldCluster != nil {
				oldCluster.WriteColorEx([]byte{90, 90, 90}, 0)
			}
			oldOpen.WriteColorEx([]byte{90, 90, 90}, 0)
			noFrame(t, frames)
			if owner == "cluster" {
				if d.ProcessSetRgbCluster(false) != 1 {
					t.Fatal("release")
				}
			} else {
				if d.ProcessSetOpenRgbIntegration(false) != 1 {
					t.Fatal("release")
				}
			}
			waitFrame(t, frames) // persisted off remains effective after release
			requireLighting(t, d.setUserRGBOff(false))
			waitFrame(t, frames) // scheduler still dark
			d.SchedulerBrightness(1)
			if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 60, 255, 0, 255, 0}) {
				t.Fatal("latest native restoration", got)
			}
		})
	}
}

func TestProfileRefreshRollbackAndDPIIndicator(t *testing.T) {
	d, _ := lightingFixture(t, true)
	frames := make(chan []byte, 40)
	d.lightingWrite = func(p []byte) { frames <- p }
	var dpiMu sync.Mutex
	var dpiData [][]byte
	d.lightingDPIWrite = func(command, data []byte) error {
		if !bytes.Equal(command, []byte{1, 32, 0}) {
			t.Error(command)
		}
		dpiMu.Lock()
		dpiData = append(dpiData, append([]byte(nil), data...))
		dpiMu.Unlock()
		return nil
	}
	var clusters []*common.ClusterController
	d.lightingClusterRegister = func(c *common.ClusterController) { clusters = append(clusters, c) }
	d.Connect()
	waitFrame(t, frames)
	requireLighting(t, d.SetLightingEffect("mouse"))
	waitFrame(t, frames)
	if d.SetMouseSniperMode(true) != 1 {
		t.Fatal("Sniper")
	}
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 255, 255, 255, 0, 0}) {
		t.Fatal("Sniper touched Logo or wrong indicator", got)
	}
	d.disconnectLighting(false)
	d.Connect()
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 255, 255, 255, 0, 0}) {
		t.Fatal("Sniper reconnect", got)
	}
	dpiMu.Lock()
	last := dpiData[len(dpiData)-1]
	dpiMu.Unlock()
	if !bytes.Equal(last, []byte{0x20, 0x03}) {
		t.Fatal("Connect changed regular DPI command", last)
	}
	// No flash/Logo substitution; release is regular stage indicator again.
	d.SetMouseSniperMode(false)
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 255, 255, 255, 255, 0}) {
		t.Fatal(got)
	}
	profile := cloneLightingProfile(*d.DeviceProfile)
	profile.Path = filepath.Join(t.TempDir(), "other.json")
	profile.Active = false
	profile.RGBCluster = true
	profile.RgbOff = true
	profile.Profile = 0
	d.UserProfiles["other"] = &profile
	requireLighting(t, d.SetLightingBrightness(45))
	waitFrame(t, frames)
	d.SchedulerBrightness(0)
	waitFrame(t, frames)
	if d.ChangeDeviceProfile("other") != 1 {
		t.Fatal("profile switch")
	}
	waitFrame(t, frames)
	s, _ := d.LightingSnapshot()
	if s.Brightness != 45 || s.ConfiguredEffect != "mouse" || !s.ClusterControlled || !d.userRGBOff || !d.schedulerDark || len(clusters) != 1 {
		t.Fatal("profile reimported Lighting/lost preferences", s)
	}
	clusters[0].WriteColorEx([]byte{1, 2, 3}, 0)
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 1, 0, 2, 0, 3}) {
		t.Fatal("profile stage overlay", got)
	}
	if d.ChangeDeviceProfile("default") != 1 {
		t.Fatal("profile restore")
	}
	waitFrame(t, frames)
	clusters[0].WriteColorEx([]byte{8, 8, 8}, 0)
	noFrame(t, frames)
	// Fail second profile write, then restore old marker without runtime publication.
	before := inputJSON(t, d)
	gen := d.externalGeneration
	calls := 0
	d.lightingProfileWrite = func(path string, data []byte) error {
		calls++
		if calls == 2 {
			return errors.New("target fail")
		}
		return writeLightingProfile(path, data)
	}
	if d.ChangeDeviceProfile("other") != 0 || calls != 3 || d.externalGeneration != gen || !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("partial profile publication")
	}
	noFrame(t, frames)
}

func TestAttachmentFailureAndIndependentFallback(t *testing.T) {
	for _, failure := range []string{"nil", "topology", "state", "owners", "worker"} {
		t.Run(failure, func(t *testing.T) {
			d, rt := lightingFixture(t, false)
			before := inputJSON(t, d)
			switch failure {
			case "nil":
				rt = nil
			case "topology":
				d.LEDChannels = 1
			case "state":
				rt.State = failingResolve{rt.State}
			case "owners":
				d.DeviceProfile.RGBCluster = true
				d.DeviceProfile.OpenRGBIntegration = true
				before = inputJSON(t, d)
			case "worker":
				d.activeRgb = rgb.Exit()
			}
			if d.attachLightingSource(rt) == nil || d.lightingCanonical != nil || d.lightingWorker != nil || d.externalQueue != nil || !bytes.Equal(before, inputJSON(t, d)) {
				t.Fatal("partial attachment")
			}
			if _, ok := d.LightingSnapshot(); ok {
				t.Fatal("failed attachment produced snapshot")
			}
			// Legacy getters work directly without any canonical source.
			if d.GetRgbProfile("static").StartColor.Red != 9 {
				t.Fatal("fallback depends on stores")
			}
		})
	}
}

type failingResolve struct {
	lightingsettings.IndependentDeviceStateAccess
}

func (f failingResolve) Resolve(string) (lightingsettings.IndependentDeviceLightingState, bool, error) {
	return lightingsettings.IndependentDeviceLightingState{}, false, errors.New("unavailable")
}

func TestGradientMutationsValidateCompleteCopies(t *testing.T) {
	d, _ := lightingFixture(t, true)
	requireLighting(t, d.SetLightingEffect("gradient"))
	before, err := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, err)
	if result, _ := d.ProcessNewGradientColor("gradient"); result != 1 {
		t.Fatal("gradient add")
	}
	after, err := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, err)
	if len(after.Gradient.Stops) != len(before.Gradient.Stops)+1 {
		t.Fatal("incomplete added settings")
	}
	requireLighting(t, lightingsettings.Validate(after))
	before.Gradient.Stops[0].Color.Red = 7
	stable, err := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, err)
	if stable.Gradient.Stops[0].Color.Red == 7 {
		t.Fatal("gradient aliases prior resolution")
	}
	if result, _ := d.ProcessDeleteGradientColor("gradient"); result != 1 {
		t.Fatal("gradient delete")
	}
}

func TestColorwarpBrightnessScalesOnce(t *testing.T) {
	d, _ := lightingFixture(t, true)
	requireLighting(t, d.SetLightingEffect("colorwarp"))
	requireLighting(t, d.SetLightingBrightness(50))
	value := frameState(t, d)
	owner := rendererOwner()
	// Equal endpoints eliminate time/randomness; half brightness must be half the
	// authored channel, not a second multiplication of cached random endpoints.
	owner.LastCycle[0].RGBStartColor = &rgb.Color{Red: 200, Green: 100, Blue: 40}
	owner.LastCycle[0].RGBEndColor = &rgb.Color{Red: 200, Green: 100, Blue: 40}
	now := time.Now()
	if got := renderLightingFrame(value, owner, &now); !bytes.Equal(got, []byte{100, 50, 20}) {
		t.Fatal(got)
	}
}
