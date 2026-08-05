package openrgbimport

import (
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"errors"
	"reflect"
	"testing"
)

func newLightingColorMutationDevice(effect string, initialSpeed float64) *Device {
	device := newLightingMutationDevice()
	device.effect = effect
	device.RGBModes = []string{"static", "rotator", "rainbow"}
	device.DeviceProfile.RGBProfile = effect
	device.Rgb = &rgb.RGB{
		Device: device.Product,
		Profiles: map[string]rgb.Profile{
			"static":  {ProfileName: "Static"},
			"rotator": {ProfileName: "Rotator", Speed: 4.25},
			"rainbow": {ProfileName: "Rainbow", Speed: 6},
		},
	}
	if initialSpeed != 0 {
		profile := device.Rgb.Profiles[effect]
		profile.Speed = initialSpeed
		device.Rgb.Profiles[effect] = profile
	}
	settings, err := lightingSettingsFromRGBProfile(effect, device.Rgb.Profiles[effect])
	if err == nil {
		if err = device.lightingEffects.Set(device.Serial, effect, settings); err != nil {
			panic(err)
		}
	}
	return device
}

func TestSetEffectColor(t *testing.T) {
	installLightingDeviceTestSeams(t)
	installLightingSpeedPersistenceSeams(t)

	color := lightingsettings.Color{Red: 12, Green: 34, Blue: 56}

	t.Run("first genuine edit materializes one complete customization", func(t *testing.T) {
		device := newLightingColorMutationDevice("static", 0)
		device.brightness = 75

		err := device.SetEffectColor(lightingDeviceTestSerial, "static", color)
		if err != nil {
			t.Fatalf("SetEffectColor: %v", err)
		}

		stopLightingSpeedWorker(device)

		profile := device.GetRgbProfile("static")
		if profile == nil || profile.StartColor.Red != 12 || profile.StartColor.Green != 34 || profile.StartColor.Blue != 56 {
			t.Fatalf("profile not materialized correctly: %+v", profile)
		}

		snapshot, ok := device.LightingSnapshot()
		if !ok || !snapshot.Customized || snapshot.SingleColorHex != "#0c2238" || snapshot.Brightness != 75 {
			t.Fatalf("snapshot mismatch: %+v", snapshot)
		}
	})

	t.Run("Rotator color editing preserves resolved Speed", func(t *testing.T) {
		device := newLightingColorMutationDevice("rotator", 4.25)
		err := device.SetEffectColor(lightingDeviceTestSerial, "rotator", color)
		if err != nil {
			t.Fatalf("SetEffectColor: %v", err)
		}
		stopLightingSpeedWorker(device)

		profile := device.GetRgbProfile("rotator")
		if profile == nil || profile.Speed != 4.25 {
			t.Fatalf("Rotator speed mutated: %+v", profile)
		}
	})

	t.Run("stale expected effect is rejected", func(t *testing.T) {
		device := newLightingColorMutationDevice("static", 0)
		err := device.SetEffectColor(lightingDeviceTestSerial, "rotator", color)
		if err == nil {
			t.Fatal("expected error for stale effect")
		}
	})

	t.Run("unsupported palette kind is rejected", func(t *testing.T) {
		device := newLightingColorMutationDevice("rainbow", 0)
		err := device.SetEffectColor(lightingDeviceTestSerial, "rainbow", color)
		if err == nil {
			t.Fatal("expected error for unsupported palette kind")
		}
	})

	t.Run("cluster-controlled devices are rejected", func(t *testing.T) {
		device := newLightingColorMutationDevice("static", 0)
		device.DeviceProfile.RGBCluster = true
		err := device.SetEffectColor(lightingDeviceTestSerial, "static", color)
		if err == nil {
			t.Fatal("expected error for cluster-controlled device")
		}
	})

	t.Run("persistence failure changes neither canonical store state nor output", func(t *testing.T) {
		device := newLightingColorMutationDevice("static", 0)
		beforeProfile := *device.GetRgbProfile("static")

		confirmed := device.lightingEffects
		device.lightingEffects = failingLightingEffectAccess{
			deviceLightingEffectAccess: confirmed,
			err:                        errors.New("persistence failure"),
		}

		err := device.SetEffectColor(lightingDeviceTestSerial, "static", color)
		if err == nil {
			t.Fatal("expected persistence error")
		}

		if !reflect.DeepEqual(*device.GetRgbProfile("static"), beforeProfile) {
			t.Fatal("canonical store changed despite persistence failure")
		}
		if device.running {
			t.Fatal("worker replacement occurred despite persistence failure")
		}
	})
}

func TestResetEffectCustomization(t *testing.T) {
	installLightingDeviceTestSeams(t)
	installLightingSpeedPersistenceSeams(t)

	t.Run("no existing customization is a successful no-op", func(t *testing.T) {
		device := newLightingColorMutationDevice("rotator", 4.25)
		device.lightingEffects.Delete(lightingDeviceTestSerial, "rotator")
		device.running = true // simulate an active animated effect

		err := device.ResetEffectCustomization(lightingDeviceTestSerial, "rotator")
		if err != nil {
			t.Fatalf("ResetEffectCustomization: %v", err)
		}
		if !device.running {
			t.Fatal("worker replacement occurred for no-op")
		}
	})

	t.Run("deletes only the selected device/effect customization", func(t *testing.T) {
		device := newLightingColorMutationDevice("static", 0)
		device.brightness = 75
		device.SetEffectColor(lightingDeviceTestSerial, "static", lightingsettings.Color{Red: 100})
		stopLightingSpeedWorker(device)

		err := device.ResetEffectCustomization(lightingDeviceTestSerial, "static")
		if err != nil {
			t.Fatalf("ResetEffectCustomization: %v", err)
		}
		stopLightingSpeedWorker(device)

		snapshot, ok := device.LightingSnapshot()
		if !ok || snapshot.Customized || snapshot.Brightness != 75 {
			t.Fatalf("snapshot not reset correctly: %+v", snapshot)
		}
	})

	t.Run("stale expected effect is rejected", func(t *testing.T) {
		device := newLightingColorMutationDevice("static", 0)
		device.SetEffectColor(lightingDeviceTestSerial, "static", lightingsettings.Color{Red: 100})
		stopLightingSpeedWorker(device)

		err := device.ResetEffectCustomization(lightingDeviceTestSerial, "rotator")
		if err == nil {
			t.Fatal("expected error for stale effect")
		}
	})

	t.Run("cluster-controlled devices are rejected", func(t *testing.T) {
		device := newLightingColorMutationDevice("static", 0)
		device.DeviceProfile.RGBCluster = true
		err := device.ResetEffectCustomization(lightingDeviceTestSerial, "static")
		if err == nil {
			t.Fatal("expected error for cluster-controlled device")
		}
	})
}
