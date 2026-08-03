package rgb

import (
	"math"
	"sort"
	"time"
)

// ColorshiftGradient runs a smooth gradient effect across multiple colors
func (r *ActiveRGB) ColorshiftGradient(startTime time.Time, gradients map[int]Color, durationSeconds float64) {
	elapsed := time.Since(startTime).Seconds()
	globalProgress := gradientProgress(elapsed, durationSeconds)
	r.colorshiftGradientAtProgress(gradients, globalProgress)
}

func gradientProgress(elapsed, durationSeconds float64) float64 {
	if durationSeconds <= 0 || math.IsNaN(durationSeconds) {
		durationSeconds = 5
	}

	return math.Mod(elapsed/durationSeconds, 1.0) // normalized 0..1
}

func (r *ActiveRGB) colorshiftGradientAtProgress(gradients map[int]Color, globalProgress float64) {
	color, ok := gradientColorAtProgress(gradients, globalProgress, r.RGBBrightness)
	if !ok {
		return
	}

	// Fill buffer
	buf := map[int][]byte{}
	for j := 0; j < r.LightChannels; j++ {
		if len(r.Buffer) > 0 {
			r.Buffer[j] = byte(color.Red)
			r.Buffer[j+r.ColorOffset] = byte(color.Green)
			r.Buffer[j+(r.ColorOffset*2)] = byte(color.Blue)
		} else {
			buf[j] = []byte{
				byte(color.Red),
				byte(color.Green),
				byte(color.Blue),
			}
			if r.IsAIO && r.HasLCD {
				if j > 15 && j < 20 {
					buf[j] = []byte{0, 0, 0}
				}
			}
		}
	}

	r.Raw = buf

	// Handle inversion/output
	if r.Inverted {
		r.Output = SetColorInverted(buf)
	} else {
		r.Output = SetColor(buf)
	}
}

func gradientColorAtProgress(gradients map[int]Color, globalProgress, brightness float64) (Color, bool) {
	if math.IsNaN(globalProgress) || math.IsInf(globalProgress, 0) {
		return Color{}, false
	}

	numColors := len(gradients)
	if numColors < 2 {
		return Color{}, false // Need at least 2 colors
	}

	// Convert map to sorted slice based on Position
	gradSlice := make([]Color, 0, numColors)
	for i := 0; i < numColors; i++ {
		color, ok := gradients[i]
		if !ok || math.IsNaN(color.Position) || math.IsInf(color.Position, 0) || color.Position < 0 || color.Position > 1 {
			return Color{}, false
		}
		color.Red = gradientChannelValue(color.Red)
		color.Green = gradientChannelValue(color.Green)
		color.Blue = gradientChannelValue(color.Blue)
		color.Brightness = gradientBrightnessValue(color.Brightness)
		gradSlice = append(gradSlice, color)
	}
	sort.Slice(gradSlice, func(i, j int) bool {
		return gradSlice[i].Position < gradSlice[j].Position
	})

	// Find the current segment based on position/time
	var colorA, colorB Color
	var segmentProgress float64

	for i := 0; i < len(gradSlice)-1; i++ {
		if globalProgress >= gradSlice[i].Position && globalProgress < gradSlice[i+1].Position {
			colorA = gradSlice[i]
			colorB = gradSlice[i+1]
			segmentRange := colorB.Position - colorA.Position
			segmentProgress = (globalProgress - colorA.Position) / segmentRange
			break
		}
	}

	// Handle the exact last stop and wrap-around (last to first)
	firstColor := gradSlice[0]
	lastColor := gradSlice[len(gradSlice)-1]
	if globalProgress == lastColor.Position {
		colorA = lastColor
		colorB = lastColor
	} else if globalProgress > lastColor.Position || globalProgress < firstColor.Position {
		colorA = lastColor
		colorB = firstColor
		var ok bool
		segmentProgress, ok = gradientWrapSegmentProgress(globalProgress, colorA.Position, colorB.Position)
		if !ok {
			return Color{}, false
		}
	}

	// Interpolate colors with brightness
	color := interpolateColorsWithBrightness(colorA, colorB, segmentProgress)
	color.Brightness = gradientBrightnessValue(brightness)
	return *ModifyBrightness(color), true
}

func gradientWrapSegmentProgress(globalProgress, lastPosition, firstPosition float64) (float64, bool) {
	segmentRange := 1.0 - lastPosition + firstPosition
	if segmentRange <= 0 {
		return 0, false
	}
	if globalProgress < firstPosition {
		globalProgress++
	}
	return (globalProgress - lastPosition) / segmentRange, true
}

func gradientBrightnessValue(value float64) float64 {
	if math.IsNaN(value) || value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func gradientChannelValue(value float64) float64 {
	if math.IsNaN(value) || value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return value
}
