package openrgbimport

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"LumenForge/src/lightingsettings"
)

func newLightingGradientMutationDevice(effect string) *Device {
	device := newLightingMutationDevice()
	device.effect = effect
	device.DeviceProfile.RGBProfile = effect
	device.RGBModes = []string{"gradient", "static", "wave", "cpu-temperature", "rainbow", "off"}
	return device
}

func gradientMutationStops() []lightingsettings.GradientStop {
	return []lightingsettings.GradientStop{
		{Position: 0, Color: lightingsettings.Color{Red: 1, Green: 2, Blue: 3}, Intensity: 0},
		{Position: 0.4, Color: lightingsettings.Color{Red: 4, Green: 5, Blue: 6}, Intensity: 0.5},
		{Position: 0.4, Color: lightingsettings.Color{Red: 7, Green: 8, Blue: 9}, Intensity: 0.75},
		{Position: 1, Color: lightingsettings.Color{Red: 10, Green: 11, Blue: 12}, Intensity: 1},
	}
}

func TestSetEffectGradient(t *testing.T) {
	t.Run("stores a defensive complete Gradient and reapplies output", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newLightingGradientMutationDevice("gradient")
		t.Cleanup(func() { stopLightingSpeedWorker(device) })
		device.brightness = 73
		before, err := device.resolveLightingSettings("gradient")
		if err != nil || before.Settings.Speed == nil {
			t.Fatalf("initial Gradient = %#v, %v", before, err)
		}
		stops := gradientMutationStops()
		want := append([]lightingsettings.GradientStop(nil), stops...)
		if err = device.SetEffectGradient(lightingDeviceTestSerial, "gradient", stops); err != nil {
			t.Fatalf("SetEffectGradient: %v", err)
		}
		stops[0].Color.Red = 255
		after, resolveErr := device.resolveLightingSettings("gradient")
		if resolveErr != nil || !after.Customized || after.Settings.Gradient == nil ||
			!reflect.DeepEqual(after.Settings.Gradient.Stops, want) || after.Settings.Speed == nil || *after.Settings.Speed != *before.Settings.Speed {
			t.Fatalf("stored Gradient = %#v, %v", after, resolveErr)
		}
		if device.GetEffect() != "gradient" || device.GetBrightness() != 73 {
			t.Fatalf("target state = effect %q brightness %d", device.GetEffect(), device.GetBrightness())
		}
		device.mu.Lock()
		runner := device.rgbRunner
		running := device.running
		device.mu.Unlock()
		profile := rgbProfileFromLightingSettings(after.Settings)
		if runner == nil || len(profile.Gradients) != len(want) || !running || calls.persistentFrames == 0 {
			t.Fatalf("Gradient runner = %#v running %t frames %d", runner, running, calls.persistentFrames)
		}
		for index, stop := range want {
			color := profile.Gradients[index]
			if color.Position != stop.Position || color.Red != stop.Color.Red || color.Green != stop.Color.Green ||
				color.Blue != stop.Color.Blue || color.Brightness != stop.Intensity {
				t.Fatalf("runner stop %d = %#v, want %#v", index, color, stop)
			}
		}
		otherEffect, otherErr := device.resolveLightingSettings("static")
		if otherErr != nil || otherEffect.Customized {
			t.Fatalf("Gradient edit affected Static = %#v, %v", otherEffect, otherErr)
		}
		otherDevice := newLightingGradientMutationDevice("gradient")
		otherDevice.Serial = "openrgb-gradient-other"
		otherDevice.lightingEffects = device.lightingEffects
		otherDevice.lightingResolver = device.lightingResolver
		otherResolution, otherErr := otherDevice.resolveLightingSettings("gradient")
		if otherErr != nil || otherResolution.Customized {
			t.Fatalf("Gradient edit affected another device = %#v, %v", otherResolution, otherErr)
		}
	})

	t.Run("rejects invalid target state and non-Gradient palettes", func(t *testing.T) {
		cases := []struct {
			name   string
			effect string
			serial string
			mutate func(*Device)
		}{
			{name: "wrong serial", effect: "gradient", serial: "wrong"},
			{name: "stale effect", effect: "static", serial: lightingDeviceTestSerial},
			{name: "Static", effect: "static", serial: lightingDeviceTestSerial, mutate: func(device *Device) { device.effect = "static" }},
			{name: "two color", effect: "wave", serial: lightingDeviceTestSerial, mutate: func(device *Device) { device.effect = "wave" }},
			{name: "temperature", effect: "cpu-temperature", serial: lightingDeviceTestSerial, mutate: func(device *Device) { device.effect = "cpu-temperature" }},
			{name: "generated", effect: "rainbow", serial: lightingDeviceTestSerial, mutate: func(device *Device) { device.effect = "rainbow" }},
			{name: "Off", effect: "off", serial: lightingDeviceTestSerial, mutate: func(device *Device) { device.effect = "off" }},
			{name: "unsupported", effect: "gradient", serial: lightingDeviceTestSerial, mutate: func(device *Device) { device.RGBModes = []string{"static"} }},
			{name: "cluster", effect: "gradient", serial: lightingDeviceTestSerial, mutate: func(device *Device) { device.DeviceProfile.RGBCluster = true }},
			{name: "inactive", effect: "gradient", serial: lightingDeviceTestSerial, mutate: func(device *Device) { device.DeviceProfile.Active = false }},
			{name: "unavailable", effect: "gradient", serial: lightingDeviceTestSerial, mutate: func(device *Device) { device.lifecycleDetached = true }},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				installLightingDeviceTestSeams(t)
				device := newLightingGradientMutationDevice("gradient")
				t.Cleanup(func() { stopLightingSpeedWorker(device) })
				if test.mutate != nil {
					test.mutate(device)
				}
				if err := device.SetEffectGradient(test.serial, test.effect, gradientMutationStops()); err == nil {
					t.Fatal("SetEffectGradient unexpectedly succeeded")
				}
			})
		}
		var nilDevice *Device
		if err := nilDevice.SetEffectGradient(lightingDeviceTestSerial, "gradient", gradientMutationStops()); err == nil {
			t.Fatal("nil SetEffectGradient unexpectedly succeeded")
		}
	})

	t.Run("canonical validation rejects malformed complete records", func(t *testing.T) {
		tooMany := make([]lightingsettings.GradientStop, 1025)
		for index := range tooMany {
			tooMany[index] = lightingsettings.GradientStop{Position: float64(index) / 1024, Color: lightingsettings.Color{}, Intensity: 1}
		}
		valid := gradientMutationStops()
		cases := []struct {
			name  string
			stops []lightingsettings.GradientStop
		}{
			{name: "too few", stops: valid[:1]},
			{name: "too many", stops: tooMany},
			{name: "NaN position", stops: func() []lightingsettings.GradientStop {
				value := append([]lightingsettings.GradientStop(nil), valid...)
				value[0].Position = math.NaN()
				return value
			}()},
			{name: "infinite intensity", stops: func() []lightingsettings.GradientStop {
				value := append([]lightingsettings.GradientStop(nil), valid...)
				value[1].Intensity = math.Inf(1)
				return value
			}()},
			{name: "position below range", stops: func() []lightingsettings.GradientStop {
				value := append([]lightingsettings.GradientStop(nil), valid...)
				value[0].Position = -0.1
				return value
			}()},
			{name: "position above range", stops: func() []lightingsettings.GradientStop {
				value := append([]lightingsettings.GradientStop(nil), valid...)
				value[len(value)-1].Position = 1.1
				return value
			}()},
			{name: "intensity below range", stops: func() []lightingsettings.GradientStop {
				value := append([]lightingsettings.GradientStop(nil), valid...)
				value[0].Intensity = -0.1
				return value
			}()},
			{name: "intensity above range", stops: func() []lightingsettings.GradientStop {
				value := append([]lightingsettings.GradientStop(nil), valid...)
				value[0].Intensity = 1.1
				return value
			}()},
			{name: "decreasing", stops: func() []lightingsettings.GradientStop {
				value := append([]lightingsettings.GradientStop(nil), valid...)
				value[1].Position = 0.8
				return value
			}()},
			{name: "invalid color", stops: func() []lightingsettings.GradientStop {
				value := append([]lightingsettings.GradientStop(nil), valid...)
				value[0].Color.Red = 256
				return value
			}()},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				installLightingDeviceTestSeams(t)
				device := newLightingGradientMutationDevice("gradient")
				t.Cleanup(func() { stopLightingSpeedWorker(device) })
				if err := device.SetEffectGradient(lightingDeviceTestSerial, "gradient", test.stops); err == nil {
					t.Fatal("SetEffectGradient unexpectedly accepted invalid stops")
				}
			})
		}
		installLightingDeviceTestSeams(t)
		device := newLightingGradientMutationDevice("gradient")
		t.Cleanup(func() { stopLightingSpeedWorker(device) })
		if err := device.SetEffectGradient(lightingDeviceTestSerial, "gradient", valid); err != nil {
			t.Fatalf("equal adjacent positions were rejected: %v", err)
		}
	})

	t.Run("persistence and output failure semantics are preserved", func(t *testing.T) {
		t.Run("persistence", func(t *testing.T) {
			_, calls := installLightingDeviceTestSeams(t)
			device := newLightingGradientMutationDevice("gradient")
			t.Cleanup(func() { stopLightingSpeedWorker(device) })
			before, err := device.resolveLightingSettings("gradient")
			if err != nil {
				t.Fatal(err)
			}
			device.lightingEffects = failingLightingEffectAccess{deviceLightingEffectAccess: device.lightingEffects, err: errors.New("persistence failure")}
			if err = device.SetEffectGradient(lightingDeviceTestSerial, "gradient", gradientMutationStops()); err == nil {
				t.Fatal("SetEffectGradient succeeded despite persistence failure")
			}
			after, resolveErr := device.resolveLightingSettings("gradient")
			device.mu.Lock()
			running := device.running
			device.mu.Unlock()
			if resolveErr != nil || !reflect.DeepEqual(before, after) || calls.persistentFrames != 0 || running {
				t.Fatalf("persistence failure changed state: %#v %#v %v frames %d running %t", before, after, resolveErr, calls.persistentFrames, running)
			}
		})
		t.Run("output", func(t *testing.T) {
			_, calls := installLightingDeviceTestSeams(t)
			calls.err = errors.New("Gradient output failure")
			device := newLightingGradientMutationDevice("gradient")
			t.Cleanup(func() { stopLightingSpeedWorker(device) })
			want := gradientMutationStops()
			err := device.SetEffectGradient(lightingDeviceTestSerial, "gradient", want)
			if err == nil || !strings.Contains(err.Error(), "Gradient output failure") {
				t.Fatalf("output error = %v", err)
			}
			resolution, resolveErr := device.resolveLightingSettings("gradient")
			if resolveErr != nil || !resolution.Customized || resolution.Settings.Gradient == nil || !reflect.DeepEqual(resolution.Settings.Gradient.Stops, want) {
				t.Fatalf("persisted Gradient after output failure = %#v, %v", resolution, resolveErr)
			}
		})
	})
}
