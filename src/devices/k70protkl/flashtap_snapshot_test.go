package k70protkl

import (
	"LumenForge/src/flashtappresentation"
	"LumenForge/src/keyboards"
	"LumenForge/src/rgb"
	"math"
	"reflect"
	"testing"
)

func flashTapTestDevice(active int) *Device {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{7: {KeyName: "D", KeyData: []uint16{7}}, 4: {KeyName: "A", KeyData: []uint16{4}}, 57: {KeyName: "Fn", KeyData: []uint16{57}}}}}}
	return &Device{Serial: "k70-pro-tkl", FlashTapModes: map[int]string{0: "Standard", 1: "Prioritize", 2: "Last input"}, DeviceProfile: &DeviceProfile{Profile: "Default", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, FlashTap: &keyboards.FlashTap{Active: active, Mode: 1, Keys: map[int]keyboards.FlashTapKey{0: {Name: "A", KeyData: 4}, 1: {Name: "D", KeyData: 7}}, Color: rgb.Color{Red: 17, Green: 93, Blue: 201}}}}
}
func TestFlashTapSnapshotPreservesK70ProTKLState(t *testing.T) {
	d := flashTapTestDevice(1)
	s, ok := d.FlashTapSnapshot()
	if !ok || !s.Supported || !s.Active || s.Mode != 1 || len(s.Modes) != 3 || len(s.Keys) != 3 || !s.Keys[0].Selected || s.Keys[0].KeyIndex != 4 || !s.Keys[1].Selected || s.Keys[1].KeyIndex != 7 || s.Keys[2].Eligible || len(s.SelectedSlots) != 2 || s.SelectedSlots[0] != (flashtappresentation.SelectedSlot{SlotIndex: 0, KeyIndex: 4}) || s.SelectedSlots[1] != (flashtappresentation.SelectedSlot{SlotIndex: 1, KeyIndex: 7}) || s.Color != (flashtappresentation.Color{Red: 17, Green: 93, Blue: 201}) {
		t.Fatalf("snapshot=%#v ok=%t", s, ok)
	}
	if _, exposed := reflect.TypeOf(flashtappresentation.SelectedSlot{}).FieldByName("KeyData"); exposed {
		t.Fatal("selected slot exposed hardware KeyData")
	}
	d.DeviceProfile.FlashTap.Active = 0
	if s, ok := d.FlashTapSnapshot(); !ok || s.Active {
		t.Fatalf("disabled snapshot=%#v ok=%t", s, ok)
	}
}

func TestFlashTapSnapshotPreservesPersistedSlotOrderForK70ProTKL(t *testing.T) {
	d := flashTapTestDevice(1)
	d.DeviceProfile.FlashTap.Keys = map[int]keyboards.FlashTapKey{0: {Name: "D", KeyData: 7}, 1: {Name: "A", KeyData: 4}}
	s, ok := d.FlashTapSnapshot()
	if !ok || len(s.SelectedSlots) != 2 || s.SelectedSlots[0] != (flashtappresentation.SelectedSlot{SlotIndex: 0, KeyIndex: 7}) || s.SelectedSlots[1] != (flashtappresentation.SelectedSlot{SlotIndex: 1, KeyIndex: 4}) {
		t.Fatalf("snapshot=%#v ok=%t", s, ok)
	}
}

func TestFlashTapSnapshotFailsClosedForInvalidK70ProTKLState(t *testing.T) {
	if _, ok := (*Device)(nil).FlashTapSnapshot(); ok {
		t.Fatal("nil device")
	}
	for _, mutate := range []func(*Device){func(d *Device) { d.DeviceProfile = nil }, func(d *Device) { d.DeviceProfile.Keyboards = nil }, func(d *Device) { d.DeviceProfile.FlashTap = nil }, func(d *Device) { d.FlashTapModes[1] = "" }, func(d *Device) { d.DeviceProfile.FlashTap.Color.Red = math.NaN() }, func(d *Device) { delete(d.DeviceProfile.FlashTap.Keys, 1) }, func(d *Device) { d.DeviceProfile.FlashTap.Keys[1] = keyboards.FlashTapKey{KeyData: 57} }, func(d *Device) { d.DeviceProfile.FlashTap.Keys[1] = keyboards.FlashTapKey{KeyData: 99} }, func(d *Device) { d.DeviceProfile.FlashTap.Keys[1] = keyboards.FlashTapKey{KeyData: 4} }, func(d *Device) { d.DeviceProfile.FlashTap.Keys[2] = keyboards.FlashTapKey{KeyData: 7} }} {
		copy := flashTapTestDevice(1)
		mutate(copy)
		if _, ok := copy.FlashTapSnapshot(); ok {
			t.Fatal("accepted malformed FlashTap state")
		}
	}
}
