package rgb

import (
	"math"
	"math/rand"
	"time"
)

func flickeringRandomUpperBound(lightChannels int, rgbModeSpeed float64) int {
	randomUpperBound := lightChannels * int(rgbModeSpeed)
	if randomUpperBound < 2 {
		return 2
	}
	return randomUpperBound
}

// Flickering will run RGB function
func (r *ActiveRGB) Flickering(startTime *time.Time) {
	elapsed := time.Since(*startTime).Milliseconds()
	progress := math.Mod(float64(elapsed)/(r.RgbModeSpeed*1000), 1.0)

	if progress >= 1.0 {
		*startTime = time.Now() // Reset startTime to the current time
		elapsed = 0             // Reset elapsed time
		progress = 0            // Reset progress
	}

	buf := map[int][]byte{}
	randomUpperBound := flickeringRandomUpperBound(r.LightChannels, r.RgbModeSpeed)
	for j := 0; j < r.LightChannels; j++ {
		t := float64(j) / float64(r.LightChannels) // Calculate interpolation factor
		colors := interpolateColors(r.RGBStartColor, r.RGBEndColor, t, r.RGBBrightness)
		flicker := rand.Intn(randomUpperBound) == 1
		if len(r.Buffer) > 0 {
			if flicker {
				r.Buffer[j] = 0
				r.Buffer[j+r.ColorOffset] = 0
				r.Buffer[j+(r.ColorOffset*2)] = 0
			} else {
				r.Buffer[j] = byte(colors.Red)
				r.Buffer[j+r.ColorOffset] = byte(colors.Green)
				r.Buffer[j+(r.ColorOffset*2)] = byte(colors.Blue)
			}
		} else {
			if flicker {
				buf[j] = []byte{0, 0, 0}
			} else {
				buf[j] = []byte{
					byte(colors.Red),
					byte(colors.Green),
					byte(colors.Blue),
				}
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
