package rgb

import (
	"math"
	"time"
)

// PlasmaCore runs repeating smooth energy pulse bands across the cluster
func (r *ActiveRGB) PlasmaCore(startTime *time.Time) {
	if r.LightChannels <= 0 {
		return
	}

	buf := map[int][]byte{}
	elapsed := time.Since(*startTime).Milliseconds()

	baseSpeed := math.Max(35.0, float64(r.LightChannels)/4.0)
	ledsPerSecond := baseSpeed / r.RgbModeSpeed

	// Normalized phase (0.0 to 1.0)
	phase := math.Mod((float64(elapsed)/1000.0*ledsPerSecond)/float64(r.LightChannels), 1.0)

	bandCount := math.Max(3.0, math.Min(8.0, float64(r.LightChannels)/50.0))
	pulseWidth := 0.08

	for i := 0; i < r.LightChannels; i++ {
		// Normalized position of LED i
		pos := float64(i) / float64(r.LightChannels)

		// periodic position relative to the bands
		x := (pos - phase) * bandCount
		fraction := math.Mod(x, 1.0)
		if fraction < 0 {
			fraction += 1.0
		}

		// Distance to the center of the nearest band (0.0 to 0.5)
		dist := fraction
		if dist > 0.5 {
			dist = 1.0 - dist
		}

		// Smooth falloff (cosine mask)
		intensity := 0.0
		if dist < pulseWidth {
			intensity = 0.5 * (1.0 + math.Cos(math.Pi*dist/pulseWidth))
		}

		// Blend StartColor (base glow) towards EndColor (pulse color) based on intensity
		color := interpolateColors(r.RGBStartColor, r.RGBEndColor, intensity, r.RGBBrightness)

		// Add a subtle white peak at the center of the pulse
		if intensity > 0.8 {
			whiteT := ((intensity - 0.8) / 0.2) * 0.6 // Subtle white-hot peak (max 60% white)
			color.Red = color.Red*(1.0-whiteT) + 255.0*whiteT
			color.Green = color.Green*(1.0-whiteT) + 255.0*whiteT
			color.Blue = color.Blue*(1.0-whiteT) + 255.0*whiteT
		}

		// Apply final brightness
		rVal := color.Red
		gVal := color.Green
		bVal := color.Blue

		if len(r.Buffer) > 0 {
			r.Buffer[i] = byte(rVal)
			r.Buffer[i+r.ColorOffset] = byte(gVal)
			r.Buffer[i+(r.ColorOffset*2)] = byte(bVal)
		} else {
			buf[i] = []byte{byte(rVal), byte(gVal), byte(bVal)}
			if r.IsAIO && r.HasLCD {
				if i > 15 && i < 20 {
					buf[i] = []byte{0, 0, 0}
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
