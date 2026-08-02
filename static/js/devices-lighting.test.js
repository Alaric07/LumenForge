"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const lighting = require("./devices-lighting.js");

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
            entry[1].callback();
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
                assert.equal(selector, "[data-lf-brightness-readout]");
                return [hero, secondReadout, unrelatedReadout];
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
        secondReadout,
        status,
        unrelatedReadout
    };
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

    assert.deepEqual(selectors, ["[data-lf-effect-selector]", "[data-lf-brightness-slider]"]);
});

test("Lighting initialization supports an isolated effect selector or brightness control", function () {
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
        return [brightnessControl.slider];
    };
    lighting.init(brightnessFixture.browser);
    assert.equal(typeof brightnessControl.handlers.input, "function");
    assert.equal(typeof brightnessFixture.numberHandlers.blur, "function");
    assert.deepEqual(timers.delays, [], "initialization scheduled a brightness status timer");
});
