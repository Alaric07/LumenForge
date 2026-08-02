package openrgbimport

import (
	"LumenForge/src/rgb"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"
	"time"
)

var expectedImporterSoftwareEffectIDs = []string{
	"arc",
	"aurora",
	"circle",
	"circleshift",
	"colorpulse",
	"colorshift",
	"colorwarp",
	"comet",
	"cpu-temperature",
	"cyberpunkglitch",
	"datastream",
	"flame",
	"flickering",
	"gpu-temperature",
	"gradient",
	"marquee",
	"nebula",
	"off",
	"pastelrainbow",
	"pastelspiralrainbow",
	"plasmacore",
	"rain",
	"rainbow",
	"rotarystack",
	"rotator",
	"sequential",
	"spinner",
	"spiralrainbow",
	"stardust",
	"static",
	"storm",
	"tokyonight",
	"visor",
	"watercolor",
	"wave",
}

func shippedSoftwareEffectProfiles(t *testing.T) map[string]rgb.Profile {
	t.Helper()

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve catalogue test source path")
	}
	path := filepath.Join(filepath.Dir(sourceFile), "..", "..", "..", "database", "rgb.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shipped RGB profiles: %v", err)
	}
	var state rgb.RGB
	if err = json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode shipped RGB profiles: %v", err)
	}
	return state.Profiles
}

func TestOpenRGBSoftwareEffectCatalogueInventoryAndCopies(t *testing.T) {
	first := importerSoftwareEffectCatalogue()
	second := importerSoftwareEffectCatalogue()
	if !slices.Equal(first, expectedImporterSoftwareEffectIDs) {
		t.Fatalf("importer catalogue = %v, want %v", first, expectedImporterSoftwareEffectIDs)
	}
	if !slices.Equal(first, second) {
		t.Fatalf("successive importer catalogues differ: %v and %v", first, second)
	}

	seen := make(map[string]struct{}, len(first))
	for _, id := range first {
		if id == "" {
			t.Fatal("importer catalogue contains an empty ID")
		}
		if _, duplicate := seen[id]; duplicate {
			t.Fatalf("importer catalogue contains duplicate ID %q", id)
		}
		seen[id] = struct{}{}
	}
	for _, id := range []string{
		"arc", "comet", "datastream", "marquee", "nebula", "pastelspiralrainbow", "plasmacore",
		"rain", "rotarystack", "sequential", "spiralrainbow", "stardust", "tokyonight", "visor",
	} {
		if !slices.Contains(first, id) {
			t.Errorf("newly supported effect %q is absent", id)
		}
	}
	for _, id := range []string{"colorwave", "led", "liquid-temperature", "probe-temperature", "rainbowwave", "tlk", "tlr"} {
		if slices.Contains(first, id) {
			t.Errorf("device-specific effect %q is present", id)
		}
	}

	descriptorsBefore := rgb.SoftwareEffectDescriptors()
	firstDevice := &Device{RGBModes: first}
	secondDevice := &Device{RGBModes: second}
	firstDevice.RGBModes[0] = "caller-mutation"
	if secondDevice.RGBModes[0] == "caller-mutation" {
		t.Fatal("device catalogues share mutable backing state")
	}
	if future := importerSoftwareEffectCatalogue(); !slices.Equal(future, expectedImporterSoftwareEffectIDs) {
		t.Fatalf("device mutation changed future catalogue: %v", future)
	}
	if descriptorsAfter := rgb.SoftwareEffectDescriptors(); !reflect.DeepEqual(descriptorsAfter, descriptorsBefore) {
		t.Fatal("device catalogue mutation changed canonical descriptors")
	}
}

func TestOpenRGBSoftwareEffectCatalogueContractsAndFrames(t *testing.T) {
	installLightingTemperatureTestSeams(t, 45, 60, 65)
	profiles := shippedSoftwareEffectProfiles(t)
	catalogue := importerSoftwareEffectCatalogue()

	for index, id := range catalogue {
		t.Run(id, func(t *testing.T) {
			descriptor, ok := rgb.SoftwareEffectDescriptorByID(id)
			if !ok {
				t.Fatalf("catalogue ID %q has no descriptor", id)
			}
			if !descriptor.Scope.Includes(rgb.EffectScopeDevice) {
				t.Fatalf("catalogue ID %q scope %d excludes Device", id, descriptor.Scope)
			}
			if descriptor.Label == "" || descriptor.Icon == "" {
				t.Fatalf("catalogue ID %q has incomplete presentation metadata: %+v", id, descriptor)
			}
			capability, known := rgb.LightingEffectCapabilities(id)
			wantCapability := rgb.LightingEffectCapability{
				Palette:         descriptor.PaletteKind,
				UsesStartColor:  descriptor.UsesStart,
				UsesMiddleColor: descriptor.UsesMiddle,
				UsesEndColor:    descriptor.UsesEnd,
				SupportsSpeed:   descriptor.SupportsSpeed,
			}
			if !known || capability != wantCapability {
				t.Fatalf("catalogue ID %q capability = %+v, %t; want %+v, true", id, capability, known, wantCapability)
			}
			if rgb.HasSpeedControl(id) != descriptor.SupportsSpeed {
				t.Fatalf("catalogue ID %q speed metadata disagrees with descriptor", id)
			}

			profile := rgb.Profile{}
			if id != "off" {
				var found bool
				profile, found = profiles[id]
				if !found || profile.ProfileName == "" {
					t.Fatalf("catalogue ID %q has no usable shipped profile", id)
				}
			}

			ledCount := 4
			switch id {
			case "visor":
				ledCount = 1
			case "arc":
				ledCount = 2
			case "comet":
				ledCount = 8
			}
			runner := newSoftwareEffectDispatchRunner(ledCount)
			startTime := time.Now().Add(-time.Second)
			if !dispatchSoftwareEffect(id, runner, &startTime, &profile) {
				t.Fatalf("catalogue ID %q has no explicit raw importer dispatch", id)
			}
			if got, want := len(runner.Output), ledCount*3; got != want {
				t.Fatalf("catalogue ID %q frame length = %d, want %d", id, got, want)
			}
			if id == "off" && !slices.Equal(runner.Output, make([]byte, ledCount*3)) {
				t.Fatalf("catalogue ID %q frame = %v, want black output", id, runner.Output)
			}
		})

		descriptor := rgb.SoftwareEffectDescriptors()[index]
		if descriptor.ID != id {
			t.Fatalf("catalogue order diverges from descriptor order at %d: %q and %q", index, id, descriptor.ID)
		}
	}
}

func TestOpenRGBSoftwareEffectCatalogueMutationValidation(t *testing.T) {
	profileDir, _ := installLightingDeviceTestSeams(t)
	installLightingTemperatureTestSeams(t, 45, 60, 65)
	profiles := shippedSoftwareEffectProfiles(t)
	profiles["off"] = rgb.Profile{}
	catalogue := importerSoftwareEffectCatalogue()

	for _, effect := range catalogue {
		t.Run("accept/"+effect, func(t *testing.T) {
			device := newLightingMutationDevice()
			device.RGBModes = importerSoftwareEffectCatalogue()
			device.colorCount = 4
			device.Rgb = &rgb.RGB{Profiles: profiles}
			if err := device.SetEffect(effect); err != nil {
				t.Fatalf("SetEffect(%q): %v", effect, err)
			}
			t.Cleanup(device.Stop)
			if device.effect != effect || device.DeviceProfile.RGBProfile != effect {
				t.Fatalf("selected effect = %q, profile = %q, want %q", device.effect, device.DeviceProfile.RGBProfile, effect)
			}
			if persisted := readLightingDeviceProfile(t, profileDir); persisted.RGBProfile != effect {
				t.Fatalf("persisted effect = %q, want %q", persisted.RGBProfile, effect)
			}
		})
	}

	for _, effect := range []string{
		"", "unknown", "ARC", " arc", "arc ",
		"colorwave", "led", "liquid-temperature", "probe-temperature", "rainbowwave", "tlk", "tlr",
	} {
		t.Run("reject/"+effect, func(t *testing.T) {
			device := newLightingMutationDevice()
			device.RGBModes = importerSoftwareEffectCatalogue()
			if err := device.SetEffect(effect); err == nil {
				t.Fatalf("SetEffect(%q) unexpectedly succeeded", effect)
			}
			if device.effect != "off" || device.DeviceProfile.RGBProfile != "off" {
				t.Fatalf("rejected effect changed state: effect = %q, profile = %q", device.effect, device.DeviceProfile.RGBProfile)
			}
		})
	}

	device := newLightingMutationDevice()
	device.RGBModes = importerSoftwareEffectCatalogue()
	device.DeviceProfile.RGBCluster = true
	if err := device.SetEffect("arc"); err == nil {
		t.Fatal("cluster-owned SetEffect(arc) unexpectedly succeeded")
	}
	if device.effect != "off" || device.DeviceProfile.RGBProfile != "off" {
		t.Fatalf("cluster rejection changed state: effect = %q, profile = %q", device.effect, device.DeviceProfile.RGBProfile)
	}
}
