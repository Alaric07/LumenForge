$(document).ready(function () {
    function updateBatteryUI(serial, level) {
        const $battery = $("#batteryLevel_" + serial);
        if (!$battery.length) return;

        $battery.css("display", "inline-flex");
        $battery.attr("title", "Battery " + level + "%");

        const maxWidth = 16;
        const width = Math.max(1, (level / 100) * maxWidth);
        
        $battery.find(".battery-level").attr("width", width);
        $battery.removeClass("battery-full battery-warn battery-low");

        if (level < 15) {
            $battery.addClass("battery-low");
        } else if (level < 30) {
            $battery.addClass("battery-warn");
        } else {
            $battery.addClass("battery-full");
        }
    }

    function refreshBatterStatus() {
        $.ajax({
            url: "/api/batteryStats",
            type: "GET",
            dataType: "json",
            success: function (result) {
                $.each(result.data, function (serial, value) {
                    updateBatteryUI(serial, value.Level);
                });
            }
        });
    }
    function autoRefresh() {
        setInterval(function () {
            refreshBatterStatus();
        }, 3000);
    }

    // Get initial value
    refreshBatterStatus();

    // Set auto refresh
    autoRefresh();

    // Sidebar toggle collapse
    const sidebar = document.querySelector(".sidebar");
    const key = "lumenforge-sidebarCollapsed";
    if (localStorage.getItem(key) === "true") {
        sidebar.classList.add("collapsed");
    }

    $('#sidebarToggle').on('click', function () {
        sidebar.classList.toggle("collapsed");
        localStorage.setItem(key, sidebar.classList.contains("collapsed"));

        const pf = {
            sidebarCollapsed: sidebar.classList.contains("collapsed")
        };

        $.ajax({
            url: '/api/dashboard/sidebar',
            type: 'POST',
            contentType: 'application/json',
            data: JSON.stringify(pf),
            cache: false,
            success: function (response) {
                try {
                    if (response.status === 1) {
                        // No action
                    } else {
                        toast.warning(response.message);
                    }
                } catch (err) {
                    toast.warning(response.message);
                }
            }
        });
    });

    function setSidebarSectionState($content, $toggle, expanded, animate) {
        $content.stop(true, true);
        $toggle.toggleClass('sidebar-section-expanded', expanded);
        $toggle.toggleClass('sidebar-section-collapsed', !expanded);

        if (!animate) {
            $content.toggleClass('show', expanded).toggle(expanded);
            return;
        }
        if (expanded) {
            $content.addClass('show').hide().slideDown(200);
        } else {
            $content.slideUp(200, function () {
                $content.removeClass('show');
            });
        }
    }

    // Resolve active-page and persisted states once before revealing sections.
    $('.sidebar-section-content').each(function () {
        const id = $(this).attr('id');
        const section = id.replace('section-', '');
        const isActive = $(this).attr('data-active') === 'true';
        const $toggle = $(`.sidebar-section-toggle[data-section="${section}"]`);
        const storedState = localStorage.getItem('lumenforge-sidebar-expanded-' + section);
        const expanded = isActive || storedState === 'true';

        setSidebarSectionState($(this), $toggle, expanded, false);
    });
    $('.sidebar').removeClass('sidebar-sections-initializing');

    // Sidebar section collapse handling
    $('.sidebar-section-toggle').on('click', function (event) {
        event.preventDefault();
        event.stopPropagation();

        const section = $(this).attr('data-section');
        const $content = $('#section-' + section);
        const expanded = !$(this).hasClass('sidebar-section-expanded');

        setSidebarSectionState($content, $(this), expanded, true);
        localStorage.setItem('lumenforge-sidebar-expanded-' + section, String(expanded));
    });

    $('.sidebar-section-content .nav-link').on('click', function (event) {
        event.stopPropagation();
    });
});
