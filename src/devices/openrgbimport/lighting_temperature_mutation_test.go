package openrgbimport

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"LumenForge/src/lightingsettings"
)

func newLightingTemperatureMutationDevice(effect string) *Device {
	device := newLightingMutationDevice()
	device.effect = effect
	device.RGBModes = []string{"cpu-temperature", "gpu-temperature", "static", "wave", "rainbow", "gradient", "off"}
	return device
}

func TestSetEffectTemperature(t *testing.T) {
	low := lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Red: 1, Green: 2, Blue: 3}, Celsius: 21.5}
	middle := lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Red: 4, Green: 5, Blue: 6}, Celsius: 52.5}
	high := lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Red: 7, Green: 8, Blue: 9}, Celsius: 83.5}

	for _, effect := range []string{"cpu-temperature", "gpu-temperature"} {
		t.Run(effect+" stores complete points", func(t *testing.T) {
			installLightingDeviceTestSeams(t)
			device := newLightingTemperatureMutationDevice(effect)
			t.Cleanup(func() {
				stopLightingSpeedWorker(device)
			})
			device.brightness = 73
			if err := device.SetEffectTemperature(lightingDeviceTestSerial, effect, low, middle, high); err != nil {
				t.Fatalf("SetEffectTemperature: %v", err)
			}
			resolution, err := device.resolveLightingSettings(effect)
			if err != nil || !resolution.Customized || resolution.Settings.Temperature == nil ||
				resolution.Settings.Temperature.Low != low || resolution.Settings.Temperature.Middle != middle || resolution.Settings.Temperature.High != high {
				t.Fatalf("temperature resolution = %#v, %v", resolution, err)
			}
			if device.GetEffect() != effect || device.GetBrightness() != 73 {
				t.Fatalf("target state = effect %q brightness %d", device.GetEffect(), device.GetBrightness())
			}
			device.mu.Lock()
			runner := device.rgbRunner
			device.mu.Unlock()
			if runner == nil || runner.RGBStartColor.Temperature != low.Celsius ||
				runner.RGBMiddleColor.Temperature != middle.Celsius || runner.RGBEndColor.Temperature != high.Celsius ||
				runner.RGBStartColor.Red != low.Color.Red || runner.RGBMiddleColor.Green != middle.Color.Green || runner.RGBEndColor.Blue != high.Color.Blue {
				t.Fatalf("temperature runner = %#v", runner)
			}
		})
	}

	tests := []struct {
		name   string
		effect string
		low    lightingsettings.TemperaturePoint
		middle lightingsettings.TemperaturePoint
		high   lightingsettings.TemperaturePoint
		match  string
	}{
		{name: "single color", effect: "static", low: low, middle: middle, high: high, match: "temperature"},
		{name: "two color", effect: "wave", low: low, middle: middle, high: high, match: "temperature"},
		{name: "generated", effect: "rainbow", low: low, middle: middle, high: high, match: "temperature"},
		{name: "Gradient", effect: "gradient", low: low, middle: middle, high: high, match: "temperature"},
		{name: "Off", effect: "off", low: low, middle: middle, high: high, match: "temperature"},
		{name: "equal", effect: "cpu-temperature", low: low, middle: low, high: high, match: "strictly ordered"},
		{name: "descending", effect: "cpu-temperature", low: high, middle: middle, high: low, match: "strictly ordered"},
		{name: "NaN", effect: "cpu-temperature", low: lightingsettings.TemperaturePoint{Color: low.Color, Celsius: math.NaN()}, middle: middle, high: high, match: "finite"},
		{name: "Inf", effect: "cpu-temperature", low: low, middle: middle, high: lightingsettings.TemperaturePoint{Color: high.Color, Celsius: math.Inf(1)}, match: "finite"},
		{name: "invalid color", effect: "cpu-temperature", low: lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Red: 256}, Celsius: 20}, middle: middle, high: high, match: "between 0 and 255"},
	}
	for _, test := range tests {
		t.Run("rejects "+test.name, func(t *testing.T) {
			installLightingDeviceTestSeams(t)
			device := newLightingTemperatureMutationDevice(test.effect)
			err := device.SetEffectTemperature(lightingDeviceTestSerial, test.effect, test.low, test.middle, test.high)
			if err == nil || !strings.Contains(err.Error(), test.match) {
				t.Fatalf("SetEffectTemperature error = %v, want %q", err, test.match)
			}
		})
	}

	t.Run("rejects stale cluster inactive and unsupported targets", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(*Device)
			serial string
			effect string
		}{
			{name: "stale", serial: lightingDeviceTestSerial, effect: "gpu-temperature"},
			{name: "wrong serial", serial: "wrong", effect: "cpu-temperature"},
			{name: "cluster", serial: lightingDeviceTestSerial, effect: "cpu-temperature", mutate: func(device *Device) { device.DeviceProfile.RGBCluster = true }},
			{name: "inactive", serial: lightingDeviceTestSerial, effect: "cpu-temperature", mutate: func(device *Device) { device.DeviceProfile.Active = false }},
			{name: "unavailable", serial: lightingDeviceTestSerial, effect: "cpu-temperature", mutate: func(device *Device) { device.lifecycleDetached = true }},
			{name: "unsupported", serial: lightingDeviceTestSerial, effect: "cpu-temperature", mutate: func(device *Device) { device.RGBModes = []string{"static"} }},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				installLightingDeviceTestSeams(t)
				device := newLightingTemperatureMutationDevice("cpu-temperature")
				if test.mutate != nil {
					test.mutate(device)
				}
				if err := device.SetEffectTemperature(test.serial, test.effect, low, middle, high); err == nil {
					t.Fatal("SetEffectTemperature unexpectedly succeeded")
				}
			})
		}
	})

	t.Run("nil device is rejected", func(t *testing.T) {
		var device *Device
		if err := device.SetEffectTemperature(lightingDeviceTestSerial, "cpu-temperature", low, middle, high); err == nil {
			t.Fatal("nil SetEffectTemperature unexpectedly succeeded")
		}
	})

	t.Run("persistence failure preserves canonical state and output", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		device := newLightingTemperatureMutationDevice("cpu-temperature")
		before, err := device.resolveLightingSettings("cpu-temperature")
		if err != nil {
			t.Fatal(err)
		}
		device.lightingEffects = failingLightingEffectAccess{
			deviceLightingEffectAccess: device.lightingEffects,
			err:                        errors.New("persistence failure"),
		}
		if err = device.SetEffectTemperature(lightingDeviceTestSerial, "cpu-temperature", low, middle, high); err == nil {
			t.Fatal("SetEffectTemperature succeeded despite persistence failure")
		}
		after, resolveErr := device.resolveLightingSettings("cpu-temperature")
		if resolveErr != nil || !reflect.DeepEqual(after, before) || calls.persistentFrames != 0 || device.running {
			t.Fatalf("failed persistence changed state: before %#v after %#v error %v frames %d running %t", before, after, resolveErr, calls.persistentFrames, device.running)
		}
	})

	t.Run("output failure retains persisted desired points", func(t *testing.T) {
		_, calls := installLightingDeviceTestSeams(t)
		calls.err = errors.New("temperature output failure")
		device := newLightingTemperatureMutationDevice("gpu-temperature")
		err := device.SetEffectTemperature(lightingDeviceTestSerial, "gpu-temperature", low, middle, high)
		if err == nil || !strings.Contains(err.Error(), "temperature output failure") {
			t.Fatalf("SetEffectTemperature output error = %v", err)
		}
		resolution, resolveErr := device.resolveLightingSettings("gpu-temperature")
		if resolveErr != nil || !resolution.Customized || resolution.Settings.Temperature == nil ||
			resolution.Settings.Temperature.Low != low || resolution.Settings.Temperature.Middle != middle || resolution.Settings.Temperature.High != high {
			t.Fatalf("persisted points after output failure = %#v, %v", resolution, resolveErr)
		}
		if calls.persistentFrames != 1 || device.running {
			t.Fatalf("output failure state = frames %d running %t", calls.persistentFrames, device.running)
		}
	})
}
