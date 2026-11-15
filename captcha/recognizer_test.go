package captcha

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"testing"
)

func buildDummyMCNNv2(t *testing.T, alphabet string) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString("MCNN")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(2))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(72))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(27))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(4))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(28))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(150))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(40))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(alphabet)))
	buf.WriteString(alphabet)
	for _, span := range [][2]uint32{{5, 18}, {17, 30}, {29, 42}, {41, 54}} {
		_ = binary.Write(&buf, binary.LittleEndian, span[0])
		_ = binary.Write(&buf, binary.LittleEndian, span[1])
	}
	_ = binary.Write(&buf, binary.LittleEndian, uint32(conv1Filters))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(conv2Filters))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(denseUnits))
	// Weights: conv1W, conv1B, conv2W, conv2B, fc1W, fc1B, fc2W, fc2B
	floatCount := conv1Filters*9 + conv1Filters +
		conv2Filters*conv1Filters*9 + conv2Filters +
		denseUnits*flat + denseUnits +
		len(alphabet)*denseUnits + len(alphabet)
	zeros := make([]float32, floatCount)
	// Bias class 1 ("b") slightly higher so all zero-weights predict 'b'.
	zeros[floatCount-len(alphabet)+1] = 1.0
	for _, f := range zeros {
		_ = binary.Write(&buf, binary.LittleEndian, f)
	}
	return buf.Bytes()
}

func TestLoadAndRecognizeGIF(t *testing.T) {
	modelPath := filepath.Join(t.TempDir(), "test.bin")
	if err := os.WriteFile(modelPath, buildDummyMCNNv2(t, "ab"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec, err := Load(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	w, h := rec.Size()
	if w != 72 || h != 27 {
		t.Fatalf("got size %dx%d, want 72x27", w, h)
	}

	// Draw 4 dark-blue vertical bars inside a 72x27 GIF.
	pal := color.Palette{color.White, color.RGBA{0, 0, 153, 255}}
	img := image.NewPaletted(image.Rect(0, 0, 72, 27), pal)
	for _, x := range []int{10, 22, 34, 46} {
		for y := 6; y < 20; y++ {
			img.SetColorIndex(x, y, 1)
		}
	}
	var gifBuf bytes.Buffer
	if err := gif.Encode(&gifBuf, img, nil); err != nil {
		t.Fatal(err)
	}
	got, err := rec.Recognize(gifBuf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got != "bbbb" {
		t.Fatalf("got %q, want bbbb", got)
	}
}
