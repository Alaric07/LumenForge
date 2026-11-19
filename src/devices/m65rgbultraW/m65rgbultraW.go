package m65rgbultraW

// Package: CORSAIR M65 RGB ULTRA WIRELESS
// Author: Nikola Jurkovic
// License: GPL-3.0 or later

import (
	"LumenForge/src/cluster"
	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/inputmanager"
	"LumenForge/src/logger"
	"LumenForge/src/macro"
	"LumenForge/src/openrgb"
	"LumenForge/src/rgb"
	"LumenForge/src/stats"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ZoneColors struct {
	Color      *rgb.Color
	ColorIndex []int
	Name       string
}

type TiltOption struct {
	Name  string
	Value uint8
}

// DeviceProfile struct contains all device profile
type DeviceProfile struct {
	ProfileName        string `json:",omitempty"`
	Active             bool
	Path               string
	Product            string
	Serial             string
	Brightness         uint8
	RGBProfile         string
	BrightnessSlider   *uint8
	OriginalBrightness uint8
	Label              string
	Profile            int
	PollingRate        int
	ZoneColors         map[int]ZoneColors
	Profiles           map[int]DPIProfile
	SleepMode          int
	AngleSnapping      int
	ButtonOptimization int
	LiftHeight         int
	MultiGesture       int
	Tilts              map[int]TiltOption
	KeyAssignmentHash  string
	OpenRGBIntegration bool
	RGBCluster         bool
	RgbOff             bool
}

type DPIProfile struct {
	Name        string `json:"name"`
	Value       uint16
	PackerIndex int
	ColorIndex  map[int][]int
	Color       *rgb.Color
	Sniper      bool
}

type Device struct {
	lightingMu                sync.Mutex
	lightingMutation          sync.Mutex
	lightingCanonical         *canonicalLighting
	schedulerDark             bool
	userRGBOff                bool
	clusterOwned              bool
	openRGBOwned              bool
	externalGeneration        uint64
	externalFrame             uint64
	logoVersion               uint64
	nativeGeneration          uint64
	lightingWorker            *lightingWorker
	externalQueue             chan lightingExternalFrame
	externalStop              chan struct{}
	externalDone              chan struct{}
	externalStarted           bool
	lightingWrite             func([]byte)
	lightingProfileWrite      func(string, []byte) error
	lightingClusterRegister   func(*common.ClusterController)
	lightingClusterRemove     func(string)
	lightingOpenRGBReconcile  func(string, *common.OpenRGBController)
	lightingBeforeApply       func(string)
	lightingAfterApply        func(string, lightingApplyResult)
	lightingDPIWrite          func([]byte, []byte) error
	lightingInputBeforeWrite  func(connectionSession, []byte)
	lightingTemperatureCancel context.CancelFunc
	lightingTemperatureDone   chan struct{}
	lightingTemperatureProbe  func(context.Context, string) float32
	lightingTemperatureTicks  <-chan time.Time
	lightingReportWrite       func([]byte) (int, error)

	lightingClusterSession    uint64
	lightingOpenRGBSession    uint64
	lightingClusterRegistered bool
	lightingOpenRGBRegistered bool
	lightingHardwareReady     bool
	lightingStopOnce          sync.Once

	lightingAssignmentWrite func(string, []byte) error

	connectionGeneration       uint64
	connectionCancel           chan struct{}
	connecting                 bool
	connectBeforeComplete      func()
	sleepBeforeCommand         func(connectionSession)
	sleepAfterCommand          func(connectionSession, error)
	assignmentBeforeRetry      func(connectionSession)
	connectPending             bool
	inputRevision              uint64
	dpiValueRevision           uint64
	timerRevision              uint64
	assignmentRevision         uint64
	gestureRevision            uint64
	angleRevision              uint64
	liftHeightRevision         uint64
	buttonOptimizationRevision uint64
	assignmentReplayDeferred   bool
	softwareModeGeneration     uint64
	lastExternal               *externalLogoFrame
	lightingBinding            *common.Slipstream
	lightingEndpoint           byte

	Debug                 bool
	dev                   *common.Slipstream
	Manufacturer          string `json:"manufacturer"`
	Product               string `json:"product"`
	Serial                string `json:"serial"`
	Path                  string `json:"path"`
	Firmware              string `json:"firmware"`
	activeRgb             *rgb.ActiveRGB
	UserProfiles          map[string]*DeviceProfile `json:"userProfiles"`
	ProfileOrder          []string                  `json:"profileOrder"`
	Devices               map[int]string            `json:"devices"`
	DeviceProfile         *DeviceProfile
	OriginalProfile       *DeviceProfile
	Template              string
	VendorId              uint16
	ProductId             uint16
	SlipstreamId          uint16
	Brightness            map[int]string
	PollingRates          map[int]string
	SwitchModes           map[int]string
	KeyAssignmentTypes    map[int]string
	LEDChannels           int
	ChangeableLedChannels int
	CpuTemp               float32
	GpuTemp               float32
	Layouts               []string
	Rgb                   *rgb.RGB
	rgbMutex              sync.RWMutex
	Endpoint              byte
	SleepModes            map[int]string
	LiftHeights           map[int]string
	Connected             bool
	Exit                  bool
	deviceLock            sync.Mutex
	inputEventMu          sync.Mutex
	heldTilts             map[int]inputmanager.KeyAssignment
	tiltHoldWrite         func(uint8, uint16, bool)
	legacyProfileSelected func(string)
	inputMacroPlayback    func(uint16)
	macroMutex            sync.Mutex
	timerKeepAlive        *time.Ticker
	keepAliveChan         chan struct{}
	timer                 *time.Ticker
	autoRefreshChan       chan struct{}
	KeyAssignment         map[int]inputmanager.KeyAssignment
	InputActions          map[uint16]inputmanager.InputAction
	PressLoop             bool
	keyAssignmentFile     string
	BatteryLevel          uint16
	KeyAssignmentData     *inputmanager.KeyAssignment
	ModifierIndex         uint32
	SniperMode            bool
	MacroTracker          map[int]uint16
	RGBModes              []string
	queue                 chan []byte
	Usb                   bool
	MinDPI                int
	MaxDPI                int
	ZoneAmount            int
	DPIAmount             int
	checkOnlineMu         sync.Mutex
	stopRepeat            chan struct{}
	stopRepeatMutex       sync.Mutex
}

var (
	pwd                       = ""
	cmdSoftwareMode           = []byte{0x01, 0x03, 0x00, 0x02}
	cmdHardwareMode           = []byte{0x01, 0x03, 0x00, 0x01}
	cmdSleepMode              = []byte{0x01, 0x03, 0x00, 0x04}
	cmdGetFirmware            = []byte{0x02, 0x13}
	cmdWriteColor             = []byte{0x06, 0x00}
	cmdOpenEndpoint           = []byte{0x0d, 0x00, 0x01}
	cmdWrite                  = []byte{0x06, 0x01}
	cmdOpenWriteEndpoint      = []byte{0x0d, 0x01, 0x02}
	cmdCloseEndpoint          = []byte{0x05, 0x01, 0x01}
	cmdAngleSnapping          = []byte{0x01, 0x07, 0x00}
	cmdButtonOptimization     = []byte{0x01, 0xb0, 0x00}
	cmdLiftHeight             = []byte{0x01, 0x06, 0x00}
	cmdMultiGesture           = []byte{0x01, 0xbf, 0x00}
	cmdLeftTilt               = []byte{0x01, 0xb9, 0x00}
	cmdRightTilt              = []byte{0x01, 0xba, 0x00}
	cmdForwardTilt            = []byte{0x01, 0xb7, 0x00}
	cmdBackwardTilt           = []byte{0x01, 0xb8, 0x00}
	cmdActivateLeftTilt       = []byte{0x01, 0xbd, 0x00}
	cmdActivateRightTilt      = []byte{0x01, 0xbe, 0x00}
	cmdActivateFrontTilt      = []byte{0x01, 0xbb, 0x00}
	cmdActivateBackTilt       = []byte{0x01, 0xbc, 0x00}
	cmdBatteryLevel           = []byte{0x02, 0x0f}
	cmdOpenSleepWriteEndpoint = []byte{0x01, 0x0d, 0x00, 0x01}
	cmdSleep                  = map[int][]byte{0: {0x01, 0x37, 0x00}, 1: {0x01, 0x0e, 0x00}}
	cmdHeartbeat              = []byte{0x12}
	cmdSetDpi                 = map[int][]byte{
		0: {0x01, 0x21, 0x00},
		1: {0x01, 0x22, 0x00},
	}
	bufferSize        = 64
	bufferSizeWrite   = bufferSize + 1
	headerSize        = 2
	headerWriteSize   = 4
	keyAmount         = 12
	minDpiValue       = 100
	maxDpiValue       = 26000
	rgbProfileUpgrade = []string{"gradient", "pastelrainbow", "pastelspiralrainbow", "flame", "aurora", "cyberpunkglitch", "tokyonight"}
	rgbModes          = []string{
		"colorpulse",
		"colorshift",
		"colorwarp",
		"cpu-temperature",
		"flickering",
		"flame",
		"aurora", "cyberpunkglitch", "tokyonight", "gpu-temperature",
		"gradient",
		"mouse",
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

func Init(vendorId, slipstreamId, productId uint16, dev *common.Slipstream, endpoint byte, serial string) *Device {
	// Set global working directory
	pwd = config.GetPaths().MutableDataRoot

	// Init new struct with HID device
	d := &Device{
		dev:          dev,
		Path:         strconv.Itoa(int(endpoint)),
		Template:     "m65rgbultraW.html",
		VendorId:     vendorId,
		ProductId:    productId,
		SlipstreamId: slipstreamId,
		Serial:       serial,
		Endpoint:     endpoint,
		Firmware:     "n/a",
		Brightness: map[int]string{
			0: "RGB Profile",
			1: "33 %",
			2: "66 %",
			3: "100 %",
		},
		Product: "M65 RGB ULTRA",
		SleepModes: map[int]string{
			1:  "1 minute",
			5:  "5 minutes",
			10: "10 minutes",
			15: "15 minutes",
			30: "30 minutes",
			60: "1 hour",
		},
		LiftHeights: map[int]string{
			7: "Ultra Low",
			2: "Low",
			3: "Medium",
			4: "High",
			6: "Calibrated",
		},
		RGBModes:              rgbModes,
		LEDChannels:           2,
		ChangeableLedChannels: 2,
		keepAliveChan:         make(chan struct{}),
		timerKeepAlive:        &time.Ticker{},
		autoRefreshChan:       make(chan struct{}),
		timer:                 &time.Ticker{},
		PollingRates: map[int]string{
			0: "Not Set",
			1: "125 Hz / 8 msec",
			2: "250 Hz / 4 msec",
			3: "500 Hz / 2 msec",
			4: "1000 Hz / 1 msec",
		},
		SwitchModes: map[int]string{
			0: "Disabled",
			1: "Enabled",
		},
		KeyAssignmentTypes: map[int]string{
			0:  "None",
			1:  "Media Keys",
			2:  "DPI +",
			3:  "Keyboard",
			4:  "DPI -",
			8:  "Sniper",
			9:  "Mouse",
			10: "Macro",
			11: "Profile Switch",
		},
		InputActions:      inputmanager.GetInputActions(),
		keyAssignmentFile: "/database/key-assignments/m65RgbUltraW.json",
		MacroTracker:      make(map[int]uint16),
		MinDPI:            minDpiValue,
		MaxDPI:            maxDpiValue,
		ZoneAmount:        1,
		DPIAmount:         6,
	}

	d.getDebugMode()       // Debug mode
	d.loadDeviceProfiles() // Load all device profiles
	d.saveDeviceProfile()  // Bootstrap input preferences before canonical attachment
	if err := d.attachLightingRuntime(config.GetPaths()); err != nil {
		logger.Log(logger.Fields{"error": err}).Warn("Canonical W Lighting unavailable; retaining independent legacy Lighting")
		d.loadRgb()
	}
	d.loadKeyAssignments() // Key Assignments
	d.startQueueWorker()   // Queue
	logger.Log(logger.Fields{"serial": d.Serial, "product": d.Product}).Info("Device successfully initialized")
	return d
}

// GetRgbProfiles will return RGB profiles for a target device
func (d *Device) GetRgbProfiles() interface{} {
	if d.lightingAttached() {
		return d.canonicalRGBProfiles()
	}
	tmp := *d.Rgb

	// Filter unsupported modes out
	profiles := make(map[string]rgb.Profile, len(tmp.Profiles))
	for key, value := range tmp.Profiles {
		if slices.Contains(rgbModes, key) {
			profiles[key] = value
		}
	}
	tmp.Profiles = profiles
	return tmp
}

// Stop will stop all device operations and switch a device back to hardware mode
func (d *Device) Stop() {
	if d.lightingAttached() {
		d.stopCanonicalLighting(true)
		return
	}
	// Placeholder
}

// StopInternal will stop all device operations and switch a device back to hardware mode
func (d *Device) StopInternal() {
	if d.lightingAttached() {
		d.stopCanonicalLighting(true)
		return
	}
	d.Exit = true
	d.releaseTiltHolds()
	logger.Log(logger.Fields{"serial": d.Serial, "product": d.Product}).Info("Stopping device...")
	if d.activeRgb != nil {
		d.activeRgb.Stop()
	}

	if d.Connected {
		d.setHardwareMode()
	}
	logger.Log(logger.Fields{"serial": d.Serial, "product": d.Product}).Info("Device stopped")
}

// StopDirty will stop all device operations in a dirty way
func (d *Device) StopDirty() uint8 {
	if d.lightingAttached() {
		d.stopCanonicalLighting(false)
		return 1
	}
	d.Exit = true
	d.releaseTiltHolds()
	logger.Log(logger.Fields{"serial": d.Serial}).Info("Stopping device (dirty)...")

	if d.activeRgb != nil {
		d.activeRgb.Stop()
	}

	d.Connected = false
	logger.Log(logger.Fields{"serial": d.Serial, "product": d.Product}).Info("Device stopped")
	return 1
}

// SetConnected will change connected status
func (d *Device) SetConnected(value bool) {
	if d.lightingAttached() {
		if !value {
			d.disconnectLighting()
		} else {
			// Receiver startup/status wake paths call Connect separately, after
			// transfer has released the shared mutex. No receiver callback passes
			// true here under that mutex; do not invent another dispatch layer.
			d.Connect()
		}
		return
	}
	if d.Connected {
		if !value {
			cluster.Get().RemoveDeviceControllerBySerial(d.Serial)
			openrgb.RemoveDeviceControllerBySerial(d.Serial)
		}

		if d.activeRgb != nil {
			d.activeRgb.Exit <- true
			d.activeRgb = nil
		}
		d.Connected = value
		if !value {
			d.releaseTiltHolds()
		}
		time.Sleep(1000 * time.Millisecond)
	}
}

// checkDeviceOnline will check if device is online
func (d *Device) checkDeviceOnline() bool {
	d.checkOnlineMu.Lock()
	defer d.checkOnlineMu.Unlock()

	for i := 0; i < 10; i++ {
		_, err := d.transfer(cmdHeartbeat, nil, "checkDeviceOnline")
		if err == nil {
			return true
		}

		logger.Log(logger.Fields{
			"error":  err,
			"serial": d.Serial,
		}).Warn("Device is offline")

		if i < 9 {
			time.Sleep(1 * time.Second)
		}
	}
	return false
}

// Connect will connect to a device
func (d *Device) Connect() {
	if d.lightingAttached() {
		d.connectLighting()
		return
	}
	if !d.Connected {
		if !d.checkDeviceOnline() {
			return
		}
		d.Connected = true
		d.getDeviceFirmware()      // Firmware
		d.setSoftwareMode()        // Activate software mode
		d.getBatterLevel()         // Battery level
		d.setAngleSnapping()       // Angle snapping
		d.setButtonOptimization()  // Button optimization
		d.setLiftHeight()          // Lift Height
		d.setGestures()            // Multi gesture
		d.initLeds()               // Init LED ports
		d.setDeviceColor()         // Device color
		d.toggleDPI()              // DPI
		d.setupKeyAssignment()     // Setup key assignments
		d.setSleepTimer()          // Sleep
		d.setupOpenRGBController() // OpenRGB Controller
		d.setupClusterController() // RGB Cluster
	}
}

// loadRgb will load RGB file if found, or create the default.
func (d *Device) loadRgb() {
	if d.lightingAttached() {
		return
	}
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
	if d.lightingAttached() {
		return d.canonicalRGBProfile(profile)
	}
	if d.Rgb == nil {
		return nil
	}

	if val, ok := d.Rgb.Profiles[profile]; ok {
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
	if d.lightingAttached() {
		return d.changeCanonicalDeviceProfile(profileName)
	}
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	profile, ok := d.UserProfiles[profileName]
	if !ok || !d.Connected || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	d.DeviceProfile.Active = false
	d.lightingMu.Unlock()
	d.saveDeviceProfile()
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.lightingMu.Lock()
	// Re-resolve after the loader replaces map pointers. Profile mutations are
	// serialized, but the map lock never spans persistence or hardware output.
	if loaded := d.UserProfiles[profileName]; loaded != nil {
		profile = loaded
	}
	profile.Active = true
	d.DeviceProfile = profile
	d.lightingMu.Unlock()
	d.saveDeviceProfile()
	d.lightingMutation.Unlock()
	d.setDeviceColor()
	d.toggleDPI()
	d.loadKeyAssignments()
	d.setupKeyAssignment()
	return 1
}

// DeleteDeviceProfile deletes a device profile and its JSON file
func (d *Device) DeleteDeviceProfile(profileName string) uint8 {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
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
	d.rebuildProfileOrder()

	return 1
}

// rotateDeviceProfile will rotate and activate next user profile
func (d *Device) rotateDeviceProfile() {
	if d.lightingAttached() {
		d.rotateCanonicalProfile()
		return
	}
	d.lightingMu.Lock()
	next := ""
	if d.DeviceProfile != nil && len(d.ProfileOrder) > 0 {
		next = d.ProfileOrder[0]
		for i, name := range d.ProfileOrder {
			if profile := d.UserProfiles[name]; profile != nil && profile.Active {
				next = d.ProfileOrder[(i+1)%len(d.ProfileOrder)]
				break
			}
		}
	}
	d.lightingMu.Unlock()
	if next != "" {
		if d.legacyProfileSelected != nil {
			d.legacyProfileSelected(next)
		}
		// The selected NAME may disappear; ChangeDeviceProfile revalidates it.
		d.ChangeDeviceProfile(next)
	}
}

// saveRgbProfile will save rgb profile data
func (d *Device) saveRgbProfile() {
	if d.lightingAttached() {
		return
	}
	rgbDirectory := pwd + "/database/rgb/"
	rgbFilename := rgbDirectory + d.Serial + ".json"
	if common.FileExists(rgbFilename) {
		if err := common.SaveJsonData(rgbFilename, d.Rgb); err != nil {
			logger.Log(logger.Fields{"error": err, "location": rgbFilename}).Error("Unable to write rgb profile data")
			return
		}
	}
}

// UpdateAngleSnapping will update angle snapping mode
func (d *Device) UpdateAngleSnapping(angleSnappingMode int) uint8 {
	if d.lightingAttached() {
		result := d.mutateInputProfile(false, func(p *DeviceProfile) error {
			if _, ok := d.SwitchModes[angleSnappingMode]; !ok || angleSnappingMode < 0 || angleSnappingMode > 1 {
				return fmt.Errorf("unsupported angle")
			}
			p.AngleSnapping = angleSnappingMode
			return nil
		})
		if result == 1 {
			d.programLightingPreference("angle")
		}
		return result
	}
	if !d.Connected {
		return 0
	}

	if d.DeviceProfile == nil {
		return 0
	}

	if d.DeviceProfile.AngleSnapping == angleSnappingMode {
		return 0
	}

	d.DeviceProfile.AngleSnapping = angleSnappingMode
	d.saveDeviceProfile()
	d.setAngleSnapping()
	return 1
}

// UpdateButtonOptimization will update button response optimization mode
func (d *Device) UpdateButtonOptimization(buttonOptimizationMode int) uint8 {
	if d.lightingAttached() {
		result := d.mutateInputProfile(false, func(p *DeviceProfile) error {
			if _, ok := d.SwitchModes[buttonOptimizationMode]; !ok || buttonOptimizationMode < 0 || buttonOptimizationMode > 1 {
				return fmt.Errorf("unsupported button")
			}
			p.ButtonOptimization = buttonOptimizationMode
			return nil
		})
		if result == 1 {
			result = lightingResult(d.programLightingPreference("button"))
		}
		return result
	}
	if !d.Connected {
		return 0
	}

	if d.DeviceProfile == nil {
		return 0
	}

	if d.DeviceProfile.ButtonOptimization == buttonOptimizationMode {
		return 0
	}

	d.DeviceProfile.ButtonOptimization = buttonOptimizationMode
	d.saveDeviceProfile()
	d.setButtonOptimization()
	return 1
}

// UpdateLiftHeight will update lift height
func (d *Device) UpdateLiftHeight(liftHeight int) uint8 {
	if d.lightingAttached() {
		result := d.mutateInputProfile(false, func(p *DeviceProfile) error {
			if _, ok := d.LiftHeights[liftHeight]; !ok {
				return fmt.Errorf("unsupported lift")
			}
			p.LiftHeight = liftHeight
			return nil
		})
		if result == 1 {
			result = lightingResult(d.programLightingPreference("lift"))
		}
		return result
	}
	if d.DeviceProfile == nil {
		return 0
	}

	if liftHeight < 2 || liftHeight > 6 {
		return 0
	}
	if d.DeviceProfile.LiftHeight == liftHeight {
		return 0
	}

	d.DeviceProfile.LiftHeight = liftHeight
	d.saveDeviceProfile()
	d.setLiftHeight()
	return 1
}

// ProcessNewGradientColor will create new gradient color
func (d *Device) ProcessNewGradientColor(profileName string) (uint8, uint) {
	if d.lightingAttached() {
		return d.changeLightingGradient(profileName, false)
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
	if d.lightingAttached() {
		return d.changeLightingGradient(profileName, true)
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
	if d.lightingAttached() {
		return d.updateCanonicalRGBProfile(profileName, profile)
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
	if d.lightingAttached() {
		return lightingResult(d.SetLightingEffect(profile))
	}
	if !d.Connected {
		return 0
	}

	if d.GetRgbProfile(profile) == nil {
		logger.Log(logger.Fields{"serial": d.Serial, "profile": profile}).Warn("Non-existing RGB profile")
		return 0
	}

	if d.DeviceProfile.RGBCluster {
		return 5
	}
	if d.DeviceProfile.OpenRGBIntegration {
		return 4
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

// setupOpenRGBController will create Cluster Controller for RGB Cluster
func (d *Device) setupClusterController() {
	if d.lightingAttached() {
		d.refreshLightingRegistrations(true, false)
		return
	}
	if d.DeviceProfile == nil {
		return
	}

	if d.DeviceProfile == nil || !d.DeviceProfile.RGBCluster {
		return
	}

	clusterController := &common.ClusterController{
		Product:      d.Product,
		Serial:       d.Serial,
		LedChannels:  uint32(d.ChangeableLedChannels),
		WriteColorEx: d.writeColorCluster,
	}

	cluster.Get().AddDeviceController(clusterController)
}

// setupOpenRGBController will create RGBController object for OpenRGB Client Integration
func (d *Device) setupOpenRGBController() {
	if d.lightingAttached() {
		d.refreshLightingRegistrations(false, true)
		return
	}
	controller := &common.OpenRGBController{
		Name:         d.Product,
		Vendor:       "Corsair", // Static value
		Description:  "LumenForge Backend Device",
		FwVersion:    d.Firmware,
		Serial:       d.Serial,
		Location:     fmt.Sprintf("Slipstream Ep: %s", d.Path), // Slipstream endpoint
		Zones:        nil,
		Colors:       make([]byte, d.ChangeableLedChannels*3),
		ActiveMode:   0,
		WriteColorEx: d.writeColorEx,
		DeviceType:   common.DeviceTypeMouse,
		ColorMode:    common.ColorModePerLed,
	}

	zone := []common.OpenRGBZone{
		{
			Name:     "Logo",
			NumLEDs:  uint32(1),
			ZoneType: common.ZoneTypeLinear,
		},
	}
	controller.Zones = zone
	openrgb.AddDeviceController(controller)
}

// ProcessSetOpenRgbIntegration will update OpenRGB integration status
func (d *Device) ProcessSetOpenRgbIntegration(enabled bool) uint8 {
	if d.lightingAttached() {
		return d.setLightingOwnership(false, enabled)
	}
	if !d.Connected {
		return 0
	}
	if d.DeviceProfile == nil {
		return 0
	}
	if d.DeviceProfile.RGBCluster {
		return 2
	}

	d.clearQueue()
	d.DeviceProfile.OpenRGBIntegration = enabled
	d.saveDeviceProfile() // Save profile
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.setDeviceColor()
	return 1
}

// ProcessSetRgbCluster will update OpenRGB integration status
func (d *Device) ProcessSetRgbCluster(enabled bool) uint8 {
	if d.lightingAttached() {
		return d.setLightingOwnership(true, enabled)
	}
	if !d.Connected {
		return 0
	}
	if d.DeviceProfile == nil {
		return 0
	}
	if d.DeviceProfile.OpenRGBIntegration {
		return 2
	}

	d.DeviceProfile.RGBCluster = enabled
	d.saveDeviceProfile() // Save profile
	if d.activeRgb != nil {
		d.activeRgb.Exit <- true
		d.activeRgb = nil
	}
	d.setDeviceColor()

	if enabled {
		clusterController := &common.ClusterController{
			Product:      d.Product,
			Serial:       d.Serial,
			LedChannels:  uint32(d.ChangeableLedChannels),
			WriteColorEx: d.writeColorCluster,
		}

		cluster.Get().AddDeviceController(clusterController)
	} else {
		cluster.Get().RemoveDeviceControllerBySerial(d.Serial)
	}
	return 1
}

// ChangeDeviceBrightness will change device brightness
func (d *Device) ChangeDeviceBrightness(mode uint8) uint8 {
	if d.lightingAttached() {
		// Device-advertised presets; RGB Profile means no global dimming.
		values := [...]uint8{100, 33, 66, 100}
		if int(mode) >= len(values) {
			return 0
		}
		return lightingResult(d.SetLightingBrightness(values[mode]))
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
	if d.lightingAttached() {
		return lightingResult(d.SetLightingBrightness(value))
	}
	if !d.Connected {
		return 0
	}
	if value < 0 || value > 100 {
		return 0
	}

	d.DeviceProfile.BrightnessSlider = &value
	d.saveDeviceProfile()

	if d.DeviceProfile.RGBProfile == "static" || d.DeviceProfile.RGBProfile == "mouse" {
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
	if d.lightingAttached() {
		return lightingResult(d.setSchedulerDark(value == 0))
	}
	if value == 0 {
		d.DeviceProfile.OriginalBrightness = *d.DeviceProfile.BrightnessSlider
		d.DeviceProfile.BrightnessSlider = &value
	} else {
		d.DeviceProfile.BrightnessSlider = &d.DeviceProfile.OriginalBrightness
	}

	d.saveDeviceProfile()
	if d.DeviceProfile.RGBProfile == "static" || d.DeviceProfile.RGBProfile == "mouse" {
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
	if d.lightingAttached() {
		return d.saveCanonicalUserProfile(profileName)
	}
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.lightingMu.Lock()
	if d.DeviceProfile == nil || profileName == "" || !common.AlphanumericDashRegex.MatchString(profileName) {
		d.lightingMu.Unlock()
		return 0
	}
	copy := cloneLightingProfile(*d.DeviceProfile)
	copy.ProfileName, copy.Active = profileName, false
	copy.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-"+profileName+".json")
	if existing := d.UserProfiles[profileName]; existing != nil {
		copy.Path = existing.Path
		copy.Active = existing == d.DeviceProfile
	} else {
		path, err := d.availableProfilePathLocked(copy.Path)
		if err != nil {
			d.lightingMu.Unlock()
			return 0
		}
		copy.Path = path
	}
	copy.KeyAssignmentHash = common.GenerateRandomMD5()
	assignments := make(map[int]inputmanager.KeyAssignment, len(d.KeyAssignment))
	for key, value := range d.KeyAssignment {
		assignments[key] = value
	}
	d.lightingMu.Unlock()
	assignmentPath := filepath.Join(pwd, "database", "key-assignments", copy.KeyAssignmentHash+".json")
	if err := common.SaveJsonData(assignmentPath, assignments); err != nil {
		return 0
	}
	if err := common.SaveJsonData(copy.Path, &copy); err != nil {
		return 0
	}
	d.loadDeviceProfiles()
	return 1
}

// SaveMouseDPI will save mouse DPI
func (d *Device) SaveMouseDPI(stages map[int]uint16) uint8 {
	if d.lightingAttached() {
		return d.saveCanonicalDPI(stages, nil)
	}
	if !d.Connected {
		return 0
	}

	i := 0
	if d.DeviceProfile == nil {
		return 0
	}

	if len(stages) == 0 {
		return 0
	}

	for key, stage := range stages {
		if _, ok := d.DeviceProfile.Profiles[key]; ok {
			profile := d.DeviceProfile.Profiles[key]
			if stage > uint16(maxDpiValue) {
				continue
			}
			if stage < uint16(minDpiValue) {
				continue
			}
			profile.Value = stage
			d.DeviceProfile.Profiles[key] = profile
			i++
		}
	}

	if i > 0 {
		d.saveDeviceProfile()
		d.toggleDPI()
		return 1
	}
	return 0
}

// SaveMouseGestures will save mouse gestures
func (d *Device) SaveMouseGestures(multiGesture int, zoneTilts map[int]uint8) uint8 {
	if d.lightingAttached() {
		result := d.mutateInputProfile(false, func(p *DeviceProfile) error {
			if multiGesture < 0 || multiGesture > 1 || p.Tilts == nil || len(zoneTilts) < 4 {
				return fmt.Errorf("invalid gestures")
			}
			p.MultiGesture = multiGesture
			for id, value := range zoneTilts {
				if value < 10 || value > 80 {
					value = 80
				}
				if tilt, ok := p.Tilts[id]; ok {
					tilt.Value = value
					p.Tilts[id] = tilt
				}
			}
			return nil
		})
		if result == 1 {
			d.setGestures()
		}
		return result
	}
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()

	if !d.Connected {
		return 0
	}

	if d.DeviceProfile == nil {
		return 0
	}

	if d.DeviceProfile.Tilts == nil {
		return 0
	}

	if multiGesture < 0 || multiGesture > 1 {
		return 0
	}

	if len(zoneTilts) == 0 || len(zoneTilts) < 4 {
		return 0
	}

	d.DeviceProfile.MultiGesture = multiGesture
	for key, val := range zoneTilts {
		if val < 10 || val > 80 {
			val = 80
		}

		if tilt, ok := d.DeviceProfile.Tilts[key]; ok {
			tilt.Value = val
			d.DeviceProfile.Tilts[key] = tilt
		}
	}
	d.saveDeviceProfile()
	d.setGestures()
	return 1
}

// SaveMouseZoneColors will save mouse zone colors
func (d *Device) SaveMouseZoneColors(dpi rgb.Color, zoneColors map[int]rgb.Color) uint8 {
	if d.lightingAttached() {
		return d.saveCanonicalMouseColors(zoneColors)
	}
	if !d.Connected {
		return 0
	}

	i := 0
	if d.DeviceProfile == nil {
		return 0
	}
	if dpi.Red > 255 ||
		dpi.Green > 255 ||
		dpi.Blue > 255 ||
		dpi.Red < 0 ||
		dpi.Green < 0 ||
		dpi.Blue < 0 {
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

// SaveMouseDpiColors will save mouse dpi colors
func (d *Device) SaveMouseDpiColors(dpi rgb.Color, dpiColors map[int]rgb.Color) uint8 {
	if d.lightingAttached() {
		return d.saveCanonicalDPIColors(dpi, dpiColors)
	}
	i := 0
	if d.DeviceProfile == nil {
		return 0
	}
	if dpi.Red > 255 ||
		dpi.Green > 255 ||
		dpi.Blue > 255 ||
		dpi.Red < 0 ||
		dpi.Green < 0 ||
		dpi.Blue < 0 {
		return 0
	}

	// Zone Colors
	for key, zone := range dpiColors {
		if zone.Red > 255 ||
			zone.Green > 255 ||
			zone.Blue > 255 ||
			zone.Red < 0 ||
			zone.Green < 0 ||
			zone.Blue < 0 {
			continue
		}
		if profileColor, ok := d.DeviceProfile.Profiles[key]; ok {
			profileColor.Color.Red = zone.Red
			profileColor.Color.Green = zone.Green
			profileColor.Color.Blue = zone.Blue
			profileColor.Color.Hex = fmt.Sprintf("#%02x%02x%02x", int(zone.Red), int(zone.Green), int(zone.Blue))
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
	_, err := d.transfer(cmdHardwareMode, nil, "setHardwareMode")
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to change device mode")
	}
}

// setSoftwareMode will switch a device to software mode
func (d *Device) setSoftwareMode() {
	_, err := d.transfer(cmdSoftwareMode, nil, "setSoftwareMode")
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to change device mode")
	}
}

// SetSleepMode will switch a device to sleep mode
func (d *Device) SetSleepMode() {
	if d.lightingAttached() {
		d.sleepLighting()
		return
	}
	if d.Connected {
		// Retire the renderer before taking the lock its current frame may need.
		d.SetConnected(false)
		d.deviceLock.Lock()
		defer d.deviceLock.Unlock()
		_, err := d.transfer(cmdSleepMode, nil, "SetSleepMode")
		if err != nil {
			logger.Log(logger.Fields{"error": err}).Error("Unable to change device mode")
		}
	}
}

// getBatterLevel will return initial battery level
func (d *Device) getBatterLevel() {
	batteryLevel, err := d.transfer(cmdBatteryLevel, nil, "getBatterLevel")
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to get battery level")
	}
	d.BatteryLevel = binary.LittleEndian.Uint16(batteryLevel[3:5]) / 10
	stats.UpdateBatteryStats(d.Serial, d.Product, d.BatteryLevel, 1)
}

// GetSleepMode will return current sleep mode
func (d *Device) GetSleepMode() int {
	if d.lightingAttached() {
		d.lightingMu.Lock()
		defer d.lightingMu.Unlock()
	}
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
		"getDeviceFirmware",
	)
	if err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to get device firmware")
	}

	if fw[1] != 0x02 {
		fw, err = d.transfer(
			cmdGetFirmware,
			nil,
			"getDeviceFirmware",
		)
		if err != nil {
			logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to get device firmware")
		}
	}

	v1, v2, v3 := int(fw[3]), int(fw[4]), int(binary.LittleEndian.Uint16(fw[5:7]))
	d.Firmware = fmt.Sprintf("%d.%d.%d", v1, v2, v3)
}

// setAngleSnapping will change Angle Snapping mode
func (d *Device) setAngleSnapping() {
	if d.lightingAttached() {
		d.programLightingPreference("angle")
		return
	}
	if d.DeviceProfile == nil {
		return
	}

	if d.DeviceProfile.AngleSnapping < 0 || d.DeviceProfile.AngleSnapping > 1 {
		return
	}

	buf := make([]byte, 1)
	buf[0] = byte(d.DeviceProfile.AngleSnapping)
	_, _ = d.transfer(cmdAngleSnapping, buf, "setAngleSnapping")
}

// setButtonOptimization will change Button Response Optimization mode
func (d *Device) setButtonOptimization() {
	if d.lightingAttached() {
		d.programLightingPreference("button")
		return
	}
	if d.DeviceProfile == nil {
		return
	}

	if d.DeviceProfile.ButtonOptimization < 0 || d.DeviceProfile.ButtonOptimization > 1 {
		return
	}

	buf := make([]byte, 1)
	buf[0] = byte(d.DeviceProfile.ButtonOptimization)
	_, _ = d.transfer(cmdButtonOptimization, buf, "setAngleSnapping")
}

// setLiftHeight will change mouse lift height
func (d *Device) setLiftHeight() {
	if d.lightingAttached() {
		d.programLightingPreference("lift")
		return
	}
	if d.DeviceProfile == nil {
		return
	}

	if d.DeviceProfile.LiftHeight < 1 || d.DeviceProfile.LiftHeight > 7 {
		return
	}

	buf := make([]byte, 1)
	buf[0] = byte(d.DeviceProfile.LiftHeight)
	_, _ = d.transfer(cmdLiftHeight, buf, "setLiftHeight")
}

// setLiftHeight will change mouse lift height
func (d *Device) setGestures() {
	if d.lightingAttached() {
		d.programLightingPreference("gestures")
		return
	}
	if d.DeviceProfile == nil {
		return
	}

	if d.DeviceProfile.MultiGesture < 0 || d.DeviceProfile.MultiGesture > 1 {
		return
	}

	buf := make([]byte, 1)
	buf[0] = byte(d.DeviceProfile.MultiGesture)
	_, _ = d.transfer(cmdMultiGesture, buf, "setGestures")

	var command []byte

	for i := 0; i < len(d.DeviceProfile.Tilts); i++ {
		switch i {
		case 0: // Left
			command = cmdLeftTilt
			break
		case 1: // Right
			command = cmdRightTilt
			break
		case 2: // Forward
			command = cmdForwardTilt
			break
		case 3: // Backward
			command = cmdBackwardTilt
			break
		}
		buf[0] = d.DeviceProfile.Tilts[i].Value
		_, _ = d.transfer(command, buf, "setGestures")
	}
}

// saveDeviceProfile will save device profile for persistent configuration
func (d *Device) saveDeviceProfile() {
	if d.lightingAttached() {
		return
	}
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
		deviceProfile.RGBProfile = "mouse"
		deviceProfile.Label = "Mouse"
		deviceProfile.Active = true
		deviceProfile.ZoneColors = map[int]ZoneColors{
			0: { // Logo
				ColorIndex: []int{0, 2, 4},
				Color: &rgb.Color{
					Red:        255,
					Green:      255,
					Blue:       0,
					Brightness: 1,
					Hex:        fmt.Sprintf("#%02x%02x%02x", 255, 255, 0),
				},
				Name: "Logo",
			},
		}
		deviceProfile.Profiles = map[int]DPIProfile{
			0: {
				Name:        "Stage 1",
				Value:       400,
				PackerIndex: 1,
				ColorIndex: map[int][]int{
					0: {1, 3, 5},
				},
				Color: &rgb.Color{
					Red:        255,
					Green:      0,
					Blue:       0,
					Brightness: 1,
					Hex:        fmt.Sprintf("#%02x%02x%02x", 255, 0, 0),
				},
			},
			1: {
				Name:        "Stage 2",
				Value:       800,
				PackerIndex: 2,
				ColorIndex: map[int][]int{
					0: {1, 3, 5},
				},
				Color: &rgb.Color{
					Red:        255,
					Green:      255,
					Blue:       255,
					Brightness: 1,
					Hex:        fmt.Sprintf("#%02x%02x%02x", 255, 255, 255),
				},
			},
			2: {
				Name:        "Stage 3",
				Value:       1200,
				PackerIndex: 3,
				ColorIndex: map[int][]int{
					0: {1, 3, 5},
				},
				Color: &rgb.Color{
					Red:        0,
					Green:      255,
					Blue:       0,
					Brightness: 1,
					Hex:        fmt.Sprintf("#%02x%02x%02x", 0, 255, 0),
				},
			},
			3: {
				Name:        "Stage 4",
				Value:       1600,
				PackerIndex: 4,
				ColorIndex: map[int][]int{
					0: {1, 3, 5},
				},
				Color: &rgb.Color{
					Red:        255,
					Green:      0,
					Blue:       255,
					Brightness: 1,
					Hex:        fmt.Sprintf("#%02x%02x%02x", 255, 0, 255),
				},
			},
			4: {
				Name:        "Stage 5",
				Value:       3000,
				PackerIndex: 5,
				ColorIndex: map[int][]int{
					0: {1, 3, 5},
				},
				Color: &rgb.Color{
					Red:        0,
					Green:      255,
					Blue:       255,
					Brightness: 1,
					Hex:        fmt.Sprintf("#%02x%02x%02x", 0, 255, 255),
				},
			},
			5: {
				Name:        "Sniper",
				Value:       200,
				PackerIndex: 6,
				ColorIndex: map[int][]int{
					0: {1, 3, 5},
				},
				Color: &rgb.Color{
					Red:        255,
					Green:      255,
					Blue:       0,
					Brightness: 1,
					Hex:        fmt.Sprintf("#%02x%02x%02x", 255, 255, 0),
				},
				Sniper: true,
			},
		}
		deviceProfile.Profile = 1
		deviceProfile.SleepMode = 15
		deviceProfile.PollingRate = 4
		deviceProfile.LiftHeight = 2
		deviceProfile.Tilts = map[int]TiltOption{
			0: {
				Name:  "Left Tilt",
				Value: 20,
			},
			1: {
				Name:  "Right Tilt",
				Value: 20,
			},
			2: {
				Name:  "Forward Tilt",
				Value: 20,
			},
			3: {
				Name:  "Backward Tilt",
				Value: 20,
			},
		}
	} else {
		if d.DeviceProfile.LiftHeight == 0 {
			deviceProfile.LiftHeight = 2
		} else {
			deviceProfile.LiftHeight = d.DeviceProfile.LiftHeight
		}

		if d.DeviceProfile.Tilts == nil {
			deviceProfile.Tilts = map[int]TiltOption{
				0: {
					Name:  "Left Tilt",
					Value: 20,
				},
				1: {
					Name:  "Right Tilt",
					Value: 20,
				},
				2: {
					Name:  "Forward Tilt",
					Value: 20,
				},
				3: {
					Name:  "Backward Tilt",
					Value: 20,
				},
			}
		} else {
			deviceProfile.Tilts = d.DeviceProfile.Tilts
		}

		// Upgrade DPI profile
		d.upgradeDpiProfiles()

		if d.DeviceProfile.BrightnessSlider == nil {
			deviceProfile.BrightnessSlider = &defaultBrightness
			d.DeviceProfile.BrightnessSlider = &defaultBrightness
		} else {
			deviceProfile.BrightnessSlider = d.DeviceProfile.BrightnessSlider
		}

		if d.DeviceProfile.PollingRate == 0 {
			deviceProfile.PollingRate = 4
		} else {
			deviceProfile.PollingRate = d.DeviceProfile.PollingRate
		}

		deviceProfile.ProfileName = d.DeviceProfile.ProfileName
		deviceProfile.Active = d.DeviceProfile.Active
		deviceProfile.Brightness = d.DeviceProfile.Brightness
		deviceProfile.OriginalBrightness = d.DeviceProfile.OriginalBrightness
		deviceProfile.RGBProfile = d.DeviceProfile.RGBProfile
		deviceProfile.Label = d.DeviceProfile.Label
		deviceProfile.Profiles = d.DeviceProfile.Profiles
		deviceProfile.Profile = d.DeviceProfile.Profile
		deviceProfile.ZoneColors = d.DeviceProfile.ZoneColors
		deviceProfile.SleepMode = d.DeviceProfile.SleepMode
		deviceProfile.AngleSnapping = d.DeviceProfile.AngleSnapping
		deviceProfile.ButtonOptimization = d.DeviceProfile.ButtonOptimization
		deviceProfile.KeyAssignmentHash = d.DeviceProfile.KeyAssignmentHash
		if len(d.DeviceProfile.Path) < 1 {
			deviceProfile.Path = profilePath
			d.DeviceProfile.Path = profilePath
		} else {
			deviceProfile.Path = d.DeviceProfile.Path
		}
		deviceProfile.OpenRGBIntegration = d.DeviceProfile.OpenRGBIntegration
		deviceProfile.RGBCluster = d.DeviceProfile.RGBCluster
		deviceProfile.MultiGesture = d.DeviceProfile.MultiGesture
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

// upgradeDpiProfiles will perform upgrade of DPI profiles in needed
func (d *Device) upgradeDpiProfiles() {
	found := false
	for _, profile := range d.DeviceProfile.Profiles {
		if strings.EqualFold(profile.Name, "Sniper") {
			found = true
			break
		}
	}

	if !found {
		index := len(d.DeviceProfile.Profiles)
		sniper := DPIProfile{
			Name:        "Sniper",
			Value:       200,
			PackerIndex: 6,
			ColorIndex: map[int][]int{
				0: {1, 3, 5},
			},
			Color: &rgb.Color{
				Red:        255,
				Green:      255,
				Blue:       0,
				Brightness: 1,
				Hex:        fmt.Sprintf("#%02x%02x%02x", 255, 255, 0),
			},
			Sniper: true,
		}
		d.DeviceProfile.Profiles[index] = sniper
	}
}

// UpdateSleepTimer will update device sleep timer
func (d *Device) UpdateSleepTimer(minutes int) uint8 {
	if d.lightingAttached() {
		result := d.mutateInputProfile(false, func(p *DeviceProfile) error {
			if _, ok := d.SleepModes[minutes]; !ok {
				return fmt.Errorf("unsupported sleep timer")
			}
			p.SleepMode = minutes
			return nil
		})
		if result == 1 {
			d.setSleepTimer()
		}
		return result
	}
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
	if d.lightingAttached() {
		return d.programCanonicalSleep()
	}
	if d.Exit {
		return 0
	}
	if d.DeviceProfile != nil {
		changed := 0
		_, err := d.transfer(cmdOpenSleepWriteEndpoint, nil, "setSleepTimer")
		if err != nil {
			logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Warn("Unable to change device sleep timer")
			return 0
		}

		buf := make([]byte, 4)
		for i := 0; i < 2; i++ {
			command := cmdSleep[i]
			if i == 0 {
				buf[0] = 0xe0
				buf[1] = 0x93
				buf[2] = 0x04
				buf[3] = 0x00
			} else {
				sleep := d.DeviceProfile.SleepMode * (60 * 1000)
				binary.LittleEndian.PutUint32(buf, uint32(sleep))
			}
			_, err = d.transfer(command, buf, "setSleepTimer")
			if err != nil {
				logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Warn("Unable to change device sleep timer")
				continue
			}
			changed++
		}

		if changed > 0 {
			return 1
		}
	}
	return 0
}

// UpdateDeviceKeyAssignment will update device key assignments
func (d *Device) UpdateDeviceKeyAssignment(keyIndex int, keyAssignment inputmanager.KeyAssignment) uint8 {
	if d.lightingAttached() {
		return d.updateCanonicalKeyAssignment(keyIndex, keyAssignment)
	}
	if val, ok := d.KeyAssignment[keyIndex]; ok {
		if !d.Connected {
			return 0
		}
		val.Default = keyAssignment.Default
		val.ActionHold = keyAssignment.ActionHold
		val.ActionType = keyAssignment.ActionType
		val.ActionCommand = keyAssignment.ActionCommand
		val.IsMacro = keyAssignment.IsMacro
		val.OnRelease = keyAssignment.OnRelease
		d.KeyAssignment[keyIndex] = val
		d.saveKeyAssignments()
		d.setupKeyAssignment()
		return 1
	}
	return 0
}

// saveKeyAssignments will save new key assignments
func (d *Device) saveKeyAssignments() {
	keyAssignmentsFile := pwd + d.keyAssignmentFile
	if len(d.DeviceProfile.KeyAssignmentHash) > 0 {
		fileFormat := fmt.Sprintf("/database/key-assignments/%s.json", d.DeviceProfile.KeyAssignmentHash)
		keyAssignmentsFile = pwd + fileFormat
	}

	if err := common.SaveJsonData(keyAssignmentsFile, d.KeyAssignment); err != nil {
		logger.Log(logger.Fields{"error": err, "location": keyAssignmentsFile}).Error("Unable to write key assignment data")
		return
	}
}

// loadKeyAssignments will load custom key assignments
func (d *Device) loadKeyAssignments() {
	if d.lightingAttached() {
		if err := d.loadCanonicalAssignments(); err != nil {
			logger.Log(logger.Fields{"error": err}).Error("Unable to load input assignments")
		}
		return
	}
	if d.DeviceProfile == nil {
		return
	}
	keyAssignmentsFile := pwd + d.keyAssignmentFile
	if len(d.DeviceProfile.KeyAssignmentHash) > 0 {
		fileFormat := fmt.Sprintf("/database/key-assignments/%s.json", d.DeviceProfile.KeyAssignmentHash)
		keyAssignmentsFile = pwd + fileFormat
	}

	if common.FileExists(keyAssignmentsFile) {
		file, err := os.Open(keyAssignmentsFile)
		if err != nil {
			logger.Log(logger.Fields{"error": err, "serial": d.Serial, "location": keyAssignmentsFile}).Warn("Unable to load JSON file")
			return
		}

		if err = json.NewDecoder(file).Decode(&d.KeyAssignment); err != nil {
			logger.Log(logger.Fields{"error": err, "serial": d.Serial, "location": keyAssignmentsFile}).Warn("Unable to decode key assignments JSON")
			return
		}

		// Prevent left click modifications
		if !d.KeyAssignment[1].Default {
			logger.Log(logger.Fields{"serial": d.Serial, "value": d.KeyAssignment[1].Default, "expectedValue": 1}).Warn("Restoring left button to original value")
			var val = d.KeyAssignment[1]
			val.Default = true
			d.KeyAssignment[1] = val
		}

		err = file.Close()
		if err != nil {
			logger.Log(logger.Fields{"location": keyAssignmentsFile, "serial": d.Serial}).Warn("Failed to close file handle")
		}

		// One time upgrade of Tilt actions
		if len(d.KeyAssignment) < 19 {
			if len(d.KeyAssignment) < 19 {
				newAssignments := map[int]inputmanager.KeyAssignment{
					2048: {
						Name:          "Tilt Right",
						Default:       true,
						ActionType:    0,
						ActionCommand: 0,
						ActionHold:    false,
						IsTilt:        true,
						TiltIndex:     1,
					},
					1024: {
						Name:          "Tilt Left",
						Default:       true,
						ActionType:    0,
						ActionCommand: 0,
						ActionHold:    false,
						IsTilt:        true,
						TiltIndex:     0,
					},
					512: {
						Name:          "Tilt Back",
						Default:       true,
						ActionType:    0,
						ActionCommand: 0,
						ActionHold:    false,
						IsTilt:        true,
						TiltIndex:     2,
					},
					256: {
						Name:          "Tilt Front",
						Default:       true,
						ActionType:    0,
						ActionCommand: 0,
						ActionHold:    false,
						IsTilt:        true,
						TiltIndex:     3,
					},
				}
				for key, assignment := range newAssignments {
					if _, exists := d.KeyAssignment[key]; !exists {
						d.KeyAssignment[key] = assignment
					}
				}
			}
			d.saveKeyAssignments()
		}
	} else {
		var keyAssignment = map[int]inputmanager.KeyAssignment{
			2048: {
				Name:          "Tilt Right",
				Default:       true,
				ActionType:    0,
				ActionCommand: 0,
				ActionHold:    false,
				IsTilt:        true,
				TiltIndex:     1,
			},
			1024: {
				Name:          "Tilt Left",
				Default:       true,
				ActionType:    0,
				ActionCommand: 0,
				ActionHold:    false,
				IsTilt:        true,
				TiltIndex:     0,
			},
			512: {
				Name:          "Tilt Back",
				Default:       true,
				ActionType:    0,
				ActionCommand: 0,
				ActionHold:    false,
				IsTilt:        true,
				TiltIndex:     2,
			},
			256: {
				Name:          "Tilt Front",
				Default:       true,
				ActionType:    0,
				ActionCommand: 0,
				ActionHold:    false,
				IsTilt:        true,
				TiltIndex:     3,
			},
			128: {
				Name:          "Sniper",
				Default:       false,
				ActionType:    8,
				ActionCommand: 0,
				ActionHold:    true,
			},
			64: {
				Name:          "DPI -",
				Default:       true,
				ActionType:    0,
				ActionCommand: 0,
				ActionHold:    false,
			},
			32: {
				Name:          "DPI +",
				Default:       true,
				ActionType:    0,
				ActionCommand: 0,
				ActionHold:    false,
			},
			8: {
				Name:          "Forward Button",
				Default:       true,
				ActionType:    0,
				ActionCommand: 0,
				ActionHold:    false,
			},
			16: {
				Name:          "Back Button",
				Default:       true,
				ActionType:    0,
				ActionCommand: 0,
				ActionHold:    false,
			},
			4: {
				Name:          "Middle Button",
				Default:       true,
				ActionType:    0,
				ActionCommand: 0,
				ActionHold:    false,
			},
			2: {
				Name:          "Right Button",
				Default:       true,
				ActionType:    0,
				ActionCommand: 0,
				ActionHold:    false,
			},
			1: {
				Name:          "Left Button",
				Default:       true,
				ActionType:    0,
				ActionCommand: 0,
				ActionHold:    false,
			},
		}

		if err := common.SaveJsonData(keyAssignmentsFile, keyAssignment); err != nil {
			logger.Log(logger.Fields{"error": err, "location": keyAssignmentsFile}).Error("Unable to save key assignments data")
			return
		}
		d.KeyAssignment = keyAssignment
	}
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

		if fileName != d.Serial && !strings.HasPrefix(fileName, d.Serial+"-") {
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

		// Runtime location is the file actually read, even after a data-root move.
		pf.Path = profileLocation
		if pf.Serial == d.Serial {
			if fileName == d.Serial {
				profileList["default"] = pf
			} else {
				name := strings.TrimPrefix(fileName, d.Serial+"-")
				if pf.ProfileName != "" && common.AlphanumericDashRegex.MatchString(pf.ProfileName) {
					name = pf.ProfileName
				}
				profileList[name] = pf
			}
			logger.Log(logger.Fields{"location": profileLocation, "serial": d.Serial}).Info("Loaded custom user profile")
		}
	}
	d.lightingMu.Lock()
	d.UserProfiles = profileList
	d.rebuildProfileOrder()
	d.getDeviceProfile()
	d.lightingMu.Unlock()
}

// rebuildProfileOrder will return profile order
func (d *Device) rebuildProfileOrder() {
	d.ProfileOrder = d.ProfileOrder[:0]
	for name := range d.UserProfiles {
		d.ProfileOrder = append(d.ProfileOrder, name)
	}
	sort.Strings(d.ProfileOrder)
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

// initLeds will initialize LED endpoint
func (d *Device) initLeds() {
	_, err := d.transfer(cmdOpenEndpoint, nil, "initLeds")
	if err != nil {
		logger.Log(logger.Fields{"error": err}).Error("Unable to change device mode")
	}
	time.Sleep(100 * time.Millisecond)
}

// getSniperColor will get sniper dpi color
func (d *Device) getSniperColor() *rgb.Color {
	for _, val := range d.DeviceProfile.Profiles {
		if val.Sniper {
			return val.Color
		}
	}
	return nil
}

// ControlDeviceRgb will change device brightness via schedulerSchedulerBrightness
func (d *Device) ControlDeviceRgb(value bool) {
	if d.lightingAttached() {
		if err := d.setUserRGBOff(value); err != nil {
			logger.Log(logger.Fields{"error": err}).Error("Unable to persist RGB-off preference")
		}
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
func (d *Device) setLegacyDeviceColor() {
	buf := make([]byte, d.LEDChannels*3)
	if d.DeviceProfile == nil {
		logger.Log(logger.Fields{"serial": d.Serial}).Error("Unable to set color. DeviceProfile is null!")
		return
	}

	// OpenRGB
	if d.DeviceProfile.OpenRGBIntegration || d.DeviceProfile.RgbOff {
		// DPI
		dpiColor := d.DeviceProfile.Profiles[d.DeviceProfile.Profile].Color
		if d.SniperMode {
			dpiColor = d.getSniperColor()
		}
		if dpiColor == nil {
			return
		}

		dpiColor.Brightness = rgb.GetBrightnessValueFloat(*d.DeviceProfile.BrightnessSlider)
		dpiColor = rgb.ModifyBrightness(*dpiColor)

		dpiLeds := d.DeviceProfile.Profiles[d.DeviceProfile.Profile]
		for i := 0; i < len(dpiLeds.ColorIndex); i++ {
			dpiColorIndexRange := dpiLeds.ColorIndex[i]
			for key, dpiColorIndex := range dpiColorIndexRange {
				switch key {
				case 0: // Red
					buf[dpiColorIndex] = byte(dpiColor.Red)
				case 1: // Green
					buf[dpiColorIndex] = byte(dpiColor.Green)
				case 2: // Blue
					buf[dpiColorIndex] = byte(dpiColor.Blue)
				}
			}
		}

		zoneKeys := make([]int, 0, len(d.DeviceProfile.ZoneColors))
		for key := range d.DeviceProfile.ZoneColors {
			zoneKeys = append(zoneKeys, key)
		}
		sort.Ints(zoneKeys)

		m := 0
		for _, key := range zoneKeys {
			zoneColor := d.DeviceProfile.ZoneColors[key]
			for _, zoneColorIndex := range zoneColor.ColorIndex {
				buf[zoneColorIndex] = 0x00
				m++
			}
		}
		d.writeColor(buf)
		logger.Log(logger.Fields{}).Info("Exiting setDeviceColor() due to OpenRGB client")
		return
	}

	// RGB Cluster
	if d.DeviceProfile.RGBCluster {
		logger.Log(logger.Fields{}).Info("Exiting setDeviceColor() due to RGB Cluster")
		return
	}

	// Reset
	for i := 0; i < d.LEDChannels*3; i++ {
		buf[i] = 0x00
	}
	d.writeColor(buf)

	// DPI
	dpiColor := d.DeviceProfile.Profiles[d.DeviceProfile.Profile].Color
	if d.SniperMode {
		dpiColor = d.getSniperColor()
	}
	if dpiColor == nil {
		return
	}

	dpiColor.Brightness = rgb.GetBrightnessValueFloat(*d.DeviceProfile.BrightnessSlider)
	dpiColor = rgb.ModifyBrightness(*dpiColor)

	dpiLeds := d.DeviceProfile.Profiles[d.DeviceProfile.Profile]
	for i := 0; i < len(dpiLeds.ColorIndex); i++ {
		dpiColorIndexRange := dpiLeds.ColorIndex[i]
		for key, dpiColorIndex := range dpiColorIndexRange {
			switch key {
			case 0: // Red
				buf[dpiColorIndex] = byte(dpiColor.Red)
			case 1: // Green
				buf[dpiColorIndex] = byte(dpiColor.Green)
			case 2: // Blue
				buf[dpiColorIndex] = byte(dpiColor.Blue)
			}
		}
	}

	if d.DeviceProfile.RGBProfile == "mouse" {
		for _, zoneColor := range d.DeviceProfile.ZoneColors {
			zoneColor.Color.Brightness = rgb.GetBrightnessValueFloat(*d.DeviceProfile.BrightnessSlider)
			zoneColor.Color = rgb.ModifyBrightness(*zoneColor.Color)
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

	if d.DeviceProfile.RGBProfile == "static" {
		profile := d.GetRgbProfile("static")
		if profile == nil {
			return
		}

		profile.StartColor.Brightness = rgb.GetBrightnessValueFloat(*d.DeviceProfile.BrightnessSlider)
		profileColor := rgb.ModifyBrightness(profile.StartColor)
		for _, zoneColor := range d.DeviceProfile.ZoneColors {
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

	go func(lightChannels int) {
		startTime := time.Now()
		d.activeRgb = rgb.Exit()

		// Generate random colors
		d.activeRgb.RGBStartColor = rgb.GenerateRandomColor(1)
		d.activeRgb.RGBEndColor = rgb.GenerateRandomColor(1)

		for {
			select {
			case <-d.activeRgb.Exit:
				return
			default:
				buff := make([]byte, 0)
				rgbCustomColor := true
				profile := d.GetRgbProfile(d.DeviceProfile.RGBProfile)
				if profile == nil {
					for i := 0; i < d.ChangeableLedChannels*3; i++ {
						buff = append(buff, []byte{0, 0, 0}...)
					}
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
					r.RGBStartColor = d.activeRgb.RGBStartColor
					r.RGBEndColor = d.activeRgb.RGBEndColor
					r.RGBMiddleColor = d.activeRgb.RGBMiddleColor
				}

				if r.RGBMiddleColor == nil {
					r.RGBMiddleColor = &rgb.Color{}
				}

				// Brightness
				r.RGBBrightness = rgb.GetBrightnessValueFloat(*d.DeviceProfile.BrightnessSlider)
				r.RGBStartColor.Brightness = r.RGBBrightness
				r.RGBEndColor.Brightness = r.RGBBrightness
				r.RGBMiddleColor.Brightness = r.RGBBrightness

				switch d.DeviceProfile.RGBProfile {
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
						r.Colorshift(&startTime, d.activeRgb)
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
						r.Colorwarp(&startTime, d.activeRgb)
						buff = append(buff, r.Output...)
					}
				}
				zoneKeys := make([]int, 0, len(d.DeviceProfile.ZoneColors))
				for key := range d.DeviceProfile.ZoneColors {
					zoneKeys = append(zoneKeys, key)
				}
				sort.Ints(zoneKeys)

				m := 0
				for _, key := range zoneKeys {
					zoneColor := d.DeviceProfile.ZoneColors[key]
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

func (d *Device) ModifyDpi(increment bool) {
	if d.lightingAttached() {
		d.modifyCanonicalDPI(increment)
		return
	}
	if increment {
		if d.DeviceProfile.Profile >= 4 {
			return
		}
		d.DeviceProfile.Profile++
	} else {
		if d.DeviceProfile.Profile <= 0 {
			return
		}
		d.DeviceProfile.Profile--
	}
	d.saveDeviceProfile()
	d.toggleDPI()
}

// setupKeyAssignment will setup mouse keys
func (d *Device) setupKeyAssignment() {
	if d.lightingAttached() {
		if err := d.programCanonicalAssignments(); err != nil {
			logger.Log(logger.Fields{"error": err}).Warn("Unable to program mouse assignments")
		}
		return
	}
	// Prevent modifications if key amount does not match the expected key amount
	definedKeyAmount := len(d.KeyAssignment)
	if definedKeyAmount < keyAmount || definedKeyAmount > keyAmount {
		logger.Log(logger.Fields{"vendorId": d.VendorId, "keys": definedKeyAmount, "expected": keyAmount}).Warn("Expected key amount does not match the expected key amount.")
		return
	}

	tilt := 0
	keys := make([]int, 0)
	for k, v := range d.KeyAssignment {
		if v.IsTilt {
			tilt++
		}
		keys = append(keys, k)
	}
	sort.Ints(keys)

	buf := make([]byte, keyAmount-tilt)
	i := 0
	for _, k := range keys {
		value := d.KeyAssignment[k]
		if value.IsTilt {
			continue
		}

		if value.Default {
			buf[i] = byte(1)
		} else {
			buf[i] = byte(0)
		}
		i++
	}
	d.writeKeyAssignmentData(buf)

	// Tilt action
	for _, k := range keys {
		value := d.KeyAssignment[k]
		if value.IsTilt {
			buffer := make([]byte, 1)
			var command []byte
			switch value.TiltIndex {
			case 0:
				command = cmdActivateLeftTilt
				break
			case 1:
				command = cmdActivateRightTilt
				break
			case 2:
				command = cmdActivateFrontTilt
				break
			case 3:
				command = cmdActivateBackTilt
				break
			}

			if !value.Default {
				buffer[0] = byte(1)
			} else {
				buffer[0] = byte(0)
			}
			_, _ = d.transfer(command, buffer, "setupKeyAssignment")
		}
	}
}

// addToMacroTracker adds or updates an entry in MacroTracker
func (d *Device) addToMacroTracker(key int, value uint16) {
	d.macroMutex.Lock()
	defer d.macroMutex.Unlock()

	if d.MacroTracker == nil {
		d.MacroTracker = make(map[int]uint16)
	}
	d.MacroTracker[key] = value
}

// deleteFromMacroTracker deletes an entry from MacroTracker
func (d *Device) deleteFromMacroTracker(key int) {
	d.macroMutex.Lock()
	defer d.macroMutex.Unlock()

	if d.MacroTracker == nil || len(d.MacroTracker) == 0 {
		return
	}
	delete(d.MacroTracker, key)
}

// releaseMacroTracker will release current MacroTracker
func (d *Device) releaseMacroTracker() {
	d.macroMutex.Lock()
	keys := d.MacroTracker
	d.MacroTracker = nil
	d.macroMutex.Unlock()
	for _, command := range keys {
		inputmanager.InputControlKeyboardHold(command, false)
	}
}

// TriggerKeyAssignment will trigger key assignment if defined
func (d *Device) TriggerKeyAssignment(value uint32) {
	d.inputEventMu.Lock()
	defer d.inputEventMu.Unlock()
	var bitDiff = value ^ d.ModifierIndex
	var pressedKeys = bitDiff & value
	var releasedKeys = bitDiff & ^value
	d.ModifierIndex = value

	for keys := pressedKeys | releasedKeys; keys != 0; {
		bitIdx := bits.TrailingZeros32(keys)
		mask := uint32(1) << bitIdx
		keys &^= mask

		isPressed := pressedKeys&mask != 0
		isReleased := releasedKeys&mask != 0

		val, ok := d.inputAssignment(int(mask))
		if !ok {
			continue
		}

		if val.OnRelease {
			isPressed, isReleased = isReleased, isPressed
		}

		if isReleased {
			// Check if we have any queue in macro tracker. If yes, release those keys
			d.releaseMacroTracker()

			if val.Default || !val.ActionHold {
				continue
			}
			switch val.ActionType {
			case 1, 3:
				inputmanager.InputControlKeyboardHold(val.ActionCommand, false)
			case 8:
				d.sniperMode(false)
			case 9:
				inputmanager.InputControlMouseHold(val.ActionCommand, false)
			}
		}

		if isPressed {
			if mask == 0x20 && val.Default {
				d.ModifyDpi(true)
				return
			}

			if mask == 0x40 && val.Default {
				d.ModifyDpi(false)
				return
			}

			if val.Default {
				return
			}

			switch val.ActionType {
			case 1, 3:
				if val.ActionHold {
					inputmanager.InputControlKeyboardHold(val.ActionCommand, true)
				} else {
					inputmanager.InputControlKeyboard(val.ActionCommand, false)
				}
				break
			case 2:
				d.ModifyDpi(true)
				break
			case 4:
				d.ModifyDpi(false)
				break
			case 8:
				d.sniperMode(true)
				break
			case 9:
				if val.ActionHold {
					inputmanager.InputControlMouseHold(val.ActionCommand, true)
				} else {
					inputmanager.InputControlMouse(val.ActionCommand)
				}
				break
			case 10:
				d.runInputMacroUnlocked(val.ActionCommand, func() {
					macroProfile := macro.GetProfile(int(val.ActionCommand))
					if macroProfile == nil {
						logger.Log(logger.Fields{"serial": d.Serial}).Error("Invalid macro profile")
						return
					}
					for i := 0; i < len(macroProfile.Actions); i++ {
						if v, valid := macroProfile.Actions[i]; valid {
							// Add to macro tracker for easier release
							if v.ActionHold {
								d.addToMacroTracker(i, v.ActionCommand)
							}

							switch v.ActionType {
							case 1, 3:
								inputmanager.InputControlKeyboard(v.ActionCommand, v.ActionHold)
							case 9:
								inputmanager.InputControlMouse(v.ActionCommand)
							case 5:
								if v.ActionDelay > 0 {
									time.Sleep(time.Duration(v.ActionDelay) * time.Millisecond)
								}
							case 6:
								if v.ActionRepeat > 0 {
									for z := 0; z < int(v.ActionRepeat); z++ {
										inputmanager.InputControlKeyboardText(v.ActionText)
										if v.ActionRepeatDelay > 0 && v.ActionRepeat > 1 {
											time.Sleep(time.Duration(v.ActionRepeatDelay) * time.Millisecond)
										}
									}
								} else {
									inputmanager.InputControlKeyboardText(v.ActionText)
								}
							case 20:
								if v.ActionRepeat > 0 && !v.ActionHold {
									d.stopRepeatMutex.Lock()
									if d.stopRepeat != nil {
										close(d.stopRepeat)
									}

									d.stopRepeat = make(chan struct{})
									localStop := d.stopRepeat
									d.stopRepeatMutex.Unlock()

									go func() {
										for z := 0; z < int(v.ActionRepeat); z++ {
											select {
											case <-localStop:
												return
											default:
												if v.MousePositionAbsolute {
													inputmanager.InputControlMoveAbsolute(int32(v.MousePositionX), int32(v.MousePositionY))
												} else {
													inputmanager.InputControlMove(int32(v.MousePositionX), int32(v.MousePositionY))
												}
											}
											if v.ActionRepeatDelay > 0 && v.ActionRepeat > 1 {
												time.Sleep(time.Duration(v.ActionRepeatDelay) * time.Millisecond)
											}
										}
									}()
								} else {
									if v.MousePositionAbsolute {
										inputmanager.InputControlMoveAbsolute(int32(v.MousePositionX), int32(v.MousePositionY))
									} else {
										inputmanager.InputControlMove(int32(v.MousePositionX), int32(v.MousePositionY))
									}
								}
							}
						}
					}
				})
				break
			case 11:
				d.rotateDeviceProfile()
				break
			}
		}
	}
}

// TriggerTiltAssignment will trigger key assignment tilt if defined
func (d *Device) TriggerTiltAssignment(value uint32) {
	d.inputEventMu.Lock()
	defer d.inputEventMu.Unlock()
	val, held := d.heldTilts[int(value)]
	if !held {
		d.lightingMu.Lock()
		available := d.Connected && !d.Exit
		d.lightingMu.Unlock()
		if !available {
			return
		}
		var ok bool
		val, ok = d.inputAssignment(int(value))
		if !ok {
			return
		}
	}
	isReleased := held
	isPressed := !held

	if isReleased {
		// Check if we have any queue in macro tracker. If yes, release those keys
		d.releaseMacroTracker()

		if val.Default || !val.ActionHold {
			return
		}
		switch val.ActionType {
		case 1, 3:
			d.sendTiltHold(val.ActionType, val.ActionCommand, false)
		case 8:
			d.sniperMode(false)
		case 9:
			d.sendTiltHold(val.ActionType, val.ActionCommand, false)
		}

		delete(d.heldTilts, int(value))
		// Reset toggle state
		val.TiltToggle = false
		d.setInputTiltToggle(int(value), val.TiltToggle)
		return
	}

	if isPressed {
		// Set toggle state
		val.TiltToggle = val.ActionHold
		d.setInputTiltToggle(int(value), val.TiltToggle)

		if val.ProfileSwitch {
			d.rotateDeviceProfile()
			return
		}

		if val.Default {
			return
		}
		if val.ActionHold {
			if d.heldTilts == nil {
				d.heldTilts = make(map[int]inputmanager.KeyAssignment)
			}
			d.heldTilts[int(value)] = val
		}

		switch val.ActionType {
		case 1, 3:
			if val.ActionHold {
				d.sendTiltHold(val.ActionType, val.ActionCommand, true)
			} else {
				inputmanager.InputControlKeyboard(val.ActionCommand, false)
			}
			break
		case 2:
			d.ModifyDpi(true)
			break
		case 4:
			d.ModifyDpi(false)
			break
		case 8:
			d.sniperMode(true)
			break
		case 9:
			if val.ActionHold {
				d.sendTiltHold(val.ActionType, val.ActionCommand, true)
			} else {
				inputmanager.InputControlMouse(val.ActionCommand)
			}
		case 10:
			d.runInputMacroUnlocked(val.ActionCommand, func() {
				macroProfile := macro.GetProfile(int(val.ActionCommand))
				if macroProfile == nil {
					logger.Log(logger.Fields{"serial": d.Serial}).Error("Invalid macro profile")
					return
				}
				for i := 0; i < len(macroProfile.Actions); i++ {
					if v, valid := macroProfile.Actions[i]; valid {
						// Add to macro tracker for easier release
						if v.ActionHold {
							d.addToMacroTracker(i, v.ActionCommand)
						}

						switch v.ActionType {
						case 1, 3:
							inputmanager.InputControlKeyboard(v.ActionCommand, v.ActionHold)
						case 9:
							inputmanager.InputControlMouse(v.ActionCommand)
						case 5:
							if v.ActionDelay > 0 {
								time.Sleep(time.Duration(v.ActionDelay) * time.Millisecond)
							}
						case 6:
							if v.ActionRepeat > 0 {
								for z := 0; z < int(v.ActionRepeat); z++ {
									inputmanager.InputControlKeyboardText(v.ActionText)
									if v.ActionRepeatDelay > 0 && v.ActionRepeat > 1 {
										time.Sleep(time.Duration(v.ActionRepeatDelay) * time.Millisecond)
									}
								}
							} else {
								inputmanager.InputControlKeyboardText(v.ActionText)
							}
						case 20:
							if v.ActionRepeat > 0 && !v.ActionHold {
								d.stopRepeatMutex.Lock()
								if d.stopRepeat != nil {
									close(d.stopRepeat)
								}

								d.stopRepeat = make(chan struct{})
								localStop := d.stopRepeat
								d.stopRepeatMutex.Unlock()

								go func() {
									for z := 0; z < int(v.ActionRepeat); z++ {
										select {
										case <-localStop:
											return
										default:
											if v.MousePositionAbsolute {
												inputmanager.InputControlMoveAbsolute(int32(v.MousePositionX), int32(v.MousePositionY))
											} else {
												inputmanager.InputControlMove(int32(v.MousePositionX), int32(v.MousePositionY))
											}
										}
										if v.ActionRepeatDelay > 0 && v.ActionRepeat > 1 {
											time.Sleep(time.Duration(v.ActionRepeatDelay) * time.Millisecond)
										}
									}
								}()
							} else {
								if v.MousePositionAbsolute {
									inputmanager.InputControlMoveAbsolute(int32(v.MousePositionX), int32(v.MousePositionY))
								} else {
									inputmanager.InputControlMove(int32(v.MousePositionX), int32(v.MousePositionY))
								}
							}
						}
					}
				}
			})
			break
		case 11:
			d.rotateDeviceProfile()
			break

		}
	}
}

// CallSniperMode calls sniper mode from outside of this device
func (d *Device) CallSniperMode(active bool) {
	if d.lightingAttached() {
		d.lightingMu.Lock()
		connected := d.Connected
		d.lightingMu.Unlock()
		if connected {
			d.canonicalSniper(active)
		}
		return
	}
	if d.Connected {
		d.sniperMode(active)
	}
}

// sniperMode will set mouse DPI to sniper mode
func (d *Device) sniperMode(active bool) {
	if d.lightingAttached() {
		d.canonicalSniper(active)
		return
	}
	d.SniperMode = active
	if active {
		for _, profile := range d.DeviceProfile.Profiles {
			if profile.Sniper {
				value := profile.Value

				// Send DPI packet
				if value < uint16(minDpiValue) {
					value = uint16(minDpiValue)
				}
				if value > uint16(maxDpiValue) {
					value = uint16(maxDpiValue)
				}

				buf := make([]byte, 2)
				binary.LittleEndian.PutUint16(buf[0:2], value)
				for i := 0; i <= 1; i++ {
					_, err := d.transfer(cmdSetDpi[i], buf, "toggleDPI")
					if err != nil {
						logger.Log(logger.Fields{"error": err, "vendorId": d.VendorId}).Error("Unable to set dpi")
					}
				}

				if d.activeRgb != nil {
					d.activeRgb.Exit <- true
					d.activeRgb = nil
				}
				d.setDeviceColor()
			}
		}
	} else {
		// Reset to normal DPI mode
		d.toggleDPI()
	}
}

// toggleDPI will change DPI mode
func (d *Device) toggleDPI() {
	if d.lightingAttached() {
		_ = d.applyCanonicalDPIAndIndicator()
		return
	}
	if d.Exit {
		return
	}
	if d.DeviceProfile != nil {
		d.deviceLock.Lock()
		profile := d.DeviceProfile.Profiles[d.DeviceProfile.Profile]
		value := profile.Value

		// Send DPI packet
		if value < uint16(minDpiValue) {
			value = uint16(minDpiValue)
		}
		if value > uint16(maxDpiValue) {
			value = uint16(maxDpiValue)
		}

		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf[0:2], value)
		for i := 0; i <= 1; i++ {
			_, err := d.transfer(cmdSetDpi[i], buf, "toggleDPI")
			if err != nil {
				logger.Log(logger.Fields{"error": err, "vendorId": d.VendorId}).Error("Unable to set dpi")
			}
		}
		d.deviceLock.Unlock()

		if d.activeRgb != nil {
			d.activeRgb.Exit <- true
			d.activeRgb = nil
		}
		d.setDeviceColor()
	}
}

// writeColorEx will write data to the device from OpenRGB client
func (d *Device) writeColorEx(data []byte, _ int) {
	if d.lightingAttached() {
		d.enqueueCurrentOpenRGB(data)
		return
	}
	if d.DeviceProfile == nil || !d.DeviceProfile.OpenRGBIntegration {
		return
	}
	if d.Exit {
		return
	}

	// Copy data to avoid race conditions, since the caller might reuse the slice
	copyData := make([]byte, len(data))
	copy(copyData, data)

	select {
	case d.queue <- copyData:
	default:
	}
}

// clearQueue will clear queue
func (d *Device) clearQueue() {
	if d.lightingAttached() {
		d.lightingMu.Lock()
		d.invalidateExternalLocked()
		d.lightingMu.Unlock()
		return
	}
	for {
		select {
		case <-d.queue:
		default:
			return
		}
	}
}

// startQueueWorker will initialize queue system and control packet flow towards the device
func (d *Device) startQueueWorker() {
	if d.lightingAttached() {
		d.startLightingQueue()
		return
	}
	d.queue = make(chan []byte, 10)

	go func() {
		for data := range d.queue {
			d.deviceLock.Lock()
			buf := make([]byte, d.LEDChannels*3)
			dpiColor := d.DeviceProfile.Profiles[d.DeviceProfile.Profile].Color
			if d.SniperMode {
				dpiColor = d.getSniperColor()
			}
			if dpiColor == nil {
				d.deviceLock.Unlock()
				continue
			}

			dpiColor.Brightness = rgb.GetBrightnessValueFloat(*d.DeviceProfile.BrightnessSlider)
			dpiColor = rgb.ModifyBrightness(*dpiColor)

			dpiLeds := d.DeviceProfile.Profiles[d.DeviceProfile.Profile]
			for i := 0; i < len(dpiLeds.ColorIndex); i++ {
				dpiColorIndexRange := dpiLeds.ColorIndex[i]
				for key, dpiColorIndex := range dpiColorIndexRange {
					switch key {
					case 0: // Red
						buf[dpiColorIndex] = byte(dpiColor.Red)
					case 1: // Green
						buf[dpiColorIndex] = byte(dpiColor.Green)
					case 2: // Blue
						buf[dpiColorIndex] = byte(dpiColor.Blue)
					}
				}
			}

			zoneKeys := make([]int, 0, len(d.DeviceProfile.ZoneColors))
			for key := range d.DeviceProfile.ZoneColors {
				zoneKeys = append(zoneKeys, key)
			}
			sort.Ints(zoneKeys)

			m := 0
			for _, key := range zoneKeys {
				zoneColor := d.DeviceProfile.ZoneColors[key]
				for _, zoneColorIndex := range zoneColor.ColorIndex {
					if m >= len(data) {
						break
					}
					buf[zoneColorIndex] = data[m]
					m++
				}
			}

			buffer := make([]byte, len(buf)+headerWriteSize)
			binary.LittleEndian.PutUint16(buffer[0:2], uint16(len(buf)))
			copy(buffer[headerWriteSize:], buf)

			_, err := d.transfer(cmdWriteColor, buffer, "writeColor")
			if err != nil {
				logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to write to color endpoint")
			}
			d.deviceLock.Unlock()
			time.Sleep(20 * time.Millisecond)
		}
	}()
}

// writeColorCluster will write cluster color
func (d *Device) writeColorCluster(data []byte, _ int) {
	if d.lightingAttached() {
		d.applyCurrentCluster(data)
		return
	}
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()

	if d.DeviceProfile == nil || !d.DeviceProfile.RGBCluster {
		return
	}

	if d.Exit {
		return
	}

	buf := make([]byte, d.LEDChannels*3)
	if d.DeviceProfile == nil {
		logger.Log(logger.Fields{"serial": d.Serial}).Error("Unable to set color. DeviceProfile is null!")
		return
	}

	// DPI
	dpiColor := d.DeviceProfile.Profiles[d.DeviceProfile.Profile].Color
	if d.SniperMode {
		dpiColor = d.getSniperColor()
	}
	if dpiColor == nil {
		return
	}

	dpiColor.Brightness = rgb.GetBrightnessValueFloat(*d.DeviceProfile.BrightnessSlider)
	dpiColor = rgb.ModifyBrightness(*dpiColor)

	dpiLeds := d.DeviceProfile.Profiles[d.DeviceProfile.Profile]
	for i := 0; i < len(dpiLeds.ColorIndex); i++ {
		dpiColorIndexRange := dpiLeds.ColorIndex[i]
		for key, dpiColorIndex := range dpiColorIndexRange {
			switch key {
			case 0: // Red
				buf[dpiColorIndex] = byte(dpiColor.Red)
			case 1: // Green
				buf[dpiColorIndex] = byte(dpiColor.Green)
			case 2: // Blue
				buf[dpiColorIndex] = byte(dpiColor.Blue)
			}
		}
	}

	zoneKeys := make([]int, 0, len(d.DeviceProfile.ZoneColors))
	for key := range d.DeviceProfile.ZoneColors {
		zoneKeys = append(zoneKeys, key)
	}
	sort.Ints(zoneKeys)

	m := 0
	for _, key := range zoneKeys {
		zoneColor := d.DeviceProfile.ZoneColors[key]
		for _, zoneColorIndex := range zoneColor.ColorIndex {
			if m >= len(data) {
				break
			}
			buf[zoneColorIndex] = data[m]
			m++
		}
	}

	buffer := make([]byte, len(buf)+headerWriteSize)
	binary.LittleEndian.PutUint16(buffer[0:2], uint16(len(buf)))
	copy(buffer[headerWriteSize:], buf)

	_, err := d.transfer(cmdWriteColor, buffer, "writeColor")
	if err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to write to color endpoint")
	}
}

// writeColor will write color data to the device
func (d *Device) writeColor(data []byte) {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()

	if d.Exit {
		return
	}
	buffer := make([]byte, len(data)+headerWriteSize)
	binary.LittleEndian.PutUint16(buffer[0:2], uint16(len(data)))
	copy(buffer[headerWriteSize:], data)

	_, err := d.transfer(cmdWriteColor, buffer, "writeColor")
	if err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to write to color endpoint")
	}
}

// writeKeyAssignmentData will write key assignment to the device.
func (d *Device) writeKeyAssignmentData(data []byte) {
	d.deviceLock.Lock()
	defer d.deviceLock.Unlock()

	if d.Exit {
		return
	}

	_, err := d.transfer(cmdOpenWriteEndpoint, nil, "writeKeyAssignmentData")
	if err != nil {
		logger.Log(logger.Fields{"error": err, "vendorId": d.VendorId}).Error("Unable to open write endpoint")
		return
	}

	buffer := make([]byte, len(data)+headerWriteSize)
	binary.LittleEndian.PutUint16(buffer[0:2], uint16(len(data)))
	copy(buffer[headerWriteSize:], data)
	_, err = d.transfer(cmdWrite, buffer, "writeKeyAssignmentData")
	if err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to write to data endpoint")
	}

	_, err = d.transfer(cmdCloseEndpoint, nil, "writeKeyAssignmentData")
	if err != nil {
		logger.Log(logger.Fields{"error": err, "vendorId": d.VendorId}).Error("Unable to close endpoint")
		return
	}
}

// transfer will send data to a device and retrieve device output
func (d *Device) transfer(endpoint, buffer []byte, caller string) ([]byte, error) {
	d.dev.Mutex.Lock()
	defer d.dev.Mutex.Unlock()

	bufferW := make([]byte, bufferSizeWrite)
	bufferW[1] = d.Endpoint
	endpointHeaderPosition := bufferW[headerSize : headerSize+len(endpoint)]
	copy(endpointHeaderPosition, endpoint)
	if len(buffer) > 0 {
		copy(bufferW[headerSize+len(endpoint):headerSize+len(endpoint)+len(buffer)], buffer)
	}

	bufferR := make([]byte, bufferSize)

	if d.lightingReportWrite != nil {
		_, err := d.lightingReportWrite(append([]byte(nil), bufferW...))
		return bufferR, err
	}
	if _, err := d.dev.Dev.Write(bufferW); err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial, "caller": caller}).Error("Unable to write to a device")
		return bufferR, err
	}

	if _, err := d.dev.Dev.ReadWithTimeout(bufferR, 1000*time.Millisecond); err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Warn("Unable to read data from device")
		return bufferR, err
	}
	return bufferR, nil
}

// ModifyBatteryLevel will modify battery level
func (d *Device) ModifyBatteryLevel(batteryLevel uint16) {
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	d.publishBatteryLocked(batteryLevel)
}

// Caller holds lightingMu; keep device state and shared statistics together.
func (d *Device) publishBatteryLocked(level uint16) {
	d.BatteryLevel = level
	stats.UpdateBatteryStats(d.Serial, d.Product, level, 1)
}
