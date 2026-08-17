"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const dashboardOpenRGBCard = require("./index.js");

function renderOpenRGBCard(device, label) {
    let parsedMarkup = "";
    const textBySelector = new Map();
    const column = {
        first: function () {
            return this;
        },
        find: function (selector) {
            return {
                text: function (value) {
                    textBySelector.set(selector, String(value));
                    return this;
                }
            };
        }
    };
    function jquery(value) {
        assert.equal(typeof value, "string");
        parsedMarkup = value;
        return column;
    }

    const view = dashboardOpenRGBCard.createView(device, label);
    assert.equal(dashboardOpenRGBCard.createColumn(jquery, view), column);
    return {parsedMarkup, textBySelector, view};
}

test("hostile OpenRGB dashboard text never enters parsed markup", function () {
    delete globalThis.__lumenforgeXSS;
    const product = "</span><img src=x onerror=window.__lumenforgeXSS=1>";
    const label = "</span><script>window.__lumenforgeXSS=2</script>";
    const effect = "rainbow</span><svg onload=window.__lumenforgeXSS=3>";
    const rendered = renderOpenRGBCard({
        Product: product,
        DeviceProfile: {RGBProfile: effect}
    }, label);

    for (const value of [product, label, effect]) {
        assert.equal(rendered.parsedMarkup.includes(value), false, value);
    }
    assert.doesNotMatch(rendered.parsedMarkup, /<(?:img|script|svg)\b/i);
    assert.doesNotMatch(rendered.parsedMarkup, /\son[a-z]+\s*=/i);
    assert.equal(rendered.textBySelector.get("[data-dashboard-openrgb-product]"), product);
    assert.equal(rendered.textBySelector.get("[data-dashboard-openrgb-label]"), label);
    assert.equal(rendered.textBySelector.get("[data-dashboard-openrgb-effect]"), effect);
    assert.equal(globalThis.__lumenforgeXSS, undefined);
});

test("OpenRGB dashboard preserves benign metacharacters as literal text", function () {
    const product = 'Corsair <RGB> "Device"';
    const label = 'Desk <Left> "Primary"';
    const effect = 'custom <Effect> "One"';
    const rendered = renderOpenRGBCard({
        Product: product,
        DeviceProfile: {RGBProfile: effect}
    }, label);

    assert.equal(rendered.textBySelector.get("[data-dashboard-openrgb-product]"), product);
    assert.equal(rendered.textBySelector.get("[data-dashboard-openrgb-label]"), label);
    assert.equal(rendered.textBySelector.get("[data-dashboard-openrgb-effect]"), effect);
});

test("normal OpenRGB dashboard card preserves its structure and values", function () {
    const rendered = renderOpenRGBCard({
        Product: "OpenRGB Controller",
        DeviceProfile: {RGBProfile: "rainbow"}
    }, "Desk");

    assert.equal(rendered.view.isOpenRGB, true);
    assert.match(rendered.parsedMarkup, /class="col-md-2"/);
    assert.match(rendered.parsedMarkup, /class="card system-card"/);
    assert.match(rendered.parsedMarkup, /class="card-header header-split"/);
    assert.match(rendered.parsedMarkup, />Status</);
    assert.match(rendered.parsedMarkup, />Connected</);
    assert.match(rendered.parsedMarkup, />Current Effect</);
    assert.equal((rendered.parsedMarkup.match(/class="settings-row"/g) || []).length, 2);
    assert.equal(rendered.textBySelector.get("[data-dashboard-openrgb-product]"), "OpenRGB Controller");
    assert.equal(rendered.textBySelector.get("[data-dashboard-openrgb-label]"), "Desk");
    assert.equal(rendered.textBySelector.get("[data-dashboard-openrgb-effect]"), "rainbow");
});

test("OpenRGB dashboard preserves clustered and fallback presentation", function () {
    const clustered = renderOpenRGBCard({
        Product: "",
        DeviceProfile: {RGBCluster: true, RGBProfile: "rainbow"}
    }, "");
    assert.equal(clustered.textBySelector.get("[data-dashboard-openrgb-product]"), "OpenRGB Device");
    assert.equal(clustered.textBySelector.get("[data-dashboard-openrgb-label]"), "");
    assert.equal(clustered.textBySelector.get("[data-dashboard-openrgb-effect]"), "Clustered");

    const fallback = renderOpenRGBCard({DeviceProfile: {}}, "");
    assert.equal(fallback.textBySelector.get("[data-dashboard-openrgb-effect]"), "None");
});
