package k70max

import (
	"LumenForge/src/flashtappresentation"
	"LumenForge/src/keyactuationpresentation"
	"LumenForge/src/keyboards"
	"math"
	"sort"
	"strings"
)

func (d *Device) KeyActuationDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) KeyActuationSnapshot() (keyactuationpresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" {
		return keyactuationpresentation.Snapshot{}, false
	}
	k := d.getCurrentKeyboard()
	if k == nil || len(k.Row) == 0 {
		return keyactuationpresentation.Snapshot{}, false
	}
	s := keyactuationpresentation.Snapshot{Supported: true, MinValue: 1, MaxValue: 40, SecondaryMinimumGap: 4}
	keys := map[int]keyboards.Key{}
	ids := make([]int, 0)
	for _, row := range k.Row {
		if len(row.Keys) == 0 {
			return keyactuationpresentation.Snapshot{}, false
		}
		for id, key := range row.Keys {
			if _, exists := keys[id]; exists {
				return keyactuationpresentation.Snapshot{}, false
			}
			keys[id] = key
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	for _, id := range ids {
		key := keys[id]
		supported := !key.OnlyColor && !key.NoActuation && (len(key.KeyData) == 0 || !hasNoActuation(key.KeyData[0]))
		if supported && (key.ActuationPoint < 1 || key.ActuationPoint > 40 || key.ActuationResetPoint < 1 || key.ActuationResetPoint >= key.ActuationPoint || key.EnableSecondaryActuationPoint && (key.SecondaryActuationPoint < key.ActuationPoint+4 || key.SecondaryActuationPoint > 40 || key.SecondaryActuationResetPoint < 1 || key.SecondaryActuationResetPoint >= key.SecondaryActuationPoint)) {
			return keyactuationpresentation.Snapshot{}, false
		}
		s.Keys = append(s.Keys, keyactuationpresentation.Key{KeyIndex: id, KeyName: key.KeyName, Supported: supported, ActuationPoint: key.ActuationPoint, ActuationResetPoint: key.ActuationResetPoint, EnableActuationPointReset: key.EnableActuationPointReset, EnableSecondaryActuationPoint: key.EnableSecondaryActuationPoint, SecondaryActuationPoint: key.SecondaryActuationPoint, SecondaryActuationResetPoint: key.SecondaryActuationResetPoint})
	}
	return s, len(s.Keys) > 0
}

func (d *Device) FlashTapDeviceID() string {
	if d == nil {
		return ""
	}
	return d.Serial
}
func (d *Device) FlashTapSnapshot() (flashtappresentation.Snapshot, bool) {
	if d == nil || d.DeviceProfile == nil || d.DeviceProfile.Profile == "" || d.DeviceProfile.FlashTap == nil || len(d.FlashTapModes) == 0 {
		return flashtappresentation.Snapshot{}, false
	}
	keyboard := d.getCurrentKeyboard()
	if keyboard == nil || len(keyboard.Row) == 0 {
		return flashtappresentation.Snapshot{}, false
	}
	flashTap := d.DeviceProfile.FlashTap
	if flashTap.Active < 0 || flashTap.Active > 1 || math.IsNaN(flashTap.Color.Red) || math.IsNaN(flashTap.Color.Green) || math.IsNaN(flashTap.Color.Blue) || math.IsInf(flashTap.Color.Red, 0) || math.IsInf(flashTap.Color.Green, 0) || math.IsInf(flashTap.Color.Blue, 0) {
		return flashtappresentation.Snapshot{}, false
	}
	modeIDs := make([]int, 0, len(d.FlashTapModes))
	for id, label := range d.FlashTapModes {
		if strings.TrimSpace(label) == "" {
			return flashtappresentation.Snapshot{}, false
		}
		modeIDs = append(modeIDs, id)
	}
	sort.Ints(modeIDs)
	if _, ok := d.FlashTapModes[flashTap.Mode]; !ok {
		return flashtappresentation.Snapshot{}, false
	}
	byData := map[uint16]int{}
	byIndex := map[int]flashtappresentation.Key{}
	for _, row := range keyboard.Row {
		if len(row.Keys) == 0 {
			return flashtappresentation.Snapshot{}, false
		}
		for index, key := range row.Keys {
			if key.KeyName == "" {
				return flashtappresentation.Snapshot{}, false
			}
			if _, exists := byIndex[index]; exists {
				return flashtappresentation.Snapshot{}, false
			}
			eligible := !key.OnlyColor && len(key.KeyData) == 1 && !hasNoFlashTap(key.KeyData[0])
			if eligible {
				if _, exists := byData[key.KeyData[0]]; exists {
					return flashtappresentation.Snapshot{}, false
				}
				byData[key.KeyData[0]] = index
			}
			byIndex[index] = flashtappresentation.Key{KeyIndex: index, KeyName: key.KeyName, Eligible: eligible}
		}
	}
	keys := make([]flashtappresentation.Key, 0, len(byIndex))
	for _, key := range byIndex {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].KeyIndex < keys[j].KeyIndex })
	if len(flashTap.Keys) != 2 {
		return flashtappresentation.Snapshot{}, false
	}
	selected := map[int]bool{}
	slots := make([]flashtappresentation.SelectedSlot, 0, 2)
	for slot := 0; slot < 2; slot++ {
		key, exists := flashTap.Keys[slot]
		if !exists {
			return flashtappresentation.Snapshot{}, false
		}
		index, ok := byData[uint16(key.KeyData)]
		if !ok || selected[index] {
			return flashtappresentation.Snapshot{}, false
		}
		selected[index] = true
		slots = append(slots, flashtappresentation.SelectedSlot{SlotIndex: slot, KeyIndex: index})
	}
	for index := range keys {
		keys[index].Selected = selected[keys[index].KeyIndex]
	}
	snapshot := flashtappresentation.Snapshot{Supported: true, Active: flashTap.Active == 1, Mode: flashTap.Mode, Keys: keys, SelectedSlots: slots, Color: flashtappresentation.Color{Red: flashTap.Color.Red, Green: flashTap.Color.Green, Blue: flashTap.Color.Blue}}
	for _, id := range modeIDs {
		snapshot.Modes = append(snapshot.Modes, flashtappresentation.Option{Value: id, Label: d.FlashTapModes[id]})
	}
	return snapshot, len(snapshot.Keys) > 0
}
