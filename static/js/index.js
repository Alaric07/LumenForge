"use strict";

const dashboardDevicePresentation = (function () {
    const deviceURL = (serial) => "/devices?device=" + encodeURIComponent(serial);
    const cardID = (source, serial) => source + ":" + serial;
    function normalize(response) { return {native: Array.isArray(response?.native) ? response.native : [], openrgb: Array.isArray(response?.openrgb) ? response.openrgb : [], memory: Array.isArray(response?.memory) ? response.memory : []}; }
    function isDenseCard(card) { return card?.source === "native" && Array.isArray(card.statusRows) && card.statusRows.length >= 4; }
    function normalizeLayout(layout) {
        const used = new Map(), ids = new Set();
        return (Array.isArray(layout) ? layout : []).reduce(function (items, input) {
            const column = Number.isInteger(input?.column) ? input.column : input?.x, order = Number.isInteger(input?.order) ? input.order : input?.y;
            if (!input || typeof input.id !== "string" || ids.has(input.id) || !Number.isInteger(column) || !Number.isInteger(order) || column < 0 || order < 0 || column > 10000 || order > 10000) return items;
            const orders = used.get(column) || new Set(); let normalizedOrder = order; while (orders.has(normalizedOrder)) normalizedOrder++;
            used.set(column, orders); orders.add(normalizedOrder); ids.add(input.id); items.push({id: input.id, column: column, order: normalizedOrder}); return items;
        }, []);
    }
    function completeLayout(saved, cards) {
        const savedItems = Array.isArray(saved) ? saved : [], known = new Set(savedItems.map((item) => item?.id));
        const nextColumn = normalizeLayout(savedItems).reduce((maximum, item) => Math.max(maximum, item.column), -1) + 1;
        return normalizeLayout([...savedItems, ...cards.filter((card) => !known.has(card.id)).map((card, index) => ({id: card.id, column: nextColumn + index, order: 0}))]);
    }
    function laneColumns(saved, cards) {
        const complete = completeLayout(saved, cards), highest = complete.reduce((maximum, item) => Math.max(maximum, item.column), -1);
        return Array.from({length: highest + 2}, (_, column) => column);
    }
    function reconcileLayout(saved, cards) {
        const byID = new Map(completeLayout(saved, cards).map((item) => [item.id, item])), visible = [];
        cards.forEach(function (card, index) {
            const item = byID.get(card.id) || {id: card.id, column: index, order: 0};
            visible.push(Object.assign({}, card, item));
        });
        return visible.sort((left, right) => left.column - right.column || left.order - right.order || left.id.localeCompare(right.id));
    }
    function moveLayout(saved, cards, id, destination) {
        const complete = completeLayout(saved, cards), current = complete.find((item) => item.id === id); if (!current) return complete;
        const targetColumn = Number.isInteger(destination?.column) ? destination.column : current.column, targetOrder = Number.isInteger(destination?.order) ? destination.order : current.order;
        const without = complete.filter((item) => item.id !== id), target = without.filter((item) => item.column === targetColumn).sort((left, right) => left.order - right.order || left.id.localeCompare(right.id));
        target.splice(Math.max(0, Math.min(targetOrder, target.length)), 0, {id: id, column: targetColumn, order: 0});
        if (current.column === targetColumn) return without.filter((item) => item.column !== targetColumn).concat(target.map((item, order) => Object.assign({}, item, {column: targetColumn, order: order})));
        const source = without.filter((item) => item.column === current.column).sort((left, right) => left.order - right.order || left.id.localeCompare(right.id));
        const unaffected = without.filter((item) => item.column !== targetColumn && item.column !== current.column);
        return unaffected.concat(source.map((item, order) => Object.assign({}, item, {column: current.column, order: order})), target.map((item, order) => Object.assign({}, item, {column: targetColumn, order: order})));
    }
    function cloneLayout(layout) { return layout.map((item) => Object.assign({}, item)); }
    function createLayoutWriteQueue(send, accept) {
        let latestRevision = 0, queue = Promise.resolve();
        return function enqueue(layout) {
            const snapshot = cloneLayout(layout), revision = ++latestRevision;
            queue = queue.then(function () { return Promise.resolve(send(snapshot)).then(function (response) { if (revision === latestRevision) accept(response, snapshot); }, function () {}); });
            return queue;
        };
    }
    return {deviceURL, cardID, normalize, isDenseCard, normalizeLayout, completeLayout, laneColumns, reconcileLayout, moveLayout, createLayoutWriteQueue};
})();
if (typeof module === "object" && module.exports) module.exports = dashboardDevicePresentation;

if (typeof window !== "undefined" && window.document) {
$(document).ready(function () {
    const i18n = window.dashboardI18n || {}, grid = $("[data-lf-dashboard-device-grid]"), section = $("[data-lf-dashboard-devices]");
    let layoutState = [], currentCards = [], dragging = false, dragState = null, deferredDeviceResponse = null, deferredLayoutResponse = null;
    function updateLightingStatus() { if (!$("#lighting-cluster-effect").length) return; $.ajax({url: "/api/dashboard/lighting", type: "GET", dataType: "json", success: function (data) { $("#lighting-cluster-effect").text(data.effect || "off"); $("#lighting-clustered-devices").text(data.clusteredLightingDevices || 0); $("#lighting-independent-devices").text(data.independentLightingDevices || 0); $("#lighting-brightness").text((data.brightness ?? 0) + "%"); }}); }
    function cardContent(card) {
        const link = $("<a>", {class: "lf-dashboard-device-card", href: dashboardDevicePresentation.deviceURL(card.serial)});
        if (card.source === "openrgb") { link.addClass("lf-dashboard-openrgb-row"); link.append($("<span>", {class: "lf-dashboard-openrgb-name", text: card.name})); const state = $("<span>", {class: "lf-dashboard-openrgb-state"}).append($("<span>", {text: card.lighting || i18n.lighting})); if (Number.isFinite(card.brightness)) state.append($("<small>", {text: card.brightness + "%"})); return link.append(state, $("<span>", {class: "lf-dashboard-openrgb-arrow", text: "›", "aria-hidden": "true"})); }
        link.append($("<h3>", {class: "lf-dashboard-device-title", text: card.name})); if (card.product && card.product !== card.name) link.append($("<p>", {class: "lf-dashboard-device-product", text: card.product})); if (card.lighting) link.append($("<div>", {class: "lf-dashboard-device-state"}).append($("<span>", {text: i18n.lighting}), $("<strong>", {text: card.lighting}))); if (Number.isFinite(card.brightness)) link.append($("<div>", {class: "lf-dashboard-device-status"}).append($("<span>", {text: i18n.brightness}), $("<strong>", {text: card.brightness + "%"})));
        (card.statusRows || []).forEach((row) => { if (row?.label && row?.value) link.append($("<div>", {class: "lf-dashboard-device-status"}).append($("<span>", {text: row.label}), $("<strong>", {text: row.value}))); });
        const telemetry = card.telemetry, history = window.dashboardHistory;
        if (card.source === "native" && telemetry && history) {
            const area = $("<div>", {class: "lf-dashboard-history"});
            const addRow = function (label, value, key) { const row = $("<div>", {class: "lf-dashboard-history-row"}).append($("<span>", {text: label}), $("<strong>", {text: value}), history.createSparkline(window.document, key)); area.append(row); };
            if (Number.isFinite(telemetry.averageFanRPM)) addRow("Average Fan Speed", telemetry.averageFanRPM + " RPM", history.keys.fanAverage(card.serial), "rpm");
            if (Number.isFinite(telemetry.coolantCelsius)) addRow("Coolant", history.temperatureText(telemetry.coolantCelsius), history.keys.coolant(card.serial), "temperature");
            (telemetry.temperatureProbes || []).forEach(function (probe) { if (Number.isFinite(probe?.celsius)) addRow(probe.label, history.temperatureText(probe.celsius), history.keys.probe(card.serial, probe.id), "temperature"); });
            if (area.children().length) link.append(area);
        }
        return link;
    }
    const enqueueLayoutWrite = dashboardDevicePresentation.createLayoutWriteQueue(function (snapshot) { return new Promise(function (resolve) {
        $.ajax({url: "/api/dashboard/layout", type: "PUT", contentType: "application/json", data: JSON.stringify({layout: snapshot}), success: resolve, error: function () { resolve(null); }});
    }); }, function (response) {
        if (response?.status === 1 && Array.isArray(response.data?.layout)) { if (dragState) { deferredLayoutResponse = response.data.layout; return; } layoutState = dashboardDevicePresentation.completeLayout(response.data.layout, currentCards); renderCards(); }
    });
    function persistLayout() { return enqueueLayoutWrite(dashboardDevicePresentation.completeLayout(layoutState, currentCards)); }
    function moveCard(id, destination) { layoutState = dashboardDevicePresentation.moveLayout(layoutState, currentCards, id, destination); renderCards(); persistLayout(); }
    function cleanupDrag() {
        const state = dragState; if (!state) return;
        dragState = null; dragging = false;
        state.handle.removeEventListener("pointermove", state.onMove); state.handle.removeEventListener("pointerup", state.onPointerUp); state.handle.removeEventListener("pointercancel", state.onPointerCancel); state.handle.removeEventListener("lostpointercapture", state.onLostPointerCapture); document.removeEventListener("keydown", state.onKeyDown);
        grid.find(".lf-dashboard-lane-drag-target").removeClass("lf-dashboard-lane-drag-target"); grid.find(".lf-dashboard-lane-insertion").remove(); state.wrapper.classList.remove("lf-dashboard-card-dragging"); document.body.classList.remove("lf-dashboard-drag-active"); if (state.ghost) state.ghost.remove();
        try { if (state.handle.hasPointerCapture(state.pointerID)) state.handle.releasePointerCapture(state.pointerID); } catch (_) {}
    }
    function flushDeferredPresentation() {
        if (deferredLayoutResponse) { layoutState = dashboardDevicePresentation.completeLayout(deferredLayoutResponse, currentCards); deferredLayoutResponse = null; renderCards(); }
        if (deferredDeviceResponse) { const response = deferredDeviceResponse; deferredDeviceResponse = null; renderCurrentDevices(response); }
    }
    function createDragGhost(wrapper, sourceRect, offset, event) {
        const ghost = wrapper.cloneNode(true);
        ghost.classList.remove("lf-dashboard-card-dragging", "lf-dashboard-card-drag-target");
        ghost.classList.add("lf-dashboard-drag-ghost");
        ghost.removeAttribute("data-lf-dashboard-card-id");
        ghost.setAttribute("aria-hidden", "true");
        ghost.querySelectorAll("a, button, [tabindex]").forEach(function (element) {
            element.setAttribute("tabindex", "-1");
            element.setAttribute("aria-hidden", "true");
            if ("disabled" in element) element.disabled = true;
        });
        ghost.style.width = sourceRect.width + "px";
        document.body.appendChild(ghost);
        positionDragGhost(ghost, offset, event);
        return ghost;
    }
    function positionDragGhost(ghost, offset, event) {
        ghost.style.left = (event.clientX - offset.x) + "px";
        ghost.style.top = (event.clientY - offset.y) + "px";
    }
    function bindDrag(wrapper) {
        const handle = wrapper.querySelector("[data-lf-dashboard-drag-handle]"); if (!handle) return;
        handle.addEventListener("pointerdown", function (event) { if (dragState || event.button !== 0 || window.matchMedia("(max-width: 560px)").matches) return; const id = wrapper.dataset.lfDashboardCardId, sourceRect = wrapper.getBoundingClientRect(), offset = {x: event.clientX - sourceRect.left, y: event.clientY - sourceRect.top}; let dragStarted = false, finished = false, ghost = null;
            handle.setPointerCapture(event.pointerId);
            const destinationAt = function (moveEvent) { const lanes = Array.from(grid[0].querySelectorAll("[data-lf-dashboard-lane]")); const lane = lanes.reduce((nearest, candidate) => { const rect = candidate.getBoundingClientRect(), distance = Math.abs(moveEvent.clientX - (rect.left + rect.width / 2)); return !nearest || distance < nearest.distance ? {candidate, distance} : nearest; }, null)?.candidate; if (!lane) return null; const cards = Array.from(lane.querySelectorAll("[data-lf-dashboard-card-id]")).filter((candidate) => candidate !== wrapper); const order = cards.findIndex((candidate) => moveEvent.clientY < candidate.getBoundingClientRect().top + candidate.getBoundingClientRect().height / 2); return {column: Number(lane.dataset.lfDashboardLane), order: order < 0 ? cards.length : order, lane: lane}; };
            const updateTarget = function (moveEvent) { const target = destinationAt(moveEvent); grid.find(".lf-dashboard-lane-drag-target").removeClass("lf-dashboard-lane-drag-target"); grid.find(".lf-dashboard-lane-insertion").remove(); if (target) { target.lane.classList.add("lf-dashboard-lane-drag-target"); const marker = document.createElement("div"), cards = Array.from(target.lane.querySelectorAll("[data-lf-dashboard-card-id]")).filter((candidate) => candidate !== wrapper); marker.className = "lf-dashboard-lane-insertion"; target.lane.insertBefore(marker, cards[target.order] || null); } };
            const finish = function (moveEvent, cancelled) { if (finished) return; finished = true; const target = !cancelled && dragStarted ? destinationAt(moveEvent) : null; cleanupDrag(); try { if (target) { deferredLayoutResponse = null; moveCard(id, target); } } finally { flushDeferredPresentation(); } };
            const onMove = function (moveEvent) { const moved = Math.abs(moveEvent.clientX - event.clientX) + Math.abs(moveEvent.clientY - event.clientY); if (!dragStarted && moved > 4) { dragStarted = true; dragging = true; document.body.classList.add("lf-dashboard-drag-active"); wrapper.classList.add("lf-dashboard-card-dragging"); ghost = createDragGhost(wrapper, sourceRect, offset, moveEvent); dragState.ghost = ghost; } if (!dragStarted) return; moveEvent.preventDefault(); positionDragGhost(ghost, offset, moveEvent); updateTarget(moveEvent); };
            const onPointerUp = (moveEvent) => finish(moveEvent, false);
            const onPointerCancel = (moveEvent) => finish(moveEvent, true);
            const onLostPointerCapture = () => finish(event, true);
            const onKeyDown = (keyEvent) => { if (keyEvent.key === "Escape") finish(event, true); };
            dragState = {handle: handle, pointerID: event.pointerId, wrapper: wrapper, ghost: ghost, onMove: onMove, onPointerUp: onPointerUp, onPointerCancel: onPointerCancel, onLostPointerCapture: onLostPointerCapture, onKeyDown: onKeyDown}; handle.addEventListener("pointermove", onMove); handle.addEventListener("pointerup", onPointerUp); handle.addEventListener("pointercancel", onPointerCancel); handle.addEventListener("lostpointercapture", onLostPointerCapture); document.addEventListener("keydown", onKeyDown);
        });
    }
    function renderCards() {
        if (dragging || !grid.length) return; const ordered = dashboardDevicePresentation.reconcileLayout(layoutState, currentCards), visibleIDs = new Set(ordered.map((card) => card.id)), existing = new Map(); grid.find("[data-lf-dashboard-card-id]").each(function () { if (visibleIDs.has(this.dataset.lfDashboardCardId)) existing.set(this.dataset.lfDashboardCardId, this); }); grid.empty();
        const denseColumns = new Set(ordered.filter(dashboardDevicePresentation.isDenseCard).map((card) => card.column)), lanes = new Map(), addLane = function (column, dense) { const lane = document.createElement("div"); lane.className = "lf-dashboard-device-lane" + (dense ? " lf-dashboard-device-lane-dense" : ""); lane.dataset.lfDashboardLane = column; lanes.set(column, lane); grid.append(lane); return lane; }; dashboardDevicePresentation.laneColumns(layoutState, currentCards).forEach((column) => addLane(column, denseColumns.has(column))); ordered.forEach(function (card) { let wrapper = existing.get(card.id); if (!wrapper) { wrapper = document.createElement("article"); wrapper.className = "lf-dashboard-layout-card"; wrapper.dataset.lfDashboardCardId = card.id; wrapper.innerHTML = '<div class="lf-dashboard-card-content"></div><div class="lf-dashboard-card-actions"><button type="button" data-lf-dashboard-drag-handle aria-label="' + i18n.drag + '">⠿</button><button type="button" data-lf-dashboard-move="up" aria-label="Move up">↑</button><button type="button" data-lf-dashboard-move="down" aria-label="Move down">↓</button><button type="button" data-lf-dashboard-move="left" aria-label="Move left">←</button><button type="button" data-lf-dashboard-move="right" aria-label="Move right">→</button></div>'; ["up", "down", "left", "right"].forEach((direction) => wrapper.querySelector('[data-lf-dashboard-move="' + direction + '"]').addEventListener("click", () => { const current = dashboardDevicePresentation.reconcileLayout(layoutState, currentCards).find((item) => item.id === card.id); if (!current) return; const destination = direction === "up" ? {column: current.column, order: current.order - 1} : direction === "down" ? {column: current.column, order: current.order + 1} : {column: Math.max(0, current.column + (direction === "left" ? -1 : 1)), order: current.order}; moveCard(card.id, destination); })); bindDrag(wrapper); } $(wrapper).find(".lf-dashboard-card-content").empty().append(cardContent(card)); lanes.get(card.column).append(wrapper); }); const laneHeight = Math.max(...Array.from(lanes.values(), (lane) => lane.scrollHeight)); lanes.forEach((lane) => { lane.style.minHeight = laneHeight + "px"; }); section.prop("hidden", ordered.length === 0);
    }
    function renderCurrentDevices(response) { if (dragState) { deferredDeviceResponse = response; return; } const devices = dashboardDevicePresentation.normalize(response), history = window.dashboardHistory; if (window.dashboardTelemetry) devices.memory.forEach((memory) => window.dashboardTelemetry.updateMemory(window.document, memory)); if (history) devices.native.forEach(function (device) { const telemetry = device.telemetry || {}; history.append(history.keys.fanAverage(device.serial), telemetry.averageFanRPM); history.append(history.keys.coolant(device.serial), telemetry.coolantCelsius); (telemetry.temperatureProbes || []).forEach(function (probe) { history.append(history.keys.probe(device.serial, probe.id), probe.celsius); }); }); currentCards = devices.native.map((device) => Object.assign({source: "native", id: dashboardDevicePresentation.cardID("native", device.serial)}, device)).concat(devices.openrgb.map((device) => Object.assign({source: "openrgb", id: dashboardDevicePresentation.cardID("openrgb", device.serial)}, device))); layoutState = dashboardDevicePresentation.completeLayout(layoutState, currentCards); renderCards(); if (history) devices.native.forEach(function (device) { const telemetry = device.telemetry || {}; history.render(window.document, history.keys.fanAverage(device.serial), "rpm"); history.render(window.document, history.keys.coolant(device.serial), "temperature"); (telemetry.temperatureProbes || []).forEach(function (probe) { history.render(window.document, history.keys.probe(device.serial, probe.id), "temperature"); }); }); }
    function updateCurrentDevices() { $.ajax({url: "/api/dashboard/devices/current", type: "GET", dataType: "json", success: renderCurrentDevices}); }
    function refreshDashboard() { updateLightingStatus(); updateCurrentDevices(); }
    $.ajax({url: "/api/dashboard/layout", type: "GET", dataType: "json", success: function (response) { if (dragState) deferredLayoutResponse = response.layout; else layoutState = dashboardDevicePresentation.completeLayout(response.layout, currentCards); refreshDashboard(); }, error: refreshDashboard}); window.setInterval(refreshDashboard, 3000);
});
}
