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

    function drawerOpenClass(drawer) {
        return drawer.getAttribute("data-lf-drawer-open-class") || "lf-device-panel-open";
    }

    function setDrawerOpen(drawer, toggle, open) {
        drawer.classList.toggle(drawerOpenClass(drawer), open);
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

        const drawers = Array.from(shell.querySelectorAll("[data-lf-drawer]"));
        for (const drawer of drawers) {
            const drawerToggle = shell.querySelector('[data-lf-drawer-toggle][aria-controls="' + drawer.id + '"]');
            if (!drawerToggle) continue;
            const openClass = drawerOpenClass(drawer);
            drawerToggle.addEventListener("click", function () {
                const open = !drawer.classList.contains(openClass);
                if (open) {
                    for (const otherDrawer of drawers) {
                        if (otherDrawer === drawer) continue;
                        const otherToggle = shell.querySelector('[data-lf-drawer-toggle][aria-controls="' + otherDrawer.id + '"]');
                        if (otherToggle && otherDrawer.classList.contains(drawerOpenClass(otherDrawer))) {
                            setDrawerOpen(otherDrawer, otherToggle, false);
                        }
                    }
                }
                setDrawerOpen(drawer, drawerToggle, open);
            });
            drawer.addEventListener("click", function (event) {
                if (event.target.closest("[data-lf-drawer-item]")) {
                    setDrawerOpen(drawer, drawerToggle, false);
                }
            });
        }
        document.addEventListener("keydown", function (event) {
            if (event.key !== "Escape") return;
            for (const drawer of drawers) {
                const drawerToggle = shell.querySelector('[data-lf-drawer-toggle][aria-controls="' + drawer.id + '"]');
                if (drawerToggle && drawer.classList.contains(drawerOpenClass(drawer))) {
                    setDrawerOpen(drawer, drawerToggle, false);
                }
            }
        });
    }

    root.LumenForgeAppShell = { initialize: initialize, setNavigationCollapsed: setNavigationCollapsed, setDrawerOpen: setDrawerOpen };
    if (root.document) initialize(root.document);
}(window));
