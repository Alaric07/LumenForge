(function (root) {
    "use strict";

    const storageKey = "lumenforge-sidebarCollapsed";

    function setNavigationCollapsed(shell, collapsed) {
        shell.classList.toggle("lf-global-nav-collapsed", collapsed);
        const toggle = shell.querySelector("[data-lf-global-nav-toggle]");
        if (toggle) {
            const label = collapsed ? "Expand navigation" : "Collapse navigation";
            toggle.setAttribute("aria-label", label);
            toggle.setAttribute("title", label);
        }
    }

    function persistNavigation(collapsed) {
        try {
            root.localStorage.setItem(storageKey, String(collapsed));
        } catch (_) {
            // The server-rendered preference remains authoritative when storage is unavailable.
        }
        if (typeof root.fetch === "function") {
            root.fetch("/api/dashboard/sidebar", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ sidebarCollapsed: collapsed })
            }).catch(function () {});
        }
    }

    function setDrawerOpen(drawer, toggle, open) {
        drawer.classList.toggle("lf-device-panel-open", open);
        toggle.setAttribute("aria-expanded", String(open));
    }

    function initialize(document) {
        const shell = document.querySelector(".lf-app-shell");
        if (!shell) return;

        const navigationToggle = shell.querySelector("[data-lf-global-nav-toggle]");
        if (navigationToggle) {
            try {
                const stored = root.localStorage.getItem(storageKey);
                if (stored !== null) setNavigationCollapsed(shell, stored === "true");
            } catch (_) {
                // Server-rendered state is used when storage is unavailable.
            }
            navigationToggle.addEventListener("click", function () {
                const collapsed = !shell.classList.contains("lf-global-nav-collapsed");
                setNavigationCollapsed(shell, collapsed);
                persistNavigation(collapsed);
            });
        }

        const drawer = shell.querySelector("[data-lf-device-drawer]");
        const drawerToggle = shell.querySelector("[data-lf-devices-drawer-toggle]");
        if (!drawer || !drawerToggle) return;

        drawerToggle.addEventListener("click", function () {
            setDrawerOpen(drawer, drawerToggle, !drawer.classList.contains("lf-device-panel-open"));
        });
        drawer.addEventListener("click", function (event) {
            if (event.target.closest(".lf-device-item")) {
                setDrawerOpen(drawer, drawerToggle, false);
            }
        });
        document.addEventListener("keydown", function (event) {
            if (event.key === "Escape" && drawer.classList.contains("lf-device-panel-open")) {
                setDrawerOpen(drawer, drawerToggle, false);
            }
        });
    }

    root.LumenForgeAppShell = { initialize: initialize, setNavigationCollapsed: setNavigationCollapsed, setDrawerOpen: setDrawerOpen };
    if (root.document) initialize(root.document);
}(window));
