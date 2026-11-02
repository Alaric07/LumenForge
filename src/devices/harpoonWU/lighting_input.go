package harpoonWU

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
	"strings"
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
func (d *Device) saveCanonicalMouseColors(dpi rgb.Color, zones map[int]rgb.Color) uint8 {
	// WU's legacy dpi parameter was validated but never applied by this endpoint.
	if !harpoonWUColorOK(dpi) {
		return 0
	}
	updates := make(map[string]lightingsettings.Color, len(zones))
	for id, color := range zones {
		updates[strconv.Itoa(id)] = lightingColor(color)
	}
	return lightingResult(d.updateLightingPalette("mouse", updates))
}

func (d *Device) saveCanonicalDPI(stages map[int]uint16, colors map[int]rgb.Color, connected bool) uint8 {
	result := d.mutateInputProfile(connected, func(p *DeviceProfile) error {
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
				if !ok || stage.Color == nil || !harpoonWUColorOK(color) {
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
	if !harpoonWUColorOK(dpi) {
		return 0
	}
	result := d.mutateInputProfile(false, func(p *DeviceProfile) error {
		changed := 0
		for id, color := range colors {
			stage, ok := p.Profiles[id]
			if !ok || stage.Color == nil || !harpoonWUColorOK(color) {
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
func (d *Device) rotateCanonicalDPI() {
	if d.mutateInputProfile(false, func(p *DeviceProfile) error {
		if p.Profile >= 4 {
			p.Profile = 0
		} else {
			p.Profile++
		}
		return nil
	}) == 1 {
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
	for _, stage := range d.DeviceProfile.Profiles {
		if stage.Sniper {
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
	// WU initializes and releases to the selected regular DPI. Sniper only
	// substitutes the separate indicator; there is no Logo flash.
	d.programCanonicalDPI(active)
	d.setDeviceColor()
	return 1
}
func (d *Device) programCanonicalDPI(sniper bool) {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	if !d.lightingOutputReadyLocked() || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		return
	}
	stage := d.DeviceProfile.Profiles[d.DeviceProfile.Profile]
	if sniper {
		for _, s := range d.DeviceProfile.Profiles {
			if s.Sniper {
				stage = s
				break
			}
		}
	}
	value := stage.Value
	d.lightingMu.Unlock()
	if value < uint16(minDpiValue) {
		value = uint16(minDpiValue)
	}
	if value > uint16(maxDpiValue) {
		value = uint16(maxDpiValue)
	}
	data := make([]byte, 2)
	binary.LittleEndian.PutUint16(data, value)
	write := d.lightingDPIWrite
	if write == nil {
		if d.dev == nil {
			return
		}
		write = func(command, data []byte) error { _, err := d.transfer(command, data, true); return err }
	}
	if err := write(cmdSetDpi, data); err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to set DPI")
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

// Retain named input profiles without aliasing the current profile or importing
// any of their serialized Lighting fields into canonical desired state.
func (d *Device) saveCanonicalUserProfile(name string) uint8 {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.Exit || d.DeviceProfile == nil || !common.AlphanumericDashRegex.MatchString(name) {
		return 0
	}
	if existing, ok := d.UserProfiles[name]; ok && existing == d.DeviceProfile {
		return 0
	}
	proposal := cloneLightingProfile(*d.DeviceProfile)
	proposal.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-"+name+".json")
	proposal.Active = false
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
	d.UserProfiles[name] = &proposal
	d.rebuildProfileOrder()
	return 1
}

func (d *Device) programLightingPreference(angle bool) {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	if d.Exit || d.DeviceProfile == nil || (!d.lightingOutputReadyLocked() && !d.lightingInitializing) || d.dev == nil {
		d.lightingMu.Unlock()
		return
	}
	value := d.DeviceProfile.ButtonOptimization
	command := cmdButtonOptimization
	if angle {
		value = d.DeviceProfile.AngleSnapping
		command = cmdAngleSnapping
	}
	d.lightingMu.Unlock()
	if value < 0 || value > 1 {
		return
	}
	_, _ = d.transfer(command, []byte{byte(value)}, true)
}
func (d *Device) programLightingSleepTimer() uint8 {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	if !d.lightingOutputReadyLocked() || d.DeviceProfile == nil || d.dev == nil {
		d.lightingMu.Unlock()
		return 0
	}
	minutes := d.DeviceProfile.SleepMode
	d.lightingMu.Unlock()
	if _, err := d.transfer(cmdOpenSleepWriteEndpoint, nil, true); err != nil {
		return 0
	}
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, uint32(minutes*60*1000))
	changed := 0
	for i := 0; i < 2; i++ {
		if _, err := d.transfer(cmdSleep[i], data, true); err == nil {
			changed++
		}
	}
	if changed > 0 {
		return 1
	}
	return 0
}

func prepareLightingInputProfile(p DeviceProfile) DeviceProfile {
	p = cloneLightingProfile(p)
	if p.SleepMode == 0 {
		p.SleepMode = 15
	}
	if p.PollingRate == 0 {
		p.PollingRate = 4
	}
	found := false
	for _, stage := range p.Profiles {
		if strings.EqualFold(stage.Name, "Sniper") {
			found = true
			break
		}
	}
	if !found {
		if p.Profiles == nil {
			p.Profiles = map[int]DPIProfile{}
		}
		p.Profiles[len(p.Profiles)] = DPIProfile{Name: "Sniper", Value: 200, PackerIndex: 6, ColorIndex: map[int][]int{0: {0, 2, 4}}, Color: &rgb.Color{Red: 255, Green: 255, Brightness: 1, Hex: "#ffff00"}, Sniper: true}
	}
	return p
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
