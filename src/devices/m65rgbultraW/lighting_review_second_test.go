package m65rgbultraW

import (
	"LumenForge/src/common"
	"LumenForge/src/rgb"
	"LumenForge/src/stats"
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

func TestAssignmentLoaderWritesOnlyChangedBindings(t *testing.T) {
	for _, repair := range []string{"none", "tilt", "left"} {
		t.Run(repair, func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			bindings := defaultWirelessAssignments()
			if repair == "tilt" {
				delete(bindings, 2048)
			}
			if repair == "left" {
				left := bindings[1]
				left.Default = false
				bindings[1] = left
			}
			path := filepath.Join(pwd, "database", "key-assignments", d.DeviceProfile.KeyAssignmentHash+".json")
			data, err := json.Marshal(bindings)
			requireLighting(t, err)
			requireLighting(t, os.WriteFile(path, data, 0600))
			writes := 0
			failure := errors.New("readable backing, writes fail")
			d.lightingAssignmentWrite = func(string, []byte) error { writes++; return failure }
			err = d.loadCanonicalAssignments()
			if repair == "none" {
				if err != nil || writes != 0 || !reflect.DeepEqual(bindings, d.inputAssignmentsSnapshot()) {
					t.Fatal("unchanged valid12 bindings required a write", err, writes)
				}
			} else {
				if !errors.Is(err, failure) || writes != 1 || !reflect.DeepEqual(bindings, d.inputAssignmentsSnapshot()) || !d.assignmentReplayDeferred {
					t.Fatal("failed real repair was hidden/published", err, writes)
				}
			}
		})
	}
}

func TestFailedSetupRestoresOnlySoftwareModeOwnedBySameBinding(t *testing.T) {
	for _, scenario := range []string{"entered", "mode-failed", "rebind", "dirty"} {
		t.Run(scenario, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			d.SetConnected(false)
			var restore atomic.Int32
			receiver := d.dev
			d.lightingReportWrite = func(report []byte) (int, error) {
				if bytes.Equal(report[2:6], cmdHardwareMode) {
					restore.Add(1)
					if report[1] != 9 {
						t.Error("wrong restore endpoint")
					}
				}
				if scenario == "mode-failed" && bytes.Equal(report[2:6], cmdSoftwareMode) {
					return 0, errors.New("mode failure")
				}
				if bytes.Equal(report[2:5], cmdOpenEndpoint) {
					return 0, errors.New("permanent endpoint failure")
				}
				return len(report), nil
			}
			d.Connect()
			d.lightingMu.Lock()
			connected := d.Connected
			d.lightingMu.Unlock()
			if connected {
				t.Fatal("failed setup published Connected")
			}
			if scenario == "rebind" {
				d.RebindReceiver(&common.Slipstream{}, 12)
			}
			if scenario == "dirty" {
				d.StopDirty()
			} else {
				d.StopInternal()
			}
			expected := int32(0)
			// A failed reply can follow an accepted software-mode write.
			if scenario == "entered" || scenario == "mode-failed" {
				expected = 1
			}
			if restore.Load() != expected {
				t.Fatal("wrong hardware-mode restoration", restore.Load(), expected)
			}
			if scenario != "rebind" && d.dev != receiver {
				t.Fatal("child replaced/closed parent transport")
			}
		})
	}
}

func TestColorOnlyEditDoesNotCancelDPIValueTransaction(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	// Use an external Logo owner so the cosmetic edit can update its indicator
	// independently, rather than starting a new Native worker.
	if d.ProcessSetRgbCluster(true) != 1 {
		t.Fatal("owner")
	}
	var values []dpiAxisWrite
	colorDone := make(chan uint8, 1)
	d.lightingDPIWrite = func(command, data []byte) error {
		if bytes.Equal(command, cmdSetDpi[0]) || bytes.Equal(command, cmdSetDpi[1]) {
			values = append(values, recordDPIAxis(t, command, data))
			if bytes.Equal(command, cmdSetDpi[0]) {
				// Model a concurrent request: persistence can finish during X/Y,
				// while its indicator output waits for that transaction's lock.
				go func() {
					sendWithin(t, colorDone, d.SaveMouseDpiColors(rgb.Color{}, map[int]rgb.Color{1: {Red: 10, Green: 20, Blue: 30}}))
				}()
				waitCondition(t, func() bool {
					d.lightingMu.Lock()
					defer d.lightingMu.Unlock()
					return d.DeviceProfile.Profiles[1].Color.Red == 10
				})
			}
		}
		return nil
	}
	if d.SaveMouseDPI(map[int]uint16{1: 1200}) != 1 {
		t.Fatal("save DPI")
	}
	if receiveWithin(t, colorDone) != 1 {
		t.Fatal("color save")
	}
	if !reflect.DeepEqual(values, []dpiAxisWrite{{[3]byte{1, 0x21, 0}, 1200}, {[3]byte{1, 0x22, 0}, 1200}}) {
		t.Fatal("cosmetic edit rejected Y", values)
	}
}

func TestSecondDPIValueRevisionRejectsOldAxes(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	values := []dpiAxisWrite{}
	d.lightingDPIWrite = func(command, data []byte) error {
		if bytes.Equal(command, cmdSetDpi[0]) || bytes.Equal(command, cmdSetDpi[1]) {
			values = append(values, recordDPIAxis(t, command, data))
			if bytes.Equal(command, cmdSetDpi[0]) && binary.LittleEndian.Uint16(data) == 800 {
				if d.mutateInputProfile(false, func(p *DeviceProfile) error {
					stage := p.Profiles[1]
					stage.Value = 1600
					p.Profiles[1] = stage
					return nil
				}) != 1 {
					t.Error("replace DPI values")
				}
			}
		}
		return nil
	}
	d.programCanonicalDPI()
	if !reflect.DeepEqual(values, []dpiAxisWrite{{[3]byte{1, 0x21, 0}, 800}}) {
		t.Fatal("old value Y survived", values)
	}
	d.programCanonicalDPI()
	if !reflect.DeepEqual(values, []dpiAxisWrite{{[3]byte{1, 0x21, 0}, 800}, {[3]byte{1, 0x21, 0}, 1600}, {[3]byte{1, 0x22, 0}, 1600}}) {
		t.Fatal("replacement value did not program both axes", values)
	}
}

func TestSuccessfulOverwriteCleansOnlyUnreferencedNamedAssignments(t *testing.T) {
	for _, scenario := range []string{"unreferenced", "referenced", "shared-default", "profile-failure", "assignment-failure"} {
		t.Run(scenario, func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			if d.SaveUserProfile("named") != 1 {
				t.Fatal("initial named snapshot")
			}
			oldHash := d.UserProfiles["named"].KeyAssignmentHash
			oldPath := filepath.Join(pwd, "database", "key-assignments", oldHash+".json")
			before, err := os.ReadFile(oldPath)
			requireLighting(t, err)
			if scenario == "referenced" {
				copy := cloneLightingProfile(*d.UserProfiles["Z"])
				copy.KeyAssignmentHash = oldHash
				d.UserProfiles["Z"] = &copy
			}
			if scenario == "shared-default" {
				d.keyAssignmentFile = "/database/key-assignments/" + oldHash + ".json"
			}
			if scenario == "profile-failure" {
				d.lightingProfileWrite = func(string, []byte) error { return errors.New("profile failure") }
			}
			if scenario == "assignment-failure" {
				d.lightingAssignmentWrite = func(string, []byte) error { return errors.New("assignment failure") }
			}
			result := d.SaveUserProfile("named")
			if scenario == "profile-failure" || scenario == "assignment-failure" {
				if result != 0 || d.UserProfiles["named"].KeyAssignmentHash != oldHash {
					t.Fatal("failed replacement published")
				}
			} else if result != 1 || d.UserProfiles["named"].KeyAssignmentHash == oldHash {
				t.Fatal("no replacement")
			}
			after, err := os.ReadFile(oldPath)
			if scenario == "unreferenced" {
				if !os.IsNotExist(err) {
					t.Fatal("unreferenced snapshot leaked", err)
				}
			} else if err != nil || !bytes.Equal(before, after) {
				t.Fatal("live/shared/failed snapshot removed", err)
			}
		})
	}
}

func TestOldConnectionBatteryCannotPublishAfterRebind(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.SetConnected(false)
	d.ModifyBatteryLevel(77)
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	d.lightingReportWrite = func(report []byte) (int, error) {
		if bytes.Equal(report[2:4], cmdBatteryLevel) {
			close(entered)
			waitDone(t, release)
		}
		return len(report), nil
	}
	go func() { d.Connect(); close(finished) }()
	waitDone(t, entered)
	d.RebindReceiver(&common.Slipstream{}, 12)
	once.Do(func() { close(release) })
	waitDone(t, finished)
	d.lightingMu.Lock()
	level := d.BatteryLevel
	session := d.sessionLocked()
	d.lightingMu.Unlock()
	if level != 77 || stats.GetBatteryStats()[d.Serial].Level != 77 {
		t.Fatal("stale battery published", level)
	}
	if !d.publishConnectionBattery(session, 42) {
		t.Fatal("current query rejected")
	}
	d.lightingMu.Lock()
	level = d.BatteryLevel
	d.lightingMu.Unlock()
	if level != 42 || stats.GetBatteryStats()[d.Serial].Level != 42 {
		t.Fatal("current battery not published", level)
	}
}

func TestWakeRetriesRequiredAssignmentReplay(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.SetConnected(false)
	d.lightingDPIWrite = nil
	var payloads, closes atomic.Int32
	d.lightingReportWrite = func(report []byte) (int, error) {
		if bytes.Equal(report[2:4], cmdWrite) {
			d.lightingMu.Lock()
			premature := d.Connected || d.lightingHardwareReady
			d.lightingMu.Unlock()
			if premature {
				t.Error("assignment replay ran after full readiness publication")
			}
			if payloads.Add(1) == 1 {
				return 0, errors.New("transient mapping payload")
			}
		}
		if bytes.Equal(report[2:5], cmdCloseEndpoint) {
			closes.Add(1)
		}
		return len(report), nil
	}
	d.Connect()
	d.stopLighting()
	d.lightingMu.Lock()
	ready := d.Connected && d.lightingHardwareReady
	d.lightingMu.Unlock()
	if !ready || payloads.Load() != 2 || closes.Load() != 2 {
		t.Fatal("single wake did not restore mappings", ready, payloads.Load(), closes.Load())
	}
}

func TestAssignmentReplayRetriesFreshSnapshotAndHonorsRetirement(t *testing.T) {
	for _, transition := range []string{"refresh", "disconnect", "rebind", "stop"} {
		t.Run(transition, func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			d.KeyAssignment = defaultWirelessAssignments()
			d.SetConnected(false)
			d.lightingMu.Lock()
			d.softwareModeGeneration = d.connectionGeneration
			session := d.sessionLocked()
			d.lightingMu.Unlock()
			paused, release, finished := make(chan struct{}), make(chan struct{}), make(chan bool, 1)
			var once sync.Once
			defer once.Do(func() { close(release) })
			var payloads atomic.Int32
			var second []byte
			d.lightingDPIWrite = func(command, data []byte) error {
				if bytes.Equal(command, cmdWrite) {
					if payloads.Add(1) == 1 {
						return errors.New("transient")
					}
					second = append([]byte(nil), data...)
				}
				return nil
			}
			d.assignmentBeforeRetry = func(connectionSession) { close(paused); waitDone(t, release) }
			go func() { sendWithin(t, finished, d.replayConnectionAssignments(session)) }()
			waitDone(t, paused)
			if transition == "refresh" {
				assignment := d.KeyAssignment[8]
				assignment.Default = false
				if d.UpdateDeviceKeyAssignment(8, assignment) != 1 {
					t.Fatal("input change")
				}
			} else if transition == "disconnect" {
				d.SetConnected(false)
			} else if transition == "rebind" {
				d.RebindReceiver(&common.Slipstream{}, 12)
			} else {
				d.StopDirty()
			}
			once.Do(func() { close(release) })
			success := receiveWithin(t, finished)
			if transition == "refresh" {
				if !success || payloads.Load() != 2 || len(second) < 8 || second[headerWriteSize+3] != 0 {
					t.Fatal("retry did not use fresh mapping", success, payloads.Load(), second)
				}
			} else if success || payloads.Load() != 1 {
				t.Fatal("old assignment replay survived retirement", success, payloads.Load())
			}
		})
	}
}

func TestLegacyAliasAndLiteralNamesUseDifferentPhysicalFiles(t *testing.T) {
	d := profileSaveFixture(t, "default")
	requireLighting(t, os.Remove(d.UserProfiles["Z"].Path))
	delete(d.UserProfiles, "Z")
	for _, logical := range []string{"Z", "Work-Night"} {
		legacy := cloneLightingProfile(*d.DeviceProfile)
		// Only explicit identity establishes a historical physical-path alias.
		legacy.ProfileName = logical
		legacy.Active = false
		legacy.Label = "legacy " + logical
		legacy.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-"+logical+"-preserved.json")
		requireLighting(t, d.persistLightingProfile(legacy))
	}
	d.loadDeviceProfiles()
	for _, logical := range []string{"Z", "Work-Night"} {
		legacy := d.UserProfiles[logical]
		if legacy == nil {
			t.Fatal("alias missing", logical)
		}
		data, err := os.ReadFile(legacy.Path)
		requireLighting(t, err)
		if d.SaveUserProfile(logical+"-preserved") != 1 {
			t.Fatal("literal save")
		}
		literal := d.UserProfiles[logical+"-preserved"]
		if literal.Path == legacy.Path || literal.KeyAssignmentHash == legacy.KeyAssignmentHash {
			t.Fatal("physical path or linkage collision")
		}
		after, err := os.ReadFile(legacy.Path)
		requireLighting(t, err)
		if !bytes.Equal(data, after) {
			t.Fatal("legacy data overwritten")
		}
	}
	if d.ChangeDeviceProfile("Z-preserved") != 1 {
		t.Fatal("activate literal")
	}
	d.stopLighting()
	// Restart on the SAME serial and backing directory.
	restarted := &Device{Serial: d.Serial, Product: d.Product}
	restarted.loadDeviceProfiles()
	for _, logical := range []string{"Z", "Work-Night"} {
		alias, literal := restarted.UserProfiles[logical], restarted.UserProfiles[logical+"-preserved"]
		if alias == nil || literal == nil || alias.Path == literal.Path || alias.Label == literal.Label || literal.KeyAssignmentHash == "" {
			t.Fatal("distinct identities/settings lost", logical, restarted.ProfileOrder)
		}
	}
	if restarted.DeviceProfile != restarted.UserProfiles["Z-preserved"] || !restarted.DeviceProfile.Active {
		t.Fatal("active identity lost")
	}
}
