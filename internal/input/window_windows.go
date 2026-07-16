//go:build windows

package input

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	gwOwner   = 4
	swRestore = 9
)

var (
	modUser32                    = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = modUser32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW           = modUser32.NewProc("GetWindowTextW")
	procIsWindowVisible          = modUser32.NewProc("IsWindowVisible")
	procGetWindow                = modUser32.NewProc("GetWindow")
	procGetClientRect            = modUser32.NewProc("GetClientRect")
	procClientToScreen           = modUser32.NewProc("ClientToScreen")
	procSetForegroundWindow      = modUser32.NewProc("SetForegroundWindow")
	procGetForegroundWindow      = modUser32.NewProc("GetForegroundWindow")
	procAttachThreadInput        = modUser32.NewProc("AttachThreadInput")
	procBringWindowToTop         = modUser32.NewProc("BringWindowToTop")
	procSetActiveWindow          = modUser32.NewProc("SetActiveWindow")
	procShowWindow               = modUser32.NewProc("ShowWindow")
	modKernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procGetCurrentThreadID       = modKernel32.NewProc("GetCurrentThreadId")
)

type winRect struct {
	Left, Top, Right, Bottom int32
}

type winPoint struct {
	X, Y int32
}

type user32WindowAPI struct {
	log *slog.Logger
}

func defaultWindowAPI(log *slog.Logger) windowAPI {
	return &user32WindowAPI{log: log}
}

func (w *user32WindowAPI) FindMainWindow(pid uint32, title string) (nativeWindow, error) {
	// Context is handed to the callback via package state instead of lParam:
	// round-tripping a Go pointer through uintptr trips go vet (unsafe.Pointer misuse),
	// and EnumWindows runs synchronously, so a mutex-guarded slot is sufficient.
	ctx := &enumWindowsContext{
		targetPID: pid,
		title:     title,
	}
	enumWindowsMu.Lock()
	enumWindowsCtx = ctx
	defer func() {
		enumWindowsCtx = nil
		enumWindowsMu.Unlock()
	}()

	ret, _, err := procEnumWindows.Call(enumWindowsCallback, 0)
	if ret == 0 {
		return 0, fmt.Errorf("enum windows: %w", err)
	}

	switch len(ctx.matches) {
	case 0:
		return 0, fmt.Errorf("find main window pid=%d title=%q: %w", pid, title, ErrWindowNotFound)
	case 1:
		return ctx.matches[0], nil
	default:
		if w.log != nil {
			w.log.Warn("multiple matching windows; using first candidate",
				"count", len(ctx.matches),
				"pid", pid,
			)
		}
		return ctx.matches[0], nil
	}
}

func (w *user32WindowAPI) ClientArea(hwnd nativeWindow) (WindowInfo, error) {
	var rect winRect
	if r, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect))); r == 0 {
		return WindowInfo{}, fmt.Errorf("get client rect hwnd=%#x: %w", hwnd, ErrInvalidClientArea)
	}

	width := int(rect.Right - rect.Left)
	height := int(rect.Bottom - rect.Top)
	if width <= 0 || height <= 0 {
		return WindowInfo{}, fmt.Errorf("client rect hwnd=%#x %dx%d: %w", hwnd, width, height, ErrInvalidClientArea)
	}

	point := winPoint{}
	if r, _, _ := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&point))); r == 0 {
		return WindowInfo{}, fmt.Errorf("client to screen hwnd=%#x: %w", hwnd, ErrInvalidClientArea)
	}

	return WindowInfo{
		Title:        getWindowText(hwnd),
		Handle:       hwnd,
		ClientLeft:   int(point.X),
		ClientTop:    int(point.Y),
		ClientWidth:  width,
		ClientHeight: height,
	}, nil
}

func (w *user32WindowAPI) Activate(hwnd nativeWindow) error {
	// Windows' foreground lock can reject SetForegroundWindow while the local
	// dashboard is active. Temporarily joining the GUI input queues lets us make
	// a bounded activation request without synthesizing Alt, mouse, or keyboard
	// input. Controller.Focus remains the fail-closed authority and verifies the
	// result with GetForegroundWindow before any gameplay input is permitted.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	_, _, _ = procShowWindow.Call(hwnd, swRestore)
	if r, _, _ := procSetForegroundWindow.Call(hwnd); r != 0 {
		return nil
	}

	currentThread, _, _ := procGetCurrentThreadID.Call()
	foreground, _, _ := procGetForegroundWindow.Call()
	foregroundThread, _, _ := procGetWindowThreadProcessId.Call(foreground, 0)
	targetThread, _, _ := procGetWindowThreadProcessId.Call(hwnd, 0)

	attachedForeground := attachThreadInput(currentThread, foregroundThread, true)
	attachedTarget := false
	if targetThread != foregroundThread {
		attachedTarget = attachThreadInput(currentThread, targetThread, true)
	}
	defer func() {
		if attachedTarget {
			attachThreadInput(currentThread, targetThread, false)
		}
		if attachedForeground {
			attachThreadInput(currentThread, foregroundThread, false)
		}
	}()

	_, _, _ = procBringWindowToTop.Call(hwnd)
	_, _, _ = procSetActiveWindow.Call(hwnd)
	_, _, _ = procSetForegroundWindow.Call(hwnd)
	return nil
}

func attachThreadInput(source, target uintptr, attach bool) bool {
	if source == 0 || target == 0 || source == target {
		return false
	}
	value := uintptr(0)
	if attach {
		value = 1
	}
	result, _, _ := procAttachThreadInput.Call(source, target, value)
	return result != 0
}

func (w *user32WindowAPI) IsForeground(hwnd nativeWindow) bool {
	foreground, _, _ := procGetForegroundWindow.Call()
	return foreground == hwnd
}

type enumWindowsContext struct {
	targetPID uint32
	title     string
	matches   []nativeWindow
}

// enumWindowsCallback is created once: syscall.NewCallback allocations are
// never released and Windows caps their total count per process.
var (
	enumWindowsCallback = syscall.NewCallback(enumWindowsProc)
	enumWindowsMu       sync.Mutex
	enumWindowsCtx      *enumWindowsContext
)

func enumWindowsProc(hwnd syscall.Handle, _ uintptr) uintptr {
	ctx := enumWindowsCtx
	if ctx == nil {
		return 0
	}

	var pid uint32
	//nolint:errcheck // GetWindowThreadProcessId meldet Fehler über pid==0, nicht über den Rückgabewert von Call.
	_, _, _ = procGetWindowThreadProcessId.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&pid)))
	if pid != ctx.targetPID {
		return 1
	}

	visible, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
	if visible == 0 {
		return 1
	}

	owner, _, _ := procGetWindow.Call(uintptr(hwnd), gwOwner)
	if owner != 0 {
		return 1
	}

	if getWindowText(uintptr(hwnd)) != ctx.title {
		return 1
	}

	ctx.matches = append(ctx.matches, uintptr(hwnd))
	return 1
}

func getWindowText(hwnd uintptr) string {
	buf := make([]uint16, 256)
	n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}
