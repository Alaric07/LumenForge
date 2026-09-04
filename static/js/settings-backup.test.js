"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const repositoryRoot = path.resolve(__dirname, "../..");
const settingsScript = fs.readFileSync(path.join(__dirname, "settings.js"), "utf8");
const adminScript = fs.readFileSync(path.join(__dirname, "admin.js"), "utf8");
const settingsTemplate = fs.readFileSync(path.join(repositoryRoot, "web/settings.html"), "utf8");
const adminTemplate = fs.readFileSync(path.join(repositoryRoot, "web/admin.html"), "utf8");
const languageDirectory = path.join(repositoryRoot, "database/language");

test("Dashboard preferences retain hidden legacy values without modern controls", function () {
    assert.doesNotMatch(settingsTemplate, /checkbox-addDeviceToDashboard|txtAddDeviceToDashboard/);
    assert.doesNotMatch(settingsScript, /addDeviceToDashboard|checkboxAddDeviceToDashboard/);
    assert.doesNotMatch(settingsTemplate, /checkbox-deviceLabels|checkbox-temperatureBar|txtShowDeviceLabels|txtShowTemperatureBar/);
    assert.doesNotMatch(settingsScript, /checkboxDeviceLabels|checkboxTemperatureBar|btnSaveTheme/);
    assert.match(settingsScript, /currentDashboardShowLabels = response\.dashboard\.showLabels === true/);
    assert.match(settingsScript, /currentDashboardTemperatureBar = response\.dashboard\.temperatureBar === true/);
    assert.match(settingsScript, /showLabels: currentDashboardShowLabels/);
    assert.match(settingsScript, /temperatureBar: currentDashboardTemperatureBar/);
    assert.match(settingsScript, /languageCode: \$\("#userLanguage"\)\.val\(\)/);
});

test("Dashboard preferences do not save until preserved dashboard state has loaded", function () {
    const saveDashboardPreferences = settingsScript.match(
        /function saveDashboardPreferences\(\) \{[\s\S]*?\n\}/
    )?.[0] || "";
    const loadDashboardSettings = settingsScript.match(
        /function loadDashboardSettings\(\) \{[\s\S]*?\n    \}/
    )?.[0] || "";

    assert.match(settingsScript, /let dashboardPreferencesLoaded = false/);
    assert.match(saveDashboardPreferences, /if \(!dashboardPreferencesLoaded\) \{[\s\S]*?toast\.warning\("Dashboard preferences are still loading\."\);[\s\S]*?return;/);
    assert.ok(saveDashboardPreferences.indexOf("dashboardPreferencesLoaded") < saveDashboardPreferences.indexOf("$.ajax"));
    assert.match(loadDashboardSettings, /currentDashboardRgbOff = response\.dashboard\.rgbOff === true/);
    assert.match(loadDashboardSettings, /currentDashboardShowLabels = response\.dashboard\.showLabels === true/);
    assert.match(loadDashboardSettings, /currentDashboardTemperatureBar = response\.dashboard\.temperatureBar === true/);
    assert.ok(loadDashboardSettings.indexOf("dashboardPreferencesLoaded = true") > loadDashboardSettings.indexOf("currentDashboardTemperatureBar = response.dashboard.temperatureBar === true"));
    assert.match(loadDashboardSettings, /if \(response\.status === 1\) \{[\s\S]*?dashboardPreferencesLoaded = true;/);
});

test("Preferences owns only the preference controls in the modern System workspace", function () {
    assert.match(settingsTemplate, /lf-app-shell lf-system-shell/);
    assert.match(settingsTemplate, /template "modern-nav"/);
    assert.match(settingsTemplate, /template "modern-system-drawer"/);
    assert.match(settingsTemplate, /lf-settings-workspace|lf-system-workspace/);
    assert.doesNotMatch(settingsTemplate, /template "sidebar"|sidebar\.js|template "temperature-bar"|temperature-bar\.js/);
    for (const section of ["Preferences", "General", "Theme", "Automation", "txtDisplay", "txtVirtualAudio"]) {
        assert.match(settingsTemplate, new RegExp(section));
    }
    for (const id of ["checkbox-celsius", "userLanguage", "keyboardLayout", "theme", "btnSaveDashboardSettings", "rgbControl", "lcdControl", "virtualAudio"]) {
        assert.match(settingsTemplate, new RegExp('id="' + id + '"'));
    }
    for (const id of ["restoreForm", "btnBackup", "backupFile", "btnRestore", "btnOpenRGBDiscover", "btnOpenRGBRefresh", "dataTable", "btnSaveSupportedDevices"]) {
        assert.doesNotMatch(settingsTemplate, new RegExp('id="' + id + '"'));
    }
    assert.match(settingsTemplate, /static\/js\/settings\.js/);
    assert.doesNotMatch(settingsTemplate, /static\/js\/admin\.js/);
    assert.doesNotMatch(settingsScript, /openrgbImportModal|dataTable|restoreForm|btnBackup/);

    const generalPanel = settingsTemplate.match(
        /<section class="lf-settings-panel">[\s\S]*?<h2>General<\/h2>[\s\S]*?<\/section>/
    )?.[0] || "";
    assert.match(generalPanel, /lf-settings-theme-section/);
    assert.match(generalPanel, /id="theme"/);
    assert.match(generalPanel, /Current theme/);
    assert.match(generalPanel, /Choose the built-in theme used throughout LumenForge\./);
    assert.match(settingsTemplate, /<footer class="lf-settings-panel-footer">[\s\S]*?id="btnSaveDashboardSettings"/);
    assert.ok(settingsTemplate.indexOf('class="lf-settings-panel-footer"') > settingsTemplate.indexOf('class="lf-settings-theme-section"'));
    assert.doesNotMatch(settingsTemplate, /Appearance/);
    assert.doesNotMatch(settingsTemplate, /id="btnSaveTheme"/);
    assert.doesNotMatch(settingsTemplate, /<section class="lf-settings-panel">\s*<header class="lf-settings-panel-header">\s*<\/main>/);

    assert.match(settingsTemplate, /Configure LumenForge general settings, theme, automation, display, and audio\./);
    assert.doesNotMatch(settingsTemplate, /Personalize LumenForge appearance, behavior, automation, display, and audio\./);
    const footerPositions = [...settingsTemplate.matchAll(/<footer class="lf-settings-panel-footer">/g)].map((match) => match.index);
    assert.equal(footerPositions.length, 4);
    const footerContents = footerPositions.map((position, index) => settingsTemplate.slice(position, footerPositions[index + 1]));
    for (const [footer, saveButton] of [
        [footerContents[0], /<button class="lf-button" id="btnSaveDashboardSettings"/],
        [footerContents[1], /<button class="lf-button saveRgbControl"/],
        [footerContents[2], /<button class="lf-button updateDisplay"/],
        [footerContents[3], /<button class="lf-button enableVirtualAudio"/]
    ]) {
        assert.match(footer, saveButton);
    }

    const completePayload = settingsScript.match(
        /function buildDashboardPreferencesPayload\(\) \{[\s\S]*?\n    \}/
    )?.[0] || "";
    for (const field of ["showLabels", "celsius", "temperatureBar", "languageCode", "theme", "keyboardLayout", "rgbOff"]) {
        assert.match(completePayload, new RegExp(field));
    }
    assert.match(completePayload, /\$\("#theme"\)\.val\(\)/);
    assert.match(completePayload, /currentDashboardRgbOff/);
    assert.match(settingsScript, /currentDashboardRgbOff = response\.dashboard\.rgbOff === true/);
    const handler = settingsScript.match(
        /\$\('#btnSaveDashboardSettings'\)\.on\('click',[\s\S]*?\n        \}\);/
    )?.[0] || "";
    assert.match(handler, /saveDashboardPreferences\(\)/);
    assert.match(settingsScript, /data: JSON\.stringify\(buildDashboardPreferencesPayload\(\), null, 2\)/);
});

test("Admin owns backup, OpenRGB management, and supported devices", function () {
    assert.match(adminTemplate, /lf-app-shell lf-system-shell/);
    assert.match(adminTemplate, /template "modern-nav"/);
    assert.match(adminTemplate, /template "modern-system-drawer"/);
    assert.match(adminTemplate, /id="lf-admin-title">Admin/);
    for (const id of ["restoreForm", "btnBackup", "backupFile", "btnRestore", "restoreRestartWarning", "openrgbImportModal", "btnOpenRGBDiscover", "btnOpenRGBDiscoverAgain", "btnOpenRGBImportSelected", "btnOpenRGBRemoveSelected", "btnOpenRGBRefresh", "dataTable", "btnSaveSupportedDevices"]) {
        assert.match(adminTemplate, new RegExp('id="' + id + '"'));
    }
    for (const id of ["checkbox-celsius", "theme", "rgbControl", "virtualAudio"]) {
        assert.doesNotMatch(adminTemplate, new RegExp('id="' + id + '"'));
    }
    assert.match(adminTemplate, /static\/js\/admin\.js/);
    assert.doesNotMatch(adminTemplate, /static\/js\/settings\.js/);
    assert.match(adminScript, /\/api\/getSupportedDevices/);
    assert.match(adminScript, /\/api\/setSupportedDevices/);
    assert.match(adminScript, /openrgb-import-checkbox/);
    assert.doesNotMatch(adminScript, /\/api\/scheduler\/rgb|virtualAudio|checkbox-celsius/);

    const gridStart = adminTemplate.indexOf('<div class="lf-settings-grid">');
    const backupPanel = adminTemplate.indexOf('<section class="lf-settings-panel">', gridStart);
    const backupHeading = adminTemplate.indexOf('txtBackupRestore', backupPanel);
    const openRGBPanel = adminTemplate.indexOf('OpenRGB SDK Integration');
    const supportedDevicesPanel = adminTemplate.indexOf('txtSupportedDevices');
    const gridEnd = adminTemplate.indexOf('</div>', supportedDevicesPanel);
    assert.ok(gridStart >= 0 && backupPanel > gridStart);
    assert.ok(backupHeading > backupPanel);
    assert.ok(openRGBPanel > backupPanel && openRGBPanel < gridEnd);
    assert.ok(supportedDevicesPanel > openRGBPanel && supportedDevicesPanel < gridEnd);
    assert.doesNotMatch(adminTemplate, /template "sidebar"|sidebar\.js|temperature-bar/);
});

test("restore success keeps the server restart message prominent", function () {
    assert.match(adminScript, /toast\.success\(response\)/);
    assert.match(adminScript, /restoreRestartWarning/);
    assert.match(adminTemplate, /id="restoreRestartWarning"[^>]*lf-status-caution[^>]*fw-bold/);
    assert.match(adminTemplate, /txtRestoreRestartWarning/);
});

test("restore errors and file selection use localized template messages", function () {
    assert.match(adminTemplate, /data-select-message="\{\{ \.Lang "txtSelectBackupFile" \}\}"/);
    assert.match(adminTemplate, /data-failure-prefix="\{\{ \.Lang "txtRestoreFailed" \}\}"/);
    assert.match(adminScript, /restoreForm\.data\("select-message"\)/);
    assert.match(adminScript, /restoreForm\.data\("failure-prefix"\)/);
});

test("settings identifies the repository-only backup guide without a moving branch link", function () {
    assert.match(adminTemplate, /<code>docs\/backup-restore\.md<\/code>\./);
    assert.match(adminTemplate, /txtRestoreGuide/);
    assert.doesNotMatch(adminTemplate, /LumenForge-Dev\/docs\/backup-restore|main\/docs\/backup-restore/);
    assert.doesNotMatch(adminTemplate, /href="[^"]*backup-restore\.md/);
    const english = JSON.parse(fs.readFileSync(path.join(languageDirectory, "en_US.json"), "utf8"));
    assert.equal(
        english.values.txtRestoreGuide,
        "Backup and restore guide: Available in the LumenForge GitHub repository under"
    );
    for (const filename of fs.readdirSync(languageDirectory).filter((name) => name.endsWith(".json"))) {
        const language = JSON.parse(fs.readFileSync(path.join(languageDirectory, filename), "utf8"));
        assert.match(language.values.txtRestoreGuide, /GitHub/, `${filename} identifies the GitHub repository`);
    }
    const restoreHandler = adminScript.match(
        /\$\("#restoreForm"\)\.on\("submit"[\s\S]+?\n    \}\);/
    );
    assert.ok(restoreHandler, "restore submit handler is present");
    assert.doesNotMatch(restoreHandler[0], /systemctl|\/api\/restart|location\.reload/);
});
