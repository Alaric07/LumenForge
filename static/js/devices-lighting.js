"use strict";

(function (root, factory) {
    const api = factory();
    if (typeof module === "object" && module.exports) {
        module.exports = api;
    }
    if (root && root.document) {
        if (root.document.readyState === "loading") {
            root.document.addEventListener("DOMContentLoaded", function () {
                api.init(root);
            });
        } else {
            api.init(root);
        }
    }
})(typeof window === "undefined" ? null : window, function () {
    const effectEndpoint = "/api/openrgbimport/effect";
    const effectTimeoutMilliseconds = 10000;
    const failureMessage = "Unable to change effect. Try again.";
    const brightnessEndpoint = "/api/openrgbimport/brightness";
    const brightnessTimeoutMilliseconds = 10000;
    const brightnessSuccessMilliseconds = 1800;
    const brightnessFailureMessage = "Unable to change brightness. Try again.";
    const speedEndpoint = "/api/openrgbimport/speed";
    const speedTimeoutMilliseconds = 10000;
    const speedSuccessMilliseconds = 1800;
    const keyboardCommitMilliseconds = 400;
    const speedFailureMessage = "Unable to change speed. Try again.";
    const colorEndpoint = "/api/openrgbimport/single-color";
    const colorTimeoutMilliseconds = 10000;
    const colorSuccessMilliseconds = 1800;
    const colorFailureMessage = "Unable to change color. Try again.";
    const twoColorEndpoint = "/api/openrgbimport/two-color";
    const twoColorTimeoutMilliseconds = 10000;
    const twoColorSuccessMilliseconds = 1800;
    const twoColorFailureMessage = "Unable to change colors. Try again.";
    const resetEndpoint = "/api/openrgbimport/effect-reset";
    const resetTimeoutMilliseconds = 10000;
    const resetSuccessMilliseconds = 3000;
    const resetFailureMessage = "Unable to reset effect. Try again.";
    const rangeAdjustmentKeys = new Set([
        "ArrowLeft",
        "ArrowRight",
        "ArrowUp",
        "ArrowDown",
        "PageUp",
        "PageDown",
        "Home",
        "End"
    ]);

    function createStatusController(browser, status, successMilliseconds) {
        let successTimer = null;
        let generation = 0;

        return function (message, clearAfterSuccess) {
            generation += 1;
            const currentGeneration = generation;
            if (successTimer !== null) {
                browser.clearTimeout(successTimer);
                successTimer = null;
            }
            if (status) {
                status.textContent = message;
            }
            if (status && clearAfterSuccess) {
                successTimer = browser.setTimeout(function () {
                    if (currentGeneration === generation) {
                        status.textContent = "";
                        successTimer = null;
                    }
                }, successMilliseconds);
            }
        };
    }

    function captureKeyboardFocus(browser, owner, numberInput, origin) {
        if (origin !== "keyboard" || !browser.document || browser.document.activeElement !== owner) {
            return null;
        }
        const focus = {owner: owner, selectionEnd: null, selectionStart: null};
        if (owner === numberInput) {
            try {
                if (typeof owner.selectionStart === "number" && typeof owner.selectionEnd === "number") {
                    focus.selectionStart = owner.selectionStart;
                    focus.selectionEnd = owner.selectionEnd;
                }
            } catch (_) {
                // Number inputs do not expose selection ranges in every browser.
            }
        }
        return focus;
    }

    function restoreKeyboardFocus(browser, focus, numberInput) {
        if (!focus || typeof focus.owner.focus !== "function") {
            return;
        }
        const activeElement = browser.document && browser.document.activeElement;
        const body = browser.document && browser.document.body;
        if (activeElement && activeElement !== focus.owner && activeElement !== body) {
            return;
        }
        try {
            focus.owner.focus({preventScroll: true});
        } catch (_) {
            focus.owner.focus();
        }
        if (focus.owner === numberInput && focus.selectionStart !== null && typeof focus.owner.setSelectionRange === "function") {
            const maximum = String(focus.owner.value).length;
            try {
                focus.owner.setSelectionRange(
                    Math.min(focus.selectionStart, maximum),
                    Math.min(focus.selectionEnd, maximum)
                );
            } catch (_) {
                // Preserve focus even when a number input rejects selection APIs.
            }
        }
    }

    function bindPairedKeyboardControlEvents(browser, slider, numberInput, previewRange, previewNumber, commit, requestInFlight) {
        let keyboardCommitTimer = null;
        let keyboardCommitGeneration = 0;
        let rangeKeyboardSession = false;
        let numberKeyboardSession = false;

        function cancelKeyboardCommit() {
            keyboardCommitGeneration += 1;
            if (keyboardCommitTimer !== null) {
                browser.clearTimeout(keyboardCommitTimer);
                keyboardCommitTimer = null;
            }
        }

        function resetKeyboardSessions() {
            rangeKeyboardSession = false;
            numberKeyboardSession = false;
            cancelKeyboardCommit();
        }

        function scheduleKeyboardCommit(owner, value) {
            cancelKeyboardCommit();
            const generation = keyboardCommitGeneration;
            keyboardCommitTimer = browser.setTimeout(function () {
                if (generation !== keyboardCommitGeneration || requestInFlight()) {
                    return;
                }
                keyboardCommitTimer = null;
                rangeKeyboardSession = false;
                numberKeyboardSession = false;
                return commit(value(), "keyboard", owner);
            }, keyboardCommitMilliseconds);
        }

        function handleRangeInput() {
            previewRange(slider.value);
            if (rangeKeyboardSession) {
                scheduleKeyboardCommit(slider, function () { return slider.value; });
            }
        }

        function handleRangeChange() {
            if (rangeKeyboardSession) {
                scheduleKeyboardCommit(slider, function () { return slider.value; });
                return;
            }
            return commit(slider.value, "pointer", slider);
        }

        function handleNumberInput() {
            previewNumber(numberInput.value);
            if (numberKeyboardSession) {
                scheduleKeyboardCommit(numberInput, function () { return numberInput.value; });
            }
        }

        function handleNumberChange() {
            if (numberKeyboardSession) {
                scheduleKeyboardCommit(numberInput, function () { return numberInput.value; });
                return;
            }
            return commit(numberInput.value, "change", numberInput);
        }

        function handleRangeKeydown(event) {
            if (rangeAdjustmentKeys.has(event.key)) {
                rangeKeyboardSession = true;
                scheduleKeyboardCommit(slider, function () { return slider.value; });
                return;
            }
            if (event.key === "Enter") {
                event.preventDefault();
                return commit(slider.value, "keyboard", slider);
            }
        }

        function handleNumberKeydown(event) {
            if (event.key === "ArrowUp" || event.key === "ArrowDown") {
                numberKeyboardSession = true;
                scheduleKeyboardCommit(numberInput, function () { return numberInput.value; });
                return;
            }
            if (event.key === "Enter") {
                event.preventDefault();
                return commit(numberInput.value, "keyboard", numberInput);
            }
            numberKeyboardSession = false;
            cancelKeyboardCommit();
        }

        function handleRangeBlur() {
            return commit(slider.value, "blur", slider);
        }

        function handleNumberBlur() {
            return commit(numberInput.value, "blur", numberInput);
        }

        function handleRangePointerdown() {
            resetKeyboardSessions();
        }

        slider.addEventListener("input", handleRangeInput);
        slider.addEventListener("change", handleRangeChange);
        slider.addEventListener("keydown", handleRangeKeydown);
        slider.addEventListener("blur", handleRangeBlur);
        slider.addEventListener("pointerdown", handleRangePointerdown);
        numberInput.addEventListener("input", handleNumberInput);
        numberInput.addEventListener("change", handleNumberChange);
        numberInput.addEventListener("blur", handleNumberBlur);
        numberInput.addEventListener("keydown", handleNumberKeydown);
        return {
            cancel: resetKeyboardSessions,
            handlers: {
                handleChange: handleRangeChange,
                handleInput: handleRangeInput,
                handleNumberBlur,
                handleNumberChange,
                handleNumberInput,
                handleNumberKeydown,
                handleRangeBlur,
                handleRangeKeydown,
                handleRangePointerdown
            }
        };
    }

    async function submitEffect(browser, serial, effect) {
        const controller = new browser.AbortController();
        const timeout = browser.setTimeout(function () {
            controller.abort();
        }, effectTimeoutMilliseconds);
        try {
            const response = await browser.fetch(effectEndpoint, {
                method: "POST",
                body: JSON.stringify({serial: serial, effect: effect}),
                signal: controller.signal
            });
            if (!response.ok) {
                throw new Error("effect request failed");
            }

            const result = await response.json();
            if (!result || result.status !== 1) {
                throw new Error("effect mutation was rejected");
            }
        } finally {
            browser.clearTimeout(timeout);
        }
    }

    async function submitBrightness(browser, serial, brightness) {
        const controller = new browser.AbortController();
        const timeout = browser.setTimeout(function () {
            controller.abort();
        }, brightnessTimeoutMilliseconds);
        try {
            const response = await browser.fetch(brightnessEndpoint, {
                method: "POST",
                body: JSON.stringify({serial: serial, brightness: brightness}),
                signal: controller.signal
            });
            if (!response.ok) {
                throw new Error("brightness request failed");
            }

            const result = await response.json();
            if (!result || result.status !== 1) {
                throw new Error("brightness mutation was rejected");
            }
        } finally {
            browser.clearTimeout(timeout);
        }
    }

    async function submitSpeed(browser, serial, effect, speed) {
        const controller = new browser.AbortController();
        const timeout = browser.setTimeout(function () {
            controller.abort();
        }, speedTimeoutMilliseconds);
        try {
            const response = await browser.fetch(speedEndpoint, {
                method: "POST",
                body: JSON.stringify({serial: serial, effect: effect, speed: speed}),
                signal: controller.signal
            });
            if (!response.ok) {
                throw new Error("speed request failed");
            }

            const result = await response.json();
            if (!result || result.status !== 1) {
                throw new Error("speed mutation was rejected");
            }
        } finally {
            browser.clearTimeout(timeout);
        }
    }

    async function submitColor(browser, serial, effect, color) {
        const controller = new browser.AbortController();
        const timeout = browser.setTimeout(function () {
            controller.abort();
        }, colorTimeoutMilliseconds);
        try {
            const response = await browser.fetch(colorEndpoint, {
                method: "POST",
                body: JSON.stringify({serial: serial, effect: effect, color: color}),
                signal: controller.signal
            });
            if (!response.ok) {
                throw new Error("color request failed");
            }
            const result = await response.json();
            if (!result || result.status !== 1) {
                throw new Error("color mutation was rejected");
            }
        } finally {
            browser.clearTimeout(timeout);
        }
    }

    async function submitTwoColor(browser, serial, effect, start, end) {
        const controller = new browser.AbortController();
        const timeout = browser.setTimeout(function () {
            controller.abort();
        }, twoColorTimeoutMilliseconds);
        try {
            const response = await browser.fetch(twoColorEndpoint, {
                method: "POST",
                body: JSON.stringify({serial: serial, effect: effect, start: start, end: end}),
                signal: controller.signal
            });
            if (!response.ok) {
                throw new Error("two-color request failed");
            }
            const result = await response.json();
            if (!result || result.status !== 1) {
                throw new Error("two-color mutation was rejected");
            }
        } finally {
            browser.clearTimeout(timeout);
        }
    }

    async function submitReset(browser, serial, effect) {
        const controller = new browser.AbortController();
        const timeout = browser.setTimeout(function () {
            controller.abort();
        }, resetTimeoutMilliseconds);
        try {
            const response = await browser.fetch(resetEndpoint, {
                method: "POST",
                body: JSON.stringify({serial: serial, effect: effect}),
                signal: controller.signal
            });
            if (!response.ok) {
                throw new Error("reset request failed");
            }
            const result = await response.json();
            if (!result || result.status !== 1) {
                throw new Error("reset mutation was rejected");
            }
        } finally {
            browser.clearTimeout(timeout);
        }
    }

    function parseBrightness(value) {
        const text = String(value);
        if (!/^\d+$/.test(text)) {
            return null;
        }
        const brightness = Number(text);
        if (!Number.isInteger(brightness) || brightness < 0 || brightness > 100) {
            return null;
        }
        return brightness;
    }

    function renderBrightness(slider, numberInput, brightness) {
        renderBrightnessRange(slider, brightness);
        numberInput.value = String(brightness);
    }

    function renderBrightnessRange(slider, brightness) {
        slider.value = String(brightness);
        slider.setAttribute("aria-valuetext", brightness + " percent");
        slider.style.setProperty("--lf-range-progress", brightness + "%");
    }

    function updateBrightnessReadouts(browser, serial, brightness) {
        const readouts = browser.document.querySelectorAll("[data-lf-brightness-readout]");
        for (const readout of readouts) {
            if (readout.dataset.lfDeviceSerial === serial) {
                readout.textContent = brightness + "%";
            }
        }
    }

    function bindBrightnessSlider(browser, slider) {
        const clusterControlled = slider.dataset.lfClusterControlled === "true";
        if (clusterControlled) {
            return null;
        }

        const numberInput = browser.document.getElementById(slider.dataset.lfNumberId);
        const status = browser.document.getElementById(slider.dataset.lfStatusId);
        if (!numberInput) {
            return null;
        }
        let confirmedBrightness = parseBrightness(slider.dataset.lfCurrentBrightness);
        if (confirmedBrightness === null) {
            return null;
        }
        let requestInFlight = false;
        let keyboardBinding;
        const setStatus = createStatusController(browser, status, brightnessSuccessMilliseconds);

        function restoreConfirmedBrightness() {
            keyboardBinding.cancel();
            renderBrightness(slider, numberInput, confirmedBrightness);
        }

        function previewRangeBrightness(value) {
            const brightness = parseBrightness(value);
            if (brightness === null) {
                return;
            }
            setStatus("", false);
            renderBrightness(slider, numberInput, brightness);
        }

        function previewNumberBrightness(value) {
            const brightness = parseBrightness(value);
            if (brightness === null) {
                return;
            }
            setStatus("", false);
            renderBrightnessRange(slider, brightness);
        }

        async function commitBrightness(value, origin, owner) {
            const brightness = parseBrightness(value);
            if (requestInFlight) {
                return;
            }
            keyboardBinding.cancel();
            if (brightness === null) {
                restoreConfirmedBrightness();
                return;
            }
            setStatus("", false);
            if (brightness === confirmedBrightness) {
                restoreConfirmedBrightness();
                return;
            }

            const keyboardFocus = captureKeyboardFocus(browser, owner, numberInput, origin);
            requestInFlight = true;
            slider.disabled = true;
            numberInput.disabled = true;
            setStatus("Saving brightness…", false);

            try {
                await submitBrightness(browser, slider.dataset.lfDeviceSerial, brightness);
                confirmedBrightness = brightness;
                slider.dataset.lfCurrentBrightness = String(brightness);
                renderBrightness(slider, numberInput, brightness);
                updateBrightnessReadouts(browser, slider.dataset.lfDeviceSerial, brightness);
                setStatus("Brightness saved.", true);
            } catch (_) {
                restoreConfirmedBrightness();
                setStatus(brightnessFailureMessage, false);
            } finally {
                slider.disabled = false;
                numberInput.disabled = false;
                requestInFlight = false;
                restoreKeyboardFocus(browser, keyboardFocus, numberInput);
            }
        }

        keyboardBinding = bindPairedKeyboardControlEvents(
            browser,
            slider,
            numberInput,
            previewRangeBrightness,
            previewNumberBrightness,
            commitBrightness,
            function () { return requestInFlight; }
        );
        return keyboardBinding.handlers;
    }

    function parseSpeedLevel(value) {
        const text = String(value);
        if (!/^\d+(?:\.\d+)?$/.test(text)) {
            return null;
        }
        const speed = Number(text);
        if (!Number.isFinite(speed) || speed < 1 || speed > 10 || Math.abs(speed * 10 - Math.round(speed * 10)) > 1e-9) {
            return null;
        }
        return speed;
    }

    function renderSpeed(speedHelper, slider, numberInput, speed) {
        const formatted = speedHelper.formatForSlider(slider, speed);
        renderSpeedRange(speedHelper, slider, speed);
        numberInput.value = formatted;
    }

    function renderSpeedRange(speedHelper, slider, speed) {
        const formatted = speedHelper.formatForSlider(slider, speed);
        const minimumText = slider.min;
        const maximumText = slider.max;
        const formattedText = String(formatted);
        const minimum = Number(minimumText);
        const maximum = Number(maximumText);
        const displayed = Number(formattedText);
        let progress = 0;
        if (typeof minimumText === "string" && minimumText.trim() !== "" &&
            typeof maximumText === "string" && maximumText.trim() !== "" && formattedText.trim() !== "" &&
            Number.isFinite(minimum) && Number.isFinite(maximum) && Number.isFinite(displayed) && maximum > minimum) {
            progress = Math.min(100, Math.max(0, ((displayed - minimum) / (maximum - minimum)) * 100));
        }
        slider.value = formatted;
        slider.setAttribute("aria-valuetext", formatted + " speed level");
        slider.style.setProperty("--lf-range-progress", progress + "%");
    }

    function updateSpeedReadouts(browser, target, storedSpeed) {
        const formatted = String(Number(storedSpeed));
        const sourceTarget = target === "base" || target === "override" ? target : "";
        const readouts = browser.document.querySelectorAll("[data-lf-speed-readout]");
        for (const readout of readouts) {
            const role = readout.dataset.lfSpeedReadout;
            if (role === "effective" || (sourceTarget && role === sourceTarget)) {
                readout.textContent = formatted;
            }
        }
    }

    function revealEffectReset(browser, serial, effect) {
        const controls = browser.document.querySelectorAll("[data-lf-reset-control]");
        for (const control of controls) {
            if (control.dataset.lfDeviceSerial === serial && control.dataset.lfEffect === effect) {
                control.hidden = false;
            }
        }
    }

    function bindSpeedSlider(browser, slider) {
        const speedHelper = browser.LumenForgeRgbSpeed;
        if (!speedHelper || typeof speedHelper.configureSlider !== "function" ||
            typeof speedHelper.storedToUiForSlider !== "function" || typeof speedHelper.markEdited !== "function" ||
            typeof speedHelper.uiToStoredForSlider !== "function" || typeof speedHelper.formatForSlider !== "function" ||
            typeof speedHelper.hasSpeedControl !== "function") {
            return null;
        }

        const effect = slider.dataset.lfEffect || "";
        const controlMode = speedHelper.SOFTWARE_CONTROL;
        if (!effect || !speedHelper.hasSpeedControl(effect, controlMode)) {
            return null;
        }
        const numberInput = browser.document.getElementById(slider.dataset.lfNumberId);
        const status = browser.document.getElementById(slider.dataset.lfStatusId);
        const storedSpeedText = slider.dataset.lfCurrentStoredSpeed;
        if (!numberInput || typeof storedSpeedText !== "string" || storedSpeedText.trim() === "") {
            return null;
        }
        const storedSpeed = Number(storedSpeedText);
        if (!Number.isFinite(storedSpeed)) {
            return null;
        }

        speedHelper.configureSlider(slider, effect, controlMode);
        let confirmedStoredSpeed = storedSpeed;
        let confirmedSpeed = speedHelper.storedToUiForSlider(slider, storedSpeed);
        renderSpeed(speedHelper, slider, numberInput, confirmedSpeed);
        if (typeof slider.closest === "function") {
            const control = slider.closest("[data-lf-speed-control]");
            if (control && control.classList) {
                control.classList.add("lf-range-control-ready");
            }
        }

        if (slider.dataset.lfClusterControlled === "true") {
            return null;
        }

        let requestInFlight = false;
        let keyboardBinding;
        const setStatus = createStatusController(browser, status, speedSuccessMilliseconds);

        function restoreConfirmedSpeed() {
            keyboardBinding.cancel();
            confirmedSpeed = speedHelper.storedToUiForSlider(slider, confirmedStoredSpeed);
            renderSpeed(speedHelper, slider, numberInput, confirmedSpeed);
        }

        function previewRangeSpeed(value) {
            const speed = parseSpeedLevel(value);
            if (speed === null) {
                return;
            }
            speedHelper.markEdited(slider);
            setStatus("", false);
            renderSpeed(speedHelper, slider, numberInput, speed);
        }

        function previewNumberSpeed(value) {
            const speed = parseSpeedLevel(value);
            if (speed === null) {
                return;
            }
            speedHelper.markEdited(slider);
            setStatus("", false);
            renderSpeedRange(speedHelper, slider, speed);
        }

        async function commitSpeed(value, origin, owner) {
            const speed = parseSpeedLevel(value);
            if (requestInFlight) {
                return;
            }
            keyboardBinding.cancel();
            if (speed === null) {
                restoreConfirmedSpeed();
                return;
            }
            setStatus("", false);
            const edited = slider.dataset.speedEdited === "true";
            const mappedSpeed = speedHelper.uiToStoredForSlider(slider, speed);
            if (!edited || mappedSpeed === confirmedStoredSpeed) {
                restoreConfirmedSpeed();
                return;
            }

            const keyboardFocus = captureKeyboardFocus(browser, owner, numberInput, origin);
            requestInFlight = true;
            slider.disabled = true;
            numberInput.disabled = true;
            setStatus("Saving speed…", false);
            try {
                await submitSpeed(browser, slider.dataset.lfDeviceSerial, effect, mappedSpeed);
                confirmedStoredSpeed = mappedSpeed;
                slider.dataset.lfCurrentStoredSpeed = String(mappedSpeed);
                confirmedSpeed = speedHelper.storedToUiForSlider(slider, mappedSpeed);
                renderSpeed(speedHelper, slider, numberInput, confirmedSpeed);
                updateSpeedReadouts(browser, slider.dataset.lfSpeedTarget, mappedSpeed);
                revealEffectReset(browser, slider.dataset.lfDeviceSerial, effect);
                setStatus("Speed saved.", true);
            } catch (_) {
                restoreConfirmedSpeed();
                setStatus(speedFailureMessage, false);
            } finally {
                slider.disabled = false;
                numberInput.disabled = false;
                requestInFlight = false;
                restoreKeyboardFocus(browser, keyboardFocus, numberInput);
            }
        }

        keyboardBinding = bindPairedKeyboardControlEvents(
            browser,
            slider,
            numberInput,
            previewRangeSpeed,
            previewNumberSpeed,
            commitSpeed,
            function () { return requestInFlight; }
        );
        return keyboardBinding.handlers;
    }

    function bindEffectSelector(browser, selector) {
        const clusterControlled = selector.dataset.lfClusterControlled === "true";
        if (clusterControlled) {
            return null;
        }

        const status = browser.document.getElementById(selector.dataset.lfStatusId);
        const configuredEffect = selector.dataset.lfCurrentEffect || "";
        let requestInFlight = false;

        async function handleChange() {
            const selectedEffect = selector.value;
            if (requestInFlight || !selectedEffect || selectedEffect === configuredEffect) {
                return;
            }

            requestInFlight = true;
            selector.disabled = true;
            if (status) {
                status.textContent = "";
            }

            try {
                await submitEffect(browser, selector.dataset.lfDeviceSerial, selectedEffect);
                browser.location.reload();
            } catch (_) {
                selector.value = configuredEffect;
                selector.disabled = false;
                if (status) {
                    status.textContent = failureMessage;
                }
            } finally {
                requestInFlight = false;
            }
        }

        selector.addEventListener("change", handleChange);
        return handleChange;
    }

    function bindColorControl(browser, colorInput) {
        const clusterControlled = colorInput.dataset.lfClusterControlled === "true";
        if (clusterControlled) return null;
        const hexInput = browser.document.getElementById(colorInput.dataset.lfHexId);
        const status = browser.document.getElementById(colorInput.dataset.lfStatusId);
        if (!hexInput) return null;

        let confirmedColor = colorInput.dataset.lfCurrentColor.toLowerCase();
        let requestInFlight = false;
        const setStatus = createStatusController(browser, status, colorSuccessMilliseconds);

        function preview(value) {
            if (/^#[0-9a-fA-F]{6}$/.test(value)) {
                colorInput.value = value.toLowerCase();
                hexInput.value = value.toLowerCase();
            }
        }

        async function commit(value) {
            if (requestInFlight) return;
            if (!/^#[0-9a-fA-F]{6}$/.test(value)) {
                colorInput.value = confirmedColor;
                hexInput.value = confirmedColor;
                return;
            }
            const normalized = value.toLowerCase();
            if (normalized === confirmedColor) return;
            requestInFlight = true;
            colorInput.disabled = true;
            hexInput.disabled = true;
            setStatus("Saving color…", false);
            try {
                await submitColor(browser, colorInput.dataset.lfDeviceSerial, colorInput.dataset.lfEffect, normalized);
                confirmedColor = normalized;
                colorInput.dataset.lfCurrentColor = normalized;
                setStatus("Color saved.", true);
                browser.location.reload();
            } catch (_) {
                colorInput.value = confirmedColor;
                hexInput.value = confirmedColor;
                setStatus(colorFailureMessage, false);
                colorInput.disabled = false;
                hexInput.disabled = false;
                requestInFlight = false;
            }
        }

        colorInput.addEventListener("input", function() { hexInput.value = colorInput.value.toLowerCase(); });
        colorInput.addEventListener("change", function() { commit(colorInput.value); });
        hexInput.addEventListener("input", function() { preview(hexInput.value); });
        hexInput.addEventListener("change", function() { commit(hexInput.value); });
        hexInput.addEventListener("keydown", function(e) {
            if (e.key === "Enter") {
                e.preventDefault();
                commit(hexInput.value);
            }
        });
        return commit;
    }

    function bindTwoColorControl(browser, container) {
        if (container.dataset.lfClusterControlled === "true") return null;

        const startColor = browser.document.getElementById(container.dataset.lfStartColorId);
        const startHex = browser.document.getElementById(container.dataset.lfStartHexId);
        const endColor = browser.document.getElementById(container.dataset.lfEndColorId);
        const endHex = browser.document.getElementById(container.dataset.lfEndHexId);
        const status = browser.document.getElementById(container.dataset.lfStatusId);
        const controls = [startColor, startHex, endColor, endHex];
        if (controls.some(function (control) { return !control; })) return null;

        function normalize(value) {
            return /^#[0-9a-fA-F]{6}$/.test(value) ? value.toLowerCase() : null;
        }

        let confirmedStart = normalize(container.dataset.lfCurrentStart || "");
        let confirmedEnd = normalize(container.dataset.lfCurrentEnd || "");
        if (confirmedStart === null || confirmedEnd === null) return null;

        let requestInFlight = false;
        const setStatus = createStatusController(browser, status, twoColorSuccessMilliseconds);

        function render(start, end) {
            startColor.value = start;
            startHex.value = start;
            endColor.value = end;
            endHex.value = end;
        }

        function restoreConfirmed() {
            render(confirmedStart, confirmedEnd);
        }

        function preview(colorInput, hexInput, value) {
            const normalized = normalize(value);
            if (normalized !== null) {
                colorInput.value = normalized;
                hexInput.value = normalized;
            }
        }

        async function commit() {
            if (requestInFlight) return;
            const start = normalize(startHex.value);
            const end = normalize(endHex.value);
            if (start === null || end === null) {
                restoreConfirmed();
                return;
            }
            if (start === confirmedStart && end === confirmedEnd) {
                restoreConfirmed();
                return;
            }

            requestInFlight = true;
            for (const control of controls) control.disabled = true;
            setStatus("Saving colors…", false);
            try {
                await submitTwoColor(browser, container.dataset.lfDeviceSerial, container.dataset.lfEffect, start, end);
                confirmedStart = start;
                confirmedEnd = end;
                container.dataset.lfCurrentStart = start;
                container.dataset.lfCurrentEnd = end;
                render(start, end);
                setStatus("Colors saved.", true);
                browser.location.reload();
            } catch (_) {
                restoreConfirmed();
                setStatus(twoColorFailureMessage, false);
                for (const control of controls) control.disabled = false;
                requestInFlight = false;
            }
        }

        startColor.addEventListener("input", function () { preview(startColor, startHex, startColor.value); });
        startColor.addEventListener("change", commit);
        startHex.addEventListener("input", function () { preview(startColor, startHex, startHex.value); });
        startHex.addEventListener("change", commit);
        startHex.addEventListener("keydown", function (event) {
            if (event.key === "Enter") {
                event.preventDefault();
                commit();
            }
        });
        endColor.addEventListener("input", function () { preview(endColor, endHex, endColor.value); });
        endColor.addEventListener("change", commit);
        endHex.addEventListener("input", function () { preview(endColor, endHex, endHex.value); });
        endHex.addEventListener("change", commit);
        endHex.addEventListener("keydown", function (event) {
            if (event.key === "Enter") {
                event.preventDefault();
                commit();
            }
        });
        return commit;
    }

    function bindResetButton(browser, button) {
        const clusterControlled = button.dataset.lfClusterControlled === "true";
        if (clusterControlled) return null;
        const status = browser.document.getElementById(button.dataset.lfStatusId);
        if (!status) return null;

        let requestInFlight = false;
        const setStatus = createStatusController(browser, status, resetSuccessMilliseconds);

        async function handleClick() {
            if (requestInFlight) return;
            requestInFlight = true;
            button.disabled = true;
            setStatus("Resetting effect…", false);
            try {
                await submitReset(browser, button.dataset.lfDeviceSerial, button.dataset.lfEffect);
                setStatus("Effect reset.", true);
                browser.location.reload();
            } catch (_) {
                setStatus(resetFailureMessage, false);
                button.disabled = false;
                requestInFlight = false;
            }
        }
        button.addEventListener("click", handleClick);
        return handleClick;
    }

    function init(browser) {
        const selectors = browser.document.querySelectorAll("[data-lf-effect-selector]");
        for (const selector of selectors) {
            bindEffectSelector(browser, selector);
        }
        const sliders = browser.document.querySelectorAll("[data-lf-brightness-slider]");
        for (const slider of sliders) {
            bindBrightnessSlider(browser, slider);
        }
        const speedSliders = browser.document.querySelectorAll("[data-lf-speed-slider]");
        for (const slider of speedSliders) {
            bindSpeedSlider(browser, slider);
        }
        const colorInputs = browser.document.querySelectorAll("[data-lf-color-input]");
        for (const input of colorInputs) {
            bindColorControl(browser, input);
        }
        const twoColorControls = browser.document.querySelectorAll("[data-lf-two-color-control]");
        for (const control of twoColorControls) {
            bindTwoColorControl(browser, control);
        }
        const resetButtons = browser.document.querySelectorAll("[data-lf-reset-button]");
        for (const button of resetButtons) {
            bindResetButton(browser, button);
        }
    }

    return {
        bindBrightnessSlider,
        bindEffectSelector,
        bindSpeedSlider,
        bindColorControl,
        bindTwoColorControl,
        bindResetButton,
        revealEffectReset,
        brightnessEndpoint,
        effectEndpoint,
        colorEndpoint,
        twoColorEndpoint,
        resetEndpoint,
        init,
        submitBrightness,
        submitEffect,
        submitSpeed,
        submitColor,
        submitTwoColor,
        submitReset,
        speedEndpoint
    };
});
