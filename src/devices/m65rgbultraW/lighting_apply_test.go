package m65rgbultraW

import (
	"LumenForge/src/common"
	"LumenForge/src/rgb"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

func TestIndependentLiftAndButtonApplyInvalidation(t *testing.T) {
	for _, kind := range []string{"lift", "button"} {
		t.Run(kind, func(t *testing.T) {
			for _, change := range []string{"dpi", "color", "sniper", "timer", "binding", "gesture", "owner", "snapshot", "save-as", "other-preference", "new-value", "profile", "connection", "rebind", "stop"} {
				t.Run(change, func(t *testing.T) {
					d := profileSaveFixture(t, "default")
					d.KeyAssignment = defaultWirelessAssignments()
					readyLighting(d)
					command := cmdLiftHeight
					initial, next := 3, 4
					if kind == "button" {
						command = cmdButtonOptimization
						initial, next = 1, 0
					}
					update := func(value int) uint8 {
						if kind == "lift" {
							return d.UpdateLiftHeight(value)
						}
						return d.UpdateButtonOptimization(value)
					}
					entered, resume := make(chan struct{}), make(chan struct{})
					var once sync.Once
					defer once.Do(func() { close(resume) })
					var first atomic.Bool
					var values []byte
					var valuesMu sync.Mutex
					d.lightingInputBeforeWrite = func(_ connectionSession, cmd []byte) {
						if bytes.Equal(cmd, command) && !first.Swap(true) {
							close(entered)
							waitDone(t, resume)
						}
					}
					d.lightingDPIWrite = func(cmd, data []byte) error {
						if bytes.Equal(cmd, command) {
							valuesMu.Lock()
							values = append(values, data[0])
							valuesMu.Unlock()
						}
						return nil
					}
					done := make(chan uint8, 1)
					go func() { sendWithin(t, done, update(initial)) }()
					waitDone(t, entered)
					d.lightingMu.Lock()
					dpi, a, g := d.dpiValueRevision, d.assignmentRevision, d.gestureRevision
					own := d.liftHeightRevision
					if kind == "button" {
						own = d.buttonOptimizationRevision
					}
					d.lightingMu.Unlock()
					changed := make(chan struct{})
					switch change {
					case "dpi":
						go func() { d.SelectMouseDPIStage(2); close(changed) }()
						waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.dpiValueRevision > dpi })
					case "color":
						d.SaveMouseDpiColors(rgb.Color{}, map[int]rgb.Color{1: {Red: 10}})
						close(changed)
					case "sniper":
						go func() { d.SetMouseSniperMode(true); close(changed) }()
						waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.SniperMode })
					case "timer":
						d.mutateInputProfile(false, func(p *DeviceProfile) error { p.SleepMode = 30; return nil })
						close(changed)
					case "binding":
						go func() {
							v := d.inputAssignmentsSnapshot()[8]
							v.Default = false
							d.UpdateDeviceKeyAssignment(8, v)
							close(changed)
						}()
						waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.assignmentRevision > a })
					case "gesture":
						go func() { d.SaveMouseGestures(1, map[int]uint8{0: 30, 1: 40, 2: 50, 3: 60}); close(changed) }()
						waitCondition(t, func() bool { d.lightingMu.Lock(); defer d.lightingMu.Unlock(); return d.gestureRevision > g })
					case "owner":
						d.ProcessSetRgbCluster(true)
						close(changed)
					case "snapshot":
						d.SaveUserProfile("default")
						close(changed)
					case "save-as":
						d.SaveUserProfile("Saved-Profile")
						close(changed)
					case "other-preference":
						d.mutateInputProfile(false, func(p *DeviceProfile) error {
							if kind == "lift" {
								p.ButtonOptimization = 1
							} else {
								p.LiftHeight = 3
							}
							return nil
						})
						close(changed)
					case "new-value":
						go func() { update(next); close(changed) }()
						waitCondition(t, func() bool {
							d.lightingMu.Lock()
							defer d.lightingMu.Unlock()
							if kind == "lift" {
								return d.liftHeightRevision > own
							}
							return d.buttonOptimizationRevision > own
						})
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
						d.RebindReceiver(&common.Slipstream{}, 13)
						close(changed)
					case "stop":
						d.StopDirty()
						close(changed)
					}
					once.Do(func() { close(resume) })
					result := receiveWithin(t, done)
					waitDone(t, changed)
					valuesMu.Lock()
					defer valuesMu.Unlock()
					expected := []byte{byte(initial)}
					want := uint8(1)
					switch change {
					case "new-value":
						expected = []byte{byte(next)}
						want = 0
					case "profile":
						value := byte(2)
						if kind == "button" {
							value = 0
						}
						expected = []byte{value}
						want = 0
					case "connection", "rebind", "stop":
						expected = nil
						want = 0
					}
					if result != want || !reflect.DeepEqual(values, expected) {
						t.Fatal("wrong preference invalidation", kind, change, result, values, expected)
					}
				})
			}
		})
	}
}
func TestSavedPreferenceApplyFailureRemainsDurableAndRecovers(t *testing.T) {
	for _, kind := range []string{"lift", "button"} {
		t.Run(kind, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			readyLighting(d)
			command := cmdLiftHeight
			value := 3
			if kind == "button" {
				command = cmdButtonOptimization
				value = 1
			}
			fail := true
			var writes []byte
			d.lightingDPIWrite = func(cmd, data []byte) error {
				if bytes.Equal(cmd, command) {
					writes = append(writes, data[0])
					if fail {
						return errors.New("preference apply failure")
					}
				}
				return nil
			}
			update := func() uint8 {
				if kind == "lift" {
					return d.UpdateLiftHeight(value)
				}
				return d.UpdateButtonOptimization(value)
			}
			if update() != 0 {
				t.Fatal("hardware failure reported success")
			}
			data, err := os.ReadFile(d.DeviceProfile.Path)
			requireLighting(t, err)
			var saved DeviceProfile
			requireLighting(t, json.Unmarshal(data, &saved))
			if (kind == "lift" && saved.LiftHeight != value) || (kind == "button" && saved.ButtonOptimization != value) {
				t.Fatal("desired preference rolled back")
			}
			fail = false
			d.SetConnected(false)
			d.lightingReportWrite = func(p []byte) (int, error) {
				if bytes.Equal(p[2:2+len(command)], command) {
					writes = append(writes, p[2+len(command)])
				}
				return len(p), nil
			}
			d.Connect()
			d.stopLighting()
			if !d.Connected || !reflect.DeepEqual(writes, []byte{byte(value), byte(value)}) {
				t.Fatal("wake did not replay saved preference", writes)
			}
		})
	}
}
func TestDPIPairedApplyReportsOnlyConfirmedSuccess(t *testing.T) {
	for _, sniper := range []bool{false, true} {
		t.Run(map[bool]string{false: "regular", true: "sniper"}[sniper], func(t *testing.T) {
			for _, persistent := range []bool{false, true} {
				t.Run(map[bool]string{false: "transient-Y", true: "persistent-Y"}[persistent], func(t *testing.T) {
					d, _ := lightingFixture(t, true)
					readyLighting(d)
					var writes []dpiAxisWrite
					failed := false
					var indicators atomic.Int32
					d.lightingWrite = func([]byte) { indicators.Add(1) }
					d.lightingDPIWrite = func(cmd, data []byte) error {
						if bytes.Equal(cmd, cmdSetDpi[0]) || bytes.Equal(cmd, cmdSetDpi[1]) {
							writes = append(writes, recordDPIAxis(t, cmd, data))
							if bytes.Equal(cmd, cmdSetDpi[1]) && (persistent || !failed) {
								failed = true
								return errors.New("Y did not reach hardware")
							}
						}
						return nil
					}
					result := uint8(0)
					value := uint16(1200)
					if sniper {
						result = d.SetMouseSniperMode(true)
						value = 200
					} else {
						result = d.SelectMouseDPIStage(2)
					}
					count, want := 4, uint8(1)
					if persistent {
						count, want = 6, 0
					}
					if result != want || len(writes) != count {
						t.Fatal("wrong pair result/retry bound", result, writes)
					}
					for i, w := range writes {
						if w.Value != value || !bytes.Equal(w.Command[:], cmdSetDpi[i%2]) {
							t.Fatal("wrong paired retry", writes)
						}
					}
					if persistent {
						if indicators.Load() != 0 {
							t.Fatal("incomplete DPI triggered success redraw")
						}
						if sniper {
							if !d.SniperMode {
								t.Fatal("lost desired transient Sniper state")
							}
						} else {
							data, err := os.ReadFile(d.DeviceProfile.Path)
							requireLighting(t, err)
							var p DeviceProfile
							requireLighting(t, json.Unmarshal(data, &p))
							if p.Profile != 2 || p.Profiles[2].Value != 1200 {
								t.Fatal("desired regular DPI not durable")
							}
						}
					}
				})
			}
		})
	}
}
func TestDPIRetryRejectsRetiredValueOrConnection(t *testing.T) {
	for _, sniper := range []bool{false, true} {
		t.Run(map[bool]string{false: "regular", true: "sniper"}[sniper], func(t *testing.T) {
			for _, change := range []string{"value", "connection", "profile", "rebind", "stop"} {
				t.Run(change, func(t *testing.T) {
					d := profileSaveFixture(t, "default")
					d.KeyAssignment = defaultWirelessAssignments()
					readyLighting(d)
					var writes []dpiAxisWrite
					var changed atomic.Bool
					joined := make(chan struct{})
					if change != "profile" {
						close(joined)
					}
					d.lightingDPIWrite = func(cmd, data []byte) error {
						if bytes.Equal(cmd, cmdSetDpi[0]) || bytes.Equal(cmd, cmdSetDpi[1]) {
							writes = append(writes, recordDPIAxis(t, cmd, data))
						}
						if bytes.Equal(cmd, cmdSetDpi[1]) && !changed.Swap(true) {
							switch change {
							case "value":
								d.mutateInputProfile(false, func(p *DeviceProfile) error {
									if sniper {
										s := p.Profiles[5]
										s.Value = 300
										p.Profiles[5] = s
									} else {
										p.Profile = 3
									}
									return nil
								})
							case "connection":
								d.SetConnected(false)
								reconnectFixtureWithoutOutput(d)
							case "profile":
								go func() { d.ChangeDeviceProfile("Z"); close(joined) }()
								waitCondition(t, func() bool {
									d.lightingMu.Lock()
									defer d.lightingMu.Unlock()
									return d.DeviceProfile == d.UserProfiles["Z"]
								})
							case "rebind":
								d.RebindReceiver(&common.Slipstream{}, 13)
							case "stop":
								d.StopDirty()
							}
							return errors.New("old Y failed")
						}
						return nil
					}
					result := uint8(0)
					if sniper {
						result = d.SetMouseSniperMode(true)
					} else {
						result = d.SelectMouseDPIStage(2)
					}
					waitDone(t, joined)
					if result != 0 {
						t.Fatal("retired pair reported success")
					}
					wanted := 2
					if change == "profile" {
						wanted = 4
					} // New profile may program its OWN pair.
					if len(writes) != wanted {
						t.Fatal("old pair retried after retirement", writes)
					}
					if change == "profile" && (writes[2].Value != 800 || writes[3].Value != 800) {
						t.Fatal("replacement profile replay", writes)
					}
				})
			}
		})
	}
}
