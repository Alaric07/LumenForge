"use strict";

(function (root, factory) {
    const api = factory();
    if (typeof module === "object" && module.exports) { module.exports = api; }
    if (root && root.document) {
        const init = function () { api.init(root); };
        if (root.document.readyState === "loading") { root.document.addEventListener("DOMContentLoaded", init); } else { init(); }
    }
})(typeof window === "undefined" ? null : window, function () {
    function saved(browser) {
        if (typeof browser.LumenForgeDevicesToast === "function") { browser.LumenForgeDevicesToast("✓ Saved", "success", 1500); }
    }

    function request(browser, url, payload) {
        return browser.fetch(url, {method: "POST", body: JSON.stringify(payload)}).then(async function (response) {
            const result = response.ok ? await response.json() : null;
            if (!result || result.status !== 1) { throw new Error("display mutation rejected"); }
        });
    }

    function bindSelect(browser, workspace, selector, url, payloadName) {
        const control = workspace.querySelector(selector);
        const status = workspace.querySelector("[data-lf-display-status]");
        if (!control || typeof browser.fetch !== "function") { return; }
        let confirmed = control.value;
        control.addEventListener("change", function () {
            const next = control.value;
            if (!next || next === confirmed) { control.value = confirmed; return; }
            const payload = {deviceId: workspace.dataset.lfDeviceId, channelId: Number(workspace.dataset.lfChannelId)};
            payload[payloadName] = payloadName === "image" ? next : Number(next);
            control.disabled = true;
            if (status) { status.textContent = ""; }
            return request(browser, url, payload).then(function () {
                confirmed = next;
                if (payloadName === "mode") {
                    const imageControl = workspace.querySelector("[data-lf-display-image-control]");
                    if (imageControl) { imageControl.hidden = next !== workspace.dataset.lfImageModeId; }
                }
                saved(browser);
            }).catch(function () {
                control.value = confirmed;
                if (status) { status.textContent = "Couldn’t save this display setting."; }
            }).finally(function () { control.disabled = false; });
        });
    }

    function init(browser) {
        const workspace = browser.document.querySelector("[data-lf-display-workspace]");
        if (!workspace) { return; }
        bindSelect(browser, workspace, "[data-lf-display-mode]", "/api/lcd", "mode");
        bindSelect(browser, workspace, "[data-lf-display-rotation]", "/api/lcd/rotation", "rotation");
        bindSelect(browser, workspace, "[data-lf-display-brightness]", "/api/lcd/brightness", "brightness");
        bindSelect(browser, workspace, "[data-lf-display-image]", "/api/lcd/image", "image");
    }

    return {bindSelect: bindSelect, init: init};
});
