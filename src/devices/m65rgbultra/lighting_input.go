package m65rgbultra

import (
	"LumenForge/src/common"
	"LumenForge/src/inputmanager"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Input preferences retain profile backing. Copies keep indicator composition
// and profile replacement from observing partially edited stage/color maps.
func (d *Device) mutateInputProfile(connected bool, change func(*DeviceProfile) error) uint8 {
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
		*d.DeviceProfile = proposal
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
				if !ok || stage.Color == nil || !m65RGBUltraColorOK(color) {
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
	})
	if result == 1 {
		d.toggleDPI()
	}
	return result
}
func (d *Device) saveCanonicalDPIColors(dpi rgb.Color, colors map[int]rgb.Color) uint8 {
	if !m65RGBUltraColorOK(dpi) {
		return 0
	}
	result := d.mutateInputProfile(false, func(p *DeviceProfile) error {
		changed := 0
		for id, color := range colors {
			stage, ok := p.Profiles[id]
			if !ok || stage.Color == nil || !m65RGBUltraColorOK(color) {
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
		d.setDeviceColor()
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
	d.lightingMu.Unlock()
	if same {
		return 1
	}

	result := d.mutateInputProfile(false, func(p *DeviceProfile) error {
		value, ok := p.Profiles[stage]
		if !ok || value.Sniper {
			return fmt.Errorf("invalid DPI stage")
		}
		p.Profile = stage
		return nil
	})
	d.lightingMu.Lock()
	sniper := d.SniperMode
	d.lightingMu.Unlock()
	if result == 1 && !sniper {
		d.toggleDPI()
	}
	return result
}
func (d *Device) modifyCanonicalDPI(increment bool) {
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
	})
	if result == 1 {
		d.toggleDPI()
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
		d.SniperMode = active
	}
	d.lightingMu.Unlock()
	if !found {
		return 0
	}
	d.programCanonicalDPI(false, active)
	d.setDeviceColor()
	return 1
}

// Ultra programs the selected value into both axes; no saved slot table.
func (d *Device) programCanonicalDPI(_ bool, sniperSelection ...bool) {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	if !d.lightingOutputReadyLocked() || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		return
	}
	stage, ok := d.DeviceProfile.Profiles[d.DeviceProfile.Profile]
	if len(sniperSelection) > 0 && sniperSelection[0] {
		ok = false
		for _, p := range d.DeviceProfile.Profiles {
			if p.Sniper {
				stage = p
				ok = true
				break
			}
		}
	}
	d.lightingMu.Unlock()
	if !ok {
		return
	}
	value := common.Clamp(int(stage.Value), minDpiValue, maxDpiValue)
	data := make([]byte, 2)
	binary.LittleEndian.PutUint16(data, uint16(value))
	for axis := 0; axis < 2; axis++ {
		if err := d.writeLightingCommand(cmdSetDpi[axis], data); err != nil {
			logger.Log(logger.Fields{"error": err}).Error("Unable to program Ultra DPI")
		}
	}
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
	}
	proposal.Active = replacingActive
	proposal.KeyAssignmentHash = common.GenerateRandomMD5()
	data, err := json.MarshalIndent(d.KeyAssignment, "", "    ")
	if err != nil {
		return 0
	}
	assignmentPath := filepath.Join(pwd, "database", "key-assignments", proposal.KeyAssignmentHash+".json")
	if err = d.persistLightingAssignments(assignmentPath, data); err != nil {
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
		d.UserProfiles[name] = d.DeviceProfile
	} else {
		d.UserProfiles[name] = &proposal
	}
	d.rebuildProfileOrder()
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
	d.lightingMu.Unlock()
	if !found {
		return 0
	}
	if same {
		return 1
	}
	return d.canonicalSniper(active)
}

// Wired lifecycle: retire output sessions before mode/close; cancellation never
// waits for an RGB Exit receiver or for a worker blocked behind deviceLock.
func (d *Device) stopCanonicalLighting(hardware bool) {
	d.lightingStopOnce.Do(func() {
		d.lightingMutation.Lock()
		d.exitLighting()
		d.refreshLightingRegistrationsLocked(true, true)
		d.lightingMutation.Unlock()
		if d.timerKeepAlive != nil {
			d.timerKeepAlive.Stop()
		}
		if d.keepAliveChan != nil {
			close(d.keepAliveChan)
		}
		if hardware && d.dev != nil {
			d.deviceLock.Lock()
			d.setHardwareMode()
			_ = d.dev.Close()
			d.deviceLock.Unlock()
		}
	})
}

// Retain Ultra command bytes and raw input values.
func (d *Device) programLightingPreference(kind string) {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	if !d.lightingOutputReadyLocked() || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		return
	}
	p := cloneLightingProfile(*d.DeviceProfile)
	d.lightingMu.Unlock()
	switch kind {
	case "angle":
		if _, ok := d.SwitchModes[p.AngleSnapping]; ok && p.AngleSnapping >= 0 && p.AngleSnapping <= 1 {
			_ = d.writeLightingCommand(cmdAngleSnapping, []byte{byte(p.AngleSnapping)})
		}
	case "button":
		if _, ok := d.SwitchModes[p.ButtonOptimization]; ok && p.ButtonOptimization >= 0 && p.ButtonOptimization <= 1 {
			_ = d.writeLightingCommand(cmdButtonOptimization, []byte{byte(p.ButtonOptimization)})
		}
	case "lift":
		if _, ok := d.LiftHeights[p.LiftHeight]; ok {
			_ = d.writeLightingCommand(cmdLiftHeight, []byte{byte(p.LiftHeight)})
		}
	case "gestures":
		if p.MultiGesture < 0 || p.MultiGesture > 1 {
			return
		}
		_ = d.writeLightingCommand(cmdMultiGesture, []byte{byte(p.MultiGesture)})
		for i, c := range [][]byte{cmdLeftTilt, cmdRightTilt, cmdForwardTilt, cmdBackwardTilt} {
			if t, ok := p.Tilts[i]; ok {
				_ = d.writeLightingCommand(c, []byte{t.Value})
			}
		}
	}
}
func (d *Device) writeLightingCommand(command, data []byte) error {
	if d.lightingDPIWrite != nil {
		return d.lightingDPIWrite(append([]byte(nil), command...), append([]byte(nil), data...))
	}
	if d.dev == nil && d.lightingReportWrite == nil {
		return fmt.Errorf("HID unavailable")
	}
	_, err := d.transfer(command, data, "canonicalInput")
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
		d.KeyAssignment = proposal
		return 1
	}()
	if result == 1 && d.dev != nil {
		d.setupKeyAssignment()
	}
	return result
}

// Source defaults include four tilt bindings, with Ultra's original tilt indices.
func defaultUltraAssignments() map[int]inputmanager.KeyAssignment {
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
		16: {
			Name:          "Forward Button",
			Default:       true,
			ActionType:    0,
			ActionCommand: 0,
			ActionHold:    false,
		},
		8: {
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
	proposal := defaultUltraAssignments()
	if !missing {
		proposal = nil
		if err = json.Unmarshal(data, &proposal); err != nil {
			return nil, err
		}
		if proposal == nil {
			return nil, fmt.Errorf("empty input assignments")
		}
		left := proposal[1]
		left.Default = true
		proposal[1] = left
	}
	upgrade := !missing && len(proposal) < 19 // retain source upgrade trigger
	if upgrade {
		for k, v := range defaultUltraAssignments() {
			if v.IsTilt {
				if _, ok := proposal[k]; !ok {
					proposal[k] = v
				}
			}
		}
	}
	if missing || upgrade {
		data, err = json.MarshalIndent(proposal, "", "    ")
		if err != nil {
			return nil, err
		}
		if err = d.persistLightingAssignments(path, data); err != nil {
			return nil, err
		}
	}
	return proposal, nil
}
func (d *Device) loadCanonicalAssignments() error {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.DeviceProfile == nil {
		return fmt.Errorf("profile unavailable")
	}
	proposal, err := d.readLightingAssignments(*d.DeviceProfile)
	if err == nil {
		d.KeyAssignment = proposal
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
