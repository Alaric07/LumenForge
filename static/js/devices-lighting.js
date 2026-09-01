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
    const openRGBLightingEndpoints = Object.freeze({
        effect: "/api/openrgbimport/effect",
        brightness: "/api/openrgbimport/brightness",
        speed: "/api/openrgbimport/speed",
        color: "/api/openrgbimport/single-color",
        twoColor: "/api/openrgbimport/two-color",
        temperature: "/api/openrgbimport/temperature",
        gradient: "/api/openrgbimport/gradient",
        reset: "/api/openrgbimport/effect-reset"
    });
    const clusterLightingEndpoints = Object.freeze({
        effect: "/api/cluster/lighting/effect",
        brightness: "/api/cluster/lighting/brightness",
        speed: "/api/cluster/lighting/speed",
        color: "/api/cluster/lighting/single-color",
        twoColor: "/api/cluster/lighting/two-color",
        temperature: "/api/cluster/lighting/temperature",
        gradient: "/api/cluster/lighting/gradient",
        reset: "/api/cluster/lighting/effect-reset"
    });
    const nativeLightingEndpoints = Object.freeze({
        effect: "/api/devices/lighting/effect",
        brightness: "/api/devices/lighting/brightness",
        speed: "/api/devices/lighting/speed",
        indexedColors: "/api/devices/lighting/indexed-colors",
        color: "/api/devices/lighting/single-color",
        twoColor: "/api/devices/lighting/two-color",
        temperature: "/api/devices/lighting/temperature",
        gradient: "/api/devices/lighting/gradient",
        reset: "/api/devices/lighting/effect-reset",
        zones: "/api/devices/lighting/zones"
    });
    const effectEndpoint = openRGBLightingEndpoints.effect;
    const effectTimeoutMilliseconds = 10000;
    const failureMessage = "Unable to change effect. Try again.";
    const brightnessEndpoint = openRGBLightingEndpoints.brightness;
    const brightnessTimeoutMilliseconds = 10000;
    const brightnessSuccessMilliseconds = 1800;
    const brightnessFailureMessage = "Unable to change brightness. Try again.";
    const speedEndpoint = openRGBLightingEndpoints.speed;
    const speedTimeoutMilliseconds = 10000;
    const speedSuccessMilliseconds = 1800;
    const keyboardCommitMilliseconds = 400;
    const speedFailureMessage = "Unable to change speed. Try again.";
    const colorEndpoint = openRGBLightingEndpoints.color;
    const colorTimeoutMilliseconds = 10000;
    const colorSuccessMilliseconds = 1800;
    const colorFailureMessage = "Unable to change color. Try again.";
    const twoColorEndpoint = openRGBLightingEndpoints.twoColor;
    const twoColorTimeoutMilliseconds = 10000;
    const twoColorSuccessMilliseconds = 1800;
    const twoColorFailureMessage = "Unable to change colors. Try again.";
    const temperatureEndpoint = openRGBLightingEndpoints.temperature;
    const temperatureTimeoutMilliseconds = 10000;
    const temperatureSuccessMilliseconds = 1800;
    const temperatureFailureMessage = "Unable to change temperature colors. Try again.";
    const temperatureOrderMessage = "Thresholds must satisfy Low < Middle < High.";
    const gradientEndpoint = openRGBLightingEndpoints.gradient;
    const gradientTimeoutMilliseconds = 10000;
    const gradientSuccessMilliseconds = 1800;
    const gradientFailureMessage = "Unable to change Gradient. Try again.";
    const gradientCompleteMessage = "Every Gradient stop needs a valid color, position, and intensity.";
    const gradientRangeMessage = "Gradient positions and intensities must be between 0 and 1.";
    const resetEndpoint = openRGBLightingEndpoints.reset;
    const resetTimeoutMilliseconds = 10000;
    const resetSuccessMilliseconds = 3000;
    const resetFailureMessage = "Unable to reset effect. Try again.";
    const ccxtProbeTemperatureEndpoint = "/api/color/setTemperatureProbe";
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

    function lightingTarget(element) {
        const targetKind = element && element.dataset && element.dataset.lfLightingTarget || "openrgb";
        const cluster = targetKind === "cluster";
        const native = targetKind === "native";
		return {
			endpoints: cluster ? clusterLightingEndpoints : native ? nativeLightingEndpoints : openRGBLightingEndpoints,
			kind: cluster ? "cluster" : native ? "native" : "openrgb",
			serial: cluster ? "" : element.dataset.lfDeviceSerial,
			targetId: element.dataset.lfLightingChannelTarget || ""
        };
    }

    function lightingExternallyOwned(element) {
        return element.dataset.lfClusterControlled === "true" || element.dataset.lfExternalControlled === "true";
    }

    function openRGBLightingTarget(serial) {
        return {endpoints: openRGBLightingEndpoints, kind: "openrgb", serial: serial};
    }

    function lightingPayload(target, values) {
        if (target.kind === "cluster") {
            return values;
        }
		const identity = target.targetId ? {serial: target.serial, targetId: target.targetId} : {serial: target.serial};
		return Object.assign(identity, values);
    }

    async function submitLightingMutation(browser, endpoint, payload, timeoutMilliseconds, requestFailure, mutationFailure) {
        const controller = new browser.AbortController();
        const timeout = browser.setTimeout(function () {
            controller.abort();
        }, timeoutMilliseconds);
        try {
            const response = await browser.fetch(endpoint, {
                method: "POST",
                body: JSON.stringify(payload),
                signal: controller.signal
            });
            if (!response.ok) {
                throw new Error(requestFailure);
            }

            const result = await response.json();
            if (!result || result.status !== 1) {
                throw new Error(mutationFailure);
            }
        } finally {
            browser.clearTimeout(timeout);
        }
    }

    const ownershipEndpoints = Object.freeze({
        cluster: "/api/devices/lighting/rgb-cluster",
        "openrgb-integration": "/api/devices/lighting/openrgb-integration"
    });
    const ownershipTimeoutMilliseconds = 10000;

    function showOwnershipSaved(browser) {
        if (typeof browser.LumenForgeDevicesToast === "function") {
            browser.LumenForgeDevicesToast("✓ Saved", "success", 1500);
        }
    }

    function updateOwnershipToggleAvailability(controls) {
        const cluster = controls.find(function (control) { return control.dataset.lfOwnershipKind === "cluster"; });
        const external = controls.find(function (control) { return control.dataset.lfOwnershipKind === "openrgb-integration"; });
        const clusterEnabled = cluster && cluster.dataset.lfConfirmedEnabled === "1";
        const externalEnabled = external && external.dataset.lfConfirmedEnabled === "1";
        for (const control of controls) {
            const input = control.querySelector("[data-lf-lighting-ownership-input]");
            if (!input) { continue; }
            input.checked = control.dataset.lfConfirmedEnabled === "1";
            input.disabled = control.dataset.lfReadOnly === "true" ||
                (control.dataset.lfOwnershipKind === "cluster" && externalEnabled) ||
                (control.dataset.lfOwnershipKind === "openrgb-integration" && clusterEnabled);
        }
    }

    function disableLocalLightingControls(browser) {
        const document = browser.document;
        if (!document || typeof document.querySelector !== "function") { return; }
        const workspace = document.querySelector("[data-lf-lighting-read-only]");
        if (!workspace || typeof workspace.querySelectorAll !== "function") { return; }
        const selectors = [
            "[data-lf-effect-selector]",
            "[data-lf-brightness-number]",
            "[data-lf-brightness-slider]",
            "[data-lf-speed-number]",
            "[data-lf-speed-slider]",
            "[data-lf-color-input]",
            "[data-lf-color-hex]",
            "[data-lf-indexed-editor] input, [data-lf-indexed-editor] button",
            "[data-lf-two-color-control] input",
			"[data-lf-temperature-control] input",
			"[data-lf-gradient-control] input, [data-lf-gradient-control] button",
			"[data-lf-ccxt-probe-temperature] input, [data-lf-ccxt-probe-temperature] button",
			"[data-lf-ccxt-three-pin-port] select",
			"[data-lf-manual-rgb-device]",
			"[data-lf-authored-zone-control] input, [data-lf-authored-zone-control] button"
        ];
        for (const selector of selectors) {
            for (const control of workspace.querySelectorAll(selector)) {
                control.disabled = true;
            }
        }
        for (const reset of workspace.querySelectorAll("[data-lf-reset-control]")) {
            reset.hidden = true;
            for (const control of reset.querySelectorAll("input, button")) {
                control.disabled = true;
            }
        }
    }

    function bindOwnershipToggle(browser, control, controls) {
        const input = control.querySelector("[data-lf-lighting-ownership-input]");
        const status = control.querySelector("[data-lf-lighting-ownership-status]");
        const endpoint = ownershipEndpoints[control.dataset.lfOwnershipKind];
        const deviceId = control.dataset.lfDeviceId;
        if (!input || !status || !endpoint || !deviceId) { return null; }
        let saving = false;
        input.addEventListener("change", async function () {
            if (saving) { return; }
            const confirmed = control.dataset.lfConfirmedEnabled === "1";
            const enabled = input.checked;
            saving = true;
            input.disabled = true;
            status.textContent = "";
            try {
                await submitLightingMutation(browser, endpoint, {deviceId: deviceId, mode: enabled ? 1 : 0},
                    ownershipTimeoutMilliseconds, "ownership request failed", "ownership save rejected");
                control.dataset.lfConfirmedEnabled = enabled ? "1" : "0";
                updateOwnershipToggleAvailability(controls);
                if (enabled) { disableLocalLightingControls(browser); }
                showOwnershipSaved(browser);
                if (browser.location && typeof browser.location.reload === "function") {
                    browser.setTimeout(function () { browser.location.reload(); }, 1500);
                }
            } catch (_) {
                input.checked = confirmed;
                status.textContent = "Unable to save lighting ownership. Try again.";
                updateOwnershipToggleAvailability(controls);
            } finally {
                saving = false;
                updateOwnershipToggleAvailability(controls);
            }
        });
        return input;
    }

    function submitEffectForTarget(browser, target, effect) {
        return submitLightingMutation(browser, target.endpoints.effect, lightingPayload(target, {effect: effect}),
            effectTimeoutMilliseconds, "effect request failed", "effect mutation was rejected");
    }

    async function submitEffect(browser, serial, effect) {
        return submitEffectForTarget(browser, openRGBLightingTarget(serial), effect);
    }

    function submitBrightnessForTarget(browser, target, brightness) {
        return submitLightingMutation(browser, target.endpoints.brightness, lightingPayload(target, {brightness: brightness}),
            brightnessTimeoutMilliseconds, "brightness request failed", "brightness mutation was rejected");
    }

    async function submitBrightness(browser, serial, brightness) {
        return submitBrightnessForTarget(browser, openRGBLightingTarget(serial), brightness);
    }

    function submitSpeedForTarget(browser, target, effect, speed) {
        return submitLightingMutation(browser, target.endpoints.speed, lightingPayload(target, {effect: effect, speed: speed}),
            speedTimeoutMilliseconds, "speed request failed", "speed mutation was rejected");
    }

    async function submitSpeed(browser, serial, effect, speed) {
        return submitSpeedForTarget(browser, openRGBLightingTarget(serial), effect, speed);
    }

    function submitColorForTarget(browser, target, effect, color) {
        return submitLightingMutation(browser, target.endpoints.color, lightingPayload(target, {effect: effect, color: color}),
            colorTimeoutMilliseconds, "color request failed", "color mutation was rejected");
    }

    function submitIndexedColorsForTarget(browser, target, colors) {
        return submitLightingMutation(browser, target.endpoints.indexedColors, lightingPayload(target, {colors: colors}), colorTimeoutMilliseconds, "indexed colors request failed", "indexed colors mutation was rejected");
    }

    async function submitColor(browser, serial, effect, color) {
        return submitColorForTarget(browser, openRGBLightingTarget(serial), effect, color);
    }

    function submitTwoColorForTarget(browser, target, effect, start, end) {
        return submitLightingMutation(browser, target.endpoints.twoColor,
            lightingPayload(target, {effect: effect, start: start, end: end}), twoColorTimeoutMilliseconds,
            "two-color request failed", "two-color mutation was rejected");
    }

    async function submitTwoColor(browser, serial, effect, start, end) {
        return submitTwoColorForTarget(browser, openRGBLightingTarget(serial), effect, start, end);
    }

    function submitTemperatureForTarget(browser, target, effect, low, middle, high) {
        return submitLightingMutation(browser, target.endpoints.temperature,
            lightingPayload(target, {effect: effect, low: low, middle: middle, high: high}), temperatureTimeoutMilliseconds,
            "temperature request failed", "temperature mutation was rejected");
    }

    async function submitTemperature(browser, serial, effect, low, middle, high) {
        return submitTemperatureForTarget(browser, openRGBLightingTarget(serial), effect, low, middle, high);
    }

    function submitGradientForTarget(browser, target, effect, stops) {
        return submitLightingMutation(browser, target.endpoints.gradient,
            lightingPayload(target, {effect: effect, stops: stops}), gradientTimeoutMilliseconds,
            "Gradient request failed", "Gradient mutation was rejected");
    }

    async function submitGradient(browser, serial, effect, stops) {
        return submitGradientForTarget(browser, openRGBLightingTarget(serial), effect, stops);
    }

    function submitResetForTarget(browser, target, effect) {
        return submitLightingMutation(browser, target.endpoints.reset, lightingPayload(target, {effect: effect}),
            resetTimeoutMilliseconds, "reset request failed", "reset mutation was rejected");
    }

    async function submitReset(browser, serial, effect) {
        return submitResetForTarget(browser, openRGBLightingTarget(serial), effect);
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

    function updateBrightnessReadouts(browser, target, brightness) {
        const readouts = browser.document.querySelectorAll("[data-lf-brightness-readout]");
        for (const readout of readouts) {
            const readoutTarget = lightingTarget(readout);
            if (readoutTarget.kind === target.kind &&
                (target.kind === "cluster" || readoutTarget.serial === target.serial)) {
                readout.textContent = brightness + "%";
            }
        }
    }

    function bindBrightnessSlider(browser, slider) {
        if (lightingExternallyOwned(slider)) {
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
        const target = lightingTarget(slider);
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
                await submitBrightnessForTarget(browser, target, brightness);
                confirmedBrightness = brightness;
                slider.dataset.lfCurrentBrightness = String(brightness);
                renderBrightness(slider, numberInput, brightness);
                updateBrightnessReadouts(browser, target, brightness);
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

    function revealEffectResetForTarget(browser, target, effect) {
        const controls = browser.document.querySelectorAll("[data-lf-reset-control]");
        for (const control of controls) {
            const controlTarget = lightingTarget(control);
            if (controlTarget.kind === target.kind &&
                (target.kind === "cluster" || controlTarget.serial === target.serial) &&
                control.dataset.lfEffect === effect) {
                control.hidden = false;
            }
        }
    }

    function revealEffectReset(browser, serial, effect) {
        revealEffectResetForTarget(browser, openRGBLightingTarget(serial), effect);
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

        if (lightingExternallyOwned(slider)) {
            return null;
        }

        let requestInFlight = false;
        let keyboardBinding;
        const target = lightingTarget(slider);
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
                await submitSpeedForTarget(browser, target, effect, mappedSpeed);
                confirmedStoredSpeed = mappedSpeed;
                slider.dataset.lfCurrentStoredSpeed = String(mappedSpeed);
                confirmedSpeed = speedHelper.storedToUiForSlider(slider, mappedSpeed);
                renderSpeed(speedHelper, slider, numberInput, confirmedSpeed);
                updateSpeedReadouts(browser, slider.dataset.lfSpeedTarget, mappedSpeed);
                revealEffectResetForTarget(browser, target, effect);
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
        if (lightingExternallyOwned(selector)) {
            return null;
        }

        const status = browser.document.getElementById(selector.dataset.lfStatusId);
        const configuredEffect = selector.dataset.lfCurrentEffect || "";
        const target = lightingTarget(selector);
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
                await submitEffectForTarget(browser, target, selectedEffect);
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

    function bindChannelLabel(browser, row) {
        const channelID = Number(row.dataset.lfChannelId);
        const deviceID = row.dataset.lfDeviceSerial;
        const label = row.querySelector("[data-lf-lighting-label]");
        const labelDisplay = row.querySelector("[data-lf-lighting-label-display]");
        const status = row.querySelector("[data-lf-lighting-label-status]");
        if (!Number.isInteger(channelID) || !deviceID || !label || !labelDisplay || typeof browser.fetch !== "function") {
            return null;
        }

        let confirmedLabel = row.dataset.lfConfirmedLabel || labelDisplay.textContent || "";
        let labelSaving = false;

        function closeLabelEditor() {
            label.hidden = true;
            labelDisplay.hidden = false;
        }

        function restoreConfirmedLabel() {
            label.value = confirmedLabel;
            labelDisplay.textContent = confirmedLabel;
            closeLabelEditor();
        }

        function openLabelEditor() {
            label.value = confirmedLabel;
            labelDisplay.hidden = true;
            label.hidden = false;
            if (typeof label.focus === "function") {
                label.focus();
            }
        }

        async function saveLabel() {
            if (labelSaving) {
                return;
            }
            const next = label.value.trim();
            if (!next || next === confirmedLabel) {
                restoreConfirmedLabel();
                return;
            }

            labelSaving = true;
            label.disabled = true;
            if (status) {
                status.textContent = "";
            }
            try {
                await submitLightingMutation(browser, "/api/label", {
                    deviceId: deviceID,
                    channelId: channelID,
                    deviceType: 1,
                    label: next
                }, effectTimeoutMilliseconds, "label request failed", "label mutation was rejected");
                confirmedLabel = next;
                row.dataset.lfConfirmedLabel = next;
                labelDisplay.textContent = next;
                closeLabelEditor();
                showOwnershipSaved(browser);
            } catch (_) {
                restoreConfirmedLabel();
                if (status) {
                    status.textContent = "Couldn’t save this lighting label.";
                }
            } finally {
                label.disabled = false;
                labelSaving = false;
            }
        }

        labelDisplay.addEventListener("click", openLabelEditor);
        label.addEventListener("blur", saveLabel);
        label.addEventListener("keydown", function (event) {
            if (event.key === "Enter") {
                event.preventDefault();
                return saveLabel();
            }
            if (event.key === "Escape") {
                event.preventDefault();
                restoreConfirmedLabel();
            }
        });
        return {open: openLabelEditor, save: saveLabel};
    }

    function bindChannelEditors(browser) {
        const toggles = Array.from(browser.document.querySelectorAll("[data-lf-channel-editor-toggle]"));
        let open = null;
        for (const toggle of toggles) {
            const editor = browser.document.getElementById(toggle.getAttribute("aria-controls"));
            if (!editor) continue;
            toggle.addEventListener("click", function () {
                if (open && open !== editor) {
                    if (open.dataset.lfEditorDirty !== "true") {
                        const openToggle = toggles.find(function (candidate) {
                            return candidate.getAttribute("aria-controls") === open.id;
                        });
                        open.hidden = true;
                        if (openToggle) {
                            openToggle.setAttribute("aria-expanded", "false");
                            openToggle.textContent = "Settings";
                        }
                        open = null;
                    } else {
                        // Explicit-save controls (notably Gradient) keep their local draft until
                        // the user intentionally closes that editor.
                    const openToggle = toggles.find(function (candidate) {
                        return candidate.getAttribute("aria-controls") === open.id;
                    });
                    if (openToggle && typeof openToggle.focus === "function") openToggle.focus();
                    return;
                    }
                }
                const expanded = editor.hidden;
                editor.hidden = !expanded;
                toggle.setAttribute("aria-expanded", expanded ? "true" : "false");
                toggle.textContent = expanded ? "Hide settings" : "Settings";
                open = expanded ? editor : null;
            });
        }
        return toggles;
    }

    function bindColorControl(browser, colorInput) {
        if (lightingExternallyOwned(colorInput)) return null;
        const hexInput = browser.document.getElementById(colorInput.dataset.lfHexId);
        const status = browser.document.getElementById(colorInput.dataset.lfStatusId);
        if (!hexInput) return null;

        let confirmedColor = colorInput.dataset.lfCurrentColor.toLowerCase();
        let requestInFlight = false;
        const target = lightingTarget(colorInput);
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
                await submitColorForTarget(browser, target, colorInput.dataset.lfEffect, normalized);
                confirmedColor = normalized;
                colorInput.dataset.lfCurrentColor = normalized;
                revealEffectResetForTarget(browser, target, colorInput.dataset.lfEffect);
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

    function bindIndexedColorEditor(browser, container) {
        const selectorButtons = Array.from(container.querySelectorAll("[data-lf-indexed-led]"));
        const picker = container.querySelector("[data-lf-indexed-picker]");
        const hex = container.querySelector("[data-lf-indexed-hex]");
        const applyAllButton = container.querySelector("[data-lf-indexed-apply-all]");
        const saveButton = container.querySelector("[data-lf-indexed-save]");
        const cancelButton = container.querySelector("[data-lf-indexed-cancel]");
        const activeLabel = container.querySelector("[data-lf-indexed-active-label]");
        const status = container.querySelector("[data-lf-indexed-status]");
        if (!picker || !hex || !applyAllButton || !saveButton || !cancelButton || !activeLabel || !status || selectorButtons.length === 0) return null;
        const normalize = function (value) { return /^#[0-9a-fA-F]{6}$/.test(value) ? value.toLowerCase() : null; };
        const indexes = selectorButtons.map(function (button) { return Number(button.dataset.lfIndex); });
        let confirmedColors = selectorButtons.map(function (button) { return normalize(button.dataset.lfCurrentColor || ""); });
        if (indexes.some(function (index, position) { return !Number.isInteger(index) || index !== position; }) || confirmedColors.some(function (color) { return color === null; })) return null;
        confirmedColors = confirmedColors.slice();
        let draftColors = confirmedColors.slice();
        let selectedIndexes = new Set();
        let referenceIndex = null;
        let currentColor = confirmedColors[0];
        let requestInFlight = false;
        const target = lightingTarget(container);
        const setStatus = createStatusController(browser, status, colorSuccessMilliseconds);

        function dirty() { return draftColors.some(function (color, index) { return color !== confirmedColors[index]; }); }
        function setEditorDirty() {
            if (typeof container.closest !== "function") return;
            const editor = container.closest("[data-lf-channel-editor]");
            if (editor) editor.dataset.lfEditorDirty = dirty() ? "true" : "false";
        }
        function render() {
            picker.value = currentColor;
            hex.value = currentColor;
            const selected = Array.from(selectedIndexes).sort(function (left, right) { return left - right; }).map(function (index) { return index + 1; });
            activeLabel.textContent = selected.length === 0 ? "Selected: None" : "Selected: " + selected.join(", ");
            selectorButtons.forEach(function (button, index) {
                const selected = selectedIndexes.has(index);
                button.style.backgroundColor = draftColors[index];
                button.setAttribute("aria-pressed", String(selected));
                button.setAttribute("aria-selected", String(selected));
                if (button.classList) button.classList.toggle("lf-indexed-led-selected", selected);
            });
            const isDirty = dirty();
            saveButton.disabled = requestInFlight || !isDirty;
            cancelButton.disabled = requestInFlight || !isDirty;
            setEditorDirty();
        }
        function setSaving(saving) {
            requestInFlight = saving;
            for (const control of selectorButtons.concat([picker, hex, applyAllButton, saveButton, cancelButton])) control.disabled = saving;
            if (!saving) render();
        }
        function updateDraft(value, revertInvalid) {
            const color = normalize(value);
            if (color === null) {
                if (revertInvalid) render();
                return;
            }
            if (requestInFlight) return;
            currentColor = color;
            for (const index of selectedIndexes) draftColors[index] = color;
            render();
        }
        function applyAll() {
            if (requestInFlight) return;
            const color = normalize(picker.value);
            if (color === null) return;
            currentColor = color;
            draftColors = draftColors.map(function () { return color; });
            render();
        }
        async function save() {
            if (requestInFlight || !dirty()) return;
            setSaving(true);
            setStatus("Saving LED colors…", false);
            try {
                await submitIndexedColorsForTarget(browser, target, draftColors.map(function (color, index) { return {index: index, color: color}; }));
                confirmedColors = draftColors.slice();
                selectorButtons.forEach(function (button, index) { button.dataset.lfCurrentColor = confirmedColors[index]; });
                setStatus("LED colors saved.", true);
            } catch (_) {
                setStatus("Unable to save LED colors. Try again.", false);
            }
            setSaving(false);
        }
        function cancel() {
            if (requestInFlight || !dirty()) return;
            draftColors = confirmedColors.slice();
            if (referenceIndex !== null && selectedIndexes.has(referenceIndex)) currentColor = draftColors[referenceIndex];
            render();
            setStatus("", false);
        }
        if (lightingExternallyOwned(container)) {
            for (const control of selectorButtons.concat([picker, hex, applyAllButton, saveButton, cancelButton])) control.disabled = true;
            return null;
        }
        for (const button of selectorButtons) button.addEventListener("click", function () {
            if (requestInFlight) return;
            const index = Number(button.dataset.lfIndex);
            if (selectedIndexes.has(index)) {
                selectedIndexes.delete(index);
                if (referenceIndex === index) {
                    referenceIndex = selectedIndexes.size > 0 ? Array.from(selectedIndexes).pop() : null;
                    if (referenceIndex !== null) currentColor = draftColors[referenceIndex];
                }
            } else {
                selectedIndexes.add(index);
                referenceIndex = index;
                currentColor = draftColors[index];
            }
            render();
        });
        picker.addEventListener("input", function (event) { updateDraft(event && event.currentTarget ? event.currentTarget.value : picker.value); });
        picker.addEventListener("change", function (event) { updateDraft(event && event.currentTarget ? event.currentTarget.value : picker.value, true); });
        hex.addEventListener("input", function () { updateDraft(hex.value); });
        hex.addEventListener("change", function () { updateDraft(hex.value, true); });
        hex.addEventListener("keydown", function (event) { if (event.key === "Enter") { event.preventDefault(); save(); } });
        applyAllButton.addEventListener("click", applyAll);
        saveButton.addEventListener("click", save);
        cancelButton.addEventListener("click", cancel);
        render();
        return {save: save, cancel: cancel, state: function () { return {confirmedColors: confirmedColors.slice(), draftColors: draftColors.slice(), selectedIndexes: Array.from(selectedIndexes).sort(function (left, right) { return left - right; }), referenceIndex: referenceIndex, currentColor: currentColor}; }};
    }

    function bindTwoColorControl(browser, container) {
        if (lightingExternallyOwned(container)) return null;

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
        const target = lightingTarget(container);
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
                await submitTwoColorForTarget(browser, target, container.dataset.lfEffect, start, end);
                confirmedStart = start;
                confirmedEnd = end;
                container.dataset.lfCurrentStart = start;
                container.dataset.lfCurrentEnd = end;
                render(start, end);
                revealEffectResetForTarget(browser, target, container.dataset.lfEffect);
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

    function bindTemperatureControl(browser, container) {
        if (lightingExternallyOwned(container)) return null;

        const roles = ["Low", "Middle", "High"];
        const points = roles.map(function (role) {
            const key = role.toLowerCase();
            return {
                color: browser.document.getElementById(container.dataset["lf" + role + "ColorId"]),
                hex: browser.document.getElementById(container.dataset["lf" + role + "HexId"]),
                celsius: browser.document.getElementById(container.dataset["lf" + role + "CelsiusId"]),
                key: key
            };
        });
        const controls = points.flatMap(function (point) { return [point.color, point.hex, point.celsius]; });
        const status = browser.document.getElementById(container.dataset.lfStatusId);
        if (controls.some(function (control) { return !control; })) return null;

        function normalizeColor(value) {
            return /^#[0-9a-fA-F]{6}$/.test(value) ? value.toLowerCase() : null;
        }

        function readState() {
            const state = {};
            for (const point of points) {
                const color = normalizeColor(point.hex.value);
                const rawCelsius = String(point.celsius.value).trim();
                const celsius = rawCelsius === "" ? NaN : Number(rawCelsius);
                if (color === null || !Number.isFinite(celsius)) return null;
                state[point.key] = {color: color, celsius: celsius};
            }
            return state;
        }

        function cloneState(state) {
            return {
                low: {color: state.low.color, celsius: state.low.celsius},
                middle: {color: state.middle.color, celsius: state.middle.celsius},
                high: {color: state.high.color, celsius: state.high.celsius}
            };
        }

        function render(state) {
            for (const point of points) {
                point.color.value = state[point.key].color;
                point.hex.value = state[point.key].color;
                point.celsius.value = String(state[point.key].celsius);
            }
        }

        let confirmed = readState();
        if (confirmed === null) {
            for (const control of controls) control.disabled = true;
            if (status) status.textContent = "Unable to load temperature settings.";
            return null;
        }
        if (!(confirmed.low.celsius < confirmed.middle.celsius && confirmed.middle.celsius < confirmed.high.celsius)) {
            for (const control of controls) control.disabled = true;
            if (status) status.textContent = temperatureOrderMessage;
            return null;
        }
        confirmed = cloneState(confirmed);
        let requestInFlight = false;
        const target = lightingTarget(container);
        const setStatus = createStatusController(browser, status, temperatureSuccessMilliseconds);

        function restoreConfirmed() {
            render(confirmed);
        }

        function equal(left, right) {
            return roles.every(function (role) {
                const key = role.toLowerCase();
                return left[key].color === right[key].color && left[key].celsius === right[key].celsius;
            });
        }

        async function commit() {
            if (requestInFlight) return;
            const next = readState();
            if (next === null) {
                restoreConfirmed();
                return;
            }
            if (!(next.low.celsius < next.middle.celsius && next.middle.celsius < next.high.celsius)) {
                restoreConfirmed();
                setStatus(temperatureOrderMessage, false);
                return;
            }
            if (equal(next, confirmed)) {
                restoreConfirmed();
                return;
            }

            requestInFlight = true;
            for (const control of controls) control.disabled = true;
            setStatus("Saving temperature colors…", false);
            try {
                await submitTemperatureForTarget(browser, target, container.dataset.lfEffect, next.low, next.middle, next.high);
                confirmed = cloneState(next);
                render(confirmed);
                revealEffectResetForTarget(browser, target, container.dataset.lfEffect);
                setStatus("Temperature colors saved.", true);
                browser.location.reload();
            } catch (_) {
                restoreConfirmed();
                setStatus(temperatureFailureMessage, false);
                for (const control of controls) control.disabled = false;
                requestInFlight = false;
            }
        }

        for (const point of points) {
            point.color.addEventListener("input", function () {
                const color = normalizeColor(point.color.value);
                if (color !== null) point.hex.value = color;
            });
            point.color.addEventListener("change", commit);
            point.hex.addEventListener("input", function () {
                const color = normalizeColor(point.hex.value);
                if (color !== null) {
                    point.color.value = color;
                    point.hex.value = color;
                }
            });
            point.hex.addEventListener("change", commit);
            point.celsius.addEventListener("change", commit);
            for (const input of [point.hex, point.celsius]) {
                input.addEventListener("keydown", function (event) {
                    if (event.key === "Enter") {
                        event.preventDefault();
                        commit();
                    }
                });
            }
        }
        return commit;
    }

    function bindGradientControl(browser, container) {
        if (lightingExternallyOwned(container)) return null;

        const rows = browser.document.getElementById(container.dataset.lfRowsId);
        const addButton = browser.document.getElementById(container.dataset.lfAddId);
        const saveButton = browser.document.getElementById(container.dataset.lfSaveId);
        const status = browser.document.getElementById(container.dataset.lfStatusId);
        if (!rows || !addButton || !saveButton || !status) return null;

        const setStatus = createStatusController(browser, status, gradientSuccessMilliseconds);
        let requestInFlight = false;
        let confirmed = null;
        const target = lightingTarget(container);

        function normalizeColor(value) {
            return /^#[0-9a-fA-F]{6}$/.test(value) ? value.toLowerCase() : null;
        }

        function controls() {
            return Array.from(container.querySelectorAll("input, button"));
        }

        function stopRows() {
            return Array.from(rows.querySelectorAll("[data-lf-gradient-stop]"));
        }

        function readDraft() {
            const currentRows = stopRows();
            if (currentRows.length < 2) return {error: "minimum"};
            const stops = [];
            for (const row of currentRows) {
                const color = normalizeColor(row.querySelector("[data-lf-gradient-hex]").value);
                const positionText = String(row.querySelector("[data-lf-gradient-position]").value).trim();
                const intensityText = String(row.querySelector("[data-lf-gradient-intensity]").value).trim();
                const position = positionText === "" ? NaN : Number(positionText);
                const intensity = intensityText === "" ? NaN : Number(intensityText);
                if (color === null || !Number.isFinite(position) || !Number.isFinite(intensity)) {
                    return {error: "complete"};
                }
                if (position < 0 || position > 1 || intensity < 0 || intensity > 1) {
                    return {error: "range"};
                }
                stops.push({position: position, color: color, intensity: intensity});
            }
            return {stops: stops};
        }

        function normalized(stops) {
            return stops.map(function (stop, index) {
                return {index: index, position: stop.position, color: stop.color, intensity: stop.intensity};
            }).sort(function (left, right) {
                return left.position - right.position || left.index - right.index;
            }).map(function (stop) {
                return {position: stop.position, color: stop.color, intensity: stop.intensity};
            });
        }

        function equal(left, right) {
            return left.length === right.length && left.every(function (stop, index) {
                const other = right[index];
                return stop.position === other.position && stop.color === other.color && stop.intensity === other.intensity;
            });
        }

        function updateButtons() {
            const draft = readDraft();
            const currentRows = stopRows();
            for (const row of currentRows) {
                row.querySelector("[data-lf-gradient-remove]").disabled = requestInFlight || currentRows.length <= 2;
            }
            addButton.disabled = requestInFlight || currentRows.length >= 1024;
            saveButton.disabled = requestInFlight || !draft.stops || equal(normalized(draft.stops), confirmed);
            if (typeof container.closest === "function") {
                const editor = container.closest("[data-lf-channel-editor]");
				if (editor) editor.dataset.lfEditorDirty = !draft.stops || !equal(normalized(draft.stops), confirmed) ? "true" : "false";
            }
        }

        function disableEditor(message) {
            for (const control of controls()) control.disabled = true;
            setStatus(message, false);
        }

        function setSaving(saving) {
            requestInFlight = saving;
            for (const control of controls()) control.disabled = saving;
            if (!saving) updateButtons();
        }

        function label(labelText, input) {
            const wrapper = browser.document.createElement("label");
            wrapper.className = "lf-gradient-field-label";
            wrapper.textContent = labelText;
            wrapper.appendChild(input);
            return wrapper;
        }

        function createInput(type, className, value, name, number) {
            const input = browser.document.createElement("input");
            input.type = type;
            input.className = className;
            input.value = String(value);
            input.id = "lf-lighting-gradient-" + name + "-" + number;
            input.setAttribute("aria-describedby", container.dataset.lfStatusId);
            return input;
        }

        function render(stops) {
            rows.replaceChildren();
            stops.forEach(function (stop, index) {
                const number = index + 1;
                const row = browser.document.createElement("div");
                row.className = "lf-gradient-stop";
                row.dataset.lfGradientStop = "";
                const heading = browser.document.createElement("h4");
                heading.dataset.lfGradientStopNumber = "";
                heading.textContent = "Stop " + number;
                row.appendChild(heading);

                const fields = browser.document.createElement("div");
                fields.className = "lf-gradient-stop-fields";
                const color = createInput("color", "lf-color-control-input", stop.color, "color", number);
                color.dataset.lfGradientColor = "";
                color.setAttribute("aria-label", "Stop " + number + " color");
                const hex = createInput("text", "lf-color-control-hex", stop.color, "hex", number);
                hex.dataset.lfGradientHex = "";
                hex.pattern = "#[0-9a-fA-F]{6}";
                hex.maxLength = 7;
                hex.setAttribute("aria-label", "Stop " + number + " color hex code");
                const colorFields = browser.document.createElement("span");
                colorFields.className = "lf-gradient-color-fields";
                colorFields.append(color, hex);
                const colorLabel = browser.document.createElement("label");
                colorLabel.className = "lf-gradient-field-label";
                colorLabel.textContent = "Color";
                colorLabel.appendChild(colorFields);
                fields.appendChild(colorLabel);

                const position = createInput("number", "lf-gradient-number-input", stop.position, "position", number);
                position.dataset.lfGradientPosition = "";
                position.min = "0";
                position.max = "1";
                position.step = "any";
                position.setAttribute("aria-label", "Stop " + number + " position from 0 start to 1 end");
                fields.appendChild(label("Position", position));

                const intensity = createInput("number", "lf-gradient-number-input", stop.intensity, "intensity", number);
                intensity.dataset.lfGradientIntensity = "";
                intensity.min = "0";
                intensity.max = "1";
                intensity.step = "any";
                intensity.setAttribute("aria-label", "Stop " + number + " relative intensity from 0 to 1");
                fields.appendChild(label("Relative intensity", intensity));

                const remove = browser.document.createElement("button");
                remove.type = "button";
                remove.className = "lf-button lf-button-secondary lf-gradient-remove";
                remove.dataset.lfGradientRemove = "";
                remove.textContent = "Remove";
                remove.setAttribute("aria-label", "Remove stop " + number);
                remove.addEventListener("click", function () {
                    if (requestInFlight || stopRows().length <= 2) return;
                    const draft = readDraft();
                    if (!draft.stops) return;
                    draft.stops.splice(index, 1);
                    render(draft.stops);
                });
                fields.appendChild(remove);
                row.appendChild(fields);
                rows.appendChild(row);

                color.addEventListener("input", function () {
                    const value = normalizeColor(color.value);
                    if (value !== null) hex.value = value;
                    updateButtons();
                });
                hex.addEventListener("input", function () {
                    const value = normalizeColor(hex.value);
                    if (value !== null) {
                        hex.value = value;
                        color.value = value;
                    }
                    updateButtons();
                });
                for (const input of [position, intensity]) input.addEventListener("input", updateButtons);
                for (const input of [hex, position, intensity]) {
                    input.addEventListener("keydown", function (event) {
                        if (event.key === "Enter") {
                            event.preventDefault();
                            save();
                        }
                    });
                }
            });
            updateButtons();
        }

        function addStop() {
            if (requestInFlight) return;
            const draft = readDraft();
            if (draft.error === "complete") {
                setStatus(gradientCompleteMessage, false);
                return;
            }
            if (draft.error === "range") {
                setStatus(gradientRangeMessage, false);
                return;
            }
            if (!draft.stops || draft.stops.length >= 1024) return;
            const ordered = normalized(draft.stops);
            let gapIndex = 0;
            let largestGap = -1;
            for (let index = 0; index < ordered.length - 1; index++) {
                const gap = ordered[index + 1].position - ordered[index].position;
                if (gap > largestGap) {
                    largestGap = gap;
                    gapIndex = index;
                }
            }
            const left = ordered[gapIndex];
            const right = ordered[gapIndex + 1];
            ordered.splice(gapIndex + 1, 0, {
                position: (left.position + right.position) / 2,
                color: left.color,
                intensity: left.intensity
            });
            render(ordered);
        }

        async function save() {
            if (requestInFlight) return;
            const draft = readDraft();
            if (draft.error === "minimum") {
                setStatus("Gradient requires at least two stops.", false);
                return;
            }
            if (draft.error === "complete") {
                setStatus(gradientCompleteMessage, false);
                return;
            }
            if (draft.error === "range") {
                setStatus(gradientRangeMessage, false);
                return;
            }
            const ordered = normalized(draft.stops);
            if (equal(ordered, confirmed)) {
                render(confirmed);
                return;
            }
            setSaving(true);
            setStatus("Saving Gradient…", false);
            try {
                await submitGradientForTarget(browser, target, container.dataset.lfEffect, ordered);
                confirmed = normalized(ordered);
                revealEffectResetForTarget(browser, target, container.dataset.lfEffect);
                setStatus("Gradient saved.", true);
                browser.location.reload();
            } catch (_) {
                setSaving(false);
                setStatus(gradientFailureMessage, false);
            }
        }

        const initial = readDraft();
        if (!initial.stops || !equal(initial.stops, normalized(initial.stops))) {
            disableEditor("Unable to load Gradient settings.");
            return null;
        }
        confirmed = normalized(initial.stops);
        for (const row of stopRows()) {
            const color = row.querySelector("[data-lf-gradient-color]");
            const hex = row.querySelector("[data-lf-gradient-hex]");
            color.addEventListener("input", function () {
                const value = normalizeColor(color.value);
                if (value !== null) hex.value = value;
                updateButtons();
            });
            hex.addEventListener("input", function () {
                const value = normalizeColor(hex.value);
                if (value !== null) {
                    hex.value = value;
                    color.value = value;
                }
                updateButtons();
            });
            for (const input of [row.querySelector("[data-lf-gradient-position]"), row.querySelector("[data-lf-gradient-intensity]")]) {
                input.addEventListener("input", updateButtons);
                input.addEventListener("keydown", function (event) {
                    if (event.key === "Enter") {
                        event.preventDefault();
                        save();
                    }
                });
            }
            row.querySelector("[data-lf-gradient-hex]").addEventListener("keydown", function (event) {
                if (event.key === "Enter") {
                    event.preventDefault();
                    save();
                }
            });
            row.querySelector("[data-lf-gradient-remove]").addEventListener("click", function () {
                if (requestInFlight || stopRows().length <= 2) return;
                const draft = readDraft();
                const index = stopRows().indexOf(row);
                if (!draft.stops || index < 0) return;
                draft.stops.splice(index, 1);
                render(draft.stops);
            });
        }
        addButton.addEventListener("click", addStop);
        saveButton.addEventListener("click", save);
        updateButtons();
        return {addStop: addStop, save: save};
    }

    function bindCCXTProbeTemperature(browser, container) {
        if (lightingExternallyOwned(container)) return null;
        const source = container.querySelector("[data-lf-probe-source]");
        const minimum = container.querySelector("[data-lf-probe-min]");
        const maximum = container.querySelector("[data-lf-probe-max]");
        const save = container.querySelector("[data-lf-probe-save]");
        const status = browser.document.getElementById(container.dataset.lfProbeStatusId);
        if (!source || !minimum || !maximum || !save) return null;
        let inFlight = false;
        save.addEventListener("click", async function () {
            if (inFlight) return;
            const probeChannelId = Number(source.value);
            const minTemp = Number(minimum.value);
            const maxTemp = Number(maximum.value);
            if (!Number.isInteger(probeChannelId) || !Number.isFinite(minTemp) || !Number.isFinite(maxTemp) || minTemp < 0 || maxTemp > 100 || maxTemp < minTemp) {
                if (status) status.textContent = "Choose a probe and a valid temperature range.";
                return;
            }
            inFlight = true;
            save.disabled = true;
            if (status) status.textContent = "Saving…";
            try {
                await submitLightingMutation(browser, ccxtProbeTemperatureEndpoint, {
                    deviceId: container.dataset.lfDeviceSerial,
                    channelId: Number(container.dataset.lfChannelId),
                    subDeviceId: 0,
                    probeChannelId: probeChannelId,
                    rgbMinTemp: minTemp,
                    rgbMaxTemp: maxTemp
                }, 10000, "Unable to save probe settings.", "Unable to save probe settings.");
                if (status) status.textContent = "";
                showOwnershipSaved(browser);
            } catch (_) {
                if (status) status.textContent = "Unable to save probe settings. Try again.";
            } finally {
                inFlight = false;
                save.disabled = false;
            }
        });
        return {save: save};
    }

    function bindCCXTThreePinPort(browser, container) {
        if (lightingExternallyOwned(container)) return null;
        const device = container.querySelector("[data-lf-three-pin-device]");
        const quantity = container.querySelector("[data-lf-three-pin-quantity]");
        const status = container.querySelector("[data-lf-three-pin-status]");
        const deviceID = container.dataset.lfDeviceId;
        if (!device || !quantity || !deviceID) return null;
        let saving = false;
        function restore(control) { control.value = control.dataset.lfConfirmedValue; }
        async function update(control, endpoint, field) {
            if (saving) return;
            const value = Number(control.value);
            if (!Number.isInteger(value) || value < 0 || value === Number(control.dataset.lfConfirmedValue)) return;
            saving = true;
            device.disabled = true;
            quantity.disabled = true;
            if (status) status.textContent = "Saving…";
            try {
                const payload = {deviceId: deviceID, portId: 0};
                payload[field] = value;
                await submitLightingMutation(browser, endpoint, payload, effectTimeoutMilliseconds, "topology request failed", "topology mutation was rejected");
                if (status) status.textContent = "";
                showOwnershipSaved(browser);
                if (browser.location && typeof browser.location.reload === "function") {
                    browser.setTimeout(function () { browser.location.reload(); }, 1500);
                }
            } catch (_) {
                restore(control);
                if (status) status.textContent = "Unable to save 3-Pin RGB Port settings. Try again.";
                device.disabled = false;
                quantity.disabled = quantity.dataset.lfInitiallyDisabled === "true";
                saving = false;
            }
        }
        quantity.dataset.lfInitiallyDisabled = quantity.disabled ? "true" : "false";
        device.addEventListener("change", function() { return update(device, "/api/hub/type", "deviceType"); });
        quantity.addEventListener("change", function() { return update(quantity, "/api/hub/amount", "deviceAmount"); });
        return {device: device, quantity: quantity};
    }

    function bindManualRGBPorts(browser, container) {
        if (lightingExternallyOwned(container)) return null;
        const deviceID = container.dataset.lfDeviceId;
        const status = container.querySelector("[data-lf-manual-rgb-status]");
        const controls = Array.from(container.querySelectorAll("[data-lf-manual-rgb-device]"));
        if (!deviceID || controls.length === 0) return null;
        let saving = false;
        async function update(control) {
            if (saving || control.disabled) return;
            const portId = Number(control.dataset.lfPortId);
            const deviceType = Number(control.value);
            const confirmed = control.dataset.lfConfirmedValue;
            if (!Number.isInteger(portId) || portId < 1 || !Number.isInteger(deviceType) || String(deviceType) === confirmed) return;
            saving = true;
            for (const item of controls) item.disabled = true;
            if (status) status.textContent = "Saving…";
            try {
                await submitLightingMutation(browser, "/api/argb", {deviceId: deviceID, portId: portId, deviceType: deviceType}, effectTimeoutMilliseconds, "manual RGB device request failed", "manual RGB device mutation was rejected");
                if (status) status.textContent = "";
                showOwnershipSaved(browser);
                if (browser.location && typeof browser.location.reload === "function") browser.setTimeout(function () { browser.location.reload(); }, 1500);
            } catch (_) {
                control.value = confirmed;
                if (status) status.textContent = "Unable to save Custom RGB Device. Try again.";
                for (const item of controls) item.disabled = false;
                saving = false;
            }
        }
        for (const control of controls) control.addEventListener("change", function () { return update(control); });
        return {controls: controls};
    }

    function bindResetButton(browser, button) {
        if (lightingExternallyOwned(button)) return null;
        const status = browser.document.getElementById(button.dataset.lfStatusId);
        if (!status) return null;

        let requestInFlight = false;
        const target = lightingTarget(button);
        const setStatus = createStatusController(browser, status, resetSuccessMilliseconds);

        async function handleClick() {
            if (requestInFlight) return;
            requestInFlight = true;
            button.disabled = true;
            setStatus("Resetting effect…", false);
            try {
                await submitResetForTarget(browser, target, button.dataset.lfEffect);
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

    function bindAuthoredZoneControl(browser, control) {
        const clusterControlled = lightingExternallyOwned(control);
        const status = browser.document.getElementById(control.dataset.lfStatusId);
        const colorInput = control.querySelector("[data-lf-authored-zone-color]");
        const hexInput = control.querySelector("[data-lf-authored-zone-hex]");
        const zones = control.querySelectorAll("[data-lf-authored-zone]");
        const actions = control.querySelectorAll("[data-lf-authored-zone-apply]");
        const clearSelection = control.querySelector("[data-lf-authored-zone-clear]");
        if (clusterControlled || !status || !colorInput || !hexInput || !clearSelection || zones.length === 0) return null;
        const selected = new Set();
        let inFlight = false;
        const target = lightingTarget(control);
        const setStatus = createStatusController(browser, status, colorSuccessMilliseconds);
        function selectedZones() { return Array.from(selected); }
        function selectedGroup() {
            const values = selectedZones();
            if (values.length === 0) return "";
            const groupID = values[0].dataset.lfGroupId;
            return groupID && values.every(function (zone) { return zone.dataset.lfGroupId === groupID; }) ? groupID : "";
        }
        function updateActions() {
            const count = selected.size;
            const groupID = selectedGroup();
            clearSelection.disabled = inFlight || count === 0;
            for (const action of actions) {
                if (inFlight) { action.disabled = true; continue; }
                const scope = action.dataset.lfAuthoredZoneApply;
                action.disabled = (scope === "zones" && count === 0) || (scope === "group" && !groupID);
            }
        }
        function toggle(zone) {
            if (selected.has(zone)) selected.delete(zone); else selected.add(zone);
            const isSelected = selected.has(zone);
            zone.classList.toggle("lf-authored-zone-selected", isSelected);
            zone.setAttribute("aria-pressed", String(isSelected));
            const swatch = zone.querySelector("span:last-child");
            const value = swatch && swatch.textContent;
            if (isSelected && selected.size === 1 && /^#[0-9a-fA-F]{6}$/.test(value || "")) { colorInput.value = value; hexInput.value = value; }
            updateActions();
        }
        for (const zone of zones) zone.addEventListener("click", function () { toggle(zone); });
        clearSelection.addEventListener("click", function () {
            if (inFlight || selected.size === 0) return;
            selected.clear();
            for (const zone of zones) {
                zone.classList.toggle("lf-authored-zone-selected", false);
                zone.setAttribute("aria-pressed", "false");
            }
            updateActions();
        });
        colorInput.addEventListener("input", function () { hexInput.value = colorInput.value; });
        hexInput.addEventListener("change", function () { if (/^#[0-9a-fA-F]{6}$/.test(hexInput.value)) colorInput.value = hexInput.value; });
        for (const action of actions) action.addEventListener("click", async function () {
            if (inFlight || !/^#[0-9a-fA-F]{6}$/.test(colorInput.value)) return;
            const scope = action.dataset.lfAuthoredZoneApply;
            const selectedValues = selectedZones();
            const groupID = selectedGroup();
            if ((scope === "zones" && selectedValues.length === 0) || (scope === "group" && !groupID)) return;
            inFlight = true;
            clearSelection.disabled = true;
            for (const button of actions) button.disabled = true;
            setStatus("Applying color…", false);
            try {
                const color = colorInput.value.toLowerCase();
                const values = {effect: control.dataset.lfEffect, scope: scope, color: color};
                if (scope === "zones") values.zoneIds = selectedValues.map(function (zone) { return zone.dataset.lfZoneId; });
                if (scope === "group") values.groupId = groupID;
                await submitLightingMutation(browser, target.endpoints.zones, lightingPayload(target, values), colorTimeoutMilliseconds, "authored-zone request failed", "authored-zone mutation was rejected");
                const affectedZones = scope === "zones" ? selectedValues : scope === "group" ? Array.from(zones).filter(function (zone) { return zone.dataset.lfGroupId === groupID; }) : Array.from(zones);
                for (const zone of affectedZones) {
                    const swatch = zone.querySelector("span:last-child");
                    if (!swatch) continue;
                    swatch.textContent = color;
                    if (swatch.style) swatch.style.backgroundColor = color;
                }
                setStatus("Color applied.", true);
                inFlight = false;
                updateActions();
            } catch (_) {
                setStatus("Unable to apply authored color. Try again.", false);
                inFlight = false;
                updateActions();
            }
        });
        updateActions();
        return {toggle: toggle};
    }

    function init(browser) {
        function isReadOnly(control) {
            if (typeof control.closest !== "function") {
                return false;
            }
            const workspace = control.closest("[data-lf-lighting-read-only]");
            return workspace && workspace.dataset.lfLightingReadOnly === "true";
        }
        const selectors = browser.document.querySelectorAll("[data-lf-effect-selector]");
        for (const selector of selectors) {
            if (!isReadOnly(selector)) {
                bindEffectSelector(browser, selector);
            }
        }
        const channelLabels = browser.document.querySelectorAll("[data-lf-lighting-channel]");
        for (const channel of channelLabels) {
            if (!isReadOnly(channel)) {
                bindChannelLabel(browser, channel);
            }
        }
        bindChannelEditors(browser);
        const sliders = browser.document.querySelectorAll("[data-lf-brightness-slider]");
        for (const slider of sliders) {
            if (!isReadOnly(slider)) {
                bindBrightnessSlider(browser, slider);
            }
        }
        const speedSliders = browser.document.querySelectorAll("[data-lf-speed-slider]");
        for (const slider of speedSliders) {
            if (!isReadOnly(slider)) {
                bindSpeedSlider(browser, slider);
            }
        }
        const colorInputs = browser.document.querySelectorAll("[data-lf-color-input]");
        for (const input of colorInputs) {
            if (!isReadOnly(input)) {
                bindColorControl(browser, input);
            }
        }
        const indexedEditors = browser.document.querySelectorAll("[data-lf-indexed-editor]");
        for (const editor of indexedEditors) {
            if (!isReadOnly(editor)) bindIndexedColorEditor(browser, editor);
        }
        const twoColorControls = browser.document.querySelectorAll("[data-lf-two-color-control]");
        for (const control of twoColorControls) {
            if (!isReadOnly(control)) {
                bindTwoColorControl(browser, control);
            }
        }
        const temperatureControls = browser.document.querySelectorAll("[data-lf-temperature-control]");
        for (const control of temperatureControls) {
            if (!isReadOnly(control)) {
                bindTemperatureControl(browser, control);
            }
        }
        const gradientControls = browser.document.querySelectorAll("[data-lf-gradient-control]");
        for (const control of gradientControls) {
            if (!isReadOnly(control)) {
                bindGradientControl(browser, control);
            }
        }
        const resetButtons = browser.document.querySelectorAll("[data-lf-reset-button]");
        for (const button of resetButtons) {
            if (!isReadOnly(button)) {
                bindResetButton(browser, button);
            }
        }
        const probeTemperatureControls = browser.document.querySelectorAll("[data-lf-ccxt-probe-temperature]");
        for (const control of probeTemperatureControls) {
            if (!isReadOnly(control)) bindCCXTProbeTemperature(browser, control);
        }
        const threePinPorts = browser.document.querySelectorAll("[data-lf-ccxt-three-pin-port]");
        for (const port of threePinPorts) {
            if (!isReadOnly(port)) bindCCXTThreePinPort(browser, port);
        }
        const manualRGBPorts = browser.document.querySelectorAll("[data-lf-manual-rgb-ports]");
        for (const ports of manualRGBPorts) {
            if (!isReadOnly(ports)) bindManualRGBPorts(browser, ports);
        }

        const authoredZoneControls = browser.document.querySelectorAll("[data-lf-authored-zone-control]");
        for (const control of authoredZoneControls) {
            if (!isReadOnly(control)) bindAuthoredZoneControl(browser, control);
        }

        const ownershipControls = Array.from(browser.document.querySelectorAll("[data-lf-lighting-ownership-control]"));
        updateOwnershipToggleAvailability(ownershipControls);
        for (const control of ownershipControls) {
            bindOwnershipToggle(browser, control, ownershipControls);
        }
    }

    return {
        bindBrightnessSlider,
        bindChannelLabel,
        bindChannelEditors,
        bindEffectSelector,
        bindSpeedSlider,
        bindColorControl,
        bindIndexedColorEditor,
        bindTwoColorControl,
        bindTemperatureControl,
        bindGradientControl,
        bindCCXTProbeTemperature,
        bindCCXTThreePinPort,
        bindManualRGBPorts,
        bindResetButton,
        bindAuthoredZoneControl,
        bindOwnershipToggle,
        lightingTarget,
        lightingPayload,
        updateOwnershipToggleAvailability,
        disableLocalLightingControls,
        revealEffectReset,
        brightnessEndpoint,
        effectEndpoint,
        colorEndpoint,
        twoColorEndpoint,
        temperatureEndpoint,
        gradientEndpoint,
        resetEndpoint,
        init,
        submitBrightness,
        submitEffect,
        submitSpeed,
        submitColor,
        submitTwoColor,
        submitTemperature,
        submitGradient,
        submitReset,
        speedEndpoint
    };
});
