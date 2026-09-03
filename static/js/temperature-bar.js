"use strict";

const dashboardTelemetry = (function () {
    const POLL_INTERVAL_MS = 3000;
    const GAUGE_MAX_CELSIUS = 100;

    function clampGaugePercent(celsius) {
        if (!Number.isFinite(celsius)) return 0;
        return Math.max(0, Math.min(100, (celsius / GAUGE_MAX_CELSIUS) * 100));
    }

    function updateGauge(gauge, celsius) {
        const progress = gauge.querySelector("[data-lf-dashboard-gauge-progress]");
        if (!progress) return;
        const radius = Number(progress.getAttribute("r"));
        if (!Number.isFinite(radius) || radius <= 0) return;
        const circumference = 2 * Math.PI * radius;
        const percent = clampGaugePercent(Number(celsius));
        progress.style.strokeDasharray = String(circumference);
        progress.style.strokeDashoffset = String(circumference * (1 - percent / 100));
    }

    function updateTemperatureElements(document, selector, value) {
        document.querySelectorAll(selector).forEach(function (element) {
            element.textContent = value;
        });
    }

    function updateCPU(document, value, celsius) {
        updateTemperatureElements(document, "[data-lf-dashboard-cpu-temperature]", value);
        const legacy = document.getElementById("cpu_temp");
        if (legacy) legacy.textContent = value;
        document.querySelectorAll('[data-lf-dashboard-gauge="cpu"]').forEach(function (gauge) {
            updateGauge(gauge, celsius);
        });
        const history = typeof globalThis.dashboardHistory === "object" ? globalThis.dashboardHistory : null;
        if (history?.append(history.keys.cpu(), celsius)) history.render(document, history.keys.cpu(), "temperature");
    }

    function updateGPU(document, index, value, celsius) {
        document.querySelectorAll("[data-lf-dashboard-gpu-temperature]").forEach(function (element) {
            if (String(element.getAttribute("data-lf-dashboard-gpu-temperature")) === String(index)) {
                element.textContent = value;
            }
        });
        const legacy = document.getElementById("gpu_temp_" + index);
        if (legacy) legacy.textContent = value;
        document.querySelectorAll('[data-lf-dashboard-gauge="gpu"]').forEach(function (gauge) {
            if (String(gauge.getAttribute("data-lf-dashboard-gpu-index")) === String(index)) {
                updateGauge(gauge, celsius);
            }
        });
        const history = typeof globalThis.dashboardHistory === "object" ? globalThis.dashboardHistory : null, key = history?.keys.gpu(index);
        if (key && history.append(key, celsius)) history.render(document, key, "temperature");
    }

    function updateMemory(document, memory) {
        document.querySelectorAll("[data-lf-dashboard-memory-temperature]").forEach(function (element) {
            if (String(element.getAttribute("data-lf-dashboard-memory-serial")) === String(memory.serial) &&
                String(element.getAttribute("data-lf-dashboard-memory-channel")) === String(memory.channelId)) {
                element.textContent = memory.temperature;
            }
        });
        document.querySelectorAll('[data-lf-dashboard-gauge="memory"]').forEach(function (gauge) {
            if (String(gauge.getAttribute("data-lf-dashboard-memory-serial")) === String(memory.serial) &&
                String(gauge.getAttribute("data-lf-dashboard-memory-channel")) === String(memory.channelId)) {
                updateGauge(gauge, memory.celsius);
            }
        });
        const history = typeof globalThis.dashboardHistory === "object" ? globalThis.dashboardHistory : null, key = history?.keys.memory(memory.serial, memory.channelId);
        if (key && history.append(key, memory.celsius)) history.render(document, key, "temperature");
    }

    function updateStorage(document, storage) {
        document.querySelectorAll("[data-lf-dashboard-storage-temperature]").forEach(function (element) {
            if (String(element.getAttribute("data-lf-dashboard-storage-temperature")) === String(storage.Key)) {
                element.textContent = storage.TemperatureString;
            }
        });
        const legacy = document.getElementById("storage_temp-" + storage.Key);
        if (legacy) legacy.textContent = storage.TemperatureString;
    }

    function fetchTemperatures(ajax, document) {
        ajax({url: "/api/cpuTemp", type: "get", success: function (result) { updateCPU(document, result.data, result.telemetry); }});
        ajax({url: "/api/gpuTemps", type: "get", success: function (result) {
            Object.entries(result.data || {}).forEach(function ([index, value]) {
                updateGPU(document, index, value, result.telemetry?.[index]);
            });
        }});
        ajax({url: "/api/storageTemp", type: "get", success: function (result) {
            (result.data || []).forEach(function (storage) { updateStorage(document, storage); });
        }});
    }

    function startPolling(ajax, document, setIntervalFn) {
        document.querySelectorAll("[data-lf-dashboard-gauge]").forEach(function (gauge) {
            updateGauge(gauge, gauge.getAttribute("data-lf-dashboard-celsius"));
        });
        fetchTemperatures(ajax, document);
        return setIntervalFn(function () { fetchTemperatures(ajax, document); }, POLL_INTERVAL_MS);
    }

    return {POLL_INTERVAL_MS, clampGaugePercent, fetchTemperatures, startPolling, updateCPU, updateGPU, updateMemory, updateStorage};
})();

if (typeof module === "object" && module.exports) module.exports = dashboardTelemetry;

if (typeof window !== "undefined" && window.document) {
    window.dashboardTelemetry = dashboardTelemetry;
    $(document).ready(function () {
        dashboardTelemetry.startPolling($.ajax, window.document, window.setInterval.bind(window));
    });
}
