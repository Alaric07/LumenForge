package systray

import (
	"LumenForge/src/devices"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/rgb"
	"github.com/godbus/dbus/v5"
	"sort"
	"strings"
)

var (
	deviceMap                   = make(map[int]string)
	getTrayDevices              = devices.GetDevicesEx
	getTrayDeviceClusterStatus  = devices.GetDeviceClusterStatus
	callTrayDeviceMethod        = devices.CallDeviceMethod
	lookupOpenRGBTrayDevice     = devices.LookupOpenRGBImport
	snapshotOpenRGBTrayLighting = func(device *openrgbimport.Device) (openrgbimport.LightingSnapshot, bool) {
		return device.LightingSnapshot()
	}
	setOpenRGBTrayEffect = func(device *openrgbimport.Device, effect string) error {
		return device.SetEffect(effect)
	}
)

type importedDeviceTrayState struct {
	device   *openrgbimport.Device
	snapshot openrgbimport.LightingSnapshot
	effects  []openrgbimport.LightingEffectOption
	ready    bool
}

func importedDeviceTrayLighting(serial string) (importedDeviceTrayState, bool) {
	wrapper, device, imported := lookupOpenRGBTrayDevice(serial)
	if !imported {
		return importedDeviceTrayState{}, false
	}
	state := importedDeviceTrayState{device: device}
	if wrapper == nil || device == nil || wrapper.Serial != serial || wrapper.Instance != device ||
		wrapper.Hidden || wrapper.Unavailable || !device.MatchesOpenRGBImport(serial) {
		return state, true
	}
	snapshot, ok := snapshotOpenRGBTrayLighting(device)
	if !ok {
		return state, true
	}
	state.snapshot = snapshot
	state.effects = append([]openrgbimport.LightingEffectOption(nil), snapshot.SupportedEffects...)
	sort.Slice(state.effects, func(first, second int) bool {
		return state.effects[first].ID < state.effects[second].ID
	})
	state.ready = true
	return state, true
}

func createSubMenuLayout(id int32, label string, items map[int32]string) MenuLayout {
	var children []dbus.Variant

	var childIds []int32
	for k := range items {
		childIds = append(childIds, k)
	}
	sort.Slice(childIds, func(i, j int) bool { return childIds[i] < childIds[j] })

	for _, childId := range childIds {
		childLabel := items[childId]
		childLayout := MenuLayout{
			ID: childId,
			Props: map[string]dbus.Variant{
				"label": dbus.MakeVariant(childLabel),
			},
		}
		children = append(children, dbus.MakeVariant(childLayout))
		menuItems[childId] = childLayout
	}

	layout := MenuLayout{
		ID: id,
		Props: map[string]dbus.Variant{
			"label":            dbus.MakeVariant(label),
			"children-display": dbus.MakeVariant("submenu"),
		},
		Children: children,
	}
	menuItems[id] = layout
	return layout
}

func RefreshDevicesMenu(parentId int32) {
	menuMutex.Lock()
	defer menuMutex.Unlock()

	allDevices := getTrayDevices()
	var validSerials []string
	for serial := range allDevices {
		if serial != "cluster" {
			validSerials = append(validSerials, serial)
		}
	}
	sort.Strings(validSerials)

	var devicesChildren []dbus.Variant

	toggleLayout := MenuLayout{
		ID: 999,
		Props: map[string]dbus.Variant{
			"label":     dbus.MakeVariant("Toggle All Standalone RGB"),
			"icon-name": dbus.MakeVariant("weather-clear-night"),
		},
	}
	devicesChildren = append(devicesChildren, dbus.MakeVariant(toggleLayout))
	menuItems[999] = toggleLayout

	sepLayout := MenuLayout{
		ID: 998,
		Props: map[string]dbus.Variant{
			"type": dbus.MakeVariant("separator"),
		},
	}
	devicesChildren = append(devicesChildren, dbus.MakeVariant(sepLayout))
	menuItems[998] = sepLayout

	for i, serial := range validSerials {
		deviceMap[i] = serial
		dev := allDevices[serial]

		baseId := int32(1000 + (i * 100))

		childItems := make(map[int32]string)

		productLabel := dev.Product
		if importedState, imported := importedDeviceTrayLighting(serial); imported {
			if importedState.ready && importedState.snapshot.ClusterControlled {
				productLabel += " [Clustered]"
				childItems[baseId] = "● Managed by Global Cluster"
			} else if importedState.ready {
				for index, effect := range importedState.effects {
					label := effect.Label
					if label == "" {
						label = effect.ID
					}
					childItems[baseId+int32(index)] = label
				}
			}
		} else if getTrayDeviceClusterStatus(serial) {
			productLabel += " [Clustered]"
			childItems[baseId] = "● Managed by Global Cluster"
		} else {
			var modes []string
			modesResult := callTrayDeviceMethod(serial, "GetRgbProfiles")
			if len(modesResult) > 0 && modesResult[0].IsValid() {
				if rgbData, ok := modesResult[0].Interface().(rgb.RGB); ok {
					for modeName := range rgbData.Profiles {
						modes = append(modes, modeName)
					}
					sort.Strings(modes)
				}
			}

			for j, mode := range modes {
				childItems[baseId+int32(j)] = strings.Title(mode)
			}
		}

		devLayout := createSubMenuLayout(baseId+99, productLabel, childItems)
		devicesChildren = append(devicesChildren, dbus.MakeVariant(devLayout))
	}

	layout := MenuLayout{
		ID: parentId,
		Props: map[string]dbus.Variant{
			"label":            dbus.MakeVariant("Individual Devices"),
			"children-display": dbus.MakeVariant("submenu"),
			"icon-name":        dbus.MakeVariant("preferences-desktop-peripherals"),
		},
		Children: devicesChildren,
	}

	menuItems[parentId] = layout

	found := false
	for _, v := range menuOrder {
		if v == parentId {
			found = true
			break
		}
	}
	if !found {
		menuOrder = append(menuOrder, parentId)
	}

	menuRevision++
	go emitMenuUpdate()
}
