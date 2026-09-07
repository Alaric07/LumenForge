package k95

import "testing"

func TestPerformanceSnapshotUsesK95OptionsAndFourSettings(t *testing.T) {
	d := &Device{DeviceProfile: &DeviceProfile{PollingRate: 1, DisableWinKey: true, DisableAltTab: true}, PollingRates: map[int]string{8: "125 Hz", 1: "1000 Hz"}}
	s, ok := d.PerformanceSnapshot()
	if !ok || s.PollingRate == nil || len(s.PollingRate.Options) != 2 || s.PollingRate.Options[0].Value != 1 || len(s.BooleanSettings) != 4 || !s.BooleanSettings[0].Enabled || s.BooleanSettings[1].Enabled || !s.BooleanSettings[2].Enabled || s.BooleanSettings[3].Enabled {
		t.Fatalf("snapshot = %#v, ok=%t", s, ok)
	}
	d.PollingRates[1] = ""
	if s, ok := d.PerformanceSnapshot(); ok || s.PollingRate != nil {
		t.Fatalf("malformed polling snapshot = %#v, ok=%t", s, ok)
	}
	d.PollingRates = map[int]string{8: "125 Hz"}
	if s, ok := d.PerformanceSnapshot(); ok || s.PollingRate != nil {
		t.Fatalf("missing selected option snapshot = %#v, ok=%t", s, ok)
	}
}
