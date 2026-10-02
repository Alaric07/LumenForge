package xc7

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

func newXC7LightingTestDevice(t *testing.T) *Device {
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
	profile := &DeviceProfile{Active: true, Serial: "XC7TEST", RGBProfile: "static", BrightnessSlider: &brightness, OriginalBrightness: 91, Path: filepath.Join(pwd, "database", "profiles", "XC7TEST.json")}
	d := &Device{Serial: profile.Serial, dev: &hid.Device{}, LEDChannels: 31, Rgb: &profiles, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"default": profile}, lightingRestart: func() {}}
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
	d := newXC7LightingTestDevice(t)
	want := []string{"circle", "circleshift", "colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "liquid-temperature", "off", "rainbow", "pastelrainbow", "rotator", "spinner", "static", "storm", "watercolor", "wave"}
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
	// Active-profile switching remains the authority; stale off flags are retired.
	brightness := uint8(37)
	d.UserProfiles["gaming"] = &DeviceProfile{Serial: d.Serial, RGBProfile: "liquid-temperature", BrightnessSlider: &brightness, RgbOff: true, Path: filepath.Join(pwd, "database", "profiles", "XC7TEST-gaming.json")}
	if d.ChangeDeviceProfile("gaming") != 1 {
		t.Fatal("switch failed")
	}
	snapshot, ok = d.LightingSnapshot()
	_, effective, off := d.lightingOutputState()
	if !ok || snapshot.ConfiguredEffect != "liquid-temperature" || snapshot.Brightness != 37 || !snapshot.HasTemperature || effective != 37 || off || d.DeviceProfile.RgbOff {
		t.Fatalf("switched=%#v effective=%d off=%t", snapshot, effective, off)
	}
}

func TestLightingMutationsPersistExistingBackingAndRestart(t *testing.T) {
	d := newXC7LightingTestDevice(t)
	restarts := 0
	d.lightingRestart = func() { restarts++ }
	if err := d.SetLightingEffect("liquid-temperature"); err != nil {
		t.Fatal(err)
	}
	before := *d.GetRgbProfile("liquid-temperature")
	settings, err := d.ResolveLightingEffectSettings("liquid-temperature")
	if err != nil {
		t.Fatal(err)
	}
	settings.Temperature.Low.Color.Red = 23
	settings.Temperature.Middle.Celsius = 42
	if err = d.SetLightingEffectSettings("liquid-temperature", settings); err != nil {
		t.Fatal(err)
	}
	after := d.GetRgbProfile("liquid-temperature")
	if after.MinTemp != before.MinTemp || after.MaxTemp != before.MaxTemp || after.Smoothness != before.Smoothness || after.Version != before.Version || after.Brightness != before.Brightness || after.StartColor.Brightness != before.StartColor.Brightness || after.StartColor.Red != 23 || after.MiddleColor.Temperature != 42 {
		t.Fatalf("profile before=%#v after=%#v", before, after)
	}
	data, _ := os.ReadFile(filepath.Join(pwd, "database", "rgb", d.Serial+".json"))
	var persisted rgb.RGB
	if err = json.Unmarshal(data, &persisted); err != nil || !reflect.DeepEqual(persisted.Profiles["liquid-temperature"], *after) {
		t.Fatalf("persisted=%#v err=%v", persisted, err)
	}
	if restarts != 2 {
		t.Fatalf("restarts=%d", restarts)
	}
	if err = d.ResetLightingEffectSettings("liquid-temperature"); err != nil {
		t.Fatal(err)
	}
	reset, _ := d.ResolveLightingEffectSettings("liquid-temperature")
	defaults, _ := d.lightingDefaults.Get("liquid-temperature")
	if !reflect.DeepEqual(reset, defaults) || d.GetRgbProfile("liquid-temperature").MinTemp != before.MinTemp {
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
	d.GlobalBrightness = 1
	if d.ChangeDeviceBrightnessValue(22) != 2 || d.SetLightingBrightness(22) == nil || *d.DeviceProfile.BrightnessSlider != 44 {
		t.Fatal("global brightness guard")
	}
	d.GlobalBrightness = 0
	if err = d.SetLightingEffect("wave"); err != nil {
		t.Fatal(err)
	}
	count = restarts
	if err = d.SetLightingBrightness(29); err != nil || restarts != count {
		t.Fatal("dynamic brightness behavior")
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
	d := newXC7LightingTestDevice(t)
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
	d := newXC7LightingTestDevice(t)
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
	for _, unavailable := range []string{"disconnected", "stopped", "runtime"} {
		d.lightingDefaults, d.dev, d.Exit = defaults, device, false
		switch unavailable {
		case "disconnected":
			d.dev = nil
		case "stopped":
			d.Exit = true
		case "runtime":
			d.lightingDefaults = nil
		}
		if _, ok := d.LightingSnapshot(); ok {
			t.Fatalf("%s snapshot available", unavailable)
		}
		if d.SetLightingEffect("wave") == nil || d.SetLightingBrightness(7) == nil || d.SetLightingEffectSettings("static", value) == nil || d.ResetLightingEffectSettings("static") == nil {
			t.Fatalf("%s mutation accepted", unavailable)
		}
	}
	d.lightingDefaults, d.dev, d.Exit = defaults, device, false
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
	d := newXC7LightingTestDevice(t)
	var workers sync.WaitGroup
	for worker := 0; worker < 5; worker++ {
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

func TestLightingAttachmentFailureLeavesOtherSnapshotsUsable(t *testing.T) {
	d := newXC7LightingTestDevice(t)
	d.lightingDefaults = nil
	d.HasLCD = true
	d.LCDModes = map[int]string{0: "Liquid Temperature"}
	d.LCDRotations = map[int]string{0: "default"}
	d.TemperatureString = "31.8°C"
	if err := d.attachLightingRuntime(config.Paths{ShippedDatabaseRoot: t.TempDir()}); err == nil {
		t.Fatal("missing runtime defaults accepted")
	}
	if _, ok := d.LightingSnapshot(); ok {
		t.Fatal("lighting did not fail closed")
	}
	if _, ok := d.DisplaySnapshot(); !ok {
		t.Fatal("display unavailable")
	}
	if snapshot, ok := d.TelemetrySnapshot(); !ok || len(snapshot.Rows) != 1 || snapshot.Rows[0].Label != "Liquid Temperature" {
		t.Fatal("telemetry changed")
	}
}

func TestLightingEditableSpeedPaletteAndGradientRemainIndependent(t *testing.T) {
	d := newXC7LightingTestDevice(t)
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
	// Retained add/remove mutations keep their original key and color semantics.
	before := d.GetRgbProfile("gradient")
	status, added := d.ProcessNewGradientColor("gradient")
	if status != 1 || d.GetRgbProfile("gradient").Gradients[int(added)] != (rgb.Color{Green: 255, Blue: 255}) {
		t.Fatal("gradient addition semantics changed")
	}
	status, deleted := d.ProcessDeleteGradientColor("gradient")
	if status != 1 || deleted != added || !reflect.DeepEqual(d.GetRgbProfile("gradient"), before) {
		t.Fatal("gradient deletion semantics changed")
	}
	d.dev = nil
	if status, _ = d.ProcessNewGradientColor("gradient"); status != 0 {
		t.Fatal("offline gradient mutation accepted")
	}
}
