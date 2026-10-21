package glaivergb

import (
	"LumenForge/src/common"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Input remains profile-owned. Copy before persistence so profile switching or
// the retained mixed color endpoint cannot expose partially edited maps.
func (d *Device) mutateInputProfile(change func(*DeviceProfile) error) uint8 {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	if d.Exit || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	p := cloneLightingProfile(*d.DeviceProfile)
	err := change(&p)
	if err == nil {
		err = d.persistLightingProfile(p)
	}
	if err == nil {
		current := d.DeviceProfile
		*current = p
	}
	d.lightingMu.Unlock()
	d.lightingMutation.Unlock()
	return lightingResult(err)
}
func (d *Device) saveCanonicalDPI(stages map[int]uint16, colors map[int]rgb.Color) uint8 {
	result := d.mutateInputProfile(func(p *DeviceProfile) error {
		if len(stages) == 0 {
			return fmt.Errorf("empty DPI values")
		}
		// The workspace method supplies a complete color/value set; the older method
		// permits a valid subset and skips invalid stages, as before.
		full := colors != nil
		if full && (p.DPIColor == nil || p.SniperColor == nil || len(stages) != len(p.Profiles) || len(colors) != len(p.Profiles)) {
			return fmt.Errorf("incomplete input state")
		}
		var regular, sniper *rgb.Color
		count := 0
		for id, value := range stages {
			stage, ok := p.Profiles[id]
			if !ok || value < 100 || value > 16000 {
				if full {
					return fmt.Errorf("invalid DPI stage")
				}
				continue
			}
			if full {
				c, found := colors[id]
				if !found || !glaiveRGBColorOK(c) {
					return fmt.Errorf("invalid DPI color")
				}
				if stage.Sniper {
					copy := c
					sniper = &copy
				} else if regular == nil {
					copy := c
					regular = &copy
				} else if !glaiveRGBSame(*regular, c) {
					return fmt.Errorf("regular colors must agree")
				}
			}
			stage.Value = value
			p.Profiles[id] = stage
			count++
		}
		if count == 0 {
			return fmt.Errorf("no valid DPI stages")
		}
		if full {
			if regular == nil || sniper == nil {
				return fmt.Errorf("missing input colors")
			}
			p.DPIColor = glaiveRGBColor(*regular)
			p.SniperColor = glaiveRGBColor(*sniper)
		}
		return nil
	})
	if result == 1 {
		d.programCanonicalDPI(true)
		d.programCanonicalDPI(false)
	}
	return result
}
func (d *Device) selectCanonicalDPI(stage int) uint8 {
	result := d.mutateInputProfile(func(p *DeviceProfile) error {
		v, ok := p.Profiles[stage]
		if !ok || v.Sniper {
			return fmt.Errorf("invalid stage")
		}
		p.Profile = stage
		return nil
	})
	d.lightingMu.Lock()
	sniper := d.SniperMode
	d.lightingMu.Unlock()
	if result == 1 && !sniper {
		d.programCanonicalDPI(false)
	}
	return result
}
func (d *Device) rotateCanonicalDPI() {
	if d.mutateInputProfile(func(p *DeviceProfile) error {
		if p.Profile >= 5 {
			p.Profile = 1
		} else {
			p.Profile++
		}
		return nil
	}) == 1 {
		d.programCanonicalDPI(false)
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
	changed := d.SniperMode != active
	if found {
		d.SniperMode = active
	}
	d.lightingMu.Unlock()
	if !found {
		return 0
	}
	if changed {
		d.programCanonicalDPI(false, active)
	}
	return 1
}

// Exact Glaive DPI records: selector 208+key, X/Y values at3..6, RGB7..9 zero.
// No input color, Sniper overlay or flash is added to the Lighting packet.
func (d *Device) programCanonicalDPI(all bool, sniperSelection ...bool) {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()
	d.lightingMu.Lock()
	if d.Exit || !d.Connected || d.DeviceProfile == nil || (d.dev == nil && d.lightingDPIWrite == nil) {
		d.lightingMu.Unlock()
		return
	}
	p := cloneLightingProfile(*d.DeviceProfile)
	d.lightingMu.Unlock()
	write := d.lightingDPIWrite
	if write == nil {
		write = func(command, data []byte) error { _, err := d.transfer(cmdWrite, command, data); return err }
	}
	if all {
		for id, stage := range p.Profiles {
			data := make([]byte, 10)
			data[0] = byte(208 + id)
			binary.LittleEndian.PutUint16(data[3:5], stage.Value)
			binary.LittleEndian.PutUint16(data[5:7], stage.Value)
			if err := write(cmdSaveDpi, data); err != nil {
				logger.Log(logger.Fields{"error": err}).Error("Unable to set DPI")
			}
		}
	} else {
		stage := byte(p.Profile)
		if len(sniperSelection) != 0 && sniperSelection[0] {
			stage = 0
		}
		if err := write(cmdSetDpi, []byte{stage}); err != nil {
			logger.Log(logger.Fields{"error": err}).Error("Unable to select DPI")
		}
	}
}

func (d *Device) saveCanonicalUserProfile(name string) uint8 {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.Exit || d.DeviceProfile == nil || !common.AlphanumericDashRegex.MatchString(name) {
		return 0
	}
	p := cloneLightingProfile(*d.DeviceProfile)
	p.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-"+name+".json")
	p.Active = false
	p.KeyAssignmentHash = common.GenerateRandomMD5()
	// Save input assignments first, then publish the named profile. Neither changes
	// the current profile, ownership session, canonical desired data or renderer.
	assignments, err := json.MarshalIndent(d.KeyAssignment, "", "    ")
	if err != nil {
		return 0
	}
	assignmentPath := filepath.Join(pwd, "database", "key-assignments", p.KeyAssignmentHash+".json")
	if err = writeLightingProfile(assignmentPath, assignments); err != nil {
		return 0
	}
	if err = d.persistLightingProfile(p); err != nil {
		_ = os.Remove(assignmentPath)
		return 0
	}
	if d.UserProfiles == nil {
		d.UserProfiles = make(map[string]*DeviceProfile)
	}
	d.UserProfiles[name] = &p
	d.rebuildProfileOrder()
	return 1
}

func (d *Device) rotateCanonicalProfile() {
	d.lightingMu.Lock()
	name := ""
	if d.DeviceProfile != nil && len(d.ProfileOrder) > 0 {
		current := ""
		for key, p := range d.UserProfiles {
			if p == d.DeviceProfile {
				current = key
				break
			}
		}
		name = d.ProfileOrder[0]
		for i, key := range d.ProfileOrder {
			if key == current {
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
