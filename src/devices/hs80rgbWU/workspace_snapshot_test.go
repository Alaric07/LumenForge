package hs80rgbWU

import "testing"

func TestUSBWirelessWorkspaceCapabilitiesExcludeOffAndSidetone(t *testing.T) {
	labels := []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"}
	p := &DeviceProfile{Equalizers: map[int]Equalizer{}, SleepMode: 15, MuteIndicator: 0}
	for i, label := range labels {
		p.Equalizers[i+1] = Equalizer{Name: label}
	}
	d := &Device{Serial: "hs80wu", DeviceProfile: p, SleepModes: map[int]string{1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}, SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}}
	sleep, ok := d.SleepTimerSnapshot()
	if !ok || len(sleep.Options) != 6 {
		t.Fatalf("sleep=%#v", sleep)
	}
	for _, option := range sleep.Options {
		if option.Value == 0 {
			t.Fatal("USB wireless inferred Off")
		}
	}
	headset, ok := d.HeadsetSnapshot()
	if !ok || headset.MuteIndicator == nil {
		t.Fatalf("headset=%#v", headset)
	}
	for i, label := range labels {
		if headset.Equalizer[i].Label != label {
			t.Fatalf("band %d = %q", i, headset.Equalizer[i].Label)
		}
	}
}
