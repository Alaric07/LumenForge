"use strict";

(function (root, factory) {
    const api = factory();
    if (typeof module === "object" && module.exports) { module.exports = api; }
    if (root && root.document) { const init = function () { api.init(root); }; if (root.document.readyState === "loading") { root.document.addEventListener("DOMContentLoaded", init); } else { init(); } }
})(typeof window === "undefined" ? null : window, function () {
    function post(browser, url, payload) { return browser.fetch(url, {method: "POST", body: JSON.stringify(payload)}).then(async function (response) { const result = response.ok ? await response.json() : null; if (!result || result.status !== 1) { throw new Error("headset setting rejected"); } }); }
    function init(browser) {
        const workspaces = browser.document.querySelectorAll("[data-lf-headset-workspace]");
        workspaces.forEach(function (workspace) {
            const deviceId = workspace.dataset.lfDeviceId;
            const equalizerSave = workspace.querySelector("[data-lf-headset-equalizer-save]");
            workspace.querySelectorAll("[data-lf-headset-equalizer-band]").forEach(function (input) {
                const value = input.closest(".lf-range-control").querySelector("[data-lf-headset-equalizer-value]");
                const update = function () { if (value) { value.textContent = input.value; } };
                input.addEventListener("input", update);
                update();
            });
            if (equalizerSave) { equalizerSave.addEventListener("click", function () { const equalizers = {}; workspace.querySelectorAll("[data-lf-headset-equalizer-band]").forEach(function (input) { equalizers[input.dataset.lfBandId] = Number(input.value); }); return post(browser, "/api/headset/equalizer", {deviceId: deviceId, equalizers: equalizers}).then(function () { if (browser.LumenForgeDevicesToast) { browser.LumenForgeDevicesToast("✓ Saved", "success", 1500); } }).catch(function () { const status = workspace.querySelector("[data-lf-headset-equalizer-status]"); if (status) { status.textContent = "Couldn’t save equalizer."; } }); }); }
            const muteSave = workspace.querySelector("[data-lf-headset-mute-indicator-save]");
            const mute = workspace.querySelector("[data-lf-headset-mute-indicator]");
            if (muteSave && mute) { muteSave.addEventListener("click", function () { return post(browser, "/api/headset/muteIndicator", {deviceId: deviceId, muteIndicator: Number(mute.value)}).then(function () { if (browser.LumenForgeDevicesToast) { browser.LumenForgeDevicesToast("✓ Saved", "success", 1500); } }).catch(function () { const status = workspace.querySelector("[data-lf-headset-mute-indicator-status]"); if (status) { status.textContent = "Couldn’t save mute indicator."; } }); }); }
            const sidetone = workspace.querySelector("[data-lf-headset-sidetone]"), sidetoneValue = workspace.querySelector("[data-lf-headset-sidetone-value]"), sidetoneOutput = workspace.querySelector("[data-lf-headset-sidetone-value-output]"), sidetoneSave = workspace.querySelector("[data-lf-headset-sidetone-save]");
            function sidetoneEnabled() { return sidetone && Number(sidetone.value) === 1; }
            if (sidetone) { sidetone.addEventListener("change", function () { if (sidetoneValue) { sidetoneValue.disabled = !sidetoneEnabled(); } }); }
            if (sidetoneValue) { sidetoneValue.addEventListener("input", function () { if (sidetoneOutput) { sidetoneOutput.textContent = sidetoneValue.value; } }); }
            if (sidetoneSave && sidetone) { sidetoneSave.addEventListener("click", function () { const enabled = Number(sidetone.value); return post(browser, "/api/headset/sidetone", {deviceId: deviceId, sideTone: enabled}).then(function () { if (enabled !== 1 || !sidetoneValue) { return; } return post(browser, "/api/headset/sidetoneValue", {deviceId: deviceId, sideToneValue: Number(sidetoneValue.value)}); }).then(function () { if (browser.LumenForgeDevicesToast) { browser.LumenForgeDevicesToast("✓ Saved", "success", 1500); } }).catch(function () { const status = workspace.querySelector("[data-lf-headset-sidetone-status]"); if (status) { status.textContent = "Couldn’t save sidetone."; } }); }); }
            workspace.querySelectorAll("[data-lf-headset-assignment]").forEach(function (assignment) { const save = assignment.querySelector("[data-lf-headset-assignment-save]"), type = assignment.querySelector("[data-lf-headset-assignment-type]"), command = assignment.querySelector("[data-lf-headset-assignment-command]"); if (save && type && command) { save.addEventListener("click", function () { const actionType = Number(type.value); return post(browser, "/api/headset/updateKeyAssignment", {deviceId: deviceId, keyIndex: Number(assignment.dataset.lfAssignmentId), enabled: assignment.dataset.lfAssignmentDefault === "1", pressAndHold: assignment.querySelector("[data-lf-headset-assignment-hold]").checked, onRelease: assignment.querySelector("[data-lf-headset-assignment-release]").checked, keyAssignmentType: actionType, keyAssignmentValue: Number(command.value), toggleDelay: 30}).then(function () { if (browser.LumenForgeDevicesToast) { browser.LumenForgeDevicesToast("✓ Saved", "success", 1500); } }).catch(function () { const status = assignment.querySelector("[data-lf-headset-assignment-status]"); if (status) { status.textContent = "Couldn’t save button assignment."; } }); }); } });
        });
    }
    return {init: init};
});
