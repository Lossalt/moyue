//go:build windows

package main

import "unsafe"

func measureTextHeight(hdc uintptr, s string, width int, font uintptr) int {
	if font == 0 {
		return 0
	}
	old := withFont(hdc, font)
	u := syscallStringToUTF16(s)
	r := RECT{0, 0, int32(width), 0}
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&r)), uintptr(DT_LEFT|DT_WORDBREAK|DT_NOPREFIX|DT_CALCRECT))
	if old != 0 {
		procSelectObject.Call(hdc, old)
	}
	return rectHeight(r)
}

func syscallStringToUTF16(s string) []uint16 {
	// Kept local to avoid importing syscall in multiple UI files.
	u := make([]uint16, 0, len(s)+1)
	for _, r := range s {
		if r == 0 {
			continue
		}
		if r < 0x10000 {
			u = append(u, uint16(r))
		} else {
			r -= 0x10000
			u = append(u, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		}
	}
	return append(u, 0)
}

func (a *App) showDialog(title, body string, danger bool) {
	if a.hwnd == 0 {
		return
	}
	a.dialogTitle = title
	a.dialogBody = body
	a.dialogDanger = danger
	a.dialogActive = true
	a.hoverID = 0
	a.pressID = 0

	var cr RECT
	procGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&cr)))
	cw, ch := rectWidth(cr), rectHeight(cr)
	w := scale(580, a.dpi)
	if w > cw-scale(48, a.dpi) {
		w = cw - scale(48, a.dpi)
	}
	if w < scale(380, a.dpi) {
		w = scale(380, a.dpi)
	}
	bodyW := w - scale(56, a.dpi)
	hdc, _, _ := procGetDC.Call(a.hwnd)
	bodyH := measureTextHeight(hdc, body, bodyW, a.fonts.ui)
	procReleaseDC.Call(a.hwnd, hdc)
	h := scale(74, a.dpi) + bodyH + scale(86, a.dpi)
	if h < scale(220, a.dpi) {
		h = scale(220, a.dpi)
	}
	if maxH := ch - scale(70, a.dpi); h > maxH {
		h = maxH
	}
	x := (cw - w) / 2
	y := (ch - h) / 2
	a.dialogCard = RECT{int32(x), int32(y), int32(x + w), int32(y + h)}
	bw := scale(92, a.dpi)
	bh := scale(36, a.dpi)
	a.dialogOK = RECT{int32(x + w - bw - scale(26, a.dpi)), int32(y + h - bh - scale(22, a.dpi)), int32(x + w - scale(26, a.dpi)), int32(y + h - scale(22, a.dpi))}
	a.dialogClose = RECT{int32(x + w - scale(42, a.dpi)), int32(y + scale(12, a.dpi)), int32(x + w - scale(12, a.dpi)), int32(y + scale(42, a.dpi))}

	a.layout()
	if a.state == stateReading {
		procShowWindow.Call(a.rich, SW_HIDE)
	}
	procSetFocus.Call(a.hwnd)
	a.invalidateAll()
}

func (a *App) dismissDialog() {
	if !a.dialogActive {
		return
	}
	a.dialogActive = false
	a.dialogDanger = false
	a.hoverID = 0
	a.pressID = 0
	a.layout()
	if a.state == stateReading {
		procSetFocus.Call(a.rich)
		a.hideCaret()
	} else {
		procSetFocus.Call(a.hwnd)
	}
	a.invalidateAll()
}

func (a *App) paintDialog(hdc uintptr, cr RECT) {
	if !a.dialogActive {
		return
	}
	scrim := lerpColor(a.theme.Paper, a.theme.Text, 0.18)
	fillRect(hdc, cr, scrim)
	card := a.dialogCard
	shadow := RECT{card.Left + int32(scale(4, a.dpi)), card.Top + int32(scale(6, a.dpi)), card.Right + int32(scale(4, a.dpi)), card.Bottom + int32(scale(6, a.dpi))}
	fillRoundRect(hdc, shadow, scale(14, a.dpi), lerpColor(a.theme.Paper, a.theme.Text, 0.10))
	fillRoundRect(hdc, card, scale(14, a.dpi), a.theme.Paper)
	strokeRoundRect(hdc, card, scale(14, a.dpi), a.theme.Border, 1)
	fillRect(hdc, RECT{card.Left + int32(scale(18, a.dpi)), card.Top, card.Left + int32(scale(58, a.dpi)), card.Top + int32(scale(3, a.dpi))}, a.theme.Accent)

	titleColor := a.theme.Text
	if a.dialogDanger {
		titleColor = a.theme.Danger
	}
	drawText(hdc, a.dialogTitle, RECT{card.Left + int32(scale(28, a.dpi)), card.Top + int32(scale(18, a.dpi)), card.Right - int32(scale(54, a.dpi)), card.Top + int32(scale(54, a.dpi))},
		DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS, a.fonts.heading, titleColor)
	fillRect(hdc, RECT{card.Left + int32(scale(28, a.dpi)), card.Top + int32(scale(60, a.dpi)), card.Right - int32(scale(28, a.dpi)), card.Top + int32(scale(61, a.dpi))}, a.theme.Border)
	body := RECT{card.Left + int32(scale(28, a.dpi)), card.Top + int32(scale(76, a.dpi)), card.Right - int32(scale(28, a.dpi)), a.dialogOK.Top - int32(scale(16, a.dpi))}
	drawText(hdc, a.dialogBody, body, DT_LEFT|DT_WORDBREAK|DT_NOPREFIX, a.fonts.ui, a.theme.Text)

	// Close X.
	xr := a.dialogClose
	if a.hoverID == idDialogClose {
		fillRoundRect(hdc, xr, scale(7, a.dpi), a.theme.Hover)
	}
	drawLine(hdc, int(xr.Left)+scale(9, a.dpi), int(xr.Top)+scale(9, a.dpi), int(xr.Right)-scale(9, a.dpi), int(xr.Bottom)-scale(9, a.dpi), a.theme.Muted, 1)
	drawLine(hdc, int(xr.Right)-scale(9, a.dpi), int(xr.Top)+scale(9, a.dpi), int(xr.Left)+scale(9, a.dpi), int(xr.Bottom)-scale(9, a.dpi), a.theme.Muted, 1)

	hover := a.hoverID == idDialogOK
	pressed := a.pressID == idDialogOK
	bg := a.theme.Accent
	if pressed {
		bg = lerpColor(bg, colorRef(0x000000), 0.12)
	} else if hover {
		bg = lerpColor(bg, colorRef(0xFFFFFF), 0.10)
	}
	fillRoundRect(hdc, a.dialogOK, scale(8, a.dpi), bg)
	drawText(hdc, "确定", a.dialogOK, DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.uiBold, a.theme.OnAccent)
}
