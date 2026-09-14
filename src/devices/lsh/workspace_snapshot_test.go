package lsh

import (
	"LumenForge/src/devices/lcd"
	"LumenForge/src/temperatures"
	"reflect"
	"testing"
)

func TestWorkspaceSnapshotsPreserveLINKCapabilities(t *testing.T) {
	previousProfiles := lshCoolingTemperatureProfiles
	lshCoolingTemperatureProfiles = func() map[string]temperatures.TemperatureProfileData {
		return map[string]temperatures.TemperatureProfileData{"Balanced": {}, "hidden": {Hidden: true}}
	}
	t.Cleanup(func() { lshCoolingTemperatureProfiles = previousProfiles })
	device := &Device{Serial: "link-hub", HasLCD: true, LCDModes: map[int]string{0: "Liquid Temperature", 10: "Image / GIF"}, LCDRotations: map[int]string{0: "default", 1: "90 degrees"}, LCDBrightnessLevels: map[int]string{0: "Off", 100: "100 %"}, DeviceProfile: &DeviceProfile{LCDModes: map[int]uint8{4: 0, 7: 10}, LCDRotations: map[int]uint8{4: 0, 7: 1}, LCDBrightness: map[int]uint8{4: 100, 7: 0}, LCDImages: map[int]string{7: "loop"}}, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}, "Quiet": {}}, Devices: map[int]*Devices{1: {ChannelId: 1, Name: "RX120", Label: "Front intake", Rpm: 1100, Profile: "Balanced", HasSpeed: true}, 4: {ChannelId: 4, Name: "H150i", Label: "CPU cooler", Rpm: 2450, Temperature: 31.8, TemperatureString: "31.8°C", Profile: "Balanced", HasSpeed: true, HasTemps: true, ContainsPump: true, AIO: true, LCDSerial: "lcd-a"}, 7: {ChannelId: 7, Name: "XD5", Label: "Reservoir", Rpm: 1800, Profile: "Balanced", HasSpeed: true, ContainsPump: true, LCDSerial: "lcd-b"}, 8: {ChannelId: 8, Name: "VRM fan", Rpm: 900, Profile: "Balanced", HasSpeed: true, IsVrmCooler: true}}}
	profiles, ok := device.DeviceProfileSnapshot()
	if !ok || !profiles.CanSwitch || !profiles.CanSave || !profiles.CanDelete || profiles.ActiveProfile != "Default" || !reflect.DeepEqual(profiles.Profiles, []string{"Default", "Quiet"}) {
		t.Fatalf("profiles = %#v, ok=%t", profiles, ok)
	}
	cooling, ok := device.CoolingSnapshot()
	if !ok || len(cooling.Channels) != 4 || cooling.Channels[1].ID != 4 || !cooling.Channels[1].ContainsPump || cooling.Channels[1].Temperature != "31.8°C" || cooling.Channels[1].Celsius == nil || *cooling.Channels[1].Celsius != 31.8 || cooling.Channels[3].ID != 8 {
		t.Fatalf("cooling = %#v, ok=%t", cooling, ok)
	}
	if len(cooling.ProfileOptions) != 1 || cooling.ProfileOptions[0].ID != "Balanced" {
		t.Fatalf("profile options = %#v", cooling.ProfileOptions)
	}
	previousImages, previousImage := lshDisplayImages, lshDisplayImage
	lshDisplayImages = func() []lcd.ImageData { return []lcd.ImageData{{Name: "loop"}} }
	lshDisplayImage = func(name string) *lcd.ImageData {
		if name == "loop" {
			return &lcd.ImageData{Name: name}
		}
		return nil
	}
	t.Cleanup(func() { lshDisplayImages, lshDisplayImage = previousImages, previousImage })
	display, ok := device.DisplaySnapshot()
	if !ok || len(display.Displays) != 2 || display.Displays[0].ChannelID != 4 || display.Displays[1].ChannelID != 7 || !display.Displays[1].ImageMode || display.Displays[1].SelectedImage != "loop" || len(display.Displays[1].BrightnessLevels) != 2 {
		t.Fatalf("display = %#v, ok=%t", display, ok)
	}
}

func TestDisplaySnapshotFailsClosedForIncompleteConnectedLCDState(t *testing.T) {
	device := &Device{Serial: "link-hub", HasLCD: true, LCDModes: map[int]string{0: "Liquid Temperature"}, LCDRotations: map[int]string{0: "default"}, LCDBrightnessLevels: map[int]string{100: "100 %"}, DeviceProfile: &DeviceProfile{LCDModes: map[int]uint8{4: 0}, LCDRotations: map[int]uint8{4: 0}}, Devices: map[int]*Devices{4: {ChannelId: 4, Name: "H150i", ContainsPump: true, LCDSerial: "lcd-a"}}}
	if snapshot, ok := device.DisplaySnapshot(); ok || snapshot.Available {
		t.Fatalf("display = %#v, ok=%t", snapshot, ok)
	}
}
