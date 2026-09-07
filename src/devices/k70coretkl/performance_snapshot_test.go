package k70coretkl

import "testing"

func TestPerformanceSnapshotK70CoreTKLFailsClosedForInvalidPollingOptions(t *testing.T) {
	d := &Device{DeviceProfile: &DeviceProfile{PollingRate: 4}, PollingRates: map[int]string{1: "125 Hz", 4: "1000 Hz"}}
	if snapshot, ok := d.PerformanceSnapshot(); !ok || snapshot.PollingRate == nil || len(snapshot.BooleanSettings) != 4 {
		t.Fatalf("snapshot=%#v ok=%t", snapshot, ok)
	}
	d.PollingRates[4] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("accepted blank polling option")
	}
	d.PollingRates = map[int]string{1: "125 Hz"}
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("accepted missing selected polling option")
	}
}
