package scimitarprorgb

import (
	"sort"

	"LumenForge/src/rgb"
)

const (
	scimitarEffectZoneCount    = 4
	scimitarRGBChannelsPerZone = 3
)

type scimitarLightingOwnership uint8

const (
	scimitarLightingUnavailable scimitarLightingOwnership = iota
	scimitarLightingLocal
	scimitarLightingCluster
)

type scimitarLogicalFrame struct {
	zones [scimitarEffectZoneCount][scimitarRGBChannelsPerZone]byte
}

type scimitarLightingAdapter struct {
	ledChannels int
	zoneColors  map[int]ZoneColors
	dpiLeds     DPIProfile
}

func newScimitarLightingAdapter(
	ledChannels int,
	zoneColors map[int]ZoneColors,
	dpiLeds DPIProfile,
) scimitarLightingAdapter {
	return scimitarLightingAdapter{
		ledChannels: ledChannels,
		zoneColors:  zoneColors,
		dpiLeds:     dpiLeds,
	}
}

func scimitarLightingOwner(profile *DeviceProfile) scimitarLightingOwnership {
	if profile == nil {
		return scimitarLightingUnavailable
	}
	if profile.RGBCluster {
		return scimitarLightingCluster
	}
	return scimitarLightingLocal
}

func (ownership scimitarLightingOwnership) allowsLocalRendering() bool {
	return ownership == scimitarLightingLocal
}

func composeScimitarLogicalFrame(data []byte) scimitarLogicalFrame {
	frame := scimitarLogicalFrame{}
	for zone := range frame.zones {
		for channel := range frame.zones[zone] {
			sourceIndex := (zone * scimitarRGBChannelsPerZone) + channel
			if sourceIndex >= len(data) {
				return frame
			}
			frame.zones[zone][channel] = data[sourceIndex]
		}
	}
	return frame
}

func composeScimitarZoneColorLogicalFrame(
	colors [scimitarEffectZoneCount]*rgb.Color,
	brightness uint8,
) scimitarLogicalFrame {
	frame := scimitarLogicalFrame{}
	for zone, color := range colors {
		if color == nil {
			continue
		}
		color.Brightness = rgb.GetBrightnessValueFloat(brightness)
		modified := rgb.ModifyBrightness(*color)
		frame.zones[zone] = [scimitarRGBChannelsPerZone]byte{
			byte(modified.Red),
			byte(modified.Green),
			byte(modified.Blue),
		}
	}
	return frame
}

func composeScimitarUniformLogicalFrame(color rgb.Color) scimitarLogicalFrame {
	frame := scimitarLogicalFrame{}
	logicalColor := [scimitarRGBChannelsPerZone]byte{
		byte(color.Red),
		byte(color.Green),
		byte(color.Blue),
	}
	for zone := range frame.zones {
		frame.zones[zone] = logicalColor
	}
	return frame
}

func composeScimitarStaticLogicalFrame(profile rgb.Profile, brightness uint8) scimitarLogicalFrame {
	profile.StartColor.Brightness = rgb.GetBrightnessValueFloat(brightness)
	color := rgb.ModifyBrightness(profile.StartColor)
	return composeScimitarUniformLogicalFrame(*color)
}

func composeScimitarColorFrame(
	ledChannels int,
	zoneColors map[int]ZoneColors,
	dpiLeds DPIProfile,
	dpiColor rgb.Color,
	data []byte,
) []byte {
	adapter := newScimitarLightingAdapter(ledChannels, zoneColors, dpiLeds)
	logicalFrame := composeScimitarLogicalFrame(data)
	return adapter.composeScimitarHardwareFrame(logicalFrame, dpiColor)
}

func (a scimitarLightingAdapter) composeScimitarHardwareFrame(
	logicalFrame scimitarLogicalFrame,
	dpiColor rgb.Color,
) []byte {
	buf := make([]byte, (a.ledChannels*3)+5) // Append 5 additional places for each LED packet index
	buf[a.dpiLeds.LEDIndexPosition] = byte(a.dpiLeds.LEDIndex)
	for i := 0; i < len(a.dpiLeds.ColorIndex); i++ {
		dpiColorIndexRange := a.dpiLeds.ColorIndex[i]
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

	zoneKeys := make([]int, 0, len(a.zoneColors))
	for key := range a.zoneColors {
		zoneKeys = append(zoneKeys, key)
	}
	sort.Ints(zoneKeys)

	for logicalZone, key := range zoneKeys {
		zoneColor := a.zoneColors[key]
		buf[zoneColor.LEDIndexPosition] = byte(zoneColor.LEDIndex)
		if logicalZone >= scimitarEffectZoneCount {
			continue
		}

		logicalColor := logicalFrame.zones[logicalZone]
		for channel, zoneColorIndex := range zoneColor.ColorIndex {
			if channel >= scimitarRGBChannelsPerZone {
				break
			}
			buf[zoneColorIndex] = logicalColor[channel]
		}
	}

	return buf
}
