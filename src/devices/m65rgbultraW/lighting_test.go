package m65rgbultraW

import (
	"LumenForge/src/common"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var auditedEffects = []string{"colorpulse", "colorshift", "colorwarp", "cpu-temperature", "flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature", "gradient", "mouse", "off", "rainbow", "pastelrainbow", "rotator", "static", "storm", "watercolor", "wave"}

func requireLighting(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func lightingFixture(t *testing.T, attach bool) (*Device, *lightingsettings.IndependentDeviceRuntime) {
	t.Helper()
	logger.Init()
	root := t.TempDir()
	previous := pwd
	pwd = root
	t.Cleanup(func() { pwd = previous })
	rt, err := lightingsettings.LoadIndependentDeviceRuntime(filepath.Join(root, "state.json"), filepath.Join(root, "effects.json"), "../../../database/rgb.json")
	requireLighting(t, err)
	brightness := uint8(17)
	stages := map[int]DPIProfile{}
	colors := []rgb.Color{{Red: 255}, {Red: 255, Green: 255, Blue: 255}, {Green: 255}, {Red: 255, Blue: 255}, {Green: 255, Blue: 255}, {Red: 255, Green: 255}}
	for id, v := range []uint16{400, 800, 1200, 1600, 3000, 200} {
		c := colors[id]
		name := []string{"Stage 1", "Stage 2", "Stage 3", "Stage 4", "Stage 5", "Sniper"}[id]
		stages[id] = DPIProfile{Name: name, Value: v, PackerIndex: []int{1, 2, 3, 4, 5, 6}[id], Color: &c, Sniper: id == 5, ColorIndex: map[int][]int{0: {1, 3, 5}}}
	}
	p := &DeviceProfile{Serial: "m65-ultra-test", Product: "M65 RGB ULTRA", Path: filepath.Join(root, "profile.json"), Active: true, RGBProfile: "gradient", BrightnessSlider: &brightness, OriginalBrightness: 82, Profile: 1, PollingRate: 4, LiftHeight: 2, Tilts: map[int]TiltOption{0: {Name: "Left Tilt", Value: 20}, 1: {Name: "Right Tilt", Value: 20}, 2: {Name: "Forward Tilt", Value: 20}, 3: {Name: "Backward Tilt", Value: 20}}, SleepMode: 15, Profiles: stages, ZoneColors: map[int]ZoneColors{0: {Name: "obsolete", Color: &rgb.Color{Red: 9}, ColorIndex: []int{99}}}}
	d := &Device{Serial: p.Serial, Product: p.Product, VendorId: 6940, ProductId: 7093, Endpoint: 9, dev: &common.Slipstream{}, Connected: true, LEDChannels: 2, ChangeableLedChannels: 2, ZoneAmount: 1, DPIAmount: 6, MinDPI: 100, MaxDPI: 26000, DeviceProfile: p, UserProfiles: map[string]*DeviceProfile{"default": p}, Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{"static": {StartColor: rgb.Color{Red: 9}}}}, lightingHardwareReady: false, LiftHeights: map[int]string{2: "Low", 3: "Medium", 4: "High", 6: "Calibrated", 7: "Ultra Low"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz", 2: "250 Hz", 3: "500 Hz", 4: "1000 Hz"}, SwitchModes: map[int]string{0: "Disabled", 1: "Enabled"}}
	d.KeyAssignment = defaultWirelessAssignments()
	p.KeyAssignmentHash = "fixtureassign"
	requireLighting(t, os.MkdirAll(filepath.Join(root, "database", "key-assignments"), 0700))
	assignmentData, err := json.Marshal(d.KeyAssignment)
	requireLighting(t, err)
	requireLighting(t, os.WriteFile(filepath.Join(root, "database", "key-assignments", p.KeyAssignmentHash+".json"), assignmentData, 0600))
	d.lightingClusterRegister = func(*common.ClusterController) {}
	d.lightingClusterRemove = func(string) {}
	d.lightingOpenRGBReconcile = func(string, *common.OpenRGBController) {}
	d.lightingDPIWrite = func([]byte, []byte) error { return nil }
	d.lightingWrite = func([]byte) {}
	d.lightingTemperatureProbe = func(context.Context, string) float32 { return 0 }
	data, _ := json.Marshal(p)
	requireLighting(t, os.WriteFile(p.Path, data, 0600))
	if attach {
		requireLighting(t, d.attachLightingSource(rt))
	}
	t.Cleanup(func() {
		d.lightingMu.Lock()
		done := d.externalDone
		temperatureDone := d.lightingTemperatureDone
		d.lightingMu.Unlock()
		d.stopCanonicalLighting(false)
		if temperatureDone != nil {
			waitDone(t, temperatureDone)
		}
		if done != nil {
			waitDone(t, done)
		}
	})
	return d, rt
}
func frameState(t *testing.T, d *Device) lightingFrameState {
	t.Helper()
	d.lightingMu.Lock()
	v, e := d.lightingFrameLocked()
	d.lightingMu.Unlock()
	requireLighting(t, e)
	return v
}
func rendererOwner() *rgb.ActiveRGB {
	o := rgb.Exit()
	o.RGBStartColor = &rgb.Color{Red: 100, Green: 80, Blue: 60}
	o.RGBEndColor = &rgb.Color{Red: 40, Green: 20, Blue: 10}
	return o
}
func render(t *testing.T, d *Device) []byte {
	t.Helper()
	start := time.Now()
	return renderLightingFrame(frameState(t, d), rendererOwner(), &start)
}
func waitDone(t *testing.T, c <-chan struct{}) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out")
	}
}
func waitFrame(t *testing.T, c <-chan []byte) []byte {
	t.Helper()
	select {
	case p := <-c:
		return p
	case <-time.After(3 * time.Second):
		t.Fatal("no frame")
		return nil
	}
}
func noFrame(t *testing.T, c <-chan []byte) {
	t.Helper()
	select {
	case p := <-c:
		t.Fatalf("unexpected output %v", p)
	case <-time.After(60 * time.Millisecond):
	}
}
func inputJSON(t *testing.T, d *Device) []byte {
	t.Helper()
	d.lightingMu.Lock()
	b, e := json.Marshal(d.DeviceProfile)
	d.lightingMu.Unlock()
	requireLighting(t, e)
	return b
}

func readyLighting(d *Device) {
	d.lightingMu.Lock()
	d.Connected = true
	d.lightingHardwareReady = true
	d.lightingMu.Unlock()
}
