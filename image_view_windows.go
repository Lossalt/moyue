//go:build windows

package main

import (
	"fmt"
	"strings"
)

func (a *App) imageFor(index int) *renderedImage {
	if a.book == nil || index < 0 || index >= len(a.book.Images) {
		return nil
	}
	if a.imageCache == nil {
		a.imageCache = make(map[int]*renderedImage)
	}
	if img, ok := a.imageCache[index]; ok {
		return img
	}
	asset := a.book.Images[index]
	if strings.Contains(strings.ToLower(asset.MediaType), "svg") {
		a.imageCache[index] = nil
		return nil
	}
	img, err := decodeRenderedImage(asset.Data)
	if err != nil {
		a.imageCache[index] = nil
		return nil
	}
	a.imageCache[index] = img
	return img
}

func (a *App) showImageViewer(index int, opening bool) {
	if a.book == nil || index < 0 || index >= len(a.book.Images) {
		return
	}
	a.closeMenu(false)
	a.viewerActive = true
	a.viewerOpening = opening
	a.viewerIndex = index
	a.viewerImage = a.imageFor(index)
	a.viewerErr = ""
	if a.viewerImage == nil {
		a.viewerErr = "这张图片的格式暂不支持"
	}
	a.updateWindowTitle(a.viewerCaption())
	a.layout()
	a.showRich()
	procSetFocus.Call(a.hwnd)
	a.invalidateAll()
}

func (a *App) closeImageViewer() {
	if !a.viewerActive {
		return
	}
	a.viewerActive = false
	a.viewerOpening = false
	a.viewerImage = nil
	a.viewerErr = ""
	if a.book != nil {
		a.updateWindowTitle(a.book.Title)
	} else {
		a.updateWindowTitle("")
	}
	a.layout()
	a.showRich()
	if a.rich != 0 {
		procSetFocus.Call(a.rich)
		a.hideCaret()
	}
	a.invalidateAll()
}

func (a *App) stepImageViewer(delta int) {
	if !a.viewerActive || a.book == nil || len(a.book.Images) == 0 || delta == 0 {
		return
	}
	n := len(a.book.Images)
	a.viewerIndex = (a.viewerIndex + delta) % n
	if a.viewerIndex < 0 {
		a.viewerIndex += n
	}
	a.viewerImage = a.imageFor(a.viewerIndex)
	a.viewerErr = ""
	if a.viewerImage == nil {
		a.viewerErr = "这张图片的格式暂不支持"
	}
	a.updateWindowTitle(a.viewerCaption())
	a.invalidateAll()
}

func (a *App) viewerCaption() string {
	if a.book == nil || a.viewerIndex < 0 || a.viewerIndex >= len(a.book.Images) {
		return "插图"
	}
	asset := a.book.Images[a.viewerIndex]
	if a.viewerIndex == a.book.CoverIndex {
		return fmt.Sprintf("封面 · %d / %d", a.viewerIndex+1, len(a.book.Images))
	}
	alt := strings.TrimSpace(asset.Alt)
	if alt == "" || alt == "插图" {
		alt = fmt.Sprintf("插图 %d", a.viewerIndex+1)
	}
	return fmt.Sprintf("%s · %d / %d", alt, a.viewerIndex+1, len(a.book.Images))
}

func (a *App) viewerLayout(cr RECT) {
	m := scale(24, a.dpi)
	a.viewerClose = RECT{
		cr.Right - int32(m+scale(104, a.dpi)), int32(scale(13, a.dpi)),
		cr.Right - int32(m), int32(scale(51, a.dpi)),
	}
	a.viewerPrev = RECT{
		int32(m), cr.Bottom - int32(scale(58, a.dpi)),
		int32(m + scale(96, a.dpi)), cr.Bottom - int32(scale(16, a.dpi)),
	}
	a.viewerNext = RECT{
		cr.Right - int32(m+scale(96, a.dpi)), cr.Bottom - int32(scale(58, a.dpi)),
		cr.Right - int32(m), cr.Bottom - int32(scale(16, a.dpi)),
	}
}

func (a *App) paintImageViewer(hdc uintptr, cr RECT) {
	bg := colorRef(0x14110D)
	fillRect(hdc, cr, bg)
	a.viewerLayout(cr)
	m := scale(24, a.dpi)

	titleColor := colorRef(0xEEE7DA)
	muted := colorRef(0x9D9587)
	drawText(hdc, a.viewerCaption(),
		RECT{int32(m), 0, a.viewerClose.Left - int32(scale(16, a.dpi)), int32(scale(64, a.dpi))},
		DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS, a.fonts.heading, titleColor)

	a.paintViewerButton(hdc, a.viewerClose, map[bool]string{true: "开始阅读", false: "关闭"}[a.viewerOpening], true, idViewerClose)

	area := RECT{
		int32(m), int32(scale(70, a.dpi)),
		cr.Right - int32(m), cr.Bottom - int32(scale(72, a.dpi)),
	}
	if a.viewerImage != nil {
		r := fitImageRect(a.viewerImage, area)
		if rectWidth(r) > 0 && rectHeight(r) > 0 {
			shadow := RECT{r.Left + int32(scale(6, a.dpi)), r.Top + int32(scale(7, a.dpi)), r.Right + int32(scale(6, a.dpi)), r.Bottom + int32(scale(7, a.dpi))}
			fillRect(hdc, shadow, colorRef(0x090807))
			drawRenderedImage(hdc, a.viewerImage, r)
			strokeRoundRect(hdc, r, scale(3, a.dpi), colorRef(0x4C463C), 1)
		}
	} else {
		msg := a.viewerErr
		if msg == "" {
			msg = "没有可显示的图片"
		}
		drawText(hdc, msg,
			RECT{area.Left + int32(scale(30, a.dpi)), area.Top, area.Right - int32(scale(30, a.dpi)), area.Bottom},
			DT_CENTER|DT_VCENTER|DT_WORDBREAK, a.fonts.sub, muted)
	}

	n := 0
	if a.book != nil {
		n = len(a.book.Images)
	}
	prevLabel := "‹  上一张"
	nextLabel := "下一张  ›"
	a.paintViewerButton(hdc, a.viewerPrev, prevLabel, false, idViewerPrev)
	a.paintViewerButton(hdc, a.viewerNext, nextLabel, false, idViewerNext)
	counter := ""
	if n > 0 {
		counter = fmt.Sprintf("%d / %d", a.viewerIndex+1, n)
	}
	drawText(hdc, counter,
		RECT{a.viewerPrev.Right, a.viewerPrev.Top, a.viewerNext.Left, a.viewerPrev.Bottom},
		DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.ui, muted)
	drawText(hdc, "←  →  切换    Esc  返回阅读",
		RECT{int32(m), cr.Bottom - int32(scale(44, a.dpi)), cr.Right - int32(m), cr.Bottom - int32(scale(18, a.dpi))},
		DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.small, muted)
}

func (a *App) paintViewerButton(hdc uintptr, r RECT, label string, primary bool, id int) {
	if rectWidth(r) <= 0 || rectHeight(r) <= 0 {
		return
	}
	hover := a.hoverID == id
	pressed := a.pressID == id
	var bg, fg, border uint32
	if primary {
		bg = a.theme.Accent
		fg = a.theme.OnAccent
		if hover {
			bg = lerpColor(bg, colorRef(0xFFFFFF), 0.10)
		}
		if pressed {
			bg = lerpColor(bg, colorRef(0x000000), 0.12)
		}
	} else {
		bg = colorRef(0x2A2926)
		fg = colorRef(0xE7E0D4)
		border = colorRef(0x4A453D)
		if hover {
			bg = colorRef(0x393732)
		}
		if pressed {
			bg = colorRef(0x211F1C)
		}
	}
	fillRoundRect(hdc, r, scale(9, a.dpi), bg)
	if border != 0 {
		strokeRoundRect(hdc, r, scale(9, a.dpi), border, 1)
	}
	drawText(hdc, label, r, DT_CENTER|DT_VCENTER|DT_SINGLELINE, a.fonts.uiBold, fg)
}
