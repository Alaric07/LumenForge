"use strict";

(function (root, factory) {
    const api = factory();
    if (typeof module === "object" && module.exports) { module.exports = api; }
    if (root && root.document) {
        const init = function () { api.init(root); };
        if (root.document.readyState === "loading") { root.document.addEventListener("DOMContentLoaded", init); } else { init(); }
    }
})(typeof window === "undefined" ? null : window, function () {
    function request(browser, url, payload) {
        return browser.fetch(url, {method: "POST", body: JSON.stringify(payload)}).then(async function (response) {
            const result = response.ok ? await response.json() : null;
            if (!result || result.status !== 1) { throw new Error("RGB topology mutation rejected"); }
        });
    }

    function saved(browser) {
        if (typeof browser.LumenForgeDevicesToast === "function") { browser.LumenForgeDevicesToast("✓ Saved", "success", 1500); }
    }

    function bindSelect(browser, workspace, row, selector, endpoint, key) {
        const control = row.querySelector(selector);
        const status = row.querySelector("[data-lf-rgb-topology-status]");
        const portID = Number(row.dataset.lfPortId);
        if (!control || !Number.isInteger(portID) || typeof browser.fetch !== "function") { return; }
        let confirmed = control.value;
        let pending = false;
        control.addEventListener("change", function () {
            const submitted = control.value;
            if (pending || submitted === confirmed) { control.value = confirmed; return Promise.resolve(); }
            pending = true; control.disabled = true; if (status) { status.textContent = ""; }
            const payload = {deviceId: workspace.dataset.lfDeviceId, portId: portID}; payload[key] = Number(submitted);
            return request(browser, endpoint, payload).then(function () {
                confirmed = submitted; saved(browser);
            }).catch(function () {
                control.value = confirmed;
                if (status) { status.textContent = "Couldn’t save this RGB topology setting."; }
            }).finally(function () { pending = false; control.disabled = false; });
        });
    }

    function init(browser) {
        const workspace = browser.document.querySelector("[data-lf-rgb-topology-workspace]");
        if (!workspace) { return; }
        Array.from(workspace.querySelectorAll("[data-lf-rgb-topology-port]")).forEach(function (row) {
            bindSelect(browser, workspace, row, "[data-lf-rgb-topology-type]", "/api/hub/type", "deviceType");
            bindSelect(browser, workspace, row, "[data-lf-rgb-topology-amount]", "/api/hub/amount", "deviceAmount");
        });
    }

    return {init: init};
});
