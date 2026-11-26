package ui

import (
	"fmt"
	"sort"
	"unsafe"

	"demoanalyzer/internal/game"
	"demoanalyzer/internal/math3d"
)

type ScoreboardRowHit struct {
	Rect     RECT
	PlayerID int32
	Name     string
}

func RenderScoreboard(
	hdc uintptr, gdi *GDICache, uiScale float32,
	curW, curH int32,
	allPlayers []game.PlayerInfo,
	selectedTargetID int32, hasSelectedTarget bool,
	cursorIdx int, cursorPos *POINT,
) ([]ScoreboardRowHit, []int32) {
	team1 := make([]game.PlayerInfo, 0, len(allPlayers))
	team2 := make([]game.PlayerInfo, 0, len(allPlayers))
	other := make([]game.PlayerInfo, 0)
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
	sortPlayers := func(list []game.PlayerInfo) {
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

	boardW := curW - math3d.ScaleI32(120, uiScale)
	maxBoardW := math3d.ScaleI32(940, uiScale)
	if boardW > maxBoardW {
		boardW = maxBoardW
	}
	minBoardW := math3d.ScaleI32(520, uiScale)
	if boardW < minBoardW {
		boardW = curW - math3d.ScaleI32(30, uiScale)
	}

	headerH := math3d.ScaleI32(56, uiScale)
	colHdrH := math3d.ScaleI32(22, uiScale)
	rowH := math3d.ScaleI32(20, uiScale)
	footerH := math3d.ScaleI32(26, uiScale)

	maxRows := len(team1)
	if len(team2) > maxRows {
		maxRows = len(team2)
	}
	if maxRows < 6 {
		maxRows = 6
	}
	maxVisibleRows := (curH - math3d.ScaleI32(80, uiScale) - headerH - colHdrH - footerH) / rowH
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

	pad := math3d.ScaleI32(12, uiScale)
	procSelectObject.Call(hdc, gdi.HFontBold)
	DrawTextShadow(hdc, fmt.Sprintf("SCOREBOARD (%d players) - ↑ ↓ and enter to select Target | [Tab/Esc] Close", len(allPlayers)),
		bx+pad, by+math3d.ScaleI32(8, uiScale), RGB(255, 220, 70))

	colW := (boardW - pad*3) / 2
	col1X := bx + pad
	col2X := bx + pad*2 + colW
	colTop := by + math3d.ScaleI32(32, uiScale)

	procSelectObject.Call(hdc, gdi.HFont)
	DrawTextShadow(hdc, fmt.Sprintf("TEAM 1 (%d)", len(team1)), col1X+4, colTop, RGB(80, 195, 255))
	DrawTextShadow(hdc, fmt.Sprintf("TEAM 2 (%d)", len(team2)), col2X+4, colTop, RGB(255, 115, 75))

	hdrY := by + headerH
	procSelectObject.Call(hdc, gdi.HFontSmall)
	drawColHeaders := func(cx int32) {
		DrawTextShadow(hdc, "ID", cx+math3d.ScaleI32(4, uiScale), hdrY, RGB(170, 185, 200))
		DrawTextShadow(hdc, "ST", cx+math3d.ScaleI32(34, uiScale), hdrY, RGB(170, 185, 200))
		DrawTextShadow(hdc, "NICKNAME", cx+math3d.ScaleI32(68, uiScale), hdrY, RGB(170, 185, 200))
		DrawTextShadow(hdc, "K/D", cx+colW-math3d.ScaleI32(86, uiScale), hdrY, RGB(170, 185, 200))
		DrawTextShadow(hdc, "SCR", cx+colW-math3d.ScaleI32(38, uiScale), hdrY, RGB(170, 185, 200))
	}
	drawColHeaders(col1X)
	drawColHeaders(col2X)

	hits := make([]ScoreboardRowHit, 0, len(allPlayers))
	drawColumn := func(list []game.PlayerInfo, cx int32, teamColor uint32) {
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
			DrawTextShadow(hdc, fmt.Sprintf("%2d", p.ID), cx+math3d.ScaleI32(4, uiScale), ry+2, RGB(200, 210, 220))
			DrawTextShadow(hdc, stateTag, cx+math3d.ScaleI32(34, uiScale), ry+2, stateColor)

			nick := p.Name
			if isTarget {
				nick = "> " + nick
			}
			if len(nick) > 22 {
				nick = nick[:21] + "…"
			}
			DrawTextShadow(hdc, nick, cx+math3d.ScaleI32(68, uiScale), ry+2, nameColor)
			DrawTextShadow(hdc, fmt.Sprintf("%d/%d", p.Kills, p.Deaths), cx+colW-math3d.ScaleI32(86, uiScale), ry+2, RGB(215, 225, 235))
			DrawTextShadow(hdc, fmt.Sprintf("%d", p.Score), cx+colW-math3d.ScaleI32(38, uiScale), ry+2, RGB(240, 240, 240))

			hits = append(hits, ScoreboardRowHit{
				Rect:     rc,
				PlayerID: p.ID,
				Name:     p.Name,
			})
		}
	}

	drawColumn(team1, col1X, RGB(120, 210, 255))
	drawColumn(team2, col2X, RGB(255, 145, 110))

	footY := by + boardH - footerH + math3d.ScaleI32(4, uiScale)
	procSelectObject.Call(hdc, gdi.HFontSmall)
	DrawTextShadow(hdc, "Tip: Click any player (Live or Dead) to lock Target immediately; Double-Click or Enter confirms & closes.",
		bx+pad, footY, RGB(170, 185, 205))

	return hits, flatOrder
}
