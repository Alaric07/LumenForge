package k65rgbRF

import (
	"LumenForge/src/config"
	"LumenForge/src/keyboards"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sstallion/go-hid"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func newLightingTestDevice(t *testing.T) *Device {
	t.Helper()
	logger.Init()
	previous := pwd
	pwd = t.TempDir()
	t.Cleanup(func() { pwd = previous })
	for _, dir := range []string{"profiles", "rgb"} {
		if err := os.MkdirAll(filepath.Join(pwd, "database", dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile("../../../database/rgb.json")
	if err != nil {
		t.Fatal(err)
	}
	var backing rgb.RGB
	if err = json.Unmarshal(data, &backing); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile("../../../database/keyboard/k65rgbRF.json")
	if err != nil {
		t.Fatal(err)
	}
	backing.Profiles["off"] = rgb.Profile{}
	var keyboard keyboards.Keyboard
	if err = json.Unmarshal(data, &keyboard); err != nil {
		t.Fatal(err)
	}
	p := &DeviceProfile{Active: true, Serial: "K65TEST", RGBProfile: "keyboard", BrightnessSlider: 70, OriginalBrightness: 91, Path: filepath.Join(pwd, "database", "profiles", "K65TEST.json"), Profile: "default", Profiles: []string{"default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"default": &keyboard}, PollingRate: 1, DisableAltTab: true}
	d := &Device{Serial: p.Serial, Product: "K65 RGB RAPIDFIRE", LEDChannels: 168, dev: &hid.Device{}, Rgb: &backing, DeviceProfile: p, UserProfiles: map[string]*DeviceProfile{"default": p}, lightingRestart: func() {}}
	if err = d.attachLightingRuntime(config.Paths{ShippedDatabaseRoot: "../../../database"}); err != nil {
		t.Fatal(err)
	}
	if err = d.persistLightingProfile(*d.DeviceProfile); err != nil {
		t.Fatal(err)
	}
	if err = saveLightingJSON(filepath.Join(pwd, "database", "rgb", d.Serial+".json"), d.Rgb); err != nil {
		t.Fatal(err)
	}
	return d
}

var verifiedLightingCatalogue = []string{"circle", "circleshift", "colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "keyboard", "off", "rainbow", "pastelrainbow", "rotator", "spinner", "static", "storm", "watercolor", "wave"}

func TestLightingCatalogueKeyboardTopologyAndSnapshot(t *testing.T) {
	d := newLightingTestDevice(t)
	s, ok := d.LightingSnapshot()
	if !ok || s.ConfiguredEffect != "keyboard" || s.Brightness != 70 || !s.EffectSupported || !s.EffectSelectionAvailable {
		t.Fatalf("snapshot %#v %v", s, ok)
	}
	got := []string{}
	for _, e := range s.SupportedEffects {
		if e.ID == "keyboard" && e.Label != "Keyboard" {
			t.Fatal("keyboard label", e.Label)
		}
		got = append(got, e.ID)
	}
	if len(got) != 23 || !reflect.DeepEqual(got, verifiedLightingCatalogue) {
		t.Fatal(got)
	}
	for _, id := range []string{"pastelspiralrainbow", "custom", "mouse", "liquid-temperature"} {
		if d.SupportsLightingEffect(id) || d.SetLightingEffect(id) == nil {
			t.Fatal(id)
		}
	}
	if s.ClusterControlled || s.ExternalControlled || s.AuthoredZoneEditor == nil || len(s.AuthoredZoneEditor.Zones) != 92 || !s.AuthoredZoneEditor.HasGroups {
		t.Fatal("capability")
	}
	if _, ok := interface{}(d).(interface{ SetLiveRGB(bool) error }); ok {
		t.Fatal("invented Live RGB")
	}
	if d.LEDChannels != 168 || colorPacketLength != 168 || maxBufferSizePerRequest != 60 || bufferSizeWrite != 65 {
		t.Fatal("transport dimensions")
	}
	for _, layout := range []string{"US", "DE"} {
		p, _ := cloneLightingProfile(d.DeviceProfile)
		p.Layout = layout
		p.Keyboards["default"] = d.lightingLayouts[layout]
		probe := &Device{DeviceProfile: p, lightingLayouts: d.lightingLayouts}
		if probe.validateLightingKeyboard() != nil {
			t.Fatal(layout)
		}
		editor := probe.keyboardZoneEditor()
		want := 92
		if layout == "DE" {
			want = 93
		}
		if len(editor.Zones) != want {
			t.Fatal(layout, len(editor.Zones))
		}
		mapped := map[int]bool{}
		for _, row := range p.Keyboards["default"].Row {
			for _, key := range row.Keys {
				for _, index := range key.PacketIndex {
					if index < 0 || index >= 168 || mapped[index] {
						t.Fatal("mapping", index)
					}
					mapped[index] = true
				}
			}
		}
		if len(mapped) != want || !mapped[8] || !mapped[139] || mapped[167] {
			t.Fatal("sparse topology")
		}
		z := editor.Zones[0]
		if z.ID != "1" || z.GroupID != "1" || z.Left != 1200 || z.Top != 0 || z.Width != 100 || z.Height != 100 {
			t.Fatalf("geometry %#v", z)
		}
	}
	// Fixture snapshots own their presentation arrays.
	original := d.DeviceProfile.Keyboards["default"].Row[1].Keys[1].Color
	s.AuthoredZoneEditor.Zones[0].ColorHex = "#000000"
	if d.DeviceProfile.Keyboards["default"].Row[1].Keys[1].Color != original {
		t.Fatal("alias")
	}
}
func TestLightingCompleteSettingsAndReset(t *testing.T) {
	d := newLightingTestDevice(t)
	count := 0
	d.lightingRestart = func() { count++ }
	for _, effect := range verifiedLightingCatalogue {
		if effect == "keyboard" {
			continue
		}
		value, err := d.ResolveLightingEffectSettings(effect)
		if err != nil {
			t.Fatal(effect, err)
		}
		if lightingsettings.Validate(value) != nil {
			t.Fatal(effect)
		}
		if err = d.SetLightingEffectSettings(effect, value); err != nil {
			t.Fatal(effect, err)
		}
		if err = d.ResetLightingEffectSettings(effect); err != nil {
			t.Fatal(effect, err)
		}
	}
	if count != 0 {
		t.Fatal("inactive effect restart")
	}
	if d.SetLightingEffect("static") != nil {
		t.Fatal("selection")
	}
	value, _ := d.ResolveLightingEffectSettings("static")
	value.SingleColor.Color = lightingsettings.Color{Red: 11, Green: 22, Blue: 33}
	if d.SetLightingEffectSettings("static", value) != nil {
		t.Fatal("settings")
	}
	value.SingleColor.Color.Red = 99
	s, ok := d.LightingSnapshot()
	if !ok || s.SingleColorHex != "#0b1621" || !s.Customized {
		t.Fatal(s)
	}
	if d.ResetLightingEffectSettings("static") != nil {
		t.Fatal("reset")
	}
	s, _ = d.LightingSnapshot()
	if s.Customized {
		t.Fatal("reset customization")
	}
	invalid, _ := d.ResolveLightingEffectSettings("static")
	invalid.SingleColor = nil
	if d.SetLightingEffectSettings("static", invalid) == nil {
		t.Fatal("sparse settings")
	}
	g, _ := d.ResolveLightingEffectSettings("gradient")
	if d.SetLightingEffectSettings("gradient", g) != nil {
		t.Fatal("gradient")
	}
	g.Gradient.Stops[0].Color.Red = 234
	resolved, _ := d.ResolveLightingEffectSettings("gradient")
	if reflect.DeepEqual(g, resolved) {
		t.Fatal("gradient alias")
	}
}
func TestLightingPersistenceRollbackAndAuthoredCopy(t *testing.T) {
	d := newLightingTestDevice(t)
	count := 0
	d.lightingRestart = func() { count++ }
	prior, _ := cloneLightingProfile(d.DeviceProfile)
	blocker := filepath.Join(pwd, "database", "profiles", "blocked.json")
	if os.Mkdir(blocker, 0700) != nil {
		t.Fatal("blocker")
	}
	d.DeviceProfile.Path = blocker
	prior.Path = blocker
	for _, mutate := range []func() error{func() error { return d.SetLightingEffect("wave") }, func() error { return d.SetLightingBrightness(20) }, func() error { return d.SetLightingZoneColors("keyboard", []string{"1"}, rgb.Color{Red: 100}) }} {
		if mutate() == nil || !reflect.DeepEqual(prior, d.DeviceProfile) || count != 0 {
			t.Fatal("failed persistence changed state")
		}
	}
	d.DeviceProfile.Path = filepath.Join(pwd, "database", "profiles", "K65TEST.json")
	alias := d.DeviceProfile.Keyboards["default"]
	if d.SetLightingZoneColors("keyboard", []string{"1", "2"}, rgb.Color{Red: 120, Blue: 40}) != nil {
		t.Fatal("keys")
	}
	if reflect.DeepEqual(alias, d.DeviceProfile.Keyboards["default"]) {
		t.Fatal("key alias")
	}
	var saved DeviceProfile
	data, _ := os.ReadFile(d.DeviceProfile.Path)
	if json.Unmarshal(data, &saved) != nil || saved.Keyboards["default"].Row[1].Keys[1].Color.Red != 120 {
		t.Fatal("authored persistence")
	}
	if d.SetLightingZoneColors("keyboard", []string{"1", "invalid"}, rgb.Color{Green: 250}) == nil || d.DeviceProfile.Keyboards["default"].Row[1].Keys[1].Color.Red != 120 {
		t.Fatal("partial selection")
	}
	value, _ := d.ResolveLightingEffectSettings("static")
	rgbBefore := copyLightingRGBProfile(*d.GetRgbProfile("static"))
	rgbPath := filepath.Join(pwd, "database", "rgb", d.Serial+".json")
	os.Remove(rgbPath)
	os.Mkdir(rgbPath, 0700)
	value.SingleColor.Color.Red = 150
	c := count
	if d.SetLightingEffectSettings("static", value) == nil || !reflect.DeepEqual(rgbBefore, *d.GetRgbProfile("static")) || count != c {
		t.Fatal("RGB rollback")
	}
}
func TestLightingDarknessDesiredMutationsAndRestoration(t *testing.T) {
	d := newLightingTestDevice(t)
	if d.SchedulerBrightness(0) != 1 {
		t.Fatal("scheduler")
	}
	if d.SetLightingBrightness(42) != nil || d.SetLightingZoneColor("keyboard", "group", "", "1", rgb.Color{Blue: 180}) != nil {
		t.Fatal("dark mutation")
	}
	_, brightness, _ := d.lightingOutputState()
	s, _ := d.LightingSnapshot()
	if brightness != 0 || s.Brightness != 42 || s.AuthoredZoneEditor.Zones[0].ColorHex != "#0000b4" {
		t.Fatal("desired")
	}
	d.ControlDeviceRgb(true)
	if d.SchedulerBrightness(100) != 1 {
		t.Fatal("clear scheduler")
	}
	_, brightness, off := d.lightingOutputState()
	if brightness != 0 || !off {
		t.Fatal("off")
	}
	if d.SetLightingEffect("wave") != nil || d.SetLightingBrightness(55) != nil {
		t.Fatal("off mutation")
	}
	d.ControlDeviceRgb(false)
	effect, brightness, off := d.lightingOutputState()
	if effect != "wave" || brightness != 55 || off {
		t.Fatal("restore")
	}
	data, _ := os.ReadFile(d.DeviceProfile.Path)
	var saved DeviceProfile
	json.Unmarshal(data, &saved)
	if saved.BrightnessSlider != 55 || saved.RGBProfile != "wave" || saved.RgbOff || saved.OriginalBrightness != 91 {
		t.Fatal("persisted darkness")
	}
}
func TestLightingGuardsAndAttachmentFallback(t *testing.T) {
	for _, name := range []string{"hid", "exit", "attachment", "rgb", "profile", "brightness", "mapping", "gradient"} {
		t.Run(name, func(t *testing.T) {
			d := newLightingTestDevice(t)
			count := 0
			d.lightingRestart = func() { count++ }
			switch name {
			case "hid":
				d.dev = nil
			case "exit":
				d.Exit = true
			case "attachment":
				d.lightingDefaults = nil
			case "rgb":
				d.Rgb = nil
			case "profile":
				d.DeviceProfile = nil
			case "brightness":
				d.DeviceProfile.BrightnessSlider = 101
			case "mapping":
				row := d.DeviceProfile.Keyboards["default"].Row[1]
				key := row.Keys[1]
				key.PacketIndex = []int{168}
				row.Keys[1] = key
			case "gradient":
				delete(d.Rgb.Profiles, "gradient")
			}
			if _, ok := d.LightingSnapshot(); ok {
				t.Fatal("available")
			}
			if d.SetLightingEffect("static") == nil || d.SetLightingBrightness(30) == nil || d.SetLightingZoneColors("keyboard", []string{"1"}, rgb.Color{}) == nil || count != 0 {
				t.Fatal("guard")
			}
		})
	}
	d := newLightingTestDevice(t)
	d.lightingDefaults = nil
	if d.attachLightingRuntime(config.Paths{ShippedDatabaseRoot: t.TempDir()}) == nil || d.lightingDefaults != nil {
		t.Fatal("attachment failure")
	}
}
func TestLightingProfilesAndLegacyUpgrade(t *testing.T) {
	d := newLightingTestDevice(t)
	if d.SaveDeviceProfile("other", true) != 1 || d.UpdateKeyboardProfile("other") != 1 {
		t.Fatal("keyboard profiles")
	}
	if d.SetLightingZoneColors("keyboard", []string{"1"}, rgb.Color{Red: 10}) != nil {
		t.Fatal("key")
	}
	if d.DeviceProfile.Keyboards["default"].Row[1].Keys[1].Color.Red == 10 {
		t.Fatal("saved keyboard aliases")
	}
	if d.DeleteKeyboardProfile("other") != 1 || d.DeviceProfile.Profile != "default" {
		t.Fatal("delete")
	}
	if d.SaveUserProfile("saved") != 1 || !d.DeviceProfile.Active || d.DeviceProfile.Path == d.UserProfiles["saved"].Path {
		t.Fatal("user profile alias")
	}
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(true)
	if d.switchLightingProfile("saved") != nil {
		t.Fatal("switch")
	}
	_, b, off := d.lightingOutputState()
	if b != 70 || off || d.schedulerDark || !d.DeviceProfile.DisableAltTab || d.DeviceProfile.PollingRate != 1 {
		t.Fatal("profile behavior")
	}
	if !reflect.DeepEqual(rgbProfileUpgrade, []string{"gradient", "pastelrainbow", "pastelspiralrainbow", "flame", "aurora", "cyberpunkglitch", "tokyonight"}) {
		t.Fatal("upgrade list")
	}
	delete(d.Rgb.Profiles, "pastelspiralrainbow")
	d.upgradeRgbProfile(filepath.Join(pwd, "database", "rgb", d.Serial+".json"), rgbProfileUpgrade)
	if d.GetRgbProfile("pastelspiralrainbow") == nil || d.SupportsLightingEffect("pastelspiralrainbow") {
		t.Fatal("upgrade catalogue distinction")
	}
}
func TestLightingPacketPlanesAndAuthoredOutput(t *testing.T) {
	d := newLightingTestDevice(t)
	var planes [3][]byte
	plane := 0
	headers := [][]byte{}
	d.lightingTransfer = func(command byte, endpoint, buffer []byte) error {
		if command == 0x7f {
			if len(endpoint) != 3 || endpoint[0] != byte(len(planes[plane])/60+1) || int(endpoint[1]) != len(buffer) || endpoint[2] != 0 {
				t.Fatal("chunk header")
			}
			planes[plane] = append(planes[plane], buffer...)
		} else if command == 0x07 {
			headers = append(headers, append([]byte(nil), endpoint...))
			plane++
		} else {
			t.Fatal(command)
		}
		return nil
	}
	if d.SetLightingZoneColor("keyboard", "all", "", "", rgb.Color{Red: 100, Green: 50, Blue: 20}) != nil {
		t.Fatal("colors")
	}
	before, _ := cloneLightingProfile(d.DeviceProfile)
	d.setDeviceColor()
	if !reflect.DeepEqual(headers, [][]byte{{0x28, 1, 3, 1}, {0x28, 2, 3, 1}, {0x28, 3, 3, 2}}) {
		t.Fatal(headers)
	}
	for i, p := range planes {
		if len(p) != 168 {
			t.Fatal("plane length")
		}
		want := []byte{70, 35, 14}[i]
		for _, row := range d.DeviceProfile.Keyboards["default"].Row {
			for _, key := range row.Keys {
				if p[key.PacketIndex[0]] != want {
					t.Fatal("channel", i, key.PacketIndex, p[key.PacketIndex[0]], want)
				}
			}
		}
		if p[167] != 0 {
			t.Fatal("hole")
		}
	}
	if !reflect.DeepEqual(before, d.DeviceProfile) {
		t.Fatal("render changed authored state")
	}
}
func TestLightingRendererCaptureStopAndConcurrentMutation(t *testing.T) {
	d := newLightingTestDevice(t)
	d.lightingRestart = nil
	d.lightingTransfer = func(byte, []byte, []byte) error { return nil }
	d.DeviceProfile.RGBProfile = "wave"
	d.setDeviceColor()
	start := time.Now()
	d.stopLightingRenderer()
	if time.Since(start) > time.Second {
		t.Fatal("unbounded stop")
	}
	d.rendererMu.Lock()
	active := d.activeRgb
	d.rendererMu.Unlock()
	if active != nil {
		t.Fatal("stale renderer")
	}
	// Temporary missing state terminates and retires the captured renderer.
	d.setDeviceColor()
	d.lightingMu.Lock()
	d.DeviceProfile = nil
	d.lightingMu.Unlock()
	time.Sleep(50 * time.Millisecond)
	d.rendererMu.Lock()
	stale := d.activeRgb
	d.rendererMu.Unlock()
	if stale != nil {
		t.Fatal("dead renderer remained active")
	}
	d.stopLightingRenderer()
	d = newLightingTestDevice(t)
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 4; j++ {
				switch n {
				case 0:
					d.SetLightingBrightness(uint8(40 + j))
				case 1:
					d.LightingSnapshot()
				case 2:
					d.SetLightingZoneColors("keyboard", []string{"1"}, rgb.Color{Blue: float64(j)})
				}
			}
		}(i)
	}
	wg.Wait()
}

var _ interface {
	LightingSnapshot() (lightingpresentation.Snapshot, bool)
} = (*Device)(nil)

func TestLightingExactKeyPacketMapping(t *testing.T) {
	d := newLightingTestDevice(t)
	hashes := map[string]string{"US": "bb3ee4b1abab27c430f37b36260ad19bb92979c11ab041d28797155560db0b2f", "DE": "b7d8290e7ecd532e9203ae78da5daa87cca5f44c9fbd33684173ace97149fbbf"}
	for layout, want := range hashes {
		k := d.lightingLayouts[layout]
		rowIDs := []int{}
		for id := range k.Row {
			rowIDs = append(rowIDs, id)
		}
		sort.Ints(rowIDs)
		var mapping strings.Builder
		for _, rowID := range rowIDs {
			row := k.Row[rowID]
			for _, id := range sortedLightingKeys(row) {
				fmt.Fprintf(&mapping, "%d/%d:%v;", rowID, id, row.Keys[id].PacketIndex)
			}
		}
		if got := fmt.Sprintf("%x", sha256.Sum256([]byte(mapping.String()))); got != want {
			t.Fatal(layout, got)
		}
	}
}
func TestLightingPersistsBeforePublicationAndRestart(t *testing.T) {
	d := newLightingTestDevice(t)
	count := 0
	d.lightingRestart = func() { count++ }
	d.lightingPersist = func(path string, value interface{}) error {
		if d.DeviceProfile.BrightnessSlider != 70 || count != 0 {
			t.Fatal("published before write")
		}
		return saveLightingJSON(path, value)
	}
	if d.SetLightingBrightness(38) != nil || d.DeviceProfile.BrightnessSlider != 38 || count != 1 {
		t.Fatal("publication ordering")
	}
	d.lightingPersist = func(string, interface{}) error { return errors.New("write failed") }
	prior, _ := cloneLightingProfile(d.DeviceProfile)
	c := count
	if d.SetLightingZoneColors("keyboard", []string{"1"}, rgb.Color{Red: 10}) == nil || !reflect.DeepEqual(prior, d.DeviceProfile) || count != c {
		t.Fatal("write failure")
	}
	if d.SaveUserProfile("saved") != 1 { // existing user-profile save is independent of canonical mutation hook
		t.Fatal("user profile")
	}
	target := d.UserProfiles["saved"]
	d.lightingPersist = func(path string, value interface{}) error {
		if path == target.Path {
			return errors.New("target write failed")
		}
		return saveLightingJSON(path, value)
	}
	if d.switchLightingProfile("saved") == nil || !d.DeviceProfile.Active || target.Active || d.DeviceProfile == target || count != c {
		t.Fatal("profile publication on failure")
	}
}

func TestLightingLayoutPreservesSavedKeyboardAndProfileFields(t *testing.T) {
	d := newLightingTestDevice(t)
	d.lightingTransfer = func(byte, []byte, []byte) error { return nil }
	if d.SaveDeviceProfile("saved-keyboard", true) != 1 || d.UpdateKeyboardProfile("saved-keyboard") != 1 {
		t.Fatal("keyboard save")
	}
	count := 0
	d.lightingRestart = func() { count++ }
	if d.ChangeKeyboardLayout("DE") != 1 {
		t.Fatal("layout")
	}
	if count != 1 || d.DeviceProfile.Layout != "DE" || d.DeviceProfile.Keyboards["default"].Layout != "DE" || d.DeviceProfile.Keyboards["saved-keyboard"].Layout != "US" || !d.DeviceProfile.DisableAltTab {
		t.Fatal("layout behavior")
	}
	s, ok := d.LightingSnapshot()
	if !ok || len(s.AuthoredZoneEditor.Zones) != 92 {
		t.Fatal("saved US topology")
	}
	if d.UpdateKeyboardProfile("default") != 1 {
		t.Fatal("default selection")
	}
	s, ok = d.LightingSnapshot()
	if !ok || len(s.AuthoredZoneEditor.Zones) != 93 {
		t.Fatal("DE topology")
	}
}
func TestLightingBrightnessAndSettingsRestartOnlyWhenRequired(t *testing.T) {
	d := newLightingTestDevice(t)
	count := 0
	d.lightingRestart = func() { count++ }
	if d.SetLightingBrightness(40) != nil || count != 1 {
		t.Fatal("keyboard brightness restart")
	}
	if d.SetLightingEffect("wave") != nil {
		t.Fatal("wave")
	}
	c := count
	if d.SetLightingBrightness(30) != nil || count != c {
		t.Fatal("dynamic brightness restart")
	}
	value, _ := d.ResolveLightingEffectSettings("wave")
	if d.SetLightingEffectSettings("wave", value) != nil || count != c+1 {
		t.Fatal("selected settings restart")
	}
	value, _ = d.ResolveLightingEffectSettings("static")
	c = count
	if d.SetLightingEffectSettings("static", value) != nil || count != c {
		t.Fatal("inactive settings restart")
	}
}
func TestLightingPerformanceOverlayPreservesCallerPlanes(t *testing.T) {
	d := newLightingTestDevice(t)
	d.DeviceProfile.Performance = true
	plane := 0
	frames := [3][]byte{}
	d.lightingTransfer = func(command byte, endpoint, buffer []byte) error {
		if command == 0x7f {
			frames[plane] = append(frames[plane], buffer...)
		} else if command == 0x07 {
			plane++
		}
		return nil
	}
	red, green, blue := make([]byte, 168), make([]byte, 168), make([]byte, 168)
	red[8] = 10
	green[8] = 20
	blue[8] = 30
	d.writeColor(red, green, blue)
	if red[8] != 10 || green[8] != 20 || blue[8] != 30 || frames[0][8] != 255 || frames[1][8] != 0 || frames[2][8] != 0 {
		t.Fatal("performance overlay/copy")
	}
}

// Exercise the real input handler, including canonical persistence and restart.
func TestHardwareBrightnessKeyCanonicalClamp(t *testing.T) {
	for _, test := range []struct{ from, to uint8 }{{0, 33}, {33, 66}, {66, 99}, {68, 100}, {85, 100}, {98, 100}, {99, 0}, {100, 0}} {
		t.Run(fmt.Sprintf("%d-to-%d", test.from, test.to), func(t *testing.T) {
			d := newLightingTestDevice(t)
			d.DeviceProfile.BrightnessSlider = test.from
			row := d.DeviceProfile.Keyboards["default"].Row[1]
			key := row.Keys[1]
			key.KeyHash = []string{"1461501637330902918203684832716283019655932542975"}
			key.ActionType = 11
			row.Keys[1] = key
			writes, restarts := 0, 0
			d.lightingPersist = func(path string, value interface{}) error {
				if d.DeviceProfile.BrightnessSlider != test.from || restarts != 0 {
					t.Fatal("published before persistence")
				}
				proposal := value.(*DeviceProfile)
				if proposal.BrightnessSlider != test.to {
					t.Fatal("proposed brightness", proposal.BrightnessSlider)
				}
				writes++
				return saveLightingJSON(path, value)
			}
			d.lightingRestart = func() {
				restarts++
				data, err := os.ReadFile(d.DeviceProfile.Path)
				if err != nil {
					t.Fatal(err)
				}
				var saved DeviceProfile
				if json.Unmarshal(data, &saved) != nil || saved.BrightnessSlider != test.to || d.DeviceProfile.BrightnessSlider != test.to {
					t.Fatal("restart preceded publication/persistence")
				}
			}
			input := make([]byte, 21)
			input[0] = 3
			for i := 1; i < len(input); i++ {
				input[i] = 255
			} // Unique synthetic hash: all 160 input bits.
			d.triggerKeyAssignment(input)
			if writes != 1 || restarts != 1 || d.DeviceProfile.BrightnessSlider != test.to {
				t.Fatal("handler path", writes, restarts, d.DeviceProfile.BrightnessSlider)
			}
		})
	}
}
func TestHardwareBrightnessKeyPersistenceFailureRetainsDesiredState(t *testing.T) {
	d := newLightingTestDevice(t)
	d.DeviceProfile.BrightnessSlider = 85
	row := d.DeviceProfile.Keyboards["default"].Row[1]
	key := row.Keys[1]
	key.KeyHash = []string{"1461501637330902918203684832716283019655932542975"}
	key.ActionType = 11
	row.Keys[1] = key
	restarts := 0
	d.lightingRestart = func() { restarts++ }
	d.lightingPersist = func(string, interface{}) error { return errors.New("write failure") }
	input := make([]byte, 21)
	input[0] = 3
	for i := 1; i < len(input); i++ {
		input[i] = 255
	}
	d.triggerKeyAssignment(input)
	if d.DeviceProfile.BrightnessSlider != 85 || restarts != 0 {
		t.Fatal("failed persistence changed brightness/output")
	}
}
func newFailedAttachmentLegacyTestDevice(t *testing.T) *Device {
	t.Helper()
	d := newLightingTestDevice(t)
	d.lightingDefaults = nil
	d.lightingLayouts = nil
	// This stored keyboard remains renderable by its packet map, but canonical
	// eligibility rejects the old metadata and authored color.
	keyboard := d.DeviceProfile.Keyboards["default"]
	keyboard.Rows = 99
	row := keyboard.Row[1]
	key := row.Keys[1]
	key.Width = 1
	key.Color.Red = -1
	row.Keys[1] = key
	before, _ := cloneLightingProfile(d.DeviceProfile)
	outputs := 0
	d.lightingTransfer = func(byte, []byte, []byte) error { outputs++; return nil }
	if d.attachLightingRuntime(config.Paths{ShippedDatabaseRoot: "../../../database"}) == nil {
		t.Fatal("canonical attachment accepted incompatible legacy metadata")
	}
	if d.lightingDefaults != nil || d.lightingLayouts != nil || outputs != 0 || !reflect.DeepEqual(before, d.DeviceProfile) {
		t.Fatal("partial attachment or attachment side effect")
	}
	if _, ok := d.LightingSnapshot(); ok {
		t.Fatal("canonical snapshot prevents LegacyLighting eligibility")
	}
	return d
}
func TestFailedCanonicalAttachmentLeavesLegacyRenderingFunctional(t *testing.T) {
	d := newFailedAttachmentLegacyTestDevice(t)
	plane := 0
	frames := [3][]byte{}
	d.lightingTransfer = func(command byte, endpoint, buffer []byte) error {
		if command == cmdWriteColor {
			frames[plane] = append(frames[plane], buffer...)
		} else if command == cmdWrite {
			plane++
		}
		return nil
	}
	state, err := d.lightingFrameState()
	if err != nil || state.Keyboards[state.Profile].Rows != 99 {
		t.Fatal("strict canonical validation blocked fallback", err)
	}
	d.setDeviceColor()
	if plane != 3 {
		t.Fatal("legacy output absent", plane)
	}
	for _, frame := range frames {
		if len(frame) != 168 || frame[167] != 0 {
			t.Fatal("legacy frame layout")
		}
	}
	if frames[1][137] == 0 || frames[2][137] == 0 {
		t.Fatal("authored keyboard output absent")
	}
	// Missing or unsafe packet backing still fails safely in legacy mode.
	d.DeviceProfile.Keyboards["default"] = nil
	if _, err = d.lightingFrameState(); err == nil {
		t.Fatal("missing legacy keyboard")
	}
	d.DeviceProfile = state
	row := state.Keyboards[state.Profile].Row[1]
	key := row.Keys[1]
	key.PacketIndex = []int{168}
	row.Keys[1] = key
	if _, err = d.lightingFrameState(); err == nil {
		t.Fatal("unsafe legacy packet map")
	}
}
func TestCanonicalAttachmentPublicationIsTransactional(t *testing.T) {
	d := newLightingTestDevice(t)
	d.lightingDefaults = nil
	d.lightingLayouts = nil
	before, _ := cloneLightingProfile(d.DeviceProfile)
	delete(d.Rgb.Profiles, "gradient")
	if d.attachLightingRuntime(config.Paths{ShippedDatabaseRoot: "../../../database"}) == nil || d.lightingLayouts != nil || d.lightingDefaults != nil || !reflect.DeepEqual(before, d.DeviceProfile) {
		t.Fatal("late attachment failure published candidate state")
	}
	d.Rgb.Profiles["gradient"] = *rgbProfileForAttachmentTest(t, "gradient")
	if d.attachLightingRuntime(config.Paths{ShippedDatabaseRoot: "../../../database"}) != nil || len(d.lightingLayouts) != 2 || d.lightingDefaults == nil {
		t.Fatal("successful attachment")
	}
	row := d.DeviceProfile.Keyboards["default"].Row[1]
	key := row.Keys[1]
	key.Width = 1
	row.Keys[1] = key
	if _, err := d.lightingFrameState(); err == nil {
		t.Fatal("attached renderer bypassed strict topology validation")
	}
	if d.SetLightingBrightness(50) == nil {
		t.Fatal("attached authored mutation bypassed strict validation")
	}
}
func rgbProfileForAttachmentTest(t *testing.T, effect string) *rgb.Profile {
	t.Helper()
	data, err := os.ReadFile("../../../database/rgb.json")
	if err != nil {
		t.Fatal(err)
	}
	var backing rgb.RGB
	if json.Unmarshal(data, &backing) != nil {
		t.Fatal("RGB fixture")
	}
	value := backing.Profiles[effect]
	return &value
}
func TestLegacyRGBMutationAfterAttachmentFailureDoesNotDeadlock(t *testing.T) {
	d := newFailedAttachmentLegacyTestDevice(t)
	var outputMu sync.Mutex
	outputs := 0
	d.lightingTransfer = func(byte, []byte, []byte) error { outputMu.Lock(); outputs++; outputMu.Unlock(); return nil }
	bounded := func(work func()) {
		done := make(chan struct{})
		go func() { work(); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("legacy RGB access deadlocked")
		}
	}
	var status uint8
	profile := *d.GetRgbProfile("static")
	profile.StartColor.Red = 123
	bounded(func() { status = d.UpdateRgbProfileData("static", profile) })
	if status != 1 {
		t.Fatal("legacy mutation rejected")
	}
	bounded(func() {
		if value := d.GetRgbProfile("static"); value == nil || value.StartColor.Red != 123 {
			t.Error("legacy getter")
		}
	})
	// Reads overlap further writes to exercise retained read/write protection.
	bounded(func() {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				d.GetRgbProfile("static")
			}
		}()
		profile.StartColor.Red = 42
		status = d.UpdateRgbProfileData("static", profile)
		wg.Wait()
	})
	if status != 1 {
		t.Fatal("subsequent legacy mutation")
	}
	bounded(func() { d.setDeviceColor() })
	outputMu.Lock()
	count := outputs
	outputMu.Unlock()
	if count == 0 {
		t.Fatal("fallback output remained blocked")
	}
	// Renderer reads also remain usable after the mutation/restart boundary.
	d.DeviceProfile.RGBProfile = "wave"
	bounded(func() { d.setDeviceColor() })
	bounded(func() { profile.Speed = 2; status = d.UpdateRgbProfileData("static", profile) })
	bounded(func() { d.stopLightingRenderer() })
	if status != 1 {
		t.Fatal("legacy mutation while renderer active")
	}
}

func TestSaveDeviceProfileCanonicalDuplicateStatus(t *testing.T) {
	for _, collection := range []string{"Profiles", "Keyboards", "nil keyboard"} {
		t.Run(collection, func(t *testing.T) {
			d := newLightingTestDevice(t)
			const name = "existing"
			switch collection {
			case "Profiles":
				d.DeviceProfile.Profiles = append(d.DeviceProfile.Profiles, name)
			case "Keyboards":
				d.DeviceProfile.Keyboards[name] = d.DeviceProfile.Keyboards[d.DeviceProfile.Profile]
			case "nil keyboard":
				d.DeviceProfile.Keyboards[name] = nil
			}
			before, err := cloneLightingProfile(d.DeviceProfile)
			if err != nil {
				t.Fatal(err)
			}
			diskBefore, err := os.ReadFile(d.DeviceProfile.Path)
			if err != nil {
				t.Fatal(err)
			}
			original := d.DeviceProfile
			writes, restarts := 0, 0
			d.lightingPersist = func(string, interface{}) error { writes++; return nil }
			d.lightingRestart = func() { restarts++ }
			if status := d.SaveDeviceProfile(name, true); status != 2 {
				t.Fatal("duplicate status", status)
			}
			diskAfter, err := os.ReadFile(d.DeviceProfile.Path)
			if err != nil {
				t.Fatal(err)
			}
			if writes != 0 || restarts != 0 || d.DeviceProfile != original || !reflect.DeepEqual(d.DeviceProfile, before) || string(diskBefore) != string(diskAfter) {
				t.Fatal("duplicate altered persistence, state, or renderer", writes, restarts)
			}
		})
	}
}

func TestSaveDeviceProfileCanonicalNewAndUpdatePersistence(t *testing.T) {
	for _, create := range []bool{true, false} {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("new=%v/fail=%v", create, fail), func(t *testing.T) {
				d := newLightingTestDevice(t)
				name := "unique"
				if !create {
					name = "default"
				} // Existing names remain valid for updates.
				before, err := cloneLightingProfile(d.DeviceProfile)
				if err != nil {
					t.Fatal(err)
				}
				original := d.DeviceProfile
				writes, restarts := 0, 0
				var proposed *DeviceProfile
				d.lightingRestart = func() { restarts++ }
				d.lightingPersist = func(path string, value interface{}) error {
					writes++
					if d.DeviceProfile != original || !reflect.DeepEqual(d.DeviceProfile, before) || restarts != 0 {
						t.Fatal("published before persistence")
					}
					proposed = value.(*DeviceProfile)
					expected, err := cloneLightingProfile(before)
					if err != nil {
						t.Fatal(err)
					}
					if create {
						expected.Profiles = append(expected.Profiles, name)
						expected.Keyboards[name] = expected.Keyboards[expected.Profile]
					}
					if !reflect.DeepEqual(proposed, expected) {
						t.Fatal("unexpected profile mutation")
					}
					if fail {
						return errors.New("write failed")
					}
					return saveLightingJSON(path, value)
				}
				status := d.SaveDeviceProfile(name, create)
				if writes != 1 || restarts != 0 {
					t.Fatal("mutation path", writes, restarts)
				}
				if fail {
					if status != 0 || d.DeviceProfile != original || !reflect.DeepEqual(d.DeviceProfile, before) {
						t.Fatal("failed persistence published state", status)
					}
					return
				}
				if status != 1 || !reflect.DeepEqual(d.DeviceProfile, proposed) {
					t.Fatal("successful publication", status)
				}
				data, err := os.ReadFile(d.DeviceProfile.Path)
				if err != nil {
					t.Fatal(err)
				}
				var saved DeviceProfile
				if err = json.Unmarshal(data, &saved); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(&saved, d.DeviceProfile) {
					t.Fatal("published state differs from persisted state")
				}
			})
		}
	}
}

func TestGradientMutationMissingProfile(t *testing.T) {
	for _, canonical := range []bool{true, false} {
		for _, operation := range []string{"add", "delete"} {
			t.Run(fmt.Sprintf("canonical=%v/%s", canonical, operation), func(t *testing.T) {
				d := newLightingTestDevice(t)
				if !canonical {
					d.lightingDefaults = nil
				}
				delete(d.Rgb.Profiles, "gradient")
				before, err := json.Marshal(d.Rgb)
				if err != nil {
					t.Fatal(err)
				}
				profileBefore, err := cloneLightingProfile(d.DeviceProfile)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(pwd, "database", "rgb", d.Serial+".json")
				diskBefore, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				writes, restarts, outputs := 0, 0, 0
				d.lightingPersist = func(string, interface{}) error { writes++; return nil }
				d.lightingRestart = func() { restarts++ }
				d.lightingTransfer = func(byte, []byte, []byte) error { outputs++; return nil }
				var status uint8
				var index uint
				if operation == "add" {
					status, index = d.ProcessNewGradientColor("gradient")
				} else {
					status, index = d.ProcessDeleteGradientColor("gradient")
				}
				after, err := json.Marshal(d.Rgb)
				if err != nil {
					t.Fatal(err)
				}
				diskAfter, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if status != 0 || index != 0 || writes != 0 || restarts != 0 || outputs != 0 || string(before) != string(after) || string(diskBefore) != string(diskAfter) || !reflect.DeepEqual(profileBefore, d.DeviceProfile) {
					t.Fatal("missing profile changed state/output", status, index, writes, restarts, outputs)
				}
			})
		}
	}
}

func TestGradientMutationValidAddDelete(t *testing.T) {
	for _, canonical := range []bool{true, false} {
		t.Run(fmt.Sprintf("canonical=%v", canonical), func(t *testing.T) {
			d := newLightingTestDevice(t)
			if !canonical {
				d.lightingDefaults = nil
			}
			d.lightingTransfer = func(byte, []byte, []byte) error { return nil }
			writes, restarts := 0, 0
			d.lightingPersist = func(path string, value interface{}) error { writes++; return saveLightingJSON(path, value) }
			d.lightingRestart = func() { restarts++ }
			before := d.GetRgbProfile("gradient")
			next := 0
			for id := range before.Gradients {
				if id >= next {
					next = id + 1
				}
			}
			status, index := d.ProcessNewGradientColor("gradient")
			added := d.GetRgbProfile("gradient")
			if status != 1 || index != uint(next) || len(added.Gradients) != len(before.Gradients)+1 || added.Gradients[next] != (rgb.Color{Green: 255, Blue: 255}) {
				t.Fatal("add", status, index)
			}
			status, index = d.ProcessDeleteGradientColor("gradient")
			if status != 1 || index != uint(next) || !reflect.DeepEqual(d.GetRgbProfile("gradient"), before) {
				t.Fatal("delete", status, index)
			}
			if canonical && (writes != 2 || restarts != 2) {
				t.Fatal("canonical mutation path", writes, restarts)
			}
			data, err := os.ReadFile(filepath.Join(pwd, "database", "rgb", d.Serial+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var saved rgb.RGB
			if err = json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(saved.Profiles["gradient"], *before) {
				t.Fatal("saved gradients")
			}
		})
	}
}

func TestLightingRendererRetriesMissingRGBProfile(t *testing.T) {
	d := newLightingTestDevice(t)
	d.lightingRestart = nil
	d.DeviceProfile.RGBProfile = "wave"
	frames := make(chan struct{}, 1)
	var outputMu sync.Mutex
	outputs := 0
	d.lightingTransfer = func(command byte, endpoint, buffer []byte) error {
		if command == cmdWrite && len(endpoint) == 4 && endpoint[1] == 3 {
			outputMu.Lock()
			outputs++
			outputMu.Unlock()
			select {
			case frames <- struct{}{}:
			default:
			}
		}
		return nil
	}
	waitFrame := func() {
		t.Helper()
		select {
		case <-frames:
		case <-time.After(time.Second):
			t.Fatal("renderer did not produce a frame")
		}
	}
	count := func() int { outputMu.Lock(); defer outputMu.Unlock(); return outputs }
	stop := func(runner *lightingRenderer) {
		t.Helper()
		done := make(chan struct{})
		go func() { d.stopLightingRenderer(); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("unbounded stop")
		}
		select {
		case <-runner.done:
		default:
			t.Fatal("renderer did not exit")
		}
		d.rendererMu.Lock()
		stale := d.activeRgb != nil || d.lightingRenderer != nil
		d.rendererMu.Unlock()
		if stale {
			t.Fatal("stale renderer ownership")
		}
	}
	d.setDeviceColor()
	t.Cleanup(d.stopLightingRenderer)
	waitFrame()
	d.rendererMu.Lock()
	runner := d.lightingRenderer
	d.rendererMu.Unlock()
	if runner == nil {
		t.Fatal("no renderer")
	}
	d.rgbMutex.Lock()
	profile := d.Rgb.Profiles["wave"]
	delete(d.Rgb.Profiles, "wave")
	d.rgbMutex.Unlock()
	// Allow any already resolved frame to finish, then verify missing-profile
	// iterations neither write frames nor retire the captured renderer.
	time.Sleep(80 * time.Millisecond)
	before := count()
	time.Sleep(80 * time.Millisecond)
	if count() != before {
		t.Fatal("output continued without a profile")
	}
	select {
	case <-runner.done:
		t.Fatal("renderer died on missing RGB profile")
	default:
	}
	d.rendererMu.Lock()
	owned := d.lightingRenderer == runner && d.activeRgb == runner.rgb
	d.rendererMu.Unlock()
	if !owned {
		t.Fatal("renderer ownership changed during retry")
	}
	select {
	case <-frames:
	default:
	}
	d.rgbMutex.Lock()
	d.Rgb.Profiles["wave"] = profile
	d.rgbMutex.Unlock()
	waitFrame()
	if count() <= before {
		t.Fatal("renderer failed to recover")
	}
	// Shutdown while retrying must still honor the original stop channel.
	d.rgbMutex.Lock()
	delete(d.Rgb.Profiles, "wave")
	d.rgbMutex.Unlock()
	time.Sleep(40 * time.Millisecond)
	stop(runner)
	d.rgbMutex.Lock()
	d.Rgb.Profiles["wave"] = profile
	d.rgbMutex.Unlock()
	select {
	case <-frames:
	default:
	}
	d.setDeviceColor()
	d.rendererMu.Lock()
	replacement := d.lightingRenderer
	d.rendererMu.Unlock()
	if replacement == nil || replacement == runner {
		t.Fatal("restart did not replace renderer")
	}
	waitFrame()
	stop(replacement)
}

func TestLegacyGradientMutationConcurrentRendererAndReads(t *testing.T) {
	d := newFailedAttachmentLegacyTestDevice(t)
	d.DeviceProfile.RGBProfile = "gradient"
	frames := make(chan struct{}, 1)
	// Calling the getter from output also proves the mutation released rgbMutex
	// before starting the replacement renderer or issuing transport writes.
	d.lightingTransfer = func(command byte, endpoint, buffer []byte) error {
		d.GetRgbProfile("gradient")
		if command == cmdWrite && len(endpoint) == 4 && endpoint[1] == 3 {
			select {
			case frames <- struct{}{}:
			default:
			}
		}
		return nil
	}
	d.setDeviceColor()
	t.Cleanup(d.stopLightingRenderer)
	waitFrame := func() {
		t.Helper()
		select {
		case <-frames:
		case <-time.After(time.Second):
			t.Fatal("animated fallback did not output")
		}
	}
	waitFrame()
	if d.lightingDefaults != nil {
		t.Fatal("canonical runtime unexpectedly available")
	}

	// Keep additional getter reads overlapping every mutation, even while the
	// old renderer is retiring and the replacement is starting.
	stopReads := make(chan struct{})
	ready := make(chan struct{}, 3)
	readErrors := make(chan struct{}, 1)
	var readers sync.WaitGroup
	for i := 0; i < 3; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			d.GetRgbProfile("gradient")
			ready <- struct{}{}
			for {
				select {
				case <-stopReads:
					return
				default:
				}
				if d.GetRgbProfile("gradient") == nil {
					select {
					case readErrors <- struct{}{}:
					default:
					}
					return
				}
				d.GetRgbProfile("wave")
			}
		}()
	}
	t.Cleanup(func() { close(stopReads); readers.Wait() })
	for i := 0; i < 3; i++ {
		<-ready
	}

	boundedMutation := func(add bool) (uint8, uint) {
		t.Helper()
		type result struct {
			status uint8
			index  uint
		}
		done := make(chan result, 1)
		go func() {
			var r result
			if add {
				r.status, r.index = d.ProcessNewGradientColor("gradient")
			} else {
				r.status, r.index = d.ProcessDeleteGradientColor("gradient")
			}
			done <- r
		}()
		select {
		case r := <-done:
			return r.status, r.index
		case <-time.After(time.Second):
			t.Fatal("gradient mutation/restart deadlocked")
			return 0, 0
		}
	}
	assertPersisted := func() {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(pwd, "database", "rgb", d.Serial+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var saved rgb.RGB
		if err = json.Unmarshal(data, &saved); err != nil {
			t.Fatal(err)
		}
		current := d.GetRgbProfile("gradient")
		if current == nil || !reflect.DeepEqual(saved.Profiles["gradient"], *current) {
			t.Fatal("gradient state was not persisted")
		}
	}
	original := d.GetRgbProfile("gradient")
	for i := 0; i < 8; i++ {
		for _, add := range []bool{true, false} {
			d.rendererMu.Lock()
			previous := d.lightingRenderer
			d.rendererMu.Unlock()
			if previous == nil {
				t.Fatal("animated renderer unavailable before mutation")
			}
			select {
			case <-frames:
			default:
			}
			status, index := boundedMutation(add)
			if status != 1 {
				t.Fatal("legacy gradient mutation failed", add, status, index)
			}
			assertPersisted()
			select {
			case <-previous.done:
			default:
				t.Fatal("previous renderer did not retire")
			}
			d.rendererMu.Lock()
			replacement := d.lightingRenderer
			owned := replacement != nil && d.activeRgb == replacement.rgb
			d.rendererMu.Unlock()
			if !owned || replacement == previous {
				t.Fatal("renderer restart ownership")
			}
			waitFrame()
		}
	}
	if !reflect.DeepEqual(d.GetRgbProfile("gradient"), original) {
		t.Fatal("add/delete changed original gradients")
	}
	select {
	case <-readErrors:
		t.Fatal("concurrent profile reads failed")
	default:
	}
	done := make(chan struct{})
	go func() { d.stopLightingRenderer(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("final renderer shutdown blocked")
	}
	d.rendererMu.Lock()
	stale := d.lightingRenderer != nil || d.activeRgb != nil
	d.rendererMu.Unlock()
	if stale || d.GetRgbProfile("gradient") == nil {
		t.Fatal("stale ownership or blocked getter")
	}
}
