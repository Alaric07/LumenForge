package psuhid

import "testing"

func TestPSUSnapshotExposesCompletePSUOverview(t *testing.T) {
	d := &Device{Serial: "psu-hid", FanModes: testPSUFanModes(), DeviceProfile: &DeviceProfile{FanMode: 6}, Devices: map[int]*Devices{
		1: {Name: "Fan 1", HasSpeed: true, Rpm: 820},
		2: {Name: "VRM Temperature", HasTemps: true, TemperatureString: "36.5 °C"},
		3: {Name: "PSU Temperature", HasTemps: true, TemperatureString: "39.0 °C"},
		4: {Name: "Power Out", Output: true, HasWatts: true, Watts: 420},
		5: {Name: "12V Rail", Rail: true, HasWatts: true, HasAmps: true, HasVolts: true, Watts: 390, Amps: 32.5, Volts: 12},
		6: {Name: "5V Rail", Rail: true, HasWatts: true, HasAmps: true, HasVolts: true, Watts: 20, Amps: 4, Volts: 5},
		7: {Name: "3V Rail", Rail: true, HasWatts: true, HasAmps: true, HasVolts: true, Watts: 10, Amps: 3, Volts: 3.3},
	}}
	snapshot, ok := d.PSUSnapshot()
	if !ok || snapshot.Fan.RPM != "820 RPM" || snapshot.PowerOut != "420 W" || len(snapshot.Temperatures) != 2 || snapshot.Temperatures[0].Value != "36.5 °C" || snapshot.Temperatures[1].Value != "39.0 °C" {
		t.Fatalf("snapshot = %#v, ok=%t", snapshot, ok)
	}
	if len(snapshot.Fan.Options) != 8 || snapshot.Fan.Options[0].Value != 0 || snapshot.Fan.Options[1].Value != 4 || snapshot.Fan.Options[7].Value != 10 || snapshot.Fan.Mode != 6 {
		t.Fatalf("fan options = %#v", snapshot.Fan)
	}
	if len(snapshot.Rails) != 3 || snapshot.Rails[0].Label != "12V Rail" || snapshot.Rails[1].Label != "5V Rail" || snapshot.Rails[2].Label != "3V Rail" || snapshot.Rails[0].Watts != "390 W" || snapshot.Rails[0].Amps != "32.5 A" || snapshot.Rails[0].Volts != "12 V" {
		t.Fatalf("rails = %#v", snapshot.Rails)
	}
}

func TestPSUSnapshotFailsClosedForIncompleteState(t *testing.T) {
	d := &Device{Serial: "psu-hid", FanModes: testPSUFanModes(), DeviceProfile: &DeviceProfile{FanMode: 6}, Devices: map[int]*Devices{}}
	if _, ok := d.PSUSnapshot(); ok {
		t.Fatal("incomplete PSU state produced a snapshot")
	}
}

func testPSUFanModes() map[int]string {
	return map[int]string{0: "Default", 4: "40 %", 5: "50 %", 6: "60 %", 7: "70 %", 8: "80 %", 9: "90 %", 10: "100 %"}
}
