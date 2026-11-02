package m65prorgb

import (
	"LumenForge/src/common"
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
				if !ok || stage.Color == nil || !m65ProRGBColorOK(color) {
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
		// Legacy SaveMouseDPI programs every slot; combined workspace settings only
		// selects the active slot. Keep this source distinction.
		if colors == nil {
			d.programCanonicalDPI(true)
		}
		d.toggleDPI()
	}
	return result
}
func (d *Device) saveCanonicalDPIColors(dpi rgb.Color, colors map[int]rgb.Color) uint8 {
	if !m65ProRGBColorOK(dpi) {
		return 0
	}
	result := d.mutateInputProfile(false, func(p *DeviceProfile) error {
		changed := 0
		for id, color := range colors {
			stage, ok := p.Profiles[id]
			if !ok || stage.Color == nil || !m65ProRGBColorOK(color) {
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

// Exact PRO DPI slot records: 209+key, Sniper 208, X/Y at 3..6,
// authored input color at 7..9. Selection uses regular key+1 or Sniper zero.
// Indicator selector 3 belongs to the separate color packet, not an effect zone.
func (d *Device) programCanonicalDPI(all bool, sniperSelection ...bool) {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	if !d.lightingOutputReadyLocked() || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		return
	}
	p := cloneLightingProfile(*d.DeviceProfile)
	d.lightingMu.Unlock()
	write := d.lightingDPIWrite
	if write == nil {
		if d.dev == nil {
			return
		}
		write = func(command, data []byte) error { return d.transfer(cmdWrite, command, data) }
	}
	if all {
		for id, stage := range p.Profiles {
			if stage.Color == nil {
				continue
			}
			data := make([]byte, 10)
			data[0] = byte(209 + id)
			if stage.Sniper {
				data[0] = 208
			}
			binary.LittleEndian.PutUint16(data[3:5], stage.Value)
			binary.LittleEndian.PutUint16(data[5:7], stage.Value)
			data[7], data[8], data[9] = byte(stage.Color.Red), byte(stage.Color.Green), byte(stage.Color.Blue)
			if err := write(cmdSaveDpi, data); err != nil {
				logger.Log(logger.Fields{"error": err}).Error("Unable to program DPI")
			}
		}
	} else {
		selected := byte(p.Profile + 1)
		if len(sniperSelection) > 0 && sniperSelection[0] {
			selected = 0
		}
		if err := write(cmdSetDpi, []byte{selected}); err != nil {
			logger.Log(logger.Fields{"error": err}).Error("Unable to select DPI")
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
	if replacingActive {
		// Keep the active file too: default uses <serial>.json, not a named path.
		proposal.Path = existing.Path
	}
	proposal.Active = replacingActive
	proposal.KeyAssignmentHash = common.GenerateRandomMD5()
	data, err := json.MarshalIndent(d.KeyAssignment, "", "    ")
	if err != nil {
		return 0
	}
	assignmentPath := filepath.Join(pwd, "database", "key-assignments", proposal.KeyAssignmentHash+".json")
	if err = writeLightingProfile(assignmentPath, data); err != nil {
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
		if d.timer != nil {
			d.timer.Stop()
		}
		if d.autoRefreshChan != nil {
			close(d.autoRefreshChan)
		}
		if hardware && d.dev != nil {
			d.deviceLock.Lock()
			d.setHardwareMode()
			_ = d.dev.Close()
			d.deviceLock.Unlock()
		}
	})
}

func (d *Device) programLightingPreference(angle bool) {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	if d.Exit || d.DeviceProfile == nil || d.dev == nil {
		d.lightingMu.Unlock()
		return
	}
	value := d.DeviceProfile.LiftHeight
	if angle {
		value = d.DeviceProfile.AngleSnapping
	}
	d.lightingMu.Unlock()
	if angle && (value < 0 || value > 1) {
		return
	}
	if !angle && (value < 2 || value > 4) {
		return
	}
	for i := 0; i <= 1; i++ {
		command := cmdLiftHeight[i]
		if angle {
			command = cmdAngleSnapping[i]
		}
		if err := d.transfer(cmdWrite, command, []byte{byte(value)}); err != nil {
			logger.Log(logger.Fields{"error": err}).Error("Unable to apply input preference")
		}
	}
}

// Mirror saveDeviceProfile's existing non-Lighting normalization on activation.
// In particular, the historical upgrade Sniper differs from the fresh default.
func prepareLightingInputProfile(profile DeviceProfile) DeviceProfile {
	p := cloneLightingProfile(profile)
	if p.SleepMode == 0 {
		p.SleepMode = 15
	}
	if p.LiftHeight == 0 {
		p.LiftHeight = 3
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
