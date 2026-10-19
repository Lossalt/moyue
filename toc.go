//go:build windows

package main

import "unsafe"

const (
	tocHeaderH = 52
	tocItemH   = 38
)

func (a *App) createTOC() {
	cls, _ := utf16Ptr("MoYueTOC")
	a.toc, _, _ = procCreateWindowExW.Call(
		0, cls, 0,
		uintptr(WS_CHILD|WS_CLIPSIBLINGS),
		0, 0, 10, 10, a.hwnd, 0, a.hInst, 0,
	)
}

func (a *App) updateTOC() {
	if a.tocCurrent >= 0 && a.doc != nil && a.tocCurrent >= len(a.doc.Chapters) {
		a.tocCurrent = -1
	}
	a.invalidateTOC()
}

func (a *App) tocHeaderHeight() int { return scale(tocHeaderH, a.dpi) }
func (a *App) tocItemHeight() int   { return scale(tocItemH, a.dpi) }

func (a *App) tocListHeight() int {
	var r RECT
	procGetClientRect.Call(a.toc, uintptr(unsafe.Pointer(&r)))
	h := rectHeight(r) - a.tocHeaderHeight() - scale(6, a.dpi)
	if h < 0 {
		return 0
	}
	return h
}

func (a *App) tocMaxScroll() int {
	if a.doc == nil {
		return 0
	}
	total := len(a.doc.Chapters) * a.tocItemHeight()
	return maxInt(0, total-a.tocListHeight())
}

func (a *App) ensureTOCVisible(idx int) {
	if idx < 0 || !a.tocVisible() {
		return
	}
	itemH := a.tocItemHeight()
	listH := a.tocListHeight()
	if listH <= 0 {
		return
	}
	top := idx * itemH
	bottom := top + itemH
	if top < a.tocScroll {
		a.tocScroll = top
	} else if bottom > a.tocScroll+listH {
		a.tocScroll = bottom - listH
	}
	a.tocScroll = clampInt(a.tocScroll, 0, a.tocMaxScroll())
	a.invalidateTOC()
}

func tocWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	a := gApp
	if a == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		a.paintTOC(hwnd)
		return 0
	case WM_SIZE:
		a.tocScroll = clampInt(a.tocScroll, 0, a.tocMaxScroll())
		a.invalidateTOC()
		return 0
	case WM_KEYDOWN:
		if a.dialogActive {
			return 0
		}
		if id := a.shortcutFor(wParam); id != 0 {
			a.handleCommand(id)
			return 0
		}
		return 0
	case WM_MOUSEMOVE:
		x, y := int(loword(lParam)), int(hiword(lParam))
		h := a.tocHoverAt(x, y)
		if h != a.tocHover {
			a.tocHover = h
			a.invalidateTOC()
		}
		if !a.mouseTracked {
			tme := TRACKMOUSEEVENT{CbSize: uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})), DwFlags: 0x00000002, HwndTrack: hwnd}
			procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
			a.mouseTracked = true
		}
		return 0
	case WM_MOUSELEAVE:
		a.mouseTracked = false
		if a.tocHover != -1 {
			a.tocHover = -1
			a.invalidateTOC()
		}
		return 0
	case WM_SETCURSOR:
		if loword(lParam) == 1 && a.tocHover != -1 {
			hand, _, _ := procLoadCursorW.Call(0, IDC_HAND)
			procSetCursor.Call(hand)
			return 1
		}
		return 0
	case WM_LBUTTONDOWN:
		x, y := int(loword(lParam)), int(hiword(lParam))
		if a.tocCloseRect().Left <= int32(x) && int32(x) < a.tocCloseRect().Right && a.tocCloseRect().Top <= int32(y) && int32(y) < a.tocCloseRect().Bottom {
			a.handleCommand(idTOC)
			return 0
		}
		if idx := a.tocIndexAt(x, y); idx >= 0 {
			procSetFocus.Call(a.rich)
			a.jumpChapter(idx)
			a.hideCaret()
		}
		return 0
	case WM_MOUSEWHEEL:
		delta := int(int16((wParam >> 16) & 0xFFFF))
		steps := delta / 120
		if steps == 0 {
			if delta > 0 {
				steps = 1
			} else {
				steps = -1
			}
		}
		a.tocScroll -= steps * a.tocItemHeight() * 3
		a.tocScroll = clampInt(a.tocScroll, 0, a.tocMaxScroll())
		a.invalidateTOC()
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func (a *App) paintTOC(hwnd uintptr) {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	var cr RECT
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
	w, h := rectWidth(cr), rectHeight(cr)
	if w <= 0 || h <= 0 {
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return
	}
	mem, _, _ := procCreateCompatibleDC.Call(hdc)
	bmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	old, _, _ := procSelectObject.Call(mem, bmp)

	headerH := a.tocHeaderHeight()
	listH := a.tocListHeight()
	bg := lerpColor(a.theme.Toolbar, a.theme.Paper, 0.32)
	fillRect(mem, cr, bg)
	fillRect(mem, RECT{cr.Right - 1, 0, cr.Right, cr.Bottom}, a.theme.Border)
	fillRect(mem, RECT{0, int32(headerH - 1), cr.Right, int32(headerH)}, a.theme.Border)

	// Header.
	drawText(mem, "目录", RECT{int32(scale(18, a.dpi)), 0, cr.Right, int32(headerH)},
		DT_LEFT|DT_VCENTER|DT_SINGLELINE, a.fonts.toc, a.theme.Text)
	count := "0"
	if a.doc != nil {
		count = itoa(len(a.doc.Chapters)) + " 章"
	}
	drawText(mem, count, RECT{int32(scale(66, a.dpi)), 0, cr.Right - int32(scale(46, a.dpi)), int32(headerH)},
		DT_LEFT|DT_VCENTER|DT_SINGLELINE, a.fonts.small, a.theme.Muted)

	// Close glyph.
	xr := a.tocCloseRect()
	drawLine(mem, int(xr.Left)+scale(8, a.dpi), int(xr.Top)+scale(8, a.dpi), int(xr.Right)-scale(8, a.dpi), int(xr.Bottom)-scale(8, a.dpi), a.theme.Muted, 1)
	drawLine(mem, int(xr.Right)-scale(8, a.dpi), int(xr.Top)+scale(8, a.dpi), int(xr.Left)+scale(8, a.dpi), int(xr.Bottom)-scale(8, a.dpi), a.theme.Muted, 1)

	if a.doc == nil || len(a.doc.Chapters) == 0 {
		drawText(mem, "这本书没有目录", RECT{0, int32(headerH + scale(20, a.dpi)), cr.Right, int32(headerH + scale(56, a.dpi))},
			DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.ui, a.theme.Muted)
		procBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), mem, 0, 0, SRCCOPY)
		procSelectObject.Call(mem, old)
		procDeleteObject.Call(bmp)
		procDeleteDC.Call(mem)
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return
	}

	itemH := a.tocItemHeight()
	start := a.tocScroll / itemH
	offset := a.tocScroll % itemH
	firstY := headerH + scale(6, a.dpi) - offset
	maxVisible := listH/itemH + 2
	for i := start; i < len(a.doc.Chapters) && i < start+maxVisible; i++ {
		y := firstY + (i-start)*itemH
		if y+itemH < headerH || y > h {
			continue
		}
		r := RECT{int32(scale(8, a.dpi)), int32(y), cr.Right - int32(scale(10, a.dpi)), int32(y + itemH)}
		if i == a.tocCurrent {
			fillRoundRect(mem, r, scale(7, a.dpi), a.theme.Selected)
			fillRect(mem, RECT{r.Left, r.Top + int32(scale(8, a.dpi)), r.Left + int32(scale(3, a.dpi)), r.Bottom - int32(scale(8, a.dpi))}, a.theme.Accent)
		} else if a.tocHover == i {
			fillRoundRect(mem, r, scale(7, a.dpi), a.theme.Hover)
		}
		level := a.doc.Chapters[i].Level
		if level > 2 {
			level = 2
		}
		tr := r
		tr.Left += int32(scale(14+level*14, a.dpi))
		tr.Right -= int32(scale(8, a.dpi))
		if level > 0 {
			drawText(mem, a.doc.Chapters[i].Title, tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS, a.fonts.ui, a.theme.Muted)
		} else {
			drawText(mem, a.doc.Chapters[i].Title, tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS, a.fonts.uiBold, a.theme.Text)
		}
	}

	// Slim scrollbar.
	if a.tocMaxScroll() > 0 {
		track := RECT{cr.Right - int32(scale(5, a.dpi)), int32(headerH + scale(8, a.dpi)), cr.Right - int32(scale(2, a.dpi)), int32(h - scale(8, a.dpi))}
		fillRoundRect(mem, track, 2, a.theme.Border)
		total := len(a.doc.Chapters)*itemH + listH
		thumbH := maxInt(scale(28, a.dpi), listH*listH/maxInt(1, total))
		maxScroll := a.tocMaxScroll()
		thumbY := int(track.Top) + int(float64(a.tocScroll)/float64(maxScroll)*float64(rectHeight(track)-thumbH))
		fillRoundRect(mem, RECT{track.Left, int32(thumbY), track.Right, int32(thumbY + thumbH)}, 2, a.theme.Scroll)
	}

	procBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), mem, 0, 0, SRCCOPY)
	procSelectObject.Call(mem, old)
	procDeleteObject.Call(bmp)
	procDeleteDC.Call(mem)
	procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
}

func (a *App) tocCloseRect() RECT {
	return RECT{int32(scale(258, a.dpi)), int32(scale(13, a.dpi)), int32(scale(286, a.dpi)), int32(scale(41, a.dpi))}
}

func (a *App) tocIndexAt(x, y int) int {
	if a.doc == nil {
		return -1
	}
	headerH := a.tocHeaderHeight()
	if y < headerH {
		return -1
	}
	rel := y - headerH - scale(6, a.dpi) + a.tocScroll
	idx := rel / a.tocItemHeight()
	if idx < 0 || idx >= len(a.doc.Chapters) {
		return -1
	}
	return idx
}

func (a *App) tocHoverAt(x, y int) int {
	cr := a.tocCloseRect()
	if x >= int(cr.Left) && x < int(cr.Right) && y >= int(cr.Top) && y < int(cr.Bottom) {
		return -2
	}
	return a.tocIndexAt(x, y)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
