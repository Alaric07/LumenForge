package cluster

// Package: cluster
// Author: Nikola Jurkovic
// License: GPL-3.0 or later

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"LumenForge/src/temperatures"
)

var (
	d                     *Device
	deviceRefreshInterval = 1000
)

const workerStopTimeout = 250 * time.Millisecond

// DeviceProfile remains an in-memory compatibility projection for the legacy
// Cluster page. Canonical lighting and layout stores are the only authorities.
type DeviceProfile struct {
	RGBProfile         string
	BrightnessSlider   *uint8
	OriginalBrightness uint8
	DeviceOrder        []string
	RgbOff             bool
	LastNonOffProfile  string
}

// LightingSnapshot is a read-only view of canonical RGB Cluster target state.
type LightingSnapshot struct {
	SelectedEffect      string
	Brightness          uint8
	EffectiveBrightness uint8
	Available           bool
}

type Device struct {
	Product         string `json:"product"`
	Serial          string `json:"serial"`
	DeviceProfile   *DeviceProfile
	Rgb             *rgb.RGB
	activeRgb       *rgb.ActiveRGB
	Controllers     []*common.ClusterController
	mutex           sync.RWMutex
	Exit            bool
	RGBModes        []string
	CpuTemp         float32
	GpuTemp         float32
	timer           *time.Ticker
	autoRefreshChan chan struct{}

	lightingState      *clusterLightingStateStore
	layout             *clusterLayoutStore
	effects            *lightingsettings.ClusterStore
	resolver           *lightingsettings.Resolver
	runtimeErr         error
	schedulerLightsOut bool

	workerMutex          sync.Mutex
	workerStop           chan struct{}
	workerDone           chan struct{}
	workerStopping       bool
	workerRestartPending bool
	workerStarts         uint64
}

func clusterRGBModes() []string {
	return []string{
		"off", "arc", "comet", "datastream", "circle", "circleshift",
		"colorpulse", "colorshift", "colorwarp", "cpu-temperature",
		"flickering", "flame", "aurora", "cyberpunkglitch", "tokyonight",
		"gpu-temperature", "gradient", "marquee", "nebula", "rain",
		"rainbow", "pastelrainbow", "plasmacore", "rotarystack", "rotator",
		"sequential", "spinner", "spiralrainbow", "pastelspiralrainbow",
		"static", "stardust", "storm", "visor", "watercolor", "wave",
	}
}

func newBaseDevice() *Device {
	return &Device{
		Product:         "Cluster",
		Serial:          lightingsettings.RGBClusterIdentity,
		RGBModes:        clusterRGBModes(),
		autoRefreshChan: make(chan struct{}),
		Controllers:     make([]*common.ClusterController, 0),
	}
}

func newDevice(paths config.Paths) (*Device, error) {
	device := newBaseDevice()
	defaults, err := lightingsettings.LoadDefaultRepository(filepath.Join(paths.ShippedDatabaseRoot, "rgb.json"))
	if err != nil {
		return device, fmt.Errorf("load RGB Cluster shipped defaults: %w", err)
	}
	effects, err := lightingsettings.LoadClusterStore(paths.ClusterEffectSettingsFile)
	if err != nil {
		return device, fmt.Errorf("load RGB Cluster effect settings: %w", err)
	}
	state, err := loadClusterLightingStateStore(paths.RGBClusterLightingStateFile)
	if err != nil {
		return device, err
	}
	layout, err := loadClusterLayoutStore(paths.RGBClusterLayoutFile)
	if err != nil {
		return device, err
	}
	resolver, err := lightingsettings.NewClusterResolver(defaults, effects)
	if err != nil {
		return device, err
	}
	device.effects = effects
	device.lightingState = state
	device.layout = layout
	device.resolver = resolver
	if err = device.refreshCompatibilityProjection(); err != nil {
		return device, err
	}
	device.setAutoRefresh()
	return device, nil
}

func Init() *Device {
	device, err := newDevice(config.GetPaths())
	if err != nil {
		device.runtimeErr = err
		logger.Log(logger.Fields{"error": err}).Error("Unable to initialize canonical RGB Cluster lighting")
		device.setAutoRefresh()
	}
	d = device
	return d
}

// Stop stops Cluster output without changing canonical desired state.
func (d *Device) Stop() {
	if d == nil {
		return
	}
	logger.Log(logger.Fields{"serial": d.Serial, "product": d.Product}).Info("Stopping device...")
	d.mutex.Lock()
	d.Exit = true
	autoRefreshChan := d.autoRefreshChan
	d.autoRefreshChan = nil
	timer := d.timer
	d.timer = nil
	d.mutex.Unlock()
	if autoRefreshChan != nil {
		close(autoRefreshChan)
	}
	if timer != nil {
		timer.Stop()
	}
	d.stopWorker()
	d.mutex.Lock()
	d.Controllers = make([]*common.ClusterController, 0)
	d.mutex.Unlock()
	logger.Log(logger.Fields{"serial": d.Serial, "product": d.Product}).Info("Device stopped")
}

func Get() *Device {
	return d
}

func (d *Device) runtimeAvailable() bool {
	return d != nil && d.runtimeErr == nil && d.lightingState != nil && d.layout != nil && d.effects != nil && d.resolver != nil
}

// LightingSnapshot returns canonical state without exposing mutable stores.
func (d *Device) LightingSnapshot() LightingSnapshot {
	if !d.runtimeAvailable() {
		return LightingSnapshot{}
	}
	state, err := d.lightingState.Snapshot()
	if err != nil {
		return LightingSnapshot{}
	}
	d.mutex.RLock()
	lightsOut := d.schedulerLightsOut
	d.mutex.RUnlock()
	effective := state.Brightness
	if lightsOut {
		effective = 0
	}
	return LightingSnapshot{
		SelectedEffect:      state.SelectedEffect,
		Brightness:          state.Brightness,
		EffectiveBrightness: effective,
		Available:           true,
	}
}

// ControllerCount returns the current runtime membership count.
func (d *Device) ControllerCount() int {
	if d == nil {
		return 0
	}
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return len(d.Controllers)
}

func (d *Device) AddDeviceController(controller *common.ClusterController) {
	if d == nil || controller == nil {
		return
	}
	d.mutex.Lock()
	duplicate := false
	for _, current := range d.Controllers {
		if current != nil && current.Serial == controller.Serial && current.ChannelId == controller.ChannelId {
			duplicate = true
			break
		}
	}
	if !duplicate {
		d.Controllers = append(d.Controllers, controller)
	}
	count := len(d.Controllers)
	d.mutex.Unlock()
	if duplicate {
		return
	}
	d.SortControllers()
	if count == 1 {
		d.restartWorker()
	}
}

// SortControllers orders runtime members from the independent layout store.
func (d *Device) SortControllers() {
	if d == nil || d.layout == nil {
		return
	}
	layout, err := d.layout.Snapshot()
	if err != nil {
		return
	}
	ranks := make(map[string]int, len(layout.DeviceOrder))
	for index, serial := range layout.DeviceOrder {
		ranks[serial] = index
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()
	sort.SliceStable(d.Controllers, func(first, second int) bool {
		firstController := d.Controllers[first]
		secondController := d.Controllers[second]
		if firstController == nil || secondController == nil {
			return firstController != nil && secondController == nil
		}
		firstRank, firstFound := ranks[firstController.Serial]
		secondRank, secondFound := ranks[secondController.Serial]
		if firstFound != secondFound {
			return firstFound
		}
		return firstFound && firstRank < secondRank
	})
}

func (d *Device) RemoveDeviceControllerBySerial(serial string) {
	if d == nil {
		return
	}
	d.mutex.Lock()
	removed := false
	for index := len(d.Controllers) - 1; index >= 0; index-- {
		if d.Controllers[index] != nil && d.Controllers[index].Serial == serial {
			d.Controllers = append(d.Controllers[:index], d.Controllers[index+1:]...)
			removed = true
		}
	}
	empty := len(d.Controllers) == 0
	d.mutex.Unlock()
	if removed && empty {
		d.stopWorker()
	}
}

func (d *Device) compatibilityProfiles() (map[string]rgb.Profile, error) {
	if !d.runtimeAvailable() {
		return nil, fmt.Errorf("RGB Cluster lighting runtime is unavailable")
	}
	profiles := make(map[string]rgb.Profile, len(d.RGBModes))
	for _, effect := range d.RGBModes {
		resolution, err := d.resolver.Resolve(lightingsettings.RGBCluster(), effect)
		if err != nil {
			return nil, err
		}
		profiles[effect] = rgbProfileFromSettings(resolution.Settings)
	}
	return profiles, nil
}

func (d *Device) refreshCompatibilityProjection() error {
	if !d.runtimeAvailable() {
		return fmt.Errorf("RGB Cluster lighting runtime is unavailable")
	}
	state, err := d.lightingState.Snapshot()
	if err != nil {
		return err
	}
	layout, err := d.layout.Snapshot()
	if err != nil {
		return err
	}
	profiles, err := d.compatibilityProfiles()
	if err != nil {
		return err
	}
	brightness := state.Brightness
	profile := &DeviceProfile{
		RGBProfile:       state.SelectedEffect,
		BrightnessSlider: &brightness,
		DeviceOrder:      append([]string(nil), layout.DeviceOrder...),
		RgbOff:           state.SelectedEffect == "off",
	}
	d.mutex.Lock()
	d.DeviceProfile = profile
	d.Rgb = &rgb.RGB{Device: d.Product, Profiles: profiles}
	d.mutex.Unlock()
	return nil
}

func (d *Device) GetRgbProfiles() interface{} {
	if d == nil {
		return &rgb.RGB{Profiles: map[string]rgb.Profile{}}
	}
	profiles, err := d.compatibilityProfiles()
	if err != nil {
		return &rgb.RGB{Device: d.Product, Profiles: map[string]rgb.Profile{}}
	}
	return &rgb.RGB{Device: d.Product, Profiles: profiles}
}

func (d *Device) GetRgbProfile(effect string) *rgb.Profile {
	if !d.runtimeAvailable() {
		return nil
	}
	resolution, err := d.resolver.Resolve(lightingsettings.RGBCluster(), effect)
	if err != nil {
		return nil
	}
	profile := rgbProfileFromSettings(resolution.Settings)
	return &profile
}

func rgbProfileFromSettings(settings lightingsettings.EffectSettings) rgb.Profile {
	profile := rgb.Profile{ProfileName: settings.EffectID, Brightness: 1}
	if settings.Speed != nil {
		profile.Speed = *settings.Speed
	}
	if settings.SingleColor != nil {
		profile.StartColor = rgbColorFromLightingColor(settings.SingleColor.Color)
		profile.EndColor = profile.StartColor
	}
	if settings.TwoColor != nil {
		profile.StartColor = rgbColorFromLightingColor(settings.TwoColor.Start)
		profile.EndColor = rgbColorFromLightingColor(settings.TwoColor.End)
	}
	if settings.Temperature != nil {
		profile.StartColor = rgbTemperatureColor(settings.Temperature.Low)
		profile.MiddleColor = rgbTemperatureColor(settings.Temperature.Middle)
		profile.EndColor = rgbTemperatureColor(settings.Temperature.High)
		profile.MinTemp = settings.Temperature.Low.Celsius
		profile.MaxTemp = settings.Temperature.High.Celsius
	}
	if settings.Gradient != nil {
		profile.Gradients = make(map[int]rgb.Color, len(settings.Gradient.Stops))
		for index, stop := range settings.Gradient.Stops {
			color := rgbColorFromLightingColor(stop.Color)
			color.Position = stop.Position
			color.Brightness = stop.Intensity
			profile.Gradients[index] = color
		}
	}
	return profile
}

func rgbColorFromLightingColor(color lightingsettings.Color) rgb.Color {
	return rgb.Color{Red: color.Red, Green: color.Green, Blue: color.Blue, Brightness: 1}
}

func rgbTemperatureColor(point lightingsettings.TemperaturePoint) rgb.Color {
	color := rgbColorFromLightingColor(point.Color)
	color.Temperature = point.Celsius
	return color
}

func lightingColorFromRGB(color rgb.Color) lightingsettings.Color {
	return lightingsettings.Color{Red: color.Red, Green: color.Green, Blue: color.Blue}
}

func settingsFromLegacyProfile(effect string, profile rgb.Profile, current lightingsettings.EffectSettings) (lightingsettings.EffectSettings, error) {
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeCluster) {
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported RGB Cluster effect")
	}
	settings := current.Clone()
	settings.SchemaVersion = lightingsettings.SchemaVersion
	settings.EffectID = effect
	if descriptor.SupportsSpeed {
		speed := profile.Speed
		settings.Speed = &speed
	}
	switch descriptor.PaletteKind {
	case rgb.LightingPaletteNone, rgb.LightingPaletteGenerated:
	case rgb.LightingPaletteStaticSingle:
		settings.SingleColor = &lightingsettings.SingleColorSettings{Color: lightingColorFromRGB(profile.StartColor)}
	case rgb.LightingPaletteTwoColor:
		settings.TwoColor = &lightingsettings.TwoColorSettings{
			Start: lightingColorFromRGB(profile.StartColor),
			End:   lightingColorFromRGB(profile.EndColor),
		}
	case rgb.LightingPaletteTemperatureThree:
		if current.Temperature == nil {
			return lightingsettings.EffectSettings{}, fmt.Errorf("complete RGB Cluster temperature settings are unavailable")
		}
		settings.Temperature = &lightingsettings.TemperatureSettings{
			Low: lightingsettings.TemperaturePoint{
				Color: lightingColorFromRGB(profile.StartColor), Celsius: profile.MinTemp,
			},
			Middle: lightingsettings.TemperaturePoint{
				Color: lightingColorFromRGB(profile.MiddleColor), Celsius: current.Temperature.Middle.Celsius,
			},
			High: lightingsettings.TemperaturePoint{
				Color: lightingColorFromRGB(profile.EndColor), Celsius: profile.MaxTemp,
			},
		}
	case rgb.LightingPaletteGradient:
		indexes := make([]int, 0, len(profile.Gradients))
		for index := range profile.Gradients {
			indexes = append(indexes, index)
		}
		sort.Ints(indexes)
		stops := make([]lightingsettings.GradientStop, 0, len(indexes))
		for _, index := range indexes {
			color := profile.Gradients[index]
			stops = append(stops, lightingsettings.GradientStop{
				Position: color.Position, Color: lightingColorFromRGB(color), Intensity: color.Brightness,
			})
		}
		sort.SliceStable(stops, func(first, second int) bool { return stops[first].Position < stops[second].Position })
		settings.Gradient = &lightingsettings.GradientSettings{Stops: stops}
	default:
		return lightingsettings.EffectSettings{}, fmt.Errorf("unsupported RGB Cluster palette")
	}
	if err := lightingsettings.Validate(settings); err != nil {
		return lightingsettings.EffectSettings{}, err
	}
	return settings, nil
}

func (d *Device) ProcessNewGradientColor(effect string) (uint8, uint) {
	if !d.runtimeAvailable() {
		return 0, 0
	}
	resolution, err := d.resolver.Resolve(lightingsettings.RGBCluster(), effect)
	if err != nil || resolution.Settings.Gradient == nil {
		return 0, 0
	}
	settings := resolution.Settings.Clone()
	stops := append([]lightingsettings.GradientStop(nil), settings.Gradient.Stops...)
	position := 0.0
	if len(stops) > 0 {
		position = stops[len(stops)-1].Position
	}
	stops = append(stops, lightingsettings.GradientStop{
		Position: position, Color: lightingsettings.Color{Green: 255, Blue: 255}, Intensity: 0,
	})
	settings.Gradient.Stops = stops
	if err = d.effects.Set(effect, settings); err != nil {
		logger.Log(logger.Fields{"error": err, "effect": effect}).Error("Unable to persist RGB Cluster Gradient")
		return 0, 0
	}
	_ = d.refreshCompatibilityProjection()
	if d.LightingSnapshot().SelectedEffect == effect {
		d.restartWorker()
	}
	return 1, uint(len(stops) - 1)
}

func (d *Device) ProcessDeleteGradientColor(effect string) (uint8, uint) {
	if !d.runtimeAvailable() {
		return 0, 0
	}
	resolution, err := d.resolver.Resolve(lightingsettings.RGBCluster(), effect)
	if err != nil || resolution.Settings.Gradient == nil {
		return 0, 0
	}
	settings := resolution.Settings.Clone()
	if len(settings.Gradient.Stops) < 3 {
		return 2, 0
	}
	removed := len(settings.Gradient.Stops) - 1
	settings.Gradient.Stops = append([]lightingsettings.GradientStop(nil), settings.Gradient.Stops[:removed]...)
	if err = d.effects.Set(effect, settings); err != nil {
		logger.Log(logger.Fields{"error": err, "effect": effect}).Error("Unable to persist RGB Cluster Gradient")
		return 0, 0
	}
	_ = d.refreshCompatibilityProjection()
	if d.LightingSnapshot().SelectedEffect == effect {
		d.restartWorker()
	}
	return 1, uint(removed)
}

func (d *Device) UpdateRgbProfileData(effect string, profile rgb.Profile) uint8 {
	if !d.runtimeAvailable() {
		return 0
	}
	resolution, err := d.resolver.Resolve(lightingsettings.RGBCluster(), effect)
	if err != nil {
		return 0
	}
	settings, err := settingsFromLegacyProfile(effect, profile, resolution.Settings)
	if err != nil {
		logger.Log(logger.Fields{"error": err, "effect": effect}).Warn("Invalid RGB Cluster effect settings")
		return 0
	}
	if err = d.effects.Set(effect, settings); err != nil {
		logger.Log(logger.Fields{"error": err, "effect": effect}).Error("Unable to persist RGB Cluster effect settings")
		return 0
	}
	_ = d.refreshCompatibilityProjection()
	if d.LightingSnapshot().SelectedEffect == effect {
		d.restartWorker()
	}
	return 1
}

func (d *Device) UpdateRgbProfile(_ int, effect string) uint8 {
	if !d.runtimeAvailable() {
		return 0
	}
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeCluster) {
		return 0
	}
	state, err := d.lightingState.Snapshot()
	if err != nil {
		return 0
	}
	state.SelectedEffect = effect
	if err = d.lightingState.Set(state); err != nil {
		logger.Log(logger.Fields{"error": err, "effect": effect}).Error("Unable to persist RGB Cluster selected effect")
		return 0
	}
	_ = d.refreshCompatibilityProjection()
	d.restartWorker()
	return 1
}

func (d *Device) UpdateDeviceOrder(order []string) uint8 {
	if !d.runtimeAvailable() {
		return 0
	}
	layout := clusterLayout{SchemaVersion: clusterPersistenceSchemaVersion, DeviceOrder: append([]string(nil), order...)}
	if err := d.layout.Set(layout); err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to persist RGB Cluster layout")
		return 0
	}
	_ = d.refreshCompatibilityProjection()
	d.SortControllers()
	d.restartWorker()
	return 1
}

func (d *Device) ChangeDeviceBrightnessValue(value uint8) uint8 {
	if !d.runtimeAvailable() || value > 100 {
		return 0
	}
	state, err := d.lightingState.Snapshot()
	if err != nil {
		return 0
	}
	state.Brightness = value
	if err = d.lightingState.Set(state); err != nil {
		logger.Log(logger.Fields{"error": err, "brightness": value}).Error("Unable to persist RGB Cluster Brightness")
		return 0
	}
	_ = d.refreshCompatibilityProjection()
	d.restartWorker()
	return 1
}

func (d *Device) SchedulerBrightness(value uint8) uint8 {
	if !d.runtimeAvailable() {
		return 0
	}
	d.mutex.Lock()
	d.schedulerLightsOut = value == 0
	d.mutex.Unlock()
	d.restartWorker()
	return 1
}

func (d *Device) ControlDeviceRgb(value bool) {
	if value {
		d.UpdateRgbProfile(0, "off")
		return
	}
	// The clean-break legacy compatibility path intentionally uses Rainbow when
	// turning lighting on so no remembered effect becomes another authority.
	d.UpdateRgbProfile(0, "rainbow")
}

func (d *Device) controllerSnapshot() ([]*common.ClusterController, int) {
	if d == nil {
		return nil, 0
	}
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	controllers := make([]*common.ClusterController, 0, len(d.Controllers))
	total := 0
	for _, controller := range d.Controllers {
		if controller == nil {
			continue
		}
		copy := *controller
		controllers = append(controllers, &copy)
		total += int(copy.LedChannels)
	}
	return controllers, total
}

func distributeColorsToControllers(buff []byte, controllers []*common.ClusterController) {
	distributeColorsToControllersUntilStopped(buff, controllers, nil)
}

func distributeColorsToControllersUntilStopped(buff []byte, controllers []*common.ClusterController, stop <-chan struct{}) {
	var wait sync.WaitGroup
	offset := 0
	for _, controller := range controllers {
		if stop != nil {
			select {
			case <-stop:
				wait.Wait()
				return
			default:
			}
		}
		length := int(controller.LedChannels) * 3
		if offset+length > len(buff) {
			break
		}
		if controller.WriteColorEx != nil {
			segment := append([]byte(nil), buff[offset:offset+length]...)
			wait.Add(1)
			go func(current *common.ClusterController, data []byte) {
				defer wait.Done()
				current.WriteColorEx(data, current.ChannelId)
			}(controller, segment)
		}
		offset += length
	}
	wait.Wait()
}

func (d *Device) distributeColors(buff []byte) {
	controllers, _ := d.controllerSnapshot()
	distributeColorsToControllers(buff, controllers)
}

func (d *Device) clearWorkerOwnershipLocked(done chan struct{}) {
	if d.workerDone != done {
		return
	}
	d.workerStop = nil
	d.workerDone = nil
	d.workerStopping = false
	d.activeRgb = nil
}

func (d *Device) stopWorkerLocked() bool {
	if d.workerDone == nil {
		return true
	}
	done := d.workerDone
	if !d.workerStopping {
		close(d.workerStop)
		d.workerStopping = true
	}
	timer := time.NewTimer(workerStopTimeout)
	defer timer.Stop()
	select {
	case <-done:
		d.clearWorkerOwnershipLocked(done)
		return true
	case <-timer.C:
		logger.Log(logger.Fields{"serial": d.Serial, "product": d.Product}).Warn("Timed out stopping RGB Cluster worker")
		return false
	}
}

func (d *Device) stopWorker() bool {
	if d == nil {
		return true
	}
	d.workerMutex.Lock()
	defer d.workerMutex.Unlock()
	d.workerRestartPending = false
	return d.stopWorkerLocked()
}

func (d *Device) restartWorker() {
	if d == nil {
		return
	}
	d.workerMutex.Lock()
	defer d.workerMutex.Unlock()
	if d.workerRestartPending && d.workerDone != nil && d.workerStopping {
		return
	}
	d.workerRestartPending = true
	if !d.stopWorkerLocked() {
		return
	}
	d.workerRestartPending = false
	d.startWorkerLocked()
}

func (d *Device) startWorker() {
	if d == nil {
		return
	}
	d.workerMutex.Lock()
	defer d.workerMutex.Unlock()
	d.startWorkerLocked()
}

func (d *Device) startWorkerLocked() {
	if d.workerDone != nil || !d.runtimeAvailable() {
		return
	}
	d.mutex.RLock()
	exiting := d.Exit
	d.mutex.RUnlock()
	if exiting {
		return
	}
	controllers, channels := d.controllerSnapshot()
	if len(controllers) == 0 || channels == 0 {
		return
	}
	snapshot := d.LightingSnapshot()
	resolution, err := d.resolver.Resolve(lightingsettings.RGBCluster(), snapshot.SelectedEffect)
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to resolve RGB Cluster lighting")
		return
	}
	profile := rgbProfileFromSettings(resolution.Settings)
	active := rgb.Exit()
	active.RGBStartColor = rgb.GenerateRandomColor(1)
	active.RGBEndColor = rgb.GenerateRandomColor(1)
	stop := make(chan struct{})
	done := make(chan struct{})
	d.activeRgb = active
	d.workerStop = stop
	d.workerDone = done
	d.workerStopping = false
	d.workerStarts++
	go d.runWorker(snapshot.SelectedEffect, snapshot.EffectiveBrightness, profile, active, stop, done)
}

func (d *Device) runWorker(effect string, brightness uint8, profile rgb.Profile, active *rgb.ActiveRGB, stop <-chan struct{}, done chan struct{}) {
	defer func() {
		close(done)
		d.workerMutex.Lock()
		if d.workerDone != done {
			d.workerMutex.Unlock()
			return
		}
		d.clearWorkerOwnershipLocked(done)
		restart := d.workerRestartPending
		d.workerRestartPending = false
		d.workerMutex.Unlock()
		if restart {
			d.startWorker()
		}
	}()
	startTime := time.Now()
	rand.New(rand.NewSource(time.Now().UnixNano()))
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		controllers, channels := d.controllerSnapshot()
		if channels > 0 {
			buffer := d.generateRgbEffectFromProfile(channels, &startTime, effect, profile, brightness, active)
			distributeColorsToControllersUntilStopped(buffer, controllers, stop)
		}
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
	}
}

func (d *Device) setDeviceColor() {
	d.restartWorker()
}

func (d *Device) generateRgbEffect(channels int, startTime *time.Time, effect string, active *rgb.ActiveRGB) []byte {
	if !d.runtimeAvailable() {
		return make([]byte, channels*3)
	}
	resolution, err := d.resolver.Resolve(lightingsettings.RGBCluster(), effect)
	if err != nil {
		return make([]byte, channels*3)
	}
	return d.generateRgbEffectFromProfile(channels, startTime, effect, rgbProfileFromSettings(resolution.Settings), d.LightingSnapshot().EffectiveBrightness, active)
}

func (d *Device) generateRgbEffectFromProfile(channels int, startTime *time.Time, effect string, profile rgb.Profile, brightness uint8, active *rgb.ActiveRGB) []byte {
	if channels <= 0 {
		return []byte{}
	}
	if active == nil {
		return make([]byte, channels*3)
	}
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeCluster) {
		return make([]byte, channels*3)
	}
	if effect == "off" {
		return make([]byte, channels*3)
	}
	rgbModeSpeed := common.FClamp(profile.Speed, 0.1, 10)
	customColor := descriptor.PaletteKind != rgb.LightingPaletteGenerated && descriptor.PaletteKind != rgb.LightingPaletteNone
	normalizedBrightness := rgb.GetBrightnessValueFloat(brightness)
	renderer := rgb.New(
		channels,
		rgbModeSpeed,
		nil,
		nil,
		normalizedBrightness,
		common.Clamp(profile.Smoothness, 1, 100),
		time.Duration(rgbModeSpeed)*time.Second,
		customColor,
	)
	if customColor {
		profile.StartColor.Brightness = normalizedBrightness
		profile.MiddleColor.Brightness = normalizedBrightness
		profile.EndColor.Brightness = normalizedBrightness
		renderer.RGBStartColor = &profile.StartColor
		renderer.RGBMiddleColor = &profile.MiddleColor
		renderer.RGBEndColor = &profile.EndColor
	} else {
		renderer.RGBStartColor = active.RGBStartColor
		renderer.RGBEndColor = active.RGBEndColor
	}
	renderer.MinTemp = profile.MinTemp
	renderer.MaxTemp = profile.MaxTemp

	switch effect {
	case "rainbow":
		renderer.Rainbow(*startTime)
	case "spiralrainbow":
		renderer.SpiralRainbow(*startTime)
	case "pastelrainbow":
		renderer.PastelRainbow(*startTime)
	case "pastelspiralrainbow":
		renderer.PastelSpiralRainbow(*startTime)
	case "arc":
		renderer.Arc(*startTime)
	case "rain":
		renderer.Rain(*startTime)
	case "watercolor":
		renderer.Watercolor(*startTime)
	case "gradient":
		renderer.ColorshiftGradient(*startTime, profile.Gradients, profile.Speed)
	case "cpu-temperature":
		cpu, _ := d.temperatureSnapshot()
		renderer.Temperature(float64(cpu))
	case "gpu-temperature":
		_, gpu := d.temperatureSnapshot()
		renderer.Temperature(float64(gpu))
	case "colorpulse":
		renderer.Colorpulse(startTime)
	case "static":
		renderer.Static()
	case "rotator":
		renderer.Rotator(startTime)
	case "rotarystack":
		renderer.RotaryStack(startTime)
	case "wave":
		renderer.Wave(startTime)
	case "storm":
		renderer.Storm()
	case "flickering":
		renderer.Flickering(startTime)
	case "flame":
		renderer.Flame(startTime)
	case "aurora":
		renderer.Aurora(startTime)
	case "cyberpunkglitch":
		renderer.CyberpunkGlitch(startTime)
	case "tokyonight":
		renderer.TokyoNight(startTime)
	case "colorshift":
		renderer.Colorshift(startTime, active)
	case "circleshift":
		renderer.CircleShift(startTime)
	case "circle":
		renderer.Circle(startTime)
	case "spinner":
		renderer.Spinner(startTime)
	case "colorwarp":
		renderer.Colorwarp(startTime, active)
	case "nebula":
		renderer.Nebula(startTime)
	case "visor":
		renderer.Visor(startTime)
	case "comet":
		renderer.Comet(startTime)
	case "datastream":
		renderer.DataStream(startTime)
	case "plasmacore":
		renderer.PlasmaCore(startTime)
	case "stardust":
		renderer.Stardust(startTime)
	case "marquee":
		renderer.Marquee(startTime)
	case "sequential":
		renderer.Sequential(startTime)
	}
	return renderer.Output
}

func (d *Device) setTemperatures() {
	cpu := temperatures.GetCpuTemperature()
	gpu := temperatures.GetGpuTemperature()

	d.mutex.Lock()
	d.CpuTemp = cpu
	d.GpuTemp = gpu
	d.mutex.Unlock()
}

func (d *Device) temperatureSnapshot() (float32, float32) {
	if d == nil {
		return 0, 0
	}

	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.CpuTemp, d.GpuTemp
}

func (d *Device) setAutoRefresh() {
	if d == nil {
		return
	}
	d.mutex.Lock()
	if d.timer != nil || d.Exit || d.autoRefreshChan == nil {
		d.mutex.Unlock()
		return
	}
	timer := time.NewTicker(time.Duration(deviceRefreshInterval) * time.Millisecond)
	autoRefreshChan := d.autoRefreshChan
	d.timer = timer
	d.mutex.Unlock()
	go func() {
		for {
			select {
			case <-timer.C:
				d.mutex.RLock()
				exiting := d.Exit
				d.mutex.RUnlock()
				if exiting {
					return
				}
				d.setTemperatures()
			case <-autoRefreshChan:
				timer.Stop()
				return
			}
		}
	}()
}

func (d *Device) MigrateDeviceOrderSerial(oldSerial, newSerial string) {
	if !d.runtimeAvailable() || oldSerial == newSerial {
		return
	}
	layout, err := d.layout.Snapshot()
	if err != nil {
		return
	}
	modified := false
	migrated := make([]string, 0, len(layout.DeviceOrder))
	seen := make(map[string]struct{}, len(layout.DeviceOrder))
	for _, serial := range layout.DeviceOrder {
		if serial == oldSerial {
			serial = newSerial
			modified = true
		}
		if _, exists := seen[serial]; exists {
			continue
		}
		seen[serial] = struct{}{}
		migrated = append(migrated, serial)
	}
	if !modified {
		return
	}
	layout.DeviceOrder = migrated
	if err = d.layout.Set(layout); err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to migrate RGB Cluster layout serial")
		return
	}
	_ = d.refreshCompatibilityProjection()
	d.SortControllers()
	d.restartWorker()
}

func (d *Device) UpdateControllerProduct(serial string, product string) {
	if d == nil {
		return
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()
	for _, controller := range d.Controllers {
		if controller != nil && controller.Serial == serial {
			controller.Product = product
			break
		}
	}
}
