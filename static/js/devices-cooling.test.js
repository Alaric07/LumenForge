"use strict";

const assert = require("assert");
const cooling = require("./devices-cooling.js");

function control(value) {
    return {value: value, textContent: value, hidden: false, disabled: false, listeners: {}, addEventListener: function (name, listener) { this.listeners[name] = listener; }, focus: function () { this.focused = true; }};
}

async function run() {
    const label = control("Front");
	label.hidden = true;
	const labelDisplay = control("Front");
    const profile = control("quiet");
    const status = {textContent: ""};
    const row = {
        dataset: {lfChannelId: "1", lfConfirmedLabel: "Front", lfConfirmedProfile: "quiet"},
        querySelector: function (selector) { return selector === "[data-lf-cooling-label]" ? label : selector === "[data-lf-cooling-label-display]" ? labelDisplay : selector === "[data-lf-cooling-profile]" ? profile : status; }
    };
	const probeLabel = control("Coolant"); probeLabel.hidden = true;
	const probeDisplay = control("Coolant");
	const probeStatus = {textContent: ""};
	const probeRow = {
		dataset: {lfChannelId: "2", lfLabelName: "Probe 1", lfConfirmedLabel: "Coolant"},
		querySelector: function (selector) { return selector === "[data-lf-cooling-label]" ? probeLabel : selector === "[data-lf-cooling-label-display]" ? probeDisplay : probeStatus; }
	};
	const rpm = control("900 RPM"); rpm.dataset = {lfChannelId: "1"};
	const temperature = control("30.0°C"); temperature.dataset = {lfChannelId: "2"};
    const workspace = {dataset: {lfDeviceId: "ccxt-1"}, querySelectorAll: function (selector) { return selector === "[data-lf-cooling-channel]" ? [row] : selector === "[data-lf-cooling-probe]" ? [probeRow] : selector === "[data-lf-cooling-rpm]" ? [rpm] : [temperature]; }};
    const requests = [];
    const browser = {
        document: {querySelector: function () { return workspace; }},
        fetch: async function (url, options) { requests.push({url: url, body: options && JSON.parse(options.body)}); return {ok: true, json: async function () { return url.indexOf("/api/devices/") === 0 ? {device: {devices: {"1": {channelId: 1, rpm: 1200}, "2": {channelId: 2, temperatureString: "33.5°C"}}}} : {status: browser.failLabel && url === "/api/label" ? 0 : 1}; }}; },
		setInterval: function (handler, delay) { browser.timer = {handler: handler, delay: delay}; return browser.timer; },
		clearInterval: function (timer) { timer.cleared = true; },
        LumenForgeDevicesToast: function (message) { browser.toast = message; }
    };

    cooling.init(browser);
    profile.value = "balanced";
    await profile.listeners.change();
	assert.strictEqual(label.hidden, true);
	labelDisplay.listeners.click();
	assert.strictEqual(label.hidden, false);
	label.value = "Intake";
	await label.listeners.keydown({key: "Enter", preventDefault: function () {}});
    assert.strictEqual(labelDisplay.textContent, "Intake");
    assert.strictEqual(label.hidden, true);
	labelDisplay.listeners.click();
	label.value = "Cancelled";
	await label.listeners.keydown({key: "Escape", preventDefault: function () {}});
	assert.strictEqual(labelDisplay.textContent, "Intake");
	browser.failLabel = true;
	labelDisplay.listeners.click();
	label.value = "Rejected";
	await label.listeners.keydown({key: "Enter", preventDefault: function () {}});
	assert.strictEqual(labelDisplay.textContent, "Intake");
	assert.strictEqual(status.textContent, "Couldn’t save this cooling setting.");
	browser.failLabel = false;
	probeDisplay.listeners.click();
	assert.strictEqual(probeLabel.hidden, false);
	probeLabel.value = "Loop probe";
	await probeLabel.listeners.keydown({key: "Enter", preventDefault: function () {}});
	assert.strictEqual(probeDisplay.textContent, "Loop probe");
	probeDisplay.listeners.click();
	probeLabel.value = "Cancelled probe";
	await probeLabel.listeners.keydown({key: "Escape", preventDefault: function () {}});
	assert.strictEqual(probeDisplay.textContent, "Loop probe");
	browser.failLabel = true;
	probeDisplay.listeners.click();
	probeLabel.value = "Rejected probe";
	await probeLabel.listeners.keydown({key: "Enter", preventDefault: function () {}});
	assert.strictEqual(probeDisplay.textContent, "Loop probe");
	assert.strictEqual(probeStatus.textContent, "Couldn’t save this cooling setting.");
	browser.failLabel = false;
	await browser.timer.handler();

    assert.deepStrictEqual(requests, [
        {url: "/api/speed", body: {deviceId: "ccxt-1", channelId: 1, profile: "balanced"}},
        {url: "/api/label", body: {deviceId: "ccxt-1", channelId: 1, deviceType: 0, label: "Intake"}},
		{url: "/api/label", body: {deviceId: "ccxt-1", channelId: 1, deviceType: 0, label: "Rejected"}},
		{url: "/api/label", body: {deviceId: "ccxt-1", channelId: 2, deviceType: 0, label: "Loop probe"}},
		{url: "/api/label", body: {deviceId: "ccxt-1", channelId: 2, deviceType: 0, label: "Rejected probe"}},
		{url: "/api/devices/ccxt-1", body: undefined}
    ]);
    assert.strictEqual(browser.toast, "✓ Saved");
	assert.strictEqual(browser.timer.delay, 1500);
	assert.strictEqual(rpm.textContent, "1200 RPM");
	assert.strictEqual(temperature.textContent, "33.5°C");
}

run().catch(function (error) { console.error(error); process.exitCode = 1; });
