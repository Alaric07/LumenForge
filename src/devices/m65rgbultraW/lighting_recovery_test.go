package m65rgbultraW

import (
	"LumenForge/src/inputmanager"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDeferredTiltRepairRetriesOnReconnect(t *testing.T) {
	for _, again := range []bool{false, true} {
		t.Run(map[bool]string{false: "writable-next-wake", true: "fails-again"}[again], func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			d.SetConnected(false)
			custom := defaultWirelessAssignments()
			for key, value := range custom {
				if value.IsTilt {
					delete(custom, key)
					continue
				}
				if key != 1 {
					value.Default = false
					value.ActionType = 3
					value.ActionCommand = uint16(key + 9)
					value.ActionHold = true
					custom[key] = value
				}
			}
			path := filepath.Join(pwd, "database", "key-assignments", d.DeviceProfile.KeyAssignmentHash+".json")
			original, err := json.Marshal(custom)
			requireLighting(t, err)
			requireLighting(t, os.WriteFile(path, original, 0600))
			fail, writes := true, 0
			d.lightingAssignmentWrite = func(path string, data []byte) error {
				writes++
				if fail {
					return errors.New("temporary storage failure")
				}
				return writeLightingProfile(path, data)
			}
			if err := d.loadCanonicalAssignments(); !errors.Is(err, errAssignmentRepairNotPersisted) {
				t.Fatal("not deferred", err)
			}
			if !d.assignmentReplayDeferred || !reflect.DeepEqual(custom, d.inputAssignmentsSnapshot()) {
				t.Fatal("unpersisted repair changed custom state")
			}
			var payloads [][]byte
			d.lightingDPIWrite = func(cmd, data []byte) error {
				if bytes.Equal(cmd, cmdWrite) {
					payloads = append(payloads, append([]byte(nil), data...))
				}
				return nil
			}
			d.lightingReportWrite = func(report []byte) (int, error) { return len(report), nil }
			if again {
				d.Connect()
				d.stopLighting()
				if !d.Connected || !d.assignmentReplayDeferred || writes != 2 || len(payloads) != 0 {
					t.Fatal("failed retry lost availability/deferred state", writes, len(payloads))
				}
				data, err := os.ReadFile(path)
				requireLighting(t, err)
				if !bytes.Equal(data, original) || !reflect.DeepEqual(custom, d.inputAssignmentsSnapshot()) {
					t.Fatal("failed retry rewrote custom data")
				}
				d.SetConnected(false)
			}
			fail = false
			d.lightingMu.Lock()
			epoch := d.connectionGeneration
			d.lightingMu.Unlock()
			d.Connect()
			d.stopLighting()
			expectedWrites := 2
			if again {
				expectedWrites = 3
			}
			if !d.Connected || d.assignmentReplayDeferred || writes != expectedWrites || len(payloads) != 1 {
				t.Fatal("reconnect did not recover deferred repair", d.Connected, d.assignmentReplayDeferred, writes, len(payloads))
			}
			if d.connectionGeneration != epoch+1 {
				t.Fatal("repair required another connection generation")
			}
			expected := make(map[int]inputmanager.KeyAssignment, len(custom)+4)
			for key, value := range custom {
				expected[key] = value
			}
			for key, value := range defaultWirelessAssignments() {
				if value.IsTilt {
					expected[key] = value
				}
			}
			if !reflect.DeepEqual(expected, d.inputAssignmentsSnapshot()) {
				t.Fatal("retry replaced custom mappings or added unexpected entries")
			}
			var stored map[int]inputmanager.KeyAssignment
			data, err := os.ReadFile(path)
			requireLighting(t, err)
			requireLighting(t, json.Unmarshal(data, &stored))
			if !reflect.DeepEqual(stored, expected) {
				t.Fatal("repaired mapping not durable")
			}
			if !bytes.Equal(payloads[0], []byte{8, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0}) {
				t.Fatal("custom main mapping not replayed", payloads)
			}
		})
	}
}

func TestAssignmentReconnectKeepsValidAndInvalidMapsSeparate(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid", true: "unrecognized-extra"}[invalid], func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			d.SetConnected(false)
			bindings := defaultWirelessAssignments()
			if invalid {
				bindings[4096] = inputmanager.KeyAssignment{Name: "obsolete custom", ActionType: 3, ActionCommand: 42}
			}
			path := filepath.Join(pwd, "database", "key-assignments", d.DeviceProfile.KeyAssignmentHash+".json")
			data, _ := json.Marshal(bindings)
			requireLighting(t, os.WriteFile(path, data, 0600))
			writes := 0
			d.lightingAssignmentWrite = func(string, []byte) error { writes++; return errors.New("no repair expected") }
			requireLighting(t, d.loadCanonicalAssignments())
			if d.assignmentReplayDeferred {
				t.Fatal("unrecognized shape marked as repair deferral")
			}
			payloads := 0
			d.lightingDPIWrite = func(cmd, data []byte) error {
				if bytes.Equal(cmd, cmdWrite) {
					payloads++
				}
				return nil
			}
			d.lightingReportWrite = func(p []byte) (int, error) { return len(p), nil }
			d.Connect()
			d.stopLighting()
			want := 1
			if invalid {
				want = 0
			}
			if !d.Connected || writes != 0 || payloads != want || !reflect.DeepEqual(bindings, d.inputAssignmentsSnapshot()) {
				t.Fatal("valid/invalid map entered repair path", writes, payloads)
			}
		})
	}
}
func TestDeferredRepairRejectsRetiredReplaySession(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.assignmentReplayDeferred = true
	d.lightingMu.Lock()
	session := d.sessionLocked()
	d.lightingMu.Unlock()
	d.SetConnected(false)
	writes := 0
	d.lightingAssignmentWrite = func(string, []byte) error { writes++; return nil }
	if !errors.Is(d.loadCanonicalAssignmentsForSession(&session), errLightingSessionRetired) || d.replayConnectionAssignments(session) || writes != 0 {
		t.Fatal("retired session repaired/programmed assignments", writes)
	}
}
