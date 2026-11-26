package game

import "demoanalyzer/internal/math3d"

const (
	// Static module offsets (BF2142 v1.51)
	OffGameStatic          uint32 = 0x61B5D8 // BF2142.exe -> 0x00A1B5D8 (Game*)
	OffClassManagerStatic  uint32 = 0x67D068 // BF2142.exe -> 0x00A7D068
	OffPlayerManagerStatic uint32 = 0x685228 // BF2142.exe -> 0x00A85228
	OffRendererStatic      uint32 = 0x1F8E58 // RendDX9.dll -> 0x101F8E58

	// Game -> DemoRecorder -> ReplayCamera (BattleRecorder spectating)
	OffGameDemoRecorder    uint32 = 0x140 // Game + 0x140 = DemoRecorder*
	OffDemoRecActiveFlag   uint32 = 0xCC  // uint32 != 0 when demo active
	OffDemoRecStoppedFlag  uint32 = 0xD5  // uint8 == 0 when playing/running
	OffDemoRecReplayCam    uint32 = 0xF0  // DemoRecorder + 0xF0 = ReplayCamera*
	OffReplayCamFollowedID uint32 = 0x08  // int32 CPlayer+0xB8 ID (-1 = none)
	OffReplayCamMode       uint32 = 0x10  // int32: 0 = FreeCam, 1 = FollowPlayer

	// CRenderer
	OffRendererActiveCam uint32 = 0xF8

	// ActiveCamera (RendDX9)
	OffCamVpWidth    uint32 = 0x10
	OffCamVpHeight   uint32 = 0x14
	OffCamFov1C      uint32 = 0x1C // Zoom ratio cot(fov/2)/cot(globalFov/2), NOT radians
	OffCamFovRad24   uint32 = 0x24 // Actual vertical FOV in radians
	OffCamAspect34   uint32 = 0x34 // Viewport aspect ratio (H/W)
	OffCamWorldMat   uint32 = 0x40
	OffCamPos        uint32 = 0x70
	OffCamViewMat    uint32 = 0x80
	OffCamProjMat    uint32 = 0x140
	OffCamNearPlaneA uint32 = 0x14C
	OffCamNearPlaneB uint32 = 0x150
	OffCamNearPlaneC uint32 = 0x154
	OffCamNearPlaneD uint32 = 0x158

	// ClassManager std::map
	OffClassMgrHead uint32 = 0x34
	OffClassMgrSize uint32 = 0x38
	OffMapRoot      uint32 = 0x04
	OffMapLeft      uint32 = 0x00
	OffMapRight     uint32 = 0x08
	OffMapKeyA      uint32 = 0x0C
	OffMapKeyB      uint32 = 0x10
	OffMapValPtr    uint32 = 0x28

	// PlayerManager std::list
	OffPlayerMgrListHead uint32 = 0x10
	OffPlayerMgrCount    uint32 = 0x14
	OffListNodePlayer    uint32 = 0x08

	// CPlayer
	OffPlayerNameA              uint32 = 0x60
	OffPlayerNameB              uint32 = 0x64
	OffPlayerSoldierPtr         uint32 = 0x80 // ObjPtr<CSoldier>* -> +0x04 = CSoldier*
	OffPlayerViewMatrix         uint32 = 0x84 // CViewMatrix* (EyePos/LookDir, can stall when occluded)
	OffPlayerID                 uint32 = 0xB8 // int32 persistent ID
	// +0x88: ObjPtr<PCO>* pointing to CSoldier when on-foot, CVehicle when in vehicle.
	// When *(CPlayer+0x88) != *(CPlayer+0x80), the player is in a vehicle.
	// (OffPlayerControlledObj = 0xCC was confirmed empty for all players — removed)
	OffPlayerControlledWrapper  uint32 = 0x88 // ObjPtr<PCO>* -> +0x04 = PCO* (vehicle or soldier)
	OffPlayerAliveD5            uint32 = 0xD5 // uint8 (CPlayer::vt[+0x74] alive/spawned check)
	OffPlayerTeam               uint32 = 0xD8 // uint32
	OffPlayerScorePtr           uint32 = 0x100
	OffPlayerAlive10C           uint32 = 0x10C

	// CObject / CSoldier / CVehicle
	OffObjPtrTarget   uint32 = 0x04 // ObjPtr wrapper -> CObject*
	OffObjectFlags    uint32 = 0x14 // bit 0x20 == destroyed/dead
	OffObjectFlagDead byte   = 0x20
	OffObjectWorldMat uint32 = 0x88 // 4x4 world matrix on CObject
	// OffObjectPos: translation row inside CObject world matrix (+0xC8 = translation XYZ)
	OffObjectPos uint32 = 0xC8

	// PlayerScore
	OffScoreTotal  uint32 = 0x00
	OffScoreDeaths uint32 = 0x10
	OffScoreKills  uint32 = 0x14

	// CViewMatrix
	OffViewMatLookDir uint32 = 0xE8
	OffViewMatEyePos  uint32 = 0xF8

	// TimeManager (Demo playback speed modifier)
	OffTimeManagerStatic uint32 = 0x763970 // BF2142.exe -> 0x00B63970 (TimeManager*)
	OffTimeMgrSpeed      uint32 = 0x1C     // float32 playback speed multiplier (default 1.0)

	MinUserAddr uint32 = 0x10000
	MaxUserAddr uint32 = 0x7FFE0000
)

type DemoState struct {
	GamePtr          uint32
	DemoRecorderPtr  uint32
	ReplayCamPtr     uint32
	DemoActive       bool
	CameraMode       int32
	FollowedPlayerID int32
}

type PlayerInfo struct {
	Addr          uint32
	ID            int32
	Name          string
	Team          uint32
	IsAlive       bool
	InVehicle     bool
	HasScore      bool
	Score         int32
	Kills         int32
	Deaths        int32
	SoldierPtr    uint32
	ControlledPtr uint32
	SoldierFlags  byte
	AliveD5       byte
	Alive10C      byte
	WorldPos      math3d.Vector3
	HasWorldPos   bool
	FeetPos       math3d.Vector3
	HeadPos       math3d.Vector3
	EyePos        math3d.Vector3
	LookDir       math3d.Vector3
	RawEyePos     math3d.Vector3
	RawLookDir    math3d.Vector3
	HasRawEye     bool
	HasRawLook    bool
	IsDeadDefault bool
	HasPose       bool // True only when CViewMatrix (+0x84) has live EyePos & LookDir
	Distance      float32
}
