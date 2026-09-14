package lsh

import (
	"LumenForge/src/common"
	"LumenForge/src/coolingpresentation"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/devices/lcd"
	"LumenForge/src/displaypresentation"
	"LumenForge/src/linkpresentation"
	"LumenForge/src/temperatures"
	"sort"
	"strconv"
)

var lshDisplayImages = lcd.GetLcdImages
var lshDisplayImage = lcd.GetLcdImage
var lshCoolingTemperatureProfiles = temperatures.GetTemperatureProfiles

func (d *Device) ConnectedDevicesDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// ConnectedDevicesSnapshot preserves LSH's persisted card order and exposes
// only topology controls with complete existing mutation contracts.
func (d *Device) ConnectedDevicesSnapshot() (linkpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.Serial == "" || len(d.Devices) == 0 {
		return linkpresentation.Snapshot{}, false
	}
	positions := d.DeviceProfile.Positions
	eligible := make(map[string]*Devices)
	for _, device := range d.Devices {
		if device == nil || device.IsVrmCooler || device.ChannelId < 0 || device.Name == "" || device.DeviceId == "" {
			return linkpresentation.Snapshot{}, false
		}
		id := "item_" + strconv.Itoa(device.ChannelId)
		if _, exists := eligible[id]; exists {
			return linkpresentation.Snapshot{}, false
		}
		eligible[id] = device
	}
	if len(positions) != len(eligible) {
		return linkpresentation.Snapshot{}, false
	}
	snapshot := linkpresentation.Snapshot{DeviceID: d.Serial}
	for _, position := range positions {
		device, ok := eligible[position]
		if !ok {
			return linkpresentation.Snapshot{}, false
		}
		delete(eligible, position)
		out := linkpresentation.Device{ChannelID: device.ChannelId, Position: position, Name: device.Name, DeviceID: device.DeviceId, Description: device.Description, ContainsPump: device.ContainsPump, AIO: device.AIO, HasSpeed: device.HasSpeed, HasLCD: device.LCDSerial != "", TitanAIO: device.TitanAIO}
		if device.IsLinkAdapter {
			selected, ok := d.DeviceProfile.ExternalAdapter[device.ChannelId]
			if !ok {
				return linkpresentation.Snapshot{}, false
			}
			seen := map[int]struct{}{}
			selectedFound := false
			for _, adapter := range d.LinkAdapter {
				if adapter.Index < 0 || adapter.Name == "" {
					return linkpresentation.Snapshot{}, false
				}
				if _, duplicate := seen[adapter.Index]; duplicate {
					return linkpresentation.Snapshot{}, false
				}
				seen[adapter.Index] = struct{}{}
				out.AdapterOptions = append(out.AdapterOptions, linkpresentation.Option{ID: adapter.Index, Label: adapter.Name})
				if adapter.Index == selected {
					selectedFound = true
				}
			}
			if len(out.AdapterOptions) == 0 || !selectedFound {
				return linkpresentation.Snapshot{}, false
			}
			sort.Slice(out.AdapterOptions, func(i, j int) bool { return out.AdapterOptions[i].ID < out.AdapterOptions[j].ID })
			out.SelectedAdapter = selected
		}
		if device.IsCommanderDuo {
			override, ok := d.DeviceProfile.CommanderDuoOverride[device.ChannelId]
			if !ok {
				return linkpresentation.Snapshot{}, false
			}
			out.CommanderDuo = &linkpresentation.CommanderDuoOverride{Enabled: override.Enabled, LEDChannels: override.LedChannels}
		}
		snapshot.Devices = append(snapshot.Devices, out)
	}
	return snapshot, len(eligible) == 0
}

func (d *Device) DeviceProfileDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// DeviceProfileSnapshot exposes LSH's existing device-scoped profiles. Those
// profiles remain responsible for both cooling and legacy RGB state.
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
	return deviceprofilepresentation.WithMutationCapabilities(snapshot, d), true
}

func (d *Device) CoolingDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// CoolingSnapshot adapts only connected LINK channels that already advertise
// speed control. It does not infer topology, pump modes, or temperature
// profile availability.
func (d *Device) CoolingSnapshot() (coolingpresentation.Snapshot, bool) {
	if d == nil {
		return coolingpresentation.Snapshot{}, false
	}
	snapshot := coolingpresentation.Snapshot{Available: true}
	for name, profile := range lshCoolingTemperatureProfiles() {
		if !profile.Hidden {
			snapshot.ProfileOptions = append(snapshot.ProfileOptions, coolingpresentation.ProfileOption{ID: name, Label: name})
		}
	}
	sort.Slice(snapshot.ProfileOptions, func(i, j int) bool { return snapshot.ProfileOptions[i].ID < snapshot.ProfileOptions[j].ID })
	for _, device := range d.Devices {
		if device == nil || !device.HasSpeed {
			continue
		}
		if device.ChannelId < 0 || device.Name == "" || device.Profile == "" {
			return coolingpresentation.Snapshot{}, false
		}
		channel := coolingpresentation.Channel{ID: device.ChannelId, Name: device.Name, Label: device.Label, RPM: device.Rpm, ContainsPump: device.ContainsPump || device.AIO, SelectedProfile: device.Profile}
		if device.HasTemps {
			channel.Temperature = device.TemperatureString
			if device.Temperature > 0 {
				value := device.Temperature
				channel.Celsius = &value
			}
		}
		snapshot.Channels = append(snapshot.Channels, channel)
	}
	sort.Slice(snapshot.Channels, func(i, j int) bool { return snapshot.Channels[i].ID < snapshot.Channels[j].ID })
	return snapshot, len(snapshot.Channels) > 0 && len(snapshot.ProfileOptions) > 0
}

func (d *Device) DisplayDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

// DisplaySnapshot retains LSH's independently persisted LCD values for every
// connected LCD-capable LINK channel. It deliberately excludes LCD assignment
// and topology controls.
func (d *Device) DisplaySnapshot() (displaypresentation.Snapshot, bool) {
	if d == nil || !d.HasLCD || d.DeviceProfile == nil {
		return displaypresentation.Snapshot{}, false
	}
	snapshot := displaypresentation.Snapshot{Available: true}
	for _, device := range d.Devices {
		if device == nil || device.LCDSerial == "" || (!device.ContainsPump && !device.AIO) {
			continue
		}
		mode, modeOK := d.DeviceProfile.LCDModes[device.ChannelId]
		rotation, rotationOK := d.DeviceProfile.LCDRotations[device.ChannelId]
		brightness, brightnessOK := d.DeviceProfile.LCDBrightness[device.ChannelId]
		if !modeOK || !rotationOK || !brightnessOK {
			return displaypresentation.Snapshot{}, false
		}
		if _, ok := d.LCDModes[int(mode)]; !ok {
			return displaypresentation.Snapshot{}, false
		}
		if _, ok := d.LCDRotations[int(rotation)]; !ok {
			return displaypresentation.Snapshot{}, false
		}
		if _, ok := d.LCDBrightnessLevels[int(brightness)]; !ok {
			return displaypresentation.Snapshot{}, false
		}
		display := displaypresentation.Display{ChannelID: device.ChannelId, Name: device.Name, SelectedMode: int(mode), Modes: lshDisplayOptions(d.LCDModes, int(mode)), SelectedRotation: int(rotation), Rotations: lshDisplayOptions(d.LCDRotations, int(rotation)), SelectedBrightness: int(brightness), BrightnessLevels: lshDisplayOptions(d.LCDBrightnessLevels, int(brightness)), SelectedImage: d.DeviceProfile.LCDImages[device.ChannelId], ImageMode: mode == lcd.DisplayImage, ImageModeID: int(lcd.DisplayImage)}
		if len(display.Modes) == 0 || len(display.Rotations) == 0 || len(display.BrightnessLevels) == 0 {
			return displaypresentation.Snapshot{}, false
		}
		for _, image := range lshDisplayImages() {
			if image.Name != "" {
				display.Images = append(display.Images, displaypresentation.ImageOption{Name: image.Name, Selected: image.Name == display.SelectedImage})
			}
		}
		sort.Slice(display.Images, func(i, j int) bool { return display.Images[i].Name < display.Images[j].Name })
		if display.ImageMode && (display.SelectedImage == "" || lshDisplayImage(display.SelectedImage) == nil) {
			return displaypresentation.Snapshot{}, false
		}
		snapshot.Displays = append(snapshot.Displays, display)
	}
	sort.Slice(snapshot.Displays, func(i, j int) bool { return snapshot.Displays[i].ChannelID < snapshot.Displays[j].ChannelID })
	return snapshot, len(snapshot.Displays) > 0
}

func lshDisplayOptions(values map[int]string, selected int) []displaypresentation.Option {
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
