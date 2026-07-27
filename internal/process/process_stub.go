//go:build !windows

package process

import "fmt"

type stubAPI struct{}

func defaultAPI() processAPI {
	return &stubAPI{}
}

func (s *stubAPI) FindProcessByName(name string) (ProcessInfo, error) {
	return ProcessInfo{}, fmt.Errorf("find process %s: %w", name, ErrNotFound)
}

func (s *stubAPI) OpenReadHandle(pid uint32) (nativeHandle, error) {
	return 0, fmt.Errorf("open process pid=%d: %w", pid, ErrAccessDenied)
}

func (s *stubAPI) BoundPID(handle nativeHandle) (uint32, error) {
	return 0, fmt.Errorf("bound PID: %w", ErrAccessDenied)
}

func (s *stubAPI) ProcessImagePath(handle nativeHandle) (string, error) {
	return "", fmt.Errorf("process image path: %w", ErrAccessDenied)
}

func (s *stubAPI) FileVersion(path string) (string, error) {
	return "", fmt.Errorf("file version: %w", ErrAccessDenied)
}

func (s *stubAPI) ModuleImage(pid uint32, moduleName string) (uintptr, uint32, error) {
	return 0, 0, fmt.Errorf("module %s in pid=%d: %w", moduleName, pid, ErrModuleNotFound)
}

func (s *stubAPI) IsAlive(handle nativeHandle) bool {
	return false
}

func (s *stubAPI) Close(handle nativeHandle) error {
	return nil
}

func (s *stubAPI) ReadMemory(handle nativeHandle, addr uintptr, buf []byte) error {
	return fmt.Errorf("read memory at %#x: %w", addr, ErrReadFailed)
}
