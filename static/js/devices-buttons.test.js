"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const buttons = require("./devices-buttons.js");

function control(value, checked) {
    const handlers = {};
    return {value: value || "0", checked: Boolean(checked), disabled: false, addEventListener: function (event, handler) { handlers[event] = handler; }, fire: function (event) { return handlers[event](); }};
}

function row(keyIndex, typeValue, commandValue, lumenForgeControlEnabled) {
    const type = control(String(typeValue));
    const command = control(String(commandValue));
    const ownership = control("", lumenForgeControlEnabled);
    const hold = control("", false);
    const release = control("", false);
    const status = {textContent: ""};
    command.children = [];
    command.replaceChildren = function () { this.children = []; };
    command.appendChild = function (option) { this.children.push(option); if (option.selected || this.children.length === 1) { this.value = option.value; } };
    command.ownerDocument = {createElement: function () { return {}; }};
    const controls = {"[data-lf-button-type]": type, "[data-lf-button-command]": command, "[data-lf-button-control]": ownership, "[data-lf-button-hold]": hold, "[data-lf-button-release]": release, "[data-lf-button-status]": status};
    return {dataset: {lfKeyIndex: String(keyIndex), lfCurrentCommand: String(commandValue)}, querySelector: function (selector) { return controls[selector] || null; }, controls: controls};
}

function response(data) { return {ok: true, json: async function () { return data; }}; }
function browser(fetch) {
    const timers = [];
    return {fetch: fetch, setTimeout: function (callback, delay) { const timer = {callback: callback, delay: delay, cancelled: false}; timers.push(timer); return timer; }, clearTimeout: function (timer) { timer.cancelled = true; }, timers: timers};
}
function toast() { return {dataset: {}, hidden: true, textContent: ""}; }

test("Buttons option helpers retain local None values and existing remote endpoints", async function () {
    [0, 2, 8, 11].forEach(function (type) { assert.deepEqual(buttons.localOptions(type), [{value: 0, label: "None"}]); });
    for (const [type, endpoint] of new Map([[1, "/api/input/media"], [3, "/api/input/keyboard"], [9, "/api/input/mouse"], [10, "/api/macro/"]])) {
        let requested;
        const result = await buttons.optionsFor({fetch: async function (url) { requested = url; return response({data: {42: {Name: "Action"}}}); }}, type);
        assert.equal(requested, endpoint);
        assert.deepEqual(result, [{value: 42, label: "Action"}]);
    }
});

test("Buttons shares remote option loads and selects each row's current command", async function () {
    const first = row(2, 3, 42);
    const second = row(4, 3, 9);
    let gets = 0;
    const target = browser(async function (url) { gets++; assert.equal(url, "/api/input/keyboard"); return response({data: {9: {Name: "Nine"}, 42: {Name: "Forty two"}}}); });
    const workspace = {dataset: {lfDeviceId: "elite-test"}};
    const cache = buttons.createOptionCache();
    await Promise.all([buttons.bindRow(target, workspace, first, cache), buttons.bindRow(target, workspace, second, cache)]);
    assert.equal(gets, 1);
    assert.equal(first.controls["[data-lf-button-command]"].value, "42");
    assert.equal(second.controls["[data-lf-button-command]"].value, "9");
});

test("Buttons control, hold, release, and key changes auto-save with legacy ownership mapping", async function () {
    const targetRow = row(2, 0, 0, false);
    const requests = [];
    const target = browser(async function (_url, options) { requests.push(JSON.parse(options.body)); return response({status: 1}); });
    await buttons.bindRow(target, {dataset: {lfDeviceId: "elite-test"}}, targetRow);
    await targetRow.controls["[data-lf-button-control]"].fire("change");
    assert.equal(requests[0].enabled, true);
    targetRow.controls["[data-lf-button-control]"].checked = true;
    await targetRow.controls["[data-lf-button-control]"].fire("change");
    assert.equal(requests[1].enabled, false);
    targetRow.controls["[data-lf-button-hold]"].checked = true;
    await targetRow.controls["[data-lf-button-hold]"].fire("change");
    assert.equal(requests[2].pressAndHold, true);
    targetRow.controls["[data-lf-button-release]"].checked = true;
    await targetRow.controls["[data-lf-button-release]"].fire("change");
    assert.equal(targetRow.controls["[data-lf-button-hold]"].checked, false);
    assert.equal(requests[3].onRelease, true);
    targetRow.controls["[data-lf-button-command]"].value = "12";
    await targetRow.controls["[data-lf-button-command]"].fire("change");
    assert.equal(requests[4].keyAssignmentValue, 12);
});

test("Buttons type changes populate before auto-save", async function () {
    const targetRow = row(2, 0, 0, true);
    let resolveOptions;
    const requests = [];
    const target = browser(function (url, options) {
        if (!options) { assert.equal(url, "/api/input/keyboard"); return new Promise(function (resolve) { resolveOptions = resolve; }); }
        requests.push(JSON.parse(options.body)); return Promise.resolve(response({status: 1}));
    });
    await buttons.bindRow(target, {dataset: {lfDeviceId: "elite-test"}}, targetRow);
    targetRow.controls["[data-lf-button-type]"].value = "3";
    const changing = targetRow.controls["[data-lf-button-type]"].fire("change");
    assert.equal(requests.length, 0);
    resolveOptions(response({data: {7: {Name: "Seven"}}}));
    await changing;
    assert.deepEqual(requests[0], {deviceId: "elite-test", keyIndex: 2, enabled: false, pressAndHold: false, keyAssignmentType: 3, keyAssignmentValue: 7, onRelease: false});
});

test("Buttons Cycle Cluster Effect clears hold and release before auto-save", async function () {
    const targetRow = row(2, 3, 7, true);
    const requests = [];
    const target = browser(async function (_url, options) { requests.push(JSON.parse(options.body)); return response({status: 1}); });
    await buttons.bindRow(target, {dataset: {lfDeviceId: "elite-test"}}, targetRow);
    targetRow.controls["[data-lf-button-hold]"].checked = true;
    targetRow.controls["[data-lf-button-release]"].checked = true;
    targetRow.controls["[data-lf-button-type]"].value = "30";
    await targetRow.controls["[data-lf-button-type]"].fire("change");
    assert.deepEqual(requests[0], {deviceId: "elite-test", keyIndex: 2, enabled: false, pressAndHold: false, keyAssignmentType: 30, keyAssignmentValue: 0, onRelease: false});
});

test("Buttons saves show success and error toast feedback at the intended durations", async function () {
    const success = row(2, 0, 0);
    const successBrowser = browser(async function () { return response({status: 1}); });
    const successToast = toast();
    await buttons.bindRow(successBrowser, {dataset: {lfDeviceId: "elite-test"}}, success, null, buttons.createToast(successBrowser, successToast));
    await success.controls["[data-lf-button-command]"].fire("change");
    assert.equal(successToast.textContent, "✓ Saved");
    assert.equal(successToast.dataset.lfButtonsToastKind, "success");
    assert.equal(successBrowser.timers[0].delay, 1500);
    successBrowser.timers[0].callback();
    assert.equal(successToast.hidden, true);
    const failed = row(4, 0, 0);
    const failedBrowser = browser(async function () { return response({status: 0}); });
    const failedToast = toast();
    await buttons.bindRow(failedBrowser, {dataset: {lfDeviceId: "elite-test"}}, failed, null, buttons.createToast(failedBrowser, failedToast));
    await failed.controls["[data-lf-button-command]"].fire("change");
    assert.equal(failedToast.textContent, "Couldn’t save button assignment.");
    assert.equal(failedToast.dataset.lfButtonsToastKind, "error");
    assert.equal(failedBrowser.timers[0].delay, 4500);
});

test("Buttons toast replaces prior feedback without stale timers or stacked elements", function () {
    const target = browser(function () {});
    const element = toast();
    const showToast = buttons.createToast(target, element);
    showToast("✓ Saved", "success", 1500);
    const stale = target.timers[0];
    showToast("Couldn’t save button assignment.", "error", 4500);
    assert.equal(target.timers.length, 2);
    assert.equal(stale.cancelled, true);
    stale.callback();
    assert.equal(element.hidden, false);
    assert.equal(element.textContent, "Couldn’t save button assignment.");
    target.timers[1].callback();
    assert.equal(element.hidden, true);
});

test("Buttons keeps assignment-option loading failures row-local", async function () {
    const targetRow = row(2, 3, 9);
    await buttons.bindRow(browser(async function () { return {ok: false}; }), {dataset: {lfDeviceId: "elite-test"}}, targetRow);
    assert.equal(targetRow.controls["[data-lf-button-status]"].textContent, "Unable to load assignment keys.");
});

test("Buttons does not save a row while assignment-key options are unavailable", async function () {
    const targetRow = row(2, 3, 9, false);
    const posts = [];
    const target = browser(async function (_url, options) {
        if (!options) { return {ok: false}; }
        posts.push(JSON.parse(options.body));
        return response({status: 1});
    });
    await buttons.bindRow(target, {dataset: {lfDeviceId: "elite-test"}}, targetRow);
    assert.equal(targetRow.controls["[data-lf-button-status]"].textContent, "Unable to load assignment keys.");
    targetRow.controls["[data-lf-button-control]"].checked = true;
    await targetRow.controls["[data-lf-button-control]"].fire("change");
    assert.equal(posts.length, 0);
    assert.equal(targetRow.dataset.lfCurrentCommand, "9");
});

test("Buttons drops a queued save when a new assignment-key load fails", async function () {
    const targetRow = row(2, 0, 0, true);
    const pending = [];
    const posts = [];
    const target = browser(function (_url, options) {
        if (!options) { return Promise.resolve({ok: false}); }
        posts.push(JSON.parse(options.body));
        return new Promise(function (resolve) { pending.push(resolve); });
    });
    await buttons.bindRow(target, {dataset: {lfDeviceId: "elite-test"}}, targetRow);
    const firstSave = targetRow.controls["[data-lf-button-command]"].fire("change");
    targetRow.controls["[data-lf-button-command]"].value = "9";
    targetRow.controls["[data-lf-button-command]"].fire("change");
    targetRow.controls["[data-lf-button-type]"].value = "3";
    await targetRow.controls["[data-lf-button-type]"].fire("change");
    assert.equal(targetRow.controls["[data-lf-button-status]"].textContent, "Unable to load assignment keys.");
    pending[0](response({status: 1}));
    await firstSave;
    assert.equal(posts.length, 1);
    assert.equal(posts[0].keyAssignmentValue, 0);
});

test("Buttons queue newer changes without mutating another row and need no Save button", async function () {
    const first = row(2, 0, 0, true);
    const second = row(4, 0, 0, false);
    const pending = [];
    const requests = [];
    let resolveThirdRequest;
    const thirdRequest = new Promise(function (resolve) { resolveThirdRequest = resolve; });
    const target = browser(function (_url, options) {
        requests.push(JSON.parse(options.body));
        if (requests.length === 3) { resolveThirdRequest(); }
        return new Promise(function (resolve) { pending.push(resolve); });
    });
    const workspace = {dataset: {lfDeviceId: "elite-test"}};
    await Promise.all([buttons.bindRow(target, workspace, first), buttons.bindRow(target, workspace, second)]);
    const firstSave = first.controls["[data-lf-button-command]"].fire("change");
    first.controls["[data-lf-button-command]"].value = "9";
    first.controls["[data-lf-button-command]"].fire("change");
    second.controls["[data-lf-button-control]"].checked = true;
    const secondSave = second.controls["[data-lf-button-control]"].fire("change");
    assert.equal(requests.length, 2);
    pending[0](response({status: 1}));
    pending[1](response({status: 1}));
    await thirdRequest;
    assert.equal(requests.length, 3);
    assert.equal(requests[2].keyAssignmentValue, 9);
    pending[2](response({status: 1}));
    await Promise.all([firstSave, secondSave]);
    assert.equal(second.controls["[data-lf-button-control]"].checked, true);
    assert.equal(first.querySelector("[data-lf-button-save]"), null);
});
