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
