package scimitarrgbelite

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"LumenForge/src/rgb"
)

func TestScimitarEliteAuthoredZoneMutationPersistsAndRespectsOwnership(t *testing.T) {
	for _, test := range []struct {
		name                           string
		cluster, external, wantRestart bool
	}{{"local mouse", false, false, true}, {"RGB Cluster", true, false, false}, {"OpenRGB integration", false, true, false}} {
		t.Run(test.name, func(t *testing.T) {
			profile := scimitarAuthoredMutationProfile()
			profile.RGBCluster, profile.OpenRGBIntegration = test.cluster, test.external
			device := &Device{Serial: "elite-authored-mutation", DeviceProfile: profile, lightingSource: authoredZoneLightingSource{}}
			prepareScimitarAuthoredMutationPersistence(t, device)
			restarts := 0
			device.lightingRestart = func() { restarts++ }
			beforeMetadata, beforeDPI := scimitarAuthoredMetadata(profile.ZoneColors), *profile.DPIColor
			if err := device.SetLightingZoneColor("mouse", "zone", "1", "", rgb.Color{Red: 1, Green: 2, Blue: 3}); err != nil {
				t.Fatal(err)
			}
			if got := profile.ZoneColors[1].Color; got.Red != 1 || got.Green != 2 || got.Blue != 3 {
				t.Fatalf("zone color = %#v", got)
			}
			if got := profile.ZoneColors[2].Color; got.Red == 1 && got.Green == 2 && got.Blue == 3 {
				t.Fatalf("unselected zone changed: %#v", got)
			}
			if err := device.SetLightingZoneColor("mouse", "all", "", "", rgb.Color{Red: 7, Green: 8, Blue: 9}); err != nil {
				t.Fatal(err)
			}
			for id, zone := range profile.ZoneColors {
				if zone.Color.Red != 7 || zone.Color.Green != 8 || zone.Color.Blue != 9 {
					t.Fatalf("zone %d = %#v", id, zone.Color)
				}
			}
			wantRestarts := 0
			if test.wantRestart {
				wantRestarts = 2
			}
			if restarts != wantRestarts {
				t.Fatalf("restarts = %d", restarts)
			}
			if got := scimitarAuthoredMetadata(profile.ZoneColors); !reflect.DeepEqual(got, beforeMetadata) {
				t.Fatalf("metadata changed: before %#v after %#v", beforeMetadata, got)
			}
			if *profile.DPIColor != beforeDPI {
				t.Fatalf("DPI state changed: %#v", profile.DPIColor)
			}
			var persisted DeviceProfile
			data, err := os.ReadFile(filepath.Join(pwd, "database", "profiles", device.Serial+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &persisted); err != nil {
				t.Fatal(err)
			}
			if len(persisted.ZoneColors) != 4 || persisted.ZoneColors[1].Color.Hex != "#070809" {
				t.Fatalf("persisted zones = %#v", persisted.ZoneColors)
			}
		})
	}
}

func TestScimitarEliteAuthoredZoneMultipleMutationIsAtomic(t *testing.T) {
	profile := scimitarAuthoredMutationProfile()
	device := &Device{Serial: "elite-authored-multiple", DeviceProfile: profile, lightingSource: authoredZoneLightingSource{}}
	prepareScimitarAuthoredMutationPersistence(t, device)
	device.lightingRestart = func() {}
	if err := device.SetLightingZoneColors("mouse", []string{"1", "4"}, rgb.Color{Red: 9}); err != nil {
		t.Fatal(err)
	}
	if profile.ZoneColors[1].Color.Red != 9 || profile.ZoneColors[4].Color.Red != 9 || profile.ZoneColors[2].Color.Red == 9 {
		t.Fatal("multiple-zone mutation did not affect only requested zones")
	}
	if err := device.SetLightingZoneColors("mouse", []string{"2", "99"}, rgb.Color{Green: 9}); err == nil {
		t.Fatal("unknown zone mutation succeeded")
	}
	if profile.ZoneColors[2].Color.Green == 9 {
		t.Fatal("invalid multi-zone mutation partially changed state")
	}
}

type scimitarZoneMetadata struct {
	Name                       string
	ColorIndex                 []int
	LEDIndex, LEDIndexPosition int
}

func scimitarAuthoredMetadata(zones map[int]ZoneColors) map[int]scimitarZoneMetadata {
	result := map[int]scimitarZoneMetadata{}
	for id, zone := range zones {
		result[id] = scimitarZoneMetadata{zone.Name, append([]int(nil), zone.ColorIndex...), zone.LEDIndex, zone.LEDIndexPosition}
	}
	return result
}
func scimitarAuthoredMutationProfile() *DeviceProfile {
	zones := map[int]ZoneColors{}
	for id := 1; id <= 4; id++ {
		zones[id] = ZoneColors{Name: "Zone", Color: &rgb.Color{Red: float64(id)}, ColorIndex: []int{id}, LEDIndex: id, LEDIndexPosition: id + 10}
	}
	return &DeviceProfile{ZoneColors: zones, DPIColor: &rgb.Color{Red: 91, Green: 92, Blue: 93}, Profiles: map[int]DPIProfile{}}
}
func prepareScimitarAuthoredMutationPersistence(t *testing.T, device *Device) {
	t.Helper()
	previousReload := reloadScimitarEliteProfilesAfterSave
	reloadScimitarEliteProfilesAfterSave = func(*Device) {}
	t.Cleanup(func() { reloadScimitarEliteProfilesAfterSave = previousReload })
	root := t.TempDir()
	previous := pwd
	pwd = root
	t.Cleanup(func() { pwd = previous })
	if err := os.MkdirAll(filepath.Join(root, "database", "profiles"), 0755); err != nil {
		t.Fatal(err)
	}
	device.DeviceProfile.Path = filepath.Join(root, "database", "profiles", device.Serial+".json")
}
