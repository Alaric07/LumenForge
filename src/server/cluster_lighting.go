package server

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"LumenForge/src/cluster"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

const (
	clusterLightingRequestLimit      = 64 << 10
	clusterLightingGradientStopLimit = 1024
)

var (
	getClusterLightingDevice = cluster.Get
	setClusterLightingEffect = func(device *cluster.Device, effect string) error {
		return device.SetLightingEffect(effect)
	}
	setClusterLightingBrightness = func(device *cluster.Device, brightness uint8) error {
		return device.SetLightingBrightness(brightness)
	}
	setClusterLightingSpeed = func(device *cluster.Device, effect string, speed float64) error {
		return device.SetLightingSpeed(effect, speed)
	}
	setClusterLightingSingleColor = func(device *cluster.Device, effect string, color lightingsettings.Color) error {
		return device.SetLightingSingleColor(effect, color)
	}
	setClusterLightingTwoColor = func(device *cluster.Device, effect string, start, end lightingsettings.Color) error {
		return device.SetLightingTwoColor(effect, start, end)
	}
	setClusterLightingTemperature = func(device *cluster.Device, effect string, low, middle, high lightingsettings.TemperaturePoint) error {
		return device.SetLightingTemperature(effect, low, middle, high)
	}
	setClusterLightingGradient = func(device *cluster.Device, effect string, stops []lightingsettings.GradientStop) error {
		return device.SetLightingGradient(effect, stops)
	}
)

type clusterLightingEffectSummary struct {
	ID       string
	Label    string
	Selected bool
}

type clusterLightingWorkspaceSummary struct {
	Available               bool
	ControllerCount         int
	ConfiguredEffect        string
	ConfiguredEffectLabel   string
	ConfiguredEffectIconURL string
	EffectSupported         bool
	SupportedEffects        []clusterLightingEffectSummary
	Brightness              uint8
	EffectiveBrightness     uint8
	HasSpeedControl         bool
	Speed                   string
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

// clusterLightingWorkspaceSummaryFromSnapshot translates only canonical
// Cluster state and resolved settings into defensive display values.
func clusterLightingWorkspaceSummaryFromSnapshot(snapshot cluster.LightingSnapshot) *clusterLightingWorkspaceSummary {
	summary := &clusterLightingWorkspaceSummary{
		Available:       snapshot.Available,
		ControllerCount: snapshot.ControllerCount,
	}
	if !snapshot.Available || snapshot.SelectedEffect == "" ||
		snapshot.Settings.EffectID != snapshot.SelectedEffect || lightingsettings.Validate(snapshot.Settings) != nil {
		summary.Available = false
		return summary
	}

	descriptor, ok := rgb.SoftwareEffectDescriptorByID(snapshot.SelectedEffect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeCluster) {
		summary.Available = false
		return summary
	}
	summary.ConfiguredEffect = snapshot.SelectedEffect
	summary.ConfiguredEffectLabel = openRGBLightingEffectDisplayLabel(descriptor.ID, descriptor.Label)
	summary.ConfiguredEffectIconURL = clusterLightingEffectIconURL(descriptor.ID)
	summary.EffectSupported = true
	summary.Brightness = snapshot.Brightness
	summary.EffectiveBrightness = snapshot.EffectiveBrightness
	summary.PaletteKind = string(descriptor.PaletteKind)
	summary.Customized = snapshot.Customized
	if descriptor.SupportsSpeed && snapshot.Settings.Speed != nil {
		summary.HasSpeedControl = true
		summary.Speed = strconv.FormatFloat(*snapshot.Settings.Speed, 'f', -1, 64)
	}

	switch descriptor.PaletteKind {
	case rgb.LightingPaletteStaticSingle:
		summary.SingleColorHex = clusterLightingColorHex(snapshot.Settings.SingleColor.Color)
	case rgb.LightingPaletteTwoColor:
		summary.TwoColorStartHex = clusterLightingColorHex(snapshot.Settings.TwoColor.Start)
		summary.TwoColorEndHex = clusterLightingColorHex(snapshot.Settings.TwoColor.End)
	case rgb.LightingPaletteTemperatureThree:
		summary.HasTemperature = true
		temperature := snapshot.Settings.Temperature
		summary.TemperatureLow = clusterLightingTemperaturePointSummary("low", "Low", temperature.Low)
		summary.TemperatureMiddle = clusterLightingTemperaturePointSummary("middle", "Middle", temperature.Middle)
		summary.TemperatureHigh = clusterLightingTemperaturePointSummary("high", "High", temperature.High)
		summary.TemperaturePoints = []openRGBLightingTemperaturePointSummary{
			summary.TemperatureLow, summary.TemperatureMiddle, summary.TemperatureHigh,
		}
	case rgb.LightingPaletteGradient:
		summary.HasGradient = true
		summary.GradientStops = make([]openRGBLightingGradientStopSummary, len(snapshot.Settings.Gradient.Stops))
		for index, stop := range snapshot.Settings.Gradient.Stops {
			summary.GradientStops[index] = openRGBLightingGradientStopSummary{
				Number: index + 1, Position: strconv.FormatFloat(stop.Position, 'f', -1, 64),
				ColorHex: clusterLightingColorHex(stop.Color), Intensity: strconv.FormatFloat(stop.Intensity, 'f', -1, 64),
			}
		}
	}

	for _, effect := range rgb.SoftwareEffectDescriptors() {
		if !effect.Scope.Includes(rgb.EffectScopeCluster) {
			continue
		}
		summary.SupportedEffects = append(summary.SupportedEffects, clusterLightingEffectSummary{
			ID: effect.ID, Label: openRGBLightingEffectDisplayLabel(effect.ID, effect.Label), Selected: effect.ID == snapshot.SelectedEffect,
		})
	}
	sort.Slice(summary.SupportedEffects, func(i, j int) bool {
		left, right := strings.ToLower(summary.SupportedEffects[i].Label), strings.ToLower(summary.SupportedEffects[j].Label)
		if left == right {
			return summary.SupportedEffects[i].ID < summary.SupportedEffects[j].ID
		}
		return left < right
	})
	return summary
}

func clusterLightingTemperaturePointSummary(role, label string, point lightingsettings.TemperaturePoint) openRGBLightingTemperaturePointSummary {
	return openRGBLightingTemperaturePointSummary{
		Role: role, Label: label, ColorHex: clusterLightingColorHex(point.Color),
		Celsius: strconv.FormatFloat(point.Celsius, 'f', -1, 64),
	}
}

func clusterLightingColorHex(color lightingsettings.Color) string {
	return fmt.Sprintf("#%02x%02x%02x",
		openRGBLightingColorComponent(color.Red),
		openRGBLightingColorComponent(color.Green),
		openRGBLightingColorComponent(color.Blue),
	)
}

func clusterLightingEffectIconURL(id string) string {
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(id)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeCluster) {
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

func decodeClusterLightingRequest(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, clusterLightingRequestLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		clusterLightingFailure(w, "Invalid request body")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		clusterLightingFailure(w, "Invalid request body")
		return false
	}
	return true
}

func clusterLightingDeviceOrFail(w http.ResponseWriter) *cluster.Device {
	device := getClusterLightingDevice()
	if device == nil {
		clusterLightingFailure(w, "RGB Cluster is unavailable")
		return nil
	}
	return device
}

func clusterLightingFailure(w http.ResponseWriter, message string) {
	(&Response{Code: http.StatusOK, Status: 0, Message: message}).Send(w)
}

func clusterLightingSuccess(w http.ResponseWriter, message string) {
	(&Response{Code: http.StatusOK, Status: 1, Message: message}).Send(w)
}

func clusterLightingDescriptor(effect string, palette rgb.LightingPaletteKind, requireSpeed bool) bool {
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeCluster) {
		return false
	}
	if palette != "" && descriptor.PaletteKind != palette {
		return false
	}
	return !requireSpeed || descriptor.SupportsSpeed
}

func setRGBClusterLightingEffect(w http.ResponseWriter, r *http.Request) {
	request := struct {
		Effect *string `json:"effect"`
	}{}
	if !decodeClusterLightingRequest(w, r, &request) {
		return
	}
	if request.Effect == nil || !clusterLightingDescriptor(*request.Effect, "", false) {
		clusterLightingFailure(w, "Invalid effect request")
		return
	}
	device := clusterLightingDeviceOrFail(w)
	if device == nil {
		return
	}
	if err := setClusterLightingEffect(device, *request.Effect); err != nil {
		clusterLightingFailure(w, "Unable to set RGB Cluster effect")
		return
	}
	clusterLightingSuccess(w, "RGB Cluster effect set")
}

func setRGBClusterLightingBrightness(w http.ResponseWriter, r *http.Request) {
	request := struct {
		Brightness *int `json:"brightness"`
	}{}
	if !decodeClusterLightingRequest(w, r, &request) {
		return
	}
	if request.Brightness == nil || *request.Brightness < 0 || *request.Brightness > 100 {
		clusterLightingFailure(w, "Invalid brightness request")
		return
	}
	device := clusterLightingDeviceOrFail(w)
	if device == nil {
		return
	}
	if err := setClusterLightingBrightness(device, uint8(*request.Brightness)); err != nil {
		clusterLightingFailure(w, "Unable to set RGB Cluster brightness")
		return
	}
	clusterLightingSuccess(w, "RGB Cluster brightness set")
}

func setRGBClusterLightingSpeed(w http.ResponseWriter, r *http.Request) {
	request := struct {
		Effect *string  `json:"effect"`
		Speed  *float64 `json:"speed"`
	}{}
	if !decodeClusterLightingRequest(w, r, &request) {
		return
	}
	if request.Effect == nil || request.Speed == nil || math.IsNaN(*request.Speed) || math.IsInf(*request.Speed, 0) ||
		!clusterLightingDescriptor(*request.Effect, "", true) {
		clusterLightingFailure(w, "Invalid speed request")
		return
	}
	minimum, maximum := rgb.ProfileSpeedRange(*request.Effect)
	if *request.Speed < minimum || *request.Speed > maximum {
		clusterLightingFailure(w, "Invalid speed request")
		return
	}
	device := clusterLightingDeviceOrFail(w)
	if device == nil {
		return
	}
	if err := setClusterLightingSpeed(device, *request.Effect, *request.Speed); err != nil {
		clusterLightingFailure(w, "Unable to set RGB Cluster speed")
		return
	}
	clusterLightingSuccess(w, "RGB Cluster speed set")
}

func setRGBClusterLightingSingleColor(w http.ResponseWriter, r *http.Request) {
	request := struct {
		Effect *string `json:"effect"`
		Color  *string `json:"color"`
	}{}
	if !decodeClusterLightingRequest(w, r, &request) {
		return
	}
	if request.Effect == nil || request.Color == nil || !clusterLightingDescriptor(*request.Effect, rgb.LightingPaletteStaticSingle, false) {
		clusterLightingFailure(w, "Invalid single-color request")
		return
	}
	color, err := parseHexColor(*request.Color)
	if err != nil {
		clusterLightingFailure(w, "Invalid single-color request")
		return
	}
	device := clusterLightingDeviceOrFail(w)
	if device == nil {
		return
	}
	if err = setClusterLightingSingleColor(device, *request.Effect, color); err != nil {
		clusterLightingFailure(w, "Unable to set RGB Cluster color")
		return
	}
	clusterLightingSuccess(w, "RGB Cluster color set")
}

func setRGBClusterLightingTwoColor(w http.ResponseWriter, r *http.Request) {
	request := struct {
		Effect *string `json:"effect"`
		Start  *string `json:"start"`
		End    *string `json:"end"`
	}{}
	if !decodeClusterLightingRequest(w, r, &request) {
		return
	}
	if request.Effect == nil || request.Start == nil || request.End == nil ||
		!clusterLightingDescriptor(*request.Effect, rgb.LightingPaletteTwoColor, false) {
		clusterLightingFailure(w, "Invalid two-color request")
		return
	}
	start, startErr := parseHexColor(*request.Start)
	end, endErr := parseHexColor(*request.End)
	if startErr != nil || endErr != nil {
		clusterLightingFailure(w, "Invalid two-color request")
		return
	}
	device := clusterLightingDeviceOrFail(w)
	if device == nil {
		return
	}
	if err := setClusterLightingTwoColor(device, *request.Effect, start, end); err != nil {
		clusterLightingFailure(w, "Unable to set RGB Cluster colors")
		return
	}
	clusterLightingSuccess(w, "RGB Cluster colors set")
}

type clusterLightingTemperaturePointRequest struct {
	Color   *string  `json:"color"`
	Celsius *float64 `json:"celsius"`
}

func setRGBClusterLightingTemperature(w http.ResponseWriter, r *http.Request) {
	request := struct {
		Effect *string                                 `json:"effect"`
		Low    *clusterLightingTemperaturePointRequest `json:"low"`
		Middle *clusterLightingTemperaturePointRequest `json:"middle"`
		High   *clusterLightingTemperaturePointRequest `json:"high"`
	}{}
	if !decodeClusterLightingRequest(w, r, &request) {
		return
	}
	if request.Effect == nil || request.Low == nil || request.Middle == nil || request.High == nil ||
		request.Low.Color == nil || request.Low.Celsius == nil || request.Middle.Color == nil || request.Middle.Celsius == nil ||
		request.High.Color == nil || request.High.Celsius == nil ||
		!clusterLightingDescriptor(*request.Effect, rgb.LightingPaletteTemperatureThree, false) {
		clusterLightingFailure(w, "Invalid temperature request")
		return
	}
	lowColor, lowErr := parseHexColor(*request.Low.Color)
	middleColor, middleErr := parseHexColor(*request.Middle.Color)
	highColor, highErr := parseHexColor(*request.High.Color)
	lowCelsius, middleCelsius, highCelsius := *request.Low.Celsius, *request.Middle.Celsius, *request.High.Celsius
	if lowErr != nil || middleErr != nil || highErr != nil ||
		math.IsNaN(lowCelsius) || math.IsInf(lowCelsius, 0) || math.IsNaN(middleCelsius) || math.IsInf(middleCelsius, 0) ||
		math.IsNaN(highCelsius) || math.IsInf(highCelsius, 0) || !(lowCelsius < middleCelsius && middleCelsius < highCelsius) {
		clusterLightingFailure(w, "Invalid temperature request")
		return
	}
	low := lightingsettings.TemperaturePoint{Color: lowColor, Celsius: lowCelsius}
	middle := lightingsettings.TemperaturePoint{Color: middleColor, Celsius: middleCelsius}
	high := lightingsettings.TemperaturePoint{Color: highColor, Celsius: highCelsius}
	device := clusterLightingDeviceOrFail(w)
	if device == nil {
		return
	}
	if err := setClusterLightingTemperature(device, *request.Effect, low, middle, high); err != nil {
		clusterLightingFailure(w, "Unable to set RGB Cluster temperature settings")
		return
	}
	clusterLightingSuccess(w, "RGB Cluster temperature settings set")
}

type clusterLightingGradientStopRequest struct {
	Position  *float64 `json:"position"`
	Color     *string  `json:"color"`
	Intensity *float64 `json:"intensity"`
}

func setRGBClusterLightingGradient(w http.ResponseWriter, r *http.Request) {
	request := struct {
		Effect *string                               `json:"effect"`
		Stops  *[]clusterLightingGradientStopRequest `json:"stops"`
	}{}
	if !decodeClusterLightingRequest(w, r, &request) {
		return
	}
	if request.Effect == nil || request.Stops == nil || len(*request.Stops) < 2 || len(*request.Stops) > clusterLightingGradientStopLimit ||
		!clusterLightingDescriptor(*request.Effect, rgb.LightingPaletteGradient, false) {
		clusterLightingFailure(w, "Invalid Gradient request")
		return
	}
	stops := make([]lightingsettings.GradientStop, len(*request.Stops))
	previousPosition := -1.0
	for index, input := range *request.Stops {
		if input.Position == nil || input.Color == nil || input.Intensity == nil {
			clusterLightingFailure(w, "Invalid Gradient request")
			return
		}
		position, intensity := *input.Position, *input.Intensity
		color, err := parseHexColor(*input.Color)
		if err != nil || math.IsNaN(position) || math.IsInf(position, 0) || position < 0 || position > 1 ||
			math.IsNaN(intensity) || math.IsInf(intensity, 0) || intensity < 0 || intensity > 1 || position < previousPosition {
			clusterLightingFailure(w, "Invalid Gradient request")
			return
		}
		stops[index] = lightingsettings.GradientStop{Position: position, Color: color, Intensity: intensity}
		previousPosition = position
	}
	device := clusterLightingDeviceOrFail(w)
	if device == nil {
		return
	}
	if err := setClusterLightingGradient(device, *request.Effect, stops); err != nil {
		clusterLightingFailure(w, "Unable to set RGB Cluster Gradient")
		return
	}
	clusterLightingSuccess(w, "RGB Cluster Gradient set")
}
