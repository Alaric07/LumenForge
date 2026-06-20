package rgb

import (
	"math"
	"time"
)

// Comet runs a single-direction wrap-around sweep with an exponential decay tail
func (r *ActiveRGB) Comet(startTime *time.Time) {
	buf := map[int][]byte{}
	elapsed := time.Since(*startTime).Milliseconds()

	baseSpeed := math.Max(35.0, float64(r.LightChannels)/4.0)
	ledsPerSecond := baseSpeed / r.RgbModeSpeed

	// Beam position in "LED units" (0..r.LightChannels)
	pos := math.Mod(float64(elapsed)/1000.0*ledsPerSecond, float64(r.LightChannels))

	tailLength := math.Max(8.0, math.Min(40.0, float64(r.LightChannels)/8.0))

	for i := 0; i < r.LightChannels; i++ {
		dist := float64(i) - pos
		if dist > 0 {
			dist -= float64(r.LightChannels)
		}

		// Exponential decay falloff
		intensity := math.Exp(dist / (tailLength / 3.0))

		// Interpolation factor from head (0.0) to tail (1.0)
		t := math.Min(1.0, math.Abs(dist)/tailLength)

		color := interpolateColors(r.RGBStartColor, r.RGBEndColor, t, r.RGBBrightness)

		rVal := color.Red * intensity
		gVal := color.Green * intensity
		bVal := color.Blue * intensity

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
