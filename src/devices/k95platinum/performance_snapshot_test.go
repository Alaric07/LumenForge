package k95platinum

import (
	"testing"

	"LumenForge/src/common"
)

func TestK95PerformanceSnapshotUsesDeviceProfileAndCopiesOptions(t *testing.T) {
	profile := &DeviceProfile{
		PollingRate: 2, DisableWinKey: true, DisableShiftTab: false, DisableAltTab: true, DisableAltF4: false,
	}
	pollingRates := map[int]string{8: "125 Hz", 1: "1000 Hz", 2: "500 Hz"}
	device := &Device{Serial: "k95-performance", DeviceProfile: profile, PollingRates: pollingRates}

	snapshot, ok := device.PerformanceSnapshot()
	if !ok || snapshot.PollingRate == nil {
		t.Fatalf("Performance snapshot = %#v, ok=%t", snapshot, ok)
	}
	if snapshot.PollingRate.Value != 2 {
		t.Errorf("Polling Rate value = %d, want 2", snapshot.PollingRate.Value)
	}
	if got := snapshot.PollingRate.Options; len(got) != 3 || got[0].Value != 1 || got[1].Value != 2 || got[2].Value != 8 {
		t.Errorf("Polling Rate options = %#v, want numeric sort", got)
	}
	wantSettings := []struct {
		id, label string
		enabled   bool
	}{
		{"perf_winKey", "Disable Win Key", true},
		{"perf_shiftTab", "Disable Shift + Tab", false},
		{"perf_altTab", "Disable Alt + Tab", true},
		{"perf_altF4", "Disable Alt + F4", false},
	}
	if len(snapshot.BooleanSettings) != len(wantSettings) {
		t.Fatalf("Boolean settings = %#v", snapshot.BooleanSettings)
	}
	for index, want := range wantSettings {
		got := snapshot.BooleanSettings[index]
		if got.ID != want.id || got.Label != want.label || got.Enabled != want.enabled {
			t.Errorf("Boolean setting %d = %#v, want %#v", index, got, want)
		}
	}

	pollingRates[1] = "changed"
	if snapshot.PollingRate.Options[0].Label != "1000 Hz" {
		t.Errorf("Snapshot options changed with source map: %#v", snapshot.PollingRate.Options)
	}
}

func TestK95PerformanceSnapshotFailsClosedWithoutProfile(t *testing.T) {
	device := &Device{Serial: "k95-performance", PollingRates: map[int]string{1: "1000 Hz"}}
	if snapshot, ok := device.PerformanceSnapshot(); ok || snapshot.PollingRate != nil || len(snapshot.BooleanSettings) != 0 {
		t.Fatalf("Performance snapshot without profile = %#v, ok=%t", snapshot, ok)
	}
}

func TestK95KeyboardPerformanceSaveEnablesPerformanceAndStoresValues(t *testing.T) {
	profile := &DeviceProfile{}
	device := &Device{DeviceProfile: profile}
	device.applyKeyboardPerformanceSave(common.KeyboardPerformanceData{WinKey: true, ShiftTab: true, AltTab: false, AltF4: true})
	if !profile.Performance || !profile.DisableWinKey || !profile.DisableShiftTab || profile.DisableAltTab || !profile.DisableAltF4 {
		t.Errorf("keyboard Performance save left profile %#v", profile)
	}
}
