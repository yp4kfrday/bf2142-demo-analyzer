package math3d

import "math"

type Vector3 struct {
	X, Y, Z float32
}

type Point2D struct {
	X, Y int32
}

type Point2DF struct {
	X, Y float32
}

type CameraData struct {
	Pos          Vector3
	ViewMatrix   [4][4]float32
	ProjMatrix   [4][4]float32
	HasValidProj bool
	VpWidth      float32
	VpHeight     float32
	FovRad1C     float32
	FovRad24     float32
	Aspect34     float32
}

func IsFiniteFloat(v float32) bool {
	f := float64(v)
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

func IsValidVec3(v Vector3) bool {
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

func IsValidMatrix(m [4][4]float32) bool {
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if !IsFiniteFloat(m[r][c]) {
				return false
			}
		}
	}
	return true
}

func IsValidProjMatrix(m [4][4]float32) bool {
	if !IsValidMatrix(m) {
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

func Vec3Dist(a, b Vector3) float32 {
	dx := float64(a.X - b.X)
	dy := float64(a.Y - b.Y)
	dz := float64(a.Z - b.Z)
	return float32(math.Sqrt(dx*dx + dy*dy + dz*dz))
}

func Vec3Normalize(v Vector3) Vector3 {
	length := float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
	if length < 1e-6 {
		return Vector3{}
	}
	inv := 1.0 / length
	return Vector3{v.X * inv, v.Y * inv, v.Z * inv}
}

func Vec3Cross(a, b Vector3) Vector3 {
	return Vector3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

func AngleBetween(v1, v2 Vector3) float32 {
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

func ClampF32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func ScaleI32(v int32, scale float32) int32 {
	res := int32(math.Round(float64(float32(v) * scale)))
	if res == 0 && v != 0 {
		if v > 0 {
			return 1
		}
		return -1
	}
	return res
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

	// ActiveCamera+0x24 is the actual vertical FOV in radians (RendDX9 Camera::updateProj 0x10066F5B).
	fovY := float32(1.178097) // 67.5 deg BF2142 default
	if IsFiniteFloat(cam.FovRad24) && cam.FovRad24 >= 0.25 && cam.FovRad24 <= 2.6 {
		fovY = cam.FovRad24
	}
	tanHalf := float32(math.Tan(float64(fovY) * 0.5))
	if tanHalf < 0.01 {
		tanHalf = 0.668179
	}
	fy := (halfH / tanHalf) * fovScale

	vpAspect := screenW / screenH
	if cam.VpWidth > 100 && cam.VpHeight > 100 {
		vpAspect = cam.VpWidth / cam.VpHeight
	} else if IsFiniteFloat(cam.Aspect34) && cam.Aspect34 > 0.2 && cam.Aspect34 < 2.0 {
		vpAspect = 1.0 / cam.Aspect34
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
	if IsFiniteFloat(cam.FovRad24) && cam.FovRad24 >= 0.25 && cam.FovRad24 <= 2.6 {
		return cam.FovRad24 * 180.0 / math.Pi
	}
	return 67.5
}

func (cam *CameraData) ViewToScreenFloat(v Vector3, screenW, screenH, fovScale float32) (Point2DF, bool) {
	if !IsFiniteFloat(v.X) || !IsFiniteFloat(v.Y) || !IsFiniteFloat(v.Z) || v.Z < 0.1 {
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
		if !IsFiniteFloat(cx) || !IsFiniteFloat(cy) || !IsFiniteFloat(cw) || cw <= 0.05 {
			return Point2DF{}, false
		}
		sx = halfW + (cx/cw)*halfW*fovScale
		sy = halfH - (cy/cw)*halfH*fovScale
	} else {
		fx, fy := cam.getFallbackFocal(screenW, screenH, fovScale)
		sx = halfW + (v.X/v.Z)*fx
		sy = halfH - (v.Y/v.Z)*fy
	}

	if !IsFiniteFloat(sx) || !IsFiniteFloat(sy) || math.Abs(float64(sx)) > 1e6 || math.Abs(float64(sy)) > 1e6 {
		return Point2DF{}, false
	}
	return Point2DF{X: sx, Y: sy}, true
}

func (cam *CameraData) WorldToScreen(p Vector3, screenW, screenH, fovScale float32) (Point2D, bool) {
	if !IsValidVec3(p) {
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
			return Point2D{X: int32(math.Round(float64(x0))), Y: int32(math.Round(float64(y0)))},
				Point2D{X: int32(math.Round(float64(x1))), Y: int32(math.Round(float64(y1)))}, true
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
	if !IsValidVec3(p1) || !IsValidVec3(p2) {
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
