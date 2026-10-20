package main

import (
	"bytes"
	"image"
	"image/color"
	stddraw "image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// renderedImage is a top-down BGRA buffer ready for StretchDIBits.
type renderedImage struct {
	W, H int
	Pix  []byte
}

const maxRenderedSide = 1800

func decodeRenderedImage(data []byte) (*renderedImage, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	if b.Empty() {
		return nil, image.ErrFormat
	}
	w, h := b.Dx(), b.Dy()
	if w > maxRenderedSide || h > maxRenderedSide {
		scale := float64(maxRenderedSide) / float64(w)
		if hs := float64(maxRenderedSide) / float64(h); hs < scale {
			scale = hs
		}
		w = int(float64(w)*scale + 0.5)
		h = int(float64(h)*scale + 0.5)
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	// Composite transparent PNG/WebP pixels over paper white instead of
	// letting GDI render them as black.
	stddraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.RGBA{R: 255, G: 255, B: 255, A: 255}}, image.Point{}, stddraw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, stddraw.Over, nil)

	r := &renderedImage{W: w, H: h, Pix: make([]byte, w*h*4)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := dst.RGBAAt(x, y)
			i := (y*w + x) * 4
			r.Pix[i] = c.B
			r.Pix[i+1] = c.G
			r.Pix[i+2] = c.R
			r.Pix[i+3] = 255
		}
	}
	return r, nil
}
