package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
)

var (
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")

	procOpenProcess          = modKernel32.NewProc("OpenProcess")
	procCloseHandle          = modKernel32.NewProc("CloseHandle")
	procReadProcessMemory    = modKernel32.NewProc("ReadProcessMemory")
	procGetExitCodeProcess   = modKernel32.NewProc("GetExitCodeProcess")
	procCreateToolhelp32Snap = modKernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First       = modKernel32.NewProc("Process32FirstW")
	procProcess32Next        = modKernel32.NewProc("Process32NextW")
	procModule32First        = modKernel32.NewProc("Module32FirstW")
	procModule32Next         = modKernel32.NewProc("Module32NextW")
)

const (
	PROCESS_VM_READ           = 0x0010
	PROCESS_QUERY_INFORMATION = 0x0400
	STILL_ACTIVE              = 259

	TH32CS_SNAPPROCESS  = 0x00000002
	TH32CS_SNAPMODULE   = 0x00000008
	TH32CS_SNAPMODULE32 = 0x00000010

	minUserAddr = uint32(0x10000)
	maxUserAddr = uint32(0x7FFE0000)
)

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
		return fmt.Errorf("address out of user-mode range: 0x%08X (len %d)", addr, len(buf))
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
		return fmt.Errorf("partial ReadProcessMemory at 0x%08X: %d/%d bytes", addr, bytesRead, len(buf))
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

func (m *MemoryReader) ReadVector3(addr uint32) ([3]float32, error) {
	var buf [12]byte
	var v [3]float32
	if err := m.ReadRaw(addr, buf[:]); err != nil {
		return v, err
	}
	v[0] = math.Float32frombits(binary.LittleEndian.Uint32(buf[0:4]))
	v[1] = math.Float32frombits(binary.LittleEndian.Uint32(buf[4:8]))
	v[2] = math.Float32frombits(binary.LittleEndian.Uint32(buf[8:12]))
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

func isValidVec3(v [3]float32) bool {
	for i := 0; i < 3; i++ {
		val := float64(v[i])
		if math.IsNaN(val) || math.IsInf(val, 0) || math.Abs(val) > 50000 {
			return false
		}
	}
	return true
}

func isValidProjMatrix(m [4][4]float32) bool {
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if !isFiniteFloat(m[r][c]) {
				return false
			}
		}
	}
	if math.Abs(float64(m[0][0])) <= 0.05 || math.Abs(float64(m[1][1])) <= 0.05 {
		return false
	}
	if m[0][0] == 1.0 && m[1][1] == 1.0 && m[2][2] == 1.0 {
		return false
	}
	return true
}

// ReadStdString safely reads an MSVC basic_string<char> (both 28-byte and 24-byte layouts).
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

	// 1. MSVC 7.1 layout (28 bytes: +0x04 buf/ptr, +0x14 size, +0x18 cap)
	if s, ok := tryLayout(4, 20, 24); ok {
		return s
	}
	// 2. MSVC 2005/2008 layout (24 bytes: +0x00 buf/ptr, +0x10 size, +0x14 cap)
	if s, ok := tryLayout(0, 16, 20); ok {
		return s
	}

	// 3. Fallback: direct null-terminated inline ASCII/UTF-8
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

// Traverse ClassManager std::map<string, void*> at +0x30
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

func scanPlayer(m *MemoryReader, playerAddr uint32) {
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
		if ptr >= 0x00400000 && ptr < maxUserAddr {
			v, err := m.ReadVector3(ptr)
			if err == nil && isValidVec3(v) && (v[0] != 0 || v[1] != 0 || v[2] != 0) {
				fmt.Printf("  Offset +0x%03X -> Ptr 0x%08X: Pos=(%.2f, %.2f, %.2f)\n", i, ptr, v[0], v[1], v[2])
			}
			matV, errM := m.ReadVector3(ptr + 0xF8)
			if errM == nil && isValidVec3(matV) && (matV[0] != 0 || matV[1] != 0 || matV[2] != 0) {
				fmt.Printf("  Offset +0x%03X -> MatrixPtr 0x%08X (+0xF8 EyePos=(%.2f, %.2f, %.2f))\n", i, ptr, matV[0], matV[1], matV[2])
			}
		}
	}
	fmt.Println("========================================================")
	fmt.Println()
}

func dumpPlayerRawRanges(m *MemoryReader, label string, playerAddr uint32) {
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

func attachToGame(sigChan <-chan os.Signal) (uint32, *MemoryReader, uint32, uint32, bool) {
	fmt.Print("[*] Waiting for BF2142.exe process... ")
	for {
		select {
		case <-sigChan:
			return 0, nil, 0, 0, false
		default:
		}
		pid, err := findProcess("BF2142.exe")
		if err == nil {
			hProcess, _, openErr := procOpenProcess.Call(
				uintptr(PROCESS_VM_READ|PROCESS_QUERY_INFORMATION),
				0,
				uintptr(pid),
			)
			if hProcess != 0 {
				bf2142Base, err1 := getModuleBase(pid, "BF2142.exe")
				rendBase, err2 := getModuleBase(pid, "RendDX9.dll")
				if err1 == nil && err2 == nil {
					fmt.Printf("FOUND (PID: %d)\n", pid)
					fmt.Printf("[*] BF2142.exe base:  0x%08X\n", bf2142Base)
					fmt.Printf("[*] RendDX9.dll base: 0x%08X\n", rendBase)
					return pid, &MemoryReader{hProcess: syscall.Handle(hProcess)}, bf2142Base, rendBase, true
				}
				procCloseHandle.Call(hProcess)
			} else {
				fmt.Printf("\n[-] OpenProcess failed (PID %d): %v\n", pid, openErr)
			}
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

	pid, reader, bf2142Base, rendBase, ok := attachToGame(sigChan)
	if !ok {
		return
	}
	defer func() {
		if reader != nil && reader.hProcess != 0 {
			procCloseHandle.Call(uintptr(reader.hProcess))
		}
	}()

	classMgrPtrAddr := bf2142Base + 0x67D068
	playerMgrPtrAddr := bf2142Base + 0x685228
	rendererPtrAddr := rendBase + 0x1F8E58

	fmt.Printf("[*] Static pointers configuration:\n")
	fmt.Printf("    ClassManager ptr:  0x%08X (BF2142.exe + 0x67D068)\n", classMgrPtrAddr)
	fmt.Printf("    PlayerManager ptr: 0x%08X (BF2142.exe + 0x685228)\n", playerMgrPtrAddr)
	fmt.Printf("    Renderer ptr:      0x%08X (RendDX9.dll + 0x1F8E58)\n\n", rendererPtrAddr)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	didScanPlayer := false

	for {
		select {
		case <-sigChan:
			fmt.Println("\n[*] Exiting dumper...")
			return
		case <-ticker.C:
			if !reader.IsProcessAlive() {
				fmt.Printf("\n[!] BF2142.exe (PID %d) closed. Re-attaching...\n", pid)
				procCloseHandle.Call(uintptr(reader.hProcess))
				reader.hProcess = 0
				pid, reader, bf2142Base, rendBase, ok = attachToGame(sigChan)
				if !ok {
					return
				}
				classMgrPtrAddr = bf2142Base + 0x67D068
				playerMgrPtrAddr = bf2142Base + 0x685228
				rendererPtrAddr = rendBase + 0x1F8E58
				didScanPlayer = false
				continue
			}

			classMgr, _ := reader.ReadUint32(classMgrPtrAddr)
			playerMgr, _ := reader.ReadUint32(playerMgrPtrAddr)
			renderer, _ := reader.ReadUint32(rendererPtrAddr)

			if (playerMgr == 0 || renderer == 0) && isValidUserAddr(classMgr) {
				classes := traverseClassManager(reader, classMgr)
				if playerMgr == 0 {
					if pm, ok := classes["PlayerManager"]; ok {
						playerMgr = pm
					}
				}
				if renderer == 0 {
					if rend, ok := classes["Renderer"]; ok {
						renderer = rend
					}
				}
			}

			fmt.Println("--------------------------------------------------------------------------------")
			fmt.Printf("[%s] Status (PID %d):\n", time.Now().Format("15:04:05"), pid)
			fmt.Printf("  ClassManager:  0x%08X\n", classMgr)
			fmt.Printf("  PlayerManager: 0x%08X\n", playerMgr)
			fmt.Printf("  Renderer:      0x%08X\n", renderer)

			if isValidUserAddr(renderer) {
				activeCam, _ := reader.ReadUint32(renderer + 0xF8)
				fmt.Printf("  ActiveCamera:  0x%08X\n", activeCam)
				if isValidUserAddr(activeCam) {
					nearA, _ := reader.ReadFloat32(activeCam + 0x14C)
					nearB, _ := reader.ReadFloat32(activeCam + 0x150)
					nearC, _ := reader.ReadFloat32(activeCam + 0x154)
					nearD, _ := reader.ReadFloat32(activeCam + 0x158)
					fmt.Printf("    NearPlane (A,B,C,D): (%.3f, %.3f, %.3f, %.3f)\n", nearA, nearB, nearC, nearD)

					// Real viewport stored on ActiveCamera (+0x10 Width, +0x14 Height)
					vpWidth, errW1 := reader.ReadFloat32(activeCam + 0x10)
					vpHeight, errH1 := reader.ReadFloat32(activeCam + 0x14)
					if errW1 == nil && errH1 == nil && isFiniteFloat(vpWidth) && isFiniteFloat(vpHeight) {
						fmt.Printf("    Viewport (Cam+0x10/+0x14): %.0f x %.0f\n", vpWidth, vpHeight)
					}

					// World matrix (+0x40)
					if worldMat, errW := reader.ReadMatrix4x4(activeCam + 0x40); errW == nil {
						fmt.Printf("    Camera World Matrix (+0x40):\n")
						fmt.Printf("      Right:   [%7.3f, %7.3f, %7.3f, %7.3f]\n", worldMat[0][0], worldMat[0][1], worldMat[0][2], worldMat[0][3])
						fmt.Printf("      Up:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", worldMat[1][0], worldMat[1][1], worldMat[1][2], worldMat[1][3])
						fmt.Printf("      Forward: [%7.3f, %7.3f, %7.3f, %7.3f]\n", worldMat[2][0], worldMat[2][1], worldMat[2][2], worldMat[2][3])
						fmt.Printf("      Pos:     [%7.2f, %7.2f, %7.2f, %7.2f]\n", worldMat[3][0], worldMat[3][1], worldMat[3][2], worldMat[3][3])
					}

					// View matrix (+0x80)
					if viewMat, errV := reader.ReadMatrix4x4(activeCam + 0x80); errV == nil {
						fmt.Printf("    Camera View Matrix (+0x80):\n")
						fmt.Printf("      R0:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", viewMat[0][0], viewMat[0][1], viewMat[0][2], viewMat[0][3])
						fmt.Printf("      R1:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", viewMat[1][0], viewMat[1][1], viewMat[1][2], viewMat[1][3])
						fmt.Printf("      R2:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", viewMat[2][0], viewMat[2][1], viewMat[2][2], viewMat[2][3])
						fmt.Printf("      R3:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", viewMat[3][0], viewMat[3][1], viewMat[3][2], viewMat[3][3])
					}

					// Real DirectX Projection matrix (+0x140)
					if projMat, errP := reader.ReadMatrix4x4(activeCam + 0x140); errP == nil {
						projValid := isValidProjMatrix(projMat)
						fmt.Printf("    Camera Proj Matrix (+0x140) [valid=%v]:\n", projValid)
						fmt.Printf("      R0:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", projMat[0][0], projMat[0][1], projMat[0][2], projMat[0][3])
						fmt.Printf("      R1:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", projMat[1][0], projMat[1][1], projMat[1][2], projMat[1][3])
						fmt.Printf("      R2:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", projMat[2][0], projMat[2][1], projMat[2][2], projMat[2][3])
						fmt.Printf("      R3:      [%7.3f, %7.3f, %7.3f, %7.3f]\n", projMat[3][0], projMat[3][1], projMat[3][2], projMat[3][3])
						if projValid && projMat[1][1] > 0.1 && projMat[1][1] < 20.0 {
							derivedFovY := 2.0 * math.Atan(1.0/float64(projMat[1][1])) * 180.0 / math.Pi
							derivedFovX := 2.0 * math.Atan(1.0/float64(projMat[0][0])) * 180.0 / math.Pi
							fmt.Printf("      -> Derived FOV: vFOV=%.2f deg (P11=%.4f), hFOV=%.2f deg (P00=%.4f)\n",
								derivedFovY, projMat[1][1], derivedFovX, projMat[0][0])
						}
					}

					fmt.Printf("    Camera Header Floats (+0x08..+0x3C):\n")
					for _, off := range []uint32{0x08, 0x0C, 0x10, 0x14, 0x1C, 0x20, 0x24, 0x2C, 0x30, 0x34, 0x38, 0x3C} {
						if val, err := reader.ReadFloat32(activeCam + off); err == nil && isFiniteFloat(val) && val != 0 {
							fmt.Printf("      +0x%03X: %9.4f\n", off, val)
						}
					}
				}
			}

			if isValidUserAddr(playerMgr) {
				listHead, errH := reader.ReadUint32(playerMgr + 0x10)
				playerCount, errC := reader.ReadUint32(playerMgr + 0x14)

				if errH == nil && errC == nil && isValidUserAddr(listHead) {
					fmt.Printf("\n  Players Count: %d (std::list at 0x%08X)\n", playerCount, listHead)

					if playerCount > 0 && playerCount <= 64 {
						fmt.Printf("  %-4s | %-18s | %-4s | %-5s | %-3s | %-13s | %-24s | %-20s\n",
							"ID", "Nickname", "Team", "Alive", "Veh", "Score (K/D)", "Eye Pos (X, Y, Z)", "Look Dir (X, Y, Z)")
						fmt.Println("  -----+--------------------+------+-------+-----+---------------+--------------------------+---------------------")

						curr, _ := reader.ReadUint32(listHead)
						firstPlayerAddr := uint32(0)
						firstLiveAddr := uint32(0)
						firstDeadAddr := uint32(0)
						visitedNodes := make(map[uint32]bool)

						for i := uint32(0); i < playerCount && isValidUserAddr(curr) && curr != listHead && !visitedNodes[curr]; i++ {
							visitedNodes[curr] = true
							playerPtr, errP := reader.ReadUint32(curr + 0x08)
							if errP == nil && isValidUserAddr(playerPtr) {
								if firstPlayerAddr == 0 {
									firstPlayerAddr = playerPtr
								}

								pidVal, errID := reader.ReadInt32(playerPtr + 0xB8)
								if errID != nil {
									pidVal = int32(i)
								}
								name := reader.ReadCPlayerName(playerPtr)
								team, _ := reader.ReadUint32(playerPtr + 0xD8)

								// InVehicle disabled until verified (CPlayer+0x88 points to wrapper even on foot)
								inVeh := false

								scoreStr := "--"
								if scorePtr, errS := reader.ReadUint32(playerPtr + 0x100); errS == nil && isValidUserAddr(scorePtr) {
									scoreVal, e0 := reader.ReadInt32(scorePtr + 0x00)
									deathsVal, e1 := reader.ReadInt32(scorePtr + 0x10)
									killsVal, e2 := reader.ReadInt32(scorePtr + 0x14)
									if e0 == nil && e1 == nil && e2 == nil {
										scoreStr = fmt.Sprintf("%4d (%2d/%2d)", scoreVal, killsVal, deathsVal)
									}
								}

								vmPtr, _ := reader.ReadUint32(playerPtr + 0x84)
								eyePosStr := "not spawned"
								lookDirStr := "not spawned"
								isAlive := false
								isDeadDefault := false

								if isValidUserAddr(vmPtr) {
									lookDir, errD := reader.ReadVector3(vmPtr + 0xE8)
									eyePos, errE := reader.ReadVector3(vmPtr + 0xF8)
									if errE == nil && isValidVec3(eyePos) {
										eyePosStr = fmt.Sprintf("(%6.1f,%6.1f,%6.1f)", eyePos[0], eyePos[1], eyePos[2])
									}
									if errD == nil && isValidVec3(lookDir) {
										lookDirStr = fmt.Sprintf("(%5.2f,%5.2f,%5.2f)", lookDir[0], lookDir[1], lookDir[2])
									}

									lookLen := math.Sqrt(float64(lookDir[0]*lookDir[0] + lookDir[1]*lookDir[1] + lookDir[2]*lookDir[2]))
									isOrigin := math.Abs(float64(eyePos[0])) < 0.05 && math.Abs(float64(eyePos[1])) < 0.05 && math.Abs(float64(eyePos[2])) < 0.05
									isDefault1 := math.Abs(float64(eyePos[0])) < 0.05 && math.Abs(float64(eyePos[1]-1.0)) < 0.05 && math.Abs(float64(eyePos[2])) < 0.05
									isDeadDefault = isOrigin || isDefault1

									if errD == nil && errE == nil && isValidVec3(eyePos) && isValidVec3(lookDir) && !isDeadDefault && lookLen > 0.5 {
										isAlive = true
									}
								}

								if isAlive && firstLiveAddr == 0 {
									firstLiveAddr = playerPtr
								} else if !isAlive && isDeadDefault && firstDeadAddr == 0 {
									firstDeadAddr = playerPtr
								}

								aliveStr := "DEAD"
								if isAlive {
									aliveStr = "LIVE"
								}
								vehStr := " "
								if inVeh {
									vehStr = "Y"
								}

								fmt.Printf("  %4d | %-18s | %4d | %-5s | %-3s | %-13s | %-24s | %-20s\n",
									pidVal, name, team, aliveStr, vehStr, scoreStr, eyePosStr, lookDirStr)
							}
							curr, _ = reader.ReadUint32(curr)
						}

						if firstLiveAddr != 0 || firstDeadAddr != 0 {
							fmt.Println("\n  --- CPlayer Offset Probe (+0x78..+0x98, +0x100..+0x120) ---")
							if firstLiveAddr != 0 {
								dumpPlayerRawRanges(reader, "LIVE", firstLiveAddr)
							}
							if firstDeadAddr != 0 {
								dumpPlayerRawRanges(reader, "DEAD(0,1,0)", firstDeadAddr)
							}
						}

						if *scanPlayerFlag && !didScanPlayer && firstPlayerAddr != 0 {
							scanPlayer(reader, firstPlayerAddr)
							didScanPlayer = true
						}
					} else {
						fmt.Println("  (No players currently active in demo)")
					}
				}
			}
			fmt.Println()
		}
	}
}
