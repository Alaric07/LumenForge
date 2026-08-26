package mm800

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

func TestMM800AuthoredZoneMutationPersistsAndRespectsOwnership(t *testing.T) {
	for _, test := range []struct {
		name              string
		cluster, external bool
		wantRestart       bool
	}{
		{"local mousepad", false, false, true},
		{"RGB Cluster", true, false, false},
		{"OpenRGB integration", false, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			device, runtime := newMM800CanonicalLightingTestDevice(t)
			prepareMM800AuthoredMutationPersistence(t, device)
			device.DeviceProfile.Mousepad = mm800AuthoredMutationMousepad()
			device.DeviceProfile.RGBCluster, device.DeviceProfile.OpenRGBIntegration = test.cluster, test.external
			if err := runtime.State.Set(device.Serial, lightingsettings.IndependentDeviceLightingState{SelectedEffect: "mousepad", Brightness: 100}); err != nil {
				t.Fatal(err)
			}
			restarts := 0
			device.lightingRestart = func() { restarts++ }
			before := mm800AuthoredMetadata(device.DeviceProfile.Mousepad)

			if err := device.SetLightingZoneColor("mousepad", "zone", "1", "", rgb.Color{Red: 1, Green: 2, Blue: 3}); err != nil {
				t.Fatal(err)
			}
			if got := device.DeviceProfile.Mousepad.Row[1].Zones[1].Color; got.Red != 1 || got.Green != 2 || got.Blue != 3 {
				t.Fatalf("zone color = %#v", got)
			}
			if got := device.DeviceProfile.Mousepad.Row[1].Zones[2].Color; got.Red == 1 && got.Green == 2 && got.Blue == 3 {
				t.Fatalf("unselected zone changed: %#v", got)
			}
			if restarts != boolToInt(test.wantRestart) {
				t.Fatalf("restarts after zone = %d", restarts)
			}

			if err := device.SetLightingZoneColor("mousepad", "group", "", "2", rgb.Color{Red: 4, Green: 5, Blue: 6}); err != nil {
				t.Fatal(err)
			}
			for id, zone := range device.DeviceProfile.Mousepad.Row[2].Zones {
				if zone.Color.Red != 4 || zone.Color.Green != 5 || zone.Color.Blue != 6 {
					t.Fatalf("group zone %d = %#v", id, zone.Color)
				}
			}
			if got := device.DeviceProfile.Mousepad.Row[1].Zones[1].Color; got.Red != 1 || got.Green != 2 || got.Blue != 3 {
				t.Fatalf("other row changed: %#v", got)
			}

			if err := device.SetLightingZoneColor("mousepad", "all", "", "", rgb.Color{Red: 7, Green: 8, Blue: 9}); err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, row := range device.DeviceProfile.Mousepad.Row {
				for _, zone := range row.Zones {
					count++
					if zone.Color.Red != 7 || zone.Color.Green != 8 || zone.Color.Blue != 9 {
						t.Fatalf("all mutation left %#v", zone.Color)
					}
				}
			}
			if count != 15 {
				t.Fatalf("zone count = %d, want 15", count)
			}
			if restarts != 3*boolToInt(test.wantRestart) {
				t.Fatalf("restarts = %d", restarts)
			}
			if got := mm800AuthoredMetadata(device.DeviceProfile.Mousepad); !reflect.DeepEqual(got, before) {
				t.Fatalf("metadata changed: before %#v, after %#v", before, got)
			}
			var persisted DeviceProfile
			data, err := os.ReadFile(filepath.Join(pwd, "database", "profiles", device.Serial+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &persisted); err != nil {
				t.Fatal(err)
			}
			if persisted.Mousepad.Row[1].Zones[1].Color.Hex != "#070809" || persisted.Mousepad.Row[2].Zones[8].Color.Hex != "#070809" {
				t.Fatalf("persisted authored colors = %#v", persisted.Mousepad)
			}
		})
	}
}

func TestMM800AuthoredZoneMutationRejectsInvalidSelections(t *testing.T) {
	device, _ := newMM800CanonicalLightingTestDevice(t)
	prepareMM800AuthoredMutationPersistence(t, device)
	device.DeviceProfile.Mousepad = mm800AuthoredMutationMousepad()
	for _, args := range [][4]string{{"mousepad", "zone", "99", ""}, {"mousepad", "group", "", "99"}, {"static", "all", "", ""}} {
		if err := device.SetLightingZoneColor(args[0], args[1], args[2], args[3], rgb.Color{}); err == nil {
			t.Fatalf("mutation succeeded for %#v", args)
		}
	}
}

func TestMM800AuthoredZoneMultipleMutationIsAtomic(t *testing.T) {
	device, _ := newMM800CanonicalLightingTestDevice(t)
	prepareMM800AuthoredMutationPersistence(t, device)
	device.DeviceProfile.Mousepad = mm800AuthoredMutationMousepad()
	if err := device.SetLightingZoneColors("mousepad", []string{"1", "8"}, rgb.Color{Red: 9}); err != nil {
		t.Fatal(err)
	}
	if device.DeviceProfile.Mousepad.Row[1].Zones[1].Color.Red != 9 || device.DeviceProfile.Mousepad.Row[2].Zones[8].Color.Red != 9 || device.DeviceProfile.Mousepad.Row[1].Zones[2].Color.Red == 9 {
		t.Fatal("multiple-zone mutation did not affect only requested zones")
	}
	if err := device.SetLightingZoneColors("mousepad", []string{"2", "99"}, rgb.Color{Green: 9}); err == nil {
		t.Fatal("unknown zone mutation succeeded")
	}
	if device.DeviceProfile.Mousepad.Row[1].Zones[2].Color.Green == 9 {
		t.Fatal("invalid multi-zone mutation partially changed state")
	}
}

type mm800ZoneMetadata struct {
	Name                     string
	Left, Top, Width, Height int
	PacketIndex              []int
}

func mm800AuthoredMetadata(mousepad *Mousepad) map[int]mm800ZoneMetadata {
	result := map[int]mm800ZoneMetadata{}
	for _, row := range mousepad.Row {
		for id, zone := range row.Zones {
			result[id] = mm800ZoneMetadata{zone.Name, zone.Left, zone.Top, zone.Width, zone.Height, append([]int(nil), zone.PacketIndex...)}
		}
	}
	return result
}
func mm800AuthoredMutationMousepad() *Mousepad {
	rows := map[int]Row{1: {Zones: map[int]Zones{}}, 2: {Zones: map[int]Zones{}}}
	for id := 1; id <= 15; id++ {
		row := 1
		if id > 7 {
			row = 2
		}
		rows[row].Zones[id] = Zones{Name: "Zone", Left: id, Top: row, Width: 2, Height: 3, PacketIndex: []int{id}, Color: rgb.Color{Red: float64(id)}}
	}
	return &Mousepad{Row: rows}
}
func prepareMM800AuthoredMutationPersistence(t *testing.T, device *Device) {
	t.Helper()
	previousReload := reloadMM800ProfilesAfterSave
	reloadMM800ProfilesAfterSave = func(*Device) {}
	t.Cleanup(func() { reloadMM800ProfilesAfterSave = previousReload })
	root := t.TempDir()
	previous := pwd
	pwd = root
	t.Cleanup(func() { pwd = previous })
	if err := os.MkdirAll(filepath.Join(root, "database", "profiles"), 0755); err != nil {
		t.Fatal(err)
	}
	device.DeviceProfile.Path = filepath.Join(root, "database", "profiles", device.Serial+".json")
}
func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
