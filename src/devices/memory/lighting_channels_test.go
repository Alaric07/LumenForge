package memory

import (
	"os"
	"path/filepath"
	"testing"

	"LumenForge/src/config"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
)

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
