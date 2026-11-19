package m65rgbultraWU

import (
	"LumenForge/src/common"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func releaseWU(t *testing.T, ch chan struct{}) func() {
	t.Helper()
	var once sync.Once
	release := func() { once.Do(func() { close(ch) }) }
	t.Cleanup(release)
	return release
}
func TestWUReviewPollingTransition(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	d.PollingRates = map[int]string{4: "1000 Hz", 3: "500 Hz"}
	reports := make(chan []byte, 128)
	entered, gate := make(chan struct{}), make(chan struct{})
	release := releaseWU(t, gate)
	d.lightingReportIO = func(r []byte) ([]byte, error) {
		reports <- append([]byte(nil), r...)
		if r[2] == 1 && r[3] == 1 && r[4] == 0 {
			close(entered)
			<-gate
		}
		return make([]byte, 64), nil
	}
	d.initLeds()
	reportLighting(t, reports) // open RGB endpoint
	mustLighting(t, d.SetLightingEffect("rainbow"))
	first := reportLighting(t, reports)
	if first[2] != 6 {
		t.Fatal("no initial Native RGB", first)
	}
	d.lightingMu.Lock()
	old := d.lightingWorker
	d.lightingMu.Unlock()
	result := make(chan uint8, 1)
	go func() { result <- d.UpdatePollingRate(3) }()
	waitWU(t, entered, "polling command reached USB stub")
	d.lightingMu.Lock()
	paused := !d.lightingHardwareReady && d.lightingWorker == nil
	d.lightingMu.Unlock()
	if !paused {
		t.Fatal("canonical Native output still authorized while polling is programmed")
	}
	doneLighting(t, old.done)
	release()
	if waitWU(t, result, "polling transition completed") != 1 {
		t.Fatal("polling transition failed")
	}
	// Existing command 01 01 00(rate=3), then existing RGB endpoint setup,
	// then current canonical Rainbow. No receiver reset commands are invented.
	for {
		r := reportLighting(t, reports)
		if r[2] == 1 && r[3] == 1 {
			if len(r) != 65 || r[1] != 8 || r[4] != 0 || r[5] != 3 {
				t.Fatal(r)
			}
			break
		}
		if r[2] != 6 {
			t.Fatal("unexpected pre-poll command", r)
		}
	}
	opened := reportLighting(t, reports)
	if opened[2] != 0x0d || opened[3] != 0 || opened[4] != 1 {
		t.Fatal("missing RGB endpoint reopen", opened)
	}
	resumed := reportLighting(t, reports)
	if resumed[1] != 8 || resumed[2] != 6 || resumed[3] != 0 {
		t.Fatal("Native output did not resume", resumed)
	}
	d.lightingMu.Lock()
	restarted := d.lightingWorker
	frame, frameErr := d.lightingFrameLocked()
	d.lightingMu.Unlock()
	mustLighting(t, frameErr)
	if restarted == nil || restarted == old || frame.effect != "rainbow" || frame.profile == nil || frame.profile.ProfileName != "rainbow" {
		t.Fatal("polling did not restore a fresh Rainbow worker/settings snapshot", frame.effect)
	}
	// Worker identity and resolved effect are deterministic. Also distinguish
	// actual animated output from Static, which uses the same command header.
	progression := time.NewTimer(2 * time.Second)
	defer progression.Stop()
	for {
		select {
		case next := <-reports:
			if len(next) != 65 || next[2] != 6 || next[3] != 0 {
				t.Fatal("unexpected resumed Rainbow report", next)
			}
			logo := [3]byte{next[8], next[10], next[12]}
			// WU's full-brightness Rainbow is a fully saturated RGB spectrum.
			low, high := logo[0], logo[0]
			for _, v := range logo[1:] {
				if v < low {
					low = v
				}
				if v > high {
					high = v
				}
			}
			if low != 0 || high != 255 {
				t.Fatal("resumed Logo is outside the source Rainbow spectrum", logo)
			}
			if logo != [3]byte{resumed[8], resumed[10], resumed[12]} {
				goto progressed
			}
		case <-progression.C:
			t.Fatal("resumed Rainbow did not progress beyond its first Logo frame")
		}
	}
progressed:
	if d.Exit || d.DeviceProfile.PollingRate != 3 {
		t.Fatal("polling disabled device or lost preference")
	}
}

func TestWUReviewEndpointOpenRecovers(t *testing.T) {
	d, _, reg := wuFixture(t, true)
	mustLighting(t, d.SetLightingEffectSettings("static", lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 90, Green: 60, Blue: 30}}}))
	reports := make(chan []byte, 16)
	opens := 0
	d.lightingReportIO = func(r []byte) ([]byte, error) {
		reports <- append([]byte(nil), r...)
		if r[2] == 0x0d && r[3] == 0 && r[4] == 1 {
			opens++
			if opens == 1 {
				return make([]byte, 64), errors.New("endpoint response read failed")
			}
		}
		return make([]byte, 64), nil
	}
	d.initLeds()
	if opens != 2 {
		t.Fatalf("one failed endpoint response left instance disabled: attempts=%d", opens)
	}
	d.setDeviceColor()
	d.setupOpenRGBController()
	reportLighting(t, reports)
	reportLighting(t, reports)
	expectWUPacket(t, reportLighting(t, reports), []byte{90, 255, 60, 255, 30, 255})
	_, o := reg.controllers()
	if o == nil || len(o.Colors) != 3 {
		t.Fatal("external registration was not restored")
	}
}

func TestWUReviewOneShotRetries(t *testing.T) {
	for _, selection := range []string{"static", "mouse", "blank"} {
		t.Run(selection, func(t *testing.T) {
			d, _, _ := wuFixture(t, true)
			mustLighting(t, d.SetLightingEffectSettings("static", lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 100, Green: 80, Blue: 60}}}))
			if selection == "mouse" {
				mustLighting(t, d.SetLightingEffect("mouse"))
			}
			if selection == "blank" {
				mustLighting(t, d.setUserRGBOff(true))
			}
			reports := make(chan []byte, 8)
			attempts := 0
			d.lightingReportIO = func(r []byte) ([]byte, error) {
				if r[2] == 6 && r[3] == 0 {
					attempts++
					reports <- append([]byte(nil), r...)
					if attempts == 1 {
						return make([]byte, 64), errors.New("transient color response failure")
					}
				}
				return make([]byte, 64), nil
			}
			d.initLeds()
			d.setDeviceColor()
			d.lightingMu.Lock()
			worker := d.lightingWorker
			d.lightingMu.Unlock()
			first, second := reportLighting(t, reports), reportLighting(t, reports)
			doneLighting(t, worker.done)
			if attempts != 2 || !reflect.DeepEqual(first, second) {
				t.Fatal("one-shot did not retry identical packet", attempts, first, second)
			}
			expected := []byte{100, 255, 80, 255, 60, 255}
			if selection == "mouse" {
				expected = []byte{255, 255, 255, 255, 0, 255}
			}
			if selection == "blank" {
				expected = []byte{0, 255, 0, 255, 0, 255}
			}
			expectWUPacket(t, second, expected)
			noLightingReport(t, reports)
		})
	}
}

func TestWUReviewOneShotFailuresBounded(t *testing.T) {
	for _, full := range []bool{false, true} {
		t.Run(map[bool]string{false: "collect-reports", true: "full-report-buffer"}[full], func(t *testing.T) {
			d, _, _ := wuFixture(t, true)
			reports := make(chan []byte, 8)
			if full {
				for i := 0; i < cap(reports); i++ {
					reports <- nil
				}
			}
			attempts := 0
			d.lightingReportIO = func(r []byte) ([]byte, error) {
				if r[2] == 6 && r[3] == 0 {
					attempts++ // Count every attempt even if its diagnostic report is dropped.
					select {
					case reports <- append([]byte(nil), r...):
					default:
					}
					return make([]byte, 64), errors.New("persistent color failure")
				}
				return make([]byte, 64), nil
			}
			d.initLeds()
			d.setDeviceColor()
			d.lightingMu.Lock()
			worker := d.lightingWorker
			d.lightingMu.Unlock()
			doneLighting(t, worker.done)
			if attempts != lightingOneShotAttempts {
				t.Fatal("unbounded or missing attempts", attempts)
			}
			count := lightingOneShotAttempts
			if full {
				count = cap(reports)
			}
			for i := 0; i < count; i++ {
				reportLighting(t, reports)
			}
			noLightingReport(t, reports)
		})
	}
}

func TestWUReviewPendingRetryCancellation(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(map[bool]string{false: "owner", true: "stop"}[stop], func(t *testing.T) {
			d, _, _ := wuFixture(t, true)
			entered, gate := make(chan struct{}), make(chan struct{})
			release := releaseWU(t, gate)
			calls := 0
			d.lightingBeforeApply = func() {
				calls++
				if calls == 2 {
					close(entered)
					<-gate
				}
			}
			reports := make(chan []byte, 8)
			attempts := 0
			d.lightingReportIO = func(r []byte) ([]byte, error) {
				if r[2] == 6 && r[3] == 0 {
					attempts++
					reports <- append([]byte(nil), r...)
					if attempts == 1 {
						return make([]byte, 64), errors.New("retry first report")
					}
				}
				return make([]byte, 64), nil
			}
			d.initLeds()
			d.setDeviceColor()
			reportLighting(t, reports)
			d.lightingMu.Lock()
			worker := d.lightingWorker
			d.lightingMu.Unlock()
			waitWU(t, entered, "Native retry queued before final USB locks")
			changed := make(chan uint8, 1)
			go func() {
				if stop {
					d.stopCanonicalLighting()
					changed <- 1
				} else {
					changed <- d.ProcessSetOpenRgbIntegration(true)
				}
			}()
			if waitWU(t, changed, "owner/stop operation completed while retry gate remains held") != 1 {
				t.Fatal("owner transition")
			}

			doneLighting(t, worker.done) // Cancellation cannot wait for the blocked pump.
			release()
			if stop {
				doneLighting(t, d.lightingDone)
				noLightingReport(t, reports)
			} else {
				expectWUPacket(t, reportLighting(t, reports), []byte{0, 255, 0, 255, 0, 255}) // new owner's blank, never stale Native retry
			}
		})
	}
}

func TestWUReviewAtomicPreferenceFailureAndReplacement(t *testing.T) {
	for _, shortWrite := range []bool{false, true} {
		t.Run(map[bool]string{false: "write-error", true: "short-write"}[shortWrite], func(t *testing.T) {
			d, _, _ := wuFixture(t, true)
			reports := wuOutput(t, d)
			reportLighting(t, reports)
			path := d.DeviceProfile.Path
			mustLighting(t, os.Chmod(path, 0640))
			original, err := os.ReadFile(path)
			mustLighting(t, err)
			d.lightingMu.Lock()
			worker, owner := d.lightingWorker, d.owner
			d.lightingMu.Unlock()
			d.lightingProfileFileWrite = func(f *os.File, data []byte) (int, error) {
				n, err := f.Write(data[:len(data)/2])
				if err != nil {
					return n, err
				}
				if shortWrite {
					return n, nil
				}
				return n, errors.New("injected temporary write failure")
			}
			if d.setUserRGBOff(true) == nil || d.ProcessSetOpenRgbIntegration(true) != 0 {
				t.Fatal("temporary write failure succeeded")
			}
			after, err := os.ReadFile(path)
			mustLighting(t, err)
			if !bytes.Equal(original, after) || !json.Valid(after) {
				t.Fatal("original profile truncated or altered")
			}
			leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".m65-ultra-wu-preference-*.tmp"))
			mustLighting(t, err)
			if len(leftovers) != 0 {
				t.Fatal("temporary files leaked", leftovers)
			}
			d.lightingMu.Lock()
			unchanged := d.lightingWorker == worker && d.owner == owner && !d.userRGBOff
			d.lightingMu.Unlock()
			if !unchanged || d.DeviceProfile.RgbOff || d.DeviceProfile.OpenRGBIntegration {
				t.Fatal("failed preference published")
			}
			noLightingReport(t, reports)
			d.lightingProfileFileWrite = nil
			mustLighting(t, d.setUserRGBOff(true))
			expectWUPacket(t, reportLighting(t, reports), []byte{0, 255, 0, 255, 0, 255})
			if d.ProcessSetOpenRgbIntegration(true) != 1 {
				t.Fatal("successful atomic ownership replacement failed")
			}
			reportLighting(t, reports)
			saved := DeviceProfile{}
			after, err = os.ReadFile(path)
			mustLighting(t, err)
			mustLighting(t, json.Unmarshal(after, &saved))
			info, err := os.Stat(path)
			mustLighting(t, err)
			if !saved.RgbOff || !saved.OpenRGBIntegration || !d.DeviceProfile.OpenRGBIntegration || info.Mode().Perm() != 0640 {
				t.Fatal("replacement/publish/permissions", saved, info.Mode())
			}
		})
	}
}

func TestWUReviewRGBOffDoesNotResolveCorruptEffect(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	speed := 2.0
	settings := lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "gradient", Speed: &speed, Gradient: &lightingsettings.GradientSettings{Stops: []lightingsettings.GradientStop{{Position: 0, Color: lightingsettings.Color{Red: 12, Green: 34, Blue: 56}, Intensity: 1}, {Position: 1, Color: lightingsettings.Color{Red: 12, Green: 34, Blue: 56}, Intensity: 1}}}}
	mustLighting(t, d.SetLightingEffectSettings("gradient", settings))
	mustLighting(t, d.SetLightingEffect("gradient"))
	d.lightingMu.Lock()
	original := d.lightingCanonical.resolver
	d.lightingCanonical.resolver = corruptWUResolver{original}
	d.lightingMu.Unlock()
	if _, err := d.ResolveLightingEffectSettings("gradient"); err == nil {
		t.Fatal("ordinary corrupt-effect validation weakened")
	}
	reports := make(chan []byte, 16)
	d.lightingReportIO = func(r []byte) ([]byte, error) {
		if r[2] == 6 && r[3] == 0 {
			reports <- append([]byte(nil), r...)
		}
		return make([]byte, 64), nil
	}
	d.initLeds()
	mustLighting(t, d.setUserRGBOff(true))
	expectWUPacket(t, reportLighting(t, reports), []byte{0, 255, 0, 255, 0, 255})
	d.lightingMu.Lock()
	d.lightingCanonical.resolver = original
	d.lightingMu.Unlock()
	mustLighting(t, d.setUserRGBOff(false))
	resumed := reportLighting(t, reports)
	if len(resumed) != 65 {
		t.Fatal("invalid resumed report", resumed)
	}
	// The existing Gradient interpolator truncates float channels to bytes. Even
	// equal stops can round down by one; this is not a transport or blackout bug.
	for i, want := range []byte{12, 34, 56} {
		got := resumed[8+i*2]
		if got > want || got < want-1 {
			t.Fatal("selected Gradient did not resume", resumed)
		}
	}
	expectWUPacket(t, resumed, []byte{resumed[8], 255, resumed[10], 255, resumed[12], 255})
}

func TestWUReviewEndpointRetriesBoundedAndSameInstanceRecovery(t *testing.T) {
	d, _, reg := wuFixture(t, true)
	reports := make(chan []byte, 16)
	opens := 0
	fail := true
	d.lightingReportIO = func(r []byte) ([]byte, error) {
		if r[2] == 0x0d {
			opens++
			if fail {
				return make([]byte, 64), errors.New("endpoint open failed")
			}
		}
		if r[2] == 6 {
			reports <- append([]byte(nil), r...)
		}
		return make([]byte, 64), nil
	}
	d.initLeds()
	d.setDeviceColor()
	d.lightingMu.Lock()
	ready := d.lightingHardwareReady
	d.lightingMu.Unlock()
	c, o := reg.controllers()
	if opens != lightingEndpointAttempts || ready || c != nil || o != nil {
		t.Fatal("unbounded retry or falsely ready registration", opens, ready)
	}
	noLightingReport(t, reports)
	fail = false
	d.initLeds()
	d.setDeviceColor()
	reportLighting(t, reports)
	_, o = reg.controllers()
	if opens != lightingEndpointAttempts+1 || o == nil {
		t.Fatal("same-instance explicit recovery failed", opens)
	}
}

// Milestones are bounded, without timing sleeps. Held stubs/gates use releaseWU
// cleanup so a failure here cannot strand the fixture's shutdown.
func waitWU[T any](t *testing.T, ch <-chan T, milestone string) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", milestone)
	}
	var zero T
	return zero
}
func unlockWU(t *testing.T, mu *sync.Mutex) func() {
	t.Helper()
	var once sync.Once
	unlock := func() { once.Do(mu.Unlock) }
	t.Cleanup(unlock)
	return unlock
}

func TestWUSecondReviewRepeatedOpenRGBEnable(t *testing.T) {
	d, _, reg := wuFixture(t, true)
	reports := wuOutput(t, d)
	reportLighting(t, reports)
	if d.ProcessSetOpenRgbIntegration(true) != 1 {
		t.Fatal("enable OpenRGB")
	}
	reportLighting(t, reports)
	_, controller := reg.controllers()
	controller.WriteColorEx([]byte{21, 42, 63}, 0)
	expectWUPacket(t, reportLighting(t, reports), []byte{21, 255, 42, 255, 63, 255})
	mustLighting(t, d.SetLightingBrightness(90)) // replay confirms the successful Logo cache
	expectWUPacket(t, reportLighting(t, reports), []byte{21, 229, 42, 229, 63, 229})
	d.lightingMu.Lock()
	owner, sequence, logo := d.owner, d.lightingSequence, d.lightingLastLogo
	d.lightingMu.Unlock()
	if d.ProcessSetOpenRgbIntegration(true) != 1 {
		t.Fatal("identical enable failed")
	}
	d.lightingMu.Lock()
	same := d.owner == owner && d.lightingSequence == sequence && d.lightingLastLogo == logo && d.lightingLastLogoOK
	d.lightingMu.Unlock()
	_, after := reg.controllers()
	if !same || after != controller {
		t.Fatal("identical enable retired owner, registrations, or current Logo")
	}
	noLightingReport(t, reports)
	if d.ProcessSetOpenRgbIntegration(false) != 1 {
		t.Fatal("real owner disable")
	}
	reportLighting(t, reports)
	if d.ProcessSetOpenRgbIntegration(true) != 1 {
		t.Fatal("real owner restore")
	}
	reportLighting(t, reports)
	controller.WriteColorEx([]byte{99, 99, 99}, 0)
	noLightingReport(t, reports)
	_, current := reg.controllers()
	current.WriteColorEx([]byte{3, 6, 9}, 0)
	expectWUPacket(t, reportLighting(t, reports), []byte{3, 229, 6, 229, 9, 229})
}

func TestWUSecondReviewPollingProfileOverlap(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	d.PollingRates = map[int]string{3: "500 Hz", 4: "1000 Hz"}
	originalPath := d.DeviceProfile.Path
	alternate := cloneLightingProfile(*d.DeviceProfile)
	alternate.Active = false
	alternate.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-alternate.json")
	mustLighting(t, common.SaveJsonData(alternate.Path, alternate))
	d.loadDeviceProfiles()
	reports := wuOutput(t, d)
	reportLighting(t, reports)
	entered, gate := make(chan struct{}), make(chan struct{})
	release := releaseWU(t, gate)
	d.lightingBeforePolling = func() { close(entered); <-gate }
	result := make(chan uint8, 1)
	go func() { result <- d.UpdatePollingRate(3) }()
	waitWU(t, entered, "polling recovery paused before delay")
	switched := make(chan uint8, 1)
	go func() { switched <- d.ChangeDeviceProfile("alternate") }()
	if waitWU(t, switched, "profile switch completed while polling gate remains held") != 1 {
		t.Fatal("profile switch during polling pause")
	}

	release()
	if waitWU(t, result, "polling completed after profile switch") != 1 {
		t.Fatal("polling failed")
	}
	reportLighting(t, reports)
	readProfile := func(path string) DeviceProfile {
		data, err := os.ReadFile(path)
		mustLighting(t, err)
		var p DeviceProfile
		mustLighting(t, json.Unmarshal(data, &p))
		return p
	}
	original, savedAlternate := readProfile(originalPath), readProfile(alternate.Path)
	if original.PollingRate != 3 || savedAlternate.PollingRate != 4 || d.DeviceProfile.Path != alternate.Path || d.DeviceProfile.PollingRate != 4 {
		t.Fatalf("polling mutated wrong profile: original=%d alternate=%d active=%s/%d", original.PollingRate, savedAlternate.PollingRate, d.DeviceProfile.Path, d.DeviceProfile.PollingRate)
	}
}

func TestWUSecondReviewPollingReadFailureRecovery(t *testing.T) {
	d, _, reg := wuFixture(t, true)
	d.PollingRates = map[int]string{3: "500 Hz", 4: "1000 Hz"}
	reports := make(chan []byte, 16)
	d.lightingReportIO = func(r []byte) ([]byte, error) {
		reports <- append([]byte(nil), r...)
		if r[2] == 1 && r[3] == 1 && r[4] == 0 {
			return make([]byte, 64), errors.New("polling request sent; response read failed")
		}
		return make([]byte, 64), nil
	}
	d.initLeds()
	reportLighting(t, reports)
	d.setDeviceColor()
	reportLighting(t, reports)
	if d.UpdatePollingRate(3) != 0 {
		t.Fatal("polling error hidden")
	}
	poll := reportLighting(t, reports)
	if poll[2] != 1 || poll[3] != 1 {
		t.Fatal("polling request not sent", poll)
	}
	d.lightingMu.Lock()
	ready := d.lightingHardwareReady
	d.lightingMu.Unlock()
	if !ready {
		t.Fatal("polling response failure permanently disabled usable Lighting")
	}
	opened := reportLighting(t, reports)
	if opened[2] != 0x0d || opened[3] != 0 || opened[4] != 1 {
		t.Fatal("endpoint recovery skipped", opened)
	}
	resumed := reportLighting(t, reports)
	if resumed[2] != 6 || resumed[3] != 0 {
		t.Fatal("current Lighting not replayed", resumed)
	}
	_, controller := reg.controllers()
	if controller == nil {
		t.Fatal("external registration not restored")
	}
}

func TestWUSecondReviewExternalBlankRetry(t *testing.T) {
	for _, clusterOwner := range []bool{false, true} {
		for _, supersede := range []bool{false, true} {
			name := map[bool]string{false: "OpenRGB", true: "Cluster"}[clusterOwner] + map[bool]string{false: "/recover", true: "/fresh-frame"}[supersede]
			t.Run(name, func(t *testing.T) {
				d, rt, reg := wuFixture(t, true)
				reports := make(chan []byte, 32)
				blankFailures := make(chan bool, 4)
				var armed atomic.Bool
				var attempts atomic.Int32
				entered, gate := make(chan struct{}), make(chan struct{})
				release := releaseWU(t, gate)
				var block sync.Once
				d.lightingBeforeApply = func() {
					if supersede && armed.Load() && attempts.Load() == 1 {
						block.Do(func() { close(entered); <-gate })
					}
				}
				d.lightingReportIO = func(r []byte) ([]byte, error) {
					if r[2] == 6 && r[3] == 0 {
						blankAttempt := armed.Load() && r[8] == 0 && r[10] == 0 && r[12] == 0
						fail := blankAttempt && attempts.Load() == 0
						if blankAttempt {
							attempts.Add(1)
						}
						// Choose outcome and publish the counter before the observable report.
						reports <- append([]byte(nil), r...)
						if blankAttempt {
							blankFailures <- fail
						}
						if fail {
							return make([]byte, 64), errors.New("one-shot external blank failed")
						}
					}
					return make([]byte, 64), nil
				}
				d.initLeds()
				d.setDeviceColor()
				reportLighting(t, reports)
				if d.setLightingOwnership(clusterOwner, true) != 1 {
					t.Fatal("external owner enable")
				}
				c, o := reg.controllers()
				var send func([]byte, int)
				if clusterOwner {
					send = c.WriteColorEx
				} else {
					send = o.WriteColorEx
					reportLighting(t, reports)
				}
				send([]byte{21, 42, 63}, 0)
				reportLighting(t, reports)
				armed.Store(true)
				mustLighting(t, d.setUserRGBOff(true))
				expectWUPacket(t, reportLighting(t, reports), []byte{0, 255, 0, 255, 0, 255})
				if !waitWU(t, blankFailures, "first external blank attempt returned failure") {
					t.Fatal("first failure was not injected")
				}
				if supersede {
					waitWU(t, entered, "external blank retry reached final-admission barrier")
					callbackWU(t, send, []byte{77, 88, 99}, "new external frame callback returned while blank-retry gate is held")
					release()
					expectWUPacket(t, reportLighting(t, reports), []byte{77, 255, 88, 255, 99, 255})
					if attempts.Load() != 1 {
						t.Fatal("obsolete blank retried after fresh external frame")
					}
				} else {
					expectWUPacket(t, reportLighting(t, reports), []byte{0, 255, 0, 255, 0, 255})
					if waitWU(t, blankFailures, "second external blank attempt returned success") {
						t.Fatal("second attempt unexpectedly failed")
					}
					if attempts.Load() != 2 {
						t.Fatal("external blank did not recover on second attempt", attempts.Load())
					}
				}
				state, _, err := rt.State.Resolve(d.Serial)
				mustLighting(t, err)
				if !d.DeviceProfile.RgbOff || state.SelectedEffect != "static" || state.Brightness != 100 {
					t.Fatal("blank changed persistent desired Lighting", state)
				}
			})
		}
	}
}

func TestWUSecondReviewExternalBlankRetryBound(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	mustLighting(t, d.setUserRGBOff(true))
	if d.ProcessSetOpenRgbIntegration(true) != 1 {
		t.Fatal("external preference")
	}
	var attempts atomic.Int32
	d.lightingReportIO = func(r []byte) ([]byte, error) {
		if r[2] == 6 && r[3] == 0 {
			attempts.Add(1)
			return make([]byte, 64), errors.New("blank remains unavailable")
		}
		return make([]byte, 64), nil
	}
	d.initLeds()
	// Exercise the exact pump request contract with an outcome channel, so retry
	// exhaustion is a bounded milestone, not a negative assertion based on sleeps.
	d.lightingMu.Lock()
	req := &lightingOutput{session: d.owner, blank: true, done: make(chan lightingApplyOutcome, 1)}
	d.startCanonicalOutputLocked()
	d.enqueueLightingLocked(req)
	d.lightingMu.Unlock()
	if waitWU(t, req.done, "external blank exhausted bounded attempts") != lightingRetry || attempts.Load() != 3 {
		t.Fatal("unbounded blank retry", attempts.Load())
	}
}

func TestWUSecondReviewAdmittedTransferDoesNotHoldLightingState(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	reports := make(chan []byte, 8)
	entered, gate := make(chan struct{}), make(chan struct{})
	release := releaseWU(t, gate)
	var first sync.Once
	d.lightingReportIO = func(r []byte) ([]byte, error) {
		if r[2] == 6 && r[3] == 0 {
			reports <- append([]byte(nil), r...)
			first.Do(func() { close(entered); <-gate })
		}
		return make([]byte, 64), nil
	}
	d.initLeds()
	d.setDeviceColor()
	waitWU(t, entered, "already-admitted Native transfer entered USB stub")
	reportLighting(t, reports) // This transfer was admitted before the owner changes.
	changed := make(chan uint8, 1)
	go func() { changed <- d.ProcessSetOpenRgbIntegration(true) }()
	if waitWU(t, changed, "ownership changed while admitted HID read remains blocked") != 1 {
		t.Fatal("owner transition")
	}
	release()
	expectWUPacket(t, reportLighting(t, reports), []byte{0, 255, 0, 255, 0, 255})
}

func TestWURecoveryPreservesOpenRGBControllerSlot(t *testing.T) {
	for _, failRecovery := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "bounded-failure-then-retry"}[failRecovery], func(t *testing.T) {
			d, _, _ := wuFixture(t, true)
			d.PollingRates = map[int]string{3: "500 Hz", 4: "1000 Hz"}
			// A single indexed stub follows the inspected ReconcileDeviceController
			// contract: replace matching serial in place, append new serial, remove nil.
			// This pins WU's membership calls without starting an SDK listener or adding
			// a shared registry API just to expose its private controller slice.
			var registryMu sync.Mutex
			var slots []*common.OpenRGBController
			d.lightingOpenRGBReconcile = func(serial string, c *common.OpenRGBController) {
				registryMu.Lock()
				defer registryMu.Unlock()
				for i, current := range slots {
					if current.Serial == serial {
						if c == nil {
							slots = append(slots[:i], slots[i+1:]...)
						} else {
							slots[i] = c
						}
						return
					}
				}
				if c != nil {
					slots = append(slots, c)
				}
			}
			snapshot := func() []*common.OpenRGBController {
				registryMu.Lock()
				defer registryMu.Unlock()
				return append([]*common.OpenRGBController(nil), slots...)
			}
			var failEndpoint atomic.Bool
			reports := make(chan []byte, 32)
			d.lightingReportIO = func(r []byte) ([]byte, error) {
				if r[2] == 0x0d && failEndpoint.Load() {
					return make([]byte, 64), errors.New("recovery endpoint response unavailable")
				}
				if r[2] == 6 && r[3] == 0 {
					reports <- append([]byte(nil), r...)
				}
				return make([]byte, 64), nil
			}
			d.initLeds()
			d.setDeviceColor()
			reportLighting(t, reports)
			if d.ProcessSetOpenRgbIntegration(true) != 1 {
				t.Fatal("enable OpenRGB")
			}
			reportLighting(t, reports)
			before := snapshot()
			if len(before) != 1 || before[0].Serial != d.Serial {
				t.Fatal("WU did not register at index0", before)
			}
			old := before[0]
			old.WriteColorEx([]byte{11, 22, 33}, 0)
			expectWUPacket(t, reportLighting(t, reports), []byte{11, 255, 22, 255, 33, 255})
			var otherWrites atomic.Int32
			following := &common.OpenRGBController{Serial: "following-device", WriteColorEx: func([]byte, int) { otherWrites.Add(1) }}
			d.lightingOpenRGBReconcile(following.Serial, following)
			entered, gate := make(chan struct{}), make(chan struct{})
			release := releaseWU(t, gate)
			d.lightingBeforePolling = func() { close(entered); <-gate }
			failEndpoint.Store(failRecovery)
			result := make(chan uint8, 1)
			go func() { result <- d.UpdatePollingRate(3) }()
			waitWU(t, entered, "polling recovery retained indexed OpenRGB membership")
			during := snapshot()
			if len(during) != 2 || during[0].Serial != d.Serial || during[1] != following {
				t.Fatal("temporary recovery shifted controller indices", during)
			}
			callbackWU(t, during[0].WriteColorEx, []byte{99, 88, 77}, "unavailable WU callback returned while polling gate is held")
			noLightingReport(t, reports)
			if otherWrites.Load() != 0 {
				t.Fatal("WU index routed to following device")
			}
			release()
			outcome := waitWU(t, result, "indexed WU polling recovery completed")
			after := snapshot()
			if len(after) != 2 || after[0].Serial != d.Serial || after[1] != following {
				t.Fatal("recovery appended duplicate or shifted controller1", after)
			}
			if failRecovery {
				if outcome != 1 {
					t.Fatal("successful polling command reported as failed")
				}
				after[0].WriteColorEx([]byte{98, 87, 76}, 0)
				noLightingReport(t, reports)
				failEndpoint.Store(false)
				d.initLeds() // Explicit same-object recovery restores the same slot.
				after = snapshot()
				if len(after) != 2 || after[0].Serial != d.Serial || after[1] != following {
					t.Fatal("retry shifted slots", after)
				}
			} else if outcome != 1 {
				t.Fatal("polling recovery failed")
			}
			expectWUPacket(t, reportLighting(t, reports), []byte{0, 255, 0, 255, 0, 255}) // existing recovery entry blank
			old.WriteColorEx([]byte{99, 99, 99}, 0)
			noLightingReport(t, reports)
			after[0].WriteColorEx([]byte{44, 55, 66}, 0)
			expectWUPacket(t, reportLighting(t, reports), []byte{44, 255, 55, 255, 66, 255})
			d.stopCanonicalLighting()
			doneLighting(t, d.lightingDone)
			retired := snapshot()
			if len(retired) != 1 || retired[0] != following {
				t.Fatal("permanent removal retained dead WU controller", retired)
			}
			after[0].WriteColorEx([]byte{1, 2, 3}, 0)
			noLightingReport(t, reports)
		})
	}
}

func TestWUCompatibilityExternalIndicatorReplay(t *testing.T) {
	for _, clusterOwner := range []bool{false, true} {
		for _, mode := range []string{"dpi", "brightness", "newer-frame", "owner-switch", "stop", "exhausted"} {
			t.Run(map[bool]string{false: "OpenRGB", true: "Cluster"}[clusterOwner]+"/"+mode, func(t *testing.T) {
				d, rt, reg := wuFixture(t, true)
				mustLighting(t, d.SetLightingEffectSettings("static", lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 100, Green: 80, Blue: 60}}}))
				reports := make(chan []byte, 32)
				failures := make(chan bool, 8)
				var armed atomic.Bool
				var attempts atomic.Int32
				entered, gate := make(chan struct{}), make(chan struct{})
				release := releaseWU(t, gate)
				var block sync.Once
				gated := mode == "newer-frame" || mode == "owner-switch" || mode == "stop"
				d.lightingBeforeApply = func() {
					if gated && armed.Load() && attempts.Load() == 1 {
						block.Do(func() { close(entered); <-gate })
					}
				}
				d.lightingReportIO = func(r []byte) ([]byte, error) {
					if r[2] == 6 && r[3] == 0 {
						replay := armed.Load() && r[8] == 21 && r[10] == 42 && r[12] == 63
						fail := replay && (attempts.Load() == 0 || mode == "exhausted")
						if replay {
							attempts.Add(1)
						}
						reports <- append([]byte(nil), r...)
						if replay {
							failures <- fail
						}
						if fail {
							return make([]byte, 64), errors.New("external indicator replay failed")
						}
					}
					return make([]byte, 64), nil
				}
				d.initLeds()
				d.setDeviceColor()
				reportLighting(t, reports)
				if d.setLightingOwnership(clusterOwner, true) != 1 {
					t.Fatal("enable external owner")
				}
				c, o := reg.controllers()
				var send func([]byte, int)
				if clusterOwner {
					send = c.WriteColorEx
				} else {
					send = o.WriteColorEx
					reportLighting(t, reports)
				}
				send([]byte{21, 42, 63}, 0)
				reportLighting(t, reports)
				armed.Store(true)
				indicator := []byte{127, 127, 127}
				var completion <-chan lightingApplyOutcome
				if mode == "dpi" {
					indicator = []byte{100, 80, 60}
					if d.SaveMouseDpiColors(rgb.Color{}, map[int]rgb.Color{1: {Red: 100, Green: 80, Blue: 60}}) != 1 {
						t.Fatal("indicator color mutation")
					}
				} else if mode == "exhausted" {
					// Observe bounded pump completion directly, without a timing-based negative
					// assertion. Public brightness/DPI replay admission is covered above/below.
					state, _, err := rt.State.Resolve(d.Serial)
					mustLighting(t, err)
					state.Brightness = 50
					mustLighting(t, rt.State.Set(d.Serial, state))
					d.lightingMu.Lock()
					req := &lightingOutput{session: d.owner, replay: true, done: make(chan lightingApplyOutcome, 1)}
					d.enqueueLightingLocked(req)
					completion = req.done
					d.lightingMu.Unlock()
				} else {
					mustLighting(t, d.SetLightingBrightness(50))
				}
				expected := []byte{21, indicator[0], 42, indicator[1], 63, indicator[2]}
				expectWUPacket(t, reportLighting(t, reports), expected)
				if !waitWU(t, failures, "first external indicator replay failed") {
					t.Fatal("first failure not injected")
				}
				switch mode {
				case "dpi", "brightness":
					expectWUPacket(t, reportLighting(t, reports), expected)
					if waitWU(t, failures, "second external indicator replay succeeded") || attempts.Load() != 2 {
						t.Fatal("replay did not recover on second attempt", attempts.Load())
					}
				case "exhausted":
					if waitWU(t, completion, "external indicator replay exhausted its retry budget") != lightingRetry || attempts.Load() != 3 {
						t.Fatal("unbounded replay retry", attempts.Load())
					}
					for i := 0; i < 2; i++ {
						expectWUPacket(t, reportLighting(t, reports), expected)
						if !waitWU(t, failures, "remaining bounded replay attempt failed") {
							t.Fatal("unexpected success")
						}
					}
				default:
					waitWU(t, entered, "external indicator retry reached admission barrier")
					if mode == "newer-frame" {
						callbackWU(t, send, []byte{77, 88, 99}, "new external frame callback returned while replay-retry gate is held")
						release()
						expectWUPacket(t, reportLighting(t, reports), []byte{77, 127, 88, 127, 99, 127})
					} else {
						changed := make(chan uint8, 1)
						go func() {
							if mode == "stop" {
								d.stopCanonicalLighting()
								changed <- 1
							} else {
								changed <- d.setLightingOwnership(clusterOwner, false)
							}
						}()
						if waitWU(t, changed, "owner/stop completed while external replay retry is gated") != 1 {
							t.Fatal("owner/stop failed")
						}
						release()
						if mode == "stop" {
							doneLighting(t, d.lightingDone)
							noLightingReport(t, reports)
						} else {
							expectWUPacket(t, reportLighting(t, reports), []byte{50, 127, 40, 127, 30, 127})
						}
					}
					if attempts.Load() != 1 {
						t.Fatal("stale replay reached USB", attempts.Load())
					}
				}
				if mode != "owner-switch" && mode != "stop" {
					afterC, afterO := reg.controllers()
					if c != afterC || o != afterO {
						t.Fatal("indicator replay replaced controller registration")
					}
				}
			})
		}
	}
}

func TestWUCompatibilityEffectiveSniperColor(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		sniper, missingRegular, valid bool
		indicator                     []byte
	}{
		{"regular", false, false, true, []byte{255, 255, 255}},
		{"Sniper", true, false, true, []byte{255, 255, 0}},
		{"Sniper-with-null-inactive-color", true, true, true, []byte{255, 255, 0}},
		{"regular-with-null-active-color", false, true, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, _, _ := wuFixture(t, true)
			mustLighting(t, d.SetLightingEffect("mouse"))
			if tc.missingRegular {
				p := d.DeviceProfile.Profiles[d.DeviceProfile.Profile]
				p.Color = nil
				d.DeviceProfile.Profiles[d.DeviceProfile.Profile] = p
			}
			d.SniperMode = tc.sniper
			d.lightingMu.Lock()
			d.captureIndicatorLocked()
			valid := d.lightingIndicatorOK
			d.lightingMu.Unlock()
			if valid != tc.valid {
				t.Fatal("inactive regular color prevented effective Sniper validation", valid)
			}
			reports := make(chan []byte, 8)
			d.lightingReportIO = func(r []byte) ([]byte, error) {
				if r[2] == 6 && r[3] == 0 {
					reports <- append([]byte(nil), r...)
				}
				return make([]byte, 64), nil
			}
			d.initLeds()
			d.setDeviceColor()
			if tc.valid {
				expectWUPacket(t, reportLighting(t, reports), []byte{255, tc.indicator[0], 255, tc.indicator[1], 0, tc.indicator[2]})
			} else {
				d.lightingMu.Lock()
				worker := d.lightingWorker
				d.lightingMu.Unlock()
				doneLighting(t, worker.done)
				noLightingReport(t, reports)
			}
		})
	}
}

// Callback results are buffered so a timeout can release the caller's gate in
// cleanup without leaving a later callback completion blocked on its result.
func callbackWU(t *testing.T, write func([]byte, int), data []byte, milestone string) {
	t.Helper()
	returned := make(chan struct{}, 1)
	go func() { write(data, 0); returned <- struct{}{} }()
	waitWU(t, returned, milestone)
}

func TestWUExternalLogoCachePublicationPrecedesQueuedReplay(t *testing.T) {
	d, _, reg := wuFixture(t, true)
	mustLighting(t, d.SetLightingEffectSettings("static", lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "static", SingleColor: &lightingsettings.SingleColorSettings{Color: lightingsettings.Color{Red: 100, Green: 80, Blue: 60}}}))
	reports := make(chan []byte, 16)
	entered, gate := make(chan struct{}), make(chan struct{})
	release := releaseWU(t, gate)
	var block sync.Once
	d.lightingReportIO = func(r []byte) ([]byte, error) {
		if r[2] == 6 && r[3] == 0 {
			reports <- append([]byte(nil), r...)
			if r[8] == 21 && r[10] == 42 && r[12] == 63 {
				// Report receipt is intentionally NOT transfer completion. Hold this
				// successful stub before return, before applyLightingOutput can cache it.
				block.Do(func() { close(entered); <-gate })
			}
		}
		return make([]byte, 64), nil
	}
	d.initLeds()
	d.setDeviceColor()
	reportLighting(t, reports)
	if d.ProcessSetOpenRgbIntegration(true) != 1 {
		t.Fatal("enable OpenRGB")
	}
	reportLighting(t, reports)
	_, controller := reg.controllers()
	callbackWU(t, controller.WriteColorEx, []byte{21, 42, 63}, "external Logo callback returned before its held USB completion")
	expectWUPacket(t, reportLighting(t, reports), []byte{21, 255, 42, 255, 63, 255})
	waitWU(t, entered, "external report published while successful transfer remains held")
	d.lightingMu.Lock()
	published := d.lightingLastLogoOK && d.lightingLastLogo == [3]byte{21, 42, 63}
	d.lightingMu.Unlock()
	if published {
		t.Fatal("fixture failed to hold the transfer before Logo cache publication")
	}
	changed := make(chan error, 1)
	go func() { changed <- d.SetLightingBrightness(50) }()
	mustLighting(t, waitWU(t, changed, "brightness replay queued before external transfer completion"))
	noLightingReport(t, reports) // The one pump cannot execute the replay yet.
	release()
	expectWUPacket(t, reportLighting(t, reports), []byte{21, 127, 42, 127, 63, 127})
	d.lightingMu.Lock()
	published = d.lightingLastLogoOK && d.lightingLastLogo == [3]byte{21, 42, 63}
	d.lightingMu.Unlock()
	if !published {
		t.Fatal("queued replay ran without the completed external Logo cache")
	}
}

// The legacy writer has no barrier seam: model its snapshot window and run
// the actual SaveJsonData/loadDeviceProfiles suffix independently of canonical fixes.
func TestWUGateLegacyRGBOffDPISaveWindow(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	d.DeviceProfile.RgbOff = true
	snapshot := cloneLightingProfile(*d.DeviceProfile)
	d.triggerKeyAssignment(0x20)
	read := func() DeviceProfile {
		t.Helper()
		raw, err := os.ReadFile(d.DeviceProfile.Path)
		mustLighting(t, err)
		var p DeviceProfile
		mustLighting(t, json.Unmarshal(raw, &p))
		return p
	}
	if p := read(); p.Profile != 2 || !p.RgbOff {
		t.Fatal("legacy DPI save", p.Profile, p.RgbOff)
	}
	mustLighting(t, common.SaveJsonData(snapshot.Path, &snapshot))
	d.loadDeviceProfiles()
	if p := read(); p.Profile != 1 || !p.RgbOff || d.DeviceProfile.Profile != 1 {
		t.Fatal("legacy lost-update demonstration", p.Profile, p.RgbOff)
	}
}

func wuGateGradient(keys []int, positions []float64) lightingsettings.EffectSettings {
	speed := 2.0
	g := &lightingsettings.GradientSettings{Keys: append([]int(nil), keys...)}
	for i, position := range positions {
		g.Stops = append(g.Stops, lightingsettings.GradientStop{Position: position, Intensity: 1, Color: lightingsettings.Color{Red: float64(40 + i*40), Green: 160, Blue: 80}})
	}
	return lightingsettings.EffectSettings{SchemaVersion: lightingsettings.SchemaVersion, EffectID: "gradient", Speed: &speed, Gradient: g}
}

func TestWUGateGradientRendererKeys(t *testing.T) {
	for _, keys := range [][]int{{2, 7}, {1, 0}, {2, 7, 19}} {
		t.Run(fmt.Sprint(keys), func(t *testing.T) {
			d, _, _ := wuFixture(t, true)
			positions := []float64{0, 1}
			if len(keys) == 3 {
				positions = []float64{0, .5, 1}
			}
			value := wuGateGradient(keys, positions)
			mustLighting(t, d.SetLightingEffectSettings("gradient", value))
			if len(keys) == 3 {
				status, key := d.ProcessDeleteGradientColor("gradient")
				if status != 1 || key != 19 {
					t.Fatal("delete sparse key", status, key)
				}
				value.Gradient.Keys = value.Gradient.Keys[:2]
				value.Gradient.Stops = value.Gradient.Stops[:2]
			}
			mustLighting(t, d.SetLightingEffect("gradient"))
			frame := wuFrame(t, d)
			for i, stop := range value.Gradient.Stops {
				if c, ok := frame.profile.Gradients[i]; !ok || c.Position != stop.Position || lightingColor(c) != stop.Color {
					t.Fatal("renderer did not receive dense authored order", frame.profile.Gradients)
				}
			}
			if data := wuRender(t, d); len(data) != 3 || data[1] < 159 || data[1] > 160 || data[2] < 79 || data[2] > 80 {
				t.Fatal("Gradient produced no expected color", data)
			}
			got, err := d.ResolveLightingEffectSettings("gradient")
			mustLighting(t, err)
			if !reflect.DeepEqual(got, value) {
				t.Fatal("rendering changed canonical identity", got, value)
			}
			editor := d.GetRgbProfile("gradient")
			for i, key := range value.Gradient.Keys {
				if lightingColor(editor.Gradients[key]) != value.Gradient.Stops[i].Color {
					t.Fatal("editor lost authored key", key)
				}
			}
		})
	}
}

func TestWUGateGradientDeletePreservesOrder(t *testing.T) {
	d, _, _ := wuFixture(t, true)
	value := wuGateGradient([]int{1, 0, 2, 3}, []float64{0, 0, .5, 1})
	mustLighting(t, d.SetLightingEffectSettings("gradient", value))
	mustLighting(t, d.SetLightingEffect("gradient"))
	assert := func(want lightingsettings.EffectSettings) {
		t.Helper()
		got, err := d.ResolveLightingEffectSettings("gradient")
		mustLighting(t, err)
		if !reflect.DeepEqual(got, want) {
			t.Fatal("canonical Gradient order changed", got, want)
		}
		frame := wuFrame(t, d)
		for i, stop := range want.Gradient.Stops {
			if lightingColor(frame.profile.Gradients[i]) != stop.Color {
				t.Fatal("renderer reordered tied authored stops", frame.profile.Gradients)
			}
		}
		store, err := lightingsettings.LoadDeviceStore(filepath.Join(pwd, "effects.json"))
		mustLighting(t, err)
		loaded, found, err := store.Get(d.Serial, "gradient")
		mustLighting(t, err)
		if !found || !reflect.DeepEqual(loaded, want) {
			t.Fatal("reloaded Gradient order changed", loaded, want)
		}
	}
	assert(value)
	status, key := d.ProcessDeleteGradientColor("gradient")
	if status != 1 || key != 3 {
		t.Fatal(status, key)
	}
	value.Gradient.Keys = value.Gradient.Keys[:3]
	value.Gradient.Stops = value.Gradient.Stops[:3]
	assert(value)
	status, key = d.ProcessDeleteGradientColor("gradient")
	if status != 1 || key != 2 {
		t.Fatal(status, key)
	}
	value.Gradient.Keys = value.Gradient.Keys[:2]
	value.Gradient.Stops = value.Gradient.Stops[:2]
	assert(value)
	if status, _ := d.ProcessDeleteGradientColor("gradient"); status != 2 {
		t.Fatal("minimum two stops not enforced", status)
	}
	assert(value)
}
