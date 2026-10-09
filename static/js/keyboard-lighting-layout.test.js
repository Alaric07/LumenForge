"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const css = fs.readFileSync(path.join(__dirname, "../css/app-shell.css"), "utf8");

function rule(selector) {
    const start = css.indexOf(selector + " {");
    assert.ok(start >= 0, "missing " + selector);
    return css.slice(start, css.indexOf("}", start));
}

const canvas = rule(".lf-app-shell .lf-authored-zone-list-keyboard");
const key = rule(".lf-app-shell .lf-authored-zone-list-keyboard .lf-authored-zone");
function close(actual, expected) {
    assert.ok(Math.abs(actual - expected) < 1e-9, `${actual} != ${expected}`);
}

const rem = 16;
const pitch = Number(canvas.match(/--lf-authored-key-pitch: ([\d.]+)rem/)[1]) * rem;
const gutter = Number(canvas.match(/--lf-authored-key-gutter: ([\d.]+)rem/)[1]) * rem;

test("keyboard CSS sizes the canvas before keys and preserves a scrollable minimum", () => {
    assert.match(canvas, /width: max\(100%, calc\(var\(--lf-authored-zone-layout-width\) \/ var\(--lf-authored-key-unit\) \* var\(--lf-authored-key-pitch\)\)\)/);
    const scroll = rule(".lf-app-shell .lf-authored-keyboard-scroll");
    assert.match(scroll, /min-width: 0;/);
    assert.match(scroll, /overflow-x: auto;/);
    assert.match(scroll, /max-width: 56rem;/);
    assert.match(key, /box-sizing: border-box;/);
    assert.match(key, /min-width: 0;/);
    assert.match(key, /min-height: 0;/);
    // Independent acceptance thresholds, rather than just copying CSS constants.
    assert.ok(pitch - gutter >= 32, "1u keys must remain readable");
    assert.ok(gutter >= 4, "visible horizontal and vertical gutters required");
});

test("keyboard CSS insets all four faces without moving logical cells", () => {
    assert.match(key, /left: calc\(100% \* var\(--lf-authored-zone-left\) \/ var\(--lf-authored-zone-layout-width\) \+ var\(--lf-authored-key-gutter\) \/ 2\);/);
    assert.match(key, /top: calc\(100% \* var\(--lf-authored-zone-top\) \/ var\(--lf-authored-zone-layout-height\) \+ var\(--lf-authored-key-gutter\) \/ 2\);/);
    assert.match(key, /width: calc\(100% \* var\(--lf-authored-zone-width\) \/ var\(--lf-authored-zone-layout-width\) - var\(--lf-authored-key-gutter\)\);/);
    assert.match(key, /height: calc\(100% \* var\(--lf-authored-zone-height\) \/ var\(--lf-authored-zone-layout-height\) - var\(--lf-authored-key-gutter\)\);/);
    // Check the CSS contract at desktop and narrow content widths. Wide faces
    // must occupy N single faces plus N-1 gutters; empty columns stay empty.
    for (const available of [320, 600, 800, 896]) {
        const width = Math.max(available, 20 * pitch);
        const cell = width / 20;
        const face = cell - gutter;
        assert.ok(face >= 32);
        const right = gutter / 2 + face;
        close(cell + gutter / 2 - right, gutter);
        // The 20:7 canvas aspect ratio gives the same vertical cell pitch.
        close((width * 7 / 20) / 7 + gutter / 2 - right, gutter);
        for (const span of [2, 3, 6]) {
            close(span * cell - gutter, span * face + (span - 1) * gutter);
            assert.ok(span * cell - gutter > face);
        }
        close(2 * cell + gutter / 2 - right, cell + gutter);
    }
    assert.match(key, /flex-direction: column;/);
    assert.match(key, /overflow: hidden;/);
    assert.match(rule(".lf-app-shell .lf-authored-zone-list-keyboard .lf-authored-zone span:first-child"), /text-overflow: ellipsis;/);
    assert.doesNotMatch(canvas + key, /packet|channel|k65/i);
});

test("existing geometry CSS preserves an upward two-row key without changing single-row keys", () => {
    // Source enter-custom is a tall rectangle, not an L-shaped polygon. The
    // adapter moves its logical top to row 4 and extends through row 5.
    const sourceCSS = fs.readFileSync(path.join(__dirname, "../css/themes/default.css"), "utf8");
    assert.match(sourceCSS, /\.enter-custom \{\s*margin-top: calc\(-1 \* var\(--keyboard-top\)\);\s*height: var\(--keyboard-height-custom\);/);
    assert.match(key, /height: calc\(100% \* var\(--lf-authored-zone-height\) \/ var\(--lf-authored-zone-layout-height\) - var\(--lf-authored-key-gutter\)\);/);
    assert.doesNotMatch(key, /clip-path|height: 100px|k65/i);
    for (const available of [320, 600, 800, 896]) {
        const cell = Math.max(available, 20 * pitch) / 20;
        const de = {left: 14 * cell + gutter / 2, top: 3 * cell + gutter / 2, width: 2 * cell - gutter, height: 2 * cell - gutter};
        const us = {left: 13 * cell + gutter / 2, top: 4 * cell + gutter / 2, width: 3 * cell - gutter, height: cell - gutter};
        assert.ok(de.height > us.height);
        close(de.top + de.height, us.top + us.height);
        close(de.left - (13 * cell + gutter / 2 + cell - gutter), gutter); // DE # / +
        close(17 * cell + gutter / 2 - (de.left + de.width), cell + gutter); // empty column 16
        assert.ok(us.height >= 32);
        close(de.height, 2 * us.height + gutter);
    }
});
