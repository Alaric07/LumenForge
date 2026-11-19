package m65rgbultraWU

import (
	"LumenForge/src/common"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

var auditedWUEffects = []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}

func mustLighting(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func doneLighting(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Lighting did not retire")
	}
}
func reportLighting(t *testing.T, reports <-chan []byte) []byte {
	t.Helper()
	select {
	case r := <-reports:
		return r
	case <-time.After(2 * time.Second):
		t.Fatal("no Lighting report")
		return nil
	}
}
func noLightingReport(t *testing.T, reports <-chan []byte) {
	t.Helper()
	select {
	case r := <-reports:
		t.Fatalf("unexpected report %x", r)
	default:
	}
}

type wuRegistry struct {
	sync.Mutex
	cluster  *common.ClusterController
	open     *common.OpenRGBController
	clusters int
	removes  int
}

func (r *wuRegistry) hooks(d *Device) {
	d.lightingClusterRegister = func(c *common.ClusterController) {
		r.Lock()
		defer r.Unlock()
		if r.cluster != nil {
			panic("duplicate Cluster registration")
		}
		r.cluster = c
		r.clusters++
	}
	d.lightingClusterRemove = func(string) { r.Lock(); r.cluster = nil; r.removes++; r.Unlock() }
	d.lightingOpenRGBReconcile = func(_ string, c *common.OpenRGBController) { r.Lock(); r.open = c; r.Unlock() }
}
func (r *wuRegistry) controllers() (*common.ClusterController, *common.OpenRGBController) {
	r.Lock()
	defer r.Unlock()
	return r.cluster, r.open
}

func wuFixture(t *testing.T, attach bool) (*Device, *lightingsettings.IndependentDeviceRuntime, *wuRegistry) {
	t.Helper()
	logger.Init()
	root := t.TempDir()
	old := pwd
	pwd = root
	t.Cleanup(func() { pwd = old })
	for _, dir := range []string{"profiles", "rgb", "key-assignments"} {
		mustLighting(t, os.MkdirAll(filepath.Join(root, "database", dir), 0700))
	}
	rt, err := lightingsettings.LoadIndependentDeviceRuntime(filepath.Join(root, "state.json"), filepath.Join(root, "effects.json"), "../../../database/rgb.json")
	mustLighting(t, err)
	d := &Device{Serial: "WU123", Product: "M65 RGB ULTRA", VendorId: 6940, ProductId: 7093, Usb: true, Connected: true, LEDChannels: 2, ChangeableLedChannels: 2, ZoneAmount: 1, DPIAmount: 6, MinDPI: 100, MaxDPI: 26000, keyAssignmentFile: "/database/key-assignments/m65RgbUltraW.json"}
	d.lightingReportIO = func([]byte) ([]byte, error) { return make([]byte, 64), nil }
	reg := &wuRegistry{}
	reg.hooks(d)
	d.saveDeviceProfile() // Exercise actual WU fresh defaults, not sibling fixtures.
	if d.DeviceProfile == nil {
		t.Fatal("profile bootstrap failed")
	}
	d.loadKeyAssignments()
	if attach {
		mustLighting(t, d.attachLightingSource(rt))
	}
	t.Cleanup(func() {
		if d.lightingAttached() {
			d.stopCanonicalLighting()
			d.lightingMu.Lock()
			started, done := d.lightingStarted, d.lightingDone
			d.lightingMu.Unlock()
			if started {
				doneLighting(t, done)
			}
		}
	})
	return d, rt, reg
}
func wuOutput(t *testing.T, d *Device) chan []byte {
	t.Helper()
	reports := make(chan []byte, 128)
	d.lightingReportIO = func(report []byte) ([]byte, error) {
		if report[2] == 6 && report[3] == 0 {
			reports <- append([]byte(nil), report...)
		}
		return make([]byte, 64), nil
	}
	mustLighting(t, d.SetLightingEffectSettings("static", lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 100, Green: 80, Blue: 60}}}))
	d.initLeds()
	d.setDeviceColor()
	return reports
}
func wuFrame(t *testing.T, d *Device) lightingFrameState {
	t.Helper()
	d.lightingMu.Lock()
	frame, err := d.lightingFrameLocked()
	d.lightingMu.Unlock()
	mustLighting(t, err)
	return frame
}
func wuRender(t *testing.T, d *Device) []byte {
	t.Helper()
	start := time.Now()
	owner := rgb.Exit()
	owner.RGBStartColor = &rgb.Color{Red: 100}
	owner.RGBEndColor = &rgb.Color{Blue: 100}
	return renderLightingFrame(wuFrame(t, d), owner, &start)
}
func expectWUPacket(t *testing.T, report, packet []byte) {
	t.Helper()
	if len(report) != 65 || !reflect.DeepEqual(report[:8], []byte{0, 8, 6, 0, 6, 0, 0, 0}) || !reflect.DeepEqual(report[8:14], packet) {
		t.Fatalf("report=%x, want packet %x", report, packet)
	}
	for _, b := range report[14:] {
		if b != 0 {
			t.Fatal("nonzero report padding")
		}
	}
}

func TestWUCanonicalDefaultsCatalogueAndAuthority(t *testing.T) {
	d, rt, _ := wuFixture(t, true)
	if !reflect.DeepEqual(rgbModes, auditedWUEffects) {
		t.Fatal(rgbModes)
	}
	if got := lightingTopology(); len(got) != 1 || got[0].Name != "Logo" || !reflect.DeepEqual(got[0].ColorIndex, []int{0, 2, 4}) {
		t.Fatal(got)
	}
	state, found, err := rt.State.Resolve(d.Serial)
	mustLighting(t, err)
	if found || state.SelectedEffect != "static" || state.Brightness != 100 {
		t.Fatal(state, found)
	}
	if _, err := os.Stat(filepath.Join(pwd, "effects.json")); !os.IsNotExist(err) {
		t.Fatal("attachment wrote effect customization", err)
	}
	if _, err := os.Stat(filepath.Join(pwd, "state.json")); !os.IsNotExist(err) {
		t.Fatal("attachment persisted clean state", err)
	}
	// Conflicting legacy authored colors, offsets and effect cannot affect canonical data.
	d.DeviceProfile.RGBProfile = "not-supported"
	*d.DeviceProfile.BrightnessSlider = 7
	d.DeviceProfile.ZoneColors[0] = ZoneColors{Name: "wrong", ColorIndex: []int{99}, Color: &rgb.Color{Blue: 9}}
	d.Rgb = &rgb.RGB{Profiles: map[string]rgb.Profile{"static": {StartColor: rgb.Color{Blue: 3}}}}
	s, ok := d.LightingSnapshot()
	if !ok || s.ConfiguredEffect != "static" || s.Brightness != 100 || len(s.SupportedEffects) != 20 {
		t.Fatal(s, ok)
	}
	for i, e := range s.SupportedEffects {
		if e.ID != auditedWUEffects[i] {
			t.Fatal(i, e)
		}
	}
	mustLighting(t, d.SetLightingEffect("mouse"))
	if got := wuRender(t, d); !reflect.DeepEqual(got, []byte{255, 255, 0}) {
		t.Fatal(got)
	}
	s, ok = d.LightingSnapshot()
	if !ok || len(s.AuthoredZoneEditor.Zones) != 1 || s.AuthoredZoneEditor.Zones[0].ID != "0" || s.AuthoredZoneEditor.Zones[0].ColorHex != "#ffff00" {
		t.Fatal(s)
	}
	if err := d.SetLightingEffect("circle"); err == nil {
		t.Fatal("unadvertised effect accepted")
	}
	legacyProfile, _ := json.Marshal(d.DeviceProfile)
	legacyRGB, _ := json.Marshal(d.Rgb)
	mustLighting(t, d.SetLightingBrightness(61))
	if d.SaveMouseZoneColors(rgb.Color{}, map[int]rgb.Color{0: {Red: 12, Green: 34, Blue: 56}}) != 1 {
		t.Fatal("Mouse color update")
	}
	afterProfile, _ := json.Marshal(d.DeviceProfile)
	afterRGB, _ := json.Marshal(d.Rgb)
	if string(legacyProfile) != string(afterProfile) || string(legacyRGB) != string(afterRGB) {
		t.Fatal("canonical mutation wrote legacy authority")
	}
	if d.activeRgb != nil || d.lightingStarted {
		t.Fatal("data-only fixture started hardware/legacy worker")
	}
}

type failWUState struct {
	lightingsettings.IndependentDeviceStateAccess
}

func (f failWUState) Set(string, lightingsettings.IndependentDeviceLightingState) error {
	return errors.New("state write failed")
}

type failWUEffects struct{ lightingEffectStore }

func (f failWUEffects) Set(string, string, lightingsettings.EffectSettings) error {
	return errors.New("effect write failed")
}
func (f failWUEffects) Delete(string, string) (bool, error) {
	return false, errors.New("effect delete failed")
}
func (f failWUEffects) SetAuthoredZones(string, string, map[string]lightingsettings.Color) error {
	return errors.New("palette write failed")
}
func TestWUPersistenceFailureDoesNotPublishOrRestart(t *testing.T) {
	d, rt, _ := wuFixture(t, true)
	reports := wuOutput(t, d)
	reportLighting(t, reports)
	d.lightingMu.Lock()
	worker := d.lightingWorker
	stateStore, effects := d.lightingCanonical.state, d.lightingCanonical.effects
	d.lightingCanonical.state = failWUState{stateStore}
	d.lightingMu.Unlock()
	if d.SetLightingBrightness(20) == nil || d.SetLightingEffect("mouse") == nil {
		t.Fatal("failed state write succeeded")
	}
	state, _, _ := rt.State.Resolve(d.Serial)
	if state.Brightness != 100 || state.SelectedEffect != "static" {
		t.Fatal(state)
	}
	d.lightingMu.Lock()
	unchanged := worker == d.lightingWorker
	d.lightingCanonical.state = stateStore
	d.lightingCanonical.effects = failWUEffects{effects}
	d.lightingMu.Unlock()
	if !unchanged {
		t.Fatal("failed write replaced renderer")
	}
	settings, err := d.ResolveLightingEffectSettings("static")
	mustLighting(t, err)
	settings.SingleColor.Color.Red = 1
	if d.SetLightingEffectSettings("static", settings) == nil || d.ResetLightingEffectSettings("static") == nil || d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 9}) == nil {
		t.Fatal("failed effects write succeeded")
	}
	d.lightingProfileWrite = func(DeviceProfile) error { return errors.New("profile failed") }
	if d.setUserRGBOff(true) == nil || d.ProcessSetOpenRgbIntegration(true) != 0 {
		t.Fatal("failed preference write succeeded")
	}
	if d.DeviceProfile.RgbOff || d.DeviceProfile.OpenRGBIntegration {
		t.Fatal("preference published before persistence")
	}
	d.lightingMu.Lock()
	unchanged = worker == d.lightingWorker && !d.userRGBOff && d.owner.kind == lightingNative
	d.lightingMu.Unlock()
	if !unchanged {
		t.Fatal("failed persistence changed output")
	}
	noLightingReport(t, reports)
}

func TestWUFailedAttachmentRetainsIndependentLegacy(t *testing.T) {
	d, rt, _ := wuFixture(t, false)
	rtBad := *rt
	rtBad.Defaults = nil
	if d.attachLightingSource(&rtBad) == nil || d.lightingAttached() || d.lightingQueue != nil {
		t.Fatal("partial attach published")
	}
	mustLighting(t, rt.State.Set(d.Serial, lightingsettings.IndependentDeviceLightingState{SelectedEffect: "circle", Brightness: 100}))
	if d.attachLightingSource(rt) == nil || d.lightingAttached() || d.lightingStarted {
		t.Fatal("unsupported effect attached")
	}
	legacy, err := os.ReadFile("../../../database/rgb.json")
	mustLighting(t, err)
	mustLighting(t, os.WriteFile(filepath.Join(pwd, "database", "rgb", d.Serial+".json"), legacy, 0600))
	d.loadRgb()
	if d.Rgb == nil || d.GetRgbProfile("static") == nil || d.GetRgbProfiles() == nil {
		t.Fatal("independent legacy unavailable")
	}
	_, err = rt.State.Delete(d.Serial)
	mustLighting(t, err)
}

func TestWUPacketBrightnessAndIndicatorSeparation(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	indicator := *d.DeviceProfile.Profiles[1].Color
	reports := wuOutput(t, d)
	expectWUPacket(t, reportLighting(t, reports), []byte{100, 255, 80, 255, 60, 255})
	mustLighting(t, d.SetLightingBrightness(40))
	expectWUPacket(t, reportLighting(t, reports), []byte{40, 102, 32, 102, 24, 102})
	mustLighting(t, d.setUserRGBOff(true))
	expectWUPacket(t, reportLighting(t, reports), []byte{0, 102, 0, 102, 0, 102})
	if *d.DeviceProfile.Profiles[1].Color != indicator {
		t.Fatal("indicator source was scaled/mutated")
	}
	mustLighting(t, d.setUserRGBOff(false))
	reportLighting(t, reports)
	mustLighting(t, d.SetLightingEffect("off"))
	expectWUPacket(t, reportLighting(t, reports), []byte{0, 102, 0, 102, 0, 102})
}

func TestWUSchedulerRuntimeOnlyLatestDesired(t *testing.T) {
	d, rt, _ := wuFixture(t, true)
	reports := wuOutput(t, d)
	reportLighting(t, reports)
	before, err := os.ReadFile(d.DeviceProfile.Path)
	mustLighting(t, err)
	mustLighting(t, d.setSchedulerDark(true))
	expectWUPacket(t, reportLighting(t, reports), make([]byte, 6))
	mustLighting(t, d.SetLightingBrightness(73))
	expectWUPacket(t, reportLighting(t, reports), make([]byte, 6))
	mustLighting(t, d.setSchedulerDark(true)) // idempotent darkness must not snapshot zero
	state, _, err := rt.State.Resolve(d.Serial)
	mustLighting(t, err)
	if state.Brightness != 73 {
		t.Fatal(state)
	}
	after, err := os.ReadFile(d.DeviceProfile.Path)
	mustLighting(t, err)
	if string(before) != string(after) {
		t.Fatal("scheduler wrote profile")
	}
	mustLighting(t, d.setSchedulerDark(false))
	expectWUPacket(t, reportLighting(t, reports), []byte{73, 186, 58, 186, 43, 186})
	s, ok := d.LightingSnapshot()
	if !ok || s.Brightness != 73 {
		t.Fatal(s)
	}
}

func TestWUExternalTopologyOwnershipAndRGBOffPrecedence(t *testing.T) {
	for _, kind := range []lightingOwner{lightingCluster, lightingOpenRGB} {
		t.Run(map[lightingOwner]string{lightingCluster: "Cluster", lightingOpenRGB: "OpenRGB"}[kind], func(t *testing.T) {
			d, rt, reg := wuFixture(t, true)
			reports := wuOutput(t, d)
			reportLighting(t, reports)
			mustLighting(t, d.SetLightingBrightness(50))
			reportLighting(t, reports)
			if d.setLightingOwnership(kind == lightingCluster, true) != 1 {
				t.Fatal("owner enable")
			}
			c, o := reg.controllers()
			var send func([]byte, int)
			if kind == lightingCluster {
				if c == nil || c.LedChannels != 1 {
					t.Fatal(c)
				}
				send = c.WriteColorEx
			} else {
				if o == nil || len(o.Colors) != 3 || len(o.Zones) != 1 || o.Zones[0].Name != "Logo" || o.Zones[0].NumLEDs != 1 || o.Location != "HID: " {
					t.Fatal(o)
				}
				send = o.WriteColorEx
				reportLighting(t, reports)
			}
			data := []byte{11, 22, 33}
			send(data, 0)
			data[0] = 99
			expectWUPacket(t, reportLighting(t, reports), []byte{11, 127, 22, 127, 33, 127})
			mustLighting(t, d.setSchedulerDark(true))
			expectWUPacket(t, reportLighting(t, reports), []byte{11, 0, 22, 0, 33, 0})
			mustLighting(t, d.setUserRGBOff(true))
			expectWUPacket(t, reportLighting(t, reports), make([]byte, 6))
			send([]byte{44, 55, 66}, 0)
			expectWUPacket(t, reportLighting(t, reports), []byte{44, 0, 55, 0, 66, 0})
			// Persisting local desired state cannot retire a valid external callback.
			mustLighting(t, d.SetLightingBrightness(80))
			reportLighting(t, reports)
			mustLighting(t, d.SetLightingEffect("mouse"))
			expectWUPacket(t, reportLighting(t, reports), []byte{44, 0, 55, 0, 66, 0}) // consume legitimate replay before next callback
			send([]byte{77, 88, 99}, 0)
			expectWUPacket(t, reportLighting(t, reports), []byte{77, 0, 88, 0, 99, 0})
			if d.setLightingOwnership(kind == lightingCluster, false) != 1 {
				t.Fatal("owner disable")
			}
			reportLighting(t, reports)
			send([]byte{200, 201, 202}, 0) // retained callback belongs to retired owner
			noLightingReport(t, reports)
			if d.setLightingOwnership(kind == lightingCluster, true) != 1 {
				t.Fatal("owner restore")
			}
			reportLighting(t, reports) // persisted RGB-off blanks the restored owner once
			c, o = reg.controllers()
			var restored func([]byte, int)
			if kind == lightingCluster {
				restored = c.WriteColorEx
			} else {
				restored = o.WriteColorEx
			}
			send([]byte{200, 201, 202}, 0) // previous owner token remains stale after restore
			restored([]byte{1, 2, 3}, 0)
			expectWUPacket(t, reportLighting(t, reports), []byte{1, 0, 2, 0, 3, 0})
			d.setupOpenRGBController()
			d.setupClusterController() // reconciliation cannot duplicate registrations
			state, _, _ := rt.State.Resolve(d.Serial)
			if state.SelectedEffect != "mouse" || state.Brightness != 80 {
				t.Fatal(state)
			}
			d.stopCanonicalLighting()
			doneLighting(t, d.lightingDone)
			c, o = reg.controllers()
			if c != nil || o != nil {
				t.Fatal("controllers survived stop")
			}
			send([]byte{200, 201, 202}, 0)
			noLightingReport(t, reports)
		})
	}
}

func TestWULatestExternalFrameAndIndicatorReplay(t *testing.T) {
	d, _, reg := wuFixture(t, true)
	reports := wuOutput(t, d)
	reportLighting(t, reports)
	if d.ProcessSetOpenRgbIntegration(true) != 1 {
		t.Fatal("enable")
	}
	reportLighting(t, reports)
	_, controller := reg.controllers()
	// Hold the final serializer, not a producer lock. Newest frame must survive
	// a burst and a simultaneous indicator refresh before any external write.
	d.deviceLock.Lock()
	unlockTransport := unlockWU(t, &d.deviceLock)
	for i := 1; i <= 40; i++ {
		controller.WriteColorEx([]byte{byte(i), 2, 3}, 0)
	}
	mustLighting(t, d.SetLightingBrightness(60))
	unlockTransport()
	expectWUPacket(t, reportLighting(t, reports), []byte{40, 153, 2, 153, 3, 153})
}

func TestWUPrivateWorkerRetirementAndFinalAuthorization(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	reports := make(chan []byte, 32)
	d.lightingReportIO = func(r []byte) ([]byte, error) { reports <- append([]byte(nil), r...); return make([]byte, 64), nil }
	entered, release := make(chan struct{}), make(chan struct{})
	unblock := releaseWU(t, release)
	var once sync.Once
	d.lightingBeforeApply = func() { once.Do(func() { close(entered); <-release }) }
	d.lightingMu.Lock()
	d.lightingHardwareReady = true
	d.lightingMu.Unlock()
	mustLighting(t, d.SetLightingEffect("rainbow"))
	waitWU(t, entered, "output pump reached held transport milestone")
	d.lightingMu.Lock()
	old := d.lightingWorker
	d.lightingMu.Unlock()
	mustLighting(t, d.SetLightingEffect("mouse"))
	doneLighting(t, old.done) // old renderer exits even with output pump unavailable
	unblock()
	expectWUPacket(t, reportLighting(t, reports), []byte{255, 255, 255, 255, 0, 255})
	d.lightingMu.Lock()
	current := d.lightingWorker
	d.lightingMu.Unlock()
	doneLighting(t, current.done)
	if d.activeRgb != nil {
		t.Fatal("canonical renderer used legacy activeRgb")
	}
	noLightingReport(t, reports)
	// Cancellation while waiting for the real USB mutex does not wait on HID.
	d.mutex.Lock()
	unlockUSB := unlockWU(t, &d.mutex)
	mustLighting(t, d.SetLightingEffect("rainbow"))
	d.lightingMu.Lock()
	worker := d.lightingWorker
	d.lightingMu.Unlock()
	d.stopCanonicalLighting()
	doneLighting(t, worker.done)
	unlockUSB()
	doneLighting(t, d.lightingDone)
	noLightingReport(t, reports)
}

func TestWUBoundedRendererCancellationDuringIssuedUSBRead(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	entered, release := make(chan struct{}), make(chan struct{})
	unblock := releaseWU(t, release)
	d.lightingReportIO = func([]byte) ([]byte, error) { close(entered); <-release; return make([]byte, 64), nil }
	d.lightingMu.Lock()
	d.lightingHardwareReady = true
	d.lightingMu.Unlock()
	mustLighting(t, d.SetLightingEffect("rainbow"))
	waitWU(t, entered, "output pump reached held transport milestone")
	d.lightingMu.Lock()
	worker := d.lightingWorker
	d.lightingMu.Unlock()
	d.stopCanonicalLighting()
	doneLighting(t, worker.done)
	// The pre-existing blocking transport call is not falsely claimed interruptible.
	unblock()
	doneLighting(t, d.lightingDone)
}

func TestWUProfileReplayKeepsCanonicalAuthority(t *testing.T) {
	d, rt, reg := wuFixture(t, true)
	alternate := cloneLightingProfile(*d.DeviceProfile)
	alternate.Active = false
	alternate.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-alternate.json")
	alternate.RGBProfile = "off"
	*alternate.BrightnessSlider = 5
	alternate.RgbOff = true
	alternate.RGBCluster = true
	alternate.Profile = 2
	mustLighting(t, common.SaveJsonData(alternate.Path, alternate))
	d.loadDeviceProfiles()
	reports := wuOutput(t, d)
	reportLighting(t, reports)
	mustLighting(t, d.SetLightingBrightness(60))
	reportLighting(t, reports)
	before, _, _ := rt.State.Resolve(d.Serial)
	if d.ChangeDeviceProfile("alternate") != 1 {
		t.Fatal("switch")
	}
	expectWUPacket(t, reportLighting(t, reports), []byte{0, 0, 0, 153, 0, 0})
	after, _, _ := rt.State.Resolve(d.Serial)
	if before != after {
		t.Fatal("profile replaced canonical desired state")
	}
	c, _ := reg.controllers()
	if c == nil || c.LedChannels != 1 {
		t.Fatal("profile did not reconcile owner")
	}
	c.WriteColorEx([]byte{8, 9, 10}, 0)
	expectWUPacket(t, reportLighting(t, reports), []byte{8, 0, 9, 153, 10, 0})
	if d.ChangeDeviceProfile("default") != 1 {
		t.Fatal("restore")
	}
	reportLighting(t, reports)
	c.WriteColorEx([]byte{99, 99, 99}, 0)
	noLightingReport(t, reports)
	saved := &DeviceProfile{}
	raw, err := os.ReadFile(alternate.Path)
	mustLighting(t, err)
	mustLighting(t, json.Unmarshal(raw, saved))
	if !saved.RgbOff {
		t.Fatal("RGB-off profile persistence lost")
	}
}

func TestWUDPISniperDefaultsCommandsAndIndicator(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	values := []uint16{400, 800, 1200, 1600, 3000, 200}
	for i, v := range values {
		p := d.DeviceProfile.Profiles[i]
		if p.Value != v || p.Sniper != (i == 5) || !reflect.DeepEqual(p.ColorIndex, map[int][]int{0: {1, 3, 5}}) {
			t.Fatal(i, p)
		}
	}
	if d.MinDPI != 100 || d.MaxDPI != 26000 || d.DeviceProfile.Profile != 1 || *d.DeviceProfile.Profiles[5].Color != (rgb.Color{Red: 255, Green: 255, Brightness: 1, Hex: "#ffff00"}) {
		t.Fatal("WU defaults")
	}
	reports := make(chan []byte, 16)
	d.lightingReportIO = func(r []byte) ([]byte, error) { reports <- append([]byte(nil), r...); return make([]byte, 64), nil }
	d.toggleDPI()
	for _, cmd := range []byte{0x21, 0x22} {
		r := reportLighting(t, reports)
		if len(r) != 65 || !reflect.DeepEqual(r[:5], []byte{0, 8, 1, cmd, 0}) || binary.LittleEndian.Uint16(r[5:7]) != 800 {
			t.Fatalf("regular DPI %x", r)
		}
	}
	d.sniperMode(true)
	for _, cmd := range []byte{0x21, 0x22} {
		r := reportLighting(t, reports)
		if r[3] != cmd || binary.LittleEndian.Uint16(r[5:7]) != 200 {
			t.Fatal("Sniper DPI", r)
		}
	}
	d.lightingMu.Lock()
	indicator := d.lightingIndicator
	d.lightingMu.Unlock()
	if indicator.Red != 255 || indicator.Green != 255 || indicator.Blue != 0 {
		t.Fatal(indicator)
	}
	if p := composeLightingPacket([3]byte{1, 2, 3}, indicator, 100); !reflect.DeepEqual(p, []byte{1, 255, 2, 255, 3, 0}) {
		t.Fatal(p)
	}
	d.sniperMode(false)
	reportLighting(t, reports)
	reportLighting(t, reports)
	for _, v := range []uint16{1, 65535} {
		p := d.DeviceProfile.Profiles[1]
		p.Value = v
		d.DeviceProfile.Profiles[1] = p
		d.toggleDPI()
		for i := 0; i < 2; i++ {
			r := reportLighting(t, reports)
			got := binary.LittleEndian.Uint16(r[5:7])
			if got < 100 || got > 26000 {
				t.Fatal(got)
			}
		}
	}
}

func TestWUGradientIdentityOrderingPersistenceAndRenderer(t *testing.T) {
	d, rt, _ := wuFixture(t, true)
	p := rgb.Profile{Speed: 2, Gradients: map[int]rgb.Color{7: {Red: 100, Position: .6, Brightness: .7}, 2: {Green: 200, Position: .1, Brightness: 1}, 19: {Blue: 255, Position: .9, Brightness: .5}}}
	if d.UpdateRgbProfileData("gradient", p) != 1 {
		t.Fatal("Gradient edit")
	}
	p.Gradients[7] = rgb.Color{Red: 9}
	got, err := d.ResolveLightingEffectSettings("gradient")
	mustLighting(t, err)
	if !reflect.DeepEqual(got.Gradient.Keys, []int{2, 7, 19}) || got.Gradient.Stops[1].Color.Red != 100 {
		t.Fatal(got)
	}
	legacy := d.GetRgbProfile("gradient")
	if legacy.Gradients[7].Red != 100 || len(legacy.Gradients) != 3 {
		t.Fatal(legacy)
	}
	legacy.Gradients[7] = rgb.Color{}
	if d.GetRgbProfile("gradient").Gradients[7].Red != 100 {
		t.Fatal("renderer profile aliases canonical data")
	}
	status, key := d.ProcessNewGradientColor("gradient")
	if status != 1 || key != 20 {
		t.Fatal(status, key)
	}
	status, key = d.ProcessDeleteGradientColor("gradient")
	if status != 1 || key != 20 {
		t.Fatal(status, key)
	}
	store, err := lightingsettings.LoadDeviceStore(filepath.Join(pwd, "effects.json"))
	mustLighting(t, err)
	loaded, found, err := store.Get(d.Serial, "gradient")
	mustLighting(t, err)
	if !found || !reflect.DeepEqual(loaded.Gradient.Keys, []int{2, 7, 19}) {
		t.Fatal(loaded)
	}
	mustLighting(t, d.SetLightingEffect("gradient"))
	if data := wuRender(t, d); len(data) != 3 || data[1] == 0 {
		t.Fatal("sparse keys failed renderer conversion", data)
	}
	got.Gradient.Stops[0].Color.Green = 0
	fresh, err := d.ResolveLightingEffectSettings("gradient")
	mustLighting(t, err)
	if fresh.Gradient.Stops[0].Color.Green != 200 {
		t.Fatal("resolved value aliases")
	}
	mustLighting(t, d.ResetLightingEffectSettings("gradient"))
	if _, found, err := rt.Effects.Get(d.Serial, "gradient"); err != nil || found {
		t.Fatal("Reset did not delete customization", found, err)
	}
	status, key = d.ProcessDeleteGradientColor("static")
	if status != 0 || key != 0 {
		t.Fatal("non-gradient mutation")
	}
}

type corruptWUResolver struct{ lightingResolver }

func (c corruptWUResolver) Resolve(target lightingsettings.Target, effect string) (lightingsettings.Resolution, error) {
	if effect == "gradient" {
		return lightingsettings.Resolution{}, errors.New("corrupt Gradient")
	}
	return c.lightingResolver.Resolve(target, effect)
}
func TestWUTargetedResetDoesNotResolveCorruptCustomization(t *testing.T) {
	d, rt, _ := wuFixture(t, true)
	settings, err := d.ResolveLightingEffectSettings("gradient")
	mustLighting(t, err)
	mustLighting(t, d.SetLightingEffectSettings("gradient", settings))
	original := d.lightingCanonical.resolver
	d.lightingCanonical.resolver = corruptWUResolver{original}
	if _, ok := d.LightingSnapshot(); ok {
		t.Fatal("corrupt settings snapshot accepted")
	}
	mustLighting(t, d.ResetLightingEffectSettings("gradient"))
	if _, found, err := rt.Effects.Get(d.Serial, "gradient"); err != nil || found {
		t.Fatal(found, err)
	}
	d.lightingCanonical.resolver = original
	if _, ok := d.LightingSnapshot(); !ok {
		t.Fatal("Reset did not recover")
	}
}

func TestWUEveryEffectDispatchAndTwoSampleRendering(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	for _, effect := range auditedWUEffects {
		mustLighting(t, d.SetLightingEffect(effect))
		if got := wuRender(t, d); len(got) != 3 {
			t.Fatalf("%s dispatched %v", effect, got)
		}
	}
}

func TestWUStaleExternalFrameRejectedAtFinalSerializer(t *testing.T) {
	d, _, reg := wuFixture(t, true)
	entered, release := make(chan struct{}), make(chan struct{})
	unblock := releaseWU(t, release)
	calls := 0 // Called only by the single output pump.
	d.lightingBeforeApply = func() {
		calls++
		if calls == 3 {
			close(entered)
			<-release
		}
	}
	reports := wuOutput(t, d)
	reportLighting(t, reports)
	if d.ProcessSetOpenRgbIntegration(true) != 1 {
		t.Fatal("enable")
	}
	reportLighting(t, reports)
	_, controller := reg.controllers()
	controller.WriteColorEx([]byte{91, 92, 93}, 0)
	waitWU(t, entered, "output pump reached held transport milestone")
	if d.ProcessSetOpenRgbIntegration(false) != 1 {
		t.Fatal("disable blocked by stale output")
	}
	unblock()
	expectWUPacket(t, reportLighting(t, reports), []byte{100, 255, 80, 255, 60, 255})
	d.lightingMu.Lock()
	worker := d.lightingWorker
	d.lightingMu.Unlock()
	doneLighting(t, worker.done)
	noLightingReport(t, reports)
}

func TestWUOutputPacingObservesCancellation(t *testing.T) {
	stop := make(chan struct{})
	close(stop)
	result := make(chan bool, 1)
	go func() { result <- lightingDelay(stop, time.Hour) }()
	select {
	case elapsed := <-result:
		if elapsed {
			t.Fatal("cancelled pacing elapsed")
		}
	case <-time.After(time.Second):
		t.Fatal("pacing did not observe cancellation")
	}
}

func TestWUFreshUSBObjectReplaysRGBOffWithoutLegacyImport(t *testing.T) {
	d, rt, reg := wuFixture(t, true)
	reports := wuOutput(t, d)
	reportLighting(t, reports)
	mustLighting(t, d.SetLightingBrightness(40))
	reportLighting(t, reports)
	if d.ProcessSetOpenRgbIntegration(true) != 1 {
		t.Fatal("enable")
	}
	reportLighting(t, reports)
	_, oldController := reg.controllers()
	mustLighting(t, d.setUserRGBOff(true))
	reportLighting(t, reports)
	d.stopCanonicalLighting()
	doneLighting(t, d.lightingDone)
	data, err := os.ReadFile(d.DeviceProfile.Path)
	mustLighting(t, err)
	p := &DeviceProfile{}
	mustLighting(t, json.Unmarshal(data, p))
	next := &Device{Serial: d.Serial, Product: d.Product, LEDChannels: 2, ChangeableLedChannels: 2, ZoneAmount: 1, DeviceProfile: p}
	reg.hooks(next)
	next.lightingReportIO = func(r []byte) ([]byte, error) {
		if r[2] == 6 && r[3] == 0 {
			reports <- append([]byte(nil), r...)
		}
		return make([]byte, 64), nil
	}
	mustLighting(t, next.attachLightingSource(rt))
	t.Cleanup(func() { next.stopCanonicalLighting(); doneLighting(t, next.lightingDone) })
	next.initLeds()
	next.toggleDPI()
	next.setupOpenRGBController()
	next.setupClusterController()
	expectWUPacket(t, reportLighting(t, reports), []byte{0, 102, 0, 102, 0, 102})
	oldController.WriteColorEx([]byte{99, 99, 99}, 0)
	noLightingReport(t, reports)
	_, controller := reg.controllers()
	if controller == nil {
		t.Fatal("fresh USB object did not register")
	}
	controller.WriteColorEx([]byte{7, 8, 9}, 0)
	expectWUPacket(t, reportLighting(t, reports), []byte{7, 102, 8, 102, 9, 102})
	state, _, err := rt.State.Resolve(d.Serial)
	mustLighting(t, err)
	if state.Brightness != 40 || state.SelectedEffect != "static" || next.Rgb != nil || next.SniperMode {
		t.Fatal("replug imported legacy or held Sniper", state)
	}
}

type pendingWUState struct {
	lightingsettings.IndependentDeviceStateAccess
	entered, release chan struct{}
}

func (p pendingWUState) Set(id string, state lightingsettings.IndependentDeviceLightingState) error {
	close(p.entered)
	<-p.release
	return p.IndependentDeviceStateAccess.Set(id, state)
}
func TestWUDesiredPublicationWaitsForPersistence(t *testing.T) {
	d, rt, _ := wuFixture(t, true)
	reports := wuOutput(t, d)
	reportLighting(t, reports)
	gate := pendingWUState{rt.State, make(chan struct{}), make(chan struct{})}
	release := releaseWU(t, gate.release)
	d.lightingMu.Lock()
	d.lightingCanonical.state = gate
	d.lightingMu.Unlock()
	result := make(chan error, 1)
	go func() { result <- d.SetLightingBrightness(20) }()
	waitWU(t, gate.entered, "desired-state writer entered persistence gate")
	state, _, err := rt.State.Resolve(d.Serial)
	mustLighting(t, err)
	if state.Brightness != 100 {
		t.Fatal("desired state published before persistence", state)
	}
	noLightingReport(t, reports)
	release()
	mustLighting(t, waitWU(t, result, "desired-state persistence mutation completed"))
	expectWUPacket(t, reportLighting(t, reports), []byte{20, 51, 16, 51, 12, 51})
}
