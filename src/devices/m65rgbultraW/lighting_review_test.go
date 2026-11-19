package m65rgbultraW

import (
	"LumenForge/src/common"
	"LumenForge/src/rgb"
	"bytes"
	"encoding/binary"
	"errors"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConnectCompletionConsumesPendingWakeAtomically(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.SetConnected(false)
	var heartbeat, completion, active, maxActive atomic.Int32
	paused, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	d.lightingReportWrite = func(report []byte) (int, error) {
		running := active.Add(1)
		defer active.Add(-1)
		if running > maxActive.Load() {
			maxActive.Store(running)
		}
		if report[2] == cmdHeartbeat[0] && heartbeat.Add(1) == 1 {
			d.SetConnected(false)
			return 0, errors.New("old attempt cancelled")
		}
		return len(report), nil
	}
	d.connectBeforeComplete = func() {
		if completion.Add(1) == 1 {
			close(paused)
			waitDone(t, release)
		}
	}
	go func() { d.Connect(); close(finished) }()
	waitDone(t, paused)
	d.Connect() // arrives at the completion boundary while the first attempt owns setup
	once.Do(func() { close(release) })
	waitDone(t, finished)
	d.stopLighting()
	d.lightingMu.Lock()
	connected, pending, connecting := d.Connected, d.connectPending, d.connecting
	d.lightingMu.Unlock()
	if !connected || pending || connecting || completion.Load() != 2 || heartbeat.Load() != 2 || maxActive.Load() != 1 {
		t.Fatal("pending wake lost or duplicate setup", connected, pending, connecting, completion.Load(), heartbeat.Load(), maxActive.Load())
	}
}

func TestSleepCannotCaptureNewWakeSession(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	paused, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	wakeEntered, wakeRelease, wakeDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var sleepRelease, wakeUnlock sync.Once
	defer sleepRelease.Do(func() { close(release) })
	defer wakeUnlock.Do(func() { close(wakeRelease) })
	captured := make(chan connectionSession, 1)
	type sleepResult struct {
		session connectionSession
		err     error
	}
	completed := make(chan sleepResult, 1)
	d.sleepBeforeCommand = func(session connectionSession) { sendWithin(t, captured, session); close(paused); waitDone(t, release) }
	d.sleepAfterCommand = func(session connectionSession, err error) { sendWithin(t, completed, sleepResult{session, err}) }
	var sleep atomic.Int32
	d.lightingReportWrite = func(report []byte) (int, error) {
		if report[2] == cmdHeartbeat[0] {
			close(wakeEntered)
			waitDone(t, wakeRelease)
		}
		if bytes.Equal(report[2:6], cmdSleepMode) {
			sleep.Add(1)
		}
		return len(report), nil
	}
	go func() { d.SetSleepMode(); close(finished) }()
	waitDone(t, paused)
	d.lightingMu.Lock()
	retired := d.sessionLocked()
	d.lightingMu.Unlock()
	actual := receiveWithin(t, captured)
	if actual.generation != retired.generation || actual.receiver != retired.receiver || actual.endpoint != retired.endpoint {
		t.Fatal("actual sleep captured wrong retired token")
	}
	go func() { d.Connect(); close(wakeDone) }()
	waitDone(t, wakeEntered)
	// New wake is still in setup, not Connected yet: this is precisely the
	// vulnerable interval in which a newly captured sleep token could pass.
	sleepRelease.Do(func() { close(release) })
	waitDone(t, finished)
	result := receiveWithin(t, completed)
	if result.session.generation != retired.generation || !errors.Is(result.err, errLightingSessionRetired) {
		t.Fatal("actual sleep goroutine was not retired", result.err)
	}
	wakeUnlock.Do(func() { close(wakeRelease) })
	waitDone(t, wakeDone)
	d.stopLighting()
	d.lightingMu.Lock()
	live := d.Connected && d.connectionGeneration != retired.generation
	d.lightingMu.Unlock()
	if !live || sleep.Load() != 0 {
		t.Fatal("sleep interrupted new wake")
	}
}

func TestDPITransactionIgnoresLightingOwnershipRevision(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	var calls [][]byte
	d.lightingDPIWrite = func(command, data []byte) error {
		calls = append(calls, append([]byte(nil), command...))
		if bytes.Equal(command, cmdSetDpi[0]) {
			if d.ProcessSetOpenRgbIntegration(true) != 1 {
				t.Error("owner transition")
			}
		}
		return nil
	}
	d.programCanonicalDPI()
	if len(calls) != 2 || !bytes.Equal(calls[1], cmdSetDpi[1]) {
		t.Fatal("Y rejected after owner change", calls)
	}
}

func TestDPITransactionRejectsConnectionAndInputReplacement(t *testing.T) {
	for _, transition := range []string{"connection", "input"} {
		t.Run(transition, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			readyLighting(d)
			writes := 0
			d.lightingDPIWrite = func(command, data []byte) error {
				writes++
				if bytes.Equal(command, cmdSetDpi[0]) {
					if transition == "connection" {
						d.SetConnected(false)
						reconnectFixtureWithoutOutput(d)
					} else {
						d.mutateInputProfile(false, func(p *DeviceProfile) error { p.Profile = 2; return nil })
					}
				}
				return nil
			}
			d.programCanonicalDPI()
			if writes != 1 {
				t.Fatal("old input transaction survived", writes)
			}
		})
	}
}

func TestDPITransactionRejectsOldProfileBetweenAxes(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	next := cloneLightingProfile(*d.DeviceProfile)
	next.Active = false
	next.Path = filepath.Join(filepath.Dir(next.Path), "next.json")
	p := next.Profiles[1]
	p.Value = 1600
	next.Profiles[1] = p
	d.UserProfiles["next"] = &next
	first, release, programmed, switched := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	values := make(chan uint16, 8)
	d.lightingDPIWrite = func(command, data []byte) error {
		if bytes.Equal(command, cmdSetDpi[0]) || bytes.Equal(command, cmdSetDpi[1]) {
			value := binary.LittleEndian.Uint16(data)
			sendWithin(t, values, value)
			if value == 800 && bytes.Equal(command, cmdSetDpi[0]) {
				close(first)
				waitDone(t, release)
			}
		}
		return nil
	}
	go func() { d.programCanonicalDPI(); close(programmed) }()
	waitDone(t, first)
	go func() { d.ChangeDeviceProfile("next"); close(switched) }()
	waitCondition(t, func() bool {
		d.lightingMu.Lock()
		defer d.lightingMu.Unlock()
		return d.DeviceProfile == d.UserProfiles["next"]
	})
	once.Do(func() { close(release) })
	waitDone(t, programmed)
	waitDone(t, switched)
	got := []uint16{receiveWithin(t, values), receiveWithin(t, values), receiveWithin(t, values)}
	if !reflect.DeepEqual(got, []uint16{800, 1600, 1600}) || len(values) != 0 {
		t.Fatal("stale profile Y programmed", got)
	}
}

func TestHyphenatedProfilesAndHistoricalPathReload(t *testing.T) {
	d := profileSaveFixture(t, "default")
	for _, name := range []string{"Work", "Work-Night", "Work-Night-Alt", "Z-preserved"} {
		if d.SaveUserProfile(name) != 1 {
			t.Fatal(name)
		}
	}
	legacy := cloneLightingProfile(*d.DeviceProfile)
	legacy.ProfileName = ""
	legacy.Active = false
	legacy.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-Z-preserved-legacy.json")
	// The historical fixture is a file with the literal Z-preserved suffix. Use
	// a separate serial so a new legitimate Z-preserved name is tested separately.
	legacy.Serial = "legacy"
	legacy.Path = filepath.Join(pwd, "database", "profiles", "legacy-Z-preserved.json")
	requireLighting(t, d.persistLightingProfile(legacy))
	if d.ChangeDeviceProfile("Work-Night") != 1 {
		t.Fatal("activate")
	}
	d.stopLighting()
	d.loadDeviceProfiles()
	for _, name := range []string{"default", "Work", "Work-Night", "Work-Night-Alt", "Z-preserved"} {
		if d.UserProfiles[name] == nil {
			t.Fatal("lost full name", name, d.ProfileOrder)
		}
	}
	if d.DeviceProfile != d.UserProfiles["Work-Night"] || !d.DeviceProfile.Active {
		t.Fatal("active marker lost")
	}
	old := &Device{Serial: "legacy"}
	old.loadDeviceProfiles()
	if old.UserProfiles["Z-preserved"] == nil || old.UserProfiles["Z"] != nil {
		t.Fatal("unnamed historical suffix must remain literal", old.ProfileOrder)
	}
}

func TestAssignmentEndpointClosesAfterPayloadFailure(t *testing.T) {
	for _, reconnect := range []bool{false, true} {
		t.Run(map[bool]string{false: "same", true: "replacement"}[reconnect], func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			readyLighting(d)
			d.KeyAssignment = defaultWirelessAssignments()
			payloadFailure := errors.New("payload write failed")
			closed := 0
			d.lightingDPIWrite = func(command, data []byte) error {
				if bytes.Equal(command, cmdWrite) {
					if reconnect {
						d.RebindReceiver(&common.Slipstream{}, 12)
						reconnectFixtureWithoutOutput(d)
					}
					return payloadFailure
				}
				if bytes.Equal(command, cmdCloseEndpoint) {
					closed++
				}
				return nil
			}
			if err := d.programCanonicalAssignments(); !errors.Is(err, payloadFailure) {
				t.Fatal("primary payload error lost", err)
			}
			if (!reconnect && closed != 1) || (reconnect && closed != 0) {
				t.Fatal("wrong cleanup session", closed)
			}
		})
	}
}

func TestOneShotRetriesTransientWriteAndCancels(t *testing.T) {
	for _, effect := range []string{"static", "mouse", "off-override"} {
		t.Run(effect, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			if effect == "mouse" {
				requireLighting(t, d.SetLightingEffect("mouse"))
			}
			if effect == "off-override" {
				requireLighting(t, d.setUserRGBOff(true))
			}
			readyLighting(d)
			d.lightingWrite = nil
			var writes atomic.Int32
			var workersMu sync.Mutex
			var workers []*lightingWorker
			d.lightingReportWrite = func(report []byte) (int, error) {
				d.lightingMu.Lock()
				worker := d.lightingWorker
				d.lightingMu.Unlock()
				workersMu.Lock()
				workers = append(workers, worker)
				workersMu.Unlock()
				if writes.Add(1) == 1 {
					return 0, errors.New("temporary receiver error")
				}
				return len(report), nil
			}
			applied := make(chan struct{}, 2)
			d.lightingAfterApply = func(_ string, result lightingApplyResult) {
				if result == lightingApplied {
					sendWithin(t, applied, struct{}{})
				}
			}
			d.setDeviceColor()
			waitDone(t, applied)
			workersMu.Lock()
			snapshot := append([]*lightingWorker(nil), workers...)
			workersMu.Unlock()
			if len(snapshot) != 2 || snapshot[0] != snapshot[1] || snapshot[0] == nil {
				t.Fatal("one-shot did not retry same worker")
			}
			waitDone(t, snapshot[0].done)
		})
	}
	for _, transition := range []string{"cancel", "stale"} {
		t.Run(transition, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			readyLighting(d)
			d.lightingWrite = nil
			observed, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			var writes atomic.Int32
			d.lightingReportWrite = func([]byte) (int, error) { writes.Add(1); return 0, errors.New("transient") }
			d.lightingAfterApply = func(_ string, result lightingApplyResult) {
				if result == lightingRetry {
					close(observed)
					waitDone(t, release)
				}
			}
			d.setDeviceColor()
			waitDone(t, observed)
			d.lightingMu.Lock()
			worker := d.lightingWorker
			d.lightingMu.Unlock()
			if transition == "cancel" {
				d.stopLighting()
			} else {
				d.SetConnected(false)
			}
			once.Do(func() { close(release) })
			waitDone(t, worker.done)
			if writes.Load() != 1 {
				t.Fatal("retired worker kept retrying")
			}
		})
	}
}

func TestTeardownOrdersCloseExternalStopOnce(t *testing.T) {
	for _, order := range [][]string{{"exit", "stop", "internal"}, {"internal", "exit", "stop"}, {"stop", "internal", "exit"}} {
		t.Run(order[0], func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			d.startLightingQueue()
			for _, method := range order {
				switch method {
				case "exit":
					d.exitLighting()
				case "stop":
					d.Stop()
				case "internal":
					d.StopInternal()
				}
			}
			d.lightingMu.Lock()
			done := d.externalDone
			d.lightingMu.Unlock()
			waitDone(t, done)
		})
	}
}

func TestSingleWakeRetriesTransientSetupFailure(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.SetConnected(false)
	var angle atomic.Int32
	var epochsMu sync.Mutex
	var epochs []uint64
	d.lightingReportWrite = func(report []byte) (int, error) {
		if bytes.Equal(report[2:5], cmdAngleSnapping) {
			d.lightingMu.Lock()
			epoch := d.connectionGeneration
			d.lightingMu.Unlock()
			epochsMu.Lock()
			epochs = append(epochs, epoch)
			epochsMu.Unlock()
			if angle.Add(1) == 1 {
				return 0, errors.New("transient angle failure")
			}
		}
		return len(report), nil
	}
	d.Connect()
	d.stopLighting()
	d.lightingMu.Lock()
	connected := d.Connected
	d.lightingMu.Unlock()
	epochsMu.Lock()
	same := len(epochs) == 2 && epochs[0] == epochs[1]
	epochsMu.Unlock()
	if !connected || angle.Load() != 2 || !same {
		t.Fatal("setup did not retry on current wake", connected, angle.Load())
	}
}

func TestWorkspaceDPISaveRequiresCompleteColors(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	before := inputJSON(t, d)
	persisted, programmed := 0, 0
	d.lightingProfileWrite = func(string, []byte) error { persisted++; return nil }
	d.lightingDPIWrite = func([]byte, []byte) error { programmed++; return nil }
	values := map[int]uint16{}
	for k, p := range d.DeviceProfile.Profiles {
		values[k] = p.Value
	}
	if d.SaveMouseDPISettings(values, nil) != 0 || d.SaveMouseDPISettings(values, map[int]rgb.Color{}) != 0 || !bytes.Equal(before, inputJSON(t, d)) || persisted != 0 || programmed != 0 {
		t.Fatal("nil workspace colors accepted")
	}
	if d.SaveMouseDPI(map[int]uint16{1: 1200}) != 1 || persisted != 1 || programmed != 2 {
		t.Fatal("value-only legacy path broken")
	}
}

func TestIndicatorRefreshPreservesExternalLogo(t *testing.T) {
	for _, owner := range []string{"cluster", "openrgb"} {
		t.Run(owner, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			enable := d.ProcessSetRgbCluster
			if owner == "openrgb" {
				enable = d.ProcessSetOpenRgbIntegration
			}
			if enable(true) != 1 {
				t.Fatal("enable")
			}
			readyLighting(d)
			frames := make(chan []byte, 8)
			d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
			result := d.applyLightingPacket(composeLightingPacket([]byte{11, 22, 33}), owner, d.externalGeneration, nil, d.connectionGeneration)
			if result != lightingApplied {
				t.Fatal(result)
			}
			waitFrame(t, frames)
			if d.SaveMouseDpiColors(rgb.Color{}, map[int]rgb.Color{1: {Red: 100, Green: 50, Blue: 25}}) != 1 {
				t.Fatal("color save")
			}
			if got := waitFrame(t, frames); !bytes.Equal(got, []byte{11, 100, 22, 50, 33, 25}) {
				t.Fatal("Logo blanked", got)
			}
			requireLighting(t, d.setUserRGBOff(true))
			waitFrame(t, frames)
			if d.SaveMouseDpiColors(rgb.Color{}, map[int]rgb.Color{1: {Red: 10, Green: 20, Blue: 30}}) != 1 {
				t.Fatal("color save while off")
			}
			if got := waitFrame(t, frames); !bytes.Equal(got, []byte{0, 10, 0, 20, 0, 30}) {
				t.Fatal("RGB-off resurrected old Logo", got)
			}
		})
	}
}

func TestDeviceProfileCapabilityDecorationDoesNotReenter(t *testing.T) {
	d, _ := lightingFixture(t, true)
	done := make(chan struct{})
	go func() {
		defer close(done)
		snapshot, ok := d.DeviceProfileSnapshot()
		if !ok || !snapshot.CanSave || !snapshot.CanSwitch || !snapshot.CanDelete {
			t.Error("capabilities unavailable")
		}
	}()
	waitDone(t, done)
}
func waitCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for !condition() {
		select {
		case <-deadline.C:
			t.Fatal("condition timed out")
		default:
			runtime.Gosched()
		}
	}
}

func TestLiteralPreservedProfileNameSurvivesBootstrap(t *testing.T) {
	d := profileSaveFixture(t, "default")
	if d.SaveUserProfile("Z-preserved") != 1 || d.ChangeDeviceProfile("Z-preserved") != 1 {
		t.Fatal("save/activate")
	}
	d.stopLighting()
	restarted := &Device{Serial: d.Serial, Product: d.Product}
	restarted.loadDeviceProfiles()
	restarted.saveDeviceProfile()
	restarted.loadDeviceProfiles()
	if restarted.UserProfiles["Z-preserved"] == nil || restarted.DeviceProfile != restarted.UserProfiles["Z-preserved"] || restarted.UserProfiles["Z"] == restarted.UserProfiles["Z-preserved"] {
		t.Fatal("bootstrap lost literal profile identity", restarted.ProfileOrder)
	}
}

func TestCachedIndicatorReplayCannotOverwriteNewerExternalFrame(t *testing.T) {
	d, _ := lightingFixture(t, true)
	if d.ProcessSetRgbCluster(true) != 1 {
		t.Fatal("owner")
	}
	readyLighting(d)
	frames := make(chan []byte, 4)
	d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
	d.writeColorCluster([]byte{1, 2, 3}, 0)
	waitFrame(t, frames)
	d.lightingMu.Lock()
	old := *d.lastExternal
	old.data = append([]byte(nil), old.data...)
	d.lightingMu.Unlock()
	d.writeColorCluster([]byte{11, 22, 33}, 0)
	waitFrame(t, frames)
	if result := d.applyLightingPacket(old.data, old.source, old.generation, nil, old.connection, old.version); result != lightingStale || len(frames) != 0 {
		t.Fatal("old replay overwrote newer Logo")
	}
}

func TestDisconnectCancelsTransientSetupRetry(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.SetConnected(false)
	var angle atomic.Int32
	d.lightingReportWrite = func(report []byte) (int, error) {
		if bytes.Equal(report[2:5], cmdAngleSnapping) {
			angle.Add(1)
			d.SetConnected(false)
			return 0, errors.New("setup retired")
		}
		return len(report), nil
	}
	done := make(chan struct{})
	go func() { d.Connect(); close(done) }()
	waitDone(t, done)
	d.lightingMu.Lock()
	connected, connecting := d.Connected, d.connecting
	d.lightingMu.Unlock()
	if connected || connecting || angle.Load() != 1 {
		t.Fatal("cancelled setup retried or published readiness", connected, connecting, angle.Load())
	}
}
