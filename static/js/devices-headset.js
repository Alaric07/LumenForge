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
        });
    }
    return {init: init};
});
