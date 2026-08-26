package scimitarrgbelite

import (
	"sort"
	"sync"

	"LumenForge/src/rgb"
)

const (
	scimitarEliteEffectZoneCount    = 4
	scimitarEliteRGBChannelsPerZone = 3
)

type scimitarEliteLightingOwnership uint8

const (
	scimitarEliteLightingUnavailable scimitarEliteLightingOwnership = iota
	scimitarEliteLightingLocal
	scimitarEliteLightingCluster
)

type scimitarEliteLogicalFrame struct {
	zones [scimitarEliteEffectZoneCount][scimitarEliteRGBChannelsPerZone]byte
}

type scimitarEliteExternalFrameOwner uint8

const (
	scimitarEliteExternalFrameNone scimitarEliteExternalFrameOwner = iota
	scimitarEliteExternalFrameOpenRGB
	scimitarEliteExternalFrameCluster
)

type scimitarEliteExternalFrameCache struct {
	mu    sync.RWMutex
	owner scimitarEliteExternalFrameOwner
	frame scimitarEliteLogicalFrame
	valid bool
}

func (cache *scimitarEliteExternalFrameCache) store(owner scimitarEliteExternalFrameOwner, data []byte) bool {
	if owner == scimitarEliteExternalFrameNone || len(data) < scimitarEliteEffectZoneCount*scimitarEliteRGBChannelsPerZone {
		return false
	}
	frame := composeScimitarEliteLogicalFrame(data)
	cache.mu.Lock()
	cache.owner = owner
	cache.frame = frame
	cache.valid = true
	cache.mu.Unlock()
	return true
}

func (cache *scimitarEliteExternalFrameCache) load(owner scimitarEliteExternalFrameOwner) (scimitarEliteLogicalFrame, bool) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	if !cache.valid || cache.owner != owner {
		return scimitarEliteLogicalFrame{}, false
	}
	return cache.frame, true
}

func (cache *scimitarEliteExternalFrameCache) clear() {
	cache.mu.Lock()
	cache.owner = scimitarEliteExternalFrameNone
	cache.frame = scimitarEliteLogicalFrame{}
	cache.valid = false
	cache.mu.Unlock()
}

type scimitarEliteLightingAdapter struct {
	ledChannels int
	zoneColors  map[int]ZoneColors
	dpiLeds     DPIProfile
}

func newScimitarEliteLightingAdapter(
	ledChannels int,
	zoneColors map[int]ZoneColors,
	dpiLeds DPIProfile,
) scimitarEliteLightingAdapter {
	return scimitarEliteLightingAdapter{
		ledChannels: ledChannels,
		zoneColors:  zoneColors,
		dpiLeds:     dpiLeds,
	}
}

func scimitarEliteLightingOwner(profile *DeviceProfile) scimitarEliteLightingOwnership {
	if profile == nil {
		return scimitarEliteLightingUnavailable
	}
	if profile.RGBCluster {
		return scimitarEliteLightingCluster
	}
	return scimitarEliteLightingLocal
}

func (ownership scimitarEliteLightingOwnership) allowsLocalRendering() bool {
	return ownership == scimitarEliteLightingLocal
}

func composeScimitarEliteLogicalFrame(data []byte) scimitarEliteLogicalFrame {
	frame := scimitarEliteLogicalFrame{}
	for zone := range frame.zones {
		for channel := range frame.zones[zone] {
			sourceIndex := (zone * scimitarEliteRGBChannelsPerZone) + channel
			if sourceIndex >= len(data) {
				return frame
			}
			frame.zones[zone][channel] = data[sourceIndex]
		}
	}
	return frame
}

func composeScimitarEliteZoneColorLogicalFrame(
	colors [scimitarEliteEffectZoneCount]*rgb.Color,
	brightness uint8,
) scimitarEliteLogicalFrame {
	frame := scimitarEliteLogicalFrame{}
	for zone, color := range colors {
		if color == nil {
			continue
		}
		resolvedColor := *color
		resolvedColor.Brightness = rgb.GetBrightnessValueFloat(brightness)
		modified := rgb.ModifyBrightness(resolvedColor)
		frame.zones[zone] = [scimitarEliteRGBChannelsPerZone]byte{
			byte(modified.Red),
			byte(modified.Green),
			byte(modified.Blue),
		}
	}
	return frame
}

func composeScimitarEliteUniformLogicalFrame(color rgb.Color) scimitarEliteLogicalFrame {
	frame := scimitarEliteLogicalFrame{}
	logicalColor := [scimitarEliteRGBChannelsPerZone]byte{
		byte(color.Red),
		byte(color.Green),
		byte(color.Blue),
	}
	for zone := range frame.zones {
		frame.zones[zone] = logicalColor
	}
	return frame
}

func composeScimitarEliteStaticLogicalFrame(profile rgb.Profile, brightness uint8) scimitarEliteLogicalFrame {
	profile.StartColor.Brightness = rgb.GetBrightnessValueFloat(brightness)
	color := rgb.ModifyBrightness(profile.StartColor)
	return composeScimitarEliteUniformLogicalFrame(*color)
}

func composeScimitarEliteDPIColor(color rgb.Color, brightness uint8) rgb.Color {
	color.Brightness = rgb.GetBrightnessValueFloat(brightness)
	return *rgb.ModifyBrightness(color)
}

func composeScimitarEliteExternallyOwnedColorFrame(
	ledChannels int,
	zoneColors map[int]ZoneColors,
	dpiLeds DPIProfile,
	dpiColor rgb.Color,
	brightness uint8,
	data []byte,
) []byte {
	return composeScimitarEliteColorFrame(
		ledChannels,
		zoneColors,
		dpiLeds,
		composeScimitarEliteDPIColor(dpiColor, brightness),
		data,
	)
}

func composeScimitarEliteColorFrame(
	ledChannels int,
	zoneColors map[int]ZoneColors,
	dpiLeds DPIProfile,
	dpiColor rgb.Color,
	data []byte,
) []byte {
	adapter := newScimitarEliteLightingAdapter(ledChannels, zoneColors, dpiLeds)
	logicalFrame := composeScimitarEliteLogicalFrame(data)
	return adapter.composeScimitarEliteHardwareFrame(logicalFrame, dpiColor)
}

func (a scimitarEliteLightingAdapter) composeScimitarEliteHardwareFrame(
	logicalFrame scimitarEliteLogicalFrame,
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
		if logicalZone >= scimitarEliteEffectZoneCount {
			continue
		}

		logicalColor := logicalFrame.zones[logicalZone]
		for channel, zoneColorIndex := range zoneColor.ColorIndex {
			if channel >= scimitarEliteRGBChannelsPerZone {
				break
			}
			buf[zoneColorIndex] = logicalColor[channel]
		}
	}

	return buf
}
