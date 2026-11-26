package visual

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestDiffIdentical(t *testing.T) {
	a := encodePNG(t, solid(20, 20, color.RGBA{255, 0, 0, 255}))
	d, err := DiffPercent(a, a)
	if err != nil {
		t.Fatal(err)
	}
	if d != 0 {
		t.Fatalf("expected 0%% diff, got %f", d)
	}
}

func TestDiffAllChanged(t *testing.T) {
	a := encodePNG(t, solid(20, 20, color.RGBA{255, 0, 0, 255}))
	b := encodePNG(t, solid(20, 20, color.RGBA{0, 0, 255, 255}))
	d, err := DiffPercent(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if d != 100 {
		t.Fatalf("expected 100%% diff, got %f", d)
	}
}

func TestDiffPartial(t *testing.T) {
	img := solid(20, 20, color.RGBA{255, 0, 0, 255})
	// repaint a 10x10 quarter black → 25% of pixels differ
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{0, 0, 0, 255})
		}
	}
	a := encodePNG(t, solid(20, 20, color.RGBA{255, 0, 0, 255}))
	b := encodePNG(t, img)
	d, err := DiffPercent(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if d < 24 || d > 26 {
		t.Fatalf("expected ~25%% diff, got %f", d)
	}
}

func TestDiffInvalidImage(t *testing.T) {
	if _, err := DiffPercent([]byte("not an image"), []byte("nope")); err == nil {
		t.Fatal("expected error for invalid image")
	}
}
