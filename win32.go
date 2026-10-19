//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Win32 types
// ---------------------------------------------------------------------------

type POINT struct{ X, Y int32 }
type POINTL struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type SIZE struct{ CX, CY int32 }
type CHARRANGE struct{ CpMin, CpMax int32 }

type MSG struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  uintptr
	LpszClassName uintptr
	HIconSm       uintptr
}

type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type TRACKMOUSEEVENT struct {
	CbSize      uint32
	DwFlags     uint32
	HwndTrack   uintptr
	DwHoverTime uint32
}

type MONITORINFO struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
}

type MINMAXINFO struct {
	PtReserved     POINT
	PtMaxSize      POINT
	PtMaxPosition  POINT
	PtMinTrackSize POINT
	PtMaxTrackSize POINT
}

type OPENFILENAMEW struct {
	LStructSize       uint32
	HwndOwner         uintptr
	HInstance         uintptr
	LpstrFilter       uintptr
	LpstrCustomFilter uintptr
	NMaxCustFilter    uint32
	NFilterIndex      uint32
	LpstrFile         uintptr
	NMaxFile          uint32
	LpstrFileTitle    uintptr
	NMaxFileTitle     uint32
	LpstrInitialDir   uintptr
	LpstrTitle        uintptr
	Flags             uint32
	NFileOffset       uint16
	NFileExtension    uint16
	LpstrDefExt       uintptr
	LCustData         uintptr
	LpfnHook          uintptr
	LpTemplateName    uintptr
	PvReserved        uintptr
	DwReserved        uint32
	FlagsEx           uint32
}

type CHARFORMATW struct {
	CbSize          uint32
	DwMask          uint32
	DwEffects       uint32
	YHeight         int32
	YOffset         int32
	CrTextColor     uint32
	BCharSet        byte
	BPitchAndFamily byte
	SzFaceName      [32]uint16
}

type PARAFORMAT2 struct {
	CbSize           uint32
	DwMask           uint32
	WNumbering       uint16
	WReserved        uint16
	DxStartIndent    int32
	DxRightIndent    int32
	DxOffset         int32
	WAlignment       uint16
	CTabCount        int16
	RgxTabs          [32]int32
	DySpaceBefore    int32
	DySpaceAfter     int32
	DyLineSpacing    int32
	SStyle           int16
	BLineSpacingRule byte
	BCharSet         byte
	WShadingWeight   uint16
	WShadingStyle    uint16
	WNumberingStart  uint16
	WNumberingStyle  uint16
	WNumberingTab    uint16
	WBorderSpace     uint16
	WBorderWidth     uint16
	WBorders         uint16
}

type ACCEL struct {
	FVirt byte
	Key   uint16
	Cmd   uint16
}

type SCROLLINFO struct {
	CbSize    uint32
	FMask     uint32
	NMin      int32
	NMax      int32
	NPage     uint32
	NPos      int32
	NTrackPos int32
}

var (
	_ [int(unsafe.Sizeof(CHARFORMATW{})) - 92]byte
	_ [int(unsafe.Sizeof(PARAFORMAT2{})) - 188]byte
	_ [int(unsafe.Sizeof(MSG{})) - 48]byte
	_ [int(unsafe.Sizeof(OPENFILENAMEW{})) - 144]byte
	_ [int(unsafe.Sizeof(SCROLLINFO{})) - 28]byte
)

// ---------------------------------------------------------------------------
// DLL procedures
// ---------------------------------------------------------------------------

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	uxtheme  = syscall.NewLazyDLL("uxtheme.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
)

var (
	procRegisterClassExW          = user32.NewProc("RegisterClassExW")
	procCreateWindowExW           = user32.NewProc("CreateWindowExW")
	procDefWindowProcW            = user32.NewProc("DefWindowProcW")
	procDestroyWindow             = user32.NewProc("DestroyWindow")
	procShowWindow                = user32.NewProc("ShowWindow")
	procUpdateWindow              = user32.NewProc("UpdateWindow")
	procGetMessageW               = user32.NewProc("GetMessageW")
	procTranslateMessage          = user32.NewProc("TranslateMessage")
	procDispatchMessageW          = user32.NewProc("DispatchMessageW")
	procPostQuitMessage           = user32.NewProc("PostQuitMessage")
	procPostMessageW              = user32.NewProc("PostMessageW")
	procSendMessageW              = user32.NewProc("SendMessageW")
	procSetWindowTextW            = user32.NewProc("SetWindowTextW")
	procGetClientRect             = user32.NewProc("GetClientRect")
	procGetWindowRect             = user32.NewProc("GetWindowRect")
	procSetWindowPos              = user32.NewProc("SetWindowPos")
	procInvalidateRect            = user32.NewProc("InvalidateRect")
	procBeginPaint                = user32.NewProc("BeginPaint")
	procEndPaint                  = user32.NewProc("EndPaint")
	procGetDC                     = user32.NewProc("GetDC")
	procReleaseDC                 = user32.NewProc("ReleaseDC")
	procLoadCursorW               = user32.NewProc("LoadCursorW")
	procLoadIconW                 = user32.NewProc("LoadIconW")
	procSetCursor                 = user32.NewProc("SetCursor")
	procSetCapture                = user32.NewProc("SetCapture")
	procReleaseCapture            = user32.NewProc("ReleaseCapture")
	procGetSystemMetrics          = user32.NewProc("GetSystemMetrics")
	procClientToScreen            = user32.NewProc("ClientToScreen")
	procSetTimer                  = user32.NewProc("SetTimer")
	procKillTimer                 = user32.NewProc("KillTimer")
	procMessageBoxW               = user32.NewProc("MessageBoxW")
	procSetFocus                  = user32.NewProc("SetFocus")
	procHideCaret                 = user32.NewProc("HideCaret")
	procSetWindowLongPtrW         = user32.NewProc("SetWindowLongPtrW")
	procGetWindowLongPtrW         = user32.NewProc("GetWindowLongPtrW")
	procCallWindowProcW           = user32.NewProc("CallWindowProcW")
	procGetDpiForWindow           = user32.NewProc("GetDpiForWindow")
	procSetProcessDPIAware        = user32.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwarenessCtx = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetDpiForSystem           = user32.NewProc("GetDpiForSystem")
	procTrackMouseEvent           = user32.NewProc("TrackMouseEvent")
	procTranslateAcceleratorW     = user32.NewProc("TranslateAcceleratorW")
	procCreateAcceleratorTableW   = user32.NewProc("CreateAcceleratorTableW")
	procDestroyAcceleratorTable   = user32.NewProc("DestroyAcceleratorTable")
	procSetForegroundWindow       = user32.NewProc("SetForegroundWindow")
	procGetKeyState               = user32.NewProc("GetKeyState")
	procCreatePopupMenu           = user32.NewProc("CreatePopupMenu")
	procAppendMenuW               = user32.NewProc("AppendMenuW")
	procTrackPopupMenu            = user32.NewProc("TrackPopupMenu")
	procDestroyMenu               = user32.NewProc("DestroyMenu")
	procSetMenuDefaultItem        = user32.NewProc("SetMenuDefaultItem")
	procMonitorFromWindow         = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW           = user32.NewProc("GetMonitorInfoW")
	procGetScrollInfo             = user32.NewProc("GetScrollInfo")
	procSetScrollInfo             = user32.NewProc("SetScrollInfo")
	procSetWindowTheme            = uxtheme.NewProc("SetWindowTheme")
	procDwmSetWindowAttribute     = dwmapi.NewProc("DwmSetWindowAttribute")
	procDwmExtendFrameIntoClient  = dwmapi.NewProc("DwmExtendFrameIntoClientArea")

	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procCreatePen              = gdi32.NewProc("CreatePen")
	procRoundRect              = gdi32.NewProc("RoundRect")
	procRectangle              = gdi32.NewProc("Rectangle")
	procEllipse                = gdi32.NewProc("Ellipse")
	procFillRect               = user32.NewProc("FillRect")
	procFrameRect              = user32.NewProc("FrameRect")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procTextOutW               = gdi32.NewProc("TextOutW")
	procDrawTextW              = user32.NewProc("DrawTextW")
	procGetTextExtentPoint32W  = gdi32.NewProc("GetTextExtentPoint32W")
	procCreateFontW            = gdi32.NewProc("CreateFontW")
	procAddFontMemResourceEx   = gdi32.NewProc("AddFontMemResourceEx")
	procGetStockObject         = gdi32.NewProc("GetStockObject")
	procMoveToEx               = gdi32.NewProc("MoveToEx")
	procLineTo                 = gdi32.NewProc("LineTo")
	procSetPixel               = gdi32.NewProc("SetPixel")
	procArc                    = gdi32.NewProc("Arc")

	procGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")

	procDragAcceptFiles = shell32.NewProc("DragAcceptFiles")
	procDragQueryFileW  = shell32.NewProc("DragQueryFileW")
	procDragFinish      = shell32.NewProc("DragFinish")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procLoadLibraryW     = kernel32.NewProc("LoadLibraryW")
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_POPUP            = 0x80000000
	WS_CHILD            = 0x40000000
	WS_VISIBLE          = 0x10000000
	WS_VSCROLL          = 0x00200000
	WS_CLIPCHILDREN     = 0x02000000
	WS_CLIPSIBLINGS     = 0x04000000
	WS_EX_APPWINDOW     = 0x00040000

	ES_MULTILINE   = 0x0004
	ES_AUTOVSCROLL = 0x0040
	ES_NOHIDESEL   = 0x0100
	ES_READONLY    = 0x0800

	CS_HREDRAW = 0x0002
	CS_VREDRAW = 0x0001
	CS_DBLCLKS = 0x0008

	WM_CREATE         = 0x0001
	WM_SETTEXT        = 0x000C
	WM_NULL           = 0x0000
	WM_ACTIVATEAPP    = 0x001C
	WM_DESTROY        = 0x0002
	WM_SIZE           = 0x0005
	WM_SETFOCUS       = 0x0007
	WM_KILLFOCUS      = 0x0008
	WM_PAINT          = 0x000F
	WM_CLOSE          = 0x0010
	WM_ERASEBKGND     = 0x0014
	WM_SETCURSOR      = 0x0020
	WM_GETMINMAXINFO  = 0x0024
	WM_COMMAND        = 0x0111
	WM_KEYDOWN        = 0x0100
	WM_TIMER          = 0x0113
	WM_MOUSEMOVE      = 0x0200
	WM_LBUTTONDOWN    = 0x0201
	WM_LBUTTONUP      = 0x0202
	WM_MOUSEWHEEL     = 0x020A
	WM_DROPFILES      = 0x0233
	WM_DPICHANGED     = 0x02E0
	WM_MOUSELEAVE     = 0x02A3
	WM_CAPTURECHANGED = 0x02A2
	WM_APP            = 0x8000
	WM_APP_LOADED     = WM_APP + 1
	WM_APP_TOC_SELECT = WM_APP + 2

	MK_CONTROL = 0x0008

	SW_SHOWNORMAL = 1
	SW_SHOW       = 5
	SW_HIDE       = 0

	CS_DROPSHADOW = 0x00020000

	IDC_ARROW = 32512
	IDC_HAND  = 32649
	IDC_IBEAM = 32513
	IDC_WAIT  = 32514

	COLOR_WINDOW = 5

	DT_LEFT         = 0x0000
	DT_CENTER       = 0x0001
	DT_RIGHT        = 0x0002
	DT_VCENTER      = 0x0004
	DT_SINGLELINE   = 0x0020
	DT_WORDBREAK    = 0x0010
	DT_END_ELLIPSIS = 0x8000
	DT_NOPREFIX     = 0x0800
	DT_CALCRECT     = 0x0400
	DT_TOP          = 0x0000

	SRCCOPY     = 0x00CC0020
	TRANSPARENT = 1

	PS_SOLID = 0
	PS_NULL  = 5

	FW_NORMAL   = 400
	FW_SEMIBOLD = 600
	FW_BOLD     = 700

	DEFAULT_CHARSET   = 1
	CLEARTYPE_QUALITY = 5
	DEFAULT_PITCH     = 0
	FF_DONTCARE       = 0

	SWP_NOSIZE       = 0x0001
	SWP_NOMOVE       = 0x0002
	SWP_NOZORDER     = 0x0004
	SWP_NOACTIVATE   = 0x0010
	SWP_FRAMECHANGED = 0x0020
	SWP_SHOWWINDOW   = 0x0040

	GWL_STYLE     = ^uintptr(15) // -16
	GWL_EXSTYLE   = ^uintptr(19) // -20
	GWLP_USERDATA = ^uintptr(20) // -21

	MONITOR_DEFAULTTONEAREST = 2

	OFN_READONLY      = 0x00000001
	OFN_EXPLORER      = 0x00080000
	OFN_FILEMUSTEXIST = 0x00001000
	OFN_PATHMUSTEXIST = 0x00000800
	OFN_NOCHANGEDIR   = 0x00000008

	MB_OK              = 0x00000000
	MB_ICONERROR       = 0x00000010
	MB_ICONINFORMATION = 0x00000040

	MF_STRING       = 0x00000000
	MF_CHECKED      = 0x00000008
	MF_SEPARATOR    = 0x00000800
	MF_GRAYED       = 0x00000001
	TPM_RETURNCMD   = 0x0100
	TPM_RIGHTALIGN  = 0x0008
	TPM_LEFTALIGN   = 0x0000
	TPM_TOPALIGN    = 0x0000
	TPM_BOTTOMALIGN = 0x0020

	SIZE_MINIMIZED = 1

	CFM_BOLD      = 0x00000001
	CFM_ITALIC    = 0x00000002
	CFM_UNDERLINE = 0x00000004
	CFM_COLOR     = 0x40000000
	CFM_SIZE      = 0x80000000
	CFM_FACE      = 0x20000000
	CFM_CHARSET   = 0x08000000
	CFE_BOLD      = 0x00000001
	CFE_ITALIC    = 0x00000002

	SCF_SELECTION = 0x0001
	SCF_ALL       = 0x0004

	PFM_STARTINDENT = 0x00000001
	PFM_RIGHTINDENT = 0x00000002
	PFM_OFFSET      = 0x00000004
	PFM_ALIGNMENT   = 0x00000008
	PFM_SPACEBEFORE = 0x00000040
	PFM_SPACEAFTER  = 0x00000080
	PFM_LINESPACING = 0x00000100

	PFA_LEFT    = 1
	PFA_RIGHT   = 2
	PFA_CENTER  = 3
	PFA_JUSTIFY = 4

	EM_GETSEL               = 0x00B0
	EM_EXSETSEL             = WM_USER + 55
	EM_EXGETSEL             = WM_USER + 52
	EM_EXLINEFROMCHAR       = WM_USER + 54
	EM_POSFROMCHAR          = WM_USER + 38
	EM_LINEINDEX            = 0x00BB
	EM_LINEFROMCHAR         = 0x00C9
	EM_LINESCROLL           = 0x00B6
	EM_SCROLLCARET          = 0x00B7
	EM_GETLINECOUNT         = 0x00BA
	EM_GETFIRSTVISIBLELINE  = 0x00CE
	EM_SETCHARFORMAT        = WM_USER + 68
	EM_SETPARAFORMAT        = WM_USER + 71
	EM_SETBKGNDCOLOR        = WM_USER + 67
	EM_SETOPTIONS           = WM_USER + 77
	EM_EXLIMITTEXT          = WM_USER + 53
	EM_SETREDRAW            = 0x00B1
	EM_SETTYPOGRAPHYOPTIONS = WM_USER + 202
	TO_ADVANCEDTYPOGRAPHY   = 0x0001
	TO_SIMPLELINEBREAK      = 0x0002
	TO_ADVANCEDLAYOUT       = 0x0008
	EM_GETSCROLLPOS         = WM_USER + 221
	EM_SETSCROLLPOS         = WM_USER + 222
	ECOOP_OR                = 2
	ECO_AUTOWORDSELECTION   = 0x00000002

	WM_USER = 0x0400

	ST_UNICODE = 8

	SB_VERT   = 1
	SIF_RANGE = 0x0001
	SIF_PAGE  = 0x0002
	SIF_POS   = 0x0004
	SIF_ALL   = SIF_RANGE | SIF_PAGE | SIF_POS
)

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

func rgb(r, g, b uint32) uint32 { return r | g<<8 | b<<16 }

func utf16Ptr(s string) (uintptr, []uint16) {
	u := syscall.StringToUTF16(s)
	return uintptr(unsafe.Pointer(&u[0])), u
}

func loword(v uintptr) int32 { return int32(v & 0xFFFF) }
func hiword(v uintptr) int32 { return int32((v >> 16) & 0xFFFF) }
func makeint(a, b int32) uintptr {
	return uintptr(uint32(a&0xFFFF) | uint32(b&0xFFFF)<<16)
}

func send(hwnd uintptr, msg uint32, w, l uintptr) uintptr {
	r, _, _ := procSendMessageW.Call(hwnd, uintptr(msg), w, l)
	return r
}

func post(hwnd uintptr, msg uint32, w, l uintptr) bool {
	r, _, _ := procPostMessageW.Call(hwnd, uintptr(msg), w, l)
	return r != 0
}

func scale(v int, dpi int) int {
	return v * dpi / 96
}

func scaleF(v float64, dpi int) int {
	return int(v*float64(dpi)/96 + 0.5)
}

func invalidateRect(hwnd uintptr, r *RECT, erase bool) {
	var p uintptr
	if r != nil {
		p = uintptr(unsafe.Pointer(r))
	}
	e := uintptr(0)
	if erase {
		e = 1
	}
	procInvalidateRect.Call(hwnd, p, e)
}

func ptrU16(s string) (uintptr, []uint16) {
	u := syscall.StringToUTF16(s)
	return uintptr(unsafe.Pointer(&u[0])), u
}
