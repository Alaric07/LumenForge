package k70protkl

import (
	"LumenForge/src/keyboards"
	"testing"
)

func actuationTestDevice() *Device {
	k := &keyboards.Keyboard{Row: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{4: {KeyName: "A", KeyData: []uint16{4}, ActuationPoint: 20, ActuationResetPoint: 19, EnableActuationPointReset: true, EnableSecondaryActuationPoint: true, SecondaryActuationPoint: 35, SecondaryActuationResetPoint: 34}, 57: {KeyName: "Fn", KeyData: []uint16{41}}}}}}
	return &Device{Serial: "k70", DeviceProfile: &DeviceProfile{Profile: "Default", Keyboards: map[string]*keyboards.Keyboard{"Default": k}}}
}
func TestKeyActuationSnapshotPreservesK70ProTKLState(t *testing.T) {
	s, ok := actuationTestDevice().KeyActuationSnapshot()
	if !ok || !s.Supported || s.MinValue != 1 || s.MaxValue != 40 || s.SecondaryMinimumGap != 4 || len(s.Keys) != 2 {
		t.Fatalf("snapshot=%#v ok=%t", s, ok)
	}
	if key := s.Keys[0]; key.KeyIndex != 4 || key.KeyName != "A" || !key.Supported || key.ActuationPoint != 20 || key.ActuationResetPoint != 19 || !key.EnableActuationPointReset || !key.EnableSecondaryActuationPoint || key.SecondaryActuationPoint != 35 || key.SecondaryActuationResetPoint != 34 {
		t.Fatalf("key=%#v", key)
	}
	if s.Keys[1].Supported {
		t.Fatalf("unsupported key=%#v", s.Keys[1])
	}
}
func TestKeyActuationSnapshotFailsClosedForInvalidState(t *testing.T) {
	if _, ok := (*Device)(nil).KeyActuationSnapshot(); ok {
		t.Fatal("nil device")
	}
	for _, mutate := range []func(*Device){func(d *Device) { d.DeviceProfile = nil }, func(d *Device) { d.DeviceProfile.Keyboards = nil }, func(d *Device) { d.DeviceProfile.Keyboards["Default"].Row[0] = keyboards.Row{} }, func(d *Device) {
		x := d.DeviceProfile.Keyboards["Default"].Row[0].Keys[4]
		x.ActuationPoint = 0
		d.DeviceProfile.Keyboards["Default"].Row[0].Keys[4] = x
	}, func(d *Device) {
		x := d.DeviceProfile.Keyboards["Default"].Row[0].Keys[4]
		x.ActuationResetPoint = 20
		d.DeviceProfile.Keyboards["Default"].Row[0].Keys[4] = x
	}, func(d *Device) {
		x := d.DeviceProfile.Keyboards["Default"].Row[0].Keys[4]
		x.SecondaryActuationPoint = 23
		d.DeviceProfile.Keyboards["Default"].Row[0].Keys[4] = x
	}, func(d *Device) {
		x := d.DeviceProfile.Keyboards["Default"].Row[0].Keys[4]
		x.SecondaryActuationResetPoint = 35
		d.DeviceProfile.Keyboards["Default"].Row[0].Keys[4] = x
	}} {
		d := actuationTestDevice()
		mutate(d)
		if _, ok := d.KeyActuationSnapshot(); ok {
			t.Fatal("accepted invalid actuation state")
		}
	}
}
