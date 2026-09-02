package memory

import (
	"LumenForge/src/config"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadSupportedDevicesAndLookup(t *testing.T) {
	devices, err := loadSupportedDevices(filepath.Join("..", "..", "..", "database", "external", "memory.json"))
	if err != nil {
		t.Fatalf("load memory metadata: %v", err)
	}
	if len(devices) != len(defaultSupportedDevices) {
		t.Fatalf("loaded %d memory families, want %d", len(devices), len(defaultSupportedDevices))
	}
	if !reflect.DeepEqual(devices, defaultSupportedDevices) {
		t.Fatalf("external memory metadata differs from built-in fallback:\n got: %#v\nwant: %#v", devices, defaultSupportedDevices)
	}

	device := &Device{supportedDevices: devices}
	ddr4 := device.getDeviceMetadata(4, "W")
	if ddr4 == nil || ddr4.Name != "VENGEANCE RGB PRO" || ddr4.LedChannels != 10 || ddr4.Register != 0x31 {
		t.Fatalf("DDR4 W metadata = %#v", ddr4)
	}
	ddr5 := device.getDeviceMetadata(5, "P")
	if ddr5 == nil || ddr5.Name != "DOMINATOR TITANIUM RGB" || ddr5.LedChannels != 11 || ddr5.Register != 0x31 {
		t.Fatalf("DDR5 P metadata = %#v", ddr5)
	}
	if unknown := device.getDeviceMetadata(5, "X"); unknown != nil {
		t.Fatalf("unknown metadata = %#v, want nil", unknown)
	}
}

func TestUpdateRGBDeviceLabelUsesExistingDIMMLabels(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "database", "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	paths, err := config.ResolvePaths(config.PathOptions{Mode: config.ServiceModeDevelopment, ApplicationRoot: root, WorkingDirectory: root})
	if err != nil {
		t.Fatal(err)
	}
	originalPath := pwd
	pwd = paths.MutableDataRoot
	t.Cleanup(func() { pwd = originalPath })

	device := &Device{
		Product:       "Memory",
		Serial:        "memory-test",
		DeviceProfile: &DeviceProfile{},
		Devices: map[int]*Devices{
			0: {ChannelId: 0, Name: "DIMM 1", Label: "DIMM 1", LedChannels: 10, RGB: "static"},
			3: {ChannelId: 3, Name: "DIMM 4", Label: "DIMM 4", LedChannels: 10, RGB: "static"},
		},
	}

	if got := device.UpdateRGBDeviceLabel(3, "Rear DIMM"); got != 1 {
		t.Fatalf("UpdateRGBDeviceLabel(3) = %d, want success", got)
	}
	if got := device.Devices[3].Label; got != "Rear DIMM" {
		t.Fatalf("DIMM 3 label = %q, want Rear DIMM", got)
	}
	if got := device.Devices[0].Label; got != "DIMM 1" {
		t.Fatalf("DIMM 0 label changed to %q", got)
	}
	if got := device.UpdateRGBDeviceLabel(2, "Missing DIMM"); got != 0 {
		t.Fatalf("UpdateRGBDeviceLabel(invalid channel) = %d, want rejection", got)
	}

	profilePath := filepath.Join(root, "database", "profiles", "memory-test.json")
	profile := &DeviceProfile{}
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, profile); err != nil {
		t.Fatal(err)
	}
	if got := profile.Labels[3]; got != "Rear DIMM" {
		t.Fatalf("persisted DIMM 3 label = %q, want Rear DIMM", got)
	}
}

func TestLoadSupportedDevicesRejectsInvalidOrEmptyMetadata(t *testing.T) {
	for name, contents := range map[string]string{
		"invalid": "{",
		"empty":   "[]",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "memory.json")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatalf("write metadata fixture: %v", err)
			}
			if _, err := loadSupportedDevices(path); err == nil {
				t.Fatal("loadSupportedDevices() returned nil error")
			}
		})
	}
}
