package lightingsettings

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAuthoredZonesCanonicalPersistenceCopiesAndGenericIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "effects.json")
	store, err := LoadDeviceStore(path)
	if err != nil {
		t.Fatal(err)
	}
	palette := map[string]Color{"0": {Red: 200, Green: 100, Blue: 40}}
	if err = store.SetAuthoredZones("harpoon", "mouse", palette); err != nil {
		t.Fatal(err)
	}
	palette["0"] = Color{}
	got, found, err := store.GetAuthoredZones("harpoon", "mouse")
	if err != nil || !found || got["0"].Red != 200 {
		t.Fatal("input alias", got)
	}
	got["0"] = Color{}
	if err = store.Set("harpoon", "static", testStaticSettings(12)); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Delete("harpoon", "static"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadDeviceStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got, _, _ = reloaded.GetAuthoredZones("harpoon", "mouse")
	if got["0"].Red != 200 {
		t.Fatal("generic mutation lost palette", got)
	}
	if _, found, _ = reloaded.Get("harpoon", "static"); found {
		t.Fatal("Mouse conflated with Static")
	}
	oldData, _ := os.ReadFile(path)
	store.writer = writerFunc(func(string, []byte) error { return errors.New("failure") })
	if store.SetAuthoredZones("harpoon", "mouse", map[string]Color{"0": {}}) == nil {
		t.Fatal("failure hidden")
	}
	got, _, _ = store.GetAuthoredZones("harpoon", "mouse")
	if got["0"].Red != 200 {
		t.Fatal("failure published palette")
	}
	after, _ := os.ReadFile(path)
	if !reflect.DeepEqual(oldData, after) {
		t.Fatal("failure changed disk")
	}
}
func TestAuthoredImportRollbackAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "effects.json")
	store, err := LoadDeviceStore(path)
	if err != nil {
		t.Fatal(err)
	}
	imports := map[string]EffectSettings{"static": testStaticSettings(17)}
	palettes := map[string]map[string]Color{"mouse": {"0": {Red: 200}}}
	if err = store.ImportDevice("harpoon", imports, palettes, func() error { return errors.New("target failure") }); err == nil {
		t.Fatal("import failure hidden")
	}
	if _, found, _ := store.Get("harpoon", "static"); found {
		t.Fatal("partial effect publication")
	}
	if _, found, _ := store.GetAuthoredZones("harpoon", "mouse"); found {
		t.Fatal("partial palette publication")
	}
	disk, err := LoadDeviceStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, found, _ := disk.GetAuthoredZones("harpoon", "mouse"); found {
		t.Fatal("disk rollback failed")
	}
	if err = store.ImportDevice("harpoon", imports, palettes, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	imports["static"] = testStaticSettings(99)
	palettes["mouse"]["0"] = Color{Red: 99}
	if err = store.ImportDevice("harpoon", imports, palettes, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	got, _, _ := store.Get("harpoon", "static")
	colors, _, _ := store.GetAuthoredZones("harpoon", "mouse")
	if got.SingleColor.Color.Red != 17 || colors["0"].Red != 200 {
		t.Fatal("import overwrote existing customization")
	}
	for _, palette := range []map[string]Color{nil, {"": {}}, {"0": {Red: 300}}} {
		if store.SetAuthoredZones("harpoon", "mouse", palette) == nil {
			t.Fatal("invalid palette accepted")
		}
	}
	if store.SetAuthoredZones("harpoon", "static", map[string]Color{"0": {}}) == nil {
		t.Fatal("authored Static introduced")
	}
}
