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
            if (!result || result.status !== 1) { throw new Error("cooling mutation rejected"); }
        });
    }

    function saved(browser) {
        if (typeof browser.LumenForgeDevicesToast === "function") { browser.LumenForgeDevicesToast("✓ Saved", "success", 1500); }
    }

    function labelText(row, label) {
        return label || row.dataset.lfLabelName || row.dataset.lfChannelName || "";
    }

    function applyTelemetry(workspace, device) {
        if (!device || !device.devices) { return false; }
        const channels = Object.values(device.devices);
        let updated = false;
        Array.from(workspace.querySelectorAll("[data-lf-cooling-rpm]")).forEach(function (element) {
            const channel = channels.find(function (value) { return String(value.channelId) === element.dataset.lfChannelId; });
            if (channel && Number.isFinite(channel.rpm)) { element.textContent = channel.rpm + " RPM"; updated = true; }
        });
        Array.from(workspace.querySelectorAll("[data-lf-cooling-temperature]")).forEach(function (element) {
            const channel = channels.find(function (value) { return String(value.channelId) === element.dataset.lfChannelId; });
            if (channel && typeof channel.temperatureString === "string") { element.textContent = channel.temperatureString; updated = true; }
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
        const label = row.querySelector("[data-lf-cooling-label]");
        const labelDisplay = row.querySelector("[data-lf-cooling-label-display]");
        const status = row.querySelector("[data-lf-cooling-status]");
        if (!Number.isInteger(channelID) || !label || !labelDisplay || typeof browser.fetch !== "function") { return; }
        let confirmedLabel = row.dataset.lfConfirmedLabel || "";
		let labelSaving = false;
        function mutate(control, url, payload, success) {
            control.disabled = true;
            if (status) { status.textContent = ""; }
            return request(browser, url, payload).then(function () { success(); saved(browser); }).catch(function () {
                if (status) { status.textContent = "Couldn’t save this cooling setting."; }
				throw new Error("cooling mutation rejected");
            }).finally(function () { control.disabled = false; });
        }
        function closeLabelEditor() {
            label.hidden = true;
            labelDisplay.hidden = false;
        }
        function openLabelEditor() {
            label.value = confirmedLabel;
            labelDisplay.hidden = true;
            label.hidden = false;
            if (typeof label.focus === "function") { label.focus(); }
        }
        function saveLabel() {
			if (labelSaving) { return Promise.resolve(); }
            const next = label.value.trim();
            if (!next || next === confirmedLabel) { label.value = confirmedLabel; closeLabelEditor(); return Promise.resolve(); }
			labelSaving = true;
			return mutate(label, "/api/label", {deviceId: workspace.dataset.lfDeviceId, channelId: channelID, deviceType: 0, label: next}, function () {
                confirmedLabel = next;
                row.dataset.lfConfirmedLabel = next;
                labelDisplay.textContent = labelText(row, next);
                closeLabelEditor();
            }).catch(function () { label.value = confirmedLabel; labelDisplay.textContent = labelText(row, confirmedLabel); closeLabelEditor(); }).finally(function () { labelSaving = false; });
        }
        labelDisplay.addEventListener("click", openLabelEditor);
        label.addEventListener("blur", saveLabel);
        label.addEventListener("keydown", function (event) {
            if (event.key === "Enter") { event.preventDefault(); return saveLabel(); }
            if (event.key === "Escape") { event.preventDefault(); label.value = confirmedLabel; closeLabelEditor(); }
        });
    }

    function bindChannel(browser, workspace, row) {
        const channelID = Number(row.dataset.lfChannelId);
        const profile = row.querySelector("[data-lf-cooling-profile]");
        const status = row.querySelector("[data-lf-cooling-status]");
        if (!Number.isInteger(channelID) || !profile || typeof browser.fetch !== "function") { return; }
        let confirmedProfile = row.dataset.lfConfirmedProfile || profile.value;
        profile.addEventListener("change", function () {
            const next = profile.value;
            if (!next || next === confirmedProfile) { profile.value = confirmedProfile; return; }
			profile.disabled = true;
			if (status) { status.textContent = ""; }
			return request(browser, "/api/speed", {deviceId: workspace.dataset.lfDeviceId, channelId: channelID, profile: next}).then(function () {
				confirmedProfile = next;
				row.dataset.lfConfirmedProfile = next;
				saved(browser);
			}).catch(function () {
				profile.value = confirmedProfile;
				if (status) { status.textContent = "Couldn’t save this cooling setting."; }
			}).finally(function () { profile.disabled = false; });
        });
    }

    function init(browser) {
        const workspace = browser.document.querySelector("[data-lf-cooling-workspace]");
        if (!workspace) { return; }
        Array.from(workspace.querySelectorAll("[data-lf-cooling-channel]")).forEach(function (row) { bindLabel(browser, workspace, row); bindChannel(browser, workspace, row); });
        Array.from(workspace.querySelectorAll("[data-lf-cooling-probe]")).forEach(function (row) { bindLabel(browser, workspace, row); });
        createTelemetryPoller(browser, workspace);
    }

    return {applyTelemetry: applyTelemetry, createTelemetryPoller: createTelemetryPoller, init: init};
});
