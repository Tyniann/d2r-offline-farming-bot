//go:build windows

package process

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	processQueryInformation = 0x0400
	processVMRead           = 0x0010
	th32csSnapModule        = 0x00000008
	// stillActive is the STILL_ACTIVE exit code from winbase.h (259 / 0x103).
	stillActive = 259
)

type windowsAPI struct{}

func defaultAPI() processAPI {
	return &windowsAPI{}
}

func (w *windowsAPI) FindProcessByName(name string) (ProcessInfo, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ProcessInfo{}, fmt.Errorf("create process snapshot: %w", err)
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snapshot, &entry); err != nil {
		return ProcessInfo{}, fmt.Errorf("enumerate processes: %w", mapWindowsError(err))
	}

	var matches []ProcessInfo

	for {
		exeName := windows.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(exeName, name) {
			matches = append(matches, ProcessInfo{
				PID:  entry.ProcessID,
				Name: exeName,
			})
		}

		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}

	switch len(matches) {
	case 0:
		return ProcessInfo{}, fmt.Errorf("find process %s: %w", name, ErrNotFound)
	case 1:
		return matches[0], nil
	default:
		return ProcessInfo{}, fmt.Errorf("find process %s: %w (%d instances)", name, ErrMultipleInstances, len(matches))
	}
}

func (w *windowsAPI) OpenReadHandle(pid uint32) (nativeHandle, error) {
	access := uint32(processQueryInformation | processVMRead)
	handle, err := windows.OpenProcess(access, false, pid)
	if err != nil {
		if isAccessDenied(err) {
			return 0, fmt.Errorf("open process pid=%d: %w (try running bot as administrator)", pid, ErrAccessDenied)
		}
		return 0, fmt.Errorf("open process pid=%d: %w", pid, mapWindowsError(err))
	}
	return nativeHandle(handle), nil
}

func (w *windowsAPI) BoundPID(handle nativeHandle) (uint32, error) {
	pid, err := windows.GetProcessId(windows.Handle(handle))
	if err != nil {
		return 0, fmt.Errorf("read bound process PID: %w", mapWindowsError(err))
	}
	return pid, nil
}

func (w *windowsAPI) ProcessImagePath(handle nativeHandle) (string, error) {
	buffer := make([]uint16, 32*1024)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(windows.Handle(handle), 0, &buffer[0], &size); err != nil {
		return "", fmt.Errorf("query bound process image path: %w", mapWindowsError(err))
	}
	path := filepath.Clean(windows.UTF16ToString(buffer[:size]))
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("canonicalize bound process image path: %w", err)
	}
	absolute, err := filepath.Abs(canonical)
	if err != nil {
		return "", fmt.Errorf("make bound process image path absolute: %w", err)
	}
	return filepath.Clean(absolute), nil
}

func (w *windowsAPI) FileVersion(path string) (string, error) {
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil {
		return "", fmt.Errorf("read file version resource size: %w", err)
	}
	if size == 0 {
		return "", fmt.Errorf("read file version resource size: empty resource")
	}
	buffer := make([]byte, size)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&buffer[0])); err != nil {
		return "", fmt.Errorf("read file version resource: %w", err)
	}
	if version := queryStringFileVersion(buffer); version != "" {
		return version, nil
	}
	var value unsafe.Pointer
	var valueSize uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&buffer[0]), `\`, unsafe.Pointer(&value), &valueSize); err != nil {
		return "", fmt.Errorf("query fixed file version: %w", err)
	}
	if value == nil || valueSize < uint32(unsafe.Sizeof(vsFixedFileInfo{})) {
		return "", fmt.Errorf("query fixed file version: malformed resource")
	}
	fixed := (*vsFixedFileInfo)(value)
	if fixed.Signature != 0xFEEF04BD {
		return "", fmt.Errorf("query fixed file version: invalid signature")
	}
	parts := []uint16{uint16(fixed.FileVersionMS >> 16), uint16(fixed.FileVersionMS), uint16(fixed.FileVersionLS >> 16), uint16(fixed.FileVersionLS)}
	return canonicalWindowsVersion(fmt.Sprintf("%d.%d.%d.%d", parts[0], parts[1], parts[2], parts[3])), nil
}

func queryStringFileVersion(buffer []byte) string {
	var translations unsafe.Pointer
	var translationBytes uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&buffer[0]), `\VarFileInfo\Translation`, unsafe.Pointer(&translations), &translationBytes); err != nil || translations == nil || translationBytes < 4 {
		return ""
	}
	values := unsafe.Slice((*uint16)(translations), translationBytes/2)
	for index := 0; index+1 < len(values); index += 2 {
		for _, field := range []string{"ProductVersion", "FileVersion"} {
			subBlock := fmt.Sprintf(`\StringFileInfo\%04x%04x\%s`, values[index], values[index+1], field)
			var text unsafe.Pointer
			var characters uint32
			if err := windows.VerQueryValue(unsafe.Pointer(&buffer[0]), subBlock, unsafe.Pointer(&text), &characters); err != nil || text == nil || characters == 0 {
				continue
			}
			if version := canonicalWindowsVersion(windows.UTF16ToString(unsafe.Slice((*uint16)(text), characters))); version != "" {
				return version
			}
		}
	}
	return ""
}

func canonicalWindowsVersion(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	value = strings.ReplaceAll(value, " ", "")
	parts := strings.Split(value, ".")
	for _, part := range parts {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return ""
		}
	}
	if len(parts) == 4 && parts[3] == "0" {
		parts = parts[:3]
	}
	if len(parts) < 3 || len(parts) > 4 {
		return ""
	}
	return strings.Join(parts, ".")
}

type vsFixedFileInfo struct {
	Signature        uint32
	StructVersion    uint32
	FileVersionMS    uint32
	FileVersionLS    uint32
	ProductVersionMS uint32
	ProductVersionLS uint32
	FileFlagsMask    uint32
	FileFlags        uint32
	FileOS           uint32
	FileType         uint32
	FileSubtype      uint32
	FileDateMS       uint32
	FileDateLS       uint32
}

func (w *windowsAPI) ModuleImage(pid uint32, moduleName string) (uintptr, uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(th32csSnapModule, pid)
	if err != nil {
		return 0, 0, fmt.Errorf("create module snapshot pid=%d: %w", pid, mapWindowsError(err))
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()

	var entry windows.ModuleEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Module32First(snapshot, &entry); err != nil {
		return 0, 0, fmt.Errorf("enumerate modules pid=%d: %w", pid, mapWindowsError(err))
	}

	for {
		modName := windows.UTF16ToString(entry.Module[:])
		if strings.EqualFold(modName, moduleName) {
			return uintptr(entry.ModBaseAddr), entry.ModBaseSize, nil
		}

		if err := windows.Module32Next(snapshot, &entry); err != nil {
			break
		}
	}

	return 0, 0, fmt.Errorf("module %s in pid=%d: %w", moduleName, pid, ErrModuleNotFound)
}

func (w *windowsAPI) IsAlive(handle nativeHandle) bool {
	if handle == 0 {
		return false
	}

	var exitCode uint32
	err := windows.GetExitCodeProcess(windows.Handle(handle), &exitCode)
	if err != nil {
		return false
	}
	return exitCode == stillActive
}

func (w *windowsAPI) Close(handle nativeHandle) error {
	if handle == 0 {
		return nil
	}
	return windows.CloseHandle(windows.Handle(handle))
}

func (w *windowsAPI) ReadMemory(handle nativeHandle, addr uintptr, buf []byte) error {
	if addr == 0 {
		return fmt.Errorf("read at address 0: %w", ErrInvalidRead)
	}
	if len(buf) == 0 {
		return fmt.Errorf("read with empty buffer: %w", ErrInvalidRead)
	}

	var bytesRead uintptr
	err := windows.ReadProcessMemory(
		windows.Handle(handle),
		addr,
		&buf[0],
		uintptr(len(buf)),
		&bytesRead,
	)
	if err != nil {
		return fmt.Errorf("read memory at %#x: %w", addr, classifyReadError(mapWindowsError(err)))
	}
	if bytesRead != uintptr(len(buf)) {
		return fmt.Errorf("read memory at %#x: read %d of %d bytes: %w",
			addr, bytesRead, len(buf), ErrPartialRead)
	}
	return nil
}

func classifyReadError(err error) error {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		if e, ok := err.(syscall.Errno); ok {
			errno = e
		} else {
			return ErrReadFailed
		}
	}

	switch errno {
	case windows.ERROR_NOACCESS, windows.ERROR_INVALID_PARAMETER, windows.ERROR_PARTIAL_COPY:
		return ErrInvalidRead
	default:
		return ErrReadFailed
	}
}

func isAccessDenied(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.ERROR_ACCESS_DENIED
	}
	if errno, ok := err.(syscall.Errno); ok {
		return errno == syscall.ERROR_ACCESS_DENIED
	}
	return false
}

func mapWindowsError(err error) error {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno
	}
	if errno, ok := err.(syscall.Errno); ok {
		return errno
	}
	return err
}
