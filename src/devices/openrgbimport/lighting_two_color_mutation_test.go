package openrgbimport

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"LumenForge/src/lightingsettings"
)

func newLightingTwoColorMutationDevice(effect string) *Device {
	device := newLightingMutationDevice()
	device.effect = effect
	device.DeviceProfile.RGBProfile = effect
	device.RGBModes = []string{"wave", "arc", "static", "rainbow", "cpu-temperature", "gradient", "off"}
	return device
}

func TestSetEffectTwoColor(t *testing.T) {
	start := lightingsettings.Color{Red: 12, Green: 34, Blue: 56}
	end := lightingsettings.Color{Red: 78, Green: 90, Blue: 123}

	t.Run("materializes complete settings and reapplies output", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newLightingTwoColorMutationDevice("wave")
		device.brightness = 73
		before, err := device.resolveLightingSettings("wave")
		if err != nil || before.Customized || before.Settings.Speed == nil {
			t.Fatalf("initial Wave resolution = %#v, %v", before, err)
		}

		if err = device.SetEffectTwoColor(lightingDeviceTestSerial, "wave", start, end); err != nil {
			t.Fatalf("SetEffectTwoColor: %v", err)
		}
		runnerSpeed, runnerStart, runnerEnd := lightingSpeedRunnerState(t, device)
		device.mu.Lock()
		running := device.running
		stop := device.stopChan
		done := device.doneChan
		device.mu.Unlock()
		stopLightingSpeedWorker(device)

		after, resolveErr := device.resolveLightingSettings("wave")
		if resolveErr != nil || !after.Customized || after.Settings.TwoColor == nil {
			t.Fatalf("customized Wave resolution = %#v, %v", after, resolveErr)
		}
		if after.Settings.TwoColor.Start != start || after.Settings.TwoColor.End != end {
			t.Fatalf("stored pair = %#v, want Start %#v End %#v", after.Settings.TwoColor, start, end)
		}
		if after.Settings.Speed == nil || *after.Settings.Speed != *before.Settings.Speed || runnerSpeed != *before.Settings.Speed {
			t.Fatalf("preserved Speed = settings %#v runner %v, want %v", after.Settings.Speed, runnerSpeed, *before.Settings.Speed)
		}
		if runnerStart.Red != start.Red || runnerStart.Green != start.Green || runnerStart.Blue != start.Blue ||
			runnerEnd.Red != end.Red || runnerEnd.Green != end.Green || runnerEnd.Blue != end.Blue {
			t.Fatalf("runner colors = Start %#v End %#v", runnerStart, runnerEnd)
		}
		if device.GetEffect() != "wave" || device.GetBrightness() != 73 {
			t.Fatalf("target state = effect %q brightness %d", device.GetEffect(), device.GetBrightness())
		}
		if !running || stop == nil || done == nil || calls.persistentFrames == 0 {
			t.Fatalf("replacement worker = running %t stop %v done %v output %d", running, stop != nil, done != nil, calls.persistentFrames)
		}

		otherEffect, otherErr := device.resolveLightingSettings("arc")
		if otherErr != nil || otherEffect.Customized {
			t.Fatalf("Wave edit affected Arc = %#v, %v", otherEffect, otherErr)
		}
		otherDevice := newLightingTwoColorMutationDevice("wave")
		otherDevice.Serial = "openrgb-lighting-other-device"
		otherDevice.lightingEffects = device.lightingEffects
		otherDevice.lightingResolver = device.lightingResolver
		otherResolution, otherErr := otherDevice.resolveLightingSettings("wave")
		if otherErr != nil || otherResolution.Customized {
			t.Fatalf("Wave edit affected another device = %#v, %v", otherResolution, otherErr)
		}
	})

	t.Run("rejects stale unsupported owned and inactive targets", func(t *testing.T) {
		tests := []struct {
			name   string
			device func() *Device
			serial string
			effect string
			match  string
		}{
			{name: "nil device", device: func() *Device { return nil }, serial: lightingDeviceTestSerial, effect: "wave", match: "not available"},
			{name: "wrong serial", device: func() *Device { return newLightingTwoColorMutationDevice("wave") }, serial: "wrong", effect: "wave", match: "identity"},
			{name: "stale effect", device: func() *Device { return newLightingTwoColorMutationDevice("wave") }, serial: lightingDeviceTestSerial, effect: "arc", match: "stale"},
			{name: "generated", device: func() *Device { return newLightingTwoColorMutationDevice("rainbow") }, serial: lightingDeviceTestSerial, effect: "rainbow", match: "two-color"},
			{name: "single color", device: func() *Device { return newLightingTwoColorMutationDevice("static") }, serial: lightingDeviceTestSerial, effect: "static", match: "two-color"},
			{name: "temperature", device: func() *Device { return newLightingTwoColorMutationDevice("cpu-temperature") }, serial: lightingDeviceTestSerial, effect: "cpu-temperature", match: "two-color"},
			{name: "Gradient", device: func() *Device { return newLightingTwoColorMutationDevice("gradient") }, serial: lightingDeviceTestSerial, effect: "gradient", match: "two-color"},
			{name: "unsupported catalogue", device: func() *Device {
				device := newLightingTwoColorMutationDevice("wave")
				device.RGBModes = []string{"static"}
				return device
			}, serial: lightingDeviceTestSerial, effect: "wave", match: "unsupported"},
			{name: "cluster owned", device: func() *Device {
				device := newLightingTwoColorMutationDevice("wave")
				device.DeviceProfile.RGBCluster = true
				return device
			}, serial: lightingDeviceTestSerial, effect: "wave", match: "RGB cluster"},
			{name: "inactive profile", device: func() *Device {
				device := newLightingTwoColorMutationDevice("wave")
				device.DeviceProfile.Active = false
				return device
			}, serial: lightingDeviceTestSerial, effect: "wave", match: "active"},
			{name: "unavailable device", device: func() *Device {
				device := newLightingTwoColorMutationDevice("wave")
				device.lifecycleDetached = true
				return device
			}, serial: lightingDeviceTestSerial, effect: "wave", match: "detached"},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				installLightingDeviceTestSeams(t)
				device := test.device()
				err := device.SetEffectTwoColor(test.serial, test.effect, start, end)
				if err == nil || !strings.Contains(err.Error(), test.match) {
					t.Fatalf("SetEffectTwoColor error = %v, want text %q", err, test.match)
				}
			})
		}
	})

	t.Run("persistence failure changes neither canonical state nor output", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newLightingTwoColorMutationDevice("wave")
		before, err := device.resolveLightingSettings("wave")
		if err != nil {
			t.Fatal(err)
		}
		confirmed := device.lightingEffects
		device.lightingEffects = failingLightingEffectAccess{
			deviceLightingEffectAccess: confirmed,
			err:                        errors.New("persistence failure"),
		}

		if err = device.SetEffectTwoColor(lightingDeviceTestSerial, "wave", start, end); err == nil {
			t.Fatal("SetEffectTwoColor succeeded despite persistence failure")
		}
		after, resolveErr := device.resolveLightingSettings("wave")
		if resolveErr != nil || !reflect.DeepEqual(after, before) {
			t.Fatalf("resolution changed from %#v to %#v, %v", before, after, resolveErr)
		}
		if calls.persistentFrames != 0 || device.running {
			t.Fatalf("failed save changed output: frames %d running %t", calls.persistentFrames, device.running)
		}
	})

	t.Run("output failure retains persisted desired pair", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		calls.err = errors.New("two-color output failure")
		device := newLightingTwoColorMutationDevice("wave")

		err := device.SetEffectTwoColor(lightingDeviceTestSerial, "wave", start, end)
		if err == nil || !strings.Contains(err.Error(), "two-color output failure") {
			t.Fatalf("SetEffectTwoColor output error = %v", err)
		}
		resolution, resolveErr := device.resolveLightingSettings("wave")
		if resolveErr != nil || !resolution.Customized || resolution.Settings.TwoColor == nil ||
			resolution.Settings.TwoColor.Start != start || resolution.Settings.TwoColor.End != end {
			t.Fatalf("persisted pair after output failure = %#v, %v", resolution, resolveErr)
		}
		if calls.persistentFrames != 1 || device.running {
			t.Fatalf("output failure state = frames %d running %t", calls.persistentFrames, device.running)
		}
	})

	t.Run("replaces at most one worker", func(t *testing.T) {
		installLightingDeviceTestSeams(t)
		device := newLightingTwoColorMutationDevice("wave")
		if err := device.SetEffectTwoColor(lightingDeviceTestSerial, "wave", start, end); err != nil {
			t.Fatal(err)
		}
		device.mu.Lock()
		firstDone := device.doneChan
		device.mu.Unlock()

		secondStart := lightingsettings.Color{Red: 1, Green: 2, Blue: 3}
		if err := device.SetEffectTwoColor(lightingDeviceTestSerial, "wave", secondStart, end); err != nil {
			t.Fatal(err)
		}
		device.mu.Lock()
		secondDone := device.doneChan
		running := device.running
		device.mu.Unlock()
		select {
		case <-firstDone:
		default:
			t.Fatal("prior worker remained active after replacement")
		}
		if !running || secondDone == nil || secondDone == firstDone {
			t.Fatalf("replacement worker = running %t first %p second %p", running, firstDone, secondDone)
		}
		stopLightingSpeedWorker(device)
	})
}
