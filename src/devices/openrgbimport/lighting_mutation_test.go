package openrgbimport

import (
	"LumenForge/src/config"
	"LumenForge/src/openrgb"
	"LumenForge/src/rgb"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"os"
	"path/filepath"
	"reflect"
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

type lightingTemperatureCalls struct {
	cpu           int
	nvidia        int
	amd           int
	nvidiaIndexes []int
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
		if calls.beforeOutput != nil {
			calls.beforeOutput()
		}
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

func installLightingTemperatureTestSeams(
	t *testing.T,
	cpuTemperature float32,
	nvidiaTemperature float32,
	amdTemperature float32,
) *lightingTemperatureCalls {
	t.Helper()

	previousCPU := getLightingCPUTemperature
	previousNVIDIA := getLightingNVIDIATemperature
	previousAMD := getLightingAMDTemperature
	calls := &lightingTemperatureCalls{}
	getLightingCPUTemperature = func() float32 {
		calls.cpu++
		return cpuTemperature
	}
	getLightingNVIDIATemperature = func(index int) float32 {
		calls.nvidia++
		calls.nvidiaIndexes = append(calls.nvidiaIndexes, index)
		return nvidiaTemperature
	}
	getLightingAMDTemperature = func() float32 {
		calls.amd++
		return amdTemperature
	}
	t.Cleanup(func() {
		getLightingCPUTemperature = previousCPU
		getLightingNVIDIATemperature = previousNVIDIA
		getLightingAMDTemperature = previousAMD
	})
	return calls
}

func newLightingMutationDevice() *Device {
	brightness := uint8(40)
	device := &Device{
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
	attachTestLightingRuntime(device)
	device.effect = "off"
	device.brightness = brightness
	return device
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
		_, calls := installLightingDeviceTestSeams(t)
		device := newLightingMutationDevice()
		var observedPersisted bool
		calls.beforeOutput = func() {
			state, found, err := device.lightingState.Resolve(device.Serial)
			observedPersisted = err == nil && found && state.Brightness == 65 && state.SelectedEffect == "off"
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
		if !observedPersisted {
			t.Fatal("output callback did not observe persisted brightness 65")
		}
	})

	t.Run("persistence failure restores state and skips output", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newLightingMutationDevice()
		confirmed := device.lightingState
		device.lightingState = failingLightingStateAccess{deviceLightingStateAccess: confirmed, err: errors.New("injected state failure")}

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
		if _, found, _ := confirmed.Resolve(device.Serial); found {
			t.Fatal("failed brightness mutation created target state")
		}
	})

	t.Run("synchronous output failure retains persisted value", func(t *testing.T) {
		preserveOpenRGBStatus(t)
		_, calls := installLightingDeviceTestSeams(t)
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
		state, found, stateErr := device.lightingState.Resolve(device.Serial)
		if stateErr != nil || !found || state.Brightness != 65 {
			t.Fatalf("persisted brightness state = %#v, %t, %v", state, found, stateErr)
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

	t.Run("RGB Cluster ownership rejects local mutation", func(t *testing.T) {
		profileDir, calls := installLightingDeviceTestSeams(t)
		device := newLightingMutationDevice()
		device.DeviceProfile.RGBCluster = true
		brightnessPointer := device.DeviceProfile.BrightnessSlider
		controllerBefore := device.ControllerID()
		detachedBefore := device.lifecycleDetached
		activatingBefore := device.lifecycleActivating

		if err := device.SetBrightness(65); err == nil {
			t.Fatal("cluster-owned SetBrightness unexpectedly succeeded")
		}
		if got := device.GetBrightness(); got != 40 {
			t.Fatalf("brightness = %d, want unchanged value 40", got)
		}
		if device.DeviceProfile.BrightnessSlider != brightnessPointer || *device.DeviceProfile.BrightnessSlider != 40 {
			t.Fatalf("profile brightness = %#v, want unchanged pointer value 40", device.DeviceProfile.BrightnessSlider)
		}
		if calls.colors != 0 || calls.frames != 0 || calls.persistentFrames != 0 {
			t.Fatalf("cluster-owned output calls = colors %d, frames %d, persistent %d, want none", calls.colors, calls.frames, calls.persistentFrames)
		}
		if _, err := os.Stat(filepath.Join(profileDir, lightingDeviceTestSerial+".json")); !os.IsNotExist(err) {
			t.Fatalf("cluster rejection unexpectedly persisted a profile: %v", err)
		}
		if device.ControllerID() != controllerBefore {
			t.Fatalf("controller ID = %d, want unchanged %d", device.ControllerID(), controllerBefore)
		}
		if device.lifecycleDetached != detachedBefore || device.lifecycleActivating != activatingBefore {
			t.Fatal("cluster rejection changed lifecycle state")
		}
	})
}

func TestOpenRGBLightingEffectPersistenceAndOutputOrdering(t *testing.T) {
	t.Run("supported effect persists before output", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newLightingMutationDevice()
		var observedPersisted bool
		calls.beforeOutput = func() {
			state, found, err := device.lightingState.Resolve(device.Serial)
			observedPersisted = err == nil && found && state.SelectedEffect == "static" && state.Brightness == 40
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
		if !observedPersisted {
			t.Fatal("output callback did not observe persisted effect static")
		}
	})

	t.Run("persistence failure restores state and leaves lifecycle untouched", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newLightingMutationDevice()
		device.lightingState = failingLightingStateAccess{deviceLightingStateAccess: device.lightingState, err: errors.New("injected state failure")}
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
		_, calls := installLightingDeviceTestSeams(t)
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
		if _, found, err := device.lightingState.Resolve(device.Serial); err != nil || found {
			t.Fatalf("rejected effect changed target state: found=%t err=%v", found, err)
		}
	})

	t.Run("synchronous output failure retains persisted effect", func(t *testing.T) {
		preserveOpenRGBStatus(t)
		_, calls := installLightingDeviceTestSeams(t)
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
		state, found, stateErr := device.lightingState.Resolve(device.Serial)
		if stateErr != nil || !found || state.SelectedEffect != "static" {
			t.Fatalf("persisted effect state = %#v, %t, %v", state, found, stateErr)
		}
		if device.ControllerID() != -1 {
			t.Fatalf("controller ID = %d, want unavailable -1", device.ControllerID())
		}
	})
}

func TestOpenRGBLightingEffectTransitionSerialization(t *testing.T) {
	_, calls := installLightingDeviceTestSeams(t)
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

	state, found, stateErr := device.lightingState.Resolve(device.Serial)
	if stateErr != nil || !found || state.SelectedEffect != "off" {
		t.Fatalf("persisted effect while first transition waited = %#v, %t, %v", state, found, stateErr)
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
	_, calls := installLightingDeviceTestSeams(t)
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
	state, found, stateErr := device.lightingState.Resolve(device.Serial)
	if stateErr != nil || !found || state.SelectedEffect != "static" {
		t.Fatalf("persisted desired effect = %#v, %t, %v", state, found, stateErr)
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
			_, calls := installLightingDeviceTestSeams(t)
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
			state, found, stateErr := device.lightingState.Resolve(device.Serial)
			if stateErr != nil || !found || state.Brightness != test.brightness {
				t.Fatalf("persisted brightness = %#v, %t, %v; want %d", state, found, stateErr, test.brightness)
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
		_, calls := installLightingDeviceTestSeams(t)
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
		state, found, stateErr := device.lightingState.Resolve(device.Serial)
		if stateErr != nil || !found || state.Brightness != 50 {
			t.Fatalf("persisted brightness = %#v, %t, %v; want 50", state, found, stateErr)
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
	_, calls := installLightingDeviceTestSeams(t)
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
	state, found, stateErr := device.lightingState.Resolve(device.Serial)
	if stateErr != nil || !found || state.SelectedEffect != "static" {
		t.Fatalf("persisted effect = %#v, %t, %v", state, found, stateErr)
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

func testRGBOverrideColors() (rgb.Color, rgb.Color, rgb.Color) {
	return rgb.Color{Red: 0, Green: 0, Blue: 0, Brightness: 0.2, Temperature: 20, Position: 0.1, Hex: "000000"},
		rgb.Color{Red: 40, Green: 50, Blue: 60, Brightness: 0.3, Temperature: 45, Position: 0.5, Hex: "28323c"},
		rgb.Color{Red: 200, Green: 210, Blue: 220, Brightness: 0.4, Temperature: 80, Position: 0.9, Hex: "c8d2dc"}
}

func normalizedTestRGBOverride(enabled bool, start, middle, end rgb.Color, speed float64) RGBOverride {
	start.Brightness = 1
	middle.Brightness = 1
	end.Brightness = 1
	return RGBOverride{
		Enabled:        enabled,
		RGBStartColor:  start,
		RGBMiddleColor: middle,
		RGBEndColor:    end,
		RgbModeSpeed:   speed,
	}
}

func TestOpenRGBProcessSetRgbOverrideNilReceiver(t *testing.T) {
	var device *Device
	if got := device.ProcessSetRgbOverride(0, 0, false, rgb.Color{}, rgb.Color{}, rgb.Color{}, 0); got != 0 {
		t.Fatalf("nil ProcessSetRgbOverride() = %d, want 0", got)
	}
}

func TestOpenRGBRGBOverrideMutationAndPersistence(t *testing.T) {
	t.Run("enable complete black override before Static output", func(t *testing.T) {
		profileDir, calls := installLightingDeviceTestSeams(t)
		device := newStaticOverrideTestDevice(100)
		device.DeviceProfile.RGBOverride = nil
		start, middle, end := testRGBOverrideColors()
		speed := 4.5
		want := normalizedTestRGBOverride(true, start, middle, end, speed)
		var observed RGBOverride
		var observationErr error
		calls.beforeOutput = func() {
			profile, err := loadLightingDeviceProfile(profileDir)
			observationErr = err
			if err == nil && profile.RGBOverride != nil {
				observed = *profile.RGBOverride
			}
		}

		if err := device.SetRGBOverride(device.Serial, 0, 0, true, start, end, middle, &speed); err != nil {
			t.Fatalf("SetRGBOverride: %v", err)
		}
		if observationErr != nil {
			t.Fatalf("load profile during output: %v", observationErr)
		}
		if observed != want {
			t.Fatalf("persisted override observed before output = %#v, want %#v", observed, want)
		}
		if calls.frames != 1 || calls.colors != 0 || !bytes.Equal(calls.frameValues[0], make([]byte, device.colorCount*3)) {
			t.Fatalf("black Static output = colors %d, frames %v, want one black frame", calls.colors, calls.frameValues)
		}
		if device.DeviceProfile.RGBOverride == nil || *device.DeviceProfile.RGBOverride != want {
			t.Fatalf("in-memory override = %#v, want %#v", device.DeviceProfile.RGBOverride, want)
		}
		persisted := readLightingDeviceProfile(t, profileDir)
		if persisted.RGBOverride == nil || *persisted.RGBOverride != want {
			t.Fatalf("persisted override = %#v, want %#v", persisted.RGBOverride, want)
		}

		reloaded := newStaticOverrideTestDevice(100)
		reloaded.loadDeviceProfiles()
		if reloaded.DeviceProfile == nil || reloaded.DeviceProfile.RGBOverride == nil || *reloaded.DeviceProfile.RGBOverride != want {
			t.Fatalf("reloaded override = %#v, want %#v", reloaded.DeviceProfile, want)
		}
	})

	t.Run("edit replaces every override field", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newStaticOverrideTestDevice(100)
		device.DeviceProfile.RGBOverride = &RGBOverride{
			Enabled:        true,
			RGBStartColor:  rgb.Color{Red: 1, Green: 2, Blue: 3, Temperature: 4},
			RGBMiddleColor: rgb.Color{Red: 5, Green: 6, Blue: 7, Temperature: 8},
			RGBEndColor:    rgb.Color{Red: 9, Green: 10, Blue: 11, Temperature: 12},
			RgbModeSpeed:   1,
		}
		start, middle, end := testRGBOverrideColors()
		start.Red = 12
		speed := 10.0
		want := normalizedTestRGBOverride(true, start, middle, end, speed)

		if err := device.SetRGBOverride(device.Serial, 0, 0, true, start, end, middle, &speed); err != nil {
			t.Fatalf("SetRGBOverride: %v", err)
		}
		if device.DeviceProfile.RGBOverride == nil || *device.DeviceProfile.RGBOverride != want {
			t.Fatalf("edited override = %#v, want complete replacement %#v", device.DeviceProfile.RGBOverride, want)
		}
		if calls.frames != 1 || calls.colors != 0 {
			t.Fatalf("edit output calls = colors %d, frames %d, want 0, 1", calls.colors, calls.frames)
		}
	})

	for _, test := range []struct {
		name      string
		prior     *RGBOverride
		speed     *float64
		wantSpeed float64
	}{
		{name: "omitted speed defaults to five", speed: nil, wantSpeed: 5},
		{name: "omitted speed preserves prior", prior: &RGBOverride{Enabled: true, RgbModeSpeed: 3.25}, speed: nil, wantSpeed: 3.25},
		{name: "zero speed accepted", speed: func() *float64 { value := 0.0; return &value }(), wantSpeed: 0},
		{name: "maximum speed accepted", speed: func() *float64 { value := 10.0; return &value }(), wantSpeed: 10},
	} {
		t.Run(test.name, func(t *testing.T) {
			profileDir, _ := installLightingDeviceTestSeams(t)
			device := newStaticOverrideTestDevice(100)
			device.DeviceProfile.RGBOverride = test.prior
			start, middle, end := testRGBOverrideColors()
			if err := device.SetRGBOverride(device.Serial, 0, 0, false, start, end, middle, test.speed); err != nil {
				t.Fatalf("SetRGBOverride: %v", err)
			}
			persisted := readLightingDeviceProfile(t, profileDir)
			if persisted.RGBOverride == nil || persisted.RGBOverride.RgbModeSpeed != test.wantSpeed || persisted.RGBOverride.Enabled {
				t.Fatalf("persisted override = %#v, want disabled speed %v", persisted.RGBOverride, test.wantSpeed)
			}
		})
	}

	t.Run("disable restores Static zone frame", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newStaticOverrideTestDevice(100)
		device.DeviceProfile.RGBOverride.Enabled = true
		baseFrame := device.buildZoneFrame()
		start, middle, end := testRGBOverrideColors()
		speed := 2.0

		if err := device.SetRGBOverride(device.Serial, 0, 0, false, start, end, middle, &speed); err != nil {
			t.Fatalf("disable override: %v", err)
		}
		if calls.frames != 1 || !bytes.Equal(calls.frameValues[0], baseFrame) {
			t.Fatalf("disabled Static frame = %v, want base %v", calls.frameValues, baseFrame)
		}
		if device.DeviceProfile.RGBOverride == nil || device.DeviceProfile.RGBOverride.Enabled {
			t.Fatalf("disabled override was not retained: %#v", device.DeviceProfile.RGBOverride)
		}
	})
}

func TestOpenRGBRGBOverrideValidation(t *testing.T) {
	validStart, validMiddle, validEnd := testRGBOverrideColors()
	validSpeed := 2.0
	tests := []struct {
		name      string
		serial    string
		channel   int
		subdevice int
		start     rgb.Color
		middle    rgb.Color
		end       rgb.Color
		speed     *float64
		prepare   func(*Device)
	}{
		{name: "nonzero channel", serial: lightingDeviceTestSerial, channel: 1, start: validStart, middle: validMiddle, end: validEnd, speed: &validSpeed},
		{name: "nonzero subdevice", serial: lightingDeviceTestSerial, subdevice: 1, start: validStart, middle: validMiddle, end: validEnd, speed: &validSpeed},
		{name: "negative red", serial: lightingDeviceTestSerial, start: func() rgb.Color { value := validStart; value.Red = -1; return value }(), middle: validMiddle, end: validEnd, speed: &validSpeed},
		{name: "middle green above range", serial: lightingDeviceTestSerial, start: validStart, middle: func() rgb.Color { value := validMiddle; value.Green = 256; return value }(), end: validEnd, speed: &validSpeed},
		{name: "nonfinite end blue", serial: lightingDeviceTestSerial, start: validStart, middle: validMiddle, end: func() rgb.Color { value := validEnd; value.Blue = math.Inf(1); return value }(), speed: &validSpeed},
		{name: "negative start temperature", serial: lightingDeviceTestSerial, start: func() rgb.Color { value := validStart; value.Temperature = -1; return value }(), middle: validMiddle, end: validEnd, speed: &validSpeed},
		{name: "middle temperature above range", serial: lightingDeviceTestSerial, start: validStart, middle: func() rgb.Color { value := validMiddle; value.Temperature = 106; return value }(), end: validEnd, speed: &validSpeed},
		{name: "nonfinite end temperature", serial: lightingDeviceTestSerial, start: validStart, middle: validMiddle, end: func() rgb.Color { value := validEnd; value.Temperature = math.NaN(); return value }(), speed: &validSpeed},
		{name: "negative speed", serial: lightingDeviceTestSerial, start: validStart, middle: validMiddle, end: validEnd, speed: func() *float64 { value := -1.0; return &value }()},
		{name: "speed above range", serial: lightingDeviceTestSerial, start: validStart, middle: validMiddle, end: validEnd, speed: func() *float64 { value := 11.0; return &value }()},
		{name: "NaN speed", serial: lightingDeviceTestSerial, start: validStart, middle: validMiddle, end: validEnd, speed: func() *float64 { value := math.NaN(); return &value }()},
		{name: "infinite speed", serial: lightingDeviceTestSerial, start: validStart, middle: validMiddle, end: validEnd, speed: func() *float64 { value := math.Inf(-1); return &value }()},
		{name: "mismatched identity", serial: "openrgb-other-device", start: validStart, middle: validMiddle, end: validEnd, speed: &validSpeed},
		{name: "inactive lifecycle", serial: lightingDeviceTestSerial, start: validStart, middle: validMiddle, end: validEnd, speed: &validSpeed, prepare: func(device *Device) { device.lifecycleDetached = true }},
		{name: "missing profile", serial: lightingDeviceTestSerial, start: validStart, middle: validMiddle, end: validEnd, speed: &validSpeed, prepare: func(device *Device) { device.DeviceProfile = nil }},
		{name: "inactive profile", serial: lightingDeviceTestSerial, start: validStart, middle: validMiddle, end: validEnd, speed: &validSpeed, prepare: func(device *Device) { device.DeviceProfile.Active = false }},
		{name: "unsupported configured effect", serial: lightingDeviceTestSerial, start: validStart, middle: validMiddle, end: validEnd, speed: &validSpeed, prepare: func(device *Device) { device.effect = "obsolete" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profileDir, calls := installLightingDeviceTestSeams(t)
			device := newStaticOverrideTestDevice(100)
			if test.prepare != nil {
				test.prepare(device)
			}
			profileBefore := cloneDeviceProfile(device.DeviceProfile)

			if err := device.SetRGBOverride(test.serial, test.channel, test.subdevice, true, test.start, test.end, test.middle, test.speed); err == nil {
				t.Fatal("invalid override unexpectedly succeeded")
			}
			if calls.colors != 0 || calls.frames != 0 || calls.persistentFrames != 0 {
				t.Fatalf("invalid override output calls = colors %d, frames %d, persistent %d", calls.colors, calls.frames, calls.persistentFrames)
			}
			entries, err := os.ReadDir(profileDir)
			if err != nil {
				t.Fatalf("read profile directory: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("invalid override wrote %d profile files", len(entries))
			}
			if !reflect.DeepEqual(device.DeviceProfile, profileBefore) {
				t.Fatalf("invalid override changed profile from %#v to %#v", profileBefore, device.DeviceProfile)
			}
		})
	}
}

func TestOpenRGBRGBOverrideFailureAndClusterSemantics(t *testing.T) {
	t.Run("persistence failure fully restores nil override and worker", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		blockedPath := filepath.Join(t.TempDir(), "profiles")
		if err := os.WriteFile(blockedPath, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		deviceProfileDir = func() string { return blockedPath }
		device := newStaticOverrideTestDevice(100)
		device.DeviceProfile.RGBOverride = nil
		profileBefore := cloneDeviceProfile(device.DeviceProfile)
		stop := make(chan struct{})
		device.running = true
		device.stopChan = stop
		start, middle, end := testRGBOverrideColors()
		speed := 2.0

		if err := device.SetRGBOverride(device.Serial, 0, 0, true, start, end, middle, &speed); err == nil {
			t.Fatal("override succeeded despite persistence failure")
		}
		if !reflect.DeepEqual(device.DeviceProfile, profileBefore) || device.DeviceProfile.RGBOverride != nil {
			t.Fatalf("profile rollback = %#v, want %#v with nil override", device.DeviceProfile, profileBefore)
		}
		if !device.running || device.stopChan != stop {
			t.Fatal("persistence failure replaced the current worker")
		}
		select {
		case <-stop:
			t.Fatal("persistence failure stopped the current worker")
		default:
		}
		if calls.colors != 0 || calls.frames != 0 || calls.persistentFrames != 0 {
			t.Fatalf("persistence failure output calls = colors %d, frames %d, persistent %d", calls.colors, calls.frames, calls.persistentFrames)
		}
		device.running = false
		device.stopChan = nil
	})

	t.Run("Static output failure retains persisted desired override", func(t *testing.T) {
		preserveOpenRGBStatus(t)
		profileDir, calls := installLightingDeviceTestSeams(t)
		calls.err = errors.New("test checked override output failure")
		device := newStaticOverrideTestDevice(100)
		start, middle, end := testRGBOverrideColors()
		start.Red = 100
		speed := 2.0
		want := normalizedTestRGBOverride(true, start, middle, end, speed)

		if err := device.SetRGBOverride(device.Serial, 0, 0, true, start, end, middle, &speed); err == nil {
			t.Fatal("override succeeded despite Static output failure")
		}
		persisted := readLightingDeviceProfile(t, profileDir)
		if persisted.RGBOverride == nil || *persisted.RGBOverride != want {
			t.Fatalf("persisted desired override = %#v, want %#v", persisted.RGBOverride, want)
		}
		if device.DeviceProfile.RGBOverride == nil || *device.DeviceProfile.RGBOverride != want {
			t.Fatalf("in-memory desired override = %#v, want %#v", device.DeviceProfile.RGBOverride, want)
		}
		if calls.frames != 1 || device.ControllerID() != -1 {
			t.Fatalf("output failure = frames %d, controller %d; want 1, -1", calls.frames, device.ControllerID())
		}
	})

	t.Run("cluster persists inactive local state without replacing output", func(t *testing.T) {
		profileDir, calls := installLightingDeviceTestSeams(t)
		device := newStaticOverrideTestDevice(100)
		device.DeviceProfile.RGBCluster = true
		stop := make(chan struct{})
		device.running = true
		device.stopChan = stop
		start, middle, end := testRGBOverrideColors()
		speed := 6.0

		if err := device.SetRGBOverride(device.Serial, 0, 0, true, start, end, middle, &speed); err != nil {
			t.Fatalf("clustered override: %v", err)
		}
		persisted := readLightingDeviceProfile(t, profileDir)
		if persisted.RGBOverride == nil || !persisted.RGBOverride.Enabled || persisted.RGBOverride.RgbModeSpeed != speed || !persisted.RGBCluster {
			t.Fatalf("clustered persisted profile = %#v", persisted)
		}
		if calls.colors != 0 || calls.frames != 0 || calls.persistentFrames != 0 {
			t.Fatalf("clustered override output calls = colors %d, frames %d, persistent %d", calls.colors, calls.frames, calls.persistentFrames)
		}
		if !device.running || device.stopChan != stop || device.DeviceProfile == nil || !device.DeviceProfile.RGBCluster {
			t.Fatal("clustered override changed cluster or local worker state")
		}
		device.running = false
		device.stopChan = nil
	})
}

func TestOpenRGBRGBOverrideAnimatedAndOffReapplication(t *testing.T) {
	t.Run("animated enable replaces one worker with persisted override state", func(t *testing.T) {
		profileDir, calls := installLightingDeviceTestSeams(t)
		device := newLightingMutationDevice()
		device.effect = "rainbow"
		device.DeviceProfile.RGBProfile = "rainbow"
		device.Rgb = &rgb.RGB{Profiles: map[string]rgb.Profile{
			"rainbow": {
				ProfileName: "rainbow",
				Speed:       2.75,
				StartColor:  rgb.Color{Red: 10, Green: 20, Blue: 30, Brightness: 1},
				MiddleColor: rgb.Color{Red: 40, Green: 50, Blue: 60, Brightness: 1},
				EndColor:    rgb.Color{Red: 70, Green: 80, Blue: 90, Brightness: 1},
			},
		}}
		base := device.Rgb.Profiles["rainbow"]
		settings, err := lightingSettingsFromRGBProfile("rainbow", base)
		if err != nil || device.lightingEffects.Set(device.Serial, "rainbow", settings) != nil {
			t.Fatalf("seed canonical Rainbow settings: %v", err)
		}
		canonical := device.GetRgbProfile("rainbow")
		if canonical == nil {
			t.Fatal("canonical Rainbow settings are unavailable")
		}
		if err := device.SetEffect("rainbow"); err != nil {
			t.Fatalf("start prior worker: %v", err)
		}
		select {
		case <-calls.persistentOutput:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for prior worker")
		}
		device.mu.Lock()
		oldDone := device.doneChan
		device.mu.Unlock()

		start, middle, end := testRGBOverrideColors()
		speed := 6.25
		want := normalizedTestRGBOverride(true, start, middle, end, speed)
		persistedOutput := make(chan struct{}, 1)
		var observationErr error
		calls.beforeOutput = func() {
			profile, err := loadLightingDeviceProfile(profileDir)
			if err != nil {
				observationErr = err
				return
			}
			if profile.RGBOverride != nil && *profile.RGBOverride == want {
				select {
				case persistedOutput <- struct{}{}:
				default:
				}
			}
		}
		if err := device.SetRGBOverride(device.Serial, 0, 0, true, start, end, middle, &speed); err != nil {
			device.Stop()
			t.Fatalf("enable animated override: %v", err)
		}
		select {
		case <-persistedOutput:
		case <-time.After(2 * time.Second):
			device.Stop()
			t.Fatalf("timed out waiting for persisted override output; observation error %v", observationErr)
		}
		device.mu.Lock()
		newDone := device.doneChan
		runnerStart := *device.rgbRunner.RGBStartColor
		runnerMiddle := *device.rgbRunner.RGBMiddleColor
		runnerEnd := *device.rgbRunner.RGBEndColor
		runnerSpeed := device.rgbRunner.RgbModeSpeed
		workerActive := device.running
		device.mu.Unlock()
		device.Stop()

		select {
		case <-oldDone:
		default:
			t.Fatal("prior animation worker was not stopped")
		}
		if !workerActive || newDone == nil || newDone == oldDone {
			t.Fatal("animated override did not install exactly one replacement worker")
		}
		if runnerStart != canonical.StartColor || runnerMiddle != canonical.MiddleColor || runnerEnd != canonical.EndColor || runnerSpeed != canonical.Speed {
			t.Fatalf("presentation-only override changed animated state = %#v/%#v/%#v speed %v, want %#v/%#v/%#v speed %v",
				runnerStart, runnerMiddle, runnerEnd, runnerSpeed,
				canonical.StartColor, canonical.MiddleColor, canonical.EndColor, canonical.Speed)
		}
	})

	t.Run("animated disable replaces one worker with base profile state", func(t *testing.T) {
		profileDir, calls := installLightingDeviceTestSeams(t)
		device := newLightingMutationDevice()
		device.effect = "rainbow"
		device.DeviceProfile.RGBProfile = "rainbow"
		base := rgb.Profile{
			ProfileName: "rainbow",
			Speed:       2.75,
			StartColor:  rgb.Color{Red: 10, Green: 20, Blue: 30, Brightness: 1},
			MiddleColor: rgb.Color{Red: 40, Green: 50, Blue: 60, Brightness: 1},
			EndColor:    rgb.Color{Red: 70, Green: 80, Blue: 90, Brightness: 1},
		}
		device.Rgb = &rgb.RGB{Profiles: map[string]rgb.Profile{"rainbow": base}}
		settings, err := lightingSettingsFromRGBProfile("rainbow", base)
		if err != nil || device.lightingEffects.Set(device.Serial, "rainbow", settings) != nil {
			t.Fatalf("seed canonical Rainbow settings: %v", err)
		}
		canonical := device.GetRgbProfile("rainbow")
		if canonical == nil {
			t.Fatal("canonical Rainbow settings are unavailable")
		}
		device.DeviceProfile.RGBOverride = &RGBOverride{
			Enabled:        true,
			RGBStartColor:  rgb.Color{Red: 101, Green: 102, Blue: 103, Brightness: 1},
			RGBMiddleColor: rgb.Color{Red: 111, Green: 112, Blue: 113, Brightness: 1},
			RGBEndColor:    rgb.Color{Red: 121, Green: 122, Blue: 123, Brightness: 1},
			RgbModeSpeed:   7,
		}
		if err := device.SetEffect("rainbow"); err != nil {
			t.Fatalf("start prior worker: %v", err)
		}
		select {
		case <-calls.persistentOutput:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for prior worker")
		}
		device.mu.Lock()
		oldDone := device.doneChan
		device.mu.Unlock()
		persistedOutput := make(chan struct{}, 1)
		var observationErr error
		calls.beforeOutput = func() {
			profile, err := loadLightingDeviceProfile(profileDir)
			observationErr = err
			if err == nil && profile.RGBOverride != nil && !profile.RGBOverride.Enabled {
				select {
				case persistedOutput <- struct{}{}:
				default:
				}
			}
		}
		start, middle, end := testRGBOverrideColors()
		speed := 3.0
		if err := device.SetRGBOverride(device.Serial, 0, 0, false, start, end, middle, &speed); err != nil {
			device.Stop()
			t.Fatalf("disable animated override: %v", err)
		}
		select {
		case <-persistedOutput:
		case <-time.After(2 * time.Second):
			device.Stop()
			t.Fatal("timed out waiting for replacement worker")
		}
		device.mu.Lock()
		newDone := device.doneChan
		runnerStart := *device.rgbRunner.RGBStartColor
		runnerMiddle := *device.rgbRunner.RGBMiddleColor
		runnerEnd := *device.rgbRunner.RGBEndColor
		runnerSpeed := device.rgbRunner.RgbModeSpeed
		workerActive := device.running
		device.mu.Unlock()
		device.Stop()

		if observationErr != nil {
			t.Fatalf("animated persisted-before-output observation error: %v", observationErr)
		}
		select {
		case <-oldDone:
		default:
			t.Fatal("prior animation worker was not stopped")
		}
		if !workerActive || newDone == nil || newDone == oldDone {
			t.Fatal("animated override did not install exactly one replacement worker")
		}
		if runnerStart != canonical.StartColor || runnerMiddle != canonical.MiddleColor || runnerEnd != canonical.EndColor || runnerSpeed != canonical.Speed {
			t.Fatalf("disabled animated state = %#v/%#v/%#v speed %v, want base %#v/%#v/%#v speed %v",
				runnerStart, runnerMiddle, runnerEnd, runnerSpeed, canonical.StartColor, canonical.MiddleColor, canonical.EndColor, canonical.Speed)
		}
	})

	t.Run("Off remains black", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newLightingMutationDevice()
		start := rgb.Color{Red: 255, Green: 1, Blue: 2, Temperature: 20}
		middle := rgb.Color{Red: 3, Green: 254, Blue: 4, Temperature: 40}
		end := rgb.Color{Red: 5, Green: 6, Blue: 253, Temperature: 80}
		speed := 2.0

		if err := device.SetRGBOverride(device.Serial, 0, 0, true, start, end, middle, &speed); err != nil {
			t.Fatalf("Off override: %v", err)
		}
		if calls.colors != 1 || calls.frames != 0 || !reflect.DeepEqual(calls.colorValues, [][]byte{{0, 0, 0}}) {
			t.Fatalf("Off output = colors %v, frames %v, want one black color", calls.colorValues, calls.frameValues)
		}
	})
}

func openRGBTemperatureMiddleColorProfile(name string) rgb.Profile {
	return rgb.Profile{
		ProfileName: name,
		Speed:       2.5,
		Brightness:  0.75,
		Smoothness:  7,
		StartColor: rgb.Color{
			Red: 11, Green: 22, Blue: 33, Brightness: 0.4, Temperature: 25,
		},
		MiddleColor: rgb.Color{
			Red: 44, Green: 55, Blue: 66, Brightness: 0.7, Temperature: 50,
		},
		EndColor: rgb.Color{
			Red: 77, Green: 88, Blue: 99, Brightness: 0.9, Temperature: 75,
		},
		Gradients: map[int]rgb.Color{
			0: {Red: 5, Green: 15, Blue: 25, Position: 0},
			1: {Red: 35, Green: 45, Blue: 55, Position: 1},
		},
		MinTemp:         25,
		MaxTemp:         75,
		AlternateColors: true,
		RgbDirection:    1,
		PerLed:          true,
		Version:         9,
	}
}

func newOpenRGBTemperatureMiddleColorDevice(t *testing.T, effect, serial string) *Device {
	t.Helper()

	device := newLightingMutationDevice()
	device.Serial = serial
	device.colorCount = 2
	device.effect = "off"
	device.RGBModes = []string{"off", "static", "cpu-temperature", "gpu-temperature", "colorpulse", "gradient"}
	device.DeviceProfile.RGBProfile = "off"
	device.Rgb = &rgb.RGB{
		Device: device.Product,
		Profiles: map[string]rgb.Profile{
			effect: openRGBTemperatureMiddleColorProfile(effect),
		},
	}
	settings, err := lightingSettingsFromRGBProfile(effect, device.Rgb.Profiles[effect])
	if err != nil {
		t.Fatalf("convert %q profile: %v", effect, err)
	}
	if err = device.lightingEffects.Set(serial, effect, settings); err != nil {
		t.Fatalf("seed canonical %q settings: %v", effect, err)
	}
	return device
}

func prepareOpenRGBTemperatureProfilePath(t *testing.T, serial string) string {
	t.Helper()

	rgbPath := filepath.Join(config.GetPaths().MutableDataRoot, "database", "rgb", serial+".json")
	if err := os.MkdirAll(filepath.Dir(rgbPath), 0o755); err != nil {
		t.Fatalf("create RGB profile directory: %v", err)
	}
	_ = os.Remove(rgbPath)
	t.Cleanup(func() { _ = os.Remove(rgbPath) })
	return rgbPath
}

func runOpenRGBTemperatureMiddleColorWorker(
	t *testing.T,
	device *Device,
	calls *lightingOutputCalls,
	effect string,
) (rgb.Color, rgb.Color, rgb.Color) {
	t.Helper()

	if err := device.SetEffect(effect); err != nil {
		t.Fatalf("SetEffect(%q): %v", effect, err)
	}
	select {
	case <-calls.persistentOutput:
	case <-time.After(2 * time.Second):
		device.Stop()
		t.Fatalf("timed out waiting for %q worker output", effect)
	}
	device.Stop()

	device.mu.Lock()
	defer device.mu.Unlock()
	if device.rgbRunner == nil || device.rgbRunner.RGBStartColor == nil ||
		device.rgbRunner.RGBMiddleColor == nil || device.rgbRunner.RGBEndColor == nil {
		t.Fatalf("%q worker did not install all three profile colors", effect)
	}
	return *device.rgbRunner.RGBStartColor, *device.rgbRunner.RGBMiddleColor, *device.rgbRunner.RGBEndColor
}

func TestOpenRGBTemperatureMiddleColorUpdateAndPersistence(t *testing.T) {
	serial := "openrgb-temperature-middle-color-persistence-test"
	profileName := "cpu-temperature"
	device := newOpenRGBTemperatureMiddleColorDevice(t, profileName, serial)
	updated := openRGBTemperatureMiddleColorProfile(profileName)
	updated.StartColor = rgb.Color{Red: 101, Green: 102, Blue: 103, Brightness: 0.95, Temperature: 20}
	updated.MiddleColor = rgb.Color{Red: 111, Green: 112, Blue: 113, Brightness: 0.85, Temperature: 45}
	updated.EndColor = rgb.Color{Red: 121, Green: 122, Blue: 123, Brightness: 0.75, Temperature: 70}

	if result := device.UpdateRgbProfileData(profileName, updated); result != 1 {
		t.Fatalf("UpdateRgbProfileData result = %d, want 1", result)
	}
	expectedSettings, err := lightingSettingsFromRGBProfile(profileName, updated)
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := device.resolveLightingSettings(profileName)
	if err != nil || !resolution.Customized || !reflect.DeepEqual(resolution.Settings, expectedSettings) {
		t.Fatalf("persisted canonical temperature settings = %#v, %v; want %#v", resolution, err, expectedSettings)
	}
	actual := device.GetRgbProfile(profileName)
	expectedProfile := rgbProfileFromLightingSettings(expectedSettings)
	if actual == nil || !reflect.DeepEqual(*actual, expectedProfile) {
		t.Fatalf("resolved RGB compatibility profile = %#v, want %#v", actual, expectedProfile)
	}

	updated.MiddleColor.Red = 250
	actual = device.GetRgbProfile(profileName)
	if actual == nil || actual.MiddleColor.Red == 250 {
		t.Fatal("stored RGB profile aliases caller-owned update data")
	}

	t.Run("GPU accepts black MiddleColor", func(t *testing.T) {
		serial := "openrgb-gpu-temperature-black-middle-color-test"
		profileName := "gpu-temperature"
		device := newOpenRGBTemperatureMiddleColorDevice(t, profileName, serial)
		updated := openRGBTemperatureMiddleColorProfile(profileName)
		updated.MiddleColor = rgb.Color{Brightness: 0.1, Temperature: 50}

		if result := device.UpdateRgbProfileData(profileName, updated); result != 1 {
			t.Fatalf("UpdateRgbProfileData result = %d, want 1", result)
		}
		actual := device.GetRgbProfile(profileName)
		if actual == nil || actual.MiddleColor.Red != 0 || actual.MiddleColor.Green != 0 || actual.MiddleColor.Blue != 0 || actual.MiddleColor.Temperature != 50 {
			t.Fatalf("updated black MiddleColor = %#v", actual)
		}
	})
}

func TestOpenRGBTemperatureMiddleColorPartialStaticUpdate(t *testing.T) {
	serial := "openrgb-temperature-middle-color-partial-static-test"
	profileName := "static"
	device := newOpenRGBTemperatureMiddleColorDevice(t, profileName, serial)
	color := rgb.Color{Red: 201, Green: 101, Blue: 51}
	complete := rgb.Profile{ProfileName: profileName, StartColor: color}

	if result := device.UpdateRgbProfileData(profileName, complete); result != 1 {
		t.Fatalf("UpdateRgbProfileData result = %d, want 1", result)
	}
	actual := device.GetRgbProfile(profileName)
	if actual == nil || actual.StartColor.Red != color.Red || actual.StartColor.Green != color.Green || actual.StartColor.Blue != color.Blue || actual.MiddleColor != (rgb.Color{}) || actual.EndColor != (rgb.Color{}) {
		t.Fatalf("resolved complete Static profile = %#v", actual)
	}
}

func TestOpenRGBTemperatureMiddleColorWorkerRefresh(t *testing.T) {
	tests := []struct {
		name              string
		effect            string
		serial            string
		cpuTemperature    float32
		nvidiaTemperature float32
		amdTemperature    float32
		wantCPU           bool
		wantNVIDIA        bool
		wantAMD           bool
	}{
		{
			name:           "CPU",
			effect:         "cpu-temperature",
			serial:         "openrgb-middle-worker-cpu",
			cpuTemperature: 42,
			wantCPU:        true,
		},
		{
			name:              "GPU NVIDIA",
			effect:            "gpu-temperature",
			serial:            "openrgb-middle-worker-gpu-nvidia",
			nvidiaTemperature: 58,
			amdTemperature:    63,
			wantNVIDIA:        true,
		},
		{
			name:              "GPU AMD fallback",
			effect:            "gpu-temperature",
			serial:            "openrgb-middle-worker-gpu-amd",
			nvidiaTemperature: 0,
			amdTemperature:    64,
			wantNVIDIA:        true,
			wantAMD:           true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, calls := installLightingDeviceTestSeams(t)
			temperatureCalls := installLightingTemperatureTestSeams(
				t,
				test.cpuTemperature,
				test.nvidiaTemperature,
				test.amdTemperature,
			)
			device := newOpenRGBTemperatureMiddleColorDevice(t, test.effect, test.serial)
			profile := device.GetRgbProfile(test.effect)
			if profile == nil {
				t.Fatalf("canonical %q settings are unavailable", test.effect)
			}

			start, middle, end := runOpenRGBTemperatureMiddleColorWorker(t, device, calls, test.effect)
			if start != profile.StartColor || middle != profile.MiddleColor || end != profile.EndColor {
				t.Fatalf("worker colors = %#v, %#v, %#v; want %#v, %#v, %#v",
					start, middle, end, profile.StartColor, profile.MiddleColor, profile.EndColor)
			}
			if (temperatureCalls.cpu > 0) != test.wantCPU ||
				(temperatureCalls.nvidia > 0) != test.wantNVIDIA ||
				(temperatureCalls.amd > 0) != test.wantAMD {
				t.Fatalf("temperature calls = CPU %d, NVIDIA %d, AMD %d; want used %t, %t, %t",
					temperatureCalls.cpu, temperatureCalls.nvidia, temperatureCalls.amd,
					test.wantCPU, test.wantNVIDIA, test.wantAMD)
			}
			if test.wantAMD && temperatureCalls.amd != temperatureCalls.nvidia {
				t.Fatalf("GPU fallback calls = NVIDIA %d, AMD %d, want one AMD fallback per NVIDIA result",
					temperatureCalls.nvidia, temperatureCalls.amd)
			}
			for _, index := range temperatureCalls.nvidiaIndexes {
				if index != config.GetConfig().DefaultNvidiaGPU {
					t.Fatalf("NVIDIA GPU index = %d, want configured default %d",
						index, config.GetConfig().DefaultNvidiaGPU)
				}
			}
		})
	}
}

func TestOpenRGBTemperatureMiddleColorOverridePrecedence(t *testing.T) {
	_, calls := installLightingDeviceTestSeams(t)
	temperatureCalls := installLightingTemperatureTestSeams(t, 45, 60, 65)
	effect := "cpu-temperature"
	device := newOpenRGBTemperatureMiddleColorDevice(t, effect, "openrgb-middle-override-test")
	base := device.GetRgbProfile(effect)
	if base == nil {
		t.Fatal("canonical temperature settings are unavailable")
	}
	overrideMiddle := rgb.Color{Red: 201, Green: 202, Blue: 203, Brightness: 1, Temperature: 55}
	device.DeviceProfile.RGBOverride = &RGBOverride{
		Enabled:        true,
		RGBStartColor:  rgb.Color{Red: 181, Green: 182, Blue: 183, Brightness: 1},
		RGBMiddleColor: overrideMiddle,
		RGBEndColor:    rgb.Color{Red: 221, Green: 222, Blue: 223, Brightness: 1},
		RgbModeSpeed:   3.5,
	}

	_, middle, _ := runOpenRGBTemperatureMiddleColorWorker(t, device, calls, effect)
	if middle != base.MiddleColor {
		t.Fatalf("presentation-only override middle = %#v, want base %#v", middle, base.MiddleColor)
	}
	device.DeviceProfile.RGBOverride.Enabled = false
	_, middle, _ = runOpenRGBTemperatureMiddleColorWorker(t, device, calls, effect)
	if middle != base.MiddleColor {
		t.Fatalf("disabled override middle = %#v, want base %#v", middle, base.MiddleColor)
	}
	if resolved := device.GetRgbProfile(effect); resolved == nil || resolved.MiddleColor != base.MiddleColor {
		t.Fatal("override execution changed canonical temperature settings")
	}
	if temperatureCalls.cpu == 0 || temperatureCalls.nvidia != 0 || temperatureCalls.amd != 0 {
		t.Fatalf("override worker temperature calls = CPU %d, NVIDIA %d, AMD %d; want CPU only",
			temperatureCalls.cpu, temperatureCalls.nvidia, temperatureCalls.amd)
	}
}

func TestOpenRGBTemperatureMiddleColorNonTemperatureAndLegacyRegression(t *testing.T) {
	t.Run("Static output", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newStaticOverrideTestDevice(100)
		device.DeviceProfile.RGBOverride.Enabled = false
		expected := device.buildZoneFrame()

		if err := device.SetEffect("static"); err != nil {
			t.Fatalf("SetEffect(static): %v", err)
		}
		if calls.frames != 1 || !bytes.Equal(calls.frameValues[0], expected) {
			t.Fatalf("Static output = %#v, want %#v", calls.frameValues, expected)
		}
	})

	t.Run("two-color and gradient renderers", func(t *testing.T) {
		start := rgb.Color{Red: 10, Green: 20, Blue: 30, Brightness: 1}
		end := rgb.Color{Red: 210, Green: 220, Blue: 230, Brightness: 1}
		middleA := rgb.Color{Red: 1, Green: 2, Blue: 3, Brightness: 1}
		middleB := rgb.Color{Red: 251, Green: 252, Blue: 253, Brightness: 1}
		newRunner := func(middle *rgb.Color) *rgb.ActiveRGB {
			runner := rgb.New(2, 1_000_000_000, &start, &end, 1, 0, 0, true)
			runner.RGBMiddleColor = middle
			return runner
		}
		pulseStart := time.Now().Add(-time.Second)
		pulseA, pulseB := newRunner(&middleA), newRunner(&middleB)
		pulseA.Colorpulse(&pulseStart)
		pulseB.Colorpulse(&pulseStart)
		if !bytes.Equal(pulseA.Output, pulseB.Output) {
			t.Fatal("two-color output changed when only MiddleColor changed")
		}

		gradients := map[int]rgb.Color{
			0: {Red: 10, Green: 30, Blue: 50, Position: 0, Brightness: 1},
			1: {Red: 60, Green: 80, Blue: 100, Position: 1, Brightness: 1},
		}
		gradientStart := time.Now().Add(-time.Second)
		gradientA, gradientB := newRunner(&middleA), newRunner(&middleB)
		gradientA.ColorshiftGradient(gradientStart, gradients, 1_000_000_000)
		gradientB.ColorshiftGradient(gradientStart, gradients, 1_000_000_000)
		if !bytes.Equal(gradientA.Output, gradientB.Output) {
			t.Fatal("gradient output changed when only MiddleColor changed")
		}
	})

	t.Run("missing legacy middle", func(t *testing.T) {
		serial := "openrgb-temperature-middle-color-legacy-test"
		rgbPath := filepath.Join(config.GetPaths().MutableDataRoot, "database", "rgb", serial+".json")
		if err := os.MkdirAll(filepath.Dir(rgbPath), 0o755); err != nil {
			t.Fatalf("create RGB profile directory: %v", err)
		}
		_ = os.Remove(rgbPath)
		t.Cleanup(func() { _ = os.Remove(rgbPath) })
		legacy := map[string]interface{}{
			"device": "Legacy OpenRGB Controller",
			"profiles": map[string]interface{}{
				"cpu-temperature": map[string]interface{}{
					"profileName": "cpu-temperature",
					"start":       rgb.Color{Red: 12, Green: 23, Blue: 34},
					"end":         rgb.Color{Red: 210, Green: 220, Blue: 230},
				},
			},
		}
		data, err := json.Marshal(legacy)
		if err != nil {
			t.Fatalf("marshal legacy RGB profile: %v", err)
		}
		if err = os.WriteFile(rgbPath, data, 0o600); err != nil {
			t.Fatalf("write legacy RGB profile: %v", err)
		}

		device := &Device{Serial: serial, Product: "Legacy OpenRGB Controller"}
		profile := device.GetRgbProfile("cpu-temperature")
		if profile != nil {
			t.Fatalf("legacy target-local RGB profile was consulted: %#v", profile)
		}
	})
}
