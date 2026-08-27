package scimitarrgbelite

import "testing"

func TestScimitarElitePerformanceSnapshotUsesDeviceProfileAndOptionMaps(t *testing.T) {
	profile := &DeviceProfile{PollingRate: 2, AngleSnapping: 1, LiftHeight: 3}
	pollingRates := map[int]string{8: "125 Hz", 1: "1000 Hz", 2: "500 Hz"}
	liftHeights := map[int]string{4: "High", 2: "Low", 3: "Medium"}
	device := &Device{Serial: "elite-performance", DeviceProfile: profile, PollingRates: pollingRates, LiftHeights: liftHeights}

	snapshot, ok := device.PerformanceSnapshot()
	if !ok || snapshot.PollingRate == nil || snapshot.AngleSnapping == nil || snapshot.LiftHeight == nil {
		t.Fatalf("Performance snapshot = %#v, ok=%t", snapshot, ok)
	}
	if snapshot.PollingRate.Value != 2 || len(snapshot.PollingRate.Options) != 3 ||
		snapshot.PollingRate.Options[0].Value != 1 || snapshot.PollingRate.Options[1].Value != 2 || snapshot.PollingRate.Options[2].Value != 8 {
		t.Fatalf("Polling Rate presentation = %#v", snapshot.PollingRate)
	}
	if !snapshot.AngleSnapping.Enabled {
		t.Fatalf("Angle Snapping presentation = %#v", snapshot.AngleSnapping)
	}
	if snapshot.LiftHeight.Value != 3 || len(snapshot.LiftHeight.Options) != 3 ||
		snapshot.LiftHeight.Options[0].Value != 2 || snapshot.LiftHeight.Options[1].Value != 3 || snapshot.LiftHeight.Options[2].Value != 4 {
		t.Fatalf("Lift Height presentation = %#v", snapshot.LiftHeight)
	}

	snapshot.PollingRate.Options[0].Label = "mutated"
	snapshot.LiftHeight.Options[0].Label = "mutated"
	if profile.PollingRate != 2 || profile.AngleSnapping != 1 || profile.LiftHeight != 3 || pollingRates[1] != "1000 Hz" || liftHeights[2] != "Low" {
		t.Fatalf("Performance snapshot mutated source state: profile=%#v polling=%#v lift=%#v", profile, pollingRates, liftHeights)
	}
}

func TestScimitarElitePerformanceSnapshotFailsClosedWithoutProfile(t *testing.T) {
	device := &Device{Serial: "elite-performance", PollingRates: map[int]string{1: "1000 Hz"}, LiftHeights: map[int]string{2: "Low"}}
	if snapshot, ok := device.PerformanceSnapshot(); ok || snapshot.PollingRate != nil || snapshot.AngleSnapping != nil || snapshot.LiftHeight != nil {
		t.Fatalf("Performance snapshot without profile = %#v, ok=%t", snapshot, ok)
	}
}
