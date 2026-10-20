//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

type appState int

const (
	stateWelcome appState = iota
	stateLoading
	stateReading
	stateError
)

type fontSet struct {
	ui, uiBold, small, sub, toc, heading, brand, brandEn, watermark uintptr
}

type uiButton struct {
	ID   int
	Rect RECT
}

type loadResult struct {
	book      *Book
	err       error
	path      string
	seq       int
	showCover bool
}

type App struct {
	hwnd, rich, toc uintptr
	hInst, hIcon    uintptr
	hAccel          uintptr
	hRichLib        uintptr
	oldRichProc     uintptr

	dpi      int
	settings Settings
	theme    Theme
	fonts    fontSet
	fontsOK  [3]bool

	state      appState
	fullscreen bool
	savedStyle uintptr
	savedRect  RECT

	book *Book
	doc  *Document

	buttons          []uiButton
	welcomeButtons   []uiButton
	welcomePrimary   RECT
	welcomeSecondary RECT
	hasSecondary     bool
	hoverID          int
	pressID          int
	mouseTracked     bool

	tocCurrent int
	tocHover   int
	tocScroll  int

	menuHwnd    uintptr
	menuItems   []menuItem
	menuHover   int
	menuPress   int
	menuW       int
	menuH       int
	menuPad     int
	menuItemH   int
	menuSepH    int
	menuClosing bool

	viewerActive  bool
	viewerIndex   int
	viewerOpening bool
	viewerImage   *renderedImage
	viewerErr     string
	viewerClose   RECT
	viewerPrev    RECT
	viewerNext    RECT
	imageCache    map[int]*renderedImage
	richDown      bool
	richDownX     int
	richDownY     int

	dialogActive bool
	dialogTitle  string
	dialogBody   string
	dialogDanger bool
	dialogCard   RECT
	dialogOK     RECT
	dialogClose  RECT

	statusLeft, statusRight string
	progress                float64
	errMsg                  string
	loadSeq                 int
	loadingPath             string
	pending                 *loadResult
	mu                      sync.Mutex
	tick                    int
}

var (
	gApp        *App
	embeddedOK  [3]bool
	fontHandles []uintptr
	lineOptions = []float64{1.2, 1.4, 1.6, 1.7, 1.8, 2.0, 2.2, 2.5, 3.0}
	lineLabels  = []string{"紧凑 1.2", "舒适 1.4", "宽松 1.6", "标准 1.7", "疏朗 1.8", "大字 2.0", "宽松 2.2", "疏朗 2.5", "极宽 3.0"}
)

const (
	idOpen = 1001 + iota
	idTOC
	idFontMenu
	idFontSerif
	idFontSans
	idFontKai
	idFontMinus
	idFontPlus
	idLineMenu
	idThemeMenu
	idMore
	idAbout
	idHelp
	idExit
	idPrevChapter
	idNextChapter
	idZoomIn
	idZoomOut
	idZoomReset
	idFullscreen
	idContinue
	idRetry
	idEscape
	idDialogOK
	idDialogClose
	idCover
	idGallery
	idViewerClose
	idViewerPrev
	idViewerNext
)

const (
	idLineBase  = 2000
	idThemeBase = 2100
)

type menuItem struct {
	label    string
	id       int
	checked  bool
	disabled bool
	sep      bool
}

func registerEmbeddedFonts() [3]bool {
	var ok [3]bool
	for i, data := range [][]byte{fontSerif, fontSans, fontKai} {
		if len(data) == 0 {
			continue
		}
		var n uint32
		r, _, _ := procAddFontMemResourceEx.Call(
			uintptr(unsafe.Pointer(&data[0])),
			uintptr(len(data)),
			0,
			uintptr(unsafe.Pointer(&n)),
		)
		if r != 0 {
			ok[i] = true
			fontHandles = append(fontHandles, r)
		}
	}
	return ok
}

func newApp() *App {
	s := loadSettings()
	a := &App{
		settings:   s,
		dpi:        96,
		tocCurrent: -1,
		tocHover:   -1,
	}
	a.theme = themes()[s.Theme]
	a.fontsOK = embeddedOK
	return a
}

func setDPIAwareness() {
	r, _, _ := procSetProcessDpiAwarenessCtx.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2
	if r == 0 {
		procSetProcessDPIAware.Call()
	}
}

func (a *App) buildFonts() {
	a.deleteFonts()
	ui := a.uiFace()
	serif := a.serifFace()
	a.fonts = fontSet{
		ui:        createFont(10.5, FW_NORMAL, ui, a.dpi),
		uiBold:    createFont(10.5, FW_SEMIBOLD, ui, a.dpi),
		small:     createFont(9, FW_NORMAL, ui, a.dpi),
		sub:       createFont(11, FW_NORMAL, ui, a.dpi),
		toc:       createFont(11.5, FW_SEMIBOLD, ui, a.dpi),
		heading:   createFont(18, FW_SEMIBOLD, serif, a.dpi),
		brand:     createFont(46, FW_BOLD, serif, a.dpi),
		brandEn:   createFont(8.5, FW_NORMAL, ui, a.dpi),
		watermark: createFont(138, FW_BOLD, serif, a.dpi),
	}
}

func (a *App) deleteFonts() {
	for _, f := range []uintptr{a.fonts.ui, a.fonts.uiBold, a.fonts.small, a.fonts.sub, a.fonts.toc, a.fonts.heading, a.fonts.brand, a.fonts.brandEn, a.fonts.watermark} {
		deleteObject(f)
	}
	a.fonts = fontSet{}
}

func (a *App) create() bool {
	a.hInst, _, _ = procGetModuleHandleW.Call(0)
	a.hIcon, _, _ = procLoadIconW.Call(a.hInst, 1)
	if a.hIcon == 0 {
		a.hIcon, _, _ = procLoadIconW.Call(0, 32512)
	}
	arrow, _, _ := procLoadCursorW.Call(0, IDC_ARROW)
	mainClass, _ := utf16Ptr("MoYueMain")
	tocClass, _ := utf16Ptr("MoYueTOC")

	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         CS_HREDRAW | CS_VREDRAW | CS_DBLCLKS,
		LpfnWndProc:   syscall.NewCallback(mainWndProc),
		HInstance:     a.hInst,
		HIcon:         a.hIcon,
		HIconSm:       a.hIcon,
		HCursor:       arrow,
		LpszClassName: mainClass,
	}
	if r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return false
	}
	wc2 := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         CS_HREDRAW | CS_VREDRAW,
		LpfnWndProc:   syscall.NewCallback(tocWndProc),
		HInstance:     a.hInst,
		HCursor:       arrow,
		LpszClassName: tocClass,
	}
	if r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc2))); r == 0 {
		return false
	}
	menuClass, _ := utf16Ptr("MoYueMenu")
	wc3 := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         CS_HREDRAW | CS_VREDRAW | CS_DROPSHADOW,
		LpfnWndProc:   syscall.NewCallback(menuWndProc),
		HInstance:     a.hInst,
		HCursor:       arrow,
		LpszClassName: menuClass,
	}
	if r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc3))); r == 0 {
		return false
	}

	a.settings.normalize()
	sysDPI, _, _ := procGetDpiForSystem.Call()
	if sysDPI == 0 {
		sysDPI = 96
	}
	firstRun := a.settings.WindowX < 0 || a.settings.WindowY < 0
	w := a.settings.WindowW
	h := a.settings.WindowH
	if firstRun {
		w = scale(w, int(sysDPI))
		h = scale(h, int(sysDPI))
	}
	sw := int(getSystemMetrics(0))
	sh := int(getSystemMetrics(1))
	if w > sw-60 {
		w = sw - 60
	}
	if h > sh-60 {
		h = sh - 60
	}
	if w < scale(760, int(sysDPI)) && sw-60 >= scale(760, int(sysDPI)) {
		w = scale(760, int(sysDPI))
	}
	if h < scale(560, int(sysDPI)) && sh-60 >= scale(560, int(sysDPI)) {
		h = scale(560, int(sysDPI))
	}
	x, y := a.settings.WindowX, a.settings.WindowY
	if firstRun || x < -w+120 || y < 0 || x > sw-120 || y > sh-80 {
		x = (sw - w) / 2
		y = (sh - h) / 2
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	title, _ := utf16Ptr("墨阅")
	a.hwnd, _, _ = procCreateWindowExW.Call(
		WS_EX_APPWINDOW,
		mainClass,
		title,
		uintptr(WS_OVERLAPPEDWINDOW|WS_CLIPCHILDREN|WS_CLIPSIBLINGS),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		0, 0, a.hInst, 0,
	)
	if a.hwnd == 0 {
		return false
	}
	procShowWindow.Call(a.hwnd, SW_SHOW)
	procUpdateWindow.Call(a.hwnd)
	// Keyboard shortcuts are handled in WM_KEYDOWN so they also work
	// while focus is inside the RichEdit child window.
	if len(os.Args) > 1 {
		p := strings.TrimSpace(os.Args[1])
		if p != "" && !strings.HasPrefix(p, "-") {
			a.openBook(p, true)
		}
	}
	return true
}

func getSystemMetrics(index int) int32 {
	r, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int32(r)
}

func (a *App) createAccelerators() {
	accels := []ACCEL{
		{FVirt: 1 | 2, Key: 'O', Cmd: idOpen},
		{FVirt: 1 | 2, Key: 'T', Cmd: idTOC},
		{FVirt: 1 | 2, Key: 0xBB, Cmd: idZoomIn},  // Ctrl +
		{FVirt: 1 | 2, Key: 0x6B, Cmd: idZoomIn},  // Ctrl + (numpad)
		{FVirt: 1 | 2, Key: 0xBD, Cmd: idZoomOut}, // Ctrl -
		{FVirt: 1 | 2, Key: 0x6D, Cmd: idZoomOut}, // Ctrl - (numpad)
		{FVirt: 1 | 2, Key: '0', Cmd: idZoomReset},
		{FVirt: 1, Key: 0x7A, Cmd: idFullscreen}, // F11
		{FVirt: 1, Key: 0x70, Cmd: idHelp},       // F1
		{FVirt: 1 | 2, Key: 0x25, Cmd: idPrevChapter},
		{FVirt: 1 | 2, Key: 0x27, Cmd: idNextChapter},
		{FVirt: 1 | 4, Key: 0x25, Cmd: idPrevChapter},
		{FVirt: 1 | 4, Key: 0x27, Cmd: idNextChapter},
		{FVirt: 1, Key: 0x1B, Cmd: idEscape},
	}
	a.hAccel, _, _ = procCreateAcceleratorTableW.Call(uintptr(unsafe.Pointer(&accels[0])), uintptr(len(accels)))
}

func (a *App) run() {
	var msg MSG
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) == -1 || r == 0 {
			break
		}
		if a.hAccel != 0 {
			t, _, _ := procTranslateAcceleratorW.Call(a.hwnd, a.hAccel, uintptr(unsafe.Pointer(&msg)))
			if t != 0 {
				continue
			}
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func mainWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	a := gApp
	if a == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	switch msg {
	case WM_CREATE:
		a.hwnd = hwnd
		if !a.onCreate() {
			return ^uintptr(0)
		}
		return 0
	case WM_SIZE:
		if wParam != SIZE_MINIMIZED {
			a.layout()
			a.invalidateAll()
		}
		return 0
	case WM_DPICHANGED:
		a.dpi = int(hiword(wParam))
		a.buildFonts()
		if lParam != 0 {
			r := (*RECT)(unsafe.Pointer(lParam))
			procSetWindowPos.Call(hwnd, 0, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), SWP_NOZORDER|SWP_NOACTIVATE)
		}
		a.layout()
		a.applyTheme()
		a.invalidateAll()
		return 0
	case WM_GETMINMAXINFO:
		if !a.fullscreen && lParam != 0 {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
			mmi.PtMinTrackSize = POINT{int32(scale(760, a.dpi)), int32(scale(560, a.dpi))}
		}
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		a.onPaint()
		return 0
	case WM_KEYDOWN:
		if a.viewerActive {
			switch wParam {
			case 0x1B, 0x0D, 0x20:
				a.closeImageViewer()
			case 0x25:
				a.stepImageViewer(-1)
			case 0x27:
				a.stepImageViewer(1)
			}
			return 0
		}
		if a.dialogActive {
			if wParam == 0x1B || wParam == 0x0D || wParam == 0x20 {
				a.dismissDialog()
			}
			return 0
		}
		if id := a.shortcutFor(wParam); id != 0 {
			a.handleCommand(id)
			return 0
		}
		return 0
	case WM_MOUSEMOVE:
		a.onMouseMove(int(loword(lParam)), int(hiword(lParam)))
		return 0
	case WM_MOUSELEAVE:
		a.mouseTracked = false
		if a.hoverID != 0 {
			a.hoverID = 0
			a.invalidateToolbar()
		}
		return 0
	case WM_SETCURSOR:
		if loword(lParam) == 1 && a.hoverID != 0 {
			hand, _, _ := procLoadCursorW.Call(0, IDC_HAND)
			procSetCursor.Call(hand)
			return 1
		}
		return 0
	case WM_LBUTTONDOWN:
		a.onMouseDown(int(loword(lParam)), int(hiword(lParam)))
		return 0
	case WM_LBUTTONUP:
		a.onMouseUp(int(loword(lParam)), int(hiword(lParam)))
		return 0
	case WM_ACTIVATEAPP:
		if wParam == 0 {
			a.closeMenu(false)
		}
		return 0
	case WM_TIMER:
		a.onTimer()
		return 0
	case WM_COMMAND:
		if hiword(wParam) == 0 {
			a.handleCommand(int(wParam & 0xFFFF))
		}
		return 0
	case WM_APP_LOADED:
		a.onLoadDone(int(wParam))
		return 0
	case WM_APP_TOC_SELECT:
		procSetFocus.Call(a.rich)
		a.jumpChapter(int(wParam))
		a.hideCaret()
		return 0
	case WM_DROPFILES:
		a.onDropFiles(wParam)
		return 0
	case WM_CLOSE:
		a.closeMenu(false)
		a.saveState(true)
		procDestroyWindow.Call(hwnd)
		return 0
	case WM_DESTROY:
		procKillTimer.Call(hwnd, 1)
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func (a *App) onCreate() bool {
	a.dpi = int(func() uint32 {
		r, _, _ := procGetDpiForWindow.Call(a.hwnd)
		if r == 0 {
			return 96
		}
		return uint32(r)
	}())
	a.theme = themes()[a.settings.Theme]
	a.buildFonts()

	if !a.createRich() {
		a.messageBox("无法加载 Windows 富文本组件（Msftedit.dll）。\r\n请确认系统组件完整后重试。", "墨阅", MB_OK|MB_ICONERROR)
		return false
	}
	a.createTOC()
	procDragAcceptFiles.Call(a.hwnd, 1)
	procSetTimer.Call(a.hwnd, 1, 400, 0)
	a.applyTheme()
	a.layout()
	return true
}

func (a *App) createRich() bool {
	libName, _ := utf16Ptr("Msftedit.dll")
	lib, _, _ := procLoadLibraryW.Call(libName)
	class := "RICHEDIT50W"
	if lib == 0 {
		riche20, _ := utf16Ptr("Riched20.dll")
		lib, _, _ = procLoadLibraryW.Call(riche20)
		class = "RichEdit20W"
	}
	if lib == 0 {
		return false
	}
	a.hRichLib = lib
	clsPtr, _ := utf16Ptr(class)
	a.rich, _, _ = procCreateWindowExW.Call(
		0, clsPtr, 0,
		uintptr(WS_CHILD|WS_VSCROLL|ES_MULTILINE|ES_READONLY|ES_AUTOVSCROLL),
		0, 0, 10, 10, a.hwnd, 0, a.hInst, 0,
	)
	if a.rich == 0 {
		return false
	}
	// Replace the window procedure (GWLP_WNDPROC is -4).
	a.oldRichProc, _, _ = procSetWindowLongPtrW.Call(a.rich, ^uintptr(3), syscall.NewCallback(richWndProc))
	send(a.rich, EM_EXLIMITTEXT, 0, 0x7FFFFFF0)
	send(a.rich, EM_SETOPTIONS, ECOOP_OR, ECO_AUTOWORDSELECTION)
	a.setRichBackground()
	return true
}

func richWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	a := gApp
	if a != nil && a.rich != 0 {
		switch msg {
		case WM_MOUSEWHEEL:
			if wParam&MK_CONTROL != 0 {
				delta := int(int16((wParam >> 16) & 0xFFFF))
				step := delta / 120
				if step == 0 {
					if delta > 0 {
						step = 1
					} else if delta < 0 {
						step = -1
					}
				}
				if step != 0 {
					a.changeFontSize(step)
					return 0
				}
			}
		case WM_LBUTTONDOWN:
			a.richDown = true
			a.richDownX = int(loword(lParam))
			a.richDownY = int(hiword(lParam))
		case WM_LBUTTONUP:
			if a.richDown && !a.viewerActive && !a.dialogActive && a.doc != nil {
				x, y := int(loword(lParam)), int(hiword(lParam))
				dx, dy := x-a.richDownX, y-a.richDownY
				if dx*dx+dy*dy <= 16 {
					pt := POINT{X: int32(x), Y: int32(y)}
					pos := int32(send(a.rich, EM_CHARFROMPOS, 0, uintptr(unsafe.Pointer(&pt))))
					idx := a.doc.ImageAt(pos)
					if idx < 0 {
						// Some RichEdit versions pack line/character into the
						// return value; retry with the low word.
						idx = a.doc.ImageAt(pos & 0xFFFF)
					}
					if idx >= 0 {
						a.richDown = false
						a.showImageViewer(idx, false)
						return 0
					}
				}
			}
			a.richDown = false
		case WM_KEYDOWN:
			if id := a.shortcutFor(wParam); id != 0 {
				a.handleCommand(id)
				return 0
			}
		}
	}
	r, _, _ := procCallWindowProcW.Call(a.oldRichProc, hwnd, uintptr(msg), wParam, lParam)
	return r
}

func (a *App) hideCaret() {
	if a.rich != 0 {
		procHideCaret.Call(a.rich)
	}
}

func keyDown(vk uintptr) bool {
	r, _, _ := procGetKeyState.Call(vk)
	return uint16(r)&0x8000 != 0
}

func (a *App) shortcutFor(key uintptr) int {
	ctrl := keyDown(0x11) // VK_CONTROL
	alt := keyDown(0x12)  // VK_MENU
	if ctrl {
		switch key {
		case 'O':
			return idOpen
		case 'T':
			return idTOC
		case '0':
			return idZoomReset
		case 0xBB, 0x6B: // = / numpad +
			return idZoomIn
		case 0xBD, 0x6D: // - / numpad -
			return idZoomOut
		case 0x25:
			return idPrevChapter
		case 0x27:
			return idNextChapter
		}
	}
	if alt {
		switch key {
		case 0x25:
			return idPrevChapter
		case 0x27:
			return idNextChapter
		}
	}
	switch key {
	case 0x7A: // F11
		return idFullscreen
	case 0x70: // F1
		return idHelp
	case 0x1B:
		return idEscape
	}
	return 0
}

func (a *App) onMouseMove(x, y int) {
	if a.dialogActive {
		id := 0
		if pointIn(a.dialogOK, x, y) {
			id = idDialogOK
		} else if pointIn(a.dialogClose, x, y) {
			id = idDialogClose
		}
		if id != a.hoverID {
			a.hoverID = id
			a.invalidateAll()
		}
		return
	}
	id := a.hitTest(x, y)
	if id != a.hoverID {
		a.hoverID = id
		a.invalidateAll()
	}
	if !a.mouseTracked {
		tme := TRACKMOUSEEVENT{CbSize: uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})), DwFlags: 0x00000002, HwndTrack: a.hwnd}
		procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
		a.mouseTracked = true
	}
}

func (a *App) onMouseDown(x, y int) {
	if a.dialogActive {
		id := 0
		if pointIn(a.dialogOK, x, y) {
			id = idDialogOK
		} else if pointIn(a.dialogClose, x, y) {
			id = idDialogClose
		}
		if id != 0 {
			a.pressID = id
			procSetCapture.Call(a.hwnd)
			a.invalidateAll()
		}
		return
	}
	id := a.hitTest(x, y)
	if id == 0 {
		return
	}
	a.pressID = id
	procSetCapture.Call(a.hwnd)
	a.invalidateAll()
}

func (a *App) onMouseUp(x, y int) {
	if a.pressID == 0 {
		return
	}
	id := a.pressID
	a.pressID = 0
	procReleaseCapture.Call()
	if a.dialogActive {
		if (id == idDialogOK && pointIn(a.dialogOK, x, y)) || (id == idDialogClose && pointIn(a.dialogClose, x, y)) {
			a.dismissDialog()
		}
		a.invalidateAll()
		return
	}
	if a.hitTest(x, y) == id {
		a.handleCommand(id)
	}
	a.invalidateAll()
}

func (a *App) onTimer() {
	a.tick++
	if a.state == stateLoading || a.state == stateError || a.state == stateWelcome {
		a.invalidateAll()
	}
	if a.state == stateReading {
		a.updateStatus(false)
	}
}

func (a *App) hitTest(x, y int) int {
	if a.viewerActive {
		switch {
		case pointIn(a.viewerClose, x, y):
			return idViewerClose
		case pointIn(a.viewerPrev, x, y):
			return idViewerPrev
		case pointIn(a.viewerNext, x, y):
			return idViewerNext
		}
		return 0
	}
	for _, b := range a.buttons {
		if pointIn(b.Rect, x, y) {
			return b.ID
		}
	}
	for _, b := range a.welcomeButtons {
		if pointIn(b.Rect, x, y) {
			return b.ID
		}
	}
	return 0
}

func (a *App) buttonRect(id int) RECT {
	for _, b := range a.buttons {
		if b.ID == id {
			return b.Rect
		}
	}
	return RECT{}
}

func (a *App) handleCommand(id int) {
	switch {
	case id == idOpen || id == idRetry:
		a.openDialog()
	case id == idContinue:
		if a.settings.LastBook != "" {
			a.openBook(a.settings.LastBook)
		}
	case id == idTOC:
		a.toggleTOC()
	case id == idCover:
		if a.book != nil && a.book.CoverIndex >= 0 {
			a.showImageViewer(a.book.CoverIndex, false)
		}
	case id == idGallery:
		if a.book != nil && len(a.book.Images) > 0 {
			start := 0
			if a.book.CoverIndex >= 0 {
				start = a.book.CoverIndex
			}
			a.showImageViewer(start, false)
		}
	case id == idViewerClose:
		a.closeImageViewer()
	case id == idViewerPrev:
		a.stepImageViewer(-1)
	case id == idViewerNext:
		a.stepImageViewer(1)
	case id == idFontMenu:
		a.showFontMenu()
	case id == idFontSerif:
		a.setFont("serif")
	case id == idFontSans:
		a.setFont("sans")
	case id == idFontKai:
		a.setFont("kai")
	case id == idFontMinus || id == idZoomOut:
		a.changeFontSize(-1)
	case id == idFontPlus || id == idZoomIn:
		a.changeFontSize(1)
	case id == idZoomReset:
		a.changeFontSize(19 - int(a.settings.FontSize+0.5))
	case id == idLineMenu:
		a.showLineMenu()
	case id == idThemeMenu:
		a.showThemeMenu()
	case id == idMore:
		a.showMoreMenu()
	case id == idAbout:
		a.showAbout()
	case id == idHelp:
		a.showHelp()
	case id == idExit:
		procPostMessageW.Call(a.hwnd, WM_CLOSE, 0, 0)
	case id == idPrevChapter:
		a.nextChapter(-1)
	case id == idNextChapter:
		a.nextChapter(1)
	case id == idFullscreen:
		a.toggleFullscreen()
	case id == idEscape:
		if a.viewerActive {
			a.closeImageViewer()
		} else if a.fullscreen {
			a.toggleFullscreen()
		}
	case id >= idLineBase && id < idLineBase+len(lineOptions):
		a.setLineSpacing(lineOptions[id-idLineBase])
	case id >= idThemeBase && id < idThemeBase+3:
		a.setTheme(id - idThemeBase)
	}
}

func (a *App) setFont(key string) {
	if a.settings.Font == key {
		return
	}
	a.settings.Font = key
	a.applyTypography()
	a.invalidateToolbar()
}

func (a *App) changeFontSize(delta int) {
	if delta == 0 {
		return
	}
	n := a.settings.FontSize + float64(delta)
	if n < 13 {
		n = 13
	}
	if n > 40 {
		n = 40
	}
	if n == a.settings.FontSize {
		return
	}
	a.settings.FontSize = n
	a.applyTypography()
	a.invalidateToolbar()
}

func (a *App) setLineSpacing(v float64) {
	if v < 1.1 {
		v = 1.1
	}
	if v > 3.0 {
		v = 3.0
	}
	if v == a.settings.LineSpacing {
		return
	}
	a.settings.LineSpacing = v
	a.applyTypography()
	a.invalidateToolbar()
}

func (a *App) setTheme(i int) {
	if i < 0 || i > 2 || i == a.settings.Theme {
		return
	}
	a.settings.Theme = i
	a.theme = themes()[i]
	a.applyTheme()
	a.invalidateAll()
}

func (a *App) applyTheme() {
	if a.hwnd == 0 {
		return
	}
	dark := int32(0)
	if a.settings.Theme == 2 {
		dark = 1
	}
	procDwmSetWindowAttribute.Call(a.hwnd, 20, uintptr(unsafe.Pointer(&dark)), 4)
	caption := a.theme.Toolbar
	procDwmSetWindowAttribute.Call(a.hwnd, 35, uintptr(unsafe.Pointer(&caption)), 4)
	text := a.theme.Text
	procDwmSetWindowAttribute.Call(a.hwnd, 36, uintptr(unsafe.Pointer(&text)), 4)
	border := a.theme.Border
	procDwmSetWindowAttribute.Call(a.hwnd, 34, uintptr(unsafe.Pointer(&border)), 4)
	if a.rich != 0 {
		a.setRichBackground()
		if a.doc != nil {
			a.applyTypography()
		}
	}
	a.invalidateTOC()
}

func (a *App) toggleTOC() {
	if a.state != stateReading {
		return
	}
	a.settings.TOC = !a.settings.TOC
	a.layout()
	if a.settings.TOC {
		a.ensureTOCVisible(a.tocCurrent)
	}
	a.hideCaret()
	a.invalidateAll()
}

func (a *App) toggleFullscreen() {
	if !a.fullscreen {
		a.savedStyle, _, _ = procGetWindowLongPtrW.Call(a.hwnd, GWL_STYLE)
		procGetWindowRect.Call(a.hwnd, uintptr(unsafe.Pointer(&a.savedRect)))
		mon, _, _ := procMonitorFromWindow.Call(a.hwnd, MONITOR_DEFAULTTONEAREST)
		var mi MONITORINFO
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		monOK, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
		if mon == 0 || monOK == 0 {
			mi.RcMonitor = a.savedRect
		}
		a.fullscreen = true
		style := (a.savedStyle &^ uintptr(WS_OVERLAPPEDWINDOW)) | WS_POPUP | WS_VISIBLE
		procSetWindowLongPtrW.Call(a.hwnd, GWL_STYLE, style)
		procSetWindowPos.Call(a.hwnd, 0,
			uintptr(mi.RcMonitor.Left), uintptr(mi.RcMonitor.Top),
			uintptr(mi.RcMonitor.Right-mi.RcMonitor.Left), uintptr(mi.RcMonitor.Bottom-mi.RcMonitor.Top),
			SWP_NOZORDER|SWP_FRAMECHANGED|SWP_SHOWWINDOW)
	} else {
		a.fullscreen = false
		procSetWindowLongPtrW.Call(a.hwnd, GWL_STYLE, a.savedStyle)
		procSetWindowPos.Call(a.hwnd, 0,
			uintptr(a.savedRect.Left), uintptr(a.savedRect.Top),
			uintptr(a.savedRect.Right-a.savedRect.Left), uintptr(a.savedRect.Bottom-a.savedRect.Top),
			SWP_NOZORDER|SWP_FRAMECHANGED|SWP_SHOWWINDOW)
	}
	a.layout()
	a.applyTheme()
	a.invalidateAll()
	if a.state == stateReading {
		procSetFocus.Call(a.rich)
		a.hideCaret()
	}
}

func (a *App) openDialog() {
	filter := utf16.Encode([]rune("EPUB 电子书 (*.epub)\x00*.epub\x00所有文件 (*.*)\x00*.*\x00\x00"))
	filter = append(filter, 0)
	titleRunes := append(utf16.Encode([]rune("打开 EPUB 电子书")), 0)
	defExt := append(utf16.Encode([]rune("epub")), 0)
	buf := make([]uint16, 32768)
	ofn := OPENFILENAMEW{
		LStructSize:  uint32(unsafe.Sizeof(OPENFILENAMEW{})),
		HwndOwner:    a.hwnd,
		LpstrFilter:  uintptr(unsafe.Pointer(&filter[0])),
		NFilterIndex: 1,
		LpstrFile:    uintptr(unsafe.Pointer(&buf[0])),
		NMaxFile:     uint32(len(buf)),
		LpstrTitle:   uintptr(unsafe.Pointer(&titleRunes[0])),
		LpstrDefExt:  uintptr(unsafe.Pointer(&defExt[0])),
		Flags:        OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST | OFN_EXPLORER | OFN_NOCHANGEDIR,
	}
	r, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return
	}
	p := syscall.UTF16ToString(buf)
	if p == "" {
		return
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	a.openBook(p, true)
}

func (a *App) onDropFiles(hDrop uintptr) {
	var buf [32768]uint16
	r, _, _ := procDragQueryFileW.Call(hDrop, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r != 0 {
		p := syscall.UTF16ToString(buf[:])
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		procDragFinish.Call(hDrop)
		a.openBook(p, true)
		return
	}
	procDragFinish.Call(hDrop)
}

func (a *App) openBook(p string, showCover ...bool) {
	p = strings.TrimSpace(p)
	if p == "" {
		return
	}
	if a.book != nil {
		a.settings.setBookProgress(a.book.Path, a.progress)
	}
	wantCover := len(showCover) > 0 && showCover[0]
	a.loadSeq++
	seq := a.loadSeq
	a.state = stateLoading
	a.loadingPath = p
	a.errMsg = ""
	a.statusLeft, a.statusRight = "", ""
	a.updateWindowTitle("正在打开…")
	a.layout()
	a.invalidateAll()
	go func() {
		var b *Book
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("解析 EPUB 时发生错误: %v", r)
				}
			}()
			b, err = LoadEPUB(p)
		}()
		a.mu.Lock()
		a.pending = &loadResult{book: b, err: err, path: p, seq: seq, showCover: wantCover}
		a.mu.Unlock()
		procPostMessageW.Call(a.hwnd, WM_APP_LOADED, uintptr(seq), 0)
	}()
}

func (a *App) onLoadDone(seq int) {
	a.mu.Lock()
	res := a.pending
	a.pending = nil
	a.mu.Unlock()
	if res == nil || res.seq != seq || seq != a.loadSeq {
		return
	}
	if res.err != nil {
		if a.book != nil && a.doc != nil {
			a.state = stateReading
			a.layout()
			a.updateWindowTitle(a.book.Title)
			a.updateStatus(true)
			a.messageBox("无法打开这本书：\r\n"+res.err.Error(), "打开失败", MB_OK|MB_ICONERROR)
			return
		}
		a.state = stateError
		a.errMsg = res.err.Error()
		a.updateWindowTitle("打开失败")
		a.layout()
		a.invalidateAll()
		return
	}
	a.book = res.book
	a.imageCache = make(map[int]*renderedImage)
	doc := BuildDocument(a.book)
	p := a.settings.bookProgress(a.book.Path)
	a.state = stateReading
	a.applyDocument(doc, p)
	a.updateWindowTitle(a.book.Title)
	a.settings.LastBook = a.book.Path
	a.settings.LastTitle = a.book.Title
	a.progress = p
	a.saveState(false)
	procSetFocus.Call(a.rich)
	a.hideCaret()
	if res.showCover && a.book.CoverIndex >= 0 {
		a.showImageViewer(a.book.CoverIndex, true)
	}
}

func (a *App) updateWindowTitle(suffix string) {
	t := "墨阅"
	if suffix != "" {
		t = "墨阅 — " + suffix
	}
	p, _ := utf16Ptr(t)
	procSetWindowTextW.Call(a.hwnd, p)
}

func (a *App) saveState(includeWindow bool) {
	if a.book != nil {
		a.settings.setBookProgress(a.book.Path, a.progress)
		a.settings.LastBook = a.book.Path
		a.settings.LastTitle = a.book.Title
	}
	if includeWindow && !a.fullscreen && a.hwnd != 0 {
		var r RECT
		procGetWindowRect.Call(a.hwnd, uintptr(unsafe.Pointer(&r)))
		a.settings.WindowX = int(r.Left)
		a.settings.WindowY = int(r.Top)
		a.settings.WindowW = rectWidth(r)
		a.settings.WindowH = rectHeight(r)
	}
	_ = saveSettings(a.settings)
}

func (a *App) showFontMenu() {
	items := []menuItem{
		{label: "宋体 · 思源宋体", id: idFontSerif, checked: a.settings.Font == "serif"},
		{label: "黑体 · 思源黑体", id: idFontSans, checked: a.settings.Font == "sans"},
		{label: "楷体 · 霞鹜文楷", id: idFontKai, checked: a.settings.Font == "kai"},
	}
	a.showMenu(a.buttonRect(idFontMenu), items, false)
}

func (a *App) showLineMenu() {
	items := make([]menuItem, 0, len(lineOptions))
	for i, v := range lineOptions {
		items = append(items, menuItem{
			label:   lineLabels[i],
			id:      idLineBase + i,
			checked: absFloat(v-a.settings.LineSpacing) < 0.05,
		})
	}
	a.showMenu(a.buttonRect(idLineMenu), items, false)
}

func (a *App) showThemeMenu() {
	items := []menuItem{
		{label: "纸黄 · 温暖纸页", id: idThemeBase, checked: a.settings.Theme == 0},
		{label: "青瓷 · 护眼淡绿", id: idThemeBase + 1, checked: a.settings.Theme == 1},
		{label: "夜读 · 深色模式", id: idThemeBase + 2, checked: a.settings.Theme == 2},
	}
	a.showMenu(a.buttonRect(idThemeMenu), items, false)
}

func (a *App) showMoreMenu() {
	items := []menuItem{
		{label: "查看封面", id: idCover, disabled: a.book == nil || a.book.CoverIndex < 0},
		{label: "查看插图", id: idGallery, disabled: a.book == nil || len(a.book.Images) == 0},
		{sep: true},
		{label: "快捷键", id: idHelp},
		{label: "关于墨阅", id: idAbout},
		{sep: true},
		{label: "退出", id: idExit},
	}
	a.showMenu(a.buttonRect(idMore), items, true)
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func (a *App) messageBox(text, title string, flags uint32) {
	a.showDialog(title, text, flags&MB_ICONERROR != 0)
}

func (a *App) showAbout() {
	a.messageBox("墨阅  1.1.0\r\n轻量 EPUB 小说阅读器 · 单文件版\r\n\r\n"+
		"内嵌字体：思源宋体、思源黑体、霞鹜文楷\r\n"+
		"支持封面、正文插图查看与图片浏览。\r\n"+
		"为长篇中文阅读设计，无需安装，绿色运行。\r\n\r\n"+
		"字体版权归原作者所有，均以 SIL Open Font License 1.1 授权。", "关于墨阅", MB_OK|MB_ICONINFORMATION)
}

func (a *App) showHelp() {
	a.messageBox("Ctrl+O        打开 EPUB\r\n"+
		"Ctrl+T        显示 / 隐藏目录\r\n"+
		"Ctrl+=        放大字号\r\n"+
		"Ctrl+-        缩小字号\r\n"+
		"Ctrl+0        恢复默认字号\r\n"+
		"Ctrl+← / →    上一章 / 下一章\r\n"+
		"Alt+← / →     上一章 / 下一章\r\n"+
		"Ctrl+滚轮     调整字号\r\n"+
		"点击〔插图〕  打开封面 / 正文图片\r\n"+
		"← / →         切换图片\r\n"+
		"F11           全屏阅读（Esc 退出）\r\n"+
		"F1            显示本帮助\r\n\r\n"+
		"也可以把 EPUB 文件直接拖进窗口。", "快捷键", MB_OK|MB_ICONINFORMATION)
}

func (a *App) invalidateAll() {
	if a.hwnd != 0 {
		procInvalidateRect.Call(a.hwnd, 0, 0)
	}
}

func (a *App) invalidateToolbar() {
	if a.hwnd == 0 || a.fullscreen {
		if a.hwnd != 0 {
			procInvalidateRect.Call(a.hwnd, 0, 0)
		}
		return
	}
	r := RECT{0, 0, 10000, int32(scale(58, a.dpi))}
	procInvalidateRect.Call(a.hwnd, uintptr(unsafe.Pointer(&r)), 0)
}

func (a *App) invalidateTOC() {
	if a.toc != 0 {
		procInvalidateRect.Call(a.toc, 0, 0)
	}
}

func (a *App) tocVisible() bool {
	return a.settings.TOC && a.state == stateReading && !a.fullscreen && !a.dialogActive && !a.viewerActive
}

func (a *App) showRich() {
	if a.rich == 0 {
		return
	}
	if a.state == stateReading && !a.dialogActive && !a.viewerActive {
		procShowWindow.Call(a.rich, SW_SHOW)
	} else {
		procShowWindow.Call(a.rich, SW_HIDE)
	}
}

func (a *App) onPaint() {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(a.hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	var cr RECT
	procGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&cr)))
	w, h := rectWidth(cr), rectHeight(cr)
	if w <= 0 || h <= 0 {
		procEndPaint.Call(a.hwnd, uintptr(unsafe.Pointer(&ps)))
		return
	}
	mem, _, _ := procCreateCompatibleDC.Call(hdc)
	bmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	old, _, _ := procSelectObject.Call(mem, bmp)
	a.paint(mem, cr)
	procBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), mem, 0, 0, SRCCOPY)
	procSelectObject.Call(mem, old)
	procDeleteObject.Call(bmp)
	procDeleteDC.Call(mem)
	procEndPaint.Call(a.hwnd, uintptr(unsafe.Pointer(&ps)))
}
