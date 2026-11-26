package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"demoanalyzer/internal/aim"
	"demoanalyzer/internal/game"
	"demoanalyzer/internal/math3d"
	"demoanalyzer/internal/ui"
)

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

	ui.EnableDPIAwareness()

	targetIdFlag := flag.Int("target", -1, "Initial target player ID (CPlayer+0xB8)")
	flag.Parse()

	fmt.Println("====================================================")
	fmt.Println("      BF2142 v1.51 External Demo Analyzer           ")
	fmt.Println("====================================================")

	overlay, err := ui.CreateOverlayWindow()
	if err != nil {
		fmt.Printf("[-] %v\n", err)
		return
	}
	defer overlay.Destroy()

	gdi := ui.InitGDICache()
	defer gdi.Cleanup()

	backbuffer := ui.NewBackbuffer(overlay.Hwnd)
	defer backbuffer.Cleanup()

	var borderless ui.BorderlessState
	defer borderless.Restore()

	aimLogFile, errLog := os.OpenFile("aim_log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if errLog != nil {
		fmt.Printf("[!] Warning: failed to open aim_log.txt: %v\n", errLog)
	} else {
		defer aimLogFile.Close()
		fmt.Fprintf(aimLogFile, "\n[%s] === BF2142 Demo Analyzer Started ===\n", time.Now().Format("2006-01-02 15:04:05"))
	}
	analyzer := aim.NewAimAnalyzer(aimLogFile)

	// Runtime toggles
	showHUD := true
	showHelp := false
	scoreboardOpen := false

	speedPresets := []float32{0.25, 0.5, 1.0, 2.0, 3.0, 5.0, 8.0, 12.0}
	speedIdx := 2 // default 1.0 (100%)

	laserMode := 0 // 0: Target Beam, 1: Target Dot, 2: Off
	laserLength := float32(40.0)
	fovScale := float32(1.0)

	selectedTargetID := int32(*targetIdFlag)
	hasSelectedTarget := *targetIdFlag >= 0
	lastDemoFollowID := int32(-1)
	hasLastDemoFollow := false

	scoreboardCursorIdx := 0
	var lastRowHits []ui.ScoreboardRowHit
	var lastFlatOrder []int32
	borderlessMsg := ""
	borderlessMsgUntil := time.Time{}

	var att *game.Attachment
	defer func() {
		if att != nil {
			att.Close()
		}
	}()

	keys := ui.NewKeyTracker()

	hideOverlay := func() {
		scoreboardOpen = false
		overlay.Hide()
		keys.Clear()
	}

	closeScoreboard := func(refocusGame bool) {
		scoreboardOpen = false
		overlay.SetClickThrough(true)
		ui.ClearPendingClicks()
		if refocusGame && att != nil {
			overlay.RefocusGame(att.CachedHwnd)
		}
	}

	fmt.Println("[*] Waiting for BF2142.exe...")
	fmt.Println("[*] Hotkeys (active only when BF2142 is focused):")
	fmt.Println("    [F1]          - Toggle Controls Help Card")
	fmt.Println("    [Insert]      - Toggle HUD & Alignment frame")
	fmt.Println("    [Tab]         - Toggle interactive Scoreboard (Click / Up+Down+Enter)")
	fmt.Println("    [PageUp/Down] - Next/Prev Alive Target (switches BattleRecorder camera)")
	fmt.Println("    [F7]          - Laser mode (Target Beam -> Target Dot -> Off)")
	fmt.Println("    [F8 / F9]     - Laser length -/+ 10m (20..300m)")
	fmt.Println("    [F11]         - Reset Aim Analyzer stats (aim_log.txt)")
	fmt.Println("    [F12]         - Toggle Borderless Fullscreen (for windowed BF2142)")
	fmt.Println("    [ = / - ]     - Fine-tune FOV scale multiplier")
	fmt.Println("    [End]         - Cleanly restore & exit")
	fmt.Println()

	badMgrFrames := 0

	for overlay.PumpMessages() {
		// Attach or verify BF2142.exe alive
		if att == nil || !att.Reader.IsProcessAlive() {
			if att != nil {
				fmt.Printf("[!] BF2142.exe (PID %d) closed. Waiting for relaunch...\n", att.PID)
				borderless.Restore()
				att.Close()
				att = nil
			}
			hideOverlay()
			nextAtt, err := game.TryAttach()
			if err != nil {
				time.Sleep(400 * time.Millisecond)
				continue
			}
			att = nextAtt
			lastDemoFollowID = -1
			hasLastDemoFollow = false
			fmt.Printf("[+] Attached to BF2142.exe (PID %d) | Base=0x%08X RendDX9=0x%08X CanWrite=%v\n",
				att.PID, att.BF2142Base, att.RendBase, att.CanWrite)
		}

		// Find/validate main game window
		if !ui.IsWindowValid(att.CachedHwnd, att.PID) {
			att.CachedHwnd = ui.FindGameWindow(att.PID)
		}
		if att.CachedHwnd == 0 {
			hideOverlay()
			time.Sleep(150 * time.Millisecond)
			continue
		}

		// Focus / Alt-Tab / Minimize guard
		if !ui.IsGameFocused(att.CachedHwnd, overlay.Hwnd, scoreboardOpen) {
			hideOverlay()
			time.Sleep(80 * time.Millisecond)
			continue
		}

		// Match client rect & restore overlay visibility reliably
		targetX, targetY, targetW, targetH := ui.GetClientBounds(att.CachedHwnd)
		if targetW <= 100 || targetH <= 100 {
			hideOverlay()
			time.Sleep(80 * time.Millisecond)
			continue
		}

		overlay.SyncPos(targetX, targetY, targetW, targetH)
		if !backbuffer.Ensure(overlay.CurW, overlay.CurH) {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		uiScale := math3d.ClampF32(float32(overlay.CurH)/720.0, 0.75, 2.5)
		gdi.UpdateScale(uiScale)

		// Process hotkeys ONLY while focused
		if keys.IsPressed(ui.VK_END) {
			break
		}
		if keys.IsPressed(ui.VK_F1) {
			showHelp = !showHelp
		}
		if keys.IsPressed(ui.VK_TAB) {
			if scoreboardOpen {
				closeScoreboard(true)
			} else {
				scoreboardOpen = true
				overlay.SetClickThrough(false)
			}
		}
		if scoreboardOpen && keys.IsPressed(ui.VK_ESCAPE) {
			closeScoreboard(true)
		}
		if keys.IsPressed(ui.VK_INSERT) {
			showHUD = !showHUD
		}
		if keys.IsPressed(ui.VK_F7) {
			laserMode = (laserMode + 1) % 3
		}
		if keys.IsPressed(ui.VK_F8) {
			laserLength -= 10.0
			if laserLength < 20.0 {
				laserLength = 20.0
			}
		}
		if keys.IsPressed(ui.VK_F9) {
			laserLength += 10.0
			if laserLength > 300.0 {
				laserLength = 300.0
			}
		}
		if keys.IsPressed(ui.VK_F12) {
			borderlessMsg = borderless.Toggle(att.CachedHwnd)
			borderlessMsgUntil = time.Now().Add(3 * time.Second)
		}
		if keys.IsPressed(ui.VK_OEM_PLUS) || keys.IsPressed(ui.VK_ADD) {
			fovScale = math3d.ClampF32(fovScale+0.02, 0.4, 2.5)
		}
		if keys.IsPressed(ui.VK_OEM_MINUS) || keys.IsPressed(ui.VK_SUBTRACT) {
			fovScale = math3d.ClampF32(fovScale-0.02, 0.4, 2.5)
		}

		// Resolve pointers & read Camera / Players / Native Demo State
		_, playerMgr, renderer := att.ResolveManagers()
		_, cam := att.ReadCamera(renderer)
		if cam.VpWidth <= 100 || cam.VpHeight <= 100 {
			cam.VpWidth = float32(overlay.CurW)
			cam.VpHeight = float32(overlay.CurH)
		}
		demoState := att.ReadDemoState()

		plist := att.ReadPlayers(playerMgr, cam.Pos, true)
		if plist.ValidList {
			badMgrFrames = 0
		} else {
			badMgrFrames++
			if badMgrFrames > 60 {
				att.NameCache = make(map[uint64]string)
				badMgrFrames = 0
			}
		}
		allPlayers := plist.AllPlayers
		alivePlayers := plist.AlivePlayers

		// Handle Scoreboard input (Clicks & Up/Down/Enter)
		targetChanged := false
		userSwitchedCam := false
		if scoreboardOpen {
			pt, dbl := ui.PopPendingClick()
			if pt != nil {
				for _, hit := range lastRowHits {
					if pt.X >= hit.Rect.Left && pt.X <= hit.Rect.Right && pt.Y >= hit.Rect.Top && pt.Y <= hit.Rect.Bottom {
						if !hasSelectedTarget || selectedTargetID != hit.PlayerID {
							selectedTargetID = hit.PlayerID
							hasSelectedTarget = true
							targetChanged = true
						}
						userSwitchedCam = true
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
				if keys.IsPressed(ui.VK_DOWN) {
					scoreboardCursorIdx = (scoreboardCursorIdx + 1) % len(lastFlatOrder)
				}
				if keys.IsPressed(ui.VK_UP) {
					scoreboardCursorIdx = (scoreboardCursorIdx - 1 + len(lastFlatOrder)) % len(lastFlatOrder)
				}
				if keys.IsPressed(ui.VK_RETURN) {
					pickID := lastFlatOrder[scoreboardCursorIdx]
					if !hasSelectedTarget || selectedTargetID != pickID {
						selectedTargetID = pickID
						hasSelectedTarget = true
						targetChanged = true
					}
					userSwitchedCam = true
					closeScoreboard(true)
				}
			}
		} else {
			ui.ClearPendingClicks()
		}

		// PageUp / PageDown cycles alive players and switches native BattleRecorder camera
		if len(alivePlayers) > 0 {
			aliveIdx := -1
			for i, p := range alivePlayers {
				if p.ID == selectedTargetID {
					aliveIdx = i
					break
				}
			}
			if keys.IsPressed(ui.VK_NEXT) {
				if aliveIdx < 0 {
					aliveIdx = 0
				} else {
					aliveIdx = (aliveIdx + 1) % len(alivePlayers)
				}
				if selectedTargetID != alivePlayers[aliveIdx].ID {
					selectedTargetID = alivePlayers[aliveIdx].ID
					hasSelectedTarget = true
					targetChanged = true
				}
				userSwitchedCam = true
			}
			if keys.IsPressed(ui.VK_PRIOR) {
				if aliveIdx < 0 {
					aliveIdx = len(alivePlayers) - 1
				} else {
					aliveIdx = (aliveIdx - 1 + len(alivePlayers)) % len(alivePlayers)
				}
				if selectedTargetID != alivePlayers[aliveIdx].ID {
					selectedTargetID = alivePlayers[aliveIdx].ID
					hasSelectedTarget = true
					targetChanged = true
				}
				userSwitchedCam = true
			}
		}

		// One-shot BattleRecorder camera switch on Tab/PgUp/PgDn, OR sync Target when user uses Space/LMB in BF2142
		if userSwitchedCam && hasSelectedTarget {
			if err := att.SetDemoFollowPlayer(selectedTargetID); err == nil {
				lastDemoFollowID = selectedTargetID
				hasLastDemoFollow = true
			}
		} else if demoState.DemoActive && demoState.CameraMode != 0 && demoState.FollowedPlayerID >= 0 {
			if !hasLastDemoFollow || demoState.FollowedPlayerID != lastDemoFollowID {
				if !hasSelectedTarget || selectedTargetID != demoState.FollowedPlayerID {
					selectedTargetID = demoState.FollowedPlayerID
					hasSelectedTarget = true
					targetChanged = true
				}
				lastDemoFollowID = demoState.FollowedPlayerID
				hasLastDemoFollow = true
			}
		}

		// Fallback initial target bootstrap if not already set from BattleRecorder
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

		var targetPlayer *game.PlayerInfo
		for i := range allPlayers {
			if allPlayers[i].ID == selectedTargetID {
				targetPlayer = &allPlayers[i]
				break
			}
		}

		if keys.IsPressed(ui.VK_F11) || targetChanged {
			tName := fmt.Sprintf("ID %d", selectedTargetID)
			if targetPlayer != nil {
				tName = targetPlayer.Name
			}
			analyzer.Reset(tName)
		}

		closestEnemy, _, minAngle, closestDist := aim.FindClosestEnemy(targetPlayer, alivePlayers)
		analyzer.ProcessFrame(targetPlayer, closestEnemy, minAngle, closestDist)

		// Render Frame
		hdc, old := backbuffer.BeginFrame(gdi, overlay.CurW, overlay.CurH)

		if showHUD {
			ui.RenderAlignmentFrame(hdc, gdi, overlay.CurW, overlay.CurH)
		}

		ui.RenderLasers(hdc, gdi, uiScale, &cam, alivePlayers, hasSelectedTarget, selectedTargetID,
			laserMode, laserLength, overlay.CurW, overlay.CurH, fovScale)

		if showHUD {
			ui.RenderHUD(hdc, gdi, uiScale, overlay.CurW,
				&cam, demoState, speedPresets[speedIdx],
				targetPlayer, closestEnemy, minAngle, closestDist, analyzer,
				laserMode, laserLength,
				borderless.Active, borderlessMsg, borderlessMsgUntil)
		}

		if showHelp {
			ui.RenderHelpPanel(hdc, gdi, overlay.CurW, overlay.CurH, uiScale)
		}

		if scoreboardOpen {
			curPt := ui.GetCursorClientPos(overlay.Hwnd)
			lastRowHits, lastFlatOrder = ui.RenderScoreboard(
				hdc, gdi, uiScale,
				overlay.CurW, overlay.CurH,
				allPlayers, selectedTargetID, hasSelectedTarget,
				scoreboardCursorIdx, &curPt,
			)
		}

		backbuffer.PresentAndRestore(overlay.CurW, overlay.CurH, old)
		time.Sleep(16 * time.Millisecond)
	}

	fmt.Println("[*] Overlay exited cleanly.")
}
