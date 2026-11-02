package harpoonWU

import (
	"LumenForge/src/common"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"sync/atomic"
	"testing"
)

// Replug mirrors Init's data attachment to a new object, without any HID,
// listener, polling command or global registry. Old object identity is retained
// only by stale callback closures in the tests below.
func freshUSBFixture(t *testing.T, old *Device, rt *lightingsettings.IndependentDeviceRuntime) *Device {
	t.Helper()
	data, err := os.ReadFile(old.DeviceProfile.Path)
	requireLighting(t, err)
	var profile DeviceProfile
	requireLighting(t, json.Unmarshal(data, &profile))
	d := &Device{Usb: true, Connected: true, Serial: old.Serial, Product: old.Product, Path: old.Path, VendorId: 6940, ProductId: 7006, LEDChannels: 2, ChangeableLedChannels: 1, ZoneAmount: 1, DPIAmount: 6, MinDPI: 200, MaxDPI: 10000, DeviceProfile: &profile, UserProfiles: map[string]*DeviceProfile{"default": &profile}}
	d.lightingWrite = func([]byte) {}
	d.lightingDPIWrite = func([]byte, []byte) error { return nil }
	d.lightingClusterRemove = func(string) {}
	d.lightingClusterRegister = func(*common.ClusterController) {}
	d.lightingOpenRGBReconcile = func(string, *common.OpenRGBController) {}
	requireLighting(t, d.attachLightingSource(rt))
	t.Cleanup(d.exitLighting)
	return d
}

func TestUSBMissingIndicatorReleasesLegacyAndCanonicalOutputLock(t *testing.T) {
	d, rt := lightingFixture(t, false)
	saved := d.DeviceProfile.Profiles[1]
	missing := saved
	missing.Color = nil
	d.DeviceProfile.Profiles[1] = missing
	done := make(chan struct{})
	go func() { d.applyLegacyOpenRGBFrame([]byte{1, 2, 3}); close(done) }()
	waitDone(t, done) // exact formerly leaking fallback branch, no HID reached
	if !d.deviceLock.TryLock() {
		t.Fatal("legacy missing indicator leaked deviceLock")
	}
	d.deviceLock.Unlock()
	requireLighting(t, d.attachLightingSource(rt))
	d.setUSBLightingAvailable(true)
	// Canonical OpenRGB exercises that same unavailable-input condition after
	// acquiring serialization, and must not terminate or leak its queue worker.
	if d.ProcessSetOpenRgbIntegration(true) != 1 {
		t.Fatal("owner")
	}
	d.applyLightingPacket(composeLightingPacket([]byte{1, 2, 3}), "openrgb", d.externalGeneration, nil)
	if !d.deviceLock.TryLock() {
		t.Fatal("canonical missing indicator leaked deviceLock")
	}
	d.deviceLock.Unlock()
	d.lightingMu.Lock()
	d.DeviceProfile.Profiles[1] = saved
	worker := d.lightingWorker
	d.cancelLightingLocked()
	d.lightingMu.Unlock()
	if worker != nil {
		waitDone(t, worker.done)
	}
	frames := make(chan []byte, 30)
	d.lightingWrite = func(p []byte) { frames <- p }
	// Cancelled retry/blank workers cannot emit after the restored input below.
	d.startLightingQueue()
	d.writeColorEx([]byte{4, 5, 6}, 0)
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 4, 255, 5, 255, 6}) {
		t.Fatal(got)
	}
	if d.ProcessSetOpenRgbIntegration(false) != 1 {
		t.Fatal("OpenRGB release deadlocked")
	}
	waitFrame(t, frames) // subsequent Native output owns deviceLock
	if d.ProcessSetRgbCluster(true) != 1 {
		t.Fatal("Cluster transition")
	}
	d.writeColorCluster([]byte{7, 8, 9}, 0)
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 7, 255, 8, 255, 9}) {
		t.Fatal(got)
	}
	done = make(chan struct{})
	go func() { d.StopDirty(); close(done) }()
	waitDone(t, done)
	next := freshUSBFixture(t, d, rt)
	next.lightingWrite = func(p []byte) { frames <- p }
	startTestUSB(next)
	next.writeColorCluster([]byte{10, 11, 12}, 0)
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 10, 255, 11, 255, 12}) {
		t.Fatal("fresh USB locked", got)
	}
}

func TestUSBReplugRetiresEveryOldInstanceOutputSource(t *testing.T) {
	for _, source := range []string{"native", "cluster", "openrgb"} {
		t.Run(source, func(t *testing.T) {
			d, rt := lightingFixture(t, true)
			requireLighting(t, d.SetLightingBrightness(37))
			frames := make(chan []byte, 20)
			d.lightingWrite = func(p []byte) { frames <- p }
			startTestUSB(d)
			waitFrame(t, frames)
			var cluster *common.ClusterController
			var open *common.OpenRGBController
			d.lightingClusterRegister = func(c *common.ClusterController) { cluster = c }
			d.lightingOpenRGBReconcile = func(_ string, c *common.OpenRGBController) {
				if c != nil {
					open = c
				}
			}
			if source == "cluster" {
				d.ProcessSetRgbCluster(true)
			}
			if source == "openrgb" {
				d.ProcessSetOpenRgbIntegration(true)
				waitFrame(t, frames)
			}
			blocked, release := make(chan struct{}), make(chan struct{})
			var once atomic.Bool
			d.lightingBeforeApply = func(s string) {
				if s == source && once.CompareAndSwap(false, true) {
					close(blocked)
					<-release
				}
			}
			done := make(chan struct{})
			if source == "native" {
				d.setDeviceColor()
				d.lightingMu.Lock()
				worker := d.lightingWorker
				d.lightingMu.Unlock()
				go func() { <-worker.done; close(done) }()
			} else if source == "cluster" {
				go func() { cluster.WriteColorEx([]byte{90, 91, 92}, 0); close(done) }()
			} else {
				// Model a frame already dequeued by the external queue at serialization.
				generation := d.externalGeneration
				go func() {
					d.applyLightingPacket(composeLightingPacket([]byte{90, 91, 92}), "openrgb", generation, nil)
					close(done)
				}()
			}
			waitDone(t, blocked)
			stopped := make(chan struct{})
			go func() { d.StopDirty(); close(stopped) }()
			waitDone(t, stopped)
			next := freshUSBFixture(t, d, rt)
			nextFrames := make(chan []byte, 20)
			next.lightingWrite = func(p []byte) { nextFrames <- p }
			next.startLightingQueue()
			startTestUSB(next)
			snapshot, ok := next.LightingSnapshot()
			if !ok || snapshot.Brightness != 37 {
				t.Fatal("lost desired state", snapshot)
			}
			if source != "cluster" {
				waitFrame(t, nextFrames)
			}
			close(release)
			waitDone(t, done)
			noFrame(t, frames)
			// Retained old registration closures remain inert after fresh USB startup.
			if cluster != nil {
				cluster.WriteColorEx([]byte{90, 91, 92}, 0)
			}
			if open != nil {
				open.WriteColorEx([]byte{90, 91, 92}, 0)
			}
			noFrame(t, frames)
			if source == "cluster" {
				next.writeColorCluster([]byte{1, 2, 3}, 0)
				waitFrame(t, nextFrames)
			}
			if source == "openrgb" {
				next.writeColorEx([]byte{1, 2, 3}, 0)
				waitFrame(t, nextFrames)
			}
		})
	}
}

func TestUSBFreshInitializationReplaysProfileRGBOffAndCurrentDesiredState(t *testing.T) {
	d, rt := lightingFixture(t, true)
	requireLighting(t, d.SetLightingEffect("mouse"))
	requireLighting(t, d.SetLightingBrightness(40))
	requireLighting(t, d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 200, Green: 100, Blue: 50}))
	requireLighting(t, d.setUserRGBOff(true))
	d.SchedulerBrightness(0) // runtime darkness must not cross fresh Init
	d.StopDirty()
	next := freshUSBFixture(t, d, rt)
	frames := make(chan []byte, 20)
	next.lightingWrite = func(p []byte) { frames <- p }
	var dpi []byte
	next.lightingDPIWrite = func(command, data []byte) error { dpi = append([]byte(nil), data...); return nil }
	startTestUSB(next)
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 0, 255, 0, 255, 0}) {
		t.Fatal("RGB-off startup replay", got)
	}
	if !bytes.Equal(dpi, []byte{0xe8, 3}) || next.SniperMode || next.schedulerDark {
		t.Fatal("borrowed W defaults/runtime", dpi)
	}
	requireLighting(t, next.setUserRGBOff(false))
	if got := waitFrame(t, frames); !bytes.Equal(got, []byte{255, 80, 255, 40, 255, 20}) {
		t.Fatal("latest persisted desired restoration", got)
	}
}

func TestUSBPollingPersistsBeforeRetiringAndPreservesCommand(t *testing.T) {
	d, rt := lightingFixture(t, true)
	d.PollingRates = map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 4: "1000 Hz / 1 msec"}
	d.setUSBLightingAvailable(true)
	before := inputJSON(t, d)
	generation := d.externalGeneration
	var calls atomic.Int32
	d.lightingPollingWrite = func(command, data []byte, read bool) error {
		calls.Add(1)
		if read || !bytes.Equal(command, []byte{1, 1, 0}) || !bytes.Equal(data, []byte{1}) || !d.lightingExited() {
			t.Error("polling contract")
		}
		disk, err := os.ReadFile(d.DeviceProfile.Path)
		requireLighting(t, err)
		var p DeviceProfile
		requireLighting(t, json.Unmarshal(disk, &p))
		if p.PollingRate != 1 {
			t.Error("polling command before persistence")
		}
		return nil
	}
	d.lightingProfileWrite = func(string, []byte) error { return errors.New("injected") }
	if d.UpdatePollingRate(1) != 0 || d.lightingExited() || d.externalGeneration != generation || calls.Load() != 0 || !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("failed polling persistence changed lifecycle")
	}
	d.lightingProfileWrite = nil
	if d.UpdatePollingRate(1) != 1 || calls.Load() != 1 || !d.lightingExited() {
		t.Fatal("polling reconfiguration")
	}
	next := freshUSBFixture(t, d, rt)
	if next.DeviceProfile.PollingRate != 1 {
		t.Fatal("replug lost polling preference")
	}
}

func TestUSBStopDirtyDoesNotWaitForIssuedBlockingRead(t *testing.T) {
	d, rt := lightingFixture(t, true)
	entered, release := make(chan struct{}), make(chan struct{})
	var released atomic.Bool
	unblock := func() {
		if released.CompareAndSwap(false, true) {
			close(release)
		}
	}
	t.Cleanup(unblock)
	d.lightingWrite = nil
	d.lightingColorTransfer = func(_, _ []byte, read bool) error {
		if !read {
			t.Error("color write must retain blocking response read")
		}
		close(entered)
		<-release // models the already-issued WU HID round trip holding deviceLock
		return nil
	}
	startTestUSB(d)
	waitDone(t, entered)
	d.lightingMu.Lock()
	worker := d.lightingWorker
	d.lightingMu.Unlock()
	stopped := make(chan struct{})
	go func() { d.StopDirty(); close(stopped) }()
	waitDone(t, stopped)
	select {
	case <-worker.stop:
	default:
		t.Fatal("blocked read prevented cancellation")
	}
	next := freshUSBFixture(t, d, rt)
	frames := make(chan []byte, 10)
	next.lightingWrite = func(p []byte) { frames <- p }
	startTestUSB(next)
	waitFrame(t, frames) // fresh own-USB instance is independent of old output lock
	unblock()
	waitDone(t, worker.done)
	if d.SetLightingEffect("mouse") == nil {
		t.Fatal("stopped old instance accepted mutation")
	}
}

func TestUSBOneShotRetriesIndicatorUnavailableAtFinalWrite(t *testing.T) {
	d, _ := lightingFixture(t, true)
	frames := make(chan []byte, 10)
	d.lightingWrite = func(p []byte) { frames <- p }
	arrived, release := make(chan struct{}), make(chan struct{})
	var once atomic.Bool
	d.lightingBeforeApply = func(source string) {
		if source == "native" && once.CompareAndSwap(false, true) {
			close(arrived)
			<-release
		}
	}
	startTestUSB(d)
	waitDone(t, arrived)
	d.lightingMu.Lock()
	stage := d.DeviceProfile.Profiles[1]
	color := stage.Color
	stage.Color = nil
	d.DeviceProfile.Profiles[1] = stage
	worker := d.lightingWorker
	d.lightingMu.Unlock()
	close(release)
	// Another source uses the same missing-indicator gate synchronously, without
	// borrowing or leaking the Native worker's lock/ownership.
	if d.applyLightingPacket(composeLightingPacket([]byte{1, 2, 3}), "native", 0, worker) {
		t.Fatal("unavailable indicator applied")
	}
	d.lightingMu.Lock()
	stage.Color = color
	d.DeviceProfile.Profiles[1] = stage
	d.lightingMu.Unlock()
	waitFrame(t, frames)
	waitDone(t, worker.done)
}
