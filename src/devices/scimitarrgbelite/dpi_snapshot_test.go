package scimitarrgbelite

import (
	"testing"

	"LumenForge/src/rgb"
)

func TestScimitarEliteDPISnapshotUsesCurrentDeviceProfile(t *testing.T) {
	profile := &DeviceProfile{Profile: 1, Profiles: map[int]DPIProfile{
		2: {Name: "Sniper", Value: 400, Color: &rgb.Color{Red: 255, Green: 170, Blue: 0}, Sniper: true},
		0: {Name: "Stage 1", Value: 800, Color: &rgb.Color{Red: 1, Green: 2, Blue: 3}},
		1: {Name: "Stage 2", Value: 1600, Color: &rgb.Color{Red: 16, Green: 32, Blue: 48}},
	}}
	device := &Device{Serial: "elite-dpi", MinDPI: 100, MaxDPI: 18000, DeviceProfile: profile}

	snapshot, ok := device.DPISnapshot()
	if !ok || snapshot.MinimumDPI != 100 || snapshot.MaximumDPI != 18000 || snapshot.ActiveRegularStageID != "1" || len(snapshot.Stages) != 3 {
		t.Fatalf("DPI snapshot = %#v, ok=%t", snapshot, ok)
	}
	if snapshot.Stages[0].ID != "0" || snapshot.Stages[0].Name != "Stage 1" || snapshot.Stages[0].DPI != 800 || snapshot.Stages[0].ColorHex != "#010203" || snapshot.Stages[0].Sniper || snapshot.Stages[0].Active {
		t.Fatalf("first DPI stage = %#v", snapshot.Stages[0])
	}
	if snapshot.Stages[1].ID != "1" || snapshot.Stages[1].Name != "Stage 2" || snapshot.Stages[1].DPI != 1600 || snapshot.Stages[1].ColorHex != "#102030" || snapshot.Stages[1].Sniper || !snapshot.Stages[1].Active {
		t.Fatalf("active regular DPI stage = %#v", snapshot.Stages[1])
	}
	if snapshot.Stages[2].ID != "2" || !snapshot.Stages[2].Sniper || snapshot.Stages[2].Active || snapshot.Stages[2].ColorHex != "#ffaa00" {
		t.Fatalf("Sniper DPI stage = %#v", snapshot.Stages[2])
	}
	if profile.Profile != 1 || profile.Profiles[1].Value != 1600 || profile.Profiles[1].Color.Red != 16 {
		t.Fatalf("DPI snapshot mutated DeviceProfile: %#v", profile)
	}

	device.SniperMode = true
	snapshot, ok = device.DPISnapshot()
	if !ok || !snapshot.Stages[1].Active || !snapshot.Stages[2].Active {
		t.Fatalf("Sniper-active DPI snapshot = %#v, ok=%t", snapshot, ok)
	}
}
