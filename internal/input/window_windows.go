//go:build windows

package input

import (
	"fmt"
	"log/slog"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const gwOwner = 4

var (
	modUser32                    = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = modUser32.NewProc("EnumWindows")
	procGetWindowThreadProcessId = modUser32.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW           = modUser32.NewProc("GetWindowTextW")
	procIsWindowVisible          = modUser32.NewProc("IsWindowVisible")
	procGetWindow                = modUser32.NewProc("GetWindow")
	procGetClientRect            = modUser32.NewProc("GetClientRect")
	procClientToScreen           = modUser32.NewProc("ClientToScreen")
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
	procGetWindowThreadProcessId.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&pid)))
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
