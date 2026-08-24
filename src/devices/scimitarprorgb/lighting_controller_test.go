package scimitarprorgb

import (
	"bytes"
	"testing"

	"LumenForge/src/rgb"
)

func TestScimitarLightingControllerWritesLocalFrame(t *testing.T) {
	zoneColors, dpi := scimitarTestFrameLayout()
	adapter := newScimitarLightingAdapter(scimitarTestLEDChannels, zoneColors, dpi)
	var activeRGB *rgb.ActiveRGB
	var writtenFrames [][]byte
	controller := newScimitarLightingController(
		adapter,
		scimitarLightingLocal,
		func(frame []byte) {
			writtenFrames = append(writtenFrames, append([]byte(nil), frame...))
		},
		&activeRGB,
	)
	logicalFrame := composeScimitarLogicalFrame([]byte{
		0x11, 0x12, 0x13,
		0x21, 0x22, 0x23,
		0x31, 0x32, 0x33,
		0x41, 0x42, 0x43,
	})

	if !controller.writeLocalFrame(logicalFrame, rgb.Color{Red: 0x51, Green: 0x52, Blue: 0x53}) {
		t.Fatal("local controller rejected a local logical frame")
	}
	if len(writtenFrames) != 1 {
		t.Fatalf("local write count = %d, want 1", len(writtenFrames))
	}
	want := []byte{
		1, 0x11, 0x12, 0x13,
		2, 0x41, 0x42, 0x43,
		3, 0x51, 0x52, 0x53,
		4, 0x21, 0x22, 0x23,
		5, 0x31, 0x32, 0x33,
	}
	if !bytes.Equal(writtenFrames[0], want) {
		t.Fatalf("controller hardware frame = %v, want %v", writtenFrames[0], want)
	}
}

func TestScimitarLightingControllerClusterOwnershipBypassesLocalOutput(t *testing.T) {
	zoneColors, dpi := scimitarTestFrameLayout()
	adapter := newScimitarLightingAdapter(scimitarTestLEDChannels, zoneColors, dpi)
	var activeRGB *rgb.ActiveRGB
	writes := 0
	controller := newScimitarLightingController(
		adapter,
		scimitarLightingCluster,
		func([]byte) { writes++ },
		&activeRGB,
	)

	if controller.writeLocalFrame(
		composeScimitarLogicalFrame([]byte{1, 2, 3}),
		rgb.Color{Red: 4, Green: 5, Blue: 6},
	) {
		t.Fatal("Cluster-owned controller accepted local output")
	}
	if writes != 0 {
		t.Fatalf("Cluster-owned local write count = %d, want 0", writes)
	}
}

func TestScimitarLightingControllerStopAndRestart(t *testing.T) {
	zoneColors, dpi := scimitarTestFrameLayout()
	adapter := newScimitarLightingAdapter(scimitarTestLEDChannels, zoneColors, dpi)
	var activeRGB *rgb.ActiveRGB
	var writtenFrames [][]byte
	controller := newScimitarLightingController(
		adapter,
		scimitarLightingLocal,
		func(frame []byte) {
			writtenFrames = append(writtenFrames, append([]byte(nil), frame...))
		},
		&activeRGB,
	)
	logicalFrame := composeScimitarLogicalFrame([]byte{
		0x11, 0x12, 0x13,
		0x21, 0x22, 0x23,
		0x31, 0x32, 0x33,
		0x41, 0x42, 0x43,
	})
	dpiColor := rgb.Color{Red: 0x51, Green: 0x52, Blue: 0x53}

	first := controller.start()
	if first == nil || activeRGB != first {
		t.Fatalf("first active RGB = %p, stored = %p", first, activeRGB)
	}
	controller.writeLocalFrame(logicalFrame, dpiColor)
	stopScimitarControllerForTest(t, controller, first)
	if activeRGB != nil {
		t.Fatalf("active RGB after stop = %p, want nil", activeRGB)
	}

	second := controller.start()
	if second == nil || second == first || activeRGB != second {
		t.Fatalf("restarted active RGB = %p, first = %p, stored = %p", second, first, activeRGB)
	}
	controller.writeLocalFrame(logicalFrame, dpiColor)
	stopScimitarControllerForTest(t, controller, second)

	if len(writtenFrames) != 2 {
		t.Fatalf("local write count across restart = %d, want 2", len(writtenFrames))
	}
	if !bytes.Equal(writtenFrames[0], writtenFrames[1]) {
		t.Fatalf("frame after restart = %v, want %v", writtenFrames[1], writtenFrames[0])
	}
}

func stopScimitarControllerForTest(
	t *testing.T,
	controller scimitarLightingController,
	activeRGB *rgb.ActiveRGB,
) {
	t.Helper()
	ready := make(chan struct{})
	stopped := make(chan bool, 1)
	go func() {
		close(ready)
		stopped <- <-activeRGB.Exit
	}()
	<-ready
	controller.stop()
	if value := <-stopped; !value {
		t.Fatal("controller stop value = false, want true")
	}
}
