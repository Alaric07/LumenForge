"use strict";

(function (root, factory) {
    const api = factory();
    if (typeof module === "object" && module.exports) { module.exports = api; }
    if (root && root.document) {
        const init = function () { api.init(root); };
        if (root.document.readyState === "loading") { root.document.addEventListener("DOMContentLoaded", init); } else { init(); }
    }
})(typeof window === "undefined" ? null : window, function () {
    function init(browser) {
        const workspace = browser.document.querySelector("[data-lf-screen-workspace]");
        if (!workspace || typeof browser.fetch !== "function") { return; }
        const control = workspace.querySelector("[data-lf-screen-profile]");
        const status = workspace.querySelector("[data-lf-screen-status]");
        if (!control) { return; }
        let confirmed = control.value;
        control.addEventListener("change", function () {
            const next = control.value;
            if (!next || next === confirmed) { control.value = confirmed; return; }
            control.disabled = true;
            if (status) { status.textContent = ""; }
            return browser.fetch("/api/lcd/profile", {method: "POST", body: JSON.stringify({deviceId: workspace.dataset.lfDeviceId, profile: next})}).then(async function (response) {
                const result = response.ok ? await response.json() : null;
                if (!result || result.status !== 1) { throw new Error("screen mutation rejected"); }
                confirmed = next;
                if (typeof browser.LumenForgeDevicesToast === "function") { browser.LumenForgeDevicesToast("✓ Saved", "success", 1500); }
            }).catch(function () {
                control.value = confirmed;
                if (status) { status.textContent = "Couldn’t save this screen selection."; }
            }).finally(function () { control.disabled = false; });
        });
    }

    return {init: init};
});
