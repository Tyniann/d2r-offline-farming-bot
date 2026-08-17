package input

import "testing"

func TestPickMainWindowPrefersUsableClientArea(t *testing.T) {
	empty := windowCandidate{hwnd: 0x1, width: 0, height: 0}
	game := windowCandidate{hwnd: 0x2, owned: true, width: 1280, height: 720}
	hwnd, ok := pickMainWindow([]windowCandidate{empty, game})
	if !ok || hwnd != 0x2 {
		t.Fatalf("hwnd=%#x ok=%v, want owned 1280×720 over 0×0", hwnd, ok)
	}
}

func TestPickMainWindowPrefersPreferredSize(t *testing.T) {
	wide := windowCandidate{hwnd: 0x1, width: 1920, height: 1080}
	game := windowCandidate{hwnd: 0x2, owned: true, width: 1280, height: 720}
	hwnd, ok := pickMainWindow([]windowCandidate{wide, game})
	if !ok || hwnd != 0x2 {
		t.Fatalf("hwnd=%#x ok=%v, want 1280×720", hwnd, ok)
	}
}

func TestPickMainWindowPrefersUnownedWhenSizesMatch(t *testing.T) {
	owned := windowCandidate{hwnd: 0x1, owned: true, width: 1280, height: 720}
	unowned := windowCandidate{hwnd: 0x2, width: 1280, height: 720}
	hwnd, ok := pickMainWindow([]windowCandidate{owned, unowned})
	if !ok || hwnd != 0x2 {
		t.Fatalf("hwnd=%#x ok=%v, want unowned", hwnd, ok)
	}
}

func TestPickMainWindowEmpty(t *testing.T) {
	if _, ok := pickMainWindow(nil); ok {
		t.Fatal("empty candidates must not pick a window")
	}
}
