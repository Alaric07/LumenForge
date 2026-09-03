"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const repositoryRoot = path.resolve(__dirname, "../..");
const settingsScript = fs.readFileSync(path.join(__dirname, "settings.js"), "utf8");
const settingsTemplate = fs.readFileSync(path.join(repositoryRoot, "web/settings.html"), "utf8");
const languageDirectory = path.join(repositoryRoot, "database/language");

test("Dashboard settings retain active controls without legacy membership state", function () {
    assert.doesNotMatch(settingsTemplate, /checkbox-addDeviceToDashboard|txtAddDeviceToDashboard/);
    assert.doesNotMatch(settingsScript, /addDeviceToDashboard|checkboxAddDeviceToDashboard/);
    assert.match(settingsScript, /response\.dashboard\.temperatureBar/);
    assert.match(settingsScript, /pf\["temperatureBar"\]/);
    assert.match(settingsScript, /pf\["languageCode"\]/);
    assert.match(settingsScript, /pf\["theme"\]/);
});

test("Settings uses the modern workspace while preserving all functional sections and controls", function () {
    assert.match(settingsTemplate, /lf-app-shell lf-system-shell/);
    assert.match(settingsTemplate, /template "modern-nav"/);
    assert.match(settingsTemplate, /template "modern-system-drawer"/);
    assert.match(settingsTemplate, /lf-settings-workspace|lf-system-workspace/);
    assert.doesNotMatch(settingsTemplate, /template "sidebar"|sidebar\.js|template "temperature-bar"|temperature-bar\.js/);
    for (const section of ["Preferences", "Automation", "txtDisplay", "txtVirtualAudio", "txtBackupRestore", "OpenRGB SDK Integration", "txtSupportedDevices"]) {
        assert.match(settingsTemplate, new RegExp(section));
    }
    for (const id of ["theme", "btnSaveDashboardSettings", "rgbControl", "lcdControl", "virtualAudio", "restoreForm", "btnBackup", "backupFile", "btnRestore", "btnOpenRGBDiscover", "btnOpenRGBRefresh", "dataTable", "btnSaveSupportedDevices"]) {
        assert.match(settingsTemplate, new RegExp('id="' + id + '"'));
    }
    const preferences = settingsTemplate.match(/<section class="lf-settings-panel">[\s\S]*?<\/section>/)?.[0] || "";
    assert.match(preferences, /id="theme"/);
});

test("restore success keeps the server restart message prominent", function () {
    assert.match(settingsScript, /toast\.success\(response\)/);
    assert.match(settingsScript, /restoreRestartWarning/);
    assert.match(settingsTemplate, /id="restoreRestartWarning"[^>]*text-warning[^>]*fw-bold/);
    assert.match(settingsTemplate, /txtRestoreRestartWarning/);
});

test("restore errors and file selection use localized template messages", function () {
    assert.match(settingsTemplate, /data-select-message="\{\{ \.Lang "txtSelectBackupFile" \}\}"/);
    assert.match(settingsTemplate, /data-failure-prefix="\{\{ \.Lang "txtRestoreFailed" \}\}"/);
    assert.match(settingsScript, /restoreForm\.data\("select-message"\)/);
    assert.match(settingsScript, /restoreForm\.data\("failure-prefix"\)/);
});

test("settings identifies the repository-only backup guide without a moving branch link", function () {
    assert.match(settingsTemplate, /<code>docs\/backup-restore\.md<\/code>\./);
    assert.match(settingsTemplate, /txtRestoreGuide/);
    assert.doesNotMatch(settingsTemplate, /LumenForge-Dev\/docs\/backup-restore|main\/docs\/backup-restore/);
    assert.doesNotMatch(settingsTemplate, /href="[^"]*backup-restore\.md/);
    const english = JSON.parse(fs.readFileSync(path.join(languageDirectory, "en_US.json"), "utf8"));
    assert.equal(
        english.values.txtRestoreGuide,
        "Backup and restore guide: Available in the LumenForge GitHub repository under"
    );
    for (const filename of fs.readdirSync(languageDirectory).filter((name) => name.endsWith(".json"))) {
        const language = JSON.parse(fs.readFileSync(path.join(languageDirectory, filename), "utf8"));
        assert.match(language.values.txtRestoreGuide, /GitHub/, `${filename} identifies the GitHub repository`);
    }
    const restoreHandler = settingsScript.match(
        /\$\("#restoreForm"\)\.on\("submit"[\s\S]+?\n    \}\);/
    );
    assert.ok(restoreHandler, "restore submit handler is present");
    assert.doesNotMatch(restoreHandler[0], /systemctl|\/api\/restart|location\.reload/);
});
