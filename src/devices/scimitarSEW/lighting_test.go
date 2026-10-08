package scimitarSEW

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"github.com/sstallion/go-hid"
)

func newScimitarLightingTestDevice(t *testing.T) *Device {
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
	for id, name := range []string{"Side", "Logo"} {
		indices := [2][3]int{{1, 5, 9}, {0, 4, 8}}
		zones[id] = ZoneColors{Name: name, ColorIndex: append([]int(nil), indices[id][:]...), Color: &rgb.Color{Red: 200, Green: 100, Blue: 40, Brightness: 1}}
	}
	profile := &DeviceProfile{ZoneColors: zones, Profiles: map[int]DPIProfile{0: {Name: "Stage 1", Value: 800, Color: &rgb.Color{Red: 255, Brightness: 1}, ColorIndex: map[int][]int{0: {2, 6, 10}}}, 1: {Name: "Stage 2", Value: 1200, Color: &rgb.Color{Green: 255, Brightness: 1}, ColorIndex: map[int][]int{0: {2, 6, 10}}}, 2: {Name: "Sniper", Value: 400, Sniper: true, Color: &rgb.Color{Red: 255, Green: 255, Brightness: 1}, ColorIndex: map[int][]int{0: {2, 6, 10}}}}, Active: true, Serial: "ScimitarTEST", RGBProfile: "static", BrightnessSlider: &brightness, OriginalBrightness: 91, PollingRate: 4, SleepMode: 15, LiftHeight: 2, Path: filepath.Join(pwd, "database", "profiles", "ScimitarTEST.json")}

	d := &Device{Serial: profile.Serial, dev: &common.Slipstream{Dev: &hid.Device{}}, LEDChannels: 4, ChangeableLedChannels: 2, Connected: true, DPIAmount: 3, MinDPI: minDpiValue, MaxDPI: maxDpiValue, Rgb: &profiles, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"default": profile}, lightingRestart: func() {}}
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
	d := newScimitarLightingTestDevice(t)
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
	d := newScimitarLightingTestDevice(t)
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
	d := newScimitarLightingTestDevice(t)
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
	d := newScimitarLightingTestDevice(t)
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
	for _, unavailable := range []string{"disconnected", "receiver-handle", "offline", "stopped", "runtime"} {
		d.lightingDefaults, d.dev, d.Exit, d.Connected = defaults, device, false, true
		switch unavailable {
		case "disconnected":
			d.dev = nil
		case "receiver-handle":
			d.dev = &common.Slipstream{}
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
	if d.SetLightingEffect("wave") == nil || d.SetLightingBrightness(8) == nil || d.SetLightingZoneColors("mouse", []string{"0", "1"}, rgb.Color{}) == nil {
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
	d := newScimitarLightingTestDevice(t)
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
	d := newScimitarLightingTestDevice(t)
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
	d := newScimitarLightingTestDevice(t)
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
	d := newScimitarLightingTestDevice(t)
	if err := d.SetLightingEffect("mouse"); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := d.LightingSnapshot()
	if !ok || snapshot.AuthoredZoneEditor == nil || len(snapshot.AuthoredZoneEditor.Zones) != 2 || snapshot.AuthoredZoneEditor.Zones[0].ID != "0" || snapshot.AuthoredZoneEditor.Zones[0].Label != "Side" {
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
	if zone.Color.Red != 9 || zone.Color.Green != 8 || zone.Color.Blue != 7 || !reflect.DeepEqual(zone.ColorIndex, []int{1, 5, 9}) || zone.Name != "Side" {
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
	d := newScimitarLightingTestDevice(t)
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
	d := newScimitarLightingTestDevice(t)
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
	d := newScimitarLightingTestDevice(t)
	if err := d.SetLightingEffect("mouse"); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := d.LightingSnapshot()
	names := []string{"Side", "Logo"}
	indices := [2][3]int{{1, 5, 9}, {0, 4, 8}}
	if !ok || len(snapshot.AuthoredZoneEditor.Zones) != 2 {
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
	if d.DeviceProfile.ZoneColors[0].Color.Red != 200 || d.DeviceProfile.ZoneColors[0].ColorIndex[0] != 1 {
		t.Fatal("output alias")
	}
	if err := d.SetLightingZoneColors("mouse", []string{"0", "1"}, rgb.Color{Red: 9, Green: 8, Blue: 7}); err != nil {
		t.Fatal(err)
	}
	if d.DeviceProfile.ZoneColors[0].Color.Red != 9 || d.DeviceProfile.ZoneColors[1].Color.Red != 9 {
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
			d := newScimitarLightingTestDevice(t)
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
	d := newScimitarLightingTestDevice(t)
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
	d := newScimitarLightingTestDevice(t)
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
	d := newScimitarLightingTestDevice(t)
	data := make([]byte, 6)
	for id := range data {
		data[id] = byte(70 + id)
	}
	before := *d.DeviceProfile.Profiles[0].Color
	zonesBefore := d.copyLightingZones()
	// Darkness changes local effective brightness, never externally supplied bytes.
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(true)
	for _, sniper := range []bool{false, true, false} {
		d.SniperMode = sniper
		frame, ok := d.externalLightingFrame(data)
		if !ok || len(frame) != 12 {
			t.Fatal("external frame unavailable")
		}
		indices := [][]int{{1, 5, 9}, {0, 4, 8}}
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
			if frame[2] != 0 || frame[6] != 0 || frame[10] != 0 {
				t.Fatal("Sniper indicator changed")
			}
		} else {
			if frame[2] != 0 || frame[6] != 0 || frame[10] != 0 {
				t.Fatal("DPI indicator changed")
			}
		}
	}
	if *d.DeviceProfile.Profiles[0].Color != before || !reflect.DeepEqual(d.copyLightingZones(), zonesBefore) {
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

func TestRetainedMouseFormPreservesDPIAndInput(t *testing.T) {
	d := newScimitarLightingTestDevice(t)
	beforeDPI := *d.DeviceProfile.Profiles[0].Color
	input, _ := json.Marshal(d.DeviceProfile.Profiles)
	beforeZones := d.copyLightingZones()
	if d.SaveMouseZoneColors(rgb.Color{Red: 8}, map[int]rgb.Color{0: {Blue: 9}}) != 1 {
		t.Fatal("Mouse form failed")
	}
	after, _ := json.Marshal(d.DeviceProfile.Profiles)
	if !reflect.DeepEqual(input, after) || *d.DeviceProfile.Profiles[0].Color != beforeDPI || beforeZones[0].Color.Blue != 40 || d.DeviceProfile.ZoneColors[0].Color.Blue != 9 {
		t.Fatal("DPI/input or alias changed")
	}
	before, _ := json.Marshal(d.DeviceProfile)
	if err := os.Remove(d.DeviceProfile.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(d.DeviceProfile.Path, 0700); err != nil {
		t.Fatal(err)
	}
	if d.SaveMouseZoneColors(rgb.Color{}, map[int]rgb.Color{0: {Red: 4}}) != 0 {
		t.Fatal("failed persistence accepted")
	}
	after, _ = json.Marshal(d.DeviceProfile)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed Mouse form published state")
	}
}

func TestDesiredMutationsDuringEachTransientOverride(t *testing.T) {
	for _, scheduler := range []bool{false, true} {
		t.Run(map[bool]string{false: "RGB-off", true: "Scheduler"}[scheduler], func(t *testing.T) {
			d := newScimitarLightingTestDevice(t)
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
			if err := d.SetLightingZoneColors("mouse", []string{"0", "1"}, rgb.Color{Blue: 19}); err != nil {
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
	d := newScimitarLightingTestDevice(t)
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
	d := newScimitarLightingTestDevice(t)
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
	d := newScimitarLightingTestDevice(t)
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
					if err := d.SetLightingZoneColors("mouse", []string{"0", "1"}, rgb.Color{Red: float64(n)}); err != nil {
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
	d := newScimitarLightingTestDevice(t)
	count := 0
	d.lightingRestart = func() { count++ }
	if d.ChangeDeviceBrightness(2) != 1 || d.DeviceProfile.Brightness != 2 || *d.DeviceProfile.BrightnessSlider != 63 || count != 0 {
		t.Fatal("generic metadata became desired brightness authority")
	}
}

func TestMalformedTopologyAndColorsFailClosed(t *testing.T) {
	for _, broken := range []string{"channels", "mapping", "name", "nil-color", "invalid-color"} {
		t.Run(broken, func(t *testing.T) {
			d := newScimitarLightingTestDevice(t)
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
			d := newScimitarLightingTestDevice(t)
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

func TestPackageSpecificDPIAndProfileFieldsPreserved(t *testing.T) {
	d := newScimitarLightingTestDevice(t)
	if minDpiValue != 100 || maxDpiValue != 33000 {
		t.Fatal("package DPI limits changed")
	}
	before := *d.DeviceProfile
	if err := d.SetLightingEffect("mouse"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetLightingBrightness(70); err != nil {
		t.Fatal(err)
	}
	after := *d.DeviceProfile
	before.RGBProfile, before.BrightnessSlider = after.RGBProfile, after.BrightnessSlider
	if !reflect.DeepEqual(before, after) {
		t.Fatal("Lighting mutated unrelated profile fields")
	}
}

func TestProfileSwitchLegacyDefaultsUseCopiesAndPersistence(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			d := newScimitarLightingTestDevice(t)
			target := *d.DeviceProfile
			target.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-older.json")
			target.Active, target.BrightnessSlider, target.PollingRate, target.SleepMode, target.LiftHeight = false, nil, 0, 0, 0

			target.Profiles = map[int]DPIProfile{0: d.DeviceProfile.Profiles[0]}
			before, _ := json.Marshal(target)
			d.UserProfiles["older"] = &target
			if fail {
				if err := os.Mkdir(target.Path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			err := d.switchLightingProfile("older")
			if fail {
				after, _ := json.Marshal(target)
				if err == nil || !reflect.DeepEqual(before, after) || d.DeviceProfile == &target {
					t.Fatal("failed normalization mutated the target or active profile")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if *target.BrightnessSlider != 100 || target.PollingRate != 4 || target.SleepMode != 15 || target.LiftHeight != 2 || len(target.Profiles) != 2 || !target.Profiles[1].Sniper || target.Profiles[1].Value != 200 {
				t.Fatal("source-backed legacy defaults not retained")
			}
			data, err := os.ReadFile(target.Path)
			var stored DeviceProfile
			if err != nil || json.Unmarshal(data, &stored) != nil || !reflect.DeepEqual(stored, target) {
				t.Fatal("normalized profile not persisted")
			}
		})
	}
}

func TestLocalMouseAndStaticOutputCopyDPIAndSniperColors(t *testing.T) {
	d := newScimitarLightingTestDevice(t)
	d.lightingRestart = nil
	var frames [][]byte
	d.lightingWrite = func(frame []byte) { frames = append(frames, frame) }
	for _, effect := range []string{"mouse", "static"} {
		for _, sniper := range []bool{false, true, false} {
			d.SniperMode = sniper
			before, _ := json.Marshal(d.DeviceProfile.Profiles)
			zones, _ := json.Marshal(d.DeviceProfile.ZoneColors)
			if err := d.SetLightingEffect(effect); err != nil {
				t.Fatal(err)
			}
			if err := d.SetLightingBrightness(50); err != nil {
				t.Fatal(err)
			}
			frame := frames[len(frames)-1]
			if len(frame) != 12 || frame[2] != 127 || frame[10] != 0 || (sniper && frame[6] != 127) || (!sniper && frame[6] != 0) {
				t.Fatalf("DPI/Sniper frame=%v", frame)
			}
			if effect == "mouse" && (frame[1] != 100 || frame[5] != 50 || frame[9] != 20) {
				t.Fatalf("Mouse frame=%v", frame)
			}
			after, _ := json.Marshal(d.DeviceProfile.Profiles)
			zonesAfter, _ := json.Marshal(d.DeviceProfile.ZoneColors)
			if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(zones, zonesAfter) {
				t.Fatal("output mutated stored pointer data")
			}
			// Mutating copied DPI metadata must never change persistent indicator indices.
			color, stage := d.lightingOutputDPI(50)
			color.Red = 0
			stage.Color.Red = 0
			stage.ColorIndex[0][0] = 35
			if d.DeviceProfile.Profiles[0].Color.Red != 255 || d.DeviceProfile.Profiles[0].ColorIndex[0][0] != 2 {
				t.Fatal("DPI alias")
			}
		}
	}
	before, _ := json.Marshal(d.DeviceProfile)
	if err := os.Remove(d.DeviceProfile.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(d.DeviceProfile.Path, 0700); err != nil {
		t.Fatal(err)
	}
	count := len(frames)
	if d.SetLightingEffect("mouse") == nil || d.SetLightingBrightness(1) == nil || d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{}) == nil {
		t.Fatal("failed write accepted")
	}
	after, _ := json.Marshal(d.DeviceProfile)
	if len(frames) != count || !reflect.DeepEqual(before, after) {
		t.Fatal("failed persistence produced output/publication")
	}
}

func TestPublicOwnershipTransitionsRetireRendererAndResumeLocal(t *testing.T) {
	for _, clusterOwner := range []bool{false, true} {
		t.Run(map[bool]string{false: "OpenRGB", true: "Cluster"}[clusterOwner], func(t *testing.T) {
			d := newScimitarLightingTestDevice(t)
			d.queue = make(chan []byte, 2)
			frames, restarts := 0, 0
			d.lightingWrite = func(frame []byte) {
				frames++
				if frame[1] != 0 || frame[5] != 0 || frame[9] != 0 || frame[0] != 0 || frame[4] != 0 || frame[8] != 0 {
					t.Fatal("external transition didn't clear local zones")
				}
			}
			d.lightingRestart = func() { restarts++ }
			d.activeRgb = rgb.Exit()
			d.activeRgb.Exit = make(chan bool, 1)
			retired := d.activeRgb
			set := d.ProcessSetOpenRgbIntegration
			if clusterOwner {
				set = d.ProcessSetRgbCluster
			}
			if set(true) != 1 || d.activeRgb != nil || len(retired.Exit) != 1 {
				t.Fatal("owner transition failed to retire local renderer")
			}
			transitionFrames := frames
			if err := d.SetLightingEffect("mouse"); err != nil {
				t.Fatal(err)
			}
			if err := d.SetLightingBrightness(37); err != nil {
				t.Fatal(err)
			}
			if err := d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Blue: 42}); err != nil {
				t.Fatal(err)
			}
			if frames != transitionFrames || restarts != 0 {
				t.Fatal("external desired mutation produced local output")
			}
			if set(false) != 1 || restarts != 1 {
				t.Fatal("ownership release didn't resume canonical renderer")
			}
			snapshot, ok := d.LightingSnapshot()
			if !ok || snapshot.ConfiguredEffect != "mouse" || snapshot.Brightness != 37 || snapshot.AuthoredZoneEditor.Zones[0].ColorHex != "#00002a" {
				t.Fatal("local restoration lost latest desired state")
			}
		})
	}
}

func TestExternalIndicatorBrightnessAndPointerCopies(t *testing.T) {
	d := newScimitarLightingTestDevice(t)
	data := []byte{90, 80, 70, 60, 50, 40}
	for _, sniper := range []bool{false, true} {
		d.SniperMode = sniper
		for _, brightness := range []uint8{70, 0, 100} {
			if err := d.SetLightingBrightness(brightness); err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(d.DeviceProfile)
			frame, ok := d.externalLightingFrame(data)
			value := byte(255 * rgb.GetBrightnessValueFloat(brightness))
			if !ok || frame[2] != value || (sniper && frame[6] != value) || (!sniper && frame[6] != 0) || frame[10] != 0 {
				t.Fatalf("external indicator frame=%v", frame)
			}
			for id, indices := range [][]int{{1, 5, 9}, {0, 4, 8}} {
				for channel, index := range indices {
					if frame[index] != data[id*3+channel] {
						t.Fatal("external bytes scaled")
					}
				}
			}
			after, _ := json.Marshal(d.DeviceProfile)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("external composition mutated pointers")
			}
		}
	}
}

func TestMalformedRGBAndDPIBackingRejectAllCanonicalMutations(t *testing.T) {
	for _, broken := range []string{"rgb-map", "missing-effect", "bad-effect", "nil-dpi", "dpi-mapping", "selected-stage"} {
		t.Run(broken, func(t *testing.T) {
			d := newScimitarLightingTestDevice(t)
			value, _ := d.ResolveLightingEffectSettings("static")
			switch broken {
			case "rgb-map":
				d.Rgb.Profiles = nil
			case "missing-effect":
				delete(d.Rgb.Profiles, "wave")
			case "bad-effect":
				p := d.Rgb.Profiles["wave"]
				p.Speed = 1000
				d.Rgb.Profiles["wave"] = p
			case "nil-dpi":
				p := d.DeviceProfile.Profiles[0]
				p.Color = nil
				d.DeviceProfile.Profiles[0] = p
			case "dpi-mapping":
				d.DeviceProfile.Profiles[0].ColorIndex[0][0] = 50
			case "selected-stage":
				d.DeviceProfile.Profile = 99
			}
			before, _ := json.Marshal(d.DeviceProfile)
			file, _ := os.ReadFile(d.DeviceProfile.Path)
			if _, ok := d.LightingSnapshot(); ok {
				t.Fatal("malformed snapshot available")
			}
			if d.SetLightingEffect("mouse") == nil || d.SetLightingBrightness(7) == nil || d.SetLightingEffectSettings("static", value) == nil || d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{}) == nil || d.ResetLightingEffectSettings("static") == nil {
				t.Fatal("malformed mutation accepted")
			}
			after, _ := json.Marshal(d.DeviceProfile)
			stored, _ := os.ReadFile(d.DeviceProfile.Path)
			if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(file, stored) {
				t.Fatal("malformed mutation modified persistence/state")
			}
		})
	}
}

// Freeze the source-proven package boundaries excluded from this migration.
// These fingerprints are deliberately independent of the Lighting adapter.
// Lifecycle methods permit only state synchronization and nonfatal attachment
// retry in Connect; receiver, input and transport behavior remains local.
func TestPackageLocalInputTransportIdentityAndStopDirtyUnchanged(t *testing.T) {
	source, err := os.ReadFile("scimitarSEW.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "scimitarSEW.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{
		"TriggerKeyAssignment":   "a6a945f97abc6da7a95792a67379d7e03e23ea713826308ac5f1845ea149724f",
		"setupKeyAssignment":     "a7b90249e6f8f77d8583cb776ad57af38d175b7b77b6aa04854256cd277d692d",
		"transfer":               "5ed592d5ae7d2131c618a372cda95fe72aac05d4ce6807a7c7721fc43d021145",
		"writeKeyAssignmentData": "eed1f1187b4e92e8ce570e40b3edab5847ee699e214fb4249b014d520a4a3716",
		"checkDeviceOnline":      "e884dae9456007b1d2aa391f325a43a20238f2d4ee403492ef41ba910902bf20",
		"SetSleepMode":           "7c082c3816a3430cf6c0b194ccc2a790cfc16b03912e12fa30bf35223655cd42",
		"loadKeyAssignments":     "72b6dc45cef70f799636bc74387699adf030fe823e23437a162b2e506bcae20f",
		"upgradeDpiProfiles":     "aff582603007aab0915de88b6d0fcc6183b734eeb3d6b5aaf2c7841537d61ce1",
		"getSniperColor":         "0c02cc6084a94e87bb33a8002ec5eae8e55dff950f6ab1062c8dfecccceaf762",
		"releaseMacroTracker":    "c1e0a661b3b1ed80acb9ae4d3167819591517dc973fcc856a16b6b167997b77b",
		"addToMacroTracker":      "ee2cf7034c2169e14db72dd813128ab47d066cc80c05ab181e6c368502aafbb8",
		"deleteFromMacroTracker": "6f5ef1bf9a28df5816b908cee806f97c01faf97c9ed4d10ebc74ffdfba751ae3",
		"Stop":                   "568ad24a06ddc3a42a4e13de4f5ee548e979aa2b7ed34715efdfe6b834435a3f",
		"StopInternal":           "43d1e2c7488786b84f16c93f9158138a474eb58eb44829076695283e1340f420",
		"StopDirty":              "de2184870f56e1e2acccf7f6c3d40188ff8be0d5e2929893770bd5772ad75bc8",
		"SetConnected":           "e57a9fcdb30ecb2e1ba30337a41dbae3a35ffa6c02754da6934f55cf55583085",
		"Connect":                "0eb4f4f778dd7b07894e80b53ab17380e7fffd0dcf3fbb8f6aa48b480a8aca22",
		"sniperMode":             "4f60c46dd79dfd8218983b05a9d6042a0478583718b94de8ebb8f237c220f12d",
		"toggleDPI":              "c0cc339c48b43e2c70eee22ee55a914eef74ed5f319b2389611fc321e9a37eec",
		"SaveMouseDPI":           "5f110569911186a73934e157cf37df2bff661dd5e18e91111e409e1d9d5472e8",
		"SaveMouseDpiColors":     "2ca00a126bc6df2e5a6f3b7e26214b129d86c9a7a4d61e4710049e296abefaac",
	}
	found := 0
	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if !ok || f.Recv == nil {
			continue
		}
		want, ok := expected[f.Name.Name]
		if !ok {
			continue
		}
		found++
		text := string(source[fset.Position(f.Pos()).Offset:fset.Position(f.End()).Offset])
		text = strings.ReplaceAll(text, "\t\td.lightingMu.Lock()\n", "")
		text = strings.ReplaceAll(text, "\t\td.lightingMu.Unlock()\n", "")
		text = strings.ReplaceAll(text, "\td.lightingMu.Lock()\n", "")
		text = strings.ReplaceAll(text, "\td.lightingMu.Unlock()\n", "")
		if f.Name.Name == "Connect" {
			text = strings.ReplaceAll(text, "\t\tif d.lightingDefaults == nil {\n\t\t\tif err := d.attachLightingRuntime(config.GetPaths()); err != nil {\n\t\t\t\tlogger.Log(logger.Fields{\"error\": err, \"serial\": d.Serial}).Warn(\"Canonical lighting unavailable; retaining legacy Lighting\")\n\t\t\t}\n\t\t}\n", "")
		}
		if fmt.Sprintf("%x", sha256.Sum256([]byte(text))) != want {
			t.Fatalf("excluded package-local %s changed", f.Name.Name)
		}
	}
	if found != len(expected) {
		t.Fatal("package-local methods missing")
	}
}

func TestLegacyKnownUnsupportedSelectionRemainsLoadableNotSelectable(t *testing.T) {
	d := newScimitarLightingTestDevice(t)
	target := *d.DeviceProfile
	target.RGBProfile = "circle"
	target.Active = false
	target.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-legacy-circle.json")
	d.UserProfiles["legacy-circle"] = &target
	if err := d.switchLightingProfile("legacy-circle"); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := d.LightingSnapshot()
	if !ok || snapshot.ConfiguredEffect != "circle" || snapshot.EffectSupported || d.SupportsLightingEffect("circle") {
		t.Fatal("legacy selection became unreachable or selectable")
	}
	for _, option := range snapshot.SupportedEffects {
		if option.ID == "circle" {
			t.Fatal("unsupported selectable effect")
		}
	}
}

// Exercise the actual dynamic renderer and its unbuffered stop channel. The
// observation deadline spans three polling intervals, letting it encounter nil
// DPI data before restoring the data or asking the normal owner to retire it.
func TestDynamicRendererSurvivesTemporaryNilDPI(t *testing.T) {
	for _, action := range []string{"restore", "canonical-restart", "profile-switch"} {
		t.Run(action, func(t *testing.T) {
			d := newScimitarLightingTestDevice(t)
			frames := make(chan []byte, 16)
			d.lightingRestart = nil
			d.lightingWrite = func(frame []byte) {
				select {
				case frames <- frame:
				default:
				}
			}
			if err := d.SetLightingEffect("wave"); err != nil {
				t.Fatal(err)
			}
			old := d.activeRgb
			awaitFrame := func() {
				t.Helper()
				select {
				case <-frames:
				case <-time.After(time.Second):
					t.Fatal("renderer didn't produce a frame")
				}
			}
			awaitFrame()
			// Cleanup has a bounded stop send even if this regression terminates the
			// receiver. Avoid a leaked sender goroutine when testing a broken loop.
			t.Cleanup(func() {
				if d.activeRgb != nil {
					select {
					case d.activeRgb.Exit <- true:
					case <-time.After(time.Second):
						t.Error("renderer cleanup stop blocked")
					}
				}
			})
			d.lightingMu.Lock()
			stage := d.DeviceProfile.Profiles[0]
			original := stage.Color
			stage.Color = nil
			d.DeviceProfile.Profiles[0] = stage
			d.lightingMu.Unlock()
			// Drain any frame already composed before the nil publication.
			for {
				select {
				case <-frames:
				default:
					goto drained
				}
			}
		drained:
			select {
			case <-time.After(120 * time.Millisecond):
			}
			if d.activeRgb != old {
				t.Fatal("render loop changed renderer ownership")
			}
			// Discard every pre-nil frame so restoration must produce new output.
			for {
				select {
				case <-frames:
				default:
					goto nilObserved
				}
			}
		nilObserved:
			if action == "restore" {
				for _, guard := range []string{"disconnected", "Exit", "Cluster", "OpenRGB"} {
					// writeColor reads Exit under deviceLock; canonical guards
					// and frame composition read state under lightingMu.
					d.deviceLock.Lock()
					d.lightingMu.Lock()
					d.Connected = guard != "disconnected"
					d.Exit = guard == "Exit"
					d.DeviceProfile.RGBCluster = guard == "Cluster"
					d.DeviceProfile.OpenRGBIntegration = guard == "OpenRGB"
					d.lightingMu.Unlock()
					d.deviceLock.Unlock()
					d.restartLighting()
					if d.activeRgb != old {
						t.Fatalf("%s restart guard changed renderer ownership", guard)
					}
				}
				d.deviceLock.Lock()
				d.lightingMu.Lock()
				d.Connected, d.Exit = true, false
				d.DeviceProfile.RGBCluster, d.DeviceProfile.OpenRGBIntegration = false, false
				d.lightingMu.Unlock()
				d.deviceLock.Unlock()
			}
			d.lightingMu.Lock()
			stage = d.DeviceProfile.Profiles[0]
			stage.Color = original
			d.DeviceProfile.Profiles[0] = stage
			d.lightingMu.Unlock()
			if action == "restore" {
				awaitFrame()
				if d.activeRgb != old {
					t.Fatal("restoration replaced renderer")
				}
				return
			}
			done := make(chan error, 1)
			if action == "canonical-restart" {
				go func() { done <- d.SetLightingEffect("static") }()
			} else {
				target := *d.DeviceProfile
				target.Active = false
				target.RGBProfile = "static"
				target.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-nil-dpi.json")
				d.UserProfiles["nil-dpi"] = &target
				go func() { done <- d.switchLightingProfile("nil-dpi") }()
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("normal renderer retirement blocked after temporary nil DPI")
			}
			if d.activeRgb != nil {
				t.Fatal("normal retirement retained old renderer")
			}
			awaitFrame()
		})
	}
}

func TestNilDPIRenderLoopSleepsAndContinues(t *testing.T) {
	source, err := os.ReadFile("scimitarSEW.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "scimitarSEW.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if !ok || f.Name.Name != "setDeviceColor" {
			continue
		}
		ast.Inspect(f.Body, func(node ast.Node) bool {
			loop, ok := node.(*ast.GoStmt)
			if !ok {
				return true
			}
			ast.Inspect(loop, func(node ast.Node) bool {
				branch, ok := node.(*ast.IfStmt)
				if !ok {
					return true
				}
				condition := string(source[fset.Position(branch.Cond.Pos()).Offset:fset.Position(branch.Cond.End()).Offset])
				if condition != "dpiColor == nil" {
					return true
				}
				found = true
				if len(branch.Body.List) != 2 {
					t.Fatal("nil DPI branch changed lifecycle")
				}
				sleep, ok := branch.Body.List[0].(*ast.ExprStmt)
				if !ok {
					t.Fatal("nil DPI branch doesn't sleep")
				}
				expression := string(source[fset.Position(sleep.Pos()).Offset:fset.Position(sleep.End()).Offset])
				if expression != "time.Sleep(40 * time.Millisecond)" {
					t.Fatal("nil DPI polling interval changed")
				}
				next, ok := branch.Body.List[1].(*ast.BranchStmt)
				if !ok || next.Tok != token.CONTINUE || next.Label != nil {
					t.Fatal("nil DPI loop terminates instead of continuing")
				}
				return false
			})
			return false
		})
	}
	if !found {
		t.Fatal("dynamic nil DPI branch missing")
	}
}

// Characterize receiver initialization without running hardware discovery,
// listeners, macros, the queue worker, or any persistence.
func TestReceiverInitializationIdentityAndInputDefaultsUnchanged(t *testing.T) {
	source, err := os.ReadFile("scimitarSEW.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "scimitarSEW.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if !ok || f.Name.Name != "Init" {
			continue
		}
		text := string(source[fset.Position(f.Pos()).Offset:fset.Position(f.End()).Offset])
		text = strings.ReplaceAll(text, "\tif err := d.attachLightingRuntime(config.GetPaths()); err != nil {\n\t\tlogger.Log(logger.Fields{\"error\": err, \"serial\": d.Serial}).Warn(\"Canonical lighting unavailable; retaining legacy Lighting\")\n\t}\n", "")
		if fmt.Sprintf("%x", sha256.Sum256([]byte(text))) != "5a1c05e58d4c3f79e36272abcc244979d531add6dcc03d51f4775b254c7689e4" {
			t.Fatal("receiver initialization identity/input/defaults changed")
		}
		return
	}
	t.Fatal("receiver initialization missing")
}

func TestFailedCanonicalPersistenceEmitsNoFrames(t *testing.T) {
	d := newScimitarLightingTestDevice(t)
	d.lightingRestart = nil
	frames := 0
	d.lightingWrite = func([]byte) { frames++ }
	before, err := json.Marshal(d.DeviceProfile)
	if err != nil {
		t.Fatal(err)
	}
	value, err := d.ResolveLightingEffectSettings("static")
	if err != nil {
		t.Fatal(err)
	}
	value.SingleColor.Color.Red = 17
	for _, path := range []string{d.DeviceProfile.Path, filepath.Join(pwd, "database", "rgb", d.Serial+".json")} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if d.SetLightingEffect("mouse") == nil || d.SetLightingBrightness(50) == nil || d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Blue: 42}) == nil || d.SetLightingEffectSettings("static", value) == nil || d.ResetLightingEffectSettings("static") == nil {
		t.Fatal("failed persistence reported success")
	}
	after, err := json.Marshal(d.DeviceProfile)
	if err != nil || frames != 0 || d.activeRgb != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("failed persistence published state, restarted renderer or emitted frames")
	}
}

func TestLightingDarknessRecordedWhileReceiverUnavailable(t *testing.T) {
	for _, scheduler := range []bool{true, false} {
		for _, unavailable := range []string{"disconnected", "receiver-handle", "stopped"} {
			name := map[bool]string{true: "scheduler", false: "RGB-off"}[scheduler] + "/" + unavailable
			t.Run(name, func(t *testing.T) {
				d := newScimitarLightingTestDevice(t)
				receiver := d.dev
				setUnavailable := func() {
					switch unavailable {
					case "disconnected":
						d.Connected = false
					case "receiver-handle":
						d.dev = &common.Slipstream{}
					case "stopped":
						d.Exit = true
					}
				}
				restoreReady := func() {
					d.dev, d.Connected, d.Exit = receiver, true, false
					if err := d.lightingReady(); err != nil {
						t.Fatal(err)
					}
				}
				request := func(dark bool) {
					if scheduler {
						value := uint8(100)
						if dark {
							value = 0
						}
						if d.SchedulerBrightness(value) != 1 {
							t.Fatal("scheduler request rejected")
						}
					} else {
						d.ControlDeviceRgb(dark)
					}
				}
				assertFlags := func(dark bool) {
					t.Helper()
					if d.schedulerDark != (scheduler && dark) || d.userRGBOff != (!scheduler && dark) {
						t.Fatal("runtime override not retained")
					}
				}
				profileBefore, err := os.ReadFile(d.DeviceProfile.Path)
				if err != nil {
					t.Fatal(err)
				}
				rgbPath := filepath.Join(pwd, "database", "rgb", d.Serial+".json")
				rgbBefore, err := os.ReadFile(rgbPath)
				if err != nil {
					t.Fatal(err)
				}
				settings, err := d.ResolveLightingEffectSettings("static")
				if err != nil {
					t.Fatal(err)
				}
				frames, restarts := 0, 0
				var lastFrame []byte
				d.lightingWrite = func(frame []byte) { frames++; lastFrame = frame }
				d.lightingRestart = func() { restarts++; d.setDeviceColor() }
				setUnavailable()
				request(true)
				assertFlags(true)
				if frames != 0 || restarts != 0 || d.activeRgb != nil {
					t.Fatal("unavailable darkness request restarted or emitted output")
				}
				// This exception must not loosen authored-state mutation readiness.
				before, err := json.Marshal(d.DeviceProfile)
				if err != nil {
					t.Fatal(err)
				}
				if d.SetLightingEffect("mouse") == nil || d.SetLightingBrightness(71) == nil || d.SetLightingEffectSettings("static", settings) == nil || d.ResetLightingEffectSettings("static") == nil || d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Blue: 42}) == nil {
					t.Fatal("unavailable authored mutation accepted")
				}
				after, err := json.Marshal(d.DeviceProfile)
				if err != nil || !reflect.DeepEqual(before, after) || frames != 0 || restarts != 0 {
					t.Fatal("rejected authored mutation changed state/output")
				}
				profileAfter, err := os.ReadFile(d.DeviceProfile.Path)
				if err != nil || !reflect.DeepEqual(profileBefore, profileAfter) {
					t.Fatal("transient request or rejected mutation persisted profile state")
				}
				rgbAfter, err := os.ReadFile(rgbPath)
				if err != nil || !reflect.DeepEqual(rgbBefore, rgbAfter) {
					t.Fatal("rejected mutation persisted RGB settings")
				}
				// Connect already calls setDeviceColor. Exercise that restoration path
				// with an inert write hook instead of calling receiver hardware.
				restoreReady()
				assertFlags(true)
				d.setDeviceColor()
				_, effective, off := d.lightingOutputState()
				if effective != 0 || off != !scheduler || frames != 1 || !reflect.DeepEqual(lastFrame, make([]byte, d.LEDChannels*3)) {
					t.Fatal("readiness restoration lost recorded darkness")
				}
				// Persist a newer desired value while dark, then clear while asleep.
				if err := d.SetLightingBrightness(71); err != nil {
					t.Fatal(err)
				}
				profileBefore, err = os.ReadFile(d.DeviceProfile.Path)
				if err != nil {
					t.Fatal(err)
				}
				frames, restarts = 0, 0
				setUnavailable()
				request(false)
				assertFlags(false)
				profileAfter, err = os.ReadFile(d.DeviceProfile.Path)
				if err != nil || !reflect.DeepEqual(profileBefore, profileAfter) || frames != 0 || restarts != 0 {
					t.Fatal("unavailable clearing persisted/restarted/emitted output")
				}
				restoreReady()
				d.setDeviceColor()
				effect, effective, off := d.lightingOutputState()
				if effect != "static" || effective != 71 || off || frames != 1 || reflect.DeepEqual(lastFrame, make([]byte, d.LEDChannels*3)) {
					t.Fatal("cleared override failed to restore latest desired state")
				}
			})
		}
	}
}

func TestLightingDarknessRequiresCanonicalAttachmentAndProfile(t *testing.T) {
	for _, scheduler := range []bool{true, false} {
		for _, missing := range []string{"attachment", "profile"} {
			t.Run(fmt.Sprintf("scheduler=%t/%s", scheduler, missing), func(t *testing.T) {
				d := newScimitarLightingTestDevice(t)
				d.schedulerDark, d.userRGBOff = true, true
				if missing == "attachment" {
					d.lightingDefaults = nil
				} else {
					d.DeviceProfile = nil
				}
				frames, restarts := 0, 0
				d.lightingWrite = func([]byte) { frames++ }
				d.lightingRestart = func() { restarts++ }
				if d.setLightingDarkness(scheduler, false) == nil || !d.schedulerDark || !d.userRGBOff || frames != 0 || restarts != 0 {
					t.Fatal("missing canonical attachment/profile accepted transient mutation")
				}
			})
		}
	}
}
