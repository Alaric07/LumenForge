"use strict";

const dashboardDevicePresentation = (function () {
    function deviceURL(serial) {
        return "/devices?device=" + encodeURIComponent(serial);
    }

    function normalize(response) {
        return {
            native: Array.isArray(response?.native) ? response.native : [],
            openrgb: Array.isArray(response?.openrgb) ? response.openrgb : [],
            memory: Array.isArray(response?.memory) ? response.memory : []
        };
    }

    return {deviceURL, normalize};
})();

if (typeof module === "object" && module.exports) {
    module.exports = dashboardDevicePresentation;
}

if (typeof window !== "undefined" && window.document) {
$(document).ready(function () {
    const dashboardI18n = window.dashboardI18n || {};

    function updateLightingStatus() {
        if ($("#lighting-cluster-effect").length === 0) return;
        $.ajax({
            url: "/api/dashboard/lighting",
            type: "GET",
            dataType: "json",
            success: function (data) {
                $("#lighting-cluster-effect").text(data.effect || "off");
                $("#lighting-clustered-devices").text(data.clusteredLightingDevices || 0);
                $("#lighting-independent-devices").text(data.independentLightingDevices || 0);
                $("#lighting-brightness").text((data.brightness ?? 0) + "%");
            },
            error: function (err) {
                console.error("Failed to fetch lighting status:", err);
            }
        });
    }

    function deviceCard(device) {
        const card = $("<a>", {class: "lf-dashboard-device-card", href: dashboardDevicePresentation.deviceURL(device.serial)});
        card.append($("<h3>", {class: "lf-dashboard-device-title", text: device.name}));
        if (device.product && device.product !== device.name) {
            card.append($("<p>", {class: "lf-dashboard-device-product", text: device.product}));
        }
        if (device.lighting) {
            const state = $("<div>", {class: "lf-dashboard-device-state"});
            state.append($("<span>", {text: dashboardI18n.lighting}), $("<strong>", {text: device.lighting}));
            card.append(state);
        }
        if (Number.isFinite(device.brightness)) {
            const brightness = $("<div>", {class: "lf-dashboard-device-status"});
            brightness.append($("<span>", {text: dashboardI18n.brightness}), $("<strong>", {text: device.brightness + "%"}));
            card.append(brightness);
        }
        (device.statusRows || []).forEach(function (row) {
            if (!row?.label || !row?.value) return;
            const status = $("<div>", {class: "lf-dashboard-device-status"});
            status.append($("<span>", {text: row.label}), $("<strong>", {text: row.value}));
            card.append(status);
        });
        return card;
    }

    function openRGBRow(device) {
        const row = $("<a>", {class: "lf-dashboard-openrgb-row", href: dashboardDevicePresentation.deviceURL(device.serial)});
        row.append($("<span>", {class: "lf-dashboard-openrgb-name", text: device.name}));
        const state = $("<span>", {class: "lf-dashboard-openrgb-state"});
        state.append($("<span>", {text: device.lighting || dashboardI18n.lighting}));
        if (Number.isFinite(device.brightness)) {
            state.append($("<small>", {text: device.brightness + "%"}));
        }
        row.append(state);
        row.append($("<span>", {class: "lf-dashboard-openrgb-arrow", text: "›", "aria-hidden": "true"}));
        return row;
    }

    function renderCurrentDevices(response) {
        const devices = dashboardDevicePresentation.normalize(response);
        if (window.dashboardTelemetry) {
            devices.memory.forEach(function (memory) {
                window.dashboardTelemetry.updateMemory(window.document, memory);
            });
        }
        const nativeSection = $("[data-lf-dashboard-devices]");
        const nativeGrid = $("[data-lf-dashboard-native-devices]");
        nativeGrid.empty();
        devices.native.forEach(function (device) {
            nativeGrid.append(deviceCard(device));
        });
        nativeSection.prop("hidden", devices.native.length === 0);

        const openRGBSection = $("[data-lf-dashboard-openrgb-devices]");
        const openRGBList = $("[data-lf-dashboard-openrgb-list]");
        openRGBList.empty();
        devices.openrgb.forEach(function (device) {
            openRGBList.append(openRGBRow(device));
        });
        openRGBSection.prop("hidden", devices.openrgb.length === 0);
    }

    function updateCurrentDevices() {
        $.ajax({
            url: "/api/dashboard/devices/current",
            type: "GET",
            dataType: "json",
            success: renderCurrentDevices,
            error: function (err) {
                console.error("Failed to fetch current Dashboard devices:", err);
            }
        });
    }

    function refreshDashboard() {
        updateLightingStatus();
        updateCurrentDevices();
    }

    refreshDashboard();
    window.setInterval(refreshDashboard, 3000);
});
}
