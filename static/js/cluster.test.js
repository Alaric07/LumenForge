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
    assert.deepEqual(Object.keys(cluster), ["bindMemberOrdering"]);
});

test("RGB Cluster ordering binding is inert without a sortable member surface", function () {
    let requests = 0;
    function $() { return {length: 0}; }
    $.ajax = function () { requests++; };

    assert.equal(cluster.bindMemberOrdering($), false);
    assert.equal(requests, 0);
});
