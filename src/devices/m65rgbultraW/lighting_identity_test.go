package m65rgbultraW

import (
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func legacyIdentityFixture(t *testing.T) *Device {
	t.Helper()
	d, _ := lightingFixture(t, false)
	root := t.TempDir()
	previous := pwd
	pwd = root
	t.Cleanup(func() { pwd = previous })
	for _, dir := range []string{"profiles", "key-assignments", "rgb"} {
		requireLighting(t, os.MkdirAll(filepath.Join(root, "database", dir), 0700))
	}
	d.Serial = "identity"
	d.DeviceProfile.Serial = d.Serial
	d.DeviceProfile.ProfileName = "Work"
	d.DeviceProfile.Path = filepath.Join(root, "database", "profiles", d.Serial+"-Work.json")
	d.UserProfiles = map[string]*DeviceProfile{"Work": d.DeviceProfile}
	d.rebuildProfileOrder()
	requireLighting(t, d.persistLightingProfile(*d.DeviceProfile))
	return d
}
func TestUnnamedPreservedNamesRemainDistinct(t *testing.T) {
	d := legacyIdentityFixture(t)
	for _, name := range []string{"Desk", "Desk-preserved", "Work-Night-preserved", "physical-alias"} {
		p := cloneLightingProfile(*d.DeviceProfile)
		p.Active = false
		p.ProfileName = ""
		p.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-"+name+".json")
		if name == "physical-alias" {
			p.ProfileName = "Known-Historical"
		}
		requireLighting(t, d.persistLightingProfile(p))
	}
	p := cloneLightingProfile(*d.DeviceProfile)
	p.Path = filepath.Join(pwd, "database", "profiles", d.Serial+".json")
	p.ProfileName = "ignored-default-field"
	p.Active = false
	requireLighting(t, d.persistLightingProfile(p))
	d.loadDeviceProfiles()
	for _, name := range []string{"default", "Desk", "Desk-preserved", "Work-Night-preserved", "Known-Historical"} {
		if d.UserProfiles[name] == nil {
			t.Fatal("missing profile", name, d.ProfileOrder)
		}
	}
	if d.UserProfiles["Desk"].Path == d.UserProfiles["Desk-preserved"].Path || d.UserProfiles["physical-alias"] != nil {
		t.Fatal("profile aliases lost distinct identity")
	}
}
func TestFallbackSaveAsClonesRequestedIdentity(t *testing.T) {
	d := legacyIdentityFixture(t)
	source := d.DeviceProfile
	before := cloneLightingProfile(*source)
	for _, name := range []string{"Travel", "Travel-Work-Night"} {
		if d.SaveUserProfile(name) != 1 {
			t.Fatal("save", name)
		}
		p := d.UserProfiles[name]
		if p == nil || p.ProfileName != name || p.Active || p.Path == source.Path {
			t.Fatal("wrong saved identity", p)
		}
		if !reflect.DeepEqual(before, *source) {
			t.Fatal("source mutated")
		}
	}
	path := d.UserProfiles["Travel"].Path
	if d.SaveUserProfile("Travel") != 1 || d.UserProfiles["Travel"].Path != path || d.UserProfiles["Travel"].Active {
		t.Fatal("inactive overwrite semantics")
	}
	reloaded := &Device{Serial: d.Serial}
	reloaded.loadDeviceProfiles()
	for _, name := range []string{"Work", "Travel", "Travel-Work-Night"} {
		if reloaded.UserProfiles[name] == nil {
			t.Fatal("reload identity missing", name)
		}
	}
	if reloaded.DeviceProfile != reloaded.UserProfiles["Work"] {
		t.Fatal("Save As changed active identity")
	}
}
func TestOwnershipWrappersPreserveOppositeOwner(t *testing.T) {
	for _, owner := range []string{"cluster", "openrgb"} {
		t.Run(owner, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			readyLighting(d)
			registrations := 0
			d.lightingClusterRegister = func(*common.ClusterController) { registrations++ }
			d.lightingClusterRemove = func(string) { registrations++ }
			d.lightingOpenRGBReconcile = func(string, *common.OpenRGBController) { registrations++ }
			if owner == "cluster" {
				if d.ProcessSetRgbCluster(true) != 1 {
					t.Fatal("owner")
				}
			} else {
				if d.ProcessSetOpenRgbIntegration(true) != 1 {
					t.Fatal("owner")
				}
			}
			d.stopLighting()
			count := registrations
			generation := d.externalGeneration
			for _, enable := range []bool{true, false} {
				result := uint8(0)
				if owner == "cluster" {
					result = d.ProcessSetOpenRgbIntegration(enable)
				} else {
					result = d.ProcessSetRgbCluster(enable)
				}
				if result != 2 {
					t.Fatal("exclusivity changed", result)
				}
			}
			d.lightingMu.Lock()
			cluster, open := d.clusterOwned, d.openRGBOwned
			worker := d.lightingWorker
			d.lightingMu.Unlock()
			if cluster != (owner == "cluster") || open != (owner == "openrgb") || registrations != count || generation != d.externalGeneration || worker != nil {
				t.Fatal("opposite owner disturbed")
			}
			d.lightingMu.Lock()
			g := d.externalGeneration
			d.lightingMu.Unlock()
			if result := d.applyLightingPacket(make([]byte, 6), owner, g, nil); result != lightingApplied {
				t.Fatal("effective owner lost", result)
			}
			if owner == "cluster" {
				d.ProcessSetRgbCluster(false)
			} else {
				d.ProcessSetOpenRgbIntegration(false)
			}
			waitCondition(t, func() bool {
				d.lightingMu.Lock()
				defer d.lightingMu.Unlock()
				return !d.clusterOwned && !d.openRGBOwned
			})
			snap, ok := d.LightingSnapshot()
			if !ok || snap.ClusterControlled || snap.ExternalControlled {
				t.Fatal("Native not restored")
			}
		})
	}
}
func TestSavedAssignmentFailureRecoversOnRetryAndWake(t *testing.T) {
	for _, recovery := range []string{"retry", "wake"} {
		t.Run(recovery, func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			d.KeyAssignment = defaultWirelessAssignments()
			readyLighting(d)
			failed := false
			var packets [][]byte
			d.lightingDPIWrite = func(cmd, data []byte) error {
				if bytes.Equal(cmd, cmdWrite) {
					packets = append(packets, append([]byte(nil), data...))
					if !failed {
						failed = true
						return errors.New("hardware unconfirmed")
					}
				}
				return nil
			}
			a := d.inputAssignmentsSnapshot()[8]
			a.Default = false
			if d.UpdateDeviceKeyAssignment(8, a) != 0 {
				t.Fatal("failure hidden")
			}
			saved, err := d.readLightingAssignments(*d.DeviceProfile)
			requireLighting(t, err)
			if saved[8].Default {
				t.Fatal("durable binding rolled back")
			}
			if recovery == "retry" {
				if d.UpdateDeviceKeyAssignment(8, a) != 1 {
					t.Fatal("identical retry skipped/failed")
				}
			} else {
				d.SetConnected(false)
				d.lightingReportWrite = func(p []byte) (int, error) { return len(p), nil }
				d.Connect()
				d.stopLighting()
				if !d.Connected {
					t.Fatal("wake failed")
				}
			}
			if len(packets) != 2 || packets[1][headerWriteSize+3] != 0 {
				t.Fatal("saved binding never recovered", packets)
			}
		})
	}
}
func TestTiltHeldConcreteActionsSurviveProfileSwitch(t *testing.T) {
	for _, cleanup := range []string{"release", "disconnect", "stop"} {
		t.Run(cleanup, func(t *testing.T) {
			d := profileSaveFixture(t, "default")
			d.KeyAssignment = defaultWirelessAssignments()
			readyLighting(d)
			for i, key := range []int{1024, 2048} {
				v := d.KeyAssignment[key]
				v.Default = false
				v.ActionHold = true
				v.ActionType = 3
				v.ActionCommand = uint16(10 + i)
				d.KeyAssignment[key] = v
			}
			next := defaultWirelessAssignments()
			for _, key := range []int{1024, 2048} {
				v := next[key]
				v.Default = false
				v.ActionHold = true
				v.ActionType = 3
				v.ActionCommand = 20
				next[key] = v
			}
			data, _ := json.Marshal(next)
			requireLighting(t, os.WriteFile(filepath.Join(pwd, "database", "key-assignments", "inactiveassign.json"), data, 0600))
			type event struct {
				key     uint16
				pressed bool
			}
			events := []event{}
			d.tiltHoldWrite = func(_ uint8, key uint16, pressed bool) { events = append(events, event{key, pressed}) }
			d.TriggerTiltAssignment(1024)
			d.TriggerTiltAssignment(2048)
			if d.ChangeDeviceProfile("Z") != 1 {
				t.Fatal("switch")
			}
			if cleanup == "release" {
				d.TriggerTiltAssignment(1024)
				d.TriggerTiltAssignment(2048)
			} else if cleanup == "disconnect" {
				d.SetConnected(false)
			} else {
				d.StopDirty()
			}
			counts := map[event]int{}
			for _, e := range events {
				counts[e]++
			}
			for _, key := range []uint16{10, 11} {
				if counts[event{key, true}] != 1 || counts[event{key, false}] != 1 {
					t.Fatal("original held action stuck", events)
				}
			}
			if len(events) != 4 || len(d.heldTilts) != 0 {
				t.Fatal("replacement key incorrectly released", events)
			}
		})
	}
}
func TestPresetBrightnessPersistsCanonicalValues(t *testing.T) {
	d, rt := lightingFixture(t, true)
	readyLighting(d)
	frames := make(chan []byte, 16)
	d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
	for mode, value := range []uint8{100, 33, 66, 100} {
		if d.ChangeDeviceBrightness(uint8(mode)) != 1 {
			t.Fatal("preset", mode)
		}
		state, _, err := rt.State.Resolve(d.Serial)
		requireLighting(t, err)
		if state.Brightness != value {
			t.Fatal("wrong canonical value", mode, state.Brightness)
		}
		reloaded, err := lightingsettings.LoadIndependentDeviceStateStore(filepath.Join(filepath.Dir(d.DeviceProfile.Path), "state.json"))
		requireLighting(t, err)
		persisted, _, err := reloaded.Resolve(d.Serial)
		requireLighting(t, err)
		if persisted.Brightness != value {
			t.Fatal("preset not durable", mode)
		}
		packet := waitFrame(t, frames)
		if packet[1] != uint8(float64(value)*255/100) {
			t.Fatal("preset did not refresh output", mode, packet)
		}
	}
	if d.ChangeDeviceBrightness(4) != 0 || d.ChangeDeviceBrightness(255) != 0 {
		t.Fatal("unsupported preset")
	}
}
func TestAttachmentFailureLeavesLegacyEligible(t *testing.T) {
	for _, failure := range []string{"store", "settings", "defaults", "topology", "preferences"} {
		t.Run(failure, func(t *testing.T) {
			d, rt := lightingFixture(t, false)
			root := t.TempDir()
			previous := pwd
			pwd = root
			t.Cleanup(func() { pwd = previous })
			requireLighting(t, os.MkdirAll(filepath.Join(root, "database", "rgb"), 0700))
			var err error
			if failure == "store" || failure == "settings" || failure == "defaults" {
				paths := config.Paths{OpenRGBDeviceLightingFile: filepath.Join(root, "state.json"), DeviceEffectSettingsFile: filepath.Join(root, "effects.json"), ShippedDatabaseRoot: filepath.Join(root, "shipped")}
				requireLighting(t, os.MkdirAll(paths.ShippedDatabaseRoot, 0700))
				defaults, e := os.ReadFile("../../../database/rgb.json")
				requireLighting(t, e)
				requireLighting(t, os.WriteFile(filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"), defaults, 0600))
				if failure == "store" {
					requireLighting(t, os.WriteFile(paths.OpenRGBDeviceLightingFile, []byte("invalid"), 0600))
				} else if failure == "settings" {
					requireLighting(t, os.WriteFile(paths.DeviceEffectSettingsFile, []byte(`{"schemaVersion":1,"devices":{"bad":{"gradient":{"schemaVersion":1,"effectId":"gradient"}}}}`), 0600))
				} else {
					requireLighting(t, os.Remove(filepath.Join(paths.ShippedDatabaseRoot, "rgb.json")))
				}
				err = d.attachLightingRuntime(paths)
			} else {
				if failure == "topology" {
					d.LEDChannels = 3
				} else {
					d.DeviceProfile = nil
				}
				err = d.attachLightingSource(rt)
			}
			if err == nil || d.lightingAttached() || d.lightingCanonical != nil || d.lightingWorker != nil || d.externalQueue != nil {
				t.Fatal("partial attachment published", err)
			}
			if _, ok := d.LightingSnapshot(); ok {
				t.Fatal("failed snapshot suppresses LegacyLighting")
			}
			legacy := &rgb.RGB{Profiles: map[string]rgb.Profile{}}
			for _, effect := range rgbProfileUpgrade {
				legacy.Profiles[effect] = rgb.Profile{}
			}
			data, marshalErr := json.Marshal(legacy)
			requireLighting(t, marshalErr)
			requireLighting(t, os.WriteFile(filepath.Join(root, "database", "rgb", d.Serial+".json"), data, 0600))
			d.Rgb = nil
			d.loadRgb()
			if d.Rgb == nil {
				t.Fatal("legacy load skipped")
			}
		})
	}
}
func TestFallbackRotationSurvivesConcurrentDeletion(t *testing.T) {
	for _, names := range [][]string{{"a", "b"}, {"a", "b", "c"}} {
		t.Run(names[len(names)-1], func(t *testing.T) {
			d := legacyIdentityFixture(t)
			requireLighting(t, os.Remove(d.DeviceProfile.Path))
			d.UserProfiles = map[string]*DeviceProfile{}
			for _, name := range names {
				p := cloneLightingProfile(*d.DeviceProfile)
				p.ProfileName = name
				p.RGBProfile = "static"
				z := p.ZoneColors[0]
				z.ColorIndex = []int{0, 2, 4}
				p.ZoneColors[0] = z
				p.Active = name == "a"
				p.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-"+name+".json")
				requireLighting(t, d.persistLightingProfile(p))
				d.UserProfiles[name] = &p
			}
			d.DeviceProfile = d.UserProfiles["a"]
			d.rebuildProfileOrder()
			d.Connected = true
			d.lightingReportWrite = func(report []byte) (int, error) { return len(report), nil }
			selected, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			var selectedNames []string
			d.legacyProfileSelected = func(name string) {
				selectedNames = append(selectedNames, name)
				if name != "b" {
					t.Error("wrong next", name)
				}
				close(selected)
				waitDone(t, release)
			}
			done := make(chan struct{})
			go func() { d.rotateDeviceProfile(); close(done) }()
			waitDone(t, selected)
			if d.DeleteDeviceProfile("b") != 1 {
				t.Fatal("delete")
			}
			once.Do(func() { close(release) })
			waitDone(t, done)
			if !reflect.DeepEqual(selectedNames, []string{"b"}) {
				t.Fatal("race candidate was not observed", selectedNames)
			}
			if d.UserProfiles["b"] != nil || d.DeviceProfile != d.UserProfiles["a"] {
				t.Fatal("invalid remaining profile")
			}
			// Snapshot selection itself also preserves wraparound after deletion.
			last := names[len(names)-1]
			if last == "b" {
				last = "a"
			}
			d.DeviceProfile.Active = false
			d.DeviceProfile = d.UserProfiles[last]
			d.DeviceProfile.Active = true
			afterDeletion := len(selectedNames)
			d.legacyProfileSelected = func(name string) { selectedNames = append(selectedNames, name) }
			d.rotateDeviceProfile()
			if !reflect.DeepEqual(selectedNames, []string{"b", "a"}) || d.DeviceProfile != d.UserProfiles["a"] || !d.DeviceProfile.Active {
				t.Fatal("wraparound did not select/activate a", selectedNames)
			}
			for _, name := range selectedNames[afterDeletion:] {
				if name == "b" {
					t.Fatal("selected deleted profile")
				}
			}
		})
	}
}

func TestTiltPressOverlappingProfileSwitchKeepsConcreteRelease(t *testing.T) {
	d := profileSaveFixture(t, "default")
	d.KeyAssignment = defaultWirelessAssignments()
	readyLighting(d)
	action := d.KeyAssignment[1024]
	action.Default = false
	action.ActionType = 3
	action.ActionHold = true
	action.ActionCommand = 10
	d.KeyAssignment[1024] = action
	next := defaultWirelessAssignments()
	action.ActionCommand = 20
	next[1024] = action
	data, _ := json.Marshal(next)
	requireLighting(t, os.WriteFile(filepath.Join(pwd, "database", "key-assignments", "inactiveassign.json"), data, 0600))
	keys := []uint16{}
	pressed := []bool{}
	d.tiltHoldWrite = func(_ uint8, key uint16, down bool) {
		if down && d.ChangeDeviceProfile("Z") != 1 {
			t.Fatal("overlapping switch")
		}
		keys = append(keys, key)
		pressed = append(pressed, down)
	}
	d.TriggerTiltAssignment(1024)
	d.TriggerTiltAssignment(1024)
	if !reflect.DeepEqual(keys, []uint16{10, 10}) || !reflect.DeepEqual(pressed, []bool{true, false}) || len(d.heldTilts) != 0 {
		t.Fatal("pressed/released replacement key", keys, pressed)
	}
}

func TestConnectedFallbackRotationActivatesExistingNextProfile(t *testing.T) {
	d := legacyIdentityFixture(t)
	requireLighting(t, os.Remove(d.DeviceProfile.Path))
	d.UserProfiles = map[string]*DeviceProfile{}
	for _, name := range []string{"a", "b"} {
		p := cloneLightingProfile(*d.DeviceProfile)
		p.ProfileName = name
		p.Active = name == "a"
		p.RGBProfile = "static"
		z := p.ZoneColors[0]
		z.ColorIndex = []int{0, 2, 4}
		p.ZoneColors[0] = z
		p.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-"+name+".json")
		requireLighting(t, d.persistLightingProfile(p))
		d.UserProfiles[name] = &p
	}
	d.DeviceProfile = d.UserProfiles["a"]
	d.rebuildProfileOrder()
	d.Connected = true
	d.lightingReportWrite = func(p []byte) (int, error) { return len(p), nil }
	var selectedNames []string
	d.legacyProfileSelected = func(name string) { selectedNames = append(selectedNames, name) }
	d.rotateDeviceProfile()
	if !reflect.DeepEqual(selectedNames, []string{"b"}) {
		t.Fatal("normal rotation did not select b", selectedNames)
	}
	if d.DeviceProfile != d.UserProfiles["b"] || !d.DeviceProfile.Active {
		t.Fatal("connected control did not activate b")
	}
}
