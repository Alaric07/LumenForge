package rgb

import (
	"math"
	"time"
)

// Aurora will run RGB function to simulate a flowing green/teal/purple northern lights effect
func (r *ActiveRGB) Aurora(startTime *time.Time) {
	elapsed := time.Since(*startTime).Milliseconds()
	tSeconds := float64(elapsed) / 1000.0
	// Respect configured speed slider/value by multiplying time factor
	// Speed factor typically ranges from 0.1 to 10
	tScaled := tSeconds * r.RgbModeSpeed

	buf := map[int][]byte{}
	for j := 0; j < r.LightChannels; j++ {
		// Calculate position factor across channels
		pos := float64(j) / float64(r.LightChannels)

		// Combine sine waves to create organic, slow wave movement
		v1 := math.Sin(pos*2.0 + tScaled*0.5)
		v2 := math.Cos(pos*4.5 - tScaled*0.3)
		v3 := math.Sin(pos*1.0 + tScaled*0.8)

		intensity := (v1 * 0.45) + (v2 * 0.35) + (v3 * 0.20)
		// Normalize to [0, 1]
		intensity = (intensity + 1.0) / 2.0
		intensity = clampFloat01(intensity)

		// Aurora Palette mapping:
		// Low: Deep Violet / Dark Blue
		// Mid: Bright Teal / Cyan
		// High: Neon Green
		var red, green, blue float64
		if intensity < 0.25 {
			factor := intensity / 0.25
			red = lerp(55, 0, factor)
			green = lerp(0, 30, factor)
			blue = lerp(110, 180, factor)
		} else if intensity < 0.65 {
			factor := (intensity - 0.25) / 0.40
			red = 0
			green = lerp(30, 200, factor)
			blue = lerp(180, 150, factor)
		} else {
			factor := (intensity - 0.65) / 0.35
			red = 0
			green = lerp(200, 255, factor)
			blue = lerp(150, 60, factor)
		}

		color := Color{
			Red:        red,
			Green:      green,
			Blue:       blue,
			Brightness: r.RGBBrightness,
		}
		modify := ModifyBrightness(color)

		if len(r.Buffer) > 0 {
			r.Buffer[j] = byte(modify.Red)
			r.Buffer[j+r.ColorOffset] = byte(modify.Green)
			r.Buffer[j+(r.ColorOffset*2)] = byte(modify.Blue)
		} else {
			buf[j] = []byte{
				byte(modify.Red),
				byte(modify.Green),
				byte(modify.Blue),
			}
			if r.IsAIO && r.HasLCD {
				if j > 15 && j < 20 {
					buf[j] = []byte{0, 0, 0}
				}
			}
		}
	}

	// Raw colors
	r.Raw = buf

	if r.Inverted {
		r.Output = SetColorInverted(buf)
	} else {
		r.Output = SetColor(buf)
	}
}
