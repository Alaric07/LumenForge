package scimitarrgbelite

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"LumenForge/src/rgb"
)

func TestScimitarEliteSaveMouseDPISettingsIsAtomic(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "database", "profiles"), 0o755); err != nil { t.Fatal(err) }
	previousPWD := pwd
	pwd = root
	t.Cleanup(func() { pwd = previousPWD })
	persisted := 0
	previousReload := reloadScimitarEliteProfilesAfterSave
	reloadScimitarEliteProfilesAfterSave = func(*Device) { persisted++ }
	t.Cleanup(func() { reloadScimitarEliteProfilesAfterSave = previousReload })
	profile := &DeviceProfile{Path: filepath.Join(root, "database", "profiles", "elite.json"), Profile: 1, PollingRate: 2, AngleSnapping: 1, LiftHeight: 3, Profiles: map[int]DPIProfile{
		0: {Name: "Stage 1", Value: 800, Color: &rgb.Color{Red: 1, Green: 2, Blue: 3, Hex: "#010203"}},
		1: {Name: "Stage 2", Value: 1600, Color: &rgb.Color{Red: 4, Green: 5, Blue: 6, Hex: "#040506"}},
	}}
	device := &Device{DeviceProfile: profile, Exit: true}
	before := *profile
	before.Profiles = map[int]DPIProfile{}
	for key, value := range profile.Profiles { before.Profiles[key] = value }

	if got := device.SaveMouseDPISettings(map[int]uint16{0: 1200, 1: 50}, map[int]rgb.Color{0: {Red: 10, Green: 20, Blue: 30}, 1: {Red: 40, Green: 50, Blue: 60}}); got != 0 {
		t.Fatalf("invalid combined save = %d", got)
	}
	if persisted != 0 || !reflect.DeepEqual(profile, &before) { t.Fatalf("invalid combined save mutated profile=%#v persisted=%d", profile, persisted) }

	if got := device.SaveMouseDPISettings(map[int]uint16{0: 1200, 1: 2400}, map[int]rgb.Color{0: {Red: 10, Green: 20, Blue: 30}, 1: {Red: 40, Green: 50, Blue: 60}}); got != 1 {
		t.Fatalf("valid combined save = %d", got)
	}
	if persisted != 1 || profile.Profiles[0].Value != 1200 || profile.Profiles[1].Value != 2400 || profile.Profiles[0].Color.Hex != "#0a141e" || profile.Profiles[1].Color.Hex != "#28323c" {
		t.Fatalf("combined save profile=%#v persisted=%d", profile, persisted)
	}
	if profile.Profile != 1 || profile.PollingRate != 2 || profile.AngleSnapping != 1 || profile.LiftHeight != 3 {
		t.Fatalf("combined save changed unrelated profile state: %#v", profile)
	}
}

func TestScimitarEliteRegularDPISelectionPreservesSniperOverride(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "database", "profiles"), 0o755); err != nil { t.Fatal(err) }
	previousPWD := pwd
	pwd = root
	t.Cleanup(func() { pwd = previousPWD })
	previousReload := reloadScimitarEliteProfilesAfterSave
	reloadScimitarEliteProfilesAfterSave = func(*Device) {}
	t.Cleanup(func() { reloadScimitarEliteProfilesAfterSave = previousReload })
	profile := &DeviceProfile{Profile: 4, Profiles: map[int]DPIProfile{
		0: {Name: "Stage 1", Value: 800},
		1: {Name: "Stage 2", Value: 1200},
		2: {Name: "Stage 3", Value: 1600},
		3: {Name: "Stage 4", Value: 2400},
		4: {Name: "Stage 5", Value: 3200},
		5: {Name: "Sniper", Value: 400, Sniper: true},
	}}
	device := &Device{Serial: "elite-dpi-cycle", MinDPI: 100, MaxDPI: 18000, DeviceProfile: profile, SniperMode: true, Exit: true}

	device.ModifyDpi()
	if profile.Profile != 0 || !device.SniperMode { t.Fatalf("physical regular cycle changed Sniper state: profile=%d sniper=%t", profile.Profile, device.SniperMode) }
	if got := device.SelectMouseDPIStage(3); got != 1 || profile.Profile != 3 || !device.SniperMode {
		t.Fatalf("regular UI selection changed Sniper state: result=%d profile=%d sniper=%t", got, profile.Profile, device.SniperMode)
	}
	snapshot, ok := device.DPISnapshot()
	if !ok || snapshot.ActiveRegularStageID != "3" || !snapshot.Stages[3].Active || !snapshot.Stages[5].Active {
		t.Fatalf("Sniper-active snapshot = %#v, ok=%t", snapshot, ok)
	}
	device.sniperMode(false)
	if device.SniperMode || profile.Profile != 3 { t.Fatalf("Sniper deactivation did not restore selected regular stage: profile=%d sniper=%t", profile.Profile, device.SniperMode) }
	snapshot, ok = device.DPISnapshot()
	if !ok || !snapshot.Stages[3].Active || snapshot.Stages[5].Active { t.Fatalf("Sniper-inactive snapshot = %#v, ok=%t", snapshot, ok) }
}
