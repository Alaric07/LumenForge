package m65rgbultraW

import (
	"LumenForge/src/common"
	"LumenForge/src/inputmanager"
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
)

func TestIndicatorReplayDoesNotCancelPendingRGBOffBlank(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		t.Run(map[bool]string{false: "replay", true: "fresh-controller"}[fresh], func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			if d.ProcessSetRgbCluster(true) != 1 {
				t.Fatal("owner")
			}
			readyLighting(d)
			frames := make(chan []byte, 8)
			d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
			d.writeColorCluster([]byte{255, 0, 0}, 0)
			waitFrame(t, frames)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			d.lightingBeforeApply = func(source string) {
				if source == "blank" {
					close(entered)
					waitDone(t, release)
				}
			}
			requireLighting(t, d.setUserRGBOff(true))
			waitDone(t, entered)
			d.lightingMu.Lock()
			worker, controller, logo := d.lightingWorker, d.externalFrame, d.logoVersion
			d.lightingMu.Unlock()
			if fresh {
				d.writeColorCluster([]byte{0, 0, 255}, 0)
			} else {
				d.refreshIndicatorLighting()
			}
			replay := waitFrame(t, frames)
			if fresh && !bytes.Equal(replay, []byte{0, 255, 0, 255, 255, 255}) {
				t.Fatal(replay)
			}
			d.lightingMu.Lock()
			freshCounter, logoCounter := d.externalFrame, d.logoVersion
			d.lightingMu.Unlock()
			if !fresh && (freshCounter != controller || logoCounter != logo) {
				t.Fatal("internal replay changed Logo/controller version")
			}
			once.Do(func() { close(release) })
			waitDone(t, worker.done)
			if fresh {
				if len(frames) != 0 {
					t.Fatal("old blank overwrote fresh controller frame")
				}
			} else {
				if got := waitFrame(t, frames); !bytes.Equal(got, []byte{0, 255, 0, 255, 0, 255}) {
					t.Fatal("replay cancelled pending RGB-off blank", got)
				}
			}
		})
	}
}

func TestRGBOffInvalidatesOlderIndicatorReplayOnly(t *testing.T) {
	d, _ := lightingFixture(t, true)
	if d.ProcessSetRgbCluster(true) != 1 {
		t.Fatal("owner")
	}
	readyLighting(d)
	frames := make(chan []byte, 8)
	d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
	d.writeColorCluster([]byte{255, 0, 0}, 0)
	waitFrame(t, frames)
	d.lightingMu.Lock()
	old := *d.lastExternal
	controller, logo := d.externalFrame, d.logoVersion
	d.lightingMu.Unlock()
	applied := make(chan struct{})
	d.lightingAfterApply = func(source string, result lightingApplyResult) {
		if source == "blank" && result == lightingApplied {
			close(applied)
		}
	}
	requireLighting(t, d.setUserRGBOff(true))
	black := waitFrame(t, frames)
	waitDone(t, applied)
	d.lightingMu.Lock()
	nextController, nextLogo := d.externalFrame, d.logoVersion
	d.lightingMu.Unlock()
	if nextController != controller || nextLogo <= logo {
		t.Fatal("blank versions overloaded", controller, nextController, logo, nextLogo)
	}
	if result := d.applyLightingPacket(old.data, old.source, old.generation, nil, old.connection, old.version); result != lightingStale || len(frames) != 0 {
		t.Fatal("old cached red resurrected after blank")
	}
	d.refreshIndicatorLighting()
	if got := waitFrame(t, frames); !bytes.Equal(got, black) {
		t.Fatal("current black cache could not replay", got)
	}
	d.lightingMu.Lock()
	unchanged := d.externalFrame == nextController && d.logoVersion == nextLogo
	d.lightingMu.Unlock()
	if !unchanged {
		t.Fatal("replay advanced versions")
	}
}

func TestRGBOffBypassesUnrelatedCorruptEffects(t *testing.T) {
	d, rt := lightingFixture(t, true)
	requireLighting(t, d.SetLightingBrightness(50))
	bad := gradientSettings(t, d, 0, 1)
	bad.Gradient.Stops = bad.Gradient.Stops[:1]
	store := &corruptEffects{lightingEffectStore: rt.Effects, bad: bad, present: true}
	d.lightingCanonical.effects = store
	d.lightingCanonical.resolver = corruptResolver{store, rt.Resolver}
	d.lightingMu.Lock()
	_, err := d.lightingFrameLocked()
	d.lightingMu.Unlock()
	if err == nil {
		t.Fatal("ordinary catalogue corruption validation weakened")
	}
	frames := make(chan []byte, 2)
	d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
	readyLighting(d)
	requireLighting(t, d.setUserRGBOff(true))
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{0, 127, 0, 127, 0, 127}) {
		t.Fatal("corrupt unrelated Gradient prevented black packet", got)
	}
	if err := d.SetLightingEffect("gradient"); err == nil {
		t.Fatal("invalid Gradient selected")
	}
	if err := d.SetLightingEffectSettings("gradient", bad); err == nil {
		t.Fatal("invalid customization accepted")
	}
}

func TestTimerIgnoresUnrelatedInputAndOwnerChanges(t *testing.T) {
	for _, change := range []string{"sniper", "color", "owner"} {
		t.Run(change, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			readyLighting(d)
			if d.mutateInputProfile(false, func(p *DeviceProfile) error { p.SleepMode = 5; return nil }) != 1 {
				t.Fatal("timer proposal")
			}
			entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan uint8, 1)
			var once sync.Once
			defer once.Do(func() { close(release) })
			durations := make(chan uint32, 2)
			d.lightingDPIWrite = func(command, data []byte) error {
				if bytes.Equal(command, cmdSleep[0]) {
					close(entered)
					waitDone(t, release)
				}
				if bytes.Equal(command, cmdSleep[1]) {
					sendWithin(t, durations, binary.LittleEndian.Uint32(data))
				}
				return nil
			}
			go func() { sendWithin(t, finished, d.programCanonicalSleep()) }()
			waitDone(t, entered)
			changed := make(chan struct{})
			if change == "sniper" {
				go func() { d.SetMouseSniperMode(true); close(changed) }()
				waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.SniperMode })
			} else if change == "color" {
				if d.SaveMouseDpiColors(rgb.Color{}, map[int]rgb.Color{1: {Red: 10}}) != 1 {
					t.Fatal("color")
				}
				close(changed)
			} else {
				if d.ProcessSetRgbCluster(true) != 1 {
					t.Fatal("owner")
				}
				close(changed)
			}
			once.Do(func() { close(release) })
			if receiveWithin(t, finished) != 1 || receiveWithin(t, durations) != 300000 {
				t.Fatal("unrelated mutation cancelled timer")
			}
			waitDone(t, changed)
		})
	}
}

func TestTimerRejectsReplacementSettingProfileAndConnection(t *testing.T) {
	for _, change := range []string{"timer", "profile", "connection"} {
		t.Run(change, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			readyLighting(d)
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			durations := make(chan uint32, 3)
			var first atomic.Bool
			d.lightingDPIWrite = func(command, data []byte) error {
				if bytes.Equal(command, cmdSleep[0]) && !first.Swap(true) {
					close(entered)
					waitDone(t, release)
				}
				if bytes.Equal(command, cmdSleep[1]) {
					sendWithin(t, durations, binary.LittleEndian.Uint32(data))
				}
				return nil
			}
			finished := make(chan uint8, 1)
			go func() { sendWithin(t, finished, d.programCanonicalSleep()) }()
			waitDone(t, entered)
			switched := make(chan struct{})
			if change == "timer" {
				if d.mutateInputProfile(false, func(p *DeviceProfile) error { p.SleepMode = 30; return nil }) != 1 {
					t.Fatal("new timer")
				}
				close(switched)
			} else if change == "connection" {
				d.SetConnected(false)
				reconnectFixtureWithoutOutput(d)
				close(switched)
			} else {
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
			once.Do(func() { close(release) })
			if receiveWithin(t, finished) != 0 || len(durations) != 0 {
				t.Fatal("retired timer wrote final duration")
			}
			waitDone(t, switched)
			if change == "timer" {
				if d.programCanonicalSleep() != 1 || receiveWithin(t, durations) != 1800000 {
					t.Fatal("replacement timer failed")
				}
			}
		})
	}
}

func TestObsoleteStoredAssignmentDoesNotBlockOneWake(t *testing.T) {
	d := profileSaveFixture(t, "default")
	d.SetConnected(false)
	bindings := defaultWirelessAssignments()
	bindings[4096] = inputmanager.KeyAssignment{Name: "obsolete custom", ActionType: 3, ActionCommand: 42}
	path := filepath.Join(pwd, "database", "key-assignments", d.DeviceProfile.KeyAssignmentHash+".json")
	data, err := json.Marshal(bindings)
	requireLighting(t, err)
	requireLighting(t, os.WriteFile(path, data, 0600))
	writes := 0
	d.lightingAssignmentWrite = func(string, []byte) error { writes++; return errors.New("must not rewrite") }
	requireLighting(t, d.loadCanonicalAssignments())
	var payloads atomic.Int32
	d.lightingDPIWrite = func(command, data []byte) error {
		if bytes.Equal(command, cmdWrite) {
			payloads.Add(1)
		}
		return nil
	}
	d.lightingReportWrite = func(report []byte) (int, error) { return len(report), nil }
	d.Connect()
	d.stopLighting()
	d.lightingMu.Lock()
	connected := d.Connected
	session := d.sessionLocked()
	d.lightingMu.Unlock()
	if !connected || payloads.Load() != 0 || writes != 0 || !reflect.DeepEqual(bindings, d.inputAssignmentsSnapshot()) {
		t.Fatal("invalid stored map blocked wake or was discarded")
	}
	if !errors.Is(d.programCanonicalAssignments(), errInvalidAssignments) {
		t.Fatal("mapping error hidden")
	}
	after, err := os.ReadFile(path)
	requireLighting(t, err)
	if !bytes.Equal(after, data) {
		t.Fatal("invalid custom data overwritten")
	}
	d.SetConnected(false)
	if d.replayConnectionAssignments(session) {
		t.Fatal("invalid-map skip admitted stale session")
	}
}

func TestAssignmentOpenTimeoutCleansEndpointBeforeRetry(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.SetConnected(false)
	d.lightingDPIWrite = nil
	open := false
	opens, closes, payloads := 0, 0, 0
	d.lightingReportWrite = func(report []byte) (int, error) {
		if bytes.Equal(report[2:5], cmdOpenWriteEndpoint) {
			opens++
			if open {
				return 0, errors.New("endpoint wedged")
			}
			open = true
			if opens == 1 {
				return 0, errors.New("accepted open; reply timed out")
			}
		}
		if bytes.Equal(report[2:5], cmdCloseEndpoint) {
			closes++
			open = false
		}
		if bytes.Equal(report[2:4], cmdWrite) {
			payloads++
		}
		return len(report), nil
	}
	d.Connect()
	d.stopLighting()
	d.lightingMu.Lock()
	connected := d.Connected
	d.lightingMu.Unlock()
	if !connected || open || opens != 2 || closes != 2 || payloads != 1 {
		t.Fatal("timeout left endpoint wedged", connected, open, opens, closes, payloads)
	}
}
func TestOpenFailurePreservesErrorAndCannotCleanReplacement(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "close-fails", true: "replacement"}[replace], func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			readyLighting(d)
			failure := errors.New("original open timeout")
			closed := 0
			d.lightingDPIWrite = func(command, data []byte) error {
				if bytes.Equal(command, cmdOpenWriteEndpoint) {
					if replace {
						d.RebindReceiver(&common.Slipstream{}, 12)
						reconnectFixtureWithoutOutput(d)
					}
					return failure
				}
				if bytes.Equal(command, cmdCloseEndpoint) {
					closed++
					return errors.New("secondary cleanup failure")
				}
				return nil
			}
			if err := d.programCanonicalAssignments(); !errors.Is(err, failure) {
				t.Fatal("open error hidden", err)
			}
			if (replace && closed != 0) || (!replace && closed != 1) {
				t.Fatal("cleanup used wrong session", closed)
			}
		})
	}
}

func TestOptionalConfigurationTimeoutDoesNotBlockWake(t *testing.T) {
	for _, setting := range []string{"gesture", "tilt", "angle"} {
		t.Run(setting, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			d.SetConnected(false)
			d.lightingDPIWrite = nil
			failed, leds, bindings := 0, 0, 0
			target := cmdMultiGesture
			if setting == "tilt" {
				target = cmdLeftTilt
			}
			if setting == "angle" {
				target = cmdAngleSnapping
			}
			d.lightingReportWrite = func(report []byte) (int, error) {
				if bytes.Equal(report[2:2+len(target)], target) {
					failed++
					return 0, errors.New("optional setting reply timeout")
				}
				if bytes.Equal(report[2:5], cmdOpenEndpoint) {
					leds++
				}
				if bytes.Equal(report[2:4], cmdWrite) {
					bindings++
				}
				return len(report), nil
			}
			d.Connect()
			d.stopLighting()
			d.lightingMu.Lock()
			connected := d.Connected
			d.lightingMu.Unlock()
			expected := 1
			if setting == "angle" {
				expected = 3
			}
			if !connected || failed != expected || leds != 1 || bindings != 1 {
				t.Fatal("optional configuration hardened wake", connected, failed, leds, bindings)
			}
		})
	}
}
