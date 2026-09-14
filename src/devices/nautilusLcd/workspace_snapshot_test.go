package nautilusLcd

import (
	"LumenForge/src/devices/lcd"
	"testing"
)

func TestDeviceProfileSnapshotRequiresCompleteActiveProfileState(t *testing.T) {
	device := &Device{Serial: "nautilus", DeviceProfile: &DeviceProfile{}, UserProfiles: map[string]*DeviceProfile{"Gaming": {}, "Default": {Active: true}}}
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

func TestDisplaySnapshotUsesOnlyNautilusLCDCapabilities(t *testing.T) {
	previousImages, previousImage := nautilusDisplayImages, nautilusDisplayImage
	t.Cleanup(func() { nautilusDisplayImages, nautilusDisplayImage = previousImages, previousImage })
	images := []lcd.ImageData{{Name: "zeta"}, {Name: "alpha"}}
	nautilusDisplayImages = func() []lcd.ImageData { return images }
	nautilusDisplayImage = func(name string) *lcd.ImageData {
		for index := range images {
			if images[index].Name == name {
				return &images[index]
			}
		}
		return nil
	}
	device := &Device{Serial: "nautilus", HasLCD: true, DeviceProfile: &DeviceProfile{LCDMode: lcd.DisplayImage, LCDRotation: 2, LCDImage: "zeta"}, LCDModes: map[int]string{10: "Image / GIF", 2: "CPU Temperature"}, LCDRotations: map[int]string{2: "180 degrees", 0: "default"}}
	snapshot, ok := device.DisplaySnapshot()
	if !ok || !snapshot.ImageMode || snapshot.ImageModeID != int(lcd.DisplayImage) || len(snapshot.BrightnessLevels) != 0 || len(snapshot.Modes) != 2 || snapshot.Modes[0].ID != 2 || snapshot.Rotations[0].ID != 0 || len(snapshot.Images) != 2 || snapshot.Images[0].Name != "alpha" || !snapshot.Images[1].Selected {
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
	device.DeviceProfile.LCDMode = 2
	if _, ok := device.DisplaySnapshot(); !ok {
		t.Fatal("non-image mode incorrectly required an image")
	}
}
