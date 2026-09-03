package server

// Package: server
// Author: Nikola Jurkovic
// License: GPL-3.0 or later

import (
	"LumenForge/src/audio"
	"LumenForge/src/backup"
	"LumenForge/src/buttonspresentation"
	"LumenForge/src/cluster"
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/coolingpresentation"
	"LumenForge/src/dashboard"
	"LumenForge/src/deviceprofilepresentation"
	"LumenForge/src/devices"
	"LumenForge/src/devices/lcd"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/display"
	"LumenForge/src/displaypresentation"
	"LumenForge/src/dpipresentation"
	"LumenForge/src/externalsources"
	"LumenForge/src/inputmanager"
	"LumenForge/src/keyboardassignmentspresentation"
	"LumenForge/src/language"
	"LumenForge/src/lifecycle"
	"LumenForge/src/lightingpresentation"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/localnetwork"
	"LumenForge/src/logger"
	"LumenForge/src/macro"
	"LumenForge/src/media"
	"LumenForge/src/memorypresentation"
	"LumenForge/src/metrics"
	"LumenForge/src/openrgb"
	"LumenForge/src/performancepresentation"
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
	"reflect"
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
	Telemetry interface{} `json:"telemetry,omitempty"`
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
	dashboardLayoutRequestLimit    = 64 << 10
)

type nativeDeviceLightingTarget interface {
	LightingDeviceID() string
	SupportsLightingEffect(string) bool
	SetLightingEffect(string) error
	SetLightingBrightness(uint8) error
	ResolveLightingEffectSettings(string) (lightingsettings.EffectSettings, error)
	SetLightingEffectSettings(string, lightingsettings.EffectSettings) error
	ResetLightingEffectSettings(string) error
}

type nativeDeviceLightingChannelTarget interface {
	SetLightingChannelEffect(string, string) error
}

type nativeDeviceLightingAllChannelTarget interface {
	SetLightingAllChannelEffects(string) error
}

type nativeDeviceLightingChannelSettingsTarget interface {
	ResolveLightingChannelEffectSettings(string, string) (lightingsettings.EffectSettings, error)
	SetLightingChannelEffectSettings(string, string, lightingsettings.EffectSettings) error
	ResetLightingChannelEffectSettings(string, string) error
}

type nativeDeviceLightingIndexedColorTarget interface {
	SetLightingIndexedColor(string, int, lightingsettings.Color) error
}

type nativeDeviceLightingIndexedColorsTarget interface {
	SetLightingIndexedColors(string, []lightingsettings.IndexedColor) error
}

type nativeDeviceAuthoredZoneLightingTarget interface {
	nativeDeviceLightingTarget
	SetLightingZoneColor(string, string, string, string, rgb.Color) error
}

type nativeDeviceAuthoredZoneLightingMultiTarget interface {
	SetLightingZoneColors(string, []string, rgb.Color) error
}

type devicesDPIWorkspaceTarget interface {
	DPIDeviceID() string
	DPISnapshot() (dpipresentation.Snapshot, bool)
	SelectMouseDPIStage(int) uint8
	SetMouseSniperMode(bool) uint8
	SaveMouseDPISettings(map[int]uint16, map[int]rgb.Color) uint8
}

var lookupNativeDeviceLightingWrapper = devices.LookupDevice
var lookupDevicesDPIWorkspaceWrapper = devices.LookupDevice

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
	temperature := temperatures.GetCpuTemperature()
	resp := &Response{
		Code:      http.StatusOK,
		Status:    1,
		Data:      dashboard.GetDashboard().TemperatureToString(temperature),
		Telemetry: temperature,
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
	telemetry := make(map[int]float32)
	for key, val := range systeminfo.GetInfo().GPU {
		temperature := temperatures.GetGpuTemperatureIndex(val.Index)
		data[key] = dashboard.GetDashboard().TemperatureToString(temperature)
		telemetry[key] = temperature
	}
	resp := &Response{
		Code:      http.StatusOK,
		Status:    1,
		Data:      data,
		Telemetry: telemetry,
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
		Effect                     string `json:"effect"`
		Brightness                 int    `json:"brightness"`
		ClusteredLightingDevices   int    `json:"clusteredLightingDevices"`
		IndependentLightingDevices int    `json:"independentLightingDevices"`
	}

	effect := "off"
	brightness := 0
	snapshot, _ := getRGBClusterLightingStatus()
	if snapshot.Available {
		effect = snapshot.SelectedEffect
		brightness = int(snapshot.Brightness)
	}
	if effect == "" {
		effect = "off"
	}

	counts := dashboardLightingDeviceCounts(devices.GetDevices())

	res := lightingResponse{
		Effect:                     effect,
		Brightness:                 brightness,
		ClusteredLightingDevices:   counts.Clustered,
		IndependentLightingDevices: counts.Independent,
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

type dashboardLightingCounts struct {
	Clustered   int
	Independent int
}

// dashboardLightingDeviceCounts classifies connected top-level lighting devices
// once, using their shared presentation ownership state.
func dashboardLightingDeviceCounts(connected map[string]*common.Device) dashboardLightingCounts {
	seen := make(map[string]struct{}, len(connected))
	counts := dashboardLightingCounts{}
	for serial, device := range connected {
		if device == nil || device.Unavailable || serial == "cluster" || device.Serial == "cluster" || device.ProductType == common.ProductTypeCluster {
			continue
		}
		provider, ok := device.Instance.(devicesLightingSnapshotProvider)
		if !ok {
			continue
		}
		snapshot, available := provider.LightingSnapshot()
		if !available {
			continue
		}
		identity := provider.LightingDeviceID()
		if identity == "" {
			identity = serial
		}
		if _, duplicate := seen[identity]; duplicate {
			continue
		}
		seen[identity] = struct{}{}
		if snapshot.ClusterControlled {
			counts.Clustered++
		} else {
			counts.Independent++
		}
	}
	return counts
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

// dashboardDeviceSummary is the small read-only presentation used by the
// Dashboard. It deliberately derives its state from the existing Devices
// workspace adapters rather than the legacy Dashboard membership list.
type dashboardDeviceSummary struct {
	Serial     string                     `json:"serial"`
	Name       string                     `json:"name"`
	Product    string                     `json:"product,omitempty"`
	Lighting   string                     `json:"lighting,omitempty"`
	Brightness *uint8                     `json:"brightness,omitempty"`
	StatusRows []dashboardDeviceStatusRow `json:"statusRows,omitempty"`
	Telemetry  *dashboardDeviceTelemetry  `json:"telemetry,omitempty"`
}

type dashboardDeviceTelemetry struct {
	AverageFanRPM     *int                                 `json:"averageFanRPM,omitempty"`
	CoolantCelsius    *float32                             `json:"coolantCelsius,omitempty"`
	TemperatureProbes []dashboardTemperatureProbeTelemetry `json:"temperatureProbes,omitempty"`
}

type dashboardTemperatureProbeTelemetry struct {
	ID      int     `json:"id"`
	Label   string  `json:"label"`
	Celsius float32 `json:"celsius"`
}

type dashboardDeviceStatusRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type dashboardDevicesCurrentResponse struct {
	Native  []dashboardDeviceSummary     `json:"native"`
	OpenRGB []dashboardDeviceSummary     `json:"openrgb"`
	Memory  []dashboardMemoryTemperature `json:"memory"`
}

type dashboardMemoryTemperature struct {
	Serial      string  `json:"serial"`
	ChannelID   int     `json:"channelId"`
	Identifier  string  `json:"identifier"`
	Name        string  `json:"name"`
	Temperature string  `json:"temperature"`
	Celsius     float32 `json:"celsius"`
}

// dashboardDeviceLabel reads the device-level label maintained by the
// authoritative hardware profile. It intentionally does not read RGB effect
// fields; lighting state comes from the shared lighting presentation instead.
func dashboardDeviceLabel(device *common.Device) string {
	if device == nil || device.Instance == nil {
		return ""
	}
	value := reflect.ValueOf(device.Instance)
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return ""
	}
	profile := value.FieldByName("DeviceProfile")
	if !profile.IsValid() || profile.Kind() != reflect.Ptr || profile.IsNil() {
		return ""
	}
	profile = profile.Elem()
	if profile.Kind() != reflect.Struct {
		return ""
	}
	label := profile.FieldByName("Label")
	if !label.IsValid() || label.Kind() != reflect.String {
		return ""
	}
	return strings.TrimSpace(label.String())
}

func dashboardLightingState(lighting *devicesLightingWorkspaceSummary) string {
	if lighting == nil {
		return ""
	}
	if lighting.ClusterControlled {
		return "Cluster"
	}
	if lighting.ExternalControlled {
		return "OpenRGB controls this device"
	}
	if lighting.BulkEffectControl != nil {
		if lighting.BulkEffectControl.Mixed {
			return "Mixed"
		}
		if lighting.BulkEffectControl.ConfiguredEffectLabel != "" {
			return lighting.BulkEffectControl.ConfiguredEffectLabel
		}
	}
	if lighting.ConfiguredEffectLabel != "" {
		return lighting.ConfiguredEffectLabel
	}
	return devicesLightingEffectDisplayLabel(lighting.ConfiguredEffect, "")
}

func dashboardDeviceStatus(summary *devicesWorkspaceSummary) []dashboardDeviceStatusRow {
	rows := make([]dashboardDeviceStatusRow, 0)
	if summary == nil {
		return rows
	}
	if summary.OverviewCooling != nil {
		for _, pump := range summary.OverviewCooling.Pumps {
			value := strings.Trim(strings.Join([]string{pump.RPM, pump.Temperature}, " · "), " ·")
			if value != "" {
				rows = append(rows, dashboardDeviceStatusRow{Label: pump.Label, Value: value})
			}
		}
		for _, fan := range summary.OverviewCooling.Fans {
			if fan.Value != "" && fan.Value != "0 RPM" {
				rows = append(rows, dashboardDeviceStatusRow{Label: fan.Label, Value: fan.Value})
			}
		}
	}
	for _, probe := range summary.TemperatureProbes {
		if probe.Value != "" {
			rows = append(rows, dashboardDeviceStatusRow{Label: probe.Label, Value: probe.Value})
		}
	}
	if len(rows) == 0 && summary.OverviewPerformance != nil {
		for _, row := range summary.OverviewPerformance.Rows {
			if row.Label != "" && row.Value != "" {
				rows = append(rows, dashboardDeviceStatusRow{Label: row.Label, Value: row.Value})
			}
		}
	}
	return rows
}

func dashboardCoolingTelemetry(snapshot coolingpresentation.Snapshot) *dashboardDeviceTelemetry {
	telemetry := &dashboardDeviceTelemetry{}
	var fanTotal, fanCount int
	for _, channel := range snapshot.Channels {
		if !channel.ContainsPump && channel.RPM > 0 {
			fanTotal += int(channel.RPM)
			fanCount++
		}
		if channel.ContainsPump && channel.Celsius != nil && *channel.Celsius > 0 && telemetry.CoolantCelsius == nil {
			value := *channel.Celsius
			telemetry.CoolantCelsius = &value
		}
	}
	if fanCount > 0 {
		average := int(math.Round(float64(fanTotal) / float64(fanCount)))
		telemetry.AverageFanRPM = &average
	}
	for _, probe := range snapshot.TemperatureProbes {
		if probe.Celsius == nil || *probe.Celsius <= 0 {
			continue
		}
		telemetry.TemperatureProbes = append(telemetry.TemperatureProbes, dashboardTemperatureProbeTelemetry{ID: probe.ID, Label: devicesOverviewCoolingLabel(probe.Label, probe.Name), Celsius: *probe.Celsius})
	}
	if telemetry.AverageFanRPM == nil && telemetry.CoolantCelsius == nil && len(telemetry.TemperatureProbes) == 0 {
		return nil
	}
	return telemetry
}

func dashboardMemoryModuleName(module devicesMemoryModuleSummary) string {
	name := strings.TrimSpace(module.Name)
	if name != "" && !strings.HasPrefix(strings.ToUpper(name), "DIMM ") {
		return name
	}
	if label := strings.TrimSpace(module.Label); label != "" {
		return label
	}
	return name
}

func dashboardMemoryTemperatures(serial string, summary *devicesMemoryWorkspaceSummary) []dashboardMemoryTemperature {
	if summary == nil {
		return nil
	}
	items := make([]dashboardMemoryTemperature, 0, len(summary.Modules))
	for _, module := range summary.Modules {
		if module.Temperature == "" || module.TemperatureCelsius <= 0 {
			continue
		}
		items = append(items, dashboardMemoryTemperature{
			Serial: serial, ChannelID: module.ChannelID, Identifier: fmt.Sprintf("DIMM %d", module.ChannelID+1),
			Name: dashboardMemoryModuleName(module), Temperature: module.Temperature, Celsius: module.TemperatureCelsius,
		})
	}
	return items
}

func dashboardCurrentDevices(connected map[string]*common.Device, battery map[string]stats.BatteryStats) dashboardDevicesCurrentResponse {
	response := dashboardDevicesCurrentResponse{
		Native:  make([]dashboardDeviceSummary, 0),
		OpenRGB: make([]dashboardDeviceSummary, 0),
		Memory:  make([]dashboardMemoryTemperature, 0),
	}
	serials := make([]string, 0, len(connected))
	for serial := range connected {
		serials = append(serials, serial)
	}
	sort.Strings(serials)
	for _, serial := range serials {
		device := connected[serial]
		if device == nil || device.Hidden || device.Unavailable || serial == "cluster" || device.Serial == "cluster" || device.ProductType == common.ProductTypeCluster {
			continue
		}
		summary, present := devicesWorkspaceSummaryForSerial(connected, battery, serial)
		if !present {
			continue
		}
		isOpenRGB := summary.OpenRGB != nil || (summary.Lighting != nil && summary.Lighting.TargetKind == "openrgb")
		meaningful := isOpenRGB || summary.Lighting != nil || summary.Memory != nil || summary.OverviewCooling != nil || summary.OverviewPerformance != nil || summary.OverviewDisplay != nil || summary.HasBattery
		if !meaningful {
			continue
		}
		name := dashboardDeviceLabel(device)
		if name == "" {
			name = summary.Product
		}
		item := dashboardDeviceSummary{Serial: summary.Serial, Name: name, Product: summary.Product, Lighting: dashboardLightingState(summary.Lighting), StatusRows: dashboardDeviceStatus(summary)}
		if coolingDevice, ok := device.Instance.(devicesCoolingSnapshotProvider); ok && coolingDevice != nil && coolingDevice.CoolingDeviceID() == serial {
			if snapshot, usable := coolingDevice.CoolingSnapshot(); usable {
				item.Telemetry = dashboardCoolingTelemetry(snapshot)
			}
		}
		if summary.Lighting != nil && summary.Lighting.HasBrightness {
			brightness := summary.Lighting.Brightness
			item.Brightness = &brightness
		}
		response.Memory = append(response.Memory, dashboardMemoryTemperatures(summary.Serial, summary.Memory)...)
		if isOpenRGB {
			response.OpenRGB = append(response.OpenRGB, item)
		} else {
			response.Native = append(response.Native, item)
		}
	}
	return response
}

// getDashboardCurrentDevices returns current top-level Devices presentation,
// not the legacy dashboard.Devices membership setting.
func getDashboardCurrentDevices(w http.ResponseWriter, _ *http.Request) {
	response := dashboardCurrentDevices(devices.GetDevices(), stats.GetBatteryStats())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func getDashboardLayout(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Layout []dashboard.LayoutItem `json:"layout"`
	}{Layout: dashboard.GetDashboardLayout()})
}

func updateDashboardLayout(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Layout []dashboard.LayoutItem `json:"layout"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, dashboardLayoutRequestLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || request.Layout == nil || decoder.Decode(&struct{}{}) != io.EOF {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Invalid dashboard layout"}).Send(w)
		return
	}
	status, layout := dashboard.UpdateDashboardLayout(request.Layout)
	if status == 0 {
		(&Response{Code: http.StatusOK, Status: 0, Message: "Unable to save dashboard layout"}).Send(w)
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Data: struct {
		Layout []dashboard.LayoutItem `json:"layout"`
	}{Layout: layout}}).Send(w)
}

func dashboardLayoutRoute(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getDashboardLayout(w, r)
	case http.MethodPut:
		updateDashboardLayout(w, r)
	default:
		http.Error(w, language.GetValue("txtMethodNotAllowed"), http.StatusMethodNotAllowed)
	}
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
	batteryStats := stats.GetBatteryStats()
	web := templates.Web{}
	web.Title = dashboard.GetDashboard().PageTitle
	web.Devices = deviceList
	web.BuildInfo = version.GetBuildInfo()
	web.SystemInfo = systeminfo.GetInfo()
	web.CpuTempCelsius = temperatures.GetCpuTemperature()
	web.CpuTemp = dashboard.GetDashboard().TemperatureToString(web.CpuTempCelsius)
	web.GpuTemp = dashboard.GetDashboard().TemperatureToString(temperatures.GetGpuTemperature())
	web.Dashboard = dashboard.GetDashboard()
	web.BatteryStats = batteryStats
	web.DashboardMemory = dashboardCurrentDevices(deviceList, batteryStats).Memory
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
	GPUTemperature         string
	Zones                  []openRGBWorkspaceZoneSummary
}

type devicesWorkspaceSummary struct {
	Product             string
	Serial              string
	Firmware            string
	Image               string
	Unavailable         bool
	HasBattery          bool
	BatteryLevel        uint16
	OpenRGB             *openRGBWorkspaceSummary
	Lighting            *devicesLightingWorkspaceSummary
	DPI                 *devicesDPIWorkspaceSummary
	Performance         *devicesPerformanceWorkspaceSummary
	Buttons             *devicesButtonsWorkspaceSummary
	DeviceProfiles      *devicesDeviceProfileWorkspaceSummary
	Cooling             *devicesCoolingWorkspaceSummary
	Display             *devicesDisplayWorkspaceSummary
	Memory              *devicesMemoryWorkspaceSummary
	OverviewCooling     *devicesOverviewCoolingStatusSummary
	TemperatureProbes   []devicesOverviewStatusRow
	OverviewPerformance *devicesOverviewPerformanceStatusSummary
	OverviewDisplay     *devicesOverviewDisplayStatusSummary
	KeyboardAssignments *devicesKeyboardAssignmentsWorkspaceSummary
	LegacyLighting      bool
	View                string
}

// devicesLightingWorkspaceSummary is the presentation model shared by device
// implementations which expose canonical lighting in the Devices workspace.
type devicesLightingEffectSummary struct {
	ID       string
	Label    string
	Selected bool
}

type devicesLightingWorkspaceSummary struct {
	TargetKind              string
	ConfiguredEffect        string
	ConfiguredEffectLabel   string
	ConfiguredEffectIconURL string
	EffectSupported         bool
	SupportedEffects        []devicesLightingEffectSummary
	HasBrightness           bool
	Brightness              uint8
	HasSpeedControl         bool
	Speed                   string
	ClusterControlled       bool
	ExternalControlled      bool
	ReadOnly                bool
	PaletteKind             string
	SingleColorHex          string
	TwoColorStartHex        string
	TwoColorEndHex          string
	HasTemperature          bool
	TemperatureLow          devicesLightingTemperaturePointSummary
	TemperatureMiddle       devicesLightingTemperaturePointSummary
	TemperatureHigh         devicesLightingTemperaturePointSummary
	TemperaturePoints       []devicesLightingTemperaturePointSummary
	HasGradient             bool
	GradientStops           []devicesLightingGradientStopSummary
	Customized              bool
	AuthoredZoneEditor      *devicesLightingAuthoredZoneEditorSummary
	ThreePinPort            *devicesLightingThreePinPortSummary
	ManualRGBPorts          []devicesLightingManualRGBPortSummary
	IndexedColors           []devicesLightingIndexedColorSummary
	Channels                []devicesLightingChannelSummary
	BulkEffectControl       *devicesLightingBulkEffectControlSummary
}

type devicesLightingBulkEffectControlSummary struct {
	ConfiguredEffect        string
	ConfiguredEffectLabel   string
	ConfiguredEffectIconURL string
	Mixed                   bool
	SupportedEffects        []devicesLightingEffectSummary
}

type devicesLightingIndexedColorSummary struct {
	Index           int
	Label, ColorHex string
}

type devicesLightingManualRGBPortSummary struct {
	PortID  string
	Name    string
	Options []devicesLightingThreePinOptionSummary
}

type devicesLightingThreePinPortSummary struct {
	DeviceOptions    []devicesLightingThreePinOptionSummary
	QuantityOptions  []devicesLightingThreePinOptionSummary
	QuantityDisabled bool
}

type devicesLightingThreePinOptionSummary struct {
	Value, Label string
	Selected     bool
}

type devicesLightingChannelSummary struct {
	TargetID, ChannelID, Name, Label string
	LEDCount                         int
	ContainsPump                     bool
	Lighting                         *devicesLightingWorkspaceSummary
	ProbeTemperature                 *devicesLightingProbeTemperatureSummary
}

type devicesLightingProbeTemperatureSummary struct {
	ProbeID, Minimum, Maximum string
	Sources                   []devicesLightingProbeTemperatureSourceSummary
}

type devicesLightingProbeTemperatureSourceSummary struct {
	ID, Label string
	Selected  bool
}

type devicesLightingAuthoredZoneEditorSummary struct {
	Heading, Description      string
	EffectID                  string
	HasGroups, HasGeometry    bool
	LayoutWidth, LayoutHeight int
	Zones                     []devicesLightingAuthoredZoneSummary
}

type devicesLightingAuthoredZoneSummary struct {
	ID, Label, ColorHex, GroupID, GroupLabel string
	HasGeometry                              bool
	Left, Top, Width, Height                 int
}

type devicesLightingTemperaturePointSummary struct {
	Role     string
	Label    string
	ColorHex string
	Celsius  string
}

type devicesLightingGradientStopSummary struct {
	Number    int
	Position  string
	ColorHex  string
	Intensity string
}

// Compatibility aliases keep older server-local presentation users on the
// generic Devices lighting model without restoring duplicate DTOs.
type openRGBLightingWorkspaceSummary = devicesLightingWorkspaceSummary
type openRGBLightingTemperaturePointSummary = devicesLightingTemperaturePointSummary
type openRGBLightingGradientStopSummary = devicesLightingGradientStopSummary

type devicesLightingSnapshotProvider interface {
	LightingDeviceID() string
	LightingSnapshot() (lightingpresentation.Snapshot, bool)
}

// devicesDPISnapshotProvider is implemented only by devices that can expose
// their existing DPI state to the read-only Devices workspace.
type devicesDPISnapshotProvider interface {
	DPIDeviceID() string
	DPISnapshot() (dpipresentation.Snapshot, bool)
}

// devicesPerformanceSnapshotProvider is implemented only by devices that
// expose their existing performance settings to the Devices workspace.
type devicesPerformanceSnapshotProvider interface {
	PerformanceDeviceID() string
	PerformanceSnapshot() (performancepresentation.Snapshot, bool)
}

// devicesButtonsSnapshotProvider is implemented only by devices that expose
// their existing assignment state to the Devices workspace.
type devicesButtonsSnapshotProvider interface {
	ButtonsDeviceID() string
	ButtonsSnapshot() (buttonspresentation.Snapshot, bool)
}

type devicesKeyboardAssignmentsSnapshotProvider interface {
	KeyboardAssignmentsDeviceID() string
	KeyboardAssignmentsSnapshot() (keyboardassignmentspresentation.Snapshot, bool)
}

type devicesDeviceProfileSnapshotProvider interface {
	DeviceProfileDeviceID() string
	DeviceProfileSnapshot() (deviceprofilepresentation.Snapshot, bool)
}

type devicesCoolingSnapshotProvider interface {
	CoolingDeviceID() string
	CoolingSnapshot() (coolingpresentation.Snapshot, bool)
}

type devicesDisplaySnapshotProvider interface {
	DisplayDeviceID() string
	DisplaySnapshot() (displaypresentation.Snapshot, bool)
}

type devicesMemorySnapshotProvider interface {
	MemoryDeviceID() string
	MemorySnapshot() (memorypresentation.Snapshot, bool)
}

type devicesDeviceProfileWorkspaceSummary struct {
	Profiles                                 []string
	ProfileDisplayLabels                     map[string]string
	ActiveProfile, Scope, Label, Description string
}

type devicesMemoryModuleSummary struct {
	ChannelID          int
	Name               string
	Label              string
	MemoryType         int
	SKU                string
	LEDCount           uint8
	Temperature        string
	TemperatureCelsius float32
}

type devicesMemoryWorkspaceSummary struct {
	Modules []devicesMemoryModuleSummary
}

type devicesCoolingProfileOptionSummary struct {
	ID, Label string
}

type devicesCoolingChannelSummary struct {
	ID                                        int
	Name, Label, Temperature, SelectedProfile string
	ContainsPump                              bool
	RPM                                       int16
}

type devicesCoolingTemperatureProbeSummary struct {
	ID                       int
	Name, Label, Temperature string
}

type devicesCoolingWorkspaceSummary struct {
	Channels          []devicesCoolingChannelSummary
	ProfileOptions    []devicesCoolingProfileOptionSummary
	TemperatureProbes []devicesCoolingTemperatureProbeSummary
}

// devicesOverviewStatusRow is a compact read-only fact for the Overview page.
// It is deliberately derived from an existing capability summary rather than
// becoming a separate device telemetry model.
type devicesOverviewStatusRow struct {
	ChannelID int
	Label     string
	Value     string
	Telemetry bool
}

type devicesOverviewCoolingPumpSummary struct {
	ChannelID   int
	Label       string
	RPM         string
	Temperature string
}

type devicesOverviewCoolingStatusSummary struct {
	Pumps []devicesOverviewCoolingPumpSummary
	Fans  []devicesOverviewStatusRow
}

type devicesOverviewPerformanceStatusSummary struct {
	Rows []devicesOverviewStatusRow
}

type devicesOverviewDisplayStatusSummary struct {
	Rows []devicesOverviewStatusRow
}

type devicesDisplayOptionSummary struct {
	ID       int
	Label    string
	Selected bool
}

type devicesDisplayImageSummary struct {
	Name     string
	Selected bool
}

type devicesDisplayWorkspaceSummary struct {
	ChannelID        int
	Modes            []devicesDisplayOptionSummary
	Rotations        []devicesDisplayOptionSummary
	BrightnessLevels []devicesDisplayOptionSummary
	Images           []devicesDisplayImageSummary
	ImageMode        bool
	ImageModeID      int
}

const (
	devicesKeyboardDeviceProfileDescription      = "Saves the complete keyboard configuration, including settings, lockouts, colors, assignments, and presets."
	devicesScimitarEliteDeviceProfileDescription = "Saves the complete mouse configuration, including performance, DPI, assignments, and lighting."
	devicesCCXTDeviceProfileDescription          = "Saves the complete controller configuration, including cooling, labels, lighting, connected-device settings, and optional display settings."
	devicesGenericDeviceProfileDescription       = "Save or switch a complete device configuration. Device profiles include supported settings across available workspaces, such as performance, assignments, lighting, layouts, and other device-wide controls for this hardware."
	devicesLightingProfileDescription            = "Saves your custom mousepad lighting layout and colors."
)

func devicesDeviceProfilePresentation(device *common.Device, hasKeyboardAssignments bool, scope string) (string, string) {
	if scope == deviceprofilepresentation.ScopeLighting {
		return "Lighting Profile", devicesLightingProfileDescription
	}
	if hasKeyboardAssignments {
		return "Device Profile", devicesKeyboardDeviceProfileDescription
	}
	if device != nil && device.ProductType == common.ProductTypeScimitarRgbElite {
		return "Device Profile", devicesScimitarEliteDeviceProfileDescription
	}
	if device != nil && (device.ProductType == common.ProductTypeCC || device.ProductType == common.ProductTypeCCXT) {
		return "Device Profile", devicesCCXTDeviceProfileDescription
	}
	return "Device Profile", devicesGenericDeviceProfileDescription
}

func devicesCoolingWorkspaceSummaryFromSnapshot(snapshot coolingpresentation.Snapshot) *devicesCoolingWorkspaceSummary {
	if !snapshot.Available || len(snapshot.Channels) == 0 || len(snapshot.ProfileOptions) == 0 {
		return nil
	}
	summary := &devicesCoolingWorkspaceSummary{}
	for _, option := range snapshot.ProfileOptions {
		if option.ID == "" || option.Label == "" {
			return nil
		}
		summary.ProfileOptions = append(summary.ProfileOptions, devicesCoolingProfileOptionSummary{ID: option.ID, Label: option.Label})
	}
	for _, channel := range snapshot.Channels {
		if channel.ID < 0 || channel.Name == "" || channel.SelectedProfile == "" {
			return nil
		}
		summary.Channels = append(summary.Channels, devicesCoolingChannelSummary{ID: channel.ID, Name: channel.Name, Label: channel.Label, RPM: channel.RPM, Temperature: channel.Temperature, ContainsPump: channel.ContainsPump, SelectedProfile: channel.SelectedProfile})
	}
	for _, probe := range snapshot.TemperatureProbes {
		if probe.ID < 0 || probe.Name == "" {
			return nil
		}
		summary.TemperatureProbes = append(summary.TemperatureProbes, devicesCoolingTemperatureProbeSummary{ID: probe.ID, Name: probe.Name, Label: probe.Label, Temperature: probe.Temperature})
	}
	return summary
}

func devicesDisplayWorkspaceSummaryFromSnapshot(snapshot displaypresentation.Snapshot) *devicesDisplayWorkspaceSummary {
	if !snapshot.Available || len(snapshot.Modes) == 0 || len(snapshot.Rotations) == 0 || len(snapshot.BrightnessLevels) == 0 {
		return nil
	}
	summary := &devicesDisplayWorkspaceSummary{ChannelID: snapshot.ChannelID, ImageMode: snapshot.ImageMode, ImageModeID: snapshot.ImageModeID}
	convertOptions := func(options []displaypresentation.Option) []devicesDisplayOptionSummary {
		converted := make([]devicesDisplayOptionSummary, 0, len(options))
		for _, option := range options {
			if option.Label == "" {
				return nil
			}
			converted = append(converted, devicesDisplayOptionSummary{ID: option.ID, Label: option.Label, Selected: option.Selected})
		}
		return converted
	}
	if summary.Modes = convertOptions(snapshot.Modes); summary.Modes == nil {
		return nil
	}
	if summary.Rotations = convertOptions(snapshot.Rotations); summary.Rotations == nil {
		return nil
	}
	if summary.BrightnessLevels = convertOptions(snapshot.BrightnessLevels); summary.BrightnessLevels == nil {
		return nil
	}
	for _, image := range snapshot.Images {
		if image.Name == "" {
			return nil
		}
		summary.Images = append(summary.Images, devicesDisplayImageSummary{Name: image.Name, Selected: image.Selected})
	}
	return summary
}

func devicesOverviewCoolingStatusFromSummary(summary *devicesCoolingWorkspaceSummary) *devicesOverviewCoolingStatusSummary {
	if summary == nil {
		return nil
	}

	status := &devicesOverviewCoolingStatusSummary{}
	for _, channel := range summary.Channels {
		if channel.ContainsPump {
			pump := devicesOverviewCoolingPumpSummary{ChannelID: channel.ID}
			if channel.RPM >= 0 {
				pump.RPM = fmt.Sprintf("%d RPM", channel.RPM)
			}
			pump.Temperature = strings.TrimSpace(channel.Temperature)
			if pump.RPM != "" || pump.Temperature != "" {
				pump.Label = devicesOverviewCoolingLabel(channel.Label, channel.Name)
				status.Pumps = append(status.Pumps, pump)
			}
			continue
		}
		if channel.RPM < 0 {
			continue
		}
		label := devicesOverviewCoolingLabel(channel.Label, channel.Name)
		status.Fans = append(status.Fans, devicesOverviewStatusRow{ChannelID: channel.ID, Label: label, Value: fmt.Sprintf("%d RPM", channel.RPM), Telemetry: true})
	}
	if len(status.Pumps) == 0 && len(status.Fans) == 0 {
		return nil
	}
	return status
}

func devicesOverviewCoolingLabel(label, fallback string) string {
	label = strings.TrimSpace(label)
	if label == "" || strings.EqualFold(label, "Set Label") {
		return strings.TrimSpace(fallback)
	}
	return label
}

// devicesOverviewCoolingStatusFromSnapshot retains read-only controller
// telemetry even when the Devices cooling workspace cannot expose controls
// because profile metadata is unavailable.
func devicesOverviewCoolingStatusFromSnapshot(snapshot coolingpresentation.Snapshot) *devicesOverviewCoolingStatusSummary {
	if !snapshot.Available {
		return nil
	}
	status := &devicesOverviewCoolingStatusSummary{}
	for _, channel := range snapshot.Channels {
		if channel.RPM <= 0 {
			continue
		}
		if channel.ContainsPump {
			status.Pumps = append(status.Pumps, devicesOverviewCoolingPumpSummary{
				ChannelID:   channel.ID,
				Label:       devicesOverviewCoolingLabel(channel.Label, channel.Name),
				RPM:         fmt.Sprintf("%d RPM", channel.RPM),
				Temperature: strings.TrimSpace(channel.Temperature),
			})
			continue
		}
		status.Fans = append(status.Fans, devicesOverviewStatusRow{
			ChannelID: channel.ID,
			Label:     devicesOverviewCoolingLabel(channel.Label, channel.Name),
			Value:     fmt.Sprintf("%d RPM", channel.RPM),
			Telemetry: true,
		})
	}
	if len(status.Pumps) == 0 && len(status.Fans) == 0 {
		return nil
	}
	return status
}

func devicesOverviewTemperatureProbesFromSummary(summary *devicesCoolingWorkspaceSummary) []devicesOverviewStatusRow {
	if summary == nil {
		return nil
	}
	probes := make([]devicesOverviewStatusRow, 0, len(summary.TemperatureProbes))
	for _, probe := range summary.TemperatureProbes {
		temperature := strings.TrimSpace(probe.Temperature)
		if temperature == "" {
			continue
		}
		label := devicesOverviewCoolingLabel(probe.Label, probe.Name)
		probes = append(probes, devicesOverviewStatusRow{ChannelID: probe.ID, Label: label, Value: temperature, Telemetry: true})
	}
	if len(probes) == 0 {
		return nil
	}
	return probes
}

func devicesOverviewTemperatureProbesFromSnapshot(snapshot coolingpresentation.Snapshot) []devicesOverviewStatusRow {
	if !snapshot.Available {
		return nil
	}
	probes := make([]devicesOverviewStatusRow, 0, len(snapshot.TemperatureProbes))
	for _, probe := range snapshot.TemperatureProbes {
		temperature := strings.TrimSpace(probe.Temperature)
		if temperature == "" {
			continue
		}
		probes = append(probes, devicesOverviewStatusRow{
			ChannelID: probe.ID,
			Label:     devicesOverviewCoolingLabel(probe.Label, probe.Name),
			Value:     temperature,
			Telemetry: true,
		})
	}
	if len(probes) == 0 {
		return nil
	}
	return probes
}

func devicesOverviewPerformanceStatusFromSummaries(dpi *devicesDPIWorkspaceSummary, performance *devicesPerformanceWorkspaceSummary) *devicesOverviewPerformanceStatusSummary {
	status := &devicesOverviewPerformanceStatusSummary{}
	if dpi != nil {
		for _, stage := range dpi.RegularStages {
			if stage.ID != dpi.ActiveRegularStageID {
				continue
			}
			status.Rows = append(status.Rows, devicesOverviewStatusRow{Label: "DPI", Value: strconv.Itoa(int(stage.DPI)), Telemetry: true})
			if stage.Name != "" {
				status.Rows = append(status.Rows, devicesOverviewStatusRow{Label: "Active Stage", Value: stage.Name})
			}
			break
		}
	}
	if performance != nil && performance.PollingRate != nil {
		for _, option := range performance.PollingRate.Options {
			if option.Value == performance.PollingRate.Value && option.Label != "" {
				status.Rows = append(status.Rows, devicesOverviewStatusRow{Label: "Polling Rate", Value: option.Label, Telemetry: true})
				break
			}
		}
	}
	if len(status.Rows) == 0 {
		return nil
	}
	return status
}

func devicesOverviewDisplayStatusFromSummary(summary *devicesDisplayWorkspaceSummary) *devicesOverviewDisplayStatusSummary {
	if summary == nil {
		return nil
	}
	status := &devicesOverviewDisplayStatusSummary{}
	for _, mode := range summary.Modes {
		if mode.Selected && mode.Label != "" {
			status.Rows = append(status.Rows, devicesOverviewStatusRow{Label: "Mode", Value: mode.Label})
			break
		}
	}
	for _, brightness := range summary.BrightnessLevels {
		if brightness.Selected && brightness.Label != "" {
			status.Rows = append(status.Rows, devicesOverviewStatusRow{Label: "Brightness", Value: brightness.Label, Telemetry: true})
			break
		}
	}
	if summary.ImageMode {
		for _, image := range summary.Images {
			if image.Selected && image.Name != "" {
				status.Rows = append(status.Rows, devicesOverviewStatusRow{Label: "Image", Value: image.Name})
				break
			}
		}
	}
	if len(status.Rows) == 0 {
		return nil
	}
	return status
}

func devicesDeviceProfileWorkspaceSummaryFromSnapshot(snapshot deviceprofilepresentation.Snapshot) *devicesDeviceProfileWorkspaceSummary {
	if !snapshot.Supported || snapshot.ActiveProfile == "" || len(snapshot.Profiles) == 0 {
		return nil
	}
	scope := snapshot.Scope
	if scope == "" {
		scope = deviceprofilepresentation.ScopeDevice
	}
	if scope != deviceprofilepresentation.ScopeDevice && scope != deviceprofilepresentation.ScopeLighting {
		return nil
	}
	profileDisplayLabels := map[string]string(nil)
	if snapshot.DefaultProfileDisplayLabel != "" {
		for _, profile := range snapshot.Profiles {
			if profile == "default" {
				profileDisplayLabels = map[string]string{"default": snapshot.DefaultProfileDisplayLabel}
				break
			}
		}
	}
	for _, profile := range snapshot.Profiles {
		if profile == snapshot.ActiveProfile {
			return &devicesDeviceProfileWorkspaceSummary{Profiles: append([]string(nil), snapshot.Profiles...), ProfileDisplayLabels: profileDisplayLabels, ActiveProfile: snapshot.ActiveProfile, Scope: scope}
		}
	}
	return nil
}

func devicesMemoryWorkspaceSummaryFromSnapshot(snapshot memorypresentation.Snapshot) *devicesMemoryWorkspaceSummary {
	if !snapshot.Available || len(snapshot.Modules) == 0 {
		return nil
	}
	summary := &devicesMemoryWorkspaceSummary{Modules: make([]devicesMemoryModuleSummary, 0, len(snapshot.Modules))}
	for _, module := range snapshot.Modules {
		if module.ChannelID < 0 || module.Name == "" {
			return nil
		}
		summary.Modules = append(summary.Modules, devicesMemoryModuleSummary{
			ChannelID: module.ChannelID, Name: module.Name, Label: module.Label,
			MemoryType: module.MemoryType, SKU: module.SKU, LEDCount: module.LEDCount,
			Temperature: module.Temperature, TemperatureCelsius: module.TemperatureCelsius,
		})
	}
	return summary
}

type devicesKeyboardAssignmentTypeSummary struct {
	ID    uint8
	Label string
}
type devicesKeyboardAssignmentKeySummary struct {
	KeyIndex                     int
	KeyName, SubKeyName          string
	Width, Height, Left, Top     int
	CSS, KeySpace, ExtraCSS      string
	Spacing                      []int
	KeyEmpty                     []string
	Assignable, Default, NoColor bool
	ActionType                   uint8
	ActionCommand                uint16
	DeviceID                     string
	ActionHold                   bool
	ToggleDelay                  uint16
	ProfileSwitch                bool
	Red, Green, Blue             float64
}
type devicesKeyboardAssignmentRowSummary struct {
	Index, Top       int
	CSS, OverrideCSS string
	Keys             []devicesKeyboardAssignmentKeySummary
}
type devicesKeyboardAssignmentsWorkspaceSummary struct {
	Rows                                                             []devicesKeyboardAssignmentRowSummary
	AssignmentTypes                                                  []devicesKeyboardAssignmentTypeSummary
	Profiles, KeyboardLayouts                                        []string
	ActiveProfile, ActiveKeyboardLayout, LayoutClass, RowLayoutClass string
	ClusterControlled, LiveRGBAvailable, LiveRGBEnabled              bool
}

func devicesKeyboardAssignmentsWorkspaceSummaryFromSnapshot(snapshot keyboardassignmentspresentation.Snapshot) *devicesKeyboardAssignmentsWorkspaceSummary {
	if !snapshot.Available || len(snapshot.Rows) == 0 || len(snapshot.AssignmentTypes) == 0 {
		return nil
	}
	summary := &devicesKeyboardAssignmentsWorkspaceSummary{Profiles: append([]string(nil), snapshot.Profiles...), ActiveProfile: snapshot.ActiveProfile, KeyboardLayouts: append([]string(nil), snapshot.KeyboardLayouts...), ActiveKeyboardLayout: snapshot.ActiveKeyboardLayout, LayoutClass: snapshot.LayoutClass, RowLayoutClass: snapshot.RowLayoutClass, ClusterControlled: snapshot.ClusterControlled, LiveRGBAvailable: snapshot.LiveRGBAvailable, LiveRGBEnabled: snapshot.LiveRGBEnabled}
	if summary.ActiveProfile == "" || len(summary.Profiles) == 0 {
		return nil
	}
	for _, assignmentType := range snapshot.AssignmentTypes {
		if assignmentType.Label == "" {
			return nil
		}
		summary.AssignmentTypes = append(summary.AssignmentTypes, devicesKeyboardAssignmentTypeSummary{ID: assignmentType.ID, Label: assignmentType.Label})
	}
	for _, row := range snapshot.Rows {
		presented := devicesKeyboardAssignmentRowSummary{Index: row.Index, Top: row.Top, CSS: row.CSS, OverrideCSS: row.OverrideCSS}
		for _, key := range row.Keys {
			if key.KeyName == "" || key.Width < 1 || key.Height < 1 {
				return nil
			}
			presented.Keys = append(presented.Keys, devicesKeyboardAssignmentKeySummary{KeyIndex: key.KeyIndex, KeyName: key.KeyName, SubKeyName: key.SubKeyName, Width: key.Width, Height: key.Height, Left: key.Left, Top: key.Top, CSS: key.CSS, KeySpace: key.KeySpace, ExtraCSS: key.ExtraCSS, Spacing: append([]int(nil), key.Spacing...), KeyEmpty: append([]string(nil), key.KeyEmpty...), Red: key.Red, Green: key.Green, Blue: key.Blue, Assignable: key.Assignable, Default: key.Default, NoColor: key.NoColor, ActionType: key.ActionType, ActionCommand: key.ActionCommand, DeviceID: key.DeviceID, ActionHold: key.ActionHold, ToggleDelay: key.ToggleDelay, ProfileSwitch: key.ProfileSwitch})
		}
		summary.Rows = append(summary.Rows, presented)
	}
	return summary
}

type devicesButtonsAssignmentTypeSummary struct {
	ID    uint8
	Label string
}

type devicesButtonsButtonSummary struct {
	KeyIndex      int
	Name          string
	Default       bool
	PressAndHold  bool
	OnRelease     bool
	ActionType    uint8
	ActionCommand uint16
	IsMacro       bool
	ProfileSwitch bool
}

type devicesButtonsWorkspaceSummary struct {
	Buttons         []devicesButtonsButtonSummary
	AssignmentTypes []devicesButtonsAssignmentTypeSummary
}

func devicesButtonsWorkspaceSummaryFromSnapshot(snapshot buttonspresentation.Snapshot) *devicesButtonsWorkspaceSummary {
	if len(snapshot.Buttons) == 0 || len(snapshot.AssignmentTypes) == 0 {
		return nil
	}
	summary := &devicesButtonsWorkspaceSummary{
		Buttons:         make([]devicesButtonsButtonSummary, len(snapshot.Buttons)),
		AssignmentTypes: make([]devicesButtonsAssignmentTypeSummary, len(snapshot.AssignmentTypes)),
	}
	for index, button := range snapshot.Buttons {
		if button.KeyIndex < 1 || button.Name == "" {
			return nil
		}
		summary.Buttons[index] = devicesButtonsButtonSummary{
			KeyIndex: button.KeyIndex, Name: button.Name, Default: button.Default,
			PressAndHold: button.PressAndHold, OnRelease: button.OnRelease,
			ActionType: button.ActionType, ActionCommand: button.ActionCommand,
			IsMacro: button.IsMacro, ProfileSwitch: button.ProfileSwitch,
		}
	}
	for index, assignmentType := range snapshot.AssignmentTypes {
		if assignmentType.Label == "" {
			return nil
		}
		summary.AssignmentTypes[index] = devicesButtonsAssignmentTypeSummary{ID: assignmentType.ID, Label: assignmentType.Label}
	}
	return summary
}

type devicesDPIStageSummary struct {
	ID, Name, ColorHex string
	DPI                uint16
	Sniper, Active     bool
}

type devicesDPIWorkspaceSummary struct {
	MinimumDPI, MaximumDPI int
	ActiveRegularStageID   string
	RegularStages          []devicesDPIStageSummary
	SniperStage            *devicesDPIStageSummary
}

type devicesPerformanceOptionSummary struct {
	Value int
	Label string
}

type devicesPerformanceSelectSummary struct {
	Value   int
	Options []devicesPerformanceOptionSummary
}

type devicesPerformanceToggleSummary struct {
	Enabled bool
}

type devicesPerformanceBooleanSummary struct {
	ID      string
	Label   string
	Enabled bool
}

type devicesPerformanceWorkspaceSummary struct {
	PollingRate         *devicesPerformanceSelectSummary
	AngleSnapping       *devicesPerformanceToggleSummary
	LiftHeight          *devicesPerformanceSelectSummary
	BooleanSettings     []devicesPerformanceBooleanSummary
	SaveBooleanSettings bool
}

func devicesPerformanceWorkspaceSummaryFromSnapshot(snapshot performancepresentation.Snapshot) *devicesPerformanceWorkspaceSummary {
	summary := &devicesPerformanceWorkspaceSummary{}
	copySelect := func(setting *performancepresentation.SelectSetting) *devicesPerformanceSelectSummary {
		if setting == nil || len(setting.Options) == 0 {
			return nil
		}
		options := make([]devicesPerformanceOptionSummary, len(setting.Options))
		for index, option := range setting.Options {
			if option.Label == "" {
				return nil
			}
			options[index] = devicesPerformanceOptionSummary{Value: option.Value, Label: option.Label}
		}
		return &devicesPerformanceSelectSummary{Value: setting.Value, Options: options}
	}
	summary.PollingRate = copySelect(snapshot.PollingRate)
	summary.LiftHeight = copySelect(snapshot.LiftHeight)
	if snapshot.AngleSnapping != nil {
		summary.AngleSnapping = &devicesPerformanceToggleSummary{Enabled: snapshot.AngleSnapping.Enabled}
	}
	if len(snapshot.BooleanSettings) > 0 {
		seen := make(map[string]struct{}, len(snapshot.BooleanSettings))
		settings := make([]devicesPerformanceBooleanSummary, 0, len(snapshot.BooleanSettings))
		valid := true
		for _, setting := range snapshot.BooleanSettings {
			if setting.ID == "" || setting.Label == "" {
				valid = false
				break
			}
			if _, duplicate := seen[setting.ID]; duplicate {
				valid = false
				break
			}
			seen[setting.ID] = struct{}{}
			settings = append(settings, devicesPerformanceBooleanSummary{ID: setting.ID, Label: setting.Label, Enabled: setting.Enabled})
		}
		if valid {
			summary.BooleanSettings = settings
			summary.SaveBooleanSettings = snapshot.SaveBooleanSettings
		}
	}
	if summary.PollingRate == nil && summary.AngleSnapping == nil && summary.LiftHeight == nil && len(summary.BooleanSettings) == 0 {
		return nil
	}
	return summary
}

func devicesDPIWorkspaceSummaryFromSnapshot(snapshot dpipresentation.Snapshot) *devicesDPIWorkspaceSummary {
	if snapshot.MinimumDPI < 1 || snapshot.MaximumDPI < snapshot.MinimumDPI || len(snapshot.Stages) == 0 {
		return nil
	}
	summary := &devicesDPIWorkspaceSummary{
		MinimumDPI: snapshot.MinimumDPI, MaximumDPI: snapshot.MaximumDPI,
		ActiveRegularStageID: snapshot.ActiveRegularStageID,
		RegularStages:        make([]devicesDPIStageSummary, 0, len(snapshot.Stages)),
	}
	activeRegularFound := false
	for _, stage := range snapshot.Stages {
		if stage.ID == "" || stage.Name == "" || stage.ColorHex == "" {
			return nil
		}
		presented := devicesDPIStageSummary{ID: stage.ID, Name: stage.Name, DPI: stage.DPI, ColorHex: stage.ColorHex, Sniper: stage.Sniper, Active: stage.Active}
		if stage.Sniper {
			if summary.SniperStage != nil {
				return nil
			}
			summary.SniperStage = &presented
			continue
		}
		summary.RegularStages = append(summary.RegularStages, presented)
		if stage.ID == summary.ActiveRegularStageID {
			activeRegularFound = true
		}
	}
	if summary.ActiveRegularStageID == "" || !activeRegularFound || len(summary.RegularStages) == 0 || summary.SniperStage == nil {
		return nil
	}
	return summary
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

var devicesOpenRGBGPUTemperature = temperatures.GetGpuTemperature

// isOpenRGBGPUController recognizes only controller metadata that explicitly
// identifies a graphics processor. Vendor is intentionally not sufficient:
// motherboard and peripheral controllers commonly share GPU vendor names.
func isOpenRGBGPUController(product, description, displayIdentifier string) bool {
	metadata := strings.ToLower(strings.Join([]string{product, description, displayIdentifier}, " "))
	for _, indicator := range []string{
		" gpu", "gpu ", "graphics card", "graphics processor", "geforce", "radeon", "intel arc", "quadro", "tesla", "rtx", "gtx",
	} {
		if strings.Contains(metadata, indicator) {
			return true
		}
	}
	return false
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
		if isOpenRGBGPUController(snapshot.Product, snapshot.Description, snapshot.DisplaySerial) {
			if temperature := devicesOpenRGBGPUTemperature(); temperature > 0 {
				summary.GPUTemperature = dashboard.GetDashboard().TemperatureToString(temperature)
			}
		}
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
	if isOpenRGBGPUController(snapshot.Product, snapshot.Description, snapshot.DisplaySerial) {
		if temperature := devicesOpenRGBGPUTemperature(); temperature > 0 {
			summary.GPUTemperature = dashboard.GetDashboard().TemperatureToString(temperature)
		}
	}
	return summary
}

func devicesLightingEffectDisplayLabel(id, label string) string {
	label = strings.TrimSpace(label)
	if label != "" && label != id {
		return label
	}
	if descriptor, ok := rgb.SoftwareEffectDescriptorByID(id); ok && descriptor.Label != "" {
		return descriptor.Label
	}
	switch id {
	case "keyboard":
		return "Keyboard"
	case "mousepad":
		return "Mousepad"
	case "led":
		return "LED"
	}
	if label != "" || id == "" {
		return label
	}
	return strings.ToUpper(id[:1]) + id[1:]
}

func devicesLightingEffectIconURL(id string) string {
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

func devicesLightingBulkEffectIconURL(effect string, mixed bool) string {
	if mixed {
		return "/static/img/icons/rgb/mixed.svg"
	}
	return devicesLightingEffectIconURL(effect)
}

func devicesLightingWorkspaceSummaryFromSnapshot(snapshot lightingpresentation.Snapshot) *devicesLightingWorkspaceSummary {
	summary := &devicesLightingWorkspaceSummary{
		TargetKind:         snapshot.TargetKind,
		ConfiguredEffect:   snapshot.ConfiguredEffect,
		EffectSupported:    snapshot.EffectSupported,
		HasBrightness:      snapshot.HasBrightness,
		Brightness:         snapshot.Brightness,
		ClusterControlled:  snapshot.ClusterControlled,
		ExternalControlled: snapshot.ExternalControlled,
		SupportedEffects:   make([]devicesLightingEffectSummary, len(snapshot.SupportedEffects)),
		PaletteKind:        snapshot.PaletteKind,
		SingleColorHex:     snapshot.SingleColorHex,
		TwoColorStartHex:   snapshot.TwoColorStartHex,
		TwoColorEndHex:     snapshot.TwoColorEndHex,
		HasTemperature:     snapshot.HasTemperature,
		HasGradient:        snapshot.HasGradient,
		Customized:         snapshot.Customized,
	}
	if summary.HasTemperature {
		summary.TemperatureLow = devicesLightingTemperaturePointSummary{
			Role: "low", Label: "Low", ColorHex: snapshot.TemperatureLow.ColorHex,
			Celsius: strconv.FormatFloat(snapshot.TemperatureLow.Celsius, 'f', -1, 64),
		}
		summary.TemperatureMiddle = devicesLightingTemperaturePointSummary{
			Role: "middle", Label: "Middle", ColorHex: snapshot.TemperatureMiddle.ColorHex,
			Celsius: strconv.FormatFloat(snapshot.TemperatureMiddle.Celsius, 'f', -1, 64),
		}
		summary.TemperatureHigh = devicesLightingTemperaturePointSummary{
			Role: "high", Label: "High", ColorHex: snapshot.TemperatureHigh.ColorHex,
			Celsius: strconv.FormatFloat(snapshot.TemperatureHigh.Celsius, 'f', -1, 64),
		}
		summary.TemperaturePoints = []devicesLightingTemperaturePointSummary{
			summary.TemperatureLow, summary.TemperatureMiddle, summary.TemperatureHigh,
		}
	}
	if summary.HasGradient {
		summary.GradientStops = make([]devicesLightingGradientStopSummary, len(snapshot.GradientStops))
		for index, stop := range snapshot.GradientStops {
			summary.GradientStops[index] = devicesLightingGradientStopSummary{
				Number: index + 1, Position: strconv.FormatFloat(stop.Position, 'f', -1, 64),
				ColorHex: stop.ColorHex, Intensity: strconv.FormatFloat(stop.Intensity, 'f', -1, 64),
			}
		}
	}
	if len(snapshot.IndexedColors) > 0 {
		summary.IndexedColors = make([]devicesLightingIndexedColorSummary, len(snapshot.IndexedColors))
		for index, color := range snapshot.IndexedColors {
			summary.IndexedColors[index] = devicesLightingIndexedColorSummary{Index: color.Index, Label: color.Label, ColorHex: color.ColorHex}
		}
	}
	if editor := snapshot.AuthoredZoneEditor; editor != nil {
		summary.AuthoredZoneEditor = &devicesLightingAuthoredZoneEditorSummary{Heading: editor.Heading, Description: editor.Description, EffectID: editor.EffectID, HasGroups: editor.HasGroups, Zones: make([]devicesLightingAuthoredZoneSummary, len(editor.Zones))}
		for index, zone := range editor.Zones {
			summary.AuthoredZoneEditor.Zones[index] = devicesLightingAuthoredZoneSummary{ID: zone.ID, Label: zone.Label, ColorHex: zone.ColorHex, GroupID: zone.GroupID, GroupLabel: zone.GroupLabel, HasGeometry: zone.HasGeometry, Left: zone.Left, Top: zone.Top, Width: zone.Width, Height: zone.Height}
			if zone.HasGeometry {
				summary.AuthoredZoneEditor.HasGeometry = true
				if right := zone.Left + zone.Width; right > summary.AuthoredZoneEditor.LayoutWidth {
					summary.AuthoredZoneEditor.LayoutWidth = right
				}
				if bottom := zone.Top + zone.Height; bottom > summary.AuthoredZoneEditor.LayoutHeight {
					summary.AuthoredZoneEditor.LayoutHeight = bottom
				}
			}
		}
	}
	if port := snapshot.ThreePinPort; port != nil {
		summary.ThreePinPort = &devicesLightingThreePinPortSummary{QuantityDisabled: port.QuantityDisabled, DeviceOptions: make([]devicesLightingThreePinOptionSummary, len(port.DeviceOptions)), QuantityOptions: make([]devicesLightingThreePinOptionSummary, len(port.QuantityOptions))}
		for index, option := range port.DeviceOptions {
			summary.ThreePinPort.DeviceOptions[index] = devicesLightingThreePinOptionSummary{Value: option.ID, Label: option.Label, Selected: option.Selected}
		}
		for index, option := range port.QuantityOptions {
			summary.ThreePinPort.QuantityOptions[index] = devicesLightingThreePinOptionSummary{Value: option.Value, Label: option.Label, Selected: option.Selected}
		}
	}
	if len(snapshot.ManualRGBPorts) > 0 {
		summary.ManualRGBPorts = make([]devicesLightingManualRGBPortSummary, 0, len(snapshot.ManualRGBPorts))
		for _, port := range snapshot.ManualRGBPorts {
			if port.PortID < 1 || port.Name == "" || len(port.Options) == 0 {
				return nil
			}
			item := devicesLightingManualRGBPortSummary{PortID: strconv.Itoa(port.PortID), Name: port.Name, Options: make([]devicesLightingThreePinOptionSummary, len(port.Options))}
			for index, option := range port.Options {
				item.Options[index] = devicesLightingThreePinOptionSummary{Value: option.ID, Label: option.Label, Selected: option.Selected}
			}
			summary.ManualRGBPorts = append(summary.ManualRGBPorts, item)
		}
	}
	if snapshot.HasSpeed {
		summary.HasSpeedControl = true
		summary.Speed = strconv.FormatFloat(snapshot.Speed, 'f', -1, 64)
	}
	for index, effect := range snapshot.SupportedEffects {
		displayLabel := devicesLightingEffectDisplayLabel(effect.ID, effect.Label)
		summary.SupportedEffects[index] = devicesLightingEffectSummary{
			ID:       effect.ID,
			Label:    displayLabel,
			Selected: snapshot.EffectSupported && effect.ID == snapshot.ConfiguredEffect,
		}
		if effect.ID == snapshot.ConfiguredEffect {
			summary.ConfiguredEffectLabel = displayLabel
			if snapshot.EffectSupported {
				summary.ConfiguredEffectIconURL = devicesLightingEffectIconURL(effect.ID)
			}
		}
	}
	if bulk := snapshot.BulkEffectControl; bulk != nil && len(bulk.SupportedEffects) > 0 {
		summary.BulkEffectControl = &devicesLightingBulkEffectControlSummary{
			ConfiguredEffect: bulk.ConfiguredEffect,
			Mixed:            bulk.Mixed,
			SupportedEffects: make([]devicesLightingEffectSummary, len(bulk.SupportedEffects)),
		}
		for index, effect := range bulk.SupportedEffects {
			label := devicesLightingEffectDisplayLabel(effect.ID, effect.Label)
			summary.BulkEffectControl.SupportedEffects[index] = devicesLightingEffectSummary{ID: effect.ID, Label: label, Selected: !bulk.Mixed && effect.ID == bulk.ConfiguredEffect}
			if effect.ID == bulk.ConfiguredEffect {
				summary.BulkEffectControl.ConfiguredEffectLabel = label
			}
		}
		summary.BulkEffectControl.ConfiguredEffectIconURL = devicesLightingBulkEffectIconURL(bulk.ConfiguredEffect, bulk.Mixed)
		sort.Slice(summary.BulkEffectControl.SupportedEffects, func(i, j int) bool {
			return strings.ToLower(summary.BulkEffectControl.SupportedEffects[i].Label) < strings.ToLower(summary.BulkEffectControl.SupportedEffects[j].Label)
		})
	}
	sort.Slice(summary.SupportedEffects, func(i, j int) bool {
		leftLabel := strings.ToLower(summary.SupportedEffects[i].Label)
		rightLabel := strings.ToLower(summary.SupportedEffects[j].Label)
		if leftLabel == rightLabel {
			return summary.SupportedEffects[i].ID < summary.SupportedEffects[j].ID
		}
		return leftLabel < rightLabel
	})
	if len(snapshot.Channels) > 0 {
		summary.Channels = make([]devicesLightingChannelSummary, 0, len(snapshot.Channels))
		for _, channel := range snapshot.Channels {
			if channel.TargetID == "" || channel.ChannelID == "" || channel.Name == "" {
				return nil
			}
			lighting := devicesLightingWorkspaceSummaryFromSnapshot(channel.Lighting)
			if lighting == nil || len(lighting.Channels) > 0 {
				return nil
			}
			channelSummary := devicesLightingChannelSummary{TargetID: channel.TargetID, ChannelID: channel.ChannelID, Name: channel.Name, Label: channel.Label, LEDCount: channel.LEDCount, ContainsPump: channel.ContainsPump, Lighting: lighting}
			if probe := channel.ProbeTemperature; probe != nil {
				channelSummary.ProbeTemperature = &devicesLightingProbeTemperatureSummary{ProbeID: strconv.Itoa(probe.ProbeID), Minimum: strconv.FormatFloat(probe.Minimum, 'f', -1, 64), Maximum: strconv.FormatFloat(probe.Maximum, 'f', -1, 64), Sources: make([]devicesLightingProbeTemperatureSourceSummary, len(probe.Sources))}
				for index, source := range probe.Sources {
					channelSummary.ProbeTemperature.Sources[index] = devicesLightingProbeTemperatureSourceSummary{ID: strconv.Itoa(source.ID), Label: source.Label, Selected: source.Selected}
				}
			}
			summary.Channels = append(summary.Channels, channelSummary)
		}
	}
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
		Product:        device.Product,
		Serial:         device.Serial,
		Firmware:       device.Firmware,
		Image:          device.Image,
		Unavailable:    device.Unavailable,
		View:           "overview",
		LegacyLighting: device.ProductType == common.ProductTypeCC || device.ProductType == common.ProductTypeCCXT,
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
		}
	}
	if lightingDevice, ok := device.Instance.(devicesLightingSnapshotProvider); ok &&
		lightingDevice != nil && lightingDevice.LightingDeviceID() == serial {
		if lightingSnapshot, usable := lightingDevice.LightingSnapshot(); usable && lightingSnapshot.TargetKind != "" {
			summary.Lighting = devicesLightingWorkspaceSummaryFromSnapshot(lightingSnapshot)
		}
	}
	if dpiDevice, ok := device.Instance.(devicesDPISnapshotProvider); ok &&
		dpiDevice != nil && dpiDevice.DPIDeviceID() == serial {
		if dpiSnapshot, usable := dpiDevice.DPISnapshot(); usable {
			summary.DPI = devicesDPIWorkspaceSummaryFromSnapshot(dpiSnapshot)
		}
	}
	if performanceDevice, ok := device.Instance.(devicesPerformanceSnapshotProvider); ok &&
		performanceDevice != nil && performanceDevice.PerformanceDeviceID() == serial {
		if performanceSnapshot, usable := performanceDevice.PerformanceSnapshot(); usable {
			summary.Performance = devicesPerformanceWorkspaceSummaryFromSnapshot(performanceSnapshot)
		}
	}
	if buttonsDevice, ok := device.Instance.(devicesButtonsSnapshotProvider); ok &&
		buttonsDevice != nil && buttonsDevice.ButtonsDeviceID() == serial {
		if buttonsSnapshot, usable := buttonsDevice.ButtonsSnapshot(); usable {
			summary.Buttons = devicesButtonsWorkspaceSummaryFromSnapshot(buttonsSnapshot)
		}
	}
	if profileDevice, ok := device.Instance.(devicesDeviceProfileSnapshotProvider); ok &&
		profileDevice != nil && profileDevice.DeviceProfileDeviceID() == serial {
		if snapshot, usable := profileDevice.DeviceProfileSnapshot(); usable {
			summary.DeviceProfiles = devicesDeviceProfileWorkspaceSummaryFromSnapshot(snapshot)
		}
	}
	var coolingSnapshot *coolingpresentation.Snapshot
	if coolingDevice, ok := device.Instance.(devicesCoolingSnapshotProvider); ok &&
		coolingDevice != nil && coolingDevice.CoolingDeviceID() == serial {
		if snapshot, usable := coolingDevice.CoolingSnapshot(); usable {
			summary.Cooling = devicesCoolingWorkspaceSummaryFromSnapshot(snapshot)
			coolingSnapshot = &snapshot
		}
	}
	if displayDevice, ok := device.Instance.(devicesDisplaySnapshotProvider); ok &&
		displayDevice != nil && displayDevice.DisplayDeviceID() == serial {
		if snapshot, usable := displayDevice.DisplaySnapshot(); usable {
			summary.Display = devicesDisplayWorkspaceSummaryFromSnapshot(snapshot)
		}
	}
	if memoryDevice, ok := device.Instance.(devicesMemorySnapshotProvider); ok &&
		memoryDevice != nil && memoryDevice.MemoryDeviceID() == serial {
		if snapshot, usable := memoryDevice.MemorySnapshot(); usable {
			summary.Memory = devicesMemoryWorkspaceSummaryFromSnapshot(snapshot)
		}
	}
	if coolingSnapshot != nil {
		summary.OverviewCooling = devicesOverviewCoolingStatusFromSnapshot(*coolingSnapshot)
		summary.TemperatureProbes = devicesOverviewTemperatureProbesFromSnapshot(*coolingSnapshot)
	} else {
		summary.OverviewCooling = devicesOverviewCoolingStatusFromSummary(summary.Cooling)
		summary.TemperatureProbes = devicesOverviewTemperatureProbesFromSummary(summary.Cooling)
	}
	summary.OverviewPerformance = devicesOverviewPerformanceStatusFromSummaries(summary.DPI, summary.Performance)
	summary.OverviewDisplay = devicesOverviewDisplayStatusFromSummary(summary.Display)
	if keyboardDevice, ok := device.Instance.(devicesKeyboardAssignmentsSnapshotProvider); ok && keyboardDevice != nil && keyboardDevice.KeyboardAssignmentsDeviceID() == serial {
		if snapshot, usable := keyboardDevice.KeyboardAssignmentsSnapshot(); usable {
			summary.KeyboardAssignments = devicesKeyboardAssignmentsWorkspaceSummaryFromSnapshot(snapshot)
		}
	}
	if summary.DeviceProfiles != nil {
		summary.DeviceProfiles.Label, summary.DeviceProfiles.Description = devicesDeviceProfilePresentation(device, summary.KeyboardAssignments != nil, summary.DeviceProfiles.Scope)
	}

	return summary, true
}

func devicesWorkspaceView(views []string, device *devicesWorkspaceSummary) string {
	if len(views) != 1 || device == nil {
		return "overview"
	}
	switch views[0] {
	case "lighting":
		if device.Lighting != nil || device.LegacyLighting || device.Memory != nil {
			return "lighting"
		}
	case "dpi":
		if device.KeyboardAssignments == nil && (device.DPI != nil || device.Performance != nil) {
			return "dpi"
		}
	case "cooling":
		if device.Cooling != nil {
			return "cooling"
		}
	case "display":
		if device.Display != nil {
			return "display"
		}
	case "buttons":
		if device.Buttons != nil {
			return "buttons"
		}
	case "keyboard":
		if device.KeyboardAssignments != nil {
			return "keyboard"
		}
	}
	return "overview"
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
		// Lighting and DPI are optional presentation modes. Unknown, empty, or
		// duplicated view values deliberately retain the Overview workspace.
		selectedDevice.View = devicesWorkspaceView(query["view"], selectedDevice)
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

func decodeNativeDeviceLightingRequest(w http.ResponseWriter, r *http.Request, dst any) bool {
	return decodeOpenRGBImportRequest(w, r, dst)
}

func getNativeDeviceLightingTarget(serial string) (nativeDeviceLightingTarget, error) {
	if serial == "" || !common.AlphanumericDashRegex.MatchString(serial) {
		return nil, fmt.Errorf("invalid native device serial")
	}
	wrapper, ok := lookupNativeDeviceLightingWrapper(serial)
	if !ok || wrapper == nil || wrapper.Hidden || wrapper.Unavailable || wrapper.Serial != serial {
		return nil, fmt.Errorf("native device is not available")
	}
	target, ok := wrapper.Instance.(nativeDeviceLightingTarget)
	if !ok || target == nil || target.LightingDeviceID() != serial {
		return nil, fmt.Errorf("native device lighting is not available")
	}
	return target, nil
}

func validateNativeDeviceLightingEffect(target nativeDeviceLightingTarget, effect string) error {
	if !target.SupportsLightingEffect(effect) {
		return fmt.Errorf("unsupported effect")
	}
	return nil
}

func updateNativeDeviceLightingSettings(target nativeDeviceLightingTarget, targetID, effect string, mutate func(*lightingsettings.EffectSettings)) error {
	if err := validateNativeDeviceLightingEffect(target, effect); err != nil {
		return err
	}
	channelSettings, isChannelTarget := target.(nativeDeviceLightingChannelSettingsTarget)
	var settings lightingsettings.EffectSettings
	var err error
	if targetID != "" {
		if !isChannelTarget {
			return fmt.Errorf("native lighting channel settings are unavailable")
		}
		settings, err = channelSettings.ResolveLightingChannelEffectSettings(targetID, effect)
	} else {
		settings, err = target.ResolveLightingEffectSettings(effect)
	}
	if err != nil {
		return err
	}
	settings = settings.Clone()
	if settings.EffectID != effect {
		return fmt.Errorf("resolved effect settings do not match requested effect")
	}
	mutate(&settings)
	if err := lightingsettings.Validate(settings); err != nil {
		return err
	}
	if targetID != "" {
		return channelSettings.SetLightingChannelEffectSettings(targetID, effect, settings)
	}
	return target.SetLightingEffectSettings(effect, settings)
}

func nativeDeviceLightingTargetForEffect(serial, effect string) (nativeDeviceLightingTarget, error) {
	target, err := getNativeDeviceLightingTarget(serial)
	if err != nil {
		return nil, err
	}
	if err := validateNativeDeviceLightingEffect(target, effect); err != nil {
		return nil, err
	}
	return target, nil
}

func nativeDeviceAuthoredZoneLightingTargetForEffect(serial, effect string) (nativeDeviceAuthoredZoneLightingTarget, error) {
	target, err := getNativeDeviceLightingTarget(serial)
	if err != nil {
		return nil, err
	}
	authored, ok := target.(nativeDeviceAuthoredZoneLightingTarget)
	if !ok || authored == nil || !authored.SupportsLightingEffect(effect) {
		return nil, fmt.Errorf("native authored-zone lighting is not available")
	}
	return authored, nil
}

func nativeDeviceLightingFailure(w http.ResponseWriter, message string) {
	(&Response{Code: http.StatusOK, Status: 0, Message: message}).Send(w)
}

func getDevicesDPIWorkspaceTarget(serial string) (devicesDPIWorkspaceTarget, error) {
	if serial == "" || !common.AlphanumericDashRegex.MatchString(serial) {
		return nil, fmt.Errorf("invalid DPI device serial")
	}
	wrapper, ok := lookupDevicesDPIWorkspaceWrapper(serial)
	if !ok || wrapper == nil || wrapper.Hidden || wrapper.Unavailable || wrapper.Serial != serial {
		return nil, fmt.Errorf("DPI device is not available")
	}
	target, ok := wrapper.Instance.(devicesDPIWorkspaceTarget)
	if !ok || target == nil || target.DPIDeviceID() != serial {
		return nil, fmt.Errorf("DPI workspace is not available")
	}
	return target, nil
}

func getDevicesDPISnapshotProvider(serial string) (devicesDPISnapshotProvider, error) {
	if serial == "" || !common.AlphanumericDashRegex.MatchString(serial) {
		return nil, fmt.Errorf("invalid DPI device serial")
	}
	wrapper, ok := lookupDevicesDPIWorkspaceWrapper(serial)
	if !ok || wrapper == nil || wrapper.Hidden || wrapper.Unavailable || wrapper.Serial != serial {
		return nil, fmt.Errorf("DPI device is not available")
	}
	provider, ok := wrapper.Instance.(devicesDPISnapshotProvider)
	if !ok || provider == nil || provider.DPIDeviceID() != serial {
		return nil, fmt.Errorf("DPI workspace is not available")
	}
	return provider, nil
}

type devicesDPIStatusResponse struct {
	Status               int    `json:"status"`
	ActiveRegularStageID string `json:"activeRegularStageId,omitempty"`
	SniperActive         bool   `json:"sniperActive"`
}

func (response devicesDPIStatusResponse) Send(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func getDevicesDPIWorkspaceStatus(w http.ResponseWriter, r *http.Request) {
	provider, err := getDevicesDPISnapshotProvider(r.URL.Query().Get("serial"))
	if err != nil {
		(devicesDPIStatusResponse{}).Send(w)
		return
	}
	snapshot, usable := provider.DPISnapshot()
	summary := devicesDPIWorkspaceSummaryFromSnapshot(snapshot)
	if !usable || summary == nil || summary.SniperStage == nil {
		(devicesDPIStatusResponse{}).Send(w)
		return
	}
	(devicesDPIStatusResponse{
		Status:               1,
		ActiveRegularStageID: summary.ActiveRegularStageID,
		SniperActive:         summary.SniperStage.Active,
	}).Send(w)
}

func selectDevicesDPIWorkspaceStage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial  *string `json:"serial"`
		StageID *string `json:"stageId"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	if req.Serial == nil || req.StageID == nil || *req.Serial == "" || *req.StageID == "" {
		nativeDeviceLightingFailure(w, "Invalid DPI stage request")
		return
	}
	target, err := getDevicesDPIWorkspaceTarget(*req.Serial)
	if err != nil {
		nativeDeviceLightingFailure(w, "DPI workspace is not available")
		return
	}
	stageID, parseErr := strconv.Atoi(*req.StageID)
	if parseErr != nil || stageID < 0 {
		nativeDeviceLightingFailure(w, "Invalid DPI stage request")
		return
	}
	snapshot, usable := target.DPISnapshot()
	if !usable {
		nativeDeviceLightingFailure(w, "Invalid DPI stage request")
		return
	}
	found := false
	for _, stage := range snapshot.Stages {
		if stage.ID == *req.StageID && !stage.Sniper {
			found = true
			break
		}
	}
	if !found || target.SelectMouseDPIStage(stageID) != 1 {
		nativeDeviceLightingFailure(w, "Unable to select DPI stage")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "DPI stage selected"}).Send(w)
}

func setDevicesDPIWorkspaceSniper(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial *string `json:"serial"`
		Active *bool   `json:"active"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	if req.Serial == nil || req.Active == nil || *req.Serial == "" {
		nativeDeviceLightingFailure(w, "Invalid Sniper mode request")
		return
	}
	target, err := getDevicesDPIWorkspaceTarget(*req.Serial)
	if err != nil {
		nativeDeviceLightingFailure(w, "DPI workspace is not available")
		return
	}
	snapshot, usable := target.DPISnapshot()
	if !usable {
		nativeDeviceLightingFailure(w, "Invalid Sniper mode request")
		return
	}
	hasSniper := false
	for _, stage := range snapshot.Stages {
		if stage.Sniper {
			hasSniper = true
			break
		}
	}
	if !hasSniper || target.SetMouseSniperMode(*req.Active) != 1 {
		nativeDeviceLightingFailure(w, "Unable to update Sniper mode")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Sniper mode updated"}).Send(w)
}

func saveDevicesDPIWorkspace(w http.ResponseWriter, r *http.Request) {
	type stageRequest struct {
		ID    *string `json:"id"`
		DPI   *int    `json:"dpi"`
		Color *string `json:"color"`
	}
	var req struct {
		Serial *string         `json:"serial"`
		Stages *[]stageRequest `json:"stages"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	if req.Serial == nil || req.Stages == nil || *req.Serial == "" {
		nativeDeviceLightingFailure(w, "Invalid DPI request")
		return
	}
	target, err := getDevicesDPIWorkspaceTarget(*req.Serial)
	if err != nil {
		nativeDeviceLightingFailure(w, "Unable to save DPI settings")
		return
	}
	snapshot, usable := target.DPISnapshot()
	if !usable || snapshot.MinimumDPI < 1 || snapshot.MaximumDPI < snapshot.MinimumDPI || len(*req.Stages) != len(snapshot.Stages) {
		nativeDeviceLightingFailure(w, "Invalid DPI request")
		return
	}
	allowed := make(map[string]struct{}, len(snapshot.Stages))
	for _, stage := range snapshot.Stages {
		allowed[stage.ID] = struct{}{}
	}
	stages := make(map[int]uint16, len(*req.Stages))
	colors := make(map[int]rgb.Color, len(*req.Stages))
	for _, stage := range *req.Stages {
		if stage.ID == nil || stage.DPI == nil || stage.Color == nil || *stage.ID == "" || *stage.DPI < snapshot.MinimumDPI || *stage.DPI > snapshot.MaximumDPI {
			nativeDeviceLightingFailure(w, "Invalid DPI request")
			return
		}
		if _, exists := allowed[*stage.ID]; !exists {
			nativeDeviceLightingFailure(w, "Invalid DPI request")
			return
		}
		id, parseErr := strconv.Atoi(*stage.ID)
		color, colorErr := parseHexColor(*stage.Color)
		if parseErr != nil || id < 0 || colorErr != nil {
			nativeDeviceLightingFailure(w, "Invalid DPI request")
			return
		}
		if _, duplicate := stages[id]; duplicate {
			nativeDeviceLightingFailure(w, "Invalid DPI request")
			return
		}
		stages[id] = uint16(*stage.DPI)
		colors[id] = rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue}
	}
	if len(stages) != len(allowed) || target.SaveMouseDPISettings(stages, colors) != 1 {
		nativeDeviceLightingFailure(w, "Unable to save DPI settings")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "DPI settings saved"}).Send(w)
}

func setNativeDeviceLightingIndexedColor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial, TargetID, Color *string
		Index                   *int `json:"index"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	if req.Serial == nil || req.TargetID == nil || req.Index == nil || req.Color == nil || *req.Serial == "" || *req.TargetID == "" {
		nativeDeviceLightingFailure(w, "Invalid indexed color request")
		return
	}
	color, colorErr := parseHexColor(*req.Color)
	target, targetErr := getNativeDeviceLightingTarget(*req.Serial)
	indexed, ok := target.(nativeDeviceLightingIndexedColorTarget)
	if colorErr != nil || targetErr != nil || !ok || indexed.SetLightingIndexedColor(*req.TargetID, *req.Index, color) != nil {
		nativeDeviceLightingFailure(w, "Invalid indexed color request")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Indexed color set"}).Send(w)
}

func setNativeDeviceLightingIndexedColors(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial   *string `json:"serial"`
		TargetID *string `json:"targetId"`
		Colors   *[]struct {
			Index *int    `json:"index"`
			Color *string `json:"color"`
		} `json:"colors"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	if req.Serial == nil || req.TargetID == nil || req.Colors == nil || *req.Serial == "" || *req.TargetID == "" {
		nativeDeviceLightingFailure(w, "Invalid indexed colors request")
		return
	}
	colors := make([]lightingsettings.IndexedColor, len(*req.Colors))
	for i, color := range *req.Colors {
		if color.Index == nil || color.Color == nil {
			nativeDeviceLightingFailure(w, "Invalid indexed colors request")
			return
		}
		if _, err := parseHexColor(*color.Color); err != nil {
			nativeDeviceLightingFailure(w, "Invalid indexed colors request")
			return
		}
		colors[i] = lightingsettings.IndexedColor{Index: *color.Index, ColorHex: *color.Color}
	}
	target, targetErr := getNativeDeviceLightingTarget(*req.Serial)
	indexed, ok := target.(nativeDeviceLightingIndexedColorsTarget)
	if targetErr != nil || !ok || indexed.SetLightingIndexedColors(*req.TargetID, colors) != nil {
		nativeDeviceLightingFailure(w, "Invalid indexed colors request")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Indexed colors set"}).Send(w)
}

func setNativeDeviceLightingEffect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial   *string `json:"serial"`
		TargetID *string `json:"targetId"`
		Effect   *string `json:"effect"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	if req.Serial == nil || *req.Serial == "" || req.Effect == nil || *req.Effect == "" {
		nativeDeviceLightingFailure(w, "Invalid effect request")
		return
	}
	target, err := getNativeDeviceLightingTarget(*req.Serial)
	if err != nil || !target.SupportsLightingEffect(*req.Effect) {
		nativeDeviceLightingFailure(w, "Unable to set effect")
		return
	}
	if req.TargetID != nil && *req.TargetID != "" {
		channelTarget, ok := target.(nativeDeviceLightingChannelTarget)
		if !ok || channelTarget.SetLightingChannelEffect(*req.TargetID, *req.Effect) != nil {
			nativeDeviceLightingFailure(w, "Unable to set effect")
			return
		}
	} else if target.SetLightingEffect(*req.Effect) != nil {
		nativeDeviceLightingFailure(w, "Unable to set effect")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Effect set"}).Send(w)
}

func setNativeDeviceLightingAllChannelEffects(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial *string `json:"serial"`
		Effect *string `json:"effect"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	if req.Serial == nil || *req.Serial == "" || req.Effect == nil || *req.Effect == "" {
		nativeDeviceLightingFailure(w, "Invalid effect request")
		return
	}
	target, err := getNativeDeviceLightingTarget(*req.Serial)
	bulk, ok := target.(nativeDeviceLightingAllChannelTarget)
	if err != nil || !ok || bulk.SetLightingAllChannelEffects(*req.Effect) != nil {
		nativeDeviceLightingFailure(w, "Unable to set effects")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Effects set"}).Send(w)
}

func setNativeDeviceLightingBrightness(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial     *string `json:"serial"`
		Brightness *int    `json:"brightness"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	if req.Serial == nil || *req.Serial == "" || req.Brightness == nil || *req.Brightness < 0 || *req.Brightness > 100 {
		nativeDeviceLightingFailure(w, "Invalid brightness request")
		return
	}
	target, err := getNativeDeviceLightingTarget(*req.Serial)
	if err != nil || target.SetLightingBrightness(uint8(*req.Brightness)) != nil {
		nativeDeviceLightingFailure(w, "Unable to set brightness")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Brightness set"}).Send(w)
}

func setNativeDeviceLightingSpeed(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial   *string  `json:"serial"`
		TargetID *string  `json:"targetId"`
		Effect   *string  `json:"effect"`
		Speed    *float64 `json:"speed"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	if req.Serial == nil || *req.Serial == "" || req.Effect == nil || *req.Effect == "" || req.Speed == nil || math.IsNaN(*req.Speed) || math.IsInf(*req.Speed, 0) {
		nativeDeviceLightingFailure(w, "Invalid speed request")
		return
	}
	target, err := nativeDeviceLightingTargetForEffect(*req.Serial, *req.Effect)
	minimum, maximum := rgb.ProfileSpeedRange(*req.Effect)
	descriptor, known := rgb.SoftwareEffectDescriptorByID(*req.Effect)
	targetID := ""
	if req.TargetID != nil {
		targetID = *req.TargetID
	}
	if err != nil || !known || !descriptor.SupportsSpeed || *req.Speed < minimum || *req.Speed > maximum || updateNativeDeviceLightingSettings(target, targetID, *req.Effect, func(settings *lightingsettings.EffectSettings) { speed := *req.Speed; settings.Speed = &speed }) != nil {
		nativeDeviceLightingFailure(w, "Invalid speed request")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Speed set"}).Send(w)
}

func setNativeDeviceLightingSingleColor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial   string `json:"serial"`
		TargetID string `json:"targetId"`
		Effect   string `json:"effect"`
		Color    string `json:"color"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	color, colorErr := parseHexColor(req.Color)
	target, targetErr := nativeDeviceLightingTargetForEffect(req.Serial, req.Effect)
	if req.Serial == "" || req.Effect == "" || req.Color == "" || colorErr != nil || targetErr != nil || updateNativeDeviceLightingSettings(target, req.TargetID, req.Effect, func(settings *lightingsettings.EffectSettings) {
		settings.SingleColor = &lightingsettings.SingleColorSettings{Color: color}
	}) != nil {
		nativeDeviceLightingFailure(w, "Invalid color request")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Applied successfully"}).Send(w)
}

func setNativeDeviceLightingTwoColor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial   string `json:"serial"`
		TargetID string `json:"targetId"`
		Effect   string `json:"effect"`
		Start    string `json:"start"`
		End      string `json:"end"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	start, startErr := parseHexColor(req.Start)
	end, endErr := parseHexColor(req.End)
	target, targetErr := nativeDeviceLightingTargetForEffect(req.Serial, req.Effect)
	if req.Serial == "" || req.Effect == "" || req.Start == "" || req.End == "" || startErr != nil || endErr != nil || targetErr != nil || updateNativeDeviceLightingSettings(target, req.TargetID, req.Effect, func(settings *lightingsettings.EffectSettings) {
		settings.TwoColor = &lightingsettings.TwoColorSettings{Start: start, End: end}
	}) != nil {
		nativeDeviceLightingFailure(w, "Invalid color request")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Applied successfully"}).Send(w)
}

type nativeDeviceLightingTemperaturePointRequest struct {
	Color   *string  `json:"color"`
	Celsius *float64 `json:"celsius"`
}
type nativeDeviceLightingGradientStopRequest struct {
	Position  *float64 `json:"position"`
	Color     *string  `json:"color"`
	Intensity *float64 `json:"intensity"`
}

func setNativeDeviceLightingTemperature(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial   string                                       `json:"serial"`
		TargetID string                                       `json:"targetId"`
		Effect   string                                       `json:"effect"`
		Low      *nativeDeviceLightingTemperaturePointRequest `json:"low"`
		Middle   *nativeDeviceLightingTemperaturePointRequest `json:"middle"`
		High     *nativeDeviceLightingTemperaturePointRequest `json:"high"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	valid := req.Serial != "" && req.Effect != "" && req.Low != nil && req.Middle != nil && req.High != nil && req.Low.Color != nil && req.Low.Celsius != nil && req.Middle.Color != nil && req.Middle.Celsius != nil && req.High.Color != nil && req.High.Celsius != nil
	if !valid {
		nativeDeviceLightingFailure(w, "Invalid temperature request")
		return
	}
	lowColor, lowErr := parseHexColor(*req.Low.Color)
	middleColor, middleErr := parseHexColor(*req.Middle.Color)
	highColor, highErr := parseHexColor(*req.High.Color)
	low, middle, high := *req.Low.Celsius, *req.Middle.Celsius, *req.High.Celsius
	target, targetErr := nativeDeviceLightingTargetForEffect(req.Serial, req.Effect)
	if lowErr != nil || middleErr != nil || highErr != nil || math.IsNaN(low) || math.IsInf(low, 0) || math.IsNaN(middle) || math.IsInf(middle, 0) || math.IsNaN(high) || math.IsInf(high, 0) || !(low < middle && middle < high) || targetErr != nil || updateNativeDeviceLightingSettings(target, req.TargetID, req.Effect, func(settings *lightingsettings.EffectSettings) {
		settings.Temperature = &lightingsettings.TemperatureSettings{Low: lightingsettings.TemperaturePoint{Color: lowColor, Celsius: low}, Middle: lightingsettings.TemperaturePoint{Color: middleColor, Celsius: middle}, High: lightingsettings.TemperaturePoint{Color: highColor, Celsius: high}}
	}) != nil {
		nativeDeviceLightingFailure(w, "Invalid temperature request")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Applied successfully"}).Send(w)
}

func setNativeDeviceLightingGradient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial   string                                     `json:"serial"`
		TargetID string                                     `json:"targetId"`
		Effect   string                                     `json:"effect"`
		Stops    *[]nativeDeviceLightingGradientStopRequest `json:"stops"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	if req.Serial == "" || req.Effect == "" || req.Stops == nil || len(*req.Stops) < 2 || len(*req.Stops) > openRGBImportGradientStopLimit {
		nativeDeviceLightingFailure(w, "Invalid Gradient request")
		return
	}
	stops := make([]lightingsettings.GradientStop, len(*req.Stops))
	previous := -1.0
	for index, input := range *req.Stops {
		if input.Position == nil || input.Color == nil || input.Intensity == nil {
			nativeDeviceLightingFailure(w, "Invalid Gradient request")
			return
		}
		color, err := parseHexColor(*input.Color)
		position, intensity := *input.Position, *input.Intensity
		if err != nil || math.IsNaN(position) || math.IsInf(position, 0) || position < 0 || position > 1 || math.IsNaN(intensity) || math.IsInf(intensity, 0) || intensity < 0 || intensity > 1 || position < previous {
			nativeDeviceLightingFailure(w, "Invalid Gradient request")
			return
		}
		stops[index] = lightingsettings.GradientStop{Position: position, Color: color, Intensity: intensity}
		previous = position
	}
	target, err := nativeDeviceLightingTargetForEffect(req.Serial, req.Effect)
	if err != nil || updateNativeDeviceLightingSettings(target, req.TargetID, req.Effect, func(settings *lightingsettings.EffectSettings) {
		settings.Gradient = &lightingsettings.GradientSettings{Stops: stops}
	}) != nil {
		nativeDeviceLightingFailure(w, "Invalid Gradient request")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Applied successfully"}).Send(w)
}

func resetNativeDeviceLightingEffectSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial   string `json:"serial"`
		TargetID string `json:"targetId"`
		Effect   string `json:"effect"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	target, err := nativeDeviceLightingTargetForEffect(req.Serial, req.Effect)
	channelSettings, isChannelTarget := target.(nativeDeviceLightingChannelSettingsTarget)
	if req.Serial == "" || req.Effect == "" || err != nil || (req.TargetID != "" && (!isChannelTarget || channelSettings.ResetLightingChannelEffectSettings(req.TargetID, req.Effect) != nil)) || (req.TargetID == "" && target.ResetLightingEffectSettings(req.Effect) != nil) {
		nativeDeviceLightingFailure(w, "Failed to reset effect customization")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Reset successfully"}).Send(w)
}

func setNativeDeviceLightingZones(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Serial  *string   `json:"serial"`
		Effect  *string   `json:"effect"`
		Scope   *string   `json:"scope"`
		ZoneID  *string   `json:"zoneId"`
		ZoneIDs *[]string `json:"zoneIds"`
		GroupID *string   `json:"groupId"`
		Color   *string   `json:"color"`
	}
	if !decodeNativeDeviceLightingRequest(w, r, &req) {
		return
	}
	if req.Serial == nil || req.Effect == nil || req.Scope == nil || req.Color == nil || *req.Serial == "" || *req.Effect == "" || *req.Color == "" {
		nativeDeviceLightingFailure(w, "Invalid authored-zone request")
		return
	}
	zoneID, groupID := "", ""
	if req.ZoneID != nil {
		zoneID = *req.ZoneID
	}
	if req.GroupID != nil {
		groupID = *req.GroupID
	}
	switch *req.Scope {
	case "zone":
		if zoneID == "" || groupID != "" {
			nativeDeviceLightingFailure(w, "Invalid authored-zone request")
			return
		}
	case "group":
		if groupID == "" || zoneID != "" {
			nativeDeviceLightingFailure(w, "Invalid authored-zone request")
			return
		}
	case "all":
		if zoneID != "" || groupID != "" {
			nativeDeviceLightingFailure(w, "Invalid authored-zone request")
			return
		}
	case "zones":
		if req.ZoneIDs == nil || len(*req.ZoneIDs) == 0 || zoneID != "" || groupID != "" {
			nativeDeviceLightingFailure(w, "Invalid authored-zone request")
			return
		}
		seen := make(map[string]struct{}, len(*req.ZoneIDs))
		for _, id := range *req.ZoneIDs {
			if id == "" {
				nativeDeviceLightingFailure(w, "Invalid authored-zone request")
				return
			}
			if _, exists := seen[id]; exists {
				nativeDeviceLightingFailure(w, "Invalid authored-zone request")
				return
			}
			seen[id] = struct{}{}
		}
	default:
		nativeDeviceLightingFailure(w, "Invalid authored-zone request")
		return
	}
	color, colorErr := parseHexColor(*req.Color)
	target, targetErr := nativeDeviceAuthoredZoneLightingTargetForEffect(*req.Serial, *req.Effect)
	if colorErr != nil || targetErr != nil {
		nativeDeviceLightingFailure(w, "Invalid authored-zone request")
		return
	}
	mutationErr := error(nil)
	if *req.Scope == "zones" {
		multi, ok := target.(nativeDeviceAuthoredZoneLightingMultiTarget)
		if !ok || multi == nil {
			mutationErr = fmt.Errorf("native authored-zone multi-selection is not available")
		} else {
			mutationErr = multi.SetLightingZoneColors(*req.Effect, *req.ZoneIDs, rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue, Hex: *req.Color})
		}
	} else {
		mutationErr = target.SetLightingZoneColor(*req.Effect, *req.Scope, zoneID, groupID, rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue, Hex: *req.Color})
	}
	if mutationErr != nil {
		nativeDeviceLightingFailure(w, "Invalid authored-zone request")
		return
	}
	(&Response{Code: http.StatusOK, Status: 1, Message: "Applied successfully"}).Send(w)
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
	handleFunc(r, "/api/dashboard/devices/current", http.MethodGet, getDashboardCurrentDevices)
	r.HandleFunc("/api/dashboard/layout", dashboardLayoutRoute)
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
	handleFunc(r, "/api/devices/dpi/status", http.MethodGet, getDevicesDPIWorkspaceStatus)

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
	handleFunc(r, "/api/devices/lighting/effect", http.MethodPost, setNativeDeviceLightingEffect)
	handleFunc(r, "/api/devices/lighting/effect-all", http.MethodPost, setNativeDeviceLightingAllChannelEffects)
	handleFunc(r, "/api/devices/lighting/indexed-color", http.MethodPost, setNativeDeviceLightingIndexedColor)
	handleFunc(r, "/api/devices/lighting/indexed-colors", http.MethodPost, setNativeDeviceLightingIndexedColors)
	handleFunc(r, "/api/devices/lighting/brightness", http.MethodPost, setNativeDeviceLightingBrightness)
	handleFunc(r, "/api/devices/lighting/speed", http.MethodPost, setNativeDeviceLightingSpeed)
	handleFunc(r, "/api/devices/lighting/single-color", http.MethodPost, setNativeDeviceLightingSingleColor)
	handleFunc(r, "/api/devices/lighting/two-color", http.MethodPost, setNativeDeviceLightingTwoColor)
	handleFunc(r, "/api/devices/lighting/temperature", http.MethodPost, setNativeDeviceLightingTemperature)
	handleFunc(r, "/api/devices/lighting/gradient", http.MethodPost, setNativeDeviceLightingGradient)
	handleFunc(r, "/api/devices/lighting/effect-reset", http.MethodPost, resetNativeDeviceLightingEffectSettings)
	handleFunc(r, "/api/devices/lighting/zones", http.MethodPost, setNativeDeviceLightingZones)
	handleFunc(r, "/api/devices/performance/polling-rate", http.MethodPost, changePollingRate)
	handleFunc(r, "/api/devices/performance/angle-snapping", http.MethodPost, changeAngleSnapping)
	handleFunc(r, "/api/devices/performance/lift-height", http.MethodPost, changeLiftHeight)
	handleFunc(r, "/api/devices/performance/keyboard", http.MethodPost, setKeyboardPerformance)
	handleFunc(r, "/api/devices/dpi", http.MethodPost, saveDevicesDPIWorkspace)
	handleFunc(r, "/api/devices/dpi/active", http.MethodPost, selectDevicesDPIWorkspaceStage)
	handleFunc(r, "/api/devices/dpi/sniper", http.MethodPost, setDevicesDPIWorkspaceSniper)
	handleFunc(r, "/api/devices/lighting/rgb-cluster", http.MethodPost, setRgbCluster)
	handleFunc(r, "/api/devices/lighting/openrgb-integration", http.MethodPost, setOpenRgbIntegration)
	handleFunc(r, "/api/cluster/lighting/effect", http.MethodPost, setRGBClusterLightingEffect)
	handleFunc(r, "/api/cluster/lighting/status", http.MethodGet, getRGBClusterLightingStatusHandler)
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
