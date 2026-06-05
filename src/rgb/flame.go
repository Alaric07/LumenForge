package rgb

import (
	"math"
	"time"
)

// Flame will run RGB function to simulate a dynamic flame effect
func (r *ActiveRGB) Flame(startTime *time.Time) {
	elapsed := time.Since(*startTime).Milliseconds()
	tSeconds := float64(elapsed) / 1000.0

	// Loop time every 10 seconds to prevent float overflow or huge numbers in sin
	if tSeconds >= 10.0 {
		*startTime = time.Now()
		elapsed = 0
		tSeconds = 0.0
	}

	buf := map[int][]byte{}
	for j := 0; j < r.LightChannels; j++ {
		// Calculate position factor across channels
		pos := float64(j) / float64(r.LightChannels)

		// Combine sine waves of different frequencies and speeds to create organic flicker
		v1 := math.Sin(pos*3.0 + tSeconds*2.0)
		v2 := math.Sin(pos*7.0 - tSeconds*4.5)
		v3 := math.Sin(pos*15.0 + tSeconds*9.0)

		intensity := (v1 * 0.5) + (v2 * 0.3) + (v3 * 0.2)
		// Normalize to [0, 1]
		intensity = (intensity + 1.0) / 2.0

		// Add dynamic spark updates (8 times per second)
		timeSlot := math.Floor(tSeconds * 8.0)
		spark := random01(float64(j), timeSlot)
		if spark > 0.96 {
			intensity += 0.25
		}

		intensity = clampFloat01(intensity)

		// Map intensity to a warm fire palette:
		// Low: Deep Crimson/Red
		// Mid: Bright Orange
		// High: Yellow / Golden White
		var red, green, blue float64
		if intensity < 0.35 {
			factor := intensity / 0.35
			red = lerp(120, 245, factor)
			green = lerp(0, 35, factor)
			blue = 0
		} else if intensity < 0.75 {
			factor := (intensity - 0.35) / 0.40
			red = lerp(245, 255, factor)
			green = lerp(35, 150, factor)
			blue = lerp(0, 10, factor)
		} else {
			factor := (intensity - 0.75) / 0.25
			red = 255
			green = lerp(150, 230, factor)
			blue = lerp(10, 140, factor)
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
