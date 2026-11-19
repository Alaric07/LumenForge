package m65rgbultraW

import (
	"LumenForge/src/common"
	"LumenForge/src/inputmanager"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Input preferences retain profile backing. Copies keep indicator composition
// and profile replacement from observing partially edited stage/color maps.
// Optional request capture runs under these locks after commit, before output waits.
func (d *Device) mutateInputProfile(connected bool, change func(*DeviceProfile) error, published ...func()) uint8 {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	if d.Exit || d.DeviceProfile == nil || (connected && !d.Connected) {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	proposal := cloneLightingProfile(*d.DeviceProfile)
	err := change(&proposal)
	if err == nil {
		err = d.persistLightingProfile(proposal)
	}
	if err == nil {
		if !sameGesturePayload(*d.DeviceProfile, proposal) {
			d.gestureRevision++
		}
		if d.DeviceProfile.AngleSnapping != proposal.AngleSnapping {
			d.angleRevision++
		}
		if d.DeviceProfile.LiftHeight != proposal.LiftHeight {
			d.liftHeightRevision++
		}
		if d.DeviceProfile.ButtonOptimization != proposal.ButtonOptimization {
			d.buttonOptimizationRevision++
		}
		if d.DeviceProfile.SleepMode != proposal.SleepMode {
			d.timerRevision++
		}
		if !sameEffectiveDPI(*d.DeviceProfile, proposal, d.SniperMode) {
			d.dpiValueRevision++
		}
		*d.DeviceProfile = proposal
		d.inputRevision++
		for _, capture := range published {
			capture()
		}
	}
	d.lightingMu.Unlock()
	d.lightingMutation.Unlock()
	return lightingResult(err)
}
func (d *Device) saveCanonicalMouseColors(zones map[int]rgb.Color) uint8 {
	updates := make(map[string]lightingsettings.Color, len(zones))
	for id, c := range zones {
		updates[strconv.Itoa(id)] = lightingColor(c)
	}
	return lightingResult(d.updateLightingPalette("mouse", updates))
}

func (d *Device) saveCanonicalDPI(stages map[int]uint16, colors map[int]rgb.Color) uint8 {
	var request dpiApplyRequest
	result := d.mutateInputProfile(false, func(p *DeviceProfile) error {
		full := colors != nil
		if len(stages) == 0 || (full && (len(stages) != len(p.Profiles) || len(colors) != len(p.Profiles))) {
			return fmt.Errorf("incomplete DPI settings")
		}
		changed := 0
		for id, value := range stages {
			stage, ok := p.Profiles[id]
			if !ok || value < uint16(minDpiValue) || value > uint16(maxDpiValue) {
				if full {
					return fmt.Errorf("invalid DPI")
				}
				continue
			}
			if full {
				color, ok := colors[id]
				if !ok || stage.Color == nil || !m65rgbultraWColorOK(color) {
					return fmt.Errorf("invalid DPI color")
				}
				stage.Color.Red, stage.Color.Green, stage.Color.Blue = color.Red, color.Green, color.Blue
				stage.Color.Hex = fmt.Sprintf("#%02x%02x%02x", int(color.Red), int(color.Green), int(color.Blue))
			}
			stage.Value = value
			p.Profiles[id] = stage
			changed++
		}
		if changed == 0 {
			return fmt.Errorf("no valid DPI stages")
		}
		return nil
	}, func() { request = d.captureDPIApplyLocked() })
	if result == 1 {
		return lightingResult(d.applyCanonicalDPIAndIndicator(request))
	}
	return result
}
func (d *Device) saveCanonicalDPIColors(dpi rgb.Color, colors map[int]rgb.Color) uint8 {
	if !m65rgbultraWColorOK(dpi) {
		return 0
	}
	result := d.mutateInputProfile(false, func(p *DeviceProfile) error {
		changed := 0
		for id, color := range colors {
			stage, ok := p.Profiles[id]
			if !ok || stage.Color == nil || !m65rgbultraWColorOK(color) {
				continue
			}
			stage.Color.Red, stage.Color.Green, stage.Color.Blue = color.Red, color.Green, color.Blue
			stage.Color.Hex = fmt.Sprintf("#%02x%02x%02x", int(color.Red), int(color.Green), int(color.Blue))
			p.Profiles[id] = stage
			changed++
		}
		if changed == 0 {
			return fmt.Errorf("no valid DPI colors")
		}
		return nil
	})
	if result == 1 {
		d.refreshIndicatorLighting()
	}
	return result
}
func (d *Device) selectCanonicalDPI(stage int) uint8 {
	d.lightingMu.Lock()
	if d.Exit || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		return 0
	}
	current, valid := d.DeviceProfile.Profiles[stage]
	same := valid && !current.Sniper && d.DeviceProfile.Profile == stage
	request := d.captureDPIApplyLocked()
	d.lightingMu.Unlock()
	if same {
		if !request.sniper {
			return lightingResult(d.applyCanonicalDPIAndIndicator(request))
		}
		return 1
	}

	result := d.mutateInputProfile(false, func(p *DeviceProfile) error {
		value, ok := p.Profiles[stage]
		if !ok || value.Sniper {
			return fmt.Errorf("invalid DPI stage")
		}
		p.Profile = stage
		return nil
	}, func() { request = d.captureDPIApplyLocked() })
	if result == 1 && !request.sniper {
		return lightingResult(d.applyCanonicalDPIAndIndicator(request))
	}
	return result
}
func (d *Device) modifyCanonicalDPI(increment bool) {
	var request dpiApplyRequest
	result := d.mutateInputProfile(false, func(p *DeviceProfile) error {
		if increment {
			if p.Profile >= 4 {
				return fmt.Errorf("last DPI stage")
			}
			p.Profile++
		} else {
			if p.Profile <= 0 {
				return fmt.Errorf("first DPI stage")
			}
			p.Profile--
		}
		return nil
	}, func() { request = d.captureDPIApplyLocked() })
	if result == 1 {
		_ = d.applyCanonicalDPIAndIndicator(request)
	}
}
func (d *Device) canonicalSniper(active bool) uint8 {
	d.lightingMu.Lock()
	if d.Exit || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		return 0
	}
	found := false
	for _, p := range d.DeviceProfile.Profiles {
		if p.Sniper {
			found = true
			break
		}
	}
	if found {
		before, beforeOK := effectiveDPIValue(*d.DeviceProfile, d.SniperMode)
		d.SniperMode = active
		d.inputRevision++
		after, afterOK := effectiveDPIValue(*d.DeviceProfile, d.SniperMode)
		if before != after || beforeOK != afterOK {
			d.dpiValueRevision++
		}
	}
	request := d.captureDPIApplyLocked()
	d.lightingMu.Unlock()
	if !found {
		return 0
	}
	return lightingResult(d.applyCanonicalDPIAndIndicator(request))
}

// A request is a transient authorization for one published effective value.
// Profile pointer identity survives snapshots but changes on activation.
type dpiApplyRequest struct {
	session              connectionSession
	profile              *DeviceProfile
	value                uint16
	valid, ready, sniper bool
}

func (d *Device) captureDPIApplyLocked() dpiApplyRequest {
	r := dpiApplyRequest{session: d.sessionLocked(), profile: d.DeviceProfile, ready: d.lightingOutputReadyLocked(), sniper: d.SniperMode}
	if d.DeviceProfile != nil {
		r.value, r.valid = effectiveDPIValue(*d.DeviceProfile, d.SniperMode)
	}
	return r
}
func (d *Device) dpiRequestCurrentLocked(r dpiApplyRequest) bool {
	return r.valid && d.DeviceProfile == r.profile && d.sessionValidLocked(r.session) && r.session.dpi == d.dpiValueRevision && d.lightingOutputReadyLocked()
}

// Pair confirmation is per apply operation, not another persistent authority.
func (d *Device) applyCanonicalDPIAndIndicator(expected ...dpiApplyRequest) error {
	var request dpiApplyRequest
	if len(expected) > 0 {
		request = expected[0]
	} else {
		d.lightingMu.Lock()
		request = d.captureDPIApplyLocked()
		d.lightingMu.Unlock()
	}
	if !request.ready {
		return nil
	} // Saved offline; wake owns later replay.
	if err := d.programCanonicalDPI(request); err != nil {
		return err
	}
	d.refreshIndicatorLighting()
	return nil
}

// Ultra programs the captured effective value into both axes. Retry the whole
// pair at most three times; every command and retry keeps the same value/session.
func (d *Device) programCanonicalDPI(expected ...dpiApplyRequest) (applyErr error) {
	defer func() {
		if applyErr != nil {
			logger.Log(logger.Fields{"error": applyErr}).Warn("Desired Ultra DPI retained; hardware apply incomplete")
		}
	}()
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	request := d.captureDPIApplyLocked()
	if len(expected) > 0 {
		request = expected[0]
	}
	if !d.dpiRequestCurrentLocked(request) {
		d.lightingMu.Unlock()
		return errLightingSessionRetired
	}
	session, value := request.session, request.value
	d.lightingMu.Unlock()
	data := make([]byte, 2)
	binary.LittleEndian.PutUint16(data, value)
	for attempt := 0; attempt < 3; attempt++ {
		applyErr = nil
		for axis := 0; axis < 2; axis++ {
			err := d.writeDPICommand(session, cmdSetDpi[axis], data)
			if errors.Is(err, errLightingSessionRetired) {
				return err
			}
			applyErr = errors.Join(applyErr, err)
		}
		d.lightingMu.Lock()
		current := d.dpiRequestCurrentLocked(request)
		d.lightingMu.Unlock()
		if !current {
			return errLightingSessionRetired
		}
		if applyErr == nil {
			return nil
		}
		if attempt < 2 && !lightingDelay(session.cancel, 40*time.Millisecond) {
			return errLightingSessionRetired
		}
	}
	return applyErr
}

func (d *Device) rotateCanonicalProfile() {
	d.lightingMu.Lock()
	name := ""
	if d.DeviceProfile != nil && len(d.ProfileOrder) > 0 {
		name = d.ProfileOrder[0]
		for i, key := range d.ProfileOrder {
			if d.UserProfiles[key] == d.DeviceProfile {
				name = d.ProfileOrder[(i+1)%len(d.ProfileOrder)]
				break
			}
		}
	}
	d.lightingMu.Unlock()
	if name != "" {
		d.changeCanonicalDeviceProfile(name)
	}
}

// Named input snapshots are independent; re-saving the active name preserves its
// identity. Neither path imports serialized Lighting into canonical desired state.
func (d *Device) saveCanonicalUserProfile(name string) uint8 {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.Exit || d.DeviceProfile == nil || !common.AlphanumericDashRegex.MatchString(name) {
		return 0
	}
	existing := d.UserProfiles[name]
	replacingActive := existing != nil && existing == d.DeviceProfile
	proposal := cloneLightingProfile(*d.DeviceProfile)
	proposal.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-"+name+".json")
	if existing != nil && existing.Path != "" {
		// Existing logical profiles retain their file, including inactive default.
		proposal.Path = existing.Path
	} else {
		path, err := d.availableProfilePathLocked(proposal.Path)
		if err != nil {
			return 0
		}
		proposal.Path = path
	}
	oldHash := ""
	if existing != nil {
		oldHash = existing.KeyAssignmentHash
	}
	proposal.ProfileName = name
	proposal.Active = replacingActive
	proposal.KeyAssignmentHash = common.GenerateRandomMD5()
	data, err := json.MarshalIndent(d.KeyAssignment, "", "    ")
	if err != nil {
		return 0
	}
	assignmentPath := filepath.Join(pwd, "database", "key-assignments", proposal.KeyAssignmentHash+".json")
	if err = d.persistLightingAssignments(assignmentPath, data); err != nil {
		_ = os.Remove(assignmentPath)
		return 0
	}
	if err = d.persistLightingProfile(proposal); err != nil {
		_ = os.Remove(assignmentPath)
		return 0
	}
	if d.UserProfiles == nil {
		d.UserProfiles = map[string]*DeviceProfile{}
	}
	if replacingActive {
		// Rotation identifies the current profile by pointer identity.
		*d.DeviceProfile = proposal
		d.inputRevision++
		d.UserProfiles[name] = d.DeviceProfile
	} else {
		d.UserProfiles[name] = &proposal
	}
	d.rebuildProfileOrder()
	d.cleanupSupersededAssignmentsLocked(oldHash, proposal.KeyAssignmentHash)
	return 1
}

func (d *Device) setCanonicalSniperMode(active bool) uint8 {
	d.lightingMu.Lock()
	if d.Exit || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		return 0
	}
	found := false
	for _, stage := range d.DeviceProfile.Profiles {
		if stage.Sniper {
			found = true
			break
		}
	}
	same := d.SniperMode == active
	request := d.captureDPIApplyLocked()
	d.lightingMu.Unlock()
	if !found {
		return 0
	}
	if same {
		return lightingResult(d.applyCanonicalDPIAndIndicator(request))
	}
	return d.canonicalSniper(active)
}

// Retain Ultra command bytes and raw input values.
func (d *Device) programLightingPreference(kind string) (applyErr error) {
	defer func() {
		if applyErr != nil {
			logger.Log(logger.Fields{"error": applyErr, "preference": kind}).Warn("Mouse preference saved; hardware apply failed")
		}
	}()
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	if !d.lightingOutputReadyLocked() || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		return nil
	}
	p := cloneLightingProfile(*d.DeviceProfile)
	session := d.sessionLocked()
	d.lightingMu.Unlock()
	switch kind {
	case "angle":
		if _, ok := d.SwitchModes[p.AngleSnapping]; ok && p.AngleSnapping >= 0 && p.AngleSnapping <= 1 {
			return d.writeAngleCommand(session, cmdAngleSnapping, []byte{byte(p.AngleSnapping)})
		}
	case "button":
		if _, ok := d.SwitchModes[p.ButtonOptimization]; ok && p.ButtonOptimization >= 0 && p.ButtonOptimization <= 1 {
			return d.writeButtonOptimizationCommand(session, cmdButtonOptimization, []byte{byte(p.ButtonOptimization)})
		}
	case "lift":
		if d.canReplayLiftHeight(p.LiftHeight) {
			return d.writeLiftHeightCommand(session, cmdLiftHeight, []byte{byte(p.LiftHeight)})
		}
	case "gestures":
		if p.MultiGesture < 0 || p.MultiGesture > 1 {
			return nil
		}
		if err := d.writeGestureCommand(session, cmdMultiGesture, []byte{byte(p.MultiGesture)}, false); err != nil {
			logger.Log(logger.Fields{"error": err}).Warn("Unable to program mouse gestures")
			if errors.Is(err, errLightingSessionRetired) {
				return err
			}
		}
		for i, c := range [][]byte{cmdLeftTilt, cmdRightTilt, cmdForwardTilt, cmdBackwardTilt} {
			if t, ok := p.Tilts[i]; ok {
				if err := d.writeGestureCommand(session, c, []byte{t.Value}, false); err != nil {
					logger.Log(logger.Fields{"error": err}).Warn("Unable to program mouse tilt preference")
					if errors.Is(err, errLightingSessionRetired) {
						return err
					}
				}
			}
		}
	}
	return nil
}
func (d *Device) writeLightingCommand(command, data []byte) error {
	d.lightingMu.Lock()
	session := d.sessionLocked()
	d.lightingMu.Unlock()
	return d.writeSessionCommand(session, command, data)
}
func (d *Device) writeSessionCommand(session connectionSession, command, data []byte) error {
	return d.writeInputCommand(session, command, data, false)
}
func (d *Device) writeInputCommand(session connectionSession, command, data []byte, cleanup bool) error {
	return d.writeValidatedInputCommand(session, command, data, func() bool { return d.lightingOutputReadyLocked() && (cleanup || session.input == d.inputRevision) })
}
func (d *Device) writeAssignmentCommand(session connectionSession, command, data []byte, cleanup bool) error {
	return d.writeValidatedInputCommand(session, command, data, func() bool {
		return d.lightingOutputReadyLocked() && (cleanup || session.assignment == d.assignmentRevision)
	})
}
func (d *Device) writeGestureCommand(session connectionSession, command, data []byte, cleanup bool) error {
	return d.writeValidatedInputCommand(session, command, data, func() bool {
		return d.lightingOutputReadyLocked() && (cleanup || session.gesture == d.gestureRevision)
	})
}
func (d *Device) writeAngleCommand(session connectionSession, command, data []byte) error {
	return d.writeValidatedInputCommand(session, command, data, func() bool {
		return d.lightingOutputReadyLocked() && session.angle == d.angleRevision
	})
}
func (d *Device) writeLiftHeightCommand(session connectionSession, command, data []byte) error {
	return d.writeValidatedInputCommand(session, command, data, func() bool { return d.lightingOutputReadyLocked() && session.lift == d.liftHeightRevision })
}
func (d *Device) writeButtonOptimizationCommand(session connectionSession, command, data []byte) error {
	return d.writeValidatedInputCommand(session, command, data, func() bool { return d.lightingOutputReadyLocked() && session.button == d.buttonOptimizationRevision })
}
func (d *Device) writeDPICommand(session connectionSession, command, data []byte) error {
	return d.writeValidatedInputCommand(session, command, data, func() bool { return d.lightingOutputReadyLocked() && session.dpi == d.dpiValueRevision })
}
func (d *Device) writeTimerCommand(session connectionSession, command, data []byte) error {
	return d.writeValidatedInputCommand(session, command, data, func() bool { return d.lightingOutputReadyLocked() && session.timer == d.timerRevision })
}
func (d *Device) writeSetupInputCommand(session connectionSession, command, data []byte, cleanup bool) error {
	return d.writeValidatedInputCommand(session, command, data, func() bool {
		return !d.Exit && d.softwareModeGeneration == session.generation && (cleanup || session.assignment == d.assignmentRevision)
	})
}

// valid runs under lightingMu, after transport serialization in production.
func (d *Device) writeValidatedInputCommand(session connectionSession, command, data []byte, valid func() bool) error {
	if d.lightingInputBeforeWrite != nil {
		d.lightingInputBeforeWrite(session, command)
	}
	if d.lightingDPIWrite != nil {
		d.lightingMu.Lock()
		ok := d.sessionValidLocked(session) && valid()
		d.lightingMu.Unlock()
		if !ok {
			return errLightingSessionRetired
		}
		return d.lightingDPIWrite(append([]byte(nil), command...), append([]byte(nil), data...))
	}
	_, err := d.connectionTransfer(session, command, data, valid)
	return err
}

// Mirror saveDeviceProfile's existing non-Lighting normalization on activation.
// Ultra fresh and upgrade Sniper both retain 200 DPI/yellow.
func prepareLightingInputProfile(profile DeviceProfile) DeviceProfile {
	p := cloneLightingProfile(profile)
	if p.LiftHeight == 0 {
		p.LiftHeight = 2
	}
	if p.PollingRate == 0 {
		p.PollingRate = 4
	}
	if p.Tilts == nil {
		p.Tilts = map[int]TiltOption{0: {Name: "Left Tilt", Value: 20}, 1: {Name: "Right Tilt", Value: 20}, 2: {Name: "Forward Tilt", Value: 20}, 3: {Name: "Backward Tilt", Value: 20}}
	}
	if p.BrightnessSlider == nil {
		b := uint8(100)
		p.BrightnessSlider = &b
	}
	if p.Profiles == nil {
		p.Profiles = make(map[int]DPIProfile)
	}
	input := Device{DeviceProfile: &p}
	input.upgradeDpiProfiles()
	return p
}

// Assignment persistence is observable by the caller, before profile linkage or
// runtime publication. The seam also lets failure tests leave real files intact.
func (d *Device) persistLightingAssignments(path string, data []byte) error {
	write := d.lightingAssignmentWrite
	if write == nil {
		write = writeLightingProfile
	}
	return write(path, data)
}

func (d *Device) updateCanonicalKeyAssignment(key int, assignment inputmanager.KeyAssignment) uint8 {
	var request assignmentApplyRequest
	result := func() uint8 {
		d.lightingMutation.Lock()
		defer d.lightingMutation.Unlock()
		d.lightingMu.Lock()
		defer d.lightingMu.Unlock()
		if d.Exit || d.DeviceProfile == nil {
			return 0
		}
		old, ok := d.KeyAssignment[key]
		if !ok {
			return 0
		}
		proposal := make(map[int]inputmanager.KeyAssignment, len(d.KeyAssignment))
		for id, value := range d.KeyAssignment {
			proposal[id] = value
		}
		old.Default, old.ActionHold, old.ActionType = assignment.Default, assignment.ActionHold, assignment.ActionType
		old.ActionCommand, old.IsMacro, old.OnRelease = assignment.ActionCommand, assignment.IsMacro, assignment.OnRelease
		proposal[key] = old
		data, err := json.MarshalIndent(proposal, "", "    ")
		if err != nil {
			return 0
		}
		path := pwd + d.keyAssignmentFile
		if hash := d.DeviceProfile.KeyAssignmentHash; hash != "" {
			path = filepath.Join(pwd, "database", "key-assignments", hash+".json")
		}
		if err = d.persistLightingAssignments(path, data); err != nil {
			return 0
		}
		if !sameAssignmentPayload(d.KeyAssignment, proposal) {
			d.assignmentRevision++
		}
		d.KeyAssignment = proposal
		d.inputRevision++
		request = assignmentApplyRequest{session: d.sessionLocked(), profile: d.DeviceProfile, key: key, ready: d.lightingOutputReadyLocked(), bindings: make(map[int]inputmanager.KeyAssignment, len(proposal))}
		for id, value := range proposal {
			request.bindings[id] = value
		}
		return 1
	}()
	if result == 1 {
		if request.ready {
			if err := d.programAssignmentsForEdit(request); err != nil {
				logger.Log(logger.Fields{"error": err}).Warn("Mouse assignments saved but hardware programming failed")
				return 0 // Durable preference remains saved; no invented rollback.
			}
		}
	}
	return result
}

// Revisions describe hardware payloads, not serialized names or local macro actions.
func sameGesturePayload(a, b DeviceProfile) bool {
	if a.MultiGesture != b.MultiGesture {
		return false
	}
	for i := 0; i < 4; i++ {
		x, xok := a.Tilts[i]
		y, yok := b.Tilts[i]
		if xok != yok || (xok && x.Value != y.Value) {
			return false
		}
	}
	return true
}
func sameAssignmentPayload(a, b map[int]inputmanager.KeyAssignment) bool {
	if len(a) != len(b) {
		return false
	}
	for key, x := range a {
		y, ok := b[key]
		if !ok || x.Default != y.Default || x.IsTilt != y.IsTilt || (x.IsTilt && x.TiltIndex != y.TiltIndex) {
			return false
		}
	}
	return true
}

// Source defaults include four tilt bindings, with Ultra's original tilt indices.
func defaultWirelessAssignments() map[int]inputmanager.KeyAssignment {
	return map[int]inputmanager.KeyAssignment{
		2048: {
			Name:          "Tilt Right",
			Default:       true,
			ActionType:    0,
			ActionCommand: 0,
			ActionHold:    false,
			IsTilt:        true,
			TiltIndex:     1,
		},
		1024: {
			Name:          "Tilt Left",
			Default:       true,
			ActionType:    0,
			ActionCommand: 0,
			ActionHold:    false,
			IsTilt:        true,
			TiltIndex:     0,
		},
		512: {
			Name:          "Tilt Back",
			Default:       true,
			ActionType:    0,
			ActionCommand: 0,
			ActionHold:    false,
			IsTilt:        true,
			TiltIndex:     2,
		},
		256: {
			Name:          "Tilt Front",
			Default:       true,
			ActionType:    0,
			ActionCommand: 0,
			ActionHold:    false,
			IsTilt:        true,
			TiltIndex:     3,
		},
		128: {
			Name:          "Sniper",
			Default:       false,
			ActionType:    8,
			ActionCommand: 0,
			ActionHold:    true,
		},
		64: {
			Name:          "DPI -",
			Default:       true,
			ActionType:    0,
			ActionCommand: 0,
			ActionHold:    false,
		},
		32: {
			Name:          "DPI +",
			Default:       true,
			ActionType:    0,
			ActionCommand: 0,
			ActionHold:    false,
		},
		8: {
			Name:          "Forward Button",
			Default:       true,
			ActionType:    0,
			ActionCommand: 0,
			ActionHold:    false,
		},
		16: {
			Name:          "Back Button",
			Default:       true,
			ActionType:    0,
			ActionCommand: 0,
			ActionHold:    false,
		},
		4: {
			Name:          "Middle Button",
			Default:       true,
			ActionType:    0,
			ActionCommand: 0,
			ActionHold:    false,
		},
		2: {
			Name:          "Right Button",
			Default:       true,
			ActionType:    0,
			ActionCommand: 0,
			ActionHold:    false,
		},
		1: {
			Name:          "Left Button",
			Default:       true,
			ActionType:    0,
			ActionCommand: 0,
			ActionHold:    false,
		},
	}
}

var errAssignmentRepairNotPersisted = errors.New("assignment repair not persisted; replay deferred")

// Read/upgrade a candidate before activating its profile. A failed assignment
// write cannot publish profile linkage or partially replace live bindings.
// Caller holds lightingMu and lightingMutation.
func (d *Device) readLightingAssignments(p DeviceProfile) (map[int]inputmanager.KeyAssignment, error) {
	path := pwd + d.keyAssignmentFile
	if p.KeyAssignmentHash != "" {
		path = filepath.Join(pwd, "database", "key-assignments", p.KeyAssignmentHash+".json")
	}
	data, err := os.ReadFile(path)
	missing := os.IsNotExist(err)
	if err != nil && !missing {
		return nil, err
	}
	proposal := defaultWirelessAssignments()
	var original map[int]inputmanager.KeyAssignment
	changed := missing
	if !missing {
		proposal = nil
		if err = json.Unmarshal(data, &proposal); err != nil {
			return nil, err
		}
		if proposal == nil {
			return nil, fmt.Errorf("empty input assignments")
		}
		original = make(map[int]inputmanager.KeyAssignment, len(proposal))
		for key, value := range proposal {
			original[key] = value
		}
		left, found := proposal[1]
		if !found {
			left = defaultWirelessAssignments()[1]
			changed = true
		}
		if !left.Default {
			left.Default = true
			changed = true
		}
		proposal[1] = left
	}
	if !missing {
		for k, v := range defaultWirelessAssignments() {
			if _, ok := proposal[k]; !ok {
				proposal[k] = v
				changed = true
			}
		}
	}
	if changed {
		data, err = json.MarshalIndent(proposal, "", "    ")
		if err != nil {
			return nil, err
		}
		if err = d.persistLightingAssignments(path, data); err != nil {
			// Return only what was actually persisted. Repair failure must stay
			// observable, without dropping the user's surviving custom bindings.
			if original != nil {
				return original, errors.Join(errAssignmentRepairNotPersisted, err)
			}
			return nil, err
		}
	}
	return proposal, nil
}
func (d *Device) loadCanonicalAssignments() error { return d.loadCanonicalAssignmentsForSession(nil) }

// A reconnect retries only the known repair-persistence deferral, under the same
// mutation/state locks as ordinary loading. Unknown/invalid maps take no new path.
func (d *Device) loadCanonicalAssignmentsForSession(expected *connectionSession) error {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if expected != nil {
		if !d.sessionValidLocked(*expected) || d.Exit {
			return errLightingSessionRetired
		}
		if !d.assignmentReplayDeferred {
			return nil
		}
	}
	if d.DeviceProfile == nil {
		return fmt.Errorf("profile unavailable")
	}
	proposal, err := d.readLightingAssignments(*d.DeviceProfile)
	if err == nil || (proposal != nil && errors.Is(err, errAssignmentRepairNotPersisted)) {
		if !sameAssignmentPayload(d.KeyAssignment, proposal) {
			d.assignmentRevision++
		}
		d.KeyAssignment = proposal
		d.assignmentReplayDeferred = err != nil
		// Repair only changes assignment payloads, not setup preference snapshots.
		if expected == nil {
			d.inputRevision++
		}
	}
	return err
}
func (d *Device) inputAssignmentsSnapshot() map[int]inputmanager.KeyAssignment {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	copy := make(map[int]inputmanager.KeyAssignment, len(d.KeyAssignment))
	for k, v := range d.KeyAssignment {
		copy[k] = v
	}
	return copy
}
func (d *Device) inputAssignment(id int) (inputmanager.KeyAssignment, bool) {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	v, ok := d.KeyAssignment[id]
	return v, ok
}
func (d *Device) setInputTiltToggle(id int, toggle bool) {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if v, ok := d.KeyAssignment[id]; ok {
		v.TiltToggle = toggle
		d.KeyAssignment[id] = v
	}
}

// Activation can require a source assignment upgrade before the two profile
// markers are written. Restore just that assignment file if activation fails.
func (d *Device) prepareLightingAssignments(p DeviceProfile) (map[int]inputmanager.KeyAssignment, func(), error) {
	path := pwd + d.keyAssignmentFile
	if p.KeyAssignmentHash != "" {
		path = filepath.Join(pwd, "database", "key-assignments", p.KeyAssignmentHash+".json")
	}
	before, err := os.ReadFile(path)
	missing := os.IsNotExist(err)
	if err != nil && !missing {
		return nil, nil, err
	}
	proposal, err := d.readLightingAssignments(p)
	if err != nil {
		return nil, nil, err
	}
	undo := func() {
		// No write is necessary when the reader did not change the file.
		after, readErr := os.ReadFile(path)
		if !missing && readErr == nil && string(before) == string(after) {
			return
		}
		var restoreErr error
		if missing {
			restoreErr = os.Remove(path)
			if os.IsNotExist(restoreErr) {
				restoreErr = nil
			}
		} else {
			restoreErr = d.persistLightingAssignments(path, before)
		}
		if restoreErr != nil {
			logger.Log(logger.Fields{"error": restoreErr}).Error("Unable to restore input assignments after failed profile activation")
		}
	}
	return proposal, undo, nil
}

// Snapshot the complete W input packet before programming; never read a mutable
// profile or assignment map while waiting on the shared receiver.
var errInvalidAssignments = errors.New("invalid peripheral assignments")

type assignmentApplyRequest struct {
	session  connectionSession
	profile  *DeviceProfile
	key      int
	ready    bool
	bindings map[int]inputmanager.KeyAssignment
}

func (d *Device) assignmentRequestCurrentLocked(r assignmentApplyRequest) bool {
	return d.DeviceProfile == r.profile && d.sessionValidLocked(r.session) && r.session.assignment == d.assignmentRevision && d.lightingOutputReadyLocked()
}

func (d *Device) programCanonicalAssignments() error { return d.programAssignmentsForSession(nil) }
func (d *Device) programAssignmentsForEdit(request assignmentApplyRequest) error {
	return d.programAssignmentTransaction(nil, &request)
}

// expected is non-nil during setup, before Connected/output readiness. Every
// attempt takes a fresh assignment/gesture snapshot for the same receiver epoch.
func (d *Device) programAssignmentsForSession(expected *connectionSession) error {
	return d.programAssignmentTransaction(expected, nil)
}

// Only an explicit edited tilt requires activation confirmation; wake and unrelated
// button edits keep best-effort tilt compatibility after the required main payload.
func (d *Device) programAssignmentTransaction(expected *connectionSession, edit *assignmentApplyRequest) error {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	if (expected == nil && !d.lightingOutputReadyLocked()) || (expected != nil && (d.Exit || !d.sessionValidLocked(*expected) || d.softwareModeGeneration != expected.generation)) {
		d.lightingMu.Unlock()
		return errLightingSessionRetired
	}
	if edit != nil && !d.assignmentRequestCurrentLocked(*edit) {
		d.lightingMu.Unlock()
		return errLightingSessionRetired
	}
	session := d.sessionLocked()
	bindings := d.KeyAssignment
	if edit != nil {
		session, bindings = edit.session, edit.bindings
	}
	assignments := make(map[int]inputmanager.KeyAssignment, len(bindings))
	keys := make([]int, 0, len(bindings))
	for k, v := range bindings {
		assignments[k] = v
		keys = append(keys, k)
	}
	d.lightingMu.Unlock()
	if len(keys) != keyAmount {
		return errInvalidAssignments
	}
	write := func(command, data []byte, cleanup bool) error {
		if expected != nil {
			return d.writeSetupInputCommand(session, command, data, cleanup)
		}
		return d.writeAssignmentCommand(session, command, data, cleanup)
	}
	sort.Ints(keys)
	data := []byte{}
	for _, k := range keys {
		v := assignments[k]
		if !v.IsTilt {
			b := byte(0)
			if v.Default {
				b = 1
			}
			data = append(data, b)
		}
	}
	if err := write(cmdOpenWriteEndpoint, nil, false); err != nil {
		// transfer writes before waiting for its reply. An errored open may
		// already have taken effect; best-effort close is bound to THIS session.
		_ = write(cmdCloseEndpoint, nil, true)
		return err
	}
	payload := make([]byte, len(data)+headerWriteSize)
	binary.LittleEndian.PutUint16(payload[:2], uint16(len(data)))
	copy(payload[headerWriteSize:], data)
	payloadErr := write(cmdWrite, payload, false)
	// Still under deviceLock: cleanup cannot close another input transaction.
	// It ignores input revision but NEVER ignores the original connection epoch.
	closeErr := write(cmdCloseEndpoint, nil, true)
	if payloadErr != nil {
		return payloadErr
	}
	if closeErr != nil {
		return closeErr
	}
	var activationErr error
	commands := [][]byte{cmdActivateLeftTilt, cmdActivateRightTilt, cmdActivateFrontTilt, cmdActivateBackTilt}
	for _, k := range keys {
		v := assignments[k]
		if v.IsTilt && v.TiltIndex >= 0 && v.TiltIndex < len(commands) {
			b := byte(0)
			if !v.Default {
				b = 1
			}
			var tiltErr error
			for attempt := 0; attempt < 3; attempt++ {
				tiltErr = write(commands[v.TiltIndex], []byte{b}, false)
				if tiltErr == nil {
					break
				}
				if errors.Is(tiltErr, errLightingSessionRetired) {
					return tiltErr
				}
				if attempt < 2 && !lightingDelay(session.cancel, 40*time.Millisecond) {
					return errLightingSessionRetired
				}
			}
			if tiltErr != nil {
				if edit != nil && k == edit.key {
					activationErr = errors.Join(activationErr, tiltErr)
				}
				logger.Log(logger.Fields{"error": tiltErr, "tilt": v.TiltIndex}).Warn("Main mouse assignments applied; optional tilt activation failed")
			}
		}
	}
	if edit != nil {
		d.lightingMu.Lock()
		current := d.assignmentRequestCurrentLocked(*edit)
		d.lightingMu.Unlock()
		if !current {
			return errLightingSessionRetired
		}
	}
	return activationErr
}
func (d *Device) programCanonicalSleep() uint8 {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	if !d.lightingOutputReadyLocked() || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		return 0
	}
	session := d.sessionLocked()
	minutes := d.DeviceProfile.SleepMode
	d.lightingMu.Unlock()
	if err := d.writeTimerCommand(session, cmdOpenSleepWriteEndpoint, nil); err != nil {
		return 0
	}
	firstErr := d.writeTimerCommand(session, cmdSleep[0], []byte{0xe0, 0x93, 0x04, 0x00})
	if errors.Is(firstErr, errLightingSessionRetired) {
		return 0
	}
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, uint32(minutes*60*1000))
	// A reply timeout does not prove the first report was unapplied. The final
	// report revalidates timer/profile/receiver identity before sending duration.
	secondErr := d.writeTimerCommand(session, cmdSleep[1], data)
	if err := errors.Join(firstErr, secondErr); err != nil {
		logger.Log(logger.Fields{"error": err}).Warn("Unable to fully program mouse sleep timer")
		return 0
	}
	return 1
}

// DPI work is invalidated by effective value changes (A), not future regular
// selection under Sniper (B), colors (C), or profile snapshots (D). Profile
// activation explicitly invalidates old work (E); connection identity covers F.
func effectiveDPIValue(profile DeviceProfile, sniper bool) (uint16, bool) {
	if sniper {
		for _, stage := range profile.Profiles {
			if stage.Sniper {
				return uint16(common.Clamp(int(stage.Value), minDpiValue, maxDpiValue)), true
			}
		}
		return 0, false
	}
	stage, ok := profile.Profiles[profile.Profile]
	if !ok {
		return 0, false
	}
	return uint16(common.Clamp(int(stage.Value), minDpiValue, maxDpiValue)), true
}
func sameEffectiveDPI(first, second DeviceProfile, sniper bool) bool {
	old, oldOK := effectiveDPIValue(first, sniper)
	next, nextOK := effectiveDPIValue(second, sniper)
	return old == next && oldOK == nextOK
}

// Called under the existing mutation/state locks, before any assignment/profile
// write. Explicit ProfileName metadata lets collision suffixes reload correctly.
func (d *Device) availableProfilePathLocked(base string) (string, error) {
	for suffix := 1; ; suffix++ {
		candidate := base
		if suffix > 1 {
			candidate = strings.TrimSuffix(base, ".json") + fmt.Sprintf("-%d.json", suffix)
		}
		owned := false
		for _, profile := range d.UserProfiles {
			if profile != nil && filepath.Clean(profile.Path) == filepath.Clean(candidate) {
				owned = true
				break
			}
		}
		if owned {
			continue
		}
		if _, err := os.Lstat(candidate); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", err
		}
		return candidate, nil
	}
}

// Only source-generated named snapshots are removable. Never delete the shared
// package default, or a hash still referenced by any surviving profile.
func (d *Device) cleanupSupersededAssignmentsLocked(oldHash, newHash string) {
	if oldHash == "" || oldHash == newHash || len(oldHash) != 32 {
		return
	}
	if _, err := hex.DecodeString(oldHash); err != nil {
		return
	}
	for _, profile := range d.UserProfiles {
		if profile != nil && profile.KeyAssignmentHash == oldHash {
			return
		}
	}
	if d.DeviceProfile != nil && d.DeviceProfile.KeyAssignmentHash == oldHash {
		return
	}
	path := filepath.Join(pwd, "database", "key-assignments", oldHash+".json")
	if filepath.Clean(path) == filepath.Clean(pwd+d.keyAssignmentFile) {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Log(logger.Fields{"error": err}).Warn("Unable to remove superseded assignment snapshot")
	}
}

// inputEventMu protects concrete pressed actions across profile replacement.
func (d *Device) sendTiltHold(kind uint8, key uint16, pressed bool) {
	if d.tiltHoldWrite != nil {
		d.tiltHoldWrite(kind, key, pressed)
		return
	}
	if kind == 9 {
		inputmanager.InputControlMouseHold(key, pressed)
	} else {
		inputmanager.InputControlKeyboardHold(key, pressed)
	}
}
func (d *Device) releaseTiltHolds() {
	d.inputEventMu.Lock()
	defer d.inputEventMu.Unlock()
	for id, action := range d.heldTilts {
		switch action.ActionType {
		case 1, 3, 9:
			d.sendTiltHold(action.ActionType, action.ActionCommand, false)
		case 8:
			d.lightingMu.Lock()
			available := d.Connected && !d.Exit
			if !available {
				d.SniperMode = false
			}
			d.lightingMu.Unlock()
			if available {
				d.sniperMode(false)
			}
		}
		d.setInputTiltToggle(id, false)
		delete(d.heldTilts, id)
	}
}

// Caller owns inputEventMu for edge/toggle bookkeeping. Macro playback is allowed
// to block, so release only that lock while executing its already-resolved action.
func (d *Device) runInputMacroUnlocked(command uint16, playback func()) {
	d.inputEventMu.Unlock()
	defer d.inputEventMu.Lock()
	if d.inputMacroPlayback != nil {
		d.inputMacroPlayback(command)
		return
	}
	playback()
}

// Legacy W accepted new inputs 2..6 and replayed stored 1..7 verbatim. Preserve
// the formerly selectable 5 on replay without advertising it as a new choice.
func (d *Device) canReplayLiftHeight(value int) bool {
	_, advertised := d.LiftHeights[value]
	return advertised || value == 5
}
