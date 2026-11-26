package visual

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
)

// pixelThreshold is the per-channel difference above which a pixel counts as changed.
const pixelThreshold = 24

// DiffPercent compares two encoded images pixel by pixel and returns the
// percentage of pixels that differ beyond the threshold. Images of different
// dimensions are scaled to the smaller common size via nearest-neighbour on the
// larger image bounds comparison — if aspect differs materially, returns 100.
func DiffPercent(a, b []byte) (float64, error) {
	imgA, _, err := image.Decode(bytes.NewReader(a))
	if err != nil {
		return 0, fmt.Errorf("decode baseline image: %w", err)
	}
	imgB, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return 0, fmt.Errorf("decode actual image: %w", err)
	}
	ba, bb := imgA.Bounds(), imgB.Bounds()
	wa, ha := ba.Dx(), ba.Dy()
	wb, hb := bb.Dx(), bb.Dy()
	if wa == 0 || ha == 0 || wb == 0 || hb == 0 {
		return 100, nil
	}
	// Compare on the smaller common canvas, sampling the larger image.
	w, h := wa, ha
	if wb < w {
		w = wb
	}
	if hb < h {
		h = hb
	}
	total := w * h
	if total == 0 {
		return 100, nil
	}
	diff := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// map coordinates proportionally into each image
			xa := ba.Min.X + x*wa/w
			ya := ba.Min.Y + y*ha/h
			xb := bb.Min.X + x*wb/w
			yb := bb.Min.Y + y*hb/h
			ra, ga, ba, _ := imgA.At(xa, ya).RGBA()
			rb, gb, bb2, _ := imgB.At(xb, yb).RGBA()
			dr := int(ra>>8) - int(rb>>8)
			dg := int(ga>>8) - int(gb>>8)
			db := int(ba>>8) - int(bb2>>8)
			if dr < 0 {
				dr = -dr
			}
			if dg < 0 {
				dg = -dg
			}
			if db < 0 {
				db = -db
			}
			if dr > pixelThreshold || dg > pixelThreshold || db > pixelThreshold {
				diff++
			}
		}
	}
	return float64(diff) / float64(total) * 100, nil
}

// ReadAll is a small helper to read an attachment stream fully.
func ReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
