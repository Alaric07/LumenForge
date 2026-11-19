package m65rgbultraW

import (
	"LumenForge/src/common"
	"LumenForge/src/inputmanager"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestAssignmentEditRemainsBoundToPublishedProfile(t *testing.T) {
	d := profileSaveFixture(t, "default")
	d.KeyAssignment = defaultWirelessAssignments()
	readyLighting(d)
	originalProfile := d.DeviceProfile
	profileB := defaultWirelessAssignments()
	data, _ := json.Marshal(profileB)
	requireLighting(t, os.WriteFile(filepath.Join(pwd, "database", "key-assignments", "inactiveassign.json"), data, 0600))
	var activations []byte
	var packets [][]byte
	var recordsMu sync.Mutex
	d.lightingDPIWrite = func(cmd, data []byte) error {
		recordsMu.Lock()
		defer recordsMu.Unlock()
		if bytes.Equal(cmd, cmdActivateLeftTilt) {
			activations = append(activations, data[0])
		}
		if bytes.Equal(cmd, cmdWrite) {
			packets = append(packets, append([]byte(nil), data...))
		}
		return nil
	}
	d.deviceLock.Lock()
	var release sync.Once
	defer release.Do(d.deviceLock.Unlock)
	edited := d.inputAssignmentsSnapshot()[1024]
	edited.Default = false
	edited.ActionType = 3
	edited.ActionCommand = 42
	applied := make(chan uint8, 1)
	go func() { sendWithin(t, applied, d.UpdateDeviceKeyAssignment(1024, edited)) }()
	waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return !d.KeyAssignment[1024].Default })
	switched := make(chan uint8, 1)
	go func() { sendWithin(t, switched, d.ChangeDeviceProfile("Z")) }()
	waitCondition(t, func() bool {
		d.lightingMu.Lock()
		defer d.lightingMu.Unlock()
		return d.DeviceProfile == d.UserProfiles["Z"]
	})
	release.Do(d.deviceLock.Unlock)
	if receiveWithin(t, applied) != 0 {
		t.Fatal("profile A edit falsely applied to B")
	}
	if receiveWithin(t, switched) != 1 {
		t.Fatal("B replay failed")
	}
	if !reflect.DeepEqual(profileB, d.inputAssignmentsSnapshot()) {
		t.Fatal("B mapping changed by old editor")
	}
	recordsMu.Lock()
	if !reflect.DeepEqual(activations, []byte{0}) || len(packets) != 1 {
		t.Fatal("retired A payload reached B", activations, packets)
	}
	recordsMu.Unlock()
	var savedA map[int]inputmanager.KeyAssignment
	stored, err := os.ReadFile(filepath.Join(pwd, "database", "key-assignments", originalProfile.KeyAssignmentHash+".json"))
	requireLighting(t, err)
	requireLighting(t, json.Unmarshal(stored, &savedA))
	if savedA[1024].Default || savedA[1024].ActionCommand != 42 {
		t.Fatal("durable A edit was rolled back")
	}
	if d.ChangeDeviceProfile("default") != 1 {
		t.Fatal("switch back")
	}
	recordsMu.Lock()
	defer recordsMu.Unlock()
	if !reflect.DeepEqual(activations, []byte{0, 1}) || !reflect.DeepEqual(savedA, d.inputAssignmentsSnapshot()) {
		t.Fatal("return to A did not replay its saved edit", activations)
	}
}

func TestCurrentProfileAssignmentEditStillApplies(t *testing.T) {
	d := profileSaveFixture(t, "default")
	d.KeyAssignment = defaultWirelessAssignments()
	readyLighting(d)
	var activation []byte
	d.lightingDPIWrite = func(cmd, data []byte) error {
		if bytes.Equal(cmd, cmdActivateLeftTilt) {
			activation = append(activation, data[0])
		}
		return nil
	}
	edited := d.inputAssignmentsSnapshot()[1024]
	edited.Default = false
	if d.UpdateDeviceKeyAssignment(1024, edited) != 1 || !reflect.DeepEqual(activation, []byte{1}) {
		t.Fatal("current edit rejected", activation)
	}
}

func TestDPIRequestConfirmsOnlyItsPublishedValue(t *testing.T) {
	for _, mode := range []string{"select", "save", "sniper"} {
		t.Run(mode, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			readyLighting(d)
			var axes []dpiAxisWrite
			var mu sync.Mutex
			d.lightingDPIWrite = func(cmd, data []byte) error {
				if bytes.Equal(cmd, cmdSetDpi[0]) || bytes.Equal(cmd, cmdSetDpi[1]) {
					mu.Lock()
					axes = append(axes, recordDPIAxis(t, cmd, data))
					mu.Unlock()
				}
				return nil
			}
			d.deviceLock.Lock()
			var release sync.Once
			defer release.Do(d.deviceLock.Unlock)
			first, second := make(chan uint8, 1), make(chan uint8, 1)
			if mode == "select" {
				go func() { sendWithin(t, first, d.SelectMouseDPIStage(2)) }()
				waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.DeviceProfile.Profile == 2 })
				go func() { sendWithin(t, second, d.SelectMouseDPIStage(3)) }()
				waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.DeviceProfile.Profile == 3 })
			} else if mode == "save" {
				go func() { sendWithin(t, first, d.SaveMouseDPI(map[int]uint16{1: 1200})) }()
				waitCondition(t, func() bool {
					d.lightingMu.Lock()
					defer d.lightingMu.Unlock()
					return d.DeviceProfile.Profiles[1].Value == 1200
				})
				go func() { sendWithin(t, second, d.SaveMouseDPI(map[int]uint16{1: 1600})) }()
				waitCondition(t, func() bool {
					d.lightingMu.Lock()
					defer d.lightingMu.Unlock()
					return d.DeviceProfile.Profiles[1].Value == 1600
				})
			} else {
				go func() { sendWithin(t, first, d.SetMouseSniperMode(true)) }()
				waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.SniperMode })
				go func() { sendWithin(t, second, d.SetMouseSniperMode(false)) }()
				waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return !d.SniperMode })
			}
			release.Do(d.deviceLock.Unlock)
			if receiveWithin(t, first) != 0 || receiveWithin(t, second) != 1 {
				t.Fatal("old request confirmed replacement value")
			}
			value := uint16(1600)
			if mode == "sniper" {
				value = 800
			}
			mu.Lock()
			expected := []dpiAxisWrite{{[3]byte{1, 0x21, 0}, value}, {[3]byte{1, 0x22, 0}, value}}
			if !reflect.DeepEqual(axes, expected) {
				t.Fatal("stale request emitted axes", axes)
			}
			mu.Unlock()
			// The same-value public retry remains authorized by its own current capture.
			var result uint8
			if mode == "select" {
				result = d.SelectMouseDPIStage(3)
			} else if mode == "save" {
				result = d.SaveMouseDPI(map[int]uint16{1: 1600})
			} else {
				result = d.SetMouseSniperMode(false)
			}
			mu.Lock()
			defer mu.Unlock()
			if result != 1 || !reflect.DeepEqual(axes, append(expected, expected...)) {
				t.Fatal("identical current retry blocked", axes)
			}
		})
	}
}

func TestPublishedDPIRequestRejectsProfileAndReplacementSession(t *testing.T) {
	for _, change := range []string{"profile", "reconnect", "rebind"} {
		t.Run(change, func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			d.KeyAssignment = defaultWirelessAssignments()
			readyLighting(d)
			var axes []dpiAxisWrite
			var mu sync.Mutex
			d.lightingDPIWrite = func(cmd, data []byte) error {
				if bytes.Equal(cmd, cmdSetDpi[0]) || bytes.Equal(cmd, cmdSetDpi[1]) {
					mu.Lock()
					axes = append(axes, recordDPIAxis(t, cmd, data))
					mu.Unlock()
				}
				return nil
			}
			d.deviceLock.Lock()
			var release sync.Once
			defer release.Do(d.deviceLock.Unlock)
			old := make(chan uint8, 1)
			go func() { sendWithin(t, old, d.SelectMouseDPIStage(2)) }()
			waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.DeviceProfile.Profile == 2 })
			changed := make(chan struct{})
			if change == "profile" {
				go func() { d.ChangeDeviceProfile("Z"); close(changed) }()
				waitCondition(t, func() bool {
					d.lightingMu.Lock()
					defer d.lightingMu.Unlock()
					return d.DeviceProfile == d.UserProfiles["Z"]
				})
			} else {
				if change == "rebind" {
					d.RebindReceiver(&common.Slipstream{}, 13)
				} else {
					d.SetConnected(false)
				}
				reconnectFixtureWithoutOutput(d)
				close(changed)
			}
			release.Do(d.deviceLock.Unlock)
			if receiveWithin(t, old) != 0 {
				t.Fatal("retired request reported success")
			}
			waitDone(t, changed)
			mu.Lock()
			defer mu.Unlock()
			if change == "profile" {
				if len(axes) != 2 || axes[0].Value != 800 || axes[1].Value != 800 {
					t.Fatal("wrong independent profile replay", axes)
				}
			} else if len(axes) != 0 {
				t.Fatal("stale request wrote replacement connection", axes)
			}
		})
	}
}

func TestHistoricalLiftFiveReplayWithoutNewCapability(t *testing.T) {
	d := profileSaveFixture(t, "default")
	d.KeyAssignment = defaultWirelessAssignments()
	d.DeviceProfile.LiftHeight = 5
	requireLighting(t, d.persistLightingProfile(*d.DeviceProfile))
	var values []byte
	d.lightingDPIWrite = func(cmd, data []byte) error {
		if bytes.Equal(cmd, cmdLiftHeight) {
			values = append(values, data[0])
		}
		return nil
	}
	readyLighting(d)
	requireLighting(t, d.programLightingPreference("lift"))
	if _, advertised := d.LiftHeights[5]; advertised {
		t.Fatal("historical value added to capability")
	}
	before, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	for _, invalid := range []int{0, 1, 5, 8, 255} {
		if d.UpdateLiftHeight(invalid) != 0 {
			t.Fatal("new unsupported value accepted", invalid)
		}
	}
	after, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	if !bytes.Equal(before, after) || !reflect.DeepEqual(values, []byte{5}) {
		t.Fatal("invalid selection persisted/programmed")
	}
	d.SetConnected(false)
	d.lightingReportWrite = func(report []byte) (int, error) {
		if bytes.Equal(report[2:2+len(cmdLiftHeight)], cmdLiftHeight) {
			values = append(values, report[2+len(cmdLiftHeight)])
		}
		return len(report), nil
	}
	d.Connect()
	d.stopLighting()
	if !d.Connected || d.DeviceProfile.LiftHeight != 5 || !reflect.DeepEqual(values, []byte{5, 5}) {
		t.Fatal("wake lost stored 5", values)
	}
	// Profile B's advertised 2, then A's historical 5, must each replay verbatim.
	if d.ChangeDeviceProfile("Z") != 1 || d.ChangeDeviceProfile("default") != 1 {
		t.Fatal("profile activation")
	}
	if !reflect.DeepEqual(values, []byte{5, 5, 2, 5}) || d.DeviceProfile.LiftHeight != 5 {
		t.Fatal("profile replay rewrote history", values)
	}
	for _, supported := range []int{2, 3, 4, 6, 7} {
		if d.UpdateLiftHeight(supported) != 1 || values[len(values)-1] != byte(supported) {
			t.Fatal("advertised lift replay", supported, values)
		}
	}
}
