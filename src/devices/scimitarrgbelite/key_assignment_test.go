package scimitarrgbelite

import (
	"testing"

	"LumenForge/src/inputmanager"
)

func TestScimitarEliteSniperKeyAssignmentModes(t *testing.T) {
	newDevice := func(actionHold bool) *Device {
		return &Device{
			Exit:          true,
			DeviceProfile: &DeviceProfile{Profiles: map[int]DPIProfile{}},
			KeyAssignment: map[int]inputmanager.KeyAssignment{1: {ActionType: 8, ActionHold: actionHold}},
		}
	}

	t.Run("press and hold", func(t *testing.T) {
		device := newDevice(true)
		device.triggerKeyAssignment(1)
		if !device.SniperMode {
			t.Fatal("Sniper was not enabled on press")
		}
		device.triggerKeyAssignment(0)
		if device.SniperMode {
			t.Fatal("Sniper was not disabled on release")
		}
	})

	t.Run("toggle", func(t *testing.T) {
		device := newDevice(false)
		device.triggerKeyAssignment(1)
		if !device.SniperMode {
			t.Fatal("Sniper was not enabled on first press")
		}
		device.triggerKeyAssignment(0)
		if !device.SniperMode {
			t.Fatal("Sniper was disabled on release")
		}
		device.triggerKeyAssignment(1)
		if device.SniperMode {
			t.Fatal("Sniper was not disabled on second press")
		}
	})
}

func TestScimitarEliteCycleClusterEffectKeyAssignmentTransitions(t *testing.T) {
	originalCycle := cycleClusterLightingEffect
	cycleCalls := 0
	cycleClusterLightingEffect = func() error {
		cycleCalls++
		return nil
	}
	t.Cleanup(func() { cycleClusterLightingEffect = originalCycle })

	device := &Device{
		Exit:          true,
		DeviceProfile: &DeviceProfile{Profiles: map[int]DPIProfile{}},
		KeyAssignment: map[int]inputmanager.KeyAssignment{1: {Default: false, ActionType: 30}},
	}
	device.triggerKeyAssignment(1)
	if cycleCalls != 1 {
		t.Fatalf("cycle calls after first press = %d, want 1", cycleCalls)
	}

	device.triggerKeyAssignment(0)
	if cycleCalls != 1 {
		t.Fatalf("cycle calls after release = %d, want 1", cycleCalls)
	}

	device.triggerKeyAssignment(1)
	if cycleCalls != 2 {
		t.Fatalf("cycle calls after second press = %d, want 2", cycleCalls)
	}
}
