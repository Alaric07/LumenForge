"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const api = require("./devices-rgb-topology.js");

test("topology mutations restore the confirmed selection when rejected", async function () {
    let listener;
    const type = {value: "1", disabled: false, addEventListener: function (_, callback) { listener = callback; }};
	const amount = {value: "1", disabled: false, addEventListener: function () {}};
    const status = {textContent: ""};
	const row = {dataset: {lfPortId: "0"}, querySelector: function (selector) { if (selector.includes("-type]")) { return type; } if (selector.includes("-amount]")) { return amount; } return status; }};
    const workspace = {dataset: {lfDeviceId: "node"}, querySelectorAll: function () { return [row]; }};
    const browser = {document: {querySelector: function () { return workspace; }}, fetch: async function () { return {ok: true, json: async function () { return {status: 0}; }}; }};
    api.init(browser); type.value = "2"; await listener();
    assert.equal(type.value, "1"); assert.equal(status.textContent, "Couldn’t save this RGB topology setting.");
});
