"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const root = path.join(__dirname, "..", "..");
const source = fs.readFileSync(path.join(__dirname, "app-shell.js"), "utf8");
const navTemplate = fs.readFileSync(path.join(root, "web", "modern-nav.html"), "utf8");
const devicesTemplate = fs.readFileSync(path.join(root, "web", "devices.html"), "utf8");
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

test("modern navigation keeps Settings in a separate bottom utility group", function () {
    assert.match(navTemplate, /<nav class="lf-global-links">[\s\S]*?href="\/macros"[\s\S]*?<\/nav>\s*<nav class="lf-global-utility-links"[\s\S]*?href="\/settings"/);
    assert.match(css, /\.lf-app-shell \.lf-global-utility-links \{[\s\S]*?margin-top: auto;/);
    assert.match(css, /\.lf-app-shell\.lf-global-nav-collapsed \.lf-global-utility-links \{[\s\S]*?width: 100%;/);
    assert.match(css, /@media \(max-width: 760px\)[\s\S]*?\.lf-app-shell \.lf-global-utility-links \{[\s\S]*?margin-top: 8px;/);
});

test("Devices drawer reuses the existing panel and has selected/no-selection initial state", function () {
    assert.match(devicesTemplate, /id="lf-device-drawer"/);
    assert.match(devicesTemplate, /if not \$hasSelectedDevice.*lf-device-panel-open/);
    assert.match(navTemplate, /data-lf-devices-drawer-toggle/);
    assert.match(navTemplate, /aria-controls="lf-device-drawer"/);
    assert.match(source, /event\.key === "Escape"/);
    assert.match(source, /lf-device-panel-open/);
    assert.match(devicesTemplate, /href="\/devices\?device=\{\{ \.Serial \}\}"/);
    assert.match(css, /position: fixed/);
    assert.match(css, /@media \(max-width: 760px\)/);
    assert.match(css, /prefers-reduced-motion: reduce/);
});
