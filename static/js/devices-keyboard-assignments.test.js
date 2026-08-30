"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const assignments = require("./devices-keyboard-assignments.js");

test("Keyboard assignment option sources preserve K95 value contracts", async function () {
    let requested = "";
    const options = await assignments.optionsFor({fetch: async function (url) { requested = url; return {ok: true, json: async function () { return {data: {9: {Name: "Action"}}}; }};}}, 3, 9);
    assert.equal(requested, "/api/input/keyboard");
    assert.deepEqual(options, [{value: "9", label: "Action"}]);
    assert.deepEqual(await assignments.optionsFor({}, 0, 0), [{value: "0", label: "None"}]);
    assert.deepEqual(await assignments.optionsFor({}, 19, 2), [{value: "2", label: "Profile 2"}]);
});

test("Keyboard assignment state retains key identity and LumenForge ownership semantics", function () {
    const key = {textContent: "G1", dataset: {lfKeyIndex: "27", lfDefault: "1", lfActionType: "8", lfActionCommand: "0", lfDeviceId: "mouse-id", lfActionHold: "1", lfToggleDelay: "45"}};
    assert.deepEqual(assignments.stateFor(key), {keyIndex: 27, name: "G1", default: true, actionType: 8, actionCommand: 0, deviceID: "mouse-id", actionHold: true, toggleDelay: 45});
});

function deferred() {
    let resolve;
    return {promise: new Promise(function (done) { resolve = done; }), resolve: resolve};
}

function commandControl(value) {
    const control = element(value);
    control.children = [];
    control.ownerDocument = {createElement: function () { return {}; }};
    control.replaceChildren = function () { this.children = []; this.value = ""; };
    control.appendChild = function (option) { this.children.push(option); if (option.selected || this.children.length === 1) { this.value = option.value; } };
    return control;
}

function editor(command) {
    return {command: command, type: {disabled: false}, hold: {disabled: false}, delay: {disabled: false}, status: {textContent: ""}};
}

function keyboardResponse(id, label) {
    return {ok: true, json: async function () { return {data: {[id]: {Name: label}}}; }};
}

test("Keyboard assignment option loads ignore stale selections and cannot re-enable the newer editor", async function () {
    const first = deferred(); const second = deferred(); const requests = [first, second];
    const c = editor(commandControl()); const keyA = {}; const keyB = {}; let revision = 1; let selected = keyA;
    const current = function (candidateRevision, candidateKey) { return candidateRevision === revision && candidateKey === selected; };
    const browser = {fetch: function () { return requests.shift().promise; }};
    const loadingA = assignments.populate(browser, c, {actionType: 3, actionCommand: 11}, 1, keyA, current);
    revision = 2; selected = keyB;
    const loadingB = assignments.populate(browser, c, {actionType: 3, actionCommand: 22}, 2, keyB, current);
    second.resolve(keyboardResponse(22, "B command"));
    assert.equal(await loadingB, true);
    assignments.updateDisabled(c, false, true);
    assert.equal(c.command.value, "22");
    assert.equal(c.command.disabled, false);
    first.resolve(keyboardResponse(11, "A command"));
    assert.equal(await loadingA, false);
    assert.equal(c.command.value, "22");
    assert.equal(c.command.disabled, false);
});

test("Keyboard assignment option-load failure preserves confirmed command and blocks saves", async function () {
    const c = editor(commandControl("42"));
    c.command.children = [{value: "42", selected: true}];
    const confirmed = {actionType: 3, actionCommand: 42};
    const loaded = await assignments.populate({fetch: async function () { return {ok: false}; }}, c, confirmed, 1, {}, function () { return true; });
    assert.equal(loaded, false);
    assignments.updateDisabled(c, false, loaded);
    assert.equal(c.command.disabled, true);
    assert.equal(c.command.value, "42");
    assert.equal(confirmed.actionCommand, 42);
    assert.equal(assignments.canSaveAssignment(loaded, c, confirmed), false);
    let posted = false;
    if (assignments.canSaveAssignment(loaded, c, confirmed)) { posted = true; }
    assert.equal(posted, false);
});

test("Keyboard assignment option-load success enables the selected command", async function () {
    const c = editor(commandControl()); const key = {};
    const loaded = await assignments.populate({fetch: async function () { return keyboardResponse(27, "Selected command"); }}, c, {actionType: 3, actionCommand: 27}, 1, key, function (revision, selected) { return revision === 1 && selected === key; });
    assignments.updateDisabled(c, false, loaded);
    assert.equal(loaded, true);
    assert.equal(c.command.value, "27");
    assert.equal(c.command.disabled, false);
    assert.equal(assignments.canSaveAssignment(loaded, c, {actionType: 3}), true);
});

test("Keyboard assignment disabled state gates every editable field", function () {
    const c = editor(commandControl("4")); assignments.updateDisabled(c, false, false);
    for (const control of [c.type, c.command, c.hold, c.delay]) { assert.equal(control.disabled, true); }
    assignments.updateDisabled(c, true, true); for (const control of [c.type, c.command, c.hold, c.delay]) { assert.equal(control.disabled, true); }
    assignments.updateDisabled(c, false, true); for (const control of [c.type, c.command, c.hold, c.delay]) { assert.equal(control.disabled, false); }
});

test("Keyboard color payload preserves all four existing scopes", function () {
    const color = assignments.rgbFromHex("#123456"); const key = {keyIndex: 7}; const selected = [{keyIndex: 7}, {keyIndex: 9}];
    assert.deepEqual(assignments.colorPayload("k95", 0, color, key, selected), {deviceId: "k95", keyId: 7, keyOption: 0, color: color, keys: undefined});
    assert.equal(assignments.colorPayload("k95", 1, color, key, selected).keyOption, 1);
    assert.deepEqual(assignments.colorPayload("k95", 2, color, null, []), {deviceId: "k95", keyId: 1, keyOption: 2, color: color, keys: undefined});
    assert.deepEqual(assignments.colorPayload("k95", 3, color, key, selected).keys, [7, 9]);
    assert.equal(assignments.colorPayload("k95", 0, color, null, []), null);
    assert.equal(assignments.colorPayload("k95", 3, color, key, []), null);
});

function element(value) {
    const handlers = {}; const attributes = {};
    return {value: value || "", checked: false, disabled: false, hidden: false, textContent: "", dataset: {}, style: {}, options: [{}], assignmentTarget: false, addEventListener: function (name, handler) { handlers[name] = handler; }, fire: function (name, event) { return handlers[name](event || {}); }, setAttribute: function (name, value) { attributes[name] = value; }, getAttribute: function (name) { return attributes[name] || null; }, hasAttribute: function (name) { return name === "data-lf-keyboard-key" && this.assignmentTarget; }, querySelector: function (selector) { return this.children && this.children[selector]; }};
}

function toolbarHarness(cluster, activeProfile, liveEnabled) {
    const confirmedProfile = activeProfile || "default"; const profile = element(confirmedProfile); const save = element(); const saveAs = element(); const remove = element();
    const keyboardLayout = element("US"); const keyboardLayoutStatus = element();
    const color = element("#123456"); const scope = element("0"); const apply = element(); const assignment = element(); const close = element();
    const dialog = element(); const name = element(); const create = element(); const cancel = element(); const profileStatus = element(); dialog.children = {"[data-lf-keyboard-profile-name]": name, "[data-lf-keyboard-profile-status]": profileStatus, "[data-lf-keyboard-profile-create]": create, "[data-lf-keyboard-profile-cancel]": cancel};
    const editorNode = element(); editorNode.hidden = true; const title = element(); const type = element("0"); const command = commandControl("0"); const hold = element(); const delay = element("30"); const delayWrap = element(); const status = element(); editorNode.children = {"[data-lf-keyboard-editor-title]": title, "[data-lf-keyboard-type]": type, "[data-lf-keyboard-command]": command, "[data-lf-keyboard-hold]": hold, "[data-lf-keyboard-delay]": delay, "[data-lf-keyboard-delay-wrap]": delayWrap, "[data-lf-keyboard-status]": status};
    const key = element(); key.textContent = "G6"; key.assignmentTarget = true; key.dataset = {lfKeyIndex: "6", lfKeyRed: "17", lfKeyGreen: "34", lfKeyBlue: "51", lfNormalColor: "rgba(17, 34, 51, 1)", lfDefault: "1", lfActionType: "0", lfActionCommand: "0", lfDeviceId: "", lfActionHold: "0", lfToggleDelay: "30"}; key.style.color = key.dataset.lfNormalColor; const keyB = element(); keyB.textContent = "G7"; keyB.assignmentTarget = true; keyB.dataset = Object.assign({}, key.dataset, {lfKeyIndex: "7", lfKeyRed: "68", lfKeyGreen: "85", lfKeyBlue: "102", lfNormalColor: "rgba(68, 85, 102, 1)"}); keyB.style.color = keyB.dataset.lfNormalColor; const keyC = element(); keyC.textContent = "G8"; keyC.assignmentTarget = true; keyC.dataset = Object.assign({}, key.dataset, {lfKeyIndex: "8", lfKeyRed: "119", lfKeyGreen: "136", lfKeyBlue: "153", lfNormalColor: "rgba(119, 136, 153, 1)"}); keyC.style.color = keyC.dataset.lfNormalColor; const led = element(); led.textContent = "M1"; led.dataset = {lfKeyIndex: "9", lfKeyRed: "170", lfKeyGreen: "187", lfKeyBlue: "204", lfNormalColor: "rgba(170, 187, 204, 1)"}; led.style.color = led.dataset.lfNormalColor;
    const colorGroup = element(); if (cluster) { colorGroup.setAttribute("aria-disabled", "true"); color.disabled = scope.disabled = apply.disabled = true; }
    const live = element(); live.checked = Boolean(liveEnabled); const liveStatus = element();
    const nodes = {"[data-lf-keyboard-editor]": editorNode, "[data-lf-keyboard-assignment-open]": assignment, "[data-lf-keyboard-assignment-close]": close, "[data-lf-keyboard-layout]": keyboardLayout, "[data-lf-keyboard-layout-status]": keyboardLayoutStatus, "[data-lf-keyboard-profile]": profile, "[data-lf-keyboard-profile-save]": save, "[data-lf-keyboard-profile-new]": saveAs, "[data-lf-keyboard-profile-delete]": remove, "[data-lf-keyboard-profile-dialog]": dialog, "[data-lf-keyboard-color-apply]": apply, "[data-lf-keyboard-color]": color, "[data-lf-keyboard-color-scope]": scope, "[data-lf-keyboard-color-group]": colorGroup, "[data-lf-keyboard-live-rgb]": live, "[data-lf-keyboard-live-rgb-status]": liveStatus};
    const keys = [key, keyB, keyC]; const colorKeys = keys.concat(led); const workspace = {dataset: {lfDeviceId: "k95"}, querySelector: function (selector) { return nodes[selector] || null; }, querySelectorAll: function (selector) { if (selector === "[data-lf-keyboard-key]") { return keys; } if (selector === "[data-lf-keyboard-color-key]" || selector === "[data-lf-key-index]") { return colorKeys; } return selector.indexOf("aria-pressed") >= 0 ? colorKeys.filter(function (item) { return item.getAttribute("aria-pressed") === "true"; }) : []; }};
    const posts = []; const timers = []; const browser = {document: {querySelector: function () { return workspace; }, readyState: "complete"}, fetch: async function (url, options) { posts.push({url: url, options: options}); return browser.response || {ok: true, json: async function () { return {status: 1}; }}; }, setInterval: function (handler) { const timer = {handler: handler, cleared: false}; timers.push(timer); return timer; }, clearInterval: function (timer) { timer.cleared = true; }, location: {reload: function () { browser.reloaded = true; }}, LumenForgeDevicesToast: function (message, kind) { browser.toasted = true; browser.toastMessage = message; browser.toastKind = kind; }};
    assignments.init(browser); return {browser: browser, nodes: nodes, workspace: workspace, key: key, keyB: keyB, keyC: keyC, led: led, posts: posts, timers: timers};
}

test("Non-assignable RGB positions remain color-selectable without activating assignments", async function () {
    const h = toolbarHarness(false); await h.led.fire("click"); assert.equal(h.led.getAttribute("aria-pressed"), "true"); assert.equal(h.led.getAttribute("data-lf-current-key"), "true"); assert.equal(h.led.style.color, "rgba(170, 187, 204, 1)"); assert.equal(h.nodes["[data-lf-keyboard-color]"].value, "#aabbcc"); h.nodes["[data-lf-keyboard-assignment-open]"].fire("click"); assert.equal(h.nodes["[data-lf-keyboard-editor]"].hidden, true); assert.equal(h.posts.length, 0); assert.equal(h.browser.toastMessage, "Key assignments are not supported for this LED."); assert.equal(h.browser.toastKind, "warning");
});

test("Non-assignable RGB positions participate in Selected Keys color payloads", async function () {
    const h = toolbarHarness(false); const scope = h.nodes["[data-lf-keyboard-color-scope]"]; scope.value = "3"; await scope.fire("change"); await h.key.fire("click"); await h.led.fire("click"); await h.nodes["[data-lf-keyboard-color-apply]"].fire("click"); const payload = JSON.parse(h.posts[h.posts.length - 1].options.body); assert.equal(payload.keyOption, 3); assert.deepEqual(payload.keys, [6, 9]);
});

test("Color Apply updates a non-assignable RGB position's normal visual color", async function () {
    const h = toolbarHarness(false); await h.led.fire("click"); h.nodes["[data-lf-keyboard-color]"].value = "#c0ffee"; await h.nodes["[data-lf-keyboard-color-apply]"].fire("click"); assert.equal(h.led.dataset.lfNormalColor, "rgba(192, 255, 238, 1)"); assert.equal(h.led.style.color, "rgba(192, 255, 238, 1)"); assert.equal(h.led.dataset.lfKeyRed, "192");
});

test("Color Apply posts the first selected target once and commits visual color after success", async function () {
    const h = toolbarHarness(false); const response = deferred(); h.browser.fetch = function (url, options) { h.posts.push({url: url, options: options}); return response.promise; }; await h.led.fire("click"); h.nodes["[data-lf-keyboard-color]"].value = "#c0ffee"; const applying = h.nodes["[data-lf-keyboard-color-apply]"].fire("click"); h.nodes["[data-lf-keyboard-color-apply]"].fire("click"); assert.equal(h.posts.length, 1); assert.deepEqual(JSON.parse(h.posts[0].options.body), {deviceId: "k95", keyId: 9, keyOption: 0, color: {red: 192, green: 255, blue: 238}}); assert.equal(h.led.style.color, "rgba(170, 187, 204, 1)"); response.resolve({ok: true, json: async function () { return {status: 1}; }}); await applying; assert.equal(h.led.style.color, "rgba(192, 255, 238, 1)");
});

test("Failed Color Apply does not commit a normal visual color", async function () {
    const h = toolbarHarness(false); h.browser.response = {ok: true, json: async function () { return {status: 0}; }}; await h.led.fire("click"); h.nodes["[data-lf-keyboard-color]"].value = "#c0ffee"; await h.nodes["[data-lf-keyboard-color-apply]"].fire("click"); assert.equal(h.led.dataset.lfNormalColor, "rgba(170, 187, 204, 1)"); assert.equal(h.led.style.color, "rgba(170, 187, 204, 1)");
});

test("Color Apply updates normal color without replacing an active Live RGB frame", async function () {
    const h = toolbarHarness(false, "default", true); assignments.renderLiveFrame(h.workspace, {9: {red: 1, green: 2, blue: 3}}); await h.led.fire("click"); h.nodes["[data-lf-keyboard-color]"].value = "#c0ffee"; await h.nodes["[data-lf-keyboard-color-apply]"].fire("click"); assert.equal(JSON.parse(h.posts[0].options.body).keyId, 9); assert.equal(h.led.dataset.lfNormalColor, "rgba(192, 255, 238, 1)"); assert.equal(h.led.style.color, "rgb(1, 2, 3)"); const toggle = h.nodes["[data-lf-keyboard-live-rgb]"]; toggle.checked = false; await toggle.fire("change"); assert.equal(h.led.style.color, "rgba(192, 255, 238, 1)");
});

test("Live RGB toggle posts enable and disable payloads", async function () {
    const enable = toolbarHarness(false); const enableToggle = enable.nodes["[data-lf-keyboard-live-rgb]"]; enableToggle.checked = true; await enableToggle.fire("change"); assert.equal(enable.posts[0].url, "/api/keyboard/liveSync"); assert.deepEqual(JSON.parse(enable.posts[0].options.body), {deviceId: "k95", mode: 1}); assert.equal(enable.browser.toasted, true);
    const disable = toolbarHarness(false, "default", true); const disableToggle = disable.nodes["[data-lf-keyboard-live-rgb]"]; disableToggle.checked = false; await disableToggle.fire("change"); assert.deepEqual(JSON.parse(disable.posts[0].options.body), {deviceId: "k95", mode: 0});
});

test("Live RGB toggle restores its confirmed state after a failed save", async function () {
    const h = toolbarHarness(false); h.browser.response = {ok: true, json: async function () { return {status: 0}; }}; const toggle = h.nodes["[data-lf-keyboard-live-rgb]"]; toggle.checked = true; await toggle.fire("change"); assert.equal(toggle.checked, false); assert.equal(toggle.disabled, false); assert.equal(h.nodes["[data-lf-keyboard-live-rgb-status]"].textContent, "Couldn’t save Live RGB setting.");
});

test("RGB Cluster leaves Live RGB enabled while retaining color editing gates", function () {
    const h = toolbarHarness(true); assert.equal(h.nodes["[data-lf-keyboard-live-rgb]"].disabled, false); assert.equal(h.nodes["[data-lf-keyboard-color-apply]"].disabled, true);
});

test("Live RGB frames update matching keys without changing keyboard selection state", function () {
    const h = toolbarHarness(false); const unmatchedColor = h.keyB.style.color; h.key.setAttribute("aria-pressed", "true"); h.key.setAttribute("data-lf-current-key", "true"); assignments.renderLiveFrame({querySelectorAll: function () { return [h.key, h.keyB, h.keyC]; }}, {6: {red: 1, green: 2, blue: 3}, 8: {red: 10, green: 20, blue: 30}}); assert.equal(h.key.style.color, "rgb(1, 2, 3)"); assert.equal(h.keyC.style.color, "rgb(10, 20, 30)"); assert.equal(h.keyB.style.color, unmatchedColor); assert.equal(h.key.getAttribute("aria-pressed"), "true"); assert.equal(h.key.getAttribute("data-lf-current-key"), "true");
});

test("Disabling Live RGB restores normal key colors and stops its poller", async function () {
    const h = toolbarHarness(false, "default", true); assignments.renderLiveFrame({querySelectorAll: function () { return [h.key, h.keyB, h.keyC]; }}, {6: {red: 1, green: 2, blue: 3}}); const toggle = h.nodes["[data-lf-keyboard-live-rgb]"]; toggle.checked = false; await toggle.fire("change"); assert.equal(h.key.style.color, "rgba(17, 34, 51, 1)"); assert.equal(h.timers[0].cleared, true);
});

test("Live RGB poller avoids overlapping requests and ignores a stopped poller", async function () {
    const h = toolbarHarness(false, "default", true); const pending = deferred(); h.browser.fetch = function (url, options) { h.posts.push({url: url, options: options}); return pending.promise; }; h.timers[0].handler(); h.timers[0].handler(); assert.equal(h.posts.length, 1); const toggle = h.nodes["[data-lf-keyboard-live-rgb]"]; toggle.checked = false; const disabling = toggle.fire("change"); assert.equal(h.timers[0].cleared, true); pending.resolve({ok: true, json: async function () { return {status: 1, data: {keys: {6: {red: 1, green: 2, blue: 3}}}}; }}); await disabling; assert.equal(h.key.style.color, "rgba(17, 34, 51, 1)");
});

test("Color and Assignment Preset selector posts the existing profile-change payload", async function () {
    const h = toolbarHarness(false, "gaming"); assert.equal(h.nodes["[data-lf-keyboard-profile-delete]"].disabled, false); h.nodes["[data-lf-keyboard-profile]"].value = "work"; await h.nodes["[data-lf-keyboard-profile]"].fire("change");
    assert.deepEqual(JSON.parse(h.posts[0].options.body), {deviceId: "k95", keyboardProfileName: "work", new: false}); assert.equal(h.posts[0].url, "/api/keyboard/profile/change");
    assert.equal(h.browser.reloaded, true);
});

test("Keyboard Save posts for an active non-default profile", async function () {
    const h = toolbarHarness(false, "gaming"); await h.nodes["[data-lf-keyboard-profile-save]"].fire("click"); assert.equal(h.posts[0].url, "/api/keyboard/profile/save");
});

test("Keyboard Save Preset As dialog rejects empty values and creates named presets", async function () {
    const h = toolbarHarness(false); const dialog = h.nodes["[data-lf-keyboard-profile-dialog]"]; h.nodes["[data-lf-keyboard-profile-new]"].fire("click"); assert.equal(dialog.hidden, false); await dialog.children["[data-lf-keyboard-profile-create]"].fire("click"); assert.equal(h.posts.length, 0); assert.equal(dialog.children["[data-lf-keyboard-profile-status]"].textContent, "Enter a preset name."); dialog.children["[data-lf-keyboard-profile-name]"].value = "gaming"; await dialog.children["[data-lf-keyboard-profile-create]"].fire("click"); assert.equal(h.posts[0].url, "/api/keyboard/profile/new"); assert.deepEqual(JSON.parse(h.posts[0].options.body), {deviceId: "k95", keyboardProfileName: "gaming", new: true}); assert.equal(h.browser.reloaded, true);
});

test("RGB Cluster disables only non-default keyboard color controls", function () {
    const h = toolbarHarness(true, "gaming"); assert.equal(h.nodes["[data-lf-keyboard-color]"].disabled, true); assert.equal(h.nodes["[data-lf-keyboard-color-scope]"].disabled, true); assert.equal(h.nodes["[data-lf-keyboard-color-apply]"].disabled, true); assert.equal(h.nodes["[data-lf-keyboard-profile]"].disabled, false); assert.equal(h.nodes["[data-lf-keyboard-profile-save]"].disabled, false); assert.equal(h.nodes["[data-lf-keyboard-profile-new]"].disabled, false); assert.equal(h.nodes["[data-lf-keyboard-profile-delete]"].disabled, false); assert.equal(h.nodes["[data-lf-keyboard-assignment-open]"].disabled, false);
});

test("Color and Assignment Preset change failure restores the confirmed preset without reloading", async function () {
    const h = toolbarHarness(false, "gaming"); h.browser.response = {ok: true, json: async function () { return {status: 0}; }}; const profile = h.nodes["[data-lf-keyboard-profile]"]; profile.value = "default"; await profile.fire("change"); assert.equal(profile.value, "gaming"); assert.equal(h.browser.reloaded, undefined); assert.equal(h.nodes["[data-lf-keyboard-editor]"].children["[data-lf-keyboard-status]"].textContent, "Couldn’t change Color & Assignment Preset.");
});

test("Keyboard Save Preset As failure retains the dialog and entered name", async function () {
    const h = toolbarHarness(false); h.browser.response = {ok: true, json: async function () { return {status: 0}; }}; const dialog = h.nodes["[data-lf-keyboard-profile-dialog]"]; h.nodes["[data-lf-keyboard-profile-new]"].fire("click"); dialog.children["[data-lf-keyboard-profile-name]"].value = "gaming"; await dialog.children["[data-lf-keyboard-profile-create]"].fire("click"); assert.equal(dialog.hidden, false); assert.equal(dialog.children["[data-lf-keyboard-profile-name]"].value, "gaming"); assert.equal(dialog.children["[data-lf-keyboard-profile-status]"].textContent, "Couldn’t create Color & Assignment Preset."); assert.equal(h.browser.reloaded, undefined); assert.equal(h.browser.toasted, undefined);
});

test("Keyboard Layout posts the selected physical layout and reloads", async function () {
    const h = toolbarHarness(false); const layout = h.nodes["[data-lf-keyboard-layout]"]; layout.value = "UK"; await layout.fire("change"); assert.equal(h.posts[0].url, "/api/keyboard/layout"); assert.deepEqual(JSON.parse(h.posts[0].options.body), {deviceId: "k95", keyboardLayout: "UK"}); assert.equal(h.browser.reloaded, true);
});

test("Keyboard Layout failure restores the confirmed physical layout", async function () {
    const h = toolbarHarness(false); h.browser.response = {ok: true, json: async function () { return {status: 0}; }}; const layout = h.nodes["[data-lf-keyboard-layout]"]; layout.value = "UK"; await layout.fire("change"); assert.equal(layout.value, "US"); assert.equal(h.nodes["[data-lf-keyboard-layout-status]"].textContent, "Couldn’t change keyboard layout.");
});

test("Color and Assignment Preset deletion posts and reloads", async function () {
    const h = toolbarHarness(false, "gaming"); const profile = h.nodes["[data-lf-keyboard-profile]"]; const remove = h.nodes["[data-lf-keyboard-profile-delete]"]; await remove.fire("click"); assert.equal(remove.disabled, false); assert.equal(h.posts[0].url, "/api/keyboard/profile/delete"); assert.deepEqual(JSON.parse(h.posts[0].options.body), {deviceId: "k95", keyboardProfileName: "gaming", new: false}); assert.equal(h.browser.reloaded, true);
});

test("Keyboard assignment panel is closed initially, opens for a selected key, and closes without mutation", async function () {
    const h = toolbarHarness(false, "gaming"); const editor = h.nodes["[data-lf-keyboard-editor]"]; const open = h.nodes["[data-lf-keyboard-assignment-open]"]; const close = h.nodes["[data-lf-keyboard-assignment-close]"]; assert.equal(editor.hidden, true); assert.equal(close.hidden, true); open.fire("click"); assert.equal(editor.hidden, true); await h.key.fire("click"); open.fire("click"); assert.equal(editor.hidden, false); assert.equal(close.hidden, false); assert.equal(editor.children["[data-lf-keyboard-editor-title]"].textContent, "Key Assignment — G6"); close.fire("click"); assert.equal(editor.hidden, true); assert.equal(close.hidden, true); assert.equal(h.posts.length, 0); assert.equal(h.key.dataset.lfDefault, "1");
});

test("Assignment fields remain disabled through pending save and current-key option loading", async function () {
    const h = toolbarHarness(false, "gaming"); const editor = h.nodes["[data-lf-keyboard-editor]"]; const fields = [editor.children["[data-lf-keyboard-type]"], editor.children["[data-lf-keyboard-command]"], editor.children["[data-lf-keyboard-hold]"], editor.children["[data-lf-keyboard-delay]"]]; const close = h.nodes["[data-lf-keyboard-assignment-close]"]; const saveResponse = deferred(); const optionResponse = deferred();
    h.browser.fetch = function (url, options) { h.posts.push({url: url, options: options}); return url === "/api/keyboard/updateKeyAssignment" ? saveResponse.promise : optionResponse.promise; };
    await h.key.fire("click"); h.nodes["[data-lf-keyboard-assignment-open]"].fire("click"); const hold = editor.children["[data-lf-keyboard-hold]"]; hold.checked = true; const saving = hold.fire("change"); for (const field of fields) { assert.equal(field.disabled, true); } assert.equal(close.disabled, false); close.fire("click"); assert.equal(editor.hidden, true);
    h.keyB.dataset.lfActionType = "3"; h.keyB.dataset.lfActionCommand = "9"; const selectingB = h.keyB.fire("click"); for (const field of fields) { assert.equal(field.disabled, true); }
    saveResponse.resolve({ok: true, json: async function () { return {status: 1}; }}); await saving; assert.equal(h.key.dataset.lfActionHold, "1"); assert.equal(h.keyB.dataset.lfActionHold, "0"); for (const field of fields) { assert.equal(field.disabled, true); }
    optionResponse.resolve(keyboardResponse(9, "B command")); await selectingB; for (const field of fields) { assert.equal(field.disabled, false); } assert.equal(close.disabled, false);
});

test("Default Color and Assignment Preset permits edits but blocks Save and Delete", function () {
    const h = toolbarHarness(false); for (const selector of ["[data-lf-keyboard-profile]", "[data-lf-keyboard-profile-new]", "[data-lf-keyboard-color]", "[data-lf-keyboard-color-scope]", "[data-lf-keyboard-color-apply]", "[data-lf-keyboard-assignment-open]"]) { assert.equal(h.nodes[selector].disabled, false, selector); } for (const selector of ["[data-lf-keyboard-profile-save]", "[data-lf-keyboard-profile-delete]"]) { assert.equal(h.nodes[selector].disabled, true, selector); } assert.equal(h.nodes["[data-lf-keyboard-editor]"].hidden, true);
});

test("Non-default Color and Assignment Preset enables assignment, Save, Delete, and color editing", function () {
    const h = toolbarHarness(false, "gaming"); for (const selector of ["[data-lf-keyboard-profile]", "[data-lf-keyboard-profile-new]", "[data-lf-keyboard-profile-save]", "[data-lf-keyboard-profile-delete]", "[data-lf-keyboard-color]", "[data-lf-keyboard-color-scope]", "[data-lf-keyboard-color-apply]", "[data-lf-keyboard-assignment-open]"]) { assert.equal(h.nodes[selector].disabled, false, selector); }
});

test("Non-default assignment edits retain the backend custom-assignment semantics", async function () {
    const h = toolbarHarness(false, "gaming"); await h.key.fire("click"); const hold = h.nodes["[data-lf-keyboard-editor]"].children["[data-lf-keyboard-hold]"]; hold.checked = true; await hold.fire("change"); assert.equal(h.posts[0].url, "/api/keyboard/updateKeyAssignment"); assert.equal(JSON.parse(h.posts[0].options.body).enabled, false);
});

test("Default Color and Assignment Preset mutations remain available while Save and Delete stay guarded", async function () {
    const h = toolbarHarness(false); await h.key.fire("click"); const editorNode = h.nodes["[data-lf-keyboard-editor]"]; editorNode.children["[data-lf-keyboard-hold]"].checked = true; await editorNode.children["[data-lf-keyboard-hold]"].fire("change"); await h.nodes["[data-lf-keyboard-color-apply]"].fire("click"); await h.nodes["[data-lf-keyboard-profile-save]"].fire("click"); await h.nodes["[data-lf-keyboard-profile-delete]"].fire("click"); assert.deepEqual(h.posts.map(function (post) { return post.url; }), ["/api/keyboard/updateKeyAssignment", "/api/keyboard/color"]);
});

test("Default profile with RGB Cluster keeps Key Assignments and Save As available", function () {
    const h = toolbarHarness(true); assert.equal(h.nodes["[data-lf-keyboard-assignment-open]"].disabled, false); assert.equal(h.nodes["[data-lf-keyboard-profile-new]"].disabled, false); assert.equal(h.nodes["[data-lf-keyboard-profile-save]"].disabled, true); assert.equal(h.nodes["[data-lf-keyboard-profile-delete]"].disabled, true); for (const selector of ["[data-lf-keyboard-color]", "[data-lf-keyboard-color-scope]", "[data-lf-keyboard-color-apply]"]) { assert.equal(h.nodes[selector].disabled, true, selector); }
});

test("Save As from default follows the edited state through the existing create route", async function () {
    const h = toolbarHarness(false); await h.key.fire("click"); const hold = h.nodes["[data-lf-keyboard-editor]"].children["[data-lf-keyboard-hold]"]; hold.checked = true; await hold.fire("change"); const dialog = h.nodes["[data-lf-keyboard-profile-dialog]"]; h.nodes["[data-lf-keyboard-profile-new]"].fire("click"); dialog.children["[data-lf-keyboard-profile-name]"].value = "editeddefault"; await dialog.children["[data-lf-keyboard-profile-create]"].fire("click"); assert.equal(h.posts[0].url, "/api/keyboard/updateKeyAssignment"); assert.equal(h.posts[1].url, "/api/keyboard/profile/new"); assert.equal(JSON.parse(h.posts[1].options.body).keyboardProfileName, "editeddefault");
});

test("Unassigned assignment type keeps a disabled placeholder until a real command is chosen", async function () {
    const c = editor(commandControl()); const key = {}; const loaded = await assignments.populate({fetch: async function () { return keyboardResponse(8, "Real value"); }}, c, {actionType: 3, actionCommand: ""}, 1, key, function () { return true; }); assignments.updateDisabled(c, false, loaded); assert.equal(loaded, true); assert.equal(c.command.value, ""); assert.equal(c.command.children[0].textContent, "Select a value"); assert.equal(c.command.children[0].disabled, true); assert.equal(assignments.canSaveAssignment(loaded, c, {actionType: 3}), false); c.command.value = "8"; assert.equal(assignments.canSaveAssignment(loaded, c, {actionType: 3}), true);
});

async function dragAcross(keys) {
    await keys[0].fire("pointerdown");
    for (const key of keys.slice(1)) { await key.fire("pointerenter", {preventDefault: function () {}}); }
    await keys[keys.length - 1].fire("pointerup");
    await keys[keys.length - 1].fire("click");
}

test("Current Key scope keeps one current key and drag moves it", async function () {
    const h = toolbarHarness(false); await h.key.fire("click"); assert.equal(h.key.getAttribute("aria-pressed"), "true"); assert.equal(h.key.getAttribute("data-lf-current-key"), "true"); await h.keyB.fire("click"); assert.equal(h.key.getAttribute("aria-pressed"), "false"); assert.equal(h.keyB.getAttribute("aria-pressed"), "true"); await h.keyB.fire("click"); assert.equal(h.keyB.getAttribute("aria-pressed"), "false");
    await dragAcross([h.key, h.keyB, h.keyC]); assert.equal(h.key.getAttribute("aria-pressed"), "false"); assert.equal(h.keyB.getAttribute("aria-pressed"), "false"); assert.equal(h.keyC.getAttribute("aria-pressed"), "true"); assert.equal(h.keyC.getAttribute("data-lf-current-key"), "true");
    const scope = h.nodes["[data-lf-keyboard-color-scope]"]; scope.value = "3"; await scope.fire("change"); await h.key.fire("click"); scope.value = "0"; await scope.fire("change"); assert.equal(h.key.getAttribute("aria-pressed"), "true"); assert.equal(h.keyC.getAttribute("aria-pressed"), "false");
});

test("Current Row scope keeps one row anchor and collapses Selected Keys", async function () {
    const h = toolbarHarness(false); const scope = h.nodes["[data-lf-keyboard-color-scope]"]; scope.value = "3"; await scope.fire("change"); await h.key.fire("click"); await h.keyB.fire("click"); scope.value = "1"; await scope.fire("change"); assert.equal(h.key.getAttribute("aria-pressed"), "false"); assert.equal(h.keyB.getAttribute("aria-pressed"), "true"); await h.keyB.fire("click"); assert.equal(h.keyB.getAttribute("aria-pressed"), "false"); await h.keyB.fire("click"); await dragAcross([h.keyB, h.keyC]); assert.equal(h.keyB.getAttribute("aria-pressed"), "false"); assert.equal(h.keyC.getAttribute("aria-pressed"), "true");
});

test("All Keys scope keeps one optional current key and does not drag-select", async function () {
    const h = toolbarHarness(false); const scope = h.nodes["[data-lf-keyboard-color-scope]"]; scope.value = "2"; await scope.fire("change"); await h.key.fire("click"); await h.keyB.fire("click"); assert.equal(h.key.getAttribute("aria-pressed"), "false"); assert.equal(h.keyB.getAttribute("aria-pressed"), "true"); await h.keyB.fire("click"); assert.equal(h.keyB.getAttribute("aria-pressed"), "false"); await dragAcross([h.key, h.keyB, h.keyC]); assert.equal(h.key.getAttribute("aria-pressed"), "false"); assert.equal(h.keyB.getAttribute("aria-pressed"), "false"); assert.equal(h.keyC.getAttribute("aria-pressed"), "true");
    await h.nodes["[data-lf-keyboard-color-apply]"].fire("click"); assert.equal(JSON.parse(h.posts[h.posts.length - 1].options.body).keyOption, 2);
});

test("Selected Keys scope supports deterministic select and deselect drags", async function () {
    const h = toolbarHarness(false); const scope = h.nodes["[data-lf-keyboard-color-scope]"]; scope.value = "3"; await scope.fire("change"); await dragAcross([h.key, h.keyB, h.keyC, h.keyB]); for (const key of [h.key, h.keyB, h.keyC]) { assert.equal(key.getAttribute("aria-pressed"), "true"); } assert.equal(h.keyC.getAttribute("data-lf-current-key"), "true");
    await dragAcross([h.keyB, h.keyC]); assert.equal(h.key.getAttribute("aria-pressed"), "true"); assert.equal(h.keyB.getAttribute("aria-pressed"), "false"); assert.equal(h.keyC.getAttribute("aria-pressed"), "false"); assert.equal(h.key.getAttribute("data-lf-current-key"), "true");
    await h.nodes["[data-lf-keyboard-color-apply]"].fire("click"); const payload = JSON.parse(h.posts[h.posts.length - 1].options.body); assert.equal(payload.keyOption, 3); assert.deepEqual(payload.keys, [6]);
    h.nodes["[data-lf-keyboard-assignment-open]"].fire("click"); assert.equal(h.nodes["[data-lf-keyboard-editor]"].children["[data-lf-keyboard-editor-title]"].textContent, "Key Assignment — G6");
});

test("Selected Keys click toggles individual keys", async function () {
    const h = toolbarHarness(false); const scope = h.nodes["[data-lf-keyboard-color-scope]"]; scope.value = "3"; await scope.fire("change"); await h.key.fire("click"); await h.keyB.fire("click"); await h.keyB.fire("click"); assert.equal(h.key.getAttribute("aria-pressed"), "true"); assert.equal(h.keyB.getAttribute("aria-pressed"), "false"); assert.equal(h.key.getAttribute("data-lf-current-key"), "true");
});

test("Pointer cancellation leaves click selection intact", async function () {
    const h = toolbarHarness(false); await h.key.fire("pointerdown"); await h.key.fire("pointercancel"); await h.key.fire("click"); assert.equal(h.key.getAttribute("aria-pressed"), "true"); assert.equal(h.key.getAttribute("data-lf-current-key"), "true");
});
