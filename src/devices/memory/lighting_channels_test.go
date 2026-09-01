package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
)

func memoryIndexedColorBatch(count int) []lightingsettings.IndexedColor {
	colors := make([]lightingsettings.IndexedColor, count)
	for index := range colors {
		colors[index] = lightingsettings.IndexedColor{Index: index, ColorHex: fmt.Sprintf("#%02x%02x%02x", index, index+1, index+2)}
	}
	return colors
}

func installMemoryProfilePersistenceTestRoot(t *testing.T) {
	t.Helper()
	previousPWD := pwd
	pwd = t.TempDir()
	if err := os.MkdirAll(filepath.Join(pwd, "database", "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pwd = previousPWD })
	paths, err := config.ResolvePaths(config.PathOptions{
		Mode:            config.ServiceModeUser,
		ApplicationRoot: filepath.Join(pwd, "app"),
		ConfigRoot:      filepath.Join(pwd, "config"),
		DataRoot:        filepath.Join(pwd, "data"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(config.UsePathsForTest(paths))
	logger.Init()
}

func memoryCanonicalLightingTestDevice(t *testing.T) (*Device, *lightingsettings.IndependentDeviceRuntime) {
	t.Helper()
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(filepath.Join(t.TempDir(), "state.json"), filepath.Join(t.TempDir(), "effects.json"), filepath.Join("..", "..", "..", "database", "rgb.json"))
	if err != nil {
		t.Fatal(err)
	}
	brightness := uint8(100)
	device := &Device{Serial: "i2c0", DeviceProfile: &DeviceProfile{BrightnessSlider: &brightness, RGBProfiles: map[int]string{}, RGBPerLed: map[int]map[int]map[int]rgb.Color{}}, Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{"static": {}, "rainbow": {}, "led": {}}}, Devices: map[int]*Devices{0: {ChannelId: 0, Name: "DIMM A", LedChannels: 10}, 1: {ChannelId: 1, Name: "DIMM B", LedChannels: 12}, 2: {ChannelId: 2, Name: "No RGB"}}, channelLightingState: runtime.State, channelLightingEffects: runtime.Effects, channelLightingResolver: runtime.Resolver, lightingRestart: func() {}}
	return device, runtime
}

func TestMemoryCanonicalRuntimeHydrationFailureLeavesLegacyFallback(t *testing.T) {
	root := t.TempDir()
	defaultsPath, err := filepath.Abs(filepath.Join("..", "..", "..", "database"))
	if err != nil {
		t.Fatal(err)
	}
	paths := config.GetPaths()
	paths.OpenRGBDeviceLightingFile = filepath.Join(root, "state.json")
	paths.DeviceEffectSettingsFile = filepath.Join(root, "effects.json")
	paths.ShippedDatabaseRoot = defaultsPath
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile, filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = runtime.State.Set("i2c0-rgb-0", lightingsettings.IndependentDeviceLightingState{SelectedEffect: "rainbow", Brightness: 100}); err != nil {
		t.Fatal(err)
	}
	device := &Device{Serial: "i2c0", Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{"static": {}}}, Devices: map[int]*Devices{0: {ChannelId: 0, LedChannels: 10}}}
	if err = device.attachCanonicalChannelLightingRuntime(paths); err == nil {
		t.Fatal("hydration unexpectedly succeeded")
	}
	if device.channelLightingState != nil || device.channelLightingEffects != nil || device.channelLightingResolver != nil {
		t.Fatalf("partial canonical runtime remained attached: %#v %#v %#v", device.channelLightingState, device.channelLightingEffects, device.channelLightingResolver)
	}
}

func TestMemoryExposesCanonicalDIMMLighting(t *testing.T) {
	device, runtime := memoryCanonicalLightingTestDevice(t)
	if _, ok := interface{}(device).(interface {
		LightingSnapshot() (lightingpresentation.Snapshot, bool)
	}); !ok {
		t.Fatal("Memory does not expose canonical lighting")
	}
	if err := runtime.State.Set("i2c0-rgb-0", lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 100}); err != nil {
		t.Fatal(err)
	}
	if err := runtime.State.Set("i2c0-rgb-1", lightingsettings.IndependentDeviceLightingState{SelectedEffect: "rainbow", Brightness: 100}); err != nil {
		t.Fatal(err)
	}
	if err := device.hydrateCanonicalChannels(); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := device.LightingSnapshot()
	if !ok || len(snapshot.Channels) != 2 || snapshot.Channels[0].TargetID != "i2c0-rgb-0" || snapshot.Channels[1].TargetID != "i2c0-rgb-1" || device.Devices[0].RGB != "static" || device.Devices[1].RGB != "rainbow" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	device.DeviceProfile.RGBCluster, device.DeviceProfile.OpenRGBIntegration = true, true
	snapshot, ok = device.LightingSnapshot()
	if !ok || !snapshot.ClusterControlled || !snapshot.ExternalControlled || !snapshot.Channels[0].Lighting.ClusterControlled || !snapshot.Channels[0].Lighting.ExternalControlled {
		t.Fatalf("ownership snapshot = %#v", snapshot)
	}
}

func TestMemoryBulkEffectControlExcludesLedAndMutatesCanonicalChildren(t *testing.T) {
	device, runtime := memoryCanonicalLightingTestDevice(t)
	installMemoryProfilePersistenceTestRoot(t)
	if err := runtime.State.Set("i2c0-rgb-0", lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 100}); err != nil {
		t.Fatal(err)
	}
	if err := runtime.State.Set("i2c0-rgb-1", lightingsettings.IndependentDeviceLightingState{SelectedEffect: "rainbow", Brightness: 100}); err != nil {
		t.Fatal(err)
	}
	if err := device.hydrateCanonicalChannels(); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := device.LightingSnapshot()
	if !ok || snapshot.BulkEffectControl == nil || !snapshot.BulkEffectControl.Mixed || snapshot.BulkEffectControl.ConfiguredEffect != "" {
		t.Fatalf("mixed bulk snapshot = %#v", snapshot.BulkEffectControl)
	}
	for _, effect := range snapshot.BulkEffectControl.SupportedEffects {
		if effect.ID == "led" {
			t.Fatal("bulk control exposed led")
		}
	}
	if len(snapshot.Channels[0].Lighting.SupportedEffects) == len(snapshot.BulkEffectControl.SupportedEffects) {
		t.Fatal("per-DIMM led selection was removed")
	}
	if err := device.SetLightingAllChannelEffects("led"); err == nil {
		t.Fatal("bulk led was accepted")
	}
	if err := device.SetLightingAllChannelEffects("static"); err != nil {
		t.Fatal(err)
	}
	for _, channelID := range []int{0, 1} {
		state, _, err := runtime.State.Resolve(device.canonicalChannelTargetID(channelID))
		if err != nil || state.SelectedEffect != "static" || device.DeviceProfile.RGBProfiles[channelID] != "static" {
			t.Fatalf("channel %d = %#v, profile=%q, err=%v", channelID, state, device.DeviceProfile.RGBProfiles[channelID], err)
		}
	}
	snapshot, ok = device.LightingSnapshot()
	if !ok || snapshot.BulkEffectControl.Mixed || snapshot.BulkEffectControl.ConfiguredEffect != "static" {
		t.Fatalf("uniform bulk snapshot = %#v", snapshot.BulkEffectControl)
	}
	device.DeviceProfile.RGBCluster = true
	if err := device.SetLightingAllChannelEffects("rainbow"); err == nil {
		t.Fatal("cluster ownership accepted bulk mutation")
	}
	if err := device.SetLightingChannelEffect("i2c0-rgb-1", "led"); err == nil {
		t.Fatal("cluster ownership unexpectedly changed child")
	}
}

func TestMemoryIndexedColorsAreIsolatedAndRequireLed(t *testing.T) {
	device, runtime := memoryCanonicalLightingTestDevice(t)
	installMemoryProfilePersistenceTestRoot(t)
	for _, id := range []int{0, 1} {
		if err := runtime.State.Set(device.canonicalChannelTargetID(id), lightingsettings.IndependentDeviceLightingState{SelectedEffect: "led", Brightness: 100}); err != nil {
			t.Fatal(err)
		}
	}
	if err := device.hydrateCanonicalChannels(); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := device.LightingSnapshot()
	if !ok || len(snapshot.Channels[0].Lighting.IndexedColors) != 10 || snapshot.Channels[0].Lighting.IndexedColors[0].ColorHex != "#000000" {
		t.Fatalf("indexed snapshot = %#v", snapshot)
	}
	if len(device.DeviceProfile.RGBPerLed) != 0 {
		t.Fatal("snapshot initialized missing RGBPerLed state")
	}
	if err := device.SetLightingIndexedColor("i2c0-rgb-0", 1, lightingsettings.Color{Red: 255}); err != nil {
		t.Fatal(err)
	}
	if got := device.DeviceProfile.RGBPerLed[0][0][1]; got.Red != 255 {
		t.Fatalf("updated color = %#v", got)
	}
	if got := len(device.DeviceProfile.RGBPerLed[0][0]); got != 10 {
		t.Fatalf("repaired LED map length = %d", got)
	}
	if _, ok := device.DeviceProfile.RGBPerLed[1][0][1]; ok {
		t.Fatal("other DIMM changed")
	}
	if err := device.SetLightingIndexedColor("missing", 0, lightingsettings.Color{}); err == nil {
		t.Fatal("invalid target accepted")
	}
	if err := device.SetLightingIndexedColor("i2c0-rgb-0", 10, lightingsettings.Color{}); err == nil {
		t.Fatal("invalid index accepted")
	}
	if err := runtime.State.Set("i2c0-rgb-0", lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 100}); err != nil {
		t.Fatal(err)
	}
	if err := device.SetLightingIndexedColor("i2c0-rgb-0", 0, lightingsettings.Color{}); err == nil {
		t.Fatal("non-led effect accepted")
	}
}

func TestMemoryIndexedColorRepairsPartialLEDMap(t *testing.T) {
	device, runtime := memoryCanonicalLightingTestDevice(t)
	installMemoryProfilePersistenceTestRoot(t)
	if err := runtime.State.Set("i2c0-rgb-0", lightingsettings.IndependentDeviceLightingState{SelectedEffect: "led", Brightness: 100}); err != nil {
		t.Fatal(err)
	}
	if err := device.hydrateCanonicalChannels(); err != nil {
		t.Fatal(err)
	}
	device.DeviceProfile.RGBPerLed = map[int]map[int]map[int]rgb.Color{0: {0: {0: {Blue: 255}}}}
	if err := device.SetLightingIndexedColor("i2c0-rgb-0", 3, lightingsettings.Color{Green: 255}); err != nil {
		t.Fatal(err)
	}
	colors := device.DeviceProfile.RGBPerLed[0][0]
	if len(colors) != int(device.Devices[0].LedChannels) || colors[3].Green != 255 {
		t.Fatalf("partial map repair = %#v", colors)
	}
	if colors[0].Green != 255 {
		t.Fatalf("partial state should be replaced by renderer defaults: %#v", colors[0])
	}
}

func TestMemoryIndexedColorBatchReplacesOnlySelectedDIMM(t *testing.T) {
	device, runtime := memoryCanonicalLightingTestDevice(t)
	installMemoryProfilePersistenceTestRoot(t)
	for _, channelID := range []int{0, 1} {
		if err := runtime.State.Set(device.canonicalChannelTargetID(channelID), lightingsettings.IndependentDeviceLightingState{SelectedEffect: "led", Brightness: 100}); err != nil {
			t.Fatal(err)
		}
	}
	if err := device.hydrateCanonicalChannels(); err != nil {
		t.Fatal(err)
	}
	other := device.generateLedObject(device.Devices[1].LedChannels)
	other[0] = rgb.Color{Blue: 255, Hex: "#0000ff"}
	device.DeviceProfile.RGBPerLed[1] = map[int]map[int]rgb.Color{0: other}
	restarts := 0
	device.lightingRestart = func() { restarts++ }
	colors := memoryIndexedColorBatch(int(device.Devices[0].LedChannels))
	if err := device.SetLightingIndexedColors("i2c0-rgb-0", colors); err != nil {
		t.Fatal(err)
	}
	got := device.DeviceProfile.RGBPerLed[0][0]
	if len(got) != int(device.Devices[0].LedChannels) || got[3].Hex != "#030405" || got[3].Red != 3 || got[3].Green != 4 || got[3].Blue != 5 {
		t.Fatalf("saved colors = %#v", got)
	}
	if device.DeviceProfile.RGBPerLed[1][0][0].Hex != "#0000ff" {
		t.Fatalf("other DIMM changed: %#v", device.DeviceProfile.RGBPerLed[1][0])
	}
	if restarts != 1 {
		t.Fatalf("restarts = %d", restarts)
	}
	if _, err := os.Stat(filepath.Join(pwd, "database", "profiles", "i2c0.json")); err != nil {
		t.Fatalf("persisted profile: %v", err)
	}
}

func TestMemoryIndexedColorBatchRejectsInvalidInputWithoutMutation(t *testing.T) {
	device, runtime := memoryCanonicalLightingTestDevice(t)
	installMemoryProfilePersistenceTestRoot(t)
	if err := runtime.State.Set("i2c0-rgb-0", lightingsettings.IndependentDeviceLightingState{SelectedEffect: "led", Brightness: 100}); err != nil {
		t.Fatal(err)
	}
	if err := device.hydrateCanonicalChannels(); err != nil {
		t.Fatal(err)
	}
	existing := device.generateLedObject(device.Devices[0].LedChannels)
	existing[0] = rgb.Color{Red: 9, Hex: "#090000"}
	device.DeviceProfile.RGBPerLed[0] = map[int]map[int]rgb.Color{0: existing}
	valid := memoryIndexedColorBatch(int(device.Devices[0].LedChannels))
	duplicate := append([]lightingsettings.IndexedColor(nil), valid...)
	duplicate[len(duplicate)-1].Index = 0
	outOfRange := append([]lightingsettings.IndexedColor(nil), valid...)
	outOfRange[len(outOfRange)-1].Index = int(device.Devices[0].LedChannels)
	invalidHex := append([]lightingsettings.IndexedColor(nil), valid...)
	invalidHex[0].ColorHex = "red"
	for _, test := range []struct {
		name   string
		colors []lightingsettings.IndexedColor
	}{
		{"duplicate index", duplicate},
		{"missing index", duplicate},
		{"out of range index", outOfRange},
		{"invalid hex", invalidHex},
		{"wrong count", valid[:len(valid)-1]},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := device.SetLightingIndexedColors("i2c0-rgb-0", test.colors); err == nil {
				t.Fatal("invalid batch accepted")
			}
			if colors := device.DeviceProfile.RGBPerLed[0][0]; len(colors) != int(device.Devices[0].LedChannels) || colors[0].Hex != "#090000" {
				t.Fatalf("validation mutated colors: %#v", colors)
			}
		})
	}
}

func TestMemoryIndexedColorBatchRequiresLocalLedOwnership(t *testing.T) {
	for _, test := range []struct {
		name     string
		effect   string
		cluster  bool
		external bool
	}{
		{"non-led effect", "static", false, false},
		{"RGB Cluster", "led", true, false},
		{"OpenRGB", "led", false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			device, runtime := memoryCanonicalLightingTestDevice(t)
			installMemoryProfilePersistenceTestRoot(t)
			if err := runtime.State.Set("i2c0-rgb-0", lightingsettings.IndependentDeviceLightingState{SelectedEffect: test.effect, Brightness: 100}); err != nil {
				t.Fatal(err)
			}
			if err := device.hydrateCanonicalChannels(); err != nil {
				t.Fatal(err)
			}
			device.DeviceProfile.RGBCluster = test.cluster
			device.DeviceProfile.OpenRGBIntegration = test.external
			if err := device.SetLightingIndexedColors("i2c0-rgb-0", memoryIndexedColorBatch(int(device.Devices[0].LedChannels))); err == nil {
				t.Fatal("unavailable ownership accepted batch")
			}
		})
	}
}

func TestMemoryCanonicalDIMMSettingsAndLedSelection(t *testing.T) {
	device, runtime := memoryCanonicalLightingTestDevice(t)
	installMemoryProfilePersistenceTestRoot(t)
	for _, id := range []int{0, 1} {
		if err := device.setChannelSelectedEffect(id, "static"); err != nil {
			t.Fatal(err)
		}
	}
	for target, color := range map[string]lightingsettings.Color{"i2c0-rgb-0": {Red: 255}, "i2c0-rgb-1": {Blue: 255}} {
		settings, err := device.ResolveLightingChannelEffectSettings(target, "static")
		if err != nil {
			t.Fatal(err)
		}
		settings.SingleColor.Color = color
		if err = device.SetLightingChannelEffectSettings(target, "static", settings); err != nil {
			t.Fatal(err)
		}
	}
	first, _ := device.ResolveLightingChannelEffectSettings("i2c0-rgb-0", "static")
	second, _ := device.ResolveLightingChannelEffectSettings("i2c0-rgb-1", "static")
	if first.SingleColor.Color.Red != 255 || second.SingleColor.Color.Blue != 255 || second.SingleColor.Color.Red != 0 {
		t.Fatalf("settings are not independent: %#v %#v", first, second)
	}
	if err := device.ResetLightingChannelEffectSettings("i2c0-rgb-0", "static"); err != nil {
		t.Fatal(err)
	}
	second, _ = device.ResolveLightingChannelEffectSettings("i2c0-rgb-1", "static")
	if second.SingleColor.Color.Blue != 255 {
		t.Fatal("reset changed the other DIMM")
	}
	if err := device.setChannelSelectedEffect(0, "led"); err != nil {
		t.Fatal(err)
	}
	state, _, err := runtime.State.Resolve("i2c0-rgb-0")
	if err != nil || state.SelectedEffect != "led" {
		t.Fatalf("led state = %#v, err=%v", state, err)
	}
	if profile := device.channelRendererProfile(device.Devices[0], "led"); profile == nil {
		t.Fatal("led did not retain the device renderer")
	}
	if err := device.SetLightingChannelEffect("i2c0-rgb-1", "rainbow"); err != nil {
		t.Fatal(err)
	}
	if device.DeviceProfile.RGBProfiles[1] != "rainbow" {
		t.Fatalf("profile effect = %q", device.DeviceProfile.RGBProfiles[1])
	}
	device.DeviceProfile.RGBCluster = true
	if err := device.SetLightingChannelEffect("i2c0-rgb-1", "static"); err == nil {
		t.Fatal("Cluster ownership allowed mutation")
	}
}
