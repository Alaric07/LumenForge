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
    const delays = [];
    const cleared = [];
    return {
        setTimeout: function (callback, delay) {
            const id = nextID++;
            scheduled.set(id, callback);
            delays.push(delay);
            return id;
        },
        clearTimeout: function (id) {
            cleared.push(id);
            scheduled.delete(id);
        },
        fireNext: function () {
            const entry = scheduled.entries().next();
            assert.equal(entry.done, false, "expected a pending timeout");
            entry.value[1]();
        },
        delays,
        cleared,
        pending: function () { return scheduled.size; }
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
