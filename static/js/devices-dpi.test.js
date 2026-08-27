"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const dpi = require("./devices-dpi.js");

test("DPI editor accepts exact in-range integers only", function () {
    assert.equal(dpi.parseDPI("100", 100, 18000), 100);
    assert.equal(dpi.parseDPI("18000", 100, 18000), 18000);
    assert.equal(dpi.parseDPI("099", 100, 18000), null);
    assert.equal(dpi.parseDPI("100.5", 100, 18000), null);
    assert.equal(dpi.parseDPI("18001", 100, 18000), null);
});

test("DPI editor normalizes hex colors", function () {
    assert.equal(dpi.normalizeColor("#AAbbCC"), "#aabbcc");
    assert.equal(dpi.normalizeColor("blue"), null);
});
