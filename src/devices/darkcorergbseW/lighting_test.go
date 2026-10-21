package darkcorergbseW

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
)

func requireLighting(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func lightingFixture(t *testing.T, attach bool) (*Device, *lightingsettings.IndependentDeviceRuntime, config.Paths) {
	t.Helper()
	logger.Init()
	root := t.TempDir()
	previous := pwd
	pwd = root
	t.Cleanup(func() { pwd = previous })
	for _, dir := range []string{"profiles", "rgb"} {
		requireLighting(t, os.MkdirAll(filepath.Join(root, "database", dir), 0700))
	}
	paths := config.Paths{ShippedDatabaseRoot: "../../../database", OpenRGBDeviceLightingFile: filepath.Join(root, "state.json"), DeviceEffectSettingsFile: filepath.Join(root, "effects.json")}
	rt, err := lightingsettings.LoadIndependentDeviceRuntime(paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile, filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	requireLighting(t, err)
	b := uint8(17)
	profile := &DeviceProfile{Active: true, Product: "DARK CORE SE", Serial: "6987", Path: filepath.Join(root, "database/profiles/6987.json"), RGBProfile: "gradient", BrightnessSlider: &b, OriginalBrightness: 82, RgbOff: false, Profile: 1, PollingRate: 1, SleepMode: 15, LiftHeight: 3, DPIColor: &rgb.Color{Green: 255}, SniperColor: &rgb.Color{Red: 255, Green: 255}, Profiles: map[int]DPIProfile{0: {Name: "Stage 1", Value: 800, PackerIndex: 1}, 1: {Name: "Stage 2", Value: 1500, PackerIndex: 2}, 2: {Name: "Stage 3", Value: 3000, PackerIndex: 3}, 3: {Name: "Sniper", Value: 200, PackerIndex: 4, Sniper: true}}, ZoneColors: map[int]ZoneColors{0: {Name: "old", ColorIndex: 99, ColorType: 99, Color: &rgb.Color{Red: 9}}}}
	d := &Device{Serial: "6987", Product: "DARK CORE SE", ProductId: 6987, VendorId: 6940, LEDChannels: 3, ZoneAmount: 3, DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"default": profile}, Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{"static": {StartColor: rgb.Color{Red: 11}}, "gradient": {Gradients: map[int]rgb.Color{0: {Red: 77}}}}}}
	requireLighting(t, os.WriteFile(profile.Path, []byte("legacy profile sentinel"), 0600))
	requireLighting(t, os.WriteFile(filepath.Join(root, "database/rgb/6987.json"), []byte("legacy RGB sentinel"), 0600))
	if attach {
		requireLighting(t, d.attachLightingSource(rt))
	}
	t.Cleanup(d.stopLighting)
	return d, rt, paths
}
func lightingFrame(t *testing.T, d *Device) [3]lightingZoneOutput {
	t.Helper()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	v, err := d.composeLightingLocked()
	requireLighting(t, err)
	return v
}
func inputJSON(t *testing.T, d *Device) []byte {
	t.Helper()
	v, err := json.Marshal(d.DeviceProfile)
	requireLighting(t, err)
	return v
}

func TestCanonicalCleanAttachmentCatalogueTopology(t *testing.T) {
	d, rt, paths := lightingFixture(t, true)
	s, ok := d.LightingSnapshot()
	if !ok || s.ConfiguredEffect != "static" || s.Brightness != 100 || s.SingleColorHex != "#00ffff" {
		t.Fatalf("clean defaults: %#v", s)
	}
	want := []string{"mouse", "static"}
	var ids []string
	for _, e := range s.SupportedEffects {
		ids = append(ids, e.ID)
	}
	if !reflect.DeepEqual(ids, want) || !reflect.DeepEqual(rgbModes, want) {
		t.Fatalf("catalogue: %v", ids)
	}
	for _, unsupported := range []string{"gradient", "off", "cpu-temperature", "gpu-temperature", "rainbow"} {
		if d.SupportsLightingEffect(unsupported) || d.SetLightingEffect(unsupported) == nil || d.GetRgbProfile(unsupported) != nil {
			t.Fatalf("invented effect %s", unsupported)
		}
	}
	if _, found, err := rt.State.Resolve(d.Serial); err != nil || found {
		t.Fatal("attachment imported/wrote legacy state")
	}
	for _, p := range []string{paths.OpenRGBDeviceLightingFile, paths.DeviceEffectSettingsFile} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatal("clean attachment persisted", p, err)
		}
	}
	requireLighting(t, d.SetLightingEffect("mouse"))
	s, ok = d.LightingSnapshot()
	expected := []struct {
		id, name, color string
		selector, kind  int
	}{{"0", "Logo", "#ffff00", 1, 3}, {"1", "Scroll", "#00ffff", 4, 5}, {"2", "Side", "#00ffff", 2, 4}}
	if !ok || s.AuthoredZoneEditor == nil || len(s.AuthoredZoneEditor.Zones) != 3 {
		t.Fatalf("zones: %#v", s)
	}
	frame := lightingFrame(t, d)
	for i, w := range expected {
		got := s.AuthoredZoneEditor.Zones[i]
		if got.ID != w.id || got.Label != w.name || got.ColorHex != w.color || frame[i].selector != w.selector || frame[i].kind != w.kind {
			t.Fatalf("zone %d: %#v %#v", i, got, frame[i])
		}
	}
	if s.ClusterControlled || s.ExternalControlled || len(s.Channels) != 0 {
		t.Fatal("invented ownership")
	}
}

func TestCanonicalAuthorityCopiesAndReset(t *testing.T) {
	d, _, paths := lightingFixture(t, true)
	before := inputJSON(t, d)
	requireLighting(t, d.SetLightingEffect("mouse"))
	requireLighting(t, d.SetLightingBrightness(60))
	requireLighting(t, d.SetLightingZoneColors("mouse", []string{"0", "2"}, rgb.Color{Red: 200, Green: 100, Blue: 50}))
	value, err := d.ResolveLightingEffectSettings("static")
	requireLighting(t, err)
	value.SingleColor.Color.Red = 120
	requireLighting(t, d.SetLightingEffectSettings("static", value))
	value.SingleColor.Color.Red = 240
	got, err := d.ResolveLightingEffectSettings("static")
	requireLighting(t, err)
	if got.SingleColor.Color.Red != 120 {
		t.Fatal("retained caller pointer")
	}
	got.SingleColor.Color.Red = 1
	snap, _ := d.LightingSnapshot()
	snap.SupportedEffects[0].ID = "gradient"
	snap.AuthoredZoneEditor.Zones[0].ColorHex = "#ffffff"
	if !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("Lighting mutation changed input/legacy profile")
	}
	// Destroy every old Lighting authority; canonical rendering/getters survive.
	d.DeviceProfile.RGBProfile = "bad"
	d.DeviceProfile.BrightnessSlider = nil
	d.DeviceProfile.ZoneColors = nil
	d.DeviceProfile.RgbOff = true
	d.Rgb = nil
	snap, ok := d.LightingSnapshot()
	if !ok || snap.ConfiguredEffect != "mouse" || snap.Brightness != 60 || snap.AuthoredZoneEditor.Zones[0].ColorHex != "#c86432" {
		t.Fatalf("legacy leaked: %#v", snap)
	}
	if lightingFrame(t, d)[0].rgb != [3]byte{120, 60, 30} {
		t.Fatal("output read legacy")
	}
	requireLighting(t, d.SetLightingEffect("static"))
	if d.GetRgbProfile("static").StartColor.Red != 120 {
		t.Fatal("canonical compatibility getter")
	}
	if code, _ := d.ProcessNewGradientColor("gradient"); code != 0 {
		t.Fatal("gradient edit exposed")
	}
	if code, _ := d.ProcessDeleteGradientColor("gradient"); code != 0 {
		t.Fatal("gradient delete exposed")
	}
	requireLighting(t, d.ResetLightingEffectSettings("static"))
	store, err := lightingsettings.LoadDeviceStore(paths.DeviceEffectSettingsFile)
	requireLighting(t, err)
	if _, found, err := store.Get(d.Serial, "static"); err != nil || found {
		t.Fatal("reset retained customization")
	}
	requireLighting(t, d.SetLightingBrightness(40))
	state, err := lightingsettings.LoadIndependentDeviceStateStore(paths.OpenRGBDeviceLightingFile)
	requireLighting(t, err)
	saved, found, err := state.Resolve(d.Serial)
	requireLighting(t, err)
	if !found || saved.Brightness != 40 || saved.SelectedEffect != "static" {
		t.Fatalf("disk: %#v", saved)
	}
	palette, found, err := store.GetAuthoredZones(d.Serial, "mouse")
	requireLighting(t, err)
	if !found || palette["0"].Red != 200 {
		t.Fatal("palette persistence")
	}
	for p, want := range map[string]string{d.DeviceProfile.Path: "legacy profile sentinel", filepath.Join(pwd, "database/rgb/6987.json"): "legacy RGB sentinel"} {
		data, err := os.ReadFile(p)
		requireLighting(t, err)
		if string(data) != want {
			t.Fatal("dual legacy write")
		}
	}
	if profiles := d.GetRgbProfiles().(rgb.RGB); len(profiles.Profiles) != 2 {
		t.Fatal("compatibility catalogue")
	}
}

func TestStaticDoesNotCompoundAcrossZonesOrFrames(t *testing.T) {
	d, _, _ := lightingFixture(t, true)
	value, err := d.ResolveLightingEffectSettings("static")
	requireLighting(t, err)
	value.SingleColor.Color = lightingsettings.Color{Red: 200, Green: 100, Blue: 50}
	requireLighting(t, d.SetLightingEffectSettings("static", value))
	requireLighting(t, d.SetLightingZoneColors("mouse", []string{"0", "2"}, rgb.Color{Red: 91, Green: 47, Blue: 23}))
	d.lightingMu.Lock()
	paletteBefore, err := d.lightingCanonical.palette(d.Serial)
	d.lightingMu.Unlock()
	requireLighting(t, err)
	inputBefore := inputJSON(t, d)
	for _, brightness := range []uint8{50, 25, 100} {
		requireLighting(t, d.SetLightingBrightness(brightness))
		for frame := 0; frame < 8; frame++ {
			out := lightingFrame(t, d)
			for _, zone := range out {
				want := [3]byte{byte(200 * float64(brightness) / 100), byte(100 * float64(brightness) / 100), byte(50 * float64(brightness) / 100)}
				if zone.rgb != want {
					t.Fatalf("brightness %d frame %d: %v want %v", brightness, frame, zone.rgb, want)
				}
			}
		}
	}
	after, err := d.ResolveLightingEffectSettings("static")
	requireLighting(t, err)
	if !reflect.DeepEqual(after, value) || !bytes.Equal(inputBefore, inputJSON(t, d)) {
		t.Fatal("scaling mutated desired/input values")
	}
	d.lightingMu.Lock()
	palette, err := d.lightingCanonical.palette(d.Serial)
	d.lightingMu.Unlock()
	requireLighting(t, err)
	if !reflect.DeepEqual(palette, paletteBefore) {
		t.Fatal("Static changed Mouse palette")
	}
}

func TestPersistenceFailureAtomicAndNoOutput(t *testing.T) {
	for _, kind := range []string{"effect-selection", "brightness", "static", "zone", "reset", "mixed"} {
		t.Run(kind, func(t *testing.T) {
			d, rt, paths := lightingFixture(t, true)
			requireLighting(t, d.SetLightingEffect("mouse"))
			requireLighting(t, d.SetLightingZoneColors("mouse", []string{"0"}, rgb.Color{Red: 90}))
			settings, err := d.ResolveLightingEffectSettings("static")
			requireLighting(t, err)
			requireLighting(t, d.SetLightingEffectSettings("static", settings))
			before, _ := d.LightingSnapshot()
			input := inputJSON(t, d)
			stateBefore, _, _ := rt.State.Resolve(d.Serial)
			path := paths.DeviceEffectSettingsFile
			if kind == "effect-selection" || kind == "brightness" {
				path = paths.OpenRGBDeviceLightingFile
			}
			requireLighting(t, os.Remove(path))
			requireLighting(t, os.Mkdir(path, 0700)) // rename failure even as root
			var writes atomic.Int32
			d.lightingWrite = func([]byte) error { writes.Add(1); return nil }
			d.beginLightingConnection()
			owner := &lightingApply{stop: make(chan struct{})}
			d.lightingApply = owner
			switch kind {
			case "effect-selection":
				err = d.SetLightingEffect("static")
			case "brightness":
				err = d.SetLightingBrightness(5)
			case "static":
				settings.SingleColor.Color.Red = 1
				err = d.SetLightingEffectSettings("static", settings)
			case "zone":
				err = d.SetLightingZoneColors("mouse", []string{"0", "1"}, rgb.Color{Red: 1})
			case "reset":
				err = d.ResetLightingEffectSettings("static")
			case "mixed":
				if d.SaveMouseZoneColorsSniper(rgb.Color{Red: 25}, map[int]rgb.Color{0: {Red: 1}}, rgb.Color{Blue: 70}) != 0 {
					t.Fatal("mixed failure hidden")
				}
				err = os.ErrPermission
			}
			if err == nil {
				t.Fatal("persistence failure hidden")
			}
			after, _ := d.LightingSnapshot()
			stateAfter, _, _ := rt.State.Resolve(d.Serial)
			if writes.Load() != 0 || d.lightingApply != owner || !reflect.DeepEqual(before, after) || stateBefore != stateAfter || !bytes.Equal(input, inputJSON(t, d)) {
				t.Fatal("failed persistence published/cancelled/output/changed input")
			}
			select {
			case <-owner.stop:
				t.Fatal("failure cancelled prior output")
			default:
			}
		})
	}
}

func TestDarknessDisconnectedMutationAndReconnect(t *testing.T) {
	d, _, _ := lightingFixture(t, true)
	var packets [][]byte
	d.lightingWrite = func(p []byte) error { packets = append(packets, p); return nil }
	requireLighting(t, d.SetLightingBrightness(70))
	if d.SchedulerBrightness(0) != 1 {
		t.Fatal("scheduler ON disconnected rejected")
	}
	requireLighting(t, d.SetLightingBrightness(40))
	d.ControlDeviceRgb(true)
	if len(packets) != 0 {
		t.Fatal("disconnected mutation wrote hardware")
	}
	if d.SchedulerBrightness(1) != 1 {
		t.Fatal("scheduler OFF disconnected rejected")
	}
	if !d.beginLightingConnection() {
		t.Fatal("connection transition")
	}
	d.setDeviceColor()
	if len(packets) != 3 {
		t.Fatal("reconnect did not apply")
	}
	for _, p := range packets {
		if !bytes.Equal(p[10:16], make([]byte, 6)) {
			t.Fatal("RGB-off lost on reconnect")
		}
	}
	d.ControlDeviceRgb(false)
	for _, p := range packets[len(packets)-3:] {
		if !bytes.Equal(p[10:13], []byte{0, 102, 102}) {
			t.Fatalf("stale restore: %v", p[10:13])
		}
	}
	d.SchedulerBrightness(0)
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(true)
	d.SchedulerBrightness(255)
	if lightingFrame(t, d)[0].rgb != [3]byte{} {
		t.Fatal("scheduler released user off")
	}
	d.ControlDeviceRgb(false)
	snap, _ := d.LightingSnapshot()
	if snap.Brightness != 40 || lightingFrame(t, d)[0].rgb != [3]byte{0, 102, 102} {
		t.Fatal("latest desired brightness not restored")
	}
	d.disconnectLighting(false)
	d.SchedulerBrightness(0)
	d.ControlDeviceRgb(true)
	d.SchedulerBrightness(1)
	d.ControlDeviceRgb(false)
	if !d.beginLightingConnection() {
		t.Fatal("second connection")
	}
	d.setDeviceColor()
	if lightingFrame(t, d)[2].rgb != [3]byte{0, 102, 102} {
		t.Fatal("second wake stale")
	}
	if d.DeviceProfile.OriginalBrightness != 82 || *d.DeviceProfile.BrightnessSlider != 17 || d.DeviceProfile.RgbOff {
		t.Fatal("override changed brightness scratch or failed to persist RGB-on")
	}
}

func TestExactHardwareAndInputIndependence(t *testing.T) {
	d, _, _ := lightingFixture(t, true)
	var packets [][]byte
	d.lightingWrite = func(p []byte) error { packets = append(packets, p); return nil }
	requireLighting(t, d.SetLightingEffect("mouse"))
	before := inputJSON(t, d)
	d.SniperMode = true
	d.beginLightingConnection()
	d.setDeviceColor()
	selectors := []byte{1, 4, 2}
	kinds := []byte{3, 5, 4}
	colors := [][3]byte{{255, 255, 0}, {0, 255, 255}, {0, 255, 255}}
	if len(packets) != 3 {
		t.Fatal("wrong report count")
	}
	for i, p := range packets {
		want := make([]byte, 65)
		copy(want, []byte{0, 7, 0xaa, 0, 0, selectors[i], 7, 0, 0, 0x64})
		copy(want[10:13], colors[i][:])
		copy(want[13:16], colors[i][:])
		want[16] = kinds[i]
		if !bytes.Equal(p, want) {
			t.Fatalf("packet %d: %x want %x", i, p, want)
		}
	}
	if !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("Lighting changed DPI/Sniper/authored backing")
	}
	packets = nil
	d.ControlDeviceRgb(true)
	packets = nil
	d.updateMouseDPI()
	d.sniperMode(true)
	if len(packets) != 5 {
		t.Fatalf("DPI/Sniper packets %d", len(packets))
	}
	for _, p := range packets[:4] {
		if p[2] != 0x13 {
			t.Fatal("DPI command changed")
		}
		if p[3] == 208 {
			if !bytes.Equal(p[10:13], []byte{255, 255, 0}) {
				t.Fatal("Sniper config scaled/darkened")
			}
		} else if !bytes.Equal(p[10:13], []byte{0, 255, 0}) {
			t.Fatal("DPI config scaled/darkened")
		}
	}
	if !bytes.Equal(packets[4][:6], []byte{0, 7, 0x13, 2, 0, 0}) {
		t.Fatal("Sniper selection changed")
	}
	// Input owns SniperMode; neither selecting it nor darkness changes palettes.
	if lightingFrame(t, d)[0].rgb != [3]byte{} {
		t.Fatal("authored zones not dark")
	}
}

func TestAttachmentFailureDoesNotPublishOrReadLegacy(t *testing.T) {
	for _, kind := range []string{"runtime", "topology", "state", "palette", "defaults"} {
		t.Run(kind, func(t *testing.T) {
			d, rt, _ := lightingFixture(t, false)
			input := inputJSON(t, d)
			switch kind {
			case "runtime":
				rt = nil
			case "topology":
				d.LEDChannels = 12
			case "state":
				requireLighting(t, rt.State.Set(d.Serial, lightingsettings.IndependentDeviceLightingState{SelectedEffect: "gradient", Brightness: 70}))
			case "palette":
				requireLighting(t, rt.Effects.SetAuthoredZones(d.Serial, "mouse", map[string]lightingsettings.Color{"0": {Red: 1}}))
			case "defaults":
				rt = &lightingsettings.IndependentDeviceRuntime{State: rt.State, Effects: rt.Effects, Resolver: rt.Resolver}
			}
			if d.attachLightingSource(rt) == nil || d.lightingAttached() {
				t.Fatal("partial attachment")
			}
			if _, ok := d.LightingSnapshot(); ok {
				t.Fatal("unusable snapshot exposed")
			}
			if !bytes.Equal(input, inputJSON(t, d)) {
				t.Fatal("failed attach changed input")
			}
		})
	}
	d, rt, _ := lightingFixture(t, false)
	d.Rgb = nil
	d.DeviceProfile.BrightnessSlider = nil
	d.DeviceProfile.ZoneColors = nil
	requireLighting(t, d.attachLightingSource(rt))
	if _, ok := d.LightingSnapshot(); !ok {
		t.Fatal("attachment depended on legacy Lighting")
	}
}

func TestApplyCancellationBoundedAndStaleOwnership(t *testing.T) {
	d, _, _ := lightingFixture(t, true)
	var writes atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	d.lightingWrite = func([]byte) error {
		if writes.Add(1) == 1 {
			close(entered)
			<-release
		}
		return nil
	}
	d.beginLightingConnection()
	oldDone := make(chan struct{})
	go func() { defer close(oldDone); d.setDeviceColor() }()
	<-entered
	d.lightingMu.Lock()
	old := d.lightingApply
	d.lightingMu.Unlock()
	stopDone := make(chan struct{})
	go func() { d.stopLighting(); close(stopDone) }()
	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("stop waited on HID")
	}
	// New owner cannot be retired by an old apply, even before the blocked write returns.
	newer := &lightingApply{stop: make(chan struct{})}
	d.lightingMu.Lock()
	d.lightingApply = newer
	d.lightingMu.Unlock()
	d.retireLightingApply(old)
	d.lightingMu.Lock()
	current := d.lightingApply
	d.lightingMu.Unlock()
	if current != newer {
		t.Fatal("stale cleanup erased newer owner")
	}
	close(release)
	select {
	case <-oldDone:
	case <-time.After(time.Second):
		t.Fatal("cancelled apply did not finish")
	}
	if writes.Load() != 1 {
		t.Fatal("cancelled apply wrote more zones")
	}
	d.stopLighting()
	d.setDeviceColor()
	if writes.Load() != 4 {
		t.Fatal("restart failed")
	}
	d.exitLighting()
	d.setDeviceColor()
	if writes.Load() != 4 || d.SchedulerBrightness(0) != 0 || d.SetLightingBrightness(70) == nil {
		t.Fatal("stopped guard")
	}
}

func TestConcurrentCanonicalMutationsConnectionAndOutput(t *testing.T) {
	d, _, _ := lightingFixture(t, true)
	var writes atomic.Int32
	d.lightingWrite = func([]byte) error { writes.Add(1); return nil }
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for i := 0; i < 6; i++ {
				switch n {
				case 0:
					d.SetLightingEffect("mouse")
					d.SetLightingZoneColors("mouse", []string{"0", "1"}, rgb.Color{Red: float64(i)})
				case 1:
					d.SchedulerBrightness(uint8(i % 2))
					d.ControlDeviceRgb(i%2 == 0)
				case 2:
					d.disconnectLighting(false)
					d.beginLightingConnection()
					d.setDeviceColor()
				case 3:
					d.SetLightingBrightness(uint8(i * 10))
					d.LightingSnapshot()
				}
			}
		}(worker)
	}
	wg.Wait()
	d.stopLighting()
	if _, ok := d.LightingSnapshot(); !ok {
		t.Fatal("concurrent state invalid")
	}
}

func TestMixedMouseFormPreservesInputCommandsAfterCanonicalCommit(t *testing.T) {
	d, _, _ := lightingFixture(t, true)
	requireLighting(t, d.SetLightingEffect("mouse"))
	var packets [][]byte
	d.lightingWrite = func(p []byte) error { packets = append(packets, p); return nil }
	d.beginLightingConnection()
	dpi := rgb.Color{Red: 12, Green: 34, Blue: 56}
	sniper := rgb.Color{Red: 78, Green: 90, Blue: 123}
	if d.SaveMouseZoneColorsSniper(dpi, map[int]rgb.Color{0: {Red: 111}}, sniper) != 1 {
		t.Fatal("combined form rejected")
	}
	if len(packets) != 7 {
		t.Fatalf("commands %d", len(packets))
	}
	for _, p := range packets[:4] {
		if p[2] != 0x13 {
			t.Fatal("Lighting applied before input config")
		}
		want := []byte{12, 34, 56}
		if p[3] == 208 {
			want = []byte{78, 90, 123}
		}
		if !bytes.Equal(p[10:13], want) {
			t.Fatal("input color source changed")
		}
	}
	for _, p := range packets[4:] {
		if p[2] != 0xaa {
			t.Fatal("zone command changed")
		}
	}
	if lightingFrame(t, d)[0].rgb != [3]byte{111, 0, 0} || d.DeviceProfile.DPIColor.Red != 12 || d.DeviceProfile.SniperColor.Red != 78 {
		t.Fatal("combined state not preserved")
	}
	if d.DeviceProfile.ZoneColors[0].Color.Red != 9 {
		t.Fatal("dual-wrote legacy authored palette")
	}
	packets = nil
	if d.SaveMouseZoneColors(dpi, map[int]rgb.Color{1: {Blue: 88}}) != 1 {
		t.Fatal("non-Sniper form rejected")
	}
	if len(packets) != 8 || !bytes.Equal(packets[4][:6], []byte{0, 7, 0x13, 2, 0, 2}) {
		t.Fatal("regular form lost DPI selection/order")
	}
}

func TestCanonicalInvalidMutationsAreAtomic(t *testing.T) {
	d, _, _ := lightingFixture(t, true)
	before, _ := d.LightingSnapshot()
	input := inputJSON(t, d)
	for _, err := range []error{
		d.SetLightingBrightness(101),
		d.SetLightingZoneColors("mouse", []string{"0", "0"}, rgb.Color{Red: 1}),
		d.SetLightingZoneColors("mouse", []string{"0", "3"}, rgb.Color{Red: 1}),
		d.SetLightingZoneColors("mouse", []string{"0", "1"}, rgb.Color{Red: 256}),
		d.SetLightingZoneColor("mouse", "group", "", "DPI", rgb.Color{}),
	} {
		if err == nil {
			t.Fatal("invalid mutation accepted")
		}
	}
	after, _ := d.LightingSnapshot()
	if !reflect.DeepEqual(before, after) || !bytes.Equal(input, inputJSON(t, d)) {
		t.Fatal("invalid request changed state")
	}
}

func TestDisconnectedStateCanAdvanceWhilePriorHIDWriteIsStalled(t *testing.T) {
	d, _, _ := lightingFixture(t, true)
	entered, release := make(chan struct{}), make(chan struct{})
	var writes atomic.Int32
	d.lightingWrite = func([]byte) error {
		if writes.Add(1) == 1 {
			close(entered)
			<-release
		}
		return nil
	}
	d.beginLightingConnection()
	done := make(chan error, 1)
	go func() { done <- d.SetLightingBrightness(70) }()
	<-entered
	d.disconnectLighting(false)
	changed := make(chan error, 1)
	go func() {
		if d.SchedulerBrightness(0) != 1 {
			changed <- os.ErrInvalid
			return
		}
		if err := d.SetLightingBrightness(40); err != nil {
			changed <- err
			return
		}
		if d.SchedulerBrightness(1) != 1 {
			changed <- os.ErrInvalid
			return
		}
		changed <- nil
	}()
	select {
	case err := <-changed:
		requireLighting(t, err)
	case <-time.After(time.Second):
		close(release)
		t.Fatal("hardware write blocked desired/transient state")
	}
	close(release)
	requireLighting(t, <-done)
	d.beginLightingConnection()
	d.setDeviceColor()
	if lightingFrame(t, d)[0].rgb != [3]byte{0, 102, 102} {
		t.Fatal("reconnect used stale brightness")
	}
}

func TestQueuedApplyCancellationDoesNotWaitForOlderHID(t *testing.T) {
	d, _, _ := lightingFixture(t, true)
	// Hold the existing transport lock, as an older HID operation would. A newer
	// Lighting apply must be cancellable while it is waiting in that queue.
	d.lightingWrite = func([]byte) error { t.Error("cancelled queued frame reached hardware"); return nil }
	d.beginLightingConnection()
	d.mutex.Lock()
	done := make(chan struct{})
	go func() { defer close(done); d.setDeviceColor() }()
	deadline := time.Now().Add(time.Second)
	for {
		d.lightingMu.Lock()
		active := d.lightingApply != nil
		d.lightingMu.Unlock()
		if active {
			break
		}
		if time.Now().After(deadline) {
			d.mutex.Unlock()
			t.Fatal("apply not installed")
		}
		time.Sleep(time.Millisecond)
	}
	d.stopLighting()
	select {
	case <-done:
	case <-time.After(time.Second):
		d.mutex.Unlock()
		t.Fatal("queued apply waited for old HID")
	}
	d.mutex.Unlock()
}
