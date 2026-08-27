"use strict";

(function (root, factory) {
    const api = factory();
    if (typeof module === "object" && module.exports) { module.exports = api; }
    if (root && root.document) {
        const init = function () { api.init(root); };
        if (root.document.readyState === "loading") { root.document.addEventListener("DOMContentLoaded", init); } else { init(); }
    }
})(typeof window === "undefined" ? null : window, function () {
    const endpoint = "/api/mouse/updateKeyAssignment";
    const optionSources = {0: null, 1: "/api/input/media", 2: null, 3: "/api/input/keyboard", 8: null, 9: "/api/input/mouse", 10: "/api/macro/", 11: null};

    function localOptions(type) {
        return (type === 0 || type === 2 || type === 8 || type === 11) ? [{value: 0, label: "None"}] : [];
    }
    function optionList(value) {
        if (Array.isArray(value)) { return value; }
        if (value && Array.isArray(value.data)) { return value.data; }
		if (value && value.data && typeof value.data === "object") { return Object.keys(value.data).map(function (key) { return Object.assign({id: key}, value.data[key]); }); }
        if (value && Array.isArray(value.actions)) { return value.actions; }
        if (value && typeof value === "object") { return Object.keys(value).map(function (key) { return value[key] && typeof value[key] === "object" ? Object.assign({id: key}, value[key]) : {id: key, name: value[key]}; }); }
        return [];
    }
    function normalizeOptions(value) {
        return optionList(value).map(function (item) {
            const rawValue = item.value !== undefined ? item.value : (item.id !== undefined ? item.id : (item.key !== undefined ? item.key : item.action));
            const label = item.label || item.name || item.Name || item.valueString || String(rawValue);
            return rawValue === undefined ? null : {value: Number(rawValue), label: label};
        }).filter(Boolean);
    }
    function createOptionCache() {
        const requests = new Map();
        return {
            get: function (browser, type) {
                if (optionSources[type] === null) { return Promise.resolve(localOptions(type)); }
                if (!optionSources[type] || typeof browser.fetch !== "function") { return Promise.resolve([]); }
                if (!requests.has(type)) {
                    const request = browser.fetch(optionSources[type]).then(async function (response) {
                        if (!response.ok) { throw new Error("options unavailable"); }
                        return normalizeOptions(await response.json());
                    });
                    requests.set(type, request);
                    request.catch(function () { requests.delete(type); });
                }
                return requests.get(type);
            }
        };
    }
    async function optionsFor(browser, type, cache) {
        if (optionSources[type] === null) { return localOptions(type); }
        if (!optionSources[type] || typeof browser.fetch !== "function") { return []; }
        if (cache) { return cache.get(browser, type); }
        const response = await browser.fetch(optionSources[type]);
        if (!response.ok) { throw new Error("options unavailable"); }
        return normalizeOptions(await response.json());
    }
    function setOptions(select, options, current) {
        select.replaceChildren();
        options.forEach(function (option) {
            const element = select.ownerDocument.createElement("option");
            element.value = String(option.value);
            element.textContent = option.label;
            element.selected = Number(option.value) === Number(current);
            select.appendChild(element);
        });
    }
    async function populate(browser, row, cache, expectedType) {
        const type = expectedType === undefined ? Number(row.querySelector("[data-lf-button-type]").value) : expectedType;
        const command = row.querySelector("[data-lf-button-command]");
        const status = row.querySelector("[data-lf-button-status]");
        command.disabled = true;
        try {
            const options = await optionsFor(browser, type, cache);
            if (Number(row.querySelector("[data-lf-button-type]").value) !== type) { return false; }
            setOptions(command, options, row.dataset.lfCurrentCommand);
            status.textContent = "";
			command.disabled = false;
			return true;
        } catch (_) {
			if (Number(row.querySelector("[data-lf-button-type]").value) === type) {
				command.replaceChildren();
				status.textContent = "Unable to load assignment keys.";
			}
			return false;
        }
    }
    function createToast(browser, toast) {
        let hideTimer = null;
        let generation = 0;
        return function showToast(message, kind, duration) {
            if (!toast) { return; }
            if (hideTimer !== null) { (browser.clearTimeout || clearTimeout)(hideTimer); }
            const currentGeneration = ++generation;
            toast.textContent = message;
            toast.dataset.lfButtonsToastKind = kind;
            toast.hidden = false;
            hideTimer = (browser.setTimeout || setTimeout)(function () {
                if (currentGeneration !== generation) { return; }
                toast.hidden = true;
                toast.textContent = "";
                delete toast.dataset.lfButtonsToastKind;
                hideTimer = null;
            }, duration);
        };
    }
    function bindRow(browser, workspace, row, cache, showToast) {
        const type = row.querySelector("[data-lf-button-type]");
        const command = row.querySelector("[data-lf-button-command]");
        const hold = row.querySelector("[data-lf-button-hold]");
        const release = row.querySelector("[data-lf-button-release]");
        const control = row.querySelector("[data-lf-button-control]");
        const status = row.querySelector("[data-lf-button-status]");
        if (!type || !command || !control || !hold || !release || !status) { return; }
        const notify = showToast || function () {};
        let saving = false;
        let queued = false;
        let savePromise = Promise.resolve();
		let assignmentOptionsReady = false;
		async function populateAssignmentOptions(expectedType) {
			const populatedType = expectedType === undefined ? Number(type.value) : expectedType;
			assignmentOptionsReady = false;
			const populated = await populate(browser, row, cache, populatedType);
			if (Number(type.value) === populatedType) { assignmentOptionsReady = populated; }
			return populated;
		}
        function requestSave() {
			if (!assignmentOptionsReady) { return Promise.resolve(); }
			queued = true;
			if (saving) { return savePromise; }
			saving = true;
			savePromise = (async function () {
				while (queued) {
					queued = false;
					if (!assignmentOptionsReady) { continue; }
					const lumenForgeControlEnabled = control.checked;
					const savedCommand = Number(command.value);
					try {
						// Legacy backend `enabled` carries KeyAssignment.Default:
						// true = device behavior, false = LumenForge assignment.
						const response = await browser.fetch(endpoint, {method: "POST", body: JSON.stringify({deviceId: workspace.dataset.lfDeviceId, keyIndex: Number(row.dataset.lfKeyIndex), enabled: !lumenForgeControlEnabled, pressAndHold: hold.checked, keyAssignmentType: Number(type.value), keyAssignmentValue: savedCommand, onRelease: release.checked})});
						const result = response.ok ? await response.json() : null;
						if (!result || result.status !== 1) { throw new Error("save rejected"); }
						row.dataset.lfCurrentCommand = String(savedCommand);
						if (!queued) { notify("✓ Saved", "success", 1500); }
					} catch (_) {
						if (!queued) { notify("Couldn’t save button assignment.", "error", 4500); }
					}
				}
				saving = false;
			})();
			return savePromise;
		}
        type.addEventListener("change", async function () {
			row.dataset.lfCurrentCommand = "0";
			if (await populateAssignmentOptions(Number(type.value))) { return requestSave(); }
		});
        control.addEventListener("change", requestSave);
        hold.addEventListener("change", function () { if (hold.checked) { release.checked = false; } return requestSave(); });
        release.addEventListener("change", function () { if (release.checked) { hold.checked = false; } return requestSave(); });
        command.addEventListener("change", requestSave);
		const initialPopulation = populateAssignmentOptions();
        return initialPopulation;
    }
    function init(browser) {
        const workspace = browser.document.querySelector("[data-lf-buttons-workspace]");
        if (!workspace) { return; }
        const cache = createOptionCache();
        const showToast = createToast(browser, workspace.querySelector("[data-lf-buttons-toast]"));
        workspace.querySelectorAll("[data-lf-button-row]").forEach(function (row) { bindRow(browser, workspace, row, cache, showToast); });
    }
    return {bindRow: bindRow, createOptionCache: createOptionCache, createToast: createToast, init: init, localOptions: localOptions, normalizeOptions: normalizeOptions, optionsFor: optionsFor, populate: populate, setOptions: setOptions};
});
