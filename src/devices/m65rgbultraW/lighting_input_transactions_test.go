package m65rgbultraW

import (
	"LumenForge/src/common"
	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
	"bytes"
	"encoding/binary"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestAssignmentPayloadSurvivesUnrelatedMutations(t *testing.T) {
	for _, change := range []string{"timer", "dpi", "color", "preference", "owner", "snapshot"} {
		t.Run(change, func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			d.KeyAssignment = defaultWirelessAssignments()
			readyLighting(d)
			var payload []byte
			d.lightingMu.Lock()
			revision := d.assignmentRevision
			d.lightingMu.Unlock()
			d.lightingDPIWrite = func(cmd, data []byte) error {
				if bytes.Equal(cmd, cmdOpenWriteEndpoint) {
					switch change {
					case "timer":
						if d.mutateInputProfile(false, func(p *DeviceProfile) error { p.SleepMode = 30; return nil }) != 1 {
							t.Fatal("timer")
						}
					case "dpi":
						if d.mutateInputProfile(false, func(p *DeviceProfile) error { s := p.Profiles[1]; s.Value = 1600; p.Profiles[1] = s; return nil }) != 1 {
							t.Fatal("DPI")
						}
					case "color":
						if d.SaveMouseDpiColors(rgb.Color{}, map[int]rgb.Color{1: {Red: 10}}) != 1 {
							t.Fatal("color")
						}
					case "preference":
						if d.mutateInputProfile(false, func(p *DeviceProfile) error { p.AngleSnapping = 1; return nil }) != 1 {
							t.Fatal("angle")
						}
					case "owner":
						if d.ProcessSetRgbCluster(true) != 1 {
							t.Fatal("owner")
						}
					case "snapshot":
						if d.SaveUserProfile("default") != 1 {
							t.Fatal("snapshot")
						}
					}
				}
				if bytes.Equal(cmd, cmdWrite) {
					payload = append([]byte(nil), data...)
				}
				return nil
			}
			assignment := d.inputAssignmentsSnapshot()[8]
			assignment.Default = false
			if d.UpdateDeviceKeyAssignment(8, assignment) != 1 {
				t.Fatal("saved assignment not programmed")
			}
			if len(payload) != headerWriteSize+8 || payload[headerWriteSize+3] != 0 {
				t.Fatal("wrong main binding packet", payload)
			}
			d.lightingMu.Lock()
			current := d.assignmentRevision
			d.lightingMu.Unlock()
			if current != revision+1 {
				t.Fatal("unrelated mutation advanced assignment revision", revision, current)
			}
		})
	}
}

func TestGestureSequenceSurvivesTimerEdit(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	writes := 0
	d.lightingDPIWrite = func(cmd, data []byte) error {
		for _, tilt := range [][]byte{cmdLeftTilt, cmdRightTilt, cmdForwardTilt, cmdBackwardTilt} {
			if bytes.Equal(cmd, tilt) {
				writes++
				if writes == 1 {
					if d.mutateInputProfile(false, func(p *DeviceProfile) error { p.SleepMode = 30; return nil }) != 1 {
						t.Fatal("timer")
					}
				}
			}
		}
		return nil
	}
	if d.SaveMouseGestures(1, map[int]uint8{0: 30, 1: 40, 2: 50, 3: 60}) != 1 || writes != 4 {
		t.Fatal("timer interrupted gestures", writes)
	}
}

func TestAssignmentTransactionsRejectRelevantReplacement(t *testing.T) {
	for _, change := range []string{"assignment", "profile", "connection", "rebind", "stop"} {
		t.Run(change, func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			d.KeyAssignment = defaultWirelessAssignments()
			readyLighting(d)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			var opened atomic.Bool
			var recordMu sync.Mutex
			opens, closes := 0, 0
			type sentPayload struct {
				attempt int
				data    []byte
			}
			var payloads []sentPayload
			d.lightingDPIWrite = func(cmd, data []byte) error {
				recordMu.Lock()
				if bytes.Equal(cmd, cmdOpenWriteEndpoint) {
					opens++
				}
				if bytes.Equal(cmd, cmdCloseEndpoint) {
					closes++
				}
				if bytes.Equal(cmd, cmdWrite) {
					payloads = append(payloads, sentPayload{opens, append([]byte(nil), data...)})
				}
				recordMu.Unlock()
				if bytes.Equal(cmd, cmdOpenWriteEndpoint) && !opened.Swap(true) {
					close(entered)
					waitDone(t, release)
				}
				return nil
			}
			finished := make(chan error, 1)
			go func() { sendWithin(t, finished, d.programCanonicalAssignments()) }()
			waitDone(t, entered)
			changed := make(chan struct{})
			d.lightingMu.Lock()
			rev := d.assignmentRevision
			d.lightingMu.Unlock()
			switch change {
			case "assignment":
				go func() {
					a := d.inputAssignmentsSnapshot()[8]
					a.Default = false
					d.UpdateDeviceKeyAssignment(8, a)
					close(changed)
				}()
			case "gesture":
				go func() { d.SaveMouseGestures(1, map[int]uint8{0: 30, 1: 40, 2: 50, 3: 60}); close(changed) }()
			case "profile":
				go func() { d.ChangeDeviceProfile("Z"); close(changed) }()
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
			if change == "assignment" || change == "gesture" || change == "profile" {
				waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.assignmentRevision > rev })
			}
			once.Do(func() { close(release) })
			if err := receiveWithin(t, finished); !errors.Is(err, errLightingSessionRetired) {
				t.Fatal("old assignment was not retired", err)
			}
			waitDone(t, changed)
			recordMu.Lock()
			defer recordMu.Unlock()
			for _, sent := range payloads {
				if sent.attempt == 1 {
					t.Fatal("retired transaction sent its main payload", sent.data)
				}
			}
			if change == "assignment" {
				if opens != 2 || closes != 2 || len(payloads) != 1 || payloads[0].attempt != 2 || payloads[0].data[headerWriteSize+3] != 0 {
					t.Fatal("replacement did not exclusively send new binding", opens, closes, payloads)
				}
			}
		})
	}
}

func TestAssignmentProgrammingFailureIsReportedAfterPersistence(t *testing.T) {
	d := profileSaveFixture(t, "default")
	d.KeyAssignment = defaultWirelessAssignments()
	readyLighting(d)
	failure := errors.New("main payload failed")
	d.lightingDPIWrite = func(cmd, data []byte) error {
		if bytes.Equal(cmd, cmdWrite) {
			return failure
		}
		return nil
	}
	a := d.inputAssignmentsSnapshot()[8]
	a.Default = false
	if d.UpdateDeviceKeyAssignment(8, a) != 0 {
		t.Fatal("reported hardware success")
	}
	if d.inputAssignmentsSnapshot()[8].Default {
		t.Fatal("rolled back durable preference")
	}
	bindings, err := d.readLightingAssignments(*d.DeviceProfile)
	requireLighting(t, err)
	if bindings[8].Default {
		t.Fatal("new binding not durable")
	}
}

func TestTiltActivationFailureRemainsOptionalDuringWake(t *testing.T) {
	for _, change := range []string{"timeout", "disconnect", "stop"} {
		t.Run(change, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			d.SetConnected(false)
			d.lightingDPIWrite = nil
			tilts, payloads := 0, 0
			d.lightingReportWrite = func(report []byte) (int, error) {
				if bytes.Equal(report[2:4], cmdWrite) {
					payloads++
				}
				if bytes.Equal(report[2:2+len(cmdActivateLeftTilt)], cmdActivateLeftTilt) {
					tilts++
					if change == "disconnect" {
						d.SetConnected(false)
					} else if change == "stop" {
						d.StopDirty()
					}
					return 0, errors.New("tilt activation timeout")
				}
				return len(report), nil
			}
			d.Connect()
			d.stopLighting()
			d.lightingMu.Lock()
			ready := d.Connected && d.lightingHardwareReady
			d.lightingMu.Unlock()
			if change == "timeout" {
				if !ready || tilts != 3 || payloads != 1 {
					t.Fatal("optional tilt blocked readiness/replayed main", ready, tilts, payloads)
				}
			} else if ready || tilts != 1 || payloads != 1 {
				t.Fatal("retired tilt work continued", ready, tilts, payloads)
			}
		})
	}
}

func TestSoftwareModeAdmissionRestoresOnOverlappingShutdown(t *testing.T) {
	for _, change := range []string{"clean", "dirty", "rebind", "rejected"} {
		t.Run(change, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			var restored atomic.Int32
			d.lightingReportWrite = func(report []byte) (int, error) {
				if bytes.Equal(report[2:2+len(cmdSoftwareMode)], cmdSoftwareMode) {
					close(entered)
					waitDone(t, release)
					return 0, errors.New("software reply timeout")
				}
				if bytes.Equal(report[2:2+len(cmdHardwareMode)], cmdHardwareMode) {
					if report[1] != 9 {
						t.Error("restored replacement endpoint")
					}
					restored.Add(1)
				}
				return len(report), nil
			}
			d.lightingMu.Lock()
			session := d.sessionLocked()
			d.lightingMu.Unlock()
			finished := make(chan error, 1)
			go func() {
				_, err := d.connectionTransfer(session, cmdSoftwareMode, nil, func() bool { return change != "rejected" && !d.Exit })
				sendWithin(t, finished, err)
			}()
			if change == "rejected" {
				if !errors.Is(receiveWithin(t, finished), errLightingSessionRetired) {
					t.Fatal("admitted invalid write")
				}
				d.stopCanonicalLighting(true)
			} else {
				waitDone(t, entered)
				if change == "rebind" {
					d.RebindReceiver(&common.Slipstream{}, 12)
				}
				stopped := make(chan struct{})
				go func() { d.stopCanonicalLighting(change != "dirty"); close(stopped) }()
				waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.Exit })
				once.Do(func() { close(release) })
				receiveWithin(t, finished)
				waitDone(t, stopped)
			}
			want := int32(0)
			if change == "clean" {
				want = 1
			}
			if restored.Load() != want {
				t.Fatal("wrong software restore", restored.Load(), want)
			}
		})
	}
}

func TestSleepFirstReportTimeoutStillAttemptsCurrentDuration(t *testing.T) {
	for _, change := range []string{"timeout", "both-errors", "normal", "timer", "connection", "profile", "rebind"} {
		t.Run(change, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			readyLighting(d)
			switched := make(chan struct{})
			if change != "profile" {
				close(switched)
			}
			durations := []uint32{}
			d.lightingDPIWrite = func(cmd, data []byte) error {
				if bytes.Equal(cmd, cmdSleep[0]) {
					switch change {
					case "timer":
						if d.mutateInputProfile(false, func(p *DeviceProfile) error { p.SleepMode = 30; return nil }) != 1 {
							t.Fatal("timer")
						}
					case "connection":
						d.SetConnected(false)
						reconnectFixtureWithoutOutput(d)
					case "rebind":
						d.RebindReceiver(&common.Slipstream{}, 12)
					case "profile":
						next := cloneLightingProfile(*d.DeviceProfile)
						next.Active = false
						next.Path = filepath.Join(filepath.Dir(next.Path), "next.json")
						d.UserProfiles["next"] = &next
						go func() { d.ChangeDeviceProfile("next"); close(switched) }()
						waitCondition(t, func() bool {
							d.lightingMu.Lock()
							defer d.lightingMu.Unlock()
							return d.DeviceProfile == d.UserProfiles["next"]
						})
					}
					if change != "normal" {
						return errors.New("first report applied; reply timed out")
					}
				}
				if bytes.Equal(cmd, cmdSleep[1]) {
					durations = append(durations, binary.LittleEndian.Uint32(data))
					if change == "both-errors" {
						return errors.New("duration reply timeout")
					}
				}
				return nil
			}
			result := d.programCanonicalSleep()
			waitDone(t, switched)
			if change == "timeout" || change == "both-errors" || change == "normal" {
				if len(durations) != 1 || durations[0] != 900000 {
					t.Fatal("duration not sent", durations)
				}
			} else if len(durations) != 0 {
				t.Fatal("stale duration sent", durations)
			}
			if (change == "normal" && result != 1) || (change != "normal" && result != 0) {
				t.Fatal("wrong error result", result)
			}
		})
	}
}

func TestAssignmentRevisionTracksOnlyEffectivePayload(t *testing.T) {
	d := profileSaveFixture(t, "default")
	d.KeyAssignment = defaultWirelessAssignments()
	a := d.KeyAssignment[8]
	a.Default = false
	if d.UpdateDeviceKeyAssignment(8, a) != 1 {
		t.Fatal("binding")
	}
	r := d.assignmentRevision
	a.ActionType = 3
	a.ActionCommand = 42
	if d.UpdateDeviceKeyAssignment(8, a) != 1 || d.assignmentRevision != r {
		t.Fatal("local action invalidated packet")
	}
	d.setInputTiltToggle(1024, true)
	if d.assignmentRevision != r {
		t.Fatal("runtime toggle invalidated packet")
	}
	// Names are serialization metadata, not bytes in either hardware sequence.
	p := cloneLightingProfile(*d.DeviceProfile)
	v := p.Tilts[0]
	v.Name = "renamed"
	p.Tilts[0] = v
	if !sameGesturePayload(*d.DeviceProfile, p) || !sameAssignmentPayload(d.KeyAssignment, mapCopyAssignments(d.KeyAssignment)) {
		t.Fatal("metadata counted as payload")
	}
}
func mapCopyAssignments(in map[int]inputmanager.KeyAssignment) map[int]inputmanager.KeyAssignment {
	out := make(map[int]inputmanager.KeyAssignment, len(in))
	for k, v := range in {
		v.Name = "renamed"
		out[k] = v
	}
	return out
}

func TestPublicSleepEditDuringAssignmentProgramming(t *testing.T) {
	d := profileSaveFixture(t, "default")
	d.KeyAssignment = defaultWirelessAssignments()
	d.SleepModes = map[int]string{15: "15 minutes", 30: "30 minutes"}
	readyLighting(d)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	var payload []byte
	var duration uint32
	d.lightingDPIWrite = func(cmd, data []byte) error {
		if bytes.Equal(cmd, cmdOpenWriteEndpoint) {
			close(entered)
			waitDone(t, release)
		}
		if bytes.Equal(cmd, cmdWrite) {
			payload = append([]byte(nil), data...)
		}
		if bytes.Equal(cmd, cmdSleep[1]) {
			duration = binary.LittleEndian.Uint32(data)
		}
		return nil
	}
	assignments, timer := make(chan uint8, 1), make(chan uint8, 1)
	go func() {
		a := d.inputAssignmentsSnapshot()[8]
		a.Default = false
		sendWithin(t, assignments, d.UpdateDeviceKeyAssignment(8, a))
	}()
	waitDone(t, entered)
	d.lightingMu.Lock()
	revision := d.timerRevision
	d.lightingMu.Unlock()
	go func() { sendWithin(t, timer, d.UpdateSleepTimer(30)) }()
	waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.timerRevision > revision })
	once.Do(func() { close(release) })
	if receiveWithin(t, assignments) != 1 || receiveWithin(t, timer) != 1 {
		t.Fatal("public update reported failure")
	}
	if len(payload) != headerWriteSize+8 || payload[headerWriteSize+3] != 0 || duration != 1800000 {
		t.Fatal("wrong overlapped hardware output", payload, duration)
	}
}
