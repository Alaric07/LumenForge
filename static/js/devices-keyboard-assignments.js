"use strict";

(function (root, factory) {
    const api = factory();
    if (typeof module === "object" && module.exports) { module.exports = api; }
    if (root && root.document) {
        const init = function () { api.init(root); };
        if (root.document.readyState === "loading") { root.document.addEventListener("DOMContentLoaded", init); } else { init(); }
    }
})(typeof window === "undefined" ? null : window, function () {
    const endpoint = "/api/keyboard/updateKeyAssignment";
    const optionSources = {0: null, 1: "/api/input/media", 3: "/api/input/keyboard", 8: "/api/devices/mouse", 9: "/api/input/mouse", 10: "/api/macro/", 19: null};

    function normalizedOptions(value) {
        const data = value && value.data !== undefined ? value.data : value;
        if (!data || typeof data !== "object") { return []; }
        return (Array.isArray(data) ? data : Object.keys(data).map(function (id) { const item = data[id]; return item && typeof item === "object" ? Object.assign({id: id}, item) : {id: id, name: item}; })).map(function (item) {
            const value = item.id !== undefined ? item.id : (item.value !== undefined ? item.value : item.key);
            return value === undefined ? null : {value: String(value), label: item.name || item.Name || item.label || String(value)};
        }).filter(Boolean);
    }
    async function optionsFor(browser, type, current) {
        if (type === 0) { return [{value: "0", label: "None"}]; }
        if (type === 19) { return [{value: String(current || 0), label: "Profile " + String(current || 0)}]; }
        const source = optionSources[type];
        if (!source) { return []; }
        const response = await browser.fetch(source);
        if (!response.ok) { throw new Error("options unavailable"); }
        return normalizedOptions(await response.json());
    }
    function setOptions(select, options, current) {
        select.replaceChildren();
        if (current === "") { const placeholder = select.ownerDocument.createElement("option"); placeholder.value = ""; placeholder.textContent = "Select a value"; placeholder.disabled = true; placeholder.selected = true; select.appendChild(placeholder); }
        options.forEach(function (option) { const element = select.ownerDocument.createElement("option"); element.value = option.value; element.textContent = option.label; element.selected = String(option.value) === String(current); select.appendChild(element); });
    }
    function createToast(browser, workspace) {
        return (browser.LumenForgeDevicesToast || function () {});
    }
    function rgbFromHex(value) { const match = /^#([0-9a-f]{6})$/i.exec(value || ""); return match ? {red: parseInt(match[1].slice(0, 2), 16), green: parseInt(match[1].slice(2, 4), 16), blue: parseInt(match[1].slice(4, 6), 16)} : null; }
    function colorPayload(deviceId, scope, color, selectedKey, selectedKeys) { if (!color || (scope !== 2 && !selectedKey) || (scope === 3 && !selectedKeys.length)) { return null; } return {deviceId: deviceId, keyId: scope === 2 ? 1 : selectedKey.keyIndex, keyOption: scope, color: color, keys: scope === 3 ? selectedKeys.map(function (key) { return key.keyIndex; }) : undefined}; }
    function controls(editor) { return {title: editor.querySelector("[data-lf-keyboard-editor-title]"), type: editor.querySelector("[data-lf-keyboard-type]"), command: editor.querySelector("[data-lf-keyboard-command]"), hold: editor.querySelector("[data-lf-keyboard-hold]"), status: editor.querySelector("[data-lf-keyboard-status]")}; }
    function updateDisabled(c, pending, optionsReady) { c.type.disabled = Boolean(pending); c.command.disabled = Boolean(pending) || !optionsReady; c.hold.disabled = Boolean(pending); }
    function stateFor(key) { return {keyIndex: Number(key.dataset.lfKeyIndex), name: key.textContent.trim(), default: key.dataset.lfDefault === "1", actionType: Number(key.dataset.lfActionType), actionCommand: Number(key.dataset.lfActionCommand), deviceID: key.dataset.lfDeviceId || "", actionHold: key.dataset.lfActionHold === "1", toggleDelay: Number(key.dataset.lfToggleDelay) || 30}; }
    function commandFor(state) { return state.actionType === 8 ? state.deviceID : String(state.actionCommand); }
    function hasSelectedCommand(c, state) {
        const value = c.command.value;
        if (value === undefined || value === null || value === "") { return false; }
        if (state.actionType === 8) { return true; }
        return Number.isInteger(Number(value));
    }
    function canSaveAssignment(optionsReady, c, state) {
        if (!optionsReady) { return false; }
        return state.actionType === 0 || hasSelectedCommand(c, state);
    }
    async function populate(browser, c, state, revision, selectedKey, isCurrent) {
        c.command.disabled = true;
        try {
            const current = commandFor(state);
            const options = await optionsFor(browser, state.actionType, current);
            if (!isCurrent(revision, selectedKey)) { return false; }
            if (current !== "" && !options.some(function (option) { return String(option.value) === current; })) { throw new Error("current assignment unavailable"); }
            setOptions(c.command, options, current);
            c.status.textContent = "";
            return true;
        } catch (_) {
            if (!isCurrent(revision, selectedKey)) { return false; }
            c.status.textContent = "Unable to load assignment values.";
            return false;
        }
    }
    function init(browser) {
        const workspace = browser.document.querySelector("[data-lf-keyboard-assignments-workspace]");
        if (!workspace) { return; }
        const editor = workspace.querySelector("[data-lf-keyboard-editor]");
        if (!editor) { return; }
        const c = controls(editor); let selected = null; let confirmed = null; let saving = false; let optionsReady = false; let revision = 0; let colorSelection = [];
        if (!c.title || !c.type || !c.command || !c.hold || !c.status) { return; }
        async function select(key, event) {
            const selectionRevision = ++revision;
            if (colorSelection.includes(key)) { colorSelection = colorSelection.filter(function (item) { return item !== key; }); } else { colorSelection = colorSelection.concat(key); }
            workspace.querySelectorAll("[data-lf-keyboard-key]").forEach(function (item) { item.setAttribute("aria-pressed", colorSelection.includes(item) ? "true" : "false"); });
            if (!colorSelection.includes(key)) { selected = colorSelection[colorSelection.length - 1] || null; if (!selected) { confirmed = null; optionsReady = false; setAssignmentPanelVisible(false); } return; }
            selected = key; confirmed = stateFor(key); optionsReady = false; setAssignmentPanelVisible(false); c.title.textContent = confirmed.name; c.type.value = String(confirmed.actionType); c.hold.checked = confirmed.actionHold;
            updateDisabled(c, false, false);
            const loaded = await populate(browser, c, confirmed, selectionRevision, key, function (candidateRevision, candidateKey) { return candidateRevision === revision && candidateKey === selected; });
            if (selectionRevision === revision && key === selected) { optionsReady = loaded; updateDisabled(c, false, optionsReady); }
        }
        function restore() { if (selected) { select(selected); } }
        async function save() {
            if (!selected || saving || !canSaveAssignment(optionsReady, c, {actionType: Number(c.type.value)})) { return; }
            saving = true; updateDisabled(c, true, optionsReady); c.status.textContent = "";
            const type = Number(c.type.value); const stringValue = type === 8;
            const savingKey = selected; const savingRevision = revision; const savingState = confirmed;
            const payload = {deviceId: workspace.dataset.lfDeviceId, keyIndex: savingState.keyIndex, enabled: false, pressAndHold: c.hold.checked, keyAssignmentType: type, toggleDelay: savingState.toggleDelay};
            payload[stringValue ? "keyAssignmentValueString" : "keyAssignmentValue"] = stringValue ? c.command.value : Number(c.command.value);
            try {
                const response = await browser.fetch(endpoint, {method: "POST", body: JSON.stringify(payload)}); const result = response.ok ? await response.json() : null;
                if (!result || result.status !== 1) { throw new Error("save rejected"); }
                savingKey.dataset.lfDefault = payload.enabled ? "1" : "0"; savingKey.dataset.lfActionType = String(type); savingKey.dataset.lfActionCommand = stringValue ? "0" : String(payload.keyAssignmentValue); savingKey.dataset.lfDeviceId = stringValue ? payload.keyAssignmentValueString : ""; savingKey.dataset.lfActionHold = payload.pressAndHold ? "1" : "0"; if (selected === savingKey && revision === savingRevision) { confirmed = stateFor(savingKey); createToast(browser, workspace)("✓ Saved", "success", 1500); }
            } catch (_) { if (selected === savingKey && revision === savingRevision) { c.status.textContent = "Couldn’t save key assignment."; await restore(); } }
            saving = false; if (selected === savingKey && revision === savingRevision) { updateDisabled(c, false, optionsReady); }
        }
        workspace.querySelectorAll("[data-lf-keyboard-key]").forEach(function (key) { key.addEventListener("click", function (event) { return select(key, event); }); });
        const openAssignment = workspace.querySelector("[data-lf-keyboard-assignment-open]");
        const closeAssignment = workspace.querySelector("[data-lf-keyboard-assignment-close]");
        function setAssignmentPanelVisible(visible) { editor.hidden = !visible; if (closeAssignment) { closeAssignment.hidden = !visible; } }
        const profile = workspace.querySelector("[data-lf-keyboard-profile]");
        const profileList = workspace.querySelector("[data-lf-keyboard-profile-list]");
        const profileOptions = Array.from(workspace.querySelectorAll("[data-lf-keyboard-profile-option]"));
        const saveProfile = workspace.querySelector("[data-lf-keyboard-profile-save]");
        const deleteProfile = workspace.querySelector("[data-lf-keyboard-profile-delete]");
        const colorApply = workspace.querySelector("[data-lf-keyboard-color-apply]"); const colorInput = workspace.querySelector("[data-lf-keyboard-color]"); const colorScope = workspace.querySelector("[data-lf-keyboard-color-scope]");
        const colorGroup = workspace.querySelector("[data-lf-keyboard-color-group]");
        function confirmedProfile() { return profile && profile.dataset ? profile.dataset.lfConfirmed : ""; }
        function isDefaultProfile() { return !profile || confirmedProfile() === "default"; }
        function syncProfileGating() { const defaultProfile = isDefaultProfile(); const colorDisabled = colorGroup && colorGroup.getAttribute("aria-disabled") === "true"; if (saveProfile) { saveProfile.disabled = defaultProfile; } if (deleteProfile) { deleteProfile.disabled = defaultProfile; } if (colorInput) { colorInput.disabled = colorDisabled; } if (colorScope) { colorScope.disabled = colorDisabled; } if (colorApply) { colorApply.disabled = colorDisabled; } }
        if (openAssignment) { openAssignment.addEventListener("click", function () { if (!selected || !optionsReady) { c.status.textContent = "Select an assignable key first."; return; } setAssignmentPanelVisible(true); }); }
        if (closeAssignment) { closeAssignment.addEventListener("click", function () { setAssignmentPanelVisible(false); }); }
        function profileRequest(url, method, name, newProfile) { return browser.fetch(url, {method: method, body: JSON.stringify({deviceId: workspace.dataset.lfDeviceId, keyboardProfileName: name, new: Boolean(newProfile)})}).then(async function (response) { const result = response.ok ? await response.json() : null; if (!result || result.status !== 1) { throw new Error("profile rejected"); } createToast(browser, workspace)("✓ Saved", "success", 1500); if (url !== "/api/keyboard/profile/save" && browser.location) { browser.location.reload(); } }); }
        function setProfileListVisible(visible) { if (!profileList) { return; } profileList.hidden = !visible; profile.setAttribute("aria-expanded", visible ? "true" : "false"); }
        function activateProfile(name) { return profileRequest("/api/keyboard/profile/change", "POST", name).catch(function () { syncProfileGating(); c.status.textContent = "Couldn’t change keyboard profile."; }); }
        if (profile) {
            profile.addEventListener("click", function () { setProfileListVisible(profileList && profileList.hidden); });
            profile.addEventListener("keydown", function (event) { if (event.key === "Escape") { setProfileListVisible(false); } if (event.key === "ArrowDown") { event.preventDefault(); setProfileListVisible(true); if (profileOptions[0] && profileOptions[0].focus) { profileOptions[0].focus(); } } });
            profileOptions.forEach(function (option) { option.addEventListener("click", function () { setProfileListVisible(false); return activateProfile(option.dataset.lfKeyboardProfileName); }); });
        }
        if (saveProfile) { saveProfile.addEventListener("click", function () { if (isDefaultProfile()) { return Promise.resolve(); } return profileRequest("/api/keyboard/profile/save", "POST", "0").catch(function () { c.status.textContent = "Couldn’t save keyboard profile."; }); }); }
        if (deleteProfile && profile) { deleteProfile.addEventListener("click", function () { if (isDefaultProfile()) { return Promise.resolve(); } return profileRequest("/api/keyboard/profile/delete", "DELETE", confirmedProfile()).catch(function () { c.status.textContent = "Couldn’t delete keyboard profile."; }); }); }
        const profileDialog = workspace.querySelector("[data-lf-keyboard-profile-dialog]"); const newProfile = workspace.querySelector("[data-lf-keyboard-profile-new]");
        if (newProfile && profileDialog) { newProfile.addEventListener("click", function () { profileDialog.hidden = false; }); const name = profileDialog.querySelector("[data-lf-keyboard-profile-name]"); const status = profileDialog.querySelector("[data-lf-keyboard-profile-status]"); profileDialog.querySelector("[data-lf-keyboard-profile-create]").addEventListener("click", function () { if (!name.value.trim()) { status.textContent = "Enter a profile name."; return Promise.resolve(); } return profileRequest("/api/keyboard/profile/new", "PUT", name.value.trim(), true).then(function () { profileDialog.hidden = true; }).catch(function () { status.textContent = "Couldn’t create keyboard profile."; }); }); profileDialog.querySelector("[data-lf-keyboard-profile-cancel]").addEventListener("click", function () { profileDialog.hidden = true; }); }
        if (colorApply && colorInput && colorScope) { colorApply.addEventListener("click", function () { if (colorGroup && colorGroup.getAttribute("aria-disabled") === "true") { return; } const keyStates = Array.from(workspace.querySelectorAll("[data-lf-keyboard-key][aria-pressed='true']")).map(stateFor); const payload = colorPayload(workspace.dataset.lfDeviceId, Number(colorScope.value), rgbFromHex(colorInput.value), selected ? stateFor(selected) : null, keyStates); if (!payload) { c.status.textContent = "Select a key before applying color."; return; } return browser.fetch("/api/keyboard/color", {method: "POST", body: JSON.stringify(payload)}).then(async function (response) { const result = response.ok ? await response.json() : null; if (!result || result.status !== 1) { throw new Error("color rejected"); } createToast(browser, workspace)("✓ Saved", "success", 1500); if (browser.location) { browser.location.reload(); } }).catch(function () { c.status.textContent = "Couldn’t apply keyboard color."; }); }); }
        setAssignmentPanelVisible(false); syncProfileGating();
        c.hold.addEventListener("change", save); c.command.addEventListener("change", save);
        c.type.addEventListener("change", async function () {
            if (!selected) { return; }
            const typeRevision = ++revision;
            const selectedKey = selected;
            const next = stateFor(selectedKey); next.actionType = Number(c.type.value); next.actionCommand = ""; next.deviceID = ""; optionsReady = false;
            updateDisabled(c, false, false);
            const loaded = await populate(browser, c, next, typeRevision, selectedKey, function (candidateRevision, candidateKey) { return candidateRevision === revision && candidateKey === selected; });
            if (typeRevision !== revision || selectedKey !== selected) { return; }
            optionsReady = loaded;
            updateDisabled(c, false, optionsReady);
            if (optionsReady) { await save(); }
        });
    }
    return {canSaveAssignment: canSaveAssignment, colorPayload: colorPayload, init: init, normalizedOptions: normalizedOptions, optionsFor: optionsFor, populate: populate, rgbFromHex: rgbFromHex, stateFor: stateFor, updateDisabled: updateDisabled};
});
