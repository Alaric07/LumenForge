package k95platinum

import (
	"testing"

	"LumenForge/src/lightingsettings"
)

type k95LightingTestState struct {
	value    lightingsettings.IndependentDeviceLightingState
	setCount int
}

func (s *k95LightingTestState) Resolve(string) (lightingsettings.IndependentDeviceLightingState, bool, error) {
	return s.value, false, nil
}
func (s *k95LightingTestState) Set(_ string, value lightingsettings.IndependentDeviceLightingState) error {
	s.value = value
	s.setCount++
	return nil
}

type k95LightingTestResolver struct {
	settings map[string]lightingsettings.EffectSettings
}

func (r k95LightingTestResolver) Resolve(_ lightingsettings.Target, effect string) (lightingsettings.Resolution, error) {
	return lightingsettings.Resolution{Settings: r.settings[effect]}, nil
}

type k95LightingTestEffects struct {
	values map[string]lightingsettings.EffectSettings
}

func (s *k95LightingTestEffects) Set(_ string, effect string, settings lightingsettings.EffectSettings) error {
	s.values[effect] = settings
	return nil
}
func (s *k95LightingTestEffects) Delete(_ string, effect string) (bool, error) {
	_, ok := s.values[effect]
	delete(s.values, effect)
	return ok, nil
}

func newK95LightingTestDevice() (*Device, *k95LightingTestState) {
	state := &k95LightingTestState{value: lightingsettings.IndependentDeviceLightingState{SelectedEffect: "static", Brightness: 100}}
	effects := &k95LightingTestEffects{values: map[string]lightingsettings.EffectSettings{}}
	device := &Device{Serial: "k95-lighting-test", DeviceProfile: &DeviceProfile{}, RGBModes: append([]string(nil), rgbModes...)}
	device.lightingSource = independentK95LightingSource{deviceID: device.Serial, state: state, effects: effects, resolver: k95LightingTestResolver{settings: map[string]lightingsettings.EffectSettings{
		"static": {SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 1, Green: 2, Blue: 3}}},
		"wave":   {SchemaVersion: lightingsettings.SchemaVersion, EffectID: "wave", Speed: float64Pointer(4), TwoColor: &lightingsettings.TwoColorSettings{Start: lightingsettings.Color{Red: 1}, End: lightingsettings.Color{Blue: 2}}},
	}}}
	return device, state
}

func float64Pointer(value float64) *float64 { return &value }

func TestK95LightingSnapshotUsesCanonicalDefaultsAndKeyboardSpecialMode(t *testing.T) {
	device, state := newK95LightingTestDevice()
	snapshot, ok := device.LightingSnapshot()
	if !ok || snapshot.ConfiguredEffect != "static" || snapshot.Brightness != 100 || snapshot.SingleColorHex != "#010203" {
		t.Fatalf("default snapshot = %#v, ok=%t", snapshot, ok)
	}
	state.value = lightingsettings.IndependentDeviceLightingState{SelectedEffect: "keyboard", Brightness: 63}
	snapshot, ok = device.LightingSnapshot()
	if !ok || !snapshot.EffectSupported || snapshot.PaletteKind != "" || snapshot.HasSpeed || snapshot.Brightness != 63 {
		t.Fatalf("keyboard snapshot = %#v, ok=%t", snapshot, ok)
	}
	found := false
	for _, effect := range snapshot.SupportedEffects {
		if effect.ID == "keyboard" && effect.Label == "Keyboard" {
			found = true
		}
	}
	if !found {
		t.Fatal("keyboard is absent from supported effects")
	}
}

func TestK95LightingMutationsUseCanonicalStateAndRejectClusterOwnership(t *testing.T) {
	device, state := newK95LightingTestDevice()
	restarts := 0
	device.lightingRestart = func() { restarts++ }
	if err := device.SetLightingEffect("keyboard"); err != nil || state.value.SelectedEffect != "keyboard" {
		t.Fatalf("SetLightingEffect = %v, state = %#v", err, state.value)
	}
	if err := device.SetLightingBrightness(42); err != nil || state.value.Brightness != 42 {
		t.Fatalf("SetLightingBrightness = %v, state = %#v", err, state.value)
	}
	if restarts != 2 {
		t.Fatalf("restarts = %d, want 2", restarts)
	}
	if err := device.SetLightingEffect("static"); err != nil {
		t.Fatalf("select static: %v", err)
	}
	settings := lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 9, Green: 8, Blue: 7}}}
	if err := device.SetLightingEffectSettings("static", settings); err != nil {
		t.Fatalf("set effect settings: %v", err)
	}
	if err := device.ResetLightingEffectSettings("static"); err != nil {
		t.Fatalf("reset effect settings: %v", err)
	}
	device.DeviceProfile.RGBCluster = true
	if err := device.SetLightingEffect("static"); err == nil {
		t.Fatal("cluster-owned effect mutation succeeded")
	}
	if err := device.SetLightingBrightness(80); err == nil {
		t.Fatal("cluster-owned brightness mutation succeeded")
	}
}

func TestK95ResolveLightingEffectSettingsRejectsUnavailableCanonicalSource(t *testing.T) {
	device := &Device{Serial: "k95-lighting-test"}
	if _, err := device.ResolveLightingEffectSettings("static"); err == nil || err.Error() != "K95 canonical lighting source is unavailable" {
		t.Fatalf("ResolveLightingEffectSettings error = %v", err)
	}
}
