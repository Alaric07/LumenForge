package openrgbimport

import (
	"LumenForge/src/config"
	"LumenForge/src/rgb"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type userProfileOverrideOutputCalls struct {
	colors           int
	frames           int
	persistentFrames int
}

func installUserProfileOverrideTestState(t *testing.T) (config.Paths, *userProfileOverrideOutputCalls) {
	t.Helper()

	root := t.TempDir()
	paths, err := config.ResolvePaths(config.PathOptions{
		Mode:             config.ServiceModeDevelopment,
		WorkingDirectory: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = config.EnsureRuntimeDirectories(paths); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(config.UsePathsForTest(paths))

	previousColor := sendLightingColor
	previousFrame := sendLightingFrame
	previousPersistentFrame := sendLightingPersistentFrame
	calls := &userProfileOverrideOutputCalls{}
	sendLightingColor = func(context.Context, uint32, int, []byte) error {
		calls.colors++
		return nil
	}
	sendLightingFrame = func(context.Context, uint32, []byte) error {
		calls.frames++
		return nil
	}
	sendLightingPersistentFrame = func(conn net.Conn, _ uint32, _ []byte) (net.Conn, error) {
		calls.persistentFrames++
		return conn, nil
	}
	t.Cleanup(func() {
		sendLightingColor = previousColor
		sendLightingFrame = previousFrame
		sendLightingPersistentFrame = previousPersistentFrame
	})

	return paths, calls
}

func newUserProfileOverrideTestDevice(t *testing.T, paths config.Paths, override *RGBOverride) *Device {
	t.Helper()

	device := newStaticOverrideTestDevice(73)
	device.Serial = "openrgb-user-profile-override-test"
	device.Product = "User Profile Override Test Controller"
	device.Config.Serial = device.Serial
	device.Config.Product = device.Product
	device.DeviceProfile.Active = true
	device.DeviceProfile.Path = filepath.Join(paths.MutableProfilesRoot, device.Serial+".json")
	device.DeviceProfile.Product = device.Product
	device.DeviceProfile.Serial = device.Serial
	device.DeviceProfile.RGBOverride = override
	if err := device.saveDeviceProfileChecked(); err != nil {
		t.Fatalf("save source profile: %v", err)
	}
	return device
}

func readUserProfileOverrideTestProfile(t *testing.T, path string) DeviceProfile {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read profile %q: %v", path, err)
	}
	var profile DeviceProfile
	if err = json.Unmarshal(data, &profile); err != nil {
		t.Fatalf("decode profile %q: %v", path, err)
	}
	return profile
}

func assertUserProfileOverrideExistingFields(t *testing.T, saved, source *DeviceProfile, expectedPath string) {
	t.Helper()

	if saved == nil {
		t.Fatal("saved user profile is missing")
	}
	if saved.Active {
		t.Fatal("new user profile unexpectedly became active")
	}
	if saved.Path != expectedPath || saved.Product != source.Product || saved.Serial != source.Serial ||
		saved.RGBCluster != source.RGBCluster {
		t.Fatalf("saved profile identity/state = %#v, source %#v, expected path %q", saved, source, expectedPath)
	}
	if saved.RGBProfile != "" || saved.BrightnessSlider != nil {
		t.Fatalf("saved legacy lighting fields = effect %q, brightness %#v", saved.RGBProfile, saved.BrightnessSlider)
	}
	if !reflect.DeepEqual(saved.ZoneColors, source.ZoneColors) {
		t.Fatalf("saved zones = %#v, source %#v", saved.ZoneColors, source.ZoneColors)
	}
}

func TestOpenRGBUserProfileOverride(t *testing.T) {
	t.Run("nil override remains nil through save and reload", func(t *testing.T) {
		paths, calls := installUserProfileOverrideTestState(t)
		device := newUserProfileOverrideTestDevice(t, paths, nil)
		source := cloneDeviceProfile(device.DeviceProfile)
		profileName := "nil-override"
		profilePath := filepath.Join(paths.MutableProfilesRoot, device.Serial+"-"+profileName+".json")

		if got := device.SaveUserProfile(profileName); got != 1 {
			t.Fatalf("SaveUserProfile() = %d, want 1", got)
		}
		if calls.colors != 0 || calls.frames != 0 || calls.persistentFrames != 0 {
			t.Fatalf("profile save emitted output: %#v", calls)
		}
		saved := device.UserProfiles[profileName]
		assertUserProfileOverrideExistingFields(t, saved, source, profilePath)
		if saved.RGBOverride != nil {
			t.Fatalf("saved nil override = %#v", saved.RGBOverride)
		}
		if persisted := readUserProfileOverrideTestProfile(t, profilePath); persisted.RGBOverride != nil {
			t.Fatalf("persisted nil override = %#v", persisted.RGBOverride)
		}

		reloaded := newStaticOverrideTestDevice(73)
		reloaded.Serial = device.Serial
		reloaded.Product = device.Product
		reloaded.loadDeviceProfiles()
		if got := reloaded.UserProfiles[profileName]; got == nil || got.RGBOverride != nil {
			t.Fatalf("reloaded nil override profile = %#v", got)
		}
	})

	t.Run("enabled override is independent and survives profile switching", func(t *testing.T) {
		paths, calls := installUserProfileOverrideTestState(t)
		override := &RGBOverride{
			Enabled:        true,
			RGBStartColor:  rgb.Color{Red: 0, Green: 0, Blue: 0, Brightness: 1, Temperature: 0, Position: 0, Hex: "#000000"},
			RGBMiddleColor: rgb.Color{Red: 10, Green: 20, Blue: 30, Brightness: 0.5, Temperature: 42, Position: 0.5, Hex: "#0a141e"},
			RGBEndColor:    rgb.Color{Red: 200, Green: 210, Blue: 220, Brightness: 0.25, Temperature: 105, Position: 1, Hex: "#c8d2dc"},
			RgbModeSpeed:   0,
		}
		device := newUserProfileOverrideTestDevice(t, paths, override)
		source := device.DeviceProfile
		sourceFields := cloneDeviceProfile(source)
		want := *source.RGBOverride
		profileName := "enabled-override"
		profilePath := filepath.Join(paths.MutableProfilesRoot, device.Serial+"-"+profileName+".json")

		if got := device.SaveUserProfile(profileName); got != 1 {
			t.Fatalf("SaveUserProfile() = %d, want 1", got)
		}
		if calls.colors != 0 || calls.frames != 0 || calls.persistentFrames != 0 {
			t.Fatalf("profile save emitted output: %#v", calls)
		}
		source = device.UserProfiles["default"]
		saved := device.UserProfiles[profileName]
		assertUserProfileOverrideExistingFields(t, saved, sourceFields, profilePath)
		if saved.RGBOverride == nil || *saved.RGBOverride != want {
			t.Fatalf("saved enabled override = %#v, want %#v", saved.RGBOverride, want)
		}
		if saved.RGBOverride == source.RGBOverride {
			t.Fatal("saved and source profiles share an RGBOverride pointer")
		}
		persisted := readUserProfileOverrideTestProfile(t, profilePath)
		if persisted.RGBOverride == nil || *persisted.RGBOverride != want {
			t.Fatalf("persisted enabled override = %#v, want %#v", persisted.RGBOverride, want)
		}

		source.RGBOverride.RGBStartColor.Red = 77
		sourceAfterMutation := *source.RGBOverride
		if *saved.RGBOverride != want {
			t.Fatalf("source mutation changed saved override to %#v", saved.RGBOverride)
		}
		if got := device.ChangeDeviceProfile(profileName); got != 1 {
			t.Fatalf("select copied profile = %d, want 1", got)
		}
		if device.DeviceProfile.RGBOverride == nil || *device.DeviceProfile.RGBOverride != want {
			t.Fatalf("selected copied override = %#v, want %#v", device.DeviceProfile.RGBOverride, want)
		}
		if got := device.ChangeDeviceProfile("default"); got != 1 {
			t.Fatalf("select default profile = %d, want 1", got)
		}
		if device.DeviceProfile.RGBOverride == nil || *device.DeviceProfile.RGBOverride != sourceAfterMutation {
			t.Fatalf("restored source override = %#v, want %#v", device.DeviceProfile.RGBOverride, sourceAfterMutation)
		}
		if got := device.ChangeDeviceProfile(profileName); got != 1 {
			t.Fatalf("reselect copied profile = %d, want 1", got)
		}
		if device.DeviceProfile.RGBOverride == nil || *device.DeviceProfile.RGBOverride != want {
			t.Fatalf("reselected copied override = %#v, want %#v", device.DeviceProfile.RGBOverride, want)
		}

		reloaded := newStaticOverrideTestDevice(73)
		reloaded.Serial = device.Serial
		reloaded.Product = device.Product
		reloaded.loadDeviceProfiles()
		if got := reloaded.UserProfiles[profileName]; got == nil || got.RGBOverride == nil || *got.RGBOverride != want {
			t.Fatalf("restart-reloaded enabled override = %#v, want %#v", got, want)
		}

		device.DeviceProfile.RGBOverride.RGBEndColor.Blue = 1
		defaultProfile := device.UserProfiles["default"]
		if defaultProfile == nil || defaultProfile.RGBOverride == nil || *defaultProfile.RGBOverride != sourceAfterMutation {
			t.Fatalf("active saved-profile mutation changed source profile to %#v", defaultProfile)
		}
	})

	t.Run("disabled override remains nonnil and preserves values", func(t *testing.T) {
		paths, calls := installUserProfileOverrideTestState(t)
		override := &RGBOverride{
			Enabled:        false,
			RGBStartColor:  rgb.Color{Red: 0, Green: 0, Blue: 0, Brightness: 1},
			RGBMiddleColor: rgb.Color{Red: 5, Green: 6, Blue: 7, Brightness: 1, Temperature: 50},
			RGBEndColor:    rgb.Color{Red: 8, Green: 9, Blue: 10, Brightness: 1, Temperature: 100},
			RgbModeSpeed:   0,
		}
		device := newUserProfileOverrideTestDevice(t, paths, override)
		device.DeviceProfile.RGBCluster = true
		if err := device.saveDeviceProfileChecked(); err != nil {
			t.Fatalf("persist source cluster state: %v", err)
		}
		source := cloneDeviceProfile(device.DeviceProfile)
		want := *source.RGBOverride
		profileName := "disabled-override"
		profilePath := filepath.Join(paths.MutableProfilesRoot, device.Serial+"-"+profileName+".json")

		if got := device.SaveUserProfile(profileName); got != 1 {
			t.Fatalf("SaveUserProfile() = %d, want 1", got)
		}
		if calls.colors != 0 || calls.frames != 0 || calls.persistentFrames != 0 {
			t.Fatalf("profile save emitted output: %#v", calls)
		}
		saved := device.UserProfiles[profileName]
		assertUserProfileOverrideExistingFields(t, saved, source, profilePath)
		if saved.RGBOverride == nil || saved.RGBOverride.Enabled || *saved.RGBOverride != want {
			t.Fatalf("saved disabled override = %#v, want %#v", saved.RGBOverride, want)
		}
		persisted := readUserProfileOverrideTestProfile(t, profilePath)
		if persisted.RGBOverride == nil || persisted.RGBOverride.Enabled || *persisted.RGBOverride != want {
			t.Fatalf("persisted disabled override = %#v, want %#v", persisted.RGBOverride, want)
		}
	})
}
