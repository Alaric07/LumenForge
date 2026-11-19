package m65rgbultraW

import (
	"LumenForge/src/common"
	"LumenForge/src/logger"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// A receiver binding is not Lighting state. A new epoch is required even when
// the owner and profile are unchanged. Only the parent owns/closes the HID handle.
var errLightingSessionRetired = errors.New("peripheral session retired")

type connectionSession struct {
	generation uint64
	receiver   *common.Slipstream
	endpoint   byte
	cancel     <-chan struct{}
	input      uint64
	dpi        uint64
	timer      uint64
	assignment uint64
	gesture    uint64
	angle      uint64
	lift       uint64
	button     uint64
}

func (d *Device) sessionLocked() connectionSession {
	return connectionSession{generation: d.connectionGeneration, receiver: d.lightingBinding, endpoint: d.lightingEndpoint, cancel: d.connectionCancel, input: d.inputRevision, dpi: d.dpiValueRevision, timer: d.timerRevision, assignment: d.assignmentRevision, gesture: d.gestureRevision, angle: d.angleRevision, lift: d.liftHeightRevision, button: d.buttonOptimizationRevision}
}
func (d *Device) sessionValidLocked(s connectionSession) bool {
	return s.generation == d.connectionGeneration && s.receiver == d.lightingBinding && s.receiver == d.dev && s.endpoint == d.lightingEndpoint && s.endpoint == d.Endpoint
}
func (d *Device) advanceConnectionLocked() {
	if d.connectionCancel != nil {
		close(d.connectionCancel)
	}
	d.connectionCancel = make(chan struct{})
	d.connectionGeneration++
	d.inputRevision++
	d.softwareModeGeneration = 0
	d.lightingBinding, d.lightingEndpoint = d.dev, d.Endpoint
	d.lightingHardwareReady = false
	d.invalidateExternalLocked()
	d.cancelLightingLocked()
}

// RebindReceiver retires the old endpoint before publishing its replacement.
// Normal USB replug constructs a new child; this also makes explicit in-place
// binding replacement safe without changing receiver discovery or transport.
func (d *Device) RebindReceiver(receiver *common.Slipstream, endpoint byte) {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	d.Connected = false
	d.connectPending = false
	d.dev, d.Endpoint, d.Path = receiver, endpoint, strconv.Itoa(int(endpoint))
	d.advanceConnectionLocked()
	d.SniperMode = false
	d.lightingMu.Unlock()
	d.refreshLightingRegistrationsLocked(true, true)
	d.lightingMutation.Unlock()
	d.releaseTiltHolds()
}

func (d *Device) disconnectLighting() (connectionSession, bool) {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	connected := d.Connected
	d.Connected = false
	d.connectPending = false
	d.advanceConnectionLocked()
	// A held input gesture is transient. Reconnect replays regular DPI AND color.
	d.SniperMode = false
	session := d.sessionLocked()
	d.lightingMu.Unlock()
	d.refreshLightingRegistrationsLocked(true, true)
	d.lightingMutation.Unlock()
	d.releaseTiltHolds()
	return session, connected
}

// Do not wait for deviceLock or an RGB Exit receiver on disconnect. Pending
// output checks the epoch after BOTH output serialization and receiver locking.
func (d *Device) stopCanonicalLighting(hardware bool) {
	d.lightingStopOnce.Do(func() {
		d.lightingMutation.Lock()
		d.lightingMu.Lock()
		restoreMode := d.softwareModeGeneration == d.connectionGeneration && d.softwareModeGeneration != 0 && d.sessionValidLocked(d.sessionLocked())
		d.Connected = false
		d.connectPending = false
		d.advanceConnectionLocked()
		d.Exit = true
		if d.lightingTemperatureCancel != nil {
			d.lightingTemperatureCancel()
		}
		d.SniperMode = false
		if d.externalStop != nil {
			close(d.externalStop)
		}
		session := d.sessionLocked()
		d.lightingMu.Unlock()
		d.refreshLightingRegistrationsLocked(true, true)
		d.lightingMutation.Unlock()
		d.releaseTiltHolds()
		if hardware && restoreMode {
			// A stopped receiver must not leave child shutdown waiting on its lock.
			acquireDeadline := make(chan struct{})
			timer := time.AfterFunc(time.Second, func() { close(acquireDeadline) })
			session.cancel = acquireDeadline
			_, _ = d.connectionTransfer(session, cmdHardwareMode, nil, func() bool { return d.Exit })
			timer.Stop()
		}
	})
}

// Receiver lock acquisition is cancellable. The established report layout and
// one-second peripheral response timeout are unchanged. No high-level state or
// mutation lock spans HID I/O. valid is evaluated under lightingMu at the FINAL
// boundary, after receiver serialization, so queued old work cannot cross wake.
func (d *Device) connectionTransfer(s connectionSession, command, data []byte, valid func() bool) ([]byte, error) {
	response := make([]byte, bufferSize)
	if s.receiver == nil {
		return response, fmt.Errorf("receiver unavailable")
	}
	select {
	case <-s.cancel:
		return response, errLightingSessionRetired
	default:
	}
	for !s.receiver.Mutex.TryLock() {
		if !lightingDelay(s.cancel, 2*time.Millisecond) {
			return response, errLightingSessionRetired
		}
	}
	defer s.receiver.Mutex.Unlock()
	report := make([]byte, bufferSizeWrite)
	if len(command)+len(data) > len(report)-headerSize {
		return response, fmt.Errorf("peripheral report too large")
	}
	available := d.lightingReportWrite != nil || s.receiver.Dev != nil
	report[1] = s.endpoint
	copy(report[headerSize:], command)
	copy(report[headerSize+len(command):], data)
	d.lightingMu.Lock()
	ok := d.sessionValidLocked(s) && valid()
	if ok && available && bytes.Equal(command, cmdSoftwareMode) {
		// Admission, not reply success: Write may take effect even on an error.
		d.softwareModeGeneration = s.generation
	}
	d.lightingMu.Unlock()
	if !ok {
		return response, errLightingSessionRetired
	}
	if d.lightingReportWrite != nil {
		_, err := d.lightingReportWrite(report)
		return response, err
	}
	if !available {
		return response, fmt.Errorf("receiver HID unavailable")
	}
	if _, err := s.receiver.Dev.Write(report); err != nil {
		return response, err
	}
	_, err := s.receiver.Dev.ReadWithTimeout(response, 1000*time.Millisecond)
	return response, err
}

// Attempt ownership and pending wake are guarded by the SAME lightingMu.
// Completion consumes pending before atomically releasing ownership.
func (d *Device) connectLighting() {
	d.lightingMu.Lock()
	if d.Exit || d.Connected {
		d.lightingMu.Unlock()
		return
	}
	if d.connecting {
		d.connectPending = true
		d.lightingMu.Unlock()
		return
	}
	d.connecting = true
	d.lightingMu.Unlock()
	for {
		d.runConnectLightingAttempt()
		if d.connectBeforeComplete != nil {
			d.connectBeforeComplete()
		}
		d.lightingMu.Lock()
		retry := d.connectPending && !d.Exit && !d.Connected
		d.connectPending = false
		if !retry {
			d.connecting = false
		}
		d.lightingMu.Unlock()
		if !retry {
			return
		}
	}
}
func (d *Device) runConnectLightingAttempt() {
	d.lightingMu.Lock()
	if d.Exit || d.Connected {
		d.lightingMu.Unlock()
		return
	}
	d.advanceConnectionLocked()
	s := d.sessionLocked()
	d.lightingMu.Unlock()
	valid := func() bool { return !d.Exit }
	online := false
	for i := 0; i < 10; i++ {
		if _, err := d.connectionTransfer(s, cmdHeartbeat, nil, valid); err == nil {
			online = true
			break
		}
		if i < 9 && !lightingDelay(s.cancel, time.Second) {
			return
		}
	}
	if !online {
		return
	}
	// Required setup failures are retried locally, on the same connection epoch.
	// Bound attempts and use cancellable pacing; a second wake is not required.
	for attempt := 0; attempt < 3; attempt++ {
		if d.setupLightingConnection(s) {
			return
		}
		if attempt < 2 && !lightingDelay(s.cancel, 40*time.Millisecond) {
			return
		}
	}
}
func (d *Device) setupLightingConnection(s connectionSession) bool {
	valid := func() bool { return !d.Exit }
	fw, err := d.connectionTransfer(s, cmdGetFirmware, nil, valid)
	if err == nil && fw[1] != 0x02 {
		fw, err = d.connectionTransfer(s, cmdGetFirmware, nil, valid)
	}
	if err == nil {
		d.lightingMu.Lock()
		if d.sessionValidLocked(s) {
			d.Firmware = fmt.Sprintf("%d.%d.%d", fw[3], fw[4], binary.LittleEndian.Uint16(fw[5:7]))
		}
		d.lightingMu.Unlock()
	}
	if _, err = d.connectionTransfer(s, cmdSoftwareMode, nil, valid); err != nil {
		return false
	}
	d.lightingMu.Lock()
	if !d.sessionValidLocked(s) || d.Exit {
		d.lightingMu.Unlock()
		return false
	}
	d.lightingMu.Unlock()
	battery, batteryErr := d.connectionTransfer(s, cmdBatteryLevel, nil, valid)
	if batteryErr == nil {
		d.publishConnectionBattery(s, binary.LittleEndian.Uint16(battery[3:5])/10)
	}
	// Input settings are snapshotted, not made authorities for desired Lighting.
	d.lightingMu.Lock()
	p := cloneLightingProfile(*d.DeviceProfile)
	s.input = d.inputRevision
	s.assignment = d.assignmentRevision
	s.gesture = d.gestureRevision
	s.angle = d.angleRevision
	s.lift = d.liftHeightRevision
	s.button = d.buttonOptimizationRevision
	d.lightingMu.Unlock()
	for _, v := range []struct {
		cmd   []byte
		value int
	}{{cmdAngleSnapping, p.AngleSnapping}, {cmdButtonOptimization, p.ButtonOptimization}, {cmdLiftHeight, p.LiftHeight}} {
		if bytes.Equal(v.cmd, cmdLiftHeight) {
			if !d.canReplayLiftHeight(v.value) {
				continue
			}
		} else if v.value < 0 || v.value > 1 {
			continue
		}
		// The legacy preference setters ignored errors. Keep transient retry,
		// but an exhausted optional preference must not gate mouse availability.
		for attempt := 0; attempt < 3; attempt++ {
			preferenceValid := func() bool { return !d.Exit && s.lift == d.liftHeightRevision }
			if bytes.Equal(v.cmd, cmdButtonOptimization) {
				preferenceValid = func() bool { return !d.Exit && s.button == d.buttonOptimizationRevision }
			}
			if bytes.Equal(v.cmd, cmdAngleSnapping) {
				preferenceValid = func() bool { return !d.Exit && s.angle == d.angleRevision }
			}
			_, err = d.connectionTransfer(s, v.cmd, []byte{byte(v.value)}, preferenceValid)
			if err == nil {
				break
			}
			if errors.Is(err, errLightingSessionRetired) {
				return false
			}
			if attempt < 2 && !lightingDelay(s.cancel, 40*time.Millisecond) {
				return false
			}
		}
		if err != nil {
			logger.Log(logger.Fields{"error": err}).Warn("Unable to apply optional mouse preference")
		}
	}
	// Legacy setGestures ignored errors for the enable byte and tilt settings.
	// These preferences are best-effort, unlike software/LED endpoint readiness.
	if p.MultiGesture >= 0 && p.MultiGesture <= 1 {
		if _, err = d.connectionTransfer(s, cmdMultiGesture, []byte{byte(p.MultiGesture)}, func() bool { return !d.Exit && s.gesture == d.gestureRevision }); err != nil {
			if errors.Is(err, errLightingSessionRetired) {
				return false
			}
			logger.Log(logger.Fields{"error": err}).Warn("Unable to apply optional multi-gesture setting")
		}
	}
	for i, cmd := range [][]byte{cmdLeftTilt, cmdRightTilt, cmdForwardTilt, cmdBackwardTilt} {
		if tilt, ok := p.Tilts[i]; ok {
			if _, err = d.connectionTransfer(s, cmd, []byte{tilt.Value}, func() bool { return !d.Exit && s.gesture == d.gestureRevision }); err != nil {
				if errors.Is(err, errLightingSessionRetired) {
					return false
				}
				logger.Log(logger.Fields{"error": err}).Warn("Unable to apply optional tilt setting")
			}
		}
	}
	if _, err = d.connectionTransfer(s, cmdOpenEndpoint, nil, valid); err != nil {
		return false
	}
	if !lightingDelay(s.cancel, 100*time.Millisecond) {
		return false
	}
	if !d.replayConnectionAssignments(s) {
		return false
	}
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	if !d.sessionValidLocked(s) || d.Exit || s.input != d.inputRevision {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return false
	}
	d.Connected, d.lightingHardwareReady = true, true
	d.SniperMode = false
	d.lightingMu.Unlock()
	d.refreshLightingRegistrationsLocked(true, true)
	d.lightingMutation.Unlock()
	// Regular DPI and its color replay together. The Sniper preference is untouched.
	d.programCanonicalDPI()
	d.setDeviceColor()
	d.setSleepTimer()
	return true
}

func (d *Device) sleepLighting() {
	s, connected := d.disconnectLighting()
	if !connected {
		return
	}
	// Identity was captured atomically with invalidation, before any new wake.
	go func() {
		session := s
		if d.sleepBeforeCommand != nil {
			d.sleepBeforeCommand(session)
		}
		_, err := d.connectionTransfer(session, cmdSleepMode, nil, func() bool { return !d.Exit && !d.Connected })
		if d.sleepAfterCommand != nil {
			d.sleepAfterCommand(session, err)
		}
		if err != nil {
			logger.Log(logger.Fields{"error": err}).Debug("Peripheral sleep command retired")
		}
	}()
}

func (d *Device) publishConnectionBattery(session connectionSession, level uint16) bool {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if !d.sessionValidLocked(session) || d.Exit {
		return false
	}
	d.publishBatteryLocked(level)
	return true
}
func (d *Device) replayConnectionAssignments(session connectionSession) bool {
	d.lightingMu.Lock()
	current := d.sessionValidLocked(session) && !d.Exit
	deferred := d.assignmentReplayDeferred
	d.lightingMu.Unlock()
	if !current {
		return false
	}
	if deferred {
		err := d.loadCanonicalAssignmentsForSession(&session)
		if errors.Is(err, errLightingSessionRetired) {
			return false
		}
		d.lightingMu.Lock()
		current = d.sessionValidLocked(session) && !d.Exit
		deferred = d.assignmentReplayDeferred
		d.lightingMu.Unlock()
		if !current {
			return false
		}
		if deferred {
			// Keep the original persisted custom map and connection availability.
			logger.Log(logger.Fields{"serial": d.Serial, "error": err}).Warn("Connected with assignment repair still deferred after storage retry")
			return true
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		err := d.programAssignmentsForSession(&session)
		if err == nil {
			return true
		}
		if errors.Is(err, errLightingSessionRetired) {
			return false
		}
		if errors.Is(err, errInvalidAssignments) {
			d.lightingMu.Lock()
			current := d.sessionValidLocked(session) && !d.Exit
			d.lightingMu.Unlock()
			if !current {
				return false
			}
			// Legacy setup skipped unsupported mapping shapes and still connected.
			// Keep stored custom data untouched and never emit its invalid payload.
			logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Warn("Connected with invalid stored assignment replay skipped")
			return true
		}
		if attempt < 2 {
			if d.assignmentBeforeRetry != nil {
				d.assignmentBeforeRetry(session)
			}
			if !lightingDelay(session.cancel, 40*time.Millisecond) {
				return false
			}
		}
	}
	return false
}
