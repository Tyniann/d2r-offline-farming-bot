package input

// defaultWindowTitle is the expected D2R main window caption on Windows.
const defaultWindowTitle = "Diablo II: Resurrected"

// nativeWindow is a platform window handle (HWND on Windows).
type nativeWindow = uintptr

// WindowInfo describes the D2R client area used by future input actions.
type WindowInfo struct {
	PID          uint32
	Title        string
	Handle       uintptr // HWND on Windows.
	ClientLeft   int
	ClientTop    int
	ClientWidth  int
	ClientHeight int
}

// windowAPI discovers the game window and measures its client area.
type windowAPI interface {
	FindMainWindow(pid uint32, title string) (nativeWindow, error)
	ClientArea(window nativeWindow) (WindowInfo, error)
}
