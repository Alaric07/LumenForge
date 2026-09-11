package vanguard96WU

import (
	"LumenForge/src/keyboards"
	"LumenForge/src/rgb"
	"testing"
)

func TestVanguard96WUModernWorkspaceSourceConstants(t *testing.T) {
	if keyboardKey != "vanguard96W-default" || defaultLayout != "vanguard96W-default-US" {
		t.Fatalf("keyboard=%q layout=%q", keyboardKey, defaultLayout)
	}
}

func TestVanguard96WUFlashTapSnapshotFlattensMultipleRows(t *testing.T) {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{1: {KeyName: "A", KeyData: []uint16{4}}, 2: {KeyName: "No data"}}}, 1: {Keys: map[int]keyboards.Key{1: {KeyName: "D", KeyData: []uint16{7}}, 2: {KeyName: "Fn", KeyData: []uint16{130}}, 3: {KeyName: "Only color", OnlyColor: true, KeyData: []uint16{8}}}}}}
	d := &Device{Serial: "usb", DeviceProfile: &DeviceProfile{Profile: "default", Keyboards: map[string]*keyboards.Keyboard{"default": keyboard}, FlashTap: &keyboards.FlashTap{Active: 1, Mode: 1, Keys: map[int]keyboards.FlashTapKey{0: {Name: "A", KeyData: 4}, 1: {Name: "D", KeyData: 7}}, Color: rgb.Color{}}}, FlashTapModes: map[int]string{0: "Neutral", 1: "Last Priority", 2: "First Priority"}}
	snapshot, ok := d.FlashTapSnapshot()
	if !ok || len(snapshot.Keys) != 5 || len(snapshot.SelectedSlots) != 2 {
		t.Fatalf("snapshot=%#v ok=%t", snapshot, ok)
	}
	seen := map[int]bool{}
	for _, key := range snapshot.Keys {
		if seen[key.KeyIndex] {
			t.Fatalf("duplicate flat index: %#v", snapshot.Keys)
		}
		seen[key.KeyIndex] = true
		if key.KeyName == "Fn" || key.KeyName == "Only color" || key.KeyName == "No data" {
			if key.Eligible {
				t.Fatalf("excluded key became eligible: %#v", key)
			}
		}
	}
}
