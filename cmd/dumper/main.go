package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"demoanalyzer/internal/game"
	"demoanalyzer/internal/math3d"
)

func scanPlayer(m *game.MemoryReader, playerAddr uint32) {
	fmt.Printf("\n========== RAW DUMP of First CPlayer (0x%08X) ==========\n", playerAddr)
	var buf [0x200]byte
	if err := m.ReadRaw(playerAddr, buf[:]); err != nil {
		fmt.Printf("Failed to read player memory: %v\n", err)
		return
	}

	for row := 0; row < 0x200; row += 16 {
		fmt.Printf("+0x%03X: ", row)
		for col := 0; col < 16; col++ {
			fmt.Printf("%02X ", buf[row+col])
		}
		fmt.Print(" | ")
		for col := 0; col < 16; col++ {
			c := buf[row+col]
			if c >= 0x20 && c <= 0x7E {
				fmt.Printf("%c", c)
			} else {
				fmt.Print(".")
			}
		}
		fmt.Println()
	}

	fmt.Println("\n--- Embedded Printable Strings ---")
	for i := 0; i < len(buf)-3; i++ {
		if buf[i] >= 0x20 && buf[i] <= 0x7E {
			start := i
			for i < len(buf) && buf[i] >= 0x20 && buf[i] <= 0x7E {
				i++
			}
			if i-start >= 3 {
				fmt.Printf("  Offset +0x%03X: %q\n", start, string(buf[start:i]))
			}
		}
	}

	fmt.Println("\n--- Valid Pointers & Matrix Candidates ---")
	for i := 0; i <= len(buf)-4; i += 4 {
		ptr := binary.LittleEndian.Uint32(buf[i : i+4])
		if ptr >= 0x00400000 && ptr < game.MaxUserAddr {
			v, err := m.ReadVector3(ptr)
			if err == nil && math3d.IsValidVec3(v) && (v.X != 0 || v.Y != 0 || v.Z != 0) {
				fmt.Printf("  Offset +0x%03X -> Ptr 0x%08X: Pos=(%.2f, %.2f, %.2f)\n", i, ptr, v.X, v.Y, v.Z)
			}
			matV, errM := m.ReadVector3(ptr + game.OffViewMatEyePos)
			if errM == nil && math3d.IsValidVec3(matV) && (matV.X != 0 || matV.Y != 0 || matV.Z != 0) {
				fmt.Printf("  Offset +0x%03X -> MatrixPtr 0x%08X (+0xF8 EyePos=(%.2f, %.2f, %.2f))\n", i, ptr, matV.X, matV.Y, matV.Z)
			}
		}
	}
	fmt.Println("========================================================")
	fmt.Println()
}

// dumpCSoldierMatrixProbe reads the CSoldier (or PCO) object and scans offsets +0x88..+0xF0
// as float32 triplets, printing those closest to the reference ViewMat EyePos.
// Use this to find the correct OffObjectPos (translation row of CObject world matrix).
func dumpCSoldierMatrixProbe(m *game.MemoryReader, label string, playerAddr uint32, refEyePos math3d.Vector3) {
	// Dereference: CPlayer+0x80 -> soldierWrapper -> +0x04 -> CSoldier*
	soldierWrapper, errW := m.ReadUint32(playerAddr + game.OffPlayerSoldierPtr)
	if errW != nil || !game.IsValidUserAddr(soldierWrapper) {
		fmt.Printf("  [PROBE %s] soldierWrapper invalid (0x%08X)\n", label, soldierWrapper)
		return
	}
	csoldier, errO := m.ReadUint32(soldierWrapper + game.OffObjPtrTarget)
	if errO != nil || !game.IsValidUserAddr(csoldier) {
		fmt.Printf("  [PROBE %s] CSoldier* invalid (0x%08X)\n", label, csoldier)
		return
	}

	activeObj := csoldier
	objLabel := "CSoldier"

	fmt.Printf("  [PROBE %s] %s=0x%08X | RefEyePos=(%.2f,%.2f,%.2f)\n",
		label, objLabel, activeObj, refEyePos.X, refEyePos.Y, refEyePos.Z)

	type candidate struct {
		off  uint32
		pos  math3d.Vector3
		dist float32
	}
	var best []candidate

	var buf [0x80]byte // scan +0x88..+0x108 (128 bytes)
	if err := m.ReadRaw(activeObj+0x88, buf[:]); err != nil {
		fmt.Printf("  [PROBE %s] ReadRaw failed: %v\n", label, err)
		return
	}

	for i := 0; i+12 <= len(buf); i += 4 {
		x := math.Float32frombits(binary.LittleEndian.Uint32(buf[i:]))
		y := math.Float32frombits(binary.LittleEndian.Uint32(buf[i+4:]))
		z := math.Float32frombits(binary.LittleEndian.Uint32(buf[i+8:]))
		if !math3d.IsFiniteFloat(x) || !math3d.IsFiniteFloat(y) || !math3d.IsFiniteFloat(z) {
			continue
		}
		if math.Abs(float64(x)) > 100000 || math.Abs(float64(y)) > 100000 || math.Abs(float64(z)) > 100000 {
			continue
		}
		p := math3d.Vector3{X: x, Y: y, Z: z}
		d := math3d.Vec3Dist(p, refEyePos)
		absOff := uint32(0x88) + uint32(i)
		best = append(best, candidate{off: absOff, pos: p, dist: d})
	}

	// Print all candidates sorted by distance (nearest first) — top 8
	for a := 0; a < len(best)-1; a++ {
		for b := a + 1; b < len(best); b++ {
			if best[b].dist < best[a].dist {
				best[a], best[b] = best[b], best[a]
			}
		}
	}
	limit := len(best)
	if limit > 10 {
		limit = 10
	}
	for _, c := range best[:limit] {
		marker := ""
		if c.dist < 2.0 {
			marker = " <<=== MATCH"
		} else if c.dist < 5.0 {
			marker = " <-- near"
		}
		fmt.Printf("    +0x%03X: (%.3f, %.3f, %.3f)  dist=%.2fm%s\n", c.off, c.pos.X, c.pos.Y, c.pos.Z, c.dist, marker)
	}
}

func dumpPlayerRawRanges(m *game.MemoryReader, label string, playerAddr uint32) {
	fmt.Printf("  [RAW %s @ 0x%08X]\n", label, playerAddr)
	var bufA [0x24]byte // +0x78 .. +0x9B (+0x78..+0x98 dwords)
	if err := m.ReadRaw(playerAddr+0x78, bufA[:]); err == nil {
		fmt.Printf("    +0x078..+0x098:")
		for off := 0; off <= 0x20; off += 4 {
			v := binary.LittleEndian.Uint32(bufA[off : off+4])
			fmt.Printf(" +%02X=%08X", 0x78+off, v)
		}
		fmt.Println()
	}
	var bufB [0x24]byte // +0x100 .. +0x123 (+0x100..+0x120 dwords)
	if err := m.ReadRaw(playerAddr+0x100, bufB[:]); err == nil {
		fmt.Printf("    +0x100..+0x120:")
		for off := 0; off <= 0x20; off += 4 {
			v := binary.LittleEndian.Uint32(bufB[off : off+4])
			fmt.Printf(" +%03X=%08X", 0x100+off, v)
		}
		fmt.Println()
	}
}

func waitForAttachment(sigChan <-chan os.Signal) (*game.Attachment, bool) {
	fmt.Print("[*] Waiting for BF2142.exe process... ")
	for {
		select {
		case <-sigChan:
			return nil, false
		default:
		}
		att, err := game.TryAttach()
		if err == nil {
			fmt.Printf("FOUND (PID: %d)\n", att.PID)
			fmt.Printf("[*] BF2142.exe base:  0x%08X\n", att.BF2142Base)
			fmt.Printf("[*] RendDX9.dll base: 0x%08X\n", att.RendBase)
			return att, true
		}
		time.Sleep(1 * time.Second)
	}
}

func main() {
	scanPlayerFlag := flag.Bool("scan-player", false, "Dump first CPlayer 0x200 bytes and search for strings/matrices")
	flag.Parse()

	fmt.Println("====================================================")
	fmt.Println("       BF2142 v1.51 External Memory Dumper          ")
	fmt.Println("       (Battlefield 2142 Demo Analyzer)             ")
	fmt.Println("====================================================")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	att, ok := waitForAttachment(sigChan)
	if !ok {
		return
	}
	defer func() {
		if att != nil {
			att.Close()
		}
	}()

	fmt.Printf("[*] Static pointers configuration:\n")
	fmt.Printf("    Game ptr:          0x%08X (BF2142.exe + 0x61B5D8)\n", att.GamePtrAddr)
	fmt.Printf("    ClassManager ptr:  0x%08X (BF2142.exe + 0x67D068)\n", att.ClassMgrPtrAddr)
	fmt.Printf("    PlayerManager ptr: 0x%08X (BF2142.exe + 0x685228)\n", att.PlayerMgrPtrAddr)
	fmt.Printf("    Renderer ptr:      0x%08X (RendDX9.dll + 0x1F8E58)\n\n", att.RendererPtrAddr)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	didScanPlayer := false

	for {
		select {
		case <-sigChan:
			fmt.Println("\n[*] Exiting dumper...")
			return
		case <-ticker.C:
			if !att.Reader.IsProcessAlive() {
				fmt.Printf("\n[!] BF2142.exe (PID %d) closed. Re-attaching...\n", att.PID)
				att.Close()
				att, ok = waitForAttachment(sigChan)
				if !ok {
					return
				}
				didScanPlayer = false
				continue
			}

			classMgr, playerMgr, renderer := att.ResolveManagers()

			fmt.Println("--------------------------------------------------------------------------------")
			fmt.Printf("[%s] Status (PID %d):\n", time.Now().Format("15:04:05"), att.PID)
			fmt.Printf("  ClassManager:  0x%08X\n", classMgr)
			fmt.Printf("  PlayerManager: 0x%08X\n", playerMgr)
			fmt.Printf("  Renderer:      0x%08X\n", renderer)

			demoSt := att.ReadDemoState()
			fmt.Printf("  Game[+0x61B5D8]: 0x%08X | DemoRec[+0x140]: 0x%08X | ReplayCam[+0xF0]: 0x%08X\n",
				demoSt.GamePtr, demoSt.DemoRecorderPtr, demoSt.ReplayCamPtr)
			if demoSt.DemoActive {
				modeStr := "FreeCam"
				if demoSt.CameraMode != 0 {
					modeStr = "FollowPlayer"
				}
				fmt.Printf("    DemoActive: YES | CameraMode(+0x10): %d (%s) | FollowedID(+0x08): %d\n",
					demoSt.CameraMode, modeStr, demoSt.FollowedPlayerID)
			} else {
				fmt.Printf("    DemoActive: NO\n")
			}

			if sp, ok := att.ReadPlaybackSpeed(); ok {
				fmt.Printf("    Playback Speed: %.2fx (%.0f%%) via TimeManager(+0x1C)\n", sp, sp*100.0)
			}

			activeCam, cam := att.ReadCamera(renderer)
			if game.IsValidUserAddr(renderer) {
				fmt.Printf("  ActiveCamera:  0x%08X\n", activeCam)
			}
			if game.IsValidUserAddr(activeCam) {
				nearA, _ := att.Reader.ReadFloat32(activeCam + game.OffCamNearPlaneA)
				nearB, _ := att.Reader.ReadFloat32(activeCam + game.OffCamNearPlaneB)
				nearC, _ := att.Reader.ReadFloat32(activeCam + game.OffCamNearPlaneC)
				nearD, _ := att.Reader.ReadFloat32(activeCam + game.OffCamNearPlaneD)
				fmt.Printf("    NearPlane (A,B,C,D): (%.3f, %.3f, %.3f, %.3f)\n", nearA, nearB, nearC, nearD)

				if cam.VpWidth > 0 && cam.VpHeight > 0 {
					fmt.Printf("    Viewport (Cam+0x10/+0x14): %.0f x %.0f\n", cam.VpWidth, cam.VpHeight)
				}

				if worldMat, errW := att.Reader.ReadMatrix4x4(activeCam + game.OffCamWorldMat); errW == nil {
					fmt.Printf("    Camera World Matrix (+0x40):\n")
					fmt.Printf("      Right:   [%7.3f, %7.3f, %7.3f, %7.3f]\n", worldMat[0][0], worldMat[0][1], worldMat[0][2], worldMat[0][3])
					fmt.Printf("      Up:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", worldMat[1][0], worldMat[1][1], worldMat[1][2], worldMat[1][3])
					fmt.Printf("      Forward: [%7.3f, %7.3f, %7.3f, %7.3f]\n", worldMat[2][0], worldMat[2][1], worldMat[2][2], worldMat[2][3])
					fmt.Printf("      Pos:     [%7.2f, %7.2f, %7.2f, %7.2f]\n", worldMat[3][0], worldMat[3][1], worldMat[3][2], worldMat[3][3])
				}

				if math3d.IsValidMatrix(cam.ViewMatrix) {
					viewMat := cam.ViewMatrix
					fmt.Printf("    Camera View Matrix (+0x80):\n")
					fmt.Printf("      R0:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", viewMat[0][0], viewMat[0][1], viewMat[0][2], viewMat[0][3])
					fmt.Printf("      R1:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", viewMat[1][0], viewMat[1][1], viewMat[1][2], viewMat[1][3])
					fmt.Printf("      R2:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", viewMat[2][0], viewMat[2][1], viewMat[2][2], viewMat[2][3])
					fmt.Printf("      R3:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", viewMat[3][0], viewMat[3][1], viewMat[3][2], viewMat[3][3])
				}

				if projMat, errP := att.Reader.ReadMatrix4x4(activeCam + game.OffCamProjMat); errP == nil {
					projValid := math3d.IsValidProjMatrix(projMat)
					fmt.Printf("    Camera Proj Matrix (+0x140) [valid=%v]:\n", projValid)
					fmt.Printf("      R0:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", projMat[0][0], projMat[0][1], projMat[0][2], projMat[0][3])
					fmt.Printf("      R1:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", projMat[1][0], projMat[1][1], projMat[1][2], projMat[1][3])
					fmt.Printf("      R2:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", projMat[2][0], projMat[2][1], projMat[2][2], projMat[2][3])
					fmt.Printf("      R3:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", projMat[3][0], projMat[3][1], projMat[3][2], projMat[3][3])
					if projValid && projMat[1][1] > 0.1 && projMat[1][1] < 20.0 {
						derivedFovY := 2.0 * math.Atan(1.0/float64(projMat[1][1])) * 180.0 / math.Pi
						derivedFovX := 2.0 * math.Atan(1.0/float64(projMat[0][0])) * 180.0 / math.Pi
						pRatio := projMat[0][0] / projMat[1][1]
						vpRatio := float32(0)
						if cam.VpWidth > 0 {
							vpRatio = cam.VpHeight / cam.VpWidth
						}
						fmt.Printf("      -> Derived FOV: vFOV=%.2f deg (P11=%.4f), hFOV=%.2f deg (P00=%.4f) | P00/P11=%.4f vs VpH/VpW=%.4f | fov24=%.4f (%.2f deg) asp34=%.4f\n",
							derivedFovY, projMat[1][1], derivedFovX, projMat[0][0], pRatio, vpRatio, cam.FovRad24, cam.FovRad24*180.0/math.Pi, cam.Aspect34)
					}
				}

				fmt.Printf("    Camera Header Floats (+0x08..+0x3C):\n")
				for _, off := range []uint32{0x08, 0x0C, 0x10, 0x14, 0x1C, 0x20, 0x24, 0x2C, 0x30, 0x34, 0x38, 0x3C} {
					if val, err := att.Reader.ReadFloat32(activeCam + off); err == nil && math3d.IsFiniteFloat(val) && val != 0 {
						fmt.Printf("      +0x%03X: %9.4f\n", off, val)
					}
				}
			}

			plist := att.ReadPlayers(playerMgr, cam.Pos, false)
			if game.IsValidUserAddr(plist.ListHead) {
				fmt.Printf("\n  Players Count: %d (std::list at 0x%08X)\n", plist.PlayerCount, plist.ListHead)

				if plist.ValidList {
					fmt.Printf("  %-4s | %-16s | %-2s | %-5s | %-2s | %-3s | %-12s | %-23s | %-23s | %-10s\n",
						"ID", "Nickname", "Tm", "Alive", "D5", "Veh", "Score (K/D)", fmt.Sprintf("CSoldier Pos (+0x%02X)", game.OffObjectPos), "ViewMat EyePos (+0xF8)", "CSoldier")
					fmt.Println("  -----+------------------+----+-------+----+-----+--------------+-------------------------+-------------------------+-----------")

					var firstPlayerAddr, firstLiveAddr, firstDeadAddr uint32
					var firstLivePlayer *game.PlayerInfo
					for i := range plist.AllPlayers {
						p := &plist.AllPlayers[i]
						if firstPlayerAddr == 0 {
							firstPlayerAddr = p.Addr
						}
						if p.IsAlive && firstLiveAddr == 0 {
							firstLiveAddr = p.Addr
							if p.HasRawEye {
								firstLivePlayer = p
							}
						} else if !p.IsAlive && p.IsDeadDefault && firstDeadAddr == 0 {
							firstDeadAddr = p.Addr
						}

						scoreStr := "--"
						if p.HasScore {
							scoreStr = fmt.Sprintf("%3d (%2d/%2d)", p.Score, p.Kills, p.Deaths)
						}
						worldPosStr := "none"
						if p.HasWorldPos {
							worldPosStr = fmt.Sprintf("(%6.1f,%6.1f,%6.1f)", p.WorldPos.X, p.WorldPos.Y, p.WorldPos.Z)
						}
						eyePosStr := "stale/none"
						if p.HasRawEye {
							eyePosStr = fmt.Sprintf("(%6.1f,%6.1f,%6.1f)", p.RawEyePos.X, p.RawEyePos.Y, p.RawEyePos.Z)
						}

						aliveStr := "DEAD"
						if p.IsAlive {
							aliveStr = "LIVE"
						}
						vehStr := " "
						if p.InVehicle {
							vehStr = "Y"
						}

						fmt.Printf("  %4d | %-16s | %2d | %-5s | %2d | %-3s | %-12s | %-23s | %-23s | 0x%08X\n",
							p.ID, p.Name, p.Team, aliveStr, p.AliveD5, vehStr, scoreStr, worldPosStr, eyePosStr, p.SoldierPtr)
					}

					if firstLiveAddr != 0 || firstDeadAddr != 0 {
						fmt.Println("\n  --- CPlayer Offset Probe (+0x78..+0x98, +0x100..+0x120) ---")
						if firstLiveAddr != 0 {
							dumpPlayerRawRanges(att.Reader, "LIVE", firstLiveAddr)
						}
						if firstDeadAddr != 0 {
							dumpPlayerRawRanges(att.Reader, "DEAD(0,1,0)", firstDeadAddr)
						}
					}

					if firstLivePlayer != nil {
						fmt.Println("\n  --- CSoldier Matrix Translation Scan (+0x88..+0x108) vs ViewMat EyePos ---")
						dumpCSoldierMatrixProbe(att.Reader, fmt.Sprintf("LIVE [%d] %s", firstLivePlayer.ID, firstLivePlayer.Name), firstLivePlayer.Addr, firstLivePlayer.RawEyePos)
					}

					if *scanPlayerFlag && !didScanPlayer && firstPlayerAddr != 0 {
						scanPlayer(att.Reader, firstPlayerAddr)
						didScanPlayer = true
					}
				} else {
					fmt.Println("  (No players currently active in demo)")
				}
			}
			fmt.Println()
		}
	}
}
