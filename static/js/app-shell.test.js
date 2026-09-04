"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const root = path.join(__dirname, "..", "..");
const source = fs.readFileSync(path.join(__dirname, "app-shell.js"), "utf8");
const navTemplate = fs.readFileSync(path.join(root, "web", "modern-nav.html"), "utf8");
const devicesTemplate = fs.readFileSync(path.join(root, "web", "devices.html"), "utf8");
const indexTemplate = fs.readFileSync(path.join(root, "web", "index.html"), "utf8");
const clusterTemplate = fs.readFileSync(path.join(root, "web", "cluster.html"), "utf8");
const headTemplate = fs.readFileSync(path.join(root, "web", "head.html"), "utf8");
const lcdTemplate = fs.readFileSync(path.join(root, "web", "lcd.html"), "utf8");
const lcdUploadTemplate = fs.readFileSync(path.join(root, "web", "lcd-upload.html"), "utf8");
const lcdArcTemplate = fs.readFileSync(path.join(root, "web", "lcd-arc.html"), "utf8");
const lcdDoubleArcTemplate = fs.readFileSync(path.join(root, "web", "lcd-doublearc.html"), "utf8");
const lcdAnimationTemplate = fs.readFileSync(path.join(root, "web", "lcd-animation.html"), "utf8");
const temperatureGraphTemplate = fs.readFileSync(path.join(root, "web", "temperatureGraph.html"), "utf8");
const systemTemplates = ["settings.html", "lcd.html", "macros.html", "temperature.html", "temperatureGraph.html"]
    .map(function (file) { return fs.readFileSync(path.join(root, "web", file), "utf8"); });
const css = fs.readFileSync(path.join(root, "static", "css", "app-shell.css"), "utf8");

test("sparkline inspection readouts use an in-strip semantic overlay", function () {
    const readout = css.match(/\.lf-dashboard-sparkline-readout \{[^}]+\}/)?.[0] || "";
    assert.match(readout, /position: absolute/);
    assert.match(readout, /right: 2px/);
    assert.match(readout, /top: 2px/);
    assert.match(readout, /display: block/);
    assert.match(readout, /visibility: visible/);
    assert.match(readout, /opacity: 1/);
    assert.match(readout, /z-index: 2/);
    assert.match(readout, /padding: 2px 6px/);
    assert.match(readout, /border-radius: 6px/);
    assert.match(readout, /background: var\(--lf-panel-elevated\)/);
    assert.match(readout, /color: var\(--lf-text-primary\)/);
    assert.match(readout, /pointer-events: none/);
    assert.match(readout, /white-space: nowrap/);
    assert.match(css, /\.lf-dashboard-sparkline-readout\[hidden\] \{\s*display: none;/);
});

test("modern shell uses the established sidebar preference and endpoint", function () {
    assert.match(source, /lumenforge-sidebarCollapsed/);
    assert.match(source, /\/api\/dashboard\/sidebar/);
    assert.match(source, /sidebarCollapsed: collapsed/);
    assert.match(navTemplate, /Dashboard\.SidebarCollapsed/);
});

test("modern navigation exposes a reusable collapse control and preserves icon labels", function () {
    assert.match(navTemplate, /define "modern-nav"/);
    assert.match(navTemplate, /data-lf-global-nav-toggle/);
    assert.match(navTemplate, /Collapse navigation/);
    assert.match(navTemplate, /Expand navigation/);
    assert.match(navTemplate, /aria-label="Dashboard"/);
    assert.match(css, /--lf-global-nav-expanded-width: 248px/);
    assert.match(css, /--lf-global-nav-collapsed-width: 84px/);
    assert.match(css, /--lf-global-nav-width: var\(--lf-global-nav-expanded-width\)/);
    assert.match(css, /--lf-global-nav-width: var\(--lf-global-nav-collapsed-width\)/);
});

test("expanded desktop navigation centers only branding while rows retain full left-aligned width", function () {
    assert.match(css, /\.lf-app-shell:not\(\.lf-global-nav-collapsed\) \.lf-brand \{[\s\S]*?justify-content: center;[\s\S]*?text-align: center;/);
    assert.match(css, /\.lf-app-shell:not\(\.lf-global-nav-collapsed\) \.lf-global-links \{[\s\S]*?align-items: stretch;/);
    assert.match(css, /\.lf-app-shell:not\(\.lf-global-nav-collapsed\) \.lf-global-link \{[\s\S]*?width: 100%;[\s\S]*?justify-content: flex-start;/);
    assert.match(css, /\.lf-app-shell\.lf-global-nav-collapsed \.lf-global-link \{[\s\S]*?justify-content: center;/);
});

test("modern navigation keeps System in a separate bottom utility group", function () {
    const mainGroup = navTemplate.match(/<nav class="lf-global-links">([\s\S]*?)<\/nav>/)?.[1] || "";
    assert.doesNotMatch(mainGroup, /Cooling profiles|aria-label="LCD"|aria-label="Macros"/);
    assert.match(navTemplate, /<nav class="lf-global-utility-links"[\s\S]*?aria-label="System"/);
    assert.match(navTemplate, /<button class="lf-global-link lf-global-system-toggle/);
    assert.doesNotMatch(navTemplate, /href="\/settings" aria-label="System"/);
    assert.match(css, /\.lf-app-shell \.lf-global-utility-links \{[\s\S]*?margin-top: auto;/);
    assert.match(css, /\.lf-app-shell\.lf-global-nav-collapsed \.lf-global-utility-links \{[\s\S]*?width: 100%;/);
    assert.match(css, /@media \(max-width: 760px\)[\s\S]*?\.lf-app-shell \.lf-global-utility-links \{[\s\S]*?margin-top: 8px;/);
});

test("System is a drawer-only toggle on every modern shell", function () {
    const utilityGroup = navTemplate.match(/<nav class="lf-global-utility-links"[^>]*>([\s\S]*?)<\/nav>/)?.[1] || "";
    assert.match(navTemplate, /data-lf-system-drawer-toggle data-lf-drawer-toggle aria-controls="lf-system-drawer" aria-expanded="false"/);
    assert.match(utilityGroup, /<button/);
    assert.doesNotMatch(utilityGroup, /href="\/settings"/);
    for (const template of [indexTemplate, clusterTemplate, devicesTemplate]) {
        assert.match(template, /template "modern-system-drawer"/);
    }
    assert.match(source, /for \(const otherDrawer of drawers\)/);
    assert.match(source, /setDrawerOpen\(otherDrawer, otherToggle, false\)/);
});

test("System routes use the modern shell with a closed, active System drawer", function () {
    for (const template of systemTemplates) {
        assert.match(template, /lf-app-shell lf-system-shell/);
        assert.match(template, /template "modern-nav"/);
        assert.match(template, /template "modern-system-drawer"/);
        assert.match(template, /lf-system-workspace/);
        assert.doesNotMatch(template, /template "sidebar"|sidebar\.js|main-content/);
    }
    assert.match(navTemplate, /define "modern-system-drawer"/);
    assert.match(navTemplate, /id="lf-system-drawer"/);
    assert.match(navTemplate, /data-lf-system-drawer-toggle[\s\S]*?aria-controls="lf-system-drawer"[\s\S]*?aria-expanded="false"/);
    assert.match(navTemplate, /href="\/settings"[\s\S]*?href="\/lcd"[\s\S]*?href="\/macros"[\s\S]*?href="\/temperature"/);
    assert.match(navTemplate, /eq \.Page "settings"[\s\S]*?eq \.Page "lcd"[\s\S]*?eq \.Page "macros"[\s\S]*?eq \.Page "temperature"/);
    assert.match(navTemplate, /lf-device-list" aria-label="System navigation"/);
    assert.match(navTemplate, /lf-device-item\{\{ if eq \.Page "settings" \}\} lf-device-item-active/);
    assert.match(navTemplate, /class="lf-device-panel lf-system-panel" id="lf-system-drawer"/);
    assert.doesNotMatch(navTemplate, /lf-system-panel lf-system-panel-open/);
    assert.match(css, /\.lf-app-shell \.lf-device-panel,\s*\.lf-app-shell \.lf-system-panel \{/);
    assert.match(css, /\.lf-app-shell \.lf-system-workspace/);
    assert.match(headTemplate, /eq \.Page "settings"[\s\S]*?eq \.Page "lcd"[\s\S]*?eq \.Page "macros"[\s\S]*?eq \.Page "temperature"[\s\S]*?app-shell\.js/);
    assert.match(headTemplate, /eq \.Page "settings"[\s\S]*?eq \.Page "lcd"[\s\S]*?eq \.Page "macros"[\s\S]*?eq \.Page "temperature"[\s\S]*?app-shell\.css/);
    assert.match(temperatureGraphTemplate, /template "temperature-bar"/);
    assert.match(temperatureGraphTemplate, /id="graphPump"[\s\S]*?id="graphFans"/);
    assert.match(temperatureGraphTemplate, /static\/js\/temperature\.js/);
});

test("LCD uses a modern System workspace without legacy layout or temperature-bar content", function () {
    assert.match(lcdTemplate, /lf-app-shell lf-system-shell[\s\S]*?template "modern-nav"[\s\S]*?template "modern-system-drawer"[\s\S]*?lf-system-workspace/);
    assert.match(lcdTemplate, /lf-lcd-workspace[\s\S]*?lf-lcd-header[\s\S]*?lf-lcd-grid/);
    assert.match(lcdTemplate, /static\/js\/lcd\.js/);
    assert.doesNotMatch(lcdTemplate, /template "sidebar"|sidebar\.js|temperature-bar|lf-system-legacy-content|page-centered-wrapper|class="row/);
    for (const template of [lcdUploadTemplate, lcdArcTemplate, lcdDoubleArcTemplate, lcdAnimationTemplate]) {
        assert.match(template, /lf-lcd-panel/);
        assert.doesNotMatch(template, /col-(?:lg|md)|\bcard(?:-|\s|\")|settings-(?:list|row)|system-card/);
    }
    assert.match(lcdUploadTemplate, /id="gifUploadForm"[\s\S]*?id="animationFile"[\s\S]*?class="lf-lcd-button uploadGifImage" id="uploadGifImage"/);
    assert.match(lcdArcTemplate, /sensorType_\{\{ \$arc\.Id \}\}[\s\S]*?margin_\{\{ \$arc\.Id \}\}[\s\S]*?saveArcProfile/);
    assert.match(lcdDoubleArcTemplate, /margin_\{\{ \$doubleArc\.Id \}\}[\s\S]*?separatorColor_\{\{ \$doubleArc\.Id \}\}[\s\S]*?sensorType_\{\{ \$key \}\}[\s\S]*?saveDoubleArcProfile/);
    assert.match(lcdAnimationTemplate, /backgroundImage_\{\{ \$animation\.Id \}\}[\s\S]*?workers_\{\{ \$animation\.Id \}\}[\s\S]*?sensorEnabled_\{\{ \$key \}\}_\{\{ \$animation\.Id \}\}[\s\S]*?saveAnimationProfile/);
    assert.match(lcdAnimationTemplate, /lcdWorkersInfo[\s\S]*?lcdFrameDelayInfo/);
    assert.match(css, /\.lf-app-shell \.lf-lcd-grid \{[\s\S]*?grid-template-columns: repeat\(2, minmax\(0, 1fr\)\)/);
    assert.match(css, /@media \(max-width: 920px\)[\s\S]*?\.lf-app-shell \.lf-lcd-grid \{[\s\S]*?grid-template-columns: minmax\(0, 1fr\)/);
});

test("System selection closes through the shared drawer handler and can be reopened", function () {
    assert.match(navTemplate, /href="\/settings" data-lf-drawer-item/);
    assert.match(navTemplate, /href="\/lcd" data-lf-drawer-item/);
    assert.match(navTemplate, /href="\/macros" data-lf-drawer-item/);
    assert.match(navTemplate, /href="\/temperature" data-lf-drawer-item/);
    assert.match(source, /setDrawerOpen\(drawer, drawerToggle, false\)/);
    assert.match(source, /!drawer\.classList\.contains\(openClass\)/);
    assert.match(source, /toggle\.setAttribute\("aria-expanded", String\(open\)\)/);
    assert.match(source, /event\.key !== "Escape"/);
});

test("drawer handling is shared without changing Devices identifiers", function () {
    assert.match(devicesTemplate, /data-lf-device-drawer data-lf-drawer/);
    assert.match(devicesTemplate, /data-lf-drawer-item/);
    assert.match(source, /querySelectorAll\("\[data-lf-drawer\]"\)/);
    assert.match(source, /data-lf-drawer-open-class/);
    assert.match(source, /data-lf-drawer-item/);
    assert.match(source, /event\.key !== "Escape"/);
});

test("Devices drawer reuses the existing panel and has selected/no-selection initial state", function () {
    assert.match(devicesTemplate, /id="lf-device-drawer"/);
    assert.match(devicesTemplate, /if not \$hasSelectedDevice.*lf-device-panel-open/);
    assert.match(navTemplate, /data-lf-devices-drawer-toggle/);
    assert.match(navTemplate, /data-lf-devices-drawer-toggle data-lf-drawer-toggle/);
    assert.match(navTemplate, /aria-controls="lf-device-drawer"/);
    assert.match(source, /event\.key !== "Escape"/);
    assert.match(source, /lf-device-panel-open/);
    assert.match(devicesTemplate, /href="\/devices\?device=\{\{ \.Serial \}\}"/);
    assert.match(css, /position: fixed/);
    assert.match(css, /@media \(max-width: 760px\)/);
    assert.match(css, /prefers-reduced-motion: reduce/);
});
