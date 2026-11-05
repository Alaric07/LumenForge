package m65rgbultra

import (
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestUltraGradientMapKeysPersistAcrossSortingAndReload(t *testing.T) {
	d, rt := lightingFixture(t, true)
	p := d.GetRgbProfile("gradient")
	p.Gradients = map[int]rgb.Color{0: {Red: 10, Position: 0, Brightness: 1}, 1: {Red: 20, Position: 1, Brightness: 1}}
	if d.UpdateRgbProfileData("gradient", *p) != 1 {
		t.Fatal("seed")
	}
	status, key := d.ProcessNewGradientColor("gradient")
	if status != 1 || key != 2 {
		t.Fatal(status, key)
	}
	p = d.GetRgbProfile("gradient")
	if p.Gradients[2] != (rgb.Color{Green: 255, Blue: 255}) {
		t.Fatal("key is not the new source-default color", p.Gradients)
	}
	// Position sorting places key 2 ahead of key 1; its identity must not change.
	stored, _, err := rt.Effects.Get(d.Serial, "gradient")
	requireLighting(t, err)
	if !reflect.DeepEqual(stored.Gradient.Keys, []int{0, 2, 1}) {
		t.Fatal(stored)
	}
	p.Gradients[2] = rgb.Color{Red: 93, Position: .25, Brightness: .7}
	if d.UpdateRgbProfileData("gradient", *p) != 1 {
		t.Fatal("edit returned key")
	}
	if d.GetRgbProfile("gradient").Gradients[2].Red != 93 || d.GetRgbProfile("gradient").Gradients[1].Red != 20 {
		t.Fatal("wrong key edited")
	}
	reloaded, err := lightingsettings.LoadIndependentDeviceRuntime(filepath.Join(filepath.Dir(d.DeviceProfile.Path), "state.json"), filepath.Join(filepath.Dir(d.DeviceProfile.Path), "effects.json"), "../../../database/rgb.json")
	requireLighting(t, err)
	value, err := reloaded.Resolver.Resolve(lightingsettings.IndependentDevice(d.Serial), "gradient")
	requireLighting(t, err)
	if !reflect.DeepEqual(ultraRendererProfile(value.Settings).Gradients, d.GetRgbProfile("gradient").Gradients) {
		t.Fatal("key identities lost on reload")
	}
	status, key = d.ProcessDeleteGradientColor("gradient")
	if status != 1 || key != 2 {
		t.Fatal(status, key)
	}
	if _, ok := d.GetRgbProfile("gradient").Gradients[2]; ok {
		t.Fatal("wrong key removed")
	}
	status, key = d.ProcessNewGradientColor("gradient")
	if status != 1 || key != 2 {
		t.Fatal("source maxKey+1 allocation changed", status, key)
	}
	before := d.GetRgbProfile("gradient")
	d.lightingCanonical.effects = failingEffects{rt.Effects}
	if status, key = d.ProcessNewGradientColor("gradient"); status != 0 || key != 0 {
		t.Fatal(status, key)
	}
	if !reflect.DeepEqual(before, d.GetRgbProfile("gradient")) {
		t.Fatal("failed add published")
	}
}

func TestUltraGradientEqualPositionsKeepNumericKeyInputOrder(t *testing.T) {
	d, _ := lightingFixture(t, true)
	p := d.GetRgbProfile("gradient")
	p.Gradients = map[int]rgb.Color{0: {Red: 1, Position: .5, Brightness: 1}, 1: {Red: 2, Position: .5, Brightness: 1}, 2: {Red: 3, Position: 0, Brightness: 1}}
	if d.UpdateRgbProfileData("gradient", *p) != 1 {
		t.Fatal("valid duplicate positions")
	}
	q := d.GetRgbProfile("gradient")
	if !reflect.DeepEqual(p.Gradients, q.Gradients) {
		t.Fatal("map identities changed", q.Gradients)
	}
	// Canonical conversion restores the original numeric-key map. The existing
	// rgb Gradient renderer remains the authority for equal-position rendering.
	before := copyLightingRGBProfile(*q)
	r := rgb.New(2, 1, nil, nil, 1, 1, 0, true)
	r.ColorshiftGradient(time.Now(), q.Gradients, 1)
	if !reflect.DeepEqual(before.Gradients, q.Gradients) {
		t.Fatal("render mutated stops")
	}
}

func TestUltraDPIAndSniperUseBothAxes(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	type call struct{ command, data []byte }
	var calls []call
	d.lightingDPIWrite = func(c, b []byte) error {
		calls = append(calls, call{append([]byte(nil), c...), append([]byte(nil), b...)})
		return nil
	}
	before, _ := d.LightingSnapshot()
	assert := func(value uint16) {
		t.Helper()
		if len(calls) != 2 {
			t.Fatal("expected both axes", calls)
		}
		for axis, c := range calls {
			if !bytes.Equal(c.command, []byte{1, byte(0x21 + axis), 0}) || len(c.data) != 2 || binary.LittleEndian.Uint16(c.data) != value {
				t.Fatal("wrong Ultra DPI packet", c)
			}
		}
		calls = nil
	}
	if d.SaveMouseDPI(map[int]uint16{1: 26000}) != 1 {
		t.Fatal("26000 rejected")
	}
	assert(26000)
	for _, v := range []uint16{99, 26001} {
		if d.SaveMouseDPI(map[int]uint16{1: v}) != 0 {
			t.Fatal("invalid DPI accepted", v)
		}
	}
	if len(calls) != 0 {
		t.Fatal("invalid DPI programmed")
	}
	if d.SetMouseSniperMode(true) != 1 {
		t.Fatal("sniper")
	}
	assert(200)
	if d.SetMouseSniperMode(false) != 1 {
		t.Fatal("release")
	}
	assert(26000)
	after, _ := d.LightingSnapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("DPI modified authored Lighting")
	}
}

func TestUltraFreshAndUpgradedDefaults(t *testing.T) {
	d, _ := lightingFixture(t, false)
	previous := pwd
	pwd = t.TempDir()
	defer func() { pwd = previous }()
	requireLighting(t, os.MkdirAll(filepath.Join(pwd, "database", "profiles"), 0700))
	d.DeviceProfile = nil
	d.Serial = "ultratest"
	d.saveDeviceProfile()
	p := d.DeviceProfile
	if p == nil || p.RGBProfile != "mouse" || p.Profile != 1 || *p.BrightnessSlider != 100 || p.PollingRate != 4 || p.LiftHeight != 2 {
		t.Fatal(p)
	}
	for id, v := range []uint16{400, 800, 1200, 1600, 3000, 200} {
		s := p.Profiles[id]
		if s.Value != v || s.PackerIndex != id+1 || !reflect.DeepEqual(s.ColorIndex, map[int][]int{0: {2, 5, 8}}) {
			t.Fatal(id, s)
		}
	}
	fresh := p.Profiles[5]
	delete(p.Profiles, 5)
	d.upgradeDpiProfiles()
	if !reflect.DeepEqual(fresh, p.Profiles[5]) || !fresh.Sniper || fresh.Color.Red != 255 || fresh.Color.Green != 255 || fresh.Color.Blue != 0 {
		t.Fatal("Ultra default asymmetry introduced")
	}
}

func TestUltraSupportedPerformanceValuesAndRejection(t *testing.T) {
	d, _ := lightingFixture(t, true)
	readyLighting(d)
	writes := 0
	d.lightingDPIWrite = func([]byte, []byte) error { writes++; return nil }
	for _, v := range []int{2, 3, 4, 6, 7} {
		if d.UpdateLiftHeight(v) != 1 {
			t.Fatal("valid lift", v)
		}
		if _, ok := d.PerformanceSnapshot(); !ok {
			t.Fatal("lift snapshot", v)
		}
	}
	for _, v := range []int{0, 1} {
		if d.UpdateAngleSnapping(v) != 1 || d.UpdateButtonOptimization(v) != 1 {
			t.Fatal("valid toggle", v)
		}
	}
	before := inputJSON(t, d)
	saved, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	count := writes
	for _, v := range []int{-1, 0, 1, 5, 8} {
		if d.UpdateLiftHeight(v) != 0 {
			t.Fatal("invalid lift", v)
		}
	}
	for _, v := range []int{-1, 2, 5, 255} {
		if d.UpdateAngleSnapping(v) != 0 || d.UpdateButtonOptimization(v) != 0 {
			t.Fatal("invalid toggle", v)
		}
	}
	if d.UpdatePollingRate(8) != 0 || d.UpdatePollingRate(-1) != 0 {
		t.Fatal("invalid polling")
	}
	disk, err := os.ReadFile(d.DeviceProfile.Path)
	requireLighting(t, err)
	if writes != count || !bytes.Equal(before, inputJSON(t, d)) || !bytes.Equal(saved, disk) {
		t.Fatal("invalid input published")
	}
	d.lightingProfileWrite = func(string, []byte) error { return errors.New("disk") }
	if d.UpdateLiftHeight(2) != 0 || d.UpdateAngleSnapping(0) != 0 || d.UpdateButtonOptimization(0) != 0 {
		t.Fatal("save error hidden")
	}
	if writes != count || !bytes.Equal(before, inputJSON(t, d)) {
		t.Fatal("save failure changed input")
	}
}

func TestUltraHardwareReportAndDPIReportFraming(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.lightingWrite = nil
	reports := make(chan []byte, 10)
	d.lightingReportWrite = func(b []byte) (int, error) { reports <- append([]byte(nil), b...); return len(b), nil }
	readyLighting(d)
	if d.ProcessSetRgbCluster(true) != 1 {
		t.Fatal("cluster")
	}
	d.writeColorCluster([]byte{1, 2, 3, 4, 5, 6}, 0)
	got := waitFrame(t, reports)
	want := make([]byte, 65)
	copy(want, []byte{0, 8, 6, 0, 9, 0, 0, 0, 4, 1, 255, 5, 2, 255, 6, 3, 255})
	if !bytes.Equal(got, want) {
		t.Fatal("planar report", got)
	}
	d.lightingDPIWrite = nil
	d.programCanonicalDPI(false)
	for axis := 0; axis < 2; axis++ {
		got = waitFrame(t, reports)
		want = make([]byte, 65)
		copy(want, []byte{0, 8, 1, byte(0x21 + axis), 0, 0x20, 3})
		if !bytes.Equal(got, want) {
			t.Fatal("DPI report", got)
		}
	}
}

func TestUltraLegacyMissingIndicatorDoesNotLeakOrRetireQueue(t *testing.T) {
	d, _ := lightingFixture(t, false)
	d.DeviceProfile.OpenRGBIntegration = true
	stage := d.DeviceProfile.Profiles[1]
	stage.Color = nil
	d.DeviceProfile.Profiles[1] = stage
	done := make(chan struct{})
	go func() {
		d.applyLegacyOpenRGBFrame([]byte{1, 2, 3, 4, 5, 6})
		d.deviceLock.Lock()
		d.deviceLock.Unlock()
		close(done)
	}()
	waitDone(t, done)
	stage.Color = &rgb.Color{Red: 255}
	d.DeviceProfile.Profiles[1] = stage
	d.DeviceProfile.ZoneColors = map[int]ZoneColors{}
	for id, z := range lightingTopology() {
		z.Color = &rgb.Color{}
		d.DeviceProfile.ZoneColors[id] = z
	}
	reports := make(chan []byte, 10)
	d.lightingReportWrite = func(b []byte) (int, error) { reports <- append([]byte(nil), b...); return len(b), nil }
	d.applyLegacyOpenRGBFrame([]byte{1, 2, 3, 4, 5, 6})
	waitFrame(t, reports)
	d.DeviceProfile.RGBProfile = "static"
	d.DeviceProfile.OpenRGBIntegration = false
	d.setDeviceColor()
	waitFrame(t, reports)
	d.DeviceProfile.RGBCluster = true
	d.writeColorCluster([]byte{1, 2, 3, 4, 5, 6}, 0)
	waitFrame(t, reports)
}

func TestUltraPollingExactMapAndWireValue(t *testing.T) {
	for _, v := range []int{0, 1, 2, 3, 4, 5, 6, 7} {
		t.Run(string(rune('0'+v)), func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			var command, data []byte
			d.lightingDPIWrite = func(c, b []byte) error {
				command = append([]byte(nil), c...)
				data = append([]byte(nil), b...)
				return nil
			}
			if d.UpdatePollingRate(v) != 1 || !bytes.Equal(command, []byte{1, 1, 0}) || !bytes.Equal(data, []byte{byte(v)}) {
				t.Fatal("polling", v, command, data)
			}
			// Retain source Not Set persistence normalization while programming raw 0.
			expected := v
			if v == 0 {
				expected = 4
			}
			if d.DeviceProfile.PollingRate != expected || !d.Exit {
				t.Fatal("polling lifecycle")
			}
		})
	}
}

func TestUltraRGBOffStartupAndProfileReplay(t *testing.T) {
	d, rt := lightingFixture(t, false)
	d.DeviceProfile.RgbOff = true
	requireLighting(t, d.attachLightingSource(rt))
	if !d.userRGBOff {
		t.Fatal("startup RGB-off")
	}
	p := cloneLightingProfile(*d.DeviceProfile)
	p.Path = filepath.Join(filepath.Dir(p.Path), "other.json")
	p.RgbOff = false
	d.UserProfiles["other"] = &p
	requireLighting(t, d.SetLightingBrightness(63))
	requireLighting(t, d.setSchedulerDark(true))
	if d.ChangeDeviceProfile("other") != 1 || d.userRGBOff || !d.schedulerDark {
		t.Fatal("profile preference replay")
	}
	state, _, err := rt.State.Resolve(d.Serial)
	requireLighting(t, err)
	if state.Brightness != 63 {
		t.Fatal("profile overwrote desired brightness")
	}
	disk, err := os.ReadFile(p.Path)
	requireLighting(t, err)
	var saved DeviceProfile
	requireLighting(t, json.Unmarshal(disk, &saved))
	if saved.RgbOff {
		t.Fatal("wrong persisted override")
	}
}

func TestConcurrentUltraProfileSaveDeleteAndInputPublication(t *testing.T) {
	d := profileSaveFixture(t, "X")
	readyLighting(d)
	d.startLightingQueue()
	actions := []func(){
		func() { d.SaveUserProfile("copy"); d.DeleteDeviceProfile("copy") },
		func() { d.SaveUserProfile("X"); d.rotateDeviceProfile() },
		func() {
			d.SetLightingEffect("gradient")
			d.ProcessNewGradientColor("gradient")
			d.ProcessDeleteGradientColor("gradient")
		},
		func() {
			d.UpdateLiftHeight(7)
			d.UpdateAngleSnapping(1)
			d.UpdateButtonOptimization(1)
			d.SaveMouseGestures(1, map[int]uint8{0: 20, 1: 30, 2: 40, 3: 50})
		},
		func() {
			d.ProcessSetOpenRgbIntegration(true)
			d.writeColorEx([]byte{1, 2, 3, 4, 5, 6}, 0)
			d.ProcessSetOpenRgbIntegration(false)
			d.ProcessSetRgbCluster(true)
			d.writeColorCluster([]byte{6, 5, 4, 3, 2, 1}, 0)
			d.ProcessSetRgbCluster(false)
		},
		func() {
			d.SchedulerBrightness(0)
			d.SetLightingBrightness(61)
			d.ControlDeviceRgb(true)
			d.ControlDeviceRgb(false)
			d.SchedulerBrightness(1)
		},
		func() {
			d.DPISnapshot()
			d.ButtonsSnapshot()
			d.PerformanceSnapshot()
			d.DeviceProfileSnapshot()
			d.LightingSnapshot()
		},
	}
	var wg sync.WaitGroup
	for _, f := range actions {
		wg.Add(1)
		go func(f func()) {
			defer wg.Done()
			for i := 0; i < 4; i++ {
				f()
			}
		}(f)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	waitDone(t, done)
	d.StopDirty()
}
