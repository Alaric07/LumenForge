package lnpro

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
	if d == nil || d.DeviceProfile == nil || len(d.DeviceProfile.ExternalHubs) == 0 || len(d.ExternalLedDevice) == 0 || len(d.ExternalLedDeviceAmount) == 0 {
		return rgbtopologypresentation.Snapshot{}, false
	}
	deviceTypes := make([]rgbtopologypresentation.Option, 0, len(d.ExternalLedDevice)+1)
	deviceTypes = append(deviceTypes, rgbtopologypresentation.Option{ID: 0, Label: "No Device"})
	for _, device := range d.ExternalLedDevice {
		if device.Name == "" {
			return rgbtopologypresentation.Snapshot{}, false
		}
		deviceTypes = append(deviceTypes, rgbtopologypresentation.Option{ID: device.Index, Label: device.Name})
	}
	amounts := make([]rgbtopologypresentation.Option, 0, len(d.ExternalLedDeviceAmount))
	for id, label := range d.ExternalLedDeviceAmount {
		if label == "" {
			return rgbtopologypresentation.Snapshot{}, false
		}
		amounts = append(amounts, rgbtopologypresentation.Option{ID: id, Label: label})
	}
	sort.Slice(deviceTypes, func(i, j int) bool { return deviceTypes[i].ID < deviceTypes[j].ID })
	sort.Slice(amounts, func(i, j int) bool { return amounts[i].ID < amounts[j].ID })
	snapshot := rgbtopologypresentation.Snapshot{DeviceID: d.Serial}
	for id, hub := range d.DeviceProfile.ExternalHubs {
		if hub == nil || int(hub.PortId) != id || !rgbTopologyOptionsContain(deviceTypes, hub.ExternalHubDeviceType) || !rgbTopologyOptionsContain(amounts, hub.ExternalHubDeviceAmount) {
			return rgbtopologypresentation.Snapshot{}, false
		}
		snapshot.Ports = append(snapshot.Ports, rgbtopologypresentation.Port{ID: id, Label: "RGB Port " + string(rune('1'+id)), DeviceTypes: append([]rgbtopologypresentation.Option(nil), deviceTypes...), SelectedType: hub.ExternalHubDeviceType, DeviceAmounts: append([]rgbtopologypresentation.Option(nil), amounts...), SelectedAmount: hub.ExternalHubDeviceAmount})
	}
	sort.Slice(snapshot.Ports, func(i, j int) bool { return snapshot.Ports[i].ID < snapshot.Ports[j].ID })
	if _, ok := interface{}(d).(interface{ UpdateExternalHubDeviceType(int, int) uint8 }); !ok {
		return rgbtopologypresentation.Snapshot{}, false
	}
	if _, ok := interface{}(d).(interface{ UpdateExternalHubDeviceAmount(int, int) uint8 }); !ok {
		return rgbtopologypresentation.Snapshot{}, false
	}
	return snapshot, len(snapshot.Ports) > 0
}

func rgbTopologyOptionsContain(options []rgbtopologypresentation.Option, selected int) bool {
	for _, option := range options {
		if option.ID == selected {
			return true
		}
	}
	return false
}
