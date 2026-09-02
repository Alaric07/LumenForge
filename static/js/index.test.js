"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const test = require("node:test");
const dashboardDevicePresentation = require("./index.js");

test("current Dashboard devices normalize into native and grouped OpenRGB collections", function () {
    const devices = dashboardDevicePresentation.normalize({
        native: [{serial: "native-1", name: "Desk cooling"}],
        openrgb: [{serial: "openrgb-1", name: "Panel lights"}],
        memory: [{serial: "memory-1", channelId: 0, celsius: 32}]
    });

    assert.equal(devices.native.length, 1);
    assert.equal(devices.openrgb.length, 1);
    assert.equal(devices.native[0].name, "Desk cooling");
    assert.equal(devices.openrgb[0].name, "Panel lights");
    assert.equal(devices.memory[0].channelId, 0);
});

test("current Dashboard device links use stable authoritative serials", function () {
    assert.equal(dashboardDevicePresentation.deviceURL("device-1"), "/devices?device=device-1");
    assert.equal(dashboardDevicePresentation.deviceURL("device;1"), "/devices?device=device%3B1");
});

test("missing current device collections render as empty rather than legacy membership data", function () {
    const devices = dashboardDevicePresentation.normalize({devices: ["legacy-selected-device"]});

    assert.deepEqual(devices.native, []);
    assert.deepEqual(devices.openrgb, []);
    assert.deepEqual(devices.memory, []);
});

test("lower Dashboard presentation uses the current-device source, not legacy selected membership", function () {
    const source = fs.readFileSync(__dirname + "/index.js", "utf8");

    assert.match(source, /\/api\/dashboard\/devices\/current/);
    assert.doesNotMatch(source, /\/api\/dashboard\/devices\/get/);
    assert.doesNotMatch(source, /dashboard\.Devices/);
    assert.doesNotMatch(source, /sortable\(/);
});

test("lower Dashboard device labels use the template-provided localization bridge", function () {
    const source = fs.readFileSync(__dirname + "/index.js", "utf8");

    assert.match(source, /const dashboardI18n = window\.dashboardI18n \|\| \{\};/);
    assert.match(source, /text: dashboardI18n\.lighting/);
    assert.match(source, /text: dashboardI18n\.brightness/);
    assert.doesNotMatch(source, /text: "Lighting"/);
    assert.doesNotMatch(source, /text: "Brightness"/);
});
