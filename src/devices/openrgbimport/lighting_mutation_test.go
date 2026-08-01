package openrgbimport

import (
	"LumenForge/src/openrgb"
	"LumenForge/src/rgb"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

const lightingDeviceTestSerial = "openrgb-lighting-device-test"

type lightingOutputCalls struct {
	colors           int
	frames           int
	colorValues      [][]byte
	frameValues      [][]byte
	persistentFrames int
	persistentOutput chan struct{}
	err              error
	beforeOutput     func()
}

func installLightingDeviceTestSeams(t *testing.T) (string, *lightingOutputCalls) {
	t.Helper()

	previousProfileDir := deviceProfileDir
	previousColor := sendLightingColor
	previousFrame := sendLightingFrame
	previousPersistentFrame := sendLightingPersistentFrame
	profileDir := t.TempDir()
	calls := &lightingOutputCalls{persistentOutput: make(chan struct{}, 1)}
	deviceProfileDir = func() string { return profileDir }
	sendLightingColor = func(_ context.Context, _ uint32, _ int, color []byte) error {
		if calls.beforeOutput != nil {
			calls.beforeOutput()
		}
		calls.colors++
		calls.colorValues = append(calls.colorValues, append([]byte(nil), color...))
		return calls.err
	}
	sendLightingFrame = func(_ context.Context, _ uint32, frame []byte) error {
		if calls.beforeOutput != nil {
			calls.beforeOutput()
		}
		calls.frames++
		calls.frameValues = append(calls.frameValues, append([]byte(nil), frame...))
		return calls.err
	}
	sendLightingPersistentFrame = func(conn net.Conn, _ uint32, _ []byte) (net.Conn, error) {
		calls.persistentFrames++
		select {
		case calls.persistentOutput <- struct{}{}:
		default:
		}
		return conn, calls.err
	}
	t.Cleanup(func() {
		deviceProfileDir = previousProfileDir
		sendLightingColor = previousColor
		sendLightingFrame = previousFrame
		sendLightingPersistentFrame = previousPersistentFrame
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

func newStaticOverrideTestDevice(brightness uint8) *Device {
	device := newLightingMutationDevice()
	device.Config = &DeviceConfig{
		Serial:  lightingDeviceTestSerial,
		Product: "Lighting Test Controller",
		Zones: []ZoneConfig{
			{Name: "Front", LedCount: 2},
			{Name: "Rear", LedCount: 1},
		},
	}
	device.colorCount = 3
	device.ZoneAmount = 2
	device.brightness = brightness
	device.effect = "static"
	device.DeviceProfile.RGBProfile = "static"
	device.DeviceProfile.BrightnessSlider = &brightness
	device.DeviceProfile.ZoneColors = map[int]ZoneColors{
		0: {
			Color:      &rgb.Color{Red: 90, Green: 30, Blue: 10, Brightness: 1},
			ColorIndex: []int{0, 1, 2, 3, 4, 5},
			Name:       "Front",
		},
		1: {
			Color:      &rgb.Color{Red: 5, Green: 60, Blue: 120, Brightness: 1},
			ColorIndex: []int{6, 7, 8},
			Name:       "Rear",
		},
	}
	device.DeviceProfile.RGBOverride = &RGBOverride{
		RGBStartColor:  rgb.Color{Red: 200, Green: 100, Blue: 50, Brightness: 1},
		RGBMiddleColor: rgb.Color{Red: 20, Green: 40, Blue: 60, Brightness: 1},
		RGBEndColor:    rgb.Color{Red: 80, Green: 100, Blue: 120, Brightness: 1},
		RgbModeSpeed:   3,
	}
	device.Rgb = &rgb.RGB{
		Device: "Lighting Test Controller",
		Profiles: map[string]rgb.Profile{
			"static": {
				ProfileName: "static",
				StartColor:  rgb.Color{Red: 12, Green: 34, Blue: 56, Brightness: 1},
			},
		},
	}
	return device
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
	select {
	case <-calls.persistentOutput:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for intercepted persistent animation output")
	}

	device.Stop()
	device.mu.Lock()
	workerActive := device.running || device.stopChan != nil || device.doneChan != nil
	device.mu.Unlock()
	if workerActive {
		t.Fatal("stopped animated effect retained worker state")
	}
	if calls.persistentFrames < 1 {
		t.Fatal("animated worker did not use the intercepted persistent sender")
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

func TestOpenRGBStaticOverrideFrame(t *testing.T) {
	tests := []struct {
		name       string
		brightness uint8
		color      rgb.Color
		expected   []byte
	}{
		{
			name:       "zero brightness",
			brightness: 0,
			color:      rgb.Color{Red: 200, Green: 100, Blue: 50},
			expected:   []byte{0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:       "intermediate brightness",
			brightness: 50,
			color:      rgb.Color{Red: 200, Green: 100, Blue: 50},
			expected:   []byte{100, 50, 25, 100, 50, 25, 100, 50, 25},
		},
		{
			name:       "full brightness",
			brightness: 100,
			color:      rgb.Color{Red: 200, Green: 100, Blue: 50},
			expected:   []byte{200, 100, 50, 200, 100, 50, 200, 100, 50},
		},
		{
			name:       "black override",
			brightness: 100,
			color:      rgb.Color{},
			expected:   []byte{0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:       "integer truncation",
			brightness: 50,
			color:      rgb.Color{Red: 1, Green: 255, Blue: 3},
			expected:   []byte{0, 127, 1, 0, 127, 1, 0, 127, 1},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			device := newStaticOverrideTestDevice(test.brightness)
			device.DeviceProfile.RGBOverride.Enabled = true
			device.DeviceProfile.RGBOverride.RGBStartColor = test.color
			profileDir := t.TempDir()
			device.DeviceProfile.Path = filepath.Join(profileDir, lightingDeviceTestSerial+".json")

			profileBefore := cloneDeviceProfile(device.DeviceProfile)
			configBefore := cloneDeviceConfig(device.Config)
			rgbBefore := cloneRGBState(device.Rgb)
			lastColorBefore := append([]byte(nil), device.lastColor...)

			frame, enabled := device.buildStaticOverrideFrame()
			if !enabled {
				t.Fatal("enabled Static override was not selected")
			}
			if !bytes.Equal(frame, test.expected) {
				t.Fatalf("Static override frame = %v, want %v", frame, test.expected)
			}
			if !reflect.DeepEqual(device.DeviceProfile, profileBefore) {
				t.Fatal("building the Static override frame changed the device profile")
			}
			if !reflect.DeepEqual(device.Config, configBefore) {
				t.Fatal("building the Static override frame changed the controller config")
			}
			if !reflect.DeepEqual(device.Rgb, rgbBefore) {
				t.Fatal("building the Static override frame changed the RGB definition")
			}
			if !bytes.Equal(device.lastColor, lastColorBefore) {
				t.Fatal("building the Static override frame changed lastColor")
			}
			entries, err := os.ReadDir(profileDir)
			if err != nil {
				t.Fatalf("read profile directory: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("building the Static override frame wrote %d profile files", len(entries))
			}
		})
	}

	device := newStaticOverrideTestDevice(100)
	if frame, enabled := device.buildStaticOverrideFrame(); enabled || frame != nil {
		t.Fatalf("disabled Static override returned frame %v, enabled %t", frame, enabled)
	}
}

func TestOpenRGBStaticOverrideBrightnessOutput(t *testing.T) {
	tests := []struct {
		name       string
		brightness uint8
		color      rgb.Color
		expected   []byte
	}{
		{
			name:       "zero brightness",
			brightness: 0,
			color:      rgb.Color{Red: 200, Green: 100, Blue: 50},
			expected:   []byte{0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:       "intermediate brightness",
			brightness: 50,
			color:      rgb.Color{Red: 200, Green: 100, Blue: 50},
			expected:   []byte{100, 50, 25, 100, 50, 25, 100, 50, 25},
		},
		{
			name:       "full brightness",
			brightness: 100,
			color:      rgb.Color{Red: 200, Green: 100, Blue: 50},
			expected:   []byte{200, 100, 50, 200, 100, 50, 200, 100, 50},
		},
		{
			name:       "integer truncation",
			brightness: 50,
			color:      rgb.Color{Red: 1, Green: 255, Blue: 3},
			expected:   []byte{0, 127, 1, 0, 127, 1, 0, 127, 1},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profileDir, calls := installLightingDeviceTestSeams(t)
			device := newStaticOverrideTestDevice(25)
			device.DeviceProfile.RGBOverride.Enabled = true
			device.DeviceProfile.RGBOverride.RGBStartColor = test.color

			zoneColorsBefore := cloneDeviceProfile(device.DeviceProfile).ZoneColors
			overrideBefore := *device.DeviceProfile.RGBOverride
			configBefore := cloneDeviceConfig(device.Config)
			rgbBefore := cloneRGBState(device.Rgb)
			lastColorBefore := append([]byte(nil), device.lastColor...)

			if err := device.SetBrightness(test.brightness); err != nil {
				t.Fatalf("SetBrightness(%d): %v", test.brightness, err)
			}
			if calls.colors != 0 || calls.frames != 1 {
				t.Fatalf("output calls = colors %d, frames %d, want 0, 1", calls.colors, calls.frames)
			}
			if len(calls.frameValues) != 1 || !bytes.Equal(calls.frameValues[0], test.expected) {
				t.Fatalf("Static override brightness frame = %v, want %v", calls.frameValues, test.expected)
			}
			profile := readLightingDeviceProfile(t, profileDir)
			if profile.BrightnessSlider == nil || *profile.BrightnessSlider != test.brightness {
				t.Fatalf("persisted brightness = %#v, want %d", profile.BrightnessSlider, test.brightness)
			}
			if !reflect.DeepEqual(device.DeviceProfile.ZoneColors, zoneColorsBefore) {
				t.Fatal("Static override brightness change modified stored zone colors")
			}
			if device.DeviceProfile.RGBOverride == nil || !reflect.DeepEqual(*device.DeviceProfile.RGBOverride, overrideBefore) {
				t.Fatal("Static override brightness change modified the override")
			}
			if !reflect.DeepEqual(device.Config, configBefore) {
				t.Fatal("Static override brightness change modified configured zones")
			}
			if !reflect.DeepEqual(device.Rgb, rgbBefore) {
				t.Fatal("Static override brightness change modified the RGB definition")
			}
			if !bytes.Equal(device.lastColor, lastColorBefore) {
				t.Fatal("Static override brightness change modified lastColor")
			}
		})
	}

	t.Run("disabled override uses stored zones", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newStaticOverrideTestDevice(100)
		zoneColorsBefore := cloneDeviceProfile(device.DeviceProfile).ZoneColors
		overrideBefore := *device.DeviceProfile.RGBOverride
		configBefore := cloneDeviceConfig(device.Config)
		rgbBefore := cloneRGBState(device.Rgb)
		lastColorBefore := append([]byte(nil), device.lastColor...)

		if err := device.SetBrightness(50); err != nil {
			t.Fatalf("SetBrightness: %v", err)
		}
		expected := []byte{45, 15, 5, 45, 15, 5, 2, 30, 60}
		if calls.colors != 0 || calls.frames != 1 || !bytes.Equal(calls.frameValues[0], expected) {
			t.Fatalf("disabled override output = colors %d, frames %v, want zone frame %v", calls.colors, calls.frameValues, expected)
		}
		if !reflect.DeepEqual(device.DeviceProfile.ZoneColors, zoneColorsBefore) {
			t.Fatal("disabled override brightness change modified stored zone colors")
		}
		if device.DeviceProfile.RGBOverride == nil || !reflect.DeepEqual(*device.DeviceProfile.RGBOverride, overrideBefore) {
			t.Fatal("disabled override brightness change modified the override")
		}
		if !reflect.DeepEqual(device.Config, configBefore) || !reflect.DeepEqual(device.Rgb, rgbBefore) || !bytes.Equal(device.lastColor, lastColorBefore) {
			t.Fatal("disabled override brightness change modified base device state")
		}
	})

	t.Run("animated effect sends no synchronous Static frame", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newStaticOverrideTestDevice(100)
		device.DeviceProfile.RGBOverride.Enabled = true
		zoneColorsBefore := cloneDeviceProfile(device.DeviceProfile).ZoneColors
		overrideBefore := *device.DeviceProfile.RGBOverride
		configBefore := cloneDeviceConfig(device.Config)
		rgbBefore := cloneRGBState(device.Rgb)
		lastColorBefore := append([]byte(nil), device.lastColor...)

		if err := device.SetEffect("rainbow"); err != nil {
			t.Fatalf("SetEffect(rainbow): %v", err)
		}
		if err := device.SetBrightness(50); err != nil {
			device.Stop()
			t.Fatalf("SetBrightness: %v", err)
		}
		device.Stop()
		if calls.colors != 0 || calls.frames != 0 {
			t.Fatalf("animated brightness output calls = colors %d, frames %d, want none", calls.colors, calls.frames)
		}
		if !reflect.DeepEqual(device.DeviceProfile.ZoneColors, zoneColorsBefore) || device.DeviceProfile.RGBOverride == nil || !reflect.DeepEqual(*device.DeviceProfile.RGBOverride, overrideBefore) {
			t.Fatal("animated brightness change modified stored colour state")
		}
		if !reflect.DeepEqual(device.Config, configBefore) || !reflect.DeepEqual(device.Rgb, rgbBefore) || !bytes.Equal(device.lastColor, lastColorBefore) {
			t.Fatal("animated brightness change modified base device state")
		}
	})

	t.Run("Off does not select Static override", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newStaticOverrideTestDevice(100)
		device.effect = "off"
		device.DeviceProfile.RGBProfile = "off"
		device.DeviceProfile.RGBOverride.Enabled = true
		zoneColorsBefore := cloneDeviceProfile(device.DeviceProfile).ZoneColors
		overrideBefore := *device.DeviceProfile.RGBOverride
		configBefore := cloneDeviceConfig(device.Config)
		rgbBefore := cloneRGBState(device.Rgb)
		lastColorBefore := append([]byte(nil), device.lastColor...)

		if err := device.SetBrightness(50); err != nil {
			t.Fatalf("SetBrightness: %v", err)
		}
		expectedBase := []byte{45, 15, 5, 45, 15, 5, 2, 30, 60}
		if calls.colors != 0 || calls.frames != 1 || !bytes.Equal(calls.frameValues[0], expectedBase) {
			t.Fatalf("Off brightness output = colors %d, frames %v, want existing base frame %v", calls.colors, calls.frameValues, expectedBase)
		}
		if device.GetEffect() != "off" {
			t.Fatalf("effect = %q, want off", device.GetEffect())
		}
		if !reflect.DeepEqual(device.DeviceProfile.ZoneColors, zoneColorsBefore) || device.DeviceProfile.RGBOverride == nil || !reflect.DeepEqual(*device.DeviceProfile.RGBOverride, overrideBefore) {
			t.Fatal("Off brightness change modified stored colour state")
		}
		if !reflect.DeepEqual(device.Config, configBefore) || !reflect.DeepEqual(device.Rgb, rgbBefore) || !bytes.Equal(device.lastColor, lastColorBefore) {
			t.Fatal("Off brightness change modified base device state")
		}
	})

	t.Run("output failure retains persisted brightness", func(t *testing.T) {
		preserveOpenRGBStatus(t)
		profileDir, calls := installLightingDeviceTestSeams(t)
		calls.err = errors.New("test Static override brightness output failure")
		device := newStaticOverrideTestDevice(100)
		device.DeviceProfile.RGBOverride.Enabled = true
		zoneColorsBefore := cloneDeviceProfile(device.DeviceProfile).ZoneColors
		overrideBefore := *device.DeviceProfile.RGBOverride
		configBefore := cloneDeviceConfig(device.Config)
		rgbBefore := cloneRGBState(device.Rgb)
		lastColorBefore := append([]byte(nil), device.lastColor...)

		if err := device.SetBrightness(50); err == nil {
			t.Fatal("Static override brightness output failure was not returned")
		}
		if calls.colors != 0 || calls.frames != 1 {
			t.Fatalf("output calls = colors %d, frames %d, want 0, 1", calls.colors, calls.frames)
		}
		profile := readLightingDeviceProfile(t, profileDir)
		if profile.BrightnessSlider == nil || *profile.BrightnessSlider != 50 {
			t.Fatalf("persisted brightness = %#v, want 50", profile.BrightnessSlider)
		}
		if device.ControllerID() != -1 {
			t.Fatalf("controller ID = %d, want unavailable -1", device.ControllerID())
		}
		if !reflect.DeepEqual(device.DeviceProfile.ZoneColors, zoneColorsBefore) {
			t.Fatal("failed Static override brightness output modified stored zone colors")
		}
		if device.DeviceProfile.RGBOverride == nil || !reflect.DeepEqual(*device.DeviceProfile.RGBOverride, overrideBefore) {
			t.Fatal("failed Static override brightness output modified the override")
		}
		if !reflect.DeepEqual(device.Config, configBefore) || !reflect.DeepEqual(device.Rgb, rgbBefore) || !bytes.Equal(device.lastColor, lastColorBefore) {
			t.Fatal("failed Static override brightness output modified base device state")
		}
	})
}

func TestOpenRGBStaticOverrideBlackProductionOutput(t *testing.T) {
	profileDir, calls := installLightingDeviceTestSeams(t)
	device := newStaticOverrideTestDevice(100)
	device.DeviceProfile.RGBOverride.Enabled = true
	device.DeviceProfile.RGBOverride.RGBStartColor = rgb.Color{}
	if err := device.saveDeviceProfileChecked(); err != nil {
		t.Fatalf("save baseline profile: %v", err)
	}
	profilePath := filepath.Join(profileDir, lightingDeviceTestSerial+".json")
	profileBefore, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read baseline profile: %v", err)
	}
	zoneColorsBefore := cloneDeviceProfile(device.DeviceProfile).ZoneColors

	if err = device.resumeDesiredState(context.Background()); err != nil {
		t.Fatalf("resume black Static override: %v", err)
	}
	if calls.colors != 0 || calls.frames != 1 {
		t.Fatalf("black Static output calls = colors %d, frames %d, want 0, 1", calls.colors, calls.frames)
	}
	if len(calls.frameValues) != 1 || len(calls.frameValues[0]) != device.colorCount*3 {
		t.Fatalf("black Static frame = %v, want %d zero bytes", calls.frameValues, device.colorCount*3)
	}
	for index, value := range calls.frameValues[0] {
		if value != 0 {
			t.Fatalf("black Static frame byte %d = %d, want 0", index, value)
		}
	}
	baseFrame := []byte{90, 30, 10, 90, 30, 10, 5, 60, 120}
	if bytes.Equal(calls.frameValues[0], baseFrame) {
		t.Fatal("black Static output selected the stored base-zone frame")
	}
	profileAfter, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read profile after black output: %v", err)
	}
	if !bytes.Equal(profileAfter, profileBefore) {
		t.Fatal("black Static replay changed persisted profile content")
	}
	if !reflect.DeepEqual(device.DeviceProfile.ZoneColors, zoneColorsBefore) {
		t.Fatal("black Static output modified stored zone colors")
	}
}

func TestOpenRGBStaticOverrideOutputSelection(t *testing.T) {
	_, calls := installLightingDeviceTestSeams(t)
	device := newStaticOverrideTestDevice(100)
	baseFrame := []byte{90, 30, 10, 90, 30, 10, 5, 60, 120}
	overrideFrame := []byte{200, 100, 50, 200, 100, 50, 200, 100, 50}

	zoneColorsBefore := cloneDeviceProfile(device.DeviceProfile).ZoneColors
	configBefore := cloneDeviceConfig(device.Config)
	rgbBefore := cloneRGBState(device.Rgb)
	lastColorBefore := append([]byte(nil), device.lastColor...)

	if err := device.resumeDesiredState(context.Background()); err != nil {
		t.Fatalf("resume disabled Static override: %v", err)
	}
	device.DeviceProfile.RGBOverride.Enabled = true
	if err := device.resumeDesiredState(context.Background()); err != nil {
		t.Fatalf("resume enabled Static override: %v", err)
	}
	device.DeviceProfile.RGBOverride.Enabled = false
	if err := device.resumeDesiredState(context.Background()); err != nil {
		t.Fatalf("resume restored Static base: %v", err)
	}

	if calls.colors != 0 || calls.frames != 3 {
		t.Fatalf("output calls = colors %d, frames %d, want 0, 3", calls.colors, calls.frames)
	}
	wantedFrames := [][]byte{baseFrame, overrideFrame, baseFrame}
	if !reflect.DeepEqual(calls.frameValues, wantedFrames) {
		t.Fatalf("Static output frames = %v, want %v", calls.frameValues, wantedFrames)
	}
	if !reflect.DeepEqual(device.DeviceProfile.ZoneColors, zoneColorsBefore) {
		t.Fatal("Static override output changed stored zone colors")
	}
	if !reflect.DeepEqual(device.Config, configBefore) {
		t.Fatal("Static override output changed configured zones")
	}
	if !reflect.DeepEqual(device.Rgb, rgbBefore) {
		t.Fatal("Static override output changed the RGB definition")
	}
	if !bytes.Equal(device.lastColor, lastColorBefore) {
		t.Fatal("Static override output changed lastColor")
	}
}

func TestOpenRGBStaticOverrideTransitionsAndResume(t *testing.T) {
	_, calls := installLightingDeviceTestSeams(t)
	device := newStaticOverrideTestDevice(50)
	device.DeviceProfile.RGBOverride.Enabled = true
	expected := []byte{100, 50, 25, 100, 50, 25, 100, 50, 25}

	if err := device.resumeDesiredState(context.Background()); err != nil {
		t.Fatalf("resume Static override: %v", err)
	}
	if err := device.SetEffect("rainbow"); err != nil {
		t.Fatalf("Static to animated: %v", err)
	}
	if err := device.SetEffect("static"); err != nil {
		t.Fatalf("animated to Static: %v", err)
	}
	if err := device.SetEffect("rainbow"); err != nil {
		t.Fatalf("second Static to animated: %v", err)
	}
	if err := device.SetEffect("static"); err != nil {
		t.Fatalf("second animated to Static: %v", err)
	}

	if calls.colors != 0 || calls.frames != 3 {
		t.Fatalf("output calls = colors %d, frames %d, want 0, 3", calls.colors, calls.frames)
	}
	for index, frame := range calls.frameValues {
		if !bytes.Equal(frame, expected) {
			t.Fatalf("Static frame %d = %v, want %v", index, frame, expected)
		}
	}
	device.mu.Lock()
	workerActive := device.running || device.stopChan != nil || device.doneChan != nil
	device.mu.Unlock()
	if workerActive {
		t.Fatal("final Static transition left an animation worker active")
	}
}

func TestOpenRGBStaticOverrideClusterBoundary(t *testing.T) {
	profileDir, calls := installLightingDeviceTestSeams(t)
	device := newStaticOverrideTestDevice(100)
	device.DeviceProfile.RGBOverride.Enabled = true
	device.DeviceProfile.RGBCluster = true

	if frame, enabled := device.buildStaticOverrideFrame(); !enabled || len(frame) != device.colorCount*3 {
		t.Fatalf("Static override helper = frame length %d, enabled %t", len(frame), enabled)
	}
	if err := device.resumeDesiredState(context.Background()); err != nil {
		t.Fatalf("clustered resume: %v", err)
	}
	if err := device.SetEffect("static"); err == nil {
		t.Fatal("clustered SetEffect(static) unexpectedly succeeded")
	}
	if calls.colors != 0 || calls.frames != 0 {
		t.Fatalf("clustered output calls = colors %d, frames %d, want none", calls.colors, calls.frames)
	}
	entries, err := os.ReadDir(profileDir)
	if err != nil {
		t.Fatalf("read profile directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("cluster rejection wrote %d profile files", len(entries))
	}
}

func TestOpenRGBStaticOverrideOutputFailure(t *testing.T) {
	preserveOpenRGBStatus(t)
	profileDir, calls := installLightingDeviceTestSeams(t)
	calls.err = errors.New("test Static override output failure")
	device := newStaticOverrideTestDevice(100)
	device.DeviceProfile.RGBOverride.Enabled = true

	zoneColorsBefore := cloneDeviceProfile(device.DeviceProfile).ZoneColors
	configBefore := cloneDeviceConfig(device.Config)
	rgbBefore := cloneRGBState(device.Rgb)
	lastColorBefore := append([]byte(nil), device.lastColor...)

	if err := device.SetEffect("static"); err == nil {
		t.Fatal("Static override output failure was not returned")
	}
	if calls.colors != 0 || calls.frames != 1 {
		t.Fatalf("output calls = colors %d, frames %d, want 0, 1", calls.colors, calls.frames)
	}
	if device.ControllerID() != -1 {
		t.Fatalf("controller ID = %d, want unavailable -1", device.ControllerID())
	}
	profile := readLightingDeviceProfile(t, profileDir)
	if profile.RGBProfile != "static" {
		t.Fatalf("persisted effect = %q, want static", profile.RGBProfile)
	}
	if !reflect.DeepEqual(device.DeviceProfile.ZoneColors, zoneColorsBefore) {
		t.Fatal("failed Static override output changed stored zone colors")
	}
	if !reflect.DeepEqual(device.Config, configBefore) {
		t.Fatal("failed Static override output changed configured zones")
	}
	if !reflect.DeepEqual(device.Rgb, rgbBefore) {
		t.Fatal("failed Static override output changed the RGB definition")
	}
	if !bytes.Equal(device.lastColor, lastColorBefore) {
		t.Fatal("failed Static override output changed lastColor")
	}
}
