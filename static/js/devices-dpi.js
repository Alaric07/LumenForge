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

    function init(browser) {
        const workspace = browser.document.querySelector("[data-lf-dpi-workspace]");
        if (!workspace) { return; }
        const minimum = Number(workspace.dataset.lfMinimum);
        const maximum = Number(workspace.dataset.lfMaximum);
        const status = workspace.querySelector("[data-lf-dpi-status]");
        const save = workspace.querySelector("[data-lf-dpi-save]");
        if (!Number.isInteger(minimum) || !Number.isInteger(maximum) || minimum < 1 || maximum < minimum || !save) { return; }
        const readers = Array.from(workspace.querySelectorAll("[data-lf-dpi-row]")).map(function (row) { return bindRow(row, minimum, maximum); });
        if (readers.some(function (reader) { return reader === null; })) { return; }
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
    }

    return {init: init, normalizeColor: normalizeColor, parseDPI: parseDPI, rangeProgress: rangeProgress};
});
