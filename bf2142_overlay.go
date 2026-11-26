package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
)

var (
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")
	modUser32   = syscall.NewLazyDLL("user32.dll")
	modGdi32    = syscall.NewLazyDLL("gdi32.dll")

	// kernel32
	procOpenProcess          = modKernel32.NewProc("OpenProcess")
	procCloseHandle          = modKernel32.NewProc("CloseHandle")
	procReadProcessMemory    = modKernel32.NewProc("ReadProcessMemory")
	procGetExitCodeProcess   = modKernel32.NewProc("GetExitCodeProcess")
	procCreateToolhelp32Snap = modKernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First       = modKernel32.NewProc("Process32FirstW")
	procProcess32Next        = modKernel32.NewProc("Process32NextW")
	procModule32First        = modKernel32.NewProc("Module32FirstW")
	procModule32Next         = modKernel32.NewProc("Module32NextW")

	// user32
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

	// gdi32
	procCreateCompatibleDC     = modGdi32.NewProc("CreateCompatibleDC")
	procDeleteDC               = modGdi32.NewProc("DeleteDC")
	procCreateCompatibleBitmap = modGdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = modGdi32.NewProc("SelectObject")
	procDeleteObject           = modGdi32.NewProc("DeleteObject")
	procBitBlt                 = modGdi32.NewProc("BitBlt")
	procCreateSolidBrush       = modGdi32.NewProc("CreateSolidBrush")
	procCreatePen              = modGdi32.NewProc("CreatePen")
	procSetBkMode              = modGdi32.NewProc("SetBkMode")
	procSetTextColor           = modGdi32.NewProc("SetTextColor")
	procTextOutW               = modGdi32.NewProc("TextOutW")
	procMoveToEx               = modGdi32.NewProc("MoveToEx")
	procLineTo                 = modGdi32.NewProc("LineTo")
	procRectangle              = modGdi32.NewProc("Rectangle")
	procEllipse                = modGdi32.NewProc("Ellipse")
	procCreateFontW            = modGdi32.NewProc("CreateFontW")
	procGetStockObject         = modGdi32.NewProc("GetStockObject")
	procGetTextExtentPoint32W  = modGdi32.NewProc("GetTextExtentPoint32W")
)

const (
	PROCESS_VM_READ           = 0x0010
	PROCESS_QUERY_INFORMATION = 0x0400
	STILL_ACTIVE              = 259

	TH32CS_SNAPPROCESS  = 0x00000002
	TH32CS_SNAPMODULE   = 0x00000008
	TH32CS_SNAPMODULE32 = 0x00000010

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

	SWP_NOSIZE       = 0x0001
	SWP_NOMOVE       = 0x0002
	SWP_NOZORDER     = 0x0004
	SWP_NOACTIVATE   = 0x0010
	SWP_FRAMECHANGED = 0x0020
	SWP_SHOWWINDOW   = 0x0040
	SWP_NOOWNERZORDER = 0x0200

	WM_DESTROY     = 0x0002
	WM_PAINT       = 0x000F
	WM_QUIT        = 0x0012
	WM_ERASEBKGND  = 0x0014
	WM_LBUTTONDOWN = 0x0201
	WM_LBUTTONDBLCLK = 0x0203

	PM_REMOVE = 0x0001

	PS_SOLID    = 0
	TRANSPARENT = 1
	SRCCOPY     = 0x00CC0020
	NULL_BRUSH  = 5

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

	minUserAddr = uint32(0x10000)
	maxUserAddr = uint32(0x7FFE0000)
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

type PROCESSENTRY32W struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

type MODULEENTRY32W struct {
	Size         uint32
	ModuleID     uint32
	ProcessID    uint32
	GlblcntUsage uint32
	ProccntUsage uint32
	ModBaseAddr  uintptr
	ModBaseSize  uint32
	HModule      uintptr
	SzModule     [256]uint16
	SzExePath    [260]uint16
}

type Vector3 struct {
	X, Y, Z float32
}

type Point2D struct {
	X, Y int32
}

type Point2DF struct {
	X, Y float32
}

type MemoryReader struct {
	hProcess syscall.Handle
}

func isValidUserAddr(addr uint32) bool {
	return addr >= minUserAddr && addr < maxUserAddr
}

func (m *MemoryReader) IsProcessAlive() bool {
	if m.hProcess == 0 {
		return false
	}
	var code uint32
	ret, _, _ := procGetExitCodeProcess.Call(uintptr(m.hProcess), uintptr(unsafe.Pointer(&code)))
	return ret != 0 && code == STILL_ACTIVE
}

func (m *MemoryReader) ReadRaw(addr uint32, buf []byte) error {
	if len(buf) == 0 {
		return nil
	}
	if !isValidUserAddr(addr) || uint64(addr)+uint64(len(buf)) > uint64(maxUserAddr) {
		return fmt.Errorf("address out of user-mode range: 0x%08X", addr)
	}
	var bytesRead uintptr
	ret, _, err := procReadProcessMemory.Call(
		uintptr(m.hProcess),
		uintptr(addr),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&bytesRead)),
	)
	if ret == 0 {
		return err
	}
	if bytesRead != uintptr(len(buf)) {
		return fmt.Errorf("partial read at 0x%08X: %d/%d", addr, bytesRead, len(buf))
	}
	return nil
}

func (m *MemoryReader) ReadUint8(addr uint32) (byte, error) {
	var buf [1]byte
	if err := m.ReadRaw(addr, buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}

func (m *MemoryReader) ReadUint32(addr uint32) (uint32, error) {
	var buf [4]byte
	if err := m.ReadRaw(addr, buf[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf[:]), nil
}

func (m *MemoryReader) ReadInt32(addr uint32) (int32, error) {
	val, err := m.ReadUint32(addr)
	if err != nil {
		return 0, err
	}
	return int32(val), nil
}

func (m *MemoryReader) ReadFloat32(addr uint32) (float32, error) {
	val, err := m.ReadUint32(addr)
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(val), nil
}

func (m *MemoryReader) ReadVector3(addr uint32) (Vector3, error) {
	var buf [12]byte
	var v Vector3
	if err := m.ReadRaw(addr, buf[:]); err != nil {
		return v, err
	}
	v.X = math.Float32frombits(binary.LittleEndian.Uint32(buf[0:4]))
	v.Y = math.Float32frombits(binary.LittleEndian.Uint32(buf[4:8]))
	v.Z = math.Float32frombits(binary.LittleEndian.Uint32(buf[8:12]))
	return v, nil
}

func (m *MemoryReader) ReadMatrix4x4(addr uint32) ([4][4]float32, error) {
	var buf [64]byte
	var mat [4][4]float32
	if err := m.ReadRaw(addr, buf[:]); err != nil {
		return mat, err
	}
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			offset := (row*4 + col) * 4
			mat[row][col] = math.Float32frombits(binary.LittleEndian.Uint32(buf[offset : offset+4]))
		}
	}
	return mat, nil
}

func isPrintableTextBytes(b []byte) bool {
	if len(b) == 0 || !utf8.Valid(b) {
		return false
	}
	for _, r := range string(b) {
		if r < 0x20 || r == 0x7F || (r >= 0x80 && r <= 0x9F) {
			return false
		}
	}
	return true
}

func isFiniteFloat(v float32) bool {
	f := float64(v)
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

func isValidVec3(v Vector3) bool {
	valX, valY, valZ := float64(v.X), float64(v.Y), float64(v.Z)
	if math.IsNaN(valX) || math.IsInf(valX, 0) ||
		math.IsNaN(valY) || math.IsInf(valY, 0) ||
		math.IsNaN(valZ) || math.IsInf(valZ, 0) {
		return false
	}
	if math.Abs(valX) > 50000 || math.Abs(valY) > 50000 || math.Abs(valZ) > 50000 {
		return false
	}
	return true
}

func (m *MemoryReader) ReadStdString(addr uint32) string {
	if !isValidUserAddr(addr) {
		return ""
	}
	var raw [32]byte
	if err := m.ReadRaw(addr, raw[:]); err != nil {
		return ""
	}

	tryLayout := func(bufOff, sizeOff, capOff int) (string, bool) {
		sz := binary.LittleEndian.Uint32(raw[sizeOff : sizeOff+4])
		cp := binary.LittleEndian.Uint32(raw[capOff : capOff+4])
		if sz == 0 || sz > cp || cp >= 100000 || sz >= 1000 {
			return "", false
		}
		if cp < 16 {
			inlineLen := sz
			if inlineLen > 16 {
				inlineLen = 16
			}
			inlineBuf := raw[bufOff : bufOff+int(inlineLen)]
			if n := bytes.IndexByte(inlineBuf, 0); n >= 0 {
				inlineBuf = inlineBuf[:n]
			}
			if isPrintableTextBytes(inlineBuf) {
				return string(inlineBuf), true
			}
			return "", false
		}
		ptr := binary.LittleEndian.Uint32(raw[bufOff : bufOff+4])
		if !isValidUserAddr(ptr) {
			return "", false
		}
		readLen := sz
		if readLen > 64 {
			readLen = 64
		}
		heapBuf := make([]byte, readLen)
		if err := m.ReadRaw(ptr, heapBuf); err == nil {
			if n := bytes.IndexByte(heapBuf, 0); n >= 0 {
				heapBuf = heapBuf[:n]
			}
			if isPrintableTextBytes(heapBuf) {
				return string(heapBuf), true
			}
		}
		return "", false
	}

	// MSVC 7.1 (28 bytes) then MSVC 2005/2008 (24 bytes)
	if s, ok := tryLayout(4, 20, 24); ok {
		return s
	}
	if s, ok := tryLayout(0, 16, 20); ok {
		return s
	}

	if n4 := bytes.IndexByte(raw[4:20], 0); n4 > 0 && isPrintableTextBytes(raw[4:4+n4]) {
		return string(raw[4 : 4+n4])
	}
	if n0 := bytes.IndexByte(raw[0:16], 0); n0 > 0 && isPrintableTextBytes(raw[0:n0]) {
		return string(raw[0:n0])
	}
	return ""
}

func (m *MemoryReader) ReadCPlayerName(playerAddr uint32) string {
	for _, off := range []uint32{0x60, 0x64} {
		s := m.ReadStdString(playerAddr + off)
		if isValidName(s) {
			return s
		}
	}

	var directBuf [32]byte
	if err := m.ReadRaw(playerAddr+0x64, directBuf[:]); err == nil {
		if n := bytes.IndexByte(directBuf[:], 0); n > 0 {
			s := string(directBuf[:n])
			if isValidName(s) {
				return s
			}
		}
	}
	return "<unknown>"
}

func isValidName(s string) bool {
	if len(s) == 0 || len(s) > 48 {
		return false
	}
	return isPrintableTextBytes([]byte(s))
}

func findProcess(name string) (uint32, error) {
	snap, _, err := procCreateToolhelp32Snap.Call(uintptr(TH32CS_SNAPPROCESS), 0)
	if snap == uintptr(syscall.InvalidHandle) {
		return 0, err
	}
	defer procCloseHandle.Call(snap)

	var entry PROCESSENTRY32W
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32First.Call(snap, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		exeName := syscall.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(exeName, name) {
			return entry.ProcessID, nil
		}
		ret, _, _ = procProcess32Next.Call(snap, uintptr(unsafe.Pointer(&entry)))
	}
	return 0, fmt.Errorf("process %q not found", name)
}

func getModuleBase(pid uint32, modName string) (uint32, error) {
	snap, _, err := procCreateToolhelp32Snap.Call(uintptr(TH32CS_SNAPMODULE|TH32CS_SNAPMODULE32), uintptr(pid))
	if snap == uintptr(syscall.InvalidHandle) {
		return 0, err
	}
	defer procCloseHandle.Call(snap)

	var entry MODULEENTRY32W
	entry.Size = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procModule32First.Call(snap, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		mName := syscall.UTF16ToString(entry.SzModule[:])
		if strings.EqualFold(mName, modName) {
			return uint32(entry.ModBaseAddr), nil
		}
		ret, _, _ = procModule32Next.Call(snap, uintptr(unsafe.Pointer(&entry)))
	}
	return 0, fmt.Errorf("module %q not found in PID %d", modName, pid)
}

func traverseClassManager(m *MemoryReader, classMgrAddr uint32) map[string]uint32 {
	classes := make(map[string]uint32)
	if !isValidUserAddr(classMgrAddr) {
		return classes
	}
	headNode, err := m.ReadUint32(classMgrAddr + 0x34)
	if err != nil || !isValidUserAddr(headNode) {
		return classes
	}
	size, err := m.ReadUint32(classMgrAddr + 0x38)
	if err != nil || size == 0 || size > 256 {
		return classes
	}
	root, err := m.ReadUint32(headNode + 0x04)
	if err != nil || !isValidUserAddr(root) || root == headNode {
		return classes
	}

	visited := make(map[uint32]bool)
	var walk func(node uint32, depth int)
	walk = func(node uint32, depth int) {
		if depth > 64 || len(visited) >= 512 || !isValidUserAddr(node) || node == headNode || visited[node] {
			return
		}
		visited[node] = true

		className := m.ReadStdString(node + 0x0C)
		if className == "" {
			className = m.ReadStdString(node + 0x10)
		}
		if valPtr, err1 := m.ReadUint32(node + 0x28); err1 == nil && isValidUserAddr(valPtr) {
			if instPtr, err2 := m.ReadUint32(valPtr); err2 == nil && isValidUserAddr(instPtr) && className != "" {
				classes[className] = instPtr
			}
		}
		if left, errL := m.ReadUint32(node + 0x00); errL == nil {
			walk(left, depth+1)
		}
		if right, errR := m.ReadUint32(node + 0x08); errR == nil {
			walk(right, depth+1)
		}
	}
	walk(root, 0)
	return classes
}

// Camera state & projection mathematics
type CameraData struct {
	Pos          Vector3
	ViewMatrix   [4][4]float32
	ProjMatrix   [4][4]float32
	HasValidProj bool
	VpWidth      float32
	VpHeight     float32
	FovRad1C     float32
}

func isValidMatrix(m [4][4]float32) bool {
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if !isFiniteFloat(m[r][c]) {
				return false
			}
		}
	}
	return true
}

func isValidProjMatrix(m [4][4]float32) bool {
	if !isValidMatrix(m) {
		return false
	}
	if math.Abs(float64(m[0][0])) <= 0.05 || math.Abs(float64(m[1][1])) <= 0.05 {
		return false
	}
	if m[0][0] == 1.0 && m[1][1] == 1.0 && m[2][2] == 1.0 {
		return false
	}
	return true
}

func (cam *CameraData) WorldToView(p Vector3) Vector3 {
	m := cam.ViewMatrix
	return Vector3{
		X: p.X*m[0][0] + p.Y*m[1][0] + p.Z*m[2][0] + m[3][0],
		Y: p.X*m[0][1] + p.Y*m[1][1] + p.Z*m[2][1] + m[3][1],
		Z: p.X*m[0][2] + p.Y*m[1][2] + p.Z*m[2][2] + m[3][2],
	}
}

func (cam *CameraData) getFallbackFocal(screenW, screenH, fovScale float32) (float32, float32) {
	halfH := screenH * 0.5
	halfW := screenW * 0.5

	fovY := float32(1.0239) // BF2142 default 58.665 deg
	if isFiniteFloat(cam.FovRad1C) && cam.FovRad1C >= 0.3 && cam.FovRad1C <= 2.5 {
		fovY = cam.FovRad1C
	}
	tanHalf := float32(math.Tan(float64(fovY) * 0.5))
	if tanHalf < 0.01 {
		tanHalf = 0.561937
	}
	fy := (halfH / tanHalf) * fovScale

	vpAspect := screenW / screenH
	if cam.VpWidth > 100 && cam.VpHeight > 100 {
		vpAspect = cam.VpWidth / cam.VpHeight
	}
	if vpAspect < 0.5 || vpAspect > 4.0 {
		vpAspect = screenW / screenH
	}
	fx := fy * ((halfW / halfH) / vpAspect)
	return fx, fy
}

func (cam *CameraData) GetVerticalFovDeg() float32 {
	if cam.HasValidProj && cam.ProjMatrix[1][1] > 0.05 && cam.ProjMatrix[1][1] < 20.0 {
		return float32(2.0 * math.Atan(1.0/float64(cam.ProjMatrix[1][1])) * 180.0 / math.Pi)
	}
	if isFiniteFloat(cam.FovRad1C) && cam.FovRad1C >= 0.3 && cam.FovRad1C <= 2.5 {
		return cam.FovRad1C * 180.0 / math.Pi
	}
	return 58.7
}

func (cam *CameraData) ViewToScreenFloat(v Vector3, screenW, screenH, fovScale float32) (Point2DF, bool) {
	if !isFiniteFloat(v.X) || !isFiniteFloat(v.Y) || !isFiniteFloat(v.Z) || v.Z < 0.1 {
		return Point2DF{}, false
	}

	halfW := screenW * 0.5
	halfH := screenH * 0.5
	var sx, sy float32

	if cam.HasValidProj {
		p := cam.ProjMatrix
		cx := v.X*p[0][0] + v.Y*p[1][0] + v.Z*p[2][0] + p[3][0]
		cy := v.X*p[0][1] + v.Y*p[1][1] + v.Z*p[2][1] + p[3][1]
		cw := v.X*p[0][3] + v.Y*p[1][3] + v.Z*p[2][3] + p[3][3]
		if !isFiniteFloat(cx) || !isFiniteFloat(cy) || !isFiniteFloat(cw) || cw <= 0.05 {
			return Point2DF{}, false
		}
		sx = halfW + (cx/cw)*halfW*fovScale
		sy = halfH - (cy/cw)*halfH*fovScale
	} else {
		fx, fy := cam.getFallbackFocal(screenW, screenH, fovScale)
		sx = halfW + (v.X/v.Z)*fx
		sy = halfH - (v.Y/v.Z)*fy
	}

	if !isFiniteFloat(sx) || !isFiniteFloat(sy) || math.Abs(float64(sx)) > 1e6 || math.Abs(float64(sy)) > 1e6 {
		return Point2DF{}, false
	}
	return Point2DF{X: sx, Y: sy}, true
}

func (cam *CameraData) WorldToScreen(p Vector3, screenW, screenH, fovScale float32) (Point2D, bool) {
	if !isValidVec3(p) {
		return Point2D{}, false
	}
	v := cam.WorldToView(p)
	sf, ok := cam.ViewToScreenFloat(v, screenW, screenH, fovScale)
	if !ok {
		return Point2D{}, false
	}
	return Point2D{
		X: int32(math.Round(float64(sf.X))),
		Y: int32(math.Round(float64(sf.Y))),
	}, true
}

func computeOutCode(x, y, maxW, maxH float32) int {
	code := 0
	if x < 0 {
		code |= 1
	} else if x > maxW {
		code |= 2
	}
	if y < 0 {
		code |= 8
	} else if y > maxH {
		code |= 4
	}
	return code
}

// Float Cohen-Sutherland 2D clipping against [0..W-1, 0..H-1] before rounding once at the end.
func clipLine2DFloat(p1, p2 Point2DF, w, h int32) (Point2D, Point2D, bool) {
	if w < 2 || h < 2 {
		return Point2D{}, Point2D{}, false
	}
	x0, y0 := p1.X, p1.Y
	x1, y1 := p2.X, p2.Y
	maxW := float32(w - 1)
	maxH := float32(h - 1)

	code0 := computeOutCode(x0, y0, maxW, maxH)
	code1 := computeOutCode(x1, y1, maxW, maxH)

	for iter := 0; iter < 12; iter++ {
		if (code0 | code1) == 0 {
			return Point2D{int32(math.Round(float64(x0))), int32(math.Round(float64(y0)))},
				Point2D{int32(math.Round(float64(x1))), int32(math.Round(float64(y1)))}, true
		}
		if (code0 & code1) != 0 {
			return Point2D{}, Point2D{}, false
		}

		outCode := code0
		if outCode == 0 {
			outCode = code1
		}

		var x, y float32
		if (outCode & 8) != 0 {
			if math.Abs(float64(y1-y0)) < 1e-6 {
				return Point2D{}, Point2D{}, false
			}
			x = x0 + (x1-x0)*(0-y0)/(y1-y0)
			y = 0
		} else if (outCode & 4) != 0 {
			if math.Abs(float64(y1-y0)) < 1e-6 {
				return Point2D{}, Point2D{}, false
			}
			x = x0 + (x1-x0)*(maxH-y0)/(y1-y0)
			y = maxH
		} else if (outCode & 2) != 0 {
			if math.Abs(float64(x1-x0)) < 1e-6 {
				return Point2D{}, Point2D{}, false
			}
			y = y0 + (y1-y0)*(maxW-x0)/(x1-x0)
			x = maxW
		} else if (outCode & 1) != 0 {
			if math.Abs(float64(x1-x0)) < 1e-6 {
				return Point2D{}, Point2D{}, false
			}
			y = y0 + (y1-y0)*(0-x0)/(x1-x0)
			x = 0
		}

		// Snap tiny float32 drift onto exact bounds to guarantee termination
		if x < 0 && x > -1e-3 {
			x = 0
		} else if x > maxW && x < maxW+1e-3 {
			x = maxW
		}
		if y < 0 && y > -1e-3 {
			y = 0
		} else if y > maxH && y < maxH+1e-3 {
			y = maxH
		}

		if outCode == code0 {
			x0, y0 = x, y
			code0 = computeOutCode(x0, y0, maxW, maxH)
		} else {
			x1, y1 = x, y
			code1 = computeOutCode(x1, y1, maxW, maxH)
		}
	}
	return Point2D{}, Point2D{}, false
}

func (cam *CameraData) ProjectLine3D(p1, p2 Vector3, screenW, screenH, fovScale float32) (Point2D, Point2D, bool) {
	if !isValidVec3(p1) || !isValidVec3(p2) {
		return Point2D{}, Point2D{}, false
	}
	v1 := cam.WorldToView(p1)
	v2 := cam.WorldToView(p2)

	const zNear float32 = 0.25
	if v1.Z < zNear && v2.Z < zNear {
		return Point2D{}, Point2D{}, false
	}
	if v1.Z < zNear {
		dz := v2.Z - v1.Z
		if math.Abs(float64(dz)) < 1e-6 {
			return Point2D{}, Point2D{}, false
		}
		t := (zNear - v1.Z) / dz
		v1.X += t * (v2.X - v1.X)
		v1.Y += t * (v2.Y - v1.Y)
		v1.Z = zNear
	} else if v2.Z < zNear {
		dz := v1.Z - v2.Z
		if math.Abs(float64(dz)) < 1e-6 {
			return Point2D{}, Point2D{}, false
		}
		t := (zNear - v2.Z) / dz
		v2.X += t * (v1.X - v2.X)
		v2.Y += t * (v1.Y - v2.Y)
		v2.Z = zNear
	}

	s1, ok1 := cam.ViewToScreenFloat(v1, screenW, screenH, fovScale)
	s2, ok2 := cam.ViewToScreenFloat(v2, screenW, screenH, fovScale)
	if !ok1 || !ok2 {
		return Point2D{}, Point2D{}, false
	}
	return clipLine2DFloat(s1, s2, int32(screenW), int32(screenH))
}

type PlayerInfo struct {
	Addr      uint32
	ID        int32
	Name      string
	Team      uint32
	IsAlive   bool
	InVehicle bool
	Score     int32
	Kills     int32
	Deaths    int32
	EyePos    Vector3
	LookDir   Vector3
	HasPose   bool
	Distance  float32
}

func vec3Dist(a, b Vector3) float32 {
	dx := float64(a.X - b.X)
	dy := float64(a.Y - b.Y)
	dz := float64(a.Z - b.Z)
	return float32(math.Sqrt(dx*dx + dy*dy + dz*dz))
}

func vec3Normalize(v Vector3) Vector3 {
	length := float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
	if length < 1e-6 {
		return Vector3{}
	}
	inv := 1.0 / length
	return Vector3{v.X * inv, v.Y * inv, v.Z * inv}
}

func vec3Cross(a, b Vector3) Vector3 {
	return Vector3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

func closestAimPoint(target *PlayerInfo, enemy *PlayerInfo) (Vector3, float32) {
	headPos := enemy.EyePos
	toHead := Vector3{
		X: headPos.X - target.EyePos.X,
		Y: headPos.Y - target.EyePos.Y,
		Z: headPos.Z - target.EyePos.Z,
	}
	bestPt := headPos
	bestAng := angleBetween(target.LookDir, toHead)

	if !enemy.InVehicle {
		bodyPos := Vector3{
			X: enemy.EyePos.X,
			Y: enemy.EyePos.Y - 0.55,
			Z: enemy.EyePos.Z,
		}
		toBody := Vector3{
			X: bodyPos.X - target.EyePos.X,
			Y: bodyPos.Y - target.EyePos.Y,
			Z: bodyPos.Z - target.EyePos.Z,
		}
		if angBody := angleBetween(target.LookDir, toBody); angBody < bestAng {
			bestAng = angBody
			bestPt = bodyPos
		}
	}
	return bestPt, bestAng
}

func angleBetween(v1, v2 Vector3) float32 {
	len1 := float64(v1.X*v1.X + v1.Y*v1.Y + v1.Z*v1.Z)
	len2 := float64(v2.X*v2.X + v2.Y*v2.Y + v2.Z*v2.Z)
	if len1 < 1e-8 || len2 < 1e-8 {
		return 180.0
	}
	inv1 := 1.0 / math.Sqrt(len1)
	inv2 := 1.0 / math.Sqrt(len2)
	dot := (float64(v1.X)*float64(v2.X) + float64(v1.Y)*float64(v2.Y) + float64(v1.Z)*float64(v2.Z)) * inv1 * inv2
	if dot > 1.0 {
		dot = 1.0
	} else if dot < -1.0 {
		dot = -1.0
	}
	return float32(math.Acos(dot) * 180.0 / math.Pi)
}

type AimTick struct {
	Time      time.Time
	LookDir   Vector3
	EnemyID   int32
	EnemyName string
	EnemyDist float32
	Angle     float32
	EnemyPos  Vector3
}

type AimAnalyzer struct {
	Ticks           []AimTick
	PrevTargetLook  Vector3
	PrevTargetPos   Vector3
	PrevEnemyPos    Vector3
	HasPrevSample   bool

	// Snap Detection Stats
	SnapCount       int
	MaxSnapAngle    float32
	LastSnapText    string
	LastSnapTime    time.Time
	LastSnapEnemyID int32

	// Lock Detection State (with jitter grace)
	IsLocking           bool
	LockStartTime       time.Time
	LockEnemyID         int32
	LockEnemyName       string
	LockEnemyDist       float32
	LockStartPos        Vector3
	LockMaxMoved        float32
	LockAngles          []float32
	LockMissFrames      int
	CurrentLockDuration float32

	LogFile *os.File
}

func NewAimAnalyzer(logFile *os.File) *AimAnalyzer {
	return &AimAnalyzer{
		Ticks:   make([]AimTick, 0, 32),
		LogFile: logFile,
	}
}

func (a *AimAnalyzer) PauseOnDeath(targetName string) {
	a.finalizeLock(time.Now(), targetName)
	a.HasPrevSample = false
}

func (a *AimAnalyzer) Reset(targetName string) {
	a.finalizeLock(time.Now(), targetName)

	a.Ticks = a.Ticks[:0]
	a.HasPrevSample = false
	a.PrevTargetLook = Vector3{}
	a.PrevTargetPos = Vector3{}
	a.PrevEnemyPos = Vector3{}
	a.SnapCount = 0
	a.MaxSnapAngle = 0
	a.LastSnapText = ""
	a.LastSnapTime = time.Time{}
	a.LastSnapEnemyID = -1
	a.IsLocking = false
	a.LockAngles = nil
	a.LockMissFrames = 0
	a.CurrentLockDuration = 0

	resetMsg := fmt.Sprintf("[%s] --- AIM ANALYZER RESET FOR: %s ---\n", time.Now().Format("15:04:05"), targetName)
	fmt.Print(resetMsg)
	if a.LogFile != nil {
		a.LogFile.WriteString(resetMsg)
	}
}

func (a *AimAnalyzer) checkSnap(now time.Time, targetName string) {
	n := len(a.Ticks)
	if n < 2 {
		return
	}
	curTick := a.Ticks[n-1]
	if curTick.Angle > 1.5 {
		return
	}

	bestIdx := -1
	maxDelta := float32(0)

	minLookback := n - 10
	if minLookback < 0 {
		minLookback = 0
	}

	for i := n - 2; i >= minLookback; i-- {
		prev := a.Ticks[i]
		if prev.EnemyID != curTick.EnemyID {
			continue
		}
		if curTick.Time.Sub(prev.Time) > 600*time.Millisecond {
			continue
		}
		if prev.Angle < 2.5 {
			continue
		}
		delta := prev.Angle - curTick.Angle
		if delta >= 2.0 && delta > maxDelta {
			maxDelta = delta
			bestIdx = i
		}
	}

	if bestIdx >= 0 {
		prevTick := a.Ticks[bestIdx]
		ticksCount := (n - 1) - bestIdx

		if curTick.EnemyID == a.LastSnapEnemyID && now.Sub(a.LastSnapTime) < 500*time.Millisecond {
			return
		}

		a.SnapCount++
		if maxDelta > a.MaxSnapAngle {
			a.MaxSnapAngle = maxDelta
		}
		a.LastSnapText = fmt.Sprintf("%.2f° -> %.2f°", prevTick.Angle, curTick.Angle)
		a.LastSnapTime = now
		a.LastSnapEnemyID = curTick.EnemyID

		dist := int(math.Round(float64(curTick.EnemyDist)))
		logLine := fmt.Sprintf("[%s] [SNAP] %s -> %s: %.2f° -> %.2f° (delta +%.2f° in %d ticks, dist %dm)\n",
			now.Format("15:04:05"), targetName, curTick.EnemyName, prevTick.Angle, curTick.Angle, maxDelta, ticksCount, dist)

		fmt.Print(logLine)
		if a.LogFile != nil {
			a.LogFile.WriteString(logLine)
		}
	}
}

func (a *AimAnalyzer) updateLock(now time.Time, targetName string, enemy *PlayerInfo, angle float32, dist float32) {
	if enemy == nil || angle > 1.8 {
		if a.IsLocking {
			a.LockMissFrames++
			if a.LockMissFrames > 2 || enemy == nil {
				a.finalizeLock(now, targetName)
			}
		}
		return
	}

	a.LockMissFrames = 0
	if !a.IsLocking {
		a.IsLocking = true
		a.LockStartTime = now
		a.LockEnemyID = enemy.ID
		a.LockEnemyName = enemy.Name
		a.LockEnemyDist = dist
		a.LockStartPos = enemy.EyePos
		a.LockMaxMoved = 0
		a.LockAngles = []float32{angle}
		a.CurrentLockDuration = 0
		return
	}

	if enemy.ID != a.LockEnemyID {
		a.finalizeLock(now, targetName)
		a.IsLocking = true
		a.LockStartTime = now
		a.LockEnemyID = enemy.ID
		a.LockEnemyName = enemy.Name
		a.LockEnemyDist = dist
		a.LockStartPos = enemy.EyePos
		a.LockMaxMoved = 0
		a.LockAngles = []float32{angle}
		a.CurrentLockDuration = 0
		return
	}

	a.LockEnemyDist = dist
	if angle <= 1.5 {
		a.LockAngles = append(a.LockAngles, angle)
	}
	moved := vec3Dist(a.LockStartPos, enemy.EyePos)
	if moved > a.LockMaxMoved {
		a.LockMaxMoved = moved
	}
	a.CurrentLockDuration = float32(now.Sub(a.LockStartTime).Seconds())
}

func (a *AimAnalyzer) finalizeLock(now time.Time, targetName string) {
	if !a.IsLocking {
		return
	}
	duration := float32(now.Sub(a.LockStartTime).Seconds())
	if duration >= 0.35 && len(a.LockAngles) >= 4 {
		var sum float32
		for _, ang := range a.LockAngles {
			sum += ang
		}
		avgAngle := sum / float32(len(a.LockAngles))
		if a.LockMaxMoved >= 0.3 || (duration >= 0.5 && avgAngle <= 1.2) {
			logLine := fmt.Sprintf("[%s] [LOCK] %s -> %s: avg %.2f° for %.2fs (enemy moved %.1fm, dist %.0fm)\n",
				now.Format("15:04:05"), targetName, a.LockEnemyName, avgAngle, duration, a.LockMaxMoved, a.LockEnemyDist)
			fmt.Print(logLine)
			if a.LogFile != nil {
				a.LogFile.WriteString(logLine)
			}
		}
	}
	a.IsLocking = false
	a.LockAngles = nil
	a.LockMissFrames = 0
	a.CurrentLockDuration = 0
}

func RGB(r, g, b byte) uint32 {
	return uint32(r) | (uint32(g) << 8) | (uint32(b) << 16)
}

func clampF32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func scaleI32(v int32, scale float32) int32 {
	res := int32(math.Round(float64(float32(v) * scale)))
	if res == 0 && v != 0 {
		if v > 0 {
			return 1
		}
		return -1
	}
	return res
}

func getTextWidth(hdc uintptr, text string) int32 {
	var sz SIZE
	u16, err := syscall.UTF16FromString(text)
	if err != nil || len(u16) <= 1 {
		return 0
	}
	procGetTextExtentPoint32W.Call(hdc, uintptr(unsafe.Pointer(&u16[0])), uintptr(len(u16)-1), uintptr(unsafe.Pointer(&sz)))
	return sz.CX
}

func drawTextShadow(hdc uintptr, text string, x, y int32, color uint32) {
	u16, err := syscall.UTF16FromString(text)
	if err != nil || len(u16) <= 1 {
		return
	}
	ptr := uintptr(unsafe.Pointer(&u16[0]))
	length := uintptr(len(u16) - 1)

	procSetTextColor.Call(hdc, uintptr(RGB(1, 1, 1)))
	procTextOutW.Call(hdc, uintptr(x+1), uintptr(y+1), ptr, length)

	procSetTextColor.Call(hdc, uintptr(color))
	procTextOutW.Call(hdc, uintptr(x), uintptr(y), ptr, length)
}

func drawCornerBox(hdc uintptr, left, top, right, bottom int32, penMain, penShadow uintptr) {
	w := right - left
	corner := w / 4
	if corner < 3 {
		corner = 3
	}
	if corner > 16 {
		corner = 16
	}

	drawCornersAt := func(offX, offY int32) {
		l := left + offX
		t := top + offY
		r := right + offX
		b := bottom + offY

		procMoveToEx.Call(hdc, uintptr(l), uintptr(t+corner), 0)
		procLineTo.Call(hdc, uintptr(l), uintptr(t))
		procLineTo.Call(hdc, uintptr(l+corner), uintptr(t))

		procMoveToEx.Call(hdc, uintptr(r-corner), uintptr(t), 0)
		procLineTo.Call(hdc, uintptr(r), uintptr(t))
		procLineTo.Call(hdc, uintptr(r), uintptr(t+corner))

		procMoveToEx.Call(hdc, uintptr(l), uintptr(b-corner), 0)
		procLineTo.Call(hdc, uintptr(l), uintptr(b))
		procLineTo.Call(hdc, uintptr(l+corner), uintptr(b))

		procMoveToEx.Call(hdc, uintptr(r-corner), uintptr(b), 0)
		procLineTo.Call(hdc, uintptr(r), uintptr(b))
		procLineTo.Call(hdc, uintptr(r), uintptr(b-corner))
	}

	procSelectObject.Call(hdc, penShadow)
	drawCornersAt(1, 1)

	procSelectObject.Call(hdc, penMain)
	drawCornersAt(0, 0)
}

func drawDiamondMarker(hdc uintptr, cx, cy, r int32, penMain, penShadow, brushFill uintptr) {
	if r < 3 {
		r = 3
	}
	drawAt := func(ox, oy int32) {
		procMoveToEx.Call(hdc, uintptr(cx+ox), uintptr(cy-r+oy), 0)
		procLineTo.Call(hdc, uintptr(cx+r+ox), uintptr(cy+oy))
		procLineTo.Call(hdc, uintptr(cx+ox), uintptr(cy+r+oy))
		procLineTo.Call(hdc, uintptr(cx-r+ox), uintptr(cy+oy))
		procLineTo.Call(hdc, uintptr(cx+ox), uintptr(cy-r+oy))
	}
	procSelectObject.Call(hdc, penShadow)
	drawAt(1, 1)
	procSelectObject.Call(hdc, penMain)
	drawAt(0, 0)
	if brushFill != 0 {
		procSelectObject.Call(hdc, brushFill)
		procEllipse.Call(hdc, uintptr(cx-2), uintptr(cy-2), uintptr(cx+3), uintptr(cy+3))
	}
}

// Thread-safe click inbox from WndProc -> main loop
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

func isWindowValid(hwnd uintptr, pid uint32) bool {
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

func findGameWindow(pid uint32) uintptr {
	searchPID = pid
	bestHwnd = 0
	maxArea = 0
	procEnumWindows.Call(enumWindowsCallback, 0)
	return bestHwnd
}

func enableDPIAwareness() {
	if procSetProcessDpiAwarenessContext.Find() == nil {
		procSetProcessDpiAwarenessContext.Call(DPI_AWARENESS_CONTEXT_PM_V2)
	} else if procSetProcessDPIAware.Find() == nil {
		procSetProcessDPIAware.Call()
	}
}

func setOverlayClickThrough(hwnd uintptr, clickThrough bool) {
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

type GDICache struct {
	BlackBrush     uintptr
	HudBrush       uintptr
	BoardBgBrush   uintptr
	RowHoverBrush  uintptr
	RowTargetBrush uintptr
	RowCursorBrush uintptr
	HudBorderPen   uintptr
	PenGreenBorder uintptr
	PenShadow      uintptr
	PenTargetBox   uintptr
	PenTargetLaser uintptr
	PenAimEnemy    uintptr
	PenAimCone     uintptr
	PenTeam1       uintptr
	PenTeam2       uintptr
	PenDead        uintptr
	BrushTargetDot uintptr
	BrushAimEnemy  uintptr
	BrushTeam1Dot  uintptr
	BrushTeam2Dot  uintptr
	NullBrush      uintptr

	HFont         uintptr
	HFontBold     uintptr
	HFontSmall    uintptr
	curFontPx     int32
	curBoldPx     int32
	curSmallPx    int32
}

func initGDICache() *GDICache {
	c := &GDICache{}
	c.BlackBrush, _, _ = procCreateSolidBrush.Call(uintptr(RGB(0, 0, 0)))
	c.HudBrush, _, _ = procCreateSolidBrush.Call(uintptr(RGB(18, 22, 28)))
	c.BoardBgBrush, _, _ = procCreateSolidBrush.Call(uintptr(RGB(14, 18, 24)))
	c.RowHoverBrush, _, _ = procCreateSolidBrush.Call(uintptr(RGB(34, 46, 62)))
	c.RowTargetBrush, _, _ = procCreateSolidBrush.Call(uintptr(RGB(68, 56, 16)))
	c.RowCursorBrush, _, _ = procCreateSolidBrush.Call(uintptr(RGB(28, 60, 74)))

	c.HudBorderPen, _, _ = procCreatePen.Call(PS_SOLID, 1, uintptr(RGB(65, 82, 105)))
	c.PenGreenBorder, _, _ = procCreatePen.Call(PS_SOLID, 1, uintptr(RGB(0, 255, 60)))
	c.PenShadow, _, _ = procCreatePen.Call(PS_SOLID, 1, uintptr(RGB(1, 1, 1)))

	// 1px crisp pens for lasers and boxes
	c.PenTargetBox, _, _ = procCreatePen.Call(PS_SOLID, 1, uintptr(RGB(255, 230, 0)))
	c.PenTargetLaser, _, _ = procCreatePen.Call(PS_SOLID, 1, uintptr(RGB(255, 235, 0)))
	c.PenAimEnemy, _, _ = procCreatePen.Call(PS_SOLID, 1, uintptr(RGB(220, 80, 255)))
	c.PenAimCone, _, _ = procCreatePen.Call(PS_SOLID, 1, uintptr(RGB(255, 195, 60)))
	c.PenTeam1, _, _ = procCreatePen.Call(PS_SOLID, 1, uintptr(RGB(50, 180, 255)))
	c.PenTeam2, _, _ = procCreatePen.Call(PS_SOLID, 1, uintptr(RGB(255, 100, 60)))
	c.PenDead, _, _ = procCreatePen.Call(PS_SOLID, 1, uintptr(RGB(115, 120, 130)))

	c.BrushTargetDot, _, _ = procCreateSolidBrush.Call(uintptr(RGB(255, 235, 0)))
	c.BrushAimEnemy, _, _ = procCreateSolidBrush.Call(uintptr(RGB(220, 80, 255)))
	c.BrushTeam1Dot, _, _ = procCreateSolidBrush.Call(uintptr(RGB(50, 180, 255)))
	c.BrushTeam2Dot, _, _ = procCreateSolidBrush.Call(uintptr(RGB(255, 100, 60)))

	c.NullBrush, _, _ = procGetStockObject.Call(NULL_BRUSH)
	c.UpdateScale(1.0)
	return c
}

func (c *GDICache) UpdateScale(scale float32) {
	mainPx := scaleI32(13, scale)
	boldPx := scaleI32(15, scale)
	smallPx := scaleI32(11, scale)
	if mainPx == c.curFontPx && boldPx == c.curBoldPx && smallPx == c.curSmallPx && c.HFont != 0 {
		return
	}

	if c.HFont != 0 {
		procDeleteObject.Call(c.HFont)
	}
	if c.HFontBold != 0 {
		procDeleteObject.Call(c.HFontBold)
	}
	if c.HFontSmall != 0 {
		procDeleteObject.Call(c.HFontSmall)
	}

	fontName, _ := syscall.UTF16PtrFromString("Segoe UI")
	c.HFont, _, _ = procCreateFontW.Call(uintptr(mainPx), 0, 0, 0, 600, 0, 0, 0, 0, 0, 0, 3, 0, uintptr(unsafe.Pointer(fontName)))
	c.HFontBold, _, _ = procCreateFontW.Call(uintptr(boldPx), 0, 0, 0, 700, 0, 0, 0, 0, 0, 0, 3, 0, uintptr(unsafe.Pointer(fontName)))
	c.HFontSmall, _, _ = procCreateFontW.Call(uintptr(smallPx), 0, 0, 0, 500, 0, 0, 0, 0, 0, 0, 3, 0, uintptr(unsafe.Pointer(fontName)))

	c.curFontPx = mainPx
	c.curBoldPx = boldPx
	c.curSmallPx = smallPx
}

func (c *GDICache) Cleanup() {
	for _, h := range []uintptr{
		c.BlackBrush, c.HudBrush, c.BoardBgBrush, c.RowHoverBrush, c.RowTargetBrush, c.RowCursorBrush,
		c.HudBorderPen, c.PenGreenBorder, c.PenShadow, c.PenTargetBox, c.PenTargetLaser,
		c.PenAimEnemy, c.PenAimCone, c.PenTeam1, c.PenTeam2, c.PenDead,
		c.BrushTargetDot, c.BrushAimEnemy, c.BrushTeam1Dot, c.BrushTeam2Dot,
		c.HFont, c.HFontBold, c.HFontSmall,
	} {
		if h != 0 {
			procDeleteObject.Call(h)
		}
	}
}

type GameAttachment struct {
	PID              uint32
	Reader           *MemoryReader
	BF2142Base       uint32
	RendBase         uint32
	ClassMgrPtrAddr  uint32
	PlayerMgrPtrAddr uint32
	RendererPtrAddr  uint32
	CachedHwnd       uintptr
	NameCache        map[uint64]string
}

func (g *GameAttachment) Close() {
	if g.Reader != nil && g.Reader.hProcess != 0 {
		procCloseHandle.Call(uintptr(g.Reader.hProcess))
		g.Reader.hProcess = 0
	}
	g.PID = 0
	g.CachedHwnd = 0
}

func tryAttachGame() (*GameAttachment, error) {
	pid, err := findProcess("BF2142.exe")
	if err != nil {
		return nil, err
	}
	hProcess, _, openErr := procOpenProcess.Call(
		uintptr(PROCESS_VM_READ|PROCESS_QUERY_INFORMATION),
		0,
		uintptr(pid),
	)
	if hProcess == 0 {
		return nil, fmt.Errorf("OpenProcess(PID %d) failed: %v", pid, openErr)
	}
	bf2142Base, err1 := getModuleBase(pid, "BF2142.exe")
	rendBase, err2 := getModuleBase(pid, "RendDX9.dll")
	if err1 != nil || err2 != nil {
		procCloseHandle.Call(hProcess)
		if err1 != nil {
			return nil, err1
		}
		return nil, err2
	}
	return &GameAttachment{
		PID:              pid,
		Reader:           &MemoryReader{hProcess: syscall.Handle(hProcess)},
		BF2142Base:       bf2142Base,
		RendBase:         rendBase,
		ClassMgrPtrAddr:  bf2142Base + 0x67D068,
		PlayerMgrPtrAddr: bf2142Base + 0x685228,
		RendererPtrAddr:  rendBase + 0x1F8E58,
		NameCache:        make(map[uint64]string),
	}, nil
}

type ScoreboardRowHit struct {
	Rect     RECT
	PlayerID int32
	Name     string
}

func renderAimCone3D(hdc uintptr, cam *CameraData, origin, lookDir Vector3, dist, halfAngleDeg, screenW, screenH, fovScale float32, penMain, penShadow uintptr) {
	if halfAngleDeg <= 0.1 || halfAngleDeg > 45.0 || dist < 1.0 {
		return
	}
	fwd := vec3Normalize(lookDir)
	if fwd.X == 0 && fwd.Y == 0 && fwd.Z == 0 {
		return
	}
	upRef := Vector3{0, 1, 0}
	if math.Abs(float64(fwd.Y)) > 0.92 {
		upRef = Vector3{1, 0, 0}
	}
	right := vec3Normalize(vec3Cross(upRef, fwd))
	up := vec3Normalize(vec3Cross(fwd, right))

	radius := dist * float32(math.Tan(float64(halfAngleDeg)*math.Pi/180.0))
	center := Vector3{
		X: origin.X + fwd.X*dist,
		Y: origin.Y + fwd.Y*dist,
		Z: origin.Z + fwd.Z*dist,
	}

	const segments = 24
	pts := make([]Vector3, segments)
	for i := 0; i < segments; i++ {
		ang := float64(i) * (2.0 * math.Pi / float64(segments))
		ca := float32(math.Cos(ang)) * radius
		sa := float32(math.Sin(ang)) * radius
		pts[i] = Vector3{
			X: center.X + right.X*ca + up.X*sa,
			Y: center.Y + right.Y*ca + up.Y*sa,
			Z: center.Z + right.Z*ca + up.Z*sa,
		}
	}

	for i := 0; i < segments; i++ {
		next := (i + 1) % segments
		s1, s2, ok := cam.ProjectLine3D(pts[i], pts[next], screenW, screenH, fovScale)
		if ok {
			procSelectObject.Call(hdc, penShadow)
			procMoveToEx.Call(hdc, uintptr(s1.X+1), uintptr(s1.Y+1), 0)
			procLineTo.Call(hdc, uintptr(s2.X+1), uintptr(s2.Y+1))

			procSelectObject.Call(hdc, penMain)
			procMoveToEx.Call(hdc, uintptr(s1.X), uintptr(s1.Y), 0)
			procLineTo.Call(hdc, uintptr(s2.X), uintptr(s2.Y))
		}
	}
}

func renderScoreboard(
	hdc uintptr, gdi *GDICache, uiScale float32,
	curW, curH int32,
	allPlayers []PlayerInfo,
	selectedTargetID int32, hasSelectedTarget bool,
	cursorIdx int, cursorPos *POINT,
) ([]ScoreboardRowHit, []int32) {
	team1 := make([]PlayerInfo, 0, len(allPlayers))
	team2 := make([]PlayerInfo, 0, len(allPlayers))
	other := make([]PlayerInfo, 0)
	for _, p := range allPlayers {
		switch p.Team {
		case 1:
			team1 = append(team1, p)
		case 2:
			team2 = append(team2, p)
		default:
			other = append(other, p)
		}
	}
	sortPlayers := func(list []PlayerInfo) {
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Score != list[j].Score {
				return list[i].Score > list[j].Score
			}
			if list[i].Kills != list[j].Kills {
				return list[i].Kills > list[j].Kills
			}
			return list[i].ID < list[j].ID
		})
	}
	sortPlayers(team1)
	sortPlayers(team2)
	sortPlayers(other)
	if len(other) > 0 {
		team2 = append(team2, other...)
	}

	flatOrder := make([]int32, 0, len(team1)+len(team2))
	for _, p := range team1 {
		flatOrder = append(flatOrder, p.ID)
	}
	for _, p := range team2 {
		flatOrder = append(flatOrder, p.ID)
	}

	var cursorTargetID int32 = -1
	hasCursorID := false
	if len(flatOrder) > 0 && cursorIdx >= 0 && cursorIdx < len(flatOrder) {
		cursorTargetID = flatOrder[cursorIdx]
		hasCursorID = true
	}

	boardW := curW - scaleI32(120, uiScale)
	maxBoardW := scaleI32(940, uiScale)
	if boardW > maxBoardW {
		boardW = maxBoardW
	}
	minBoardW := scaleI32(520, uiScale)
	if boardW < minBoardW {
		boardW = curW - scaleI32(30, uiScale)
	}

	headerH := scaleI32(56, uiScale)
	colHdrH := scaleI32(22, uiScale)
	rowH := scaleI32(20, uiScale)
	footerH := scaleI32(26, uiScale)

	maxRows := len(team1)
	if len(team2) > maxRows {
		maxRows = len(team2)
	}
	if maxRows < 6 {
		maxRows = 6
	}
	maxVisibleRows := (curH - scaleI32(80, uiScale) - headerH - colHdrH - footerH) / rowH
	if maxVisibleRows < 4 {
		maxVisibleRows = 4
	}
	if maxRows > int(maxVisibleRows) {
		maxRows = int(maxVisibleRows)
	}

	boardH := headerH + colHdrH + int32(maxRows)*rowH + footerH
	bx := (curW - boardW) / 2
	by := (curH - boardH) / 2

	rcBoard := RECT{bx, by, bx + boardW, by + boardH}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rcBoard)), gdi.BoardBgBrush)
	procSelectObject.Call(hdc, gdi.HudBorderPen)
	procSelectObject.Call(hdc, gdi.NullBrush)
	procRectangle.Call(hdc, uintptr(rcBoard.Left), uintptr(rcBoard.Top), uintptr(rcBoard.Right), uintptr(rcBoard.Bottom))

	pad := scaleI32(12, uiScale)
	procSelectObject.Call(hdc, gdi.HFontBold)
	drawTextShadow(hdc, fmt.Sprintf("SCOREBOARD (%d players) - Click or [Up/Down + Enter] to select Target | [Tab/Esc] Close", len(allPlayers)),
		bx+pad, by+scaleI32(8, uiScale), RGB(255, 220, 70))

	colW := (boardW - pad*3) / 2
	col1X := bx + pad
	col2X := bx + pad*2 + colW
	colTop := by + scaleI32(32, uiScale)

	procSelectObject.Call(hdc, gdi.HFont)
	drawTextShadow(hdc, fmt.Sprintf("TEAM 1 (%d)", len(team1)), col1X+4, colTop, RGB(80, 195, 255))
	drawTextShadow(hdc, fmt.Sprintf("TEAM 2 (%d)", len(team2)), col2X+4, colTop, RGB(255, 115, 75))

	hdrY := by + headerH
	procSelectObject.Call(hdc, gdi.HFontSmall)
	drawColHeaders := func(cx int32) {
		drawTextShadow(hdc, "ID", cx+scaleI32(4, uiScale), hdrY, RGB(170, 185, 200))
		drawTextShadow(hdc, "ST", cx+scaleI32(34, uiScale), hdrY, RGB(170, 185, 200))
		drawTextShadow(hdc, "NICKNAME", cx+scaleI32(68, uiScale), hdrY, RGB(170, 185, 200))
		drawTextShadow(hdc, "K/D", cx+colW-scaleI32(86, uiScale), hdrY, RGB(170, 185, 200))
		drawTextShadow(hdc, "SCR", cx+colW-scaleI32(38, uiScale), hdrY, RGB(170, 185, 200))
	}
	drawColHeaders(col1X)
	drawColHeaders(col2X)

	hits := make([]ScoreboardRowHit, 0, len(allPlayers))
	drawColumn := func(list []PlayerInfo, cx int32, teamColor uint32) {
		limit := len(list)
		if limit > maxRows {
			limit = maxRows
		}
		for i := 0; i < limit; i++ {
			p := list[i]
			ry := hdrY + colHdrH + int32(i)*rowH
			rc := RECT{cx, ry, cx + colW, ry + rowH - 1}

			isTarget := hasSelectedTarget && p.ID == selectedTargetID
			isCursor := hasCursorID && p.ID == cursorTargetID
			isHover := cursorPos != nil && cursorPos.X >= rc.Left && cursorPos.X <= rc.Right && cursorPos.Y >= rc.Top && cursorPos.Y <= rc.Bottom

			if isTarget {
				procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), gdi.RowTargetBrush)
			} else if isHover {
				procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), gdi.RowHoverBrush)
			} else if isCursor {
				procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), gdi.RowCursorBrush)
			}

			stateTag := "DEAD"
			stateColor := RGB(130, 135, 145)
			nameColor := RGB(155, 160, 170)
			if p.IsAlive {
				stateTag = "LIVE"
				stateColor = RGB(80, 240, 120)
				nameColor = teamColor
				if p.InVehicle {
					stateTag = "VEH"
					stateColor = RGB(255, 200, 70)
				}
			}
			if isTarget {
				nameColor = RGB(255, 240, 80)
			}

			procSelectObject.Call(hdc, gdi.HFontSmall)
			drawTextShadow(hdc, fmt.Sprintf("%2d", p.ID), cx+scaleI32(4, uiScale), ry+2, RGB(200, 210, 220))
			drawTextShadow(hdc, stateTag, cx+scaleI32(34, uiScale), ry+2, stateColor)

			nick := p.Name
			if isTarget {
				nick = "> " + nick
			}
			if len(nick) > 22 {
				nick = nick[:21] + "…"
			}
			drawTextShadow(hdc, nick, cx+scaleI32(68, uiScale), ry+2, nameColor)
			drawTextShadow(hdc, fmt.Sprintf("%d/%d", p.Kills, p.Deaths), cx+colW-scaleI32(86, uiScale), ry+2, RGB(215, 225, 235))
			drawTextShadow(hdc, fmt.Sprintf("%d", p.Score), cx+colW-scaleI32(38, uiScale), ry+2, RGB(240, 240, 240))

			hits = append(hits, ScoreboardRowHit{
				Rect:     rc,
				PlayerID: p.ID,
				Name:     p.Name,
			})
		}
	}

	drawColumn(team1, col1X, RGB(120, 210, 255))
	drawColumn(team2, col2X, RGB(255, 145, 110))

	footY := by + boardH - footerH + scaleI32(4, uiScale)
	procSelectObject.Call(hdc, gdi.HFontSmall)
	drawTextShadow(hdc, "Tip: Click any player (Live or Dead) to lock Target immediately; Double-Click or Enter confirms & closes.",
		bx+pad, footY, RGB(170, 185, 205))

	return hits, flatOrder
}

func main() {
	runtime.LockOSThread()

	defer func() {
		if r := recover(); r != nil {
			f, err := os.OpenFile("crash.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				fmt.Fprintf(f, "[%s] PANIC: %v\n", time.Now().Format("2006-01-02 15:04:05"), r)
				buf := make([]byte, 16384)
				n := runtime.Stack(buf, false)
				f.Write(buf[:n])
				f.Close()
			}
		}
	}()

	enableDPIAwareness()

	targetIdFlag := flag.Int("target", -1, "Initial target player ID (CPlayer+0xB8)")
	flag.Parse()

	fmt.Println("====================================================")
	fmt.Println("      BF2142 v1.51 External Demo Analyzer           ")
	fmt.Println("      (Read-Only Transparent Overlay)               ")
	fmt.Println("====================================================")

	className, _ := syscall.UTF16PtrFromString("BF2142DemoAnalyzerOverlay")
	windowName, _ := syscall.UTF16PtrFromString("BF2142 Demo Analyzer")

	var wc WNDCLASSEXW
	wc.Size = uint32(unsafe.Sizeof(wc))
	wc.Style = 0x0008 // CS_DBLCLKS
	wc.WndProc = syscall.NewCallback(overlayWndProc)
	wc.ClassName = className

	if atom, _, regErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		fmt.Printf("[-] RegisterClassExW failed: %v\n", regErr)
		return
	}

	screenW, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	screenH, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
	if screenW < 300 {
		screenW = 1280
	}
	if screenH < 200 {
		screenH = 720
	}

	hwndOverlay, _, errCreate := procCreateWindowExW.Call(
		uintptr(WS_EX_TOPMOST|WS_EX_LAYERED|WS_EX_TRANSPARENT|WS_EX_TOOLWINDOW),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(WS_POPUP),
		0, 0, screenW, screenH,
		0, 0, 0, 0,
	)
	if hwndOverlay == 0 {
		fmt.Printf("[-] Failed to create overlay window: %v\n", errCreate)
		return
	}
	defer procDestroyWindow.Call(hwndOverlay)

	procSetLayeredWindowAttrs.Call(hwndOverlay, uintptr(RGB(0, 0, 0)), 0, uintptr(LWA_COLORKEY))
	procUpdateWindow.Call(hwndOverlay)

	gdi := initGDICache()
	defer gdi.Cleanup()

	hdcScreen, _, _ := procGetDC.Call(hwndOverlay)
	defer procReleaseDC.Call(hwndOverlay, hdcScreen)

	memDC, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	defer procDeleteDC.Call(memDC)

	var memBmp uintptr
	var backW, backH int32

	updateBackbuffer := func(w, h int32) bool {
		if w <= 0 || h <= 0 {
			return false
		}
		if memBmp == 0 || backW != w || backH != h {
			newBmp, _, _ := procCreateCompatibleBitmap.Call(hdcScreen, uintptr(w), uintptr(h))
			if newBmp == 0 {
				return false
			}
			if memBmp != 0 {
				procDeleteObject.Call(memBmp)
			}
			memBmp = newBmp
			backW = w
			backH = h
		}
		return true
	}
	defer func() {
		if memBmp != 0 {
			procDeleteObject.Call(memBmp)
		}
	}()

	var borderless BorderlessState
	defer borderless.Restore()

	aimLogFile, errLog := os.OpenFile("aim_log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if errLog != nil {
		fmt.Printf("[!] Warning: failed to open aim_log.txt: %v\n", errLog)
	} else {
		defer aimLogFile.Close()
		fmt.Fprintf(aimLogFile, "\n[%s] === BF2142 Demo Analyzer Started ===\n", time.Now().Format("2006-01-02 15:04:05"))
	}
	analyzer := NewAimAnalyzer(aimLogFile)

	// Runtime toggles
	showHUD := true
	showESP := true
	showAimCone := true
	scoreboardOpen := false
	overlayVisible := false
	isClickThrough := true

	laserMode := 0 // 0: Target Beam, 1: All Beams, 2: Target Endpoint, 3: All Endpoints, 4: Off
	laserLength := float32(40.0)
	aimFovDeg := float32(5.0)
	viewDistance := float32(400.0)
	fovScale := float32(1.0)

	selectedTargetID := int32(*targetIdFlag)
	hasSelectedTarget := *targetIdFlag >= 0

	scoreboardCursorIdx := 0
	var lastRowHits []ScoreboardRowHit
	var lastFlatOrder []int32
	borderlessMsg := ""
	borderlessMsgUntil := time.Time{}

	curOverlayX := int32(0)
	curOverlayY := int32(0)
	curOverlayW := int32(screenW)
	curOverlayH := int32(screenH)

	var game *GameAttachment
	defer func() {
		if game != nil {
			game.Close()
		}
	}()

	keyState := make(map[int]bool)
	clearKeyState := func() {
		for k := range keyState {
			delete(keyState, k)
		}
	}
	isKeyPressed := func(vk int) bool {
		ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
		pressed := (ret & 0x8000) != 0
		if pressed {
			if !keyState[vk] {
				keyState[vk] = true
				return true
			}
		} else {
			keyState[vk] = false
		}
		return false
	}

	hideOverlay := func() {
		if !isClickThrough {
			setOverlayClickThrough(hwndOverlay, true)
			isClickThrough = true
		}
		scoreboardOpen = false
		if overlayVisible {
			procShowWindow.Call(hwndOverlay, SW_HIDE)
			overlayVisible = false
		}
		clearKeyState()
	}

	closeScoreboard := func(refocusGame bool) {
		scoreboardOpen = false
		if !isClickThrough {
			setOverlayClickThrough(hwndOverlay, true)
			isClickThrough = true
		}
		clickMu.Lock()
		pendingClick = nil
		pendingDbl = false
		clickMu.Unlock()
		if refocusGame && game != nil && game.CachedHwnd != 0 {
			procSetForegroundWindow.Call(game.CachedHwnd)
		}
	}

	fmt.Println("[*] Waiting for BF2142.exe...")
	fmt.Println("[*] Hotkeys (active only when BF2142 is focused):")
	fmt.Println("    [Insert]      - HUD & Alignment frame")
	fmt.Println("    [Tab]         - Toggle interactive Scoreboard (Click / Up+Down+Enter)")
	fmt.Println("    [PageUp/Down] - Next/Prev Alive Target")
	fmt.Println("    [F6]          - Toggle ESP")
	fmt.Println("    [F7]          - Laser mode (Target Beam / All Beams / Target Dot / All Dots / Off)")
	fmt.Println("    [F8 / F9]     - Laser length -/+ 10m (20..300m)")
	fmt.Println("    [F10]         - Toggle 3D Target Aim FOV cone")
	fmt.Println("    [ [ / ] ]     - Decrease / Increase Aim FOV cone angle (1..20 deg)")
	fmt.Println("    [F11]         - Reset Aim Analyzer stats")
	fmt.Println("    [F12]         - Toggle Borderless Fullscreen (for windowed BF2142)")
	fmt.Println("    [ = / - ]     - Fine-tune FOV scale multiplier")
	fmt.Println("    [End]         - Cleanly restore & exit")
	fmt.Println()

	running := true
	var msg MSG
	badMgrFrames := 0

	for running {
		for {
			ret, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, PM_REMOVE)
			if ret == 0 {
				break
			}
			if msg.Message == WM_QUIT {
				running = false
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
		if !running {
			break
		}

		// Attach or verify BF2142.exe alive
		if game == nil || !game.Reader.IsProcessAlive() {
			if game != nil {
				fmt.Printf("[!] BF2142.exe (PID %d) closed. Waiting for relaunch...\n", game.PID)
				borderless.Restore()
				game.Close()
				game = nil
			}
			hideOverlay()
			att, err := tryAttachGame()
			if err != nil {
				time.Sleep(400 * time.Millisecond)
				continue
			}
			game = att
			fmt.Printf("[+] Attached to BF2142.exe (PID %d) | Base=0x%08X RendDX9=0x%08X\n",
				game.PID, game.BF2142Base, game.RendBase)
		}

		// Find/validate main game window
		if !isWindowValid(game.CachedHwnd, game.PID) {
			game.CachedHwnd = findGameWindow(game.PID)
		}
		if game.CachedHwnd == 0 {
			hideOverlay()
			time.Sleep(150 * time.Millisecond)
			continue
		}

		// Focus / Alt-Tab / Minimize guard
		resIconic, _, _ := procIsIconic.Call(game.CachedHwnd)
		fgHwnd, _, _ := procGetForegroundWindow.Call()
		isFocused := resIconic == 0 && (fgHwnd == game.CachedHwnd || (scoreboardOpen && fgHwnd == hwndOverlay))
		if !isFocused {
			hideOverlay()
			time.Sleep(80 * time.Millisecond)
			continue
		}

		// Match client rect & restore overlay visibility reliably
		var clientRect RECT
		var clientPoint POINT
		procGetClientRect.Call(game.CachedHwnd, uintptr(unsafe.Pointer(&clientRect)))
		procClientToScreen.Call(game.CachedHwnd, uintptr(unsafe.Pointer(&clientPoint)))

		targetW := clientRect.Right - clientRect.Left
		targetH := clientRect.Bottom - clientRect.Top
		targetX := clientPoint.X
		targetY := clientPoint.Y

		if targetW <= 100 || targetH <= 100 {
			hideOverlay()
			time.Sleep(80 * time.Millisecond)
			continue
		}

		if !overlayVisible || targetX != curOverlayX || targetY != curOverlayY || targetW != curOverlayW || targetH != curOverlayH {
			curOverlayX = targetX
			curOverlayY = targetY
			curOverlayW = targetW
			curOverlayH = targetH
			procSetWindowPos.Call(
				hwndOverlay,
				HWND_TOPMOST,
				uintptr(targetX), uintptr(targetY),
				uintptr(targetW), uintptr(targetH),
				SWP_NOACTIVATE|SWP_SHOWWINDOW,
			)
			overlayVisible = true
		}

		if !updateBackbuffer(curOverlayW, curOverlayH) {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		uiScale := clampF32(float32(curOverlayH)/720.0, 0.75, 2.5)
		gdi.UpdateScale(uiScale)

		// Process hotkeys ONLY while focused
		if isKeyPressed(VK_END) {
			running = false
			break
		}
		if isKeyPressed(VK_TAB) {
			if scoreboardOpen {
				closeScoreboard(true)
			} else {
				scoreboardOpen = true
				if isClickThrough {
					setOverlayClickThrough(hwndOverlay, false)
					isClickThrough = false
				}
			}
		}
		if scoreboardOpen && isKeyPressed(VK_ESCAPE) {
			closeScoreboard(true)
		}
		if isKeyPressed(VK_INSERT) {
			showHUD = !showHUD
		}
		if isKeyPressed(VK_F6) {
			showESP = !showESP
		}
		if isKeyPressed(VK_F7) {
			laserMode = (laserMode + 1) % 5
		}
		if isKeyPressed(VK_F8) {
			laserLength -= 10.0
			if laserLength < 20.0 {
				laserLength = 20.0
			}
		}
		if isKeyPressed(VK_F9) {
			laserLength += 10.0
			if laserLength > 300.0 {
				laserLength = 300.0
			}
		}
		if isKeyPressed(VK_F10) {
			showAimCone = !showAimCone
		}
		if isKeyPressed(VK_OEM_4) { // [
			aimFovDeg -= 1.0
			if aimFovDeg < 1.0 {
				aimFovDeg = 1.0
			}
		}
		if isKeyPressed(VK_OEM_6) { // ]
			aimFovDeg += 1.0
			if aimFovDeg > 20.0 {
				aimFovDeg = 20.0
			}
		}
		if isKeyPressed(VK_F12) {
			borderlessMsg = borderless.Toggle(game.CachedHwnd)
			borderlessMsgUntil = time.Now().Add(3 * time.Second)
		}
		if isKeyPressed(VK_OEM_PLUS) || isKeyPressed(VK_ADD) {
			fovScale = clampF32(fovScale+0.02, 0.4, 2.5)
		}
		if isKeyPressed(VK_OEM_MINUS) || isKeyPressed(VK_SUBTRACT) {
			fovScale = clampF32(fovScale-0.02, 0.4, 2.5)
		}

		// Resolve pointers (with ClassManager fallback)
		playerMgr, _ := game.Reader.ReadUint32(game.PlayerMgrPtrAddr)
		renderer, _ := game.Reader.ReadUint32(game.RendererPtrAddr)
		if !isValidUserAddr(playerMgr) || !isValidUserAddr(renderer) {
			if classMgr, errCM := game.Reader.ReadUint32(game.ClassMgrPtrAddr); errCM == nil && isValidUserAddr(classMgr) {
				classes := traverseClassManager(game.Reader, classMgr)
				if !isValidUserAddr(playerMgr) {
					playerMgr = classes["PlayerManager"]
				}
				if !isValidUserAddr(renderer) {
					renderer = classes["Renderer"]
				}
			}
		}

		// Read Camera Data with verified 1.51 offsets & runtime guardrail
		var cam CameraData
		if isValidUserAddr(renderer) {
			if activeCam, errAC := game.Reader.ReadUint32(renderer + 0xF8); errAC == nil && isValidUserAddr(activeCam) {
				if pos, errP := game.Reader.ReadVector3(activeCam + 0x70); errP == nil && isValidVec3(pos) {
					cam.Pos = pos
				}
				if vm, errV := game.Reader.ReadMatrix4x4(activeCam + 0x80); errV == nil && isValidMatrix(vm) {
					cam.ViewMatrix = vm
				}
				if pm, errProj := game.Reader.ReadMatrix4x4(activeCam + 0x140); errProj == nil && isValidProjMatrix(pm) {
					cam.ProjMatrix = pm
					cam.HasValidProj = true
				}
				if vpW, errW := game.Reader.ReadFloat32(activeCam + 0x10); errW == nil && isFiniteFloat(vpW) {
					cam.VpWidth = vpW
				}
				if vpH, errH := game.Reader.ReadFloat32(activeCam + 0x14); errH == nil && isFiniteFloat(vpH) {
					cam.VpHeight = vpH
				}
				if fov1C, errF := game.Reader.ReadFloat32(activeCam + 0x1C); errF == nil && isFiniteFloat(fov1C) {
					cam.FovRad1C = fov1C
				}
			}
		}
		if cam.VpWidth <= 100 || cam.VpHeight <= 100 {
			cam.VpWidth = float32(curOverlayW)
			cam.VpHeight = float32(curOverlayH)
		}

		// Read ALL Players (both alive & dead for scoreboard + target retention)
		var allPlayers []PlayerInfo
		var alivePlayers []PlayerInfo
		if isValidUserAddr(playerMgr) {
			listHead, errH := game.Reader.ReadUint32(playerMgr + 0x10)
			playerCount, errC := game.Reader.ReadUint32(playerMgr + 0x14)
			if errH == nil && errC == nil && isValidUserAddr(listHead) && playerCount > 0 && playerCount <= 64 {
				badMgrFrames = 0
				curr, _ := game.Reader.ReadUint32(listHead)
				visitedNodes := make(map[uint32]bool)

				for i := uint32(0); i < playerCount && isValidUserAddr(curr) && curr != listHead && !visitedNodes[curr]; i++ {
					visitedNodes[curr] = true
					playerPtr, errP := game.Reader.ReadUint32(curr + 0x08)
					if errP == nil && isValidUserAddr(playerPtr) {
						pidVal, errID := game.Reader.ReadInt32(playerPtr + 0xB8)
						if errID != nil {
							pidVal = int32(i)
						}

						cacheKey := (uint64(playerPtr) << 32) | uint64(uint32(pidVal))
						name, okName := game.NameCache[cacheKey]
						if !okName || name == "" || name == "<unknown>" {
							name = game.Reader.ReadCPlayerName(playerPtr)
							if name != "" && name != "<unknown>" {
								if len(game.NameCache) > 256 {
									game.NameCache = make(map[uint64]string)
								}
								game.NameCache[cacheKey] = name
							}
						}

						team, errTeam := game.Reader.ReadUint32(playerPtr + 0xD8)
						if errTeam != nil {
							team = 0
						}

						// Determine IsAlive reliably from CViewMatrix (+0x84):
						// valid finite EyePos not at (0,0,0)/(0,1,0) (eps 0.05) and |LookDir| > 0.5
						inVeh := false
						var eyePos, lookDir Vector3
						var dist float32
						hasPose := false

						if vmPtr, errVM := game.Reader.ReadUint32(playerPtr + 0x84); errVM == nil && isValidUserAddr(vmPtr) {
							ld, errD := game.Reader.ReadVector3(vmPtr + 0xE8)
							ep, errE := game.Reader.ReadVector3(vmPtr + 0xF8)
							lookLen := float32(math.Sqrt(float64(ld.X*ld.X + ld.Y*ld.Y + ld.Z*ld.Z)))
							if errD == nil && errE == nil && isValidVec3(ep) && isValidVec3(ld) && lookLen > 0.5 {
								isOrigin := math.Abs(float64(ep.X)) < 0.05 && math.Abs(float64(ep.Y)) < 0.05 && math.Abs(float64(ep.Z)) < 0.05
								isDefault1 := math.Abs(float64(ep.X)) < 0.05 && math.Abs(float64(ep.Y-1.0)) < 0.05 && math.Abs(float64(ep.Z)) < 0.05
								if !isOrigin && !isDefault1 {
									eyePos = ep
									lookDir = vec3Normalize(ld)
									dist = vec3Dist(eyePos, cam.Pos)
									hasPose = true
								}
							}
						}

						var scoreVal, killsVal, deathsVal int32
						if scorePtr, errS := game.Reader.ReadUint32(playerPtr + 0x100); errS == nil && isValidUserAddr(scorePtr) {
							scoreVal, _ = game.Reader.ReadInt32(scorePtr + 0x00)
							deathsVal, _ = game.Reader.ReadInt32(scorePtr + 0x10)
							killsVal, _ = game.Reader.ReadInt32(scorePtr + 0x14)
						}

						info := PlayerInfo{
							Addr:      playerPtr,
							ID:        pidVal,
							Name:      name,
							Team:      team,
							IsAlive:   hasPose,
							InVehicle: inVeh,
							Score:     scoreVal,
							Kills:     killsVal,
							Deaths:    deathsVal,
							EyePos:    eyePos,
							LookDir:   lookDir,
							HasPose:   hasPose,
							Distance:  dist,
						}
						allPlayers = append(allPlayers, info)
						if info.IsAlive {
							alivePlayers = append(alivePlayers, info)
						}
					}
					curr, _ = game.Reader.ReadUint32(curr)
				}
			} else {
				badMgrFrames++
				if badMgrFrames > 60 {
					game.NameCache = make(map[uint64]string)
					badMgrFrames = 0
				}
			}
		}

		// Handle Scoreboard input (Clicks & Up/Down/Enter)
		targetChanged := false
		if scoreboardOpen {
			// Mouse clicks from WndProc
			clickMu.Lock()
			pt := pendingClick
			dbl := pendingDbl
			pendingClick = nil
			pendingDbl = false
			clickMu.Unlock()

			if pt != nil {
				for _, hit := range lastRowHits {
					if pt.X >= hit.Rect.Left && pt.X <= hit.Rect.Right && pt.Y >= hit.Rect.Top && pt.Y <= hit.Rect.Bottom {
						if !hasSelectedTarget || selectedTargetID != hit.PlayerID {
							selectedTargetID = hit.PlayerID
							hasSelectedTarget = true
							targetChanged = true
						}
						if dbl {
							closeScoreboard(true)
						}
						break
					}
				}
			}

			if len(lastFlatOrder) > 0 {
				if scoreboardCursorIdx >= len(lastFlatOrder) {
					scoreboardCursorIdx = len(lastFlatOrder) - 1
				}
				if isKeyPressed(VK_DOWN) {
					scoreboardCursorIdx = (scoreboardCursorIdx + 1) % len(lastFlatOrder)
				}
				if isKeyPressed(VK_UP) {
					scoreboardCursorIdx = (scoreboardCursorIdx - 1 + len(lastFlatOrder)) % len(lastFlatOrder)
				}
				if isKeyPressed(VK_RETURN) {
					pickID := lastFlatOrder[scoreboardCursorIdx]
					if !hasSelectedTarget || selectedTargetID != pickID {
						selectedTargetID = pickID
						hasSelectedTarget = true
						targetChanged = true
					}
					closeScoreboard(true)
				}
			}
		} else {
			// Clear stale clicks if scoreboard closed
			clickMu.Lock()
			pendingClick = nil
			pendingDbl = false
			clickMu.Unlock()
		}

		// Initial target bootstrap (only if no target has ever been selected yet)
		if !hasSelectedTarget {
			if len(alivePlayers) > 0 {
				selectedTargetID = alivePlayers[0].ID
				hasSelectedTarget = true
				targetChanged = true
			} else if len(allPlayers) > 0 {
				selectedTargetID = allPlayers[0].ID
				hasSelectedTarget = true
				targetChanged = true
			}
		}

		// PageUp / PageDown cycles alive players manually (keeps selected ID across deaths!)
		if len(alivePlayers) > 0 {
			aliveIdx := -1
			for i, p := range alivePlayers {
				if p.ID == selectedTargetID {
					aliveIdx = i
					break
				}
			}
			if isKeyPressed(VK_NEXT) {
				if aliveIdx < 0 {
					aliveIdx = 0
				} else {
					aliveIdx = (aliveIdx + 1) % len(alivePlayers)
				}
				if selectedTargetID != alivePlayers[aliveIdx].ID {
					selectedTargetID = alivePlayers[aliveIdx].ID
					targetChanged = true
				}
			}
			if isKeyPressed(VK_PRIOR) {
				if aliveIdx < 0 {
					aliveIdx = len(alivePlayers) - 1
				} else {
					aliveIdx = (aliveIdx - 1 + len(alivePlayers)) % len(alivePlayers)
				}
				if selectedTargetID != alivePlayers[aliveIdx].ID {
					selectedTargetID = alivePlayers[aliveIdx].ID
					targetChanged = true
				}
			}
		}

		// Locate current selected target in allPlayers (so dead targets stay selected cleanly)
		var targetPlayer *PlayerInfo
		for i := range allPlayers {
			if allPlayers[i].ID == selectedTargetID {
				targetPlayer = &allPlayers[i]
				break
			}
		}

		if isKeyPressed(VK_F11) || targetChanged {
			tName := fmt.Sprintf("ID %d", selectedTargetID)
			if targetPlayer != nil {
				tName = targetPlayer.Name
			}
			analyzer.Reset(tName)
		}

		// Aim Analyzer calculation
		var closestEnemy *PlayerInfo
		var closestAimPt Vector3
		var minAngle float32 = 999.0
		var closestDist float32

		if targetPlayer != nil && targetPlayer.IsAlive && targetPlayer.HasPose {
			for i := range alivePlayers {
				p := &alivePlayers[i]
				if p.ID == targetPlayer.ID {
					continue
				}
				if targetPlayer.Team != 0 && p.Team == targetPlayer.Team {
					continue
				}

				aimPt, ang := closestAimPoint(targetPlayer, p)
				if ang < minAngle {
					minAngle = ang
					closestEnemy = p
					closestAimPt = aimPt
					closestDist = vec3Dist(p.EyePos, targetPlayer.EyePos)
				}
			}

			now := time.Now()
			isNewTick := !analyzer.HasPrevSample ||
				angleBetween(analyzer.PrevTargetLook, targetPlayer.LookDir) > 0.005 ||
				vec3Dist(analyzer.PrevTargetPos, targetPlayer.EyePos) > 0.005 ||
				(closestEnemy != nil && vec3Dist(analyzer.PrevEnemyPos, closestEnemy.EyePos) > 0.005)

			if isNewTick && closestEnemy != nil {
				analyzer.PrevTargetLook = targetPlayer.LookDir
				analyzer.PrevTargetPos = targetPlayer.EyePos
				analyzer.PrevEnemyPos = closestEnemy.EyePos
				analyzer.HasPrevSample = true

				tick := AimTick{
					Time:      now,
					LookDir:   targetPlayer.LookDir,
					EnemyID:   closestEnemy.ID,
					EnemyName: closestEnemy.Name,
					EnemyDist: closestDist,
					Angle:     minAngle,
					EnemyPos:  closestEnemy.EyePos,
				}
				analyzer.Ticks = append(analyzer.Ticks, tick)
				if len(analyzer.Ticks) > 30 {
					analyzer.Ticks = analyzer.Ticks[len(analyzer.Ticks)-30:]
				}
				analyzer.checkSnap(now, targetPlayer.Name)
			}

			analyzer.updateLock(now, targetPlayer.Name, closestEnemy, minAngle, closestDist)
		} else if targetPlayer != nil {
			analyzer.PauseOnDeath(targetPlayer.Name)
		} else {
			analyzer.PauseOnDeath("None")
		}

		// Render Frame
		oldBmp, _, _ := procSelectObject.Call(memDC, memBmp)
		oldFont, _, _ := procSelectObject.Call(memDC, gdi.HFont)
		oldPen, _, _ := procSelectObject.Call(memDC, gdi.HudBorderPen)
		oldBrush, _, _ := procSelectObject.Call(memDC, gdi.NullBrush)

		clearRect := RECT{0, 0, curOverlayW, curOverlayH}
		procFillRect.Call(memDC, uintptr(unsafe.Pointer(&clearRect)), gdi.BlackBrush)
		procSetBkMode.Call(memDC, TRANSPARENT)

		screenWFloat := float32(curOverlayW)
		screenHFloat := float32(curOverlayH)

		// 1. 1px Alignment border (with HUD)
		if showHUD {
			procSelectObject.Call(memDC, gdi.PenGreenBorder)
			procSelectObject.Call(memDC, gdi.NullBrush)
			procRectangle.Call(memDC, 0, 0, uintptr(curOverlayW), uintptr(curOverlayH))
		}

		// 2. Target 3D Aim FOV cone ring (projected around suspect LookDir)
		if showAimCone && targetPlayer != nil && targetPlayer.IsAlive && targetPlayer.HasPose {
			coneDist := laserLength
			if closestEnemy != nil && minAngle <= aimFovDeg*1.5 && closestDist >= 3.0 && closestDist < coneDist {
				coneDist = closestDist
			}
			renderAimCone3D(memDC, &cam, targetPlayer.EyePos, targetPlayer.LookDir, coneDist,
				aimFovDeg, screenWFloat, screenHFloat, fovScale, gdi.PenAimCone, gdi.PenShadow)

			// If closest enemy falls inside Target FOV cone, draw visual aim-offset tether
			if closestEnemy != nil && minAngle <= aimFovDeg {
				rayAtEnemy := Vector3{
					X: targetPlayer.EyePos.X + targetPlayer.LookDir.X*closestDist,
					Y: targetPlayer.EyePos.Y + targetPlayer.LookDir.Y*closestDist,
					Z: targetPlayer.EyePos.Z + targetPlayer.LookDir.Z*closestDist,
				}
				if t1, t2, okT := cam.ProjectLine3D(rayAtEnemy, closestAimPt, screenWFloat, screenHFloat, fovScale); okT {
					procSelectObject.Call(memDC, gdi.PenAimEnemy)
					procMoveToEx.Call(memDC, uintptr(t1.X), uintptr(t1.Y), 0)
					procLineTo.Call(memDC, uintptr(t2.X), uintptr(t2.Y))
				}
			}
		}

		// 3. 1px Lasers & Endpoint-Only Mode
		if laserMode != 4 && len(alivePlayers) > 0 {
			beamsOnlyTarget := (laserMode == 0 || laserMode == 2)
			endpointOnly := (laserMode == 2 || laserMode == 3)

			for _, p := range alivePlayers {
				isTarget := hasSelectedTarget && p.ID == selectedTargetID
				if beamsOnlyTarget && !isTarget {
					continue
				}

				curLen := laserLength
				if !isTarget {
					curLen = 18.0
					if curLen > laserLength {
						curLen = laserLength
					}
				}

				p1 := p.EyePos
				p2 := Vector3{
					X: p1.X + p.LookDir.X*curLen,
					Y: p1.Y + p.LookDir.Y*curLen,
					Z: p1.Z + p.LookDir.Z*curLen,
				}

				var penLine, brushDot uintptr
				if isTarget {
					penLine = gdi.PenTargetLaser
					brushDot = gdi.BrushTargetDot
				} else if p.Team == 1 {
					penLine = gdi.PenTeam1
					brushDot = gdi.BrushTeam1Dot
				} else {
					penLine = gdi.PenTeam2
					brushDot = gdi.BrushTeam2Dot
				}

				if !endpointOnly {
					s1, s2, okLine := cam.ProjectLine3D(p1, p2, screenWFloat, screenHFloat, fovScale)
					if okLine {
						procSelectObject.Call(memDC, penLine)
						procMoveToEx.Call(memDC, uintptr(s1.X), uintptr(s1.Y), 0)
						procLineTo.Call(memDC, uintptr(s2.X), uintptr(s2.Y))

						if sEye, okEye := cam.WorldToScreen(p1, screenWFloat, screenHFloat, fovScale); okEye {
							if sEye.X >= 0 && sEye.X < curOverlayW && sEye.Y >= 0 && sEye.Y < curOverlayH {
								procSelectObject.Call(memDC, brushDot)
								procEllipse.Call(memDC, uintptr(sEye.X-2), uintptr(sEye.Y-2), uintptr(sEye.X+3), uintptr(sEye.Y+3))
							}
						}
					}
				}

				// Always draw endpoint marker in Endpoint-Only modes (or crisp terminal dot on Target beam)
				if endpointOnly || isTarget {
					if sEnd, okEnd := cam.WorldToScreen(p2, screenWFloat, screenHFloat, fovScale); okEnd {
						if sEnd.X >= 0 && sEnd.X < curOverlayW && sEnd.Y >= 0 && sEnd.Y < curOverlayH {
							markerR := scaleI32(4, uiScale)
							if !isTarget {
								markerR = scaleI32(3, uiScale)
							}
							drawDiamondMarker(memDC, sEnd.X, sEnd.Y, markerR, penLine, gdi.PenShadow, brushDot)
						}
					}
				}
			}
		}

		// 4. 2D ESP Boxes & Labels
		if showESP && len(alivePlayers) > 0 {
			procSelectObject.Call(memDC, gdi.NullBrush)

			for _, p := range alivePlayers {
				if p.Distance > viewDistance {
					continue
				}
				isTarget := hasSelectedTarget && p.ID == selectedTargetID

				headOff, feetOff := float32(0.25), float32(1.55)
				if p.InVehicle {
					headOff, feetOff = 0.45, 0.85
				}
				headPos := Vector3{X: p.EyePos.X, Y: p.EyePos.Y + headOff, Z: p.EyePos.Z}
				feetPos := Vector3{X: p.EyePos.X, Y: p.EyePos.Y - feetOff, Z: p.EyePos.Z}

				sHead, okH := cam.WorldToScreen(headPos, screenWFloat, screenHFloat, fovScale)
				sFeet, okF := cam.WorldToScreen(feetPos, screenWFloat, screenHFloat, fovScale)
				if !okH || !okF {
					continue
				}
				if sHead.X < -200 || sHead.X > curOverlayW+200 || sFeet.Y < -200 || sHead.Y > curOverlayH+200 {
					continue
				}

				boxH := sFeet.Y - sHead.Y
				if boxH < 6 {
					boxH = 6
				}
				if boxH > curOverlayH {
					boxH = curOverlayH
				}
				boxW := int32(float32(boxH) * 0.45)
				if p.InVehicle {
					boxW = int32(float32(boxH) * 0.90)
				}
				if boxW < 4 {
					boxW = 4
				}

				left := sHead.X - boxW/2
				top := sHead.Y
				right := left + boxW
				bottom := top + boxH

				var penBox uintptr
				var textColor uint32
				if isTarget {
					penBox = gdi.PenTargetBox
					textColor = RGB(255, 230, 0)
				} else if p.Team == 1 {
					penBox = gdi.PenTeam1
					textColor = RGB(50, 180, 255)
				} else {
					penBox = gdi.PenTeam2
					textColor = RGB(255, 100, 60)
				}

				drawCornerBox(memDC, left, top, right, bottom, penBox, gdi.PenShadow)
				if isTarget {
					procSelectObject.Call(memDC, gdi.PenTargetBox)
					procRectangle.Call(memDC, uintptr(left), uintptr(top), uintptr(right), uintptr(bottom))
				}

				aimBadgeCutoff := float32(15.0)
				if aimFovDeg > aimBadgeCutoff {
					aimBadgeCutoff = aimFovDeg
				}
				isClosestAimEnemy := (closestEnemy != nil && p.ID == closestEnemy.ID && minAngle <= aimBadgeCutoff && !isTarget)
				if isClosestAimEnemy {
					drawCornerBox(memDC, left-2, top-2, right+2, bottom+2, gdi.PenAimEnemy, gdi.PenShadow)
					aimBadge := fmt.Sprintf("▲ AIM: %.2f°", minAngle)
					drawTextShadow(memDC, aimBadge, left-2, top-scaleI32(28, uiScale), RGB(220, 80, 255))
				}

				dx := sHead.X - curOverlayW/2
				dy := sHead.Y - curOverlayH/2
				centerRadius := scaleI32(150, uiScale)
				isNearCenter := (dx*dx + dy*dy) <= (centerRadius * centerRadius)
				if isTarget || p.Distance <= 60.0 || isNearCenter || isClosestAimEnemy {
					topLabel := p.Name
					if p.InVehicle {
						topLabel += " [VEH]"
					}
					if isTarget {
						topLabel = "[TARGET] " + topLabel
					}
					drawTextShadow(memDC, topLabel, left-2, top-scaleI32(15, uiScale), textColor)
				}

				distLabel := fmt.Sprintf("%.0fm", p.Distance)
				drawTextShadow(memDC, distLabel, left+boxW/2-scaleI32(10, uiScale), bottom+2, RGB(220, 220, 220))
			}
		}

		// 5. Dynamic Scaled HUD Panel
		if showHUD {
			padX := scaleI32(10, uiScale)
			rowStep := scaleI32(19, uiScale)
			hudX := scaleI32(14, uiScale)
			hudY := scaleI32(14, uiScale)

			projTag := "Proj+0x140"
			if !cam.HasValidProj {
				projTag = "Fallback+0x1C"
			}
			vFovDeg := cam.GetVerticalFovDeg()
			row1 := fmt.Sprintf("Win: %dx%d @ (%d,%d) | %s | vFOV: %.1f° | Scale: %.2fx",
				curOverlayW, curOverlayH, curOverlayX, curOverlayY, projTag, vFovDeg, fovScale)

			targetText := "Target: None"
			targetColor := RGB(255, 255, 255)
			if targetPlayer != nil {
				status := "LIVE"
				if !targetPlayer.IsAlive {
					status = "DEAD"
					targetColor = RGB(255, 130, 110)
				} else if targetPlayer.InVehicle {
					status = "VEH"
				}
				targetText = fmt.Sprintf("Target: [%d] %s (Team %d, %s) | K/D: %d/%d (%d) | Dist: %.1fm",
					targetPlayer.ID, targetPlayer.Name, targetPlayer.Team, status,
					targetPlayer.Kills, targetPlayer.Deaths, targetPlayer.Score, targetPlayer.Distance)
			}

			var aimTargetStr string
			var aimColor uint32
			if targetPlayer != nil && !targetPlayer.IsAlive {
				aimTargetStr = "[WAITING RESPAWN]"
				aimColor = RGB(255, 140, 100)
			} else if closestEnemy != nil {
				aimTargetStr = fmt.Sprintf("[%s]: %.2f° (%.0fm)", closestEnemy.Name, minAngle, closestDist)
				if minAngle <= 1.5 {
					aimColor = RGB(255, 70, 70)
				} else if minAngle <= aimFovDeg {
					aimColor = RGB(255, 230, 0)
				} else {
					aimColor = RGB(170, 170, 170)
				}
			} else {
				aimTargetStr = "[None]"
				aimColor = RGB(170, 170, 170)
			}
			snapStatsStr := " | Snaps: 0"
			if analyzer.SnapCount > 0 {
				snapStatsStr = fmt.Sprintf(" | Last: %s | Snaps: %d (max %.1f°)",
					analyzer.LastSnapText, analyzer.SnapCount, analyzer.MaxSnapAngle)
			}
			lockStr := ""
			if analyzer.IsLocking && (analyzer.CurrentLockDuration >= 0.1 || minAngle <= 1.5) {
				lockStr = fmt.Sprintf(" [LOCK %.1fs]", analyzer.CurrentLockDuration)
			}
			aimFull := "Aim -> " + aimTargetStr + snapStatsStr + lockStr

			laserNames := []string{"Target Beam", "All Beams", "Target Endpoint", "All Endpoints", "OFF"}
			espTxt := "OFF"
			if showESP {
				espTxt = "ON"
			}
			coneTxt := "OFF"
			if showAimCone {
				coneTxt = fmt.Sprintf("%.0f°", aimFovDeg)
			}
			row4 := fmt.Sprintf("Laser: %s (%.0fm) | ESP: %s | AimCone: %s | Borderless: %v",
				laserNames[laserMode], laserLength, espTxt, coneTxt, borderless.Active)
			if borderlessMsg != "" && time.Now().Before(borderlessMsgUntil) {
				row4 += " [" + borderlessMsg + "]"
			}
			row5 := "[Tab] Scoreboard | [PgUp/PgDn] Alive | [F6] ESP | [F7] Laser | [F8/F9] Len | [F10 [/]] Cone | [F11] Reset | [F12] Borderless | [Ins] HUD | [End] Exit"

			procSelectObject.Call(memDC, gdi.HFont)
			maxW := getTextWidth(memDC, row1)
			for _, line := range []string{targetText, aimFull, row4, row5} {
				if w := getTextWidth(memDC, line); w > maxW {
					maxW = w
				}
			}
			hudW := maxW + padX*2
			if hudW < scaleI32(520, uiScale) {
				hudW = scaleI32(520, uiScale)
			}
			if hudW > curOverlayW-hudX*2 {
				hudW = curOverlayW - hudX*2
			}
			hudH := scaleI32(34, uiScale) + rowStep*5 + scaleI32(8, uiScale)

			rcHud := RECT{hudX, hudY, hudX + hudW, hudY + hudH}
			procFillRect.Call(memDC, uintptr(unsafe.Pointer(&rcHud)), gdi.HudBrush)
			procSelectObject.Call(memDC, gdi.HudBorderPen)
			procSelectObject.Call(memDC, gdi.NullBrush)
			procRectangle.Call(memDC, uintptr(rcHud.Left), uintptr(rcHud.Top), uintptr(rcHud.Right), uintptr(rcHud.Bottom))

			procSelectObject.Call(memDC, gdi.HFontBold)
			drawTextShadow(memDC, "BF2142 Demo Analyzer v1.51 (External Read-Only)", hudX+padX, hudY+scaleI32(7, uiScale), RGB(255, 215, 0))

			procSelectObject.Call(memDC, gdi.HFont)
			y0 := hudY + scaleI32(28, uiScale)
			drawTextShadow(memDC, row1, hudX+padX, y0, RGB(180, 230, 255))
			drawTextShadow(memDC, targetText, hudX+padX, y0+rowStep, targetColor)

			aimX := hudX + padX
			aimY := y0 + rowStep*2
			drawTextShadow(memDC, "Aim -> ", aimX, aimY, RGB(180, 230, 255))
			aimX += getTextWidth(memDC, "Aim -> ")
			drawTextShadow(memDC, aimTargetStr, aimX, aimY, aimColor)
			aimX += getTextWidth(memDC, aimTargetStr)
			drawTextShadow(memDC, snapStatsStr, aimX, aimY, RGB(210, 225, 240))
			aimX += getTextWidth(memDC, snapStatsStr)
			if lockStr != "" {
				drawTextShadow(memDC, lockStr, aimX, aimY, RGB(255, 80, 60))
			}

			drawTextShadow(memDC, row4, hudX+padX, y0+rowStep*3, RGB(200, 225, 250))
			drawTextShadow(memDC, row5, hudX+padX, y0+rowStep*4, RGB(165, 185, 205))
		}

		// 6. Interactive Toggle Tab Scoreboard
		if scoreboardOpen {
			var curPt POINT
			procGetCursorPos.Call(uintptr(unsafe.Pointer(&curPt)))
			procScreenToClient.Call(hwndOverlay, uintptr(unsafe.Pointer(&curPt)))
			lastRowHits, lastFlatOrder = renderScoreboard(
				memDC, gdi, uiScale,
				curOverlayW, curOverlayH,
				allPlayers, selectedTargetID, hasSelectedTarget,
				scoreboardCursorIdx, &curPt,
			)
		}

		procBitBlt.Call(hdcScreen, 0, 0, uintptr(curOverlayW), uintptr(curOverlayH), memDC, 0, 0, SRCCOPY)

		// Restore DC selections cleanly
		procSelectObject.Call(memDC, oldBrush)
		procSelectObject.Call(memDC, oldPen)
		procSelectObject.Call(memDC, oldFont)
		procSelectObject.Call(memDC, oldBmp)

		time.Sleep(16 * time.Millisecond)
	}

	fmt.Println("[*] Overlay exited cleanly.")
}
