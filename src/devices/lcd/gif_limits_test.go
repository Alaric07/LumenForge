package lcd

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLCDGIFPreflightLimits(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "frame count",
			data: compactGIFForTest(1, 1, 1, 1, maxLCDGIFFrames+1),
			want: "frame count",
		},
		{
			name: "logical width",
			data: compactGIFForTest(maxLCDGIFLogicalDimension+1, 1, 1, 1, 1),
			want: "logical dimensions",
		},
		{
			name: "aggregate pixel work",
			data: compactGIFForTest(1024, 1024, 1024, 1024, maxLCDGIFFrames),
			want: "pixel work",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if len(test.data) >= 5*1024*1024 {
				t.Fatalf("hostile fixture size = %d, want below upload limit", len(test.data))
			}
			if _, err := preflightLCDGIF(bytes.NewReader(test.data)); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("preflightLCDGIF() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLCDGIFPreflightRejectsTruncatedData(t *testing.T) {
	valid := compactGIFForTest(1, 1, 1, 1, 1)
	for _, data := range [][]byte{
		valid[:5],
		valid[:len(valid)-2],
	} {
		if _, err := preflightLCDGIF(bytes.NewReader(data)); err == nil {
			t.Fatalf("preflightLCDGIF() accepted truncated data of %d bytes", len(data))
		}
	}
}

func TestLCDGIFPreflightAcceptsBoundaryMetadata(t *testing.T) {
	dimensionBoundary := compactGIFForTest(
		maxLCDGIFLogicalDimension,
		maxLCDGIFLogicalDimension,
		1,
		1,
		1,
	)
	metadata, err := preflightLCDGIF(bytes.NewReader(dimensionBoundary))
	if err != nil {
		t.Fatalf("dimension boundary preflight: %v", err)
	}
	if metadata.Width != maxLCDGIFLogicalDimension || metadata.Height != maxLCDGIFLogicalDimension || metadata.Frames != 1 {
		t.Fatalf("dimension boundary metadata = %#v", metadata)
	}

	frameBoundary := compactGIFForTest(1, 1, 1, 1, maxLCDGIFFrames)
	metadata, err = preflightLCDGIF(bytes.NewReader(frameBoundary))
	if err != nil {
		t.Fatalf("frame boundary preflight: %v", err)
	}
	if metadata.Frames != maxLCDGIFFrames {
		t.Fatalf("frame boundary metadata = %#v", metadata)
	}
}

func TestShippedLCDGIFRemainsWithinLimits(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "..", "database", "lcd", "images", "concentric.gif"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()

	metadata, err := preflightLCDGIF(file)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Width != 480 || metadata.Height != 480 || metadata.Frames != 60 {
		t.Fatalf("shipped GIF metadata = %#v", metadata)
	}
}

func TestDecodeLCDGIFPreservesFramesDelaysAndOutputSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "normal.gif")
	writeDependentLCDGIFForTest(t, path)
	useLCDOutputSizeForTest(t, 480, 480)

	data, err := decodeLCDImage(path, "normal", ImageFormatGif)
	if err != nil {
		t.Fatal(err)
	}
	if data.Frames != 2 || len(data.Buffer) != 2 || len(data.PalettedFrames) != 2 {
		t.Fatalf("decoded animation = %#v", data)
	}
	if data.Buffer[0].Delay != 40 || data.Buffer[1].Delay != 70 {
		t.Fatalf("decoded delays = %v, %v", data.Buffer[0].Delay, data.Buffer[1].Delay)
	}
	for index, frame := range data.PalettedFrames {
		if frame.Bounds().Dx() != 480 || frame.Bounds().Dy() != 480 {
			t.Fatalf("frame %d bounds = %v", index, frame.Bounds())
		}
	}
}

func TestDecodeLCDGIFPreservesDependentFrameComposition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dependent.gif")
	writeDependentLCDGIFForTest(t, path)
	useLCDOutputSizeForTest(t, 2, 1)

	data, err := decodeLCDImage(path, "dependent", ImageFormatGif)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.PalettedFrames) != 2 {
		t.Fatalf("decoded frame count = %d", len(data.PalettedFrames))
	}
	second := data.PalettedFrames[1]
	left := color.RGBAModel.Convert(second.At(0, 0)).(color.RGBA)
	right := color.RGBAModel.Convert(second.At(1, 0)).(color.RGBA)
	if left.R <= left.B || right.B <= right.R {
		t.Fatalf("dependent frame colors = left %#v, right %#v", left, right)
	}
}

func TestOverBudgetLCDGIFTransactionPreservesStateAndRemovesStage(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "shared.gif")
	temp, err := os.CreateTemp(root, ".shared-upload-*")
	if err != nil {
		t.Fatal(err)
	}
	tempPath := temp.Name()
	if _, err = temp.Write(compactGIFForTest(1, 1, 1, 1, maxLCDGIFFrames+1)); err != nil {
		_ = temp.Close()
		t.Fatal(err)
	}
	if err = temp.Close(); err != nil {
		t.Fatal(err)
	}
	preserveLCDTransactionState(t, []ImageData{{Name: "shared", Frames: 7}}, map[string][]AnimationFrames{
		"shared": {{Delay: 99}},
	})

	err = transactMutableLCDUpload(
		root,
		"shared",
		tempPath,
		destination,
		ImageFormatGif,
		defaultLCDUploadTransactionOps(),
	)
	if err == nil || !strings.Contains(err.Error(), "frame count") {
		t.Fatalf("over-budget transaction error = %v", err)
	}
	if _, statErr := os.Lstat(tempPath); !os.IsNotExist(statErr) {
		t.Fatalf("staged over-budget GIF remains: %v", statErr)
	}
	if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("over-budget GIF was installed: %v", statErr)
	}
	assertLCDLiveStateUnchanged(t, "shared")
	assertNoLCDTransactionArtifacts(t, root)
}

func compactGIFForTest(logicalWidth, logicalHeight, frameWidth, frameHeight, frames int) []byte {
	data := []byte{'G', 'I', 'F', '8', '9', 'a'}
	data = appendGIFUint16ForTest(data, logicalWidth)
	data = appendGIFUint16ForTest(data, logicalHeight)
	data = append(data,
		0x80, 0x00, 0x00,
		0x00, 0x00, 0x00,
		0xff, 0xff, 0xff,
	)
	for index := 0; index < frames; index++ {
		data = append(data, 0x2c, 0x00, 0x00, 0x00, 0x00)
		data = appendGIFUint16ForTest(data, frameWidth)
		data = appendGIFUint16ForTest(data, frameHeight)
		data = append(data,
			0x00,
			0x02,
			0x02, 0x44, 0x01,
			0x00,
		)
	}
	return append(data, 0x3b)
}

func appendGIFUint16ForTest(data []byte, value int) []byte {
	return append(data, byte(value), byte(value>>8))
}

func writeDependentLCDGIFForTest(t *testing.T, path string) {
	t.Helper()
	palette := color.Palette{
		color.RGBA{},
		color.RGBA{R: 255, A: 255},
		color.RGBA{B: 255, A: 255},
	}
	first := image.NewPaletted(image.Rect(0, 0, 2, 1), palette)
	first.Pix[0] = 1
	first.Pix[1] = 1
	second := image.NewPaletted(image.Rect(1, 0, 2, 1), palette)
	second.Pix[0] = 2

	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = gif.EncodeAll(file, &gif.GIF{
		Image:    []*image.Paletted{first, second},
		Delay:    []int{4, 7},
		Disposal: []byte{gif.DisposalNone, gif.DisposalNone},
	}); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
}

func useLCDOutputSizeForTest(t *testing.T, width, height int) {
	t.Helper()
	mutex.Lock()
	originalWidth := imgWidth
	originalHeight := imgHeight
	imgWidth = width
	imgHeight = height
	mutex.Unlock()
	t.Cleanup(func() {
		mutex.Lock()
		imgWidth = originalWidth
		imgHeight = originalHeight
		mutex.Unlock()
	})
}
