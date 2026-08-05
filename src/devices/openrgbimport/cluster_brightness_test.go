package openrgbimport

import (
	"LumenForge/src/rgb"
	"bytes"
	"net"
	"testing"
)

func captureOpenRGBClusterFrames(t *testing.T) *[][]byte {
	t.Helper()

	previous := sendClusterFrame
	frames := make([][]byte, 0)
	sendClusterFrame = func(_ net.Conn, _ uint32, frame []byte) (net.Conn, error) {
		frames = append(frames, append([]byte(nil), frame...))
		return nil, nil
	}
	t.Cleanup(func() {
		sendClusterFrame = previous
	})
	return &frames
}

func TestOpenRGBClusterOutputIgnoresMemberBrightness(t *testing.T) {
	frames := captureOpenRGBClusterFrames(t)
	completedClusterFrame := []byte{200, 100, 50, 40, 20, 10}
	device := newLightingMutationDevice()
	device.colorCount = 2
	device.DeviceProfile.RGBCluster = true

	for _, brightness := range []uint8{0, 40, 100, 25} {
		device.brightness = brightness
		device.DeviceProfile.BrightnessSlider = &brightness
		input := append([]byte(nil), completedClusterFrame...)

		device.writeColorCluster(input, 0)

		if len(*frames) == 0 {
			t.Fatalf("member brightness %d produced no cluster frame", brightness)
		}
		got := (*frames)[len(*frames)-1]
		if !bytes.Equal(got, completedClusterFrame) {
			t.Fatalf("cluster frame with member brightness %d = %v, want %v", brightness, got, completedClusterFrame)
		}
		if !bytes.Equal(input, completedClusterFrame) {
			t.Fatalf("cluster callback mutated its input at member brightness %d: got %v, want %v", brightness, input, completedClusterFrame)
		}
	}

	if len(*frames) != 4 {
		t.Fatalf("cluster frame count = %d, want 4", len(*frames))
	}
}

func TestOpenRGBIndependentOutputAppliesLocalBrightnessOnce(t *testing.T) {
	tests := []struct {
		name       string
		brightness uint8
		want       []byte
	}{
		{name: "black", brightness: 0, want: []byte{0, 0, 0}},
		{name: "intermediate", brightness: 50, want: []byte{100, 50, 25}},
		{name: "maximum", brightness: 100, want: []byte{200, 100, 50}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, calls := installLightingDeviceTestSeams(t)
			device := newLightingMutationDevice()
			device.brightness = test.brightness
			device.DeviceProfile.BrightnessSlider = &test.brightness
			setCanonicalStaticColor(t, device, rgb.Color{Red: 200, Green: 100, Blue: 50})
			if err := device.SetEffect("static"); err != nil {
				t.Fatalf("SetEffect(static) at %d%%: %v", test.brightness, err)
			}
			if calls.colors != 0 || calls.frames != 1 {
				t.Fatalf("output calls at %d%% = colors %d, frames %d, want 0, 1", test.brightness, calls.colors, calls.frames)
			}
			if len(calls.frameValues) != 1 || !bytes.Equal(calls.frameValues[0], test.want) {
				t.Fatalf("independent output at %d%% = %v, want %v", test.brightness, calls.frameValues, test.want)
			}
		})
	}
}

func TestOpenRGBLeavingClusterRestoresStoredLocalBrightness(t *testing.T) {
	_, calls := installLightingDeviceTestSeams(t)
	frames := captureOpenRGBClusterFrames(t)
	brightness := uint8(40)
	device := newLightingMutationDevice()
	device.brightness = brightness
	device.effect = ""
	device.DeviceProfile.BrightnessSlider = &brightness
	device.DeviceProfile.RGBCluster = true

	device.writeColorCluster([]byte{200, 100, 50}, 0)
	if len(*frames) != 1 || !bytes.Equal((*frames)[0], []byte{200, 100, 50}) {
		t.Fatalf("cluster-owned output = %v, want unmodified [200 100 50]", *frames)
	}

	device.mu.Lock()
	device.DeviceProfile.RGBCluster = false
	device.mu.Unlock()
	if device.DeviceProfile.RGBCluster {
		t.Fatal("device remained cluster-controlled")
	}
	if device.brightness != brightness || device.DeviceProfile.BrightnessSlider == nil || *device.DeviceProfile.BrightnessSlider != brightness {
		t.Fatalf("stored local brightness after leaving cluster = device %d, profile %#v, want %d", device.brightness, device.DeviceProfile.BrightnessSlider, brightness)
	}

	setCanonicalStaticColor(t, device, rgb.Color{Red: 200, Green: 100, Blue: 50})
	if err := device.SetEffect("static"); err != nil {
		t.Fatalf("independent SetEffect(static) after leaving cluster: %v", err)
	}
	if calls.colors != 0 || calls.frames != 1 || len(calls.frameValues) != 1 {
		t.Fatalf("independent output calls = colors %d, frames %d, values %v", calls.colors, calls.frames, calls.frameValues)
	}
	if want := []byte{80, 40, 20}; !bytes.Equal(calls.frameValues[0], want) {
		t.Fatalf("independent output after leaving cluster = %v, want %v", calls.frameValues[0], want)
	}
}
