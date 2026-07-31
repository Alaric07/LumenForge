package openrgbimport

import (
	"LumenForge/src/openrgb"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const lightingDeviceTestSerial = "openrgb-lighting-device-test"

type lightingOutputCalls struct {
	colors       int
	frames       int
	colorValues  [][]byte
	err          error
	beforeOutput func()
}

func installLightingDeviceTestSeams(t *testing.T) (string, *lightingOutputCalls) {
	t.Helper()

	previousProfileDir := deviceProfileDir
	previousColor := sendLightingColor
	previousFrame := sendLightingFrame
	profileDir := t.TempDir()
	calls := &lightingOutputCalls{}
	deviceProfileDir = func() string { return profileDir }
	sendLightingColor = func(_ context.Context, _ uint32, _ int, color []byte) error {
		if calls.beforeOutput != nil {
			calls.beforeOutput()
		}
		calls.colors++
		calls.colorValues = append(calls.colorValues, append([]byte(nil), color...))
		return calls.err
	}
	sendLightingFrame = func(context.Context, uint32, []byte) error {
		if calls.beforeOutput != nil {
			calls.beforeOutput()
		}
		calls.frames++
		return calls.err
	}
	t.Cleanup(func() {
		deviceProfileDir = previousProfileDir
		sendLightingColor = previousColor
		sendLightingFrame = previousFrame
	})
	return profileDir, calls
}

func newLightingMutationDevice() *Device {
	brightness := uint8(40)
	return &Device{
		Product:      "Lighting Test Controller",
		Serial:       lightingDeviceTestSerial,
		IsOpenRGB:    true,
		controllerId: 7,
		colorCount:   1,
		brightness:   brightness,
		lastColor:    []byte{100, 150, 200},
		effect:       "off",
		speed:        2,
		RGBModes:     []string{"off", "static", "rainbow"},
		DeviceProfile: &DeviceProfile{
			Active:           true,
			RGBProfile:       "off",
			BrightnessSlider: &brightness,
			ZoneColors:       map[int]ZoneColors{},
		},
	}
}

func loadLightingDeviceProfile(profileDir string) (DeviceProfile, error) {
	data, err := os.ReadFile(filepath.Join(profileDir, lightingDeviceTestSerial+".json"))
	if err != nil {
		return DeviceProfile{}, err
	}
	var profile DeviceProfile
	if err = json.Unmarshal(data, &profile); err != nil {
		return DeviceProfile{}, err
	}
	return profile, nil
}

func readLightingDeviceProfile(t *testing.T, profileDir string) DeviceProfile {
	t.Helper()
	profile, err := loadLightingDeviceProfile(profileDir)
	if err != nil {
		t.Fatalf("load persisted profile: %v", err)
	}
	return profile
}

func waitLightingEffectResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for effect transition")
		return nil
	}
}

func preserveOpenRGBStatus(t *testing.T) {
	t.Helper()
	state, statusErr := openrgb.GetStatus()
	t.Cleanup(func() {
		switch state {
		case openrgb.StateConnected:
			openrgb.SetConnected()
		case openrgb.StateNotConfigured:
			openrgb.SetNotConfigured()
		default:
			openrgb.SetDisconnected(statusErr)
		}
	})
}

func TestOpenRGBLightingBrightnessPersistenceAndOutputOrdering(t *testing.T) {
	t.Run("persistence success precedes output", func(t *testing.T) {
		profileDir, calls := installLightingDeviceTestSeams(t)
		device := newLightingMutationDevice()
		var observationMutex sync.Mutex
		var observedPersisted bool
		var observationErr error
		calls.beforeOutput = func() {
			profile, err := loadLightingDeviceProfile(profileDir)
			observationMutex.Lock()
			defer observationMutex.Unlock()
			observationErr = err
			observedPersisted = err == nil && profile.BrightnessSlider != nil && *profile.BrightnessSlider == 65
		}

		if err := device.SetBrightness(65); err != nil {
			t.Fatalf("SetBrightness: %v", err)
		}
		if calls.colors != 1 || calls.frames != 0 {
			t.Fatalf("output calls = colors %d, frames %d, want 1, 0", calls.colors, calls.frames)
		}
		if got := device.GetBrightness(); got != 65 {
			t.Fatalf("brightness = %d, want 65", got)
		}
		observationMutex.Lock()
		observedErr := observationErr
		persistedBeforeOutput := observedPersisted
		observationMutex.Unlock()
		if observedErr != nil {
			t.Fatalf("load profile during output: %v", observedErr)
		}
		if !persistedBeforeOutput {
			t.Fatal("output callback did not observe persisted brightness 65")
		}
		profile := readLightingDeviceProfile(t, profileDir)
		if profile.BrightnessSlider == nil || *profile.BrightnessSlider != 65 {
			t.Fatalf("persisted brightness = %#v, want 65", profile.BrightnessSlider)
		}
	})

	t.Run("persistence failure restores state and skips output", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		blockedPath := filepath.Join(t.TempDir(), "profiles")
		if err := os.WriteFile(blockedPath, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		deviceProfileDir = func() string { return blockedPath }
		device := newLightingMutationDevice()

		if err := device.SetBrightness(65); err == nil {
			t.Fatal("SetBrightness succeeded despite persistence failure")
		}
		if calls.colors != 0 || calls.frames != 0 {
			t.Fatalf("output calls = colors %d, frames %d, want none", calls.colors, calls.frames)
		}
		if got := device.GetBrightness(); got != 40 {
			t.Fatalf("brightness = %d, want restored value 40", got)
		}
		if device.DeviceProfile.BrightnessSlider == nil || *device.DeviceProfile.BrightnessSlider != 40 {
			t.Fatalf("profile brightness = %#v, want restored value 40", device.DeviceProfile.BrightnessSlider)
		}
	})

	t.Run("synchronous output failure retains persisted value", func(t *testing.T) {
		preserveOpenRGBStatus(t)
		profileDir, calls := installLightingDeviceTestSeams(t)
		calls.err = errors.New("test output failure")
		device := newLightingMutationDevice()

		if err := device.SetBrightness(65); err == nil {
			t.Fatal("SetBrightness succeeded despite output failure")
		}
		if calls.colors != 1 || calls.frames != 0 {
			t.Fatalf("output calls = colors %d, frames %d, want 1, 0", calls.colors, calls.frames)
		}
		if got := device.GetBrightness(); got != 65 {
			t.Fatalf("brightness = %d, want persisted desired value 65", got)
		}
		profile := readLightingDeviceProfile(t, profileDir)
		if profile.BrightnessSlider == nil || *profile.BrightnessSlider != 65 {
			t.Fatalf("persisted brightness = %#v, want 65", profile.BrightnessSlider)
		}
		if device.ControllerID() != -1 {
			t.Fatalf("controller ID = %d, want unavailable -1", device.ControllerID())
		}
	})

	t.Run("out of range is rejected before persistence or output", func(t *testing.T) {
		profileDir, calls := installLightingDeviceTestSeams(t)
		device := newLightingMutationDevice()

		if err := device.SetBrightness(101); err == nil {
			t.Fatal("SetBrightness accepted 101")
		}
		if calls.colors != 0 || calls.frames != 0 {
			t.Fatalf("output calls = colors %d, frames %d, want none", calls.colors, calls.frames)
		}
		if _, err := os.Stat(filepath.Join(profileDir, lightingDeviceTestSerial+".json")); !os.IsNotExist(err) {
			t.Fatalf("unexpected persisted profile after rejected brightness: %v", err)
		}
	})
}

func TestOpenRGBLightingEffectPersistenceAndOutputOrdering(t *testing.T) {
	t.Run("supported effect persists before output", func(t *testing.T) {
		profileDir, calls := installLightingDeviceTestSeams(t)
		device := newLightingMutationDevice()
		var observationMutex sync.Mutex
		var observedPersisted bool
		var observationErr error
		calls.beforeOutput = func() {
			profile, err := loadLightingDeviceProfile(profileDir)
			observationMutex.Lock()
			defer observationMutex.Unlock()
			observationErr = err
			observedPersisted = err == nil && profile.RGBProfile == "static"
		}

		if err := device.SetEffect("static"); err != nil {
			t.Fatalf("SetEffect: %v", err)
		}
		if calls.colors != 1 || calls.frames != 0 {
			t.Fatalf("output calls = colors %d, frames %d, want 1, 0", calls.colors, calls.frames)
		}
		if got := device.GetEffect(); got != "static" {
			t.Fatalf("effect = %q, want static", got)
		}
		observationMutex.Lock()
		observedErr := observationErr
		persistedBeforeOutput := observedPersisted
		observationMutex.Unlock()
		if observedErr != nil {
			t.Fatalf("load profile during output: %v", observedErr)
		}
		if !persistedBeforeOutput {
			t.Fatal("output callback did not observe persisted effect static")
		}
		profile := readLightingDeviceProfile(t, profileDir)
		if profile.RGBProfile != "static" {
			t.Fatalf("persisted effect = %q, want static", profile.RGBProfile)
		}
	})

	t.Run("persistence failure restores state and leaves lifecycle untouched", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		blockedPath := filepath.Join(t.TempDir(), "profiles")
		if err := os.WriteFile(blockedPath, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		deviceProfileDir = func() string { return blockedPath }
		device := newLightingMutationDevice()
		stop := make(chan struct{})
		device.running = true
		device.stopChan = stop

		if err := device.SetEffect("static"); err == nil {
			t.Fatal("SetEffect succeeded despite persistence failure")
		}
		if calls.colors != 0 || calls.frames != 0 {
			t.Fatalf("output calls = colors %d, frames %d, want none", calls.colors, calls.frames)
		}
		if got := device.GetEffect(); got != "off" || device.DeviceProfile.RGBProfile != "off" {
			t.Fatalf("effect state = %q/%q, want restored off/off", got, device.DeviceProfile.RGBProfile)
		}
		if !device.running || device.stopChan != stop {
			t.Fatal("previous effect lifecycle was replaced after persistence failure")
		}
		select {
		case <-stop:
			t.Fatal("previous effect stop channel was closed after persistence failure")
		default:
		}
	})

	t.Run("unsupported effect performs no persistence or output", func(t *testing.T) {
		profileDir, calls := installLightingDeviceTestSeams(t)
		device := newLightingMutationDevice()

		if err := device.SetEffect("STATIC"); err == nil {
			t.Fatal("SetEffect accepted unsupported effect")
		}
		if calls.colors != 0 || calls.frames != 0 {
			t.Fatalf("output calls = colors %d, frames %d, want none", calls.colors, calls.frames)
		}
		if got := device.GetEffect(); got != "off" || device.DeviceProfile.RGBProfile != "off" {
			t.Fatalf("effect state = %q/%q, want off/off", got, device.DeviceProfile.RGBProfile)
		}
		if _, err := os.Stat(filepath.Join(profileDir, lightingDeviceTestSerial+".json")); !os.IsNotExist(err) {
			t.Fatalf("unexpected persisted profile after rejected effect: %v", err)
		}
	})

	t.Run("synchronous output failure retains persisted effect", func(t *testing.T) {
		preserveOpenRGBStatus(t)
		profileDir, calls := installLightingDeviceTestSeams(t)
		calls.err = errors.New("test output failure")
		device := newLightingMutationDevice()

		if err := device.SetEffect("static"); err == nil {
			t.Fatal("SetEffect succeeded despite output failure")
		}
		if calls.colors != 1 || calls.frames != 0 {
			t.Fatalf("output calls = colors %d, frames %d, want 1, 0", calls.colors, calls.frames)
		}
		if got := device.GetEffect(); got != "static" {
			t.Fatalf("effect = %q, want persisted desired effect static", got)
		}
		profile := readLightingDeviceProfile(t, profileDir)
		if profile.RGBProfile != "static" {
			t.Fatalf("persisted effect = %q, want static", profile.RGBProfile)
		}
		if device.ControllerID() != -1 {
			t.Fatalf("controller ID = %d, want unavailable -1", device.ControllerID())
		}
	})
}

func TestOpenRGBLightingEffectTransitionSerialization(t *testing.T) {
	profileDir, calls := installLightingDeviceTestSeams(t)
	device := newLightingMutationDevice()
	device.effect = "rainbow"
	device.DeviceProfile.RGBProfile = "rainbow"
	oldStop := make(chan struct{})
	oldDone := make(chan struct{})
	device.running = true
	device.stopChan = oldStop
	device.doneChan = oldDone

	firstResult := make(chan error, 1)
	go func() {
		firstResult <- device.SetEffect("off")
	}()

	select {
	case <-oldStop:
	case <-time.After(2 * time.Second):
		t.Fatal("first transition did not enter the stop/wait window")
	}
	if device.effectTransitionMu.TryLock() {
		device.effectTransitionMu.Unlock()
		t.Fatal("effect transition mutex was not held during the stop/wait window")
	}

	secondStarted := make(chan struct{})
	secondResult := make(chan error, 1)
	go func() {
		close(secondStarted)
		secondResult <- device.SetEffect("static")
	}()
	<-secondStarted
	select {
	case err := <-secondResult:
		t.Fatalf("second transition passed serialization boundary early: %v", err)
	default:
	}

	profile := readLightingDeviceProfile(t, profileDir)
	if profile.RGBProfile != "off" {
		t.Fatalf("persisted effect while first transition waited = %q, want off", profile.RGBProfile)
	}
	close(oldDone)

	if err := waitLightingEffectResult(t, firstResult); err != nil {
		t.Fatalf("first SetEffect: %v", err)
	}
	if err := waitLightingEffectResult(t, secondResult); err != nil {
		t.Fatalf("second SetEffect: %v", err)
	}

	device.mu.Lock()
	finalEffect := device.effect
	finalProfileEffect := device.DeviceProfile.RGBProfile
	workerActive := device.running || device.stopChan != nil || device.doneChan != nil
	device.mu.Unlock()
	if finalEffect != "static" || finalProfileEffect != "static" {
		t.Fatalf("final effect state = %q/%q, want static/static", finalEffect, finalProfileEffect)
	}
	if workerActive {
		t.Fatal("static transition left an animation worker or worker channels active")
	}

	if calls.colors != 2 || calls.frames != 0 {
		t.Fatalf("output calls = colors %d, frames %d, want 2, 0", calls.colors, calls.frames)
	}
	if len(calls.colorValues) != 2 || !bytes.Equal(calls.colorValues[0], []byte{0, 0, 0}) || !bytes.Equal(calls.colorValues[1], []byte{40, 60, 80}) {
		t.Fatalf("output colors = %v, want off then scaled static", calls.colorValues)
	}
}

func TestOpenRGBLightingAnimatedEffectStartsOneWorker(t *testing.T) {
	_, calls := installLightingDeviceTestSeams(t)
	device := newLightingMutationDevice()

	if err := device.SetEffect("rainbow"); err != nil {
		t.Fatalf("SetEffect: %v", err)
	}
	device.mu.Lock()
	stop := device.stopChan
	done := device.doneChan
	running := device.running
	device.mu.Unlock()
	if !running || stop == nil || done == nil {
		t.Fatalf("animated worker state = running %t, stop %v, done %v", running, stop != nil, done != nil)
	}
	if calls.colors != 0 || calls.frames != 0 {
		t.Fatalf("synchronous output calls = colors %d, frames %d, want none", calls.colors, calls.frames)
	}

	device.Stop()
	device.mu.Lock()
	workerActive := device.running || device.stopChan != nil || device.doneChan != nil
	device.mu.Unlock()
	if workerActive {
		t.Fatal("stopped animated effect retained worker state")
	}
}

func TestOpenRGBLightingEffectRevalidatesAfterStop(t *testing.T) {
	profileDir, calls := installLightingDeviceTestSeams(t)
	device := newLightingMutationDevice()
	device.effect = "rainbow"
	device.DeviceProfile.RGBProfile = "rainbow"
	oldStop := make(chan struct{})
	oldDone := make(chan struct{})
	device.running = true
	device.stopChan = oldStop
	device.doneChan = oldDone

	result := make(chan error, 1)
	go func() {
		result <- device.SetEffect("static")
	}()

	select {
	case <-oldStop:
	case <-time.After(2 * time.Second):
		t.Fatal("transition did not enter the stop/wait window")
	}
	device.mu.Lock()
	device.controllerId = -1
	device.mu.Unlock()
	close(oldDone)

	if err := waitLightingEffectResult(t, result); err == nil {
		t.Fatal("SetEffect succeeded after the controller became unavailable")
	}
	if calls.colors != 0 || calls.frames != 0 {
		t.Fatalf("output calls = colors %d, frames %d, want none", calls.colors, calls.frames)
	}
	profile := readLightingDeviceProfile(t, profileDir)
	if profile.RGBProfile != "static" {
		t.Fatalf("persisted desired effect = %q, want static", profile.RGBProfile)
	}

	device.mu.Lock()
	desiredEffect := device.effect
	desiredProfileEffect := device.DeviceProfile.RGBProfile
	workerActive := device.running || device.stopChan != nil || device.doneChan != nil
	device.mu.Unlock()
	if desiredEffect != "static" || desiredProfileEffect != "static" {
		t.Fatalf("desired effect state = %q/%q, want static/static", desiredEffect, desiredProfileEffect)
	}
	if workerActive {
		t.Fatal("failed transition left an animation worker or worker channels active")
	}
}
