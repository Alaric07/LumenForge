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
    }

    return {bindEffectSelector, effectEndpoint, init, submitEffect};
});
