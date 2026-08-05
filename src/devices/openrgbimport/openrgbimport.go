package openrgbimport

import (
	"LumenForge/src/cluster"
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/dashboard"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/openrgb"
	"LumenForge/src/rgb"
	"LumenForge/src/temperatures"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// importerSoftwareEffectCatalogue returns a fresh stable-ID slice in the
// canonical descriptor presentation order.
func importerSoftwareEffectCatalogue() []string {
	descriptors := rgb.SoftwareEffectDescriptors()
	effects := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if descriptor.Scope.Includes(rgb.EffectScopeDevice) {
			effects = append(effects, descriptor.ID)
		}
	}
	return effects
}

const (
	hardwareBufferDrainDelay   = 75 * time.Millisecond
	initialEffectOutputTimeout = 5 * time.Second
)

var (
	configStoreMutex sync.Mutex
	configStorePath  = func() string {
		return config.GetPaths().OpenRGBImportFile
	}
	renameConfigStore            = os.Rename
	sendConfigFrame              = openrgb.SendFrame
	sendClusterFrame             = openrgb.SendFramePersistent
	sendLightingColor            = openrgb.SendColorContext
	sendLightingFrame            = openrgb.SendFrameContext
	sendLightingPersistentFrame  = openrgb.SendFramePersistent
	getLightingCPUTemperature    = temperatures.GetCpuTemperature
	getLightingNVIDIATemperature = temperatures.GetNVIDIAGpuTemperature
	getLightingAMDTemperature    = temperatures.GetAMDGpuTemperature
	checkConfigHealth            = openrgb.HealthCheck
	getConfigCluster             = cluster.Get
	deviceProfileDir             = func() string {
		return filepath.Join(config.GetPaths().MutableDataRoot, "database", "profiles")
	}
	initialEffectOutputContext = func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), initialEffectOutputTimeout)
	}
)

type ConfigStore struct {
	Devices map[string]DeviceConfig `json:"devices"`
}

type ZoneConfig struct {
	Name     string `json:"name"`
	LedCount int    `json:"ledCount"`
}

type DeviceConfig struct {
	Serial         string       `json:"serial"`
	Product        string       `json:"product,omitempty"`
	ExternalSerial string       `json:"externalSerial,omitempty"`
	Location       string       `json:"location,omitempty"`
	Vendor         string       `json:"vendor,omitempty"`
	Zones          []ZoneConfig `json:"zones"`
	Disabled       bool         `json:"disabled,omitempty"`
}

type ZoneColors struct {
	Color      *rgb.Color
	ColorIndex []int
	Name       string
}

type RGBOverride struct {
	Enabled        bool
	RGBStartColor  rgb.Color
	RGBEndColor    rgb.Color
	RGBMiddleColor rgb.Color
	RgbModeSpeed   float64
}

type DeviceProfile struct {
	Active           bool               `json:"Active"`
	Path             string             `json:"Path"`
	Product          string             `json:"Product"`
	Serial           string             `json:"Serial"`
	RGBProfile       string             `json:"-"`
	BrightnessSlider *uint8             `json:"-"`
	ZoneColors       map[int]ZoneColors `json:"ZoneColors"`
	RGBCluster       bool               `json:"RGBCluster"`
	RGBOverride      *RGBOverride       `json:"RGBOverride"`
}

type Device struct {
	Product            string
	Serial             string
	IsOpenRGB          bool
	DisplaySerial      string
	DisplaySerialLabel string
	instance           *common.Device
	controllerId       int
	colorCount         int
	LEDCount           int
	ZoneAmount         int
	Version            string
	Description        string
	Config             *DeviceConfig
	DeviceProfile      *DeviceProfile
	UserProfiles       map[string]*DeviceProfile
	Rgb                *rgb.RGB
	rgbMutex           sync.RWMutex
	RGBModes           []string
	lightingState      deviceLightingStateAccess
	lightingEffects    deviceLightingEffectAccess
	lightingResolver   deviceLightingResolverAccess

	brightness uint8
	lastColor  []byte

	effect      string
	speed       float64
	rgbRunner   *rgb.ActiveRGB
	stopChan    chan struct{}
	doneChan    chan struct{}
	running     bool
	openrgbConn net.Conn
	// Effect transitions acquire effectTransitionMu before mu and retain it
	// while stopEffectLoopLocked temporarily releases mu to wait for a worker.
	effectTransitionMu  sync.Mutex
	mu                  sync.Mutex
	clusterMutationMu   sync.Mutex
	lifecycleDetached   bool
	lifecycleActivating bool
}

// DeviceSnapshot is an immutable presentation/configuration view of an imported device.
// It intentionally excludes live connections, workers, channels, mutexes, and callbacks.
type DeviceSnapshot struct {
	Product            string
	Serial             string
	IsOpenRGB          bool
	DisplaySerial      string
	DisplaySerialLabel string
	LEDCount           int
	ZoneAmount         int
	Version            string
	Description        string
	Config             *DeviceConfig
	DeviceProfile      *DeviceProfile
	UserProfiles       map[string]*DeviceProfile
	Rgb                *rgb.RGB
	RGBModes           []string
	Effect             string `json:"-"`
	Speed              string `json:"-"`
	Brightness         uint8  `json:"-"`
	RGBCluster         bool   `json:"-"`
}

func isUsableDisplaySerial(value string) bool {
	v := sanitizeDisplaySerial(value)
	if v == "" {
		return false
	}

	lower := strings.ToLower(v)
	switch lower {
	case "0", "dir", "dire", "off", "on", "none", "n/a", "na", "null", "unknown", "default",
		"undefined", "unavailable", "not available", "not applicable", "no serial", "serial":
		return false
	}

	if strings.Trim(lower, "0- ") == "" ||
		strings.HasPrefix(lower, "hid:") ||
		strings.Contains(lower, "hidraw") ||
		strings.Contains(lower, "/dev/") ||
		strings.Contains(lower, `\\?\`) {
		return false
	}

	hasAlphaNum := false
	for _, r := range v {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			hasAlphaNum = true
			continue
		}
		switch r {
		case '-', '_', '.', ' ':
			continue
		default:
			return false
		}
	}

	if !hasAlphaNum {
		return false
	}

	return true
}

func sanitizeDisplaySerial(value string) string {
	v := strings.Map(func(r rune) rune {
		if r == '\uFFFD' {
			return -1
		}
		if unicode.IsControl(r) || !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, value)

	return strings.TrimSpace(v)
}

func usableExternalSerial(value string) string {
	serial := sanitizeDisplaySerial(value)
	if !isUsableDisplaySerial(serial) {
		return ""
	}
	return serial
}

func pickDisplaySerialAndLabel(dc openrgb.DiscoveredController) (string, string) {
	serial := sanitizeDisplaySerial(dc.Serial)
	if isUsableDisplaySerial(serial) {
		return serial, "SERIAL"
	}

	version := sanitizeDisplaySerial(dc.Version)
	if isUsableDisplaySerial(version) {
		return version, "VERSION"
	}

	hashInput := fmt.Sprintf("%s|%s|%s|%s|%d", dc.Name, dc.Vendor, dc.Version, dc.Description, len(dc.Zones))
	hash := sha256.Sum256([]byte(hashInput))
	fallback := fmt.Sprintf("ORGB-Import-%x", hash[:6])
	return fallback, "FALLBACK"
}

func getConfigPath() string {
	return configStorePath()
}

func emptyConfigStore() *ConfigStore {
	return &ConfigStore{Devices: make(map[string]DeviceConfig)}
}

func loadConfigStoreUnlocked(configPath string) (*ConfigStore, error) {
	store := &ConfigStore{
		Devices: make(map[string]DeviceConfig),
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, fmt.Errorf("read OpenRGB import store: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("OpenRGB import store is empty")
	}

	if err = json.Unmarshal(data, store); err != nil {
		return nil, fmt.Errorf("decode OpenRGB import store: %w", err)
	}

	if store.Devices == nil {
		store.Devices = make(map[string]DeviceConfig)
	}

	return store, nil
}

func loadConfigStore() (*ConfigStore, error) {
	configStoreMutex.Lock()
	defer configStoreMutex.Unlock()
	return loadConfigStoreUnlocked(getConfigPath())
}

func saveConfigStoreUnlocked(configPath string, store *ConfigStore) error {
	if store == nil {
		store = emptyConfigStore()
	}
	if store.Devices == nil {
		store.Devices = make(map[string]DeviceConfig)
	}
	for serial, device := range store.Devices {
		store.Devices[serial] = canonicalDeviceConfigForPersistence(device)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	temporary, err := os.CreateTemp(filepath.Dir(configPath), ".openrgbimport-zones-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err = temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err = temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = renameConfigStore(temporaryPath, configPath); err != nil {
		return err
	}

	return nil
}

func canonicalDeviceConfigForPersistence(cfg DeviceConfig) DeviceConfig {
	canonical := *cloneDeviceConfig(&cfg)
	canonical.ExternalSerial = usableExternalSerial(canonical.ExternalSerial)
	return canonical
}

func saveConfigStore(store *ConfigStore) error {
	configStoreMutex.Lock()
	defer configStoreMutex.Unlock()
	return saveConfigStoreUnlocked(getConfigPath(), store)
}

func updateConfigStore(update func(*ConfigStore) error) error {
	return updateConfigStoreIfChanged(func(store *ConfigStore) (bool, error) {
		return true, update(store)
	})
}

func updateConfigStoreIfChanged(update func(*ConfigStore) (bool, error)) error {
	configStoreMutex.Lock()
	defer configStoreMutex.Unlock()

	store, err := loadConfigStoreUnlocked(getConfigPath())
	if err != nil {
		return err
	}
	changed, err := update(store)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return saveConfigStoreUnlocked(getConfigPath(), store)
}

func getDeviceConfig(serial string) (*DeviceConfig, error) {
	store, err := loadConfigStore()
	if err != nil {
		return nil, err
	}
	if cfg, ok := store.Devices[serial]; ok {
		deviceCfg := cfg
		return &deviceCfg, nil
	}
	return nil, nil
}

func sanitizeZoneName(name string) string {
	v := strings.Map(func(r rune) rune {
		if r == '\uFFFD' {
			return -1
		}
		if unicode.IsControl(r) || !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, name)
	return strings.TrimSpace(v)
}

func buildDefaultDeviceConfig(serial string, dc openrgb.DiscoveredController) *DeviceConfig {
	cfg := &DeviceConfig{Serial: serial, Product: dc.Name}

	if len(dc.Zones) > 0 {
		zoneLimit := len(dc.Zones)
		if zoneLimit > 128 {
			zoneLimit = 128
		}
		cfg.Zones = make([]ZoneConfig, zoneLimit)
		totalLeds := 0
		for i := 0; i < zoneLimit; i++ {
			z := dc.Zones[i]
			name := sanitizeZoneName(z.Name)
			if name == "" {
				name = fmt.Sprintf("Zone %d", i+1)
			}
			ledCount := z.LEDCount
			if ledCount <= 0 {
				ledCount = 1
			} else if ledCount > 1024 {
				ledCount = 1024
			}
			if totalLeds+ledCount > 4096 {
				ledCount = 4096 - totalLeds
				if ledCount <= 0 {
					ledCount = 1
				}
			}
			totalLeds += ledCount
			cfg.Zones[i] = ZoneConfig{
				Name:     name,
				LedCount: ledCount,
			}
		}
		return cfg
	}

	cfg.Zones = []ZoneConfig{
		{Name: "Zone 1", LedCount: 1},
	}

	return cfg
}

func configLedCount(cfg *DeviceConfig) int {
	if cfg == nil {
		return 0
	}

	total := 0
	for _, zone := range cfg.Zones {
		if zone.LedCount > 0 {
			total += zone.LedCount
		}
	}
	return total
}

func validateDeviceConfig(targetSerial string, input DeviceConfig, allowLegacyEmptySerial bool) (DeviceConfig, error) {
	if !common.AlphanumericDashRegex.MatchString(targetSerial) {
		return DeviceConfig{}, fmt.Errorf("OpenRGB import %q has an unusable internal serial; expected only letters, numbers, and dashes", targetSerial)
	}

	validated := input
	validated.Zones = append([]ZoneConfig(nil), input.Zones...)

	if validated.Serial == "" {
		if !allowLegacyEmptySerial {
			return DeviceConfig{}, fmt.Errorf("OpenRGB import %q has an empty internal serial; expected %q", targetSerial, targetSerial)
		}
		validated.Serial = targetSerial
	} else if validated.Serial != targetSerial {
		return DeviceConfig{}, fmt.Errorf("OpenRGB import %q stores conflicting internal serial %q; expected %q", targetSerial, validated.Serial, targetSerial)
	}

	if len(validated.Zones) < 1 || len(validated.Zones) > 128 {
		return DeviceConfig{}, fmt.Errorf("OpenRGB import %q has %d zones; expected 1 through 128", targetSerial, len(validated.Zones))
	}

	total := 0
	for index, zone := range validated.Zones {
		if zone.LedCount < 1 || zone.LedCount > 1024 {
			return DeviceConfig{}, fmt.Errorf("OpenRGB import %q zone %d has %d LEDs; expected 1 through 1024", targetSerial, index+1, zone.LedCount)
		}
		if zone.LedCount > 4096-total {
			return DeviceConfig{}, fmt.Errorf("OpenRGB import %q zone %d has %d LEDs and would exceed the permitted total range of 1 through 4096", targetSerial, index+1, zone.LedCount)
		}
		total += zone.LedCount

		name := sanitizeZoneName(zone.Name)
		if name == "" {
			name = fmt.Sprintf("Zone %d", index+1)
		}
		validated.Zones[index].Name = name
	}
	if total < 1 {
		return DeviceConfig{}, fmt.Errorf("OpenRGB import %q has a total of %d LEDs; expected 1 through 4096", targetSerial, total)
	}

	return validated, nil
}

func validateStoredDeviceConfig(mapSerial string, cfg DeviceConfig) (DeviceConfig, error) {
	return validateDeviceConfig(mapSerial, cfg, true)
}

func validateConfiguredStore(store *ConfigStore) error {
	for serial, cfg := range store.Devices {
		validated, err := validateStoredDeviceConfig(serial, cfg)
		if err != nil {
			return err
		}
		store.Devices[serial] = validated
	}
	return nil
}

func validatedConfigForController(cfg *DeviceConfig, _ openrgb.DiscoveredController) (*DeviceConfig, bool) {
	if cfg == nil {
		return nil, false
	}
	validated, err := validateDeviceConfig(cfg.Serial, *cfg, false)
	if err != nil {
		return nil, false
	}
	return &validated, true
}

func isConfigValidForController(cfg *DeviceConfig, dc openrgb.DiscoveredController) bool {
	_, valid := validatedConfigForController(cfg, dc)
	return valid
}

func resolveDeviceConfig(serial string, dc openrgb.DiscoveredController) *DeviceConfig {
	cfg, err := getDeviceConfig(serial)
	if err != nil {
		logger.Log(logger.Fields{"error": err, "serial": serial}).Error("Unable to load OpenRGB import configuration")
		return nil
	}
	if validated, valid := validatedConfigForController(cfg, dc); valid {
		cfg = validated
		if cfg.Product != dc.Name {
			cfg.Product = dc.Name
			if err := updateConfigStore(func(store *ConfigStore) error {
				stored := store.Devices[serial]
				stored.Product = dc.Name
				store.Devices[serial] = stored
				return nil
			}); err != nil {
				logger.Log(logger.Fields{"error": err, "serial": serial}).Error("Unable to update OpenRGB import configuration")
			}
		}
		return cfg
	}

	cfg = buildDefaultDeviceConfig(serial, dc)
	if validated, valid := validatedConfigForController(cfg, dc); valid {
		cfg = validated
	} else {
		logger.Log(logger.Fields{
			"serial":  serial,
			"product": dc.Name,
		}).Warn("OpenRGB controller returned invalid layout config. Enforcing fallback layout: 1 zone, 1 LED.")
		cfg = &DeviceConfig{
			Serial:  serial,
			Product: dc.Name,
			Zones: []ZoneConfig{
				{Name: "Zone 1", LedCount: 1},
			},
		}
	}

	if err := updateConfigStore(func(store *ConfigStore) error {
		store.Devices[serial] = *cloneDeviceConfig(cfg)
		return nil
	}); err != nil {
		logger.Log(logger.Fields{"error": err, "serial": serial}).Error("Unable to save OpenRGB import configuration")
	}

	return cfg
}

func buildZoneColorsFromConfig(cfg *DeviceConfig, defaultColor []byte) map[int]ZoneColors {
	zoneColors := make(map[int]ZoneColors)

	red := float64(99)
	green := float64(213)
	blue := float64(255)
	if len(defaultColor) >= 3 {
		red = float64(defaultColor[0])
		green = float64(defaultColor[1])
		blue = float64(defaultColor[2])
	}

	ledOffset := 0
	for zoneIndex, zoneCfg := range cfg.Zones {
		colorIndex := make([]int, 0, zoneCfg.LedCount*3)
		for led := 0; led < zoneCfg.LedCount; led++ {
			base := (ledOffset + led) * 3
			colorIndex = append(colorIndex, base, base+1, base+2)
		}

		zoneColors[zoneIndex] = ZoneColors{
			Color: &rgb.Color{
				Red:        red,
				Green:      green,
				Blue:       blue,
				Brightness: 1,
				Hex:        fmt.Sprintf("#%02x%02x%02x", int(red), int(green), int(blue)),
			},
			ColorIndex: colorIndex,
			Name:       zoneCfg.Name,
		}

		ledOffset += zoneCfg.LedCount
	}

	return zoneColors
}

func cloneDeviceConfig(cfg *DeviceConfig) *DeviceConfig {
	if cfg == nil {
		return nil
	}

	cloned := &DeviceConfig{
		Serial:         cfg.Serial,
		Product:        cfg.Product,
		ExternalSerial: cfg.ExternalSerial,
		Location:       cfg.Location,
		Vendor:         cfg.Vendor,
		Zones:          append([]ZoneConfig(nil), cfg.Zones...),
		Disabled:       cfg.Disabled,
	}
	return cloned
}

func cloneDeviceProfile(profile *DeviceProfile) *DeviceProfile {
	if profile == nil {
		return nil
	}
	cloned := *profile
	if profile.BrightnessSlider != nil {
		brightness := *profile.BrightnessSlider
		cloned.BrightnessSlider = &brightness
	}
	if profile.RGBOverride != nil {
		override := *profile.RGBOverride
		cloned.RGBOverride = &override
	}
	if profile.ZoneColors != nil {
		cloned.ZoneColors = make(map[int]ZoneColors, len(profile.ZoneColors))
		for index, zone := range profile.ZoneColors {
			zoneCopy := zone
			if zone.Color != nil {
				colorCopy := *zone.Color
				zoneCopy.Color = &colorCopy
			}
			zoneCopy.ColorIndex = append([]int(nil), zone.ColorIndex...)
			cloned.ZoneColors[index] = zoneCopy
		}
	}
	return &cloned
}

func cloneRGBState(state *rgb.RGB) *rgb.RGB {
	if state == nil {
		return nil
	}
	cloned := *state
	if state.Profiles != nil {
		cloned.Profiles = make(map[string]rgb.Profile, len(state.Profiles))
		for name, profile := range state.Profiles {
			cloned.Profiles[name] = cloneRGBProfile(profile)
		}
	}
	return &cloned
}

func cloneRGBProfile(profile rgb.Profile) rgb.Profile {
	cloned := profile
	if profile.Gradients != nil {
		cloned.Gradients = make(map[int]rgb.Color, len(profile.Gradients))
		for index, color := range profile.Gradients {
			cloned.Gradients[index] = color
		}
	}
	return cloned
}

func hasLEDCountIncrease(savedCfg *DeviceConfig, newCfg *DeviceConfig) bool {
	if newCfg == nil {
		return false
	}

	for i, zone := range newCfg.Zones {
		savedLEDCount := 0
		if savedCfg != nil && i < len(savedCfg.Zones) {
			savedLEDCount = savedCfg.Zones[i].LedCount
		}
		if zone.LedCount > savedLEDCount {
			return true
		}
	}

	return false
}

func (d *Device) applyConfigLocked(cfg *DeviceConfig, brightness uint8) {
	if cfg == nil {
		d.Config = nil
		d.colorCount = 0
		d.ZoneAmount = 0
		d.DeviceProfile = nil
		d.effect = defaultDeviceLightingEffect
		return
	}

	var wasCluster bool
	if d.DeviceProfile != nil {
		wasCluster = d.DeviceProfile.RGBCluster
	}

	d.Config = cloneDeviceConfig(cfg)
	d.colorCount = configLedCount(cfg)
	d.ZoneAmount = len(cfg.Zones)
	d.DeviceProfile = &DeviceProfile{
		RGBProfile:       defaultDeviceLightingEffect,
		BrightnessSlider: &brightness,
		ZoneColors:       buildZoneColorsFromConfig(cfg, d.lastColor),
		RGBCluster:       wasCluster,
	}
	d.effect = defaultDeviceLightingEffect
}

func checkOpenRGBStable(attempts int, delay time.Duration) error {
	for i := 0; i < attempts; i++ {
		if i > 0 {
			time.Sleep(delay)
		}
		if err := checkConfigHealth(); err != nil {
			return err
		}
	}

	return nil
}

func (d *Device) resolveControllerId() {
	if d.controllerId < 0 && !d.lifecycleInactiveLocked() {
		requestReconciliation()
	}
}

func (d *Device) lifecycleInactiveLocked() bool {
	return d.lifecycleDetached || d.lifecycleActivating
}

func (d *Device) lifecycleMutationErrorLocked() error {
	if d.lifecycleDetached {
		return fmt.Errorf("OpenRGB import %q is detached", d.Serial)
	}
	return fmt.Errorf("OpenRGB import %q is still activating", d.Serial)
}

func (d *Device) finishLifecycleActivation() {
	d.mu.Lock()
	if d.lifecycleActivating && d.controllerId >= 0 {
		d.controllerId = -1
		if d.openrgbConn != nil {
			_ = d.openrgbConn.Close()
			d.openrgbConn = nil
		}
	}
	d.lifecycleActivating = false
	d.mu.Unlock()
}

func (d *Device) SaveDeviceConfig(cfg *DeviceConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}

	d.mu.Lock()
	if d.lifecycleInactiveLocked() {
		err := d.lifecycleMutationErrorLocked()
		d.mu.Unlock()
		return err
	}
	d.mu.Unlock()

	validated, err := validateDeviceConfig(d.Serial, *cfg, false)
	if err != nil {
		return err
	}

	total := configLedCount(&validated)
	if total <= 0 {
		return fmt.Errorf("OpenRGB import %q has a total of %d LEDs; expected 1 through 4096", d.Serial, total)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lifecycleInactiveLocked() {
		return d.lifecycleMutationErrorLocked()
	}

	savedCfg, err := getDeviceConfig(d.Serial)
	if err != nil {
		return err
	}
	if savedCfg != nil {
		validated.Product = savedCfg.Product
		validated.ExternalSerial = savedCfg.ExternalSerial
		validated.Location = savedCfg.Location
		validated.Vendor = savedCfg.Vendor
		validated.Disabled = savedCfg.Disabled
	} else {
		validated.Disabled = false
		if validated.Product == "" {
			validated.Product = d.Product
		}
	}
	riskyIncrease := hasLEDCountIncrease(savedCfg, &validated)

	previousCfg := cloneDeviceConfig(d.Config)
	previousProfile := cloneDeviceProfile(d.DeviceProfile)
	if d.lightingState == nil {
		return fmt.Errorf("OpenRGB device lighting state is unavailable")
	}
	previousPersistedLightingState, previousLightingStateFound, err := d.lightingState.Resolve(d.Serial)
	if err != nil {
		return fmt.Errorf("load device lighting state before configuration save: %w", err)
	}
	previousEffectiveLightingState := previousPersistedLightingState
	if !previousLightingStateFound {
		previousEffectiveLightingState = DefaultDeviceLightingState()
	}
	brightness := previousEffectiveLightingState.Brightness
	temporaryLightingState := DeviceLightingState{
		SelectedEffect: defaultDeviceLightingEffect,
		Brightness:     brightness,
	}
	if err = d.lightingState.Set(d.Serial, temporaryLightingState); err != nil {
		return fmt.Errorf("save Static effect selection for device configuration: %w", err)
	}
	restoreInMemory := func(state DeviceLightingState) {
		d.applyConfigLocked(previousCfg, state.Brightness)
		if previousProfile != nil {
			d.DeviceProfile = cloneDeviceProfile(previousProfile)
		}
		d.effect = state.SelectedEffect
		d.brightness = state.Brightness
		if d.DeviceProfile != nil {
			d.DeviceProfile.RGBProfile = state.SelectedEffect
			value := state.Brightness
			d.DeviceProfile.BrightnessSlider = &value
		}
	}
	rollback := func(cause error) error {
		var rollbackErr error
		if previousLightingStateFound {
			rollbackErr = d.lightingState.Set(d.Serial, previousPersistedLightingState)
		} else {
			_, rollbackErr = d.lightingState.Delete(d.Serial)
		}
		if rollbackErr != nil {
			restoreInMemory(temporaryLightingState)
			return errors.Join(cause, fmt.Errorf("restore device lighting state after configuration failure: %w", rollbackErr))
		}
		restoreInMemory(previousEffectiveLightingState)
		return cause
	}

	d.stopEffectLoopLocked()
	d.applyConfigLocked(&validated, brightness)

	d.resolveControllerId()

	if d.controllerId >= 0 {
		time.Sleep(hardwareBufferDrainDelay)
		frame, frameErr := d.buildStaticFrame(brightness)
		if frameErr != nil {
			return rollback(fmt.Errorf("resolve Static output after device configuration: %w", frameErr))
		}
		if err := sendConfigFrame(uint32(d.controllerId), frame); err != nil {
			d.recordOutputFailureLocked(err)
			return rollback(err)
		}
		if riskyIncrease {
			if err := checkOpenRGBStable(4, 500*time.Millisecond); err != nil {
				return rollback(fmt.Errorf("OpenRGB became unavailable after applying increased LED counts; config was not saved. Confirm zone and LED counts in OpenRGB and try again"))
			}
		}
	}

	if err := updateConfigStore(func(store *ConfigStore) error {
		if current, ok := store.Devices[d.Serial]; ok {
			validated.ExternalSerial = current.ExternalSerial
			validated.Location = current.Location
			validated.Vendor = current.Vendor
			validated.Disabled = current.Disabled
			if validated.Product == "" {
				validated.Product = current.Product
			}
		} else {
			validated.Disabled = false
		}
		store.Devices[d.Serial] = canonicalDeviceConfigForPersistence(validated)
		return nil
	}); err != nil {
		return rollback(err)
	}

	if d.DeviceProfile != nil {
		d.saveDeviceProfile()
		if d.DeviceProfile.RGBCluster {
			clusterController := d.clusterControllerLocked()
			d.mu.Unlock()
			d.replaceClusterController(clusterController)
			d.mu.Lock()
		}
	}

	return nil
}

func newOfflineDevice(serial string, cfg DeviceConfig, runtime *deviceLightingRuntime) (*Device, error) {
	colorCount := configLedCount(&cfg)
	productName := strings.TrimSpace(cfg.Product)
	if productName == "" {
		productName = "Imported OpenRGB Device"
	}

	d := &Device{
		Product:            productName,
		Serial:             serial,
		IsOpenRGB:          true,
		DisplaySerial:      "",
		DisplaySerialLabel: "",
		controllerId:       -1,
		colorCount:         colorCount,
		brightness:         100,
		lastColor:          []byte{99, 213, 255},
		effect:             "static",
		speed:              2.0,
		stopChan:           nil,
		doneChan:           nil,
		running:            false,
		Config:             cloneDeviceConfig(&cfg),
		ZoneAmount:         len(cfg.Zones),
		LEDCount:           colorCount,
	}

	d.RGBModes = importerSoftwareEffectCatalogue()
	if err := d.attachLightingRuntime(runtime); err != nil {
		return nil, err
	}

	defaultBrightness := uint8(100)
	d.DeviceProfile = &DeviceProfile{
		Active:           true,
		RGBProfile:       "static",
		BrightnessSlider: &defaultBrightness,
		ZoneColors:       buildZoneColorsFromConfig(&cfg, d.lastColor),
	}
	d.loadDeviceProfiles()
	if d.DeviceProfile != nil {
		d.DeviceProfile.RGBProfile = d.effect
		brightness := d.brightness
		d.DeviceProfile.BrightnessSlider = &brightness
	}
	d.setupClusterController()

	return d, nil
}

func InitAll() []*common.Device {
	store, err := loadConfigStore()
	if err != nil {
		openrgb.SetDisconnected(err)
		logger.Log(logger.Fields{"error": err, "location": getConfigPath()}).Error("Unable to load OpenRGB import store")
		setConfiguredDevices(nil)
		return nil
	}

	if len(store.Devices) == 0 {
		openrgb.SetNotConfigured()
		setConfiguredDevices(nil)
		return nil
	}
	if err = validateConfiguredStore(store); err != nil {
		openrgb.SetDisconnected(err)
		logger.Log(logger.Fields{"error": err, "location": getConfigPath()}).Error("Invalid OpenRGB import store")
		setConfiguredDevices(nil)
		return nil
	}

	serials := make([]string, 0, len(store.Devices))
	for serial, cfg := range store.Devices {
		if cfg.Disabled {
			continue
		}
		serials = append(serials, serial)
	}
	if len(serials) == 0 {
		openrgb.SetNotConfigured()
		setConfiguredDevices(nil)
		return nil
	}
	openrgb.SetDisconnected(nil)
	sort.Strings(serials)
	runtime, err := loadDeviceLightingRuntime(config.GetPaths())
	if err != nil {
		openrgb.SetDisconnected(err)
		logger.Log(logger.Fields{"error": err}).Error("Unable to load OpenRGB device lighting state")
		setConfiguredDevices(nil)
		return nil
	}

	configured := make(map[string]*Device, len(serials))
	result := make([]*common.Device, 0, len(serials))
	for _, serial := range serials {
		d, deviceErr := newOfflineDevice(serial, store.Devices[serial], runtime)
		if deviceErr != nil {
			logger.Log(logger.Fields{"error": deviceErr, "serial": serial}).Error("Unable to initialize OpenRGB device lighting state")
			continue
		}
		d.createDevice()
		d.instance.Unavailable = true
		configured[serial] = d
		result = append(result, d.instance)
	}
	setConfiguredDevices(configured)
	return result
}

func migrateDeviceData(dc openrgb.DiscoveredController, newSerial string) {
	var candidateSerial string
	err := updateConfigStoreIfChanged(func(store *ConfigStore) (bool, error) {
		// Search order 1: Look for any older hash-based serial (openrgb-hash-*) with same product name
		for s, cfg := range store.Devices {
			if strings.HasPrefix(s, "openrgb-hash-") && cfg.Product == dc.Name && s != newSerial {
				candidateSerial = s
				break
			}
		}

		// Search order 2: Look for the specific ID-based serial (openrgb-import-ID)
		if candidateSerial == "" {
			oldImportSerial := fmt.Sprintf("openrgb-import-%d", dc.ID)
			if _, exists := store.Devices[oldImportSerial]; exists {
				candidateSerial = oldImportSerial
			}
		}

		// Search order 3: Look for any other entry with same product name
		if candidateSerial == "" {
			for s, cfg := range store.Devices {
				if cfg.Product == dc.Name && s != newSerial {
					candidateSerial = s
					break
				}
			}
		}

		if candidateSerial == "" {
			return false, nil
		}

		oldCfg := store.Devices[candidateSerial]
		oldCfg.Serial = newSerial
		store.Devices[newSerial] = oldCfg
		delete(store.Devices, candidateSerial)
		return true, nil
	})
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to save migrated OpenRGB import store")
		return
	}
	if candidateSerial == "" {
		return
	}

	logger.Log(logger.Fields{
		"oldSerial": candidateSerial,
		"newSerial": newSerial,
		"product":   dc.Name,
	}).Info("Migrating OpenRGB device config and profiles to new persistent serial")

	// 2. Migrate dashboard settings
	dashboard.MigrateDeviceSerial(candidateSerial, newSerial)
	if cluster.Get() != nil {
		cluster.Get().MigrateDeviceOrderSerial(candidateSerial, newSerial)
	}

	// 3. Migrate profile files in database/profiles/
	profileDir := filepath.Join(config.GetPaths().MutableDataRoot, "database", "profiles")
	files, err := os.ReadDir(profileDir)
	if err != nil {
		return
	}

	for _, fi := range files {
		if fi.IsDir() {
			continue
		}
		if !common.IsValidExtension(fi.Name(), ".json") {
			continue
		}

		fileName := strings.TrimSuffix(fi.Name(), ".json")
		var newFileName string
		if fileName == candidateSerial {
			newFileName = newSerial + ".json"
		} else if strings.HasPrefix(fileName, candidateSerial+"-") {
			newFileName = newSerial + "-" + strings.TrimPrefix(fileName, candidateSerial+"-") + ".json"
		} else {
			continue
		}

		oldPath := filepath.Join(profileDir, fi.Name())
		newPath := filepath.Join(profileDir, newFileName)

		data, err := os.ReadFile(oldPath)
		if err != nil {
			continue
		}

		var pf DeviceProfile
		if err := json.Unmarshal(data, &pf); err != nil {
			continue
		}

		pf.Serial = newSerial
		pf.Path = newPath

		newData, err := json.MarshalIndent(pf, "", "  ")
		if err != nil {
			continue
		}

		if err := os.WriteFile(newPath, newData, 0o644); err == nil {
			_ = os.Remove(oldPath)
		}
	}
}

func (d *Device) resolveDeviceIcon() string {
	nameLower := strings.ToLower(d.Product)
	descLower := strings.ToLower(d.Description)

	if strings.Contains(descLower, "motherboard") || strings.Contains(nameLower, "motherboard") || strings.Contains(nameLower, "z690") || strings.Contains(nameLower, "x570") || strings.Contains(nameLower, "z790") || strings.Contains(nameLower, "b650") {
		return "icon-motherboard.svg"
	}
	if strings.Contains(descLower, "gpu") || strings.Contains(descLower, "vga") || strings.Contains(nameLower, "geforce") || strings.Contains(nameLower, "radeon") {
		return "icon-device.svg"
	}
	if strings.Contains(descLower, "dram") || strings.Contains(nameLower, "ram") || strings.Contains(nameLower, "memory") || strings.Contains(nameLower, "ddr4") || strings.Contains(nameLower, "ddr5") {
		return "icon-ram.svg"
	}
	if strings.Contains(descLower, "keyboard") || strings.Contains(nameLower, "keyboard") {
		return "icon-keyboard.svg"
	}
	if strings.Contains(descLower, "mouse") || strings.Contains(nameLower, "mouse") {
		return "icon-mouse.svg"
	}
	if strings.Contains(nameLower, "strimer") || strings.Contains(nameLower, "controller") || strings.Contains(nameLower, "hub") || strings.Contains(nameLower, "node") || strings.Contains(nameLower, "commander") {
		return "icon-controller.svg"
	}

	return "icon-rgb.svg"
}

func (d *Device) createDevice() {
	d.instance = &common.Device{
		ProductType: common.ProductTypeMotherboard,
		Product:     d.Product,
		Serial:      d.Serial,
		Firmware:    "",
		Image:       d.resolveDeviceIcon(),
		Instance:    d,
		GetDevice:   d,
	}
}

// Snapshot returns a race-safe immutable copy for WebUI and JSON presentation.
func (d *Device) Snapshot() DeviceSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()

	effect := d.effect
	if effect == "" {
		effect = "static"
	}
	resolvedSpeed := d.speed
	if descriptor, ok := rgb.SoftwareEffectDescriptorByID(effect); ok && descriptor.SupportsSpeed {
		if profile := d.GetRgbProfile(effect); profile != nil {
			resolvedSpeed = profile.Speed
		}
	}
	speed := "normal"
	switch resolvedSpeed {
	case 4.0:
		speed = "slow"
	case 0.8:
		speed = "fast"
	}

	var userProfiles map[string]*DeviceProfile
	if d.UserProfiles != nil {
		userProfiles = make(map[string]*DeviceProfile, len(d.UserProfiles))
		for name, profile := range d.UserProfiles {
			userProfiles[name] = cloneDeviceProfile(profile)
		}
	}
	rgbCluster := d.DeviceProfile != nil && d.DeviceProfile.RGBCluster

	var rgbState *rgb.RGB
	if resolved := d.GetRgbProfiles(); resolved != nil {
		if state, ok := resolved.(rgb.RGB); ok {
			rgbState = cloneRGBState(&state)
		}
	}
	presentationProfile := cloneDeviceProfile(d.DeviceProfile)
	if presentationProfile != nil {
		presentationProfile.RGBProfile = effect
		brightness := d.brightness
		presentationProfile.BrightnessSlider = &brightness
	}

	return DeviceSnapshot{
		Product:            d.Product,
		Serial:             d.Serial,
		IsOpenRGB:          d.IsOpenRGB,
		DisplaySerial:      d.DisplaySerial,
		DisplaySerialLabel: d.DisplaySerialLabel,
		LEDCount:           d.LEDCount,
		ZoneAmount:         d.ZoneAmount,
		Version:            d.Version,
		Description:        d.Description,
		Config:             cloneDeviceConfig(d.Config),
		DeviceProfile:      presentationProfile,
		UserProfiles:       userProfiles,
		Rgb:                rgbState,
		RGBModes:           append([]string(nil), d.RGBModes...),
		Effect:             effect,
		Speed:              speed,
		Brightness:         d.brightness,
		RGBCluster:         rgbCluster,
	}
}

// MatchesOpenRGBImport verifies the immutable importer identity and current lifecycle state.
func (d *Device) MatchesOpenRGBImport(serial string) bool {
	if d == nil {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return serial != "" && d.Serial == serial && d.IsOpenRGB && !d.lifecycleInactiveLocked()
}

// SupportsEffect reports whether the importer currently exposes effect.
func (d *Device) SupportsEffect(effect string) bool {
	if d == nil || effect == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Contains(d.RGBModes, effect)
}

func (d *Device) GetDeviceTemplate() string {
	return "openrgb.html"
}

func (d *Device) ControllerID() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.controllerId
}

func (d *Device) bindController(dc openrgb.DiscoveredController) bool {
	d.mu.Lock()
	if d.lifecycleDetached {
		d.mu.Unlock()
		return false
	}
	changed := d.controllerId != dc.ID
	if changed {
		d.stopEffectLoopLocked()
		if d.openrgbConn != nil {
			_ = d.openrgbConn.Close()
			d.openrgbConn = nil
		}
	}

	d.controllerId = dc.ID
	d.LEDCount = dc.LEDCount
	d.Version = dc.Version
	d.Description = dc.Description
	if strings.TrimSpace(dc.Name) != "" {
		d.Product = dc.Name
	}
	d.DisplaySerial, d.DisplaySerialLabel = pickDisplaySerialAndLabel(dc)
	d.updateIdentityMetadataLocked(dc)
	clusterController := d.clusterControllerLocked()
	d.mu.Unlock()

	d.registerClusterController(clusterController)
	return changed
}

func (d *Device) wrapperPresentation() (product, firmware, image string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.Product, d.Version, d.resolveDeviceIcon()
}

func (d *Device) markUnavailable() bool {
	d.mu.Lock()
	changed := d.controllerId >= 0 || d.openrgbConn != nil || d.running
	d.stopEffectLoopLocked()
	d.controllerId = -1
	if d.openrgbConn != nil {
		_ = d.openrgbConn.Close()
		d.openrgbConn = nil
	}
	d.mu.Unlock()
	return changed
}

func (d *Device) updateIdentityMetadata(dc openrgb.DiscoveredController) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.updateIdentityMetadataLocked(dc)
}

func (d *Device) updateIdentityMetadataLocked(dc openrgb.DiscoveredController) {
	if d.Config == nil {
		return
	}
	d.Config.ExternalSerial = safePresentationString(usableExternalSerial(d.Config.ExternalSerial), 512)
	if externalSerial := usableExternalSerial(dc.Serial); externalSerial != "" {
		d.Config.ExternalSerial = safePresentationString(externalSerial, 512)
	}
	if location := safePresentationString(dc.Location, 512); location != "" {
		d.Config.Location = location
	}
	if vendor := safePresentationString(dc.Vendor, 512); vendor != "" {
		d.Config.Vendor = vendor
	}
	if product := safePresentationString(dc.Name, 512); product != "" {
		d.Config.Product = product
	}
}

func (d *Device) resumeDesiredState(ctx context.Context) error {
	d.mu.Lock()
	if d.lifecycleInactiveLocked() {
		d.mu.Unlock()
		return nil
	}
	if d.controllerId < 0 || d.DeviceProfile == nil || d.DeviceProfile.RGBCluster {
		d.mu.Unlock()
		return nil
	}
	effect := d.effect
	if effect == "" {
		effect = defaultDeviceLightingEffect
	}
	d.mu.Unlock()

	return d.setEffectContext(ctx, effect, false, false, false)
}

func (d *Device) recordOutputFailureLocked(err error) {
	if d.openrgbConn != nil {
		_ = d.openrgbConn.Close()
		d.openrgbConn = nil
	}
	d.controllerId = -1
	openrgb.SetDisconnected(err)
	reportOutputFailure(d, err)
}

func (d *Device) handleOutputFailure(err error) {
	if err == nil {
		return
	}
	d.mu.Lock()
	d.stopEffectLoopLocked()
	d.recordOutputFailureLocked(err)
	d.mu.Unlock()
}

func applyBrightnessValue(rgbBytes []byte, brightness uint8) []byte {
	if len(rgbBytes) < 3 {
		return []byte{0, 0, 0}
	}

	b := int(brightness)
	out := make([]byte, len(rgbBytes))
	for i := 0; i+2 < len(rgbBytes); i += 3 {
		out[i] = byte((int(rgbBytes[i]) * b) / 100)
		out[i+1] = byte((int(rgbBytes[i+1]) * b) / 100)
		out[i+2] = byte((int(rgbBytes[i+2]) * b) / 100)
	}
	return out
}

func (d *Device) stopEffectLoopLocked() {
	if d.running && d.stopChan != nil {
		stop := d.stopChan
		done := d.doneChan
		d.stopChan = nil
		d.doneChan = nil
		d.running = false

		close(stop)

		d.mu.Unlock()
		if done != nil {
			<-done
		}
		d.mu.Lock()
	}
}

// buildStaticFrame resolves the complete canonical Static settings and fills
// the independent device scope uniformly. Legacy profile colors and RGB
// Override remain presentation-only compatibility data and are not consulted.
func (d *Device) buildStaticFrame(brightness uint8) ([]byte, error) {
	resolution, err := d.resolveLightingSettings(defaultDeviceLightingEffect)
	if err != nil {
		return nil, err
	}
	if resolution.Settings.SingleColor == nil {
		return nil, fmt.Errorf("resolved Static settings do not contain a single color")
	}

	color := resolution.Settings.SingleColor.Color
	frame := make([]byte, d.colorCount*3)
	for offset := 0; offset+2 < len(frame); offset += 3 {
		frame[offset] = byte(color.Red)
		frame[offset+1] = byte(color.Green)
		frame[offset+2] = byte(color.Blue)
	}
	if len(frame) == 0 {
		return frame, nil
	}
	return applyBrightnessValue(frame, brightness), nil
}

func (d *Device) SetBrightness(brightness uint8) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lifecycleInactiveLocked() {
		return d.lifecycleMutationErrorLocked()
	}

	d.resolveControllerId()
	if d.controllerId < 0 {
		return fmt.Errorf("controllerId not set")
	}
	if d.DeviceProfile != nil && d.DeviceProfile.RGBCluster {
		return fmt.Errorf("device is controlled by RGB cluster")
	}

	if brightness > 100 {
		return fmt.Errorf("brightness must be between 0 and 100")
	}

	if d.lightingState == nil {
		return fmt.Errorf("OpenRGB device lighting state is unavailable")
	}
	var staticFrame []byte
	if d.effect == defaultDeviceLightingEffect {
		var err error
		staticFrame, err = d.buildStaticFrame(brightness)
		if err != nil {
			return fmt.Errorf("resolve Static output: %w", err)
		}
	}

	if err := d.lightingState.Set(d.Serial, DeviceLightingState{
		SelectedEffect: d.effect,
		Brightness:     brightness,
	}); err != nil {
		return fmt.Errorf("save brightness: %w", err)
	}
	d.brightness = brightness
	if d.DeviceProfile != nil {
		d.DeviceProfile.BrightnessSlider = &brightness
	}

	// If an effect is running, let the effect loop pick up the new brightness.
	if d.running {
		return nil
	}

	var frame []byte
	if d.effect == "static" {
		frame = staticFrame
	}
	if frame != nil {
		err := sendLightingFrame(context.Background(), uint32(d.controllerId), frame)
		if err != nil {
			d.recordOutputFailureLocked(err)
		}
		return err
	}

	if d.effect != "off" {
		return nil
	}
	err := sendLightingColor(context.Background(), uint32(d.controllerId), d.colorCount, []byte{0, 0, 0})
	if err != nil {
		d.recordOutputFailureLocked(err)
	}
	return err
}

// acquireEffectMutationLock validates the expected serial, device profile state,
// cluster control, and effect staleness.
//
// On success, d.mu is acquired and remains held for the caller.
// On failure, d.mu is not held and an error is returned.
func (d *Device) acquireEffectMutationLock(expectedSerial, effect string) error {
	d.mu.Lock()
	if err := d.validateEffectTransitionLocked(expectedSerial); err != nil {
		d.mu.Unlock()
		return err
	}
	if d.DeviceProfile == nil || !d.DeviceProfile.Active {
		d.mu.Unlock()
		return fmt.Errorf("active OpenRGB device profile is not available")
	}
	if d.DeviceProfile.RGBCluster {
		d.mu.Unlock()
		return fmt.Errorf("device is controlled by RGB cluster")
	}
	if effect == "" || effect != d.effect {
		d.mu.Unlock()
		return fmt.Errorf("OpenRGB effect selection is stale")
	}
	if !slices.Contains(d.RGBModes, effect) {
		d.mu.Unlock()
		return fmt.Errorf("unsupported OpenRGB effect")
	}
	return nil
}

// SetEffectSpeed persists and reapplies the current imported-device software
// effect speed. The supplied speed is the renderer value, not the UI level.
func (d *Device) SetEffectSpeed(expectedSerial, effect string, speed float64) error {
	if d == nil {
		return fmt.Errorf("OpenRGB import is not available")
	}
	if math.IsNaN(speed) || math.IsInf(speed, 0) {
		return fmt.Errorf("OpenRGB effect speed is invalid")
	}

	d.effectTransitionMu.Lock()
	defer d.effectTransitionMu.Unlock()

	if err := d.acquireEffectMutationLock(expectedSerial, effect); err != nil {
		return err
	}
	descriptor, known := rgb.SoftwareEffectDescriptorByID(effect)
	if !known || !descriptor.Scope.Includes(rgb.EffectScopeDevice) || !descriptor.SupportsSpeed {
		d.mu.Unlock()
		return fmt.Errorf("OpenRGB effect does not support persistent speed")
	}
	minimum, maximum := rgb.ProfileSpeedRange(effect)
	if speed < minimum || speed > maximum {
		d.mu.Unlock()
		return fmt.Errorf("OpenRGB effect speed is outside the accepted range")
	}

	resolution, err := d.resolveLightingSettings(effect)
	if err != nil {
		d.mu.Unlock()
		return fmt.Errorf("resolve OpenRGB effect settings: %w", err)
	}
	settings := resolution.Settings.Clone()
	settings.Speed = &speed
	if err = lightingsettings.Validate(settings); err != nil {
		d.mu.Unlock()
		return fmt.Errorf("validate OpenRGB effect speed: %w", err)
	}
	if d.lightingEffects == nil {
		d.mu.Unlock()
		return fmt.Errorf("OpenRGB effect customization store is unavailable")
	}
	if err = d.lightingEffects.Set(d.Serial, effect, settings); err != nil {
		d.mu.Unlock()
		return fmt.Errorf("save OpenRGB effect speed: %w", err)
	}

	return d.applyPersistedEffectTransitionLocked(context.Background(), effect, true, expectedSerial, true)
}

// SetEffectColor persists and reapplies the current imported-device software
// effect's single color. The supplied color is the complete target state.
func (d *Device) SetEffectColor(expectedSerial, effect string, color lightingsettings.Color) error {
	if d == nil {
		return fmt.Errorf("OpenRGB import is not available")
	}

	d.effectTransitionMu.Lock()
	defer d.effectTransitionMu.Unlock()

	if err := d.acquireEffectMutationLock(expectedSerial, effect); err != nil {
		return err
	}
	descriptor, known := rgb.SoftwareEffectDescriptorByID(effect)
	if !known || !descriptor.Scope.Includes(rgb.EffectScopeDevice) || descriptor.PaletteKind != rgb.LightingPaletteStaticSingle {
		d.mu.Unlock()
		return fmt.Errorf("OpenRGB effect does not support single-color customization")
	}

	resolution, err := d.resolveLightingSettings(effect)
	if err != nil {
		d.mu.Unlock()
		return fmt.Errorf("resolve OpenRGB effect settings: %w", err)
	}
	settings := resolution.Settings.Clone()
	settings.SingleColor = &lightingsettings.SingleColorSettings{Color: color}
	if err = lightingsettings.Validate(settings); err != nil {
		d.mu.Unlock()
		return fmt.Errorf("validate OpenRGB effect color: %w", err)
	}
	if d.lightingEffects == nil {
		d.mu.Unlock()
		return fmt.Errorf("OpenRGB effect customization store is unavailable")
	}
	if err = d.lightingEffects.Set(d.Serial, effect, settings); err != nil {
		d.mu.Unlock()
		return fmt.Errorf("save OpenRGB effect color: %w", err)
	}

	return d.applyPersistedEffectTransitionLocked(context.Background(), effect, true, expectedSerial, true)
}

// ResetEffectCustomization removes the selected effect's local customization and,
// when one existed, restarts the renderer with the resolved hidden default.
func (d *Device) ResetEffectCustomization(expectedSerial, effect string) error {
	if d == nil {
		return fmt.Errorf("OpenRGB import is not available")
	}

	d.effectTransitionMu.Lock()
	defer d.effectTransitionMu.Unlock()

	if err := d.acquireEffectMutationLock(expectedSerial, effect); err != nil {
		return err
	}
	if d.lightingEffects == nil {
		d.mu.Unlock()
		return fmt.Errorf("OpenRGB effect customization store is unavailable")
	}
	deleted, err := d.lightingEffects.Delete(d.Serial, effect)
	if err != nil {
		d.mu.Unlock()
		return fmt.Errorf("reset OpenRGB effect customization: %w", err)
	}
	if !deleted {
		d.mu.Unlock()
		return nil
	}

	return d.applyPersistedEffectTransitionLocked(context.Background(), effect, true, expectedSerial, true)
}

func (d *Device) SetSpeed(speed string) error {
	value := 2.0
	switch speed {
	case "slow":
		value = 4.0
	case "fast":
		value = 0.8
	}
	d.mu.Lock()
	if d.lifecycleInactiveLocked() {
		err := d.lifecycleMutationErrorLocked()
		d.mu.Unlock()
		return err
	}
	serial := d.Serial
	effect := d.effect
	d.mu.Unlock()
	descriptor, supportsCanonicalSpeed := rgb.SoftwareEffectDescriptorByID(effect)
	if supportsCanonicalSpeed && descriptor.SupportsSpeed {
		if err := d.SetEffectSpeed(serial, effect, value); err != nil {
			return err
		}
	}
	d.mu.Lock()
	d.speed = value
	d.mu.Unlock()
	return nil
}

func (d *Device) SetEffect(effect string) error {
	return d.setEffectContext(context.Background(), effect, true, true, true)
}

func (d *Device) validateEffectTransitionLocked(expectedSerial string) error {
	if d.lifecycleInactiveLocked() {
		return d.lifecycleMutationErrorLocked()
	}
	if expectedSerial == "" || d.Serial != expectedSerial || !d.IsOpenRGB {
		return fmt.Errorf("OpenRGB import identity is not active")
	}

	d.resolveControllerId()
	if d.controllerId < 0 {
		return fmt.Errorf("controllerId not set")
	}
	if d.DeviceProfile != nil && d.DeviceProfile.RGBCluster {
		return fmt.Errorf("device is controlled by RGB cluster")
	}
	return nil
}

func (d *Device) setEffectContext(ctx context.Context, effect string, reportFailure, validateEffect, persist bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	d.effectTransitionMu.Lock()
	defer d.effectTransitionMu.Unlock()

	d.mu.Lock()
	expectedSerial := d.Serial
	if err := d.validateEffectTransitionLocked(expectedSerial); err != nil {
		d.mu.Unlock()
		return err
	}
	if validateEffect && (effect == "" || !slices.Contains(d.RGBModes, effect)) {
		d.mu.Unlock()
		return fmt.Errorf("unsupported OpenRGB effect")
	}

	if _, err := d.resolveLightingSettings(effect); err != nil {
		d.mu.Unlock()
		return fmt.Errorf("resolve effect: %w", err)
	}
	if persist {
		if d.lightingState == nil {
			d.mu.Unlock()
			return fmt.Errorf("OpenRGB device lighting state is unavailable")
		}
		if err := d.lightingState.Set(d.Serial, DeviceLightingState{
			SelectedEffect: effect,
			Brightness:     d.brightness,
		}); err != nil {
			d.mu.Unlock()
			return fmt.Errorf("save effect: %w", err)
		}
	}
	d.effect = effect
	if d.DeviceProfile != nil {
		d.DeviceProfile.RGBProfile = effect
	}

	return d.applyPersistedEffectTransitionLocked(ctx, effect, reportFailure, expectedSerial, false)
}

// dispatchEligibleSoftwareEffect keeps preserved effect IDs intact while
// limiting normal rendering to the device's exact advertised catalogue.
func dispatchEligibleSoftwareEffect(effect string, supportedEffects []string, runner *rgb.ActiveRGB, startTime *time.Time, profile *rgb.Profile) bool {
	if !slices.Contains(supportedEffects, effect) {
		runner.Static()
		return false
	}
	return dispatchSoftwareEffect(effect, runner, startTime, profile)
}

func waitForInitialEffectOutput(initialOutput <-chan error) error {
	waitCtx, cancel := initialEffectOutputContext()
	defer cancel()
	select {
	case err := <-initialOutput:
		return err
	case <-waitCtx.Done():
		return fmt.Errorf("wait for initial OpenRGB effect output: %w", waitCtx.Err())
	}
}

func dispatchSoftwareEffect(effect string, runner *rgb.ActiveRGB, startTime *time.Time, profile *rgb.Profile) bool {
	switch effect {
	case "off":
		startColor := runner.RGBStartColor
		runner.RGBStartColor = &rgb.Color{}
		runner.Static()
		runner.RGBStartColor = startColor
	case "static":
		runner.Static()
	case "arc":
		runner.Arc(*startTime)
	case "comet":
		runner.Comet(startTime)
	case "datastream":
		runner.DataStream(startTime)
	case "marquee":
		runner.Marquee(startTime)
	case "nebula":
		runner.Nebula(startTime)
	case "plasmacore":
		runner.PlasmaCore(startTime)
	case "rain":
		runner.Rain(*startTime)
	case "rotarystack":
		runner.RotaryStack(startTime)
	case "sequential":
		runner.Sequential(startTime)
	case "stardust":
		runner.Stardust(startTime)
	case "tokyonight":
		runner.TokyoNight(startTime)
	case "visor":
		runner.Visor(startTime)
	case "rainbow":
		runner.Rainbow(*startTime)
	case "pastelrainbow":
		runner.PastelRainbow(*startTime)
	case "spiralrainbow":
		runner.SpiralRainbow(*startTime)
	case "pastelspiralrainbow":
		runner.PastelSpiralRainbow(*startTime)
	case "watercolor":
		runner.Watercolor(*startTime)
	case "gradient":
		var gradients map[int]rgb.Color
		var speed float64 = 2.0
		if profile != nil {
			gradients = profile.Gradients
			speed = profile.Speed
		}
		runner.ColorshiftGradient(*startTime, gradients, speed)
	case "cpu-temperature":
		runner.Temperature(float64(getLightingCPUTemperature()))
	case "gpu-temperature":
		gpuTemperature := getLightingNVIDIATemperature(config.GetConfig().DefaultNvidiaGPU)
		if gpuTemperature == 0 {
			gpuTemperature = getLightingAMDTemperature()
		}
		runner.Temperature(float64(gpuTemperature))
	case "colorpulse":
		runner.Colorpulse(startTime)
	case "rotator":
		runner.Rotator(startTime)
	case "wave":
		runner.Wave(startTime)
	case "storm":
		runner.Storm()
	case "flickering":
		runner.Flickering(startTime)
	case "flame":
		runner.Flame(startTime)
	case "aurora":
		runner.Aurora(startTime)
	case "cyberpunkglitch":
		runner.CyberpunkGlitch(startTime)
	case "colorshift":
		runner.Colorshift(startTime, runner)
	case "circleshift":
		runner.CircleShift(startTime)
	case "circle":
		runner.Circle(startTime)
	case "spinner":
		runner.Spinner(startTime)
	case "colorwarp":
		runner.Colorwarp(startTime, runner)
	default:
		runner.Static()
		return false
	}
	return true
}

// applyPersistedEffectTransitionLocked replaces the active output after its
// desired profile state has been persisted. The caller holds effectTransitionMu
// and d.mu; this method releases d.mu before returning.
func (d *Device) applyPersistedEffectTransitionLocked(ctx context.Context, effect string, reportFailure bool, expectedSerial string, awaitInitialOutput bool) error {
	// Persistence succeeded, so replacing the prior effect can now proceed.
	d.stopEffectLoopLocked()
	if err := d.validateEffectTransitionLocked(expectedSerial); err != nil {
		d.mu.Unlock()
		return err
	}

	// off just sets black and exits
	if effect == "off" {
		if d.openrgbConn != nil {
			d.openrgbConn.Close()
			d.openrgbConn = nil
		}

		controllerID := d.controllerId
		colorCount := d.colorCount

		// Wait for hardware buffer to drain, matching the static color sequence
		if err := waitForContext(ctx, hardwareBufferDrainDelay); err != nil {
			d.mu.Unlock()
			return err
		}
		err := sendLightingColor(ctx, uint32(controllerID), colorCount, []byte{0, 0, 0})
		if err != nil && reportFailure && ctx.Err() == nil {
			d.recordOutputFailureLocked(err)
		}
		d.mu.Unlock()
		return err
	}

	// Static just reapplies current color once
	if effect == "static" {
		if d.openrgbConn != nil {
			d.openrgbConn.Close()
			d.openrgbConn = nil
		}

		frame, err := d.buildStaticFrame(d.brightness)
		if err != nil {
			d.mu.Unlock()
			return fmt.Errorf("resolve Static output: %w", err)
		}
		if err = waitForContext(ctx, hardwareBufferDrainDelay); err != nil {
			d.mu.Unlock()
			return err
		}
		controllerID := d.controllerId
		err = sendLightingFrame(ctx, uint32(controllerID), frame)
		if err != nil && reportFailure && ctx.Err() == nil {
			d.recordOutputFailureLocked(err)
		}
		d.mu.Unlock()
		return err
	}
	if err := ctx.Err(); err != nil {
		d.mu.Unlock()
		return err
	}

	profile := d.GetRgbProfile(effect)
	if profile == nil {
		d.mu.Unlock()
		return fmt.Errorf("resolved OpenRGB effect settings are unavailable")
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	d.stopChan = stop
	d.doneChan = done
	d.running = true
	runner := rgb.New(
		d.colorCount,
		profile.Speed,
		&profile.StartColor,
		&profile.EndColor,
		rgb.GetBrightnessValueFloat(d.brightness),
		0,
		0,
		true,
	)
	d.rgbRunner = runner

	controllerId := d.controllerId
	d.mu.Unlock()

	var initialOutput chan error
	if awaitInitialOutput {
		initialOutput = make(chan error, 1)
	}
	go func() {
		defer close(done)
		initialOutputPending := initialOutput != nil
		reportInitialOutput := func(err error) {
			if initialOutputPending {
				initialOutput <- err
				initialOutputPending = false
			}
		}

		startTime := time.Now()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				reportInitialOutput(fmt.Errorf("OpenRGB effect transition stopped before initial output"))
				return
			case <-ticker.C:
				d.mu.Lock()

				// Check if effect changed or stopped
				if d.lifecycleInactiveLocked() || !d.running || d.effect == "static" || d.effect == "off" {
					reportInitialOutput(fmt.Errorf("OpenRGB effect transition stopped before initial output"))
					d.mu.Unlock()
					return
				}

				// Refresh from the canonical resolver. Resolution returns a defensive
				// complete record and never materializes a customization.
				pf := d.GetRgbProfile(d.effect)
				if pf == nil {
					d.running = false
					d.stopChan = nil
					d.doneChan = nil
					err := fmt.Errorf("resolved OpenRGB effect settings are unavailable")
					d.recordOutputFailureLocked(err)
					d.mu.Unlock()
					reportInitialOutput(err)
					return
				}
				runner.RgbModeSpeed = pf.Speed
				runner.RGBBrightness = rgb.GetBrightnessValueFloat(d.brightness)
				runner.RGBStartColor = &pf.StartColor
				runner.RGBMiddleColor = &pf.MiddleColor
				runner.RGBEndColor = &pf.EndColor
				runner.MinTemp = pf.MinTemp
				runner.MaxTemp = pf.MaxTemp

				if runner.RGBMiddleColor == nil {
					runner.RGBMiddleColor = &rgb.Color{}
				}

				dispatchEligibleSoftwareEffect(d.effect, d.RGBModes, runner, &startTime, pf)

				frame := make([]byte, len(runner.Output))
				copy(frame, runner.Output)

				conn, err := sendLightingPersistentFrame(d.openrgbConn, uint32(controllerId), frame)
				if err != nil {
					d.running = false
					d.stopChan = nil
					d.doneChan = nil
					d.recordOutputFailureLocked(err)
					d.mu.Unlock()
					reportInitialOutput(err)
					return
				} else {
					d.openrgbConn = conn
				}
				d.mu.Unlock()
				reportInitialOutput(nil)
			}
		}
	}()

	if initialOutput != nil {
		return waitForInitialEffectOutput(initialOutput)
	}
	return nil
}

func waitForContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (d *Device) GetEffect() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.effect == "" {
		return "static"
	}
	return d.effect
}

func (d *Device) GetSpeed() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	resolvedSpeed := d.speed
	if descriptor, ok := rgb.SoftwareEffectDescriptorByID(d.effect); ok && descriptor.SupportsSpeed {
		if profile := d.GetRgbProfile(d.effect); profile != nil {
			resolvedSpeed = profile.Speed
		}
	}
	switch resolvedSpeed {
	case 4.0:
		return "slow"
	case 0.8:
		return "fast"
	default:
		return "normal"
	}
}

func (d *Device) GetBrightness() uint8 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.brightness
}

func (d *Device) GetRGBCluster() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.DeviceProfile == nil {
		return false
	}
	return d.DeviceProfile.RGBCluster
}

func (d *Device) saveDeviceProfileChecked() error {
	if d.DeviceProfile == nil {
		return nil
	}

	profileDir := deviceProfileDir()
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return fmt.Errorf("prepare profile directory: %w", err)
	}

	profilePath := d.DeviceProfile.Path
	if len(profilePath) == 0 {
		profilePath = filepath.Join(profileDir, d.Serial+".json")
		d.DeviceProfile.Path = profilePath
	}
	d.DeviceProfile.Serial = d.Serial
	d.DeviceProfile.Product = d.Product

	data, err := json.MarshalIndent(d.DeviceProfile, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profile: %w", err)
	}

	if err = os.WriteFile(profilePath, data, 0o644); err != nil {
		return fmt.Errorf("write profile: %w", err)
	}
	d.loadDeviceProfiles()
	return nil
}

func (d *Device) saveDeviceProfile() {
	if d.DeviceProfile == nil {
		return
	}

	profileDir := deviceProfileDir()
	_ = os.MkdirAll(profileDir, 0o755)

	profilePath := d.DeviceProfile.Path
	if len(profilePath) == 0 {
		profilePath = filepath.Join(profileDir, d.Serial+".json")
		d.DeviceProfile.Path = profilePath
	}
	d.DeviceProfile.Serial = d.Serial
	d.DeviceProfile.Product = d.Product

	data, err := json.MarshalIndent(d.DeviceProfile, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(profilePath, data, 0o644)
	d.loadDeviceProfiles()
}

func (d *Device) loadDeviceProfiles() {
	profileList := make(map[string]*DeviceProfile)
	profileDir := deviceProfileDir()

	files, err := os.ReadDir(profileDir)
	if err != nil {
		if os.IsNotExist(err) {
			d.UserProfiles = profileList
			return
		}
		logger.Log(logger.Fields{"error": err, "location": profileDir, "serial": d.Serial}).Warn("Unable to read profiles directory")
		return
	}

	for _, fi := range files {
		if fi.IsDir() {
			continue
		}

		profileLocation := filepath.Join(profileDir, fi.Name())

		if !common.IsValidExtension(profileLocation, ".json") {
			continue
		}

		fileName := strings.Split(fi.Name(), ".")[0]
		if m, _ := regexp.MatchString("^[a-zA-Z0-9-]+$", fileName); !m {
			continue
		}

		var profileName string
		if fileName == d.Serial {
			profileName = "default"
		} else if strings.HasPrefix(fileName, d.Serial+"-") {
			profileName = strings.TrimPrefix(fileName, d.Serial+"-")
		} else {
			continue
		}

		file, err := os.Open(profileLocation)
		if err != nil {
			logger.Log(logger.Fields{"error": err, "serial": d.Serial, "location": profileLocation}).Warn("Unable to load profile")
			continue
		}

		pf := &DeviceProfile{}
		if d.Config != nil {
			pf.ZoneColors = buildZoneColorsFromConfig(d.Config, d.lastColor)
		}
		if err = json.NewDecoder(file).Decode(pf); err != nil {
			logger.Log(logger.Fields{"error": err, "serial": d.Serial, "location": profileLocation}).Warn("Unable to decode profile")
			file.Close()
			continue
		}
		file.Close()

		pf.Path = profileLocation
		pf.Serial = d.Serial
		pf.Product = d.Product

		profileList[profileName] = pf
		logger.Log(logger.Fields{"location": profileLocation, "serial": d.Serial}).Info("Loaded custom user profile")
	}

	d.UserProfiles = profileList
	d.getDeviceProfile()
}

func (d *Device) getDeviceProfile() {
	if len(d.UserProfiles) == 0 {
		logger.Log(logger.Fields{"serial": d.Serial}).Warn("No profile found for device. Probably initial start")
	} else {
		foundActive := false
		for _, pf := range d.UserProfiles {
			if pf.Active {
				d.DeviceProfile = pf
				foundActive = true
				break
			}
		}
		if !foundActive {
			if pf, ok := d.UserProfiles["default"]; ok {
				pf.Active = true
				d.DeviceProfile = pf
			}
		}
	}
}

// SaveUserProfile will generate a new user profile configuration and save it to a file
func (d *Device) SaveUserProfile(profileName string) uint8 {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lifecycleInactiveLocked() {
		return 0
	}

	if d.DeviceProfile != nil {
		profileDir := filepath.Join(config.GetPaths().MutableDataRoot, "database", "profiles")
		profilePath := filepath.Join(profileDir, d.Serial+"-"+profileName+".json")

		// Deep copy ZoneColors map
		copiedZoneColors := make(map[int]ZoneColors)
		for k, v := range d.DeviceProfile.ZoneColors {
			var copiedColor *rgb.Color
			if v.Color != nil {
				copiedColor = &rgb.Color{
					Red:        v.Color.Red,
					Green:      v.Color.Green,
					Blue:       v.Color.Blue,
					Brightness: v.Color.Brightness,
					Hex:        v.Color.Hex,
				}
			}

			var copiedColorIndex []int
			if v.ColorIndex != nil {
				copiedColorIndex = make([]int, len(v.ColorIndex))
				copy(copiedColorIndex, v.ColorIndex)
			}

			copiedZoneColors[k] = ZoneColors{
				Color:      copiedColor,
				ColorIndex: copiedColorIndex,
				Name:       v.Name,
			}
		}

		// Deep copy BrightnessSlider pointer
		var copiedBrightness *uint8
		if d.DeviceProfile.BrightnessSlider != nil {
			val := *d.DeviceProfile.BrightnessSlider
			copiedBrightness = &val
		}

		var copiedRGBOverride *RGBOverride
		if d.DeviceProfile.RGBOverride != nil {
			override := *d.DeviceProfile.RGBOverride
			copiedRGBOverride = &override
		}

		newProfile := &DeviceProfile{
			Active:           false,
			Path:             profilePath,
			Product:          d.Product,
			Serial:           d.Serial,
			RGBProfile:       d.DeviceProfile.RGBProfile,
			BrightnessSlider: copiedBrightness,
			ZoneColors:       copiedZoneColors,
			RGBCluster:       d.DeviceProfile.RGBCluster,
			RGBOverride:      copiedRGBOverride,
		}

		buffer, err := json.MarshalIndent(newProfile, "", "  ")
		if err != nil {
			logger.Log(logger.Fields{"error": err}).Error("Unable to convert to json format")
			return 0
		}

		// Create profile filename
		file, err := os.Create(profilePath)
		if err != nil {
			logger.Log(logger.Fields{"error": err, "location": profilePath}).Error("Unable to create new device profile")
			return 0
		}
		defer file.Close()

		_, err = file.Write(buffer)
		if err != nil {
			logger.Log(logger.Fields{"error": err, "location": profilePath}).Error("Unable to write data")
			return 0
		}

		d.loadDeviceProfiles()
		return 1
	}
	return 0
}

// SaveDeviceProfile will save the current active device profile
func (d *Device) SaveDeviceProfile(_ string, _ bool) uint8 {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lifecycleInactiveLocked() {
		return 0
	}
	d.saveDeviceProfile()
	return 1
}

// ChangeDeviceProfile will change the active device profile
func (d *Device) ChangeDeviceProfile(profileName string) uint8 {
	d.mu.Lock()
	if d.lifecycleInactiveLocked() {
		d.mu.Unlock()
		return 0
	}
	profile, ok := d.UserProfiles[profileName]
	if !ok {
		d.mu.Unlock()
		return 0
	}

	currentProfile := d.DeviceProfile
	if currentProfile != nil {
		currentProfile.Active = false
		d.saveDeviceProfile()
	}

	// Stop any running effect loop
	d.stopEffectLoopLocked()

	newProfile := profile
	newProfile.Active = true
	d.DeviceProfile = newProfile
	d.saveDeviceProfile()

	clusterController := d.clusterControllerLocked()
	d.mu.Unlock()

	d.replaceClusterController(clusterController)
	if clusterController == nil {
		_ = d.resumeDesiredState(context.Background())
	}

	return 1
}

// DeleteDeviceProfile deletes a device profile and its JSON file
func (d *Device) DeleteDeviceProfile(profileName string) uint8 {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.lifecycleInactiveLocked() {
		return 0
	}

	profile, ok := d.UserProfiles[profileName]
	if !ok {
		return 0
	}

	if !common.IsValidExtension(profile.Path, ".json") {
		return 0
	}

	if profile.Active {
		return 2
	}

	if err := os.Remove(profile.Path); err != nil {
		return 3
	}

	delete(d.UserProfiles, profileName)
	return 1
}

func (d *Device) setupClusterController() {
	d.mu.Lock()
	clusterController := d.clusterControllerLocked()
	d.mu.Unlock()
	d.registerClusterController(clusterController)
}

func (d *Device) clusterControllerLocked() *common.ClusterController {
	if d.DeviceProfile == nil || !d.DeviceProfile.RGBCluster {
		return nil
	}
	return &common.ClusterController{
		Product:      d.Product,
		Serial:       d.Serial,
		LedChannels:  uint32(d.colorCount),
		WriteColorEx: d.writeColorCluster,
	}
}

func addClusterController(controller *common.ClusterController) error {
	if controller == nil {
		return nil
	}
	if clusterDevice := getConfigCluster(); clusterDevice != nil {
		clusterDevice.AddDeviceController(controller)
	}
	return nil
}

func removeClusterController(serial string) error {
	if clusterDevice := getConfigCluster(); clusterDevice != nil {
		clusterDevice.RemoveDeviceControllerBySerial(serial)
	}
	return nil
}

func (d *Device) registerClusterController(controller *common.ClusterController) {
	_, _ = d.registerClusterControllerWith(controller, addClusterController)
}

func (d *Device) registerClusterControllerWith(
	controller *common.ClusterController,
	add func(*common.ClusterController) error,
) (bool, error) {
	return d.registerClusterControllerWithPolicy(controller, add, false)
}

func (d *Device) registerLifecycleClusterControllerWith(
	controller *common.ClusterController,
	add func(*common.ClusterController) error,
) (bool, error) {
	return d.registerClusterControllerWithPolicy(controller, add, true)
}

func (d *Device) registerClusterControllerWithPolicy(
	controller *common.ClusterController,
	add func(*common.ClusterController) error,
	allowActivating bool,
) (bool, error) {
	if controller == nil {
		return false, nil
	}

	d.clusterMutationMu.Lock()
	defer d.clusterMutationMu.Unlock()

	d.mu.Lock()
	detached := d.lifecycleDetached
	activating := d.lifecycleActivating
	d.mu.Unlock()
	if detached || (activating && !allowActivating) {
		return false, nil
	}
	if err := add(controller); err != nil {
		return false, err
	}
	return true, nil
}

func (d *Device) replaceClusterController(controller *common.ClusterController) {
	_ = d.replaceClusterControllerWith(controller, addClusterController, removeClusterController)
}

func (d *Device) replaceClusterControllerWith(
	controller *common.ClusterController,
	add func(*common.ClusterController) error,
	remove func(string) error,
) error {
	d.clusterMutationMu.Lock()
	defer d.clusterMutationMu.Unlock()

	d.mu.Lock()
	inactive := d.lifecycleInactiveLocked()
	serial := d.Serial
	d.mu.Unlock()
	if inactive {
		return nil
	}
	if err := remove(serial); err != nil {
		return err
	}
	if controller == nil {
		return nil
	}
	return add(controller)
}

func (d *Device) removeClusterControllerWith(remove func(string) error) error {
	d.clusterMutationMu.Lock()
	defer d.clusterMutationMu.Unlock()

	d.mu.Lock()
	serial := d.Serial
	d.mu.Unlock()
	return remove(serial)
}

// writeColorCluster will write data to the device from cluster client
func (d *Device) writeColorCluster(data []byte, _ int) {
	d.mu.Lock()

	if d.lifecycleInactiveLocked() || d.controllerId < 0 || d.DeviceProfile == nil || !d.DeviceProfile.RGBCluster {
		d.mu.Unlock()
		return
	}

	// Clamp data to our LED count in case the cluster sends more bytes than we own
	expected := d.colorCount * 3
	frame := make([]byte, expected)
	if len(data) >= expected {
		copy(frame, data[:expected])
	} else {
		copy(frame, data)
	}

	// The cluster renderer already applied cluster-owned brightness. Local
	// device brightness remains stored but is inactive while clustered.
	conn, err := sendClusterFrame(d.openrgbConn, uint32(d.controllerId), frame)
	if err != nil {
		d.recordOutputFailureLocked(err)
	} else {
		d.openrgbConn = conn
	}
	d.mu.Unlock()
}

// ProcessSetRgbCluster will update OpenRGB integration status for cluster
func (d *Device) ProcessSetRgbCluster(enabled bool) uint8 {
	d.mu.Lock()
	if d.lifecycleInactiveLocked() {
		d.mu.Unlock()
		return 0
	}
	if d.DeviceProfile == nil {
		d.mu.Unlock()
		return 0
	}

	d.DeviceProfile.RGBCluster = enabled
	d.saveDeviceProfile()

	if enabled {
		d.stopEffectLoopLocked()
		clusterController := d.clusterControllerLocked()
		d.mu.Unlock()
		d.replaceClusterController(clusterController)
	} else {
		if d.openrgbConn != nil {
			d.openrgbConn.Close()
			d.openrgbConn = nil
		}
		effect := d.effect
		d.mu.Unlock()
		d.replaceClusterController(nil)
		if effect != "" {
			go func() {
				_ = d.resumeDesiredState(context.Background())
			}()
		}
	}

	return 1
}

func (d *Device) Stop() {
	d.mu.Lock()
	d.stopEffectLoopLocked()
	if d.openrgbConn != nil {
		d.openrgbConn.Close()
		d.openrgbConn = nil
	}
	d.mu.Unlock()
}

func (d *Device) detachForRemoval(remove func(string) error) (*common.ClusterController, error) {
	d.clusterMutationMu.Lock()
	defer d.clusterMutationMu.Unlock()

	d.mu.Lock()
	clusterController := d.clusterControllerLocked()
	d.lifecycleDetached = true
	d.controllerId = -1
	if d.openrgbConn != nil {
		_ = d.openrgbConn.Close()
		d.openrgbConn = nil
	}
	d.stopEffectLoopLocked()
	serial := d.Serial
	d.mu.Unlock()

	return clusterController, remove(serial)
}

func (d *Device) restoreAfterRemoval(
	controller *common.ClusterController,
	add func(*common.ClusterController) error,
) error {
	d.clusterMutationMu.Lock()
	defer d.clusterMutationMu.Unlock()

	d.mu.Lock()
	d.lifecycleDetached = false
	d.mu.Unlock()
	if controller == nil {
		return nil
	}
	return add(controller)
}

func (d *Device) StopDirty() uint8 {
	d.mu.Lock()
	d.stopEffectLoopLocked()
	if d.openrgbConn != nil {
		d.openrgbConn.Close()
		d.openrgbConn = nil
	}
	d.mu.Unlock()
	return 2
}

func (d *Device) GetRgbProfiles() interface{} {
	if d == nil || d.lightingResolver == nil {
		return nil
	}
	profiles := make(map[string]rgb.Profile, len(d.RGBModes))
	for _, effect := range d.RGBModes {
		resolution, err := d.resolveLightingSettings(effect)
		if err != nil {
			continue
		}
		profiles[effect] = rgbProfileFromLightingSettings(resolution.Settings)
	}
	return rgb.RGB{Device: d.Product, Profiles: profiles}
}

func (d *Device) GetRgbProfile(profile string) *rgb.Profile {
	if d == nil || d.lightingResolver == nil {
		return nil
	}
	resolution, err := d.resolveLightingSettings(profile)
	if err != nil {
		return nil
	}
	resolved := rgbProfileFromLightingSettings(resolution.Settings)
	return &resolved
}

func (d *Device) UpdateRgbProfileData(profileName string, profile rgb.Profile) uint8 {
	d.mu.Lock()
	if d.lifecycleInactiveLocked() {
		d.mu.Unlock()
		return 0
	}

	if !slices.Contains(d.RGBModes, profileName) || d.lightingEffects == nil {
		d.mu.Unlock()
		return 0
	}
	settings, err := lightingSettingsFromRGBProfile(profileName, cloneRGBProfile(profile))
	if err != nil || d.lightingEffects.Set(d.Serial, profileName, settings) != nil {
		d.mu.Unlock()
		return 0
	}
	reapply := d.effect == profileName
	d.mu.Unlock()

	// If we are currently running this effect, we want to restart/reapply it to pick up changes!
	if reapply {
		_ = d.setEffectContext(context.Background(), profileName, true, true, false)
	}

	return 1
}

func (d *Device) UpdateRgbProfile(_ int, profile string) uint8 {
	d.mu.Lock()
	if d.lifecycleInactiveLocked() {
		d.mu.Unlock()
		return 0
	}
	if d.DeviceProfile == nil {
		d.mu.Unlock()
		return 0
	}

	if d.GetRgbProfile(profile) == nil {
		d.mu.Unlock()
		return 0
	}

	if d.DeviceProfile.RGBCluster {
		d.mu.Unlock()
		return 5
	}
	d.mu.Unlock()

	err := d.SetEffect(profile)
	if err != nil {
		return 0
	}

	return 1
}

func (d *Device) ProcessGetRgbOverride(channelId, subDeviceId int) interface{} {
	d.mu.Lock()
	defer d.mu.Unlock()

	defaultOverride := &RGBOverride{
		Enabled:        false,
		RGBStartColor:  rgb.Color{Red: 255, Green: 255, Blue: 255},
		RGBMiddleColor: rgb.Color{Red: 255, Green: 255, Blue: 255},
		RGBEndColor:    rgb.Color{Red: 255, Green: 255, Blue: 255},
		RgbModeSpeed:   5.0,
	}

	if d.DeviceProfile == nil {
		return defaultOverride
	}

	if d.lifecycleInactiveLocked() {
		if d.DeviceProfile.RGBOverride != nil {
			override := *d.DeviceProfile.RGBOverride
			return &override
		}
		return defaultOverride
	}

	if d.DeviceProfile.RGBOverride == nil {
		d.DeviceProfile.RGBOverride = defaultOverride
	}

	return d.DeviceProfile.RGBOverride
}

func validRGBOverrideNumber(value, minimum, maximum float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minimum && value <= maximum
}

func validRGBOverrideColor(color rgb.Color) bool {
	return validRGBOverrideNumber(color.Red, 0, 255) &&
		validRGBOverrideNumber(color.Green, 0, 255) &&
		validRGBOverrideNumber(color.Blue, 0, 255) &&
		validRGBOverrideNumber(color.Temperature, 0, 105)
}

// SetRGBOverride applies one complete controller-wide OpenRGB override. A nil
// speed preserves the current override speed or uses the established default.
func (d *Device) SetRGBOverride(expectedSerial string, channelId, subDeviceId int, enabled bool, startColor, endColor, middleColor rgb.Color, speed *float64) error {
	if d == nil {
		return fmt.Errorf("OpenRGB import is not available")
	}
	if channelId != 0 || subDeviceId != 0 {
		return fmt.Errorf("OpenRGB RGB override selectors must be zero")
	}
	if !validRGBOverrideColor(startColor) || !validRGBOverrideColor(middleColor) || !validRGBOverrideColor(endColor) {
		return fmt.Errorf("OpenRGB RGB override color is invalid")
	}
	if speed != nil && !validRGBOverrideNumber(*speed, 0, 10) {
		return fmt.Errorf("OpenRGB RGB override speed is invalid")
	}

	d.effectTransitionMu.Lock()
	defer d.effectTransitionMu.Unlock()

	d.mu.Lock()
	if d.lifecycleInactiveLocked() {
		err := d.lifecycleMutationErrorLocked()
		d.mu.Unlock()
		return err
	}
	if expectedSerial == "" || d.Serial != expectedSerial || !d.IsOpenRGB {
		d.mu.Unlock()
		return fmt.Errorf("OpenRGB import identity is not active")
	}
	d.resolveControllerId()
	if d.controllerId < 0 {
		d.mu.Unlock()
		return fmt.Errorf("controllerId not set")
	}
	if d.DeviceProfile == nil || !d.DeviceProfile.Active {
		d.mu.Unlock()
		return fmt.Errorf("active OpenRGB device profile is not available")
	}

	resolvedSpeed := 5.0
	if d.DeviceProfile.RGBOverride != nil {
		resolvedSpeed = d.DeviceProfile.RGBOverride.RgbModeSpeed
	}
	if speed != nil {
		resolvedSpeed = *speed
	}
	if !validRGBOverrideNumber(resolvedSpeed, 0, 10) {
		d.mu.Unlock()
		return fmt.Errorf("OpenRGB RGB override speed is invalid")
	}

	effect := d.effect
	clustered := d.DeviceProfile.RGBCluster
	if !clustered && (effect == "" || !slices.Contains(d.RGBModes, effect)) {
		d.mu.Unlock()
		return fmt.Errorf("unsupported OpenRGB effect")
	}

	startColor.Brightness = 1
	middleColor.Brightness = 1
	endColor.Brightness = 1
	previousProfile := cloneDeviceProfile(d.DeviceProfile)
	d.DeviceProfile.RGBOverride = &RGBOverride{
		Enabled:        enabled,
		RGBStartColor:  startColor,
		RGBMiddleColor: middleColor,
		RGBEndColor:    endColor,
		RgbModeSpeed:   resolvedSpeed,
	}
	if err := d.saveDeviceProfileChecked(); err != nil {
		*d.DeviceProfile = *previousProfile
		d.mu.Unlock()
		return fmt.Errorf("save OpenRGB RGB override: %w", err)
	}

	if d.DeviceProfile != nil && d.DeviceProfile.RGBCluster {
		d.mu.Unlock()
		return nil
	}
	if d.DeviceProfile == nil {
		d.mu.Unlock()
		return fmt.Errorf("active OpenRGB device profile is not available after save")
	}

	effect = d.effect
	return d.applyPersistedEffectTransitionLocked(context.Background(), effect, true, expectedSerial, false)
}

func (d *Device) ProcessSetRgbOverride(channelId, subDeviceId int, enabled bool, startColor, endColor, middleColor rgb.Color, speed float64) uint8 {
	if d == nil {
		return 0
	}
	d.mu.Lock()
	expectedSerial := d.Serial
	d.mu.Unlock()
	if err := d.SetRGBOverride(expectedSerial, channelId, subDeviceId, enabled, startColor, endColor, middleColor, &speed); err != nil {
		return 0
	}
	return 1
}
