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

func TestDevicesThemeIntegrationContract(t *testing.T) {
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
		"#5599ff",
	} {
		if strings.Contains(strings.ToLower(appShell), fixedAccent) {
			t.Errorf("app-shell.css retains fixed semantic accent %q", fixedAccent)
		}
	}

	devicesTemplate := devicesThemeReadFile(t, filepath.Join(root, "web", "devices.html"))
	iconRule := devicesThemeRuleBody(t, appShell, ".lf-app-shell .lf-lighting-effect-icon-art")
	for _, contract := range []string{
		"background-color: var(--lf-accent-highlight)",
		"-webkit-mask-image: var(--lf-lighting-effect-mask)",
		"mask-image: var(--lf-lighting-effect-mask)",
		"-webkit-mask-repeat: no-repeat",
		"mask-repeat: no-repeat",
		"-webkit-mask-position: center",
		"mask-position: center",
		"-webkit-mask-size: contain",
		"mask-size: contain",
		"var(--lf-glow-selected)",
	} {
		if !strings.Contains(iconRule, contract) {
			t.Errorf("effect icon CSS does not contain %q", contract)
		}
	}
	fallbackRule := devicesThemeRuleBody(t, appShell, ".lf-app-shell .lf-lighting-effect-icon-fallback")
	if !strings.Contains(fallbackRule, "color: var(--lf-text-muted)") {
		t.Error("generic effect icon fallback does not use the subdued semantic text token")
	}
	for _, theme := range []string{"default", "catppuccin-macchiato", "cyberpunk", "dracula", "tokyonight"} {
		themeCSS := devicesThemeReadFile(t, filepath.Join(root, "static", "css", "themes", theme+".css"))
		if strings.Contains(themeCSS, "lf-lighting-effect-icon") {
			t.Errorf("theme %q duplicates the shared effect-icon component", theme)
		}
		if strings.Contains(themeCSS, "lf-range-control") {
			t.Errorf("theme %q duplicates the shared range-control component", theme)
		}
	}

	rangeRules := []struct {
		selector  string
		contracts []string
	}{
		{selector: ".lf-app-shell .lf-range-control-input::-webkit-slider-runnable-track", contracts: []string{
			"var(--lf-control-fill-start)",
			"var(--lf-control-fill-end)",
			"var(--lf-range-progress)",
			"var(--lf-control-track)",
		}},
		{selector: ".lf-app-shell .lf-range-control-input::-webkit-slider-thumb", contracts: []string{
			"var(--lf-control-thumb)",
			"var(--lf-control-thumb-border)",
			"var(--lf-control-thumb-glow)",
		}},
		{selector: ".lf-app-shell .lf-range-control-input::-moz-range-track", contracts: []string{
			"var(--lf-control-track)",
		}},
		{selector: ".lf-app-shell .lf-range-control-input::-moz-range-progress", contracts: []string{
			"var(--lf-control-fill-start)",
			"var(--lf-control-fill-end)",
		}},
		{selector: ".lf-app-shell .lf-range-control-input::-moz-range-thumb", contracts: []string{
			"var(--lf-control-thumb)",
			"var(--lf-control-thumb-border)",
			"var(--lf-control-thumb-glow)",
		}},
		{selector: ".lf-app-shell .lf-range-control-input:focus-visible", contracts: []string{
			"var(--lf-focus-ring)",
		}},
		{selector: ".lf-app-shell .lf-range-control-input:disabled", contracts: []string{
			"cursor: not-allowed",
		}},
		{selector: ".lf-app-shell .lf-color-control-input:focus-visible", contracts: []string{
			"var(--lf-focus-ring)",
		}},
		{selector: ".lf-app-shell .lf-range-control-input:disabled::-webkit-slider-runnable-track", contracts: []string{
			"var(--lf-control-disabled-track)",
			"var(--lf-disabled-border)",
		}},
		{selector: ".lf-app-shell .lf-range-control-input:disabled::-moz-range-progress", contracts: []string{
			"var(--lf-control-disabled-track)",
			"var(--lf-disabled-border)",
		}},
		{selector: ".lf-app-shell .lf-range-control-number", contracts: []string{
			"width: 6ch",
			"color: var(--lf-text-primary)",
			"background: transparent",
			"font-variant-numeric: tabular-nums",
		}},
		{selector: ".lf-app-shell .lf-range-control-number:focus-visible", contracts: []string{
			"var(--lf-focus-ring)",
		}},
		{selector: ".lf-app-shell .lf-range-control-number:disabled", contracts: []string{
			"var(--lf-text-muted)",
			"cursor: not-allowed",
		}},
		{selector: ".lf-app-shell .lf-range-control-status:empty", contracts: []string{
			"display: none",
		}},
		{selector: ".lf-app-shell .lf-range-control-speed:not(.lf-range-control-ready)", contracts: []string{
			"visibility: hidden",
		}},
	}
	for _, rangeRule := range rangeRules {
		rule := devicesThemeRuleBody(t, appShell, rangeRule.selector)
		for _, contract := range rangeRule.contracts {
			if !strings.Contains(rule, contract) {
				t.Errorf("range selector %q does not contain %q", rangeRule.selector, contract)
			}
		}
	}
	rangeStart := strings.Index(appShell, ".lf-app-shell .lf-range-control {")
	rangeEnd := strings.Index(appShell, ".lf-app-shell .lf-color-control {")
	if rangeStart < 0 || rangeEnd <= rangeStart {
		t.Error("shared range-control CSS block is missing or misplaced")
	} else if literal := regexp.MustCompile(`(?i)#[0-9a-f]{3,8}|rgba?\(`).FindString(appShell[rangeStart:rangeEnd]); literal != "" {
		t.Errorf("shared range-control CSS contains a theme-specific literal color %q", literal)
	}

	lowerTemplate := strings.ToLower(devicesTemplate)
	for _, forbiddenControl := range []string{
		"<form",
		"<script",
		"fetch(",
		"setinterval(",
		"/api/",
		"onclick=",
		"onchange=",
	} {
		if strings.Contains(lowerTemplate, forbiddenControl) {
			t.Errorf("Devices template contains prohibited control or behavior %q", forbiddenControl)
		}
	}
	for _, selectorContract := range []string{
		`<label class="lf-lighting-label" for="lf-lighting-effect-selector">`,
		`class="lf-lighting-effect-selector"`,
		`data-lf-effect-selector`,
		`aria-live="polite"`,
	} {
		if !strings.Contains(devicesTemplate, selectorContract) {
			t.Errorf("Devices effect selector is missing %q", selectorContract)
		}
	}
	for _, rangeContract := range []string{
		`<label class="lf-range-control-label" for="lf-lighting-brightness-slider">`,
		`class="lf-range-control-number"`,
		`type="number"`,
		`min="0"`,
		`max="100"`,
		`step="1"`,
		`data-lf-brightness-number`,
		`aria-label="Brightness percentage"`,
		`class="lf-range-control-suffix" aria-hidden="true">%</span>`,
		`class="lf-range-control-input"`,
		`type="range"`,
		`data-lf-brightness-slider`,
		`aria-live="polite"`,
	} {
		if !strings.Contains(devicesTemplate, rangeContract) {
			t.Errorf("Devices brightness range control is missing %q", rangeContract)
		}
	}
	for _, forbiddenCopy := range []string{
		"Stored local output level for this device.",
		"Changes are saved when the value is committed.",
		"Brightness unavailable for this stored lighting profile.",
	} {
		if strings.Contains(devicesTemplate, forbiddenCopy) {
			t.Errorf("Devices brightness control retains permanent helper copy %q", forbiddenCopy)
		}
	}
	for _, speedContract := range []string{
		`<label class="lf-range-control-label" for="lf-lighting-speed-slider">Speed</label>`,
		`class="lf-range-control-number"`,
		`type="number"`,
		`min="1"`,
		`max="10"`,
		`step="0.1"`,
		`data-lf-speed-number`,
		`aria-label="Speed level"`,
		`class="lf-range-control-input"`,
		`type="range"`,
		`data-lf-speed-slider`,
		`data-lf-speed-control-mode="software"`,
		`<div class="lf-range-control-scale" aria-hidden="true"><span>Slow</span><span>Fast</span></div>`,
	} {
		if !strings.Contains(devicesTemplate, speedContract) {
			t.Errorf("Devices Speed range control is missing %q", speedContract)
		}
	}
	for _, forbiddenCopy := range []string{
		"Stored animation speed",
		"Release to save",
		"Adjust how quickly the effect moves",
		"Speed unavailable",
		"Persistent speed</small>",
		"Effective speed</span>",
	} {
		if strings.Contains(devicesTemplate, forbiddenCopy) {
			t.Errorf("Devices Speed control retains duplicate or permanent copy %q", forbiddenCopy)
		}
	}
	for _, iconContract := range []string{
		`class="lf-lighting-effect-icon-frame" aria-hidden="true"`,
		`style="--lf-lighting-effect-mask: url('{{ .ConfiguredEffectIconURL }}');"`,
		`class="lf-lighting-effect-icon-fallback"`,
	} {
		if !strings.Contains(devicesTemplate, iconContract) {
			t.Errorf("Devices effect icon template is missing %q", iconContract)
		}
	}
	if strings.Contains(devicesTemplate, `<img class="lf-lighting-effect`) {
		t.Error("Devices effect icon is rendered as an image instead of a theme-colored CSS mask")
	}
	headTemplate := devicesThemeReadFile(t, filepath.Join(root, "web", "head.html"))
	securityScript := `<script src="/static/js/security.js?v=1"></script>`
	speedScript := `<script src="/static/js/rgb-speed.js?v=3"></script>`
	lightingScript := `<script src="/static/js/devices-lighting.js" defer></script>`
	securityScriptIndex := strings.Index(headTemplate, securityScript)
	speedScriptIndex := strings.Index(headTemplate, speedScript)
	lightingScriptIndex := strings.Index(headTemplate, lightingScript)
	if securityScriptIndex < 0 {
		t.Error("protected fetch wrapper is not loaded by the shared head template")
	} else if lightingScriptIndex < 0 {
		t.Error("Devices Lighting script is not loaded by the shared head template")
	} else if speedScriptIndex < 0 {
		t.Error("shared RGB speed mapping helper is not loaded by the shared head template")
	} else if securityScriptIndex > lightingScriptIndex {
		t.Error("Devices Lighting script loads before the protected fetch wrapper")
	} else if speedScriptIndex > lightingScriptIndex {
		t.Error("Devices Lighting script loads before the shared RGB speed mapping helper")
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
