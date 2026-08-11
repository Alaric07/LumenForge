"use strict";

(function (root, factory) {
    const api = factory();
    if (typeof module === "object" && module.exports) {
        module.exports = api;
    }
    if (root && root.document && root.jQuery) {
        root.jQuery(function () {
            api.bindMemberOrdering(root.jQuery);
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

    return {bindMemberOrdering};
});
