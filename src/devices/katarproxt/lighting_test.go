package katarproxt

import (
	"LumenForge/src/common"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"LumenForge/src/config"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"github.com/sstallion/go-hid"
)

func newKatarProXTLightingTestDevice(t *testing.T) *Device {
	t.Helper()
	logger.Init()
	previous := pwd
	pwd = t.TempDir()
	t.Cleanup(func() { pwd = previous })
	for _, dir := range []string{"rgb", "profiles"} {
		if err := os.MkdirAll(filepath.Join(pwd, "database", dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile("../../../database/rgb.json")
	if err != nil {
		t.Fatal(err)
	}
	var profiles rgb.RGB
	if err = json.Unmarshal(data, &profiles); err != nil {
		t.Fatal(err)
	}
	profiles.Profiles["off"] = rgb.Profile{}
	brightness := uint8(63)
	profile := &DeviceProfile{ZoneColors: map[int]ZoneColors{0: {Name: "Scroll", ColorIndex: []int{0, 1, 2}, Color: &rgb.Color{Red: 200, Green: 100, Blue: 40, Brightness: 1}}}, Profile: 1, Profiles: map[int]DPIProfile{1: {Name: "Stage 2", Value: 1500, Color: &rgb.Color{Red: 255, Green: 255, Blue: 255, Brightness: 1}}, 3: {Name: "Sniper", Sniper: true, Color: &rgb.Color{Red: 120, Green: 80, Blue: 40, Brightness: 1}}}, Active: true, Serial: "KatarProXTTEST", RGBProfile: "static", BrightnessSlider: &brightness, OriginalBrightness: 91, Path: filepath.Join(pwd, "database", "profiles", "KatarProXTTEST.json")}
	d := &Device{Serial: profile.Serial, dev: &hid.Device{}, LEDChannels: 1, ChangeableLedChannels: 1, ZoneAmount: 1, Connected: true, MinDPI: minDpiValue, MaxDPI: maxDpiValue, Rgb: &profiles, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"default": profile}, lightingRestart: func() {}}
	if err = d.attachLightingRuntime(config.Paths{ShippedDatabaseRoot: "../../../database"}); err != nil {
		t.Fatal(err)
	}
	if err = d.persistLightingProfile(*profile); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(pwd, "database", "rgb", d.Serial+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestLightingSnapshotPublishedCatalogueAndActiveProfile(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	want := []string{"colorpulse", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	snapshot, ok := d.LightingSnapshot()
	var got []string
	for _, option := range snapshot.SupportedEffects {
		got = append(got, option.ID)
	}
	if !ok || !reflect.DeepEqual(got, want) || d.SupportsLightingEffect("custom") || snapshot.ConfiguredEffect != "static" || snapshot.Brightness != 63 || snapshot.AuthoredZoneEditor != nil || len(snapshot.Channels) != 0 || snapshot.ClusterControlled || snapshot.ExternalControlled {
		t.Fatalf("snapshot=%#v catalogue=%v", snapshot, got)
	}
	for _, effect := range want {
		d.DeviceProfile.RGBProfile = effect
		if snapshot, ok = d.LightingSnapshot(); !ok || !snapshot.EffectSupported {
			t.Fatalf("effect=%s snapshot=%#v", effect, snapshot)
		}
	}

	// Loading another active persisted profile changes the source used by snapshots.
	brightness := uint8(37)
	gaming := *d.DeviceProfile
	gaming.RGBProfile, gaming.BrightnessSlider, gaming.Path = "mouse", &brightness, filepath.Join(pwd, "database", "profiles", d.Serial+"-gaming.json")
	d.DeviceProfile.Active = false
	if err := d.persistLightingProfile(*d.DeviceProfile); err != nil {
		t.Fatal(err)
	}
	gaming.Active = true
	if err := d.persistLightingProfile(gaming); err != nil {
		t.Fatal(err)
	}
	d.loadDeviceProfiles()
	snapshot, ok = d.LightingSnapshot()
	if !ok || snapshot.ConfiguredEffect != "mouse" || snapshot.Brightness != 37 || snapshot.AuthoredZoneEditor == nil {
		t.Fatalf("active snapshot=%#v", snapshot)
	}
	for _, unsupported := range []string{"colorshift", "circle", "circleshift", "spinner", "liquid-temperature", "probe-temperature"} {
		if d.SupportsLightingEffect(unsupported) {
			t.Fatalf("unsupported %s", unsupported)
		}
	}
}

func TestLightingMutationsPersistExistingBackingAndRestart(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	restarts := 0
	d.lightingRestart = func() { restarts++ }
	if err := d.SetLightingEffect("cpu-temperature"); err != nil {
		t.Fatal(err)
	}
	before := *d.GetRgbProfile("cpu-temperature")
	settings, err := d.ResolveLightingEffectSettings("cpu-temperature")
	if err != nil {
		t.Fatal(err)
	}
	settings.Temperature.Low.Color.Red = 23
	settings.Temperature.Middle.Celsius = 42
	if err = d.SetLightingEffectSettings("cpu-temperature", settings); err != nil {
		t.Fatal(err)
	}
	after := d.GetRgbProfile("cpu-temperature")
	if after.MinTemp != before.MinTemp || after.MaxTemp != before.MaxTemp || after.Smoothness != before.Smoothness || after.Version != before.Version || after.Brightness != before.Brightness || after.StartColor.Brightness != before.StartColor.Brightness || after.StartColor.Red != 23 || after.MiddleColor.Temperature != 42 {
		t.Fatalf("profile before=%#v after=%#v", before, after)
	}
	data, _ := os.ReadFile(filepath.Join(pwd, "database", "rgb", d.Serial+".json"))
	var persisted rgb.RGB
	if err = json.Unmarshal(data, &persisted); err != nil || !reflect.DeepEqual(persisted.Profiles["cpu-temperature"], *after) {
		t.Fatalf("persisted=%#v err=%v", persisted, err)
	}
	if restarts != 2 {
		t.Fatalf("restarts=%d", restarts)
	}
	if err = d.ResetLightingEffectSettings("cpu-temperature"); err != nil {
		t.Fatal(err)
	}
	reset, _ := d.ResolveLightingEffectSettings("cpu-temperature")
	defaults, _ := d.lightingDefaults.Get("cpu-temperature")
	if !reflect.DeepEqual(reset, defaults) || d.GetRgbProfile("cpu-temperature").MinTemp != before.MinTemp {
		t.Fatal("reset changed renderer metadata or failed to restore defaults")
	}
	// Existing brightness behavior: static restarts, dynamic reads next frame.
	if err = d.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	count := restarts
	if d.ChangeDeviceBrightnessValue(44) != 1 || restarts != count+1 {
		t.Fatal("static brightness restart")
	}
	if err = d.SetLightingEffect("wave"); err != nil {
		t.Fatal(err)
	}
	count = restarts
	if err = d.SetLightingBrightness(29); err != nil || restarts != count {
		t.Fatal("dynamic brightness behavior")
	}
	if err = d.SetLightingEffect("mouse"); err != nil {
		t.Fatal(err)
	}
	count = restarts
	if err = d.SetLightingBrightness(33); err != nil || restarts != count+1 {
		t.Fatal("Mouse brightness restart")
	}
	if err = d.SetLightingEffect("wave"); err != nil {
		t.Fatal(err)
	}
	if err = d.SetLightingBrightness(29); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(d.DeviceProfile.Path)
	var profile DeviceProfile
	if err = json.Unmarshal(data, &profile); err != nil || profile.RGBProfile != "wave" || *profile.BrightnessSlider != 29 {
		t.Fatalf("persisted profile=%#v err=%v", profile, err)
	}
	// An inactive effect edit persists without restarting the selected effect.
	value, _ := d.ResolveLightingEffectSettings("static")
	value.SingleColor.Color.Red = 99
	count = restarts
	if err = d.SetLightingEffectSettings("static", value); err != nil || restarts != count {
		t.Fatal("inactive effect restarted output")
	}
}

func TestLightingTransientOverridesPreserveDesiredState(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	original, _ := os.ReadFile(d.DeviceProfile.Path)
	settings, _ := d.ResolveLightingEffectSettings("static")
	for i := 0; i < 2; i++ {
		if d.SchedulerBrightness(0) != 1 {
			t.Fatal("scheduler failed")
		}
	}
	d.ControlDeviceRgb(true)
	snapshot, _ := d.LightingSnapshot()
	effect, brightness, off := d.lightingOutputState()
	if snapshot.Brightness != 63 || snapshot.ConfiguredEffect != "static" || effect != "static" || brightness != 0 || !off || d.DeviceProfile.RgbOff || d.DeviceProfile.OriginalBrightness != 91 {
		t.Fatalf("snapshot=%#v brightness=%d off=%t", snapshot, brightness, off)
	}
	unchanged, _ := os.ReadFile(d.DeviceProfile.Path)
	if !reflect.DeepEqual(original, unchanged) {
		t.Fatal("transient override persisted")
	}
	if err := d.SetLightingBrightness(48); err != nil {
		t.Fatal(err)
	}
	_, brightness, _ = d.lightingOutputState()
	if brightness != 0 {
		t.Fatal("manual brightness broke darkness")
	}
	d.SchedulerBrightness(100)
	_, brightness, _ = d.lightingOutputState()
	if brightness != 0 {
		t.Fatal("scheduler clear broke user off")
	}
	d.ControlDeviceRgb(false)
	_, brightness, off = d.lightingOutputState()
	restored, _ := d.ResolveLightingEffectSettings("static")
	if brightness != 48 || off || !reflect.DeepEqual(settings, restored) {
		t.Fatal("desired state not restored")
	}
}

func TestLightingUnavailableAndPersistenceFailuresDoNotMutate(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	value, _ := d.ResolveLightingEffectSettings("static")
	value.SingleColor.Color.Red = 77
	restarts := 0
	d.lightingRestart = func() { restarts++ }
	beforeProfile := *d.DeviceProfile
	beforeRGB := *d.GetRgbProfile("static")
	profileData, _ := os.ReadFile(d.DeviceProfile.Path)
	rgbPath := filepath.Join(pwd, "database", "rgb", d.Serial+".json")
	rgbData, _ := os.ReadFile(rgbPath)
	defaults, device := d.lightingDefaults, d.dev
	for _, unavailable := range []string{"disconnected", "offline", "stopped", "runtime"} {
		d.lightingDefaults, d.dev, d.Exit, d.Connected = defaults, device, false, true
		switch unavailable {
		case "disconnected":
			d.dev = nil
		case "offline":
			d.Connected = false
		case "stopped":
			d.Exit = true
		case "runtime":
			d.lightingDefaults = nil
		}
		if _, ok := d.LightingSnapshot(); ok {
			t.Fatalf("%s snapshot available", unavailable)
		}
		if d.SetLightingEffect("wave") == nil || d.SetLightingBrightness(7) == nil || d.SetLightingEffectSettings("static", value) == nil || d.ResetLightingEffectSettings("static") == nil || d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{}) == nil {
			t.Fatalf("%s mutation accepted", unavailable)
		}
	}
	d.lightingDefaults, d.dev, d.Exit, d.Connected = defaults, device, false, true
	if d.SetLightingEffect("custom") == nil || d.SetLightingBrightness(101) == nil {
		t.Fatal("invalid mutation accepted")
	}
	afterProfile, _ := os.ReadFile(d.DeviceProfile.Path)
	afterRGB, _ := os.ReadFile(rgbPath)
	if restarts != 0 || !reflect.DeepEqual(beforeProfile, *d.DeviceProfile) || !reflect.DeepEqual(beforeRGB, *d.GetRgbProfile("static")) || !reflect.DeepEqual(profileData, afterProfile) || !reflect.DeepEqual(rgbData, afterRGB) {
		t.Fatal("rejected mutation changed backing state/output")
	}
	// Force deterministic open failures without relying on filesystem permissions.
	if err := os.Remove(d.DeviceProfile.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(d.DeviceProfile.Path, 0700); err != nil {
		t.Fatal(err)
	}
	if d.SetLightingEffect("wave") == nil || d.SetLightingBrightness(8) == nil {
		t.Fatal("profile persistence failure accepted")
	}
	if err := os.Remove(rgbPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(rgbPath, 0700); err != nil {
		t.Fatal(err)
	}
	if d.SetLightingEffectSettings("static", value) == nil || d.ResetLightingEffectSettings("static") == nil {
		t.Fatal("RGB persistence failure accepted")
	}
	if restarts != 0 || !reflect.DeepEqual(beforeProfile, *d.DeviceProfile) || !reflect.DeepEqual(beforeRGB, *d.GetRgbProfile("static")) {
		t.Fatal("failed persistence changed memory/output")
	}
}

func TestLightingConcurrentSnapshotsAndBrightnessOverrides(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	var workers sync.WaitGroup
	for worker := 0; worker < 6; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for n := 0; n < 10; n++ {
				switch worker {
				case 0:
					if _, ok := d.LightingSnapshot(); !ok {
						t.Error("snapshot unavailable")
					}
				case 1:
					if err := d.SetLightingBrightness(uint8(40 + n)); err != nil {
						t.Error(err)
					}
				case 2:
					d.SchedulerBrightness(0)
					d.ControlDeviceRgb(true)
					d.SchedulerBrightness(100)
					d.ControlDeviceRgb(false)
				case 3:
					value, err := d.ResolveLightingEffectSettings("static")
					if err != nil {
						t.Error(err)
						continue
					}
					value.SingleColor.Color.Red = float64(n)
					if err = d.SetLightingEffectSettings("static", value); err != nil {
						t.Error(err)
					}
				case 5:
					d.lightingOutputZones(50, true)
					d.lightingOutputState()
				case 4:
					_ = d.GetRgbProfile("static")
					_ = d.GetRgbProfiles()
				}
			}
		}(worker)
	}
	workers.Wait()
	_, brightness, off := d.lightingOutputState()
	if brightness != 49 || off {
		t.Fatalf("effective=%d off=%t", brightness, off)
	}
}

func TestLightingAttachmentFailureLeavesMouseUsable(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	d.lightingDefaults = nil
	if err := d.attachLightingRuntime(config.Paths{ShippedDatabaseRoot: t.TempDir()}); err == nil {
		t.Fatal("missing defaults accepted")
	}
	if _, ok := d.LightingSnapshot(); ok {
		t.Fatal("snapshot available")
	}
	if _, ok := d.DPISnapshot(); !ok {
		t.Fatal("DPI unavailable")
	}
}

func TestLightingEditableSpeedPaletteAndGradientRemainIndependent(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	wave, err := d.ResolveLightingEffectSettings("wave")
	if err != nil {
		t.Fatal(err)
	}
	*wave.Speed = 3
	wave.TwoColor.Start.Red, wave.TwoColor.End.Blue = 19, 27
	if err = d.SetLightingEffectSettings("wave", wave); err != nil {
		t.Fatal(err)
	}
	if got, err := d.ResolveLightingEffectSettings("wave"); err != nil || !reflect.DeepEqual(got, wave) {
		t.Fatalf("wave=%#v err=%v", got, err)
	}
	gradient, err := d.ResolveLightingEffectSettings("gradient")
	if err != nil {
		t.Fatal(err)
	}
	gradient.Gradient.Stops[0].Color.Red = 71
	gradient.Gradient.Stops[0].Intensity = 0.4
	if err = d.SetLightingEffectSettings("gradient", gradient); err != nil {
		t.Fatal(err)
	}
	if got, err := d.ResolveLightingEffectSettings("gradient"); err != nil || !reflect.DeepEqual(got, gradient) {
		t.Fatalf("gradient=%#v err=%v", got, err)
	}
	// Returned settings and renderer profiles must be defensive copies.
	gradient.Gradient.Stops[0].Color.Red = 255
	profile := d.GetRgbProfile("gradient")
	profile.Gradients[0] = rgb.Color{}
	if got, _ := d.ResolveLightingEffectSettings("gradient"); got.Gradient.Stops[0].Color.Red != 71 {
		t.Fatal("caller mutated backing settings")
	}
}

func TestMouseZoneMutationAndTransientSniperCopies(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	if err := d.SetLightingEffect("mouse"); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := d.LightingSnapshot()
	if !ok || snapshot.AuthoredZoneEditor == nil || len(snapshot.AuthoredZoneEditor.Zones) != 1 || snapshot.AuthoredZoneEditor.Zones[0].ID != "0" || snapshot.AuthoredZoneEditor.Zones[0].Label != "Scroll" {
		t.Fatalf("snapshot=%#v", snapshot)
	}
	authored := *d.DeviceProfile.ZoneColors[0].Color
	sniper := *d.DeviceProfile.Profiles[3].Color
	for _, active := range []bool{false, true, false} {
		d.SniperMode = active
		got := *d.lightingOutputZones(50, true)[0].Color
		want := authored
		if active {
			want = sniper
		}
		want.Brightness = 0.5
		if got != *rgb.ModifyBrightness(want) {
			t.Fatalf("render=%#v", got)
		}
	}
	if *d.DeviceProfile.ZoneColors[0].Color != authored || *d.DeviceProfile.Profiles[3].Color != sniper {
		t.Fatal("render mutated authored colors")
	}
	if err := d.SetLightingZoneColor("mouse", "zone", "0", "", rgb.Color{Red: 9, Green: 8, Blue: 7}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(d.DeviceProfile.Path)
	var persisted DeviceProfile
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	zone := persisted.ZoneColors[0]
	if zone.Color.Red != 9 || zone.Color.Green != 8 || zone.Color.Blue != 7 || !reflect.DeepEqual(zone.ColorIndex, []int{0, 1, 2}) || zone.Name != "Scroll" {
		t.Fatalf("persisted zone=%#v", zone)
	}
	before := *d.DeviceProfile.ZoneColors[0].Color
	d.Exit = true
	if d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{}) == nil || *d.DeviceProfile.ZoneColors[0].Color != before {
		t.Fatal("stopped zone mutation")
	}
	d.Exit = false
	if err = os.Remove(d.DeviceProfile.Path); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(d.DeviceProfile.Path, 0700); err != nil {
		t.Fatal(err)
	}
	if d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{}) == nil || *d.DeviceProfile.ZoneColors[0].Color != before {
		t.Fatal("failed zone persistence mutated state")
	}
}

func TestProfileSwitchRetiresStaleOffWithoutChangingDesiredBrightness(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	brightness := uint8(37)
	gaming := *d.DeviceProfile
	gaming.RGBProfile, gaming.BrightnessSlider, gaming.RgbOff, gaming.Path, gaming.Active = "mouse", &brightness, true, filepath.Join(pwd, "database", "profiles", d.Serial+"-gaming.json"), false
	d.UserProfiles["gaming"] = &gaming
	// Stop existing transport calls; Mouse rendering has no background loop.
	d.lightingProfileChanged = func() {}
	d.keyAssignmentFile = "/database/key-assignments/katarproxt.json"
	if err := os.MkdirAll(filepath.Join(pwd, "database", "key-assignments"), 0700); err != nil {
		t.Fatal(err)
	}
	if d.ChangeDeviceProfile("gaming") != 1 {
		t.Fatal("profile switch failed")
	}
	snapshot, ok := d.LightingSnapshot()
	_, effective, off := d.lightingOutputState()
	if !ok || snapshot.ConfiguredEffect != "mouse" || snapshot.Brightness != 37 || effective != 37 || off || d.DeviceProfile.RgbOff {
		t.Fatalf("snapshot=%#v effective=%d off=%t", snapshot, effective, off)
	}
}

func TestLightingRetainedGradientMutationsUseGuardAndPersistenceFirst(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	before := d.GetRgbProfile("gradient")
	status, added := d.ProcessNewGradientColor("gradient")
	if status != 1 || d.GetRgbProfile("gradient").Gradients[int(added)].Brightness != 1 {
		t.Fatal("gradient addition changed")
	}
	status, deleted := d.ProcessDeleteGradientColor("gradient")
	if status != 1 || added != deleted || !reflect.DeepEqual(d.GetRgbProfile("gradient"), before) {
		t.Fatal("gradient removal changed")
	}
	d.Connected = false
	if status, _ := d.ProcessNewGradientColor("gradient"); status != 0 {
		t.Fatal("offline addition accepted")
	}
	d.Connected = true
	path := filepath.Join(pwd, "database", "rgb", d.Serial+".json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if status, _ := d.ProcessNewGradientColor("gradient"); status != 0 || !reflect.DeepEqual(d.GetRgbProfile("gradient"), before) {
		t.Fatal("failed gradient persistence changed memory")
	}
}

func TestXTInjectedPersistenceFailureAndPublicationOrdering(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	active := d.DeviceProfile
	pointer := active.ZoneColors[0].Color
	before := cloneXTProfile(*active)
	rgbBefore := d.GetRgbProfile("gradient")
	restarts, outputs := 0, 0
	d.lightingRestart = func() { restarts++ }
	d.lightingReport = func([]byte) error { outputs++; return nil }
	injected := errors.New("injected write failure")
	d.lightingWriteJSON = func(path string, value interface{}) error {
		if d.DeviceProfile != active || active.ZoneColors[0].Color != pointer || !reflect.DeepEqual(before, *active) || !reflect.DeepEqual(rgbBefore, d.GetRgbProfileForTestOnly("gradient")) {
			t.Fatal("published before persistence")
		}
		return injected
	}
	// The writer runs under rgbMutex for effect writes; inspection of the live map
	// here deliberately does not recursively lock the mutex.
	settings, _ := d.ResolveLightingEffectSettings("gradient")
	settings.Gradient.Stops[0].Color.Red = 55
	mutations := []func() error{
		func() error { return d.SetLightingEffect("mouse") },
		func() error { return d.SetLightingBrightness(17) },
		func() error { return d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 7}) },
		func() error { return d.SetLightingEffectSettings("gradient", settings) },
		func() error { return d.ResetLightingEffectSettings("gradient") },
	}
	for _, mutate := range mutations {
		if !errors.Is(mutate(), injected) {
			t.Fatal("write failure hidden")
		}
	}
	if status, _ := d.ProcessNewGradientColor("gradient"); status != 0 {
		t.Fatal("gradient failure hidden")
	}
	if d.ChangeDeviceProfile("default") != 0 {
		t.Fatal("switch write failure hidden")
	}
	if d.ChangeDeviceBrightness(2) != 0 {
		t.Fatal("metadata failure hidden")
	}
	if d.DeviceProfile != active || active.ZoneColors[0].Color != pointer || !reflect.DeepEqual(before, *active) || !reflect.DeepEqual(rgbBefore, d.GetRgbProfile("gradient")) || restarts != 0 || outputs != 0 {
		t.Fatal("failed mutation changed state or output")
	}
	d.lightingWriteJSON = func(path string, value interface{}) error {
		if d.DeviceProfile != active || *active.BrightnessSlider != 63 {
			t.Fatal("brightness published before write")
		}
		return common.SaveJsonData(path, value)
	}
	if err := d.SetLightingBrightness(17); err != nil {
		t.Fatal(err)
	}
	if *active.BrightnessSlider != 17 || restarts != 1 {
		t.Fatal("successful write did not publish/refresh")
	}
}

// Only for inspection from the injected writer while its caller owns rgbMutex.
func (d *Device) GetRgbProfileForTestOnly(effect string) *rgb.Profile {
	value := copyLightingRGBProfile(d.Rgb.Profiles[effect])
	return &value
}

func TestXTOutputDispatchAndExactHIDFrame(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	reports := make(chan []byte, 16)
	d.lightingReport = func(report []byte) error {
		select {
		case reports <- report:
		default:
		}
		return nil
	}
	d.lightingRestart = nil
	want := []string{"colorpulse", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	if !reflect.DeepEqual(rgbModes, want) {
		t.Fatal("catalogue drift")
	}
	for _, effect := range want {
		// All mutations and rendering are exercised on XT, with HID replaced only at
		// the fully framed report boundary.
		if err := d.SetLightingEffect(effect); err != nil {
			t.Fatal(effect, err)
		}
		select {
		case report := <-reports:
			if len(report) != 65 || !bytes.Equal(report[:8], []byte{0, 8, 6, 0, 3, 0, 0, 0}) || !bytes.Equal(report[11:], make([]byte, 54)) {
				t.Fatalf("%s report=%v", effect, report)
			}
			if effect == "mouse" && !bytes.Equal(report[8:11], []byte{126, 63, 25}) {
				t.Fatalf("Mouse RGB=%v", report[8:11])
			}
			if effect == "off" && !bytes.Equal(report[8:11], []byte{0, 0, 0}) {
				t.Fatal("off frame")
			}
		case <-time.After(time.Second):
			t.Fatal("no hardware dispatch", effect)
		}
		d.rendererMu.Lock()
		done := d.rendererDone
		d.rendererMu.Unlock()
		d.stopLighting()
		if done != nil {
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("renderer did not retire", effect)
			}
		}
		for len(reports) > 0 {
			<-reports
		}
	}
}

func TestXTDPIFlashAndMouseOnlySniperComposition(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	before := cloneXTProfile(*d.DeviceProfile)
	color := d.DeviceProfile.Profiles[1].Color
	if !bytes.Equal(d.dpiFlashFrame(d.DeviceProfile.Profiles[1]), []byte{255, 255, 255}) || *color != *before.Profiles[1].Color {
		t.Fatal("DPI flash mutated input or scaled feedback")
	}
	d.SniperMode = true
	d.DeviceProfile.Profiles[3] = DPIProfile{Name: "Sniper", Sniper: true, Color: nil}
	if got := d.lightingOutputZones(50, true)[0].Color; got.Red != 100 || got.Green != 50 {
		t.Fatal("missing transient Sniper did not retain authored Mouse color")
	}
	d.DeviceProfile.Profiles[3] = before.Profiles[3]
	outputs := [][]byte{}
	d.lightingReport = func(report []byte) error { outputs = append(outputs, append([]byte(nil), report[8:11]...)); return nil }
	d.lightingRestart = nil
	if err := d.SetLightingEffect("mouse"); err != nil {
		t.Fatal(err)
	}
	waitXTFrame(t, d)
	if !bytes.Equal(outputs[len(outputs)-1], []byte{75, 50, 25}) {
		t.Fatal("Mouse Sniper color not composed")
	}
	if err := d.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	pf := d.GetRgbProfile("static")
	pf.StartColor.Brightness = .63
	scaled := rgb.ModifyBrightness(pf.StartColor)
	waitXTFrame(t, d)
	if !bytes.Equal(outputs[len(outputs)-1], []byte{byte(scaled.Red), byte(scaled.Green), byte(scaled.Blue)}) {
		t.Fatal("Sniper overlaid Static")
	}
	if *d.DeviceProfile.ZoneColors[0].Color != *before.ZoneColors[0].Color || *d.DeviceProfile.Profiles[3].Color != *before.Profiles[3].Color {
		t.Fatal("output altered authored state")
	}
	// Exercise the real toggle path: it retains active regular stage and writes
	// an unscaled flash even during off/dark, then restores the selected output.
	d.ControlDeviceRgb(true)
	d.SchedulerBrightness(0)
	waitXTFrame(t, d)
	var control [][]byte
	d.lightingReport = func(report []byte) error { control = append(control, append([]byte(nil), report...)); return nil }
	start := time.Now()
	d.toggleDPI(false)
	waitXTFrame(t, d)
	if elapsed := time.Since(start); elapsed < 500*time.Millisecond {
		t.Fatal("DPI flash interval shortened")
	}
	if len(control) != 4 || !bytes.Equal(control[0][2:7], []byte{1, 0x21, 0, 0xdc, 5}) || !bytes.Equal(control[1][2:7], []byte{1, 0x22, 0, 0xdc, 5}) || !bytes.Equal(control[2][8:11], []byte{255, 255, 255}) || !bytes.Equal(control[3][8:11], []byte{0, 0, 0}) {
		t.Fatalf("DPI feedback packets=%v", control)
	}
	if d.DeviceProfile.Profile != 1 || !d.SniperMode || *d.DeviceProfile.ZoneColors[0].Color != *before.ZoneColors[0].Color {
		t.Fatal("DPI feedback changed authored selection/input state")
	}
}

func TestXTProfileSwitchCandidatesDefaultsAndFailures(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	target := cloneXTProfile(*d.DeviceProfile)
	target.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-old.json")
	target.Active = false
	target.RGBProfile = "mouse"
	target.BrightnessSlider = nil
	target.PollingRate = 0
	target.RgbOff = true
	target.Profiles = map[int]DPIProfile{0: d.DeviceProfile.Profiles[1], 1: d.DeviceProfile.Profiles[1], 2: d.DeviceProfile.Profiles[1]}
	target.Label = "preserve label"
	target.ButtonOptimization = 1
	target.KeyAssignmentHash = "existing-input"
	d.UserProfiles["old"] = &target
	old := d.DeviceProfile
	before := cloneXTProfile(*old)
	targetBefore := cloneXTProfile(target)
	restarts, input := 0, 0
	d.lightingRestart = func() { restarts++ }
	d.lightingProfileChanged = func() { input++ }
	fail := errors.New("target write failed")
	d.lightingWriteJSON = func(path string, value interface{}) error {
		if d.DeviceProfile != old || !reflect.DeepEqual(*old, before) || !reflect.DeepEqual(target, targetBefore) {
			t.Fatal("switch published before writes")
		}
		if path == target.Path {
			return fail
		}
		return common.SaveJsonData(path, value)
	}
	if d.ChangeDeviceProfile("old") != 0 || d.DeviceProfile != old || restarts != 0 || input != 0 {
		t.Fatal("failed switch changed profile/output")
	}
	data, _ := os.ReadFile(old.Path)
	var disk DeviceProfile
	if err := json.Unmarshal(data, &disk); err != nil || !disk.Active {
		t.Fatal("failed switch left old disk profile inactive")
	}
	d.lightingWriteJSON = nil
	d.ControlDeviceRgb(true)
	d.SchedulerBrightness(0)
	restarts = 0
	d.SniperMode = true
	if d.ChangeDeviceProfile("old") != 1 {
		t.Fatal("switch failed")
	}
	snapshot, ok := d.LightingSnapshot()
	_, effective, off := d.lightingOutputState()
	if !ok || snapshot.ConfiguredEffect != "mouse" || snapshot.Brightness != 100 || effective != 0 || off || d.DeviceProfile.PollingRate != 4 || d.DeviceProfile.Profile != 1 || !d.SniperMode || d.DeviceProfile.Label != "preserve label" || d.DeviceProfile.KeyAssignmentHash != "existing-input" || restarts != 1 || input != 1 {
		t.Fatal("switch defaults/identity/overrides")
	}
	sniper := d.DeviceProfile.Profiles[3] // exact legacy len(map) insertion rule
	if !sniper.Sniper || sniper.Value != 200 || sniper.Color.Hex != "#ffff00" || !reflect.DeepEqual(sniper.ColorIndex[0], []int{0, 1, 2}) {
		t.Fatal("XT Sniper upgrade changed")
	}
}

func TestXTDefaultCreationAndNoBorrowedCapabilities(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	d.DeviceProfile = nil
	d.UserProfiles = nil
	d.lightingDefaults = nil
	d.saveDeviceProfile()
	p := d.DeviceProfile
	if p == nil || p.RGBProfile != "mouse" || *p.BrightnessSlider != 100 || p.OriginalBrightness != 100 || p.PollingRate != 4 || p.Profile != 1 || p.Profiles[0].Value != 800 || p.Profiles[1].Value != 1500 || p.Profiles[2].Value != 3000 || p.Profiles[3].Value != 200 || p.ZoneColors[0].Color.Hex != "#00ffff" {
		t.Fatal("XT defaults drift")
	}
	for _, name := range []string{"ProcessSetRgbCluster", "ProcessSetOpenRgbIntegration", "WriteColorEx"} {
		if reflect.ValueOf(d).MethodByName(name).IsValid() {
			t.Fatal("invented external ownership", name)
		}
	}
}

func TestXTAttachmentIsAtomicAndLegacyRemainsUsable(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	d.lightingDefaults = nil
	d.DeviceProfile.RgbOff = true
	d.rgbMutex.Lock()
	delete(d.Rgb.Profiles, "wave")
	d.rgbMutex.Unlock()
	old := cloneXTProfile(*d.DeviceProfile)
	if d.attachLightingRuntime(config.Paths{ShippedDatabaseRoot: "../../../database"}) == nil || d.lightingDefaults != nil || !reflect.DeepEqual(old, *d.DeviceProfile) {
		t.Fatal("partial attachment")
	}
	d.lightingRestart = nil
	// Canonical topology checks must not be imposed on fallback composition.
	zone := d.DeviceProfile.ZoneColors[0]
	zone.Name = "legacy label"
	d.DeviceProfile.ZoneColors[0] = zone
	writes := 0
	d.lightingReport = func([]byte) error { writes++; return nil }
	if d.ChangeDeviceBrightnessValue(51) != 1 || writes != 1 || d.DeviceProfile.RGBProfile != "static" {
		t.Fatal("legacy fallback rejected normal mutation")
	}
	if _, ok := d.LightingSnapshot(); ok {
		t.Fatal("unattached snapshot")
	}
}

func TestXTRendererRetryRetirementAndBoundedRestart(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	reports := make(chan []byte, 64)
	d.lightingReport = func(data []byte) error {
		select {
		case reports <- data:
		default:
		}
		return nil
	}
	d.lightingRestart = nil
	if err := d.SetLightingEffect("wave"); err != nil {
		t.Fatal(err)
	}
	d.rendererMu.Lock()
	firstDone := d.rendererDone
	firstStop := d.rendererStop
	d.rendererMu.Unlock()
	select {
	case <-reports:
	case <-time.After(time.Second):
		t.Fatal("wave renderer")
	}
	profile := d.GetRgbProfile("wave")
	d.rgbMutex.Lock()
	delete(d.Rgb.Profiles, "wave")
	d.rgbMutex.Unlock()
	time.Sleep(85 * time.Millisecond)
	select {
	case <-firstDone:
		t.Fatal("temporary missing effect killed renderer")
	default:
	}
	d.rgbMutex.Lock()
	d.Rgb.Profiles["wave"] = *profile
	d.rgbMutex.Unlock()
	for len(reports) > 0 {
		<-reports
	}
	select {
	case <-reports:
	case <-time.After(time.Second):
		t.Fatal("renderer did not recover")
	}
	d.stopLighting()
	d.stopLighting()
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("unbuffered stop hang")
	}
	select {
	case <-firstStop:
	default:
		t.Fatal("correct stop channel not retired")
	}
	// A previously dead goroutine must never make a future mutation wait for Exit.
	done := make(chan struct{})
	go func() { _ = d.SetLightingEffect("mouse"); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("restart blocked on retired owner")
	}
	waitXTFrame(t, d)
	d.rendererMu.Lock()
	active := d.activeRgb
	d.rendererMu.Unlock()
	if active != nil {
		t.Fatal("Mouse left a dead activeRgb")
	}
}

func TestXTCanonicalTopologyGuardsAndOfflineOverrides(t *testing.T) {
	cases := []struct {
		name         string
		breakBacking func(*Device)
	}{
		{"channels", func(d *Device) { d.ChangeableLedChannels = 2 }},
		{"zones", func(d *Device) { d.ZoneAmount = 2 }},
		{"zone name", func(d *Device) {
			zone := d.DeviceProfile.ZoneColors[0]
			zone.Name = "Logo"
			d.DeviceProfile.ZoneColors[0] = zone
		}},
		{"indices", func(d *Device) {
			zone := d.DeviceProfile.ZoneColors[0]
			zone.ColorIndex = []int{2, 1, 0}
			d.DeviceProfile.ZoneColors[0] = zone
		}},
		{"color", func(d *Device) {
			zone := d.DeviceProfile.ZoneColors[0]
			zone.Color = nil
			d.DeviceProfile.ZoneColors[0] = zone
		}},
		{"brightness", func(d *Device) { d.DeviceProfile.BrightnessSlider = nil }},
		{"RGB map", func(d *Device) { d.Rgb.Profiles = nil }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			d := newKatarProXTLightingTestDevice(t)
			test.breakBacking(d)
			writes, restarts := 0, 0
			d.lightingWriteJSON = func(string, interface{}) error { writes++; return nil }
			d.lightingRestart = func() { restarts++ }
			if _, ok := d.LightingSnapshot(); ok || d.SetLightingBrightness(20) == nil || d.SetLightingEffect("mouse") == nil || d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{}) == nil || d.ChangeDeviceProfile("default") != 0 || writes != 0 || restarts != 0 {
				t.Fatal("unavailable authored mutation accepted")
			}
		})
	}
	d := newKatarProXTLightingTestDevice(t)
	restarts := 0
	d.lightingRestart = func() { restarts++ }
	d.Exit = true
	d.Connected = false
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(true)
	if !d.schedulerDark || !d.userRGBOff || restarts != 0 || *d.DeviceProfile.BrightnessSlider != 63 {
		t.Fatal("recording override improperly required immediate output")
	}
	d.SchedulerBrightness(255)
	d.ControlDeviceRgb(false)
	if d.schedulerDark || d.userRGBOff || restarts != 0 {
		t.Fatal("clear override emitted unavailable output")
	}
}

func TestXTRendererAndMutationsShareCopiesWithoutMapRaces(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	d.lightingReport = func([]byte) error { return nil }
	d.lightingRestart = nil
	if err := d.SetLightingEffect("gradient"); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for n := 0; n < 12; n++ {
				switch worker {
				case 0:
					_, _ = d.LightingSnapshot()
					_ = d.GetRgbProfiles()
				case 1:
					if err := d.SetLightingBrightness(uint8(30 + n)); err != nil {
						t.Error(err)
					}
				case 2:
					settings, err := d.ResolveLightingEffectSettings("gradient")
					if err != nil {
						t.Error(err)
						continue
					}
					settings.Gradient.Stops[0].Color.Red = float64(n)
					if err = d.SetLightingEffectSettings("gradient", settings); err != nil {
						t.Error(err)
					}
				case 3:
					d.SchedulerBrightness(0)
					d.SchedulerBrightness(255)
				}
			}
		}(worker)
	}
	workers.Wait()
	d.rendererMu.Lock()
	done := d.rendererDone
	d.rendererMu.Unlock()
	d.stopLighting()
	if done != nil {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("renderer retirement")
		}
	}
}

func waitXTFrame(t *testing.T, d *Device) {
	t.Helper()
	d.rendererMu.Lock()
	done := d.rendererDone
	d.rendererMu.Unlock()
	if done != nil {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("one-shot frame did not retire")
		}
	}
}

func TestXTRestartDoesNotWaitForStalledPriorWrite(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	d.lightingReport = func([]byte) error { once.Do(func() { close(entered) }); <-release; return nil }
	d.lightingRestart = nil
	if err := d.SetLightingEffect("mouse"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("prior output did not start")
	}
	d.rendererMu.Lock()
	priorDone := d.rendererDone
	d.rendererMu.Unlock()
	mutation := make(chan error, 1)
	go func() { mutation <- d.SetLightingEffect("static") }()
	select {
	case err := <-mutation:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("restart waited for old HID write")
	}
	d.rendererMu.Lock()
	latestDone := d.rendererDone
	d.rendererMu.Unlock()
	d.stopLighting()
	// Release original transport, then any admitted later write uses a hook which
	// does not close the once-only entry channel. The report callback is protected
	// by the transport mutex, so replacement while old callback runs is unsafe.
	close(release)
	select {
	case <-priorDone:
	case <-time.After(time.Second):
		t.Fatal("prior renderer retirement")
	}
	select {
	case <-latestDone:
	case <-time.After(time.Second):
		t.Fatal("latest renderer retirement")
	}
	d.rendererMu.Lock()
	active := d.activeRgb
	d.rendererMu.Unlock()
	if active != nil {
		t.Fatal("late renderer cleanup left stale ownership")
	}
}

func TestXTRejectsSparseCanonicalGradientWithoutChangingFallback(t *testing.T) {
	d := newKatarProXTLightingTestDevice(t)
	profile := d.GetRgbProfile("gradient")
	profile.Gradients[len(profile.Gradients)] = profile.Gradients[0]
	delete(profile.Gradients, 0)
	if _, err := xtEffectSettingsFromProfile("gradient", *profile); err == nil {
		t.Fatal("sparse renderer palette accepted")
	}
	d.lightingDefaults = nil
	d.Rgb.Profiles["gradient"] = *profile
	if d.attachLightingRuntime(config.Paths{ShippedDatabaseRoot: "../../../database"}) == nil || d.lightingDefaults != nil {
		t.Fatal("sparse gradient attachment published")
	}
}
