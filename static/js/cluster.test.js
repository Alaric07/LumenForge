"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const cluster = require("./cluster.js");

test("RGB Cluster frontend preserves ordered member persistence only", function () {
    let sortableOptions;
    let request;
    const body = {};
    const rows = [{serial: "member-two"}, {serial: "member-one"}];
    const sortable = {
        length: 1,
        sortable: function (options) {
            sortableOptions = options;
            return this;
        },
        disableSelection: function () { return this; }
    };

    function $(value) {
        if (value === "#clusterSortable") return sortable;
        if (value === body) {
            return {
                children: function (selector) {
                    assert.equal(selector, "tr");
                    return {each: function (callback) { rows.forEach(function (row) { callback.call(row); }); }};
                }
            };
        }
        if (rows.includes(value)) {
            return {data: function (name) { assert.equal(name, "serial"); return value.serial; }};
        }
        assert.fail("unexpected jQuery value");
    }
    $.ajax = function (options) { request = options; };

    assert.equal(cluster.bindMemberOrdering($), true);
    assert.equal(sortableOptions.axis, "y");
    sortableOptions.update.call(body);

    assert.equal(request.url, "/api/cluster/order");
    assert.equal(request.type, "PUT");
    assert.equal(request.contentType, "application/json");
    assert.deepEqual(JSON.parse(request.data), {deviceOrder: ["member-two", "member-one"]});
    assert.deepEqual(Object.keys(cluster), ["bindMemberOrdering", "bindLightingStatus"]);
});

function statusBrowser(currentEffect, fetch) {
    const timers = [];
    const selector = currentEffect === null ? null : {dataset: {lfCurrentEffect: currentEffect}};
    let reloads = 0;
    return {
        document: {querySelector: function (query) { assert.equal(query, '[data-lf-effect-selector][data-lf-lighting-target="cluster"]'); return selector; }},
        fetch: fetch,
        setInterval: function (callback, delay) { const timer = {callback: callback, delay: delay, cleared: false}; timers.push(timer); return timer; },
        clearInterval: function (timer) { timer.cleared = true; },
        location: {reload: function () { reloads++; }},
        timers: timers,
        reloads: function () { return reloads; }
    };
}

function statusResponse(status, effect) { return {ok: true, json: async function () { return {status: status, effect: effect}; }}; }

test("RGB Cluster lighting status binding is inert without a Cluster selector", function () {
    let polls = 0;
    const browser = statusBrowser(null, async function () { polls++; return statusResponse(1, "wave"); });
    assert.equal(cluster.bindLightingStatus(browser), false);
    assert.equal(browser.timers.length, 0);
    assert.equal(polls, 0);
});

test("RGB Cluster lighting status reloads once only when canonical effect changes", async function () {
    const browser = statusBrowser("rainbow", async function () { return statusResponse(1, "wave"); });
    assert.equal(cluster.bindLightingStatus(browser), true);
    assert.equal(browser.timers[0].delay, 1000);
    await browser.timers[0].callback();
    await browser.timers[0].callback();
    assert.equal(browser.reloads(), 1);
    assert.equal(browser.timers[0].cleared, true);
});

test("RGB Cluster lighting status does not reload for matching or unavailable results", async function () {
    const responses = [statusResponse(1, "rainbow"), statusResponse(0, ""), {ok: false}];
    const browser = statusBrowser("rainbow", async function () { return responses.shift(); });
    assert.equal(cluster.bindLightingStatus(browser), true);
    await browser.timers[0].callback();
    await browser.timers[0].callback();
    await browser.timers[0].callback();
    assert.equal(browser.reloads(), 0);
    assert.equal(browser.timers[0].cleared, false);
});

test("RGB Cluster ordering binding is inert without a sortable member surface", function () {
    let requests = 0;
    function $() { return {length: 0}; }
    $.ajax = function () { requests++; };

    assert.equal(cluster.bindMemberOrdering($), false);
    assert.equal(requests, 0);
});
