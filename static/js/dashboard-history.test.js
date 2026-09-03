"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const history = require("./dashboard-history.js");

function fakeDocument() {
    function node(namespaceURI = "http://www.w3.org/1999/xhtml") {
        return {namespaceURI, attributes: {}, children: [], hidden: false, className: "", parentElement: null, setAttribute(name, value) { this.attributes[name] = String(value); }, getAttribute(name) { return this.attributes[name]; }, querySelector(selector) { return selector === "[data-lf-sparkline-readout]" ? this.children.find((child) => child.attributes?.["data-lf-sparkline-readout"] !== undefined) || null : null; }, append(...children) { children.forEach((child) => { child.parentElement = this; this.children.push(child); }); }};
    }
    return {createElement() { return node(); }, createElementNS(namespaceURI) { return node(namespaceURI); }};
}

test("history keys use stable telemetry identities", function () {
    assert.equal(history.keys.cpu(), "cpu");
    assert.equal(history.keys.gpu(1), "gpu:1");
    assert.equal(history.keys.memory("dimm", 2), "memory:dimm:2");
    assert.equal(history.keys.fanAverage("core"), "fan-average:native:core");
    assert.equal(history.keys.coolant("core"), "coolant:native:core");
    assert.equal(history.keys.probe("core", 3), "probe:native:core:3");
});

test("history accepts only finite numbers and retains the newest bounded samples", function () {
    const key = "test:bounded";
    assert.equal(history.append(key, NaN), false);
    assert.equal(history.append(key, Infinity), false);
    assert.equal(history.append(key, null), false);
    for (let index = 0; index <= history.HISTORY_MAX_SAMPLES; index++) history.append(key, index);
    assert.equal(history.read(key).length, history.HISTORY_MAX_SAMPLES);
    assert.equal(history.read(key)[0], 1);
    assert.equal(history.read(key).at(-1), history.HISTORY_MAX_SAMPLES);
});

test("history reads return snapshots that cannot mutate stored samples", function () {
    const key = "test:private";
    history.append(key, 1);
    history.append(key, 2);
    const snapshot = history.read(key);
    snapshot.push(NaN);
    snapshot[0] = 999;
    assert.deepEqual(history.read(key), [1, 2]);
});

test("sparkline geometry remains bounded and chronological", function () {
    assert.deepEqual(history.points([], "temperature"), []);
    assert.deepEqual(history.points([42], "temperature"), [{x: 60, y: 18}]);
    const rising = history.points([20, 22, 24], "temperature");
    assert.equal(rising.length, 3);
    assert.ok(rising[0].x < rising[1].x && rising[1].x < rising[2].x);
    rising.forEach((point) => { assert.ok(point.x >= 0 && point.x <= 120); assert.ok(point.y >= 0 && point.y <= 36); });
    const constant = history.points([900, 900], "rpm");
    assert.equal(constant[0].y, constant[1].y);
});

test("sparkline traces and inspection map positions to bounded numeric history", function () {
    const samples = [20, 22, 24, 26, 28];
    assert.match(history.trace(samples, "temperature"), /^M /);
    assert.equal(history.sampleIndex(samples, -12, 120), 0);
    assert.equal(history.sampleIndex(samples, 120, 120), 4);
    assert.equal(history.sampleIndex(samples, 60, 120), 2);
    assert.equal(history.sampleIndex([42], 99, 120), 0);
    assert.equal(history.sampleIndex(samples, 10, 0), 0);
});

test("history inspection formats relative age and telemetry values without timestamps", function () {
    assert.equal(history.ageText(0), "now");
    assert.equal(history.ageText(1), "3s ago");
    assert.equal(history.ageText(20), "1m 00s ago");
    assert.equal(history.ageText(199), "9m 57s ago");
    assert.equal(history.valueText(27.25, "temperature"), "27.3 °C");
    assert.equal(history.valueText(811.6, "rpm"), "812 RPM");
    assert.equal(history.inspection(history.keys.cpu(), "temperature", 0), null);
});

test("native cooling series retain independent traces and inspect their stored samples", function () {
    const fan = history.keys.fanAverage("core"), coolant = history.keys.coolant("core"), probe = history.keys.probe("core", 4);
    [600, 650].forEach((value) => history.append(fan, value));
    [31.2, 31.5].forEach((value) => history.append(coolant, value));
    [25.1, 25.4].forEach((value) => history.append(probe, value));
    assert.match(history.trace(history.read(fan), "rpm"), /^M /);
    assert.match(history.trace(history.read(coolant), "temperature"), /^M /);
    assert.match(history.trace(history.read(probe), "temperature"), /^M /);
    assert.equal(history.inspection(probe, "temperature", 0).text, "25.1 °C · 3s ago");
});

test("dynamically created cooling sparklines use a native SVG with the shared trace contract", function () {
    const inspector = history.createSparkline(fakeDocument(), history.keys.coolant("core"));
    const svg = inspector.children[0];
    assert.equal(inspector.className, "lf-dashboard-sparkline-inspector");
    assert.equal(svg.namespaceURI, "http://www.w3.org/2000/svg");
    assert.equal(svg.getAttribute("class"), "lf-dashboard-sparkline");
    assert.equal(svg.getAttribute("viewBox"), "0 0 120 36");
    assert.equal(svg.getAttribute("data-lf-sparkline-key"), "coolant:native:core");
    const readout = inspector.children[1];
    assert.equal(readout.namespaceURI, "http://www.w3.org/1999/xhtml");
    assert.equal(readout.hidden, true);
    assert.equal(history.readoutFor(svg), readout);
    history.showReadout(svg, "31.5 °C · now");
    assert.equal(readout.hidden, false);
    assert.equal(readout.textContent, "31.5 °C · now");
    history.clearReadout(svg);
    assert.equal(readout.hidden, true);
    assert.equal(readout.textContent, "");
});

test("redraw cleanup clears stale inspection state after a bounded-history shift", function () {
    const readout = {hidden: false, textContent: "2.0 °C · 3s ago"}, marker = {removed: false, remove() { this.removed = true; }};
    const svg = {dataset: {lfSparklineIndex: "1"}, querySelector(selector) { return selector === "[data-lf-sparkline-marker]" ? marker : null; }, parentElement: {querySelector(selector) { return selector === "[data-lf-sparkline-readout]" ? readout : null; }}};
    const before = [1, 2, 3], after = [2, 3, 4];
    assert.equal(before[Number(svg.dataset.lfSparklineIndex)], 2);
    history.clearInspection(svg);
    assert.equal(marker.removed, true);
    assert.equal(readout.hidden, true);
    assert.equal(readout.textContent, "");
    assert.equal(Object.hasOwn(svg.dataset, "lfSparklineIndex"), false);
    assert.equal(after.includes(2), true);
});
