package scheduler

import "testing"

func TestReconcileScheduledBrightnessReappliesEffectiveState(t *testing.T) {
	mu.Lock()
	previousScheduler := scheduler
	mu.Unlock()
	previousScheduleBrightness := scheduleDeviceBrightness
	previousScheduleLcdBrightness := scheduleDeviceLcdBrightness
	previousSave := saveSchedulerSettings
	t.Cleanup(func() {
		mu.Lock()
		scheduler = previousScheduler
		mu.Unlock()
		scheduleDeviceBrightness = previousScheduleBrightness
		scheduleDeviceLcdBrightness = previousScheduleLcdBrightness
		saveSchedulerSettings = previousSave
	})

	var brightnessCalls []uint8
	var lcdCalls []uint8
	var saved []Scheduler
	scheduleDeviceBrightness = func(mode uint8) {
		brightnessCalls = append(brightnessCalls, mode)
	}
	scheduleDeviceLcdBrightness = func(mode uint8) {
		lcdCalls = append(lcdCalls, mode)
	}
	saveSchedulerSettings = func(value any) uint8 {
		current, ok := value.(Scheduler)
		if !ok {
			t.Fatalf("saved scheduler type = %T", value)
		}
		saved = append(saved, current)
		return 1
	}

	mu.Lock()
	scheduler = Scheduler{LightsOut: true, RGBControl: true, LCDControl: true}
	mu.Unlock()
	reconcileScheduledBrightness(true)
	if len(brightnessCalls) != 1 || brightnessCalls[0] != 0 {
		t.Fatalf("startup inside lights-out brightness calls = %v, want [0]", brightnessCalls)
	}
	if len(lcdCalls) != 1 || lcdCalls[0] != 0 {
		t.Fatalf("startup inside lights-out LCD calls = %v, want [0]", lcdCalls)
	}
	if len(saved) != 0 {
		t.Fatalf("unchanged persisted lights-out state was rewritten: %#v", saved)
	}

	brightnessCalls = nil
	lcdCalls = nil
	reconcileScheduledBrightness(false)
	if len(brightnessCalls) != 1 || brightnessCalls[0] != 1 {
		t.Fatalf("startup outside lights-out brightness calls = %v, want [1]", brightnessCalls)
	}
	if len(lcdCalls) != 1 || lcdCalls[0] != 1 {
		t.Fatalf("startup outside lights-out LCD calls = %v, want [1]", lcdCalls)
	}
	if len(saved) != 1 || saved[0].LightsOut {
		t.Fatalf("outside-range reconciliation persistence = %#v, want LightsOut false", saved)
	}
	if current := GetScheduler(); current.LightsOut {
		t.Fatalf("scheduler remained in lights-out after outside-range reconciliation: %#v", current)
	}

	brightnessCalls = nil
	lcdCalls = nil
	saved = nil
	mu.Lock()
	scheduler = Scheduler{LightsOut: true, RGBControl: true, LCDControl: true}
	mu.Unlock()
	if result := UpdateRgbSettings(false, "22:00", "06:00", true); result != 1 {
		t.Fatalf("disabling scheduler = %d, want 1", result)
	}
	if len(brightnessCalls) != 1 || brightnessCalls[0] != 1 || len(lcdCalls) != 1 || lcdCalls[0] != 1 {
		t.Fatalf("scheduler disable restore calls = brightness %v, LCD %v; want [1]/[1]", brightnessCalls, lcdCalls)
	}
	if len(saved) != 1 || saved[0].LightsOut || saved[0].RGBControl {
		t.Fatalf("disabled scheduler persistence = %#v", saved)
	}
}
