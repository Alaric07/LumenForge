package server

import (
	"LumenForge/src/common"
	"LumenForge/src/devices/k100"
	"LumenForge/src/devices/k100airW"
	"LumenForge/src/devices/k100airWU"
	"LumenForge/src/devices/k55"
	"LumenForge/src/devices/k55core"
	"LumenForge/src/devices/k55coretkl"
	"LumenForge/src/devices/k55pro"
	"LumenForge/src/devices/k55proXT"
	"LumenForge/src/devices/k57rgbW"
	"LumenForge/src/devices/k57rgbWU"
	"LumenForge/src/devices/k60rgbpro"
	"LumenForge/src/devices/k65plusW"
	"LumenForge/src/devices/k65plusWU"
	"LumenForge/src/devices/k65pm"
	"LumenForge/src/devices/k65rgb"
	"LumenForge/src/devices/k65rgbRF"
	"LumenForge/src/devices/k65rm"
	"LumenForge/src/devices/k68rgb"
	"LumenForge/src/devices/k70core"
	"LumenForge/src/devices/k70coretkl"
	"LumenForge/src/devices/k70coretklW"
	"LumenForge/src/devices/k70coretklWU"
	"LumenForge/src/devices/k70lux"
	"LumenForge/src/devices/k70luxrgb"
	"LumenForge/src/devices/k70max"
	"LumenForge/src/devices/k70mk2"
	"LumenForge/src/devices/k70pmW"
	"LumenForge/src/devices/k70pmWU"
	"LumenForge/src/devices/k70pro"
	"LumenForge/src/devices/k70protkl"
	"LumenForge/src/devices/k70rgbRF"
	"LumenForge/src/devices/k70rgbtklcs"
	"LumenForge/src/devices/k95"
	"LumenForge/src/devices/k95platinum"
	"LumenForge/src/devices/k95platinumXT"
	"LumenForge/src/devices/strafergbmk2"
	"LumenForge/src/keyboards"
	"LumenForge/src/rgb"
	"LumenForge/src/stats"
	"LumenForge/src/templates"
	"net/http"
)

const commanderDuoModernPreviewSerial = "preview-commander-duo-modern"

type modernDevicePreviewView struct {
	ID    string
	Label string
}

type modernDevicePreviewFixture struct {
	Key         string
	Title       string
	ProductType uint16
	DeviceType  uint32
	Views       []modernDevicePreviewView
	Build       func() *devicesWorkspaceSummary
}

type modernDevicePreviewNavigationView struct {
	Label  string
	Href   string
	Active bool
}

type modernDevicePreviewNavigation struct {
	Views []modernDevicePreviewNavigationView
}

var modernDevicePreviewFixtures = []modernDevicePreviewFixture{
	{Key: "k55-rgb-modern", Title: "K55 RGB", ProductType: common.ProductTypeK55, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK55RGBModernPreview},
	{Key: "k55-core-modern", Title: "K55 CORE RGB", ProductType: common.ProductTypeK55Core, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK55CoreModernPreview},
	{Key: "k55-core-tkl-modern", Title: "K55 CORE TKL", ProductType: common.ProductTypeK55CoreTkl, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK55CoreTKLModernPreview},
	{Key: "k55-pro-modern", Title: "K55 PRO RGB", ProductType: common.ProductTypeK55Pro, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK55ProModernPreview},
	{Key: "k55-pro-xt-modern", Title: "K55 RGB PRO XT", ProductType: common.ProductTypeK55ProXT, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK55ProXTModernPreview},
	{Key: "k57-rgb-wireless-modern", Title: "K57 RGB Wireless", ProductType: common.ProductTypeK57RgbW, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK57RGBWirelessModernPreview},
	{Key: "k57-rgb-usb-modern", Title: "K57 RGB USB", ProductType: common.ProductTypeK57RgbWU, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK57RGBUSBModernPreview},
	{Key: "k100-modern", Title: "K100", ProductType: common.ProductTypeK100, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK100ModernPreview},
	{Key: "k100-air-wireless-modern", Title: "K100 AIR Wireless", ProductType: common.ProductTypeK100AirW, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK100AirWirelessModernPreview},
	{Key: "k100-air-usb-modern", Title: "K100 AIR USB", ProductType: common.ProductTypeK100AirWU, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK100AirUSBModernPreview},
	{Key: "k60-rgb-pro-modern", Title: "K60 RGB PRO", ProductType: common.ProductTypeK60RgbPro, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK60RGBProModernPreview},
	{Key: "k65-plus-usb-modern", Title: "K65 PLUS", ProductType: common.ProductTypeK65Plus, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK65PlusUSBModernPreview},
	{Key: "k65-plus-wireless-modern", Title: "K65 PLUS Wireless", ProductType: common.ProductTypeK65PlusW, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK65PlusWirelessModernPreview},
	{Key: "k65-pro-mini-modern", Title: "K65 PRO MINI", ProductType: common.ProductTypeK65PM, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK65ProMiniModernPreview},
	{Key: "k65-rgb-mini-modern", Title: "K65 RGB MINI", ProductType: common.ProductTypeK65RM, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK65RGBMiniModernPreview},
	{Key: "k65-rgb-modern", Title: "K65 RGB", ProductType: common.ProductTypeK65Rgb, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK65RGBModernPreview},
	{Key: "k65-rgb-rapidfire-modern", Title: "K65 RGB RAPIDFIRE", ProductType: common.ProductTypeK65Rgb, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK65RGBRapidfireModernPreview},
	{Key: "k68-rgb-modern", Title: "K68 RGB", ProductType: common.ProductTypeK68Rgb, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK68RGBModernPreview},
	{Key: "k70-core-modern", Title: "K70 CORE", ProductType: common.ProductTypeK70Core, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70CoreModernPreview},
	{Key: "k70-core-tkl-modern", Title: "K70 CORE TKL", ProductType: common.ProductTypeK70CoreTkl, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70CoreTKLModernPreview},
	{Key: "k70-core-tkl-wireless-modern", Title: "K70 CORE RGB TKL Wireless", ProductType: common.ProductTypeK70CoreTklW, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70CoreTKLWirelessModernPreview},
	{Key: "k70-core-tkl-usb-modern", Title: "K70 CORE RGB TKL USB", ProductType: common.ProductTypeK70CoreTklWU, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70CoreTKLUSBModernPreview},
	{Key: "k70-pro-modern", Title: "K70 RGB PRO", ProductType: common.ProductTypeK70Pro, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70ProModernPreview},
	{Key: "k70-pro-tkl-modern", Title: "K70 RGB PRO TKL", ProductType: common.ProductTypeK70ProTkl, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70ProTKLModernPreview},
	{Key: "k70-rgb-tkl-cs-modern", Title: "K70 RGB TKL CS", ProductType: common.ProductTypeK70RgbTkl, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70RGBTKLCSModernPreview},
	{Key: "k70-pro-mini-wireless-modern", Title: "K70 PRO MINI Wireless", ProductType: common.ProductTypeK70PMW, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70ProMiniWirelessModernPreview},
	{Key: "k70-pro-mini-usb-modern", Title: "K70 PRO MINI", ProductType: common.ProductTypeK70PMWU, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70ProMiniUSBModernPreview},
	{Key: "k70-max-modern", Title: "K70 MAX", ProductType: common.ProductTypeK70Max, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70MaxModernPreview},
	{Key: "k70-lux-modern", Title: "K70 LUX", ProductType: common.ProductTypeK70LUX, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70LUXModernPreview},
	{Key: "k70-lux-rgb-modern", Title: "K70 LUX RGB", ProductType: common.ProductTypeK70LUXRgb, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70LUXRGBModernPreview},
	{Key: "k70-rgb-rf-modern", Title: "K70 RGB RF", ProductType: common.ProductTypeK70RgbRF, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70RGBRFModernPreview},
	{Key: "k70-mk2-modern", Title: "K70 MK2", ProductType: common.ProductTypeK70MK2, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK70MK2ModernPreview},
	{Key: "strafe-rgb-mk2-modern", Title: "STRAFE RGB MK2", ProductType: common.ProductTypeStrafeRgbMk2, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildStrafeRGBMK2ModernPreview},
	{Key: "k95-modern", Title: "K95", ProductType: common.ProductTypeK95, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK95ModernPreview},
	{Key: "k95-platinum-modern", Title: "K95 PLATINUM", ProductType: common.ProductTypeK95Platinum, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK95PlatinumModernPreview},
	{Key: "k95-platinum-xt-modern", Title: "K95 PLATINUM XT", ProductType: common.ProductTypeK95PlatinumXT, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "keyboard", Label: "Keyboard"}}, Build: buildK95PlatinumXTModernPreview},
	{Key: "commander-duo-modern", Title: "Commander Duo", ProductType: common.ProductTypeCCXT, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "cooling", Label: "Cooling"}}, Build: buildCommanderDuoModernPreview},
	{Key: "commander-pro-modern", Title: "Commander Pro", ProductType: common.ProductTypeCPro, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "cooling", Label: "Cooling"}}, Build: buildCommanderProModernPreview},
	{Key: "harpoon-rgb-pro-modern", Title: "Harpoon RGB Pro", ProductType: common.ProductTypeHarpoonRgbPro, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildHarpoonRGBProModernPreview},
	{Key: "glaive-rgb-pro-modern", Title: "Glaive RGB Pro", ProductType: common.ProductTypeGlaiveRgbPro, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildGlaiveRGBProModernPreview},
	{Key: "glaive-rgb-modern", Title: "Glaive RGB", ProductType: common.ProductTypeGlaiveRgb, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildGlaiveRGBModernPreview},
	{Key: "m65-rgb-elite-modern", Title: "M65 RGB Elite", ProductType: common.ProductTypeM65RgbElite, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildM65RGBEliteModernPreview},
	{Key: "sabre-rgb-pro-modern", Title: "Sabre RGB Pro", ProductType: common.ProductTypeSabreRgbPro, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildSabreRGBProModernPreview},
	{Key: "nightsword-rgb-modern", Title: "Nightsword RGB", ProductType: common.ProductTypeNightswordRgb, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildNightswordRGBModernPreview},
	{Key: "ironclaw-rgb-modern", Title: "Ironclaw RGB", ProductType: common.ProductTypeIronClawRgb, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildIronclawRGBModernPreview},
	{Key: "m55-modern", Title: "M55", ProductType: common.ProductTypeM55, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildM55ModernPreview},
	{Key: "m55-rgb-pro-modern", Title: "M55 RGB PRO", ProductType: common.ProductTypeM55RgbPro, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildM55RGBProModernPreview},
	{Key: "m65-pro-rgb-modern", Title: "M65 PRO RGB", ProductType: common.ProductTypeM65RgbElite, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildM65ProRGBModernPreview},
	{Key: "m65-rgb-ultra-modern", Title: "M65 RGB ULTRA", ProductType: common.ProductTypeM65RgbUltra, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildM65RGBUltraModernPreview},
	{Key: "m75-modern", Title: "M75", ProductType: common.ProductTypeM75, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildM75ModernPreview},
	{Key: "m75-wireless-modern", Title: "M75 Wireless", ProductType: common.ProductTypeM75W, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildM75WirelessModernPreview},
	{Key: "m75-air-wireless-modern", Title: "M75 AIR Wireless", ProductType: common.ProductTypeM75AirW, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreview("M75 AIR WIRELESS", "preview-m75-air-wireless-modern", true, true)
	}},
	{Key: "m65-rgb-ultra-wireless-modern", Title: "M65 RGB Ultra Wireless", ProductType: common.ProductTypeM65RgbUltraW, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreview("M65 RGB ULTRA WIRELESS", "preview-m65-rgb-ultra-wireless-modern", true, true)
	}},
	{Key: "harpoon-wireless-modern", Title: "Harpoon Wireless", ProductType: common.ProductTypeHarpoonRgbW, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreview("HARPOON WIRELESS", "preview-harpoon-wireless-modern", false, false)
	}},
	{Key: "m55-wireless-modern", Title: "M55 Wireless", ProductType: common.ProductTypeM55W, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreview("M55 WIRELESS", "preview-m55-wireless-modern", false, false)
	}},
	{Key: "nightsabre-wireless-modern", Title: "Nightsabre Wireless", ProductType: common.ProductTypeNightsabreW, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreview("NIGHTSABRE WIRELESS", "preview-nightsabre-wireless-modern", true, true)
	}},
	{Key: "sabre-rgb-pro-wireless-modern", Title: "Sabre RGB Pro Wireless", ProductType: common.ProductTypeSabreRgbProW, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreview("SABRE RGB PRO WIRELESS", "preview-sabre-rgb-pro-wireless-modern", true, true)
	}},
	{Key: "ironclaw-wireless-modern", Title: "Ironclaw Wireless", ProductType: common.ProductTypeIronClawRgbW, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreview("IRONCLAW WIRELESS", "preview-ironclaw-wireless-modern", false, false)
	}},
	{Key: "ironclaw-wireless-se-modern", Title: "Ironclaw Wireless SE", ProductType: common.ProductTypeIronClawSEW, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreview("IRONCLAW WIRELESS SE", "preview-ironclaw-wireless-se-modern", false, true)
	}},
	{Key: "scimitar-rgb-elite-wireless-modern", Title: "Scimitar RGB Elite Wireless", ProductType: common.ProductTypeScimitarRgbEliteW, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreview("SCIMITAR RGB ELITE WIRELESS", "preview-scimitar-rgb-elite-wireless-modern", false, true)
	}},
	{Key: "scimitar-elite-wireless-se-modern", Title: "Scimitar Elite Wireless SE", ProductType: common.ProductTypeScimitarRgbEliteSEW, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreview("SCIMITAR ELITE WIRELESS SE", "preview-scimitar-elite-wireless-se-modern", false, true)
	}},
	{Key: "dark-core-rgb-pro-se-wireless-modern", Title: "Dark Core RGB Pro SE Wireless", ProductType: common.ProductTypeDarkCoreRgbProSEW, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreviewWithCapabilities("DARK CORE RGB PRO SE WIRELESS", "preview-dark-core-rgb-pro-se-wireless-modern", false, true, true, false)
	}},
	{Key: "dark-core-rgb-pro-wireless-modern", Title: "Dark Core RGB Pro Wireless", ProductType: common.ProductTypeDarkCoreRgbProW, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreviewWithCapabilities("DARK CORE RGB PRO WIRELESS", "preview-dark-core-rgb-pro-wireless-modern", false, true, true, false)
	}},
	{Key: "dark-core-rgb-se-wireless-modern", Title: "Dark Core RGB SE Wireless", ProductType: common.ProductTypeDarkCoreRgbSEW, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreviewWithCapabilities("DARK CORE RGB SE WIRELESS", "preview-dark-core-rgb-se-wireless-modern", false, false, true, true)
	}},
	{Key: "sabre-v2-pro-wireless-modern", Title: "Sabre V2 Pro Wireless", ProductType: common.ProductTypeSabreV2Pro, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: func() *devicesWorkspaceSummary {
		return buildWirelessMouseModernPreviewWithCapabilities("SABRE V2 PRO WIRELESS", "preview-sabre-v2-pro-wireless-modern", false, false, true, true)
	}},
	{Key: "sabre-pro-cs-modern", Title: "SABRE PRO CS", ProductType: common.ProductTypeSabreProCs, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildSabreProCSModernPreview},
	{Key: "scimitar-rgb-modern", Title: "SCIMITAR RGB", ProductType: common.ProductTypeScimitarRgb, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildScimitarRGBModernPreview},
	{Key: "scimitar-elite-modern", Title: "SCIMITAR ELITE", ProductType: common.ProductTypeScimitarRgbElite, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildScimitarEliteModernPreview},
	{Key: "katar-pro-modern", Title: "Katar Pro", ProductType: common.ProductTypeKatarPro, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildKatarProModernPreview},
	{Key: "katar-pro-xt-modern", Title: "Katar Pro XT", ProductType: common.ProductTypeKatarProXT, DeviceType: common.DeviceTypeMouse, Views: []modernDevicePreviewView{{ID: "overview", Label: "Overview"}, {ID: "lighting", Label: "Lighting"}, {ID: "dpi", Label: "DPI"}, {ID: "buttons", Label: "Buttons"}}, Build: buildKatarProXTModernPreview},
}

func buildK55RGBModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k55-rgb-modern"
	keyboard := keyboards.GetKeyboard("k55-default-US")
	profile := &k55.DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true}
	device := &k55.Device{Serial: serial, UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: k55BasicAssignmentTypes(), PollingRates: k55PollingRates(), DeviceProfile: profile, UserProfiles: map[string]*k55.DeviceProfile{"Default": {Active: true}}}
	return buildK55ModernPreviewSummary(serial, "K55 RGB", common.ProductTypeK55, device)
}

func buildK55CoreModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k55-core-modern"
	keyboard := keyboards.GetKeyboard("k55core-default-US")
	profile := &k55core.DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true}
	device := &k55core.Device{Serial: serial, UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: k55BasicAssignmentTypes(), PollingRates: k55PollingRates(), DeviceProfile: profile, UserProfiles: map[string]*k55core.DeviceProfile{"Default": {Active: true}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K55 CORE RGB", ProductType: common.ProductTypeK55Core, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK55CoreTKLModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k55-core-tkl-modern"
	keyboard := keyboards.GetKeyboard("k55coretkl-default-US")
	profile := &k55coretkl.DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true}
	device := &k55coretkl.Device{Serial: serial, UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-21", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}, PollingRates: k55PollingRates(), DeviceProfile: profile, UserProfiles: map[string]*k55coretkl.DeviceProfile{"Default": {Active: true}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K55 CORE TKL", ProductType: common.ProductTypeK55CoreTkl, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK55ProModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k55-pro-modern"
	keyboard := keyboards.GetKeyboard("k55pro-default-US")
	profile := &k55pro.DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true}
	device := &k55pro.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: k55BasicAssignmentTypes(), PollingRates: k55PollingRates(), DeviceProfile: profile, UserProfiles: map[string]*k55pro.DeviceProfile{"Default": {Active: true}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K55 PRO RGB", ProductType: common.ProductTypeK55Pro, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK55ProXTModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k55-pro-xt-modern"
	keyboard := keyboards.GetKeyboard("k55proXT-default-US")
	profile := &k55proXT.DeviceProfile{Profile: "Default", Profiles: []string{"Default"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true}
	device := &k55proXT.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: k55BasicAssignmentTypes(), PollingRates: k55PollingRates(), DeviceProfile: profile, UserProfiles: map[string]*k55proXT.DeviceProfile{"Default": {Active: true}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K55 RGB PRO XT", ProductType: common.ProductTypeK55ProXT, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK55ModernPreviewSummary(serial, product string, productType uint16, device *k55.Device) *devicesWorkspaceSummary {
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: product, ProductType: productType, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func k55BasicAssignmentTypes() map[int]string {
	return map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}
}

func k55PollingRates() map[int]string {
	return map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec"}
}

func buildK70CoreModernPreview() *devicesWorkspaceSummary {
	return buildK70CorePreview("preview-k70-core-modern", "K70 CORE", common.ProductTypeK70Core, "k70core-default-US", "keyboard-6", "keyboard-row-25")
}
func buildK65ProMiniModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k65-pro-mini-modern"
	keyboard := keyboards.GetKeyboard("k65pm-default-US")
	profile := &k65pm.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true, DisableAltTab: true}
	device := &k65pm.Device{Serial: serial, UIKeyboard: "keyboard-5", UIKeyboardRow: "keyboard-row-17", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k65pm.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K65 PRO MINI", ProductType: common.ProductTypeK65PM, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}
func buildK65PlusWirelessModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k65-plus-wireless-modern"
	k := keyboards.GetKeyboard("k65plus-default-US")
	p := &k65plusW.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, ControlDial: 1, SleepMode: 15, DisableWinKey: true}
	d := &k65plusW.Device{Serial: serial, UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-17", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}, ControlDialOptions: map[int]string{1: "Volume Control", 2: "Brightness"}, SleepModes: map[int]string{5: "5 minutes", 15: "15 minutes"}, DeviceProfile: p, UserProfiles: map[string]*k65plusW.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	s, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K65 PLUS", ProductType: common.ProductTypeK65PlusW, Instance: d}}, map[string]stats.BatteryStats{}, serial)
	s.LegacyLighting = true
	return s
}

func buildK65PlusUSBModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k65-plus-usb-modern"
	keyboard := keyboards.GetKeyboard("k65plus-default-US")
	profile := &k65plusWU.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, ControlDial: 1, DisableWinKey: true}
	device := &k65plusWU.Device{Serial: serial, UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-17", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}, ControlDialOptions: map[int]string{1: "Volume Control", 2: "Brightness"}, DeviceProfile: profile, UserProfiles: map[string]*k65plusWU.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K65 PLUS", ProductType: common.ProductTypeK65Plus, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}
func buildK65RGBMiniModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k65-rgb-mini-modern"
	keyboard := keyboards.GetKeyboard("k65rm-default-US")
	profile := &k65rm.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true, DisableAltTab: true}
	device := &k65rm.Device{Serial: serial, UIKeyboard: "keyboard-5", UIKeyboardRow: "keyboard-row-16", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k65rm.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K65 RGB MINI", ProductType: common.ProductTypeK65RM, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}
func buildK65RGBModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k65-rgb-modern"
	keyboard := keyboards.GetKeyboard("k65rgb-default-US")
	profile := &k65rgb.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 1, DisableWinKey: true, DisableAltTab: true}
	device := &k65rgb.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), PollingRates: map[int]string{0: "Not Set", 1: "1000 Hz / 1 msec", 2: "500 Hz / 2 msec", 4: "250 Hz / 4 msec", 8: "125 Hz / 8 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k65rgb.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K65 RGB", ProductType: common.ProductTypeK65Rgb, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK60RGBProModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k60-rgb-pro-modern"
	keyboard := keyboards.GetKeyboard("k60rgbpro-default-US")
	profile := &k60rgbpro.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true, DisableAltTab: true}
	device := &k60rgbpro.Device{Serial: serial, UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k60rgbpro.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K60 RGB PRO", ProductType: common.ProductTypeK60RgbPro, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK57RGBWirelessModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k57-rgb-wireless-modern"
	keyboard := keyboards.GetKeyboard("k57rgb-default-US")
	profile := &k57rgbW.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, SleepMode: 15, DisableWinKey: true, DisableAltTab: true}
	device := &k57rgbW.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Brightness +", 12: "Brightness -", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}, SleepModes: map[int]string{1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, DeviceProfile: profile, UserProfiles: map[string]*k57rgbW.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K57 RGB", ProductType: common.ProductTypeK57RgbW, Instance: device}}, map[string]stats.BatteryStats{serial: {Level: 78}}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK57RGBUSBModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k57-rgb-usb-modern"
	keyboard := keyboards.GetKeyboard("k57rgb-default-US")
	profile := &k57rgbWU.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true, DisableAltTab: true}
	device := &k57rgbWU.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k57rgbWU.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K57 RGB", ProductType: common.ProductTypeK57RgbWU, Instance: device}}, map[string]stats.BatteryStats{serial: {Level: 78}}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK100ModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k100-modern"
	keyboard := keyboards.GetKeyboard("k100-default-US")
	profile := &k100.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, ControlDial: 1, DebounceTime: 1, DisableWinKey: true, DisableAltTab: true, ControlDialColors: map[int]*rgb.Color{1: {Red: 255, Green: 255, Blue: 255}, 2: {Red: 0, Green: 255, Blue: 255}, 3: {Red: 255, Green: 127, Blue: 0}, 4: {Red: 255, Green: 0, Blue: 255}, 5: {Red: 0, Green: 255, Blue: 0}, 6: {Red: 0, Green: 0, Blue: 255}, 7: {Red: 255, Green: 0, Blue: 0}}}
	device := &k100.Device{Serial: serial, UIKeyboard: "keyboard-8", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US", "DE", "FR", "SE", "UK"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 2: "DPI +", 3: "Keyboard", 4: "DPI -", 8: "Sniper", 9: "Mouse", 10: "Macro", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}, DebounceTimes: map[int]string{1: "1ms", 2: "2ms", 3: "3ms", 4: "4ms", 5: "5ms", 6: "6ms", 7: "7ms", 8: "8ms", 9: "9ms"}, ControlDialOptions: map[int]string{1: "Volume Control", 2: "Brightness", 3: "Vertical Scroll", 4: "Zoom", 5: "Screen Brightness", 6: "Media Control", 7: "Horizontal Scroll"}, DeviceProfile: profile, UserProfiles: map[string]*k100.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K100", ProductType: common.ProductTypeK100, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK100AirWirelessModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k100-air-wireless-modern"
	keyboard := keyboards.GetKeyboard("k100air-default-US")
	profile := &k100airW.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, SleepMode: 15, AutoBrightness: 1, DisableWinKey: true, DisableAltTab: true}
	device := &k100airW.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US", "DE", "FR"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), SleepModes: map[int]string{1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}, DeviceProfile: profile, UserProfiles: map[string]*k100airW.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K100 AIR WIRELESS", ProductType: common.ProductTypeK100AirW, Instance: device}}, map[string]stats.BatteryStats{serial: {Level: 78}}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK100AirUSBModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k100-air-usb-modern"
	keyboard := keyboards.GetKeyboard("k100air-default-US")
	profile := &k100airWU.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, AutoBrightness: 1, DisableWinKey: true, DisableAltTab: true}
	device := &k100airWU.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US", "DE", "FR"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k100airWU.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K100 AIR USB", ProductType: common.ProductTypeK100AirWU, Instance: device}}, map[string]stats.BatteryStats{serial: {Level: 78}}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK65RGBRapidfireModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k65-rgb-rapidfire-modern"
	keyboard := keyboards.GetKeyboard("k65rgbRF-default-US")
	profile := &k65rgbRF.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 1, DisableWinKey: true, DisableAltTab: true}
	device := &k65rgbRF.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), PollingRates: map[int]string{0: "Not Set", 1: "1000 Hz / 1 msec", 2: "500 Hz / 2 msec", 4: "250 Hz / 4 msec", 8: "125 Hz / 8 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k65rgbRF.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K65 RGB RAPIDFIRE", ProductType: common.ProductTypeK65Rgb, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK68RGBModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k68-rgb-modern"
	keyboard := keyboards.GetKeyboard("k68rgb-default-US")
	profile := &k68rgb.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 1, DisableWinKey: true, DisableAltTab: true}
	device := &k68rgb.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US", "DE"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), PollingRates: map[int]string{0: "Not Set", 8: "125 Hz / 8 msec", 4: "250 Hz / 4 msec", 2: "500 Hz / 2 msec", 1: "1000 Hz / 1 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k68rgb.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K68 RGB", ProductType: common.ProductTypeK68Rgb, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}
func buildK70CoreTKLModernPreview() *devicesWorkspaceSummary {
	return buildK70CoreTKLPreview("preview-k70-core-tkl-modern", "K70 CORE TKL", common.ProductTypeK70CoreTkl, "k70coretkl-default-US", "keyboard-6", "keyboard-row-20")
}
func buildK70CoreTKLWirelessModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k70-core-tkl-wireless-modern"
	k := keyboards.GetKeyboard("k70coretklW-default-US")
	p := &k70coretklW.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, SleepMode: 15, ControlDial: 1, DisableWinKey: true}
	d := &k70coretklW.Device{Serial: serial, UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), ControlDialOptions: k70CoreTKLControlDialOptions(), SleepModes: k70CoreTKLSleepModes(), DeviceProfile: p, UserProfiles: map[string]*k70coretklW.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	s, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K70 CORE RGB TKL Wireless", ProductType: common.ProductTypeK70CoreTklW, Instance: d}}, map[string]stats.BatteryStats{serial: {Level: 78}}, serial)
	s.LegacyLighting = true
	return s
}
func buildK70CoreTKLUSBModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k70-core-tkl-usb-modern"
	k := keyboards.GetKeyboard("k70coretklW-default-US")
	p := &k70coretklWU.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, PollingRate: 4, ControlDial: 1, DisableWinKey: true}
	d := &k70coretklWU.Device{Serial: serial, UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), ControlDialOptions: k70CoreTKLControlDialOptions(), PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec"}, DeviceProfile: p, UserProfiles: map[string]*k70coretklWU.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	s, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K70 CORE RGB TKL USB", ProductType: common.ProductTypeK70CoreTklWU, Instance: d}}, map[string]stats.BatteryStats{serial: {Level: 78}}, serial)
	s.LegacyLighting = true
	return s
}
func k70CoreTKLControlDialOptions() map[int]string {
	return map[int]string{1: "Volume Control", 2: "Brightness", 3: "Scroll", 4: "Zoom", 5: "Screen Brightness", 6: "Media Control", 7: "Horizontal Scroll"}
}
func k70CoreTKLSleepModes() map[int]string {
	return map[int]string{1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}
}
func buildK70ProModernPreview() *devicesWorkspaceSummary {
	return buildK70ProModernWorkspacePreview("preview-k70-pro-modern", "K70 RGB PRO", common.ProductTypeK70Pro, "k70pro-default-US", "keyboard-7", "keyboard-row-25")
}
func buildK70ProTKLModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k70-pro-tkl-modern"
	keyboard := keyboards.GetKeyboard("k70protkl-default-US")
	profile := &k70protkl.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true, DisableAltTab: true}
	device := &k70protkl.Device{Serial: serial, UIKeyboard: "keyboard-6", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), PollingRates: map[int]string{1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k70protkl.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K70 RGB PRO TKL", ProductType: common.ProductTypeK70ProTkl, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.KeyActuation = &devicesKeyActuationWorkspaceSummary{Supported: true, MinValue: 1, MaxValue: 40, SecondaryMinimumGap: 4, Keys: []devicesKeyActuationKeySummary{{KeyIndex: 4, KeyName: "A", Supported: true, ActuationPoint: 20, ActuationResetPoint: 18, EnableActuationPointReset: true, EnableSecondaryActuationPoint: true, SecondaryActuationPoint: 30, SecondaryActuationResetPoint: 29}, {KeyIndex: 57, KeyName: "Fn", Supported: false}}}
	summary.FlashTap = &devicesFlashTapWorkspaceSummary{Supported: true, Active: true, Mode: 1, Modes: []devicesFlashTapOptionSummary{{Value: 0, Label: "Neutral"}, {Value: 1, Label: "Last Priority"}, {Value: 2, Label: "First Priority"}}, Keys: []devicesFlashTapKeySummary{{KeyIndex: 4, KeyName: "A", Eligible: true, Selected: true}, {KeyIndex: 7, KeyName: "D", Eligible: true, Selected: true}, {KeyIndex: 57, KeyName: "Fn", Eligible: false}}, SelectedSlots: []devicesFlashTapSelectedSlotSummary{{SlotIndex: 0, KeyIndex: 7}, {SlotIndex: 1, KeyIndex: 4}}, Color: devicesFlashTapColorSummary{Red: 17, Green: 93, Blue: 201}}
	summary.LegacyLighting = true
	return summary
}
func buildK70RGBTKLCSModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k70-rgb-tkl-cs-modern"
	keyboard := keyboards.GetKeyboard("k70rgbtklcs-default-US")
	profile := &k70rgbtklcs.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true, DisableAltTab: true}
	device := &k70rgbtklcs.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-20", Layouts: []string{"US"}, KeyAssignmentTypes: k70RGBTKLCSPreviewAssignmentTypes(), PollingRates: k70RGBTKLCSPreviewPollingRates(), DeviceProfile: profile, UserProfiles: map[string]*k70rgbtklcs.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K70 RGB TKL CS", ProductType: common.ProductTypeK70RgbTkl, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	return summary
}
func k70RGBTKLCSPreviewAssignmentTypes() map[int]string {
	return map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Brightness +", 12: "Brightness -", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}
}
func k70RGBTKLCSPreviewPollingRates() map[int]string {
	return map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}
}
func buildK70ProMiniWirelessModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k70-pro-mini-wireless-modern"
	k := keyboards.GetKeyboard("k70pm-default-US")
	p := &k70pmW.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, SleepMode: 15, DisableWinKey: true}
	d := &k70pmW.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-18", Layouts: []string{"US"}, KeyAssignmentTypes: k70ProMiniPreviewAssignmentTypes(), SleepModes: k70ProMiniPreviewSleepModes(), DeviceProfile: p, UserProfiles: map[string]*k70pmW.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	s, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K70 PRO MINI", ProductType: common.ProductTypeK70PMW, Instance: d}}, map[string]stats.BatteryStats{serial: {Level: 78}}, serial)
	s.LegacyLighting = true
	return s
}
func buildK70ProMiniUSBModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k70-pro-mini-usb-modern"
	k := keyboards.GetKeyboard("k70pm-default-US")
	p := &k70pmWU.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, PollingRate: 4, DisableWinKey: true}
	d := &k70pmWU.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-18", Layouts: []string{"US"}, KeyAssignmentTypes: k70ProMiniPreviewAssignmentTypes(), PollingRates: k70ProMiniPreviewPollingRates(), DeviceProfile: p, UserProfiles: map[string]*k70pmWU.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	s, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K70 PRO MINI", ProductType: common.ProductTypeK70PMWU, Instance: d}}, map[string]stats.BatteryStats{serial: {Level: 78}}, serial)
	s.LegacyLighting = true
	return s
}
func k70ProMiniPreviewAssignmentTypes() map[int]string {
	return map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}
}
func k70ProMiniPreviewSleepModes() map[int]string {
	return map[int]string{1: "1 minute", 5: "5 minutes", 10: "10 minutes", 15: "15 minutes", 30: "30 minutes", 60: "1 hour"}
}
func k70ProMiniPreviewPollingRates() map[int]string {
	return map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}
}
func buildK70MaxModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k70-max-modern"
	keyboard := keyboards.GetKeyboard("k70max-default-US")
	profile := &k70max.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true, DisableAltTab: true}
	device := &k70max.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), PollingRates: map[int]string{1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k70max.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K70 MAX", ProductType: common.ProductTypeK70Max, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	if summary.KeyboardAssignments != nil && len(summary.KeyboardAssignments.ModifierOptions) > 1 {
		for row := range summary.KeyboardAssignments.Rows {
			for key := range summary.KeyboardAssignments.Rows[row].Keys {
				if summary.KeyboardAssignments.Rows[row].Keys[key].KeyIndex == 71 {
					summary.KeyboardAssignments.Rows[row].Keys[key].ModifierKey = summary.KeyboardAssignments.ModifierOptions[1].ID
					summary.KeyboardAssignments.Rows[row].Keys[key].RetainOriginal = true
				}
			}
		}
	}
	summary.KeyActuation = &devicesKeyActuationWorkspaceSummary{Supported: true, MinValue: 1, MaxValue: 40, SecondaryMinimumGap: 4, Keys: []devicesKeyActuationKeySummary{{KeyIndex: 71, KeyName: "A", Supported: true, ActuationPoint: 20, ActuationResetPoint: 18, EnableActuationPointReset: true, EnableSecondaryActuationPoint: true, SecondaryActuationPoint: 30, SecondaryActuationResetPoint: 29}, {KeyIndex: 4, KeyName: "Logo", Supported: false}}}
	summary.FlashTap = &devicesFlashTapWorkspaceSummary{Supported: true, Active: true, Mode: 1, Modes: []devicesFlashTapOptionSummary{{Value: 0, Label: "Neutral"}, {Value: 1, Label: "Last Priority"}, {Value: 2, Label: "First Priority"}}, Keys: []devicesFlashTapKeySummary{{KeyIndex: 71, KeyName: "A", Eligible: true, Selected: true}, {KeyIndex: 73, KeyName: "D", Eligible: true, Selected: true}, {KeyIndex: 4, KeyName: "Logo", Eligible: false}}, SelectedSlots: []devicesFlashTapSelectedSlotSummary{{SlotIndex: 0, KeyIndex: 73}, {SlotIndex: 1, KeyIndex: 71}}, Color: devicesFlashTapColorSummary{Red: 19, Green: 97, Blue: 203}}
	summary.LegacyLighting = true
	return summary
}
func buildK70CorePreview(serial, product string, typ uint16, layout, ui, row string) *devicesWorkspaceSummary {
	k := keyboards.GetKeyboard(layout)
	p := &k70core.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, PollingRate: 4, DisableWinKey: true}
	d := &k70core.Device{Serial: serial, UIKeyboard: ui, UIKeyboardRow: row, Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec"}, DeviceProfile: p, UserProfiles: map[string]*k70core.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	s, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: product, ProductType: typ, Instance: d}}, map[string]stats.BatteryStats{}, serial)
	s.LegacyLighting = true
	return s
}
func buildK70CoreTKLPreview(serial, product string, typ uint16, layout, ui, row string) *devicesWorkspaceSummary {
	k := keyboards.GetKeyboard(layout)
	p := &k70coretkl.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, PollingRate: 4, DisableWinKey: true}
	d := &k70coretkl.Device{Serial: serial, UIKeyboard: ui, UIKeyboardRow: row, Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec"}, DeviceProfile: p, UserProfiles: map[string]*k70coretkl.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	s, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: product, ProductType: typ, Instance: d}}, map[string]stats.BatteryStats{}, serial)
	s.LegacyLighting = true
	return s
}
func buildK70ProModernWorkspacePreview(serial, product string, typ uint16, layout, ui, row string) *devicesWorkspaceSummary {
	k := keyboards.GetKeyboard(layout)
	p := &k70pro.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": k}, PollingRate: 4, DisableWinKey: true}
	d := &k70pro.Device{Serial: serial, UIKeyboard: ui, UIKeyboardRow: row, Layouts: []string{"US"}, KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Brightness +", 12: "Brightness -", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec", 5: "2000 Hz / 0.5 msec", 6: "4000 Hz / 0.25 msec", 7: "8000 Hz / 0.125 msec"}, DeviceProfile: p, UserProfiles: map[string]*k70pro.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	s, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: product, ProductType: typ, Instance: d}}, map[string]stats.BatteryStats{}, serial)
	s.LegacyLighting = true
	return s
}

func buildK70LUXModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k70-lux-modern"
	keyboard := keyboards.GetKeyboard("k70lux-default-US")
	profile := &k70lux.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 1, DisableWinKey: true, DisableAltTab: true}
	device := &k70lux.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), PollingRates: map[int]string{0: "Not Set", 1: "1000 Hz / 1 msec", 2: "500 Hz / 2 msec", 4: "250 Hz / 4 msec", 8: "125 Hz / 8 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k70lux.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K70 LUX", ProductType: common.ProductTypeK70LUX, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK70LUXRGBModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k70-lux-rgb-modern"
	keyboard := keyboards.GetKeyboard("k70luxrgb-default-US")
	profile := &k70luxrgb.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 1, DisableWinKey: true, DisableAltTab: true}
	device := &k70luxrgb.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), PollingRates: map[int]string{0: "Not Set", 1: "1000 Hz / 1 msec", 2: "500 Hz / 2 msec", 4: "250 Hz / 4 msec", 8: "125 Hz / 8 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k70luxrgb.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K70 LUX RGB", ProductType: common.ProductTypeK70LUXRgb, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK70RGBRFModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k70-rgb-rf-modern"
	keyboard := keyboards.GetKeyboard("k70rgbRF-default-US")
	profile := &k70rgbRF.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 1, DisableWinKey: true, DisableAltTab: true}
	device := &k70rgbRF.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), PollingRates: map[int]string{0: "Not Set", 1: "1000 Hz / 1 msec", 2: "500 Hz / 2 msec", 4: "250 Hz / 4 msec", 8: "125 Hz / 8 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k70rgbRF.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K70 RGB RF", ProductType: common.ProductTypeK70RgbRF, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK70MK2ModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k70-mk2-modern"
	keyboard := keyboards.GetKeyboard("k70mk2-default-US")
	profile := &k70mk2.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 1, DisableWinKey: true, DisableAltTab: true}
	device := &k70mk2.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), PollingRates: map[int]string{0: "Not Set", 1: "1000 Hz / 1 msec", 2: "500 Hz / 2 msec", 4: "250 Hz / 4 msec", 8: "125 Hz / 8 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k70mk2.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K70 MK2", ProductType: common.ProductTypeK70MK2, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildStrafeRGBMK2ModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-strafe-rgb-mk2-modern"
	keyboard := keyboards.GetKeyboard("strafergbmk2-default-US")
	profile := &strafergbmk2.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 1, DisableWinKey: true, DisableShiftTab: true, DisableAltTab: true, DisableAltF4: true}
	device := &strafergbmk2.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-25", Layouts: []string{"US"}, KeyAssignmentTypes: k70ModernPreviewAssignmentTypes(), PollingRates: map[int]string{0: "Not Set", 8: "125 Hz / 8 msec", 4: "250 Hz / 4 msec", 2: "500 Hz / 2 msec", 1: "1000 Hz / 1 msec"}, DeviceProfile: profile, UserProfiles: map[string]*strafergbmk2.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "STRAFE RGB MK2", ProductType: common.ProductTypeStrafeRgbMk2, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func k70ModernPreviewAssignmentTypes() map[int]string {
	return map[int]string{0: "None", 1: "Media Keys", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 13: "Scroll Up", 14: "Scroll Down", 15: "Zoom In", 16: "Zoom Out", 17: "Screen Brightness +", 18: "Screen Brightness -"}
}

func buildK95ModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k95-modern"
	keyboard := keyboards.GetKeyboard("k95-default-US")
	profile := &k95.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 1, DisableWinKey: true, DisableAltTab: true}
	device := &k95.Device{Serial: serial, UIKeyboard: "keyboard-7", UIKeyboardRow: "keyboard-row-27", Layouts: []string{"US", "UK"}, KeyAssignmentTypes: map[int]string{0: "None", 10: "Macro"}, PollingRates: map[int]string{0: "Not Set", 1: "1000 Hz / 1 msec", 2: "500 Hz / 2 msec", 4: "250 Hz / 4 msec", 8: "125 Hz / 8 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k95.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K95", ProductType: common.ProductTypeK95, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK95PlatinumModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k95-platinum-modern"
	keyboard := keyboards.GetKeyboard("k95platinum-default-US")
	profile := &k95platinum.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, KeyboardLiveSync: true, DisableWinKey: true, DisableAltTab: true}
	device := &k95platinum.Device{Serial: serial, UIKeyboard: "keyboard-8", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US", "UK"}, KeyAssignmentTypes: map[int]string{0: "None", 10: "Macro"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k95platinum.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K95 PLATINUM", ProductType: common.ProductTypeK95Platinum, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func buildK95PlatinumXTModernPreview() *devicesWorkspaceSummary {
	const serial = "preview-k95-platinum-xt-modern"
	keyboard := keyboards.GetKeyboard("k95platinumXT-default-US")
	profile := &k95platinumXT.DeviceProfile{Profile: "Default", Profiles: []string{"Default", "Gaming"}, Layout: "US", Keyboards: map[string]*keyboards.Keyboard{"Default": keyboard}, PollingRate: 4, DisableWinKey: true, DisableAltTab: true}
	device := &k95platinumXT.Device{Serial: serial, UIKeyboard: "keyboard-8", UIKeyboardRow: "keyboard-row-26", Layouts: []string{"US", "UK"}, KeyAssignmentTypes: map[int]string{0: "None", 10: "Macro"}, PollingRates: map[int]string{0: "Not Set", 1: "125 Hz / 8 msec", 2: "250 Hz / 4 msec", 3: "500 Hz / 2 msec", 4: "1000 Hz / 1 msec"}, DeviceProfile: profile, UserProfiles: map[string]*k95platinumXT.DeviceProfile{"Default": {Active: true}, "Gaming": {}}}
	summary, _ := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: {Serial: serial, Product: "K95 PLATINUM XT", ProductType: common.ProductTypeK95PlatinumXT, Instance: device}}, map[string]stats.BatteryStats{}, serial)
	summary.LegacyLighting = true
	return summary
}

func modernDevicePreviewFixtureByKey(key string) (modernDevicePreviewFixture, bool) {
	for _, fixture := range modernDevicePreviewFixtures {
		if fixture.Key == key {
			return fixture, true
		}
	}
	return modernDevicePreviewFixture{}, false
}

// uiModernDevicePreview renders fixture-backed modern workspace data. It does
// not create or register a hardware device, and its CSP disables interaction.
func uiModernDevicePreview(w http.ResponseWriter, r *http.Request) {
	key, valid := getVar("/dev/device-preview/", r)
	if !valid {
		http.NotFound(w, r)
		return
	}
	fixture, ok := modernDevicePreviewFixtureByKey(key)
	if !ok {
		http.NotFound(w, r)
		return
	}

	summary := fixture.Build()
	summary.View = devicesWorkspaceView(r.URL.Query()["view"], summary)
	web := legacyDevicePreviewWeb()
	web.Page = "devices"
	web.Devices = map[string]*common.Device{
		summary.Serial: {
			Serial:      summary.Serial,
			Product:     summary.Product,
			Firmware:    summary.Firmware,
			ProductType: fixture.ProductType,
			DeviceType:  fixture.DeviceType,
			Image:       summary.Image,
		},
	}
	web.Device = summary
	web.BatteryStats = map[string]stats.BatteryStats{}
	web.ModernDevicePreview = modernDevicePreviewNavigationForFixture(fixture, summary.View)
	renderModernDevicePreview(w, "devices.html", web)
}

func buildCommanderProModernPreview() *devicesWorkspaceSummary {
	summary := &devicesWorkspaceSummary{
		Product: "Commander Pro", Serial: "preview-commander-pro-modern", Firmware: "1.4.17", Image: "icon-device.svg", View: "overview", LegacyLighting: true,
		Cooling:           &devicesCoolingWorkspaceSummary{ProfileOptions: []devicesCoolingProfileOptionSummary{{ID: "Balanced", Label: "Balanced"}, {ID: "Quiet", Label: "Quiet"}, {ID: "Performance", Label: "Performance"}}, Channels: []devicesCoolingChannelSummary{{ID: 0, Name: "Fan Channel 1", Label: "Front intake", RPM: 940, SelectedProfile: "Quiet"}, {ID: 1, Name: "Fan Channel 2", Label: "Radiator fan", RPM: 1380, SelectedProfile: "Balanced"}, {ID: 2, Name: "Fan Channel 3", Label: "Rear exhaust", RPM: 1160, SelectedProfile: "Performance"}}, TemperatureProbes: []devicesCoolingTemperatureProbeSummary{{ID: 6, Name: "Temperature Probe 1", Label: "Coolant", Temperature: "31.4°C"}, {ID: 7, Name: "Temperature Probe 2", Label: "Case", Temperature: "28.8°C"}}},
		DeviceProfiles:    &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"Default", "Quiet", "Studio"}, ActiveProfile: "Default", Scope: "device", Label: "Device Profile", Description: devicesGenericDeviceProfileDescription},
		OverviewCooling:   &devicesOverviewCoolingStatusSummary{Fans: []devicesOverviewStatusRow{{ChannelID: 0, Label: "Front intake", Value: "940 RPM", Telemetry: true}, {ChannelID: 1, Label: "Radiator fan", Value: "1380 RPM", Telemetry: true}, {ChannelID: 2, Label: "Rear exhaust", Value: "1160 RPM", Telemetry: true}}},
		TemperatureProbes: []devicesOverviewStatusRow{{ChannelID: 6, Label: "Coolant", Value: "31.4°C", Telemetry: true}, {ChannelID: 7, Label: "Case", Value: "28.8°C", Telemetry: true}},
		OverviewTelemetry: []devicesOverviewStatusRow{{Label: "+12V", Value: "12.08 V", Telemetry: true}, {Label: "+5V", Value: "5.02 V", Telemetry: true}, {Label: "+3.3V", Value: "3.31 V", Telemetry: true}},
	}
	return summary
}

func modernDevicePreviewNavigationForFixture(fixture modernDevicePreviewFixture, currentView string) modernDevicePreviewNavigation {
	navigation := modernDevicePreviewNavigation{Views: make([]modernDevicePreviewNavigationView, 0, len(fixture.Views))}
	for _, view := range fixture.Views {
		href := "/dev/device-preview/" + fixture.Key
		if view.ID != "overview" {
			href += "?view=" + view.ID
		}
		navigation.Views = append(navigation.Views, modernDevicePreviewNavigationView{Label: view.Label, Href: href, Active: view.ID == currentView})
	}
	return navigation
}

func renderModernDevicePreview(w http.ResponseWriter, name string, data any) {
	applyLegacyDevicePreviewHeaders(w)
	w.Header().Set("Content-Security-Policy", legacyDevicePreviewCSP)
	executeTemplateOrRespond(w, templates.GetTemplate(), name, data, true)
}

func buildCommanderDuoModernPreview() *devicesWorkspaceSummary {
	return &devicesWorkspaceSummary{
		Product:        "iCUE COMMANDER DUO",
		Serial:         commanderDuoModernPreviewSerial,
		Firmware:       "0.9.42",
		Image:          "icon-device.svg",
		View:           "overview",
		LegacyLighting: true,
		Cooling: &devicesCoolingWorkspaceSummary{
			ProfileOptions: []devicesCoolingProfileOptionSummary{
				{ID: "Balanced", Label: "Balanced"},
				{ID: "Quiet", Label: "Quiet"},
				{ID: "Performance", Label: "Performance"},
			},
			Channels: []devicesCoolingChannelSummary{
				{ID: 0, Name: "Fan Channel 1", Label: "Front intake", RPM: 980, SelectedProfile: "Quiet"},
				{ID: 1, Name: "Fan Channel 2", Label: "Radiator pump", RPM: 2240, ContainsPump: true, SelectedProfile: "Performance"},
				{ID: 2, Name: "Fan Channel 3", Label: "Radiator fan", RPM: 1320, SelectedProfile: "Balanced"},
			},
			TemperatureProbes: []devicesCoolingTemperatureProbeSummary{
				{ID: 3, Name: "Temperature Probe 1", Label: "Coolant", Temperature: "31.2°C"},
				{ID: 4, Name: "Temperature Probe 2", Label: "Case exhaust", Temperature: "28.6°C"},
			},
		},
		DeviceProfiles: &devicesDeviceProfileWorkspaceSummary{
			Profiles:  []string{"Default", "Quiet", "Studio"},
			CanSwitch: true, CanSave: true, CanDelete: true,
			ActiveProfile: "Default",
			Scope:         "device",
			Label:         "Device Profile",
			Description:   devicesCCXTDeviceProfileDescription,
		},
		OverviewCooling: &devicesOverviewCoolingStatusSummary{
			Pumps: []devicesOverviewCoolingPumpSummary{{ChannelID: 1, Label: "Radiator pump", RPM: "2240 RPM"}},
			Fans:  []devicesOverviewStatusRow{{ChannelID: 0, Label: "Front intake", Value: "980 RPM", Telemetry: true}, {ChannelID: 2, Label: "Radiator fan", Value: "1320 RPM", Telemetry: true}},
		},
		TemperatureProbes: []devicesOverviewStatusRow{{ChannelID: 3, Label: "Coolant", Value: "31.2°C", Telemetry: true}, {ChannelID: 4, Label: "Case exhaust", Value: "28.6°C", Telemetry: true}},
	}
}

func buildHarpoonRGBProModernPreview() *devicesWorkspaceSummary {
	regularStages := []devicesDPIStageSummary{
		{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#ff0000"},
		{ID: "1", Name: "Stage 2", DPI: 1500, ColorHex: "#ffa500", Active: true},
		{ID: "2", Name: "Stage 3", DPI: 3000, ColorHex: "#ffff00"},
		{ID: "3", Name: "Stage 4", DPI: 6000, ColorHex: "#00ff00"},
		{ID: "4", Name: "Stage 5", DPI: 9000, ColorHex: "#0000ff"},
	}
	return &devicesWorkspaceSummary{
		Product: "HARPOON RGB PRO", Serial: "preview-harpoon-rgb-pro-modern", Firmware: "1.12.41", Image: "icon-mouse.svg", View: "overview", LegacyLighting: true,
		DPI:                 &devicesDPIWorkspaceSummary{MinimumDPI: 200, MaximumDPI: 12000, ActiveRegularStageID: "1", RegularStages: regularStages, SniperStage: &devicesDPIStageSummary{ID: "5", Name: "Sniper", DPI: 200, ColorHex: "#ffff00", Sniper: true}},
		Performance:         &devicesPerformanceWorkspaceSummary{PollingRate: &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Not Set"}, {Value: 1, Label: "1000 Hz / 1 msec"}, {Value: 2, Label: "500 Hz / 2 msec"}, {Value: 4, Label: "250 Hz / 4 msec"}, {Value: 8, Label: "125 Hz / 8 msec"}}}},
		Buttons:             &devicesButtonsWorkspaceSummary{Buttons: []devicesButtonsButtonSummary{{KeyIndex: 1, Name: "Left Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 8, Name: "Back Button", Default: true}, {KeyIndex: 16, Name: "Forward Button", Default: true}, {KeyIndex: 32, Name: "DPI Button", Default: true}}, AssignmentTypes: []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI"}, {ID: 3, Label: "Keyboard"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}},
		DeviceProfiles:      &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"Default", "FPS"}, ActiveProfile: "Default", Scope: "device", Label: "Device Profile", Description: devicesGenericDeviceProfileDescription},
		OverviewPerformance: &devicesOverviewPerformanceStatusSummary{Rows: []devicesOverviewStatusRow{{Label: "DPI", Value: "1500", Telemetry: true}, {Label: "Active Stage", Value: "Stage 2"}, {Label: "Polling Rate", Value: "1000 Hz / 1 msec", Telemetry: true}}},
	}
}

func buildGlaiveRGBProModernPreview() *devicesWorkspaceSummary {
	return &devicesWorkspaceSummary{
		Product: "GLAIVE RGB PRO", Serial: "preview-glaive-rgb-pro-modern", Firmware: "1.6.19", Image: "icon-mouse.svg", View: "overview", LegacyLighting: true,
		DPI:                 &devicesDPIWorkspaceSummary{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "1", RegularStages: []devicesDPIStageSummary{{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#00ff00"}, {ID: "1", Name: "Stage 2", DPI: 1500, ColorHex: "#00ff00", Active: true}, {ID: "2", Name: "Stage 3", DPI: 3000, ColorHex: "#00ff00"}}, SniperStage: &devicesDPIStageSummary{ID: "3", Name: "Sniper", DPI: 400, ColorHex: "#ffff00", Sniper: true}},
		Performance:         &devicesPerformanceWorkspaceSummary{PollingRate: &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Not Set"}, {Value: 1, Label: "1000 Hz / 1 msec"}, {Value: 2, Label: "500 Hz / 2 msec"}, {Value: 4, Label: "250 Hz / 4 msec"}, {Value: 8, Label: "125 Hz / 8 msec"}}}, AngleSnapping: &devicesPerformanceToggleSummary{Enabled: true}},
		Buttons:             &devicesButtonsWorkspaceSummary{Buttons: []devicesButtonsButtonSummary{{KeyIndex: 1, Name: "Left Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 8, Name: "Back Button", Default: true}, {KeyIndex: 16, Name: "Forward Button", Default: true}, {KeyIndex: 32, Name: "DPI Up", Default: true}, {KeyIndex: 64, Name: "DPI Down", Default: true}}, AssignmentTypes: []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI"}, {ID: 3, Label: "Keyboard"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}},
		DeviceProfiles:      &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"Default", "FPS"}, ActiveProfile: "Default", Scope: "device", Label: "Device Profile", Description: devicesGenericDeviceProfileDescription},
		OverviewPerformance: &devicesOverviewPerformanceStatusSummary{Rows: []devicesOverviewStatusRow{{Label: "DPI", Value: "1500", Telemetry: true}, {Label: "Active Stage", Value: "Stage 2"}, {Label: "Polling Rate", Value: "1000 Hz / 1 msec", Telemetry: true}}},
	}
}

func buildGlaiveRGBModernPreview() *devicesWorkspaceSummary {
	s := buildGlaiveRGBProModernPreview()
	s.Product, s.Serial, s.Firmware = "GLAIVE RGB", "preview-glaive-rgb-modern", "1.2.7"
	s.DPI.MaximumDPI = 16000
	s.DPI.RegularStages = append(s.DPI.RegularStages, devicesDPIStageSummary{ID: "3", Name: "Stage 4", DPI: 6000, ColorHex: "#00ff00"}, devicesDPIStageSummary{ID: "4", Name: "Stage 5", DPI: 9000, ColorHex: "#00ff00"})
	s.DPI.SniperStage.ID, s.DPI.SniperStage.DPI = "5", 200
	s.Performance.ButtonOptimization = &devicesPerformanceSelectSummary{Value: 4, Options: []devicesPerformanceOptionSummary{{Value: 1, Label: "Extreme"}, {Value: 2, Label: "Very Fast"}, {Value: 3, Label: "Fast"}, {Value: 4, Label: "Normal"}}}
	s.Performance.LiftHeight = &devicesPerformanceSelectSummary{Value: 3, Options: []devicesPerformanceOptionSummary{{Value: 2, Label: "Low"}, {Value: 3, Label: "Medium"}, {Value: 4, Label: "High"}}}
	s.Buttons.Buttons = s.Buttons.Buttons[:6]
	s.Buttons.Buttons[5].Name = "DPI Toggle"
	return s
}

func buildM65RGBEliteModernPreview() *devicesWorkspaceSummary {
	return &devicesWorkspaceSummary{Product: "M65 RGB ELITE", Serial: "preview-m65-rgb-elite-modern", Firmware: "1.10.22", Image: "icon-mouse.svg", View: "overview", LegacyLighting: true,
		DPI:            &devicesDPIWorkspaceSummary{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "1", RegularStages: []devicesDPIStageSummary{{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#ff0000"}, {ID: "1", Name: "Stage 2", DPI: 1500, ColorHex: "#ffffff", Active: true}, {ID: "2", Name: "Stage 3", DPI: 3000, ColorHex: "#00ff00"}, {ID: "3", Name: "Stage 4", DPI: 6000, ColorHex: "#800080"}, {ID: "4", Name: "Stage 5", DPI: 9000, ColorHex: "#00bfff"}}, SniperStage: &devicesDPIStageSummary{ID: "5", Name: "Sniper", DPI: 400, ColorHex: "#ffff00", Sniper: true}},
		Performance:    &devicesPerformanceWorkspaceSummary{PollingRate: &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Not Set"}, {Value: 1, Label: "1000 Hz / 1 msec"}, {Value: 2, Label: "500 Hz / 2 msec"}, {Value: 4, Label: "250 Hz / 4 msec"}, {Value: 8, Label: "125 Hz / 8 msec"}}}, ButtonOptimization: &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Disabled"}, {Value: 1, Label: "Enabled"}}}, AngleSnapping: &devicesPerformanceToggleSummary{Enabled: true}},
		Buttons:        &devicesButtonsWorkspaceSummary{Buttons: []devicesButtonsButtonSummary{{KeyIndex: 1, Name: "Left Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 8, Name: "Back Button", Default: true}, {KeyIndex: 16, Name: "Forward Button", Default: true}, {KeyIndex: 32, Name: "DPI Up", Default: true}, {KeyIndex: 64, Name: "DPI Down", Default: true}, {KeyIndex: 128, Name: "Sniper", PressAndHold: true}}, AssignmentTypes: []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI +"}, {ID: 3, Label: "Keyboard"}, {ID: 4, Label: "DPI -"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}},
		DeviceProfiles: &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"Default", "FPS"}, CanSwitch: true, CanSave: true, CanDelete: true, ActiveProfile: "Default", Scope: "device", Label: "Device Profile", Description: devicesGenericDeviceProfileDescription}, OverviewPerformance: &devicesOverviewPerformanceStatusSummary{Rows: []devicesOverviewStatusRow{{Label: "DPI", Value: "1500", Telemetry: true}, {Label: "Active Stage", Value: "Stage 2"}, {Label: "Polling Rate", Value: "1000 Hz / 1 msec", Telemetry: true}}}}
}

func buildSabreRGBProModernPreview() *devicesWorkspaceSummary {
	s := buildM65RGBEliteModernPreview()
	s.Product, s.Serial, s.Firmware = "SABRE RGB PRO", "preview-sabre-rgb-pro-modern", "0.8.15"
	s.Performance.PollingRate = &devicesPerformanceSelectSummary{Value: 7, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Not Set"}, {Value: 1, Label: "125 Hz / 8 msec"}, {Value: 2, Label: "250 Hz / 4 msec"}, {Value: 3, Label: "500 Hz / 2 msec"}, {Value: 4, Label: "1000 Hz / 1 msec"}, {Value: 5, Label: "2000 Hz / 0.5 msec"}, {Value: 6, Label: "4000 Hz / 0.25 msec"}, {Value: 7, Label: "8000 Hz / 0.125 msec"}}}
	s.Buttons.Buttons = s.Buttons.Buttons[:6]
	s.Buttons.Buttons[5].Name = "DPI"
	s.Buttons.AssignmentTypes = []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI"}, {ID: 3, Label: "Keyboard"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}
	return s
}

func buildNightswordRGBModernPreview() *devicesWorkspaceSummary {
	return &devicesWorkspaceSummary{Product: "NIGHTSWORD RGB", Serial: "preview-nightsword-rgb-modern", Firmware: "1.4.12", Image: "icon-mouse.svg", View: "overview", LegacyLighting: true,
		DPI:            &devicesDPIWorkspaceSummary{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "1", RegularStages: []devicesDPIStageSummary{{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#00ff00"}, {ID: "1", Name: "Stage 2", DPI: 1500, ColorHex: "#00ff00", Active: true}, {ID: "2", Name: "Stage 3", DPI: 3000, ColorHex: "#00ff00"}}, SniperStage: &devicesDPIStageSummary{ID: "3", Name: "Sniper", DPI: 200, ColorHex: "#ffff00", Sniper: true}},
		Performance:    &devicesPerformanceWorkspaceSummary{PollingRate: &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Not Set"}, {Value: 1, Label: "1000 Hz / 1 msec"}, {Value: 2, Label: "500 Hz / 2 msec"}, {Value: 4, Label: "250 Hz / 4 msec"}, {Value: 8, Label: "125 Hz / 8 msec"}}}},
		Buttons:        &devicesButtonsWorkspaceSummary{Buttons: []devicesButtonsButtonSummary{{KeyIndex: 1, Name: "Left Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 8, Name: "Back Button", Default: true}, {KeyIndex: 16, Name: "Forward Button", Default: true}, {KeyIndex: 32, Name: "DPI Up", Default: true}, {KeyIndex: 64, Name: "DPI Down", Default: true}, {KeyIndex: 128, Name: "Sniper", PressAndHold: true}, {KeyIndex: 256, Name: "Profile Up", Default: true, ProfileSwitch: true}, {KeyIndex: 512, Name: "Profile Down", Default: true, ProfileSwitch: true}}, AssignmentTypes: []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI"}, {ID: 3, Label: "Keyboard"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}},
		DeviceProfiles: &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"Default", "FPS"}, CanSwitch: true, CanSave: true, CanDelete: true, ActiveProfile: "Default", Scope: "device", Label: "Device Profile", Description: devicesGenericDeviceProfileDescription}, OverviewPerformance: &devicesOverviewPerformanceStatusSummary{Rows: []devicesOverviewStatusRow{{Label: "DPI", Value: "1500", Telemetry: true}, {Label: "Active Stage", Value: "Stage 2"}, {Label: "Polling Rate", Value: "1000 Hz / 1 msec", Telemetry: true}}}}
}

func buildIronclawRGBModernPreview() *devicesWorkspaceSummary {
	s := buildNightswordRGBModernPreview()
	s.Product, s.Serial, s.Firmware = "IRONCLAW RGB", "preview-ironclaw-rgb-modern", "2.1.8"
	s.Buttons.Buttons = []devicesButtonsButtonSummary{{KeyIndex: 1, Name: "Left Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 8, Name: "Back Button", Default: true}, {KeyIndex: 16, Name: "Forward Button", Default: true}, {KeyIndex: 32, Name: "DPI Button", Default: true}, {KeyIndex: 256, Name: "Profile Button", Default: true, ProfileSwitch: true}}
	s.Performance.AngleSnapping = &devicesPerformanceToggleSummary{Enabled: true}
	return s
}

func buildM55ModernPreview() *devicesWorkspaceSummary {
	s := buildSabreRGBProModernPreview()
	s.Product, s.Serial = "M55", "preview-m55-modern"
	s.Buttons.Buttons = s.Buttons.Buttons[:6]
	return s
}
func buildM55RGBProModernPreview() *devicesWorkspaceSummary {
	s := buildM65RGBEliteModernPreview()
	s.Product, s.Serial = "M55 RGB PRO", "preview-m55-rgb-pro-modern"
	s.Buttons.Buttons = []devicesButtonsButtonSummary{{KeyIndex: 1, Name: "Left Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 8, Name: "Left Back", Default: true}, {KeyIndex: 16, Name: "Left Forward", Default: true}, {KeyIndex: 32, Name: "Right Back", Default: true}, {KeyIndex: 64, Name: "Right Forward", Default: true}, {KeyIndex: 128, Name: "DPI Button", Default: true}}
	s.Performance.ButtonOptimization = nil
	s.Performance.AngleSnapping = nil
	return s
}
func buildM65ProRGBModernPreview() *devicesWorkspaceSummary {
	s := buildM65RGBEliteModernPreview()
	s.Product, s.Serial = "M65 PRO RGB", "preview-m65-pro-rgb-modern"
	s.Performance.ButtonOptimization = nil
	s.Performance.LiftHeight = &devicesPerformanceSelectSummary{Value: 3, Options: []devicesPerformanceOptionSummary{{Value: 2, Label: "Low"}, {Value: 3, Label: "Medium"}, {Value: 4, Label: "High"}}}
	return s
}
func buildM65RGBUltraModernPreview() *devicesWorkspaceSummary {
	s := buildM65RGBEliteModernPreview()
	s.Product, s.Serial = "M65 RGB ULTRA", "preview-m65-rgb-ultra-modern"
	s.Buttons.Buttons = append(s.Buttons.Buttons, devicesButtonsButtonSummary{KeyIndex: 256, Name: "Tilt Front", Default: true}, devicesButtonsButtonSummary{KeyIndex: 512, Name: "Tilt Back", Default: true}, devicesButtonsButtonSummary{KeyIndex: 1024, Name: "Tilt Left", Default: true}, devicesButtonsButtonSummary{KeyIndex: 2048, Name: "Tilt Right", Default: true})
	s.Performance.LiftHeight = &devicesPerformanceSelectSummary{Value: 3, Options: []devicesPerformanceOptionSummary{{Value: 2, Label: "Low"}, {Value: 3, Label: "Medium"}, {Value: 4, Label: "High"}}}
	return s
}
func buildM75ModernPreview() *devicesWorkspaceSummary {
	s := buildM55RGBProModernPreview()
	s.Product, s.Serial = "M75", "preview-m75-modern"
	s.Performance.ButtonOptimization = &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Disabled"}, {Value: 1, Label: "Enabled"}}}
	s.Performance.AngleSnapping = &devicesPerformanceToggleSummary{Enabled: true}
	s.Performance.LiftHeight = &devicesPerformanceSelectSummary{Value: 3, Options: []devicesPerformanceOptionSummary{{Value: 2, Label: "Low"}, {Value: 3, Label: "Medium"}, {Value: 4, Label: "High"}}}
	return s
}

func buildM75WirelessModernPreview() *devicesWorkspaceSummary {
	s := buildM75ModernPreview()
	s.Product, s.Serial, s.Firmware = "M75 WIRELESS", "preview-m75-wireless-modern", "1.4.32"
	s.HasBattery, s.BatteryLevel = true, 78
	s.SleepTimer = &devicesSleepTimerWorkspaceSummary{Value: 15, Options: []devicesSleepTimerOptionSummary{{Value: 1, Label: "1 minute"}, {Value: 5, Label: "5 minutes"}, {Value: 10, Label: "10 minutes"}, {Value: 15, Label: "15 minutes"}, {Value: 30, Label: "30 minutes"}, {Value: 60, Label: "1 hour"}}}
	return s
}
func buildWirelessMouseModernPreview(product, serial string, polling, lift bool) *devicesWorkspaceSummary {
	return buildWirelessMouseModernPreviewWithCapabilities(product, serial, polling, true, true, lift)
}

func buildWirelessMouseModernPreviewWithCapabilities(product, serial string, polling, buttonOptimization, angleSnapping, lift bool) *devicesWorkspaceSummary {
	s := buildM75WirelessModernPreview()
	s.Product, s.Serial = product, serial
	if !polling {
		s.Performance.PollingRate = nil
	}
	if !buttonOptimization {
		s.Performance.ButtonOptimization = nil
	}
	if !angleSnapping {
		s.Performance.AngleSnapping = nil
	}
	if !lift {
		s.Performance.LiftHeight = nil
	}
	return s
}
func buildSabreProCSModernPreview() *devicesWorkspaceSummary {
	s := buildSabreRGBProModernPreview()
	s.Product, s.Serial = "SABRE PRO CS", "preview-sabre-pro-cs-modern"
	s.Performance.ButtonOptimization = &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Disabled"}, {Value: 1, Label: "Enabled"}}}
	return s
}
func buildScimitarRGBModernPreview() *devicesWorkspaceSummary {
	s := buildM65ProRGBModernPreview()
	s.Product, s.Serial = "SCIMITAR RGB", "preview-scimitar-rgb-modern"
	s.Buttons.Buttons = s.Buttons.Buttons[:5]
	s.Buttons.Buttons = append(s.Buttons.Buttons, devicesButtonsButtonSummary{KeyIndex: 256, Name: "Side Button 1", Default: true}, devicesButtonsButtonSummary{KeyIndex: 512, Name: "Side Button 2", Default: true}, devicesButtonsButtonSummary{KeyIndex: 1024, Name: "Side Button 3", Default: true}, devicesButtonsButtonSummary{KeyIndex: 2048, Name: "Side Button 4", Default: true}, devicesButtonsButtonSummary{KeyIndex: 4096, Name: "Side Button 5", Default: true}, devicesButtonsButtonSummary{KeyIndex: 8192, Name: "Side Button 6", Default: true}, devicesButtonsButtonSummary{KeyIndex: 16384, Name: "Side Button 7", Default: true}, devicesButtonsButtonSummary{KeyIndex: 32768, Name: "Side Button 8", Default: true}, devicesButtonsButtonSummary{KeyIndex: 65536, Name: "Side Button 9", Default: true}, devicesButtonsButtonSummary{KeyIndex: 131072, Name: "Side Button 10", Default: true}, devicesButtonsButtonSummary{KeyIndex: 262144, Name: "Side Button 11", Default: true}, devicesButtonsButtonSummary{KeyIndex: 524288, Name: "Side Button 12", Default: true})
	return s
}
func buildScimitarEliteModernPreview() *devicesWorkspaceSummary {
	s := buildScimitarRGBModernPreview()
	s.Product, s.Serial = "SCIMITAR ELITE", "preview-scimitar-elite-modern"
	s.Performance.LiftHeight = nil
	return s
}

func buildKatarProModernPreview() *devicesWorkspaceSummary {
	return buildKatarModernPreview("KATAR PRO", "preview-katar-pro-modern", "2.4.17", "Stage 2")
}

func buildKatarProXTModernPreview() *devicesWorkspaceSummary {
	return buildKatarModernPreview("KATAR PRO XT", "preview-katar-pro-xt-modern", "3.1.28", "Stage 2")
}

func buildKatarModernPreview(product, serial, firmware, activeStage string) *devicesWorkspaceSummary {
	return &devicesWorkspaceSummary{
		Product: product, Serial: serial, Firmware: firmware, Image: "icon-mouse.svg", View: "overview", LegacyLighting: true,
		DPI:                 &devicesDPIWorkspaceSummary{MinimumDPI: 100, MaximumDPI: 12400, ActiveRegularStageID: "1", RegularStages: []devicesDPIStageSummary{{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#ff0000"}, {ID: "1", Name: activeStage, DPI: 1600, ColorHex: "#00ff00", Active: true}, {ID: "2", Name: "Stage 3", DPI: 3200, ColorHex: "#0000ff"}}, SniperStage: &devicesDPIStageSummary{ID: "3", Name: "Sniper", DPI: 400, ColorHex: "#ffff00", Sniper: true}},
		Performance:         &devicesPerformanceWorkspaceSummary{PollingRate: &devicesPerformanceSelectSummary{Value: 4, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Not Set"}, {Value: 1, Label: "125 Hz / 8 msec"}, {Value: 2, Label: "250 Hz / 4 msec"}, {Value: 3, Label: "500 Hz / 2 msec"}, {Value: 4, Label: "1000 Hz / 1 msec"}}}, ButtonOptimization: &devicesPerformanceSelectSummary{Value: 1, Options: []devicesPerformanceOptionSummary{{Value: 0, Label: "Disabled"}, {Value: 1, Label: "Enabled"}}}},
		Buttons:             &devicesButtonsWorkspaceSummary{Buttons: []devicesButtonsButtonSummary{{KeyIndex: 1, Name: "Left Button", Default: true}, {KeyIndex: 2, Name: "Right Button", Default: true}, {KeyIndex: 4, Name: "Middle Button", Default: true}, {KeyIndex: 8, Name: "Forward Button", Default: true}, {KeyIndex: 16, Name: "Back Button", Default: true}, {KeyIndex: 32, Name: "DPI Button", Default: true}}, AssignmentTypes: []devicesButtonsAssignmentTypeSummary{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI"}, {ID: 3, Label: "Keyboard"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}},
		DeviceProfiles:      &devicesDeviceProfileWorkspaceSummary{Profiles: []string{"Default", "FPS"}, ActiveProfile: "Default", Scope: "device", Label: "Device Profile", Description: devicesGenericDeviceProfileDescription},
		OverviewPerformance: &devicesOverviewPerformanceStatusSummary{Rows: []devicesOverviewStatusRow{{Label: "DPI", Value: "1600", Telemetry: true}, {Label: "Active Stage", Value: activeStage}, {Label: "Polling Rate", Value: "1000 Hz / 1 msec", Telemetry: true}}},
	}
}
