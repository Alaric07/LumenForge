"use strict";

const assert = require("assert");
const screen = require("./devices-screen.js");

function control(value) {
    return {value: value, disabled: false, listeners: {}, addEventListener: function (name, listener) { this.listeners[name] = listener; }};
}

async function run() {
    const selector = control("cpu-info");
    const status = {textContent: ""};
    const workspace = {
        dataset: {lfDeviceId: "nexus-serial"},
        querySelector: function (value) { return value === "[data-lf-screen-profile]" ? selector : status; }
    };
    const requests = [];
    const browser = {
        document: {querySelector: function () { return workspace; }},
        fetch: async function (url, options) {
            requests.push({url: url, body: JSON.parse(options.body)});
            return {ok: true, json: async function () { return {status: browser.reject ? 0 : 1}; }};
        },
        LumenForgeDevicesToast: function (message) { browser.toast = message; }
    };

    screen.init(browser);
    selector.value = "aio-serial;7";
    await selector.listeners.change();
    assert.deepStrictEqual(requests, [{url: "/api/lcd/profile", body: {deviceId: "nexus-serial", profile: "aio-serial;7"}}]);
    assert.strictEqual(browser.toast, "✓ Saved");

    browser.reject = true;
    selector.value = "time-info";
    await selector.listeners.change();
    assert.strictEqual(selector.value, "aio-serial;7");
    assert.strictEqual(status.textContent, "Couldn’t save this screen selection.");

    const noControlBrowser = {document: {querySelector: function () { return {dataset: {lfDeviceId: "nexus-serial"}, querySelector: function () { return null; }}; }}, fetch: function () { throw new Error("unexpected request"); }};
    screen.init(noControlBrowser);
}

run().catch(function (error) { console.error(error); process.exitCode = 1; });
