package rgb

import (
	"math"
	"time"
)

// packetHash generates a deterministic pseudo-random float64 in [0.0, 1.0) based on packet index
func packetHash(idx int) float64 {
	u := uint32(idx)
	if idx < 0 {
		u = uint32(-idx)
	}
	u = ((u >> 16) ^ u) * 0x45d9f3b
	u = ((u >> 16) ^ u) * 0x45d9f3b
	u = (u >> 16) ^ u
	return float64(u%1000) / 1000.0
}

// DataStream runs moving data packets with short trails and deterministic pseudo-random spacing
func (r *ActiveRGB) DataStream(startTime *time.Time) {
	if r.LightChannels <= 0 {
		return
	}

	buf := map[int][]byte{}
	elapsed := time.Since(*startTime).Milliseconds()

	baseSpeed := math.Max(35.0, float64(r.LightChannels)/4.0)
	ledsPerSecond := baseSpeed / r.RgbModeSpeed

	// Position offset of the stream (0..r.LightChannels)
	pos := math.Mod(float64(elapsed)/1000.0*ledsPerSecond, float64(r.LightChannels))

	packetSpacing := math.Max(12.0, math.Min(40.0, float64(r.LightChannels)/8.0))
	trailLength := math.Max(4.0, math.Min(12.0, packetSpacing/3.0))

	for i := 0; i < r.LightChannels; i++ {
		offset := float64(i) - pos
		packetIdx := int(math.Floor(offset / packetSpacing))

		distFromHead := math.Mod(offset, packetSpacing)
		if distFromHead > 0 {
			distFromHead -= packetSpacing
		}

		intensity := 0.0
		var color Color

		// Deterministic pseudo-random variation per packet
		hash := packetHash(packetIdx)
		if hash >= 0.25 { // 75% active packets
			effectiveTrail := trailLength * (0.6 + 0.4*packetHash(packetIdx+7))

			if distFromHead >= -effectiveTrail {
				// Normalized position along the trail (0.0 at head, 1.0 at trail end)
				t := distFromHead / -effectiveTrail

				// Exponential falloff
				intensity = math.Exp(distFromHead / (effectiveTrail / 2.5))

				color = interpolateColors(r.RGBStartColor, r.RGBEndColor, t, r.RGBBrightness)
			}
		}

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
