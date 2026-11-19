package m65rgbultraW

import (
	"LumenForge/src/common"
	"LumenForge/src/inputmanager"
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
)

func TestCorruptAssignmentJSONStillConnects(t *testing.T) {
	for _, kind := range []string{"json", "unreadable"} {
		t.Run(kind, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			d.SetConnected(false)
			d.KeyAssignment = nil
			path := filepath.Join(pwd, "database", "key-assignments", d.DeviceProfile.KeyAssignmentHash+".json")
			if kind == "json" {
				requireLighting(t, os.WriteFile(path, []byte("{invalid custom mapping"), 0600))
			} else {
				requireLighting(t, os.Remove(path))
				requireLighting(t, os.Mkdir(path, 0700))
			}
			if err := d.loadCanonicalAssignments(); err == nil {
				t.Fatal("load error hidden")
			}
			var payloads atomic.Int32
			d.lightingDPIWrite = func(cmd, data []byte) error {
				if bytes.Equal(cmd, cmdWrite) {
					payloads.Add(1)
				}
				return nil
			}
			d.lightingReportWrite = func(p []byte) (int, error) { return len(p), nil }
			d.Connect()
			d.stopLighting()
			if !d.Connected || !d.lightingHardwareReady || payloads.Load() != 0 {
				t.Fatal("corrupt local map blocked availability", payloads.Load())
			}
			if kind == "json" {
				data, err := os.ReadFile(path)
				requireLighting(t, err)
				if string(data) != "{invalid custom mapping" {
					t.Fatal("corrupt data silently replaced")
				}
			}
			if !errors.Is(d.programCanonicalAssignments(), errInvalidAssignments) {
				t.Fatal("invalid mapping no longer observable")
			}
		})
	}
}
func TestAngleTransactionIgnoresUnrelatedMutations(t *testing.T) {
	for _, change := range []string{"snapshot", "save-as", "timer", "color", "owner", "new-angle", "profile", "connection", "rebind", "stop"} {
		t.Run(change, func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			d.KeyAssignment = defaultWirelessAssignments()
			readyLighting(d)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			d.lightingInputBeforeWrite = func(_ connectionSession, cmd []byte) {
				if bytes.Equal(cmd, cmdAngleSnapping) {
					close(entered)
					waitDone(t, release)
				}
			}
			var sent atomic.Int32
			d.lightingDPIWrite = func(cmd, data []byte) error {
				if bytes.Equal(cmd, cmdAngleSnapping) {
					if !bytes.Equal(data, []byte{1}) {
						t.Error("wrong angle", data)
					}
					sent.Add(1)
				}
				return nil
			}
			finished := make(chan uint8, 1)
			go func() { sendWithin(t, finished, d.UpdateAngleSnapping(1)) }()
			waitDone(t, entered)
			changed := make(chan struct{})
			switch change {
			case "snapshot":
				if d.SaveUserProfile("default") != 1 {
					t.Fatal("snapshot")
				}
				close(changed)
			case "save-as":
				if d.SaveUserProfile("New-Profile") != 1 {
					t.Fatal("save as")
				}
				close(changed)
			case "timer":
				if d.mutateInputProfile(false, func(p *DeviceProfile) error { p.SleepMode = 30; return nil }) != 1 {
					t.Fatal("timer")
				}
				close(changed)
			case "color":
				if d.SaveMouseDpiColors(rgb.Color{}, map[int]rgb.Color{1: {Red: 10}}) != 1 {
					t.Fatal("color")
				}
				close(changed)
			case "owner":
				d.ProcessSetRgbCluster(true)
				close(changed)
			case "new-angle":
				d.mutateInputProfile(false, func(p *DeviceProfile) error { p.AngleSnapping = 0; return nil })
				close(changed)
			case "profile":
				go func() { d.ChangeDeviceProfile("Z"); close(changed) }()
				waitCondition(t, func() bool {
					d.lightingMu.Lock()
					defer d.lightingMu.Unlock()
					return d.DeviceProfile == d.UserProfiles["Z"]
				})
			case "connection":
				d.SetConnected(false)
				reconnectFixtureWithoutOutput(d)
				close(changed)
			case "rebind":
				d.RebindReceiver(&common.Slipstream{}, 12)
				close(changed)
			case "stop":
				d.StopDirty()
				close(changed)
			}
			once.Do(func() { close(release) })
			receiveWithin(t, finished)
			waitDone(t, changed)
			want := int32(1)
			if change == "new-angle" || change == "profile" || change == "connection" || change == "rebind" || change == "stop" {
				want = 0
			}
			if sent.Load() != want {
				t.Fatal("wrong angle invalidation", sent.Load(), want)
			}
		})
	}
}
func TestMovedProfilePathsUseActualLoadedLocation(t *testing.T) {
	d := legacyIdentityFixture(t)
	oldRoot := t.TempDir()
	paths := map[string]string{}
	for _, name := range []string{"Travel", "default", "Desk-preserved"} {
		p := cloneLightingProfile(*d.DeviceProfile)
		p.Active = false
		p.ProfileName = ""
		file := d.Serial + "-" + name + ".json"
		if name == "default" {
			file = d.Serial + ".json"
		}
		actual := filepath.Join(pwd, "database", "profiles", file)
		p.Path = filepath.Join(oldRoot, file)
		data, _ := json.Marshal(p)
		requireLighting(t, os.WriteFile(actual, data, 0600))
		paths[name] = actual
	}
	d.loadDeviceProfiles()
	for name, path := range paths {
		if d.UserProfiles[name] == nil || d.UserProfiles[name].Path != path {
			t.Fatal("serialized stale path retained", name)
		}
	}
	d.DeviceProfile.Label = "saved at new root"
	if d.SaveUserProfile("Travel") != 1 {
		t.Fatal("overwrite")
	}
	d.loadDeviceProfiles()
	if d.UserProfiles["Travel"].Label != "saved at new root" || d.UserProfiles["Travel"].Path != paths["Travel"] {
		t.Fatal("overwrite left current root")
	}
	entries, err := os.ReadDir(oldRoot)
	requireLighting(t, err)
	if len(entries) != 0 {
		t.Fatal("wrote old data root")
	}
}
func TestMacroPlaybackDoesNotBlockHeldTiltCleanup(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(map[bool]string{false: "disconnect", true: "stop"}[stop], func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			readyLighting(d)
			a := d.KeyAssignment[1024]
			a.Default = false
			a.ActionHold = true
			a.ActionType = 3
			a.ActionCommand = 10
			d.KeyAssignment[1024] = a
			d.KeyAssignment[8] = inputmanager.KeyAssignment{ActionType: 10, ActionCommand: 42}
			released := make(chan struct{}, 1)
			var presses atomic.Int32
			d.tiltHoldWrite = func(_ uint8, key uint16, down bool) {
				if key != 10 {
					t.Error("wrong action")
				}
				if down {
					presses.Add(1)
				} else {
					sendWithin(t, released, struct{}{})
				}
			}
			d.TriggerTiltAssignment(1024)
			entered, resume := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(resume) })
			d.inputMacroPlayback = func(command uint16) {
				if command != 42 {
					t.Error("wrong macro")
				}
				close(entered)
				waitDone(t, resume)
			}
			played := make(chan struct{})
			go func() { d.TriggerKeyAssignment(8); close(played) }()
			waitDone(t, entered)
			cleaned := make(chan struct{})
			go func() {
				if stop {
					d.StopDirty()
				} else {
					d.SetConnected(false)
				}
				close(cleaned)
			}()
			receiveWithin(t, released)
			waitDone(t, cleaned)
			select {
			case <-played:
				t.Fatal("macro was not blocked")
			default:
			}
			if presses.Load() != 1 {
				t.Fatal("duplicate hold")
			}
			once.Do(func() { close(resume) })
			waitDone(t, played)
		})
	}
}
func TestIdenticalDPIAndSniperRetryBothAxes(t *testing.T) {
	for _, sniper := range []bool{false, true} {
		t.Run(map[bool]string{false: "regular", true: "sniper"}[sniper], func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			readyLighting(d)
			var writes []dpiAxisWrite
			failed := false
			d.lightingDPIWrite = func(cmd, data []byte) error {
				if bytes.Equal(cmd, cmdSetDpi[0]) || bytes.Equal(cmd, cmdSetDpi[1]) {
					writes = append(writes, recordDPIAxis(t, cmd, data))
					if bytes.Equal(cmd, cmdSetDpi[1]) && !failed {
						failed = true
						return errors.New("Y unconfirmed")
					}
				}
				return nil
			}
			request := func() {
				if sniper {
					if d.SetMouseSniperMode(true) != 1 {
						t.Fatal("sniper")
					}
				} else if d.selectCanonicalDPI(2) != 1 {
					t.Fatal("regular")
				}
			}
			request()
			if len(writes) != 4 {
				t.Fatal("partial pair did not retry automatically", writes)
			}
			request()
			value := uint16(1200)
			if sniper {
				value = 200
			}
			if len(writes) != 6 {
				t.Fatal("identical retry skipped", writes)
			}
			for i, w := range writes {
				var command [3]byte
				copy(command[:], cmdSetDpi[i%2])
				if w.Value != value || w.Command != command {
					t.Fatal("wrong retry axes", writes)
				}
			}
			if sniper {
				before := len(writes)
				if d.selectCanonicalDPI(d.DeviceProfile.Profile) != 1 || len(writes) != before {
					t.Fatal("regular stage disturbed active Sniper")
				}
			}
		})
	}
}
func TestGestureAndAssignmentTransactionsAreIndependent(t *testing.T) {
	for _, first := range []string{"gesture", "assignment"} {
		t.Run(first, func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			d.KeyAssignment = defaultWirelessAssignments()
			readyLighting(d)
			entered, resume := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(resume) })
			var paused atomic.Bool
			var tilts, payloads atomic.Int32
			d.lightingDPIWrite = func(cmd, data []byte) error {
				if (first == "gesture" && bytes.Equal(cmd, cmdMultiGesture)) || (first == "assignment" && bytes.Equal(cmd, cmdOpenWriteEndpoint)) {
					if !paused.Swap(true) {
						close(entered)
						waitDone(t, resume)
					}
				}
				if bytes.Equal(cmd, cmdWrite) {
					payloads.Add(1)
				}
				for _, threshold := range [][]byte{cmdLeftTilt, cmdRightTilt, cmdForwardTilt, cmdBackwardTilt} {
					if bytes.Equal(cmd, threshold) {
						tilts.Add(1)
					}
				}
				return nil
			}
			gesture := func() uint8 { return d.SaveMouseGestures(1, map[int]uint8{0: 30, 1: 40, 2: 50, 3: 60}) }
			assignment := func() uint8 {
				a := d.inputAssignmentsSnapshot()[8]
				a.Default = false
				return d.UpdateDeviceKeyAssignment(8, a)
			}
			firstDone, secondDone := make(chan uint8, 1), make(chan uint8, 1)
			if first == "gesture" {
				go func() { sendWithin(t, firstDone, gesture()) }()
			} else {
				go func() { sendWithin(t, firstDone, assignment()) }()
			}
			waitDone(t, entered)
			d.lightingMu.Lock()
			a, g := d.assignmentRevision, d.gestureRevision
			d.lightingMu.Unlock()
			if first == "gesture" {
				go func() { sendWithin(t, secondDone, assignment()) }()
				waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.assignmentRevision > a })
			} else {
				go func() { sendWithin(t, secondDone, gesture()) }()
				waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.gestureRevision > g })
			}
			once.Do(func() { close(resume) })
			if receiveWithin(t, firstDone) != 1 || receiveWithin(t, secondDone) != 1 || tilts.Load() != 4 || payloads.Load() != 1 {
				t.Fatal("independent transaction cancelled", tilts.Load(), payloads.Load())
			}
		})
	}
}
func TestGestureReplacementAndProfileRetireOldThresholds(t *testing.T) {
	for _, change := range []string{"gesture", "profile"} {
		t.Run(change, func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			d.KeyAssignment = defaultWirelessAssignments()
			readyLighting(d)
			entered, resume := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(resume) })
			var first atomic.Bool
			type thresholdWrite struct {
				command string
				value   []byte
			}
			var writes []thresholdWrite
			var writesMu sync.Mutex
			d.lightingInputBeforeWrite = func(_ connectionSession, cmd []byte) {
				if bytes.Equal(cmd, cmdMultiGesture) && !first.Swap(true) {
					close(entered)
					waitDone(t, resume)
				}
			}
			d.lightingDPIWrite = func(cmd, data []byte) error {
				for _, v := range [][]byte{cmdLeftTilt, cmdRightTilt, cmdForwardTilt, cmdBackwardTilt} {
					if bytes.Equal(cmd, v) {
						writesMu.Lock()
						writes = append(writes, thresholdWrite{string(cmd), append([]byte(nil), data...)})
						writesMu.Unlock()
					}
				}
				return nil
			}
			done := make(chan uint8, 1)
			go func() { sendWithin(t, done, d.SaveMouseGestures(1, map[int]uint8{0: 30, 1: 40, 2: 50, 3: 60})) }()
			waitDone(t, entered)
			changed := make(chan struct{})
			d.lightingMu.Lock()
			revision := d.gestureRevision
			d.lightingMu.Unlock()
			if change == "gesture" {
				go func() { d.SaveMouseGestures(0, map[int]uint8{0: 20, 1: 20, 2: 20, 3: 20}); close(changed) }()
			} else {
				go func() { d.ChangeDeviceProfile("Z"); close(changed) }()
			}
			waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.gestureRevision > revision })
			once.Do(func() { close(resume) })
			receiveWithin(t, done)
			waitDone(t, changed)
			writesMu.Lock()
			defer writesMu.Unlock()
			if change == "profile" {
				if len(writes) != 0 {
					t.Fatal("retired profile thresholds written", writes)
				}
			} else {
				expected := []thresholdWrite{}
				for _, command := range [][]byte{cmdLeftTilt, cmdRightTilt, cmdForwardTilt, cmdBackwardTilt} {
					expected = append(expected, thresholdWrite{string(command), []byte{20}})
				}
				if !reflect.DeepEqual(writes, expected) {
					t.Fatal("replacement must exclusively write current thresholds", writes, expected)
				}
			}
		})
	}
}

func TestExplicitTiltActivationFailureIsReportedAndRecovers(t *testing.T) {
	for _, key := range []int{1024, 8} {
		t.Run(map[int]string{1024: "tilt", 8: "button"}[key], func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			d.KeyAssignment = defaultWirelessAssignments()
			readyLighting(d)
			fail := true
			attempts := 0
			d.lightingDPIWrite = func(cmd, data []byte) error {
				if bytes.Equal(cmd, cmdActivateLeftTilt) {
					attempts++
					if fail {
						return errors.New("activation failed")
					}
				}
				return nil
			}
			a := d.inputAssignmentsSnapshot()[key]
			a.Default = false
			result := d.UpdateDeviceKeyAssignment(key, a)
			want := uint8(0)
			if key == 8 {
				want = 1
			}
			if result != want || attempts != 3 {
				t.Fatal("wrong edited-tilt policy", result, attempts)
			}
			saved, err := d.readLightingAssignments(*d.DeviceProfile)
			requireLighting(t, err)
			if saved[key].Default {
				t.Fatal("durable edit rolled back")
			}
			fail = false
			d.SetConnected(false)
			d.lightingReportWrite = func(p []byte) (int, error) { return len(p), nil }
			d.Connect()
			d.stopLighting()
			if !d.Connected || attempts != 4 {
				t.Fatal("wake did not recover activation", attempts)
			}
		})
	}
}
func TestProfileSwitchFixtureReadsActualAssignmentSource(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	source := defaultWirelessAssignments()
	v := source[8]
	v.Default = false
	source[8] = v
	data, _ := json.Marshal(source)
	requireLighting(t, os.WriteFile(filepath.Join(pwd, "database", "key-assignments", d.DeviceProfile.KeyAssignmentHash+".json"), data, 0600))
	next := cloneLightingProfile(*d.DeviceProfile)
	next.Active = false
	next.Path = filepath.Join(filepath.Dir(next.Path), "next.json")
	next.RgbOff = true
	d.UserProfiles["next"] = &next
	frames := make(chan []byte, 4)
	d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
	if d.ChangeDeviceProfile("next") != 1 {
		t.Fatal("fixture blocked profile switch")
	}
	if !reflect.DeepEqual(source, d.inputAssignmentsSnapshot()) {
		t.Fatal("assignment preparation bypassed")
	}
	packet := waitFrame(t, frames)
	if packet[0] != 0 || packet[2] != 0 || packet[4] != 0 {
		t.Fatal("profile did not reach blank output", packet)
	}
}
