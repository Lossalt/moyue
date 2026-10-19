//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const (
	OUT_DEFAULT_PRECIS  = 0
	CLIP_DEFAULT_PRECIS = 0
	NULL_BRUSH_INDEX    = 5
	NULL_PEN_INDEX      = 8
	WHITE_BRUSH_INDEX   = 0
	PS_DOT              = 2
)

func createFont(pt float64, weight int, family string, dpi int) uintptr {
	height := -int(pt*float64(dpi)/72 + 0.5)
	if height == 0 {
		height = -12
	}
	facePtr, face := utf16Ptr(family)
	h, _, _ := procCreateFontW.Call(
		uintptr(int32(height)), 0, 0, 0,
		uintptr(weight), 0, 0, 0,
		DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS,
		CLEARTYPE_QUALITY, DEFAULT_PITCH|FF_DONTCARE,
		facePtr,
	)
	_ = face
	return h
}

func deleteObject(h uintptr) {
	if h != 0 {
		procDeleteObject.Call(h)
	}
}

func withFont(hdc, font uintptr) uintptr {
	if font == 0 {
		return 0
	}
	old, _, _ := procSelectObject.Call(hdc, font)
	return old
}

func fillRect(hdc uintptr, r RECT, color uint32) {
	br, _, _ := procCreateSolidBrush.Call(uintptr(color))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), br)
	procDeleteObject.Call(br)
}

func fillRoundRect(hdc uintptr, r RECT, radius int, color uint32) {
	br, _, _ := procCreateSolidBrush.Call(uintptr(color))
	nullPen, _, _ := procGetStockObject.Call(NULL_PEN_INDEX)
	oldBr, _, _ := procSelectObject.Call(hdc, br)
	oldPen, _, _ := procSelectObject.Call(hdc, nullPen)
	procRoundRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius*2), uintptr(radius*2))
	procSelectObject.Call(hdc, oldBr)
	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(br)
}

func strokeRoundRect(hdc uintptr, r RECT, radius int, color uint32, width int) {
	nullBrush, _, _ := procGetStockObject.Call(NULL_BRUSH_INDEX)
	pen, _, _ := procCreatePen.Call(PS_SOLID, uintptr(width), uintptr(color))
	oldBr, _, _ := procSelectObject.Call(hdc, nullBrush)
	oldPen, _, _ := procSelectObject.Call(hdc, pen)
	procRoundRect.Call(hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius*2), uintptr(radius*2))
	procSelectObject.Call(hdc, oldBr)
	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(pen)
}

func drawLine(hdc uintptr, x1, y1, x2, y2 int, color uint32, width int) {
	pen, _, _ := procCreatePen.Call(PS_SOLID, uintptr(width), uintptr(color))
	old, _, _ := procSelectObject.Call(hdc, pen)
	procMoveToEx.Call(hdc, uintptr(x1), uintptr(y1), 0)
	procLineTo.Call(hdc, uintptr(x2), uintptr(y2))
	procSelectObject.Call(hdc, old)
	procDeleteObject.Call(pen)
}

func drawCircle(hdc uintptr, cx, cy, radius int, color uint32, filled bool) {
	if filled {
		br, _, _ := procCreateSolidBrush.Call(uintptr(color))
		nullPen, _, _ := procGetStockObject.Call(NULL_PEN_INDEX)
		ob, _, _ := procSelectObject.Call(hdc, br)
		op, _, _ := procSelectObject.Call(hdc, nullPen)
		procEllipse.Call(hdc, uintptr(cx-radius), uintptr(cy-radius), uintptr(cx+radius), uintptr(cy+radius))
		procSelectObject.Call(hdc, ob)
		procSelectObject.Call(hdc, op)
		procDeleteObject.Call(br)
		return
	}
	nullBrush, _, _ := procGetStockObject.Call(NULL_BRUSH_INDEX)
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, uintptr(color))
	ob, _, _ := procSelectObject.Call(hdc, nullBrush)
	op, _, _ := procSelectObject.Call(hdc, pen)
	procEllipse.Call(hdc, uintptr(cx-radius), uintptr(cy-radius), uintptr(cx+radius), uintptr(cy+radius))
	procSelectObject.Call(hdc, ob)
	procSelectObject.Call(hdc, op)
	procDeleteObject.Call(pen)
}

func drawText(hdc uintptr, s string, r RECT, flags uint32, font uintptr, color uint32) {
	if s == "" || font == 0 {
		return
	}
	procSetBkMode.Call(hdc, TRANSPARENT)
	procSetTextColor.Call(hdc, uintptr(color))
	old := withFont(hdc, font)
	u := syscall.StringToUTF16(s)
	n := len(u) - 1
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&u[0])), uintptr(n), uintptr(unsafe.Pointer(&r)), uintptr(flags|DT_NOPREFIX))
	if old != 0 {
		procSelectObject.Call(hdc, old)
	}
}

func textWidth(hdc uintptr, s string, font uintptr) int {
	if font == 0 || s == "" {
		return 0
	}
	old := withFont(hdc, font)
	u := syscall.StringToUTF16(s)
	var sz SIZE
	procGetTextExtentPoint32W.Call(hdc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&sz)))
	if old != 0 {
		procSelectObject.Call(hdc, old)
	}
	return int(sz.CX)
}

func trimsToWidth(hdc uintptr, s string, font uintptr, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	if textWidth(hdc, s, font) <= max {
		return s
	}
	runes := []rune(s)
	for i := len(runes) - 1; i > 0; i-- {
		cand := string(runes[:i]) + "…"
		if textWidth(hdc, cand, font) <= max {
			return cand
		}
	}
	return "…"
}

func rectWidth(r RECT) int  { return int(r.Right - r.Left) }
func rectHeight(r RECT) int { return int(r.Bottom - r.Top) }

func insetRect(r RECT, dx, dy int) RECT {
	return RECT{r.Left + int32(dx), r.Top + int32(dy), r.Right - int32(dx), r.Bottom - int32(dy)}
}

func offsetRect(r RECT, dx, dy int) RECT {
	return RECT{r.Left + int32(dx), r.Top + int32(dy), r.Right + int32(dx), r.Bottom + int32(dy)}
}

func pointIn(r RECT, x, y int) bool {
	return int32(x) >= r.Left && int32(x) < r.Right && int32(y) >= r.Top && int32(y) < r.Bottom
}

func centerRect(parent RECT, w, h int) RECT {
	cx := (parent.Left + parent.Right) / 2
	cy := (parent.Top + parent.Bottom) / 2
	return RECT{cx - int32(w/2), cy - int32(h/2), cx + int32(w/2), cy + int32(h/2)}
}

func gradientRect(hdc uintptr, r RECT, top, bottom uint32) {
	// A tiny vertical gradient built from solid bands. It is cheap and avoids
	// depending on msimg32, while looking smoother than a flat title bar.
	h := rectHeight(r)
	if h <= 0 {
		return
	}
	const bands = 24
	for i := 0; i < bands; i++ {
		y1 := r.Top + int32(h*i/bands)
		y2 := r.Top + int32(h*(i+1)/bands)
		t := float64(i) / float64(bands-1)
		c := lerpColor(top, bottom, t)
		fillRect(hdc, RECT{r.Left, y1, r.Right, y2 + 1}, c)
	}
}

func lerpColor(a, b uint32, t float64) uint32 {
	ar, ag, ab := a&0xFF, (a>>8)&0xFF, (a>>16)&0xFF
	br, bg, bb := b&0xFF, (b>>8)&0xFF, (b>>16)&0xFF
	r := uint32(float64(ar) + (float64(br)-float64(ar))*t)
	g := uint32(float64(ag) + (float64(bg)-float64(ag))*t)
	bl := uint32(float64(ab) + (float64(bb)-float64(ab))*t)
	return rgb(r, g, bl)
}
