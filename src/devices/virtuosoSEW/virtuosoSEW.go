package virtuosoSEW

// Package: CORSAIR VIRTUOSO SE WIRELESS
// Author: Nikola Jurkovic
// License: GPL-3.0 or later

import (
	"LumenForge/src/audio"
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"LumenForge/src/stats"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sstallion/go-hid"
)

type ZoneColors struct {
	Color      *rgb.Color
	ColorIndex []int
	Name       string
}

type Equalizer struct {
	Name  string
	Value float64
}

// DeviceProfile struct contains all device profile
type DeviceProfile struct {
	Active              bool
	Path                string
	Product             string
	Serial              string
	Brightness          uint8
	RGBProfile          string
	BrightnessSlider    *uint8
	OriginalBrightness  uint8
	Label               string
	Profile             int
	ZoneColors          map[int]ZoneColors
	Equalizers          map[int]Equalizer
	SleepMode           int
	MuteIndicator       int
	DisableMicIndicator int
	RgbOff              bool
}

type DPIProfile struct {
	Name        string `json:"name"`
	Value       uint16
	PackerIndex int
	ColorIndex  map[int][]int
	Color       *rgb.Color
}

type Device struct {
	Debug                 bool
	dev                   *hid.Device
	listener              *hid.Device
	Manufacturer          string `json:"manufacturer"`
	Product               string `json:"product"`
	Serial                string `json:"serial"`
	Firmware              string `json:"firmware"`
	activeRgb             *rgb.ActiveRGB
	UserProfiles          map[string]*DeviceProfile `json:"userProfiles"`
	Devices               map[int]string            `json:"devices"`
	DeviceProfile         *DeviceProfile
	OriginalProfile       *DeviceProfile
	Template              string
	VendorId              uint16
	ProductId             uint16
	SlipstreamId          uint16
	Brightness            map[int]string
	LEDChannels           int
	ChangeableLedChannels int
	CpuTemp               float32
	GpuTemp               float32
	Layouts               []string
	Rgb                   *rgb.RGB
	lightingMu            sync.Mutex
	lightingMutation      sync.Mutex
	lightingDefaults      *lightingsettings.DefaultRepository
	schedulerDark         bool
	userRGBOff            bool
	lightingRestart       func()
	lightingWrite         func([]byte)
	rgbMutex              sync.RWMutex
	Endpoint              byte
	SleepModes            map[int]string
	Connected             bool
	mutex                 sync.Mutex
	Exit                  bool
	MuteStatus            byte
	MuteIndicators        map[int]string
	BatteryLevel          uint16
	RGBModes              []string
	ZoneAmount            int
	Usb                   bool
}

var (
	pwd                       = ""
	cmdSoftwareMode           = []byte{0x01, 0x03, 0x00, 0x02}
	cmdHardwareMode           = []byte{0x01, 0x03, 0x00, 0x01}
	cmdSleepMode              = []byte{0x01, 0x03, 0x00, 0x04}
	cmdGetFirmware            = []byte{0x02, 0x13}
	cmdWriteColor             = []byte{0x06, 0x00}
	cmdOpenEndpoint           = []byte{0x0d, 0x00, 0x01}
	cmdOpenSleepWriteEndpoint = []byte{0x01, 0x0d, 0x00}
	cmdSleep                  = []byte{0x01, 0x0e, 0x00}
	cmdBatteryLevel           = []byte{0x02, 0x0f}
	cmdEnable                 = []byte{0x01}
	cmdDisable                = []byte{0x00}
	bufferSize                = 64
	bufferSizeWrite           = bufferSize + 1
	headerSize                = 3
	headerWriteSize           = 4
	rgbProfileUpgrade         = []string{"gradient", "pastelrainbow", "pastelspiralrainbow", "flame", "aurora", "cyberpunkglitch", "tokyonight"}
	rgbModes                  = []string{
		"colorpulse",
		"colorshift",
		"colorwarp",
		"cpu-temperature",
		"flickering",
		"flame",
		"aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature",
		"gradient",
		"headset",
		"off",
		"rainbow",
		"pastelrainbow",
		"rotator",
		"static",
		"storm",
		"watercolor",
		"wave",
	}
)

func Init(vendorId, slipstreamId, productId uint16, dev *hid.Device, endpoint byte, serial string) *Device {
	// Set global working directory
	pwd = config.GetPaths().MutableDataRoot

	// Init new struct with HID device
	d := &Device{
		dev:          dev,
		Template:     "virtuosoSEW.html",
		VendorId:     vendorId,
		ProductId:    productId,
		SlipstreamId: slipstreamId,
		Serial:       strconv.Itoa(int(productId)),
		Endpoint:     endpoint,
		Firmware:     "n/a",
		Brightness: map[int]string{
			0: "RGB Profile",
			1: "33 %",
			2: "66 %",
			3: "100 %",
		},
		Product: "VIRTUOSO SE",
		SleepModes: map[int]string{
			0:  "Off",
			1:  "1 minute",
			5:  "5 minutes",
			10: "10 minutes",
			15: "15 minutes",
			30: "30 minutes",
			60: "1 hour",
		},
		RGBModes:              rgbModes,
		LEDChannels:           3,
		ChangeableLedChannels: 3,
		MuteIndicators: map[int]string{
			0: "Disabled",
			1: "Enabled",
		},
		ZoneAmount: 3,
	}

	d.getDebugMode()       // Debug mode
	d.loadRgb()            // Load RGB
	d.loadDeviceProfiles() // Load all device profiles
	d.saveDeviceProfile()  // Save profile
	if err := d.attachLightingRuntime(config.GetPaths()); err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Warn("Canonical Lighting unavailable; retaining legacy Lighting")
	}
	d.getMuteStatus() // Mute status
	return d
}

// GetRgbProfiles will return RGB profiles for a target device
func (d *Device) GetRgbProfiles() interface{} {
	if d.lightingDefaults != nil {
		d.rgbMutex.RLock()
		defer d.rgbMutex.RUnlock()
	}
	if d.Rgb == nil {
		return nil
	}
	tmp := *d.Rgb

	// Filter unsupported modes out
	profiles := make(map[string]rgb.Profile, len(tmp.Profiles))
	for key, value := range tmp.Profiles {
		if slices.Contains(rgbModes, key) {
			profiles[key] = copyLightingRGBProfile(value)
		}
	}
	tmp.Profiles = profiles
	return tmp
}

// GetZoneColors will return current device zone colors
func (d *Device) GetZoneColors() interface{} {
	if d.lightingDefaults != nil {
		d.lightingMu.Lock()
		defer d.lightingMu.Unlock()
		return d.copyLightingZones()
	}
	if d.DeviceProfile == nil {
		return nil
	}
	return d.DeviceProfile.ZoneColors
}

// Stop will stop all device operations and switch a device back to hardware mode
func (d *Device) Stop() {
	// Placeholder
}

// StopInternal will stop all device operations and switch a device back to hardware mode
func (d *Device) StopInternal() {
	d.Exit = true
	logger.Log(logger.Fields{"serial": d.Serial, "product": d.Product}).Info("Stopping device...")
	if d.activeRgb != nil {
		d.activeRgb.Stop()
	}

	if d.Connected {
		d.setHardwareMode()
	}
	logger.Log(logger.Fields{"serial": d.Serial, "product": d.Product}).Info("Device stopped")
}

// StopDirty will stop devices in a dirty way
func (d *Device) StopDirty() uint8 {
	d.Exit = true
	logger.Log(logger.Fields{"serial": d.Serial, "product": d.Product}).Info("Stopping device (dirty)...")

	if d.activeRgb != nil {
		d.activeRgb.Stop()
	}

	logger.Log(logger.Fields{"serial": d.Serial, "product": d.Product}).Info("Device stopped")
	d.Connected = false
	return 1
}

// SetConnected will change connected status
func (d *Device) SetConnected(value bool) {
	if d.Connected {
		if d.activeRgb != nil {
			d.activeRgb.Exit <- true
			d.activeRgb = nil
		}
		d.Connected = value
		time.Sleep(1000 * time.Millisecond)
	}
}

// Connect will connect to a device
func (d *Device) Connect() {
	if !d.Connected {
		d.Connected = true
		d.getDeviceFirmware() // Firmware
		d.setSoftwareMode()   // Activate software mode
		d.getBatterLevel()    // Battery level
		d.initLeds()          // Init LED ports
		d.setDeviceColor()    // Device color
		d.setEqualizer()      // Equalizer
	}
}

// loadRgb will load RGB file if found, or create the default.
func (d *Device) loadRgb() {
	rgbDirectory := pwd + "/database/rgb/"
	rgbFilename := rgbDirectory + d.Serial + ".json"

	// Check if filename has .json extension
	if !common.IsValidExtension(rgbFilename, ".json") {
		return
	}

	if !common.FileExists(rgbFilename) {
		profile := rgb.GetRGB()
		profile.Device = d.Product

		if err := common.SaveJsonData(rgbFilename, profile); err != nil {
			logger.Log(logger.Fields{"error": err, "location": rgbFilename}).Error("Unable to write rgb profile data")
			return
		}
	}

	file, err := os.Open(rgbFilename)
	if err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial, "location": rgbFilename}).Warn("Unable to load RGB")
		return
	}
	if err = json.NewDecoder(file).Decode(&d.Rgb); err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial, "location": rgbFilename}).Warn("Unable to decode profile")
		return
	}
	err = file.Close()
	if err != nil {
		logger.Log(logger.Fields{"location": rgbFilename, "serial": d.Serial}).Warn("Failed to close file handle")
	}

	d.upgradeRgbProfile(rgbFilename, rgbProfileUpgrade)
}

// upgradeRgbProfile will upgrade current rgb profile list
func (d *Device) upgradeRgbProfile(path string, profiles []string) {
	save := false
	for _, profile := range profiles {
		pf := d.GetRgbProfile(profile)
		if pf == nil {
			save = true
			logger.Log(logger.Fields{"profile": profile}).Info("Upgrading RGB profile")
			template := rgb.GetRgbProfile(profile)
			if template == nil {
				d.Rgb.Profiles[profile] = rgb.Profile{}
			} else {
				d.Rgb.Profiles[profile] = *template
			}
		}
	}
	for key, val := range d.Rgb.Profiles {
		template := rgb.GetRgbProfile(key)
		if template == nil {
			continue
		}

		if val.Version != template.Version {
			d.Rgb.Profiles[key] = *template
			save = true
		}
	}
	if save {
		if err := common.SaveJsonData(path, d.Rgb); err != nil {
			logger.Log(logger.Fields{"error": err, "location": path}).Error("Unable to upgrade rgb profile data")
			return
		}
	}
}

// GetRgbProfile will return rgb.Profile struct
func (d *Device) GetRgbProfile(profile string) *rgb.Profile {
	if d.lightingDefaults != nil {
		d.rgbMutex.RLock()
		defer d.rgbMutex.RUnlock()
	}
	if d.Rgb == nil {
		return nil
	}

	if val, ok := d.Rgb.Profiles[profile]; ok {
		val = copyLightingRGBProfile(val)
		return &val
	}
	return nil
}

// GetDeviceTemplate will return device template name
func (d *Device) GetDeviceTemplate() string {
	return d.Template
}

// ChangeDeviceProfile will change device profile
func (d *Device) ChangeDeviceProfile(profileName string) uint8 {
	if d.lightingDefaults != nil {
		if d.switchLightingProfile(profileName) != nil {
			return 0
		}
		return 1
	}

	if profile, ok := d.UserProfiles[profileName]; ok {
		if !d.Connected {
			return 0
		}
		currentProfile := d.DeviceProfile
		currentProfile.Active = false
		d.DeviceProfile = currentProfile
		d.saveDeviceProfile()

		// RGB reset
		if d.activeRgb != nil {
			d.activeRgb.Exit <- true
			d.activeRgb = nil
		}

		newProfile := profile
		newProfile.Active = true
		d.DeviceProfile = newProfile
		d.saveDeviceProfile()
		d.setDeviceColor()
		return 1
	}
	return 0
}

// DeleteDeviceProfile deletes a device profile and its JSON file
func (d *Device) DeleteDeviceProfile(profileName string) uint8 {
	if !d.Connected {
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

// saveRgbProfile will save rgb profile data
func (d *Device) saveRgbProfile() {
	rgbDirectory := pwd + "/database/rgb/"
	rgbFilename := rgbDirectory + d.Serial + ".json"
	if common.FileExists(rgbFilename) {
		if err := common.SaveJsonData(rgbFilename, d.Rgb); err != nil {
			logger.Log(logger.Fields{"error": err, "location": rgbFilename}).Error("Unable to write rgb profile data")
			return
		}
	}
}

// ProcessNewGradientColor will create new gradient color
func (d *Device) ProcessNewGradientColor(profileName string) (uint8, uint) {
	if d.lightingDefaults != nil {
		d.lightingMutation.Lock()
		defer d.lightingMutation.Unlock()
		d.lightingMu.Lock()
		if _, err := d.resolveLightingSettings(profileName); err != nil {
			d.lightingMu.Unlock()
			return 0, 0
		}
		pf := d.GetRgbProfile(profileName)
		if pf.Gradients == nil {
			d.lightingMu.Unlock()
			return 0, 0
		}

		nextID := 0
		for key := range pf.Gradients {
			if key >= nextID {
				nextID = key + 1
			}
		}
		pf.Gradients[nextID] = rgb.Color{Green: 255, Blue: 255}
		index := nextID

		if _, err := lightingsettings.EffectSettingsFromRGBProfile(profileName, *pf); err != nil {
			d.lightingMu.Unlock()
			return 0, 0
		}
		err := d.persistLightingRGBProfile(profileName, *pf)
		selected := d.DeviceProfile.RGBProfile == profileName
		d.lightingMu.Unlock()
		if err != nil {
			return 0, 0
		}
		if selected {
			d.restartLighting()
		}
		return 1, uint(index)
	}

	if d.GetRgbProfile(profileName) == nil {
		logger.Log(logger.Fields{"serial": d.Serial, "profile": profileName}).Warn("Non-existing RGB profile")
		return 0, 0
	}

	pf := d.GetRgbProfile(profileName)
	if pf == nil {
		return 0, 0
	}

	if pf.Gradients == nil {
		return 0, 0
	}

	// find next available key
	nextID := 0
	for k := range pf.Gradients {
		if k >= nextID {
			nextID = k + 1
		}
	}
	pf.Gradients[nextID] = rgb.Color{Red: 0, Green: 255, Blue: 255}

	d.Rgb.Profiles[profileName] = *pf
	d.saveRgbProfile()
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.setDeviceColor()
	return 1, uint(nextID)
}

// ProcessDeleteGradientColor will delete gradient color
func (d *Device) ProcessDeleteGradientColor(profileName string) (uint8, uint) {
	if d.lightingDefaults != nil {
		d.lightingMutation.Lock()
		defer d.lightingMutation.Unlock()
		d.lightingMu.Lock()
		if _, err := d.resolveLightingSettings(profileName); err != nil {
			d.lightingMu.Unlock()
			return 0, 0
		}
		pf := d.GetRgbProfile(profileName)
		if pf.Gradients == nil {
			d.lightingMu.Unlock()
			return 0, 0
		}

		if len(pf.Gradients) < 3 {
			d.lightingMu.Unlock()
			return 2, 0
		}
		index := -1
		for key := range pf.Gradients {
			if key > index {
				index = key
			}
		}
		delete(pf.Gradients, index)

		if _, err := lightingsettings.EffectSettingsFromRGBProfile(profileName, *pf); err != nil {
			d.lightingMu.Unlock()
			return 0, 0
		}
		err := d.persistLightingRGBProfile(profileName, *pf)
		selected := d.DeviceProfile.RGBProfile == profileName
		d.lightingMu.Unlock()
		if err != nil {
			return 0, 0
		}
		if selected {
			d.restartLighting()
		}
		return 1, uint(index)
	}

	if d.GetRgbProfile(profileName) == nil {
		logger.Log(logger.Fields{"serial": d.Serial, "profile": profileName}).Warn("Non-existing RGB profile")
		return 0, 0
	}

	pf := d.GetRgbProfile(profileName)
	if pf == nil {
		return 0, 0
	}

	if len(pf.Gradients) < 3 {
		return 2, 0
	}

	maxKey := -1
	for k := range pf.Gradients {
		if k > maxKey {
			maxKey = k
		}
	}
	delete(pf.Gradients, maxKey)

	d.Rgb.Profiles[profileName] = *pf
	d.saveRgbProfile()
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.setDeviceColor()
	return 1, uint(maxKey)
}

// UpdateRgbProfileData will update RGB profile data
func (d *Device) UpdateRgbProfileData(profileName string, profile rgb.Profile) uint8 {
	if d.lightingDefaults != nil {
		value, err := lightingsettings.EffectSettingsFromRGBProfile(profileName, profile)
		if err != nil || d.SetLightingEffectSettings(profileName, value) != nil {
			return 0
		}
		return 1
	}

	d.rgbMutex.Lock()
	defer d.rgbMutex.Unlock()

	if !d.Connected {
		return 0
	}

	if d.GetRgbProfile(profileName) == nil {
		logger.Log(logger.Fields{"serial": d.Serial, "profile": profile}).Warn("Non-existing RGB profile")
		return 0
	}

	pf := d.GetRgbProfile(profileName)
	if pf == nil {
		return 0
	}

	if profile.StartColor.Temperature < 0 || profile.StartColor.Temperature > 105 {
		return 0
	}

	if profile.MiddleColor.Temperature < 0 || profile.MiddleColor.Temperature > 105 {
		return 0
	}

	if profile.EndColor.Temperature < 0 || profile.EndColor.Temperature > 105 {
		return 0
	}

	profile.StartColor.Brightness = pf.StartColor.Brightness
	profile.EndColor.Brightness = pf.EndColor.Brightness
	profile.MiddleColor.Brightness = pf.MiddleColor.Brightness
	pf.StartColor = profile.StartColor
	pf.EndColor = profile.EndColor
	pf.MiddleColor = profile.MiddleColor
	pf.Speed = profile.Speed
	pf.Gradients = profile.Gradients

	d.Rgb.Profiles[profileName] = *pf
	d.saveRgbProfile()
	if d.Connected {
		if d.activeRgb != nil {
			d.activeRgb.Exit <- true
			d.activeRgb = nil
		}
		d.setDeviceColor()
	}
	return 1
}

// UpdateRgbProfile will update device RGB profile
func (d *Device) UpdateRgbProfile(_ int, profile string) uint8 {
	if d.lightingDefaults != nil {
		if d.SetLightingEffect(profile) != nil {
			return 0
		}
		return 1
	}

	if !d.Connected {
		return 0
	}

	if d.GetRgbProfile(profile) == nil {
		logger.Log(logger.Fields{"serial": d.Serial, "profile": profile}).Warn("Non-existing RGB profile")
		return 0
	}
	d.DeviceProfile.RGBProfile = profile // Set profile
	d.saveDeviceProfile()                // Save profile
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.setDeviceColor()
	return 1
}

// ChangeDeviceBrightness will change device brightness
func (d *Device) ChangeDeviceBrightness(mode uint8) uint8 {
	if d.lightingDefaults != nil {
		d.lightingMutation.Lock()
		defer d.lightingMutation.Unlock()
		d.lightingMu.Lock()
		defer d.lightingMu.Unlock()
		if d.lightingReady() != nil || mode > 3 {
			return 0
		}
		profile := *d.DeviceProfile
		profile.Brightness = mode
		if d.persistLightingProfile(profile) != nil {
			return 0
		}
		return 1
	}

	d.DeviceProfile.Brightness = mode
	d.saveDeviceProfile()
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.setDeviceColor()
	return 1
}

// ChangeDeviceBrightnessValue will change device brightness via slider
func (d *Device) ChangeDeviceBrightnessValue(value uint8) uint8 {
	if d.lightingDefaults != nil {
		if d.SetLightingBrightness(value) != nil {
			return 0
		}
		return 1
	}

	if !d.Connected {
		return 0
	}
	if value < 0 || value > 100 {
		return 0
	}

	d.DeviceProfile.BrightnessSlider = &value
	d.saveDeviceProfile()

	if d.DeviceProfile.RGBProfile == "static" || d.DeviceProfile.RGBProfile == "headset" {
		if d.activeRgb != nil {
			d.activeRgb.Exit <- true
			d.activeRgb = nil
		}
		d.setDeviceColor()
	}
	return 1
}

// SchedulerBrightness will change device brightness via scheduler
func (d *Device) SchedulerBrightness(value uint8) uint8 {
	if d.lightingDefaults != nil {
		if d.setLightingDarkness(true, value == 0) != nil {
			return 0
		}
		return 1
	}

	if value == 0 {
		d.DeviceProfile.OriginalBrightness = *d.DeviceProfile.BrightnessSlider
		d.DeviceProfile.BrightnessSlider = &value
	} else {
		d.DeviceProfile.BrightnessSlider = &d.DeviceProfile.OriginalBrightness
	}

	d.saveDeviceProfile()
	if d.DeviceProfile.RGBProfile == "static" || d.DeviceProfile.RGBProfile == "headset" {
		if d.activeRgb != nil {
			d.activeRgb.Exit <- true
			d.activeRgb = nil
		}
		d.setDeviceColor()
	}
	return 1
}

// SaveUserProfile will generate a new user profile configuration and save it to a file
func (d *Device) SaveUserProfile(profileName string) uint8 {
	if d.DeviceProfile != nil {
		profilePath := pwd + "/database/profiles/" + d.Serial + "-" + profileName + ".json"

		newProfile := d.DeviceProfile
		newProfile.Path = profilePath
		newProfile.Active = false

		buffer, err := json.Marshal(newProfile)
		if err != nil {
			logger.Log(logger.Fields{"error": err}).Error("Unable to convert to json format")
			return 0
		}

		// Create profile filename
		file, err := os.Create(profilePath)
		if err != nil {
			logger.Log(logger.Fields{"error": err, "location": newProfile.Path}).Error("Unable to create new device profile")
			return 0
		}

		_, err = file.Write(buffer)
		if err != nil {
			logger.Log(logger.Fields{"error": err, "location": newProfile.Path}).Error("Unable to write data")
			return 0
		}

		err = file.Close()
		if err != nil {
			logger.Log(logger.Fields{"error": err, "location": newProfile.Path}).Error("Unable to close file handle")
			return 0
		}
		d.loadDeviceProfiles()
		return 1
	}
	return 0
}

// SaveHeadsetZoneColors will save headset zone colors
func (d *Device) SaveHeadsetZoneColors(zoneColors map[int]rgb.Color) uint8 {
	if d.lightingDefaults != nil {
		if d.saveLightingHeadsetColors(zoneColors) != nil {
			return 0
		}
		return 1
	}

	i := 0
	if d.DeviceProfile == nil {
		return 0
	}

	// Zone Colors
	for key, zone := range zoneColors {
		if zone.Red > 255 ||
			zone.Green > 255 ||
			zone.Blue > 255 ||
			zone.Red < 0 ||
			zone.Green < 0 ||
			zone.Blue < 0 {
			continue
		}
		if zoneColor, ok := d.DeviceProfile.ZoneColors[key]; ok {
			zoneColor.Color.Red = zone.Red
			zoneColor.Color.Green = zone.Green
			zoneColor.Color.Blue = zone.Blue
			zoneColor.Color.Hex = fmt.Sprintf("#%02x%02x%02x", int(zone.Red), int(zone.Green), int(zone.Blue))
		}
		i++
	}

	if i > 0 {
		d.saveDeviceProfile()
		if d.activeRgb != nil {
			d.activeRgb.Exit <- true
			d.activeRgb = nil
		}
		d.setDeviceColor()
		return 1
	}
	return 0
}

// getManufacturer will return device manufacturer
func (d *Device) getDebugMode() {
	d.Debug = config.GetConfig().Debug
}

// setHardwareMode will switch a device to hardware mode
func (d *Device) setHardwareMode() {
	_, err := d.transfer(cmdHardwareMode, nil)
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to change device mode")
	}
}

// getBatterLevel will return initial battery level
func (d *Device) getBatterLevel() {
	batteryLevel, err := d.transfer(cmdBatteryLevel, nil)
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to get battery level")
	}
	d.BatteryLevel = binary.LittleEndian.Uint16(batteryLevel[4:6]) / 10
	stats.UpdateBatteryStats(d.Serial, d.Product, d.BatteryLevel, 2)
}

// setSoftwareMode will switch a device to software mode
func (d *Device) setSoftwareMode() {
	_, err := d.transfer(cmdSoftwareMode, nil)
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to change device mode")
	}
}

// SetSleepMode will switch a device to sleep mode
func (d *Device) SetSleepMode() {
	_, err := d.transfer(cmdSleepMode, nil)
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to change device mode")
	}
	//d.Connected = false
}

// GetSleepMode will return current sleep mode
func (d *Device) GetSleepMode() int {
	if d.DeviceProfile != nil {
		return d.DeviceProfile.SleepMode
	}
	return 0
}

// getDeviceFirmware will return a device firmware version out as string
func (d *Device) getDeviceFirmware() {
	fw, err := d.transfer(
		cmdGetFirmware,
		nil,
	)
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to write to a device")
	}

	v1, v2, v3 := int(fw[4]), int(fw[5]), int(fw[6])
	d.Firmware = fmt.Sprintf("%d.%d.%d", v1, v2, v3)
}

// saveDeviceProfile will save device profile for persistent configuration
func (d *Device) saveDeviceProfile() {
	var defaultBrightness = uint8(100)
	profilePath := pwd + "/database/profiles/" + d.Serial + ".json"

	deviceProfile := &DeviceProfile{
		Product:            d.Product,
		Serial:             d.Serial,
		Path:               profilePath,
		BrightnessSlider:   &defaultBrightness,
		OriginalBrightness: 100,
	}

	if d.DeviceProfile == nil {
		deviceProfile.RGBProfile = "headset"
		deviceProfile.Label = "Headset"
		deviceProfile.Active = true
		deviceProfile.ZoneColors = map[int]ZoneColors{
			0: { // Logo
				ColorIndex: []int{0, 3, 6},
				Color: &rgb.Color{
					Red:        255,
					Green:      255,
					Blue:       0,
					Brightness: 1,
					Hex:        fmt.Sprintf("#%02x%02x%02x", 255, 255, 0),
				},
				Name: "Logo",
			},
			1: { // Microphone
				ColorIndex: []int{2, 5, 8},
				Color: &rgb.Color{
					Red:        0,
					Green:      255,
					Blue:       255,
					Brightness: 1,
					Hex:        fmt.Sprintf("#%02x%02x%02x", 0, 255, 255),
				},
				Name: "Microphone",
			},
			2: { // Indicator LED
				ColorIndex: []int{1, 4, 7},
				Color: &rgb.Color{
					Red:        0,
					Green:      255,
					Blue:       255,
					Brightness: 1,
					Hex:        fmt.Sprintf("#%02x%02x%02x", 0, 255, 255),
				},
				Name: "Indicator LED",
			},
		}

		deviceProfile.SleepMode = 15
		deviceProfile.MuteIndicator = 0
		deviceProfile.DisableMicIndicator = 0
		deviceProfile.Equalizers = map[int]Equalizer{
			1:  {Name: "32", Value: 0},
			2:  {Name: "64", Value: 0},
			3:  {Name: "125", Value: 0},
			4:  {Name: "250", Value: 0},
			5:  {Name: "500", Value: 0},
			6:  {Name: "1K", Value: 0},
			7:  {Name: "2K", Value: 0},
			8:  {Name: "4K", Value: 0},
			9:  {Name: "8K", Value: 0},
			10: {Name: "16K", Value: 0},
		}
	} else {
		if d.DeviceProfile.Equalizers == nil {
			deviceProfile.Equalizers = map[int]Equalizer{
				1:  {Name: "32", Value: 0},
				2:  {Name: "64", Value: 0},
				3:  {Name: "125", Value: 0},
				4:  {Name: "250", Value: 0},
				5:  {Name: "500", Value: 0},
				6:  {Name: "1K", Value: 0},
				7:  {Name: "2K", Value: 0},
				8:  {Name: "4K", Value: 0},
				9:  {Name: "8K", Value: 0},
				10: {Name: "16K", Value: 0},
			}
		} else {
			deviceProfile.Equalizers = d.DeviceProfile.Equalizers
		}

		if d.DeviceProfile.BrightnessSlider == nil {
			deviceProfile.BrightnessSlider = &defaultBrightness
			d.DeviceProfile.BrightnessSlider = &defaultBrightness
		} else {
			deviceProfile.BrightnessSlider = d.DeviceProfile.BrightnessSlider
		}
		deviceProfile.Active = d.DeviceProfile.Active
		deviceProfile.Brightness = d.DeviceProfile.Brightness
		deviceProfile.OriginalBrightness = d.DeviceProfile.OriginalBrightness
		deviceProfile.RGBProfile = d.DeviceProfile.RGBProfile
		deviceProfile.Label = d.DeviceProfile.Label
		deviceProfile.ZoneColors = d.DeviceProfile.ZoneColors
		deviceProfile.SleepMode = d.DeviceProfile.SleepMode
		deviceProfile.MuteIndicator = d.DeviceProfile.MuteIndicator
		deviceProfile.DisableMicIndicator = d.DeviceProfile.DisableMicIndicator

		if len(d.DeviceProfile.Path) < 1 {
			deviceProfile.Path = profilePath
			d.DeviceProfile.Path = profilePath
		} else {
			deviceProfile.Path = d.DeviceProfile.Path
		}
		deviceProfile.RgbOff = d.DeviceProfile.RgbOff
	}

	// Fix profile paths if folder database/ folder is moved
	filename := filepath.Base(deviceProfile.Path)
	path := fmt.Sprintf("%s/database/profiles/%s", pwd, filename)
	if deviceProfile.Path != path {
		logger.Log(logger.Fields{"original": deviceProfile.Path, "new": path}).Warn("Detected mismatching device profile path. Fixing paths...")
		deviceProfile.Path = path
	}

	// Save profile
	if err := common.SaveJsonData(deviceProfile.Path, deviceProfile); err != nil {
		logger.Log(logger.Fields{"error": err, "location": deviceProfile.Path}).Error("Unable to write device profile data")
		return
	}

	d.loadDeviceProfiles()
}

// setEqualizer will set audio equalizer
func (d *Device) setEqualizer() {
	if d.DeviceProfile == nil {
		return
	}

	if d.DeviceProfile.Equalizers == nil {
		return
	}

	if !audio.GetAudio().Enabled {
		return
	}

	for k, v := range d.DeviceProfile.Equalizers {
		audio.SetBand(k, v.Value)
	}
}

// GetEqualizers will return equalizers
func (d *Device) GetEqualizers() interface{} {
	if d.DeviceProfile == nil || d.DeviceProfile.Equalizers == nil {
		return nil
	}
	return d.DeviceProfile.Equalizers
}

// UpdateEqualizer will update device equalizer
func (d *Device) UpdateEqualizer(values map[int]float64) uint8 {
	tmp := map[int]float64{}

	if d.DeviceProfile == nil || d.DeviceProfile.Equalizers == nil {
		return 0
	}

	updated := 0

	for key, value := range values {
		if key < 1 || key > 10 {
			continue
		}

		if value > 12 || value < -12 {
			value = 0
		}

		equalizer, ok := d.DeviceProfile.Equalizers[key]
		if !ok {
			continue
		}

		if equalizer.Value == value {
			continue
		}

		equalizer.Value = value
		d.DeviceProfile.Equalizers[key] = equalizer

		tmp[key] = value
		updated++
	}

	if updated > 0 {
		d.saveDeviceProfile()
		if len(tmp) > 0 && audio.GetAudio().Enabled {
			for k, v := range tmp {
				audio.SetBand(k, v)
			}
		}
		return 1
	} else {
		return 2
	}
}

// UpdateMuteIndicator will update device mute indicator
func (d *Device) UpdateMuteIndicator(value int) uint8 {
	if d.DeviceProfile != nil {
		d.DeviceProfile.DisableMicIndicator = value
		d.saveDeviceProfile()
		if d.activeRgb != nil {
			d.activeRgb.Exit <- true
			d.activeRgb = nil
		}
		d.setDeviceColor()
		return 1
	}
	return 0
}

// UpdateSleepTimer will update device sleep timer
func (d *Device) UpdateSleepTimer(minutes int) uint8 {
	if d.DeviceProfile != nil {
		d.DeviceProfile.SleepMode = minutes
		d.saveDeviceProfile()
		d.setSleepTimer()
		return 1
	}
	return 0
}

// setSleepTimer will set device sleep timer
func (d *Device) setSleepTimer() uint8 {
	if d.Exit {
		return 0
	}
	if d.DeviceProfile != nil {
		if d.DeviceProfile.SleepMode == 0 {
			_, err := d.transfer(cmdOpenSleepWriteEndpoint, cmdDisable)
			if err != nil {
				logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Warn("Unable to change device sleep timer")
				return 0
			}
			return 1
		} else {
			_, err := d.transfer(cmdOpenSleepWriteEndpoint, cmdEnable)
			if err != nil {
				logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Warn("Unable to change device sleep timer")
				return 0
			}

			buf := make([]byte, 4)
			sleep := d.DeviceProfile.SleepMode * (60 * 1000)
			binary.LittleEndian.PutUint32(buf, uint32(sleep))

			_, err = d.transfer(cmdSleep, buf)
			if err != nil {
				logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Warn("Unable to change device sleep timer")
				return 0
			}
			return 1
		}
	}
	return 0
}

// loadDeviceProfiles will load custom user profiles
func (d *Device) loadDeviceProfiles() {
	profileList := make(map[string]*DeviceProfile)
	userProfileDirectory := pwd + "/database/profiles/"

	files, err := os.ReadDir(userProfileDirectory)
	if err != nil {
		logger.Log(logger.Fields{"error": err, "location": userProfileDirectory, "serial": d.Serial}).Error("Unable to read content of a folder")
		return
	}

	for _, fi := range files {
		pf := &DeviceProfile{}
		if fi.IsDir() {
			continue // Exclude folders if any
		}

		// Define a full path of filename
		profileLocation := userProfileDirectory + fi.Name()

		// Check if filename has .json extension
		if !common.IsValidExtension(profileLocation, ".json") {
			continue
		}

		fileName := strings.Split(fi.Name(), ".")[0]
		if !common.AlphanumericDashRegex.MatchString(fileName) {
			continue
		}

		fileSerial := ""
		if strings.Contains(fileName, "-") {
			fileSerial = strings.Split(fileName, "-")[0]
		} else {
			fileSerial = fileName
		}

		if fileSerial != d.Serial {
			continue
		}

		file, err := os.Open(profileLocation)
		if err != nil {
			logger.Log(logger.Fields{"error": err, "serial": d.Serial, "location": profileLocation}).Warn("Unable to load profile")
			continue
		}
		if err = json.NewDecoder(file).Decode(pf); err != nil {
			logger.Log(logger.Fields{"error": err, "serial": d.Serial, "location": profileLocation}).Warn("Unable to decode profile")
			continue
		}
		err = file.Close()
		if err != nil {
			logger.Log(logger.Fields{"location": profileLocation, "serial": d.Serial}).Warn("Failed to close file handle")
		}

		if pf.Serial == d.Serial {
			if fileName == d.Serial {
				profileList["default"] = pf
			} else {
				name := strings.Split(fileName, "-")[1]
				profileList[name] = pf
			}
			logger.Log(logger.Fields{"location": profileLocation, "serial": d.Serial}).Info("Loaded custom user profile")
		}
	}
	d.UserProfiles = profileList
	d.getDeviceProfile()
}

// getDeviceProfile will load persistent device configuration
func (d *Device) getDeviceProfile() {
	if len(d.UserProfiles) == 0 {
		logger.Log(logger.Fields{"serial": d.Serial}).Warn("No profile found for device. Probably initial start")
	} else {
		for _, pf := range d.UserProfiles {
			if pf.Active {
				d.DeviceProfile = pf
			}
		}
	}
}

// getMuteStatus will check mute status on startup
func (d *Device) getMuteStatus() {
	mute, err := common.GetPulseAudioMuteStatus()
	if err == nil {
		if mute {
			d.MuteStatus = 1
		} else {
			d.MuteStatus = 0
		}
	} else {
		mute, err = common.GetAlsaMuteStatus()
		if err == nil {
			if mute {
				d.MuteStatus = 1
			} else {
				d.MuteStatus = 0
			}
		}
	}
}

// initLeds will initialize LED endpoint
func (d *Device) initLeds() {
	_, err := d.transfer(cmdOpenEndpoint, nil)
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to change device mode")
	}
}

// ControlDeviceRgb will change device brightness via schedulerSchedulerBrightness
func (d *Device) ControlDeviceRgb(value bool) {
	if d.lightingDefaults != nil {
		_ = d.setLightingDarkness(false, value)
		return
	}

	if d.DeviceProfile == nil {
		return
	}

	d.DeviceProfile.RgbOff = value
	d.saveDeviceProfile()

	if d.Connected {
		if d.activeRgb != nil {
			d.activeRgb.Exit <- true
			d.activeRgb = nil
		}
		d.setDeviceColor()
	}
}

// setDeviceColor will activate and set device RGB
func (d *Device) setDeviceColor() {
	d.lightingMu.Lock()
	unavailable := d.Exit || !d.Connected || d.dev == nil
	d.lightingMu.Unlock()
	if unavailable {
		return
	}
	buf := make([]byte, d.LEDChannels*3)
	effect, brightness, off := d.lightingOutputState()
	zones := d.lightingOutputZones(brightness, effect == "headset")
	if d.DeviceProfile == nil {
		logger.Log(logger.Fields{"serial": d.Serial}).Error("Unable to set color. DeviceProfile is null!")
		return
	}

	if off {
		for _, zoneColor := range zones {
			zoneColorIndexRange := zoneColor.ColorIndex
			for key, zoneColorIndex := range zoneColorIndexRange {
				switch key {
				case 0: // Red
					buf[zoneColorIndex] = 0x00
				case 1: // Green
					buf[zoneColorIndex] = 0x00
				case 2: // Blue
					buf[zoneColorIndex] = 0x00
				}
			}
		}
		d.writeColor(buf)
		return
	}

	if effect == "headset" {
		for _, zoneColor := range zones {
			zoneColorIndexRange := zoneColor.ColorIndex
			for key, zoneColorIndex := range zoneColorIndexRange {
				switch key {
				case 0: // Red
					buf[zoneColorIndex] = byte(zoneColor.Color.Red)
				case 1: // Green
					buf[zoneColorIndex] = byte(zoneColor.Color.Green)
				case 2: // Blue
					buf[zoneColorIndex] = byte(zoneColor.Color.Blue)
				}
			}
		}
		d.writeColor(buf)
		return
	}

	if effect == "static" {
		profile := d.GetRgbProfile("static")
		if profile == nil {
			return
		}

		profile.StartColor.Brightness = rgb.GetBrightnessValueFloat(brightness)
		profileColor := rgb.ModifyBrightness(profile.StartColor)
		for _, zoneColor := range zones {
			zoneColorIndexRange := zoneColor.ColorIndex
			for key, zoneColorIndex := range zoneColorIndexRange {
				switch key {
				case 0: // Red
					buf[zoneColorIndex] = byte(profileColor.Red)
				case 1: // Green
					buf[zoneColorIndex] = byte(profileColor.Green)
				case 2: // Blue
					buf[zoneColorIndex] = byte(profileColor.Blue)
				}
			}
		}
		d.writeColor(buf)
		return
	}

	d.activeRgb = rgb.Exit()
	active := d.activeRgb
	go func(lightChannels int) {
		startTime := time.Now()

		// Generate random colors
		active.RGBStartColor = rgb.GenerateRandomColor(1)
		active.RGBEndColor = rgb.GenerateRandomColor(1)

		for {
			select {
			case <-active.Exit:
				return
			default:
				effect, brightness, _ := d.lightingOutputState()
				zones := d.lightingOutputZones(brightness, false)
				buff := make([]byte, 0)
				rgbCustomColor := true
				profile := d.GetRgbProfile(effect)
				if profile == nil {
					for i := 0; i < d.ChangeableLedChannels*3; i++ {
						buff = append(buff, []byte{0, 0, 0}...)
					}
					time.Sleep(40 * time.Millisecond)
					continue
				}
				rgbModeSpeed := common.FClamp(profile.Speed, 0.1, 10)

				// Check if we have custom colors
				if (rgb.Color{}) == profile.StartColor || (rgb.Color{}) == profile.EndColor {
					rgbCustomColor = false
				}

				r := rgb.New(
					d.ChangeableLedChannels,
					rgbModeSpeed,
					nil,
					nil,
					profile.Brightness,
					common.Clamp(profile.Smoothness, 1, 100),
					time.Duration(rgbModeSpeed)*time.Second,
					rgbCustomColor,
				)

				if rgbCustomColor {
					r.RGBStartColor = &profile.StartColor
					r.RGBEndColor = &profile.EndColor
					r.RGBMiddleColor = &profile.MiddleColor
				} else {
					r.RGBStartColor = active.RGBStartColor
					r.RGBEndColor = active.RGBEndColor
					r.RGBMiddleColor = active.RGBMiddleColor
				}

				if r.RGBMiddleColor == nil {
					r.RGBMiddleColor = &rgb.Color{}
				}

				// Brightness
				r.RGBBrightness = rgb.GetBrightnessValueFloat(brightness)
				r.RGBStartColor.Brightness = r.RGBBrightness
				r.RGBEndColor.Brightness = r.RGBBrightness
				r.RGBMiddleColor.Brightness = r.RGBBrightness

				switch effect {
				case "off":
					{
						for n := 0; n < d.ChangeableLedChannels; n++ {
							buff = append(buff, []byte{0, 0, 0}...)
						}
					}
				case "rainbow":
					{
						r.Rainbow(startTime)
						buff = append(buff, r.Output...)
					}
				case "pastelrainbow":
					{
						r.PastelRainbow(startTime)
						buff = append(buff, r.Output...)
					}
				case "watercolor":
					{
						r.Watercolor(startTime)
						buff = append(buff, r.Output...)
					}
				case "gradient":
					{
						r.ColorshiftGradient(startTime, profile.Gradients, profile.Speed)
						buff = append(buff, r.Output...)
					}
				case "cpu-temperature":
					{
						r.MinTemp = profile.MinTemp
						r.MaxTemp = profile.MaxTemp
						r.Temperature(float64(d.CpuTemp))
						buff = append(buff, r.Output...)
					}
				case "gpu-temperature":
					{
						r.MinTemp = profile.MinTemp
						r.MaxTemp = profile.MaxTemp
						r.Temperature(float64(d.GpuTemp))
						buff = append(buff, r.Output...)
					}
				case "colorpulse":
					{
						r.Colorpulse(&startTime)
						buff = append(buff, r.Output...)
					}
				case "static":
					{
						r.Static()
						buff = append(buff, r.Output...)
					}
				case "rotator":
					{
						r.Rotator(&startTime)
						buff = append(buff, r.Output...)
					}
				case "wave":
					{
						r.Wave(&startTime)
						buff = append(buff, r.Output...)
					}
				case "storm":
					{
						r.Storm()
						buff = append(buff, r.Output...)
					}
				case "flickering":
					{

						r.Flickering(&startTime)
						buff = append(buff, r.Output...)
					}
				case "flame":
					{

						r.Flame(&startTime)
						buff = append(buff, r.Output...)
					}
				case "aurora":
					{

						r.Aurora(&startTime)
						buff = append(buff, r.Output...)
					}
				case "cyberpunkglitch":
					{

						r.CyberpunkGlitch(&startTime)
						buff = append(buff, r.Output...)
					}
				case "tokyonight":
					{

						r.TokyoNight(&startTime)
						buff = append(buff, r.Output...)
					}
				case "colorshift":
					{
						r.Colorshift(&startTime, active)
						buff = append(buff, r.Output...)
					}
				case "circleshift":
					{
						r.CircleShift(&startTime)
						buff = append(buff, r.Output...)
					}
				case "circle":
					{
						r.Circle(&startTime)
						buff = append(buff, r.Output...)
					}
				case "spinner":
					{
						r.Spinner(&startTime)
						buff = append(buff, r.Output...)
					}
				case "colorwarp":
					{
						r.Colorwarp(&startTime, active)
						buff = append(buff, r.Output...)
					}
				}
				zoneKeys := make([]int, 0, len(zones))
				for key := range zones {
					zoneKeys = append(zoneKeys, key)
				}
				sort.Ints(zoneKeys)

				m := 0
				for _, key := range zoneKeys {
					zoneColor := zones[key]
					for _, zoneColorIndex := range zoneColor.ColorIndex {
						if m >= len(buff) {
							break
						}
						buf[zoneColorIndex] = buff[m]
						m++
					}
				}

				d.writeColor(buf)
				time.Sleep(40 * time.Millisecond)
			}
		}
	}(d.ChangeableLedChannels)
}

// writeColor will write color data to the device
func (d *Device) writeColor(data []byte) {
	d.lightingMu.Lock()
	stopped := d.Exit || !d.Connected || d.dev == nil
	d.lightingMu.Unlock()
	if stopped {
		return
	}

	data = d.lightingMicFrame(data)
	if d.lightingWrite != nil {
		d.lightingWrite(data)
		return
	}

	buffer := make([]byte, len(data)+headerWriteSize)
	binary.LittleEndian.PutUint16(buffer[0:2], uint16(len(data)))
	copy(buffer[headerWriteSize:], data)
	_, err := d.transfer(cmdWriteColor, buffer)
	if err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to write to color endpoint")
	}
}

// transfer will send data to a device and retrieve device output
func (d *Device) transfer(endpoint, buffer []byte) ([]byte, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	bufferW := make([]byte, bufferSizeWrite)
	bufferW[1] = 0x02
	bufferW[2] = d.Endpoint
	endpointHeaderPosition := bufferW[headerSize : headerSize+len(endpoint)]
	copy(endpointHeaderPosition, endpoint)
	if len(buffer) > 0 {
		copy(bufferW[headerSize+len(endpoint):headerSize+len(endpoint)+len(buffer)], buffer)
	}

	reports := make([]byte, 1)
	err := d.dev.SetNonblock(true)
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to SetNonblock")
	}

	for {
		n, e := d.dev.Read(reports)
		if e != nil {
			if n < 0 {
				//
			}
			if e == hid.ErrTimeout || n == 0 {
				break
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	err = d.dev.SetNonblock(false)
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to SetNonblock")
	}

	bufferR := make([]byte, bufferSize)

	if _, err := d.dev.Write(bufferW); err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to write to a device")
		return bufferR, err
	}

	if _, err := d.dev.Read(bufferR); err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to read data from device")
		return bufferR, err
	}
	return bufferR, nil
}

// NotifyMuteChanged will change microphone LED based on microphone status
func (d *Device) NotifyMuteChanged(status byte) {
	if status == 1 {
		if d.MuteStatus == 1 {
			d.MuteStatus = 0
		} else {
			d.MuteStatus = 1
		}
		mute, err := common.MuteWithPulseAudioEx()
		if err == nil {
			if mute {
				d.MuteStatus = 1
			} else {
				d.MuteStatus = 0
			}
		} else {
			mute, err = common.MuteWithALSAEx()
			if err == nil {
				if mute {
					d.MuteStatus = 1
				} else {
					d.MuteStatus = 0
				}
			}
		}

		if d.activeRgb != nil {
			d.activeRgb.Exit <- true
			d.activeRgb = nil
		}
		d.setDeviceColor()
	}
}

// ModifyBatteryLevel will modify battery level
func (d *Device) ModifyBatteryLevel(batteryLevel uint16) {
	d.BatteryLevel = batteryLevel
	stats.UpdateBatteryStats(d.Serial, d.Product, d.BatteryLevel, 2)
}
