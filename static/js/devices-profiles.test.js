"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const profiles = require("./devices-profiles.js");

function element(value) {
    const handlers = {};
    return {value: value || "", hidden: false, textContent: "", children: {}, addEventListener: function (name, handler) { handlers[name] = handler; }, fire: function (name) { return handlers[name](); }, querySelector: function (selector) { return this.children[selector] || null; }};
}

function harness() {
    const profile = element("default"); const deleteSelect = element("gaming"); const remove = element(); const status = element(); const saveAs = element(); const dialog = element(); const name = element(); const create = element(); const cancel = element(); const dialogStatus = element(); dialog.children = {"[data-lf-device-profile-name]": name, "[data-lf-device-profile-create]": create, "[data-lf-device-profile-cancel]": cancel, "[data-lf-device-profile-dialog-status]": dialogStatus};
    const nodes = {"[data-lf-device-profile]": profile, "[data-lf-device-profile-delete-select]": deleteSelect, "[data-lf-device-profile-delete]": remove, "[data-lf-device-profile-status]": status, "[data-lf-device-profile-new]": saveAs, "[data-lf-device-profile-dialog]": dialog};
    const workspace = {dataset: {lfDeviceId: "k95"}, querySelector: function (selector) { return nodes[selector] || null; }};
    const posts = []; const browser = {document: {readyState: "complete", querySelector: function () { return workspace; }}, fetch: async function (url, options) { posts.push({url: url, options: options}); return browser.response || {ok: true, json: async function () { return {status: 1}; }}; }, location: {reload: function () { browser.reloaded = true; }}, LumenForgeDevicesToast: function () { browser.toasted = true; }};
    profiles.init(browser); return {browser: browser, nodes: nodes, posts: posts};
}

test("Device Profile selector posts the legacy user-profile change payload", async function () {
    const h = harness(); const profile = h.nodes["[data-lf-device-profile]"]; profile.value = "studio"; await profile.fire("change"); assert.equal(h.posts[0].url, "/api/userProfile/change"); assert.deepEqual(JSON.parse(h.posts[0].options.body), {deviceId: "k95", userProfileName: "studio"}); assert.equal(h.browser.reloaded, true);
});

test("Device Profile selector leaves the active value alone and restores it after failure", async function () {
    const h = harness(); const profile = h.nodes["[data-lf-device-profile]"]; await profile.fire("change"); assert.equal(h.posts.length, 0); h.browser.response = {ok: true, json: async function () { return {status: 0}; }}; profile.value = "studio"; await profile.fire("change"); assert.equal(profile.value, "default"); assert.equal(h.nodes["[data-lf-device-profile-status]"].textContent, "Couldn’t change Device Profile.");
});

test("Device Profile Save As and deletion use existing user-profile routes", async function () {
    const h = harness(); const dialog = h.nodes["[data-lf-device-profile-dialog]"]; h.nodes["[data-lf-device-profile-new]"].fire("click"); await dialog.children["[data-lf-device-profile-create]"].fire("click"); assert.equal(dialog.children["[data-lf-device-profile-dialog-status]"].textContent, "Enter a Device Profile name."); dialog.children["[data-lf-device-profile-name]"].value = "studio"; await dialog.children["[data-lf-device-profile-create]"].fire("click"); assert.equal(h.posts[0].url, "/api/userProfile"); assert.deepEqual(JSON.parse(h.posts[0].options.body), {deviceId: "k95", userProfileName: "studio"}); const deleted = harness(); await deleted.nodes["[data-lf-device-profile-delete]"].fire("click"); assert.equal(deleted.posts[0].url, "/api/userProfile/delete"); assert.deepEqual(JSON.parse(deleted.posts[0].options.body), {deviceId: "k95", userProfileName: "gaming"});
});
