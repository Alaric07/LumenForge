"use strict";

(function (root, factory) {
    const api = factory();
    if (typeof module === "object" && module.exports) {
        module.exports = api;
    }
    if (root && root.document && root.jQuery) {
        root.jQuery(function () {
            api.bindMemberOrdering(root.jQuery);
            api.bindLightingStatus(root);
        });
    }
})(typeof window === "undefined" ? null : window, function () {
    function bindMemberOrdering($) {
        const sortable = $("#clusterSortable");
        if (!sortable.length || typeof sortable.sortable !== "function") {
            return false;
        }

        sortable.sortable({
            helper: function (_event, row) {
                const originals = row.children();
                const helper = row.clone();
                helper.children().each(function (index) {
                    $(this).width(originals.eq(index).width());
                });
                helper.css("background-color", "rgba(255, 255, 255, 0.05)");
                return helper;
            },
            axis: "y",
            update: function () {
                const deviceOrder = [];
                $(this).children("tr").each(function () {
                    deviceOrder.push($(this).data("serial").toString());
                });

                $.ajax({
                    url: "/api/cluster/order",
                    type: "PUT",
                    data: JSON.stringify({deviceOrder: deviceOrder}),
                    contentType: "application/json",
                    success: function (response) {
                        if (response.status === 1) {
                            toast.success(response.message);
                        } else {
                            toast.warning(response.message);
                        }
                    },
                    error: function () {
                        toast.error("Failed to update cluster order");
                    }
                });
            }
        }).disableSelection();
        return true;
    }

    function bindLightingStatus(browser) {
        const selector = browser && browser.document && browser.document.querySelector('[data-lf-effect-selector][data-lf-lighting-target="cluster"]');
        if (!selector || !selector.dataset || !selector.dataset.lfCurrentEffect || typeof browser.fetch !== "function" || typeof browser.setInterval !== "function") {
            return false;
        }

        const currentEffect = selector.dataset.lfCurrentEffect;
        let reloading = false;
        let polling = false;
        let timer = null;
        const poll = async function () {
            if (reloading || polling) { return; }
            polling = true;
            try {
                const response = await browser.fetch("/api/cluster/lighting/status");
                const status = response && response.ok ? await response.json() : null;
                if (!status || status.status !== 1 || !status.effect || status.effect === currentEffect || reloading) { return; }
                reloading = true;
                if (timer !== null && typeof browser.clearInterval === "function") { browser.clearInterval(timer); }
                browser.location.reload();
            } catch (_) {
                // The next interval retries unavailable or failed status reads.
            } finally {
                polling = false;
            }
        };
        timer = browser.setInterval(poll, 1000);
        return true;
    }

    return {bindMemberOrdering, bindLightingStatus};
});
