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

    function applyTelemetry(workspace, device) {
        if (!device || !device.devices) { return false; }
        const modules = Object.values(device.devices);
        let updated = false;
        Array.from(workspace.querySelectorAll("[data-lf-memory-temperature]")).forEach(function (element) {
            const module = modules.find(function (value) { return String(value.channelId) === element.dataset.lfChannelId; });
            if (module && Number(module.temperature) > 0 && typeof module.temperatureString === "string") {
                element.textContent = module.temperatureString;
                updated = true;
            }
        });
        return updated;
    }

    function createTelemetryPoller(browser, workspace) {
        const deviceID = workspace.dataset.lfDeviceId;
        if (!deviceID || typeof browser.fetch !== "function") { return null; }
        let pending = false;
        async function poll() {
            if (pending) { return false; }
            pending = true;
            try {
                const response = await browser.fetch("/api/devices/" + encodeURIComponent(deviceID));
                const result = response.ok ? await response.json() : null;
                return result ? applyTelemetry(workspace, result.device) : false;
            } catch (_) {
                return false;
            } finally {
                pending = false;
            }
        }
        const timer = typeof browser.setInterval === "function" ? browser.setInterval(poll, 1500) : null;
        return {poll: poll, stop: function () { if (timer !== null && typeof browser.clearInterval === "function") { browser.clearInterval(timer); } }};
    }

    function bindLabel(browser, workspace, row) {
        const channelID = Number(row.dataset.lfChannelId);
        const input = row.querySelector("[data-lf-memory-label]");
        const display = row.querySelector("[data-lf-memory-label-display]");
        const status = row.querySelector("[data-lf-memory-label-status]");
        if (!Number.isInteger(channelID) || !input || !display || typeof browser.fetch !== "function") { return; }
        let confirmed = row.dataset.lfConfirmedLabel || "";
        let saving = false;
        function close() { input.hidden = true; display.hidden = false; }
        function open() { input.value = confirmed; display.hidden = true; input.hidden = false; if (typeof input.focus === "function") { input.focus(); } }
        function restore() { input.value = confirmed; display.textContent = confirmed || row.dataset.lfModuleName || ""; close(); }
        function save() {
            if (saving) { return Promise.resolve(); }
            const next = input.value.trim();
            if (!next || next === confirmed) { restore(); return Promise.resolve(); }
            saving = true;
            input.disabled = true;
            if (status) { status.textContent = ""; }
            return browser.fetch("/api/label", {method: "POST", body: JSON.stringify({deviceId: workspace.dataset.lfDeviceId, channelId: channelID, deviceType: 0, label: next})}).then(async function (response) {
                const result = response.ok ? await response.json() : null;
                if (!result || result.status !== 1) { throw new Error("memory label rejected"); }
                confirmed = next;
                row.dataset.lfConfirmedLabel = next;
                display.textContent = next;
                close();
                saved(browser);
            }).catch(function () {
                restore();
                if (status) { status.textContent = "Couldn’t save this memory label."; }
            }).finally(function () { input.disabled = false; saving = false; });
        }
        display.addEventListener("click", open);
        input.addEventListener("blur", save);
        input.addEventListener("keydown", function (event) {
            if (event.key === "Enter") { event.preventDefault(); return save(); }
            if (event.key === "Escape") { event.preventDefault(); restore(); }
        });
    }

    function init(browser) {
        const labelWorkspace = browser.document.querySelector("[data-lf-memory-label-workspace]");
        if (labelWorkspace) {
            Array.from(labelWorkspace.querySelectorAll("[data-lf-memory-module]")).forEach(function (row) { bindLabel(browser, labelWorkspace, row); });
        }
        const telemetryWorkspace = browser.document.querySelector("[data-lf-memory-workspace]");
        if (telemetryWorkspace) { createTelemetryPoller(browser, telemetryWorkspace); }
    }

    return {applyTelemetry: applyTelemetry, bindLabel: bindLabel, createTelemetryPoller: createTelemetryPoller, init: init};
});
