package server

import (
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"fmt"
	"strconv"
)

func parseHexColor(hex string) (lightingsettings.Color, error) {
	if len(hex) != 7 || hex[0] != '#' {
		return lightingsettings.Color{}, fmt.Errorf("color must be a #RRGGBB hex string")
	}
	value, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return lightingsettings.Color{}, fmt.Errorf("color must be a valid hex string")
	}
	r := float64(uint8(value >> 16))
	g := float64(uint8(value >> 8))
	b := float64(uint8(value))
	return lightingsettings.Color{Red: r, Green: g, Blue: b}, nil
}

func openRGBLightingColorComponent(component float64) uint8 {
	if component <= 0 {
		return 0
	}
	if component >= 255 {
		return 255
	}
	return uint8(component + 0.5)
}

func openRGBLightingColorHex(color rgb.Color) string {
	return fmt.Sprintf("#%02x%02x%02x",
		openRGBLightingColorComponent(color.Red),
		openRGBLightingColorComponent(color.Green),
		openRGBLightingColorComponent(color.Blue),
	)
}
