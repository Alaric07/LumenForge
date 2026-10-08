package scufenvisionproW

import (
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/inputmanager"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/sstallion/go-hid"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testLightingDevice(t *testing.T) *Device {
	t.Helper()
	logger.Init()
	old := pwd
	pwd = t.TempDir()
	t.Cleanup(func() { pwd = old })
	for _, dir := range []string{"profiles", "rgb"} {
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
	brightness := uint8(70)
	zones := map[int]ZoneColors{}
	zones[0] = ZoneColors{Name: "Controller", ColorIndex: []int{0, 9, 18}, Color: &rgb.Color{Red: 200, Green: 100, Blue: 40, Brightness: 1}}

	profile := &DeviceProfile{Active: true, Serial: "scufenvisionproW-test", RGBProfile: "controller", BrightnessSlider: brightness, ZoneColors: zones, Path: filepath.Join(pwd, "database", "profiles", "scufenvisionproW-test.json")}
	d := &Device{Serial: profile.Serial, dev: &hid.Device{}, Connected: true, LEDChannels: 9, ChangeableLedChannels: 9, DeviceProfile: profile, Rgb: &profiles, UserProfiles: map[string]*DeviceProfile{"default": profile}, lightingRestart: func() {}}
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
func TestExactLightingCatalogueTopologyAndAuthority(t *testing.T) {
	d := testLightingDevice(t)
	s, ok := d.LightingSnapshot()
	want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "controller", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	got := []string{}
	for _, e := range s.SupportedEffects {
		got = append(got, e.ID)
	}
	if !ok || !reflect.DeepEqual(got, want) || s.ConfiguredEffect != "controller" || s.Brightness != 70 || s.ClusterControlled || s.ExternalControlled || len(s.Channels) != 0 {
		t.Fatalf("snapshot=%#v", s)
	}
	for _, bad := range []string{"mouse", "custom", "circle", "circleshift", "spinner", "liquid-temperature", "probe-temperature"} {
		if d.SupportsLightingEffect(bad) || d.SetLightingEffect(bad) == nil {
			t.Fatal(bad)
		}
	}
	if s.AuthoredZoneEditor == nil || len(s.AuthoredZoneEditor.Zones) != 1 || s.AuthoredZoneEditor.Zones[0].Label != "Controller" || s.AuthoredZoneEditor.Zones[0].ColorHex != "#c86428" || !reflect.DeepEqual(d.DeviceProfile.ZoneColors[0].ColorIndex, []int{0, 9, 18}) {
		t.Fatal("topology")
	}

	for _, effect := range want {
		d.DeviceProfile.RGBProfile = effect
		if s, ok = d.LightingSnapshot(); !ok || !s.EffectSupported {
			t.Fatalf("effect %s", effect)
		}
	}
	d.DeviceProfile.RGBProfile = "controller"
	before, _ := json.Marshal(d.DeviceProfile)
	d.leftTriggerArmed = true
	d.listener = &hid.Device{}
	s, _ = d.LightingSnapshot()
	after, _ := json.Marshal(d.DeviceProfile)
	if !reflect.DeepEqual(before, after) || s.ConfiguredEffect != "controller" {
		t.Fatal("mic became authority")
	}
}
func TestPersistenceOrderingRollbackAndRestarts(t *testing.T) {
	d := testLightingDevice(t)
	count := 0
	d.lightingRestart = func() {
		count++
		data, err := os.ReadFile(d.DeviceProfile.Path)
		var p DeviceProfile
		if err != nil || json.Unmarshal(data, &p) != nil || p.RGBProfile != d.DeviceProfile.RGBProfile || p.BrightnessSlider != d.DeviceProfile.BrightnessSlider {
			t.Fatal("restart before persist")
		}
	}
	if err := d.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	if d.ChangeDeviceBrightnessValue(35) != 1 || count != 2 {
		t.Fatal("static restart")
	}
	value, _ := d.ResolveLightingEffectSettings("static")
	value.SingleColor.Color.Red = 27
	if err := d.SetLightingEffectSettings("static", value); err != nil || count != 3 {
		t.Fatal(err)
	}
	if err := d.ResetLightingEffectSettings("static"); err != nil {
		t.Fatal(err)
	}
	reset, _ := d.ResolveLightingEffectSettings("static")
	defaults, _ := d.lightingDefaults.Get("static")
	if !reflect.DeepEqual(reset, defaults) {
		t.Fatal("reset")
	}
	if err := d.SetLightingEffect("wave"); err != nil {
		t.Fatal(err)
	}
	c := count
	if d.SetLightingBrightness(42) != nil || count != c {
		t.Fatal("dynamic brightness restart")
	}
	for _, kind := range []string{"profile", "rgb"} {
		t.Run(kind, func(t *testing.T) {
			before, _ := json.Marshal(d.DeviceProfile)
			rgbBefore, _ := json.Marshal(d.Rgb)
			c := count
			path := d.DeviceProfile.Path
			if kind == "rgb" {
				path = filepath.Join(pwd, "database", "rgb", d.Serial+".json")
			}
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err = os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			if kind == "profile" {
				if d.SetLightingEffect("controller") == nil || d.SetLightingBrightness(7) == nil || d.SetLightingZoneColors("controller", []string{"0"}, rgb.Color{Red: 1}) == nil || d.SaveControllerZoneColors(map[int]rgb.Color{0: {Red: 2}}) != 0 {
					t.Fatal("failed write accepted")
				}
			} else {
				if d.SetLightingEffectSettings("static", value) == nil || d.ResetLightingEffectSettings("static") == nil {
					t.Fatal("failed rgb write accepted")
				}
				if code, _ := d.ProcessNewGradientColor("gradient"); code != 0 {
					t.Fatal("gradient publication")
				}
			}
			after, _ := json.Marshal(d.DeviceProfile)
			rgbAfter, _ := json.Marshal(d.Rgb)
			if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(rgbBefore, rgbAfter) || count != c {
				t.Fatal("rollback/restart")
			}
			if err = os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestTransientOverridesLatestDesiredAndCopies(t *testing.T) {
	d := testLightingDevice(t)
	for _, dark := range []bool{true} {
		if err := d.setLightingDarkness(dark); err != nil {
			t.Fatal(err)
		}
		if d.SetLightingBrightness(29) != nil || d.SetLightingEffect("controller") != nil || d.SetLightingZoneColors("controller", []string{"0"}, rgb.Color{Red: 123, Green: 44}) != nil {
			t.Fatal("dark mutation")
		}
		value, _ := d.ResolveLightingEffectSettings("static")
		value.SingleColor.Color.Blue = 19
		if d.SetLightingEffectSettings("static", value) != nil {
			t.Fatal("dark settings")
		}
		_, brightness, _ := d.lightingOutputState()
		s, _ := d.LightingSnapshot()
		if brightness != 0 || s.Brightness != 29 || s.AuthoredZoneEditor.Zones[0].ColorHex != "#7b2c00" {
			t.Fatal("desired overwritten")
		}
		before, _ := json.Marshal(d.DeviceProfile)
		zones := d.lightingOutputZones(0, true)
		zones[0].Color.Red = 255
		zones[0].ColorIndex[0] = 99
		after, _ := json.Marshal(d.DeviceProfile)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("pointer alias")
		}
		if d.setLightingDarkness(false) != nil {
			t.Fatal("clear")
		}
		_, brightness, _ = d.lightingOutputState()
		if brightness != 29 {
			t.Fatal("latest desired not restored")
		}
	}
	p := d.GetRgbProfile("gradient")
	p.Gradients[0] = rgb.Color{Red: 1}
	if reflect.DeepEqual(p.Gradients, d.Rgb.Profiles["gradient"].Gradients) {
		t.Fatal("gradient alias")
	}
}
func TestProfileSwitchLegacyPathsFailureAndOverrideRetirement(t *testing.T) {
	d := testLightingDevice(t)
	current := d.DeviceProfile
	target := *current
	target.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-gaming.json")
	target.BrightnessSlider = 0
	target.RgbOff = true
	d.UserProfiles["gaming"] = &target
	d.setLightingDarkness(true)
	if err := os.Mkdir(target.Path, 0700); err != nil {
		t.Fatal(err)
	}
	file, _ := os.ReadFile(current.Path)
	c := 0
	d.lightingRestart = func() { c++ }
	if d.switchLightingProfile("gaming") == nil || d.DeviceProfile != current || target.BrightnessSlider != 0 || c != 0 {
		t.Fatal("switch rollback")
	}
	stored, _ := os.ReadFile(current.Path)
	if !reflect.DeepEqual(file, stored) {
		t.Fatal("disk rollback")
	}
	os.Remove(target.Path)
	old := rgb.Exit()
	old.Exit = make(chan bool, 1)
	d.activeRgb = old
	if err := d.switchLightingProfile("gaming"); err != nil {
		t.Fatal(err)
	}
	if d.DeviceProfile != &target || target.BrightnessSlider != 0 || d.userRGBOff || target.RgbOff || c != 1 || d.activeRgb != nil || len(old.Exit) != 1 {
		t.Fatal("switch normalization")
	}
}
func TestUnavailableGuardsAndAttachFailure(t *testing.T) {
	for _, broken := range []string{"runtime", "disconnected", "stopped", "hid", "profile", "brightness", "zones", "mapping", "rgb", "effect"} {
		t.Run(broken, func(t *testing.T) {
			d := testLightingDevice(t)
			value, _ := d.ResolveLightingEffectSettings("static")
			switch broken {
			case "runtime":
				d.lightingDefaults = nil
			case "disconnected":
				d.Connected = false
			case "stopped":
				d.Exit = true
			case "hid":
				d.dev = nil
			case "profile":
				d.DeviceProfile = nil
			case "brightness":
				d.DeviceProfile.BrightnessSlider = 101
			case "zones":
				z := d.DeviceProfile.ZoneColors[0]
				z.Color = nil
				d.DeviceProfile.ZoneColors[0] = z
			case "mapping":
				d.DeviceProfile.ZoneColors[0].ColorIndex[0] = 8
			case "rgb":
				d.Rgb = nil
			case "effect":
				delete(d.Rgb.Profiles, "wave")
			}
			before, _ := json.Marshal(d.DeviceProfile)
			if _, ok := d.LightingSnapshot(); ok {
				t.Fatal("available")
			}
			if d.SetLightingEffect("controller") == nil || d.SetLightingBrightness(33) == nil || d.SetLightingEffectSettings("static", value) == nil || d.ResetLightingEffectSettings("static") == nil || d.SetLightingZoneColors("controller", []string{"0"}, rgb.Color{}) == nil || d.switchLightingProfile("default") == nil {
				t.Fatal("unguarded mutation")
			}
			after, _ := json.Marshal(d.DeviceProfile)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("guard mutated")
			}
		})
	}
	d := testLightingDevice(t)
	d.lightingDefaults = nil
	if d.attachLightingRuntime(config.Paths{ShippedDatabaseRoot: "missing"}) == nil || d.lightingDefaults != nil {
		t.Fatal("attachment")
	}
}
func TestRealDynamicRendererRetiresAfterCanonicalMutation(t *testing.T) {
	d := testLightingDevice(t)
	d.lightingRestart = nil
	frames := make(chan []byte, 16)
	d.lightingWrite = func(data []byte) {
		select {
		case frames <- data:
		default:
		}
	}
	if d.SetLightingEffect("wave") != nil {
		t.Fatal("wave")
	}
	select {
	case <-frames:
	case <-time.After(time.Second):
		t.Fatal("no frame")
	}
	if d.SetLightingBrightness(21) != nil {
		t.Fatal("brightness")
	}
	done := make(chan error, 1)
	go func() { done <- d.SetLightingEffect("controller") }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("renderer stop blocked")
	}
	if d.activeRgb != nil {
		t.Fatal("old renderer retained")
	}
}
func TestGenericRGBFailureRetainsRendererAndGradientMaps(t *testing.T) {
	d := testLightingDevice(t)
	old := rgb.Exit()
	old.Exit = make(chan bool, 1)
	d.activeRgb = old
	count := 0
	d.lightingRestart = func() { count++ }
	before, _ := json.Marshal(d.Rgb)
	path := filepath.Join(pwd, "database", "rgb", d.Serial+".json")
	os.Remove(path)
	os.Mkdir(path, 0700)
	value, _ := d.ResolveLightingEffectSettings("gradient")
	value.Gradient.Stops[0].Color.Blue = 7
	if d.SetLightingEffectSettings("gradient", value) == nil {
		t.Fatal("failed write accepted")
	}
	if code, _ := d.ProcessNewGradientColor("gradient"); code != 0 {
		t.Fatal("add failed write")
	}
	if code, _ := d.ProcessDeleteGradientColor("gradient"); code == 1 {
		t.Fatal("delete failed write")
	}
	after, _ := json.Marshal(d.Rgb)
	if !reflect.DeepEqual(before, after) || d.activeRgb != old || len(old.Exit) != 0 || count != 0 {
		t.Fatal("failed persistence retired renderer or changed RGB")
	}
}

func TestControllerOutputAndNonLightingState(t *testing.T) {
	d := testLightingDevice(t)
	d.DeviceProfile.LeftVibrationValue = 61
	d.DeviceProfile.RightVibrationValue = 52
	d.DeviceProfile.AnalogData = map[int]AnalogData{0: {DeadZoneMin: 5, Points: map[int]common.CurveData{0: {X: 20, Y: 20}}}}
	d.DeviceProfile.KeyAssignmentHash = "unchanged"
	d.leftTriggerArmed = true
	d.rightTriggerPressed = true
	d.leftStick.lastX = 123
	d.KeyAssignment = map[int]inputmanager.KeyAssignment{1: {}}
	beforeAnalog, _ := json.Marshal(d.DeviceProfile.AnalogData)
	if d.SetLightingBrightness(25) != nil || d.SetLightingZoneColors("controller", []string{"0"}, rgb.Color{Red: 200, Green: 100, Blue: 40}) != nil {
		t.Fatal("mutation")
	}
	d.lightingRestart = nil
	var frame []byte
	d.lightingWrite = func(data []byte) { frame = append([]byte(nil), data...) }
	before, _ := json.Marshal(d.DeviceProfile)
	d.setDeviceColor()
	after, _ := json.Marshal(d.DeviceProfile)
	color := *d.DeviceProfile.ZoneColors[0].Color
	color.Brightness = .25
	scaled := rgb.ModifyBrightness(color)
	for channel, want := range []byte{byte(scaled.Red), byte(scaled.Green), byte(scaled.Blue)} {
		for i := 0; i < 9; i++ {
			if frame[channel*9+i] != want {
				t.Fatal("controller byte mapping")
			}
		}
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("render mutated authored pointers")
	}
	d.ControlDeviceRgb(true)
	if !reflect.DeepEqual(frame, make([]byte, 27)) {
		t.Fatal("off output")
	}
	if d.SetLightingBrightness(80) != nil {
		t.Fatal("off brightness")
	}
	if !reflect.DeepEqual(frame, make([]byte, 27)) {
		t.Fatal("off mutation output")
	}
	d.ControlDeviceRgb(false)
	if d.DeviceProfile.BrightnessSlider != 80 || frame[0] == 0 {
		t.Fatal("restore")
	}
	analog, _ := json.Marshal(d.DeviceProfile.AnalogData)
	if !reflect.DeepEqual(beforeAnalog, analog) || d.DeviceProfile.LeftVibrationValue != 61 || d.DeviceProfile.RightVibrationValue != 52 || d.DeviceProfile.KeyAssignmentHash != "unchanged" || !d.leftTriggerArmed || !d.rightTriggerPressed || d.leftStick.lastX != 123 || len(d.KeyAssignment) != 1 {
		t.Fatal("controller state changed")
	}
}
func TestNoInventedSchedulerOrExternalOwnership(t *testing.T) {
	typ := reflect.TypeOf(&Device{})
	for _, method := range []string{"SchedulerBrightness", "ProcessSetRgbCluster", "ProcessSetOpenRgbIntegration"} {
		if _, ok := typ.MethodByName(method); ok {
			t.Fatal(method)
		}
	}
}

func TestPackageLocalHardwareIdentityAndLifecyclePreserved(t *testing.T) {
	source, err := os.ReadFile("scufenvisionproW.go")
	if err != nil {
		t.Fatal(err)
	}
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "device.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"SetConnected": "8e3e0313bb6e1ded90a33433bf66f75c3dc64d3603daed643a2b898767d00e7e", "Connect": "4c1f03b589828215050d8f99312fc0f5ff2bef6a39445b6580afae982c2d167b", "Stop": "6f7649f7b0eda8e75b0f4cd0f716664d61edc40d3c047ccf46619daa3c6e7c42", "StopInternal": "05d0f484a9afc05a16ba3a5d3e2eded2d76b611222b7b1b67e7ab3299abd5afa", "StopDirty": "1583f16f41e50747805d05cb85dbe83b41197fe717892acc523d944c5893bd02", "setAnalogDevice": "3e2d1a11766adc143fae7c40fc2ae93188712b355de5613d1da8ee777c0ccc1c", "setupAnalogDevices": "47f8aa237739ab5422746f2e657fca3752ccb47e72fe39da85c1123a780d9d04", "initTriggerEndpoint": "10b7b3ff9cfd4b46131a12c50e56f2d0819776bfedb64ae8c73452b5f364c025", "analogDataListener": "25daab9bbb7ebb685b3f10d335ca912b6cae0fcd59f637350e34b296b2345564", "setVibrationModuleValues": "c4a4dd7c0c4a96a6cee6dda0dde4be71831935d1f6ffa2aa307cd2f6960fc775", "setupKeyAssignment": "0b788538d9d66c60165277a4fe461caa3618d3ba3ef455616696c7531d283ad0", "loadKeyAssignments": "ba0fde58e0bc4674006c80041049cd2b0c524f2bc819add60e37f8715293c342", "transfer": "5d432585bb6e0b4758bbcc89860f2252dbc1c2b02f7f64a0c37e5535a3abd30d", "setTemperatures": "4e70de6cba6ce5872a84347b9c1bde2a11b40e5aedeb3db24c6ed0932d311239"}
	found := 0
	for _, decl := range file.Decls {
		f, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		expected, ok := want[f.Name.Name]
		if !ok {
			continue
		}
		found++
		body := source[fs.Position(f.Pos()).Offset:fs.Position(f.End()).Offset]
		if fmt.Sprintf("%x", sha256.Sum256(body)) != expected {
			t.Fatal(f.Name.Name)
		}
	}
	if found != len(want) {
		t.Fatal("missing hardware methods")
	}
	if !strings.Contains(string(source), "/database/key-assignments/scufenvisionpro.json") {
		t.Fatal("identity resource")
	}
}
func TestCompleteGenericSettingsResetAndMetadata(t *testing.T) {
	d := testLightingDevice(t)
	count := 0
	d.lightingRestart = func() { count++ }
	for _, effect := range []string{"static", "colorpulse", "cpu-temperature", "gpu-temperature", "gradient", "wave"} {
		before := *d.GetRgbProfile(effect)
		value, err := d.ResolveLightingEffectSettings(effect)
		if err != nil {
			t.Fatal(err)
		}
		if value.SingleColor != nil {
			value.SingleColor.Color.Red = 31
		}
		if value.TwoColor != nil {
			value.TwoColor.Start.Green = 47
			value.TwoColor.End.Blue = 71
		}
		if value.Temperature != nil {
			value.Temperature.Low.Color.Red = 23
			value.Temperature.Middle.Celsius = 42
		}
		if value.Speed != nil {
			*value.Speed = 2
		}
		if value.Gradient != nil {
			value.Gradient.Stops[0].Color.Red = 37
		}
		if err = d.SetLightingEffectSettings(effect, value); err != nil {
			t.Fatalf("%s: %v", effect, err)
		}
		resolved, _ := d.ResolveLightingEffectSettings(effect)
		if !reflect.DeepEqual(value, resolved) {
			t.Fatal(effect)
		}
		after := d.GetRgbProfile(effect)
		if before.MinTemp != after.MinTemp || before.MaxTemp != after.MaxTemp || before.Smoothness != after.Smoothness || before.Version != after.Version || before.Brightness != after.Brightness || before.StartColor.Brightness != after.StartColor.Brightness {
			t.Fatal("renderer metadata")
		}
		data, err := os.ReadFile(filepath.Join(pwd, "database", "rgb", d.Serial+".json"))
		var persisted rgb.RGB
		if err != nil || json.Unmarshal(data, &persisted) != nil || !reflect.DeepEqual(persisted.Profiles[effect], *after) {
			t.Fatal("persisted settings")
		}
		if err = d.ResetLightingEffectSettings(effect); err != nil {
			t.Fatal(err)
		}
		reset, _ := d.ResolveLightingEffectSettings(effect)
		defaults, _ := d.lightingDefaults.Get(effect)
		if !reflect.DeepEqual(reset, defaults) {
			t.Fatal("reset", effect)
		}
	}
	if count != 0 {
		t.Fatal("inactive settings restarted")
	}
	d.DeviceProfile.RGBProfile = "gradient"
	before := d.GetRgbProfile("gradient")
	code, key := d.ProcessNewGradientColor("gradient")
	if code != 1 || len(d.GetRgbProfile("gradient").Gradients) != len(before.Gradients)+1 {
		t.Fatal("gradient add")
	}
	code, deleted := d.ProcessDeleteGradientColor("gradient")
	if code != 1 || deleted != key || !reflect.DeepEqual(before.Gradients, d.GetRgbProfile("gradient").Gradients) || count != 2 {
		t.Fatal("gradient delete")
	}
}

func TestProfileSwitchValidationAndLegacyDefaults(t *testing.T) {
	d := testLightingDevice(t)
	current := d.DeviceProfile
	target := *current
	target.Active = false
	target.Path = "/old/data/" + d.Serial + "-gaming.json"
	target.BrightnessSlider = 0 // A scalar zero is valid desired brightness, not a missing pointer.
	target.LeftVibrationValue = 63
	target.RightVibrationValue = 54
	target.KeyAssignmentHash = "gaming"
	d.UserProfiles["gaming"] = &target
	invalid := target
	invalid.ZoneColors = nil
	d.UserProfiles["invalid"] = &invalid
	before, _ := os.ReadFile(current.Path)
	if d.switchLightingProfile("invalid") == nil || d.switchLightingProfile("missing") == nil || d.DeviceProfile != current {
		t.Fatal("invalid switch")
	}
	after, _ := os.ReadFile(current.Path)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("invalid target persisted")
	}
	if d.switchLightingProfile("gaming") != nil {
		t.Fatal("valid switch")
	}
	if target.Path != filepath.Join(pwd, "database", "profiles", d.Serial+"-gaming.json") || target.BrightnessSlider != 0 || target.LeftVibrationValue != 63 || target.RightVibrationValue != 54 || target.KeyAssignmentHash != "gaming" {
		t.Fatal("legacy path or controller fields")
	}
	if target.ZoneColors[0].Color == current.ZoneColors[0].Color {
		t.Fatal("profile color alias")
	}
	target.ZoneColors[0].Color.Red = 11
	target.ZoneColors[0].ColorIndex[0] = 99
	if current.ZoneColors[0].Color.Red == 11 || current.ZoneColors[0].ColorIndex[0] == 99 {
		t.Fatal("profile mapping alias")
	}
}
