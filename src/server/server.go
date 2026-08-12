package server

// Package: server
// Author: Nikola Jurkovic
// License: GPL-3.0 or later

import (
	"LumenForge/src/audio"
	"LumenForge/src/backup"
	"LumenForge/src/cluster"
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/dashboard"
	"LumenForge/src/devices"
	"LumenForge/src/devices/lcd"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/display"
	"LumenForge/src/externalsources"
	"LumenForge/src/inputmanager"
	"LumenForge/src/language"
	"LumenForge/src/lifecycle"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/localnetwork"
	"LumenForge/src/logger"
	"LumenForge/src/macro"
	"LumenForge/src/media"
	"LumenForge/src/metrics"
	"LumenForge/src/openrgb"
	"LumenForge/src/rgb"
	"LumenForge/src/scheduler"
	"LumenForge/src/server/requests"
	"LumenForge/src/stats"
	"LumenForge/src/systeminfo"
	"LumenForge/src/systray"
	"LumenForge/src/temperatures"
	"LumenForge/src/templates"
	"LumenForge/src/version"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Response contains data what is sent back to a client
type Response struct {
	sync.Mutex
	Code      int         `json:"code"`
	Status    int         `json:"status"`
	Message   string      `json:"message,omitempty"`
	Device    interface{} `json:"device,omitempty"`
	Devices   interface{} `json:"devices,omitempty"`
	Dashboard interface{} `json:"dashboard,omitempty"`
	Data      interface{} `json:"data,omitempty"` // For dataTables
}

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

var headers []Header
var (
	server      *http.Server
	serveDone   chan struct{}
	serverMutex sync.Mutex

	discoverOpenRGBImports = openrgbimport.DiscoverPreview
	importOpenRGBImports   = func(ctx context.Context, keys []string) (openrgbimport.ImportResult, error) {
		return openrgbimport.ImportControllers(ctx, keys, openRGBImportRegistryHooks())
	}
	removeOpenRGBImports = func(ctx context.Context, serials []string) (openrgbimport.RemoveResult, error) {
		return openrgbimport.RemoveConfiguredImports(ctx, serials, openRGBImportRegistryHooks())
	}
	refreshOpenRGBImports           = openrgbimport.RefreshManager
	lookupOpenRGBImportForLighting  = devices.LookupOpenRGBImport
	setOpenRGBImportBrightnessValue = func(device *openrgbimport.Device, brightness uint8) error {
		return device.SetBrightness(brightness)
	}
	setOpenRGBImportEffectValue = func(device *openrgbimport.Device, effect string) error {
		return device.SetEffect(effect)
	}
	setOpenRGBImportSpeedValue = func(device *openrgbimport.Device, serial, effect string, speed float64) error {
		return device.SetEffectSpeed(serial, effect, speed)
	}
	setOpenRGBImportColorValue = func(device *openrgbimport.Device, serial, effect string, color lightingsettings.Color) error {
		return device.SetEffectColor(serial, effect, color)
	}
	setOpenRGBImportTwoColorValue = func(device *openrgbimport.Device, serial, effect string, start, end lightingsettings.Color) error {
		return device.SetEffectTwoColor(serial, effect, start, end)
	}
	setOpenRGBImportTemperatureValue = func(device *openrgbimport.Device, serial, effect string, low, middle, high lightingsettings.TemperaturePoint) error {
		return device.SetEffectTemperature(serial, effect, low, middle, high)
	}
	setOpenRGBImportGradientValue = func(device *openrgbimport.Device, serial, effect string, stops []lightingsettings.GradientStop) error {
		return device.SetEffectGradient(serial, effect, stops)
	}
	resetOpenRGBImportCustomizationValue = func(device *openrgbimport.Device, serial, effect string) error {
		return device.ResetEffectCustomization(serial, effect)
	}
	mediaInputControl           = inputmanager.InputControlKeyboard
	getRGBClusterLightingStatus = func() (cluster.LightingSnapshot, int) {
		device := cluster.Get()
		if device == nil {
			return cluster.LightingSnapshot{}, 0
		}
		return device.LightingSnapshot(), device.ControllerCount()
	}
)

const (
	openRGBImportRequestLimit      = 64 << 10
	openRGBImportBatchLimit        = 64
	openRGBImportGradientStopLimit = 1024
)

func openRGBImportRegistryHooks() openrgbimport.RegistryHooks {
	return openrgbimport.RegistryHooks{
		Register: devices.RegisterOpenRGBImport,
		Remove:   devices.RemoveOpenRGBImport,
		Lookup:   devices.LookupOpenRGBImport,
	}
}

// Send will process response and send it back to a client
func (r *Response) Send(w http.ResponseWriter) {
	r.Lock()
	defer r.Unlock()

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(r.Code)

	data, err := json.Marshal(r)
	if err != nil {
		_, err := fmt.Println(w, err.Error())
		if err != nil {
			return
		}
		return
	}

	_, err = w.Write(data)
	if err != nil {
		return
	}
}

func executeTemplateOrRespond(w http.ResponseWriter, t *template.Template, name string, data any, logError ...bool) bool {
	err := t.ExecuteTemplate(w, name, data)
	if err != nil {
		if len(logError) > 0 && logError[0] {
			fmt.Println(err)
		}
		resp := &Response{
			Code:    http.StatusInternalServerError,
			Message: language.GetValue("txtUnableToServeWebContent"),
		}
		resp.Send(w)
		return false
	}
	return true
}

// homePage returns response on /
func homePage(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Device: devices.GetDevices(),
	}
	resp.Send(w)
}

// getOpenRGBStatus returns OpenRGB connection state and last error if any
func getOpenRGBStatus(w http.ResponseWriter, _ *http.Request) {
	state, err := openrgb.GetStatus()
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	type openRGBStatus struct {
		State string `json:"state"`
		Error string `json:"error,omitempty"`
	}

	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data: openRGBStatus{
			State: string(state),
			Error: errMsg,
		},
	}
	resp.Send(w)
}

// getCpuTemperature will return current cpu temperature in string format
func getCpuTemperature(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   dashboard.GetDashboard().TemperatureToString(temperatures.GetCpuTemperature()),
	}
	resp.Send(w)
}

// getCpuTemperatureClean will return current cpu temperature in float value
func getCpuTemperatureClean(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   temperatures.GetCpuTemperature(),
	}
	resp.Send(w)
}

// getGpuTemperature will return current gpu temperature in string format
func getGpuTemperature(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature()),
	}
	resp.Send(w)
}

// getGpuTemperatures will return current gpu temperature in string format
func getGpuTemperatures(w http.ResponseWriter, _ *http.Request) {
	data := make(map[int]interface{})
	for key, val := range systeminfo.GetInfo().GPU {
		data[key] = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperatureIndex(val.Index))
	}
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   data,
	}
	resp.Send(w)
}

// getCpuLoad will return current cpu load
func getCpuLoad(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   systeminfo.GetCpuUtilization(),
	}
	resp.Send(w)
}

// getGpuLoad will return current gpu load
func getGpuLoad(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   systeminfo.GetGPUUtilization(),
	}
	resp.Send(w)
}

// getGpuTemperatureClean will return current gpu temperature in float value
func getGpuTemperatureClean(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   temperatures.GetGpuTemperature(),
	}
	resp.Send(w)
}

// getStorageTemperature will return current storage temperature
func getStorageTemperature(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   temperatures.GetStorageTemperatures(),
	}
	resp.Send(w)
}

// getBatteryStats will return battery stats
func getBatteryStats(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   stats.GetBatteryStats(),
	}
	resp.Send(w)
}

// getDeviceMetrics will return a list device metrics in prometheus format
func getDeviceMetrics(w http.ResponseWriter, r *http.Request) {
	devices.UpdateDeviceMetrics()
	metrics.Handler(w, r)
}

// getSupportedDevices will return list of supported devices
func getSupportedDevices(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code: http.StatusOK,
		Data: devices.GetSupportedDevices(),
	}
	resp.Send(w)
}

// setSupportedDevices handles enable / disable of supported devices
func setSupportedDevices(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSetSupportedDevices(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// getDevices returns response on /devices
func getDevices(w http.ResponseWriter, r *http.Request) {
	deviceId, valid := getVar("/api/devices/", r)
	if !valid {
		resp := &Response{
			Code:    http.StatusOK,
			Devices: devices.GetDevicesEx(),
		}
		resp.Send(w)
	} else {
		resp := &Response{
			Code:   http.StatusOK,
			Device: snapshotDeviceForResponse(devices.GetDevice(deviceId)),
		}
		resp.Send(w)
	}
}

func snapshotDeviceForResponse(device interface{}) interface{} {
	if openrgbDevice, ok := device.(*openrgbimport.Device); ok {
		snapshot := openrgbDevice.Snapshot()
		return &snapshot
	}
	return device
}

// getDeviceLed returns response on /led
func getDeviceLed(w http.ResponseWriter, r *http.Request) {
	deviceId, valid := getVar("/api/led/", r)
	if !valid {
		resp := &Response{
			Code:   http.StatusOK,
			Status: 1,
			Data:   devices.GetDevicesLedData(),
		}
		resp.Send(w)
	} else {
		results := devices.CallDeviceMethod(
			deviceId,
			"GetDeviceLedData",
		)

		if len(results) > 0 {
			resp := &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   results[0].Interface(),
			}
			resp.Send(w)
		} else {
			resp := &Response{
				Code:   http.StatusOK,
				Status: 0,
				Data:   nil,
			}
			resp.Send(w)
		}
	}
}

// updateDeviceLed handles device LED changes
func updateDeviceLed(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessLedChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// getMacro returns response on /macro
func getMacro(w http.ResponseWriter, r *http.Request) {
	macroId, valid := getVar("/api/macro/", r)
	if !valid {
		resp := &Response{
			Code:   http.StatusOK,
			Status: 1,
			Data:   macro.GetProfiles(),
		}
		resp.Send(w)
	} else {
		val, err := strconv.Atoi(macroId)
		if err != nil {
			resp := &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtUnableToParseMacroId"),
			}
			resp.Send(w)
		} else {
			resp := &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   macro.GetProfile(val),
			}
			resp.Send(w)
		}
	}
}

// getKeyName returns response on /api/macro/keyInfo/
func getKeyName(w http.ResponseWriter, r *http.Request) {
	keyIndex, valid := getVar("/api/macro/keyInfo/", r)
	if !valid {
		resp := &Response{
			Code:    http.StatusOK,
			Status:  0,
			Message: language.GetValue("txtUnableToParseKeyIndex"),
		}
		resp.Send(w)
	} else {
		val, err := strconv.ParseUint(keyIndex, 10, 8) // base 10, 8-bit size
		if err != nil {
			resp := &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtUnableToParseMacroId"),
			}
			resp.Send(w)
		} else {
			resp := &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   inputmanager.GetKeyName(uint16(val)),
			}
			resp.Send(w)
		}
	}
}

// getTemperatures returns response on /temperatures
func getTemperature(w http.ResponseWriter, r *http.Request) {
	resp := &Response{}
	profile, valid := getVar("/api/temperatures/", r)
	if !valid {
		resp = &Response{
			Code:   http.StatusOK,
			Status: 0,
			Data:   temperatures.GetTemperatureProfiles(),
		}
	} else {
		if temperatureProfile := temperatures.GetTemperatureProfile(profile); temperatureProfile != nil {
			resp = &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   temperatureProfile,
			}
		} else {
			resp = &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtNoSuchTemperatureProfile"),
			}
		}
	}
	resp.Send(w)
}

// getExternalSources returns only registry ids and human-readable names.
func getExternalSources(w http.ResponseWriter, _ *http.Request) {
	entries, missing, err := externalsources.List(config.GetPaths())
	if err != nil {
		logger.Log(logger.Fields{
			"error":  err,
			"caller": "getExternalSources()",
		}).Error("Unable to load external source registry")
		(&Response{
			Code:    http.StatusOK,
			Status:  0,
			Message: "The external source registry is unavailable",
			Data:    []externalsources.Info{},
		}).Send(w)
		return
	}

	message := ""
	if missing {
		message = "No external sources are configured"
	}
	(&Response{
		Code:    http.StatusOK,
		Status:  1,
		Message: message,
		Data:    entries,
	}).Send(w)
}

// getTemperatureGraph returns response on for temperature graph
func getTemperatureGraph(w http.ResponseWriter, r *http.Request) {
	resp := &Response{}
	profile, valid := getVar("/api/temperatures/graph/", r)
	if !valid {
		resp = &Response{
			Code:    http.StatusOK,
			Status:  0,
			Message: language.GetValue("txtNoSuchTemperatureProfile"),
		}
	} else {
		if temperatureProfile := temperatures.GetTemperatureGraph(profile); temperatureProfile != nil {
			resp = &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   temperatureProfile,
			}
		} else {
			resp = &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtNoSuchTemperatureProfile"),
			}
		}
	}
	resp.Send(w)
}

// getColor returns response on /color
func getColor(w http.ResponseWriter, r *http.Request) {
	resp := &Response{}
	deviceId, valid := getVar("/api/color/", r)

	if !valid {
		resp = &Response{
			Code:   http.StatusOK,
			Status: 0,
			Data:   devices.GetRgbProfiles(),
		}
		resp.Send(w)
	} else {
		results := devices.CallDeviceMethod(deviceId, "GetRgbProfiles")
		if len(results) > 0 {
			resp = &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   results[0].Interface(),
			}
			resp.Send(w)
		} else {
			resp = &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtNoSuchRGBProfile"),
			}
			resp.Send(w)
		}
	}
}

// getZoneColor returns device zone colors
func getZoneColor(w http.ResponseWriter, r *http.Request) {
	resp := &Response{}
	deviceId, valid := getVar("/api/color/zone/", r)

	if !valid {
		resp = &Response{
			Code:   http.StatusOK,
			Status: 0,
			Data:   language.GetValue("txtNonExistingDevice"),
		}
		resp.Send(w)
	} else {
		results := devices.CallDeviceMethod(deviceId, "GetZoneColors")
		if len(results) > 0 {
			resp = &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   results[0].Interface(),
			}
			resp.Send(w)
		} else {
			resp = &Response{
				Code:   http.StatusOK,
				Status: 0,
				Data:   nil,
			}
			resp.Send(w)
		}
	}
}

// getColorData returns color profile data
func getColorData(w http.ResponseWriter, r *http.Request) {
	resp := &Response{}
	deviceId, validId := getDeviceID("/api/color/profile/", r)
	profileName, validProfile := getVarLast(r)

	if !validId || !validProfile {
		resp = &Response{
			Code:   http.StatusOK,
			Status: 0,
			Data:   language.GetValue("txtNoSuchRGBProfile"),
		}
		resp.Send(w)
	} else {
		results := devices.CallDeviceMethod(deviceId, "GetRgbProfile", profileName)
		if len(results) > 0 {
			resp = &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   results[0].Interface(),
			}
			resp.Send(w)
		} else {
			resp = &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtNoSuchRGBProfile"),
			}
			resp.Send(w)
		}
	}
}

// getPositionData returns device positions data
func getPositionData(w http.ResponseWriter, r *http.Request) {
	resp := &Response{}
	deviceId, valid := getVar("/api/position/", r)
	if !valid {
		resp = &Response{
			Code:   http.StatusOK,
			Status: 0,
			Data:   language.GetValue("txtInvalidPosition"),
		}
		resp.Send(w)
	} else {
		results := devices.CallDeviceMethod(deviceId, "GetDevicePositions")
		if len(results) > 0 {
			resp = &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   results[0].Interface(),
			}
			resp.Send(w)
		} else {
			resp = &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtInvalidPosition"),
			}
			resp.Send(w)
		}
	}
}

// getCommanderDuoOverride returns commander duo override
func getCommanderDuoOverride(w http.ResponseWriter, r *http.Request) {
	resp := &Response{}
	deviceId, valid := getVar("/api/color/override/", r)
	if !valid {
		resp = &Response{
			Code:   http.StatusOK,
			Status: 0,
			Data:   language.GetValue("txtUnableToValidateRequest"),
		}
		resp.Send(w)
	} else {
		results := devices.CallDeviceMethod(deviceId, "GetCommanderDuoOverride")
		if len(results) > 0 {
			resp = &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   results[0].Interface(),
			}
			resp.Send(w)
		} else {
			resp = &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtUnableToValidateRequest"),
			}
			resp.Send(w)
		}
	}
}

// getMediaKeys will return a map of media keys
func getMediaKeys(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   inputmanager.GetMediaKeys(),
	}
	resp.Send(w)
}

// getMediaKeys will return a map of non-media keys
func getInputKeys(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   inputmanager.GetInputKeys(),
	}
	resp.Send(w)
}

// getControllerKeys will return a map of non-media keys
func getControllerKeys(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   inputmanager.GetControllerKeys(),
	}
	resp.Send(w)
}

// getMouseButtons will return a map of mouse buttons
func getMouseButtons(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   inputmanager.GetMouseButtons(),
	}
	resp.Send(w)
}

// getSystrayData will return systray data
func getSystrayData(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   systray.Get(),
	}
	resp.Send(w)
}

// getKeyAssignmentTypes returns list of key assignment types for keyboard
func getKeyAssignmentTypes(w http.ResponseWriter, r *http.Request) {
	deviceId, valid := getVar("/api/keyboard/assignmentsTypes/", r)
	if !valid {
		resp := &Response{
			Code:    http.StatusOK,
			Status:  0,
			Message: language.GetValue("txtInvalidDeviceId"),
		}
		resp.Send(w)
	} else {
		results := devices.CallDeviceMethod(deviceId, "ProcessGetKeyAssignmentTypes")
		if len(results) > 0 {
			resp := &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   results[0].Interface(),
			}
			resp.Send(w)
		} else {
			resp := &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtUnableToGetAssignmentsTypes"),
			}
			resp.Send(w)
		}
	}
}

// getKeyAssignmentModifiers returns list of key assignment modifiers for keyboard
func getKeyAssignmentModifiers(w http.ResponseWriter, r *http.Request) {
	deviceId, valid := getVar("/api/keyboard/assignmentsModifiers/", r)
	if !valid {
		resp := &Response{
			Code:    http.StatusOK,
			Status:  0,
			Message: language.GetValue("txtInvalidDeviceId"),
		}
		resp.Send(w)
	} else {
		results := devices.CallDeviceMethod(deviceId, "ProcessGetKeyAssignmentModifiers")
		if len(results) > 0 {
			resp := &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   results[0].Interface(),
			}
			resp.Send(w)
		} else {
			resp := &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtUnableToGetAssignmentsModifiers"),
			}
			resp.Send(w)
		}
	}
}

// getKeyboardPerformance returns keyboard performance data
func getKeyboardPerformance(w http.ResponseWriter, r *http.Request) {
	deviceId, valid := getVar("/api/keyboard/getPerformance/", r)
	if !valid {
		resp := &Response{
			Code:    http.StatusOK,
			Status:  0,
			Message: language.GetValue("txtInvalidDeviceId"),
		}
		resp.Send(w)
	} else {
		results := devices.CallDeviceMethod(deviceId, "ProcessGetKeyboardPerformance")
		if len(results) > 0 {
			resp := &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   results[0].Interface(),
			}
			resp.Send(w)
		} else {
			resp := &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtUnableToGetKeyboardPerformance"),
			}
			resp.Send(w)
		}
	}
}

// getKeyboardFlashTap returns keyboard FlashTap data
func getKeyboardFlashTap(w http.ResponseWriter, r *http.Request) {
	deviceId, valid := getVar("/api/keyboard/getFlashTap/", r)
	if !valid {
		resp := &Response{
			Code:    http.StatusOK,
			Status:  0,
			Message: language.GetValue("txtInvalidDeviceId"),
		}
		resp.Send(w)
	} else {
		results := devices.CallDeviceMethod(deviceId, "GetFlashTap")
		if len(results) > 0 {
			resp := &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   results[0].Interface(),
			}
			resp.Send(w)
		} else {
			resp := &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtNoFlashTapData"),
			}
			resp.Send(w)
		}
	}
}

// getControlDialColors returns list of control dial colors
func getControlDialColors(w http.ResponseWriter, r *http.Request) {
	deviceId, valid := getVar("/api/keyboard/dial/getColors/", r)
	if !valid {
		resp := &Response{
			Code:    http.StatusOK,
			Status:  0,
			Message: language.GetValue("txtInvalidDeviceId"),
		}
		resp.Send(w)
	} else {
		results := devices.CallDeviceMethod(deviceId, "ProcessGetKeyboardControlDialColors")
		if len(results) > 0 {
			resp := &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   results[0].Interface(),
			}
			resp.Send(w)
		} else {
			resp := &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtUnableToGetControlDialColors"),
			}
			resp.Send(w)
		}
	}
}

// getEqualizers returns device equalizers
func getEqualizers(w http.ResponseWriter, r *http.Request) {
	resp := &Response{}
	deviceId, valid := getVar("/api/headset/getEqualizers/", r)
	if !valid {
		resp = &Response{
			Code:   http.StatusOK,
			Status: 0,
			Data:   language.GetValue("txtInvalidPosition"),
		}
		resp.Send(w)
	} else {
		results := devices.CallDeviceMethod(deviceId, "GetEqualizers")
		if len(results) > 0 {
			resp = &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   results[0].Interface(),
			}
			resp.Send(w)
		} else {
			resp = &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtInvalidPosition"),
			}
			resp.Send(w)
		}
	}
}

// getTemperatureProbes returns device temperature probes
func getTemperatureProbes(w http.ResponseWriter, r *http.Request) {
	resp := &Response{}
	deviceId, valid := getVar("/api/devices/probes/", r)
	if !valid {
		resp = &Response{
			Code:   http.StatusOK,
			Status: 0,
			Data:   language.GetValue("txtInvalidDeviceId"),
		}
		resp.Send(w)
	} else {
		results := devices.CallDeviceMethod(deviceId, "GetTemperatureProbes")
		if len(results) > 0 {
			resp = &Response{
				Code:   http.StatusOK,
				Status: 1,
				Data:   results[0].Interface(),
			}
			resp.Send(w)
		} else {
			resp = &Response{
				Code:    http.StatusOK,
				Status:  0,
				Message: language.GetValue("txtInvalidDeviceId"),
			}
			resp.Send(w)
		}
	}
}

// getLanguageData will return language data
func getLanguageData(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   language.GetLanguage(""),
	}
	resp.Send(w)
}

// getMouseDevice will return a map of mouse devices
func getMouseDevice(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:   http.StatusOK,
		Status: 1,
		Data:   devices.GetMouse(),
	}
	resp.Send(w)
}

// getMediaPlayback will return active media playback
func getMediaPlayback(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{Code: http.StatusOK, Status: 0}

	player, err := media.GetCurrentlyPlaying()
	if err != nil {
		resp.Data = err.Error()
	} else {
		resp.Data = player
		resp.Status = 1
	}
	resp.Send(w)
}

// getMediaPlayback will return active media playback
func mediaPlaybackControl(w http.ResponseWriter, r *http.Request) {
	resp := &Response{Code: http.StatusOK, Status: 0}
	mediaPlaybackAction, valid := getVar("/api/media/", r)
	if !valid {
		resp.Message = "Invalid playback action"
	} else {
		resp.Status = 1
		resp.Message = "OK"

		switch mediaPlaybackAction {
		case "previous":
			mediaInputControl(inputmanager.MediaPrev, false)
			break
		case "stop":
			mediaInputControl(inputmanager.MediaStop, false)
			break
		case "play":
			mediaInputControl(inputmanager.MediaPlayPause, false)
			break
		case "next":
			mediaInputControl(inputmanager.MediaNext, false)
			break
		case "volumeDown":
			mediaInputControl(inputmanager.VolumeDown, false)
			break
		case "volumeUp":
			mediaInputControl(inputmanager.VolumeUp, false)
			break
		case "mute":
			mediaInputControl(inputmanager.VolumeMute, false)
			break
		default:
			resp.Message = "Invalid playback action"
			resp.Status = 0
		}
	}
	resp.Send(w)
}

// updateDeviceEqualizers handles device equalizer update
func updateDeviceEqualizers(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessUpdateDeviceEqualizer(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// updateClusterOrder handles cluster device reordering
func updateClusterOrder(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessUpdateClusterOrder(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// updateRgbProfile handles device rgb profile update
func updateRgbProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessUpdateRgbProfile(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// newTemperatureProfile handles creation of new temperature profile
func newTemperatureProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessNewTemperatureProfile(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// deleteTemperatureProfile handles deletion of temperature profile
func deleteTemperatureProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessDeleteTemperatureProfile(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// updateTemperatureProfile handles update of temperature profile
func updateTemperatureProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessUpdateTemperatureProfile(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// updateTemperatureProfile handles update of temperature profile
func updateTemperatureProfileGraph(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessUpdateTemperatureProfileGraph(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setDeviceSpeed handles device speed changes
func setDeviceSpeed(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeSpeed(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setOperatingMode handles PWM operating mode
func setOperatingMode(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessOperatingMode(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setDeviceLabel handles device label changes
func setDeviceLabel(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessLabelChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setDeviceLcd handles device LCD changes
func setDeviceLcd(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessLcdChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setDeviceLcdProfile handles device LCD changes
func setDeviceLcdProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessLcdProfileChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeDeviceLcd handles device LCD updates
func changeDeviceLcd(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessLcdDeviceChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setDeviceLcdRotation handles device LCD rotation changes
func setDeviceLcdRotation(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessLcdRotationChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setDeviceLcdBrightness handles device LCD brightness changes
func setDeviceLcdBrightness(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessLcdBrightnessChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setDeviceLcdImage handles device LCD image changes
func setDeviceLcdImage(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessLcdImageChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// updateLcdProfile handles update of LCD profile
func updateLcdProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessLcdProfileUpdate(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// saveUserProfile handles saving custom user profiles
func saveUserProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSaveUserProfile(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeUserProfile handles user profile change
func changeUserProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeUserProfile(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// deleteUserProfile handles user profile deletion
func deleteUserProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessDeleteUserProfile(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeBrightness handles user brightness change
func changeBrightness(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessBrightnessChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeBrightnessGradual handles user brightness change via defined number from 0-100
func changeBrightnessGradual(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessBrightnessChangeGradual(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changePosition handles device position change
func changePosition(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessPositionChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setManualDeviceSpeed handles manual device speed changes
func setManualDeviceSpeed(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessManualChangeSpeed(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setDeviceColor handles device color changes
func setDeviceColor(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeColor(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setGlobalDeviceColor handles global color changes
func setGlobalDeviceColor(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessGlobalChangeColor(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setAllDevicesColor pushes a single static color to every registered device
func setAllDevicesColor(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSetAllDevicesColor(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setLinkAdapterColor handles LINK adapter color changes
func setLinkAdapterColor(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeLinkAdapterColor(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setLinkAdapterBulkColor handles LINK adapter bulk color changes
func setLinkAdapterBulkColor(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeLinkAdapterColorBulk(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// getRgbOverride return RGB override for given device
func getRgbOverride(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessGetRgbOverride(r)
	resp := &Response{
		Code:   request.Code,
		Status: request.Status,
		Data:   request.Data,
	}
	resp.Send(w)
}

// getRgbOverride return RGB override for given device
func setRgbOverride(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSetRgbOverride(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setTemperatureProbe return RGB override for given temperature probe
func setTemperatureProbe(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSetRgbTemperatureProbe(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// getLedData return RGB LED data
func getLedData(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessGetLedData(r)
	resp := &Response{
		Code:   request.Code,
		Status: request.Status,
		Data:   request.Data,
	}
	resp.Send(w)
}

// setLedData saves RGB LED data
func setLedData(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSetLedData(r)
	resp := &Response{
		Code:   request.Code,
		Status: request.Status,
		Data:   request.Data,
	}
	resp.Send(w)
}

// setOpenRgbIntegration saves OpenRGB integration state
func setOpenRgbIntegration(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSetOpenRgbIntegration(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setRgbCluster saves RGB cluster state
func setRgbCluster(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSetRgbCluster(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setKeyboardLiveSync saves keyboard live RGB sync state
func setKeyboardLiveSync(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSetKeyboardLiveSync(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setDeviceHardwareColor handles device hardware color changes
func setDeviceHardwareColor(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessHardwareChangeColor(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setDeviceStrip handles device RGB strip changes
func setDeviceStrip(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeStrip(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setDeviceLinkAdapter handles LINK adapter device change
func setDeviceLinkAdapter(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeLinkAdapter(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setExternalHubDeviceType handles device change of external-LED hub
func setExternalHubDeviceType(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessExternalHubDeviceType(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setARGBDevice handles device change of ARGB 3-pin devices
func setARGBDevice(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessARGBDevice(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setExternalHubDeviceAmount handles device amount change of external-LED hub
func setExternalHubDeviceAmount(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessExternalHubDeviceAmount(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// getDashboardSettings will get dashboard settings
func getDashboardSettings(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:      http.StatusOK,
		Status:    1,
		Dashboard: dashboard.GetDashboard(),
	}
	resp.Send(w)
}

// getDashboardLighting will get dashboard lighting status
func getDashboardLighting(w http.ResponseWriter, _ *http.Request) {
	type lightingResponse struct {
		Effect               string `json:"effect"`
		Brightness           int    `json:"brightness"`
		ClusterMembers       int    `json:"clusterMembers"`
		NonClusterRgbDevices int    `json:"nonClusterRgbDevices"`
	}

	effect := "off"
	brightness := 0
	snapshot, clusterMembers := getRGBClusterLightingStatus()
	if snapshot.Available {
		effect = snapshot.SelectedEffect
		brightness = int(snapshot.Brightness)
	}
	if effect == "" {
		effect = "off"
	}

	nonClusterCount := 0
	for _, dev := range devices.GetDevices() {
		if dev.Serial == "cluster" {
			continue
		}
		if devices.CallDeviceMethod(dev.Serial, "GetRgbProfiles") != nil && !devices.GetDeviceClusterStatus(dev.Serial) {
			nonClusterCount++
		}
	}

	res := lightingResponse{
		Effect:               effect,
		Brightness:           brightness,
		ClusterMembers:       clusterMembers,
		NonClusterRgbDevices: nonClusterCount,
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	data, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data)
}

// getDashboardDevices will get dashboard devices
func getDashboardDevices(w http.ResponseWriter, _ *http.Request) {
	resp := &Response{
		Code:    http.StatusOK,
		Status:  1,
		Devices: dashboard.GetDevices(),
	}
	resp.Send(w)
}

// addDashboardDevice will add dashboard device
func addDashboardDevice(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessAddDashboardDevice(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// removeDashboardDevice will remove dashboard device
func removeDashboardDevice(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessRemoveDashboardDevice(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// updateDashboardDeviceOrder persists dashboard devices in the requested order.
func updateDashboardDeviceOrder(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessUpdateDashboardDeviceOrder(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
		Devices: request.DeviceOrder,
	}
	resp.Send(w)
}

// setDashboardSettings handles dashboard settings change
func setDashboardSettings(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessDashboardSettingsChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setAudioSettings handles Virtual Audio settings change
func setAudioSettings(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessAudioSettingsChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setAudioOutputDeviceSettings handles Virtual Audio output device settings change
func setAudioOutputDeviceSettings(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessAudioOutputDeviceChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setDashboardSidebar handles dashboard sidebar collapse persistence
func setDashboardSidebar(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessDashboardSidebarChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setKeyboardColor handles keyboard color change
func setKeyboardColor(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessKeyboardColor(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setMiscColor handles misc device color change
func setMiscColor(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessMiscColor(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// saveDeviceProfile handles a new device profile
func saveDeviceProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSaveDeviceProfile(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeKeyboardLayout handles keyboard layout change
func changeKeyboardLayout(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeKeyboardLayout(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeControlDial handles keyboard control dial function change
func changeControlDial(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeControlDial(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeSleepMode handles device sleep mode change
func changeSleepMode(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeSleepMode(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeSleepMode handles device sleep mode change
func changeControllerSleepMode(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeControllerSleepMode(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changePollingRate handles device USB polling rate
func changePollingRate(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangePollingRate(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeAngleSnapping handles device angle snapping mode
func changeAngleSnapping(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeAngleSnapping(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeButtonOptimization handles device button optimization mode
func changeButtonOptimization(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeButtonOptimization(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeLeftHandMode handles device left hand mode
func changeLeftHandMode(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeLeftHandMode(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeLiftHeight handles device lift height change
func changeLiftHeight(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeLiftHeight(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeRippleControl handles device ripple control mode
func changeRippleControl(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeRippleControl(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeMotionSync handles device ripple control mode
func changeMotionSync(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeMotionSync(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeAutoBrightness handles device auto brightness mode
func changeAutoBrightness(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeAutoBrightness(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeDebounceTime handles device switch debounce time
func changeDebounceTime(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeDebounceTime(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeKeyAssignment handles device key assignment update
func changeKeyAssignment(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeKeyAssignment(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeKeyActuation handles device key assignment update
func changeKeyActuation(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeKeyActuation(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeMuteIndicator handles device mute indicator change
func changeMuteIndicator(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeMuteIndicator(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeActiveNoiseCancellation handles device Active Noise Cancellation
func changeActiveNoiseCancellation(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessActiveNoiseCancellation(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeSidetone handles device Sidetone
func changeSidetone(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSidetone(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeSidetoneValue handles device Sidetone value
func changeSidetoneValue(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSidetoneValue(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeHeadsetWheelOption handles headset wheel option value
func changeWheelOption(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessUpdateWheelOption(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeRgbScheduler handles RGB scheduler change
func changeRgbScheduler(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeRgbScheduler(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// deleteKeyboardProfile handles deletion of keyboard profile
func deleteKeyboardProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessDeleteKeyboardProfile(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeKeyboardProfile handles keyboard profile change
func changeKeyboardProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessChangeKeyboardProfile(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changePsuFanMode handles PSU fan mode change
func changePsuFanMode(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessPsuFanModeChange(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// saveMouseDpi handles mouse DPI save
func saveMouseDpi(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessMouseDpiSave(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// saveMouseGestures handles mouse gestures save
func saveMouseGestures(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessMouseGestureUpdate(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// saveMouseZoneColors handles mouse zone colors save
func saveMouseZoneColors(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessMouseZoneColorsSave(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// saveMouseDpiColors handles mouse DPI colors save
func saveMouseDpiColors(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessMouseDpiColorsSave(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// saveHeadsetZoneColors handles headset zone colors save
func saveHeadsetZoneColors(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessHeadsetZoneColorsSave(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// saveControllerZoneColors handles controller zone colors save
func saveControllerZoneColors(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessControllerZoneColorsSave(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// deleteMacroValue handles deletion of macro profile value
func deleteMacroValue(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessDeleteMacroValue(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// updateMacroValue handles update of macro profile value
func updateMacroValue(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessUpdateMacroValue(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// updateMacroSettings handles update of macro settings
func updateMacroSettings(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessUpdateMacroSettings(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// deleteMacroProfile handles deletion of macro profile
func deleteMacroProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessDeleteMacroProfile(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// newMacroProfile handles creation of new macro profile
func newMacroProfile(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessNewMacroProfile(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// newMacroProfileValue handles creation of new macro profile value
func newMacroProfileValue(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessNewMacroProfileValue(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// getGetKeyboardKey handles information about keyboard get
func getGetKeyboardKey(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessGetKeyboardKey(r)
	resp := &Response{
		Code:   request.Code,
		Status: request.Status,
		Data:   request.Data,
	}
	resp.Send(w)
}

// getGetKeyboardKeys handles information about keyboard get
func getGetKeyboardKeys(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessGetKeyboardKeys(r)
	resp := &Response{
		Code:   request.Code,
		Status: request.Status,
		Data:   request.Data,
	}
	resp.Send(w)
}

// setKeyboardPerformance handles setting keyboard performance
func setKeyboardPerformance(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSetKeyboardPerformance(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setKeyboardFlashTap handles setting keyboard flash tap settings
func setKeyboardFlashTap(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSetKeyboardFlashTap(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// setKeyboardPerformance handles setting keyboard performance
func setKeyboardControlDialColors(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSetKeyboardControlDialColors(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeControllerVibration handles device vibration module change
func changeControllerVibration(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessControllerVibration(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// changeControllerEmulation handles device emulation change
func changeControllerEmulation(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessControllerEmulation(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// getControllerGraph handles getting controller graph
func getControllerGraph(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessGetControllerGraph(r)
	resp := &Response{
		Code:   request.Code,
		Status: request.Status,
		Data:   request.Data,
	}
	resp.Send(w)
}

// setControllerGraph handles setting controller graph
func setControllerGraph(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessSetControllerGraph(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// newDeviceGradientColor handles creation of new gradient color
func newDeviceGradientColor(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessNewGradientColor(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
		Data:    request.Data,
	}
	resp.Send(w)
}

// deleteDeviceGradientColor handles deletion of gradient color
func deleteDeviceGradientColor(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessDeleteGradientColor(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
		Data:    request.Data,
	}
	resp.Send(w)
}

// setCommanderDuoOverride handles deletion of gradient color
func setCommanderDuoOverride(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessCommanderDuoOverride(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// getChannelData handles getting device channel data
func getChannelData(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessGetChannelDevice(r)
	resp := &Response{
		Code:   request.Code,
		Status: request.Status,
		Data:   request.Data,
	}
	resp.Send(w)
}

// updateDisplayData handles update of display values
func updateDisplayData(w http.ResponseWriter, r *http.Request) {
	request := requests.ProcessUpdateDisplayData(r)
	resp := &Response{
		Code:    request.Code,
		Status:  request.Status,
		Message: request.Message,
	}
	resp.Send(w)
}

// uiDeviceOverview handles device overview
func uiDeviceOverview(w http.ResponseWriter, r *http.Request) {
	deviceId, valid := getVar("/device/", r)
	if !valid {
		resp := &Response{
			Code:    http.StatusInternalServerError,
			Status:  0,
			Message: language.GetValue("txtUnableToProcessDeviceRequest"),
		}
		resp.Send(w)
		return
	}
	template := ""
	device := devices.GetDevice(deviceId)
	if device == nil {
		template = "404.html"
	} else {
		results := devices.CallDeviceMethod(
			deviceId,
			"GetDeviceTemplate",
		)
		if len(results) > 0 {
			template = results[0].String()
		}
	}

	if len(template) == 0 {
		resp := &Response{
			Code:    http.StatusInternalServerError,
			Status:  0,
			Message: language.GetValue("txtUnableToProcessDeviceRequest"),
		}
		resp.Send(w)
		return
	}

	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = devices.GetDevices()
	if openrgbDevice, ok := device.(*openrgbimport.Device); ok {
		snapshot := openrgbDevice.Snapshot()
		web.Device = &snapshot
		web.OpenRGBImportDevice = true
		web.OpenRGBImportConfig = snapshot.Config
		web.OpenRGBImportDisplaySerial = snapshot.DisplaySerial
		web.OpenRGBImportDisplaySerialLabel = snapshot.DisplaySerialLabel
		web.OpenRGBImportRGBCluster = snapshot.RGBCluster
	} else {
		web.Device = device
	}
	web.Lcd = lcd.GetLcdDevices()
	web.LCDImages = lcd.GetLcdImages()
	web.Temperatures = temperatures.GetTemperatureProfiles()
	web.Rgb = rgb.GetRGB().Profiles
	web.BuildInfo = version.GetBuildInfo()
	web.SystemInfo = systeminfo.GetInfo()
	web.Stats = stats.GetAIOStats()
	web.Macros = macro.GetProfiles()
	web.Dashboard = dashboard.GetDashboard()

	web.CpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetCpuTemperature())
	web.GpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature())
	t := templates.GetTemplate()

	for header := range headers {
		w.Header().Set(headers[header].Key, headers[header].Value)
	}

	executeTemplateOrRespond(w, t, template, web, true)
}

// uiIndex handles index page
func uiIndex(w http.ResponseWriter, _ *http.Request) {
	deviceList := devices.GetDevices()
	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = deviceList
	web.BuildInfo = version.GetBuildInfo()
	web.SystemInfo = systeminfo.GetInfo()
	web.CpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetCpuTemperature())
	web.GpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature())
	web.Dashboard = dashboard.GetDashboard()
	web.BatteryStats = stats.GetBatteryStats()
	web.Page = "index"

	t := templates.GetTemplate()
	for header := range headers {
		w.Header().Set(headers[header].Key, headers[header].Value)
	}

	executeTemplateOrRespond(w, t, "index.html", web, true)
}

type openRGBWorkspaceZoneSummary struct {
	Name     string
	LEDCount int
}

type openRGBWorkspaceSummary struct {
	DisplayIdentifier      string
	DisplayIdentifierLabel string
	HasMetadata            bool
	Vendor                 string
	Location               string
	Description            string
	Effect                 string
	Brightness             uint8
	Zones                  []openRGBWorkspaceZoneSummary
}

type devicesWorkspaceSummary struct {
	Product      string
	Serial       string
	Firmware     string
	Image        string
	Unavailable  bool
	HasBattery   bool
	BatteryLevel uint16
	OpenRGB      *openRGBWorkspaceSummary
	Lighting     *openRGBLightingWorkspaceSummary
	View         string
}

type openRGBLightingEffectSummary struct {
	ID       string
	Label    string
	Selected bool
}

type openRGBLightingWorkspaceSummary struct {
	ConfiguredEffect        string
	ConfiguredEffectLabel   string
	ConfiguredEffectIconURL string
	EffectSupported         bool
	SupportedEffects        []openRGBLightingEffectSummary
	HasBrightness           bool
	Brightness              uint8
	HasSpeedControl         bool
	Speed                   string
	ClusterControlled       bool
	PaletteKind             string
	SingleColorHex          string
	TwoColorStartHex        string
	TwoColorEndHex          string
	HasTemperature          bool
	TemperatureLow          openRGBLightingTemperaturePointSummary
	TemperatureMiddle       openRGBLightingTemperaturePointSummary
	TemperatureHigh         openRGBLightingTemperaturePointSummary
	TemperaturePoints       []openRGBLightingTemperaturePointSummary
	HasGradient             bool
	GradientStops           []openRGBLightingGradientStopSummary
	Customized              bool
}

type openRGBLightingTemperaturePointSummary struct {
	Role     string
	Label    string
	ColorHex string
	Celsius  string
}

type openRGBLightingGradientStopSummary struct {
	Number    int
	Position  string
	ColorHex  string
	Intensity string
}

func openRGBWorkspaceDisplayIdentifierLabel(label string) string {
	switch label {
	case "SERIAL":
		return "Serial"
	case "VERSION":
		return "Firmware"
	case "FALLBACK":
		return "OpenRGB ID"
	default:
		return ""
	}
}

func openRGBWorkspaceSummaryFromSnapshot(snapshot openrgbimport.DeviceSnapshot) *openRGBWorkspaceSummary {
	summary := &openRGBWorkspaceSummary{
		DisplayIdentifier:      snapshot.DisplaySerial,
		DisplayIdentifierLabel: openRGBWorkspaceDisplayIdentifierLabel(snapshot.DisplaySerialLabel),
		Description:            snapshot.Description,
		Effect:                 snapshot.Effect,
		Brightness:             snapshot.Brightness,
	}
	if snapshot.Config == nil {
		summary.HasMetadata = summary.DisplayIdentifier != "" || summary.Description != ""
		return summary
	}

	summary.Vendor = snapshot.Config.Vendor
	summary.Location = snapshot.Config.Location
	summary.HasMetadata = summary.DisplayIdentifier != "" || summary.Vendor != "" ||
		summary.Location != "" || summary.Description != ""
	summary.Zones = make([]openRGBWorkspaceZoneSummary, len(snapshot.Config.Zones))
	for index, zone := range snapshot.Config.Zones {
		summary.Zones[index] = openRGBWorkspaceZoneSummary{
			Name:     zone.Name,
			LEDCount: zone.LedCount,
		}
	}
	return summary
}

func openRGBLightingEffectDisplayLabel(id, label string) string {
	if strings.TrimSpace(label) != "" || id == "" {
		return label
	}
	return strings.ToUpper(id[:1]) + id[1:]
}

func openRGBLightingEffectIconURL(id string) string {
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(id)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeDevice) {
		return ""
	}
	stem, ok := strings.CutSuffix(descriptor.Icon, ".svg")
	if !ok || stem == "" {
		return ""
	}
	for _, character := range stem {
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') && character != '-' {
			return ""
		}
	}
	return "/static/img/icons/rgb/" + stem + ".svg"
}

func openRGBLightingWorkspaceSummaryFromSnapshot(snapshot openrgbimport.LightingSnapshot) *openRGBLightingWorkspaceSummary {
	summary := &openRGBLightingWorkspaceSummary{
		ConfiguredEffect:  snapshot.ConfiguredEffect,
		EffectSupported:   snapshot.EffectSupported,
		HasBrightness:     snapshot.HasBrightness,
		Brightness:        snapshot.Brightness,
		ClusterControlled: snapshot.ClusterControlled,
		SupportedEffects:  make([]openRGBLightingEffectSummary, len(snapshot.SupportedEffects)),
		PaletteKind:       snapshot.PaletteKind,
		SingleColorHex:    snapshot.SingleColorHex,
		TwoColorStartHex:  snapshot.TwoColorStartHex,
		TwoColorEndHex:    snapshot.TwoColorEndHex,
		HasTemperature:    snapshot.HasTemperature,
		HasGradient:       snapshot.HasGradient,
		Customized:        snapshot.Customized,
	}
	if summary.HasTemperature {
		summary.TemperatureLow = openRGBLightingTemperaturePointSummary{
			Role: "low", Label: "Low", ColorHex: snapshot.TemperatureLow.ColorHex,
			Celsius: strconv.FormatFloat(snapshot.TemperatureLow.Celsius, 'f', -1, 64),
		}
		summary.TemperatureMiddle = openRGBLightingTemperaturePointSummary{
			Role: "middle", Label: "Middle", ColorHex: snapshot.TemperatureMiddle.ColorHex,
			Celsius: strconv.FormatFloat(snapshot.TemperatureMiddle.Celsius, 'f', -1, 64),
		}
		summary.TemperatureHigh = openRGBLightingTemperaturePointSummary{
			Role: "high", Label: "High", ColorHex: snapshot.TemperatureHigh.ColorHex,
			Celsius: strconv.FormatFloat(snapshot.TemperatureHigh.Celsius, 'f', -1, 64),
		}
		summary.TemperaturePoints = []openRGBLightingTemperaturePointSummary{
			summary.TemperatureLow, summary.TemperatureMiddle, summary.TemperatureHigh,
		}
	}
	if summary.HasGradient {
		summary.GradientStops = make([]openRGBLightingGradientStopSummary, len(snapshot.GradientStops))
		for index, stop := range snapshot.GradientStops {
			summary.GradientStops[index] = openRGBLightingGradientStopSummary{
				Number: index + 1, Position: strconv.FormatFloat(stop.Position, 'f', -1, 64),
				ColorHex: stop.ColorHex, Intensity: strconv.FormatFloat(stop.Intensity, 'f', -1, 64),
			}
		}
	}
	if snapshot.HasSpeed {
		summary.HasSpeedControl = true
		summary.Speed = strconv.FormatFloat(snapshot.Speed, 'f', -1, 64)
	}
	for index, effect := range snapshot.SupportedEffects {
		displayLabel := openRGBLightingEffectDisplayLabel(effect.ID, effect.Label)
		summary.SupportedEffects[index] = openRGBLightingEffectSummary{
			ID:       effect.ID,
			Label:    displayLabel,
			Selected: snapshot.EffectSupported && effect.ID == snapshot.ConfiguredEffect,
		}
		if effect.ID == snapshot.ConfiguredEffect {
			summary.ConfiguredEffectLabel = displayLabel
			if snapshot.EffectSupported {
				summary.ConfiguredEffectIconURL = openRGBLightingEffectIconURL(effect.ID)
			}
		}
	}
	sort.Slice(summary.SupportedEffects, func(i, j int) bool {
		leftLabel := strings.ToLower(summary.SupportedEffects[i].Label)
		rightLabel := strings.ToLower(summary.SupportedEffects[j].Label)
		if leftLabel == rightLabel {
			return summary.SupportedEffects[i].ID < summary.SupportedEffects[j].ID
		}
		return leftLabel < rightLabel
	})
	return summary
}

func devicesWorkspaceSummaryForSerial(
	deviceList map[string]*common.Device,
	batteryStats map[string]stats.BatteryStats,
	serial string,
) (*devicesWorkspaceSummary, bool) {
	device, ok := deviceList[serial]
	if !ok || device == nil || device.Hidden || device.Serial != serial {
		return nil, false
	}

	summary := &devicesWorkspaceSummary{
		Product:     device.Product,
		Serial:      device.Serial,
		Firmware:    device.Firmware,
		Image:       device.Image,
		Unavailable: device.Unavailable,
		View:        "overview",
	}
	if battery, found := batteryStats[serial]; found {
		summary.HasBattery = true
		summary.BatteryLevel = battery.Level
	}
	if openRGBDevice, isOpenRGB := device.Instance.(*openrgbimport.Device); isOpenRGB &&
		openRGBDevice != nil && openRGBDevice.Serial == serial {
		snapshot := openRGBDevice.Snapshot()
		if snapshot.IsOpenRGB && snapshot.Serial == serial {
			summary.OpenRGB = openRGBWorkspaceSummaryFromSnapshot(snapshot)
			if lightingSnapshot, usable := openRGBDevice.LightingSnapshot(); usable {
				summary.Lighting = openRGBLightingWorkspaceSummaryFromSnapshot(lightingSnapshot)
			}
		}
	}

	return summary, true
}

// uiDevices handles the devices workspace
func uiDevices(w http.ResponseWriter, r *http.Request) {
	deviceList := devices.GetDevices()
	batteryStats := stats.GetBatteryStats()

	for header := range headers {
		w.Header().Set(headers[header].Key, headers[header].Value)
	}

	// Reject malformed encoding anywhere in the query, while deliberately
	// ignoring well-formed keys other than the optional device selection.
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "Invalid device selection", http.StatusBadRequest)
		return
	}

	var selectedDevice *devicesWorkspaceSummary
	if values, found := query["device"]; found {
		if len(values) != 1 || values[0] == "" || !common.AlphanumericDashSemiColon.MatchString(values[0]) {
			http.Error(w, "Invalid device selection", http.StatusBadRequest)
			return
		}

		selectedDevice, found = devicesWorkspaceSummaryForSerial(deviceList, batteryStats, values[0])
		if !found {
			http.NotFound(w, r)
			return
		}
		// Lighting is an optional presentation mode. Unknown, empty, or
		// duplicated view values deliberately retain the Overview workspace.
		if views := query["view"]; len(views) == 1 && views[0] == "lighting" && selectedDevice.Lighting != nil {
			selectedDevice.View = "lighting"
		}
	}

	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = deviceList
	if selectedDevice != nil {
		web.Device = selectedDevice
	}
	web.BatteryStats = batteryStats
	web.BuildInfo = version.GetBuildInfo()
	web.Dashboard = dashboard.GetDashboard()
	web.Page = "devices"

	t := templates.GetTemplate()
	executeTemplateOrRespond(w, t, "devices.html", web, true)
}

// uiTemperatureOverview handles overview of temperature profiles
func uiTemperatureOverview(w http.ResponseWriter, _ *http.Request) {
	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = devices.GetDevices()
	web.TemperatureProbes = devices.GetTemperatureProbes()
	web.HwMonSensors = temperatures.GetExternalHwMonSensors()
	web.Temperatures = temperatures.GetTemperatureProfiles()
	web.BuildInfo = version.GetBuildInfo()
	web.SystemInfo = systeminfo.GetInfo()
	web.Dashboard = dashboard.GetDashboard()
	web.CpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetCpuTemperature())
	web.GpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature())
	web.Page = "temperature"

	t := templates.GetTemplate()

	for header := range headers {
		w.Header().Set(headers[header].Key, headers[header].Value)
	}

	tpl := "temperature.html"
	if config.GetConfig().GraphProfiles {
		tpl = "temperatureGraph.html"
	}

	executeTemplateOrRespond(w, t, tpl, web, true)
}

// uiTemperatureGraphOverview handles overview of graph temperature profiles
func uiTemperatureGraphOverview(w http.ResponseWriter, _ *http.Request) {
	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = devices.GetDevices()
	web.TemperatureProbes = devices.GetTemperatureProbes()
	web.HwMonSensors = temperatures.GetExternalHwMonSensors()
	web.Temperatures = temperatures.GetTemperatureProfiles()
	web.BuildInfo = version.GetBuildInfo()
	web.SystemInfo = systeminfo.GetInfo()
	web.Dashboard = dashboard.GetDashboard()
	web.CpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetCpuTemperature())
	web.GpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature())
	web.Page = "temperature"

	t := templates.GetTemplate()

	for header := range headers {
		w.Header().Set(headers[header].Key, headers[header].Value)
	}

	executeTemplateOrRespond(w, t, "temperatureGraph.html", web)
}

// uiSchedulerOverview handles overview of scheduler settings
func uiSchedulerOverview(w http.ResponseWriter, _ *http.Request) {
	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = devices.GetDevices()
	web.Scheduler = scheduler.GetScheduler()
	web.BuildInfo = version.GetBuildInfo()
	web.SystemInfo = systeminfo.GetInfo()
	web.Dashboard = dashboard.GetDashboard()
	web.CpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetCpuTemperature())
	web.GpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature())
	web.Page = "scheduler"
	t := templates.GetTemplate()

	for header := range headers {
		w.Header().Set(headers[header].Key, headers[header].Value)
	}

	executeTemplateOrRespond(w, t, "scheduler.html", web)
}

// uiRgbEditor handles overview of RGB profiles
func uiRgbEditor(w http.ResponseWriter, _ *http.Request) {
	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = devices.GetDevices()
	web.RGBProfiles = devices.GetRgbProfiles()
	web.BuildInfo = version.GetBuildInfo()
	web.SystemInfo = systeminfo.GetInfo()
	web.Dashboard = dashboard.GetDashboard()
	web.CpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetCpuTemperature())
	web.GpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature())
	web.Page = "rgb"

	t := templates.GetTemplate()

	for header := range headers {
		w.Header().Set(headers[header].Key, headers[header].Value)
	}

	executeTemplateOrRespond(w, t, "rgb.html", web)
}

// uiRgbCluster handles overview of RGB Cluster
func uiRgbCluster(w http.ResponseWriter, _ *http.Request) {
	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = devices.GetDevices()
	web.Device = devices.GetDevice("cluster")
	web.BuildInfo = version.GetBuildInfo()
	web.SystemInfo = systeminfo.GetInfo()
	web.Dashboard = dashboard.GetDashboard()
	web.CpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetCpuTemperature())
	web.GpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature())
	web.Page = "rgbCluster"
	snapshot, _ := getRGBClusterLightingStatus()
	page := struct {
		templates.Web
		Lighting *clusterLightingWorkspaceSummary
	}{
		Web:      web,
		Lighting: clusterLightingWorkspaceSummaryFromSnapshot(snapshot),
	}

	t := templates.GetTemplate()

	for header := range headers {
		w.Header().Set(headers[header].Key, headers[header].Value)
	}

	executeTemplateOrRespond(w, t, "cluster.html", page, true)
}

// uiColorOverview handles overview or RGB profiles
func uiColorOverview(w http.ResponseWriter, _ *http.Request) {
	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = devices.GetDevices()
	web.Rgb = rgb.GetRgbProfiles()
	web.BuildInfo = version.GetBuildInfo()
	web.SystemInfo = systeminfo.GetInfo()
	web.Dashboard = dashboard.GetDashboard()
	web.CpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetCpuTemperature())
	web.GpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature())
	web.Page = "colors"
	t := templates.GetTemplate()

	for header := range headers {
		w.Header().Set(headers[header].Key, headers[header].Value)
	}

	executeTemplateOrRespond(w, t, "rgb.html", web)
}

// uiMacrosOverview handles overview of macro profiles
func uiMacrosOverview(w http.ResponseWriter, _ *http.Request) {
	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = devices.GetDevices()
	web.TemperatureProbes = devices.GetTemperatureProbes()
	web.Macros = macro.GetProfiles()
	web.InputActions = inputmanager.GetInputActions()
	web.BuildInfo = version.GetBuildInfo()
	web.SystemInfo = systeminfo.GetInfo()
	web.Dashboard = dashboard.GetDashboard()
	web.CpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetCpuTemperature())
	web.GpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature())
	web.Page = "macros"

	t := templates.GetTemplate()

	for header := range headers {
		w.Header().Set(headers[header].Key, headers[header].Value)
	}

	executeTemplateOrRespond(w, t, "macros.html", web)
}

// uiLcdOverview handles overview of LCD profiles
func uiLcdOverview(w http.ResponseWriter, _ *http.Request) {
	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = devices.GetDevices()
	web.TemperatureProbes = devices.GetTemperatureProbes()
	web.LCDProfiles = lcd.GetCustomLcdProfiles()
	web.LCDSensors = lcd.GetLcdSensors()
	web.InputActions = inputmanager.GetInputActions()
	web.BuildInfo = version.GetBuildInfo()
	web.SystemInfo = systeminfo.GetInfo()
	web.Dashboard = dashboard.GetDashboard()
	web.CpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetCpuTemperature())
	web.GpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature())
	web.Page = "lcd"

	t := templates.GetTemplate()

	for header := range headers {
		w.Header().Set(headers[header].Key, headers[header].Value)
	}

	executeTemplateOrRespond(w, t, "lcd.html", web, true)
}

// uiSettings handles index page
func uiSettings(w http.ResponseWriter, _ *http.Request) {
	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = devices.GetDevices()
	web.Scheduler = scheduler.GetScheduler()
	web.BuildInfo = version.GetBuildInfo()
	web.SystemInfo = systeminfo.GetInfo()
	web.Dashboard = dashboard.GetDashboard()
	web.Languages = language.GetLanguages()
	web.LanguageCode = dashboard.GetDashboard().LanguageCode
	web.Dashboard = dashboard.GetDashboard()
	web.CpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetCpuTemperature())
	web.GpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature())
	web.Displays = display.GetDisplays()
	web.AudioSettings = audio.GetAudio()
	web.OutputDevices = audio.GetSinks()
	web.SystemService = config.IsSystemService()
	web.RGBModes = []string{
		"circle",
		"circleshift",
		"comet",
		"datastream",
		"colorpulse",
		"colorshift",
		"colorwarp",
		"cpu-temperature",
		"flickering",
		"gpu-temperature",
		"off",
		"plasmacore",
		"rainbow",
		"pastelrainbow",
		"rotator",
		"spinner",
		"static",
		"stardust",
		"storm",
		"watercolor",
		"wave",
	}
	web.Page = "settings"

	t := templates.GetTemplate()

	for header := range headers {
		w.Header().Set(headers[header].Key, headers[header].Value)
	}

	executeTemplateOrRespond(w, t, "settings.html", web)
}

// uiXeneon handles kiosk page
func uiXeneon(w http.ResponseWriter, _ *http.Request) {
	var xeneon *common.Device
	for _, val := range devices.GetDevices() {
		if val.ProductType == common.ProductTypeXeneonEdge {
			xeneon = val
		}
	}

	if xeneon == nil {
		resp := &Response{
			Code:    http.StatusInternalServerError,
			Message: language.GetValue("txtXeneonEdgeUnavailable"),
		}
		resp.Send(w)
		return
	}

	deviceList := devices.GetDevices()
	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = deviceList
	web.BuildInfo = version.GetBuildInfo()
	web.SystemInfo = systeminfo.GetInfo()
	web.CpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetCpuTemperature())
	web.GpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature())
	web.Dashboard = dashboard.GetDashboard()
	web.BatteryStats = stats.GetBatteryStats()
	web.Device = xeneon
	web.Page = "xeneon"

	t := templates.GetTemplate()
	for header := range headers {
		w.Header().Set(headers[header].Key, headers[header].Value)
	}

	err := t.ExecuteTemplate(w, "xeneon.html", web)
	if err != nil {
		fmt.Println(err)
		resp := &Response{
			Code:    http.StatusInternalServerError,
			Message: language.GetValue("txtUnableToServeWebContent"),
		}
		resp.Send(w)
	}
}

// getVar will extract dynamic path from GET request
func getVar(path string, r *http.Request) (string, bool) {
	value := strings.TrimPrefix(r.URL.Path, path)
	if value == "" || strings.Contains(value, "/") {
		return "", false
	}

	if !common.AlphanumericDashSemiColon.MatchString(value) {
		return "", false
	}

	return value, true
}

// getVarLast will extract dynamic path from GET request
func getVarLast(r *http.Request) (string, bool) {
	parts := strings.Split(r.URL.Path, "/")
	value := parts[len(parts)-1]

	if !common.AlphanumericDashSemiColon.MatchString(value) {
		return "", false
	}
	return value, true
}

// getDeviceID will extract device id from dynamic path
func getDeviceID(uri string, r *http.Request) (string, bool) {
	path := strings.TrimPrefix(r.URL.Path, uri)
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", false
	}

	value := parts[0]
	if !common.AlphanumericDashSemiColon.MatchString(value) {
		return "", false
	}
	return value, true
}

func getOpenRGBImportDeviceBySerial(serial string) (*openrgbimport.Device, error) {
	allDevices := devices.GetDevices()
	commonDev, ok := allDevices[serial]
	if !ok {
		return nil, fmt.Errorf("Device not found")
	}

	dev, ok := commonDev.Instance.(*openrgbimport.Device)
	if !ok {
		return nil, fmt.Errorf("Invalid device instance")
	}
	return dev, nil
}

func getOpenRGBImportLightingDeviceBySerial(serial string) (*openrgbimport.Device, error) {
	if serial == "" || !common.AlphanumericDashRegex.MatchString(serial) {
		return nil, fmt.Errorf("invalid OpenRGB device serial")
	}

	wrapper, device, ok := lookupOpenRGBImportForLighting(serial)
	if !ok || wrapper == nil || device == nil || wrapper.Hidden || wrapper.Unavailable || wrapper.Serial != serial {
		return nil, fmt.Errorf("OpenRGB device is not available")
	}
	wrapperDevice, ok := wrapper.Instance.(*openrgbimport.Device)
	if !ok || wrapperDevice == nil || wrapperDevice != device || !device.MatchesOpenRGBImport(serial) {
		return nil, fmt.Errorf("OpenRGB device is not available")
	}
	return device, nil
}

func decodeRequestBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	err := json.NewDecoder(r.Body).Decode(dst)
	if err != nil {
		resp := &Response{
			Code:    http.StatusOK,
			Status:  0,
			Message: "Invalid request body",
		}
		resp.Send(w)
		return false
	}
	return true
}

func decodeOpenRGBImportRequest(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, openRGBImportRequestLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid request body"}).Send(w)
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid request body"}).Send(w)
		return false
	}
	return true
}

func discoverOpenRGBImportControllers(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength > openRGBImportRequestLimit {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Request body is too large"}).Send(w)
		return
	}
	data, err := discoverOpenRGBImports(r.Context())
	if err != nil {
		(&Response{
			Code:    http.StatusOK,
			Status:  0,
			Message: err.Error(),
			Data:    data,
		}).Send(w)
		return
	}
	(&Response{
		Code:    http.StatusOK,
		Status:  1,
		Message: "OpenRGB controller discovery completed",
		Data:    data,
	}).Send(w)
}

func importOpenRGBImportControllers(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Keys []string `json:"keys"`
	}
	if !decodeOpenRGBImportRequest(w, r, &request) {
		return
	}
	if len(request.Keys) == 0 {
		(&Response{Code: http.StatusOK, Status: 0, Message: "At least one OpenRGB selection key is required"}).Send(w)
		return
	}
	if len(request.Keys) > openRGBImportBatchLimit {
		(&Response{Code: http.StatusOK, Status: 0, Message: fmt.Sprintf("Too many OpenRGB selections; maximum batch size is %d", openRGBImportBatchLimit)}).Send(w)
		return
	}
	data, err := importOpenRGBImports(r.Context(), request.Keys)
	if err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: err.Error(), Data: data}).Send(w)
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "OpenRGB controllers imported", Data: data}).Send(w)
}

func removeOpenRGBImportControllers(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Serials []string `json:"serials"`
	}
	if !decodeOpenRGBImportRequest(w, r, &request) {
		return
	}
	if len(request.Serials) == 0 {
		(&Response{Code: http.StatusOK, Status: 0, Message: "At least one OpenRGB import serial is required"}).Send(w)
		return
	}
	if len(request.Serials) > openRGBImportBatchLimit {
		(&Response{Code: http.StatusOK, Status: 0, Message: fmt.Sprintf("Too many OpenRGB imports; maximum batch size is %d", openRGBImportBatchLimit)}).Send(w)
		return
	}
	data, err := removeOpenRGBImports(r.Context(), request.Serials)
	if err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: err.Error(), Data: data}).Send(w)
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "OpenRGB imports removed", Data: data}).Send(w)
}

func refreshOpenRGBImportManager(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength > openRGBImportRequestLimit {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Request body is too large"}).Send(w)
		return
	}
	if err := refreshOpenRGBImports(r.Context()); err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: err.Error()}).Send(w)
		return
	}
	(&Response{
		Code:    http.StatusOK,
		Status:  1,
		Message: "OpenRGB import reconciliation requested",
		Data:    map[string]bool{"queued": true},
	}).Send(w)
}

func setOpenRGBImportBrightness(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial     *string `json:"serial"`
		Brightness *int    `json:"brightness"`
	}

	if !decodeOpenRGBImportRequest(w, r, &req) {
		return
	}
	if req.Serial == nil || *req.Serial == "" || req.Brightness == nil || *req.Brightness < 0 || *req.Brightness > 100 {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid brightness request"}).Send(w)
		return
	}

	dev, err := getOpenRGBImportLightingDeviceBySerial(*req.Serial)
	if err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "OpenRGB device is not available"}).Send(w)
		return
	}

	err = setOpenRGBImportBrightnessValue(dev, uint8(*req.Brightness))
	if err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Unable to set brightness"}).Send(w)
		return
	}

	resp := &Response{
		Code:    http.StatusOK,
		Status:  1,
		Message: "Brightness set",
	}
	resp.Send(w)
}

func setOpenRGBImportSingleColor(w http.ResponseWriter, r *http.Request) {
	request := struct {
		Serial string `json:"serial"`
		Effect string `json:"effect"`
		Color  string `json:"color"`
	}{}
	if !decodeOpenRGBImportRequest(w, r, &request) {
		return
	}
	if request.Serial == "" || request.Effect == "" || request.Color == "" {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Missing required properties"}).Send(w)
		return
	}
	color, err := parseHexColor(request.Color)
	if err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid color format"}).Send(w)
		return
	}
	device, err := getOpenRGBImportLightingDeviceBySerial(request.Serial)
	if err != nil || device == nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "OpenRGB import is unavailable"}).Send(w)
		return
	}
	if !device.SupportsEffect(request.Effect) {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Unsupported effect"}).Send(w)
		return
	}
	if err := setOpenRGBImportColorValue(device, request.Serial, request.Effect, color); err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Failed to set device color"}).Send(w)
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Applied successfully"}).Send(w)
}

func setOpenRGBImportTwoColor(w http.ResponseWriter, r *http.Request) {
	request := struct {
		Serial string `json:"serial"`
		Effect string `json:"effect"`
		Start  string `json:"start"`
		End    string `json:"end"`
	}{}
	if !decodeOpenRGBImportRequest(w, r, &request) {
		return
	}
	if request.Serial == "" || request.Effect == "" || request.Start == "" || request.End == "" {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Missing required properties"}).Send(w)
		return
	}
	start, err := parseHexColor(request.Start)
	if err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid color format"}).Send(w)
		return
	}
	end, err := parseHexColor(request.End)
	if err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid color format"}).Send(w)
		return
	}
	device, err := getOpenRGBImportLightingDeviceBySerial(request.Serial)
	if err != nil || device == nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "OpenRGB import is unavailable"}).Send(w)
		return
	}
	if !device.SupportsEffect(request.Effect) {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Unsupported effect"}).Send(w)
		return
	}
	if err := setOpenRGBImportTwoColorValue(device, request.Serial, request.Effect, start, end); err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Failed to set device colors"}).Send(w)
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Applied successfully"}).Send(w)
}

type openRGBImportTemperaturePointRequest struct {
	Color   *string  `json:"color"`
	Celsius *float64 `json:"celsius"`
}

func setOpenRGBImportTemperature(w http.ResponseWriter, r *http.Request) {
	request := struct {
		Serial string                                `json:"serial"`
		Effect string                                `json:"effect"`
		Low    *openRGBImportTemperaturePointRequest `json:"low"`
		Middle *openRGBImportTemperaturePointRequest `json:"middle"`
		High   *openRGBImportTemperaturePointRequest `json:"high"`
	}{}
	if !decodeOpenRGBImportRequest(w, r, &request) {
		return
	}
	if request.Serial == "" || request.Effect == "" || request.Low == nil || request.Middle == nil || request.High == nil ||
		request.Low.Color == nil || request.Low.Celsius == nil || request.Middle.Color == nil || request.Middle.Celsius == nil ||
		request.High.Color == nil || request.High.Celsius == nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Missing required properties"}).Send(w)
		return
	}
	lowColor, lowErr := parseHexColor(*request.Low.Color)
	middleColor, middleErr := parseHexColor(*request.Middle.Color)
	highColor, highErr := parseHexColor(*request.High.Color)
	if lowErr != nil || middleErr != nil || highErr != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid color format"}).Send(w)
		return
	}
	lowCelsius, middleCelsius, highCelsius := *request.Low.Celsius, *request.Middle.Celsius, *request.High.Celsius
	if math.IsNaN(lowCelsius) || math.IsInf(lowCelsius, 0) || math.IsNaN(middleCelsius) || math.IsInf(middleCelsius, 0) ||
		math.IsNaN(highCelsius) || math.IsInf(highCelsius, 0) || !(lowCelsius < middleCelsius && middleCelsius < highCelsius) {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid temperature request"}).Send(w)
		return
	}
	device, err := getOpenRGBImportLightingDeviceBySerial(request.Serial)
	if err != nil || device == nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "OpenRGB import is unavailable"}).Send(w)
		return
	}
	if !device.SupportsEffect(request.Effect) {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Unsupported effect"}).Send(w)
		return
	}
	low := lightingsettings.TemperaturePoint{Color: lowColor, Celsius: lowCelsius}
	middle := lightingsettings.TemperaturePoint{Color: middleColor, Celsius: middleCelsius}
	high := lightingsettings.TemperaturePoint{Color: highColor, Celsius: highCelsius}
	if err := setOpenRGBImportTemperatureValue(device, request.Serial, request.Effect, low, middle, high); err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Failed to set temperature colors"}).Send(w)
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Applied successfully"}).Send(w)
}

type openRGBImportGradientStopRequest struct {
	Position  *float64 `json:"position"`
	Color     *string  `json:"color"`
	Intensity *float64 `json:"intensity"`
}

func setOpenRGBImportGradient(w http.ResponseWriter, r *http.Request) {
	request := struct {
		Serial string                              `json:"serial"`
		Effect string                              `json:"effect"`
		Stops  *[]openRGBImportGradientStopRequest `json:"stops"`
	}{}
	if !decodeOpenRGBImportRequest(w, r, &request) {
		return
	}
	if request.Serial == "" || request.Effect == "" || request.Stops == nil ||
		len(*request.Stops) < 2 || len(*request.Stops) > openRGBImportGradientStopLimit {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid Gradient request"}).Send(w)
		return
	}

	stops := make([]lightingsettings.GradientStop, len(*request.Stops))
	previousPosition := -1.0
	for index, input := range *request.Stops {
		if input.Position == nil || input.Color == nil || input.Intensity == nil {
			(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid Gradient request"}).Send(w)
			return
		}
		position, intensity := *input.Position, *input.Intensity
		if math.IsNaN(position) || math.IsInf(position, 0) || position < 0 || position > 1 ||
			math.IsNaN(intensity) || math.IsInf(intensity, 0) || intensity < 0 || intensity > 1 ||
			position < previousPosition {
			(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid Gradient request"}).Send(w)
			return
		}
		color, err := parseHexColor(*input.Color)
		if err != nil {
			(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid color format"}).Send(w)
			return
		}
		stops[index] = lightingsettings.GradientStop{Position: position, Color: color, Intensity: intensity}
		previousPosition = position
	}

	device, err := getOpenRGBImportLightingDeviceBySerial(request.Serial)
	if err != nil || device == nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "OpenRGB import is unavailable"}).Send(w)
		return
	}
	if !device.SupportsEffect(request.Effect) {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Unsupported effect"}).Send(w)
		return
	}
	if err := setOpenRGBImportGradientValue(device, request.Serial, request.Effect, stops); err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Failed to set Gradient"}).Send(w)
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Applied successfully"}).Send(w)
}

func resetOpenRGBImportEffectCustomization(w http.ResponseWriter, r *http.Request) {
	request := struct {
		Serial string `json:"serial"`
		Effect string `json:"effect"`
	}{}
	if !decodeOpenRGBImportRequest(w, r, &request) {
		return
	}
	if request.Serial == "" || request.Effect == "" {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Missing required properties"}).Send(w)
		return
	}
	device, err := getOpenRGBImportLightingDeviceBySerial(request.Serial)
	if err != nil || device == nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "OpenRGB import is unavailable"}).Send(w)
		return
	}
	if !device.SupportsEffect(request.Effect) {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Unsupported effect"}).Send(w)
		return
	}
	if err := resetOpenRGBImportCustomizationValue(device, request.Serial, request.Effect); err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Failed to reset effect customization"}).Send(w)
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Reset successfully"}).Send(w)
}

func setOpenRGBImportEffect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial *string `json:"serial"`
		Effect *string `json:"effect"`
	}

	if !decodeOpenRGBImportRequest(w, r, &req) {
		return
	}
	if req.Serial == nil || *req.Serial == "" || req.Effect == nil || *req.Effect == "" {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid effect request"}).Send(w)
		return
	}

	dev, err := getOpenRGBImportLightingDeviceBySerial(*req.Serial)
	if err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "OpenRGB device is not available"}).Send(w)
		return
	}
	if !dev.SupportsEffect(*req.Effect) {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Unsupported effect"}).Send(w)
		return
	}

	err = setOpenRGBImportEffectValue(dev, *req.Effect)
	if err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Unable to set effect"}).Send(w)
		return
	}

	resp := &Response{
		Code:    http.StatusOK,
		Status:  1,
		Message: "Effect set",
	}
	resp.Send(w)
}

func setOpenRGBImportSpeed(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial *string         `json:"serial"`
		Effect *string         `json:"effect"`
		Speed  json.RawMessage `json:"speed"`
	}

	if !decodeOpenRGBImportRequest(w, r, &req) {
		return
	}

	var speed float64
	if req.Serial == nil || *req.Serial == "" || req.Effect == nil || *req.Effect == "" || len(req.Speed) == 0 ||
		json.Unmarshal(req.Speed, &speed) != nil || math.IsNaN(speed) || math.IsInf(speed, 0) {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid speed request"}).Send(w)
		return
	}
	minimum, maximum := rgb.ProfileSpeedRange(*req.Effect)
	descriptor, known := rgb.SoftwareEffectDescriptorByID(*req.Effect)
	if !known || !descriptor.Scope.Includes(rgb.EffectScopeDevice) || !descriptor.SupportsSpeed ||
		speed < minimum || speed > maximum {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid speed request"}).Send(w)
		return
	}

	dev, err := getOpenRGBImportLightingDeviceBySerial(*req.Serial)
	if err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "OpenRGB device is not available"}).Send(w)
		return
	}
	if !dev.SupportsEffect(*req.Effect) {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Unsupported effect"}).Send(w)
		return
	}

	if err = setOpenRGBImportSpeedValue(dev, *req.Serial, *req.Effect, speed); err != nil {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Unable to set speed"}).Send(w)
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Speed set"}).Send(w)
}

func setOpenRGBImportConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial string                     `json:"serial"`
		Zones  []openrgbimport.ZoneConfig `json:"zones"`
	}

	if !decodeRequestBody(w, r, &req) {
		return
	}

	serial := req.Serial
	if serial == "" {
		serial = "openrgb-mobo-1"
	}

	dev, err := getOpenRGBImportDeviceBySerial(serial)
	if err != nil {
		resp := &Response{
			Code:    http.StatusOK,
			Status:  0,
			Message: err.Error(),
		}
		resp.Send(w)
		return
	}

	err = dev.SaveDeviceConfig(&openrgbimport.DeviceConfig{
		Serial: serial,
		Zones:  req.Zones,
	})
	if err != nil {
		resp := &Response{
			Code:    http.StatusOK,
			Status:  0,
			Message: err.Error(),
		}
		resp.Send(w)
		return
	}

	resp := &Response{
		Code:    http.StatusOK,
		Status:  1,
		Message: "Config saved",
	}
	resp.Send(w)
}

func handleFunc(mux *http.ServeMux, path, method string, handler func(w http.ResponseWriter, r *http.Request)) {
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == method {
			handler(w, r)
		} else {
			http.Error(w, language.GetValue("txtMethodNotAllowed"), http.StatusMethodNotAllowed)
		}
	})
}

// setRoutes will set up all routes
func setRoutes() http.Handler {
	protection := newLocalAPIProtection(config.GetConfig().ListenPort)
	r := http.NewServeMux()
	fs := http.FileServer(http.Dir(config.GetPaths().StaticAssetRoot))
	r.Handle("/static/", http.StripPrefix("/static/", fs))

	// GET
	handleFunc(r, "/api/", http.MethodGet, homePage)
	handleFunc(r, "/api/cpuTemp", http.MethodGet, getCpuTemperature)
	handleFunc(r, "/api/cpuTemp/clean", http.MethodGet, getCpuTemperatureClean)
	handleFunc(r, "/api/cpuLoad", http.MethodGet, getCpuLoad)
	handleFunc(r, "/api/gpuTemp", http.MethodGet, getGpuTemperature)
	handleFunc(r, "/api/gpuTemps", http.MethodGet, getGpuTemperatures)
	handleFunc(r, "/api/gpuTemp/clean", http.MethodGet, getGpuTemperatureClean)
	handleFunc(r, "/api/gpuLoad", http.MethodGet, getGpuLoad)
	handleFunc(r, "/api/storageTemp", http.MethodGet, getStorageTemperature)
	handleFunc(r, "/api/batteryStats", http.MethodGet, getBatteryStats)
	handleFunc(r, "/api/devices/", http.MethodGet, getDevices)
	handleFunc(r, "/api/color/", http.MethodGet, getColor)
	handleFunc(r, "/api/color/zone/", http.MethodGet, getZoneColor)
	handleFunc(r, "/api/color/profile/", http.MethodGet, getColorData)
	handleFunc(r, "/api/color/override/", http.MethodGet, getCommanderDuoOverride)
	handleFunc(r, "/api/temperatures/", http.MethodGet, getTemperature)
	handleFunc(r, "/api/temperatures/graph/", http.MethodGet, getTemperatureGraph)
	handleFunc(r, "/api/external-sources", http.MethodGet, getExternalSources)
	handleFunc(r, "/api/input/media", http.MethodGet, getMediaKeys)
	handleFunc(r, "/api/input/keyboard", http.MethodGet, getInputKeys)
	handleFunc(r, "/api/input/mouse", http.MethodGet, getMouseButtons)
	handleFunc(r, "/api/input/controller", http.MethodGet, getControllerKeys)
	handleFunc(r, "/api/led/", http.MethodGet, getDeviceLed)
	handleFunc(r, "/api/macro/", http.MethodGet, getMacro)
	handleFunc(r, "/api/macro/keyInfo/", http.MethodGet, getKeyName)
	handleFunc(r, "/api/dashboard", http.MethodGet, getDashboardSettings)
	handleFunc(r, "/api/dashboard/lighting", http.MethodGet, getDashboardLighting)
	handleFunc(r, "/api/dashboard/devices/get", http.MethodGet, getDashboardDevices)
	handleFunc(r, "/api/keyboard/assignmentsTypes/", http.MethodGet, getKeyAssignmentTypes)
	handleFunc(r, "/api/keyboard/assignmentsModifiers/", http.MethodGet, getKeyAssignmentModifiers)
	handleFunc(r, "/api/keyboard/getPerformance/", http.MethodGet, getKeyboardPerformance)
	handleFunc(r, "/api/keyboard/getFlashTap/", http.MethodGet, getKeyboardFlashTap)
	handleFunc(r, "/api/systray", http.MethodGet, getSystrayData)
	handleFunc(r, "/api/keyboard/dial/getColors/", http.MethodGet, getControlDialColors)
	handleFunc(r, "/api/getSupportedDevices", http.MethodGet, getSupportedDevices)
	handleFunc(r, "/api/openrgb/status", http.MethodGet, getOpenRGBStatus)
	handleFunc(r, "/api/backup", http.MethodGet, backup.PerformBackup)
	handleFunc(r, "/api/position/", http.MethodGet, getPositionData)
	handleFunc(r, "/api/headset/getEqualizers/", http.MethodGet, getEqualizers)
	handleFunc(r, "/api/language/", http.MethodGet, getLanguageData)
	handleFunc(r, "/api/devices/probes/", http.MethodGet, getTemperatureProbes)
	handleFunc(r, "/api/devices/mouse", http.MethodGet, getMouseDevice)
	handleFunc(r, "/api/media/playback", http.MethodGet, getMediaPlayback)
	handleFunc(r, "/api/security/token", http.MethodGet, protection.tokenHandler)

	// POST
	handleFunc(r, "/api/media/", http.MethodPost, mediaPlaybackControl)
	handleFunc(r, "/api/openrgbimport/discover", http.MethodPost, discoverOpenRGBImportControllers)
	handleFunc(r, "/api/openrgbimport/import", http.MethodPost, importOpenRGBImportControllers)
	handleFunc(r, "/api/openrgbimport/remove", http.MethodPost, removeOpenRGBImportControllers)
	handleFunc(r, "/api/openrgbimport/refresh", http.MethodPost, refreshOpenRGBImportManager)
	handleFunc(r, "/api/openrgbimport/speed", http.MethodPost, setOpenRGBImportSpeed)
	handleFunc(r, "/api/openrgbimport/single-color", http.MethodPost, setOpenRGBImportSingleColor)
	handleFunc(r, "/api/openrgbimport/two-color", http.MethodPost, setOpenRGBImportTwoColor)
	handleFunc(r, "/api/openrgbimport/temperature", http.MethodPost, setOpenRGBImportTemperature)
	handleFunc(r, "/api/openrgbimport/gradient", http.MethodPost, setOpenRGBImportGradient)
	handleFunc(r, "/api/openrgbimport/effect-reset", http.MethodPost, resetOpenRGBImportEffectCustomization)
	handleFunc(r, "/api/openrgbimport/effect", http.MethodPost, setOpenRGBImportEffect)
	handleFunc(r, "/api/openrgbimport/brightness", http.MethodPost, setOpenRGBImportBrightness)
	handleFunc(r, "/api/cluster/lighting/effect", http.MethodPost, setRGBClusterLightingEffect)
	handleFunc(r, "/api/cluster/lighting/effect-reset", http.MethodPost, resetRGBClusterLightingEffect)
	handleFunc(r, "/api/cluster/lighting/brightness", http.MethodPost, setRGBClusterLightingBrightness)
	handleFunc(r, "/api/cluster/lighting/speed", http.MethodPost, setRGBClusterLightingSpeed)
	handleFunc(r, "/api/cluster/lighting/single-color", http.MethodPost, setRGBClusterLightingSingleColor)
	handleFunc(r, "/api/cluster/lighting/two-color", http.MethodPost, setRGBClusterLightingTwoColor)
	handleFunc(r, "/api/cluster/lighting/temperature", http.MethodPost, setRGBClusterLightingTemperature)
	handleFunc(r, "/api/cluster/lighting/gradient", http.MethodPost, setRGBClusterLightingGradient)
	handleFunc(r, "/api/openrgbimport/config", http.MethodPost, setOpenRGBImportConfig)
	handleFunc(r, "/api/temperatures/new", http.MethodPost, newTemperatureProfile)
	handleFunc(r, "/api/speed", http.MethodPost, setDeviceSpeed)
	handleFunc(r, "/api/speed/manual", http.MethodPost, setManualDeviceSpeed)
	handleFunc(r, "/api/operatingMode", http.MethodPost, setOperatingMode)
	handleFunc(r, "/api/color", http.MethodPost, setDeviceColor)
	handleFunc(r, "/api/color/global", http.MethodPost, setGlobalDeviceColor)
	handleFunc(r, "/api/color/all", http.MethodPost, setAllDevicesColor)
	handleFunc(r, "/api/color/linkAdapter", http.MethodPost, setLinkAdapterColor)
	handleFunc(r, "/api/color/linkAdapter/bulk", http.MethodPost, setLinkAdapterBulkColor)
	handleFunc(r, "/api/color/getOverride", http.MethodPost, getRgbOverride)
	handleFunc(r, "/api/color/setOverride", http.MethodPost, setRgbOverride)
	handleFunc(r, "/api/color/setTemperatureProbe", http.MethodPost, setTemperatureProbe)
	handleFunc(r, "/api/color/getLedData", http.MethodPost, getLedData)
	handleFunc(r, "/api/color/setLedData", http.MethodPost, setLedData)
	handleFunc(r, "/api/color/setOpenRgbIntegration", http.MethodPost, setOpenRgbIntegration)
	handleFunc(r, "/api/color/setCluster", http.MethodPost, setRgbCluster)
	handleFunc(r, "/api/keyboard/liveSync", http.MethodPost, setKeyboardLiveSync)
	handleFunc(r, "/api/color/hardware", http.MethodPost, setDeviceHardwareColor)
	handleFunc(r, "/api/color/gradient/add", http.MethodPost, newDeviceGradientColor)
	handleFunc(r, "/api/color/gradient/delete", http.MethodPost, deleteDeviceGradientColor)
	handleFunc(r, "/api/color/override/update", http.MethodPost, setCommanderDuoOverride)
	handleFunc(r, "/api/hub/strip", http.MethodPost, setDeviceStrip)
	handleFunc(r, "/api/hub/linkAdapter", http.MethodPost, setDeviceLinkAdapter)
	handleFunc(r, "/api/hub/type", http.MethodPost, setExternalHubDeviceType)
	handleFunc(r, "/api/hub/amount", http.MethodPost, setExternalHubDeviceAmount)
	handleFunc(r, "/api/label", http.MethodPost, setDeviceLabel)
	handleFunc(r, "/api/lcd", http.MethodPost, setDeviceLcd)
	handleFunc(r, "/api/lcd/device", http.MethodPost, changeDeviceLcd)
	handleFunc(r, "/api/lcd/rotation", http.MethodPost, setDeviceLcdRotation)
	handleFunc(r, "/api/lcd/brightness", http.MethodPost, setDeviceLcdBrightness)
	handleFunc(r, "/api/lcd/profile", http.MethodPost, setDeviceLcdProfile)
	handleFunc(r, "/api/lcd/image", http.MethodPost, setDeviceLcdImage)
	handleFunc(r, "/api/brightness", http.MethodPost, changeBrightness)
	handleFunc(r, "/api/brightness/gradual", http.MethodPost, changeBrightnessGradual)
	handleFunc(r, "/api/position/update", http.MethodPost, changePosition)
	handleFunc(r, "/api/dashboard/update", http.MethodPost, setDashboardSettings)
	handleFunc(r, "/api/dashboard/sidebar", http.MethodPost, setDashboardSidebar)
	handleFunc(r, "/api/dashboard/devices/add", http.MethodPost, addDashboardDevice)
	handleFunc(r, "/api/argb", http.MethodPost, setARGBDevice)
	handleFunc(r, "/api/keyboard/color", http.MethodPost, setKeyboardColor)
	handleFunc(r, "/api/misc/color", http.MethodPost, setMiscColor)
	handleFunc(r, "/api/userProfile/change", http.MethodPost, changeUserProfile)
	handleFunc(r, "/api/keyboard/profile/change", http.MethodPost, changeKeyboardProfile)
	handleFunc(r, "/api/keyboard/profile/save", http.MethodPost, saveDeviceProfile)
	handleFunc(r, "/api/keyboard/layout", http.MethodPost, changeKeyboardLayout)
	handleFunc(r, "/api/keyboard/dial", http.MethodPost, changeControlDial)
	handleFunc(r, "/api/keyboard/sleep", http.MethodPost, changeSleepMode)
	handleFunc(r, "/api/keyboard/pollingRate", http.MethodPost, changePollingRate)
	handleFunc(r, "/api/keyboard/autoBrightness", http.MethodPost, changeAutoBrightness)
	handleFunc(r, "/api/keyboard/debounceTime", http.MethodPost, changeDebounceTime)
	handleFunc(r, "/api/scheduler/rgb", http.MethodPost, changeRgbScheduler)
	handleFunc(r, "/api/psu/speed", http.MethodPost, changePsuFanMode)
	handleFunc(r, "/api/mouse/dpi", http.MethodPost, saveMouseDpi)
	handleFunc(r, "/api/mouse/gestures", http.MethodPost, saveMouseGestures)
	handleFunc(r, "/api/mouse/zoneColors", http.MethodPost, saveMouseZoneColors)
	handleFunc(r, "/api/mouse/dpiColors", http.MethodPost, saveMouseDpiColors)
	handleFunc(r, "/api/mouse/sleep", http.MethodPost, changeSleepMode)
	handleFunc(r, "/api/mouse/pollingRate", http.MethodPost, changePollingRate)
	handleFunc(r, "/api/mouse/angleSnapping", http.MethodPost, changeAngleSnapping)
	handleFunc(r, "/api/mouse/rippleControl", http.MethodPost, changeRippleControl)
	handleFunc(r, "/api/mouse/motionSync", http.MethodPost, changeMotionSync)
	handleFunc(r, "/api/mouse/buttonOptimization", http.MethodPost, changeButtonOptimization)
	handleFunc(r, "/api/mouse/leftHandMode", http.MethodPost, changeLeftHandMode)
	handleFunc(r, "/api/mouse/liftHeight", http.MethodPost, changeLiftHeight)
	handleFunc(r, "/api/mouse/updateKeyAssignment", http.MethodPost, changeKeyAssignment)
	handleFunc(r, "/api/headset/updateKeyAssignment", http.MethodPost, changeKeyAssignment)
	handleFunc(r, "/api/headset/zoneColors", http.MethodPost, saveHeadsetZoneColors)
	handleFunc(r, "/api/headset/sleep", http.MethodPost, changeSleepMode)
	handleFunc(r, "/api/headset/muteIndicator", http.MethodPost, changeMuteIndicator)
	handleFunc(r, "/api/led/update", http.MethodPost, updateDeviceLed)
	handleFunc(r, "/api/macro/newValue", http.MethodPost, newMacroProfileValue)
	handleFunc(r, "/api/keyboard/getKey/", http.MethodPost, getGetKeyboardKey)
	handleFunc(r, "/api/keyboard/getKeys/", http.MethodPost, getGetKeyboardKeys)
	handleFunc(r, "/api/keyboard/updateKeyAssignment", http.MethodPost, changeKeyAssignment)
	handleFunc(r, "/api/keyboard/updateActuation", http.MethodPost, changeKeyActuation)
	handleFunc(r, "/api/keyboard/setPerformance", http.MethodPost, setKeyboardPerformance)
	handleFunc(r, "/api/keyboard/setFlashTap", http.MethodPost, setKeyboardFlashTap)
	handleFunc(r, "/api/macro/updateValue", http.MethodPost, updateMacroValue)
	handleFunc(r, "/api/macro/updateSettings", http.MethodPost, updateMacroSettings)
	handleFunc(r, "/api/keyboard/dial/setColors", http.MethodPost, setKeyboardControlDialColors)
	handleFunc(r, "/api/setSupportedDevices", http.MethodPost, setSupportedDevices)
	handleFunc(r, "/api/restore", http.MethodPost, backup.PerformRestore)
	handleFunc(r, "/api/lcd/upload", http.MethodPost, lcd.PerformImageUpload)
	handleFunc(r, "/api/headset/anc", http.MethodPost, changeActiveNoiseCancellation)
	handleFunc(r, "/api/headset/sidetone", http.MethodPost, changeSidetone)
	handleFunc(r, "/api/headset/sidetoneValue", http.MethodPost, changeSidetoneValue)
	handleFunc(r, "/api/headset/wheelOption", http.MethodPost, changeWheelOption)
	handleFunc(r, "/api/headset/equalizer", http.MethodPost, updateDeviceEqualizers)
	handleFunc(r, "/api/controller/vibration", http.MethodPost, changeControllerVibration)
	handleFunc(r, "/api/controller/zoneColors", http.MethodPost, saveControllerZoneColors)
	handleFunc(r, "/api/controller/updateKeyAssignment", http.MethodPost, changeKeyAssignment)
	handleFunc(r, "/api/controller/emulation", http.MethodPost, changeControllerEmulation)
	handleFunc(r, "/api/controller/getGraph", http.MethodPost, getControllerGraph)
	handleFunc(r, "/api/controller/setGraph", http.MethodPost, setControllerGraph)
	handleFunc(r, "/api/controller/sleep", http.MethodPost, changeControllerSleepMode)
	handleFunc(r, "/api/audio/update", http.MethodPost, setAudioSettings)
	handleFunc(r, "/api/audio/outputDevice", http.MethodPost, setAudioOutputDeviceSettings)
	handleFunc(r, "/api/devices/channel", http.MethodPost, getChannelData)
	handleFunc(r, "/api/display/update", http.MethodPost, updateDisplayData)

	// PUT
	handleFunc(r, "/api/temperatures/update", http.MethodPut, updateTemperatureProfile)
	handleFunc(r, "/api/temperatures/updateGraph", http.MethodPut, updateTemperatureProfileGraph)
	handleFunc(r, "/api/lcd/modes", http.MethodPut, updateLcdProfile)
	handleFunc(r, "/api/userProfile", http.MethodPut, saveUserProfile)
	handleFunc(r, "/api/keyboard/profile/new", http.MethodPut, saveDeviceProfile)
	handleFunc(r, "/api/macro/new", http.MethodPut, newMacroProfile)
	handleFunc(r, "/api/color/change", http.MethodPut, updateRgbProfile)
	handleFunc(r, "/api/cluster/order", http.MethodPut, updateClusterOrder)
	handleFunc(r, "/api/dashboard/devices/order", http.MethodPut, updateDashboardDeviceOrder)

	// DELETE
	handleFunc(r, "/api/keyboard/profile/delete", http.MethodDelete, deleteKeyboardProfile)
	handleFunc(r, "/api/macro/value", http.MethodDelete, deleteMacroValue)
	handleFunc(r, "/api/temperatures/delete", http.MethodDelete, deleteTemperatureProfile)
	handleFunc(r, "/api/macro/profile", http.MethodDelete, deleteMacroProfile)
	handleFunc(r, "/api/userProfile/delete", http.MethodDelete, deleteUserProfile)
	handleFunc(r, "/api/dashboard/devices/delete", http.MethodDelete, removeDashboardDevice)

	// Prometheus metrics
	if config.GetConfig().Metrics {
		handleFunc(r, "/api/metrics", http.MethodGet, getDeviceMetrics)
	}

	if config.GetConfig().Frontend {
		handleFunc(r, "/", http.MethodGet, uiIndex)
		handleFunc(r, "/devices", http.MethodGet, uiDevices)
		handleFunc(r, "/device/", http.MethodGet, uiDeviceOverview)
		handleFunc(r, "/temperature", http.MethodGet, uiTemperatureOverview)
		handleFunc(r, "/temperatureGraphs", http.MethodGet, uiTemperatureGraphOverview)
		handleFunc(r, "/color", http.MethodGet, uiColorOverview)
		handleFunc(r, "/scheduler", http.MethodGet, uiSchedulerOverview)
		handleFunc(r, "/rgb", http.MethodGet, uiRgbEditor)
		handleFunc(r, "/rgbCluster", http.MethodGet, uiRgbCluster)
		handleFunc(r, "/macros", http.MethodGet, uiMacrosOverview)
		handleFunc(r, "/lcd", http.MethodGet, uiLcdOverview)
		handleFunc(r, "/settings", http.MethodGet, uiSettings)
		//handleFunc(r, "/xeneon", http.MethodGet, uiXeneon)
	}
	return protection.wrap(r)
}

// Init will start a new web server used for metrics and fan control
func Init() {
	headers = []Header{
		{
			Key:   "Cache-Control",
			Value: "no-cache, no-store, must-revalidate",
		},
		{
			Key:   "Pragma",
			Value: "no-cache",
		},
		{
			Key:   "Expires",
			Value: "0",
		},
	}

	if config.GetConfig().ListenPort > 0 {
		templates.Init()
		address := httpListenAddress(config.GetConfig())
		srv := &http.Server{
			Addr:    address,
			Handler: setRoutes(),
		}
		serverMutex.Lock()
		server = srv
		serveDone = make(chan struct{})
		done := serveDone
		serverMutex.Unlock()

		fmt.Println(
			fmt.Sprintf("[Server] Running REST and WebUI on %s. WebUI is accessible via: http://%s",
				srv.Addr,
				srv.Addr,
			),
		)
		go func() {
			defer close(done)
			err := srv.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Log(logger.Fields{"error": err}).Error("Unable to run REST server")
				lifecycle.Request(1)
			}
		}()
	} else {
		logger.Log(logger.Fields{}).Info("REST server is disabled")
	}
}

func httpListenAddress(cfg config.Configuration) string {
	return localnetwork.Address(cfg.ListenPort)
}

// Shutdown stops accepting requests and waits for active requests until ctx expires.
func Shutdown(ctx context.Context) error {
	serverMutex.Lock()
	srv := server
	done := serveDone
	serverMutex.Unlock()
	if srv == nil {
		return nil
	}
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
