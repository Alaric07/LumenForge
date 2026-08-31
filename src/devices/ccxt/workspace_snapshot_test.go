package ccxt

import (
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"LumenForge/src/temperatures"
	"reflect"
	"testing"
)

type ccxtChannelState struct {
	values map[string]lightingsettings.IndependentDeviceLightingState
}

func (s *ccxtChannelState) Resolve(id string) (lightingsettings.IndependentDeviceLightingState, bool, error) {
	v, ok := s.values[id]
	if !ok {
		return lightingsettings.DefaultIndependentDeviceLightingState(), false, nil
	}
	return v, true, nil
}
func (s *ccxtChannelState) Set(id string, state lightingsettings.IndependentDeviceLightingState) error {
	s.values[id] = state
	return nil
}
func (s *ccxtChannelState) Delete(id string) (bool, error) {
	_, ok := s.values[id]
	delete(s.values, id)
	return ok, nil
}

func TestCCXTDeviceProfileSnapshotUsesActiveUserProfile(t *testing.T) {
	device := &Device{Serial: "ccxt-profile", UserProfiles: map[string]*DeviceProfile{
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
}

func TestCCXTDeviceProfileSnapshotFailsClosedWithoutActiveProfile(t *testing.T) {
	device := &Device{UserProfiles: map[string]*DeviceProfile{"default": {Active: false}, "missing": nil}}

	snapshot, ok := device.DeviceProfileSnapshot()
	if ok || !snapshot.Supported || snapshot.ActiveProfile != "" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
}

func TestCCXTCoolingSnapshotUsesExistingChannelState(t *testing.T) {
	originalProfiles := coolingTemperatureProfiles
	coolingTemperatureProfiles = func() map[string]temperatures.TemperatureProfileData {
		return map[string]temperatures.TemperatureProfileData{"quiet": {}, "hidden": {Hidden: true}}
	}
	t.Cleanup(func() { coolingTemperatureProfiles = originalProfiles })

	device := &Device{Serial: "ccxt-cooling", Devices: map[int]*Devices{
		2: {ChannelId: 2, Name: "Fan 2", Label: "Rear fan", Rpm: 912, Profile: "quiet", HasSpeed: true},
		1: {ChannelId: 1, Name: "Probe 1", Label: "Coolant", TemperatureString: "31.2°C", IsTemperatureProbe: true, HasTemps: true},
		3: {ChannelId: 3, Name: "RGB port", Label: "ignored"},
	}}

	snapshot, ok := device.CoolingSnapshot()
	if !ok || len(snapshot.Channels) != 1 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	channel := snapshot.Channels[0]
	if channel.ID != 2 || channel.Name != "Fan 2" || channel.Label != "Rear fan" || channel.RPM != 912 || channel.SelectedProfile != "quiet" {
		t.Fatalf("channel = %#v", channel)
	}
	if len(snapshot.TemperatureProbes) != 1 || snapshot.TemperatureProbes[0].ID != 1 || snapshot.TemperatureProbes[0].Temperature != "31.2°C" {
		t.Fatalf("probes = %#v", snapshot.TemperatureProbes)
	}
	if want := "quiet"; len(snapshot.ProfileOptions) != 1 || snapshot.ProfileOptions[0].ID != want {
		t.Fatalf("profile options = %#v", snapshot.ProfileOptions)
	}
}

func TestCCXTExposesCanonicalLightingSnapshotProvider(t *testing.T) {
	device := &Device{}
	if _, ok := interface{}(device).(interface {
		LightingSnapshot() (lightingpresentation.Snapshot, bool)
	}); !ok {
		t.Fatal("CCXT does not expose canonical lighting")
	}
}

func TestCCXTThreePinPortSnapshotUsesExistingExternalHubMetadata(t *testing.T) {
	device := &Device{DeviceProfile: &DeviceProfile{ExternalHubDeviceType: 6, ExternalHubDeviceAmount: 3}, ExternalLedDevice: []ExternalLedDevice{
		{Index: 10, Name: "Hydro X XD5", Total: 16},
		{Index: 0, Name: "No Device"},
		{Index: 6, Name: "8-LED Series Fan", Total: 8},
		{Index: 13, Name: "SP120 RGB Elite Kit", Total: 8, Kit: true},
		{Index: 7, Name: "Hydro X XC7", Total: 16},
	}}

	port := device.threePinPortSnapshot()
	if port == nil || port.DeviceType != 6 || port.Quantity != 3 || port.QuantityDisabled {
		t.Fatalf("ordinary port snapshot = %#v", port)
	}
	if got := []string{port.DeviceOptions[0].ID, port.DeviceOptions[1].ID, port.DeviceOptions[2].ID, port.DeviceOptions[3].ID, port.DeviceOptions[4].ID}; !reflect.DeepEqual(got, []string{"0", "6", "7", "10", "13"}) {
		t.Fatalf("device options = %#v", got)
	}
	if got := threePinQuantityValues(port); !reflect.DeepEqual(got, []string{"0", "1", "2", "3", "4", "5", "6"}) {
		t.Fatalf("ordinary quantity options = %#v", got)
	}

	device.DeviceProfile.ExternalHubDeviceType = 7
	if got := threePinQuantityValues(device.threePinPortSnapshot()); !reflect.DeepEqual(got, []string{"0", "1"}) {
		t.Fatalf("Hydro X single-device options = %#v", got)
	}
	device.DeviceProfile.ExternalHubDeviceType = 10
	if got := threePinQuantityValues(device.threePinPortSnapshot()); !reflect.DeepEqual(got, []string{"0", "1", "2"}) {
		t.Fatalf("Hydro X two-device options = %#v", got)
	}
	device.DeviceProfile.ExternalHubDeviceType = 13
	port = device.threePinPortSnapshot()
	if got := threePinQuantityValues(port); !reflect.DeepEqual(got, []string{"1"}) || !port.QuantityDisabled {
		t.Fatalf("kit quantity options = %#v, disabled=%t", got, port.QuantityDisabled)
	}
	device.DeviceProfile.ExternalHubDeviceType = 0
	device.DeviceProfile.ExternalHubDeviceAmount = 0
	port = device.threePinPortSnapshot()
	if got := threePinQuantityValues(port); !reflect.DeepEqual(got, []string{"0"}) || !port.QuantityDisabled {
		t.Fatalf("no-device quantity options = %#v, disabled=%t", got, port.QuantityDisabled)
	}
}

func threePinQuantityValues(port *lightingpresentation.ThreePinPort) []string {
	values := make([]string, len(port.QuantityOptions))
	for index, option := range port.QuantityOptions {
		values[index] = option.Value
	}
	return values
}

func TestCCXTCanonicalChannelsHydrateIndependentEffects(t *testing.T) {
	state := &ccxtChannelState{values: map[string]lightingsettings.IndependentDeviceLightingState{
		"ccxt-rgb-0": {SelectedEffect: "static", Brightness: 100},
		"ccxt-rgb-1": {SelectedEffect: "rainbow", Brightness: 100},
	}}
	device := &Device{Serial: "ccxt", channelLightingState: state, DeviceProfile: &DeviceProfile{RGBCluster: true}, Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{"static": {}, "rainbow": {}}}, RgbDevices: map[int]*Devices{
		0: {ChannelId: 0, Name: "Port 1", LedChannels: 8},
		1: {ChannelId: 1, Name: "Port 2", LedChannels: 16},
		8: {ChannelId: 8, Name: "Generated", LedChannels: 8, ExternalLed: true},
	}}
	if err := device.hydrateCanonicalChannels(); err != nil {
		t.Fatal(err)
	}
	if device.RgbDevices[0].RGB != "static" || device.RgbDevices[1].RGB != "rainbow" {
		t.Fatalf("hydrated effects = %q, %q", device.RgbDevices[0].RGB, device.RgbDevices[1].RGB)
	}
	snapshot, ok := device.LightingSnapshot()
	if !ok || len(snapshot.Channels) != 2 {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	if !snapshot.ClusterControlled || snapshot.ExternalControlled {
		t.Fatalf("snapshot ownership = %#v", snapshot)
	}
	if snapshot.Channels[0].TargetID != "ccxt-rgb-0" || snapshot.Channels[1].TargetID != "ccxt-rgb-1" || snapshot.Channels[0].Lighting.ConfiguredEffect != "static" || snapshot.Channels[1].Lighting.ConfiguredEffect != "rainbow" || !snapshot.Channels[0].Lighting.ClusterControlled || !snapshot.Channels[1].Lighting.ClusterControlled {
		t.Fatalf("channels = %#v", snapshot.Channels)
	}
	device.DeviceProfile.RGBCluster = false
	snapshot, ok = device.LightingSnapshot()
	if !ok || snapshot.ClusterControlled || snapshot.Channels[0].Lighting.ClusterControlled || snapshot.Channels[1].Lighting.ClusterControlled || snapshot.Channels[0].Lighting.ConfiguredEffect != "static" || snapshot.Channels[1].Lighting.ConfiguredEffect != "rainbow" {
		t.Fatalf("cluster release changed channel state: %#v", snapshot)
	}
}
