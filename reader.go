//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

type blockStyle struct {
	sizeFactor float64
	color      uint32
	bold       bool
	align      uint16
	start      int32
	right      int32
	offset     int32
	before     float64 // in em
	after      float64 // in em
}

func fontIndex(key string) int {
	switch key {
	case "sans":
		return 1
	case "kai":
		return 2
	default:
		return 0
	}
}

func (a *App) faceName() string {
	idx := fontIndex(a.settings.Font)
	if a.fontsOK[idx] {
		switch idx {
		case 1:
			return familySans
		case 2:
			return familyKai
		default:
			return familySerif
		}
	}
	switch idx {
	case 1:
		return "Microsoft YaHei UI"
	case 2:
		return "KaiTi"
	default:
		return "SimSun"
	}
}

func (a *App) uiFace() string {
	if a.fontsOK[1] {
		return familySans
	}
	return "Microsoft YaHei UI"
}

func (a *App) serifFace() string {
	if a.fontsOK[0] {
		return familySerif
	}
	return "SimSun"
}

func (a *App) setRichText(text []uint16) {
	send(a.rich, EM_EXLIMITTEXT, 0, 0x7FFFFFF0)
	buf := make([]uint16, len(text)+1)
	copy(buf, text)
	send(a.rich, WM_SETTEXT, 0, uintptr(unsafe.Pointer(&buf[0])))
}

func (a *App) selectRange(start, end int32) {
	cr := CHARRANGE{CpMin: start, CpMax: end}
	send(a.rich, EM_EXSETSEL, 0, uintptr(unsafe.Pointer(&cr)))
}

func (a *App) selection() (int32, int32) {
	var cr CHARRANGE
	send(a.rich, EM_EXGETSEL, 0, uintptr(unsafe.Pointer(&cr)))
	return cr.CpMin, cr.CpMax
}

func (a *App) setCharFormat(start, end int32, sizePt float64, color uint32, bold, italic bool) {
	cf := CHARFORMATW{
		CbSize:      uint32(unsafe.Sizeof(CHARFORMATW{})),
		DwMask:      CFM_FACE | CFM_SIZE | CFM_COLOR | CFM_BOLD | CFM_ITALIC | CFM_CHARSET,
		DwEffects:   0,
		YHeight:     int32(sizePt*20 + 0.5),
		CrTextColor: color,
		BCharSet:    DEFAULT_CHARSET,
	}
	if bold {
		cf.DwEffects |= CFE_BOLD
	}
	if italic {
		cf.DwEffects |= CFE_ITALIC
	}
	copyFace(&cf.SzFaceName, a.faceName())
	a.selectRange(start, end)
	send(a.rich, EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&cf)))
}

func copyFace(dst *[32]uint16, name string) {
	u := syscall.StringToUTF16(name)
	for i := 0; i < 31 && i < len(u)-1; i++ {
		dst[i] = u[i]
	}
	dst[31] = 0
}

func (a *App) setInline(start, end int32, bold, italic bool) {
	if bold {
		cf := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD, DwEffects: CFE_BOLD}
		a.selectRange(start, end)
		send(a.rich, EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&cf)))
	}
	if italic {
		cf := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_ITALIC, DwEffects: CFE_ITALIC}
		a.selectRange(start, end)
		send(a.rich, EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&cf)))
	}
}

func (a *App) setParagraph(start, end int32, st blockStyle) {
	p := PARAFORMAT2{
		CbSize:        uint32(unsafe.Sizeof(PARAFORMAT2{})),
		DwMask:        PFM_ALIGNMENT | PFM_STARTINDENT | PFM_RIGHTINDENT | PFM_OFFSET | PFM_SPACEBEFORE | PFM_SPACEAFTER,
		WAlignment:    st.align,
		DxStartIndent: st.start,
		DxRightIndent: st.right,
		DxOffset:      st.offset,
		DySpaceBefore: int32(float64(a.settings.FontSize*20) * st.before),
		DySpaceAfter:  int32(float64(a.settings.FontSize*20) * st.after),
	}
	a.selectRange(start, end)
	send(a.rich, EM_SETPARAFORMAT, 0, uintptr(unsafe.Pointer(&p)))
}

func (a *App) globalParagraph() {
	em := int32(a.settings.FontSize*20 + 0.5)
	p := PARAFORMAT2{
		CbSize:           uint32(unsafe.Sizeof(PARAFORMAT2{})),
		DwMask:           PFM_ALIGNMENT | PFM_STARTINDENT | PFM_RIGHTINDENT | PFM_OFFSET | PFM_SPACEBEFORE | PFM_SPACEAFTER | PFM_LINESPACING,
		WAlignment:       PFA_JUSTIFY,
		DxStartIndent:    em * 2,
		DxOffset:         -em * 2,
		DxRightIndent:    0,
		DySpaceBefore:    0,
		DySpaceAfter:     0,
		DyLineSpacing:    int32(a.settings.LineSpacing*20 + 0.5),
		BLineSpacingRule: 5,
	}
	a.selectRange(0, a.doc.Length)
	send(a.rich, EM_SETPARAFORMAT, 0, uintptr(unsafe.Pointer(&p)))
}

func (a *App) blockStyleFor(pb PositionedBlock) blockStyle {
	em := int32(a.settings.FontSize*20 + 0.5)
	base := blockStyle{
		sizeFactor: 1,
		color:      a.theme.Text,
		align:      PFA_JUSTIFY,
		start:      em * 2,
		offset:     -em * 2,
	}
	switch pb.Kind {
	case KindTitlePage:
		base.sizeFactor = 2.1
		base.bold = true
		base.align = PFA_CENTER
		base.start, base.offset = 0, 0
		base.before, base.after = 4.2, 1.2
	case KindAuthor:
		base.sizeFactor = 1.02
		base.align = PFA_CENTER
		base.color = a.theme.Muted
		base.start, base.offset = 0, 0
		base.before, base.after = 0.2, 0.2
	case KindMeta:
		base.sizeFactor = 0.88
		base.align = PFA_CENTER
		base.color = a.theme.Muted
		base.start, base.offset = 0, 0
		base.before, base.after = 0.15, 0.15
	case KindChapterTitle:
		base.sizeFactor = 1.42
		base.bold = true
		base.align = PFA_CENTER
		base.start, base.offset = 0, 0
		base.before, base.after = 2.2, 1.2
	case KindHeading1:
		base.sizeFactor = 1.22
		base.bold = true
		base.align = PFA_CENTER
		base.start, base.offset = 0, 0
		base.before, base.after = 1.7, 0.8
	case KindHeading2:
		base.sizeFactor = 1.1
		base.bold = true
		base.align = PFA_CENTER
		base.start, base.offset = 0, 0
		base.before, base.after = 1.2, 0.6
	case KindHeading3:
		base.sizeFactor = 1.02
		base.bold = true
		base.align = PFA_LEFT
		base.start, base.offset = 0, 0
		base.before, base.after = 1.0, 0.4
	case KindQuote:
		base.sizeFactor = 0.95
		base.color = a.theme.Muted
		base.start, base.offset, base.right = em*2, 0, em*2
		base.before, base.after = 0.25, 0.25
	case KindDivider:
		base.sizeFactor = 0.9
		base.color = a.theme.Muted
		base.align = PFA_CENTER
		base.start, base.offset = 0, 0
		base.before, base.after = 1.5, 1.5
	default:
		if pb.Align == AlignCenter {
			base.align = PFA_CENTER
			base.start, base.offset = 0, 0
		}
	}
	return base
}

func (a *App) formatAll() {
	if a.doc == nil || a.rich == 0 {
		return
	}
	// Reset everything to body style first, then re-apply the exceptions.
	a.setCharFormat(0, a.doc.Length, a.settings.FontSize, a.theme.Text, false, false)
	a.globalParagraph()
	for _, pb := range a.doc.Blocks {
		st := a.blockStyleFor(pb)
		special := pb.Kind != KindBody || pb.Align == AlignCenter
		if special {
			a.setCharFormat(pb.Start, pb.End, a.settings.FontSize*st.sizeFactor, st.color, st.bold, false)
			a.setParagraph(pb.Start, pb.End, st)
		}
		for _, run := range pb.Runs {
			if run.Bold || run.Italic {
				a.setInline(run.Start, run.End, run.Bold, run.Italic)
			}
		}
	}
	a.selectRange(0, 0)
}

func (a *App) setRichBackground() {
	send(a.rich, EM_SETBKGNDCOLOR, 0, uintptr(a.theme.Paper))
	themeName := "Explorer"
	if a.settings.Theme == 2 {
		themeName = "DarkMode_Explorer"
	}
	p, _ := utf16Ptr(themeName)
	procSetWindowTheme.Call(a.rich, p, 0)
}

func (a *App) applyDocument(doc *Document, restore float64) {
	a.doc = doc
	a.tocCurrent = -1
	a.tocScroll = 0
	send(a.rich, EM_SETREDRAW, 0, 0)
	a.setRichText(doc.Text)
	a.setRichBackground()
	a.formatAll()
	send(a.rich, EM_SETREDRAW, 1, 0)
	invalidateRect(a.rich, nil, false)
	a.updateTOC()
	a.layout()
	a.showRich()
	a.scrollToPercent(restore)
	a.updateStatus(true)
	a.hideCaret()
	a.invalidateAll()
}

func (a *App) applyTypography() {
	if a.doc == nil || a.rich == 0 {
		return
	}
	top := a.topChar()
	selMin, selMax := a.selection()
	send(a.rich, EM_SETREDRAW, 0, 0)
	a.setRichBackground()
	a.formatAll()
	send(a.rich, EM_SETREDRAW, 1, 0)
	a.selectRange(selMin, selMax)
	a.scrollToChar(top)
	invalidateRect(a.rich, nil, false)
	a.updateStatus(true)
	a.hideCaret()
	a.invalidateAll()
}

func (a *App) topChar() int32 {
	line := int32(send(a.rich, EM_GETFIRSTVISIBLELINE, 0, 0))
	if line < 0 {
		return 0
	}
	pos := int32(send(a.rich, EM_LINEINDEX, uintptr(line), ^uintptr(0)))
	if pos < 0 {
		return 0
	}
	if a.doc != nil && pos > a.doc.Length {
		return a.doc.Length
	}
	return pos
}

func (a *App) maxScrollY() int32 {
	var si SCROLLINFO
	si.CbSize = uint32(unsafe.Sizeof(si))
	si.FMask = SIF_ALL
	r, _, _ := procGetScrollInfo.Call(a.rich, SB_VERT, uintptr(unsafe.Pointer(&si)))
	if r == 0 || si.NMax <= si.NMin {
		return 0
	}
	maxY := si.NMax - int32(si.NPage) + 1
	if maxY < 0 {
		return 0
	}
	return maxY
}

func (a *App) setScrollY(y int32) {
	if maxY := a.maxScrollY(); y > maxY {
		y = maxY
	}
	if y < 0 {
		y = 0
	}
	pt := POINT{X: 0, Y: y}
	send(a.rich, EM_SETSCROLLPOS, 0, uintptr(unsafe.Pointer(&pt)))
}

func (a *App) scrollToChar(pos int32) {
	if a.doc == nil {
		return
	}
	if pos < 0 {
		pos = 0
	}
	if pos > a.doc.Length {
		pos = a.doc.Length
	}
	a.selectRange(pos, pos)
	line := int32(send(a.rich, EM_EXLINEFROMCHAR, 0, uintptr(pos)))
	if line <= 0 {
		a.setScrollY(0)
		return
	}
	// EM_LINESCROLL counts wrapped display lines while EM_EXLINEFROMCHAR uses
	// paragraph indices. Scroll position is monotonic with the first visible
	// paragraph, so binary-search the pixel position instead of guessing.
	maxY := a.maxScrollY()
	if maxY <= 0 {
		return
	}
	lo, hi := int32(0), maxY
	for i := 0; i < 24 && lo < hi; i++ {
		mid := lo + (hi-lo)/2
		a.setScrollY(mid)
		top := int32(send(a.rich, EM_GETFIRSTVISIBLELINE, 0, 0))
		if top < line {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	a.setScrollY(lo)
}

func (a *App) scrollPercent() float64 {
	if a.rich == 0 {
		return 0
	}
	var si SCROLLINFO
	si.CbSize = uint32(unsafe.Sizeof(si))
	si.FMask = SIF_ALL
	r, _, _ := procGetScrollInfo.Call(a.rich, SB_VERT, uintptr(unsafe.Pointer(&si)))
	if r != 0 && si.NMax > si.NMin {
		span := int64(si.NMax-si.NMin+1) - int64(si.NPage)
		if span <= 0 {
			return 0
		}
		p := float64(si.NPos-si.NMin) / float64(span)
		if p < 0 {
			return 0
		}
		if p > 1 {
			return 1
		}
		return p
	}
	lines := int32(send(a.rich, EM_GETLINECOUNT, 0, 0))
	top := int32(send(a.rich, EM_GETFIRSTVISIBLELINE, 0, 0))
	if lines <= 1 || top <= 0 {
		return 0
	}
	p := float64(top) / float64(lines-1)
	if p > 1 {
		p = 1
	}
	return p
}

func (a *App) scrollToPercent(p float64) {
	if a.doc == nil {
		return
	}
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	maxY := a.maxScrollY()
	a.setScrollY(int32(float64(maxY) * p))
}

func (a *App) currentChapter() int {
	if a.doc == nil || len(a.doc.Chapters) == 0 || a.rich == 0 {
		return -1
	}
	pos := a.topChar()
	if pos < a.doc.Chapters[0].Start {
		return -1
	}
	return a.doc.ChapterAt(pos)
}

func (a *App) jumpChapter(idx int) {
	if a.doc == nil || idx < 0 || idx >= len(a.doc.Chapters) {
		return
	}
	a.scrollToChar(a.doc.Chapters[idx].Start)
	a.tocCurrent = idx
	a.updateTOC()
	a.updateStatus(true)
	a.hideCaret()
	a.invalidateAll()
}

func (a *App) nextChapter(delta int) {
	cur := a.currentChapter()
	if cur < 0 {
		if delta > 0 {
			a.jumpChapter(0)
		}
		return
	}
	a.jumpChapter(cur + delta)
}

func (a *App) updateStatus(force bool) {
	if a.state != stateReading || a.doc == nil {
		return
	}
	p := a.scrollPercent()
	a.progress = p
	cur := a.currentChapter()
	left := a.book.Title
	right := "扉页"
	if cur >= 0 && cur < len(a.doc.Chapters) {
		left = a.doc.Chapters[cur].Title
		right = "第 " + itoa(cur+1) + " / " + itoa(len(a.doc.Chapters)) + " 章"
		if cur != a.tocCurrent {
			a.tocCurrent = cur
			a.ensureTOCVisible(cur)
		}
	} else if a.tocCurrent != -1 {
		a.tocCurrent = -1
		a.invalidateTOC()
	}
	right += " · " + itoa(int(p*100+0.5)) + "%"
	if force || left != a.statusLeft || right != a.statusRight {
		a.statusLeft, a.statusRight = left, right
		r := a.statusRect()
		invalidateRect(a.hwnd, &r, false)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func (a *App) statusRect() RECT {
	var r RECT
	procGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&r)))
	h := int32(scale(30, a.dpi))
	return RECT{0, r.Bottom - h, r.Right, r.Bottom}
}
