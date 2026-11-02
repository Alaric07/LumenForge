package harpoonWU

import (
	"fmt"
	"time"
)

func (d *Device) lightingConnected() bool {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return !d.Exit && d.Connected
}

func (d *Device) lightingExited() bool {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	return d.Exit
}

// WU owns a direct USB handle. Init sets up software mode and the LED endpoint;
// replug constructs a fresh Device, never a receiver-child reconnect session.
// Availability changes retire output without touching persisted desired state.
func (d *Device) setUSBLightingAvailable(ready bool) {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	if d.Exit {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return
	}
	changed := d.Connected != ready || d.lightingHardwareReady != ready
	d.Connected, d.lightingHardwareReady = ready, ready
	if changed {
		d.invalidateExternalLocked()
		d.cancelLightingLocked()
	}
	d.lightingMu.Unlock()
	d.refreshLightingRegistrationsLocked(true, true)
	d.lightingMutation.Unlock()
}

// No output lock or HID wait here: a blocked Read cannot prevent retirement of
// the worker, queued frames or registration closures. An issued write may finish.
func (d *Device) stopCanonicalLighting() {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.exitLighting()
	d.refreshLightingRegistrationsLocked(true, true)
}

func (d *Device) stopKeepAlive() {
	d.keepAliveStop.Do(func() {
		if d.keepAliveChan != nil {
			close(d.keepAliveChan)
		}
	})
}

func (d *Device) reconfigureCanonicalPolling(rate int) uint8 {
	if _, ok := d.PollingRates[rate]; !ok {
		return 0
	}
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	if d.Exit || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	proposal := cloneLightingProfile(*d.DeviceProfile)
	proposal.PollingRate = rate
	err := d.persistLightingProfile(proposal)
	if err != nil {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	*d.DeviceProfile = proposal
	d.lightingMu.Unlock()
	// Preserve WU's terminal Exit transition, settling delay and no-read command.
	d.exitLighting()
	d.refreshLightingRegistrationsLocked(true, true)
	d.lightingMutation.Unlock()
	time.Sleep(40 * time.Millisecond)
	write := d.lightingPollingWrite
	if write == nil {
		write = func(command, data []byte, read bool) error {
			if d.dev == nil {
				return fmt.Errorf("USB handle unavailable")
			}
			_, err := d.transfer(command, data, read)
			return err
		}
	}
	if err := write(cmdSetPollingRate, []byte{byte(rate)}, false); err != nil {
		return 0
	}
	return 1
}
