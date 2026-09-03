package dashboard

import (
	"LumenForge/src/config"
	"LumenForge/src/logger"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestThemeFallback(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create themes subdirectories
	themesDir := filepath.Join(tempDir, "static", "css", "themes")
	if err := os.MkdirAll(themesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a dummy CSS theme file to verify scanner
	dummyTheme := filepath.Join(themesDir, "tokyonight.css")
	if err := os.WriteFile(dummyTheme, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Save original working directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)

	// Change working directory to tempDir so config.Init() targets it
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	// Initialize config & logger in temp directory
	config.Init()
	logger.Init()

	// Force dashboard path to point to tempDir/dashboard.json
	location = filepath.Join(tempDir, "dashboard.json")

	// Initialize dashboard to load initial data in temp directory
	Init()

	// 1. Ensure "default" is in themes list
	if !slices.Contains(dashboard.Themes, "default") {
		t.Errorf("expected dashboard.Themes to contain 'default'")
	}

	// 2. Ensure the dummy "tokyonight" theme was also successfully scanned
	if !slices.Contains(dashboard.Themes, "tokyonight") {
		t.Errorf("expected dashboard.Themes to contain 'tokyonight'")
	}

	// 3. Set an invalid/missing theme
	dashboard.Theme = "nonexistent_theme_abc_123"
	SaveDashboardSettings(dashboard, false)

	// 4. Re-initialize, which should trigger the fallback and save
	Init()

	if dashboard.Theme != "default" {
		t.Errorf("expected fallback theme to be 'default', got '%s'", dashboard.Theme)
	}

	// 5. Verify file persistence in temp directory
	file, err := os.Open(location)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var check Dashboard
	if err := json.NewDecoder(file).Decode(&check); err != nil {
		t.Fatal(err)
	}

	if check.Theme != "default" {
		t.Errorf("expected persisted theme to be 'default', got '%s'", check.Theme)
	}
}

func TestDashboardIgnoresLegacyMembershipFields(t *testing.T) {
	var decoded Dashboard
	if err := json.Unmarshal([]byte(`{"devices":["legacy-device"],"addDeviceToDashboard":true,"dashboardLayout":[{"id":"native:device-a","column":1,"order":0}]}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.DashboardLayout, []LayoutItem{{ID: "native:device-a", Column: 1, Order: 0}}) {
		t.Fatalf("decoded layout = %#v", decoded.DashboardLayout)
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || string(encoded) == `{"devices":["legacy-device"]}` || string(encoded) == `{"addDeviceToDashboard":true}` {
		t.Fatalf("unexpected dashboard serialization: %s", encoded)
	}
	var persisted map[string]any
	if err := json.Unmarshal(encoded, &persisted); err != nil {
		t.Fatal(err)
	}
	if _, exists := persisted["devices"]; exists {
		t.Fatal("legacy devices field was serialized")
	}
	if _, exists := persisted["addDeviceToDashboard"]; exists {
		t.Fatal("legacy add-device field was serialized")
	}
}

func TestUpdateDashboardLayoutPersistsAndMigratesOpenRGBSerial(t *testing.T) {
	originalDashboard := dashboard
	originalLocation := location
	t.Cleanup(func() { dashboard, location = originalDashboard, originalLocation })
	location = filepath.Join(t.TempDir(), "dashboard.json")
	dashboard = Dashboard{}
	layout := []LayoutItem{{ID: "native:device-a", Column: 1, Order: 0}, {ID: "openrgb:device-b", Column: 1, Order: 0}}
	status, persisted := UpdateDashboardLayout(layout)
	if status != 1 || len(persisted) != 2 || persisted[1].Order != 1 {
		t.Fatalf("layout update = %#v", persisted)
	}
	MigrateDeviceSerial("device-b", "replacement")
	if got := GetDashboardLayout()[1].ID; got != "openrgb:replacement" {
		t.Fatalf("migrated layout ID = %q", got)
	}
	if status, _ := UpdateDashboardLayout([]LayoutItem{{ID: "native:device-a", Column: -1, Order: 0}}); status != 0 {
		t.Fatal("invalid coordinate was accepted")
	}
	if status, _ := UpdateDashboardLayout([]LayoutItem{{ID: "native:device-a", Column: 0, Order: -1}}); status != 0 {
		t.Fatal("negative stack order was accepted")
	}
	if status, _ := UpdateDashboardLayout([]LayoutItem{{ID: "native:device-a", Column: 0, Order: 0}, {ID: "native:device-a", Column: 1, Order: 0}}); status != 0 {
		t.Fatal("duplicate layout ID was accepted")
	}
	var legacy LayoutItem
	if err := json.Unmarshal([]byte(`{"id":"native:legacy","x":2,"y":3,"w":4,"h":5}`), &legacy); err != nil || legacy.Column != 2 || legacy.Order != 3 {
		t.Fatalf("legacy layout was not mapped: %#v, %v", legacy, err)
	}
}
