package m65rgbultraW

import (
	"LumenForge/src/common"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"bytes"
	"encoding/binary"
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

func TestWirelessCatalogueTopologyEveryDispatch(t *testing.T) {
	d, rt := lightingFixture(t, true)
	if !reflect.DeepEqual(rgbModes, auditedEffects) {
		t.Fatal(rgbModes)
	}
	snap, ok := d.LightingSnapshot()
	if !ok || snap.ConfiguredEffect != "static" || snap.Brightness != 100 || len(snap.SupportedEffects) != 20 {
		t.Fatal(snap)
	}
	for i, e := range snap.SupportedEffects {
		if e.ID != auditedEffects[i] || e.Label == "" {
			t.Fatal(i, e)
		}
	}
	if _, found, err := rt.State.Resolve(d.Serial); err != nil || found {
		t.Fatal("legacy import", err)
	}
	if got := lightingTopology(); len(got) != 1 || got[0].Name != "Logo" || !reflect.DeepEqual(got[0].ColorIndex, []int{0, 2, 4}) {
		t.Fatal(got)
	}
	topology := lightingTopology()
	topology[0].ColorIndex[0] = 99
	if lightingTopology()[0].ColorIndex[0] != 0 {
		t.Fatal("mutable topology")
	}
	if got := composeLightingPacket([]byte{1, 2, 3}); !bytes.Equal(got, []byte{1, 0, 2, 0, 3, 0}) {
		t.Fatal(got)
	}
	for _, effect := range auditedEffects {
		requireLighting(t, d.SetLightingEffect(effect))
		if len(render(t, d)) != 3 {
			t.Fatal("missing dispatch", effect)
		}
	}
	requireLighting(t, d.SetLightingEffect("mouse"))
	snap, _ = d.LightingSnapshot()
	zones := snap.AuthoredZoneEditor.Zones
	if len(zones) != 1 || zones[0].ID != "0" || zones[0].Label != "Logo" || zones[0].ColorHex != "#ffff00" {
		t.Fatal(zones)
	}
	before := render(t, d)
	d.DeviceProfile.RGBProfile = "off"
	*d.DeviceProfile.BrightnessSlider = 0
	d.DeviceProfile.ZoneColors[0].Color.Red = 1
	if !bytes.Equal(before, render(t, d)) {
		t.Fatal("legacy authority")
	}
	oldJSON := inputJSON(t, d)
	legacy, _ := json.Marshal(d.Rgb)
	requireLighting(t, d.SetLightingBrightness(50))
	requireLighting(t, d.SetLightingZoneColor("mouse", "zone", "0", "", rgb.Color{Red: 200, Green: 100, Blue: 40}))
	if got := render(t, d); !bytes.Equal(got, []byte{100, 50, 20}) {
		t.Fatal(got)
	}
	if !bytes.Equal(oldJSON, inputJSON(t, d)) {
		t.Fatal("canonical wrote legacy profile")
	}
	after, _ := json.Marshal(d.Rgb)
	if !bytes.Equal(legacy, after) {
		t.Fatal("canonical wrote legacy rgb")
	}
	if d.SetLightingZoneColor("mouse", "zone", "1", "", rgb.Color{}) == nil {
		t.Fatal("indicator exposed")
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
	return errors.New("settings failure")
}
func (f failingEffects) SetAuthoredZones(string, string, map[string]lightingsettings.Color) error {
	return errors.New("palette failure")
}
func TestWirelessFailureLeavesStateWorkerAndSessions(t *testing.T) {
	d, _ := lightingFixture(t, true)
	before := inputJSON(t, d)
	snap, _ := d.LightingSnapshot()
	native, external, connection := d.nativeGeneration, d.externalGeneration, d.connectionGeneration
	original := d.lightingCanonical.state
	d.lightingCanonical.state = failingState{original}
	if d.SetLightingEffect("mouse") == nil || d.SetLightingBrightness(40) == nil {
		t.Fatal("failure hidden")
	}
	d.lightingCanonical.state = original
	effects := d.lightingCanonical.effects
	d.lightingCanonical.effects = failingEffects{effects}
	if d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 1}) == nil {
		t.Fatal("palette failure hidden")
	}
	p := d.GetRgbProfile("gradient")
	if d.UpdateRgbProfileData("gradient", *p) != 0 {
		t.Fatal("settings failure hidden")
	}
	d.lightingCanonical.effects = effects
	d.lightingProfileWrite = func(string, []byte) error { return errors.New("profile failure") }
	if d.setUserRGBOff(true) == nil || d.SaveMouseDPI(map[int]uint16{1: 1200}) != 0 {
		t.Fatal("profile error hidden")
	}
	after, _ := d.LightingSnapshot()
	if !reflect.DeepEqual(snap, after) || !bytes.Equal(before, inputJSON(t, d)) || native != d.nativeGeneration || external != d.externalGeneration || connection != d.connectionGeneration {
		t.Fatal("failure published")
	}
}

func TestWirelessBrightnessSchedulerRGBOffReconnect(t *testing.T) {
	d, _ := lightingFixture(t, true)
	frames := make(chan []byte, 20)
	d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
	requireLighting(t, d.SetLightingEffect("mouse"))
	before := inputJSON(t, d)
	d.SetConnected(false)
	requireLighting(t, d.SetLightingBrightness(30))
	d.SchedulerBrightness(0)
	requireLighting(t, d.SetLightingBrightness(70))
	if !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("darkness persisted")
	}
	if frameState(t, d).brightness != 0 {
		t.Fatal("not dark")
	}
	requireLighting(t, d.setUserRGBOff(true))
	var saved DeviceProfile
	b, _ := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, json.Unmarshal(b, &saved))
	if !saved.RgbOff {
		t.Fatal("RGB-off not persisted")
	}
	reconnectFixture(d)
	packet := waitFrame(t, frames)
	if !bytes.Equal(packet, make([]byte, 6)) {
		t.Fatal("dark reconnect", packet)
	}
	d.stopLighting()
	d.SchedulerBrightness(1)
	packet = waitFrame(t, frames)
	if !bytes.Equal(packet, []byte{0, 178, 0, 178, 0, 178}) {
		t.Fatal("RGB-off/indicator", packet)
	}
	if d.ProcessSetRgbCluster(true) != 1 {
		t.Fatal("cluster")
	}
	waitFrame(t, frames)
	d.writeColorCluster([]byte{11, 22, 33}, 0)
	packet = waitFrame(t, frames)
	if !bytes.Equal(packet, []byte{11, 178, 22, 178, 33, 178}) {
		t.Fatal("external precedence/scaling", packet)
	}
	if d.ProcessSetRgbCluster(false) != 1 {
		t.Fatal("release")
	}
	waitFrame(t, frames)
	requireLighting(t, d.setUserRGBOff(false))
	packet = waitFrame(t, frames)
	if packet[0] != 178 || packet[2] != 178 {
		t.Fatal("latest desired", packet)
	}
}

// Deterministic receiver-free publication helper. Real Connect is separately
// exercised through a 65-byte report hook and retains its LED setup pacing.
func reconnectFixture(d *Device) {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	d.advanceConnectionLocked()
	d.Connected = true
	d.lightingHardwareReady = true
	d.SniperMode = false
	d.lightingMu.Unlock()
	d.refreshLightingRegistrationsLocked(true, true)
	d.lightingMutation.Unlock()
	d.toggleDPI()
}

func TestWirelessExternalConnectionSessionsAndRegistrations(t *testing.T) {
	for _, owner := range []string{"cluster", "openrgb"} {
		t.Run(owner, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			readyLighting(d)
			var cluster *common.ClusterController
			var open *common.OpenRGBController
			d.lightingClusterRegister = func(c *common.ClusterController) { cluster = c }
			d.lightingClusterRemove = func(string) { cluster = nil }
			d.lightingOpenRGBReconcile = func(_ string, c *common.OpenRGBController) { open = c }
			enable := d.ProcessSetRgbCluster
			if owner == "openrgb" {
				enable = d.ProcessSetOpenRgbIntegration
			}
			if enable(true) != 1 {
				t.Fatal("enable")
			}
			d.stopLighting()
			d.refreshLightingRegistrations(true, true)
			if open == nil || len(open.Colors) != 3 || len(open.Zones) != 1 || open.Zones[0].Name != "Logo" {
				t.Fatal(open)
			}
			callback := open.WriteColorEx
			if owner == "cluster" {
				if cluster == nil || cluster.LedChannels != 1 {
					t.Fatal(cluster)
				}
				callback = cluster.WriteColorEx
			}
			oldConnection := d.connectionGeneration
			oldGeneration := d.externalGeneration
			d.enqueueOpenRGB([]byte{1, 2, 3}, oldGeneration)
			d.SetConnected(false)
			if cluster != nil || open != nil || d.connectionGeneration == oldConnection || len(d.externalQueue) != 0 {
				t.Fatal("disconnect failed to retire")
			}
			requireLighting(t, d.SetLightingBrightness(40))
			reconnectFixture(d)
			d.stopLighting()
			frames := make(chan []byte, 5)
			d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
			callback([]byte{7, 8, 9}, 0)
			d.applyLightingPacket(composeLightingPacket([]byte{7, 8, 9}), owner, oldGeneration, nil, oldConnection)
			if len(frames) != 0 || len(d.externalQueue) != 0 {
				t.Fatal("old connection accepted")
			}
			current := open.WriteColorEx
			if owner == "cluster" {
				current = cluster.WriteColorEx
			}
			current([]byte{11, 22, 33}, 0)
			if owner == "openrgb" {
				frame := receiveWithin(t, d.externalQueue)
				d.applyLightingPacket(composeLightingPacket(frame.data), owner, frame.generation, nil, frame.connection)
			}
			if got := waitFrame(t, frames); !bytes.Equal(got, []byte{11, 102, 22, 102, 33, 102}) {
				t.Fatal(got)
			}
			old := current
			d.RebindReceiver(&common.Slipstream{}, 12)
			reconnectFixture(d)
			d.stopLighting()
			old([]byte{7, 8, 9}, 0)
			if len(frames) != 0 || len(d.externalQueue) != 0 {
				t.Fatal("old binding accepted")
			}
			d.stopCanonicalLighting(false)
			if open != nil || cluster != nil {
				t.Fatal("registrations survived stop")
			}
			d.Connect()
			if d.Connected {
				t.Fatal("stopped peripheral resurrected")
			}
		})
	}
}

func TestWirelessLatestPendingAndThousandFrameBurst(t *testing.T) {
	d, _ := lightingFixture(t, true)
	if d.ProcessSetOpenRgbIntegration(true) != 1 {
		t.Fatal("enable")
	}
	readyLighting(d)
	generation := d.externalGeneration
	if cap(d.externalQueue) != 1 {
		t.Fatal("capacity")
	}
	d.enqueueOpenRGB([]byte{1, 2, 3}, generation)
	d.enqueueOpenRGB([]byte{4, 5, 6}, generation)
	queued := receiveWithin(t, d.externalQueue)
	if !bytes.Equal(queued.data, []byte{4, 5, 6}) {
		t.Fatal(queued)
	}
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	frames := make(chan []byte, 4)
	d.lightingWrite = func(p []byte) {
		if p[0] == 1 {
			close(started)
			waitDone(t, release)
		}
		sendWithin(t, frames, p)
	}
	d.enqueueOpenRGB([]byte{1, 2, 3}, generation)
	d.startLightingQueue()
	waitDone(t, started)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			d.enqueueOpenRGB([]byte{2, byte(i), 3}, generation)
		}
		data := []byte{251, 252, 253}
		d.enqueueOpenRGB(data, generation)
		data[0] = 99
		close(done)
	}()
	waitDone(t, done)
	once.Do(func() { close(release) })
	waitFrame(t, frames)
	last := waitFrame(t, frames)
	if !bytes.Equal(last, []byte{251, 255, 252, 255, 253, 255}) {
		t.Fatal("final lost", last)
	}
}

func TestWirelessSleepCancelWhileWriterWaitsForDeviceLock(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	requireLighting(t, d.SetLightingEffect("rainbow"))
	d.stopLighting()
	entered := make(chan struct{})
	d.lightingBeforeApply = func(string) {
		select {
		case <-entered:
		default:
			close(entered)
		}
	}
	d.deviceLock.Lock()
	var releaseLock sync.Once
	unlock := func() { releaseLock.Do(d.deviceLock.Unlock) }
	defer unlock()
	d.setDeviceColor()
	waitDone(t, entered)
	d.lightingMu.Lock()
	worker := d.lightingWorker
	d.lightingMu.Unlock()
	done := make(chan struct{})
	go func() { d.SetSleepMode(); close(done) }()
	waitDone(t, done)
	// This is the prior lock cycle: cancellation must finish BEFORE releasing
	// deviceLock, while the renderer is blocked trying to obtain that lock.
	unlock()
	waitDone(t, worker.done)
	d.lightingMu.Lock()
	survived := d.lightingWorker != nil || d.Connected
	d.lightingMu.Unlock()
	if survived {
		t.Fatal("worker survived sleep")
	}
}

func TestWirelessReceiverLockFinalGateAndDynamicEndpoint(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	if d.ProcessSetRgbCluster(true) != 1 {
		t.Fatal("enable")
	}
	receiver := d.dev
	var writes atomic.Int32
	d.lightingWrite = nil
	d.lightingReportWrite = func(report []byte) (int, error) {
		writes.Add(1)
		if len(report) != 65 || report[0] != 0 || report[1] != d.Endpoint || !bytes.Equal(report[2:8], []byte{6, 0, 6, 0, 0, 0}) {
			t.Errorf("report %v", report)
		}
		return len(report), nil
	}
	entered := make(chan struct{})
	d.lightingBeforeApply = func(string) { close(entered) }
	receiver.Mutex.Lock()
	var releaseLock sync.Once
	unlock := func() { releaseLock.Do(receiver.Mutex.Unlock) }
	defer unlock()
	done := make(chan struct{})
	go func() { d.writeColorCluster([]byte{1, 2, 3}, 0); close(done) }()
	waitDone(t, entered)
	d.SetConnected(false)
	unlock()
	waitDone(t, done)
	if writes.Load() != 0 {
		t.Fatal("stale receiver write")
	}
	d.lightingBeforeApply = nil
	d.RebindReceiver(&common.Slipstream{}, 12)
	reconnectFixture(d)
	d.writeColorCluster([]byte{1, 2, 3}, 0)
	if writes.Load() != 1 {
		t.Fatal("new binding no output")
	}
}

func TestWirelessDPIAndSniperReconnectAndDefaults(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	var commands [][]byte
	var values []uint16
	d.lightingDPIWrite = func(c, b []byte) error {
		if bytes.Equal(c, cmdSetDpi[0]) || bytes.Equal(c, cmdSetDpi[1]) {
			commands = append(commands, c)
			values = append(values, binary.LittleEndian.Uint16(b))
		}
		return nil
	}
	if d.SaveMouseDPI(map[int]uint16{1: 26000}) != 1 {
		t.Fatal("max")
	}
	if len(values) != 2 || values[0] != 26000 || values[1] != 26000 || !bytes.Equal(commands[0], []byte{1, 0x21, 0}) || !bytes.Equal(commands[1], []byte{1, 0x22, 0}) {
		t.Fatal(commands, values)
	}
	for _, v := range []uint16{99, 26001} {
		if d.SaveMouseDPI(map[int]uint16{1: v}) != 0 {
			t.Fatal("range", v)
		}
	}
	if d.SetMouseSniperMode(true) != 1 {
		t.Fatal("sniper")
	}
	if values[len(values)-1] != 200 {
		t.Fatal(values)
	}
	d.lightingMu.Lock()
	color, _ := d.indicatorColorLocked()
	d.lightingMu.Unlock()
	if color.Red != 255 || color.Green != 255 || color.Blue != 0 {
		t.Fatal("yellow", color)
	}
	d.SetConnected(false)
	d.SetMouseSniperMode(false)
	reconnectFixture(d)
	d.stopLighting()
	if d.SniperMode || values[len(values)-1] != 26000 {
		t.Fatal("stale sniper", values)
	}
	d.lightingMu.Lock()
	color, _ = d.indicatorColorLocked()
	d.lightingMu.Unlock()
	if color.Red != 255 || color.Green != 255 || color.Blue != 255 {
		t.Fatal("regular color not restored", color)
	}
	original := pwd
	pwd = t.TempDir()
	defer func() { pwd = original }()
	requireLighting(t, os.MkdirAll(filepath.Join(pwd, "database", "profiles"), 0700))
	fresh := &Device{Serial: "wirelessdefault", Product: "M65 RGB ULTRA"}
	fresh.saveDeviceProfile()
	p := fresh.DeviceProfile
	if p == nil || len(p.ZoneColors) != 1 || !reflect.DeepEqual(p.ZoneColors[0].ColorIndex, []int{0, 2, 4}) {
		t.Fatal(p)
	}
	for id, v := range []uint16{400, 800, 1200, 1600, 3000, 200} {
		stage := p.Profiles[id]
		if stage.Value != v || !reflect.DeepEqual(stage.ColorIndex, map[int][]int{0: {1, 3, 5}}) {
			t.Fatal(id, stage)
		}
	}
	sniper := p.Profiles[5]
	delete(p.Profiles, 5)
	fresh.upgradeDpiProfiles()
	if !reflect.DeepEqual(sniper, p.Profiles[5]) {
		t.Fatal("upgraded defaults")
	}
}

func TestWirelessLiftSwitchSleepValidation(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	var count int
	d.lightingDPIWrite = func([]byte, []byte) error { count++; return nil }
	for _, v := range []int{2, 3, 4, 6, 7} {
		if d.UpdateLiftHeight(v) != 1 {
			t.Fatal(v)
		}
		if _, ok := d.PerformanceSnapshot(); !ok {
			t.Fatal("snapshot", v)
		}
	}
	before := inputJSON(t, d)
	writes := count
	for _, v := range []int{-1, 0, 1, 5, 8} {
		if d.UpdateLiftHeight(v) != 0 {
			t.Fatal("invalid lift", v)
		}
	}
	if d.UpdateAngleSnapping(2) != 0 || d.UpdateButtonOptimization(-1) != 0 || d.UpdateSleepTimer(-1) != 0 {
		t.Fatal("invalid setting")
	}
	if !bytes.Equal(before, inputJSON(t, d)) || count != writes {
		t.Fatal("invalid mutated/programmed")
	}
	if _, ok := any(d).(interface{ UpdatePollingRate(int) uint8 }); ok {
		t.Fatal("child polling invented")
	}
}

func TestWirelessMissingIndicatorReleasesLocksAndWorkerSurvives(t *testing.T) {
	d, _ := lightingFixture(t, true)
	if d.ProcessSetOpenRgbIntegration(true) != 1 {
		t.Fatal("enable")
	}
	readyLighting(d)
	d.lightingMu.Lock()
	stage := d.DeviceProfile.Profiles[1]
	d.DeviceProfile.Profiles[1] = DPIProfile{}
	epoch, generation := d.connectionGeneration, d.externalGeneration
	d.lightingMu.Unlock()
	processed := make(chan lightingApplyResult, 4)
	frames := make(chan []byte, 4)
	d.lightingAfterApply = func(source string, result lightingApplyResult) {
		if source == "openrgb" {
			sendWithin(t, processed, result)
		}
	}
	d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
	d.startLightingQueue()
	d.lightingMu.Lock()
	done := d.externalDone
	d.lightingMu.Unlock()
	d.writeColorEx([]byte{1, 2, 3}, 0)
	if receiveWithin(t, processed) != lightingRetry {
		t.Fatal("missing indicator path not processed")
	}
	// The SAME consumer must remain alive and handle a later valid packet.
	d.lightingMu.Lock()
	d.DeviceProfile.Profiles[1] = stage
	d.lightingMu.Unlock()
	d.writeColorEx([]byte{11, 22, 33}, 0)
	if receiveWithin(t, processed) != lightingApplied {
		t.Fatal("same worker did not apply valid frame")
	}
	if !bytes.Equal(waitFrame(t, frames), []byte{11, 255, 22, 255, 33, 255}) {
		t.Fatal("wrong follow-up frame")
	}
	d.lightingMu.Lock()
	same := d.externalDone == done && d.connectionGeneration == epoch && d.externalGeneration == generation
	d.lightingMu.Unlock()
	select {
	case <-done:
		t.Fatal("queue worker exited")
	default:
	}
	if !same {
		t.Fatal("consumer/session replaced")
	}
}

func TestWirelessConcurrentDesiredProfilesSessionsAndInputs(t *testing.T) {
	d, _ := lightingFixture(t, true)
	previous := pwd
	pwd = t.TempDir()
	defer func() { pwd = previous }()
	for _, dir := range []string{"profiles", "key-assignments"} {
		requireLighting(t, os.MkdirAll(filepath.Join(pwd, "database", dir), 0700))
	}
	d.KeyAssignment = defaultWirelessAssignments()
	d.keyAssignmentFile = "/database/key-assignments/m65RgbUltraW.json"

	readyLighting(d)
	d.startLightingQueue()
	next := cloneLightingProfile(*d.DeviceProfile)
	next.Path = filepath.Join(filepath.Dir(next.Path), "next.json")
	next.Active = false
	d.UserProfiles["next"] = &next
	var wg sync.WaitGroup
	actions := []func(){
		func() { _ = d.SetLightingEffect("rainbow"); _ = d.SetLightingBrightness(40); d.LightingSnapshot() },
		func() { d.ProcessNewGradientColor("gradient"); d.ProcessDeleteGradientColor("gradient") },
		func() {
			d.ProcessSetRgbCluster(true)
			d.writeColorCluster([]byte{1, 2, 3}, 0)
			d.ProcessSetRgbCluster(false)
		},
		func() {
			d.ProcessSetOpenRgbIntegration(true)
			d.writeColorEx([]byte{4, 5, 6}, 0)
			d.ProcessSetOpenRgbIntegration(false)
		},
		func() { d.ChangeDeviceProfile("next"); d.ChangeDeviceProfile("default") },
		func() {
			d.SchedulerBrightness(0)
			d.ControlDeviceRgb(true)
			d.ControlDeviceRgb(false)
			d.SchedulerBrightness(1)
		},
		func() {
			d.SelectMouseDPIStage(2)
			d.SetMouseSniperMode(true)
			d.SetMouseSniperMode(false)
			d.DPISnapshot()
			d.PerformanceSnapshot()
			d.DeviceProfileSnapshot()
		},
		func() { d.SetConnected(false); reconnectFixture(d) },
		func() { d.RebindReceiver(&common.Slipstream{}, 11); reconnectFixture(d) },
		func() { d.SaveUserProfile("saved"); d.DeleteDeviceProfile("saved") },
	}
	for _, f := range actions {
		wg.Add(1)
		go func(f func()) {
			defer wg.Done()
			for i := 0; i < 3; i++ {
				f()
			}
		}(f)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	waitDone(t, done)
	d.StopDirty()
}

func TestWirelessFailedAttachmentIndependentFallback(t *testing.T) {
	d, rt := lightingFixture(t, false)
	d.ZoneAmount = 2
	if d.attachLightingSource(rt) == nil {
		t.Fatal("wrong topology accepted")
	}
	if _, ok := d.LightingSnapshot(); ok {
		t.Fatal("partial authority")
	}
	if d.GetRgbProfile("static").StartColor.Red != 9 {
		t.Fatal("fallback depends on canonical")
	}
	d.ZoneAmount = 1
	requireLighting(t, d.attachLightingSource(rt))
	snapshot, ok := d.LightingSnapshot()
	if !ok || snapshot.ConfiguredEffect != "static" {
		t.Fatal(snapshot)
	}
}

func TestWirelessConnectRunsOriginalReceiverSetupAndReplaysOff(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.SetConnected(false)
	requireLighting(t, d.SetLightingEffect("mouse"))
	requireLighting(t, d.SetLightingBrightness(60))
	requireLighting(t, d.setUserRGBOff(true))
	d.lightingWrite = nil
	d.lightingDPIWrite = nil
	var reports [][]byte
	applied := make(chan struct{}, 4)
	var mu sync.Mutex
	d.lightingReportWrite = func(report []byte) (int, error) {
		mu.Lock()
		reports = append(reports, append([]byte(nil), report...))
		if bytes.Equal(report[2:4], cmdWriteColor) {
			sendWithin(t, applied, struct{}{})
		}
		mu.Unlock()
		return len(report), nil
	}
	d.Connect()
	waitDone(t, applied)
	d.stopLighting()
	d.deviceLock.Lock()
	d.deviceLock.Unlock()
	d.lightingMu.Lock()
	ready, connected, epoch := d.lightingHardwareReady, d.Connected, d.connectionGeneration
	d.lightingMu.Unlock()
	if !ready || !connected {
		t.Fatal("setup did not publish readiness")
	}
	mu.Lock()
	snapshot := append([][]byte(nil), reports...)
	mu.Unlock()
	haveMode, haveLed, haveX, haveY, haveBlack := false, false, false, false, false
	for _, r := range snapshot {
		if len(r) != 65 || r[0] != 0 || r[1] != 9 {
			t.Fatal("receiver report", r)
		}
		switch {
		case bytes.Equal(r[2:6], cmdSoftwareMode):
			haveMode = true
		case bytes.Equal(r[2:5], cmdOpenEndpoint):
			haveLed = true
		case bytes.Equal(r[2:5], cmdSetDpi[0]):
			haveX = true
			if binary.LittleEndian.Uint16(r[5:7]) != 800 {
				t.Fatal("regular X")
			}
		case bytes.Equal(r[2:5], cmdSetDpi[1]):
			haveY = true
			if binary.LittleEndian.Uint16(r[5:7]) != 800 {
				t.Fatal("regular Y")
			}
		case bytes.Equal(r[2:4], cmdWriteColor):
			haveBlack = true
			if !bytes.Equal(r[4:14], []byte{6, 0, 0, 0, 0, 153, 0, 153, 0, 153}) {
				t.Fatal("off replay/planar frame", r[4:14])
			}
		}
	}
	if !haveMode || !haveLed || !haveX || !haveY || !haveBlack {
		t.Fatal("setup commands missing", haveMode, haveLed, haveX, haveY, haveBlack)
	}
	d.Connect()
	d.lightingMu.Lock()
	duplicate := epoch != d.connectionGeneration
	d.lightingMu.Unlock()
	if duplicate {
		t.Fatal("duplicate connect established another session")
	}
	d.SetConnected(false)
	d.RebindReceiver(&common.Slipstream{}, 13)
	type secondFrame struct {
		generation uint64
		endpoint   byte
		packet     []byte
	}
	fresh := make(chan secondFrame, 4)
	d.lightingReportWrite = func(report []byte) (int, error) {
		if report[1] != 13 {
			t.Errorf("old endpoint %d after rebind", report[1])
		}
		if bytes.Equal(report[2:4], cmdWriteColor) {
			d.lightingMu.Lock()
			generation := d.connectionGeneration
			d.lightingMu.Unlock()
			sendWithin(t, fresh, secondFrame{generation, report[1], append([]byte(nil), report[4:14]...)})
		}
		return len(report), nil
	}
	d.Connect()
	d.lightingMu.Lock()
	secondReady, secondConnected, secondEpoch := d.lightingHardwareReady, d.Connected, d.connectionGeneration
	d.lightingMu.Unlock()
	if !secondReady || !secondConnected || secondEpoch <= epoch {
		t.Fatal("second connection not ready/current", secondReady, secondConnected, secondEpoch, epoch)
	}
	frame := receiveWithin(t, fresh)
	if frame.generation != secondEpoch || frame.endpoint != 13 || !bytes.Equal(frame.packet, []byte{6, 0, 0, 0, 0, 153, 0, 153, 0, 153}) {
		t.Fatal("missing/wrong second-session RGB-off output", frame)
	}
	d.stopLighting()
	d.deviceLock.Lock()
	d.deviceLock.Unlock()
	if !d.DeviceProfile.RgbOff {
		t.Fatal("lost persisted off")
	}
}

func TestWirelessConnectFailureAndInFlightDisconnect(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.SetConnected(false)
	d.lightingReportWrite = func(report []byte) (int, error) {
		if bytes.Equal(report[2:5], cmdOpenEndpoint) {
			return 0, errors.New("LED endpoint failure")
		}
		return len(report), nil
	}
	d.Connect()
	if d.Connected || d.lightingHardwareReady {
		t.Fatal("failed setup published output")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	d.lightingReportWrite = func(report []byte) (int, error) {
		if report[2] == cmdHeartbeat[0] {
			close(entered)
			waitDone(t, release)
		}
		return len(report), nil
	}
	done := make(chan struct{})
	go func() { d.Connect(); close(done) }()
	waitDone(t, entered)
	stopped := make(chan struct{})
	go func() { d.SetConnected(false); close(stopped) }()
	waitDone(t, stopped)
	once.Do(func() { close(release) })
	waitDone(t, done)
	if d.Connected || d.lightingHardwareReady {
		t.Fatal("retired setup republished readiness")
	}
}

func TestWirelessNativeWorkerIdentityAndConnectionGate(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	requireLighting(t, d.SetLightingEffect("rainbow"))
	d.stopLighting()
	d.deviceLock.Lock()
	var releaseLock sync.Once
	unlock := func() { releaseLock.Do(d.deviceLock.Unlock) }
	defer unlock()
	entered := make(chan struct{}, 8)
	d.lightingBeforeApply = func(string) { sendWithin(t, entered, struct{}{}) }
	d.setDeviceColor()
	waitDone(t, entered)
	d.lightingMu.Lock()
	old := d.lightingWorker
	d.lightingMu.Unlock()
	d.SetConnected(false)
	reconnectFixtureWithoutOutput(d)
	d.setDeviceColor()
	waitDone(t, entered)
	d.lightingMu.Lock()
	current := d.lightingWorker
	d.lightingMu.Unlock()
	d.retireLightingWorker(old)
	d.lightingMu.Lock()
	retired := current == nil || d.lightingWorker != current
	d.lightingMu.Unlock()
	if retired {
		t.Fatal("old worker retired current")
	}
	unlock()
	waitDone(t, old.done)
	d.stopLighting()
	waitDone(t, current.done)
}
func reconnectFixtureWithoutOutput(d *Device) {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	d.advanceConnectionLocked()
	d.Connected, d.lightingHardwareReady = true, true
	d.SniperMode = false
	d.lightingMu.Unlock()
	d.refreshLightingRegistrationsLocked(true, true)
	d.lightingMutation.Unlock()
}

func TestWirelessProfileRGBOffReplayAndOfflineSwitchRejection(t *testing.T) {
	d, _ := lightingFixture(t, true)
	requireLighting(t, d.SetLightingEffect("mouse"))
	requireLighting(t, d.SetLightingBrightness(70))
	next := cloneLightingProfile(*d.DeviceProfile)
	next.RgbOff = true
	next.Active = false
	next.Path = filepath.Join(filepath.Dir(next.Path), "off.json")
	d.UserProfiles["off"] = &next
	frames := make(chan []byte, 10)
	d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
	// Install the write seam before any renderer can access it.
	readyLighting(d)
	if d.ChangeDeviceProfile("off") != 1 {
		t.Fatal("switch")
	}
	if p := waitFrame(t, frames); !bytes.Equal(p, []byte{0, 178, 0, 178, 0, 178}) {
		t.Fatal(p)
	}
	snap, _ := d.LightingSnapshot()
	if snap.ConfiguredEffect != "mouse" || snap.Brightness != 70 {
		t.Fatal("profile imported legacy Lighting")
	}
	d.SetConnected(false)
	before := inputJSON(t, d)
	if d.ChangeDeviceProfile("default") != 0 || !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("offline switch semantics")
	}
	requireLighting(t, d.SetLightingBrightness(40))
	d.SchedulerBrightness(0)
	reconnectFixture(d)
	if p := waitFrame(t, frames); !bytes.Equal(p, make([]byte, 6)) {
		t.Fatal("scheduler profile replay", p)
	}
}

func TestWirelessExternalOwnerAndProfileTransitionsRejectDequeuedWork(t *testing.T) {
	for _, owner := range []string{"cluster", "openrgb"} {
		for _, transition := range []string{"owner", "profile"} {
			t.Run(owner+"/"+transition, func(t *testing.T) {
				d, _ := lightingFixture(t, true)
				var callback func([]byte, int)
				d.lightingClusterRegister = func(c *common.ClusterController) {
					if owner == "cluster" {
						callback = c.WriteColorEx
					}
				}
				d.lightingOpenRGBReconcile = func(_ string, c *common.OpenRGBController) {
					if owner == "openrgb" && c != nil {
						callback = c.WriteColorEx
					}
				}
				enable := d.ProcessSetRgbCluster
				if owner == "openrgb" {
					enable = d.ProcessSetOpenRgbIntegration
				}
				if enable(true) != 1 {
					t.Fatal("enable")
				}
				readyLighting(d)
				d.refreshLightingRegistrations(true, true)
				old := callback
				generation, connection := d.externalGeneration, d.connectionGeneration
				if owner == "openrgb" {
					old([]byte{1, 2, 3}, 0)
					if len(d.externalQueue) != 1 {
						t.Fatal("enqueue")
					}
					receiveWithin(t, d.externalQueue)
					old([]byte{4, 5, 6}, 0)
				}
				if transition == "owner" {
					if enable(false) != 1 || enable(true) != 1 {
						t.Fatal("owner transition")
					}
				} else {
					next := cloneLightingProfile(*d.DeviceProfile)
					next.Active = false
					next.Path = filepath.Join(filepath.Dir(next.Path), "next.json")
					d.UserProfiles["next"] = &next
					if d.ChangeDeviceProfile("next") != 1 {
						t.Fatal("profile transition")
					}
				}
				d.stopLighting()
				d.deviceLock.Lock()
				d.deviceLock.Unlock()
				frames := make(chan []byte, 8)
				d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
				old([]byte{1, 2, 3}, 0)
				d.applyLightingPacket(composeLightingPacket([]byte{4, 5, 6}), owner, generation, nil, connection)
				if len(frames) != 0 || len(d.externalQueue) != 0 {
					t.Fatal("stale callback/dequeued frame applied")
				}
				callback([]byte{11, 22, 33}, 0)
				if owner == "openrgb" {
					f := receiveWithin(t, d.externalQueue)
					d.applyLightingPacket(composeLightingPacket(f.data), owner, f.generation, nil, f.connection)
				}
				if got := waitFrame(t, frames); !bytes.Equal(got, []byte{11, 255, 22, 255, 33, 255}) {
					t.Fatal(got)
				}
			})
		}
	}
}

func TestWirelessSparseGradientIdentityDoesNotProduceEmptyFrame(t *testing.T) {
	d, _ := lightingFixture(t, true)
	p := d.GetRgbProfile("gradient")
	p.Gradients = map[int]rgb.Color{3: {Red: 200, Position: 0, Brightness: 1}, 8: {Blue: 200, Position: 1, Brightness: 1}}
	if d.UpdateRgbProfileData("gradient", *p) != 1 {
		t.Fatal("sparse key identity rejected")
	}
	requireLighting(t, d.SetLightingEffect("gradient"))
	if len(render(t, d)) != 3 {
		t.Fatal("sparse identity produced empty renderer output")
	}
	result, key := d.ProcessNewGradientColor("gradient")
	if result != 1 || key != 9 {
		t.Fatal("max-key identity", result, key)
	}
	p = d.GetRgbProfile("gradient")
	p.Gradients[3] = rgb.Color{}
	if d.GetRgbProfile("gradient").Gradients[3].Red != 200 {
		t.Fatal("getter aliases canonical gradient")
	}
	result, key = d.ProcessDeleteGradientColor("gradient")
	if result != 1 || key != 9 {
		t.Fatal("delete identity", result, key)
	}
}

func receiveWithin[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("channel receive timed out")
		var zero T
		return zero
	}
}
func sendWithin[T any](t *testing.T, ch chan<- T, value T) {
	t.Helper()
	select {
	case ch <- value:
	case <-time.After(3 * time.Second):
		t.Error("channel send timed out")
	}
}
