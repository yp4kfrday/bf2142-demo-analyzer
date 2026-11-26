package game

import (
	"fmt"
	"math"

	"demoanalyzer/internal/math3d"
)

func (a *Attachment) ResolveManagers() (classMgr, playerMgr, renderer uint32) {
	classMgr, _ = a.Reader.ReadUint32(a.ClassMgrPtrAddr)
	playerMgr, _ = a.Reader.ReadUint32(a.PlayerMgrPtrAddr)
	renderer, _ = a.Reader.ReadUint32(a.RendererPtrAddr)

	if (!IsValidUserAddr(playerMgr) || !IsValidUserAddr(renderer)) && IsValidUserAddr(classMgr) {
		classes := TraverseClassManager(a.Reader, classMgr)
		if !IsValidUserAddr(playerMgr) {
			if pm, ok := classes["PlayerManager"]; ok {
				playerMgr = pm
			}
		}
		if !IsValidUserAddr(renderer) {
			if rend, ok := classes["Renderer"]; ok {
				renderer = rend
			}
		}
	}
	return classMgr, playerMgr, renderer
}

func (a *Attachment) ReadCamera(renderer uint32) (uint32, math3d.CameraData) {
	var cam math3d.CameraData
	if !IsValidUserAddr(renderer) {
		return 0, cam
	}
	activeCam, errAC := a.Reader.ReadUint32(renderer + OffRendererActiveCam)
	if errAC != nil || !IsValidUserAddr(activeCam) {
		return 0, cam
	}

	if pos, errP := a.Reader.ReadVector3(activeCam + OffCamPos); errP == nil && math3d.IsValidVec3(pos) {
		cam.Pos = pos
	}
	if vm, errV := a.Reader.ReadMatrix4x4(activeCam + OffCamViewMat); errV == nil && math3d.IsValidMatrix(vm) {
		cam.ViewMatrix = vm
	}
	if pm, errProj := a.Reader.ReadMatrix4x4(activeCam + OffCamProjMat); errProj == nil && math3d.IsValidProjMatrix(pm) {
		cam.ProjMatrix = pm
		cam.HasValidProj = true
	}
	if vpW, errW := a.Reader.ReadFloat32(activeCam + OffCamVpWidth); errW == nil && math3d.IsFiniteFloat(vpW) {
		cam.VpWidth = vpW
	}
	if vpH, errH := a.Reader.ReadFloat32(activeCam + OffCamVpHeight); errH == nil && math3d.IsFiniteFloat(vpH) {
		cam.VpHeight = vpH
	}
	if fov1C, errF := a.Reader.ReadFloat32(activeCam + OffCamFov1C); errF == nil && math3d.IsFiniteFloat(fov1C) {
		cam.FovRad1C = fov1C
	}
	if fov24, errF2 := a.Reader.ReadFloat32(activeCam + OffCamFovRad24); errF2 == nil && math3d.IsFiniteFloat(fov24) {
		cam.FovRad24 = fov24
	}
	if asp34, errA := a.Reader.ReadFloat32(activeCam + OffCamAspect34); errA == nil && math3d.IsFiniteFloat(asp34) {
		cam.Aspect34 = asp34
	}
	return activeCam, cam
}

// ReadDemoState reads the native BattleRecorder controller via [0x00A1B5D8]+0x140+0xF0.
func (a *Attachment) ReadDemoState() DemoState {
	var st DemoState
	if a == nil || a.Reader == nil || !IsValidUserAddr(a.GamePtrAddr) {
		return st
	}
	gamePtr, err := a.Reader.ReadUint32(a.GamePtrAddr)
	if err != nil || !IsValidUserAddr(gamePtr) {
		return st
	}
	st.GamePtr = gamePtr

	demoRec, err := a.Reader.ReadUint32(gamePtr + OffGameDemoRecorder)
	if err != nil || !IsValidUserAddr(demoRec) {
		return st
	}
	st.DemoRecorderPtr = demoRec

	activeFlag, errA := a.Reader.ReadUint32(demoRec + OffDemoRecActiveFlag)
	stoppedFlag, errS := a.Reader.ReadUint8(demoRec + OffDemoRecStoppedFlag)
	if errA != nil || errS != nil || activeFlag == 0 || stoppedFlag != 0 {
		return st
	}

	replayCam, errRC := a.Reader.ReadUint32(demoRec + OffDemoRecReplayCam)
	if errRC != nil || !IsValidUserAddr(replayCam) {
		return st
	}
	st.ReplayCamPtr = replayCam
	st.DemoActive = true

	if mode, errM := a.Reader.ReadInt32(replayCam + OffReplayCamMode); errM == nil {
		st.CameraMode = mode
	}
	if fid, errID := a.Reader.ReadInt32(replayCam + OffReplayCamFollowedID); errID == nil {
		st.FollowedPlayerID = fid
	} else {
		st.FollowedPlayerID = -1
	}
	return st
}

// SetDemoFollowPlayer performs a one-shot write to ReplayCamera to switch BattleRecorder's spectated player.
func (a *Attachment) SetDemoFollowPlayer(playerID int32) error {
	if a == nil || a.Reader == nil || !a.CanWrite {
		return fmt.Errorf("process write access unavailable")
	}
	st := a.ReadDemoState()
	if !st.DemoActive || !IsValidUserAddr(st.ReplayCamPtr) {
		return fmt.Errorf("BattleRecorder demo camera not active")
	}
	if err := a.Reader.WriteInt32(st.ReplayCamPtr+OffReplayCamFollowedID, playerID); err != nil {
		return err
	}
	if st.CameraMode == 0 {
		if err := a.Reader.WriteInt32(st.ReplayCamPtr+OffReplayCamMode, 1); err != nil {
			return err
		}
	}
	return nil
}

// ReadPlaybackSpeed reads the current demo playback speed multiplier.
func (a *Attachment) ReadPlaybackSpeed() (float32, bool) {
	if a == nil || a.Reader == nil || a.BF2142Base == 0 {
		return 1.0, false
	}
	timeMgrPtrAddr := a.BF2142Base + OffTimeManagerStatic
	timeMgr, err := a.Reader.ReadUint32(timeMgrPtrAddr)
	if err != nil || !IsValidUserAddr(timeMgr) {
		return 1.0, false
	}
	sp, errS := a.Reader.ReadFloat32(timeMgr + OffTimeMgrSpeed)
	if errS != nil || !math3d.IsFiniteFloat(sp) || sp <= 0.01 {
		return 1.0, false
	}
	return sp, true
}

// SetPlaybackSpeed sets the demo playback speed multiplier via TimeManager (+0x1C).
func (a *Attachment) SetPlaybackSpeed(speed float32) error {
	if a == nil || a.Reader == nil || !a.CanWrite || a.BF2142Base == 0 {
		return fmt.Errorf("write access unavailable")
	}
	timeMgrPtrAddr := a.BF2142Base + OffTimeManagerStatic
	timeMgr, err := a.Reader.ReadUint32(timeMgrPtrAddr)
	if err != nil || !IsValidUserAddr(timeMgr) {
		return fmt.Errorf("TimeManager unavailable")
	}
	return a.Reader.WriteFloat32(timeMgr+OffTimeMgrSpeed, speed)
}

func isDefaultDeadPos(v math3d.Vector3) bool {
	isZero := math.Abs(float64(v.X)) < 0.05 && math.Abs(float64(v.Y)) < 0.05 && math.Abs(float64(v.Z)) < 0.05
	isOrigin1 := math.Abs(float64(v.X)) < 0.05 && math.Abs(float64(v.Y-1.0)) < 0.05 && math.Abs(float64(v.Z)) < 0.05
	return isZero || isOrigin1
}

func (a *Attachment) readValidObject(wrapperPtrAddr uint32) (objPtr uint32, flags byte, pos math3d.Vector3, valid bool) {
	wrapper, errW := a.Reader.ReadUint32(wrapperPtrAddr)
	if errW != nil || !IsValidUserAddr(wrapper) {
		return 0, 0, math3d.Vector3{}, false
	}
	obj, errO := a.Reader.ReadUint32(wrapper + OffObjPtrTarget)
	if errO != nil || !IsValidUserAddr(obj) {
		return 0, 0, math3d.Vector3{}, false
	}
	fl, errF := a.Reader.ReadUint8(obj + OffObjectFlags)
	if errF != nil || (fl&OffObjectFlagDead) != 0 {
		return obj, fl, math3d.Vector3{}, false
	}
	p, errP := a.Reader.ReadVector3(obj + OffObjectPos)
	if errP != nil || !math3d.IsValidVec3(p) || isDefaultDeadPos(p) {
		return obj, fl, math3d.Vector3{}, false
	}
	// Rotation matrix rows and unit vectors have all components <= 1.05.
	// Map coordinates in BF2142 are never inside a 1-meter cube around origin.
	if math.Abs(float64(p.X)) <= 1.05 && math.Abs(float64(p.Y)) <= 1.05 && math.Abs(float64(p.Z)) <= 1.05 {
		return obj, fl, math3d.Vector3{}, false
	}
	return obj, fl, p, true
}

type PlayerListResult struct {
	ListHead     uint32
	PlayerCount  uint32
	ValidList    bool
	AllPlayers   []PlayerInfo
	AlivePlayers []PlayerInfo
}

func (a *Attachment) ReadPlayers(playerMgr uint32, camPos math3d.Vector3, useNameCache bool) PlayerListResult {
	var res PlayerListResult
	if !IsValidUserAddr(playerMgr) {
		return res
	}
	listHead, errH := a.Reader.ReadUint32(playerMgr + OffPlayerMgrListHead)
	playerCount, errC := a.Reader.ReadUint32(playerMgr + OffPlayerMgrCount)
	if errH != nil || errC != nil || !IsValidUserAddr(listHead) {
		return res
	}
	res.ListHead = listHead
	res.PlayerCount = playerCount
	if playerCount == 0 || playerCount > 64 {
		return res
	}
	res.ValidList = true

	curr, _ := a.Reader.ReadUint32(listHead)
	visitedNodes := make(map[uint32]bool)
	visitedPlayers := make(map[uint32]bool)
	visitedIDs := make(map[int32]bool)

	for i := uint32(0); i < playerCount && IsValidUserAddr(curr) && curr != listHead && !visitedNodes[curr]; i++ {
		visitedNodes[curr] = true
		playerPtr, errP := a.Reader.ReadUint32(curr + OffListNodePlayer)
		if errP == nil && IsValidUserAddr(playerPtr) && !visitedPlayers[playerPtr] {
			visitedPlayers[playerPtr] = true
			pidVal, errID := a.Reader.ReadInt32(playerPtr + OffPlayerID)
			if errID != nil {
				pidVal = int32(i)
			}
			if visitedIDs[pidVal] {
				curr, _ = a.Reader.ReadUint32(curr)
				continue
			}
			visitedIDs[pidVal] = true

			var name string
			if useNameCache && a.NameCache != nil {
				cacheKey := (uint64(playerPtr) << 32) | uint64(uint32(pidVal))
				var okName bool
				name, okName = a.NameCache[cacheKey]
				if !okName || name == "" || name == "<unknown>" {
					name = a.Reader.ReadCPlayerName(playerPtr)
					if name != "" && name != "<unknown>" {
						if len(a.NameCache) > 256 {
							a.NameCache = make(map[uint64]string)
						}
						a.NameCache[cacheKey] = name
					}
				}
			} else {
				name = a.Reader.ReadCPlayerName(playerPtr)
			}

			team, errTeam := a.Reader.ReadUint32(playerPtr + OffPlayerTeam)
			if errTeam != nil {
				team = 0
			}

			aliveD5, errD5 := a.Reader.ReadUint8(playerPtr + OffPlayerAliveD5)
			alive10C, _ := a.Reader.ReadUint8(playerPtr + OffPlayerAlive10C)

			var scoreVal, killsVal, deathsVal int32
			hasScore := false
			if scorePtr, errS := a.Reader.ReadUint32(playerPtr + OffPlayerScorePtr); errS == nil && IsValidUserAddr(scorePtr) {
				s, e0 := a.Reader.ReadInt32(scorePtr + OffScoreTotal)
				d, e1 := a.Reader.ReadInt32(scorePtr + OffScoreDeaths)
				k, e2 := a.Reader.ReadInt32(scorePtr + OffScoreKills)
				if e0 == nil && e1 == nil && e2 == nil {
					scoreVal, deathsVal, killsVal = s, d, k
					hasScore = true
				}
			}

			// Read CViewMatrix (+0x84) for LookDir & EyePos (active only for spectated player during demo replay)
			var eyePos, lookDir, rawEyePos, rawLookDir math3d.Vector3
			hasPose := false
			hasRawEye := false
			hasRawLook := false
			isDeadDefault := false

			if vmPtr, errVM := a.Reader.ReadUint32(playerPtr + OffPlayerViewMatrix); errVM == nil && IsValidUserAddr(vmPtr) {
				ld, errD := a.Reader.ReadVector3(vmPtr + OffViewMatLookDir)
				ep, errE := a.Reader.ReadVector3(vmPtr + OffViewMatEyePos)
				if errE == nil && math3d.IsValidVec3(ep) {
					rawEyePos = ep
					hasRawEye = true
				}
				if errD == nil && math3d.IsValidVec3(ld) {
					rawLookDir = ld
					hasRawLook = true
				}
				lookLen := math.Sqrt(float64(ld.X*ld.X + ld.Y*ld.Y + ld.Z*ld.Z))
				isDeadDefault = isDefaultDeadPos(ep)

				if errD == nil && errE == nil && math3d.IsValidVec3(ep) && math3d.IsValidVec3(ld) && !isDeadDefault && lookLen > 0.5 {
					eyePos = ep
					lookDir = math3d.Vec3Normalize(ld)
					hasPose = true
				}
			}

			// Read CSoldier (CPlayer + 0x80 -> +0x04 or direct, pos at +0xC8 or Nametag Comp +0x314 -> +0x60)
			soldierPtr, soldierFlags, soldierPos, hasSoldier := a.readValidObject(playerPtr + OffPlayerSoldierPtr)

			// Secondary / PCO position: CPlayer + 0x88
			pcoPtr, _, pcoPos, hasPco := a.readValidObject(playerPtr + OffPlayerControlledWrapper)

			// InVehicle check:
			// Default InVehicle = false for standing infantry.
			// Infantry on foot ALWAYS has hasSoldier = true. Only treat InVehicle = true
			// if player has NO soldier pawn on foot (!hasSoldier) but HAS a valid controlled vehicle object.
			inVeh := false
			if !hasSoldier && hasPco && IsValidUserAddr(pcoPtr) {
				inVeh = true
			}

			// Primary world position:
			var worldPos math3d.Vector3
			hasWorldPos := false
			if inVeh && hasPco {
				worldPos = pcoPos
				hasWorldPos = true
			} else if hasSoldier {
				worldPos = soldierPos
				hasWorldPos = true
			} else if hasPco {
				worldPos = pcoPos
				hasWorldPos = true
			}

			// Validate CSoldier/CObject position against known CViewMatrix EyePos (+0xF8).
			// If distance between worldPos and eyePos exceeds 15m, CObject pos offset is wrong or stale.
			if hasWorldPos && hasPose && math3d.Vec3Dist(worldPos, eyePos) > 15.0 {
				hasWorldPos = false
			}

			// Alive status verified from ReplayCamera (0x005F9E86): CPlayer+0xD5 != 0 && (hasWorldPos || hasPose)
			var isAlive bool
			if errD5 == nil {
				isAlive = (aliveD5 != 0) && (hasWorldPos || hasPose)
			} else {
				isAlive = hasWorldPos || hasPose
			}

			// Anchor FeetPos & HeadPos cleanly on live CSoldier/PCO origin (zero underground boxes)
			var feetPos, headPos math3d.Vector3
			if hasWorldPos {
				feetPos = worldPos
				if inVeh {
					headPos = math3d.Vector3{X: worldPos.X, Y: worldPos.Y + 2.2, Z: worldPos.Z}
				} else {
					height := float32(1.75)
					if hasPose {
						dx := math.Abs(float64(eyePos.X - feetPos.X))
						dz := math.Abs(float64(eyePos.Z - feetPos.Z))
						dy := eyePos.Y - feetPos.Y
						if dx < 1.5 && dz < 1.5 && dy >= 0.2 && dy <= 2.1 {
							height = dy + 0.22
						}
					}
					headPos = math3d.Vector3{X: feetPos.X, Y: feetPos.Y + height, Z: feetPos.Z}
				}
				if !hasPose {
					eyePos = math3d.Vector3{X: feetPos.X, Y: headPos.Y - 0.18, Z: feetPos.Z}
				}
			} else if hasPose {
				// Fallback to ViewMat EyePos (+0xF8): derive soldier origin and head/feet accurately
				worldPos = math3d.Vector3{X: eyePos.X, Y: eyePos.Y - 1.55, Z: eyePos.Z}
				feetPos = worldPos
				headPos = math3d.Vector3{X: eyePos.X, Y: eyePos.Y + 0.22, Z: eyePos.Z}
				hasWorldPos = true
			}

			var dist float32
			if isAlive && (hasWorldPos || hasPose) {
				dist = math3d.Vec3Dist(worldPos, camPos)
			}

			info := PlayerInfo{
				Addr:          playerPtr,
				ID:            pidVal,
				Name:          name,
				Team:          team,
				IsAlive:       isAlive,
				InVehicle:     inVeh,
				HasScore:      hasScore,
				Score:         scoreVal,
				Kills:         killsVal,
				Deaths:        deathsVal,
				SoldierPtr:    soldierPtr,
				ControlledPtr: pcoPtr,
				SoldierFlags:  soldierFlags,
				AliveD5:       aliveD5,
				Alive10C:      alive10C,
				WorldPos:      worldPos,
				HasWorldPos:   hasWorldPos,
				FeetPos:       feetPos,
				HeadPos:       headPos,
				EyePos:        eyePos,
				LookDir:       lookDir,
				RawEyePos:     rawEyePos,
				RawLookDir:    rawLookDir,
				HasRawEye:     hasRawEye,
				HasRawLook:    hasRawLook,
				IsDeadDefault: isDeadDefault,
				HasPose:       hasPose,
				Distance:      dist,
			}
			res.AllPlayers = append(res.AllPlayers, info)
			if info.IsAlive {
				res.AlivePlayers = append(res.AlivePlayers, info)
			}
		}
		curr, _ = a.Reader.ReadUint32(curr)
	}
	return res
}
