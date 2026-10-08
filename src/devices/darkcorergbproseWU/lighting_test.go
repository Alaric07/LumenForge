package darkcorergbproseWU

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

func newDarkCoreLightingTestDevice(t *testing.T) *Device {
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
	zones := make(map[int]ZoneColors)
	for id, name := range []string{"Scroll", "Logo", "Side Accent 1", "Side Accent 2", "Side Accent 3", "Side Accent 4", "Side Accent 5", "Side Accent 6"} {
		indices := [8][3]int{{0, 12, 24}, {6, 18, 30}, {1, 13, 25}, {2, 14, 26}, {3, 15, 27}, {4, 16, 28}, {5, 17, 29}, {7, 19, 31}}
		zones[id] = ZoneColors{Name: name, ColorIndex: append([]int(nil), indices[id][:]...), Color: &rgb.Color{Red: 200, Green: 100, Blue: 40, Brightness: 1}}
	}
	profile := &DeviceProfile{ZoneColors: zones, DPIColor: &rgb.Color{Red: 255, Brightness: 1}, Profiles: map[int]DPIProfile{0: {Name: "Stage 1", Value: 800, ColorIndex: map[int][]int{0: {8, 20, 32}}}, 1: {Name: "Sniper", Value: 400, Sniper: true, ColorIndex: map[int][]int{0: {9, 21, 33}}}}, Active: true, Serial: "DarkCoreTEST", RGBProfile: "static", BrightnessSlider: &brightness, OriginalBrightness: 91, Path: filepath.Join(pwd, "database", "profiles", "DarkCoreTEST.json")}

	d := &Device{Serial: profile.Serial, dev: &hid.Device{}, LEDChannels: 12, ChangeableLedChannels: 12, Connected: true, DPIAmount: 2, MinDPI: minDpiValue, MaxDPI: maxDpiValue, Rgb: &profiles, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"default": profile}, lightingRestart: func() {}}
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
	d := newDarkCoreLightingTestDevice(t)
	want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
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
	for _, unsupported := range []string{"circle", "circleshift", "spinner", "liquid-temperature", "probe-temperature"} {
		if d.SupportsLightingEffect(unsupported) {
			t.Fatalf("unsupported %s", unsupported)
		}
	}
}

func TestLightingMutationsPersistExistingBackingAndRestart(t *testing.T) {
	d := newDarkCoreLightingTestDevice(t)
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
	d := newDarkCoreLightingTestDevice(t)
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
	d := newDarkCoreLightingTestDevice(t)
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
	if d.SetLightingEffect("wave") == nil || d.SetLightingBrightness(8) == nil || d.SetLightingZoneColors("mouse", []string{"0", "7"}, rgb.Color{}) == nil {
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
	d := newDarkCoreLightingTestDevice(t)
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
	d := newDarkCoreLightingTestDevice(t)
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
	d := newDarkCoreLightingTestDevice(t)
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
	d := newDarkCoreLightingTestDevice(t)
	if err := d.SetLightingEffect("mouse"); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := d.LightingSnapshot()
	if !ok || snapshot.AuthoredZoneEditor == nil || len(snapshot.AuthoredZoneEditor.Zones) != 8 || snapshot.AuthoredZoneEditor.Zones[0].ID != "0" || snapshot.AuthoredZoneEditor.Zones[0].Label != "Scroll" {
		t.Fatalf("snapshot=%#v", snapshot)
	}
	authored := *d.DeviceProfile.ZoneColors[0].Color
	for _, active := range []bool{false, true, false} {
		d.SniperMode = active
		got := *d.lightingOutputZones(50, true)[0].Color
		want := authored
		want.Brightness = 0.5
		if got != *rgb.ModifyBrightness(want) {
			t.Fatalf("render=%#v", got)
		}
	}
	if *d.DeviceProfile.ZoneColors[0].Color != authored {
		t.Fatal("output mutated authored color")
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
	if zone.Color.Red != 9 || zone.Color.Green != 8 || zone.Color.Blue != 7 || !reflect.DeepEqual(zone.ColorIndex, []int{0, 12, 24}) || zone.Name != "Scroll" {
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
	d := newDarkCoreLightingTestDevice(t)
	brightness := uint8(37)
	gaming := *d.DeviceProfile
	gaming.RGBProfile, gaming.BrightnessSlider, gaming.RgbOff, gaming.Path, gaming.Active = "mouse", &brightness, true, filepath.Join(pwd, "database", "profiles", d.Serial+"-gaming.json"), false
	d.UserProfiles["gaming"] = &gaming
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(true)
	if d.switchLightingProfile("gaming") != nil {
		t.Fatal("profile switch failed")
	}

	snapshot, ok := d.LightingSnapshot()
	_, effective, off := d.lightingOutputState()
	if !ok || snapshot.ConfiguredEffect != "mouse" || snapshot.Brightness != 37 || effective != 37 || off || d.DeviceProfile.RgbOff {
		t.Fatalf("snapshot=%#v effective=%d off=%t", snapshot, effective, off)
	}
}

func TestLightingRetainedGradientMutationsUseGuardAndPersistenceFirst(t *testing.T) {
	d := newDarkCoreLightingTestDevice(t)
	before := d.GetRgbProfile("gradient")
	count := 0
	d.lightingRestart = func() { count++ }
	status, added := d.ProcessNewGradientColor("gradient")
	if status != 1 || d.GetRgbProfile("gradient").Gradients[int(added)] != (rgb.Color{Green: 255, Blue: 255}) {
		t.Fatal("gradient addition changed")
	}
	status, deleted := d.ProcessDeleteGradientColor("gradient")
	if status != 1 || added != deleted || !reflect.DeepEqual(d.GetRgbProfile("gradient"), before) {
		t.Fatal("gradient removal changed")
	}
	if count != 0 {
		t.Fatal("inactive gradient edit restarted renderer")
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

func TestAllAuthoredZoneMappingsAndCopies(t *testing.T) {
	d := newDarkCoreLightingTestDevice(t)
	if err := d.SetLightingEffect("mouse"); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := d.LightingSnapshot()
	names := []string{"Scroll", "Logo", "Side Accent 1", "Side Accent 2", "Side Accent 3", "Side Accent 4", "Side Accent 5", "Side Accent 6"}
	indices := [8][3]int{{0, 12, 24}, {6, 18, 30}, {1, 13, 25}, {2, 14, 26}, {3, 15, 27}, {4, 16, 28}, {5, 17, 29}, {7, 19, 31}}
	if !ok || len(snapshot.AuthoredZoneEditor.Zones) != 8 {
		t.Fatal("zones unavailable")
	}
	for id, name := range names {
		zone := snapshot.AuthoredZoneEditor.Zones[id]
		if zone.Label != name || zone.ColorHex != "#c86428" || !reflect.DeepEqual(d.DeviceProfile.ZoneColors[id].ColorIndex, indices[id][:]) {
			t.Fatalf("zone %d=%#v", id, zone)
		}
	}
	copies := d.lightingOutputZones(20, true)
	copies[0].Color.Red = 99
	copies[0].ColorIndex[0] = 35
	if d.DeviceProfile.ZoneColors[0].Color.Red != 200 || d.DeviceProfile.ZoneColors[0].ColorIndex[0] != 0 {
		t.Fatal("output alias")
	}
	if err := d.SetLightingZoneColors("mouse", []string{"0", "7"}, rgb.Color{Red: 9, Green: 8, Blue: 7}); err != nil {
		t.Fatal(err)
	}
	if d.DeviceProfile.ZoneColors[1].Color.Red != 200 || d.DeviceProfile.ZoneColors[7].Color.Red != 9 {
		t.Fatal("multi-zone edit")
	}
	before := *d.DeviceProfile.ZoneColors[0].Color
	for _, ids := range [][]string{{"0", "0"}, {"8"}, {"bad"}, {}} {
		if d.SetLightingZoneColors("mouse", ids, rgb.Color{}) == nil || *d.DeviceProfile.ZoneColors[0].Color != before {
			t.Fatal("invalid zones accepted")
		}
	}
	if err := d.SetLightingZoneColor("mouse", "all", "", "", rgb.Color{Blue: 42}); err != nil {
		t.Fatal(err)
	}
	for _, zone := range d.DeviceProfile.ZoneColors {
		if zone.Color.Blue != 42 {
			t.Fatal("all-zone edit")
		}
	}
}

func TestExternalOwnersRetainDesiredStateAndSuppressLocalOutput(t *testing.T) {
	for _, clusterOwner := range []bool{false, true} {
		t.Run(map[bool]string{false: "OpenRGB", true: "Cluster"}[clusterOwner], func(t *testing.T) {
			d := newDarkCoreLightingTestDevice(t)
			restarts := 0
			d.lightingRestart = func() { restarts++ }
			if err := d.setLightingOwnership(clusterOwner, true); err != nil {
				t.Fatal(err)
			}
			snapshot, ok := d.LightingSnapshot()
			if !ok || snapshot.ClusterControlled != clusterOwner || snapshot.ExternalControlled == clusterOwner {
				t.Fatal("owner snapshot")
			}
			if d.setLightingOwnership(!clusterOwner, true) == nil {
				t.Fatal("conflicting owner")
			}
			if err := d.SetLightingEffect("mouse"); err != nil {
				t.Fatal(err)
			}
			if err := d.SetLightingBrightness(48); err != nil {
				t.Fatal(err)
			}
			value, _ := d.ResolveLightingEffectSettings("static")
			value.SingleColor.Color.Red = 3
			if err := d.SetLightingEffectSettings("static", value); err != nil {
				t.Fatal(err)
			}
			if err := d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 9}); err != nil {
				t.Fatal(err)
			}
			d.SchedulerBrightness(0)
			d.ControlDeviceRgb(true)
			if restarts != 0 {
				t.Fatal("external owner got local output/restart")
			}
			snapshot, ok = d.LightingSnapshot()
			if !ok || snapshot.ConfiguredEffect != "mouse" || snapshot.Brightness != 48 || snapshot.AuthoredZoneEditor.Zones[0].ColorHex != "#090000" {
				t.Fatal("desired state lost under external owner")
			}
			if err := d.setLightingOwnership(clusterOwner, false); err != nil {
				t.Fatal(err)
			}
			d.restartLighting()
			if restarts != 1 {
				t.Fatal("local resume")
			}
			d.SchedulerBrightness(100)
			d.ControlDeviceRgb(false)
			_, brightness, _ := d.lightingOutputState()
			if brightness != 48 {
				t.Fatal("latest local desired state not restored")
			}
		})
	}
}

func TestPersistencePrecedesPublicationAndOutput(t *testing.T) {
	d := newDarkCoreLightingTestDevice(t)
	d.lightingRestart = func() {
		data, err := os.ReadFile(d.DeviceProfile.Path)
		var persisted DeviceProfile
		if err != nil || json.Unmarshal(data, &persisted) != nil || persisted.RGBProfile != d.DeviceProfile.RGBProfile || *persisted.BrightnessSlider != *d.DeviceProfile.BrightnessSlider {
			t.Fatal("restart before profile persistence")
		}
		data, err = os.ReadFile(filepath.Join(pwd, "database", "rgb", d.Serial+".json"))
		var stored rgb.RGB
		if err != nil || json.Unmarshal(data, &stored) != nil {
			t.Fatal("restart before RGB persistence")
		}
		if !reflect.DeepEqual(stored.Profiles["static"], *d.GetRgbProfile("static")) {
			t.Fatal("RGB publication before persistence")
		}
	}
	if err := d.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetLightingBrightness(31); err != nil {
		t.Fatal(err)
	}
	value, _ := d.ResolveLightingEffectSettings("static")
	value.SingleColor.Color.Red = 11
	if err := d.SetLightingEffectSettings("static", value); err != nil {
		t.Fatal(err)
	}
}

func TestLightingProfileSwitchFailureRetainsAuthority(t *testing.T) {
	d := newDarkCoreLightingTestDevice(t)
	target := *d.DeviceProfile
	brightness := uint8(23)
	target.BrightnessSlider = &brightness
	target.Path = filepath.Join(pwd, "database", "profiles", "unwritable.json")
	target.Active = false
	if err := os.Mkdir(target.Path, 0700); err != nil {
		t.Fatal(err)
	}
	d.UserProfiles["bad"] = &target
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(true)
	if d.switchLightingProfile("bad") == nil {
		t.Fatal("failed profile write accepted")
	}
	snapshot, ok := d.LightingSnapshot()
	_, effective, off := d.lightingOutputState()
	if !ok || snapshot.Brightness != 63 || !d.DeviceProfile.Active || effective != 0 || !off || target.Active {
		t.Fatal("failed switch changed authority")
	}
}

func TestExternalFrameBytesAndIndicatorsRemainOwned(t *testing.T) {
	d := newDarkCoreLightingTestDevice(t)
	data := make([]byte, 36)
	for id := range data {
		data[id] = byte(70 + id)
	}
	before := *d.DeviceProfile.DPIColor
	zonesBefore := d.copyLightingZones()
	// Darkness changes local effective brightness, never externally supplied bytes.
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(true)
	for _, sniper := range []bool{false, true, false} {
		d.SniperMode = sniper
		frame, ok := d.externalLightingFrame(data)
		if !ok || len(frame) != 36 {
			t.Fatal("external frame unavailable")
		}
		indices := [][]int{{0, 12, 24}, {6, 18, 30}, {1, 13, 25}, {2, 14, 26}, {3, 15, 27}, {4, 16, 28}, {5, 17, 29}, {7, 19, 31}}
		m := 0
		for _, zone := range indices {
			for _, index := range zone {
				if frame[index] != data[m] {
					t.Fatalf("external byte %d changed", m)
				}
				m++
			}
		}
		if sniper {
			if frame[9] != 255 || frame[21] != 255 || frame[33] != 0 {
				t.Fatal("Sniper indicator changed")
			}
		} else {
			if frame[8] != 255 || frame[20] != 0 || frame[32] != 0 {
				t.Fatal("DPI indicator changed")
			}
		}
	}
	if *d.DeviceProfile.DPIColor != before || !reflect.DeepEqual(d.copyLightingZones(), zonesBefore) {
		t.Fatal("external composition mutated authored data")
	}
	// OpenRGB's actual queue ingress keeps the existing defensive-copy and owner guard.
	d.queue = make(chan []byte, 2)
	d.writeColorEx(data, 0)
	if len(d.queue) != 0 {
		t.Fatal("unowned OpenRGB frame accepted")
	}
	if err := d.setLightingOwnership(false, true); err != nil {
		t.Fatal(err)
	}
	d.writeColorEx(data, 0)
	data[0] = 0
	queued := <-d.queue
	if queued[0] != 70 {
		t.Fatal("OpenRGB queue aliases caller bytes")
	}
	d.Exit = true
	d.writeColorEx(data, 0)
	if len(d.queue) != 0 {
		t.Fatal("stopped OpenRGB frame accepted")
	}
}

func TestRetainedMouseFormPersistsDPICopyWithoutChangingInput(t *testing.T) {
	d := newDarkCoreLightingTestDevice(t)
	beforeDPI := d.DeviceProfile.DPIColor
	beforeZones := d.copyLightingZones()
	input := d.DeviceProfile.Profiles
	if d.SaveMouseZoneColors(rgb.Color{Red: 8, Green: 9, Blue: 10}, map[int]rgb.Color{0: {Blue: 9}}) != 1 {
		t.Fatal("retained Mouse form failed")
	}
	if beforeDPI.Red != 255 || beforeZones[0].Color.Blue != 40 || d.DeviceProfile.DPIColor.Red != 8 || d.DeviceProfile.ZoneColors[0].Color.Blue != 9 || !reflect.DeepEqual(input, d.DeviceProfile.Profiles) {
		t.Fatal("DPI/input or authored alias changed")
	}
	before := *d.DeviceProfile.DPIColor
	if err := os.Remove(d.DeviceProfile.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(d.DeviceProfile.Path, 0700); err != nil {
		t.Fatal(err)
	}
	if d.SaveMouseZoneColors(rgb.Color{}, map[int]rgb.Color{0: {Red: 4}}) != 0 || *d.DeviceProfile.DPIColor != before {
		t.Fatal("failed retained Mouse form published DPI color")
	}
}

func TestDesiredMutationsDuringEachTransientOverride(t *testing.T) {
	for _, scheduler := range []bool{false, true} {
		t.Run(map[bool]string{false: "RGB-off", true: "Scheduler"}[scheduler], func(t *testing.T) {
			d := newDarkCoreLightingTestDevice(t)
			if err := d.setLightingDarkness(scheduler, true); err != nil {
				t.Fatal(err)
			}
			if err := d.SetLightingEffect("wave"); err != nil {
				t.Fatal(err)
			}
			wave, _ := d.ResolveLightingEffectSettings("wave")
			*wave.Speed = 3
			wave.TwoColor.Start.Red = 17
			if err := d.SetLightingEffectSettings("wave", wave); err != nil {
				t.Fatal(err)
			}
			if err := d.SetLightingBrightness(28); err != nil {
				t.Fatal(err)
			}
			snapshot, ok := d.LightingSnapshot()
			_, brightness, _ := d.lightingOutputState()
			if !ok || snapshot.ConfiguredEffect != "wave" || snapshot.Brightness != 28 || snapshot.Speed != 3 || brightness != 0 {
				t.Fatal("dynamic mutation escaped darkness or lost desired settings")
			}
			if err := d.SetLightingEffect("mouse"); err != nil {
				t.Fatal(err)
			}
			if err := d.SetLightingZoneColors("mouse", []string{"1", "6"}, rgb.Color{Blue: 19}); err != nil {
				t.Fatal(err)
			}
			zones := d.lightingOutputZones(0, true)
			if zones[1].Color.Blue != 0 || d.DeviceProfile.ZoneColors[1].Color.Blue != 19 {
				t.Fatal("dark output mutated authored state")
			}
			if err := d.setLightingDarkness(scheduler, false); err != nil {
				t.Fatal(err)
			}
			effect, brightness, off := d.lightingOutputState()
			if effect != "mouse" || brightness != 28 || off {
				t.Fatal("latest desired state not restored")
			}
			resolved, _ := d.ResolveLightingEffectSettings("wave")
			if !reflect.DeepEqual(resolved, wave) {
				t.Fatal("override changed desired effect customization")
			}
		})
	}
}

func TestUnavailableOwnershipChangesAndFailedOpenRGBPersistencePreserveQueue(t *testing.T) {
	d := newDarkCoreLightingTestDevice(t)
	d.queue = make(chan []byte, 2)
	d.queue <- []byte{71, 72, 73}
	d.Connected = false
	if d.ProcessSetOpenRgbIntegration(true) != 0 || d.ProcessSetRgbCluster(true) != 0 || len(d.queue) != 1 {
		t.Fatal("unavailable owner mutation changed queue")
	}
	d.Connected = true
	if err := os.Remove(d.DeviceProfile.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(d.DeviceProfile.Path, 0700); err != nil {
		t.Fatal(err)
	}
	if d.ProcessSetOpenRgbIntegration(true) != 0 || d.ProcessSetRgbCluster(true) != 0 || len(d.queue) != 1 || d.DeviceProfile.OpenRGBIntegration || d.DeviceProfile.RGBCluster {
		t.Fatal("failed ownership persistence changed authority/output")
	}
}

func TestProfileSwitchRetiresPriorRendererForExternalProfile(t *testing.T) {
	d := newDarkCoreLightingTestDevice(t)
	target := *d.DeviceProfile
	target.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-cluster.json")
	target.RGBCluster, target.Active = true, false
	d.UserProfiles["cluster"] = &target
	d.activeRgb = rgb.Exit()
	// Buffered signal observes retirement without a renderer or hardware goroutine.
	d.activeRgb.Exit = make(chan bool, 1)
	retired := d.activeRgb
	if err := d.switchLightingProfile("cluster"); err != nil {
		t.Fatal(err)
	}
	if d.activeRgb != nil || len(retired.Exit) != 1 {
		t.Fatal("prior renderer retained under external profile")
	}
}

func TestConcurrentActiveProfileSnapshots(t *testing.T) {
	d := newDarkCoreLightingTestDevice(t)
	target := *d.DeviceProfile
	brightness := uint8(37)
	target.BrightnessSlider, target.RGBProfile, target.Path, target.Active = &brightness, "mouse", filepath.Join(pwd, "database", "profiles", d.Serial+"-gaming.json"), false
	d.UserProfiles["gaming"] = &target
	var workers sync.WaitGroup
	for id := 0; id < 3; id++ {
		workers.Add(1)
		go func(id int) {
			defer workers.Done()
			for n := 0; n < 10; n++ {
				if id == 0 {
					if _, ok := d.LightingSnapshot(); !ok {
						t.Error("profile snapshot unavailable")
					}
				} else if id == 1 {
					name := "default"
					if n%2 == 0 {
						name = "gaming"
					}
					if err := d.switchLightingProfile(name); err != nil {
						t.Error(err)
					}
				} else {
					if err := d.SetLightingZoneColors("mouse", []string{"0", "7"}, rgb.Color{Red: float64(n)}); err != nil {
						t.Error(err)
					}
					d.lightingOutputZones(50, true)
				}
			}
		}(id)
	}
	workers.Wait()
	if _, ok := d.LightingSnapshot(); !ok {
		t.Fatal("final snapshot unavailable")
	}
}

func TestLegacyBrightnessMetadataDoesNotReplaceDesiredSlider(t *testing.T) {
	d := newDarkCoreLightingTestDevice(t)
	count := 0
	d.lightingRestart = func() { count++ }
	if d.ChangeDeviceBrightness(2) != 1 || d.DeviceProfile.Brightness != 2 || *d.DeviceProfile.BrightnessSlider != 63 || count != 0 {
		t.Fatal("generic metadata became desired brightness authority")
	}
}

func TestMalformedTopologyAndColorsFailClosed(t *testing.T) {
	for _, broken := range []string{"channels", "mapping", "name", "nil-color", "invalid-color"} {
		t.Run(broken, func(t *testing.T) {
			d := newDarkCoreLightingTestDevice(t)
			zone := d.DeviceProfile.ZoneColors[0]
			switch broken {
			case "channels":
				d.ChangeableLedChannels = 8
			case "mapping":
				zone.ColorIndex = []int{0, 1, 2}
			case "name":
				zone.Name = "Unknown"
			case "nil-color":
				zone.Color = nil
			case "invalid-color":
				zone.Color.Red = 256
			}
			d.DeviceProfile.ZoneColors[0] = zone
			if _, ok := d.LightingSnapshot(); ok {
				t.Fatal("malformed runtime snapshot available")
			}
			if d.SetLightingBrightness(7) == nil || d.SetLightingEffect("wave") == nil {
				t.Fatal("malformed runtime mutation accepted")
			}
		})
	}
}

func TestProfileSwitchNormalizesLegacyMissingBrightnessSlider(t *testing.T) {
	for _, scenario := range []string{"missing-slider", "missing-slider-persistence-failure", "explicit-over-100"} {
		t.Run(scenario, func(t *testing.T) {
			d := newDarkCoreLightingTestDevice(t)
			active := d.DeviceProfile
			beforeActive := *active
			beforeSnapshot, _ := d.LightingSnapshot()
			beforeRGB, err := json.Marshal(d.Rgb)
			if err != nil {
				t.Fatal(err)
			}
			beforeActiveFile, err := os.ReadFile(active.Path)
			if err != nil {
				t.Fatal(err)
			}

			legacy := *active
			legacy.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-legacy.json")
			legacy.Active, legacy.RGBProfile, legacy.BrightnessSlider = false, "mouse", nil
			legacy.Product, legacy.Label = "Legacy mouse", "Legacy profile"
			legacy.Profile, legacy.PollingRate, legacy.SleepMode = 1, 4, 10
			legacy.AngleSnapping, legacy.ButtonOptimization = 1, 1
			legacy.KeyAssignmentHash = "legacy-button-assignments"
			// Exercise an actual inactive JSON profile with the field absent.
			data, err := json.Marshal(legacy)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			delete(fields, "BrightnessSlider")
			if scenario == "explicit-over-100" {
				fields["BrightnessSlider"] = json.RawMessage("101")
			}
			if err = saveLightingJSON(legacy.Path, fields); err != nil {
				t.Fatal(err)
			}
			beforeTargetFile, err := os.ReadFile(legacy.Path)
			if err != nil {
				t.Fatal(err)
			}
			var target DeviceProfile
			if err = json.Unmarshal(beforeTargetFile, &target); err != nil {
				t.Fatal(err)
			}
			d.UserProfiles["legacy"] = &target
			beforeTarget := target
			if scenario != "explicit-over-100" && target.BrightnessSlider != nil {
				t.Fatal("legacy fixture unexpectedly has a slider")
			}
			if scenario == "missing-slider-persistence-failure" {
				if err = os.Remove(target.Path); err != nil {
					t.Fatal(err)
				}
				if err = os.Mkdir(target.Path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			d.SchedulerBrightness(0)
			d.ControlDeviceRgb(true)
			restarts := 0
			d.lightingRestart = func() {
				restarts++
				persisted, readErr := os.ReadFile(target.Path)
				var stored DeviceProfile
				if readErr != nil || json.Unmarshal(persisted, &stored) != nil || stored.BrightnessSlider == nil || *stored.BrightnessSlider != 100 || !stored.Active {
					t.Fatal("restart preceded normalized target persistence")
				}
			}
			err = d.switchLightingProfile("legacy")
			if scenario == "missing-slider" {
				if err != nil {
					t.Fatalf("legacy profile switch failed: %v", err)
				}
				snapshot, ok := d.LightingSnapshot()
				if !ok || snapshot.Brightness != 100 || snapshot.ConfiguredEffect != "mouse" || d.DeviceProfile != &target || !target.Active || active.Active || restarts != 1 {
					t.Fatal("legacy switch did not publish normalized desired state")
				}
				data, err = os.ReadFile(target.Path)
				if err != nil {
					t.Fatal(err)
				}
				var persisted DeviceProfile
				if err = json.Unmarshal(data, &persisted); err != nil {
					t.Fatal(err)
				}
				expected := beforeTarget
				brightness := uint8(100)
				expected.BrightnessSlider, expected.Active, expected.RgbOff = &brightness, true, false
				if !reflect.DeepEqual(target, expected) || !reflect.DeepEqual(persisted, expected) {
					t.Fatal("normalization lost or changed unrelated DPI/button/profile data")
				}
			} else {
				if err == nil {
					t.Fatal("failed persistence or explicit brightness above 100 was accepted")
				}
				snapshot, ok := d.LightingSnapshot()
				_, effective, off := d.lightingOutputState()
				afterActiveFile, readErr := os.ReadFile(active.Path)
				if readErr != nil || !reflect.DeepEqual(beforeActiveFile, afterActiveFile) || d.DeviceProfile != active || !reflect.DeepEqual(*active, beforeActive) || !reflect.DeepEqual(target, beforeTarget) || !ok || !reflect.DeepEqual(snapshot, beforeSnapshot) || effective != 0 || !off || restarts != 0 {
					t.Fatal("rejected switch changed active/canonical authority, target, overrides or output")
				}
				if scenario == "explicit-over-100" {
					afterTargetFile, readErr := os.ReadFile(target.Path)
					if readErr != nil || !reflect.DeepEqual(afterTargetFile, beforeTargetFile) {
						t.Fatal("invalid explicit brightness changed persisted target")
					}
				}
			}
			afterRGB, err := json.Marshal(d.Rgb)
			if err != nil || !reflect.DeepEqual(beforeRGB, afterRGB) {
				t.Fatal("profile normalization changed RGB settings")
			}
		})
	}
}
