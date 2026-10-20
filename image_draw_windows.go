//go:build windows

package main

import "unsafe"

// drawRenderedImage blits a BGRA image into dst, stretching to fit.
func drawRenderedImage(hdc uintptr, img *renderedImage, dst RECT) {
	if hdc == 0 || img == nil || img.W <= 0 || img.H <= 0 || len(img.Pix) < img.W*img.H*4 {
		return
	}
	w, h := rectWidth(dst), rectHeight(dst)
	if w <= 0 || h <= 0 {
		return
	}
	bmi := BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth:       int32(img.W),
		BiHeight:      -int32(img.H),
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: BI_RGB,
		BiSizeImage:   uint32(len(img.Pix)),
	}
	procSetStretchBltMode.Call(hdc, HALFTONE)
	procStretchDIBits.Call(
		hdc,
		uintptr(dst.Left), uintptr(dst.Top), uintptr(w), uintptr(h),
		0, 0, uintptr(img.W), uintptr(img.H),
		uintptr(unsafe.Pointer(&img.Pix[0])),
		uintptr(unsafe.Pointer(&bmi)),
		DIB_RGB_COLORS, SRCCOPY,
	)
}

func fitImageRect(img *renderedImage, area RECT) RECT {
	if img == nil || img.W <= 0 || img.H <= 0 {
		return RECT{}
	}
	aw, ah := rectWidth(area), rectHeight(area)
	if aw <= 0 || ah <= 0 {
		return RECT{}
	}
	scale := float64(aw) / float64(img.W)
	if s := float64(ah) / float64(img.H); s < scale {
		scale = s
	}
	w := int(float64(img.W)*scale + 0.5)
	h := int(float64(img.H)*scale + 0.5)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	x := int(area.Left) + (aw-w)/2
	y := int(area.Top) + (ah-h)/2
	return RECT{int32(x), int32(y), int32(x + w), int32(y + h)}
}
