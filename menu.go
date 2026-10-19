//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func (a *App) showMenu(anchor RECT, items []menuItem, alignRight bool) {
	if anchor.Right == 0 || len(items) == 0 {
		return
	}
	a.closeMenu(false)
	a.menuItems = items
	a.menuHover = -1
	a.menuPress = -1
	a.menuPad = scale(6, a.dpi)
	a.menuItemH = scale(36, a.dpi)
	a.menuSepH = scale(9, a.dpi)

	hdc, _, _ := procGetDC.Call(a.hwnd)
	maxLabel := 0
	for _, it := range items {
		if it.sep {
			continue
		}
		if w := textWidth(hdc, it.label, a.fonts.ui); w > maxLabel {
			maxLabel = w
		}
	}
	procReleaseDC.Call(a.hwnd, hdc)
	w := maxLabel + scale(58, a.dpi)
	if w < scale(190, a.dpi) {
		w = scale(190, a.dpi)
	}
	if w > scale(300, a.dpi) {
		w = scale(300, a.dpi)
	}
	h := a.menuPad * 2
	for _, it := range items {
		if it.sep {
			h += a.menuSepH
		} else {
			h += a.menuItemH
		}
	}
	a.menuW, a.menuH = w, h

	tl := POINT{X: anchor.Left, Y: anchor.Top}
	br := POINT{X: anchor.Right, Y: anchor.Bottom}
	procClientToScreen.Call(a.hwnd, uintptr(unsafe.Pointer(&tl)))
	procClientToScreen.Call(a.hwnd, uintptr(unsafe.Pointer(&br)))
	x := tl.X
	if alignRight {
		x = br.X - int32(w)
	}
	y := br.Y + int32(scale(2, a.dpi))
	sw := int(getSystemMetrics(0))
	sh := int(getSystemMetrics(1))
	if int(y)+h > sh {
		y = tl.Y - int32(h) - int32(scale(2, a.dpi))
	}
	if int(x)+w > sw {
		x = int32(sw - w)
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	pt := POINT{X: x, Y: y}
	cls, _ := utf16Ptr("MoYueMenu")
	a.menuHwnd, _, _ = procCreateWindowExW.Call(
		0x00000080, // WS_EX_TOOLWINDOW
		cls, 0,
		uintptr(WS_POPUP),
		uintptr(pt.X), uintptr(pt.Y), uintptr(w), uintptr(h),
		a.hwnd, 0, a.hInst, 0,
	)
	if a.menuHwnd == 0 {
		return
	}
	procShowWindow.Call(a.menuHwnd, SW_SHOW)
	procSetCapture.Call(a.menuHwnd)
	procSetFocus.Call(a.menuHwnd)
	invalidateRect(a.menuHwnd, nil, false)
}

func (a *App) closeMenu(restoreFocus bool) {
	if a.menuHwnd == 0 {
		return
	}
	h := a.menuHwnd
	a.menuHwnd = 0
	a.menuClosing = true
	procReleaseCapture.Call()
	procShowWindow.Call(h, SW_HIDE)
	procDestroyWindow.Call(h)
	a.menuClosing = false
	a.menuItems = nil
	a.menuHover = -1
	if restoreFocus {
		if a.state == stateReading {
			procSetFocus.Call(a.rich)
		} else {
			procSetFocus.Call(a.hwnd)
		}
	}
	a.invalidateAll()
}

func menuWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	a := gApp
	if a == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		a.paintMenu(hwnd)
		return 0
	case WM_MOUSEMOVE:
		x, y := int(loword(lParam)), int(hiword(lParam))
		idx := a.menuIndexAt(x, y)
		if idx != a.menuHover {
			a.menuHover = idx
			invalidateRect(hwnd, nil, false)
		}
		return 0
	case WM_MOUSELEAVE:
		if a.menuHover != -1 {
			a.menuHover = -1
			invalidateRect(hwnd, nil, false)
		}
		return 0
	case WM_LBUTTONDOWN:
		x, y := int(loword(lParam)), int(hiword(lParam))
		idx := a.menuIndexAt(x, y)
		if idx < 0 {
			a.closeMenu(true)
			return 0
		}
		a.menuPress = idx
		invalidateRect(hwnd, nil, false)
		return 0
	case WM_LBUTTONUP:
		x, y := int(loword(lParam)), int(hiword(lParam))
		idx := a.menuIndexAt(x, y)
		if idx >= 0 && idx == a.menuPress {
			id := a.menuItems[idx].id
			a.menuPress = -1
			a.closeMenu(false)
			a.handleCommand(id)
			if a.menuHwnd == 0 {
				if a.state == stateReading {
					procSetFocus.Call(a.rich)
					a.hideCaret()
				} else {
					procSetFocus.Call(a.hwnd)
				}
			}
		} else {
			a.menuPress = -1
			invalidateRect(hwnd, nil, false)
		}
		return 0
	case WM_KEYDOWN:
		switch wParam {
		case 0x1B:
			a.closeMenu(true)
		case 0x26:
			a.moveMenuHover(-1)
		case 0x28:
			a.moveMenuHover(1)
		case 0x0D, 0x20:
			if a.menuHover >= 0 && a.menuHover < len(a.menuItems) {
				id := a.menuItems[a.menuHover].id
				a.closeMenu(false)
				a.handleCommand(id)
			}
		}
		return 0
	case WM_CAPTURECHANGED:
		if !a.menuClosing {
			a.closeMenu(false)
		}
		return 0
	case WM_SETCURSOR:
		if loword(lParam) == 1 && a.menuHover >= 0 {
			hand, _, _ := procLoadCursorW.Call(0, IDC_HAND)
			procSetCursor.Call(hand)
			return 1
		}
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func (a *App) menuIndexAt(x, y int) int {
	if a.menuHwnd == 0 || x < 0 || y < 0 || x >= a.menuW || y >= a.menuH {
		return -1
	}
	cur := a.menuPad
	for i, it := range a.menuItems {
		h := a.menuItemH
		if it.sep {
			h = a.menuSepH
		}
		if y >= cur && y < cur+h {
			if it.sep {
				return -1
			}
			return i
		}
		cur += h
	}
	return -1
}

func (a *App) moveMenuHover(delta int) {
	if len(a.menuItems) == 0 {
		return
	}
	i := a.menuHover
	for n := 0; n < len(a.menuItems); n++ {
		i += delta
		if i < 0 {
			i = len(a.menuItems) - 1
		}
		if i >= len(a.menuItems) {
			i = 0
		}
		if !a.menuItems[i].sep && !a.menuItems[i].disabled {
			a.menuHover = i
			invalidateRect(a.menuHwnd, nil, false)
			return
		}
	}
}

func (a *App) paintMenu(hwnd uintptr) {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	w, h := a.menuW, a.menuH
	mem, _, _ := procCreateCompatibleDC.Call(hdc)
	bmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	old, _, _ := procSelectObject.Call(mem, bmp)
	fillRect(mem, RECT{0, 0, int32(w), int32(h)}, a.theme.Paper)
	strokeRoundRect(mem, RECT{0, 0, int32(w), int32(h)}, scale(7, a.dpi), a.theme.Border, 1)

	y := a.menuPad
	for i, it := range a.menuItems {
		if it.sep {
			drawLine(mem, scale(12, a.dpi), y+a.menuSepH/2, w-scale(12, a.dpi), y+a.menuSepH/2, a.theme.Border, 1)
			y += a.menuSepH
			continue
		}
		r := RECT{int32(scale(4, a.dpi)), int32(y), int32(w - scale(4, a.dpi)), int32(y + a.menuItemH)}
		if i == a.menuHover {
			fillRoundRect(mem, r, scale(6, a.dpi), a.theme.Hover)
		}
		if it.checked {
			fillRoundRect(mem, RECT{r.Left + int32(scale(5, a.dpi)), r.Top + int32(scale(6, a.dpi)), r.Left + int32(scale(9, a.dpi)), r.Bottom - int32(scale(6, a.dpi))}, scale(2, a.dpi), a.theme.Accent)
		}
		tr := r
		tr.Left += int32(scale(22, a.dpi))
		tr.Right -= int32(scale(10, a.dpi))
		color := a.theme.Text
		if it.disabled {
			color = a.theme.Muted
		}
		drawText(mem, it.label, tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS, a.fonts.ui, color)
		y += a.menuItemH
	}
	procBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), mem, 0, 0, SRCCOPY)
	procSelectObject.Call(mem, old)
	procDeleteObject.Call(bmp)
	procDeleteDC.Call(mem)
	procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
}

var _ = syscall.NewCallback
