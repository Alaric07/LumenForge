"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const lighting = require("./devices-lighting.js");
const rgbSpeed = require("./rgb-speed.js");

function selectorFixture(overrides) {
    let changeHandler;
    const selector = {
        dataset: {
            lfClusterControlled: "false",
            lfCurrentEffect: "static",
            lfDeviceSerial: "openrgb-test-device",
            lfStatusId: "effect-status"
        },
        disabled: false,
        value: "wave",
        addEventListener: function (event, handler) {
            assert.equal(event, "change");
            changeHandler = handler;
        }
    };
    Object.assign(selector, overrides || {});
    return {selector, handler: function () { return changeHandler; }};
}

function timerFixture() {
    let nextID = 1;
    const scheduled = new Map();
    const history = [];
    const delays = [];
    const cleared = [];
    return {
        setTimeout: function (callback, delay) {
            const id = nextID++;
            const timer = {callback, delay, id};
            scheduled.set(id, timer);
            history.push(timer);
            delays.push(delay);
            return id;
        },
        clearTimeout: function (id) {
            cleared.push(id);
            scheduled.delete(id);
        },
        fireNext: function (delay) {
            const entry = Array.from(scheduled.entries()).find(function (item) {
                return delay === undefined || item[1].delay === delay;
            });
            assert.ok(entry, "expected a pending timeout" + (delay === undefined ? "" : " at " + delay + "ms"));
            scheduled.delete(entry[0]);
            return entry[1].callback();
        },
        callbackForDelay: function (delay) {
            let timer;
            for (let index = history.length - 1; index >= 0; index--) {
                if (history[index].delay === delay) {
                    timer = history[index];
                    break;
                }
            }
            assert.ok(timer, "expected a recorded timeout at " + delay + "ms");
            return timer.callback;
        },
        delays,
        cleared,
        pending: function (delay) {
            if (delay === undefined) return scheduled.size;
            return Array.from(scheduled.values()).filter(function (timer) { return timer.delay === delay; }).length;
        }
    };
}

function resetVisibilityFixture(effect) {
    return {
        dataset: {
            lfDeviceSerial: "openrgb-test-device",
            lfEffect: effect || "circle"
        },
        hidden: true
    };
}

function browserFixture(fetchImplementation, overrides) {
    const status = {textContent: ""};
    let reloads = 0;
    const browser = {
        AbortController,
        clearTimeout,
        document: {
            getElementById: function (id) {
                assert.equal(id, "effect-status");
                return status;
            }
        },
        fetch: fetchImplementation,
        location: {reload: function () { reloads++; }},
        setTimeout
    };
    Object.assign(browser, overrides || {});
    return {
        browser,
        status,
        reloads: function () { return reloads; }
    };
}

function brightnessSliderFixture(overrides) {
    const handlers = {};
    const attributes = {};
    const styleValues = {};
    const slider = {
        dataset: {
            lfClusterControlled: "false",
            lfCurrentBrightness: "40",
            lfDeviceSerial: "openrgb-test-device",
            lfNumberId: "brightness-number",
            lfStatusId: "brightness-status"
        },
        disabled: false,
        value: "40",
        addEventListener: function (event, handler) {
            handlers[event] = handler;
        },
        setAttribute: function (name, value) {
            attributes[name] = value;
        },
        style: {
            setProperty: function (name, value) {
                styleValues[name] = value;
            }
        }
    };
    Object.assign(slider, overrides || {});
    return {attributes, handlers, slider, styleValues};
}

function brightnessBrowserFixture(fetchImplementation, overrides) {
    const numberHandlers = {};
    const numberInput = {
        disabled: false,
        value: "40",
        addEventListener: function (event, handler) {
            numberHandlers[event] = handler;
        }
    };
    const hero = {
        dataset: {lfDeviceSerial: "openrgb-test-device"},
        textContent: "40%"
    };
    const secondReadout = {
        dataset: {lfDeviceSerial: "openrgb-test-device"},
        textContent: "40%"
    };
    const unrelatedReadout = {
        dataset: {lfDeviceSerial: "openrgb-other-device"},
        textContent: "75%"
    };
    const status = {textContent: ""};
    const resetControl = resetVisibilityFixture();
    let reloads = 0;
    const browser = {
        AbortController,
        clearTimeout,
        document: {
            getElementById: function (id) {
                if (id === "brightness-number") return numberInput;
                if (id === "brightness-status") return status;
                assert.fail("unexpected element id " + id);
            },
            querySelectorAll: function (selector) {
                if (selector === "[data-lf-brightness-readout]") return [hero, secondReadout, unrelatedReadout];
                if (selector === "[data-lf-reset-control]") return [resetControl];
                assert.fail("unexpected selector " + selector);
            }
        },
        fetch: fetchImplementation,
        location: {reload: function () { reloads++; }},
        setTimeout
    };
    Object.assign(browser, overrides || {});
    return {
        browser,
        hero,
        numberHandlers,
        numberInput,
        reloads: function () { return reloads; },
        resetControl,
        secondReadout,
        status,
        unrelatedReadout
    };
}

function speedSliderFixture(overrides) {
    const handlers = {};
    const attributes = {};
    const styleValues = {};
    const readyClasses = [];
    const slider = {
        dataset: {
            lfClusterControlled: "false",
            lfCurrentStoredSpeed: "2",
            lfDeviceSerial: "openrgb-test-device",
            lfEffect: "circle",
            lfNumberId: "speed-number",
            lfSpeedControlMode: "software",
            lfSpeedTarget: "base",
            lfStatusId: "speed-status"
        },
        disabled: false,
        max: "10",
        min: "1",
        step: "0.1",
        value: "2",
        addEventListener: function (event, handler) { handlers[event] = handler; },
        closest: function (selector) {
            assert.equal(selector, "[data-lf-speed-control]");
            return {classList: {add: function (name) { readyClasses.push(name); }}};
        },
        setAttribute: function (name, value) { attributes[name] = value; },
        style: {setProperty: function (name, value) { styleValues[name] = value; }}
    };
    Object.assign(slider, overrides || {});
    return {attributes, handlers, readyClasses, slider, styleValues};
}

function speedBrowserFixture(fetchImplementation, overrides) {
    const numberHandlers = {};
    const numberInput = {
        disabled: false,
        value: "",
        addEventListener: function (event, handler) { numberHandlers[event] = handler; }
    };
    const readouts = {
        base: {dataset: {lfSpeedReadout: "base"}, textContent: "2"},
        override: {dataset: {lfSpeedReadout: "override"}, textContent: "8"},
        effective: {dataset: {lfSpeedReadout: "effective"}, textContent: "2"},
        empty: {dataset: {lfSpeedReadout: ""}, textContent: "4"}
    };
    const status = {textContent: ""};
    const resetControl = resetVisibilityFixture();
    const browser = {
        AbortController,
        LumenForgeRgbSpeed: rgbSpeed,
        clearTimeout,
        document: {
            getElementById: function (id) {
                if (id === "speed-number") return numberInput;
                if (id === "speed-status") return status;
                assert.fail("unexpected element id " + id);
            },
            querySelectorAll: function (selector) {
                if (selector === "[data-lf-speed-readout]") return [readouts.base, readouts.override, readouts.effective, readouts.empty];
                if (selector === "[data-lf-reset-control]") return [resetControl];
                assert.fail("unexpected selector " + selector);
            }
        },
        fetch: fetchImplementation,
        setTimeout
    };
    Object.assign(browser, overrides || {});
    return {browser, numberHandlers, numberInput, readouts, resetControl, status};
}

test("effect selector sends the established protected mutation contract and reloads", async function () {
    let request;
    const timers = timerFixture();
    const fixture = browserFixture(async function (url, options) {
        request = {url, options};
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {
        clearTimeout: timers.clearTimeout,
        setTimeout: timers.setTimeout
    });
    const control = selectorFixture();
    lighting.bindEffectSelector(fixture.browser, control.selector);

    await control.handler()();

    assert.equal(request.url, "/api/openrgbimport/effect");
    assert.equal(request.options.method, "POST");
    assert.deepEqual(JSON.parse(request.options.body), {
        serial: "openrgb-test-device",
        effect: "wave"
    });
    assert.ok(request.options.signal instanceof AbortSignal);
    assert.deepEqual(timers.delays, [10000]);
    assert.equal(timers.cleared.length, 1);
    assert.equal(timers.pending(), 0);
    assert.equal(control.selector.disabled, true);
    assert.equal(fixture.reloads(), 1);
    assert.equal(fixture.status.textContent, "");
});

test("effect selector restores its server-rendered value after application failure", async function () {
    const timers = timerFixture();
    const fixture = browserFixture(async function () {
        return {ok: true, json: async function () { return {status: 0, message: "internal detail"}; }};
    }, {
        clearTimeout: timers.clearTimeout,
        setTimeout: timers.setTimeout
    });
    const control = selectorFixture();
    lighting.bindEffectSelector(fixture.browser, control.selector);

    await control.handler()();

    assert.equal(control.selector.value, "static");
    assert.equal(control.selector.disabled, false);
    assert.equal(fixture.reloads(), 0);
    assert.equal(fixture.status.textContent, "Unable to change effect. Try again.");
    assert.doesNotMatch(fixture.status.textContent, /internal detail/);
    assert.equal(timers.cleared.length, 1);
    assert.equal(timers.pending(), 0);
});

test("effect selector aborts a stalled request and permits a later genuine change", async function () {
    const timers = timerFixture();
    let requests = 0;
    let stalledSignal;
    const fixture = browserFixture(function (_, options) {
        requests++;
        if (requests > 1) {
            return Promise.resolve({ok: true, json: async function () { return {status: 1}; }});
        }
        stalledSignal = options.signal;
        return new Promise(function (_, reject) {
            options.signal.addEventListener("abort", function () {
                const error = new Error("internal abort detail");
                error.name = "AbortError";
                reject(error);
            }, {once: true});
        });
    }, {
        clearTimeout: timers.clearTimeout,
        setTimeout: timers.setTimeout
    });
    const control = selectorFixture();
    lighting.bindEffectSelector(fixture.browser, control.selector);

    const stalledRequest = control.handler()();
    assert.equal(requests, 1);
    assert.ok(stalledSignal instanceof AbortSignal);
    assert.equal(stalledSignal.aborted, false);
    assert.deepEqual(timers.delays, [10000]);

    timers.fireNext();
    await stalledRequest;

    assert.equal(stalledSignal.aborted, true);
    assert.equal(requests, 1);
    assert.equal(control.selector.value, "static");
    assert.equal(control.selector.disabled, false);
    assert.equal(fixture.reloads(), 0);
    assert.equal(fixture.status.textContent, "Unable to change effect. Try again.");
    assert.doesNotMatch(fixture.status.textContent, /abort|internal/i);
    assert.equal(timers.cleared.length, 1);
    assert.equal(timers.pending(), 0);

    await Promise.resolve();
    assert.equal(requests, 1, "timeout triggered an automatic retry");

    control.selector.value = "off";
    await control.handler()();
    assert.equal(requests, 2);
    assert.equal(fixture.reloads(), 1);
    assert.equal(timers.cleared.length, 2);
    assert.equal(timers.pending(), 0);
});

test("effect selector handles HTTP, parse, and network failures without navigation", async function (t) {
    const failures = [
        async function () { return {ok: false, json: async function () { return {status: 1}; }}; },
        async function () { return {ok: true, json: async function () { throw new Error("bad json"); }}; },
        async function () { throw new Error("offline"); }
    ];
    for (const [index, failure] of failures.entries()) {
        await t.test(String(index), async function () {
            const timers = timerFixture();
            const fixture = browserFixture(failure, {
                clearTimeout: timers.clearTimeout,
                setTimeout: timers.setTimeout
            });
            const control = selectorFixture();
            lighting.bindEffectSelector(fixture.browser, control.selector);
            await control.handler()();
            assert.equal(control.selector.value, "static");
            assert.equal(control.selector.disabled, false);
            assert.equal(fixture.reloads(), 0);
            assert.equal(fixture.status.textContent, "Unable to change effect. Try again.");
            assert.equal(timers.cleared.length, 1);
            assert.equal(timers.pending(), 0);
        });
    }
});

test("effect selector ignores unchanged, empty, cluster-owned, and in-flight changes", async function () {
    let requests = 0;
    let releaseRequest;
    const requestPending = new Promise(function (resolve) { releaseRequest = resolve; });
    const fixture = browserFixture(async function () {
        requests++;
        await requestPending;
        return {ok: true, json: async function () { return {status: 1}; }};
    });

    const unchanged = selectorFixture({value: "static"});
    lighting.bindEffectSelector(fixture.browser, unchanged.selector);
    await unchanged.handler()();

    const empty = selectorFixture({value: ""});
    lighting.bindEffectSelector(fixture.browser, empty.selector);
    await empty.handler()();

    const cluster = selectorFixture({
        dataset: Object.assign({}, selectorFixture().selector.dataset, {lfClusterControlled: "true"}),
        disabled: true
    });
    assert.equal(lighting.bindEffectSelector(fixture.browser, cluster.selector), null);
    assert.equal(cluster.handler(), undefined);
    assert.equal(requests, 0, "cluster-owned selector submitted a local mutation");

    const active = selectorFixture();
    lighting.bindEffectSelector(fixture.browser, active.selector);
    const first = active.handler()();
    await active.handler()();
    assert.equal(requests, 1);
    releaseRequest();
    await first;

    assert.equal(requests, 1);
    assert.equal(fixture.reloads(), 1);
});

test("brightness range and number inputs preview one synchronized value without changing confirmed readouts", function () {
    let requests = 0;
    const timers = timerFixture();
    const fixture = brightnessBrowserFixture(async function () {
        requests++;
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = brightnessSliderFixture();
    lighting.bindBrightnessSlider(fixture.browser, control.slider);
    assert.equal(fixture.resetControl.hidden, true);
    assert.deepEqual(timers.delays, [], "binding scheduled status cleanup before any mutation");

    control.slider.value = "73";
    control.handlers.input();
    assert.equal(fixture.numberInput.value, "73");
    assert.equal(control.styleValues["--lf-range-progress"], "73%");
    assert.equal(control.attributes["aria-valuetext"], "73 percent");

    fixture.numberInput.value = "26";
    fixture.numberHandlers.input();
    assert.equal(control.slider.value, "26");
    assert.equal(control.styleValues["--lf-range-progress"], "26%");

    fixture.numberInput.value = "";
    fixture.numberHandlers.input();
    assert.equal(fixture.numberInput.value, "", "temporary empty numeric input was forced to zero");
    assert.equal(control.slider.value, "26");
    assert.equal(requests, 0);
    assert.equal(fixture.hero.textContent, "40%");
    assert.equal(fixture.secondReadout.textContent, "40%");
    assert.equal(control.slider.dataset.lfCurrentBrightness, "40");
    assert.deepEqual(timers.delays, [], "preview interaction scheduled status cleanup");
});

test("Brightness keyboard editing preserves raw integer text until commit or restoration", async function () {
    const timers = timerFixture();
    let requests = 0;
    const fixture = brightnessBrowserFixture(async function () {
        requests++;
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = brightnessSliderFixture();
    lighting.bindBrightnessSlider(fixture.browser, control.slider);

    let rawValue = fixture.numberInput.value;
    let valueAssignments = 0;
    Object.defineProperty(fixture.numberInput, "value", {
        configurable: true,
        get: function () { return rawValue; },
        set: function (value) {
            valueAssignments++;
            rawValue = String(value);
        }
    });
    fixture.numberInput.selectionStart = 0;
    fixture.numberInput.selectionEnd = 2;

    rawValue = "5";
    fixture.numberHandlers.input();
    assert.equal(rawValue, "5");
    assert.equal(valueAssignments, 0, "valid input preview rewrote the Brightness field");
    assert.equal(fixture.numberInput.selectionStart, 0);
    assert.equal(fixture.numberInput.selectionEnd, 2);
    assert.equal(control.slider.value, "5");
    assert.equal(control.attributes["aria-valuetext"], "5 percent");
    assert.equal(requests, 0);

    rawValue = "50";
    fixture.numberInput.selectionStart = 2;
    fixture.numberInput.selectionEnd = 2;
    fixture.numberHandlers.input();
    assert.equal(rawValue, "50");
    assert.equal(valueAssignments, 0, "continued integer input rewrote the Brightness field");
    assert.equal(fixture.numberInput.selectionStart, 2);
    assert.equal(fixture.numberInput.selectionEnd, 2);
    assert.equal(control.slider.value, "50");
    assert.doesNotMatch(rawValue, /\.0$/);
    assert.equal(requests, 0);

    rawValue = "";
    fixture.numberHandlers.input();
    assert.equal(rawValue, "");
    assert.equal(valueAssignments, 0, "temporary empty input was rewritten");
    rawValue = "invalid";
    fixture.numberHandlers.input();
    assert.equal(rawValue, "invalid");
    assert.equal(valueAssignments, 0, "malformed input was rewritten before correction");
    await fixture.numberHandlers.blur();
    assert.equal(rawValue, "40");
    assert.doesNotMatch(rawValue, /\.0$/);
    assert.equal(control.slider.value, "40");
    assert.equal(requests, 0);
});

test("Brightness keyboard editing coalesces numeric arrows and keeps timers isolated", async function () {
    const timers = timerFixture();
    const submittedBrightness = [];
    const submittedSpeed = [];
    const fixture = brightnessBrowserFixture(async function (_, options) {
        submittedBrightness.push(JSON.parse(options.body).brightness);
        fixture.browser.document.activeElement = fixture.browser.document.body;
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = brightnessSliderFixture();
    fixture.browser.document.body = {};
    fixture.browser.document.activeElement = fixture.numberInput;
    let focusCalls = 0;
    let restoredSelection;
    fixture.numberInput.focus = function () {
        focusCalls++;
        fixture.browser.document.activeElement = fixture.numberInput;
    };
    fixture.numberInput.selectionStart = 2;
    fixture.numberInput.selectionEnd = 2;
    fixture.numberInput.setSelectionRange = function (start, end) {
        restoredSelection = [start, end];
    };
    lighting.bindBrightnessSlider(fixture.browser, control.slider);

    fixture.numberHandlers.keydown({key: "ArrowUp"});
    fixture.numberInput.value = "41";
    fixture.numberHandlers.input();
    fixture.numberHandlers.change();
    assert.equal(timers.pending(400), 1);
    assert.deepEqual(submittedBrightness, []);

    const staleCommit = timers.callbackForDelay(400);
    fixture.numberHandlers.keydown({key: "ArrowUp"});
    fixture.numberInput.value = "42";
    fixture.numberHandlers.input();
    fixture.numberHandlers.change();
    assert.equal(timers.pending(400), 1, "repeated Brightness arrow did not reset the idle timer");
    await staleCommit();
    assert.deepEqual(submittedBrightness, [], "stale Brightness timer submitted an older value");

    await timers.fireNext(400);
    assert.deepEqual(submittedBrightness, [42]);
    assert.equal(focusCalls, 1);
    assert.equal(fixture.browser.document.activeElement, fixture.numberInput);
    assert.deepEqual(restoredSelection, [2, 2]);
    assert.equal(timers.pending(1800), 1);

    fixture.numberHandlers.keydown({key: "ArrowDown"});
    assert.equal(timers.pending(400), 1);
    assert.equal(timers.pending(1800), 1, "keyboard scheduling cleared Brightness success feedback");
    fixture.numberInput.value = "41";
    fixture.numberHandlers.input();
    fixture.numberHandlers.change();
    fixture.numberHandlers.keydown({key: "ArrowDown"});
    fixture.numberInput.value = "40";
    fixture.numberHandlers.input();
    fixture.numberHandlers.change();
    assert.equal(control.slider.value, "40");
    assert.equal(timers.pending(400), 1, "repeated ArrowDown did not reset the Brightness timer");
    assert.equal(timers.pending(1800), 0, "valid preview did not clear stale success feedback");

    const speedFixture = speedBrowserFixture(async function (_, options) {
        submittedSpeed.push(JSON.parse(options.body).speed);
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const speed = speedSliderFixture();
    lighting.bindSpeedSlider(speedFixture.browser, speed.slider);
    speed.handlers.keydown({key: "ArrowUp"});
    speed.slider.value = "9.1";
    speed.handlers.input();
    speed.handlers.change();
    assert.equal(timers.pending(400), 2, "Brightness and Speed did not retain independent idle timers");

    await timers.fireNext(400);
    assert.deepEqual(submittedBrightness, [42, 40]);
    assert.deepEqual(submittedSpeed, []);
    await timers.fireNext(400);
    assert.deepEqual(submittedSpeed, [1.9]);
});

test("Brightness keyboard editing immediate numeric commits cancel idle work and respect focus", async function (t) {
    await t.test("Enter commits once and restores keyboard focus", async function () {
        const timers = timerFixture();
        let requests = 0;
        const fixture = brightnessBrowserFixture(async function () {
            requests++;
            fixture.browser.document.activeElement = fixture.browser.document.body;
            return {ok: true, json: async function () { return {status: 1}; }};
        }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
        const control = brightnessSliderFixture();
        fixture.browser.document.body = {};
        fixture.browser.document.activeElement = fixture.numberInput;
        let focusCalls = 0;
        fixture.numberInput.focus = function () {
            focusCalls++;
            fixture.browser.document.activeElement = fixture.numberInput;
        };
        lighting.bindBrightnessSlider(fixture.browser, control.slider);

        fixture.numberHandlers.keydown({key: "ArrowUp"});
        fixture.numberInput.value = "41";
        fixture.numberHandlers.input();
        fixture.numberHandlers.change();
        let prevented = false;
        await fixture.numberHandlers.keydown({
            key: "Enter",
            preventDefault: function () { prevented = true; }
        });
        await fixture.numberHandlers.change();
        await fixture.numberHandlers.blur();
        assert.equal(prevented, true);
        assert.equal(requests, 1);
        assert.equal(timers.pending(400), 0);
        assert.equal(focusCalls, 1);
    });

    await t.test("blur commits once without stealing focus", async function () {
        const timers = timerFixture();
        let requests = 0;
        const fixture = brightnessBrowserFixture(async function () {
            requests++;
            return {ok: true, json: async function () { return {status: 1}; }};
        }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
        const control = brightnessSliderFixture();
        const otherElement = {};
        fixture.browser.document.body = {};
        fixture.browser.document.activeElement = fixture.numberInput;
        let focusCalls = 0;
        fixture.numberInput.focus = function () { focusCalls++; };
        lighting.bindBrightnessSlider(fixture.browser, control.slider);

        fixture.numberHandlers.keydown({key: "ArrowDown"});
        fixture.numberInput.value = "39";
        fixture.numberHandlers.input();
        fixture.numberHandlers.change();
        fixture.browser.document.activeElement = otherElement;
        await fixture.numberHandlers.blur();
        await fixture.numberHandlers.change();
        assert.equal(requests, 1);
        assert.equal(timers.pending(400), 0);
        assert.equal(focusCalls, 0);
        assert.equal(fixture.browser.document.activeElement, otherElement);
    });
});

test("Brightness keyboard editing coalesces range keys and preserves pointer commits", async function () {
    const timers = timerFixture();
    const submitted = [];
    const fixture = brightnessBrowserFixture(async function (_, options) {
        submitted.push(JSON.parse(options.body).brightness);
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = brightnessSliderFixture();
    fixture.browser.document.body = {};
    fixture.browser.document.activeElement = control.slider;
    let focusCalls = 0;
    control.slider.focus = function () {
        focusCalls++;
        fixture.browser.document.activeElement = control.slider;
    };
    lighting.bindBrightnessSlider(fixture.browser, control.slider);

    for (const key of ["PageUp", "PageDown", "Home", "End"]) {
        control.handlers.keydown({key: key});
        assert.equal(timers.pending(400), 1, key + " was not recognized as a range adjustment");
        control.handlers.pointerdown();
        assert.equal(timers.pending(400), 0, key + " timer was not cancelled");
    }

    control.handlers.keydown({key: "ArrowRight"});
    control.slider.value = "41";
    control.handlers.input();
    control.handlers.change();
    control.handlers.keydown({key: "ArrowUp"});
    control.slider.value = "42";
    control.handlers.input();
    control.handlers.change();
    assert.deepEqual(submitted, [], "keyboard-generated range change committed immediately");
    assert.equal(timers.pending(400), 1);
    await timers.fireNext(400);
    assert.deepEqual(submitted, [42]);
    assert.equal(focusCalls, 1);

    control.handlers.keydown({key: "ArrowLeft"});
    control.slider.value = "41";
    control.handlers.input();
    control.handlers.change();
    control.handlers.keydown({key: "ArrowLeft"});
    control.slider.value = "40";
    control.handlers.input();
    control.handlers.change();
    control.handlers.keydown({key: "ArrowDown"});
    control.slider.value = "39";
    control.handlers.input();
    control.handlers.change();
    control.handlers.keydown({key: "ArrowDown"});
    control.slider.value = "38";
    control.handlers.input();
    control.handlers.change();
    await control.handlers.keydown({key: "Enter", preventDefault: function () {}});
    await control.handlers.change();
    await control.handlers.blur();
    assert.deepEqual(submitted, [42, 38]);
    assert.equal(timers.pending(400), 0);

    control.handlers.keydown({key: "ArrowRight"});
    control.slider.value = "41";
    control.handlers.input();
    control.handlers.change();
    fixture.browser.document.activeElement = fixture.browser.document.body;
    await control.handlers.blur();
    assert.deepEqual(submitted, [42, 38, 41]);
    assert.equal(focusCalls, 2, "blur-origin range commit restored focus");
    assert.equal(timers.pending(400), 0);

    const staleKeyboardCommit = timers.callbackForDelay(400);
    control.handlers.pointerdown();
    control.slider.value = "50";
    control.handlers.input();
    await control.handlers.change();
    await staleKeyboardCommit();
    assert.deepEqual(submitted, [42, 38, 41, 50]);
    assert.equal(timers.pending(400), 0);
    assert.equal(focusCalls, 2, "pointer-origin range commit restored focus");
});

test("Brightness keyboard editing restores failure state and respects deliberate focus movement", async function (t) {
    await t.test("failure restores confirmed state and keyboard focus", async function () {
        const timers = timerFixture();
        const fixture = brightnessBrowserFixture(async function () {
            fixture.browser.document.activeElement = fixture.browser.document.body;
            throw new Error("private failure");
        }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
        const control = brightnessSliderFixture();
        fixture.browser.document.body = {};
        fixture.browser.document.activeElement = fixture.numberInput;
        let focusCalls = 0;
        fixture.numberInput.focus = function () {
            focusCalls++;
            fixture.browser.document.activeElement = fixture.numberInput;
        };
        lighting.bindBrightnessSlider(fixture.browser, control.slider);

        fixture.numberHandlers.keydown({key: "ArrowUp"});
        fixture.numberInput.value = "41";
        fixture.numberHandlers.input();
        await timers.fireNext(400);
        assert.equal(fixture.numberInput.value, "40");
        assert.equal(control.slider.value, "40");
        assert.equal(fixture.status.textContent, "Unable to change brightness. Try again.");
        assert.equal(focusCalls, 1);
        assert.equal(timers.pending(400), 0);
    });

    await t.test("completed keyboard request does not steal deliberately moved focus", async function () {
        const timers = timerFixture();
        let releaseRequest;
        const pendingRequest = new Promise(function (resolve) { releaseRequest = resolve; });
        const fixture = brightnessBrowserFixture(async function () {
            await pendingRequest;
            return {ok: true, json: async function () { return {status: 1}; }};
        }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
        const control = brightnessSliderFixture();
        const otherElement = {};
        fixture.browser.document.body = {};
        fixture.browser.document.activeElement = fixture.numberInput;
        let focusCalls = 0;
        fixture.numberInput.focus = function () { focusCalls++; };
        lighting.bindBrightnessSlider(fixture.browser, control.slider);

        fixture.numberHandlers.keydown({key: "ArrowUp"});
        fixture.numberInput.value = "41";
        fixture.numberHandlers.input();
        const mutation = timers.fireNext(400);
        fixture.browser.document.activeElement = otherElement;
        releaseRequest();
        await mutation;
        assert.equal(focusCalls, 0);
        assert.equal(fixture.browser.document.activeElement, otherElement);
    });
});

test("numeric brightness accepts exact zero and one hundred boundaries", async function () {
    const requests = [];
    const timers = timerFixture();
    const fixture = brightnessBrowserFixture(async function (_, options) {
        requests.push(JSON.parse(options.body).brightness);
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = brightnessSliderFixture();
    lighting.bindBrightnessSlider(fixture.browser, control.slider);

    for (const value of ["0", "100"]) {
        fixture.numberInput.value = value;
        fixture.numberHandlers.input();
        await fixture.numberHandlers.change();
        assert.equal(control.slider.value, value);
        assert.equal(fixture.numberInput.value, value);
    }
    assert.deepEqual(requests, [0, 100]);
});

test("brightness mutation shares range and number state and clears success feedback", async function () {
    let request;
    let releaseRequest;
    const responsePending = new Promise(function (resolve) { releaseRequest = resolve; });
    const timers = timerFixture();
    const fixture = brightnessBrowserFixture(async function (url, options) {
        request = {url, options};
        await responsePending;
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = brightnessSliderFixture();
    lighting.bindBrightnessSlider(fixture.browser, control.slider);

    control.slider.value = "80";
    const mutation = control.handlers.change();

    assert.equal(request.url, "/api/openrgbimport/brightness");
    assert.equal(request.options.method, "POST");
    assert.deepEqual(JSON.parse(request.options.body), {serial: "openrgb-test-device", brightness: 80});
    assert.equal(typeof JSON.parse(request.options.body).brightness, "number");
    assert.ok(request.options.signal instanceof AbortSignal);
    assert.deepEqual(timers.delays, [10000]);
    assert.equal(control.slider.disabled, true);
    assert.equal(fixture.numberInput.disabled, true);
    assert.equal(fixture.status.textContent, "Saving brightness…");

    releaseRequest();
    await mutation;

    assert.equal(control.slider.disabled, false);
    assert.equal(fixture.numberInput.disabled, false);
    assert.equal(control.slider.value, "80");
    assert.equal(fixture.numberInput.value, "80");
    assert.equal(control.slider.dataset.lfCurrentBrightness, "80");
    assert.equal(fixture.hero.textContent, "80%");
    assert.equal(fixture.secondReadout.textContent, "80%");
    assert.equal(fixture.unrelatedReadout.textContent, "75%");
    assert.equal(fixture.status.textContent, "Brightness saved.");
    assert.equal(fixture.resetControl.hidden, true, "Brightness exposed effect Reset");
    assert.deepEqual(timers.delays, [10000, 1800]);
    assert.equal(timers.pending(1800), 1);
    timers.fireNext(1800);
    assert.equal(fixture.status.textContent, "");
    assert.equal(timers.pending(), 0);
    assert.equal(fixture.reloads(), 0);
});

test("numeric Enter, change, and blur commit once and use the shared baseline", async function () {
    let requests = 0;
    let releaseRequest;
    const pending = new Promise(function (resolve) { releaseRequest = resolve; });
    const timers = timerFixture();
    const fixture = brightnessBrowserFixture(async function () {
        requests++;
        await pending;
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = brightnessSliderFixture();
    lighting.bindBrightnessSlider(fixture.browser, control.slider);

    fixture.numberInput.value = "55";
    fixture.numberHandlers.input();
    let prevented = false;
    const mutation = fixture.numberHandlers.keydown({
        key: "Enter",
        preventDefault: function () { prevented = true; }
    });
    await fixture.numberHandlers.change();
    await fixture.numberHandlers.blur();
    assert.equal(prevented, true);
    assert.equal(requests, 1);

    releaseRequest();
    await mutation;
    await fixture.numberHandlers.change();
    await fixture.numberHandlers.blur();
    assert.equal(requests, 1, "Enter followed by change and blur submitted duplicate mutations");
    assert.equal(control.slider.dataset.lfCurrentBrightness, "55");
    assert.equal(control.slider.value, "55");
    assert.equal(fixture.numberInput.value, "55");
});

test("numeric native change and blur each commit one changed value", async function () {
    for (const event of ["change", "blur"]) {
        let requests = 0;
        const timers = timerFixture();
        const fixture = brightnessBrowserFixture(async function () {
            requests++;
            return {ok: true, json: async function () { return {status: 1}; }};
        }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
        const control = brightnessSliderFixture();
        lighting.bindBrightnessSlider(fixture.browser, control.slider);
        fixture.numberInput.value = "65";
        await fixture.numberHandlers[event]();
        assert.equal(requests, 1, event + " did not submit exactly one mutation");
        assert.equal(control.slider.dataset.lfCurrentBrightness, "65");
    }
});

test("invalid and abandoned numeric values never mutate and restore on commit or blur", async function () {
    let requests = 0;
    const fixture = brightnessBrowserFixture(async function () {
        requests++;
        return {ok: true, json: async function () { return {status: 1}; }};
    });

    for (const value of ["", "invalid", "-1", "101", "1.5"]) {
        const control = brightnessSliderFixture();
        lighting.bindBrightnessSlider(fixture.browser, control.slider);
        fixture.numberInput.value = value;
        fixture.numberHandlers.input();
        assert.equal(fixture.numberInput.value, value);
        await fixture.numberHandlers.blur();
        assert.equal(fixture.numberInput.value, "40");
        assert.equal(control.slider.value, "40");
        assert.equal(requests, 0, "submitted rejected brightness " + JSON.stringify(value));
    }

    for (const value of ["40", "", "invalid", "-1", "101", "1.5"]) {
        const control = brightnessSliderFixture();
        lighting.bindBrightnessSlider(fixture.browser, control.slider);
        control.slider.value = value;
        await control.handlers.change();
        assert.equal(requests, 0, "submitted manipulated range brightness " + JSON.stringify(value));
    }
});

test("brightness failures restore both controls and remain until valid interaction", async function (t) {
    const failures = [
        {name: "HTTP", fetch: async function () { return {ok: false, json: async function () { return {status: 1}; }}; }},
        {name: "application", fetch: async function () { return {ok: true, json: async function () { return {status: 0, message: "private persistence detail"}; }}; }},
        {name: "missing response", fetch: async function () { return {ok: true, json: async function () { return null; }}; }},
        {name: "invalid JSON", fetch: async function () { return {ok: true, json: async function () { throw new Error("private parse detail"); }}; }},
        {name: "network", fetch: async function () { throw new Error("private transport detail"); }}
    ];
    for (const failure of failures) {
        await t.test(failure.name, async function () {
            const timers = timerFixture();
            const fixture = brightnessBrowserFixture(failure.fetch, {
                clearTimeout: timers.clearTimeout,
                setTimeout: timers.setTimeout
            });
            const control = brightnessSliderFixture();
            lighting.bindBrightnessSlider(fixture.browser, control.slider);
            fixture.numberInput.value = "81";
            fixture.numberHandlers.input();

            await fixture.numberHandlers.change();

            assert.equal(control.slider.value, "40");
            assert.equal(fixture.numberInput.value, "40");
            assert.equal(control.slider.dataset.lfCurrentBrightness, "40");
            assert.equal(control.slider.disabled, false);
            assert.equal(fixture.numberInput.disabled, false);
            assert.equal(fixture.hero.textContent, "40%");
            assert.equal(fixture.status.textContent, "Unable to change brightness. Try again.");
            assert.doesNotMatch(fixture.status.textContent, /private|persistence|parse|transport/i);
            assert.equal(timers.pending(), 0, "failure scheduled transient-status cleanup");

            fixture.numberInput.value = "50";
            fixture.numberHandlers.input();
            assert.equal(fixture.status.textContent, "", "valid interaction did not clear persistent failure");
        });
    }
});

test("brightness timeout aborts once, restores both controls, and does not retry", async function () {
    const timers = timerFixture();
    let requests = 0;
    let stalledSignal;
    const fixture = brightnessBrowserFixture(function (_, options) {
        requests++;
        stalledSignal = options.signal;
        return new Promise(function (_, reject) {
            options.signal.addEventListener("abort", function () {
                const error = new Error("private abort detail");
                error.name = "AbortError";
                reject(error);
            }, {once: true});
        });
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = brightnessSliderFixture();
    lighting.bindBrightnessSlider(fixture.browser, control.slider);
    fixture.numberInput.value = "62";

    const mutation = fixture.numberHandlers.change();
    assert.equal(requests, 1);
    timers.fireNext(10000);
    await mutation;

    assert.equal(stalledSignal.aborted, true);
    assert.equal(requests, 1);
    assert.equal(control.slider.value, "40");
    assert.equal(fixture.numberInput.value, "40");
    assert.equal(control.slider.disabled, false);
    assert.equal(fixture.numberInput.disabled, false);
    assert.equal(fixture.status.textContent, "Unable to change brightness. Try again.");
    assert.doesNotMatch(fixture.status.textContent, /abort|private/i);
    assert.equal(timers.pending(), 0);
    await Promise.resolve();
    assert.equal(requests, 1, "timeout triggered an automatic retry");
});

test("brightness controls prevent concurrent requests and retain the newest confirmed baseline", async function () {
    let requests = 0;
    let releaseFirst;
    const firstPending = new Promise(function (resolve) { releaseFirst = resolve; });
    const timers = timerFixture();
    const fixture = brightnessBrowserFixture(async function () {
        requests++;
        if (requests === 1) {
            await firstPending;
            return {ok: true, json: async function () { return {status: 1}; }};
        }
        return {ok: true, json: async function () { return {status: requests === 3 ? 1 : 0}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = brightnessSliderFixture();
    lighting.bindBrightnessSlider(fixture.browser, control.slider);

    control.slider.value = "60";
    const first = control.handlers.change();
    await control.handlers.change();
    fixture.numberInput.value = "70";
    await fixture.numberHandlers.change();
    assert.equal(requests, 1, "range or cross-control input bypassed the shared in-flight lock");

    releaseFirst();
    await first;
    assert.equal(control.slider.dataset.lfCurrentBrightness, "60");

    fixture.numberInput.value = "70";
    await fixture.numberHandlers.change();
    assert.equal(requests, 2);
    assert.equal(control.slider.value, "60");
    assert.equal(fixture.numberInput.value, "60");
    assert.equal(fixture.hero.textContent, "60%");

    fixture.numberInput.value = "70";
    await fixture.numberHandlers.change();
    assert.equal(requests, 3);
    assert.equal(control.slider.dataset.lfCurrentBrightness, "70");
    assert.equal(fixture.hero.textContent, "70%");
});

test("stale success cleanup cannot erase newer pending or failure feedback", async function () {
    let requests = 0;
    let rejectSecond;
    const timers = timerFixture();
    const fixture = brightnessBrowserFixture(async function () {
        requests++;
        if (requests === 1) {
            return {ok: true, json: async function () { return {status: 1}; }};
        }
        return new Promise(function (_, reject) { rejectSecond = reject; });
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = brightnessSliderFixture();
    lighting.bindBrightnessSlider(fixture.browser, control.slider);

    control.slider.value = "50";
    await control.handlers.change();
    assert.equal(fixture.status.textContent, "Brightness saved.");
    const staleCleanup = timers.callbackForDelay(1800);

    fixture.numberInput.value = "60";
    const second = fixture.numberHandlers.change();
    assert.equal(fixture.status.textContent, "Saving brightness…");
    staleCleanup();
    assert.equal(fixture.status.textContent, "Saving brightness…");

    rejectSecond(new Error("private failure"));
    await second;
    assert.equal(fixture.status.textContent, "Unable to change brightness. Try again.");
    staleCleanup();
    assert.equal(fixture.status.textContent, "Unable to change brightness. Try again.");
});

test("cluster-owned brightness controls do not bind local interactions", function () {
    let requests = 0;
    const fixture = brightnessBrowserFixture(async function () { requests++; });
    fixture.numberInput.disabled = true;
    const control = brightnessSliderFixture({
        dataset: Object.assign({}, brightnessSliderFixture().slider.dataset, {lfClusterControlled: "true"}),
        disabled: true
    });

    assert.equal(lighting.bindBrightnessSlider(fixture.browser, control.slider), null);
    assert.deepEqual(control.handlers, {});
    assert.deepEqual(fixture.numberHandlers, {});
    assert.equal(control.slider.disabled, true);
    assert.equal(fixture.numberInput.disabled, true);
    assert.equal(requests, 0);
});

test("multiple brightness controls keep transient success timers isolated", async function () {
    const timers = timerFixture();
    const controls = [
        brightnessSliderFixture({dataset: {
            lfClusterControlled: "false", lfCurrentBrightness: "40", lfDeviceSerial: "device-a",
            lfNumberId: "number-a", lfStatusId: "status-a"
        }}),
        brightnessSliderFixture({dataset: {
            lfClusterControlled: "false", lfCurrentBrightness: "60", lfDeviceSerial: "device-b",
            lfNumberId: "number-b", lfStatusId: "status-b"
        }, value: "60"})
    ];
    const numbers = {};
    const statuses = {"status-a": {textContent: ""}, "status-b": {textContent: ""}};
    for (const entry of [["number-a", "40"], ["number-b", "60"]]) {
        const handlers = {};
        numbers[entry[0]] = {
            disabled: false,
            handlers,
            value: entry[1],
            addEventListener: function (event, handler) { handlers[event] = handler; }
        };
    }
    const browser = {
        AbortController,
        clearTimeout: timers.clearTimeout,
        document: {
            getElementById: function (id) { return numbers[id] || statuses[id]; },
            querySelectorAll: function () { return []; }
        },
        fetch: async function () { return {ok: true, json: async function () { return {status: 1}; }}; },
        setTimeout: timers.setTimeout
    };
    browser.document.querySelectorAll = function (selector) {
        if (selector === "[data-lf-effect-selector]") return [];
        if (selector === "[data-lf-brightness-slider]") {
            return controls.map(function (control) { return control.slider; });
        }
        if (selector === "[data-lf-speed-slider]") return [];
        if (selector === "[data-lf-color-input]") return [];
        if (selector === "[data-lf-two-color-control]") return [];
        if (selector === "[data-lf-temperature-control]") return [];
        if (selector === "[data-lf-gradient-control]") return [];
        if (selector === "[data-lf-reset-button]") return [];
        assert.equal(selector, "[data-lf-brightness-readout]");
        return [];
    };
    lighting.init(browser);

    numbers["number-a"].value = "45";
    numbers["number-b"].value = "65";
    await Promise.all([numbers["number-a"].handlers.change(), numbers["number-b"].handlers.change()]);
    assert.equal(statuses["status-a"].textContent, "Brightness saved.");
    assert.equal(statuses["status-b"].textContent, "Brightness saved.");
    assert.equal(timers.pending(1800), 2);

    timers.fireNext(1800);
    assert.equal(statuses["status-a"].textContent, "");
    assert.equal(statuses["status-b"].textContent, "Brightness saved.");
    timers.fireNext(1800);
    assert.equal(statuses["status-b"].textContent, "");
});

test("Speed initialization uses canonical software mappings without a placeholder flash", function () {
    const cases = [
        {effect: "circle", stored: "5.2", displayed: "5.8"},
        {effect: "flame", stored: "0.8", displayed: "5.5"},
        {effect: "cyberpunkglitch", stored: "0.55", displayed: "5.5"},
        {effect: "rain", stored: "2", displayed: "5.5"},
        {effect: "aurora", stored: "4.1", displayed: "4.1"},
        {effect: "gradient", stored: "2", displayed: "9.0"}
    ];
    for (const entry of cases) {
        const fixture = speedBrowserFixture(async function () { assert.fail("initialization submitted a request"); });
        const control = speedSliderFixture({
            dataset: Object.assign({}, speedSliderFixture().slider.dataset, {
                lfCurrentStoredSpeed: entry.stored,
                lfEffect: entry.effect
            })
        });

        lighting.bindSpeedSlider(fixture.browser, control.slider);

        assert.equal(control.slider.value, entry.displayed, entry.effect);
        assert.equal(fixture.numberInput.value, entry.displayed, entry.effect);
        assert.equal(control.slider.dataset.storedSpeed, entry.stored, entry.effect);
        assert.equal(control.slider.dataset.speedEdited, "false", entry.effect);
        assert.equal(control.attributes["aria-valuetext"], entry.displayed + " speed level", entry.effect);
        assert.deepEqual(control.readyClasses, ["lf-range-control-ready"], entry.effect);
    }
});

test("Speed progress is finite and clamped for valid and malformed range metadata", function () {
    const cases = [
        {name: "midpoint", minimum: "1", maximum: "10", formatted: "5.5", expected: "50%"},
        {name: "minimum", minimum: "1", maximum: "10", formatted: "1", expected: "0%"},
        {name: "maximum", minimum: "1", maximum: "10", formatted: "10", expected: "100%"},
        {name: "below range", minimum: "1", maximum: "10", formatted: "0", expected: "0%"},
        {name: "above range", minimum: "1", maximum: "10", formatted: "11", expected: "100%"},
        {name: "missing minimum", maximum: "10", formatted: "5", expected: "0%"},
        {name: "missing maximum", minimum: "1", formatted: "5", expected: "0%"},
        {name: "blank minimum", minimum: "", maximum: "10", formatted: "5", expected: "0%"},
        {name: "blank maximum", minimum: "1", maximum: "", formatted: "5", expected: "0%"},
        {name: "nonnumeric minimum", minimum: "slow", maximum: "10", formatted: "5", expected: "0%"},
        {name: "nonnumeric maximum", minimum: "1", maximum: "fast", formatted: "5", expected: "0%"},
        {name: "equal bounds", minimum: "5", maximum: "5", formatted: "5", expected: "0%"},
        {name: "nonnumeric formatted value", minimum: "1", maximum: "10", formatted: "invalid", expected: "0%"}
    ];

    for (const entry of cases) {
        const fixture = speedBrowserFixture(async function () { assert.fail("progress rendering submitted a request"); });
        const control = speedSliderFixture();
        const helper = {
            SOFTWARE_CONTROL: "software",
            configureSlider: function (slider) {
                if (Object.hasOwn(entry, "minimum")) slider.min = entry.minimum;
                else delete slider.min;
                if (Object.hasOwn(entry, "maximum")) slider.max = entry.maximum;
                else delete slider.max;
            },
            formatForSlider: function () { return entry.formatted; },
            hasSpeedControl: function () { return true; },
            markEdited: function () {},
            storedToUiForSlider: function () { return 5; },
            uiToStoredForSlider: function (_, value) { return value; }
        };
        fixture.browser.LumenForgeRgbSpeed = helper;

        lighting.bindSpeedSlider(fixture.browser, control.slider);

        const progress = control.styleValues["--lf-range-progress"];
        assert.equal(progress, entry.expected, entry.name);
        const numericProgress = Number(progress.slice(0, -1));
        assert.ok(Number.isFinite(numericProgress), entry.name + " produced non-finite progress");
        assert.ok(numericProgress >= 0 && numericProgress <= 100, entry.name + " produced overflowing progress");
    }
});

test("Speed controls with missing or malformed stored values do not bind", function () {
    const cases = [
        {name: "missing"},
        {name: "empty", value: ""},
        {name: "whitespace", value: " \t "},
        {name: "nonnumeric", value: "fast"},
        {name: "NaN", value: "NaN"},
        {name: "positive infinity", value: "Infinity"},
        {name: "negative infinity", value: "-Infinity"}
    ];
    for (const entry of cases) {
        let requests = 0;
        const fixture = speedBrowserFixture(async function () { requests++; });
        const dataset = Object.assign({}, speedSliderFixture().slider.dataset);
        if (Object.hasOwn(entry, "value")) dataset.lfCurrentStoredSpeed = entry.value;
        else delete dataset.lfCurrentStoredSpeed;
        const control = speedSliderFixture({dataset: dataset});

        assert.equal(lighting.bindSpeedSlider(fixture.browser, control.slider), null, entry.name);
        assert.deepEqual(control.handlers, {}, entry.name);
        assert.equal(requests, 0, entry.name);
    }

    const validFixture = speedBrowserFixture(async function () { assert.fail("valid initialization submitted a request"); });
    const validControl = speedSliderFixture({dataset: Object.assign({}, speedSliderFixture().slider.dataset, {
        lfCurrentStoredSpeed: "0"
    })});
    assert.notEqual(lighting.bindSpeedSlider(validFixture.browser, validControl.slider), null);
    assert.ok(validControl.handlers.change);
});

test("Speed range previews locally and commits one mapped numeric mutation", async function () {
    let request;
    const timers = timerFixture();
    const fixture = speedBrowserFixture(async function (url, options) {
        request = {url, options};
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = speedSliderFixture();
    lighting.bindSpeedSlider(fixture.browser, control.slider);
    assert.equal(fixture.resetControl.hidden, true);

    control.slider.value = "5";
    control.handlers.input();
    assert.equal(fixture.numberInput.value, "5.0");
    assert.equal(control.slider.dataset.speedEdited, "true");
    assert.equal(control.styleValues["--lf-range-progress"], "44.44444444444444%");
    assert.equal(request, undefined, "range preview submitted before release");
    assert.equal(fixture.readouts.base.textContent, "2");

    await control.handlers.change();

    assert.equal(request.url, "/api/openrgbimport/speed");
    assert.equal(request.options.method, "POST");
    assert.deepEqual(JSON.parse(request.options.body), {
        serial: "openrgb-test-device",
        effect: "circle",
        speed: 6
    });
    assert.equal(typeof JSON.parse(request.options.body).speed, "number");
    assert.ok(request.options.signal instanceof AbortSignal);
    assert.equal(control.slider.dataset.lfCurrentStoredSpeed, "6");
    assert.equal(control.slider.dataset.storedSpeed, "6");
    assert.equal(control.slider.dataset.speedEdited, "false");
    assert.equal(fixture.readouts.base.textContent, "6");
    assert.equal(fixture.readouts.effective.textContent, "6");
    assert.equal(fixture.readouts.override.textContent, "8");
    assert.equal(fixture.status.textContent, "Speed saved.");
    assert.equal(fixture.resetControl.hidden, false, "successful first Speed customization did not reveal Reset");
    assert.deepEqual(timers.delays, [10000, 1800]);
    timers.fireNext(1800);
    assert.equal(fixture.status.textContent, "");
});

test("successful first Aurora Speed customization reveals effect Reset", async function () {
    let requests = 0;
    const fixture = speedBrowserFixture(async function () {
        requests++;
        return {ok: true, json: async function () { return {status: 1}; }};
    });
    fixture.resetControl.dataset.lfEffect = "aurora";
    const control = speedSliderFixture({dataset: Object.assign({}, speedSliderFixture().slider.dataset, {
        lfCurrentStoredSpeed: "4.1",
        lfEffect: "aurora"
    })});
    lighting.bindSpeedSlider(fixture.browser, control.slider);

    assert.equal(fixture.resetControl.hidden, true);
    control.slider.value = "5";
    control.handlers.input();
    await control.handlers.change();

    assert.equal(requests, 1);
    assert.equal(fixture.resetControl.hidden, false);
});

test("unchanged and invalid Speed interactions do not reveal effect Reset", async function () {
    let requests = 0;
    const fixture = speedBrowserFixture(async function () { requests++; });
    const control = speedSliderFixture();
    lighting.bindSpeedSlider(fixture.browser, control.slider);

    await control.handlers.change();
    fixture.numberInput.value = "invalid";
    fixture.numberHandlers.input();
    await fixture.numberHandlers.blur();

    assert.equal(requests, 0);
    assert.equal(fixture.resetControl.hidden, true);
});

test("Speed override success updates only override and effective stored readouts", async function () {
    const fixture = speedBrowserFixture(async function () {
        return {ok: true, json: async function () { return {status: 1}; }};
    });
    const control = speedSliderFixture({dataset: Object.assign({}, speedSliderFixture().slider.dataset, {
        lfSpeedTarget: "override"
    })});
    lighting.bindSpeedSlider(fixture.browser, control.slider);
    control.slider.value = "5";
    control.handlers.input();
    await control.handlers.change();

    assert.equal(fixture.readouts.base.textContent, "2");
    assert.equal(fixture.readouts.override.textContent, "6");
    assert.equal(fixture.readouts.effective.textContent, "6");
});

test("Speed empty or unrecognized targets update only the effective readout", async function () {
    for (const target of ["", "future"]) {
        const fixture = speedBrowserFixture(async function () {
            return {ok: true, json: async function () { return {status: 1}; }};
        });
        const control = speedSliderFixture({dataset: Object.assign({}, speedSliderFixture().slider.dataset, {
            lfSpeedTarget: target
        })});
        lighting.bindSpeedSlider(fixture.browser, control.slider);
        control.slider.value = "5";
        control.handlers.input();
        await control.handlers.change();

        assert.equal(fixture.readouts.base.textContent, "2", target || "empty target");
        assert.equal(fixture.readouts.override.textContent, "8", target || "empty target");
        assert.equal(fixture.readouts.effective.textContent, "6", target || "empty target");
        assert.equal(fixture.readouts.empty.textContent, "4", target || "empty target");
    }
});

test("Speed preserves an exact legacy stored value until a genuine edit", async function () {
    const requests = [];
    const fixture = speedBrowserFixture(async function (_, options) {
        requests.push(JSON.parse(options.body).speed);
        return {ok: true, json: async function () { return {status: 1}; }};
    });
    const control = speedSliderFixture({dataset: Object.assign({}, speedSliderFixture().slider.dataset, {
        lfCurrentStoredSpeed: "9",
        lfEffect: "flame"
    })});
    lighting.bindSpeedSlider(fixture.browser, control.slider);

    assert.equal(control.slider.value, "10.0");
    assert.equal(control.slider.dataset.storedSpeed, "9");
    await fixture.numberHandlers.blur();
    assert.deepEqual(requests, []);
    assert.equal(control.slider.dataset.storedSpeed, "9");

    fixture.numberInput.value = "1.1";
    fixture.numberHandlers.input();
    await fixture.numberHandlers.change();
    assert.deepEqual(requests, [0.116]);
    assert.equal(control.slider.dataset.storedSpeed, "0.116");
});

test("Speed numeric commits deduplicate Enter, change, and blur", async function () {
    let requests = 0;
    let releaseRequest;
    const pending = new Promise(function (resolve) { releaseRequest = resolve; });
    const fixture = speedBrowserFixture(async function () {
        requests++;
        await pending;
        return {ok: true, json: async function () { return {status: 1}; }};
    });
    const control = speedSliderFixture();
    lighting.bindSpeedSlider(fixture.browser, control.slider);
    fixture.numberInput.value = "6.5";
    fixture.numberHandlers.input();
    let prevented = false;
    const mutation = fixture.numberHandlers.keydown({key: "Enter", preventDefault: function () { prevented = true; }});
    await fixture.numberHandlers.change();
    await fixture.numberHandlers.blur();
    assert.equal(prevented, true);
    assert.equal(requests, 1);
    assert.equal(fixture.resetControl.hidden, true, "in-flight Speed mutation revealed Reset");
    releaseRequest();
    await mutation;
    await fixture.numberHandlers.change();
    await fixture.numberHandlers.blur();
    assert.equal(requests, 1);
    assert.equal(fixture.resetControl.hidden, false);
});

test("Speed keyboard editing preserves raw numeric text until commit or restoration", async function () {
    const timers = timerFixture();
    let requests = 0;
    const fixture = speedBrowserFixture(async function () {
        requests++;
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = speedSliderFixture();
    lighting.bindSpeedSlider(fixture.browser, control.slider);

    let rawValue = fixture.numberInput.value;
    let valueAssignments = 0;
    Object.defineProperty(fixture.numberInput, "value", {
        configurable: true,
        get: function () { return rawValue; },
        set: function (value) {
            valueAssignments++;
            rawValue = String(value);
        }
    });
    fixture.numberInput.selectionStart = 0;
    fixture.numberInput.selectionEnd = 3;

    rawValue = "5";
    fixture.numberHandlers.input();
    assert.equal(rawValue, "5");
    assert.equal(valueAssignments, 0, "valid input preview rewrote the numeric field");
    assert.equal(fixture.numberInput.selectionStart, 0);
    assert.equal(fixture.numberInput.selectionEnd, 3);
    assert.equal(control.slider.value, "5.0");
    assert.equal(control.attributes["aria-valuetext"], "5.0 speed level");
    assert.equal(requests, 0);

    rawValue = "5.5";
    fixture.numberInput.selectionStart = 3;
    fixture.numberInput.selectionEnd = 3;
    fixture.numberHandlers.input();
    assert.equal(rawValue, "5.5");
    assert.equal(valueAssignments, 0, "continued decimal input rewrote the numeric field");
    assert.equal(fixture.numberInput.selectionStart, 3);
    assert.equal(fixture.numberInput.selectionEnd, 3);
    assert.equal(control.slider.value, "5.5");

    rawValue = "5";
    fixture.numberHandlers.input();
    await fixture.numberHandlers.change();
    assert.equal(requests, 1);
    assert.equal(rawValue, "5.0", "explicit commit did not normalize the numeric field");
    assert.ok(valueAssignments > 0, "explicit commit did not render confirmed state");

    valueAssignments = 0;
    rawValue = "";
    fixture.numberHandlers.input();
    assert.equal(rawValue, "");
    assert.equal(valueAssignments, 0, "temporary empty input was rewritten");
    await fixture.numberHandlers.blur();
    assert.equal(rawValue, "5.0");
    assert.equal(requests, 1, "invalid blur submitted a mutation");
});

test("Speed keyboard editing coalesces numeric arrows and keeps timers independent", async function () {
    const timers = timerFixture();
    const submitted = [];
    const fixture = speedBrowserFixture(async function (_, options) {
        submitted.push(JSON.parse(options.body).speed);
        fixture.browser.document.activeElement = fixture.browser.document.body;
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = speedSliderFixture();
    const body = {};
    let focusCalls = 0;
    let restoredSelection;
    fixture.browser.document.body = body;
    fixture.browser.document.activeElement = fixture.numberInput;
    fixture.numberInput.focus = function () {
        focusCalls++;
        fixture.browser.document.activeElement = fixture.numberInput;
    };
    fixture.numberInput.selectionStart = 3;
    fixture.numberInput.selectionEnd = 3;
    fixture.numberInput.setSelectionRange = function (start, end) {
        restoredSelection = [start, end];
    };
    lighting.bindSpeedSlider(fixture.browser, control.slider);

    fixture.numberHandlers.keydown({key: "ArrowUp"});
    fixture.numberInput.value = "9.1";
    fixture.numberHandlers.input();
    fixture.numberHandlers.change();
    assert.equal(timers.pending(400), 1);
    assert.equal(submitted.length, 0);

    const staleCommit = timers.callbackForDelay(400);
    fixture.numberHandlers.keydown({key: "ArrowUp"});
    fixture.numberInput.value = "9.2";
    fixture.numberHandlers.input();
    fixture.numberHandlers.change();
    assert.equal(timers.pending(400), 1, "repeated arrow did not reset the idle timer");
    await staleCommit();
    assert.equal(submitted.length, 0, "stale keyboard timer submitted an older value");

    await timers.fireNext(400);
    assert.deepEqual(submitted, [1.8]);
    assert.equal(focusCalls, 1);
    assert.equal(fixture.browser.document.activeElement, fixture.numberInput);
    assert.deepEqual(restoredSelection, [3, 3]);
    assert.equal(timers.pending(1800), 1);

    const brightnessFixture = brightnessBrowserFixture(async function () {
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const brightness = brightnessSliderFixture();
    lighting.bindBrightnessSlider(brightnessFixture.browser, brightness.slider);
    brightness.slider.value = "45";
    await brightness.handlers.change();
    assert.equal(timers.pending(1800), 2);

    fixture.numberHandlers.keydown({key: "ArrowDown"});
    fixture.numberInput.value = "9.1";
    fixture.numberHandlers.input();
    fixture.numberHandlers.change();
    assert.equal(timers.pending(400), 1);
    assert.equal(timers.pending(1800), 1);
    timers.fireNext(1800);
    assert.equal(brightnessFixture.status.textContent, "");
    assert.deepEqual(submitted, [1.8], "Brightness cleanup triggered a Speed keyboard commit");
    await timers.fireNext(400);
    assert.deepEqual(submitted, [1.8, 1.9]);
});

test("Speed keyboard editing immediate numeric commits cancel idle work and respect focus changes", async function (t) {
    await t.test("Enter commits once and restores keyboard focus", async function () {
        const timers = timerFixture();
        let requests = 0;
        const fixture = speedBrowserFixture(async function () {
            requests++;
            fixture.browser.document.activeElement = fixture.browser.document.body;
            return {ok: true, json: async function () { return {status: 1}; }};
        }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
        const control = speedSliderFixture();
        fixture.browser.document.body = {};
        fixture.browser.document.activeElement = fixture.numberInput;
        let focusCalls = 0;
        fixture.numberInput.focus = function () {
            focusCalls++;
            fixture.browser.document.activeElement = fixture.numberInput;
        };
        lighting.bindSpeedSlider(fixture.browser, control.slider);

        fixture.numberHandlers.keydown({key: "ArrowUp"});
        fixture.numberInput.value = "9.1";
        fixture.numberHandlers.input();
        fixture.numberHandlers.change();
        let prevented = false;
        await fixture.numberHandlers.keydown({
            key: "Enter",
            preventDefault: function () { prevented = true; }
        });
        await fixture.numberHandlers.change();
        await fixture.numberHandlers.blur();
        assert.equal(prevented, true);
        assert.equal(requests, 1);
        assert.equal(timers.pending(400), 0);
        assert.equal(focusCalls, 1);
    });

    await t.test("blur commits once without stealing focus", async function () {
        const timers = timerFixture();
        let requests = 0;
        const fixture = speedBrowserFixture(async function () {
            requests++;
            return {ok: true, json: async function () { return {status: 1}; }};
        }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
        const control = speedSliderFixture();
        const otherElement = {};
        fixture.browser.document.body = {};
        fixture.browser.document.activeElement = fixture.numberInput;
        let focusCalls = 0;
        fixture.numberInput.focus = function () { focusCalls++; };
        lighting.bindSpeedSlider(fixture.browser, control.slider);

        fixture.numberHandlers.keydown({key: "ArrowUp"});
        fixture.numberInput.value = "9.1";
        fixture.numberHandlers.input();
        fixture.numberHandlers.change();
        fixture.browser.document.activeElement = otherElement;
        await fixture.numberHandlers.blur();
        await fixture.numberHandlers.change();
        assert.equal(requests, 1);
        assert.equal(timers.pending(400), 0);
        assert.equal(focusCalls, 0);
        assert.equal(fixture.browser.document.activeElement, otherElement);
    });
});

test("Speed keyboard editing coalesces range keys without changing pointer commits", async function () {
    const timers = timerFixture();
    const submitted = [];
    const fixture = speedBrowserFixture(async function (_, options) {
        submitted.push(JSON.parse(options.body).speed);
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = speedSliderFixture();
    fixture.browser.document.body = {};
    fixture.browser.document.activeElement = control.slider;
    let focusCalls = 0;
    control.slider.focus = function () {
        focusCalls++;
        fixture.browser.document.activeElement = control.slider;
    };
    lighting.bindSpeedSlider(fixture.browser, control.slider);

    control.handlers.keydown({key: "ArrowRight"});
    control.slider.value = "9.1";
    control.handlers.input();
    control.handlers.change();
    control.handlers.keydown({key: "ArrowRight"});
    control.slider.value = "9.2";
    control.handlers.input();
    control.handlers.change();
    assert.equal(submitted.length, 0, "keyboard-generated range change committed immediately");
    assert.equal(timers.pending(400), 1);
    await timers.fireNext(400);
    assert.deepEqual(submitted, [1.8]);
    assert.equal(focusCalls, 1);

    control.handlers.keydown({key: "ArrowUp"});
    control.slider.value = "9.3";
    control.handlers.input();
    control.handlers.change();
    await control.handlers.keydown({key: "Enter", preventDefault: function () {}});
    await control.handlers.change();
    await control.handlers.blur();
    assert.deepEqual(submitted, [1.8, 1.7]);
    assert.equal(timers.pending(400), 0);

    control.handlers.keydown({key: "ArrowUp"});
    control.slider.value = "9.4";
    control.handlers.input();
    control.handlers.change();
    fixture.browser.document.activeElement = fixture.browser.document.body;
    await control.handlers.blur();
    assert.deepEqual(submitted, [1.8, 1.7, 1.6]);
    assert.equal(focusCalls, 2, "blur-origin range commit restored focus");
    assert.equal(timers.pending(400), 0);

    const staleKeyboardCommit = timers.callbackForDelay(400);
    control.handlers.pointerdown();
    control.slider.value = "9.5";
    control.handlers.input();
    await control.handlers.change();
    await staleKeyboardCommit();
    assert.deepEqual(submitted, [1.8, 1.7, 1.6, 1.5]);
    assert.equal(timers.pending(400), 0);
});

test("Speed keyboard editing restores failure state and does not steal deliberately moved focus", async function (t) {
    await t.test("failure restores confirmed state and keyboard focus", async function () {
        const timers = timerFixture();
        const fixture = speedBrowserFixture(async function () {
            fixture.browser.document.activeElement = fixture.browser.document.body;
            throw new Error("private failure");
        }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
        const control = speedSliderFixture();
        fixture.browser.document.body = {};
        fixture.browser.document.activeElement = fixture.numberInput;
        let focusCalls = 0;
        fixture.numberInput.focus = function () {
            focusCalls++;
            fixture.browser.document.activeElement = fixture.numberInput;
        };
        lighting.bindSpeedSlider(fixture.browser, control.slider);

        fixture.numberHandlers.keydown({key: "ArrowUp"});
        fixture.numberInput.value = "9.1";
        fixture.numberHandlers.input();
        await timers.fireNext(400);
        assert.equal(fixture.numberInput.value, "9.0");
        assert.equal(control.slider.value, "9.0");
        assert.equal(fixture.status.textContent, "Unable to change speed. Try again.");
        assert.equal(focusCalls, 1);
        assert.equal(timers.pending(400), 0);
    });

    await t.test("completed keyboard request respects later focus movement", async function () {
        const timers = timerFixture();
        let releaseRequest;
        const pendingRequest = new Promise(function (resolve) { releaseRequest = resolve; });
        const fixture = speedBrowserFixture(async function () {
            await pendingRequest;
            return {ok: true, json: async function () { return {status: 1}; }};
        }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
        const control = speedSliderFixture();
        const otherElement = {};
        fixture.browser.document.body = {};
        fixture.browser.document.activeElement = fixture.numberInput;
        let focusCalls = 0;
        fixture.numberInput.focus = function () { focusCalls++; };
        lighting.bindSpeedSlider(fixture.browser, control.slider);

        fixture.numberHandlers.keydown({key: "ArrowUp"});
        fixture.numberInput.value = "9.1";
        fixture.numberHandlers.input();
        const mutation = timers.fireNext(400);
        fixture.browser.document.activeElement = otherElement;
        releaseRequest();
        await mutation;
        assert.equal(focusCalls, 0);
        assert.equal(fixture.browser.document.activeElement, otherElement);
    });
});

test("invalid Speed edits never mutate and abandoned values restore", async function () {
    let requests = 0;
    const fixture = speedBrowserFixture(async function () { requests++; });
    for (const value of ["", "invalid", "0.9", "10.1", "5.55", "-1"]) {
        const control = speedSliderFixture();
        lighting.bindSpeedSlider(fixture.browser, control.slider);
        fixture.numberInput.value = value;
        fixture.numberHandlers.input();
        assert.equal(fixture.numberInput.value, value);
        await fixture.numberHandlers.blur();
        assert.equal(fixture.numberInput.value, "9.0");
        assert.equal(requests, 0, "submitted invalid Speed " + JSON.stringify(value));
    }
});

test("Speed failure and timeout restore exact confirmed state without retry", async function (t) {
    const failures = [
        {name: "application", fetch: async function () { return {ok: true, json: async function () { return {status: 0, message: "private"}; }}; }},
        {name: "HTTP", fetch: async function () { return {ok: false, json: async function () { return {status: 1}; }}; }},
        {name: "invalid JSON", fetch: async function () { return {ok: true, json: async function () { throw new Error("private parse detail"); }}; }},
        {name: "network", fetch: async function () { throw new Error("private network detail"); }}
    ];
    for (const failure of failures) {
        await t.test(failure.name, async function () {
            const timers = timerFixture();
            const fixture = speedBrowserFixture(failure.fetch, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
            const control = speedSliderFixture();
            lighting.bindSpeedSlider(fixture.browser, control.slider);
            control.slider.value = "5";
            control.handlers.input();
            await control.handlers.change();
            assert.equal(control.slider.value, "9.0");
            assert.equal(fixture.numberInput.value, "9.0");
            assert.equal(control.slider.dataset.storedSpeed, "2");
            assert.equal(control.slider.disabled, false);
            assert.equal(fixture.numberInput.disabled, false);
            assert.equal(fixture.status.textContent, "Unable to change speed. Try again.");
            assert.doesNotMatch(fixture.status.textContent, /private|network/i);
            assert.equal(fixture.readouts.base.textContent, "2");
            assert.equal(fixture.resetControl.hidden, true, "failed Speed mutation revealed Reset");
            assert.equal(timers.pending(), 0);
        });
    }

    const timers = timerFixture();
    let requests = 0;
    let signal;
    const timeoutFixture = speedBrowserFixture(function (_, options) {
        requests++;
        signal = options.signal;
        return new Promise(function (_, reject) {
            signal.addEventListener("abort", function () { reject(new Error("private abort")); }, {once: true});
        });
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const timeoutControl = speedSliderFixture();
    lighting.bindSpeedSlider(timeoutFixture.browser, timeoutControl.slider);
    timeoutControl.slider.value = "5";
    timeoutControl.handlers.input();
    const mutation = timeoutControl.handlers.change();
    timers.fireNext(10000);
    await mutation;
    assert.equal(signal.aborted, true);
    assert.equal(requests, 1);
    assert.equal(timeoutFixture.status.textContent, "Unable to change speed. Try again.");
    assert.equal(timeoutControl.slider.disabled, false);
    assert.equal(timeoutFixture.numberInput.disabled, false);
    assert.equal(timeoutFixture.resetControl.hidden, true, "timed-out Speed mutation revealed Reset");
    await Promise.resolve();
    assert.equal(requests, 1, "Speed timeout retried automatically");
});

test("a later valid Speed interaction clears failure and can succeed", async function () {
    let requests = 0;
    const fixture = speedBrowserFixture(async function () {
        requests++;
        if (requests === 1) throw new Error("private first failure");
        return {ok: true, json: async function () { return {status: 1}; }};
    });
    const control = speedSliderFixture();
    lighting.bindSpeedSlider(fixture.browser, control.slider);

    control.slider.value = "5";
    control.handlers.input();
    await control.handlers.change();
    assert.equal(fixture.status.textContent, "Unable to change speed. Try again.");
    assert.equal(control.slider.value, "9.0");

    control.slider.value = "6";
    control.handlers.input();
    assert.equal(fixture.status.textContent, "");
    await control.handlers.change();
    assert.equal(requests, 2);
    assert.equal(fixture.status.textContent, "Speed saved.");
    assert.equal(control.slider.disabled, false);
    assert.equal(fixture.numberInput.disabled, false);
});

test("Speed cluster ownership and missing helper never bind mutations", function () {
    let requests = 0;
    const clusterFixture = speedBrowserFixture(async function () { requests++; });
    clusterFixture.numberInput.disabled = true;
    const cluster = speedSliderFixture({
        dataset: Object.assign({}, speedSliderFixture().slider.dataset, {lfClusterControlled: "true"}),
        disabled: true
    });
    assert.equal(lighting.bindSpeedSlider(clusterFixture.browser, cluster.slider), null);
    assert.deepEqual(cluster.handlers, {});
    assert.equal(cluster.slider.value, "9.0", "cluster Speed did not initialize through the mapping helper");
    assert.deepEqual(cluster.readyClasses, ["lf-range-control-ready"]);
    assert.equal(clusterFixture.resetControl.hidden, true, "cluster-owned Speed control revealed Reset");

    const missingFixture = speedBrowserFixture(async function () { requests++; });
    delete missingFixture.browser.LumenForgeRgbSpeed;
    const missing = speedSliderFixture();
    assert.equal(lighting.bindSpeedSlider(missingFixture.browser, missing.slider), null);
    assert.deepEqual(missing.handlers, {});
    assert.deepEqual(missing.readyClasses, []);
    assert.equal(requests, 0);
});

test("Speed stale success cleanup cannot erase newer feedback or brightness feedback", async function () {
    let requests = 0;
    let rejectSecond;
    const timers = timerFixture();
    const fixture = speedBrowserFixture(async function () {
        requests++;
        if (requests === 1) return {ok: true, json: async function () { return {status: 1}; }};
        return new Promise(function (_, reject) { rejectSecond = reject; });
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = speedSliderFixture();
    lighting.bindSpeedSlider(fixture.browser, control.slider);
    control.slider.value = "5";
    control.handlers.input();
    await control.handlers.change();
    const staleCleanup = timers.callbackForDelay(1800);

    control.slider.value = "6";
    control.handlers.input();
    const second = control.handlers.change();
    assert.equal(fixture.status.textContent, "Saving speed…");
    staleCleanup();
    assert.equal(fixture.status.textContent, "Saving speed…");
    rejectSecond(new Error("private"));
    await second;
    staleCleanup();
    assert.equal(fixture.status.textContent, "Unable to change speed. Try again.");
});

test("Brightness and Speed success timers remain independent", async function () {
    const timers = timerFixture();
    const response = async function () { return {ok: true, json: async function () { return {status: 1}; }}; };
    const brightnessFixture = brightnessBrowserFixture(response, {
        clearTimeout: timers.clearTimeout,
        setTimeout: timers.setTimeout
    });
    const brightness = brightnessSliderFixture();
    lighting.bindBrightnessSlider(brightnessFixture.browser, brightness.slider);
    brightness.slider.value = "45";
    await brightness.handlers.change();

    const speedFixture = speedBrowserFixture(response, {
        clearTimeout: timers.clearTimeout,
        setTimeout: timers.setTimeout
    });
    const speed = speedSliderFixture();
    lighting.bindSpeedSlider(speedFixture.browser, speed.slider);
    speed.slider.value = "5";
    speed.handlers.input();
    await speed.handlers.change();

    assert.equal(brightnessFixture.status.textContent, "Brightness saved.");
    assert.equal(speedFixture.status.textContent, "Speed saved.");
    assert.equal(timers.pending(1800), 2);
    timers.fireNext(1800);
    assert.equal(brightnessFixture.status.textContent, "");
    assert.equal(speedFixture.status.textContent, "Speed saved.");
    timers.fireNext(1800);
    assert.equal(speedFixture.status.textContent, "");
});

test("Lighting initialization tolerates pages without interactive controls", function () {
    const selectors = [];
    const browser = {
        document: {
            querySelectorAll: function (selector) {
                selectors.push(selector);
                return [];
            }
        }
    };

    lighting.init(browser);

    assert.deepEqual(selectors, ["[data-lf-effect-selector]", "[data-lf-brightness-slider]", "[data-lf-speed-slider]", "[data-lf-color-input]", "[data-lf-two-color-control]", "[data-lf-temperature-control]", "[data-lf-gradient-control]", "[data-lf-reset-button]"]);
});

test("Lighting initialization supports isolated and combined interactive controls", function () {
    const effectControl = selectorFixture();
    const effectFixture = browserFixture(async function () {
        return {ok: true, json: async function () { return {status: 1}; }};
    });
    effectFixture.browser.document.querySelectorAll = function (selector) {
        return selector === "[data-lf-effect-selector]" ? [effectControl.selector] : [];
    };
    lighting.init(effectFixture.browser);
    assert.equal(typeof effectControl.handler(), "function");

    const timers = timerFixture();
    const brightnessFixture = brightnessBrowserFixture(async function () {
        return {ok: true, json: async function () { return {status: 1}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const brightnessControl = brightnessSliderFixture();
    brightnessFixture.browser.document.querySelectorAll = function (selector) {
        if (selector === "[data-lf-effect-selector]") return [];
        if (selector === "[data-lf-brightness-slider]") return [brightnessControl.slider];
        return [];
    };
    lighting.init(brightnessFixture.browser);
    assert.equal(typeof brightnessControl.handlers.input, "function");
    assert.equal(typeof brightnessFixture.numberHandlers.blur, "function");
    assert.deepEqual(timers.delays, [], "initialization scheduled a brightness status timer");

    const speedFixture = speedBrowserFixture(async function () {
        return {ok: true, json: async function () { return {status: 1}; }};
    });
    const speedControl = speedSliderFixture();
    speedFixture.browser.document.querySelectorAll = function (selector) {
        if (selector === "[data-lf-speed-slider]") return [speedControl.slider];
        if (selector === "[data-lf-color-input]" || selector === "[data-lf-reset-button]") return [];
        return [];
    };
    lighting.init(speedFixture.browser);
    assert.equal(typeof speedControl.handlers.input, "function");
    assert.equal(typeof speedFixture.numberHandlers.blur, "function");

    const combinedEffect = selectorFixture();
    const combinedBrightness = brightnessSliderFixture({dataset: {
        lfClusterControlled: "false",
        lfCurrentBrightness: "40",
        lfDeviceSerial: "openrgb-test-device",
        lfNumberId: "combined-brightness-number",
        lfStatusId: "combined-brightness-status"
    }});
    const combinedSpeed = speedSliderFixture({dataset: Object.assign({}, speedSliderFixture().slider.dataset, {
        lfNumberId: "combined-speed-number",
        lfStatusId: "combined-speed-status"
    })});
    const combinedElements = {
        "effect-status": {textContent: ""},
        "combined-brightness-number": {disabled: false, value: "40", addEventListener: function () {}},
        "combined-brightness-status": {textContent: ""},
        "combined-speed-number": {disabled: false, value: "", addEventListener: function () {}},
        "combined-speed-status": {textContent: ""}
    };
    const combinedBrowser = {
        AbortController,
        LumenForgeRgbSpeed: rgbSpeed,
        clearTimeout,
        document: {
            getElementById: function (id) { return combinedElements[id]; },
            querySelectorAll: function (selector) {
                if (selector === "[data-lf-effect-selector]") return [combinedEffect.selector];
                if (selector === "[data-lf-brightness-slider]") return [combinedBrightness.slider];
                if (selector === "[data-lf-speed-slider]") return [combinedSpeed.slider];
                if (selector === "[data-lf-color-input]" || selector === "[data-lf-reset-button]") return [];
                return [];
            }
        },
        fetch: async function () { return {ok: true, json: async function () { return {status: 1}; }}; },
        location: {reload: function () {}},
        setTimeout
    };
    lighting.init(combinedBrowser);
    assert.equal(typeof combinedEffect.handler(), "function");
    assert.equal(typeof combinedBrightness.handlers.input, "function");
    assert.equal(typeof combinedSpeed.handlers.input, "function");
});
function colorBrowserFixture(fetchImplementation, overrides) {
    const colorHandlers = {};
    const hexHandlers = {};
    const resetHandlers = {};
    const colorInput = {
        disabled: false,
        dataset: {
            lfDeviceSerial: "test-device",
            lfEffect: "static",
            lfCurrentColor: "#ff0000",
            lfHexId: "hex-input",
            lfStatusId: "color-status"
        },
        value: "#ff0000",
        addEventListener: function(event, handler) { colorHandlers[event] = handler; }
    };
    const hexInput = {
        disabled: false,
        value: "#ff0000",
        addEventListener: function(event, handler) { hexHandlers[event] = handler; }
    };
    const status = {textContent: ""};
    const resetButton = {
        disabled: false,
        dataset: {
            lfDeviceSerial: "test-device",
            lfEffect: "static",
            lfStatusId: "reset-status"
        },
        addEventListener: function(event, handler) { resetHandlers[event] = handler; }
    };
    let reloads = 0;
    const browser = {
        AbortController: AbortController,
        document: {
            getElementById: function(id) {
                if (id === "hex-input") return hexInput;
                if (id === "color-status") return status;
                if (id === "reset-status") return status;
                return null;
            }
        },
        fetch: fetchImplementation,
        location: {
            reload: function() { reloads++; }
        },
        setTimeout: setTimeout,
        clearTimeout: clearTimeout
    };
    Object.assign(browser, overrides || {});
    return {browser, colorInput, colorHandlers, hexInput, hexHandlers, resetButton, resetHandlers, status, getReloads: () => reloads};
}

function twoColorBrowserFixture(fetchImplementation, overrides) {
    function input(value) {
        const handlers = {};
        return {
            control: {
                disabled: false,
                value: value,
                addEventListener: function(event, handler) { handlers[event] = handler; }
            },
            handlers
        };
    }

    const startColor = input("#112233");
    const startHex = input("#112233");
    const endColor = input("#aabbcc");
    const endHex = input("#aabbcc");
    const status = {textContent: ""};
    const container = {
        dataset: {
            lfClusterControlled: "false",
            lfCurrentStart: "#112233",
            lfCurrentEnd: "#aabbcc",
            lfDeviceSerial: "test-device",
            lfEffect: "wave",
            lfStartColorId: "start-color",
            lfStartHexId: "start-hex",
            lfEndColorId: "end-color",
            lfEndHexId: "end-hex",
            lfStatusId: "two-color-status"
        }
    };
    const elements = {
        "start-color": startColor.control,
        "start-hex": startHex.control,
        "end-color": endColor.control,
        "end-hex": endHex.control,
        "two-color-status": status
    };
    let reloads = 0;
    const browser = {
        AbortController,
        clearTimeout,
        document: {getElementById: function(id) { return elements[id] || null; }},
        fetch: fetchImplementation,
        location: {reload: function() { reloads++; }},
        setTimeout
    };
    Object.assign(browser, overrides || {});
    return {
        browser,
        container,
        endColor,
        endHex,
        reloads: function() { return reloads; },
        startColor,
        startHex,
        status
    };
}

test("bindTwoColorControl commits normalized complete Start and End pairs", async function(t) {
    for (const role of ["Start", "End"]) {
        await t.test(role, async function() {
            let request;
            const timers = timerFixture();
            const fixture = twoColorBrowserFixture(async function(url, options) {
                request = {url, options};
                return {ok: true, json: async function() { return {status: 1}; }};
            }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
            const commit = lighting.bindTwoColorControl(fixture.browser, fixture.container);

            if (role === "Start") {
                fixture.startHex.control.value = "#DDEEFF";
                fixture.startHex.handlers.input();
                assert.equal(fixture.startColor.control.value, "#ddeeff");
            } else {
                fixture.endColor.control.value = "#445566";
                fixture.endColor.handlers.input();
                assert.equal(fixture.endHex.control.value, "#445566");
            }

            const promise = commit();
            for (const control of [fixture.startColor.control, fixture.startHex.control, fixture.endColor.control, fixture.endHex.control]) {
                assert.equal(control.disabled, true);
            }
            assert.equal(fixture.status.textContent, "Saving colors…");
            await promise;

            assert.equal(request.url, "/api/openrgbimport/two-color");
            assert.equal(request.options.method, "POST");
            assert.ok(request.options.signal instanceof AbortSignal);
            assert.deepEqual(JSON.parse(request.options.body), role === "Start" ? {
                serial: "test-device", effect: "wave", start: "#ddeeff", end: "#aabbcc"
            } : {
                serial: "test-device", effect: "wave", start: "#112233", end: "#445566"
            });
            assert.equal(fixture.status.textContent, "Colors saved.");
            assert.equal(fixture.reloads(), 1);
            assert.equal(timers.pending(10000), 0);
        });
    }
});

test("bindTwoColorControl rejects unchanged invalid incomplete and cluster-owned interactions", async function() {
    let requests = 0;
    const fixture = twoColorBrowserFixture(async function() {
        requests++;
        return {ok: true, json: async function() { return {status: 1}; }};
    });
    const commit = lighting.bindTwoColorControl(fixture.browser, fixture.container);

    await commit();
    fixture.startHex.control.value = "#123";
    await commit();
    fixture.endHex.control.value = "invalid";
    await commit();

    assert.equal(requests, 0);
    assert.equal(fixture.startHex.control.value, "#112233");
    assert.equal(fixture.endHex.control.value, "#aabbcc");

    fixture.container.dataset.lfClusterControlled = "true";
    assert.equal(lighting.bindTwoColorControl(fixture.browser, fixture.container), null);
    assert.equal(requests, 0);
});

test("bindTwoColorControl deduplicates Enter change and concurrent commits", async function() {
    let requests = 0;
    let resolveRequest;
    const fixture = twoColorBrowserFixture(function() {
        requests++;
        return new Promise(function(resolve) { resolveRequest = resolve; });
    });
    lighting.bindTwoColorControl(fixture.browser, fixture.container);
    fixture.startHex.control.value = "#334455";
    fixture.startHex.handlers.input();
    fixture.startHex.handlers.keydown({key: "Enter", preventDefault: function() {}});
    fixture.startHex.handlers.change();
    fixture.endHex.handlers.change();

    assert.equal(requests, 1);
    resolveRequest({ok: true, json: async function() { return {status: 1}; }});
    await new Promise(function(resolve) { setImmediate(resolve); });
    assert.equal(requests, 1);
    assert.equal(fixture.reloads(), 1);
});

test("bindTwoColorControl restores both confirmed colors after request failures", async function(t) {
    const failures = [
        async function() { return {ok: false, json: async function() { return {status: 1}; }}; },
        async function() { return {ok: true, json: async function() { return {status: 0}; }}; },
        async function() { throw new Error("offline detail"); }
    ];
    for (const [index, failure] of failures.entries()) {
        await t.test(String(index), async function() {
            const fixture = twoColorBrowserFixture(failure);
            const commit = lighting.bindTwoColorControl(fixture.browser, fixture.container);
            fixture.startHex.control.value = "#010203";
            fixture.endHex.control.value = "#040506";
            await commit();

            assert.equal(fixture.startColor.control.value, "#112233");
            assert.equal(fixture.startHex.control.value, "#112233");
            assert.equal(fixture.endColor.control.value, "#aabbcc");
            assert.equal(fixture.endHex.control.value, "#aabbcc");
            for (const control of [fixture.startColor.control, fixture.startHex.control, fixture.endColor.control, fixture.endHex.control]) {
                assert.equal(control.disabled, false);
            }
            assert.equal(fixture.status.textContent, "Unable to change colors. Try again.");
            assert.equal(fixture.reloads(), 0);
        });
    }
});

test("bindTwoColorControl aborts a stalled request and restores both colors", async function() {
    const timers = timerFixture();
    let requests = 0;
    let signal;
    const fixture = twoColorBrowserFixture(function(_, options) {
        requests++;
        signal = options.signal;
        return new Promise(function(_, reject) {
            signal.addEventListener("abort", function() {
                const error = new Error("internal timeout detail");
                error.name = "AbortError";
                reject(error);
            }, {once: true});
        });
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const commit = lighting.bindTwoColorControl(fixture.browser, fixture.container);
    fixture.endHex.control.value = "#010203";
    const pending = commit();

    assert.equal(requests, 1);
    assert.equal(signal.aborted, false);
    timers.fireNext(10000);
    await pending;

    assert.equal(signal.aborted, true);
    assert.equal(requests, 1);
    assert.equal(fixture.startHex.control.value, "#112233");
    assert.equal(fixture.endHex.control.value, "#aabbcc");
    assert.equal(fixture.status.textContent, "Unable to change colors. Try again.");
    assert.equal(fixture.reloads(), 0);
});

function temperatureBrowserFixture(fetchImplementation, overrides) {
    function input(value) {
        const handlers = {};
        return {
            disabled: false,
            value: value,
            handlers,
            addEventListener: function(event, handler) { handlers[event] = handler; }
        };
    }
    const elements = {"temperature-status": {textContent: ""}};
    const points = {
        low: {color: input("#00ff00"), hex: input("#00ff00"), celsius: input("20")},
        middle: {color: input("#ffff00"), hex: input("#ffff00"), celsius: input("50")},
        high: {color: input("#ff0000"), hex: input("#ff0000"), celsius: input("95")}
    };
    const dataset = {
        lfClusterControlled: "false", lfDeviceSerial: "test-device", lfEffect: "cpu-temperature",
        lfStatusId: "temperature-status"
    };
    for (const role of ["low", "middle", "high"]) {
        const title = role[0].toUpperCase() + role.slice(1);
        for (const field of ["color", "hex", "celsius"]) {
            const id = role + "-" + field;
            dataset["lf" + title + field[0].toUpperCase() + field.slice(1) + "Id"] = id;
            elements[id] = points[role][field];
        }
    }
    let reloads = 0;
    const browser = {
        AbortController,
        clearTimeout,
        document: {getElementById: function(id) { return elements[id] || null; }},
        fetch: fetchImplementation,
        location: {reload: function() { reloads++; }},
        setTimeout
    };
    Object.assign(browser, overrides || {});
    return {browser, container: {dataset}, points, status: elements["temperature-status"], reloads: function() { return reloads; }};
}

test("bindTemperatureControl commits each semantic point as one complete normalized payload", async function(t) {
    const edits = [
        {role: "low", field: "hex", value: "#A1B2C3"},
        {role: "middle", field: "color", value: "#D4E5F6"},
        {role: "high", field: "hex", value: "#010203"},
        {role: "low", field: "celsius", value: "20.5"},
        {role: "middle", field: "celsius", value: "50.25"},
        {role: "high", field: "celsius", value: "95.75"}
    ];
    for (const edit of edits) {
        await t.test(edit.role + " " + edit.field, async function() {
            let request;
            const timers = timerFixture();
            const fixture = temperatureBrowserFixture(async function(url, options) {
                request = {url, options};
                return {ok: true, json: async function() { return {status: 1}; }};
            }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
            const commit = lighting.bindTemperatureControl(fixture.browser, fixture.container);
            const control = fixture.points[edit.role][edit.field];
            control.value = edit.value;
            if (control.handlers.input) control.handlers.input();
            const pending = commit();
            for (const point of Object.values(fixture.points)) {
                for (const input of Object.values(point)) assert.equal(input.disabled, true);
            }
            assert.equal(fixture.status.textContent, "Saving temperature colors…");
            await pending;
            const body = JSON.parse(request.options.body);
            assert.equal(request.url, "/api/openrgbimport/temperature");
            assert.ok(request.options.signal instanceof AbortSignal);
            assert.deepEqual(Object.keys(body), ["serial", "effect", "low", "middle", "high"]);
            if (edit.field === "celsius") assert.equal(body[edit.role].celsius, Number(edit.value));
            else assert.equal(body[edit.role].color, edit.value.toLowerCase());
            assert.equal(fixture.status.textContent, "Temperature colors saved.");
            assert.equal(fixture.reloads(), 1);
            assert.equal(timers.pending(10000), 0);
        });
    }
});

test("bindTemperatureControl rejects incomplete unchanged and unordered state locally", async function() {
    let requests = 0;
    const fixture = temperatureBrowserFixture(async function() {
        requests++;
        return {ok: true, json: async function() { return {status: 1}; }};
    });
    const commit = lighting.bindTemperatureControl(fixture.browser, fixture.container);
    await commit();
    fixture.points.low.hex.value = "invalid";
    await commit();
    fixture.points.middle.celsius.value = "";
    await commit();
    fixture.points.low.celsius.value = "50";
    await commit();
    assert.equal(requests, 0);
    assert.equal(fixture.points.low.celsius.value, "20");
    assert.equal(fixture.status.textContent, "Thresholds must satisfy Low < Middle < High.");

    fixture.container.dataset.lfClusterControlled = "true";
    assert.equal(lighting.bindTemperatureControl(fixture.browser, fixture.container), null);
});

test("bindTemperatureControl disables malformed or unordered server-rendered temperature state", function() {
    const cases = [
        {
            name: "malformed",
            prepare: function(fixture) {
                fixture.points.low.hex.value = "invalid";
            },
            message: "Unable to load temperature settings."
        },
        {
            name: "unordered",
            prepare: function(fixture) {
                fixture.points.low.celsius.value = "50";
            },
            message: "Thresholds must satisfy Low < Middle < High."
        }
    ];

    for (const testCase of cases) {
        const fixture = temperatureBrowserFixture(async function() {
            throw new Error("invalid rendered state must not make a request");
        });

        testCase.prepare(fixture);

        assert.equal(lighting.bindTemperatureControl(fixture.browser, fixture.container), null);

        const controls = [
            fixture.points.low.color,
            fixture.points.low.hex,
            fixture.points.low.celsius,
            fixture.points.middle.color,
            fixture.points.middle.hex,
            fixture.points.middle.celsius,
            fixture.points.high.color,
            fixture.points.high.hex,
            fixture.points.high.celsius
        ];

        for (const control of controls) {
            assert.equal(control.disabled, true, testCase.name + " control was left enabled");
        }
        assert.equal(fixture.status.textContent, testCase.message);
    }
});

test("bindTemperatureControl deduplicates Enter change and concurrent commits", async function() {
    let requests = 0;
    let resolveRequest;
    const fixture = temperatureBrowserFixture(function() {
        requests++;
        return new Promise(function(resolve) { resolveRequest = resolve; });
    });
    lighting.bindTemperatureControl(fixture.browser, fixture.container);
    fixture.points.middle.celsius.value = "55";
    fixture.points.middle.celsius.handlers.keydown({key: "Enter", preventDefault: function() {}});
    fixture.points.middle.celsius.handlers.change();
    fixture.points.high.hex.handlers.change();
    assert.equal(requests, 1);
    resolveRequest({ok: true, json: async function() { return {status: 1}; }});
    await new Promise(function(resolve) { setImmediate(resolve); });
    assert.equal(requests, 1);
    assert.equal(fixture.reloads(), 1);
});

test("bindTemperatureControl restores all confirmed values after failures and timeout", async function(t) {
    const failures = [
        async function() { return {ok: false, json: async function() { return {status: 1}; }}; },
        async function() { return {ok: true, json: async function() { return {status: 0}; }}; },
        async function() { throw new Error("offline"); }
    ];
    for (const failure of failures) {
        await t.test("failure", async function() {
            const fixture = temperatureBrowserFixture(failure);
            const commit = lighting.bindTemperatureControl(fixture.browser, fixture.container);
            fixture.points.low.hex.value = "#010203";
            fixture.points.middle.celsius.value = "55";
            await commit();
            assert.equal(fixture.points.low.hex.value, "#00ff00");
            assert.equal(fixture.points.middle.celsius.value, "50");
            assert.equal(fixture.status.textContent, "Unable to change temperature colors. Try again.");
            for (const point of Object.values(fixture.points)) {
                for (const input of Object.values(point)) assert.equal(input.disabled, false);
            }
            assert.equal(fixture.reloads(), 0);
        });
    }

    await t.test("timeout", async function() {
        const timers = timerFixture();
        let signal;
        const fixture = temperatureBrowserFixture(function(_, options) {
            signal = options.signal;
            return new Promise(function(_, reject) {
                signal.addEventListener("abort", function() { reject(new Error("timeout")); }, {once: true});
            });
        }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
        const commit = lighting.bindTemperatureControl(fixture.browser, fixture.container);
        fixture.points.high.celsius.value = "96";
        const pending = commit();
        timers.fireNext(10000);
        await pending;
        assert.equal(signal.aborted, true);
        assert.equal(fixture.points.high.celsius.value, "95");
        assert.equal(fixture.status.textContent, "Unable to change temperature colors. Try again.");
    });
});

test("bindColorControl normalizes and commits a valid color and reloads on success", async function() {
    let requested = false;
    const {browser, colorInput, colorHandlers, hexInput, hexHandlers, getReloads, status} = colorBrowserFixture(async function(url, options) {
        assert.equal(url, "/api/openrgbimport/single-color");
        const body = JSON.parse(options.body);
        assert.equal(body.color, "#00ff00");
        assert.equal(body.serial, "test-device");
        assert.equal(body.effect, "static");
        requested = true;
        return {ok: true, json: async () => ({status: 1})};
    });

    const commit = lighting.bindColorControl(browser, colorInput);
    hexInput.value = "#00FF00";
    hexHandlers.input(); // trigger preview
    assert.equal(colorInput.value, "#00ff00");

    const promise = commit(colorInput.value);
    assert.ok(colorInput.disabled);
    assert.ok(hexInput.disabled);
    assert.equal(status.textContent, "Saving color…");

    await promise;

    assert.ok(requested);
    assert.equal(getReloads(), 1);
    assert.equal(status.textContent, "Color saved.");
});

test("bindColorControl restores confirmed state and controls on failure", async function() {
    let requested = false;
    const {browser, colorInput, colorHandlers, hexInput, hexHandlers, getReloads, status} = colorBrowserFixture(async function(url, options) {
        requested = true;
        return {ok: false, json: async () => ({})};
    });

    const commit = lighting.bindColorControl(browser, colorInput);

    colorInput.value = "#0000ff";
    const promise = commit(colorInput.value);

    await promise;

    assert.ok(requested);
    assert.equal(getReloads(), 0);
    assert.equal(status.textContent, "Unable to change color. Try again.");
    assert.ok(!colorInput.disabled);
    assert.ok(!hexInput.disabled);
    assert.equal(colorInput.value, "#ff0000"); // restored to original
    assert.equal(hexInput.value, "#ff0000"); // restored to original
});

test("bindColorControl ignores invalid colors and does not make requests", async function() {
    let requested = false;
    const {browser, colorInput, colorHandlers, hexInput} = colorBrowserFixture(async function(url, options) {
        requested = true;
        return {ok: true, json: async () => ({status: 1})};
    });

    const commit = lighting.bindColorControl(browser, colorInput);

    colorInput.value = "invalid";
    await commit(colorInput.value);

    assert.ok(!requested);
    assert.equal(colorInput.value, "#ff0000"); // restored to original
});

test("bindResetButton commits a reset and reloads on success", async function() {
    let requested = false;
    const {browser, resetButton, resetHandlers, getReloads, status} = colorBrowserFixture(async function(url, options) {
        assert.equal(url, "/api/openrgbimport/effect-reset");
        const body = JSON.parse(options.body);
        assert.equal(body.serial, "test-device");
        assert.equal(body.effect, "static");
        requested = true;
        return {ok: true, json: async () => ({status: 1})};
    });

    const commit = lighting.bindResetButton(browser, resetButton);

    const promise = commit();
    assert.ok(resetButton.disabled);
    assert.equal(status.textContent, "Resetting effect…");

    await promise;

    assert.ok(requested);
    assert.equal(getReloads(), 1);
    assert.equal(status.textContent, "Effect reset.");
});

test("bindResetButton restores control on failure", async function() {
    let requested = false;
    const {browser, resetButton, resetHandlers, getReloads, status} = colorBrowserFixture(async function(url, options) {
        requested = true;
        return {ok: false, json: async () => ({})};
    });

    const commit = lighting.bindResetButton(browser, resetButton);

    const promise = commit();
    assert.ok(resetButton.disabled);
    assert.equal(status.textContent, "Resetting effect…");

    await promise;

    assert.ok(requested);
    assert.equal(getReloads(), 0);
    assert.ok(!resetButton.disabled);
    assert.equal(status.textContent, "Unable to reset effect. Try again.");
});

test("bindResetButton aborts a stalled request and restores control", async function() {
    let aborted = false;
    let promiseResolve;
    const fetchPromise = new Promise(resolve => { promiseResolve = resolve; });
    const timers = [];

    const {browser, resetButton, status} = colorBrowserFixture(async function(url, options) {
        options.signal.addEventListener("abort", () => { aborted = true; });
        await fetchPromise;
        if (aborted) throw new Error("AbortError");
        return {ok: true, json: async () => ({status: 1})};
    }, {
        setTimeout: function(fn, ms) { timers.push(fn); return timers.length; },
        clearTimeout: function() {}
    });

    const commit = lighting.bindResetButton(browser, resetButton);
    const promise = commit();

    assert.equal(timers.length, 1); // abort timer
    timers[0](); // trigger timeout
    promiseResolve();

    await promise;

    assert.ok(aborted);
    assert.ok(!resetButton.disabled);
    assert.equal(status.textContent, "Unable to reset effect. Try again.");
});

function gradientBrowserFixture(fetchImplementation, overrides, initialStops) {
    function matches(element, selector) {
        if (selector === "input, button") return element.tagName === "input" || element.tagName === "button";
        const match = /^\[data-lf-([a-z-]+)\]$/.exec(selector);
        if (!match) return false;
        const key = "lf" + match[1].split("-").map(function(part) { return part[0].toUpperCase() + part.slice(1); }).join("");
        return Object.prototype.hasOwnProperty.call(element.dataset, key);
    }
    function element(tagName) {
        const handlers = {};
        const value = {
            tagName,
            children: [],
            className: "",
            dataset: {},
            disabled: false,
            handlers,
            textContent: "",
            value: "",
            addEventListener: function(event, handler) { handlers[event] = handler; },
            appendChild: function(child) { this.children.push(child); return child; },
            append: function(...children) { this.children.push(...children); },
            replaceChildren: function(...children) { this.children = children; },
            setAttribute: function(name, attributeValue) { this[name] = String(attributeValue); },
            querySelectorAll: function(selector) {
                const found = [];
                function visit(parent) {
                    for (const child of parent.children) {
                        if (matches(child, selector)) found.push(child);
                        visit(child);
                    }
                }
                visit(this);
                return found;
            },
            querySelector: function(selector) { return this.querySelectorAll(selector)[0] || null; }
        };
        return value;
    }
    function input(type, value, dataKey) {
        const control = element("input");
        control.type = type;
        control.value = String(value);
        control.dataset[dataKey] = "";
        return control;
    }
    function makeRow(stop, number) {
        const row = element("div");
        row.dataset.lfGradientStop = "";
        const heading = element("h4");
        heading.dataset.lfGradientStopNumber = "";
        heading.textContent = "Stop " + number;
        const color = input("color", stop.color, "lfGradientColor");
        const hex = input("text", stop.color, "lfGradientHex");
        const position = input("number", stop.position, "lfGradientPosition");
        const intensity = input("number", stop.intensity, "lfGradientIntensity");
        const remove = element("button");
        remove.dataset.lfGradientRemove = "";
        row.append(heading, color, hex, position, intensity, remove);
        return row;
    }

    const stops = initialStops || [
        {position: 0, color: "#ff0000", intensity: 1},
        {position: 0.33, color: "#00ff00", intensity: 1},
        {position: 0.66, color: "#0000ff", intensity: 1},
        {position: 1, color: "#ffff00", intensity: 1}
    ];
    const container = element("section");
    container.dataset = {
        lfClusterControlled: "false", lfDeviceSerial: "gradient-device", lfEffect: "gradient",
        lfRowsId: "gradient-rows", lfAddId: "gradient-add", lfSaveId: "gradient-save", lfStatusId: "gradient-status"
    };
    const rows = element("div");
    rows.id = "gradient-rows";
    rows.children = stops.map(makeRow);
    const add = element("button");
    add.id = "gradient-add";
    const save = element("button");
    save.id = "gradient-save";
    save.disabled = true;
    const status = element("p");
    status.id = "gradient-status";
    container.append(rows, add, save, status);
    const byID = {"gradient-rows": rows, "gradient-add": add, "gradient-save": save, "gradient-status": status};
    let reloads = 0;
    const browser = {
        AbortController,
        clearTimeout,
        document: {
            createElement: element,
            getElementById: function(id) { return byID[id] || null; }
        },
        fetch: fetchImplementation,
        location: {reload: function() { reloads++; }},
        setTimeout
    };
    Object.assign(browser, overrides || {});
    return {
        add, browser, container, rows, save, status,
        reloads: function() { return reloads; },
        stopRows: function() { return rows.querySelectorAll("[data-lf-gradient-stop]"); }
    };
}

test("bindGradientControl initializes a complete draft and saves a stable ordered payload", async function() {
    let request;
    const fixture = gradientBrowserFixture(async function(url, options) {
        request = {url, options};
        return {ok: true, json: async function() { return {status: 1}; }};
    });
    const controller = lighting.bindGradientControl(fixture.browser, fixture.container);
    assert.ok(controller);
    assert.equal(fixture.stopRows().length, 4);
    assert.equal(fixture.save.disabled, true);

    const first = fixture.stopRows()[0];
    const second = fixture.stopRows()[1];
    const third = fixture.stopRows()[2];
    const firstColor = first.querySelector("[data-lf-gradient-color]");
    const firstHex = first.querySelector("[data-lf-gradient-hex]");
    firstColor.value = "#112233";
    firstColor.handlers.input();
    assert.equal(firstHex.value, "#112233");
    firstHex.value = "#AABBCC";
    firstHex.handlers.input();
    assert.equal(firstColor.value, "#aabbcc");
    assert.equal(second.querySelector("[data-lf-gradient-hex]").value, "#00ff00");

    second.querySelector("[data-lf-gradient-position]").value = "0.5";
    second.querySelector("[data-lf-gradient-position]").handlers.input();
    third.querySelector("[data-lf-gradient-position]").value = "0.5";
    third.querySelector("[data-lf-gradient-position]").handlers.input();
    first.querySelector("[data-lf-gradient-intensity]").value = "0";
    first.querySelector("[data-lf-gradient-intensity]").handlers.input();
    assert.equal(request, undefined);
    assert.equal(fixture.save.disabled, false);

    const pending = controller.save();
    assert.equal(fixture.status.textContent, "Saving Gradient…");
    for (const control of fixture.container.querySelectorAll("input, button")) assert.equal(control.disabled, true);
    await pending;
    const body = JSON.parse(request.options.body);
    assert.equal(request.url, "/api/openrgbimport/gradient");
    assert.ok(request.options.signal instanceof AbortSignal);
    assert.equal(body.stops[0].color, "#aabbcc");
    assert.equal(body.stops[0].position, 0);
    assert.equal(body.stops[0].intensity, 0);
    assert.equal(body.stops[1].color, "#00ff00");
    assert.equal(body.stops[2].color, "#0000ff");
    assert.equal(body.stops[1].position, 0.5);
    assert.equal(body.stops[2].position, 0.5);
    assert.equal(fixture.status.textContent, "Gradient saved.");
    assert.equal(fixture.reloads(), 1);
});

test("bindGradientControl Add and Remove remain local and preserve minimum and maximum sizes", function() {
    let requests = 0;
    const fixture = gradientBrowserFixture(async function() { requests++; return {ok: true, json: async function() { return {status: 1}; }}; });
    const controller = lighting.bindGradientControl(fixture.browser, fixture.container);
    controller.addStop();
    assert.equal(requests, 0);
    assert.equal(fixture.stopRows().length, 5);
    const inserted = fixture.stopRows()[3];
    assert.equal(inserted.querySelector("[data-lf-gradient-position]").value, "0.8300000000000001");
    assert.equal(inserted.querySelector("[data-lf-gradient-hex]").value, "#0000ff");
    inserted.querySelector("[data-lf-gradient-remove]").handlers.click();
    assert.equal(fixture.stopRows().length, 4);
    assert.equal(requests, 0);

    const minimum = gradientBrowserFixture(async function() {}, null, [
        {position: 0, color: "#000000", intensity: 1}, {position: 1, color: "#ffffff", intensity: 1}
    ]);
    lighting.bindGradientControl(minimum.browser, minimum.container);
    for (const row of minimum.stopRows()) assert.equal(row.querySelector("[data-lf-gradient-remove]").disabled, true);

    const maximumStops = Array.from({length: 1024}, function(_, index) {
        return {position: index / 1023, color: "#000000", intensity: 1};
    });
    const maximum = gradientBrowserFixture(async function() {}, null, maximumStops);
    lighting.bindGradientControl(maximum.browser, maximum.container);
    assert.equal(maximum.add.disabled, true);
});

test("bindGradientControl Add reports invalid drafts without requests or mutation", async function(t) {
    const cases = [
        {name: "incomplete", field: "position", value: "", message: "Every Gradient stop needs a valid color, position, and intensity."},
        {name: "out of range", field: "intensity", value: "1.1", message: "Gradient positions and intensities must be between 0 and 1."}
    ];
    for (const testCase of cases) {
        await t.test(testCase.name, function() {
            let requests = 0;
            const fixture = gradientBrowserFixture(async function() { requests++; });
            const controller = lighting.bindGradientControl(fixture.browser, fixture.container);
            const initialCount = fixture.stopRows().length;
            fixture.stopRows()[0].querySelector("[data-lf-gradient-" + testCase.field + "]").value = testCase.value;
            controller.addStop();
            assert.equal(requests, 0);
            assert.equal(fixture.stopRows().length, initialCount);
            assert.equal(fixture.status.textContent, testCase.message);
        });
    }
});

test("bindGradientControl validates malformed drafts without requests", async function(t) {
    const cases = [
        {name: "malformed color", field: "hex", value: "red", message: "Every Gradient stop needs a valid color, position, and intensity."},
        {name: "blank Position", field: "position", value: "", message: "Every Gradient stop needs a valid color, position, and intensity."},
        {name: "blank Intensity", field: "intensity", value: "", message: "Every Gradient stop needs a valid color, position, and intensity."},
        {name: "Position below range", field: "position", value: "-0.1", message: "Gradient positions and intensities must be between 0 and 1."},
        {name: "Intensity above range", field: "intensity", value: "1.1", message: "Gradient positions and intensities must be between 0 and 1."}
    ];
    for (const testCase of cases) {
        await t.test(testCase.name, async function() {
            let requests = 0;
            const fixture = gradientBrowserFixture(async function() { requests++; });
            const controller = lighting.bindGradientControl(fixture.browser, fixture.container);
            fixture.stopRows()[0].querySelector("[data-lf-gradient-" + testCase.field + "]").value = testCase.value;
            await controller.save();
            assert.equal(requests, 0);
            assert.equal(fixture.status.textContent, testCase.message);
        });
    }
});

test("bindGradientControl keeps a coherent retryable draft after failures and timeout", async function(t) {
    const failures = [
        async function() { return {ok: false, json: async function() { return {status: 1}; }}; },
        async function() { return {ok: true, json: async function() { return {status: 0}; }}; },
        async function() { throw new Error("offline"); }
    ];
    for (const failure of failures) {
        await t.test("failure", async function() {
            const fixture = gradientBrowserFixture(failure);
            const controller = lighting.bindGradientControl(fixture.browser, fixture.container);
            const position = fixture.stopRows()[1].querySelector("[data-lf-gradient-position]");
            position.value = "0.4";
            await controller.save();
            assert.equal(position.value, "0.4");
            assert.equal(fixture.status.textContent, "Unable to change Gradient. Try again.");
            assert.equal(fixture.save.disabled, false);
            assert.equal(fixture.reloads(), 0);
        });
    }

    await t.test("timeout and in-flight deduplication", async function() {
        const timers = timerFixture();
        let requests = 0;
        let signal;
        const fixture = gradientBrowserFixture(function(_, options) {
            requests++;
            signal = options.signal;
            return new Promise(function(_, reject) {
                signal.addEventListener("abort", function() { reject(new Error("timeout")); }, {once: true});
            });
        }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
        const controller = lighting.bindGradientControl(fixture.browser, fixture.container);
        const position = fixture.stopRows()[1].querySelector("[data-lf-gradient-position]");
        position.value = "0.4";
        const pending = controller.save();
        controller.save();
        fixture.save.handlers.click();
        assert.equal(requests, 1);
        timers.fireNext(10000);
        await pending;
        assert.equal(signal.aborted, true);
        assert.equal(position.value, "0.4");
        assert.equal(fixture.save.disabled, false);
        assert.equal(fixture.status.textContent, "Unable to change Gradient. Try again.");
    });
});

test("bindGradientControl disables malformed initial state and ignores cluster-owned or absent editors", function() {
    const malformed = gradientBrowserFixture(async function() {}, null, [
        {position: 0.8, color: "#000000", intensity: 1}, {position: 0.2, color: "#ffffff", intensity: 1}
    ]);
    assert.equal(lighting.bindGradientControl(malformed.browser, malformed.container), null);
    assert.equal(malformed.status.textContent, "Unable to load Gradient settings.");
    for (const control of malformed.container.querySelectorAll("input, button")) assert.equal(control.disabled, true);

    const cluster = gradientBrowserFixture(async function() {});
    cluster.container.dataset.lfClusterControlled = "true";
    assert.equal(lighting.bindGradientControl(cluster.browser, cluster.container), null);

    const selectors = [];
    lighting.init({document: {querySelectorAll: function(selector) { selectors.push(selector); return []; }}});
    assert.ok(selectors.includes("[data-lf-gradient-control]"));
});

test("shared lighting controls use dedicated serial-free RGB Cluster mutations", async function() {
    const requests = [];
    async function fetchMutation(url, options) {
        requests.push({url, body: JSON.parse(options.body)});
        return {ok: true, json: async function() { return {status: 1}; }};
    }
    function timers() {
        const fixture = timerFixture();
        return {clearTimeout: fixture.clearTimeout, setTimeout: fixture.setTimeout};
    }

    const effect = selectorFixture();
    effect.selector.dataset.lfLightingTarget = "cluster";
    const effectBrowser = browserFixture(fetchMutation, timers());
    lighting.bindEffectSelector(effectBrowser.browser, effect.selector);
    await effect.handler()();

    const brightness = brightnessSliderFixture();
    brightness.slider.dataset.lfLightingTarget = "cluster";
    const brightnessBrowser = brightnessBrowserFixture(fetchMutation, timers());
    lighting.bindBrightnessSlider(brightnessBrowser.browser, brightness.slider);
    brightness.slider.value = "41";
    await brightness.handlers.change();

    const speed = speedSliderFixture();
    speed.slider.dataset.lfLightingTarget = "cluster";
    const speedBrowser = speedBrowserFixture(fetchMutation, timers());
    lighting.bindSpeedSlider(speedBrowser.browser, speed.slider);
    speed.slider.value = "5";
    speed.handlers.input();
    await speed.handlers.change();
    assert.equal(speedBrowser.resetControl.hidden, true, "Cluster Speed mutation revealed OpenRGB Reset");

    const color = colorBrowserFixture(fetchMutation, timers());
    color.colorInput.dataset.lfLightingTarget = "cluster";
    const commitColor = lighting.bindColorControl(color.browser, color.colorInput);
    await commitColor("#00ff00");

    const twoColor = twoColorBrowserFixture(fetchMutation, timers());
    twoColor.container.dataset.lfLightingTarget = "cluster";
    const commitTwoColor = lighting.bindTwoColorControl(twoColor.browser, twoColor.container);
    twoColor.startHex.control.value = "#ddeeff";
    await commitTwoColor();

    const temperature = temperatureBrowserFixture(fetchMutation, timers());
    temperature.container.dataset.lfLightingTarget = "cluster";
    const commitTemperature = lighting.bindTemperatureControl(temperature.browser, temperature.container);
    temperature.points.high.celsius.value = "96";
    await commitTemperature();

    const gradient = gradientBrowserFixture(fetchMutation, timers());
    gradient.container.dataset.lfLightingTarget = "cluster";
    const gradientController = lighting.bindGradientControl(gradient.browser, gradient.container);
    gradient.stopRows()[1].querySelector("[data-lf-gradient-position]").value = "0.4";
    await gradientController.save();

    assert.deepEqual(requests.map(function(request) { return request.url; }), [
        "/api/cluster/lighting/effect",
        "/api/cluster/lighting/brightness",
        "/api/cluster/lighting/speed",
        "/api/cluster/lighting/single-color",
        "/api/cluster/lighting/two-color",
        "/api/cluster/lighting/temperature",
        "/api/cluster/lighting/gradient"
    ]);
    assert.deepEqual(requests[0].body, {effect: "wave"});
    assert.deepEqual(requests[1].body, {brightness: 41});
    assert.deepEqual(requests[2].body, {effect: "circle", speed: 6});
    assert.deepEqual(requests[3].body, {effect: "static", color: "#00ff00"});
    assert.deepEqual(requests[4].body, {effect: "wave", start: "#ddeeff", end: "#aabbcc"});
    assert.deepEqual(Object.keys(requests[5].body), ["effect", "low", "middle", "high"]);
    assert.equal(requests[5].body.effect, "cpu-temperature");
    assert.equal(requests[5].body.high.celsius, 96);
    assert.equal(requests[6].body.effect, "gradient");
    assert.equal(requests[6].body.stops[1].position, 0.4);
    for (const request of requests) {
        assert.equal(Object.prototype.hasOwnProperty.call(request.body, "serial"), false);
        assert.doesNotMatch(request.url, /^\/api\/(?:color|brightness\/gradual)/);
    }
});

test("RGB Cluster mutation failure rolls back and cannot expose OpenRGB Reset", async function() {
    const timers = timerFixture();
    let requests = 0;
    const fixture = brightnessBrowserFixture(async function() {
        requests++;
        return {ok: true, json: async function() { return {status: 0}; }};
    }, {clearTimeout: timers.clearTimeout, setTimeout: timers.setTimeout});
    const control = brightnessSliderFixture();
    control.slider.dataset.lfLightingTarget = "cluster";
    lighting.bindBrightnessSlider(fixture.browser, control.slider);
    control.slider.value = "70";
    await control.handlers.change();

    assert.equal(requests, 1);
    assert.equal(control.slider.value, "40");
    assert.equal(fixture.numberInput.value, "40");
    assert.equal(fixture.status.textContent, "Unable to change brightness. Try again.");
    assert.equal(fixture.resetControl.hidden, true);

    const resetFixture = colorBrowserFixture(async function() { requests++; });
    resetFixture.resetButton.dataset.lfLightingTarget = "cluster";
    assert.equal(lighting.bindResetButton(resetFixture.browser, resetFixture.resetButton), null);
    assert.equal(requests, 1);
});
