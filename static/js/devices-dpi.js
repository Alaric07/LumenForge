"use strict";

(function (root, factory) {
    const api = factory();
    if (typeof module === "object" && module.exports) {
        module.exports = api;
    }
    if (root && root.document) {
        const init = function () { api.init(root); };
        if (root.document.readyState === "loading") {
            root.document.addEventListener("DOMContentLoaded", init);
        } else {
            init();
        }
    }
})(typeof window === "undefined" ? null : window, function () {
    const endpoint = "/api/devices/dpi";
    const activeStageEndpoint = "/api/devices/dpi/active";
    const sniperEndpoint = "/api/devices/dpi/sniper";
    const statusEndpoint = "/api/devices/dpi/status";
    const performanceEndpoints = {
        pollingRate: "/api/devices/performance/polling-rate",
        angleSnapping: "/api/devices/performance/angle-snapping",
        liftHeight: "/api/devices/performance/lift-height",
        keyboard: "/api/devices/performance/keyboard"
    };
    const keyboardPerformanceFields = {
        perf_winKey: "perf_winKey",
        perf_shiftTab: "perf_shiftTab",
        perf_altTab: "perf_altTab",
        perf_altF4: "perf_altF4"
    };

    function parseDPI(value, minimum, maximum) {
        if (!/^\d+$/.test(String(value))) {
            return null;
        }
        const dpi = Number(value);
        return Number.isInteger(dpi) && dpi >= minimum && dpi <= maximum ? dpi : null;
    }

    function normalizeColor(value) {
        const match = /^#([0-9a-f]{6})$/i.exec(String(value));
        return match ? "#" + match[1].toLowerCase() : null;
    }

    function rangeProgress(value, minimum, maximum) {
        return maximum === minimum ? 100 : ((value - minimum) * 100) / (maximum - minimum);
    }

    function bindRow(row, minimum, maximum) {
        const slider = row.querySelector("[data-lf-dpi-slider]");
        const number = row.querySelector("[data-lf-dpi-number]");
        const color = row.querySelector("[data-lf-dpi-color]");
        const hex = row.querySelector("[data-lf-dpi-hex]");
        if (!slider || !number || !color || !hex) {
            return null;
        }
        function setDPI(value) {
            const dpi = parseDPI(value, minimum, maximum);
            if (dpi === null) { return false; }
            slider.value = String(dpi);
            number.value = String(dpi);
            slider.style.setProperty("--lf-range-progress", rangeProgress(dpi, minimum, maximum) + "%");
            return true;
        }
        function setColor(value) {
            const normalized = normalizeColor(value);
            if (!normalized) { return false; }
            color.value = normalized;
            hex.value = normalized;
            return true;
        }
        setDPI(slider.value);
        setColor(color.value);
        slider.addEventListener("input", function () { setDPI(slider.value); });
        number.addEventListener("input", function () { setDPI(number.value); });
        number.addEventListener("change", function () { if (!setDPI(number.value)) { setDPI(slider.value); } });
        color.addEventListener("input", function () { setColor(color.value); });
        hex.addEventListener("input", function () { setColor(hex.value); });
        hex.addEventListener("change", function () { if (!setColor(hex.value)) { setColor(color.value); } });
        return function () {
            const dpi = parseDPI(number.value, minimum, maximum);
            const normalized = normalizeColor(hex.value);
            return dpi === null || !normalized ? null : {id: row.dataset.lfStageId, dpi: dpi, color: normalized};
        };
    }

    function setStageActive(row, active) {
        const activeClass = "lf-dpi-stage-active";
        const header = row.querySelector(".lf-dpi-stage-header");
        let changed = false;
        if (row.classList.contains(activeClass) !== active) {
            row.classList.toggle(activeClass, active);
            changed = true;
        }
        if (!header) { return changed; }
        const badge = header.querySelector(".lf-dpi-stage-state");
        if (active && !badge) {
            const nextBadge = row.ownerDocument.createElement("span");
            nextBadge.className = "lf-dpi-stage-state";
            nextBadge.textContent = "Active";
            header.appendChild(nextBadge);
            changed = true;
        } else if (!active && badge) {
            badge.remove();
            changed = true;
        }
        return changed;
    }

    function applyActiveStageState(workspace, state) {
        if (!state || typeof state.activeRegularStageId !== "string" || !state.activeRegularStageId || typeof state.sniperActive !== "boolean") {
            return false;
        }
        const rows = Array.from(workspace.querySelectorAll("[data-lf-dpi-row]"));
        const regularRows = rows.filter(function (row) { return !row.classList.contains("lf-dpi-stage-sniper"); });
        const activeRegular = regularRows.find(function (row) { return row.dataset.lfStageId === state.activeRegularStageId; });
        const sniper = rows.find(function (row) { return row.classList.contains("lf-dpi-stage-sniper"); });
        if (!activeRegular || !sniper) { return false; }
        let changed = false;
        regularRows.forEach(function (row) { changed = setStageActive(row, row === activeRegular) || changed; });
        return setStageActive(sniper, state.sniperActive) || changed;
    }

    function applyOverviewDPIState(workspace, state) {
        if (!state || typeof state.activeRegularStageId !== "string" || !state.activeRegularStageId) { return false; }
        const metadata = Array.from(workspace.querySelectorAll("[data-lf-overview-dpi-metadata]")).find(function (element) {
            return element.dataset.lfStageId === state.activeRegularStageId;
        });
        const value = workspace.querySelector("[data-lf-overview-dpi-value]");
        const stage = workspace.querySelector("[data-lf-overview-dpi-stage]");
        if (!metadata || !value || !stage) { return false; }
        let changed = false;
        if (value.textContent !== metadata.dataset.lfStageDpi) { value.textContent = metadata.dataset.lfStageDpi; changed = true; }
        if (stage.textContent !== metadata.dataset.lfStageName) { stage.textContent = metadata.dataset.lfStageName; changed = true; }
        return changed;
    }

    function createStatusPoller(browser, workspace) {
        const serial = workspace.dataset.lfDeviceSerial;
        if (!serial || typeof browser.fetch !== "function") { return null; }
        let pending = false;
        let generation = 0;
        async function poll() {
            if (pending) { return false; }
            pending = true;
            const requestGeneration = generation;
            try {
                const response = await browser.fetch(statusEndpoint + "?serial=" + encodeURIComponent(serial));
                const result = response.ok ? await response.json() : null;
                if (!result || result.status !== 1 || requestGeneration !== generation) { return false; }
                if (workspace.querySelector("[data-lf-overview-dpi-value]")) { return applyOverviewDPIState(workspace, result); }
                return applyActiveStageState(workspace, result);
            } catch (_) {
                return false;
            } finally {
                pending = false;
            }
        }
        const timer = typeof browser.setInterval === "function" ? browser.setInterval(poll, 1000) : null;
        return {
            invalidate: function () { generation += 1; },
            poll: poll,
            stop: function () { if (timer !== null && typeof browser.clearInterval === "function") { browser.clearInterval(timer); } }
        };
    }

    function sniperActive(workspace) {
        const sniper = Array.from(workspace.querySelectorAll("[data-lf-dpi-row]")).find(function (row) { return row.classList.contains("lf-dpi-stage-sniper"); });
        return sniper ? sniper.classList.contains("lf-dpi-stage-active") : false;
    }

    function activeRegularStageID(workspace) {
        const regular = Array.from(workspace.querySelectorAll("[data-lf-dpi-row]")).find(function (row) {
            return !row.classList.contains("lf-dpi-stage-sniper") && row.classList.contains("lf-dpi-stage-active");
        });
        return regular ? regular.dataset.lfStageId : null;
    }

    async function selectActiveStage(browser, workspace, poller, stageID) {
        const rows = Array.from(workspace.querySelectorAll("[data-lf-dpi-row]"));
        const selected = rows.find(function (row) { return row.dataset.lfStageId === stageID && !row.classList.contains("lf-dpi-stage-sniper"); });
        if (!selected) { return false; }
        try {
            const response = await browser.fetch(activeStageEndpoint, {method: "POST", body: JSON.stringify({serial: workspace.dataset.lfDeviceSerial, stageId: stageID})});
            const result = response.ok ? await response.json() : null;
            if (!result || result.status !== 1) { return false; }
            if (poller) { poller.invalidate(); }
            return applyActiveStageState(workspace, {activeRegularStageId: stageID, sniperActive: sniperActive(workspace)});
        } catch (_) {
            return false;
        }
    }

    async function setSniperActive(browser, workspace, poller, active) {
        const rows = Array.from(workspace.querySelectorAll("[data-lf-dpi-row]"));
        const sniper = rows.find(function (row) { return row.classList.contains("lf-dpi-stage-sniper"); });
        const regularStageID = activeRegularStageID(workspace);
        if (!sniper || !regularStageID) { return false; }
        try {
            const response = await browser.fetch(sniperEndpoint, {method: "POST", body: JSON.stringify({serial: workspace.dataset.lfDeviceSerial, active: active})});
            const result = response.ok ? await response.json() : null;
            if (!result || result.status !== 1) { return false; }
            if (poller) { poller.invalidate(); }
            return applyActiveStageState(workspace, {activeRegularStageId: regularStageID, sniperActive: active});
        } catch (_) {
            return false;
        }
    }

    function bindStageSelection(browser, workspace, poller) {
        let selecting = false;
        workspace.querySelectorAll("[data-lf-dpi-row]").forEach(function (row) {
            row.addEventListener("click", async function (event) {
                if (event.target.closest("input, label, button, a") || selecting) { return; }
                selecting = true;
                try {
                    if (row.classList.contains("lf-dpi-stage-sniper")) {
                        await setSniperActive(browser, workspace, poller, !row.classList.contains("lf-dpi-stage-active"));
                    } else {
                        await selectActiveStage(browser, workspace, poller, row.dataset.lfStageId);
                    }
                } finally {
                    selecting = false;
                }
            });
        });
    }

    function performanceValue(control, input) {
        return control.dataset.lfPerformanceKind === "angleSnapping" ? (input.checked ? 1 : 0) : Number(input.value);
    }

    function showPerformanceSaved(browser) {
        if (typeof browser.LumenForgeDevicesToast === "function") {
            browser.LumenForgeDevicesToast("✓ Saved", "success", 1500);
        }
    }

    async function savePerformanceControl(browser, workspace, control) {
        const kind = control.dataset.lfPerformanceKind;
        if (kind === "keyboardBoolean") {
            return saveKeyboardPerformanceControl(browser, workspace);
        }
        const endpointForKind = performanceEndpoints[kind];
        const input = control.querySelector("[data-lf-performance-input]");
        const status = control.querySelector("[data-lf-performance-status]");
        const deviceID = workspace.dataset.lfDeviceId;
        if (!endpointForKind || !input || !status || !deviceID || control.dataset.lfSaving === "true") { return false; }
        const previous = Number(control.dataset.lfConfirmedValue);
        const value = performanceValue(control, input);
        if (!Number.isInteger(value)) { return false; }
        control.dataset.lfSaving = "true";
        input.disabled = true;
        status.textContent = "";
        const payload = {deviceId: deviceID};
        payload[kind] = value;
        try {
            const response = await browser.fetch(endpointForKind, {method: "POST", body: JSON.stringify(payload)});
            const result = response.ok ? await response.json() : null;
            if (!result || result.status !== 1) { throw new Error("save rejected"); }
            control.dataset.lfConfirmedValue = String(value);
            status.textContent = "";
            showPerformanceSaved(browser);
            return true;
        } catch (_) {
            if (kind === "angleSnapping") {
                input.checked = previous !== 0;
            } else {
                input.value = String(previous);
            }
            status.textContent = "Unable to save setting. Try again.";
            return false;
        } finally {
            control.dataset.lfSaving = "false";
            input.disabled = false;
        }
    }

    function keyboardPerformanceControls(workspace) {
        const controls = Array.from(workspace.querySelectorAll("[data-lf-performance-control]"))
            .filter(function (control) { return control.dataset.lfPerformanceKind === "keyboardBoolean"; });
        const byID = {};
        for (const control of controls) {
            const id = control.dataset.lfPerformanceSettingId;
            const input = control.querySelector("[data-lf-performance-input]");
            const status = control.querySelector("[data-lf-performance-status]");
            if (!Object.prototype.hasOwnProperty.call(keyboardPerformanceFields, id) || byID[id] || !input || !status) {
                return null;
            }
            byID[id] = {control: control, input: input, status: status};
        }
        return Object.keys(keyboardPerformanceFields).every(function (id) { return byID[id]; }) && controls.length === Object.keys(keyboardPerformanceFields).length ? byID : null;
    }

    function keyboardPerformanceState(controls, confirmed) {
        const state = {};
        for (const id of Object.keys(keyboardPerformanceFields)) {
            const value = confirmed ? Number(controls[id].control.dataset.lfConfirmedValue) : (controls[id].input.checked ? 1 : 0);
            if (value !== 0 && value !== 1) { return null; }
            state[keyboardPerformanceFields[id]] = value === 1;
        }
        return state;
    }

    function setKeyboardPerformanceControls(controls, disabled, message) {
        Object.keys(controls).forEach(function (id) {
            controls[id].input.disabled = disabled;
            controls[id].status.textContent = message;
        });
    }

    async function saveKeyboardPerformanceControl(browser, workspace) {
        const deviceID = workspace.dataset.lfDeviceId;
        const controls = keyboardPerformanceControls(workspace);
        if (!deviceID || !controls || workspace.dataset.lfKeyboardPerformanceSaving === "true") { return false; }
        const state = keyboardPerformanceState(controls, false);
        if (!state) { return false; }
        workspace.dataset.lfKeyboardPerformanceSaving = "true";
        setKeyboardPerformanceControls(controls, true, "");
        try {
            const payload = Object.assign({deviceId: deviceID}, state);
            const response = await browser.fetch(performanceEndpoints.keyboard, {method: "POST", body: JSON.stringify(payload)});
            const result = response.ok ? await response.json() : null;
            if (!result || result.status !== 1) { throw new Error("save rejected"); }
            Object.keys(controls).forEach(function (id) { controls[id].control.dataset.lfConfirmedValue = controls[id].input.checked ? "1" : "0"; });
            setKeyboardPerformanceControls(controls, false, "");
            showPerformanceSaved(browser);
            return true;
        } catch (_) {
            const confirmed = keyboardPerformanceState(controls, true);
            if (confirmed) {
                Object.keys(keyboardPerformanceFields).forEach(function (id) { controls[id].input.checked = confirmed[keyboardPerformanceFields[id]]; });
            }
            setKeyboardPerformanceControls(controls, false, "Unable to save setting. Try again.");
            return false;
        } finally {
            workspace.dataset.lfKeyboardPerformanceSaving = "false";
        }
    }

    function initPerformance(browser) {
        const workspace = browser.document.querySelector("[data-lf-performance-workspace]");
        if (!workspace) { return; }
        workspace.querySelectorAll("[data-lf-performance-control]").forEach(function (control) {
            const input = control.querySelector("[data-lf-performance-input]");
            if (!input) { return; }
            input.addEventListener("change", function () { savePerformanceControl(browser, workspace, control); });
        });
    }

    function init(browser) {
        initPerformance(browser);
        const workspace = browser.document.querySelector("[data-lf-dpi-workspace]");
        const overview = browser.document.querySelector("[data-lf-overview-dpi]");
        if (!workspace) {
            if (overview) { createStatusPoller(browser, overview); }
            return;
        }
        const minimum = Number(workspace.dataset.lfMinimum);
        const maximum = Number(workspace.dataset.lfMaximum);
        const status = workspace.querySelector("[data-lf-dpi-status]");
        const save = workspace.querySelector("[data-lf-dpi-save]");
        if (!Number.isInteger(minimum) || !Number.isInteger(maximum) || minimum < 1 || maximum < minimum || !save) { return; }
        const readers = Array.from(workspace.querySelectorAll("[data-lf-dpi-row]")).map(function (row) { return bindRow(row, minimum, maximum); });
        if (readers.some(function (reader) { return reader === null; })) { return; }
        const statusPoller = createStatusPoller(browser, workspace);
        bindStageSelection(browser, workspace, statusPoller);
        let saving = false;
        save.addEventListener("click", async function () {
            if (saving) { return; }
            const stages = readers.map(function (reader) { return reader(); });
            if (stages.some(function (stage) { return stage === null || !stage.id; })) {
                status.textContent = "Enter valid DPI values and colors.";
                return;
            }
            saving = true;
            save.disabled = true;
            status.textContent = "Saving…";
            try {
                const response = await browser.fetch(endpoint, {method: "POST", body: JSON.stringify({serial: workspace.dataset.lfDeviceSerial, stages: stages})});
                const result = response.ok && await response.json();
                if (!result || result.status !== 1) { throw new Error("save rejected"); }
                status.textContent = "DPI settings saved.";
            } catch (_) {
                status.textContent = "Unable to save DPI settings. Try again.";
            } finally {
                saving = false;
                save.disabled = false;
            }
        });
        if (overview) { createStatusPoller(browser, overview); }
    }

    return {applyActiveStageState: applyActiveStageState, applyOverviewDPIState: applyOverviewDPIState, createStatusPoller: createStatusPoller, init: init, initPerformance: initPerformance, keyboardPerformanceControls: keyboardPerformanceControls, normalizeColor: normalizeColor, parseDPI: parseDPI, rangeProgress: rangeProgress, saveKeyboardPerformanceControl: saveKeyboardPerformanceControl, savePerformanceControl: savePerformanceControl, selectActiveStage: selectActiveStage, setSniperActive: setSniperActive};
});
