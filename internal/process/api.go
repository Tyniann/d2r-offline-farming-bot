package process

// ProcessInfo holds metadata for a discovered OS process.
type ProcessInfo struct {
	PID  uint32
	Name string
}

// nativeHandle is an opaque OS process handle used by the platform adapter.
type nativeHandle uintptr

// processAPI abstracts Windows process operations for testability.
type processAPI interface {
	FindProcessByName(name string) (ProcessInfo, error)
	OpenReadHandle(pid uint32) (nativeHandle, error)
	BoundPID(handle nativeHandle) (uint32, error)
	ProcessImagePath(handle nativeHandle) (string, error)
	FileVersion(path string) (string, error)
	ModuleImage(pid uint32, moduleName string) (base uintptr, size uint32, err error)
	IsAlive(handle nativeHandle) bool
	Close(handle nativeHandle) error
	ReadMemory(handle nativeHandle, addr uintptr, buf []byte) error
}
