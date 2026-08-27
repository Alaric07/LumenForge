"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const dpi = require("./devices-dpi.js");

test("DPI editor accepts exact in-range integers only", function () {
    assert.equal(dpi.parseDPI("100", 100, 18000), 100);
    assert.equal(dpi.parseDPI("18000", 100, 18000), 18000);
    assert.equal(dpi.parseDPI("099", 100, 18000), null);
    assert.equal(dpi.parseDPI("100.5", 100, 18000), null);
    assert.equal(dpi.parseDPI("18001", 100, 18000), null);
});

test("DPI editor normalizes hex colors", function () {
    assert.equal(dpi.normalizeColor("#AAbbCC"), "#aabbcc");
    assert.equal(dpi.normalizeColor("blue"), null);
});

function classList(initial) {
    const values = new Set(initial || []);
    let changes = 0;
    return {
        add: function (value) { if (!values.has(value)) { values.add(value); changes += 1; } },
        contains: function (value) { return values.has(value); },
        toggle: function (value, force) { if (force) { this.add(value); } else if (values.delete(value)) { changes += 1; } },
        changes: function () { return changes; }
    };
}

function stageRow(id, sniper, active) {
    const initialClasses = sniper ? ["lf-dpi-stage-sniper"] : [];
    if (active) { initialClasses.push("lf-dpi-stage-active"); }
    const classes = classList(initialClasses);
    const header = {
        badge: null,
        querySelector: function (selector) { return selector === ".lf-dpi-stage-state" ? this.badge : null; },
        appendChild: function (badge) { this.badge = badge; badge.remove = () => { this.badge = null; }; }
    };
    if (active) { header.badge = {remove: () => { header.badge = null; }}; }
    return {
        classList: classes,
        dataset: {lfStageId: id},
        draft: {dpi: id === "0" ? "800" : "1600", color: id === "0" ? "#010203" : "#102030"},
        ownerDocument: {createElement: function () { return {}; }},
        querySelector: function (selector) { return selector === ".lf-dpi-stage-header" ? header : null; },
        changes: classes.changes,
        header: header
    };
}

function workspace(rows) {
    return {querySelectorAll: function (selector) { return selector === "[data-lf-dpi-row]" ? rows : []; }};
}

test("DPI active-stage helper follows physical runtime changes without changing drafts", function () {
    const first = stageRow("0", false, true);
    const second = stageRow("1", false, false);
    const sniper = stageRow("5", true, false);
    const view = workspace([first, second, sniper]);
    assert.equal(dpi.applyActiveStageState(view, {activeRegularStageId: "1", sniperActive: false}), true);
    assert.equal(first.classList.contains("lf-dpi-stage-active"), false);
    assert.equal(second.classList.contains("lf-dpi-stage-active"), true);
    assert.equal(sniper.classList.contains("lf-dpi-stage-active"), false);
    assert.deepEqual(first.draft, {dpi: "800", color: "#010203"});
    assert.deepEqual(second.draft, {dpi: "1600", color: "#102030"});
    assert.equal(dpi.applyActiveStageState(view, {activeRegularStageId: "1", sniperActive: true}), true);
    assert.equal(second.classList.contains("lf-dpi-stage-active"), true);
    assert.equal(sniper.classList.contains("lf-dpi-stage-active"), true);
    assert.equal(dpi.applyActiveStageState(view, {activeRegularStageId: "1", sniperActive: false}), true);
    assert.equal(sniper.classList.contains("lf-dpi-stage-active"), false);
    const before = first.changes() + second.changes() + sniper.changes();
    assert.equal(dpi.applyActiveStageState(view, {activeRegularStageId: "1", sniperActive: false}), false);
    assert.equal(first.changes() + second.changes() + sniper.changes(), before);
});

test("DPI workspace selection is not reverted by an already in-flight status poll", async function () {
    const first = stageRow("0", false, true);
    const second = stageRow("1", false, false);
    const sniper = stageRow("5", true, false);
    const view = workspace([first, second, sniper]);
    view.dataset = {lfDeviceSerial: "dpi-test"};
    let resolveStatus;
    const browser = {
        fetch: function (url, options) {
            if (options && options.method === "POST") {
                assert.equal(url, "/api/devices/dpi/active");
                return Promise.resolve({ok: true, json: async function () { return {status: 1}; }});
            }
            return new Promise(function (resolve) { resolveStatus = resolve; });
        },
        setInterval: function () { return 1; },
        clearInterval: function () {}
    };
    const poller = dpi.createStatusPoller(browser, view);
    const stalePoll = poller.poll();
    assert.equal(await dpi.selectActiveStage(browser, view, poller, "1"), true);
    assert.equal(first.classList.contains("lf-dpi-stage-active"), false);
    assert.equal(second.classList.contains("lf-dpi-stage-active"), true);
    resolveStatus({ok: true, json: async function () { return {status: 1, activeRegularStageId: "0", sniperActive: false}; }});
    assert.equal(await stalePoll, false);
    assert.equal(second.classList.contains("lf-dpi-stage-active"), true);
    assert.equal(sniper.classList.contains("lf-dpi-stage-active"), false);
    assert.deepEqual(first.draft, {dpi: "800", color: "#010203"});
    assert.deepEqual(second.draft, {dpi: "1600", color: "#102030"});
    poller.stop();
});

test("DPI workspace Sniper selection preserves the regular stage and drafts", async function () {
    const first = stageRow("0", false, false);
    const second = stageRow("1", false, true);
    const sniper = stageRow("5", true, false);
    const view = workspace([first, second, sniper]);
    view.dataset = {lfDeviceSerial: "dpi-test"};
    const requests = [];
    const browser = {
        fetch: function (url, options) {
            requests.push({url: url, body: JSON.parse(options.body)});
            return Promise.resolve({ok: true, json: async function () { return {status: 1}; }});
        }
    };
    const poller = {invalidations: 0, invalidate: function () { this.invalidations += 1; }};
    assert.equal(await dpi.setSniperActive(browser, view, poller, true), true);
    assert.deepEqual(requests[0], {url: "/api/devices/dpi/sniper", body: {serial: "dpi-test", active: true}});
    assert.equal(second.classList.contains("lf-dpi-stage-active"), true);
    assert.equal(sniper.classList.contains("lf-dpi-stage-active"), true);
    assert.equal(poller.invalidations, 1);
    assert.equal(await dpi.setSniperActive(browser, view, poller, false), true);
    assert.equal(second.classList.contains("lf-dpi-stage-active"), true);
    assert.equal(sniper.classList.contains("lf-dpi-stage-active"), false);
    assert.deepEqual(first.draft, {dpi: "800", color: "#010203"});
    assert.deepEqual(second.draft, {dpi: "1600", color: "#102030"});
});

test("DPI status poller prevents overlapping requests", async function () {
    const rows = [stageRow("0", false, true), stageRow("5", true, false)];
    const view = workspace(rows);
    view.dataset = {lfDeviceSerial: "dpi-test"};
    let requests = 0;
    let resolveRequest;
    const browser = {
        fetch: function () { requests += 1; return new Promise(function (resolve) { resolveRequest = resolve; }); },
        setInterval: function () { return 1; },
        clearInterval: function () {}
    };
    const poller = dpi.createStatusPoller(browser, view);
    const pending = poller.poll();
    assert.equal(await poller.poll(), false);
    assert.equal(requests, 1);
    resolveRequest({ok: true, json: async function () { return {status: 1, activeRegularStageId: "0", sniperActive: false}; }});
    assert.equal(await pending, false);
    poller.stop();
});

function performanceControl(kind, confirmed, value, checked) {
    const input = {value: String(value), checked: Boolean(checked), disabled: false};
    const status = {textContent: ""};
    return {
        dataset: {lfPerformanceKind: kind, lfConfirmedValue: String(confirmed)},
        querySelector: function (selector) {
            if (selector === "[data-lf-performance-input]") { return input; }
            if (selector === "[data-lf-performance-status]") { return status; }
            return null;
        },
        input: input,
        status: status
    };
}

test("shared Performance controls send generic device payloads", async function () {
    const workspace = {dataset: {lfDeviceId: "elite-performance"}};
    const requests = [];
    const browser = {fetch: function (url, options) {
        requests.push({url: url, body: JSON.parse(options.body)});
        return Promise.resolve({ok: true, json: async function () { return {status: 1}; }});
    }};
    assert.equal(await dpi.savePerformanceControl(browser, workspace, performanceControl("pollingRate", 1, 2)), true);
    assert.equal(await dpi.savePerformanceControl(browser, workspace, performanceControl("angleSnapping", 0, 1, true)), true);
    assert.equal(await dpi.savePerformanceControl(browser, workspace, performanceControl("liftHeight", 2, 3)), true);
    assert.deepEqual(requests, [
        {url: "/api/devices/performance/polling-rate", body: {deviceId: "elite-performance", pollingRate: 2}},
        {url: "/api/devices/performance/angle-snapping", body: {deviceId: "elite-performance", angleSnapping: 1}},
        {url: "/api/devices/performance/lift-height", body: {deviceId: "elite-performance", liftHeight: 3}}
    ]);
});

test("failed Performance mutations restore the confirmed control value", async function () {
    const workspace = {dataset: {lfDeviceId: "elite-performance"}};
    const browser = {fetch: function () { return Promise.resolve({ok: true, json: async function () { return {status: 0}; }}); }};
    const select = performanceControl("pollingRate", 1, 2);
    assert.equal(await dpi.savePerformanceControl(browser, workspace, select), false);
    assert.equal(select.input.value, "1");
    assert.equal(select.input.disabled, false);
    assert.equal(select.status.textContent, "Unable to save setting. Try again.");
    const toggle = performanceControl("angleSnapping", 0, 1, true);
    assert.equal(await dpi.savePerformanceControl(browser, workspace, toggle), false);
    assert.equal(toggle.input.checked, false);
    assert.equal(toggle.input.disabled, false);
});
