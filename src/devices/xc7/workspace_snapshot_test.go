package xc7

import (
	"LumenForge/src/devices/lcd"
	"testing"
)

func TestDeviceProfileSnapshotRequiresCompleteActiveProfileState(t *testing.T) {
	device := &Device{Serial: "xc7", DeviceProfile: &DeviceProfile{}, UserProfiles: map[string]*DeviceProfile{"Gaming": {}, "Default": {Active: true}}}
	snapshot, ok := device.DeviceProfileSnapshot()
	if !ok || snapshot.Scope != "device" || snapshot.ActiveProfile != "Default" || len(snapshot.Profiles) != 2 || snapshot.Profiles[0] != "Default" || !snapshot.CanSwitch || !snapshot.CanSave || !snapshot.CanDelete {
		t.Fatalf("snapshot=%#v ok=%t", snapshot, ok)
	}
	for _, profiles := range []map[string]*DeviceProfile{{"Default": {}}, {"Default": {Active: true}, "Gaming": {Active: true}}, {"": {Active: true}}, {"Default": nil}} {
		device.UserProfiles = profiles
		if _, ok := device.DeviceProfileSnapshot(); ok {
			t.Fatalf("accepted malformed profiles=%#v", profiles)
		}
	}
}

func TestDisplaySnapshotUsesOnlyXC7LCDCapabilities(t *testing.T) {
	previousImages, previousImage := xc7DisplayImages, xc7DisplayImage
	t.Cleanup(func() { xc7DisplayImages, xc7DisplayImage = previousImages, previousImage })
	images := []lcd.ImageData{{Name: "zeta"}, {Name: "alpha"}}
	xc7DisplayImages = func() []lcd.ImageData { return images }
	xc7DisplayImage = func(name string) *lcd.ImageData {
		for index := range images {
			if images[index].Name == name {
				return &images[index]
			}
		}
		return nil
	}
	device := &Device{Serial: "xc7", HasLCD: true, DeviceProfile: &DeviceProfile{LCDMode: lcd.DisplayImage, LCDRotation: 2, LCDImage: "zeta"}, LCDModes: map[int]string{10: "Image / GIF", 0: "Liquid Temperature"}, LCDRotations: map[int]string{2: "180 degrees", 0: "default"}}
	snapshot, ok := device.DisplaySnapshot()
	if !ok || !snapshot.ImageMode || snapshot.ImageModeID != int(lcd.DisplayImage) || len(snapshot.BrightnessLevels) != 0 || len(snapshot.Modes) != 2 || snapshot.Modes[0].ID != 0 || snapshot.Rotations[0].ID != 0 || len(snapshot.Images) != 2 || snapshot.Images[0].Name != "alpha" || !snapshot.Images[1].Selected {
		t.Fatalf("snapshot=%#v ok=%t", snapshot, ok)
	}
	device.HasLCD = false
	if _, ok := device.DisplaySnapshot(); ok {
		t.Fatal("non-LCD device exposed display")
	}
	device.HasLCD = true
	device.DeviceProfile.LCDMode = 9
	if _, ok := device.DisplaySnapshot(); ok {
		t.Fatal("unknown mode exposed display")
	}
	device.DeviceProfile.LCDMode = lcd.DisplayImage
	device.DeviceProfile.LCDRotation = 9
	if _, ok := device.DisplaySnapshot(); ok {
		t.Fatal("unknown rotation exposed display")
	}
	device.DeviceProfile.LCDRotation = 2
	device.DeviceProfile.LCDImage = "missing"
	if _, ok := device.DisplaySnapshot(); ok {
		t.Fatal("unknown image exposed display")
	}
	device.DeviceProfile.LCDMode = 0
	if _, ok := device.DisplaySnapshot(); !ok {
		t.Fatal("non-image mode incorrectly required an image")
	}
}

func TestTelemetrySnapshotOnlyExposesLiquidTemperature(t *testing.T) {
	device := &Device{Serial: "xc7", Temperature: 31.8, TemperatureString: "31.8°C", CpuTemp: 55, GpuTemp: 62}
	snapshot, ok := device.TelemetrySnapshot()
	if !ok || len(snapshot.Rows) != 1 || snapshot.Rows[0].Label != "Liquid Temperature" || snapshot.Rows[0].Value != "31.8°C" {
		t.Fatalf("snapshot=%#v ok=%t", snapshot, ok)
	}
	device.TemperatureString = ""
	if _, ok := device.TelemetrySnapshot(); ok {
		t.Fatal("missing temperature string exposed telemetry")
	}
}
