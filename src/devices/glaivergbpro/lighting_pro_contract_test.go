package glaivergbpro

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

func TestProDPIProgrammingAcrossLightingOwnersAndDarkness(t *testing.T) {
	d, _ := lightingFixture(t, true)
	// Pro source defaults: regular0..4 at209..213, Sniper5 at208.
	d.DeviceProfile.Profiles = map[int]DPIProfile{}
	for id, value := range []uint16{800, 1500, 3000, 6000, 9000, 200} {
		d.DeviceProfile.Profiles[id] = DPIProfile{Value: value, Sniper: id == 5}
	}
	var writes [][]byte
	d.lightingDPIWrite = func(cmd, data []byte) error {
		if !bytes.Equal(cmd, cmdSaveDpi) {
			t.Fatal("unexpected command", cmd)
		}
		writes = append(writes, append([]byte(nil), data...))
		return nil
	}
	for _, owner := range []string{"native", "cluster", "openrgb"} {
		if owner == "cluster" {
			if d.ProcessSetRgbCluster(true) != 1 {
				t.Fatal(owner)
			}
		}
		if owner == "openrgb" {
			if d.ProcessSetOpenRgbIntegration(true) != 1 {
				t.Fatal(owner)
			}
		}
		requireLighting(t, d.setSchedulerDark(true))
		requireLighting(t, d.setUserRGBOff(true))
		before, _ := d.LightingSnapshot()
		writes = nil
		d.updateMouseDPI()
		if len(writes) != 6 {
			t.Fatal(owner, writes)
		}
		seen := map[byte]bool{}
		for _, data := range writes {
			if len(data) != 10 || data[1] != 0 || data[2] != 0 {
				t.Fatal(data)
			}
			id := int(data[0]) - 209
			color := []byte{0, 255, 0}
			if data[0] == 208 {
				id = 5
				color = []byte{255, 255, 0}
			}
			if id < 0 || id > 5 || seen[data[0]] {
				t.Fatal(data)
			}
			seen[data[0]] = true
			if binary.LittleEndian.Uint16(data[3:5]) != d.DeviceProfile.Profiles[id].Value ||
				!bytes.Equal(data[3:5], data[5:7]) || !bytes.Equal(data[7:10], color) {
				t.Fatal(owner, data)
			}
		}
		after, _ := d.LightingSnapshot()
		if !reflect.DeepEqual(before, after) {
			t.Fatal("input programmed authored Lighting")
		}
		if owner == "cluster" {
			if d.ProcessSetRgbCluster(false) != 1 {
				t.Fatal("release")
			}
		}
		if owner == "openrgb" {
			if d.ProcessSetOpenRgbIntegration(false) != 1 {
				t.Fatal("release")
			}
		}
	}
}

func TestProInputPersistenceFailureAndCombinedColorOutput(t *testing.T) {
	d, _ := lightingFixture(t, true)
	before := inputJSON(t, d)
	var output atomic.Int32
	d.lightingDPIWrite = func([]byte, []byte) error { output.Add(1); return nil }
	d.lightingProfileWrite = func(string, []byte) error { return errors.New("input failure") }
	if d.SaveMouseDPI(map[int]uint16{1: 18000}) != 0 || d.SelectMouseDPIStage(1) != 0 ||
		d.SaveMouseDPISettings(map[int]uint16{0: 200, 1: 18000}, map[int]rgb.Color{0: {Blue: 9}, 1: {Red: 8}}) != 0 {
		t.Fatal("input save error hidden")
	}
	if output.Load() != 0 || !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("input failure published")
	}
	d.lightingProfileWrite = nil
	if d.SaveMouseZoneColorsSniper(rgb.Color{Red: 20}, map[int]rgb.Color{0: {Blue: 50}}, rgb.Color{Blue: 30}) != 1 {
		t.Fatal("mixed save")
	}
	if output.Load() != 3 {
		t.Fatal("mixed endpoint must program two records then select", output.Load())
	}
	if d.DeviceProfile.DPIColor.Red != 20 || d.DeviceProfile.SniperColor.Blue != 30 || d.DeviceProfile.ZoneColors[0].Color.Red != 9 {
		t.Fatal("input/palette authority mixed")
	}
}

func TestProStoppedDesiredAndSchedulerAvailability(t *testing.T) {
	d, rt := lightingFixture(t, true)
	var output atomic.Int32
	d.lightingWrite = func([]byte) { output.Add(1) }
	d.exitLighting()
	// Source scheduler/brightness/preference mutations are not Connected-gated.
	// Canonical persistence remains available; stopped output stays suppressed.
	requireLighting(t, d.SetLightingBrightness(48))
	if d.SchedulerBrightness(0) != 1 || d.SchedulerBrightness(0) != 1 {
		t.Fatal("scheduler unavailable")
	}
	requireLighting(t, d.SetLightingBrightness(67))
	requireLighting(t, d.setUserRGBOff(true))
	if d.SchedulerBrightness(1) != 1 {
		t.Fatal("scheduler restore")
	}
	state, _, err := rt.State.Resolve(d.Serial)
	requireLighting(t, err)
	if state.Brightness != 67 || d.schedulerDark || !d.userRGBOff || output.Load() != 0 {
		t.Fatal("stopped state/output")
	}
	if *d.DeviceProfile.BrightnessSlider != 17 || d.DeviceProfile.OriginalBrightness != 82 {
		t.Fatal("scheduler legacy write")
	}
}

func TestProProfileRegistrationSessionsAndExactSelectionReplay(t *testing.T) {
	d, _ := lightingFixture(t, true)
	var callbacks []func([]byte, int)
	count := 0
	d.lightingClusterRemove = func(string) { count = 0 }
	d.lightingClusterRegister = func(c *common.ClusterController) { count++; callbacks = append(callbacks, c.WriteColorEx) }
	var openCallbacks []func([]byte, int)
	registered := false
	d.lightingOpenRGBRegister = func(c *common.OpenRGBController, replace bool) {
		if replace != registered {
			t.Fatal("OpenRGB replacement flag", replace, registered)
		}
		registered = true
		if c.Serial != d.Serial || c.DeviceType != common.DeviceTypeMouse || c.ColorMode != common.ColorModePerLed ||
			!reflect.DeepEqual(c.Zones, []common.OpenRGBZone{{Name: "Front", NumLEDs: 1, ZoneType: common.ZoneTypeLinear}, {Name: "Logo", NumLEDs: 1, ZoneType: common.ZoneTypeLinear}, {Name: "Side", NumLEDs: 1, ZoneType: common.ZoneTypeLinear}}) {
			t.Fatal("Pro target", c)
		}
		openCallbacks = append(openCallbacks, c.WriteColorEx)
	}
	d.setupOpenRGBController()
	d.setupOpenRGBController()
	if d.ProcessSetRgbCluster(true) != 1 {
		t.Fatal("cluster")
	}
	d.setupClusterController()
	d.setupClusterController()
	if count != 1 {
		t.Fatal("duplicate Cluster")
	}
	old := callbacks[len(callbacks)-1]
	p := cloneLightingProfile(*d.DeviceProfile)
	p.Path = filepath.Join(t.TempDir(), "open.json")
	p.RGBCluster = false
	p.OpenRGBIntegration = true
	p.Profile = 2
	d.UserProfiles["open"] = &p
	generation := d.externalGeneration
	if d.ChangeDeviceProfile("open") != 1 || count != 0 || d.externalGeneration == generation {
		t.Fatal("profile reconciliation")
	}
	var frames atomic.Int32
	d.lightingWrite = func([]byte) { frames.Add(1) }
	// Offline profile switch must not open HID or program DPI.
	old(make([]byte, 9), 0)
	if frames.Load() != 0 {
		t.Fatal("old Cluster session")
	}
	currentOpen := openCallbacks[len(openCallbacks)-1]
	currentOpen(make([]byte, 9), 0)
	queued := <-d.externalQueue
	if d.ChangeDeviceProfile("default") != 1 || count != 1 {
		t.Fatal("return Cluster")
	}
	d.applyLightingPacket(composeLightingPacket(queued.data), "openrgb", queued.generation, nil)
	currentOpen(make([]byte, 9), 0)
	select {
	case <-d.externalQueue:
		t.Fatal("old OpenRGB callback queued")
	default:
	}
	if frames.Load() != 0 {
		t.Fatal("old profile output")
	}
}

func TestProLegacyProfileMissingFieldAndRGBUpgrades(t *testing.T) {
	d, _ := lightingFixture(t, false)
	previous := pwd
	pwd = t.TempDir()
	defer func() { pwd = previous }()
	requireLighting(t, os.MkdirAll(filepath.Join(pwd, "database", "profiles"), 0700))
	d.Serial = "7028"
	d.DeviceProfile.Serial = d.Serial
	d.DeviceProfile.Path = filepath.Join(pwd, "database", "profiles", d.Serial+".json")
	d.DeviceProfile.BrightnessSlider = nil
	d.DeviceProfile.SleepMode = 0
	d.saveDeviceProfile()
	if d.DeviceProfile == nil || *d.DeviceProfile.BrightnessSlider != 100 || d.DeviceProfile.SleepMode != 15 || d.DeviceProfile.Profiles[1].Value != 800 {
		t.Fatal("profile upgrade")
	}
	// Existing RGB upgrade semantics stay in independent legacy fallback.
	// Source newer templates replace mismatched versions, not the catalogue.
	restore := config.UsePathsForTest(config.Paths{ShippedDatabaseRoot: "../../../database"})
	defer restore()
	rgb.Init()
	d.Rgb = &rgb.RGB{Profiles: map[string]rgb.Profile{"cpu-temperature": {Version: -1}}}
	path := filepath.Join(t.TempDir(), "rgb.json")
	d.upgradeRgbProfile(path, rgbProfileUpgrade)
	if d.GetRgbProfile("gradient") == nil || d.GetRgbProfile("pastelspiralrainbow") == nil || d.GetRgbProfile("cpu-temperature").Version != rgb.GetRgbProfile("cpu-temperature").Version {
		t.Fatal("RGB upgrade")
	}
	var saved rgb.RGB
	data, err := os.ReadFile(path)
	requireLighting(t, err)
	requireLighting(t, json.Unmarshal(data, &saved))
	if len(saved.Profiles) == 0 || d.SupportsLightingEffect("pastelspiralrainbow") {
		t.Fatal("upgrades changed selectable catalogue")
	}
}

func TestProStaticAndGradientRenderingCopies(t *testing.T) {
	d, _ := lightingFixture(t, true)
	requireLighting(t, d.SetLightingBrightness(50))
	v := frameState(t, d)
	start := time.Now()
	a := renderLightingFrame(v, rendererOwner(), &start)
	b := renderLightingFrame(v, rendererOwner(), &start)
	if !bytes.Equal(a, []byte{0, 127, 127, 0, 127, 127, 0, 127, 127}) || !bytes.Equal(a, b) {
		t.Fatal("Static scaling", a, b)
	}
	requireLighting(t, d.SetLightingEffect("gradient"))
	v = frameState(t, d)
	original := copyLightingRGBProfile(*v.profile)
	_ = renderLightingFrame(v, rendererOwner(), &start)
	if !reflect.DeepEqual(*v.profile, original) {
		t.Fatal("gradient renderer mutated authored settings")
	}
}

func TestProAttachmentRejectsPartialPaletteAndConflictingOwners(t *testing.T) {
	for _, invalid := range []string{"palette", "owners"} {
		t.Run(invalid, func(t *testing.T) {
			d, rt := lightingFixture(t, false)
			if invalid == "palette" {
				requireLighting(t, rt.Effects.SetAuthoredZones(d.Serial, "mouse", map[string]lightingsettings.Color{"0": {Red: 1}}))
			} else {
				d.DeviceProfile.RGBCluster = true
				d.DeviceProfile.OpenRGBIntegration = true
			}
			before := inputJSON(t, d)
			if d.attachLightingSource(rt) == nil || d.lightingCanonical != nil || d.externalQueue != nil || d.lightingWorker != nil || d.clusterOwned || d.openRGBOwned {
				t.Fatal("partial attachment")
			}
			if !bytes.Equal(before, inputJSON(t, d)) || d.GetRgbProfile("static") == nil {
				t.Fatal("fallback disturbed")
			}
		})
	}
}

func TestProTemperatureSourcesAndGradientMutationValidation(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.CpuTemp, d.GpuTemp = 20, 95
	start := time.Now()
	requireLighting(t, d.SetLightingEffect("cpu-temperature"))
	cpu := renderLightingFrame(frameState(t, d), rendererOwner(), &start)
	requireLighting(t, d.SetLightingEffect("gpu-temperature"))
	gpu := renderLightingFrame(frameState(t, d), rendererOwner(), &start)
	if !bytes.Equal(cpu, []byte{0, 255, 0, 0, 255, 0, 0, 255, 0}) || !bytes.Equal(gpu, []byte{255, 0, 0, 255, 0, 0, 255, 0, 0}) {
		t.Fatal("temperature sources", cpu, gpu)
	}
	gradient, err := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, err)
	original := gradient.Clone()
	gradient.Gradient.Stops[0].Position = -1
	if d.SetLightingEffectSettings("gradient", gradient) == nil {
		t.Fatal("invalid gradient accepted")
	}
	after, err := d.ResolveLightingEffectSettings("gradient")
	requireLighting(t, err)
	if !reflect.DeepEqual(after, original) {
		t.Fatal("validation partially mutated stops")
	}
	if code, _ := d.ProcessNewGradientColor("gradient"); code != 1 {
		t.Fatal("add gradient")
	}
	if code, _ := d.ProcessDeleteGradientColor("gradient"); code != 1 {
		t.Fatal("delete gradient")
	}
}

func TestProProfileSwitchSelectsStageWithoutReprogrammingDPIColors(t *testing.T) {
	d, _ := lightingFixture(t, true)
	p := cloneLightingProfile(*d.DeviceProfile)
	p.Path = filepath.Join(t.TempDir(), "next.json")
	p.Profile = 3
	p.DPIColor = &rgb.Color{Red: 11}
	p.SniperColor = &rgb.Color{Blue: 22}
	d.UserProfiles["next"] = &p
	var commands, payloads [][]byte
	d.lightingDPIWrite = func(cmd, data []byte) error {
		commands = append(commands, append([]byte(nil), cmd...))
		payloads = append(payloads, append([]byte(nil), data...))
		return nil
	}
	before, _ := d.LightingSnapshot()
	if d.ChangeDeviceProfile("next") != 1 {
		t.Fatal("profile switch")
	}
	if len(commands) != 1 || !bytes.Equal(commands[0], []byte{0x13, 0x02, 0x00}) || !bytes.Equal(payloads[0], []byte{3}) {
		t.Fatal("profile changed source DPI replay contract", commands, payloads)
	}
	after, _ := d.LightingSnapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("profile replaced desired Lighting")
	}
	if d.DeviceProfile.DPIColor.Red != 11 || d.DeviceProfile.SniperColor.Blue != 22 {
		t.Fatal("input colors lost")
	}
	// Replay all programmed records remains the existing explicit operation.
	commands, payloads = nil, nil
	d.updateMouseDPI()
	if len(commands) != 2 || !bytes.Equal(commands[0], []byte{0x13}) || !bytes.Equal(commands[1], []byte{0x13}) {
		t.Fatal("DPI programming", commands)
	}
}
