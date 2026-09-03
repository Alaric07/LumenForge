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
    const saved = [{id: "native:device-1", column: 2, order: 0}, {id: "openrgb:device-1", column: 0, order: 0}];
    const connected = dashboardDevicePresentation.reconcileLayout(saved, [
        {id: dashboardDevicePresentation.cardID("native", "device-1")},
        {id: dashboardDevicePresentation.cardID("openrgb", "device-1")}
    ]);
    assert.equal(connected[0].id, "openrgb:device-1");
    assert.equal(connected[1].id, "native:device-1");
    const reconnected = dashboardDevicePresentation.reconcileLayout(saved, [{id: "native:device-1"}]);
    assert.deepEqual(reconnected[0], {id: "native:device-1", column: 2, order: 0});
});

test("Dashboard lane layout normalizes orders and maps Phase 3A positions", function () {
    const valid = {id: "native:last-column", column: 8, order: 0};
    assert.deepEqual(dashboardDevicePresentation.normalizeLayout([valid]), [valid]);
    assert.deepEqual(dashboardDevicePresentation.normalizeLayout([{id: "native:legacy", x: 2, y: 3, w: 4, h: 5}]), [{id: "native:legacy", column: 2, order: 3}]);
    assert.deepEqual(dashboardDevicePresentation.normalizeLayout([{id: "native:negative", column: -1, order: 0}]), []);
    assert.deepEqual(dashboardDevicePresentation.normalizeLayout([{id: "native:a", column: 1, order: 0}, {id: "native:b", column: 1, order: 0}]), [{id: "native:a", column: 1, order: 0}, {id: "native:b", column: 1, order: 1}]);
});

test("Dashboard renders sparse persisted columns plus one trailing empty lane", function () {
    assert.deepEqual(dashboardDevicePresentation.laneColumns([{id: "native:mouse", column: 1, order: 0}], [{id: "native:mouse"}]), [0, 1, 2]);
    assert.deepEqual(dashboardDevicePresentation.laneColumns([{id: "native:a", column: 0, order: 0}, {id: "native:b", column: 2, order: 0}], [{id: "native:a"}, {id: "native:b"}]), [0, 1, 2, 3]);
    assert.deepEqual(dashboardDevicePresentation.laneColumns([{id: "native:disconnected", column: 3, order: 0}], []), [0, 1, 2, 3, 4]);
});

test("Dashboard dense lane classification uses native status-row content", function () {
    assert.equal(dashboardDevicePresentation.isDenseCard({source: "native", statusRows: [{}, {}, {}, {}]}), true);
    assert.equal(dashboardDevicePresentation.isDenseCard({source: "native", statusRows: [{}, {}]}), false);
    assert.equal(dashboardDevicePresentation.isDenseCard({source: "openrgb", statusRows: [{}, {}, {}, {}]}), false);
});

test("Dashboard moves only the affected lane and retains disconnected positions", function () {
    const saved = [
        {id: "native:disconnected", column: 0, order: 0},
        {id: "native:a", column: 1, order: 0},
        {id: "native:b", column: 1, order: 1},
        {id: "native:c", column: 2, order: 0}
    ];
    const connected = [{id: "native:a"}, {id: "native:b"}, {id: "native:c"}];
    const moved = dashboardDevicePresentation.moveLayout(saved, connected, "native:b", {column: 1, order: 0});
    assert.deepEqual(moved.find((item) => item.id === "native:b"), {id: "native:b", column: 1, order: 0});
    assert.deepEqual(moved.find((item) => item.id === "native:a"), {id: "native:a", column: 1, order: 1});
    assert.deepEqual(moved.find((item) => item.id === "native:c"), {id: "native:c", column: 2, order: 0});
    assert.ok(moved.some((item) => item.id === "native:disconnected"));
    assert.deepEqual(dashboardDevicePresentation.laneColumns(moved, connected), [0, 1, 2, 3]);
});

test("Dashboard layout writes serialize immutable snapshots and accept only the latest revision", async function () {
    const sent = [], resolvers = [], accepted = [];
    const enqueue = dashboardDevicePresentation.createLayoutWriteQueue(function (snapshot) {
        sent.push(snapshot);
        return new Promise(function (resolve) { resolvers.push(resolve); });
    }, function (response) { accepted.push(response); });
    const first = [{id: "native:a", column: 0, order: 0}];
    const firstWrite = enqueue(first);
    first[0].column = 99;
    const secondWrite = enqueue([{id: "native:a", column: 1, order: 0}]);
    await Promise.resolve();
    assert.equal(sent.length, 1);
    assert.equal(sent[0][0].column, 0);
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

test("Dashboard native and OpenRGB cards share the fill-lane card class", function () {
    const source = fs.readFileSync(__dirname + "/index.js", "utf8");

    assert.match(source, /class: "lf-dashboard-device-card"/);
    assert.match(source, /link\.addClass\("lf-dashboard-openrgb-row"\)/);
});

test("Dashboard cooling cards use native SVG sparklines from the shared history renderer", function () {
    const source = fs.readFileSync(__dirname + "/index.js", "utf8");

    assert.match(source, /history\.createSparkline\(window\.document, key\)/);
    assert.match(source, /history\.keys\.fanAverage\(card\.serial\)/);
    assert.match(source, /history\.keys\.coolant\(card\.serial\)/);
    assert.match(source, /history\.keys\.probe\(card\.serial, probe\.id\)/);
    assert.doesNotMatch(source, /\$\("<svg>"/);
});

test("Dashboard drag feedback keeps a source state, drop target, and reliable cleanup", function () {
    const source = fs.readFileSync(__dirname + "/index.js", "utf8");

    assert.match(source, /classList\.add\("lf-dashboard-card-dragging"\)/);
    assert.match(source, /classList\.add\("lf-dashboard-lane-drag-target"\)/);
    assert.match(source, /removeClass\("lf-dashboard-lane-drag-target"\)/);
    assert.match(source, /classList\.remove\("lf-dashboard-card-dragging"\)/);
    assert.match(source, /pointercancel/);
    assert.match(source, /const handle = wrapper\.querySelector\("\[data-lf-dashboard-drag-handle\]"\); if \(!handle\) return;/);
    assert.match(source, /event\.button !== 0 \|\| window\.matchMedia\("\(max-width: 560px\)"\)\.matches/);
    assert.match(source, /data-lf-dashboard-lane/);
    assert.match(source, /laneColumns\(layoutState, currentCards\)\.forEach/);
    assert.match(source, /lane\.style\.minHeight = laneHeight/);
    assert.match(source, /lf-dashboard-lane-insertion/);
    assert.match(source, /lf-dashboard-device-lane-dense/);
    assert.match(source, /function cleanupDrag\(\)/);
    assert.match(source, /lostpointercapture/);
    assert.match(source, /document\.body\.classList\.remove\("lf-dashboard-drag-active"\)/);
    assert.match(source, /try \{ if \(state\.handle\.hasPointerCapture/);
    assert.match(source, /if \(dragState\) \{ deferredDeviceResponse = response; return; \}/);
    assert.match(source, /finally \{ flushDeferredPresentation\(\); \}/);
    assert.doesNotMatch(source, /resize(?:able|handle|preset)/i);
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
    assert.match(source, /if \(state\.ghost\) state\.ghost\.remove\(\)/);
    assert.match(source, /onPointerCancel/);
    assert.match(source, /keyEvent\.key === "Escape"/);
});
