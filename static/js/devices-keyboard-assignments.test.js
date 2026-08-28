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
    return {command: command, type: {disabled: false}, hold: {disabled: false}, status: {textContent: ""}};
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
    return {value: value || "", checked: false, disabled: false, hidden: false, textContent: "", dataset: {}, options: [{}], addEventListener: function (name, handler) { handlers[name] = handler; }, fire: function (name, event) { return handlers[name](event || {}); }, setAttribute: function (name, value) { attributes[name] = value; }, getAttribute: function (name) { return attributes[name] || null; }, querySelector: function (selector) { return this.children && this.children[selector]; }};
}

function toolbarHarness(cluster, activeProfile) {
    const confirmedProfile = activeProfile || "default"; const profile = element(); profile.textContent = confirmedProfile; profile.dataset.lfConfirmed = confirmedProfile; profile.setAttribute("aria-expanded", "false"); const profileList = element(); profileList.hidden = true; const profileOptions = ["default", "gaming", "work"].map(function (name) { const option = element(); option.textContent = name; option.dataset.lfKeyboardProfileName = name; option.setAttribute("aria-selected", name === confirmedProfile ? "true" : "false"); return option; }); const save = element(); const saveAs = element(); const remove = element();
    const color = element("#123456"); const scope = element("0"); const apply = element(); const assignment = element(); const close = element();
    const dialog = element(); const name = element(); const create = element(); const cancel = element(); const profileStatus = element(); dialog.children = {"[data-lf-keyboard-profile-name]": name, "[data-lf-keyboard-profile-status]": profileStatus, "[data-lf-keyboard-profile-create]": create, "[data-lf-keyboard-profile-cancel]": cancel};
    const editorNode = element(); editorNode.hidden = true; const title = element(); const type = element("0"); const command = commandControl("0"); const hold = element(); const status = element(); editorNode.children = {"[data-lf-keyboard-editor-title]": title, "[data-lf-keyboard-type]": type, "[data-lf-keyboard-command]": command, "[data-lf-keyboard-hold]": hold, "[data-lf-keyboard-status]": status};
    const key = element(); key.textContent = "G6"; key.dataset = {lfKeyIndex: "6", lfDefault: "1", lfActionType: "0", lfActionCommand: "0", lfDeviceId: "", lfActionHold: "0", lfToggleDelay: "30"}; const keyB = element(); keyB.textContent = "G7"; keyB.dataset = Object.assign({}, key.dataset, {lfKeyIndex: "7"}); const keyC = element(); keyC.textContent = "G8"; keyC.dataset = Object.assign({}, key.dataset, {lfKeyIndex: "8"});
    const colorGroup = element(); if (cluster) { colorGroup.setAttribute("aria-disabled", "true"); color.disabled = scope.disabled = apply.disabled = true; }
    const nodes = {"[data-lf-keyboard-editor]": editorNode, "[data-lf-keyboard-assignment-open]": assignment, "[data-lf-keyboard-assignment-close]": close, "[data-lf-keyboard-profile]": profile, "[data-lf-keyboard-profile-list]": profileList, "[data-lf-keyboard-profile-save]": save, "[data-lf-keyboard-profile-new]": saveAs, "[data-lf-keyboard-profile-delete]": remove, "[data-lf-keyboard-profile-dialog]": dialog, "[data-lf-keyboard-color-apply]": apply, "[data-lf-keyboard-color]": color, "[data-lf-keyboard-color-scope]": scope, "[data-lf-keyboard-color-group]": colorGroup};
    const keys = [key, keyB, keyC]; const workspace = {dataset: {lfDeviceId: "k95"}, querySelector: function (selector) { return nodes[selector] || null; }, querySelectorAll: function (selector) { if (selector === "[data-lf-keyboard-key]") { return keys; } if (selector === "[data-lf-keyboard-profile-option]") { return profileOptions; } return selector.indexOf("aria-pressed") >= 0 ? keys.filter(function (item) { return item.getAttribute("aria-pressed") === "true"; }) : []; }};
    const posts = []; const browser = {document: {querySelector: function () { return workspace; }, readyState: "complete"}, fetch: async function (url, options) { posts.push({url: url, options: options}); return browser.response || {ok: true, json: async function () { return {status: 1}; }}; }, location: {reload: function () { browser.reloaded = true; }}, LumenForgeDevicesToast: function () { browser.toasted = true; }};
    assignments.init(browser); return {browser: browser, nodes: nodes, profileOptions: profileOptions, key: key, keyB: keyB, keyC: keyC, posts: posts};
}

test("Keyboard toolbar profile controls post existing payloads for a non-default profile", async function () {
    const h = toolbarHarness(false, "gaming"); assert.equal(h.nodes["[data-lf-keyboard-profile-delete]"].disabled, false); await h.profileOptions[2].fire("click");
    assert.deepEqual(JSON.parse(h.posts[0].options.body), {deviceId: "k95", keyboardProfileName: "work", new: false}); assert.equal(h.posts[0].url, "/api/keyboard/profile/change");
    assert.equal(h.browser.reloaded, true);
});

test("Keyboard profile picker activates the selected default entry again", async function () {
    const h = toolbarHarness(false, "default"); const profile = h.nodes["[data-lf-keyboard-profile]"]; const list = h.nodes["[data-lf-keyboard-profile-list]"];
    await h.key.fire("click"); const hold = h.nodes["[data-lf-keyboard-editor]"].children["[data-lf-keyboard-hold]"]; hold.checked = true; await hold.fire("change");
    profile.fire("click"); assert.equal(list.hidden, false); assert.equal(profile.getAttribute("aria-expanded"), "true"); assert.equal(h.profileOptions[0].getAttribute("aria-selected"), "true");
    await h.profileOptions[0].fire("click");
    assert.equal(list.hidden, true); assert.equal(h.posts[1].url, "/api/keyboard/profile/change"); assert.deepEqual(JSON.parse(h.posts[1].options.body), {deviceId: "k95", keyboardProfileName: "default", new: false}); assert.equal(h.browser.reloaded, true);
    assert.equal(h.nodes["[data-lf-keyboard-profile-save]"].disabled, true); assert.equal(h.nodes["[data-lf-keyboard-profile-delete]"].disabled, true);
});

test("Keyboard Save posts for an active non-default profile", async function () {
    const h = toolbarHarness(false, "gaming"); await h.nodes["[data-lf-keyboard-profile-save]"].fire("click"); assert.equal(h.posts[0].url, "/api/keyboard/profile/save");
});

test("Keyboard Save As dialog rejects empty values and creates named profiles", async function () {
    const h = toolbarHarness(false); const dialog = h.nodes["[data-lf-keyboard-profile-dialog]"]; h.nodes["[data-lf-keyboard-profile-new]"].fire("click"); assert.equal(dialog.hidden, false); await dialog.children["[data-lf-keyboard-profile-create]"].fire("click"); assert.equal(h.posts.length, 0); assert.equal(dialog.children["[data-lf-keyboard-profile-status]"].textContent, "Enter a profile name."); dialog.children["[data-lf-keyboard-profile-name]"].value = "gaming"; await dialog.children["[data-lf-keyboard-profile-create]"].fire("click"); assert.equal(h.posts[0].url, "/api/keyboard/profile/new"); assert.deepEqual(JSON.parse(h.posts[0].options.body), {deviceId: "k95", keyboardProfileName: "gaming", new: true}); assert.equal(h.browser.reloaded, true);
});

test("RGB Cluster disables only non-default keyboard color controls", function () {
    const h = toolbarHarness(true, "gaming"); assert.equal(h.nodes["[data-lf-keyboard-color]"].disabled, true); assert.equal(h.nodes["[data-lf-keyboard-color-scope]"].disabled, true); assert.equal(h.nodes["[data-lf-keyboard-color-apply]"].disabled, true); assert.equal(h.nodes["[data-lf-keyboard-profile]"].disabled, false); assert.equal(h.nodes["[data-lf-keyboard-profile-save]"].disabled, false); assert.equal(h.nodes["[data-lf-keyboard-profile-new]"].disabled, false); assert.equal(h.nodes["[data-lf-keyboard-profile-delete]"].disabled, false); assert.equal(h.nodes["[data-lf-keyboard-assignment-open]"].disabled, false);
});

test("Keyboard profile change failure restores the confirmed profile without reloading", async function () {
    const h = toolbarHarness(false, "gaming"); h.browser.response = {ok: true, json: async function () { return {status: 0}; }}; const profile = h.nodes["[data-lf-keyboard-profile]"]; await h.profileOptions[0].fire("click"); assert.equal(profile.dataset.lfConfirmed, "gaming"); assert.equal(h.browser.reloaded, undefined); assert.equal(h.nodes["[data-lf-keyboard-editor]"].children["[data-lf-keyboard-status]"].textContent, "Couldn’t change keyboard profile.");
});

test("Keyboard Save As failure retains the dialog and entered name", async function () {
    const h = toolbarHarness(false); h.browser.response = {ok: true, json: async function () { return {status: 0}; }}; const dialog = h.nodes["[data-lf-keyboard-profile-dialog]"]; h.nodes["[data-lf-keyboard-profile-new]"].fire("click"); dialog.children["[data-lf-keyboard-profile-name]"].value = "gaming"; await dialog.children["[data-lf-keyboard-profile-create]"].fire("click"); assert.equal(dialog.hidden, false); assert.equal(dialog.children["[data-lf-keyboard-profile-name]"].value, "gaming"); assert.equal(dialog.children["[data-lf-keyboard-profile-status]"].textContent, "Couldn’t create keyboard profile."); assert.equal(h.browser.reloaded, undefined); assert.equal(h.browser.toasted, undefined);
});

test("Keyboard non-default profile deletion posts and reloads", async function () {
    const h = toolbarHarness(false, "gaming"); const profile = h.nodes["[data-lf-keyboard-profile]"]; const remove = h.nodes["[data-lf-keyboard-profile-delete]"]; await remove.fire("click"); assert.equal(remove.disabled, false); assert.equal(h.posts[0].url, "/api/keyboard/profile/delete"); assert.deepEqual(JSON.parse(h.posts[0].options.body), {deviceId: "k95", keyboardProfileName: "gaming", new: false}); assert.equal(h.browser.reloaded, true);
});

test("Keyboard assignment panel is closed initially, opens for a selected key, and closes without mutation", async function () {
    const h = toolbarHarness(false, "gaming"); const editor = h.nodes["[data-lf-keyboard-editor]"]; const open = h.nodes["[data-lf-keyboard-assignment-open]"]; const close = h.nodes["[data-lf-keyboard-assignment-close]"]; assert.equal(editor.hidden, true); assert.equal(close.hidden, true); open.fire("click"); assert.equal(editor.hidden, true); await h.key.fire("click"); open.fire("click"); assert.equal(editor.hidden, false); assert.equal(close.hidden, false); assert.equal(editor.children["[data-lf-keyboard-editor-title]"].textContent, "G6"); close.fire("click"); assert.equal(editor.hidden, true); assert.equal(close.hidden, true); assert.equal(h.posts.length, 0); assert.equal(h.key.dataset.lfDefault, "1");
});

test("Default keyboard profile permits working-copy edits but blocks Save and Delete", function () {
    const h = toolbarHarness(false); for (const selector of ["[data-lf-keyboard-profile]", "[data-lf-keyboard-profile-new]", "[data-lf-keyboard-color]", "[data-lf-keyboard-color-scope]", "[data-lf-keyboard-color-apply]", "[data-lf-keyboard-assignment-open]"]) { assert.equal(h.nodes[selector].disabled, false, selector); } for (const selector of ["[data-lf-keyboard-profile-save]", "[data-lf-keyboard-profile-delete]"]) { assert.equal(h.nodes[selector].disabled, true, selector); } assert.equal(h.nodes["[data-lf-keyboard-editor]"].hidden, true);
});

test("Non-default keyboard profile enables assignment, Save, Delete, and color editing", function () {
    const h = toolbarHarness(false, "gaming"); for (const selector of ["[data-lf-keyboard-profile]", "[data-lf-keyboard-profile-new]", "[data-lf-keyboard-profile-save]", "[data-lf-keyboard-profile-delete]", "[data-lf-keyboard-color]", "[data-lf-keyboard-color-scope]", "[data-lf-keyboard-color-apply]", "[data-lf-keyboard-assignment-open]"]) { assert.equal(h.nodes[selector].disabled, false, selector); }
});

test("Non-default assignment edits retain the backend custom-assignment semantics", async function () {
    const h = toolbarHarness(false, "gaming"); await h.key.fire("click"); const hold = h.nodes["[data-lf-keyboard-editor]"].children["[data-lf-keyboard-hold]"]; hold.checked = true; await hold.fire("change"); assert.equal(h.posts[0].url, "/api/keyboard/updateKeyAssignment"); assert.equal(JSON.parse(h.posts[0].options.body).enabled, false);
});

test("Default keyboard profile mutations update the working copy but Save and Delete remain guarded", async function () {
    const h = toolbarHarness(false); await h.key.fire("click"); const editorNode = h.nodes["[data-lf-keyboard-editor]"]; editorNode.children["[data-lf-keyboard-hold]"].checked = true; await editorNode.children["[data-lf-keyboard-hold]"].fire("change"); await h.nodes["[data-lf-keyboard-color-apply]"].fire("click"); await h.nodes["[data-lf-keyboard-profile-save]"].fire("click"); await h.nodes["[data-lf-keyboard-profile-delete]"].fire("click"); assert.deepEqual(h.posts.map(function (post) { return post.url; }), ["/api/keyboard/updateKeyAssignment", "/api/keyboard/color"]);
});

test("Default profile with RGB Cluster keeps Key Assignments and Save As available", function () {
    const h = toolbarHarness(true); assert.equal(h.nodes["[data-lf-keyboard-assignment-open]"].disabled, false); assert.equal(h.nodes["[data-lf-keyboard-profile-new]"].disabled, false); assert.equal(h.nodes["[data-lf-keyboard-profile-save]"].disabled, true); assert.equal(h.nodes["[data-lf-keyboard-profile-delete]"].disabled, true); for (const selector of ["[data-lf-keyboard-color]", "[data-lf-keyboard-color-scope]", "[data-lf-keyboard-color-apply]"]) { assert.equal(h.nodes[selector].disabled, true, selector); }
});

test("Save As from default follows the edited working state through the existing create route", async function () {
    const h = toolbarHarness(false); await h.key.fire("click"); const hold = h.nodes["[data-lf-keyboard-editor]"].children["[data-lf-keyboard-hold]"]; hold.checked = true; await hold.fire("change"); const dialog = h.nodes["[data-lf-keyboard-profile-dialog]"]; h.nodes["[data-lf-keyboard-profile-new]"].fire("click"); dialog.children["[data-lf-keyboard-profile-name]"].value = "workingcopy"; await dialog.children["[data-lf-keyboard-profile-create]"].fire("click"); assert.equal(h.posts[0].url, "/api/keyboard/updateKeyAssignment"); assert.equal(h.posts[1].url, "/api/keyboard/profile/new"); assert.equal(JSON.parse(h.posts[1].options.body).keyboardProfileName, "workingcopy");
});

test("Unassigned assignment type keeps a disabled placeholder until a real command is chosen", async function () {
    const c = editor(commandControl()); const key = {}; const loaded = await assignments.populate({fetch: async function () { return keyboardResponse(8, "Real value"); }}, c, {actionType: 3, actionCommand: ""}, 1, key, function () { return true; }); assignments.updateDisabled(c, false, loaded); assert.equal(loaded, true); assert.equal(c.command.value, ""); assert.equal(c.command.children[0].textContent, "Select a value"); assert.equal(c.command.children[0].disabled, true); assert.equal(assignments.canSaveAssignment(loaded, c, {actionType: 3}), false); c.command.value = "8"; assert.equal(assignments.canSaveAssignment(loaded, c, {actionType: 3}), true);
});

test("Keyboard color selection toggles every click and retains the last selected key as current", async function () {
    const h = toolbarHarness(false); await h.key.fire("click"); await h.keyB.fire("click"); assert.equal(h.key.getAttribute("aria-pressed"), "true"); assert.equal(h.keyB.getAttribute("aria-pressed"), "true"); await h.keyB.fire("click"); assert.equal(h.key.getAttribute("aria-pressed"), "true"); assert.equal(h.keyB.getAttribute("aria-pressed"), "false"); await h.key.fire("click"); assert.equal(h.key.getAttribute("aria-pressed"), "false");
});
