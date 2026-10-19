//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

func (a *App) layout() {
	if a.hwnd == 0 || a.rich == 0 {
		return
	}
	var cr RECT
	procGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&cr)))
	if rectWidth(cr) <= 0 || rectHeight(cr) <= 0 {
		return
	}
	top := 0
	bottom := rectHeight(cr)
	if !a.fullscreen {
		top = scale(58, a.dpi)
		bottom = rectHeight(cr) - scale(30, a.dpi)
	}
	top32 := int32(top)
	tocW := 0
	if a.tocVisible() {
		tocW = scale(300, a.dpi)
	}
	contentLeft := int32(tocW)

	if a.state == stateReading {
		padX := scale(28, a.dpi)
		topPad := scale(10, a.dpi)
		if a.fullscreen {
			padX = scale(64, a.dpi)
			topPad = scale(46, a.dpi)
		}
		avail := int(cr.Right - contentLeft)
		rw := avail - 2*padX
		maxW := scale(820, a.dpi)
		if a.fullscreen {
			maxW = scale(920, a.dpi)
		}
		if rw > maxW {
			rw = maxW
		}
		if rw < 280 {
			rw = avail - 2*scale(12, a.dpi)
		}
		if rw < 80 {
			rw = 80
		}
		x := int(contentLeft) + (avail-rw)/2
		y := top + topPad
		h := bottom - top - 2*topPad
		if h < 80 {
			h = 80
		}
		procSetWindowPos.Call(a.rich, 0, uintptr(x), uintptr(y), uintptr(rw), uintptr(h), SWP_NOZORDER|SWP_NOACTIVATE)
	}

	if a.tocVisible() {
		procSetWindowPos.Call(a.toc, 0, 0, uintptr(top32), uintptr(tocW), uintptr(bottom-top), SWP_NOZORDER|SWP_NOACTIVATE|SWP_SHOWWINDOW)
	} else if a.toc != 0 {
		procShowWindow.Call(a.toc, SW_HIDE)
	}

	a.layoutButtons(cr)
	a.layoutWelcome(cr, top, bottom)
	a.showRich()
}

func (a *App) layoutButtons(cr RECT) {
	a.buttons = nil
	if a.fullscreen {
		return
	}
	y := scale(11, a.dpi)
	h := scale(36, a.dpi)
	x := scale(16, a.dpi)
	add := func(id int, xx, yy, ww, hh int) {
		a.buttons = append(a.buttons, uiButton{ID: id, Rect: RECT{int32(xx), int32(yy), int32(xx + ww), int32(yy + hh)}})
	}
	add(idOpen, x, y, scale(86, a.dpi), h)
	x += scale(86+8, a.dpi)
	add(idTOC, x, y, scale(86, a.dpi), h)
	x += scale(86+14, a.dpi)
	add(idFontMenu, x, y, scale(104, a.dpi), h)
	x += scale(112, a.dpi)
	groupW := scale(118, a.dpi)
	add(idFontMinus, x, y, scale(34, a.dpi), h)
	add(idFontPlus, x+groupW-scale(34, a.dpi), y, scale(34, a.dpi), h)
	x += groupW + scale(10, a.dpi)
	add(idLineMenu, x, y, scale(112, a.dpi), h)
	x += scale(122, a.dpi)
	add(idThemeMenu, x, y, scale(40, a.dpi), h)
	// The overflow menu sits at the far right.
	mx := rectWidth(cr) - scale(16+40, a.dpi)
	if mx > x+scale(50, a.dpi) {
		add(idMore, mx, y, scale(40, a.dpi), h)
	}
}

func (a *App) layoutWelcome(cr RECT, top, bottom int) {
	a.welcomeButtons = nil
	a.hasSecondary = false
	h := bottom - top
	if h <= 0 {
		return
	}
	pw := scale(214, a.dpi)
	ph := scale(48, a.dpi)
	px := (rectWidth(cr) - pw) / 2
	py := top + h*58/100
	a.welcomePrimary = RECT{int32(px), int32(py), int32(px + pw), int32(py + ph)}
	primaryID := idOpen
	if a.state == stateError {
		primaryID = idRetry
	}
	if a.state == stateWelcome || a.state == stateError {
		a.welcomeButtons = append(a.welcomeButtons, uiButton{ID: primaryID, Rect: a.welcomePrimary})
	}
	if a.state == stateWelcome && a.settings.LastBook != "" {
		if st, err := os.Stat(a.settings.LastBook); err == nil && !st.IsDir() {
			sw := scale(246, a.dpi)
			sh := scale(40, a.dpi)
			sx := (rectWidth(cr) - sw) / 2
			sy := py + ph + scale(14, a.dpi)
			a.welcomeSecondary = RECT{int32(sx), int32(sy), int32(sx + sw), int32(sy + sh)}
			a.welcomeButtons = append(a.welcomeButtons, uiButton{ID: idContinue, Rect: a.welcomeSecondary})
			a.hasSecondary = true
		}
	}
}

func (a *App) paint(hdc uintptr, cr RECT) {
	fillRect(hdc, cr, a.theme.Paper)
	if !a.fullscreen {
		a.paintToolbar(hdc, cr)
	}
	if a.state != stateReading {
		a.paintWelcome(hdc, cr)
	}
	if !a.fullscreen {
		a.paintStatus(hdc, cr)
	} else if a.state == stateReading {
		h := rectHeight(cr)
		fillRect(hdc, RECT{0, int32(h - 3), cr.Right, int32(h)}, a.theme.Border)
		pw := int(float64(rectWidth(cr)) * a.progress)
		if pw > 0 {
			fillRect(hdc, RECT{0, int32(h - 3), int32(pw), int32(h)}, a.theme.Accent)
		}
	}
	a.paintDialog(hdc, cr)
}

func (a *App) paintToolbar(hdc uintptr, cr RECT) {
	h := scale(58, a.dpi)
	r := RECT{0, 0, cr.Right, int32(h)}
	gradientRect(hdc, r, a.theme.Toolbar, lerpColor(a.theme.Toolbar, a.theme.Paper, 0.18))
	fillRect(hdc, RECT{0, int32(h - 1), cr.Right, int32(h)}, a.theme.Border)

	// Background for the grouped A-/size/A+ control.
	var minus, plus RECT
	for _, b := range a.buttons {
		if b.ID == idFontMinus {
			minus = b.Rect
		}
		if b.ID == idFontPlus {
			plus = b.Rect
		}
	}
	if minus.Right != 0 && plus.Right != 0 {
		group := RECT{minus.Left, minus.Top, plus.Right, plus.Bottom}
		fillRoundRect(hdc, group, scale(8, a.dpi), a.theme.Control)
		strokeRoundRect(hdc, group, scale(8, a.dpi), a.theme.Border, 1)
	}

	// A quiet divider after the navigation group.
	if len(a.buttons) > 2 {
		x := a.buttons[1].Rect.Right + int32(scale(7, a.dpi))
		drawLine(hdc, int(x), scale(16, a.dpi), int(x), h-scale(16, a.dpi), a.theme.Border, 1)
	}

	for _, b := range a.buttons {
		a.paintButton(hdc, b)
	}
	// The current size label sits in the middle of the group.
	if minus.Right != 0 && plus.Left != 0 {
		mid := RECT{minus.Right, minus.Top, plus.Left, minus.Bottom}
		drawText(hdc, itoa(int(a.settings.FontSize+0.5)), mid, DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.uiBold, a.theme.Text)
	}
}

func (a *App) paintButton(hdc uintptr, b uiButton) {
	hover := a.hoverID == b.ID
	pressed := a.pressID == b.ID
	switch b.ID {
	case idOpen:
		bg := a.theme.Accent
		if pressed {
			bg = lerpColor(bg, colorRef(0x000000), 0.12)
		} else if hover {
			bg = lerpColor(bg, colorRef(0xFFFFFF), 0.10)
		}
		fillRoundRect(hdc, b.Rect, scale(8, a.dpi), bg)
		cy := int((b.Rect.Top + b.Rect.Bottom) / 2)
		drawFolderIcon(hdc, int(b.Rect.Left)+scale(24, a.dpi), cy, a.theme.OnAccent)
		tr := b.Rect
		tr.Left += int32(scale(38, a.dpi))
		tr.Right -= int32(scale(8, a.dpi))
		drawText(hdc, "打开", tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE, a.fonts.ui, a.theme.OnAccent)
	case idTOC:
		bg := uint32(0)
		if a.tocVisible() {
			bg = a.theme.Selected
		} else if hover {
			bg = a.theme.Hover
		}
		if bg != 0 {
			fillRoundRect(hdc, b.Rect, scale(8, a.dpi), bg)
		}
		cy := int((b.Rect.Top + b.Rect.Bottom) / 2)
		drawListIcon(hdc, int(b.Rect.Left)+scale(22, a.dpi), cy, a.theme.Text)
		tr := b.Rect
		tr.Left += int32(scale(36, a.dpi))
		drawText(hdc, "目录", tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE, a.fonts.ui, a.theme.Text)
	case idFontMenu, idLineMenu:
		bg := a.theme.Control
		if pressed {
			bg = a.theme.Selected
		} else if hover {
			bg = a.theme.Hover
		}
		fillRoundRect(hdc, b.Rect, scale(8, a.dpi), bg)
		strokeRoundRect(hdc, b.Rect, scale(8, a.dpi), a.theme.Border, 1)
		tr := b.Rect
		tr.Left += int32(scale(11, a.dpi))
		tr.Right -= int32(scale(24, a.dpi))
		label := a.fontLabel()
		if b.ID == idLineMenu {
			label = fmt.Sprintf("行距 %.1f", a.settings.LineSpacing)
		}
		drawText(hdc, label, tr, DT_LEFT|DT_VCENTER|DT_SINGLELINE, a.fonts.ui, a.theme.Text)
		cx := int(b.Rect.Right) - scale(13, a.dpi)
		cy := int((b.Rect.Top + b.Rect.Bottom) / 2)
		drawChevron(hdc, cx, cy, a.theme.Muted, scale(4, a.dpi))
	case idFontMinus, idFontPlus:
		label := "A−"
		if b.ID == idFontPlus {
			label = "A+"
		}
		if hover || pressed {
			inner := insetRect(b.Rect, scale(3, a.dpi), scale(3, a.dpi))
			bg := a.theme.Hover
			if pressed {
				bg = a.theme.Selected
			}
			fillRoundRect(hdc, inner, scale(6, a.dpi), bg)
		}
		drawText(hdc, label, b.Rect, DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.ui, a.theme.Text)
	case idThemeMenu:
		if hover || pressed {
			bg := a.theme.Hover
			if pressed {
				bg = a.theme.Selected
			}
			fillRoundRect(hdc, b.Rect, scale(8, a.dpi), bg)
		}
		cx := int((b.Rect.Left + b.Rect.Right) / 2)
		cy := int((b.Rect.Top + b.Rect.Bottom) / 2)
		a.drawThemeIcon(hdc, cx, cy)
	case idMore:
		if hover || pressed {
			bg := a.theme.Hover
			if pressed {
				bg = a.theme.Selected
			}
			fillRoundRect(hdc, b.Rect, scale(8, a.dpi), bg)
		}
		cx := int((b.Rect.Left + b.Rect.Right) / 2)
		cy := int((b.Rect.Top + b.Rect.Bottom) / 2)
		drawMoreIcon(hdc, cx, cy, a.theme.Muted)
	}
}

func (a *App) fontLabel() string {
	switch a.settings.Font {
	case "sans":
		return "黑体"
	case "kai":
		return "楷体"
	default:
		return "宋体"
	}
}

func (a *App) drawThemeIcon(hdc uintptr, cx, cy int) {
	if a.settings.Theme == 2 {
		rad := scale(7, a.dpi)
		drawCircle(hdc, cx, cy, rad, a.theme.Accent, true)
		drawCircle(hdc, cx+scale(3, a.dpi), cy-scale(2, a.dpi), rad-scale(1, a.dpi), a.theme.Paper, true)
		return
	}
	rad := scale(6, a.dpi)
	drawCircle(hdc, cx, cy, rad, a.theme.Accent, false)
	for i := 0; i < 8; i++ {
		dx := []int{0, 4, 6, 4, 0, -4, -6, -4}[i]
		dy := []int{-6, -4, 0, 4, 6, 4, 0, -4}[i]
		x1 := cx + dx*rad/6
		y1 := cy + dy*rad/6
		x2 := cx + dx*rad/3
		y2 := cy + dy*rad/3
		drawLine(hdc, x1, y1, x2, y2, a.theme.Accent, 1)
	}
}

func (a *App) paintStatus(hdc uintptr, cr RECT) {
	h := scale(30, a.dpi)
	r := RECT{0, cr.Bottom - int32(h), cr.Right, cr.Bottom}
	fillRect(hdc, r, a.theme.Toolbar)
	fillRect(hdc, RECT{0, r.Top, cr.Right, r.Top + 1}, a.theme.Border)
	// Reading progress.
	fillRect(hdc, RECT{0, r.Top + 1, cr.Right, r.Top + 3}, a.theme.Border)
	pw := int(float64(rectWidth(cr)) * a.progress)
	if pw > 0 {
		fillRect(hdc, RECT{0, r.Top + 1, int32(pw), r.Top + 3}, a.theme.Accent)
	}
	left, right := "未打开书籍", "Ctrl+O  打开"
	switch a.state {
	case stateLoading:
		left = "正在打开…"
		right = filepath.Base(a.loadingPath)
	case stateReading:
		left, right = a.statusLeft, a.statusRight
	case stateError:
		left, right = "打开失败", ""
	}
	tr := RECT{r.Left + int32(scale(16, a.dpi)), r.Top, r.Right - int32(scale(16, a.dpi)), r.Bottom}
	lr := tr
	lr.Right = tr.Left + int32(rectWidth(tr)*62/100)
	drawText(hdc, left, lr, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS, a.fonts.ui, a.theme.Text)
	drawText(hdc, right, tr, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS, a.fonts.small, a.theme.Muted)
}

func (a *App) paintWelcome(hdc uintptr, cr RECT) {
	top, bottom := 0, rectHeight(cr)
	if !a.fullscreen {
		top = scale(58, a.dpi)
		bottom = rectHeight(cr) - scale(30, a.dpi)
	}
	content := RECT{0, int32(top), cr.Right, int32(bottom)}
	h := bottom - top
	if h <= 0 {
		return
	}
	// A very quiet watermark on the right.
	wmW := scale(210, a.dpi)
	wmH := scale(220, a.dpi)
	wm := RECT{content.Right - int32(wmW+scale(20, a.dpi)), content.Bottom - int32(wmH+scale(40, a.dpi)), content.Right - int32(scale(20, a.dpi)), content.Bottom - int32(scale(40, a.dpi))}
	drawText(hdc, "墨", wm, DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.watermark, a.theme.Watermark)

	cx := rectWidth(cr) / 2
	switch a.state {
	case stateLoading:
		a.paintLoading(hdc, content)
		return
	case stateError:
		a.paintError(hdc, content)
		return
	}

	brandY := top + h*24/100
	brandRect := RECT{0, int32(brandY), cr.Right, int32(brandY + scale(80, a.dpi))}
	drawText(hdc, "墨阅", brandRect, DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.brand, a.theme.Text)

	lineY := brandY + scale(74, a.dpi)
	lineW := scale(46, a.dpi)
	fillRect(hdc, RECT{int32(cx - lineW/2), int32(lineY), int32(cx + lineW/2), int32(lineY + 2)}, a.theme.Accent)
	drawText(hdc, "I N K   R E A D E R", RECT{0, int32(lineY + scale(12, a.dpi)), cr.Right, int32(lineY + scale(32, a.dpi))},
		DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.brandEn, a.theme.Muted)
	drawText(hdc, "轻量 EPUB 小说阅读器", RECT{0, int32(lineY + scale(38, a.dpi)), cr.Right, int32(lineY + scale(62, a.dpi))},
		DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.sub, a.theme.Muted)

	a.paintPill(hdc, a.welcomePrimary, "打开一本 EPUB", true, idOpen)
	if a.hasSecondary {
		title := a.settings.LastTitle
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(a.settings.LastBook), filepath.Ext(a.settings.LastBook))
		}
		label := "继续阅读《" + truncateRunes(title, 12) + "》"
		a.paintPill(hdc, a.welcomeSecondary, label, false, idContinue)
	}
	hintY := a.welcomePrimary.Bottom + int32(scale(22, a.dpi))
	if a.hasSecondary {
		hintY = a.welcomeSecondary.Bottom + int32(scale(18, a.dpi))
	}
	drawText(hdc, "拖入 EPUB 文件，或按 Ctrl+O 打开", RECT{0, hintY, cr.Right, hintY + int32(scale(26, a.dpi))},
		DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.small, a.theme.Muted)

	drawText(hdc, "v1.0  ·  单文件版", RECT{int32(scale(20, a.dpi)), content.Bottom - int32(scale(28, a.dpi)), int32(scale(220, a.dpi)), content.Bottom - int32(scale(6, a.dpi))},
		DT_LEFT|DT_VCENTER|DT_SINGLELINE, a.fonts.small, a.theme.Muted)
	drawText(hdc, "无需安装  ·  无广告", RECT{content.Right - int32(scale(220, a.dpi)), content.Bottom - int32(scale(28, a.dpi)), content.Right - int32(scale(20, a.dpi)), content.Bottom - int32(scale(6, a.dpi))},
		DT_RIGHT|DT_VCENTER|DT_SINGLELINE, a.fonts.small, a.theme.Muted)
}

func (a *App) paintLoading(hdc uintptr, content RECT) {
	cy := (content.Top + content.Bottom) / 2
	title := "正在展开书页" + strings.Repeat("·", a.tick%4)
	drawText(hdc, title, RECT{content.Left, cy - int32(scale(40, a.dpi)), content.Right, cy + int32(scale(10, a.dpi))},
		DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.heading, a.theme.Text)
	name := filepath.Base(a.loadingPath)
	drawText(hdc, name, RECT{content.Left, cy + int32(scale(6, a.dpi)), content.Right, cy + int32(scale(32, a.dpi))},
		DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS, a.fonts.small, a.theme.Muted)
	// Indeterminate segment.
	trackW := scale(240, a.dpi)
	tx := (rectWidth(RECT{content.Left, 0, content.Right, 0}) - trackW) / 2
	ty := cy + int32(scale(54, a.dpi))
	fillRect(hdc, RECT{int32(tx), ty, int32(tx + trackW), ty + 2}, a.theme.Border)
	seg := scale(70, a.dpi)
	pos := (a.tick * scale(18, a.dpi)) % (trackW + seg)
	if pos > trackW {
		pos = trackW
	}
	fillRect(hdc, RECT{int32(tx + pos - seg), ty, int32(tx + pos), ty + 2}, a.theme.Accent)
	drawText(hdc, "正在解析目录与正文，请稍候", RECT{content.Left, ty + int32(scale(16, a.dpi)), content.Right, ty + int32(scale(42, a.dpi))},
		DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.small, a.theme.Muted)
}

func (a *App) paintError(hdc uintptr, content RECT) {
	cy := (content.Top + content.Bottom) / 2
	drawText(hdc, "未能打开这本书", RECT{content.Left, cy - int32(scale(130, a.dpi)), content.Right, cy - int32(scale(84, a.dpi))},
		DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.heading, a.theme.Danger)

	msgRect := RECT{content.Left + int32(scale(80, a.dpi)), cy - int32(scale(72, a.dpi)), content.Right - int32(scale(80, a.dpi)), cy + int32(scale(30, a.dpi))}
	drawText(hdc, a.errMsg, msgRect, DT_CENTER|DT_WORDBREAK|DT_NOPREFIX, a.fonts.ui, a.theme.Muted)
	a.paintPill(hdc, a.welcomePrimary, "重新选择文件", true, idRetry)
	drawText(hdc, "支持 EPUB 2 / EPUB 3，无 DRM 的电子书", RECT{content.Left, a.welcomePrimary.Bottom + int32(scale(20, a.dpi)), content.Right, a.welcomePrimary.Bottom + int32(scale(46, a.dpi))},
		DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.small, a.theme.Muted)
}

func (a *App) paintPill(hdc uintptr, r RECT, label string, primary bool, id int) {
	hover := a.hoverID == id
	pressed := a.pressID == id
	if primary {
		bg := a.theme.Accent
		if pressed {
			bg = lerpColor(bg, colorRef(0x000000), 0.12)
		} else if hover {
			bg = lerpColor(bg, colorRef(0xFFFFFF), 0.10)
		}
		fillRoundRect(hdc, r, scale(10, a.dpi), bg)
		drawText(hdc, label, r, DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.uiBold, a.theme.OnAccent)
	} else {
		bg := lerpColor(a.theme.Paper, a.theme.Toolbar, 0.55)
		if hover {
			bg = a.theme.Hover
		}
		fillRoundRect(hdc, r, scale(10, a.dpi), bg)
		strokeRoundRect(hdc, r, scale(10, a.dpi), a.theme.Border, 1)
		drawText(hdc, label, r, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS, a.fonts.uiBold, a.theme.Text)
	}
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func drawChevron(hdc uintptr, cx, cy int, color uint32, size int) {
	drawLine(hdc, cx-size, cy-size/2, cx, cy+size/2, color, 1)
	drawLine(hdc, cx, cy+size/2, cx+size, cy-size/2, color, 1)
}

func drawFolderIcon(hdc uintptr, cx, cy int, color uint32) {
	s := 8
	drawLine(hdc, cx-s, cy-s/2, cx-s/3, cy-s/2, color, 1)
	drawLine(hdc, cx-s/3, cy-s/2, cx, cy-s+2, color, 1)
	drawLine(hdc, cx, cy-s+2, cx+s, cy-s+2, color, 1)
	drawLine(hdc, cx+s, cy-s+2, cx+s, cy+s/2, color, 1)
	drawLine(hdc, cx-s, cy-s/2, cx-s, cy+s/2, color, 1)
	drawLine(hdc, cx-s, cy+s/2, cx+s, cy+s/2, color, 1)
}

func drawListIcon(hdc uintptr, cx, cy int, color uint32) {
	for i := -1; i <= 1; i++ {
		y := cy + i*5
		drawCircle(hdc, cx-7, y, 1, color, true)
		drawLine(hdc, cx-2, y, cx+7, y, color, 1)
	}
}

func drawMoreIcon(hdc uintptr, cx, cy int, color uint32) {
	for i := -1; i <= 1; i++ {
		drawCircle(hdc, cx+i*6, cy, 2, color, true)
	}
}
