"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const controller = require("./devices-controller.js");

test("canvasPosition converts CSS pixels into canvas coordinates and rejects zero-sized canvases", function () {
    const canvas = {width: 480, height: 260, getBoundingClientRect: function () { return {left: 20, top: 10, width: 240, height: 130}; }};
    assert.deepEqual(controller.canvasPosition(canvas, {clientX: 140, clientY: 75}), {x: 240, y: 130, width: 480, height: 260});
    canvas.getBoundingClientRect = function () { return {left: 0, top: 0, width: 0, height: 130}; };
    assert.equal(controller.canvasPosition(canvas, {clientX: 1, clientY: 1}), null);
});

test("disclosure redraws only after an analog editor is opened", function () {
    let toggle; let draws = 0;
    const details = {tagName: "DETAILS", open: false, addEventListener: function (name, listener) { assert.equal(name, "toggle"); toggle = listener; }};
    controller.bindDisclosureRedraw(details, function () { draws++; });
    toggle(); assert.equal(draws, 0);
    details.open = true; toggle(); assert.equal(draws, 1);
});
