package cluster

import (
	"bytes"
	"os"
	"testing"
	"time"

	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
)

func TestMain(testingMain *testing.M) {
	restorePaths := config.UsePathsForTest(config.Paths{DefaultLogDestination: "-"})
	logger.Init()
	code := testingMain.Run()
	restorePaths()
	os.Exit(code)
}

func TestClusterStaticBrightnessAppliedOnce(t *testing.T) {
	tests := []struct {
		name       string
		brightness uint8
		want       []byte
	}{
		{name: "black", brightness: 0, want: []byte{0, 0, 0, 0, 0, 0}},
		{name: "intermediate", brightness: 50, want: []byte{100, 50, 25, 100, 50, 25}},
		{name: "maximum", brightness: 100, want: []byte{200, 100, 50, 200, 100, 50}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			device := &Device{}
			profile := rgb.Profile{
				StartColor: rgb.Color{Red: 200, Green: 100, Blue: 50, Brightness: 1},
				EndColor:   rgb.Color{Red: 200, Green: 100, Blue: 50, Brightness: 1},
			}
			startTime := time.Unix(0, 0)
			got := device.generateRgbEffectFromProfile(2, &startTime, "static", profile, test.brightness, rgb.Exit())
			if !bytes.Equal(got, test.want) {
				t.Fatalf("static cluster frame at %d%% = %v, want %v", test.brightness, got, test.want)
			}
		})
	}
}

func TestClusterDistributeColorsCopiesOrderedMemberSegments(t *testing.T) {
	aggregate := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	original := append([]byte(nil), aggregate...)
	var first, second []byte
	device := &Device{Controllers: []*common.ClusterController{
		{
			Serial:      "first",
			LedChannels: 1,
			WriteColorEx: func(data []byte, _ int) {
				first = append([]byte(nil), data...)
				data[0] = 255
			},
		},
		{
			Serial:      "second",
			LedChannels: 2,
			WriteColorEx: func(data []byte, _ int) {
				second = append([]byte(nil), data...)
				data[0] = 254
			},
		},
	}}

	device.distributeColors(aggregate)

	if !bytes.Equal(first, []byte{1, 2, 3}) {
		t.Fatalf("first member segment = %v, want [1 2 3]", first)
	}
	if !bytes.Equal(second, []byte{4, 5, 6, 7, 8, 9}) {
		t.Fatalf("second member segment = %v, want [4 5 6 7 8 9]", second)
	}
	if !bytes.Equal(aggregate, original) {
		t.Fatalf("aggregate frame mutated during dispatch: got %v, want %v", aggregate, original)
	}
}
