package app

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveScreenAnchorPNGPublishesImage(t *testing.T) {
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	path, err := saveScreenAnchorPNG(dir, "character-screen", time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC), img)
	if err != nil {
		t.Fatalf("saveScreenAnchorPNG() error = %v", err)
	}
	if filepath.Dir(path) != dir || filepath.Ext(path) != ".png" {
		t.Fatalf("path = %q", path)
	}
	if info, statErr := os.Stat(path); statErr != nil || info.Size() == 0 {
		t.Fatalf("published image info=%v err=%v", info, statErr)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".screen-anchor-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files = %v, err=%v", matches, err)
	}
}
