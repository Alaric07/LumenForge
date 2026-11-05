package m65rgbelite

import (
	"LumenForge/src/common"
	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

func TestEliteButtonOptimizationUsesHardwareCapabilities(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	var commands, data [][]byte
	d.lightingDPIWrite = func(c, b []byte) error {
		commands = append(commands, append([]byte(nil), c...))
		data = append(data, append([]byte(nil), b...))
		return nil
	}
	saves := 0
	d.lightingProfileWrite = func(path string, b []byte) error { saves++; return writeLightingProfile(path, b) }
	for _, value := range []int{0, 1} {
		if d.UpdateButtonOptimization(value) != 1 || d.DeviceProfile.ButtonOptimization != value {
			t.Fatal("supported", value)
		}
		if !bytes.Equal(commands[len(commands)-1], []byte{0x30, 0, 0}) || !bytes.Equal(data[len(data)-1], []byte{byte(value + 1)}) {
			t.Fatal("Elite programming", commands, data)
		}
		if s, ok := d.PerformanceSnapshot(); !ok || s.ButtonOptimization == nil || s.LiftHeight != nil {
			t.Fatal(s)
		}
	}
	before := inputJSON(t, d)
	disk, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	snapshot, _ := d.LightingSnapshot()
	worker := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{})}
	d.lightingWorker = worker
	native, external := d.nativeGeneration, d.externalGeneration
	for _, value := range []int{-1, 2, 3, 4, 5, 255} {
		if d.UpdateButtonOptimization(value) != 0 {
			t.Fatal("unsupported", value)
		}
		if _, ok := d.PerformanceSnapshot(); !ok {
			t.Fatal("rejection hid Performance")
		}
	}
	delete(d.SwitchModes, 0)
	if d.UpdateButtonOptimization(0) != 0 {
		t.Fatal("missing capability accepted")
	}
	after, _ := d.LightingSnapshot()
	afterDisk, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	if saves != 2 || len(commands) != 2 || !bytes.Equal(before, inputJSON(t, d)) || !bytes.Equal(disk, afterDisk) || !reflect.DeepEqual(snapshot, after) || d.lightingWorker != worker || d.nativeGeneration != native || d.externalGeneration != external {
		t.Fatal("rejection published")
	}
}

func TestEliteInputBoundsAndWorkspaceProgramsEveryDPISlot(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	var commands, data [][]byte
	d.lightingDPIWrite = func(c, b []byte) error {
		commands = append(commands, append([]byte(nil), c...))
		data = append(data, append([]byte(nil), b...))
		return nil
	}
	before := inputJSON(t, d)
	for _, value := range []int{-1, 2, 3, 4} {
		if d.UpdateAngleSnapping(value) != 0 {
			t.Fatal("invalid angle", value)
		}
	}
	for _, value := range []int{-1, 3, 5, 6, 7, 9, 10} {
		if d.UpdatePollingRate(value) != 0 {
			t.Fatal("invalid polling", value)
		}
	}
	if len(commands) != 0 || !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("invalid input published/programmed")
	}
	stages, colors := map[int]uint16{}, map[int]rgb.Color{}
	for id, p := range d.DeviceProfile.Profiles {
		stages[id] = p.Value
		colors[id] = *p.Color
	}
	stages[0] = 18000
	if d.SaveMouseDPISettings(stages, colors) != 1 || len(commands) != 7 {
		t.Fatal("Elite workspace must program six table records and selection", commands)
	}
	for i := 0; i < 6; i++ {
		if !bytes.Equal(commands[i], []byte{0x13}) || len(data[i]) != 10 {
			t.Fatal(commands[i], data[i])
		}
	}
	if !bytes.Equal(commands[6], []byte{0x13, 2, 0}) || !bytes.Equal(data[6], []byte{2}) {
		t.Fatal("selection", commands[6], data[6])
	}
	before = inputJSON(t, d)
	stages[0] = 18001
	if d.SaveMouseDPISettings(stages, colors) != 0 || len(commands) != 7 || !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("DPI ceiling accepted")
	}
	stages[0] = 99
	if d.SaveMouseDPISettings(stages, colors) != 0 || len(commands) != 7 {
		t.Fatal("DPI floor accepted")
	}
}

func TestEliteAssignmentFailureIsObservableBeforeAnyProfileWrite(t *testing.T) {
	for _, name := range []string{"X", "Y", "Z"} {
		t.Run(name, func(t *testing.T) {
			d := profileSaveFixture(t, "X")
			active, inactive := d.DeviceProfile, d.UserProfiles["Z"]
			before, files := inputJSON(t, d), profileSaveFiles(t)
			snapshot, _ := d.LightingSnapshot()
			worker, native, external := d.lightingWorker, d.nativeGeneration, d.externalGeneration
			profiles, assignments := 0, 0
			var outputs atomic.Int32
			d.lightingWrite = func([]byte) { outputs.Add(1) }
			d.lightingProfileWrite = func(string, []byte) error { profiles++; return nil }
			d.lightingAssignmentWrite = func(string, []byte) error { assignments++; return errors.New("injected assignment failure") }
			if d.SaveUserProfile(name) != 0 || profiles != 0 || assignments != 1 {
				t.Fatal("failure hidden or profile written")
			}
			after, _ := d.LightingSnapshot()
			if !reflect.DeepEqual(files, profileSaveFiles(t)) || !bytes.Equal(before, inputJSON(t, d)) || !reflect.DeepEqual(snapshot, after) || d.DeviceProfile != active || d.UserProfiles["X"] != active || d.UserProfiles["Z"] != inactive || d.lightingWorker != worker || d.nativeGeneration != native || d.externalGeneration != external || outputs.Load() != 0 {
				t.Fatal("assignment failure published")
			}
			oldKeys := make(map[int]inputmanager.KeyAssignment)
			for id, key := range d.KeyAssignment {
				oldKeys[id] = key
			}
			if d.UpdateDeviceKeyAssignment(1, inputmanager.KeyAssignment{Default: false, ActionType: 3}) != 0 || !reflect.DeepEqual(oldKeys, d.KeyAssignment) || profiles != 0 || !reflect.DeepEqual(files, profileSaveFiles(t)) {
				t.Fatal("assignment edit failure hidden/published")
			}
		})
	}
}

// Elite source explicitly permits external effect bytes after its one-shot
// RGB-off blank. Scheduler darkness scales only the locally owned indicator.
func TestEliteRGBOffExternalPrecedenceAndProfileReplay(t *testing.T) {
	for _, source := range []string{"cluster", "openrgb"} {
		t.Run(source, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			requireLighting(t, d.SetLightingEffect("mouse"))
			requireLighting(t, d.SetLightingBrightness(50))
			var callback func([]byte, int)
			d.lightingClusterRegister = func(c *common.ClusterController) { callback = c.WriteColorEx }
			d.lightingOpenRGBReconcile = func(_ string, c *common.OpenRGBController) {
				if source == "openrgb" && c != nil {
					callback = c.WriteColorEx
				}
			}
			enable := d.ProcessSetRgbCluster
			if source == "openrgb" {
				enable = d.ProcessSetOpenRgbIntegration
			}
			if enable(true) != 1 {
				t.Fatal("enable")
			}
			requireLighting(t, d.setUserRGBOff(true))
			readyLighting(d)
			d.setupClusterController()
			d.setupOpenRGBController()
			d.startQueueWorker()
			frames := make(chan []byte, 20)
			d.lightingWrite = func(p []byte) { frames <- p }
			d.setDeviceColor()
			blank := waitFrame(t, frames)
			if !bytes.Equal(blank, []byte{2, 0, 0, 0, 3, 127, 127, 127, 4, 0, 0, 0}) {
				t.Fatal("Elite RGB-off", blank)
			}
			if callback == nil {
				t.Fatal("missing controller")
			}
			raw := []byte{11, 22, 33, 44, 55, 66}
			callback(raw, 0)
			packet := waitFrame(t, frames)
			if !bytes.Equal(packet, []byte{2, 44, 55, 66, 3, 127, 127, 127, 4, 11, 22, 33}) {
				t.Fatal("source external override", packet)
			}
			requireLighting(t, d.setSchedulerDark(true))
			requireLighting(t, d.SetLightingBrightness(70))
			callback(raw, 0)
			packet = waitFrame(t, frames)
			if !bytes.Equal(packet, []byte{2, 44, 55, 66, 3, 0, 0, 0, 4, 11, 22, 33}) {
				t.Fatal("scheduler scaled externally owned bytes", packet)
			}
			requireLighting(t, d.setSchedulerDark(false))
			callback(raw, 0)
			packet = waitFrame(t, frames)
			if packet[5] != 178 || packet[9] != 11 {
				t.Fatal("latest brightness", packet)
			}
			next := cloneLightingProfile(*d.DeviceProfile)
			next.Path = filepath.Join(filepath.Dir(next.Path), "next.json")
			d.lightingMu.Lock()
			d.UserProfiles["next"] = &next
			d.lightingMu.Unlock()
			old := callback
			if d.ChangeDeviceProfile("next") != 1 {
				t.Fatal("profile switch")
			}
			packet = waitFrame(t, frames)
			if packet[1] != 0 || packet[9] != 0 || packet[5] != 178 {
				t.Fatal("RGB-off not replayed", packet)
			}
			old(raw, 0)
			noFrame(t, frames)
			callback(raw, 0)
			packet = waitFrame(t, frames)
			if packet[1] != 44 || packet[9] != 11 {
				t.Fatal("new owner session", packet)
			}
			if enable(false) != 1 {
				t.Fatal("release")
			}
			packet = waitFrame(t, frames)
			if packet[1] != 0 || packet[9] != 0 {
				t.Fatal("RGB-off lost on release", packet)
			}
			requireLighting(t, d.setUserRGBOff(false))
			packet = waitFrame(t, frames)
			if packet[1] != 178 || packet[10] != 178 || packet[5] != 178 {
				t.Fatal("Native desired state not restored", packet)
			}
		})
	}
}

func TestConcurrentEliteProfileSaveDeleteAndAssignments(t *testing.T) {
	d := profileSaveFixture(t, "X")
	snapshot, _ := d.LightingSnapshot()
	var wg sync.WaitGroup
	for _, action := range []func(){
		func() { d.SaveUserProfile("Y"); d.DeleteDeviceProfile("Y") },
		func() { d.SaveUserProfile("X") },
		func() { d.UpdateDeviceKeyAssignment(1, inputmanager.KeyAssignment{Default: true}) },
		func() { d.DeviceProfileSnapshot(); d.ButtonsSnapshot(); d.LightingSnapshot() },
	} {
		wg.Add(1)
		go func(f func()) {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				f()
			}
		}(action)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	waitDone(t, done)
	after, _ := d.LightingSnapshot()
	if d.DeviceProfile != d.UserProfiles["X"] || !d.DeviceProfile.Active || !reflect.DeepEqual(snapshot, after) {
		t.Fatal("concurrent profile integrity")
	}
	data, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	var stored DeviceProfile
	requireLighting(t, json.Unmarshal(data, &stored))
	if !reflect.DeepEqual(stored, *d.DeviceProfile) {
		t.Fatal("runtime/disk identity")
	}
}

func TestEliteFreshAndUpgradeSniperKeepMetadataAndNormalIndicatorSlot(t *testing.T) {
	for _, upgraded := range []bool{false, true} {
		for _, source := range []string{"native", "cluster", "openrgb"} {
			t.Run(fmt.Sprintf("upgrade=%v/%s", upgraded, source), func(t *testing.T) {
				d, _ := lightingFixture(t, true)
				if upgraded {
					delete(d.DeviceProfile.Profiles, 5)
					proposal := prepareLightingInputProfile(*d.DeviceProfile)
					*d.DeviceProfile = proposal
				}
				expected := cloneLightingProfile(*d.DeviceProfile)
				sniper := expected.Profiles[5]
				offset, indices, value := 4, []int{5, 6, 7}, uint16(400)
				if upgraded {
					offset, indices, value = 8, []int{9, 10, 11}, 200
				}
				if sniper.Value != value || sniper.LEDIndex != 3 || sniper.LEDIndexPosition != offset || !reflect.DeepEqual(sniper.ColorIndex[0], indices) || sniper.Color.Red != 255 || sniper.Color.Green != 255 || sniper.Color.Blue != 0 {
					t.Fatal("source metadata", sniper)
				}
				requireLighting(t, d.SetLightingEffect("mouse"))
				if source == "cluster" && d.ProcessSetRgbCluster(true) != 1 {
					t.Fatal("cluster")
				}
				if source == "openrgb" && d.ProcessSetOpenRgbIntegration(true) != 1 {
					t.Fatal("OpenRGB")
				}
				before := inputJSON(t, d)
				readyLighting(d)
				frames := make(chan []byte, 10)
				d.lightingWrite = func(packet []byte) { frames <- packet }
				var commands, data [][]byte
				d.lightingDPIWrite = func(c, b []byte) error {
					commands = append(commands, append([]byte(nil), c...))
					data = append(data, append([]byte(nil), b...))
					return nil
				}
				if d.SetMouseSniperMode(true) != 1 {
					t.Fatal("sniper")
				}
				if len(commands) != 1 || !bytes.Equal(commands[0], []byte{0x13, 2, 0}) || !bytes.Equal(data[0], []byte{0}) {
					t.Fatal("invented flash/programming", commands, data)
				}
				var packet []byte
				switch source {
				case "native":
					packet = waitFrame(t, frames)
				case "cluster":
					d.writeColorCluster([]byte{1, 2, 3, 4, 5, 6}, 0)
					packet = waitFrame(t, frames)
				case "openrgb":
					waitFrame(t, frames)
					d.startQueueWorker()
					d.writeColorEx([]byte{1, 2, 3, 4, 5, 6}, 0)
					packet = waitFrame(t, frames)
				}
				// Sniper substitutes only the color. The selected regular stage
				// still supplies the offset4 layout, even for upgrade metadata8.
				if packet[4] != 3 || !bytes.Equal(packet[5:8], []byte{255, 255, 0}) || packet[8] != 4 {
					t.Fatal("indicator/Scroll mapping", packet)
				}
				if !bytes.Equal(before, inputJSON(t, d)) {
					t.Fatal("render/programming normalized stored metadata or authored input")
				}
			})
		}
	}
}
