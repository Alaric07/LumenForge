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

func TestUpdateDeviceOrder(t *testing.T) {
	originalDashboard := dashboard
	originalLocation := location
	t.Cleanup(func() {
		dashboard = originalDashboard
		location = originalLocation
	})

	tests := []struct {
		name      string
		current   []string
		requested []string
		expected  []string
	}{
		{
			name:      "normal reorder",
			current:   []string{"a", "b", "c"},
			requested: []string{"c", "a", "b"},
			expected:  []string{"c", "a", "b"},
		},
		{
			name:      "duplicate submitted serials",
			current:   []string{"a", "b", "c"},
			requested: []string{"b", "b", "a"},
			expected:  []string{"b", "a", "c"},
		},
		{
			name:      "unknown submitted serials ignored",
			current:   []string{"a", "b"},
			requested: []string{"unknown", "b"},
			expected:  []string{"b", "a"},
		},
		{
			name:      "missing selected serials appended in previous order",
			current:   []string{"a", "b", "c", "d"},
			requested: []string{"c", "a"},
			expected:  []string{"c", "a", "b", "d"},
		},
		{
			name:      "reorder does not add new devices",
			current:   []string{"a", "b"},
			requested: []string{"new-device", "b", "a"},
			expected:  []string{"b", "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			location = filepath.Join(t.TempDir(), "dashboard.json")
			dashboard = Dashboard{Devices: append([]string(nil), tt.current...)}

			status, order := UpdateDeviceOrder(tt.requested)
			if status != 1 {
				t.Fatalf("expected successful update, got status %d", status)
			}
			if !reflect.DeepEqual(order, tt.expected) {
				t.Fatalf("expected order %v, got %v", tt.expected, order)
			}
			if !reflect.DeepEqual(dashboard.Devices, tt.expected) {
				t.Fatalf("expected dashboard devices %v, got %v", tt.expected, dashboard.Devices)
			}

			var persisted Dashboard
			file, err := os.Open(location)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.NewDecoder(file).Decode(&persisted); err != nil {
				_ = file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(persisted.Devices, tt.expected) {
				t.Fatalf("expected persisted order %v, got %v", tt.expected, persisted.Devices)
			}
		})
	}
}
