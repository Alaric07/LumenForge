package cc

import (
	"LumenForge/src/devices/lcd"
	"LumenForge/src/displaypresentation"
	"sort"
)

var displayImages = lcd.GetLcdImages

// DisplayDeviceID and DisplaySnapshot adapt Commander CORE's existing optional
// LCD state without changing its LCD transport, rendering, or profile behavior.
func (d *Device) DisplayDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) DisplaySnapshot() (displaypresentation.Snapshot, bool) {
	if d == nil || !d.HasLCD || d.DeviceProfile == nil {
		return displaypresentation.Snapshot{}, false
	}

	snapshot := displaypresentation.Snapshot{
		Available:          true,
		ChannelID:          displayChannelID(d),
		SelectedMode:       int(d.DeviceProfile.LCDMode),
		SelectedRotation:   int(d.DeviceProfile.LCDRotation),
		SelectedBrightness: int(d.DeviceProfile.LCDBrightness),
		SelectedImage:      d.DeviceProfile.LCDImage,
		ImageMode:          d.DeviceProfile.LCDMode == lcd.DisplayImage,
		ImageModeID:        int(lcd.DisplayImage),
		Modes:              displayOptions(d.LCDModes, int(d.DeviceProfile.LCDMode)),
		Rotations:          displayOptions(d.LCDRotations, int(d.DeviceProfile.LCDRotation)),
		BrightnessLevels:   displayOptions(d.LCDBrightnessLevels, int(d.DeviceProfile.LCDBrightness)),
	}
	for _, image := range displayImages() {
		if image.Name != "" {
			snapshot.Images = append(snapshot.Images, displaypresentation.ImageOption{Name: image.Name, Selected: image.Name == snapshot.SelectedImage})
		}
	}
	sort.Slice(snapshot.Images, func(i, j int) bool { return snapshot.Images[i].Name < snapshot.Images[j].Name })
	return snapshot, len(snapshot.Modes) > 0 && len(snapshot.Rotations) > 0 && len(snapshot.BrightnessLevels) > 0
}

func displayChannelID(d *Device) int {
	channelID := 0
	first := true
	for _, device := range d.Devices {
		if device != nil && (first || device.ChannelId < channelID) {
			channelID, first = device.ChannelId, false
		}
	}
	return channelID
}

func displayOptions(values map[int]string, selected int) []displaypresentation.Option {
	options := make([]displaypresentation.Option, 0, len(values))
	for id, label := range values {
		if label != "" {
			options = append(options, displaypresentation.Option{ID: id, Label: label, Selected: id == selected})
		}
	}
	sort.Slice(options, func(i, j int) bool { return options[i].ID < options[j].ID })
	return options
}
