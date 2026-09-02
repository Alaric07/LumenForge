"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const telemetry = require("./temperature-bar.js");

function element(attributes = {}, progress = null) {
    return {
        textContent: "",
        attributes,
        getAttribute(name) { return this.attributes[name]; },
        querySelector(selector) {
            return selector === "[data-lf-dashboard-gauge-progress]" ? progress : null;
        }
    };
}

function gauge(attributes = {}) {
    const progress = element({r: "42"});
    progress.style = {};
    return {gauge: element(attributes, progress), progress};
}

function documentFor(groups, ids = {}) {
    return {
        querySelectorAll(selector) { return groups[selector] || []; },
        getElementById(id) { return ids[id] || null; }
    };
}

test("CPU telemetry uses numeric Celsius for gauge progress while preserving displayed text", function () {
    const ring = gauge();
    const value = element();
    const document = documentFor({
        "[data-lf-dashboard-cpu-temperature]": [value],
        '[data-lf-dashboard-gauge="cpu"]': [ring.gauge]
    });

    telemetry.updateCPU(document, "122.0 °F", 50);
    assert.equal(value.textContent, "122.0 °F");
    assert.equal(Number(ring.progress.style.strokeDashoffset).toFixed(4), (Math.PI * 42).toFixed(4));
    assert.equal(telemetry.clampGaugePercent(-5), 0);
    assert.equal(telemetry.clampGaugePercent(55), 55);
});

test("CPU gauge clamps numeric Celsius progress at the visual bounds", function () {
    for (const [celsius, fraction] of [[0, 1], [50, 0.5], [100, 0], [-4, 1], [145, 0]]) {
        const ring = gauge();
        const document = documentFor({
            "[data-lf-dashboard-cpu-temperature]": [],
            '[data-lf-dashboard-gauge="cpu"]': [ring.gauge]
        });
        telemetry.updateCPU(document, "unmodified display", celsius);
        assert.equal(Number(ring.progress.style.strokeDashoffset).toFixed(4), (2 * Math.PI * 42 * fraction).toFixed(4));
    }
});

test("GPU telemetry updates every reported GPU index and its numeric ring", function () {
    const first = element({"data-lf-dashboard-gpu-temperature": "0"});
    const second = element({"data-lf-dashboard-gpu-temperature": "1"});
    const firstRing = gauge({"data-lf-dashboard-gpu-index": "0"});
    const secondRing = gauge({"data-lf-dashboard-gpu-index": "1"});
    const document = documentFor({
        "[data-lf-dashboard-gpu-temperature]": [first, second],
        '[data-lf-dashboard-gauge="gpu"]': [firstRing.gauge, secondRing.gauge]
    });

    telemetry.updateGPU(document, 0, "54.0 °C", 54);
    telemetry.updateGPU(document, 1, "149.0 °F", 65);
    assert.equal(first.textContent, "54.0 °C");
    assert.equal(second.textContent, "149.0 °F");
    assert.equal(Number(firstRing.progress.style.strokeDashoffset).toFixed(4), (2 * Math.PI * 42 * 0.46).toFixed(4));
    assert.equal(Number(secondRing.progress.style.strokeDashoffset).toFixed(4), (2 * Math.PI * 42 * 0.35).toFixed(4));
});

test("storage telemetry maps updates by stable storage key", function () {
    const first = element({"data-lf-dashboard-storage-temperature": "nvme0n1"});
    const second = element({"data-lf-dashboard-storage-temperature": "sda"});
    const document = documentFor({"[data-lf-dashboard-storage-temperature]": [first, second]});

    telemetry.updateStorage(document, {Key: "sda", TemperatureString: "41.0 °C"});
    assert.equal(first.textContent, "");
    assert.equal(second.textContent, "41.0 °C");
});

test("temperature polling uses the established three-second cadence", function () {
    const calls = [];
    let interval;
    const document = documentFor({
        "[data-lf-dashboard-cpu-temperature]": [],
        '[data-lf-dashboard-gauge="cpu"]': [],
        "[data-lf-dashboard-gpu-temperature]": [],
        '[data-lf-dashboard-gauge="gpu"]': [],
        "[data-lf-dashboard-gauge]": [],
        "[data-lf-dashboard-storage-temperature]": []
    });
    telemetry.startPolling(function (request) { calls.push(request.url); }, document, function (_, milliseconds) {
        interval = milliseconds;
        return 1;
    });

    assert.deepEqual(calls, ["/api/cpuTemp", "/api/gpuTemps", "/api/storageTemp"]);
    assert.equal(interval, 3000);
});
