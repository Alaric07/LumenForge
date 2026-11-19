package m65rgbultraW

import (
	"LumenForge/src/rgb"
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIndicatorRefreshChoosesLatestLogoAfterSerialization(t *testing.T) {
	for _, owner := range []string{"cluster", "openrgb"} {
		t.Run(owner, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			if owner == "cluster" {
				d.ProcessSetRgbCluster(true)
			} else {
				d.ProcessSetOpenRgbIntegration(true)
			}
			readyLighting(d)
			frames := make(chan []byte, 8)
			inflight, release, refreshing := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var blocked, refreshIntent atomic.Bool
			var once sync.Once
			defer once.Do(func() { close(release) })
			d.lightingWrite = func(p []byte) {
				sendWithin(t, frames, p)
				if p[0] == 20 && !blocked.Swap(true) {
					close(inflight)
					waitDone(t, release)
				}
			}
			d.lightingBeforeApply = func(source string) {
				if source == owner && refreshIntent.CompareAndSwap(true, false) {
					close(refreshing)
				}
			}
			d.lightingMu.Lock()
			g, connection := d.externalGeneration, d.connectionGeneration
			d.lightingMu.Unlock()
			if d.applyLightingPacket(composeLightingPacket([]byte{1, 2, 3}), owner, g, nil, connection) != lightingApplied {
				t.Fatal("A")
			}
			waitFrame(t, frames)
			done := make(chan struct{})
			go func() {
				d.applyLightingPacket(composeLightingPacket([]byte{20, 40, 60}), owner, g, nil, connection)
				close(done)
			}()
			waitDone(t, inflight)
			refreshIntent.Store(true)
			updated := make(chan uint8, 1)
			go func() {
				sendWithin(t, updated, d.SaveMouseDpiColors(rgb.Color{}, map[int]rgb.Color{1: {Red: 90, Green: 80, Blue: 70}}))
			}()
			waitDone(t, refreshing)
			once.Do(func() { close(release) })
			waitDone(t, done)
			if receiveWithin(t, updated) != 1 {
				t.Fatal("indicator save")
			}
			// B was composed with the previous white indicator before the refresh intent.
			if p := waitFrame(t, frames); !bytes.Equal(p, []byte{20, 255, 40, 255, 60, 255}) {
				t.Fatal("inflight B", p)
			}
			expected := []byte{20, 90, 40, 80, 60, 70}
			if p := waitFrame(t, frames); !bytes.Equal(p, expected) {
				t.Fatal("refresh lost B or new indicator", p)
			}
			d.lightingMu.Lock()
			controller, logo := d.externalFrame, d.logoVersion
			d.lightingMu.Unlock()
			if controller != 2 || logo != 2 {
				t.Fatal("refresh advanced authored counters", controller, logo)
			}
			d.refreshIndicatorLighting()
			if p := waitFrame(t, frames); !bytes.Equal(p, expected) {
				t.Fatal("ordinary refresh", p)
			}
			d.lightingMu.Lock()
			unchanged := d.externalFrame == controller && d.logoVersion == logo
			d.lightingMu.Unlock()
			if !unchanged {
				t.Fatal("ordinary refresh advanced counters")
			}
		})
	}
}

func TestSchedulerRefreshesExternalIndicatorWithoutChangingLogo(t *testing.T) {
	for _, owner := range []string{"cluster", "openrgb"} {
		t.Run(owner, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			requireLighting(t, d.SetLightingBrightness(70))
			if owner == "cluster" {
				d.ProcessSetRgbCluster(true)
			} else {
				d.ProcessSetOpenRgbIntegration(true)
			}
			readyLighting(d)
			frames := make(chan []byte, 8)
			d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
			d.lightingMu.Lock()
			g, connection := d.externalGeneration, d.connectionGeneration
			d.lightingMu.Unlock()
			if d.applyLightingPacket(composeLightingPacket([]byte{21, 43, 65}), owner, g, nil, connection) != lightingApplied {
				t.Fatal("cache")
			}
			waitFrame(t, frames)
			d.lightingMu.Lock()
			controller, logo := d.externalFrame, d.logoVersion
			d.lightingMu.Unlock()
			before := inputJSON(t, d)
			if d.SchedulerBrightness(0) != 1 {
				t.Fatal("dark")
			}
			if p := waitFrame(t, frames); !bytes.Equal(p, []byte{21, 0, 43, 0, 65, 0}) {
				t.Fatal("scheduler dark did not redraw indicator", p)
			}
			if d.SchedulerBrightness(1) != 1 {
				t.Fatal("restore")
			}
			if p := waitFrame(t, frames); !bytes.Equal(p, []byte{21, 178, 43, 178, 65, 178}) {
				t.Fatal("scheduler restore", p)
			}
			d.lightingMu.Lock()
			unchanged := d.externalFrame == controller && d.logoVersion == logo && d.externalGeneration == g
			d.lightingMu.Unlock()
			if !unchanged || !bytes.Equal(before, inputJSON(t, d)) {
				t.Fatal("scheduler stole ownership or persisted override")
			}
			// RGB-off establishes black Logo cache; scheduler must use that newer state.
			requireLighting(t, d.setUserRGBOff(true))
			waitFrame(t, frames)
			d.SchedulerBrightness(0)
			if p := waitFrame(t, frames); !bytes.Equal(p, make([]byte, 6)) {
				t.Fatal("scheduler resurrected Logo after RGB-off", p)
			}
		})
	}
}

func TestTemperatureFramesReuseLifecycleSample(t *testing.T) {
	for _, effect := range []string{"cpu-temperature", "gpu-temperature"} {
		t.Run(effect, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			ticks := make(chan time.Time, 2)
			d.lightingTemperatureTicks = ticks
			var calls, sample atomic.Int32
			sample.Store(20)
			probed := make(chan float32, 4)
			d.lightingTemperatureProbe = func(ctx context.Context, got string) float32 {
				if got != effect {
					t.Error("wrong probe", got)
				}
				calls.Add(1)
				v := float32(sample.Load())
				sendWithin(t, probed, v)
				return v
			}
			requireLighting(t, d.SetLightingEffect(effect))
			readyLighting(d)
			frames := make(chan []byte, 32)
			d.lightingWrite = func(p []byte) { sendWithin(t, frames, p) }
			d.setDeviceColor()
			receiveWithin(t, probed)
			waitCondition(t, func() bool {
				d.lightingMu.Lock()
				defer d.lightingMu.Unlock()
				if effect == "cpu-temperature" {
					return d.CpuTemp == 20
				}
				return d.GpuTemp == 20
			})
			for i := 0; i < 5; i++ {
				waitFrame(t, frames)
			}
			if calls.Load() != 1 {
				t.Fatal("renderer probes every frame", calls.Load())
			}
			initial := frameState(t, d)
			start := time.Now()
			low := renderLightingFrame(initial, rendererOwner(), &start)
			sample.Store(90)
			ticks <- time.Now()
			receiveWithin(t, probed)
			waitCondition(t, func() bool {
				d.lightingMu.Lock()
				defer d.lightingMu.Unlock()
				if effect == "cpu-temperature" {
					return d.CpuTemp == 90
				}
				return d.GpuTemp == 90
			})
			updated := frameState(t, d)
			high := renderLightingFrame(updated, rendererOwner(), &start)
			if bytes.Equal(low, high) {
				t.Fatal("sample did not change temperature result", low, high)
			}
			seen := false
			deadline := time.NewTimer(time.Second)
		waitForSample:
			for {
				select {
				case p := <-frames:
					if bytes.Equal([]byte{p[0], p[2], p[4]}, high) {
						seen = true
						break waitForSample
					}
				case <-deadline.C:
					break waitForSample
				}
			}
			deadline.Stop()
			if !seen || calls.Load() != 2 {
				t.Fatal("renderer did not reuse updated sample", calls.Load())
			}
			// The shared probes' unavailable value is 0, retaining the low-color fallback.
			sample.Store(0)
			ticks <- time.Now()
			receiveWithin(t, probed)
			waitCondition(t, func() bool {
				d.lightingMu.Lock()
				defer d.lightingMu.Unlock()
				if effect == "cpu-temperature" {
					return d.CpuTemp == 0
				}
				return d.GpuTemp == 0
			})
			fallback := renderLightingFrame(frameState(t, d), rendererOwner(), &start)
			if !bytes.Equal(fallback, low) {
				t.Fatal("unavailable fallback changed", fallback, low)
			}
			d.lightingMu.Lock()
			done := d.lightingTemperatureDone
			d.lightingMu.Unlock()
			d.StopDirty()
			waitDone(t, done)
		})
	}
}
func TestTemperatureSamplerStopCancelsInFlightProbe(t *testing.T) {
	d, _ := lightingFixture(t, true)
	entered, cancelled := make(chan struct{}), make(chan struct{})
	d.lightingTemperatureProbe = func(ctx context.Context, _ string) float32 { close(entered); <-ctx.Done(); close(cancelled); return 99 }
	requireLighting(t, d.SetLightingEffect("gpu-temperature"))
	readyLighting(d)
	d.setDeviceColor()
	waitDone(t, entered)
	d.lightingMu.Lock()
	done := d.lightingTemperatureDone
	d.lightingMu.Unlock()
	d.StopDirty()
	waitDone(t, cancelled)
	waitDone(t, done)
	d.lightingMu.Lock()
	late := d.GpuTemp
	d.lightingMu.Unlock()
	if late != 0 {
		t.Fatal("retired sampler published late value", late)
	}
}
