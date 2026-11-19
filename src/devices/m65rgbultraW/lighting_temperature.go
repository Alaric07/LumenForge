package m65rgbultraW

import (
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/temperatures"
	"context"
	"os/exec"
	"strconv"
	"time"
)

// This package has telemetry fields but no legacy periodic updater. Start one
// sampler lazily for canonical temperature rendering; ordinary frames only read
// CpuTemp/GpuTemp. A single sequential loop caps sampling at one query per second.
func (d *Device) startLightingTemperatureSamplerLocked() {
	if d.Exit || d.lightingTemperatureCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.lightingTemperatureCancel = cancel
	done := make(chan struct{})
	d.lightingTemperatureDone = done
	probe := d.lightingTemperatureProbe
	if probe == nil {
		probe = sampleLightingTemperature
	}
	ticks := d.lightingTemperatureTicks
	go func() {
		defer close(done)
		if ticks == nil {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			ticks = ticker.C
		}
		for {
			if ctx.Err() != nil {
				return
			}
			d.lightingMu.Lock()
			effect := ""
			if d.lightingOutputReadyLocked() && !d.clusterOwned && !d.openRGBOwned && !d.userRGBOff {
				state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
				if err == nil {
					effect = state.SelectedEffect
				}
			}
			d.lightingMu.Unlock()
			if effect == "cpu-temperature" || effect == "gpu-temperature" {
				sampleCtx, stop := context.WithTimeout(ctx, time.Second)
				value := probe(sampleCtx, effect)
				stop()
				if ctx.Err() != nil {
					return
				}
				d.lightingMu.Lock()
				if !d.Exit {
					if effect == "cpu-temperature" {
						d.CpuTemp = value
					} else {
						d.GpuTemp = value
					}
				}
				d.lightingMu.Unlock()
			}
			select {
			case <-ctx.Done():
				return
			case <-ticks:
			}
		}
	}()
}

// Reuse the established CPU/AMD probes and unavailable=0 behavior. NVIDIA's
// existing public probe cannot be cancelled, so issue its identical query here
// with the sampler's bounded context rather than leaving a process on stop.
func sampleLightingTemperature(ctx context.Context, effect string) float32 {
	if ctx.Err() != nil {
		return 0
	}
	if effect == "cpu-temperature" {
		return temperatures.GetCpuTemperature()
	}
	return sampleLightingGPU(ctx, config.GetConfig().DefaultNvidiaGPU, queryLightingNVIDIA, temperatures.GetAMDGpuTemperature)
}

// Reserve most of the one-second sample budget for fallback and publication.
const lightingNVIDIATimeout = 250 * time.Millisecond

func queryLightingNVIDIA(ctx context.Context, index int) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "nvidia-smi", "-i", strconv.Itoa(index), "--query-gpu=temperature.gpu", "--format=csv,noheader,nounits")
	cmd.WaitDelay = 100 * time.Millisecond
	return cmd.Output()
}

func sampleLightingGPU(ctx context.Context, index int, nvidia func(context.Context, int) ([]byte, error), amd func() float32) float32 {
	if ctx.Err() != nil {
		return 0
	}
	if index != -1 {
		child, cancel := context.WithTimeout(ctx, lightingNVIDIATimeout)
		output, err := nvidia(child, index)
		cancel()
		// A child timeout may use AMD; parent cancellation may not start more I/O.
		if ctx.Err() != nil {
			return 0
		}
		if err == nil {
			if value, err := common.AtoiTrim(string(output)); err == nil && value != 0 {
				return float32(value)
			}
		}
	}
	if ctx.Err() != nil {
		return 0
	}
	return amd()
}
