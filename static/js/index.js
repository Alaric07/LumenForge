"use strict";

const dashboardDevicePresentation = (function () {
    const layoutColumns = 4;
    const deviceURL = (serial) => "/devices?device=" + encodeURIComponent(serial);
    const cardID = (source, serial) => source + ":" + serial;
    function normalize(response) { return {native: Array.isArray(response?.native) ? response.native : [], openrgb: Array.isArray(response?.openrgb) ? response.openrgb : [], memory: Array.isArray(response?.memory) ? response.memory : []}; }
    function normalizeLayout(layout) {
        const used = new Set(), ids = new Set();
        return (Array.isArray(layout) ? layout : []).reduce(function (items, item) {
            if (!item || typeof item.id !== "string" || ids.has(item.id) || !Number.isInteger(item.x) || !Number.isInteger(item.y) || item.x < 0 || item.x >= layoutColumns || item.y < 0 || item.w !== 1 || item.h !== 1) return items;
            let x = item.x, y = item.y;
            while (used.has(x + ":" + y)) { x = (x + 1) % layoutColumns; if (x === 0) y++; }
            ids.add(item.id); used.add(x + ":" + y); items.push({id: item.id, x: x, y: y, w: 1, h: 1}); return items;
        }, []);
    }
    function completeLayout(saved, cards) {
        const complete = normalizeLayout(saved), ids = new Set(complete.map((item) => item.id)), used = new Set(complete.map((item) => item.x + ":" + item.y));
        cards.forEach(function (card) {
            if (ids.has(card.id)) return;
            let x = 0, y = 0;
            while (used.has(x + ":" + y)) { x = (x + 1) % layoutColumns; if (x === 0) y++; }
            complete.push({id: card.id, x: x, y: y, w: 1, h: 1}); ids.add(card.id); used.add(x + ":" + y);
        });
        return complete;
    }
    function reconcileLayout(saved, cards) {
        const byID = new Map(completeLayout(saved, cards).map((item) => [item.id, item])), visible = [];
        cards.forEach(function (card, index) {
            const item = byID.get(card.id) || {id: card.id, x: index % layoutColumns, y: Math.floor(index / layoutColumns), w: 1, h: 1};
            visible.push(Object.assign({}, card, {x: item.x, y: item.y, w: 1, h: 1}));
        });
        return visible.sort((left, right) => left.y - right.y || left.x - right.x || left.id.localeCompare(right.id));
    }
    function moveLayout(saved, cards, id, destination) {
        const complete = completeLayout(saved, cards), visibleIDs = new Set(cards.map((card) => card.id));
        const visible = complete.filter((item) => visibleIDs.has(item.id)).sort((left, right) => left.y - right.y || left.x - right.x || left.id.localeCompare(right.id));
        const index = visible.findIndex((item) => item.id === id);
        if (index < 0 || destination < 0 || destination >= visible.length) return complete;
        const slots = visible.map((item) => ({x: item.x, y: item.y}));
        visible.splice(destination, 0, visible.splice(index, 1)[0]);
        const positions = new Map(visible.map((item, position) => [item.id, slots[position]]));
        return complete.map((item) => positions.has(item.id) ? Object.assign({}, item, positions.get(item.id)) : item);
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
    return {deviceURL, cardID, normalize, normalizeLayout, completeLayout, reconcileLayout, moveLayout, createLayoutWriteQueue};
})();
if (typeof module === "object" && module.exports) module.exports = dashboardDevicePresentation;

if (typeof window !== "undefined" && window.document) {
$(document).ready(function () {
    const i18n = window.dashboardI18n || {}, grid = $("[data-lf-dashboard-device-grid]"), section = $("[data-lf-dashboard-devices]");
    let layoutState = [], currentCards = [], dragging = false;
    function updateLightingStatus() { if (!$("#lighting-cluster-effect").length) return; $.ajax({url: "/api/dashboard/lighting", type: "GET", dataType: "json", success: function (data) { $("#lighting-cluster-effect").text(data.effect || "off"); $("#lighting-clustered-devices").text(data.clusteredLightingDevices || 0); $("#lighting-independent-devices").text(data.independentLightingDevices || 0); $("#lighting-brightness").text((data.brightness ?? 0) + "%"); }}); }
    function cardContent(card) {
        const link = $("<a>", {class: "lf-dashboard-device-card", href: dashboardDevicePresentation.deviceURL(card.serial)});
        if (card.source === "openrgb") { link.addClass("lf-dashboard-openrgb-row"); link.append($("<span>", {class: "lf-dashboard-openrgb-name", text: card.name})); const state = $("<span>", {class: "lf-dashboard-openrgb-state"}).append($("<span>", {text: card.lighting || i18n.lighting})); if (Number.isFinite(card.brightness)) state.append($("<small>", {text: card.brightness + "%"})); return link.append(state, $("<span>", {class: "lf-dashboard-openrgb-arrow", text: "›", "aria-hidden": "true"})); }
        link.append($("<h3>", {class: "lf-dashboard-device-title", text: card.name})); if (card.product && card.product !== card.name) link.append($("<p>", {class: "lf-dashboard-device-product", text: card.product})); if (card.lighting) link.append($("<div>", {class: "lf-dashboard-device-state"}).append($("<span>", {text: i18n.lighting}), $("<strong>", {text: card.lighting}))); if (Number.isFinite(card.brightness)) link.append($("<div>", {class: "lf-dashboard-device-status"}).append($("<span>", {text: i18n.brightness}), $("<strong>", {text: card.brightness + "%"})));
        (card.statusRows || []).forEach((row) => { if (row?.label && row?.value) link.append($("<div>", {class: "lf-dashboard-device-status"}).append($("<span>", {text: row.label}), $("<strong>", {text: row.value}))); }); return link;
    }
    const enqueueLayoutWrite = dashboardDevicePresentation.createLayoutWriteQueue(function (snapshot) { return new Promise(function (resolve) {
        $.ajax({url: "/api/dashboard/layout", type: "PUT", contentType: "application/json", data: JSON.stringify({layout: snapshot}), success: resolve, error: function () { resolve(null); }});
    }); }, function (response) {
        if (response?.status === 1 && Array.isArray(response.data?.layout)) { layoutState = dashboardDevicePresentation.completeLayout(response.data.layout, currentCards); renderCards(); }
    });
    function persistLayout() { return enqueueLayoutWrite(dashboardDevicePresentation.completeLayout(layoutState, currentCards)); }
    function moveCard(id, destination) {
        const ordered = dashboardDevicePresentation.reconcileLayout(layoutState, currentCards), index = ordered.findIndex((card) => card.id === id); if (index < 0 || destination < 0 || destination >= ordered.length) return;
        layoutState = dashboardDevicePresentation.moveLayout(layoutState, currentCards, id, destination); renderCards(); persistLayout();
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
        handle.addEventListener("pointerdown", function (event) { if (event.button !== 0 || window.matchMedia("(max-width: 560px)").matches) return; const id = wrapper.dataset.lfDashboardCardId, sourceRect = wrapper.getBoundingClientRect(), offset = {x: event.clientX - sourceRect.left, y: event.clientY - sourceRect.top}; let dragStarted = false, ghost = null;
            handle.setPointerCapture(event.pointerId);
            const nearestTarget = function (moveEvent) { const targets = Array.from(grid[0].querySelectorAll("[data-lf-dashboard-card-id]")); return targets.reduce((nearest, candidate) => { const rect = candidate.getBoundingClientRect(), distance = Math.hypot(moveEvent.clientX - (rect.left + rect.width / 2), moveEvent.clientY - (rect.top + rect.height / 2)); return candidate === wrapper || (nearest && distance >= nearest.distance) ? nearest : {candidate, distance}; }, null); };
            const updateTarget = function (moveEvent) { const target = nearestTarget(moveEvent); grid.find(".lf-dashboard-card-drag-target").removeClass("lf-dashboard-card-drag-target"); if (target) target.candidate.classList.add("lf-dashboard-card-drag-target"); };
            const cleanup = function () { handle.removeEventListener("pointermove", onMove); handle.removeEventListener("pointerup", onPointerUp); handle.removeEventListener("pointercancel", onPointerCancel); document.removeEventListener("keydown", onKeyDown); grid.find(".lf-dashboard-card-drag-target").removeClass("lf-dashboard-card-drag-target"); wrapper.classList.remove("lf-dashboard-card-dragging"); if (ghost) ghost.remove(); dragging = false; if (handle.hasPointerCapture(event.pointerId)) handle.releasePointerCapture(event.pointerId); };
            const finish = function (moveEvent, cancelled) { const target = !cancelled && dragStarted ? nearestTarget(moveEvent) : null; cleanup(); if (target && target.candidate.dataset.lfDashboardCardId !== id) { const ordered = dashboardDevicePresentation.reconcileLayout(layoutState, currentCards); moveCard(id, ordered.findIndex((card) => card.id === target.candidate.dataset.lfDashboardCardId)); } };
            const onMove = function (moveEvent) { const moved = Math.abs(moveEvent.clientX - event.clientX) + Math.abs(moveEvent.clientY - event.clientY); if (!dragStarted && moved > 4) { dragStarted = true; dragging = true; wrapper.classList.add("lf-dashboard-card-dragging"); ghost = createDragGhost(wrapper, sourceRect, offset, moveEvent); } if (!dragStarted) return; moveEvent.preventDefault(); positionDragGhost(ghost, offset, moveEvent); updateTarget(moveEvent); };
            const onPointerUp = (moveEvent) => finish(moveEvent, false);
            const onPointerCancel = (moveEvent) => finish(moveEvent, true);
            const onKeyDown = (keyEvent) => { if (keyEvent.key === "Escape") finish(event, true); };
            handle.addEventListener("pointermove", onMove); handle.addEventListener("pointerup", onPointerUp); handle.addEventListener("pointercancel", onPointerCancel); document.addEventListener("keydown", onKeyDown);
        });
    }
    function renderCards() {
        if (dragging || !grid.length) return; const ordered = dashboardDevicePresentation.reconcileLayout(layoutState, currentCards), visibleIDs = new Set(ordered.map((card) => card.id)); grid.children("[data-lf-dashboard-card-id]").each(function () { if (!visibleIDs.has(this.dataset.lfDashboardCardId)) this.remove(); });
        ordered.forEach(function (card) { let wrapper = grid[0].querySelector('[data-lf-dashboard-card-id="' + CSS.escape(card.id) + '"]'); if (!wrapper) { wrapper = document.createElement("article"); wrapper.className = "lf-dashboard-layout-card"; wrapper.dataset.lfDashboardCardId = card.id; wrapper.innerHTML = '<div class="lf-dashboard-card-content"></div><div class="lf-dashboard-card-actions"><button type="button" data-lf-dashboard-drag-handle aria-label="' + i18n.drag + '">⠿</button><button type="button" data-lf-dashboard-move="earlier" aria-label="' + i18n.moveEarlier + '">←</button><button type="button" data-lf-dashboard-move="later" aria-label="' + i18n.moveLater + '">→</button></div>'; wrapper.querySelector('[data-lf-dashboard-move="earlier"]').addEventListener("click", () => { const position = dashboardDevicePresentation.reconcileLayout(layoutState, currentCards).findIndex((item) => item.id === card.id); moveCard(card.id, position - 1); }); wrapper.querySelector('[data-lf-dashboard-move="later"]').addEventListener("click", () => { const position = dashboardDevicePresentation.reconcileLayout(layoutState, currentCards).findIndex((item) => item.id === card.id); moveCard(card.id, position + 1); }); bindDrag(wrapper); } $(wrapper).find(".lf-dashboard-card-content").empty().append(cardContent(card)); grid.append(wrapper); }); section.prop("hidden", ordered.length === 0);
    }
    function renderCurrentDevices(response) { const devices = dashboardDevicePresentation.normalize(response); if (window.dashboardTelemetry) devices.memory.forEach((memory) => window.dashboardTelemetry.updateMemory(window.document, memory)); currentCards = devices.native.map((device) => Object.assign({source: "native", id: dashboardDevicePresentation.cardID("native", device.serial)}, device)).concat(devices.openrgb.map((device) => Object.assign({source: "openrgb", id: dashboardDevicePresentation.cardID("openrgb", device.serial)}, device))); layoutState = dashboardDevicePresentation.completeLayout(layoutState, currentCards); renderCards(); }
    function updateCurrentDevices() { $.ajax({url: "/api/dashboard/devices/current", type: "GET", dataType: "json", success: renderCurrentDevices}); }
    function refreshDashboard() { updateLightingStatus(); updateCurrentDevices(); }
    $.ajax({url: "/api/dashboard/layout", type: "GET", dataType: "json", success: function (response) { layoutState = dashboardDevicePresentation.completeLayout(response.layout, currentCards); refreshDashboard(); }, error: refreshDashboard}); window.setInterval(refreshDashboard, 3000);
});
}
