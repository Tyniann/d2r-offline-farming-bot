package input

const (
	preferredClientWidth  = 1280
	preferredClientHeight = 720
)

type windowCandidate struct {
	hwnd   nativeWindow
	owned  bool
	width  int
	height int
}

func (c windowCandidate) usable() bool {
	return c.width > 0 && c.height > 0
}

func (c windowCandidate) preferredSize() bool {
	return c.width == preferredClientWidth && c.height == preferredClientHeight
}

// pickMainWindow chooses the D2R client among visible top-level candidates.
// A measured client area beats a 0×0/minimized HWND; 1280×720 beats other sizes;
// an unowned window wins ties so owned dialogs stay secondary.
func pickMainWindow(candidates []windowCandidate) (nativeWindow, bool) {
	if len(candidates) == 0 {
		return 0, false
	}
	best := 0
	for i := 1; i < len(candidates); i++ {
		if betterWindow(candidates[i], candidates[best]) {
			best = i
		}
	}
	return candidates[best].hwnd, true
}

func betterWindow(a, b windowCandidate) bool {
	if a.usable() != b.usable() {
		return a.usable()
	}
	if a.preferredSize() != b.preferredSize() {
		return a.preferredSize()
	}
	if a.owned != b.owned {
		return !a.owned
	}
	return false
}
