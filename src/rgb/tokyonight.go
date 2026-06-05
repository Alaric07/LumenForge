package rgb

import (
	"math"
	"time"
)

// TokyoNight simulates a rainy night in Tokyo: dark blue-gray base,
// neon lavender/pink billboard shimmers, and cyan rain ripples.
func (r *ActiveRGB) TokyoNight(startTime *time.Time) {
	elapsed := time.Since(*startTime).Milliseconds()
	tSeconds := float64(elapsed) / 1000.0
	tScaled := tSeconds * r.RgbModeSpeed

	// Each time slot is 0.60 seconds long for rain drop detection
	slotDuration := 0.60
	currentSlot := math.Floor(tScaled / slotDuration)

	buf := map[int][]byte{}
	for j := 0; j < r.LightChannels; j++ {
		pos := float64(j) / float64(r.LightChannels)

		// 1. Base Night Sky (Deep Blue-Gray)
		breath := math.Sin(tScaled*0.8 + pos*3.0)
		red := 22.0 + 8.0*breath
		green := 22.0 + 4.0*math.Cos(tScaled*0.6)
		blue := 34.0 + 12.0*breath

		// 2. Distant Neon Billboard Shimmer (Pink / Lavender / Orange)
		pinkShimmer := math.Max(0, math.Sin(tScaled*0.3+pos*1.5)) * 40.0
		orangeShimmer := math.Max(0, math.Cos(tScaled*0.2-pos*2.0)) * 25.0

		red += pinkShimmer + orangeShimmer
		green += pinkShimmer*0.4 + orangeShimmer*0.6
		blue += pinkShimmer*0.5 + orangeShimmer*0.4

		// 3. Rain Puddle Ripples
		for slotOffset := -1; slotOffset <= 0; slotOffset++ {
			slot := currentSlot + float64(slotOffset)

			// 50% chance of a rain drop hitting in this slot
			if random01(slot, 21.0) > 0.5 {
				center := random01(slot, 22.0)
				startOffset := random01(slot, 23.0) * 0.15
				duration := 0.25 + random01(slot, 24.0)*0.2 // 0.25s to 0.45s
				speed := 2.0 + random01(slot, 25.0)*1.5

				age := tScaled - (slot*slotDuration + startOffset)
				if age >= 0 && age < duration {
					progress := age / duration
					radius := age * speed
					dist := math.Abs(pos - center)
					thickness := 0.12

					if dist < radius && dist > (radius-thickness) {
						waveIntensity := (1.0 - progress) * (1.0 - (radius-dist)/thickness)
						waveIntensity = clampFloat01(waveIntensity)

						// Bright cyan raindrop ripple
						rRain, gRain, bRain := 125.0, 207.0, 255.0

						red = lerp(red, rRain, waveIntensity)
						green = lerp(green, gRain, waveIntensity)
						blue = lerp(blue, bRain, waveIntensity)
					}
				}
			}
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

	r.Raw = buf

	if r.Inverted {
		r.Output = SetColorInverted(buf)
	} else {
		r.Output = SetColor(buf)
	}
}
