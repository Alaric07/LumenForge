package cluster

import (
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/rgb"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestClusterStaticBrightnessAppliedOnce(t *testing.T) {
	tests := []struct {
		name       string
		brightness uint8
		want       []byte
	}{
		{name: "black", brightness: 0, want: []byte{0, 0, 0, 0, 0, 0}},
		{name: "intermediate", brightness: 50, want: []byte{100, 50, 25, 100, 50, 25}},
		{name: "maximum", brightness: 100, want: []byte{200, 100, 50, 200, 100, 50}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			brightness := test.brightness
			device := &Device{
				DeviceProfile: &DeviceProfile{
					RGBProfile:       "static",
					BrightnessSlider: &brightness,
				},
				Rgb: &rgb.RGB{Profiles: map[string]rgb.Profile{
					"static": {
						Speed:      1,
						Brightness: 1,
						StartColor: rgb.Color{Red: 200, Green: 100, Blue: 50, Brightness: 1},
						EndColor:   rgb.Color{Red: 200, Green: 100, Blue: 50, Brightness: 1},
					},
				}},
			}
			startTime := time.Unix(0, 0)

			got := device.generateRgbEffect(2, &startTime, "static", rgb.Exit())
			if !bytes.Equal(got, test.want) {
				t.Fatalf("static cluster frame at %d%% = %v, want %v", test.brightness, got, test.want)
			}
		})
	}
}

func TestClusterDistributeColorsCopiesOrderedMemberSegments(t *testing.T) {
	aggregate := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	original := append([]byte(nil), aggregate...)
	var first, second []byte
	device := &Device{Controllers: []*common.ClusterController{
		{
			Serial:      "first",
			LedChannels: 1,
			WriteColorEx: func(data []byte, _ int) {
				first = append([]byte(nil), data...)
				data[0] = 255
			},
		},
		{
			Serial:      "second",
			LedChannels: 2,
			WriteColorEx: func(data []byte, _ int) {
				second = append([]byte(nil), data...)
				data[0] = 254
			},
		},
	}}

	device.distributeColors(aggregate)

	if !bytes.Equal(first, []byte{1, 2, 3}) {
		t.Fatalf("first member segment = %v, want [1 2 3]", first)
	}
	if !bytes.Equal(second, []byte{4, 5, 6, 7, 8, 9}) {
		t.Fatalf("second member segment = %v, want [4 5 6 7 8 9]", second)
	}
	if !bytes.Equal(aggregate, original) {
		t.Fatalf("aggregate frame mutated during dispatch: got %v, want %v", aggregate, original)
	}
}

func TestFreshClusterRgbUsesCanonicalDefaults(t *testing.T) {
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	repositoryRoot := filepath.Clean(filepath.Join(originalWorkingDirectory, "..", ".."))
	temporaryConfig := t.TempDir()
	paths, err := config.ResolvePaths(config.PathOptions{
		Mode:             config.ServiceModeDevelopment,
		ApplicationRoot:  repositoryRoot,
		ConfigRoot:       temporaryConfig,
		DataRoot:         temporaryConfig,
		WorkingDirectory: repositoryRoot,
	})
	if err != nil {
		t.Fatalf("resolve temporary paths: %v", err)
	}
	t.Cleanup(config.UsePathsForTest(paths))

	rgb.Init()
	canonical := rgb.GetRGB()
	canonical.Device = "Cluster"

	for _, directory := range []string{"rgb", "profiles"} {
		if err = os.MkdirAll(filepath.Join(temporaryConfig, "database", directory), 0o755); err != nil {
			t.Fatalf("create isolated %s directory: %v", directory, err)
		}
	}

	originalConfigPath := pwd
	pwd = temporaryConfig
	t.Cleanup(func() {
		pwd = originalConfigPath
	})

	rgbModes := make([]string, 0, len(canonical.Profiles))
	for profile := range canonical.Profiles {
		rgbModes = append(rgbModes, profile)
	}

	device := &Device{
		Product:  "Cluster",
		Serial:   "cluster",
		RGBModes: rgbModes,
	}
	device.saveDeviceProfile()
	if _, err = os.Stat(filepath.Join(temporaryConfig, "database", "profiles", "cluster.json")); err != nil {
		t.Fatalf("cluster profile was not written beneath mutable database root: %v", err)
	}
	device.loadRgb()

	generatedPath := filepath.Join(temporaryConfig, "database", "rgb", "cluster.json")
	generatedData, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("read generated cluster RGB defaults: %v", err)
	}

	var generated rgb.RGB
	if err = json.Unmarshal(generatedData, &generated); err != nil {
		t.Fatalf("decode generated cluster RGB defaults: %v", err)
	}
	if !reflect.DeepEqual(generated, canonical) {
		t.Fatal("fresh cluster RGB defaults drifted from database/rgb.json")
	}

	var topLevel map[string]json.RawMessage
	if err = json.Unmarshal(generatedData, &topLevel); err != nil {
		t.Fatalf("decode generated top-level fields: %v", err)
	}
	for _, key := range []string{
		"DeviceOrder",
		"RGBProfile",
		"LastNonOffProfile",
		"RgbOff",
		"deviceOrder",
		"rgbProfile",
		"lastNonOffProfile",
		"rgbOff",
	} {
		if _, exists := topLevel[key]; exists {
			t.Errorf("generated RGB defaults contain machine-specific field %q", key)
		}
	}
	for key := range topLevel {
		if key != "device" && key != "defaultColor" && key != "profiles" {
			t.Errorf("generated RGB defaults contain unexpected top-level field %q", key)
		}
	}

	expectedProfiles := map[string]struct {
		speed      float64
		smoothness int
	}{
		"comet":           {speed: 5, smoothness: 20},
		"datastream":      {speed: 9.8, smoothness: 20},
		"flame":           {speed: 0.8, smoothness: 20},
		"cyberpunkglitch": {speed: 0.55, smoothness: 20},
		"nebula":          {speed: 4, smoothness: 0},
		"tokyonight":      {speed: 2.8, smoothness: 20},
	}
	for name, expected := range expectedProfiles {
		profile, exists := generated.Profiles[name]
		if !exists {
			t.Errorf("generated RGB defaults are missing %q", name)
			continue
		}
		if profile.Speed != expected.speed {
			t.Errorf("%s speed = %v, want %v", name, profile.Speed, expected.speed)
		}
		if profile.Smoothness != expected.smoothness {
			t.Errorf("%s smoothness = %d, want %d", name, profile.Smoothness, expected.smoothness)
		}
	}
}
