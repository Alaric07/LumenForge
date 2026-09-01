"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const memory = require("./devices-memory.js");

function labelRow() {
    const handlers = {};
    const display = {hidden: false, textContent: "DIMM 1", addEventListener: function (name, handler) { handlers["display:" + name] = handler; }};
    const input = {hidden: true, value: "", disabled: false, focus: function () {}, addEventListener: function (name, handler) { handlers["input:" + name] = handler; }};
    return {
        dataset: {lfChannelId: "3", lfModuleName: "DIMM 1", lfConfirmedLabel: ""},
        querySelector: function (selector) {
            if (selector === "[data-lf-memory-label]") { return input; }
            if (selector === "[data-lf-memory-label-display]") { return display; }
            return null;
        },
        handlers: handlers,
        display: display,
        input: input
    };
}

test("Memory Lighting label edits retain the existing label mutation payload", async function () {
    const row = labelRow();
    const workspace = {dataset: {lfDeviceId: "memory-device"}};
    const requests = [];
    memory.bindLabel({fetch: function (url, options) {
        requests.push({url: url, body: JSON.parse(options.body)});
        return Promise.resolve({ok: true, json: async function () { return {status: 1}; }});
    }}, workspace, row);

    row.handlers["display:click"]();
    row.input.value = "Front DIMM";
    await row.handlers["input:blur"]();

    assert.deepEqual(requests, [{url: "/api/label", body: {deviceId: "memory-device", channelId: 3, deviceType: 0, label: "Front DIMM"}}]);
    assert.equal(row.display.textContent, "Front DIMM");
    assert.equal(row.input.hidden, true);
});
