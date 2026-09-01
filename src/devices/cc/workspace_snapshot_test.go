package cc

import (
	"LumenForge/src/devices/lcd"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"LumenForge/src/temperatures"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCommanderCoreDisplaySnapshotIsGatedOnlyByHasLCD(t *testing.T) {
	originalImages := displayImages
	displayImages = func() []lcd.ImageData { return []lcd.ImageData{{Name: "loop"}, {Name: "status"}} }
	t.Cleanup(func() { displayImages = originalImages })

	device := commanderCoreLightingTestDevice()
	device.DeviceProfile.LCDMode = lcd.DisplayImage
	device.DeviceProfile.LCDImage = "status"
	device.DeviceProfile.LCDRotation = 2
	device.DeviceProfile.LCDBrightness = 64
	device.LCDModes = map[int]string{0: "Liquid Temperature", 10: "Image / GIF"}
	device.LCDRotations = map[int]string{0: "default", 2: "180 degrees"}
	device.LCDBrightnessLevels = map[int]string{1: "Off", 64: "100 %"}

	if snapshot, ok := device.DisplaySnapshot(); ok || snapshot.Available {
		t.Fatalf("non-LCD display snapshot = %#v, ok=%t", snapshot, ok)
	}
	device.HasLCD = true
	snapshot, ok := device.DisplaySnapshot()
	if !ok || !snapshot.Available || !snapshot.ImageMode || snapshot.ChannelID != 0 {
		t.Fatalf("display snapshot = %#v, ok=%t", snapshot, ok)
	}
	if len(snapshot.Modes) != 2 || !snapshot.Modes[1].Selected || len(snapshot.Rotations) != 2 || !snapshot.Rotations[1].Selected || len(snapshot.BrightnessLevels) != 2 || !snapshot.BrightnessLevels[1].Selected {
		t.Fatalf("display options = %#v", snapshot)
	}
	if len(snapshot.Images) != 2 || snapshot.Images[0].Name != "loop" || !snapshot.Images[1].Selected {
		t.Fatalf("display images = %#v", snapshot.Images)
	}
	if device.RgbDevices[1].RGB != "static" || !device.RgbDevices[1].ContainsPump {
		t.Fatalf("display snapshot changed pump RGB = %#v", device.RgbDevices[1])
	}
	if device.Devices[0].ChannelId != 0 || !device.Devices[0].ContainsPump {
		t.Fatalf("display snapshot changed cooling capability = %#v", device.Devices[0])
	}
}

func TestCommanderCoreDisplayMutationsRejectUnadvertisedValues(t *testing.T) {
	device := commanderCoreLightingTestDevice()
	device.HasLCD = true
	device.LCDModes = map[int]string{0: "Liquid Temperature"}
	device.LCDRotations = map[int]string{0: "default"}
	device.LCDBrightnessLevels = map[int]string{64: "100 %"}
	device.DeviceProfile.LCDMode = 0
	device.DeviceProfile.LCDRotation = 0
	device.DeviceProfile.LCDBrightness = 64

	if got := device.UpdateDeviceLcd(0, 99); got != 0 || device.DeviceProfile.LCDMode != 0 {
		t.Fatalf("invalid mode result=%d profile=%#v", got, device.DeviceProfile)
	}
	if got := device.UpdateDeviceLcdRotation(0, 3); got != 0 || device.DeviceProfile.LCDRotation != 0 {
		t.Fatalf("invalid rotation result=%d profile=%#v", got, device.DeviceProfile)
	}
	if got := device.UpdateDeviceLcdBrightness(0, 33); got != 0 || device.DeviceProfile.LCDBrightness != 64 {
		t.Fatalf("invalid brightness result=%d profile=%#v", got, device.DeviceProfile)
	}
}

type commanderCoreChannelState struct {
	values map[string]lightingsettings.IndependentDeviceLightingState
}

func (s *commanderCoreChannelState) Resolve(id string) (lightingsettings.IndependentDeviceLightingState, bool, error) {
	value, ok := s.values[id]
	if !ok {
		return lightingsettings.DefaultIndependentDeviceLightingState(), false, nil
	}
	return value, true, nil
}
func (s *commanderCoreChannelState) Set(id string, value lightingsettings.IndependentDeviceLightingState) error {
	s.values[id] = value
	return nil
}
func (s *commanderCoreChannelState) Delete(id string) (bool, error) {
	_, ok := s.values[id]
	delete(s.values, id)
	return ok, nil
}

func commanderCoreLightingTestDevice() *Device {
	return &Device{Serial: "cc-lighting", DeviceProfile: &DeviceProfile{RGBProfiles: map[int]string{}, RGBLabels: map[int]string{}, LCDMode: 4, CustomLEDs: map[int]int{4: 6}}, Devices: map[int]*Devices{0: {ChannelId: 0, ContainsPump: true}}, Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{"static": {}, "rainbow": {}, "liquid-temperature": {}, "led": {}}}, channelLightingState: &commanderCoreChannelState{values: map[string]lightingsettings.IndependentDeviceLightingState{"cc-lighting-rgb-0": {SelectedEffect: "rainbow", Brightness: 100}, "cc-lighting-rgb-4": {SelectedEffect: "static", Brightness: 100}}}, RgbDevices: map[int]*Devices{9: {ChannelId: 4, Name: "Configured port", LedChannels: 8, RGB: "static"}, 1: {ChannelId: 0, Name: "H100i ELITE CAPELLIX", LedChannels: 24, ContainsPump: true, RGB: "static"}, 7: {ChannelId: 6, Name: "No LEDs", LedChannels: 0}}}
}

func installCommanderCoreProfilePersistenceTestRoot(t *testing.T) {
	t.Helper()
	previous := pwd
	pwd = t.TempDir()
	if err := os.MkdirAll(filepath.Join(pwd, "database", "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pwd = previous })
	logger.Init()
}

func TestCommanderCoreDeviceProfileSnapshotUsesActiveUserProfile(t *testing.T) {
	device := &Device{Serial: "cc-profile", UserProfiles: map[string]*DeviceProfile{
		"studio":  {Active: false},
		"default": {Active: true},
		"missing": nil,
	}}

	snapshot, ok := device.DeviceProfileSnapshot()
	if !ok || !snapshot.Supported || snapshot.ActiveProfile != "default" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	if want := []string{"default", "studio"}; !reflect.DeepEqual(snapshot.Profiles, want) {
		t.Fatalf("profiles = %#v, want %#v", snapshot.Profiles, want)
	}
	if snapshot.DefaultProfileDisplayLabel != "" {
		t.Fatalf("default display label = %q", snapshot.DefaultProfileDisplayLabel)
	}
	if device.DeviceProfileDeviceID() != "cc-profile" {
		t.Fatalf("profile device ID = %q", device.DeviceProfileDeviceID())
	}
}

func TestCommanderCoreDeviceProfileSnapshotFailsClosedWithoutActiveProfile(t *testing.T) {
	device := &Device{UserProfiles: map[string]*DeviceProfile{"default": {Active: false}, "missing": nil}}

	snapshot, ok := device.DeviceProfileSnapshot()
	if ok || !snapshot.Supported || snapshot.ActiveProfile != "" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}

func TestCommanderCoreCoolingSnapshotPreservesPumpAndTelemetry(t *testing.T) {
	originalProfiles := coolingTemperatureProfiles
	coolingTemperatureProfiles = func() map[string]temperatures.TemperatureProfileData {
		return map[string]temperatures.TemperatureProfileData{"quiet": {}, "hidden": {Hidden: true}}
	}
	t.Cleanup(func() { coolingTemperatureProfiles = originalProfiles })

	device := &Device{Serial: "cc-cooling", Devices: map[int]*Devices{
		0: {ChannelId: 0, Name: "H150i", Label: "Pump", Rpm: 2440, TemperatureString: "31.2°C", Profile: "quiet", HasSpeed: true, HasTemps: true, ContainsPump: true},
		1: {ChannelId: 1, Name: "Fan 1", Label: "Front", Rpm: 912, Profile: "quiet", HasSpeed: true},
		7: {ChannelId: 7, Name: "Temperature Probe 1", Label: "Coolant", TemperatureString: "29.4°C", IsTemperatureProbe: true, HasTemps: true},
		8: {ChannelId: 8, Name: "RGB port", Label: "ignored"},
	}}

	snapshot, ok := device.CoolingSnapshot()
	if !ok || len(snapshot.Channels) != 2 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	pump, fan := snapshot.Channels[0], snapshot.Channels[1]
	if pump.ID != 0 || !pump.ContainsPump || pump.Name != "H150i" || pump.RPM != 2440 || pump.Temperature != "31.2°C" || pump.SelectedProfile != "quiet" {
		t.Fatalf("pump = %#v", pump)
	}
	if fan.ID != 1 || fan.ContainsPump || fan.Name != "Fan 1" || fan.RPM != 912 || fan.Temperature != "" || fan.SelectedProfile != "quiet" {
		t.Fatalf("fan = %#v", fan)
	}
	if len(snapshot.TemperatureProbes) != 1 || snapshot.TemperatureProbes[0].ID != 7 || snapshot.TemperatureProbes[0].Temperature != "29.4°C" {
		t.Fatalf("probes = %#v", snapshot.TemperatureProbes)
	}
	if len(snapshot.ProfileOptions) != 1 || snapshot.ProfileOptions[0].ID != "quiet" {
		t.Fatalf("profile options = %#v", snapshot.ProfileOptions)
	}
}

func TestCommanderCoreExposesCanonicalLightingSnapshotProvider(t *testing.T) {
	device := commanderCoreLightingTestDevice()
	if _, ok := interface{}(device).(interface {
		LightingSnapshot() (lightingpresentation.Snapshot, bool)
	}); !ok {
		t.Fatal("Commander CORE does not expose canonical lighting")
	}
}

func TestCommanderCoreCanonicalLightingUsesPhysicalChannelIdentityAndPumpRGB(t *testing.T) {
	device := commanderCoreLightingTestDevice()
	device.HasLCD = true
	snapshot, ok := device.LightingSnapshot()
	if !ok || len(snapshot.Channels) != 2 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	pump, port := snapshot.Channels[0], snapshot.Channels[1]
	if pump.TargetID != "cc-lighting-rgb-0" || pump.ChannelID != "0" || pump.Name != "H100i ELITE CAPELLIX" || !pump.ContainsPump || pump.LEDCount != 24 || pump.Lighting.ConfiguredEffect != "rainbow" {
		t.Fatalf("pump snapshot = %#v", pump)
	}
	if port.TargetID != "cc-lighting-rgb-4" || port.ChannelID != "4" || port.LEDCount != 8 || port.Lighting.ConfiguredEffect != "static" {
		t.Fatalf("port snapshot = %#v", port)
	}
	if device.DeviceProfile.LCDMode != 4 {
		t.Fatal("lighting snapshot changed LCD profile state")
	}
	if !device.SupportsLightingEffect("liquid-temperature") {
		t.Fatal("pump-capable controller did not advertise liquid-temperature")
	}
	device.Devices = map[int]*Devices{}
	if device.SupportsLightingEffect("liquid-temperature") {
		t.Fatal("controller without pump advertised liquid-temperature")
	}
}

func TestCommanderCoreManualRGBPortsUseOnlyBackendFreePortMetadata(t *testing.T) {
	device := commanderCoreLightingTestDevice()
	device.FreeLedPorts = map[int]string{0: "Pump", 2: "RGB Port 2", 7: "Invalid"}
	device.ExternalLedDevice = []ExternalLedDevice{{Index: 0, Name: "No Device"}, {Index: 6, Name: "8-LED Series Fan", Total: 8}}
	device.DeviceProfile.CustomLEDs = map[int]int{2: 6}

	snapshot, ok := device.LightingSnapshot()
	if !ok || len(snapshot.ManualRGBPorts) != 1 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	port := snapshot.ManualRGBPorts[0]
	if port.PortID != 2 || port.Name != "RGB Port 2" || port.Selected != 6 || len(port.Options) != 2 || !port.Options[1].Selected {
		t.Fatalf("manual RGB port = %#v", port)
	}
}

func TestCommanderCoreCanonicalLightingHydratesAndRestoresProfileState(t *testing.T) {
	device := commanderCoreLightingTestDevice()
	state := device.channelLightingState.(*commanderCoreChannelState)
	if err := device.setChannelSelectedEffect(4, "rainbow"); err != nil {
		t.Fatal(err)
	}
	if got := state.values["cc-lighting-rgb-4"].SelectedEffect; got != "rainbow" {
		t.Fatalf("canonical state = %q", got)
	}
	if err := device.hydrateCanonicalChannels(); err != nil || device.RgbDevices[9].RGB != "rainbow" {
		t.Fatalf("hydration err=%v rgb=%q", err, device.RgbDevices[9].RGB)
	}
	profile := &DeviceProfile{RGBProfiles: map[int]string{0: "static", 4: "rainbow"}, LCDMode: 3, CustomLEDs: map[int]int{4: 6}, SpeedProfiles: map[int]string{0: "quiet"}}
	if err := device.restoreCanonicalChannelProfile(profile); err != nil {
		t.Fatal(err)
	}
	if device.RgbDevices[1].RGB != "static" || device.RgbDevices[9].RGB != "rainbow" || profile.LCDMode != 3 || profile.CustomLEDs[4] != 6 || profile.SpeedProfiles[0] != "quiet" {
		t.Fatalf("profile restore = %#v, channels=%#v", profile, device.RgbDevices)
	}
	profile.RGBProfiles[4] = "invalid"
	if err := device.restoreCanonicalChannelProfile(profile); err == nil {
		t.Fatal("invalid saved effect restored")
	}
}

func TestCommanderCoreCanonicalLightingOwnershipBlocksMutation(t *testing.T) {
	device := commanderCoreLightingTestDevice()
	device.DeviceProfile.RGBCluster = true
	if err := device.SetLightingChannelEffect("cc-lighting-rgb-0", "static"); err == nil {
		t.Fatal("Cluster ownership allowed mutation")
	}
	device.DeviceProfile.RGBCluster = false
	device.DeviceProfile.OpenRGBIntegration = true
	if err := device.SetLightingChannelEffect("cc-lighting-rgb-0", "static"); err == nil {
		t.Fatal("OpenRGB ownership allowed mutation")
	}
}

func TestCommanderCoreBulkEffectControlUsesCanonicalChildTransaction(t *testing.T) {
	device := commanderCoreLightingTestDevice()
	installCommanderCoreProfilePersistenceTestRoot(t)
	restarts := 0
	device.lightingRestart = func() { restarts++ }
	state := device.channelLightingState.(*commanderCoreChannelState)

	snapshot, ok := device.LightingSnapshot()
	if !ok || snapshot.BulkEffectControl == nil || !snapshot.BulkEffectControl.Mixed || snapshot.BulkEffectControl.ConfiguredEffect != "" {
		t.Fatalf("mixed bulk snapshot = %#v", snapshot.BulkEffectControl)
	}
	for _, option := range snapshot.BulkEffectControl.SupportedEffects {
		if option.ID == "led" || option.ID == "keyboard" || option.ID == "liquid-temperature" {
			t.Fatalf("unsupported bulk option = %q", option.ID)
		}
	}
	if err := device.SetLightingAllChannelEffects("led"); err == nil {
		t.Fatal("invalid bulk effect accepted")
	}
	if state.values["cc-lighting-rgb-0"].SelectedEffect != "rainbow" || state.values["cc-lighting-rgb-4"].SelectedEffect != "static" {
		t.Fatalf("invalid bulk mutation changed state: %#v", state.values)
	}
	if err := device.SetLightingAllChannelEffects("static"); err != nil {
		t.Fatal(err)
	}
	for _, channelID := range []int{0, 4} {
		if state.values[device.canonicalChannelTargetID(channelID)].SelectedEffect != "static" || device.DeviceProfile.RGBProfiles[channelID] != "static" {
			t.Fatalf("channel %d state/profile = %#v/%q", channelID, state.values, device.DeviceProfile.RGBProfiles[channelID])
		}
	}
	if restarts != 1 {
		t.Fatalf("restarts = %d", restarts)
	}
	if _, err := os.Stat(filepath.Join(pwd, "database", "profiles", "cc-lighting.json")); err != nil {
		t.Fatalf("bulk profile was not saved: %v", err)
	}
	snapshot, ok = device.LightingSnapshot()
	if !ok || snapshot.BulkEffectControl.Mixed || snapshot.BulkEffectControl.ConfiguredEffect != "static" {
		t.Fatalf("uniform bulk snapshot = %#v", snapshot.BulkEffectControl)
	}
	if err := device.SetLightingChannelEffect("cc-lighting-rgb-4", "rainbow"); err != nil {
		t.Fatal(err)
	}
	if state.values["cc-lighting-rgb-4"].SelectedEffect != "rainbow" || state.values["cc-lighting-rgb-0"].SelectedEffect != "static" {
		t.Fatalf("child edit after bulk = %#v", state.values)
	}
	device.DeviceProfile.RGBCluster = true
	if err := device.SetLightingAllChannelEffects("static"); err == nil {
		t.Fatal("Cluster ownership allowed bulk mutation")
	}
	device.DeviceProfile.RGBCluster = false
	device.DeviceProfile.OpenRGBIntegration = true
	if err := device.SetLightingAllChannelEffects("static"); err == nil {
		t.Fatal("OpenRGB ownership allowed bulk mutation")
	}
	for _, value := range state.values {
		if value.SelectedEffect == "mixed" {
			t.Fatal("Mixed was stored as a lighting effect")
		}
	}
}

func TestCommanderCoreCanonicalChannelMutationHydratesActiveProfileSnapshot(t *testing.T) {
	device := commanderCoreLightingTestDevice()
	if err := device.setChannelSelectedEffect(0, "static"); err != nil {
		t.Fatal(err)
	}
	if err := device.hydrateCanonicalChannels(); err != nil {
		t.Fatal(err)
	}
	device.snapshotCanonicalChannelEffects()
	state := device.channelLightingState.(*commanderCoreChannelState)
	if state.values["cc-lighting-rgb-0"].SelectedEffect != "static" || device.RgbDevices[1].RGB != "static" || device.DeviceProfile.RGBProfiles[0] != "static" {
		t.Fatalf("state=%#v channel=%#v profile=%#v", state.values, device.RgbDevices[1], device.DeviceProfile.RGBProfiles)
	}
	if err := device.setChannelSelectedEffect(0, "invalid"); err == nil {
		t.Fatal("invalid canonical effect was accepted")
	}
}

func TestCommanderCoreCanonicalChannelEffectSettingsAreIndependent(t *testing.T) {
	runtime, err := lightingsettings.LoadIndependentDeviceRuntime(filepath.Join(t.TempDir(), "state.json"), filepath.Join(t.TempDir(), "effects.json"), filepath.Join("..", "..", "..", "database", "rgb.json"))
	if err != nil {
		t.Fatal(err)
	}
	device := &Device{Serial: "cc-settings", DeviceProfile: &DeviceProfile{}, Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{"static": {}, "rainbow": {}}}, channelLightingState: runtime.State, channelLightingEffects: runtime.Effects, channelLightingResolver: runtime.Resolver, lightingRestart: func() {}, RgbDevices: map[int]*Devices{
		0: {ChannelId: 0, Name: "Pump", LedChannels: 8, RGB: "static"},
		1: {ChannelId: 1, Name: "Fan", LedChannels: 8, RGB: "static"},
	}}
	first, second := "cc-settings-rgb-0", "cc-settings-rgb-1"
	for _, targetID := range []string{first, second} {
		if err := device.setChannelSelectedEffect(int(targetID[len(targetID)-1]-'0'), "static"); err != nil {
			t.Fatal(err)
		}
	}
	setColor := func(targetID string, color lightingsettings.Color) {
		settings, err := device.ResolveLightingChannelEffectSettings(targetID, "static")
		if err != nil {
			t.Fatal(err)
		}
		settings.SingleColor.Color = color
		if err = device.SetLightingChannelEffectSettings(targetID, "static", settings); err != nil {
			t.Fatal(err)
		}
	}
	setColor(first, lightingsettings.Color{Red: 255})
	setColor(second, lightingsettings.Color{Green: 255})
	if settings, _ := device.ResolveLightingChannelEffectSettings(first, "static"); settings.SingleColor.Color.Red != 255 || settings.SingleColor.Color.Green != 0 {
		t.Fatalf("first static settings = %#v", settings)
	}
	if settings, _ := device.ResolveLightingChannelEffectSettings(second, "static"); settings.SingleColor.Color.Green != 255 || settings.SingleColor.Color.Red != 0 {
		t.Fatalf("second static settings = %#v", settings)
	}
	for targetID, speed := range map[string]float64{first: 1.5, second: 4.5} {
		settings, err := device.ResolveLightingChannelEffectSettings(targetID, "rainbow")
		if err != nil {
			t.Fatal(err)
		}
		settings.Speed = &speed
		if err = device.SetLightingChannelEffectSettings(targetID, "rainbow", settings); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, ok := device.LightingSnapshot()
	if !ok || len(snapshot.Channels) != 2 || snapshot.Channels[0].Lighting.SingleColorHex != "#ff0000" || snapshot.Channels[1].Lighting.SingleColorHex != "#00ff00" {
		t.Fatalf("channel snapshots = %#v", snapshot)
	}
	if err := device.ResetLightingChannelEffectSettings(first, "static"); err != nil {
		t.Fatal(err)
	}
	if settings, _ := device.ResolveLightingChannelEffectSettings(second, "static"); settings.SingleColor.Color.Green != 255 {
		t.Fatalf("reset of first channel changed second settings: %#v", settings)
	}
	if state, _, _ := runtime.State.Resolve(first); state.SelectedEffect != "static" {
		t.Fatalf("first selected effect = %#v", state)
	}
	if state, _, _ := runtime.State.Resolve(second); state.SelectedEffect != "static" {
		t.Fatalf("second selected effect = %#v", state)
	}
}

func TestCommanderCoreMixedRendererKeepsCanonicalSingleColorProfiles(t *testing.T) {
	device := commanderCoreLightingTestDevice()
	first, second := device.RgbDevices[1], device.RgbDevices[9]
	red := &rgb.Profile{StartColor: rgb.Color{Red: 255}}
	blue := &rgb.Profile{StartColor: rgb.Color{Blue: 255}}
	if red.StartColor == blue.StartColor {
		t.Fatal("test setup lost independent static colors")
	}

	// The all-static path keeps both independently resolved single colors.
	if !device.channelRendererUsesResolvedColors(first, "static", red) || !device.channelRendererUsesResolvedColors(second, "static", blue) {
		t.Fatal("canonical static profiles were not treated as resolved colors")
	}
	// A generated sibling enters the mixed renderer but must not change the
	// static channel's resolved-color decision, including after a restart.
	if device.channelRendererUsesResolvedColors(second, "rainbow", &rgb.Profile{}) {
		t.Fatal("generated canonical palette was treated as a custom color")
	}
	if !device.channelRendererUsesResolvedColors(first, "static", red) {
		t.Fatal("mixed renderer restart replaced canonical static color semantics")
	}
	if !device.channelRendererUsesResolvedColors(first, "arc", &rgb.Profile{StartColor: rgb.Color{Red: 1}, EndColor: rgb.Color{Blue: 1}}) || !device.channelRendererUsesResolvedColors(first, "cpu-temperature", &rgb.Profile{StartColor: rgb.Color{Red: 1}, MiddleColor: rgb.Color{Green: 1}, EndColor: rgb.Color{Blue: 1}}) {
		t.Fatal("canonical two-color or temperature palette lost resolved-color behavior")
	}
	if device.channelRendererUsesResolvedColors(first, "gradient", &rgb.Profile{}) {
		t.Fatal("canonical gradient changed its existing renderer behavior")
	}
	// Legacy/generated topology channels retain their existing heuristic.
	legacy := &Devices{ChannelId: 7, LedChannels: 8}
	if device.channelRendererUsesResolvedColors(legacy, "static", red) {
		t.Fatal("non-canonical channel bypassed legacy color heuristic")
	}
}

func TestCommanderCoreUnavailableCanonicalRuntimeFailsClosed(t *testing.T) {
	device := &Device{Serial: "cc-unavailable", DeviceProfile: &DeviceProfile{}, RgbDevices: map[int]*Devices{0: {ChannelId: 0, LedChannels: 8}}}
	if device.canonicalChannel(device.RgbDevices[0]) {
		t.Fatal("unattached canonical runtime exposed a channel")
	}
	if snapshot, ok := device.LightingSnapshot(); ok || snapshot.TargetKind != "" {
		t.Fatalf("unavailable snapshot = %#v, ok=%t", snapshot, ok)
	}
}

func TestCommanderCoreUnsupportedPersistedEffectFallsBackToDefault(t *testing.T) {
	device := commanderCoreLightingTestDevice()
	state := device.channelLightingState.(*commanderCoreChannelState)
	state.values["cc-lighting-rgb-0"] = lightingsettings.IndependentDeviceLightingState{SelectedEffect: "liquid-temperature", Brightness: 100}
	if err := device.hydrateCanonicalChannels(); err != nil || device.RgbDevices[1].RGB != "liquid-temperature" {
		t.Fatalf("pump hydration err=%v channel=%q", err, device.RgbDevices[1].RGB)
	}
	device.Devices = map[int]*Devices{}
	if err := device.hydrateCanonicalChannels(); err != nil {
		t.Fatal(err)
	}
	if got := state.values["cc-lighting-rgb-0"].SelectedEffect; got != lightingsettings.DefaultIndependentDeviceEffect || device.RgbDevices[1].RGB != lightingsettings.DefaultIndependentDeviceEffect {
		t.Fatalf("fallback state=%q channel=%q", got, device.RgbDevices[1].RGB)
	}
	state.values["cc-lighting-rgb-0"] = lightingsettings.IndependentDeviceLightingState{SelectedEffect: "invalid", Brightness: 100}
	if err := device.hydrateCanonicalChannels(); err == nil {
		t.Fatal("malformed persisted state silently fell back")
	}
}

func TestCommanderCoreProfileSwitchPreflightProtectsOutgoingCanonicalState(t *testing.T) {
	device := commanderCoreLightingTestDevice()
	current := device.DeviceProfile
	current.Active = true
	target := &DeviceProfile{Active: false, RGBProfiles: map[int]string{0: "static", 4: "rainbow"}}
	invalid := &DeviceProfile{Active: false, RGBProfiles: map[int]string{0: "static", 4: "invalid"}}
	state := device.channelLightingState.(*commanderCoreChannelState)
	if _, err := device.canonicalChannelProfileSelections(invalid); err == nil {
		t.Fatal("invalid target profile passed preflight")
	}
	if !current.Active || target.Active || state.values["cc-lighting-rgb-0"].SelectedEffect != "rainbow" || state.values["cc-lighting-rgb-4"].SelectedEffect != "static" {
		t.Fatalf("failed preflight changed profile/state: current=%#v target=%#v state=%#v", current, target, state.values)
	}
	selections, err := device.canonicalChannelProfileSelections(target)
	if err != nil {
		t.Fatal(err)
	}
	if err = device.applyCanonicalChannelSelections(selections); err != nil {
		t.Fatal(err)
	}
	// Applying prepared target state does not hydrate renderer-facing channels
	// until the outgoing profile has been persisted from its own snapshot.
	if device.RgbDevices[1].RGB != "static" || device.RgbDevices[9].RGB != "static" {
		t.Fatalf("target state prematurely overwrote outgoing renderer state: %#v", device.RgbDevices)
	}
	if err = device.hydrateCanonicalChannels(); err != nil || device.RgbDevices[1].RGB != "static" || device.RgbDevices[9].RGB != "rainbow" {
		t.Fatalf("target hydration err=%v channels=%#v", err, device.RgbDevices)
	}
}
