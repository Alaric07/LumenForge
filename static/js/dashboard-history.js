"use strict";

const dashboardHistory = (function () {
    const HISTORY_MAX_SAMPLES = 200;
    const POLL_INTERVAL_MS = 3000;
    const MINIMUM_RANGE = {temperature: 5, rpm: 200};
    const SVG_NAMESPACE = "http://www.w3.org/2000/svg";
    const series = new Map();
    const keys = {cpu: () => "cpu", gpu: (index) => "gpu:" + index, memory: (serial, channelID) => "memory:" + serial + ":" + channelID, fanAverage: (serial) => "fan-average:native:" + serial, coolant: (serial) => "coolant:native:" + serial, probe: (serial, id) => "probe:native:" + serial + ":" + id};
    function append(key, value) { if (!Number.isFinite(value)) return false; const samples = series.get(key) || []; samples.push(value); if (samples.length > HISTORY_MAX_SAMPLES) samples.splice(0, samples.length - HISTORY_MAX_SAMPLES); series.set(key, samples); return true; }
    function read(key) { return (series.get(key) || []).slice(); }
    function points(samples, kind, width = 120, height = 36) {
        const valid = samples.filter(Number.isFinite); if (!valid.length) return [];
        let low = Math.min(...valid), high = Math.max(...valid), range = Math.max(high - low, MINIMUM_RANGE[kind] || MINIMUM_RANGE.temperature); low = (low + high - range) / 2; high = low + range;
        if (valid.length === 1) return [{x: width / 2, y: height / 2}];
        return valid.map((value, index) => ({x: index * width / (valid.length - 1), y: Math.max(0, Math.min(height, height - ((value - low) / (high - low)) * height))}));
    }
    function trace(samples, kind) { const values = points(samples, kind); if (!values.length) return ""; if (values.length === 1) return "M " + (values[0].x - 2).toFixed(2) + " " + values[0].y.toFixed(2) + " L " + (values[0].x + 2).toFixed(2) + " " + values[0].y.toFixed(2); return "M " + values.map((point) => point.x.toFixed(2) + " " + point.y.toFixed(2)).join(" L "); }
    function sampleIndex(samples, position, width) { if (!samples.length) return -1; if (samples.length === 1 || !Number.isFinite(width) || width <= 0) return 0; return Math.max(0, Math.min(samples.length - 1, Math.round((Math.max(0, Math.min(width, position)) / width) * (samples.length - 1)))); }
    function ageText(samplesAgo) { const seconds = Math.max(0, samplesAgo) * POLL_INTERVAL_MS / 1000; if (!seconds) return "now"; if (seconds < 60) return seconds + "s ago"; return Math.floor(seconds / 60) + "m " + String(seconds % 60).padStart(2, "0") + "s ago"; }
    function valueText(value, kind) { return kind === "rpm" ? Math.round(value) + " RPM" : value.toFixed(1) + " °C"; }
    function inspection(key, kind, index) { const samples = read(key), selected = Math.max(0, Math.min(samples.length - 1, index)); if (!samples.length || !Number.isFinite(samples[selected])) return null; return {index: selected, text: valueText(samples[selected], kind) + " · " + ageText(samples.length - 1 - selected)}; }
    function createSparkline(document, key) {
        const wrapper = document.createElement("div"), svg = document.createElementNS(SVG_NAMESPACE, "svg"), readout = document.createElement("span");
        wrapper.className = "lf-dashboard-sparkline-inspector"; wrapper.setAttribute("data-lf-sparkline-inspector", "");
        svg.setAttribute("class", "lf-dashboard-sparkline"); svg.setAttribute("viewBox", "0 0 120 36"); svg.setAttribute("preserveAspectRatio", "none"); svg.setAttribute("data-lf-sparkline-key", key); svg.setAttribute("tabindex", "-1"); svg.setAttribute("aria-label", "Telemetry history");
        readout.className = "lf-dashboard-sparkline-readout"; readout.setAttribute("data-lf-sparkline-readout", ""); readout.hidden = true;
        wrapper.append(svg, readout); return wrapper;
    }
    function readoutFor(svg) { return svg.parentElement?.querySelector("[data-lf-sparkline-readout]") || null; }
    function showReadout(svg, text) { const readout = readoutFor(svg); if (!readout) return; readout.textContent = text; readout.hidden = false; }
    function clearReadout(svg) { const readout = readoutFor(svg); if (!readout) return; readout.hidden = true; readout.textContent = ""; }
    function clearInspection(svg) { const marker = svg.querySelector("[data-lf-sparkline-marker]"); if (marker) marker.remove(); clearReadout(svg); delete svg.dataset.lfSparklineIndex; }
    function bindInspection(document, svg, key, kind) {
        if (svg.dataset.lfSparklineBound) return; svg.dataset.lfSparklineBound = "true";
        const clear = function () { clearInspection(svg); };
        const select = function (index) { const detail = inspection(key, kind, index); if (!detail) return; const chartPoints = points(read(key), kind), point = chartPoints[detail.index]; let marker = svg.querySelector("[data-lf-sparkline-marker]"); if (!marker) { marker = document.createElementNS(SVG_NAMESPACE, "circle"); marker.setAttribute("data-lf-sparkline-marker", ""); marker.setAttribute("r", "2.5"); svg.appendChild(marker); } marker.setAttribute("cx", point.x); marker.setAttribute("cy", point.y); showReadout(svg, detail.text); svg.dataset.lfSparklineIndex = String(detail.index); };
        svg.addEventListener("pointermove", function (event) { const rect = svg.getBoundingClientRect(), samples = read(key); select(sampleIndex(samples, event.clientX - rect.left, rect.width)); });
        svg.addEventListener("pointerleave", clear); svg.addEventListener("blur", clear);
        svg.addEventListener("focus", function () { const samples = read(key); select(samples.length - 1); });
        svg.addEventListener("keydown", function (event) { if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return; const samples = read(key), current = Number(svg.dataset.lfSparklineIndex); select((Number.isInteger(current) ? current : samples.length - 1) + (event.key === "ArrowLeft" ? -1 : 1)); event.preventDefault(); });
    }
    function render(document, key, kind) { if (!document) return; const d = trace(read(key), kind); document.querySelectorAll("[data-lf-sparkline-key]").forEach(function (svg) { if (svg.getAttribute("data-lf-sparkline-key") !== key) return; clearInspection(svg); let path = svg.querySelector("path"); if (!path) { path = document.createElementNS(SVG_NAMESPACE, "path"); path.setAttribute("data-lf-sparkline-trace", ""); svg.appendChild(path); } path.setAttribute("d", d); svg.setAttribute("tabindex", d ? "0" : "-1"); bindInspection(document, svg, key, kind); }); }
    return {HISTORY_MAX_SAMPLES, POLL_INTERVAL_MS, keys, append, read, points, trace, sampleIndex, ageText, valueText, inspection, createSparkline, readoutFor, showReadout, clearReadout, clearInspection, render};
})();
if (typeof module === "object" && module.exports) module.exports = dashboardHistory;
if (typeof window !== "undefined") window.dashboardHistory = dashboardHistory;
