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

test("Dashboard layout identities are source-namespaced and reconciliation retains reconnect positions", function () {
    const saved = [{id: "native:device-1", x: 2, y: 0, w: 1, h: 1}, {id: "openrgb:device-1", x: 0, y: 0, w: 1, h: 1}];
    const connected = dashboardDevicePresentation.reconcileLayout(saved, [
        {id: dashboardDevicePresentation.cardID("native", "device-1")},
        {id: dashboardDevicePresentation.cardID("openrgb", "device-1")}
    ]);
    assert.equal(connected[0].id, "openrgb:device-1");
    assert.equal(connected[1].id, "native:device-1");
    const reconnected = dashboardDevicePresentation.reconcileLayout(saved, [{id: "native:device-1"}]);
    assert.deepEqual(reconnected[0], {id: "native:device-1", x: 2, y: 0, w: 1, h: 1});
});

test("Dashboard layout normalization accepts only the four logical columns", function () {
    const valid = {id: "native:last-column", x: 3, y: 0, w: 1, h: 1};
    assert.deepEqual(dashboardDevicePresentation.normalizeLayout([valid]), [valid]);
    assert.deepEqual(dashboardDevicePresentation.normalizeLayout([{id: "native:out-of-range", x: 4, y: 0, w: 1, h: 1}]), []);
    assert.deepEqual(dashboardDevicePresentation.normalizeLayout([{id: "native:negative", x: -1, y: 0, w: 1, h: 1}]), []);
});

test("Dashboard visible moves preserve disconnected layout entries and their occupied slots", function () {
    const saved = [
        {id: "native:disconnected", x: 0, y: 0, w: 1, h: 1},
        {id: "native:a", x: 1, y: 0, w: 1, h: 1},
        {id: "native:b", x: 2, y: 0, w: 1, h: 1}
    ];
    const connected = [{id: "native:a"}, {id: "native:b"}];
    const moved = dashboardDevicePresentation.moveLayout(saved, connected, "native:b", 0);
    assert.deepEqual(moved, [
        {id: "native:disconnected", x: 0, y: 0, w: 1, h: 1},
        {id: "native:a", x: 2, y: 0, w: 1, h: 1},
        {id: "native:b", x: 1, y: 0, w: 1, h: 1}
    ]);
    const reconnected = dashboardDevicePresentation.reconcileLayout(moved, [{id: "native:disconnected"}, ...connected]);
    assert.deepEqual(reconnected.map((card) => card.id), ["native:disconnected", "native:b", "native:a"]);
});

test("Dashboard layout writes serialize immutable snapshots and accept only the latest revision", async function () {
    const sent = [], resolvers = [], accepted = [];
    const enqueue = dashboardDevicePresentation.createLayoutWriteQueue(function (snapshot) {
        sent.push(snapshot);
        return new Promise(function (resolve) { resolvers.push(resolve); });
    }, function (response) { accepted.push(response); });
    const first = [{id: "native:a", x: 0, y: 0, w: 1, h: 1}];
    const firstWrite = enqueue(first);
    first[0].x = 99;
    const secondWrite = enqueue([{id: "native:a", x: 1, y: 0, w: 1, h: 1}]);
    await Promise.resolve();
    assert.equal(sent.length, 1);
    assert.equal(sent[0][0].x, 0);
    resolvers.shift()({layout: "older"});
    await firstWrite;
    await Promise.resolve();
    assert.equal(sent.length, 2);
    resolvers.shift()({layout: "newer"});
    await secondWrite;
    assert.deepEqual(accepted, [{layout: "newer"}]);
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
    assert.match(source, /\/api\/dashboard\/layout/);
    assert.doesNotMatch(source, /\/api\/dashboard\/devices\/get/);
    assert.doesNotMatch(source, /dashboard\.Devices/);
    assert.doesNotMatch(source, /sortable\(/);
});

test("lower Dashboard device labels use the template-provided localization bridge", function () {
    const source = fs.readFileSync(__dirname + "/index.js", "utf8");

    assert.match(source, /const i18n = window\.dashboardI18n \|\| \{\}/);
    assert.match(source, /text: i18n\.lighting/);
    assert.match(source, /text: i18n\.brightness/);
    assert.doesNotMatch(source, /text: "Lighting"/);
    assert.doesNotMatch(source, /text: "Brightness"/);
});

test("Dashboard drag feedback keeps a source state, drop target, and reliable cleanup", function () {
    const source = fs.readFileSync(__dirname + "/index.js", "utf8");

    assert.match(source, /classList\.add\("lf-dashboard-card-dragging"\)/);
    assert.match(source, /classList\.add\("lf-dashboard-card-drag-target"\)/);
    assert.match(source, /removeClass\("lf-dashboard-card-drag-target"\)/);
    assert.match(source, /classList\.remove\("lf-dashboard-card-dragging"\)/);
    assert.match(source, /pointercancel/);
    assert.match(source, /const handle = wrapper\.querySelector\("\[data-lf-dashboard-drag-handle\]"\); if \(!handle\) return;/);
    assert.match(source, /event\.button !== 0 \|\| window\.matchMedia\("\(max-width: 560px\)"\)\.matches/);
    assert.doesNotMatch(source, /resize(?:able|handle)/i);
});

test("Dashboard drag ghost is inert, pointer-offset, and cleaned up with the drag", function () {
    const source = fs.readFileSync(__dirname + "/index.js", "utf8");

    assert.match(source, /function createDragGhost\(wrapper, sourceRect, offset, event\)/);
    assert.match(source, /ghost\.setAttribute\("aria-hidden", "true"\)/);
    assert.match(source, /element\.setAttribute\("tabindex", "-1"\)/);
    assert.match(source, /ghost\.style\.width = sourceRect\.width/);
    assert.match(source, /event\.clientX - offset\.x/);
    assert.match(source, /!dragStarted && moved > 4/);
    assert.match(source, /ghost = createDragGhost/);
    assert.match(source, /if \(ghost\) ghost\.remove\(\)/);
    assert.match(source, /onPointerCancel/);
    assert.match(source, /keyEvent\.key === "Escape"/);
});
