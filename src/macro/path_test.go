package macro

import (
	"LumenForge/src/config"
	"LumenForge/src/logger"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func useMacroTestPaths(t *testing.T) config.Paths {
	t.Helper()

	root := t.TempDir()
	paths, err := config.ResolvePaths(config.PathOptions{
		Mode:            config.ServiceModeUser,
		ApplicationRoot: filepath.Join(root, "app"),
		ConfigRoot:      filepath.Join(root, "config"),
		DataRoot:        filepath.Join(root, "data"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = config.EnsureRuntimeDirectories(paths); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(config.UsePathsForTest(paths))
	logger.Init()

	originalPWD := pwd
	originalLocation := location
	originalMacros := macros
	t.Cleanup(func() {
		pwd = originalPWD
		location = originalLocation
		macros = originalMacros
	})
	macros = map[int]Macro{}

	return paths
}

func TestNewMacroProfileWritesToMutableDatabaseRoot(t *testing.T) {
	paths := useMacroTestPaths(t)

	Init()
	if status := NewMacroProfile("TestProfile"); status != 1 {
		t.Fatalf("NewMacroProfile() status = %d, want 1", status)
	}
	expected := filepath.Join(paths.MutableMacrosRoot, "testprofile.json")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("macro profile was not written at %q: %v", expected, err)
	}
}

func TestMacroProfilePathRejectsTraversalAtWriteAndDeleteSinks(t *testing.T) {
	tests := []struct {
		name string
		run  func(int) uint8
	}{
		{
			name: "update",
			run: func(id int) uint8 {
				return UpdateMacroSettings(id, 9, 10)
			},
		},
		{
			name: "delete",
			run:  DeleteMacroProfile,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths := useMacroTestPaths(t)
			const id = 7
			const maliciousName = "../../config"
			macros[id] = Macro{
				Id:          id,
				Name:        maliciousName,
				Repeat:      2,
				RepeatDelay: 3,
				Actions:     map[int]Actions{},
			}

			escapedPath := filepath.Clean(filepath.Join(paths.MutableMacrosRoot, maliciousName+".json"))
			const sentinel = "outside macro root"
			if err := os.WriteFile(escapedPath, []byte(sentinel), 0o600); err != nil {
				t.Fatal(err)
			}

			if status := test.run(id); status != 0 {
				t.Fatalf("%s malicious macro status = %d, want 0", test.name, status)
			}
			content, err := os.ReadFile(escapedPath)
			if err != nil {
				t.Fatalf("%s removed outside sentinel: %v", test.name, err)
			}
			if string(content) != sentinel {
				t.Fatalf("%s changed outside sentinel to %q", test.name, content)
			}
			profile, exists := macros[id]
			if !exists {
				t.Fatalf("%s removed malicious macro from memory", test.name)
			}
			if profile.Repeat != 2 || profile.RepeatDelay != 3 {
				t.Fatalf("%s changed malicious macro settings to repeat=%d delay=%d", test.name, profile.Repeat, profile.RepeatDelay)
			}
		})
	}
}

func TestInitSkipsMacroProfileWithInvalidEmbeddedName(t *testing.T) {
	paths := useMacroTestPaths(t)
	profile := Macro{Id: 8, Name: "../../config", Actions: map[int]Actions{}}
	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(paths.MutableMacrosRoot, "placed.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	Init()
	if loaded := GetProfile(profile.Id); loaded != nil {
		t.Fatalf("Init() loaded macro with invalid embedded name: %#v", loaded)
	}
}

func TestValidMacroProfileLoadsUpdatesAndDeletes(t *testing.T) {
	paths := useMacroTestPaths(t)
	profile := Macro{
		Id:          9,
		Name:        "ValidMacro",
		Repeat:      1,
		RepeatDelay: 2,
		Actions:     map[int]Actions{},
	}
	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	profileFile := filepath.Join(paths.MutableMacrosRoot, "validmacro.json")
	if err = os.WriteFile(profileFile, data, 0o600); err != nil {
		t.Fatal(err)
	}

	Init()
	if loaded := GetProfile(profile.Id); loaded == nil || loaded.Name != profile.Name {
		t.Fatalf("Init() valid macro = %#v", loaded)
	}
	if status := UpdateMacroSettings(profile.Id, 4, 5); status != 1 {
		t.Fatalf("UpdateMacroSettings() status = %d, want 1", status)
	}
	updatedData, err := os.ReadFile(profileFile)
	if err != nil {
		t.Fatal(err)
	}
	var updated Macro
	if err = json.Unmarshal(updatedData, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Name != profile.Name || updated.Repeat != 4 || updated.RepeatDelay != 5 {
		t.Fatalf("saved valid macro = %#v", updated)
	}
	if status := DeleteMacroProfile(profile.Id); status != 1 {
		t.Fatalf("DeleteMacroProfile() status = %d, want 1", status)
	}
	if _, err = os.Stat(profileFile); !os.IsNotExist(err) {
		t.Fatalf("deleted valid macro file still exists: %v", err)
	}
	if loaded := GetProfile(profile.Id); loaded != nil {
		t.Fatalf("deleted valid macro remains loaded: %#v", loaded)
	}
}

func TestMacroNameValidationMatchesCreationContract(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "Ab3", want: true},
		{name: "ValidMacro123", want: true},
		{name: "ab", want: false},
		{name: "../../config", want: false},
		{name: `/absolute`, want: false},
		{name: `back\\slash`, want: false},
		{name: "with space", want: false},
	}
	for _, test := range tests {
		if got := IsValidName(test.name); got != test.want {
			t.Errorf("IsValidName(%q) = %t, want %t", test.name, got, test.want)
		}
	}
}
