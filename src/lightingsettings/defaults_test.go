package lightingsettings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"LumenForge/src/rgb"
)

func shippedDefaultsPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "database", "rgb.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("shipped defaults fixture: %v", err)
	}
	return path
}

func loadTestDefaults(t *testing.T) *DefaultRepository {
	t.Helper()
	repository, err := LoadDefaultRepository(shippedDefaultsPath(t))
	if err != nil {
		t.Fatalf("LoadDefaultRepository() error = %v", err)
	}
	return repository
}

func TestDefaultRepositoryLoadsKnownCompleteShippedDefinitions(t *testing.T) {
	repository := loadTestDefaults(t)

	gradient, err := repository.Get("gradient")
	if err != nil {
		t.Fatal(err)
	}
	if gradient.Gradient == nil || gradient.Speed == nil || len(gradient.Gradient.Stops) != 4 || *gradient.Speed != 10 {
		t.Fatalf("Gradient default = %#v", gradient)
	}
	cpu, err := repository.Get("cpu-temperature")
	if err != nil {
		t.Fatal(err)
	}
	if cpu.Temperature == nil || cpu.Temperature.Low.Celsius != 20 || cpu.Temperature.Middle.Celsius != 50 || cpu.Temperature.High.Celsius != 95 {
		t.Fatalf("CPU temperature default = %#v", cpu)
	}
	off, err := repository.Get("off")
	if err != nil || off.EffectID != "off" || off.Speed != nil || off.SingleColor != nil {
		t.Fatalf("Off default = %#v, %v", off, err)
	}
}

func TestDefaultRepositoryReturnsDeepDefensiveCopies(t *testing.T) {
	repository := loadTestDefaults(t)
	first, err := repository.Get("gradient")
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.Get("gradient")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated reads differ:\nfirst=%#v\nsecond=%#v", first, second)
	}
	*first.Speed = 1
	first.EffectID = "changed"
	first.Gradient.Stops[0].Position = 0.75
	first.Gradient.Stops[0].Color.Red = 7
	first.Gradient.Stops = first.Gradient.Stops[:1]

	later, err := repository.Get("gradient")
	if err != nil {
		t.Fatal(err)
	}
	if later.EffectID != "gradient" || *later.Speed != 10 || len(later.Gradient.Stops) != 4 || later.Gradient.Stops[0].Position != 0 || later.Gradient.Stops[0].Color.Red != 255 {
		t.Fatalf("mutating returned Gradient changed repository: %#v", later)
	}

	temperature, err := repository.Get("gpu-temperature")
	if err != nil {
		t.Fatal(err)
	}
	temperature.Temperature.Middle.Color.Green = 0
	temperature.Temperature.Middle.Celsius = 1
	laterTemperature, err := repository.Get("gpu-temperature")
	if err != nil {
		t.Fatal(err)
	}
	if laterTemperature.Temperature.Middle.Color.Green != 255 || laterTemperature.Temperature.Middle.Celsius != 50 {
		t.Fatalf("mutating returned temperature changed repository: %#v", laterTemperature)
	}
}

func TestDefaultRepositoryDistinguishesUnknownMissingAndMalformedData(t *testing.T) {
	repository := loadTestDefaults(t)
	if _, err := repository.Get("not-an-effect"); !errors.Is(err, ErrUnknownEffect) {
		t.Fatalf("unknown effect error = %v", err)
	}
	repository.settings = map[string]EffectSettings{}
	if _, err := repository.Get("static"); !errors.Is(err, ErrDefaultUnavailable) {
		t.Fatalf("unavailable default error = %v", err)
	}

	missingPath := filepath.Join(t.TempDir(), "missing.json")
	if _, err := LoadDefaultRepository(missingPath); !errors.Is(err, ErrDefaultUnavailable) {
		t.Fatalf("missing file error = %v", err)
	}
	malformedPath := filepath.Join(t.TempDir(), "rgb.json")
	if err := os.WriteFile(malformedPath, []byte(`{"profiles":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDefaultRepository(malformedPath); err == nil || errors.Is(err, ErrDefaultUnavailable) {
		t.Fatalf("malformed file error = %v", err)
	}
	incompletePath := filepath.Join(t.TempDir(), "rgb.json")
	if err := os.WriteFile(incompletePath, []byte(`{"profiles":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDefaultRepository(incompletePath); !errors.Is(err, ErrDefaultUnavailable) {
		t.Fatalf("missing definition error = %v", err)
	}
}

func TestReadingDefaultsCreatesNoMutableState(t *testing.T) {
	directory := t.TempDir()
	data, err := os.ReadFile(shippedDefaultsPath(t))
	if err != nil {
		t.Fatal(err)
	}
	defaultsPath := filepath.Join(directory, "rgb.json")
	if err = os.WriteFile(defaultsPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	repository, err := LoadDefaultRepository(defaultsPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.Get("static"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "rgb.json" {
		t.Fatalf("default reads created mutable state: %v", entries)
	}
}

func TestShippedConversionRejectsMissingRequiredData(t *testing.T) {
	staticDescriptor, ok := rgb.SoftwareEffectDescriptorByID("static")
	if !ok {
		t.Fatal("Static descriptor is missing")
	}
	if _, err := settingsFromShippedProfile(staticDescriptor, shippedProfile{}); err == nil {
		t.Fatal("missing shipped Static color was accepted")
	}
	rainbowDescriptor, ok := rgb.SoftwareEffectDescriptorByID("rainbow")
	if !ok {
		t.Fatal("Rainbow descriptor is missing")
	}
	if _, err := settingsFromShippedProfile(rainbowDescriptor, shippedProfile{}); err == nil {
		t.Fatal("missing shipped Rainbow Speed was accepted")
	}
}

func TestShippedGradientConversionPreservesRendererPositionOrdering(t *testing.T) {
	descriptor, ok := rgb.SoftwareEffectDescriptorByID("gradient")
	if !ok {
		t.Fatal("Gradient descriptor is missing")
	}
	settings, err := settingsFromShippedProfile(descriptor, shippedProfile{
		Speed: testSpeed(5),
		Gradients: map[int]rgb.Color{
			0: {Red: 1, Position: 0.8, Brightness: 1},
			1: {Red: 2, Position: 0.2, Brightness: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if settings.Gradient.Stops[0].Position != 0.2 || settings.Gradient.Stops[0].Color.Red != 2 ||
		settings.Gradient.Stops[1].Position != 0.8 || settings.Gradient.Stops[1].Color.Red != 1 {
		t.Fatalf("shipped Gradient order = %#v", settings.Gradient.Stops)
	}
}

func TestPersistedSettingsDistinguishCompleteBlackFromMissingNestedData(t *testing.T) {
	complete := testStaticSettings(0)
	data, err := json.Marshal(complete)
	if err != nil {
		t.Fatal(err)
	}
	var decoded EffectSettings
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("complete black settings failed to decode: %v", err)
	}
	if err = Validate(decoded); err != nil {
		t.Fatalf("complete black settings failed validation: %v", err)
	}

	for _, malformed := range []string{
		`{"schemaVersion":1,"effectId":"static","singleColor":{}}`,
		`{"schemaVersion":1,"effectId":"static","singleColor":{"color":{"red":0,"green":0}}}`,
		`{"schemaVersion":1,"effectId":"static","singleColor":{"color":{"red":0,"green":0,"blue":0,"extra":1}}}`,
	} {
		if err = json.Unmarshal([]byte(malformed), &decoded); err == nil {
			t.Fatalf("incomplete nested settings decoded successfully: %s", malformed)
		}
	}
}
