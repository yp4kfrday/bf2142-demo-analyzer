package aim

import (
	"fmt"
	"math"
	"os"
	"time"

	"demoanalyzer/internal/game"
	"demoanalyzer/internal/math3d"
)

type AimTick struct {
	Time      time.Time
	LookDir   math3d.Vector3
	EnemyID   int32
	EnemyName string
	EnemyDist float32
	Angle     float32
	EnemyPos  math3d.Vector3
}

type AimAnalyzer struct {
	Ticks          []AimTick
	PrevTargetLook math3d.Vector3
	PrevTargetPos  math3d.Vector3
	PrevEnemyPos   math3d.Vector3
	HasPrevSample  bool

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
	LockStartPos        math3d.Vector3
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

func ClosestAimPoint(target *game.PlayerInfo, enemy *game.PlayerInfo) (math3d.Vector3, float32) {
	headPos := enemy.HeadPos
	if enemy.HasPose {
		headPos = enemy.EyePos
	} else if !enemy.HasWorldPos {
		headPos = enemy.EyePos
	}
	toHead := math3d.Vector3{
		X: headPos.X - target.EyePos.X,
		Y: headPos.Y - target.EyePos.Y,
		Z: headPos.Z - target.EyePos.Z,
	}
	bestPt := headPos
	bestAng := math3d.AngleBetween(target.LookDir, toHead)

	if !enemy.InVehicle && (enemy.HasWorldPos || enemy.HasPose) {
		bodyY := headPos.Y - 0.55
		if enemy.HasWorldPos {
			bodyY = enemy.FeetPos.Y + (headPos.Y-enemy.FeetPos.Y)*0.62
		}
		bodyPos := math3d.Vector3{
			X: headPos.X,
			Y: bodyY,
			Z: headPos.Z,
		}
		toBody := math3d.Vector3{
			X: bodyPos.X - target.EyePos.X,
			Y: bodyPos.Y - target.EyePos.Y,
			Z: bodyPos.Z - target.EyePos.Z,
		}
		if angBody := math3d.AngleBetween(target.LookDir, toBody); angBody < bestAng {
			bestAng = angBody
			bestPt = bodyPos
		}
	}
	return bestPt, bestAng
}

func FindClosestEnemy(targetPlayer *game.PlayerInfo, alivePlayers []game.PlayerInfo) (closestEnemy *game.PlayerInfo, closestAimPt math3d.Vector3, minAngle float32, closestDist float32) {
	minAngle = 999.0
	if targetPlayer == nil || !targetPlayer.IsAlive || !targetPlayer.HasPose {
		return nil, math3d.Vector3{}, minAngle, 0
	}
	for i := range alivePlayers {
		p := &alivePlayers[i]
		if p.ID == targetPlayer.ID {
			continue
		}
		if targetPlayer.Team != 0 && p.Team == targetPlayer.Team {
			continue
		}
		if !p.HasWorldPos && !p.HasPose {
			continue
		}

		aimPt, ang := ClosestAimPoint(targetPlayer, p)
		if ang < minAngle {
			minAngle = ang
			closestEnemy = p
			closestAimPt = aimPt
			enemyOrigin := p.EyePos
			if p.HasWorldPos {
				enemyOrigin = p.WorldPos
			}
			closestDist = math3d.Vec3Dist(enemyOrigin, targetPlayer.EyePos)
		}
	}
	return closestEnemy, closestAimPt, minAngle, closestDist
}

func (a *AimAnalyzer) PauseOnDeath(targetName string) {
	a.FinalizeLock(time.Now(), targetName)
	a.HasPrevSample = false
}

func (a *AimAnalyzer) Reset(targetName string) {
	a.FinalizeLock(time.Now(), targetName)

	a.Ticks = a.Ticks[:0]
	a.HasPrevSample = false
	a.PrevTargetLook = math3d.Vector3{}
	a.PrevTargetPos = math3d.Vector3{}
	a.PrevEnemyPos = math3d.Vector3{}
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

func (a *AimAnalyzer) ProcessFrame(targetPlayer *game.PlayerInfo, closestEnemy *game.PlayerInfo, minAngle, closestDist float32) {
	if targetPlayer == nil || !targetPlayer.IsAlive || !targetPlayer.HasPose {
		if targetPlayer != nil {
			a.PauseOnDeath(targetPlayer.Name)
		} else {
			a.PauseOnDeath("None")
		}
		return
	}

	now := time.Now()
	isNewTick := !a.HasPrevSample ||
		math3d.AngleBetween(a.PrevTargetLook, targetPlayer.LookDir) > 0.005 ||
		math3d.Vec3Dist(a.PrevTargetPos, targetPlayer.EyePos) > 0.005 ||
		(closestEnemy != nil && math3d.Vec3Dist(a.PrevEnemyPos, closestEnemy.EyePos) > 0.005)

	if isNewTick && closestEnemy != nil {
		a.PrevTargetLook = targetPlayer.LookDir
		a.PrevTargetPos = targetPlayer.EyePos
		a.PrevEnemyPos = closestEnemy.EyePos
		a.HasPrevSample = true

		tick := AimTick{
			Time:      now,
			LookDir:   targetPlayer.LookDir,
			EnemyID:   closestEnemy.ID,
			EnemyName: closestEnemy.Name,
			EnemyDist: closestDist,
			Angle:     minAngle,
			EnemyPos:  closestEnemy.EyePos,
		}
		a.Ticks = append(a.Ticks, tick)
		if len(a.Ticks) > 30 {
			a.Ticks = a.Ticks[len(a.Ticks)-30:]
		}
		a.CheckSnap(now, targetPlayer.Name)
	}

	a.UpdateLock(now, targetPlayer.Name, closestEnemy, minAngle, closestDist)
}

func (a *AimAnalyzer) CheckSnap(now time.Time, targetName string) {
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

func (a *AimAnalyzer) UpdateLock(now time.Time, targetName string, enemy *game.PlayerInfo, angle float32, dist float32) {
	if enemy == nil || angle > 1.8 {
		if a.IsLocking {
			a.LockMissFrames++
			if a.LockMissFrames > 2 || enemy == nil {
				a.FinalizeLock(now, targetName)
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
		a.FinalizeLock(now, targetName)
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
	moved := math3d.Vec3Dist(a.LockStartPos, enemy.EyePos)
	if moved > a.LockMaxMoved {
		a.LockMaxMoved = moved
	}
	a.CurrentLockDuration = float32(now.Sub(a.LockStartTime).Seconds())
}

func (a *AimAnalyzer) FinalizeLock(now time.Time, targetName string) {
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
