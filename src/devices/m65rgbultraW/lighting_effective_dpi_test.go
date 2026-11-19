package m65rgbultraW

import (
	"LumenForge/src/inputmanager"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
)

type dpiAxisWrite struct {
	Command [3]byte
	Value   uint16
}

func recordDPIAxis(t *testing.T, command, data []byte) dpiAxisWrite {
	t.Helper()
	if len(command) != 3 || len(data) != 2 {
		t.Errorf("invalid DPI wire layout: %v %v", command, data)
		return dpiAxisWrite{}
	}
	return dpiAxisWrite{[3]byte{command[0], command[1], command[2]}, binary.LittleEndian.Uint16(data)}
}
func TestDPIAxisWireIdentifiers(t *testing.T) {
	if !bytes.Equal(cmdSetDpi[0], []byte{1, 0x21, 0}) || !bytes.Equal(cmdSetDpi[1], []byte{1, 0x22, 0}) {
		t.Fatal("X/Y wire identifiers changed", cmdSetDpi)
	}
}
func TestSniperRegularSelectionPreservesBothAxesAndReleasesToNewStage(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	writes := []dpiAxisWrite{}
	selected := false
	d.lightingDPIWrite = func(command, data []byte) error {
		writes = append(writes, recordDPIAxis(t, command, data))
		if bytes.Equal(command, cmdSetDpi[0]) && binary.LittleEndian.Uint16(data) == 200 && !selected {
			selected = true
			if d.SelectMouseDPIStage(2) != 1 {
				t.Error("future regular selection failed")
			}
		}
		return nil
	}
	if d.SetMouseSniperMode(true) != 1 {
		t.Fatal("Sniper activation")
	}
	expected := []dpiAxisWrite{{[3]byte{1, 0x21, 0}, 200}, {[3]byte{1, 0x22, 0}, 200}}
	if !reflect.DeepEqual(writes, expected) {
		t.Fatal("regular selection cancelled Sniper Y", writes)
	}
	d.lightingMu.Lock()
	active, stage := d.SniperMode, d.DeviceProfile.Profile
	d.lightingMu.Unlock()
	if !active || stage != 2 {
		t.Fatal("future stage not preserved")
	}
	data, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	var persisted DeviceProfile
	requireLighting(t, json.Unmarshal(data, &persisted))
	if persisted.Profile != 2 {
		t.Fatal("stage not persisted")
	}
	if d.SetMouseSniperMode(false) != 1 {
		t.Fatal("Sniper release")
	}
	expected = append(expected, dpiAxisWrite{[3]byte{1, 0x21, 0}, 1200}, dpiAxisWrite{[3]byte{1, 0x22, 0}, 1200})
	if !reflect.DeepEqual(writes, expected) {
		t.Fatal("release did not program newly selected regular stage on X/Y", writes)
	}
}
func TestSniperValueChangeSupersedesEffectiveDPITransaction(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	writes := []dpiAxisWrite{}
	d.lightingDPIWrite = func(command, data []byte) error {
		writes = append(writes, recordDPIAxis(t, command, data))
		if bytes.Equal(command, cmdSetDpi[0]) && binary.LittleEndian.Uint16(data) == 200 {
			if d.mutateInputProfile(false, func(p *DeviceProfile) error { s := p.Profiles[5]; s.Value = 300; p.Profiles[5] = s; return nil }) != 1 {
				t.Error("Sniper value change")
			}
		}
		return nil
	}
	if d.SetMouseSniperMode(true) != 0 {
		t.Fatal("superseded partial apply reported success")
	}
	d.programCanonicalDPI()
	expected := []dpiAxisWrite{{[3]byte{1, 0x21, 0}, 200}, {[3]byte{1, 0x21, 0}, 300}, {[3]byte{1, 0x22, 0}, 300}}
	if !reflect.DeepEqual(writes, expected) {
		t.Fatal("stale Sniper transaction survived", writes)
	}
}
func TestActiveSnapshotSaveDoesNotCancelEffectiveDPI(t *testing.T) {
	for _, scenario := range []string{"snapshot", "metadata", "lighting"} {
		t.Run(scenario, func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			readyLighting(d)
			if d.mutateInputProfile(false, func(p *DeviceProfile) error { s := p.Profiles[1]; s.Value = 1200; p.Profiles[1] = s; return nil }) != 1 {
				t.Fatal("prepare1200")
			}
			d.lightingMu.Lock()
			revision := d.dpiValueRevision
			pointer := d.DeviceProfile
			d.lightingMu.Unlock()
			writes := []dpiAxisWrite{}
			d.lightingDPIWrite = func(command, data []byte) error {
				writes = append(writes, recordDPIAxis(t, command, data))
				if bytes.Equal(command, cmdSetDpi[0]) {
					if scenario == "metadata" && d.mutateInputProfile(false, func(p *DeviceProfile) error { p.Label = "updated label"; return nil }) != 1 {
						t.Error("metadata save")
					}
					if scenario == "lighting" {
						if err := d.SetLightingEffect("mouse"); err != nil {
							t.Error(err)
						}
					}
					if d.SaveUserProfile("default") != 1 {
						t.Error("active re-save")
					}
				}
				return nil
			}
			d.programCanonicalDPI()
			expected := []dpiAxisWrite{{[3]byte{1, 0x21, 0}, 1200}, {[3]byte{1, 0x22, 0}, 1200}}
			d.lightingMu.Lock()
			unchanged := d.dpiValueRevision == revision && d.DeviceProfile == pointer && d.UserProfiles["default"] == pointer
			d.lightingMu.Unlock()
			if !unchanged || !reflect.DeepEqual(writes, expected) {
				t.Fatal("profile snapshot cancelled Y or changed identity", writes)
			}
		})
	}
}
func TestEffectiveDPIRevisionClassification(t *testing.T) {
	for _, test := range []struct {
		name        string
		sniper      bool
		change      func(*DeviceProfile)
		invalidates bool
	}{
		{"A-regular-effective", false, func(p *DeviceProfile) { p.Profile = 2 }, true},
		{"B-future-regular-under-sniper", true, func(p *DeviceProfile) { p.Profile = 2 }, false},
		{"B-inactive-regular-value", false, func(p *DeviceProfile) { s := p.Profiles[2]; s.Value = 1400; p.Profiles[2] = s }, false},
		{"A-sniper-value", true, func(p *DeviceProfile) { s := p.Profiles[5]; s.Value = 300; p.Profiles[5] = s }, true},
		{"C-color", false, func(p *DeviceProfile) { p.Profiles[1].Color.Red = 42 }, false},
		{"D-metadata", false, func(p *DeviceProfile) { p.Label = "renamed" }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			d.lightingMu.Lock()
			d.SniperMode = test.sniper
			revision := d.dpiValueRevision
			d.lightingMu.Unlock()
			if d.mutateInputProfile(false, func(p *DeviceProfile) error { test.change(p); return nil }) != 1 {
				t.Fatal("mutation failed")
			}
			d.lightingMu.Lock()
			invalidated := d.dpiValueRevision != revision
			d.lightingMu.Unlock()
			if invalidated != test.invalidates {
				t.Fatal("wrong effective-DPI invalidation", invalidated)
			}
		})
	}
	// F: lifecycle uses the independent connection epoch, not a DPI-value edit.
	d, _ := lightingFixture(t, true)
	d.lightingMu.Lock()
	dpi, connection := d.dpiValueRevision, d.connectionGeneration
	d.lightingMu.Unlock()
	d.SetConnected(false)
	d.lightingMu.Lock()
	valid := d.dpiValueRevision == dpi && d.connectionGeneration != connection
	d.lightingMu.Unlock()
	if !valid {
		t.Fatal("connection overloaded DPI value revision")
	}
}

func TestMissingForwardBindingRepairAndOneWake(t *testing.T) {
	expectedPayload := []byte{8, 0, 0, 0, 1, 0, 0, 1, 0, 0, 0, 0}
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "persisted-repair", true: "deferred-on-persistence-failure"}[fail], func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			d.lightingMu.Lock()
			d.cancelLightingLocked()
			d.KeyAssignment = nil
			d.lightingMu.Unlock()
			d.SetConnected(false)
			bindings := defaultWirelessAssignments()
			delete(bindings, 8)
			for key, value := range bindings {
				if key != 1 {
					value.Default = false
					value.ActionType = 3
					value.ActionCommand = uint16(key + 5)
					value.ActionHold = true
					bindings[key] = value
				}
			}
			path := filepath.Join(pwd, "database", "key-assignments", d.DeviceProfile.KeyAssignmentHash+".json")
			data, err := json.Marshal(bindings)
			requireLighting(t, err)
			requireLighting(t, os.WriteFile(path, data, 0600))
			var repairWrites atomic.Int32
			failure := errors.New("repair persistence denied")
			d.lightingAssignmentWrite = func(path string, data []byte) error {
				repairWrites.Add(1)
				if fail {
					return failure
				}
				return writeLightingProfile(path, data)
			}
			err = d.loadCanonicalAssignments()
			if fail {
				if !errors.Is(err, failure) || !errors.Is(err, errAssignmentRepairNotPersisted) {
					t.Fatal("repair failure hidden", err)
				}
			} else {
				requireLighting(t, err)
			}
			loaded := d.inputAssignmentsSnapshot()
			for key, value := range bindings {
				if !reflect.DeepEqual(loaded[key], value) {
					t.Fatal("custom binding changed", key)
				}
			}
			if fail {
				if len(loaded) != 11 {
					t.Fatal("unpersisted repair published")
				}
			} else if len(loaded) != 12 || !reflect.DeepEqual(loaded[8], defaultWirelessAssignments()[8]) {
				t.Fatal("missing Forward was not the only repair")
			}
			payloads := make(chan []byte, 3)
			d.lightingDPIWrite = func(command, data []byte) error {
				if bytes.Equal(command, cmdWrite) {
					sendWithin(t, payloads, append([]byte(nil), data...))
				}
				return nil
			}
			d.lightingReportWrite = func(report []byte) (int, error) { return len(report), nil }
			done := make(chan struct{})
			go func() { d.Connect(); close(done) }()
			waitDone(t, done)
			d.stopLighting()
			d.lightingMu.Lock()
			connected, deferred := d.Connected, d.assignmentReplayDeferred
			d.lightingMu.Unlock()
			expectedWrites := int32(1)
			if fail {
				expectedWrites = 2
			} // initial load plus one reconnect retry
			if !connected || deferred != fail || repairWrites.Load() != expectedWrites {
				t.Fatal("recoverable assignment blocked wake or used wrong repair retry count", connected, deferred, repairWrites.Load())
			}
			if fail {
				if len(payloads) != 0 {
					t.Fatal("unpersisted repairs programmed")
				}
				after, err := os.ReadFile(path)
				requireLighting(t, err)
				if !bytes.Equal(data, after) {
					t.Fatal("failed repair changed disk")
				}
				// A later successful load/replay recovers the mapping while
				// already connected; it does not require another receiver wake.
				d.lightingMu.Lock()
				epoch := d.connectionGeneration
				d.lightingMu.Unlock()
				d.lightingAssignmentWrite = writeLightingProfile
				requireLighting(t, d.loadCanonicalAssignments())
				requireLighting(t, d.programCanonicalAssignments())
				if recoveredPayload := receiveWithin(t, payloads); !bytes.Equal(recoveredPayload, expectedPayload) {
					t.Fatal("deferred repair lost custom mapping", recoveredPayload, expectedPayload)
				}
				d.lightingMu.Lock()
				recovered := !d.assignmentReplayDeferred && d.Connected && d.connectionGeneration == epoch
				d.lightingMu.Unlock()
				if !recovered {
					t.Fatal("deferred mapping did not recover in current connection")
				}
			} else {
				payload := receiveWithin(t, payloads)
				// Keys1,2,4,8,16,32,64,128: only mandatory Left and repaired Forward use
				// native defaults; every existing custom/default bit is preserved.
				if !bytes.Equal(payload, expectedPayload) {
					t.Fatal("repaired/custom mapping was not replayed", payload)
				}
				stored, err := os.ReadFile(path)
				requireLighting(t, err)
				var repaired map[int]inputmanager.KeyAssignment
				requireLighting(t, json.Unmarshal(stored, &repaired))
				if !reflect.DeepEqual(repaired, loaded) {
					t.Fatal("repair not durable")
				}
			}
		})
	}
}
