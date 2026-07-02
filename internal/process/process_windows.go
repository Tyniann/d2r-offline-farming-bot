//go:build windows

package process

import (
	"errors"
	"fmt"
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
