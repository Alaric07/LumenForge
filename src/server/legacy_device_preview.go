package server

import (
	"LumenForge/src/common"
	"LumenForge/src/dashboard"
	"LumenForge/src/devices/cpro"
	"LumenForge/src/devices/k70pro"
	"LumenForge/src/devices/virtuosoW"
	"LumenForge/src/keyboards"
	"LumenForge/src/rgb"
	"LumenForge/src/temperatures"
	"LumenForge/src/templates"
	"LumenForge/src/version"
	"net/http"
)

const (
	legacyDevicePreviewCSP       = "default-src 'self'; script-src 'none'; connect-src 'none'; form-action 'none'; base-uri 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:;"
	legacyDevicePreviewPickerCSP = "default-src 'self'; script-src 'none'; connect-src 'none'; form-action 'self'; base-uri 'none'; style-src 'self' 'unsafe-inline'; img-src 'self' data:;"
)

type legacyDevicePreviewFixture struct {
	Key      string
	Title    string
	Template string
	Build    func() interface{}
}

type legacyDevicePreviewPicker struct {
	templates.Web
	Fixtures []legacyDevicePreviewFixture
}

var legacyDevicePreviewFixtures = []legacyDevicePreviewFixture{
	{Key: "commander-pro", Title: "Commander Pro", Template: "cpro.html", Build: buildCommanderProPreview},
	{Key: "k70-pro", Title: "K70 Pro", Template: "k70pro.html", Build: buildK70ProPreview},
	{Key: "virtuoso-wireless", Title: "Virtuoso Wireless", Template: "virtuosoW.html", Build: buildVirtuosoWirelessPreview},
}

func legacyDevicePreviewFixtureByKey(key string) (legacyDevicePreviewFixture, bool) {
	for _, fixture := range legacyDevicePreviewFixtures {
		if fixture.Key == key {
			return fixture, true
		}
	}
	return legacyDevicePreviewFixture{}, false
}

func legacyDevicePreviewWeb() templates.Web {
	dash := dashboard.GetDashboard()
	if dash.Theme == "" {
		dash.Theme = "default"
	}
	if dash.PageTitle == "" {
		dash.PageTitle = "LumenForge"
	}
	dash.TemperatureBar = false

	return templates.Web{
		Title:               dash.PageTitle,
		Devices:             map[string]*common.Device{},
		Temperatures:        map[string]temperatures.TemperatureProfileData{},
		Rgb:                 map[string]rgb.Profile{},
		Dashboard:           dash,
		BuildInfo:           &version.BuildInfo{Revision: "preview", BuildVersion: version.Version},
		LegacyDevicePreview: true,
	}
}

func uiLegacyDevicePreviewPicker(w http.ResponseWriter, r *http.Request) {
	if key := r.URL.Query().Get("fixture"); key != "" {
		if _, ok := legacyDevicePreviewFixtureByKey(key); !ok {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/dev/device-preview/"+key, http.StatusFound)
		return
	}
	web := legacyDevicePreviewPicker{Web: legacyDevicePreviewWeb(), Fixtures: legacyDevicePreviewFixtures}
	web.LegacyDevicePreview = false
	renderLegacyDevicePreviewPicker(w, "device-preview.html", web)
}

func uiDevelopmentRouteNotFound(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func uiLegacyDevicePreview(w http.ResponseWriter, r *http.Request) {
	key, valid := getVar("/dev/device-preview/", r)
	if !valid {
		http.NotFound(w, r)
		return
	}
	fixture, ok := legacyDevicePreviewFixtureByKey(key)
	if !ok {
		http.NotFound(w, r)
		return
	}

	web := legacyDevicePreviewWeb()
	web.Device = fixture.Build()
	renderLegacyDevicePreview(w, fixture.Template, web)
}

func renderLegacyDevicePreview(w http.ResponseWriter, name string, data any) {
	applyLegacyDevicePreviewHeaders(w)
	w.Header().Set("Content-Security-Policy", legacyDevicePreviewCSP)
	executeTemplateOrRespond(w, templates.GetTemplate(), name, data, true)
}

func renderLegacyDevicePreviewPicker(w http.ResponseWriter, name string, data any) {
	applyLegacyDevicePreviewHeaders(w)
	w.Header().Set("Content-Security-Policy", legacyDevicePreviewPickerCSP)
	executeTemplateOrRespond(w, templates.GetTemplate(), name, data, true)
}

func applyLegacyDevicePreviewHeaders(w http.ResponseWriter) {
	for _, header := range headers {
		w.Header().Set(header.Key, header.Value)
	}
}

func buildCommanderProPreview() interface{} {
	brightness := uint8(72)
	return &cpro.Device{
		Product:  "Commander Pro",
		Serial:   "preview-commander-pro",
		Firmware: "0.9.42",
		RGBModes: []string{"static", "rainbow", "wave"},
		DeviceProfile: &cpro.DeviceProfile{
			Brightness:       brightness,
			BrightnessSlider: &brightness,
			RGBProfiles:      map[int]string{0: "static", 1: "rainbow"},
			SpeedProfiles:    map[int]string{0: "Quiet", 1: "Balanced", 2: "Performance"},
			ExternalHubs:     map[int]*cpro.ExternalHubData{0: {PortId: 0, ExternalHubDeviceType: 1, ExternalHubDeviceAmount: 2}},
			Labels:           map[int]string{0: "Front intake", 1: "Radiator pump", 2: "Radiator fan"},
		},
		UserProfiles: map[string]*cpro.DeviceProfile{"Default": {Active: true}, "Quiet": {}},
		Devices: map[int]*cpro.Devices{
			0: {ChannelId: 0, DeviceId: "fan-0", Name: "Front intake", Label: "Front intake", Rpm: 900, Temperature: 31.5, TemperatureString: "31.5°C", HasSpeed: true, HasTemps: true, Profile: "Balanced"},
			1: {ChannelId: 1, DeviceId: "pump-1", Name: "Radiator pump", Label: "Radiator pump", Rpm: 2200, Temperature: 34.2, TemperatureString: "34.2°C", HasSpeed: true, HasTemps: true, ContainsPump: true, Profile: "Performance"},
			2: {ChannelId: 2, DeviceId: "rgb-2", Name: "Radiator RGB", Label: "Radiator RGB", LedChannels: 16, RGB: "rainbow"},
		},
		ExternalLedDeviceAmount: map[int]string{0: "No Device", 1: "1 Device", 2: "2 Devices"},
		ExternalLedDevice:       []cpro.ExternalLedDevice{{Index: 0, Name: "RGB Fan", Total: 8, Amount: 2}},
		RailVoltages:            map[int]*cpro.RailVoltage{0: {Name: "+12V", Value: 12.08}, 1: {Name: "+5V", Value: 5.02}, 2: {Name: "+3.3V", Value: 3.31}},
	}
}

func buildK70ProPreview() interface{} {
	keyboard := &keyboards.Keyboard{Row: map[int]keyboards.Row{
		0: previewKeyboardRow("Esc", "F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12", "PrtSc", "Scroll", "Pause"),
		1: previewKeyboardRow("`", "1", "2", "3", "4", "5", "6", "7", "8", "9", "0", "-", "=", "Backspace"),
		2: previewKeyboardRow("Tab", "Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P", "[", "]", "\\"),
		3: previewKeyboardRow("Caps", "A", "S", "D", "F", "G", "H", "J", "K", "L", ";", "'", "Enter"),
		4: previewKeyboardRow("Shift", "Z", "X", "C", "V", "B", "N", "M", ",", ".", "/", "Shift"),
		5: previewKeyboardRow("Ctrl", "Win", "Alt", "Space", "Alt", "Fn", "Menu", "Ctrl"),
	}}
	previewKeyboardKeySpace(keyboard, 1, 13, "keyboard-key wide3")
	previewKeyboardKeySpace(keyboard, 2, 0, "keyboard-key wide2")
	previewKeyboardKeySpace(keyboard, 2, 13, "keyboard-key wide2")
	previewKeyboardKeySpace(keyboard, 3, 0, "keyboard-key wide2")
	previewKeyboardKeySpace(keyboard, 3, 12, "keyboard-key wide3")
	previewKeyboardKeySpace(keyboard, 4, 0, "keyboard-key wide3")
	previewKeyboardKeySpace(keyboard, 4, 11, "keyboard-key wide3")
	previewKeyboardKeySpace(keyboard, 5, 3, "keyboard-key wide8")
	profile := &k70pro.DeviceProfile{
		RGBProfile:  "rainbow",
		Layout:      "US",
		PollingRate: 1000,
		Profile:     "default",
		Profiles:    []string{"default", "gaming"},
		Keyboards:   map[string]*keyboards.Keyboard{"default": keyboard},
	}
	return &k70pro.Device{
		Product:       "K70 RGB PRO",
		Serial:        "preview-k70-pro",
		Firmware:      "1.2.17",
		RGBModes:      []string{"static", "rainbow", "wave"},
		Layouts:       []string{"US", "UK"},
		PollingRates:  map[int]string{125: "125 Hz", 500: "500 Hz", 1000: "1000 Hz"},
		UserProfiles:  map[string]*k70pro.DeviceProfile{"Default": {Active: true}, "Gaming": {}},
		DeviceProfile: profile,
		UIKeyboard:    "keyboard-6",
		UIKeyboardRow: "keyboard-row-20",
	}
}

func previewKeyboardRow(labels ...string) keyboards.Row {
	colors := []rgb.Color{
		{Red: 247, Green: 196, Blue: 92},
		{Red: 90, Green: 196, Blue: 255},
		{Red: 100, Green: 220, Blue: 160},
		{Red: 180, Green: 130, Blue: 255},
	}
	keys := make(map[int]keyboards.Key, len(labels))
	for index, label := range labels {
		keys[index] = keyboards.Key{KeyName: label, Color: colors[index%len(colors)]}
	}
	return keyboards.Row{Keys: keys}
}

func previewKeyboardKeySpace(keyboard *keyboards.Keyboard, rowIndex, keyIndex int, keySpace string) {
	row := keyboard.Row[rowIndex]
	key := row.Keys[keyIndex]
	key.KeySpace = keySpace
	row.Keys[keyIndex] = key
	keyboard.Row[rowIndex] = row
}

func buildVirtuosoWirelessPreview() interface{} {
	red := rgb.Color{Red: 235, Green: 92, Blue: 92}
	blue := rgb.Color{Red: 86, Green: 170, Blue: 255}
	brightness := uint8(68)
	profile := &virtuosoW.DeviceProfile{
		Brightness:          brightness,
		BrightnessSlider:    &brightness,
		RGBProfile:          "rainbow",
		SleepMode:           15,
		DisableMicIndicator: 1,
		ZoneColors:          map[int]virtuosoW.ZoneColors{0: {Name: "Left earcup", Color: &red}, 1: {Name: "Right earcup", Color: &blue}},
		Equalizers:          map[int]virtuosoW.Equalizer{0: {Name: "32 Hz", Value: 2}, 1: {Name: "1 kHz", Value: -1}, 2: {Name: "8 kHz", Value: 3}},
	}
	return &virtuosoW.Device{
		Connected:     true,
		Usb:           false,
		Product:       "VIRTUOSO Wireless",
		Serial:        "preview-virtuoso-wireless",
		Firmware:      "5.12.18",
		RGBModes:      []string{"static", "rainbow", "wave"},
		SleepModes:    map[int]string{0: "Off", 5: "5 minutes", 15: "15 minutes"},
		ZoneAmount:    2,
		UserProfiles:  map[string]*virtuosoW.DeviceProfile{"Default": {Active: true}, "Movie": {}},
		DeviceProfile: profile,
	}
}
