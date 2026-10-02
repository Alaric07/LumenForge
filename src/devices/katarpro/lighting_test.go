package katarpro

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"LumenForge/src/config"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"github.com/sstallion/go-hid"
)

func newKatarProLightingTestDevice(t *testing.T) *Device {
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
	profile := &DeviceProfile{ZoneColors: map[int]ZoneColors{0: {Name: "Scroll", ColorIndex: []int{0, 1, 2}, Color: &rgb.Color{Red: 200, Green: 100, Blue: 40, Brightness: 1}}}, Profiles: map[int]DPIProfile{3: {Sniper: true, Color: &rgb.Color{Red: 120, Green: 80, Blue: 40, Brightness: 1}}}, Active: true, Serial: "KatarProTEST", RGBProfile: "static", BrightnessSlider: &brightness, OriginalBrightness: 91, Path: filepath.Join(pwd, "database", "profiles", "KatarProTEST.json")}
	d := &Device{Serial: profile.Serial, dev: &hid.Device{}, LEDChannels: 1, Connected: true, MinDPI: minDpiValue, MaxDPI: maxDpiValue, Rgb: &profiles, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"default": profile}, lightingRestart: func() {}}
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
	d := newKatarProLightingTestDevice(t)
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
	d := newKatarProLightingTestDevice(t)
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
	d := newKatarProLightingTestDevice(t)
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
	d := newKatarProLightingTestDevice(t)
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
	d := newKatarProLightingTestDevice(t)
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
	d := newKatarProLightingTestDevice(t)
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
	d := newKatarProLightingTestDevice(t)
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
	d := newKatarProLightingTestDevice(t)
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
	d := newKatarProLightingTestDevice(t)
	brightness := uint8(37)
	gaming := *d.DeviceProfile
	gaming.RGBProfile, gaming.BrightnessSlider, gaming.RgbOff, gaming.Path, gaming.Active = "mouse", &brightness, true, filepath.Join(pwd, "database", "profiles", d.Serial+"-gaming.json"), false
	d.UserProfiles["gaming"] = &gaming
	// Stop existing transport calls; Mouse rendering has no background loop.
	d.Exit = true
	d.keyAssignmentFile = "/database/key-assignments/katarpro.json"
	if err := os.MkdirAll(filepath.Join(pwd, "database", "key-assignments"), 0700); err != nil {
		t.Fatal(err)
	}
	if d.ChangeDeviceProfile("gaming") != 1 {
		t.Fatal("profile switch failed")
	}
	d.Exit = false
	snapshot, ok := d.LightingSnapshot()
	_, effective, off := d.lightingOutputState()
	if !ok || snapshot.ConfiguredEffect != "mouse" || snapshot.Brightness != 37 || effective != 37 || off || d.DeviceProfile.RgbOff {
		t.Fatalf("snapshot=%#v effective=%d off=%t", snapshot, effective, off)
	}
}

func TestLightingRetainedGradientMutationsUseGuardAndPersistenceFirst(t *testing.T) {
	d := newKatarProLightingTestDevice(t)
	before := d.GetRgbProfile("gradient")
	status, added := d.ProcessNewGradientColor("gradient")
	if status != 1 || d.GetRgbProfile("gradient").Gradients[int(added)] != (rgb.Color{Green: 255, Blue: 255}) {
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
