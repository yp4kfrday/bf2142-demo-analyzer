package game

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"syscall"
	"unicode/utf8"
	"unsafe"

	"demoanalyzer/internal/math3d"
)

var (
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")

	procOpenProcess          = modKernel32.NewProc("OpenProcess")
	procCloseHandle          = modKernel32.NewProc("CloseHandle")
	procReadProcessMemory    = modKernel32.NewProc("ReadProcessMemory")
	procWriteProcessMemory   = modKernel32.NewProc("WriteProcessMemory")
	procGetExitCodeProcess   = modKernel32.NewProc("GetExitCodeProcess")
	procCreateToolhelp32Snap = modKernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First       = modKernel32.NewProc("Process32FirstW")
	procProcess32Next        = modKernel32.NewProc("Process32NextW")
	procModule32First        = modKernel32.NewProc("Module32FirstW")
	procModule32Next         = modKernel32.NewProc("Module32NextW")
)

const (
	processVMOperation      = 0x0008
	processVMRead           = 0x0010
	processVMWrite          = 0x0020
	processQueryInformation = 0x0400
	stillActive             = 259

	th32csSnapProcess  = 0x00000002
	th32csSnapModule   = 0x00000008
	th32csSnapModule32 = 0x00000010
)

type processEntry32W struct {
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

type moduleEntry32W struct {
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

func IsValidUserAddr(addr uint32) bool {
	return addr >= MinUserAddr && addr < MaxUserAddr
}

func (m *MemoryReader) Close() {
	if m != nil && m.hProcess != 0 {
		procCloseHandle.Call(uintptr(m.hProcess))
		m.hProcess = 0
	}
}

func (m *MemoryReader) IsProcessAlive() bool {
	if m == nil || m.hProcess == 0 {
		return false
	}
	var code uint32
	ret, _, _ := procGetExitCodeProcess.Call(uintptr(m.hProcess), uintptr(unsafe.Pointer(&code)))
	return ret != 0 && code == stillActive
}

func (m *MemoryReader) ReadRaw(addr uint32, buf []byte) error {
	if len(buf) == 0 {
		return nil
	}
	if !IsValidUserAddr(addr) || uint64(addr)+uint64(len(buf)) > uint64(MaxUserAddr) {
		return fmt.Errorf("address out of user-mode range: 0x%08X", addr)
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
		return fmt.Errorf("partial read at 0x%08X: %d/%d", addr, bytesRead, len(buf))
	}
	return nil
}

func (m *MemoryReader) WriteRaw(addr uint32, buf []byte) error {
	if len(buf) == 0 {
		return nil
	}
	if !IsValidUserAddr(addr) || uint64(addr)+uint64(len(buf)) > uint64(MaxUserAddr) {
		return fmt.Errorf("address out of user-mode range: 0x%08X", addr)
	}
	var bytesWritten uintptr
	ret, _, err := procWriteProcessMemory.Call(
		uintptr(m.hProcess),
		uintptr(addr),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&bytesWritten)),
	)
	if ret == 0 {
		return err
	}
	if bytesWritten != uintptr(len(buf)) {
		return fmt.Errorf("partial write at 0x%08X: %d/%d", addr, bytesWritten, len(buf))
	}
	return nil
}

func (m *MemoryReader) WriteInt32(addr uint32, val int32) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(val))
	return m.WriteRaw(addr, buf[:])
}

func (m *MemoryReader) WriteFloat32(addr uint32, val float32) error {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], math.Float32bits(val))
	return m.WriteRaw(addr, buf[:])
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

func (m *MemoryReader) ReadVector3(addr uint32) (math3d.Vector3, error) {
	var buf [12]byte
	var v math3d.Vector3
	if err := m.ReadRaw(addr, buf[:]); err != nil {
		return v, err
	}
	v.X = math.Float32frombits(binary.LittleEndian.Uint32(buf[0:4]))
	v.Y = math.Float32frombits(binary.LittleEndian.Uint32(buf[4:8]))
	v.Z = math.Float32frombits(binary.LittleEndian.Uint32(buf[8:12]))
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

func IsPrintableTextBytes(b []byte) bool {
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

func IsValidName(s string) bool {
	if len(s) == 0 || len(s) > 48 {
		return false
	}
	return IsPrintableTextBytes([]byte(s))
}

// ReadStdString safely reads an MSVC basic_string<char> (both 28-byte and 24-byte layouts).
func (m *MemoryReader) ReadStdString(addr uint32) string {
	if !IsValidUserAddr(addr) {
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
			if IsPrintableTextBytes(inlineBuf) {
				return string(inlineBuf), true
			}
			return "", false
		}
		ptr := binary.LittleEndian.Uint32(raw[bufOff : bufOff+4])
		if !IsValidUserAddr(ptr) {
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
			if IsPrintableTextBytes(heapBuf) {
				return string(heapBuf), true
			}
		}
		return "", false
	}

	if s, ok := tryLayout(4, 20, 24); ok {
		return s
	}
	if s, ok := tryLayout(0, 16, 20); ok {
		return s
	}

	if n4 := bytes.IndexByte(raw[4:20], 0); n4 > 0 && IsPrintableTextBytes(raw[4:4+n4]) {
		return string(raw[4 : 4+n4])
	}
	if n0 := bytes.IndexByte(raw[0:16], 0); n0 > 0 && IsPrintableTextBytes(raw[0:n0]) {
		return string(raw[0:n0])
	}
	return ""
}

func (m *MemoryReader) ReadCPlayerName(playerAddr uint32) string {
	for _, off := range []uint32{OffPlayerNameA, OffPlayerNameB} {
		s := m.ReadStdString(playerAddr + off)
		if IsValidName(s) {
			return s
		}
	}

	var directBuf [32]byte
	if err := m.ReadRaw(playerAddr+OffPlayerNameB, directBuf[:]); err == nil {
		if n := bytes.IndexByte(directBuf[:], 0); n > 0 {
			s := string(directBuf[:n])
			if IsValidName(s) {
				return s
			}
		}
	}
	return "<unknown>"
}

func FindProcess(name string) (uint32, error) {
	snap, _, err := procCreateToolhelp32Snap.Call(uintptr(th32csSnapProcess), 0)
	if snap == uintptr(syscall.InvalidHandle) {
		return 0, err
	}
	defer procCloseHandle.Call(snap)

	var entry processEntry32W
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

func GetModuleBase(pid uint32, modName string) (uint32, error) {
	snap, _, err := procCreateToolhelp32Snap.Call(uintptr(th32csSnapModule|th32csSnapModule32), uintptr(pid))
	if snap == uintptr(syscall.InvalidHandle) {
		return 0, err
	}
	defer procCloseHandle.Call(snap)

	var entry moduleEntry32W
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

func TraverseClassManager(m *MemoryReader, classMgrAddr uint32) map[string]uint32 {
	classes := make(map[string]uint32)
	if !IsValidUserAddr(classMgrAddr) {
		return classes
	}
	headNode, err := m.ReadUint32(classMgrAddr + OffClassMgrHead)
	if err != nil || !IsValidUserAddr(headNode) {
		return classes
	}
	size, err := m.ReadUint32(classMgrAddr + OffClassMgrSize)
	if err != nil || size == 0 || size > 256 {
		return classes
	}
	root, err := m.ReadUint32(headNode + OffMapRoot)
	if err != nil || !IsValidUserAddr(root) || root == headNode {
		return classes
	}

	visited := make(map[uint32]bool)
	var walk func(node uint32, depth int)
	walk = func(node uint32, depth int) {
		if depth > 64 || len(visited) >= 512 || !IsValidUserAddr(node) || node == headNode || visited[node] {
			return
		}
		visited[node] = true

		className := m.ReadStdString(node + OffMapKeyA)
		if className == "" {
			className = m.ReadStdString(node + OffMapKeyB)
		}
		if valPtr, err1 := m.ReadUint32(node + OffMapValPtr); err1 == nil && IsValidUserAddr(valPtr) {
			if instPtr, err2 := m.ReadUint32(valPtr); err2 == nil && IsValidUserAddr(instPtr) && className != "" {
				classes[className] = instPtr
			}
		}
		if left, errL := m.ReadUint32(node + OffMapLeft); errL == nil {
			walk(left, depth+1)
		}
		if right, errR := m.ReadUint32(node + OffMapRight); errR == nil {
			walk(right, depth+1)
		}
	}
	walk(root, 0)
	return classes
}

type Attachment struct {
	PID              uint32
	Reader           *MemoryReader
	CanWrite         bool
	BF2142Base       uint32
	RendBase         uint32
	GamePtrAddr      uint32
	ClassMgrPtrAddr  uint32
	PlayerMgrPtrAddr uint32
	RendererPtrAddr  uint32
	CachedHwnd       uintptr
	NameCache        map[uint64]string
}

func (a *Attachment) Close() {
	if a == nil {
		return
	}
	if a.Reader != nil {
		a.Reader.Close()
	}
	a.PID = 0
	a.CachedHwnd = 0
}

func TryAttach() (*Attachment, error) {
	pid, err := FindProcess("BF2142.exe")
	if err != nil {
		return nil, err
	}
	canWrite := true
	hProcess, _, openErr := procOpenProcess.Call(
		uintptr(processVMRead|processVMWrite|processVMOperation|processQueryInformation),
		0,
		uintptr(pid),
	)
	if hProcess == 0 {
		canWrite = false
		hProcess, _, openErr = procOpenProcess.Call(
			uintptr(processVMRead|processQueryInformation),
			0,
			uintptr(pid),
		)
		if hProcess == 0 {
			return nil, fmt.Errorf("OpenProcess(PID %d) failed: %v", pid, openErr)
		}
	}
	bf2142Base, err1 := GetModuleBase(pid, "BF2142.exe")
	rendBase, err2 := GetModuleBase(pid, "RendDX9.dll")
	if err1 != nil || err2 != nil {
		procCloseHandle.Call(hProcess)
		if err1 != nil {
			return nil, err1
		}
		return nil, err2
	}
	return &Attachment{
		PID:              pid,
		Reader:           &MemoryReader{hProcess: syscall.Handle(hProcess)},
		CanWrite:         canWrite,
		BF2142Base:       bf2142Base,
		RendBase:         rendBase,
		GamePtrAddr:      bf2142Base + OffGameStatic,
		ClassMgrPtrAddr:  bf2142Base + OffClassManagerStatic,
		PlayerMgrPtrAddr: bf2142Base + OffPlayerManagerStatic,
		RendererPtrAddr:  rendBase + OffRendererStatic,
		NameCache:        make(map[uint64]string),
	}, nil
}
