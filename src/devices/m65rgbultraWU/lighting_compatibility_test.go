package m65rgbultraWU

import (
	"LumenForge/src/cluster"
	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func readWUCompatibilityProfile(t *testing.T, path string) DeviceProfile {
	t.Helper()
	raw, err := os.ReadFile(path)
	mustLighting(t, err)
	var p DeviceProfile
	mustLighting(t, json.Unmarshal(raw, &p))
	return p
}

func TestWUCompatibilityPreferenceDPIOverlap(t *testing.T) {
	for _, ownership := range []bool{true, false} {
		t.Run(map[bool]string{true: "actual-OpenRGB-toggle", false: "RGB-off"}[ownership], func(t *testing.T) {
			d, _, _ := wuFixture(t, true)
			path := d.DeviceProfile.Path
			entered, gate := make(chan struct{}), make(chan struct{})
			release := releaseWU(t, gate)
			d.lightingProfileFileWrite = func(f *os.File, data []byte) (int, error) {
				close(entered)
				<-gate
				return f.Write(data)
			}
			saved := make(chan bool, 1)
			go func() {
				if ownership {
					saved <- d.ProcessSetOpenRgbIntegration(true) == 1
				} else {
					saved <- d.setUserRGBOff(true) == nil
				}
			}()
			waitWU(t, entered, "preference snapshot held before replacement")
			attempted, done := make(chan struct{}, 1), make(chan struct{}, 1)
			go func() { attempted <- struct{}{}; d.triggerKeyAssignment(0x20); done <- struct{}{} }()
			waitWU(t, attempted, "physical DPI+ handling launched during preference save")
			// Do not wait for DPI persistence while holding the preference gate: the
			// corrected path serializes it behind this transaction.
			release()
			if !waitWU(t, saved, "preference committed") {
				t.Fatal("preference failed")
			}
			waitWU(t, done, "DPI+ save and USB programming completed")
			check := func(p DeviceProfile) {
				t.Helper()
				if p.Profile != 2 || (ownership && !p.OpenRGBIntegration) || (!ownership && !p.RgbOff) {
					t.Fatal("lost stage or Lighting preference", p.Profile, p.OpenRGBIntegration, p.RgbOff)
				}
			}
			check(*d.DeviceProfile)
			check(readWUCompatibilityProfile(t, path))
			d.loadDeviceProfiles()
			check(*d.DeviceProfile)
		})
	}
}

func TestWUCompatibilityPollingSuccessLightingFailure(t *testing.T) {
	d, _, reg := wuFixture(t, true)
	d.PollingRates = map[int]string{3: "500 Hz"}
	polls, opens := 0, 0
	d.lightingReportIO = func(r []byte) ([]byte, error) {
		if r[2] == 1 && r[3] == 1 {
			polls++
		}
		if r[2] == 0x0d {
			opens++
			return nil, errors.New("endpoint unavailable")
		}
		return make([]byte, 64), nil
	}
	if d.UpdatePollingRate(3) != 1 || polls != 1 || opens != lightingEndpointAttempts {
		t.Fatal("polling/recovery result", polls, opens)
	}
	d.lightingMu.Lock()
	ready := d.lightingHardwareReady
	d.lightingMu.Unlock()
	_, controller := reg.controllers()
	if ready || controller != nil {
		t.Fatal("failed initial Lighting recovery advertised usable output")
	}
}

func TestWUCompatibilityStartupRecoveryByEffectSelection(t *testing.T) {
	d, _, reg := wuFixture(t, true)
	attempts := 0
	fail := true
	reports := make(chan []byte, 16)
	d.lightingReportIO = func(r []byte) ([]byte, error) {
		if r[2] == 0x0d {
			attempts++
			if fail {
				return nil, errors.New("startup endpoint unavailable")
			}
		}
		if r[2] == 6 && r[3] == 0 {
			select {
			case reports <- append([]byte(nil), r...):
			default:
			}
		}
		return make([]byte, 64), nil
	}
	d.initLeds()
	if attempts != lightingEndpointAttempts {
		t.Fatal("unbounded/missing startup attempts", attempts)
	}
	_, controller := reg.controllers()
	if controller != nil {
		t.Fatal("unusable startup controller advertised")
	}
	// A failed explicit retry remains bounded and reports a recovery error.
	if err := d.SetLightingEffect("static"); err == nil || attempts != 2*lightingEndpointAttempts {
		t.Fatal("failed mutation recovery hidden or unbounded", err, attempts)
	}
	_, controller = reg.controllers()
	if controller != nil {
		t.Fatal("failed mutation recovery advertised hardware")
	}
	// A normal per-device effect-selection route retries the same USB object.
	fail = false
	mustLighting(t, d.SetLightingEffect("static"))
	reportLighting(t, reports)
	_, controller = reg.controllers()
	if attempts != 2*lightingEndpointAttempts+1 || controller == nil {
		t.Fatal("same-instance recovery did not restore output", attempts)
	}
	d.lightingMu.Lock()
	ready := d.lightingHardwareReady
	d.lightingMu.Unlock()
	if !ready {
		t.Fatal("recovery not published")
	}
	mustLighting(t, d.SetLightingEffect("static"))
	if attempts != 2*lightingEndpointAttempts+1 {
		t.Fatal("healthy mutation reopened endpoint")
	}
}

func TestWUCompatibilityInheritedRGBOffClearedByEffectSelection(t *testing.T) {
	d, rt, _ := wuFixture(t, false)
	d.DeviceProfile.RgbOff = true
	d.saveDeviceProfile()
	d.loadDeviceProfiles()
	mustLighting(t, d.attachLightingSource(rt))
	reports := wuOutput(t, d)
	expectWUPacket(t, reportLighting(t, reports), []byte{0, 255, 0, 255, 0, 255})
	mustLighting(t, d.SetLightingEffect("static"))
	expectWUPacket(t, reportLighting(t, reports), []byte{100, 255, 80, 255, 60, 255})
	p := readWUCompatibilityProfile(t, d.DeviceProfile.Path)
	if p.RgbOff {
		t.Fatal("resume did not persist per-profile RGB-off clearing")
	}
	d.loadDeviceProfiles()
	if d.DeviceProfile.RgbOff {
		t.Fatal("RGB-off restored on reload")
	}
}

func TestWUCompatibilityGradientAddPreservesTies(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	value := wuGateGradient([]int{1, 0, 2}, []float64{0, 0, 1})
	mustLighting(t, d.SetLightingEffectSettings("gradient", value))
	check := func(want lightingsettings.EffectSettings) {
		t.Helper()
		got, err := d.ResolveLightingEffectSettings("gradient")
		mustLighting(t, err)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("Gradient add changed authored ordering", got, want)
		}
		store, err := lightingsettings.LoadDeviceStore(filepath.Join(pwd, "effects.json"))
		mustLighting(t, err)
		loaded, found, err := store.Get(d.Serial, "gradient")
		mustLighting(t, err)
		if !found || !reflect.DeepEqual(loaded, want) {
			t.Fatal("Gradient reload changed authored ordering", loaded, want)
		}
	}
	check(value)
	status, key := d.ProcessNewGradientColor("gradient")
	if status != 1 || key != 3 {
		t.Fatal("new authored key", status, key)
	}
	value.Gradient.Keys = []int{1, 0, 3, 2}
	stops := value.Gradient.Stops
	value.Gradient.Stops = []lightingsettings.GradientStop{stops[0], stops[1], {Position: .5, Color: lightingsettings.Color{Green: 255, Blue: 255}}, stops[2]}
	check(value)
}

func TestWUCompatibilityRealClusterDistributor(t *testing.T) {
	d, _, reg := wuFixture(t, true)
	reports := wuOutput(t, d)
	reportLighting(t, reports)
	mustLighting(t, d.SetLightingBrightness(50))
	reportLighting(t, reports)
	if d.ProcessSetRgbCluster(true) != 1 {
		t.Fatal("enable Cluster")
	}
	controller, _ := reg.controllers()
	if controller == nil || controller.LedChannels != 1 {
		t.Fatal("WU must advertise one authored Logo")
	}
	root := t.TempDir()
	restore := config.UsePathsForTest(config.Paths{ShippedDatabaseRoot: "../../../database", ClusterEffectSettingsFile: filepath.Join(root, "effects.json"), RGBClusterLightingStateFile: filepath.Join(root, "state.json"), RGBClusterLayoutFile: filepath.Join(root, "layout.json")})
	t.Cleanup(restore)
	distributor := cluster.Init()
	t.Cleanup(distributor.Stop)
	mustLighting(t, distributor.SetLightingEffect("static"))
	mustLighting(t, distributor.SetLightingSingleColor("static", lightingsettings.Color{Red: 21, Green: 42, Blue: 63}))
	lengths := make(chan int, 1)
	original := controller.WriteColorEx
	var once sync.Once
	copyController := *controller
	copyController.WriteColorEx = func(data []byte, channel int) { once.Do(func() { lengths <- len(data) }); original(data, channel) }
	distributor.AddDeviceController(&copyController)
	if got := waitWU(t, lengths, "real distributor callback"); got != int(controller.LedChannels)*3 {
		t.Fatal("advertised Cluster frame length", got)
	}
	expectWUPacket(t, reportLighting(t, reports), []byte{21, 127, 42, 127, 63, 127})
	distributor.Stop()
}
