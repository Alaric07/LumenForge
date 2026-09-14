package nautilusLcd

import (
	"LumenForge/src/common"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/devices/lcd"
	"LumenForge/src/displaypresentation"
	"sort"
)

var nautilusDisplayImages = lcd.GetLcdImages
var nautilusDisplayImage = lcd.GetLcdImage

// DeviceProfileDeviceID and DeviceProfileSnapshot expose Nautilus LCD Cap's
// existing complete device profiles without changing their persistence or LCD
// behavior.
func (d *Device) DeviceProfileDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil {
		return deviceprofilepresentation.Snapshot{}, false
	}

	snapshot := deviceprofilepresentation.Snapshot{Supported: true, Scope: deviceprofilepresentation.ScopeDevice}
	for name, profile := range d.UserProfiles {
		if !common.AlphanumericRegex.MatchString(name) || profile == nil {
			return deviceprofilepresentation.Snapshot{}, false
		}
		snapshot.Profiles = append(snapshot.Profiles, name)
		if profile.Active {
			if snapshot.ActiveProfile != "" {
				return deviceprofilepresentation.Snapshot{}, false
			}
			snapshot.ActiveProfile = name
		}
	}
	if snapshot.ActiveProfile == "" || len(snapshot.Profiles) == 0 {
		return deviceprofilepresentation.Snapshot{}, false
	}
	sort.Strings(snapshot.Profiles)
	snapshot = deviceprofilepresentation.WithMutationCapabilities(snapshot, d)
	return snapshot, true
}

// DisplayDeviceID and DisplaySnapshot expose Nautilus LCD Cap's existing LCD
// profile state without changing LCD transport, rendering, or lifecycle.
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
	mode := int(d.DeviceProfile.LCDMode)
	rotation := int(d.DeviceProfile.LCDRotation)
	if _, ok := d.LCDModes[mode]; !ok {
		return displaypresentation.Snapshot{}, false
	}
	if _, ok := d.LCDRotations[rotation]; !ok {
		return displaypresentation.Snapshot{}, false
	}

	snapshot := displaypresentation.Snapshot{
		Available:        true,
		SelectedMode:     mode,
		Modes:            nautilusDisplayOptions(d.LCDModes, mode),
		SelectedRotation: rotation,
		Rotations:        nautilusDisplayOptions(d.LCDRotations, rotation),
		SelectedImage:    d.DeviceProfile.LCDImage,
		ImageMode:        d.DeviceProfile.LCDMode == lcd.DisplayImage,
		ImageModeID:      int(lcd.DisplayImage),
	}
	if len(snapshot.Modes) == 0 || len(snapshot.Rotations) == 0 {
		return displaypresentation.Snapshot{}, false
	}
	for _, image := range nautilusDisplayImages() {
		if image.Name == "" {
			continue
		}
		snapshot.Images = append(snapshot.Images, displaypresentation.ImageOption{Name: image.Name, Selected: image.Name == snapshot.SelectedImage})
	}
	sort.Slice(snapshot.Images, func(i, j int) bool { return snapshot.Images[i].Name < snapshot.Images[j].Name })
	if snapshot.ImageMode && (snapshot.SelectedImage == "" || nautilusDisplayImage(snapshot.SelectedImage) == nil) {
		return displaypresentation.Snapshot{}, false
	}
	return snapshot, true
}

func nautilusDisplayOptions(values map[int]string, selected int) []displaypresentation.Option {
	options := make([]displaypresentation.Option, 0, len(values))
	for id, label := range values {
		if label == "" {
			return nil
		}
		options = append(options, displaypresentation.Option{ID: id, Label: label, Selected: id == selected})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].ID < options[j].ID })
	return options
}
