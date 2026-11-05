package lightingsettings

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGradientAuthoredKeysValidationCopyAndPersistence(t *testing.T) {
	defaults, err := LoadDefaultRepository("../../database/rgb.json")
	if err != nil {
		t.Fatal(err)
	}
	value, err := defaults.Get("gradient")
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]int, len(value.Gradient.Stops))
	for i := range keys {
		keys[i] = len(keys) - i - 1
	}
	value.Gradient.Keys = keys
	if err = Validate(value); err != nil {
		t.Fatal(err)
	}
	cloned := value.Clone()
	cloned.Gradient.Keys[0] = 999
	if value.Gradient.Keys[0] == 999 {
		t.Fatal("key slice aliases")
	}
	path := filepath.Join(t.TempDir(), "effects.json")
	store, err := LoadDeviceStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Set("ultra", "gradient", value); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadDeviceStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got, found, err := reloaded.Get("ultra", "gradient")
	if err != nil || !found || !reflect.DeepEqual(got, value) {
		t.Fatal(got, found, err)
	}
	for _, invalid := range [][]int{{0}, {0, 0}, {-1, 1}} {
		bad := value.Clone()
		bad.Gradient.Keys = invalid
		if Validate(bad) == nil {
			t.Fatal("invalid keys accepted", invalid)
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded EffectSettings
	if err = json.Unmarshal(raw, &decoded); err != nil || !reflect.DeepEqual(decoded, value) {
		t.Fatal("keys JSON roundtrip", err)
	}
	value.Gradient.Keys = nil
	raw, err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &decoded); err != nil || decoded.Gradient.Keys != nil {
		t.Fatal("keyless records changed", err)
	}
}
