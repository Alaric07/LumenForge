"use strict";
$(document).ready(function () {
    $('.saveRgbControl').on('click', function () {
        const rgbControl = $("#rgbControl").is(':checked');
        const rgbOff = $("#rgbOff").val();
        const rgbOn = $("#rgbOn").val();
        const lcdControl = $("#lcdControl").is(':checked');

        const pf = {};
        pf["rgbControl"] = rgbControl;
        pf["rgbOff"] = rgbOff;
        pf["rgbOn"] = rgbOn;
        pf["lcdControl"] = lcdControl;

        $.ajax({
            url: '/api/scheduler/rgb',
            type: 'POST',
            data: JSON.stringify(pf, null, 2),
            cache: false,
            success: function(response) {
                try {
                    if (response.status === 1) {
                        toast.success(response.message);
                    } else {
                        toast.warning(response.message);
                    }
                } catch (err) {
                    toast.warning(response.message);
                }
            }
        });
    });

    $('.enableVirtualAudio').on('click', function () {
        const v_virtualAudio = $("#virtualAudio").is(':checked');

        const pf = {};
        pf["enabled"] = v_virtualAudio;
        const json = JSON.stringify(pf, null, 2);
        $.ajax({
            url: '/api/audio/update',
            type: 'POST',
            data: json,
            cache: false,
            success: function(response) {
                try {
                    if (response.status === 1) {
                        location.reload();
                    } else {
                        toast.warning(response.message);
                    }
                } catch (err) {
                    toast.warning(response.message);
                }
            }
        });
    });

    $('.updateDisplay').on('click', function () {
        const index = $(this).data('info');

        const width = parseInt($('.displayWidth_' + index).val());
        const height = parseInt($('.displayHeight_' + index).val());
        const position = parseInt($('.displayPosition_' + index).val());

        if (Number.isNaN(width) || width <= 0) {
            toastr.warning('Invalid display width');
            return;
        }

        if (Number.isNaN(height) || height <= 0) {
            toastr.warning('Invalid display height');
            return;
        }

        const left = position === 1;
        const top = position === 2;

        const pf = {};
        pf["displayIndex"] = index;
        pf["displayWidth"] = width;
        pf["displayHeight"] = height;
        pf["displayLeft"] = left;
        pf["displayTop"] = top;
        const json = JSON.stringify(pf, null, 2);

        $.ajax({
            url: '/api/display/update',
            type: 'POST',
            dataType: 'json',
            data: json,
            cache: false,
            success: function (response) {
                if (response.status === 1) {
                    toast.success(response.message);
                } else {
                    toastr.error(response.message || 'Unable to update display');
                }
            },
            error: function () {
                toastr.error('Unable to update display');
            }
        });
    });

    $('.setTargetDevice').on('click', function () {
        const outputDevice = $("#outputDevice").val();
        const data = outputDevice.split(";");

        if (data.length < 2) {
            toast.warning('Invalid target device');
            return false;
        }

        const deviceSerial = parseInt(data[2]);
        const deviceDesc = data[1];
        const deviceName = data[0];
        
        const pf = {};
        pf["outputDeviceSerial"] = deviceSerial;
        pf["outputDeviceName"] = deviceName;
        pf["outputDeviceDesc"] = deviceDesc;

        const json = JSON.stringify(pf, null, 2);
        $.ajax({
            url: '/api/audio/outputDevice',
            type: 'POST',
            data: json,
            cache: false,
            success: function(response) {
                try {
                    if (response.status === 1) {
                        location.reload();
                    } else {
                        toast.warning(response.message);
                    }
                } catch (err) {
                    toast.warning(response.message);
                }
            }
        });
    });

    const checkboxCelsius = $('#checkbox-celsius');
    let currentDashboardRgbOff = false;
    let currentDashboardShowLabels = false;
    let currentDashboardTemperatureBar = false;
    let dashboardPreferencesLoaded = false;

    function buildDashboardPreferencesPayload() {
        return {
            showLabels: currentDashboardShowLabels,
            celsius: checkboxCelsius.is(':checked'),
            temperatureBar: currentDashboardTemperatureBar,
            languageCode: $("#userLanguage").val(),
            theme: $("#theme").val(),
            keyboardLayout: parseInt($("#keyboardLayout").val()),
            rgbOff: currentDashboardRgbOff
        };
    }

    function saveDashboardPreferences() {
        if (!dashboardPreferencesLoaded) {
            toast.warning("Dashboard preferences are still loading.");
            return;
        }
        $.ajax({
            url: '/api/dashboard/update',
            type: 'POST',
            data: JSON.stringify(buildDashboardPreferencesPayload(), null, 2),
            cache: false,
            success: function(response) {
                try {
                    if (response.status === 1) {
                        location.reload();
                    } else {
                        toast.warning(response.message);
                    }
                } catch (err) {
                    toast.warning(response.message);
                }
            }
        });
    }

    function loadDashboardSettings() {
        // Load current settings
        $.ajax({
            url: '/api/dashboard',
            type: 'GET',
            cache: false,
            success: function(response) {
                if (response.status === 1) {
                    currentDashboardRgbOff = response.dashboard.rgbOff === true;
                    currentDashboardShowLabels = response.dashboard.showLabels === true;
                    if (response.dashboard.celsius === true) {
                        checkboxCelsius.attr('Checked','Checked');
                    }
                    currentDashboardTemperatureBar = response.dashboard.temperatureBar === true;
                    dashboardPreferencesLoaded = true;
                }
            }
        });

        $('#btnSaveDashboardSettings').on('click', function () {
            saveDashboardPreferences();
        });
    }
    loadDashboardSettings();
});
