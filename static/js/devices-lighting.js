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
        slider.value = String(brightness);
        slider.setAttribute("aria-valuetext", brightness + " percent");
        slider.style.setProperty("--lf-range-progress", brightness + "%");
        if (numberInput) {
            numberInput.value = String(brightness);
        }
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
        let successTimer = null;
        let statusGeneration = 0;

        function setStatus(message, clearAfterSuccess) {
            statusGeneration += 1;
            const generation = statusGeneration;
            if (successTimer !== null) {
                browser.clearTimeout(successTimer);
                successTimer = null;
            }
            if (status) {
                status.textContent = message;
            }
            if (status && clearAfterSuccess) {
                successTimer = browser.setTimeout(function () {
                    if (generation === statusGeneration) {
                        status.textContent = "";
                        successTimer = null;
                    }
                }, brightnessSuccessMilliseconds);
            }
        }

        function previewBrightness(value) {
            const brightness = parseBrightness(value);
            if (brightness === null) {
                return;
            }
            setStatus("", false);
            renderBrightness(slider, numberInput, brightness);
        }

        async function commitBrightness(value) {
            const brightness = parseBrightness(value);
            if (requestInFlight) {
                return;
            }
            if (brightness === null) {
                renderBrightness(slider, numberInput, confirmedBrightness);
                return;
            }
            setStatus("", false);
            if (brightness === confirmedBrightness) {
                renderBrightness(slider, numberInput, confirmedBrightness);
                return;
            }

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
                renderBrightness(slider, numberInput, confirmedBrightness);
                setStatus(brightnessFailureMessage, false);
            } finally {
                slider.disabled = false;
                numberInput.disabled = false;
                requestInFlight = false;
            }
        }

        function handleRangeInput() {
            previewBrightness(slider.value);
        }

        function handleRangeChange() {
            return commitBrightness(slider.value);
        }

        function handleNumberInput() {
            previewBrightness(numberInput.value);
        }

        function handleNumberChange() {
            return commitBrightness(numberInput.value);
        }

        function handleNumberBlur() {
            return commitBrightness(numberInput.value);
        }

        function handleNumberKeydown(event) {
            if (event.key !== "Enter") {
                return;
            }
            event.preventDefault();
            return commitBrightness(numberInput.value);
        }

        slider.addEventListener("input", handleRangeInput);
        slider.addEventListener("change", handleRangeChange);
        numberInput.addEventListener("input", handleNumberInput);
        numberInput.addEventListener("change", handleNumberChange);
        numberInput.addEventListener("blur", handleNumberBlur);
        numberInput.addEventListener("keydown", handleNumberKeydown);
        return {
            handleChange: handleRangeChange,
            handleInput: handleRangeInput,
            handleNumberBlur,
            handleNumberChange,
            handleNumberInput,
            handleNumberKeydown
        };
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

    function init(browser) {
        const selectors = browser.document.querySelectorAll("[data-lf-effect-selector]");
        for (const selector of selectors) {
            bindEffectSelector(browser, selector);
        }
        const sliders = browser.document.querySelectorAll("[data-lf-brightness-slider]");
        for (const slider of sliders) {
            bindBrightnessSlider(browser, slider);
        }
    }

    return {
        bindBrightnessSlider,
        bindEffectSelector,
        brightnessEndpoint,
        effectEndpoint,
        init,
        submitBrightness,
        submitEffect
    };
});
