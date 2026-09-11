"use strict";

(function (root, factory) {
    const api = factory();
    if (typeof module === "object" && module.exports) { module.exports = api; }
    if (root && root.document) { const init = function () { api.init(root); }; if (root.document.readyState === "loading") { root.document.addEventListener("DOMContentLoaded", init); } else { init(); } }
})(typeof window === "undefined" ? null : window, function () {
    function post(browser, url, payload) {
        return browser.fetch(url, {method: "POST", body: JSON.stringify(payload)}).then(async function (response) { const result = response.ok ? await response.json() : null; if (!result || result.status !== 1) { throw new Error("controller request rejected"); } return result; });
    }
    function status(node, message) { const target = node.querySelector("[data-lf-controller-status]"); if (target) { target.textContent = message; } }
    function number(node, selector) { return Number(node.querySelector(selector).value); }
    function bindRange(node) { node.addEventListener("input", function () { const output = node.parentNode.querySelector("[data-lf-controller-value]"); if (output) { output.textContent = node.value; } }); }
    function canvasPosition(canvas, event) {
        const rect = canvas.getBoundingClientRect();
        if (!rect || rect.width <= 0 || rect.height <= 0) { return null; }
        return {x: (event.clientX - rect.left) * canvas.width / rect.width, y: (event.clientY - rect.top) * canvas.height / rect.height, width: canvas.width, height: canvas.height};
    }
    function bindDisclosureRedraw(details, redraw) {
        if (!details || details.tagName !== "DETAILS") { return; }
        details.addEventListener("toggle", function () { if (details.open) { redraw(); } });
    }
    function curve(canvas, container) {
        const values = canvas.textContent.trim().split(";").filter(Boolean).map(function (item) { const split = item.split(":"); const xy = split[1].split(","); return {index: Number(split[0]), x: Number(xy[0]), y: Number(xy[1])}; });
        const context = canvas.getContext("2d");
        function draw() { const width = canvas.width, height = canvas.height, margin = 28; context.clearRect(0, 0, width, height); context.strokeStyle = "#637083"; context.strokeRect(margin, margin, width - margin * 2, height - margin * 2); context.strokeStyle = "#60a5fa"; context.lineWidth = 2; context.beginPath(); values.forEach(function (point, index) { const x = margin + point.x * (width - margin * 2) / 100; const y = height - margin - point.y * (height - margin * 2) / 100; if (index) { context.lineTo(x, y); } else { context.moveTo(x, y); } }); context.stroke(); }
        function syncInputs(point) { const row = container.querySelector('[data-lf-controller-point][data-lf-point-index="' + point.index + '"]'); if (!row) { return; } row.querySelector("[data-lf-controller-point-x]").value = point.x; row.querySelector("[data-lf-controller-point-y]").value = point.y; }
        function updatePoint(index, axis, value) { if (!Number.isInteger(value) || value < 0 || value > 100 || !values[index]) { return; } values[index][axis] = value; syncInputs(values[index]); draw(); }
        let dragging = -1;
        function closest(event) { const pos = canvasPosition(canvas, event); if (!pos) { return -1; } const margin = 28; let selected = -1, distance = Infinity; values.forEach(function (point, index) { const x = margin + point.x * (pos.width - margin * 2) / 100; const y = pos.height - margin - point.y * (pos.height - margin * 2) / 100; const next = Math.pow(x - pos.x, 2) + Math.pow(y - pos.y, 2); if (next < distance) { distance = next; selected = index; } }); return distance <= 144 ? selected : -1; }
        canvas.addEventListener("pointerdown", function (event) { const selected = closest(event); if (selected > 0 && selected < values.length - 1) { dragging = selected; canvas.setPointerCapture(event.pointerId); } });
        canvas.addEventListener("pointermove", function (event) { if (dragging < 0) { return; } const pos = canvasPosition(canvas, event); if (!pos) { return; } const margin = 28; updatePoint(dragging, "y", Math.round(Math.max(0, Math.min(100, (pos.height - margin - pos.y) * 100 / (pos.height - margin * 2))))); });
        canvas.addEventListener("pointerup", function () { dragging = -1; });
        container.querySelectorAll("[data-lf-controller-point]").forEach(function (row) { const index = Number(row.dataset.lfPointIndex); row.querySelector("[data-lf-controller-point-x]").addEventListener("input", function (event) { updatePoint(index, "x", Number(event.target.value)); }); row.querySelector("[data-lf-controller-point-y]").addEventListener("input", function (event) { updatePoint(index, "y", Number(event.target.value)); }); });
        values.redraw = draw;
        draw(); return values;
    }
    function init(browser) {
        const root = browser.document.querySelector("[data-lf-controller-workspace]"); if (!root) { return; }
        const workspace = browser.document;
        const deviceId = root.dataset.lfDeviceId;
        workspace.querySelectorAll("[data-lf-controller-vibration-input]").forEach(function (input) { bindRange(input); input.addEventListener("change", function () { const row = input.closest("[data-lf-controller-vibration]"); const section = input.closest("[data-lf-controller-workspace]"); post(browser, "/api/controller/vibration", {deviceId: deviceId, vibrationModule: Number(row.dataset.lfModule), vibrationValue: Number(input.value)}).then(function () { status(section, "Saved"); }).catch(function () { status(section, "Couldn’t save vibration."); }); }); });
        workspace.querySelectorAll("[data-lf-controller-thumbstick]").forEach(function (row) { row.querySelectorAll("[data-lf-controller-sensitivity-x], [data-lf-controller-sensitivity-y]").forEach(bindRange); row.querySelector("[data-lf-controller-thumbstick-save]").addEventListener("click", function () { post(browser, "/api/controller/emulation", {deviceId: deviceId, emulationDevice: Number(row.dataset.lfModule), emulationMode: number(row, "[data-lf-controller-mode]"), sensitivityX: number(row, "[data-lf-controller-sensitivity-x]"), sensitivityY: number(row, "[data-lf-controller-sensitivity-y]"), invertYAxis: row.querySelector("[data-lf-controller-invert-y]").checked}).then(function () { status(row, "Saved"); }).catch(function () { status(row, "Couldn’t save thumbstick settings."); }); }); });
        workspace.querySelectorAll("[data-lf-controller-assignment]").forEach(function (row) { row.querySelector("[data-lf-controller-assignment-save]").addEventListener("click", function () { post(browser, "/api/controller/updateKeyAssignment", {deviceId: deviceId, keyIndex: Number(row.dataset.lfKeyIndex), enabled: row.querySelector("[data-lf-controller-default]").checked, pressAndHold: row.querySelector("[data-lf-controller-hold]").checked, keyAssignmentType: number(row, "[data-lf-controller-assignment-type]"), keyAssignmentValue: number(row, "[data-lf-controller-assignment-command]")}).then(function () { status(row, "Saved"); }).catch(function () { status(row, "Couldn’t save assignment."); }); }); });
        workspace.querySelectorAll("[data-lf-controller-analog]").forEach(function (row) { const points = curve(row.querySelector("[data-lf-controller-curve]"), row); bindDisclosureRedraw(row, points.redraw); row.querySelectorAll("[data-lf-controller-deadzone-min], [data-lf-controller-deadzone-max]").forEach(bindRange); row.querySelector("[data-lf-controller-analog-save]").addEventListener("click", function () { post(browser, "/api/controller/setGraph", {deviceId: deviceId, analogDevice: Number(row.dataset.lfAnalogDevice), deadZoneMin: number(row, "[data-lf-controller-deadzone-min]"), deadZoneMax: number(row, "[data-lf-controller-deadzone-max]"), curveData: points.map(function (point) { return {x: point.x, y: point.y}; })}).then(function () { status(row, "Saved"); }).catch(function () { status(row, "Couldn’t save analog curve."); }); }); });
    }
    return {init: init, curve: curve, canvasPosition: canvasPosition, bindDisclosureRedraw: bindDisclosureRedraw};
});
