"use strict";

(function (root, factory) {
    const api = factory();
    if (typeof module === "object" && module.exports) { module.exports = api; }
    if (root && root.document) {
        const init = function () { api.init(root); };
        if (root.document.readyState === "loading") { root.document.addEventListener("DOMContentLoaded", init); } else { init(); }
    }
})(typeof window === "undefined" ? null : window, function () {
    function request(browser, workspace, url, method, name) {
        return browser.fetch(url, {method: method, body: JSON.stringify({deviceId: workspace.dataset.lfDeviceId, userProfileName: name})}).then(async function (response) {
            const result = response.ok ? await response.json() : null;
            if (!result || result.status !== 1) { throw new Error("device profile rejected"); }
            if (browser.LumenForgeDevicesToast) { browser.LumenForgeDevicesToast("✓ Saved", "success", 1500); }
            if (browser.location) { browser.location.reload(); }
        });
    }

    function init(browser) {
        const workspace = browser.document.querySelector("[data-lf-device-profiles-workspace]");
        if (!workspace) { return; }
        const profile = workspace.querySelector("[data-lf-device-profile]");
        const deleteSelect = workspace.querySelector("[data-lf-device-profile-delete-select]");
        const deleteProfile = workspace.querySelector("[data-lf-device-profile-delete]");
        const status = workspace.querySelector("[data-lf-device-profile-status]");
        const dialog = workspace.querySelector("[data-lf-device-profile-dialog]");
        const saveAs = workspace.querySelector("[data-lf-device-profile-new]");
		const profileLabel = workspace.dataset.lfDeviceProfileLabel || "Device Profile";
        const confirmedProfile = profile ? profile.value : "";
        if (profile) {
            profile.addEventListener("change", function () {
                if (profile.value === confirmedProfile) { return Promise.resolve(); }
                return request(browser, workspace, "/api/userProfile/change", "POST", profile.value).catch(function () {
                    profile.value = confirmedProfile;
					if (status) { status.textContent = "Couldn’t change " + profileLabel + "."; }
                });
            });
        }
        if (deleteProfile && deleteSelect) {
            deleteProfile.addEventListener("click", function () {
                return request(browser, workspace, "/api/userProfile/delete", "DELETE", deleteSelect.value).catch(function () {
					if (status) { status.textContent = "Couldn’t delete " + profileLabel + "."; }
                });
            });
        }
        if (saveAs && dialog) {
            const name = dialog.querySelector("[data-lf-device-profile-name]");
            const dialogStatus = dialog.querySelector("[data-lf-device-profile-dialog-status]");
            saveAs.addEventListener("click", function () { dialog.hidden = false; });
            dialog.querySelector("[data-lf-device-profile-create]").addEventListener("click", function () {
				if (!name.value.trim()) { dialogStatus.textContent = "Enter a " + profileLabel + " name."; return Promise.resolve(); }
				return request(browser, workspace, "/api/userProfile", "PUT", name.value.trim()).then(function () { dialog.hidden = true; }).catch(function () { dialogStatus.textContent = "Couldn’t save " + profileLabel + "."; });
            });
            dialog.querySelector("[data-lf-device-profile-cancel]").addEventListener("click", function () { dialog.hidden = true; });
        }
    }

    return {init: init};
});
