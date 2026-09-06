package m55W

import "testing"

func TestM55WWorkspaceSnapshotsFailClosed(t *testing.T) {
	var d *Device
	if _, ok := d.DPISnapshot(); ok {
		t.Fatal("nil DPI snapshot accepted")
	}
	if _, ok := d.ButtonsSnapshot(); ok {
		t.Fatal("nil buttons snapshot accepted")
	}
	if _, ok := d.PerformanceSnapshot(); ok {
		t.Fatal("nil performance snapshot accepted")
	}
	if _, ok := d.DeviceProfileSnapshot(); ok {
		t.Fatal("nil profile snapshot accepted")
	}
	if _, ok := d.SleepTimerSnapshot(); ok {
		t.Fatal("nil sleep timer snapshot accepted")
	}
	if d.SelectMouseDPIStage(0) != 0 || d.SetMouseSniperMode(true) != 0 || d.SaveMouseDPISettings(nil, nil) != 0 {
		t.Fatal("nil DPI mutation accepted")
	}
}
