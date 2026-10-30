package harpoonW

import (
	"LumenForge/src/common"
	"LumenForge/src/rgb"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

func TestPrivateWorkerBoundedCancellationAndStaleRetirement(t *testing.T) {
	d, _ := lightingFixture(t, true)
	legacy := rgb.Exit()
	d.activeRgb = legacy
	requireLighting(t, d.SetLightingEffect("rainbow"))
	started, release := make(chan struct{}), make(chan struct{})
	frames := make(chan []byte, 10)
	var first atomic.Bool
	d.lightingWrite = func(p []byte) {
		if first.CompareAndSwap(false, true) {
			close(started)
			<-release
		}
		frames <- p
	}
	d.Connect()
	waitDone(t, started)
	d.lightingMu.Lock()
	old := d.lightingWorker
	d.lightingMu.Unlock()
	done := make(chan struct{})
	go func() { _ = d.SetLightingEffect("static"); close(done) }()
	waitDone(t, done)
	d.lightingMu.Lock()
	current := d.lightingWorker
	d.lightingMu.Unlock()
	if old == nil || current == nil || current == old || d.activeRgb != legacy {
		t.Fatal("mixed worker ownership")
	}
	d.retireLightingWorker(old)
	d.lightingMu.Lock()
	retained := d.lightingWorker
	d.lightingMu.Unlock()
	if retained != current {
		t.Fatal("stale worker retired new owner")
	}
	close(release)
	waitFrame(t, frames)
	waitFrame(t, frames)
	waitDone(t, old.done)
	waitDone(t, current.done)
	if d.activeRgb != legacy {
		t.Fatal("canonical worker touched legacy Exit")
	}
}

func TestStaleNativeFrameRejectedAfterOutputSerialization(t *testing.T) {
	for _, transition := range []string{"cluster", "openrgb", "disconnect", "profile"} {
		t.Run(transition, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			frames := make(chan []byte, 20)
			d.lightingWrite = func(p []byte) { frames <- p }
			blocked, release := make(chan struct{}), make(chan struct{})
			var once atomic.Bool
			d.lightingBeforeApply = func(source string) {
				if source == "native" && once.CompareAndSwap(false, true) {
					close(blocked)
					<-release
				}
			}
			next := cloneLightingProfile(*d.DeviceProfile)
			next.Path = filepath.Join(t.TempDir(), "next.json")
			next.RGBCluster = true
			d.UserProfiles["next"] = &next
			d.Connect()
			waitDone(t, blocked)
			d.lightingMu.Lock()
			old := d.lightingWorker
			d.lightingMu.Unlock()
			switch transition {
			case "cluster":
				if d.ProcessSetRgbCluster(true) != 1 {
					t.Fatal("cluster")
				}
			case "openrgb":
				if d.ProcessSetOpenRgbIntegration(true) != 1 {
					t.Fatal("openrgb")
				}
				waitFrame(t, frames)
			case "disconnect":
				d.disconnectLighting(false)
			case "profile":
				if d.ChangeDeviceProfile("next") != 1 {
					t.Fatal("profile")
				}
			}
			close(release)
			waitDone(t, old.done)
			noFrame(t, frames)
		})
	}
}

func TestExternalRetainedAndDequeuedFramesRejectOldSessions(t *testing.T) {
	for _, source := range []string{"cluster", "openrgb"} {
		for _, transition := range []string{"owner", "disconnect", "profile", "stop"} {
			t.Run(source+"/"+transition, func(t *testing.T) {
				d, _ := lightingFixture(t, true)
				frames := make(chan []byte, 20)
				d.lightingWrite = func(p []byte) { frames <- p }
				blocked, release := make(chan struct{}), make(chan struct{})
				var once atomic.Bool
				d.lightingBeforeApply = func(s string) {
					if s == source && once.CompareAndSwap(false, true) {
						close(blocked)
						<-release
					}
				}
				d.Connect()
				waitFrame(t, frames)
				if source == "cluster" {
					d.ProcessSetRgbCluster(true)
				} else {
					d.ProcessSetOpenRgbIntegration(true)
					waitFrame(t, frames)
				}
				next := cloneLightingProfile(*d.DeviceProfile)
				next.Path = filepath.Join(t.TempDir(), "next.json")
				d.UserProfiles["next"] = &next
				generation := d.externalGeneration
				done := make(chan struct{})
				go func() {
					d.applyLightingPacket(composeLightingPacket([]byte{77, 88, 99}), source, generation, nil)
					close(done)
				}()
				waitDone(t, blocked)
				switch transition {
				case "owner":
					if source == "cluster" {
						d.ProcessSetRgbCluster(false)
						waitFrame(t, frames)
						d.ProcessSetRgbCluster(true)
					} else {
						d.ProcessSetOpenRgbIntegration(false)
						waitFrame(t, frames)
						d.ProcessSetOpenRgbIntegration(true)
						waitFrame(t, frames)
					}
				case "disconnect":
					d.disconnectLighting(false)
					d.Connect()
					if source == "openrgb" {
						waitFrame(t, frames)
					}
				case "stop":
					d.StopDirty()
				case "profile":
					if d.ChangeDeviceProfile("next") != 1 {
						t.Fatal("profile")
					}
					if source == "openrgb" {
						waitFrame(t, frames)
					}
				}
				close(release)
				waitDone(t, done)
				noFrame(t, frames)
				if transition == "stop" {
					return
				}
				// A fresh-session callback remains usable with exact external RGB bytes.
				d.applyLightingPacket(composeLightingPacket([]byte{1, 2, 3}), source, d.externalGeneration, nil)
				if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 1, 255, 2, 255, 3}) {
					t.Fatal(got)
				}
			})
		}
	}
}

func TestOpenRGBQueueCopiesAndInvalidatesPendingFrames(t *testing.T) {
	d, _ := lightingFixture(t, true)
	frames := make(chan []byte, 20)
	d.lightingWrite = func(p []byte) { frames <- p }
	d.Connect()
	waitFrame(t, frames)
	d.ProcessSetOpenRgbIntegration(true)
	waitFrame(t, frames)
	data := []byte{1, 2, 3}
	d.writeColorEx(data, 0)
	data[0] = 99
	frame := <-d.externalQueue
	if !bytes.Equal(frame.data, []byte{1, 2, 3}) {
		t.Fatal("queue aliases caller")
	}
	d.writeColorEx([]byte{4, 5, 6}, 0)
	d.disconnectLighting(false)
	if len(d.externalQueue) != 0 {
		t.Fatal("pending old frame survived disconnect")
	}
	d.Connect()
	waitFrame(t, frames)
	d.applyLightingPacket(composeLightingPacket(frame.data), "openrgb", frame.generation, nil)
	noFrame(t, frames)
	d.startLightingQueue()
	d.writeColorEx([]byte{5, 6, 7}, 0)
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 5, 255, 6, 255, 7}) {
		t.Fatal(got)
	}
}

func TestTemporaryUnavailableWorkerRetriesAndStops(t *testing.T) {
	d, _ := lightingFixture(t, true)
	frames := make(chan []byte, 10)
	d.lightingWrite = func(p []byte) { frames <- p }
	requireLighting(t, d.SetLightingEffect("rainbow"))
	d.Connect()
	waitFrame(t, frames)
	d.lightingMu.Lock()
	worker := d.lightingWorker
	source := d.lightingCanonical
	d.lightingCanonical = &canonicalLighting{state: failingResolve{source.state}, effects: source.effects, defaults: source.defaults, resolver: source.resolver}
	d.lightingMu.Unlock()
	// The private token remains cancellable even with settings temporarily unavailable.
	d.stopLighting()
	waitDone(t, worker.done)
	d.lightingMu.Lock()
	d.lightingCanonical = source
	d.lightingMu.Unlock()
	d.setDeviceColor()
	waitFrame(t, frames)
	d.disconnectLighting(false)
	// No permanently exiting lock-owning queue on missing input color.
	d.lightingMu.Lock()
	d.DeviceProfile.Profiles[1] = DPIProfile{}
	d.lightingMu.Unlock()
	done := make(chan struct{})
	go func() { d.applyCurrentCluster([]byte{1, 2, 3}); close(done) }()
	waitDone(t, done)
}

func TestConcurrentLightingOwnersReceiverProfilesAndInput(t *testing.T) {
	d, _ := lightingFixture(t, true)
	var writes atomic.Int32
	d.lightingWrite = func([]byte) { writes.Add(1) }
	var cluster atomic.Pointer[common.ClusterController]
	var open atomic.Pointer[common.OpenRGBController]
	d.lightingClusterRegister = func(c *common.ClusterController) { cluster.Store(c) }
	d.lightingOpenRGBReconcile = func(_ string, c *common.OpenRGBController) { open.Store(c) }
	next := cloneLightingProfile(*d.DeviceProfile)
	next.Path = filepath.Join(t.TempDir(), "next.json")
	next.RGBCluster = true
	d.UserProfiles["next"] = &next
	d.Connect()
	d.startLightingQueue()
	var wait sync.WaitGroup
	for task := 0; task < 7; task++ {
		wait.Add(1)
		go func(task int) {
			defer wait.Done()
			for i := 0; i < 12; i++ {
				switch task {
				case 0:
					_ = d.SetLightingEffect([]string{"rainbow", "mouse", "gradient", "static"}[i%4])
					_ = d.SetLightingBrightness(uint8(i * 7))
					_ = d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: float64(i)})
				case 1:
					d.ProcessSetRgbCluster(i%2 == 0)
					d.ProcessSetOpenRgbIntegration(i%2 != 0)
				case 2:
					d.disconnectLighting(false)
					d.Connect()
				case 3:
					d.ChangeDeviceProfile([]string{"default", "next"}[i%2])
				case 4:
					d.SchedulerBrightness(uint8(i % 2))
					_ = d.setUserRGBOff(i%2 == 0)
				case 5:
					if c := cluster.Load(); c != nil {
						c.WriteColorEx([]byte{1, 2, 3}, 0)
					}
					if c := open.Load(); c != nil {
						c.WriteColorEx([]byte{4, 5, 6}, 0)
					}
				case 6:
					d.SelectMouseDPIStage(i % 5)
					d.SetMouseSniperMode(i%2 == 0)
					d.DPISnapshot()
					d.LightingSnapshot()
				}
			}
		}(task)
	}
	done := make(chan struct{})
	go func() { wait.Wait(); close(done) }()
	waitDone(t, done)
	d.stopCanonicalLighting(false)
}

func TestConnectionSetupCannotPublishAfterDisconnect(t *testing.T) {
	d, _ := lightingFixture(t, true)
	started, release := make(chan struct{}), make(chan struct{})
	d.lightingSetup = func() bool { close(started); <-release; return true }
	var writes atomic.Int32
	d.lightingWrite = func([]byte) { writes.Add(1) }
	done := make(chan struct{})
	go func() { d.Connect(); close(done) }()
	waitDone(t, started)
	d.disconnectLighting(false)
	close(release)
	waitDone(t, done)
	if d.lightingConnected() || writes.Load() != 0 {
		t.Fatal("stale setup became ready")
	}
}

func TestOneShotRetriesMissingIndicatorWithoutRetiring(t *testing.T) {
	d, _ := lightingFixture(t, true)
	frames := make(chan []byte, 10)
	d.lightingWrite = func(p []byte) { frames <- p }
	d.lightingMu.Lock()
	stage := d.DeviceProfile.Profiles[1]
	saved := stage.Color
	stage.Color = nil
	d.DeviceProfile.Profiles[1] = stage
	d.lightingMu.Unlock()
	d.Connect()
	d.lightingMu.Lock()
	worker := d.lightingWorker
	d.lightingMu.Unlock()
	if worker == nil {
		t.Fatal("no retry owner")
	}
	d.lightingMu.Lock()
	stage.Color = saved
	d.DeviceProfile.Profiles[1] = stage
	d.lightingMu.Unlock()
	waitFrame(t, frames)
	waitDone(t, worker.done)
}

func TestDPISettingsCopiesAndAuthoredStateIndependence(t *testing.T) {
	d, _ := lightingFixture(t, true)
	prior := render(t, d)
	stages := map[int]uint16{}
	colors := map[int]rgb.Color{}
	for id, p := range d.DeviceProfile.Profiles {
		stages[id] = p.Value
		colors[id] = *p.Color
	}
	colors[5] = rgb.Color{Red: 12, Green: 34, Blue: 56}
	if d.SaveMouseDPISettings(stages, colors) != 1 {
		t.Fatal("offline input settings")
	}
	colors[5] = rgb.Color{Red: 99}
	d.lightingMu.Lock()
	sniper := *d.DeviceProfile.Profiles[5].Color
	d.lightingMu.Unlock()
	if sniper.Red != 12 || sniper.Green != 34 || !bytes.Equal(prior, render(t, d)) {
		t.Fatal("input/authored alias")
	}
	if d.SaveMouseZoneColors(rgb.Color{}, map[int]rgb.Color{0: {Red: 88}}) != 0 {
		t.Fatal("legacy zone setter changed disconnected semantics")
	}
	if d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 88}) != nil {
		t.Fatal("canonical offline palette")
	}
}

func TestSourceInputDefaultsAndUpgrades(t *testing.T) {
	d, _ := lightingFixture(t, false)
	// Only the legacy bootstrap constructs input profiles. No HID is involved.
	oldRoot := pwd
	pwd = t.TempDir()
	defer func() { pwd = oldRoot }()
	requireLighting(t, os.MkdirAll(filepath.Join(pwd, "database", "profiles"), 0700))
	d.Serial = "7006" // source profile filenames use the receiver serial token
	d.DeviceProfile = nil
	d.saveDeviceProfile()
	p := d.DeviceProfile
	if p == nil || p.Profile != 1 || p.RGBProfile != "mouse" || *p.BrightnessSlider != 100 || p.OriginalBrightness != 100 || p.SleepMode != 15 || p.PollingRate != 4 || p.AngleSnapping != 0 || p.ButtonOptimization != 0 || p.RgbOff || p.RGBCluster || p.OpenRGBIntegration {
		t.Fatal("source defaults", p)
	}
	values := []uint16{400, 800, 1200, 1600, 3200, 200}
	for id, value := range values {
		if p.Profiles[id].Value != value || p.Profiles[id].PackerIndex != id+1 || !reflect.DeepEqual(p.Profiles[id].ColorIndex, map[int][]int{0: {0, 2, 4}}) {
			t.Fatal("DPI defaults", id)
		}
	}
	if !reflect.DeepEqual(p.ZoneColors[0].ColorIndex, []int{1, 3, 5}) || p.ZoneColors[0].Name != "Logo" {
		t.Fatal("source topology")
	}
	proposal := cloneLightingProfile(*p)
	delete(proposal.Profiles, 5)
	proposal.SleepMode = 0
	proposal.PollingRate = 0
	proposal = prepareLightingInputProfile(proposal)
	if !proposal.Profiles[5].Sniper || proposal.Profiles[5].Value != 200 || proposal.Profiles[5].Color.Hex != "#ffff00" || proposal.SleepMode != 15 || proposal.PollingRate != 4 {
		t.Fatal("input upgrade")
	}
	if !reflect.DeepEqual(rgbProfileUpgrade, []string{"gradient", "pastelrainbow", "pastelspiralrainbow", "flame", "aurora", "cyberpunkglitch", "tokyonight"}) {
		t.Fatal("fallback RGB upgrade catalogue changed")
	}
}

func TestDelayedBlankCannotOverwriteNewerExternalFrame(t *testing.T) {
	for _, owner := range []string{"cluster", "openrgb"} {
		t.Run(owner, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			frames := make(chan []byte, 10)
			d.lightingWrite = func(p []byte) { frames <- p }
			blocked, release := make(chan struct{}), make(chan struct{})
			var once atomic.Bool
			d.lightingBeforeApply = func(source string) {
				if source == "blank" && once.CompareAndSwap(false, true) {
					close(blocked)
					<-release
				}
			}
			d.Connect()
			waitFrame(t, frames)
			if owner == "cluster" {
				d.ProcessSetRgbCluster(true)
				requireLighting(t, d.setUserRGBOff(true))
			} else {
				d.ProcessSetOpenRgbIntegration(true)
			}
			waitDone(t, blocked)
			d.lightingMu.Lock()
			blank := d.lightingWorker
			generation := d.externalGeneration
			d.lightingMu.Unlock()
			d.applyLightingPacket(composeLightingPacket([]byte{11, 22, 33}), owner, generation, nil)
			if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 11, 255, 22, 255, 33}) {
				t.Fatal(got)
			}
			close(release)
			waitDone(t, blank.done)
			noFrame(t, frames)
		})
	}
}
