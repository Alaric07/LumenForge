package openrgbimport

import (
	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
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

func canonicalTestSettingsFromRGBProfile(effect string, profile rgb.Profile) (lightingsettings.EffectSettings, error) {
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeDevice) {
		return lightingsettings.EffectSettings{}, errors.New("unsupported OpenRGB effect")
	}
	color := func(value rgb.Color) lightingsettings.Color {
		return lightingsettings.Color{Red: value.Red, Green: value.Green, Blue: value.Blue}
	}
	settings := lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: effect}
	if descriptor.SupportsSpeed {
		speed := profile.Speed
		settings.Speed = &speed
	}
	switch descriptor.PaletteKind {
	case rgb.LightingPaletteNone, rgb.LightingPaletteGenerated:
	case rgb.LightingPaletteStaticSingle:
		settings.SingleColor = &lightingsettings.SingleColorSettings{Color: color(profile.StartColor)}
	case rgb.LightingPaletteTwoColor:
		settings.TwoColor = &lightingsettings.TwoColorSettings{
			Start: color(profile.StartColor),
			End:   color(profile.EndColor),
		}
	case rgb.LightingPaletteTemperatureThree:
		settings.Temperature = &lightingsettings.TemperatureSettings{
			Low: lightingsettings.TemperaturePoint{
				Color: color(profile.StartColor), Celsius: profile.StartColor.Temperature,
			},
			Middle: lightingsettings.TemperaturePoint{
				Color: color(profile.MiddleColor), Celsius: profile.MiddleColor.Temperature,
			},
			High: lightingsettings.TemperaturePoint{
				Color: color(profile.EndColor), Celsius: profile.EndColor.Temperature,
			},
		}
	case rgb.LightingPaletteGradient:
		stops := make([]lightingsettings.GradientStop, 0, len(profile.Gradients))
		for index := 0; index < len(profile.Gradients); index++ {
			value, found := profile.Gradients[index]
			if !found {
				return lightingsettings.EffectSettings{}, errors.New("Gradient stops must use contiguous indexes")
			}
			stops = append(stops, lightingsettings.GradientStop{
				Position:  value.Position,
				Color:     color(value),
				Intensity: value.Brightness,
			})
		}
		settings.Gradient = &lightingsettings.GradientSettings{Stops: stops}
	default:
		return lightingsettings.EffectSettings{}, errors.New("unsupported OpenRGB effect palette")
	}
	if err := lightingsettings.Validate(settings); err != nil {
		return lightingsettings.EffectSettings{}, err
	}
	return settings, nil
}

func newCanonicalStaticTestDevice(brightness uint8) *Device {
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
	return device
}

func setCanonicalStaticColor(t *testing.T, device *Device, color rgb.Color) {
	t.Helper()
	settings := lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      defaultDeviceLightingEffect,
		SingleColor: &lightingsettings.SingleColorSettings{
			Color: lightingsettings.Color{Red: color.Red, Green: color.Green, Blue: color.Blue},
		},
	}
	if err := device.lightingEffects.Set(device.Serial, defaultDeviceLightingEffect, settings); err != nil {
		t.Fatalf("seed canonical Static settings: %v", err)
	}
}

func canonicalStaticFrame(t *testing.T, device *Device, brightness uint8) []byte {
	t.Helper()
	frame, err := device.buildStaticFrame(brightness)
	if err != nil {
		t.Fatalf("build canonical Static frame: %v", err)
	}
	return frame
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
		if calls.colors != 0 || calls.frames != 1 {
			t.Fatalf("output calls = colors %d, frames %d, want 0, 1", calls.colors, calls.frames)
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
		if calls.colors != 0 || calls.frames != 1 {
			t.Fatalf("output calls = colors %d, frames %d, want 0, 1", calls.colors, calls.frames)
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

	if calls.colors != 1 || calls.frames != 1 {
		t.Fatalf("output calls = colors %d, frames %d, want 1, 1", calls.colors, calls.frames)
	}
	if len(calls.colorValues) != 1 || !bytes.Equal(calls.colorValues[0], []byte{0, 0, 0}) ||
		len(calls.frameValues) != 1 || !bytes.Equal(calls.frameValues[0], []byte{0, 102, 102}) {
		t.Fatalf("output = colors %v, frames %v; want off then canonical Static", calls.colorValues, calls.frameValues)
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

func TestOpenRGBCanonicalStaticFrame(t *testing.T) {
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
			name:       "black",
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
			device := newCanonicalStaticTestDevice(test.brightness)
			setCanonicalStaticColor(t, device, test.color)
			profileDir := t.TempDir()
			device.DeviceProfile.Path = filepath.Join(profileDir, lightingDeviceTestSerial+".json")

			profileBefore := cloneDeviceProfile(device.DeviceProfile)
			configBefore := cloneDeviceConfig(device.Config)
			lastColorBefore := append([]byte(nil), device.lastColor...)

			frame := canonicalStaticFrame(t, device, test.brightness)
			if !bytes.Equal(frame, test.expected) {
				t.Fatalf("canonical Static frame = %v, want %v", frame, test.expected)
			}
			if !reflect.DeepEqual(device.DeviceProfile, profileBefore) {
				t.Fatal("building the canonical Static frame changed the device profile")
			}
			if !reflect.DeepEqual(device.Config, configBefore) {
				t.Fatal("building the canonical Static frame changed the controller config")
			}
			if !bytes.Equal(device.lastColor, lastColorBefore) {
				t.Fatal("building the canonical Static frame changed lastColor")
			}
			entries, err := os.ReadDir(profileDir)
			if err != nil {
				t.Fatalf("read profile directory: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("building the canonical Static frame wrote %d profile files", len(entries))
			}
		})
	}
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
	settings, err := canonicalTestSettingsFromRGBProfile(effect, openRGBTemperatureMiddleColorProfile(effect))
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
			profile := device.resolvedRendererProfile(test.effect)
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

func TestOpenRGBTemperatureMiddleColorNonTemperatureAndLegacyRegression(t *testing.T) {
	t.Run("Static output", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newCanonicalStaticTestDevice(100)
		expected := canonicalStaticFrame(t, device, device.brightness)

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
		profile := device.resolvedRendererProfile("cpu-temperature")
		if profile != nil {
			t.Fatalf("legacy target-local RGB profile was consulted: %#v", profile)
		}
	})
}
