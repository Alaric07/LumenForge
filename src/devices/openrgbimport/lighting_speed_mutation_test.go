package openrgbimport

import (
	"LumenForge/src/rgb"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type lightingDeadlineContext struct {
	context.Context
	done <-chan struct{}
}

func (c lightingDeadlineContext) Done() <-chan struct{} { return c.done }
func (lightingDeadlineContext) Err() error              { return context.DeadlineExceeded }

func installLightingSpeedPersistenceSeams(t *testing.T) string {
	t.Helper()
	previousDirectory := rgbProfileDir
	previousSave := saveRGBProfileData
	directory := t.TempDir()
	rgbProfileDir = func() string { return directory }
	t.Cleanup(func() {
		rgbProfileDir = previousDirectory
		saveRGBProfileData = previousSave
	})
	return directory
}

func newLightingSpeedMutationDevice(effect string, speed float64) *Device {
	device := newLightingMutationDevice()
	device.effect = effect
	device.RGBModes = []string{effect, "static", "off"}
	device.DeviceProfile.RGBProfile = effect
	device.Rgb = &rgb.RGB{
		Device: device.Product,
		Profiles: map[string]rgb.Profile{
			effect: {
				ProfileName: effect,
				Speed:       speed,
				StartColor:  rgb.Color{Red: 20, Green: 40, Blue: 60, Brightness: 1},
				EndColor:    rgb.Color{Red: 90, Green: 70, Blue: 50, Brightness: 1},
			},
		},
	}
	return device
}

func stopLightingSpeedWorker(device *Device) {
	device.effectTransitionMu.Lock()
	device.mu.Lock()
	device.stopEffectLoopLocked()
	device.mu.Unlock()
	device.effectTransitionMu.Unlock()
}

func lightingSpeedRunnerState(t *testing.T, device *Device) (float64, rgb.Color, rgb.Color) {
	t.Helper()
	device.mu.Lock()
	defer device.mu.Unlock()
	if device.rgbRunner == nil || device.rgbRunner.RGBStartColor == nil || device.rgbRunner.RGBEndColor == nil {
		t.Fatal("lighting runner color state is not initialized")
	}
	return device.rgbRunner.RgbModeSpeed, *device.rgbRunner.RGBStartColor, *device.rgbRunner.RGBEndColor
}

func readLightingRGBProfile(t *testing.T, directory, effect string) rgb.Profile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(directory, lightingDeviceTestSerial+".json"))
	if err != nil {
		t.Fatalf("read persisted RGB profile: %v", err)
	}
	var state rgb.RGB
	if err = json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode persisted RGB profile: %v", err)
	}
	profile, ok := state.Profiles[effect]
	if !ok {
		t.Fatalf("persisted RGB profile does not contain %q", effect)
	}
	return profile
}

func TestOpenRGBLightingSetEffectSpeedPersistenceSources(t *testing.T) {
	t.Run("Gradient without override", func(t *testing.T) {
		installLightingDeviceTestSeams(t)
		installLightingSpeedPersistenceSeams(t)
		device := newLightingSpeedMutationDevice("gradient", 2)

		if err := device.SetEffectSpeed(lightingDeviceTestSerial, "gradient", 4); err != nil {
			t.Fatalf("SetEffectSpeed: %v", err)
		}
		runnerSpeed, _, _ := lightingSpeedRunnerState(t, device)
		stopLightingSpeedWorker(device)
		if runnerSpeed != 4 {
			t.Fatalf("Gradient runner speed = %v, want base speed 4", runnerSpeed)
		}
	})

	t.Run("base device definition", func(t *testing.T) {
		installLightingDeviceTestSeams(t)
		rgbDirectory := installLightingSpeedPersistenceSeams(t)
		device := newLightingSpeedMutationDevice("rainbow", 2)
		var globalProfileBefore *rgb.Profile
		if current := rgb.GetRgbProfile("rainbow"); current != nil {
			copied := *current
			globalProfileBefore = &copied
		}

		if err := device.SetEffectSpeed(lightingDeviceTestSerial, "rainbow", 4.25); err != nil {
			t.Fatalf("SetEffectSpeed: %v", err)
		}
		if !device.running || device.GetEffect() != "rainbow" {
			t.Fatalf("effect reapply state = running %t, effect %q", device.running, device.GetEffect())
		}
		stopLightingSpeedWorker(device)
		if got := device.GetRgbProfile("rainbow").Speed; got != 4.25 {
			t.Fatalf("base speed = %v, want 4.25", got)
		}
		if got := readLightingRGBProfile(t, rgbDirectory, "rainbow").Speed; got != 4.25 {
			t.Fatalf("persisted base speed = %v, want 4.25", got)
		}
		if globalProfileBefore != nil {
			globalProfileAfter := rgb.GetRgbProfile("rainbow")
			if globalProfileAfter == nil || globalProfileAfter.Speed != globalProfileBefore.Speed {
				t.Fatalf("global RGB database changed from %#v to %#v", globalProfileBefore, globalProfileAfter)
			}
		}
		snapshot, ok := device.LightingSnapshot()
		if !ok || snapshot.BaseDefinition == nil || snapshot.Effective == nil ||
			snapshot.BaseDefinition.Speed != 4.25 || snapshot.Effective.Speed != 4.25 {
			t.Fatalf("Lighting snapshot after base speed update = %#v", snapshot)
		}
	})

	t.Run("enabled local override", func(t *testing.T) {
		profileDirectory, _ := installLightingDeviceTestSeams(t)
		installLightingSpeedPersistenceSeams(t)
		device := newLightingSpeedMutationDevice("rain", 2)
		device.DeviceProfile.RGBOverride = &RGBOverride{
			Enabled:        true,
			RGBStartColor:  rgb.Color{Red: 1, Green: 2, Blue: 3},
			RGBMiddleColor: rgb.Color{Red: 4, Green: 5, Blue: 6},
			RGBEndColor:    rgb.Color{Red: 7, Green: 8, Blue: 9},
			RgbModeSpeed:   3,
		}
		overrideBefore := *device.DeviceProfile.RGBOverride

		if err := device.SetEffectSpeed(lightingDeviceTestSerial, "rain", 2.5); err != nil {
			t.Fatalf("SetEffectSpeed: %v", err)
		}
		runnerSpeed, runnerStart, runnerEnd := lightingSpeedRunnerState(t, device)
		stopLightingSpeedWorker(device)
		if runnerSpeed != 2.5 || runnerStart != device.DeviceProfile.RGBOverride.RGBStartColor ||
			runnerEnd != device.DeviceProfile.RGBOverride.RGBEndColor {
			t.Fatalf("override runner state = speed %v, start %#v, end %#v", runnerSpeed, runnerStart, runnerEnd)
		}
		if got := device.GetRgbProfile("rain").Speed; got != 2 {
			t.Fatalf("base speed = %v, want unchanged 2", got)
		}
		if *device.DeviceProfile.RGBOverride != (RGBOverride{
			Enabled:        overrideBefore.Enabled,
			RGBStartColor:  overrideBefore.RGBStartColor,
			RGBMiddleColor: overrideBefore.RGBMiddleColor,
			RGBEndColor:    overrideBefore.RGBEndColor,
			RgbModeSpeed:   2.5,
		}) {
			t.Fatalf("non-Gradient speed mutation changed unrelated override state: %#v", device.DeviceProfile.RGBOverride)
		}
		persisted := readLightingDeviceProfile(t, profileDirectory)
		if persisted.RGBOverride == nil || !persisted.RGBOverride.Enabled || persisted.RGBOverride.RgbModeSpeed != 2.5 ||
			persisted.RGBOverride.RGBStartColor.Red != 1 || persisted.RGBOverride.RGBMiddleColor.Green != 5 ||
			persisted.RGBOverride.RGBEndColor.Blue != 9 {
			t.Fatalf("persisted override = %#v", persisted.RGBOverride)
		}
		snapshot, ok := device.LightingSnapshot()
		if !ok || snapshot.Override == nil || snapshot.Effective == nil ||
			snapshot.Override.Speed != 2.5 || snapshot.Effective.Speed != 2.5 || snapshot.BaseDefinition.Speed != 2 {
			t.Fatalf("Lighting snapshot after override speed update = %#v", snapshot)
		}
	})

	t.Run("Gradient ignores enabled override", func(t *testing.T) {
		installLightingDeviceTestSeams(t)
		rgbDirectory := installLightingSpeedPersistenceSeams(t)
		device := newLightingSpeedMutationDevice("gradient", 2)
		device.DeviceProfile.RGBOverride = &RGBOverride{
			Enabled:        true,
			RGBStartColor:  rgb.Color{Red: 1, Green: 2, Blue: 3},
			RGBMiddleColor: rgb.Color{Red: 4, Green: 5, Blue: 6},
			RGBEndColor:    rgb.Color{Red: 7, Green: 8, Blue: 9},
			RgbModeSpeed:   8,
		}
		overrideBefore := *device.DeviceProfile.RGBOverride

		if err := device.SetEffectSpeed(lightingDeviceTestSerial, "gradient", 6); err != nil {
			t.Fatalf("SetEffectSpeed: %v", err)
		}
		runnerSpeed, runnerStart, runnerEnd := lightingSpeedRunnerState(t, device)
		stopLightingSpeedWorker(device)
		if runnerSpeed != 6 {
			t.Fatalf("Gradient runner speed = %v, want base speed 6", runnerSpeed)
		}
		if runnerStart != device.DeviceProfile.RGBOverride.RGBStartColor || runnerEnd != device.DeviceProfile.RGBOverride.RGBEndColor {
			t.Fatalf("Gradient runner did not retain override colors: start %#v, end %#v", runnerStart, runnerEnd)
		}
		if device.DeviceProfile.RGBOverride.RgbModeSpeed != 8 {
			t.Fatalf("Gradient changed override speed to %v", device.DeviceProfile.RGBOverride.RgbModeSpeed)
		}
		if *device.DeviceProfile.RGBOverride != overrideBefore {
			t.Fatalf("Gradient speed mutation changed override state from %#v to %#v", overrideBefore, device.DeviceProfile.RGBOverride)
		}
		if got := readLightingRGBProfile(t, rgbDirectory, "gradient").Speed; got != 6 {
			t.Fatalf("persisted Gradient speed = %v, want 6", got)
		}
		snapshot, ok := device.LightingSnapshot()
		if !ok || snapshot.BaseDefinition == nil || snapshot.Effective == nil || snapshot.Override == nil ||
			snapshot.BaseDefinition.Speed != 6 || snapshot.Effective.Speed != 6 || snapshot.Override.Speed != 8 {
			t.Fatalf("Gradient snapshot precedence = %#v", snapshot)
		}

		if err := device.SetEffect("gradient"); err != nil {
			t.Fatalf("reapply Gradient: %v", err)
		}
		reappliedSpeed, reappliedStart, reappliedEnd := lightingSpeedRunnerState(t, device)
		stopLightingSpeedWorker(device)
		if reappliedSpeed != 6 || reappliedStart != device.DeviceProfile.RGBOverride.RGBStartColor ||
			reappliedEnd != device.DeviceProfile.RGBOverride.RGBEndColor {
			t.Fatalf("reapplied Gradient runner state = speed %v, start %#v, end %#v", reappliedSpeed, reappliedStart, reappliedEnd)
		}
	})
}

func TestOpenRGBLightingInitialOutputWait(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		initialOutput := make(chan error, 1)
		initialOutput <- nil
		if err := waitForInitialEffectOutput(initialOutput); err != nil {
			t.Fatalf("initial output success returned %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		want := errors.New("test initial output failure")
		initialOutput := make(chan error, 1)
		initialOutput <- want
		if err := waitForInitialEffectOutput(initialOutput); !errors.Is(err, want) {
			t.Fatalf("initial output failure = %v, want %v", err, want)
		}
	})

	t.Run("timeout permits late publication", func(t *testing.T) {
		previousContext := initialEffectOutputContext
		done := make(chan struct{})
		close(done)
		initialEffectOutputContext = func() (context.Context, context.CancelFunc) {
			return lightingDeadlineContext{Context: context.Background(), done: done}, func() {}
		}
		t.Cleanup(func() { initialEffectOutputContext = previousContext })

		initialOutput := make(chan error, 1)
		err := waitForInitialEffectOutput(initialOutput)
		if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "wait for initial OpenRGB effect output") {
			t.Fatalf("initial output timeout = %v", err)
		}
		published := make(chan struct{})
		go func() {
			initialOutput <- nil
			close(published)
		}()
		<-published
	})
}

func TestOpenRGBLightingSetEffectSpeedInitialOutputTimeoutRetainsPersistedSpeed(t *testing.T) {
	installLightingDeviceTestSeams(t)
	rgbDirectory := installLightingSpeedPersistenceSeams(t)
	device := newLightingSpeedMutationDevice("rainbow", 2)
	profileBefore := *device.GetRgbProfile("rainbow")
	previousContext := initialEffectOutputContext
	timeoutDone := make(chan struct{})
	initialEffectOutputContext = func() (context.Context, context.CancelFunc) {
		return lightingDeadlineContext{Context: context.Background(), done: timeoutDone}, func() {}
	}
	t.Cleanup(func() { initialEffectOutputContext = previousContext })

	outputStarted := make(chan struct{}, 1)
	releaseOutput := make(chan struct{})
	outputReturned := make(chan struct{}, 1)
	sendLightingPersistentFrame = func(conn net.Conn, _ uint32, _ []byte) (net.Conn, error) {
		select {
		case outputStarted <- struct{}{}:
		default:
		}
		<-releaseOutput
		select {
		case outputReturned <- struct{}{}:
		default:
		}
		return conn, nil
	}

	result := make(chan error, 1)
	go func() {
		result <- device.SetEffectSpeed(lightingDeviceTestSerial, "rainbow", 5)
	}()
	<-outputStarted
	close(timeoutDone)
	err := <-result
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "wait for initial OpenRGB effect output") {
		t.Fatalf("SetEffectSpeed timeout = %v", err)
	}
	if got := device.GetRgbProfile("rainbow").Speed; got != 5 {
		t.Fatalf("in-memory speed after timeout = %v, want persisted 5", got)
	}
	if got := readLightingRGBProfile(t, rgbDirectory, "rainbow").Speed; got != 5 {
		t.Fatalf("saved speed after timeout = %v, want 5", got)
	}

	close(releaseOutput)
	<-outputReturned
	stopLightingSpeedWorker(device)
	profileAfter := device.GetRgbProfile("rainbow")
	if profileAfter.StartColor != profileBefore.StartColor || profileAfter.MiddleColor != profileBefore.MiddleColor ||
		profileAfter.EndColor != profileBefore.EndColor || device.GetEffect() != "rainbow" {
		t.Fatalf("timeout changed effect or color state: effect %q, profile %#v", device.GetEffect(), profileAfter)
	}
}

func TestOpenRGBLightingSetEffectSpeedValidation(t *testing.T) {
	tests := []struct {
		name   string
		device func() *Device
		serial string
		effect string
		speed  float64
		match  string
	}{
		{name: "nil receiver", device: func() *Device { return nil }, serial: lightingDeviceTestSerial, effect: "rainbow", speed: 2, match: "not available"},
		{name: "detached", device: func() *Device {
			d := newLightingSpeedMutationDevice("rainbow", 2)
			d.lifecycleDetached = true
			return d
		}, serial: lightingDeviceTestSerial, effect: "rainbow", speed: 2, match: "detached"},
		{name: "identity", device: func() *Device { return newLightingSpeedMutationDevice("rainbow", 2) }, serial: "openrgb-other", effect: "rainbow", speed: 2, match: "identity"},
		{name: "unavailable", device: func() *Device { d := newLightingSpeedMutationDevice("rainbow", 2); d.controllerId = -1; return d }, serial: lightingDeviceTestSerial, effect: "rainbow", speed: 2, match: "controllerId"},
		{name: "missing profile", device: func() *Device { d := newLightingSpeedMutationDevice("rainbow", 2); d.DeviceProfile = nil; return d }, serial: lightingDeviceTestSerial, effect: "rainbow", speed: 2, match: "active"},
		{name: "inactive profile", device: func() *Device {
			d := newLightingSpeedMutationDevice("rainbow", 2)
			d.DeviceProfile.Active = false
			return d
		}, serial: lightingDeviceTestSerial, effect: "rainbow", speed: 2, match: "active"},
		{name: "cluster", device: func() *Device {
			d := newLightingSpeedMutationDevice("rainbow", 2)
			d.DeviceProfile.RGBCluster = true
			return d
		}, serial: lightingDeviceTestSerial, effect: "rainbow", speed: 2, match: "device is controlled by RGB cluster"},
		{name: "stale effect", device: func() *Device { return newLightingSpeedMutationDevice("rainbow", 2) }, serial: lightingDeviceTestSerial, effect: "wave", speed: 2, match: "stale"},
		{name: "unsupported catalogue", device: func() *Device {
			d := newLightingSpeedMutationDevice("rainbow", 2)
			d.RGBModes = []string{"static"}
			return d
		}, serial: lightingDeviceTestSerial, effect: "rainbow", speed: 2, match: "unsupported"},
		{name: "no speed capability", device: func() *Device { return newLightingSpeedMutationDevice("static", 2) }, serial: lightingDeviceTestSerial, effect: "static", speed: 2, match: "does not support"},
		{name: "Off has no speed capability", device: func() *Device { return newLightingSpeedMutationDevice("off", 2) }, serial: lightingDeviceTestSerial, effect: "off", speed: 2, match: "does not support"},
		{name: "CPU temperature has no speed capability", device: func() *Device { return newLightingSpeedMutationDevice("cpu-temperature", 2) }, serial: lightingDeviceTestSerial, effect: "cpu-temperature", speed: 2, match: "does not support"},
		{name: "GPU temperature has no speed capability", device: func() *Device { return newLightingSpeedMutationDevice("gpu-temperature", 2) }, serial: lightingDeviceTestSerial, effect: "gpu-temperature", speed: 2, match: "does not support"},
		{name: "unknown capability", device: func() *Device { return newLightingSpeedMutationDevice("future-effect", 2) }, serial: lightingDeviceTestSerial, effect: "future-effect", speed: 2, match: "does not support"},
		{name: "missing definition", device: func() *Device {
			d := newLightingSpeedMutationDevice("rainbow", 2)
			d.Rgb.Profiles = map[string]rgb.Profile{}
			return d
		}, serial: lightingDeviceTestSerial, effect: "rainbow", speed: 2, match: "definition"},
		{name: "NaN", device: func() *Device { return newLightingSpeedMutationDevice("rainbow", 2) }, serial: lightingDeviceTestSerial, effect: "rainbow", speed: math.NaN(), match: "invalid"},
		{name: "positive infinity", device: func() *Device { return newLightingSpeedMutationDevice("rainbow", 2) }, serial: lightingDeviceTestSerial, effect: "rainbow", speed: math.Inf(1), match: "invalid"},
		{name: "negative infinity", device: func() *Device { return newLightingSpeedMutationDevice("rainbow", 2) }, serial: lightingDeviceTestSerial, effect: "rainbow", speed: math.Inf(-1), match: "invalid"},
		{name: "below generic range", device: func() *Device { return newLightingSpeedMutationDevice("rainbow", 2) }, serial: lightingDeviceTestSerial, effect: "rainbow", speed: 0.99, match: "range"},
		{name: "above range", device: func() *Device { return newLightingSpeedMutationDevice("rainbow", 2) }, serial: lightingDeviceTestSerial, effect: "rainbow", speed: 10.01, match: "range"},
		{name: "below calibrated range", device: func() *Device { return newLightingSpeedMutationDevice("flame", 1) }, serial: lightingDeviceTestSerial, effect: "flame", speed: 0.09, match: "range"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installLightingDeviceTestSeams(t)
			installLightingSpeedPersistenceSeams(t)
			device := test.device()
			err := device.SetEffectSpeed(test.serial, test.effect, test.speed)
			if err == nil || !strings.Contains(err.Error(), test.match) {
				t.Fatalf("SetEffectSpeed error = %v, want text %q", err, test.match)
			}
		})
	}

	for _, test := range []struct {
		effect string
		speed  float64
	}{{effect: "rainbow", speed: 1}, {effect: "rainbow", speed: 10}, {effect: "flame", speed: 0.1}, {effect: "flame", speed: 10}} {
		t.Run("accepted boundary "+test.effect, func(t *testing.T) {
			installLightingDeviceTestSeams(t)
			installLightingSpeedPersistenceSeams(t)
			device := newLightingSpeedMutationDevice(test.effect, 2)
			if err := device.SetEffectSpeed(lightingDeviceTestSerial, test.effect, test.speed); err != nil {
				t.Fatalf("SetEffectSpeed(%v): %v", test.speed, err)
			}
			stopLightingSpeedWorker(device)
		})
	}
}

func TestOpenRGBLightingSetEffectSpeedPersistenceFailureRollsBack(t *testing.T) {
	t.Run("RGB definition", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		installLightingSpeedPersistenceSeams(t)
		saveRGBProfileData = func(string, interface{}) error { return errors.New("test RGB save failure") }
		device := newLightingSpeedMutationDevice("rainbow", 2)

		err := device.SetEffectSpeed(lightingDeviceTestSerial, "rainbow", 5)
		if err == nil || !strings.Contains(err.Error(), "save OpenRGB effect speed") {
			t.Fatalf("SetEffectSpeed error = %v", err)
		}
		if got := device.GetRgbProfile("rainbow").Speed; got != 2 {
			t.Fatalf("speed after failed save = %v, want restored 2", got)
		}
		if device.running || calls.colors != 0 || calls.frames != 0 || calls.persistentFrames != 0 {
			t.Fatalf("failed save re-applied output: running %t, calls %#v", device.running, calls)
		}
	})

	t.Run("enabled override", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		installLightingSpeedPersistenceSeams(t)
		blocked := filepath.Join(t.TempDir(), "profiles")
		if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		deviceProfileDir = func() string { return blocked }
		device := newLightingSpeedMutationDevice("rainbow", 2)
		device.DeviceProfile.RGBOverride = &RGBOverride{Enabled: true, RgbModeSpeed: 3}

		if err := device.SetEffectSpeed(lightingDeviceTestSerial, "rainbow", 5); err == nil {
			t.Fatal("SetEffectSpeed succeeded despite override persistence failure")
		}
		if device.DeviceProfile.RGBOverride == nil || device.DeviceProfile.RGBOverride.RgbModeSpeed != 3 {
			t.Fatalf("override speed after failed save = %#v", device.DeviceProfile.RGBOverride)
		}
		if device.running || calls.colors != 0 || calls.frames != 0 || calls.persistentFrames != 0 {
			t.Fatalf("failed override save re-applied output: running %t, calls %#v", device.running, calls)
		}
	})

	t.Run("later failure restores confirmed speed", func(t *testing.T) {
		installLightingDeviceTestSeams(t)
		installLightingSpeedPersistenceSeams(t)
		device := newLightingSpeedMutationDevice("rainbow", 2)

		if err := device.SetEffectSpeed(lightingDeviceTestSerial, "rainbow", 5); err != nil {
			t.Fatalf("first SetEffectSpeed: %v", err)
		}
		stopLightingSpeedWorker(device)
		saveRGBProfileData = func(string, interface{}) error { return errors.New("later RGB save failure") }
		if err := device.SetEffectSpeed(lightingDeviceTestSerial, "rainbow", 7); err == nil {
			t.Fatal("second SetEffectSpeed succeeded despite persistence failure")
		}
		if got := device.GetRgbProfile("rainbow").Speed; got != 5 {
			t.Fatalf("speed after later failed save = %v, want confirmed 5", got)
		}
	})
}

func TestOpenRGBLightingSetEffectSpeedOutputFailureRetainsPersistedSpeed(t *testing.T) {
	_, calls := installLightingDeviceTestSeams(t)
	rgbDirectory := installLightingSpeedPersistenceSeams(t)
	calls.err = errors.New("test speed output failure")
	device := newLightingSpeedMutationDevice("rainbow", 2)

	err := device.SetEffectSpeed(lightingDeviceTestSerial, "rainbow", 5)
	if err == nil || !strings.Contains(err.Error(), "test speed output failure") {
		t.Fatalf("SetEffectSpeed output error = %v", err)
	}
	if got := readLightingRGBProfile(t, rgbDirectory, "rainbow").Speed; got != 5 {
		t.Fatalf("persisted speed after output failure = %v, want desired 5", got)
	}
	if got := device.GetRgbProfile("rainbow").Speed; got != 5 {
		t.Fatalf("in-memory speed after output failure = %v, want desired 5", got)
	}
	if calls.persistentFrames != 1 || device.running || device.ControllerID() != -1 {
		t.Fatalf("output failure state = persistent frames %d, running %t, controller %d", calls.persistentFrames, device.running, device.ControllerID())
	}
}

func TestOpenRGBLightingSetEffectSpeedClusterPreservesState(t *testing.T) {
	profileDirectory, calls := installLightingDeviceTestSeams(t)
	rgbDirectory := installLightingSpeedPersistenceSeams(t)
	device := newLightingSpeedMutationDevice("rainbow", 2)
	device.DeviceProfile.RGBCluster = true
	device.DeviceProfile.RGBOverride = &RGBOverride{Enabled: true, RgbModeSpeed: 3}

	err := device.SetEffectSpeed(lightingDeviceTestSerial, "rainbow", 5)
	if err == nil || err.Error() != "device is controlled by RGB cluster" {
		t.Fatalf("cluster SetEffectSpeed error = %v", err)
	}
	if device.DeviceProfile.RGBOverride.RgbModeSpeed != 3 || device.GetRgbProfile("rainbow").Speed != 2 {
		t.Fatal("cluster rejection changed stored speed state")
	}
	if calls.colors != 0 || calls.frames != 0 || calls.persistentFrames != 0 || device.running {
		t.Fatal("cluster rejection emitted or started local output")
	}
	for _, path := range []string{
		filepath.Join(profileDirectory, lightingDeviceTestSerial+".json"),
		filepath.Join(rgbDirectory, lightingDeviceTestSerial+".json"),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("cluster rejection persisted %s: %v", path, statErr)
		}
	}
}
