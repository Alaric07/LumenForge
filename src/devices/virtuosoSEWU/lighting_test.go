package virtuosoSEWU

import (
	"LumenForge/src/config"
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
	for id, name := range []string{"Logo", "Microphone", "Indicator LED"} {
		indices := [][3]int{{0, 3, 6}, {2, 5, 8}, {1, 4, 7}}
		zones[id] = ZoneColors{Name: name, ColorIndex: append([]int(nil), indices[id][:]...), Color: &rgb.Color{Red: 200, Green: 100, Blue: 40, Brightness: 1}}
	}
	profile := &DeviceProfile{Active: true, Serial: "virtuosoSEWU-test", RGBProfile: "headset", BrightnessSlider: &brightness, ZoneColors: zones, Equalizers: map[int]Equalizer{1: {Name: "32", Value: 2}}, Path: filepath.Join(pwd, "database", "profiles", "virtuosoSEWU-test.json")}
	d := &Device{Serial: profile.Serial, dev: &hid.Device{}, Connected: true, Usb: true, LEDChannels: 3, ChangeableLedChannels: 3, DeviceProfile: profile, Rgb: &profiles, UserProfiles: map[string]*DeviceProfile{"default": profile}, lightingRestart: func() {}}
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
	want := []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "headset", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}
	got := []string{}
	for _, e := range s.SupportedEffects {
		got = append(got, e.ID)
	}
	if !ok || !reflect.DeepEqual(got, want) || s.ConfiguredEffect != "headset" || s.Brightness != 70 || s.ClusterControlled || s.ExternalControlled || len(s.Channels) != 0 {
		t.Fatalf("snapshot=%#v", s)
	}
	for _, bad := range []string{"mouse", "custom", "circle", "circleshift", "spinner", "liquid-temperature", "probe-temperature"} {
		if d.SupportsLightingEffect(bad) || d.SetLightingEffect(bad) == nil {
			t.Fatal(bad)
		}
	}
	for id, name := range []string{"Logo", "Microphone", "Indicator LED"} {
		indices := [][3]int{{0, 3, 6}, {2, 5, 8}, {1, 4, 7}}
		if s.AuthoredZoneEditor.Zones[id].Label != name || !reflect.DeepEqual(d.DeviceProfile.ZoneColors[id].ColorIndex, indices[id][:]) || s.AuthoredZoneEditor.Zones[id].ColorHex != "#c86428" {
			t.Fatal("zones")
		}
	}
	for _, effect := range want {
		d.DeviceProfile.RGBProfile = effect
		if s, ok = d.LightingSnapshot(); !ok || !s.EffectSupported {
			t.Fatalf("effect %s", effect)
		}
	}
	d.DeviceProfile.RGBProfile = "headset"
	before, _ := json.Marshal(d.DeviceProfile)
	d.MuteStatus = 1
	d.listener = &hid.Device{}
	s, _ = d.LightingSnapshot()
	after, _ := json.Marshal(d.DeviceProfile)
	if !reflect.DeepEqual(before, after) || s.ConfiguredEffect != "headset" {
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
		if err != nil || json.Unmarshal(data, &p) != nil || p.RGBProfile != d.DeviceProfile.RGBProfile || *p.BrightnessSlider != *d.DeviceProfile.BrightnessSlider {
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
				if d.SetLightingEffect("headset") == nil || d.SetLightingBrightness(7) == nil || d.SetLightingZoneColors("headset", []string{"0"}, rgb.Color{Red: 1}) == nil || d.SaveHeadsetZoneColors(map[int]rgb.Color{1: {Red: 2}}) != 0 {
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
	for _, scheduler := range []bool{true, false} {
		if err := d.setLightingDarkness(scheduler, true); err != nil {
			t.Fatal(err)
		}
		if d.SetLightingBrightness(29) != nil || d.SetLightingEffect("headset") != nil || d.SetLightingZoneColors("headset", []string{"0"}, rgb.Color{Red: 123, Green: 44}) != nil {
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
		if d.setLightingDarkness(scheduler, false) != nil {
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
func TestProfileSwitchNormalizationFailureAndOverrideRetirement(t *testing.T) {
	d := testLightingDevice(t)
	current := d.DeviceProfile
	target := *current
	target.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-gaming.json")
	target.BrightnessSlider = nil
	target.Equalizers = nil
	target.RgbOff = true
	d.UserProfiles["gaming"] = &target
	d.setLightingDarkness(true, true)
	d.setLightingDarkness(false, true)
	if err := os.Mkdir(target.Path, 0700); err != nil {
		t.Fatal(err)
	}
	file, _ := os.ReadFile(current.Path)
	c := 0
	d.lightingRestart = func() { c++ }
	if d.switchLightingProfile("gaming") == nil || d.DeviceProfile != current || target.BrightnessSlider != nil || target.Equalizers != nil || c != 0 {
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
	if d.DeviceProfile != &target || *target.BrightnessSlider != 100 || len(target.Equalizers) != 10 || d.schedulerDark || d.userRGBOff || target.RgbOff || c != 1 || d.activeRgb != nil || len(old.Exit) != 1 {
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
				*d.DeviceProfile.BrightnessSlider = 101
			case "zones":
				z := d.DeviceProfile.ZoneColors[1]
				z.Color = nil
				d.DeviceProfile.ZoneColors[1] = z
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
			if d.SetLightingEffect("headset") == nil || d.SetLightingBrightness(33) == nil || d.SetLightingEffectSettings("static", value) == nil || d.ResetLightingEffectSettings("static") == nil || d.SetLightingZoneColors("headset", []string{"0"}, rgb.Color{}) == nil || d.switchLightingProfile("default") == nil {
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
func TestActualAuthoredAndMicrophoneOutputPreservesIndicatorsAndProfile(t *testing.T) {
	d := testLightingDevice(t)
	d.lightingRestart = nil
	var frame []byte
	d.lightingWrite = func(data []byte) { frame = append([]byte(nil), data...) }
	for _, muted := range []byte{0, 1} {
		for _, disable := range []int{0, 1} {
			d.MuteStatus = muted
			d.DeviceProfile.DisableMicIndicator = disable
			before, _ := json.Marshal(d.DeviceProfile)
			d.setDeviceColor()
			after, _ := json.Marshal(d.DeviceProfile)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("output changed stored colors")
			}
			for _, id := range []int{0, 2} {
				zone := d.DeviceProfile.ZoneColors[id]
				color := *zone.Color
				color.Brightness = .7
				color = *rgb.ModifyBrightness(color)
				want := []byte{byte(color.Red), byte(color.Green), byte(color.Blue)}
				for channel, index := range zone.ColorIndex {
					if frame[index] != want[channel] {
						t.Fatalf("indicator/logo frame=%v", frame)
					}
				}
			}
			if muted == 1 && (frame[2] != 255 || frame[5] != 0 || frame[8] != 0) {
				t.Fatal("mute overlay")
			}
			if muted == 0 && disable == 1 && (frame[2] != 0 || frame[5] != 0 || frame[8] != 0) {
				t.Fatal("mic disabled")
			}
		}
	}
	input := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	before := append([]byte(nil), input...)
	d.MuteStatus = 1
	d.writeColor(input)
	if !reflect.DeepEqual(input, before) {
		t.Fatal("frame alias")
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
	go func() { done <- d.SetLightingEffect("headset") }()
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
func TestExcludedPackageLocalIdentityAudioAndListenerMethods(t *testing.T) {
	source, err := os.ReadFile("virtuosoSEWU.go")
	if err != nil {
		t.Fatal(err)
	}
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "virtuosoSEWU.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"getSerial": "5bd747eda13589395d1d678de3584329d1eb9590f537d33c528e35cacbe7b942", "backendListener": "e0c0e27a8b1e6f5593ba331b5e98279125f2b5406a839b51bdbf5dfa023025b6", "getListenerData": "cbcd22f96dfbd11b8d53ea5d80be508544a29d150989b3483c7a9888919131a2", "notifyMuteChanged": "242875d72e5be13a327d75bf206b00b32b71ce471eb5d5c4146972248a02879a", "getMuteStatus": "c8e0e27c0dd623b86c211ff3d9283eacd672860d5b934fbaab02126b20204457", "setEqualizer": "47a2c140c8379352e98b980caf99d646348cab5a62c9941e79ad9686ebd59799", "UpdateEqualizer": "895c51b77fd0c46784fb119140a659d73b83e28f1ba3d88a580c44cae23b504d", "saveDeviceProfile": "aeadd91d3718a96639d5ec415202c19db716e16775e9192e904b7ebd97b262dc", "transfer": "218eb355cbd8ff286d4479ea0ed6888b1e78393e8bffb3f3524435407758c8ae"}
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
			t.Fatalf("package-local %s changed", f.Name.Name)
		}
	}
	if found != len(want) {
		t.Fatal("missing methods")
	}
}

func TestGenericSettingsMetadataAndPackageNonLightingState(t *testing.T) {
	d := testLightingDevice(t)
	listener := &hid.Device{}
	d.listener = listener
	d.MuteStatus = 1
	d.DeviceProfile.MuteStatus = 1
	unrelated := *d.DeviceProfile
	restarts := 0
	d.lightingRestart = func() { restarts++ }
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
			t.Fatalf("settings %s", effect)
		}
		after := d.GetRgbProfile(effect)
		if before.MinTemp != after.MinTemp || before.MaxTemp != after.MaxTemp || before.Smoothness != after.Smoothness || before.Version != after.Version || before.Brightness != after.Brightness || before.StartColor.Brightness != after.StartColor.Brightness {
			t.Fatal("renderer metadata changed")
		}
		data, err := os.ReadFile(filepath.Join(pwd, "database", "rgb", d.Serial+".json"))
		var persisted rgb.RGB
		if err != nil || json.Unmarshal(data, &persisted) != nil || !reflect.DeepEqual(persisted.Profiles[effect], *after) {
			t.Fatal("RGB persistence")
		}
	}
	if d.SetLightingBrightness(45) != nil || d.SetLightingZoneColor("headset", "all", "", "", rgb.Color{Blue: 128}) != nil || d.ChangeDeviceBrightness(2) != 1 {
		t.Fatal("lighting mutation")
	}
	if d.listener != listener || d.MuteStatus != 1 || !reflect.DeepEqual(d.DeviceProfile.Equalizers, unrelated.Equalizers) || d.DeviceProfile.MuteIndicator != unrelated.MuteIndicator || d.DeviceProfile.DisableMicIndicator != unrelated.DisableMicIndicator || d.DeviceProfile.SleepMode != unrelated.SleepMode || d.DeviceProfile.Serial != unrelated.Serial {
		t.Fatal("non-Lighting state changed")
	}
	if d.DeviceProfile.MuteStatus != unrelated.MuteStatus {
		t.Fatal("profile mic state changed")
	}
	if _, ok := reflect.TypeOf(DeviceProfile{}).FieldByName("SideTone"); ok {
		t.Fatal("invented SE SideTone field")
	}
	if restarts != 2 {
		t.Fatalf("inactive settings restarted: %d", restarts)
	}
	before, _ := json.Marshal(d.DeviceProfile)
	zones := d.GetZoneColors().(map[int]ZoneColors)
	zones[1].Color.Red = 10
	zones[1].ColorIndex[0] = 77
	after, _ := json.Marshal(d.DeviceProfile)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("presentation aliases profile")
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
