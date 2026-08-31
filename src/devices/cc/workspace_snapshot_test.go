package cc

import (
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"LumenForge/src/temperatures"
	"reflect"
	"testing"
)

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
