package controller

// Package: controller
// Author: Nikola Jurkovic
// License: GPL-3.0 or later

import (
	"LumenForge/src/audio"
	"LumenForge/src/config"
	"LumenForge/src/dashboard"
	"LumenForge/src/devices"
	"LumenForge/src/devices/lcd"
	"LumenForge/src/display"
	"LumenForge/src/inputmanager"
	"LumenForge/src/keyboards"
	"LumenForge/src/language"
	"LumenForge/src/logger"
	"LumenForge/src/macro"
	"LumenForge/src/media"
	"LumenForge/src/metrics"
	"LumenForge/src/monitor"
	"LumenForge/src/motherboards"
	"LumenForge/src/rgb"
	"LumenForge/src/scheduler"
	"LumenForge/src/server"
	"LumenForge/src/stats"
	"LumenForge/src/systeminfo"
	"LumenForge/src/systray"
	"LumenForge/src/temperatures"
	"LumenForge/src/version"
)

// Start will start new controller session
func Start() {
	version.Init()      // Build info
	config.Init()       // Configuration
	logger.Init()       // Logger
	display.Init()      // Displays
	media.Init()        // Media client
	audio.Init()        // Audio
	dashboard.Init()    // Dashboard
	systeminfo.Init()   // Build system info
	metrics.Init()      // Metrics
	rgb.Init()          // RGB
	lcd.Init()          // LCD
	temperatures.Init() // Temperatures
	keyboards.Init()    // Keyboards
	inputmanager.Init() // Input Manager
	stats.Init()        // Statistics
	macro.Init()        // Macro
	motherboards.Init() // Motherboards
	devices.Init()      // Devices
	monitor.Init()      // Monitor
	language.Init()     // Language
	scheduler.Init()    // Scheduler
	systray.InitTray()  // System Tray
	server.Init()       // REST & WebUI
}

// Stop will stop device control
func Stop() {
	devices.Stop()      // Devices
	inputmanager.Stop() // Cleanup virtual devices
	audio.StopAudio()   // Virtual Audio
	media.Stop()        // Media client
}
