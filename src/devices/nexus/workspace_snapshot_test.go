package nexus

import "testing"

func TestScreenSnapshotExposesSelectedAuthoredProfile(t *testing.T) {
	device := &Device{
		Serial:        "nexus-screen-test",
		HasLCD:        true,
		DeviceProfile: &DeviceProfile{LCDMode: "time-info"},
		LCDProfiles: &LCDProfiles{Profiles: map[string]Profile{
			"default":   {Name: "Default"},
			"time-info": {Name: "Time"},
			"cpu-info":  {Name: "CPU Info"},
		}},
	}

	snapshot, ok := device.ScreenSnapshot()
	if !ok || !snapshot.Available || snapshot.Selected != "time-info" {
		t.Fatalf("snapshot = %#v, ok = %t", snapshot, ok)
	}
	foundCPU, foundTime := false, false
	for _, option := range snapshot.Options {
		switch option.ID {
		case "default":
			t.Fatal("default profile was exposed")
		case "cpu-info":
			foundCPU = option.Label == "CPU Info" && !option.Selected
		case "time-info":
			foundTime = option.Label == "Time" && option.Selected
		}
	}
	if !foundCPU || !foundTime {
		t.Fatalf("options = %#v", snapshot.Options)
	}
}
