package rgb

import (
	"math"
	"time"
)

// Stardust runs a calm, ambient starfield effect with slow, drifting virtual particles
func (r *ActiveRGB) Stardust(startTime *time.Time) {
	if r.LightChannels <= 0 {
		return
	}

	buf := map[int][]byte{}
	elapsed := time.Since(*startTime).Milliseconds()
	timeSec := float64(elapsed) / 1000.0

	// Particle count and width configuration scaled to cluster length
	particleCount := int(math.Max(4, math.Min(18, float64(r.LightChannels)/20.0)))
	particleWidth := math.Max(0.45, math.Min(1.2, float64(r.LightChannels)/300.0))

	// Base glow configuration (12% background intensity on StartColor)
	baseFactor := 0.12
	baseR := r.RGBStartColor.Red * baseFactor
	baseG := r.RGBStartColor.Green * baseFactor
	baseB := r.RGBStartColor.Blue * baseFactor

	starR := r.RGBEndColor.Red
	starG := r.RGBEndColor.Green
	starB := r.RGBEndColor.Blue

	// For each LED, track the max particle intensity contribution
	intensities := make([]float64, r.LightChannels)

	// Drift speed configuration (lower speed value in profile means faster animation)
	driftBase := 2.0 / r.RgbModeSpeed
	// Fade speed configuration for smooth twinkling
	fadeBase := 0.3 / r.RgbModeSpeed

	// Simulate each virtual particle
	for p := 0; p < particleCount; p++ {
		// Generate deterministic hashes for this particle
		hSeed := packetHash(p)
		hPos := packetHash(p + 3)
		hSpeed := packetHash(p + 7)
		hPhase := packetHash(p + 11)
		hFadeSpeed := packetHash(p + 19)

		// 1. Initial starting position
		startPos := hPos * float64(r.LightChannels)

		// 2. Drift speed and direction (slowly drift)
		// We make some drift left and some right based on hSeed
		direction := 1.0
		if hSeed < 0.5 {
			direction = -1.0
		}
		driftSpeed := direction * driftBase * (0.5 + hSpeed*0.8)

		// 3. Current position wrapped around the cluster size
		currPos := startPos + timeSec*driftSpeed
		currPos = math.Mod(currPos, float64(r.LightChannels))
		if currPos < 0 {
			currPos += float64(r.LightChannels)
		}

		// 4. Fade envelope (shaped to spend more time dim/off and only briefly peak)
		fadeSpeed := fadeBase * (0.6 + hFadeSpeed*0.8)
		phaseOffset := hPhase * 2.0 * math.Pi
		rawFade := 0.5 + 0.5*math.Sin(timeSec*fadeSpeed*2.0*math.Pi+phaseOffset)
		fade := math.Pow(rawFade, 4.0)

		// 5. Apply particle contribution to each LED
		for i := 0; i < r.LightChannels; i++ {
			// Wrap-around distance calculation
			diff := math.Abs(float64(i) - currPos)
			if diff > float64(r.LightChannels)/2.0 {
				diff = float64(r.LightChannels) - diff
			}

			// Gaussian shape
			expDist := math.Exp(-(diff * diff) / (2.0 * particleWidth * particleWidth))
			particleIntensity := fade * expDist * 0.75

			if particleIntensity > intensities[i] {
				intensities[i] = particleIntensity
			}
		}
	}

	// Render the colors based on calculated intensities
	for i := 0; i < r.LightChannels; i++ {
		intensity := math.Min(intensities[i], 1.0)

		rValFloat := (baseR + starR*intensity) * r.RGBBrightness
		gValFloat := (baseG + starG*intensity) * r.RGBBrightness
		bValFloat := (baseB + starB*intensity) * r.RGBBrightness

		rVal := math.Max(0, math.Min(255, rValFloat))
		gVal := math.Max(0, math.Min(255, gValFloat))
		bVal := math.Max(0, math.Min(255, bValFloat))

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
