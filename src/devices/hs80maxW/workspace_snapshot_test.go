package hs80maxW

import (
	"LumenForge/src/inputmanager"
	"testing"
)

func TestWorkspaceSnapshotsExposeOnlyCompleteHS80MAXCapabilities(t *testing.T) {
	labels := []string{"32", "64", "125", "250", "500", "1K", "2K", "4K", "8K", "16K"}
	profile := &DeviceProfile{SleepMode: 15, DisableMicIndicator: 0, SideTone: 1, SideToneValue: 50, Equalizers: map[int]Equalizer{}}
	for i, label := range labels {
		profile.Equalizers[i+1] = Equalizer{Name: label}
	}
	d := &Device{Serial: "hs80max", DeviceProfile: profile, UserProfiles: map[string]*DeviceProfile{"Default": {Active: true}}, SleepModes: map[int]string{0: "Off", 1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, MuteIndicators: map[int]string{0: "Disabled", 1: "Enabled"}, SideToneModes: map[int]string{0: "Disabled", 1: "Enabled"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 9: "Mouse", 10: "Macro", 11: "Profile Switch"}, KeyAssignment: map[int]inputmanager.KeyAssignment{1: {Name: "Scroll Press", Default: true}}}
	sleep, ok := d.SleepTimerSnapshot()
	if !ok || len(sleep.Options) != 7 || sleep.Options[0].Label != "Off" {
		t.Fatalf("sleep=%#v", sleep)
	}
	headset, ok := d.HeadsetSnapshot()
	if !ok {
		t.Fatal("headset snapshot unavailable")
	}
	if len(headset.Equalizer) != 10 || headset.Equalizer[0].Label != "32" || headset.Equalizer[9].Label != "16K" || headset.Sidetone == nil || headset.Sidetone.ValueRange.Minimum != 0 || headset.Sidetone.ValueRange.Maximum != 100 {
		t.Fatalf("headset=%#v", headset)
	}
	if len(headset.Assignments) != 1 || headset.Assignments[0].Label != "Scroll Press" || len(headset.Assignments[0].Types) != 6 {
		t.Fatalf("assignments=%#v", headset.Assignments)
	}
	d.KeyAssignment = nil
	if _, ok := d.HeadsetSnapshot(); ok {
		t.Fatal("incomplete assignment state exposed")
	}
}
