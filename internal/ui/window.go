package ui

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

var (
	modUser32 = syscall.NewLazyDLL("user32.dll")

	procSetProcessDpiAwarenessContext = modUser32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = modUser32.NewProc("SetProcessDPIAware")
	procRegisterClassExW              = modUser32.NewProc("RegisterClassExW")
	procCreateWindowExW               = modUser32.NewProc("CreateWindowExW")
	procDefWindowProcW                = modUser32.NewProc("DefWindowProcW")
	procDestroyWindow                 = modUser32.NewProc("DestroyWindow")
	procShowWindow                    = modUser32.NewProc("ShowWindow")
	procUpdateWindow                  = modUser32.NewProc("UpdateWindow")
	procSetWindowPos                  = modUser32.NewProc("SetWindowPos")
	procSetForegroundWindow           = modUser32.NewProc("SetForegroundWindow")
	procGetForegroundWindow           = modUser32.NewProc("GetForegroundWindow")
	procSetLayeredWindowAttrs         = modUser32.NewProc("SetLayeredWindowAttributes")
	procGetWindowLongW                = modUser32.NewProc("GetWindowLongW")
	procSetWindowLongW                = modUser32.NewProc("SetWindowLongW")
	procGetWindowRect                 = modUser32.NewProc("GetWindowRect")
	procGetClientRect                 = modUser32.NewProc("GetClientRect")
	procClientToScreen                = modUser32.NewProc("ClientToScreen")
	procScreenToClient                = modUser32.NewProc("ScreenToClient")
	procGetCursorPos                  = modUser32.NewProc("GetCursorPos")
	procMonitorFromWindow             = modUser32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW               = modUser32.NewProc("GetMonitorInfoW")
	procGetDC                         = modUser32.NewProc("GetDC")
	procReleaseDC                     = modUser32.NewProc("ReleaseDC")
	procGetSystemMetrics              = modUser32.NewProc("GetSystemMetrics")
	procEnumWindows                   = modUser32.NewProc("EnumWindows")
	procGetWindowThreadProcessId      = modUser32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible               = modUser32.NewProc("IsWindowVisible")
	procIsWindow                      = modUser32.NewProc("IsWindow")
	procIsIconic                      = modUser32.NewProc("IsIconic")
	procPeekMessageW                  = modUser32.NewProc("PeekMessageW")
	procTranslateMessage              = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW              = modUser32.NewProc("DispatchMessageW")
	procPostQuitMessage               = modUser32.NewProc("PostQuitMessage")
	procGetAsyncKeyState              = modUser32.NewProc("GetAsyncKeyState")
	procBeginPaint                    = modUser32.NewProc("BeginPaint")
	procEndPaint                      = modUser32.NewProc("EndPaint")
	procFillRect                      = modUser32.NewProc("FillRect")
)

const (
	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TRANSPARENT = 0x00000020
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_LAYERED     = 0x00080000

	WS_POPUP       = 0x80000000
	WS_VISIBLE     = 0x10000000
	WS_CAPTION     = 0x00C00000
	WS_THICKFRAME  = 0x00040000
	WS_MINIMIZEBOX = 0x00020000
	WS_MAXIMIZEBOX = 0x00010000
	WS_SYSMENU     = 0x00080000

	GWL_STYLE   = ^uintptr(15) // -16
	GWL_EXSTYLE = ^uintptr(19) // -20

	HWND_TOPMOST = ^uintptr(0) // -1
	HWND_TOP     = uintptr(0)

	DPI_AWARENESS_CONTEXT_PM_V2 = ^uintptr(3) // -4

	MONITOR_DEFAULTTONEAREST = 2

	LWA_COLORKEY = 0x00000001

	SW_HIDE   = 0
	SW_SHOW   = 5
	SW_SHOWNA = 8

	SWP_NOSIZE        = 0x0001
	SWP_NOMOVE        = 0x0002
	SWP_NOZORDER      = 0x0004
	SWP_NOACTIVATE    = 0x0010
	SWP_FRAMECHANGED  = 0x0020
	SWP_SHOWWINDOW    = 0x0040
	SWP_NOOWNERZORDER = 0x0200

	WM_DESTROY       = 0x0002
	WM_PAINT         = 0x000F
	WM_QUIT          = 0x0012
	WM_ERASEBKGND    = 0x0014
	WM_LBUTTONDOWN   = 0x0201
	WM_LBUTTONDBLCLK = 0x0203

	PM_REMOVE = 0x0001

	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	VK_TAB       = 0x09
	VK_RETURN    = 0x0D
	VK_ESCAPE    = 0x1B
	VK_PRIOR     = 0x21 // Page Up
	VK_NEXT      = 0x22 // Page Down
	VK_END       = 0x23
	VK_UP        = 0x26
	VK_DOWN      = 0x28
	VK_INSERT    = 0x2D
	VK_F1        = 0x70
	VK_F4        = 0x73
	VK_F5        = 0x74
	VK_F6        = 0x75
	VK_F7        = 0x76
	VK_F8        = 0x77
	VK_F9        = 0x78
	VK_F10       = 0x79
	VK_F11       = 0x7A
	VK_F12       = 0x7B
	VK_ADD       = 0x6B // Numpad +
	VK_SUBTRACT  = 0x6D // Numpad -
	VK_OEM_PLUS  = 0xBB // =/+
	VK_OEM_MINUS = 0xBD // -/_
	VK_OEM_4     = 0xDB // [
	VK_OEM_6     = 0xDD // ]
)

type RECT struct {
	Left, Top, Right, Bottom int32
}

type POINT struct {
	X, Y int32
}

type SIZE struct {
	CX, CY int32
}

type MONITORINFO struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
}

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type PAINTSTRUCT struct {
	Hdc         uintptr
	Erase       int32
	RcPaint     RECT
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

type WNDCLASSEXW struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

var (
	clickMu      sync.Mutex
	pendingClick *POINT
	pendingDbl   bool
)

func overlayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_LBUTTONDOWN, WM_LBUTTONDBLCLK:
		x := int32(int16(lParam & 0xFFFF))
		y := int32(int16((lParam >> 16) & 0xFFFF))
		clickMu.Lock()
		pendingClick = &POINT{X: x, Y: y}
		pendingDbl = (msg == WM_LBUTTONDBLCLK)
		clickMu.Unlock()
		return 0
	case WM_PAINT:
		var ps PAINTSTRUCT
		procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

var (
	searchPID uint32
	bestHwnd  uintptr
	maxArea   int32

	enumWindowsCallback = syscall.NewCallback(enumWindowsProc)
)

func enumWindowsProc(hwnd uintptr, lparam uintptr) uintptr {
	var wndPid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&wndPid)))
	if wndPid == searchPID {
		vis, _, _ := procIsWindowVisible.Call(hwnd)
		if vis != 0 {
			var rc RECT
			procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
			w := rc.Right - rc.Left
			h := rc.Bottom - rc.Top
			if w > 300 && h > 200 {
				area := w * h
				if area > maxArea {
					maxArea = area
					bestHwnd = hwnd
				}
			}
		}
	}
	return 1
}

func IsWindowValid(hwnd uintptr, pid uint32) bool {
	if hwnd == 0 {
		return false
	}
	ret, _, _ := procIsWindow.Call(hwnd)
	if ret == 0 {
		return false
	}
	var wndPid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&wndPid)))
	if wndPid != pid {
		return false
	}
	vis, _, _ := procIsWindowVisible.Call(hwnd)
	if vis == 0 {
		return false
	}
	var rc RECT
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	return (rc.Right-rc.Left) > 300 && (rc.Bottom-rc.Top) > 200
}

func FindGameWindow(pid uint32) uintptr {
	searchPID = pid
	bestHwnd = 0
	maxArea = 0
	procEnumWindows.Call(enumWindowsCallback, 0)
	return bestHwnd
}

func EnableDPIAwareness() {
	if procSetProcessDpiAwarenessContext.Find() == nil {
		procSetProcessDpiAwarenessContext.Call(DPI_AWARENESS_CONTEXT_PM_V2)
	} else if procSetProcessDPIAware.Find() == nil {
		procSetProcessDPIAware.Call()
	}
}

func SetOverlayClickThrough(hwnd uintptr, clickThrough bool) {
	exStyle, _, _ := procGetWindowLongW.Call(hwnd, GWL_EXSTYLE)
	if clickThrough {
		exStyle |= WS_EX_TRANSPARENT
	} else {
		exStyle &^= WS_EX_TRANSPARENT
	}
	procSetWindowLongW.Call(hwnd, GWL_EXSTYLE, exStyle)
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0,
		SWP_NOMOVE|SWP_NOSIZE|SWP_NOZORDER|SWP_NOACTIVATE|SWP_FRAMECHANGED)
}

func IsGameFocused(gameHwnd, overlayHwnd uintptr, scoreboardOpen bool) bool {
	resIconic, _, _ := procIsIconic.Call(gameHwnd)
	fgHwnd, _, _ := procGetForegroundWindow.Call()
	return resIconic == 0 && (fgHwnd == gameHwnd || (scoreboardOpen && fgHwnd == overlayHwnd))
}

func GetClientBounds(hwnd uintptr) (x, y, w, h int32) {
	var clientRect RECT
	var clientPoint POINT
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&clientRect)))
	procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&clientPoint)))
	return clientPoint.X, clientPoint.Y, clientRect.Right - clientRect.Left, clientRect.Bottom - clientRect.Top
}

func GetCursorClientPos(hwnd uintptr) POINT {
	var curPt POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&curPt)))
	procScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&curPt)))
	return curPt
}

func PopPendingClick() (*POINT, bool) {
	clickMu.Lock()
	pt := pendingClick
	dbl := pendingDbl
	pendingClick = nil
	pendingDbl = false
	clickMu.Unlock()
	return pt, dbl
}

func ClearPendingClicks() {
	clickMu.Lock()
	pendingClick = nil
	pendingDbl = false
	clickMu.Unlock()
}

type KeyTracker struct {
	state map[int]bool
}

func NewKeyTracker() *KeyTracker {
	return &KeyTracker{state: make(map[int]bool)}
}

func (k *KeyTracker) Clear() {
	for vk := range k.state {
		delete(k.state, vk)
	}
}

func (k *KeyTracker) IsPressed(vk int) bool {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	pressed := (ret & 0x8000) != 0
	if pressed {
		if !k.state[vk] {
			k.state[vk] = true
			return true
		}
	} else {
		k.state[vk] = false
	}
	return false
}

type BorderlessState struct {
	Active    bool
	Hwnd      uintptr
	OrigStyle uintptr
	OrigRect  RECT
}

func (b *BorderlessState) Restore() {
	if !b.Active || b.Hwnd == 0 {
		b.Active = false
		b.Hwnd = 0
		return
	}
	if vis, _, _ := procIsWindow.Call(b.Hwnd); vis != 0 {
		procSetWindowLongW.Call(b.Hwnd, GWL_STYLE, b.OrigStyle)
		w := b.OrigRect.Right - b.OrigRect.Left
		h := b.OrigRect.Bottom - b.OrigRect.Top
		if w > 100 && h > 100 {
			procSetWindowPos.Call(
				b.Hwnd, HWND_TOP,
				uintptr(b.OrigRect.Left), uintptr(b.OrigRect.Top),
				uintptr(w), uintptr(h),
				SWP_FRAMECHANGED|SWP_NOZORDER|SWP_NOOWNERZORDER|SWP_NOACTIVATE,
			)
		}
	}
	b.Active = false
	b.Hwnd = 0
}

func (b *BorderlessState) Toggle(hwnd uintptr) string {
	if hwnd == 0 {
		return "No game window"
	}
	if b.Active {
		b.Restore()
		return "Borderless OFF (restored)"
	}

	style, _, _ := procGetWindowLongW.Call(hwnd, GWL_STYLE)
	var rc RECT
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))

	hMon, _, _ := procMonitorFromWindow.Call(hwnd, MONITOR_DEFAULTTONEAREST)
	if hMon == 0 {
		return "MonitorFromWindow failed"
	}
	var mi MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	ret, _, _ := procGetMonitorInfoW.Call(hMon, uintptr(unsafe.Pointer(&mi)))
	if ret == 0 {
		return "GetMonitorInfoW failed"
	}

	b.Active = true
	b.Hwnd = hwnd
	b.OrigStyle = style
	b.OrigRect = rc

	newStyle := style &^ uintptr(WS_CAPTION|WS_THICKFRAME|WS_MINIMIZEBOX|WS_MAXIMIZEBOX|WS_SYSMENU)
	procSetWindowLongW.Call(hwnd, GWL_STYLE, newStyle)
	monW := mi.RcMonitor.Right - mi.RcMonitor.Left
	monH := mi.RcMonitor.Bottom - mi.RcMonitor.Top
	procSetWindowPos.Call(
		hwnd, HWND_TOP,
		uintptr(mi.RcMonitor.Left), uintptr(mi.RcMonitor.Top),
		uintptr(monW), uintptr(monH),
		SWP_FRAMECHANGED|SWP_NOZORDER|SWP_NOOWNERZORDER|SWP_NOACTIVATE,
	)
	return fmt.Sprintf("Borderless ON (%dx%d)", monW, monH)
}

type OverlayWindow struct {
	Hwnd           uintptr
	Visible        bool
	IsClickThrough bool
	CurX, CurY     int32
	CurW, CurH     int32
}

func CreateOverlayWindow() (*OverlayWindow, error) {
	className, _ := syscall.UTF16PtrFromString("BF2142DemoAnalyzerOverlay")
	windowName, _ := syscall.UTF16PtrFromString("BF2142 Demo Analyzer")

	var wc WNDCLASSEXW
	wc.Size = uint32(unsafe.Sizeof(wc))
	wc.Style = 0x0008 // CS_DBLCLKS
	wc.WndProc = syscall.NewCallback(overlayWndProc)
	wc.ClassName = className

	if atom, _, regErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return nil, fmt.Errorf("RegisterClassExW failed: %v", regErr)
	}

	screenW, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	screenH, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
	if screenW < 300 {
		screenW = 1280
	}
	if screenH < 200 {
		screenH = 720
	}

	hwnd, _, errCreate := procCreateWindowExW.Call(
		uintptr(WS_EX_TOPMOST|WS_EX_LAYERED|WS_EX_TRANSPARENT|WS_EX_TOOLWINDOW),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(WS_POPUP),
		0, 0, screenW, screenH,
		0, 0, 0, 0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("failed to create overlay window: %v", errCreate)
	}

	procSetLayeredWindowAttrs.Call(hwnd, uintptr(RGB(0, 0, 0)), 0, uintptr(LWA_COLORKEY))
	procUpdateWindow.Call(hwnd)

	return &OverlayWindow{
		Hwnd:           hwnd,
		IsClickThrough: true,
		CurW:           int32(screenW),
		CurH:           int32(screenH),
	}, nil
}

func (o *OverlayWindow) Destroy() {
	if o != nil && o.Hwnd != 0 {
		procDestroyWindow.Call(o.Hwnd)
		o.Hwnd = 0
	}
}

func (o *OverlayWindow) PumpMessages() bool {
	var msg MSG
	for {
		ret, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, PM_REMOVE)
		if ret == 0 {
			return true
		}
		if msg.Message == WM_QUIT {
			return false
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func (o *OverlayWindow) SyncPos(x, y, w, h int32) {
	if !o.Visible || x != o.CurX || y != o.CurY || w != o.CurW || h != o.CurH {
		o.CurX = x
		o.CurY = y
		o.CurW = w
		o.CurH = h
		procSetWindowPos.Call(
			o.Hwnd,
			HWND_TOPMOST,
			uintptr(x), uintptr(y),
			uintptr(w), uintptr(h),
			SWP_NOACTIVATE|SWP_SHOWWINDOW,
		)
		o.Visible = true
	}
}

func (o *OverlayWindow) Hide() {
	if !o.IsClickThrough {
		SetOverlayClickThrough(o.Hwnd, true)
		o.IsClickThrough = true
	}
	if o.Visible {
		procShowWindow.Call(o.Hwnd, SW_HIDE)
		o.Visible = false
	}
}

func (o *OverlayWindow) SetClickThrough(enable bool) {
	if o.IsClickThrough != enable {
		SetOverlayClickThrough(o.Hwnd, enable)
		o.IsClickThrough = enable
	}
}

func (o *OverlayWindow) RefocusGame(gameHwnd uintptr) {
	if gameHwnd != 0 {
		procSetForegroundWindow.Call(gameHwnd)
	}
}
