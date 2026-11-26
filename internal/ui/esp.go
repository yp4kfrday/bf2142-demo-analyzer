package ui

import (
	"fmt"
	"time"
	"unsafe"

	"demoanalyzer/internal/aim"
	"demoanalyzer/internal/game"
	"demoanalyzer/internal/math3d"
)

func RenderAlignmentFrame(hdc uintptr, gdi *GDICache, curW, curH int32) {
	procSelectObject.Call(hdc, gdi.PenGreenBorder)
	procSelectObject.Call(hdc, gdi.NullBrush)
	procRectangle.Call(hdc, 0, 0, uintptr(curW), uintptr(curH))
}

// RenderLasers renders 1px laser beam / endpoint strictly for the spectated target.
// laserMode: 0 = Target Beam, 1 = Target Dot, 2 = Off
func RenderLasers(
	hdc uintptr, gdi *GDICache, uiScale float32,
	cam *math3d.CameraData,
	alivePlayers []game.PlayerInfo,
	hasSelectedTarget bool, selectedTargetID int32,
	laserMode int, laserLength float32,
	curW, curH int32, fovScale float32,
) {
	if laserMode >= 2 || !hasSelectedTarget || len(alivePlayers) == 0 {
		return
	}

	var target *game.PlayerInfo
	for i := range alivePlayers {
		if alivePlayers[i].ID == selectedTargetID {
			target = &alivePlayers[i]
			break
		}
	}
	if target == nil || !target.HasPose {
		return
	}

	screenW := float32(curW)
	screenH := float32(curH)

	p1 := target.EyePos
	p2 := math3d.Vector3{
		X: p1.X + target.LookDir.X*laserLength,
		Y: p1.Y + target.LookDir.Y*laserLength,
		Z: p1.Z + target.LookDir.Z*laserLength,
	}

	penLine := gdi.PenTargetLaser
	brushDot := gdi.BrushTargetDot

	// Mode 0: Target Beam (1px line from EyePos + start dot)
	if laserMode == 0 {
		s1, s2, okLine := cam.ProjectLine3D(p1, p2, screenW, screenH, fovScale)
		if okLine {
			procSelectObject.Call(hdc, penLine)
			procMoveToEx.Call(hdc, uintptr(s1.X), uintptr(s1.Y), 0)
			procLineTo.Call(hdc, uintptr(s2.X), uintptr(s2.Y))

			if sEye, okEye := cam.WorldToScreen(p1, screenW, screenH, fovScale); okEye {
				if sEye.X >= 0 && sEye.X < curW && sEye.Y >= 0 && sEye.Y < curH {
					procSelectObject.Call(hdc, brushDot)
					procEllipse.Call(hdc, uintptr(sEye.X-2), uintptr(sEye.Y-2), uintptr(sEye.X+3), uintptr(sEye.Y+3))
				}
			}
		}
	}

	// Mode 0 or 1: draw endpoint marker
	if sEnd, okEnd := cam.WorldToScreen(p2, screenW, screenH, fovScale); okEnd {
		if sEnd.X >= 0 && sEnd.X < curW && sEnd.Y >= 0 && sEnd.Y < curH {
			markerR := math3d.ScaleI32(4, uiScale)
			DrawDiamondMarker(hdc, sEnd.X, sEnd.Y, markerR, penLine, gdi.PenShadow, brushDot)
		}
	}
}

// RenderHelpPanel displays a centered dark card with all active hotkeys in Russian UTF-16.
func RenderHelpPanel(hdc uintptr, gdi *GDICache, curW, curH int32, uiScale float32) {
	cardW := math3d.ScaleI32(480, uiScale)
	cardH := math3d.ScaleI32(310, uiScale)
	if cardW > curW-40 {
		cardW = curW - 40
	}
	if cardH > curH-40 {
		cardH = curH - 40
	}

	x0 := (curW - cardW) / 2
	y0 := (curH - cardH) / 2
	x1 := x0 + cardW
	y1 := y0 + cardH

	rcCard := RECT{x0, y0, x1, y1}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rcCard)), gdi.HudBrush)

	// Border
	procSelectObject.Call(hdc, gdi.HudBorderPen)
	procSelectObject.Call(hdc, gdi.NullBrush)
	procRectangle.Call(hdc, uintptr(x0), uintptr(y0), uintptr(x1), uintptr(y1))

	padX := math3d.ScaleI32(20, uiScale)
	padY := math3d.ScaleI32(16, uiScale)
	rowH := math3d.ScaleI32(21, uiScale)

	// Title
	procSelectObject.Call(hdc, gdi.HFontBold)
	title := "Горячие клавиши"
	DrawTextShadow(hdc, title, x0+padX, y0+padY, RGB(220, 235, 255))

	yCur := y0 + padY + math3d.ScaleI32(30, uiScale)

	// Separator line
	procSelectObject.Call(hdc, gdi.HudBorderPen)
	procMoveToEx.Call(hdc, uintptr(x0+padX), uintptr(yCur), 0)
	procLineTo.Call(hdc, uintptr(x1-padX), uintptr(yCur))
	yCur += math3d.ScaleI32(10, uiScale)

	procSelectObject.Call(hdc, gdi.HFont)

	type keyHelp struct {
		key  string
		desc string
	}
	helps := []keyHelp{
		{"Tab", "Таблица игроков и переключение камеры"},
		{"PgUp / PgDn", "Следующий / предыдущий живой игрок"},
		{"F7", "Лазер цели (Луч -> Точка -> Выкл)"},
		{"F8 / F9", "Длина лазера (-/+ 10м)"},
		{"F11", "Сброс статистики Aim-анализатора"},
		{"F12", "Полный экран без рамки (Borderless)"},
		{"Insert", "Скрыть / показать верхний HUD"},
		{"F1", "Показать / скрыть эту справку"},
		{"End", "Закрыть оверлей"},
	}

	keyColW := math3d.ScaleI32(110, uiScale)

	for _, h := range helps {
		DrawTextShadow(hdc, h.key, x0+padX, yCur, RGB(255, 215, 80))
		DrawTextShadow(hdc, "—  "+h.desc, x0+padX+keyColW, yCur, RGB(205, 220, 235))
		yCur += rowH
	}
}

func RenderHUD(
	hdc uintptr, gdi *GDICache, uiScale float32,
	curW int32,
	cam *math3d.CameraData,
	demoState game.DemoState,
	playbackSpeed float32,
	targetPlayer, closestEnemy *game.PlayerInfo,
	minAngle, closestDist float32,
	analyzer *aim.AimAnalyzer,
	laserMode int, laserLength float32,
	borderlessActive bool,
	borderlessMsg string, borderlessMsgUntil time.Time,
) {
	padX := math3d.ScaleI32(7, uiScale)
	padY := math3d.ScaleI32(5, uiScale)
	rowStep := math3d.ScaleI32(15, uiScale)
	hudX := math3d.ScaleI32(10, uiScale)
	hudY := math3d.ScaleI32(10, uiScale)

	camModeTag := "Free"
	if demoState.DemoActive {
		if demoState.CameraMode != 0 {
			camModeTag = "Follow"
		}
	} else {
		camModeTag = "Off"
	}

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
		targetText = fmt.Sprintf("[%d] %s T%d %s | %d/%d | Cam:%s",
			targetPlayer.ID, targetPlayer.Name, targetPlayer.Team, status,
			targetPlayer.Kills, targetPlayer.Deaths, camModeTag)
	} else {
		targetText = fmt.Sprintf("Target: None | Cam:%s", camModeTag)
	}

	showAimLine := targetPlayer != nil && targetPlayer.IsAlive && targetPlayer.HasPose &&
		closestEnemy != nil && minAngle <= 12.0 && closestDist <= 120.0

	var aimTargetStr, snapStatsStr, lockStr string
	var aimColor uint32
	if showAimLine {
		aimTargetStr = fmt.Sprintf("%s %.1f° (%.0fm)", closestEnemy.Name, minAngle, closestDist)
		if minAngle <= 1.5 {
			aimColor = RGB(255, 70, 70)
		} else if minAngle <= 5.0 {
			aimColor = RGB(255, 230, 0)
		} else {
			aimColor = RGB(195, 210, 225)
		}
		if analyzer.SnapCount > 0 {
			snapStatsStr = fmt.Sprintf(" | Snaps:%d (%.1f°)", analyzer.SnapCount, analyzer.MaxSnapAngle)
		}
		if analyzer.IsLocking && (analyzer.CurrentLockDuration >= 0.1 || minAngle <= 1.5) {
			lockStr = fmt.Sprintf(" [LOCK %.1fs]", analyzer.CurrentLockDuration)
		}
	}

	laserNames := []string{"Target Beam", "Target Dot", "Off"}
	lModeName := "Off"
	if laserMode >= 0 && laserMode < len(laserNames) {
		lModeName = laserNames[laserMode]
	}

	statusRow := fmt.Sprintf("Laser: %s (%.0fm) | [F1] Справка | [Tab][End]", lModeName, laserLength)
	if borderlessActive {
		statusRow = fmt.Sprintf("Laser: %s (%.0fm) | Borderless | [F1] Справка | [Tab][End]", lModeName, laserLength)
	}
	if borderlessMsg != "" && time.Now().Before(borderlessMsgUntil) {
		statusRow += " [" + borderlessMsg + "]"
	}

	procSelectObject.Call(hdc, gdi.HFontSmall)
	maxW := GetTextWidth(hdc, targetText)
	if showAimLine {
		if w := GetTextWidth(hdc, "Aim -> "+aimTargetStr+snapStatsStr+lockStr); w > maxW {
			maxW = w
		}
	}
	if w := GetTextWidth(hdc, statusRow); w > maxW {
		maxW = w
	}

	numRows := int32(2)
	if showAimLine {
		numRows = 3
	}
	hudW := maxW + padX*2
	if hudW < math3d.ScaleI32(240, uiScale) {
		hudW = math3d.ScaleI32(240, uiScale)
	}
	if hudW > curW-hudX*2 {
		hudW = curW - hudX*2
	}
	hudH := padY*2 + rowStep*numRows

	rcHud := RECT{hudX, hudY, hudX + hudW, hudY + hudH}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rcHud)), gdi.HudBrush)
	procSelectObject.Call(hdc, gdi.HudBorderPen)
	procSelectObject.Call(hdc, gdi.NullBrush)
	procRectangle.Call(hdc, uintptr(rcHud.Left), uintptr(rcHud.Top), uintptr(rcHud.Right), uintptr(rcHud.Bottom))

	yCur := hudY + padY
	DrawTextShadow(hdc, targetText, hudX+padX, yCur, targetColor)
	yCur += rowStep

	if showAimLine {
		aimX := hudX + padX
		DrawTextShadow(hdc, "Aim -> ", aimX, yCur, RGB(180, 230, 255))
		aimX += GetTextWidth(hdc, "Aim -> ")
		DrawTextShadow(hdc, aimTargetStr, aimX, yCur, aimColor)
		aimX += GetTextWidth(hdc, aimTargetStr)
		if snapStatsStr != "" {
			DrawTextShadow(hdc, snapStatsStr, aimX, yCur, RGB(210, 225, 240))
			aimX += GetTextWidth(hdc, snapStatsStr)
		}
		if lockStr != "" {
			DrawTextShadow(hdc, lockStr, aimX, yCur, RGB(255, 80, 60))
		}
		yCur += rowStep
	}

	DrawTextShadow(hdc, statusRow, hudX+padX, yCur, RGB(165, 185, 205))
}
