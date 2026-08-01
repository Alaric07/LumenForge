package server

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

var devicesThemeContract = []string{
	"--lf-page-background",
	"--lf-navigation-surface",
	"--lf-device-list-surface",
	"--lf-workspace-surface",
	"--lf-panel-elevated",
	"--lf-panel-secondary",
	"--lf-surface-selected",
	"--lf-surface-subdued",
	"--lf-text-primary",
	"--lf-text-secondary",
	"--lf-text-muted",
	"--lf-text-technical",
	"--lf-text-on-accent",
	"--lf-accent-primary",
	"--lf-accent-secondary",
	"--lf-accent-highlight",
	"--lf-glow-ambient",
	"--lf-glow-selected",
	"--lf-glow-emphasized",
	"--lf-border-subtle",
	"--lf-border-panel",
	"--lf-border-selected",
	"--lf-border-emphasized",
	"--lf-shadow-soft",
	"--lf-shadow-elevated",
	"--lf-surface-hover",
	"--lf-surface-active",
	"--lf-focus-ring",
	"--lf-disabled-border",
	"--lf-control-track",
	"--lf-control-fill-start",
	"--lf-control-fill-end",
	"--lf-control-thumb",
	"--lf-control-thumb-border",
	"--lf-control-thumb-glow",
	"--lf-control-disabled-track",
	"--lf-control-popup",
	"--lf-control-popup-border",
	"--lf-control-option-selected",
	"--lf-control-option-hover",
}

var cssCustomPropertyPattern = regexp.MustCompile(`(?m)^\s*(--[a-z0-9-]+)\s*:\s*([^;]+);`)

func TestDevicesThemeContract(t *testing.T) {
	root := devicesThemeRepositoryRoot(t)
	themesDirectory := filepath.Join(root, "static", "css", "themes")
	wantThemes := []string{
		"catppuccin-macchiato",
		"cyberpunk",
		"default",
		"dracula",
		"tokyonight",
	}
	wantAccents := map[string][3]string{
		"catppuccin-macchiato": {"#8aadf4", "#c6a0f6", "#91d7e3"},
		"cyberpunk":            {"#00e5ff", "#ff007c", "#fcee09"},
		"default":              {"#1e90ff", "#2ecc71", "#79c7ff"},
		"dracula":              {"#bd93f9", "#8be9fd", "#ff79c6"},
		"tokyonight":           {"#7aa2f7", "#bb9af7", "#7dcfff"},
	}
	wantRootScopes := map[string]int{
		"catppuccin-macchiato": 1,
		"cyberpunk":            1,
		"default":              2,
		"dracula":              1,
		"tokyonight":           1,
	}

	entries, err := os.ReadDir(themesDirectory)
	if err != nil {
		t.Fatalf("read built-in theme directory: %v", err)
	}
	var gotThemes []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".css" {
			gotThemes = append(gotThemes, strings.TrimSuffix(entry.Name(), ".css"))
		}
	}
	sort.Strings(gotThemes)
	if !reflect.DeepEqual(gotThemes, wantThemes) {
		t.Fatalf("built-in themes changed without updating the contract test: got %v, want %v", gotThemes, wantThemes)
	}

	for _, theme := range wantThemes {
		t.Run(theme, func(t *testing.T) {
			css := devicesThemeReadFile(t, filepath.Join(themesDirectory, theme+".css"))
			rootBodies := devicesThemeRuleBodies(t, css, ":root")
			if count := len(rootBodies); count != wantRootScopes[theme] {
				t.Fatalf("theme :root scopes changed: got %d, want %d", count, wantRootScopes[theme])
			}

			properties := devicesThemeMergedProperties(t, rootBodies)
			for _, token := range devicesThemeContract {
				if _, ok := properties[token]; !ok {
					t.Errorf("theme does not explicitly define %s", token)
				}
			}

			accents := wantAccents[theme]
			for index, token := range []string{"--lf-accent-primary", "--lf-accent-secondary", "--lf-accent-highlight"} {
				if got := properties[token]; got != accents[index] {
					t.Errorf("%s = %q, want %q", token, got, accents[index])
				}
			}
		})
	}
}

func TestDevicesThemeRuleSelectionContract(t *testing.T) {
	t.Run("exact selector in list", func(t *testing.T) {
		css := `.lf-app-shell-header {
    --wrong-rule: prefix;
}

.other-selector,
.lf-app-shell
{
    --right-rule: exact;
}`

		bodies := devicesThemeRuleBodies(t, css, ".lf-app-shell")
		if len(bodies) != 1 {
			t.Fatalf("exact selector matched %d rules, want 1", len(bodies))
		}
		properties := devicesThemeProperties(t, bodies[0])
		if got := properties["--right-rule"]; got != "exact" {
			t.Errorf("exact selector property = %q, want %q", got, "exact")
		}
		if _, exists := properties["--wrong-rule"]; exists {
			t.Error("selector-prefix rule redirected exact selector lookup")
		}
	})

	t.Run("all exact root rules merge in source order", func(t *testing.T) {
		css := `:root::before {
    --wrong-rule: pseudo-element;
}

:root .child {
    --wrong-descendant: descendant;
}

:root {
    --first-only: first;
    --overridden: earlier;
}

@media (max-width: 1000px) {
    :root {
        --second-only: second;
        --overridden: later;
    }
}`

		bodies := devicesThemeRuleBodies(t, css, ":root")
		if len(bodies) != 2 {
			t.Fatalf("exact :root selector matched %d rules, want 2", len(bodies))
		}
		properties := devicesThemeMergedProperties(t, bodies)
		for property, want := range map[string]string{
			"--first-only":  "first",
			"--second-only": "second",
			"--overridden":  "later",
		} {
			if got := properties[property]; got != want {
				t.Errorf("%s = %q, want %q", property, got, want)
			}
		}
		for _, property := range []string{"--wrong-rule", "--wrong-descendant"} {
			if _, exists := properties[property]; exists {
				t.Errorf("non-exact :root rule contributed %s", property)
			}
		}
	})

	t.Run("missing exact selector", func(t *testing.T) {
		_, err := devicesThemeFindRuleBodies(`.lf-app-shell-header {}`, ".lf-app-shell")
		if err == nil {
			t.Fatal("missing exact selector returned no error")
		}
		want := `CSS selector ".lf-app-shell" not found`
		if err.Error() != want {
			t.Errorf("missing-selector error = %q, want %q", err, want)
		}
	})
}

func TestDevicesThemeIntegrationRemainsReadOnly(t *testing.T) {
	root := devicesThemeRepositoryRoot(t)
	appShell := devicesThemeReadFile(t, filepath.Join(root, "static", "css", "app-shell.css"))
	consumedTokens := []string{
		"--lf-page-background",
		"--lf-navigation-surface",
		"--lf-device-list-surface",
		"--lf-workspace-surface",
		"--lf-panel-elevated",
		"--lf-panel-secondary",
		"--lf-surface-selected",
		"--lf-surface-subdued",
		"--lf-text-primary",
		"--lf-text-secondary",
		"--lf-text-muted",
		"--lf-text-technical",
		"--lf-accent-primary",
		"--lf-accent-secondary",
		"--lf-accent-highlight",
		"--lf-glow-ambient",
		"--lf-glow-selected",
		"--lf-glow-emphasized",
		"--lf-border-subtle",
		"--lf-border-panel",
		"--lf-border-selected",
		"--lf-border-emphasized",
		"--lf-shadow-soft",
		"--lf-shadow-elevated",
		"--lf-surface-hover",
		"--lf-focus-ring",
	}
	for _, token := range consumedTokens {
		if !strings.Contains(appShell, "var("+token+")") {
			t.Errorf("app-shell.css does not consume %s", token)
		}
	}

	shellProperties := devicesThemeProperties(t, devicesThemeRuleBody(t, appShell, ".lf-app-shell"))
	for _, token := range devicesThemeContract {
		if _, ok := shellProperties[token]; ok {
			t.Errorf("app-shell.css redeclares theme-owned token %s", token)
		}
	}
	if match := regexp.MustCompile(`var\(\s*--lf-[a-z0-9-]+\s*,`).FindString(appShell); match != "" {
		t.Errorf("app-shell.css contains a scattered semantic fallback: %s", match)
	}
	for _, fixedAccent := range []string{
		"#4d8dff",
		"#35d8ff",
		"#e146ff",
		"#9c5cff",
		"rgba(77, 141, 255",
		"rgba(53, 216, 255",
	} {
		if strings.Contains(strings.ToLower(appShell), fixedAccent) {
			t.Errorf("app-shell.css retains fixed semantic accent %q", fixedAccent)
		}
	}

	devicesTemplate := devicesThemeReadFile(t, filepath.Join(root, "web", "devices.html"))
	for _, renderedColor := range []string{
		`fill="{{ .Hex }}"`,
		`class="lf-lighting-color-hex">{{ .Hex }}</code>`,
		`class="lf-lighting-color-rgb">{{ .RGB }}</span>`,
	} {
		if !strings.Contains(devicesTemplate, renderedColor) {
			t.Errorf("device palette no longer renders the server value %q", renderedColor)
		}
	}
	lowerTemplate := strings.ToLower(devicesTemplate)
	for _, forbiddenControl := range []string{
		"<form",
		"<input",
		"<select",
		"<script",
		"fetch(",
		"setinterval(",
		"/api/",
		"onclick=",
		"onchange=",
	} {
		if strings.Contains(lowerTemplate, forbiddenControl) {
			t.Errorf("read-only Devices template contains %q", forbiddenControl)
		}
	}

	for _, templateName := range []string{"head.html", "xeneon.html"} {
		templateSource := devicesThemeReadFile(t, filepath.Join(root, "web", templateName))
		defaultTheme := `/static/css/themes/default.css`
		selectedTheme := `/static/css/themes/{{ .Dashboard.Theme }}.css`
		if strings.Count(templateSource, defaultTheme) != 2 || strings.Count(templateSource, selectedTheme) != 2 {
			t.Errorf("%s must load and preload the canonical default plus the selected theme", templateName)
		}
		if strings.Index(templateSource, defaultTheme) > strings.Index(templateSource, selectedTheme) ||
			strings.LastIndex(templateSource, defaultTheme) > strings.LastIndex(templateSource, selectedTheme) {
			t.Errorf("%s must load the canonical default before a selected custom theme", templateName)
		}
		if !strings.Contains(templateSource, `{{ if ne .Dashboard.Theme "default" }}`) {
			t.Errorf("%s does not avoid loading the default theme twice", templateName)
		}
	}

	settingsTemplate := devicesThemeReadFile(t, filepath.Join(root, "web", "settings.html"))
	settingsScript := devicesThemeReadFile(t, filepath.Join(root, "static", "js", "settings.js"))
	if !strings.Contains(settingsTemplate, `id="theme"`) || !strings.Contains(settingsScript, `$("#theme").val()`) {
		t.Error("existing theme selector identifier changed")
	}
}

func devicesThemeRepositoryRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate theme contract test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func devicesThemeReadFile(t *testing.T, path string) string {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}

func devicesThemeRuleBody(t *testing.T, css, selector string) string {
	t.Helper()

	bodies := devicesThemeRuleBodies(t, css, selector)
	return bodies[0]
}

func devicesThemeRuleBodies(t *testing.T, css, selector string) []string {
	t.Helper()

	bodies, err := devicesThemeFindRuleBodies(css, selector)
	if err != nil {
		t.Fatal(err)
	}
	return bodies
}

func devicesThemeFindRuleBodies(css, selector string) ([]string, error) {
	selectorPattern := regexp.MustCompile(`(?m)(?:^|,)\s*` + regexp.QuoteMeta(selector) + `\s*(?:,|\{)`)
	matches := selectorPattern.FindAllStringIndex(css, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("CSS selector %q not found", selector)
	}

	bodies := make([]string, 0, len(matches))
	seenOpeningBraces := make(map[int]struct{}, len(matches))
	for _, match := range matches {
		openingIndex := match[1] - 1
		if css[openingIndex] != '{' {
			openingOffset := strings.IndexByte(css[match[1]:], '{')
			if openingOffset < 0 {
				return nil, fmt.Errorf("CSS selector %q has no rule body", selector)
			}
			openingIndex = match[1] + openingOffset
			if strings.ContainsAny(css[match[1]:openingIndex], ";}") {
				return nil, fmt.Errorf("CSS selector %q has no rule body", selector)
			}
		}
		if _, seen := seenOpeningBraces[openingIndex]; seen {
			continue
		}
		seenOpeningBraces[openingIndex] = struct{}{}

		depth := 0
		closed := false
		for index := openingIndex; index < len(css); index++ {
			switch css[index] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					bodies = append(bodies, css[openingIndex+1:index])
					closed = true
				}
			}
			if closed {
				break
			}
		}
		if !closed {
			return nil, fmt.Errorf("CSS selector %q has an unterminated rule body", selector)
		}
	}
	return bodies, nil
}

func devicesThemeProperties(t *testing.T, ruleBody string) map[string]string {
	t.Helper()

	properties := make(map[string]string)
	for _, match := range cssCustomPropertyPattern.FindAllStringSubmatch(ruleBody, -1) {
		name := match[1]
		value := strings.TrimSpace(match[2])
		if previous, exists := properties[name]; exists && previous != value {
			t.Errorf("custom property %s has conflicting values %q and %q", name, previous, value)
			continue
		}
		properties[name] = value
	}
	return properties
}

func devicesThemeMergedProperties(t *testing.T, ruleBodies []string) map[string]string {
	t.Helper()

	properties := make(map[string]string)
	for _, ruleBody := range ruleBodies {
		for name, value := range devicesThemeProperties(t, ruleBody) {
			properties[name] = value
		}
	}
	return properties
}
