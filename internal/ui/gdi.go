package ui

import (
	"syscall"
	"unsafe"

	"demoanalyzer/internal/math3d"
)

var (
	modGdi32 = syscall.NewLazyDLL("gdi32.dll")

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
	PS_SOLID    = 0
	TRANSPARENT = 1
	SRCCOPY     = 0x00CC0020
	NULL_BRUSH  = 5
)

func RGB(r, g, b byte) uint32 {
	return uint32(r) | (uint32(g) << 8) | (uint32(b) << 16)
}

func GetTextWidth(hdc uintptr, text string) int32 {
	var sz SIZE
	u16, err := syscall.UTF16FromString(text)
	if err != nil || len(u16) <= 1 {
		return 0
	}
	procGetTextExtentPoint32W.Call(hdc, uintptr(unsafe.Pointer(&u16[0])), uintptr(len(u16)-1), uintptr(unsafe.Pointer(&sz)))
	return sz.CX
}

func DrawTextShadow(hdc uintptr, text string, x, y int32, color uint32) {
	u16, err := syscall.UTF16FromString(text)
	if err != nil || len(u16) <= 1 {
		return
	}
	ptr := uintptr(unsafe.Pointer(&u16[0]))
	length := uintptr(len(u16) - 1)

	// Single clean TextOutW call at (x, y) with specified color.
	// Removed the duplicate offset shadow call at (x+1, y+1) in RGB(1,1,1)
	// which caused unreadable blurry ghosting / double-text under ClearType on layered windows.
	procSetTextColor.Call(hdc, uintptr(color))
	procTextOutW.Call(hdc, uintptr(x), uintptr(y), ptr, length)
}

func DrawCornerBox(hdc uintptr, left, top, right, bottom int32, penMain, penShadow uintptr) {
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

func DrawDiamondMarker(hdc uintptr, cx, cy, r int32, penMain, penShadow, brushFill uintptr) {
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

// DrawAimSilhouette renders a slim, sleek aim-style contour / silhouette brackets
// around each player's body proportions (shoulder -> waist -> feet subtle taper)
// instead of a full square/box.
func DrawAimSilhouette(hdc uintptr, cx, top, bottom, boxW, boxH int32, inVehicle, isTarget bool, penMain, penShadow uintptr) {
	if boxH < 6 {
		return
	}
	halfW := boxW / 2
	if halfW < 3 {
		halfW = 3
	}

	drawContour := func(ox, oy int32) {
		if inVehicle {
			// Vehicle brackets: sleek tactical outer corner pair
			bracketW := halfW / 4
			if bracketW < 4 {
				bracketW = 4
			}
			l := cx - halfW + ox
			r := cx + halfW + ox
			t := top + oy
			b := bottom + oy

			// Left bracket
			procMoveToEx.Call(hdc, uintptr(l+bracketW), uintptr(t), 0)
			procLineTo.Call(hdc, uintptr(l), uintptr(t))
			procLineTo.Call(hdc, uintptr(l), uintptr(b))
			procLineTo.Call(hdc, uintptr(l+bracketW), uintptr(b))

			// Right bracket
			procMoveToEx.Call(hdc, uintptr(r-bracketW), uintptr(t), 0)
			procLineTo.Call(hdc, uintptr(r), uintptr(t))
			procLineTo.Call(hdc, uintptr(r), uintptr(b))
			procLineTo.Call(hdc, uintptr(r-bracketW), uintptr(b))
		} else {
			// Infantry aim contour: narrow shoulder/torso/leg vertical side contours with subtle top/bottom taper
			shoulderW := halfW
			waistW := int32(float32(halfW) * 0.82)
			feetW := int32(float32(halfW) * 0.70)
			if waistW < 2 {
				waistW = 2
			}
			if feetW < 2 {
				feetW = 2
			}

			yShoulder := top + int32(float32(boxH)*0.16) + oy
			yWaist := top + int32(float32(boxH)*0.55) + oy
			yFeet := bottom + oy
			tick := halfW / 3
			if tick < 2 {
				tick = 2
			}
			if tick > 6 {
				tick = 6
			}

			// Left side contour: shoulder tick -> shoulder -> waist -> feet -> feet tick
			procMoveToEx.Call(hdc, uintptr(cx-shoulderW+tick+ox), uintptr(yShoulder), 0)
			procLineTo.Call(hdc, uintptr(cx-shoulderW+ox), uintptr(yShoulder))
			procLineTo.Call(hdc, uintptr(cx-waistW+ox), uintptr(yWaist))
			procLineTo.Call(hdc, uintptr(cx-feetW+ox), uintptr(yFeet))
			procLineTo.Call(hdc, uintptr(cx-feetW+tick+ox), uintptr(yFeet))

			// Right side contour: shoulder tick -> shoulder -> waist -> feet -> feet tick
			procMoveToEx.Call(hdc, uintptr(cx+shoulderW-tick+ox), uintptr(yShoulder), 0)
			procLineTo.Call(hdc, uintptr(cx+shoulderW+ox), uintptr(yShoulder))
			procLineTo.Call(hdc, uintptr(cx+waistW+ox), uintptr(yWaist))
			procLineTo.Call(hdc, uintptr(cx+feetW+ox), uintptr(yFeet))
			procLineTo.Call(hdc, uintptr(cx+feetW-tick+ox), uintptr(yFeet))
		}
	}

	procSelectObject.Call(hdc, penShadow)
	drawContour(1, 1)

	procSelectObject.Call(hdc, penMain)
	drawContour(0, 0)
}

// DrawHeadMarker renders a small crisp head circle / dot at HeadPos, with a lock-on crosshair tick for Target.
func DrawHeadMarker(hdc uintptr, cx, headY, boxH int32, isTarget bool, penMain, penShadow, brushDot, nullBrush uintptr) {
	headR := int32(float32(boxH) * 0.08)
	if headR < 2 {
		headR = 2
	}
	if headR > 7 {
		headR = 7
	}
	cy := headY + headR

	drawCircle := func(ox, oy int32) {
		procEllipse.Call(hdc, uintptr(cx-headR+ox), uintptr(cy-headR+oy), uintptr(cx+headR+1+ox), uintptr(cy+headR+1+oy))
	}

	// 1. Shadow outline
	procSelectObject.Call(hdc, penShadow)
	procSelectObject.Call(hdc, nullBrush)
	drawCircle(1, 1)

	// 2. Main circle
	procSelectObject.Call(hdc, penMain)
	drawCircle(0, 0)

	// 3. Dot fill or Target lock-on accent
	if isTarget {
		// Target gets a filled core dot + 4 lock-on tick marks
		if brushDot != 0 {
			procSelectObject.Call(hdc, brushDot)
			dotR := headR / 2
			if dotR < 1 {
				dotR = 1
			}
			procEllipse.Call(hdc, uintptr(cx-dotR), uintptr(cy-dotR), uintptr(cx+dotR+1), uintptr(cy+dotR+1))
		}
		// Lock-on crosshair ticks outside the head circle
		tickLen := int32(3)
		drawTicks := func(ox, oy int32) {
			procMoveToEx.Call(hdc, uintptr(cx-headR-tickLen+ox), uintptr(cy+oy), 0)
			procLineTo.Call(hdc, uintptr(cx-headR+ox), uintptr(cy+oy))

			procMoveToEx.Call(hdc, uintptr(cx+headR+ox), uintptr(cy+oy), 0)
			procLineTo.Call(hdc, uintptr(cx+headR+tickLen+ox), uintptr(cy+oy))

			procMoveToEx.Call(hdc, uintptr(cx+ox), uintptr(cy-headR-tickLen+oy), 0)
			procLineTo.Call(hdc, uintptr(cx+ox), uintptr(cy-headR+oy))

			procMoveToEx.Call(hdc, uintptr(cx+ox), uintptr(cy+headR+oy), 0)
			procLineTo.Call(hdc, uintptr(cx+ox), uintptr(cy+headR+tickLen+oy))
		}
		procSelectObject.Call(hdc, penShadow)
		drawTicks(1, 1)
		procSelectObject.Call(hdc, penMain)
		drawTicks(0, 0)
	} else if headR >= 3 && brushDot != 0 {
		// Clean 1px center dot
		procSelectObject.Call(hdc, brushDot)
		procEllipse.Call(hdc, uintptr(cx-1), uintptr(cy-1), uintptr(cx+2), uintptr(cy+2))
	}
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

	HFont      uintptr
	HFontBold  uintptr
	HFontSmall uintptr
	curFontPx  int32
	curBoldPx  int32
	curSmallPx int32
}

func InitGDICache() *GDICache {
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
	mainPx := math3d.ScaleI32(13, scale)
	boldPx := math3d.ScaleI32(15, scale)
	smallPx := math3d.ScaleI32(11, scale)
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

type Backbuffer struct {
	hwnd      uintptr
	hdcScreen uintptr
	memDC     uintptr
	memBmp    uintptr
	backW     int32
	backH     int32
}

func NewBackbuffer(hwnd uintptr) *Backbuffer {
	hdcScreen, _, _ := procGetDC.Call(hwnd)
	memDC, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	return &Backbuffer{
		hwnd:      hwnd,
		hdcScreen: hdcScreen,
		memDC:     memDC,
	}
}

func (b *Backbuffer) Ensure(w, h int32) bool {
	if w <= 0 || h <= 0 {
		return false
	}
	if b.memBmp == 0 || b.backW != w || b.backH != h {
		newBmp, _, _ := procCreateCompatibleBitmap.Call(b.hdcScreen, uintptr(w), uintptr(h))
		if newBmp == 0 {
			return false
		}
		if b.memBmp != 0 {
			procDeleteObject.Call(b.memBmp)
		}
		b.memBmp = newBmp
		b.backW = w
		b.backH = h
	}
	return true
}

func (b *Backbuffer) BeginFrame(gdi *GDICache, w, h int32) (uintptr, [4]uintptr) {
	var old [4]uintptr
	old[0], _, _ = procSelectObject.Call(b.memDC, b.memBmp)
	old[1], _, _ = procSelectObject.Call(b.memDC, gdi.HFont)
	old[2], _, _ = procSelectObject.Call(b.memDC, gdi.HudBorderPen)
	old[3], _, _ = procSelectObject.Call(b.memDC, gdi.NullBrush)

	clearRect := RECT{0, 0, w, h}
	procFillRect.Call(b.memDC, uintptr(unsafe.Pointer(&clearRect)), gdi.BlackBrush)
	procSetBkMode.Call(b.memDC, TRANSPARENT)
	return b.memDC, old
}

func (b *Backbuffer) PresentAndRestore(w, h int32, old [4]uintptr) {
	procBitBlt.Call(b.hdcScreen, 0, 0, uintptr(w), uintptr(h), b.memDC, 0, 0, SRCCOPY)

	procSelectObject.Call(b.memDC, old[3])
	procSelectObject.Call(b.memDC, old[2])
	procSelectObject.Call(b.memDC, old[1])
	procSelectObject.Call(b.memDC, old[0])
}

func (b *Backbuffer) Cleanup() {
	if b.memBmp != 0 {
		procDeleteObject.Call(b.memBmp)
		b.memBmp = 0
	}
	if b.memDC != 0 {
		procDeleteDC.Call(b.memDC)
		b.memDC = 0
	}
	if b.hdcScreen != 0 {
		procReleaseDC.Call(b.hwnd, b.hdcScreen)
		b.hdcScreen = 0
	}
}
