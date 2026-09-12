"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const psu = require("./devices-psu.js");

function deferred() {
    let resolve;
    const promise = new Promise(function (done) { resolve = done; });
    return {promise: promise, resolve: resolve};
}

function harness() {
    const handlers = {};
    const mode = {value: "6", disabled: false, addEventListener: function (name, handler) { handlers[name] = handler; }};
    const status = {textContent: ""};
    const workspace = {dataset: {lfDeviceId: "psu-serial"}, querySelector: function (selector) { return selector === "[data-lf-psu-fan-mode]" ? mode : status; }};
    const calls = [];
    const browser = {document: {querySelector: function () { return workspace; }}, fetch: function (url, options) { const request = deferred(); calls.push({url: url, body: JSON.parse(options.body), request: request}); return request.promise; }, LumenForgeDevicesToast: function () { browser.saved = true; }};
    psu.init(browser);
    return {browser: browser, calls: calls, handlers: handlers, mode: mode, status: status};
}

test("PSU fan mode captures and serializes the submitted value", async function () {
    const h = harness();
    h.mode.value = "8";
    const save = h.handlers.change();
    assert.equal(h.mode.disabled, true);
    assert.deepEqual(h.calls.map(function (call) { return {url: call.url, body: call.body}; }), [{url: "/api/psu/speed", body: {deviceId: "psu-serial", fanMode: 8}}]);
    h.mode.value = "10";
    h.calls[0].request.resolve({ok: true, json: async function () { return {status: 1}; }});
    await save;
    assert.equal(h.mode.disabled, false);
    h.mode.value = "7";
    const reject = h.handlers.change();
    h.calls[1].request.resolve({ok: true, json: async function () { return {status: 0}; }});
    await reject;
    assert.equal(h.mode.value, "8");
});

test("PSU fan mode prevents overlapping requests and restores the last confirmed mode", async function () {
    const h = harness();
    h.mode.value = "8";
    const first = h.handlers.change();
    assert.equal(h.mode.disabled, true);
    h.mode.value = "9";
    await h.handlers.change();
    assert.equal(h.calls.length, 1);
    assert.equal(h.mode.value, "8");
    h.calls[0].request.resolve({ok: true, json: async function () { return {status: 1}; }});
    await first;
    assert.equal(h.mode.disabled, false);
    h.mode.value = "3";
    await h.handlers.change();
    assert.equal(h.mode.value, "8");
});
