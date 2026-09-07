package k70lux

import "testing"

func TestPerformanceSnapshotK70LUX(t *testing.T) {
	d := &Device{DeviceProfile: &DeviceProfile{PollingRate: 1, DisableWinKey: true}, PollingRates: map[int]string{1: "1000 Hz", 8: "125 Hz"}}
	s, ok := d.PerformanceSnapshot()
	if !ok || s.PollingRate == nil || len(s.PollingRate.Options) != 2 || len(s.BooleanSettings) != 4 || !s.BooleanSettings[0].Enabled {
		t.Fatalf("snapshot=%#v ok=%t", s, ok)
	}
	d.PollingRates[1] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("accepted blank option")
	}
}
