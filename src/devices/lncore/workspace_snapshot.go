package lncore

import (
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/rgbtopologypresentation"
	"sort"
)

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
	snapshot := deviceprofilepresentation.Snapshot{Supported: true}
	for name, profile := range d.UserProfiles {
		if profile != nil {
			snapshot.Profiles = append(snapshot.Profiles, name)
			if profile.Active {
				snapshot.ActiveProfile = name
			}
		}
	}
	sort.Strings(snapshot.Profiles)
	snapshot = deviceprofilepresentation.WithMutationCapabilities(snapshot, d)
	return snapshot, snapshot.ActiveProfile != "" && len(snapshot.Profiles) > 0
}

func (d *Device) RGBTopologyDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}

func (d *Device) RGBTopologySnapshot() (rgbtopologypresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || len(d.ExternalLedDevice) == 0 || len(d.ExternalLedDeviceAmount) == 0 {
		return rgbtopologypresentation.Snapshot{}, false
	}
	port := rgbtopologypresentation.Port{ID: 0, Label: "RGB Hub", SelectedType: d.DeviceProfile.ExternalHubDeviceType, SelectedAmount: d.DeviceProfile.ExternalHubDeviceAmount}
	port.DeviceTypes = append(port.DeviceTypes, rgbtopologypresentation.Option{ID: 0, Label: "No Device"})
	for _, device := range d.ExternalLedDevice {
		if device.Name == "" {
			return rgbtopologypresentation.Snapshot{}, false
		}
		port.DeviceTypes = append(port.DeviceTypes, rgbtopologypresentation.Option{ID: device.Index, Label: device.Name})
	}
	for id, label := range d.ExternalLedDeviceAmount {
		if label == "" {
			return rgbtopologypresentation.Snapshot{}, false
		}
		port.DeviceAmounts = append(port.DeviceAmounts, rgbtopologypresentation.Option{ID: id, Label: label})
	}
	sort.Slice(port.DeviceTypes, func(i, j int) bool { return port.DeviceTypes[i].ID < port.DeviceTypes[j].ID })
	sort.Slice(port.DeviceAmounts, func(i, j int) bool { return port.DeviceAmounts[i].ID < port.DeviceAmounts[j].ID })
	if !rgbTopologyOptionsContain(port.DeviceTypes, port.SelectedType) || !rgbTopologyOptionsContain(port.DeviceAmounts, port.SelectedAmount) {
		return rgbtopologypresentation.Snapshot{}, false
	}
	if _, ok := interface{}(d).(interface{ UpdateExternalHubDeviceType(int, int) uint8 }); !ok {
		return rgbtopologypresentation.Snapshot{}, false
	}
	if _, ok := interface{}(d).(interface{ UpdateExternalHubDeviceAmount(int, int) uint8 }); !ok {
		return rgbtopologypresentation.Snapshot{}, false
	}
	return rgbtopologypresentation.Snapshot{DeviceID: d.Serial, Ports: []rgbtopologypresentation.Port{port}}, true
}

func rgbTopologyOptionsContain(options []rgbtopologypresentation.Option, selected int) bool {
	for _, option := range options {
		if option.ID == selected {
			return true
		}
	}
	return false
}
