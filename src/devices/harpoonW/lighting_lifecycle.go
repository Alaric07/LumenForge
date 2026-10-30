package harpoonW

import (
	"LumenForge/src/logger"
	"time"
)

func (d *Device) lightingConnected() bool {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return !d.Exit && d.Connected
}

// A receiver child can have a valid desired-state source while the mouse is
// unavailable. Connection epochs also retire setup work interrupted by sleep.
func (d *Device) connectLighting() {
	d.lightingMu.Lock()
	if d.Exit || d.Connected || d.lightingConnecting {
		d.lightingMu.Unlock()
		return
	}
	d.lightingConnecting = true
	d.lightingConnection++
	connection := d.lightingConnection
	d.lightingMu.Unlock()
	ready := false
	if d.lightingSetup != nil {
		ready = d.lightingSetup()
	} else if d.checkDeviceOnline() {
		d.lightingMu.Lock()
		current := !d.Exit && connection == d.lightingConnection
		d.lightingMu.Unlock()
		if current {
			d.getDeviceFirmware()
			_, modeErr := d.transfer(cmdSoftwareMode, nil)
			d.getBatterLevel()
			d.setAngleSnapping()
			d.setButtonOptimization()
			_, ledErr := d.transfer(cmdOpenEndpoint, nil)
			ready = modeErr == nil && ledErr == nil
			if !ready {
				logger.Log(logger.Fields{"serial": d.Serial}).Warn("Lighting endpoint is not ready")
			}
		}
	}
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	if d.Exit || connection != d.lightingConnection {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return
	}
	d.lightingConnecting = false
	d.Connected, d.lightingHardwareReady = ready, ready
	d.invalidateExternalLocked()
	d.cancelLightingLocked()
	d.lightingMu.Unlock()
	d.refreshLightingRegistrationsLocked(true, true)
	d.lightingMutation.Unlock()
	if ready {
		d.toggleDPI() // source regular DPI command and current Sniper indicator; one renderer start
		if d.lightingSetup == nil {
			d.setupKeyAssignment()
			d.setSleepTimer()
		}
	}
}

func (d *Device) disconnectLighting(value bool) bool {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	// SetConnected(true) was never the receiver child's connection entry point.
	if !d.Connected && !d.lightingConnecting {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return false
	}
	d.lightingConnection++
	d.lightingConnecting = false
	d.Connected = value
	d.lightingHardwareReady = false
	d.cancelLightingLocked()
	d.invalidateExternalLocked()
	d.lightingMu.Unlock()
	d.refreshLightingRegistrationsLocked(true, true)
	d.lightingMutation.Unlock()
	return true
}

func (d *Device) stopCanonicalLighting(hardware bool) {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	wasConnected := d.Connected
	d.lightingConnection++
	d.lightingConnecting = false
	d.lightingMu.Unlock()
	d.exitLighting()
	d.refreshLightingRegistrationsLocked(true, true)
	d.lightingMutation.Unlock()
	if hardware && wasConnected && d.dev != nil {
		d.setHardwareMode()
	}
}

func (d *Device) sleepCanonicalLighting() {
	if !d.lightingConnected() {
		return
	}
	// Never hold deviceLock while cancelling workers or reconciling controllers.
	d.disconnectLighting(false)
	d.lightingMu.Lock()
	connection := d.lightingConnection
	d.lightingMu.Unlock()
	time.Sleep(time.Second) // source disconnect settling delay
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	current := !d.Exit && !d.Connected && !d.lightingConnecting && connection == d.lightingConnection
	d.lightingMu.Unlock()
	if current && d.dev != nil {
		_, _ = d.transfer(cmdSleepMode, nil)
	}
}
