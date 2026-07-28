package app

import (
	"image"
	"path/filepath"
	"testing"
)

func TestWriteCharacterAnchorPNGIsAtomicAndDimensionBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ui", "characters", "mrbones-selected.png")
	if err := writeCharacterAnchorPNG(path, image.NewRGBA(image.Rect(0, 0, 210, 60))); err != nil {
		t.Fatal(err)
	}
	if !validPNGSize(path, phase16CharacterAnchorSize) {
		t.Fatal("published PNG is invalid")
	}
	if err := writeCharacterAnchorPNG(path, image.NewRGBA(image.Rect(0, 0, 209, 60))); err == nil {
		t.Fatal("wrong crop size was accepted")
	}
	if !validPNGSize(path, phase16CharacterAnchorSize) {
		t.Fatal("invalid replacement changed the valid PNG")
	}
}

func TestPhase16CharacterAnchorCropIsExact(t *testing.T) {
	if phase16CharacterAnchorRect != image.Rect(1035, 48, 1245, 108) || phase16CharacterAnchorRect.Size() != phase16CharacterAnchorSize {
		t.Fatalf("rect=%v size=%v", phase16CharacterAnchorRect, phase16CharacterAnchorSize)
	}
}
