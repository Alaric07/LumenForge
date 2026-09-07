package k95platinumXT

import "testing"

func TestPerformanceSnapshotUsesK95PlatinumXTOptionsAndFourSettings(t *testing.T) {
	d := &Device{DeviceProfile: &DeviceProfile{PollingRate: 4, DisableWinKey: true, DisableAltTab: true}, PollingRates: map[int]string{4: "1000 Hz", 1: "125 Hz"}}
	s, ok := d.PerformanceSnapshot()
	if !ok || s.PollingRate == nil || len(s.PollingRate.Options) != 2 || s.PollingRate.Options[0].Value != 1 || len(s.BooleanSettings) != 4 || !s.BooleanSettings[0].Enabled || s.BooleanSettings[1].Enabled || !s.BooleanSettings[2].Enabled || s.BooleanSettings[3].Enabled {
		t.Fatalf("snapshot = %#v, ok=%t", s, ok)
	}
	d.PollingRates[4] = ""
	if s, ok := d.PerformanceSnapshot(); ok || s.PollingRate != nil {
		t.Fatalf("malformed polling snapshot = %#v, ok=%t", s, ok)
	}
	d.PollingRates = map[int]string{1: "125 Hz"}
	if s, ok := d.PerformanceSnapshot(); ok || s.PollingRate != nil {
		t.Fatalf("missing selected option snapshot = %#v, ok=%t", s, ok)
	}
}
