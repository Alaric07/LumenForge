package k70rgbRF

import "testing"

func TestPerformanceSnapshotK70RGBRF(t *testing.T) {
	d := &Device{DeviceProfile: &DeviceProfile{PollingRate: 1, DisableWinKey: true}, PollingRates: map[int]string{1: "1000 Hz", 8: "125 Hz"}}
	s, ok := d.PerformanceSnapshot()
	if !ok || len(s.BooleanSettings) != 4 || s.PollingRate.Options[0].Value != 1 {
		t.Fatalf("snapshot=%#v ok=%t", s, ok)
	}
	d.PollingRates[1] = ""
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("accepted blank option")
	}
}
