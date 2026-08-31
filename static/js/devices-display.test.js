"use strict";

const assert = require("assert");
const display = require("./devices-display.js");

function control(value) {
    return {value: value, disabled: false, listeners: {}, addEventListener: function (name, listener) { this.listeners[name] = listener; }};
}

async function run() {
    const mode = control("10");
    const rotation = control("0");
    const brightness = control("64");
    const image = control("status");
    const imageControl = {hidden: false};
    const status = {textContent: ""};
    const workspace = {
        dataset: {lfDeviceId: "cc-lcd", lfChannelId: "0", lfImageModeId: "10"},
        querySelector: function (selector) {
            return selector === "[data-lf-display-mode]" ? mode : selector === "[data-lf-display-rotation]" ? rotation : selector === "[data-lf-display-brightness]" ? brightness : selector === "[data-lf-display-image]" ? image : selector === "[data-lf-display-image-control]" ? imageControl : status;
        }
    };
    const requests = [];
    const browser = {
        document: {querySelector: function () { return workspace; }},
        fetch: async function (url, options) {
            requests.push({url: url, body: JSON.parse(options.body)});
            return {ok: true, json: async function () { return {status: browser.reject && url === "/api/lcd/rotation" ? 0 : 1}; }};
        },
        LumenForgeDevicesToast: function (message) { browser.toast = message; }
    };

    display.init(browser);
    mode.value = "0";
    await mode.listeners.change();
    rotation.value = "2";
    browser.reject = true;
    await rotation.listeners.change();
	assert.strictEqual(rotation.value, "0");
	assert.strictEqual(status.textContent, "Couldn’t save this display setting.");
    browser.reject = false;
    brightness.value = "1";
    await brightness.listeners.change();
    image.value = "loop";
    await image.listeners.change();

    assert.deepStrictEqual(requests, [
        {url: "/api/lcd", body: {deviceId: "cc-lcd", channelId: 0, mode: 0}},
        {url: "/api/lcd/rotation", body: {deviceId: "cc-lcd", channelId: 0, rotation: 2}},
        {url: "/api/lcd/brightness", body: {deviceId: "cc-lcd", channelId: 0, brightness: 1}},
        {url: "/api/lcd/image", body: {deviceId: "cc-lcd", channelId: 0, image: "loop"}}
    ]);
    assert.strictEqual(imageControl.hidden, true);
    assert.strictEqual(browser.toast, "✓ Saved");
}

run().catch(function (error) { console.error(error); process.exitCode = 1; });
